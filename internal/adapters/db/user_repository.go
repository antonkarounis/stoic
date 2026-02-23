package db

import (
	"context"
	"fmt"

	"github.com/antonkarounis/balance/internal/adapters/db/gen"
	"github.com/antonkarounis/balance/internal/ports"
)

type UserRepository struct {
	queries *gen.Queries
}

func NewUserRepository(q *gen.Queries) *UserRepository {
	return &UserRepository{queries: q}
}

// GetUserByID implements [auth.UserRepository].
func (s *UserRepository) GetUserByID(ctx context.Context, userId int64) (ports.User, error) {
	user, err := s.queries.GetUserByID(ctx, userId)
	if err != nil {
		return ports.User{}, err
	}
	return ports.User{
		AuthSub:     user.AuthSub,
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}, nil
}

// UpsertUser creates or updates a user record and returns the database ID.
func (s *UserRepository) UpsertUser(ctx context.Context, authSub, email, displayName string) (int64, error) {
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
