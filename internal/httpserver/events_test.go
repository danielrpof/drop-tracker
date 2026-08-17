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
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
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
// name (HIST-01, DATA-02).
type eventsResponseBody struct {
	Events         []events.Event `json:"events"`
	NextCursor     *int64         `json:"next_cursor"`
	HasOlderEvents bool           `json:"has_older_events"`
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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()

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

// recordingQuerier is a minimal sqlc.Querier double that records the
// ListEventsParams it receives and returns an empty result. It embeds a nil
// sqlc.Querier so it satisfies the interface without implementing every
// method -- only ListEvents is ever called through it. This is used solely
// to prove the limit=100000 case reaches events.Service.List's own clamp
// (T-06-06): stubEventsStore bypasses the domain service entirely, so it
// cannot observe that clamp.
type recordingQuerier struct {
	sqlc.Querier
	gotParams sqlc.ListEventsParams
}

func (q *recordingQuerier) ListEvents(_ context.Context, arg sqlc.ListEventsParams) ([]sqlc.ListEventsRow, error) {
	q.gotParams = arg
	return nil, nil
}

// HasOlderEvents is an explicit stub, not left to resolve through the
// embedded nil sqlc.Querier: Service.List now calls HasOlderEvents on every
// List (Phase 10, DATA-02), and the nil embed would compile fine but
// nil-panic at runtime the moment it's called through -- go build cannot
// catch this, only a test run can (see this file's header comment and
// 10-02-PLAN.md's corrections-carried-forward note).
func (q *recordingQuerier) HasOlderEvents(_ context.Context, _ sqlc.HasOlderEventsParams) (bool, error) {
	return false, nil
}

var _ sqlc.Querier = (*recordingQuerier)(nil)

// TestHandleListEvents_Validation pins HIST-01/T-06-06 through T-06-10: every
// malformed artist_id/event_type/cursor/limit is rejected with a 400 before
// the store is ever called, an empty param is treated as absent rather than
// malformed, valid filters populate ListParams, and an over-large limit is
// clamped rather than rejected.
func TestHandleListEvents_Validation(t *testing.T) {
	t.Run("rejects malformed params without calling the store", func(t *testing.T) {
		cases := []struct {
			name  string
			query string
		}{
			{"artist_id non-numeric", "artist_id=abc"},
			{"artist_id zero", "artist_id=0"},
			{"artist_id negative", "artist_id=-3"},
			{"cursor non-numeric", "cursor=abc"},
			{"cursor zero", "cursor=0"},
			{"event_type not allow-listed", "event_type=bogus"},
			{"limit zero", "limit=0"},
			{"limit negative", "limit=-1"},
			{"limit non-numeric", "limit=abc"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				called := false
				stub := stubEventsStore{listFunc: func(context.Context, events.ListParams) (events.Page, error) {
					called = true
					return events.Page{}, nil
				}}
				srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
				ts := httptest.NewServer(srv.Router())
				defer ts.Close()

				resp, err := http.Get(ts.URL + "/events?" + tc.query)
				if err != nil {
					t.Fatalf("GET /events?%s: %v", tc.query, err)
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
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
					t.Fatalf("error body has empty message: %s", data)
				}
				if strings.Contains(strings.ToLower(eb.Error), "sql") || strings.Contains(eb.Error, "pq:") {
					t.Fatalf("error body looks like it leaked driver text: %s", data)
				}
				if called {
					t.Fatalf("store.List was called for a rejected request (%s)", tc.query)
				}
			})
		}
	})

	t.Run("empty artist_id is treated as absent, not malformed", func(t *testing.T) {
		called := false
		stub := stubEventsStore{listFunc: func(_ context.Context, p events.ListParams) (events.Page, error) {
			called = true
			if p.ArtistID != nil {
				t.Fatalf("ArtistID = %v, want nil for an empty artist_id param", *p.ArtistID)
			}
			return events.Page{}, nil
		}}
		srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
		ts := httptest.NewServer(srv.Router())
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/events?artist_id=")
		if err != nil {
			t.Fatalf("GET /events?artist_id=: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if !called {
			t.Fatal("store.List was not called for an empty (absent) artist_id")
		}
	})

	t.Run("artist_id and event_type filters both populate ListParams", func(t *testing.T) {
		var got events.ListParams
		stub := stubEventsStore{listFunc: func(_ context.Context, p events.ListParams) (events.Page, error) {
			got = p
			return events.Page{}, nil
		}}
		srv := httpserver.New(noopPinger{}, stubStore{}, stub, nil, discardLogger())
		ts := httptest.NewServer(srv.Router())
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/events?artist_id=7&event_type=new_release")
		if err != nil {
			t.Fatalf("GET /events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if got.ArtistID == nil || *got.ArtistID != 7 {
			t.Fatalf("ArtistID = %v, want 7", got.ArtistID)
		}
		if got.EventType == nil || *got.EventType != "new_release" {
			t.Fatalf("EventType = %v, want new_release", got.EventType)
		}
	})

	t.Run("limit above the maximum is clamped, not rejected", func(t *testing.T) {
		rq := &recordingQuerier{}
		svc := events.NewService(rq, 90)
		srv := httpserver.New(noopPinger{}, stubStore{}, svc, nil, discardLogger())
		ts := httptest.NewServer(srv.Router())
		defer ts.Close()

		resp, err := http.Get(ts.URL + "/events?limit=100000")
		if err != nil {
			t.Fatalf("GET /events?limit=100000: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		if rq.gotParams.PageSize > events.MaxPageSize {
			t.Fatalf("requested PageSize = %d, want <= %d (events.MaxPageSize)", rq.gotParams.PageSize, events.MaxPageSize)
		}
	})
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

// insertTestEventAt mirrors insertTestEvent but takes an explicit createdAt,
// which insertTestEvent cannot do -- it relies on the events table's
// DEFAULT now() and has no created_at column in its INSERT list. Phase 10's
// retention tests (DATA-02) need to seed rows at specific ages relative to
// now, so this helper sets created_at explicitly instead.
func insertTestEventAt(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID string, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	row := pool.QueryRow(context.Background(), `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name, created_at)
		VALUES ($1, 'musicbrainz', 'new_release', $2, 'Title', 'Artist', $3)
		RETURNING id`, artistID, externalID, createdAt)
	if err := row.Scan(&id); err != nil {
		t.Fatalf("insert test event at %v: %v", createdAt, err)
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

	svc := events.NewService(sqlc.New(pool), 90)

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

// TestListEvents_RetentionExcludesAgedOutRows is this plan's tracer test
// (Task 1, DATA-02): it proves the whole retention path end-to-end -- from
// the events table, through Service.List's cutoff computation, through
// GET /events's JSON response -- before any edge case is layered on top.
func TestListEvents_RetentionExcludesAgedOutRows(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)

	now := time.Now()
	agedOutID := insertTestEventAt(t, pool, artistID, mbid+"-old", now.AddDate(0, 0, -120))
	recentID := insertTestEventAt(t, pool, artistID, mbid+"-new", now.AddDate(0, 0, -1))

	svc := events.NewService(sqlc.New(pool), 90)
	srv := httpserver.New(pool, stubStore{}, svc, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(fmt.Sprintf("%s/events?artist_id=%d", ts.URL, artistID))
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body eventsResponseBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}

	if len(body.Events) != 1 || body.Events[0].ID != recentID {
		t.Fatalf("events = %+v, want exactly one event with id %d (the 1-day-old row)", body.Events, recentID)
	}
	for _, e := range body.Events {
		if e.ID == agedOutID {
			t.Fatalf("events = %+v, want the 120-day-old event (id %d) excluded by the retention window", body.Events, agedOutID)
		}
	}

	// The filter is read-side only -- both rows must still be physically
	// present in the table after the request (DATA-02, roadmap "nothing is
	// ever deleted").
	var count int
	row := pool.QueryRow(context.Background(), "SELECT count(*) FROM events WHERE artist_id = $1", artistID)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count events for artist: %v", err)
	}
	if count != 2 {
		t.Fatalf("events row count = %d, want 2 (retention must never delete rows)", count)
	}
}

// TestListEvents_RetentionBoundaryIsInclusive pins D-04: an event exactly at
// the retention cutoff must remain visible -- the comparison is >=, never >.
// The intuitive ">" reading is the wrong one here; this test is what pins
// ">=" in place.
//
// This calls sqlc.Queries.ListEvents directly with an explicit, fixed
// Cutoff, rather than going through events.Service.List with a
// wall-clock-derived retention window. Service.List always recomputes
// time.Now() internally (it is not injectable), so a boundary row timed
// relative to *this test's* clock would race the *service's own*, slightly
// later time.Now() read -- and any margin wide enough to reliably avoid
// that race is also wide enough to satisfy a strict ">" just as well as
// ">=", which would make this test unable to actually distinguish the two
// operators (silently passing even if D-04 regressed). Fixing Cutoff to a
// literal value the test controls removes the race and tests exactly what
// D-04 locks: the SQL predicate's own boundary inclusivity.
func TestListEvents_RetentionBoundaryIsInclusive(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)

	cutoff := time.Now().AddDate(0, 0, -90)
	atCutoff := cutoff // exactly equal to the cutoff -- ">=" must include, ">" must exclude
	beforeCutoff := cutoff.Add(-1 * time.Minute)

	atCutoffID := insertTestEventAt(t, pool, artistID, mbid+"-at-cutoff", atCutoff)
	beforeCutoffID := insertTestEventAt(t, pool, artistID, mbid+"-before-cutoff", beforeCutoff)

	q := sqlc.New(pool)
	rows, err := q.ListEvents(context.Background(), sqlc.ListEventsParams{
		ArtistID: &artistID,
		Cutoff:   pgtype.Timestamptz{Time: cutoff, Valid: true},
		PageSize: events.MaxPageSize,
	})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	seen := make(map[int64]bool, len(rows))
	for _, r := range rows {
		seen[r.ID] = true
	}
	if !seen[atCutoffID] {
		t.Fatalf("rows = %+v, want the exactly-at-cutoff event (id %d) included -- D-04 requires >=, not >", rows, atCutoffID)
	}
	if seen[beforeCutoffID] {
		t.Fatalf("rows = %+v, want the before-cutoff event (id %d) excluded", rows, beforeCutoffID)
	}
}

// TestListEvents_RetentionPagesNeverRepeatAnID resolves DATA-02's
// concurrency edge probe: each page recomputes its own cutoff from
// time.Now(), and because the cutoff only ever advances forward while
// pagination walks id DESC from newest to oldest, a later page can only
// exclude more old rows -- it can never resurrect an already-excluded one
// or re-serve a row the previous page returned.
func TestListEvents_RetentionPagesNeverRepeatAnID(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)

	now := time.Now()
	var inWindowIDs []int64
	for i := 0; i < 5; i++ {
		id := insertTestEventAt(t, pool, artistID, mbid+"-in-"+string(rune('a'+i)), now.AddDate(0, 0, -i))
		inWindowIDs = append(inWindowIDs, id)
	}
	agedOutA := insertTestEventAt(t, pool, artistID, mbid+"-aged-a", now.AddDate(0, 0, -120))
	agedOutB := insertTestEventAt(t, pool, artistID, mbid+"-aged-b", now.AddDate(0, 0, -200))

	svc := events.NewService(sqlc.New(pool), 90)

	seen := make(map[int64]bool)
	var cursor *int64
	for i := 0; i < len(inWindowIDs)+2; i++ { // extra iterations guard against an infinite loop if pagination breaks
		page, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistID, PageSize: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("List (page %d): %v", i, err)
		}
		if len(page.Events) == 0 {
			break
		}
		for _, e := range page.Events {
			if seen[e.ID] {
				t.Fatalf("event id %d appeared on two pages", e.ID)
			}
			seen[e.ID] = true
		}
		cursor = page.NextCursor
		if cursor == nil {
			break
		}
	}

	if len(seen) != len(inWindowIDs) {
		t.Fatalf("total distinct events paged through = %d, want %d (only the in-window rows)", len(seen), len(inWindowIDs))
	}
	for _, id := range inWindowIDs {
		if !seen[id] {
			t.Fatalf("in-window event id %d never appeared across any page", id)
		}
	}
	if seen[agedOutA] || seen[agedOutB] {
		t.Fatalf("an aged-out event id leaked into a page: seen = %+v", seen)
	}
}

// insertTestEventTyped mirrors insertTestEvent but takes an explicit
// event_type, so TestListEvents_Filters can seed all three event types per
// artist (HIST-01's filter-composition coverage).
func insertTestEventTyped(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID, eventType string) int64 {
	t.Helper()
	var id int64
	row := pool.QueryRow(context.Background(), `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		VALUES ($1, 'musicbrainz', $2, $3, 'Title', 'Artist')
		RETURNING id`, artistID, eventType, externalID)
	if err := row.Scan(&id); err != nil {
		t.Fatalf("insert test event: %v", err)
	}
	return id
}

// TestListEvents_Filters proves the artist_id and event_type filters apply
// independently and compose (HIST-01, D-06), and that a cursor page and its
// predecessor share no row. Two distinct artists are each seeded with all
// three event types, so a filter that silently did nothing to the predicate
// would fail an assertion here.
func TestListEvents_Filters(t *testing.T) {
	pool := testutil.NewTestPool(t)
	// testMBID derives its value from t.Name() alone, so two calls with the
	// same t return the same string -- distinct suffixes keep artist A and
	// artist B from colliding on the mbid unique constraint.
	base := testMBID(t)
	mbidA := base + "-a"
	mbidB := base + "-b"
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = ANY($1)", []string{mbidA, mbidB}); err != nil {
			t.Fatalf("cleanup: delete artists rows: %v", err)
		}
	})

	artistA := insertTestArtist(t, pool, mbidA)
	artistB := insertTestArtist(t, pool, mbidB)

	// eventIDs[artistID][eventType] = the id inserted for that combination.
	eventIDs := map[int64]map[string]int64{artistA: {}, artistB: {}}
	for _, artistID := range []int64{artistA, artistB} {
		mbid := mbidA
		if artistID == artistB {
			mbid = mbidB
		}
		for _, et := range watchlist.EventTypes {
			eventIDs[artistID][et] = insertTestEventTyped(t, pool, artistID, mbid+"-"+et, et)
		}
	}

	svc := events.NewService(sqlc.New(pool), 90)

	t.Run("artist_id filter applies independently", func(t *testing.T) {
		page, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistA})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page.Events) != len(watchlist.EventTypes) {
			t.Fatalf("len(Events) = %d, want %d", len(page.Events), len(watchlist.EventTypes))
		}
		for _, e := range page.Events {
			if e.ArtistID != artistA {
				t.Fatalf("event %d has ArtistID %d, want %d -- artist filter leaked another artist's row", e.ID, e.ArtistID, artistA)
			}
		}
	})

	t.Run("event_type filter applies independently", func(t *testing.T) {
		et := "deluxe_change"
		page, err := svc.List(context.Background(), events.ListParams{EventType: &et, PageSize: events.MaxPageSize})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		seen := make(map[int64]bool, len(page.Events))
		for _, e := range page.Events {
			if e.EventType != et {
				t.Fatalf("event %d has EventType %q, want %q -- event_type filter leaked a non-matching row", e.ID, e.EventType, et)
			}
			seen[e.ID] = true
		}
		if !seen[eventIDs[artistA][et]] {
			t.Fatalf("expected artist A's %s event (id %d) in the filtered results", et, eventIDs[artistA][et])
		}
		if !seen[eventIDs[artistB][et]] {
			t.Fatalf("expected artist B's %s event (id %d) in the filtered results", et, eventIDs[artistB][et])
		}
		// A filter that silently did nothing would also return the other
		// two event types for these two artists -- assert those specific
		// ids are absent.
		for _, otherType := range watchlist.EventTypes {
			if otherType == et {
				continue
			}
			if seen[eventIDs[artistA][otherType]] || seen[eventIDs[artistB][otherType]] {
				t.Fatalf("event_type=%s filter returned a %s row -- filter did not apply", et, otherType)
			}
		}
	})

	t.Run("artist_id and event_type filters compose", func(t *testing.T) {
		et := "guest_feature"
		page, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistA, EventType: &et})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(page.Events) != 1 {
			t.Fatalf("len(Events) = %d, want 1 (artist_id and event_type both applied)", len(page.Events))
		}
		if page.Events[0].ID != eventIDs[artistA][et] {
			t.Fatalf("event id = %d, want %d", page.Events[0].ID, eventIDs[artistA][et])
		}
		if page.Events[0].ArtistID != artistA || page.Events[0].EventType != et {
			t.Fatalf("event = %+v, want ArtistID %d and EventType %q", page.Events[0], artistA, et)
		}
	})

	t.Run("a cursor page shares no row with its predecessor", func(t *testing.T) {
		seen := make(map[int64]bool)
		var cursor *int64
		total := 0
		for i := 0; i < len(watchlist.EventTypes)+1; i++ { // one extra iteration guards against an infinite loop if pagination breaks
			page, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistA, PageSize: 1, Cursor: cursor})
			if err != nil {
				t.Fatalf("List (page %d): %v", i, err)
			}
			if len(page.Events) == 0 {
				break
			}
			for _, e := range page.Events {
				if seen[e.ID] {
					t.Fatalf("event id %d appeared on two pages", e.ID)
				}
				seen[e.ID] = true
				total++
			}
			cursor = page.NextCursor
			if cursor == nil {
				break
			}
		}
		if total != len(watchlist.EventTypes) {
			t.Fatalf("total events paged through = %d, want %d", total, len(watchlist.EventTypes))
		}
	})
}

