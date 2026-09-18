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
// group's lines land in more than one chunk. title is the bare heading
// text with no bold markers -- continuationHeading/continuationNote need
// it undecorated to build their own wording (D-09/D-10).
type digestGroup struct {
	title   string
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
		groups = append(groups, digestGroup{title: h.title, heading: "**" + h.title + "**\n", lines: lines})
	}
	return groups
}

// chunkDigest walks groups in D-06's amended group-preferred fill order:
// append whole groups until the next group would overflow the current
// chunk, then break there and start a fresh chunk with that group -- so a
// chunk boundary falls between two whole groups in the common case, never
// inside one. The one exception is a group that alone exceeds
// chunkContentBudget: splitOversizedGroup is the whole-line-boundary
// fallback for that rare case (D-06's legality rule, unchanged from the
// pre-group-preferred splitter and kept intact rather than deleted, since
// it is the floor the rare oversized group lands on). A group that does
// not fit even an empty chunk is line-split immediately rather than
// deferred to a next chunk with the same problem -- the loop's
// termination argument. D-08 makes the resulting partly-empty messages
// irrelevant, so no packing heuristic recovers the unused remainder.
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

	for _, g := range groups {
		rendered, ids, groupLen := renderGroup(g)

		switch {
		case groupLen > chunkContentBudget:
			// The only case D-06 allows a boundary inside a group.
			flush()
			chunks = append(chunks, splitOversizedGroup(g)...)
		case curLen > 0 && curLen+1+groupLen > chunkContentBudget:
			// Fits an empty chunk but not this chunk's remainder: break
			// here rather than spilling the next group's lines into the
			// gap (D-06's group-preferred policy).
			flush()
			cur.WriteString(rendered)
			curIDs = append(curIDs, ids...)
			curLen = groupLen
		default:
			// Fits (either into the current chunk's remainder, or fresh).
			if curLen > 0 {
				cur.WriteString("\n")
				curLen++
			}
			cur.WriteString(rendered)
			curIDs = append(curIDs, ids...)
			curLen += groupLen
		}
	}
	flush()
	return chunks
}

// renderGroup concatenates one group's heading and every line with no
// leading separator, returning the ids in the same order -- the atomic
// unit chunkDigest's group-preferred fast path packs whole.
func renderGroup(g digestGroup) (text string, ids []int64, runeLen int) {
	var b strings.Builder
	b.WriteString(g.heading)
	ids = make([]int64, 0, len(g.lines))
	for _, l := range g.lines {
		b.WriteString(l.text)
		ids = append(ids, l.id)
	}
	text = b.String()
	return text, ids, utf8.RuneCountInString(text)
}

// splitOversizedGroup is D-06's whole-line-boundary fallback for the one
// group that alone exceeds chunkContentBudget -- the pre-group-preferred
// splitter's algorithm, now scoped to a single group instead of the full
// groups slice, and stamping D-09/D-10's continuation markers at both ends
// of every mid-group cut: the chunk being left carries continuationNote as
// its last line, and the chunk being resumed opens with continuationHeading
// in place of the group's ordinary heading. A chunk boundary that falls
// between two whole groups (chunkDigest's common case) never calls this
// function and so never carries either marker. A line whose own rendered
// unit still can't fit an empty chunk degrades via oversizedLineNote
// (D-19) so the loop provably terminates and the event still acks;
// resuming is left true afterward so the (rare, nested) next line still
// picks up the continuation heading.
func splitOversizedGroup(g digestGroup) []digestChunk {
	var chunks []digestChunk
	var cur strings.Builder
	var curIDs []int64
	curLen := 0
	resuming := false

	flushMidGroup := func() {
		if curLen == 0 {
			return
		}
		cur.WriteString(continuationNote(g.title))
		chunks = append(chunks, digestChunk{description: cur.String(), ids: curIDs})
		cur.Reset()
		curIDs = nil
		curLen = 0
		resuming = true
	}

	flushFinal := func() {
		if curLen == 0 {
			return
		}
		chunks = append(chunks, digestChunk{description: cur.String(), ids: curIDs})
		cur.Reset()
		curIDs = nil
		curLen = 0
	}

	for i, entry := range g.lines {
		heading := ""
		switch {
		case i == 0:
			heading = g.heading
		case resuming:
			heading = continuationHeading(g.title) + "\n"
		}
		unit := heading + entry.text
		n := utf8.RuneCountInString(unit)

		if curLen > 0 && curLen+n > chunkContentBudget {
			flushMidGroup()
			heading = continuationHeading(g.title) + "\n"
			unit = heading + entry.text
			n = utf8.RuneCountInString(unit)
		}
		resuming = false

		if curLen == 0 && n > chunkContentBudget {
			note := oversizedLineNote()
			budget := chunkContentBudget - utf8.RuneCountInString(note)
			if budget < 0 {
				budget = 0
			}
			chunks = append(chunks, digestChunk{description: truncateRunes(unit, budget) + note, ids: []int64{entry.id}})
			resuming = true
			continue
		}

		cur.WriteString(unit)
		curIDs = append(curIDs, entry.id)
		curLen += n
	}
	flushFinal()
	return chunks
}

