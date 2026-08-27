package detection

// This file is package detection (whitebox, not detection_test) since
// notifyGate and onOrAfterCutoff are unexported -- mirroring
// baseline_test.go's and filter_test.go's established convention for
// covering unexported detection internals that also need real-Postgres
// integration coverage, and reusing filter_test.go's filterTestMBID/
// insertFilterTestArtist/filterTestLogger helpers and its
// noReleaseDetailSource double rather than duplicating them.
//
// The bug these tests exist to catch (.planning/debug/
// backlog-songs-trigger-discord.md): notification suppression used to be
// decided purely by INSERT TIMING -- seed mode, a one-shot latch that
// flips off the instant the first event row for an (artist_id, source)
// pair exists. detectGuestFeatures is deliberately MULTI-cycle
// (maxNewGuestFeatureLookupsPerCycle caps one artist at 20 lookups per
// cycle, and per-recording lookup errors defer their recordings too), so
// an artist with a large back catalogue kept inserting backlog rows on
// later, non-seed cycles. Those rows were born with notified_at = NULL,
// and ListUnnotified applies no release-date filter at all, so the entire
// back catalogue was delivered to Discord 20 rows per poll cycle. In
// production this sent 242 back-catalogue releases (reaching back to
// 2015) in a single day.
//
// TestDetectMusicBrainz_NonSeedCycle_* and
// TestDetectGuestFeatures_PastLookupCap_* are therefore the load-bearing
// regression tests: both run on an artist that is explicitly NOT in seed
// mode, which is the exact condition the old code had no defence for.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// datedRecordingSource is a controllable detection.RecordingSource double
// that also answers ReleasesForRecording, so a guest_feature row gets a
// real release_date via earliestReleaseDate -- noRecordingSource returns
// nothing for both calls and cannot exercise the freshness gate.
type datedRecordingSource struct {
	recordings []musicbrainz.Recording
	releases   map[string][]musicbrainz.RecordingRelease
}

func (f datedRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	return f.recordings, nil
}

func (f datedRecordingSource) ReleasesForRecording(ctx context.Context, mbid string) ([]musicbrainz.RecordingRelease, error) {
	return f.releases[mbid], nil
}

// pendingCount reports how many rows for artistID are still pending
// delivery (notified_at IS NULL) -- i.e. exactly what ListUnnotified would
// hand the notifier, and therefore exactly what would become a Discord
// message. This is the assertion target for every test in this file: the
// bug is not "a row was created," it is "a row was created in a
// deliverable state."
func pendingCount(t *testing.T, pool *pgxpool.Pool, artistID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM events WHERE artist_id = $1 AND notified_at IS NULL",
		artistID,
	).Scan(&n); err != nil {
		t.Fatalf("count pending events: %v", err)
	}
	return n
}

// pendingExternalIDs returns the external ids still pending for artistID,
// so a test can assert exactly WHICH rows are deliverable rather than only
// how many.
func pendingExternalIDs(t *testing.T, pool *pgxpool.Pool, artistID int64) map[string]struct{} {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		"SELECT external_id FROM events WHERE artist_id = $1 AND notified_at IS NULL", artistID)
	if err != nil {
		t.Fatalf("query pending external ids: %v", err)
	}
	defer rows.Close()
	ids := map[string]struct{}{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan pending external id: %v", err)
		}
		ids[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate pending external ids: %v", err)
	}
	return ids
}

