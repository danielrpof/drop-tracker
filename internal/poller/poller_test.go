package poller

// This file is package poller (whitebox), matching internal/musicbrainz and
// internal/deezer's test convention -- it exercises RunMusicBrainzCycle and
// RunDeezerCycle directly against fake ReleaseGroupSource/AlbumSource
// doubles and a stub watchlist.Store, with log output captured through a
// real slog.JSONHandler so attribute assertions are on real emitted
// records, not on internal state.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/detection"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// var _ EventRecorder = (*detection.Detector)(nil) asserts detection.Detector
// satisfies the seam poller.go declares, kept here (not in cmd/server/main.go
// or production poller.go) so internal/poller itself stays free of a
// detection import (04-01 acceptance criteria).
var _ EventRecorder = (*detection.Detector)(nil)

// newTestLogger builds a *slog.Logger writing newline-delimited JSON into
// buf, so a test can decode each emitted record and assert on its
// attributes directly.
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), buf
}

// decodeLogRecords parses every JSON line in buf into a map, in emission
// order.
func decodeLogRecords(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		records = append(records, rec)
	}
	return records
}

func deezerID(id string) *string { return &id }

// fakeRecordingSource is a no-op detection.RecordingSource double, used only
// to satisfy detection.New's widened signature (Phase 4 plan 04-03) for the
// integration tests below, which exercise new_release wiring, not
// guest-feature detection.
type fakeRecordingSource struct{}

func (fakeRecordingSource) RecordingsByArtist(ctx context.Context, mbid string) ([]musicbrainz.Recording, error) {
	return nil, nil
}

// fakeReleaseDetailSource is a no-op detection.ReleaseDetailSource double,
// used only to satisfy detection.New's widened signature (Phase 4 plan
// 04-04) for the integration tests below, which exercise new_release
// wiring, not deluxe-change detection -- every release-group these tests
// fetch is brand-new within its own cycle (D-04), so the deluxe pass never
// calls this.
type fakeReleaseDetailSource struct{}

func (fakeReleaseDetailSource) ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]musicbrainz.Release, error) {
	return nil, nil
}

// stubStore is a file-local double for watchlist.Store, mirroring
// internal/httpserver/watchlist_test.go's stubStore -- every write method
// records its call count so a test can assert the poll cycle never writes
// (D-04).
type stubStore struct {
	listFunc func(ctx context.Context) ([]watchlist.Entry, error)

	listCalls   int32
	addCalls    int32
	updateCalls int32
	removeCalls int32
}

func (s *stubStore) Add(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error) {
	atomic.AddInt32(&s.addCalls, 1)
	return watchlist.Entry{}, nil
}

func (s *stubStore) List(ctx context.Context) ([]watchlist.Entry, error) {
	atomic.AddInt32(&s.listCalls, 1)
	if s.listFunc != nil {
		return s.listFunc(ctx)
	}
	return nil, nil
}

func (s *stubStore) UpdatePreferences(ctx context.Context, id int64, p watchlist.PreferencesParams) (watchlist.Entry, error) {
	atomic.AddInt32(&s.updateCalls, 1)
	return watchlist.Entry{}, nil
}

func (s *stubStore) Remove(ctx context.Context, id int64) error {
	atomic.AddInt32(&s.removeCalls, 1)
	return nil
}

var _ watchlist.Store = (*stubStore)(nil)

// fakeReleaseGroupSource is a file-local double for ReleaseGroupSource. It
// tracks call count, the MBIDs it was called with (in order), and the
// maximum number of concurrently in-flight calls it ever observed -- the
// real observation TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount,
// TestMusicBrainzCycle_WorkerCountOneIsSequential,
// TestMusicBrainzCycle_WorkerCountAboveEntryCountFansOutToEntryCount, and
// TestMusicBrainzCycle_SingleEntryReachesOneInFlight assert on (PERF-01).
type fakeReleaseGroupSource struct {
	fn func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error)

	calls       int32
	inFlight    int32
	maxInFlight int32

	mu    sync.Mutex
	mbids []string
}

func (f *fakeReleaseGroupSource) ReleaseGroupsByArtist(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
	atomic.AddInt32(&f.calls, 1)
	cur := atomic.AddInt32(&f.inFlight, 1)
	defer atomic.AddInt32(&f.inFlight, -1)
	for {
		old := atomic.LoadInt32(&f.maxInFlight)
		if cur <= old {
			break
		}
		if atomic.CompareAndSwapInt32(&f.maxInFlight, old, cur) {
			break
		}
	}

	f.mu.Lock()
	f.mbids = append(f.mbids, mbid)
	f.mu.Unlock()

	if f.fn != nil {
		return f.fn(ctx, mbid)
	}
	return nil, nil
}

// fakeEventRecorder is a file-local double for EventRecorder, mirroring
// fakeReleaseGroupSource/fakeAlbumSource's call-tracking convention. fn and
// deezerFn are nil by default (a silent no-op success), matching what a real
// Detector with nothing new to record would do.
type fakeEventRecorder struct {
	fn       func(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error
	deezerFn func(ctx context.Context, entry watchlist.Entry, albums []deezer.Album) error

	calls       int32
	deezerCalls int32

	mu          sync.Mutex
	mbids       []string
	deezerMBIDs []string
}

func (f *fakeEventRecorder) DetectMusicBrainz(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	f.mbids = append(f.mbids, entry.MBID)
	f.mu.Unlock()

	if f.fn != nil {
		return f.fn(ctx, entry, groups)
	}
	return nil
}

func (f *fakeEventRecorder) DetectDeezer(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, albums []deezer.Album) error {
	atomic.AddInt32(&f.deezerCalls, 1)
	f.mu.Lock()
	f.deezerMBIDs = append(f.deezerMBIDs, entry.MBID)
	f.mu.Unlock()

	if f.deezerFn != nil {
		return f.deezerFn(ctx, entry, albums)
	}
	return nil
}

var _ EventRecorder = (*fakeEventRecorder)(nil)

// fakeNotifier is a file-local double for poller.Notifier, mirroring
// fakeEventRecorder's call-tracking convention. Declared here rather than
// importing internal/notifier -- the test only needs something that
// satisfies this package's own locally-declared seam (04-01's
// EventRecorder-import-freedom acceptance criteria applies equally to
// Notifier).
type fakeNotifier struct {
	fn func(ctx context.Context, logger *slog.Logger) error

	calls int32
}

func (f *fakeNotifier) NotifyPending(ctx context.Context, logger *slog.Logger) error {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		return f.fn(ctx, logger)
	}
	return nil
}

var _ Notifier = (*fakeNotifier)(nil)

// testArtistMBID derives a short, unique-per-test artist mbid from
// t.Name(), matching internal/watchlist/service_test.go's testMBID
// convention -- used only by the real-Postgres integration tests in this
// file, which need a genuine artists row to satisfy events' foreign key.
func testArtistMBID(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

// fakeAlbumSource is fakeReleaseGroupSource's Deezer-side twin.
type fakeAlbumSource struct {
	fn func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error)

	calls       int32
	inFlight    int32
	maxInFlight int32

	mu        sync.Mutex
	artistIDs []string
}

