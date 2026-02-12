package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"

	"github.com/antonkarounis/stoic/internal/platform/config"
	"github.com/antonkarounis/stoic/internal/platform/db/gen"
)

// AuthService encapsulates all authentication state and operations.
type AuthService struct {
	provider      *oidc.Provider
	oauth2Config  oauth2.Config
	verifier      *oidc.IDTokenVerifier
	queries       *gen.Queries
	cfg           *config.Config
	roleExtractor RoleExtractor
}

// RoleExtractor extracts roles from raw OIDC claims.
// The default implementation handles Keycloak realm_access and resource_access claims.
// Replace with a custom function for other OIDC providers (Auth0, Okta, etc.).
type RoleExtractor func(rawClaims json.RawMessage, clientID string) ([]string, error)

type SessionData struct {
	Token       *oauth2.Token
	IDToken     string
	UserID      string // auth provider subject ID
	UserDBID    int64  // users.id in the database
	Email       string
	DisplayName string
	Roles       []string
	Expires     time.Time
}

// StandardClaims are the provider-independent OIDC claims (sub, email, name).
type StandardClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// tokenData is the JSON-serializable representation stored in sessions.token_data
type tokenData struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Roles        []string  `json:"roles"`
}

func NewAuthService(ctx context.Context, cfg *config.Config, queries *gen.Queries) (*AuthService, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC provider: %w", err)
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
		provider:      provider,
		oauth2Config:  oauth2Config,
		verifier:      verifier,
		queries:       queries,
		cfg:           cfg,
		roleExtractor: KeycloakRoleExtractor,
	}, nil
}

// KeycloakRoleExtractor extracts roles from Keycloak-specific claims (realm_access, resource_access).
// For other OIDC providers, replace AuthService.roleExtractor with a custom function.
func KeycloakRoleExtractor(rawClaims json.RawMessage, clientID string) ([]string, error) {
	var claims struct {
		RealmAccess    struct{ Roles []string `json:"roles"` } `json:"realm_access"`
		ResourceAccess map[string]struct{ Roles []string `json:"roles"` } `json:"resource_access"`
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
func (s *AuthService) encryptToken(token *oauth2.Token, roles []string) ([]byte, error) {
	plaintext, err := tokenToJSON(token, roles)
	if err != nil {
		return nil, fmt.Errorf("marshaling token data: %w", err)
	}
	return encrypt(plaintext, s.cfg.SecretKey)
}

// decryptToken decrypts and deserializes token data from storage.
func (s *AuthService) decryptToken(data []byte) (*oauth2.Token, []string, error) {
	plaintext, err := decrypt(data, s.cfg.SecretKey)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypting token data: %w", err)
	}
	return tokenFromJSON(plaintext)
}

func GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *AuthService) GetSession(ctx context.Context, sessionID string) (*SessionData, bool) {
	dbSession, err := s.queries.GetSession(ctx, sessionID)
	if err != nil {
		return nil, false
	}

	token, roles, err := s.decryptToken(dbSession.TokenData)
	if err != nil {
		return nil, false
	}

	user, err := s.queries.GetUserByID(ctx, dbSession.UserID)
	if err != nil {
		return nil, false
	}

	return &SessionData{
		Token:       token,
		IDToken:     dbSession.IDToken,
		UserID:      user.AuthSub,
		UserDBID:    user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		Roles:       roles,
		Expires:     dbSession.ExpiresAt.Time,
	}, true
}

func (s *AuthService) SetSession(ctx context.Context, sessionID string, session *SessionData) error {
	tokenEncrypted, err := s.encryptToken(session.Token, session.Roles)
	if err != nil {
		return fmt.Errorf("encrypting token data: %w", err)
	}

	return s.queries.CreateSession(ctx, gen.CreateSessionParams{
		SessionID: sessionID,
		UserID:    session.UserDBID,
		TokenData: tokenEncrypted,
		IDToken:   session.IDToken,
		ExpiresAt: pgtype.Timestamptz{Time: session.Expires, Valid: true},
	})
}

func (s *AuthService) DeleteSession(ctx context.Context, sessionID string) {
	_ = s.queries.DeleteSession(ctx, sessionID)
}

func (s *AuthService) RefreshToken(ctx context.Context, sessionID string, session *SessionData) error {
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
	return s.queries.UpdateSessionToken(ctx, gen.UpdateSessionTokenParams{
		SessionID: sessionID,
		TokenData: tokenEncrypted,
	})
}

// UpsertUser creates or updates a user record and returns the database ID.
func (s *AuthService) UpsertUser(ctx context.Context, authSub, email, displayName string) (int64, error) {
	user, err := s.queries.UpsertUser(ctx, gen.UpsertUserParams{
		AuthSub:     authSub,
		Email:       email,
		DisplayName: displayName,
	})
	if err != nil {
		return 0, fmt.Errorf("upserting user: %w", err)
	}
	return user.ID, nil
}
