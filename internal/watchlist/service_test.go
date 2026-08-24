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
	"slices"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
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

// strptr returns a pointer to a freshly allocated copy of s, so callers never
// take the address of a loop variable or a shared literal by accident.
func strptr(s string) *string { return &s }

// TestService_Add_RefreshesArtistMetadataOnReAdd pins WR-01/G-02-2a: a re-add
// carrying a changed disambiguation or image_url must update the stored
// artists row, and the Entry Service.Add returns must reflect the new
// values -- not the stale ones UpsertArtist's ON CONFLICT clause used to
// silently keep. This is the "a supplied value refreshes" half of the
// COALESCE contract task 2 completes with the "an omitted value preserves"
// half.
func TestService_Add_RefreshesArtistMetadataOnReAdd(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	if _, err := svc.Add(ctx, watchlist.AddParams{
		MBID:           mbid,
		Name:           "First Add",
		Disambiguation: strptr("US rapper"),
		ImageURL:       strptr("https://example.test/old.jpg"),
	}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE artist_id = (SELECT id FROM artists WHERE mbid = $1)", mbid); err != nil {
		t.Fatalf("delete watchlist row: %v", err)
	}

	entry, err := svc.Add(ctx, watchlist.AddParams{
		MBID:           mbid,
		Name:           "Second Add",
		Disambiguation: strptr("Chicago rapper"),
		ImageURL:       strptr("https://example.test/new.jpg"),
	})
	if err != nil {
		t.Fatalf("second Add (re-adding after watchlist row removed): %v", err)
	}

	if entry.Disambiguation == nil || *entry.Disambiguation != "Chicago rapper" {
		t.Fatalf("returned Entry.Disambiguation = %v, want %q", entry.Disambiguation, "Chicago rapper")
	}
	if entry.ImageURL == nil || *entry.ImageURL != "https://example.test/new.jpg" {
		t.Fatalf("returned Entry.ImageURL = %v, want %q", entry.ImageURL, "https://example.test/new.jpg")
	}

	var disambiguation, imageURL *string
	row := pool.QueryRow(ctx, "SELECT disambiguation, image_url FROM artists WHERE mbid = $1", mbid)
	if err := row.Scan(&disambiguation, &imageURL); err != nil {
		t.Fatalf("query stored artist metadata: %v", err)
	}
	if disambiguation == nil || *disambiguation != "Chicago rapper" {
		t.Fatalf("stored disambiguation = %v, want %q", disambiguation, "Chicago rapper")
	}
	if imageURL == nil || *imageURL != "https://example.test/new.jpg" {
		t.Fatalf("stored image_url = %v, want %q", imageURL, "https://example.test/new.jpg")
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artists WHERE mbid = $1", mbid).Scan(&count); err != nil {
		t.Fatalf("query artists count: %v", err)
	}
	if count != 1 {
		t.Fatalf("artists row count = %d, want 1 (refreshing metadata must not fork the master row, D-03)", count)
	}
}

