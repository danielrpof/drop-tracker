package musicbrainz

// This file is package musicbrainz (whitebox), matching
// recording_lookup_test.go's convention, so it can reuse
// newTestClient/unlimitedLimiter and point a Client at an httptest.Server via
// the unexported baseURL field. The fixture below is NOT [ASSUMED] -- both
// the relation and alias shapes were confirmed against a live
// musicbrainz.org/ws/2 response during planning (see the plan's
// verified-api-shapes block), unlike recording_lookup_test.go's fixture.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const sampleArtistDetailMBID = "a74b1b7f-71a5-4011-9441-d0b5e4122711"

const artistDetailFixture = `{
  "id": "a74b1b7f-71a5-4011-9441-d0b5e4122711",
  "name": "Radiohead",
  "relations": [
    {
      "type": "free streaming",
      "type-id": "769085a1-c2f7-4c24-a532-2375a77693bd",
      "direction": "forward",
      "target-type": "url",
      "url": { "id": "b7d62bce-0000-0000-0000-000000000001", "resource": "https://www.deezer.com/artist/6144" },
      "begin": null, "end": null, "ended": false
    },
    {
      "type": "streaming",
      "type-id": "63cc5d1f-f096-4c94-a43f-ecb32ea94161",
      "direction": "forward",
      "target-type": "url",
      "url": { "id": "b7d62bce-0000-0000-0000-000000000002", "resource": "https://music.apple.com/gb/artist/657515" },
      "begin": null, "end": null, "ended": false
    }
  ],
  "aliases": [
    { "name": "Radio Head", "sort-name": "Radio Head", "type": "Search hint", "type-id": "1937e404-0000-0000-0000-000000000001", "primary": null, "locale": null, "begin": null, "end": null, "ended": false },
    { "name": "RH", "sort-name": "RH", "type": "Legal name", "type-id": "1937e404-0000-0000-0000-000000000002", "primary": true, "locale": "en", "begin": null, "end": null, "ended": false },
    { "name": "Radiohead Band", "sort-name": "Radiohead Band", "type": "Artist name", "type-id": "1937e404-0000-0000-0000-000000000003", "primary": null, "locale": null, "begin": null, "end": null, "ended": false }
  ]
}`

const noRelationsNoAliasesFixture = `{"id":"a74b1b7f-71a5-4011-9441-d0b5e4122711","name":"Radiohead"}`

func TestLookupArtist_DecodesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistDetailFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	detail, err := c.LookupArtist(context.Background(), sampleArtistDetailMBID)
	if err != nil {
		t.Fatalf("LookupArtist: %v", err)
	}
	if detail.MBID != sampleArtistDetailMBID {
		t.Errorf("MBID = %q, want %q", detail.MBID, sampleArtistDetailMBID)
	}
	if detail.Name != "Radiohead" {
		t.Errorf("Name = %q, want %q", detail.Name, "Radiohead")
	}

	if len(detail.Relations) != 2 {
		t.Fatalf("len(Relations) = %d, want 2", len(detail.Relations))
	}
	if detail.Relations[0].Type != "free streaming" {
		t.Errorf("Relations[0].Type = %q, want %q", detail.Relations[0].Type, "free streaming")
	}
	if detail.Relations[0].URL.Resource != "https://www.deezer.com/artist/6144" {
		t.Errorf("Relations[0].URL.Resource = %q, want %q", detail.Relations[0].URL.Resource, "https://www.deezer.com/artist/6144")
	}
	if detail.Relations[1].Type != "streaming" {
		t.Errorf("Relations[1].Type = %q, want %q", detail.Relations[1].Type, "streaming")
	}
	if detail.Relations[1].URL.Resource != "https://music.apple.com/gb/artist/657515" {
		t.Errorf("Relations[1].URL.Resource = %q, want %q", detail.Relations[1].URL.Resource, "https://music.apple.com/gb/artist/657515")
	}

	if len(detail.Aliases) != 3 {
		t.Fatalf("len(Aliases) = %d, want 3", len(detail.Aliases))
	}
	if detail.Aliases[0].Name != "Radio Head" {
		t.Errorf("Aliases[0].Name = %q, want %q", detail.Aliases[0].Name, "Radio Head")
	}
	if detail.Aliases[0].Type != "Search hint" {
		t.Errorf("Aliases[0].Type = %q, want %q", detail.Aliases[0].Type, "Search hint")
	}
	if detail.Aliases[0].Primary != false {
		t.Errorf("Aliases[0].Primary = %v, want false (JSON null decodes to zero value)", detail.Aliases[0].Primary)
	}
	if detail.Aliases[0].Locale != "" {
		t.Errorf("Aliases[0].Locale = %q, want empty string (JSON null decodes to zero value)", detail.Aliases[0].Locale)
	}
	if detail.Aliases[1].Name != "RH" {
		t.Errorf("Aliases[1].Name = %q, want %q", detail.Aliases[1].Name, "RH")
	}
	if detail.Aliases[1].Type != "Legal name" {
		t.Errorf("Aliases[1].Type = %q, want %q", detail.Aliases[1].Type, "Legal name")
	}
	if detail.Aliases[1].Primary != true {
		t.Errorf("Aliases[1].Primary = %v, want true", detail.Aliases[1].Primary)
	}
	if detail.Aliases[1].Locale != "en" {
		t.Errorf("Aliases[1].Locale = %q, want %q", detail.Aliases[1].Locale, "en")
	}
}

