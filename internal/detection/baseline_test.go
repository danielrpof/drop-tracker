package detection

// This file is package detection (whitebox, not detection_test) since
// advanceGroupBaseline is unexported -- mirrors filter_test.go's own
// convention for testing an unexported method that needs real-Postgres
// coverage (11-RESEARCH.md's plan text describes this as living alongside
// detector_test.go's helpers, but detector_test.go/deezer_test.go are
// package detection_test (blackbox), which cannot see an unexported
// method; filter_test.go already establishes the whitebox real-Postgres
// pattern this file follows, reusing its filterTestMBID/
// insertFilterTestArtist helpers and noRecordingSource/
// noReleaseDetailSource doubles rather than duplicating them).
//
// PERF-04's canonical proof: 11-RESEARCH.md Pattern 2 (a FOR UPDATE
// locking CTE plus UPDATE...RETURNING, wrapped by advanceGroupBaseline)
// replaces the former two-statement groupBaseline SELECT + setGroupBaseline
// UPDATE, closing a lost-update race between concurrent callers racing the
// same release group. TestAdvanceGroupBaseline_ConcurrentRace is the
// deliberate proof: it races callers and asserts on the value read back
// from the database, not on the functions' return values alone -- the
// failure this test exists to catch is a lost update, which leaves every
// caller reporting success while the database holds the wrong number.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// int32Ptr returns a pointer to a freshly allocated copy of v, mirroring
// detector_test.go's nullableStringForTest convention for *int32 instead
// of *string.
func int32Ptr(v int32) *int32 { return &v }

