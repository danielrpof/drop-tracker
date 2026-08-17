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
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/url"
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

// schemaScopedDSN derives a DSN pointing at the same database as dsn, but
// scoped to schema via a search_path query parameter. pgx treats
// unrecognised URL query parameters as Postgres runtime parameters applied
// to every connection the pool opens, so every unqualified table reference
// -- both a package's own INSERT/SELECT statements and golang-migrate's own
// schema_migrations bookkeeping -- resolves against schema instead of the
// connection role's default search_path (mirrors
// internal/db/migrate_test.go's identical technique for the from-scratch
// migration test).
func schemaScopedDSN(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// NewIsolatedTestPool is NewTestPool's schema-isolated sibling: it drops and
// recreates schema fresh, migrates only that schema, and returns a pool
// scoped to it via schemaScopedDSN.
//
// Use this instead of NewTestPool for any test whose assertions depend on
// an exact row count or exact call count against a table -- such as
// notifier.NotifyPending's ListUnnotified, a deliberately global,
// unfiltered query (D-06) -- that other concurrently-running packages'
// integration tests also write into via the shared fixture's default
// (public) schema. Without this, a concurrently-running package's own
// pending row is indistinguishable from this test's own rows to a global
// query, inflating observed counts, and a real NotifyPending call can even
// mark a concurrently-running package's own row as notified out from under
// its test. Package-level test parallelism (Go's default, and the reason
// this phase's own suite-stability requirement exists at all) is exactly
// when this race becomes reachable.
//
// A fixed schema name plus an if-exists guard is sufficient per package:
// Go runs one test binary per package, so two instances of a given
// package's tests cannot overlap and drop the same schema out from under
// each other. Different packages must pass different schema names to stay
// isolated from one another too.
func NewIsolatedTestPool(t *testing.T, schema string) *pgxpool.Pool {
	t.Helper()

	dsn := RequirePostgresDSN(t)
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: drop schema %s (setup): %v", schema, err)
	}
	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if _, err := sqlDB.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
			t.Errorf("testutil.NewIsolatedTestPool: cleanup: drop schema %s: %v", schema, err)
		}
	})

	scopedDSN, err := schemaScopedDSN(dsn, schema)
	if err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: derive scoped DSN: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := db.RunMigrations(ctx, scopedDSN, logger); err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: run migrations: %v", err)
	}

	pool, err := db.NewPool(ctx, scopedDSN)
	if err != nil {
		t.Fatalf("testutil.NewIsolatedTestPool: create pool: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}
