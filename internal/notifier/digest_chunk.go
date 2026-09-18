// digest_chunk.go splits a digest's grouped, sorted event lines into one or
// more Discord-message-sized chunks (DGST-11/DGST-12). buildDigestGroups
// carries forward digest_format.go's grouping/sorting rules unchanged;
// chunkDigest is the pure, whole-line-only splitter (D-06's legality rule --
// group-preferred fill policy is plan 23-03's expansion); buildDigestChunks
// orchestrates both and stamps D-04's self-contained window header onto
// every chunk it returns. Each digestEntry carries its event id alongside
// its rendered text so a chunk's ids slice always matches exactly what its
// description rendered (D-16, D-19) -- the property per-chunk acking
// depends on.
package notifier

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// discordDescriptionLimit is Discord's documented ceiling on an embed's
// Description field, in runes. chunkContentBudget (below) is always
// strictly under this -- no chunk's description can ever reach it.
const discordDescriptionLimit = 4096

// chunkOverheadReserve is D-20's single named worst-case per-chunk overhead,
// reserved *before* splitting so dropping any unneeded portion afterward
// only ever frees runes -- it can never make the ≤4096 invariant false.
// It sums the widest reasonable case of each budget line this and later
// plans stamp onto a chunk: (a) the longest header wording, the
// NULL-watermark case "Everything pending since digest mode was enabled"
// (~50 runes); (b) a three-digit "(N/Total)" position indicator, " ·
// (999/999)" (~15 runes, plan 23-03); (c) the longest trailing
// continuation note, "\n*Deluxe Changes* continues in the next message"
// (~60 runes, plan 23-03); (d) the longest capped-run remainder marker,
// "(20/20) · 999 events still pending, continuing in the next digest"
// (~70 runes, plan 23-04). Summed with headroom: 300.
const chunkOverheadReserve = 300

// chunkContentBudget is the real per-chunk rune budget chunkDigest splits
// against -- discordDescriptionLimit minus chunkOverheadReserve.
const chunkContentBudget = discordDescriptionLimit - chunkOverheadReserve

// digestChunkSpacing paces consecutive chunk sends within one
// SendDigestIfDue run (D-23) -- a digest-specific one-second constant, not
// n.spacing/spacingWait, which are sized for sporadic real-time sends and
// sit, by their own comment, at Discord's ceiling. A sustained multi-chunk
// burst through one webhook is a different regime.
const digestChunkSpacing = time.Second

// digestChunkWait is the seam SendDigestIfDue's inter-chunk select waits
// on, mirroring spacingWait's shape (notifier.go): a var, not time.After
// directly, so a test can substitute an already-fired channel and turn an
// elapsed-wall-clock assertion into a deterministic requested-duration one.
var digestChunkWait = time.After

// digestEntry is one rendered event line plus the id it came from -- the
// pairing per-chunk acking needs (D-16), which digest_format.go's former
// bare []string segments discarded.
type digestEntry struct {
	text string
	id   int64
}

// digestGroup is one event-type heading plus its collated-sorted lines.
// heading is its own string, not glued onto the first line (D-17) -- the
// chunker needs that boundary to decide whether/how to re-stamp it when a
// group's lines land in more than one chunk.
type digestGroup struct {
	heading string
	lines   []digestEntry
}

// digestChunk is one Discord message's worth of digest content: the
// rendered description (window header plus every line this chunk carries)
// and the ids that description rendered, in the same order they ack.
type digestChunk struct {
	description string
	ids         []int64
}

// buildDigestGroups groups events by type in digestHeadings' fixed order,
// omitting any heading whose group has zero events, and sorts each group by
// collated watched artist (sortDigestGroup) -- carried forward unchanged
// from the former buildDigestEmbed. One collate.Collator per call, never
// package-level: collate.Collator is not safe for concurrent use, and two
// schedulers (or a scheduler plus a test) could otherwise share one.
func buildDigestGroups(events []sqlc.Event) []digestGroup {
	grouped := make(map[string][]sqlc.Event, len(digestHeadings))
	for _, ev := range events {
		grouped[ev.EventType] = append(grouped[ev.EventType], ev)
	}

	collator := collate.New(language.Und, collate.IgnoreCase)

	groups := make([]digestGroup, 0, len(digestHeadings))
	for _, h := range digestHeadings {
		group := grouped[h.eventType]
		if len(group) == 0 {
			continue
		}
		sortDigestGroup(group, collator)

		lines := make([]digestEntry, 0, len(group))
		for _, ev := range group {
			lines = append(lines, digestEntry{text: digestLine(h.eventType, ev), id: ev.ID})
		}
		groups = append(groups, digestGroup{heading: "**" + h.title + "**\n", lines: lines})
	}
	return groups
}

