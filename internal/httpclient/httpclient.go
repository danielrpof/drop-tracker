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
	"errors"
	"net/http"

	"golang.org/x/time/rate"
)

// Do is a RED-phase placeholder -- not yet implemented. See
// httpclient_test.go for the behavior this must satisfy.
func Do(ctx context.Context, req *http.Request, limiter *rate.Limiter, httpClient *http.Client, component string) (*http.Response, error) {
	return nil, errors.New("httpclient: Do not yet implemented")
}
