package notifier_test

// digest_schedule_test.go proves DGST-05/DGST-06's behavioural matrix by
// driving a real *DigestScheduler over a real *Notifier -- never
// SendDigestIfDue directly -- through an injected mutable clock and a
// manually-driven tick source: exactly one Discord send per digest slot
// across all four 2026/2027 America/New_York DST transitions for both
// cadences (Task 1), plus the grace-window catch-up/refusal/nothing-lost
// matrix (Task 2, appended below). Reuses notifier_test.go/digest_test.go's
// fixtures (insertTestArtist, insertPendingEventTyped, isNotified,
// newTestLogger, digestSettingsReader, decodeLogRecords/logRecord,
// snapshotSettings) and scheduler_test.go's neverFiringTickSource, since
// this file shares the same notifier_test package.

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
	"github.com/danielrpof/drop-tracker/internal/notifier"
	"github.com/danielrpof/drop-tracker/internal/settings"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// mustLoadDigestZone loads settings.ZoneName once per test, failing on
// error rather than skipping -- both host and CI carry zoneinfo (plan
// 22-04's own Alpine boot proof is a separate CI step, not this test).
func mustLoadDigestZone(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(settings.ZoneName)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q): %v", settings.ZoneName, err)
	}
	return loc
}

// mutableClock is a goroutine-safe now-source the sweep helper advances
// between checks, injected as DigestScheduler's now func. Every write
// happens-before the tick-channel send (or the Start call) that triggers
// the check reading it, and every read inside a check happens-before the
// signalingSink's done signal the test goroutine waits on next -- so plain
// mutex-guarded fields are sufficient; nothing here needs to be lock-free.
type mutableClock struct {
	mu  sync.Mutex
	cur time.Time
}

func newMutableClock(start time.Time) *mutableClock {
	return &mutableClock{cur: start}
}

func (c *mutableClock) set(t time.Time) {
	c.mu.Lock()
	c.cur = t
	c.mu.Unlock()
}

func (c *mutableClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

// signalingSink wraps a notifier.Sink (a real *notifier.Notifier in every
// sweep test below) and signals done after every SendDigestIfDue call
// returns, so sweep can wait for "the resulting check has completed"
// before advancing the clock to the next step -- synchronising on a
// channel the test goroutine reads, never a wall-clock sleep.
type signalingSink struct {
	inner notifier.Sink
	done  chan struct{}
}

func newSignalingSink(inner notifier.Sink) *signalingSink {
	return &signalingSink{inner: inner, done: make(chan struct{}, 1)}
}

func (s *signalingSink) NotifyPending(ctx context.Context, logger *slog.Logger) error {
	return s.inner.NotifyPending(ctx, logger)
}

func (s *signalingSink) SendDigestIfDue(ctx context.Context, logger *slog.Logger, now time.Time) error {
	err := s.inner.SendDigestIfDue(ctx, logger, now)
	select {
	case s.done <- struct{}{}:
	default:
	}
	return err
}

var _ notifier.Sink = (*signalingSink)(nil)

// waitSweepStep blocks until sink signals that a check has completed, or
// fails the test after timeout -- never a polling sleep loop.
func waitSweepStep(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for a sweep step's check to complete")
	}
}

// sweep drives sched (built over a signalingSink) from start (inclusive) to
// end (exclusive) in 5-minute real-time steps: it sets clock to each
// step's instant, fires exactly one check per step -- the immediate check
// Start itself runs for the first step, a manually-driven tick for every
// step after -- and waits for that check to finish via sink's done channel
// before advancing, never a wall-clock sleep. Stops sched once the sweep
// ends.
func sweep(t *testing.T, sched *notifier.DigestScheduler, sink *signalingSink, clock *mutableClock, tickCh chan<- time.Time, start, end time.Time) {
	t.Helper()
	const step = 5 * time.Minute

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock.set(start)
	sched.Start(ctx)
	waitSweepStep(t, sink.done)

	for cur := start.Add(step); cur.Before(end); cur = cur.Add(step) {
		clock.set(cur)
		tickCh <- cur
		waitSweepStep(t, sink.done)
	}

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v, want nil", err)
	}
}

// newManualTickSource returns a bidirectional tick channel the sweep helper
// drives, plus the tickSource func DigestScheduler needs -- mirroring
// scheduler_test.go's TestDigestScheduler_TickSource_NPlusOneCalls pattern.
func newManualTickSource() (chan time.Time, func() (<-chan time.Time, func())) {
	tickCh := make(chan time.Time)
	return tickCh, func() (<-chan time.Time, func()) { return tickCh, func() {} }
}

