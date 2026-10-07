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

// WithTags supplies the tags domain dependency backing POST
// /watchlist/{id}/tags and DELETE /watchlist/{id}/tags/{tag_id}. Absent,
// the handlers answer 503 with the shared fixed error body, mirroring
// WithSettings/WithStatus; a cmd/server binary always wires it.
func WithTags(store TagStore) Option {
	return func(c *serverConfig) {
		c.tags = store
	}
}

// parseTagID reads and validates a tag-id path segment named param the same
// way parseWatchlistID validates {id} -- tags.id is BIGSERIAL, so 0 and
// negatives are never valid. param is "tag_id" on the watchlist-scoped
// attach/detach routes and "id" on the /tags/{id} vocabulary routes.
func parseTagID(r *http.Request, param string) (int64, error) {
	raw := chi.URLParam(r, param)
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
// tags.Service validates the name.
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

// tagCollisionResponse is the 409 body for a rename whose normalized name
// collides with a different existing tag (D-22): everything the merge
// confirm dialog needs, verbatim (24-UI-SPEC.md Copywriting).
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

// handleRenameTag implements PATCH /tags/{id} (TAG-05, D-09, D-22, D-23).
// tags.Service validates the name.
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

// handleDeleteTag implements DELETE /tags/{id} (TAG-06, D-11, SC4): removes
// the tag from every artist (including removed ones) and reports the
// watched-carrier count. Unlike detach, a missing tag id is a genuine 404 --
// deleting a tag that never existed is not an idempotent no-op.
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

// handleMergeTag implements POST /tags/{id}/merge (TAG-05, D-19, D-22,
// D-23): the confirmed-merge counterpart to a 409 rename collision. {id} is
// the source tag; into is the target.
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
