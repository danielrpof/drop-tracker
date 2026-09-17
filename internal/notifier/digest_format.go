// digest_format.go builds the scheduled digest's single-embed body (D-05).
//
// Community-editable artist/title text is rune-capped and backslash-escaped
// before it reaches the Description (D-21, closes T-22-07), eventURL
// (format.go) is the one shared switch deciding every line's link (T-22-14),
// each group sorts by collated watched artist with a total tie-break
// (D-06/D-22), guest-feature lines carry the host credit (D-20), and deluxe
// lines carry the track-count suffix (D-26). buildDigestEmbed's signature,
// its call site in digest.go, and the single-embed shape do not change --
// discord.Client.Send needs no interface change either.
package notifier

import (
	"sort"
	"strings"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

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
// digestHeadings' fixed order, each group sorted by collated watched artist
// with a total tie-break (D-06/D-22) so the rendered output is fully
// determined by the input set and never by ListUnnotified's arrival order.
// A heading whose group has zero events is omitted entirely. Event content
// lives in Description text only -- the embed's structured-field array is
// never populated here, which is what lets one embed carry an entire digest
// instead of one event per message (D-05). Callers must never invoke this
// with an empty events slice; digest.go's empty-skip branch short-circuits
// before message assembly for that case.
func buildDigestEmbed(events []sqlc.Event) discord.Embed {
	grouped := make(map[string][]sqlc.Event, len(digestHeadings))
	for _, ev := range events {
		grouped[ev.EventType] = append(grouped[ev.EventType], ev)
	}

	// One collator per call, never a package-level value: collate.Collator
	// is not safe for concurrent use, and two schedulers or a scheduler
	// plus a test could otherwise share one (D-22).
	collator := collate.New(language.Und, collate.IgnoreCase)

	var b strings.Builder
	for _, h := range digestHeadings {
		group := grouped[h.eventType]
		if len(group) == 0 {
			continue
		}
		sortDigestGroup(group, collator)

		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("**")
		b.WriteString(h.title)
		b.WriteString("**\n")
		for _, ev := range group {
			b.WriteString("- [")
			b.WriteString(lineLabel(ev))
			b.WriteString("](")
			b.WriteString(eventURL(ev))
			b.WriteString(")")
			if h.eventType == eventTypeDeluxeChange {
				// D-26: the track-count suffix is appended after the link's
				// closing parenthesis, and omitted entirely (never a bare
				// "()") when neither count is known.
				if suffix := tracksFieldValue(ev.PreviousTrackCount, ev.TrackCount); suffix != "" {
					b.WriteString(" (")
					b.WriteString(suffix)
					b.WriteString(")")
				}
			}
			b.WriteString("\n")
		}
	}

	return discord.Embed{Description: b.String()}
}

// sortDigestGroup orders one event-type group by collated artistKey, tied
// broken by collated Title, then by ascending event ID. ev.ID is unique, so
// this ordering is total: the rendered output is fully determined by the
// input set, never by the order events arrived in (D-06/D-22). No
// deduplication happens here -- two events describing the same release from
// two sources, or two events for the same artist within one group, stay two
// separate lines (D-26).
func sortDigestGroup(group []sqlc.Event, collator *collate.Collator) {
	sort.SliceStable(group, func(i, j int) bool {
		a, b := group[i], group[j]
		if c := collator.CompareString(artistKey(a), artistKey(b)); c != 0 {
			return c < 0
		}
		if c := collator.CompareString(a.Title, b.Title); c != 0 {
			return c < 0
		}
		return a.ID < b.ID
	})
}

// artistKey returns one event's watched-artist sort/display key (D-20):
// WatchedArtistName when non-nil and non-empty, falling back to
// ArtistName. Keying a guest_feature line on artist_name would file it
// under the host credit -- an artist the operator does not watch -- and
// hide which watched artist is actually on the track. Migration 000007
// backfilled every existing row, so the fallback is a guard, not a normal
// path.
func artistKey(ev sqlc.Event) string {
	if ev.WatchedArtistName != nil && *ev.WatchedArtistName != "" {
		return *ev.WatchedArtistName
	}
	return ev.ArtistName
}

// lineLabel builds one digest line's escaped, rune-capped label. A
// guest_feature line also names the host credit -- artist_name on those
// rows is the host's primary credit, not the watched artist (D-20) -- in
// the form "Watched on Host — Title", using the em dash (U+2014) per
// 22-CONTEXT.md's <specifics> shape. The host credit is escaped too: it is
// the same community-editable artist_name column.
func lineLabel(ev sqlc.Event) string {
	title := escapeMarkdown(truncateRunes(ev.Title, digestTitleLimit))
	watched := escapeMarkdown(artistKey(ev))
	if ev.EventType == eventTypeGuestFeature {
		host := escapeMarkdown(ev.ArtistName)
		return watched + " on " + host + " — " + title
	}
	return watched + " — " + title
}
