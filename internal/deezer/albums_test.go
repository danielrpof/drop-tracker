package deezer

// This file is package deezer (whitebox), mirroring search_test.go.
// Fixtures below are transcribed verbatim from the live-verified Deezer
// responses in 03-RESEARCH.md.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
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
		_, _ = w.Write([]byte(habibtiAlbumsFixture))
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
