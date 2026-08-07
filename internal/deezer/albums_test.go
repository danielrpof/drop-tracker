package deezer

// This file is package deezer (whitebox), mirroring search_test.go.
// Fixtures below are transcribed verbatim from the live-verified Deezer
// responses in 03-RESEARCH.md.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

const habibtiAlbumsFixture = `{
  "data": [
    {
      "id": 983217461,
      "title": "HABIBTI",
      "link": "https://www.deezer.com/album/983217461",
      "cover": "https://api.deezer.com/album/983217461/image",
      "release_date": "2026-05-15",
      "record_type": "album",
      "tracklist": "https://api.deezer.com/album/983217461/tracks",
      "explicit_lyrics": true,
      "type": "album"
    }
  ],
  "total": 78,
  "next": "https://api.deezer.com/artist/246791/albums?limit=3&index=3"
}`

const emptyAlbumsFixture = `{"data":[],"total":0}`

const twoSameTitleAlbumsFixture = `{
  "data": [
    {"id": 100, "title": "HABIBTI", "link": "https://www.deezer.com/album/100", "cover": "https://api.deezer.com/album/100/image", "release_date": "2026-05-15", "record_type": "album", "explicit_lyrics": false, "type": "album"},
    {"id": 200, "title": "HABIBTI", "link": "https://www.deezer.com/album/200", "cover": "https://api.deezer.com/album/200/image", "release_date": "2026-05-15", "record_type": "album", "explicit_lyrics": false, "type": "album"}
  ],
  "total": 2
}`

const partialDateAlbumsFixture = `{
  "data": [
    {"id": 1, "title": "Partial Year Only", "release_date": "2026", "record_type": "album", "type": "album"},
    {"id": 2, "title": "Partial Year Month", "release_date": "2026-05", "record_type": "album", "type": "album"}
  ],
  "total": 2
}`

func TestArtistAlbums_DecodesFixture(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("index") == "0" {
			_, _ = w.Write([]byte(habibtiAlbumsFixture))
			return
		}
		// habibtiAlbumsFixture's live-verified "total":78 would otherwise
		// drive ArtistAlbums's pagination loop (WR-02) into fetching
		// further pages; terminate here so this test exercises only the
		// first page's field decoding -- pagination itself is covered by
		// TestArtistAlbums_PaginationCollectsAllPagesInOrder below.
		_, _ = w.Write([]byte(`{"data":[],"total":78}`))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 3)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != 1 {
		t.Fatalf("len(albums) = %d, want 1", len(albums))
	}
	got := albums[0]
	if got.ID != 983217461 {
		t.Errorf("ID = %d, want %d", got.ID, 983217461)
	}
	if got.Title != "HABIBTI" {
		t.Errorf("Title = %q, want %q", got.Title, "HABIBTI")
	}
	if got.ReleaseDate != "2026-05-15" {
		t.Errorf("ReleaseDate = %q, want %q", got.ReleaseDate, "2026-05-15")
	}
	if got.RecordType != "album" {
		t.Errorf("RecordType = %q, want %q", got.RecordType, "album")
	}
	if got.Cover != "https://api.deezer.com/album/983217461/image" {
		t.Errorf("Cover = %q, want %q", got.Cover, "https://api.deezer.com/album/983217461/image")
	}
	if got.ExplicitLyrics != true {
		t.Errorf("ExplicitLyrics = %v, want true", got.ExplicitLyrics)
	}

	if gotPath != "/artist/246791/albums" {
		t.Errorf("path = %q, want %q", gotPath, "/artist/246791/albums")
	}
}

func TestArtistAlbums_EmptyArtistIDReturnsErrorWithZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(habibtiAlbumsFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "", 10)
	if !errors.Is(err, ErrEmptyArtistID) {
		t.Fatalf("ArtistAlbums error = %v, want ErrEmptyArtistID", err)
	}
	if albums != nil {
		t.Fatalf("albums = %v, want nil", albums)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 -- no doubled-slash request should ever be built", got)
	}
}

func TestArtistAlbums_WhitespaceOnlyArtistIDReturnsErrorWithZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(habibtiAlbumsFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "   ", 10)
	if !errors.Is(err, ErrEmptyArtistID) {
		t.Fatalf("ArtistAlbums error = %v, want ErrEmptyArtistID", err)
	}
	if albums != nil {
		t.Fatalf("albums = %v, want nil", albums)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0", got)
	}
}