// insertPriorEvent inserts one already-notified event row for artistID,
// which is what takes the artist OUT of seed mode (isSeedMode is
// NOT EXISTS over (artist_id, source)). Every regression test in this file
// calls this first: seed mode was never the broken path, and a test that
// let the artist stay in seed mode would pass against the buggy code.
func insertPriorEvent(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name, notified_at)
		VALUES ($1, 'musicbrainz', 'new_release', $2, 'Prior Release', 'Prior Artist', now())`,
		artistID, externalID); err != nil {
		t.Fatalf("insert prior event: %v", err)
	}
}

func TestNotifyGate_NotifiedAt(t *testing.T) {
	// A fixed "now" so the table's expectations are stable rather than
	// relative to wall-clock time. maxAgeDays 7 puts the cutoff at
	// 2026-08-19.
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		seedMode     bool
		releaseDate  string
		wantSuppress bool
	}{
		{
			name:         "seed mode suppresses a fresh release",
			seedMode:     true,
			releaseDate:  "2026-08-26",
			wantSuppress: true,
		},
		{
			name:         "seed mode suppresses a backlog release",
			seedMode:     true,
			releaseDate:  "2015-05-21",
			wantSuppress: true,
		},
		{
			name:         "non-seed backlog release is suppressed",
			seedMode:     false,
			releaseDate:  "2015-05-21",
			wantSuppress: true,
		},
		{
			name:         "non-seed release dated today is delivered",
			seedMode:     false,
			releaseDate:  "2026-08-26",
			wantSuppress: false,
		},
		{
			name:         "non-seed release inside the window is delivered",
			seedMode:     false,
			releaseDate:  "2026-08-22",
			wantSuppress: false,
		},
		{
			name:         "non-seed release exactly on the cutoff is delivered (inclusive boundary)",
			seedMode:     false,
			releaseDate:  "2026-08-19",
			wantSuppress: false,
		},
		{
			name:         "non-seed release one day before the cutoff is suppressed",
			seedMode:     false,
			releaseDate:  "2026-08-18",
			wantSuppress: true,
		},
		{
			name:         "non-seed future-dated release is delivered",
			seedMode:     false,
			releaseDate:  "2026-08-28",
			wantSuppress: false,
		},
		{
			name:         "non-seed backlog year-only date is suppressed",
			seedMode:     false,
			releaseDate:  "2015",
			wantSuppress: true,
		},
		{
			name:         "non-seed backlog year-month date is suppressed",
			seedMode:     false,
			releaseDate:  "2015-06",
			wantSuppress: true,
		},
		{
			name:         "non-seed partial year date is suppressed",
			seedMode:     false,
			releaseDate:  "2026",
			wantSuppress: true,
		},
		{
			name:         "non-seed absent date is suppressed",
			seedMode:     false,
			releaseDate:  "",
			wantSuppress: true,
		},
		{
			name:         "non-seed malformed short date is suppressed",
			seedMode:     false,
			releaseDate:  "20",
			wantSuppress: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate := newNotifyGate(tt.seedMode, 7, now)
			got := gate.notifiedAt(tt.releaseDate)
			if got.Valid != tt.wantSuppress {
				verb := map[bool]string{true: "suppressed", false: "delivered"}
				t.Fatalf("notifiedAt(%q) with seedMode=%v was %s, want %s",
					tt.releaseDate, tt.seedMode, verb[got.Valid], verb[tt.wantSuppress])
			}
		})
	}
}

// TestNotifyGate_SeedRowsShareOneTimestamp pins D-13: every row a single
// seed cycle inserts must carry ONE timestamp, never a per-row time.Now()
// re-read. The gate captures its stamp once at construction, so this holds
// for freshness-suppressed rows too.
func TestNotifyGate_SeedRowsShareOneTimestamp(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	gate := newNotifyGate(true, 7, now)

	first := gate.notifiedAt("2020-01-01")
	second := gate.notifiedAt("2026-08-26")
	third := gate.notifiedAt("")

	if !first.Time.Equal(second.Time) || !second.Time.Equal(third.Time) {
		t.Fatalf("seed-cycle rows carry differing timestamps (%v, %v, %v), want one shared timestamp (D-13)",
			first.Time, second.Time, third.Time)
	}
	if !first.Time.Equal(now) {
		t.Fatalf("seed timestamp = %v, want the captured now %v", first.Time, now)
	}
}

// TestDetectMusicBrainz_NonSeedCycle_BacklogNewReleaseNeverGoesPending is
// the new_release half of the regression: on a cycle where the artist is
// already out of seed mode, a back-catalogue release-group must be
// recorded in history but must NOT be queued for Discord, while a release
// dated today must be.
func TestDetectMusicBrainz_NonSeedCycle_BacklogNewReleaseNeverGoesPending(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := filterTestMBID(t)
	artistID := insertFilterTestArtist(t, pool, mbid, "Backlog Artist")
	insertPriorEvent(t, pool, artistID, mbid+"-prior")

	today := time.Now().UTC().Format(time.DateOnly)
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-old", Title: "Ancient Album", PrimaryType: "Album", FirstReleaseDate: "2015-05-21"},
		{MBID: mbid + "-new", Title: "Todays Album", PrimaryType: "Album", FirstReleaseDate: today},
	}

	d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})
	entry := watchlist.Entry{
		ArtistID:     artistID,
		MBID:         mbid,
		Name:         "Backlog Artist",
		ReleaseTypes: []string{"album", "single", "ep"},
	}
	if err := d.DetectMusicBrainz(ctx, filterTestLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	// Both rows must exist -- the fix suppresses DELIVERY, never detection:
	// the History feed must still show the back catalogue.
	//
	// Matched on the two exact external ids rather than a `LIKE mbid-%`
	// prefix, because insertPriorEvent's own row (mbid+"-prior") is also a
	// new_release under the same prefix and would inflate the count.
	var total int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND external_id IN ($2, $3)",
		artistID, mbid+"-old", mbid+"-new",
	).Scan(&total); err != nil {
		t.Fatalf("count new_release rows: %v", err)
	}
	if total != 2 {
		t.Fatalf("new_release rows recorded = %d, want 2 (both the backlog and the fresh release must still be detected)", total)
	}

	pending := pendingExternalIDs(t, pool, artistID)
	if _, ok := pending[mbid+"-old"]; ok {
		t.Error("the 2015 backlog release is queued for Discord delivery, want suppressed")
	}
	if _, ok := pending[mbid+"-new"]; !ok {
		t.Errorf("today's release is NOT queued for Discord delivery, want delivered (pending set: %v)", pending)
	}
	if len(pending) != 1 {
		t.Fatalf("pending rows = %d, want exactly 1 (today's release only)", len(pending))
	}
}

// TestDetectGuestFeatures_PastLookupCap_BacklogNeverGoesPending reproduces
// the exact production failure: an already-watched artist (NOT in seed
// mode) whose guest-feature catalogue exceeds
// maxNewGuestFeatureLookupsPerCycle. Before the fix every one of these
// back-catalogue rows was inserted with notified_at = NULL and delivered
// to Discord.
func TestDetectGuestFeatures_PastLookupCap_BacklogNeverGoesPending(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := filterTestMBID(t)
	artistID := insertFilterTestArtist(t, pool, mbid, "Featured Artist")
	insertPriorEvent(t, pool, artistID, mbid+"-prior")

	// More recordings than the per-cycle lookup cap, so this mirrors the
	// production shape where the remainder spills into later, non-seed
	// cycles.
	const backlogRecordings = maxNewGuestFeatureLookupsPerCycle + 5
	primary := mkGateCredit("other-artist-mbid", "Primary Artist")
	recordings := make([]musicbrainz.Recording, 0, backlogRecordings)
	releases := map[string][]musicbrainz.RecordingRelease{}
	for i := range backlogRecordings {
		recMBID := fmt.Sprintf("%s-rec-%02d", mbid, i)
		recordings = append(recordings, musicbrainz.Recording{
			MBID:         recMBID,
			Title:        fmt.Sprintf("Old Feature %02d", i),
			ArtistCredit: []musicbrainz.ArtistCreditEntry{primary},
		})
		releases[recMBID] = []musicbrainz.RecordingRelease{
			{MBID: recMBID + "-rel", Title: "Old Album", Date: fmt.Sprintf("201%d-03-11", i%10)},
		}
	}

	d := New(sqlc.New(pool), datedRecordingSource{recordings: recordings, releases: releases}, noReleaseDetailSource{})
	entry := watchlist.Entry{
		ArtistID:     artistID,
		MBID:         mbid,
		Name:         "Featured Artist",
		ReleaseTypes: []string{"album", "single", "ep"},
	}
	if err := d.DetectMusicBrainz(ctx, filterTestLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var recorded int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'guest_feature'",
		artistID,
	).Scan(&recorded); err != nil {
		t.Fatalf("count guest_feature rows: %v", err)
	}
	if recorded == 0 {
		t.Fatal("no guest_feature rows recorded at all -- the test fixture is not exercising the guest-feature pass")
	}

	if got := pendingCount(t, pool, artistID); got != 0 {
		t.Fatalf("back-catalogue guest features queued for Discord delivery = %d, want 0 (recorded %d rows in history, all dated 2010-2019)", got, recorded)
	}
}

// TestDetectGuestFeatures_NonSeedCycle_FreshFeatureStillDelivered is the
// counterweight to the test above: the fix must not suppress everything.
// A genuinely-new guest feature on a non-seed cycle -- the entire point of
// the product -- must still reach Discord.
func TestDetectGuestFeatures_NonSeedCycle_FreshFeatureStillDelivered(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := filterTestMBID(t)
	artistID := insertFilterTestArtist(t, pool, mbid, "Fresh Feature Artist")
	insertPriorEvent(t, pool, artistID, mbid+"-prior")

	today := time.Now().UTC().Format(time.DateOnly)
	recMBID := mbid + "-rec-fresh"
	recordings := []musicbrainz.Recording{{
		MBID:         recMBID,
		Title:        "Brand New Feature",
		ArtistCredit: []musicbrainz.ArtistCreditEntry{mkGateCredit("other-artist-mbid", "Primary Artist")},
	}}
	releases := map[string][]musicbrainz.RecordingRelease{
		recMBID: {{MBID: recMBID + "-rel", Title: "Brand New Album", Date: today}},
	}

	d := New(sqlc.New(pool), datedRecordingSource{recordings: recordings, releases: releases}, noReleaseDetailSource{})
	entry := watchlist.Entry{
		ArtistID:     artistID,
		MBID:         mbid,
		Name:         "Fresh Feature Artist",
		ReleaseTypes: []string{"album", "single", "ep"},
	}
	if err := d.DetectMusicBrainz(ctx, filterTestLogger(), entry, nil); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	pending := pendingExternalIDs(t, pool, artistID)
	if _, ok := pending[recMBID]; !ok {
		t.Fatalf("a guest feature released today was not queued for Discord delivery (pending set: %v) -- the freshness gate is over-suppressing", pending)
	}
}

// mkGateCredit mirrors detector_test.go's mkCredit, redeclared here because
// that helper lives in package detection_test (blackbox) and is not visible
// from this whitebox file.
func mkGateCredit(mbid, name string) musicbrainz.ArtistCreditEntry {
	var e musicbrainz.ArtistCreditEntry
	e.Name = name
	e.Artist.MBID = mbid
	e.Artist.Name = name
	return e
}
