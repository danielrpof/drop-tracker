package musicbrainz

// This file is package musicbrainz (whitebox), not musicbrainz_test, so it
// can point a Client at an httptest.Server via the unexported baseURL
// field -- NewClient deliberately has no exported base-URL setter
// (T-03-07). Fixtures below are transcribed verbatim from the
// live-verified MusicBrainz responses in 03-RESEARCH.md.

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

const driveFixture = `{
  "created": "2026-08-07T20:30:23.945Z",
  "count": 303,
  "offset": 0,
  "artists": [
    {
      "id": "9fff2f8a-21e6-47de-a2b8-7f449929d43f",
      "type": "Person",
      "score": 100,
      "name": "Drake",
      "sort-name": "Drake",
      "country": "CA",
      "disambiguation": "Canadian rapper"
    }
  ]
}`

const emptyFixture = `{"created":"2026-08-07T20:30:23.945Z","count":0,"offset":0,"artists":[]}`

const twoSameScoreFixture = `{
  "created": "2026-08-07T20:30:23.945Z",
  "count": 2,
  "offset": 0,
  "artists": [
    {"id": "aaaaaaaa-0000-0000-0000-000000000001", "type": "Person", "score": 100, "name": "Artist A", "sort-name": "Artist A", "disambiguation": ""},
    {"id": "bbbbbbbb-0000-0000-0000-000000000002", "type": "Person", "score": 100, "name": "Artist B", "sort-name": "Artist B", "disambiguation": ""}
  ]
}`

// unlimitedLimiter gives tests that aren't exercising rate-limiting itself
// a limiter that never blocks.
func unlimitedLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}

// newTestClient builds a Client wired to ts instead of musicbrainz.org.
func newTestClient(t *testing.T, ts *httptest.Server, userAgent string, limiter *rate.Limiter) *Client {
	t.Helper()
	c := NewClient(userAgent, limiter, ts.Client())
	c.baseURL = ts.URL
	return c
}

func TestSearchArtists_DecodesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(driveFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "drake", 25)
	if err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if len(artists) != 1 {
		t.Fatalf("len(artists) = %d, want 1", len(artists))
	}
	got := artists[0]
	if got.MBID != "9fff2f8a-21e6-47de-a2b8-7f449929d43f" {
		t.Errorf("MBID = %q, want %q", got.MBID, "9fff2f8a-21e6-47de-a2b8-7f449929d43f")
	}
	if got.Name != "Drake" {
		t.Errorf("Name = %q, want %q", got.Name, "Drake")
	}
	if got.Disambiguation != "Canadian rapper" {
		t.Errorf("Disambiguation = %q, want %q", got.Disambiguation, "Canadian rapper")
	}
	if got.Type != "Person" {
		t.Errorf("Type = %q, want %q", got.Type, "Person")
	}
}

func TestSearchArtists_SetsUserAgentAndAccept(t *testing.T) {
	const wantUA = "drop-tracker-test/1.0 (+contact@example.test)"
	var gotUA, gotAccept string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(driveFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, wantUA, unlimitedLimiter())
	if _, err := c.SearchArtists(context.Background(), "drake", 25); err != nil {
		t.Fatalf("SearchArtists: %v", err)
	}
	if gotUA != wantUA {
		t.Errorf("User-Agent = %q, want %q", gotUA, wantUA)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

func TestSearchArtists_QueryParams(t *testing.T) {
	tests := []struct {
		name      string
		limit     int
		wantLimit string
	}{
		{name: "within range", limit: 10, wantLimit: "10"},
		{name: "above max clamps to 100", limit: 500, wantLimit: "100"},
		{name: "zero clamps to default", limit: 0, wantLimit: "25"},
		{name: "negative clamps to default", limit: -5, wantLimit: "25"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery, gotFmt, gotLimit string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query().Get("query")
				gotFmt = r.URL.Query().Get("fmt")
				gotLimit = r.URL.Query().Get("limit")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(driveFixture))
			}))
			defer ts.Close()

			c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
			if _, err := c.SearchArtists(context.Background(), "drake", tt.limit); err != nil {
				t.Fatalf("SearchArtists: %v", err)
			}
			if gotQuery != "artist:drake" {
				t.Errorf("query = %q, want %q", gotQuery, "artist:drake")
			}
			if gotFmt != "json" {
				t.Errorf("fmt = %q, want %q", gotFmt, "json")
			}
			if gotLimit != tt.wantLimit {
				t.Errorf("limit = %q, want %q", gotLimit, tt.wantLimit)
			}
		})
	}
}

func TestSearchArtists_EmptyResultIsNonNilZeroLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "zzzzzznomatch", 25)
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
		_, _ = w.Write([]byte(twoSameScoreFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
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

func TestSearchArtists_UpstreamErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	artists, err := c.SearchArtists(context.Background(), "drake", 25)
	if err == nil {
		t.Fatal("SearchArtists: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name the status code 503", err.Error())
	}
	if artists != nil {
		t.Fatalf("artists = %v, want nil", artists)
	}
}

func TestSearchArtists_RateLimiterPacesRequests(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(driveFixture))
	}))
	defer ts.Close()

	// rate.Limit(20) with burst 1: the first call is free, the second and
	// third each wait ~50ms for a fresh token -- ~100ms total, kept fast
	// enough that the test suite stays quick.
	limiter := rate.NewLimiter(rate.Limit(20), 1)
	c := newTestClient(t, ts, "drop-tracker-test/1.0", limiter)

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

func TestSearchArtists_CancelledContextMakesZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(driveFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	artists, err := c.SearchArtists(ctx, "drake", 25)
	if err == nil {
		t.Fatal("SearchArtists: got nil error, want a wrapped context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SearchArtists error = %v, want it to wrap context.Canceled", err)
	}
	if artists != nil {
		t.Fatalf("artists = %v, want nil", artists)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (a cancelled context must not reach the server)", got)
	}
}
