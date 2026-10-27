// Package dbtest runs repository tests against a real Postgres in a container,
// with the project migrations applied.
package dbtest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/db"
	"github.com/Adriano-silva131/planejo-backend/migrations"
)

var conn *gorm.DB

// Main starts the database and runs the package tests. Call it from TestMain.
func Main(m *testing.M) {
	code, err := run(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dbtest:", err)
		os.Exit(1)
	}
	os.Exit(code)
}

func run(m *testing.M) (int, error) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if err != nil {
		return 0, err
	}
	url, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, err
	}

	log := slog.New(slog.DiscardHandler)
	conn, err = db.Open(ctx, url, log)
	if err != nil {
		return 0, err
	}
	sqlDB, err := conn.DB()
	if err != nil {
		return 0, err
	}
	if err := db.Migrate(ctx, sqlDB, migrations.FS, log); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// Tx returns a transaction that is rolled back when the test ends, so tests
// never see each other's rows.
func Tx(t *testing.T) *gorm.DB {
	t.Helper()
	tx := conn.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })
	return tx
}
