package httpserver_test

// This file proves GET /events end-to-end (HIST-01, UI-03): the unit-level
// envelope/error-leak branches against a stub events.Store, mirroring
// watchlist_test.go's stubStore pattern, plus real-Postgres coverage of the
// keyset pagination and ordering behavior ListEvents (and Service.List
// wrapping it) actually provide.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// stubEventsStore is a file-local double for events.Store, mirroring
// stubStore's func-field pattern: each method calls its func field when
// set and otherwise returns a zero value with a nil error, so a bare
// stubEventsStore{} is safe to pass into every test that never exercises
// the events routes.
type stubEventsStore struct {
	listFunc func(ctx context.Context, p events.ListParams) (events.Page, error)
}

func (s stubEventsStore) List(ctx context.Context, p events.ListParams) (events.Page, error) {
	if s.listFunc != nil {
		return s.listFunc(ctx, p)
	}
	return events.Page{}, nil
}

var _ events.Store = stubEventsStore{}

// eventsResponseBody mirrors the GET /events response envelope by field
// name (HIST-01).
type eventsResponseBody struct {
	Events     []events.Event `json:"events"`
	NextCursor *int64         `json:"next_cursor"`
}

func TestHandleListEvents_HappyPathReturns200WithEnvelope(t *testing.T) {
	want := []events.Event{
		{ID: 2, ArtistID: 1, Source: "musicbrainz", EventType: "new_release", ExternalID: "x", Title: "T", ArtistName: "A"},
	}
	stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
		return events.Page{Events: want, NextCursor: nil}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body eventsResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if len(body.Events) != 1 || body.Events[0].ID != 2 {
		t.Fatalf("events = %+v, want one event with id 2", body.Events)
	}
	if body.NextCursor != nil {
		t.Fatalf("next_cursor = %v, want nil", body.NextCursor)
	}
}

// TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor pins HIST-01's
// "events table with no matching rows encodes as events: [], never null"
// must-have, plus the paired "no more pages" contract for next_cursor.
func TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(t *testing.T) {
	stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
		return events.Page{Events: []events.Event{}, NextCursor: nil}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	raw := string(data)
	if !strings.Contains(raw, `"events":[]`) {
		t.Fatalf("body = %s, want events to encode as [] not null", raw)
	}
	if !strings.Contains(raw, `"next_cursor":null`) {
		t.Fatalf("body = %s, want next_cursor to encode as null", raw)
	}
}

// TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray covers the
// handler's own defensive nil-substitution backstop, mirroring
// TestWatchlist_List_NilSliceStillEncodesAsEmptyArray.
func TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray(t *testing.T) {
	stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
		return events.Page{Events: nil, NextCursor: nil}, nil
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if !strings.Contains(string(data), `"events":[]`) {
		t.Fatalf("body = %s, want events to encode as [] not null even for a nil store slice", data)
	}
}

// TestHandleListEvents_StoreErrorReturns500WithFixedMessage pins T-06-01:
// a store error must never leak raw text into the response body.
func TestHandleListEvents_StoreErrorReturns500WithFixedMessage(t *testing.T) {
	const rawErr = "connection refused: db-error-marker"
	stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
		return events.Page{}, errors.New(rawErr)
	}}
	srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/events")
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusInternalServerError)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	var eb errorBody
	if err := json.Unmarshal(data, &eb); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if eb.Error != "internal error" {
		t.Fatalf("error = %q, want %q", eb.Error, "internal error")
	}
	if strings.Contains(string(data), rawErr) {
		t.Fatalf("response body leaked raw store error text: %s", data)
	}
}

// The tests below exercise events.Service.List against real Postgres,
// proving the keyset pagination and ordering contract ListEvents (and its
// Service wrapper) provide -- newest first, no row appears on two
// consecutive pages, a full page sets next_cursor, a partial page leaves it
// nil.

