package db

import (
	"context"

	"github.com/antonkarounis/stoic/internal/adapters/db/gen"
	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserRepository struct {
	queries *gen.Queries
}

var _ ports.UserRepository = (*UserRepository)(nil)

func NewUserRepository(q *gen.Queries) *UserRepository {
	return &UserRepository{queries: q}
}

// Save implements [ports.UserRepository].
func (r *UserRepository) Save(ctx context.Context, user models.User) error {
	return r.queries.UpsertUser(ctx, gen.UpsertUserParams{
		ID:          string(user.ID),
		Name:        user.Name,
		Email:       user.Email,
		Role:        string(user.Role),
		CreatedAt:   pgtype.Timestamptz{Time: user.CreatedAt, Valid: true},
	})
}

// FindByID implements [ports.UserRepository].
func (r *UserRepository) FindByID(ctx context.Context, id models.UserID) (models.User, error) {
	row, err := r.queries.GetUserByID(ctx, string(id))
	if err != nil {
		return models.User{}, mapErr(err)
	}
	return models.User{
		ID:        models.UserID(row.ID),
		Name:      row.Name,
		Email:     row.Email,
		Role:      models.Role(row.Role),
		CreatedAt: row.CreatedAt.Time,
	}, nil
}

// FindByEmail implements [ports.UserRepository].
func (r *UserRepository) FindByEmail(ctx context.Context, email string) (models.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return models.User{}, mapErr(err)
	}
	return models.User{
		ID:        models.UserID(row.ID),
		Name:      row.Name,
		Email:     row.Email,
		Role:      models.Role(row.Role),
		CreatedAt: row.CreatedAt.Time,
	}, nil
}
