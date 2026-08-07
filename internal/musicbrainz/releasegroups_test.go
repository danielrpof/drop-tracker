package musicbrainz

// This file is package musicbrainz (whitebox), matching search_test.go's
// convention, so it can reuse newTestClient/unlimitedLimiter and point a
// Client at an httptest.Server via the unexported baseURL field. Fixtures
// below are transcribed verbatim from the live-verified MusicBrainz
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

const sampleArtistMBID = "9fff2f8a-21e6-47de-a2b8-7f449929d43f"

const releaseGroupFixture = `{
  "release-group-count": 61,
  "release-group-offset": 0,
  "release-groups": [
    {
      "id": "0b6c2da8-ac60-4663-beae-bd3dc6dea94a",
      "title": "Nothing Was the Same",
      "primary-type": "Album",
      "secondary-types": [],
      "first-release-date": "2013-09-19",
      "disambiguation": ""
    }
  ]
}`

const emptyReleaseGroupFixture = `{"release-group-count":0,"release-group-offset":0,"release-groups":[]}`

const duplicateTitleReleaseGroupFixture = `{
  "release-group-count": 2,
  "release-group-offset": 0,
  "release-groups": [
    {"id": "aaaaaaaa-0000-0000-0000-000000000001", "title": "Nothing Was the Same", "primary-type": "Album", "secondary-types": [], "first-release-date": "2013-09-19", "disambiguation": ""},
    {"id": "bbbbbbbb-0000-0000-0000-000000000002", "title": "Nothing Was the Same", "primary-type": "Album", "secondary-types": [], "first-release-date": "2013-09-19", "disambiguation": ""}
  ]
}`

const partialDateReleaseGroupFixture = `{
  "release-group-count": 3,
  "release-group-offset": 0,
  "release-groups": [
    {"id": "cccccccc-0000-0000-0000-000000000001", "title": "Year Only", "primary-type": "Album", "secondary-types": [], "first-release-date": "2013", "disambiguation": ""},
    {"id": "dddddddd-0000-0000-0000-000000000002", "title": "Year Month", "primary-type": "Album", "secondary-types": [], "first-release-date": "2013-09", "disambiguation": ""},
    {"id": "eeeeeeee-0000-0000-0000-000000000003", "title": "No Date", "primary-type": "Album", "secondary-types": [], "first-release-date": "", "disambiguation": ""}
  ]
}`

func TestReleaseGroupsByArtist_DecodesFixture(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	got := groups[0]
	if got.MBID != "0b6c2da8-ac60-4663-beae-bd3dc6dea94a" {
		t.Errorf("MBID = %q, want %q", got.MBID, "0b6c2da8-ac60-4663-beae-bd3dc6dea94a")
	}
	if got.Title != "Nothing Was the Same" {
		t.Errorf("Title = %q, want %q", got.Title, "Nothing Was the Same")
	}
	if got.PrimaryType != "Album" {
		t.Errorf("PrimaryType = %q, want %q", got.PrimaryType, "Album")
	}
	if got.FirstReleaseDate != "2013-09-19" {
		t.Errorf("FirstReleaseDate = %q, want %q", got.FirstReleaseDate, "2013-09-19")
	}
	if len(got.SecondaryTypes) != 0 {
		t.Errorf("SecondaryTypes = %v, want empty", got.SecondaryTypes)
	}
	if got.Disambiguation != "" {
		t.Errorf("Disambiguation = %q, want empty", got.Disambiguation)
	}
}

func TestReleaseGroupsByArtist_RequestShape(t *testing.T) {
	var gotPath, gotArtist, gotFmt, gotLimit, gotOffset, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotArtist = r.URL.Query().Get("artist")
		gotFmt = r.URL.Query().Get("fmt")
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	if _, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID); err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if gotPath != "/release-group" {
		t.Errorf("path = %q, want %q", gotPath, "/release-group")
	}
	if gotArtist != sampleArtistMBID {
		t.Errorf("artist = %q, want %q", gotArtist, sampleArtistMBID)
	}
	if gotFmt != "json" {
		t.Errorf("fmt = %q, want %q", gotFmt, "json")
	}
	if gotLimit == "" {
		t.Error("limit param missing")
	}
	if gotOffset != "0" {
		t.Errorf("offset = %q, want %q", gotOffset, "0")
	}
	if gotUA == "" {
		t.Error("User-Agent header missing")
	}
}

func TestReleaseGroupsByArtist_NoTypeFilterSent(t *testing.T) {
	sawType := false
	var gotType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v := r.URL.Query().Get("type"); v != "" {
			sawType = true
			gotType = v
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	if _, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID); err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if sawType {
		t.Errorf("type param = %q, want no type param sent", gotType)
	}
}