// chunkDigest walks groups in order and packs heading+lines into
// chunkContentBudget-sized chunks, never cutting inside a rendered line
// (D-06's legality rule -- group-preferred fill policy is plan 23-03's
// expansion, not this function's job). Each appended line's id is carried
// into the current chunk's ids in the same step, never a second pass. A
// single line that alone exceeds chunkContentBudget (D-18 makes this
// unreachable in practice) is emitted truncated as its own chunk via
// oversizedLineNote, so the loop provably terminates and the event still
// acks (D-19).
func chunkDigest(groups []digestGroup) []digestChunk {
	var chunks []digestChunk
	var cur strings.Builder
	var curIDs []int64
	curLen := 0

	flush := func() {
		if curLen == 0 {
			return
		}
		chunks = append(chunks, digestChunk{description: cur.String(), ids: curIDs})
		cur.Reset()
		curIDs = nil
		curLen = 0
	}

	// appendLine is the sole place a rendered unit (a line, optionally
	// heading-prefixed) lands in the output -- flushing to a fresh chunk
	// when it wouldn't fit, and degrading to the oversized-line floor when
	// it still wouldn't fit alone.
	appendLine := func(text string, id int64, heading string) {
		unit := text
		if heading != "" {
			h := heading
			if curLen > 0 {
				h = "\n" + h
			}
			unit = h + text
		}
		n := utf8.RuneCountInString(unit)

		if curLen > 0 && curLen+n > chunkContentBudget {
			flush()
			// Fresh chunk: the heading (if any) no longer needs a leading
			// separator, since it is now first in its own chunk.
			if heading != "" {
				unit = heading + text
			} else {
				unit = text
			}
			n = utf8.RuneCountInString(unit)
		}

		if curLen == 0 && n > chunkContentBudget {
			note := oversizedLineNote()
			budget := chunkContentBudget - utf8.RuneCountInString(note)
			if budget < 0 {
				budget = 0
			}
			chunks = append(chunks, digestChunk{description: truncateRunes(unit, budget) + note, ids: []int64{id}})
			return
		}

		cur.WriteString(unit)
		curIDs = append(curIDs, id)
		curLen += n
	}

	for _, g := range groups {
		for i, entry := range g.lines {
			heading := ""
			if i == 0 {
				heading = g.heading
			}
			appendLine(entry.text, entry.id, heading)
		}
	}
	flush()
	return chunks
}

// digestWindowHeader renders D-03's two locked wordings: a Discord
// relative-timestamp token when lastSentAt is known, or the NULL-watermark
// wording naming the real reason there is no prior watermark. This string
// is never passed through escapeMarkdown (T-23-02) -- markdownEscaper's `>`
// and `#` rules would turn the `<t:...:R>` token into literal text, and no
// event-derived text ever reaches this function.
func digestWindowHeader(lastSentAt *time.Time) string {
	if lastSentAt == nil {
		return "Everything pending since digest mode was enabled"
	}
	return fmt.Sprintf("Everything pending since <t:%d:R>", lastSentAt.Unix())
}

// buildDigestChunks is the orchestrator: group, split, then stamp D-04's
// self-contained window header onto every chunk (not only the first) so a
// message arriving out of order, or alone, still states the window it
// covers. Callers must never invoke it with an empty events slice --
// digest.go's empty-skip branch short-circuits before message assembly for
// that case, exactly as the former buildDigestEmbed documented.
func buildDigestChunks(events []sqlc.Event, lastSentAt *time.Time) []digestChunk {
	groups := buildDigestGroups(events)
	chunks := chunkDigest(groups)
	header := digestWindowHeader(lastSentAt) + "\n"
	for i := range chunks {
		chunks[i].description = header + chunks[i].description
	}
	return chunks
}
