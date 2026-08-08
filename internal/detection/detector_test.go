package detection_test

// This is internal/detection's first test file: real-Postgres coverage of
// DetectMusicBrainz's new_release diff (DTCT-01), matching
// internal/watchlist/service_test.go's established convention -- every
// test shares one database (testutil.NewTestPool), so each derives a
// unique artist mbid from t.Name() rather than a hardcoded literal, and
// registers a t.Cleanup that deletes the rows it created.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeRecordingSource is a controllable double for detection.RecordingSource,
// shared by every test in this package (and deezer_test.go, same package)
// that constructs a Detector -- the zero value returns no recordings and no
// error, a no-op for guest-feature detection, which is what every
// new_release-focused test in this file needs.
type fakeRecordingSource struct {
	recordings []musicbrainz.Recording
	err        error
}

func (f fakeRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	return f.recordings, f.err
}

// mkCredit builds an ArtistCreditEntry field-by-field rather than as a
// composite literal of ArtistCreditEntry.Artist's anonymous nested struct
// type -- avoids duplicating that type's exact field/tag shape at every call
// site.
func mkCredit(mbid, name string) musicbrainz.ArtistCreditEntry {
	var e musicbrainz.ArtistCreditEntry
	e.Name = name
	e.Artist.MBID = mbid
	e.Artist.Name = name
	return e
}

// fakeReleaseDetailSource is a controllable double for
// detection.ReleaseDetailSource, shared by every deluxe-change test in this
// file (and by other files in this package/module that only need a no-op).
// callCount lets a test assert "zero calls" precisely -- checking only the
// resulting row count would pass even if the fetch had been issued and its
// result discarded, which is exactly the request-volume waste the
// preference gates (deluxeDetectionEnabled/eventTypeMuted) exist to
// prevent. errByGroup lets a test make one group's fetch fail without
// affecting any other group (per-group error isolation).
type fakeReleaseDetailSource struct {
	mu         sync.Mutex
	callCount  int
	releases   map[string][]musicbrainz.Release
	errByGroup map[string]error
}

func (f *fakeReleaseDetailSource) ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]musicbrainz.Release, error) {
	f.mu.Lock()
	f.callCount++
	f.mu.Unlock()
	if f.errByGroup != nil {
		if err, ok := f.errByGroup[groupMBID]; ok {
			return nil, err
		}
	}
	if f.releases == nil {
		return nil, nil
	}
	return f.releases[groupMBID], nil
}

func (f *fakeReleaseDetailSource) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callCount
}

func (f *fakeReleaseDetailSource) setReleases(groupMBID string, releases []musicbrainz.Release) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.releases == nil {
		f.releases = map[string][]musicbrainz.Release{}
	}
	f.releases[groupMBID] = releases
}

// mkRelease builds a musicbrainz.Release with one medium per entry in
// trackCounts -- e.g. mkRelease(mbid, "Title", "2020-01-01", 12, 9) builds a
// two-disc release totalling 21 tracks.
func mkRelease(mbid, title, date string, trackCounts ...int) musicbrainz.Release {
	media := make([]musicbrainz.Medium, len(trackCounts))
	for i, tc := range trackCounts {
		media[i] = musicbrainz.Medium{Format: "CD", Position: i + 1, TrackCount: tc}
	}
	return musicbrainz.Release{MBID: mbid, Title: title, Status: "Official", Date: date, Media: media}
}

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

	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})

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

	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})

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
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})

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
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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
	d := detection.New(q, fakeRecordingSource{}, &fakeReleaseDetailSource{})
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

// The tests below (04-03 task 1) prove DTCT-03's guest-feature slice
// end-to-end: a recording where the watched artist is not the first
// artist-credit entry becomes a guest_feature event; a recording where they
// ARE the first entry (their own primary-credit catalogue) never does.

