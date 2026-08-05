// Package testutil is the single shared Postgres test fixture for every
// DB-backed test in this and later phases. The chosen strategy is an
// environment-supplied DSN pointing at the docker-compose Postgres, not
// testcontainers-go: this keeps the test dependency set to what is already
// in go.mod, needs no Docker socket access from inside the test process, and
// works unchanged against the Phase 7 GitHub Actions Postgres service
// container.
package testutil

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RequirePostgresDSN returns a DSN for a reachable Postgres instance,
// reading TEST_DATABASE_URL and falling back to DATABASE_URL. It skips the
// calling test — with a message that always names TEST_DATABASE_URL and
// states how to start the fixture, so a skip is never mistaken for a pass —
// when testing.Short() is true or when neither variable is set.
func RequirePostgresDSN(t *testing.T) string {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping DB-backed test: -short is set; unset it and provide TEST_DATABASE_URL to run this test")
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("skipping DB-backed test: TEST_DATABASE_URL (or DATABASE_URL) is not set; run `docker compose up -d --wait postgres` and set TEST_DATABASE_URL to include this test")
	}

	return dsn
}

// NewTestPool returns a ready-to-use pool for the fixture Postgres instance:
// it resolves the DSN via RequirePostgresDSN, applies the embedded
// migrations, constructs the pool, and registers t.Cleanup to close it.
// Any failure along the way fails the test immediately.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := RequirePostgresDSN(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.RunMigrations(ctx, dsn, logger); err != nil {
		t.Fatalf("testutil.NewTestPool: run migrations: %v", err)
	}

	pool, err := db.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("testutil.NewTestPool: create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}
