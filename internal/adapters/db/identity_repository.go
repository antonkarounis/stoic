package db

import (
	"context"
	"fmt"

	"github.com/antonkarounis/stoic/internal/adapters/db/gen"
	"github.com/antonkarounis/stoic/internal/domain/models"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/jackc/pgx/v5/pgtype"
)

type IdentityRepository struct {
	queries *gen.Queries
}

var _ ports.IdentityRepository = (*IdentityRepository)(nil)

func NewIdentityRepository(q *gen.Queries) *IdentityRepository {
	return &IdentityRepository{queries: q}
}

func identityUserID(uid pgtype.Text) *models.UserID {
	if !uid.Valid {
		return nil
	}
	id := models.UserID(uid.String)
	return &id
}

func (r *IdentityRepository) GetIdentityByID(ctx context.Context, identityID int64) (models.Identity, error) {
	row, err := r.queries.GetIdentityByID(ctx, identityID)
	if err != nil {
		return models.Identity{}, mapErr(err)
	}
	return models.Identity{
		ID:      row.ID,
		AuthSub: row.AuthSub,
		UserID:  identityUserID(row.UserID),
	}, nil
}

func (r *IdentityRepository) UpsertIdentity(ctx context.Context, authSub string) (models.Identity, error) {
	row, err := r.queries.UpsertIdentity(ctx, authSub)
	if err != nil {
		return models.Identity{}, fmt.Errorf("upserting identity: %w", err)
	}
	return models.Identity{
		ID:      row.ID,
		AuthSub: row.AuthSub,
		UserID:  identityUserID(row.UserID),
	}, nil
}

func (r *IdentityRepository) LinkUser(ctx context.Context, identityID int64, userID models.UserID) error {
	return r.queries.LinkIdentityToUser(ctx, gen.LinkIdentityToUserParams{
		ID:     identityID,
		UserID: pgtype.Text{String: string(userID), Valid: true},
	})
}
