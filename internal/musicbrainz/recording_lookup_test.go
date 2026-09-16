package musicbrainz

// This file is package musicbrainz (whitebox), matching
// releases_test.go/releasegroups_test.go's convention, so it can reuse
// newTestClient/unlimitedLimiter and point a Client at an httptest.Server
// via the unexported baseURL field. The fixture below is [ASSUMED] per
// 13-RESEARCH.md Assumption A1 -- see recording_lookup.go's header comment
// -- reconstructed by direct analogy to the live-verified release-group and
// recording browse envelopes, not confirmed against a live MusicBrainz
// response this session.

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

const sampleRecordingMBID = "cccccccc-2222-0000-0000-000000000001"

const recordingLookupFixture = `{
  "id": "cccccccc-2222-0000-0000-000000000001",
  "title": "Feature Track",
  "releases": [
    {
      "id": "aaaaaaaa-1111-0000-0000-000000000001",
      "title": "Some Album",
      "date": "2026-03-04",
      "release-group": {
        "id": "rg-1",
        "title": "Some Album"
      }
    }
  ]
}`

const emptyRecordingLookupFixture = `{"id":"cccccccc-2222-0000-0000-000000000001","title":"Feature Track","releases":[]}`

func TestReleasesForRecording_DecodesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingLookupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesForRecording(context.Background(), sampleRecordingMBID)
	if err != nil {
		t.Fatalf("ReleasesForRecording: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("len(releases) = %d, want 1", len(releases))
	}
	got := releases[0]
	if got.MBID != "aaaaaaaa-1111-0000-0000-000000000001" {
		t.Errorf("MBID = %q, want %q", got.MBID, "aaaaaaaa-1111-0000-0000-000000000001")
	}
	if got.Title != "Some Album" {
		t.Errorf("Title = %q, want %q", got.Title, "Some Album")
	}
	if got.Date != "2026-03-04" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-03-04")
	}
	if got.ReleaseGroup.MBID != "rg-1" {
		t.Errorf("ReleaseGroup.MBID = %q, want %q", got.ReleaseGroup.MBID, "rg-1")
	}
	if got.ReleaseGroup.Title != "Some Album" {
		t.Errorf("ReleaseGroup.Title = %q, want %q", got.ReleaseGroup.Title, "Some Album")
	}
}

func TestReleasesForRecording_RequestShape(t *testing.T) {
	var gotPath, gotInc, gotFmt, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotInc = r.URL.Query().Get("inc")
		gotFmt = r.URL.Query().Get("fmt")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingLookupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	// A slash and a space in the mbid exercise url.PathEscape (T-13-01) --
	// raw concatenation would corrupt the request path or let the caller
	// influence the path structure.
	mbidWithSpecialChars := "abc/def 123"
	if _, err := c.ReleasesForRecording(context.Background(), mbidWithSpecialChars); err != nil {
		t.Fatalf("ReleasesForRecording: %v", err)
	}
	wantPath := "/recording/" + url.PathEscape(mbidWithSpecialChars)
	if gotPath != wantPath {
		t.Errorf("path = %q, want %q", gotPath, wantPath)
	}
	if gotInc != "releases+release-groups" {
		t.Errorf("inc = %q, want %q", gotInc, "releases+release-groups")
	}
	if gotFmt != "json" {
		t.Errorf("fmt = %q, want %q", gotFmt, "json")
	}
	if gotUA == "" {
		t.Error("User-Agent header missing")
	}
}

func TestReleasesForRecording_EmptyMBID(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingLookupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	for _, mbid := range []string{"", "   "} {
		releases, err := c.ReleasesForRecording(context.Background(), mbid)
		if !errors.Is(err, ErrEmptyMBID) {
			t.Fatalf("mbid %q: err = %v, want ErrEmptyMBID", mbid, err)
		}
		if releases != nil {
			t.Fatalf("mbid %q: releases = %v, want nil", mbid, releases)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (empty mbid must not issue a request)", got)
	}
}

func TestReleasesForRecording_NonOKStatus(t *testing.T) {
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
	releases, err := c.ReleasesForRecording(context.Background(), sampleRecordingMBID)
	if err == nil {
		t.Fatal("ReleasesForRecording: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if releases != nil {
		t.Fatalf("releases = %v, want nil", releases)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requestCount = %d, want exactly 1 (no retry)", got)
	}
}

func TestReleasesForRecording_MalformedJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not json"))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesForRecording(context.Background(), sampleRecordingMBID)
	if err == nil {
		t.Fatal("ReleasesForRecording: got nil error, want a decode error")
	}
	if releases != nil {
		t.Fatalf("releases = %v, want nil", releases)
	}
}

func TestReleasesForRecording_EmptyResultIsNonNilZeroLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyRecordingLookupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesForRecording(context.Background(), sampleRecordingMBID)
	if err != nil {
		t.Fatalf("ReleasesForRecording: %v", err)
	}
	if releases == nil {
		t.Fatal("releases is nil, want a non-nil zero-length slice")
	}
	if len(releases) != 0 {
		t.Fatalf("len(releases) = %d, want 0", len(releases))
	}
}
