package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/antonkarounis/stoic/internal/adapters/web/framework"
	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// AuthConfig contains only auth-specific configuration, decoupling core/auth from infrastructure concerns
type AuthConfig struct {
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCLogoutURL    string
	AppURL           string
	SecretKey        []byte
	IsDev            bool
}

// Claims are the provider-independent OIDC claims (sub, email, name).
type oidcClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// AuthService encapsulates all authentication state and operations.
type AuthService struct {
	provider             *oidc.Provider
	oauth2Config         oauth2.Config
	verifier             *oidc.IDTokenVerifier
	sessionManager       ports.SessionRepository
	identityManager      ports.IdentityRepository
	cfg                  *AuthConfig
	roleExtractor        RoleExtractor
	loginRedirect        string
	loginFailureRedirect string
	onFirstLogin         func(ctx context.Context, email, name string) (models.UserID, error)
	onLogin              func(ctx context.Context, userID models.UserID, email, name string) error
}

// RoleExtractor extracts roles from raw OIDC claims.
// The default implementation handles Keycloak realm_access and resource_access claims.
// Replace with a custom function for other OIDC providers (Auth0, Okta, etc.).
type RoleExtractor func(rawClaims json.RawMessage, clientID string) ([]string, error)

// tokenData is the JSON-serializable representation stored in sessions.token_data
type tokenData struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Roles        []string  `json:"roles"`
}

func NewAuthService(ctx context.Context, cfg *AuthConfig, sessionManager ports.SessionRepository, identityManager ports.IdentityRepository) (*AuthService, error) {
	provider, err := connectOIDCProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, err
	}

	oauth2Config := oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.AppURL + "/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.OIDCClientID,
	})

	return &AuthService{
		provider:        provider,
		oauth2Config:    oauth2Config,
		verifier:        verifier,
		sessionManager:  sessionManager,
		identityManager: identityManager,
		cfg:             cfg,
		roleExtractor:   KeycloakRoleExtractor,
	}, nil
}

