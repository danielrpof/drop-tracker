package httpclient

// This file is package httpclient (whitebox), mirroring
// internal/musicbrainz/search_test.go's style: httptest.NewServer, a
// rate.NewLimiter(rate.Inf, 1) helper for non-limiter-focused cases, and
// errors.Is assertions.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// unlimitedLimiter gives tests that aren't exercising rate-limiting itself
// a limiter that never blocks.
func unlimitedLimiter() *rate.Limiter {
	return rate.NewLimiter(rate.Inf, 1)
}

func TestDo_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	resp, err := Do(context.Background(), req, unlimitedLimiter(), ts.Client(), "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	if resp == nil {
		t.Fatal("resp is nil, want non-nil")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	buf := make([]byte, 2)
	n, err := resp.Body.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("resp.Body.Read: %v", err)
	}
	if string(buf[:n]) != "ok" {
		t.Fatalf("body = %q, want %q", string(buf[:n]), "ok")
	}
}

func TestDo_LimiterWaitErrorOnCancelledContext(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := Do(ctx, req, unlimitedLimiter(), ts.Client(), "testcomp")
	if err == nil {
		t.Fatal("Do: got nil error, want a wrapped context.Canceled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do error = %v, want it to wrap context.Canceled", err)
	}
	if resp != nil {
		t.Fatalf("resp = %v, want nil", resp)
	}

	mu.Lock()
	got := requestCount
	mu.Unlock()
	if got != 0 {
		t.Fatalf("requestCount = %d, want 0 (a cancelled context must not reach the server)", got)
	}
}

func TestDo_TimeoutReturnsWrappedDeadlineExceeded(t *testing.T) {
	block := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block // never responds within the test's lifetime
	}))
	defer func() {
		close(block)
		ts.Close()
	}()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	httpClient := &http.Client{
		Timeout:   200 * time.Millisecond,
		Transport: ts.Client().Transport,
	}

	start := time.Now()
	resp, err := Do(context.Background(), req, unlimitedLimiter(), httpClient, "testcomp")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Do: got nil error, want a timeout error")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("elapsed = %v, want Do to return promptly rather than block indefinitely", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Do error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if resp != nil {
		t.Fatalf("resp = %v, want nil", resp)
	}
}

// capturingRoundTripper wraps another RoundTripper and records the *last*
// *http.Request's Context() it receives, so a test can inspect that
// captured context's Err() before and after the returned response's
// Body.Close() is called.
type capturingRoundTripper struct {
	base    http.RoundTripper
	mu      sync.Mutex
	lastCtx context.Context
}

func (c *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	c.lastCtx = req.Context()
	c.mu.Unlock()
	return c.base.RoundTrip(req)
}

func (c *capturingRoundTripper) capturedContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastCtx
}

func TestDo_CloseCancelsTheDerivedContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	rt := &capturingRoundTripper{base: ts.Client().Transport}
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: rt,
	}

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}

	resp, err := Do(context.Background(), req, unlimitedLimiter(), httpClient, "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}

	captured := rt.capturedContext()
	if captured == nil {
		t.Fatal("capturedContext is nil, want the derived request context")
	}
	if captured.Err() != nil {
		t.Fatalf("captured.Err() = %v, want nil immediately after Do returns", captured.Err())
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("resp.Body.Close: %v", err)
	}

	if !errors.Is(captured.Err(), context.Canceled) {
		t.Fatalf("captured.Err() after Close = %v, want context.Canceled", captured.Err())
	}
}
