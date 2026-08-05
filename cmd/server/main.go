// Command server is drop-tracker's single process entrypoint. It runs the
// HTTP API today and will run the robfig/cron scheduler in this same
// process starting Phase 3 — PROJECT.md locks a single-binary architecture.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
)

// HTTP server timeouts (WR-02): an http.Server with all zero-value timeouts
// lets a client that opens a connection and sends headers/body slowly (or
// never) hold a goroutine and connection open indefinitely -- the classic
// Slowloris-style resource-exhaustion pattern. These are conservative
// defaults appropriate for a JSON API with no large uploads/downloads or
// long-lived streaming responses.
const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run sequences the boot path: config, then logging, then migrations, then
// the connection pool, then the HTTP server. Migrations complete before the
// listener starts (D-09). Any failure before listening returns a non-nil
// error so main exits non-zero — the service never reaches a
// running-but-broken state.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := logging.New(cfg)

	ctx := context.Background()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	srv := httpserver.New(pool, logger)

	addr := fmt.Sprintf(":%d", cfg.HTTPPort)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	logger.Info("starting server", "addr", addr)
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}
