package httpserver_test

// This file proves POST /watchlist end-to-end (WLST-02): the happy path
// against a real Postgres (row persisted, D-08 defaults applied), plus the
// unit-level validation and error-leak branches against a stub
// watchlist.Store -- mirroring health_test.go's stubPinger pattern.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// testMBID derives a short, unique-per-test mbid from t.Name() so tests
// sharing one database never collide, without hardcoding a literal. Mirrors
// internal/watchlist/service_test.go's helper of the same name -- kept
// file-local here since it is unexported and this is a different package.
func testMBID(t *testing.T) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name()))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

// stubStore is a file-local double for watchlist.Store. Each method calls
// its func field when set and otherwise returns a zero value with a nil
// error, so a bare stubStore{} is safe to pass into the health and server
// tests that never touch watchlist routes.
type stubStore struct {
	addFunc    func(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error)
	listFunc   func(ctx context.Context) ([]watchlist.Entry, error)
	updateFunc func(ctx context.Context, id int64, p watchlist.PreferencesParams) (watchlist.Entry, error)
	removeFunc func(ctx context.Context, id int64) error
}

func (s stubStore) Add(ctx context.Context, p watchlist.AddParams) (watchlist.Entry, error) {
	if s.addFunc != nil {
		return s.addFunc(ctx, p)
	}
	return watchlist.Entry{}, nil
}

func (s stubStore) List(ctx context.Context) ([]watchlist.Entry, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx)
	}
	return nil, nil
}

func (s stubStore) UpdatePreferences(ctx context.Context, id int64, p watchlist.PreferencesParams) (watchlist.Entry, error) {
	if s.updateFunc != nil {
		return s.updateFunc(ctx, id, p)
	}
	return watchlist.Entry{}, nil
}

func (s stubStore) Remove(ctx context.Context, id int64) error {
	if s.removeFunc != nil {
		return s.removeFunc(ctx, id)
	}
	return nil
}

var _ watchlist.Store = stubStore{}

// errorBody mirrors the D-13 {"error": "..."} response contract by field
// name.
type errorBody struct {
	Error string `json:"error"`
}

// watchlistEntryBody mirrors the fields of watchlist.Entry this test
// asserts on, decoded by field name rather than raw string comparison.
type watchlistEntryBody struct {
	ID              int64    `json:"id"`
	ArtistID        int64    `json:"artist_id"`
	MBID            string   `json:"mbid"`
	Name            string   `json:"name"`
	ReleaseTypes    []string `json:"release_types"`
	MutedEventTypes []string `json:"muted_event_types"`
}

// noopPinger always reports the database as up -- these tests exercise the
// watchlist store branch, not the health branch, so the Pinger side just
// needs to be valid.
type noopPinger struct{}

func (noopPinger) Ping(context.Context) error { return nil }

var _ httpserver.Pinger = noopPinger{}

