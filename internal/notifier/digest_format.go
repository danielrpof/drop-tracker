// digest_format.go builds the scheduled digest's single-embed body (D-05).
//
// Community-editable artist/title text is rune-capped and backslash-escaped
// before it reaches the Description (D-21, closes T-22-07), eventURL
// (format.go) is the one shared switch deciding every line's link (T-22-14),
// each group sorts by collated watched artist with a total tie-break
// (D-06/D-22), guest-feature lines carry the host credit (D-20), and deluxe
// lines carry the track-count suffix (D-26). buildDigestEmbed's signature,
// its call site in digest.go, and the single-embed shape do not change --
// discord.Client.Send needs no interface change either. The whole Description
// is now capped at discordDescriptionLimit (closes T-22-15/CR-01): an
// oversized batch truncates at a full line boundary instead of letting
// Discord reject the request.
package notifier

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

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
// cosmetics. That whole-Description cap is enforced by assembleDescription
// below (T-22-15) -- this per-line cap alone was never sufficient once a
// digest slot's event count grew large enough.
const digestTitleLimit = 100

// discordDescriptionLimit is Discord's documented ceiling on an embed's
// Description field, in runes. assembleDescription never returns more than
// this many runes.
const discordDescriptionLimit = 4096

// truncationNoteReserve is a generous rune-count headroom for
// assembleDescription's trailing note; used only to size a preallocated
// builder, never as a second hard limit -- the real invariant is checked
// exactly against discordDescriptionLimit.
const truncationNoteReserve = 40

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

	segments := make([]string, 0, len(events))
	for _, h := range digestHeadings {
		group := grouped[h.eventType]
		if len(group) == 0 {
			continue
		}
		sortDigestGroup(group, collator)

		for i, ev := range group {
			line := digestLine(h.eventType, ev)
			if i == 0 {
				// The group's first segment carries its leading separator
				// ("\n" only when this is not the very first segment
				// overall) plus the bold heading -- the same rule the prior
				// shared-builder version expressed via `b.Len() > 0`, now
				// keyed off `len(segments) > 0`.
				heading := "**" + h.title + "**\n"
				if len(segments) > 0 {
					heading = "\n" + heading
				}
				line = heading + line
			}
			segments = append(segments, line)
		}
	}

	return discord.Embed{Description: assembleDescription(segments)}
}

// digestLine renders exactly one event's line -- "- [label](url)" plus the
// deluxe track-count suffix -- ending in "\n". Extracted from
// buildDigestEmbed's former inner-loop body so assembleDescription can
// truncate on a whole-line boundary.
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

// assembleDescription joins segments (one per event, a group's first
// segment also carrying that group's leading separator + heading) into the
// final Description. When the full join already fits discordDescriptionLimit
// runes, the result is byte-identical to a plain strings.Join -- no note, no
// truncation (T-22-15's "unaffected when under budget" contract). Otherwise
// it keeps the largest whole-segment prefix whose rune count, plus
// truncationNote's rune count for however many segments that prefix omits,
// stays within discordDescriptionLimit, and appends that note. It never cuts
// inside a segment: dropped segments are dropped whole.
func assembleDescription(segments []string) string {
	cum := make([]int, len(segments)+1)
	for i, s := range segments {
		cum[i+1] = cum[i] + utf8.RuneCountInString(s)
	}
	if cum[len(segments)] <= discordDescriptionLimit {
		return strings.Join(segments, "")
	}

	for k := len(segments) - 1; k >= 0; k-- {
		note := truncationNote(len(segments) - k)
		if cum[k]+utf8.RuneCountInString(note) <= discordDescriptionLimit {
			var b strings.Builder
			b.Grow(discordDescriptionLimit + truncationNoteReserve)
			for _, s := range segments[:k] {
				b.WriteString(s)
			}
			b.WriteString(note)
			return b.String()
		}
	}
	// Pathological case: even omitting every segment, the note itself would
	// not fit. Return the note alone as the best-effort result -- there is
	// nothing smaller to fall back to.
	return truncationNote(len(segments))
}

// truncationNote reports how many sendable events assembleDescription
// dropped. SendDigestIfDue's ack step does not consult Description content
// -- every dropped event's id is still acked as delivered once Send succeeds
// (digest.go) -- so this note exists only so the operator can see the digest
// is incomplete.
func truncationNote(omitted int) string {
	if omitted == 1 {
		return "... 1 more event"
	}
	return fmt.Sprintf("... %d more events", omitted)
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
