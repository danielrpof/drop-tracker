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

// The three bounds below exist because pgx applies NONE of them on its own,
// and their combined absence is what let one wedged socket hang this process
// forever (.planning/debug/resolved/notify-pass-hangs-forever.md).
//
// pgx's only cancellation mechanism is a context watcher that sets a socket
// deadline when the caller's ctx becomes Done. The poll cycles run under a
// context derived from signal.NotifyContext, which is not Done until
// shutdown — so on a socket that is TCP-ESTABLISHED but never answers (the
// state Docker Desktop's Windows userland port-proxy leaves behind when the
// container-side connection dies: it keeps ACKing keepalives, so Go's own
// TCP keepalives never detect the break either), every read below blocks for
// the lifetime of the process with no error, no log line, and no recovery.
const (
	// connectTimeout bounds establishment — dial, TLS, and the Postgres
	// startup handshake. pgx sets Config.ConnectTimeout only when the DSN
	// carries connect_timeout=, and this project's DSN does not, so without
	// this the dialer is a bare &net.Dialer{} and the handshake read has no
	// deadline at all.
	connectTimeout = 5 * time.Second

	// pingHealthTimeout bounds the liveness ping pgxpool issues at acquire
	// time for any connection idle longer than a second. pgxpool.Config has
	// no default for this (unlike every other field), so it stays zero and
	// the ping inherits the caller's unbounded context — meaning the very
	// check meant to detect a dead connection is itself what hangs on one.
	pingHealthTimeout = 2 * time.Second

	// maxConnIdleTime must stay well under the idle window after which an
	// intermediary (Docker Desktop's port-proxy, a WSL2 NAT, a cloud NAT
	// gateway — commonly 4–5 minutes) silently drops a connection's state.
	// pgxpool's own default is 30 minutes, i.e. far longer than any of them,
	// which is what makes a half-open socket the normal case rather than a
	// rare one. Reconnecting is cheap; inheriting a rotted socket is not.
	maxConnIdleTime = 1 * time.Minute
)

// NewPool constructs the process-wide pgxpool.Pool for dsn and proves it is
// actually reachable with a timeout-bounded Ping before returning. The pool
// must be constructed exactly once at startup and shared; callers must never
// create a new pool per request or per health check.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := PoolConfig(dsn)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
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

// PoolConfig parses dsn and applies this package's explicit timeout bounds on
// top of pgxpool's defaults. It is exported (rather than inlined into
// NewPool) so a test can build a pool against an unreachable target and
// assert that operations fail rather than hang — NewPool itself cannot serve
// that test, because its own Ping refuses to return such a pool at all.
//
// A caller-supplied connect_timeout in the DSN is deliberately not
// overridden: pgx parses it into ConnConfig.ConnectTimeout, and an operator
// who set one explicitly meant it.
func PoolConfig(dsn string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config for %s: %w", redactedTarget(dsn), err)
	}

	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = connectTimeout
	}
	cfg.PingTimeout = pingHealthTimeout
	cfg.MaxConnIdleTime = maxConnIdleTime

	return cfg, nil
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
