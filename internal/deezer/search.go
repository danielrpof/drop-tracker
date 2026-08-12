package deezer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Artist is Deezer's /search/artist result shape, decoded from the
// live-verified response documented in 03-RESEARCH.md. ID is declared as
// int64 explicitly rather than letting it land in an untyped decode --
// Deezer ids exceed float64's exact-integer range and a lossy decode would
// silently corrupt the id used to poll that artist (D-12's ArtistAlbums
// callee).
type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Link    string `json:"link"`
	Picture string `json:"picture"`
	NbAlbum int    `json:"nb_album"`
	Type    string `json:"type"`
}

// artistSearchResponse is the unexported envelope Deezer's /search/artist
// endpoint returns. Total/Next are not consumed -- no pagination in this
// plan (03-RESEARCH.md Open Question 1), matching internal/musicbrainz's
// scope.
type artistSearchResponse struct {
	Data  []Artist `json:"data"`
	Total int      `json:"total"`
	Next  string   `json:"next"`
}

// SearchArtists queries Deezer's /search/artist endpoint for query. limit is
// clamped to this package's valid range via clampLimit. Results are returned
// in Deezer's own order -- this method never sorts.
func (c *Client) SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error) {
	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, ErrEmptyQuery
	}

	u, err := url.Parse(c.baseURL + "/search/artist")
	if err != nil {
		return nil, fmt.Errorf("deezer: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("q", trimmed)
	q.Set("limit", strconv.Itoa(clampLimit(limit)))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("deezer: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("deezer: search artists: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code (mirrors
		// T-03-01/V13 in internal/musicbrainz).
		return nil, fmt.Errorf("deezer: search artists: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deezer: read search response: %w", err)
	}

	var env artistSearchResponse
	if err := decodeChecked(body, &env); err != nil {
		return nil, fmt.Errorf("deezer: search artists: %w", err)
	}

	// Always a non-nil, zero-length-when-empty slice -- never nil -- so no
	// consumer needs to special-case an empty search result.
	artists := make([]Artist, 0, len(env.Data))
	artists = append(artists, env.Data...)
	return artists, nil
}
