package detection

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const (
	sourceMusicBrainz     = "musicbrainz"
	eventTypeNewRelease   = "new_release"
	eventTypeGuestFeature = "guest_feature"
)

// DetectMusicBrainz diffs groups -- freshly fetched for entry via
// ReleaseGroupsByArtist -- against the seen store and records each
// previously-unseen release-group as a new_release event (DTCT-01), gated by
// both of entry's preference axes (D-17, D-18) and by per-source seed mode
// (D-13/D-14/D-15). It then runs detectGuestFeatures (DTCT-03), which fetches
// and diffs entry's recording-by-artist-credit browse in the same call --
// one MusicBrainz poll cycle covers both new_release and guest_feature
// detection under the same PollInterval and rate limiter (D-07), with no
// second scheduler cadence. No deluxe-change here -- that is a later plan.
//
// The seed-mode decision (isSeedMode) is made exactly once, before either
// pass, and its resulting notified_at value is threaded into every
// insertEvent call this whole method makes -- across both event types.
// Reading it lazily per pass would flip the answer mid-call (the
// new_release pass's own inserts would make the guest_feature pass see a
// non-zero event count for the source and read as already-seeded) and leave
// the later pass's rows unseeded, violating D-13's "every row from one seed
// cycle shares one timestamp" contract now that two event types share one
// (artist_id, source) seed-mode scope.
//
// The mute axis (D-18) is checked once per event type, before its own
// pass's fetch/seen-set/insert work: a muted event type does no seen-set
// lookup and no insert at all (new_release additionally skips no per-group
// work since it never reaches the loop). The release-type axis (D-17) is
// checked per group, inside the new_release loop, before the seen-set
// lookup -- a group failing either check never reaches the database, so the
// seen store only ever holds what the artist's current preferences actually
// want.
//
// An id already in the seen store is skipped without an InsertEvent call --
// the ON CONFLICT DO NOTHING clause would no-op it anyway, but skipping
// client-side avoids a wasted round trip for what is, on a typical
// steady-state cycle, the overwhelming majority of an artist's catalogue.
func (d *Detector) DetectMusicBrainz(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
	seedMode, err := d.isSeedMode(ctx, entry.ArtistID, sourceMusicBrainz)
	if err != nil {
		return err
	}
	notifiedAt := seedNotifiedAt(seedMode)

	if eventTypeMuted(entry, eventTypeNewRelease) {
		logger.Info("detection result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("event_type", eventTypeNewRelease),
			slog.Int("candidate_count", len(groups)),
			slog.Int("inserted_count", 0),
			slog.Int("filtered_count", len(groups)),
			slog.Bool("muted", true),
		)
	} else {
		seen, err := d.seenExternalIDs(ctx, entry.ArtistID, sourceMusicBrainz, eventTypeNewRelease)
		if err != nil {
			return err
		}

		inserted := 0
		filtered := 0
		// range only -- groups is an externally-supplied slice (T-04-01, ASVS
		// V5); never index a fixed position on it.
		for _, g := range groups {
			if !releaseTypeAllowed(entry, g.PrimaryType) {
				filtered++
				continue
			}
			if _, ok := seen[g.MBID]; ok {
				continue
			}

			mbid := g.MBID
			coverArt := coverArtURLForReleaseGroup(mbid)
			newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
				ArtistID:         entry.ArtistID,
				Source:           sourceMusicBrainz,
				EventType:        eventTypeNewRelease,
				ExternalID:       mbid,
				ReleaseGroupMbid: &mbid,
				Title:            g.Title,
				ArtistName:       entry.Name,
				ReleaseDate:      nullableString(g.FirstReleaseDate),
				CoverArtUrl:      &coverArt,
				TrackCount:       nil,
				NotifiedAt:       notifiedAt,
			})
			if err != nil {
				return fmt.Errorf("detection: detect musicbrainz: %w", err)
			}
			if newly {
				inserted++
			}
		}

		logger.Info("detection result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("event_type", eventTypeNewRelease),
			slog.Int("candidate_count", len(groups)),
			slog.Int("inserted_count", inserted),
			slog.Int("filtered_count", filtered),
			slog.Bool("seed_mode", seedMode),
		)
	}

	return d.detectGuestFeatures(ctx, logger, entry, seedMode, notifiedAt)
}

