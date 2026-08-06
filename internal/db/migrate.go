package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used below
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Default retry/backoff parameters for RunMigrations (D-10). Exported so the
// numbers are discoverable and adjustable rather than buried in the retry
// loop. RESEARCH.md assumption A1 records these as engineering judgement: a
// worst case near 20 seconds, comfortably covering a typical 2-to-10-second
// Postgres container cold start without hanging on a genuinely dead
// database.
const DefaultMaxAttempts = 6
const DefaultBaseDelay = 500 * time.Millisecond
const DefaultMaxDelay = 8 * time.Second

// retryConfig holds the bounded retry/backoff parameters for RunMigrations.
type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// RetryOption customizes RunMigrations' retry/backoff behavior. RunMigrations
// builds its policy from the Default* constants and then applies each
// supplied option in order, so the production call site in cmd/server/main.go
// (which passes no options) is unaffected by this type existing.
type RetryOption func(*retryConfig)

// WithMaxAttempts overrides the default number of attempts RunMigrations
// makes before giving up.
func WithMaxAttempts(n int) RetryOption {
	return func(cfg *retryConfig) { cfg.maxAttempts = n }
}

// WithBaseDelay overrides the default initial backoff delay between failed
// attempts.
func WithBaseDelay(d time.Duration) RetryOption {
	return func(cfg *retryConfig) { cfg.baseDelay = d }
}

// WithMaxDelay overrides the default cap on the exponentially growing
// backoff delay.
func WithMaxDelay(d time.Duration) RetryOption {
	return func(cfg *retryConfig) { cfg.maxDelay = d }
}

func newRetryConfig(opts ...RetryOption) retryConfig {
	cfg := retryConfig{
		maxAttempts: DefaultMaxAttempts,
		baseDelay:   DefaultBaseDelay,
		maxDelay:    DefaultMaxDelay,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// userInfoPattern matches a DSN's "scheme://user:password@" (or
// "scheme://user@") prefix. It exists so any error text that happens to
// embed a raw DSN verbatim can still be scrubbed before it reaches a log
// line or a returned error (RESEARCH.md Pitfall 3: pgx/driver connection
// errors can embed the DSN, and the DSN carries the Postgres password).
var userInfoPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^/@\s]*@`)

// kvPasswordPattern matches a libpq keyword/value-form password assignment
// -- "password=..." -- including the whitespace-padded, single-quoted and
// differently-cased spellings libpq accepts, plus a password passed as a
// URL query parameter, which has the identical "password=value" shape once
// URL query syntax is considered. It exists because userInfoPattern
// requires an "@" and therefore structurally cannot reach either shape:
// config.Config places no format constraint on DATABASE_URL, and pgx/
// golang-migrate accept both the keyword/value DSN form and a
// query-parameter password.
//
// The pattern deliberately requires an assignment (a "=" after the
// keyword), not just the word "password" appearing in text: Postgres's own
// authentication-failure message ("password authentication failed for user
// ...") merely mentions the word without assigning one, and destroying
// that text would cost real diagnostic value while protecting nothing.
//
// redactDSN solved the same problem for the DSN itself by delegating to
// pgconn.ParseConfig -- a route unavailable here, since redactError's
// input is arbitrary error text that merely happens to embed a DSN
// somewhere inside it and is not parseable as one.
//
// The quoted alternative must come first in the alternation: Go's regexp
// prefers the earlier alternative at a given starting position, so an
// unquoted branch listed first would stop at the value's first space and
// leave the remainder of a quoted value (which libpq permits containing
// spaces) in the output.
var kvPasswordPattern = regexp.MustCompile(`(?i)\bpassword\s*=\s*(?:'(?:[^'\\]|\\.)*'|\S+)`)

// redactDSN reduces dsn to a credential-free "host=... database=..."
// description, computed once at RunMigrations' entry so nothing downstream
// ever needs the raw DSN again for logging or error messages.
//
// It delegates to pgconn.ParseConfig rather than hand-rolling a url.Parse
// based parser, because dsn may be either the URL form
// (postgres://user:pass@host/db) or libpq's keyword/value form
// (host=... user=... password=... dbname=...) -- config.go places no format
// constraint on DATABASE_URL, and pgx/golang-migrate accept both. pgconn
// understands both forms; a hand-rolled url.Parse-based parser does not (it
// silently treats an entire keyword/value DSN as an opaque path and echoes
// it back verbatim, password included -- see CR-01).
func redactDSN(dsn string) string {
	cfg, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return "database=<unparseable>"
	}
	return fmt.Sprintf("host=%s database=%s", cfg.Host, cfg.Database)
}