func TestReleaseGroupsByArtist_DuplicateTitlePreservedAcrossCalls(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(duplicateTitleReleaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	for i := 0; i < 3; i++ {
		groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
		if err != nil {
			t.Fatalf("call %d: ReleaseGroupsByArtist: %v", i, err)
		}
		if len(groups) != 2 {
			t.Fatalf("call %d: len(groups) = %d, want 2", i, len(groups))
		}
		if groups[0].MBID == groups[1].MBID {
			t.Fatalf("call %d: both entries share MBID %q, want distinct", i, groups[0].MBID)
		}
		if groups[0].Title != "Nothing Was the Same" || groups[1].Title != "Nothing Was the Same" {
			t.Fatalf("call %d: titles = [%q, %q], want both %q", i, groups[0].Title, groups[1].Title, "Nothing Was the Same")
		}
	}
}

func TestReleaseGroupsByArtist_PartialDatesPreservedVerbatim(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(partialDateReleaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != 3 {
		t.Fatalf("len(groups) = %d, want 3", len(groups))
	}
	want := []string{"2013", "2013-09", ""}
	for i, w := range want {
		if groups[i].FirstReleaseDate != w {
			t.Errorf("groups[%d].FirstReleaseDate = %q, want %q", i, groups[i].FirstReleaseDate, w)
		}
	}
}

func TestReleaseGroupsByArtist_EmptyResultIsNonNilZeroLength(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(emptyReleaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if groups == nil {
		t.Fatal("groups is nil, want a non-nil zero-length slice")
	}
	if len(groups) != 0 {
		t.Fatalf("len(groups) = %d, want 0", len(groups))
	}
}

func TestReleaseGroupsByArtist_EmptyMBID(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())

	for _, mbid := range []string{"", "   "} {
		groups, err := c.ReleaseGroupsByArtist(context.Background(), mbid)
		if !errors.Is(err, ErrEmptyMBID) {
			t.Fatalf("mbid %q: err = %v, want ErrEmptyMBID", mbid, err)
		}
		if groups != nil {
			t.Fatalf("mbid %q: groups = %v, want nil", mbid, groups)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (empty mbid must not issue a request)", got)
	}
}

func TestReleaseGroupsByArtist_UpstreamErrorStatusNoRetry(t *testing.T) {
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
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err == nil {
		t.Fatal("ReleaseGroupsByArtist: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if groups != nil {
		t.Fatalf("groups = %v, want nil", groups)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requestCount = %d, want exactly 1 (no retry)", got)
	}
}

func TestReleaseGroupsByArtist_CancelledContextMakesZeroRequests(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(releaseGroupFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	groups, err := c.ReleaseGroupsByArtist(ctx, sampleArtistMBID)
	if err == nil {
		t.Fatal("ReleaseGroupsByArtist: got nil error, want a wrapped context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReleaseGroupsByArtist error = %v, want it to wrap context.Canceled", err)
	}
	if groups != nil {
		t.Fatalf("groups = %v, want nil", groups)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (a cancelled context must not reach the server)", got)
	}
}

// --- Task 2: bounded pagination and proven pacing across pages ---

// pagedReleaseGroupServer serves release-groups across pages based on the
// offset query param, recording each request's offset for assertions.
func pagedReleaseGroupServer(t *testing.T, total int, pageSize int) (*httptest.Server, *[]int) {
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
		_, _ = w.Write([]byte(buildReleaseGroupPageJSON(total, offset, n)))
	}))
	return ts, &offsets
}

func buildReleaseGroupPageJSON(count, offset, n int) string {
	var sb strings.Builder
	sb.WriteString(`{"release-group-count":`)
	sb.WriteString(strconv.Itoa(count))
	sb.WriteString(`,"release-group-offset":`)
	sb.WriteString(strconv.Itoa(offset))
	sb.WriteString(`,"release-groups":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		idx := offset + i
		sb.WriteString(`{"id":"gen-`)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","title":"Generated `)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","primary-type":"Album","secondary-types":[],"first-release-date":"2020","disambiguation":""}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func TestReleaseGroupsByArtist_PaginationCollectsAllPagesInOrder(t *testing.T) {
	ts, offsets := pagedReleaseGroupServer(t, 250, 100)
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != 250 {
		t.Fatalf("len(groups) = %d, want 250", len(groups))
	}
	for i, g := range groups {
		want := "gen-" + strconv.Itoa(i)
		if g.MBID != want {
			t.Fatalf("groups[%d].MBID = %q, want %q (page order broken)", i, g.MBID, want)
		}
	}

	want := []int{0, 100, 200}
	if len(*offsets) != len(want) {
		t.Fatalf("offsets = %v, want %v", *offsets, want)
	}
	for i, o := range want {
		if (*offsets)[i] != o {
			t.Errorf("offsets[%d] = %d, want %d", i, (*offsets)[i], o)
		}
	}
}

func TestReleaseGroupsByArtist_PacingAppliesAcrossPages(t *testing.T) {
	ts, _ := pagedReleaseGroupServer(t, 250, 100)
	defer ts.Close()

	// rate.Limit(20) with burst 1: the first page is free, the second and
	// third each wait ~50ms for a fresh token -- pagination cannot outrun
	// the configured rate.
	limiter := rate.NewLimiter(rate.Limit(20), 1)
	c := newTestClient(t, ts, "drop-tracker-test/1.0", limiter)

	start := time.Now()
	if _, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID); err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least 100ms (every page must be separately paced)", elapsed)
	}
}

func TestReleaseGroupsByArtist_StopsAsSoonAsCountReached(t *testing.T) {
	ts, offsets := pagedReleaseGroupServer(t, 61, 100)
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != 61 {
		t.Fatalf("len(groups) = %d, want 61", len(groups))
	}
	if len(*offsets) != 1 {
		t.Fatalf("requestCount = %d, want exactly 1", len(*offsets))
	}
}

func TestReleaseGroupsByArtist_PageCapStopsRunawayCount(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		w.Header().Set("Content-Type", "application/json")
		// Always claims a huge count while always returning a full page --
		// a hostile/runaway upstream that would otherwise drive requests
		// forever (T-03-12).
		_, _ = w.Write([]byte(buildReleaseGroupPageJSON(1000000, offset, releaseGroupPageSize)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != maxReleaseGroupPages*releaseGroupPageSize {
		t.Fatalf("len(groups) = %d, want %d", len(groups), maxReleaseGroupPages*releaseGroupPageSize)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != maxReleaseGroupPages {
		t.Fatalf("requestCount = %d, want exactly maxReleaseGroupPages (%d)", got, maxReleaseGroupPages)
	}
}

func TestReleaseGroupsByArtist_ZeroEntryPageTerminatesLoop(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		// Reports a huge count but always returns zero entries -- the loop
		// must terminate on the empty page rather than spin forever.
		_, _ = w.Write([]byte(`{"release-group-count":1000000,"release-group-offset":0,"release-groups":[]}`))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("ReleaseGroupsByArtist: %v", err)
	}
	if len(groups) != 0 {
		t.Fatalf("len(groups) = %d, want 0", len(groups))
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 1 {
		t.Fatalf("requestCount = %d, want exactly 1", got)
	}
}

func TestReleaseGroupsByArtist_MidFetchErrorStopsWithNoRetry(t *testing.T) {
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
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildReleaseGroupPageJSON(250, offset, 100)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(context.Background(), sampleArtistMBID)
	if err == nil {
		t.Fatal("ReleaseGroupsByArtist: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if groups != nil {
		t.Fatalf("groups = %v, want nil", groups)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 2 {
		t.Fatalf("requestCount = %d, want exactly 2 (no retry, no continuation past the failure)", got)
	}
}

func TestReleaseGroupsByArtist_CancellationBetweenPagesAborts(t *testing.T) {
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
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(buildReleaseGroupPageJSON(250, offset, 100)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	groups, err := c.ReleaseGroupsByArtist(ctx, sampleArtistMBID)
	if err == nil {
		t.Fatal("ReleaseGroupsByArtist: got nil error, want ctx.Err()")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ReleaseGroupsByArtist error = %v, want it to wrap context.Canceled", err)
	}
	if groups != nil {
		t.Fatalf("groups = %v, want nil", groups)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got >= 3 {
		t.Fatalf("requestCount = %d, want fewer than the full page count (3)", got)
	}
}

func TestReleaseGroupsByArtist_FractionalRateLimiterDelaysSecondRequest(t *testing.T) {
	limiter := rate.NewLimiter(rate.Limit(0.5), 1)
	// Consume the initial burst token.
	if !limiter.Allow() {
		t.Fatal("expected the first Allow() to succeed (burst token available)")
	}
	delay := limiter.Reserve().Delay()
	if delay < 1900*time.Millisecond {
		t.Fatalf("delay = %v, want at least ~2s for rate.Limit(0.5)", delay)
	}
}