// seedNotificationSettings sets the singleton notification_settings row's
// digest_enabled/digest_cadence/digest_last_slot_at columns directly via
// SQL, bypassing settings.Service.Update's own D-14 re-anchor logic, so a
// test can establish an arbitrary "already handled up to this slot"
// starting state without that logic recomputing its own slot.
func seedNotificationSettings(t *testing.T, pool *pgxpool.Pool, enabled bool, cadence settings.Cadence, lastSlot *time.Time) {
	t.Helper()
	if _, err := pool.Exec(context.Background(),
		`UPDATE notification_settings SET digest_enabled = $1, digest_cadence = $2, digest_last_slot_at = $3 WHERE id = 1`,
		enabled, string(cadence), lastSlot,
	); err != nil {
		t.Fatalf("seed notification_settings: %v", err)
	}
}

// insertPendingEventTypedRaw is insertPendingEventTyped's t-free twin, for
// use inside a fakeSender callback: that callback runs on the
// DigestScheduler's own goroutine, and calling t.Fatalf there would violate
// testing's single-goroutine contract for FailNow. Errors are returned
// instead, for the caller to check back on the test goroutine once the
// channel-synchronised sweep step has completed.
func insertPendingEventTypedRaw(pool *pgxpool.Pool, artistID int64, eventType, externalID, title string) (int64, error) {
	var eventID int64
	err := pool.QueryRow(context.Background(),
		`INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name, release_date)
		 VALUES ($1, 'musicbrainz', $2, $3, $4, 'Test Artist', $5)
		 RETURNING id`,
		artistID, eventType, externalID, title, todayDate,
	).Scan(&eventID)
	return eventID, err
}

// ---------------------------------------------------------------------
// Task 1: the DST matrix.
// ---------------------------------------------------------------------

// TestDigestScheduler_DST_Daily proves the daily cadence produces exactly
// one send per calendar day across all four 2026/2027 America/New_York DST
// transition dates, including the two boundary offsets (-05:00 spring
// pre-transition, -04:00 fall post-transition already implied by
// wantSlot's own zone-aware construction).
func TestDigestScheduler_DST_Daily(t *testing.T) {
	cases := []struct {
		name  string
		year  int
		month time.Month
		day   int
	}{
		{"2026-03-08_spring-forward", 2026, time.March, 8},
		{"2026-11-01_fall-back", 2026, time.November, 1},
		{"2027-03-14_spring-forward", 2027, time.March, 14},
		{"2027-11-07_fall-back", 2027, time.November, 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loc := mustLoadDigestZone(t)
			pool := testutil.NewIsolatedTestPool(t, "notifier_test")
			q := sqlc.New(pool)

			dayStart := time.Date(tc.year, tc.month, tc.day, 0, 0, 0, 0, loc)
			dayEnd := dayStart.AddDate(0, 0, 1)
			wantSlot := time.Date(tc.year, tc.month, tc.day, 0, 5, 0, 0, loc)
			prevSlot := wantSlot.AddDate(0, 0, -1)

			seedNotificationSettings(t, pool, true, settings.CadenceDaily, &prevSlot)
			artistID := insertTestArtist(t, pool, "dst-daily")
			insertPendingEventTyped(t, pool, artistID, "new_release", "dst-daily-ext", "DST Daily Title")

			sender := &fakeSender{}
			svc := settings.NewService(q, loc)
			n := notifier.New(q, sender, svc, time.Millisecond, notifier.WithLocation(loc))
			logger, _ := newTestLogger()

			clock := newMutableClock(dayStart)
			tickCh, manual := newManualTickSource()
			sink := newSignalingSink(n)
			sched := notifier.NewDigestScheduler(sink, logger, clock.now, notifier.WithTickSource(manual))

			sweep(t, sched, sink, clock, tickCh, dayStart, dayEnd)

			if got := atomic.LoadInt32(&sender.calls); got != 1 {
				t.Fatalf("send count = %d, want exactly 1", got)
			}
			row, err := q.GetNotificationSettings(context.Background())
			if err != nil {
				t.Fatalf("get notification settings: %v", err)
			}
			if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(wantSlot) {
				t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, wantSlot)
			}
			if !row.DigestLastSentAt.Valid {
				t.Fatalf("digest_last_sent_at is NULL, want non-NULL")
			}
		})
	}
}

