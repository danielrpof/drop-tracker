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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/discord"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/notifier"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeRecordingSource is a controllable double for detection.RecordingSource,
// shared by every test in this package (and deezer_test.go, same package)
// that constructs a Detector -- the zero value returns no recordings and no
// error, a no-op for guest-feature detection, which is what every
// new_release-focused test in this file needs. releasesForRecording is
// likewise omitted (nil map, no-op) by default -- ReleasesForRecording then
// returns a nil slice and a nil error for every mbid, so the 30+ existing
// call sites that construct fakeRecordingSource{} unchanged keep seeing no
// per-recording lookup result, exactly as before this field existed.
type fakeRecordingSource struct {
	recordings           []musicbrainz.Recording
	err                  error
	releasesForRecording map[string][]musicbrainz.RecordingRelease
	releasesErr          error
}

func (f fakeRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	return f.recordings, f.err
}

func (f fakeRecordingSource) ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error) {
	if f.releasesErr != nil {
		return nil, f.releasesErr
	}
	return f.releasesForRecording[mbid], nil
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

func (f *fakeReleaseDetailSource) setErr(groupMBID string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errByGroup == nil {
		f.errByGroup = map[string]error{}
	}
	f.errByGroup[groupMBID] = err
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
		release_date, cover_art_url, event_type, source, release_type, watched_artist_name
		FROM events WHERE artist_id = $1 ORDER BY external_id`, artistID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	type row struct {
		externalID, releaseGroupMbid, title, artistName string
		releaseDate, coverArtURL                        *string
		eventType, source                               string
		releaseType                                     *string
		watchedArtistName                               *string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.externalID, &r.releaseGroupMbid, &r.title, &r.artistName,
			&r.releaseDate, &r.coverArtURL, &r.eventType, &r.source, &r.releaseType, &r.watchedArtistName); err != nil {
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
		if got[i].releaseType == nil || *got[i].releaseType != "album" {
			t.Fatalf("row %d release_type = %v, want %q (lowercased/trimmed PrimaryType %q)", i, got[i].releaseType, "album", "Album")
		}
		if got[i].watchedArtistName == nil || *got[i].watchedArtistName != "Test Artist" {
			t.Fatalf("row %d watched_artist_name = %v, want %q (equal to artist_name for new_release)", i, got[i].watchedArtistName, "Test Artist")
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
	var releaseType *string
	var previousTrackCount *int32
	var watchedArtistName *string
	row := pool.QueryRow(ctx, `SELECT event_type, source, external_id, release_group_mbid, release_date, cover_art_url, title, artist_name, release_type, previous_track_count, watched_artist_name
		FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'`, artistID)
	if err := row.Scan(&eventType, &source, &externalID, &releaseGroupMbid, &releaseDate, &coverArtURL, &title, &artistName, &releaseType, &previousTrackCount, &watchedArtistName); err != nil {
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
	if releaseType != nil {
		t.Errorf("release_type = %v, want NULL (neither NTFY-02 nor any requirement asks for it on guest_feature)", *releaseType)
	}
	if previousTrackCount != nil {
		t.Errorf("previous_track_count = %v, want NULL", *previousTrackCount)
	}
	if watchedArtistName == nil || *watchedArtistName != "Guest Feature Artist" {
		t.Errorf("watched_artist_name = %v, want %q (the watchlist entry's own name, not the primary credit)", watchedArtistName, "Guest Feature Artist")
	}
	if artistName == *watchedArtistName {
		t.Errorf("artist_name (%q) and watched_artist_name (%q) must differ on a guest_feature row -- that difference IS the fix", artistName, *watchedArtistName)
	}
}

// TestDetectMusicBrainz_GuestFeatureStoresReleaseDateAndCoverArt proves
// D-01's end-to-end wiring: a previously-unseen guest-feature recording
// whose ReleasesForRecording lookup returns one dated release produces an
// event row carrying release_date, release_group_mbid and cover_art_url.
// Like every other DetectMusicBrainz test in this file, the artist's first
// cycle is a seed cycle (isSeedMode); that only affects notified_at, not
// the release_date/cover_art_url wiring this test asserts on.
func TestDetectMusicBrainz_GuestFeatureStoresReleaseDateAndCoverArt(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Dated Guest Feature Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Dated Guest Feature Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	recordingMBID := mbid + "-rec1"
	recordings := []musicbrainz.Recording{
		{
			MBID:  recordingMBID,
			Title: "Feature Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("primary-mbid-0000", "Primary Artist"),
				mkCredit(mbid, "Dated Guest Feature Artist"),
			},
		},
	}
	releasesForRecording := map[string][]musicbrainz.RecordingRelease{
		recordingMBID: {
			{
				MBID:  "rel-1",
				Title: "Some Album",
				Date:  "2026-03-04",
				ReleaseGroup: musicbrainz.RecordingReleaseGroup{
					MBID:  "rg-1",
					Title: "Some Album",
				},
			},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings, releasesForRecording: releasesForRecording}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var releaseGroupMbid, releaseDate, coverArtURL *string
	row := pool.QueryRow(ctx, `SELECT release_group_mbid, release_date, cover_art_url
		FROM events WHERE artist_id = $1 AND event_type = 'guest_feature' AND external_id = $2`, artistID, recordingMBID)
	if err := row.Scan(&releaseGroupMbid, &releaseDate, &coverArtURL); err != nil {
		t.Fatalf("query guest_feature row: %v", err)
	}
	if releaseGroupMbid == nil || *releaseGroupMbid != "rg-1" {
		t.Errorf("release_group_mbid = %v, want %q", releaseGroupMbid, "rg-1")
	}
	if releaseDate == nil || *releaseDate != "2026-03-04" {
		t.Errorf("release_date = %v, want %q", releaseDate, "2026-03-04")
	}
	wantCoverArt := "https://coverartarchive.org/release-group/rg-1/front"
	if coverArtURL == nil || *coverArtURL != wantCoverArt {
		t.Errorf("cover_art_url = %v, want %q", coverArtURL, wantCoverArt)
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

// TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier extends
// TestDetectMusicBrainz_GuestFeature_Muted's proof through the notifier
// path (NTFY-04): a muted guest_feature never becomes a row (restated here
// as a precondition check, so a future regression fails at the right
// layer), and a real NotifyPending call against a real discord.Client only
// ever delivers the sibling new_release event from the same detection
// cycle -- the muted recording's distinguishing title never appears in any
// request the notifier issues.
func TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Muted Guest Artist Notifier")
	entry := watchlist.Entry{
		ArtistID:        artistID,
		MBID:            mbid,
		Name:            "Muted Guest Artist Notifier",
		ReleaseTypes:    []string{"album", "single", "ep", "deluxe"},
		MutedEventTypes: []string{"guest_feature"},
	}
	seedGroups := []musicbrainz.ReleaseGroup{{MBID: mbid + "-seed-rg", Title: "Seed Album", PrimaryType: "Album"}}
	groups := append(append([]musicbrainz.ReleaseGroup{}, seedGroups...),
		musicbrainz.ReleaseGroup{MBID: mbid + "-rg1", Title: "Allowed Release", PrimaryType: "Album"})
	recordings := []musicbrainz.Recording{
		{
			MBID:  mbid + "-rec1",
			Title: "Suppressed Guest Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("other-mbid", "Other Artist"),
				mkCredit(mbid, "Muted Guest Artist Notifier"),
			},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})

	// Seed cycle first (D-14): a bare DetectMusicBrainz call establishes
	// this (artist_id, source) pair as no-longer-first-cycle, so the real
	// test cycle below inserts a genuinely unnotified new_release row
	// rather than a pre-notified seed row -- otherwise NotifyPending would
	// have nothing pending to drain and this test would prove nothing about
	// the notifier path. The seed cycle's own recording is intentionally
	// omitted (fakeRecordingSource{} default, no-op) so it contributes no
	// guest_feature noise.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, seedGroups); err != nil {
		t.Fatalf("seed DetectMusicBrainz: %v", err)
	}

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	// Precondition: confirm detection's own mute filter ran -- the same
	// assertion TestDetectMusicBrainz_GuestFeature_Muted already makes,
	// restated here so a regression at the detection layer fails at this
	// layer, not the notifier layer exercised below. newReleaseCount is 2
	// (the seed cycle's own row plus the second cycle's genuinely-new
	// "Allowed Release" row) since the seed cycle intentionally inserts one
	// real new_release row to end seed mode.
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
	if newReleaseCount != 2 {
		t.Fatalf("new_release event row count = %d, want 2 (the seed row plus the second cycle's new row; mute is per-event-type, new_release must still land)", newReleaseCount)
	}

	var eventID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, mbid+"-rg1").Scan(&eventID); err != nil {
		t.Fatalf("select new_release event id: %v", err)
	}

	var mu sync.Mutex
	reqCount := 0
	var bodies []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqCount++
		bodies = append(bodies, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := discord.NewClient(ts.URL, ts.Client())
	n := notifier.New(sqlc.New(pool), client, time.Millisecond)
	if err := n.NotifyPending(ctx, testLogger()); err != nil {
		t.Fatalf("NotifyPending: %v", err)
	}

	mu.Lock()
	gotReqCount := reqCount
	gotBodies := append([]string(nil), bodies...)
	mu.Unlock()

	if gotReqCount != 1 {
		t.Fatalf("server received %d requests, want exactly 1 (only the new_release event)", gotReqCount)
	}
	for _, b := range gotBodies {
		if strings.Contains(b, "Suppressed Guest Track") {
			t.Fatalf("a request body contained the muted guest_feature recording's distinguishing title -- it must never reach the notifier: %s", b)
		}
	}

	var notified bool
	if err := pool.QueryRow(ctx, "SELECT notified_at IS NOT NULL FROM events WHERE id = $1", eventID).Scan(&notified); err != nil {
		t.Fatalf("query notified_at: %v", err)
	}
	if !notified {
		t.Fatal("new_release row's notified_at should be non-NULL after NotifyPending")
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

// The tests below (13-01 task 2) prove D-02's precision-aware earliest-date
// rule, D-03's no-releases fallback, OQ-02's per-recording error isolation,
// and D-13's per-cycle lookup cap all reach the database correctly, not just
// their unit-level helpers (musicbrainz_test.go's TestEarliestReleaseDate/
// TestGuestFeatureArt).

func TestDetectMusicBrainz_GuestFeature_EarliestDateReachesDB(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Earliest Date DB Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Earliest Date DB Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

	recordingMBID := mbid + "-rec1"
	recordings := []musicbrainz.Recording{
		{
			MBID:  recordingMBID,
			Title: "Feature Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("primary-mbid-0000", "Primary Artist"),
				mkCredit(mbid, "Earliest Date DB Artist"),
			},
		},
	}
	// A same-year prefix pair ("2020" vs "2020-01-05") proves the DB-level
	// wiring uses earliestReleaseDate's corrected comparator, not a plain
	// lexicographic pick that would wrongly choose "2020".
	releasesForRecording := map[string][]musicbrainz.RecordingRelease{
		recordingMBID: {
			{MBID: "rel-vague", Title: "Reissue", Date: "2020", ReleaseGroup: musicbrainz.RecordingReleaseGroup{MBID: "rg-vague"}},
			{MBID: "rel-precise", Title: "Original", Date: "2020-01-05", ReleaseGroup: musicbrainz.RecordingReleaseGroup{MBID: "rg-precise"}},
		},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings, releasesForRecording: releasesForRecording}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var releaseDate string
	if err := pool.QueryRow(ctx, "SELECT release_date FROM events WHERE artist_id = $1 AND event_type = 'guest_feature' AND external_id = $2", artistID, recordingMBID).Scan(&releaseDate); err != nil {
		t.Fatalf("query release_date: %v", err)
	}
	if releaseDate != "2020-01-05" {
		t.Fatalf("release_date = %q, want %q (the more precise same-year date, not the vaguer one)", releaseDate, "2020-01-05")
	}
}

func TestDetectMusicBrainz_GuestFeature_EmptyReleaseListInsertsWithNulls(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Empty Release List Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Empty Release List Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

	recordingMBID := mbid + "-rec1"
	recordings := []musicbrainz.Recording{
		{
			MBID:  recordingMBID,
			Title: "Feature Track",
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("primary-mbid-0000", "Primary Artist"),
				mkCredit(mbid, "Empty Release List Artist"),
			},
		},
	}
	// releasesForRecording deliberately omits recordingMBID -- the fake's
	// zero-value default (nil slice, nil error) mirrors a lookup that
	// succeeds but returns no releases (D-03).
	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	var releaseGroupMbid, releaseDate, coverArtURL *string
	row := pool.QueryRow(ctx, `SELECT release_group_mbid, release_date, cover_art_url
		FROM events WHERE artist_id = $1 AND event_type = 'guest_feature' AND external_id = $2`, artistID, recordingMBID)
	if err := row.Scan(&releaseGroupMbid, &releaseDate, &coverArtURL); err != nil {
		t.Fatalf("query guest_feature row: %v", err)
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
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if count != 1 {
		t.Fatalf("guest_feature event row count = %d, want 1 (D-03: still inserted, just with NULL date/art)", count)
	}
}

func TestDetectMusicBrainz_GuestFeature_PerRecordingLookupErrorIsolated(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Per Recording Error Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Per Recording Error Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

	okMBID1 := mbid + "-rec-ok-1"
	failMBID := mbid + "-rec-fail"
	okMBID2 := mbid + "-rec-ok-2"
	recordings := []musicbrainz.Recording{
		{MBID: okMBID1, Title: "OK Track 1", ArtistCredit: []musicbrainz.ArtistCreditEntry{mkCredit("primary-mbid-0000", "Primary Artist"), mkCredit(mbid, "Per Recording Error Artist")}},
		{MBID: failMBID, Title: "Failing Track", ArtistCredit: []musicbrainz.ArtistCreditEntry{mkCredit("primary-mbid-0000", "Primary Artist"), mkCredit(mbid, "Per Recording Error Artist")}},
		{MBID: okMBID2, Title: "OK Track 2", ArtistCredit: []musicbrainz.ArtistCreditEntry{mkCredit("primary-mbid-0000", "Primary Artist"), mkCredit(mbid, "Per Recording Error Artist")}},
	}

	d := detection.New(sqlc.New(pool), erroringRecordingSource{
		recordings: recordings,
		errByMBID:  map[string]error{failMBID: errors.New("recording release lookup failed")},
	}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v, want nil (a per-recording lookup error must not fail the cycle)", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if count != 2 {
		t.Fatalf("guest_feature event row count = %d, want 2 (the two siblings still insert; only the failing recording is skipped)", count)
	}
	var failCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature' AND external_id = $2", artistID, failMBID).Scan(&failCount); err != nil {
		t.Fatalf("count failing recording's events: %v", err)
	}
	if failCount != 0 {
		t.Fatalf("failing recording's event row count = %d, want 0", failCount)
	}
}

func TestDetectMusicBrainz_GuestFeature_PerCycleLookupCap(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Lookup Cap Artist")
	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Lookup Cap Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}

	const totalRecordings = 25 // maxNewGuestFeatureLookupsPerCycle (20) + 5
	recordings := make([]musicbrainz.Recording, totalRecordings)
	releasesForRecording := make(map[string][]musicbrainz.RecordingRelease, totalRecordings)
	for i := 0; i < totalRecordings; i++ {
		recMBID := fmt.Sprintf("%s-rec%d", mbid, i)
		recordings[i] = musicbrainz.Recording{
			MBID:  recMBID,
			Title: fmt.Sprintf("Track %d", i),
			ArtistCredit: []musicbrainz.ArtistCreditEntry{
				mkCredit("primary-mbid-0000", "Primary Artist"),
				mkCredit(mbid, "Lookup Cap Artist"),
			},
		}
		releasesForRecording[recMBID] = []musicbrainz.RecordingRelease{
			{MBID: "rel-" + recMBID, Title: "Album", Date: "2020-01-01", ReleaseGroup: musicbrainz.RecordingReleaseGroup{MBID: "rg-" + recMBID}},
		}
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{recordings: recordings, releasesForRecording: releasesForRecording}, &fakeReleaseDetailSource{})
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events: %v", err)
	}
	if count != 20 {
		t.Fatalf("guest_feature event row count = %d, want 20 (maxNewGuestFeatureLookupsPerCycle)", count)
	}

	// The excess recordings must be absent from the seen store too (not
	// just un-inserted), so a subsequent cycle would reconsider them --
	// re-running the same cycle must insert the remaining 5.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz (second cycle): %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'", artistID).Scan(&count); err != nil {
		t.Fatalf("count guest_feature events after second cycle: %v", err)
	}
	if count != totalRecordings {
		t.Fatalf("guest_feature event row count after second cycle = %d, want %d (the excess 5 must have been eligible again)", count, totalRecordings)
	}
}

