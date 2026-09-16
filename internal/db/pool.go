// Package db owns the process's single Postgres connection pool and its
// embedded schema migrations. Neither the pool nor the migration runner ever
// logs the DSN — it embeds the connection password.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// pingTimeout bounds the eager connectivity check after the pool is built:
// pgxpool.New is lazy, so an unreachable DB otherwise fails only on first query.
const pingTimeout = 5 * time.Second

// The three bounds below exist because pgx applies NONE of them itself. A
// TCP-ESTABLISHED but unanswering socket otherwise hangs the process forever
// with no error; see .planning/debug/resolved/notify-pass-hangs-forever.md.
const (
	// connectTimeout bounds establishment (dial, TLS, Postgres handshake). pgx
	// only honors connect_timeout= from the DSN, which this project's DSN omits.
	connectTimeout = 5 * time.Second

	// pingHealthTimeout bounds pgxpool's acquire-time liveness ping (no default
	// exists, so it otherwise inherits the caller's unbounded context).
	pingHealthTimeout = 2 * time.Second

	// maxConnIdleTime stays well under the ~4-5 min idle window after which an
	// intermediary NAT silently drops the connection (pgxpool default: 30 min).
	maxConnIdleTime = 1 * time.Minute
)

// pollWorkerHeadroom and maxAccountedPollWorkers size the computed MaxConns
// default against poll-worker concurrency (G-11-1), not pgxpool's
// max(4, runtime.NumCPU()) which is below the poll-worker sum on a small VPS.
const (
	// pollWorkerHeadroom reserves connections beyond the worker ceiling: both
	// cycles can fan out at once, each also runs store.List + NotifyPending, and
	// the HTTP API shares the pool.
	pollWorkerHeadroom = 4

	// maxAccountedPollWorkers is an overflow guard: it keeps the int-to-int32
	// conversion in poolMaxConnsForWorkers total, so gosec has nothing to flag.
	maxAccountedPollWorkers = 1000
)

// poolMaxConnsForWorkers maps a poll-worker count to the MaxConns ceiling used
// when the operator has not set pool_max_conns. pollWorkers is clamped into
// [0, maxAccountedPollWorkers] so the int32 conversion cannot overflow.
func poolMaxConnsForWorkers(pollWorkers int) int32 {
	switch {
	case pollWorkers < 0:
		pollWorkers = 0
	case pollWorkers > maxAccountedPollWorkers:
		pollWorkers = maxAccountedPollWorkers
	}
	return int32(pollWorkers) + pollWorkerHeadroom
}

// dsnSetsMaxConns reports whether dsn explicitly carries pool_max_conns.
// pgxpool consumes and deletes that key, so its *pgxpool.Config cannot tell an
// explicit value from a default; a separate pgx.ParseConfig leaves it in
// RuntimeParams, which is what makes the distinction possible.
func dsnSetsMaxConns(dsn string) (bool, error) {
	connCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return false, err
	}
	_, ok := connCfg.RuntimeParams["pool_max_conns"]
	return ok, nil
}

// NewPool constructs the process-wide pgxpool.Pool for dsn and proves it
// reachable with a timeout-bounded Ping. Construct it once at startup and share
// it. pollWorkers is the worker ceiling this pool must serve -- see PoolConfig.
func NewPool(ctx context.Context, dsn string, pollWorkers int) (*pgxpool.Pool, error) {
	cfg, err := PoolConfig(dsn, pollWorkers)
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

// PoolConfig parses dsn and applies this package's timeout bounds on top of
// pgxpool's defaults. Exported (not inlined into NewPool) so a test can build a
// pool against an unreachable target. A connect_timeout already in the DSN is
// kept. pollWorkers sizes the MaxConns default (poolMaxConnsForWorkers) only
// when the DSN omits pool_max_conns; an explicit value survives untouched.
func PoolConfig(dsn string, pollWorkers int) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool config for %s: %w", redactedTarget(dsn), err)
	}

	if cfg.ConnConfig.ConnectTimeout == 0 {
		cfg.ConnConfig.ConnectTimeout = connectTimeout
	}
	cfg.PingTimeout = pingHealthTimeout
	cfg.MaxConnIdleTime = maxConnIdleTime

	// Defensive: ParseConfig already succeeded on this same dsn, so this second
	// parse should not fail -- handled anyway rather than panic.
	explicitMaxConns, err := dsnSetsMaxConns(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool_max_conns override for %s: %w", redactedTarget(dsn), err)
	}
	if !explicitMaxConns {
		cfg.MaxConns = poolMaxConnsForWorkers(pollWorkers)
	}

	return cfg, nil
}

// redactedTarget describes dsn by host and database name only, never its
// credentials. pgx errors frequently embed the DSN (with password) verbatim, so
// every error this package returns is built from this form.
func redactedTarget(dsn string) string {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return "<unparseable DSN>"
	}
	return fmt.Sprintf("%s/%s", cfg.ConnConfig.Host, cfg.ConnConfig.Database)
}