func (f *fakeAlbumSource) ArtistAlbums(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
	atomic.AddInt32(&f.calls, 1)
	cur := atomic.AddInt32(&f.inFlight, 1)
	defer atomic.AddInt32(&f.inFlight, -1)
	for {
		old := atomic.LoadInt32(&f.maxInFlight)
		if cur <= old {
			break
		}
		if atomic.CompareAndSwapInt32(&f.maxInFlight, old, cur) {
			break
		}
	}

	f.mu.Lock()
	f.artistIDs = append(f.artistIDs, artistID)
	f.mu.Unlock()

	if f.fn != nil {
		return f.fn(ctx, artistID, limit)
	}
	return nil, nil
}

func threeEntries() []watchlist.Entry {
	return []watchlist.Entry{
		{MBID: "mbid-1", Name: "Artist One", DeezerID: deezerID("101")},
		{MBID: "mbid-2", Name: "Artist Two", DeezerID: nil},
		{MBID: "mbid-3", Name: "Artist Three", DeezerID: deezerID("103")},
	}
}

// newTestPoller builds a Poller for tests that don't care about detection,
// defaulting events to a no-op fakeEventRecorder -- events is variadic so
// this helper's signature (and all its existing call sites) stays stable
// across New's widened parameter list; a test that does care passes its own
// recorder explicitly.
func newTestPoller(t *testing.T, store watchlist.Store, mb ReleaseGroupSource, dz AlbumSource, logger *slog.Logger, events ...EventRecorder) *Poller {
	t.Helper()
	var er EventRecorder = &fakeEventRecorder{}
	if len(events) > 0 {
		er = events[0]
	}
	p, err := New(store, mb, dz, er, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

// --- MusicBrainz cycle ---

func TestMusicBrainzCycle_CallsSourceOncePerEntry(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	if got := atomic.LoadInt32(&mb.calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
	// Membership, not order: under concurrent fan-out the three workers can
	// finish in any order, so the assertion is on the set of MBIDs called,
	// not a fixed dispatch-order sequence.
	want := []string{"mbid-1", "mbid-2", "mbid-3"}
	mb.mu.Lock()
	got := append([]string(nil), mb.mbids...)
	mb.mu.Unlock()
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("mbids = %v, want %v in any order", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mbids = %v, want %v in any order", got, want)
		}
	}

	if atomic.LoadInt32(&store.addCalls) != 0 || atomic.LoadInt32(&store.updateCalls) != 0 || atomic.LoadInt32(&store.removeCalls) != 0 {
		t.Fatal("cycle must never write to the store (D-04)")
	}
}

// TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount is the tracer's
// canonical proof for PERF-01: over 6 watchlist entries and a worker pool
// bounded to 3 (WithMusicBrainzWorkers(3)), the fan-out must reach the
// ceiling (proving it actually ran concurrently) and never exceed it
// (proving the bound is enforced). fakeReleaseGroupSource's fetch sleeps
// briefly so overlapping in-flight calls are actually observable -- without
// a delay, a fast fake could complete each call before the next dispatches,
// making maxInFlight read 1 even under a correct concurrent implementation.
func TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount(t *testing.T) {
	entries := []watchlist.Entry{
		{MBID: "mbid-1", Name: "Artist One"},
		{MBID: "mbid-2", Name: "Artist Two"},
		{MBID: "mbid-3", Name: "Artist Three"},
		{MBID: "mbid-4", Name: "Artist Four"},
		{MBID: "mbid-5", Name: "Artist Five"},
		{MBID: "mbid-6", Name: "Artist Six"},
	}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return entries, nil }}
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithMusicBrainzWorkers(3))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	if got := atomic.LoadInt32(&mb.maxInFlight); got != 3 {
		t.Fatalf("max in-flight = %d, want exactly 3", got)
	}
}

// TestMusicBrainzCycle_WorkerCountOneIsSequential proves the old sequential
// guarantee this phase's previous (now-removed) test used to hard-code as
// the default is still reachable -- it is now a configuration
// (WithMusicBrainzWorkers(1)), not a default. Deterministic with no
// sleep/gating needed: with a pool size of
// 1, the semaphore's single slot is only released after a worker's entire
// body (including its fetch call) completes, so the dispatch loop cannot
// even launch the next worker until the current one has fully finished --
// maxInFlight can never exceed 1 regardless of how fast or slow the fake
// runs.
func TestMusicBrainzCycle_WorkerCountOneIsSequential(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{
			{MBID: "mbid-1", Name: "Artist One"},
			{MBID: "mbid-2", Name: "Artist Two"},
			{MBID: "mbid-3", Name: "Artist Three"},
			{MBID: "mbid-4", Name: "Artist Four"},
		}, nil
	}}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithMusicBrainzWorkers(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	if got := atomic.LoadInt32(&mb.maxInFlight); got != 1 {
		t.Fatalf("max in-flight = %d, want exactly 1 (WithMusicBrainzWorkers(1) must still be reachable)", got)
	}
	if got := atomic.LoadInt32(&mb.calls); got != 4 {
		t.Fatalf("calls = %d, want 4 (every entry still polled, just individually rather than concurrently)", got)
	}
}

// TestMusicBrainzCycle_WorkerCountAboveEntryCountFansOutToEntryCount proves
// a worker count larger than the entry count fans out to at most the entry
// count and the dispatch loop never blocks waiting for slots it will never
// need. release gates every fetch call so the test can deterministically
// observe all 3 entries in flight simultaneously (rather than trusting a
// fixed sleep duration is "long enough" on a loaded machine) before letting
// them complete.
func TestMusicBrainzCycle_WorkerCountAboveEntryCountFansOutToEntryCount(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	release := make(chan struct{})
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		<-release
		return nil, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithMusicBrainzWorkers(8))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunMusicBrainzCycle(context.Background()) }()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&mb.inFlight) < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for 3 in-flight calls, last observed %d", atomic.LoadInt32(&mb.inFlight))
		}
		time.Sleep(time.Millisecond)
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunMusicBrainzCycle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunMusicBrainzCycle did not return promptly after release -- possible deadlock on an unused semaphore slot")
	}

	if got := atomic.LoadInt32(&mb.maxInFlight); got != 3 {
		t.Fatalf("max in-flight = %d, want exactly 3 (the entry count, not the 8-worker pool ceiling)", got)
	}
}

// TestMusicBrainzCycle_SingleEntryReachesOneInFlight is deterministic with
// no gating needed: a single entry can never produce more than 1 in-flight
// call regardless of pool size.
func TestMusicBrainzCycle_SingleEntryReachesOneInFlight(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One"}}, nil
	}}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithMusicBrainzWorkers(3))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	if got := atomic.LoadInt32(&mb.maxInFlight); got != 1 {
		t.Fatalf("max in-flight = %d, want exactly 1", got)
	}
}

