package notifier

// This file is package notifier (whitebox), not notifier_test -- formatEmbed
// is unexported, mirroring internal/discord/client_test.go's and
// internal/detection/musicbrainz_test.go's whitebox convention for testing
// an unexported function directly. Pure table-driven unit tests, no DB, no
// HTTP -- every case operates on a hand-built sqlc.Event value.

import (
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)

func strPtr(s string) *string { return &s }
func i32Ptr(n int32) *int32   { return &n }

func TestFormatEmbed_ThreeEventTypesAreDistinct(t *testing.T) {
	newRelease := formatEmbed(sqlc.Event{
		Source: sourceMusicBrainz, EventType: eventTypeNewRelease,
		ExternalID: "rg-1", Title: "Album", ArtistName: "Artist",
	})
	guestFeature := formatEmbed(sqlc.Event{
		Source: sourceMusicBrainz, EventType: eventTypeGuestFeature,
		ExternalID: "rec-1", Title: "Track", ArtistName: "Primary Artist",
	})
	deluxeChange := formatEmbed(sqlc.Event{
		Source: sourceMusicBrainz, EventType: eventTypeDeluxeChange,
		ExternalID: "rel-1", Title: "Album (Deluxe)", ArtistName: "Artist",
		TrackCount: i32Ptr(18), PreviousTrackCount: i32Ptr(12),
	})

	colors := map[int]string{
		newRelease.Color:   "new_release",
		guestFeature.Color: "guest_feature",
		deluxeChange.Color: "deluxe_change",
	}
	if len(colors) != 3 {
		t.Fatalf("distinct colors = %d, want 3 (new_release=%d guest_feature=%d deluxe_change=%d)",
			len(colors), newRelease.Color, guestFeature.Color, deluxeChange.Color)
	}

	prefixes := map[string]string{
		emojiPrefix(newRelease.Title):   "new_release",
		emojiPrefix(guestFeature.Title): "guest_feature",
		emojiPrefix(deluxeChange.Title): "deluxe_change",
	}
	if len(prefixes) != 3 {
		t.Fatalf("distinct title emoji prefixes = %d, want 3 (new_release=%q guest_feature=%q deluxe_change=%q)",
			len(prefixes), newRelease.Title, guestFeature.Title, deluxeChange.Title)
	}
}

// emojiPrefix returns the leading rune of an embed title -- every case above
// prefixes the title with one of the three distinct emoji constants.
func emojiPrefix(title string) string {
	r, _ := utf8.DecodeRuneInString(title)
	return string(r)
}

func TestFormatEmbed_NewRelease_MusicBrainz(t *testing.T) {
	ev := sqlc.Event{
		Source:      sourceMusicBrainz,
		EventType:   eventTypeNewRelease,
		ExternalID:  "rg-mbid-1",
		Title:       "First Album",
		ArtistName:  "Test Artist",
		ReleaseDate: strPtr("2020-01-01"),
		CoverArtUrl: strPtr("https://coverartarchive.org/release-group/rg-mbid-1/front"),
		ReleaseType: strPtr("album"),
	}
	embed := formatEmbed(ev)

	if embed.Color != colorNewRelease {
		t.Errorf("Color = %d, want %d", embed.Color, colorNewRelease)
	}
	if !strings.HasPrefix(embed.Title, emojiNewRelease) || !strings.Contains(embed.Title, "First Album") {
		t.Errorf("Title = %q, want it to start with %q and contain %q", embed.Title, emojiNewRelease, "First Album")
	}
	wantURL := "https://musicbrainz.org/release-group/rg-mbid-1"
	if embed.URL != wantURL {
		t.Errorf("URL = %q, want %q", embed.URL, wantURL)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != *ev.CoverArtUrl {
		t.Errorf("Thumbnail = %v, want %q", embed.Thumbnail, *ev.CoverArtUrl)
	}

	fieldValue := func(name string) (string, bool) {
		for _, f := range embed.Fields {
			if f.Name == name {
				return f.Value, true
			}
		}
		return "", false
	}
	if v, ok := fieldValue("Artist"); !ok || v != "Test Artist" {
		t.Errorf("Artist field = (%q, %v), want (%q, true)", v, ok, "Test Artist")
	}
	if v, ok := fieldValue("Release Date"); !ok || v != "2020-01-01" {
		t.Errorf("Release Date field = (%q, %v), want (%q, true)", v, ok, "2020-01-01")
	}
	if v, ok := fieldValue("Type"); !ok || v != "Album" {
		t.Errorf("Type field = (%q, %v), want (%q, true) (title-cased for display)", v, ok, "Album")
	}
}

func TestFormatEmbed_NewRelease_Deezer(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceDeezer,
		EventType:  eventTypeNewRelease,
		ExternalID: "123456",
		Title:      "Deezer Album",
		ArtistName: "Test Artist",
	}
	embed := formatEmbed(ev)

	wantURL := "https://www.deezer.com/album/123456"
	if embed.URL != wantURL {
		t.Errorf("URL = %q, want %q", embed.URL, wantURL)
	}
	if embed.Color != colorNewRelease {
		t.Errorf("Color = %d, want %d (must match the MusicBrainz new_release arm)", embed.Color, colorNewRelease)
	}
}

