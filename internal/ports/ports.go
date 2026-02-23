package ports

import (
	"context"
	"encoding/json"
	"time"

	"golang.org/x/oauth2"
)

type Config struct {
	Environment string // "dev" or "prod"
	AppURL      string // e.g. "http://localhost:8080"
	Addr        string // e.g. ":8080"

	DatabaseURL string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCLogoutURL    string // optional: omit to skip provider-side logout

	SecretKey []byte // 32-byte key for token encryption and CSRF protection
}

type User struct {
	AuthSub     string
	ID          int64
	Email       string
	DisplayName string
}

type SessionData struct {
	Token       *oauth2.Token
	TokenData   []byte // encrypted token bytes from the database
	IDToken     string
	UserID      string // auth provider subject ID
	UserDBID    int64  // users.id in the database
	Email       string
	DisplayName string
	Roles       []string
	Expires     time.Time
}

// Claims are the provider-independent OIDC claims (sub, email, name).
type Claims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type SessionRepository interface {
	CreateSession(ctx context.Context, sessionID string, session SessionData) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*SessionData, error)
	UpdateSessionToken(ctx context.Context, sessionID string, session SessionData) error
}

type UserRepository interface {
	GetUserByID(ctx context.Context, userId int64) (User, error)
	UpsertUser(ctx context.Context, authSub, email, displayName string) (int64, error)
}

type AuthService interface {
	GenerateState() string
	AuthCodeURL(state string) string
	ExchangeToken(ctx context.Context, code string) (*oauth2.Token, string, error)
	VerifyToken(ctx context.Context, rawIdToken string, claimsStruct interface{}) (interface{}, json.RawMessage, error)
	ExtractRoles(rawClaims json.RawMessage) ([]string, error)
	SetSession(ctx context.Context, sessionId string, sessionData SessionData) error
	GetSession(ctx context.Context, cookieValue string) (*SessionData, bool)
	RevokeSession(session SessionData)
	DeleteSession(ctx context.Context, cookieValue string)
}
