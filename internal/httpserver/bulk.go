package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const (
	// maxBulkIDs and maxBulkBodyBytes bound every bulk request before the store runs.
	maxBulkIDs       = 1000
	maxBulkBodyBytes = 65536
)

const (
	codeBulkIDsRequired = "bulk_ids_required"
	codeBulkTooManyIDs  = "bulk_too_many_ids"
	codeBulkInvalidID   = "bulk_invalid_id"
)

var (
	errBulkIDsRequired = errors.New("bulk ids required")
	errBulkTooManyIDs  = errors.New("bulk too many ids")
	errBulkInvalidID   = errors.New("bulk invalid id")
)

// WatchlistBulkStore is the narrow surface the bulk watchlist routes need.
type WatchlistBulkStore interface {
	BulkRemove(ctx context.Context, ids []int64) (watchlist.BulkRemoveResult, error)
}

// WithWatchlistBulk supplies the bulk store. Absent, the bulk watchlist routes
// answer 503, like WithTags.
func WithWatchlistBulk(store WatchlistBulkStore) Option {
	return func(c *serverConfig) {
		c.watchlistBulk = store
	}
}

// validateBulkIDs bounds the raw array, rejects ids below 1 and dedupes to
// first occurrence.
func validateBulkIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, errBulkIDsRequired
	}
	if len(ids) > maxBulkIDs {
		return nil, errBulkTooManyIDs
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id < 1 {
			return nil, errBulkInvalidID
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// bulkTooManyResponse carries max so the SPA never hardcodes the bound.
type bulkTooManyResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Max   int    `json:"max"`
}

// writeBulkIDsError maps a validateBulkIDs error to its 400 body.
func writeBulkIDsError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errBulkTooManyIDs):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(bulkTooManyResponse{Error: "too many ids", Code: codeBulkTooManyIDs, Max: maxBulkIDs})
	case errors.Is(err, errBulkInvalidID):
		writeErrorCode(w, http.StatusBadRequest, codeBulkInvalidID, "invalid watchlist id")
	default:
		writeErrorCode(w, http.StatusBadRequest, codeBulkIDsRequired, "ids are required")
	}
}

type bulkDeleteRequest struct {
	IDs []int64 `json:"ids"`
}

type bulkDeleteResponse struct {
	Removed     []int64 `json:"removed"`
	AlreadyGone []int64 `json:"already_gone"`
}

// handleBulkDelete implements POST /watchlist/bulk/delete (BULK-04).
func (s *Server) handleBulkDelete(w http.ResponseWriter, r *http.Request) {
	if s.watchlistBulk == nil {
		writeError(w, http.StatusServiceUnavailable, "watchlist bulk not available")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBulkBodyBytes)
	var req bulkDeleteRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ids, err := validateBulkIDs(req.IDs)
	if err != nil {
		writeBulkIDsError(w, err)
		return
	}

	result, err := s.watchlistBulk.BulkRemove(r.Context(), ids)
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("watchlist_bulk_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := bulkDeleteResponse{Removed: result.Removed, AlreadyGone: result.AlreadyGone}
	if resp.Removed == nil {
		resp.Removed = []int64{}
	}
	if resp.AlreadyGone == nil {
		resp.AlreadyGone = []int64{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// bulkSkipWire is one skipped selection; name is set only for cap skips.
type bulkSkipWire struct {
	ID     int64  `json:"id"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason"`
}

// toSkipWire converts skips to the wire shape and never returns nil.
func toSkipWire(skips []tags.BulkSkip) []bulkSkipWire {
	out := make([]bulkSkipWire, 0, len(skips))
	for _, sk := range skips {
		out = append(out, bulkSkipWire{ID: sk.ID, Name: sk.Name, Reason: sk.Reason})
	}
	return out
}

type bulkAddTagRequest struct {
	IDs  []int64 `json:"ids"`
	Name string  `json:"name"`
}

type bulkAddTagResponse struct {
	Tag     tagResponse    `json:"tag"`
	Applied []int64        `json:"applied"`
	Skipped []bulkSkipWire `json:"skipped"`
}

// handleBulkAddTag implements POST /watchlist/bulk/tags/add (BULK-02).
func (s *Server) handleBulkAddTag(w http.ResponseWriter, r *http.Request) {
	if s.tags == nil {
		writeError(w, http.StatusServiceUnavailable, "tags not available")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBulkBodyBytes)
	var req bulkAddTagRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ids, err := validateBulkIDs(req.IDs)
	if err != nil {
		writeBulkIDsError(w, err)
		return
	}

	result, err := s.tags.BulkAttach(r.Context(), ids, req.Name)
	if err != nil {
		writeTagError(w, r, err)
		return
	}

	applied := result.Applied
	if applied == nil {
		applied = []int64{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(bulkAddTagResponse{
		Tag:     tagResponse{ID: result.Tag.ID, Name: result.Tag.Name},
		Applied: applied,
		Skipped: toSkipWire(result.Skipped),
	})
}
