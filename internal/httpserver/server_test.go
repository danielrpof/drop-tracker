package httpserver_test

// This file proves OPS-02: every response carries an X-Request-Id that
// matches the ID in that request's structured JSON log line, inbound IDs are
// honoured rather than overridden, concurrent requests never share an ID,
// and no captured log line carries the configured DSN.

import (
	"bytes"
	"context"
	"encoding/json"
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