// KeycloakRoleExtractor extracts roles from Keycloak-specific claims (realm_access, resource_access).
// For other OIDC providers, replace AuthService.roleExtractor with a custom function.
func KeycloakRoleExtractor(rawClaims json.RawMessage, clientID string) ([]string, error) {
	var claims struct {
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		ResourceAccess map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if err := json.Unmarshal(rawClaims, &claims); err != nil {
		return nil, fmt.Errorf("parsing role claims: %w", err)
	}

	var allRoles []string
	allRoles = append(allRoles, claims.RealmAccess.Roles...)
	if clientRoles, ok := claims.ResourceAccess[clientID]; ok {
		allRoles = append(allRoles, clientRoles.Roles...)
	}

	// Filter out default Keycloak roles
	roles := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		if !isDefaultKeycloakRole(role) {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

func isDefaultKeycloakRole(role string) bool {
	if strings.HasPrefix(role, "default-roles-") {
		return true
	}
	switch role {
	case "offline_access", "uma_authorization":
		return true
	}
	return false
}

func tokenToJSON(token *oauth2.Token, roles []string) ([]byte, error) {
	td := tokenData{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
		Roles:        roles,
	}
	return json.Marshal(td)
}

func tokenFromJSON(data []byte) (*oauth2.Token, []string, error) {
	var td tokenData
	if err := json.Unmarshal(data, &td); err != nil {
		return nil, nil, err
	}
	token := &oauth2.Token{
		AccessToken:  td.AccessToken,
		TokenType:    td.TokenType,
		RefreshToken: td.RefreshToken,
		Expiry:       td.Expiry,
	}
	return token, td.Roles, nil
}

func (s *AuthService) SetLoginRedirect(url string) {
	s.loginRedirect = url
}

func (s *AuthService) SetLoginFailureRedirect(url string) {
	s.loginFailureRedirect = url
}

// SetFirstLoginHook registers a function called on the first successful OIDC login
// for an identity that has no linked domain user yet. It should provision a User
// and return the new UserID so the identity can be linked.
func (s *AuthService) SetFirstLoginHook(fn func(ctx context.Context, email, name string) (models.UserID, error)) {
	s.onFirstLogin = fn
}

// SetOnLoginHook registers a function called on every successful login for an identity
// that already has a linked domain user. Use this to sync updated email/name from the
// OIDC provider to the domain user record.
func (s *AuthService) SetOnLoginHook(fn func(ctx context.Context, userID models.UserID, email, name string) error) {
	s.onLogin = fn
}

// encryptToken serializes and encrypts token data for storage.
// Returns a JSON-safe base64-encoded string (compatible with JSONB columns).
func (s *AuthService) encryptToken(token *oauth2.Token, roles []string) ([]byte, error) {
	plaintext, err := tokenToJSON(token, roles)
	if err != nil {
		return nil, fmt.Errorf("marshaling token data: %w", err)
	}
	ciphertext, err := encrypt(plaintext, s.cfg.SecretKey)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	// Wrap in JSON string so it's valid for JSONB storage
	return json.Marshal(encoded)
}

// decryptToken decrypts and deserializes token data from storage.
func (s *AuthService) decryptToken(data []byte) (*oauth2.Token, []string, error) {
	// Unwrap JSON string
	var encoded string
	if err := json.Unmarshal(data, &encoded); err != nil {
		return nil, nil, fmt.Errorf("unmarshaling encrypted envelope: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, nil, fmt.Errorf("decoding base64 ciphertext: %w", err)
	}
	plaintext, err := decrypt(ciphertext, s.cfg.SecretKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypting token data: %w", err)
	}
	return tokenFromJSON(plaintext)
}

// connectOIDCProvider attempts to connect to the OIDC provider, retrying for up to 3 minutes.
func connectOIDCProvider(ctx context.Context, issuerURL string) (*oidc.Provider, error) {
	const maxWait = 3 * time.Minute
	const retryInterval = 2 * time.Second

	deadline := time.Now().Add(maxWait)
	var lastErr error

	for {
		provider, err := oidc.NewProvider(ctx, issuerURL)
		if err == nil {
			return provider, nil
		}
		lastErr = err

		if time.Now().After(deadline) {
			break
		}
		if ctx.Err() != nil {
			break
		}

		slog.Debug("OIDC provider not ready, retrying", "interval", retryInterval, "error", err)
		select {
		case <-time.After(retryInterval):
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled waiting for OIDC provider: %w", ctx.Err())
		}
	}

	return nil, fmt.Errorf("failed to connect to OIDC provider after %s: %w", maxWait, lastErr)
}

func (s *AuthService) GenerateState() string {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("crypto/rand is unavailable: " + err.Error())
	}
	return base64.URLEncoding.EncodeToString(b)
}

func (s *AuthService) GetSession(ctx context.Context, sessionID string) (*models.SessionData, bool) {
	session, err := s.sessionManager.GetSession(ctx, sessionID)
	if err != nil {
		return nil, false
	}

	token, roles, err := s.decryptToken(session.TokenData)
	if err != nil {
		return nil, false
	}

	session.Token = token
	session.Roles = roles

	identity, err := s.identityManager.GetIdentityByID(ctx, session.IdentityID)
	if err != nil {
		return nil, false
	}

	session.SubjectID = identity.AuthSub
	session.UserID = identity.UserID

	return session, true
}

func (s *AuthService) SetSession(ctx context.Context, sessionID string, session models.SessionData) error {
	tokenEncrypted, err := s.encryptToken(session.Token, session.Roles)
	if err != nil {
		return fmt.Errorf("encrypting token data: %w", err)
	}
	session.TokenData = tokenEncrypted

	return s.sessionManager.CreateSession(ctx, sessionID, session)
}

func (s *AuthService) RefreshToken(ctx context.Context, sessionID string, session *models.SessionData) error {
	if session.Token.Expiry.After(time.Now()) {
		return nil
	}

	tokenSource := s.oauth2Config.TokenSource(ctx, session.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return err
	}
	session.Token = newToken

	tokenEncrypted, err := s.encryptToken(newToken, session.Roles)
	if err != nil {
		return fmt.Errorf("encrypting refreshed token: %w", err)
	}
	session.TokenData = tokenEncrypted

	return s.sessionManager.UpdateSessionToken(ctx, sessionID, *session)
}

// AuthCodeURL generates the OAuth2 authorization code URL with the given state
func (s *AuthService) AuthCodeURL(state string) string {
	return s.oauth2Config.AuthCodeURL(state)
}

// registrationCodeURL generates a Keycloak registration URL by replacing the OIDC
// auth endpoint (/auth) with the registration endpoint (/registrations).
// All standard OAuth2 parameters (state, client_id, redirect_uri, scope) are preserved,
// so the callback flow is identical to a normal login.
func (s *AuthService) registrationCodeURL(state string) string {
	authURL := s.oauth2Config.AuthCodeURL(state)
	return strings.Replace(authURL, "/protocol/openid-connect/auth", "/protocol/openid-connect/registrations", 1)
}

// ExchangeToken exchanges an authorization code for an OAuth2 token and extracts the raw ID token
func (s *AuthService) ExchangeToken(ctx context.Context, code string) (*oauth2.Token, string, error) {
	token, err := s.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, "", err
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, "", fmt.Errorf("no id_token in response")
	}

	return token, rawIDToken, nil
}

// VerifyToken verifies an ID token and extracts standard and raw claims
func (s *AuthService) VerifyToken(ctx context.Context, rawIDToken string, claimsStruct interface{}) (interface{}, json.RawMessage, error) {
	idToken, err := s.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return claimsStruct, nil, fmt.Errorf("failed to verify token: %w", err)
	}

	if err := idToken.Claims(&claimsStruct); err != nil {
		return claimsStruct, nil, fmt.Errorf("failed to parse standard claims: %w", err)
	}

	var rawClaims json.RawMessage
	if err := idToken.Claims(&rawClaims); err != nil {
		return claimsStruct, nil, fmt.Errorf("failed to parse raw claims: %w", err)
	}

	return claimsStruct, rawClaims, nil
}

// ExtractRoles extracts roles from OIDC claims
func (s *AuthService) ExtractRoles(rawClaims json.RawMessage) ([]string, error) {
	return s.roleExtractor(rawClaims, s.cfg.OIDCClientID)
}

// RevokeSession revokes an OIDC session via backchannel logout.
// The HTTP request is sent in a goroutine so logout does not block.
func (s *AuthService) RevokeSession(session models.SessionData) {
	if s.cfg.OIDCLogoutURL == "" || session.Token == nil || session.Token.RefreshToken == "" {
		return
	}

	logoutURL := s.cfg.OIDCLogoutURL
	clientID := s.cfg.OIDCClientID
	clientSecret := s.cfg.OIDCClientSecret
	refreshToken := session.Token.RefreshToken

	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.PostForm(logoutURL, url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"refresh_token": {refreshToken},
		})
		if err != nil {
			slog.Warn("OIDC backchannel logout request failed", "error", err)
			return
		}
		resp.Body.Close()
	}()
}

