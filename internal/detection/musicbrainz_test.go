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

func TestReleaseTypeForStorage(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name        string
		primaryType string
		want        *string
	}{
		{"typical MusicBrainz value is lowercased", "Album", strPtr("album")},
		{"surrounding whitespace is trimmed", "  Single  ", strPtr("single")},
		{"absent primary type stores SQL NULL, not an empty string", "", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := releaseTypeForStorage(tt.primaryType)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("releaseTypeForStorage(%q) = %q, want nil", tt.primaryType, *got)
				}
				return
			}
			if got == nil {
				t.Fatalf("releaseTypeForStorage(%q) = nil, want %q", tt.primaryType, *tt.want)
			}
			if *got != *tt.want {
				t.Fatalf("releaseTypeForStorage(%q) = %q, want %q", tt.primaryType, *got, *tt.want)
			}
		})
	}
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

func TestEarliestReleaseDate(t *testing.T) {
	tests := []struct {
		name     string
		releases []musicbrainz.RecordingRelease
		want     string
	}{
		{
			name: "different years picks the smallest year",
			releases: []musicbrainz.RecordingRelease{
				{Date: "2021-05"},
				{Date: "2019-08-14"},
				{Date: "2020"},
			},
			want: "2019-08-14",
		},
		{
			// Pins Q2's fix: the original plain-`<` design would have
			// wrongly picked "2020" here, since "2020" < "2020-01-05"
			// lexicographically. The more precise same-year date must win.
			name: "same year, one a strict prefix of the other -- the more precise date wins",
			releases: []musicbrainz.RecordingRelease{
				{Date: "2020"},
				{Date: "2020-01-05"},
			},
			want: "2020-01-05",
		},
		{
			// Same case, reversed order in the input slice -- earlierDate
			// must be order-independent.
			name: "same year, prefix case with reversed input order",
			releases: []musicbrainz.RecordingRelease{
				{Date: "2020-01-05"},
				{Date: "2020"},
			},
			want: "2020-01-05",
		},
		{
			name: "same year, equal precision -- plain comparison is correct",
			releases: []musicbrainz.RecordingRelease{
				{Date: "2020-03"},
				{Date: "2020-01"},
			},
			want: "2020-01",
		},
		{
			name: "empty dates never win by sorting first",
			releases: []musicbrainz.RecordingRelease{
				{Date: ""},
				{Date: ""},
				{Date: "2020"},
			},
			want: "2020",
		},
		{
			name: "all dates empty returns empty",
			releases: []musicbrainz.RecordingRelease{
				{Date: ""},
				{Date: ""},
			},
			want: "",
		},
		{
			name:     "no releases returns empty",
			releases: nil,
			want:     "",
		},
		{
			// WR-01 (13-REVIEW.md): a malformed/community-edited MusicBrainz
			// date shorter than 4 characters must never reach earlierDate's
			// a[:4]/b[:4] slicing -- treat it the same as an empty date.
			name: "malformed date shorter than 4 characters is treated as empty, not selected",
			releases: []musicbrainz.RecordingRelease{
				{Date: "9"},
				{Date: "2020"},
			},
			want: "2020",
		},
		{
			name: "malformed short date does not panic when it is the only release",
			releases: []musicbrainz.RecordingRelease{
				{Date: "20"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := earliestReleaseDate(tt.releases); got != tt.want {
				t.Fatalf("earliestReleaseDate(%+v) = %q, want %q", tt.releases, got, tt.want)
			}
		})
	}
}

func TestGuestFeatureArt(t *testing.T) {
	t.Run("returns the first release carrying a release-group MBID", func(t *testing.T) {
		releases := []musicbrainz.RecordingRelease{
			{Date: "2020-01-01", ReleaseGroup: musicbrainz.RecordingReleaseGroup{}},
			{Date: "2020-02-01", ReleaseGroup: musicbrainz.RecordingReleaseGroup{MBID: "rg-1"}},
		}
		gotMBID, gotURL := guestFeatureArt(releases)
		if gotMBID != "rg-1" {
			t.Errorf("releaseGroupMBID = %q, want %q", gotMBID, "rg-1")
		}
		wantURL := "https://coverartarchive.org/release-group/rg-1/front"
		if gotURL != wantURL {
			t.Errorf("coverArtURL = %q, want %q", gotURL, wantURL)
		}
	})

	t.Run("no release carries a release-group MBID returns two empty strings", func(t *testing.T) {
		releases := []musicbrainz.RecordingRelease{
			{Date: "2020-01-01"},
		}
		gotMBID, gotURL := guestFeatureArt(releases)
		if gotMBID != "" || gotURL != "" {
			t.Fatalf("guestFeatureArt = (%q, %q), want (\"\", \"\")", gotMBID, gotURL)
		}
	})

	t.Run("empty release list returns two empty strings", func(t *testing.T) {
		gotMBID, gotURL := guestFeatureArt(nil)
		if gotMBID != "" || gotURL != "" {
			t.Fatalf("guestFeatureArt(nil) = (%q, %q), want (\"\", \"\")", gotMBID, gotURL)
		}
	})
}

func TestWithinDeluxeRecheckWindow(t *testing.T) {
	tests := []struct {
		name             string
		firstReleaseDate string
		cutoff           string
		want             bool
	}{
		{"after cutoff is checked", "2026-08-01", "2026-05-28", true},
		{"one day after cutoff is checked", "2026-05-29", "2026-05-28", true},
		{"exactly on the cutoff day is checked -- boundary is inclusive", "2026-05-28", "2026-05-28", true},
		{"one day before cutoff is skipped", "2026-05-27", "2026-05-28", false},
		{"years old is skipped", "2020-01-01", "2026-05-28", false},
		{"year-month straddling the cutoff's own year-month is checked", "2026-05", "2026-05-28", true},
		{"year-month before the cutoff's year-month is skipped", "2026-04", "2026-05-28", false},
		{"year-only equal to the cutoff's year is checked", "2026", "2026-05-28", true},
		{"year-only before the cutoff's year is skipped", "2025", "2026-05-28", false},
		{"undated (MusicBrainz's value for an undated group) is checked", "", "2026-05-28", true},
		{"malformed under-4-char date '20' is checked", "20", "2026-05-28", true},
		{"malformed under-4-char date '9' is checked", "9", "2026-05-28", true},
		{"non-numeric garbage is checked", "abcd", "2026-05-28", true},
		{"longer-than-expected shape (full timestamp) is checked", "2026-05-28T00:00:00Z", "2026-05-28", true},
		{
			// Clock-crossing case: cutoff is in the previous calendar year.
			// A year-only date for the cutoff's own year could resolve to
			// December, so it must be checked, not skipped.
			"year-only for a cutoff in the previous calendar year is checked", "2025", "2025-11-03", true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinDeluxeRecheckWindow(tt.firstReleaseDate, tt.cutoff); got != tt.want {
				t.Fatalf("withinDeluxeRecheckWindow(%q, %q) = %v, want %v", tt.firstReleaseDate, tt.cutoff, got, tt.want)
			}
		})
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
