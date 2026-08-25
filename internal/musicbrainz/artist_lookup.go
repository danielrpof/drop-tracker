package musicbrainz

import (
	"context"
	"errors"
)

// ArtistRelationURL is the nested url object on an ArtistRelation entry.
type ArtistRelationURL struct {
	Resource string `json:"resource"`
}

// ArtistRelation is a single url-rels entry from ws/2/artist's
// inc=url-rels response. Only Type and URL.Resource are decoded -- the
// ~10 other relation keys MusicBrainz returns are deliberately not modeled,
// since nothing in this codebase consumes them and encoding/json ignores
// unknown keys.
type ArtistRelation struct {
	Type string            `json:"type"`
	URL  ArtistRelationURL `json:"url"`
}

// ArtistAlias is a single aliases entry from ws/2/artist's inc=aliases
// response. Primary and Locale are plain value types, not pointers: upstream
// sends JSON null for both on most aliases, and encoding/json decoding null
// into a non-pointer bool/string is a no-op, leaving the Go zero value
// (false/"") with no decode error.
type ArtistAlias struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
	Locale  string `json:"locale"`
}

// ArtistDetail is the envelope ws/2/artist/{mbid}?inc=url-rels+aliases
// returns.
type ArtistDetail struct {
	MBID      string           `json:"id"`
	Name      string           `json:"name"`
	Relations []ArtistRelation `json:"relations"`
	Aliases   []ArtistAlias    `json:"aliases"`
}

// LookupArtist is a RED-phase placeholder -- not yet implemented. See
// artist_lookup_test.go for the behavior this must satisfy.
func (c *Client) LookupArtist(ctx context.Context, mbid string) (ArtistDetail, error) {
	return ArtistDetail{}, errors.New("musicbrainz: LookupArtist not yet implemented")
}
