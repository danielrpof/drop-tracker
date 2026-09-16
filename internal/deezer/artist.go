package deezer

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ArtistByID fetches a single artist by id from Deezer's /artist/{id}
// endpoint -- the confirming fetch D-09r Tier 0 makes after resolving a
// candidate id from a MusicBrainz url-rel, so a pulled or renumbered Deezer
// artist page is caught before Tier 0 reports a confident match. Modeled on
// fetchArtistAlbumsPage: same trim-then-ErrEmptyArtistID guard, same
// ctx.Err() check, same url.PathEscape path building, same doRequest seam,
// same status-code-only error text, same io.ReadAll + decodeChecked decode
// order. The one difference: no limit/index query parameters at all, since
// this endpoint returns a single entity.
//
// Routing the decode through decodeChecked is load-bearing, not
// boilerplate: a Deezer artist page that was pulled or renumbered answers
// with HTTP 200 and an in-body error envelope, so a status-only check would
// hand the caller a zero-valued Artist and let it report a confident match
// on an id that no longer exists.
func (c *Client) ArtistByID(ctx context.Context, artistID string) (Artist, error) {
	trimmed := strings.TrimSpace(artistID)
	if trimmed == "" {
		return Artist{}, ErrEmptyArtistID
	}
	if err := ctx.Err(); err != nil {
		return Artist{}, err
	}

	u, err := url.Parse(c.baseURL + "/artist/" + url.PathEscape(trimmed))
	if err != nil {
		return Artist{}, fmt.Errorf("deezer: parse base url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Artist{}, fmt.Errorf("deezer: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return Artist{}, fmt.Errorf("deezer: artist by id: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code (mirrors
		// T-03-01/V13).
		return Artist{}, fmt.Errorf("deezer: artist by id: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Artist{}, fmt.Errorf("deezer: read artist by id response: %w", err)
	}

	var artist Artist
	if err := decodeChecked(body, &artist); err != nil {
		return Artist{}, fmt.Errorf("deezer: artist by id: %w", err)
	}
	return artist, nil
}
