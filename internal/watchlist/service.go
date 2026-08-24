// Package watchlist implements the watchlist domain: adding, listing,
// updating preferences for, and removing artists a user is tracking. It
// wraps the sqlc-generated Queries behind a narrow Store interface --
// internal/httpserver's Pinger-analog for this phase -- so handler tests
// can substitute a stub instead of a live Postgres connection.
package watchlist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/danielrpof/drop-tracker/internal/artistart"
	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// matchTimeout bounds the add-time artist-art match attempt Service.Add
// makes when a caller supplies no ImageURL (D-06). This bound is
// load-bearing, not decorative: artistart.Matcher.Match's tie-break path can
// reach the MusicBrainz limiter, a whole-process budget shared with the poll
// cycle and paced near one request per second (D-07), and
// cmd/server/main.go sets the HTTP server's write timeout to 15 seconds.
// Without this bound, a saturated limiter could turn an add request into a
// write-timeout failure instead of merely an add with no resolved art.
const matchTimeout = 8 * time.Second

// ReleaseTypes and EventTypes mirror the watchlist_release_types_valid and
// watchlist_muted_event_types_valid CHECK constraints
// (internal/db/migrations/000002_watchlist.up.sql) -- the single Go-side
// source of truth for both allow-lists.
var (
	ReleaseTypes = []string{"album", "single", "ep", "deluxe"}
	EventTypes   = []string{"new_release", "guest_feature", "deluxe_change"}
)

var (
	// ErrDuplicate is returned when an artist already on the watchlist is
	// added again (D-09).
	ErrDuplicate = errors.New("artist already on watchlist")
	// ErrNotFound is returned when a watchlist entry id does not exist.
	ErrNotFound = errors.New("watchlist entry not found")
	// ErrInvalidReleaseType is returned when a release type outside
	// ReleaseTypes is supplied.
	ErrInvalidReleaseType = errors.New("invalid release type")
	// ErrInvalidEventType is returned when an event type outside
	// EventTypes is supplied.
	ErrInvalidEventType = errors.New("invalid event type")
	// ErrNoPreferencesSupplied is returned when a preferences update
	// supplies neither axis: an update that changes nothing is a caller
	// mistake, and reporting success for it would claim a change that never
	// happened (WR-01, G-02-1).
	ErrNoPreferencesSupplied = errors.New("no preferences supplied")
)

