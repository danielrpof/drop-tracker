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

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Test Artist"}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "First Album", FirstReleaseDate: "2020-01-01"},
		{MBID: mbid + "-rg2", Title: "Second Album", FirstReleaseDate: "2021-06-15"},
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

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Undated Group Artist"}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Undated Album", FirstReleaseDate: ""},
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
