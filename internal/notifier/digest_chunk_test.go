package notifier

// This file is package notifier (whitebox), matching digest_format_test.go's
// convention -- digestWindowHeader, chunkDigest, buildDigestGroups, and
// buildDigestChunks are all unexported. Pure table-driven/property-style
// unit tests, no DB, no HTTP.

import (
	"fmt"
	"math/rand"
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
// description is at most chunkContentBudget runes, and no chunk ends
// mid-line.
func TestChunkDigest_OverBudgetMultipleChunks(t *testing.T) {
	groups := buildDigestGroups(syntheticEvents(200))
	chunks := chunkDigest(groups)

	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want > 1", len(chunks))
	}
	for i, c := range chunks {
		if rc := utf8.RuneCountInString(c.description); rc > chunkContentBudget {
			t.Fatalf("chunk %d has %d runes, want <= %d", i, rc, chunkContentBudget)
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