// insertBaselineNewRelease inserts a minimal musicbrainz new_release event
// row keyed on externalID, with trackCount as its starting track_count
// (nil encodes "no baseline recorded yet" -- SQL NULL). release_group_mbid
// is set to externalID too, matching what a real musicbrainz DetectMusicBrainz
// insert does (the new_release row's own external_id and release_group_mbid
// hold the same release-group MBID), even though advanceGroupBaseline only
// ever reads/writes by external_id.
func insertBaselineNewRelease(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID string, trackCount *int32) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (artist_id, source, event_type, external_id, release_group_mbid, title, artist_name, track_count)
		VALUES ($1, 'musicbrainz', 'new_release', $2, $2, 'Baseline Test Release', 'Baseline Test Artist', $3)`,
		artistID, externalID, trackCount); err != nil {
		t.Fatalf("insert baseline test new_release row: %v", err)
	}
}

// readBaselineTrackCount reads back the stored track_count for externalID
// directly from the database -- the read-back this file's tests assert
// final state against, independent of whatever advanceGroupBaseline itself
// reported.
func readBaselineTrackCount(t *testing.T, pool *pgxpool.Pool, externalID string) *int32 {
	t.Helper()
	ctx := context.Background()
	var trackCount *int32
	if err := pool.QueryRow(ctx,
		"SELECT track_count FROM events WHERE event_type = 'new_release' AND source = 'musicbrainz' AND external_id = $1",
		externalID,
	).Scan(&trackCount); err != nil {
		t.Fatalf("read back track_count: %v", err)
	}
	return trackCount
}

// TestAdvanceGroupBaseline_ConcurrentRace is the phase's canonical proof
// for ROADMAP criterion 4 (PERF-04): a deliberate race between callers
// sharing one release group must never lose the higher count.
func TestAdvanceGroupBaseline_ConcurrentRace(t *testing.T) {
	t.Run("two callers, distinct counts, higher wins", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := filterTestMBID(t)
		artistID := insertFilterTestArtist(t, pool, mbid, "Race Artist")
		groupMBID := mbid + "-group"
		insertBaselineNewRelease(t, pool, artistID, groupMBID, int32Ptr(3))

		d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if _, _, _, err := d.advanceGroupBaseline(ctx, groupMBID, 10); err != nil {
				t.Errorf("advanceGroupBaseline(10): %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if _, _, _, err := d.advanceGroupBaseline(ctx, groupMBID, 12); err != nil {
				t.Errorf("advanceGroupBaseline(12): %v", err)
			}
		}()
		close(start)
		wg.Wait()

		// The assertion is on the value read back from the database, not
		// on the functions' return values alone -- see this file's header
		// comment for why that distinction is the whole point of this test.
		got := readBaselineTrackCount(t, pool, groupMBID)
		if got == nil || *got != 12 {
			t.Fatalf("stored track_count = %v, want 12 (the higher of the two racing counts, regardless of which goroutine ran first)", got)
		}
	})

	t.Run("eight callers, counts 1 through 8, maximum wins", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := filterTestMBID(t)
		artistID := insertFilterTestArtist(t, pool, mbid, "Eight-Way Race Artist")
		groupMBID := mbid + "-group"
		insertBaselineNewRelease(t, pool, artistID, groupMBID, nil)

		d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})

		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 1; i <= 8; i++ {
			wg.Add(1)
			go func(count int) {
				defer wg.Done()
				<-start
				if _, _, _, err := d.advanceGroupBaseline(ctx, groupMBID, count); err != nil {
					t.Errorf("advanceGroupBaseline(%d): %v", count, err)
				}
			}(i)
		}
		close(start)
		wg.Wait()

		got := readBaselineTrackCount(t, pool, groupMBID)
		if got == nil || *got != 8 {
			t.Fatalf("stored track_count = %v, want 8", got)
		}
	})

	t.Run("two callers, identical counts, exactly one advances", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := filterTestMBID(t)
		artistID := insertFilterTestArtist(t, pool, mbid, "Equal-Count Race Artist")
		groupMBID := mbid + "-group"
		insertBaselineNewRelease(t, pool, artistID, groupMBID, nil)

		d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})

		start := make(chan struct{})
		var wg sync.WaitGroup
		var advancedCount int32
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				advanced, _, _, err := d.advanceGroupBaseline(ctx, groupMBID, 7)
				if err != nil {
					t.Errorf("advanceGroupBaseline(7): %v", err)
					return
				}
				if advanced {
					atomic.AddInt32(&advancedCount, 1)
				}
			}()
		}
		close(start)
		wg.Wait()

		if advancedCount != 1 {
			t.Fatalf("callers reporting an advance = %d, want exactly 1 (which one wins is deliberately unspecified)", advancedCount)
		}
		got := readBaselineTrackCount(t, pool, groupMBID)
		if got == nil || *got != 7 {
			t.Fatalf("stored track_count = %v, want 7", got)
		}
	})
}

// TestAdvanceGroupBaseline_SingleCallerContract table-drives the
// single-caller behaviors advanceGroupBaseline must preserve from the
// former groupBaseline+setGroupBaseline pair: each case asserts both the
// returned (advanced, hadBaseline, previousBaseline) triple and the
// read-back stored value.
func TestAdvanceGroupBaseline_SingleCallerContract(t *testing.T) {
	tests := []struct {
		name                 string
		initialTrackCount    *int32
		missingRow           bool
		count                int
		wantAdvanced         bool
		wantHadBaseline      bool
		wantPreviousBaseline int
		wantStoredTrackCount *int32
	}{
		{
			name:                 "NULL starting value establishes",
			initialTrackCount:    nil,
			count:                6,
			wantAdvanced:         true,
			wantHadBaseline:      false,
			wantPreviousBaseline: 0,
			wantStoredTrackCount: int32Ptr(6),
		},
		{
			name:                 "equal count does not advance",
			initialTrackCount:    int32Ptr(6),
			count:                6,
			wantAdvanced:         false,
			wantStoredTrackCount: int32Ptr(6),
		},
		{
			name:                 "lower count does not advance",
			initialTrackCount:    int32Ptr(6),
			count:                4,
			wantAdvanced:         false,
			wantStoredTrackCount: int32Ptr(6),
		},
		{
			name:                 "missing row returns no advance and no error",
			missingRow:           true,
			count:                6,
			wantAdvanced:         false,
			wantStoredTrackCount: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testutil.NewTestPool(t)
			ctx := context.Background()
			mbid := filterTestMBID(t)
			artistID := insertFilterTestArtist(t, pool, mbid, "Single Caller Artist")
			groupMBID := mbid + "-group"
			if !tt.missingRow {
				insertBaselineNewRelease(t, pool, artistID, groupMBID, tt.initialTrackCount)
			}

			d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})
			advanced, hadBaseline, previousBaseline, err := d.advanceGroupBaseline(ctx, groupMBID, tt.count)
			if err != nil {
				t.Fatalf("advanceGroupBaseline: %v", err)
			}
			if advanced != tt.wantAdvanced {
				t.Fatalf("advanced = %v, want %v", advanced, tt.wantAdvanced)
			}
			if hadBaseline != tt.wantHadBaseline {
				t.Fatalf("hadBaseline = %v, want %v", hadBaseline, tt.wantHadBaseline)
			}
			if previousBaseline != tt.wantPreviousBaseline {
				t.Fatalf("previousBaseline = %d, want %d", previousBaseline, tt.wantPreviousBaseline)
			}

			if tt.missingRow {
				return
			}
			got := readBaselineTrackCount(t, pool, groupMBID)
			if tt.wantStoredTrackCount == nil {
				if got != nil {
					t.Fatalf("stored track_count = %v, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.wantStoredTrackCount {
				t.Fatalf("stored track_count = %v, want %v", got, *tt.wantStoredTrackCount)
			}
		})
	}

	t.Run("repeated identical call advances only once", func(t *testing.T) {
		pool := testutil.NewTestPool(t)
		ctx := context.Background()
		mbid := filterTestMBID(t)
		artistID := insertFilterTestArtist(t, pool, mbid, "Repeat Caller Artist")
		groupMBID := mbid + "-group"
		insertBaselineNewRelease(t, pool, artistID, groupMBID, nil)

		d := New(sqlc.New(pool), noRecordingSource{}, noReleaseDetailSource{})

		advanced1, hadBaseline1, _, err := d.advanceGroupBaseline(ctx, groupMBID, 9)
		if err != nil {
			t.Fatalf("first advanceGroupBaseline: %v", err)
		}
		if !advanced1 || hadBaseline1 {
			t.Fatalf("first call: advanced=%v hadBaseline=%v, want advanced=true hadBaseline=false", advanced1, hadBaseline1)
		}

		advanced2, _, _, err := d.advanceGroupBaseline(ctx, groupMBID, 9)
		if err != nil {
			t.Fatalf("second advanceGroupBaseline: %v", err)
		}
		if advanced2 {
			t.Fatal("second call with the same count reported an advance, want no advance (advances on the first call only)")
		}

		got := readBaselineTrackCount(t, pool, groupMBID)
		if got == nil || *got != 9 {
			t.Fatalf("stored track_count = %v, want 9 (unchanged by the second, non-advancing call)", got)
		}
	})
}
