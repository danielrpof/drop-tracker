package httpserver_test

// This file proves OPS-02: every response carries an X-Request-Id that
// matches the ID in that request's structured JSON log line, inbound IDs are
// honoured rather than overridden, concurrent requests never share an ID,
// and no captured log line carries the configured DSN.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
)

// syncBuffer is a mutex-guarded io.Writer. httplog.RequestLogger writes from
// the goroutine serving each request, so a plain bytes.Buffer would race
// under the concurrent test below.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// requestIDsInLog decodes each captured JSON log line and returns the
// request_id field's value, if present. Decoding into a map and reading the
// field by name (rather than substring-matching the raw line) means a
// change to the log schema's field names would surface as a missing value
// here instead of silently passing.
func requestIDsInLog(t *testing.T, logOutput string) []string {
	t.Helper()

	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(logOutput), "\n") {
		if line == "" {
			continue
		}
		var fields map[string]any
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if id, ok := fields["request_id"].(string); ok && id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// newCapturingServer builds an httpserver.Server backed by a no-op stub
// Pinger and a logger writing JSON to buf, so callers can serve requests and
// then inspect exactly what was logged for them.
func newCapturingServer(t *testing.T, buf *syncBuffer) *httpserver.Server {
	t.Helper()

	t.Setenv("DATABASE_URL", "postgres://placeholder:placeholder@localhost:5432/placeholder?sslmode=disable")
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	logger := logging.NewWithWriter(cfg, buf)
	stub := stubPinger{pingFunc: func(context.Context) error { return nil }}
	return httpserver.New(stub, stubStore{}, stubEventsStore{}, nil, logger)
}

func TestRequestID(t *testing.T) {
	buf := &syncBuffer{}
	srv := newCapturingServer(t, buf)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	id := resp.Header.Get("X-Request-Id")
	if id == "" {
		t.Fatal("X-Request-Id response header is empty")
	}

	logged := requestIDsInLog(t, buf.String())
	for _, l := range logged {
		if l == id {
			return
		}
	}
	t.Fatalf("request_id %q from X-Request-Id header not found in captured log output: %v", id, logged)
}

func TestRequestID_HonoursInboundHeader(t *testing.T) {
	buf := &syncBuffer{}
	srv := newCapturingServer(t, buf)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const inboundID = "test-inbound-id-0001"

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("X-Request-Id", inboundID)

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if got := resp.Header.Get("X-Request-Id"); got != inboundID {
		t.Fatalf("X-Request-Id = %q, want echoed inbound id %q", got, inboundID)
	}
}

func TestRequestID_DistinctPerConcurrentRequest(t *testing.T) {
	buf := &syncBuffer{}
	srv := newCapturingServer(t, buf)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const n = 10
	ids := make([]string, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()

			resp, err := http.Get(ts.URL + "/health")
			if err != nil {
				t.Errorf("request %d: %v", idx, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()
			ids[idx] = resp.Header.Get("X-Request-Id")
		}(i)
	}
	wg.Wait()

	issued := make(map[string]int, n)
	for i, id := range ids {
		if id == "" {
			t.Fatalf("request %d: empty X-Request-Id", i)
		}
		issued[id]++
	}
	if len(issued) != n {
		t.Fatalf("got %d distinct ids from %d requests, want %d distinct ids", len(issued), n, n)
	}

	logCounts := make(map[string]int)
	for _, id := range requestIDsInLog(t, buf.String()) {
		logCounts[id]++
	}
	for id, requestCount := range issued {
		loggedCount, ok := logCounts[id]
		if !ok {
			t.Fatalf("issued id %q never appeared in captured logs", id)
		}
		if loggedCount > requestCount {
			t.Fatalf("id %q appeared in %d log lines, more than the %d request(s) that produced it", id, loggedCount, requestCount)
		}
	}
}

func TestNoDSNInLogs(t *testing.T) {
	const dsnWithSecret = "postgres://tracker_user:Sup3rSecretPass!@localhost:5432/drop_tracker?sslmode=disable"

	t.Setenv("DATABASE_URL", dsnWithSecret)
	t.Setenv("LOG_FORMAT", "json")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}

	buf := &syncBuffer{}
	logger := logging.NewWithWriter(cfg, buf)
	stub := stubPinger{pingFunc: func(context.Context) error { return nil }}
	srv := httpserver.New(stub, stubStore{}, stubEventsStore{}, nil, logger)
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	_ = resp.Body.Close()

	logOutput := buf.String()
	for _, leak := range []string{"tracker_user:Sup3rSecretPass!", "postgres://"} {
		if strings.Contains(logOutput, leak) {
			t.Fatalf("captured log output leaked %q:\n%s", leak, logOutput)
		}
	}
}

// --- Phase 14 instance passphrase gate (GATE-01, GATE-07, D-14) ---

// newGatedServer builds a Server with the passphrase gate enabled, mirroring
// newCapturingServer's construction shape. Most callers pass
// trustProxyHeaders=false; the one X-Forwarded-For test passes true. The
// existing 5-argument httpserver.New call sites are deliberately untouched.
func newGatedServer(t *testing.T, passphrase string, trustProxyHeaders bool) *httpserver.Server {
	t.Helper()
	return httpserver.New(
		noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger(),
		httpserver.WithAuthGate(passphrase, trustProxyHeaders, nil),
	)
}

// gatedRoutes is the exact v1.2 route set that must move behind the gate.
var gatedRoutes = []struct {
	method, path string
}{
	{http.MethodGet, "/search"},
	{http.MethodPost, "/watchlist"},
	{http.MethodGet, "/watchlist"},
	{http.MethodPatch, "/watchlist/1"},
	{http.MethodDelete, "/watchlist/1"},
	{http.MethodGet, "/events"},
}

// TestInertPath_FiveArgConstructor proves GATE-07: a server built through the
// existing five-argument New answers every one of the seven v1.2 routes at
// its own handler, none returns 401, and the two session paths are not
// registered (they fall through to the SPA NotFound handler, HTML not JSON).
func TestInertPath_FiveArgConstructor(t *testing.T) {
	srv := httpserver.New(noopPinger{}, stubStore{}, stubEventsStore{}, nil, discardLogger())
	assertInert(t, srv)
}

// TestInertPath_EmptyPassphraseIsIndistinguishable closes the GATE-07
// adjacency edge: WithAuthGate with an empty passphrase must produce the
// identical inert route table -- no gate, no /session, no RealIP.
func TestInertPath_EmptyPassphraseIsIndistinguishable(t *testing.T) {
	srv := newGatedServer(t, "", false)
	assertInert(t, srv)
}

func assertInert(t *testing.T, srv *httpserver.Server) {
	t.Helper()
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	// /health -> 200 JSON
	hResp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	_ = hResp.Body.Close()
	if hResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /health status = %d, want 200", hResp.StatusCode)
	}

	// six data routes -> never 401
	for _, r := range gatedRoutes {
		req, _ := http.NewRequest(r.method, ts.URL+r.path, strings.NewReader("{}"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s %s: %v", r.method, r.path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusUnauthorized {
			t.Fatalf("%s %s returned 401 on an ungated server", r.method, r.path)
		}
	}

	// /session paths not registered -> fall through to the SPA shell (HTML)
	for _, m := range []string{http.MethodPost, http.MethodDelete} {
		req, _ := http.NewRequest(m, ts.URL+"/session", strings.NewReader("{}"))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s /session: %v", m, err)
		}
		ct := resp.Header.Get("Content-Type")
		_ = resp.Body.Close()
		if strings.Contains(ct, "application/json") {
			t.Fatalf("%s /session returned JSON (Content-Type %q) -- a session route is registered on an ungated server", m, ct)
		}
	}
}

