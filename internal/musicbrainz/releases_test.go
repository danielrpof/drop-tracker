package musicbrainz

// This file is package musicbrainz (whitebox), matching
// releasegroups_test.go/recordings_test.go's convention, so it can reuse
// newTestClient/unlimitedLimiter and point a Client at an httptest.Server
// via the unexported baseURL field. Fixtures below are [ASSUMED] per
// 04-RESEARCH.md Assumption A1 -- see releases.go's header comment --
// reconstructed by direct analogy to the live-verified release-group
// envelope, not confirmed against a live MusicBrainz response.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const sampleReleaseGroupMBID = "0b6c2da8-ac60-4663-beae-bd3dc6dea94a"

const releaseFixture = `{
  "release-count": 1,
  "release-offset": 0,
  "releases": [
    {
      "id": "aaaaaaaa-1111-0000-0000-000000000001",
      "title": "Nothing Was the Same (Deluxe)",
      "status": "Official",
      "date": "2013-09-24",
      "media": [
        {"format": "CD", "position": 1, "track-count": 12},
        {"format": "CD", "position": 2, "track-count": 9}
      ]
    }
  ]
}`

const emptyReleaseFixture = `{"release-count":0,"release-offset":0,"releases":[]}`

func TestReleasesByReleaseGroup_DecodesEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesByReleaseGroup(context.Background(), sampleReleaseGroupMBID)
	if err != nil {
		t.Fatalf("ReleasesByReleaseGroup: %v", err)
	}
	if len(releases) != 1 {
		t.Fatalf("len(releases) = %d, want 1", len(releases))
	}
	got := releases[0]
	if got.MBID != "aaaaaaaa-1111-0000-0000-000000000001" {
		t.Errorf("MBID = %q, want %q", got.MBID, "aaaaaaaa-1111-0000-0000-000000000001")
	}
	if got.Title != "Nothing Was the Same (Deluxe)" {
		t.Errorf("Title = %q, want %q", got.Title, "Nothing Was the Same (Deluxe)")
	}
	if got.Status != "Official" {
		t.Errorf("Status = %q, want %q", got.Status, "Official")
	}
	if got.Date != "2013-09-24" {
		t.Errorf("Date = %q, want %q", got.Date, "2013-09-24")
	}
	if len(got.Media) != 2 {
		t.Fatalf("len(Media) = %d, want 2", len(got.Media))
	}
	if got.Media[0].TrackCount != 12 || got.Media[1].TrackCount != 9 {
		t.Errorf("Media track counts = [%d, %d], want [12, 9]", got.Media[0].TrackCount, got.Media[1].TrackCount)
	}
}

