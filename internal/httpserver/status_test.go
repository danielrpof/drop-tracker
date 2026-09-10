package httpserver_test

// This file proves the GET /status body shape (STAT-01) and the skip signal
// surfacing (RUN-02): the fresh-instance empty state, the poll-interval
// encoding, the newest-first ordering, the 50-entry bound, and that
// watchlist_size is exactly the counter's return. Where the behaviour under
// test is really the run store's, the tests drive a real pollruns.Store so
// the two halves cannot drift apart.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/pollruns"
)

// fakeStatusStore is a file-local double for httpserver.StatusStore in
// stubEventsStore's func-field shape.
type fakeStatusStore struct {
	fn func() map[string]pollruns.SourceSnapshot
}

func (f fakeStatusStore) Snapshot() map[string]pollruns.SourceSnapshot {
	if f.fn != nil {
		return f.fn()
	}
	return map[string]pollruns.SourceSnapshot{}
}

var _ httpserver.StatusStore = fakeStatusStore{}

// fakeWatchlistCounter is a file-local double for httpserver.WatchlistCounter.
type fakeWatchlistCounter struct {
	fn func(context.Context) (int64, error)
}

func (f fakeWatchlistCounter) CountWatchlist(ctx context.Context) (int64, error) {
	if f.fn != nil {
		return f.fn(ctx)
	}
	return 0, nil
}

var _ httpserver.WatchlistCounter = fakeWatchlistCounter{}

// statusRunBody decodes one run object from the frozen contract by field name.
type statusRunBody struct {
	CycleID        string `json:"cycle_id"`
	StartedAt      string `json:"started_at"`
	FinishedAt     string `json:"finished_at"`
	DurationMS     int64  `json:"duration_ms"`
	ArtistsChecked int    `json:"artists_checked"`
	ArtistsSkipped int    `json:"artists_skipped"`
	ArtistsErrored int    `json:"artists_errored"`
	EventsRecorded int    `json:"events_recorded"`
	Outcome        string `json:"outcome"`
	Summary        string `json:"summary"`
}

type statusSourceBody struct {
	LastRun          *statusRunBody  `json:"last_run"`
	History          []statusRunBody `json:"history"`
	LastSkippedAt    *string         `json:"last_skipped_at"`
	ConsecutiveSkips int             `json:"consecutive_skips"`
}

// statusBody decodes the whole GET /status envelope by field name.
type statusBody struct {
	PollIntervalSeconds int   `json:"poll_interval_seconds"`
	WatchlistSize       int64 `json:"watchlist_size"`
	Instance            struct {
		AppVersion     string `json:"app_version"`
		SchemaApplied  *uint  `json:"schema_applied"`
		SchemaExpected uint   `json:"schema_expected"`
	} `json:"instance"`
	Sources map[string]statusSourceBody `json:"sources"`
}

type statusOpts struct {
	store        httpserver.StatusStore
	counter      httpserver.WatchlistCounter
	schema       httpserver.SchemaVersioner
	expected     uint
	appVersion   string
	pollInterval time.Duration
	passphrase   string
	omitStatus   bool
}

func newStatusServer(t *testing.T, o statusOpts) *httpserver.Server {
	t.Helper()
	opts := []httpserver.Option{}
	if o.passphrase != "" {
		opts = append(opts, httpserver.WithAuthGate(o.passphrase, false, nil))
	}
	if o.schema != nil || o.expected != 0 {
		opts = append(opts, httpserver.WithReadiness(o.schema, o.expected))
	}
	if !o.omitStatus {
		store := o.store
		if store == nil {
			store = fakeStatusStore{fn: func() map[string]pollruns.SourceSnapshot {
				return pollruns.NewStore().Snapshot()
			}}
		}
		counter := o.counter
		if counter == nil {
			counter = fakeWatchlistCounter{}
		}
		pi := o.pollInterval
		if pi == 0 {
			pi = 15 * time.Minute
		}
		opts = append(opts, httpserver.WithStatus(httpserver.StatusDeps{
			Store:        store,
			Counter:      counter,
			AppVersion:   o.appVersion,
			PollInterval: pi,
		}))
	}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(), opts...)
	t.Cleanup(srv.Close)
	return srv
}

