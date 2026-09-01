package detection

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

const (
	sourceMusicBrainz     = "musicbrainz"
	eventTypeNewRelease   = "new_release"
	eventTypeGuestFeature = "guest_feature"
	eventTypeDeluxeChange = "deluxe_change"

	// maxNewGuestFeatureLookupsPerCycle bounds one artist's ReleasesForRecording
	// lookups in a single detectGuestFeatures call (D-13), so a newly-added
	// artist's seed cycle cannot spend the whole shared MusicBrainz rate budget.
	// Recordings beyond the cap are skipped this cycle (not inserted, not marked
	// seen), like a lookup error, and retried next cycle.
	maxNewGuestFeatureLookupsPerCycle = 20

	// deluxeRecheckWindowDays bounds how far back detectDeluxeChanges re-fetches
	// release detail for an already-seen group. Without it every already-seen
	// group gets a ReleasesByReleaseGroup call every cycle forever -- the largest
	// unbounded consumer of the shared MusicBrainz limiter, starving search and
	// detectGuestFeatures. A group older than this has finished gaining deluxe
	// editions. Generous by design: too short risks a missed alert, too long
	// only costs bounded extra traffic.
	deluxeRecheckWindowDays = 90
)

// DetectMusicBrainz diffs freshly-fetched groups against the seen store and
// records each unseen release-group as a new_release event (DTCT-01), gated by
// entry's two preference axes (D-17, D-18) and per-source seed mode
// (D-13/D-14/D-15). It then runs detectGuestFeatures (DTCT-03) and
// detectDeluxeChanges (DTCT-02) -- one poll cycle covers all three under one
// rate limiter (D-07). isSeedMode and preCycleSeenGroups are each captured ONCE
// before any pass inserts: reading them lazily would flip the answer mid-call
// (D-13) and hand a just-discovered group a release-detail fetch (D-04).
func (d *Detector) DetectMusicBrainz(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, groups []musicbrainz.ReleaseGroup) error {
	seedMode, err := d.isSeedMode(ctx, entry.ArtistID, sourceMusicBrainz)
	if err != nil {
		return err
	}
	notify := newNotifyGate(seedMode, d.notifyMaxReleaseAgeDays, time.Now().UTC())

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
		// range only -- groups is externally-supplied (T-04-01, ASVS V5).
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
			watchedName := entry.Name
			newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
				ArtistID:          entry.ArtistID,
				Source:            sourceMusicBrainz,
				EventType:         eventTypeNewRelease,
				ExternalID:        mbid,
				ReleaseGroupMbid:  &mbid,
				Title:             g.Title,
				ArtistName:        entry.Name,
				ReleaseDate:       nullableString(g.FirstReleaseDate),
				CoverArtUrl:       &coverArt,
				TrackCount:        nil,
				ReleaseType:       releaseTypeForStorage(g.PrimaryType),
				NotifiedAt:        notify.notifiedAt(g.FirstReleaseDate),
				WatchedArtistName: &watchedName,
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

	if err := d.detectGuestFeatures(ctx, logger, entry, seedMode, notify); err != nil {
		return err
	}

	return d.detectDeluxeChanges(ctx, logger, entry, groups, preCycleSeenGroups, notify)
}

