// Package db owns the process's single Postgres connection pool and its
// embedded schema migrations. Both the pool and the migration runner take a
// raw DSN string; neither ever logs it, because the DSN embeds the
// connection password.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// pingTimeout bounds the eager connectivity check performed after the pool
// is constructed. pgxpool.New is lazy — it validates the DSN string but does
// not connect — so without this bound a startup against an unreachable
// database would report success and only fail on the first real query.
const pingTimeout = 5 * time.Second

// NewPool constructs the process-wide pgxpool.Pool for dsn and proves it is
// actually reachable with a timeout-bounded Ping before returning. The pool
// must be constructed exactly once at startup and shared; callers must never
// create a new pool per request or per health check.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("create pool for %s: %w", redactedTarget(dsn), err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping %s: %w", redactedTarget(dsn), err)
	}

	return pool, nil
}

// redactedTarget describes dsn using only its host and database name, never
// its credentials. pgx connection errors frequently embed the DSN verbatim
// (which carries the password), so every error this package returns must be
// built from this redacted form rather than from dsn or from a raw pgx error
// message that might contain it.
func redactedTarget(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "<unparseable DSN>"
	}
	return fmt.Sprintf("%s/%s", cfg.ConnConfig.Host, cfg.ConnConfig.Database)
}
