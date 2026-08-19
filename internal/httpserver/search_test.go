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

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// stubDeezerArtistSearcher is a file-local double for deezer.ArtistSearcher,
// used to drive httpserver.NewDeezerSource directly rather than through a
// real *deezer.Client / httptest.Server pair -- internal/deezer/search_test.go
// already covers the real client.
type stubDeezerArtistSearcher struct {
	searchFunc func(ctx context.Context, query string, limit int) ([]deezer.Artist, error)
}

func (s stubDeezerArtistSearcher) SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, query, limit)
	}
	return nil, nil
}

var _ deezer.ArtistSearcher = stubDeezerArtistSearcher{}

// stubMusicBrainzArtistSearcher is a file-local double for
// musicbrainz.ArtistSearcher, mirroring stubDeezerArtistSearcher's
// func-field pattern -- used to drive httpserver.NewMusicBrainzSource
// directly rather than through a real *musicbrainz.Client / httptest.Server
// pair.
type stubMusicBrainzArtistSearcher struct {
	searchFunc func(ctx context.Context, query string, limit int) ([]musicbrainz.Artist, error)
}

func (s stubMusicBrainzArtistSearcher) SearchArtists(ctx context.Context, query string, limit int) ([]musicbrainz.Artist, error) {
	if s.searchFunc != nil {
		return s.searchFunc(ctx, query, limit)
	}
	return nil, nil
}

var _ musicbrainz.ArtistSearcher = stubMusicBrainzArtistSearcher{}

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
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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

// TestSearch_MusicBrainzCountryReachesResponseBody proves D-09/D-10's
// end-to-end path: a MusicBrainz artist with a blank disambiguation but a
// populated country survives client struct -> adapter -> JSON encode ->
// HTTP body, through the real router (not a stubSearchSource shortcut).
func TestSearch_MusicBrainzCountryReachesResponseBody(t *testing.T) {
	stub := stubMusicBrainzArtistSearcher{searchFunc: func(context.Context, string, int) ([]musicbrainz.Artist, error) {
		return []musicbrainz.Artist{
			{
				MBID:           "9fff2f8a-21e6-47de-a2b8-7f449929d43f",
				Name:           "Drake",
				Type:           "Person",
				Score:          100,
				Disambiguation: "",
				Country:        "CA",
			},
		}, nil
	}}
	src := httpserver.NewMusicBrainzSource(stub)
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body searchResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	mb, ok := body.Sources["musicbrainz"]
	if !ok {
		t.Fatal("sources.musicbrainz missing")
	}
	if mb.Status != "ok" {
		t.Fatalf("sources.musicbrainz.status = %q, want %q", mb.Status, "ok")
	}
	if len(mb.Artists) != 1 {
		t.Fatalf("len(sources.musicbrainz.artists) = %d, want 1", len(mb.Artists))
	}
	got := mb.Artists[0]
	if got.Disambiguation != nil {
		t.Errorf("Disambiguation = %v, want nil", got.Disambiguation)
	}
	if got.Country == nil || *got.Country != "CA" {
		t.Errorf("Country = %v, want a pointer to %q", got.Country, "CA")
	}
}

