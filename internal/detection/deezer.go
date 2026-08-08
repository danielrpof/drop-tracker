package detection

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const sourceDeezer = "deezer"

// DetectDeezer diffs albums -- freshly fetched for entry via ArtistAlbums --
// against the seen store and records each previously-unseen album as a
// new_release event, in Deezer's own numeric-id namespace (source='deezer',
// T-04-08), structurally mirroring DetectMusicBrainz: mute check, seed-mode
// check, seen-set lookup, per-album preference filter, insert.
//
// external_id is formatted from the decoded int64 album id via
// strconv.FormatInt, never from raw response text (T-04-08) --
// release_group_mbid is always nil, since Deezer has no release-group
// concept and that column exists for the MusicBrainz baseline lookup only.
//
// DetectDeezer produces new_release events only. There is no guest-feature
// or deluxe-change branch here, and there never will be one in this file:
// Deezer's client has no track- or credit-level fetch at all (D-08), and
// Deezer's Album.Tracklist field is a URL, not real track data (D-03). This
// is a deliberate scope decision, not an oversight.
func (d *Detector) DetectDeezer(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, albums []deezer.Album) error {
	if eventTypeMuted(entry, eventTypeNewRelease) {
		logger.Info("detection result",
			slog.String("artist_mbid", entry.MBID),
			slog.String("source", sourceDeezer),
			slog.String("event_type", eventTypeNewRelease),
			slog.Int("candidate_count", len(albums)),
			slog.Int("inserted_count", 0),
			slog.Int("filtered_count", len(albums)),
			slog.Bool("muted", true),
		)
		return nil
	}

	seedMode, err := d.isSeedMode(ctx, entry.ArtistID, sourceDeezer)
	if err != nil {
		return err
	}
	notifiedAt := seedNotifiedAt(seedMode)

	seen, err := d.seenExternalIDs(ctx, entry.ArtistID, sourceDeezer, eventTypeNewRelease)
	if err != nil {
		return err
	}

	inserted := 0
	filtered := 0
	// range only -- albums is an externally-supplied slice (T-04-01, ASVS
	// V5); never index a fixed position on it.
	for _, a := range albums {
		if !releaseTypeAllowed(entry, a.RecordType) {
			filtered++
			continue
		}

		externalID := strconv.FormatInt(a.ID, 10)
		if _, ok := seen[externalID]; ok {
			continue
		}

		newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
			ArtistID:         entry.ArtistID,
			Source:           sourceDeezer,
			EventType:        eventTypeNewRelease,
			ExternalID:       externalID,
			ReleaseGroupMbid: nil,
			Title:            a.Title,
			ArtistName:       entry.Name,
			ReleaseDate:      nullableString(a.ReleaseDate),
			CoverArtUrl:      nullableString(a.Cover),
			TrackCount:       nil,
			NotifiedAt:       notifiedAt,
		})
		if err != nil {
			return fmt.Errorf("detection: detect deezer: %w", err)
		}
		if newly {
			inserted++
		}
	}

	logger.Info("detection result",
		slog.String("artist_mbid", entry.MBID),
		slog.String("source", sourceDeezer),
		slog.String("event_type", eventTypeNewRelease),
		slog.Int("candidate_count", len(albums)),
		slog.Int("inserted_count", inserted),
		slog.Int("filtered_count", filtered),
		slog.Bool("seed_mode", seedMode),
	)

	return nil
}
