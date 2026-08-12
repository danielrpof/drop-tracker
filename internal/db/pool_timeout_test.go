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
	cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable")
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
	cfg, err := PoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable&connect_timeout=17")
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

	cfg, err := PoolConfig(dsn)
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