func getStatusRaw(t *testing.T, srv *httpserver.Server) (int, string) {
	t.Helper()
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(raw)
}

func getStatus(t *testing.T, srv *httpserver.Server) (int, statusBody, string) {
	t.Helper()
	code, raw := getStatusRaw(t, srv)
	var body statusBody
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("decode body %q: %v", raw, err)
	}
	return code, body, raw
}

func aRun(source, cycleID string, started time.Time) pollruns.RunResult {
	return pollruns.RunResult{
		Source:         source,
		CycleID:        cycleID,
		StartedAt:      started,
		FinishedAt:     started.Add(time.Second),
		DurationMS:     1000,
		ArtistsChecked: 3,
		Outcome:        pollruns.OutcomeOK,
	}
}

func TestStatus_EmptyHistory(t *testing.T) {
	store := pollruns.NewStore()
	srv := newStatusServer(t, statusOpts{
		store:      store,
		counter:    fakeWatchlistCounter{fn: func(context.Context) (int64, error) { return 5, nil }},
		schema:     schemaAt(7, false),
		expected:   7,
		appVersion: "dev",
	})

	code, body, raw := getStatus(t, srv)
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", code, raw)
	}
	for _, src := range []string{"musicbrainz", "deezer"} {
		s, ok := body.Sources[src]
		if !ok {
			t.Fatalf("sources missing %q: %s", src, raw)
		}
		if s.LastRun != nil {
			t.Fatalf("%s.last_run = %+v, want null", src, s.LastRun)
		}
		if s.History == nil || len(s.History) != 0 {
			t.Fatalf("%s.history = %+v, want empty", src, s.History)
		}
		if s.LastSkippedAt != nil {
			t.Fatalf("%s.last_skipped_at = %v, want null", src, *s.LastSkippedAt)
		}
		if s.ConsecutiveSkips != 0 {
			t.Fatalf("%s.consecutive_skips = %d, want 0", src, s.ConsecutiveSkips)
		}
	}
	if body.WatchlistSize != 5 || body.PollIntervalSeconds != 900 {
		t.Fatalf("watchlist_size/poll_interval_seconds = %d/%d, want 5/900", body.WatchlistSize, body.PollIntervalSeconds)
	}
	if body.Instance.AppVersion != "dev" || body.Instance.SchemaApplied == nil || *body.Instance.SchemaApplied != 7 || body.Instance.SchemaExpected != 7 {
		t.Fatalf("instance = %+v, want dev / 7 / 7", body.Instance)
	}

	// The raw encoded history for an empty source must be [], not null:
	// decoding into a Go slice hides that, and Phase 19 maps over it directly.
	if !strings.Contains(raw, `"history":[]`) {
		t.Fatalf("raw body does not contain \"history\":[] : %s", raw)
	}
	if strings.Contains(raw, `"history":null`) {
		t.Fatalf("raw body encodes history as null: %s", raw)
	}
}

func TestStatus_PollInterval(t *testing.T) {
	cases := []struct {
		interval time.Duration
		want     int
	}{
		{15 * time.Minute, 900},
		{90 * time.Second, 90},
	}
	for _, tc := range cases {
		srv := newStatusServer(t, statusOpts{pollInterval: tc.interval})
		_, body, raw := getStatus(t, srv)
		if body.PollIntervalSeconds != tc.want {
			t.Fatalf("interval %s: poll_interval_seconds = %d, want %d (body %s)", tc.interval, body.PollIntervalSeconds, tc.want, raw)
		}
	}
}