func TestLookupArtist_RequestShape(t *testing.T) {
	var gotPath, gotInc, gotFmt, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotInc = r.URL.Query().Get("inc")
		gotFmt = r.URL.Query().Get("fmt")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistDetailFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	// A slash and a space in the mbid exercise url.PathEscape (T-13-01) --
	// raw concatenation would corrupt the request path or let the caller
	// influence the path structure.
	mbidWithSpecialChars := "abc/def 123"
	if _, err := c.LookupArtist(context.Background(), mbidWithSpecialChars); err != nil {
		t.Fatalf("LookupArtist: %v", err)
	}
	wantPath := "/artist/" + url.PathEscape(mbidWithSpecialChars)
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotInc != "url-rels+aliases" {
		t.Errorf("inc = %q, want %q", gotInc, "url-rels+aliases")
	}
	if gotFmt != "json" {
		t.Errorf("fmt = %q, want %q", gotFmt, "json")
	}
	if gotUA == "" {
		t.Error("User-Agent header missing")
	}
}

func TestLookupArtist_EmptyMBID(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(artistDetailFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	for _, mbid := range []string{"", "   "} {
		detail, err := c.LookupArtist(context.Background(), mbid)
		if !errors.Is(err, ErrEmptyMBID) {
			t.Fatalf("mbid %q: err = %v, want ErrEmptyMBID", mbid, err)
		}
		if detail.MBID != "" || detail.Name != "" || detail.Relations != nil || detail.Aliases != nil {
			t.Fatalf("mbid %q: detail = %+v, want the zero ArtistDetail", mbid, detail)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (empty mbid must not issue a request)", got)
	}
}

func TestLookupArtist_NonOKStatus(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	detail, err := c.LookupArtist(context.Background(), sampleArtistDetailMBID)
	if err == nil {
		t.Fatal("LookupArtist: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if detail.MBID != "" || detail.Relations != nil || detail.Aliases != nil {
		t.Fatalf("detail = %+v, want the zero ArtistDetail", detail)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requestCount = %d, want exactly 1 (no retry)", got)
	}
}

func TestLookupArtist_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not json"))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	detail, err := c.LookupArtist(context.Background(), sampleArtistDetailMBID)
	if err == nil {
		t.Fatal("LookupArtist: got nil error, want a decode error")
	}
	if detail.MBID != "" || detail.Relations != nil || detail.Aliases != nil {
		t.Fatalf("detail = %+v, want the zero ArtistDetail", detail)
	}
}

func TestLookupArtist_NoRelationsNoAliasesYieldsNonNilZeroLengthSlices(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(noRelationsNoAliasesFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	detail, err := c.LookupArtist(context.Background(), sampleArtistDetailMBID)
	if err != nil {
		t.Fatalf("LookupArtist: %v", err)
	}
	if detail.Relations == nil {
		t.Fatal("Relations is nil, want a non-nil zero-length slice")
	}
	if len(detail.Relations) != 0 {
		t.Fatalf("len(Relations) = %d, want 0", len(detail.Relations))
	}
	if detail.Aliases == nil {
		t.Fatal("Aliases is nil, want a non-nil zero-length slice")
	}
	if len(detail.Aliases) != 0 {
		t.Fatalf("len(Aliases) = %d, want 0", len(detail.Aliases))
	}
}
