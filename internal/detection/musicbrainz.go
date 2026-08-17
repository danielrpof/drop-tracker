package detection

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const (
	sourceMusicBrainz     = "musicbrainz"
	eventTypeNewRelease   = "new_release"
	eventTypeGuestFeature = "guest_feature"
	eventTypeDeluxeChange = "deluxe_change"
)

// DetectMusicBrainz diffs groups -- freshly fetched for entry via
// ReleaseGroupsByArtist -- against the seen store and records each
// previously-unseen release-group as a new_release event (DTCT-01), gated by
// both of entry's preference axes (D-17, D-18) and by per-source seed mode
// (D-13/D-14/D-15). It then runs detectGuestFeatures (DTCT-03), which fetches
// and diffs entry's recording-by-artist-credit browse in the same call, and
// detectDeluxeChanges (DTCT-02), which fetches per-release track-count
// detail for release-groups already in the seen store -- one MusicBrainz
// poll cycle covers all three under the same PollInterval and rate limiter
// (D-07), with no second scheduler cadence.
//
// The seed-mode decision (isSeedMode) is made exactly once, before any
// pass, and its resulting notified_at value is threaded into every
// insertEvent call this whole method makes -- across all three event types.
// Reading it lazily per pass would flip the answer mid-call (an earlier
// pass's own inserts would make a later pass see a non-zero event count for
// the source and read as already-seeded) and leave that pass's rows
// unseeded, violating D-13's "every row from one seed cycle shares one
// timestamp" contract now that three event types share one (artist_id,
// source) seed-mode scope.
//
// preCycleSeenGroups -- the set of release-group MBIDs already recorded as
// new_release events -- is likewise captured exactly once, before the
// new_release pass inserts anything this cycle, and handed to
// detectDeluxeChanges unchanged. Re-querying it after the new_release pass
// would include groups that pass just inserted a moment earlier in this
// same call, so a brand-new group would receive a release-detail fetch on
// its own discovery cycle -- exactly what D-04 forbids.
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

	preCycleSeenGroups, err := d.seenExternalIDs(ctx, entry.ArtistID, sourceMusicBrainz, eventTypeNewRelease)
	if err != nil {
		return err
	}

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
		seen := preCycleSeenGroups

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
				ReleaseType:      releaseTypeForStorage(g.PrimaryType),
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

	if err := d.detectGuestFeatures(ctx, logger, entry, seedMode, notifiedAt); err != nil {
		return err
	}

	return d.detectDeluxeChanges(ctx, logger, entry, groups, preCycleSeenGroups, notifiedAt)
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

