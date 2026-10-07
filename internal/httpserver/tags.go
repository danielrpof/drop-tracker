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

// maxTagBodyBytes bounds tag request bodies (T-24-05); a name is capped at
// tags.MaxNameRunes, so a body this small is never legitimate large input.
const maxTagBodyBytes = 4096

const (
	codeTagNameRequired  = "tag_name_required"
	codeTagNameTooLong   = "tag_name_too_long"
	codeTagNameInvalid   = "tag_name_invalid"
	codeTagCapReached    = "tag_cap_reached"
	codeTagNameTaken     = "tag_name_taken"
	codeTagNotFound      = "tag_not_found"
	codeTagMergeIntoSelf = "tag_merge_into_self"
	codeEntryNotFound    = "watchlist_entry_not_found"
)

// tagErrors is the one error -> (status, code) table for every tags route.
var tagErrors = []domainError{
	{tags.ErrNameRequired, http.StatusBadRequest, codeTagNameRequired},
	{tags.ErrNameTooLong, http.StatusBadRequest, codeTagNameTooLong},
	{tags.ErrNameInvalid, http.StatusBadRequest, codeTagNameInvalid},
	{tags.ErrTagCapReached, http.StatusConflict, codeTagCapReached},
	{tags.ErrTagNotFound, http.StatusNotFound, codeTagNotFound},
	{tags.ErrMergeIntoSelf, http.StatusBadRequest, codeTagMergeIntoSelf},
	{tags.ErrEntryNotFound, http.StatusNotFound, codeEntryNotFound},
}

// writeTagError writes the mapped tags error, or logs and answers 500.
func writeTagError(w http.ResponseWriter, r *http.Request, err error) {
	if writeDomainError(w, tagErrors, err) {
		return
	}
	httplog.SetAttrs(r.Context(), slog.String("tags_error", err.Error()))
	writeError(w, http.StatusInternalServerError, "internal error")
}

// TagStore is the minimal surface internal/httpserver needs from the tags
// domain -- narrower than *tags.Service so a stub can implement it in
// tests. Mirrors watchlist.Store / SettingsStore.
type TagStore interface {
	Attach(ctx context.Context, entryID int64, name string) (tags.AttachResult, error)
	Detach(ctx context.Context, entryID, tagID int64) error
	List(ctx context.Context) ([]tags.Summary, error)
	Rename(ctx context.Context, id int64, name string) (tags.Tag, error)
	Delete(ctx context.Context, id int64) (int64, error)
	Merge(ctx context.Context, sourceID, targetID int64) (tags.Summary, error)
}

// WithTags supplies the tags store backing every tag route: attach/detach
// under /watchlist/{id}/tags and the /tags list, rename, merge and delete
// routes. Absent, they all answer 503, like WithSettings/WithStatus.
func WithTags(store TagStore) Option {
	return func(c *serverConfig) {
		c.tags = store
	}
}

// parseTagID validates a tag-id path segment like parseWatchlistID; param is
// "tag_id" on the watchlist-scoped routes and "id" on /tags/{id}.
func parseTagID(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid tag id: %q", raw)
	}
	return id, nil
}

// attachTagRequest is the request DTO for POST /watchlist/{id}/tags; the id
// comes from the path, so an over-posted one fails DisallowUnknownFields.
type attachTagRequest struct {
	Name string `json:"name"`
}

// tagResponse is the wire shape for a single attached/detached tag.
type tagResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// handleAttachTag implements POST /watchlist/{id}/tags (TAG-01).
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

	result, err := s.tags.Attach(r.Context(), id, req.Name)
	if err != nil {
		writeTagError(w, r, err)
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

// handleDetachTag implements DELETE /watchlist/{id}/tags/{tag_id}. Only an
// unknown entry is 404; a missing link is idempotent 204 (D-21).
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
	tagID, err := parseTagID(r, "tag_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	if err := s.tags.Detach(r.Context(), id, tagID); err != nil {
		writeTagError(w, r, err)
		return
	}

	// 204 carries no payload -- no Content-Type, no encoder call.
	w.WriteHeader(http.StatusNoContent)
}

// handleListTags implements GET /tags (TAG-05): the whole vocabulary with
// watched-only carrier counts, also the autocomplete source.
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	summaries, err := s.tags.List(r.Context())
	if err != nil {
		writeTagError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(summaries)
}

// renameTagRequest is the request DTO for PATCH /tags/{id}.
type renameTagRequest struct {
	Name string `json:"name"`
}

// tagCollisionResponse is the 409 body for a rename collision: everything
// the merge confirm dialog needs (D-22).
type tagCollisionResponse struct {
	Error                  string   `json:"error"`
	Code                   string   `json:"code"`
	Target                 tags.Tag `json:"target"`
	CarrierCountAfterMerge int64    `json:"carrier_count_after_merge"`
}

// deleteTagResponse is the 200 body for DELETE /tags/{id} (TAG-06): the
// watched-carrier count the delete removed, for the confirmation toast.
type deleteTagResponse struct {
	CarrierCount int64 `json:"carrier_count"`
}

// handleRenameTag implements PATCH /tags/{id} (TAG-05).
func (s *Server) handleRenameTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	id, err := parseTagID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTagBodyBytes)
	var req renameTagRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	renamed, err := s.tags.Rename(r.Context(), id, req.Name)
	var collision *tags.CollisionError
	if errors.As(err, &collision) {
		// Fixed text: CollisionError.Error() embeds the stored tag name.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(tagCollisionResponse{
			Error:                  "tag name already exists",
			Code:                   codeTagNameTaken,
			Target:                 collision.Target,
			CarrierCountAfterMerge: collision.CarrierCountAfterMerge,
		})
		return
	}
	if err != nil {
		writeTagError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tagResponse{ID: renamed.ID, Name: renamed.Name})
}

// handleDeleteTag implements DELETE /tags/{id} (TAG-06). Unlike detach, an
// unknown tag id is a genuine 404.
func (s *Server) handleDeleteTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	id, err := parseTagID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	count, err := s.tags.Delete(r.Context(), id)
	if err != nil {
		writeTagError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(deleteTagResponse{CarrierCount: count})
}

// mergeTagRequest is the request DTO for POST /tags/{id}/merge.
type mergeTagRequest struct {
	Into int64 `json:"into"`
}

// handleMergeTag implements POST /tags/{id}/merge (D-19), the confirmed
// counterpart to a 409 rename collision. {id} is the source; into the target.
func (s *Server) handleMergeTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	id, err := parseTagID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid tag id")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxTagBodyBytes)
	var req mergeTagRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Into < 1 {
		writeError(w, http.StatusBadRequest, "invalid target tag id")
		return
	}

	summary, err := s.tags.Merge(r.Context(), id, req.Into)
	if err != nil {
		writeTagError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(summary)
}