// continuationHeading renders D-09's "(continued)" heading, stamped at the
// top of any chunk where a group resumes after a line-boundary split --
// same bold style digestHeadings' plain heading uses, exact wording
// 23-CONTEXT.md's <specifics> locks (e.g. "**Guest Features (continued)**").
// Never passed through escapeMarkdown: title is a compile-time
// digestHeadings literal, not event-derived text.
func continuationHeading(title string) string {
	return "**" + title + " (continued)**"
}

// continuationNote renders D-10's trailing note, appended as the last line
// of the chunk a group's remaining lines are cut away from. It names the
// group and says it continues in the next message, but never a message
// number or total (regex-verified digit-free by its own test): the note
// is composed and sent before the next message exists, so a failed send
// or an expired grace window would leave a numbered pointer dangling with
// nothing on the other end. The "(continued)" heading half is verifiable
// at send time; this half is a best-effort forward reference only -- D-10
// reduces that hazard, it does not remove it. Never passed through
// escapeMarkdown, for the same reason continuationHeading is not.
func continuationNote(title string) string {
	return "\n*" + title + " continues in the next message*\n"
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

// positionIndicator renders D-21's " · (N/Total)" fragment, empty when
// total is 1 -- so an ordinary single-message digest differs from the
// pre-phase output by exactly one line (D-21), the cheapest possible
// regression surface. Total means chunks in this run (D-07 amended), not
// an uncapped count a later run might never reach.
func positionIndicator(n, total int) string {
	if total <= 1 {
		return ""
	}
	return fmt.Sprintf(" · (%d/%d)", n, total)
}

// buildDigestChunks is the orchestrator: group, split, then stamp D-04's
// self-contained window header plus D-21's position indicator onto every
// chunk (not only the first) so a message arriving out of order, or alone,
// still states the window it covers and its place in the run. This is a
// single forward pass spending chunkOverheadReserve (D-20/D-07 amended) --
// never a second pass that re-measures the split after stamping, since
// widening the indicator (e.g. "(9/9)" to "(10/10)") could otherwise push
// a line out of a chunk and change Total, which is the exact fixed-point
// bug D-07 forbids reintroducing. Callers must never invoke this with an
// empty events slice -- digest.go's empty-skip branch short-circuits
// before message assembly for that case, exactly as the former
// buildDigestEmbed documented.
func buildDigestChunks(events []sqlc.Event, lastSentAt *time.Time) []digestChunk {
	groups := buildDigestGroups(events)
	chunks := chunkDigest(groups)
	base := digestWindowHeader(lastSentAt)
	total := len(chunks)
	for i := range chunks {
		header := base + positionIndicator(i+1, total) + "\n"
		chunks[i].description = header + chunks[i].description
	}
	return chunks
}
