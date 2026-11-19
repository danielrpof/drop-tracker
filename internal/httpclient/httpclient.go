// Package httpclient is the shared rate-limited, retrying request seam for
// internal/musicbrainz and internal/deezer, so limiting, timeouts and retry
// policy live in one tested place (#17).
package httpclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

// maxDrainBytes caps how much of a retried response body is read so a large
// error page can't stall the retry.
const maxDrainBytes = 64 << 10

// Policy configures Do's retries. The zero Policy makes a single attempt.
// MaxRetryAfter caps a server's Retry-After (0 ignores the header);
// MinAttemptBudget is the caller-deadline headroom a retry needs beyond its
// delay. sleep and jitter are test hooks; nil selects the real timer and
// +/-20% jitter.
type Policy struct {
	MaxAttempts      int
	BaseBackoff      time.Duration
	MaxRetryAfter    time.Duration
	MinAttemptBudget time.Duration

	sleep  func(context.Context, time.Duration) error
	jitter func(time.Duration) time.Duration
}

// DefaultPolicy returns the production policy: 3 attempts, ~1s then ~2s
// backoff, Retry-After capped at 5s (#17). A constructor, not a var, so
// nothing can mutate a shared default.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts:      3,
		BaseBackoff:      time.Second,
		MaxRetryAfter:    5 * time.Second,
		MinAttemptBudget: 2 * time.Second,
	}
}

// Do performs a rate-limited HTTP request, retrying GET/HEAD on transport
// errors, 503 and 429 per policy. Exhausted retries return the final response
// with a nil error so callers' status checks keep their own messages. A retry
// is skipped when ctx's deadline can't fit its delay plus MinAttemptBudget.
// Every error is prefixed with component (e.g. "musicbrainz: ...").
//
// Each attempt waits on limiter and runs under its own httpClient.Timeout;
// the final response's timeout is released when the caller closes its body.
// Do sets no headers: callers prepare req beforehand.
func Do(ctx context.Context, req *http.Request, limiter *rate.Limiter, httpClient *http.Client, policy Policy, component string) (*http.Response, error) {
	attempts := 1
	if idempotent(req.Method) && policy.MaxAttempts > 1 {
		attempts = policy.MaxAttempts
	}
	sleep, jitter := policy.sleep, policy.jitter
	if sleep == nil {
		sleep = defaultSleep
	}
	if jitter == nil {
		jitter = defaultJitter
	}

	for n := 1; ; n++ {
		resp, err := attempt(ctx, req, limiter, httpClient)
		var we *waitError
		if errors.As(err, &we) {
			return nil, fmt.Errorf("%s: rate limiter wait: %w", component, we.err)
		}

		throttled := err == nil && (resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests)
		if (err == nil && !throttled) || n >= attempts || ctx.Err() != nil {
			return finish(resp, err, component)
		}

		delay := jitter(policy.BaseBackoff << (n - 1))
		if throttled && policy.MaxRetryAfter > 0 {
			if ra, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
				delay = max(delay, min(ra, policy.MaxRetryAfter))
			}
		}
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) < delay+policy.MinAttemptBudget {
			return finish(resp, err, component)
		}

		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
			_ = resp.Body.Close()
		}
		if err := sleep(ctx, delay); err != nil {
			return nil, fmt.Errorf("%s: retry backoff: %w", component, err)
		}
	}
}

func finish(resp *http.Response, err error, component string) (*http.Response, error) {
	if err != nil {
		return nil, fmt.Errorf("%s: do request: %w", component, err)
	}
	return resp, nil
}

// parseRetryAfter reads delta-seconds or an HTTP-date; a past date is 0.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		// Clamp so the Duration multiply below can't overflow.
		return time.Duration(min(secs, 365*24*3600)) * time.Second, true
	}
	t, err := http.ParseTime(v)
	if err != nil {
		return 0, false
	}
	return max(t.Sub(now), 0), true
}

func idempotent(method string) bool {
	return method == "" || method == http.MethodGet || method == http.MethodHead
}

// waitError marks a limiter failure, which Do reports without retrying: the
// caller's time budget failed, not the upstream.
type waitError struct{ err error }

func (e *waitError) Error() string { return e.err.Error() }
func (e *waitError) Unwrap() error { return e.err }

// attempt makes one request. A positive httpClient.Timeout bounds the limiter
// wait and the request together, and keeps bounding the caller's body read
// via cancelReadCloser; a zero Timeout is left unwrapped because
// context.WithTimeout would treat it as already expired.
func attempt(ctx context.Context, req *http.Request, limiter *rate.Limiter, httpClient *http.Client) (*http.Response, error) {
	cancel := func() {}
	if httpClient.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, httpClient.Timeout)
	}

	if err := limiter.Wait(ctx); err != nil {
		cancel()
		return nil, &waitError{err: err}
	}

	// #nosec G704 -- callers build req from package-const base URLs and Do
	// never alters req.URL (T-03-07).
	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = &cancelReadCloser{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// defaultJitter spreads d uniformly within +/-20% so concurrent callers
// don't retry in lockstep.
func defaultJitter(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	span := int64(d) / 5
	// #nosec G404 -- retry pacing, not a security primitive.
	return d + time.Duration(rand.Int64N(2*span+1)-span)
}

// cancelReadCloser wraps a response body so the context.CancelFunc created
// by attempt's context.WithTimeout is released exactly when the caller closes
// the body -- calling cancel() any earlier would truncate a healthy response.
type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
