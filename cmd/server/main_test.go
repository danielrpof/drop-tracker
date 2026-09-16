package main

// This file tests the boot path directly by calling the unexported run
// function -- whitebox, same package as cmd/server itself -- so the
// graceful-shutdown branch can be driven by cancelling a real context
// rather than signalling the test process (09-RESEARCH.md Pitfall 2).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// nonEmptyLines splits a captured log buffer into its non-blank lines, so a
// test can assert a helper emitted exactly one record -- a future second log
// call inside that helper then fails the count.
func nonEmptyLines(s string) []string {
	var out []string
	for _, ln := range strings.Split(s, "\n") {
		if strings.TrimSpace(ln) != "" {
			out = append(out, ln)
		}
	}
	return out
}

// decodeRecord parses one JSON slog record into a map.
func decodeRecord(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("decode log record %q: %v", line, err)
	}
	return m
}

// recordMentions reports whether substr appears in any key or stringified
// value of rec. Callers delete the "time" key first: the JSON timestamp is
// full of digits and a two-digit passphrase length would eventually collide
// with it, making a raw-buffer scan flaky.
func recordMentions(rec map[string]any, substr string) bool {
	for k, v := range rec {
		if strings.Contains(k, substr) {
			return true
		}
		if strings.Contains(fmt.Sprintf("%v", v), substr) {
			return true
		}
	}
	return false
}

// TestLogInstanceGateStatus pins the G-14-1 observability fix: every boot
// emits exactly one Info record stating whether the instance passphrase gate
// is active or inert, and the active-branch record carries neither the
// passphrase nor its decimal rune count.
func TestLogInstanceGateStatus(t *testing.T) {
	const passphrase = "correct-horse-battery-staple-9times"
	runeCount := strconv.Itoa(len([]rune(passphrase)))

	newLogger := func(buf *bytes.Buffer) *slog.Logger {
		return logging.NewWithWriter(&config.Config{LogLevel: "info", LogFormat: "json"}, buf)
	}

	t.Run("active passphrase", func(t *testing.T) {
		var buf bytes.Buffer
		logInstanceGateStatus(newLogger(&buf), passphrase)

		lines := nonEmptyLines(buf.String())
		if len(lines) != 1 {
			t.Fatalf("emitted %d records, want exactly 1: %q", len(lines), buf.String())
		}
		rec := decodeRecord(t, lines[0])
		if rec["level"] != "INFO" {
			t.Errorf("record level = %v, want INFO", rec["level"])
		}
		if !recordMentions(rec, "active") {
			t.Errorf("active-branch record does not report the gate as active: %v", rec)
		}
		delete(rec, "time")
		if recordMentions(rec, passphrase) {
			t.Errorf("record leaked the passphrase: %v", rec)
		}
		if recordMentions(rec, runeCount) {
			t.Errorf("record leaked the passphrase rune count %s: %v", runeCount, rec)
		}
	})

	t.Run("empty passphrase", func(t *testing.T) {
		var buf bytes.Buffer
		logInstanceGateStatus(newLogger(&buf), "")

		lines := nonEmptyLines(buf.String())
		if len(lines) != 1 {
			t.Fatalf("emitted %d records, want exactly 1: %q", len(lines), buf.String())
		}
		rec := decodeRecord(t, lines[0])
		if rec["level"] != "INFO" {
			t.Errorf("record level = %v, want INFO", rec["level"])
		}
		if !recordMentions(rec, "inert") {
			t.Errorf("inert-branch record does not report the gate as inert: %v", rec)
		}
		if !recordMentions(rec, ".env") {
			t.Errorf("inert-branch record does not name the repo-root .env remediation channel: %v", rec)
		}
	})
}

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
	deadline := time.After(10 * time.Second)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

pollHealthy:
	for {
		select {
		case runErr := <-done:
			// run() returned before the server ever became healthy -- report
			// its real error directly instead of letting the deadline case
			// below mask it as a generic health-timeout.
			t.Fatalf("run() returned before the server became healthy: %v", runErr)
		case <-deadline:
			t.Fatal("server never became healthy before the deadline")
		case <-ticker.C:
			// Bounded per-attempt timeout: a bare http.Get has none, so a
			// connection that completes its TCP handshake but never writes a
			// response (e.g. something else already accepting on the port)
			// would block this select case indefinitely -- starving the
			// done/deadline cases above of ever being chosen again and
			// defeating this loop's own bounded-wait contract.
			pollCtx, pollCancel := context.WithTimeout(ctx, 2*time.Second)
			req, reqErr := http.NewRequestWithContext(pollCtx, http.MethodGet, healthURL, nil)
			if reqErr != nil {
				pollCancel()
				t.Fatalf("build health request: %v", reqErr)
			}
			resp, getErr := http.DefaultClient.Do(req)
			pollCancel()
			if getErr != nil {
				continue
			}
			if resp.StatusCode == http.StatusOK {
				_ = resp.Body.Close()
				break pollHealthy
			}
			_ = resp.Body.Close()
		}
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
