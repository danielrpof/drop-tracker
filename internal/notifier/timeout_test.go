package notifier

// This file is package notifier (whitebox), mirroring
// internal/discord/client_test.go's own rationale: it shrinks the unexported
// dbOpTimeout so the regression below runs in milliseconds instead of
// waiting out the real ten-second bound.
//
// It guards the failure documented in
// .planning/debug/resolved/notify-pass-hangs-forever.md: a database call
// that never returns parks NotifyPending forever while it holds the
// notifying CAS guard, so every later poll cycle logs "skipping notify
// pass: already in progress" for the lifetime of the process and no
// notification is ever delivered again.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)

// wedgingQuerier reproduces a pgx call on a socket that is TCP-ESTABLISHED
// but never answers: ListUnnotified returns only when its context is done,
// which -- absent a deadline somewhere in the chain -- is never. The
// embedded nil sqlc.Querier is deliberate: any method this test does not
// override is one NotifyPending must not be reaching, and would panic
// loudly rather than silently pass.
type wedgingQuerier struct {
	sqlc.Querier

	calls atomic.Int32
	// wedgeFirstCallOnly makes only the first call block, so a test can
	// prove the guard was released by driving a second, healthy pass
	// through the very same Notifier instance.
	wedgeFirstCallOnly bool
}

func (w *wedgingQuerier) ListUnnotified(ctx context.Context) ([]sqlc.Event, error) {
	if w.calls.Add(1) > 1 && w.wedgeFirstCallOnly {
		return nil, nil
	}
	<-ctx.Done()
	return nil, ctx.Err()
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// noopSender is a local stand-in rather than notifier_test.go's fakeSender,
// which lives in the external notifier_test package and is unreachable from
// this whitebox file. No test here ever reaches a send: every pass either
// fails in ListUnnotified or drains an empty batch.
type noopSender struct{}

func (noopSender) Send(context.Context, discord.Embed) error { return nil }

// shrinkDBOpTimeout shrinks the package's database-operation bound for the
// duration of one test.
func shrinkDBOpTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	orig := dbOpTimeout
	dbOpTimeout = d
	t.Cleanup(func() { dbOpTimeout = orig })
}

// callNotifyPending runs NotifyPending on its own goroutine and fails the
// test if it has not returned within limit. Without this guard a regression
// would hang the test binary until Go's global timeout rather than
// reporting a comprehensible failure -- and hanging forever is precisely
// the defect under test.
func callNotifyPending(t *testing.T, n *Notifier, ctx context.Context, limit time.Duration) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- n.NotifyPending(ctx, discardLogger()) }()

	select {
	case err := <-done:
		return err
	case <-time.After(limit):
		t.Fatalf("NotifyPending did not return within %v: it is blocked on an unbounded database call, which wedges the notifying guard for the process's lifetime", limit)
		return nil
	}
}

// TestNotifyPending_UnresponsiveDatabase_ReturnsInsteadOfWedging is the
// primary regression guard. An unbounded context reaching pgx is what made
// the original hang permanent and silent; NotifyPending must bound its own
// database calls so the failure surfaces as an ordinary error.
func TestNotifyPending_UnresponsiveDatabase_ReturnsInsteadOfWedging(t *testing.T) {
	shrinkDBOpTimeout(t, 50*time.Millisecond)

	n := New(&wedgingQuerier{}, noopSender{}, time.Millisecond)

	// context.Background() is the point: it never becomes Done, exactly like
	// the poll cycle's runCtx (derived from signal.NotifyContext). The bound
	// must come from NotifyPending itself, not from its caller.
	err := callNotifyPending(t, n, context.Background(), 5*time.Second)

	if err == nil {
		t.Fatal("NotifyPending: want an error when the database never responds, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("NotifyPending error = %v, want it to wrap context.DeadlineExceeded", err)
	}
	if !strings.Contains(err.Error(), "list unnotified") {
		t.Fatalf("NotifyPending error = %q, want it to name the failing operation", err.Error())
	}
}

// TestNotifyPending_RecoversAfterUnresponsiveDatabase is the guard for the
// user-visible symptom itself. The original bug was never that one pass
// failed -- it was that the failed pass never returned, so the notifying
// CAS guard stayed set and every subsequent cycle logged "skipping notify
// pass: already in progress" forever. A later healthy pass must run.
func TestNotifyPending_RecoversAfterUnresponsiveDatabase(t *testing.T) {
	shrinkDBOpTimeout(t, 50*time.Millisecond)

	q := &wedgingQuerier{wedgeFirstCallOnly: true}
	n := New(q, noopSender{}, time.Millisecond)

	if err := callNotifyPending(t, n, context.Background(), 5*time.Second); err == nil {
		t.Fatal("first pass: want an error while the database is unresponsive, got nil")
	}

	// The second pass must actually reach the querier. If the guard had
	// leaked, NotifyPending would return nil immediately after logging
	// "skipping notify pass: already in progress" and calls would stay at 1.
	if err := callNotifyPending(t, n, context.Background(), 5*time.Second); err != nil {
		t.Fatalf("second pass: want success once the database recovers, got %v", err)
	}
	if got := q.calls.Load(); got != 2 {
		t.Fatalf("ListUnnotified calls = %d, want 2: the second pass was skipped, meaning the notifying guard was never released", got)
	}
}

// TestNotifyPending_ParentCancellationStillPropagates is the boundary
// neighbour of the bound above: introducing a deadline must not swallow the
// caller's own cancellation. Shutdown cancels the poll context to unwind
// in-flight cycles, and that must still win when it fires first.
func TestNotifyPending_ParentCancellationStillPropagates(t *testing.T) {
	// Deliberately far longer than the test's own patience: if the parent's
	// cancellation were ignored in favour of this bound, the assertion below
	// would fail rather than silently pass on a timeout that happened to
	// look similar.
	shrinkDBOpTimeout(t, time.Hour)

	n := New(&wedgingQuerier{}, noopSender{}, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	defer cancel()

	err := callNotifyPending(t, n, ctx, 5*time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NotifyPending error = %v, want it to wrap context.Canceled", err)
	}
}
