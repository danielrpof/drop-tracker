package notifier

// This file is package notifier (whitebox), not notifier_test -- escapeMarkdown,
// eventURL, and buildDigestEmbed are unexported, mirroring format_test.go's,
// musicbrainz_test.go's, and normalize_test.go's whitebox convention for
// testing unexported functions directly. Pure table-driven unit tests, no
// DB, no HTTP.

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

func TestEscapeMarkdown(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"ordinary name unchanged", "Bad Bunny", "Bad Bunny"},
		{"square brackets", "Album [Deluxe]", `Album \[Deluxe\]`},
		{"round brackets", "Album (Remix)", `Album \(Remix\)`},
		{"asterisk", "a*b", `a\*b`},
		{"underscore", "a_b", `a\_b`},
		{"single backslash doubled, applied first", `a\b`, `a\\b`},
		{"backslash then metacharacter, not double-escaped", `\*`, `\\\*`},
		{"tilde", "a~b", `a\~b`},
		{"backtick", "a`b", "a\\`b"},
		{"pipe", "a|b", `a\|b`},
		{"greater-than", "a>b", `a\>b`},
		{"hash", "a#b", `a\#b`},
		{"empty string", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeMarkdown(tt.in); got != tt.want {
				t.Errorf("escapeMarkdown(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestEscapeMarkdown_AllElevenMetacharactersInOnePass proves the eleven-
// character set is escaped together, not just individually -- and that the
// backslash-first rule doesn't leave any escape re-escaped.
func TestEscapeMarkdown_AllElevenMetacharactersInOnePass(t *testing.T) {
	in := "\\*_~`|>#[]()"
	got := escapeMarkdown(in)
	want := "\\\\\\*\\_\\~\\`\\|\\>\\#\\[\\]\\(\\)"
	if got != want {
		t.Fatalf("escapeMarkdown(%q) = %q, want %q", in, got, want)
	}
}

func TestDigestTitleLimit_CapsBeforeEscaping(t *testing.T) {
	// 150-rune title capped to 100 runes before escaping is applied.
	title := strings.Repeat("a", 150)
	capped := truncateRunes(title, digestTitleLimit)
	if got := utf8.RuneCountInString(capped); got != digestTitleLimit {
		t.Fatalf("rune count of capped title = %d, want %d", got, digestTitleLimit)
	}

	// A 100-rune title made entirely of multi-byte characters survives
	// uncapped -- the cap counts runes, not bytes.
	multiByte := strings.Repeat("水", digestTitleLimit)
	if got := utf8.RuneCountInString(multiByte); got != digestTitleLimit {
		t.Fatalf("test fixture has %d runes, want exactly %d", got, digestTitleLimit)
	}
	got := truncateRunes(multiByte, digestTitleLimit)
	if got != multiByte {
		t.Fatalf("truncateRunes(100-rune multi-byte title, %d) truncated it, want unchanged", digestTitleLimit)
	}
	if !utf8.ValidString(got) {
		t.Fatal("result is not valid UTF-8")
	}
}

func TestEventURL(t *testing.T) {
	tests := []struct {
		name string
		ev   sqlc.Event
		want string
	}{
		{
			name: "new_release musicbrainz",
			ev:   sqlc.Event{EventType: eventTypeNewRelease, Source: sourceMusicBrainz, ExternalID: "rg-1"},
			want: "https://musicbrainz.org/release-group/rg-1",
		},
		{
			name: "new_release deezer",
			ev:   sqlc.Event{EventType: eventTypeNewRelease, Source: sourceDeezer, ExternalID: "999"},
			want: "https://www.deezer.com/album/999",
		},
		{
			name: "guest_feature",
			ev:   sqlc.Event{EventType: eventTypeGuestFeature, ExternalID: "rec-1"},
			want: "https://musicbrainz.org/recording/rec-1",
		},
		{
			name: "deluxe_change",
			ev:   sqlc.Event{EventType: eventTypeDeluxeChange, ExternalID: "rel-1"},
			want: "https://musicbrainz.org/release/rel-1",
		},
		{
			name: "unrecognised event type returns empty string",
			ev:   sqlc.Event{EventType: "something_else", ExternalID: "x"},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventURL(tt.ev); got != tt.want {
				t.Errorf("eventURL(%+v) = %q, want %q", tt.ev, got, tt.want)
			}
		})
	}
}

// TestDigestLine_TitleWithBracketsCannotRetargetLink is the assertion T-22-07
// exists to satisfy: a title carrying markdown metacharacters is escaped
// inline, and the URL between the line's `](` and its closing `)` is
// exactly the expected link -- proving the title text cannot terminate the
// masked link's label or retarget its href.
func TestDigestLine_TitleWithBracketsCannotRetargetLink(t *testing.T) {
	ev := sqlc.Event{
		EventType:  eventTypeNewRelease,
		Source:     sourceMusicBrainz,
		ExternalID: "rg-1",
		Title:      "Album [Deluxe]",
		ArtistName: "Bad Bunny",
	}
	embed := buildDigestEmbed([]sqlc.Event{ev})

	if !strings.Contains(embed.Description, `Album \[Deluxe\]`) {
		t.Fatalf("Description = %q, want it to contain the escaped title %q", embed.Description, `Album \[Deluxe\]`)
	}

	wantURL := "https://musicbrainz.org/release-group/rg-1"
	openIdx := strings.Index(embed.Description, "](")
	if openIdx == -1 {
		t.Fatalf("Description = %q, want it to contain a markdown link", embed.Description)
	}
	rest := embed.Description[openIdx+2:]
	closeIdx := strings.Index(rest, ")")
	if closeIdx == -1 {
		t.Fatalf("Description = %q, link has no closing paren", embed.Description)
	}
	gotURL := rest[:closeIdx]
	if gotURL != wantURL {
		t.Fatalf("link URL = %q, want %q -- title text must never retarget the link", gotURL, wantURL)
	}
}

func TestArtistKey(t *testing.T) {
	tests := []struct {
		name string
		ev   sqlc.Event
		want string
	}{
		{"non-nil non-empty WatchedArtistName wins", sqlc.Event{WatchedArtistName: strPtr("Rauw Alejandro"), ArtistName: "Drake"}, "Rauw Alejandro"},
		{"nil WatchedArtistName falls back to ArtistName", sqlc.Event{WatchedArtistName: nil, ArtistName: "Drake"}, "Drake"},
		{"empty-string WatchedArtistName falls back to ArtistName", sqlc.Event{WatchedArtistName: strPtr(""), ArtistName: "Drake"}, "Drake"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := artistKey(tt.ev); got != tt.want {
				t.Errorf("artistKey(%+v) = %q, want %q", tt.ev, got, tt.want)
			}
		})
	}
}

func TestLineLabel(t *testing.T) {
	t.Run("new_release label is Artist — Title", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album"}
		want := "Bad Bunny — Album"
		if got := lineLabel(ev); got != want {
			t.Errorf("lineLabel(%+v) = %q, want %q", ev, got, want)
		}
	})
	t.Run("guest_feature label names watched artist and host credit", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:         eventTypeGuestFeature,
			WatchedArtistName: strPtr("Rauw Alejandro"),
			ArtistName:        "Drake",
			Title:             "Track",
		}
		want := "Rauw Alejandro on Drake — Track"
		if got := lineLabel(ev); got != want {
			t.Errorf("lineLabel(%+v) = %q, want %q", ev, got, want)
		}
	})
	t.Run("guest_feature host credit is escaped too", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:         eventTypeGuestFeature,
			WatchedArtistName: strPtr("Rauw Alejandro"),
			ArtistName:        "Drake [Remix]",
			Title:             "Track",
		}
		if got := lineLabel(ev); !strings.Contains(got, `Drake \[Remix\]`) {
			t.Errorf("lineLabel(%+v) = %q, want the host credit escaped", ev, got)
		}
	})
	t.Run("deluxe_change label is Artist — Title, no host credit", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeDeluxeChange, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album (Deluxe)"}
		want := `Bad Bunny — Album \(Deluxe\)`
		if got := lineLabel(ev); got != want {
			t.Errorf("lineLabel(%+v) = %q, want %q", ev, got, want)
		}
	})
}

