package httpserver_test

// This is the tracer: one entry point (attach) wired through every backend
// layer against a real Postgres, proving the whole chain -- normalize,
// get-or-create, link, and the single-query GET /watchlist enrichment.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// tagWire is the JSON shape of a single tag on the wire.
type tagWire struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// watchlistTagsEntryBody is the subset of GET /watchlist's per-entry body
// this file asserts on.
type watchlistTagsEntryBody struct {
	ID   int64     `json:"id"`
	MBID string    `json:"mbid"`
	Tags []tagWire `json:"tags"`
	Note *string   `json:"note"`
}

func TestTags_AttachEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	tagged, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-tagged", Name: "Tagged Artist"})
	if err != nil {
		t.Fatalf("seed tagged entry: %v", err)
	}
	untagged, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-untagged", Name: "Untagged Artist"})
	if err != nil {
		t.Fatalf("seed untagged entry: %v", err)
	}

	body := `{"name":"Reggaeton "}`
	resp, err := http.Post(ts.URL+"/watchlist/"+strconv.FormatInt(tagged.ID, 10)+"/tags", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist/{id}/tags: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	var attached tagWire
	if err := json.NewDecoder(resp.Body).Decode(&attached); err != nil {
		t.Fatalf("decode attach response: %v", err)
	}
	if attached.Name != "Reggaeton" {
		t.Fatalf("attached tag name = %q, want %q (trimmed)", attached.Name, "Reggaeton")
	}
	if attached.ID == 0 {
		t.Fatal("attached tag id is zero")
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()

	var entries []watchlistTagsEntryBody
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode watchlist list: %v", err)
	}

	var taggedEntry, untaggedEntry *watchlistTagsEntryBody
	for i := range entries {
		switch entries[i].ID {
		case tagged.ID:
			taggedEntry = &entries[i]
		case untagged.ID:
			untaggedEntry = &entries[i]
		}
	}
	if taggedEntry == nil {
		t.Fatal("tagged entry missing from GET /watchlist")
	}
	if untaggedEntry == nil {
		t.Fatal("untagged entry missing from GET /watchlist")
	}

	if len(taggedEntry.Tags) != 1 || taggedEntry.Tags[0].ID != attached.ID || taggedEntry.Tags[0].Name != "Reggaeton" {
		t.Fatalf("tagged entry tags = %+v, want [{%d Reggaeton}]", taggedEntry.Tags, attached.ID)
	}
	if taggedEntry.Note != nil {
		t.Fatalf("tagged entry note = %v, want nil", *taggedEntry.Note)
	}

	if untaggedEntry.Tags == nil {
		t.Fatal("untagged entry tags is null, want []")
	}
	if len(untaggedEntry.Tags) != 0 {
		t.Fatalf("untagged entry tags = %+v, want []", untaggedEntry.Tags)
	}
}

// --- Task 1: GET /tags (vocabulary list) ---

// tagSummaryWire is the JSON shape of a single GET /tags entry.
type tagSummaryWire struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CarrierCount int64  `json:"carrier_count"`
}