// TestArtistAlbums_NonexistentArtistReturnsEmptyNonNilNoError proves the
// D-06 companion behavior: a stale/nonexistent Deezer artist id degrades
// gracefully (empty result, nil error) rather than failing the poll cycle.
func TestArtistAlbums_NonexistentArtistReturnsEmptyNonNilNoError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyAlbumsFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "999999999999999", 10)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v, want nil error for a nonexistent artist", err)
	}
	if albums == nil {
		t.Fatal("albums is nil, want a non-nil zero-length slice")
	}
	if len(albums) != 0 {
		t.Fatalf("len(albums) = %d, want 0", len(albums))
	}
}

func TestArtistAlbums_TwoAlbumsSameTitleAndDateStayDistinct(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(twoSameTitleAlbumsFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	for i := 0; i < 3; i++ {
		albums, err := c.ArtistAlbums(context.Background(), "246791", 10)
		if err != nil {
			t.Fatalf("call %d: ArtistAlbums: %v", i, err)
		}
		if len(albums) != 2 {
			t.Fatalf("call %d: len(albums) = %d, want 2", i, len(albums))
		}
		if albums[0].ID != 100 || albums[1].ID != 200 {
			t.Fatalf("call %d: ids = [%d, %d], want [100, 200] -- distinct entries, upstream order", i, albums[0].ID, albums[1].ID)
		}
		if albums[0].Title != albums[1].Title {
			t.Fatalf("call %d: titles differ unexpectedly: %q vs %q", i, albums[0].Title, albums[1].Title)
		}
	}
}

// TestArtistAlbums_QuotaErrorInBodyWithHTTP200 mirrors search_test.go's
// equivalent case for the albums endpoint.
func TestArtistAlbums_QuotaErrorInBodyWithHTTP200(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(quotaErrorFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 10)
	if err == nil {
		t.Fatal("ArtistAlbums: got nil error, want a non-nil *APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ArtistAlbums error = %v (%T), want it to be/wrap *APIError", err, err)
	}
	if albums != nil {
		t.Fatalf("albums = %v, want nil", albums)
	}
}

func TestArtistAlbums_PreservesPartialReleaseDatesVerbatim(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(partialDateAlbumsFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 10)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != 2 {
		t.Fatalf("len(albums) = %d, want 2", len(albums))
	}
	if albums[0].ReleaseDate != "2026" {
		t.Errorf("albums[0].ReleaseDate = %q, want %q (raw, unpadded)", albums[0].ReleaseDate, "2026")
	}
	if albums[1].ReleaseDate != "2026-05" {
		t.Errorf("albums[1].ReleaseDate = %q, want %q (raw, unpadded)", albums[1].ReleaseDate, "2026-05")
	}
}

// --- WR-02 fix: bounded pagination and proven pacing across pages ---
//
// Mirrors internal/musicbrainz/releasegroups_test.go's "Task 2" pagination
// suite. habibtiAlbumsFixture's live-verified "total":78 (03-RESEARCH.md)
// is the concrete evidence 03-REVIEW.md WR-02 cited for why a single,
// unpaginated request silently truncates any artist with more than one
// page of Deezer albums.

// pagedArtistAlbumsServer serves albums across pages based on the "index"
// query param, recording each request's index for assertions.
func pagedArtistAlbumsServer(t *testing.T, total int, pageSize int) (*httptest.Server, *[]int) {
	t.Helper()
	var mu sync.Mutex
	var indexes []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		index, _ := strconv.Atoi(r.URL.Query().Get("index"))
		mu.Lock()
		indexes = append(indexes, index)
		mu.Unlock()

		remaining := total - index
		if remaining < 0 {
			remaining = 0
		}
		n := pageSize
		if remaining < n {
			n = remaining
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildArtistAlbumsPageJSON(total, index, n)))
	}))
	return ts, &indexes
}

