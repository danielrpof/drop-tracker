package musicbrainz

// This file is package musicbrainz (whitebox), matching
// releasegroups_test.go's convention, so it can reuse newTestClient /
// unlimitedLimiter / sampleArtistMBID (defined in search_test.go) and point
// a Client at an httptest.Server via the unexported baseURL field. The
// recording-browse envelope shape below is [ASSUMED] per 04-RESEARCH.md
// Assumption A2 -- reconstructed by direct analogy to the live-verified
// release-group envelope from Phase 3, not confirmed against a live
// MusicBrainz response this session. See recordings.go's own header comment.

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

const recordingFixture = `{
  "recording-count": 1,
  "recording-offset": 0,
  "recordings": [
    {
      "id": "aaaaaaaa-0000-0000-0000-000000000001",
      "title": "Feature Track",
      "artist-credit": [
        {"name": "Primary Artist", "joinphrase": " feat. ", "artist": {"id": "primary-mbid-0000", "name": "Primary Artist"}},
        {"name": "Test Artist", "joinphrase": "", "artist": {"id": "9fff2f8a-21e6-47de-a2b8-7f449929d43f", "name": "Test Artist"}}
      ]
    }
  ]
}`

func TestRecordingsByArtist_DecodesEnvelope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	recordings, err := c.RecordingsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("RecordingsByArtist: %v", err)
	}
	if len(recordings) != 1 {
		t.Fatalf("len(recordings) = %d, want 1", len(recordings))
	}
	got := recordings[0]
	if got.MBID != "aaaaaaaa-0000-0000-0000-000000000001" {
		t.Errorf("MBID = %q, want %q", got.MBID, "aaaaaaaa-0000-0000-0000-000000000001")
	}
	if got.Title != "Feature Track" {
		t.Errorf("Title = %q, want %q", got.Title, "Feature Track")
	}
	if len(got.ArtistCredit) != 2 {
		t.Fatalf("len(ArtistCredit) = %d, want 2", len(got.ArtistCredit))
	}
	if got.ArtistCredit[0].Artist.MBID != "primary-mbid-0000" {
		t.Errorf("ArtistCredit[0].Artist.MBID = %q, want %q", got.ArtistCredit[0].Artist.MBID, "primary-mbid-0000")
	}
	if got.ArtistCredit[0].Artist.Name != "Primary Artist" {
		t.Errorf("ArtistCredit[0].Artist.Name = %q, want %q", got.ArtistCredit[0].Artist.Name, "Primary Artist")
	}
	if got.ArtistCredit[0].JoinPhrase != " feat. " {
		t.Errorf("ArtistCredit[0].JoinPhrase = %q, want %q", got.ArtistCredit[0].JoinPhrase, " feat. ")
	}
	if got.ArtistCredit[1].Artist.MBID != sampleArtistMBID {
		t.Errorf("ArtistCredit[1].Artist.MBID = %q, want %q", got.ArtistCredit[1].Artist.MBID, sampleArtistMBID)
	}
}

func TestRecordingsByArtist_RequestShape(t *testing.T) {
	var gotPath, gotArtist, gotInc, gotFmt, gotLimit, gotOffset, gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotArtist = r.URL.Query().Get("artist")
		gotInc = r.URL.Query().Get("inc")
		gotFmt = r.URL.Query().Get("fmt")
		gotLimit = r.URL.Query().Get("limit")
		gotOffset = r.URL.Query().Get("offset")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	if _, err := c.RecordingsByArtist(context.Background(), sampleArtistMBID); err != nil {
		t.Fatalf("RecordingsByArtist: %v", err)
	}
	if gotPath != "/recording" {
		t.Errorf("path = %q, want %q", gotPath, "/recording")
	}
	if gotArtist != sampleArtistMBID {
		t.Errorf("artist = %q, want %q", gotArtist, sampleArtistMBID)
	}
	if gotInc != "artist-credits" {
		t.Errorf("inc = %q, want %q", gotInc, "artist-credits")
	}
	if gotFmt != "json" {
		t.Errorf("fmt = %q, want %q", gotFmt, "json")
	}
	if gotLimit != "100" {
		t.Errorf("limit = %q, want %q", gotLimit, "100")
	}
	if gotOffset != "0" {
		t.Errorf("offset = %q, want %q", gotOffset, "0")
	}
	if gotUA == "" {
		t.Error("User-Agent header missing")
	}
}