// detectDeluxeChanges diffs freshGroups against preCycleSeen -- the
// pre-cycle new_release seen-set DetectMusicBrainz captured before its own
// pass ran -- and, for every group already in that set, fetches per-release
// track-count detail and compares it against a persisted baseline (D-01,
// D-02). A release-group not in preCycleSeen was discovered for the first
// time this very cycle: D-04 forbids fetching its release detail in the
// same cycle it was discovered, so it is skipped entirely, at zero request
// cost.
//
// Both preference gates (deluxeDetectionEnabled, eventTypeMuted) are
// checked before any fetch: an artist who has not opted into deluxe alerts,
// or who has muted deluxe_change, costs zero release-detail requests.
//
// For each already-seen group, the maximum TrackCount() across its fetched
// releases is passed to advanceGroupBaseline, a single atomic
// compare-and-set (PERF-04, 11-RESEARCH.md Pattern 2) that replaced the
// former two-step read-then-write:
//   - No advance (fresh maximum not a genuine increase): no-op, baseline
//     left untouched -- D-02 counts increases only, and lowering the
//     baseline on a downward blip would let the same tracklist re-fire
//     later.
//   - Advance with no prior baseline: the fresh maximum silently BECOMES
//     the baseline and no event fires -- a first measurement is a
//     baseline, not an observed increase (04-RESEARCH.md Pitfall #1).
//   - Advance with a prior baseline: a deluxe_change event is inserted,
//     keyed on the winning release's own MBID (D-10), carrying the
//     returned previous value as PreviousTrackCount.
//
// A fresh maximum of 0 (absent/empty media, or media entries with no
// track-count) means the response carried no usable data -- the group is
// skipped and the baseline (if any) is left untouched, exactly like a
// group that was never fetched.
//
// A per-group release-detail error is logged and that group is skipped;
// the pass continues to the next group and this method still returns nil,
// so one unreachable release-group never discards the cycle's other
// events (mirrors detectGuestFeatures's own error-isolation contract).
//
// Known, accepted edge: advanceGroupBaseline's atomic write necessarily
// commits before the deluxe_change event insert below, because the advance
// is what decides whether an event fires at all -- the previous
// insert-then-advance ordering (baseline advanced only after the event was
// already durably recorded) cannot be preserved once the two are collapsed
// into one statement. A process crash between the advance committing and
// the event insert permanently loses that group's notification: the next
// cycle's fresh count is no longer an increase over the already-advanced
// baseline, so nothing will fire for it later either. PERF-04's
// atomicity requirement is satisfied regardless of this ordering; closing
// this separate, narrower window would require wrapping both calls in an
// explicit database transaction, which needs a transaction-capable seam
// this codebase's Detector does not have today (it holds only a
// sqlc.Querier interface). That machinery is not justified for a window
// measured in the time between two commits -- this is an accepted,
// documented residual, mirroring isSeedMode's existing precedent for
// documenting an accepted edge rather than closing it.
//
// This InsertEvent failure has a second, distinct consequence beyond the
// failing group itself: the `return fmt.Errorf(...)` below propagates out
// of the `for _, g := range freshGroups` loop, so every group after the
// failing one in that artist's freshGroups slice is skipped for the rest
// of the current cycle. Those skipped groups are NOT lost the way the
// failing group is -- their baselines were never advanced, so the next
// cycle recomputes them from scratch and they are simply delayed by one
// poll interval. The asymmetry is load-bearing: the failing group's
// notification is permanently lost, while the groups behind it in the
// slice are merely delayed. This failure mode is a pre-existing pattern
// shared with the sibling detection passes, not something introduced by
// the atomic-CTE rewrite above.
func (d *Detector) detectDeluxeChanges(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, freshGroups []musicbrainz.ReleaseGroup, preCycleSeen map[string]struct{}, notifiedAt pgtype.Timestamptz) error {
	if !deluxeDetectionEnabled(entry) {
		return nil
	}
	if eventTypeMuted(entry, eventTypeDeluxeChange) {
		return nil
	}

	detailFetchCount := 0
	baselineEstablishedCount := 0
	inserted := 0
	pageCeilingReached := false

	// range only -- freshGroups is externally-supplied (T-04-01, ASVS V5).
	for _, g := range freshGroups {
		if _, ok := preCycleSeen[g.MBID]; !ok {
			// D-04: a group discovered this very cycle gets no release-detail
			// fetch -- it is a new_release event and nothing else.
			continue
		}

		detailFetchCount++
		releases, err := d.releases.ReleasesByReleaseGroup(ctx, g.MBID)
		if err != nil {
			logger.Error("release detail fetch failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("release_group_mbid", g.MBID),
				slog.String("musicbrainz_error", err.Error()),
			)
			continue
		}
		if len(releases) >= musicbrainz.MaxReleaseBrowseItems {
			pageCeilingReached = true
		}

		// The comparison uses the maximum total track count across every
		// release in the group (D-02), never the first or last encountered
		// -- the outcome must not depend on upstream ordering.
		maxCount := 0
		var winner musicbrainz.Release
		// range only -- releases is externally-supplied (T-04-01, ASVS V5).
		for _, r := range releases {
			if tc := r.TrackCount(); tc > maxCount {
				maxCount = tc
				winner = r
			}
		}
		if maxCount == 0 {
			// No usable media data in this fetch -- not "the release has no
			// tracks." Leave any existing baseline untouched.
			continue
		}

		advanced, hadBaseline, previousBaseline, err := d.advanceGroupBaseline(ctx, g.MBID, maxCount)
		if err != nil {
			return err
		}

		switch {
		case !advanced:
			// Equal or lower: D-02 counts increases only. A lower count is
			// an upstream data correction, and lowering the baseline would
			// let the same tracklist re-fire later. advanceGroupBaseline
			// already decided this atomically; nothing left to do here.
		case !hadBaseline:
			baselineEstablishedCount++
			logger.Info("baseline_established",
				slog.String("artist_mbid", entry.MBID),
				slog.String("release_group_mbid", g.MBID),
				slog.Int("track_count", maxCount),
			)
		default: // advanced && hadBaseline
			groupMBID := g.MBID
			coverArt := coverArtURLForReleaseGroup(groupMBID)
			trackCount := int32(maxCount) //nolint:gosec // maxCount sums MusicBrainz media.TrackCount fields; a real release is always orders of magnitude under int32 range (worst case on a malformed upstream value is a wrong stored number, not a security defect)
			previousTrackCount := int32(previousBaseline) //nolint:gosec // previousBaseline is read back from advanceGroupBaseline's own previously-stored int32 column, never a fresh unbounded external value
			newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
				ArtistID:           entry.ArtistID,
				Source:             sourceMusicBrainz,
				EventType:          eventTypeDeluxeChange,
				ExternalID:         winner.MBID,
				ReleaseGroupMbid:   &groupMBID,
				Title:              winner.Title,
				ArtistName:         entry.Name,
				ReleaseDate:        nullableString(winner.Date),
				CoverArtUrl:        &coverArt,
				TrackCount:         &trackCount,
				PreviousTrackCount: &previousTrackCount,
				NotifiedAt:         notifiedAt,
			})
			if err != nil {
				// Known, accepted edge (see this method's doc comment): the
				// baseline already advanced above, so this tracklist
				// expansion will not be re-detected on a later cycle. Log
				// at Warn (distinct from a generic insert-level DB outage,
				// mirroring the notifier's WR-03 line) before returning, so
				// this specific failure mode is identifiable in production
				// logs via the static `window` field below.
				logger.Warn("deluxe change event insert failed after baseline advance: this tracklist expansion will not be re-detected",
					slog.String("artist_mbid", entry.MBID),
					slog.String("release_group_mbid", groupMBID),
					slog.String("window", "baseline_advanced_insert_failed"),
					slog.String("error", err.Error()),
				)
				return fmt.Errorf("detection: detect deluxe changes: %w", err)
			}
			if newly {
				inserted++
			}
		}
	}

	logger.Info("detection result",
		slog.String("artist_mbid", entry.MBID),
		slog.String("event_type", eventTypeDeluxeChange),
		slog.Int("detail_fetch_count", detailFetchCount),
		slog.Int("baseline_established_count", baselineEstablishedCount),
		slog.Int("inserted_count", inserted),
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

// releaseTypeForStorage returns the same lowercased, trimmed normalization
// releaseTypeAllowed already applies to a release-group's raw PrimaryType --
// storing the identical vocabulary (detectableReleaseTypes) keeps a future
// query grouping events by release_type in agreement with the preference
// axis that let the row through in the first place. Wrapped in
// nullableString so an absent PrimaryType stores SQL NULL rather than an
// empty-string literal (D-04, 05-RESEARCH.md Pitfall 3) -- display
// title-casing is the formatter's concern, not storage's.
func releaseTypeForStorage(primaryType string) *string {
	return nullableString(strings.ToLower(strings.TrimSpace(primaryType)))
}

// coverArtURLForReleaseGroup builds the deterministic Cover Art Archive URL
// for a release-group MBID (D-12, 04-RESEARCH.md Pitfall #6) -- MusicBrainz
// responses never carry a cover-art field, and this URL pattern needs no
// extra HTTP call to construct.
func coverArtURLForReleaseGroup(mbid string) string {
	return "https://coverartarchive.org/release-group/" + mbid + "/front"
}
