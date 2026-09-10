package pollruns_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/pollruns"
)

// mkRun builds a RunResult whose CycleID and three counters all encode seq, so
// a torn read (a copy assembled from two different records) is detectable by
// the fields disagreeing with each other.
func mkRun(src string, seq int) pollruns.RunResult {
	return pollruns.RunResult{
		Source:         src,
		CycleID:        fmt.Sprintf("%s-%d", src, seq),
		ArtistsChecked: seq,
		ArtistsErrored: seq,
		EventsRecorded: seq,
		Outcome:        pollruns.OutcomeOK,
	}
}

func TestStore_RingBound(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()

	for i := 0; i < 60; i++ {
		if err := s.RecordRun(ctx, mkRun(pollruns.SourceMusicBrainz, i)); err != nil {
			t.Fatalf("RecordRun %d: %v", i, err)
		}
	}

	snap := s.Snapshot()[pollruns.SourceMusicBrainz]
	if len(snap.History) != pollruns.N {
		t.Fatalf("history len = %d, want exactly %d", len(snap.History), pollruns.N)
	}
	if got := snap.History[0].CycleID; got != "musicbrainz-59" {
		t.Fatalf("History[0].CycleID = %q, want musicbrainz-59 (newest first)", got)
	}
	if got := snap.History[len(snap.History)-1].CycleID; got != "musicbrainz-10" {
		t.Fatalf("oldest retained CycleID = %q, want musicbrainz-10 (last 50 kept)", got)
	}
	if snap.LastRun == nil {
		t.Fatal("LastRun is nil")
	}
	if snap.LastRun.CycleID != snap.History[0].CycleID {
		t.Fatalf("LastRun.CycleID = %q, want %q (== History[0])", snap.LastRun.CycleID, snap.History[0].CycleID)
	}
}

func TestStore_NewestFirst(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := s.RecordRun(ctx, mkRun(pollruns.SourceDeezer, i)); err != nil {
			t.Fatalf("RecordRun %d: %v", i, err)
		}
	}

	snap := s.Snapshot()[pollruns.SourceDeezer]
	want := []string{"deezer-2", "deezer-1", "deezer-0"}
	if len(snap.History) != len(want) {
		t.Fatalf("history len = %d, want %d", len(snap.History), len(want))
	}
	for i, id := range want {
		if snap.History[i].CycleID != id {
			t.Fatalf("History[%d].CycleID = %q, want %q", i, snap.History[i].CycleID, id)
		}
	}
	if snap.LastRun == nil || snap.LastRun.CycleID != "deezer-2" {
		t.Fatalf("LastRun = %+v, want CycleID deezer-2", snap.LastRun)
	}
}

func TestStore_EmptyStoreSnapshot(t *testing.T) {
	snap := pollruns.NewStore().Snapshot()

	for _, src := range pollruns.KnownSources {
		ss, ok := snap[src]
		if !ok {
			t.Fatalf("snapshot missing known source %q", src)
		}
		if ss.History == nil {
			t.Fatalf("%s: History is nil, want a non-nil empty slice", src)
		}
		if len(ss.History) != 0 {
			t.Fatalf("%s: History len = %d, want 0", src, len(ss.History))
		}
		if ss.LastRun != nil {
			t.Fatalf("%s: LastRun = %+v, want nil", src, ss.LastRun)
		}
		if ss.LastSkippedAt != nil {
			t.Fatalf("%s: LastSkippedAt = %v, want nil", src, ss.LastSkippedAt)
		}
		if ss.ConsecutiveSkips != 0 {
			t.Fatalf("%s: ConsecutiveSkips = %d, want 0", src, ss.ConsecutiveSkips)
		}
	}
}

func TestStore_EqualStartedAtKeepsInsertionOrder(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()
	ts := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	first := mkRun(pollruns.SourceDeezer, 1)
	first.StartedAt = ts
	second := mkRun(pollruns.SourceDeezer, 2)
	second.StartedAt = ts

	if err := s.RecordRun(ctx, first); err != nil {
		t.Fatalf("RecordRun first: %v", err)
	}
	if err := s.RecordRun(ctx, second); err != nil {
		t.Fatalf("RecordRun second: %v", err)
	}

	snap := s.Snapshot()[pollruns.SourceDeezer]
	if len(snap.History) != 2 {
		t.Fatalf("history len = %d, want 2", len(snap.History))
	}
	if snap.History[0].CycleID != "deezer-2" || snap.History[1].CycleID != "deezer-1" {
		t.Fatalf("order = [%q, %q], want [deezer-2, deezer-1] — reverse insertion, never sorted on StartedAt",
			snap.History[0].CycleID, snap.History[1].CycleID)
	}
}

