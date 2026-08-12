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
	// recordingPageSize mirrors releaseGroupPageSize -- MusicBrainz's
	// documented "limit" ceiling (maxLimit).
	recordingPageSize = maxLimit

	// maxRecordingPages bounds RecordingsByArtist's pagination loop at the
	// same ceiling ReleaseGroupsByArtist uses (D-05 locks the recording
	// browse to the same bounded-pagination pattern rather than a tighter,
	// recording-specific cap) -- a malformed or hostile recording-count from
	// upstream cannot drive requests forever (T-04-13). A prolific artist's
	// total recording volume can exceed their release-group count by an
	// order of magnitude (04-RESEARCH.md Pitfall #4), so hitting this
	// ceiling is materially more likely here than for release-groups.
	maxRecordingPages = 10

	// MaxRecordingBrowseItems is the hard ceiling RecordingsByArtist can
	// ever return (recordingPageSize * maxRecordingPages). Exported so a
	// caller (internal/detection) can detect a truncated fetch --
	// len(recordings) >= MaxRecordingBrowseItems -- and make it visible in
	// structured logs rather than silently under-detecting guest features,
	// without reaching into this package's unexported pagination constants.
	MaxRecordingBrowseItems = recordingPageSize * maxRecordingPages
)

// ArtistCreditEntry and Recording are ws/2/recording browse-by-artist-credit
// result shapes (D-05, D-06). [ASSUMED] per 04-RESEARCH.md Assumption A2 --
// reconstructed by direct analogy to the live-verified release-group
// envelope from Phase 3 (release-group-count/release-group-offset/
// release-groups -> recording-count/recording-offset/recordings), not
// confirmed against a live MusicBrainz response this session (MusicBrainz
// was unreachable -- see 04-RESEARCH.md's Assumptions Log). Go's
// encoding/json silently leaves a mismatched field at its zero value rather
// than erroring, so a wrong field name here would make DTCT-03 silently
// never fire. Re-verify against a live response and correct these field
// names if MusicBrainz becomes reachable during execution.
type ArtistCreditEntry struct {
	Name       string `json:"name"`
	JoinPhrase string `json:"joinphrase"`
	Artist     struct {
		MBID string `json:"id"`
		Name string `json:"name"`
	} `json:"artist"`
}

// Recording is a single ws/2/recording browse result. ArtistCredit carries
// every credited artist in upstream order -- internal/detection's
// isGuestFeature applies D-06's positional rule against index 0 of this
// slice; RecordingsByArtist itself never filters or reorders it.
type Recording struct {
	MBID         string              `json:"id"`
	Title        string              `json:"title"`
	ArtistCredit []ArtistCreditEntry `json:"artist-credit"`
}

// recordingEnvelope is the unexported envelope MusicBrainz's
// /ws/2/recording browse endpoint returns. Count and Offset drive
// RecordingsByArtist's pagination loop.
type recordingEnvelope struct {
	Recordings []Recording `json:"recordings"`
	Count      int         `json:"recording-count"`
	Offset     int         `json:"recording-offset"`
}

// RecordingsByArtist browses every recording MusicBrainz has linked to the
// artist identified by mbid, in ANY credit position (D-05) -- this is not
// pre-filtered to guest appearances. It returns the artist's own
// primary-credit catalogue too; internal/detection's isGuestFeature applies
// D-06's positional filter to every entry before any event is created
// (04-RESEARCH.md Common Pitfall #3).
//
// Pages are fetched sequentially (never concurrently) and bounded by
// maxRecordingPages (T-04-13), mirroring ReleaseGroupsByArtist's contract
// exactly: a malformed or hostile recording-count from upstream cannot drive
// an unbounded request loop, and issuing pages concurrently would multiply
// this client's instantaneous rate against MusicBrainz beyond the
// operator-configured limiter. Hitting the page ceiling returns the
// accumulated recordings and a nil error -- a truncated fetch is a
// data-completeness limit, not a failure; the caller is responsible for
// surfacing truncation (len(recordings) >= MaxRecordingBrowseItems) in its
// own structured logs.
func (c *Client) RecordingsByArtist(ctx context.Context, mbid string) ([]Recording, error) {
	trimmed := strings.TrimSpace(mbid)
	if trimmed == "" {
		return nil, ErrEmptyMBID
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	recordings := make([]Recording, 0, recordingPageSize)
	offset := 0
	for page := 0; page < maxRecordingPages; page++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		env, err := c.fetchRecordingPage(ctx, trimmed, offset)
		if err != nil {
			return nil, err
		}

		recordings = append(recordings, env.Recordings...)

		// Terminate as soon as either holds: we've collected everything
		// upstream reports (Count), or the page came back empty -- a
		// runaway/hostile Count must never keep the loop spinning past an
		// empty page (T-04-13).
		if len(env.Recordings) == 0 || len(recordings) >= env.Count {
			return recordings, nil
		}
		// Advance by the number of entries actually returned, not the
		// requested page size, so a short page can never skip records.
		offset += len(env.Recordings)
	}

	return recordings, nil
}

// fetchRecordingPage issues a single /recording page request at offset
// through the shared doRequest helper so no page request can bypass the
// limiter or User-Agent header (D-07, T-04-14). RecordingsByArtist wraps
// this in a bounded, sequential pagination loop -- pages are never fetched
// concurrently.
func (c *Client) fetchRecordingPage(ctx context.Context, mbid string, offset int) (recordingEnvelope, error) {
	u, err := url.Parse(c.baseURL + "/recording")
	if err != nil {
		return recordingEnvelope{}, fmt.Errorf("musicbrainz: parse base url: %w", err)
	}
	q := url.Values{}
	q.Set("artist", mbid)
	q.Set("inc", "artist-credits")
	q.Set("fmt", "json")
	q.Set("limit", strconv.Itoa(recordingPageSize))
	q.Set("offset", strconv.Itoa(offset))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return recordingEnvelope{}, fmt.Errorf("musicbrainz: build request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return recordingEnvelope{}, fmt.Errorf("musicbrainz: recordings by artist: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Never echo the response body -- only the status code, which is
		// operator-facing and carries no upstream text (T-04-15, V13).
		return recordingEnvelope{}, fmt.Errorf("musicbrainz: recordings by artist: unexpected status %d", resp.StatusCode)
	}

	var env recordingEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return recordingEnvelope{}, fmt.Errorf("musicbrainz: decode recording response: %w", err)
	}
	return env, nil
}
