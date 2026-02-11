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

/*
	Keycloak setup example:

    	Click "Clients" -> click on "Create client"
		Client authentication - on
		Standard flow - on
		All others - off

	Click "Client scopes" -> click on "roles"
		Include in token scope - on

	For other OIDC providers (Auth0, Okta, etc.), configure a
	standard Authorization Code Flow with the scopes: openid, profile, email.
*/

var (
	provider     *oidc.Provider
	OAuth2Config oauth2.Config
	Verifier     *oidc.IDTokenVerifier
	queries      *gen.Queries
	cfg          *config.Config
)

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

// OIDCClaims represents the JWT claims from an OIDC provider.
// The JSON structure is Keycloak-compatible (realm_access, resource_access).
// Role extraction assumes Keycloak claim structure. Modify GetAllRoles() for other providers.
type OIDCClaims struct {
	Sub            string         `json:"sub"`
	Email          string         `json:"email"`
	Name           string         `json:"name"`
	RealmAccess    RealmAccess    `json:"realm_access"`
	ResourceAccess ResourceAccess `json:"resource_access"`
}

type RealmAccess struct {
	Roles []string `json:"roles"`
}

type ResourceAccess map[string]struct {
	Roles []string `json:"roles"`
}

// tokenData is the JSON-serializable representation stored in sessions.token_data
type tokenData struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	Roles        []string  `json:"roles"`
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

// GetAllRoles returns combined realm and client roles, filtering out default Keycloak roles.
// For non-Keycloak providers, modify this method to extract roles from your provider's claim structure.
func (c *OIDCClaims) GetAllRoles(clientID string) []string {
	allRoles := make([]string, 0)
	allRoles = append(allRoles, c.RealmAccess.Roles...)
	if clientRoles, ok := c.ResourceAccess[clientID]; ok {
		allRoles = append(allRoles, clientRoles.Roles...)
	}

	// Filter out default Keycloak roles
	roles := make([]string, 0)
	for _, role := range allRoles {
		if isDefaultRole(role) {
			continue
		}
		roles = append(roles, role)
	}
	return roles
}

func isDefaultRole(role string) bool {
	if strings.HasPrefix(role, "default-roles-") {
		return true
	}
	switch role {
	case "offline_access", "uma_authorization":
		return true
	}
	return false
}

func Init(ctx context.Context, c *config.Config) error {
	cfg = c

	var err error
	provider, err = oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
	if err != nil {
		return fmt.Errorf("failed to create OIDC provider: %w", err)
	}

	OAuth2Config = oauth2.Config{
		ClientID:     cfg.OIDCClientID,
		ClientSecret: cfg.OIDCClientSecret,
		RedirectURL:  cfg.AppURL + "/callback",
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	Verifier = provider.Verifier(&oidc.Config{
		ClientID: cfg.OIDCClientID,
	})

	return nil
}

// InitDB sets the database queries instance for session and user operations.
func InitDB(q *gen.Queries) {
	queries = q
}

func GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func GetSession(ctx context.Context, sessionID string) (*SessionData, bool) {
	dbSession, err := queries.GetSession(ctx, sessionID)
	if err != nil {
		return nil, false
	}

	token, roles, err := tokenFromJSON(dbSession.TokenData)
	if err != nil {
		return nil, false
	}

	user, err := queries.GetUserByID(ctx, dbSession.UserID)
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

func SetSession(ctx context.Context, sessionID string, session *SessionData) error {
	tokenJSON, err := tokenToJSON(session.Token, session.Roles)
	if err != nil {
		return fmt.Errorf("marshaling token data: %w", err)
	}

	return queries.CreateSession(ctx, gen.CreateSessionParams{
		SessionID: sessionID,
		UserID:    session.UserDBID,
		TokenData: tokenJSON,
		IDToken:   session.IDToken,
		ExpiresAt: pgtype.Timestamptz{Time: session.Expires, Valid: true},
	})
}

func DeleteSession(ctx context.Context, sessionID string) {
	_ = queries.DeleteSession(ctx, sessionID)
}

func RefreshToken(ctx context.Context, sessionID string, session *SessionData) error {
	if session.Token.Expiry.After(time.Now()) {
		return nil
	}

	tokenSource := OAuth2Config.TokenSource(ctx, session.Token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return err
	}
	session.Token = newToken

	tokenJSON, err := tokenToJSON(newToken, session.Roles)
	if err != nil {
		return fmt.Errorf("marshaling refreshed token: %w", err)
	}
	return queries.UpdateSessionToken(ctx, gen.UpdateSessionTokenParams{
		SessionID: sessionID,
		TokenData: tokenJSON,
	})
}

// UpsertUser creates or updates a user record and returns the database ID.
func UpsertUser(ctx context.Context, authSub, email, displayName string) (int64, error) {
	user, err := queries.UpsertUser(ctx, gen.UpsertUserParams{
		AuthSub:     authSub,
		Email:       email,
		DisplayName: displayName,
	})
	if err != nil {
		return 0, fmt.Errorf("upserting user: %w", err)
	}
	return user.ID, nil
}
