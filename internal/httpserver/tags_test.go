package httpserver_test

// This is the tracer: one entry point (attach) wired through every backend
// layer against a real Postgres, proving the whole chain -- normalize,
// get-or-create, link, and the single-query GET /watchlist enrichment.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
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

	attachURL := ts.URL + "/watchlist/" + strconv.FormatInt(tagged.ID, 10) + "/tags"

	tooLong := `{"name":"` + strings.Repeat("a", tags.MaxNameRunes+1) + `"}`
	longResp, err := http.Post(attachURL, "application/json", strings.NewReader(tooLong))
	if err != nil {
		t.Fatalf("POST too-long name: %v", err)
	}
	defer func() { _ = longResp.Body.Close() }()
	if longResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("too-long status = %d, want 400", longResp.StatusCode)
	}
	var longBody struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(longResp.Body).Decode(&longBody); err != nil {
		t.Fatalf("decode too-long body: %v", err)
	}
	if longBody.Code != "tag_name_too_long" {
		t.Fatalf("too-long code = %q, want tag_name_too_long", longBody.Code)
	}

	emoji := `{"name":"` + strings.Repeat("🎵", tags.MaxNameRunes) + `"}`
	emojiResp, err := http.Post(attachURL, "application/json", strings.NewReader(emoji))
	if err != nil {
		t.Fatalf("POST emoji name: %v", err)
	}
	defer func() { _ = emojiResp.Body.Close() }()
	if emojiResp.StatusCode != http.StatusCreated {
		t.Fatalf("32-emoji status = %d, want 201", emojiResp.StatusCode)
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

// --- Task 2: Rename (409 collision), Delete ---

func TestTags_Rename_Success200(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{renameFunc: func(_ context.Context, id int64, name string) (tags.Tag, error) {
		return tags.Tag{ID: id, Name: name}, nil
	}}})

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/tags/7", strings.NewReader(`{"name":"hip hop"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got tagWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 7 || got.Name != "hip hop" {
		t.Fatalf("body = %+v, want {7 hip hop}", got)
	}
}

func TestTags_Rename_CollisionReturns409WithBody(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{renameFunc: func(context.Context, int64, string) (tags.Tag, error) {
		return tags.Tag{}, &tags.CollisionError{Target: tags.Tag{ID: 3, Name: "trap"}, CarrierCountAfterMerge: 3}
	}}})

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/tags/9", strings.NewReader(`{"name":"TRAP"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	var got struct {
		Error                  string  `json:"error"`
		Target                 tagWire `json:"target"`
		CarrierCountAfterMerge int64   `json:"carrier_count_after_merge"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Error != "tag name already exists" {
		t.Fatalf("error = %q, want %q", got.Error, "tag name already exists")
	}
	if got.Target.ID != 3 || got.Target.Name != "trap" {
		t.Fatalf("target = %+v, want {3 trap}", got.Target)
	}
	if got.CarrierCountAfterMerge != 3 {
		t.Fatalf("carrier_count_after_merge = %d, want 3", got.CarrierCountAfterMerge)
	}
}

func TestTags_Rename_NotFoundReturns404(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{renameFunc: func(context.Context, int64, string) (tags.Tag, error) {
		return tags.Tag{}, tags.ErrTagNotFound
	}}})

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/tags/999", strings.NewReader(`{"name":"whatever"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestTags_Rename_NonNumericIDReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/tags/abc", strings.NewReader(`{"name":"x"}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Rename_BadJSONReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/tags/1", strings.NewReader(`{"name":`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Delete_Success200WithCarrierCount(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{deleteFunc: func(context.Context, int64) (int64, error) {
		return 2, nil
	}}})

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/tags/5", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		CarrierCount int64 `json:"carrier_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CarrierCount != 2 {
		t.Fatalf("carrier_count = %d, want 2", got.CarrierCount)
	}
}

func TestTags_Delete_NotFoundReturns404(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{deleteFunc: func(context.Context, int64) (int64, error) {
		return 0, tags.ErrTagNotFound
	}}})

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/tags/999", nil)
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

