package detection_test

// Real-Postgres coverage of DetectDeezer's new_release diff, mirroring
// detector_test.go's conventions (testMBID/testLogger/insertTestArtist are
// shared package-level helpers defined there, this file is the same
// detection_test package).

import (
	"context"
	"strconv"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

func TestDetectDeezer_NewRelease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deezer New Release Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deezer New Release Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	albums := []deezer.Album{
		{ID: 111, Title: "First Album", RecordType: "album", Cover: "https://example.test/111.jpg", ReleaseDate: "2020-01-01"},
		{ID: 222, Title: "Second Album", RecordType: "album", Cover: "https://example.test/222.jpg", ReleaseDate: "2021-06-15"},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{})
	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("DetectDeezer: %v", err)
	}

	rows, err := pool.Query(ctx, `SELECT external_id, release_group_mbid, title, artist_name,
		release_date, cover_art_url, event_type, source
		FROM events WHERE artist_id = $1 ORDER BY external_id`, artistID)
	if err != nil {
		t.Fatalf("query events: %v", err)
	}
	defer rows.Close()

	type row struct {
		externalID, title, artistName              string
		releaseGroupMbid, releaseDate, coverArtURL *string
		eventType, source                          string
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
		externalID, title, releaseDate, coverArtURL string
	}{
		{"111", "First Album", "2020-01-01", "https://example.test/111.jpg"},
		{"222", "Second Album", "2021-06-15", "https://example.test/222.jpg"},
	}
	for i, w := range want {
		if got[i].externalID != w.externalID {
			t.Fatalf("row %d external_id = %q, want %q", i, got[i].externalID, w.externalID)
		}
		if got[i].releaseGroupMbid != nil {
			t.Fatalf("row %d release_group_mbid = %v, want NULL (Deezer has no release-group concept)", i, *got[i].releaseGroupMbid)
		}
		if got[i].title != w.title {
			t.Fatalf("row %d title = %q, want %q", i, got[i].title, w.title)
		}
		if got[i].artistName != "Deezer New Release Artist" {
			t.Fatalf("row %d artist_name = %q, want %q", i, got[i].artistName, "Deezer New Release Artist")
		}
		if got[i].releaseDate == nil || *got[i].releaseDate != w.releaseDate {
			t.Fatalf("row %d release_date = %v, want %q", i, got[i].releaseDate, w.releaseDate)
		}
		if got[i].coverArtURL == nil || *got[i].coverArtURL != w.coverArtURL {
			t.Fatalf("row %d cover_art_url = %v, want %q", i, got[i].coverArtURL, w.coverArtURL)
		}
		if got[i].eventType != "new_release" {
			t.Fatalf("row %d event_type = %q, want new_release", i, got[i].eventType)
		}
		if got[i].source != "deezer" {
			t.Fatalf("row %d source = %q, want deezer", i, got[i].source)
		}
	}
}

func TestDetectDeezer_ReDetectionInsertsNothing(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deezer Re-Detection Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deezer Re-Detection Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	albums := []deezer.Album{
		{ID: 1, Title: "Album One", RecordType: "album"},
		{ID: 2, Title: "Album Two", RecordType: "album"},
	}
	d := detection.New(sqlc.New(pool), fakeRecordingSource{})

	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("first DetectDeezer: %v", err)
	}
	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("second DetectDeezer: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (re-detection must insert nothing new)", count)
	}
}

func TestDetectDeezer_FiltersByRecordType(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Deezer Filter Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deezer Filter Artist", ReleaseTypes: []string{"album"}}
	albums := []deezer.Album{
		{ID: 1, Title: "Album", RecordType: "album"},
		{ID: 2, Title: "Single", RecordType: "single"},
		{ID: 3, Title: "Compilation", RecordType: "compilation"},
	}

	d := detection.New(sqlc.New(pool), fakeRecordingSource{})
	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("DetectDeezer: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event row count = %d, want 1 (only the album record_type passes ReleaseTypes=[album]; single and compilation must be filtered out)", count)
	}

	var externalID string
	if err := pool.QueryRow(ctx, "SELECT external_id FROM events WHERE artist_id = $1", artistID).Scan(&externalID); err != nil {
		t.Fatalf("query surviving row: %v", err)
	}
	if externalID != "1" {
		t.Fatalf("surviving row external_id = %q, want %q", externalID, "1")
	}
}

func TestDetectDeezer_SeedsIndependentlyOfMusicBrainz(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Cross Source Seed Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Cross Source Seed Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	d := detection.New(sqlc.New(pool), fakeRecordingSource{})

	mbGroups := []musicbrainz.ReleaseGroup{{MBID: mbid + "-rg1", Title: "MB Album", PrimaryType: "Album"}}
	if err := d.DetectMusicBrainz(ctx, testLogger(), entry, mbGroups); err != nil {
		t.Fatalf("seed DetectMusicBrainz: %v", err)
	}

	albums := []deezer.Album{{ID: 1, Title: "Deezer Album", RecordType: "album"}}
	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("DetectDeezer: %v", err)
	}

	var notifiedAt *string
	if err := pool.QueryRow(ctx, "SELECT notified_at::text FROM events WHERE artist_id = $1 AND source = 'deezer'", artistID).Scan(&notifiedAt); err != nil {
		t.Fatalf("query deezer row: %v", err)
	}
	if notifiedAt == nil {
		t.Fatal("deezer row notified_at is NULL, want non-NULL -- the first deezer cycle must seed independently of an already-seeded musicbrainz source (D-15)")
	}
}

func TestDetectDeezer_SameIDDifferentSourceCoexist(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testMBID(t)
	artistID := insertTestArtist(t, pool, mbid, "Same ID Different Source Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Same ID Different Source Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	sharedExternalID := int64(123456)

	q := sqlc.New(pool)
	// Seed an existing musicbrainz row directly through InsertEvent (not
	// DetectMusicBrainz) whose external_id happens to be byte-equal to the
	// Deezer album id string below -- a real MusicBrainz MBID would never
	// literally be an all-digit string, but T-04-08's mitigation must hold
	// regardless of how implausible the collision is.
	if _, err := q.InsertEvent(ctx, sqlc.InsertEventParams{
		ArtistID:   artistID,
		Source:     "musicbrainz",
		EventType:  "new_release",
		ExternalID: strconv.FormatInt(sharedExternalID, 10),
		Title:      "MB Album",
		ArtistName: entry.Name,
	}); err != nil {
		t.Fatalf("seed musicbrainz InsertEvent: %v", err)
	}

	d := detection.New(q, fakeRecordingSource{})
	albums := []deezer.Album{{ID: sharedExternalID, Title: "Deezer Album", RecordType: "album"}}
	if err := d.DetectDeezer(ctx, testLogger(), entry, albums); err != nil {
		t.Fatalf("DetectDeezer: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1 AND external_id = $2", artistID, strconv.FormatInt(sharedExternalID, 10)).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (source must separate the dedup namespace)", count)
	}
}
