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

// The four tests below cover Service.List (WLST-04): every entry present
// with both id columns distinct, deterministic name-then-id ordering, and a
// non-nil empty-slice contract so the handler never has to special-case a
// nil result.

func TestService_List_ReturnsAllEntriesWithBothIDs(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	base := testMBID(t)
	seedMBID := base + "-seed"
	mbid1 := base + "-1"
	mbid2 := base + "-2"
	t.Cleanup(func() {
		for _, m := range []string{seedMBID, mbid1, mbid2} {
			if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", m); err != nil {
				t.Fatalf("cleanup: delete artists row: %v", err)
			}
		}
	})

	// Seed a throwaway artist row (no watchlist entry) so the artists and
	// watchlist id sequences cannot coincidentally align.
	if _, err := pool.Exec(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2)", seedMBID, "Seed Artist"); err != nil {
		t.Fatalf("insert seed artist: %v", err)
	}

	e1, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid1, Name: "Entry One"})
	if err != nil {
		t.Fatalf("Add entry one: %v", err)
	}
	e2, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid2, Name: "Entry Two"})
	if err != nil {
		t.Fatalf("Add entry two: %v", err)
	}

	entries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found1, found2 *watchlist.Entry
	for i := range entries {
		switch entries[i].MBID {
		case mbid1:
			found1 = &entries[i]
		case mbid2:
			found2 = &entries[i]
		}
	}
	if found1 == nil || found2 == nil {
		t.Fatalf("List did not return both added entries (found1=%v found2=%v)", found1, found2)
	}

	if found1.ID == found1.ArtistID {
		t.Fatalf("entry one: ID (%d) and ArtistID (%d) coincide, want distinct values", found1.ID, found1.ArtistID)
	}
	if found2.ID == found2.ArtistID {
		t.Fatalf("entry two: ID (%d) and ArtistID (%d) coincide, want distinct values", found2.ID, found2.ArtistID)
	}
	if found1.ID != e1.ID || found1.ArtistID != e1.ArtistID {
		t.Fatalf("entry one: ID=%d ArtistID=%d, want ID=%d ArtistID=%d", found1.ID, found1.ArtistID, e1.ID, e1.ArtistID)
	}
	if found2.ID != e2.ID || found2.ArtistID != e2.ArtistID {
		t.Fatalf("entry two: ID=%d ArtistID=%d, want ID=%d ArtistID=%d", found2.ID, found2.ArtistID, e2.ID, e2.ArtistID)
	}
	if found1.MBID != mbid1 || found1.Name != "Entry One" {
		t.Fatalf("entry one: MBID=%q Name=%q, want MBID=%q Name=%q", found1.MBID, found1.Name, mbid1, "Entry One")
	}
	if !reflect.DeepEqual(found1.ReleaseTypes, watchlist.ReleaseTypes) {
		t.Fatalf("entry one: ReleaseTypes=%v, want %v (from watchlist table, D-08 default)", found1.ReleaseTypes, watchlist.ReleaseTypes)
	}
	if len(found1.MutedEventTypes) != 0 {
		t.Fatalf("entry one: MutedEventTypes=%v, want empty (D-08 default)", found1.MutedEventTypes)
	}
}

