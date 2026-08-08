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
	sourceMusicBrainz   = "musicbrainz"
	eventTypeNewRelease = "new_release"
)

// DetectMusicBrainz diffs groups -- freshly fetched for entry via
// ReleaseGroupsByArtist -- against the seen store and records each
// previously-unseen release-group as a new_release event (DTCT-01), gated by
// both of entry's preference axes (D-17, D-18). No guest-feature, no
// deluxe-change, no seed mode here -- those are later plans. notified_at is
// always NULL from this method; seed mode (D-13) arrives in plan 04-02.
//
// The mute axis (D-18) is checked once, before the loop: an entry that has
// muted new_release skips every group with no seen-set lookup and no insert
// at all. The release-type axis (D-17) is checked per group, inside the
// loop, before the seen-set lookup -- a group failing either check never
// reaches the database, so the seen store only ever holds what the artist's
// current preferences actually want.
//
// A group whose MBID is already in the seen store is skipped without an
// InsertEvent call -- the ON CONFLICT DO NOTHING clause would no-op it
// anyway, but skipping client-side avoids a wasted round trip for what is,
// on a typical steady-state cycle, the overwhelming majority of an
// artist's discography.
func (d *Detector) DetectMusicBrainz(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
	if eventTypeMuted(entry, eventTypeNewRelease) {
		logger.Info("detection result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("event_type", eventTypeNewRelease),
			slog.Int("candidate_count", len(groups)),
			slog.Int("inserted_count", 0),
			slog.Int("filtered_count", len(groups)),
			slog.Bool("muted", true),
		)
		return nil
	}

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
			NotifiedAt:       pgtype.Timestamptz{},
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
	)

	return nil
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
