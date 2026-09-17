// digest_format.go builds the scheduled digest's single-embed body (D-05).
//
// Task 1 of plan 22-03 closed T-22-07: community-editable artist/title text
// is now rune-capped and backslash-escaped before it reaches the Description,
// and eventURL (format.go) is the one shared switch deciding every line's
// link. Task 2 still owes alphabetical collation sorting (D-06/D-22), the
// guest-feature host credit (D-20), and the deluxe track-count suffix
// (D-26). buildDigestEmbed's signature, its call site in digest.go, and the
// single-embed shape below do not change when those land -- discord.Client.Send
// needs no interface change either.
package notifier

import (
	"strings"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)

// markdownEscaper backslash-escapes Discord's markdown metacharacters so
// community-editable text (an artist name or title) can never terminate or
// retarget a masked link, nor bleed emphasis into a heading (D-21, closes
// T-22-07). The backslash pair is listed first so a reviewer can see the
// escape-the-escaper rule is honored; strings.NewReplacer performs one pass
// over the source and never rescans its own replacement output, so pair
// order cannot cause a double-escape regardless.
var markdownEscaper = strings.NewReplacer(
	`\`, `\\`,
	`*`, `\*`,
	`_`, `\_`,
	`~`, `\~`,
	"`", "\\`",
	`|`, `\|`,
	`>`, `\>`,
	`#`, `\#`,
	`[`, `\[`,
	`]`, `\]`,
	`(`, `\(`,
	`)`, `\)`,
)

// escapeMarkdown applies markdownEscaper to s.
func escapeMarkdown(s string) string {
	return markdownEscaper.Replace(s)
}

// digestTitleLimit caps a digest line's title at 100 runes before escaping
// -- Description is capped at 4096 characters and every line's URL counts
// toward it (D-21), so per-line length is a real budget, not only
// cosmetics.
const digestTitleLimit = 100

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
			title := escapeMarkdown(truncateRunes(ev.Title, digestTitleLimit))
			artist := escapeMarkdown(digestArtistName(ev))
			b.WriteString("- [")
			b.WriteString(artist)
			b.WriteString(" — ")
			b.WriteString(title)
			b.WriteString("](")
			b.WriteString(eventURL(ev))
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
