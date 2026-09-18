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

// ---------------------------------------------------------------------
// Task 2: the grace-window matrix.
// ---------------------------------------------------------------------

// TestDigestScheduler_Catchup_RestartInsideGrace is ROADMAP criterion 3
// (D-10 + D-12): a freshly constructed scheduler whose clock reads a
// missed slot plus 2 hours must send exactly once from its immediate first
// check, before any tick source ever fires.
func TestDigestScheduler_Catchup_RestartInsideGrace(t *testing.T) {
	loc := time.UTC
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)

	slot := time.Date(2026, 6, 15, 0, 5, 0, 0, loc)
	prevSlot := slot.AddDate(0, 0, -1)
	seedNotificationSettings(t, pool, true, settings.CadenceDaily, &prevSlot)

	artistID := insertTestArtist(t, pool, "catchup-restart")
	insertPendingEventTyped(t, pool, artistID, "new_release", "catchup-restart-ext", "Catchup Restart Title")

	sender := &fakeSender{}
	svc := settings.NewService(q, loc)
	n := notifier.New(q, sender, svc, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	sink := newSignalingSink(n)
	restartNow := slot.Add(2 * time.Hour)
	sched := notifier.NewDigestScheduler(sink, logger, func() time.Time { return restartNow }, notifier.WithTickSource(neverFiringTickSource))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)
	waitSweepStep(t, sink.done)

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	if err := sched.Stop(stopCtx); err != nil {
		t.Fatalf("Stop: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (the immediate first check must catch up, before any tick)", got)
	}
	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(slot) {
		t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, slot)
	}
}

// TestDigestScheduler_Grace_InsideWindow_SlotPlus11h59m proves a check well
// inside the daily grace window still sends and writes both columns.
func TestDigestScheduler_Grace_InsideWindow_SlotPlus11h59m(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 16, 0, 5, 0, 0, loc)
	prevSlot := slot.AddDate(0, 0, -1)

	artistID := insertTestArtist(t, pool, "grace-inside")
	insertPendingEventTyped(t, pool, artistID, "new_release", "grace-inside-ext", "Grace Inside Title")

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, &prevSlot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	now := slot.Add(11*time.Hour + 59*time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1", got)
	}
	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(slot) {
		t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, slot)
	}
	if !row.DigestLastSentAt.Valid {
		t.Fatalf("digest_last_sent_at is NULL, want non-NULL")
	}
}

// TestDigestScheduler_Grace_Boundary_SlotPlus12h00m_StillSends pins the
// inclusive boundary: the rule is now-slot > grace, so equality with the
// daily grace window still sends.
func TestDigestScheduler_Grace_Boundary_SlotPlus12h00m_StillSends(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 17, 0, 5, 0, 0, loc)
	prevSlot := slot.AddDate(0, 0, -1)

	artistID := insertTestArtist(t, pool, "grace-boundary-daily")
	insertPendingEventTyped(t, pool, artistID, "new_release", "grace-boundary-daily-ext", "Grace Boundary Daily Title")

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, &prevSlot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	now := slot.Add(12 * time.Hour)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (now-slot == grace is inside the window, not outside it)", got)
	}
}

// TestDigestScheduler_Grace_Boundary_SlotPlus12h01m_DoesNotSend is the
// boundary's other side: one minute past the daily grace window sends
// nothing, logs exactly one Warn, and leaves both settings columns
// byte-identical to their pre-check values.
func TestDigestScheduler_Grace_Boundary_SlotPlus12h01m_DoesNotSend(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 18, 0, 5, 0, 0, loc)
	prevSlot := slot.AddDate(0, 0, -1)

	artistID := insertTestArtist(t, pool, "grace-expired-daily")
	eventID := insertPendingEventTyped(t, pool, artistID, "new_release", "grace-expired-daily-ext", "Grace Expired Daily Title")

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, &prevSlot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	now := slot.Add(12*time.Hour + time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if isNotified(t, pool, eventID) {
		t.Fatalf("event %d was acked past the grace window", eventID)
	}

	var warns []logRecord
	for _, r := range decodeLogRecords(t, buf) {
		if r.Level == "WARN" {
			warns = append(warns, r)
		}
	}
	if len(warns) != 1 {
		t.Fatalf("WARN record count = %d, want exactly 1: %+v", len(warns), warns)
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h00m_StillSends mirrors
// the daily inclusive-boundary case for the weekly cadence's 48h grace.
func TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h00m_StillSends(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 19, 0, 5, 0, 0, loc) // a Friday
	prevSlot := slot.AddDate(0, 0, -7)

	artistID := insertTestArtist(t, pool, "grace-boundary-weekly")
	insertPendingEventTyped(t, pool, artistID, "new_release", "grace-boundary-weekly-ext", "Grace Boundary Weekly Title")

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceWeekly, &prevSlot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	now := slot.Add(48 * time.Hour)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (now-slot == grace is inside the window, not outside it)", got)
	}
}

// TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h01m_DoesNotSend mirrors
// the daily past-window case for the weekly cadence's 48h grace: no send,
// exactly one Warn, no column writes.
func TestDigestScheduler_Grace_WeeklyBoundary_SlotPlus48h01m_DoesNotSend(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 26, 0, 5, 0, 0, loc) // a Friday
	prevSlot := slot.AddDate(0, 0, -7)

	artistID := insertTestArtist(t, pool, "grace-expired-weekly")
	eventID := insertPendingEventTyped(t, pool, artistID, "new_release", "grace-expired-weekly-ext", "Grace Expired Weekly Title")

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceWeekly, &prevSlot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	now := slot.Add(48*time.Hour + time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if isNotified(t, pool, eventID) {
		t.Fatalf("event %d was acked past the grace window", eventID)
	}

	var warns []logRecord
	for _, r := range decodeLogRecords(t, buf) {
		if r.Level == "WARN" {
			warns = append(warns, r)
		}
	}
	if len(warns) != 1 {
		t.Fatalf("WARN record count = %d, want exactly 1: %+v", len(warns), warns)
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestDigestScheduler_Grace_NothingLost_CarriesForwardToNextSlot is the
// assertion that turns "we did not send late" into "we did not drop
// anything": starting from a past-the-window state (0 sends, nothing
// written, the event still pending), an event accumulated during the
// skipped period is inserted, then the clock advances to the next daily
// slot. That single next-slot check must send exactly once, acking both
// the originally-pending event and the one that accumulated during the
// gap.
func TestDigestScheduler_Grace_NothingLost_CarriesForwardToNextSlot(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 15, 0, 5, 0, 0, loc)
	prevSlot := slot.AddDate(0, 0, -1)
	seedNotificationSettings(t, pool, true, settings.CadenceDaily, &prevSlot)

	artistID := insertTestArtist(t, pool, "nothing-lost")
	pendingID := insertPendingEventTyped(t, pool, artistID, "new_release", "nothing-lost-pending", "Pending During Skip")

	svc := settings.NewService(q, loc)
	sender := &fakeSender{}
	n := notifier.New(q, sender, svc, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	// Past the window: 0 sends, nothing written, the pending event carries.
	pastWindow := slot.Add(12*time.Hour + time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, pastWindow); err != nil {
		t.Fatalf("SendDigestIfDue (past window): %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls after the past-window check = %d, want 0", got)
	}
	if isNotified(t, pool, pendingID) {
		t.Fatalf("event %d was acked past the grace window", pendingID)
	}

	// Something new accumulates during the skipped period.
	accumulatedID := insertPendingEventTyped(t, pool, artistID, "new_release", "nothing-lost-accumulated", "Accumulated During Skip")

	nextSlotNow := slot.AddDate(0, 0, 1) // the next daily slot, exactly
	if err := n.SendDigestIfDue(context.Background(), logger, nextSlotNow); err != nil {
		t.Fatalf("SendDigestIfDue (next slot): %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls after the next-slot check = %d, want 1", got)
	}
	for _, id := range []int64{pendingID, accumulatedID} {
		if !isNotified(t, pool, id) {
			t.Fatalf("event %d was not acked by the next slot's send", id)
		}
	}
}

// TestDigestScheduler_NotDue_Quiet_NoLogsNoWrites proves the not-due branch
// is silent at Info level and above (D-24: Debug only) and touches
// neither settings column.
func TestDigestScheduler_NotDue_Quiet_NoLogsNoWrites(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	slot := time.Date(2026, 6, 20, 0, 5, 0, 0, loc)

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, &slot) // already handled up to this slot
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	now := slot.Add(10 * time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	for _, r := range decodeLogRecords(t, buf) {
		if r.Level == "INFO" || r.Level == "WARN" || r.Level == "ERROR" {
			t.Fatalf("unexpected %s record: %+v", r.Level, r)
		}
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}