func TestTags_Delete_NonNumericIDReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/tags/abc", nil)
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

// TestTags_RenameEndToEnd_AppliesToWatchlist proves a real rename against
// Postgres applies everywhere: PATCH /tags/{id} returns 200 and GET
// /watchlist shows the new name on every carrier.
func TestTags_RenameEndToEnd_AppliesToWatchlist(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_rename_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	entry, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-rename", Name: "Rename Artist"})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	attached, err := tagsSvc.Attach(ctx, entry.ID, "hiphop")
	if err != nil {
		t.Fatalf("seed attach: %v", err)
	}

	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/tags/%d", ts.URL, attached.Tag.ID), strings.NewReader(`{"name":"hip hop"}`))
	if err != nil {
		t.Fatalf("build PATCH: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200", resp.StatusCode)
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var entries []watchlistTagsEntryBody
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode watchlist: %v", err)
	}
	var found *watchlistTagsEntryBody
	for i := range entries {
		if entries[i].ID == entry.ID {
			found = &entries[i]
		}
	}
	if found == nil {
		t.Fatal("entry missing from GET /watchlist")
	}
	if len(found.Tags) != 1 || found.Tags[0].Name != "hip hop" {
		t.Fatalf("entry tags = %+v, want [{%d hip hop}]", found.Tags, attached.Tag.ID)
	}
}

