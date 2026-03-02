package db

import (
	"errors"

	"github.com/antonkarounis/stoic/internal/domain/ports"
	"github.com/jackc/pgx/v5"
)

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ErrNotFound
	}
	return err
}