// TestMusicBrainzCycle_LogsCycleDurationAndArtistCount pins D-04/D-05: every
// completed cycle emits exactly one "poll cycle complete" line carrying
// artist_count equal to the watchlist size and a non-negative integer
// duration_ms (Milliseconds() truncates toward zero -- the contract is an
// integer, never a fractional JSON number).
func TestMusicBrainzCycle_LogsCycleDurationAndArtistCount(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	records := decodeLogRecords(t, buf)
	var found map[string]any
	count := 0
	for _, rec := range records {
		if rec["msg"] == "poll cycle complete" {
			found = rec
			count++
		}
	}
	if count != 1 {
		t.Fatalf("found %d 'poll cycle complete' records, want exactly 1", count)
	}

	artistCount, ok := found["artist_count"].(float64)
	if !ok {
		t.Fatalf("artist_count = %v (%T), want a JSON number", found["artist_count"], found["artist_count"])
	}
	if int(artistCount) != 3 {
		t.Fatalf("artist_count = %v, want 3", artistCount)
	}

	durationMS, ok := found["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms = %v (%T), want a JSON number", found["duration_ms"], found["duration_ms"])
	}
	if durationMS < 0 {
		t.Fatalf("duration_ms = %v, want >= 0", durationMS)
	}
	if durationMS != math.Trunc(durationMS) {
		t.Fatalf("duration_ms = %v, want an integer (no fractional part)", durationMS)
	}
}

func TestMusicBrainzCycle_LogsStructuredResult(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One"}}, nil
	}}
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		return []musicbrainz.ReleaseGroup{{MBID: "rg-1"}, {MBID: "rg-2"}}, nil
	}}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	records := decodeLogRecords(t, buf)
	var found map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == "mbid-1" {
			found = rec
		}
	}
	if found == nil {
		t.Fatal("no log record found for artist_mbid=mbid-1")
	}
	if found["source"] != sourceMusicBrainz {
		t.Fatalf("source = %v, want %q", found["source"], sourceMusicBrainz)
	}
	if found["artist_name"] != "Artist One" {
		t.Fatalf("artist_name = %v, want %q", found["artist_name"], "Artist One")
	}
	itemCount, ok := found["item_count"].(float64)
	if !ok || itemCount != 2 {
		t.Fatalf("item_count = %v, want 2", found["item_count"])
	}
	if _, ok := found["cycle_id"].(string); !ok {
		t.Fatalf("cycle_id missing or not a string: %v", found["cycle_id"])
	}
}

func TestMusicBrainzCycle_CycleIDSharedAcrossRecordsDiffersBetweenCycles(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle (1): %v", err)
	}
	firstRecords := decodeLogRecords(t, buf)
	if len(firstRecords) == 0 {
		t.Fatal("expected at least one log record for the first cycle")
	}
	firstCycleID, _ := firstRecords[0]["cycle_id"].(string)
	if firstCycleID == "" {
		t.Fatal("first cycle_id is empty")
	}
	for _, rec := range firstRecords {
		if rec["cycle_id"] != firstCycleID {
			t.Fatalf("record cycle_id = %v, want %q (all records in one cycle must share cycle_id)", rec["cycle_id"], firstCycleID)
		}
	}

	buf.Reset()
	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle (2): %v", err)
	}
	secondRecords := decodeLogRecords(t, buf)
	if len(secondRecords) == 0 {
		t.Fatal("expected at least one log record for the second cycle")
	}
	secondCycleID, _ := secondRecords[0]["cycle_id"].(string)
	if secondCycleID == firstCycleID {
		t.Fatalf("second cycle_id (%q) must differ from first (%q)", secondCycleID, firstCycleID)
	}
}

func TestMusicBrainzCycle_PerArtistErrorContinuesCycle(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	failOn := "mbid-2"
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		if mbid == failOn {
			return nil, errors.New("upstream exploded")
		}
		return []musicbrainz.ReleaseGroup{}, nil
	}}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	err := p.RunMusicBrainzCycle(context.Background())
	if err != nil {
		t.Fatalf("RunMusicBrainzCycle returned %v, want nil (a per-artist failure is not a cycle failure)", err)
	}
	if got := atomic.LoadInt32(&mb.calls); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}

	records := decodeLogRecords(t, buf)
	var errRecord map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == failOn && rec["level"] == "ERROR" {
			errRecord = rec
		}
	}
	if errRecord == nil {
		t.Fatalf("no error-level record found for artist_mbid=%s; records=%v", failOn, records)
	}
}

func TestMusicBrainzCycle_ListErrorReturnsZeroCallsNonNilError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, errors.New("db down") }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	err := p.RunMusicBrainzCycle(context.Background())
	if err == nil {
		t.Fatal("expected non-nil error when store.List fails")
	}
	if got := atomic.LoadInt32(&mb.calls); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}

// TestMusicBrainzCycle_EmptyWatchlistNoCallsNilError also pins the empty
// side of PERF-01's fan-out (zero goroutines, zero source calls), D-04/D-05
// (the cycle-end log line still fires with artist_count 0), and D-05's
// notifier contract (NotifyPending still runs exactly once even when there
// is nothing to poll) -- constructed via New directly, not newTestPoller,
// so the notifier fake is reachable for the assertion below.
func TestMusicBrainzCycle_EmptyWatchlistNoCallsNilError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	mb := &fakeReleaseGroupSource{}
	notifier := &fakeNotifier{}
	logger, buf := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, notifier, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&mb.calls); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&notifier.calls); got != 1 {
		t.Fatalf("notifier.calls = %d, want 1", got)
	}

	records := decodeLogRecords(t, buf)
	var found map[string]any
	for _, rec := range records {
		if rec["msg"] == "poll cycle complete" {
			found = rec
		}
	}
	if found == nil {
		t.Fatal("no 'poll cycle complete' log record found")
	}
	artistCount, ok := found["artist_count"].(float64)
	if !ok || int(artistCount) != 0 {
		t.Fatalf("artist_count = %v, want 0", found["artist_count"])
	}
}

