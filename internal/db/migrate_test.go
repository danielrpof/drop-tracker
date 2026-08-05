package db_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/config"
	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/logging"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// closedPortDSN returns a DSN pointing at loopback on a port nothing
// listens on, so connection attempts are refused promptly instead of
// timing out — an unroutable address would turn a one-second test into a
// slow one. The embedded password is a recognizable marker so tests can
// assert it never reaches a log line or a returned error (RESEARCH.md
// Pitfall 3).
func closedPortDSN(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("closedPortDSN: find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("closedPortDSN: close listener: %v", err)
	}

	return fmt.Sprintf("postgres://drop_tracker:VerySecretPassw0rd@127.0.0.1:%d/drop_tracker?sslmode=disable", port)
}

// closedPortKeywordValueDSN is closedPortDSN's libpq keyword/value-form
// equivalent (host=... user=... password=... dbname=...) -- a form pgx and
// golang-migrate both accept, and config.go places no format constraint on
// DATABASE_URL that would rule it out. This exists to catch the CR-01
// regression class: redactDSN previously only handled the URL form and
// silently echoed a keyword/value DSN back verbatim, password included.
func closedPortKeywordValueDSN(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("closedPortKeywordValueDSN: find free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("closedPortKeywordValueDSN: close listener: %v", err)
	}

	return fmt.Sprintf("host=127.0.0.1 port=%d user=drop_tracker password=VerySecretPassw0rd dbname=drop_tracker sslmode=disable", port)
}

// syncBuffer is a mutex-guarded bytes.Buffer used as a log sink, so the
// captured log output can be inspected safely regardless of what goroutine
// produced it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]byte, b.buf.Len())
	copy(out, b.buf.Bytes())
	return out
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// countWarnLines decodes each captured log line as JSON and counts how many
// have level "WARN" — a substring count of the raw text would be inflated
// by any message that happens to contain the word.
func countWarnLines(t *testing.T, data []byte) int {
	t.Helper()

	count := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal(line, &entry); err != nil {
			t.Fatalf("countWarnLines: decode log line %q: %v", line, err)
		}
		if level, _ := entry["level"].(string); level == "WARN" {
			count++
		}
	}
	return count
}

// newCapturingLogger builds a logger writing JSON to buf at Debug level, so
// every Warn line RunMigrations emits is captured.
func newCapturingLogger(buf io.Writer) *slog.Logger {
	return logging.NewWithWriter(&config.Config{LogLevel: "debug", LogFormat: "json"}, buf)
}

func TestRunMigrations_AppliesFromScratch(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	ctx := context.Background()

	// Reset state before running: drop schema_migrations if present, so
	// this test proves the apply path rather than trivially landing on the
	// no-change path when it happens to run after another test.
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()
	if _, err := sqlDB.ExecContext(ctx, "DROP TABLE IF EXISTS schema_migrations"); err != nil {
		t.Fatalf("drop schema_migrations: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Leave the database migrated afterwards so test ordering does not
	// affect other packages' fixtures.
	t.Cleanup(func() {
		_ = db.RunMigrations(context.Background(), dsn, logger)
	})

	if err := db.RunMigrations(ctx, dsn, logger); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var version int
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if version != 1 || dirty {
		t.Fatalf("schema_migrations = (version=%d, dirty=%v), want (1, false)", version, dirty)
	}
}

func TestRunMigrations_IsIdempotent(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.RunMigrations(context.Background(), dsn, logger); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	// The second call's migrate.ErrNoChange must be treated as success —
	// RESEARCH.md Pitfall 1: works on first boot, fails on every restart
	// after, if this is ever wrong.
	if err := db.RunMigrations(context.Background(), dsn, logger); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}
}

func TestRunMigrations_RetriesThenFails(t *testing.T) {
	dsn := closedPortDSN(t)
	buf := &syncBuffer{}
	logger := newCapturingLogger(buf)

	err := db.RunMigrations(context.Background(), dsn, logger,
		db.WithMaxAttempts(3),
		db.WithBaseDelay(10*time.Millisecond),
		db.WithMaxDelay(20*time.Millisecond),
	)
	if err == nil {
		t.Fatal("RunMigrations: want non-nil error, got nil")
	}
	if !strings.Contains(err.Error(), "3 attempts") {
		t.Fatalf("error message %q does not name the attempt count", err.Error())
	}

	// 3 attempts means 2 inter-attempt waits, so exactly 2 Warn lines.
	if got := countWarnLines(t, buf.Bytes()); got != 2 {
		t.Fatalf("warn line count = %d, want 2\ncaptured log:\n%s", got, buf.String())
	}
}

func TestRunMigrations_HonoursContextCancellation(t *testing.T) {
	dsn := closedPortDSN(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := db.RunMigrations(ctx, dsn, logger,
		db.WithMaxAttempts(6),
		db.WithBaseDelay(2*time.Second),
		db.WithMaxDelay(8*time.Second),
	)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunMigrations error = %v, want errors.Is(err, context.Canceled)", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("RunMigrations took %v, want well under the configured base delay", elapsed)
	}
}

func TestRunMigrations_NeverLogsDSN(t *testing.T) {
	dsn := closedPortDSN(t)
	buf := &syncBuffer{}
	logger := newCapturingLogger(buf)

	err := db.RunMigrations(context.Background(), dsn, logger,
		db.WithMaxAttempts(2),
		db.WithBaseDelay(5*time.Millisecond),
		db.WithMaxDelay(10*time.Millisecond),
	)
	if err == nil {
		t.Fatal("RunMigrations: want non-nil error, got nil")
	}

	const password = "VerySecretPassw0rd"
	logOutput := buf.String()

	if strings.Contains(logOutput, password) {
		t.Fatalf("captured log contains the DSN password:\n%s", logOutput)
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("returned error contains the DSN password: %v", err)
	}
	if strings.Contains(logOutput, "postgres://") {
		t.Fatalf("captured log contains the postgres:// scheme prefix:\n%s", logOutput)
	}
	if strings.Contains(err.Error(), "postgres://") {
		t.Fatalf("returned error contains the postgres:// scheme prefix: %v", err)
	}
}

// TestRunMigrations_NeverLogsDSN_KeywordValueForm mirrors
// TestRunMigrations_NeverLogsDSN but exercises a libpq keyword/value-form
// DSN instead of the URL form, guarding against the CR-01 regression: a
// hand-rolled url.Parse-based redactDSN silently echoed this DSN form back
// verbatim (password included) because url.Parse treats a scheme-less
// string as an opaque path.
func TestRunMigrations_NeverLogsDSN_KeywordValueForm(t *testing.T) {
	dsn := closedPortKeywordValueDSN(t)
	buf := &syncBuffer{}
	logger := newCapturingLogger(buf)

	err := db.RunMigrations(context.Background(), dsn, logger,
		db.WithMaxAttempts(2),
		db.WithBaseDelay(5*time.Millisecond),
		db.WithMaxDelay(10*time.Millisecond),
	)
	if err == nil {
		t.Fatal("RunMigrations: want non-nil error, got nil")
	}

	const password = "VerySecretPassw0rd"
	logOutput := buf.String()

	if strings.Contains(logOutput, password) {
		t.Fatalf("captured log contains the DSN password:\n%s", logOutput)
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("returned error contains the DSN password: %v", err)
	}
}
