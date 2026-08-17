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
	"net/url"
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

	return fmt.Sprintf("postgres://drop_tracker:local-test-fixture-password@127.0.0.1:%d/drop_tracker?sslmode=disable", port)
}

// closedPortKeywordValueDSN is closedPortDSN's libpq keyword/value-form
// equivalent (host=... user=... password=... dbname=...) -- a form pgx and
// golang-migrate both accept, and config.go places no format constraint on
// DATABASE_URL that would rule it out. Historically redactDSN was
// url.Parse-based and silently echoed this form back verbatim, password
// included, which is why the form is exercised here at all; redactDSN has
// since been rebuilt on pgconn.ParseConfig and this helper's own migration
// test only exercises the dial-failure path below, not redactDSN's or
// redactError's regex/parse coverage -- see redact_test.go for that.
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

	return fmt.Sprintf("host=127.0.0.1 port=%d user=drop_tracker password=local-test-fixture-password dbname=drop_tracker sslmode=disable", port)
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

// scratchSchemaDSN derives a DSN pointing at the same database as dsn, but
// scoped to the given schema via a search_path query parameter. pgx treats
// unrecognised URL query parameters as Postgres runtime parameters, so a
// connection opened with this DSN resolves every unqualified table
// reference -- both the migration files' CREATE TABLE statements and
// golang-migrate's own schema_migrations bookkeeping -- against schema
// instead of the connection role's default search_path.
func scratchSchemaDSN(dsn, schema string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func TestRunMigrations_AppliesFromScratch(t *testing.T) {
	dsn := testutil.RequirePostgresDSN(t)
	ctx := context.Background()

	// This test used to prove the apply-from-scratch path by dropping and
	// recreating the shared fixture's default (public) schema wholesale on a
	// bare sql.Open connection. That statement ran entirely outside
	// golang-migrate's advisory-lock serialisation and could remove tables
	// (e.g. artists) out from under any other package's concurrently-running
	// integration test (RESEARCH.md Pitfall 6). Instead, this test now gets
	// its own dedicated "migrate_scratch" schema and proves apply-from-scratch
	// there, leaving the shared fixture's default schema untouched. A fixed
	// schema name plus an if-exists guard is sufficient: Go runs one test
	// binary per package, so two instances of this test cannot overlap.
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// Registered before the schema-drop cleanup below so it runs after that
	// cleanup: t.Cleanup runs its registered funcs in LIFO order, and the
	// drop needs a live connection to run against.
	t.Cleanup(func() { _ = sqlDB.Close() })

	if _, err := sqlDB.ExecContext(ctx, "DROP SCHEMA IF EXISTS migrate_scratch CASCADE"); err != nil {
		t.Fatalf("drop scratch schema (setup): %v", err)
	}
	if _, err := sqlDB.ExecContext(ctx, "CREATE SCHEMA migrate_scratch"); err != nil {
		t.Fatalf("create scratch schema: %v", err)
	}

	// Capture the "before" reading of the shared fixture's default schema
	// ahead of any work this test does, so a genuinely unmigrated fixture is
	// never mistaken for a regression this test caused.
	var beforePublicArtists sql.NullString
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('public.artists')::text").Scan(&beforePublicArtists); err != nil {
		t.Fatalf("query public.artists before setup: %v", err)
	}
	if !beforePublicArtists.Valid {
		t.Fatal("public.artists does not exist before this test's setup -- the shared fixture was not migrated; run `make db-up` and any other DB-backed test first")
	}

	// Cleanup drops the scratch schema; it no longer needs to re-run
	// RunMigrations against the shared DSN "so test ordering does not affect
	// other packages' fixtures" -- that step existed only to repair the
	// damage the removed public-schema reset caused, and the shared schema
	// is never touched by this test anymore.
	t.Cleanup(func() {
		if _, err := sqlDB.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS migrate_scratch CASCADE"); err != nil {
			t.Errorf("cleanup: drop scratch schema: %v", err)
		}
	})

	scratchDSN, err := scratchSchemaDSN(dsn, "migrate_scratch")
	if err != nil {
		t.Fatalf("derive scratch DSN: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	if err := db.RunMigrations(ctx, scratchDSN, logger); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var version int
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT version, dirty FROM migrate_scratch.schema_migrations").Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	// Phase 5 added migration 000004_events_display_fields, so "from
	// scratch" now lands on version 4, not 3 (Phase 4's value, before this
	// migration existed).
	if version != 4 || dirty {
		t.Fatalf("schema_migrations = (version=%d, dirty=%v), want (4, false)", version, dirty)
	}

	// Isolation assertion 1: the migrations really landed in the scratch
	// schema. If the search_path parameter were silently ignored, this would
	// be NULL while the migration above still reported success (against
	// public instead).
	var scratchArtists sql.NullString
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('migrate_scratch.artists')::text").Scan(&scratchArtists); err != nil {
		t.Fatalf("query migrate_scratch.artists: %v", err)
	}
	if !scratchArtists.Valid {
		t.Fatal("migrate_scratch.artists does not exist after RunMigrations -- the search_path parameter was not honoured")
	}

	// Isolation assertion 2: the shared fixture's default schema was never
	// disturbed by this test.
	var afterPublicArtists sql.NullString
	if err := sqlDB.QueryRowContext(ctx, "SELECT to_regclass('public.artists')::text").Scan(&afterPublicArtists); err != nil {
		t.Fatalf("query public.artists after test: %v", err)
	}
	if !afterPublicArtists.Valid {
		t.Fatal("public.artists no longer exists after this test -- the shared fixture's default schema was disturbed")
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

	const password = "local-test-fixture-password"
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

// TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath mirrors
// TestRunMigrations_NeverLogsDSN but exercises a libpq keyword/value-form
// DSN instead of the URL form. Its DSN points at a closed port, so
// RunMigrations fails at PingContext with a TCP dial-refused error -- a
// dial-refused error string never contains DSN text at all, so this test
// exercises the dial-failure path end to end and proves redactDSN produces
// a usable target description for the keyword/value form. It does not, and
// cannot, exercise redactError's regex coverage for that form (a
// dial-refused error has no DSN text in it to fail to redact); that
// coverage lives in redact_test.go's TestRedactError_NeverEchoesPassword.
func TestRunMigrations_NeverLogsDSN_KeywordValueForm_DialFailurePath(t *testing.T) {
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

	const password = "local-test-fixture-password"
	logOutput := buf.String()

	if strings.Contains(logOutput, password) {
		t.Fatalf("captured log contains the DSN password:\n%s", logOutput)
	}
	if strings.Contains(err.Error(), password) {
		t.Fatalf("returned error contains the DSN password: %v", err)
	}
}