// TestMusicBrainzCycle_ContextCancelledStopsIteration pins the cancellation
// contract: dispatching new workers stops once ctx is cancelled, and the
// cycle returns the context error. This test explicitly bounds the pool to
// 1 worker (WithMusicBrainzWorkers(1)), rather than relying on
// newTestPoller's default of 3 over these same 3 entries -- with pool size
// >= entry count the dispatch loop's semaphore send never actually blocks
// (11-01-PLAN.md's own "never blocks" truth for that configuration), so
// cancellation could only be observed by a goroutine-scheduling race, not a
// deterministic happens-before edge. Forcing genuine semaphore contention
// (1 worker, so dispatching mbid-2 must wait for mbid-1's worker to release
// its slot) makes the ordering deterministic instead: mbid-1's fetch calls
// cancel() and only *then* returns (releasing the semaphore via its
// deferred `<-sem`), so ctx.Done() is guaranteed to close strictly before
// that release -- the blocked dispatch-loop select always observes
// cancellation, never the freed slot.
func TestMusicBrainzCycle_ContextCancelledStopsIteration(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	ctx, cancel := context.WithCancel(context.Background())
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		if mbid == "mbid-1" {
			cancel()
		}
		return []musicbrainz.ReleaseGroup{}, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithMusicBrainzWorkers(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cycleErr := p.RunMusicBrainzCycle(ctx)
	if !errors.Is(cycleErr, context.Canceled) {
		t.Fatalf("RunMusicBrainzCycle error = %v, want context.Canceled", cycleErr)
	}
	if got := atomic.LoadInt32(&mb.calls); got >= 3 {
		t.Fatalf("calls = %d, want < 3 (iteration must stop once ctx is cancelled)", got)
	}
}

// TestPoller_RunMusicBrainzCycle_RecordsNewRelease is the end-to-end proof
// that the EventRecorder seam is actually wired into RunMusicBrainzCycle,
// not just that detection.Detector works in isolation (04-01 task 2): a
// real detection.Detector over a real pool, driven through the cycle
// exactly as cmd/server/main.go wires it, must leave the same event rows
// TestDetectMusicBrainz_NewRelease proves at the detection-package level.
func TestPoller_RunMusicBrainzCycle_RecordsNewRelease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testArtistMBID(t)

	var artistID int64
	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "Poller Integration Artist").Scan(&artistID); err != nil {
		t.Fatalf("insert test artist: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE id = $1", artistID); err != nil {
			t.Fatalf("cleanup: delete artist: %v", err)
		}
	})

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Poller Integration Artist", ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return []watchlist.Entry{entry}, nil }}
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, m string) ([]musicbrainz.ReleaseGroup, error) {
		return []musicbrainz.ReleaseGroup{
			{MBID: mbid + "-rg1", Title: "Album One", PrimaryType: "Album"},
			{MBID: mbid + "-rg2", Title: "Album Two", PrimaryType: "Album"},
		}, nil
	}}
	recorder := detection.New(sqlc.New(pool), fakeRecordingSource{}, fakeReleaseDetailSource{})
	logger, _ := newTestLogger()

	p, err := New(store, mb, &fakeAlbumSource{}, recorder, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(ctx); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND source = 'musicbrainz'", artistID,
	).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (the seam must actually be wired into RunMusicBrainzCycle)", count)
	}
}

// --- Deezer cycle ---

// TestPoller_RunDeezerCycle_RecordsNewRelease is the Deezer-side twin of
// TestPoller_RunMusicBrainzCycle_RecordsNewRelease (04-01): the end-to-end
// proof that the widened EventRecorder seam is actually wired into
// RunDeezerCycle, not just that detection.Detector.DetectDeezer works in
// isolation (04-RESEARCH.md's TestPoller_RunDeezerCycle_RecordsNewRelease).
func TestPoller_RunDeezerCycle_RecordsNewRelease(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()
	mbid := testArtistMBID(t)

	var artistID int64
	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "Deezer Poller Integration Artist").Scan(&artistID); err != nil {
		t.Fatalf("insert test artist: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE id = $1", artistID); err != nil {
			t.Fatalf("cleanup: delete artist: %v", err)
		}
	})

	entry := watchlist.Entry{ArtistID: artistID, MBID: mbid, Name: "Deezer Poller Integration Artist", DeezerID: deezerID("999"), ReleaseTypes: []string{"album", "single", "ep", "deluxe"}}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return []watchlist.Entry{entry}, nil }}
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		return []deezer.Album{
			{ID: 1, Title: "Album One", RecordType: "album"},
			{ID: 2, Title: "Album Two", RecordType: "album"},
		}, nil
	}}
	recorder := detection.New(sqlc.New(pool), fakeRecordingSource{}, fakeReleaseDetailSource{})
	logger, _ := newTestLogger()

	p, err := New(store, &fakeReleaseGroupSource{}, dz, recorder, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunDeezerCycle(ctx); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx,
		"SELECT count(*) FROM events WHERE artist_id = $1 AND event_type = 'new_release' AND source = 'deezer'", artistID,
	).Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	if count != 2 {
		t.Fatalf("event row count = %d, want 2 (the seam must actually be wired into RunDeezerCycle)", count)
	}
}

// TestPoller_RunDeezerCycle_SkipsNilDeezerID proves the EventRecorder seam
// is never even reached for an entry with a nil DeezerID -- no fetch, no
// recorder call, no row, exactly as before this plan (TestDeezerCycle_
// SkipsNilDeezerID above already proves the no-fetch half; this test proves
// the widened recorder wiring didn't change that contract).
func TestPoller_RunDeezerCycle_SkipsNilDeezerID(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	events := &fakeEventRecorder{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger, events)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("dz.calls = %d, want 2 (only the two non-nil DeezerID entries)", got)
	}
	if got := atomic.LoadInt32(&events.deezerCalls); got != 2 {
		t.Fatalf("events.deezerCalls = %d, want 2 (only the two non-nil DeezerID entries)", got)
	}
	events.mu.Lock()
	calledMBIDs := append([]string(nil), events.deezerMBIDs...)
	events.mu.Unlock()
	for _, mbid := range calledMBIDs {
		if mbid == "mbid-2" {
			t.Fatalf("DetectDeezer called for mbid-2, which has a nil DeezerID and must be skipped entirely")
		}
	}
}

func TestDeezerCycle_SkipsNilDeezerID(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}
	// Membership, not order: under concurrent fan-out the two workers can
	// finish in either order, so the assertion is on the set of artist ids
	// called, not a fixed dispatch-order sequence.
	dz.mu.Lock()
	ids := append([]string(nil), dz.artistIDs...)
	dz.mu.Unlock()
	sort.Strings(ids)
	want := []string{"101", "103"}
	if len(ids) != len(want) {
		t.Fatalf("artistIDs = %v, want %v in any order", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("artistIDs = %v, want %v in any order", ids, want)
		}
	}

	records := decodeLogRecords(t, buf)
	var skipRecord map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == "mbid-2" {
			skipRecord = rec
		}
	}
	if skipRecord == nil {
		t.Fatal("no log record found for the skipped artist (mbid-2)")
	}
	if skipRecord["level"] != "INFO" {
		t.Fatalf("skip record level = %v, want INFO", skipRecord["level"])
	}
	msg, _ := skipRecord["msg"].(string)
	if !strings.Contains(strings.ToLower(msg), "deezer_id") {
		t.Fatalf("skip record message %q does not mention the missing deezer_id reason", msg)
	}
}

