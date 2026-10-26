package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Adriano-silva131/planejo-backend/internal/platform/auth"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/config"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/db"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/httpx"
	"github.com/Adriano-silva131/planejo-backend/internal/platform/logger"
	"github.com/Adriano-silva131/planejo-backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuração inválida", "err", err)
		os.Exit(1)
	}

	log := logger.New(os.Stdout, cfg.Env, cfg.LogLevel)
	slog.SetDefault(log)

	if err := run(cfg, log); err != nil {
		log.Error("API encerrada", "err", err)
		os.Exit(1)
	}
}

// connectTimeout applies only to connecting: migrations can take longer, and
// waiting for another instance's migration lock is bounded by goose.
const connectTimeout = 30 * time.Second

func run(cfg config.Config, log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// After the first signal, a second one kills the process right away.
	context.AfterFunc(ctx, stop)
	connectCtx, cancelConnect := context.WithTimeout(ctx, connectTimeout)
	gdb, err := db.Open(connectCtx, cfg.DatabaseURL, log)
	cancelConnect()
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	if err := db.Migrate(ctx, sqlDB, migrations.FS, log); err != nil {
		return err
	}

	production := cfg.Env == config.Production
	verifier, err := auth.NewSupabaseVerifier(ctx, cfg.SupabaseURL, production)
	if err != nil {
		return err
	}

	router := httpx.New(httpx.Options{
		Production:               production,
		AllowedOrigins:           cfg.CORSAllowedOrigins,
		RequestTimeout:           10 * time.Second,
		Verifier:                 verifier,
		PublicRateLimit:          60,
		AuthenticatedIPRateLimit: 600,
		UserRateLimit:            300,
	}, log)

	srv := newServer(cfg.Port, router.Handler)
	log.Info("API ouvindo", "port", cfg.Port, "env", cfg.Env)
	return serve(ctx, srv, log)
}

// shutdownTimeout is longer than the request timeout, so requests in progress
// can finish.
const shutdownTimeout = 15 * time.Second

func serve(ctx context.Context, srv *http.Server, log *slog.Logger) error {
	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	log.Info("encerrando a API")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("API encerrada")
	return nil
}

func newServer(port int, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}
