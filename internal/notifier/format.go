package notifier

import (
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

// formatEmbed is a pure event -> discord.Embed transform, operating only on
// ev's already-populated display-snapshot fields (D-05/D-12: never a live
// re-fetch). This tracer task establishes the switch's final shape for all
// three event types; plan 05-03 fills in the remaining per-type fields,
// links, and truncation -- deliberately left thin here per the plan's own
// scope note.
func formatEmbed(ev sqlc.Event) discord.Embed {
	switch ev.EventType {
	case eventTypeNewRelease:
		embed := discord.Embed{
			Title: emojiNewRelease + " " + ev.Title,
			Color: colorNewRelease,
			Fields: []discord.EmbedField{
				{Name: "Artist", Value: ev.ArtistName},
			},
		}
		if ev.CoverArtUrl != nil {
			embed.Thumbnail = &discord.EmbedImage{URL: *ev.CoverArtUrl}
		}
		return embed
	case eventTypeGuestFeature:
		return discord.Embed{
			Title: emojiGuestFeature + " " + ev.Title,
			Color: colorGuestFeature,
			Fields: []discord.EmbedField{
				{Name: "Artist", Value: ev.ArtistName},
			},
		}
	case eventTypeDeluxeChange:
		embed := discord.Embed{
			Title: emojiDeluxeChange + " " + ev.Title,
			Color: colorDeluxeChange,
		}
		if ev.CoverArtUrl != nil {
			embed.Thumbnail = &discord.EmbedImage{URL: *ev.CoverArtUrl}
		}
		return embed
	default:
		// No color/emoji for an event_type this switch does not recognize --
		// the DB CHECK constraint should make this unreachable, but a titled
		// embed with no color is a safer failure mode than a zero Embed{}
		// (an empty message body Discord would reject outright).
		return discord.Embed{Title: ev.Title}
	}
}