// Entry is the API-facing joined artist + watchlist row.
type Entry struct {
	ID              int64     `json:"id"`
	ArtistID        int64     `json:"artist_id"`
	MBID            string    `json:"mbid"`
	Name            string    `json:"name"`
	DeezerID        *string   `json:"deezer_id"`
	Disambiguation  *string   `json:"disambiguation"`
	ImageURL        *string   `json:"image_url"`
	ReleaseTypes    []string  `json:"release_types"`
	MutedEventTypes []string  `json:"muted_event_types"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// AddParams carries the fields needed to add a new artist to the
// watchlist. ReleaseTypes and MutedEventTypes being nil means "apply the
// D-08 defaults" -- all release types enabled, nothing muted.
type AddParams struct {
	MBID            string
	Name            string
	DeezerID        *string
	Disambiguation  *string
	ImageURL        *string
	ReleaseTypes    []string
	MutedEventTypes []string
}

// PreferencesParams carries a partial preferences update. A nil field means
// "leave this axis untouched."
type PreferencesParams struct {
	ReleaseTypes    *[]string
	MutedEventTypes *[]string
}

// Store is the minimal surface internal/httpserver needs for the watchlist
// resource -- narrower than sqlc's generated Querier so a stub can
// implement it in tests without a live Postgres connection. This mirrors
// httpserver.Pinger (internal/httpserver/server.go).
type Store interface {
	Add(ctx context.Context, p AddParams) (Entry, error)
	List(ctx context.Context) ([]Entry, error)
	UpdatePreferences(ctx context.Context, id int64, p PreferencesParams) (Entry, error)
	Remove(ctx context.Context, id int64) error
}

// ArtistMatcher is the narrow seam Service depends on to resolve artist art
// at add time (D-06) -- declared here, in the consumer, mirroring
// detection.RecordingSource (internal/detection/detector.go), so tests
// substitute a stub instead of a concrete *artistart.Matcher. artistart must
// not import watchlist; this package is the only side of the dependency.
type ArtistMatcher interface {
	Match(ctx context.Context, mbid, name string) (artistart.Result, error)
}

// Option customizes a Service's construction, mirroring poller.Option's
// functional-option pattern (poller.New builds a Poller from its defaults
// and applies each supplied option in order). This is load-bearing for diff
// size: NewService has call sites in cmd/server/main.go and across
// internal/httpserver's test files, and a variadic option is what lets all
// of them compile unchanged when a new dependency is added.
type Option func(*Service)

// WithArtistArt supplies the add-time artist-art match dependency (D-06,
// D-10). gate is optional: nil means "no coordination with a backfill
// sweep" (e.g. in tests, or a hypothetical deployment with no backfill) --
// WithArtistArt never panics or nil-dereferences when handed one, and Add
// simply skips registering on it. logger falls back to slog.Default() when
// nil, since this option is useless without a way to report a failed or
// unresolved match rather than silently swallowing it.
func WithArtistArt(m ArtistMatcher, gate *artistart.ActivityGate, logger *slog.Logger) Option {
	if logger == nil {
		logger = slog.Default()
	}
	return func(s *Service) {
		s.matcher = m
		s.activityGate = gate
		s.logger = logger
	}
}

// Service is the sqlc-backed implementation of Store.
type Service struct {
	q sqlc.Querier

	// matcher, activityGate and logger are populated only by WithArtistArt.
	// matcher == nil is the "no art matching configured" state Add gates on
	// -- NewService(q) with no options behaves exactly as it did before D-06.
	matcher      ArtistMatcher
	activityGate *artistart.ActivityGate
	logger       *slog.Logger
}

// NewService builds a Service backed by q, applying each supplied Option in
// order. With no options, behavior is unchanged from before D-06: no art
// match attempt is ever made.
func NewService(q sqlc.Querier, opts ...Option) *Service {
	s := &Service{q: q}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

var _ Store = (*Service)(nil)

// Add upserts the artist by mbid, then creates its watchlist entry. The two
// writes are not wrapped in a transaction: under D-03, artists is master
// data whose lifetime is independent of any watchlist entry, so an artists
// row with no watchlist row is a legitimate state that Phase 4 and Phase 6
// will also produce.
func (s *Service) Add(ctx context.Context, p AddParams) (Entry, error) {
	// Validate both preference axes before any database call -- a rejected
	// request must leave no artists row behind (D-05, D-08, D-11). The two
	// axes are independent dimensions (catalog scope versus alert-noise
	// control) and are never merged into one validated set.
	var releaseTypes []string
	if p.ReleaseTypes == nil {
		releaseTypes = append([]string{}, ReleaseTypes...)
	} else {
		var err error
		releaseTypes, err = normalizeSet(p.ReleaseTypes, ReleaseTypes, ErrInvalidReleaseType)
		if err != nil {
			return Entry{}, err
		}
	}

	var mutedEventTypes []string
	if p.MutedEventTypes == nil {
		mutedEventTypes = []string{}
	} else {
		var err error
		mutedEventTypes, err = normalizeSet(p.MutedEventTypes, EventTypes, ErrInvalidEventType)
		if err != nil {
			return Entry{}, err
		}
	}

	// D-06: resolve artist art synchronously when the caller supplied none.
	// Gated on s.matcher != nil so a Service built with NewService(q) (no
	// options) never attempts a match. Art resolution can fail in any way
	// -- no match, upstream error, timeout -- without ever failing the add
	// (D-09): a Match error is logged and the add proceeds with no art.
	if s.matcher != nil && p.ImageURL == nil {
		// D-10: register on the shared ActivityGate for exactly the
		// duration of this match call, so a concurrently-running backfill
		// sweep (artistart.Backfill, sibling plan) can detect it and yield.
		// gate is optional -- nil simply means no coordination to perform.
		// cancel/end run via defer inside this closure (not as plain
		// statements after the call) so a panic inside Match still releases
		// both, without extending the gate's active window past the match
		// itself into the rest of Add -- an un-deferred end() would leave
		// the ActivityGate permanently active (WR-03, 13-REVIEW.md).
		res, matchErr := func() (artistart.Result, error) {
			matchCtx, cancel := context.WithTimeout(ctx, matchTimeout)
			defer cancel()
			if s.activityGate != nil {
				end := s.activityGate.Begin()
				defer end()
			}
			return s.matcher.Match(matchCtx, p.MBID, p.Name)
		}()

		switch {
		case matchErr != nil:
			s.logger.Warn("artist art match failed",
				slog.String("artist_mbid", p.MBID),
				slog.String("artist_name", p.Name),
				slog.String("error", matchErr.Error()),
			)
		case res.Matched:
			// A caller-supplied Deezer id always wins over a matched one;
			// ImageURL is only ever nil here (the gate above already
			// required it), so it is always assigned outright.
			p.ImageURL = res.ImageURL
			if p.DeezerID == nil {
				p.DeezerID = res.DeezerID
			}
		}
		// res.Matched == false: D-09 fail-closed. Leave p.ImageURL/p.DeezerID
		// exactly as they were -- UpsertArtist's COALESCE clauses below
		// preserve whatever the row already holds.
	}

	artist, err := s.q.UpsertArtist(ctx, sqlc.UpsertArtistParams{
		Mbid:           p.MBID,
		DeezerID:       p.DeezerID,
		Name:           p.Name,
		Disambiguation: p.Disambiguation,
		ImageUrl:       p.ImageURL,
	})
	if err != nil {
		return Entry{}, err
	}

	entry, err := s.q.CreateWatchlistEntry(ctx, sqlc.CreateWatchlistEntryParams{
		ArtistID:        artist.ID,
		ReleaseTypes:    releaseTypes,
		MutedEventTypes: mutedEventTypes,
	})
	if err != nil {
		// D-09: a duplicate add is never treated as an implicit preferences
		// update. All three conditions are required -- matching the
		// SQLSTATE alone would misattribute a future unique violation on
		// some other constraint to "artist already on watchlist"; matching
		// on err.Error() text would break the moment Postgres changes its
		// message wording or runs under a different locale. The
		// UpsertArtist call above legitimately refreshed the artist's
		// master name/deezer_id (D-03 master-data maintenance) but the
		// watchlist row itself is left exactly as it was -- no fallthrough
		// to an update, retry, or delete-and-reinsert.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key" {
			return Entry{}, ErrDuplicate
		}
		return Entry{}, fmt.Errorf("create watchlist entry: %w", err)
	}

	return toEntry(artist, entry), nil
}

// List returns every watchlist entry, joined with its artist's master data,
// ordered by artist name then artist id (WLST-04). The result is always
// non-nil -- allocated with make([]Entry, 0, len(rows)) -- so an empty
// watchlist encodes as JSON [] rather than null; every consumer from this
// phase's own tests through Phase 6's UI is spared from special-casing nil.
func (s *Service) List(ctx context.Context) ([]Entry, error) {
	rows, err := s.q.ListWatchlist(ctx)
	if err != nil {
		return nil, fmt.Errorf("list watchlist: %w", err)
	}

	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, Entry{
			ID:              row.ID,
			ArtistID:        row.ArtistID,
			MBID:            row.Mbid,
			Name:            row.Name,
			DeezerID:        row.DeezerID,
			Disambiguation:  row.Disambiguation,
			ImageURL:        row.ImageUrl,
			ReleaseTypes:    row.ReleaseTypes,
			MutedEventTypes: row.MutedEventTypes,
			CreatedAt:       row.CreatedAt.Time,
			UpdatedAt:       row.UpdatedAt.Time,
		})
	}
	return entries, nil
}

// UpdatePreferences applies a partial update to one or both preference axes
// (WLST-05, WLST-06, D-11). A nil PreferencesParams field means "leave this
// axis untouched"; a non-nil pointer -- including one pointing at an empty
// slice -- means "use exactly this normalised set." The two axes are always
// validated and written independently: nothing in this method may make one
// axis's resolved value depend on the other's (D-05).
//
// A call supplying neither axis is rejected with ErrNoPreferencesSupplied
// before the database is ever touched (WR-01, G-02-1). The rule lives here
// rather than at the HTTP boundary because Store is documented above as the
// reusable surface later phases build on -- a rule enforced only in
// internal/httpserver would not protect Phase 3's search proxy or Phase 4's
// poller, both of which are non-HTTP callers of this same method.
//
// The merge itself happens in a single round trip: UpdateWatchlistPreferences
// resolves each untouched axis's carried-forward value from the row version
// its own UPDATE locked, so there is no window between reading an axis and
// writing it for a concurrent writer to land in (G-02-2b, T-02-19). A
// zero-row result -- the id does not exist, whether it never did or was
// deleted a moment before this write landed -- surfaces as pgx.ErrNoRows,
// which is translated to ErrNotFound below, the same honest 404 Remove
// already produces from its :execrows count.
func (s *Service) UpdatePreferences(ctx context.Context, id int64, p PreferencesParams) (Entry, error) {
	// Reject an empty update before any database call -- and before the id
	// lookup, so an empty update against an unknown id reports this
	// sentinel, not ErrNotFound (WR-01, G-02-1).
	if p.ReleaseTypes == nil && p.MutedEventTypes == nil {
		return Entry{}, ErrNoPreferencesSupplied
	}

	// Validate first, before any database call, so a rejected request never
	// leaves a partially-applied row behind.
	params := sqlc.UpdateWatchlistPreferencesParams{
		ID:              id,
		ReleaseTypes:    []string{},
		MutedEventTypes: []string{},
	}

	if p.ReleaseTypes != nil {
		normalized, err := normalizeSet(*p.ReleaseTypes, ReleaseTypes, ErrInvalidReleaseType)
		if err != nil {
			return Entry{}, err
		}
		params.SetReleaseTypes = true
		params.ReleaseTypes = normalized
	}
	if p.MutedEventTypes != nil {
		normalized, err := normalizeSet(*p.MutedEventTypes, EventTypes, ErrInvalidEventType)
		if err != nil {
			return Entry{}, err
		}
		params.SetMutedEventTypes = true
		params.MutedEventTypes = normalized
	}

	updated, err := s.q.UpdateWatchlistPreferences(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, ErrNotFound
		}
		return Entry{}, fmt.Errorf("update watchlist preferences: %w", err)
	}

	return Entry{
		ID:              updated.ID,
		ArtistID:        updated.ArtistID,
		MBID:            updated.Mbid,
		Name:            updated.Name,
		DeezerID:        updated.DeezerID,
		Disambiguation:  updated.Disambiguation,
		ImageURL:        updated.ImageUrl,
		ReleaseTypes:    updated.ReleaseTypes,
		MutedEventTypes: updated.MutedEventTypes,
		CreatedAt:       updated.CreatedAt.Time,
		UpdatedAt:       updated.UpdatedAt.Time,
	}, nil
}

// Remove hard-deletes a watchlist entry by id (WLST-03, D-10): no status
// column, no soft-delete timestamp, no filtering predicate anywhere else in
// the schema or queries -- the row is gone, and nothing downstream can keep
// reading it. The artists master row is never touched: the FK cascade runs
// from artists to watchlist, not the other way (D-03), so removing a
// watchlist entry cannot take the artist with it. DeleteWatchlistEntry's
// :execrows affected-row count is what distinguishes "deleted" from "there
// was nothing to delete" -- 0 rows affected (including on a repeat delete of
// the same id) is ErrNotFound, never a silently-reported success.
func (s *Service) Remove(ctx context.Context, id int64) error {
	affected, err := s.q.DeleteWatchlistEntry(ctx, id)
	if err != nil {
		return fmt.Errorf("delete watchlist entry: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// normalizeSet validates values against allowed and returns a new slice
// containing each allowed value at most once, ordered by its position in
// allowed -- so submission order never affects the stored/returned order and
// a duplicate submission is silently collapsed rather than rejected (a
// preference array is semantically a set; sending the same filter twice
// expresses no contradiction). An empty or all-duplicate input yields
// []string{}, never nil, so the JSON encoder emits [] rather than null. On
// the first value outside allowed, normalizeSet returns
// fmt.Errorf("%w: %q", invalidErr, value) -- errors.Is(err, invalidErr)
// still matches, and the message names the offending value for the 400
// response.
func normalizeSet(values []string, allowed []string, invalidErr error) ([]string, error) {
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		found := false
		for _, a := range allowed {
			if v == a {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: %q", invalidErr, v)
		}
		seen[v] = true
	}

	out := make([]string, 0, len(allowed))
	for _, a := range allowed {
		if seen[a] {
			out = append(out, a)
		}
	}
	return out, nil
}

// toEntry joins an artist row and its watchlist row into the API-facing
// Entry shape.
func toEntry(artist sqlc.Artist, w sqlc.Watchlist) Entry {
	return Entry{
		ID:              w.ID,
		ArtistID:        artist.ID,
		MBID:            artist.Mbid,
		Name:            artist.Name,
		DeezerID:        artist.DeezerID,
		Disambiguation:  artist.Disambiguation,
		ImageURL:        artist.ImageUrl,
		ReleaseTypes:    w.ReleaseTypes,
		MutedEventTypes: w.MutedEventTypes,
		CreatedAt:       w.CreatedAt.Time,
		UpdatedAt:       w.UpdatedAt.Time,
	}
}