func TestDeezerCycle_LogsStructuredResult(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One", DeezerID: deezerID("101")}}, nil
	}}
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		return []deezer.Album{{ID: 1}, {ID: 2}, {ID: 3}}, nil
	}}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	records := decodeLogRecords(t, buf)
	var found map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == "mbid-1" {
			found = rec
		}
	}
	if found == nil {
		t.Fatal("no log record found for artist_mbid=mbid-1")
	}
	if found["source"] != sourceDeezer {
		t.Fatalf("source = %v, want %q", found["source"], sourceDeezer)
	}
	itemCount, ok := found["item_count"].(float64)
	if !ok || itemCount != 3 {
		t.Fatalf("item_count = %v, want 3", found["item_count"])
	}
	if _, ok := found["cycle_id"].(string); !ok {
		t.Fatalf("cycle_id missing or not a string: %v", found["cycle_id"])
	}
}

func TestDeezerCycle_CycleIDSharedAcrossRecordsDiffersBetweenCycles(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle (1): %v", err)
	}
	firstRecords := decodeLogRecords(t, buf)
	if len(firstRecords) == 0 {
		t.Fatal("expected at least one log record for the first cycle")
	}
	firstCycleID, _ := firstRecords[0]["cycle_id"].(string)
	for _, rec := range firstRecords {
		if rec["cycle_id"] != firstCycleID {
			t.Fatalf("record cycle_id = %v, want %q", rec["cycle_id"], firstCycleID)
		}
	}

	buf.Reset()
	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle (2): %v", err)
	}
	secondRecords := decodeLogRecords(t, buf)
	if len(secondRecords) == 0 {
		t.Fatal("expected at least one log record for the second cycle")
	}
	secondCycleID, _ := secondRecords[0]["cycle_id"].(string)
	if secondCycleID == firstCycleID {
		t.Fatalf("second cycle_id (%q) must differ from first (%q)", secondCycleID, firstCycleID)
	}
}

func TestDeezerCycle_PerArtistErrorContinuesCycle(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		if artistID == "101" {
			return nil, errors.New("upstream exploded")
		}
		return []deezer.Album{}, nil
	}}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	err := p.RunDeezerCycle(context.Background())
	if err != nil {
		t.Fatalf("RunDeezerCycle returned %v, want nil", err)
	}
	// threeEntries has one nil-DeezerID entry (skipped, no call) and two
	// non-nil entries -- both must still be attempted despite the first
	// one erroring.
	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("calls = %d, want 2", got)
	}

	records := decodeLogRecords(t, buf)
	var errRecord map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == "mbid-1" && rec["level"] == "ERROR" {
			errRecord = rec
		}
	}
	if errRecord == nil {
		t.Fatalf("no error-level record found for artist_mbid=mbid-1; records=%v", records)
	}
}

func TestDeezerCycle_ListErrorReturnsZeroCallsNonNilError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, errors.New("db down") }}
	dz := &fakeAlbumSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	err := p.RunDeezerCycle(context.Background())
	if err == nil {
		t.Fatal("expected non-nil error when store.List fails")
	}
	if got := atomic.LoadInt32(&dz.calls); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}

func TestDeezerCycle_EmptyWatchlistNoCallsNilError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	dz := &fakeAlbumSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&dz.calls); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}

// TestDeezerCycle_ContextCancelledStopsIteration pins the cancellation
// contract, mirroring TestMusicBrainzCycle_ContextCancelledStopsIteration's
// own comment: WithDeezerWorkers(1) is required, not the newTestPoller
// default (5), because with pool size >= the 2 Deezer-capable entries in
// threeEntries() the dispatch loop's semaphore send never blocks, making
// cancellation observation a goroutine-scheduling race rather than a
// deterministic happens-before edge -- confirmed empirically flaky (3/100
// failures) before this fix. Forcing genuine semaphore contention (1
// worker, so dispatching mbid-3 must wait for mbid-1's worker to release
// its slot) makes mbid-1's cancel()-then-return strictly precede mbid-3's
// dispatch.
func TestDeezerCycle_ContextCancelledStopsIteration(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	ctx, cancel := context.WithCancel(context.Background())
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		if artistID == "101" {
			cancel()
		}
		return []deezer.Album{}, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithDeezerWorkers(1))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cycleErr := p.RunDeezerCycle(ctx)
	if !errors.Is(cycleErr, context.Canceled) {
		t.Fatalf("RunDeezerCycle error = %v, want context.Canceled", cycleErr)
	}
	if got := atomic.LoadInt32(&dz.calls); got >= 2 {
		t.Fatalf("calls = %d, want < 2 (iteration must stop once ctx is cancelled)", got)
	}
}

// TestDeezerCycle_ConcurrencyBoundedByWorkerCount is RunDeezerCycle's own
// canonical proof for PERF-01, mirroring
// TestMusicBrainzCycle_ConcurrencyBoundedByWorkerCount: over 9
// Deezer-capable entries and a worker pool bounded to 4
// (WithDeezerWorkers(4)), the fan-out must reach the ceiling (proving it
// actually ran concurrently) and never exceed it (proving the bound is
// enforced). fakeAlbumSource's fetch sleeps briefly so overlapping
// in-flight calls are actually observable.
func TestDeezerCycle_ConcurrencyBoundedByWorkerCount(t *testing.T) {
	var entries []watchlist.Entry
	for i := 1; i <= 9; i++ {
		id := fmt.Sprintf("%d0%d", i, i)
		entries = append(entries, watchlist.Entry{MBID: fmt.Sprintf("mbid-%d", i), Name: fmt.Sprintf("Artist %d", i), DeezerID: deezerID(id)})
	}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return entries, nil }}
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithDeezerWorkers(4))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	if got := atomic.LoadInt32(&dz.maxInFlight); got != 4 {
		t.Fatalf("max in-flight = %d, want exactly 4", got)
	}
}

// TestDeezerCycle_WorkerCountEqualToEntryCountRunsAllConcurrently pins the
// PERF-02 adjacency case: when the configured worker count exactly equals
// the entry count, every entry runs concurrently -- maxInFlight equals the
// entry count exactly, with no worker left queued.
func TestDeezerCycle_WorkerCountEqualToEntryCountRunsAllConcurrently(t *testing.T) {
	entries := []watchlist.Entry{
		{MBID: "mbid-1", Name: "Artist One", DeezerID: deezerID("101")},
		{MBID: "mbid-2", Name: "Artist Two", DeezerID: deezerID("102")},
		{MBID: "mbid-3", Name: "Artist Three", DeezerID: deezerID("103")},
	}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return entries, nil }}
	release := make(chan struct{})
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		<-release
		return nil, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, &fakeEventRecorder{}, &fakeNotifier{}, 15*time.Minute, logger, WithDeezerWorkers(3))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunDeezerCycle(context.Background()) }()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&dz.inFlight) < 3 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for 3 in-flight calls, last observed %d", atomic.LoadInt32(&dz.inFlight))
		}
		time.Sleep(time.Millisecond)
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunDeezerCycle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunDeezerCycle did not return promptly after release -- possible deadlock on an unused semaphore slot")
	}

	if got := atomic.LoadInt32(&dz.maxInFlight); got != 3 {
		t.Fatalf("max in-flight = %d, want exactly 3 (worker count == entry count)", got)
	}
}

