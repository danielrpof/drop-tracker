package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// TestCountWatchlist_Integration proves the generated CountWatchlist method
// against a real database: its return matches a direct count of the
// watchlist table, and inserting one row moves it by exactly one. It asserts
// relative movement rather than an absolute count so a concurrently-running
// package's own watchlist rows in the shared fixture cannot make it flake.
func TestCountWatchlist_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)
	queries := sqlc.New(pool)
	ctx := context.Background()

	mbid := fmt.Sprintf("count-watchlist-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	before, err := queries.CountWatchlist(ctx)
	if err != nil {
		t.Fatalf("CountWatchlist (before): %v", err)
	}
	var directBefore int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM watchlist").Scan(&directBefore); err != nil {
		t.Fatalf("direct count (before): %v", err)
	}
	if before != directBefore {
		t.Fatalf("CountWatchlist = %d, direct count(*) = %d", before, directBefore)
	}

	var artistID int64
	if err := pool.QueryRow(ctx,
		"INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "Count Watchlist Test",
	).Scan(&artistID); err != nil {
		t.Fatalf("insert artist: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1)", artistID); err != nil {
		t.Fatalf("insert watchlist row: %v", err)
	}

	after, err := queries.CountWatchlist(ctx)
	if err != nil {
		t.Fatalf("CountWatchlist (after): %v", err)
	}
	if after != before+1 {
		t.Fatalf("CountWatchlist after insert = %d, want %d (before+1)", after, before+1)
	}
}