// TestService_Add_OmittedMetadataSurvivesReAdd guards the other half of the
// COALESCE contract task 1 established: nil on an optional pointer field
// means "I am not saying anything about this field", not "clear it". This
// test is expected to pass on first run -- it drives no new production code
// -- its value is directional, failing the moment someone "simplifies" the
// three COALESCE wrappers in queries/artists.sql into bare EXCLUDED
// assignments, which would turn every metadata-omitting re-add into a
// silent blanking of stored data. Add accepts partial metadata by design, so
// that regression would fire on ordinary traffic, not as an edge case.
func TestService_Add_OmittedMetadataSurvivesReAdd(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))

	if _, err := svc.Add(ctx, watchlist.AddParams{
		MBID:           mbid,
		Name:           "First Add",
		DeezerID:       strptr("12345"),
		Disambiguation: strptr("Atlanta trio"),
		ImageURL:       strptr("https://example.test/keep.jpg"),
	}); err != nil {
		t.Fatalf("first Add: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE artist_id = (SELECT id FROM artists WHERE mbid = $1)", mbid); err != nil {
		t.Fatalf("delete watchlist row: %v", err)
	}

	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Second Add"})
	if err != nil {
		t.Fatalf("second Add (re-adding with all optional metadata omitted): %v", err)
	}

	var deezerID, disambiguation, imageURL *string
	row := pool.QueryRow(ctx, "SELECT deezer_id, disambiguation, image_url FROM artists WHERE mbid = $1", mbid)
	if err := row.Scan(&deezerID, &disambiguation, &imageURL); err != nil {
		t.Fatalf("query stored artist metadata: %v", err)
	}
	if deezerID == nil || *deezerID != "12345" {
		t.Fatalf("stored deezer_id = %v, want %q (omitted field must not blank it)", deezerID, "12345")
	}
	if disambiguation == nil || *disambiguation != "Atlanta trio" {
		t.Fatalf("stored disambiguation = %v, want %q (omitted field must not blank it)", disambiguation, "Atlanta trio")
	}
	if imageURL == nil || *imageURL != "https://example.test/keep.jpg" {
		t.Fatalf("stored image_url = %v, want %q (omitted field must not blank it)", imageURL, "https://example.test/keep.jpg")
	}

	if entry.DeezerID == nil || *entry.DeezerID != "12345" {
		t.Fatalf("returned Entry.DeezerID = %v, want %q", entry.DeezerID, "12345")
	}
	if entry.Disambiguation == nil || *entry.Disambiguation != "Atlanta trio" {
		t.Fatalf("returned Entry.Disambiguation = %v, want %q", entry.Disambiguation, "Atlanta trio")
	}
	if entry.ImageURL == nil || *entry.ImageURL != "https://example.test/keep.jpg" {
		t.Fatalf("returned Entry.ImageURL = %v, want %q", entry.ImageURL, "https://example.test/keep.jpg")
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

// TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost pins G-02-2b /
// WR-02's lost-update failure mode: a PATCH to one axis must never revert a
// concurrently committed write to the other axis. The interleaving is forced
// deterministically with a held row lock (an uncommitted transaction), not a
// race between two goroutines hoping to interleave -- so this test either
// proves the property or it doesn't, on every run.
//
// Why this is red before the single-statement rewrite: the old
// UpdatePreferences reads the current row via an unlocked ListWatchlist
// SELECT, so it observes the pre-commit muted_event_types (empty) and later
// writes that captured empty value back over the committed
// {deluxe_change} -- silently erasing it. Step 6's mute assertion is the
// failure.
func TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Concurrent Axis Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, "UPDATE watchlist SET muted_event_types = ARRAY['deluxe_change']::text[] WHERE id = $1", entry.ID); err != nil {
		t.Fatalf("held-lock update: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
			ReleaseTypes: &[]string{"single"},
		})
		errCh <- err
	}()

	select {
	case <-errCh:
		t.Fatal("UpdatePreferences completed without ever waiting for the row lock held by the uncommitted transaction")
	case <-time.After(300 * time.Millisecond):
		// Expected: the goroutine is blocked behind the row lock.
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	updateErr := <-errCh
	if updateErr != nil {
		t.Fatalf("UpdatePreferences: %v", updateErr)
	}

	var releaseTypes, mutedEventTypes []string
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
		t.Fatalf("query stored preferences: %v", err)
	}
	if len(releaseTypes) != 1 || releaseTypes[0] != "single" {
		t.Fatalf("release_types = %v, want [single]", releaseTypes)
	}
	if len(mutedEventTypes) != 1 || mutedEventTypes[0] != "deluxe_change" {
		t.Fatalf("muted_event_types = %v, want [deluxe_change] (the committed value from the concurrent transaction must survive)", mutedEventTypes)
	}
}

// TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound pins
// G-02-2b / WR-02's other failure mode: a PATCH blocked behind a concurrent
// DELETE of the same row must resolve to watchlist.ErrNotFound, not a wrapped
// driver error. Like its sibling above, the interleaving is forced
// deterministically with a held row lock.
//
// Why this is red before the single-statement rewrite: the old
// implementation's UPDATE returns pgx.ErrNoRows once the delete commits, and
// that error is wrapped with fmt.Errorf("update watchlist preferences: %w",
// err) without ever being translated to ErrNotFound -- which is exactly what
// makes the handler answer 500 where it should answer 404.
func TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Deleted Mid Write Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err := tx.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", entry.ID); err != nil {
		t.Fatalf("held-lock delete: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{
			ReleaseTypes: &[]string{"album"},
		})
		errCh <- err
	}()

	select {
	case <-errCh:
		t.Fatal("UpdatePreferences completed without ever waiting for the row lock held by the uncommitted transaction")
	case <-time.After(300 * time.Millisecond):
		// Expected: the goroutine is blocked behind the row lock.
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}

	updateErr := <-errCh
	if !errors.Is(updateErr, watchlist.ErrNotFound) {
		t.Fatalf("err = %v, want errors.Is(err, watchlist.ErrNotFound)", updateErr)
	}

	var pgErr *pgconn.PgError
	if errors.As(updateErr, &pgErr) {
		t.Fatalf("err %v exposes a raw *pgconn.PgError to the caller", updateErr)
	}
	if errors.Is(updateErr, pgx.ErrNoRows) {
		t.Fatalf("err = %v wraps pgx.ErrNoRows directly instead of the translated watchlist.ErrNotFound sentinel", updateErr)
	}
}

// TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied
// pins WR-01 (G-02-1): a call to UpdatePreferences supplying neither axis
// must be rejected by the domain itself, before any database write, not
// merely by the HTTP handler one layer above it. The updated_at assertion
// below is what carries the finding -- it is the observable proof that no
// write reached the database at all, not merely that the response looked
// like a rejection.
//
// Why this is red today: the call returns (Entry, nil) and the statement's
// updated_at = now() fires with both CASE branches resolving to the
// column's own value.
func TestService_UpdatePreferences_NeitherAxisReturnsErrNoPreferencesSupplied(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	svc := watchlist.NewService(sqlc.New(pool))
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Neither Axis Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	var beforeReleaseTypes, beforeMutedEventTypes []string
	var beforeUpdatedAt time.Time
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types, updated_at FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&beforeReleaseTypes, &beforeMutedEventTypes, &beforeUpdatedAt); err != nil {
		t.Fatalf("query preferences before update: %v", err)
	}

	got, err := svc.UpdatePreferences(ctx, entry.ID, watchlist.PreferencesParams{})
	if !errors.Is(err, watchlist.ErrNoPreferencesSupplied) {
		t.Fatalf("err = %v, want errors.Is(err, watchlist.ErrNoPreferencesSupplied)", err)
	}
	if !reflect.DeepEqual(got, watchlist.Entry{}) {
		t.Fatalf("returned Entry = %+v, want the zero value", got)
	}

	var afterReleaseTypes, afterMutedEventTypes []string
	var afterUpdatedAt time.Time
	row = pool.QueryRow(ctx, "SELECT release_types, muted_event_types, updated_at FROM watchlist WHERE id = $1", entry.ID)
	if err := row.Scan(&afterReleaseTypes, &afterMutedEventTypes, &afterUpdatedAt); err != nil {
		t.Fatalf("query preferences after update: %v", err)
	}
	if !reflect.DeepEqual(afterReleaseTypes, beforeReleaseTypes) {
		t.Fatalf("release_types = %v, want unchanged %v (a rejected empty update must issue no write)", afterReleaseTypes, beforeReleaseTypes)
	}
	if !reflect.DeepEqual(afterMutedEventTypes, beforeMutedEventTypes) {
		t.Fatalf("muted_event_types = %v, want unchanged %v (a rejected empty update must issue no write)", afterMutedEventTypes, beforeMutedEventTypes)
	}
	if !afterUpdatedAt.Equal(beforeUpdatedAt) {
		t.Fatalf("updated_at = %v, want unchanged %v -- a rejected empty update must leave no write behind at all, not merely fail to change the returned entry", afterUpdatedAt, beforeUpdatedAt)
	}
}