func TestListEvents_NoMatchingRowsReturnsNonNilEmptySlice(t *testing.T) {
	pool := testutil.NewTestPool(t)
	svc := events.NewService(sqlc.New(pool), 90)

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

// TestRetention_DetectionStateQueriesStayUnfiltered is this phase's single
// most load-bearing test (Task 3, DATA-02): the automated proof of the
// roadmap's success criteria 3-5, and the guardrail against a future
// "consistency" pass adding the retention predicate to a query that must
// never have it. Adding a retention predicate to ListExternalIDs,
// HasAnyEvent, GroupTrackCountBaseline, or ListUnnotified is the exact
// regression this test exists to catch: (1) dedup-key loss -- the detector
// would treat an already-recorded release as fresh and re-notify it; (2)
// seed-mode reset -- the artist would fall back into seed mode and
// re-announce its entire back catalogue on the next poll cycle; (3) deluxe
// baseline loss -- a later tracklist expansion would either false-positive
// or silently miss the change entirely.
func TestRetention_DetectionStateQueriesStayUnfiltered(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)

	// One musicbrainz new_release event, 200 days old -- well outside any
	// plausible retention window -- with a populated release_group_mbid, a
	// non-zero track_count, and notified_at left NULL (pending).
	const externalID = "aged-out-release"
	releaseGroupMbid := mbid + "-group"
	trackCount := int32(12)
	createdAt := time.Now().AddDate(0, 0, -200)

	var eventID int64
	row := pool.QueryRow(context.Background(), `
		INSERT INTO events (artist_id, source, event_type, external_id, release_group_mbid, title, artist_name, track_count, created_at)
		VALUES ($1, 'musicbrainz', 'new_release', $2, $3, 'Title', 'Artist', $4, $5)
		RETURNING id`, artistID, externalID, releaseGroupMbid, trackCount, createdAt)
	if err := row.Scan(&eventID); err != nil {
		t.Fatalf("insert aged-out event: %v", err)
	}

	q := sqlc.New(pool)
	ctx := context.Background()

	t.Run("dedup key intact (criterion 3)", func(t *testing.T) {
		ids, err := q.ListExternalIDs(ctx, sqlc.ListExternalIDsParams{ArtistID: artistID, Source: "musicbrainz", EventType: "new_release"})
		if err != nil {
			t.Fatalf("ListExternalIDs: %v", err)
		}
		found := false
		for _, id := range ids {
			if id == externalID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListExternalIDs = %v, want %q present -- an aged-out dedup key must never disappear, or the detector would re-notify an already-recorded release", ids, externalID)
		}
	})

	t.Run("seed mode not reset (criterion 4)", func(t *testing.T) {
		hasAny, err := q.HasAnyEvent(ctx, sqlc.HasAnyEventParams{ArtistID: artistID, Source: "musicbrainz"})
		if err != nil {
			t.Fatalf("HasAnyEvent: %v", err)
		}
		if !hasAny {
			t.Fatal("HasAnyEvent = false, want true -- an aged-out row must still count, or the artist would fall back into seed mode and re-announce its entire back catalogue")
		}
	})

	t.Run("deluxe baseline survives (criterion 5)", func(t *testing.T) {
		// The fixture deliberately sets external_id ("aged-out-release")
		// and release_group_mbid (releaseGroupMbid) to different values,
		// so AdvanceGroupTrackCountBaseline (keyed on external_id, not
		// release_group_mbid) must be called with externalID here. A
		// count one greater than the fixture's stored track_count (12)
		// both advances the row and lets the returned previous value
		// prove the aged-out row was found and read -- a zero-row result
		// would be ambiguous between "the aged-out row was invisible"
		// and "no advance was needed," whereas one row carrying the
		// exact prior value proves both.
		higherCount := trackCount + 1
		rows, err := q.AdvanceGroupTrackCountBaseline(ctx, sqlc.AdvanceGroupTrackCountBaselineParams{
			ExternalID: externalID,
			TrackCount: &higherCount,
		})
		if err != nil {
			t.Fatalf("AdvanceGroupTrackCountBaseline: %v", err)
		}
		if len(rows) != 1 {
			t.Fatalf("AdvanceGroupTrackCountBaseline returned %d rows, want 1 -- an aged-out row must still supply the deluxe-change baseline", len(rows))
		}
		if rows[0] == nil {
			t.Fatal("previous_track_count = nil, want non-nil (the aged-out row already had a baseline)")
		}
		if *rows[0] != trackCount {
			t.Fatalf("previous_track_count = %d, want %d", *rows[0], trackCount)
		}
	})

	t.Run("pending notification still visible", func(t *testing.T) {
		rows, err := q.ListUnnotified(ctx)
		if err != nil {
			t.Fatalf("ListUnnotified: %v", err)
		}
		found := false
		for _, r := range rows {
			if r.ID == eventID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("ListUnnotified did not include the aged-out row (id %d) while notified_at is NULL", eventID)
		}
	})

	// Without this contrast, the four assertions above would also pass on a
	// build where the retention filter was never wired into ListEvents at
	// all -- this is what makes them meaningful.
	t.Run("contrast: the same row is absent from Service.List with a 90-day window", func(t *testing.T) {
		svc := events.NewService(q, 90)
		page, err := svc.List(ctx, events.ListParams{ArtistID: &artistID})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for _, e := range page.Events {
			if e.ID == eventID {
				t.Fatalf("events = %+v, want the 200-day-old event (id %d) excluded from the history feed", page.Events, eventID)
			}
		}
	})
}