// erroringRecordingSource is a controllable detection.RecordingSource double
// that fails ReleasesForRecording only for mbids listed in errByMBID --
// unlike fakeRecordingSource's single err field (all-or-nothing), this lets
// a test fail exactly one recording's lookup while its siblings succeed.
type erroringRecordingSource struct {
	recordings []musicbrainz.Recording
	errByMBID  map[string]error
}

func (s erroringRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	return s.recordings, nil
}

func (s erroringRecordingSource) ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error) {
	if err, ok := s.errByMBID[mbid]; ok {
		return nil, err
	}
	return nil, nil
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

// The three tests below (quick/260826-gj8) prove the recheck-window gate:
// detectDeluxeChanges must not issue a ReleasesByReleaseGroup call for an
// already-seen release-group whose FirstReleaseDate is older than
// deluxeRecheckWindowDays, while a group inside the window (or with an
// absent date) is still checked exactly as before. Each reuses
// TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline's
// two-cycle arrangement: cycle 1 discovers the group as a new_release
// (D-04: no fetch that cycle regardless of date), cycle 2 is the first
// cycle in which the deluxe pass would fetch it.

func TestDetectMusicBrainz_DeluxeChange_SkipsGroupOutsideRecheckWindow(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Window Outside Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Window Outside Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	// Years old -- well outside deluxeRecheckWindowDays (90 days).
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album", FirstReleaseDate: "2020-01-01"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	// Cycle 1: discovers the group -- new_release only, no release-detail
	// fetch this cycle regardless of date (D-04).
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 0 {
		t.Fatalf("cycle 1 release-detail calls = %d, want 0 (D-04)", got)
	}

	// Cycle 2: the group is now already-seen, but its FirstReleaseDate is
	// outside the recheck window -- the age gate must suppress the fetch
	// that D-04 alone would otherwise have allowed.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 0 {
		t.Fatalf("cycle 2 release-detail calls = %d, want 0 (the age gate must suppress this fetch)", got)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change event row count = %d, want 0", deluxeCount)
	}

	var trackCount *int32
	if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
		t.Fatalf("query baseline track_count: %v", err)
	}
	if trackCount != nil {
		t.Fatalf("baseline track_count = %v, want NULL (no fetch occurred, so no baseline should have been established)", *trackCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_ChecksGroupInsideRecheckWindow(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Window Inside Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Window Inside Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	// 10 days ago, derived from the clock rather than a hardcoded literal so
	// this fixture cannot silently rot into an outside-window case as time
	// passes -- well inside deluxeRecheckWindowDays (90 days).
	recentDate := time.Now().UTC().AddDate(0, 0, -10).Format(time.DateOnly)
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album", FirstReleaseDate: recentDate}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", recentDate, 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 0 {
		t.Fatalf("cycle 1 release-detail calls = %d, want 0 (D-04)", got)
	}

	// Cycle 2: the group is already-seen and inside the recheck window --
	// the gate must not suppress this fetch, proving it did not break the
	// normal path.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 1 {
		t.Fatalf("cycle 2 release-detail calls = %d, want 1 (inside the window, the fetch must still happen)", got)
	}

	var trackCount *int32
	if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
		t.Fatalf("query baseline track_count: %v", err)
	}
	if trackCount == nil || *trackCount != 12 {
		t.Fatalf("baseline track_count = %v, want 12 (the fetch happened and established the baseline)", trackCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_UndatedGroupIsStillChecked(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Window Undated Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Window Undated Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	// Deliberately empty FirstReleaseDate -- this is the assertion under
	// test, not an omission. Do not "tidy" a date into this fixture: an
	// undated group (MusicBrainz's actual value for many real release
	// groups) must never be silently dropped from detection.
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album", FirstReleaseDate: ""}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 0 {
		t.Fatalf("cycle 1 release-detail calls = %d, want 0 (D-04)", got)
	}

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 DetectMusicBrainz: %v", err)
	}
	if got := releases.calls(); got != 1 {
		t.Fatalf("cycle 2 release-detail calls = %d, want 1 (an undated group must still be checked)", got)
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
	var previousTrackCount *int32
	row := pool.QueryRow(ctx, `SELECT event_type, source, external_id, release_group_mbid, title, track_count, previous_track_count
		FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'`, artistID)
	if err := row.Scan(&eventType, &source, &externalID, &releaseGroupMbid, &title, &trackCount, &previousTrackCount); err != nil {
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
	if previousTrackCount == nil || *previousTrackCount != 12 {
		t.Errorf("previous_track_count = %v, want 12 (the pre-change baseline, captured before setGroupBaseline overwrote it, D-04)", previousTrackCount)
	}

	var deluxeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change events: %v", err)
	}
	if deluxeCount != 1 {
		t.Fatalf("deluxe_change event row count = %d, want exactly 1", deluxeCount)
	}
}

// errInsertEventFailingQuerier is the sentinel insertEventFailingQuerier
// returns in place of a real InsertEvent failure -- distinct from any real
// Postgres error so a test can assert precisely which path produced it.
var errInsertEventFailingQuerier = errors.New("insertEventFailingQuerier: forced insert failure")

// insertEventFailingQuerier embeds sqlc.Querier (an interface, not a
// concrete *sqlc.Queries -- the seam this test relies on, since Detector
// only ever holds a sqlc.Querier) so every method delegates to the real
// querier with no boilerplate, except InsertEvent, which is overridden to
// fail only for the deluxe_change event type. Scoping the failure this way
// lets a test seed a real new_release baseline row through the same
// querier first, then force only the deluxe-change insert to fail.
type insertEventFailingQuerier struct {
	sqlc.Querier
}

func (q *insertEventFailingQuerier) InsertEvent(ctx context.Context, arg sqlc.InsertEventParams) (int64, error) {
	if arg.EventType == "deluxe_change" {
		return 0, errInsertEventFailingQuerier
	}
	return q.Querier.InsertEvent(ctx, arg)
}

// TestDetectMusicBrainz_DeluxeChange_InsertFailureLogsWindowSignal proves the
// D-12 window log signal actually fires on the real baseline-advanced/
// insert-failed code path in detectDeluxeChanges's default: branch, rather
// than merely existing as a string literal in the source. Mirrors
// TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease's arrange phase (plain
// querier establishes the baseline across two cycles), then swaps in
// insertEventFailingQuerier for the third cycle so the increase is real but
// the resulting InsertEvent fails.
func TestDetectMusicBrainz_DeluxeChange_InsertFailureLogsWindowSignal(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deluxe Insert Failure Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deluxe Insert Failure Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-orig", "Album", "2020-01-01", 12)})

	// The baseline-establishing cycles use the plain querier -- only the
	// cycle under test needs the failing decorator.
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (establish baseline at 12): %v", err)
	}

	deluxeMBID := groupMBID + "-deluxe"
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(deluxeMBID, "Album (Deluxe)", "2020-06-01", 18)})

	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	failing := &insertEventFailingQuerier{Querier: sqlc.New(pool)}
	dFailing := detection.New(failing, fakeRecordingSource{}, releases)

	if err := dFailing.DetectMusicBrainz(ctx, logger, entry, groups); err == nil {
		t.Fatal("cycle 3 (increase to 18, insert forced to fail): DetectMusicBrainz returned nil error, want non-nil")
	}

	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if rec["window"] != "baseline_advanced_insert_failed" {
			continue
		}
		found = true
		if rec["artist_mbid"] != mbid {
			t.Errorf("artist_mbid = %v, want %q", rec["artist_mbid"], mbid)
		}
		if rec["release_group_mbid"] != groupMBID {
			t.Errorf("release_group_mbid = %v, want %q", rec["release_group_mbid"], groupMBID)
		}
	}
	if !found {
		t.Fatal("no log record with window = \"baseline_advanced_insert_failed\" found")
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

// The tests below (04-04 task 2) harden the deluxe pass against malformed
// media, verify both preference gates cost zero requests (not just zero
// rows), and prove the pass is order-independent, non-refiring, and
// per-group error-isolated.

func TestDetectMusicBrainz_DeluxeChange_EmptyMediaLeavesBaseline(t *testing.T) {
	t.Run("no baseline yet, none recorded", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := testMBID(t)
		artistID := insertTestArtist(t, pool, mbid, "Empty Media No Baseline Artist")

		entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Empty Media No Baseline Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
		groupMBID := mbid + "-rg1"
		groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

		releases := &fakeReleaseDetailSource{}
		releases.setReleases(groupMBID, []musicbrainz.Release{
			{MBID: groupMBID + "-rel1", Title: "No Media At All"},
		})
		d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

		if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
			t.Fatalf("cycle 1 (discover): %v", err)
		}
		if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
			t.Fatalf("cycle 2 (zero-total fetch): %v", err)
		}

		var trackCount *int32
		if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
			t.Fatalf("query track_count: %v", err)
		}
		if trackCount != nil {
			t.Fatalf("track_count = %v, want NULL (a zero-total fetch must not record a baseline)", *trackCount)
		}

		var deluxeCount int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
			t.Fatalf("count deluxe_change: %v", err)
		}
		if deluxeCount != 0 {
			t.Fatalf("deluxe_change count = %d, want 0", deluxeCount)
		}
	})

	t.Run("existing baseline is left unchanged", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := testMBID(t)
		artistID := insertTestArtist(t, pool, mbid, "Empty Media Existing Baseline Artist")

		entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Empty Media Existing Baseline Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
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

		// Cycle 3: absent media, empty media, and media with no track-count
		// all sum to 0 -- none of them is usable data.
		releases.setReleases(groupMBID, []musicbrainz.Release{
			{MBID: groupMBID + "-a", Title: "No Media"},
			{MBID: groupMBID + "-b", Title: "Empty Media", Media: []musicbrainz.Medium{}},
			{MBID: groupMBID + "-c", Title: "No Track Count", Media: []musicbrainz.Medium{{Format: "CD", Position: 1}}},
		})
		if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
			t.Fatalf("cycle 3 (malformed media): %v", err)
		}

		var trackCount *int32
		if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
			t.Fatalf("query track_count: %v", err)
		}
		if trackCount == nil || *trackCount != 12 {
			t.Fatalf("track_count = %v, want 12 (a zero-total fetch must leave the existing baseline untouched)", trackCount)
		}

		var deluxeCount int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
			t.Fatalf("count deluxe_change: %v", err)
		}
		if deluxeCount != 0 {
			t.Fatalf("deluxe_change count = %d, want 0", deluxeCount)
		}
	})
}

