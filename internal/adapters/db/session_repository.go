package db

import (
	"context"
	"log"
	"time"

	"github.com/antonkarounis/balance/internal/adapters/db/gen"
	"github.com/antonkarounis/balance/internal/ports"
	"github.com/jackc/pgx/v5/pgtype"
)

type SessionRepository struct {
	queries *gen.Queries
}

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
					log.Printf("Failed to cleanup expired sessions: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// CreateSession implements [auth.SessionRepository].
func (s *SessionRepository) CreateSession(ctx context.Context, sessionID string, session ports.SessionData) error {
	return s.queries.CreateSession(ctx, gen.CreateSessionParams{
		SessionID: sessionID,
		UserID:    session.UserDBID,
		TokenData: session.TokenData,
		IDToken:   session.IDToken,
		ExpiresAt: pgtype.Timestamptz{Time: session.Expires, Valid: true},
	})
}

// DeleteSession implements [auth.SessionRepository].
func (s *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return s.queries.DeleteSession(ctx, sessionID)
}

// GetSession implements [auth.SessionRepository].
func (s *SessionRepository) GetSession(ctx context.Context, sessionID string) (*ports.SessionData, error) {
	session, err := s.queries.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	return &ports.SessionData{
		IDToken:   session.IDToken,
		UserDBID:  session.UserID,
		Expires:   session.ExpiresAt.Time,
		TokenData: session.TokenData,
	}, nil
}

// UpdateSessionToken implements [auth.SessionRepository].
func (s *SessionRepository) UpdateSessionToken(ctx context.Context, sessionID string, session ports.SessionData) error {
	return s.queries.UpdateSessionToken(ctx, gen.UpdateSessionTokenParams{
		SessionID: sessionID,
		TokenData: session.TokenData,
	})
}
