package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
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

// LookupArtist looks up mbid's curated url-rels (D-09r Tier 0's Deezer
// link source) and aliases (Tier 1's alias-retry source) in a single round
// trip -- requesting both inc values together is the point: Tier 1's alias
// retry must not cost a second MusicBrainz round trip against a ~1 req/sec
// budget. Issues exactly one GET through the shared doRequest seam, so it
// inherits the operator-configured limiter and mandatory User-Agent header
// exactly like every other method in this package.
func (c *Client) LookupArtist(ctx context.Context, mbid string) (ArtistDetail, error) {
	trimmed := strings.TrimSpace(mbid)
	if trimmed == "" {
		return ArtistDetail{}, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return ArtistDetail{}, err
	}

	// url.PathEscape is required here, never raw concatenation of the
	// caller-influenced mbid (ASVS V5, T-13-01) -- mbid ultimately
	// originates from community-editable upstream data.
	u, err := url.Parse(c.baseURL + "/artist/" + url.PathEscape(trimmed))
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("inc", "url-rels+aliases")
	q.Set("fmt", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return ArtistDetail{}, fmt.Errorf("musicbrainz: lookup artist: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-13-02, V13).
		return ArtistDetail{}, fmt.Errorf("musicbrainz: lookup artist: unexpected status %d", resp.StatusCode)
	}

	var detail ArtistDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return ArtistDetail{}, fmt.Errorf("musicbrainz: decode artist lookup response: %w", err)
	}
	if detail.Relations == nil {
		detail.Relations = []ArtistRelation{}
	}
	if detail.Aliases == nil {
		detail.Aliases = []ArtistAlias{}
	}
	return detail, nil
}