func buildArtistAlbumsPageJSON(total, index, n int) string {
	var sb strings.Builder
	sb.WriteString(`{"data":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		idx := index + i
		sb.WriteString(`{"id":`)
		sb.WriteString(strconv.Itoa(idx + 1))
		sb.WriteString(`,"title":"Generated `)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","record_type":"album","type":"album"}`)
	}
	sb.WriteString(`],"total":`)
	sb.WriteString(strconv.Itoa(total))
	sb.WriteString(`}`)
	return sb.String()
}

// TestArtistAlbums_PaginationCollectsAllPagesInOrder proves WR-02's fix
// against the live-verified "total":78 evidence cited in 03-REVIEW.md: a
// watched artist with more than one page's worth of Deezer albums must have
// every album fetched, not just the first fixed-size window.
func TestArtistAlbums_PaginationCollectsAllPagesInOrder(t *testing.T) {
	ts, indexes := pagedArtistAlbumsServer(t, 78, 10)
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 10)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != 78 {
		t.Fatalf("len(albums) = %d, want 78", len(albums))
	}
	for i, a := range albums {
		want := int64(i + 1)
		if a.ID != want {
			t.Fatalf("albums[%d].ID = %d, want %d (page order broken)", i, a.ID, want)
		}
	}

	want := []int{0, 10, 20, 30, 40, 50, 60, 70}
	if len(*indexes) != len(want) {
		t.Fatalf("indexes = %v, want %v", *indexes, want)
	}
	for i, idx := range want {
		if (*indexes)[i] != idx {
			t.Errorf("indexes[%d] = %d, want %d", i, (*indexes)[i], idx)
		}
	}
}

func TestArtistAlbums_PacingAppliesAcrossPages(t *testing.T) {
	ts, _ := pagedArtistAlbumsServer(t, 250, 100)
	defer ts.Close()

	// rate.Limit(20) with burst 1: the first page is free, the second and
	// third each wait ~50ms for a fresh token -- pagination cannot outrun
	// the configured rate.
	limiter := rate.NewLimiter(rate.Limit(20), 1)
	c := newTestClient(t, ts, limiter)

	start := time.Now()
	if _, err := c.ArtistAlbums(context.Background(), "246791", 100); err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least 100ms (every page must be separately paced)", elapsed)
	}
}

func TestArtistAlbums_StopsAsSoonAsTotalReached(t *testing.T) {
	ts, indexes := pagedArtistAlbumsServer(t, 61, 100)
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 100)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != 61 {
		t.Fatalf("len(albums) = %d, want 61", len(albums))
	}
	if len(*indexes) != 1 {
		t.Fatalf("requestCount = %d, want exactly 1", len(*indexes))
	}
}

func TestArtistAlbums_PageCapStopsRunawayTotal(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		index, _ := strconv.Atoi(r.URL.Query().Get("index"))
		w.Header().Set("Content-Type", "application/json")
		// Always claims a huge total while always returning a full page --
		// a hostile/runaway upstream that would otherwise drive requests
		// forever (mirrors T-03-12).
		_, _ = w.Write([]byte(buildArtistAlbumsPageJSON(1000000, index, 100)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 100)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != maxAlbumPages*100 {
		t.Fatalf("len(albums) = %d, want %d", len(albums), maxAlbumPages*100)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != maxAlbumPages {
		t.Fatalf("requestCount = %d, want exactly maxAlbumPages (%d)", got, maxAlbumPages)
	}
}

func TestArtistAlbums_ZeroEntryPageTerminatesLoop(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// Reports a huge total but always returns zero entries -- the loop
		// must terminate on the empty page rather than spin forever.
		_, _ = w.Write([]byte(`{"data":[],"total":1000000}`))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 100)
	if err != nil {
		t.Fatalf("ArtistAlbums: %v", err)
	}
	if len(albums) != 0 {
		t.Fatalf("len(albums) = %d, want 0", len(albums))
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requestCount = %d, want exactly 1", got)
	}
}

func TestArtistAlbums_MidFetchErrorStopsWithNoRetry(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		n := requestCount
		mu.Unlock()

		if n == 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		index, _ := strconv.Atoi(r.URL.Query().Get("index"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildArtistAlbumsPageJSON(250, index, 100)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(context.Background(), "246791", 100)
	if err == nil {
		t.Fatal("ArtistAlbums: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if albums != nil {
		t.Fatalf("albums = %v, want nil", albums)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 2 {
		t.Fatalf("requestCount = %d, want exactly 2 (no retry, no continuation past the failure)", got)
	}
}

func TestArtistAlbums_CancellationBetweenPagesAborts(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ctx, cancel := context.WithCancel(context.Background())

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		n := requestCount
		mu.Unlock()

		if n == 1 {
			// Cancel after the first page is served so the loop's next
			// ctx check aborts before issuing a second request.
			cancel()
		}
		index, _ := strconv.Atoi(r.URL.Query().Get("index"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildArtistAlbumsPageJSON(250, index, 100)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	albums, err := c.ArtistAlbums(ctx, "246791", 100)
	if err == nil {
		t.Fatal("ArtistAlbums: got nil error, want ctx.Err()")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ArtistAlbums error = %v, want it to wrap context.Canceled", err)
	}
	if albums != nil {
		t.Fatalf("albums = %v, want nil", albums)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got >= 3 {
		t.Fatalf("requestCount = %d, want fewer than the full page count (3)", got)
	}
}
