package httpserver_test

// This file proves GET /search (WLST-01, CLNT-03, D-01/D-02/D-03) against
// stubSearchSource doubles -- no real musicbrainz.Client is involved here;
// internal/musicbrainz/search_test.go already covers the real client
// against an httptest.Server.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/httpserver"
)

// stubSearchSource is a file-local double for httpserver.SearchSource,
// mirroring stubStore/stubPinger's func-field pattern.
type stubSearchSource struct {
	name       string
	searchFunc func(ctx context.Context, q string, limit int) ([]httpserver.SearchArtist, error)
}

func (s stubSearchSource) Name() string { return s.name }

func (s stubSearchSource) SearchArtists(ctx context.Context, q string, limit int) ([]httpserver.SearchArtist, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, q, limit)
	}
	return nil, nil
}

var _ httpserver.SearchSource = stubSearchSource{}

// searchResponseBody mirrors the GET /search response envelope by field
// name (D-01/D-02).
type searchResponseBody struct {
	Query   string `json:"query"`
	Sources map[string]struct {
		Status  string                    `json:"status"`
		Error   string                    `json:"error"`
		Artists []httpserver.SearchArtist `json:"artists"`
	} `json:"sources"`
}

func TestSearch_Success(t *testing.T) {
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return []httpserver.SearchArtist{{Source: "musicbrainz", ID: "9fff2f8a-21e6-47de-a2b8-7f449929d43f", Name: "Drake"}}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body searchResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Query != "drake" {
		t.Fatalf("query = %q, want %q", body.Query, "drake")
	}
	mb, ok := body.Sources["musicbrainz"]
	if !ok {
		t.Fatal("sources.musicbrainz missing")
	}
	if mb.Status != "ok" {
		t.Fatalf("sources.musicbrainz.status = %q, want %q", mb.Status, "ok")
	}
	if len(mb.Artists) != 1 || mb.Artists[0].Source != "musicbrainz" {
		t.Fatalf("sources.musicbrainz.artists = %+v, want one entry with source musicbrainz", mb.Artists)
	}
}

func TestSearch_SourceErrorReturns200WithErrorStatus(t *testing.T) {
	const rawErr = "connection refused: upstream-error-marker"
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New(rawErr)
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (a degraded source must never fail the whole request)", resp.StatusCode, http.StatusOK)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var body searchResponseBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	mb, ok := body.Sources["musicbrainz"]
	if !ok {
		t.Fatal("sources.musicbrainz missing")
	}
	if mb.Status != "error" {
		t.Fatalf("sources.musicbrainz.status = %q, want %q", mb.Status, "error")
	}
	if mb.Error != "source unavailable" {
		t.Fatalf("sources.musicbrainz.error = %q, want %q", mb.Error, "source unavailable")
	}
	if len(mb.Artists) != 0 {
		t.Fatalf("sources.musicbrainz.artists = %+v, want empty", mb.Artists)
	}

	if strings.Contains(string(data), rawErr) {
		t.Fatalf("response body leaked raw upstream error text: %s", data)
	}
}

func TestSearch_MissingOrBlankQReturns400(t *testing.T) {
	paths := []string{"/search", "/search?q=", "/search?q=%20%20"}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			called := false
			src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
				called = true
				return nil, nil
			}}
			srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			resp, err := http.Get(ts.URL + p)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			var eb errorBody
			if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if eb.Error != "q is required" {
				t.Fatalf("error = %q, want %q", eb.Error, "q is required")
			}
			if called {
				t.Fatal("source was called for an empty/blank q")
			}
		})
	}
}

func TestSearch_QOverLongReturns400(t *testing.T) {
	over := strings.Repeat("a", 513)

	src := stubSearchSource{name: "musicbrainz"}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=" + over)
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	var eb errorBody
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if eb.Error != "q is too long" {
		t.Fatalf("error = %q, want %q", eb.Error, "q is too long")
	}
}

func TestSearch_QExactly512RunesReturns200(t *testing.T) {
	exact := strings.Repeat("a", 512)

	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=" + exact)
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSearch_ZeroArtistsReturnsEmptyArrayNeverNull(t *testing.T) {
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=zzzznomatch")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(data), `"artists":[]`) {
		t.Fatalf("body = %s, want artists to encode as [] not null", data)
	}
}

