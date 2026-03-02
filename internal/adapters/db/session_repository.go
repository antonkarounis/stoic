package db

import (
	"context"
	"log/slog"
	"time"

	"github.com/antonkarounis/stoic/internal/adapters/db/gen"
	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/jackc/pgx/v5/pgtype"
)

type SessionRepository struct {
	queries *gen.Queries
}

var _ ports.SessionRepository = (*SessionRepository)(nil)

func NewSessionRepository(ctx context.Context, q *gen.Queries) *SessionRepository {
	startCleanupRoutine(ctx, q)
	return &SessionRepository{queries: q}
}

func startCleanupRoutine(ctx context.Context, queries *gen.Queries) {
	// Periodically clean up expired sessions
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := queries.DeleteExpiredSessions(ctx); err != nil {
					slog.Warn("failed to clean up expired sessions", "error", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// CreateSession implements [auth.SessionRepository].
func (s *SessionRepository) CreateSession(ctx context.Context, sessionID string, session models.SessionData) error {
	return s.queries.CreateSession(ctx, gen.CreateSessionParams{
		SessionID:  sessionID,
		IdentityID: session.IdentityID,
		TokenData:  session.TokenData,
		IDToken:    session.IDToken,
		ExpiresAt:  pgtype.Timestamptz{Time: session.Expires, Valid: true},
	})
}

// DeleteSession implements [auth.SessionRepository].
func (s *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return s.queries.DeleteSession(ctx, sessionID)
}

// GetSession implements [auth.SessionRepository].
func (s *SessionRepository) GetSession(ctx context.Context, sessionID string) (*models.SessionData, error) {
	session, err := s.queries.GetSession(ctx, sessionID)
	if err != nil {
		return nil, mapErr(err)
	}

	return &models.SessionData{
		IDToken:    session.IDToken,
		IdentityID: session.IdentityID,
		Expires:    session.ExpiresAt.Time,
		TokenData:  session.TokenData,
	}, nil
}

// UpdateSessionToken implements [auth.SessionRepository].
func (s *SessionRepository) UpdateSessionToken(ctx context.Context, sessionID string, session models.SessionData) error {
	return s.queries.UpdateSessionToken(ctx, gen.UpdateSessionTokenParams{
		SessionID: sessionID,
		TokenData: session.TokenData,
	})
}
