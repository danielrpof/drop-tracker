package httpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

var _ httpserver.WatchlistBulkStore = (*watchlist.Service)(nil)

// bulkDeleteWire is the JSON shape of a bulk-delete outcome.
type bulkDeleteWire struct {
	Removed     []int64 `json:"removed"`
	AlreadyGone []int64 `json:"already_gone"`
}

// Tracer: three selected entries deleted over HTTP in one statement against
// a real Postgres, including an id that is already gone.
func TestBulk_DeleteEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "bulk_http_test")
	ctx := t.Context()

	svc := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, svc, stubEventsStore{}, nil, discardLogger(), httpserver.WithWatchlistBulk(svc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	var ids []int64
	for _, suffix := range []string{"-a", "-b", "-c"} {
		e, err := svc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + suffix, Name: "Bulk Artist" + suffix})
		if err != nil {
			t.Fatalf("seed entry %s: %v", suffix, err)
		}
		ids = append(ids, e.ID)
	}
	a, b, c := ids[0], ids[1], ids[2]

	body, err := json.Marshal(map[string]any{"ids": []int64{a, b, 999999999}})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	resp, err := http.Post(ts.URL+"/watchlist/bulk/delete", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("POST /watchlist/bulk/delete: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got bulkDeleteWire
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := []int64{a, b}; !slices.Equal(got.Removed, want) {
		t.Fatalf("removed = %v, want %v", got.Removed, want)
	}
	if want := []int64{999999999}; !slices.Equal(got.AlreadyGone, want) {
		t.Fatalf("already_gone = %v, want %v", got.AlreadyGone, want)
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var entries []struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != c {
		t.Fatalf("remaining entries = %+v, want only id %d", entries, c)
	}
}

// fakeBulkStore is a func-field double for httpserver.WatchlistBulkStore;
// non-nil removeCalls counts calls and lastIDs records the latest ids.
type fakeBulkStore struct {
	removeFunc  func(ctx context.Context, ids []int64) (watchlist.BulkRemoveResult, error)
	removeCalls *int32
	lastIDs     *[]int64

	updateFunc  func(ctx context.Context, ids []int64, d watchlist.PreferencesDelta) (watchlist.BulkUpdateResult, error)
	updateCalls *int32
}

func (f fakeBulkStore) BulkUpdatePreferences(ctx context.Context, ids []int64, d watchlist.PreferencesDelta) (watchlist.BulkUpdateResult, error) {
	if f.updateCalls != nil {
		atomic.AddInt32(f.updateCalls, 1)
	}
	if f.updateFunc != nil {
		return f.updateFunc(ctx, ids, d)
	}
	return watchlist.BulkUpdateResult{Applied: ids, Gone: []int64{}}, nil
}

func (f fakeBulkStore) BulkRemove(ctx context.Context, ids []int64) (watchlist.BulkRemoveResult, error) {
	if f.removeCalls != nil {
		atomic.AddInt32(f.removeCalls, 1)
	}
	if f.lastIDs != nil {
		*f.lastIDs = slices.Clone(ids)
	}
	if f.removeFunc != nil {
		return f.removeFunc(ctx, ids)
	}
	return watchlist.BulkRemoveResult{Removed: ids, AlreadyGone: []int64{}}, nil
}

var _ httpserver.WatchlistBulkStore = fakeBulkStore{}

// bulkServerOpts/newBulkServer mirror newTagsServer: gated or inert, a fake
// bulk store, or WithWatchlistBulk omitted entirely.
type bulkServerOpts struct {
	passphrase string
	store      httpserver.WatchlistBulkStore
	watchlist  watchlist.Store
	omitBulk   bool
}

func newBulkServer(t *testing.T, o bulkServerOpts) *httptest.Server {
	t.Helper()
	opts := []httpserver.Option{}
	if o.passphrase != "" {
		opts = append(opts, httpserver.WithAuthGate(o.passphrase, false, nil))
	}
	if !o.omitBulk {
		store := o.store
		if store == nil {
			store = fakeBulkStore{}
		}
		opts = append(opts, httpserver.WithWatchlistBulk(store))
	}
	wl := o.watchlist
	if wl == nil {
		wl = stubStore{}
	}
	srv := httpserver.New(noopPinger{}, wl, stubEventsStore{}, nil, discardLogger(), opts...)
	t.Cleanup(srv.Close)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts
}

func postBulkDelete(t *testing.T, ts *httptest.Server, body string) (*http.Response, []byte) {
	t.Helper()
	resp, err := http.Post(ts.URL+"/watchlist/bulk/delete", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist/bulk/delete: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, raw
}

func TestBulk_Gated401NoCookie(t *testing.T) {
	var calls int32
	ts := newBulkServer(t, bulkServerOpts{passphrase: settingsTestPassphrase, store: fakeBulkStore{removeCalls: &calls}})

	resp, _ := postBulkDelete(t, ts, `{"ids":[1]}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store called %d times, want 0", got)
	}
}

func TestBulk_GatedForbiddenWithoutCSRFHeader(t *testing.T) {
	var calls int32
	ts := newBulkServer(t, bulkServerOpts{passphrase: settingsTestPassphrase, store: fakeBulkStore{removeCalls: &calls}})
	cookie := loginForSettings(t, ts)

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/watchlist/bulk/delete", strings.NewReader(`{"ids":[1]}`))
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
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Fatalf("store called %d times, want 0", got)
	}
}

func TestBulk_UnavailableWithoutOption(t *testing.T) {
	ts := newBulkServer(t, bulkServerOpts{omitBulk: true})

	resp, raw := postBulkDelete(t, ts, `{"ids":[1]}`)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	var eb errorBody
	if err := json.Unmarshal(raw, &eb); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if eb.Error != "watchlist bulk not available" {
		t.Fatalf("error = %q, want %q", eb.Error, "watchlist bulk not available")
	}
}

// The literal bulk path must reach the bulk handler, never /watchlist/{id}.
func TestBulk_DeleteReachesBulkHandler(t *testing.T) {
	var calls int32
	wl := stubStore{removeFunc: func(context.Context, int64) error {
		t.Errorf("single-entry Remove reached by the bulk path")
		return nil
	}}
	ts := newBulkServer(t, bulkServerOpts{store: fakeBulkStore{removeCalls: &calls}, watchlist: wl})

	resp, raw := postBulkDelete(t, ts, `{"ids":[4]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("bulk store called %d times, want 1", got)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := keys["removed"]; !ok {
		t.Fatalf("body %s lacks the removed key", raw)
	}
}

func TestBulk_DeleteIDValidation(t *testing.T) {
	idsBody := func(n int) string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprint(i + 1)
		}
		return `{"ids":[` + strings.Join(ids, ",") + `]}`
	}

	rejected := []struct {
		name string
		body string
		code string
		max  int
	}{
		{"empty object", `{}`, "bulk_ids_required", 0},
		{"null ids", `{"ids":null}`, "bulk_ids_required", 0},
		{"empty ids", `{"ids":[]}`, "bulk_ids_required", 0},
		{"zero id", `{"ids":[0]}`, "bulk_invalid_id", 0},
		{"negative id", `{"ids":[-3]}`, "bulk_invalid_id", 0},
		{"fractional id", `{"ids":[1.5]}`, "", 0},
		{"string id", `{"ids":["1"]}`, "", 0},
		{"out of range id", `{"ids":[9223372036854775808]}`, "", 0},
		{"unknown field", `{"ids":[1],"note":"x"}`, "", 0},
		{"concatenated objects", `{"ids":[1]}{"ids":[2]}`, "", 0},
		{"body over 64 KiB", `{"ids":[1]` + strings.Repeat(" ", 70000) + `}`, "", 0},
		{"1001 ids", idsBody(1001), "bulk_too_many_ids", 1000},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			var calls int32
			ts := newBulkServer(t, bulkServerOpts{store: fakeBulkStore{removeCalls: &calls}})

			resp, raw := postBulkDelete(t, ts, tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", resp.StatusCode, raw)
			}
			var got struct {
				Error string `json:"error"`
				Code  string `json:"code"`
				Max   int    `json:"max"`
			}
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if tc.code == "" {
				if got.Error != "invalid request body" {
					t.Fatalf("error = %q, want %q", got.Error, "invalid request body")
				}
			} else if got.Code != tc.code {
				t.Fatalf("code = %q, want %q", got.Code, tc.code)
			}
			if got.Max != tc.max {
				t.Fatalf("max = %d, want %d", got.Max, tc.max)
			}
			if n := atomic.LoadInt32(&calls); n != 0 {
				t.Fatalf("store called %d times on a rejected body, want 0", n)
			}
		})
	}

	t.Run("exactly 1000 ids reach the store once", func(t *testing.T) {
		var calls int32
		var last []int64
		ts := newBulkServer(t, bulkServerOpts{store: fakeBulkStore{removeCalls: &calls, lastIDs: &last}})

		resp, raw := postBulkDelete(t, ts, idsBody(1000))
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %.200s)", resp.StatusCode, raw)
		}
		if n := atomic.LoadInt32(&calls); n != 1 || len(last) != 1000 {
			t.Fatalf("store calls = %d with %d ids, want 1 with 1000", n, len(last))
		}
	})

	t.Run("duplicates collapse to first occurrence", func(t *testing.T) {
		var calls int32
		var last []int64
		ts := newBulkServer(t, bulkServerOpts{store: fakeBulkStore{removeCalls: &calls, lastIDs: &last}})

		resp, _ := postBulkDelete(t, ts, `{"ids":[5,5,7]}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if want := []int64{5, 7}; !slices.Equal(last, want) {
			t.Fatalf("store ids = %v, want %v", last, want)
		}
	})

	t.Run("nil result slices serialize as empty arrays", func(t *testing.T) {
		store := fakeBulkStore{removeFunc: func(context.Context, []int64) (watchlist.BulkRemoveResult, error) {
			return watchlist.BulkRemoveResult{}, nil
		}}
		ts := newBulkServer(t, bulkServerOpts{store: store})

		resp, raw := postBulkDelete(t, ts, `{"ids":[1]}`)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		if got, want := strings.TrimSpace(string(raw)), `{"removed":[],"already_gone":[]}`; got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
	})
}

func TestBulk_DeleteStoreErrorIs500(t *testing.T) {
	store := fakeBulkStore{removeFunc: func(context.Context, []int64) (watchlist.BulkRemoveResult, error) {
		return watchlist.BulkRemoveResult{}, errors.New("pq: connection refused to 10.0.0.9")
	}}
	ts := newBulkServer(t, bulkServerOpts{store: store})

	resp, raw := postBulkDelete(t, ts, `{"ids":[1]}`)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
	var eb errorBody
	if err := json.Unmarshal(raw, &eb); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if eb.Error != "internal error" {
		t.Fatalf("error = %q, want %q", eb.Error, "internal error")
	}
	if strings.Contains(string(raw), "connection refused") {
		t.Fatalf("body leaks the store error: %s", raw)
	}
}

// bulkOutcomeWire is the JSON shape of a bulk preferences outcome.
type bulkOutcomeWire struct {
	Applied []int64 `json:"applied"`
	Skipped []struct {
		ID     int64  `json:"id"`
		Reason string `json:"reason"`
	} `json:"skipped"`
}

func patchBulkPreferences(t *testing.T, ts *httptest.Server, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/watchlist/bulk/preferences", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build PATCH: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /watchlist/bulk/preferences: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, raw
}

// Tracer: a per-type delta over a mixed selection keeps each artist's own
// untouched types, including a mute axis sent without unset (nil-slice guard).
func TestBulk_PreferencesEndToEnd(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "bulk_prefs_http_test")
	ctx := t.Context()

	svc := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, svc, stubEventsStore{}, nil, discardLogger(), httpserver.WithWatchlistBulk(svc))
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	var ids []int64
	for _, suffix := range []string{"-a", "-b", "-c"} {
		e, err := svc.Add(ctx, watchlist.AddParams{MBID: testMBID(t) + suffix, Name: "Prefs Artist" + suffix})
		if err != nil {
			t.Fatalf("seed entry %s: %v", suffix, err)
		}
		ids = append(ids, e.ID)
	}
	a, b, c := ids[0], ids[1], ids[2]

	rel, mute := []string{"single"}, []string{"new_release"}
	if _, err := svc.UpdatePreferences(ctx, b, watchlist.PreferencesParams{ReleaseTypes: &rel, MutedEventTypes: &mute}); err != nil {
		t.Fatalf("seed B preferences: %v", err)
	}

	body := fmt.Sprintf(`{"ids":[%d,%d,999999999],"release_types":{"set":["ep"],"unset":["deluxe"]},"muted_event_types":{"set":["guest_feature"]}}`, a, b)
	resp, raw := patchBulkPreferences(t, ts, body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, raw)
	}
	var got bulkOutcomeWire
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if want := []int64{a, b}; !slices.Equal(got.Applied, want) {
		t.Fatalf("applied = %v, want %v", got.Applied, want)
	}
	if len(got.Skipped) != 1 || got.Skipped[0].ID != 999999999 || got.Skipped[0].Reason != "gone" {
		t.Fatalf("skipped = %+v, want one gone entry for 999999999", got.Skipped)
	}

	listResp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer func() { _ = listResp.Body.Close() }()
	var entries []struct {
		ID              int64    `json:"id"`
		ReleaseTypes    []string `json:"release_types"`
		MutedEventTypes []string `json:"muted_event_types"`
	}
	if err := json.NewDecoder(listResp.Body).Decode(&entries); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	type prefs struct{ release, mute []string }
	want := map[int64]prefs{
		a: {[]string{"album", "single", "ep"}, []string{"guest_feature"}},
		b: {[]string{"single", "ep"}, []string{"new_release", "guest_feature"}},
		c: {[]string{"album", "single", "ep", "deluxe"}, []string{}},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries = %d, want %d", len(entries), len(want))
	}
	for _, e := range entries {
		w := want[e.ID]
		if !slices.Equal(e.ReleaseTypes, w.release) || !slices.Equal(e.MutedEventTypes, w.mute) {
			t.Errorf("entry %d = release %v mute %v, want release %v mute %v", e.ID, e.ReleaseTypes, e.MutedEventTypes, w.release, w.mute)
		}
	}
}
