package db_test

// TestSQLCPing proves the sqlc codegen path is wired end to end: the
// generated Queries type executes over the same pgx pool the service uses,
// against a real Postgres, and the health-backing query returns real data.

import (
	"context"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

func TestSQLCPing(t *testing.T) {
	pool := testutil.NewTestPool(t)
	queries := sqlc.New(pool)

	got, err := queries.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if got != 1 {
		t.Fatalf("Ping = %d, want 1", got)
	}
}
