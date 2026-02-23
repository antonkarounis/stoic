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
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/antonkarounis/balance/internal/adapters/web/framework"
	"github.com/antonkarounis/balance/internal/ports"
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

// AuthService encapsulates all authentication state and operations.
type AuthService struct {
	provider       *oidc.Provider
	oauth2Config   oauth2.Config
	verifier       *oidc.IDTokenVerifier
	sessionManager ports.SessionRepository
	userManager    ports.UserRepository
	cfg            *AuthConfig
	roleExtractor  RoleExtractor
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

func NewAuthService(ctx context.Context, cfg *AuthConfig, sessionManager ports.SessionRepository, userManager ports.UserRepository) (*AuthService, error) {
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
		provider:       provider,
		oauth2Config:   oauth2Config,
		verifier:       verifier,
		sessionManager: sessionManager,
		userManager:    userManager,
		cfg:            cfg,
		roleExtractor:  KeycloakRoleExtractor,
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

		log.Printf("OIDC provider not ready, retrying in %s...", retryInterval)
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
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *AuthService) GetSession(ctx context.Context, sessionID string) (*ports.SessionData, bool) {
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

	user, err := s.userManager.GetUserByID(ctx, session.UserDBID)
	if err != nil {
		return nil, false
	}

	session.UserID = user.AuthSub
	session.Email = user.Email
	session.DisplayName = user.DisplayName

	return session, true
}

func (s *AuthService) SetSession(ctx context.Context, sessionID string, session ports.SessionData) error {
	tokenEncrypted, err := s.encryptToken(session.Token, session.Roles)
	if err != nil {
		return fmt.Errorf("encrypting token data: %w", err)
	}
	session.TokenData = tokenEncrypted

	return s.sessionManager.CreateSession(ctx, sessionID, session)
}

func (s *AuthService) RefreshToken(ctx context.Context, sessionID string, session *ports.SessionData) error {
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

// RevokeSession revokes an OIDC session via backchannel logout
func (s *AuthService) RevokeSession(session ports.SessionData) {
	if s.cfg.OIDCLogoutURL == "" || session.Token == nil || session.Token.RefreshToken == "" {
		return
	}

	resp, err := http.PostForm(s.cfg.OIDCLogoutURL, url.Values{
		"client_id":     {s.cfg.OIDCClientID},
		"client_secret": {s.cfg.OIDCClientSecret},
		"refresh_token": {session.Token.RefreshToken},
	})
	if err != nil {
		log.Printf("OIDC backchannel logout request failed: %v", err)
		return
	}
	resp.Body.Close()
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

// RequireAuth is middleware that requires a valid session
func (s *AuthService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if session := framework.GetOptionalSession(r); session != nil {
			cookie, _ := r.Cookie("session_id")
			if cookie != nil {
				if err := s.RefreshToken(r.Context(), cookie.Value, session); err != nil {
					http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("session_id")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		session, exists := s.GetSession(r.Context(), cookie.Value)
		if !exists || time.Now().After(session.Expires) {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		if err := s.RefreshToken(r.Context(), cookie.Value, session); err != nil {
			http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, framework.SetSessionInContext(r, session))
	})
}

// OptionalAuth adds session to context if logged in
func (s *AuthService) OptionalAuth(next http.Handler) http.Handler {
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

		err = s.RefreshToken(r.Context(), cookie.Value, session)
		if err != nil {
			s.DeleteSession(w, r)
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, framework.SetSessionInContext(r, session))
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

// Callback handles GET /callback — OIDC callback
func (s *AuthService) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	stateCookie, err := r.Cookie("oauth_state")
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		s.DeleteSession(w, r)
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	code := r.URL.Query().Get("code")
	token, rawIDToken, err := s.ExchangeToken(ctx, code)
	if err != nil {
		log.Printf("Token exchange error: %v", err)
		s.DeleteSession(w, r)
		http.Error(w, "Failed to exchange token", http.StatusInternalServerError)
		return
	}

	claims := &ports.Claims{}

	stdClaims, rawClaims, err := s.VerifyToken(ctx, rawIDToken, claims)
	if err != nil {
		s.DeleteSession(w, r)
		log.Printf("Token verification error: %v", err)
		http.Error(w, "Failed to verify token", http.StatusUnauthorized)
		return
	}

	claims = stdClaims.(*ports.Claims)

	roles, err := s.ExtractRoles(rawClaims)
	if err != nil {
		log.Printf("Role extraction error: %v", err)
		roles = nil
	}

	displayName := claims.Name
	if displayName == "" {
		displayName = claims.Email
	}

	userDBID, err := s.userManager.UpsertUser(ctx, claims.Sub, claims.Email, displayName)
	if err != nil {
		s.DeleteSession(w, r)
		log.Printf("User upsert error: %v", err)
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	sessionID := s.GenerateState()
	if err := s.SetSession(ctx, sessionID, ports.SessionData{
		Token:       token,
		IDToken:     rawIDToken,
		UserID:      claims.Sub,
		UserDBID:    userDBID,
		Email:       claims.Email,
		DisplayName: displayName,
		Roles:       roles,
		Expires:     time.Now().Add(24 * time.Hour),
	}); err != nil {
		s.DeleteSession(w, r)
		log.Printf("Session creation error: %v", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
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

	http.Redirect(w, r, "/u/dashboard", http.StatusTemporaryRedirect)
}

// Logout handles POST /logout
func (s *AuthService) Logout(w http.ResponseWriter, r *http.Request) {
	s.DeleteSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *AuthService) DeleteSession(w http.ResponseWriter, r *http.Request) {
	fmt.Println("deleting session")

	cookie, err := r.Cookie("session_id")
	if err == nil {
		if session, exists := s.GetSession(r.Context(), cookie.Value); exists {
			s.RevokeSession(*session)
		}
		_ = s.sessionManager.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:   "session_id",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}