// TestDigestScheduler_DST_Daily_FallBack_SlotRecordNotEmptyOutboxPreventsSecondSend
// is 22-CONTEXT.md's <specifics> proof, on the 25-hour 2026-11-01 fall-back
// day where a duration-based due rule would double-fire: a second pending
// event is inserted the instant the first send is observed (inside the
// fake Sender's own callback, so it lands strictly after sentIDs was
// already computed from the first read), and the sweep continues to the
// end of that local day. Exactly one send and an un-acked second event is
// what distinguishes "the slot record prevented the second fire" from "the
// outbox happened to be empty" -- an implementation with no slot record at
// all would still pass every other DST subtest above.
func TestDigestScheduler_DST_Daily_FallBack_SlotRecordNotEmptyOutboxPreventsSecondSend(t *testing.T) {
	loc := mustLoadDigestZone(t)
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)

	dayStart := time.Date(2026, time.November, 1, 0, 0, 0, 0, loc)
	dayEnd := dayStart.AddDate(0, 0, 1)
	prevSlot := time.Date(2026, time.October, 31, 0, 5, 0, 0, loc)

	seedNotificationSettings(t, pool, true, settings.CadenceDaily, &prevSlot)
	artistID := insertTestArtist(t, pool, "dst-fallback-slotproof")
	firstID := insertPendingEventTyped(t, pool, artistID, "new_release", "dst-fallback-slotproof-1", "Fall Back Slot Proof First")

	var secondID int64
	var insertErr error
	sender := &fakeSender{}
	sender.fn = func(ctx context.Context, embed discord.Embed) error {
		if atomic.LoadInt32(&sender.calls) == 1 {
			id, err := insertPendingEventTypedRaw(pool, artistID, "new_release", "dst-fallback-slotproof-2", "Fall Back Slot Proof Second")
			if err != nil {
				insertErr = err
			} else {
				secondID = id
			}
		}
		return nil
	}

	svc := settings.NewService(q, loc)
	n := notifier.New(q, sender, svc, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	clock := newMutableClock(dayStart)
	tickCh, manual := newManualTickSource()
	sink := newSignalingSink(n)
	sched := notifier.NewDigestScheduler(sink, logger, clock.now, notifier.WithTickSource(manual))

	sweep(t, sched, sink, clock, tickCh, dayStart, dayEnd)

	if insertErr != nil {
		t.Fatalf("insert second event: %v", insertErr)
	}
	if secondID == 0 {
		t.Fatal("the second event was never inserted -- the fixture itself is broken")
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("send count = %d, want exactly 1 (the slot record, not an empty outbox, must be what prevents a second send)", got)
	}
	if !isNotified(t, pool, firstID) {
		t.Fatalf("first event %d was never acked", firstID)
	}
	if isNotified(t, pool, secondID) {
		t.Fatalf("second event %d was acked -- it must remain pending until the NEXT slot", secondID)
	}
}

// TestDigestScheduler_DST_Weekly proves the weekly cadence produces exactly
// one send per Friday-to-Friday local week across each week containing one
// of the four 2026/2027 DST transition dates.
func TestDigestScheduler_DST_Weekly(t *testing.T) {
	cases := []struct {
		name        string
		fridayYear  int
		fridayMonth time.Month
		fridayDay   int
	}{
		{"week_containing_2026-03-08_spring-forward", 2026, time.March, 6},
		{"week_containing_2026-11-01_fall-back", 2026, time.October, 30},
		{"week_containing_2027-03-14_spring-forward", 2027, time.March, 12},
		{"week_containing_2027-11-07_fall-back", 2027, time.November, 5},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loc := mustLoadDigestZone(t)
			pool := testutil.NewIsolatedTestPool(t, "notifier_test")
			q := sqlc.New(pool)

			weekStart := time.Date(tc.fridayYear, tc.fridayMonth, tc.fridayDay, 0, 0, 0, 0, loc)
			weekEnd := weekStart.AddDate(0, 0, 7)
			wantSlot := time.Date(tc.fridayYear, tc.fridayMonth, tc.fridayDay, 0, 5, 0, 0, loc)
			prevSlot := wantSlot.AddDate(0, 0, -7)

			seedNotificationSettings(t, pool, true, settings.CadenceWeekly, &prevSlot)
			artistID := insertTestArtist(t, pool, "dst-weekly")
			insertPendingEventTyped(t, pool, artistID, "new_release", "dst-weekly-ext", "DST Weekly Title")

			sender := &fakeSender{}
			svc := settings.NewService(q, loc)
			n := notifier.New(q, sender, svc, time.Millisecond, notifier.WithLocation(loc))
			logger, _ := newTestLogger()

			clock := newMutableClock(weekStart)
			tickCh, manual := newManualTickSource()
			sink := newSignalingSink(n)
			sched := notifier.NewDigestScheduler(sink, logger, clock.now, notifier.WithTickSource(manual))

			sweep(t, sched, sink, clock, tickCh, weekStart, weekEnd)

			if got := atomic.LoadInt32(&sender.calls); got != 1 {
				t.Fatalf("send count = %d, want exactly 1", got)
			}
			row, err := q.GetNotificationSettings(context.Background())
			if err != nil {
				t.Fatalf("get notification settings: %v", err)
			}
			if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(wantSlot) {
				t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, wantSlot)
			}
		})
	}
}
