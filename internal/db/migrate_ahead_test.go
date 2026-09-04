package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// scratchSchemaDSN mirrors migrate_test.go's helper of the same name
// exactly, reimplemented locally rather than shared: migrate_test.go is
// package db_test and this file is package db (in-package, to reach the
// unexported runMigrationsWithSource seam), so the two packages cannot
// share an unexported identifier across files.
func scratchSchemaDSN(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// requirePostgresDSN mirrors internal/testutil.RequirePostgresDSN's gating
// exactly (skip on -short or a missing TEST_DATABASE_URL/DATABASE_URL), but
// is reimplemented locally rather than imported: this file is package db
// (in-package, to reach the unexported runMigrationsWithSource seam), and
// internal/testutil imports internal/db -- importing it here would be an
// import cycle.
func requirePostgresDSN(t *testing.T) string {
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

// mapFSSubdir is the fixed subdirectory mapFSSource builds its synthetic
// migration files under -- iofs.New requires a subdirectory name, and a
// fixed one is fine since each call gets its own fstest.MapFS instance.
const mapFSSubdir = "m"

// mapFSSource builds a synthetic source.Driver over versions 1..max
// (inclusive), each a trivial "SELECT 1;" up/down pair. It is deliberately
// decoupled from the real migration count (7 today, and moving) so this
// test's guarantee never depends on how many real migrations exist.
func mapFSSource(t *testing.T, maxVersion uint) source.Driver {
	t.Helper()

	mapFS := fstest.MapFS{}
	for v := uint(1); v <= maxVersion; v++ {
		up := fmt.Sprintf("%s/%06d_step.up.sql", mapFSSubdir, v)
		down := fmt.Sprintf("%s/%06d_step.down.sql", mapFSSubdir, v)
		mapFS[up] = &fstest.MapFile{Data: []byte("SELECT 1;")}
		mapFS[down] = &fstest.MapFile{Data: []byte("SELECT 1;")}
	}

	src, err := iofs.New(mapFS, mapFSSubdir)
	if err != nil {
		t.Fatalf("mapFSSource: iofs.New: %v", err)
	}
	return src
}

// migrateAheadScratchDB opens a dedicated scratch schema (name distinct per
// call) and returns both a *sql.DB against the unscoped dsn (for direct
// schema_migrations assertions/mutations) and the schema-scoped DSN
// runMigrationsWithSource should be called against. Mirrors
// migrate_test.go's TestRunMigrations_AppliesFromScratch drop/create/cleanup
// dance, generalized to a caller-supplied schema name so each sub-test in
// this file gets its own isolated schema.
func migrateAheadScratchDB(t *testing.T, dsn, schema string) (*sql.DB, string) {
	t.Helper()
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("migrateAheadScratchDB: open db: %v", err)
	}
	// Registered before the schema-drop cleanup below so it runs after that
	// cleanup (t.Cleanup is LIFO): the drop needs a live connection.
	t.Cleanup(func() { _ = sqlDB.Close() })

	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
		t.Fatalf("migrateAheadScratchDB: drop schema %s (setup): %v", schema, err)
	}
	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatalf("migrateAheadScratchDB: create schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if _, err := sqlDB.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
			t.Errorf("migrateAheadScratchDB: cleanup: drop schema %s: %v", schema, err)
		}
	})

	scratchDSN, err := scratchSchemaDSN(dsn, schema)
	if err != nil {
		t.Fatalf("migrateAheadScratchDB: derive scratch DSN: %v", err)
	}
	return sqlDB, scratchDSN
}

func migrateAheadDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource is the SC #4 proof
// (RESEARCH.md Finding 1 / D-02 amended, D-18): the previously-released
// binary's boot migration, driven through the real runMigrationsWithSource
// boot path, no-ops against a schema_migrations version ahead of its
// embedded source rather than erroring. Before the ahead-of-source guard
// lands (commit 2, this file), this test is expected RED -- the failure
// text is the recorded evidence that golang-migrate v4.19.1's Up() returns
// a hard "no migration found for version N+1" error, not ErrNoChange, on an
// unmodified runMigrationsOnce. Commit 3 lands the guard and turns it GREEN.
func TestRunMigrationsWithSource_NoOpsAgainstAheadOfSource(t *testing.T) {
	dsn := requirePostgresDSN(t)
	ctx := context.Background()
	logger := migrateAheadDiscardLogger()

	const n = 5
	sqlDB, scratchDSN := migrateAheadScratchDB(t, dsn, "migrate_ahead_scratch_noop")

	fullSrc := mapFSSource(t, n+1)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("prime to n+1: runMigrationsWithSource: %v", err)
	}

	nSrc := mapFSSource(t, n)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, nSrc); err != nil {
		t.Fatalf("runMigrationsWithSource against ahead-of-source schema: %v", err)
	}

	var version int
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT version, dirty FROM migrate_ahead_scratch_noop.schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != n+1 || dirty {
		t.Fatalf("schema_migrations = (version=%d, dirty=%v), want (%d, false) -- the old binary must observe the schema, not rewrite it", version, dirty, n+1)
	}
}

