package detection_test

// This is internal/detection's first test file: real-Postgres coverage of
// DetectMusicBrainz's new_release diff (DTCT-01), matching
// internal/watchlist/service_test.go's established convention -- every
// test shares one database (testutil.NewTestPool), so each derives a
// unique artist mbid from t.Name() rather than a hardcoded literal, and
// registers a t.Cleanup that deletes the rows it created.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testMBID derives a short, unique-per-test artist mbid from t.Name(), the
// same helper convention as internal/watchlist/service_test.go's testMBID.
func testMBID(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

// testLogger is a discard-output *slog.Logger, matching
// internal/testutil.NewTestPool's own logger convention -- these tests
// assert on database rows, not log output, so nothing needs to be
// captured.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// insertTestArtist inserts a minimal artists row directly (this package
// must not depend on internal/watchlist.Service, which is a consumer of
// internal/detection's own seam, not a dependency of it) and registers its
// cleanup.
func insertTestArtist(t *testing.T, pool *pgxpool.Pool, mbid, name string) int64 {
	t.Helper()
	ctx := context.Background()
	var id int64
	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, name).Scan(&id); err != nil {
		t.Fatalf("insert test artist: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE id = $1", id); err != nil {
			t.Fatalf("cleanup: delete artist: %v", err)
		}
	})
	return id
}