// TestDeezerCycle_NilDeezerIDConsumesNoWorkerSlot proves a nil-DeezerID
// entry never occupies a worker slot or spawns a goroutine (D-06): a
// watchlist where the number of nil-DeezerID entries exceeds the worker
// count must still reach the non-nil entries' worker ceiling and call
// ArtistAlbums exactly once per non-nil entry.
func TestDeezerCycle_NilDeezerIDConsumesNoWorkerSlot(t *testing.T) {
	entries := []watchlist.Entry{
		{MBID: "mbid-1", Name: "Artist One", DeezerID: nil},
		{MBID: "mbid-2", Name: "Artist Two", DeezerID: nil},
		{MBID: "mbid-3", Name: "Artist Three", DeezerID: nil},
		{MBID: "mbid-4", Name: "Artist Four", DeezerID: deezerID("104")},
		{MBID: "mbid-5", Name: "Artist Five", DeezerID: deezerID("105")},
	}
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return entries, nil }}
	release := make(chan struct{})
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		<-release
		return nil, nil
	}}
	events := &fakeEventRecorder{}
	logger, buf := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, events, &fakeNotifier{}, 15*time.Minute, logger, WithDeezerWorkers(2))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunDeezerCycle(context.Background()) }()

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&dz.inFlight) < 2 {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for 2 in-flight calls, last observed %d", atomic.LoadInt32(&dz.inFlight))
		}
		time.Sleep(time.Millisecond)
	}

	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunDeezerCycle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunDeezerCycle did not return promptly after release")
	}

	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("dz.calls = %d, want 2 (only the non-nil DeezerID entries)", got)
	}
	dz.mu.Lock()
	gotIDs := append([]string(nil), dz.artistIDs...)
	dz.mu.Unlock()
	sort.Strings(gotIDs)
	wantIDs := []string{"104", "105"}
	if len(gotIDs) != len(wantIDs) || gotIDs[0] != wantIDs[0] || gotIDs[1] != wantIDs[1] {
		t.Fatalf("dz.artistIDs = %v, want %v in any order", gotIDs, wantIDs)
	}
	if got := atomic.LoadInt32(&events.deezerCalls); got != 2 {
		t.Fatalf("events.deezerCalls = %d, want 2", got)
	}

	records := decodeLogRecords(t, buf)
	skipCount := 0
	for _, rec := range records {
		if rec["artist_mbid"] == "mbid-1" || rec["artist_mbid"] == "mbid-2" || rec["artist_mbid"] == "mbid-3" {
			msg, _ := rec["msg"].(string)
			if strings.Contains(strings.ToLower(msg), "deezer_id") {
				skipCount++
			}
		}
	}
	if skipCount != 3 {
		t.Fatalf("found %d skip log records for nil-DeezerID entries, want 3", skipCount)
	}
}

// TestDeezerCycle_LogsCycleDurationAndArtistCount is
// TestMusicBrainzCycle_LogsCycleDurationAndArtistCount's Deezer twin,
// pinning D-04/D-05 for RunDeezerCycle.
func TestDeezerCycle_LogsCycleDurationAndArtistCount(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle: %v", err)
	}

	records := decodeLogRecords(t, buf)
	var found map[string]any
	count := 0
	for _, rec := range records {
		if rec["msg"] == "poll cycle complete" {
			found = rec
			count++
		}
	}
	if count != 1 {
		t.Fatalf("found %d 'poll cycle complete' records, want exactly 1", count)
	}

	artistCount, ok := found["artist_count"].(float64)
	if !ok {
		t.Fatalf("artist_count = %v (%T), want a JSON number", found["artist_count"], found["artist_count"])
	}
	if int(artistCount) != 3 {
		t.Fatalf("artist_count = %v, want 3", artistCount)
	}

	durationMS, ok := found["duration_ms"].(float64)
	if !ok {
		t.Fatalf("duration_ms = %v (%T), want a JSON number", found["duration_ms"], found["duration_ms"])
	}
	if durationMS < 0 {
		t.Fatalf("duration_ms = %v, want >= 0", durationMS)
	}
	if durationMS != math.Trunc(durationMS) {
		t.Fatalf("duration_ms = %v, want an integer (no fractional part)", durationMS)
	}
}

func TestNew_RejectsNonPositiveInterval(t *testing.T) {
	logger, _ := newTestLogger()
	store := &stubStore{}
	for _, interval := range []time.Duration{0, -1 * time.Second} {
		if _, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, interval, logger); err == nil {
			t.Fatalf("New with interval=%s: expected error, got nil", interval)
		}
	}
}

// --- EventRecorder-wired overlap guard and error isolation (04-01 task 3) ---
//
// These prove the same overlap-guard and error-isolation contracts the
// existing "Overlap guard (D-09)" section below already covers for the
// MusicBrainz *fetch* step, but now with detection wired into the cycle:
// blocking happens inside the recorder call, not the fetch call, and a
// detection error (not just a fetch error) must be isolated per artist.

func TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	events := &fakeEventRecorder{fn: func(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunMusicBrainzCycle(context.Background()) }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first cycle to block inside the detection call")
	}

	err = p.RunMusicBrainzCycle(context.Background())
	if !errors.Is(err, ErrCycleInProgress) {
		t.Fatalf("second RunMusicBrainzCycle error = %v, want ErrCycleInProgress", err)
	}
	// store.listCalls, not events.calls: with the default worker pool, all
	// 3 entries can be concurrently in flight and blocked inside events.fn
	// by the time the "started" signal fires, and the still-running first
	// cycle's own already-dispatched workers keep incrementing events.calls
	// in the background regardless of what the second call does -- that
	// makes any snapshot-based events.calls comparison inherently racy
	// against the first cycle's own progress, not a measurement of the
	// second call's behavior. store.List is the first thing the cycle body
	// does after the CAS guard succeeds, so store.listCalls staying at 1
	// is the deterministic proof the second, skipped call never performed
	// any work at all -- not just zero detection calls.
	if got := atomic.LoadInt32(&store.listCalls); got != 1 {
		t.Fatalf("store.listCalls = %d, want 1 (the skipped tick must perform zero store reads)", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first RunMusicBrainzCycle: %v", err)
	}
}

func TestPoller_RunMusicBrainzCycle_GuardReleasedAfterDetectionError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	events := &fakeEventRecorder{fn: func(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
		return errors.New("detection exploded")
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("first RunMusicBrainzCycle returned %v, want nil (a detection error is not a cycle failure)", err)
	}
	if p.mbRunning.Load() {
		t.Fatal("guard must be released after a cycle whose every artist's detection call errored")
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("second RunMusicBrainzCycle: %v, want nil (guard must have released after the first)", err)
	}
	if got := atomic.LoadInt32(&events.calls); got != 6 {
		t.Fatalf("events.calls = %d, want 6 (2 cycles x 3 entries)", got)
	}
}

func TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	events := &fakeEventRecorder{deezerFn: func(ctx context.Context, entry watchlist.Entry, albums []deezer.Album) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunDeezerCycle(context.Background()) }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first cycle to block inside the detection call")
	}

	err = p.RunDeezerCycle(context.Background())
	if !errors.Is(err, ErrCycleInProgress) {
		t.Fatalf("second RunDeezerCycle error = %v, want ErrCycleInProgress", err)
	}
	// store.listCalls, not events.deezerCalls: with the default worker pool,
	// both Deezer-capable entries can be concurrently in flight and blocked
	// inside events.deezerFn by the time the "started" signal fires, and the
	// still-running first cycle's own already-dispatched workers keep
	// incrementing events.deezerCalls in the background regardless of what
	// the second call does -- that makes any snapshot-based
	// events.deezerCalls comparison inherently racy against the first
	// cycle's own progress, not a measurement of the second call's
	// behavior. store.List is the first thing the cycle body does after the
	// CAS guard succeeds, so store.listCalls staying at 1 is the
	// deterministic proof the second, skipped call never performed any work
	// at all -- not just zero detection calls.
	if got := atomic.LoadInt32(&store.listCalls); got != 1 {
		t.Fatalf("store.listCalls = %d, want 1 (the skipped tick must perform zero store reads)", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first RunDeezerCycle: %v", err)
	}
}

func TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	dz := &fakeAlbumSource{}
	events := &fakeEventRecorder{deezerFn: func(ctx context.Context, entry watchlist.Entry, albums []deezer.Album) error {
		return errors.New("detection exploded")
	}}
	logger, _ := newTestLogger()
	p, err := New(store, &fakeReleaseGroupSource{}, dz, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("first RunDeezerCycle returned %v, want nil (a detection error is not a cycle failure)", err)
	}
	if p.dzRunning.Load() {
		t.Fatal("guard must be released after a cycle whose every artist's detection call errored")
	}

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("second RunDeezerCycle: %v, want nil (guard must have released after the first)", err)
	}
	if got := atomic.LoadInt32(&events.deezerCalls); got != 4 {
		t.Fatalf("events.deezerCalls = %d, want 4 (2 cycles x 2 entries with a non-nil DeezerID)", got)
	}
}

func TestPoller_RunMusicBrainzCycle_DetectionErrorIsolatedPerArtist(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{
			{MBID: "mbid-1", Name: "Artist One"},
			{MBID: "mbid-2", Name: "Artist Two"},
		}, nil
	}}
	mb := &fakeReleaseGroupSource{}
	failOn := "mbid-1"
	events := &fakeEventRecorder{fn: func(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
		if entry.MBID == failOn {
			return errors.New("detection exploded for this artist")
		}
		return nil
	}}
	logger, buf := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle returned %v, want nil (a per-artist detection failure is not a cycle failure)", err)
	}

	if got := atomic.LoadInt32(&events.calls); got != 2 {
		t.Fatalf("events.calls = %d, want 2 (both artists must still be attempted)", got)
	}
	// Membership, not order: under concurrent fan-out the two workers can
	// finish in either order, so asserting the set {mbid-1, mbid-2} was
	// attempted (PERF-03's actual guarantee) rather than a fixed
	// [mbid-1, mbid-2] sequence is what the concurrent design can promise.
	events.mu.Lock()
	gotMBIDs := append([]string(nil), events.mbids...)
	events.mu.Unlock()
	sort.Strings(gotMBIDs)
	wantMBIDs := []string{"mbid-1", "mbid-2"}
	if len(gotMBIDs) != len(wantMBIDs) || gotMBIDs[0] != wantMBIDs[0] || gotMBIDs[1] != wantMBIDs[1] {
		t.Fatalf("events.mbids = %v, want %v in any order (the second artist's detection must still run)", gotMBIDs, wantMBIDs)
	}

	records := decodeLogRecords(t, buf)
	var errRecord map[string]any
	for _, rec := range records {
		if rec["artist_mbid"] == failOn && rec["level"] == "ERROR" && rec["detection_error"] != nil {
			errRecord = rec
		}
	}
	if errRecord == nil {
		t.Fatalf("no error-level record with detection_error found for artist_mbid=%s; records=%v", failOn, records)
	}
}

func TestPoller_RunMusicBrainzCycle_EmptyWatchlist(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	mb := &fakeReleaseGroupSource{}
	events := &fakeEventRecorder{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&events.calls); got != 0 {
		t.Fatalf("events.calls = %d, want 0 (an empty watchlist writes nothing)", got)
	}
	if p.mbRunning.Load() {
		t.Fatal("guard must be released after an empty-watchlist cycle")
	}
}

func TestPoller_CyclesAreIndependentAcrossSources(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	events := &fakeEventRecorder{fn: func(ctx context.Context, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	dz := &fakeAlbumSource{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, dz, events, &fakeNotifier{}, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- p.RunMusicBrainzCycle(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the musicbrainz cycle to block inside the detection call")
	}

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle while musicbrainz cycle is blocked inside detection: %v, want nil (the guards are independent, D-08)", err)
	}
	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("deezer calls = %d, want 2", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("musicbrainz cycle: %v", err)
	}
}

// --- Overlap guard (D-09) ---

func TestMusicBrainzCycle_OverlapGuard_SkipsWhileInFlight(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return []musicbrainz.ReleaseGroup{}, nil
	}}
	logger, buf := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	done := make(chan error, 1)
	go func() { done <- p.RunMusicBrainzCycle(context.Background()) }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first cycle to start")
	}

	err := p.RunMusicBrainzCycle(context.Background())
	if !errors.Is(err, ErrCycleInProgress) {
		t.Fatalf("second RunMusicBrainzCycle error = %v, want ErrCycleInProgress", err)
	}
	// store.listCalls is the deterministic proof here, not mb.calls: with
	// the default worker pool, more than one entry can be concurrently in
	// flight and blocked inside mb.fn by the time the "started" signal
	// fires, and the still-running first cycle's own already-dispatched
	// workers keep incrementing mb.calls in the background regardless of
	// what the second call does -- a snapshot-based mb.calls comparison
	// would be racing the first cycle's own progress, not measuring the
	// second call's behavior. store.List is the first thing the cycle body
	// does after the CAS guard succeeds, so store.listCalls staying at 1
	// proves the second, skipped call never performed any work at all --
	// not just zero source calls.
	if got := atomic.LoadInt32(&store.listCalls); got != 1 {
		t.Fatalf("store.listCalls = %d, want 1 (the skipped tick must perform zero store reads)", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first RunMusicBrainzCycle: %v", err)
	}

	var warnFound bool
	for _, rec := range decodeLogRecords(t, buf) {
		if rec["level"] == "WARN" && rec["source"] == sourceMusicBrainz {
			warnFound = true
		}
	}
	if !warnFound {
		t.Fatal("expected a warn-level log record naming the source for the skipped overlapping tick (D-09)")
	}
}

