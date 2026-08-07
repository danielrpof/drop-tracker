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
// with artistSearchResponse. Total drives ArtistAlbums's pagination loop;
// Next is not consumed -- the loop advances by "index" itself rather than
// following Deezer's opaque next-page URL, mirroring
// musicbrainz.ReleaseGroupsByArtist's offset-advance shape.
type artistAlbumsResponse struct {
	Data  []Album `json:"data"`
	Total int     `json:"total"`
	Next  string  `json:"next"`
}

// maxAlbumPages bounds ArtistAlbums's pagination loop (mirrors
// musicbrainz.maxReleaseGroupPages, see 03-REVIEW.md WR-02). Deezer's
// "total" is a value the *upstream* controls: a malformed or hostile total
// would otherwise drive requests forever against a free public API and
// stall the whole poll cycle.
const maxAlbumPages = 10

// ArtistAlbums fetches every album/release Deezer has for artistID from the
// /artist/{id}/albums endpoint (D-12), paginating as needed via Deezer's
// "index" offset parameter and returning results in Deezer's own upstream
// order across pages -- this method never sorts. Pages are fetched
// sequentially (never concurrently) and bounded by maxAlbumPages, mirroring
// musicbrainz.ReleaseGroupsByArtist: a malformed or hostile "total" cannot
// drive an unbounded request loop, and issuing pages concurrently would
// multiply this client's instantaneous rate against Deezer beyond the
// operator-configured limiter. Hitting the page ceiling returns the
// accumulated albums and a nil error -- a truncated fetch is a
// data-completeness limit, not a failure, and turning it into an error
// would let one prolific artist abort the whole poll cycle.
//
// An empty or whitespace-only artistID -- the shape of a nil
// watchlist.Entry.DeezerID -- returns ErrEmptyArtistID before any URL is
// built, so a caller-supplied empty id can never construct a request path
// with a doubled slash (D-06 pitfall 3, T-03-10). A nonexistent artist id
// is not an error: Deezer returns HTTP 200 with an empty data array, and
// this method returns a non-nil, zero-length slice with a nil error so a
// stale Deezer id degrades gracefully rather than failing the poll cycle.
func (c *Client) ArtistAlbums(ctx context.Context, artistID string, limit int) ([]Album, error) {
	trimmed := strings.TrimSpace(artistID)
	if trimmed == "" {
		return nil, ErrEmptyArtistID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	pageSize := clampLimit(limit)
	albums := make([]Album, 0, pageSize)
	index := 0
	for page := 0; page < maxAlbumPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		env, err := c.fetchArtistAlbumsPage(ctx, trimmed, pageSize, index)
		if err != nil {
			return nil, err
		}

		albums = append(albums, env.Data...)

		// Terminate as soon as either holds: we've collected everything
		// upstream reports (Total), or the page came back empty -- a
		// runaway/hostile Total must never keep the loop spinning past an
		// empty page (mirrors T-03-12).
		if len(env.Data) == 0 || len(albums) >= env.Total {
			return albums, nil
		}
		// Advance by the number of entries actually returned, not the
		// requested page size, so a short page can never skip records.
		index += len(env.Data)
	}

	return albums, nil
}

// fetchArtistAlbumsPage issues a single /artist/{id}/albums page request at
// index through the shared doRequest helper so no page request can bypass
// the limiter (D-07). ArtistAlbums wraps this in a bounded, sequential
// pagination loop -- pages are never fetched concurrently, which would
// multiply this client's instantaneous rate against Deezer beyond the
// operator-configured limiter.
func (c *Client) fetchArtistAlbumsPage(ctx context.Context, artistID string, limit, index int) (artistAlbumsResponse, error) {
	u, err := url.Parse(c.baseURL + "/artist/" + url.PathEscape(artistID) + "/albums")
	if err != nil {
		return artistAlbumsResponse{}, fmt.Errorf("deezer: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("index", strconv.Itoa(index))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return artistAlbumsResponse{}, fmt.Errorf("deezer: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return artistAlbumsResponse{}, fmt.Errorf("deezer: artist albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code (mirrors
		// T-03-01/V13).
		return artistAlbumsResponse{}, fmt.Errorf("deezer: artist albums: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return artistAlbumsResponse{}, fmt.Errorf("deezer: read artist albums response: %w", err)
	}

	var env artistAlbumsResponse
	if err := decodeChecked(body, &env); err != nil {
		return artistAlbumsResponse{}, fmt.Errorf("deezer: artist albums: %w", err)
	}
	return env, nil
}
