package notifier

// This file is package notifier (whitebox), matching digest_format_test.go's
// convention -- digestWindowHeader, chunkDigest, buildDigestGroups, and
// buildDigestChunks are all unexported. Pure table-driven/property-style
// unit tests, no DB, no HTTP.

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

// TestDigestWindowHeader_NonNilLastSentAt pins D-01/D-03's timestamped
// wording, built from lastSentAt.Unix() rendered in decimal.
func TestDigestWindowHeader_NonNilLastSentAt(t *testing.T) {
	ts := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	want := fmt.Sprintf("Everything pending since <t:%d:R>", ts.Unix())
	if got := digestWindowHeader(&ts); got != want {
		t.Fatalf("digestWindowHeader(%v) = %q, want %q", ts, got, want)
	}
}

// TestDigestWindowHeader_NilLastSentAt pins D-03's NULL-watermark wording.
func TestDigestWindowHeader_NilLastSentAt(t *testing.T) {
	want := "Everything pending since digest mode was enabled"
	if got := digestWindowHeader(nil); got != want {
		t.Fatalf("digestWindowHeader(nil) = %q, want %q", got, want)
	}
}

// TestDigestWindowHeader_NoBackslash proves the header never passed through
// escapeMarkdown -- markdownEscaper's `>` and `#` rules would turn the
// `<t:...:R>` token into literal text (T-23-02).
func TestDigestWindowHeader_NoBackslash(t *testing.T) {
	ts := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	for _, got := range []string{digestWindowHeader(&ts), digestWindowHeader(nil)} {
		if strings.Contains(got, `\`) {
			t.Fatalf("digestWindowHeader output %q contains a backslash -- it must never pass through escapeMarkdown", got)
		}
	}
}

// syntheticEvents builds n distinct new_release events with long-enough
// titles/artists to force chunkDigest into more than one chunk.
func syntheticEvents(n int) []sqlc.Event {
	events := make([]sqlc.Event, n)
	for i := 0; i < n; i++ {
		events[i] = sqlc.Event{
			ID:                int64(i + 1),
			EventType:         eventTypeNewRelease,
			Source:            sourceMusicBrainz,
			ExternalID:        fmt.Sprintf("nr-%04d", i),
			WatchedArtistName: strPtr(fmt.Sprintf("Artist %04d", i)),
			Title:             fmt.Sprintf("Album Title Number %04d With Extra Padding Text To Grow The Line", i),
		}
	}
	return events
}

// TestChunkDigest_UnderBudgetSingleChunk proves a batch whose total rendered
// size is under chunkContentBudget returns exactly one chunk, whose
// description is the plain concatenation of every group heading and line.
func TestChunkDigest_UnderBudgetSingleChunk(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist A"), Title: "Album A"},
		{ID: 2, EventType: eventTypeGuestFeature, WatchedArtistName: strPtr("Artist B"), ArtistName: "Host", Title: "Track B"},
	}
	groups := buildDigestGroups(events)
	chunks := chunkDigest(groups)

	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}

	var want strings.Builder
	for i, g := range groups {
		if i > 0 {
			want.WriteString("\n")
		}
		want.WriteString(g.heading)
		for _, line := range g.lines {
			want.WriteString(line.text)
		}
	}
	if chunks[0].description != want.String() {
		t.Fatalf("chunks[0].description = %q, want %q", chunks[0].description, want.String())
	}
}

// TestChunkDigest_OverBudgetMultipleChunks proves a synthetic batch large
// enough to need several chunks returns more than one chunk, every chunk's
// description is at most discordDescriptionLimit runes (chunkContentBudget
// plus D-20's reserved overhead, since this single-group batch triggers
// splitOversizedGroup's continuation-note stamping), and no chunk ends
// mid-line.
func TestChunkDigest_OverBudgetMultipleChunks(t *testing.T) {
	groups := buildDigestGroups(syntheticEvents(200))
	chunks := chunkDigest(groups)

	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want > 1", len(chunks))
	}
	for i, c := range chunks {
		if rc := utf8.RuneCountInString(c.description); rc > discordDescriptionLimit {
			t.Fatalf("chunk %d has %d runes, want <= %d", i, rc, discordDescriptionLimit)
		}
		if !strings.HasSuffix(c.description, "\n") {
			t.Fatalf("chunk %d description = %q, want it to end on a newline (no partial line)", i, c.description)
		}
	}
}

// TestChunkDigest_IDsUnionMatchesInput proves the union of every returned
// chunk's ids equals the input id set with no duplicate and no omission.
func TestChunkDigest_IDsUnionMatchesInput(t *testing.T) {
	events := syntheticEvents(200)
	groups := buildDigestGroups(events)
	chunks := chunkDigest(groups)

	seen := make(map[int64]int, len(events))
	for _, c := range chunks {
		for _, id := range c.ids {
			seen[id]++
		}
	}
	if len(seen) != len(events) {
		t.Fatalf("union has %d distinct ids, want %d", len(seen), len(events))
	}
	for _, ev := range events {
		if seen[ev.ID] != 1 {
			t.Fatalf("event id %d appears %d times across chunks, want exactly 1", ev.ID, seen[ev.ID])
		}
	}
}

// TestChunkDigest_ConcatenationReproducesLineOrder proves concatenating
// every chunk's line text, in chunk order, reproduces the same ordered line
// sequence a single unsplit render produces.
func TestChunkDigest_ConcatenationReproducesLineOrder(t *testing.T) {
	events := syntheticEvents(200)
	groups := buildDigestGroups(events)

	var wantIDOrder []int64
	for _, g := range groups {
		for _, line := range g.lines {
			wantIDOrder = append(wantIDOrder, line.id)
		}
	}

	chunks := chunkDigest(groups)
	var gotText strings.Builder
	var gotIDOrder []int64
	for _, c := range chunks {
		gotText.WriteString(c.description)
		gotIDOrder = append(gotIDOrder, c.ids...)
	}

	// The concatenation of every chunk's description contains every
	// rendered line exactly once, in the same order buildDigestGroups
	// produced -- headings and any oversized-line note are the only extra
	// content a chunk's description carries beyond the plain lines.
	if len(gotIDOrder) != len(wantIDOrder) {
		t.Fatalf("len(gotIDOrder) = %d, want %d", len(gotIDOrder), len(wantIDOrder))
	}
	for i := range wantIDOrder {
		if gotIDOrder[i] != wantIDOrder[i] {
			t.Fatalf("chunk id order[%d] = %d, want %d (order must match buildDigestGroups' line order)", i, gotIDOrder[i], wantIDOrder[i])
		}
	}
	for _, line := range wantLinesOf(groups) {
		if !strings.Contains(gotText.String(), line) {
			t.Fatalf("concatenated chunk text is missing line %q", line)
		}
	}
}

// wantLinesOf flattens every group's rendered lines in order, for
// TestChunkDigest_ConcatenationReproducesLineOrder's containment check.
func wantLinesOf(groups []digestGroup) []string {
	var lines []string
	for _, g := range groups {
		for _, line := range g.lines {
			lines = append(lines, line.text)
		}
	}
	return lines
}

// TestChunkDigest_SingleOversizedLineOwnChunkTerminates proves a single
// event whose rendered line alone exceeds chunkContentBudget still produces
// a chunk, that chunk's ids still contains that event's id, and chunkDigest
// terminates rather than looping (D-19).
func TestChunkDigest_SingleOversizedLineOwnChunkTerminates(t *testing.T) {
	huge := strings.Repeat("x", chunkContentBudget*2)
	events := []sqlc.Event{
		{ID: 42, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: huge},
	}
	groups := buildDigestGroups(events)

	done := make(chan []digestChunk, 1)
	go func() { done <- chunkDigest(groups) }()

	select {
	case chunks := <-done:
		if len(chunks) != 1 {
			t.Fatalf("len(chunks) = %d, want 1", len(chunks))
		}
		if rc := utf8.RuneCountInString(chunks[0].description); rc > chunkContentBudget {
			t.Fatalf("degraded chunk has %d runes, want <= %d", rc, chunkContentBudget)
		}
		if len(chunks[0].ids) != 1 || chunks[0].ids[0] != 42 {
			t.Fatalf("chunks[0].ids = %v, want [42]", chunks[0].ids)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("chunkDigest did not terminate on an oversized single line")
	}
}

// makeSyntheticGroup builds a digestGroup directly (bypassing
// buildDigestGroups) so a test can control exact chunk-boundary arithmetic
// -- each of n lines is lineLen runes long including its trailing "\n".
func makeSyntheticGroup(title string, startID int64, n, lineLen int) digestGroup {
	lines := make([]digestEntry, n)
	for i := 0; i < n; i++ {
		lines[i] = digestEntry{text: strings.Repeat("x", lineLen-1) + "\n", id: startID + int64(i)}
	}
	return digestGroup{title: title, heading: "**" + title + "**\n", lines: lines}
}

// containsID reports whether id is present in ids.
func containsID(ids []int64, id int64) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// TestChunkDigest_GroupPreferredBoundariesOnlyBetweenGroups proves that
// when three groups each individually fit chunkContentBudget but their
// combined size does not, every returned chunk contains only whole groups
// -- each group's heading and its complete line set land in exactly one
// chunk (D-06 group-preferred).
func TestChunkDigest_GroupPreferredBoundariesOnlyBetweenGroups(t *testing.T) {
	groups := []digestGroup{
		makeSyntheticGroup("G1", 1, 1, 1800),
		makeSyntheticGroup("G2", 101, 1, 1800),
		makeSyntheticGroup("G3", 201, 1, 1800),
	}
	chunks := chunkDigest(groups)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want > 1 to exercise a real boundary", len(chunks))
	}

	for _, g := range groups {
		found := -1
		for ci, c := range chunks {
			hasHeading := strings.Contains(c.description, g.heading)
			for _, entry := range g.lines {
				hasID := containsID(c.ids, entry.id)
				if hasID != hasHeading {
					t.Fatalf("chunk %d: heading %q present=%v but line id %d present=%v -- boundary fell inside a group", ci, g.heading, hasHeading, entry.id, hasID)
				}
			}
			if hasHeading {
				if found != -1 {
					t.Fatalf("group %q's heading appears in more than one chunk (%d and %d)", g.heading, found, ci)
				}
				found = ci
			}
		}
		if found == -1 {
			t.Fatalf("group %q's heading not found in any chunk", g.heading)
		}
	}
}

// TestChunkDigest_SecondGroupStartsFreshChunkRatherThanSpillingLines proves
// that when a first group fills most of a chunk and a second group would
// overflow the remainder, the second group starts an entirely fresh chunk
// rather than spilling only the lines that would have fit.
func TestChunkDigest_SecondGroupStartsFreshChunkRatherThanSpillingLines(t *testing.T) {
	// chunkContentBudget is 3796. G1 fills most of it (3600), leaving ~196
	// runes of remainder -- not enough for any of G2's 100-rune lines'
	// heading, but individually G2's lines would fit that remainder.
	g1 := makeSyntheticGroup("G1", 1, 1, 3600)
	g2 := makeSyntheticGroup("G2", 101, 5, 100)
	chunks := chunkDigest([]digestGroup{g1, g2})

	if len(chunks) != 2 {
		t.Fatalf("len(chunks) = %d, want 2", len(chunks))
	}
	if !strings.Contains(chunks[0].description, "**G1**") || strings.Contains(chunks[0].description, "**G2**") {
		t.Fatalf("chunks[0].description = %q, want only G1", chunks[0].description)
	}
	if !strings.Contains(chunks[1].description, "**G2**") || strings.Contains(chunks[1].description, "**G1**") {
		t.Fatalf("chunks[1].description = %q, want only G2, all in a fresh chunk", chunks[1].description)
	}
	for _, entry := range g2.lines {
		if !containsID(chunks[1].ids, entry.id) {
			t.Fatalf("chunks[1].ids = %v, want it to contain G2's line id %d", chunks[1].ids, entry.id)
		}
	}
}

// TestChunkDigest_OversizedGroupLineSplitDoesNotSwallowNextGroupHeading
// proves a group alone larger than chunkContentBudget is split across
// chunks on whole-line boundaries, and the following group (which fits)
// still gets its own visible heading rather than being absorbed into the
// oversized group's line-split output.
func TestChunkDigest_OversizedGroupLineSplitDoesNotSwallowNextGroupHeading(t *testing.T) {
	big := makeSyntheticGroup("Big", 1, 100, 100) // 100 * 100 = 10000 runes, > budget
	small := makeSyntheticGroup("Small", 1001, 1, 50)
	chunks := chunkDigest([]digestGroup{big, small})

	if len(chunks) < 3 {
		t.Fatalf("len(chunks) = %d, want >= 3 (multiple line-split chunks for Big, plus Small)", len(chunks))
	}
	last := chunks[len(chunks)-1]
	if !strings.Contains(last.description, "**Small**") {
		t.Fatalf("last chunk description = %q, want it to contain Small's heading", last.description)
	}
	if !containsID(last.ids, 1001) {
		t.Fatalf("last chunk ids = %v, want it to contain Small's line id 1001", last.ids)
	}

	seen := make(map[int64]int)
	for _, c := range chunks {
		for _, id := range c.ids {
			seen[id]++
		}
	}
	for i := int64(1); i <= 100; i++ {
		if seen[i] != 1 {
			t.Fatalf("Big's line id %d appears %d times across chunks, want exactly 1", i, seen[i])
		}
	}
	if seen[1001] != 1 {
		t.Fatalf("Small's line id 1001 appears %d times across chunks, want exactly 1", seen[1001])
	}
}

// TestChunkDigest_SeparatorOnlyOnNonFirstGroupInChunk proves a chunk's
// first group heading carries no leading newline, and a subsequent group
// heading in the same chunk carries exactly one (the corrected D-17 rule).
func TestChunkDigest_SeparatorOnlyOnNonFirstGroupInChunk(t *testing.T) {
	g1 := makeSyntheticGroup("First", 1, 1, 50)
	g2 := makeSyntheticGroup("Second", 101, 1, 50)
	chunks := chunkDigest([]digestGroup{g1, g2})
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1 (both groups small enough to share a chunk)", len(chunks))
	}
	desc := chunks[0].description
	if strings.HasPrefix(desc, "\n") {
		t.Fatalf("description = %q, first group heading must not carry a leading newline", desc)
	}
	g1Rendered, _, _ := renderGroup(g1)
	g2Rendered, _, _ := renderGroup(g2)
	want := g1Rendered + "\n" + g2Rendered
	if desc != want {
		t.Fatalf("description = %q, want %q (exactly one separator newline between the two whole-group renders)", desc, want)
	}
}

// TestChunkDigest_NoEmptyChunkNoEmptyIDs proves the degenerate cases above
// never produce a chunk with an empty description or an empty ids slice.
func TestChunkDigest_NoEmptyChunkNoEmptyIDs(t *testing.T) {
	groups := []digestGroup{
		makeSyntheticGroup("G1", 1, 1, 1800),
		makeSyntheticGroup("G2", 101, 1, 1800),
		makeSyntheticGroup("G3", 201, 1, 1800),
		makeSyntheticGroup("Big", 301, 100, 100),
	}
	chunks := chunkDigest(groups)
	for i, c := range chunks {
		if c.description == "" {
			t.Fatalf("chunk %d has an empty description", i)
		}
		if len(c.ids) == 0 {
			t.Fatalf("chunk %d has an empty ids slice", i)
		}
	}
}

// TestContinuationHeading_ExactWording pins D-09's exact locked shape.
func TestContinuationHeading_ExactWording(t *testing.T) {
	want := "**Guest Features (continued)**"
	if got := continuationHeading("Guest Features"); got != want {
		t.Fatalf("continuationHeading(%q) = %q, want %q", "Guest Features", got, want)
	}
}

// TestContinuationNote_NoDigit proves D-10's amended trailing note names no
// message number: no digit anywhere in its output.
func TestContinuationNote_NoDigit(t *testing.T) {
	re := regexp.MustCompile(`^[^0-9]*$`)
	for _, title := range []string{"New Releases", "Guest Features", "Deluxe Changes"} {
		got := continuationNote(title)
		if !re.MatchString(got) {
			t.Fatalf("continuationNote(%q) = %q, want no digit anywhere", title, got)
		}
		if !strings.Contains(got, title) {
			t.Fatalf("continuationNote(%q) = %q, want it to name the group", title, got)
		}
	}
}

// TestChunkDigest_OversizedGroupContinuationMarkersAtBothEnds proves a
// group split across two chunks carries D-09's "(continued)" heading on
// the resuming chunk and D-10's trailing note on the chunk it was cut
// away from, and that the note contains no digit.
func TestChunkDigest_OversizedGroupContinuationMarkersAtBothEnds(t *testing.T) {
	g := makeSyntheticGroup("Guest Features", 1, 60, 100) // 6000 runes, > budget
	chunks := chunkDigest([]digestGroup{g})
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want >= 2", len(chunks))
	}

	if !strings.Contains(chunks[0].description, "*Guest Features continues in the next message*") {
		t.Fatalf("chunks[0].description = %q, want it to end with the trailing continuation note", chunks[0].description)
	}
	if strings.ContainsAny(chunks[0].description[strings.LastIndex(chunks[0].description, "*Guest Features continues"):], "0123456789") {
		t.Fatalf("chunks[0].description trailing note contains a digit: %q", chunks[0].description)
	}
	if !strings.Contains(chunks[1].description, "**Guest Features (continued)**") {
		t.Fatalf("chunks[1].description = %q, want it to open with the continuation heading", chunks[1].description)
	}
	if strings.Contains(chunks[1].description, "**Guest Features**\n") {
		t.Fatalf("chunks[1].description = %q, must not also carry the plain (non-continuation) heading", chunks[1].description)
	}
}

// TestChunkDigest_OversizedGroupThreeChunksBothMarkersRepeat proves a group
// split across three chunks carries the continuation heading on chunks two
// and three, and the trailing note on chunks one and two (not the last).
func TestChunkDigest_OversizedGroupThreeChunksBothMarkersRepeat(t *testing.T) {
	g := makeSyntheticGroup("Deluxe Changes", 1, 120, 100) // 12000 runes
	chunks := chunkDigest([]digestGroup{g})
	if len(chunks) < 3 {
		t.Fatalf("len(chunks) = %d, want >= 3", len(chunks))
	}

	for i, c := range chunks {
		hasNote := strings.Contains(c.description, "continues in the next message")
		hasContinuedHeading := strings.Contains(c.description, "**Deluxe Changes (continued)**")
		switch {
		case i == 0:
			if !hasNote {
				t.Errorf("chunk 0 description = %q, want the trailing note", c.description)
			}
			if hasContinuedHeading {
				t.Errorf("chunk 0 description = %q, must not carry the continuation heading (it's the group's first chunk)", c.description)
			}
		case i == len(chunks)-1:
			if hasNote {
				t.Errorf("last chunk (%d) description = %q, must not carry a trailing note -- nothing follows it", i, c.description)
			}
			if !hasContinuedHeading {
				t.Errorf("last chunk (%d) description = %q, want the continuation heading", i, c.description)
			}
		default:
			if !hasNote {
				t.Errorf("middle chunk %d description = %q, want the trailing note", i, c.description)
			}
			if !hasContinuedHeading {
				t.Errorf("middle chunk %d description = %q, want the continuation heading", i, c.description)
			}
		}
	}
}

// TestChunkDigest_GroupBoundaryCarriesNoContinuationMarkers proves the
// common case -- a chunk boundary falling between two whole groups -- never
// stamps either continuation marker.
func TestChunkDigest_GroupBoundaryCarriesNoContinuationMarkers(t *testing.T) {
	groups := []digestGroup{
		makeSyntheticGroup("G1", 1, 1, 1800),
		makeSyntheticGroup("G2", 101, 1, 1800),
		makeSyntheticGroup("G3", 201, 1, 1800),
	}
	chunks := chunkDigest(groups)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want > 1 to exercise a real group boundary", len(chunks))
	}
	for i, c := range chunks {
		if strings.Contains(c.description, "(continued)") {
			t.Errorf("chunk %d description = %q, must not carry a continuation heading -- boundary falls between whole groups", i, c.description)
		}
		if strings.Contains(c.description, "continues in the next message") {
			t.Errorf("chunk %d description = %q, must not carry a trailing note -- boundary falls between whole groups", i, c.description)
		}
	}
}

// TestChunkDigest_ContinuationMarkersNeverExceedBudget proves stamping
// both continuation markers never pushes a chunk past
// discordDescriptionLimit (D-20's reserve is what makes this true),
// using a group deliberately sized to land just under chunkContentBudget's
// boundary.
func TestChunkDigest_ContinuationMarkersNeverExceedBudget(t *testing.T) {
	g := makeSyntheticGroup("Deluxe Changes", 1, 50, 100)
	chunks := chunkDigest([]digestGroup{g})
	for i, c := range chunks {
		if rc := utf8.RuneCountInString(c.description); rc > discordDescriptionLimit {
			t.Fatalf("chunk %d has %d runes, want <= %d", i, rc, discordDescriptionLimit)
		}
	}
}

// TestBuildDigestChunks_HeaderOnEveryChunk proves buildDigestChunks stamps
// the window header as the first line of every chunk it returns, including
// chunks after the first (D-04).
func TestBuildDigestChunks_HeaderOnEveryChunk(t *testing.T) {
	events := syntheticEvents(200)
	chunks := buildDigestChunks(events, nil)

	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want > 1 to exercise every-chunk header repetition", len(chunks))
	}
	wantHeader := digestWindowHeader(nil)
	for i, c := range chunks {
		if !strings.HasPrefix(c.description, wantHeader) {
			t.Fatalf("chunk %d description does not start with the window header: %q", i, c.description)
		}
		if rc := utf8.RuneCountInString(c.description); rc > discordDescriptionLimit {
			t.Fatalf("chunk %d has %d runes, want <= %d", i, rc, discordDescriptionLimit)
		}
	}
}

// TestBuildDigestChunks_SingleChunkFitsOneMessage proves an ordinary small
// digest still produces exactly one chunk (the regression-safety path,
// D-14) whose description carries the header plus every rendered line.
func TestBuildDigestChunks_SingleChunkFitsOneMessage(t *testing.T) {
	ts := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist A"), Title: "Album A"},
	}
	chunks := buildDigestChunks(events, &ts)
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	if !strings.HasPrefix(chunks[0].description, digestWindowHeader(&ts)) {
		t.Fatalf("chunks[0].description = %q, want it to start with the window header", chunks[0].description)
	}
	if !strings.Contains(chunks[0].description, "**New Releases**") {
		t.Fatalf("chunks[0].description = %q, want the New Releases heading", chunks[0].description)
	}
	if len(chunks[0].ids) != 1 || chunks[0].ids[0] != 1 {
		t.Fatalf("chunks[0].ids = %v, want [1]", chunks[0].ids)
	}
}

// TestBuildDigestGroups_OrderingIsCollatedCaseAndAccentInsensitive carries
// forward the former buildDigestEmbed ordering coverage onto
// buildDigestGroups (D-17).
func TestBuildDigestGroups_OrderingIsCollatedCaseAndAccentInsensitive(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Zion & Lennox"), Title: "Z"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Ñengo Flow"), Title: "N"},
		{ID: 3, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("bad bunny"), Title: "B"},
	}
	groups := buildDigestGroups(events)
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	ids := make([]int64, len(groups[0].lines))
	for i, l := range groups[0].lines {
		ids[i] = l.id
	}
	want := []int64{3, 2, 1} // bad bunny, Ñengo Flow, Zion & Lennox
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

// TestBuildDigestGroups_TieBreaksByTitleThenID mirrors the former
// buildDigestEmbed tie-break coverage.
func TestBuildDigestGroups_TieBreaksByTitleThenID(t *testing.T) {
	t.Run("same artist, different titles sort by title", func(t *testing.T) {
		events := []sqlc.Event{
			{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Zeta"},
			{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Alpha"},
		}
		groups := buildDigestGroups(events)
		if groups[0].lines[0].id != 1 || groups[0].lines[1].id != 2 {
			t.Fatalf("ids = [%d %d], want [1 2] (Alpha before Zeta)", groups[0].lines[0].id, groups[0].lines[1].id)
		}
	})
	t.Run("same artist and title sort by ascending event id", func(t *testing.T) {
		events := []sqlc.Event{
			{ID: 20, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("Artist"), Title: "Same", ExternalID: "later"},
			{ID: 5, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("Artist"), Title: "Same", ExternalID: "earlier"},
		}
		groups := buildDigestGroups(events)
		if groups[0].lines[0].id != 5 || groups[0].lines[1].id != 20 {
			t.Fatalf("ids = [%d %d], want [5 20]", groups[0].lines[0].id, groups[0].lines[1].id)
		}
	})
}

// TestBuildDigestChunks_ShuffleInvariant proves the rendered output is fully
// determined by the input set, never by input order.
func TestBuildDigestChunks_ShuffleInvariant(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Zion & Lennox"), Title: "Z"},
		{ID: 2, EventType: eventTypeGuestFeature, WatchedArtistName: strPtr("Rauw Alejandro"), ArtistName: "Drake", Title: "Feature"},
		{ID: 3, EventType: eventTypeDeluxeChange, WatchedArtistName: strPtr("bad bunny"), Title: "Deluxe", PreviousTrackCount: i32Ptr(12), TrackCount: i32Ptr(15)},
		{ID: 4, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Ñengo Flow"), Title: "N"},
		{ID: 5, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Alpha"},
	}
	want := buildDigestChunks(events, nil)[0].description

	shuffled := make([]sqlc.Event, len(events))
	copy(shuffled, events)
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	got := buildDigestChunks(shuffled, nil)[0].description
	if got != want {
		t.Fatalf("shuffled input produced a different Description.\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildDigestChunks_GuestFeatureGroupedAndSortedByWatchedArtist(t *testing.T) {
	ev := sqlc.Event{
		EventType:         eventTypeGuestFeature,
		WatchedArtistName: strPtr("Rauw Alejandro"),
		ArtistName:        "Drake",
		Title:             "Track",
	}
	chunks := buildDigestChunks([]sqlc.Event{ev}, nil)
	desc := chunks[0].description
	if !strings.Contains(desc, "**Guest Features**") {
		t.Fatalf("description = %q, want the Guest Features heading", desc)
	}
	if !strings.Contains(desc, "Rauw Alejandro on Drake — Track") {
		t.Fatalf("description = %q, want the label %q", desc, "Rauw Alejandro on Drake — Track")
	}
}

func TestBuildDigestChunks_DeluxeTrackCountSuffix(t *testing.T) {
	t.Run("both counts known renders the suffix after the link", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:          eventTypeDeluxeChange,
			WatchedArtistName:  strPtr("Artist"),
			Title:              "Album",
			ExternalID:         "rel-1",
			PreviousTrackCount: i32Ptr(12),
			TrackCount:         i32Ptr(15),
		}
		desc := buildDigestChunks([]sqlc.Event{ev}, nil)[0].description
		want := "](https://musicbrainz.org/release/rel-1) (12 → 15 tracks)\n"
		if !strings.Contains(desc, want) {
			t.Fatalf("description = %q, want it to contain %q", desc, want)
		}
	})
	t.Run("both counts nil renders no suffix and no empty parens", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:         eventTypeDeluxeChange,
			WatchedArtistName: strPtr("Artist"),
			Title:             "Album",
			ExternalID:        "rel-1",
		}
		desc := buildDigestChunks([]sqlc.Event{ev}, nil)[0].description
		if strings.Contains(desc, "()") {
			t.Fatalf("description = %q, must not contain an empty ()", desc)
		}
		want := "](https://musicbrainz.org/release/rel-1)\n"
		if !strings.Contains(desc, want) {
			t.Fatalf("description = %q, want it to end the line right after the link with no suffix", desc)
		}
	})
}

func TestBuildDigestChunks_WatchedArtistNameFallback(t *testing.T) {
	t.Run("nil falls back to artist_name for label and sort key", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: nil, ArtistName: "Fallback Artist", Title: "T"}
		desc := buildDigestChunks([]sqlc.Event{ev}, nil)[0].description
		if !strings.Contains(desc, "Fallback Artist — T") {
			t.Fatalf("description = %q, want it to contain %q", desc, "Fallback Artist — T")
		}
	})
	t.Run("empty string falls back to artist_name", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: strPtr(""), ArtistName: "Fallback Artist", Title: "T"}
		desc := buildDigestChunks([]sqlc.Event{ev}, nil)[0].description
		if !strings.Contains(desc, "Fallback Artist — T") {
			t.Fatalf("description = %q, want it to contain %q", desc, "Fallback Artist — T")
		}
	})
}

func TestBuildDigestChunks_GroupOrderFixedRegardlessOfInputOrder(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeDeluxeChange, WatchedArtistName: strPtr("A"), Title: "D", ExternalID: "d1"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("A"), Title: "N", ExternalID: "n1"},
		{ID: 3, EventType: eventTypeGuestFeature, WatchedArtistName: strPtr("A"), ArtistName: "Host", Title: "G", ExternalID: "g1"},
	}
	desc := buildDigestChunks(events, nil)[0].description

	iNew := strings.Index(desc, "**New Releases**")
	iGuest := strings.Index(desc, "**Guest Features**")
	iDeluxe := strings.Index(desc, "**Deluxe Changes**")
	if iNew == -1 || iGuest == -1 || iDeluxe == -1 {
		t.Fatalf("description = %q, want all three headings present", desc)
	}
	if iNew >= iGuest || iGuest >= iDeluxe {
		t.Fatalf("description = %q, want headings in New Releases, Guest Features, Deluxe Changes order", desc)
	}
}

func TestBuildDigestChunks_EmptyGroupOmitsHeadingEntirely(t *testing.T) {
	ev := sqlc.Event{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("A"), Title: "T"}
	desc := buildDigestChunks([]sqlc.Event{ev}, nil)[0].description

	for _, heading := range []string{"**Guest Features**", "**Deluxe Changes**"} {
		if strings.Contains(desc, heading) {
			t.Errorf("description = %q, must not contain the empty group's heading %q", desc, heading)
		}
	}
}

func TestBuildDigestChunks_DuplicateSourcesRenderAsTwoLines(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("A"), Title: "Same Release", ExternalID: "mb-1"},
		{ID: 2, EventType: eventTypeNewRelease, Source: sourceDeezer, WatchedArtistName: strPtr("A"), Title: "Same Release", ExternalID: "dz-1"},
	}
	desc := buildDigestChunks(events, nil)[0].description
	if got := strings.Count(desc, "Same Release"); got != 2 {
		t.Fatalf("description = %q, want 2 separate lines for the same release from two sources, got %d", desc, got)
	}
}

func TestBuildDigestChunks_SameArtistTwoEventsRenderAsTwoLines(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album One"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album Two"},
	}
	desc := buildDigestChunks(events, nil)[0].description
	if got := strings.Count(desc, "- ["); got != 2 {
		t.Fatalf("description = %q, want exactly 2 separate lines, got %d", desc, got)
	}
}

// TestPositionIndicator_EmptyAtTotalOne proves D-21's indicator is omitted
// entirely when total is 1, not rendered as "(1/1)".
func TestPositionIndicator_EmptyAtTotalOne(t *testing.T) {
	if got := positionIndicator(1, 1); got != "" {
		t.Fatalf("positionIndicator(1, 1) = %q, want empty string", got)
	}
}

// TestPositionIndicator_NonEmptyAboveTotalOne proves every total greater
// than 1 renders a non-empty parenthesised N-of-Total fragment.
func TestPositionIndicator_NonEmptyAboveTotalOne(t *testing.T) {
	tests := []struct {
		n, total int
		want     string
	}{
		{1, 3, " · (1/3)"},
		{2, 3, " · (2/3)"},
		{3, 3, " · (3/3)"},
		{9, 10, " · (9/10)"},
	}
	for _, tt := range tests {
		if got := positionIndicator(tt.n, tt.total); got != tt.want {
			t.Errorf("positionIndicator(%d, %d) = %q, want %q", tt.n, tt.total, got, tt.want)
		}
	}
}

// TestBuildDigestChunks_OneChunkNoIndicator proves a one-chunk digest's
// header line carries no parenthesis at all.
func TestBuildDigestChunks_OneChunkNoIndicator(t *testing.T) {
	ts := time.Date(2026, 9, 17, 1, 0, 0, 0, time.UTC)
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist A"), Title: "Album A"},
	}
	chunks := buildDigestChunks(events, &ts)
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	firstLine := strings.SplitN(chunks[0].description, "\n", 2)[0]
	want := fmt.Sprintf("Everything pending since <t:%d:R>", ts.Unix())
	if firstLine != want {
		t.Fatalf("first line = %q, want %q (no indicator)", firstLine, want)
	}
	if strings.Contains(firstLine, "(") {
		t.Fatalf("first line = %q, want no parenthesis at all", firstLine)
	}
}

// TestBuildDigestChunks_ThreeChunksIndicatorsMatchPosition proves a
// three-chunk digest's three header lines carry indicators for positions
// 1, 2 and 3, all with total 3, and a nil watermark uses the
// digest-mode-enabled wording plus the same indicator.
func TestBuildDigestChunks_ThreeChunksIndicatorsMatchPosition(t *testing.T) {
	groups := []digestGroup{
		makeSyntheticGroup("G1", 1, 1, 2000),
		makeSyntheticGroup("G2", 101, 1, 2000),
		makeSyntheticGroup("G3", 201, 1, 2000),
	}
	// buildDigestChunks always groups from raw events via buildDigestGroups,
	// so drive chunkDigest directly and stamp headers the same way
	// buildDigestChunks does, to control exactly three chunks. Each group
	// is over half of chunkContentBudget so no two groups can share a
	// chunk, forcing one group per chunk.
	chunks := chunkDigest(groups)
	if len(chunks) != 3 {
		t.Fatalf("test fixture produced %d chunks, want exactly 3 (adjust makeSyntheticGroup sizes)", len(chunks))
	}
	base := digestWindowHeader(nil)
	total := len(chunks)
	for i := range chunks {
		chunks[i].description = base + positionIndicator(i+1, total) + "\n" + chunks[i].description
	}

	for i, c := range chunks {
		firstLine := strings.SplitN(c.description, "\n", 2)[0]
		want := fmt.Sprintf("%s · (%d/3)", base, i+1)
		if firstLine != want {
			t.Fatalf("chunk %d first line = %q, want %q", i, firstLine, want)
		}
	}
}

// syntheticInvariantBatch builds n events cycling through all three event
// types with a deterministic, reproducible index-driven generator (no
// randomness), each with a distinct id, so a failure reproduces exactly.
// Titles and artist names mix multi-byte (Á, ñ, 水) and emoji (🎵)
// characters so the invariants' rune-count assertions measure what they
// claim to measure rather than ASCII only.
func syntheticInvariantBatch(n int) []sqlc.Event {
	types := []string{eventTypeNewRelease, eventTypeGuestFeature, eventTypeDeluxeChange}
	events := make([]sqlc.Event, n)
	for i := 0; i < n; i++ {
		et := types[i%len(types)]
		ev := sqlc.Event{
			ID:                int64(i + 1),
			EventType:         et,
			Source:            sourceMusicBrainz,
			ExternalID:        fmt.Sprintf("ext-%05d", i),
			WatchedArtistName: strPtr(fmt.Sprintf("Ártist Ñame 水 %04d", i)),
			Title:             fmt.Sprintf("Álbum 水 Title Number %04d 🎵 With Extra Padding Text To Grow The Line", i),
		}
		if et == eventTypeGuestFeature {
			ev.ArtistName = fmt.Sprintf("Host 水 %04d", i)
		} else {
			ev.ArtistName = fmt.Sprintf("Fallback Ártist %04d", i)
		}
		if et == eventTypeDeluxeChange {
			ev.PreviousTrackCount = i32Ptr(int32(i % 20))
			ev.TrackCount = i32Ptr(int32(i%20 + 3))
		}
		events[i] = ev
	}
	return events
}

// invariantBatchSize is large enough to force chunkDigest's oversized-group
// line-split fallback across all three groups and produce at least 20
// chunks (verified by TestChunkDigest_InvariantBatchProducesAtLeast20Chunks
// below) -- also reusable by plan 23-04's cap tests per the plan's own note.
const invariantBatchSize = 700

// TestChunkDigest_InvariantBatchProducesAtLeast20Chunks pins the fixture
// size's own precondition -- if this ever fails, invariantBatchSize needs
// raising, not the invariant tests below relaxing.
func TestChunkDigest_InvariantBatchProducesAtLeast20Chunks(t *testing.T) {
	chunks := buildDigestChunks(syntheticInvariantBatch(invariantBatchSize), nil)
	if len(chunks) < 20 {
		t.Fatalf("len(chunks) = %d, want >= 20 -- raise invariantBatchSize", len(chunks))
	}
}

// TestChunkInvariant1_EveryChunkAtMostDiscordLimit is Verification
// Invariant 1 (23-CONTEXT.md <specifics>): over a synthetic batch large
// enough to produce at least 20 chunks, every chunk's description is at
// most 4096 runes.
func TestChunkInvariant1_EveryChunkAtMostDiscordLimit(t *testing.T) {
	chunks := buildDigestChunks(syntheticInvariantBatch(invariantBatchSize), nil)
	for i, c := range chunks {
		if rc := utf8.RuneCountInString(c.description); rc > discordDescriptionLimit {
			t.Errorf("chunk %d has %d runes, want <= %d", i, rc, discordDescriptionLimit)
		}
	}
}

// TestChunkInvariant2_IDUnionExactlyOnce is Verification Invariant 2: the
// union of every chunk's ids equals the sendable set, each id exactly once.
func TestChunkInvariant2_IDUnionExactlyOnce(t *testing.T) {
	events := syntheticInvariantBatch(invariantBatchSize)
	chunks := buildDigestChunks(events, nil)

	seen := make(map[int64]int, len(events))
	for _, c := range chunks {
		for _, id := range c.ids {
			seen[id]++
		}
	}
	if len(seen) != len(events) {
		t.Fatalf("union has %d distinct ids, want %d", len(seen), len(events))
	}
	for _, ev := range events {
		if seen[ev.ID] != 1 {
			t.Errorf("event id %d appears %d times across chunks, want exactly 1", ev.ID, seen[ev.ID])
		}
	}
}

// stripChunkMarkers removes every chunk's header line, group heading
// (plain or "(continued)"), and trailing continuation note from a chunk
// description, so Invariant 3 can compare event lines to event lines only.
// Test-only -- not a production helper.
func stripChunkMarkers(desc string) string {
	lines := strings.Split(desc, "\n")
	var kept []string
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "Everything pending since"):
			continue
		case strings.HasPrefix(line, "**") && strings.HasSuffix(line, "**"):
			// A group heading, plain or "(continued)" -- both open and
			// close with the bold markers digestHeadings/continuationHeading
			// use, and no rendered event line does.
			continue
		case strings.HasPrefix(line, "*") && strings.HasSuffix(line, "continues in the next message*"):
			continue
		case line == "":
			continue
		default:
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return ""
	}
	return strings.Join(kept, "\n") + "\n"
}

// TestChunkInvariant3_ConcatenationReproducesUnsplitOrder is Verification
// Invariant 3: stripping every header line, continuation heading, and
// trailing note from the concatenated chunk descriptions reproduces the
// ordered line sequence a single unsplit render of the same groups
// produces, exactly.
func TestChunkInvariant3_ConcatenationReproducesUnsplitOrder(t *testing.T) {
	events := syntheticInvariantBatch(invariantBatchSize)
	groups := buildDigestGroups(events)

	var wantLines []string
	for _, g := range groups {
		for _, line := range g.lines {
			wantLines = append(wantLines, strings.TrimSuffix(line.text, "\n"))
		}
	}
	want := strings.Join(wantLines, "\n") + "\n"

	chunks := buildDigestChunks(events, nil)
	var got strings.Builder
	for _, c := range chunks {
		got.WriteString(stripChunkMarkers(c.description))
	}
	if got.String() != want {
		t.Fatalf("stripped concatenation does not match the unsplit render.\nwant %d runes, got %d runes", utf8.RuneCountInString(want), utf8.RuneCountInString(got.String()))
	}
}

// TestBuildDigestChunks_LargeBatchShuffleInvariant is the plan's seventh
// behavior bullet: the same event set fed in a different arrival order
// produces byte-identical chunk descriptions and identical per-chunk id
// sets, over the same large synthetic batch the three invariants above use.
func TestBuildDigestChunks_LargeBatchShuffleInvariant(t *testing.T) {
	events := syntheticInvariantBatch(invariantBatchSize)
	want := buildDigestChunks(events, nil)

	shuffled := make([]sqlc.Event, len(events))
	copy(shuffled, events)
	rng := rand.New(rand.NewSource(7))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	got := buildDigestChunks(shuffled, nil)
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, len(want) = %d", len(got), len(want))
	}
	for i := range want {
		if got[i].description != want[i].description {
			t.Fatalf("chunk %d description differs after shuffling input order", i)
		}
		if len(got[i].ids) != len(want[i].ids) {
			t.Fatalf("chunk %d ids length differs after shuffling input order: got %d, want %d", i, len(got[i].ids), len(want[i].ids))
		}
		for j := range want[i].ids {
			if got[i].ids[j] != want[i].ids[j] {
				t.Fatalf("chunk %d ids[%d] differs after shuffling input order: got %d, want %d", i, j, got[i].ids[j], want[i].ids[j])
			}
		}
	}
}
