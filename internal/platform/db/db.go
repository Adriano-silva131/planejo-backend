package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func Open(ctx context.Context, url string, log *slog.Logger) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(url), &gorm.Config{
		Logger: gormlogger.NewSlogLogger(log, gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      true,
		}),
		TranslateError:       true,
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, fmt.Errorf("db: open: %w", err)
	}
	configurePool(sqlDB)
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return gdb, nil
}

// Each instance keeps at most maxOpenConns connections, so traffic spikes do
// not exhaust Postgres' max_connections. Recycling connections picks up
// failovers and DNS changes.
const (
	maxOpenConns    = 20
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

func configurePool(sqlDB *sql.DB) {
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxOpenConns)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
}
