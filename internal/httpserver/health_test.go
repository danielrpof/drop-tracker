package httpserver_test

// This file proves the operator-facing /health contract from both directions
// (D-03/D-04): the up path, the down path, the down-on-timeout path (the
// handler must never optimistically report ok when the check did not
// actually succeed), and that 20 concurrent pollers against the single
// shared pool are independent of one another.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/httpserver"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// healthBody mirrors the exact D-03/D-04 response shape by field name, not
// by raw string comparison, so a formatting change to the JSON encoding
// doesn't make an otherwise-correct response fail these tests.
type healthBody struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

// stubPinger is a file-local double for httpserver.Pinger. It exists so the
// failure and timeout branches of the handler can be driven with no
// database present and no build tag — this is exactly why plan 01-01
// declared Pinger as a seam instead of taking *pgxpool.Pool directly.
type stubPinger struct {
	pingFunc func(context.Context) error
}

func (s stubPinger) Ping(ctx context.Context) error {
	return s.pingFunc(ctx)
}

var _ httpserver.Pinger = stubPinger{}

// discardLogger builds a logger that writes nowhere, so failure-path log
// noise from the stub-backed tests below doesn't pollute test output.
func discardLogger() *slog.Logger {
	return logging.NewWithWriter(&config.Config{LogLevel: "info", LogFormat: "text"}, io.Discard)
}

func TestHealth_Up(t *testing.T) {
	pool := testutil.NewTestPool(t)
	srv := httpserver.New(pool, stubStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body healthBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Status != "ok" || body.DB != "up" {
		t.Fatalf("body = %+v, want {Status:ok DB:up}", body)
	}
}

func TestHealth_Down(t *testing.T) {
	pingErr := errors.New("connection refused: db-error-marker")
	stub := stubPinger{pingFunc: func(context.Context) error { return pingErr }}
	srv := httpserver.New(stub, stubStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	var body healthBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Status != "degraded" || body.DB != "down" {
		t.Fatalf("body = %+v, want {Status:degraded DB:down}", body)
	}

	raw := string(data)
	for _, leak := range []string{pingErr.Error(), "postgres://", "password"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("response body leaked %q: %s", leak, raw)
		}
	}
}

func TestHealth_DownOnTimeout(t *testing.T) {
	stub := stubPinger{pingFunc: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}
	srv := httpserver.New(stub, stubStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	var body healthBody
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Status != "degraded" || body.DB != "down" {
		t.Fatalf("body = %+v, want {Status:degraded DB:down} (the handler must never optimistically report ok on a check that did not succeed)", body)
	}
}

func TestHealth_Concurrent(t *testing.T) {
	pool := testutil.NewTestPool(t)
	srv := httpserver.New(pool, stubStore{}, nil, discardLogger())
	ts := httptest.NewServer(srv.Router())
	defer ts.Close()

	const n = 20

	type result struct {
		status int
		body   healthBody
		err    error
	}
	results := make([]result, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()

			resp, err := http.Get(ts.URL + "/health")
			if err != nil {
				results[idx] = result{err: err}
				return
			}
			defer resp.Body.Close()

			var body healthBody
			decodeErr := json.NewDecoder(resp.Body).Decode(&body)
			results[idx] = result{status: resp.StatusCode, body: body, err: decodeErr}
		}(i)
	}
	wg.Wait()

	for i, r := range results {
		if r.err != nil {
			t.Fatalf("request %d: %v", i, r.err)
		}
		if r.status != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i, r.status, http.StatusOK)
		}
		if r.body.Status != "ok" || r.body.DB != "up" {
			t.Fatalf("request %d: body = %+v, want {Status:ok DB:up}", i, r.body)
		}
	}
}