func TestSearch_SourceErrorReturns200WithErrorStatus(t *testing.T) {
	const rawErr = "connection refused: upstream-error-marker"
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New(rawErr)
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
			srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			resp, err := http.Get(ts.URL + p)
			if err != nil {
				t.Fatalf("GET %s: %v", p, err)
			}
			defer func() { _ = resp.Body.Close() }()

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
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=" + over)
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=" + exact)
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSearch_ZeroArtistsReturnsEmptyArrayNeverNull(t *testing.T) {
	src := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=zzzznomatch")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, sources, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	done := make(chan struct{})
	go func() {
		resp, err := http.Get(ts.URL + "/search?q=drake")
		if err == nil {
			_ = resp.Body.Close()
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

	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{failing, healthy}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{src}, discardLogger())

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

// TestNewDeezerSource_MapsFields proves the deezer.Artist -> SearchArtist
// mapping directly against the adapter: ID goes through
// strconv.FormatInt(a.ID, 10), ImageURL comes from Picture when non-empty,
// Disambiguation is always nil (Deezer has no equivalent field), Type falls
// back to "artist" when the upstream field is empty, and Source is always
// "deezer".
func TestNewDeezerSource_MapsFields(t *testing.T) {
	stub := stubDeezerArtistSearcher{searchFunc: func(ctx context.Context, query string, limit int) ([]deezer.Artist, error) {
		return []deezer.Artist{
			{ID: 246791, Name: "Drake", Picture: "https://api.deezer.com/artist/246791/image", Type: "artist"},
			{ID: 999, Name: "No Picture No Type", Picture: "", Type: ""},
		}, nil
	}}
	src := httpserver.NewDeezerSource(stub)

	if got := src.Name(); got != "deezer" {
		t.Fatalf("Name() = %q, want %q", got, "deezer")
	}

	artists, err := src.SearchArtists(context.Background(), "drake", 10)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("len(artists) = %d, want 2", len(artists))
	}

	first := artists[0]
	if first.Source != "deezer" {
		t.Errorf("first.Source = %q, want %q", first.Source, "deezer")
	}
	if first.ID != "246791" {
		t.Errorf("first.ID = %q, want %q", first.ID, "246791")
	}
	if first.Name != "Drake" {
		t.Errorf("first.Name = %q, want %q", first.Name, "Drake")
	}
	if first.Disambiguation != nil {
		t.Errorf("first.Disambiguation = %v, want nil", first.Disambiguation)
	}
	if first.Country != nil {
		t.Errorf("first.Country = %v, want nil (Deezer has no equivalent field)", first.Country)
	}
	if first.Type != "artist" {
		t.Errorf("first.Type = %q, want %q", first.Type, "artist")
	}
	if first.ImageURL == nil || *first.ImageURL != "https://api.deezer.com/artist/246791/image" {
		t.Errorf("first.ImageURL = %v, want a pointer to the picture URL", first.ImageURL)
	}

	second := artists[1]
	if second.ID != "999" {
		t.Errorf("second.ID = %q, want %q", second.ID, "999")
	}
	if second.Country != nil {
		t.Errorf("second.Country = %v, want nil (Deezer has no equivalent field)", second.Country)
	}
	if second.ImageURL != nil {
		t.Errorf("second.ImageURL = %v, want nil for an empty Picture", second.ImageURL)
	}
	if second.Type != "artist" {
		t.Errorf("second.Type = %q, want %q (fallback for an empty upstream type)", second.Type, "artist")
	}
}

// TestNewMusicBrainzSource_MapsFields proves the musicbrainz.Artist ->
// SearchArtist mapping directly against the adapter: source is always
// "musicbrainz", id comes from the MBID, disambiguation and country each
// map to a non-nil pointer when the upstream value is non-empty and nil
// when it is blank, image URL is always nil.
func TestNewMusicBrainzSource_MapsFields(t *testing.T) {
	stub := stubMusicBrainzArtistSearcher{searchFunc: func(context.Context, string, int) ([]musicbrainz.Artist, error) {
		return []musicbrainz.Artist{
			{MBID: "9fff2f8a-21e6-47de-a2b8-7f449929d43f", Name: "Drake", Type: "Person", Disambiguation: "Canadian rapper", Country: "CA"},
			{MBID: "bbbbbbbb-0000-0000-0000-000000000002", Name: "Blank Fields Artist", Type: "Person", Disambiguation: "", Country: ""},
		}, nil
	}}
	src := httpserver.NewMusicBrainzSource(stub)

	if got := src.Name(); got != "musicbrainz" {
		t.Fatalf("Name() = %q, want %q", got, "musicbrainz")
	}

	artists, err := src.SearchArtists(context.Background(), "drake", 10)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 2 {
		t.Fatalf("len(artists) = %d, want 2", len(artists))
	}

	first := artists[0]
	if first.Source != "musicbrainz" {
		t.Errorf("first.Source = %q, want %q", first.Source, "musicbrainz")
	}
	if first.ID != "9fff2f8a-21e6-47de-a2b8-7f449929d43f" {
		t.Errorf("first.ID = %q, want the MBID", first.ID)
	}
	if first.Disambiguation == nil || *first.Disambiguation != "Canadian rapper" {
		t.Errorf("first.Disambiguation = %v, want a pointer to %q", first.Disambiguation, "Canadian rapper")
	}
	if first.Country == nil || *first.Country != "CA" {
		t.Errorf("first.Country = %v, want a pointer to %q", first.Country, "CA")
	}
	if first.ImageURL != nil {
		t.Errorf("first.ImageURL = %v, want nil", first.ImageURL)
	}

	second := artists[1]
	if second.Disambiguation != nil {
		t.Errorf("second.Disambiguation = %v, want nil for a blank upstream value", second.Disambiguation)
	}
	if second.Country != nil {
		t.Errorf("second.Country = %v, want nil for a blank upstream value", second.Country)
	}
}

// TestNewMusicBrainzSource_PreservesOrder is insurance that Deezer's
// popularity sort (12-02) never leaks into the MusicBrainz adapter, which
// lives in this same file (D-06/D-07): the stub's order deliberately
// matches neither alphabetical name order nor descending score order, so
// any sort introduced on this path fails it.
func TestNewMusicBrainzSource_PreservesOrder(t *testing.T) {
	stub := stubMusicBrainzArtistSearcher{searchFunc: func(context.Context, string, int) ([]musicbrainz.Artist, error) {
		return []musicbrainz.Artist{
			{MBID: "1", Name: "Zeta", Type: "Person", Score: 90},
			{MBID: "2", Name: "Alpha", Type: "Person", Score: 100},
			{MBID: "3", Name: "Mu", Type: "Person", Score: 70},
			{MBID: "4", Name: "Beta", Type: "Person", Score: 95},
		}, nil
	}}
	src := httpserver.NewMusicBrainzSource(stub)

	artists, err := src.SearchArtists(context.Background(), "test", 10)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}

	want := []string{"Zeta", "Alpha", "Mu", "Beta"}
	if len(artists) != len(want) {
		t.Fatalf("len(artists) = %d, want %d", len(artists), len(want))
	}
	for i, w := range want {
		if artists[i].Name != w {
			t.Errorf("artists[%d].Name = %q, want %q (adapter must never sort)", i, artists[i].Name, w)
		}
	}
}

// TestSearchArtist_WireShapeKeySet pins the exact JSON key set SearchArtist
// marshals to (D-05): any field silently added to the wire struct --
// including a future fan-count leak -- fails this test until it is
// deliberately accepted into the expected set.
func TestSearchArtist_WireShapeKeySet(t *testing.T) {
	disambiguation := "Canadian rapper"
	country := "CA"
	imageURL := "https://example.test/image.png"
	artist := httpserver.SearchArtist{
		Source:         "musicbrainz",
		ID:             "9fff2f8a-21e6-47de-a2b8-7f449929d43f",
		Name:           "Drake",
		Disambiguation: &disambiguation,
		Country:        &country,
		Type:           "Person",
		ImageURL:       &imageURL,
	}

	data, err := json.Marshal(artist)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	want := map[string]bool{
		"source": true, "id": true, "name": true, "disambiguation": true,
		"country": true, "type": true, "image_url": true,
	}
	if len(raw) != len(want) {
		t.Errorf("key count = %d, want %d (raw = %+v)", len(raw), len(want), raw)
	}
	for key := range raw {
		if !want[key] {
			t.Errorf("unexpected wire key %q -- SearchArtist must carry exactly the seven documented keys (D-05)", key)
		}
	}
}

// TestSearch_BothSourcesOK proves the multi-source success shape: both
// sources.musicbrainz and sources.deezer are present, each ok, each carrying
// its own untouched artist list (D-01, D-02) -- an artist appearing under
// both source stubs is not merged or deduped.
func TestSearch_BothSourcesOK(t *testing.T) {
	mb := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return []httpserver.SearchArtist{{Source: "musicbrainz", ID: "9fff2f8a-21e6-47de-a2b8-7f449929d43f", Name: "Drake"}}, nil
	}}
	dz := stubSearchSource{name: "deezer", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return []httpserver.SearchArtist{{Source: "deezer", ID: "246791", Name: "Drake"}}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{mb, dz}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body searchResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	mbResult, ok := body.Sources["musicbrainz"]
	if !ok {
		t.Fatal("sources.musicbrainz missing")
	}
	if mbResult.Status != "ok" || len(mbResult.Artists) != 1 || mbResult.Artists[0].Source != "musicbrainz" {
		t.Fatalf("sources.musicbrainz = %+v, want one ok musicbrainz-sourced artist", mbResult)
	}

	dzResult, ok := body.Sources["deezer"]
	if !ok {
		t.Fatal("sources.deezer missing")
	}
	if dzResult.Status != "ok" || len(dzResult.Artists) != 1 || dzResult.Artists[0].Source != "deezer" {
		t.Fatalf("sources.deezer = %+v, want one ok deezer-sourced artist", dzResult)
	}

	if mbResult.Artists[0].Name != dzResult.Artists[0].Name {
		t.Fatalf("expected both sources to independently return %q, got %q and %q", "Drake", mbResult.Artists[0].Name, dzResult.Artists[0].Name)
	}
}

