package detection

// This file is package detection (whitebox, not detection_test) since
// releaseTypeAllowed/deluxeDetectionEnabled/eventTypeMuted are unexported --
// mirrors internal/watchlist's normalize_test.go convention for testing an
// unexported function in a separate file.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

func TestFilter_ReleaseTypeAllowed(t *testing.T) {
	tests := []struct {
		name         string
		releaseTypes []string
		primaryType  string
		want         bool
	}{
		{
			name:         "Album against full preference set is allowed",
			releaseTypes: []string{"album", "single", "ep", "deluxe"},
			primaryType:  "Album",
			want:         true,
		},
		{
			name:         "Album against a preference set that excludes it is not allowed",
			releaseTypes: []string{"single"},
			primaryType:  "Album",
			want:         false,
		},
		{
			name:         "already-lowercase album is allowed",
			releaseTypes: []string{"album"},
			primaryType:  "album",
			want:         true,
		},
		{
			name:         "EP against a preference set containing ep is allowed",
			releaseTypes: []string{"ep"},
			primaryType:  "EP",
			want:         true,
		},
		{
			name:         "Broadcast is never allowed regardless of preference",
			releaseTypes: []string{"album", "single", "ep", "deluxe"},
			primaryType:  "Broadcast",
			want:         false,
		},
		{
			name:         "Other is never allowed regardless of preference",
			releaseTypes: []string{"album", "single", "ep", "deluxe"},
			primaryType:  "Other",
			want:         false,
		},
		{
			name:         "empty string is never allowed",
			releaseTypes: []string{"album", "single", "ep", "deluxe"},
			primaryType:  "",
			want:         false,
		},
		{
			name:         "an entry with an empty ReleaseTypes slice allows nothing",
			releaseTypes: []string{},
			primaryType:  "Album",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := watchlist.Entry{ReleaseTypes: tt.releaseTypes}
			if got := releaseTypeAllowed(entry, tt.primaryType); got != tt.want {
				t.Fatalf("releaseTypeAllowed(entry{ReleaseTypes:%v}, %q) = %v, want %v", tt.releaseTypes, tt.primaryType, got, tt.want)
			}
		})
	}
}

func TestFilter_DeluxeIsAGateNotAType(t *testing.T) {
	tests := []struct {
		name            string
		releaseTypes    []string
		wantGateEnabled bool
	}{
		{
			name:            "deluxe present enables the gate",
			releaseTypes:    []string{"album", "deluxe"},
			wantGateEnabled: true,
		},
		{
			name:            "deluxe absent disables the gate",
			releaseTypes:    []string{"album", "single", "ep"},
			wantGateEnabled: false,
		},
		{
			name:            "empty ReleaseTypes disables the gate",
			releaseTypes:    []string{},
			wantGateEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := watchlist.Entry{ReleaseTypes: tt.releaseTypes}
			if got := deluxeDetectionEnabled(entry); got != tt.wantGateEnabled {
				t.Fatalf("deluxeDetectionEnabled(entry{ReleaseTypes:%v}) = %v, want %v", tt.releaseTypes, got, tt.wantGateEnabled)
			}

			// releaseTypeAllowed(entry, "deluxe") must be false for every
			// entry, including one whose ReleaseTypes contains "deluxe" --
			// "deluxe" is never a MusicBrainz primary-type value, so it can
			// never pass detectableReleaseTypes regardless of preference.
			if got := releaseTypeAllowed(entry, "deluxe"); got {
				t.Fatalf("releaseTypeAllowed(entry{ReleaseTypes:%v}, \"deluxe\") = true, want false", tt.releaseTypes)
			}
		})
	}
}