func TestMusicBrainzCycle_GuardReleasesAfterCompletion_ThirdCallRunsNormally(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	for i := 0; i < 3; i++ {
		if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if got := atomic.LoadInt32(&mb.calls); got != 9 {
		t.Fatalf("calls = %d, want 9 (3 cycles x 3 entries -- the guard must release after every completed cycle)", got)
	}
}

func TestMusicBrainzCycle_GuardReleasesOnError(t *testing.T) {
	callCount := 0
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		callCount++
		if callCount == 1 {
			return nil, errors.New("db down")
		}
		return nil, nil
	}}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err == nil {
		t.Fatal("expected an error from the first call")
	}
	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("second call after an error-returning cycle should run normally, got %v", err)
	}
}

func TestMusicBrainzCycle_GuardReleasesOnPanic(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One"}}, nil
	}}
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		panic("boom")
	}}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	func() {
		defer func() { _ = recover() }()
		_ = p.RunMusicBrainzCycle(context.Background())
	}()

	if p.mbRunning.Load() {
		t.Fatal("guard must be released after a panicking cycle -- a wedged flag would silently stop the source polling for the process's lifetime")
	}

	mb.fn = nil
	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle after panic recovery: %v", err)
	}
}

func TestDeezerCycle_RunsIndependentlyDuringMusicBrainzCycle(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return []musicbrainz.ReleaseGroup{}, nil
	}}
	dz := &fakeAlbumSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, dz, logger)

	done := make(chan error, 1)
	go func() { done <- p.RunMusicBrainzCycle(context.Background()) }()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the musicbrainz cycle to start")
	}

	if err := p.RunDeezerCycle(context.Background()); err != nil {
		t.Fatalf("RunDeezerCycle while musicbrainz cycle is in flight: %v, want nil (the guards are independent, D-08)", err)
	}
	if got := atomic.LoadInt32(&dz.calls); got != 2 {
		t.Fatalf("deezer calls = %d, want 2", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("musicbrainz cycle: %v", err)
	}
}

// --- Cron registration ---

func TestNew_RegistersTwoIndependentCronEntries(t *testing.T) {
	logger, _ := newTestLogger()
	store := &stubStore{}
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, logger)

	if got := len(p.cron.Entries()); got != 2 {
		t.Fatalf("len(cron.Entries()) = %d, want 2", got)
	}
}

func TestNew_RegistersEveryIntervalSpecOnBothEntries(t *testing.T) {
	logger, _ := newTestLogger()
	for _, interval := range []time.Duration{15 * time.Minute, 90 * time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			store := &stubStore{}
			p, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, interval, logger)
			if err != nil {
				t.Fatalf("New(interval=%s): %v", interval, err)
			}
			entries := p.cron.Entries()
			if len(entries) != 2 {
				t.Fatalf("len(cron.Entries()) = %d, want 2", len(entries))
			}
			for _, entry := range entries {
				schedule, ok := entry.Schedule.(cron.ConstantDelaySchedule)
				if !ok {
					t.Fatalf("entry.Schedule = %T, want cron.ConstantDelaySchedule (i.e. an @every spec)", entry.Schedule)
				}
				if schedule.Delay != interval {
					t.Fatalf("entry delay = %s, want %s", schedule.Delay, interval)
				}
			}
		})
	}
}

func TestNew_ZeroOrNegativeIntervalRegistersNoCronEntry(t *testing.T) {
	logger, _ := newTestLogger()
	store := &stubStore{}
	for _, interval := range []time.Duration{0, -1 * time.Second} {
		p, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, interval, logger)
		if err == nil {
			t.Fatalf("New(interval=%s): expected error, got nil", interval)
		}
		if p != nil {
			t.Fatalf("New(interval=%s): expected nil Poller on error, got %+v", interval, p)
		}
	}
}

// --- Start / Stop ---
//
// cron.Cron.Stop()'s returned context only tracks jobs its own dispatch
// loop started (via an internal sync.WaitGroup) -- a cycle invoked directly
// by test code, bypassing cron, is invisible to it. So unlike the overlap
// guard tests above, these three drive a real (short) interval and wait for
// an actual cron tick, mirroring task 3's own real-tick lifecycle test.

func TestStop_ReturnsNilOnceInFlightCycleFinishes(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One"}}, nil
	}}
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return []musicbrainz.ReleaseGroup{}, nil
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 50*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Start(context.Background())

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a real cron tick to start a cycle")
	}

	stopDone := make(chan error, 1)
	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		stopDone <- p.Stop(stopCtx)
	}()

	time.Sleep(50 * time.Millisecond)
	close(release)

	if err := <-stopDone; err != nil {
		t.Fatalf("Stop() = %v, want nil once the in-flight cycle drains", err)
	}
}

func TestStop_ReturnsCallerContextErrorWhenCycleOutlivesIt(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{{MBID: "mbid-1", Name: "Artist One"}}, nil
	}}
	started := make(chan struct{}, 1)
	mb := &fakeReleaseGroupSource{fn: func(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		// Blocks until Stop cancels the retained cycle context (which only
		// happens once the caller's Stop context has itself expired) --
		// this is what proves Stop is bounded by the caller's context
		// rather than blocking forever on a hung cycle.
		<-ctx.Done()
		return nil, ctx.Err()
	}}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 50*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Start(context.Background())

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a real cron tick to start a cycle")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = p.Stop(stopCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Stop() = %v, want context.DeadlineExceeded (the caller's context must bound the wait)", err)
	}
}

func TestStop_NoFurtherCycleBeginsAfterStop(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, &fakeNotifier{}, 30*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.Start(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&store.listCalls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for at least one real cron tick")
		}
		time.Sleep(5 * time.Millisecond)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop(): %v", err)
	}

	before := atomic.LoadInt32(&store.listCalls)
	time.Sleep(150 * time.Millisecond) // several intervals' worth, had Stop not taken effect
	after := atomic.LoadInt32(&store.listCalls)
	if after != before {
		t.Fatalf("store.listCalls changed from %d to %d after Stop -- a cycle began after Stop returned", before, after)
	}
}

// TestPoller_StartStop_LifecycleWithRealCronTick is the only test in this
// file that exercises a real cron tick end to end (Start -> at least one
// real dispatch -> Stop) rather than driving a cycle method directly --
// kept to a short interval and an explicit deadline so it stays well under
// a second.
func TestPoller_StartStop_LifecycleWithRealCronTick(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	mb := &fakeReleaseGroupSource{}
	dz := &fakeAlbumSource{}
	logger, _ := newTestLogger()
	p, err := New(store, mb, dz, &fakeEventRecorder{}, &fakeNotifier{}, 50*time.Millisecond, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	p.Start(context.Background())

	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&store.listCalls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for at least one real cron tick to drive a cycle")
		}
		time.Sleep(5 * time.Millisecond)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.Stop(stopCtx); err != nil {
		t.Fatalf("Stop() = %v, want nil", err)
	}

	before := atomic.LoadInt32(&store.listCalls)
	time.Sleep(150 * time.Millisecond)
	after := atomic.LoadInt32(&store.listCalls)
	if after != before {
		t.Fatalf("store.listCalls changed from %d to %d after Stop -- no further calls should arrive once Stop has returned", before, after)
	}
}
