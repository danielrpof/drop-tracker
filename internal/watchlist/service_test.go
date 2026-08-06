package watchlist_test

// This is internal/watchlist's first test file: real-Postgres coverage of
// Service.Add's error-translation and preferences behaviour (D-09, D-08,
// D-11). Every test shares one database (testutil.NewTestPool), so each
// derives a unique mbid from t.Name() rather than a hardcoded literal, and
// registers a t.Cleanup that deletes the rows it created -- tests never
// collide with each other or with a previous run.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/jackc/pgx/v5/pgconn"
)

// testMBID derives a short, unique-per-test mbid from t.Name() so tests
// sharing one database never collide, without hardcoding a literal.
func testMBID(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

func TestService_Add_DuplicateReturnsErrDuplicate(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	ctx := context.Background()

	if _, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "First Add"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	_, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Second Add"})
	if !errors.Is(err, watchlist.ErrDuplicate) {
		t.Fatalf("second Add error = %v, want errors.Is(err, watchlist.ErrDuplicate)", err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		t.Fatalf("second Add returned a raw *pgconn.PgError to the caller: %v", err)
	}
}

func TestService_Add_DuplicateLeavesPreferencesUntouched(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	ctx := context.Background()

	if _, err := svc.Add(ctx, watchlist.AddParams{
		MBID:            mbid,
		Name:            "First Add",
		ReleaseTypes:    []string{"album"},
		MutedEventTypes: []string{"deluxe_change"},
	}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	_, err := svc.Add(ctx, watchlist.AddParams{
		MBID:         mbid,
		Name:         "Second Add",
		ReleaseTypes: []string{"single", "ep", "deluxe"},
	})
	if !errors.Is(err, watchlist.ErrDuplicate) {
		t.Fatalf("second Add error = %v, want errors.Is(err, watchlist.ErrDuplicate)", err)
	}

	var releaseTypes, mutedEventTypes []string
	row := pool.QueryRow(ctx,
		`SELECT w.release_types, w.muted_event_types
		 FROM watchlist w JOIN artists a ON a.id = w.artist_id
		 WHERE a.mbid = $1`, mbid)
	if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query stored preferences: %v", err)
	}

	if len(releaseTypes) != 1 || releaseTypes[0] != "album" {
		t.Fatalf("release_types = %v, want [album] (unchanged by the rejected duplicate)", releaseTypes)
	}
	if len(mutedEventTypes) != 1 || mutedEventTypes[0] != "deluxe_change" {
		t.Fatalf("muted_event_types = %v, want [deluxe_change] (unchanged by the rejected duplicate)", mutedEventTypes)
	}
}

func TestService_Add_ReusesExistingArtistRow(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	if _, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "First Add"}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE artist_id = (SELECT id FROM artists WHERE mbid = $1)", mbid); err != nil {
		t.Fatalf("delete watchlist row: %v", err)
	}

	if _, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Second Add"}); err != nil {
		t.Fatalf("second Add (re-adding after watchlist row removed): %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artists WHERE mbid = $1", mbid).Scan(&count); err != nil {
		t.Fatalf("query artists count: %v", err)
	}
	if count != 1 {
		t.Fatalf("artists row count = %d, want 1 (the upsert must reuse the master row, not create a second one)", count)
	}
}
