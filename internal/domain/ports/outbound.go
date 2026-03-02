package ports

import (
	"context"

	"github.com/antonkarounis/stoic/internal/domain/models"
)

type SessionRepository interface {
	CreateSession(ctx context.Context, sessionID string, session models.SessionData) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (*models.SessionData, error)
	UpdateSessionToken(ctx context.Context, sessionID string, session models.SessionData) error
}

type IdentityRepository interface {
	GetIdentityByID(ctx context.Context, identityID int64) (models.Identity, error)
	UpsertIdentity(ctx context.Context, authSub string) (models.Identity, error)
	LinkUser(ctx context.Context, identityID int64, userID models.UserID) error
}

type UserRepository interface {
	Save(ctx context.Context, user models.User) error
	FindByID(ctx context.Context, id models.UserID) (models.User, error)
	FindByEmail(ctx context.Context, email string) (models.User, error)
}
