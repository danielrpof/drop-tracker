package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// maxAddWatchlistBodyBytes bounds the POST /watchlist request body so an
// oversized payload fails json.Decoder rather than being buffered into
// memory in full (T-02-04).
const maxAddWatchlistBodyBytes = 65536

// maxMBIDRunes and maxNameRunes cap the two free-text fields on the add path
// before any database call (T-02-05). 36 is the width of a canonical MBID;
// this is a length bound only -- format is not validated (assumption
// A-02-01, plan 02-01).
const (
	maxMBIDRunes = 36
	maxNameRunes = 512
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
	MBID            string    `json:"mbid"`
	Name            string    `json:"name"`
	DeezerID        *string   `json:"deezer_id"`
	Disambiguation  *string   `json:"disambiguation"`
	ImageURL        *string   `json:"image_url"`
	ReleaseTypes    *[]string `json:"release_types"`
	MutedEventTypes *[]string `json:"muted_event_types"`
}

// handleAddWatchlist implements POST /watchlist (WLST-02): decode, reject
// unknown/over-posted keys, reject blank mbid/name (D-04), add via the
// watchlist store, and translate the duplicate-artist sentinel into 409
// (D-09; error translation itself lands in plan 02-02, this handler wires
// the branch now so the shape is right).
func (s *Server) handleAddWatchlist(w http.ResponseWriter, r *http.Request) {
	// Bound the body before decoding: a body past this ceiling makes the
	// decoder fail below, which the existing decode-error branch already
	// turns into a 400 -- no separate branch needed (T-02-04).
	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)

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

	// Rune-counted, not byte-counted, so a name with multi-byte characters
	// is not rejected for its byte count (T-02-05).
	if utf8.RuneCountInString(mbid) > maxMBIDRunes {
		writeError(w, http.StatusBadRequest, "mbid must be at most 36 characters")
		return
	}
	if utf8.RuneCountInString(name) > maxNameRunes {
		writeError(w, http.StatusBadRequest, "name must be at most 512 characters")
		return
	}

	// Reject an out-of-allow-list preference value before ever calling the
	// store -- fail-fast against the same watchlist.ReleaseTypes /
	// watchlist.EventTypes allow-lists that Service.Add's normalizeSet
	// re-validates as the non-bypassable backstop (T-02-13). This is a
	// membership check only; canonical ordering and de-duplication happen
	// once, inside Service.Add.
	if req.ReleaseTypes != nil {
		for _, v := range *req.ReleaseTypes {
			if !slices.Contains(watchlist.ReleaseTypes, v) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid release type: %q", v))
				return
			}
		}
	}
	if req.MutedEventTypes != nil {
		for _, v := range *req.MutedEventTypes {
			if !slices.Contains(watchlist.EventTypes, v) {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid event type: %q", v))
				return
			}
		}
	}

	params := watchlist.AddParams{
		MBID:           mbid,
		Name:           name,
		DeezerID:       req.DeezerID,
		Disambiguation: req.Disambiguation,
		ImageURL:       req.ImageURL,
	}
	if req.ReleaseTypes != nil {
		params.ReleaseTypes = *req.ReleaseTypes
	}
	if req.MutedEventTypes != nil {
		params.MutedEventTypes = *req.MutedEventTypes
	}

	entry, err := s.watchlist.Add(r.Context(), params)
	switch {
	case errors.Is(err, watchlist.ErrDuplicate):
		writeError(w, http.StatusConflict, "artist already on watchlist")
		return
	case errors.Is(err, watchlist.ErrInvalidReleaseType), errors.Is(err, watchlist.ErrInvalidEventType):
		// These sentinels wrap only the offending value, which came from
		// the client, so echoing err.Error() leaks nothing.
		writeError(w, http.StatusBadRequest, err.Error())
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
