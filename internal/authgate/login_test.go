package authgate

// Whitebox tests for plan 14-02's brute-force defense and audit layer: the
// per-IP login throttle, the fixed jittered delay, the maxConcurrentLogins
// shed path, the limiter-map sweeper, the process-wide failed-attempt counter,
// and the D-13 structured audit lines. These live in package authgate (not
// authgate_test) so they can drive loginThrottle / globalCounter directly and
// read m.counter after a run.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// --- shared whitebox test helpers (also used by alerter_test.go) ---

// syncBuf is a mutex-guarded buffer: dispatchAlert logs from its own goroutine
// so a plain bytes.Buffer would race the test reading it.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func wbLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// loginReq builds a POST /session request with the given JSON passphrase and
// RemoteAddr, the reliable way to exercise per-IP behaviour without a proxy.
func loginReq(t *testing.T, passphrase, remoteAddr string) *http.Request {
	t.Helper()
	body, err := json.Marshal(map[string]string{"passphrase": passphrase})
	if err != nil {
		t.Fatalf("marshal login body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/session", bytes.NewReader(body))
	r.RemoteAddr = remoteAddr
	return r
}

// recordingAlerter counts Alert calls and returns a configurable error.
type recordingAlerter struct {
	mu      sync.Mutex
	n       int
	lastMsg string
	err     error
}

func (a *recordingAlerter) Alert(_ context.Context, msg string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.n++
	a.lastMsg = msg
	return a.err
}

func (a *recordingAlerter) calls() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.n
}

func (a *recordingAlerter) message() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lastMsg
}

// --- Task 1: per-IP throttle boundary + refill ---

func TestLoginThrottle_BoundaryAndSecondAddressUnaffected(t *testing.T) {
	defer SetLoginDelayForTest(time.Millisecond, 0)()
	defer SetLoginBurstForTest(5)()
	defer SetLoginRateForTest(rate.Every(time.Hour))()

	m := NewManager("the-correct-instance-passphrase", nil, wbLogger())
	defer m.Close()

	// attempts 1..5 from one address are answered by the handler (401 here,
	// wrong passphrase) -- never 429.
	for i := 1; i <= 5; i++ {
		rec := httptest.NewRecorder()
		m.HandleLogin(rec, loginReq(t, "wrong", "198.51.100.7:44321"))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: code = %d, want 401", i, rec.Code)
		}
	}

	// attempt 6 from that address is throttled.
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", "198.51.100.7:44321"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 6: code = %d, want 429", rec.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode 429 body: %v", err)
	}
	if body.Error != "too many attempts" {
		t.Fatalf("429 error field = %q, want %q", body.Error, "too many attempts")
	}

	// a first attempt from a second address in the same window is not throttled.
	rec = httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", "203.0.113.42:5555"))
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("second address returned 429 -- limiter is not per-IP")
	}
}

func TestLoginThrottle_RefillRestoresService(t *testing.T) {
	defer SetLoginDelayForTest(time.Millisecond, 0)()
	defer SetLoginBurstForTest(1)()
	defer SetLoginRateForTest(rate.Every(25 * time.Millisecond))()

	m := NewManager("the-correct-instance-passphrase", nil, wbLogger())
	defer m.Close()

	const addr = "198.51.100.9:1111"
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", addr))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 1: code = %d, want 401", rec.Code)
	}
	rec = httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", addr))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 2: code = %d, want 429", rec.Code)
	}

	time.Sleep(40 * time.Millisecond) // let the bucket refill one token

	rec = httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", addr))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after refill: code = %d, want 401 (service restored)", rec.Code)
	}
}

// --- Task 1: fixed delay on comparison paths, none on 429 ---

func TestLoginDelay_OnComparisonPathButNotOn429(t *testing.T) {
	const shrunkMin = 60 * time.Millisecond
	defer SetLoginDelayForTest(shrunkMin, 0)()
	defer SetLoginBurstForTest(1)()
	defer SetLoginRateForTest(rate.Every(time.Hour))()

	m := NewManager("the-correct-instance-passphrase", nil, wbLogger())
	defer m.Close()

	// one comparison-path (wrong-passphrase 401) response is paced by >= min.
	start := time.Now()
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", "192.0.2.1:9"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", rec.Code)
	}
	if elapsed := time.Since(start); elapsed < shrunkMin {
		t.Fatalf("comparison-path response took %s, want >= %s", elapsed, shrunkMin)
	}

	// the next attempt from that address is throttled and returns well under
	// half the shrunk minimum -- it is never slept on.
	start = time.Now()
	rec = httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, "wrong", "192.0.2.1:9"))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("code = %d, want 429", rec.Code)
	}
	if elapsed := time.Since(start); elapsed >= shrunkMin/2 {
		t.Fatalf("429 response took %s, want < %s (must not be delayed)", elapsed, shrunkMin/2)
	}
}

