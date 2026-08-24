package artistart

// This file is stub-driven, no-database: stubStore satisfies Store, and
// every Matcher under test is a real *Matcher built over plan 13-02's own
// searcher/album/group stub seams (stubAlbumLister, stubGroupLister, defined
// in match_test.go) plus perArtistSearcher below, so Backfill's plumbing is
// exercised through the real match rule rather than a second mock.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
)

// testLogger returns a *slog.Logger that discards everything, so these
// tests emit no output on the deliberately-driven error paths.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// perArtistOutcome is one queried name's canned ArtistSearcher response.
type perArtistOutcome struct {
	artists []deezer.Artist
	err     error
}

// perArtistSearcher is an ArtistSearcher keyed by the exact query string
// Match calls it with (which is always the artist's own Name, since Match
// calls m.search.SearchArtists(ctx, name, searchLimit)). A name with no
// configured outcome returns zero candidates -- D-09 fail-closed, exactly
// like a real "no such artist on Deezer" response.
type perArtistSearcher struct {
	byName map[string]perArtistOutcome
}

func (s perArtistSearcher) SearchArtists(_ context.Context, query string, _ int) ([]deezer.Artist, error) {
	outcome, ok := s.byName[query]
	if !ok {
		return nil, nil
	}
	if outcome.err != nil {
		return nil, outcome.err
	}
	return outcome.artists, nil
}

// matchingCandidate builds a single Deezer candidate whose Name is exactly
// name, so it always resolves as Match's one close-name match (no tie).
func matchingCandidate(name string, deezerID int64, picture string) []deezer.Artist {
	return []deezer.Artist{{ID: deezerID, Name: name, Picture: picture}}
}

// stubStore is a test double for Store. Every UpsertArtist/RecordArtMatchAttempt
// call is recorded in order, so tests can assert both which artists were
// written and which were skipped. upsertErrByMbid/attemptErrByMbid let a
// single test drive a per-artist write failure without a second stub type.
type stubStore struct {
	artists []sqlc.Artist
	listErr error

	upserts          []sqlc.UpsertArtistParams
	upsertErrByMbid  map[string]error
	attempts         []string
	attemptErrByMbid map[string]error
}

func (s *stubStore) ListArtistsMissingImage(_ context.Context) ([]sqlc.Artist, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.artists, nil
}

func (s *stubStore) UpsertArtist(_ context.Context, arg sqlc.UpsertArtistParams) (sqlc.Artist, error) {
	s.upserts = append(s.upserts, arg)
	if err, ok := s.upsertErrByMbid[arg.Mbid]; ok {
		return sqlc.Artist{}, err
	}
	return sqlc.Artist{Mbid: arg.Mbid, Name: arg.Name, DeezerID: arg.DeezerID, ImageUrl: arg.ImageUrl}, nil
}

func (s *stubStore) RecordArtMatchAttempt(_ context.Context, mbid string) error {
	s.attempts = append(s.attempts, mbid)
	if err, ok := s.attemptErrByMbid[mbid]; ok {
		return err
	}
	return nil
}

func TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach(t *testing.T) {
	artists := []sqlc.Artist{
		{Mbid: "mb-1", Name: "Artist One"},
		{Mbid: "mb-2", Name: "Artist Two"},
		{Mbid: "mb-3", Name: "Artist Three"},
	}
	searcher := perArtistSearcher{byName: map[string]perArtistOutcome{
		"Artist One":   {artists: matchingCandidate("Artist One", 101, "https://example.test/1.jpg")},
		"Artist Two":   {artists: matchingCandidate("Artist Two", 102, "https://example.test/2.jpg")},
		"Artist Three": {artists: matchingCandidate("Artist Three", 103, "https://example.test/3.jpg")},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})
	store := &stubStore{artists: artists}

	stats, err := Backfill(context.Background(), testLogger(), store, m, nil)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	if stats.Visited != 3 || stats.Matched != 3 || stats.Unmatched != 0 || stats.Errored != 0 {
		t.Fatalf("stats = %+v, want Visited=3 Matched=3 Unmatched=0 Errored=0", stats)
	}

	if len(store.upserts) != 3 {
		t.Fatalf("len(store.upserts) = %d, want 3", len(store.upserts))
	}
	for _, u := range store.upserts {
		if u.Disambiguation != nil {
			t.Fatalf("UpsertArtistParams{Mbid: %q}.Disambiguation = %v, want nil (COALESCE-preserved)", u.Mbid, u.Disambiguation)
		}
		if u.DeezerID == nil || u.ImageUrl == nil {
			t.Fatalf("UpsertArtistParams{Mbid: %q} = %+v, want both DeezerID and ImageUrl set", u.Mbid, u)
		}
	}
	if len(store.attempts) != 3 {
		t.Fatalf("len(store.attempts) = %d, want 3 (one RecordArtMatchAttempt per visited artist)", len(store.attempts))
	}
}

func TestBackfill_UnmatchedArtist_NoUpsertButRecordsAttempt(t *testing.T) {
	artists := []sqlc.Artist{{Mbid: "mb-unmatched", Name: "No Match Artist"}}
	// No configured outcome for this name: zero candidates, D-09 fail-closed.
	searcher := perArtistSearcher{byName: map[string]perArtistOutcome{}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})
	store := &stubStore{artists: artists}

	stats, err := Backfill(context.Background(), testLogger(), store, m, nil)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}

	if stats.Visited != 1 || stats.Matched != 0 || stats.Unmatched != 1 || stats.Errored != 0 {
		t.Fatalf("stats = %+v, want Visited=1 Matched=0 Unmatched=1 Errored=0", stats)
	}
	if len(store.upserts) != 0 {
		t.Fatalf("len(store.upserts) = %d, want 0 (D-09 fail-closed writes nothing)", len(store.upserts))
	}
	if len(store.attempts) != 1 || store.attempts[0] != "mb-unmatched" {
		t.Fatalf("store.attempts = %v, want exactly [%q] (D-12: an unmatched outcome is still a considered attempt)", store.attempts, "mb-unmatched")
	}
}

func TestBackfill_MatchError_NoUpsertNoRecordAttempt_ContinuesOthers(t *testing.T) {
	artists := []sqlc.Artist{
		{Mbid: "mb-ok-1", Name: "Good Artist One"},
		{Mbid: "mb-erroring", Name: "Erroring Artist"},
		{Mbid: "mb-ok-2", Name: "Good Artist Two"},
	}
	searcher := perArtistSearcher{byName: map[string]perArtistOutcome{
		"Good Artist One": {artists: matchingCandidate("Good Artist One", 201, "https://example.test/g1.jpg")},
		"Erroring Artist": {err: errors.New("upstream unreachable")},
		"Good Artist Two": {artists: matchingCandidate("Good Artist Two", 202, "https://example.test/g2.jpg")},
	}}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})
	store := &stubStore{artists: artists}

	stats, err := Backfill(context.Background(), testLogger(), store, m, nil)
	if err != nil {
		t.Fatalf("Backfill returned err = %v, want nil (one artist's match error must not fail the whole sweep)", err)
	}

	if stats.Visited != 3 {
		t.Fatalf("stats.Visited = %d, want 3 (the erroring artist was still visited, and the other two were still processed)", stats.Visited)
	}
	if stats.Errored != 1 {
		t.Fatalf("stats.Errored = %d, want 1", stats.Errored)
	}
	if stats.Matched != 2 {
		t.Fatalf("stats.Matched = %d, want 2 (the two good artists)", stats.Matched)
	}

	for _, mbid := range store.attempts {
		if mbid == "mb-erroring" {
			t.Fatalf("store.attempts = %v, must not contain %q (a transient match error is not a considered attempt, D-12)", store.attempts, "mb-erroring")
		}
	}
	for _, u := range store.upserts {
		if u.Mbid == "mb-erroring" {
			t.Fatalf("store.upserts contains an entry for %q, want none", "mb-erroring")
		}
	}
	if len(store.upserts) != 2 || len(store.attempts) != 2 {
		t.Fatalf("len(store.upserts)=%d len(store.attempts)=%d, want 2 and 2 (only the two good artists)", len(store.upserts), len(store.attempts))
	}
}

