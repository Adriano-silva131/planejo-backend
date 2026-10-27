package db

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"

	"github.com/Adriano-silva131/planejo-backend/migrations"
)

var (
	testDB  *gorm.DB
	testURL string
	discard = slog.New(slog.DiscardHandler)
)

func TestMain(m *testing.M) {
	code, err := runWithPostgres(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

func runWithPostgres(m *testing.M) (int, error) {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	defer func() { _ = testcontainers.TerminateContainer(ctr) }()
	if err != nil {
		return 0, err
	}

	testURL, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return 0, err
	}
	testDB, err = Open(ctx, testURL, discard)
	if err != nil {
		return 0, err
	}
	if err := testDB.Exec("CREATE TABLE items (name text PRIMARY KEY)").Error; err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// freshDatabase gives the test its own database, so goose version history
// does not leak between tests.
func freshDatabase(t *testing.T) *sql.DB {
	t.Helper()
	name := strings.ToLower(t.Name())
	require.NoError(t, testDB.Exec("CREATE DATABASE "+name).Error)

	u, err := url.Parse(testURL)
	require.NoError(t, err)
	u.Path = "/" + name
	gdb, err := Open(t.Context(), u.String(), discard)
	require.NoError(t, err)

	conn, err := gdb.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func itemExists(t *testing.T, name string) bool {
	t.Helper()
	var count int64
	require.NoError(t, testDB.Table("items").Where("name = ?", name).Count(&count).Error)
	return count > 0
}

func insertItem(ctx context.Context, name string) error {
	return Conn(ctx, testDB).Exec("INSERT INTO items (name) VALUES (?)", name).Error
}

func TestOpenFailsWhenDatabaseIsUnreachable(t *testing.T) {
	_, err := Open(t.Context(), "postgres://127.0.0.1:1/db?connect_timeout=1", discard)

	assert.Error(t, err)
}

func TestMigrateAppliesPendingMigrationsOnce(t *testing.T) {
	fsys := fstest.MapFS{
		"00001_create_notes.sql": {Data: []byte(
			"-- +goose Up\nCREATE TABLE notes (id int PRIMARY KEY);\n-- +goose Down\nDROP TABLE notes;\n",
		)},
	}

	conn := freshDatabase(t)

	require.NoError(t, Migrate(t.Context(), conn, fsys, discard))
	require.NoError(t, Migrate(t.Context(), conn, fsys, discard))

	var exists bool
	require.NoError(t, conn.QueryRowContext(t.Context(), "SELECT to_regclass('notes') IS NOT NULL").Scan(&exists))
	assert.True(t, exists)
}

func TestMigrateAcceptsEmbeddedMigrations(t *testing.T) {
	assert.NoError(t, Migrate(t.Context(), freshDatabase(t), migrations.FS, discard))
}

func TestMigrateWithoutMigrationsIsNoop(t *testing.T) {
	assert.NoError(t, Migrate(t.Context(), freshDatabase(t), fstest.MapFS{}, discard))
}

func TestInTxCommits(t *testing.T) {
	err := NewTransactor(testDB).InTx(t.Context(), func(ctx context.Context) error {
		return insertItem(ctx, "committed")
	})

	require.NoError(t, err)
	assert.True(t, itemExists(t, "committed"))
}

func TestInTxRollsBackOnError(t *testing.T) {
	errBoom := errors.New("boom")

	err := NewTransactor(testDB).InTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, insertItem(ctx, "rolled-back"))
		return errBoom
	})

	require.ErrorIs(t, err, errBoom)
	assert.False(t, itemExists(t, "rolled-back"))
}

func TestInTxNestedFailureRollsBackOnlyInnerWork(t *testing.T) {
	tx := NewTransactor(testDB)

	err := tx.InTx(t.Context(), func(ctx context.Context) error {
		require.NoError(t, insertItem(ctx, "outer"))
		innerErr := tx.InTx(ctx, func(ctx context.Context) error {
			require.NoError(t, insertItem(ctx, "inner"))
			return errors.New("inner failed")
		})
		assert.Error(t, innerErr)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, itemExists(t, "outer"))
	assert.False(t, itemExists(t, "inner"))
}

func TestConnInTxHonorsTheGivenContext(t *testing.T) {
	_ = NewTransactor(testDB).InTx(t.Context(), func(ctx context.Context) error {
		short, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		err := Conn(short, testDB).Exec("SELECT pg_sleep(5)").Error

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Less(t, time.Since(start), 2*time.Second)
		return nil
	})
}

func TestOpenLimitsThePool(t *testing.T) {
	conn, err := testDB.DB()
	require.NoError(t, err)

	assert.Equal(t, maxOpenConns, conn.Stats().MaxOpenConnections)
}

func TestMigrateLogsMigrationsAppliedBeforeAFailure(t *testing.T) {
	fsys := fstest.MapFS{
		"00001_create_a.sql": {Data: []byte("-- +goose Up\nCREATE TABLE a (id int);\n")},
		"00002_broken.sql":   {Data: []byte("-- +goose Up\nCREATE TABLE;\n")},
	}
	var logs bytes.Buffer

	err := Migrate(t.Context(), freshDatabase(t), fsys, slog.New(slog.NewTextHandler(&logs, nil)))

	require.Error(t, err)
	assert.Contains(t, logs.String(), "00001_create_a.sql")
	assert.NotContains(t, logs.String(), "00002_broken.sql")
}

type ownedRow struct {
	ID     string `gorm:"primaryKey"`
	UserID uuid.UUID
	Value  int
}

func (ownedRow) TableName() string {
	return "owned_rows"
}

func TestOwnedUpsert(t *testing.T) {
	require.NoError(t, testDB.Exec("CREATE TABLE owned_rows (id text PRIMARY KEY, user_id uuid NOT NULL, value int)").Error)
	t.Cleanup(func() { testDB.Exec("DROP TABLE owned_rows") })
	upsert := OwnedUpsert{Table: "owned_rows", Conflict: []string{"id"}, Update: []string{"value"}}
	owner, other := uuid.New(), uuid.New()

	require.NoError(t, upsert.Save(t.Context(), testDB, &ownedRow{ID: "a", UserID: owner, Value: 1}, owner))
	updated := ownedRow{ID: "a", UserID: owner, Value: 2}
	require.NoError(t, upsert.Save(t.Context(), testDB, &updated, owner))
	err := upsert.Save(t.Context(), testDB, &ownedRow{ID: "a", UserID: other, Value: 99}, other)

	assert.ErrorIs(t, err, ErrNotOwned)
	var got ownedRow
	require.NoError(t, testDB.Take(&got, "id = ?", "a").Error)
	assert.Equal(t, 2, got.Value, "the owner's row is kept")
	assert.Equal(t, 2, updated.Value, "read back with RETURNING")
}