// TestGatedServer_TrustProxyHeaders_RealIPWiring proves D-14: middleware.RealIP
// is wired only when trustProxyHeaders is true. It is observed through the
// access log's client IP field -- RealIP rewrites r.RemoteAddr from
// X-Forwarded-For before httplog reads it.
func TestGatedServer_TrustProxyHeaders_RealIPWiring(t *testing.T) {
	const spoofIP = "203.0.113.9"

	newWithLog := func(trust bool) (*httptest.Server, *syncBuffer) {
		buf := &syncBuffer{}
		logger := logging.NewWithWriter(&config.Config{LogLevel: "info", LogFormat: "json"}, buf)
		srv := httpserver.New(
			noopPinger{}, stubStore{}, stubEventsStore{}, nil, logger,
			httpserver.WithAuthGate("a-real-passphrase", trust, nil),
		)
		ts := httptest.NewServer(srv.Router())
		t.Cleanup(ts.Close)
		return ts, buf
	}

	t.Run("trustProxyHeaders=true rewrites the client IP from X-Forwarded-For", func(t *testing.T) {
		ts, buf := newWithLog(true)
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
		req.Header.Set("X-Forwarded-For", spoofIP)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if !strings.Contains(buf.String(), spoofIP) {
			t.Fatalf("access log does not carry the proxy-supplied IP %s (RealIP not wired):\n%s", spoofIP, buf.String())
		}
	})

	t.Run("trustProxyHeaders=false keeps the direct peer address", func(t *testing.T) {
		ts, buf := newWithLog(false)
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/health", nil)
		req.Header.Set("X-Forwarded-For", spoofIP)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /health: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if strings.Contains(buf.String(), spoofIP) {
			t.Fatalf("access log carries the spoofed X-Forwarded-For %s even though trustProxyHeaders=false:\n%s", spoofIP, buf.String())
		}
	})
}
