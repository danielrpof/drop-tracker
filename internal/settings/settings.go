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
// row.
type Settings struct {
	DigestEnabled    bool
	DigestCadence    Cadence
	DigestLastSentAt *time.Time
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
	q sqlc.Querier
}

// NewService builds a Service backed by q.
func NewService(q sqlc.Querier) *Service {
	return &Service{q: q}
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

	row, err := s.q.UpdateNotificationSettings(ctx, sqlc.UpdateNotificationSettingsParams{
		DigestEnabled: p.DigestEnabled,
		DigestCadence: string(cadence),
	})
	if err != nil {
		return Settings{}, fmt.Errorf("update notification settings: %w", err)
	}
	return toSettings(row), nil
}

// toSettings maps a generated row onto the API-facing shape.
// DigestLastSentAt is nil when the column is NULL (D-05: nothing in this
// phase ever writes it).
func toSettings(row sqlc.NotificationSetting) Settings {
	var lastSent *time.Time
	if row.DigestLastSentAt.Valid {
		t := row.DigestLastSentAt.Time
		lastSent = &t
	}
	return Settings{
		DigestEnabled:    row.DigestEnabled,
		DigestCadence:    Cadence(row.DigestCadence),
		DigestLastSentAt: lastSent,
		UpdatedAt:        row.UpdatedAt.Time,
	}
}
