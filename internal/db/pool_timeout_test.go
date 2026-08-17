package db

// Guards the pool half of .planning/debug/resolved/notify-pass-hangs-forever.md.
//
// pgxpool applies a default to every Config field except PingTimeout, and pgx
// sets ConnectTimeout only when the DSN carries connect_timeout= -- which this
// project's DSN does not. Built with pgxpool.New(ctx, dsn) and no Config, the
// pool therefore had no bound on establishment and no bound on the liveness
// ping it issues at acquire time. Since pgx's only cancellation mechanism is a
// context watcher that sets a socket deadline once the caller's ctx is Done,
// and the poll cycles run under a context that is not Done until shutdown, a
// socket that was TCP-ESTABLISHED but never answered blocked the process
// forever with no error.

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// blackHoleAddr returns the address of a listener that accepts TCP
// connections and then never reads or writes them. This is the shape of a
// socket left behind by a userland port-proxy whose backend connection is
// gone: the client stays ESTABLISHED, keepalives are answered, and no
// Postgres response ever arrives.
func blackHoleAddr(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	held := make(chan net.Conn, 16)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Retain the connection so it stays open (and is closed at
			// cleanup) without ever being serviced.
			select {
			case held <- conn:
			default:
				_ = conn.Close()
			}
		}
	}()

	// A single Cleanup, ordered so ln.Close() unblocks Accept and wg.Wait()
	// proves the goroutine has returned *before* held is touched -- closing
	// held first (or concurrently) raced against the goroutine's still-live
	// `held <- conn` send (found by CI's -race run, phase 07 Task 3 PR
	// verification).
	t.Cleanup(func() {
		_ = ln.Close()
		wg.Wait()
		close(held)
		for c := range held {
			_ = c.Close()
		}
	})

	return ln.Addr().String()
}

// TestPoolConfig_AppliesExplicitBounds pins the three settings whose absence
// composed into the hang. Asserting the values (not merely "non-zero") is
// deliberate: MaxConnIdleTime in particular is only useful while it stays
// below the few-minute idle window after which an intermediary silently
// drops a connection, so a future edit that relaxes it toward pgxpool's
// 30-minute default should fail here and be argued for explicitly.
func TestPoolConfig_AppliesExplicitBounds(t *testing.T) {
	// The pollWorkers argument is irrelevant to what this test asserts; 8 is
	// an arbitrary, obvious value kept consistent across this file's other
	// PoolConfig call sites that don't test MaxConns sizing.
	cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable", 8)
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}

	if cfg.ConnConfig.ConnectTimeout != connectTimeout {
		t.Errorf("ConnectTimeout = %v, want %v (pgx leaves this zero unless the DSN carries connect_timeout)", cfg.ConnConfig.ConnectTimeout, connectTimeout)
	}
	if cfg.PingTimeout != pingHealthTimeout {
		t.Errorf("PingTimeout = %v, want %v (pgxpool has no default, so zero means the acquire-time liveness ping is unbounded)", cfg.PingTimeout, pingHealthTimeout)
	}
	if cfg.MaxConnIdleTime != maxConnIdleTime {
		t.Errorf("MaxConnIdleTime = %v, want %v (pgxpool's 30m default outlives every common NAT/proxy idle window)", cfg.MaxConnIdleTime, maxConnIdleTime)
	}
}

// TestPoolConfig_RespectsExplicitConnectTimeoutInDSN is the boundary
// neighbour: an operator who set connect_timeout deliberately must keep it.
func TestPoolConfig_RespectsExplicitConnectTimeoutInDSN(t *testing.T) {
	// See TestPoolConfig_AppliesExplicitBounds: the pollWorkers argument is
	// irrelevant here too.
	cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable&connect_timeout=17", 8)
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}

	if got, want := cfg.ConnConfig.ConnectTimeout, 17*time.Second; got != want {
		t.Fatalf("ConnectTimeout = %v, want %v (an explicit DSN setting must not be overridden)", got, want)
	}
}

