// Package detection implements the diff-based event detection Phase 4 owns
// (DTCT-01 through DTCT-05): it diffs a poll cycle's freshly fetched
// results against the events table -- the "seen" store (D-09) -- and
// records each previously-unseen item as an event row, idempotently at the
// database level via InsertEvent's ON CONFLICT DO NOTHING (D-20). Detector
// implements poller.EventRecorder, the narrow seam RunMusicBrainzCycle
// calls at the end of its per-artist loop.
package detection

import (
	"context"
	"fmt"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// Detector wraps sqlc.Querier -- the consuming package declares its own
// narrower interface (poller.EventRecorder) rather than this type
// depending on one, mirroring watchlist.Store/Service's split.
type Detector struct {
	q sqlc.Querier
}

// New builds a Detector backed by q.
func New(q sqlc.Querier) *Detector {
	return &Detector{q: q}
}

// insertEvent calls InsertEvent and reports whether the row was newly
// inserted. 0 affected rows is not an error -- it means the
// (event_type, source, external_id) dedup key already existed (D-20), i.e.
// "already seen," not "something went wrong."
func (d *Detector) insertEvent(ctx context.Context, params sqlc.InsertEventParams) (newlyDetected bool, err error) {
	affected, err := d.q.InsertEvent(ctx, params)
	if err != nil {
		return false, fmt.Errorf("detection: insert event: %w", err)
	}
	return affected > 0, nil
}

// nullableString turns an empty string into a nil *string. MusicBrainz
// returns "" (never omits the field) for a group's undated
// first-release-date, and this project's *string column convention treats
// SQL NULL, not an empty string literal, as "no value."
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
