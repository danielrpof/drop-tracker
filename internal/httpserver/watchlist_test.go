package httpserver_test

// This file proves POST /watchlist end-to-end (WLST-02): the happy path
// against a real Postgres (row persisted, D-08 defaults applied), plus the
// unit-level validation and error-leak branches against a stub
// watchlist.Store -- mirroring health_test.go's stubPinger pattern.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

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
