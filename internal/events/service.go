// Package events implements the release-history domain (HIST-01, UI-03): a
// single global, keyset-paginated, artist/event-type-filterable read over
// the events table. It mirrors internal/watchlist/service.go's Store /
// Service / NewService shape exactly, so internal/httpserver can depend on a
// narrow interface instead of sqlc.Querier directly.
package events

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// DefaultPageSize is the page size used when a caller supplies zero or a
// negative PageSize -- 06-UI-SPEC.md locks 24 events per History feed fetch
// (D-07).
const DefaultPageSize = 24

// MaxPageSize is the hard ceiling List clamps PageSize to, regardless of
// what a caller (HTTP or otherwise) requests -- T-06-02's DoS mitigation:
// the clamp lives in the domain service, not the HTTP boundary, so no caller
// can request an unbounded scan.
const MaxPageSize = 100

// Event is the API-facing shape of one events row. Field names and JSON
// tags mirror the events table's column names (internal/db/sqlc/models.go),
// so the wire shape is a direct, unsurprising projection of the schema.
type Event struct {
	ID                 int64      `json:"id"`
	ArtistID           int64      `json:"artist_id"`
	Source             string     `json:"source"`
	EventType          string     `json:"event_type"`
	ExternalID         string     `json:"external_id"`
	ReleaseGroupMBID   *string    `json:"release_group_mbid"`
	Title              string     `json:"title"`
	ArtistName         string     `json:"artist_name"`
	ReleaseDate        *string    `json:"release_date"`
	CoverArtURL        *string    `json:"cover_art_url"`
	TrackCount         *int32     `json:"track_count"`
	PreviousTrackCount *int32     `json:"previous_track_count"`
	ReleaseType        *string    `json:"release_type"`
	NotifiedAt         *time.Time `json:"notified_at"`
	CreatedAt          time.Time  `json:"created_at"`
}

// ListParams carries the optional filter/pagination axes for List. A nil
// ArtistID/EventType/Cursor means "no filter on this axis" -- mirrors
// watchlist.PreferencesParams's nil-means-untouched convention. A PageSize
// of zero or less is clamped to DefaultPageSize by List.
type ListParams struct {
	ArtistID  *int64
	EventType *string
	Cursor    *int64
	PageSize  int32
}

// Page is one page of the history feed: Events is always non-nil (D-05).
// NextCursor is set to the last returned row's id only when the page came
// back completely full -- the client's unambiguous "no more pages" signal
// when it is nil.
type Page struct {
	Events     []Event
	NextCursor *int64
}

// Store is the minimal surface internal/httpserver needs for the events
// resource -- narrower than sqlc.Querier so a stub can implement it in
// tests without a live Postgres connection, mirroring watchlist.Store and
// httpserver.Pinger.
type Store interface {
	List(ctx context.Context, p ListParams) (Page, error)
}

// Service is the sqlc-backed implementation of Store.
type Service struct {
	q             sqlc.Querier
	retentionDays int
}

// NewService builds a Service backed by q, applying a retention window of
// retentionDays to every List call (DATA-02, D-01) -- the cutoff is computed
// here, in this domain service, not at the HTTP boundary and not as a
// SQL-side now() expression, matching this file's own "clamped here, not at
// the HTTP boundary" convention already established by the PageSize clamp
// below.
func NewService(q sqlc.Querier, retentionDays int) *Service {
	return &Service{q: q, retentionDays: retentionDays}
}

var _ Store = (*Service)(nil)

// List returns one page of the history feed, newest first (D-05), applying
// p's optional artist/event-type filters and cursor. PageSize is clamped
// here -- not at the HTTP boundary -- so no caller, HTTP or otherwise, can
// request an unbounded scan (T-06-02): zero or negative becomes
// DefaultPageSize, anything above MaxPageSize becomes MaxPageSize. The
// returned Events slice is always non-nil (make([]Event, 0, len(rows))), so
// an empty page encodes as JSON [] rather than null.
func (s *Service) List(ctx context.Context, p ListParams) (Page, error) {
	pageSize := p.PageSize
	switch {
	case pageSize <= 0:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}

	// The retention cutoff (DATA-02, D-01/D-04): events created before this
	// instant are hidden from the feed, but Valid is always explicitly true
	// -- following seedNotifiedAt's precedent (internal/detection/detector.go)
	// -- because a zero-value pgtype.Timestamptz marshals as SQL NULL, and
	// "created_at >= NULL" is never true, which would silently return an
	// empty feed for every request instead of erroring (T-10-02).
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)

	rows, err := s.q.ListEvents(ctx, sqlc.ListEventsParams{
		ArtistID:  p.ArtistID,
		EventType: p.EventType,
		Cursor:    p.Cursor,
		Cutoff:    pgtype.Timestamptz{Time: cutoff, Valid: true},
		PageSize:  pageSize,
	})
	if err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}

	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, toEvent(row))
	}

	var nextCursor *int64
	if len(rows) == int(pageSize) {
		last := out[len(out)-1].ID
		nextCursor = &last
	}

	return Page{Events: out, NextCursor: nextCursor}, nil
}

// toEvent converts one sqlc-generated ListEventsRow into the API-facing
// Event shape, converting pgtype.Timestamptz into time.Time (CreatedAt,
// always set) and *time.Time (NotifiedAt, nil until Phase 5's notifier acks
// the row).
func toEvent(row sqlc.ListEventsRow) Event {
	e := Event{
		ID:                 row.ID,
		ArtistID:           row.ArtistID,
		Source:             row.Source,
		EventType:          row.EventType,
		ExternalID:         row.ExternalID,
		ReleaseGroupMBID:   row.ReleaseGroupMbid,
		Title:              row.Title,
		ArtistName:         row.ArtistName,
		ReleaseDate:        row.ReleaseDate,
		CoverArtURL:        row.CoverArtUrl,
		TrackCount:         row.TrackCount,
		PreviousTrackCount: row.PreviousTrackCount,
		ReleaseType:        row.ReleaseType,
		CreatedAt:          row.CreatedAt.Time,
	}
	if row.NotifiedAt.Valid {
		t := row.NotifiedAt.Time
		e.NotifiedAt = &t
	}
	return e
}