// TestService_UpdatePreferences_NeitherAxisOutranksUnknownID pins the
// guard's position as UpdatePreferences' first statement, ahead of the id
// lookup: an empty update against an id no row holds must report
// ErrNoPreferencesSupplied, not ErrNotFound. Without this, a later refactor
// could satisfy the sibling test above while quietly making the rejection
// depend on the row existing.
//
// Why this is red today: the statement runs, matches zero rows, and the
// method translates pgx.ErrNoRows to ErrNotFound.
func TestService_UpdatePreferences_NeitherAxisOutranksUnknownID(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	var unknownID int64
	row := pool.QueryRow(ctx, "SELECT COALESCE(MAX(id), 0) + 1 FROM watchlist")
	if err := row.Scan(&unknownID); err != nil {
		t.Fatalf("query an id no row holds: %v", err)
	}

	_, err := svc.UpdatePreferences(ctx, unknownID, watchlist.PreferencesParams{})
	if !errors.Is(err, watchlist.ErrNoPreferencesSupplied) {
		t.Fatalf("err = %v, want errors.Is(err, watchlist.ErrNoPreferencesSupplied)", err)
	}
	if errors.Is(err, watchlist.ErrNotFound) {
		t.Fatalf("err = %v also matches watchlist.ErrNotFound -- the neither-axis guard must outrank the id lookup, not merely coincide with it", err)
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

// The two tests below cover Phase 13's ListArtistsMissingImage (D-06/D-07,
// D-12 cooldown amended by grilling round Q4) and RecordArtMatchAttempt
// (D-12), the sqlc queries the artist-art backfill sweep (sibling plan
// 13-03) depends on. testutil.NewTestPool resets schema, not table
// contents (TestService_List_EmptyReturnsNonNilSlice's own precedent), so
// both tests scope every assertion to the ids they themselves seeded.

// insertArtistForArtTest inserts a bare artists row with the given mbid,
// image_url and art_match_attempted_at (attemptedAgo == 0 means NULL --
// never attempted), returning its id.
func insertArtistForArtTest(t *testing.T, pool *pgxpool.Pool, mbid string, imageURL *string, attemptedAgo time.Duration) int64 {
	t.Helper()
	ctx := context.Background()
	var artistID int64
	if attemptedAgo == 0 {
		if err := pool.QueryRow(ctx,
			"INSERT INTO artists (mbid, name, image_url) VALUES ($1, $2, $3) RETURNING id",
			mbid, "Art Test "+mbid, imageURL,
		).Scan(&artistID); err != nil {
			t.Fatalf("insert artist %s: %v", mbid, err)
		}
		return artistID
	}
	if err := pool.QueryRow(ctx,
		"INSERT INTO artists (mbid, name, image_url, art_match_attempted_at) VALUES ($1, $2, $3, now() - $4::interval) RETURNING id",
		mbid, "Art Test "+mbid, imageURL, attemptedAgo.String(),
	).Scan(&artistID); err != nil {
		t.Fatalf("insert artist %s: %v", mbid, err)
	}
	return artistID
}

func TestListArtistsMissingImage_CooldownAndScope(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	base := testMBID(t)

	mbidNeverAttempted1 := base + "-never-1"
	mbidNeverAttempted2 := base + "-never-2"
	mbidStaleAttempt := base + "-stale"
	mbidWithinCooldown := base + "-cooldown"
	mbidHasImage := base + "-has-image"
	mbidNotWatchlisted := base + "-not-watchlisted"
	allMBIDs := []string{mbidNeverAttempted1, mbidNeverAttempted2, mbidStaleAttempt, mbidWithinCooldown, mbidHasImage, mbidNotWatchlisted}
	t.Cleanup(func() {
		for _, m := range allMBIDs {
			if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", m); err != nil {
				t.Fatalf("cleanup: delete artists row %s: %v", m, err)
			}
		}
	})

	idNever1 := insertArtistForArtTest(t, pool, mbidNeverAttempted1, nil, 0)
	idNever2 := insertArtistForArtTest(t, pool, mbidNeverAttempted2, nil, 0)
	idStale := insertArtistForArtTest(t, pool, mbidStaleAttempt, nil, 25*time.Hour)
	idCooldown := insertArtistForArtTest(t, pool, mbidWithinCooldown, nil, 1*time.Hour)
	hasImage := "https://example.test/has-image.jpg"
	idHasImage := insertArtistForArtTest(t, pool, mbidHasImage, &hasImage, 0)
	idNotWatchlisted := insertArtistForArtTest(t, pool, mbidNotWatchlisted, nil, 0)

	for _, id := range []int64{idNever1, idNever2, idStale, idCooldown, idHasImage} {
		if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1)", id); err != nil {
			t.Fatalf("insert watchlist row for artist %d: %v", id, err)
		}
	}
	// idNotWatchlisted deliberately gets no watchlist row.

	got, err := q.ListArtistsMissingImage(ctx)
	if err != nil {
		t.Fatalf("ListArtistsMissingImage: %v", err)
	}

	var gotIDs []int64
	for _, a := range got {
		switch a.ID {
		case idNever1, idNever2, idStale, idCooldown, idHasImage, idNotWatchlisted:
			gotIDs = append(gotIDs, a.ID)
		}
	}

	wantIDs := []int64{idNever1, idNever2, idStale}
	slices.Sort(wantIDs)
	slices.Sort(gotIDs)
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("scoped result ids = %v, want %v (never-attempted x2 plus the 25h-stale row, excluding within-cooldown/has-image/not-watchlisted)", gotIDs, wantIDs)
	}

	if !slices.IsSorted(artistIDs(got)) {
		t.Fatalf("ListArtistsMissingImage result is not ascending by id")
	}
}