func TestBackfill_ListArtistsMissingImageErrors_ReturnsErrNoWrites(t *testing.T) {
	store := &stubStore{listErr: errors.New("connection refused")}
	m := NewMatcher(perArtistSearcher{}, &stubAlbumLister{}, &stubGroupLister{})

	stats, err := Backfill(context.Background(), testLogger(), store, m, nil)
	if err == nil {
		t.Fatal("Backfill err = nil, want a non-nil error")
	}
	if stats != (Stats{}) {
		t.Fatalf("stats = %+v, want the zero value", stats)
	}
	if len(store.upserts) != 0 || len(store.attempts) != 0 {
		t.Fatalf("store.upserts=%d store.attempts=%d, want 0 and 0 (the sweep never started)", len(store.upserts), len(store.attempts))
	}
}

func TestBackfill_ZeroArtists_ReturnsNilNoCalls(t *testing.T) {
	store := &stubStore{artists: nil}
	m := NewMatcher(perArtistSearcher{}, &stubAlbumLister{}, &stubGroupLister{})

	stats, err := Backfill(context.Background(), testLogger(), store, m, nil)
	if err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	if stats != (Stats{}) {
		t.Fatalf("stats = %+v, want the zero value", stats)
	}
	if len(store.upserts) != 0 || len(store.attempts) != 0 {
		t.Fatalf("store.upserts=%d store.attempts=%d, want 0 and 0", len(store.upserts), len(store.attempts))
	}
}

// cancelingSearcher wraps another ArtistSearcher and invokes cancel the
// moment a specific query arrives, deterministically forcing a mid-sweep
// cancellation rather than racing a real timer against the loop.
type cancelingSearcher struct {
	inner   ArtistSearcher
	onQuery string
	cancel  context.CancelFunc
}

func (s cancelingSearcher) SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error) {
	if query == s.onQuery {
		s.cancel()
	}
	return s.inner.SearchArtists(ctx, query, limit)
}