func TestService_List_OrdersByNameThenID(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	base := testMBID(t)
	mbidAA := base + "-aa"
	mbidBB1 := base + "-bb1"
	mbidBB2 := base + "-bb2"
	t.Cleanup(func() {
		for _, m := range []string{mbidAA, mbidBB1, mbidBB2} {
			if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", m); err != nil {
				t.Fatalf("cleanup: delete artists row: %v", err)
			}
		}
	})

	if _, err := svc.Add(ctx, watchlist.AddParams{MBID: mbidAA, Name: "AA"}); err != nil {
		t.Fatalf("Add AA: %v", err)
	}
	bb1, err := svc.Add(ctx, watchlist.AddParams{MBID: mbidBB1, Name: "BB"})
	if err != nil {
		t.Fatalf("Add BB (first): %v", err)
	}
	bb2, err := svc.Add(ctx, watchlist.AddParams{MBID: mbidBB2, Name: "BB"})
	if err != nil {
		t.Fatalf("Add BB (second): %v", err)
	}

	for i := 0; i < 3; i++ {
		entries, err := svc.List(ctx)
		if err != nil {
			t.Fatalf("run %d: List: %v", i, err)
		}

		var filtered []watchlist.Entry
		for _, e := range entries {
			if e.MBID == mbidAA || e.MBID == mbidBB1 || e.MBID == mbidBB2 {
				filtered = append(filtered, e)
			}
		}
		if len(filtered) != 3 {
			t.Fatalf("run %d: filtered entries = %d, want 3", i, len(filtered))
		}
		if filtered[0].MBID != mbidAA {
			t.Fatalf("run %d: filtered[0].MBID = %q, want %q (AA sorts before BB)", i, filtered[0].MBID, mbidAA)
		}
		if filtered[1].ArtistID != bb1.ArtistID || filtered[2].ArtistID != bb2.ArtistID {
			t.Fatalf("run %d: filtered[1].ArtistID=%d filtered[2].ArtistID=%d, want %d then %d (ascending artist id tiebreak among equal names)",
				i, filtered[1].ArtistID, filtered[2].ArtistID, bb1.ArtistID, bb2.ArtistID)
		}
	}
}

func TestService_List_IdenticalNamesStayDistinct(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	base := testMBID(t)
	mbid1 := base + "-x1"
	mbid2 := base + "-x2"
	t.Cleanup(func() {
		for _, m := range []string{mbid1, mbid2} {
			if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", m); err != nil {
				t.Fatalf("cleanup: delete artists row: %v", err)
			}
		}
	})

	e1, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid1, Name: "Same Name"})
	if err != nil {
		t.Fatalf("Add entry one: %v", err)
	}
	e2, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid2, Name: "Same Name"})
	if err != nil {
		t.Fatalf("Add entry two: %v", err)
	}

	entries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var found1, found2 *watchlist.Entry
	for i := range entries {
		switch entries[i].MBID {
		case mbid1:
			found1 = &entries[i]
		case mbid2:
			found2 = &entries[i]
		}
	}
	if found1 == nil || found2 == nil {
		t.Fatalf("List did not return both same-named entries (found1=%v found2=%v)", found1, found2)
	}
	if found1.ID == found2.ID {
		t.Fatalf("both entries share ID %d, want distinct ids", found1.ID)
	}
	if found1.ArtistID == found2.ArtistID {
		t.Fatalf("both entries share ArtistID %d, want distinct artist ids", found1.ArtistID)
	}
	if found1.MBID == found2.MBID {
		t.Fatalf("both entries share MBID %q, want distinct mbids", found1.MBID)
	}
	_ = e1
	_ = e2
}

func TestService_List_EmptyReturnsNonNilSlice(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM watchlist").Scan(&count); err != nil {
		t.Fatalf("query watchlist count: %v", err)
	}

	entries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if entries == nil {
		t.Fatal("List returned a nil slice, want a non-nil slice (nil encodes as JSON null)")
	}
	if count == 0 && len(entries) != 0 {
		t.Fatalf("watchlist table was empty but List returned %d entries", len(entries))
	}
}

// The five tests below cover Service.Remove (WLST-03): a real hard delete
// that leaves the artists master row untouched (D-03), reports ErrNotFound
// on a missing or already-removed id, and leaves no tombstone blocking a
// fresh re-add with the same mbid (D-10).