// detectGuestFeatures diffs entry's recordings-by-artist-credit browse
// (D-05) against the seen store and records each previously-unseen guest
// appearance as a guest_feature event (DTCT-03, D-06). seedMode/notifiedAt
// are computed once by DetectMusicBrainz, before this pass or the
// new_release pass runs, and threaded through here so both event types
// share one seed decision and one notified_at timestamp for this cycle.
//
// The mute check (D-18) runs before any recording fetch: a muted event type
// spends no rate-limiter budget on a fetch whose result would be discarded
// anyway.
//
// A recording-source error is logged and this pass returns nil rather than
// propagating the error -- a failed recording browse must never discard the
// new_release events the same cycle's earlier pass already recorded.
//
// isGuestFeature runs against every fetched recording before any
// event-creation logic (04-RESEARCH.md Common Pitfall #3): RecordingsByArtist
// returns every recording the artist is credited on, in ANY position, not
// just guest appearances -- treating the raw fetch as the guest-feature set
// would massively over-notify.
func (d *Detector) detectGuestFeatures(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, seedMode bool, notifiedAt pgtype.Timestamptz) error {
	if eventTypeMuted(entry, eventTypeGuestFeature) {
		logger.Info("detection result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("event_type", eventTypeGuestFeature),
			slog.Int("recording_count", 0),
			slog.Int("inserted_count", 0),
			slog.Bool("seed_mode", seedMode),
			slog.Bool("muted", true),
			slog.Bool("page_ceiling_reached", false),
		)
		return nil
	}

	recordings, err := d.recordings.RecordingsByArtist(ctx, entry.MBID)
	if err != nil {
		logger.Error("recording browse failed",
			slog.String("artist_mbid", entry.MBID),
			slog.String("artist_name", entry.Name),
			slog.String("musicbrainz_error", err.Error()),
		)
		return nil
	}

	seen, err := d.seenExternalIDs(ctx, entry.ArtistID, sourceMusicBrainz, eventTypeGuestFeature)
	if err != nil {
		return err
	}

	inserted := 0
	// range only -- recordings is externally-supplied (T-04-12, ASVS V5);
	// isGuestFeature applies its own defensive length guard before indexing.
	for _, rec := range recordings {
		if !isGuestFeature(rec, entry.MBID) {
			continue
		}
		if _, ok := seen[rec.MBID]; ok {
			continue
		}

		newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
			ArtistID:   entry.ArtistID,
			Source:     sourceMusicBrainz,
			EventType:  eventTypeGuestFeature,
			ExternalID: rec.MBID,
			Title:      rec.Title,
			ArtistName: displayArtistName(rec, entry.Name),
			NotifiedAt: notifiedAt,
		})
		if err != nil {
			return fmt.Errorf("detection: detect guest features: %w", err)
		}
		if newly {
			inserted++
			// Guard against the same recording MBID appearing twice within
			// one browse result (D-10's dedup key already makes this safe
			// at the DB level via ON CONFLICT DO NOTHING; marking it seen
			// here just avoids a second wasted round trip for the
			// duplicate).
			seen[rec.MBID] = struct{}{}
		}
	}

	pageCeilingReached := len(recordings) >= musicbrainz.MaxRecordingBrowseItems

	logger.Info("detection result",
		slog.String("artist_mbid", entry.MBID),
		slog.String("event_type", eventTypeGuestFeature),
		slog.Int("recording_count", len(recordings)),
		slog.Int("inserted_count", inserted),
		slog.Bool("seed_mode", seedMode),
		slog.Bool("page_ceiling_reached", pageCeilingReached),
	)

	return nil
}

// isGuestFeature implements D-06's positional rule: rec is a guest
// appearance for watchedArtistMBID when the recording's first artist-credit
// entry is NOT the watched artist. The length guard is load-bearing, not
// defensive noise (T-04-12, ASVS V5) -- MusicBrainz is community-editable,
// semi-trusted data, and indexing position zero without it is a real panic
// risk on a malformed response.
//
// A first-credit entry whose nested artist MBID is empty is treated as "not
// the watched artist" -- an unidentifiable primary credit errs toward an
// extra alert rather than a silently missed feature.
func isGuestFeature(rec musicbrainz.Recording, watchedArtistMBID string) bool {
	if len(rec.ArtistCredit) == 0 {
		return false
	}
	return rec.ArtistCredit[0].Artist.MBID != watchedArtistMBID
}

// displayArtistName is what a guest_feature row stores in artist_name
// (D-12): the primary-credit artist's name, not the watched artist's --
// artist_id already identifies the watched artist, so the useful display
// datum is who the track is credited to, letting Phase 5's NTFY-02 message
// render "<watched artist> appears on <title> by <primary artist>" without
// a second external call. Falls back to the first entry's credited Name,
// then to fallback (the watched artist's own name), if the nested artist
// name is empty.
func displayArtistName(rec musicbrainz.Recording, fallback string) string {
	if len(rec.ArtistCredit) == 0 {
		return fallback
	}
	first := rec.ArtistCredit[0]
	if first.Artist.Name != "" {
		return first.Artist.Name
	}
	if first.Name != "" {
		return first.Name
	}
	return fallback
}

// seenExternalIDs returns the set of external ids already recorded for
// artistID under source/eventType -- the "seen" half of the fresh-vs-seen
// diff (D-10).
func (d *Detector) seenExternalIDs(ctx context.Context, artistID int64, source, eventType string) (map[string]struct{}, error) {
	ids, err := d.q.ListExternalIDs(ctx, sqlc.ListExternalIDsParams{
		ArtistID:  artistID,
		Source:    source,
		EventType: eventType,
	})
	if err != nil {
		return nil, fmt.Errorf("detection: list seen external ids: %w", err)
	}

	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return seen, nil
}

// coverArtURLForReleaseGroup builds the deterministic Cover Art Archive URL
// for a release-group MBID (D-12, 04-RESEARCH.md Pitfall #6) -- MusicBrainz
// responses never carry a cover-art field, and this URL pattern needs no
// extra HTTP call to construct.
func coverArtURLForReleaseGroup(mbid string) string {
	return "https://coverartarchive.org/release-group/" + mbid + "/front"
}