func TestFormatEmbed_GuestFeature(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeGuestFeature,
		ExternalID: "rec-mbid-1",
		Title:      "Feature Track",
		ArtistName: "Primary Artist",
	}
	embed := formatEmbed(ev)

	if embed.Color != colorGuestFeature {
		t.Errorf("Color = %d, want %d", embed.Color, colorGuestFeature)
	}
	wantURL := "https://musicbrainz.org/recording/rec-mbid-1"
	if embed.URL != wantURL {
		t.Errorf("URL = %q, want %q", embed.URL, wantURL)
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Name != "Artist" || embed.Fields[0].Value != "Primary Artist" {
		t.Errorf("Fields = %+v, want exactly one Artist=%q field (D-02: no full artist-credit fetch)", embed.Fields, "Primary Artist")
	}
}

func TestFormatEmbed_DeluxeChange_BothCountsPresent(t *testing.T) {
	ev := sqlc.Event{
		Source:             sourceMusicBrainz,
		EventType:          eventTypeDeluxeChange,
		ExternalID:         "rel-mbid-1",
		Title:              "Album (Deluxe)",
		ArtistName:         "Test Artist",
		TrackCount:         i32Ptr(18),
		PreviousTrackCount: i32Ptr(12),
		CoverArtUrl:        strPtr("https://coverartarchive.org/release-group/rg-mbid-1/front"),
	}
	embed := formatEmbed(ev)

	if embed.Color != colorDeluxeChange {
		t.Errorf("Color = %d, want %d", embed.Color, colorDeluxeChange)
	}
	wantURL := "https://musicbrainz.org/release/rel-mbid-1"
	if embed.URL != wantURL {
		t.Errorf("URL = %q, want %q", embed.URL, wantURL)
	}
	if embed.Thumbnail == nil || embed.Thumbnail.URL != *ev.CoverArtUrl {
		t.Errorf("Thumbnail = %v, want %q", embed.Thumbnail, *ev.CoverArtUrl)
	}
	if len(embed.Fields) != 1 || embed.Fields[0].Name != "Tracks" {
		t.Fatalf("Fields = %+v, want exactly one Tracks field", embed.Fields)
	}
	tracks := embed.Fields[0].Value
	if !strings.Contains(tracks, "12") || !strings.Contains(tracks, "18") {
		t.Errorf("Tracks field = %q, want it to contain both 12 and 18", tracks)
	}
}

func TestFormatEmbed_DeluxeChange_NilPreviousTrackCount(t *testing.T) {
	ev := sqlc.Event{
		Source:      sourceMusicBrainz,
		EventType:   eventTypeDeluxeChange,
		ExternalID:  "rel-mbid-1",
		Title:       "Album (Deluxe)",
		ArtistName:  "Test Artist",
		TrackCount:  i32Ptr(18),
		// PreviousTrackCount left nil -- a row inserted before migration 000004.
	}

	var embed struct{ ok bool }
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("formatEmbed panicked on a nil PreviousTrackCount: %v", r)
			}
			embed.ok = true
		}()
		e := formatEmbed(ev)
		if len(e.Fields) != 1 || e.Fields[0].Name != "Tracks" {
			t.Fatalf("Fields = %+v, want exactly one Tracks field", e.Fields)
		}
		if e.Fields[0].Value != "18 tracks" {
			t.Errorf("Tracks field = %q, want %q (current count alone, no arrow)", e.Fields[0].Value, "18 tracks")
		}
		if strings.Contains(e.Fields[0].Value, "→") {
			t.Errorf("Tracks field = %q, must not contain an arrow when there is no previous count", e.Fields[0].Value)
		}
	}()
	if !embed.ok {
		t.Fatal("formatEmbed did not complete")
	}
}

func TestFormatEmbed_DeluxeChange_NilBothCounts(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeDeluxeChange,
		ExternalID: "rel-mbid-1",
		Title:      "Album (Deluxe)",
		ArtistName: "Test Artist",
	}
	embed := formatEmbed(ev)
	if len(embed.Fields) != 0 {
		t.Fatalf("Fields = %+v, want none (both counts nil -- the field must be omitted entirely, never an empty Value)", embed.Fields)
	}
}

func TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeNewRelease,
		ExternalID: "rg-mbid-2",
		Title:      "Undated Album",
		ArtistName: "Test Artist",
		// ReleaseDate, CoverArtUrl, TrackCount, ReleaseType all nil.
	}
	embed := formatEmbed(ev)

	if embed.Thumbnail != nil {
		t.Errorf("Thumbnail = %v, want nil", embed.Thumbnail)
	}
	for _, f := range embed.Fields {
		if f.Value == "" {
			t.Errorf("field %q has an empty Value -- Discord rejects an empty-valued field", f.Name)
		}
		if f.Name == "Release Date" || f.Name == "Type" || f.Name == "Tracks" {
			t.Errorf("unexpected field %q present, want it omitted when the source column is NULL", f.Name)
		}
	}
}