func TestBuildDigestEmbed_OrderingIsCollatedCaseAndAccentInsensitive(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Zion & Lennox"), Title: "Z"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Ñengo Flow"), Title: "N"},
		{ID: 3, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("bad bunny"), Title: "B"},
	}
	embed := buildDigestEmbed(events)

	iBad := strings.Index(embed.Description, "bad bunny")
	iNengo := strings.Index(embed.Description, "Ñengo Flow")
	iZion := strings.Index(embed.Description, "Zion & Lennox")
	if iBad == -1 || iNengo == -1 || iZion == -1 {
		t.Fatalf("Description = %q, want all three artists present", embed.Description)
	}
	if iBad >= iNengo || iNengo >= iZion {
		t.Fatalf("Description order = %q, want bad bunny, Ñengo Flow, Zion & Lennox in that order", embed.Description)
	}
}

func TestBuildDigestEmbed_TieBreaksByTitleThenID(t *testing.T) {
	t.Run("same artist, different titles sort by title", func(t *testing.T) {
		events := []sqlc.Event{
			{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Zeta"},
			{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Alpha"},
		}
		embed := buildDigestEmbed(events)
		iAlpha := strings.Index(embed.Description, "Alpha")
		iZeta := strings.Index(embed.Description, "Zeta")
		if iAlpha == -1 || iZeta == -1 || iAlpha > iZeta {
			t.Fatalf("Description = %q, want Alpha before Zeta", embed.Description)
		}
	})
	t.Run("same artist and title sort by ascending event id", func(t *testing.T) {
		events := []sqlc.Event{
			{ID: 20, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("Artist"), Title: "Same", ExternalID: "later"},
			{ID: 5, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("Artist"), Title: "Same", ExternalID: "earlier"},
		}
		embed := buildDigestEmbed(events)
		iEarlier := strings.Index(embed.Description, "https://musicbrainz.org/release-group/earlier")
		iLater := strings.Index(embed.Description, "https://musicbrainz.org/release-group/later")
		if iEarlier == -1 || iLater == -1 || iEarlier > iLater {
			t.Fatalf("Description = %q, want the id=5 line before the id=20 line", embed.Description)
		}
	})
}

func TestBuildDigestEmbed_ShuffleInvariant(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Zion & Lennox"), Title: "Z"},
		{ID: 2, EventType: eventTypeGuestFeature, WatchedArtistName: strPtr("Rauw Alejandro"), ArtistName: "Drake", Title: "Feature"},
		{ID: 3, EventType: eventTypeDeluxeChange, WatchedArtistName: strPtr("bad bunny"), Title: "Deluxe", PreviousTrackCount: i32Ptr(12), TrackCount: i32Ptr(15)},
		{ID: 4, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Ñengo Flow"), Title: "N"},
		{ID: 5, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Artist"), Title: "Alpha"},
	}
	want := buildDigestEmbed(events).Description

	shuffled := make([]sqlc.Event, len(events))
	copy(shuffled, events)
	rng := rand.New(rand.NewSource(42))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	got := buildDigestEmbed(shuffled).Description
	if got != want {
		t.Fatalf("shuffled input produced a different Description.\nwant: %q\ngot:  %q", want, got)
	}
}

func TestBuildDigestEmbed_GuestFeatureGroupedAndSortedByWatchedArtist(t *testing.T) {
	ev := sqlc.Event{
		EventType:         eventTypeGuestFeature,
		WatchedArtistName: strPtr("Rauw Alejandro"),
		ArtistName:        "Drake",
		Title:             "Track",
	}
	embed := buildDigestEmbed([]sqlc.Event{ev})
	if !strings.Contains(embed.Description, "**Guest Features**") {
		t.Fatalf("Description = %q, want the Guest Features heading", embed.Description)
	}
	if !strings.Contains(embed.Description, "Rauw Alejandro on Drake — Track") {
		t.Fatalf("Description = %q, want the label %q", embed.Description, "Rauw Alejandro on Drake — Track")
	}
}

func TestBuildDigestEmbed_DeluxeTrackCountSuffix(t *testing.T) {
	t.Run("both counts known renders the suffix after the link", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:          eventTypeDeluxeChange,
			WatchedArtistName:  strPtr("Artist"),
			Title:              "Album",
			ExternalID:         "rel-1",
			PreviousTrackCount: i32Ptr(12),
			TrackCount:         i32Ptr(15),
		}
		embed := buildDigestEmbed([]sqlc.Event{ev})
		want := "](https://musicbrainz.org/release/rel-1) (12 → 15 tracks)\n"
		if !strings.Contains(embed.Description, want) {
			t.Fatalf("Description = %q, want it to contain %q", embed.Description, want)
		}
	})
	t.Run("both counts nil renders no suffix and no empty parens", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:         eventTypeDeluxeChange,
			WatchedArtistName: strPtr("Artist"),
			Title:             "Album",
			ExternalID:        "rel-1",
		}
		embed := buildDigestEmbed([]sqlc.Event{ev})
		line := strings.TrimSuffix(strings.TrimSpace(embed.Description), "")
		if strings.Contains(line, "()") {
			t.Fatalf("Description = %q, must not contain an empty ()", embed.Description)
		}
		want := "](https://musicbrainz.org/release/rel-1)\n"
		if !strings.Contains(embed.Description, want) {
			t.Fatalf("Description = %q, want it to end the line right after the link with no suffix", embed.Description)
		}
	})
}

