// Command registry runs the Spotter model registry.
//
// Configuration comes only from environment variables: DATABASE_URL (required),
// HTTP_ADDR (default :8080), SHUTDOWN_TIMEOUT (default 10s).
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pohi11/spotter/services/registry/internal/adapters/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// SIGINT (Ctrl+C) and SIGTERM (Kubernetes stopping the pod) both cancel ctx.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Getenv, logger, nil); err != nil {
		logger.Error("registry stopped", "error", err)
		os.Exit(1)
	}
}

// run starts the registry and blocks until ctx is cancelled, then shuts down gracefully.
// onListening, if set, is called with the bound HTTP address once the server accepts
// connections (tests use it with HTTP_ADDR=127.0.0.1:0).
func run(ctx context.Context, getenv func(string) string, logger *slog.Logger, onListening func(addr string)) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("DATABASE_URL: %w", err)
	}
	defer pool.Close()

	applied, err := postgres.Migrate(ctx, pool)
	if err != nil {
		return err
	}
	logger.Info("schema up to date", "migrations_applied", applied)

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("HTTP_ADDR: %w", err)
	}
	srv := &http.Server{Handler: healthHandler(pool)}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("registry listening", "addr", ln.Addr().String())
	if onListening != nil {
		onListening(ln.Addr().String())
	}

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout.String())
	// A fresh context: ctx is already cancelled, but in-flight requests get the timeout.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}
