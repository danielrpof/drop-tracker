package notifier

import (
	"fmt"
	"net/url"
	"unicode"
	"unicode/utf8"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)

// Event-type constants match the DB CHECK constraint's vocabulary
// (events_event_type_valid, 000003_events.up.sql) verbatim, rather than
// importing internal/detection's unexported equivalents -- this package
// reads rows straight off the events table, so the database's vocabulary,
// not detection's internal constants, is the correct contract here.
const (
	eventTypeNewRelease   = "new_release"
	eventTypeGuestFeature = "guest_feature"
	eventTypeDeluxeChange = "deluxe_change"
)

// Source constants likewise match events_source_valid's CHECK constraint --
// the only two legal values a new_release row's Source column can ever hold.
const (
	sourceMusicBrainz = "musicbrainz"
	sourceDeezer      = "deezer"
)

// Embed side-bar colors (D-01, decimal RGB -- Discord ignores a hex
// string) and emoji title prefixes, one pair per event type, so the three
// types stay scannable at a glance in a busy channel.
const (
	colorNewRelease   = 5763719  // #57F287, green
	colorGuestFeature = 16705372 // #FEE75C, yellow
	colorDeluxeChange = 15418782 // #EB459E, fuchsia

	emojiNewRelease   = "\U0001F195" // 🆕
	emojiGuestFeature = "\U0001F3A4" // 🎤
	emojiDeluxeChange = "\U0001F4BF" // 💿
)

// Discord's documented embed limits (05-RESEARCH.md Code Examples): a title
// over 256 runes or a field value over 1024 runes causes Discord to reject
// the whole request outright, silently dropping a real notification -- this
// is a functional requirement, not only a defensive one, since
// MusicBrainz/Deezer titles are community-editable data with no length
// guarantee (Phase 4's AR-04-01/AR-04-04).
const (
	titleLimit      = 256
	fieldValueLimit = 1024
)

// formatEmbed is a pure event -> discord.Embed transform, operating only on
// ev's already-populated display-snapshot fields (D-05/D-12: never a live
// re-fetch).
func formatEmbed(ev sqlc.Event) discord.Embed {
	switch ev.EventType {
	case eventTypeNewRelease:
		return formatNewRelease(ev)
	case eventTypeGuestFeature:
		return formatGuestFeature(ev)
	case eventTypeDeluxeChange:
		return formatDeluxeChange(ev)
	default:
		// No color/emoji for an event_type this switch does not recognize --
		// the DB CHECK constraint should make this unreachable, but a titled
		// embed with no color is a safer failure mode than a zero Embed{}
		// (an empty message body Discord would reject outright).
		return discord.Embed{Title: truncateRunes(ev.Title, titleLimit)}
	}
}

// formatNewRelease builds NTFY-01's embed: title, artist, cover-art
// thumbnail, release date, and release type, linked to the source's own
// catalogue page.
func formatNewRelease(ev sqlc.Event) discord.Embed {
	embed := discord.Embed{
		Title: truncateRunes(emojiNewRelease+" "+ev.Title, titleLimit),
		Color: colorNewRelease,
		URL:   newReleaseURL(ev.Source, ev.ExternalID),
	}
	embed.Fields = appendField(embed.Fields, "Artist", ev.ArtistName)
	if ev.ReleaseDate != nil {
		embed.Fields = appendField(embed.Fields, "Release Date", *ev.ReleaseDate)
	}
	if ev.ReleaseType != nil {
		embed.Fields = appendField(embed.Fields, "Type", titleCaseReleaseType(*ev.ReleaseType))
	}
	if ev.CoverArtUrl != nil && *ev.CoverArtUrl != "" {
		embed.Thumbnail = &discord.EmbedImage{URL: *ev.CoverArtUrl}
	}
	return embed
}

// formatGuestFeature builds NTFY-02's embed: title, the primary credited
// artist from the row's own snapshot (D-02 -- no full artist-credit
// re-fetch), and a link to the recording on MusicBrainz.
func formatGuestFeature(ev sqlc.Event) discord.Embed {
	embed := discord.Embed{
		Title: truncateRunes(emojiGuestFeature+" "+ev.Title, titleLimit),
		Color: colorGuestFeature,
		URL:   musicBrainzRecordingURL(ev.ExternalID),
	}
	embed.Fields = appendField(embed.Fields, "Artist", ev.ArtistName)
	return embed
}

