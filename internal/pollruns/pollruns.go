// Package pollruns is the in-process poll-run history: a mutex-guarded ring
// of the last N RunResult values per source, read by GET /status.
// See docs/adr/0001-in-process-ring-buffer-for-poll-run-history.md.
package pollruns

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// N bounds the per-source history ring. 50 is roughly twelve hours of history
// at the default fifteen-minute poll interval (RUN-04: a compile-time
// constant, deliberately not an environment variable).
const N = 50

// Source names -- the values the poll cycle tags each RunResult with.
const (
	SourceMusicBrainz = "musicbrainz"
	SourceDeezer      = "deezer"
)

// KnownSources is pre-seeded by NewStore so every Snapshot carries both keys
// on a fresh instance.
var KnownSources = []string{SourceMusicBrainz, SourceDeezer}

// Outcome values. RecordRun normalizes anything else to OutcomeError, so a
// snapshot's Outcome is always one of these three for the /status contract.
const (
	OutcomeOK        = "ok"
	OutcomeError     = "error"
	OutcomeCancelled = "cancelled"
)

// RunResult is the immutable summary of one completed poll cycle. Summary is
// composed by RecordRun from the counts and the normalized outcome -- never
// from a driver error, upstream body, DSN, webhook URL, or path (D-09).
type RunResult struct {
	Source         string
	CycleID        string
	StartedAt      time.Time
	FinishedAt     time.Time
	DurationMS     int64
	ArtistsChecked int // entries dispatched to a worker
	ArtistsSkipped int
	ArtistsErrored int
	EventsRecorded int
	Outcome        string
	Summary        string
}

type sourceState struct {
	ring             []RunResult // newest-last; len <= N
	lastSkippedAt    time.Time   // zero value = never skipped
	consecutiveSkips int
}

// Store holds the per-source history rings behind one mutex covering the ring,
// the last-skipped stamp, and the consecutive-skip count.
type Store struct {
	mu      sync.Mutex
	sources map[string]*sourceState
}

// NewStore pre-seeds a state for each KnownSources entry, which is what keeps
// the /status body shape stable on a fresh instance.
func NewStore() *Store {
	s := &Store{sources: make(map[string]*sourceState, len(KnownSources))}
	for _, src := range KnownSources {
		s.sources[src] = &sourceState{}
	}
	return s
}

// RecordRun appends r to its source's ring, trims the ring to the newest N,
// and resets that source's consecutive-skip count. ctx is accepted for seam
// symmetry with EventRecorder/Notifier; the append never blocks on it.
func (s *Store) RecordRun(_ context.Context, r RunResult) error {
	r.Outcome = normalizeOutcome(r.Outcome)
	// Summary is recomposed here so a caller-supplied value can never reach a snapshot.
	r.Summary = composeSummary(r)

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stateLocked(r.Source)
	st.ring = append(st.ring, r)
	if len(st.ring) > N {
		st.ring = st.ring[len(st.ring)-N:]
	}
	st.consecutiveSkips = 0
	return nil
}

// RecordSkip bumps the source's consecutive-skip count and stamps its
// last-skipped time. A skipped tick is a signal, not a run entry -- no history
// entry is created (RUN-02).
func (s *Store) RecordSkip(source string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.stateLocked(source)
	st.consecutiveSkips++
	st.lastSkippedAt = time.Now()
}

func (s *Store) stateLocked(source string) *sourceState {
	st := s.sources[source]
	if st == nil {
		st = &sourceState{}
		s.sources[source] = st
	}
	return st
}

func normalizeOutcome(o string) string {
	switch o {
	case OutcomeOK, OutcomeError, OutcomeCancelled:
		return o
	default:
		return OutcomeError
	}
}

func composeSummary(r RunResult) string {
	return fmt.Sprintf("%s — %d checked, %d errored, %d events",
		r.Outcome, r.ArtistsChecked, r.ArtistsErrored, r.EventsRecorded)
}

// SourceSnapshot is a race-free copy of one source's state handed to the
// /status handler.
type SourceSnapshot struct {
	LastRun          *RunResult
	History          []RunResult // newest-first copy
	LastSkippedAt    *time.Time
	ConsecutiveSkips int
}

// Snapshot returns a per-source copy of the history. Each source's entries are
// copied into a freshly allocated slice while the mutex is held, so a later
// append can never mutate a response already being encoded.
func (s *Store) Snapshot() map[string]SourceSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[string]SourceSnapshot, len(s.sources))
	for src, st := range s.sources {
		hist := make([]RunResult, len(st.ring))
		for i, r := range st.ring { // fill in reverse: newest recorded at index 0
			hist[len(st.ring)-1-i] = r
		}
		snap := SourceSnapshot{History: hist, ConsecutiveSkips: st.consecutiveSkips}
		if len(hist) > 0 {
			lr := hist[0]
			snap.LastRun = &lr
		}
		if !st.lastSkippedAt.IsZero() {
			t := st.lastSkippedAt
			snap.LastSkippedAt = &t
		}
		out[src] = snap
	}
	return out
}