func TestStore_SnapshotDoesNotAlias(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := s.RecordRun(ctx, mkRun(pollruns.SourceMusicBrainz, i)); err != nil {
			t.Fatalf("RecordRun %d: %v", i, err)
		}
	}

	held := s.Snapshot()[pollruns.SourceMusicBrainz].History
	if len(held) != 5 {
		t.Fatalf("held history len = %d, want 5", len(held))
	}
	heldFirst := held[0].CycleID

	// Enough further records to trim the ring (5 + 60 = 65 -> 50).
	for i := 5; i < 65; i++ {
		if err := s.RecordRun(ctx, mkRun(pollruns.SourceMusicBrainz, i)); err != nil {
			t.Fatalf("RecordRun %d: %v", i, err)
		}
	}

	if len(held) != 5 {
		t.Fatalf("held slice length changed to %d — Snapshot returned a slice aliasing the ring", len(held))
	}
	if held[0].CycleID != heldFirst {
		t.Fatalf("held slice first element changed from %q to %q — Snapshot aliased the ring", heldFirst, held[0].CycleID)
	}
}

func TestStore_Skip(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()

	s.RecordSkip(pollruns.SourceMusicBrainz)
	snap := s.Snapshot()[pollruns.SourceMusicBrainz]
	if snap.ConsecutiveSkips != 1 {
		t.Fatalf("after one skip: ConsecutiveSkips = %d, want 1", snap.ConsecutiveSkips)
	}
	if snap.LastSkippedAt == nil || snap.LastSkippedAt.IsZero() {
		t.Fatalf("after one skip: LastSkippedAt = %v, want a non-zero stamp", snap.LastSkippedAt)
	}
	if len(snap.History) != 0 {
		t.Fatalf("after one skip: History len = %d, want 0 (a skip is a signal, not a run)", len(snap.History))
	}

	s.RecordSkip(pollruns.SourceMusicBrainz)
	s.RecordSkip(pollruns.SourceMusicBrainz)
	snap = s.Snapshot()[pollruns.SourceMusicBrainz]
	if snap.ConsecutiveSkips != 3 {
		t.Fatalf("after three skips: ConsecutiveSkips = %d, want 3", snap.ConsecutiveSkips)
	}
	stampBefore := *snap.LastSkippedAt

	if err := s.RecordRun(ctx, mkRun(pollruns.SourceMusicBrainz, 0)); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	snap = s.Snapshot()[pollruns.SourceMusicBrainz]
	if snap.ConsecutiveSkips != 0 {
		t.Fatalf("after a run: ConsecutiveSkips = %d, want 0 (the run resets the counter)", snap.ConsecutiveSkips)
	}
	if snap.LastSkippedAt == nil || !snap.LastSkippedAt.Equal(stampBefore) {
		t.Fatalf("after a run: LastSkippedAt = %v, want it still %v (the run resets the count but never clears the stamp)", snap.LastSkippedAt, stampBefore)
	}
	if len(snap.History) != 1 {
		t.Fatalf("after a run: History len = %d, want 1", len(snap.History))
	}
}

func TestStore_SkipIsPerSource(t *testing.T) {
	s := pollruns.NewStore()

	s.RecordSkip(pollruns.SourceMusicBrainz)
	s.RecordSkip(pollruns.SourceMusicBrainz)

	snap := s.Snapshot()
	if got := snap[pollruns.SourceMusicBrainz].ConsecutiveSkips; got != 2 {
		t.Fatalf("musicbrainz ConsecutiveSkips = %d, want 2", got)
	}
	dz := snap[pollruns.SourceDeezer]
	if dz.ConsecutiveSkips != 0 {
		t.Fatalf("deezer ConsecutiveSkips = %d, want 0 (one source's skip must not touch the other)", dz.ConsecutiveSkips)
	}
	if dz.LastSkippedAt != nil {
		t.Fatalf("deezer LastSkippedAt = %v, want nil", dz.LastSkippedAt)
	}
}

