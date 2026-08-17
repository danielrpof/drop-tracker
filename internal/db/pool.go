// Package db owns the process's single Postgres connection pool and its
// embedded schema migrations. Both the pool and the migration runner take a
// raw DSN string; neither ever logs it, because the DSN embeds the
// connection password.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

// pollWorkerHeadroom and maxAccountedPollWorkers size the computed MaxConns
// default against the poll-worker concurrency this pool must actually serve
// (G-11-1), rather than against pgxpool's own max(4, runtime.NumCPU())
// default -- which is below MusicBrainzPollWorkers + DeezerPollWorkers on
// any host with fewer vCPUs than that sum, including the small VPS this
// project targets.
const (
	// pollWorkerHeadroom reserves connections beyond the poll-worker ceiling
	// itself: both poll cycles can be fanned out at the same time (they are
	// separate cron entries on the same interval), each cycle also runs its
	// own store.List at the start and NotifyPending after wg.Wait() while the
	// other source's cycle may still be fanned out, and the HTTP API draws
	// from the same pool for the /health ping, watchlist CRUD, and /events.
	pollWorkerHeadroom = 4

	// maxAccountedPollWorkers is an overflow guard, not a policy ceiling on
	// worker counts: it keeps the int-to-int32 conversion in
	// poolMaxConnsForWorkers total, so gosec's integer-conversion check has
	// nothing to flag and no nolint directive is needed.
	maxAccountedPollWorkers = 1000
)

// poolMaxConnsForWorkers maps a poll-worker count to the MaxConns ceiling
// PoolConfig should apply when the operator has not set pool_max_conns
// explicitly. pollWorkers is clamped into [0, maxAccountedPollWorkers] before
// conversion to int32 so the conversion itself can never overflow or go
// negative regardless of what a misconfigured caller passes.
func poolMaxConnsForWorkers(pollWorkers int) int32 {
	switch {
	case pollWorkers < 0:
		pollWorkers = 0
	case pollWorkers > maxAccountedPollWorkers:
		pollWorkers = maxAccountedPollWorkers
	}
	return int32(pollWorkers) + pollWorkerHeadroom
}

// dsnSetsMaxConns reports whether dsn explicitly carries pool_max_conns, per
// the only reliable operator-intent signal for this setting: pgxpool.ParseConfig
// always populates Config.MaxConns (defaulting to max(4, runtime.NumCPU())
// when the DSN is silent on it) and, when the DSN did carry pool_max_conns,
// deletes that key from RuntimeParams as it consumes it -- so the returned
// *pgxpool.Config can never distinguish an operator's explicit value from an
// incidental default. A separate pgx.ParseConfig call leaves pool_max_conns
// in RuntimeParams untouched, which is what makes the distinction possible.
func dsnSetsMaxConns(dsn string) (bool, error) {
	connCfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return false, err
	}
	_, ok := connCfg.RuntimeParams["pool_max_conns"]
	return ok, nil
}

// NewPool constructs the process-wide pgxpool.Pool for dsn and proves it is
// actually reachable with a timeout-bounded Ping before returning. The pool
// must be constructed exactly once at startup and shared; callers must never
// create a new pool per request or per health check.
//
// pollWorkers is the poll-cycle worker ceiling (MusicBrainzPollWorkers +
// DeezerPollWorkers) this pool must be sized to serve -- see PoolConfig.
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

// PoolConfig parses dsn and applies this package's explicit timeout bounds on
// top of pgxpool's defaults. It is exported (rather than inlined into
// NewPool) so a test can build a pool against an unreachable target and
// assert that operations fail rather than hang — NewPool itself cannot serve
// that test, because its own Ping refuses to return such a pool at all.
//
// A caller-supplied connect_timeout in the DSN is deliberately not
// overridden: pgx parses it into ConnConfig.ConnectTimeout, and an operator
// who set one explicitly meant it.
//
// pollWorkers is the poll-cycle worker ceiling (MusicBrainzPollWorkers +
// DeezerPollWorkers) this pool must be sized to serve (G-11-1): the computed
// MaxConns default is poolMaxConnsForWorkers(pollWorkers), applied only when
// the DSN does not already carry pool_max_conns. An operator who set
// pool_max_conns explicitly meant it, including when their value sits below
// the worker ceiling, because a tight server-side max_connections or a
// PgBouncer limit is a real reason to cap it -- that value must survive
// untouched even when it is smaller than what this function would otherwise
// compute.
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

	// pgxpool.ParseConfig above has already succeeded, so an error from this
	// second parse is defensive rather than expected -- both calls parse the
	// same dsn, so a failure here would mean the first call should also have
	// failed. Handled anyway so this path never panics or silently
	// misattributes the override check.
	explicitMaxConns, err := dsnSetsMaxConns(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pool_max_conns override for %s: %w", redactedTarget(dsn), err)
	}
	if !explicitMaxConns {
		cfg.MaxConns = poolMaxConnsForWorkers(pollWorkers)
	}

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