func TestWatchlist_AddEndToEnd(t *testing.T) {
	const mbid = "5b11f4ce-a62d-471e-81fc-a69a8278c7da"

	pool := testutil.NewTestPool(t)

	// This is the only test that writes this mbid, but NewTestPool does not
	// reset table contents between test runs (only schema). Delete any
	// artists row for it -- cascading to its watchlist row -- both before
	// and after this test, so a prior interrupted run or a rerun of this
	// same test never collides with the 409-on-duplicate constraint that
	// plan 02-02 wires error translation for.
	cleanup := func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	store := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, store, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := `{"mbid":"` + mbid + `","name":"Radiohead"}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var entry watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&entry); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if entry.ID == 0 {
		t.Fatal("id is zero, want a non-zero id")
	}
	if entry.ArtistID == 0 {
		t.Fatal("artist_id is zero, want a non-zero artist_id")
	}
	if entry.MBID != mbid {
		t.Fatalf("mbid = %q, want the posted mbid", entry.MBID)
	}
	if entry.Name != "Radiohead" {
		t.Fatalf("name = %q, want %q", entry.Name, "Radiohead")
	}
	wantReleaseTypes := []string{"album", "single", "ep", "deluxe"}
	if !reflect.DeepEqual(entry.ReleaseTypes, wantReleaseTypes) {
		t.Fatalf("release_types = %v, want %v (D-08 default)", entry.ReleaseTypes, wantReleaseTypes)
	}
	if len(entry.MutedEventTypes) != 0 {
		t.Fatalf("muted_event_types = %v, want empty (D-08 default)", entry.MutedEventTypes)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM watchlist WHERE artist_id = $1", entry.ArtistID).Scan(&count); err != nil {
		t.Fatalf("query watchlist row count: %v", err)
	}
	if count != 1 {
		t.Fatalf("watchlist row count = %d, want 1 (the row must really be in Postgres)", count)
	}
}

func TestWatchlist_Add_RejectsBlankFields(t *testing.T) {
	bodies := []string{
		`{"mbid":"","name":"x"}`,
		`{"name":"x"}`,
		`{"mbid":"x","name":"   "}`,
	}

	for _, body := range bodies {
		t.Run(body, func(t *testing.T) {
			called := false
			stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
				called = true
				return watchlist.Entry{}, nil
			}}
			srv := httpserver.New(noopPinger{}, stub, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatalf("POST /watchlist: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}

			var eb errorBody
			if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
				t.Fatalf("decode response body: %v", err)
			}
			if eb.Error == "" {
				t.Fatal("error message is empty, want a non-empty message")
			}
			if called {
				t.Fatal("addFunc was called for a body with a blank mbid/name")
			}
		})
	}
}

func TestWatchlist_Add_RejectsUnknownFields(t *testing.T) {
	called := false
	stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
		called = true
		return watchlist.Entry{}, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"mbid":"x","name":"y","id":99}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if called {
		t.Fatal("addFunc was called for an over-posted body (server-owned key \"id\" was accepted)")
	}
}

func TestWatchlist_Add_DuplicateReturns409(t *testing.T) {
	stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
		return watchlist.Entry{}, watchlist.ErrDuplicate
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"mbid":"x","name":"y"}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var eb errorBody
	if err := json.Unmarshal(data, &eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if eb.Error == "" {
		t.Fatal("error message is empty, want a non-empty message")
	}

	raw := string(data)
	for _, leak := range []string{"23505", "pgconn", "watchlist_artist_id_key"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("response body leaked %q: %s", leak, raw)
		}
	}
}

func TestWatchlist_Add_InvalidPreferenceValueReturns400(t *testing.T) {
	called := false
	stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
		called = true
		return watchlist.Entry{}, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"mbid":"x","name":"y","release_types":["mixtape"]}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var eb errorBody
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if !strings.Contains(eb.Error, "mixtape") {
		t.Fatalf("error message = %q, want it to contain %q", eb.Error, "mixtape")
	}
	if called {
		t.Fatal("addFunc was called for a body with an out-of-allow-list release type")
	}
}

func TestWatchlist_Add_RejectsOversizeBody(t *testing.T) {
	called := false
	stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
		called = true
		return watchlist.Entry{}, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := `{"mbid":"x","name":"` + strings.Repeat("a", 70000) + `"}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if called {
		t.Fatal("addFunc was called for an oversize body")
	}
}

func TestWatchlist_Add_RejectsOverlongFields(t *testing.T) {
	tests := []struct {
		name string
		mbid string
		aid  string
	}{
		{name: "overlong mbid", mbid: strings.Repeat("a", 37), aid: "y"},
		{name: "overlong name", mbid: "x", aid: strings.Repeat("a", 513)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
				called = true
				return watchlist.Entry{}, nil
			}}
			srv := httpserver.New(noopPinger{}, stub, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			reqBody, err := json.Marshal(map[string]string{"mbid": tt.mbid, "name": tt.aid})
			if err != nil {
				t.Fatalf("marshal request body: %v", err)
			}

			resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(string(reqBody)))
			if err != nil {
				t.Fatalf("POST /watchlist: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			if called {
				t.Fatal("addFunc was called for an overlong field")
			}
		})
	}
}

// The two tests below cover the empty-list contract for GET /watchlist
// (WLST-04): whether the store returns an explicit empty slice or a nil
// slice, the handler must always encode a bare `[]`, never `null`. The
// assertion is on raw response bytes, not a decoded value, since a decoded
// nil slice and a decoded empty slice are indistinguishable in Go.

