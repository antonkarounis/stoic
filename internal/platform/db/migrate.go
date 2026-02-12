package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var PlatformMigrations embed.FS

type MigrateLogger struct{}

func (ml *MigrateLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func (ml *MigrateLogger) Verbose() bool {
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
			log.Printf("Failed to release migration lock: %v", err)
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

	m.Log = &MigrateLogger{}

	log.Println("starting migrations")

	err = m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		log.Println("no migrations required")
		return nil
	} else if err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	log.Println("completed migrations")
	return nil
}