// TestTags_DeleteEndToEnd_LeavesWatchlistUntouched proves TAG-06/D-11/SC4
// against real Postgres: DELETE reports the watched count and every
// watchlist row's preferences and note stay byte-identical.
func TestTags_DeleteEndToEnd_LeavesWatchlistUntouched(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_delete_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	entry, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-delete", Name: "Delete Artist"})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	attached, err := tagsSvc.Attach(ctx, entry.ID, "delete-me-http")
	if err != nil {
		t.Fatalf("seed attach: %v", err)
	}

	var beforeReleaseTypes, beforeMutedTypes []string
	var beforeNote *string
	if err := pool.QueryRow(ctx, "SELECT release_types, muted_event_types, note FROM watchlist WHERE id = $1", entry.ID).
		Scan(&beforeReleaseTypes, &beforeMutedTypes, &beforeNote); err != nil {
		t.Fatalf("read watchlist row before delete: %v", err)
	}

	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/tags/%d", ts.URL, attached.Tag.ID), nil)
	if err != nil {
		t.Fatalf("build DELETE: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		CarrierCount int64 `json:"carrier_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.CarrierCount != 1 {
		t.Fatalf("carrier_count = %d, want 1", got.CarrierCount)
	}

	var afterReleaseTypes, afterMutedTypes []string
	var afterNote *string
	if err := pool.QueryRow(ctx, "SELECT release_types, muted_event_types, note FROM watchlist WHERE id = $1", entry.ID).
		Scan(&afterReleaseTypes, &afterMutedTypes, &afterNote); err != nil {
		t.Fatalf("read watchlist row after delete: %v", err)
	}
	if !reflect.DeepEqual(beforeReleaseTypes, afterReleaseTypes) {
		t.Fatalf("release_types changed: before=%v after=%v", beforeReleaseTypes, afterReleaseTypes)
	}
	if !reflect.DeepEqual(beforeMutedTypes, afterMutedTypes) {
		t.Fatalf("muted_event_types changed: before=%v after=%v", beforeMutedTypes, afterMutedTypes)
	}
	if (beforeNote == nil) != (afterNote == nil) {
		t.Fatalf("note nil-ness changed: before=%v after=%v", beforeNote, afterNote)
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
	renameFunc  func(ctx context.Context, id int64, name string) (tags.Tag, error)
	deleteFunc  func(ctx context.Context, id int64) (int64, error)
	mergeFunc   func(ctx context.Context, sourceID, targetID int64) (tags.Summary, error)
	attachCalls *int32
	detachCalls *int32
	renameCalls *int32
	deleteCalls *int32
	mergeCalls  *int32
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

func (f fakeTagStore) Rename(ctx context.Context, id int64, name string) (tags.Tag, error) {
	if f.renameCalls != nil {
		atomic.AddInt32(f.renameCalls, 1)
	}
	if f.renameFunc != nil {
		return f.renameFunc(ctx, id, name)
	}
	return tags.Tag{ID: id, Name: name}, nil
}

func (f fakeTagStore) Delete(ctx context.Context, id int64) (int64, error) {
	if f.deleteCalls != nil {
		atomic.AddInt32(f.deleteCalls, 1)
	}
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, id)
	}
	return 0, nil
}

func (f fakeTagStore) Merge(ctx context.Context, sourceID, targetID int64) (tags.Summary, error) {
	if f.mergeCalls != nil {
		atomic.AddInt32(f.mergeCalls, 1)
	}
	if f.mergeFunc != nil {
		return f.mergeFunc(ctx, sourceID, targetID)
	}
	return tags.Summary{ID: targetID, Name: "merged", CarrierCount: 0}, nil
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

// --- 24-02 Task 3: Merge, and gate/CSRF coverage for all four vocabulary routes ---

func TestTags_Merge_Success200(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{mergeFunc: func(_ context.Context, sourceID, targetID int64) (tags.Summary, error) {
		return tags.Summary{ID: targetID, Name: "trap", CarrierCount: 3}, nil
	}}})

	resp, err := http.Post(ts.URL+"/tags/2/merge", "application/json", strings.NewReader(`{"into":1}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got tagSummaryWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 1 || got.Name != "trap" || got.CarrierCount != 3 {
		t.Fatalf("body = %+v, want {1 trap 3}", got)
	}
}

func TestTags_Merge_SelfReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{mergeFunc: func(context.Context, int64, int64) (tags.Summary, error) {
		return tags.Summary{}, tags.ErrMergeIntoSelf
	}}})

	resp, err := http.Post(ts.URL+"/tags/1/merge", "application/json", strings.NewReader(`{"into":1}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Merge_UnknownTargetReturns404(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{mergeFunc: func(context.Context, int64, int64) (tags.Summary, error) {
		return tags.Summary{}, tags.ErrTagNotFound
	}}})

	resp, err := http.Post(ts.URL+"/tags/1/merge", "application/json", strings.NewReader(`{"into":999}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestTags_Merge_ZeroIntoReturns400(t *testing.T) {
	var calls int32
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{mergeCalls: &calls}})

	resp, err := http.Post(ts.URL+"/tags/1/merge", "application/json", strings.NewReader(`{"into":0}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store.Merge called %d times, want 0", got)
	}
}

func TestTags_Merge_MissingIntoKeyReturns400(t *testing.T) {
	var calls int32
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{mergeCalls: &calls}})

	resp, err := http.Post(ts.URL+"/tags/1/merge", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store.Merge called %d times, want 0", got)
	}
}

func TestTags_Merge_BadJSONReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	resp, err := http.Post(ts.URL+"/tags/1/merge", "application/json", strings.NewReader(`{"into":`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTags_Merge_NonNumericIDReturns400(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{})

	resp, err := http.Post(ts.URL+"/tags/abc/merge", "application/json", strings.NewReader(`{"into":1}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

// TestTags_MergeEndToEnd proves a real merge against Postgres: the target's
// carrier count unions both tags' watched artists, the source tag is gone,
// and a 10-tag artist carrying only the source still merges (ADR test 3, at
// the HTTP layer).
func TestTags_MergeEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_http_merge_test")
	ctx := context.Background()

	watchlistSvc := watchlist.NewService(sqlc.New(pool))
	tagsSvc := tags.NewService(pool)
	srv := httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	a, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-merge-a", Name: "Merge A"})
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	b, err := watchlistSvc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + "-merge-b", Name: "Merge B"})
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}

	trap, err := tagsSvc.Attach(ctx, a.ID, "http-trap")
	if err != nil {
		t.Fatalf("attach trap: %v", err)
	}
	rap, err := tagsSvc.Attach(ctx, b.ID, "http-rap")
	if err != nil {
		t.Fatalf("attach rap: %v", err)
	}

	resp, err := http.Post(fmt.Sprintf("%s/tags/%d/merge", ts.URL, rap.Tag.ID), "application/json", strings.NewReader(fmt.Sprintf(`{"into":%d}`, trap.Tag.ID)))
	if err != nil {
		t.Fatalf("POST merge: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got tagSummaryWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != trap.Tag.ID || got.Name != "http-trap" || got.CarrierCount != 2 {
		t.Fatalf("merge summary = %+v, want {%d http-trap 2}", got, trap.Tag.ID)
	}

	listResp, err := http.Get(ts.URL + "/tags")
	if err != nil {
		t.Fatalf("GET /tags: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var vocab []tagSummaryWire
	if err := json.NewDecoder(listResp.Body).Decode(&vocab); err != nil {
		t.Fatalf("decode vocab: %v", err)
	}
	for _, s := range vocab {
		if s.Name == "http-rap" {
			t.Fatalf("http-rap still present in vocabulary after merge: %+v", s)
		}
	}
}

// TestTags_Vocabulary_Gated401NoCookie proves all four vocabulary routes
// answer 401, not 403, without a session.
func TestTags_Vocabulary_Gated401NoCookie(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{passphrase: settingsTestPassphrase})

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"list", http.MethodGet, "/tags", ""},
		{"rename", http.MethodPatch, "/tags/1", `{"name":"x"}`},
		{"merge", http.MethodPost, "/tags/1/merge", `{"into":2}`},
		{"delete", http.MethodDelete, "/tags/1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Fatalf("%s %s status = %d, want 401", tc.method, tc.path, resp.StatusCode)
			}
		})
	}
}

// TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader proves the three
// vocabulary write routes answer 403 and never reach the store without
// X-Requested-With, even with a valid session cookie. GET /tags is a read
// verb, so it is not part of this check (the CSRF-header requirement is a
// no-op for it, mirroring TestTags_Gated401NoCookie's own /status
// precedent).
func TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader(t *testing.T) {
	var renameCalls, mergeCalls, deleteCalls int32
	store := fakeTagStore{renameCalls: &renameCalls, mergeCalls: &mergeCalls, deleteCalls: &deleteCalls}
	ts := newTagsServer(t, tagsServerOpts{passphrase: settingsTestPassphrase, store: store})
	cookie := loginForSettings(t, ts)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"rename", http.MethodPatch, "/tags/1", `{"name":"x"}`},
		{"merge", http.MethodPost, "/tags/1/merge", `{"into":2}`},
		{"delete", http.MethodDelete, "/tags/1", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, ts.URL+tc.path, body)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			req.AddCookie(cookie)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", tc.method, tc.path, err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("%s %s status = %d, want 403", tc.method, tc.path, resp.StatusCode)
			}
		})
	}

	if got := atomic.LoadInt32(&renameCalls); got != 0 {
		t.Fatalf("store.Rename called %d times, want 0", got)
	}
	if got := atomic.LoadInt32(&mergeCalls); got != 0 {
		t.Fatalf("store.Merge called %d times, want 0", got)
	}
	if got := atomic.LoadInt32(&deleteCalls); got != 0 {
		t.Fatalf("store.Delete called %d times, want 0", got)
	}
}

// TestTags_ErrorContract pins status + code + body text for every tags
// sentinel on every route that can return it, and the uncoded 500 fallback.
func TestTags_ErrorContract(t *testing.T) {
	type route struct {
		name   string
		method string
		path   string
		body   string
		store  func(err error) fakeTagStore
	}
	routes := []route{
		{"attach", http.MethodPost, "/watchlist/1/tags", `{"name":"rap"}`, func(err error) fakeTagStore {
			return fakeTagStore{attachFunc: func(context.Context, int64, string) (tags.AttachResult, error) { return tags.AttachResult{}, err }}
		}},
		{"rename", http.MethodPatch, "/tags/1", `{"name":"rap"}`, func(err error) fakeTagStore {
			return fakeTagStore{renameFunc: func(context.Context, int64, string) (tags.Tag, error) { return tags.Tag{}, err }}
		}},
		{"merge", http.MethodPost, "/tags/1/merge", `{"into":2}`, func(err error) fakeTagStore {
			return fakeTagStore{mergeFunc: func(context.Context, int64, int64) (tags.Summary, error) { return tags.Summary{}, err }}
		}},
		{"delete", http.MethodDelete, "/tags/1", "", func(err error) fakeTagStore {
			return fakeTagStore{deleteFunc: func(context.Context, int64) (int64, error) { return 0, err }}
		}},
		{"detach", http.MethodDelete, "/watchlist/1/tags/2", "", func(err error) fakeTagStore {
			return fakeTagStore{detachFunc: func(context.Context, int64, int64) error { return err }}
		}},
	}
	cases := []struct {
		routes     []string
		err        error
		wantStatus int
		wantCode   string
	}{
		{[]string{"attach", "rename"}, tags.ErrNameRequired, http.StatusBadRequest, "tag_name_required"},
		{[]string{"attach", "rename"}, tags.ErrNameTooLong, http.StatusBadRequest, "tag_name_too_long"},
		{[]string{"attach", "rename"}, tags.ErrNameInvalid, http.StatusBadRequest, "tag_name_invalid"},
		{[]string{"attach"}, tags.ErrTagCapReached, http.StatusConflict, "tag_cap_reached"},
		{[]string{"attach", "detach"}, tags.ErrEntryNotFound, http.StatusNotFound, "watchlist_entry_not_found"},
		{[]string{"rename", "merge", "delete"}, tags.ErrTagNotFound, http.StatusNotFound, "tag_not_found"},
		{[]string{"merge"}, tags.ErrMergeIntoSelf, http.StatusBadRequest, "tag_merge_into_self"},
	}
	for _, tc := range cases {
		for _, rt := range routes {
			if !slices.Contains(tc.routes, rt.name) {
				continue
			}
			t.Run(rt.name+"/"+tc.wantCode, func(t *testing.T) {
				ts := newTagsServer(t, tagsServerOpts{store: rt.store(tc.err)})
				status, body := doTagsRequest(t, ts, rt.method, rt.path, rt.body)
				if status != tc.wantStatus {
					t.Fatalf("status = %d, want %d", status, tc.wantStatus)
				}
				if body["code"] != tc.wantCode || body["error"] != tc.err.Error() {
					t.Fatalf("body = %v, want code %q error %q", body, tc.wantCode, tc.err.Error())
				}
			})
		}
	}

	for _, rt := range routes {
		t.Run(rt.name+"/unknown error is uncoded 500", func(t *testing.T) {
			ts := newTagsServer(t, tagsServerOpts{store: rt.store(errors.New("boom"))})
			status, body := doTagsRequest(t, ts, rt.method, rt.path, rt.body)
			if status != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", status)
			}
			if _, ok := body["code"]; ok || body["error"] != "internal error" {
				t.Fatalf("body = %v, want only error \"internal error\"", body)
			}
		})
	}
}

func TestTags_Rename_CollisionBodyHasCode(t *testing.T) {
	ts := newTagsServer(t, tagsServerOpts{store: fakeTagStore{renameFunc: func(context.Context, int64, string) (tags.Tag, error) {
		return tags.Tag{}, &tags.CollisionError{Target: tags.Tag{ID: 3, Name: "trap"}, CarrierCountAfterMerge: 3}
	}}})
	status, body := doTagsRequest(t, ts, http.MethodPatch, "/tags/9", `{"name":"TRAP"}`)
	if status != http.StatusConflict || body["code"] != "tag_name_taken" || body["error"] != "tag name already exists" {
		t.Fatalf("status = %d body = %v, want 409 tag_name_taken with the fixed text", status, body)
	}
}

func doTagsRequest(t *testing.T, ts *httptest.Server, method, path, body string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(method, ts.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return resp.StatusCode, out
}
