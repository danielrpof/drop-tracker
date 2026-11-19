package httpclient

// This file is package httpclient (whitebox), mirroring
// internal/musicbrainz/search_test.go's style: httptest.NewServer, a
// rate.NewLimiter(rate.Inf, 1) helper for non-limiter-focused cases, and
// errors.Is assertions.

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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

	resp, err := Do(context.Background(), req, unlimitedLimiter(), ts.Client(), Policy{}, "testcomp")
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

	resp, err := Do(ctx, req, unlimitedLimiter(), ts.Client(), Policy{}, "testcomp")
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
	resp, err := Do(context.Background(), req, unlimitedLimiter(), httpClient, newTestPolicy(&sleepRecorder{}), "testcomp")
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

	resp, err := Do(context.Background(), req, unlimitedLimiter(), httpClient, Policy{}, "testcomp")
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

// sleepRecorder replaces real backoff sleeps and records each requested delay.
type sleepRecorder struct {
	mu     sync.Mutex
	delays []time.Duration
}

func (r *sleepRecorder) sleep(ctx context.Context, d time.Duration) error {
	r.mu.Lock()
	r.delays = append(r.delays, d)
	r.mu.Unlock()
	return ctx.Err()
}

func (r *sleepRecorder) recorded() []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Duration(nil), r.delays...)
}

func newTestPolicy(rec *sleepRecorder) Policy {
	return Policy{
		MaxAttempts: 3,
		BaseBackoff: time.Second,
		sleep:       rec.sleep,
		jitter:      func(d time.Duration) time.Duration { return d },
	}
}

// statusServer answers with statuses in order, repeating the last one, and
// counts hits.
func statusServer(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(hits.Add(1)) - 1
		if i >= len(statuses) {
			i = len(statuses) - 1
		}
		w.WriteHeader(statuses[i])
		if statuses[i] == http.StatusOK {
			_, _ = w.Write([]byte("ok"))
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &hits
}

type rtFunc func(*http.Request) (*http.Response, error)

func (f rtFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func stubResponse(status int) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
}

func newReq(t *testing.T, method, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("http.NewRequest: %v", err)
	}
	return req
}

func TestDo_RetriesTransientStatus(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		statuses   []int
		wantHits   int32
		wantDelays []time.Duration
	}{
		{"503 503 200", http.MethodGet, []int{503, 503, 200}, 3, []time.Duration{time.Second, 2 * time.Second}},
		{"429 200", http.MethodGet, []int{429, 200}, 2, []time.Duration{time.Second}},
		{"head 503 200", http.MethodHead, []int{503, 200}, 2, []time.Duration{time.Second}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts, hits := statusServer(t, tt.statuses...)
			rec := &sleepRecorder{}
			resp, err := Do(context.Background(), newReq(t, tt.method, ts.URL), unlimitedLimiter(), ts.Client(), newTestPolicy(rec), "testcomp")
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("StatusCode = %d, want 200", resp.StatusCode)
			}
			if tt.method == http.MethodGet {
				body, err := io.ReadAll(resp.Body)
				if err != nil || string(body) != "ok" {
					t.Fatalf("body = %q, err = %v; want %q", body, err, "ok")
				}
			}
			if got := hits.Load(); got != tt.wantHits {
				t.Errorf("hits = %d, want %d", got, tt.wantHits)
			}
			if got := rec.recorded(); !reflect.DeepEqual(got, tt.wantDelays) {
				t.Errorf("delays = %v, want %v", got, tt.wantDelays)
			}
		})
	}
}

func TestDo_ExhaustedRetriesReturnFinalResponse(t *testing.T) {
	ts, hits := statusServer(t, 503)
	rec := &sleepRecorder{}
	resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), newTestPolicy(rec), "testcomp")
	if err != nil {
		t.Fatalf("Do: got error %v, want the final 503 response with a nil error", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", resp.StatusCode)
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("hits = %d, want 3", got)
	}
	if got, want := rec.recorded(), []time.Duration{time.Second, 2 * time.Second}; !reflect.DeepEqual(got, want) {
		t.Errorf("delays = %v, want %v", got, want)
	}
}

func TestDo_NoRetryOnPermanentStatus(t *testing.T) {
	for _, status := range []int{400, 404, 500} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			ts, hits := statusServer(t, status)
			rec := &sleepRecorder{}
			resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), newTestPolicy(rec), "testcomp")
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != status {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, status)
			}
			if got := hits.Load(); got != 1 {
				t.Errorf("hits = %d, want 1", got)
			}
			if got := rec.recorded(); len(got) != 0 {
				t.Errorf("delays = %v, want none", got)
			}
		})
	}
}

func TestDo_NoRetryForNonIdempotentMethod(t *testing.T) {
	ts, hits := statusServer(t, 503)
	resp, err := Do(context.Background(), newReq(t, http.MethodPost, ts.URL), unlimitedLimiter(), ts.Client(), newTestPolicy(&sleepRecorder{}), "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := hits.Load(); got != 1 {
		t.Errorf("hits = %d, want 1", got)
	}
}