func TestRelease_TrackCountSumsMedia(t *testing.T) {
	tests := []struct {
		name string
		r    Release
		want int
	}{
		{"two-disc release sums both media", Release{Media: []Medium{{TrackCount: 12}, {TrackCount: 9}}}, 21},
		{"absent media reports 0", Release{}, 0},
		{"media entries with no track-count report 0", Release{Media: []Medium{{Format: "CD", Position: 1}}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.TrackCount(); got != tt.want {
				t.Errorf("TrackCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestReleasesByReleaseGroup_RequestShape(t *testing.T) {
	var gotPath, gotGroup, gotInc, gotFmt, gotLimit, gotOffset, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotGroup = r.URL.Query().Get("release-group")
		gotInc = r.URL.Query().Get("inc")
		gotFmt = r.URL.Query().Get("fmt")
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	if _, err := c.ReleasesByReleaseGroup(context.Background(), sampleReleaseGroupMBID); err != nil {
		t.Fatalf("ReleasesByReleaseGroup: %v", err)
	}
	if gotPath != "/release" {
		t.Errorf("path = %q, want %q", gotPath, "/release")
	}
	if gotGroup != sampleReleaseGroupMBID {
		t.Errorf("release-group = %q, want %q", gotGroup, sampleReleaseGroupMBID)
	}
	if gotInc != "media" {
		t.Errorf("inc = %q, want %q", gotInc, "media")
	}
	if gotFmt != "json" {
		t.Errorf("fmt = %q, want %q", gotFmt, "json")
	}
	if gotLimit != strconv.Itoa(releasePageSize) {
		t.Errorf("limit = %q, want %q", gotLimit, strconv.Itoa(releasePageSize))
	}
	if gotOffset != "0" {
		t.Errorf("offset = %q, want %q", gotOffset, "0")
	}
	if gotUA == "" {
		t.Error("User-Agent header missing")
	}
}

// pagedReleaseServer serves releases across pages based on the offset query
// param, recording each request's offset for assertions -- mirrors
// releasegroups_test.go's pagedReleaseGroupServer.
func pagedReleaseServer(t *testing.T, total, pageSize int) (*httptest.Server, *[]int) {
	t.Helper()
	var mu sync.Mutex
	var offsets []int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		mu.Lock()
		offsets = append(offsets, offset)
		mu.Unlock()

		remaining := total - offset
		if remaining < 0 {
			remaining = 0
		}
		n := pageSize
		if remaining < n {
			n = remaining
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildReleasePageJSON(total, offset, n)))
	}))
	return ts, &offsets
}

func buildReleasePageJSON(count, offset, n int) string {
	var sb strings.Builder
	sb.WriteString(`{"release-count":`)
	sb.WriteString(strconv.Itoa(count))
	sb.WriteString(`,"release-offset":`)
	sb.WriteString(strconv.Itoa(offset))
	sb.WriteString(`,"releases":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		idx := offset + i
		sb.WriteString(`{"id":"rel-`)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","title":"Generated `)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","status":"Official","date":"2020","media":[{"format":"CD","position":1,"track-count":10}]}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func TestReleasesByReleaseGroup_Paginates(t *testing.T) {
	ts, offsets := pagedReleaseServer(t, 150, 100)
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesByReleaseGroup(context.Background(), sampleReleaseGroupMBID)
	if err != nil {
		t.Fatalf("ReleasesByReleaseGroup: %v", err)
	}
	if len(releases) != 150 {
		t.Fatalf("len(releases) = %d, want 150", len(releases))
	}
	for i, r := range releases {
		want := "rel-" + strconv.Itoa(i)
		if r.MBID != want {
			t.Fatalf("releases[%d].MBID = %q, want %q (page order broken)", i, r.MBID, want)
		}
	}
	want := []int{0, 100}
	if len(*offsets) != len(want) {
		t.Fatalf("offsets = %v, want %v", *offsets, want)
	}
	for i, o := range want {
		if (*offsets)[i] != o {
			t.Errorf("offsets[%d] = %d, want %d", i, (*offsets)[i], o)
		}
	}
}

func TestReleasesByReleaseGroup_EmptyMBID(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	for _, mbid := range []string{"", "   "} {
		releases, err := c.ReleasesByReleaseGroup(context.Background(), mbid)
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

func TestReleasesByReleaseGroup_NonOKStatus(t *testing.T) {
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
	releases, err := c.ReleasesByReleaseGroup(context.Background(), sampleReleaseGroupMBID)
	if err == nil {
		t.Fatal("ReleasesByReleaseGroup: got nil error, want a non-nil error naming the status")
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

func TestReleasesByReleaseGroup_EmptyResultIsNonNilZeroLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyReleaseFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	releases, err := c.ReleasesByReleaseGroup(context.Background(), sampleReleaseGroupMBID)
	if err != nil {
		t.Fatalf("ReleasesByReleaseGroup: %v", err)
	}
	if releases == nil {
		t.Fatal("releases is nil, want a non-nil zero-length slice")
	}
	if len(releases) != 0 {
		t.Fatalf("len(releases) = %d, want 0", len(releases))
	}
}