func TestDetectMusicBrainz_GuestFeature(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Guest Feature Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Guest Feature Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	recordingMBID := mbid + "-rec1"
	recordings := []musicbrainz.Recording{
		{
			MBID:  recordingMBID,
			Title: "Feature Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("primary-mbid-0000", "Primary Artist"),
				mkCredit(mbid, "Guest Feature Artist"),
			},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var eventType, source, externalID, title, artistName string
	var releaseGroupMbid, releaseDate, coverArtURL *string
	row := pool.QueryRow(ctx, `SELECT event_type, source, external_id, release_group_mbid, release_date, cover_art_url, title, artist_name
		FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'`, artistID)
	if err := row.Scan(&eventType, &source, &externalID, &releaseGroupMbid, &releaseDate, &coverArtURL, &title, &artistName); err != nil {
		t.Fatalf("query guest_feature row: %v", err)
	}
	if eventType != "guest_feature" {
		t.Errorf("event_type = %q, want %q", eventType, "guest_feature")
	}
	if source != "musicbrainz" {
		t.Errorf("source = %q, want %q", source, "musicbrainz")
	}
	if externalID != recordingMBID {
		t.Errorf("external_id = %q, want %q", externalID, recordingMBID)
	}
	if releaseGroupMbid != nil {
		t.Errorf("release_group_mbid = %v, want NULL", *releaseGroupMbid)
	}
	if releaseDate != nil {
		t.Errorf("release_date = %v, want NULL", *releaseDate)
	}
	if coverArtURL != nil {
		t.Errorf("cover_art_url = %v, want NULL", *coverArtURL)
	}
	if title != "Feature Track" {
		t.Errorf("title = %q, want %q", title, "Feature Track")
	}
	if artistName != "Primary Artist" {
		t.Errorf("artist_name = %q, want %q (the primary credit's artist, not the watched artist)", artistName, "Primary Artist")
	}
}

func TestDetectMusicBrainz_GuestFeature_SkipsOwnPrimaryCredit(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Own Primary Credit Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Own Primary Credit Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	recordings := []musicbrainz.Recording{
		{
			MBID:         mbid + "-rec1",
			Title:        "Own Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{mkCredit(mbid, "Own Primary Credit Artist")},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if count != 0 {
		t.Fatalf("guest_feature event row count = %d, want 0 (the watched artist's own primary-credit catalogue must never become a guest_feature event)", count)
	}
}

// The tests below (04-03 task 2) harden the guest pass against
// over-detection, truncation blindness, and malformed credits.

func TestDetectMusicBrainz_GuestFeature_LogsTruncation(t *testing.T) {
	t.Run("below ceiling", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := testMBID(t)
		artistID := insertTestArtist(t, pool, mbid, "Below Ceiling Artist")
		entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Below Ceiling Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

		buf := &bytes.Buffer{}
		logger := slog.New(slog.NewJSONHandler(buf, nil))
		recordings := []musicbrainz.Recording{
			{
				MBID:  mbid + "-rec1",
				Title: "Track",
				ArtistCredit: []musicbrainz.ArtistCreditEntry{
					mkCredit("other-mbid", "Other Artist"),
					mkCredit(mbid, "Below Ceiling Artist"),
				},
			},
		}

		d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
		if err := d.DetectMusicBrainz(ctx, logger, entry, nil); err != nil {
			t.Fatalf("DetectMusicBrainz: %v", err)
		}

		if reached := decodeGuestFeaturePageCeiling(t, buf); reached {
			t.Fatal("page_ceiling_reached = true, want false")
		}
	})

	t.Run("at ceiling", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := testMBID(t)
		artistID := insertTestArtist(t, pool, mbid, "At Ceiling Artist")
		entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "At Ceiling Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

		buf := &bytes.Buffer{}
		logger := slog.New(slog.NewJSONHandler(buf, nil))
		full := make([]musicbrainz.Recording, musicbrainz.MaxRecordingBrowseItems)
		for i := range full {
			full[i] = musicbrainz.Recording{
				MBID:  fmt.Sprintf("%s-rec%d", mbid, i),
				Title: fmt.Sprintf("Track %d", i),
				ArtistCredit: []musicbrainz.ArtistCreditEntry{
					mkCredit("other-mbid", "Other Artist"),
					mkCredit(mbid, "At Ceiling Artist"),
				},
			}
		}

		d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: full}, &fakeReleaseDetailSource{})
		if err := d.DetectMusicBrainz(ctx, logger, entry, nil); err != nil {
			t.Fatalf("DetectMusicBrainz: %v", err)
		}

		if reached := decodeGuestFeaturePageCeiling(t, buf); !reached {
			t.Fatal("page_ceiling_reached = false, want true")
		}
	})
}

