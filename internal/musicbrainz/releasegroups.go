package musicbrainz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// ErrEmptyMBID is returned when ReleaseGroupsByArtist is called with an
// empty/whitespace-only artist MBID, before any outbound request is built.
var ErrEmptyMBID = errors.New("musicbrainz: empty artist mbid")

const (
	// releaseGroupPageSize is MusicBrainz's documented "limit" ceiling
	// (maxLimit) -- fetching a full page per request means a typical
	// artist's discography fits in one request, and only prolific artists
	// ever need a second page.
	releaseGroupPageSize = maxLimit

	// maxReleaseGroupPages bounds ReleaseGroupsByArtist's pagination loop
	// (T-03-12). release-group-count is a value the *upstream* controls: a
	// malformed or hostile count would otherwise drive requests forever
	// against a free public API and stall the whole poll cycle. At
	// releaseGroupPageSize (100) this is a 1000 release-group ceiling per
	// artist per poll cycle -- far above any real discography.
	maxReleaseGroupPages = 10
)

// ReleaseGroup is MusicBrainz's ws/2/release-group browse-by-artist result
// shape (D-10), decoded from the live-verified response documented in
// 03-RESEARCH.md. FirstReleaseDate is kept as the opaque string MusicBrainz
// returns: MusicBrainz allows partial dates (year-only, year-month) and an
// empty string for undated release-groups, so parsing into time.Time here
// would either fail or invent a month/day that Phase 4's new-release diff
// would then treat as real data.
type ReleaseGroup struct {
	MBID             string   `json:"id"`
	Title            string   `json:"title"`
	PrimaryType      string   `json:"primary-type"`
	SecondaryTypes   []string `json:"secondary-types"`
	FirstReleaseDate string   `json:"first-release-date"`
	Disambiguation   string   `json:"disambiguation"`
}

// releaseGroupEnvelope is the unexported envelope MusicBrainz's
// /ws/2/release-group browse endpoint returns. Count and Offset drive
// ReleaseGroupsByArtist's pagination loop.
type releaseGroupEnvelope struct {
	ReleaseGroups []ReleaseGroup `json:"release-groups"`
	Count         int            `json:"release-group-count"`
	Offset        int            `json:"release-group-offset"`
}

// ReleaseGroupsByArtist browses MusicBrainz's release-groups for the artist
// identified by mbid (D-10), returning results in MusicBrainz's own
// upstream order -- this method never sorts. No "type" filter is sent:
// Phase 4's per-artist release-type preferences (Phase 2 WLST-05) are
// applied at detection time, and filtering at fetch time would make a
// preference change silently invisible until the next full refetch.
//
// This issues exactly one page request; pagination across an artist's full
// discography is added in a later plan step.
func (c *Client) ReleaseGroupsByArtist(ctx context.Context, mbid string) ([]ReleaseGroup, error) {
	trimmed := strings.TrimSpace(mbid)
	if trimmed == "" {
		return nil, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	env, err := c.fetchReleaseGroupPage(ctx, trimmed, 0)
	if err != nil {
		return nil, err
	}

	groups := make([]ReleaseGroup, 0, len(env.ReleaseGroups))
	groups = append(groups, env.ReleaseGroups...)
	return groups, nil
}

// fetchReleaseGroupPage issues a single /release-group page request at
// offset through the shared doRequest helper so no page request can bypass
// the limiter or User-Agent header (D-07). ReleaseGroupsByArtist wraps this
// in a bounded, sequential pagination loop -- pages are never fetched
// concurrently, which would multiply this client's instantaneous rate
// against MusicBrainz beyond the operator-configured limiter.
func (c *Client) fetchReleaseGroupPage(ctx context.Context, mbid string, offset int) (releaseGroupEnvelope, error) {
	u, err := url.Parse(c.baseURL + "/release-group")
	if err != nil {
		return releaseGroupEnvelope{}, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("artist", mbid)
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(releaseGroupPageSize))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return releaseGroupEnvelope{}, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return releaseGroupEnvelope{}, fmt.Errorf("musicbrainz: release groups by artist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-03-16, V13).
		return releaseGroupEnvelope{}, fmt.Errorf("musicbrainz: release groups by artist: unexpected status %d", resp.StatusCode)
	}

	var env releaseGroupEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return releaseGroupEnvelope{}, fmt.Errorf("musicbrainz: decode release group response: %w", err)
	}
	return env, nil
}