func TestWatchlist_List_EmptyReturnsEmptyArray(t *testing.T) {
	stub := stubStore{listFunc: func(context.Context) ([]watchlist.Entry, error) {
		return []watchlist.Entry{}, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "[]" {
		t.Fatalf("body = %q, want %q", got, "[]")
	}
}

func TestWatchlist_List_NilSliceStillEncodesAsEmptyArray(t *testing.T) {
	stub := stubStore{listFunc: func(context.Context) ([]watchlist.Entry, error) {
		return nil, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "[]" {
		t.Fatalf("body = %q, want %q (a nil store slice must still encode as an empty array)", got, "[]")
	}
}

// TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404 mirrors
// TestHealth_Concurrent's goroutine/result-slice shape but against a real
// Postgres-backed store, proving T-02-15's row-lock-based concurrency
// guarantee end to end: two DELETE requests racing for the same watchlist id
// produce exactly one 204 and one 404, never a 500.
func TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	store := watchlist.NewService(sqlc.New(pool))
	entry, err := store.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Concurrent Delete Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	srv := httpserver.New(pool, store, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const n = 2
	statuses := make([]int, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()

			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/watchlist/%d", ts.URL, entry.ID), nil)
			if err != nil {
				t.Errorf("request %d: build request: %v", idx, err)
				return
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("request %d: %v", idx, err)
				return
			}
			defer resp.Body.Close()
			statuses[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	sort.Ints(statuses)
	want := []int{http.StatusNoContent, http.StatusNotFound}
	if !reflect.DeepEqual(statuses, want) {
		t.Fatalf("sorted statuses = %v, want %v", statuses, want)
	}
}

// TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive is the end-to-end
// analogue of internal/watchlist/service_test.go's
// TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost (G-02-2b,
// WR-02): the pre-fix behaviour was that a PATCH to one axis could silently
// revert a concurrently-applied PATCH to the other. Unlike the service-layer
// test, these two real HTTP requests race with no forced interleaving, so a
// single run cannot be guaranteed to catch a regression -- this loops 25
// times so an unlucky scheduling on any one iteration does not make a green
// run meaningless. The deterministic proof lives in the service-layer test
// above; this proves the same property survives the handler and the real
// network round trip.
func TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	store := watchlist.NewService(sqlc.New(pool))
	entry, err := store.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Concurrent Patch Axes Test"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	srv := httpserver.New(pool, store, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const iterations = 25
	for i := 0; i < iterations; i++ {
		if _, err := pool.Exec(ctx,
			"UPDATE watchlist SET release_types = ARRAY['album','single','ep','deluxe']::text[], muted_event_types = '{}'::text[] WHERE id = $1",
			entry.ID,
		); err != nil {
			t.Fatalf("iteration %d: reset row: %v", i, err)
		}

		var wg sync.WaitGroup
		var barrier sync.WaitGroup
		barrier.Add(1)
		statuses := make([]int, 2)

		wg.Add(2)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/watchlist/%d", ts.URL, entry.ID), strings.NewReader(`{"release_types":["single"]}`))
			if err != nil {
				t.Errorf("iteration %d: build release_types request: %v", i, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			barrier.Wait()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("iteration %d: release_types PATCH: %v", i, err)
				return
			}
			defer resp.Body.Close()
			statuses[0] = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/watchlist/%d", ts.URL, entry.ID), strings.NewReader(`{"muted_event_types":["deluxe_change"]}`))
			if err != nil {
				t.Errorf("iteration %d: build muted_event_types request: %v", i, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			barrier.Wait()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("iteration %d: muted_event_types PATCH: %v", i, err)
				return
			}
			defer resp.Body.Close()
			statuses[1] = resp.StatusCode
		}()

		barrier.Done()
		wg.Wait()

		if statuses[0] != http.StatusOK {
			t.Errorf("iteration %d: release_types PATCH status = %d, want %d", i, statuses[0], http.StatusOK)
		}
		if statuses[1] != http.StatusOK {
			t.Errorf("iteration %d: muted_event_types PATCH status = %d, want %d", i, statuses[1], http.StatusOK)
		}

		var releaseTypes, mutedEventTypes []string
		row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types FROM watchlist WHERE id = $1", entry.ID)
		if err := row.Scan(&releaseTypes, &mutedEventTypes); err != nil {
			t.Fatalf("iteration %d: query stored preferences: %v", i, err)
		}
		if len(releaseTypes) != 1 || releaseTypes[0] != "single" {
			t.Errorf("iteration %d: release_types = %v, want [single] (the muted_event_types PATCH must not have reverted it)", i, releaseTypes)
		}
		if len(mutedEventTypes) != 1 || mutedEventTypes[0] != "deluxe_change" {
			t.Errorf("iteration %d: muted_event_types = %v, want [deluxe_change] (the release_types PATCH must not have reverted it)", i, mutedEventTypes)
		}
	}
}

// TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500 is the end-to-end
// analogue of internal/watchlist/service_test.go's
// TestService_UpdatePreferences_RowDeletedMidWriteReturnsErrNotFound
// (G-02-2b, WR-02): the pre-fix behaviour was a 500 when a PATCH raced a
// DELETE for the same row. Like its sibling above, this cannot force the
// interleaving -- it races 25 real request pairs and asserts the invariant
// holds on every one; the deterministic proof lives in the service-layer
// test.
func TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	ctx := context.Background()
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	store := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, store, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const iterations = 25
	for i := 0; i < iterations; i++ {
		entry, err := store.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Concurrent Patch Delete Test"})
		if err != nil {
			t.Fatalf("iteration %d: Add: %v", i, err)
		}

		var wg sync.WaitGroup
		var barrier sync.WaitGroup
		barrier.Add(1)
		var patchStatus, deleteStatus int
		var patchBody []byte

		wg.Add(2)
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/watchlist/%d", ts.URL, entry.ID), strings.NewReader(`{"release_types":["album"]}`))
			if err != nil {
				t.Errorf("iteration %d: build PATCH request: %v", i, err)
				return
			}
			req.Header.Set("Content-Type", "application/json")
			barrier.Wait()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("iteration %d: PATCH: %v", i, err)
				return
			}
			defer resp.Body.Close()
			patchStatus = resp.StatusCode
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("iteration %d: read PATCH response body: %v", i, err)
				return
			}
			patchBody = body
		}()
		go func() {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/watchlist/%d", ts.URL, entry.ID), nil)
			if err != nil {
				t.Errorf("iteration %d: build DELETE request: %v", i, err)
				return
			}
			barrier.Wait()
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Errorf("iteration %d: DELETE: %v", i, err)
				return
			}
			defer resp.Body.Close()
			deleteStatus = resp.StatusCode
		}()

		barrier.Done()
		wg.Wait()

		if deleteStatus != http.StatusNoContent {
			t.Errorf("iteration %d: DELETE status = %d, want %d", i, deleteStatus, http.StatusNoContent)
		}

		if patchStatus != http.StatusOK && patchStatus != http.StatusNotFound {
			t.Errorf("iteration %d: PATCH status = %d, want %d or %d (never 500)", i, patchStatus, http.StatusOK, http.StatusNotFound)
		}

		if patchStatus == http.StatusNotFound {
			var eb errorBody
			if err := json.Unmarshal(patchBody, &eb); err != nil {
				t.Errorf("iteration %d: decode 404 body: %v (body: %s)", i, err, patchBody)
			} else {
				if eb.Error == "" {
					t.Errorf("iteration %d: 404 error message is empty, want a non-empty message", i)
				}
				lower := strings.ToLower(eb.Error)
				if strings.Contains(lower, "sqlstate") || strings.Contains(lower, "pgconn") || strings.Contains(lower, "pgx") || strings.Contains(eb.Error, "@") {
					t.Errorf("iteration %d: 404 error message %q looks like it leaks driver/DSN internals", i, eb.Error)
				}
			}
		}

		// Self-consistency: if the DELETE somehow did not land (e.g. it lost
		// the race to a PATCH that also failed), remove the row so the next
		// iteration's Add starts clean.
		if deleteStatus != http.StatusNoContent {
			if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", entry.ID); err != nil {
				t.Fatalf("iteration %d: self-consistency cleanup delete: %v", i, err)
			}
		}
	}
}

