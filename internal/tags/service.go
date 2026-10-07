package tags

import (
	"context"
	"errors"
	"fmt"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Constraint names the package matches; each literal lives only here.
const (
	tagCapConstraint     = "artist_tags_max_per_artist"
	tagNameLengthCheck   = "tags_name_length"
	tagNameTrimmedCheck  = "tags_name_trimmed"
	tagNameLowerUniqueIx = "tags_name_lower_idx"
)

var (
	// ErrEntryNotFound is returned when the watchlist entry id does not
	// exist (D-20).
	ErrEntryNotFound = errors.New("watchlist entry not found")
	// ErrTagNotFound is returned by tag-id-addressed operations (rename,
	// merge, delete) when the id does not exist.
	ErrTagNotFound = errors.New("tag not found")
	// ErrTagCapReached is returned when an attach would exceed
	// MaxTagsPerArtist (ADR 0004).
	ErrTagCapReached = fmt.Errorf("artist already has the maximum of %d tags", MaxTagsPerArtist)
	// ErrMergeIntoSelf is returned when Merge's source and target ids are
	// the same.
	ErrMergeIntoSelf = errors.New("cannot merge a tag into itself")
)

// MaxTagsPerArtist mirrors the trigger's literal cap in migration 000010 (ADR 0004).
const MaxTagsPerArtist = 10

// Tag is the API-facing shape of a tags row.
type Tag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AttachResult reports the attached tag plus whether the link was newly
// created (201) or already existed (200, D-21).
type AttachResult struct {
	Tag     Tag
	Created bool
}

// Summary is the vocabulary-list shape of a tag: its identity plus how many
// currently-watched artists carry it (D-11).
type Summary struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	CarrierCount int64  `json:"carrier_count"`
}

// CollisionError is returned by Rename when the new name matches a different
// tag (D-22). Target keeps its stored casing; CarrierCountAfterMerge is what
// a confirmed merge would return.
type CollisionError struct {
	Target                 Tag
	CarrierCountAfterMerge int64
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("tag name already exists: %q (id %d)", e.Target.Name, e.Target.ID)
}

// DB is the seam Service needs beyond sqlc's DBTX: a transaction starter.
// *pgxpool.Pool satisfies it.
type DB interface {
	sqlc.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service is the sqlc-backed implementation of the tags domain.
type Service struct {
	db DB
	q  *sqlc.Queries
}

// NewService builds a Service backed by db.
func NewService(db DB) *Service {
	return &Service{db: db, q: sqlc.New(db)}
}

// Attach gets-or-creates the tag and links it to entryID's artist in one
// transaction, so a link refused by the cap trigger leaves no orphan tag (D-29).
func (s *Service) Attach(ctx context.Context, entryID int64, name string) (AttachResult, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return AttachResult{}, err
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return AttachResult{}, fmt.Errorf("begin attach tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	artistID, err := qtx.GetWatchlistArtistID(ctx, entryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AttachResult{}, ErrEntryNotFound
		}
		return AttachResult{}, fmt.Errorf("resolve watchlist artist: %w", err)
	}

	tag, err := qtx.GetOrCreateTag(ctx, normalized)
	if err != nil {
		return AttachResult{}, mapTagError(err)
	}

	affected, err := qtx.AttachTag(ctx, sqlc.AttachTagParams{ArtistID: artistID, TagID: tag.ID})
	if err != nil {
		return AttachResult{}, mapTagError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return AttachResult{}, fmt.Errorf("commit attach tx: %w", err)
	}

	return AttachResult{Tag: Tag{ID: tag.ID, Name: tag.Name}, Created: affected == 1}, nil
}

// Detach removes a tag link; an already-gone link is success (D-21). An
// unknown entry id is still ErrEntryNotFound.
func (s *Service) Detach(ctx context.Context, entryID, tagID int64) error {
	artistID, err := s.q.GetWatchlistArtistID(ctx, entryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrEntryNotFound
		}
		return fmt.Errorf("resolve watchlist artist: %w", err)
	}

	if _, err := s.q.DetachTag(ctx, sqlc.DetachTagParams{ArtistID: artistID, TagID: tagID}); err != nil {
		return fmt.Errorf("detach tag: %w", err)
	}
	return nil
}

// List returns the whole vocabulary ordered by lower(name) then id, with
// watched-only carrier counts, zero-count tags included (D-11). Never nil.
func (s *Service) List(ctx context.Context) ([]Summary, error) {
	rows, err := s.q.ListTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	summaries := make([]Summary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, Summary{ID: row.ID, Name: row.Name, CarrierCount: row.CarrierCount})
	}
	return summaries, nil
}