func TestService_Remove_DeletesRow(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Remove Deletes Row Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := svc.Remove(ctx, entry.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	entries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, e := range entries {
		if e.ID == entry.ID {
			t.Fatalf("List still contains removed entry id %d", entry.ID)
		}
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM watchlist WHERE id = $1", entry.ID).Scan(&count); err != nil {
		t.Fatalf("query watchlist count: %v", err)
	}
	if count != 0 {
		t.Fatalf("watchlist row count = %d, want 0", count)
	}
}

func TestService_Remove_LeavesArtistRowIntact(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Remove Leaves Artist Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := svc.Remove(ctx, entry.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artists WHERE mbid = $1", mbid).Scan(&count); err != nil {
		t.Fatalf("query artists count: %v", err)
	}
	if count != 1 {
		t.Fatalf("artists row count = %d, want 1 (D-03: master row survives a watchlist removal)", count)
	}
}

func TestService_Remove_SecondCallReturnsErrNotFound(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Remove Twice Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if err := svc.Remove(ctx, entry.ID); err != nil {
		t.Fatalf("first Remove: %v", err)
	}

	err = svc.Remove(ctx, entry.ID)
	if !errors.Is(err, watchlist.ErrNotFound) {
		t.Fatalf("second Remove error = %v, want errors.Is(err, watchlist.ErrNotFound)", err)
	}
}

func TestService_Remove_UnknownIDReturnsErrNotFound(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	err := svc.Remove(ctx, 999999999)
	if !errors.Is(err, watchlist.ErrNotFound) {
		t.Fatalf("Remove error = %v, want errors.Is(err, watchlist.ErrNotFound)", err)
	}
}

func TestService_Remove_ThenReAddSucceeds(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	first, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Re-Add Test"})
	if err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if err := svc.Remove(ctx, first.ID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	second, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Re-Add Test"})
	if err != nil {
		t.Fatalf("re-Add: %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("re-Add reused the removed watchlist id %d, want a new id (no tombstone, D-10)", first.ID)
	}
}

// The seven tests below cover Service.UpdatePreferences (WLST-05, WLST-06,
// D-11): independent partial updates to each preference axis, the
// absent-vs-empty distinction, de-duplication/canonicalisation, allow-list
// rejection with the stored row untouched, and an unknown id.

func TestService_UpdatePreferences_SetsReleaseTypes(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Update Release Types Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
		ReleaseTypes: &[]string{"album", "ep"},
	})
	if err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}

	wantReleaseTypes := []string{"album", "ep"}
	if !reflect.DeepEqual(updated.ReleaseTypes, wantReleaseTypes) {
		t.Fatalf("release_types = %v, want %v", updated.ReleaseTypes, wantReleaseTypes)
	}
	if len(updated.MutedEventTypes) != 0 {
		t.Fatalf("muted_event_types = %v, want empty (untouched axis)", updated.MutedEventTypes)
	}

	var releaseTypes, mutedEventTypes []string
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query stored preferences: %v", err)
	}
	if !reflect.DeepEqual(releaseTypes, wantReleaseTypes) {
		t.Fatalf("stored release_types = %v, want %v", releaseTypes, wantReleaseTypes)
	}
	if len(mutedEventTypes) != 0 {
		t.Fatalf("stored muted_event_types = %v, want empty", mutedEventTypes)
	}
}

func TestService_UpdatePreferences_SetsMutedEventTypes(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Update Muted Event Types Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	updated, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
		MutedEventTypes: &[]string{"guest_feature"},
	})
	if err != nil {
		t.Fatalf("UpdatePreferences: %v", err)
	}

	if !reflect.DeepEqual(updated.ReleaseTypes, watchlist.ReleaseTypes) {
		t.Fatalf("release_types = %v, want unchanged default %v", updated.ReleaseTypes, watchlist.ReleaseTypes)
	}
	wantMutedEventTypes := []string{"guest_feature"}
	if !reflect.DeepEqual(updated.MutedEventTypes, wantMutedEventTypes) {
		t.Fatalf("muted_event_types = %v, want %v", updated.MutedEventTypes, wantMutedEventTypes)
	}

	var releaseTypes, mutedEventTypes []string
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query stored preferences: %v", err)
	}
	if !reflect.DeepEqual(releaseTypes, watchlist.ReleaseTypes) {
		t.Fatalf("stored release_types = %v, want unchanged default %v", releaseTypes, watchlist.ReleaseTypes)
	}
	if !reflect.DeepEqual(mutedEventTypes, wantMutedEventTypes) {
		t.Fatalf("stored muted_event_types = %v, want %v", mutedEventTypes, wantMutedEventTypes)
	}
}

func TestService_UpdatePreferences_AxesAreIndependent(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Axes Independent Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
		ReleaseTypes: &[]string{"single"},
	}); err != nil {
		t.Fatalf("first UpdatePreferences: %v", err)
	}

	if _, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
		MutedEventTypes: &[]string{"deluxe_change"},
	}); err != nil {
		t.Fatalf("second UpdatePreferences: %v", err)
	}

	var releaseTypes, mutedEventTypes []string
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query stored preferences: %v", err)
	}
	if len(releaseTypes) != 1 || releaseTypes[0] != "single" {
		t.Fatalf("release_types = %v, want [single] (the second call must not reset it)", releaseTypes)
	}
	if len(mutedEventTypes) != 1 || mutedEventTypes[0] != "deluxe_change" {
		t.Fatalf("muted_event_types = %v, want [deluxe_change]", mutedEventTypes)
	}
}

