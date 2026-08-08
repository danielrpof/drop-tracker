package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	// releasePageSize mirrors releaseGroupPageSize -- MusicBrainz's
	// documented "limit" ceiling (maxLimit).
	releasePageSize = maxLimit

	// maxReleasePages bounds ReleasesByReleaseGroup's pagination loop
	// (T-04-19) at the same ceiling ReleaseGroupsByArtist/RecordingsByArtist
	// use -- a release-group's own release count is upstream-controlled, and
	// a malformed or hostile count must not drive requests forever against a
	// free public API.
	maxReleasePages = 10

	// MaxReleaseBrowseItems is the hard ceiling ReleasesByReleaseGroup can
	// ever return (releasePageSize * maxReleasePages), exported so a caller
	// (internal/detection) can detect a truncated fetch without reaching
	// into this package's unexported pagination constants -- mirrors
	// MaxRecordingBrowseItems's existing convention.
	MaxReleaseBrowseItems = releasePageSize * maxReleasePages
)

// Medium and Release are ws/2/release browse-by-release-group result shapes
// (D-01, D-02). [ASSUMED] per 04-RESEARCH.md Assumption A1 -- reconstructed
// by direct analogy to the live-verified release-group envelope from Phase
// 3 (release-group-count/release-group-offset/release-groups ->
// release-count/release-offset/releases), not confirmed against a live
// MusicBrainz response this session (MusicBrainz was unreachable -- see
// 04-RESEARCH.md's Assumptions Log). Go's encoding/json silently leaves a
// mismatched field at its zero value rather than erroring, so a wrong field
// name here would make DTCT-02 silently never fire. Re-verify against a
// live response and correct these field names if MusicBrainz becomes
// reachable during execution.
type Medium struct {
	Format     string `json:"format"`
	Position   int    `json:"position"`
	TrackCount int    `json:"track-count"`
}

// Release is a single ws/2/release browse result, with per-medium
// track-count detail via inc=media. Date is kept as the opaque partial-date
// string MusicBrainz returns -- same rationale as ReleaseGroup.
// FirstReleaseDate: MusicBrainz allows partial dates (year-only,
// year-month) and an empty string for an undated release, so parsing into
// time.Time here would either fail or invent a month/day.
type Release struct {
	MBID   string   `json:"id"`
	Title  string   `json:"title"`
	Status string   `json:"status"`
	Date   string   `json:"date"`
	Media  []Medium `json:"media"`
}

// TrackCount sums every medium's track count -- required for multi-disc
// releases, where D-02's comparison must use the release's TOTAL track
// count, not any single disc's (a two-disc deluxe edition would otherwise
// read as smaller than a single-disc original). Summed by range with no
// fixed-index access (T-04-18, ASVS V5): an absent, empty, or
// partially-populated media array yields 0 rather than panicking.
func (r Release) TrackCount() int {
	total := 0
	for _, m := range r.Media {
		total += m.TrackCount
	}
	return total
}

// releaseEnvelope is the unexported envelope MusicBrainz's /ws/2/release
// browse endpoint returns. Count and Offset drive ReleasesByReleaseGroup's
// pagination loop.
type releaseEnvelope struct {
	Releases []Release `json:"releases"`
	Count    int       `json:"release-count"`
	Offset   int       `json:"release-offset"`
}

// ReleasesByReleaseGroup browses every release MusicBrainz has within
// groupMBID's release-group, with per-medium track-count detail (D-01,
// inc=media). Used only for release-groups already in the seen store
// (D-04) -- internal/detection never calls this for a group discovered in
// the same cycle.
//
// Pages are fetched sequentially (never concurrently) and bounded by
// maxReleasePages (T-04-19), mirroring ReleaseGroupsByArtist's contract
// exactly: a malformed or hostile release-count from upstream cannot drive
// an unbounded request loop, and issuing pages concurrently would multiply
// this client's instantaneous rate against MusicBrainz beyond the
// operator-configured limiter. Hitting the page ceiling returns the
// accumulated releases and a nil error -- a truncated fetch is a
// data-completeness limit, not a failure.
func (c *Client) ReleasesByReleaseGroup(ctx context.Context, groupMBID string) ([]Release, error) {
	trimmed := strings.TrimSpace(groupMBID)
	if trimmed == "" {
		return nil, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	releases := make([]Release, 0, releasePageSize)
	offset := 0
	for page := 0; page < maxReleasePages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		env, err := c.fetchReleasePage(ctx, trimmed, offset)
		if err != nil {
			return nil, err
		}

		releases = append(releases, env.Releases...)

		// Terminate as soon as either holds: we've collected everything
		// upstream reports (Count), or the page came back empty -- a
		// runaway/hostile Count must never keep the loop spinning past an
		// empty page (T-04-19).
		if len(env.Releases) == 0 || len(releases) >= env.Count {
			return releases, nil
		}
		// Advance by the number of entries actually returned, not the
		// requested page size, so a short page can never skip records.
		offset += len(env.Releases)
	}

	return releases, nil
}

// fetchReleasePage issues a single /release page request at offset through
// the shared doRequest helper so no page request can bypass the limiter or
// User-Agent header (T-04-20) -- every ReleasesByReleaseGroup page request
// inherits the operator-configured rate limiter and the mandatory
// User-Agent header this way, never touching the transport directly.
func (c *Client) fetchReleasePage(ctx context.Context, groupMBID string, offset int) (releaseEnvelope, error) {
	u, err := url.Parse(c.baseURL + "/release")
	if err != nil {
		return releaseEnvelope{}, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("release-group", groupMBID)
	q.Set("inc", "media")
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(releasePageSize))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return releaseEnvelope{}, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return releaseEnvelope{}, fmt.Errorf("musicbrainz: releases by release group: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-04-23, V13).
		return releaseEnvelope{}, fmt.Errorf("musicbrainz: releases by release group: unexpected status %d", resp.StatusCode)
	}

	var env releaseEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return releaseEnvelope{}, fmt.Errorf("musicbrainz: decode release response: %w", err)
	}
	return env, nil
}