func insertTestEvent(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID string) int64 {
	t.Helper()
	var id int64
	row := pool.QueryRow(context.Background(), `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		VALUES ($1, 'musicbrainz', 'new_release', $2, 'Title', 'Artist')
		RETURNING id`, artistID, externalID)
	if err := row.Scan(&id); err != nil {
		t.Fatalf("insert test event: %v", err)
	}
	return id
}

func insertTestArtist(t *testing.T, pool *pgxpool.Pool, mbid string) int64 {
	t.Helper()
	var id int64
	row := pool.QueryRow(context.Background(), `
		INSERT INTO artists (mbid, name) VALUES ($1, 'Events Test Artist') RETURNING id`, mbid)
	if err := row.Scan(&id); err != nil {
		t.Fatalf("insert test artist: %v", err)
	}
	return id
}

func TestListEvents_OrderedNewestFirstAndKeysetPaginates(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)

	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, insertTestEvent(t, pool, artistID, mbid+"-ext-"+string(rune('a'+i))))
	}

	svc := events.NewService(sqlc.New(pool))

	// Page 1: page size 2, no cursor -- expect the two highest ids, newest
	// first, and a non-nil next_cursor (the page came back full).
	page1, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistID, PageSize: 2})
	if err != nil {
		t.Fatalf("List page 1: %v", err)
	}
	if len(page1.Events) != 2 {
		t.Fatalf("page1 events = %d, want 2", len(page1.Events))
	}
	if page1.Events[0].ID != ids[4] || page1.Events[1].ID != ids[3] {
		t.Fatalf("page1 ids = [%d %d], want [%d %d] (newest first)", page1.Events[0].ID, page1.Events[1].ID, ids[4], ids[3])
	}
	if page1.NextCursor == nil || *page1.NextCursor != ids[3] {
		t.Fatalf("page1 next_cursor = %v, want %d", page1.NextCursor, ids[3])
	}

	// Page 2: cursor = page1's next_cursor -- must contain no row that
	// appeared on page 1.
	page2, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistID, PageSize: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatalf("List page 2: %v", err)
	}
	if len(page2.Events) != 2 {
		t.Fatalf("page2 events = %d, want 2", len(page2.Events))
	}
	seenOnPage1 := map[int64]bool{page1.Events[0].ID: true, page1.Events[1].ID: true}
	for _, e := range page2.Events {
		if seenOnPage1[e.ID] {
			t.Fatalf("event id %d appeared on both page 1 and page 2", e.ID)
		}
	}
	if page2.Events[0].ID != ids[2] || page2.Events[1].ID != ids[1] {
		t.Fatalf("page2 ids = [%d %d], want [%d %d]", page2.Events[0].ID, page2.Events[1].ID, ids[2], ids[1])
	}

	// Page 3: exactly one row left -- a partial page must leave next_cursor
	// nil.
	page3, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistID, PageSize: 2, Cursor: page2.NextCursor})
	if err != nil {
		t.Fatalf("List page 3: %v", err)
	}
	if len(page3.Events) != 1 || page3.Events[0].ID != ids[0] {
		t.Fatalf("page3 events = %+v, want exactly one event with id %d", page3.Events, ids[0])
	}
	if page3.NextCursor != nil {
		t.Fatalf("page3 next_cursor = %v, want nil (partial page)", page3.NextCursor)
	}
}

func TestListEvents_NoMatchingRowsReturnsNonNilEmptySlice(t *testing.T) {
	pool := testutil.NewTestPool(t)
	svc := events.NewService(sqlc.New(pool))

	missing := int64(-1)
	page, err := svc.List(context.Background(), events.ListParams{ArtistID: &missing})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if page.Events == nil {
		t.Fatal("Events is nil, want a non-nil empty slice")
	}
	if len(page.Events) != 0 {
		t.Fatalf("len(Events) = %d, want 0", len(page.Events))
	}
	if page.NextCursor != nil {
		t.Fatalf("NextCursor = %v, want nil", page.NextCursor)
	}
}
