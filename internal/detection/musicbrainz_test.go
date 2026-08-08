package detection

// This file is package detection (whitebox, not detection_test) since
// isGuestFeature is unexported -- mirrors filter_test.go's convention for
// testing an unexported predicate directly.

import (
	"testing"

	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// creditFor builds an ArtistCreditEntry field-by-field, matching
// detector_test.go's mkCredit convention (that file lives in the external
// detection_test package, so this small duplicate avoids crossing the
// internal/external test package split just to share one helper).
func creditFor(mbid, name string) musicbrainz.ArtistCreditEntry {
	var e musicbrainz.ArtistCreditEntry
	e.Name = name
	e.Artist.MBID = mbid
	e.Artist.Name = name
	return e
}

func TestIsGuestFeature_EmptyCredit(t *testing.T) {
	nilCredit := musicbrainz.Recording{MBID: "rec1", Title: "Track", ArtistCredit: nil}
	if isGuestFeature(nilCredit, "watched-mbid") {
		t.Fatal("isGuestFeature with a nil ArtistCredit = true, want false")
	}

	emptyCredit := musicbrainz.Recording{MBID: "rec2", Title: "Track", ArtistCredit: []musicbrainz.ArtistCreditEntry{}}
	if isGuestFeature(emptyCredit, "watched-mbid") {
		t.Fatal("isGuestFeature with an empty ArtistCredit slice = true, want false")
	}
}

func TestIsGuestFeature_MissingArtistID(t *testing.T) {
	rec := musicbrainz.Recording{
		MBID:         "rec1",
		Title:        "Track",
		ArtistCredit: []musicbrainz.ArtistCreditEntry{creditFor("", "Unknown Primary Artist")},
	}
	if !isGuestFeature(rec, "watched-mbid") {
		t.Fatal("isGuestFeature with an empty first-credit artist id = false, want true (an unidentifiable primary credit must not be silently dropped)")
	}
}

func TestIsGuestFeature_Positional(t *testing.T) {
	const watched = "watched-mbid"
	const other = "other-mbid"

	guestAtSecond := musicbrainz.Recording{
		ArtistCredit: []musicbrainz.ArtistCreditEntry{creditFor(other, "Other"), creditFor(watched, "Watched")},
	}
	if !isGuestFeature(guestAtSecond, watched) {
		t.Fatal("watched artist at position 1 = not a guest, want guest")
	}

	primaryFirst := musicbrainz.Recording{
		ArtistCredit: []musicbrainz.ArtistCreditEntry{creditFor(watched, "Watched"), creditFor(other, "Other")},
	}
	if isGuestFeature(primaryFirst, watched) {
		t.Fatal("watched artist at position 0 = guest, want not a guest")
	}

	soloRecording := musicbrainz.Recording{
		ArtistCredit: []musicbrainz.ArtistCreditEntry{creditFor(watched, "Watched")},
	}
	if isGuestFeature(soloRecording, watched) {
		t.Fatal("watched artist at position 0 of a single-entry credit list = guest, want not a guest")
	}
}
