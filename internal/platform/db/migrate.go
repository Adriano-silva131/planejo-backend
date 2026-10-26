package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Migrate applies pending migrations under a Postgres advisory lock, so
// instances starting at the same time do not run them twice.
func Migrate(ctx context.Context, sqlDB *sql.DB, migrations fs.FS, log *slog.Logger) error {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations, goose.WithSessionLocker(locker))
	if errors.Is(err, goose.ErrNoMigrations) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}

	results, err := provider.Up(ctx)
	if partial, ok := errors.AsType[*goose.PartialError](err); ok {
		results = partial.Applied
	}
	for _, r := range results {
		log.Info("migration aplicada", "file", r.Source.Path, "duration", r.Duration)
	}
	if err != nil {
		return fmt.Errorf("db: migrate: %w", err)
	}
	return nil
}
