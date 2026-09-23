package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/tags"
)

// maxTagBodyBytes bounds the attach request body (T-24-05); a tag name is
// capped at 32 runes before any query, so a request this small is never a
// legitimate large payload.
const maxTagBodyBytes = 4096

// TagStore is the minimal surface internal/httpserver needs from the tags
// domain -- narrower than *tags.Service so a stub can implement it in
// tests. Mirrors watchlist.Store / SettingsStore. Detach is added by the
// plan's Task 3.
type TagStore interface {
	Attach(ctx context.Context, entryID int64, name string) (tags.AttachResult, error)
}

// WithTags supplies the tags domain dependency backing POST
// /watchlist/{id}/tags (and, once Task 3 lands, DELETE
// /watchlist/{id}/tags/{tag_id}). Absent, the handlers answer 503 with the
// shared fixed error body, mirroring WithSettings/WithStatus; a cmd/server
// binary always wires it.
func WithTags(store TagStore) Option {
	return func(c *serverConfig) {
		c.tags = store
	}
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
