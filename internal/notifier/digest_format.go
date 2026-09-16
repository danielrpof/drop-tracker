// digest_format.go builds the scheduled digest's single-embed body (D-05).
//
// This is a first cut, not the finished shape: plan 22-03 supplies markdown
// escaping (D-21), alphabetical collation sorting (D-06/D-22), the
// guest-feature host credit (D-20), and the deluxe track-count suffix
// (D-26) before the phase closes. buildDigestEmbed's signature, its call
// site in digest.go, and the single-embed shape below do not change when
// those land -- discord.Client.Send needs no interface change either.
package notifier

import (
	"strings"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)

// digestHeading pairs one event type with its fixed display heading, in
// D-04's fixed order: New Releases, Guest Features, Deluxe Changes.
type digestHeading struct {
	eventType string
	title     string
}

var digestHeadings = []digestHeading{
	{eventTypeNewRelease, "New Releases"},
	{eventTypeGuestFeature, "Guest Features"},
	{eventTypeDeluxeChange, "Deluxe Changes"},
}

// buildDigestEmbed assembles D-05's single embed: one Description carrying
// every sendable event grouped under its type's bold heading, in
// digestHeadings' fixed order, preserving ListUnnotified's order within
// each group (collation sorting is plan 22-03). A heading whose group has
// zero events is omitted entirely. Event content lives in Description text
// only -- the embed's structured-field array is never populated here,
// which is what lets one embed carry an entire digest instead of one event
// per message (D-05). Callers
// must never invoke this with an empty events slice; digest.go's empty-skip
// branch short-circuits before message assembly for that case.
func buildDigestEmbed(events []sqlc.Event) discord.Embed {
	grouped := make(map[string][]sqlc.Event, len(digestHeadings))
	for _, ev := range events {
		grouped[ev.EventType] = append(grouped[ev.EventType], ev)
	}

	var b strings.Builder
	for _, h := range digestHeadings {
		group := grouped[h.eventType]
		if len(group) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("**")
		b.WriteString(h.title)
		b.WriteString("**\n")
		for _, ev := range group {
			b.WriteString("- [")
			b.WriteString(digestArtistName(ev))
			b.WriteString(" — ")
			b.WriteString(ev.Title)
			b.WriteString("](")
			b.WriteString(digestEventURL(ev))
			b.WriteString(")\n")
		}
	}

	return discord.Embed{Description: b.String()}
}

// digestArtistName derives one digest line's display artist (D-20):
// WatchedArtistName when non-nil and non-empty, falling back to
// ArtistName. Migration 000007 backfilled every existing row, so the
// fallback is a guard, not a normal path.
func digestArtistName(ev sqlc.Event) string {
	if ev.WatchedArtistName != nil && *ev.WatchedArtistName != "" {
		return *ev.WatchedArtistName
	}
	return ev.ArtistName
}

// digestEventURL derives one line's link URL by event type, reusing
// format.go's existing external_id-based URL helpers -- never Title or
// ArtistName (community-editable free text never builds a URL, T-05-06).
// Plan 22-03 extracts this switch into a shared helper formatEmbed also
// calls; duplicating it here is deliberate for this plan's scope.
func digestEventURL(ev sqlc.Event) string {
	switch ev.EventType {
	case eventTypeNewRelease:
		return newReleaseURL(ev.Source, ev.ExternalID)
	case eventTypeGuestFeature:
		return musicBrainzRecordingURL(ev.ExternalID)
	case eventTypeDeluxeChange:
		return musicBrainzReleaseURL(ev.ExternalID)
	default:
		return ""
	}
}