func TestDetectMusicBrainz_DeluxeChange_UsesGroupMaximumNotOrder(t *testing.T) {
	buildReleases := func(groupMBID string) []musicbrainz.Release {
		return []musicbrainz.Release{
			mkRelease(groupMBID+"-a", "Release A", "2020-01-01", 14),
			mkRelease(groupMBID+"-b", "Release B", "2020-02-01", 21),
			mkRelease(groupMBID+"-c", "Release C", "2020-03-01", 9),
		}
	}
	orders := []struct {
		name  string
		order []int
	}{
		{"ascending", []int{0, 1, 2}},
		{"descending", []int{2, 1, 0}},
		{"shuffled", []int{1, 2, 0}},
	}

	for _, tt := range orders {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.NewTestPool(t)
			ctx := context.Background()
			mbid := testMBID(t)
			artistID := insertTestArtist(t, pool, mbid, "Group Maximum Artist "+tt.name)

			entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Group Maximum Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
			groupMBID := mbid + "-rg1"
			groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

			all := buildReleases(groupMBID)
			ordered := make([]musicbrainz.Release, len(tt.order))
			for i, idx := range tt.order {
				ordered[i] = all[idx]
			}

			releases := &fakeReleaseDetailSource{}
			releases.setReleases(groupMBID, ordered)
			d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

			if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
				t.Fatalf("cycle 1 (discover): %v", err)
			}
			if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
				t.Fatalf("cycle 2 (establish baseline): %v", err)
			}

			var trackCount *int32
			if err := pool.QueryRow(ctx, "SELECT track_count FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id = $2", artistID, groupMBID).Scan(&trackCount); err != nil {
				t.Fatalf("query track_count: %v", err)
			}
			if trackCount == nil || *trackCount != 21 {
				t.Fatalf("track_count = %v, want 21 regardless of upstream order (the comparison is a maximum, not a scan-until-first-hit)", trackCount)
			}
		})
	}
}

