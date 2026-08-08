package detection

import (
	"strings"

	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// detectableReleaseTypes is deliberately narrower than watchlist.ReleaseTypes
// (which also carries "deluxe"): "deluxe" is a pseudo-type with no
// MusicBrainz primary-type counterpart -- MusicBrainz's primary-type field
// only ever takes values like Album/Single/EP/Broadcast/Other
// (04-RESEARCH.md Pitfall #7) -- so matching it against a primary-type field
// can never succeed. "deluxe" is instead a boolean gate on whether
// deluxe-change detection runs for an artist at all (deluxeDetectionEnabled),
// never a type a release-group can be filtered by.
var detectableReleaseTypes = map[string]struct{}{
	"album":  {},
	"single": {},
	"ep":     {},
}

// releaseTypeAllowed reports whether a release-group's primaryType (a raw
// MusicBrainz value such as "Album", capitalised) is one of the entry's
// preferred release types (D-17). primaryType is lowercased and trimmed
// before comparison -- entry.ReleaseTypes is already normalised lowercase
// (watchlist.normalizeSet), so only the caller-supplied MusicBrainz value
// needs folding here.
//
// primaryType is checked against detectableReleaseTypes first: an empty
// string, "Broadcast", "Other", or any other MusicBrainz primary-type
// outside {album, single, ep} is never allowed, regardless of what entry
// prefers -- and "deluxe" specifically can never match here, since no
// MusicBrainz primary-type field is ever the literal "deluxe"
// (deluxeDetectionEnabled is the separate boolean gate for that axis).
//
// entry.ReleaseTypes is read by range-based membership, never a fixed-index
// read (T-04-09) -- an empty ReleaseTypes slice allows nothing.
func releaseTypeAllowed(entry watchlist.Entry, primaryType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(primaryType))
	if _, ok := detectableReleaseTypes[normalized]; !ok {
		return false
	}
	for _, rt := range entry.ReleaseTypes {
		if rt == normalized {
			return true
		}
	}
	return false
}

// deluxeDetectionEnabled reports whether "deluxe" is present in
// entry.ReleaseTypes -- the boolean gate on whether deluxe-change detection
// runs for this artist at all (plan 04-04 consumes this), independent of any
// MusicBrainz primary-type field.
func deluxeDetectionEnabled(entry watchlist.Entry) bool {
	for _, rt := range entry.ReleaseTypes {
		if rt == "deluxe" {
			return true
		}
	}
	return false
}

// eventTypeMuted reports whether eventType is present in
// entry.MutedEventTypes (D-18, WLST-06/NTFY-04), applied at detection time
// rather than deferred to Phase 5's notify step. Unlike releaseTypeAllowed,
// which applies only to new_release (a recording has no primary-type), the
// mute axis applies uniformly to all three event types
// (new_release/guest_feature/deluxe_change).
func eventTypeMuted(entry watchlist.Entry, eventType string) bool {
	for _, t := range entry.MutedEventTypes {
		if t == eventType {
			return true
		}
	}
	return false
}
