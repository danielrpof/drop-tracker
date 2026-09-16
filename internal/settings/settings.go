// Package settings implements the digest notification settings resource: a
// narrow Get/Update seam over the singleton notification_settings row,
// mirroring internal/pollruns.Store's narrow-seam shape. Service holds only
// a sqlc.Querier and no cached copy of the row -- every Get is a fresh
// single-row query, which is what makes "takes effect without a restart"
// true by construction rather than by invalidation logic (D-05).
package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// Cadence is the digest send frequency. The two constants below are the
// only values the digest_cadence column's CHECK constraint
// (internal/db/migrations/000008_notification_settings.up.sql) allows.
type Cadence string

const (
	CadenceDaily  Cadence = "daily"
	CadenceWeekly Cadence = "weekly"
)

// ErrInvalidCadence is returned by ParseCadence for any value outside
// CadenceDaily/CadenceWeekly. It wraps only the client-supplied value, so
// echoing it leaks nothing -- mirrors watchlist.ErrInvalidReleaseType.
var ErrInvalidCadence = errors.New("invalid digest cadence")

// ParseCadence validates s against the allow-list the column's CHECK also
// enforces. This is the middle of three independent validation layers (the
// HTTP handler resolves it first, this re-validates it, the CHECK is the
// final backstop) -- a non-HTTP caller of Service.Update cannot bypass it.
func ParseCadence(s string) (Cadence, error) {
	switch Cadence(s) {
	case CadenceDaily:
		return CadenceDaily, nil
	case CadenceWeekly:
		return CadenceWeekly, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidCadence, s)
	}
}

// Settings is the API-facing shape of the singleton notification_settings
// row. DigestLastSlotAt is D-13's slot record (whether a digest send is
// due) -- deliberately absent from internal/httpserver's settingsResponse,
// which keeps exposing only the four wire-contract fields it always has.
type Settings struct {
	DigestEnabled    bool
	DigestCadence    Cadence
	DigestLastSentAt *time.Time
	DigestLastSlotAt *time.Time
	UpdatedAt        time.Time
}

// UpdateParams carries a full-object update: both fields are always
// supplied, matching the PUT semantics the HTTP boundary exposes (no
// partial-update ambiguity to resolve here, unlike watchlist preferences).
type UpdateParams struct {
	DigestEnabled bool
	DigestCadence Cadence
}

// Store is the minimal surface internal/httpserver needs for the digest
// settings resource -- mirrors internal/pollruns.Store's narrow Get/Update
// seam.
type Store interface {
	Get(ctx context.Context) (Settings, error)
	Update(ctx context.Context, p UpdateParams) (Settings, error)
}

// Service is the sqlc-backed implementation of Store.
type Service struct {
	q   sqlc.Querier
	loc *time.Location
	now func() time.Time
}

// Option customises a Service at construction.
type Option func(*Service)

// WithClock overrides the clock Update's re-anchor computation reads,
// mirroring notifier.Option's shape -- test injection only (D-15); NewService
// defaults now to time.Now.
func WithClock(now func() time.Time) Option {
	return func(s *Service) { s.now = now }
}

// NewService builds a Service backed by q. loc is a required positional
// parameter, not an Option -- mirroring the identical reasoning
// notifier.New already records for its SettingsReader parameter: a
// forgotten option would silently ship a service scheduling against the
// wrong zone, and a wrong-zone schedule is exactly the silent failure D-23
// exists to prevent.
func NewService(q sqlc.Querier, loc *time.Location, opts ...Option) *Service {
	s := &Service{q: q, loc: loc, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var _ Store = (*Service)(nil)

// Get reads the singleton row. On a from-scratch database this is D-05's
// seeded default row, never pgx.ErrNoRows.
func (s *Service) Get(ctx context.Context) (Settings, error) {
	row, err := s.q.GetNotificationSettings(ctx)
	if err != nil {
		return Settings{}, fmt.Errorf("get notification settings: %w", err)
	}
	return toSettings(row), nil
}

// Update re-validates p.DigestCadence before touching the database, so a
// non-HTTP caller cannot bypass the handler's own check -- the column's
// CHECK is the third layer, not the first.
func (s *Service) Update(ctx context.Context, p UpdateParams) (Settings, error) {
	cadence, err := ParseCadence(string(p.DigestCadence))
	if err != nil {
		return Settings{}, err
	}

	// D-14: the re-anchor slot is always computed and supplied here; the
	// SQL CASE (queries/notification_settings.sql) is what decides whether
	// it is actually applied -- enabling digest mode, or changing cadence
	// while it is already on, re-anchors digest_last_slot_at to this
	// instant, and every other update path leaves the stored value
	// untouched.
	slot := MostRecentSlot(s.now(), cadence, s.loc)

	row, err := s.q.UpdateNotificationSettings(ctx, sqlc.UpdateNotificationSettingsParams{
		DigestEnabled: p.DigestEnabled,
		DigestCadence: string(cadence),
		ReanchorSlot:  pgtype.Timestamptz{Time: slot, Valid: true},
	})
	if err != nil {
		return Settings{}, fmt.Errorf("update notification settings: %w", err)
	}
	return toSettings(row), nil
}

// toSettings maps a generated row onto the API-facing shape.
// DigestLastSentAt/DigestLastSlotAt are nil when their column is NULL
// (D-05: nothing writes DigestLastSentAt in this phase; DigestLastSlotAt is
// NULL until the first re-anchoring Update or the first due-check writes
// it).
func toSettings(row sqlc.NotificationSetting) Settings {
	var lastSent *time.Time
	if row.DigestLastSentAt.Valid {
		t := row.DigestLastSentAt.Time
		lastSent = &t
	}
	var lastSlot *time.Time
	if row.DigestLastSlotAt.Valid {
		t := row.DigestLastSlotAt.Time
		lastSlot = &t
	}
	return Settings{
		DigestEnabled:    row.DigestEnabled,
		DigestCadence:    Cadence(row.DigestCadence),
		DigestLastSentAt: lastSent,
		DigestLastSlotAt: lastSlot,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}