func TestFilter_EventTypeMuted(t *testing.T) {
	tests := []struct {
		name            string
		mutedEventTypes []string
		eventType       string
		want            bool
	}{
		{
			name:            "new_release present in muted types is muted",
			mutedEventTypes: []string{"new_release"},
			eventType:       "new_release",
			want:            true,
		},
		{
			name:            "guest_feature present in muted types is muted",
			mutedEventTypes: []string{"guest_feature", "deluxe_change"},
			eventType:       "guest_feature",
			want:            true,
		},
		{
			name:            "deluxe_change present in muted types is muted",
			mutedEventTypes: []string{"guest_feature", "deluxe_change"},
			eventType:       "deluxe_change",
			want:            true,
		},
		{
			name:            "event type absent from muted types is not muted",
			mutedEventTypes: []string{"guest_feature"},
			eventType:       "new_release",
			want:            false,
		},
		{
			name:            "nil MutedEventTypes never mutes",
			mutedEventTypes: nil,
			eventType:       "new_release",
			want:            false,
		},
		{
			name:            "empty MutedEventTypes never mutes",
			mutedEventTypes: []string{},
			eventType:       "new_release",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := watchlist.Entry{MutedEventTypes: tt.mutedEventTypes}
			if got := eventTypeMuted(entry, tt.eventType); got != tt.want {
				t.Fatalf("eventTypeMuted(entry{MutedEventTypes:%v}, %q) = %v, want %v", tt.mutedEventTypes, tt.eventType, got, tt.want)
			}
		})
	}
}

// The two tests below are real-Postgres integration assertions proving the
// filter predicates are actually wired into DetectMusicBrainz (D-17, D-18),
// not just correct in isolation. They live in this whitebox file (package
// detection, not detection_test) per this task's own file list, so they
// duplicate detector_test.go's small fixture-helper convention locally
// rather than importing across the internal/external test package split.

// filterTestMBID derives a short, unique-per-test artist mbid from t.Name(),
// matching detector_test.go's testMBID convention.
func filterTestMBID(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

// filterTestLogger is a discard-output *slog.Logger, matching
// detector_test.go's testLogger convention.
func filterTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// insertFilterTestArtist inserts a minimal artists row directly and
// registers its cleanup, matching detector_test.go's insertTestArtist
// convention.
func insertFilterTestArtist(t *testing.T, pool *pgxpool.Pool, mbid, name string) int64 {
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

func TestDetectMusicBrainz_FiltersByReleaseType(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := filterTestMBID(t)
	artistID := insertFilterTestArtist(t, pool, mbid, "Filter Release Type Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Filter Release Type Artist", ReleaseTypes: []string{"album"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "Album Release", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Single Release", PrimaryType: "Single"},
	}

	d := New(sqlc.New(pool))
	if err := d.DetectMusicBrainz(ctx, filterTestLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 1 {
		t.Fatalf("event row count = %d, want 1 (only the Album group should pass the release_types filter)", count)
	}

	var externalID string
	if err := pool.QueryRow(ctx, "SELECT external_id FROM events WHERE artist_id = $1", artistID).Scan(&externalID); err != nil {
		t.Fatalf("query surviving row: %v", err)
	}
	if externalID != mbid+"-rg1" {
		t.Fatalf("surviving row external_id = %q, want %q", externalID, mbid+"-rg1")
	}
}

func TestDetectMusicBrainz_SkipsMutedEventType(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := filterTestMBID(t)
	artistID := insertFilterTestArtist(t, pool, mbid, "Skips Muted Event Type Artist")

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Skips Muted Event Type Artist", MutedEventTypes: []string{"new_release"}}
	groups := []musicbrainz.ReleaseGroup{
		{MBID: mbid + "-rg1", Title: "First Album", PrimaryType: "Album"},
		{MBID: mbid + "-rg2", Title: "Second Album", PrimaryType: "Album"},
	}

	d := New(sqlc.New(pool))
	if err := d.DetectMusicBrainz(ctx, filterTestLogger(), entry, groups); err != nil {
		t.Fatalf("DetectMusicBrainz: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM events WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 0 {
		t.Fatalf("event row count = %d, want 0 (new_release is muted, no rows should be created)", count)
	}
}
