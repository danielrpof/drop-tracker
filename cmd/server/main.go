// Command server is drop-tracker's single process entrypoint. It runs the
// HTTP API today and will run the robfig/cron scheduler in this same
// process starting Phase 3 — PROJECT.md locks a single-binary architecture.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
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
	logger.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, srv.Router()); err != nil {
		return fmt.Errorf("serve http: %w", err)
	}

	return nil
}