func TestBackfill_ContextCancelledPartway_StopsPromptly(t *testing.T) {
	artists := []sqlc.Artist{
		{Mbid: "mb-1", Name: "Artist One"},
		{Mbid: "mb-2", Name: "Artist Two"},
		{Mbid: "mb-3", Name: "Artist Three"},
	}
	inner := perArtistSearcher{byName: map[string]perArtistOutcome{
		"Artist One":   {artists: matchingCandidate("Artist One", 301, "https://example.test/1.jpg")},
		"Artist Two":   {artists: matchingCandidate("Artist Two", 302, "https://example.test/2.jpg")},
		"Artist Three": {artists: matchingCandidate("Artist Three", 303, "https://example.test/3.jpg")},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel the instant "Artist Two" is queried -- its own Match call still
	// completes and is counted, and the top-of-loop ctx.Err() check catches
	// the cancellation before "Artist Three" is ever visited.
	searcher := cancelingSearcher{inner: inner, onQuery: "Artist Two", cancel: cancel}
	m := NewMatcher(searcher, &stubAlbumLister{}, &stubGroupLister{})
	store := &stubStore{artists: artists}

	stats, err := Backfill(ctx, testLogger(), store, m, nil)
	if err == nil {
		t.Fatal("Backfill err = nil, want a non-nil error (context was cancelled partway through)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want errors.Is(err, context.Canceled)", err)
	}
	if stats.Visited >= len(artists) {
		t.Fatalf("stats.Visited = %d, want strictly less than %d", stats.Visited, len(artists))
	}
	if stats.Visited == 0 {
		t.Fatal("stats.Visited = 0, want at least the artists processed before cancellation")
	}
}

func TestStats_MatchRatePercent(t *testing.T) {
	if got := (Stats{}).MatchRatePercent(); got != 0 {
		t.Fatalf("Stats{}.MatchRatePercent() = %v, want 0 (no divide-by-zero on Visited==0)", got)
	}

	got := Stats{Visited: 10, Matched: 4}.MatchRatePercent()
	if got != 40.0 {
		t.Fatalf("Stats{Visited: 10, Matched: 4}.MatchRatePercent() = %v, want 40.0", got)
	}
}

// recordingTimeSearcher wraps an ArtistSearcher and records the wall-clock
// time of its first call into *calledAt, so a test can measure how long
// Backfill waited before issuing its first match attempt.
type recordingTimeSearcher struct {
	inner    ArtistSearcher
	calledAt *time.Time
}

func (s *recordingTimeSearcher) SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error) {
	if s.calledAt.IsZero() {
		*s.calledAt = time.Now()
	}
	return s.inner.SearchArtists(ctx, query, limit)
}

// TestBackfill_ActivityGate_DelaysThenProceeds is -race-clean (no shared
// state is written from more than one goroutine without synchronization:
// ActivityGate's own counter is atomic, and calledAt is only ever written
// from inside Backfill's own single goroutine). It proves D-10's bounded
// yield: the sweep's first match attempt is measurably delayed while a
// gate is active, and proceeds well before backfillActivityMaxWait once the
// gate goes inactive -- and an inactive/nil gate adds no delay at all.
func TestBackfill_ActivityGate_DelaysThenProceeds(t *testing.T) {
	searcher := perArtistSearcher{byName: map[string]perArtistOutcome{
		"Gate Artist": {artists: matchingCandidate("Gate Artist", 401, "https://example.test/gate.jpg")},
	}}

	gate := NewActivityGate()
	end := gate.Begin()
	// Held well under backfillActivityYieldInterval so the sweep observes
	// "inactive" at its first poll tick rather than waiting out the full
	// backfillActivityMaxWait bound.
	time.AfterFunc(80*time.Millisecond, end)

	var matchAt time.Time
	recorder := &recordingTimeSearcher{inner: searcher, calledAt: &matchAt}
	m := NewMatcher(recorder, &stubAlbumLister{}, &stubGroupLister{})
	store := &stubStore{artists: []sqlc.Artist{{Mbid: "mb-gate", Name: "Gate Artist"}}}

	start := time.Now()
	if _, err := Backfill(context.Background(), testLogger(), store, m, gate); err != nil {
		t.Fatalf("Backfill: %v", err)
	}
	elapsed := matchAt.Sub(start)

	if elapsed < backfillActivityYieldInterval {
		t.Fatalf("elapsed = %v, want >= %v (the sweep must yield to the active gate for at least one poll interval)", elapsed, backfillActivityYieldInterval)
	}
	if elapsed >= backfillActivityMaxWait {
		t.Fatalf("elapsed = %v, want < %v (the sweep must still proceed once the gate deactivates, well before the max-wait bound -- not indefinite)", elapsed, backfillActivityMaxWait)
	}

	// Control: a nil gate adds no delay at all.
	var controlMatchAt time.Time
	controlRecorder := &recordingTimeSearcher{inner: searcher, calledAt: &controlMatchAt}
	controlMatcher := NewMatcher(controlRecorder, &stubAlbumLister{}, &stubGroupLister{})
	controlStore := &stubStore{artists: []sqlc.Artist{{Mbid: "mb-gate", Name: "Gate Artist"}}}

	controlStart := time.Now()
	if _, err := Backfill(context.Background(), testLogger(), controlStore, controlMatcher, nil); err != nil {
		t.Fatalf("control Backfill: %v", err)
	}
	controlElapsed := controlMatchAt.Sub(controlStart)
	if controlElapsed >= backfillActivityYieldInterval {
		t.Fatalf("control elapsed = %v, want < %v (a nil gate must add no delay)", controlElapsed, backfillActivityYieldInterval)
	}
}
