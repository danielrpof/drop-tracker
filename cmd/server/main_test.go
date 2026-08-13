package main

// This file tests the boot path directly by calling the unexported run
// function -- whitebox, same package as cmd/server itself -- so the
// graceful-shutdown branch can be driven by cancelling a real context
// rather than signalling the test process (09-RESEARCH.md Pitfall 2).

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// TestRun_ConfigLoadFailureReturnsEarly proves the fail-fast branch: an
// empty DATABASE_URL (the only notEmpty field) must make run return before
// it ever reaches the migration step, not fail later for an unrelated
// reason. No database is needed for this test.
func TestRun_ConfigLoadFailureReturnsEarly(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	err := run(context.Background())
	if err == nil {
		t.Fatal("run() error = nil, want non-nil (DATABASE_URL is required)")
	}
	if !strings.Contains(err.Error(), "load config:") {
		t.Fatalf("run() error = %q, want it to contain %q", err.Error(), "load config:")
	}
	if strings.Contains(err.Error(), "run migrations:") {
		t.Fatalf("run() error = %q, must not contain %q -- run() reached the migration step instead of returning on the config-load failure", err.Error(), "run migrations:")
	}
}

// TestRun_BootServesHealthThenGracefulShutdownOnCancel proves the other
// half of the boot path: run boots to a serving /health endpoint against a
// real Postgres instance, and returns nil after a graceful shutdown once
// its context is cancelled -- it must neither error nor hang.
func TestRun_BootServesHealthThenGracefulShutdownOnCancel(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)

	// Reserve a free TCP port by listening on port zero and reading the
	// assigned port back, then close the listener before run binds it --
	// a hardcoded port could collide with a developer's running server or
	// a parallel CI job (T-09-11).
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a free port: %v", err)
	}
	_, portStr, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("split reserved listener address: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}

	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("HTTP_PORT", portStr)
	t.Setenv("LOG_FORMAT", "json")
	t.Setenv("LOG_LEVEL", "error")
	// Long enough that no poll cycle can fire during the test.
	t.Setenv("POLL_INTERVAL", "1h")
	// Empty so notifier.Select resolves to its no-op selection.
	t.Setenv("DISCORD_WEBHOOK_URL", "")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx)
	}()

	healthURL := "http://127.0.0.1:" + portStr + "/health"
	deadline := time.Now().Add(10 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, getErr := http.Get(healthURL)
		if getErr != nil {
			lastErr = getErr
			time.Sleep(50 * time.Millisecond)
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr != nil {
		cancel()
		<-done
		t.Fatalf("server never became healthy before the deadline: %v", lastErr)
	}

	cancel()

	select {
	case runErr := <-done:
		if runErr != nil {
			t.Fatalf("run() returned error = %v, want nil (graceful shutdown)", runErr)
		}
	case <-time.After(shutdownTimeout + 5*time.Second):
		t.Fatal("run() did not return within the shutdown deadline after context cancellation -- shutdown hung")
	}
}
