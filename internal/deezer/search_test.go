package deezer

// This file is package deezer (whitebox), not deezer_test, so it can point
// a Client at an httptest.Server via the unexported baseURL field -- NewClient
// deliberately has no exported base-URL setter, mirroring
// internal/musicbrainz's client_test.go. Fixtures below are transcribed
// verbatim from the live-verified Deezer responses in 03-RESEARCH.md; the
// quota-error fixture matches the community-sourced shape documented there
// (assumption A1).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

const drakeSearchFixture = `{
  "data": [
    {
      "id": 246791,
      "name": "Drake",
      "link": "https://www.deezer.com/artist/246791",
      "picture": "https://api.deezer.com/artist/246791/image",
      "nb_album": 78,
      "nb_fan": 24047501,
      "tracklist": "https://api.deezer.com/artist/246791/top?limit=50",
      "type": "artist"
    }
  ],
  "total": 100,
  "next": "https://api.deezer.com/search/artist?q=Drake&limit=2&index=2"
}`

const emptySearchFixture = `{"data":[],"total":0}`

const largeIDSearchFixture = `{
  "data": [
    {
      "id": 9007199254740993,
      "name": "Big Id Artist",
      "link": "https://www.deezer.com/artist/9007199254740993",
      "picture": "https://api.deezer.com/artist/9007199254740993/image",
      "nb_album": 1,
      "type": "artist"
    }
  ],
  "total": 1
}`

const twoArtistsSearchFixture = `{
  "data": [
    {"id": 1, "name": "Artist A", "link": "https://www.deezer.com/artist/1", "picture": "https://api.deezer.com/artist/1/image", "nb_album": 5, "type": "artist"},
    {"id": 2, "name": "Artist B", "link": "https://www.deezer.com/artist/2", "picture": "https://api.deezer.com/artist/2/image", "nb_album": 3, "type": "artist"}
  ],
  "total": 2
}`

const quotaErrorFixture = `{"error":{"type":"Exception","message":"Quota limit exceeded","code":4}}`

// unlimitedLimiter gives tests that aren't exercising rate-limiting itself a
// limiter that never blocks.
func unlimitedLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}

// newTestClient builds a Client wired to ts instead of api.deezer.com.
func newTestClient(t *testing.T, ts *httptest.Server, limiter *rate.Limiter) *Client {
	t.Helper()
	c := NewClient(limiter, ts.Client())
	c.baseURL = ts.URL
	return c
}

func TestSearchArtists_DecodesFixture(t *testing.T) {
	var gotPath string
	var gotQ, gotLimit string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQ = r.URL.Query().Get("q")
		gotLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeSearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "drake", 25)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	got := artists[0]
	if got.ID != 246791 {
		t.Errorf("ID = %d, want %d", got.ID, 246791)
	}
	if got.Name != "Drake" {
		t.Errorf("Name = %q, want %q", got.Name, "Drake")
	}
	if got.Link != "https://www.deezer.com/artist/246791" {
		t.Errorf("Link = %q, want %q", got.Link, "https://www.deezer.com/artist/246791")
	}
	if got.Picture != "https://api.deezer.com/artist/246791/image" {
		t.Errorf("Picture = %q, want %q", got.Picture, "https://api.deezer.com/artist/246791/image")
	}
	if got.NbAlbum != 78 {
		t.Errorf("NbAlbum = %d, want %d", got.NbAlbum, 78)
	}

	if gotPath != "/search/artist" {
		t.Errorf("path = %q, want %q", gotPath, "/search/artist")
	}
	if gotQ != "drake" {
		t.Errorf("q = %q, want %q", gotQ, "drake")
	}
	if gotLimit != "25" {
		t.Errorf("limit = %q, want %q", gotLimit, "25")
	}
}

func TestSearchArtists_LargeIDDecodesExactly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(largeIDSearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "big id", 25)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	const wantID int64 = 9007199254740993
	if artists[0].ID != wantID {
		t.Fatalf("ID = %d, want %d (exact round-trip, no float64 precision loss)", artists[0].ID, wantID)
	}
}

func TestSearchArtists_EmptyResultIsNonNilZeroLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptySearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "zzzznomatch", 25)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if artists == nil {
		t.Fatal("artists is nil, want a non-nil zero-length slice")
	}
	if len(artists) != 0 {
		t.Fatalf("len(artists) = %d, want 0", len(artists))
	}
}

func TestSearchArtists_PreservesUpstreamOrderNoSorting(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(twoArtistsSearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	for i := 0; i < 3; i++ {
		artists, err := c.SearchArtists(context.Background(), "artist", 25)
		if err != nil {
			t.Fatalf("call %d: SearchArtists: %v", i, err)
		}
		if len(artists) != 2 {
			t.Fatalf("call %d: len(artists) = %d, want 2", i, len(artists))
		}
		if artists[0].Name != "Artist A" || artists[1].Name != "Artist B" {
			t.Fatalf("call %d: order = [%q, %q], want [Artist A, Artist B]", i, artists[0].Name, artists[1].Name)
		}
	}
}

// TestSearchArtists_QuotaErrorInBodyWithHTTP200 proves T-03-05: an
// HTTP-200 in-body quota error must never decode into a zero-valued
// success -- it must surface as a real *APIError.
func TestSearchArtists_QuotaErrorInBodyWithHTTP200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(quotaErrorFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "drake", 25)
	if err == nil {
		t.Fatal("SearchArtists: got nil error, want a non-nil *APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SearchArtists error = %v (%T), want it to be/wrap *APIError", err, err)
	}
	if apiErr.Code != 4 {
		t.Errorf("APIError.Code = %d, want %d", apiErr.Code, 4)
	}
	if artists != nil {
		t.Fatalf("artists = %v, want nil (never a zero-valued success)", artists)
	}
}

func TestSearchArtists_UpstreamErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "drake", 25)
	if err == nil {
		t.Fatal("SearchArtists: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %q, want it to name the status code 500", err.Error())
	}
	if artists != nil {
		t.Fatalf("artists = %v, want nil", artists)
	}
}

func TestSearchArtists_SetsAcceptHeader(t *testing.T) {
	var gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeSearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	if _, err := c.SearchArtists(context.Background(), "drake", 25); err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

func TestSearchArtists_RateLimiterPacesRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeSearchFixture))
	}))
	defer ts.Close()

	// rate.Limit(20) with burst 1: the first call is free, the second and
	// third each wait ~50ms for a fresh token -- ~100ms total.
	limiter := rate.NewLimiter(rate.Limit(20), 1)
	c := newTestClient(t, ts, limiter)

	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.SearchArtists(context.Background(), "drake", 25); err != nil {
			t.Fatalf("call %d: SearchArtists: %v", i, err)
		}
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least 100ms (rate limiter should pace requests)", elapsed)
	}
}

func TestSearchArtists_EmptyOrWhitespaceQueryMakesZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeSearchFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())

	for _, q := range []string{"", "   "} {
		artists, err := c.SearchArtists(context.Background(), q, 25)
		if !errors.Is(err, ErrEmptyQuery) {
			t.Fatalf("q=%q: SearchArtists error = %v, want ErrEmptyQuery", q, err)
		}
		if artists != nil {
			t.Fatalf("q=%q: artists = %v, want nil", q, artists)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0", got)
	}
}