func encrypt(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// --- middleware ---

// RequireAuth is middleware that requires both a valid session and a resolved domain user.
// Redirects to loginFailureRedirect if either is absent.
func (s *AuthService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if framework.GetAuthSession(r) == nil {
			http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
			return
		}
		if framework.GetLoggedInUser(r) == nil {
			http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CheckAuth validates the session cookie and stores the auth session in the request context.
// It does not load the domain user — that is handled by the ResolveUser middleware.
func (s *AuthService) CheckAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		session, exists := s.GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			s.DeleteSession(w, r)
			next.ServeHTTP(w, r)
			return
		}

		if err := s.RefreshToken(r.Context(), cookie.Value, session); err != nil {
			slog.Warn("token refresh failed, deleting session", "session_id", cookie.Value, "error", err)
			s.DeleteSession(w, r)
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, framework.SetAuthSession(r, session))
	})
}

// --- route handlers ---

// Login handles GET /login — redirects to OIDC provider
func (s *AuthService) Login(w http.ResponseWriter, r *http.Request) {
	state := s.GenerateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev,
		SameSite: http.SameSiteLaxMode,
	})

	authURL := s.AuthCodeURL(state)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// Register handles GET /register — redirects to Keycloak's registration page.
// After the user registers, Keycloak redirects back to /callback as normal.
func (s *AuthService) Register(w http.ResponseWriter, r *http.Request) {
	state := s.GenerateState()

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, s.registrationCodeURL(state), http.StatusTemporaryRedirect)
}