func TestService_UpdatePreferences_EmptyArrayIsNotOmission(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "release_types"},
		{name: "muted_event_types"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.NewTestPool(t)
			mbid := testMBID(t) + "-" + tt.name
			ctx := context.Background()
			t.Cleanup(func() {
				if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
					t.Fatalf("cleanup: delete artists row: %v", err)
				}
			})

			svc := watchlist.NewService(sqlc.New(pool))
			entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Empty Not Omission Test"})
			if err != nil {
				t.Fatalf("Add: %v", err)
			}

			empty := []string{}
			var firstParams, secondParams watchlist.PreferencesParams
			if tt.name == "release_types" {
				firstParams.ReleaseTypes = &empty
				secondParams.MutedEventTypes = &[]string{"new_release"}
			} else {
				firstParams.MutedEventTypes = &empty
				secondParams.ReleaseTypes = &[]string{"album"}
			}

			first, err := svc.UpdatePreferences(ctx, entry.ID, firstParams)
			if err != nil {
				t.Fatalf("first UpdatePreferences (set empty): %v", err)
			}

			var firstGot []string
			if tt.name == "release_types" {
				firstGot = first.ReleaseTypes
			} else {
				firstGot = first.MutedEventTypes
			}
			if firstGot == nil || len(firstGot) != 0 {
				t.Fatalf("after setting empty, %s = %v, want a non-nil empty slice", tt.name, firstGot)
			}

			second, err := svc.UpdatePreferences(ctx, entry.ID, secondParams)
			if err != nil {
				t.Fatalf("second UpdatePreferences (nil pointer for %s): %v", tt.name, err)
			}

			var secondGot []string
			if tt.name == "release_types" {
				secondGot = second.ReleaseTypes
			} else {
				secondGot = second.MutedEventTypes
			}
			if len(secondGot) != 0 {
				t.Fatalf("after nil-pointer call, %s = %v, want [] (must stay empty, not reset)", tt.name, secondGot)
			}
		})
	}
}

func TestService_UpdatePreferences_DeduplicatesAndCanonicalises(t *testing.T) {
	tests := []struct {
		name   string
		axis   string
		submit []string
		want   []string
	}{
		{name: "release_types", axis: "release", submit: []string{"deluxe", "album", "album"}, want: []string{"album", "deluxe"}},
		{name: "muted_event_types", axis: "muted", submit: []string{"deluxe_change", "new_release", "deluxe_change"}, want: []string{"new_release", "deluxe_change"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.NewTestPool(t)
			mbid := testMBID(t) + "-" + tt.name
			ctx := context.Background()
			t.Cleanup(func() {
				if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
					t.Fatalf("cleanup: delete artists row: %v", err)
				}
			})

			svc := watchlist.NewService(sqlc.New(pool))
			entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Dedup Canon Test"})
			if err != nil {
				t.Fatalf("Add: %v", err)
			}

			var params watchlist.PreferencesParams
			submit := tt.submit
			if tt.axis == "release" {
				params.ReleaseTypes = &submit
			} else {
				params.MutedEventTypes = &submit
			}

			updated, err := svc.UpdatePreferences(ctx, entry.ID, params)
			if err != nil {
				t.Fatalf("UpdatePreferences: %v", err)
			}

			var got []string
			if tt.axis == "release" {
				got = updated.ReleaseTypes
			} else {
				got = updated.MutedEventTypes
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("%s = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestService_UpdatePreferences_RejectsUnknownValue(t *testing.T) {
	tests := []struct {
		name    string
		axis    string
		bad     []string
		wantErr error
	}{
		{name: "release_types", axis: "release", bad: []string{"mixtape"}, wantErr: watchlist.ErrInvalidReleaseType},
		{name: "muted_event_types", axis: "muted", bad: []string{"remix_drop"}, wantErr: watchlist.ErrInvalidEventType},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.NewTestPool(t)
			mbid := testMBID(t) + "-" + tt.name
			ctx := context.Background()
			t.Cleanup(func() {
				if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
					t.Fatalf("cleanup: delete artists row: %v", err)
				}
			})

			svc := watchlist.NewService(sqlc.New(pool))
			entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Rejects Unknown Test"})
			if err != nil {
				t.Fatalf("Add: %v", err)
			}

			var params watchlist.PreferencesParams
			bad := tt.bad
			if tt.axis == "release" {
				params.ReleaseTypes = &bad
			} else {
				params.MutedEventTypes = &bad
			}

			_, err = svc.UpdatePreferences(ctx, entry.ID, params)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want errors.Is(err, %v)", err, tt.wantErr)
			}

			var releaseTypes, mutedEventTypes []string
			row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
			if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
				t.Fatalf("query stored preferences: %v", err)
			}
			if !reflect.DeepEqual(releaseTypes, watchlist.ReleaseTypes) {
				t.Fatalf("release_types = %v, want unchanged default %v", releaseTypes, watchlist.ReleaseTypes)
			}
			if len(mutedEventTypes) != 0 {
				t.Fatalf("muted_event_types = %v, want unchanged empty default", mutedEventTypes)
			}
		})
	}
}

