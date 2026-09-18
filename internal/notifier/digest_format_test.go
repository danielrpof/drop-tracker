package notifier

// This file is package notifier (whitebox), not notifier_test -- escapeMarkdown,
// eventURL, digestLine, and lineLabel are unexported, mirroring format_test.go's,
// musicbrainz_test.go's, and normalize_test.go's whitebox convention for
// testing unexported functions directly. Pure table-driven unit tests, no
// DB, no HTTP. Grouping/chunking/header tests live in digest_chunk_test.go
// (D-17) -- this file covers only line/label/sort rendering.

import (
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
	line := digestLine(eventTypeNewRelease, ev)

	if !strings.Contains(line, `Album \[Deluxe\]`) {
		t.Fatalf("line = %q, want it to contain the escaped title %q", line, `Album \[Deluxe\]`)
	}

	wantURL := "https://musicbrainz.org/release-group/rg-1"
	openIdx := strings.Index(line, "](")
	if openIdx == -1 {
		t.Fatalf("line = %q, want it to contain a markdown link", line)
	}
	rest := line[openIdx+2:]
	closeIdx := strings.Index(rest, ")")
	if closeIdx == -1 {
		t.Fatalf("line = %q, link has no closing paren", line)
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
	t.Run("watched artist over digestArtistLimit runes is capped on a rune boundary before escaping", func(t *testing.T) {
		long := strings.Repeat("水", digestArtistLimit+10)
		ev := sqlc.Event{EventType: eventTypeNewRelease, WatchedArtistName: strPtr(long), Title: "Album"}
		got := lineLabel(ev)
		wantWatched := strings.Repeat("水", digestArtistLimit)
		if !strings.HasPrefix(got, wantWatched+" — ") {
			t.Fatalf("lineLabel(%+v) = %q, want it to start with the %d-rune-capped watched artist", ev, got, digestArtistLimit)
		}
		if !utf8.ValidString(got) {
			t.Fatal("result is not valid UTF-8")
		}
	})
	t.Run("guest_feature host credit over digestArtistLimit runes is capped on a rune boundary before escaping", func(t *testing.T) {
		long := strings.Repeat("a", digestArtistLimit+10)
		ev := sqlc.Event{
			EventType:         eventTypeGuestFeature,
			WatchedArtistName: strPtr("Watched"),
			ArtistName:        long,
			Title:             "Track",
		}
		got := lineLabel(ev)
		wantHost := strings.Repeat("a", digestArtistLimit)
		want := "Watched on " + wantHost + " — Track"
		if got != want {
			t.Fatalf("lineLabel(%+v) = %q, want %q", ev, got, want)
		}
	})
}

// TestDigestLine_RendersLabelURLAndDeluxeSuffix pins digestLine's output
// byte-for-byte -- chunkDigest (digest_chunk.go) depends on it never
// changing shape, since a chunk boundary can only fall between two whole
// digestLine outputs.
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