// Rename detects a collision from the tags_name_lower_idx unique violation,
// never a pre-check (D-22); a case-only rename of the same row cannot collide.
func (s *Service) Rename(ctx context.Context, id int64, name string) (Tag, error) {
	normalized, err := NormalizeName(name)
	if err != nil {
		return Tag{}, err
	}

	row, err := s.q.RenameTag(ctx, sqlc.RenameTagParams{ID: id, Name: normalized})
	if err == nil {
		return Tag{ID: row.ID, Name: row.Name}, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Tag{}, ErrTagNotFound
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation || pgErr.ConstraintName != tagNameLowerUniqueIx {
		return Tag{}, mapTagError(err)
	}

	target, terr := s.q.GetTagByName(ctx, normalized)
	if terr != nil {
		if !errors.Is(terr, pgx.ErrNoRows) {
			return Tag{}, fmt.Errorf("resolve rename collision target: %w", terr)
		}
		// The colliding tag vanished between the two statements (a
		// concurrent delete/merge) -- the name is free again, so retry the
		// rename once rather than reporting a stale collision.
		retryRow, retryErr := s.q.RenameTag(ctx, sqlc.RenameTagParams{ID: id, Name: normalized})
		if retryErr != nil {
			if errors.Is(retryErr, pgx.ErrNoRows) {
				return Tag{}, ErrTagNotFound
			}
			return Tag{}, mapTagError(retryErr)
		}
		return Tag{ID: retryRow.ID, Name: retryRow.Name}, nil
	}

	count, cerr := s.q.CountCarriersForTags(ctx, []int64{id, target.ID})
	if cerr != nil {
		return Tag{}, fmt.Errorf("count carriers after rename collision: %w", cerr)
	}

	return Tag{}, &CollisionError{
		Target:                 Tag{ID: target.ID, Name: target.Name},
		CarrierCountAfterMerge: count,
	}
}

// Delete removes the tag and every link, removed artists' included, and
// returns the watched carrier count for the confirmation toast (TAG-06).
func (s *Service) Delete(ctx context.Context, id int64) (int64, error) {
	row, err := s.q.DeleteTagCountingCarriers(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrTagNotFound
		}
		return 0, fmt.Errorf("delete tag: %w", err)
	}
	return row.CarrierCount, nil
}

// Merge moves sourceID's links onto targetID, keeps the target's casing and
// deletes the source in one transaction, never inserting links (ADR 0004).
func (s *Service) Merge(ctx context.Context, sourceID, targetID int64) (Summary, error) {
	if sourceID == targetID {
		return Summary{}, ErrMergeIntoSelf
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("begin merge tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	qtx := s.q.WithTx(tx)

	locked, err := qtx.LockTagsForMerge(ctx, []int64{sourceID, targetID})
	if err != nil {
		return Summary{}, fmt.Errorf("lock merge tags: %w", err)
	}
	var target *sqlc.LockTagsForMergeRow
	sourceFound := false
	for i := range locked {
		switch locked[i].ID {
		case sourceID:
			sourceFound = true
		case targetID:
			target = &locked[i]
		}
	}
	if !sourceFound || target == nil {
		return Summary{}, ErrTagNotFound
	}

	if _, err := qtx.DeleteDuplicateSourceLinks(ctx, sqlc.DeleteDuplicateSourceLinksParams{SourceID: sourceID, TargetID: targetID}); err != nil {
		return Summary{}, fmt.Errorf("delete duplicate source links: %w", err)
	}
	if _, err := qtx.RepointSourceLinks(ctx, sqlc.RepointSourceLinksParams{SourceID: sourceID, TargetID: targetID}); err != nil {
		return Summary{}, fmt.Errorf("repoint source links: %w", err)
	}
	if _, err := qtx.DeleteTag(ctx, sourceID); err != nil {
		return Summary{}, fmt.Errorf("delete source tag: %w", err)
	}

	count, err := qtx.CountCarriers(ctx, targetID)
	if err != nil {
		return Summary{}, fmt.Errorf("count carriers after merge: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Summary{}, fmt.Errorf("commit merge tx: %w", err)
	}

	return Summary{ID: target.ID, Name: target.Name, CarrierCount: count}, nil
}

// mapTagError translates a Postgres constraint violation into a typed
// sentinel: the cap trigger's check_violation, plus the tags table's own
// length/trim CHECKs (reachable only if normalization is bypassed).
func mapTagError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("tag operation: %w", err)
	}
	switch {
	case pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == tagCapConstraint:
		return ErrTagCapReached
	case pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == tagNameLengthCheck:
		return ErrNameTooLong
	case pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == tagNameTrimmedCheck:
		return ErrNameInvalid
	default:
		return fmt.Errorf("tag operation: %w", err)
	}
}
