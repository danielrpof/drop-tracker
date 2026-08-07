// Package musicbrainz is a hand-rolled, rate-limited client for
// MusicBrainz's ws/2 JSON API. It exists as a thin net/http wrapper rather
// than a generated or community client per CLAUDE.md's "hand-rolled
// clients" constraint -- the surface this project needs (artist search in
// this plan, release-groups browse in a later plan) is small enough that a
// typed, testable wrapper is simpler than adopting an external client's
// broader abstractions.
package musicbrainz

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultBaseURL is the only production base URL this client ever
	// talks to -- it is a package const, never derived from caller input,
	// so no request path can be redirected by user-supplied data (T-03-07).
	defaultBaseURL = "https://musicbrainz.org/ws/2"

	// defaultTimeout bounds every outbound request via httpClient.Timeout:
	// long enough that a briefly-slow MusicBrainz response does not
	// spuriously fail a request, short enough that a hung upstream cannot
	// stall the sequential poll loop plan 03-04 introduces
	// (03-RESEARCH.md Open Question 2, T-03-03).
	defaultTimeout = 10 * time.Second

	// maxLimit is MusicBrainz's documented ceiling for the "limit" query
	// parameter; defaultLimit is its documented default. Both are enforced
	// by clampLimit so no caller can request an out-of-range value.
	maxLimit     = 100
	defaultLimit = 25
)

// Client is a rate-limited, User-Agent-identified wrapper around
// MusicBrainz's ws/2 JSON API. baseURL is unexported and settable only by
// this package's own tests (search_test.go points it at an
// httptest.Server) -- production callers always get defaultBaseURL via
// NewClient, with no exported way to redirect requests elsewhere (T-03-07).
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	limiter    *rate.Limiter
}

// NewClient builds a Client that identifies itself with userAgent on every
// request -- MusicBrainz keys its throttling off User-Agent rather than an
// API key, and a missing/generic UA drops this process into MusicBrainz's
// shared anonymous throttle pool (CLAUDE.md, reproduced in
// 03-RESEARCH.md's Pitfall 1) -- and paces outbound calls through limiter
// (D-07). httpClient defaults to a client with a defaultTimeout timeout
// when nil.
func NewClient(userAgent string, limiter *rate.Limiter, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    defaultBaseURL,
		userAgent:  userAgent,
		httpClient: httpClient,
		limiter:    limiter,
	}
}

// ArtistSearcher is the narrow seam internal/httpserver's search handler
// depends on, mirroring httpserver.Pinger and watchlist.Store (D-11):
// consumers depend on this interface, never on *Client directly, so a test
// can substitute a stub with no real HTTP client.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error)
}

var _ ArtistSearcher = (*Client)(nil)

// doRequest is the single request path for this package: it waits on
// limiter before every call so no call site can bypass the rate limit
// (D-07, T-03-08), then sets the identifying headers MusicBrainz's
// throttling keys off of (T-03-04), then performs the request. Every
// outbound MusicBrainz request in this package -- and in plan 03-03's
// release-groups browse -- must go through this one helper.
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("musicbrainz: rate limiter wait: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	return c.httpClient.Do(req)
}

// clampLimit maps a caller-requested limit onto MusicBrainz's valid range:
// non-positive becomes defaultLimit, anything above maxLimit is capped to
// it.
func clampLimit(limit int) int {
	if limit <= 0 {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