func TestDetectMusicBrainz_DeluxeChange_RequiresDeluxePreference(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "No Deluxe Preference Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "No Deluxe Preference Artist", ReleaseTypes: []string{"album", "single", "ep"}}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (group now already-seen): %v", err)
	}

	if got := releases.calls(); got != 0 {
		t.Fatalf("release-detail calls = %d, want 0 (an entry whose ReleaseTypes omits deluxe must never fetch, checked on the fake's call counter, not row count alone)", got)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&count); err != nil {
		t.Fatalf("count deluxe_change: %v", err)
	}
	if count != 0 {
		t.Fatalf("deluxe_change count = %d, want 0", count)
	}
}

func TestDetectMusicBrainz_DeluxeChange_Muted(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Muted Deluxe Artist")

	entry := watchlist.Entry{
		ArtistID:        artistID,
		MBID:            mbid,
		Name:            "Muted Deluxe Artist",
		ReleaseTypes:    []string{"album", "single", "ep", "deluxe"},
		MutedEventTypes: []string{"deluxe_change"},
	}
	groupMBID := mbid + "-rg1"
	groups := []musicbrainz.ReleaseGroup{{MBID: groupMBID, Title: "Album", PrimaryType: "Album"}}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(groupMBID, []musicbrainz.Release{mkRelease(groupMBID+"-rel1", "Album", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (group now already-seen): %v", err)
	}

	if got := releases.calls(); got != 0 {
		t.Fatalf("release-detail calls = %d, want 0 (a muted deluxe_change must never fetch, checked on the fake's call counter, not row count alone)", got)
	}

	var deluxeCount, newReleaseCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&deluxeCount); err != nil {
		t.Fatalf("count deluxe_change: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release'", artistID).Scan(&newReleaseCount); err != nil {
		t.Fatalf("count new_release: %v", err)
	}
	if deluxeCount != 0 {
		t.Fatalf("deluxe_change count = %d, want 0 (muted)", deluxeCount)
	}
	if newReleaseCount != 1 {
		t.Fatalf("new_release count = %d, want 1 (mute is per-event-type; new_release must still land)", newReleaseCount)
	}
}

func TestDetectMusicBrainz_DeluxeChange_DoesNotRefireForSameRelease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "No Refire Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "No Refire Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
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
		t.Fatalf("cycle 3 (fires): %v", err)
	}

	// Cycle 4: identical input again -- must not produce a second row,
	// both because the baseline now matches 18 and because the release
	// MBID is already in the dedup key.
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 4 (re-run): %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change'", artistID).Scan(&count); err != nil {
		t.Fatalf("count deluxe_change: %v", err)
	}
	if count != 1 {
		t.Fatalf("deluxe_change count = %d, want exactly 1 (must not refire for the same release)", count)
	}
}

