package httpserver_test

// This file exercises the real boot chain — config.Load -> logging.New ->
// db.RunMigrations -> db.NewPool -> httpserver.New -> Router() — against a
// real Postgres instance, package httpserver_test so it imports httpserver
// exactly as cmd/server does rather than reaching into its internals.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

func TestBootToHealth_EndToEnd(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("HTTP_PORT", "0")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	logger := logging.New(cfg)
	ctx := context.Background()

	if err := db.RunMigrations(ctx, cfg.DatabaseURL, logger); err != nil {
		t.Fatalf("db.RunMigrations: %v", err)
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("db.NewPool: %v", err)
	}
	defer pool.Close()

	store := watchlist.NewService(sqlc.New(pool))
	eventsStore := events.NewService(sqlc.New(pool), 90)
	srv := httpserver.New(pool, store, eventsStore, nil, logger)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("http.Get /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		Status string `json:"status"`
		DB     string `json:"db"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Status != "ok" || body.DB != "up" {
		t.Fatalf("body = %+v, want {Status:ok DB:up}", body)
	}
}

func TestBootToHealth_MigrationsAreIdempotent(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	if err := db.RunMigrations(ctx, dsn, logger); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := db.RunMigrations(ctx, dsn, logger); err != nil {
		t.Fatalf("second RunMigrations (migrate.ErrNoChange must be treated as success): %v", err)
	}
}