func TestStore_SummaryAlwaysComposed(t *testing.T) {
	s := pollruns.NewStore()
	ctx := context.Background()

	const secret = "postgres://user:hunter2@db.internal:5432/drop_tracker?sslmode=disable"
	r := pollruns.RunResult{
		Source:         pollruns.SourceDeezer,
		CycleID:        "deezer-7",
		ArtistsChecked: 9,
		ArtistsErrored: 4,
		EventsRecorded: 0,
		Outcome:        pollruns.OutcomeError,
		Summary:        secret,
	}
	if err := s.RecordRun(ctx, r); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	snap := s.Snapshot()[pollruns.SourceDeezer]
	const want = "error — 9 checked, 4 errored, 0 events"
	if snap.LastRun.Summary != want {
		t.Fatalf("Summary = %q, want %q (composed from counts + outcome, never the caller's text)", snap.LastRun.Summary, want)
	}
	rendered := fmt.Sprintf("%+v", snap)
	if strings.Contains(rendered, "hunter2") || strings.Contains(rendered, secret) {
		t.Fatalf("snapshot leaked the caller-supplied Summary text: %s", rendered)
	}
}

func TestStore_OutcomeNormalized(t *testing.T) {
	ctx := context.Background()

	t.Run("unknown normalizes to error", func(t *testing.T) {
		s := pollruns.NewStore()
		r := mkRun(pollruns.SourceMusicBrainz, 1)
		r.Outcome = "kaboom"
		if err := s.RecordRun(ctx, r); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
		if got := s.Snapshot()[pollruns.SourceMusicBrainz].LastRun.Outcome; got != pollruns.OutcomeError {
			t.Fatalf("Outcome = %q, want %q", got, pollruns.OutcomeError)
		}
	})

	for _, oc := range []string{pollruns.OutcomeOK, pollruns.OutcomeError, pollruns.OutcomeCancelled} {
		t.Run(oc+" round-trips unchanged", func(t *testing.T) {
			s := pollruns.NewStore()
			r := mkRun(pollruns.SourceMusicBrainz, 1)
			r.Outcome = oc
			if err := s.RecordRun(ctx, r); err != nil {
				t.Fatalf("RecordRun: %v", err)
			}
			if got := s.Snapshot()[pollruns.SourceMusicBrainz].LastRun.Outcome; got != oc {
				t.Fatalf("Outcome = %q, want %q", got, oc)
			}
		})
	}
}

func TestStore_TwoSourceConcurrent(t *testing.T) {
	// go test -race is unavailable on this box and absent from CI, so exact
	// equality plus repetition stands in for the detector: 1000 iterations,
	// each a fresh store, two goroutines recording their own source, then
	// exact-equality assertions (not range checks) on per-source length,
	// order, and every entry's internal self-consistency (RUN-04).
	const iterations = 1000
	const perSource = 60
	ctx := context.Background()
	sources := []string{pollruns.SourceMusicBrainz, pollruns.SourceDeezer}

	for iter := 0; iter < iterations; iter++ {
		s := pollruns.NewStore()

		var wg sync.WaitGroup
		for _, src := range sources {
			wg.Add(1)
			go func(src string) {
				defer wg.Done()
				for i := 0; i < perSource; i++ {
					if err := s.RecordRun(ctx, mkRun(src, i)); err != nil {
						t.Errorf("iter %d: RecordRun(%s, %d): %v", iter, src, i, err)
						return
					}
				}
			}(src)
		}
		wg.Wait()

		snap := s.Snapshot()
		for _, src := range sources {
			h := snap[src].History
			if len(h) != pollruns.N {
				t.Fatalf("iter %d: %s history len = %d, want exactly %d", iter, src, len(h), pollruns.N)
			}
			for pos, r := range h {
				wantSeq := perSource - 1 - pos // newest-first: 59, 58, ..., 10
				wantID := fmt.Sprintf("%s-%d", src, wantSeq)
				if r.Source != src {
					t.Fatalf("iter %d: %s history[%d].Source = %q (interleaved from the other goroutine)", iter, src, pos, r.Source)
				}
				if r.CycleID != wantID {
					t.Fatalf("iter %d: %s history[%d].CycleID = %q, want %q", iter, src, pos, r.CycleID, wantID)
				}
				if r.ArtistsChecked != wantSeq || r.ArtistsErrored != wantSeq || r.EventsRecorded != wantSeq {
					t.Fatalf("iter %d: %s history[%d] torn read: checked=%d errored=%d events=%d, want all %d",
						iter, src, pos, r.ArtistsChecked, r.ArtistsErrored, r.EventsRecorded, wantSeq)
				}
			}
		}
	}
}
