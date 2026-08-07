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

// Album is Deezer's /artist/{id}/albums result shape, decoded from the
// live-verified response documented in 03-RESEARCH.md. ID is int64 for the
// same reason Artist.ID is (no float64 precision loss). ReleaseDate is kept
// a raw string: Deezer returns partial dates for some releases (e.g. "2026"
// or "2026-05"), and parsing into time.Time here would either fail or
// invent a day/month that Phase 4's diff logic would then treat as real
// data.
//
// RecordType is carried as an opaque string and is not validated against an
// allow-list here -- only "album" was observed live this research session;
// "single", "ep" and "compilation" are community-sourced (assumption A2).
// An unexpected value therefore cannot drop a release; Phase 4 re-verifies
// the value set before filtering on it.
type Album struct {
	ID             int64  `json:"id"`
	Title          string `json:"title"`
	Link           string `json:"link"`
	Cover          string `json:"cover"`
	ReleaseDate    string `json:"release_date"`
	RecordType     string `json:"record_type"`
	Tracklist      string `json:"tracklist"`
	ExplicitLyrics bool   `json:"explicit_lyrics"`
	Type           string `json:"type"`
}

// artistAlbumsResponse is the unexported envelope Deezer's
// /artist/{id}/albums endpoint returns. Shares the Data/Total/Next shape
// with artistSearchResponse.
type artistAlbumsResponse struct {
	Data  []Album `json:"data"`
	Total int     `json:"total"`
	Next  string  `json:"next"`
}

// ArtistAlbums fetches the albums/releases for artistID from Deezer's
// /artist/{id}/albums endpoint (D-12). An empty or whitespace-only
// artistID -- the shape of a nil watchlist.Entry.DeezerID -- returns
// ErrEmptyArtistID before any URL is built, so a caller-supplied empty id
// can never construct a request path with a doubled slash (D-06 pitfall 3,
// T-03-10). A nonexistent artist id is not an error: Deezer returns HTTP
// 200 with an empty data array, and this method returns a non-nil,
// zero-length slice with a nil error so a stale Deezer id degrades
// gracefully rather than failing the poll cycle.
func (c *Client) ArtistAlbums(ctx context.Context, artistID string, limit int) ([]Album, error) {
	trimmed := strings.TrimSpace(artistID)
	if trimmed == "" {
		return nil, ErrEmptyArtistID
	}

	u, err := url.Parse(c.baseURL + "/artist/" + url.PathEscape(trimmed) + "/albums")
	if err != nil {
		return nil, fmt.Errorf("deezer: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(clampLimit(limit)))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("deezer: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("deezer: artist albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code (mirrors
		// T-03-01/V13).
		return nil, fmt.Errorf("deezer: artist albums: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("deezer: read artist albums response: %w", err)
	}

	var env artistAlbumsResponse
	if err := decodeChecked(body, &env); err != nil {
		return nil, fmt.Errorf("deezer: artist albums: %w", err)
	}

	// Always a non-nil, zero-length-when-empty slice -- never nil -- so a
	// nonexistent/stale artist id degrades to an empty result rather than an
	// error (D-06 companion behavior).
	albums := make([]Album, 0, len(env.Data))
	albums = append(albums, env.Data...)
	return albums, nil
}