// detectGuestFeatures diffs entry's recordings-by-artist-credit browse (D-05)
// against the seen store and records each unseen guest appearance as a
// guest_feature event (DTCT-03, D-06). seedMode/notify come from
// DetectMusicBrainz so both event types share one decision this cycle. The mute
// check (D-18) runs before any fetch. A recording-source error is logged and
// returns nil -- a failed browse must not discard the cycle's new_release rows.
// isGuestFeature filters every recording first (04-RESEARCH.md Pitfall #3):
// RecordingsByArtist returns every credit, not just guest ones.
func (d *Detector) detectGuestFeatures(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, seedMode bool, notify notifyGate) error {
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
	lookupCount := 0
	lookupCapReachedAt := 0
	releaseLinkCeilingCount := 0
	// range only -- recordings is externally-supplied (T-04-12, ASVS V5).
	for _, rec := range recordings {
		if !isGuestFeature(rec, entry.MBID) {
			continue
		}
		if _, ok := seen[rec.MBID]; ok {
			continue
		}

		// D-13: past the lookup budget, remaining recordings are skipped (not
		// inserted, not marked seen) and retried next cycle.
		if lookupCount >= maxNewGuestFeatureLookupsPerCycle {
			if lookupCapReachedAt == 0 {
				lookupCapReachedAt = maxNewGuestFeatureLookupsPerCycle
			}
			continue
		}

		// D-01: source a release date and release-group MBID via one lookup.
		releases, err := d.recordings.ReleasesForRecording(ctx, rec.MBID)
		lookupCount++
		if err != nil {
			// OQ-02: a lookup error is isolated to this recording -- not
			// inserted, not marked seen, retried next cycle.
			logger.Error("recording release lookup failed",
				slog.String("artist_mbid", entry.MBID),
				slog.String("recording_mbid", rec.MBID),
				slog.String("musicbrainz_error", err.Error()),
			)
			continue
		}
		if len(releases) >= musicbrainz.MaxRecordingReleaseLinks {
			releaseLinkCeilingCount++
		}

		// D-02: earliestReleaseDate picks the precision-aware earliest date;
		// guestFeatureArt independently finds a release-group MBID for art.
		releaseDate := earliestReleaseDate(releases)
		releaseGroupMBID, coverArt := guestFeatureArt(releases)

		// artist_name is the track's primary credit; watched_artist_name is the
		// watchlist entry that caused the insert. Distinct facts, not redundant
		// -- a guest_feature row's watched artist is usually not the primary credit.
		watchedName := entry.Name
		params := sqlc.InsertEventParams{
			ArtistID:          entry.ArtistID,
			Source:            sourceMusicBrainz,
			EventType:         eventTypeGuestFeature,
			ExternalID:        rec.MBID,
			Title:             rec.Title,
			ArtistName:        displayArtistName(rec, entry.Name),
			ReleaseDate:       nullableString(releaseDate),
			NotifiedAt:        notify.notifiedAt(releaseDate),
			WatchedArtistName: &watchedName,
		}
		if releaseGroupMBID != "" {
			groupMBID := releaseGroupMBID
			params.ReleaseGroupMbid = &groupMBID
			params.CoverArtUrl = &coverArt
		}

		newly, err := d.insertEvent(ctx, params)
		if err != nil {
			return fmt.Errorf("detection: detect guest features: %w", err)
		}
		if newly {
			inserted++
			// Guard against a duplicate recording MBID in one browse result
			// (D-10 already makes this DB-safe; this just skips a round trip).
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
		slog.Int("release_link_ceiling_count", releaseLinkCeilingCount),
		slog.Int("guest_feature_lookup_cap_reached_at", lookupCapReachedAt),
	)

	return nil
}

// detectDeluxeChanges compares the max TrackCount() of every freshGroup already
// in preCycleSeen against a persisted baseline (D-01, D-02) via
// advanceGroupBaseline's atomic CAS (PERF-04, 11-RESEARCH.md Pattern 2). Groups
// not in preCycleSeen (D-04) or outside deluxeRecheckWindowDays (quick/260826-gj8)
// are skipped pre-fetch, as are muted/disabled preference axes. First measurement
// becomes the baseline silently (04-RESEARCH.md Pitfall #1); a real increase
// fires a deluxe_change keyed on the winning release MBID (D-10). Residual: a
// crash between baseline commit and event insert permanently loses that alert.
func (d *Detector) detectDeluxeChanges(ctx context.Context, logger *slog.Logger, entry watchlist.Entry, freshGroups []musicbrainz.ReleaseGroup, preCycleSeen map[string]struct{}, notify notifyGate) error {
	if !deluxeDetectionEnabled(entry) {
		return nil
	}
	if eventTypeMuted(entry, eventTypeDeluxeChange) {
		return nil
	}

	detailFetchCount := 0
	baselineEstablishedCount := 0
	windowSkippedCount := 0
	inserted := 0
	pageCeilingReached := false

	// Captured once per call, not per group, so a midnight-straddling pass
	// judges every group by one cutoff (mirrors newNotifyGate).
	cutoff := time.Now().UTC().AddDate(0, 0, -deluxeRecheckWindowDays).Format(time.DateOnly)

	// range only -- freshGroups is externally-supplied (T-04-01, ASVS V5).
	for _, g := range freshGroups {
		if _, ok := preCycleSeen[g.MBID]; !ok {
			// D-04: a group discovered this cycle gets no release-detail fetch.
			continue
		}

		if !withinDeluxeRecheckWindow(g.FirstReleaseDate, cutoff) {
			// quick/260826-gj8: outside the recheck window -- pure omission,
			// baseline untouched, so widening the window later resumes cleanly.
			windowSkippedCount++
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

		// Max track count across the group (D-02), never first/last -- the
		// outcome must not depend on upstream ordering.
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
			// No usable media data -- not "no tracks". Leave the baseline alone.
			continue
		}

		advanced, hadBaseline, previousBaseline, err := d.advanceGroupBaseline(ctx, g.MBID, maxCount)
		if err != nil {
			return err
		}

		switch {
		case !advanced:
			// Equal or lower: D-02 counts increases only, and lowering the
			// baseline would let the same tracklist re-fire later.
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
			trackCount := int32(maxCount)                 //nolint:gosec // maxCount sums MusicBrainz media.TrackCount fields; a real release is always orders of magnitude under int32 range (worst case on a malformed upstream value is a wrong stored number, not a security defect)
			previousTrackCount := int32(previousBaseline) //nolint:gosec // previousBaseline is read back from advanceGroupBaseline's own previously-stored int32 column, never a fresh unbounded external value
			watchedName := entry.Name
			// KNOWN NARROWING (accepted): the group is inside the 90-day
			// recheck window, but notifiedAt applies the 7-day notify window to
			// the WINNING RELEASE's date -- so a deluxe edition dated to match
			// the original album is recorded in history but never sent. Left
			// as-is: zero deluxe_change rows exist in production, so it narrows
			// nothing observable; widening it is a product decision.
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
				NotifiedAt:         notify.notifiedAt(winner.Date),
				WatchedArtistName:  &watchedName,
			})
			if err != nil {
				// Accepted edge (see the doc comment): the baseline already
				// advanced, so this expansion is not re-detected later. Warn
				// (like the notifier's WR-03 line), identifiable via `window`.
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
		slog.Int("window_skipped_count", windowSkippedCount),
		slog.Int("inserted_count", inserted),
		slog.Bool("page_ceiling_reached", pageCeilingReached),
	)

	return nil
}

// isGuestFeature implements D-06's positional rule: rec is a guest appearance
// when its first artist-credit entry is NOT the watched artist. The length guard
// is load-bearing (T-04-12, ASVS V5) -- MusicBrainz is semi-trusted data and
// indexing position zero without it can panic. An empty first-credit MBID counts
// as "not the watched artist" -- errs toward an extra alert.
func isGuestFeature(rec musicbrainz.Recording, watchedArtistMBID string) bool {
	if len(rec.ArtistCredit) == 0 {
		return false
	}
	return rec.ArtistCredit[0].Artist.MBID != watchedArtistMBID
}

// displayArtistName is a guest_feature row's artist_name (D-12): the
// primary-credit artist, not the watched one (artist_id already has that), so
// NTFY-02 can render "<watched> appears on <title> by <primary>" with no extra
// call. Falls back to the entry's credited Name, then to fallback.
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

// earliestReleaseDate implements D-02: the earliest release date among releases,
// compared as strings (MusicBrainz partial dates can't round-trip through
// time.Time). Dates under 4 chars are filtered first -- empty sorts before every
// real date and a short one would panic earlierDate's slicing (WR-01,
// 13-REVIEW.md) -- and the rest are folded via earlierDate, whose precision rule
// (the longer, more precise date wins a same-year prefix tie) plain `<` gets wrong.
func earliestReleaseDate(releases []musicbrainz.RecordingRelease) string {
	earliest := ""
	// range only -- releases is externally-supplied (T-04-12, ASVS V5).
	for _, r := range releases {
		// Under 4 chars is malformed -- skip so it never reaches earlierDate's
		// a[:4]/b[:4] slicing (WR-01, 13-REVIEW.md).
		if len(r.Date) < 4 {
			continue
		}
		if earliest == "" {
			earliest = r.Date
			continue
		}
		earliest = earlierDate(earliest, r.Date)
	}
	return earliest
}

// earlierDate returns whichever of a, b is earlier under earliestReleaseDate's
// precision-aware rule. Both are guaranteed >= 4 chars by its length filter.
func earlierDate(a, b string) string {
	yearA, yearB := a[:4], b[:4]
	if yearA != yearB {
		// Fixed-width 4-digit numerals: lexicographic and numeric order agree.
		if yearA < yearB {
			return a
		}
		return b
	}
	// Same year, precision difference (one a strict prefix of the other): the
	// more precise value wins, e.g. "2020" vs "2020-01-05" -> "2020-01-05".
	if strings.HasPrefix(b, a) {
		return b
	}
	if strings.HasPrefix(a, b) {
		return a
	}
	// Same year, equal precision: plain comparison is correct.
	if a < b {
		return a
	}
	return b
}

// withinDeluxeRecheckWindow reports whether firstReleaseDate is recent enough
// (against cutoff) to warrant a fresh ReleasesByReleaseGroup fetch. Compares
// strings only; every ambiguous case resolves to true (still check), like
// isGuestFeature's doctrine: under 4 chars (absent/malformed) returns true;
// otherwise compare truncated to the shorter operand, so a year-only date is
// judged at its own precision; >= not >, so a group dated exactly on the cutoff
// is still checked; a garbage or oddly-shaped date sorts above the cutoff -- an
// extra fetch, never a silent skip.
func withinDeluxeRecheckWindow(firstReleaseDate, cutoff string) bool {
	if len(firstReleaseDate) < 4 {
		return true
	}
	n := min(len(firstReleaseDate), len(cutoff))
	return firstReleaseDate[:n] >= cutoff[:n]
}

// guestFeatureArt returns the first non-empty release-group MBID in releases (by
// range, never a fixed index -- T-04-12, ASVS V5) and its Cover Art Archive URL,
// or two empty strings when none carries one (D-03's fallback). Independent of
// earliestReleaseDate -- the art source need not be the same release.
func guestFeatureArt(releases []musicbrainz.RecordingRelease) (releaseGroupMBID string, coverArtURL string) {
	for _, r := range releases {
		if r.ReleaseGroup.MBID != "" {
			return r.ReleaseGroup.MBID, coverArtURLForReleaseGroup(r.ReleaseGroup.MBID)
		}
	}
	return "", ""
}

// seenExternalIDs returns the external ids already recorded for artistID under
// source/eventType -- the "seen" half of the fresh-vs-seen diff (D-10).
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

// releaseTypeForStorage applies the same lowercase/trim normalization
// releaseTypeAllowed uses, so a stored release_type agrees with the preference
// axis that admitted the row. nullableString maps an absent PrimaryType to SQL
// NULL, not "" (D-04, 05-RESEARCH.md Pitfall 3); title-casing is display's job.
func releaseTypeForStorage(primaryType string) *string {
	return nullableString(strings.ToLower(strings.TrimSpace(primaryType)))
}

// coverArtURLForReleaseGroup builds the deterministic Cover Art Archive URL for
// a release-group MBID (D-12, 04-RESEARCH.md Pitfall #6) -- no extra HTTP call.
func coverArtURLForReleaseGroup(mbid string) string {
	return "https://coverartarchive.org/release-group/" + mbid + "/front"
}