// redactError reduces err's message to a form that cannot carry
// credentials. Two passes run in sequence: the URL-form userinfo strip
// first, then the keyword/value substitution over its result. Order
// matters -- the userinfo strip removes a whole "scheme://user:pass@" span
// including the scheme, so running it first means the keyword/value pass
// only ever sees what survived.
//
// Coverage: URL-form userinfo, keyword/value assignments in any accepted
// spelling (whitespace-padded, single-quoted, differently-cased), and
// password-bearing URL query parameters. This does not rely on any
// driver's internal redaction: pgx v5.10.0's ParseConfigError happens to
// self-redact its embedded connection string today, but that behaviour is
// undocumented, covers exactly one error type, and a version bump could
// remove it with no signal here (see internal/db/redact_test.go and
// .planning/debug/migrate-redacterror-keyword-value-dsn-leak.md).
func redactError(err error) string {
	s := userInfoPattern.ReplaceAllString(err.Error(), "")
	return kvPasswordPattern.ReplaceAllString(s, "password=<redacted>")
}

// RunMigrations applies every embedded migration to the database at dsn,
// retrying with exponential backoff (policy from cfg/opts, defaulting to the
// D-10 values) to tolerate a Postgres container that is still starting under
// docker-compose. It returns nil both when migrations were applied and when
// there was nothing new to apply (migrate.ErrNoChange) — treating
// ErrNoChange as anything other than success would fail every restart after
// the first successful migration.
//
// dsn itself is never logged or returned: RunMigrations reduces it to a
// credential-free host/database description at entry, and scrubs any
// DSN-shaped user-info segment out of underlying error text before it is
// logged or wrapped, since pgx/driver errors can embed DSN credentials.
func RunMigrations(ctx context.Context, dsn string, logger *slog.Logger, opts ...RetryOption) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	cfg := newRetryConfig(opts...)
	target := redactDSN(dsn)

	var lastErrMsg string
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		if err := runMigrationsOnce(ctx, dsn, src); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			lastErrMsg = redactError(err)
		} else {
			return nil
		}

		if attempt == cfg.maxAttempts {
			break
		}

		delay := cfg.baseDelay * time.Duration(uint64(1)<<uint(attempt-1))
		if delay > cfg.maxDelay {
			delay = cfg.maxDelay
		}
		logger.Warn("migration attempt failed, retrying",
			slog.Int("attempt", attempt),
			slog.Duration("delay", delay),
			slog.String("target", target),
			slog.String("error", lastErrMsg))

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("migrations failed after %d attempts against %s: %s", cfg.maxAttempts, target, lastErrMsg)
}

// runMigrationsOnce opens a fresh database/sql connection via the pgx
// stdlib driver (never lib/pq — CLAUDE.md forbids it, and golang-migrate's
// generic "postgres" database driver depends on lib/pq internally) and runs
// migrate.Up against it.
//
// ctx bounds the whole attempt, not just the connectivity check: the initial
// PingContext(ctx) catches an unreachable/slow-to-accept database before any
// migration work starts, and m.Up() itself (which has no context-aware API)
// is run on a background goroutine so a hang mid-migration (e.g. a stuck
// advisory lock) cannot block RunMigrations past ctx's cancellation (WR-01).
// If ctx is done first, runMigrationsOnce returns ctx.Err() and abandons the
// in-flight m.Up() goroutine; sqlDB.Close() (deferred) may then run
// concurrently with that goroutine's use of sqlDB, which database/sql
// supports safely -- Close causes the in-flight operation to fail rather
// than corrupt state or panic.
func runMigrationsOnce(ctx context.Context, dsn string, src source.Driver) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database/sql handle: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migrate database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- m.Up()
	}()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("apply migrations: %w", err)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