func TestLoginDelay_AppliedOnSuccessPath(t *testing.T) {
	const shrunkMin = 60 * time.Millisecond
	defer SetLoginDelayForTest(shrunkMin, 0)()
	defer SetLoginBurstForTest(5)()
	defer SetLoginRateForTest(rate.Every(time.Hour))()

	const pass = "the-correct-instance-passphrase"
	m := NewManager(pass, nil, wbLogger())
	defer m.Close()

	start := time.Now()
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, pass, "192.0.2.5:9"))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204", rec.Code)
	}
	if elapsed := time.Since(start); elapsed < shrunkMin {
		t.Fatalf("success response took %s, want >= %s", elapsed, shrunkMin)
	}
}

// --- Task 1: maxConcurrentLogins shed path ---

func TestLoginConcurrency_ShedsExcessWith503(t *testing.T) {
	defer SetMaxConcurrentLoginsForTest(2)()
	defer SetLoginBurstForTest(100)()
	defer SetLoginRateForTest(rate.Every(time.Hour))()

	entered := make(chan struct{}, 8)
	release := make(chan struct{})
	defer SetLoginSleepForTest(func(time.Duration) {
		entered <- struct{}{}
		<-release
	})()

	const pass = "the-correct-instance-passphrase"
	m := NewManager(pass, nil, wbLogger())
	defer m.Close()

	// two in-flight logins occupy both slots and block in the fixed delay.
	for i := 0; i < 2; i++ {
		go func() {
			rec := httptest.NewRecorder()
			m.HandleLogin(rec, loginReq(t, pass, fmt.Sprintf("10.1.1.%d:1", i)))
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for two logins to occupy the semaphore")
		}
	}

	// a third simultaneous login sheds immediately with 503, no delay.
	start := time.Now()
	rec := httptest.NewRecorder()
	m.HandleLogin(rec, loginReq(t, pass, "10.1.1.9:1"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("third login: code = %d, want 503", rec.Code)
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("503 shed took %s, want immediate", elapsed)
	}
	var body struct {
		Error string `json:"error"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Error != "server busy" {
		t.Fatalf("503 error field = %q, want %q", body.Error, "server busy")
	}

	close(release)
}

// --- Task 1: limiter-map sweeper ---

func TestLimiterSweep_EvictsIdleEntries(t *testing.T) {
	defer SetLimiterIdleTTLForTest(50 * time.Millisecond)()

	th := newLoginThrottle()
	now := time.Now()
	th.getLimiter("1.1.1.1", now)
	th.getLimiter("2.2.2.2", now)
	if th.size() != 2 {
		t.Fatalf("map size = %d, want 2 after two distinct addresses", th.size())
	}

	th.sweep(now.Add(time.Minute)) // advance the clock past the idle TTL
	if th.size() != 0 {
		t.Fatalf("map size = %d after sweep, want 0 (idle entries evicted)", th.size())
	}
}

// --- Task 1: concurrent attempts from one address neither panic nor corrupt ---

func TestLoginThrottle_ParallelSameAddressConsistent(t *testing.T) {
	defer SetLoginDelayForTest(time.Millisecond, 0)()
	defer SetLoginBurstForTest(5)()
	defer SetLoginRateForTest(rate.Every(time.Hour))()

	m := NewManager("the-correct-instance-passphrase", nil, wbLogger())
	defer m.Close()

	const n = 40
	codes := make([]int, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			rec := httptest.NewRecorder()
			m.HandleLogin(rec, loginReq(t, "wrong", "198.51.100.200:7"))
			codes[idx] = rec.Code
		}(i)
	}
	wg.Wait()

	var allowed, throttled int
	for _, c := range codes {
		switch c {
		case http.StatusUnauthorized:
			allowed++
		case http.StatusTooManyRequests:
			throttled++
		default:
			t.Fatalf("unexpected status %d from a parallel attempt", c)
		}
	}
	if allowed+throttled != n {
		t.Fatalf("allowed(%d) + throttled(%d) != %d", allowed, throttled, n)
	}
	if allowed > 5 {
		t.Fatalf("allowed %d comparison-path responses, want <= burst 5", allowed)
	}
}
