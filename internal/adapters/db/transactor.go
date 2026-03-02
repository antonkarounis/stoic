package db

import (
	"context"
	"fmt"

	"github.com/antonkarounis/stoic/internal/adapters/db/gen"
	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgxTransactor struct {
	pool    *pgxpool.Pool
	queries *gen.Queries
}

var _ ports.Transactor = (*pgxTransactor)(nil)

func NewTransactor(pool *pgxpool.Pool, queries *gen.Queries) ports.Transactor {
	return &pgxTransactor{pool: pool, queries: queries}
}

// InTx implements [ports.Transactor].
func (t *pgxTransactor) InTx(ctx context.Context, fn func(ctx context.Context, repos ports.TxRepositories) error) error {
	tx, err := t.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	q := t.queries.WithTx(tx)
	repos := ports.TxRepositories{
		Users:      NewUserRepository(q),
	}

	if err := fn(ctx, repos); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