func TestDo_RetriesTransportErrors(t *testing.T) {
	errBoom := errors.New("boom")

	t.Run("recovers", func(t *testing.T) {
		var calls atomic.Int32
		client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
			if calls.Add(1) <= 2 {
				return nil, errBoom
			}
			return stubResponse(200), nil
		})}
		resp, err := Do(context.Background(), newReq(t, http.MethodGet, "http://example.invalid/"), unlimitedLimiter(), client, newTestPolicy(&sleepRecorder{}), "testcomp")
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		_ = resp.Body.Close()
		if got := calls.Load(); got != 3 {
			t.Errorf("calls = %d, want 3", got)
		}
	})

	t.Run("always fails", func(t *testing.T) {
		var calls atomic.Int32
		rec := &sleepRecorder{}
		client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errBoom
		})}
		resp, err := Do(context.Background(), newReq(t, http.MethodGet, "http://example.invalid/"), unlimitedLimiter(), client, newTestPolicy(rec), "testcomp")
		if resp != nil {
			t.Fatalf("resp = %v, want nil", resp)
		}
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want it to wrap the transport error", err)
		}
		if !strings.HasPrefix(err.Error(), "testcomp: do request:") {
			t.Errorf("err = %q, want prefix %q", err.Error(), "testcomp: do request:")
		}
		if got := calls.Load(); got != 3 {
			t.Errorf("calls = %d, want 3", got)
		}
		if got := len(rec.recorded()); got != 2 {
			t.Errorf("delays = %d, want 2", got)
		}
	})
}

type trackingBody struct {
	r      io.Reader
	eof    atomic.Bool
	closed atomic.Bool
}

func (b *trackingBody) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if errors.Is(err, io.EOF) {
		b.eof.Store(true)
	}
	return n, err
}

func (b *trackingBody) Close() error {
	b.closed.Store(true)
	return nil
}

func TestDo_DrainsAndClosesRetriedBody(t *testing.T) {
	failed := &trackingBody{r: strings.NewReader("busy")}
	var calls atomic.Int32
	var drainedBeforeRetry, closedBeforeRetry bool
	client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: failed}, nil
		}
		drainedBeforeRetry, closedBeforeRetry = failed.eof.Load(), failed.closed.Load()
		return stubResponse(200), nil
	})}
	resp, err := Do(context.Background(), newReq(t, http.MethodGet, "http://example.invalid/"), unlimitedLimiter(), client, newTestPolicy(&sleepRecorder{}), "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if !drainedBeforeRetry {
		t.Error("503 body was not read to EOF before the retry")
	}
	if !closedBeforeRetry {
		t.Error("503 body was not closed before the retry")
	}
}

func TestDo_WaitsOnLimiterEveryAttempt(t *testing.T) {
	ts, _ := statusServer(t, 503, 503, 200)
	limiter := rate.NewLimiter(rate.Every(time.Hour), 3)
	resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), limiter, ts.Client(), newTestPolicy(&sleepRecorder{}), "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if got := limiter.Tokens(); got >= 0.5 {
		t.Errorf("tokens after 3 attempts = %v, want < 0.5", got)
	}

	ctlServer, _ := statusServer(t, 200)
	ctl := rate.NewLimiter(rate.Every(time.Hour), 3)
	resp, err = Do(context.Background(), newReq(t, http.MethodGet, ctlServer.URL), ctl, ctlServer.Client(), newTestPolicy(&sleepRecorder{}), "testcomp")
	if err != nil {
		t.Fatalf("Do (control): %v", err)
	}
	_ = resp.Body.Close()
	if got := ctl.Tokens(); got < 1.5 || got > 2.5 {
		t.Errorf("control tokens after 1 attempt = %v, want ~2", got)
	}
}

func TestDo_EachAttemptGetsItsOwnTimeout(t *testing.T) {
	var hits atomic.Int32
	block := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			<-block
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer func() {
		close(block)
		ts.Close()
	}()

	client := &http.Client{Timeout: 150 * time.Millisecond, Transport: ts.Client().Transport}
	resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), client, newTestPolicy(&sleepRecorder{}), "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if got := hits.Load(); got != 2 {
		t.Errorf("hits = %d, want 2", got)
	}
}

func TestDo_NoRetryAfterCallerCancel(t *testing.T) {
	t.Run("during request", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var calls atomic.Int32
		rec := &sleepRecorder{}
		client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			cancel()
			return stubResponse(503), nil
		})}
		resp, err := Do(ctx, newReq(t, http.MethodGet, "http://example.invalid/"), unlimitedLimiter(), client, newTestPolicy(rec), "testcomp")
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != 503 {
			t.Errorf("StatusCode = %d, want 503", resp.StatusCode)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("calls = %d, want 1", got)
		}
		if got := rec.recorded(); len(got) != 0 {
			t.Errorf("sleep called with %v, want never", got)
		}
	})

	t.Run("during backoff", func(t *testing.T) {
		ts, hits := statusServer(t, 503)
		policy := newTestPolicy(&sleepRecorder{})
		policy.sleep = func(context.Context, time.Duration) error { return context.Canceled }
		resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), policy, "testcomp")
		if resp != nil {
			t.Fatalf("resp = %v, want nil", resp)
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want it to wrap context.Canceled", err)
		}
		if !strings.HasPrefix(err.Error(), "testcomp: ") {
			t.Errorf("err = %q, want prefix %q", err.Error(), "testcomp: ")
		}
		if got := hits.Load(); got != 1 {
			t.Errorf("hits = %d, want 1", got)
		}
	})
}

