package httpserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// errorResponse is the single D-13 error-body shape for every watchlist
// handler: {"error": "message"}.
type errorResponse struct {
	Error string `json:"error"`
}

// writeError writes a D-13 {"error": "..."} JSON body with the given
// status code. msg is always an operator-authored fixed string -- never raw
// error text from a downstream dependency, which could leak internals
// (T-02-03).
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

// addWatchlistRequest is the request DTO for POST /watchlist. It carries
// only client-suppliable fields -- id, artist_id, created_at and updated_at
// are server-owned and are deliberately absent so an over-posted key is
// rejected by DisallowUnknownFields rather than silently discarded
// (T-02-01).
type addWatchlistRequest struct {
	MBID           string  `json:"mbid"`
	Name           string  `json:"name"`
	DeezerID       *string `json:"deezer_id"`
	Disambiguation *string `json:"disambiguation"`
	ImageURL       *string `json:"image_url"`
}

// handleAddWatchlist implements POST /watchlist (WLST-02): decode, reject
// unknown/over-posted keys, reject blank mbid/name (D-04), add via the
// watchlist store, and translate the duplicate-artist sentinel into 409
// (D-09; error translation itself lands in plan 02-02, this handler wires
// the branch now so the shape is right).
func (s *Server) handleAddWatchlist(w http.ResponseWriter, r *http.Request) {
	var req addWatchlistRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	mbid := strings.TrimSpace(req.MBID)
	name := strings.TrimSpace(req.Name)
	if mbid == "" || name == "" {
		writeError(w, http.StatusBadRequest, "mbid and name are required")
		return
	}

	entry, err := s.watchlist.Add(r.Context(), watchlist.AddParams{
		MBID:           mbid,
		Name:           name,
		DeezerID:       req.DeezerID,
		Disambiguation: req.Disambiguation,
		ImageURL:       req.ImageURL,
	})
	switch {
	case errors.Is(err, watchlist.ErrDuplicate):
		writeError(w, http.StatusConflict, "artist already on watchlist")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(entry)
}