func TestBuildDigestEmbed_WatchedArtistNameFallback(t *testing.T) {
	t.Run("nil falls back to artist_name for label and sort key", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: nil, ArtistName: "Fallback Artist", Title: "T"}
		embed := buildDigestEmbed([]sqlc.Event{ev})
		if !strings.Contains(embed.Description, "Fallback Artist — T") {
			t.Fatalf("Description = %q, want it to contain %q", embed.Description, "Fallback Artist — T")
		}
	})
	t.Run("empty string falls back to artist_name", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: strPtr(""), ArtistName: "Fallback Artist", Title: "T"}
		embed := buildDigestEmbed([]sqlc.Event{ev})
		if !strings.Contains(embed.Description, "Fallback Artist — T") {
			t.Fatalf("Description = %q, want it to contain %q", embed.Description, "Fallback Artist — T")
		}
	})
}

func TestBuildDigestEmbed_GroupOrderFixedRegardlessOfInputOrder(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeDeluxeChange, WatchedArtistName: strPtr("A"), Title: "D", ExternalID: "d1"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("A"), Title: "N", ExternalID: "n1"},
		{ID: 3, EventType: eventTypeGuestFeature, WatchedArtistName: strPtr("A"), ArtistName: "Host", Title: "G", ExternalID: "g1"},
	}
	embed := buildDigestEmbed(events)

	iNew := strings.Index(embed.Description, "**New Releases**")
	iGuest := strings.Index(embed.Description, "**Guest Features**")
	iDeluxe := strings.Index(embed.Description, "**Deluxe Changes**")
	if iNew == -1 || iGuest == -1 || iDeluxe == -1 {
		t.Fatalf("Description = %q, want all three headings present", embed.Description)
	}
	if iNew >= iGuest || iGuest >= iDeluxe {
		t.Fatalf("Description = %q, want headings in New Releases, Guest Features, Deluxe Changes order", embed.Description)
	}
}

