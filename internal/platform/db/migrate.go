package db

import (
	"context"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type MigrateLogger struct {
	ctx context.Context
}

func (ml *MigrateLogger) Printf(format string, v ...interface{}) {
	fmt.Fprintln(os.Stdout, format, v)
}

func (ml *MigrateLogger) Verbose() bool {
	return true
}

func Migrate(ctx context.Context, migrationPath string, dbUrl string) error {
	m, err := migrate.New(migrationPath, dbUrl)
	if err != nil {
		return fmt.Errorf("creating migration instance: %w", err)
	}
	defer m.Close()

	m.Log = &MigrateLogger{ctx: ctx}

	fmt.Println("starting migrations")

	err = m.Up()
	if err != nil && err == migrate.ErrNoChange {
		fmt.Println("no migrations required")
		return nil
	} else if err != nil {
		return fmt.Errorf("error running migrations: %w", err)
	}

	fmt.Println("completed migrations")
	return nil
}