func TestSearch_FanOutIsConcurrent(t *testing.T) {
	release := make(chan struct{})
	var mu sync.Mutex
	inFlight := 0
	maxInFlight := 0

	makeSrc := func(name string) stubSearchSource {
		return stubSearchSource{name: name, searchFunc: func(ctx context.Context, q string, limit int) ([]httpserver.SearchArtist, error) {
			mu.Lock()
			inFlight++
			if inFlight > maxInFlight {
				maxInFlight = inFlight
			}
			mu.Unlock()
			<-release
			mu.Lock()
			inFlight--
			mu.Unlock()
			return nil, nil
		}}
	}

	sources := []httpserver.SearchSource{makeSrc("musicbrainz"), makeSrc("deezer")}
	srv := httpserver.New(noopPinger{}, stubStore{}, sources, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	done := make(chan struct{})
	go func() {
		resp, err := http.Get(ts.URL + "/search?q=drake")
		if err == nil {
			resp.Body.Close()
		}
		close(done)
	}()

	// Give both source goroutines a chance to reach the blocking point
	// before inspecting maxInFlight.
	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	got := maxInFlight
	mu.Unlock()
	close(release)
	<-done

	if got < 2 {
		t.Fatalf("max concurrent in-flight sources = %d, want 2 (fan-out must be concurrent, not sequential)", got)
	}
}

// TestSearch_PartialFailure_OneSourceDownAnotherHealthy proves D-03 with
// two stub sources (plan 03-02 supplies the real second source, Deezer):
// a failing source's status must never suppress a healthy source's
// results, and the failing source's entry must leak neither the stub's raw
// error text nor "musicbrainz.org" (T-03-01, V13).
func TestSearch_PartialFailure_OneSourceDownAnotherHealthy(t *testing.T) {
	const rawErr = "connection refused: upstream-error-marker"
	failing := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New(rawErr)
	}}
	healthy := stubSearchSource{name: "deezer", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return []httpserver.SearchArtist{{Source: "deezer", ID: "246791", Name: "Drake"}}, nil
	}}

	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{failing, healthy}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var body searchResponseBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	mb, ok := body.Sources["musicbrainz"]
	if !ok {
		t.Fatal("sources.musicbrainz missing")
	}
	if mb.Status != "error" {
		t.Fatalf("sources.musicbrainz.status = %q, want %q", mb.Status, "error")
	}

	dz, ok := body.Sources["deezer"]
	if !ok {
		t.Fatal("sources.deezer missing")
	}
	if dz.Status != "ok" {
		t.Fatalf("sources.deezer.status = %q, want %q -- one degraded source must never suppress another", dz.Status, "ok")
	}
	if len(dz.Artists) != 1 || dz.Artists[0].Name != "Drake" {
		t.Fatalf("sources.deezer.artists = %+v, want the healthy source's full artist list", dz.Artists)
	}

	raw := string(data)
	for _, leak := range []string{rawErr, "musicbrainz.org"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("response body leaked %q: %s", leak, raw)
		}
	}
}

// TestSearch_CancelledInboundRequestWritesNoPartialBody proves that
// cancelling the inbound request mid-flight -- while a source is still
// in-flight -- never causes handleSearch to emit a truncated JSON body:
// WriteHeader only happens after the WaitGroup joins, so the recorder's
// body is always either empty or one complete, json.Valid document.
func TestSearch_CancelledInboundRequestWritesNoPartialBody(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(ctx context.Context, q string, limit int) ([]httpserver.SearchArtist, error) {
		close(entered)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-release:
			return nil, nil
		}
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, []httpserver.SearchSource{src}, discardLogger())

	req := httptest.NewRequest(http.MethodGet, "/search?q=drake", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.Router().ServeHTTP(rec, req)
		close(done)
	}()

	<-entered
	cancel()
	<-done
	close(release)

	body := rec.Body.Bytes()
	if len(body) != 0 && !json.Valid(body) {
		t.Fatalf("body = %q, want either empty or a single complete JSON document", body)
	}
}