func TestBuildDigestEmbed_EmptyGroupOmitsHeadingEntirely(t *testing.T) {
	ev := sqlc.Event{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("A"), Title: "T"}
	embed := buildDigestEmbed([]sqlc.Event{ev})

	for _, heading := range []string{"**Guest Features**", "**Deluxe Changes**"} {
		if strings.Contains(embed.Description, heading) {
			t.Errorf("Description = %q, must not contain the empty group's heading %q", embed.Description, heading)
		}
	}
}

func TestBuildDigestEmbed_DuplicateSourcesRenderAsTwoLines(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, WatchedArtistName: strPtr("A"), Title: "Same Release", ExternalID: "mb-1"},
		{ID: 2, EventType: eventTypeNewRelease, Source: sourceDeezer, WatchedArtistName: strPtr("A"), Title: "Same Release", ExternalID: "dz-1"},
	}
	embed := buildDigestEmbed(events)
	if got := strings.Count(embed.Description, "Same Release"); got != 2 {
		t.Fatalf("Description = %q, want 2 separate lines for the same release from two sources, got %d", embed.Description, got)
	}
}

func TestBuildDigestEmbed_SameArtistTwoEventsRenderAsTwoLines(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album One"},
		{ID: 2, EventType: eventTypeNewRelease, WatchedArtistName: strPtr("Bad Bunny"), Title: "Album Two"},
	}
	embed := buildDigestEmbed(events)
	if got := strings.Count(embed.Description, "- ["); got != 2 {
		t.Fatalf("Description = %q, want exactly 2 separate lines, got %d", embed.Description, got)
	}
}

