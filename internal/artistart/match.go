// Package artistart owns D-08's artist-art match rule and D-09's
// fail-closed policy in exactly one place. Every artists.image_url row in
// this database is NULL and always has been: MusicBrainz has no artist
// images, and adds only ever flow from a MusicBrainz search result. Deezer
// has artist pictures but no MusicBrainz id, and this project has
// deliberately never had cross-source identity resolution -- this package
// is the one place that resolution happens, so bug #3's two call sites (the
// add-time match and the backfill sweep, sibling plan 13-03) can never
// drift apart by each carrying their own copy of the rule.
package artistart

import (
	"context"

	"github.com/danielrpof/drop-tracker/internal/deezer"
	"github.com/danielrpof/drop-tracker/internal/musicbrainz"
)

// Result is the outcome of a Match call. DeezerID and ImageURL are *string
// specifically so they drop into sqlc.UpsertArtistParams unchanged, where
// the existing COALESCE clauses read nil as "the caller said nothing about
// this field" rather than "blank it". Matched: false is D-09's fail-closed
// outcome, and both pointers are always nil in that case.
type Result struct {
	DeezerID *string
	ImageURL *string
	Matched  bool
}

// ArtistSearcher is the narrow seam this package depends on for Deezer
// artist search.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]deezer.Artist, error)
}

// AlbumLister is the narrow seam this package depends on for Deezer album
// listing.
type AlbumLister interface {
	ArtistAlbums(ctx context.Context, artistID string, limit int) ([]deezer.Album, error)
}

// ReleaseGroupLister is the narrow seam this package depends on for
// MusicBrainz release-group browsing.
type ReleaseGroupLister interface {
	ReleaseGroupsByArtist(ctx context.Context, mbid string) ([]musicbrainz.ReleaseGroup, error)
}

// Matcher is the not-yet-implemented RED-phase skeleton: its fields exist so
// the package compiles and NewMatcher's constructor signature is final, but
// Match itself is a placeholder GREEN will replace.
type Matcher struct {
	search ArtistSearcher
	albums AlbumLister
	groups ReleaseGroupLister
}

// NewMatcher builds a Matcher backed by search, albums and groups.
func NewMatcher(search ArtistSearcher, albums AlbumLister, groups ReleaseGroupLister) *Matcher {
	return &Matcher{search: search, albums: albums, groups: groups}
}

// normalizeArtistName is a RED-phase placeholder -- it deliberately does not
// yet fold whitespace/case/apostrophes/diacritics, so
// TestNormalizeArtistName fails until GREEN implements it.
func normalizeArtistName(s string) string {
	return s
}

// Match is a RED-phase placeholder that always fails closed, so every
// positive-match behavior test fails until GREEN implements the real rule.
func (m *Matcher) Match(_ context.Context, _, _ string) (Result, error) {
	return Result{}, nil
}