// formatDeluxeChange builds NTFY-03's embed: title, the old-to-new
// track-count delta, and a link to the winning release on MusicBrainz.
func formatDeluxeChange(ev sqlc.Event) discord.Embed {
	embed := discord.Embed{
		Title: truncateRunes(emojiDeluxeChange+" "+ev.Title, titleLimit),
		Color: colorDeluxeChange,
		URL:   musicBrainzReleaseURL(ev.ExternalID),
	}
	embed.Fields = appendField(embed.Fields, "Tracks", tracksFieldValue(ev.PreviousTrackCount, ev.TrackCount))
	if ev.CoverArtUrl != nil && *ev.CoverArtUrl != "" {
		embed.Thumbnail = &discord.EmbedImage{URL: *ev.CoverArtUrl}
	}
	return embed
}

// tracksFieldValue renders the Tracks field's three possible shapes (D-03):
// both counts present renders the arrow delta; only the current count
// present (a deluxe_change row written before migration 000004, whose
// previous_track_count is permanently NULL) renders that count alone with
// no arrow; neither present returns "" so appendField omits the field
// entirely rather than emitting an empty-valued one.
func tracksFieldValue(previous, current *int32) string {
	switch {
	case previous != nil && current != nil:
		return fmt.Sprintf("%d → %d tracks", *previous, *current)
	case current != nil:
		return fmt.Sprintf("%d tracks", *current)
	default:
		return ""
	}
}

// appendField truncates value to fieldValueLimit runes and appends an
// EmbedField named name to fields -- unless the truncated value is empty,
// in which case fields is returned unchanged. Discord rejects an
// empty-valued field, and an omitted field reads correctly while an
// empty-valued one reads as a rendering bug.
func appendField(fields []discord.EmbedField, name, value string) []discord.EmbedField {
	value = truncateRunes(value, fieldValueLimit)
	if value == "" {
		return fields
	}
	return append(fields, discord.EmbedField{Name: name, Value: value})
}

// newReleaseURL branches on ev.Source (the two legal values enforced by the
// events_source_valid CHECK constraint) to build a new_release embed's
// link -- MusicBrainz's release-group page or Deezer's album page.
func newReleaseURL(source, externalID string) string {
	switch source {
	case sourceMusicBrainz:
		return musicBrainzReleaseGroupURL(externalID)
	case sourceDeezer:
		return "https://www.deezer.com/album/" + url.PathEscape(externalID)
	default:
		return ""
	}
}

// The three musicbrainz*URL helpers, and newReleaseURL's Deezer branch
// above, all build a link by escaping external_id into a fixed path
// template with url.PathEscape -- never by interpolating Title or
// ArtistName (community-editable free text) into a URL. external_id is the
// only trustworthy id-namespace input a link is built from (T-05-06).
func musicBrainzReleaseGroupURL(externalID string) string {
	return "https://musicbrainz.org/release-group/" + url.PathEscape(externalID)
}

func musicBrainzRecordingURL(externalID string) string {
	return "https://musicbrainz.org/recording/" + url.PathEscape(externalID)
}

func musicBrainzReleaseURL(externalID string) string {
	return "https://musicbrainz.org/release/" + url.PathEscape(externalID)
}

// titleCaseReleaseType upper-cases the first rune of a stored release_type
// value for display ("album" -> "Album") -- the stored column itself stays
// lowercase, matching detection.detectableReleaseTypes' vocabulary; only
// this formatting layer changes casing.
func titleCaseReleaseType(releaseType string) string {
	if releaseType == "" {
		return ""
	}
	r := []rune(releaseType)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// truncateRunes cuts s to at most limit runes, cutting on a rune boundary.
// Go's len/slicing on a string counts bytes, and slicing to a byte offset
// can split a multi-byte character in half, producing an invalid UTF-8
// sequence Discord renders as a replacement glyph (T-05-02).
func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	r := []rune(s)
	return string(r[:limit])
}
