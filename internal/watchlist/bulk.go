package watchlist

import (
	"context"
	"fmt"
	"slices"
)

// BulkRemoveResult is the per-id outcome of a bulk remove.
type BulkRemoveResult struct {
	Removed     []int64
	AlreadyGone []int64
}

// missingIDs returns the distinct requested ids absent from found, ascending
// and never nil.
func missingIDs(requested, found []int64) []int64 {
	have := make(map[int64]struct{}, len(found))
	for _, id := range found {
		have[id] = struct{}{}
	}
	missing := []int64{}
	seen := make(map[int64]struct{}, len(requested))
	for _, id := range requested {
		if _, ok := have[id]; ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		missing = append(missing, id)
	}
	slices.Sort(missing)
	return missing
}

// BulkRemove hard-deletes every listed entry in one statement and reports ids
// that no longer existed as already gone (D-03). Artists, tags and pending
// events are untouched.
func (s *Service) BulkRemove(ctx context.Context, ids []int64) (BulkRemoveResult, error) {
	if len(ids) == 0 {
		return BulkRemoveResult{Removed: []int64{}, AlreadyGone: []int64{}}, nil
	}
	removed, err := s.q.BulkDeleteWatchlist(ctx, ids)
	if err != nil {
		return BulkRemoveResult{}, fmt.Errorf("bulk delete watchlist: %w", err)
	}
	if removed == nil {
		removed = []int64{}
	}
	slices.Sort(removed)
	return BulkRemoveResult{Removed: removed, AlreadyGone: missingIDs(ids, removed)}, nil
}
