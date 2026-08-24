// Package httpclient is the shared rate-limited request seam extracted from
// internal/musicbrainz and internal/deezer's formerly-duplicated doRequest
// helpers. Both packages independently implemented the same timeout-wrap +
// limiter.Wait + cancel-on-error + cancel-on-close body wrapping logic; this
// package consolidates that logic into one tested Do function so a future
// client (or a retry/backoff addition) has a single seam to change instead
// of two that must be kept in sync by hand.
package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/time/rate"
)

// Do performs a single rate-limited HTTP request: it waits on limiter before
// calling httpClient.Do so no call site can bypass the rate limit, wrapping
// component into every error it returns so each caller's wrapped errors keep
// naming themselves exactly as before (e.g. "musicbrainz: ...",
// "deezer: ...").
//
// When httpClient.Timeout is positive, ctx is wrapped in a
// context.WithTimeout bounded by it so the same budget also bounds
// limiter.Wait: a caller stuck behind a saturated limiter must fail within
// the same deadline as the request itself, not hang indefinitely on a ctx
// with no deadline of its own. A zero Timeout means "no client timeout" in
// net/http's own convention, so it is left unwrapped rather than fed to
// context.WithTimeout, which would treat a zero duration as "already
// expired" and fail every request immediately. The cancel func is not
// deferred here -- it is attached to the response body via a
// cancelReadCloser so the timeout keeps bounding the caller's body read
// after Do returns, and is released exactly once the caller closes the
// body.
//
// Do does not set any HTTP headers on req -- header construction stays each
// caller's responsibility, applied to req before calling Do (req.WithContext
// performs a shallow copy that shares the same Header map, so headers set
// beforehand survive).
func Do(ctx context.Context, req *http.Request, limiter *rate.Limiter, httpClient *http.Client, component string) (*http.Response, error) {
	cancel := func() {}
	if httpClient.Timeout > 0 {
		var timeoutCtx context.Context
		timeoutCtx, cancel = context.WithTimeout(ctx, httpClient.Timeout)
		ctx = timeoutCtx
	}
	req = req.WithContext(ctx)

	if err := limiter.Wait(ctx); err != nil {
		cancel()
		return nil, fmt.Errorf("%s: rate limiter wait: %w", component, err)
	}

	// #nosec G704 -- gosec's SSRF taint check flags this because Do is a
	// generic, exported function and cannot trace req's origin across
	// package boundaries. Do never builds or mutates req.URL itself; it
	// only forwards a request its caller already constructed. Both current
	// callers (internal/musicbrainz, internal/deezer) build req from a
	// package-const base URL that is never derived from external input
	// (T-03-07's "no request path redirected by caller-supplied data"
	// invariant, unchanged by this extraction) -- the same call shape gosec
	// does not flag in either package's own (now-removed) doRequest.
	resp, err := httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%s: do request: %w", component, err)
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

// cancelReadCloser wraps a response body so the context.CancelFunc created
// by Do's context.WithTimeout is released exactly when the caller closes
// the body -- calling cancel() any earlier (e.g. immediately after
// httpClient.Do returns, before the body is read) would truncate a
// perfectly healthy response.
type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
