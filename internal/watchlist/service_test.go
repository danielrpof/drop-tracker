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
	"reflect"
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

func TestService_Add_DefaultsWhenPreferencesOmitted(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Defaults Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !reflect.DeepEqual(entry.ReleaseTypes, watchlist.ReleaseTypes) {
		t.Fatalf("release_types = %v, want %v (D-08 default)", entry.ReleaseTypes, watchlist.ReleaseTypes)
	}
	if len(entry.MutedEventTypes) != 0 {
		t.Fatalf("muted_event_types = %v, want empty (D-08 default)", entry.MutedEventTypes)
	}
}

// TestService_Add_DBDefaultsMatchGoAllowList and TestCheckConstraintRejectsUnknownValue
// prove the column DEFAULT clauses / CHECK constraints in
// internal/db/migrations/000002_watchlist.up.sql have not drifted apart from
// the watchlist.ReleaseTypes / watchlist.EventTypes Go constants. A future
// migration that adds a release or event type must update both sides, or
// these two tests fail.
func TestService_Add_DBDefaultsMatchGoAllowList(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	var artistID int64
	if err := pool.QueryRow(ctx,
		"INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "DB Defaults Test",
	).Scan(&artistID); err != nil {
		t.Fatalf("insert artist: %v", err)
	}

	if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1)", artistID); err != nil {
		t.Fatalf("insert watchlist row with no preference columns: %v", err)
	}

	var releaseTypes, mutedEventTypes []string
	if err := pool.QueryRow(ctx,
		"SELECT release_types, muted_event_types FROM watchlist WHERE artist_id = $1", artistID,
	).Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query defaults: %v", err)
	}

	if !reflect.DeepEqual(releaseTypes, watchlist.ReleaseTypes) {
		t.Fatalf("DB column default release_types = %v, want %v (watchlist.ReleaseTypes)", releaseTypes, watchlist.ReleaseTypes)
	}
	if len(mutedEventTypes) != 0 {
		t.Fatalf("DB column default muted_event_types = %v, want empty", mutedEventTypes)
	}
}

func TestService_Add_PersistsSuppliedPreferences(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	entry, err := svc.Add(ctx, watchlist.AddParams{
		MBID:            mbid,
		Name:            "Supplied Preferences Test",
		ReleaseTypes:    []string{"ep", "album"},
		MutedEventTypes: []string{"guest_feature"},
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	wantReleaseTypes := []string{"album", "ep"}
	if !reflect.DeepEqual(entry.ReleaseTypes, wantReleaseTypes) {
		t.Fatalf("release_types = %v, want %v (canonical order)", entry.ReleaseTypes, wantReleaseTypes)
	}
	wantMutedEventTypes := []string{"guest_feature"}
	if !reflect.DeepEqual(entry.MutedEventTypes, wantMutedEventTypes) {
		t.Fatalf("muted_event_types = %v, want %v", entry.MutedEventTypes, wantMutedEventTypes)
	}
}

func TestService_Add_RejectsUnknownPreferenceValues(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	_, err := svc.Add(ctx, watchlist.AddParams{
		MBID:         mbid,
		Name:         "Rejects Unknown Values Test",
		ReleaseTypes: []string{"mixtape"},
	})
	if !errors.Is(err, watchlist.ErrInvalidReleaseType) {
		t.Fatalf("err = %v, want errors.Is(err, watchlist.ErrInvalidReleaseType)", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artists WHERE mbid = $1", mbid).Scan(&count); err != nil {
		t.Fatalf("query artists count: %v", err)
	}
	if count != 0 {
		t.Fatalf("artists row count = %d, want 0 (validation must run before any write)", count)
	}
}

func TestCheckConstraintRejectsUnknownValue(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Check Constraint Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	_, err = pool.Exec(ctx, "UPDATE watchlist SET release_types = ARRAY['mixtape']::text[] WHERE id = $1", entry.ID)
	if err == nil {
		t.Fatal("raw UPDATE with an unknown release type succeeded, want a CHECK constraint violation")
	}
}