func TestDetectMusicBrainz_DeluxeChange_PerGroupErrorIsolated(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Per Group Error Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Per Group Error Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	group1 := mbid + "-rg1"
	group2 := mbid + "-rg2"
	groups := []musicbrainz.ReleaseGroup{
		{MBID: group1, Title: "Album One", PrimaryType: "Album"},
		{MBID: group2, Title: "Album Two", PrimaryType: "Album"},
	}

	releases := &fakeReleaseDetailSource{}
	releases.setReleases(group1, []musicbrainz.Release{mkRelease(group1+"-rel1", "Album One", "2020-01-01", 12)})
	releases.setReleases(group2, []musicbrainz.Release{mkRelease(group2+"-rel1", "Album Two", "2020-01-01", 12)})
	d := detection.New(sqlc.New(pool), fakeRecordingSource{}, releases)

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 1 (discover both): %v", err)
	}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 2 (establish both baselines): %v", err)
	}

	// Cycle 3: group1's release-detail fetch fails; group2 must still get
	// its deluxe_change, and DetectMusicBrainz must still return nil.
	releases.setErr(group1, errors.New("release detail fetch failed"))
	group2DeluxeMBID := group2 + "-deluxe"
	releases.setReleases(group2, []musicbrainz.Release{mkRelease(group2DeluxeMBID, "Album Two (Deluxe)", "2020-06-01", 18)})

	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, groups); err != nil {
		t.Fatalf("cycle 3 (per-group error must be isolated): %v, want nil", err)
	}

	var group1Deluxe, group2Deluxe int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change' AND release_group_mbid = $2", artistID, group1).Scan(&group1Deluxe); err != nil {
		t.Fatalf("count group1 deluxe_change: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'deluxe_change' AND release_group_mbid = $2", artistID, group2).Scan(&group2Deluxe); err != nil {
		t.Fatalf("count group2 deluxe_change: %v", err)
	}
	if group1Deluxe != 0 {
		t.Fatalf("group1 deluxe_change count = %d, want 0 (its release-detail fetch errored)", group1Deluxe)
	}
	if group2Deluxe != 1 {
		t.Fatalf("group2 deluxe_change count = %d, want 1 (a sibling group's error must not block it)", group2Deluxe)
	}
}