// TestRunMigrationsWithSource_DirtyAheadStillErrors pins the guard's one
// deliberate non-inert path (RESEARCH.md Finding 1 / Pitfall F, T-16-01):
// a dirty schema_migrations row at an ahead-of-source version must never be
// swallowed as a benign no-op -- a half-applied forward migration blocks
// the old binary too and must surface as an error.
func TestRunMigrationsWithSource_DirtyAheadStillErrors(t *testing.T) {
	dsn := requirePostgresDSN(t)
	ctx := context.Background()
	logger := migrateAheadDiscardLogger()

	const n = 5
	const schema = "migrate_ahead_scratch_dirty"
	sqlDB, scratchDSN := migrateAheadScratchDB(t, dsn, schema)

	fullSrc := mapFSSource(t, n+1)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("prime to n+1: runMigrationsWithSource: %v", err)
	}

	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("UPDATE %s.schema_migrations SET dirty = true", schema)); err != nil {
		t.Fatalf("force dirty: %v", err)
	}

	nSrc := mapFSSource(t, n)
	err := runMigrationsWithSource(ctx, scratchDSN, logger, nSrc)
	if err == nil {
		t.Fatal("runMigrationsWithSource against a dirty ahead-of-source schema: want non-nil error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "dirty") {
		t.Fatalf("error %q does not name a dirty database state", err.Error())
	}
}

// TestRunMigrationsWithSource_BehindBySourceStillMigratesForward pins that
// the guard is inert when cur < smax: a database one version behind the
// source still migrates forward normally.
func TestRunMigrationsWithSource_BehindBySourceStillMigratesForward(t *testing.T) {
	dsn := requirePostgresDSN(t)
	ctx := context.Background()
	logger := migrateAheadDiscardLogger()

	const n = 5
	const schema = "migrate_ahead_scratch_behind"
	sqlDB, scratchDSN := migrateAheadScratchDB(t, dsn, schema)

	behindSrc := mapFSSource(t, n-1)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, behindSrc); err != nil {
		t.Fatalf("prime to n-1: runMigrationsWithSource: %v", err)
	}

	fullSrc := mapFSSource(t, n)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("runMigrationsWithSource against a behind-by-one schema: %v", err)
	}

	var version int
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, fmt.Sprintf("SELECT version, dirty FROM %s.schema_migrations", schema)).Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != n || dirty {
		t.Fatalf("schema_migrations = (version=%d, dirty=%v), want (%d, false)", version, dirty, n)
	}
}

// TestRunMigrationsWithSource_FreshDatabaseAppliesFromScratch pins that the
// guard does not fire on a fresh database (m.Version() returns
// migrate.ErrNilVersion, RESEARCH.md Finding 1's ErrNilVersion note) -- the
// full source is applied from scratch exactly as before the guard existed.
func TestRunMigrationsWithSource_FreshDatabaseAppliesFromScratch(t *testing.T) {
	dsn := requirePostgresDSN(t)
	ctx := context.Background()
	logger := migrateAheadDiscardLogger()

	const n = 5
	const schema = "migrate_ahead_scratch_fresh"
	sqlDB, scratchDSN := migrateAheadScratchDB(t, dsn, schema)

	fullSrc := mapFSSource(t, n)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("runMigrationsWithSource against a fresh database: %v", err)
	}

	var version int
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, fmt.Sprintf("SELECT version, dirty FROM %s.schema_migrations", schema)).Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != n || dirty {
		t.Fatalf("schema_migrations = (version=%d, dirty=%v), want (%d, false)", version, dirty, n)
	}
}

// TestRunMigrationsWithSource_IsIdempotentAtEqualVersion pins that calling
// runMigrationsWithSource twice in a row with the same source returns nil
// both times -- the second call resolves through migrate.ErrNoChange
// (equal version), not the ahead-of-source guard.
func TestRunMigrationsWithSource_IsIdempotentAtEqualVersion(t *testing.T) {
	dsn := requirePostgresDSN(t)
	ctx := context.Background()
	logger := migrateAheadDiscardLogger()

	const n = 5
	const schema = "migrate_ahead_scratch_idempotent"
	_, scratchDSN := migrateAheadScratchDB(t, dsn, schema)

	fullSrc := mapFSSource(t, n)
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("first runMigrationsWithSource: %v", err)
	}
	if err := runMigrationsWithSource(ctx, scratchDSN, logger, fullSrc); err != nil {
		t.Fatalf("second runMigrationsWithSource: %v", err)
	}
}