// artistIDs extracts the id column from a []sqlc.Artist slice for an
// ascending-order assertion.
func artistIDs(artists []sqlc.Artist) []int64 {
	ids := make([]int64, len(artists))
	for i, a := range artists {
		ids[i] = a.ID
	}
	return ids
}

func TestRecordArtMatchAttempt_SetsTimestampLeavesImageUntouched(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	q := sqlc.New(pool)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	wantImage := "https://example.test/record-attempt.jpg"
	insertArtistForArtTest(t, pool, mbid, &wantImage, 0)

	if err := q.RecordArtMatchAttempt(ctx, mbid); err != nil {
		t.Fatalf("RecordArtMatchAttempt: %v", err)
	}

	var attemptedAt pgtype.Timestamptz
	var imageURL *string
	row := pool.QueryRow(ctx, "SELECT art_match_attempted_at, image_url FROM artists WHERE mbid = $1", mbid)
	if err := row.Scan(&attemptedAt, &imageURL); err != nil {
		t.Fatalf("query stored artist: %v", err)
	}
	if !attemptedAt.Valid {
		t.Fatalf("art_match_attempted_at = NULL, want non-NULL")
	}
	if time.Since(attemptedAt.Time) > time.Minute {
		t.Fatalf("art_match_attempted_at = %v, want approximately now", attemptedAt.Time)
	}
	if imageURL == nil || *imageURL != wantImage {
		t.Fatalf("image_url = %v, want unchanged %q", imageURL, wantImage)
	}
}