func TestWatchlist_Delete_MissingReturns404(t *testing.T) {
	stub := stubStore{removeFunc: func(context.Context, int64) error {
		return watchlist.ErrNotFound
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	var eb errorBody
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if eb.Error == "" {
		t.Fatal("error message is empty, want a non-empty message")
	}
}

func TestWatchlist_Delete_SuccessReturns204(t *testing.T) {
	stub := stubStore{removeFunc: func(context.Context, int64) error {
		return nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/watchlist/1", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("body length = %d, want 0 (204 must carry no payload)", len(data))
	}
}

func TestWatchlist_Delete_BadIDReturns400(t *testing.T) {
	paths := []string{
		"/watchlist/abc",
		"/watchlist/-1",
		"/watchlist/0",
		"/watchlist/9999999999999999999999",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			called := false
			stub := stubStore{removeFunc: func(context.Context, int64) error {
				called = true
				return nil
			}}
			srv := httpserver.New(noopPinger{}, stub, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			req, err := http.NewRequest(http.MethodDelete, ts.URL+p, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("DELETE %s: %v", p, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			if called {
				t.Fatal("removeFunc was called for a malformed id")
			}
		})
	}
}

// The five tests below cover PATCH /watchlist/{id} (WLST-05, WLST-06,
// T-02-11): 200 with the full updated entry, 400 naming an invalid
// preference value, 404 for a missing id, 400 for a malformed id (store
// never called), and 400 for an over-posted body (T-02-11's mass-assignment
// gate).

func TestWatchlist_Patch_Returns200WithUpdatedEntry(t *testing.T) {
	want := watchlist.Entry{
		ID:              1,
		ArtistID:        2,
		MBID:            "mbid-x",
		Name:            "Name X",
		ReleaseTypes:    []string{"album", "ep"},
		MutedEventTypes: []string{"deluxe_change"},
	}
	stub := stubStore{updateFunc: func(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
		return want, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"release_types":["album","ep"]}`
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/watchlist/1", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var got watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if got.ID != want.ID || got.ArtistID != want.ArtistID || got.MBID != want.MBID || got.Name != want.Name {
		t.Fatalf("decoded body = %+v, want id/artist_id/mbid/name matching %+v", got, want)
	}
	if !reflect.DeepEqual(got.ReleaseTypes, want.ReleaseTypes) {
		t.Fatalf("release_types = %v, want %v", got.ReleaseTypes, want.ReleaseTypes)
	}
	if !reflect.DeepEqual(got.MutedEventTypes, want.MutedEventTypes) {
		t.Fatalf("muted_event_types = %v, want %v", got.MutedEventTypes, want.MutedEventTypes)
	}
}

func TestWatchlist_Patch_InvalidValueReturns400(t *testing.T) {
	stub := stubStore{updateFunc: func(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
		return watchlist.Entry{}, fmt.Errorf("%w: %q", watchlist.ErrInvalidReleaseType, "mixtape")
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"release_types":["mixtape"]}`
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/watchlist/1", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}

	var eb errorBody
	if err := json.NewDecoder(resp.Body).Decode(&eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if !strings.Contains(eb.Error, "mixtape") {
		t.Fatalf("error message = %q, want it to contain %q", eb.Error, "mixtape")
	}
}

func TestWatchlist_Patch_MissingReturns404(t *testing.T) {
	stub := stubStore{updateFunc: func(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
		return watchlist.Entry{}, watchlist.ErrNotFound
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"release_types":["album"]}`
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/watchlist/1", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestWatchlist_Patch_BadIDReturns400(t *testing.T) {
	paths := []string{"/watchlist/abc", "/watchlist/0"}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			called := false
			stub := stubStore{updateFunc: func(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
				called = true
				return watchlist.Entry{}, nil
			}}
			srv := httpserver.New(noopPinger{}, stub, discardLogger())
			ts := httptest.NewServer(srv.Router())
			defer ts.Close()

			const body = `{"release_types":["album"]}`
			req, err := http.NewRequest(http.MethodPatch, ts.URL+p, strings.NewReader(body))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("PATCH %s: %v", p, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
			}
			if called {
				t.Fatal("updateFunc was called for a malformed id")
			}
		})
	}
}

func TestWatchlist_Patch_RejectsUnknownFields(t *testing.T) {
	called := false
	stub := stubStore{updateFunc: func(context.Context, int64, watchlist.PreferencesParams) (watchlist.Entry, error) {
		called = true
		return watchlist.Entry{}, nil
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"release_types":["album"],"artist_id":7}`
	req, err := http.NewRequest(http.MethodPatch, ts.URL+"/watchlist/1", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH /watchlist/1: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if called {
		t.Fatal("updateFunc was called for an over-posted body")
	}
}

// TestWatchlist_FullLifecycle walks the full D-11 four-route surface against
// one live server and real Postgres, demonstrating WLST-02 through WLST-06
// end to end: add, list, narrow release types, mute an event category,
// reject a duplicate add, remove, confirm removal, reject a repeat remove,
// and re-add successfully with a fresh id.
func TestWatchlist_FullLifecycle(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	store := watchlist.NewService(sqlc.New(pool))
	srv := httpserver.New(pool, store, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	body := `{"mbid":"` + mbid + `","name":"Lifecycle Artist"}`

	// POST a new artist -> 201.
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	var created watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode POST response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	// GET -> the entry appears with default preferences.
	resp, err = http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET /watchlist: %v", err)
	}
	var listed []watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&listed); err != nil {
		t.Fatalf("decode GET response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var found bool
	for _, e := range listed {
		if e.ID == created.ID {
			found = true
			if !reflect.DeepEqual(e.ReleaseTypes, watchlist.ReleaseTypes) {
				t.Fatalf("listed release_types = %v, want default %v", e.ReleaseTypes, watchlist.ReleaseTypes)
			}
		}
	}
	if !found {
		t.Fatal("GET /watchlist did not include the just-created entry")
	}

	// PATCH narrowing release_types -> 200 with the narrowed set.
	wantReleaseTypes := []string{"album", "ep"}
	req, err := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/watchlist/%d", ts.URL, created.ID), strings.NewReader(`{"release_types":["album","ep"]}`))
	if err != nil {
		t.Fatalf("build first PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH release_types: %v", err)
	}
	var patched1 watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&patched1); err != nil {
		t.Fatalf("decode first PATCH response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("first PATCH status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !reflect.DeepEqual(patched1.ReleaseTypes, wantReleaseTypes) {
		t.Fatalf("release_types = %v, want %v", patched1.ReleaseTypes, wantReleaseTypes)
	}

	// PATCH muting an event category -> 200 with both axes as expected.
	req, err = http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/watchlist/%d", ts.URL, created.ID), strings.NewReader(`{"muted_event_types":["deluxe_change"]}`))
	if err != nil {
		t.Fatalf("build second PATCH request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH muted_event_types: %v", err)
	}
	var patched2 watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&patched2); err != nil {
		t.Fatalf("decode second PATCH response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second PATCH status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !reflect.DeepEqual(patched2.ReleaseTypes, wantReleaseTypes) {
		t.Fatalf("release_types after second PATCH = %v, want unchanged %v", patched2.ReleaseTypes, wantReleaseTypes)
	}
	wantMuted := []string{"deluxe_change"}
	if !reflect.DeepEqual(patched2.MutedEventTypes, wantMuted) {
		t.Fatalf("muted_event_types = %v, want %v", patched2.MutedEventTypes, wantMuted)
	}

	// POST the same mbid again -> 409.
	resp, err = http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("second POST /watchlist: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("second POST status = %d, want %d", resp.StatusCode, http.StatusConflict)
	}

	// DELETE -> 204.
	req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/watchlist/%d", ts.URL, created.ID), nil)
	if err != nil {
		t.Fatalf("build first DELETE request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE status = %d, want %d", resp.StatusCode, http.StatusNoContent)
	}

	// GET -> the entry is gone.
	resp, err = http.Get(ts.URL + "/watchlist")
	if err != nil {
		t.Fatalf("GET after delete: %v", err)
	}
	var afterDelete []watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&afterDelete); err != nil {
		t.Fatalf("decode GET-after-delete response: %v", err)
	}
	resp.Body.Close()
	for _, e := range afterDelete {
		if e.ID == created.ID {
			t.Fatalf("GET after delete still contains entry id %d", created.ID)
		}
	}

	// DELETE again -> 404.
	req, err = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/watchlist/%d", ts.URL, created.ID), nil)
	if err != nil {
		t.Fatalf("build second DELETE request: %v", err)
	}
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("second DELETE: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("second DELETE status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}

	// POST the same mbid once more -> 201 with a new id.
	resp, err = http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("third POST /watchlist: %v", err)
	}
	var recreated watchlistEntryBody
	if err := json.NewDecoder(resp.Body).Decode(&recreated); err != nil {
		t.Fatalf("decode third POST response: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("third POST status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}
	if recreated.ID == created.ID {
		t.Fatalf("re-added entry reused id %d, want a new id (no tombstone, D-10)", created.ID)
	}
}

func TestWatchlist_Add_DoesNotLeakInternals(t *testing.T) {
	addErr := errors.New("pq: connection to postgres://user:hunter2@db:5432 failed")
	stub := stubStore{addFunc: func(context.Context, watchlist.AddParams) (watchlist.Entry, error) {
		return watchlist.Entry{}, addErr
	}}
	srv := httpserver.New(noopPinger{}, stub, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const body = `{"mbid":"x","name":"y"}`
	resp, err := http.Post(ts.URL+"/watchlist", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /watchlist: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	raw := string(data)
	for _, leak := range []string{"hunter2", "postgres://", "password"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("response body leaked %q: %s", leak, raw)
		}
	}
}
