package httpserver

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/events"
)

// eventsResponse is the GET /events response envelope (HIST-01, UI-03):
// events is always a JSON array, never null (Service.List's non-nil-slice
// guarantee, reinforced by the defensive backstop below); next_cursor is
// null once the feed is exhausted -- the client's unambiguous "hide Load
// more" signal.
type eventsResponse struct {
	Events     []events.Event `json:"events"`
	NextCursor *int64         `json:"next_cursor"`
}

// handleListEvents implements GET /events (HIST-01, UI-03): this task's
// handler reads no query parameters yet -- it calls s.events.List with a
// zero-value ListParams and lets the domain's own clamp supply the page
// size (a later plan adds artist_id/event_type/cursor parsing). On error,
// this mirrors every other handler exactly: log the raw error via
// httplog.SetAttrs, then respond with the fixed operator-authored
// "internal error" message (T-06-01, V13) -- never raw DB/driver error
// text.
func (s *Server) handleListEvents(w http.ResponseWriter, r *http.Request) {
	page, err := s.events.List(r.Context(), events.ListParams{})
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("events_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	evs := page.Events
	if evs == nil {
		evs = []events.Event{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(eventsResponse{Events: evs, NextCursor: page.NextCursor})
}