// TestSearch_DeezerOnlyFailure proves the mirror image of
// TestSearch_PartialFailure_OneSourceDownAnotherHealthy: Deezer erroring
// while MusicBrainz succeeds still responds 200 with musicbrainz's results
// intact and deezer degraded (D-03).
func TestSearch_DeezerOnlyFailure(t *testing.T) {
	const rawErr = "connection refused: deezer-error-marker"
	healthy := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return []httpserver.SearchArtist{{Source: "musicbrainz", ID: "9fff2f8a-21e6-47de-a2b8-7f449929d43f", Name: "Drake"}}, nil
	}}
	failing := stubSearchSource{name: "deezer", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New(rawErr)
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{healthy, failing}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	if !ok || mb.Status != "ok" || len(mb.Artists) != 1 {
		t.Fatalf("sources.musicbrainz = %+v, want one healthy result", mb)
	}

	dz, ok := body.Sources["deezer"]
	if !ok {
		t.Fatal("sources.deezer missing")
	}
	if dz.Status != "error" {
		t.Fatalf("sources.deezer.status = %q, want %q", dz.Status, "error")
	}
	if dz.Error != "source unavailable" {
		t.Fatalf("sources.deezer.error = %q, want %q", dz.Error, "source unavailable")
	}
	if len(dz.Artists) != 0 {
		t.Fatalf("sources.deezer.artists = %+v, want empty", dz.Artists)
	}

	if strings.Contains(string(data), rawErr) {
		t.Fatalf("response body leaked raw upstream error text: %s", data)
	}
}

// TestSearch_BothSourcesFailed proves that both sources failing at once is
// still a 200 with both statuses "error" -- never a 5xx (D-03).
func TestSearch_BothSourcesFailed(t *testing.T) {
	mb := stubSearchSource{name: "musicbrainz", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New("musicbrainz down")
	}}
	dz := stubSearchSource{name: "deezer", searchFunc: func(context.Context, string, int) ([]httpserver.SearchArtist, error) {
		return nil, errors.New("deezer down")
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, []httpserver.SearchSource{mb, dz}, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/search?q=drake")
	if err != nil {
		t.Fatalf("GET /search: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d (both sources failing must never produce a 5xx)", resp.StatusCode, http.StatusOK)
	}

	var body searchResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	mbResult, ok := body.Sources["musicbrainz"]
	if !ok || mbResult.Status != "error" {
		t.Fatalf("sources.musicbrainz = %+v, want status error", mbResult)
	}
	dzResult, ok := body.Sources["deezer"]
	if !ok || dzResult.Status != "error" {
		t.Fatalf("sources.deezer = %+v, want status error", dzResult)
	}
}
