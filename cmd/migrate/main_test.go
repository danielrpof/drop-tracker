package main

// Whitebox tests: package main so it can drive the unexported run function
// directly (mirrors cmd/coverage-report/main_test.go).

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/testutil"
)

func TestRun_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	if err := run(context.Background()); err == nil {
		t.Fatal("run: want non-nil error when DATABASE_URL is empty, got nil")
	}
}

// scratchSchemaDSN derives a DSN pointing at the same database as dsn, but
// scoped to schema via a search_path query parameter -- the same technique
// internal/db/migrate_test.go and internal/testutil use.
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

// upMigrationVersionPattern extracts the NNNNNN version prefix from an
// up-migration filename.
var upMigrationVersionPattern = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

// highestMigrationVersion reads internal/db/migrations directly rather than
// hard-coding a migration count -- migrate_test.go's hard-coded version has
// already had to be bumped once (Phase 05-01).
func highestMigrationVersion(t *testing.T) int {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join("..", "..", "internal", "db", "migrations"))
	if err != nil {
		t.Fatalf("highestMigrationVersion: read migrations dir: %v", err)
	}

	max := 0
	for _, e := range entries {
		m := upMigrationVersionPattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if v > max {
			max = v
		}
	}
	if max == 0 {
		t.Fatal("highestMigrationVersion: no *.up.sql files found")
	}
	return max
}

// TestRun_AppliesHeadSchema points run() at a scratch schema via
// DATABASE_URL and asserts the resulting schema_migrations version matches
// the highest *.up.sql version on disk -- proving run() drives the same
// db.RunMigrations path cmd/server uses at boot, not a second code path.
func TestRun_AppliesHeadSchema(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	ctx := context.Background()
	const schema = "cmd_migrate_scratch"

	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
		t.Fatalf("drop scratch schema (setup): %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %s", schema)); err != nil {
		t.Fatalf("create scratch schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := sqlDB.ExecContext(context.Background(), fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema)); err != nil {
			t.Errorf("cleanup: drop scratch schema: %v", err)
		}
	})

	scratchDSN, err := scratchSchemaDSN(dsn, schema)
	if err != nil {
		t.Fatalf("derive scratch DSN: %v", err)
	}
	t.Setenv("DATABASE_URL", scratchDSN)

	if err := run(ctx); err != nil {
		t.Fatalf("run: %v", err)
	}

	var version int
	if err := sqlDB.QueryRowContext(ctx, fmt.Sprintf("SELECT version FROM %s.schema_migrations", schema)).Scan(&version); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	want := highestMigrationVersion(t)
	if version != want {
		t.Fatalf("schema_migrations.version = %d, want %d (highest *.up.sql on disk)", version, want)
	}
}