// decodeGuestFeaturePageCeiling parses every JSON log line in buf and
// returns the page_ceiling_reached attribute from the guest_feature
// "detection result" record.
func decodeGuestFeaturePageCeiling(t *testing.T, buf *bytes.Buffer) bool {
	t.Helper()
	var reached bool
	found := false
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if rec["event_type"] == "guest_feature" {
			if v, ok := rec["page_ceiling_reached"].(bool); ok {
				reached = v
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no guest_feature detection result log record found")
	}
	return reached
}

func TestDetectMusicBrainz_GuestFeature_DedupesRepeatedMBID(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Dedup Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Dedup Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

	recordingMBID := mbid + "-rec1"
	rec := musicbrainz.Recording{
		MBID:  recordingMBID,
		Title: "Repeated Track",
		ArtistCredit: []musicbrainz.ArtistCreditEntry{
			mkCredit("other-mbid", "Other Artist"),
			mkCredit(mbid, "Dedup Artist"),
		},
	}
	recordings := []musicbrainz.Recording{rec, rec}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if count != 1 {
		t.Fatalf("guest_feature event row count = %d, want 1 (the same recording MBID returned twice within one browse result must dedup to one row)", count)
	}
}

func TestDetectMusicBrainz_GuestFeature_Muted(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Muted Guest Artist")
	entry := watchlist.Entry{
		ArtistID:        artistID,
		MBID:            mbid,
		Name:            "Muted Guest Artist",
		ReleaseTypes:    []string{"album", "single", "ep", "deluxe"},
		MutedEventTypes: []string{"guest_feature"},
	}
	groups := []musicbrainz.ReleaseGroup{{MBID: mbid + "-rg1", Title: "Album", PrimaryType: "Album"}}
	recordings := []musicbrainz.Recording{
		{
			MBID:  mbid + "-rec1",
			Title: "Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("other-mbid", "Other Artist"),
				mkCredit(mbid, "Muted Guest Artist"),
			},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var guestCount, newReleaseCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&guestCount); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release'", artistID).Scan(&newReleaseCount); err != nil {
		t.Fatalf("count new_release events: %v", err)
	}
	if guestCount != 0 {
		t.Fatalf("guest_feature event row count = %d, want 0 (muted)", guestCount)
	}
	if newReleaseCount != 1 {
		t.Fatalf("new_release event row count = %d, want 1 (mute is per-event-type; new_release must still land)", newReleaseCount)
	}
}

func TestDetectMusicBrainz_GuestFeature_SourceErrorPreservesNewReleases(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Source Error Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Source Error Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groups := []musicbrainz.ReleaseGroup{{MBID: mbid + "-rg1", Title: "Album", PrimaryType: "Album"}}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{err: errors.New("recording browse failed")}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v, want nil (a recording-source error must not fail the cycle)", err)
	}

	var newReleaseCount, guestCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release'", artistID).Scan(&newReleaseCount); err != nil {
		t.Fatalf("count new_release events: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&guestCount); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if newReleaseCount != 1 {
		t.Fatalf("new_release event row count = %d, want 1 (must survive a recording-source error)", newReleaseCount)
	}
	if guestCount != 0 {
		t.Fatalf("guest_feature event row count = %d, want 0", guestCount)
	}
}

// The tests below (04-04 task 1) prove DTCT-02's deluxe/tracklist-change
// slice end-to-end: for a release-group already in the seen store, a
// per-release track-count fetch is compared against a persisted baseline,
// with the first-ever measurement establishing that baseline silently
// rather than firing a false alarm (04-RESEARCH.md Pitfall #1).

func TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Baseline Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Baseline Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})

	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	// Cycle 1: discovers the group -- new_release only, no release-detail
	// fetch this cycle (D-04).
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 0 {
		t.Fatalf("cycle 1 release-detail calls = %d, want 0 (D-04: a brand-new group must not be fetched)", got)
	}

	// Cycle 2: the group is now already-seen -- the fetch happens, and this
	// is the group's first-ever track-count measurement, so it must
	// silently establish the baseline rather than firing an event.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 1 {
		t.Fatalf("cycle 2 release-detail calls = %d, want 1", got)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change event row count = %d, want 0 (the first comparison must establish a baseline silently)", deluxeCount)
	}

	var trackCount *int32
	if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
		t.Fatalf("query baseline track_count: %v", err)
	}
	if trackCount == nil || *trackCount != 12 {
		t.Fatalf("baseline track_count = %v, want 12", trackCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Increase Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Increase Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-orig", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (establish baseline at 12): %v", err)
	}

	deluxeMBID := groupMBID + "-deluxe"
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(deluxeMBID, "Album (Deluxe)", "2020-06-01", 18)})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 3 (fires on increase to 18): %v", err)
	}

	var eventType, source, externalID, releaseGroupMbid, title string
	var trackCount int32
	row := pool.QueryRow(ctx, `SELECT event_type, source, external_id, release_group_mbid, title, track_count
		FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'`, artistID)
	if err := row.Scan(&eventType, &source, &externalID, &releaseGroupMbid, &title, &trackCount); err != nil {
		t.Fatalf("query deluxe_change row: %v", err)
	}
	if eventType != "deluxe_change" {
		t.Errorf("event_type = %q, want %q", eventType, "deluxe_change")
	}
	if source != "musicbrainz" {
		t.Errorf("source = %q, want %q", source, "musicbrainz")
	}
	if externalID != deluxeMBID {
		t.Errorf("external_id = %q, want %q (the winning release's own MBID, D-10)", externalID, deluxeMBID)
	}
	if releaseGroupMbid != groupMBID {
		t.Errorf("release_group_mbid = %q, want %q (the parent group)", releaseGroupMbid, groupMBID)
	}
	if title != "Album (Deluxe)" {
		t.Errorf("title = %q, want %q", title, "Album (Deluxe)")
	}
	if trackCount != 18 {
		t.Errorf("track_count = %d, want 18", trackCount)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 1 {
		t.Fatalf("deluxe_change event row count = %d, want exactly 1", deluxeCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_NoEventOnEqualCount(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Equal Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Equal Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (establish baseline at 12): %v", err)
	}
	// Cycle 3: fetch reports the same 12-track release again -- no change.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 3 (equal count): %v", err)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change event row count = %d, want 0 (an equal count must never fire)", deluxeCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_NoEventOnDecrease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Decrease Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Decrease Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-deluxe", "Album (Deluxe)", "2020-06-01", 18)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (establish baseline at 18): %v", err)
	}

	// Cycle 3: a lower count (an upstream data correction) must not fire
	// and must not lower the baseline.
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 3 (decrease): %v", err)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change event row count = %d, want 0 (a decrease must never fire)", deluxeCount)
	}

	var trackCount *int32
	if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
		t.Fatalf("query baseline track_count: %v", err)
	}
	if trackCount == nil || *trackCount != 18 {
		t.Fatalf("baseline track_count = %v, want 18 (a decrease must never lower the baseline)", trackCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_SkipsBrandNewGroup(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Brand New Group Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Brand New Group Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	if got := releases.calls(); got != 0 {
		t.Fatalf("release-detail calls = %d, want 0 (a brand-new group must never be fetched in its own discovery cycle, D-04)", got)
	}

	var newReleaseCount, deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release'", artistID).Scan(&newReleaseCount); err != nil {
		t.Fatalf("count new_release: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change: %v", err)
	}
	if newReleaseCount != 1 {
		t.Fatalf("new_release count = %d, want 1", newReleaseCount)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change count = %d, want 0", deluxeCount)
	}
}
