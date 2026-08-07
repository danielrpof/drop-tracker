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
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

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

// stubStore is a file-local double for watchlist.Store, mirroring
// internal/httpserver/watchlist_test.go's stubStore -- every write method
// records its call count so a test can assert the poll cycle never writes
// (D-04).
type stubStore struct {
	listFunc func(ctx context.Context) ([]watchlist.Entry, error)

	addCalls    int32
	updateCalls int32
	removeCalls int32
}

func (s *stubStore) Add(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error) {
	atomic.AddInt32(&s.addCalls, 1)
	return watchlist.Entry{}, nil
}

func (s *stubStore) List(ctx context.Context) ([]watchlist.Entry, error) {
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
// real observation TestMusicBrainzCycle_Sequential asserts on.
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

func newTestPoller(t *testing.T, store watchlist.Store, mb ReleaseGroupSource, dz AlbumSource, logger *slog.Logger) *Poller {
	t.Helper()
	p, err := New(store, mb, dz, 15*time.Minute, logger)
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
	want := []string{"mbid-1", "mbid-2", "mbid-3"}
	mb.mu.Lock()
	got := append([]string(nil), mb.mbids...)
	mb.mu.Unlock()
	if len(got) != len(want) {
		t.Fatalf("mbids = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mbids[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if atomic.LoadInt32(&store.addCalls) != 0 || atomic.LoadInt32(&store.updateCalls) != 0 || atomic.LoadInt32(&store.removeCalls) != 0 {
		t.Fatal("cycle must never write to the store (D-04)")
	}
}

func TestMusicBrainzCycle_Sequential(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v", err)
	}

	if got := atomic.LoadInt32(&mb.maxInFlight); got > 1 {
		t.Fatalf("max in-flight = %d, want <= 1 (artists must be polled one at a time, D-07)", got)
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

func TestMusicBrainzCycle_EmptyWatchlistNoCallsNilError(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return nil, nil }}
	mb := &fakeReleaseGroupSource{}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&mb.calls); got != 0 {
		t.Fatalf("calls = %d, want 0", got)
	}
}

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
	p := newTestPoller(t, store, mb, &fakeAlbumSource{}, logger)

	err := p.RunMusicBrainzCycle(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunMusicBrainzCycle error = %v, want context.Canceled", err)
	}
	if got := atomic.LoadInt32(&mb.calls); got >= 3 {
		t.Fatalf("calls = %d, want < 3 (iteration must stop once ctx is cancelled)", got)
	}
}

// --- Deezer cycle ---

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
	dz.mu.Lock()
	ids := append([]string(nil), dz.artistIDs...)
	dz.mu.Unlock()
	want := []string{"101", "103"}
	if len(ids) != len(want) {
		t.Fatalf("artistIDs = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("artistIDs[%d] = %q, want %q", i, ids[i], want[i])
		}
		if ids[i] == "" {
			t.Fatal("artist id must never be empty")
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

func TestDeezerCycle_ContextCancelledStopsIteration(t *testing.T) {
	store := &stubStore{listFunc: func(ctx context.Context) ([]watchlist.Entry, error) { return threeEntries(), nil }}
	ctx, cancel := context.WithCancel(context.Background())
	dz := &fakeAlbumSource{fn: func(ctx context.Context, artistID string, limit int) ([]deezer.Album, error) {
		cancel()
		return []deezer.Album{}, nil
	}}
	logger, _ := newTestLogger()
	p := newTestPoller(t, store, &fakeReleaseGroupSource{}, dz, logger)

	err := p.RunDeezerCycle(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunDeezerCycle error = %v, want context.Canceled", err)
	}
	if got := atomic.LoadInt32(&dz.calls); got >= 2 {
		t.Fatalf("calls = %d, want < 2 (iteration must stop once ctx is cancelled)", got)
	}
}

func TestNew_RejectsNonPositiveInterval(t *testing.T) {
	logger, _ := newTestLogger()
	store := &stubStore{}
	for _, interval := range []time.Duration{0, -1 * time.Second} {
		if _, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, interval, logger); err == nil {
			t.Fatalf("New with interval=%s: expected error, got nil", interval)
		}
	}
}