func TestStatus_SkipSignal(t *testing.T) {
	store := pollruns.NewStore()
	store.RecordSkip(pollruns.SourceMusicBrainz)
	store.RecordSkip(pollruns.SourceMusicBrainz)
	store.RecordSkip(pollruns.SourceMusicBrainz)

	srv := newStatusServer(t, statusOpts{store: store})
	_, body, raw := getStatus(t, srv)

	mb := body.Sources["musicbrainz"]
	if mb.ConsecutiveSkips != 3 || mb.LastSkippedAt == nil {
		t.Fatalf("musicbrainz skip signal = %d / %v, want 3 / non-null (body %s)", mb.ConsecutiveSkips, mb.LastSkippedAt, raw)
	}
	dz := body.Sources["deezer"]
	if dz.ConsecutiveSkips != 0 || dz.LastSkippedAt != nil {
		t.Fatalf("deezer skip signal = %d / %v, want 0 / null", dz.ConsecutiveSkips, dz.LastSkippedAt)
	}
}

func TestStatus_HistoryNewestFirst(t *testing.T) {
	store := pollruns.NewStore()
	base := time.Now().Add(-time.Hour).UTC()
	for i := 0; i < 3; i++ {
		if err := store.RecordRun(context.Background(),
			aRun(pollruns.SourceMusicBrainz, cycleID(i), base.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
	}

	srv := newStatusServer(t, statusOpts{store: store})
	_, body, raw := getStatus(t, srv)

	mb := body.Sources["musicbrainz"]
	if len(mb.History) != 3 {
		t.Fatalf("history len = %d, want 3 (body %s)", len(mb.History), raw)
	}
	want := []string{cycleID(2), cycleID(1), cycleID(0)}
	for i, w := range want {
		if mb.History[i].CycleID != w {
			t.Fatalf("history[%d].cycle_id = %q, want %q (newest-first)", i, mb.History[i].CycleID, w)
		}
	}
	if mb.LastRun == nil || mb.LastRun.CycleID != cycleID(2) {
		t.Fatalf("last_run = %+v, want cycle %q", mb.LastRun, cycleID(2))
	}

	// last_run must be byte-identical to history[0].
	var rawSrc struct {
		Sources map[string]struct {
			LastRun json.RawMessage   `json:"last_run"`
			History []json.RawMessage `json:"history"`
		} `json:"sources"`
	}
	if err := json.Unmarshal([]byte(raw), &rawSrc); err != nil {
		t.Fatalf("decode raw sources: %v", err)
	}
	rmb := rawSrc.Sources["musicbrainz"]
	if !bytes.Equal(rmb.LastRun, rmb.History[0]) {
		t.Fatalf("last_run (%s) not byte-identical to history[0] (%s)", rmb.LastRun, rmb.History[0])
	}
}

func TestStatus_HistoryBounded(t *testing.T) {
	store := pollruns.NewStore()
	base := time.Now().Add(-2 * time.Hour).UTC()
	for i := 0; i < 60; i++ {
		if err := store.RecordRun(context.Background(),
			aRun(pollruns.SourceDeezer, cycleID(i), base.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
	}

	srv := newStatusServer(t, statusOpts{store: store})
	_, body, _ := getStatus(t, srv)

	if got := len(body.Sources["deezer"].History); got != 50 {
		t.Fatalf("deezer history len = %d, want 50 (the N=50 bound)", got)
	}
}

func TestStatus_WatchlistSize(t *testing.T) {
	for _, want := range []int64{0, 1, 42} {
		srv := newStatusServer(t, statusOpts{
			counter: fakeWatchlistCounter{fn: func(context.Context) (int64, error) { return want, nil }},
		})
		_, body, raw := getStatus(t, srv)
		if body.WatchlistSize != want {
			t.Fatalf("watchlist_size = %d, want %d (body %s)", body.WatchlistSize, want, raw)
		}
	}
}

func cycleID(i int) string { return "musicbrainz-" + string(rune('a'+i)) }