// TestTags_ListEndToEnd is the tracer for the vocabulary routes: an empty
// isolated schema proves GET /tags answers a literal "[]" (never null,
// never omitted), then a seeded vocabulary proves watched-only carrier
// counts (D-11), a zero-link tag surfacing (D-12), and a removed-artist-only
// link still surfacing uncounted (D-13), all in the DB's lower(name) order.
func TestTags_ListEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_list_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	emptyResp, err := http.Get(ts.URL + "/tags")
	if err != nil {
		t.Fatalf("GET /tags (empty vocabulary): %v", err)
	}
	emptyBody, err := io.ReadAll(emptyResp.Body)
	_ = emptyResp.Body.Close()
	if err != nil {
		t.Fatalf("read empty body: %v", err)
	}
	if emptyResp.StatusCode != http.StatusOK {
		t.Fatalf("empty status = %d, want 200", emptyResp.StatusCode)
	}
	if got := strings.TrimSpace(string(emptyBody)); got != "[]" {
		t.Fatalf("empty vocabulary body = %q, want %q", got, "[]")
	}

	watched1, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-list-1", Name: "List Artist 1"})
	if err != nil {
		t.Fatalf("seed watched1: %v", err)
	}
	watched2, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-list-2", Name: "List Artist 2"})
	if err != nil {
		t.Fatalf("seed watched2: %v", err)
	}
	removed, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-list-removed", Name: "List Removed Artist"})
	if err != nil {
		t.Fatalf("seed removed: %v", err)
	}

	if _, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", "Drill"); err != nil {
		t.Fatalf("seed zero-link tag: %v", err)
	}
	if _, err := tagsSvc.Attach(ctx, watched1.ID, "reggaeton"); err != nil {
		t.Fatalf("attach reggaeton to watched1: %v", err)
	}
	if _, err := tagsSvc.Attach(ctx, watched2.ID, "reggaeton"); err != nil {
		t.Fatalf("attach reggaeton to watched2: %v", err)
	}
	if _, err := tagsSvc.Attach(ctx, watched1.ID, "latin"); err != nil {
		t.Fatalf("attach latin to watched1: %v", err)
	}
	if _, err := tagsSvc.Attach(ctx, removed.ID, "latin"); err != nil {
		t.Fatalf("attach latin to removed: %v", err)
	}
	if err := watchlistSvc.Remove(ctx, removed.ID); err != nil {
		t.Fatalf("remove watchlist entry: %v", err)
	}

	resp, err := http.Get(ts.URL + "/tags")
	if err != nil {
		t.Fatalf("GET /tags: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var got []tagSummaryWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	byName := make(map[string]tagSummaryWire, len(got))
	names := make([]string, len(got))
	for i, s := range got {
		byName[s.Name] = s
		names[i] = s.Name
	}

	wantOrder := []string{"Drill", "latin", "reggaeton"}
	if !reflect.DeepEqual(names, wantOrder) {
		t.Fatalf("tag order = %v, want %v", names, wantOrder)
	}
	if got := byName["Drill"].CarrierCount; got != 0 {
		t.Fatalf("Drill carrier_count = %d, want 0", got)
	}
	if got := byName["latin"].CarrierCount; got != 1 {
		t.Fatalf("latin carrier_count = %d, want 1", got)
	}
	if got := byName["reggaeton"].CarrierCount; got != 2 {
		t.Fatalf("reggaeton carrier_count = %d, want 2", got)
	}
}

func TestTags_List_ServiceUnavailableWhenStoreOmitted(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{omitTags: true})

	resp, err := http.Get(ts.URL + "/tags")
	if err != nil {
		t.Fatalf("GET /tags: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

// --- Task 3: error mapping, concurrency, gate/CSRF ---

// fakeTagStore is a file-local double for httpserver.TagStore, mirroring
// fakeSettingsStore's func-field shape. attachCalls/detachCalls, when
// non-nil, count invocations so "store never called" is a direct
// observation.
type fakeTagStore struct {
	attachFunc  func(ctx context.Context, entryID int64, name string) (tags.AttachResult, error)
	detachFunc  func(ctx context.Context, entryID, tagID int64) error
	listFunc    func(ctx context.Context) ([]tags.Summary, error)
	attachCalls *int32
	detachCalls *int32
}

func (f fakeTagStore) Attach(ctx context.Context, entryID int64, name string) (tags.AttachResult, error) {
	if f.attachCalls != nil {
		atomic.AddInt32(f.attachCalls, 1)
	}
	if f.attachFunc != nil {
		return f.attachFunc(ctx, entryID, name)
	}
	return tags.AttachResult{Tag: tags.Tag{ID: 1, Name: name}, Created: true}, nil
}

func (f fakeTagStore) Detach(ctx context.Context, entryID, tagID int64) error {
	if f.detachCalls != nil {
		atomic.AddInt32(f.detachCalls, 1)
	}
	if f.detachFunc != nil {
		return f.detachFunc(ctx, entryID, tagID)
	}
	return nil
}

func (f fakeTagStore) List(ctx context.Context) ([]tags.Summary, error) {
	if f.listFunc != nil {
		return f.listFunc(ctx)
	}
	return []tags.Summary{}, nil
}

var _ httpserver.TagStore = fakeTagStore{}

// tagsServerOpts/newTagsServer mirror settingsServerOpts/newSettingsServer's
// shape: gated or inert, a supplied fake store, or WithTags omitted
// entirely.
type tagsServerOpts struct {
	passphrase string
	store      httpserver.TagStore
	omitTags   bool
}

func newTagsServer(t *testing.T, o tagsServerOpts) *httptest.Server {
	t.Helper()
	opts := []httpserver.Option{}
	if o.passphrase != "" {
		opts = append(opts, httpserver.WithAuthGate(o.passphrase, false, nil))
	}
	if !o.omitTags {
		store := o.store
		if store == nil {
			store = fakeTagStore{}
		}
		opts = append(opts, httpserver.WithTags(store))
	}
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(), opts...)
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func TestTags_Attach_UnknownEntryReturns404(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{attachFunc: func(context.Context, int64, string) (tags.AttachResult, error) {
		return tags.AttachResult{}, tags.ErrEntryNotFound
	}}})

	resp, err := http.Post(ts.URL+"/watchlist/999/tags", "application/json", strings.NewReader(`{"name":"rap"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestTags_Attach_TooLongNameReturns400(t *testing.T) {
	var calls int32
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{attachCalls: &calls}})

	body := `{"name":"` + strings.Repeat("a", 33) + `"}`
	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store.Attach called %d times, want 0 (fail-fast before the store call)", got)
	}
}

func TestTags_Attach_BlankNameReturns400(t *testing.T) {
	var calls int32
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{attachCalls: &calls}})

	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(`{"name":"   "}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store.Attach called %d times, want 0 (fail-fast before the store call)", got)
	}
}

func TestTags_Attach_AtCapReturns409(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{attachFunc: func(context.Context, int64, string) (tags.AttachResult, error) {
		return tags.AttachResult{}, tags.ErrTagCapReached
	}}})

	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(`{"name":"rap"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

func TestTags_Attach_BadJSONReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(`{"name":`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Attach_UnknownKeyReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(`{"name":"rap","extra":"x"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Detach_TwiceReturns204Both(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1/tags/2", nil)
		if err != nil {
			t.Fatalf("build request %d: %v", i, err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("DELETE %d: %v", i, err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("attempt %d: status = %d, want 204", i, resp.StatusCode)
		}
	}
}

func TestTags_Detach_UnknownEntryReturns404(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{detachFunc: func(context.Context, int64, int64) error {
		return tags.ErrEntryNotFound
	}}})

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/999/tags/1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestTags_Detach_NonNumericTagIDReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1/tags/abc", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// postTag issues POST /watchlist/{id}/tags with the given name and returns
// the status code, closing the response body.
func postTag(t *testing.T, ts *httptest.Server, entryID int64, name string) int {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q}`, name)
	resp, err := http.Post(ts.URL+"/watchlist/"+strconv.FormatInt(entryID, 10)+"/tags", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST tag %q: %v", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// TestTags_Attach_ConcurrentCapRace is ADR 0004 required test 1 at the HTTP
// layer: two concurrent POSTs of two different new names against an artist
// at 9 links yield exactly one 201 and one 409, and the artist ends with
// exactly 10 links. Looped with fresh names each iteration -- an unforced
// race can pass by luck on a single run.
func TestTags_Attach_ConcurrentCapRace(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_race_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	entry, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-race", Name: "Race Artist"})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}

	for iter := 0; iter < 10; iter++ {
		if _, err := pool.Exec(ctx, "DELETE FROM artist_tags WHERE artist_id = $1", entry.ArtistID); err != nil {
			t.Fatalf("iter %d: reset links: %v", iter, err)
		}
		for i := 0; i < 9; i++ {
			name := fmt.Sprintf("race-seed-%d-%d", iter, i)
			if _, err := tagsSvc.Attach(ctx, entry.ID, name); err != nil {
				t.Fatalf("iter %d: seed attach %d: %v", iter, i, err)
			}
		}

		nameA := fmt.Sprintf("race-a-%d", iter)
		nameB := fmt.Sprintf("race-b-%d", iter)

		statuses := make([]int, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			statuses[0] = postTag(t, ts, entry.ID, nameA)
		}()
		go func() {
			defer wg.Done()
			statuses[1] = postTag(t, ts, entry.ID, nameB)
		}()
		wg.Wait()

		sort.Ints(statuses)
		want := []int{http.StatusCreated, http.StatusConflict}
		if !reflect.DeepEqual(statuses, want) {
			t.Fatalf("iter %d: sorted statuses = %v, want %v", iter, statuses, want)
		}

		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", entry.ArtistID).Scan(&count); err != nil {
			t.Fatalf("iter %d: count links: %v", iter, err)
		}
		if count != 10 {
			t.Fatalf("iter %d: artist_tags count = %d, want 10", iter, count)
		}
	}
}

// TestTags_Detach_ConcurrentSameLinkBoth204 proves detach's idempotency
// (D-21) survives a real, unforced concurrent race: two DELETEs of the same
// link both return 204.
func TestTags_Detach_ConcurrentSameLinkBoth204(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_race_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	entry, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-detach-race", Name: "Detach Race Artist"})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	result, err := tagsSvc.Attach(ctx, entry.ID, "detach-race-tag")
	if err != nil {
		t.Fatalf("seed attach: %v", err)
	}

	const n = 2
	statuses := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/watchlist/%d/tags/%d", ts.URL, entry.ID, result.Tag.ID), nil)
			if err != nil {
				t.Errorf("request %d: build request: %v", idx, err)
				return
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("request %d: %v", idx, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()
			statuses[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, s := range statuses {
		if s != http.StatusNoContent {
			t.Fatalf("request %d status = %d, want 204", i, s)
		}
	}
}

// TestTags_Gated401NoCookie proves both tag routes answer 401, not 403,
// without a session -- gate.Authenticate runs before
// gate.RequireCSRFHeader (server.go's gated-group construction).
func TestTags_Gated401NoCookie(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{passphrase: settingsTestPassphrase})

	resp, err := http.Post(ts.URL+"/watchlist/1/tags", "application/json", strings.NewReader(`{"name":"rap"}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST status = %d, want 401", resp.StatusCode)
	}

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1/tags/1", nil)
	if err != nil {
		t.Fatalf("build DELETE: %v", err)
	}
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("DELETE status = %d, want 401", resp2.StatusCode)
	}
}

// TestTags_GatedForbiddenWithoutCSRFHeader proves both tag routes answer
// 403 and never reach the store without X-Requested-With, even with a
// valid session cookie.
func TestTags_GatedForbiddenWithoutCSRFHeader(t *testing.T) {
	var attachCalls, detachCalls int32
	ts := newTagsServer(t, tagsServerOpts{passphrase: settingsTestPassphrase, store: fakeTagStore{attachCalls: &attachCalls, detachCalls: &detachCalls}})
	cookie := loginForSettings(t, ts)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/watchlist/1/tags", strings.NewReader(`{"name":"rap"}`))
	if err != nil {
		t.Fatalf("build POST: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("POST status = %d, want 403", resp.StatusCode)
	}

	req2, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1/tags/1", nil)
	if err != nil {
		t.Fatalf("build DELETE: %v", err)
	}
	req2.AddCookie(cookie)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE status = %d, want 403", resp2.StatusCode)
	}

	if got := atomic.LoadInt32(&attachCalls); got != 0 {
		t.Fatalf("store.Attach called %d times, want 0", got)
	}
	if got := atomic.LoadInt32(&detachCalls); got != 0 {
		t.Fatalf("store.Detach called %d times, want 0", got)
	}
}
