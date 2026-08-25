package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// eventsResponse is the GET /events response envelope (HIST-01, UI-03):
// events is always a JSON array, never null (Service.List's non-nil-slice
// guarantee, reinforced by the defensive backstop below); next_cursor is an
// opaque token -- a feed position, not an id -- once the feed is exhausted
// it is null, the client's unambiguous "hide Load more" signal. The token
// is opaque to the client: the only correct client behaviour is to hand it
// back verbatim as the next request's cursor query param, never parse,
// compare, or arithmetically manipulate it. has_older_events (DATA-02,
// D-06) is a JSON boolean, never a day count or the raw
// EVENT_RETENTION_DAYS value -- it tells the frontend whether this scope
// has any event hidden by the retention window, without exposing the
// window's configured length.
type eventsResponse struct {
	Events         []events.Event `json:"events"`
	NextCursor     *string        `json:"next_cursor"`
	HasOlderEvents bool           `json:"has_older_events"`
}

// parseOptionalPositiveInt64 reads the named query param from r: an absent
// or empty value means "no filter on this axis" (ok=true, value=nil); a
// value that fails to parse as base-10 int64 or that is below 1 is rejected
// (ok=false) -- ids are BIGSERIAL, so zero and negatives are never valid,
// exactly as parseWatchlistID already reasons for the watchlist id path
// segment. A present, valid value returns a pointer to it (ok=true).
func parseOptionalPositiveInt64(r *http.Request, name string) (*int64, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, true
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || v < 1 {
		return nil, false
	}
	return &v, true
}

// parseOptionalPageSize reads the limit query param: absent or empty means
// "let events.Service.List's DefaultPageSize apply" (ok=true, size=0); a
// value that fails to parse or is below 1 is rejected (ok=false). A value
// above events.MaxPageSize is deliberately NOT rejected here -- it is passed
// through unclamped so events.Service.List's own clamp (T-06-06) reduces it,
// which keeps the DoS mitigation in the domain layer where no caller, HTTP
// or otherwise, can bypass it.
func parseOptionalPageSize(r *http.Request) (int32, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return 0, false
	}
	return int32(v), true //nolint:gosec // int32 wraparound from a huge v is harmless here: every reachable output is either <=0 or >MaxPageSize (caught by events.Service.List's clamp, see comment above) or lands directly in the already-valid [1,MaxPageSize] range
}

// handleListEvents implements GET /events (HIST-01, UI-03): reads four
// optional query params -- artist_id, event_type, cursor, limit -- and
// builds a populated events.ListParams, rejecting any malformed value with a
// 400 before ever calling the store (T-06-06 through T-06-10). On a store
// error, this mirrors every other handler exactly: log the raw error to the
// request-scoped structured logger below, then respond with the fixed
// operator-authored "internal error" message (T-06-01, V13) -- never raw
// DB/driver error text.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	artistID, ok := parseOptionalPositiveInt64(r, "artist_id")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid artist_id")
		return
	}

	var eventType *string
	if raw := strings.TrimSpace(r.URL.Query().Get("event_type")); raw != "" {
		if !slices.Contains(watchlist.EventTypes, raw) {
			writeError(w, http.StatusBadRequest, "invalid event type")
			return
		}
		eventType = &raw
	}

	// cursor is parsed through events.DecodeCursor, not
	// parseOptionalPositiveInt64: it is now an opaque encoded token, not a
	// raw id. Absent/empty means no cursor; any decode error means 400
	// before the store is ever called. The response message string is
	// reused verbatim from the pre-composite-cursor behavior so the
	// existing 400 table cases (cursor=abc, cursor=0) keep passing.
	var cursor *events.Cursor
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := events.DecodeCursor(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid cursor")
			return
		}
		cursor = &decoded
	}

	pageSize, ok := parseOptionalPageSize(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid limit")
		return
	}

	page, err := s.events.List(r.Context(), events.ListParams{
		ArtistID:  artistID,
		EventType: eventType,
		Cursor:    cursor,
		PageSize:  pageSize,
	})
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("events_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	evs := page.Events
	if evs == nil {
		evs = []events.Event{}
	}

	// The decoded release-date string reaches SQL only as a bound sqlc
	// parameter via events.Service.List -- never interpolated into a query
	// string here or anywhere downstream (T-g6i-02).
	var nextCursor *string
	if page.NextCursor != nil {
		encoded := events.EncodeCursor(*page.NextCursor)
		nextCursor = &encoded
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(eventsResponse{Events: evs, NextCursor: nextCursor, HasOlderEvents: page.HasOlderEvents})
}
