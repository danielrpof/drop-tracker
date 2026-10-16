package watchlist

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
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

// ErrPreferenceConflict is returned when one type is both set and unset on
// the same axis.
var ErrPreferenceConflict = errors.New("a type cannot be both set and unset")

// AxisDelta names the types to add to and remove from one preference axis.
type AxisDelta struct {
	Set   []string
	Unset []string
}

// PreferencesDelta is a per-type change applied to many entries (D-05).
type PreferencesDelta struct {
	ReleaseTypes    AxisDelta
	MutedEventTypes AxisDelta
}

// BulkUpdateResult is the per-id outcome of a bulk preferences update.
type BulkUpdateResult struct {
	Applied []int64
	Gone    []int64
}

func normalizeAxis(d AxisDelta, allowed []string, invalidErr error) (set, unset []string, err error) {
	if set, err = normalizeSet(d.Set, allowed, invalidErr); err != nil {
		return nil, nil, err
	}
	if unset, err = normalizeSet(d.Unset, allowed, invalidErr); err != nil {
		return nil, nil, err
	}
	return set, unset, nil
}

func axesOverlap(set, unset []string) bool {
	for _, t := range set {
		if slices.Contains(unset, t) {
			return true
		}
	}
	return false
}

// BulkUpdatePreferences applies a per-type delta to every listed entry in one
// statement (D-05). Validation runs before the empty-ids shortcut so a bad
// delta is rejected regardless of ids (D-21).
func (s *Service) BulkUpdatePreferences(ctx context.Context, ids []int64, d PreferencesDelta) (BulkUpdateResult, error) {
	relSet, relUnset, err := normalizeAxis(d.ReleaseTypes, ReleaseTypes, ErrInvalidReleaseType)
	if err != nil {
		return BulkUpdateResult{}, err
	}
	evSet, evUnset, err := normalizeAxis(d.MutedEventTypes, EventTypes, ErrInvalidEventType)
	if err != nil {
		return BulkUpdateResult{}, err
	}
	if len(relSet)+len(relUnset)+len(evSet)+len(evUnset) == 0 {
		return BulkUpdateResult{}, ErrNoPreferencesSupplied
	}
	if axesOverlap(relSet, relUnset) || axesOverlap(evSet, evUnset) {
		return BulkUpdateResult{}, ErrPreferenceConflict
	}
	if len(ids) == 0 {
		return BulkUpdateResult{Applied: []int64{}, Gone: []int64{}}, nil
	}

	applied, err := s.q.BulkUpdateWatchlistPreferences(ctx, sqlc.BulkUpdateWatchlistPreferencesParams{
		Ids:               ids,
		AllReleaseTypes:   ReleaseTypes,
		SetReleaseTypes:   relSet,
		UnsetReleaseTypes: relUnset,
		AllEventTypes:     EventTypes,
		SetEventTypes:     evSet,
		UnsetEventTypes:   evUnset,
	})
	if err != nil {
		return BulkUpdateResult{}, fmt.Errorf("bulk update watchlist preferences: %w", err)
	}
	if applied == nil {
		applied = []int64{}
	}
	slices.Sort(applied)
	return BulkUpdateResult{Applied: applied, Gone: missingIDs(ids, applied)}, nil
}
