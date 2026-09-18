// digest_format.go renders one event's digest line and label (D-05):
// community-editable artist/title text is rune-capped and
// backslash-escaped before it reaches a line (D-21, closes T-22-07),
// eventURL (format.go) is the one shared switch deciding every line's link
// (T-22-14), each group sorts by collated watched artist with a total
// tie-break (D-06/D-22), guest-feature lines carry the host credit (D-20),
// and deluxe lines carry the track-count suffix (D-26). Grouping, chunk
// splitting, and the window header now live in digest_chunk.go (D-17) --
// this file keeps only line/label/sort rendering, the per-chunk-consumed
// unit the chunker builds on.
package notifier

import (
	"sort"
	"strings"

	"golang.org/x/text/collate"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
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
// -- a chunk's content budget is finite and every line's URL counts toward
// it (D-21), so per-line length is a real budget, not only cosmetics.
const digestTitleLimit = 100

// digestArtistLimit caps a digest line's watched-artist name and (on a
// guest_feature line) its host credit at 60 runes before escaping, digest
// path only (D-18) -- formatEmbed's real-time path has its own caps and is
// untouched. events.artist_name/watched_artist_name are unbounded TEXT, so
// without this a single line has no upper bound, and D-06 forbids a
// mid-line chunk boundary -- an oversized line would otherwise have no
// legal placement. Capping both fields bounds the worst-case line to well
// under chunkContentBudget.
const digestArtistLimit = 60

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

// digestLine renders exactly one event's line -- "- [label](url)" plus the
// deluxe track-count suffix -- ending in "\n". Its output is the atomic
// unit chunkDigest (digest_chunk.go) never cuts inside.
func digestLine(eventType string, ev sqlc.Event) string {
	var b strings.Builder
	b.WriteString("- [")
	b.WriteString(lineLabel(ev))
	b.WriteString("](")
	b.WriteString(eventURL(ev))
	b.WriteString(")")
	if eventType == eventTypeDeluxeChange {
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
	return b.String()
}

// oversizedLineNote is D-19's floor for a single rendered line that alone
// exceeds chunkContentBudget (digest_chunk.go's chunkDigest): the line is
// rune-truncated and this note appended, so that chunk stays within budget
// and the splitter provably terminates rather than looping or wedging. The
// event still acks -- an event that renders but never acks re-enters every
// future digest without bound. digestArtistLimit/digestTitleLimit make this
// path unreachable in practice, but it must still exist as a legal floor.
func oversizedLineNote() string {
	return " ...(truncated)\n"
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
	watched := escapeMarkdown(truncateRunes(artistKey(ev), digestArtistLimit))
	if ev.EventType == eventTypeGuestFeature {
		host := escapeMarkdown(truncateRunes(ev.ArtistName, digestArtistLimit))
		return watched + " on " + host + " — " + title
	}
	return watched + " — " + title
}
