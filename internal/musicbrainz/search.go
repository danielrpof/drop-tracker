package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ErrEmptyQuery is returned when SearchArtists is called with an
// empty/whitespace-only query, before any outbound request is built.
var ErrEmptyQuery = errors.New("musicbrainz: empty search query")

// luceneSpecial matches every character (and the two-character operators
// && / ||) that Lucene's query grammar treats specially. MusicBrainz's
// /ws/2/artist search endpoint parses "query" as a Lucene expression, so an
// unescaped caller-supplied string containing any of these either produces
// a query MusicBrainz's parser rejects, or silently changes what is
// matched (e.g. a stray OR/NOT widening or narrowing the search beyond the
// literal string the user typed). See 03-REVIEW.md WR-01.
var luceneSpecial = regexp.MustCompile(`([+\-!(){}\[\]^"~*?:\\]|&&|\|\|)`)

// escapeLucene backslash-escapes every Lucene special character in s so it
// is treated as a literal by MusicBrainz's query parser rather than as
// query-grammar syntax.
func escapeLucene(s string) string {
	return luceneSpecial.ReplaceAllString(s, `\$1`)
}

// Artist is MusicBrainz's ws/2/artist search result shape, decoded from
// the live-verified response documented in 03-RESEARCH.md. Only the fields
// this phase's search-proxy needs are kept -- MusicBrainz's response
// carries more (country, life-span, etc.) with no consumer yet.
type Artist struct {
	MBID           string `json:"id"`
	Name           string `json:"name"`
	SortName       string `json:"sort-name"`
	Disambiguation string `json:"disambiguation"`
	Type           string `json:"type"`
	Score          int    `json:"score"`
}

// artistSearchResponse is the unexported envelope MusicBrainz's
// /ws/2/artist search endpoint returns. count/offset are not consumed --
// no pagination in this plan (03-RESEARCH.md Open Question 1).
type artistSearchResponse struct {
	Artists []Artist `json:"artists"`
}

// SearchArtists queries MusicBrainz's /ws/2/artist endpoint for query,
// scoped to the artist field via Lucene syntax (D-11). limit is clamped to
// MusicBrainz's documented range via clampLimit. Results are returned in
// MusicBrainz's own relevance order -- this method never sorts.
func (c *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, ErrEmptyQuery
	}

	u, err := url.Parse(c.baseURL + "/artist")
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	// Lucene field-scoped form (artist:<query>) rather than a bare query,
	// which would also match alias/sortname (03-RESEARCH.md Code
	// Examples).
	q.Set("query", "artist:"+escapeLucene(trimmed))
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(clampLimit(limit)))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: search artists: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-03-01, V13).
		return nil, fmt.Errorf("musicbrainz: search artists: unexpected status %d", resp.StatusCode)
	}

	var env artistSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("musicbrainz: decode search response: %w", err)
	}

	// Always a non-nil, zero-length-when-empty slice -- never nil -- so no
	// consumer needs to special-case an empty search result.
	artists := make([]Artist, 0, len(env.Artists))
	artists = append(artists, env.Artists...)
	return artists, nil
}