// TestPoolConfig_UnresponsiveServerFailsInsteadOfHanging is the behavioural
// half: it proves the bound above is what converts an unrecoverable hang
// into an ordinary error. The context passed to Exec is deliberately
// context.Background() -- carrying no deadline of its own, exactly like the
// poll cycle's runCtx -- so the only thing that can end this call is the
// pool's own configuration.
func TestPoolConfig_UnresponsiveServerFailsInsteadOfHanging(t *testing.T) {
	dsn := fmt.Sprintf("postgres://u:p@%s/db?sslmode=disable", blackHoleAddr(t))

	// See TestPoolConfig_AppliesExplicitBounds: the pollWorkers argument is
	// irrelevant here too.
	cfg, err := PoolConfig(dsn, 8)
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}
	// Shrink only the duration, not the mechanism: whether the bound exists
	// at all is what TestPoolConfig_AppliesExplicitBounds pins, and honouring
	// it is what this test proves. Keeping the real five seconds here would
	// buy no extra coverage and cost five seconds on every run.
	cfg.ConnConfig.ConnectTimeout = 500 * time.Millisecond

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	t.Cleanup(pool.Close)

	done := make(chan error, 1)
	go func() {
		_, execErr := pool.Exec(context.Background(), "SELECT 1")
		done <- execErr
	}()

	select {
	case execErr := <-done:
		if execErr == nil {
			t.Fatal("Exec against a black-hole server returned nil error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Exec against a black-hole server never returned: the pool is unbounded, and a poll cycle blocked here would hold its guard for the lifetime of the process")
	}
}

// TestPoolConfig_ComputesMaxConnsFromPollWorkers pins G-11-1's computed
// default: MaxConns must track the pollWorkers argument, strictly greater
// than the worker ceiling by exactly pollWorkerHeadroom, regardless of
// runtime.NumCPU() on the machine running the test. Asserting the exact
// value -- not merely "greater than the ceiling" -- is what keeps a future
// edit from quietly reintroducing a runtime.NumCPU() dependence.
func TestPoolConfig_ComputesMaxConnsFromPollWorkers(t *testing.T) {
	tests := []struct {
		name        string
		pollWorkers int
		wantMax     int32
	}{
		{name: "worker ceiling 8 (default MB=3 + Deezer=5)", pollWorkers: 8, wantMax: 12},
		{name: "worker ceiling 64 tracks the argument, not a fixed constant", pollWorkers: 64, wantMax: 68},
		{name: "worker ceiling 0 (test pools) yields headroom only", pollWorkers: 0, wantMax: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable", tt.pollWorkers)
			if err != nil {
				t.Fatalf("PoolConfig: %v", err)
			}
			if cfg.MaxConns != tt.wantMax {
				t.Errorf("MaxConns = %d, want %d (must not depend on runtime.NumCPU())", cfg.MaxConns, tt.wantMax)
			}
		})
	}
}

// TestPoolConfig_RespectsExplicitMaxConnsInDSN is the boundary neighbour to
// TestPoolConfig_RespectsExplicitConnectTimeoutInDSN: an operator who set
// pool_max_conns explicitly must keep it, even when it sits below the
// worker ceiling that would otherwise be computed -- a tight server-side
// max_connections or a PgBouncer limit is a real reason to cap it below
// what the poll workers alone would want. This is the assertion that fails
// if the override check is implemented against pgxpool's already-consumed
// Config.RuntimeParams instead of a separate pgx.ParseConfig call: by the
// time pgxpool.ParseConfig returns, it has already deleted pool_max_conns
// from RuntimeParams as it consumes it, so an operator's explicit value and
// an incidental NumCPU-derived default become indistinguishable.
func TestPoolConfig_RespectsExplicitMaxConnsInDSN(t *testing.T) {
	cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable&pool_max_conns=6", 8)
	if err != nil {
		t.Fatalf("PoolConfig: %v", err)
	}

	if got, want := cfg.MaxConns, int32(6); got != want {
		t.Fatalf("MaxConns = %d, want %d (an explicit DSN setting must not be overridden by the worker ceiling, even when it is smaller)", got, want)
	}
}