// pagedRecordingServer serves recordings across pages based on the offset
// query param, recording each request's offset for assertions -- mirrors
// releasegroups_test.go's pagedReleaseGroupServer.
func pagedRecordingServer(t *testing.T, total int, pageSize int) (*httptest.Server, *[]int) {
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
		_, _ = w.Write([]byte(buildRecordingPageJSON(total, offset, n)))
	}))
	return ts, &offsets
}

func buildRecordingPageJSON(count, offset, n int) string {
	var sb strings.Builder
	sb.WriteString(`{"recording-count":`)
	sb.WriteString(strconv.Itoa(count))
	sb.WriteString(`,"recording-offset":`)
	sb.WriteString(strconv.Itoa(offset))
	sb.WriteString(`,"recordings":[`)
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		idx := offset + i
		sb.WriteString(`{"id":"gen-`)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","title":"Generated `)
		sb.WriteString(strconv.Itoa(idx))
		sb.WriteString(`","artist-credit":[{"name":"Other","joinphrase":"","artist":{"id":"other-mbid","name":"Other"}}]}`)
	}
	sb.WriteString(`]}`)
	return sb.String()
}

func TestRecordingsByArtist_Paginates(t *testing.T) {
	ts, offsets := pagedRecordingServer(t, 150, 100)
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	recordings, err := c.RecordingsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("RecordingsByArtist: %v", err)
	}
	if len(recordings) != 150 {
		t.Fatalf("len(recordings) = %d, want 150", len(recordings))
	}
	for i, r := range recordings {
		want := "gen-" + strconv.Itoa(i)
		if r.MBID != want {
			t.Fatalf("recordings[%d].MBID = %q, want %q (page order broken)", i, r.MBID, want)
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

func TestRecordingsByArtist_EmptyMBID(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(recordingFixture))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	for _, mbid := range []string{"", "   "} {
		recordings, err := c.RecordingsByArtist(context.Background(), mbid)
		if !errors.Is(err, ErrEmptyMBID) {
			t.Fatalf("mbid %q: err = %v, want ErrEmptyMBID", mbid, err)
		}
		if recordings != nil {
			t.Fatalf("mbid %q: recordings = %v, want nil", mbid, recordings)
		}
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (empty mbid must not issue a request)", got)
	}
}

func TestRecordingsByArtist_NonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	recordings, err := c.RecordingsByArtist(context.Background(), sampleArtistMBID)
	if err == nil {
		t.Fatal("RecordingsByArtist: got nil error, want a non-nil error naming the status")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error = %q, want it to name status 503", err.Error())
	}
	if recordings != nil {
		t.Fatalf("recordings = %v, want nil", recordings)
	}
}

// --- Task 2: page-ceiling truncation ---

func TestRecordingsByArtist_StopsAtPageCeiling(t *testing.T) {
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
		// forever (T-04-13).
		_, _ = w.Write([]byte(buildRecordingPageJSON(1000000, offset, recordingPageSize)))
	}))
	defer ts.Close()

	c := newTestClient(t, ts, "drop-tracker-test/1.0", unlimitedLimiter())
	recordings, err := c.RecordingsByArtist(context.Background(), sampleArtistMBID)
	if err != nil {
		t.Fatalf("RecordingsByArtist: %v", err)
	}
	if len(recordings) != MaxRecordingBrowseItems {
		t.Fatalf("len(recordings) = %d, want %d", len(recordings), MaxRecordingBrowseItems)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != maxRecordingPages {
		t.Fatalf("requestCount = %d, want exactly maxRecordingPages (%d)", got, maxRecordingPages)
	}
}
