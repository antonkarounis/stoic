package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var PlatformMigrations embed.FS

// slogMigrateLogger adapts slog to the golang-migrate Logger interface.
type slogMigrateLogger struct{}

func (l *slogMigrateLogger) Printf(format string, v ...interface{}) {
	slog.Info(fmt.Sprintf(format, v...))
}

func (l *slogMigrateLogger) Verbose() bool {
	return true
}

// Migrate runs database migrations from the provided fs.FS.
// Uses a PostgreSQL advisory lock to prevent races when multiple instances start concurrently.
func Migrate(ctx context.Context, migrations fs.FS, subdir string, dbUrl string) error {
	// A3: Acquire advisory lock to prevent migration races across instances
	conn, err := pgx.Connect(ctx, dbUrl)
	if err != nil {
		return fmt.Errorf("connecting for migration lock: %w", err)
	}
	defer conn.Close(ctx)

	const migrationLockID = 1
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquiring migration lock: %w", err)
	}
	defer func() {
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockID); err != nil {
			slog.Warn("failed to release migration lock", "error", err)
		}
	}()

	source, err := iofs.New(migrations, subdir)
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, dbUrl)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}
	defer m.Close()

	m.Log = &slogMigrateLogger{}

	slog.Info("starting migrations")

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		slog.Info("no migrations required")
		return nil
	} else if err != nil {
		version, dirty, vErr := m.Version()

		if dirty && vErr == nil {
			slog.Warn("dirty migration state, attempting rollback", "version", version)
			m.Force(int(version))
			m.Down()
		}

		return fmt.Errorf("error running migrations: %w", err)
	}

	slog.Info("completed migrations")
	return nil
}
