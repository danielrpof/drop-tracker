package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// MaxRecordingReleaseLinks is the ceiling MusicBrainz applies to linked
// entities per single-entity lookup -- a recording carrying more than 25
// linked releases via inc=releases has its release list truncated by
// MusicBrainz itself, not by this client. This is an accepted truncation
// edge (13-RESEARCH.md Assumption A2), not a paginated fetch: there is no
// offset/limit mechanism on this endpoint shape to page through the
// remainder. Nothing in this file consumes this constant; internal/detection
// uses it to make a truncated linked-release list observable in structured
// logs.
const MaxRecordingReleaseLinks = 25

// RecordingReleaseGroup is the nested release-group object on each entry of
// RecordingRelease's Releases -- [ASSUMED] per 13-RESEARCH.md Assumption A1:
// this nesting has not been confirmed against a live MusicBrainz response
// this session. Go's encoding/json silently leaves a mismatched field at its
// zero value rather than erroring, so a wrong field name here would make
// D-01's cover-art sourcing silently never fire.
type RecordingReleaseGroup struct {
	MBID  string `json:"id"`
	Title string `json:"title"`
}

// RecordingRelease is a single release entry from ws/2/recording's
// inc=releases+release-groups response (D-01). [ASSUMED] per 13-RESEARCH.md
// Assumption A1 -- the releases[].release-group nesting is reconstructed by
// direct analogy to this package's other browse-by-artist envelopes, not
// confirmed against a live MusicBrainz response this session (this dev
// box's WSL2 network path cannot reach musicbrainz.org -- see STATE.md's
// waived Phase 3 blocker). Date is kept as the opaque partial-date string
// MusicBrainz returns and is never parsed into time.Time -- MusicBrainz
// allows partial dates (year-only, year-month) and an empty string for an
// undated release, so parsing here would either fail or invent a month/day,
// exactly the rationale Release.Date and ReleaseGroup.FirstReleaseDate
// already document.
type RecordingRelease struct {
	MBID         string                `json:"id"`
	Title        string                `json:"title"`
	Date         string                `json:"date"`
	ReleaseGroup RecordingReleaseGroup `json:"release-group"`
}

// recordingLookupResponse is the unexported envelope MusicBrainz's
// ws/2/recording/{mbid} single-entity lookup endpoint returns. Unlike this
// package's browse-by-query-param methods (ReleaseGroupsByArtist,
// RecordingsByArtist, ReleasesByReleaseGroup), this endpoint returns the
// recording entity itself -- there is no *-count/*-offset pagination
// envelope and no pagination loop.
type recordingLookupResponse struct {
	MBID     string             `json:"id"`
	Title    string             `json:"title"`
	Releases []RecordingRelease `json:"releases"`
}

// ReleasesForRecording looks up mbid's linked releases and release-groups
// (D-01) -- called once per genuinely-new guest-feature recording inside
// internal/detection's detectGuestFeatures, never in a pagination loop.
// Issues exactly one GET through the shared doRequest seam, so it inherits
// the operator-configured rate limiter and mandatory User-Agent header
// exactly like every other method in this package.
func (c *Client) ReleasesForRecording(ctx context.Context, mbid string) ([]RecordingRelease, error) {
	trimmed := strings.TrimSpace(mbid)
	if trimmed == "" {
		return nil, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// url.PathEscape is required here, never raw concatenation of the
	// caller-influenced mbid (ASVS V5, T-13-01) -- mbid ultimately
	// originates from a prior MusicBrainz browse response, semi-trusted
	// community-editable upstream data, not operator input.
	u, err := url.Parse(c.baseURL + "/recording/" + url.PathEscape(trimmed))
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("inc", "releases+release-groups")
	q.Set("fmt", "json")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("musicbrainz: releases for recording: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-13-02, V13).
		return nil, fmt.Errorf("musicbrainz: releases for recording: unexpected status %d", resp.StatusCode)
	}

	var env recordingLookupResponse
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, fmt.Errorf("musicbrainz: decode recording lookup response: %w", err)
	}
	if env.Releases == nil {
		// Non-nil zero-length slice so detectGuestFeatures never has to
		// nil-check -- mirrors ReleasesByReleaseGroup/RecordingsByArtist's
		// existing convention for an absent/empty result.
		return []RecordingRelease{}, nil
	}
	return env.Releases, nil
}