func TestDetectMusicBrainz_NewRelease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Test Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Test Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "First Album", PrimaryType: "Album", FirstReleaseDate: "2020-01-01"},
		{MBID: mbid + "-rg2", Title: "Second Album", PrimaryType: "Album", FirstReleaseDate: "2021-06-15"},
	}

	d := detection.New(sqlc.New(pool))
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT external_id, release_group_mbid, title, artist_name,
		release_date, cover_art_url, event_type, source
		FROM events WHERE artist_id = $1 ORDER BY external_id`, artistID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	type row struct {
		externalID, releaseGroupMbid, title, artistName string
		releaseDate, coverArtURL                        *string
		eventType, source                               string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.externalID, &r.releaseGroupMbid, &r.title, &r.artistName,
			&r.releaseDate, &r.coverArtURL, &r.eventType, &r.source); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("event row count = %d, want 2", len(got))
	}

	want := []struct {
		externalID, title, releaseDate string
	}{
		{mbid + "-rg1", "First Album", "2020-01-01"},
		{mbid + "-rg2", "Second Album", "2021-06-15"},
	}
	for i, w := range want {
		if got[i].externalID != w.externalID {
			t.Fatalf("row %d external_id = %q, want %q", i, got[i].externalID, w.externalID)
		}
		if got[i].releaseGroupMbid != w.externalID {
			t.Fatalf("row %d release_group_mbid = %q, want %q", i, got[i].releaseGroupMbid, w.externalID)
		}
		if got[i].title != w.title {
			t.Fatalf("row %d title = %q, want %q", i, got[i].title, w.title)
		}
		if got[i].artistName != "Test Artist" {
			t.Fatalf("row %d artist_name = %q, want %q", i, got[i].artistName, "Test Artist")
		}
		if got[i].releaseDate == nil || *got[i].releaseDate != w.releaseDate {
			t.Fatalf("row %d release_date = %v, want %q", i, got[i].releaseDate, w.releaseDate)
		}
		wantCoverArt := "https://coverartarchive.org/release-group/" + w.externalID + "/front"
		if got[i].coverArtURL == nil || *got[i].coverArtURL != wantCoverArt {
			t.Fatalf("row %d cover_art_url = %v, want %q", i, got[i].coverArtURL, wantCoverArt)
		}
		if got[i].eventType != "new_release" {
			t.Fatalf("row %d event_type = %q, want new_release", i, got[i].eventType)
		}
		if got[i].source != "musicbrainz" {
			t.Fatalf("row %d source = %q, want musicbrainz", i, got[i].source)
		}
	}
}

func TestDetectMusicBrainz_NewRelease_EmptyInput(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Empty Input Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Empty Input Artist"}
	d := detection.New(sqlc.New(pool))

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz with nil groups: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("event row count = %d, want 0", count)
	}
}

func TestDetectMusicBrainz_NewRelease_UndatedGroup(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Undated Group Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Undated Group Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Undated Album", PrimaryType: "Album", FirstReleaseDate: ""},
	}

	d := detection.New(sqlc.New(pool))
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var releaseDateIsNull bool
	if err := pool.QueryRow(ctx, "SELECT release_date IS NULL FROM events WHERE artist_id = $1", artistID).Scan(&releaseDateIsNull); err != nil {
		t.Fatalf("query release_date: %v", err)
	}
	if !releaseDateIsNull {
		t.Fatal("release_date is not NULL for an undated group, want NULL")
	}
}

// The tests below (04-01 task 3) prove the idempotency and overlap-guard
// contracts the tracer above now depends on: DTCT-04's dedup mechanism
// (04-RESEARCH.md Assumption A4, Pitfall #2) and D-19's re-derivation
// recovery model, proven against real Postgres rather than assumed.

func TestInsertEvent_Idempotent(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Idempotent Artist")

	q := sqlc.New(pool)
	params := sqlc.InsertEventParams{
		ArtistID:   artistID,
		Source:     "musicbrainz",
		EventType:  "new_release",
		ExternalID: mbid + "-rg1",
		Title:      "Album",
		ArtistName: "Idempotent Artist",
	}

	first, err := q.InsertEvent(ctx, params)
	if err != nil {
		t.Fatalf("first InsertEvent: %v", err)
	}
	if first != 1 {
		t.Fatalf("first InsertEvent affected rows = %d, want 1", first)
	}

	second, err := q.InsertEvent(ctx, params)
	if err != nil {
		t.Fatalf("second InsertEvent: %v", err)
	}
	if second != 0 {
		t.Fatalf("second InsertEvent affected rows = %d, want 0 (dedup key already existed)", second)
	}
}

func TestInsertEvent_SnapshotIsWriteOnce(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Snapshot Write Once Artist")

	q := sqlc.New(pool)
	externalID := mbid + "-rg1"
	first := sqlc.InsertEventParams{
		ArtistID:    artistID,
		Source:      "musicbrainz",
		EventType:   "new_release",
		ExternalID:  externalID,
		Title:       "Original Title",
		ArtistName:  "Original Artist Name",
		ReleaseDate: nullableStringForTest("2020-01-01"),
		CoverArtUrl: nullableStringForTest("https://example.test/original.jpg"),
	}
	if affected, err := q.InsertEvent(ctx, first); err != nil || affected != 1 {
		t.Fatalf("first InsertEvent: affected=%d err=%v, want affected=1 err=nil", affected, err)
	}

	conflicting := first
	conflicting.Title = "Changed Title"
	conflicting.ArtistName = "Changed Artist Name"
	conflicting.ReleaseDate = nullableStringForTest("2099-12-31")
	conflicting.CoverArtUrl = nullableStringForTest("https://example.test/changed.jpg")
	if affected, err := q.InsertEvent(ctx, conflicting); err != nil || affected != 0 {
		t.Fatalf("conflicting InsertEvent: affected=%d err=%v, want affected=0 err=nil", affected, err)
	}

	var title, artistName, releaseDate, coverArtURL string
	row := pool.QueryRow(ctx, "SELECT title, artist_name, release_date, cover_art_url FROM events WHERE artist_id = $1 AND external_id = $2", artistID, externalID)
	if err := row.Scan(&title, &artistName, &releaseDate, &coverArtURL); err != nil {
		t.Fatalf("query stored snapshot: %v", err)
	}
	if title != "Original Title" {
		t.Fatalf("title = %q, want %q (D-20: snapshot is write-once)", title, "Original Title")
	}
	if artistName != "Original Artist Name" {
		t.Fatalf("artist_name = %q, want %q", artistName, "Original Artist Name")
	}
	if releaseDate != "2020-01-01" {
		t.Fatalf("release_date = %q, want %q", releaseDate, "2020-01-01")
	}
	if coverArtURL != "https://example.test/original.jpg" {
		t.Fatalf("cover_art_url = %q, want %q", coverArtURL, "https://example.test/original.jpg")
	}
}

func TestInsertEvent_SourceSeparatesNamespaces(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Source Namespace Artist")

	q := sqlc.New(pool)
	sharedExternalID := "123456" // plausible as both an MBID-shaped and a Deezer numeric id

	mbParams := sqlc.InsertEventParams{
		ArtistID: artistID, Source: "musicbrainz", EventType: "new_release",
		ExternalID: sharedExternalID, Title: "MB Album", ArtistName: "Source Namespace Artist",
	}
	dzParams := mbParams
	dzParams.Source = "deezer"
	dzParams.Title = "Deezer Album"

	if affected, err := q.InsertEvent(ctx, mbParams); err != nil || affected != 1 {
		t.Fatalf("musicbrainz InsertEvent: affected=%d err=%v, want affected=1 err=nil", affected, err)
	}
	if affected, err := q.InsertEvent(ctx, dzParams); err != nil || affected != 1 {
		t.Fatalf("deezer InsertEvent: affected=%d err=%v, want affected=1 err=nil (a different source is a different dedup key)", affected, err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND external_id = $2", artistID, sharedExternalID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (source must separate the dedup namespace)", count)
	}
}

func TestDetectMusicBrainz_ReDetectionInsertsNothing(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Re-Detection Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Re-Detection Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
	}
	d := detection.New(sqlc.New(pool))

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("first DetectMusicBrainz: %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("second DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (re-detection must insert nothing new)", count)
	}
}

func TestDetectMusicBrainz_PartialCycleResumes(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Partial Cycle Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Partial Cycle Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	all := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		{MBID: mbid + "-rg3", Title: "Album Three", PrimaryType: "Album"},
	}
	d := detection.New(sqlc.New(pool))

	// Simulate a cycle that crashed after writing only the first group
	// (D-19: recovery is re-derivation, not resume state).
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, all[:1]); err != nil {
		t.Fatalf("partial DetectMusicBrainz: %v", err)
	}

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, all); err != nil {
		t.Fatalf("full re-derivation DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 3 {
		t.Fatalf("event row count = %d, want 3 (exactly the two missing rows must appear, no duplicate of the first)", count)
	}

	var dupCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND external_id = $2", artistID, all[0].MBID).Scan(&dupCount); err != nil {
		t.Fatalf("count first group's rows: %v", err)
	}
	if dupCount != 1 {
		t.Fatalf("rows for the first (already-recorded) group = %d, want 1", dupCount)
	}
}

func TestDetectMusicBrainz_InsertionOrderIsStable(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Insertion Order Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Insertion Order Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		{MBID: mbid + "-rg3", Title: "Album Three", PrimaryType: "Album"},
	}
	d := detection.New(sqlc.New(pool))
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	rows, err := pool.Query(ctx, "SELECT external_id FROM events WHERE artist_id = $1 ORDER BY created_at ASC, id ASC", artistID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var externalID string
		if err := rows.Scan(&externalID); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, externalID)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}

	want := []string{groups[0].MBID, groups[1].MBID, groups[2].MBID}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("row %d external_id = %q, want %q (ORDER BY created_at ASC, id ASC must reproduce input order)", i, got[i], want[i])
		}
	}
}

// nullableStringForTest returns a pointer to a freshly allocated copy of s,
// mirroring internal/watchlist/service_test.go's strptr -- so callers never
// take the address of a loop variable or a shared literal by accident.
func nullableStringForTest(s string) *string { return &s }

// The tests below (04-02 task 2) prove D-13 through D-16's seed-mode
// contract: a newly watched artist's existing catalogue is absorbed into
// the seen store as already-notified rather than queued for alerting, later
// cycles leave new events unnotified, seeding is per-source, and a
// watchlist remove-then-re-add resumes rather than re-seeds.

// unnotifiedForArtist filters q.ListUnnotified's global result down to
// artistID's rows -- ListUnnotified has no per-artist parameter (it is
// Phase 5's cross-artist notify-queue query), and every test in this
// package shares one database, so tests run sequentially (never
// t.Parallel) and each cleans up its own artist (and, via ON DELETE
// CASCADE, its own events) before the next test's insert.
func unnotifiedForArtist(t *testing.T, q sqlc.Querier, ctx context.Context, artistID int64) []sqlc.Event {
	t.Helper()
	all, err := q.ListUnnotified(ctx)
	if err != nil {
		t.Fatalf("ListUnnotified: %v", err)
	}
	var mine []sqlc.Event
	for _, e := range all {
		if e.ArtistID == artistID {
			mine = append(mine, e)
		}
	}
	return mine
}

func TestDetector_SeedMode_FirstCyclePreNotifies(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Seed Mode Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Seed Mode Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		{MBID: mbid + "-rg3", Title: "Album Three", PrimaryType: "Album"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	rows, err := pool.Query(ctx, "SELECT notified_at IS NOT NULL FROM events WHERE artist_id = $1", artistID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()
	var count int
	for rows.Next() {
		var notified bool
		if err := rows.Scan(&notified); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if !notified {
			t.Fatal("seed-cycle row has NULL notified_at, want non-NULL")
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if count != 3 {
		t.Fatalf("event row count = %d, want 3", count)
	}

	if got := unnotifiedForArtist(t, q, ctx, artistID); len(got) != 0 {
		t.Fatalf("ListUnnotified returned %d rows for a seeded artist, want 0", len(got))
	}
}

func TestDetector_SecondCycleLeavesNotifiedAtNull(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Second Cycle Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Second Cycle Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	seedGroups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		{MBID: mbid + "-rg3", Title: "Album Three", PrimaryType: "Album"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, seedGroups); err != nil {
		t.Fatalf("seed DetectMusicBrainz: %v", err)
	}

	nextGroups := append(append([]musicbrainz.ReleaseGroup{}, seedGroups...),
		musicbrainz.ReleaseGroup{MBID: mbid + "-rg4", Title: "Album Four", PrimaryType: "Album"})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nextGroups); err != nil {
		t.Fatalf("second DetectMusicBrainz: %v", err)
	}

	var notifiedCount, unnotifiedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND notified_at IS NOT NULL", artistID).Scan(&notifiedCount); err != nil {
		t.Fatalf("count notified: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND notified_at IS NULL", artistID).Scan(&unnotifiedCount); err != nil {
		t.Fatalf("count unnotified: %v", err)
	}
	if notifiedCount != 3 {
		t.Fatalf("notified row count = %d, want 3 (the seeded rows must stay notified)", notifiedCount)
	}
	if unnotifiedCount != 1 {
		t.Fatalf("unnotified row count = %d, want 1 (only the new group)", unnotifiedCount)
	}

	got := unnotifiedForArtist(t, q, ctx, artistID)
	if len(got) != 1 {
		t.Fatalf("ListUnnotified returned %d rows for this artist, want 1", len(got))
	}
	if got[0].ExternalID != mbid+"-rg4" {
		t.Fatalf("unnotified row external_id = %q, want %q", got[0].ExternalID, mbid+"-rg4")
	}
}

func TestDetector_SeedModeIsPerSource(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Per Source Seed Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Per Source Seed Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	hasMB, err := q.HasAnyEvent(ctx, sqlc.HasAnyEventParams{ArtistID: artistID, Source: "musicbrainz"})
	if err != nil {
		t.Fatalf("HasAnyEvent(musicbrainz): %v", err)
	}
	if !hasMB {
		t.Fatal("HasAnyEvent(musicbrainz) = false, want true after a musicbrainz cycle")
	}

	hasDZ, err := q.HasAnyEvent(ctx, sqlc.HasAnyEventParams{ArtistID: artistID, Source: "deezer"})
	if err != nil {
		t.Fatalf("HasAnyEvent(deezer): %v", err)
	}
	if hasDZ {
		t.Fatal("HasAnyEvent(deezer) = true, want false -- a musicbrainz-only cycle must not seed the deezer source (D-15)")
	}
}

func TestDetector_ReAddDoesNotReSeed(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Re-Add Artist")

	var watchlistID int64
	if err := pool.QueryRow(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1) RETURNING id", artistID).Scan(&watchlistID); err != nil {
		t.Fatalf("insert watchlist row: %v", err)
	}

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Re-Add Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	seedGroups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, seedGroups); err != nil {
		t.Fatalf("seed DetectMusicBrainz: %v", err)
	}

	// Remove then re-add the watchlist row -- event rows are keyed on
	// artist_id (Phase 2 D-03 master data), which survives watchlist-row
	// deletion, so this must not put the artist back into seed mode (D-16).
	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", watchlistID); err != nil {
		t.Fatalf("delete watchlist row: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1)", artistID); err != nil {
		t.Fatalf("re-insert watchlist row: %v", err)
	}

	nextGroups := append(append([]musicbrainz.ReleaseGroup{}, seedGroups...),
		musicbrainz.ReleaseGroup{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nextGroups); err != nil {
		t.Fatalf("post-re-add DetectMusicBrainz: %v", err)
	}

	var notifiedAt *string
	if err := pool.QueryRow(ctx, "SELECT notified_at::text FROM events WHERE artist_id = $1 AND external_id = $2", artistID, mbid+"-rg2").Scan(&notifiedAt); err != nil {
		t.Fatalf("query new row notified_at: %v", err)
	}
	if notifiedAt != nil {
		t.Fatalf("notified_at for the post-re-add row = %v, want NULL (a re-add must not re-seed)", *notifiedAt)
	}
}

func TestDetector_SeedModeRespectsFilters(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Seed Filter Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Seed Filter Artist", ReleaseTypes: []string{"album"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Single One", PrimaryType: "Single"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event row count = %d, want 1 (filtering applies in seed mode too, D-17)", count)
	}
}

func TestDetector_SeedRowsShareOneTimestamp(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Seed Timestamp Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Seed Timestamp Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		{MBID: mbid + "-rg3", Title: "Album Three", PrimaryType: "Album"},
	}

	q := sqlc.New(pool)
	d := detection.New(q)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	rows, err := pool.Query(ctx, "SELECT DISTINCT notified_at FROM events WHERE artist_id = $1", artistID)
	if err != nil {
		t.Fatalf("query distinct notified_at: %v", err)
	}
	defer rows.Close()
	var distinctCount int
	for rows.Next() {
		distinctCount++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows.Err: %v", err)
	}
	if distinctCount != 1 {
		t.Fatalf("distinct notified_at values = %d, want 1 (every row from one seed cycle must share one timestamp)", distinctCount)
	}
}