func TestFormatEmbed_TitleTruncatedToRuneLimit(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeNewRelease,
		ExternalID: "rg-1",
		Title:      strings.Repeat("a", 300),
		ArtistName: "Artist",
	}
	embed := formatEmbed(ev)

	if !utf8.ValidString(embed.Title) {
		t.Fatal("Title is not valid UTF-8 after truncation")
	}
	if got := utf8.RuneCountInString(embed.Title); got != titleLimit {
		t.Fatalf("rune count of Title = %d, want %d", got, titleLimit)
	}
}

func TestFormatEmbed_TitleTruncatedToRuneLimit_MultiByte(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeNewRelease,
		ExternalID: "rg-1",
		Title:      strings.Repeat("水", 300), // 3-byte CJK rune, stresses byte-vs-rune truncation
		ArtistName: "Artist",
	}
	embed := formatEmbed(ev)

	if !utf8.ValidString(embed.Title) {
		t.Fatal("Title is not valid UTF-8 after truncation")
	}
	if strings.ContainsRune(embed.Title, utf8.RuneError) {
		t.Fatal("Title contains the UTF-8 replacement character -- a multi-byte rune was split")
	}
	if got := utf8.RuneCountInString(embed.Title); got != titleLimit {
		t.Fatalf("rune count of Title = %d, want %d", got, titleLimit)
	}
}

func TestFormatEmbed_FieldValueTruncatedToRuneLimit(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeNewRelease,
		ExternalID: "rg-1",
		Title:      "Album",
		ArtistName: strings.Repeat("b", 1100),
	}
	embed := formatEmbed(ev)

	var artist string
	for _, f := range embed.Fields {
		if f.Name == "Artist" {
			artist = f.Value
		}
	}
	if got := utf8.RuneCountInString(artist); got != fieldValueLimit {
		t.Fatalf("rune count of Artist field = %d, want %d", got, fieldValueLimit)
	}
}

func TestFormatEmbed_URLsMatchExpectedHostAndPath(t *testing.T) {
	tests := []struct {
		name       string
		ev         sqlc.Event
		wantHost   string
		wantPrefix string
	}{
		{
			name:       "new_release musicbrainz",
			ev:         sqlc.Event{Source: sourceMusicBrainz, EventType: eventTypeNewRelease, ExternalID: "rg-1", Title: "T", ArtistName: "A"},
			wantHost:   "musicbrainz.org",
			wantPrefix: "/release-group/",
		},
		{
			name:       "new_release deezer",
			ev:         sqlc.Event{Source: sourceDeezer, EventType: eventTypeNewRelease, ExternalID: "999", Title: "T", ArtistName: "A"},
			wantHost:   "www.deezer.com",
			wantPrefix: "/album/",
		},
		{
			name:       "guest_feature",
			ev:         sqlc.Event{Source: sourceMusicBrainz, EventType: eventTypeGuestFeature, ExternalID: "rec-1", Title: "T", ArtistName: "A"},
			wantHost:   "musicbrainz.org",
			wantPrefix: "/recording/",
		},
		{
			name:       "deluxe_change",
			ev:         sqlc.Event{Source: sourceMusicBrainz, EventType: eventTypeDeluxeChange, ExternalID: "rel-1", Title: "T", ArtistName: "A"},
			wantHost:   "musicbrainz.org",
			wantPrefix: "/release/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embed := formatEmbed(tt.ev)
			u, err := url.Parse(embed.URL)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", embed.URL, err)
			}
			if u.Host != tt.wantHost {
				t.Errorf("host = %q, want %q", u.Host, tt.wantHost)
			}
			if !strings.HasPrefix(u.Path, tt.wantPrefix) {
				t.Errorf("path = %q, want prefix %q", u.Path, tt.wantPrefix)
			}
		})
	}
}

func TestFormatEmbed_ExternalIDIsPercentEscapedInURL(t *testing.T) {
	ev := sqlc.Event{
		Source:     sourceMusicBrainz,
		EventType:  eventTypeGuestFeature,
		ExternalID: "not a valid/mbid?with=unsafe&chars",
		Title:      "T",
		ArtistName: "A",
	}
	embed := formatEmbed(ev)

	if strings.Contains(embed.URL, " ") {
		t.Fatalf("URL = %q, contains a raw unescaped space", embed.URL)
	}
	u, err := url.Parse(embed.URL)
	if err != nil {
		t.Fatalf("url.Parse(%q): %v", embed.URL, err)
	}
	if u.Path == "" {
		t.Fatalf("parsed URL has an empty path: %q", embed.URL)
	}
}
