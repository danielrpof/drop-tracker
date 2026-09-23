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

// tagCapConstraint is the check_violation constraint name migration 000010's
// trigger raises at 10 links (ADR 0004, D-18). Kept as a const next to the
// mapping below so the literal never drifts from the migration.
const tagCapConstraint = "artist_tags_max_per_artist"

var (
	// ErrEntryNotFound is returned when the watchlist entry id does not
	// exist (D-20).
	ErrEntryNotFound = errors.New("watchlist entry not found")
	// ErrTagNotFound is reserved for tag-id-addressed operations (rename,
	// merge, delete) added in a later plan.
	ErrTagNotFound = errors.New("tag not found")
	// ErrTagCapReached is returned when an attach would exceed
	// MaxTagsPerArtist (TAG-04, ADR 0004).
	ErrTagCapReached = errors.New("artist already has the maximum of 10 tags")
)

// MaxTagsPerArtist mirrors the artist_tags_cap_trigger's cap (migration
// 000010) -- the Go-side constant documenting the same number the database
// enforces, not a second source of truth (the trigger is non-bypassable
// regardless of what this side believes).
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

// Attach normalizes name, then in one transaction: resolves entryID's
// artist, gets-or-creates the tag (D-29), and links it (the cap trigger
// does the enforcement, ADR 0004). The get-or-create and the link insert
// share the transaction so a refused link never leaves a new orphan tag
// behind.
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

// Detach removes a tag link. Any affected count is success -- 0 rows means
// the link was already gone, which D-21 treats as idempotent success, not
// an error. An unknown entry id is still ErrEntryNotFound.
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
	case pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == "tags_name_length":
		return ErrNameTooLong
	case pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == "tags_name_trimmed":
		return ErrNameInvalid
	default:
		return fmt.Errorf("tag operation: %w", err)
	}
}