func TestService_UpdatePreferences_UnknownIDReturnsErrNotFound(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	_, err := svc.UpdatePreferences(ctx, 999999999, watchlist.PreferencesParams{
		ReleaseTypes: &[]string{"album"},
	})
	if !errors.Is(err, watchlist.ErrNotFound) {
		t.Fatalf("err = %v, want errors.Is(err, watchlist.ErrNotFound)", err)
	}
}

// The four tests below prove the watchlist_release_types_valid and
// watchlist_muted_event_types_valid CHECK constraints reject an
// out-of-allow-list value written by raw SQL that bypasses Service entirely
// -- the non-bypassable second layer behind normalizeSet's 400 (T-02-10).

func TestCheckConstraint_RejectsUnknownReleaseType(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Constraint Release Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	_, err = pool.Exec(ctx, "UPDATE watchlist SET release_types = ARRAY['mixtape']::text[] WHERE id = $1", entry.ID)
	if err == nil {
		t.Fatal("raw UPDATE with an unknown release type succeeded, want a CHECK constraint violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("err = %v (%T), want *pgconn.PgError", err, err)
	}
	if pgErr.ConstraintName != "watchlist_release_types_valid" {
		t.Fatalf("ConstraintName = %q, want %q", pgErr.ConstraintName, "watchlist_release_types_valid")
	}
}

func TestCheckConstraint_RejectsUnknownEventType(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Constraint Event Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	_, err = pool.Exec(ctx, "UPDATE watchlist SET muted_event_types = ARRAY['remix_drop']::text[] WHERE id = $1", entry.ID)
	if err == nil {
		t.Fatal("raw UPDATE with an unknown event type succeeded, want a CHECK constraint violation")
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("err = %v (%T), want *pgconn.PgError", err, err)
	}
	if pgErr.ConstraintName != "watchlist_muted_event_types_valid" {
		t.Fatalf("ConstraintName = %q, want %q", pgErr.ConstraintName, "watchlist_muted_event_types_valid")
	}
}

func TestCheckConstraint_AcceptsEmptyArrays(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Constraint Empty Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := pool.Exec(ctx,
		"UPDATE watchlist SET release_types = '{}'::text[], muted_event_types = '{}'::text[] WHERE id = $1", entry.ID,
	); err != nil {
		t.Fatalf("UPDATE with empty arrays failed: %v (an empty set must be representable, not a constraint violation)", err)
	}
}

func TestCheckConstraint_AcceptsFullAllowLists(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Constraint Full Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	if _, err := pool.Exec(ctx,
		"UPDATE watchlist SET release_types = $2::text[], muted_event_types = $3::text[] WHERE id = $1",
		entry.ID, watchlist.ReleaseTypes, watchlist.EventTypes,
	); err != nil {
		t.Fatalf("UPDATE with every allow-listed value failed: %v (Go allow-list and DB CHECK constraint have drifted)", err)
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
