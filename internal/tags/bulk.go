package tags

import (
	"context"
	"fmt"
	"slices"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// Skip reasons a bulk operation reports per entry.
const (
	SkipReasonCap  = "cap"
	SkipReasonGone = "gone"
)

// BulkSkip is one selected entry a bulk operation did not apply. Name is set
// only for cap skips, so the UI can name the full artist (D-01).
type BulkSkip struct {
	ID     int64
	Name   string
	Reason string
}

// BulkAttachResult is the outcome of BulkAttach. Tag.ID is 0 when nothing was
// applied, because the transaction rolled back.
type BulkAttachResult struct {
	Tag     Tag
	Applied []int64
	Skipped []BulkSkip
}

// BulkAttach adds one tag to many entries' artists in one transaction. The
// ordered pre-lock keeps the cap race and deadlocks out (ADR 0004 amendment).
func (s *Service) BulkAttach(ctx context.Context, entryIDs []int64, name string) (BulkAttachResult, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return BulkAttachResult{}, err
	}
	result := BulkAttachResult{Applied: []int64{}, Skipped: []BulkSkip{}}
	if len(entryIDs) == 0 {
		return result, nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return BulkAttachResult{}, fmt.Errorf("begin bulk attach tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	tag, err := qtx.GetOrCreateTag(ctx, normalized)
	if err != nil {
		return BulkAttachResult{}, mapTagError(err)
	}
	locked, err := qtx.LockWatchlistArtistsForBulkTag(ctx, entryIDs)
	if err != nil {
		return BulkAttachResult{}, mapTagError(err)
	}

	found := make(map[int64]struct{}, len(locked))
	entryByArtist := make(map[int64]sqlc.LockWatchlistArtistsForBulkTagRow, len(locked))
	artistIDs := make([]int64, 0, len(locked))
	for _, row := range locked {
		found[row.EntryID] = struct{}{}
		entryByArtist[row.ArtistID] = row
		artistIDs = append(artistIDs, row.ArtistID)
	}
	seen := make(map[int64]struct{}, len(entryIDs))
	for _, id := range entryIDs {
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := found[id]; !ok {
			result.Skipped = append(result.Skipped, BulkSkip{ID: id, Reason: SkipReasonGone})
		}
	}

	if len(artistIDs) > 0 {
		outcomes, err := qtx.BulkAttachTag(ctx, sqlc.BulkAttachTagParams{ArtistIds: artistIDs, TagID: tag.ID})
		if err != nil {
			return BulkAttachResult{}, mapTagError(err)
		}
		for _, o := range outcomes {
			row := entryByArtist[o.ArtistID]
			if o.HadTag || o.Inserted {
				result.Applied = append(result.Applied, row.EntryID)
				continue
			}
			result.Skipped = append(result.Skipped, BulkSkip{ID: row.EntryID, Name: row.Name, Reason: SkipReasonCap})
		}
	}

	slices.Sort(result.Applied)
	slices.SortFunc(result.Skipped, func(a, b BulkSkip) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})

	if len(result.Applied) == 0 {
		// No commit: the rollback discards a tag created on the fly (D-19).
		result.Tag = Tag{Name: tag.Name}
		return result, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return BulkAttachResult{}, fmt.Errorf("commit bulk attach tx: %w", err)
	}
	result.Tag = Tag{ID: tag.ID, Name: tag.Name}
	return result, nil
}
