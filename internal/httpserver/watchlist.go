package httpserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
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

// maxDeezerIDRunes, maxDisambiguationRunes and maxImageURLRunes cap the
// three optional metadata fields on the add path the same way maxMBIDRunes
// and maxNameRunes cap the two required ones (T-02-34; T-02-05 covered only
// mbid and name, and these three were left unbounded and untrimmed, an
// oversight caught by the phase 02 review). deezer_id is a short numeric
// identifier;
// disambiguation mirrors name's cap since both are short free-text
// descriptors; image_url uses the common practical URL-length ceiling.
// Format (e.g. that image_url parses as a URL) is deliberately not
// validated here -- rendering-surface validation for image_url is Phase 5
// (Discord embeds) and Phase 6 (React UI)'s responsibility, per T-02-18.
const (
	maxDeezerIDRunes       = 64
	maxDisambiguationRunes = 512
	maxImageURLRunes       = 2048
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

// decodeJSONBody is the one shared decode path for every watchlist JSON
// route. It rejects unknown fields the same way both call sites already
// did, and additionally asserts the body held exactly one JSON value: a
// decoder consumes exactly one JSON value per call and never asserts
// end-of-stream on its own, and DisallowUnknownFields constrains keys
// *inside* the decoded object, not values concatenated after it -- so
// neither mechanism already in place catches a second top-level value
// (WR-02, G-02-1). The check is a second decode into a throwaway struct that
// must report end-of-stream (errors.Is(err, io.EOF)); anything else -- a nil
// error, a syntax error, a type error -- means the body carried more than
// one JSON value.
//
// The helper deliberately does not classify its failure: every mode maps to
// the same error, and by extension the same 400 with the same message, at
// each call site. A caller learning which parser rule they tripped learns
// about the parser, not about their request.
func decodeJSONBody(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("request body must contain exactly one JSON value")
	}
	return nil
}

// parseWatchlistID reads and validates the {id} path segment shared by
// DELETE /watchlist/{id} (this plan) and plan 02-04's PATCH
// /watchlist/{id}. Ids are BIGSERIAL, so 0 and negatives are never valid --
// rejecting them here, before any service call, keeps nonsense out of the
// query (T-02-07).
func parseWatchlistID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid watchlist id: %q", raw)
	}
	return id, nil
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
	if err := decodeJSONBody(r, &req); err != nil {
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

	// Trim and length-cap the three optional metadata fields the same way
	// mbid and name are bounded above -- omitted entirely from T-02-05's
	// original scope, which named only mbid and name. A nil pointer (the
	// field was absent from the body) is left untouched; only a supplied
	// value is trimmed and measured.
	if req.DeezerID != nil {
		v := strings.TrimSpace(*req.DeezerID)
		if utf8.RuneCountInString(v) > maxDeezerIDRunes {
			writeError(w, http.StatusBadRequest, "deezer_id must be at most 64 characters")
			return
		}
		req.DeezerID = &v
	}
	if req.Disambiguation != nil {
		v := strings.TrimSpace(*req.Disambiguation)
		if utf8.RuneCountInString(v) > maxDisambiguationRunes {
			writeError(w, http.StatusBadRequest, "disambiguation must be at most 512 characters")
			return
		}
		req.Disambiguation = &v
	}
	if req.ImageURL != nil {
		v := strings.TrimSpace(*req.ImageURL)
		if utf8.RuneCountInString(v) > maxImageURLRunes {
			writeError(w, http.StatusBadRequest, "image_url must be at most 2048 characters")
			return
		}
		req.ImageURL = &v
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

// handleListWatchlist implements GET /watchlist (WLST-04): every watchlisted
// artist as a bare JSON array, with no envelope (D-12). An empty watchlist
// still returns 200 with a body of exactly [] -- never null -- so the
// nil-substitution below is a defensive backstop even though Service.List
// already guarantees a non-nil slice.
func (s *Server) handleListWatchlist(w http.ResponseWriter, r *http.Request) {
	entries, err := s.watchlist.List(r.Context())
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if entries == nil {
		entries = []watchlist.Entry{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entries)
}

// updateWatchlistRequest is the request DTO for PATCH /watchlist/{id}. It
// carries only the two preference axes -- id, artist_id, mbid, name and the
// timestamps are not modifiable through this route and are deliberately
// absent so an attempt to set them is rejected by DisallowUnknownFields
// rather than silently discarded (T-02-11).
type updateWatchlistRequest struct {
	ReleaseTypes    *[]string `json:"release_types"`
	MutedEventTypes *[]string `json:"muted_event_types"`
}

// handleUpdateWatchlist implements PATCH /watchlist/{id} (WLST-05, WLST-06,
// D-11): partial-update semantics for two independent preference axes. A
// nil field in the request means "leave this axis untouched"; an explicit
// empty array means "watch/mute nothing on this axis" -- distinct states,
// both representable via updateWatchlistRequest's *[]string fields. A body
// that supplies neither key is rejected before the response is written --
// the domain itself now enforces this rule and returns a dedicated sentinel
// before touching the database (WR-01, G-02-1); this handler's job is only
// to translate that rejection to 400.
func (s *Server) handleUpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	id, err := parseWatchlistID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid watchlist id")
		return
	}

	// Bound the body before decoding, matching the ceiling the add path
	// applies (T-02-16).
	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)

	var req updateWatchlistRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	entry, err := s.watchlist.UpdatePreferences(r.Context(), id, watchlist.PreferencesParams{
		ReleaseTypes:    req.ReleaseTypes,
		MutedEventTypes: req.MutedEventTypes,
	})
	switch {
	case errors.Is(err, watchlist.ErrNoPreferencesSupplied):
		writeError(w, http.StatusBadRequest, "no preferences supplied")
		return
	case errors.Is(err, watchlist.ErrNotFound):
		writeError(w, http.StatusNotFound, "watchlist entry not found")
		return
	case errors.Is(err, watchlist.ErrInvalidReleaseType), errors.Is(err, watchlist.ErrInvalidEventType):
		// These sentinels wrap only the offending value, which came from the
		// client, so echoing err.Error() leaks nothing.
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entry)
}

// handleRemoveWatchlist implements DELETE /watchlist/{id} (WLST-03): a hard
// delete (D-10) that responds 204 with no body on success, 404 when the id
// does not exist (including a repeat delete of an id already removed), and
// 400 -- before any service call -- for a malformed id.
func (s *Server) handleRemoveWatchlist(w http.ResponseWriter, r *http.Request) {
	id, err := parseWatchlistID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid watchlist id")
		return
	}

	err = s.watchlist.Remove(r.Context(), id)
	switch {
	case errors.Is(err, watchlist.ErrNotFound):
		writeError(w, http.StatusNotFound, "watchlist entry not found")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 204 carries no payload -- no Content-Type, no encoder call.
	w.WriteHeader(http.StatusNoContent)
}