// TestDigestLine_RendersLabelURLAndDeluxeSuffix pins digestLine's output
// byte-for-byte -- it is the extracted-verbatim body buildDigestEmbed's
// former inner loop wrote directly, and assembleDescription's whole-segment
// truncation depends on it never changing shape.
func TestDigestLine_RendersLabelURLAndDeluxeSuffix(t *testing.T) {
	t.Run("new_release renders label and url with no suffix", func(t *testing.T) {
		ev := sqlc.Event{EventType: eventTypeNewRelease, Source: sourceMusicBrainz, ExternalID: "rg-1", WatchedArtistName: strPtr("Bad Bunny"), Title: "Album"}
		want := "- [Bad Bunny — Album](https://musicbrainz.org/release-group/rg-1)\n"
		if got := digestLine(eventTypeNewRelease, ev); got != want {
			t.Fatalf("digestLine(...) = %q, want %q", got, want)
		}
	})
	t.Run("deluxe_change appends track-count suffix", func(t *testing.T) {
		ev := sqlc.Event{
			EventType:          eventTypeDeluxeChange,
			ExternalID:         "rel-1",
			WatchedArtistName:  strPtr("Artist"),
			Title:              "Album",
			PreviousTrackCount: i32Ptr(12),
			TrackCount:         i32Ptr(15),
		}
		want := "- [Artist — Album](https://musicbrainz.org/release/rel-1) (12 → 15 tracks)\n"
		if got := digestLine(eventTypeDeluxeChange, ev); got != want {
			t.Fatalf("digestLine(...) = %q, want %q", got, want)
		}
	})
}

// TestAssembleDescription_FitsUnderLimitReturnsJoinUnchanged pins the early
// return that keeps every pre-existing buildDigestEmbed assertion above
// byte-identical: a combined rune count at or under the limit produces a
// plain join, no note.
func TestAssembleDescription_FitsUnderLimitReturnsJoinUnchanged(t *testing.T) {
	segments := []string{"a\n", "b\n", "c\n"}
	want := strings.Join(segments, "")
	got := assembleDescription(segments)
	if got != want {
		t.Fatalf("assembleDescription(...) = %q, want byte-identical join %q", got, want)
	}
	if strings.Contains(got, "more event") {
		t.Fatalf("assembleDescription(...) = %q, must not contain a truncation note when under budget", got)
	}
}

