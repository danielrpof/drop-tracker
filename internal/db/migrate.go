package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used below
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const (
	defaultMaxAttempts = 6
	defaultBaseDelay   = 500 * time.Millisecond
	defaultMaxDelay    = 8 * time.Second
)

// retryConfig holds the bounded retry/backoff parameters for RunMigrations.
// Plan 05 makes these injectable through RetryOption and adds the
// behavioural tests; the variadic signature exists from this task so that
// refactor does not touch RunMigrations' call site.
type retryConfig struct {
	maxAttempts int
	baseDelay   time.Duration
	maxDelay    time.Duration
}

// RetryOption customizes RunMigrations' retry/backoff behavior.
type RetryOption func(*retryConfig)

func newRetryConfig(opts ...RetryOption) retryConfig {
	cfg := retryConfig{
		maxAttempts: defaultMaxAttempts,
		baseDelay:   defaultBaseDelay,
		maxDelay:    defaultMaxDelay,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// RunMigrations applies every embedded migration to the database at dsn,
// retrying with exponential backoff to tolerate a Postgres container that is
// still starting under docker-compose. It returns nil both when migrations
// were applied and when there was nothing new to apply
// (migrate.ErrNoChange) — treating ErrNoChange as anything other than
// success would fail every restart after the first successful migration.
//
// dsn is never logged: only the attempt number and delay are logged on
// retry, never the error's string form, since pgx/driver errors can embed
// the DSN's credentials.
func RunMigrations(ctx context.Context, dsn string, logger *slog.Logger, opts ...RetryOption) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}

	cfg := newRetryConfig(opts...)

	var lastErr error
	for attempt := 1; attempt <= cfg.maxAttempts; attempt++ {
		if err := runMigrationsOnce(dsn, src); err != nil {
			lastErr = err
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
			slog.Int("attempt", attempt), slog.Duration("delay", delay))

		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("migrations failed after %d attempts: %w", cfg.maxAttempts, lastErr)
}

// runMigrationsOnce opens a fresh database/sql connection via the pgx
// stdlib driver (never lib/pq — CLAUDE.md forbids it, and golang-migrate's
// generic "postgres" database driver depends on lib/pq internally) and runs
// migrate.Up against it.
func runMigrationsOnce(dsn string, src source.Driver) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database/sql handle: %w", err)
	}
	defer sqlDB.Close()

	dbDriver, err := pgxmigrate.WithInstance(sqlDB, &pgxmigrate.Config{})
	if err != nil {
		return fmt.Errorf("create migrate database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", dbDriver)
	if err != nil {
		return fmt.Errorf("create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