// TestHandleListEvents_HasOlderEventsSignal proves D-06's three named states
// through a real httptest server and a decoded envelope (Task 2 of
// 10-02-PLAN.md): an empty table, a table with only in-window events, and a
// table with at least one aged-out event.
func TestHandleListEvents_HasOlderEventsSignal(t *testing.T) {
	pool := testutil.NewTestPool(t)
	mbid := testMBID(t)
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = $1", mbid); err != nil {
			t.Fatalf("cleanup: delete artists row: %v", err)
		}
	})

	artistID := insertTestArtist(t, pool, mbid)
	svc := events.NewService(sqlc.New(pool), 90)
	srv := httpserver.New(pool, stubStore{}, svc, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	get := func(t *testing.T) eventsResponseBody {
		t.Helper()
		resp, err := http.Get(fmt.Sprintf("%s/events?artist_id=%d", ts.URL, artistID))
		if err != nil {
			t.Fatalf("GET /events: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		var body eventsResponseBody
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode response body: %v", err)
		}
		return body
	}

	t.Run("empty table returns false", func(t *testing.T) {
		body := get(t)
		if body.HasOlderEvents {
			t.Fatal("has_older_events = true, want false for an artist with no events at all")
		}
	})

	t.Run("only in-window events returns false", func(t *testing.T) {
		insertTestEventAt(t, pool, artistID, mbid+"-recent", time.Now().AddDate(0, 0, -1))
		body := get(t)
		if body.HasOlderEvents {
			t.Fatal("has_older_events = true, want false when every event is within the retention window")
		}
	})

	t.Run("at least one aged-out event returns true", func(t *testing.T) {
		insertTestEventAt(t, pool, artistID, mbid+"-aged", time.Now().AddDate(0, 0, -120))
		body := get(t)
		if !body.HasOlderEvents {
			t.Fatal("has_older_events = false, want true once an aged-out event exists")
		}
	})
}

// TestListEvents_HasOlderEventsRespectsFilters proves the flag is scoped by
// the request's own artist_id filter (D-06), not answering a table-wide
// question: artist A has an aged-out event, artist B has only in-window
// events, and the artist_id=B case must be the assertion that actually
// distinguishes "the query applies its filters" from "the query always
// answers true once anything anywhere is aged out."
func TestListEvents_HasOlderEventsRespectsFilters(t *testing.T) {
	pool := testutil.NewTestPool(t)
	base := testMBID(t)
	mbidA := base + "-a"
	mbidB := base + "-b"
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE mbid = ANY($1)", []string{mbidA, mbidB}); err != nil {
			t.Fatalf("cleanup: delete artists rows: %v", err)
		}
	})

	artistA := insertTestArtist(t, pool, mbidA)
	artistB := insertTestArtist(t, pool, mbidB)

	now := time.Now()
	insertTestEventAt(t, pool, artistA, mbidA+"-aged", now.AddDate(0, 0, -120))
	insertTestEventAt(t, pool, artistB, mbidB+"-recent", now.AddDate(0, 0, -1))

	svc := events.NewService(sqlc.New(pool), 90)

	unfiltered, err := svc.List(context.Background(), events.ListParams{})
	if err != nil {
		t.Fatalf("List (unfiltered): %v", err)
	}
	if !unfiltered.HasOlderEvents {
		t.Fatal("HasOlderEvents = false, want true for the unfiltered scope (artist A's aged-out row is in scope)")
	}

	pageA, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistA})
	if err != nil {
		t.Fatalf("List (artist_id=A): %v", err)
	}
	if !pageA.HasOlderEvents {
		t.Fatal("HasOlderEvents = false, want true for artist_id=A, who has an aged-out event")
	}

	pageB, err := svc.List(context.Background(), events.ListParams{ArtistID: &artistB})
	if err != nil {
		t.Fatalf("List (artist_id=B): %v", err)
	}
	if pageB.HasOlderEvents {
		t.Fatal("HasOlderEvents = true, want false for artist_id=B, who has only in-window events -- the query must apply its own artist_id filter, not answer a table-wide question")
	}
}