func TestDefaultJitter_WithinTwentyPercent(t *testing.T) {
	for i := 0; i < 1000; i++ {
		got := defaultJitter(time.Second)
		if got < 800*time.Millisecond || got > 1200*time.Millisecond {
			t.Fatalf("jitter(1s) = %v, want within [800ms, 1200ms]", got)
		}
	}
	if got := defaultJitter(0); got != 0 {
		t.Errorf("jitter(0) = %v, want 0", got)
	}
}

func TestDefaultSleep(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := defaultSleep(cancelled, time.Hour); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled sleep err = %v, want context.Canceled", err)
	}
	if time.Since(start) > time.Second {
		t.Error("cancelled sleep did not return promptly")
	}
	if err := defaultSleep(context.Background(), time.Millisecond); err != nil {
		t.Errorf("live sleep err = %v, want nil", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in     string
		want   time.Duration
		wantOK bool
	}{
		{"3", 3 * time.Second, true},
		{"0", 0, true},
		{"-1", 0, false},
		{"soon", 0, false},
		{"", 0, false},
		{now.Add(4 * time.Second).Format(http.TimeFormat), 4 * time.Second, true},
		{now.Add(-time.Hour).Format(http.TimeFormat), 0, true},
	}
	for _, tt := range tests {
		got, ok := parseRetryAfter(tt.in, now)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("parseRetryAfter(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestDo_HonoursRetryAfter(t *testing.T) {
	tests := []struct {
		name          string
		header        string
		maxRetryAfter time.Duration
		min, max      time.Duration
	}{
		{"seconds", "3", 5 * time.Second, 3 * time.Second, 3 * time.Second},
		{"capped", "120", 5 * time.Second, 5 * time.Second, 5 * time.Second},
		{"http-date", time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat), 10 * time.Second, 2 * time.Second, 5 * time.Second},
		{"unparseable falls back to backoff", "soon", 5 * time.Second, time.Second, time.Second},
		{"zero cap ignores header", "3", 0, time.Second, time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits atomic.Int32
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if hits.Add(1) == 1 {
					w.Header().Set("Retry-After", tt.header)
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				_, _ = w.Write([]byte("ok"))
			}))
			defer ts.Close()

			rec := &sleepRecorder{}
			policy := newTestPolicy(rec)
			policy.MaxRetryAfter = tt.maxRetryAfter
			policy.MinAttemptBudget = 2 * time.Second
			resp, err := Do(context.Background(), newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), policy, "testcomp")
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			_ = resp.Body.Close()
			if got := hits.Load(); got != 2 {
				t.Errorf("hits = %d, want 2", got)
			}
			delays := rec.recorded()
			if len(delays) != 1 || delays[0] < tt.min || delays[0] > tt.max {
				t.Errorf("delays = %v, want one delay in [%v, %v]", delays, tt.min, tt.max)
			}
		})
	}
}

func TestDo_SkipsRetryWhenDeadlineTooShort(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		var hits atomic.Int32
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("busy"))
		}))
		defer ts.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		rec := &sleepRecorder{}
		policy := newTestPolicy(rec)
		policy.MinAttemptBudget = 2 * time.Second
		resp, err := Do(ctx, newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), policy, "testcomp")
		if err != nil {
			t.Fatalf("Do: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("StatusCode = %d, want 503", resp.StatusCode)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "busy" {
			t.Errorf("body = %q, err = %v; want %q (undrained)", body, err, "busy")
		}
		if got := hits.Load(); got != 1 {
			t.Errorf("hits = %d, want 1", got)
		}
		if got := rec.recorded(); len(got) != 0 {
			t.Errorf("delays = %v, want none", got)
		}
	})

	t.Run("transport", func(t *testing.T) {
		errBoom := errors.New("boom")
		var calls atomic.Int32
		client := &http.Client{Transport: rtFunc(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			return nil, errBoom
		})}
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		rec := &sleepRecorder{}
		_, err := Do(ctx, newReq(t, http.MethodGet, "http://example.invalid/"), unlimitedLimiter(), client, newTestPolicy(rec), "testcomp")
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v, want it to wrap the transport error", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("calls = %d, want 1", got)
		}
		if got := rec.recorded(); len(got) != 0 {
			t.Errorf("delays = %v, want none", got)
		}
	})
}

func TestDo_RetriesWhenDeadlineAllows(t *testing.T) {
	ts, hits := statusServer(t, 503, 200)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	policy := newTestPolicy(&sleepRecorder{})
	policy.MinAttemptBudget = 2 * time.Second
	resp, err := Do(ctx, newReq(t, http.MethodGet, ts.URL), unlimitedLimiter(), ts.Client(), policy, "testcomp")
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("hits = %d, want 2", got)
	}
}