// Callback handles GET /callback — OIDC callback.
// On first login (identity.UserID == nil), calls onFirstLogin to provision a domain user,
// then links the identity to that user.
func (s *AuthService) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		s.DeleteSession(w, r)
		http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev,
		SameSite: http.SameSiteLaxMode,
	})

	code := r.URL.Query().Get("code")
	token, rawIDToken, err := s.ExchangeToken(ctx, code)
	if err != nil {
		slog.Error("token exchange failed", "error", err)
		s.DeleteSession(w, r)
		http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
		return
	}

	claims := &oidcClaims{}
	stdClaims, rawClaims, err := s.VerifyToken(ctx, rawIDToken, claims)
	if err != nil {
		slog.Error("token verification failed", "error", err)
		s.DeleteSession(w, r)
		http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
		return
	}

	claims = stdClaims.(*oidcClaims)

	roles, err := s.ExtractRoles(rawClaims)
	if err != nil {
		slog.Warn("role extraction failed, proceeding without roles", "error", err)
		roles = nil
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.Email
	}

	identity, err := s.identityManager.UpsertIdentity(ctx, claims.Sub)
	if err != nil {
		slog.Error("identity upsert failed", "error", err)
		s.DeleteSession(w, r)
		http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
		return
	}

	if identity.UserID == nil && s.onFirstLogin != nil {
		userID, err := s.onFirstLogin(ctx, claims.Email, displayName)
		if err != nil {
			slog.Error("first login provisioning failed", "error", err)
			s.DeleteSession(w, r)
			http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
			return
		}
		if err := s.identityManager.LinkUser(ctx, identity.ID, userID); err != nil {
			slog.Error("identity link failed", "identity_id", identity.ID, "user_id", userID, "error", err)
			s.DeleteSession(w, r)
			http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
			return
		}
		slog.Info("first login: provisioned user and linked identity", "identity_id", identity.ID, "user_id", userID)
	} else if identity.UserID != nil && s.onLogin != nil {
		if err := s.onLogin(ctx, *identity.UserID, claims.Email, displayName); err != nil {
			slog.Warn("onLogin hook failed", "identity_id", identity.ID, "user_id", identity.UserID, "error", err)
		}
		slog.Info("login", "identity_id", identity.ID, "user_id", identity.UserID)
	}

	sessionID := s.GenerateState()
	if err := s.SetSession(ctx, sessionID, models.SessionData{
		Token:      token,
		IDToken:    rawIDToken,
		SubjectID:  claims.Sub,
		IdentityID: identity.ID,
		Roles:      roles,
		Expires:    time.Now().Add(24 * time.Hour),
	}); err != nil {
		slog.Error("session creation failed", "error", err)
		s.DeleteSession(w, r)
		http.Redirect(w, r, framework.UrlFor(r, s.loginFailureRedirect), http.StatusTemporaryRedirect)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, framework.UrlFor(r, s.loginRedirect), http.StatusTemporaryRedirect)
}

// Logout handles POST /logout
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	s.DeleteSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *AuthService) DeleteSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		if session, exists := s.GetSession(r.Context(), cookie.Value); exists {
			s.RevokeSession(*session)
		}
		_ = s.sessionManager.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !s.cfg.IsDev,
		SameSite: http.SameSiteLaxMode,
	})
}
