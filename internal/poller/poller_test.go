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
	"log/slog"
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
	p, err := New(store, mb, dz, er, 15*time.Minute, logger)
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

	p, err := New(store, mb, &fakeAlbumSource{}, recorder, 15*time.Minute, logger)
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

	p, err := New(store, &fakeReleaseGroupSource{}, dz, recorder, 15*time.Minute, logger)
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
		if _, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, interval, logger); err == nil {
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
	p, err := New(store, mb, &fakeAlbumSource{}, events, 15*time.Minute, logger)
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
	if got := atomic.LoadInt32(&events.calls); got != 1 {
		t.Fatalf("events.calls = %d, want 1 (the skipped tick must perform zero detection calls)", got)
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
	p, err := New(store, mb, &fakeAlbumSource{}, events, 15*time.Minute, logger)
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
	p, err := New(store, mb, &fakeAlbumSource{}, events, 15*time.Minute, logger)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if err := p.RunMusicBrainzCycle(context.Background()); err != nil {
		t.Fatalf("RunMusicBrainzCycle returned %v, want nil (a per-artist detection failure is not a cycle failure)", err)
	}

	if got := atomic.LoadInt32(&events.calls); got != 2 {
		t.Fatalf("events.calls = %d, want 2 (both artists must still be attempted)", got)
	}
	events.mu.Lock()
	gotMBIDs := append([]string(nil), events.mbids...)
	events.mu.Unlock()
	if len(gotMBIDs) != 2 || gotMBIDs[0] != "mbid-1" || gotMBIDs[1] != "mbid-2" {
		t.Fatalf("events.mbids = %v, want [mbid-1 mbid-2] (the second artist's detection must still run)", gotMBIDs)
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
	p, err := New(store, mb, &fakeAlbumSource{}, events, 15*time.Minute, logger)
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
	p, err := New(store, mb, dz, events, 15*time.Minute, logger)
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
	if got := atomic.LoadInt32(&store.listCalls); got != 1 {
		t.Fatalf("store.listCalls = %d, want 1 (the skipped tick must perform zero store reads)", got)
	}
	if got := atomic.LoadInt32(&mb.calls); got != 1 {
		t.Fatalf("mb.calls = %d, want 1 (the skipped tick must perform zero source calls)", got)
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
			p, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, interval, logger)
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
		p, err := New(store, &fakeReleaseGroupSource{}, &fakeAlbumSource{}, &fakeEventRecorder{}, interval, logger)
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
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, 50*time.Millisecond, logger)
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
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, 50*time.Millisecond, logger)
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
	p, err := New(store, mb, &fakeAlbumSource{}, &fakeEventRecorder{}, 30*time.Millisecond, logger)
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
	p, err := New(store, mb, dz, &fakeEventRecorder{}, 50*time.Millisecond, logger)
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
