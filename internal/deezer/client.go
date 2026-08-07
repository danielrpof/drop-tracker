// Package deezer is a hand-rolled, rate-limited client for Deezer's public
// JSON API. Like internal/musicbrainz (plan 03-01), it is a thin net/http
// wrapper rather than a generated or community client per CLAUDE.md's
// "hand-rolled clients" constraint. Unlike MusicBrainz, Deezer imposes no
// User-Agent requirement and needs no auth for the search/catalog endpoints
// this package uses (D-11-equivalent, D-12), so Client carries no user-agent
// field.
package deezer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

const (
	// defaultBaseURL is the only production base URL this client ever talks
	// to -- a package const, never derived from caller input, so no request
	// path can be redirected by user-supplied data (mirrors T-03-07).
	defaultBaseURL = "https://api.deezer.com"

	// defaultTimeout bounds every outbound request via httpClient.Timeout,
	// matching the MusicBrainz client's budget (03-RESEARCH.md Open
	// Question 2, T-03-03b).
	defaultTimeout = 10 * time.Second

	// maxLimit and defaultLimit bound the "limit" query parameter via
	// clampLimit so no caller can request an out-of-range value.
	maxLimit     = 100
	defaultLimit = 25
)

// ErrEmptyQuery is returned when SearchArtists is called with an
// empty/whitespace-only query, before any outbound request is built.
var ErrEmptyQuery = errors.New("deezer: empty search query")

// ErrEmptyArtistID is returned when ArtistAlbums is called with an
// empty/whitespace-only artist id, before any outbound request is built.
// A nil watchlist.Entry.DeezerID is D-06's explicit skip case and must never
// reach URL construction (D-06 pitfall 3, T-03-10).
var ErrEmptyArtistID = errors.New("deezer: empty artist id")

// Client is a rate-limited wrapper around Deezer's public JSON API. baseURL
// is unexported and settable only by this package's own tests (search_test.go,
// albums_test.go point it at an httptest.Server) -- production callers
// always get defaultBaseURL via NewClient.
type Client struct {
	baseURL    string
	httpClient *http.Client
	limiter    *rate.Limiter
}

// NewClient builds a Client that paces outbound calls through limiter
// (D-07, D-08). httpClient defaults to a client with a defaultTimeout
// timeout when nil.
func NewClient(limiter *rate.Limiter, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{
		baseURL:    defaultBaseURL,
		httpClient: httpClient,
		limiter:    limiter,
	}
}

// ArtistSearcher is the narrow seam internal/httpserver's search handler
// depends on, mirroring musicbrainz.ArtistSearcher: consumers depend on this
// interface, never on *Client directly.
type ArtistSearcher interface {
	SearchArtists(ctx context.Context, query string, limit int) ([]Artist, error)
}

var _ ArtistSearcher = (*Client)(nil)

// APIError represents Deezer's in-body error envelope, returned with an
// HTTP 200 status on a quota breach or other API-level failure
// (03-RESEARCH.md pitfall 2, assumption A1) -- a client that only checks the
// HTTP status code would silently treat this as a successful empty result.
type APIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Error renders every field of the Deezer error envelope so a caller (or a
// log line) sees the full upstream diagnostic without needing to unwrap.
func (e *APIError) Error() string {
	return fmt.Sprintf("deezer: api error type=%s code=%d message=%s", e.Type, e.Code, e.Message)
}

// errorProbe is decoded first by decodeChecked to detect Deezer's
// HTTP-200-with-in-body-error shape before attempting to decode into the
// caller's real success type.
type errorProbe struct {
	Error *APIError `json:"error"`
}

// decodeChecked unmarshals body into an errorProbe first: if Deezer signaled
// an in-body error, that error is returned and v is left untouched -- an
// HTTP-200 quota response can never silently decode into a zero-valued
// Artist/Album. Only when no in-body error is present does it unmarshal into
// v. Assumption A1 (the exact quota-error shape) is community-sourced, so
// this check runs unconditionally alongside the HTTP status check every call
// site already performs.
func decodeChecked(body []byte, v any) error {
	var probe errorProbe
	if err := json.Unmarshal(body, &probe); err != nil {
		return fmt.Errorf("deezer: decode error probe: %w", err)
	}
	if probe.Error != nil {
		return probe.Error
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("deezer: decode response: %w", err)
	}
	return nil
}

// doRequest is the single request path for this package: it waits on
// limiter before every call so no call site can bypass the rate limit
// (D-07, D-08), sets Accept, then performs the request. Every outbound
// Deezer request in this package -- and in plan 03-04's poll cycle -- must
// go through this one helper.
//
// When httpClient.Timeout is positive, ctx is wrapped in a
// context.WithTimeout bounded by it so the same budget also bounds
// limiter.Wait, mirroring internal/musicbrainz's doRequest. The cancel func
// is attached to the response body via cancelReadCloser rather than
// deferred, so the timeout keeps bounding the caller's body read after
// doRequest returns instead of truncating a healthy response.
func (c *Client) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	cancel := func() {}
	if c.httpClient.Timeout > 0 {
		var timeoutCtx context.Context
		timeoutCtx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		ctx = timeoutCtx
	}
	req = req.WithContext(ctx)

	if err := c.limiter.Wait(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("deezer: rate limiter wait: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("deezer: do request: %w", err)
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelReadCloser wraps a response body so the context.CancelFunc created
// by doRequest's context.WithTimeout is released exactly when the caller
// closes the body -- mirrors internal/musicbrainz's cancelReadCloser.
type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

// clampLimit maps a caller-requested limit onto this package's valid range:
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
