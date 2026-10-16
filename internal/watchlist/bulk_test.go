package watchlist_test

import (
	"context"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// seedBulkEntries adds n entries and returns their ids in insertion order.
func seedBulkEntries(t *testing.T, ctx context.Context, svc *watchlist.Service, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := range n {
		e, err := svc.Add(ctx, watchlist.AddParams{
			MBID: testMBID(t) + "-" + string(rune('a'+i)),
			Name: "Bulk Artist " + string(rune('A'+i)),
		})
		if err != nil {
			t.Fatalf("seed entry %d: %v", i, err)
		}
		ids = append(ids, e.ID)
	}
	return ids
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func assertIDs(t *testing.T, label string, got, want []int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want non-nil %v", label, want)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestService_BulkRemove(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	t.Run("removes every existing row", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		res, err := svc.BulkRemove(ctx, []int64{ids[2], ids[0], ids[1]})
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, ids)
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{})
		for _, id := range ids {
			if n := countRows(t, ctx, pool, "SELECT count(*) FROM watchlist WHERE id = $1", id); n != 0 {
				t.Fatalf("watchlist row %d still present", id)
			}
		}
	})

	t.Run("duplicates and order are normalized", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		a, c := ids[0], ids[2]
		res, err := svc.BulkRemove(ctx, []int64{c, a, 999999999, a})
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{a, c})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{999999999})
		if n := countRows(t, ctx, pool, "SELECT count(*) FROM watchlist WHERE id = $1", ids[1]); n != 1 {
			t.Fatalf("unselected row count = %d, want 1", n)
		}
	})

	t.Run("an id deleted elsewhere is already gone", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", ids[1]); err != nil {
			t.Fatalf("direct delete: %v", err)
		}
		res, err := svc.BulkRemove(ctx, ids)
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{ids[0], ids[2]})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{ids[1]})
	})

	t.Run("a repeat reports everything already gone", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 2)
		if _, err := svc.BulkRemove(ctx, ids); err != nil {
			t.Fatalf("first BulkRemove: %v", err)
		}
		res, err := svc.BulkRemove(ctx, ids)
		if err != nil {
			t.Fatalf("second BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, ids)
	})

	t.Run("no ids returns empty non-nil slices", func(t *testing.T) {
		for _, in := range [][]int64{nil, {}} {
			res, err := svc.BulkRemove(ctx, in)
			if err != nil {
				t.Fatalf("BulkRemove(%v): %v", in, err)
			}
			assertIDs(t, "Removed", res.Removed, []int64{})
			assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{})
		}
	})
}

func TestService_BulkRemove_KeepsArtistTagsAndPendingEvents(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	mbid := testMBID(t)
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Keeper"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	note := "remember me"
	if _, err := svc.UpdateNote(ctx, entry.ID, &note); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	var tagID int64
	if err := pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ($1) RETURNING id", "bulk-keeper").Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", entry.ArtistID, tagID); err != nil {
		t.Fatalf("seed artist_tags: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		VALUES ($1, 'musicbrainz', 'new_release', $2, 'Title', 'Keeper')`, entry.ArtistID, "bulk-keeps-"+mbid); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	res, err := svc.BulkRemove(ctx, []int64{entry.ID})
	if err != nil {
		t.Fatalf("BulkRemove: %v", err)
	}
	assertIDs(t, "Removed", res.Removed, []int64{entry.ID})

	if n := countRows(t, ctx, pool, "SELECT count(*) FROM artists WHERE id = $1", entry.ArtistID); n != 1 {
		t.Fatalf("artists rows = %d, want 1", n)
	}
	if n := countRows(t, ctx, pool, "SELECT count(*) FROM artist_tags WHERE artist_id = $1 AND tag_id = $2", entry.ArtistID, tagID); n != 1 {
		t.Fatalf("artist_tags links = %d, want 1", n)
	}
	if n := countRows(t, ctx, pool, "SELECT count(*) FROM events WHERE artist_id = $1 AND notified_at IS NULL", entry.ArtistID); n != 1 {
		t.Fatalf("unnotified events = %d, want 1", n)
	}

	again, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Keeper"})
	if err != nil {
		t.Fatalf("re-Add: %v", err)
	}
	if again.Note != nil {
		t.Fatalf("Note after re-add = %q, want nil", *again.Note)
	}
}
