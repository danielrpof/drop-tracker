package deezer

// This file is package deezer (whitebox), mirroring albums_test.go/search_test.go.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const drakeArtistByIDFixture = `{
  "id": 246791,
  "name": "Drake",
  "link": "https://www.deezer.com/artist/246791",
  "picture": "https://api.deezer.com/artist/246791/image",
  "nb_fan": 24047501,
  "type": "artist"
}`

const deadArtistErrorFixture = `{"error":{"type":"DataException","message":"no data","code":800}}`

func TestArtistByID_DecodesFixture(t *testing.T) {
	var gotPath string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeArtistByIDFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artist, err := c.ArtistByID(context.Background(), "6144")
	if err != nil {
		t.Fatalf("ArtistByID: %v", err)
	}
	if artist.ID != 246791 {
		t.Errorf("ID = %d, want %d", artist.ID, 246791)
	}
	if artist.Name != "Drake" {
		t.Errorf("Name = %q, want %q", artist.Name, "Drake")
	}
	if artist.Link != "https://www.deezer.com/artist/246791" {
		t.Errorf("Link = %q, want %q", artist.Link, "https://www.deezer.com/artist/246791")
	}
	if artist.Picture != "https://api.deezer.com/artist/246791/image" {
		t.Errorf("Picture = %q, want %q", artist.Picture, "https://api.deezer.com/artist/246791/image")
	}
	if artist.NbFan != 24047501 {
		t.Errorf("NbFan = %d, want %d", artist.NbFan, 24047501)
	}
	if gotPath != "/artist/6144" {
		t.Errorf("path = %q, want %q", gotPath, "/artist/6144")
	}
}

func TestArtistByID_RequestShapeNoQueryParams(t *testing.T) {
	var gotPath, gotRawQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeArtistByIDFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	// A slash in the artist id exercises url.PathEscape.
	idWithSlash := "abc/def"
	if _, err := c.ArtistByID(context.Background(), idWithSlash); err != nil {
		t.Fatalf("ArtistByID: %v", err)
	}
	wantPath := "/artist/abc%2Fdef"
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotRawQuery != "" {
		t.Errorf("RawQuery = %q, want empty (no query parameters)", gotRawQuery)
	}
}

func TestArtistByID_EmptyOrWhitespaceIDReturnsErrorWithZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(drakeArtistByIDFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	for _, id := range []string{"", "   "} {
		artist, err := c.ArtistByID(context.Background(), id)
		if !errors.Is(err, ErrEmptyArtistID) {
			t.Fatalf("id %q: err = %v, want ErrEmptyArtistID", id, err)
		}
		if artist.ID != 0 || artist.Name != "" {
			t.Fatalf("id %q: artist = %+v, want the zero Artist", id, artist)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0", got)
	}
}

// TestArtistByID_DeadArtistInBodyErrorSurfacesAsAPIError proves the
// dead/renumbered-id signal Tier 0 needs: Deezer answers with HTTP 200 and
// an in-body error envelope, which decodeChecked must convert into a real
// *APIError rather than a zero-valued success.
func TestArtistByID_DeadArtistInBodyErrorSurfacesAsAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(deadArtistErrorFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artist, err := c.ArtistByID(context.Background(), "999999999")
	if err == nil {
		t.Fatal("ArtistByID: got nil error, want a non-nil *APIError")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ArtistByID error = %v (%T), want it to be/wrap *APIError", err, err)
	}
	if apiErr.Code != 800 {
		t.Errorf("APIError.Code = %d, want %d", apiErr.Code, 800)
	}
	if artist.ID != 0 || artist.Name != "" {
		t.Fatalf("artist = %+v, want the zero Artist (never a zero-valued success)", artist)
	}
}

func TestArtistByID_NonOKStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{name: "404", status: http.StatusNotFound},
		{name: "503", status: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer ts.Close()

			c := newTestClient(t, ts, unlimitedLimiter())
			artist, err := c.ArtistByID(context.Background(), "6144")
			if err == nil {
				t.Fatal("ArtistByID: got nil error, want a non-nil error naming the status")
			}
			wantStatus := "404"
			if tt.status == http.StatusServiceUnavailable {
				wantStatus = "503"
			}
			if !strings.Contains(err.Error(), wantStatus) {
				t.Errorf("error = %q, want it to name status %s", err.Error(), wantStatus)
			}
			if artist.ID != 0 || artist.Name != "" {
				t.Fatalf("artist = %+v, want the zero Artist", artist)
			}
		})
	}
}

func TestArtistByID_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not json"))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, unlimitedLimiter())
	artist, err := c.ArtistByID(context.Background(), "6144")
	if err == nil {
		t.Fatal("ArtistByID: got nil error, want a decode error")
	}
	if artist.ID != 0 || artist.Name != "" {
		t.Fatalf("artist = %+v, want the zero Artist", artist)
	}
}