// TestAssembleDescription_ExceedsLimitTruncatesOnSegmentBoundary proves the
// three properties the design contract requires once the combined rune
// count exceeds discordDescriptionLimit: the result stays within the limit,
// only whole segments survive (never a partial line), and the note's
// reported omitted count reconciles exactly with how many segments were
// dropped.
func TestAssembleDescription_ExceedsLimitTruncatesOnSegmentBoundary(t *testing.T) {
	// Each segment is exactly 100 runes (99 'x' plus a newline); 50 of them
	// is 5000 runes, comfortably over the 4096 limit.
	segment := strings.Repeat("x", 99) + "\n"
	segments := make([]string, 50)
	for i := range segments {
		segments[i] = segment
	}

	got := assembleDescription(segments)

	if rc := utf8.RuneCountInString(got); rc > discordDescriptionLimit {
		t.Fatalf("assembleDescription(...) has %d runes, want <= %d", rc, discordDescriptionLimit)
	}
	if !strings.Contains(got, "more event") {
		t.Fatalf("assembleDescription(...) = %q, want a truncation note", got)
	}

	kept := strings.Count(got, "x")
	if kept%99 != 0 {
		t.Fatalf("assembleDescription(...) cut mid-segment: %d 'x' characters is not a multiple of 99", kept)
	}
	keptSegments := kept / 99
	omitted := len(segments) - keptSegments
	wantNote := truncationNote(omitted)
	if !strings.HasSuffix(got, wantNote) {
		t.Fatalf("assembleDescription(...) = %q, want it to end with %q (kept=%d, omitted=%d)", got, wantNote, keptSegments, omitted)
	}
}

func TestTruncationNote_SingularAndPlural(t *testing.T) {
	if got := truncationNote(1); got != "... 1 more event" {
		t.Fatalf("truncationNote(1) = %q, want singular %q", got, "... 1 more event")
	}
	if got := truncationNote(2); got != "... 2 more events" {
		t.Fatalf("truncationNote(2) = %q, want plural %q", got, "... 2 more events")
	}
}

// TestBuildDigestEmbed_SmallOrdinaryBatchNoTruncationNote pins the
// "unaffected when under budget" contract explicitly at buildDigestEmbed's
// level, on top of what the pre-existing suite above already implies.
func TestBuildDigestEmbed_SmallOrdinaryBatchNoTruncationNote(t *testing.T) {
	events := []sqlc.Event{
		{ID: 1, EventType: eventTypeNewRelease, Source: sourceMusicBrainz, ExternalID: "nr-1", WatchedArtistName: strPtr("Artist A"), Title: "Album A"},
		{ID: 2, EventType: eventTypeGuestFeature, ExternalID: "gf-1", WatchedArtistName: strPtr("Artist B"), ArtistName: "Host", Title: "Track B"},
		{ID: 3, EventType: eventTypeDeluxeChange, ExternalID: "dc-1", WatchedArtistName: strPtr("Artist C"), Title: "Album C", PreviousTrackCount: i32Ptr(10), TrackCount: i32Ptr(12)},
	}
	embed := buildDigestEmbed(events)
	if strings.Contains(embed.Description, "more event") {
		t.Fatalf("Description = %q, must not contain a truncation note for a small batch well under the limit", embed.Description)
	}
	if rc := utf8.RuneCountInString(embed.Description); rc > discordDescriptionLimit {
		t.Fatalf("Description has %d runes, want <= %d", rc, discordDescriptionLimit)
	}
}

// TestBuildDigestEmbed_OversizedBatchTruncatesAndReconciles is the
// synthetic-oversized-batch case from the design contract's behavior block:
// enough loop-generated events that the concatenated rendered lines exceed
// 4096 runes, proving the Description never exceeds the limit, contains the
// note, and that rendered-line count plus the note's parsed-back omitted
// count equals len(events).
func TestBuildDigestEmbed_OversizedBatchTruncatesAndReconciles(t *testing.T) {
	const n = 200
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
	embed := buildDigestEmbed(events)

	if rc := utf8.RuneCountInString(embed.Description); rc > discordDescriptionLimit {
		t.Fatalf("Description has %d runes, want <= %d", rc, discordDescriptionLimit)
	}
	if !strings.Contains(embed.Description, "more event") {
		t.Fatalf("Description = %q, want a truncation note -- fixture did not exceed the limit", embed.Description)
	}

	rendered := strings.Count(embed.Description, "- [")
	if rendered == 0 || rendered == n {
		t.Fatalf("rendered line count = %d, want strictly between 0 and %d to prove real truncation occurred", rendered, n)
	}
	omitted := n - rendered
	wantNote := truncationNote(omitted)
	if !strings.HasSuffix(embed.Description, wantNote) {
		t.Fatalf("Description = %q, want it to end with %q (rendered=%d, omitted=%d)", embed.Description, wantNote, rendered, omitted)
	}
}
