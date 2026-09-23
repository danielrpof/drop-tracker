package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/tags"
)

// maxTagBodyBytes bounds the attach request body (T-24-05); a tag name is
// capped at 32 runes before any query, so a request this small is never a
// legitimate large payload.
const maxTagBodyBytes = 4096

// TagStore is the minimal surface internal/httpserver needs from the tags
// domain -- narrower than *tags.Service so a stub can implement it in
// tests. Mirrors watchlist.Store / SettingsStore.
type TagStore interface {
	Attach(ctx context.Context, entryID int64, name string) (tags.AttachResult, error)
	Detach(ctx context.Context, entryID, tagID int64) error
	List(ctx context.Context) ([]tags.Summary, error)
}

// WithTags supplies the tags domain dependency backing POST
// /watchlist/{id}/tags and DELETE /watchlist/{id}/tags/{tag_id}. Absent,
// the handlers answer 503 with the shared fixed error body, mirroring
// WithSettings/WithStatus; a cmd/server binary always wires it.
func WithTags(store TagStore) Option {
	return func(c *serverConfig) {
		c.tags = store
	}
}

// parseTagID reads and validates the {tag_id} path segment the same way
// parseWatchlistID validates {id} -- tags.id is BIGSERIAL, so 0 and
// negatives are never valid.
func parseTagID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "tag_id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid tag id: %q", raw)
	}
	return id, nil
}

// attachTagRequest is the request DTO for POST /watchlist/{id}/tags. It
// carries only the client-suppliable field -- id is server-derived from the
// path, so an over-posted id is rejected by DisallowUnknownFields.
type attachTagRequest struct {
	Name string `json:"name"`
}

// tagResponse is the wire shape for a single attached/detached tag.
type tagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// handleAttachTag implements POST /watchlist/{id}/tags (TAG-01, D-20, D-21).
// The client-facing normalization check is a fail-fast convenience -- the
// service re-validates non-bypassably (three-layer validation).
func (s *Server) handleAttachTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	id, err := parseWatchlistID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid watchlist id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTagBodyBytes)
	var req attachTagRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Fail-fast, non-bypassable check happens again inside Attach -- this
	// call only avoids a round trip for the common client-side typo case.
	if _, err := tags.NormalizeName(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, tagNameErrorMessage(err))
		return
	}

	result, err := s.tags.Attach(r.Context(), id, req.Name)
	switch {
	case errors.Is(err, tags.ErrNameRequired), errors.Is(err, tags.ErrNameTooLong), errors.Is(err, tags.ErrNameInvalid):
		writeError(w, http.StatusBadRequest, tagNameErrorMessage(err))
		return
	case errors.Is(err, tags.ErrEntryNotFound):
		writeError(w, http.StatusNotFound, "watchlist entry not found")
		return
	case errors.Is(err, tags.ErrTagCapReached):
		writeError(w, http.StatusConflict, "artist already has the maximum of 10 tags")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("tags_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if result.Created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(tagResponse{ID: result.Tag.ID, Name: result.Tag.Name})
}

// handleDetachTag implements DELETE /watchlist/{id}/tags/{tag_id} (TAG-02,
// D-20, D-21). Any outcome short of "watchlist entry not found" is 204 --
// detaching a link that never existed is idempotent success, never 404.
func (s *Server) handleDetachTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	id, err := parseWatchlistID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid watchlist id")
		return
	}
	tagID, err := parseTagID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	err = s.tags.Detach(r.Context(), id, tagID)
	switch {
	case errors.Is(err, tags.ErrEntryNotFound):
		writeError(w, http.StatusNotFound, "watchlist entry not found")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("tags_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// 204 carries no payload -- no Content-Type, no encoder call.
	w.WriteHeader(http.StatusNoContent)
}

// handleListTags implements GET /tags (TAG-05, D-11, D-13, D-30): the whole
// vocabulary with watched-only carrier counts, in the stable lower(name)
// order the DB query already guarantees. Also the autocomplete source
// (D-30) and Manage tags' list (D-22).
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	summaries, err := s.tags.List(r.Context())
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("tags_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if summaries == nil {
		summaries = []tags.Summary{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(summaries)
}

// tagNameErrorMessage maps a tags name-validation sentinel to its fixed,
// operator-authored 400 body -- never the submitted name (T-24-07).
func tagNameErrorMessage(err error) string {
	switch {
	case errors.Is(err, tags.ErrNameRequired):
		return "tag name is required"
	case errors.Is(err, tags.ErrNameTooLong):
		return "tag name must be at most 32 characters"
	case errors.Is(err, tags.ErrNameInvalid):
		return "tag name contains invalid characters"
	default:
		return "invalid tag name"
	}
}
