package notifier_test

// This file follows internal/detection/detector_test.go's real-Postgres
// integration style, combined with internal/musicbrainz/search_test.go's
// httptest.Server style for the end-to-end drain case -- the tracer's proof
// that a pending events row travels poll cycle -> notifier -> Discord ->
// acked row.
//
// Every test here uses testutil.NewIsolatedTestPool (not NewTestPool):
// NotifyPending's ListUnnotified is a deliberately global, unfiltered query
// (D-06), so a pool scoped to the shared fixture's default schema would let
// any other concurrently-running package's own pending events rows leak
// into this package's exact-count assertions -- and a real NotifyPending
// call here could even mark one of those foreign rows notified out from
// under its own test. NewIsolatedTestPool gives this package's tests their
// own dedicated schema, migrated independently, so counts here reflect only
// what this package's own tests inserted.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
	"github.com/danielrpof/drop-tracker/internal/notifier"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// newTestLogger builds a *slog.Logger writing newline-delimited JSON into
// buf, mirroring internal/poller/poller_test.go's convention, so a test can
// assert on log content (e.g. D-10's disabled line, D-09's failure log).
func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), buf
}

// fakeSender is a controllable double for notifier.Sender, mirroring
// internal/poller/poller_test.go's fakeReleaseGroupSource call-tracking
// convention.
type fakeSender struct {
	fn func(ctx context.Context, embed discord.Embed) error

	calls int32
}

func (f *fakeSender) Send(ctx context.Context, embed discord.Embed) error {
	atomic.AddInt32(&f.calls, 1)
	if f.fn != nil {
		return f.fn(ctx, embed)
	}
	return nil
}

var _ notifier.Sender = (*fakeSender)(nil)

// spacingRecorder installs a recording spacingWait seam for the duration of
// t (via notifier.SetSpacingWaitForTest) and returns a func reporting the
// durations NotifyPending's send loop requested. Each request is answered
// with an already-fired channel, so the select in the send loop never
// actually waits -- what is recorded is what NotifyPending asked for, which
// is deterministic, rather than how long a goroutine happened to sleep,
// which is not under CPU or scheduler contention.
func spacingRecorder(t *testing.T) func() []time.Duration {
	t.Helper()
	var mu sync.Mutex
	var recorded []time.Duration
	notifier.SetSpacingWaitForTest(t, func(d time.Duration) <-chan time.Time {
		mu.Lock()
		recorded = append(recorded, d)
		mu.Unlock()
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	})
	return func() []time.Duration {
		mu.Lock()
		defer mu.Unlock()
		return append([]time.Duration(nil), recorded...)
	}
}

// testArtistMBID derives a short, unique-per-test-and-suffix artist mbid,
// matching internal/poller/poller_test.go's testArtistMBID convention.
func testArtistMBID(t *testing.T, suffix string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name() + "|" + suffix))
	return "test-" + hex.EncodeToString(sum[:])[:12]
}

// insertTestArtist inserts a real artists row (events' foreign key target)
// and registers a t.Cleanup that deletes it -- events cascade-delete via
// events_artist_id_fkey's ON DELETE CASCADE, so no separate event cleanup
// is needed.
func insertTestArtist(t *testing.T, pool *pgxpool.Pool, suffix string) int64 {
	t.Helper()
	ctx := context.Background()
	mbid := testArtistMBID(t, suffix)

	var artistID int64
	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "Notifier Test Artist").Scan(&artistID); err != nil {
		t.Fatalf("insert test artist: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM artists WHERE id = $1", artistID); err != nil {
			t.Fatalf("cleanup: delete artist: %v", err)
		}
	})
	return artistID
}

// insertPendingEvent inserts a new_release events row with notified_at NULL
// -- exactly the outbox state NotifyPending is meant to drain.
func insertPendingEvent(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID string) int64 {
	t.Helper()
	ctx := context.Background()

	var eventID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		 VALUES ($1, 'musicbrainz', 'new_release', $2, 'Test Title', 'Test Artist')
		 RETURNING id`,
		artistID, externalID,
	).Scan(&eventID); err != nil {
		t.Fatalf("insert pending event: %v", err)
	}
	return eventID
}

// insertPendingEventTitled inserts a new_release events row with notified_at
// NULL and a caller-supplied title, so a fake Sender or a test server can key
// its behavior off which row is being sent -- formatEmbed always carries
// ev.Title verbatim (prefixed with an emoji) into the embed's Title field,
// making it the one reliably distinguishing value across a batch of rows.
func insertPendingEventTitled(t *testing.T, pool *pgxpool.Pool, artistID int64, externalID, title string) int64 {
	t.Helper()
	ctx := context.Background()

	var eventID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		 VALUES ($1, 'musicbrainz', 'new_release', $2, $3, 'Test Artist')
		 RETURNING id`,
		artistID, externalID, title,
	).Scan(&eventID); err != nil {
		t.Fatalf("insert pending event: %v", err)
	}
	return eventID
}

// isNotified reports whether eventID's notified_at is non-NULL.
func isNotified(t *testing.T, pool *pgxpool.Pool, eventID int64) bool {
	t.Helper()
	var notified bool
	if err := pool.QueryRow(context.Background(), "SELECT notified_at IS NOT NULL FROM events WHERE id = $1", eventID).Scan(&notified); err != nil {
		t.Fatalf("query notified_at: %v", err)
	}
	return notified
}

func TestNotifyPending_ZeroPendingRows_NoRequestNoMarkNoError(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	// Drain whatever is currently pending -- ListUnnotified is a global
	// query (D-06), so this proves a genuine zero-row pass rather than
	// relying on the table happening to already be empty.
	drain := &fakeSender{}
	if err := notifier.New(q, drain, time.Millisecond).NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("drain NotifyPending: %v", err)
	}

	sender := &fakeSender{}
	n := notifier.New(q, sender, time.Millisecond)
	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
}

func TestNotifyPending_OnePendingRow_204MarksNotified(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "e2e")
	eventID := insertPendingEvent(t, pool, artistID, "e2e-ext-1")

	var reqCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := discord.NewClient(ts.URL, ts.Client())
	n := notifier.New(q, client, time.Millisecond)

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v", err)
	}
	if got := atomic.LoadInt32(&reqCount); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if !isNotified(t, pool, eventID) {
		t.Fatal("notified_at is still NULL after a successful send")
	}
}

func TestNotifyPending_SendFails_LeavesNotifiedAtNullAndRePicksUpNextPass(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, buf := newTestLogger()

	artistID := insertTestArtist(t, pool, "failpath")
	eventID := insertPendingEvent(t, pool, artistID, "failpath-ext-1")

	failing := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		return errors.New("send exploded")
	}}
	n := notifier.New(q, failing, time.Millisecond)

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v, want nil (a send failure must not abort the pass, D-09)", err)
	}
	if isNotified(t, pool, eventID) {
		t.Fatal("notified_at must remain NULL after a failed send")
	}
	if !strings.Contains(buf.String(), "send exploded") {
		t.Fatalf("expected the send failure to be logged, got: %s", buf.String())
	}

	// A second pass re-selects the same row (D-09's re-pickup contract).
	succeeding := &fakeSender{}
	n2 := notifier.New(q, succeeding, time.Millisecond)
	if err := n2.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("second NotifyPending: %v", err)
	}
	if got := atomic.LoadInt32(&succeeding.calls); got != 1 {
		t.Fatalf("succeeding.calls = %d, want 1 (the previously-failed row must be re-selected)", got)
	}
	if !isNotified(t, pool, eventID) {
		t.Fatal("notified_at should be non-NULL after the second pass succeeds")
	}
}

func TestNotifyPending_ReentrantCallSkippedWhileInFlight(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "reentrant")
	insertPendingEvent(t, pool, artistID, "reentrant-ext-1")

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	n := notifier.New(q, sender, time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- n.NotifyPending(context.Background(), logger) }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first pass to block inside Send")
	}

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("reentrant NotifyPending: %v, want nil (D-06 CAS skip)", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (the reentrant call must issue zero requests)", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first NotifyPending: %v", err)
	}
}

func TestSelect_EmptyWebhookURL_ReturnsNoOpAndLogsDisabledLine(t *testing.T) {
	logger, buf := newTestLogger()

	sink := notifier.Select("", nil, nil, logger)
	if _, ok := sink.(notifier.NoOp); !ok {
		t.Fatalf("Select(\"\", ...) = %T, want notifier.NoOp", sink)
	}
	if !strings.Contains(strings.ToLower(buf.String()), "disabled") {
		t.Fatalf("expected a log line noting Discord notifications are disabled, got: %s", buf.String())
	}

	if err := sink.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NoOp.NotifyPending: %v, want nil", err)
	}
}

// TestNotifyPending_ConcurrentCallsNoDoublePost is the genuine two-goroutine
// proof D-06's CAS guard exists for: the first call is provably still
// inside Send (signaled via started, mirroring commit e53d48c's
// TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning idiom) when the second,
// independently-scheduled call is issued. A test that never gives the Go
// scheduler a chance to interleave the two calls would prove nothing about
// the race this guard exists to survive.
func TestNotifyPending_ConcurrentCallsNoDoublePost(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "concurrent")
	eventID := insertPendingEvent(t, pool, artistID, "concurrent-ext-1")

	release := make(chan struct{})
	started := make(chan struct{}, 1)
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return nil
	}}
	n := notifier.New(q, sender, time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- n.NotifyPending(context.Background(), logger) }()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the first call to block inside Send")
	}

	// Issued from the test's own goroutine while the first call is provably
	// still blocked inside Send.
	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("concurrent NotifyPending: %v, want nil (D-06 CAS skip)", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls immediately after the concurrent call returns = %d, want 1 (the CAS guard, not luck, must have prevented a second send)", got)
	}

	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first NotifyPending: %v", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls after both calls completed = %d, want exactly 1", got)
	}
	if !isNotified(t, pool, eventID) {
		t.Fatal("notified_at should be non-NULL after the first call completes")
	}
}

// TestNotifyPending_BatchSpacingBetweenSends proves D-07's inter-send
// spacing by asserting which spacing durations NotifyPending requested
// through the spacingWait seam, not by measuring elapsed wall-clock time
// across a real multi-row batch. An elapsed-time lower bound is also
// satisfied by an unrelated slow machine (a false pass) and can be missed
// under CPU or scheduler contention (a false failure); asserting the
// requested duration directly is both a stronger and a deterministic form
// of the same property.
func TestNotifyPending_BatchSpacingBetweenSends(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "spacing")
	insertPendingEvent(t, pool, artistID, "spacing-ext-1")
	insertPendingEvent(t, pool, artistID, "spacing-ext-2")
	insertPendingEvent(t, pool, artistID, "spacing-ext-3")

	sender := &fakeSender{}

	const spacing = 50 * time.Millisecond
	recorded := spacingRecorder(t)
	n := notifier.New(q, sender, spacing)
	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 3 {
		t.Fatalf("sender.calls = %d, want 3", got)
	}

	got := recorded()
	if len(got) != 2 {
		t.Fatalf("recorded %d spacing requests, want exactly 2 (three rows, spacing skipped after the last): %v", len(got), got)
	}
	for i, d := range got {
		if d != spacing {
			t.Fatalf("spacing request %d = %v, want exactly %v", i, d, spacing)
		}
	}
}

// markNotifiedFailingQuerier wraps a real sqlc.Querier and overrides only
// MarkNotified to always fail, delegating every other method (ListUnnotified
// included) to the real, Postgres-backed querier -- lets a test land
// squarely in the narrow "Send succeeded, MarkNotified failed" window WR-03
// describes without hand-writing every other Querier method.
type markNotifiedFailingQuerier struct {
	sqlc.Querier
}

func (markNotifiedFailingQuerier) MarkNotified(ctx context.Context, id int64) (int64, error) {
	return 0, errors.New("mark notified exploded")
}

// TestNotifyPending_MarkNotifiedFails_LogsWarnAndReturnsError is the WR-03
// regression guard: a MarkNotified failure after a successful Send is a
// known duplicate-post risk window (the row's notified_at stays NULL, so
// the next pass re-sends it) -- this must be logged at Warn, distinguishable
// from a generic DB-outage error, and still returned as a hard failure.
func TestNotifyPending_MarkNotifiedFails_LogsWarnAndReturnsError(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, buf := newTestLogger()

	artistID := insertTestArtist(t, pool, "marknotifiedfail")
	eventID := insertPendingEvent(t, pool, artistID, "marknotifiedfail-ext-1")

	sender := &fakeSender{}
	n := notifier.New(markNotifiedFailingQuerier{Querier: q}, sender, time.Millisecond)

	err := n.NotifyPending(context.Background(), logger)
	if err == nil {
		t.Fatal("NotifyPending: expected an error when MarkNotified fails, got nil")
	}
	if !strings.Contains(err.Error(), "mark notified exploded") {
		t.Fatalf("error = %q, want it to wrap the MarkNotified failure", err.Error())
	}
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (Send must have succeeded before MarkNotified failed)", got)
	}
	if isNotified(t, pool, eventID) {
		t.Fatal("notified_at should still be NULL: MarkNotified failed, so the DB was never updated")
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "mark notified exploded") {
		t.Fatalf("expected the MarkNotified failure to be logged, got: %s", logOut)
	}
	if !strings.Contains(strings.ToLower(logOut), `"level":"warn"`) {
		t.Fatalf("expected a Warn-level log line for the MarkNotified failure, got: %s", logOut)
	}
}

// TestNotifyPending_SpacingAppliedEvenAfterFailedSend is the WR-01
// regression guard: the inter-send spacing wait must not be skipped on a
// failed Send, since a backlog of failing sends (e.g. during a Discord
// outage) is exactly the scenario D-07's pacing exists to protect. It
// asserts the requested spacing duration through the spacingWait seam
// rather than measuring elapsed wall-clock time -- deterministic under CPU
// or scheduler contention, where an elapsed-time lower bound is not.
func TestNotifyPending_SpacingAppliedEvenAfterFailedSend(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "failspacing")
	insertPendingEvent(t, pool, artistID, "failspacing-ext-1")
	insertPendingEvent(t, pool, artistID, "failspacing-ext-2")
	insertPendingEvent(t, pool, artistID, "failspacing-ext-3")

	failing := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		return errors.New("send exploded")
	}}

	const spacing = 50 * time.Millisecond
	recorded := spacingRecorder(t)
	n := notifier.New(q, failing, spacing)
	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v, want nil (a send failure must not abort the pass, D-09)", err)
	}

	if got := atomic.LoadInt32(&failing.calls); got != 3 {
		t.Fatalf("failing.calls = %d, want 3 (all three rows must still be attempted)", got)
	}

	got := recorded()
	if len(got) != 2 {
		t.Fatalf("recorded %d spacing requests, want exactly 2 on the all-failed path (spacing must not be skipped after a failed Send): %v", len(got), got)
	}
	for i, d := range got {
		if d != spacing {
			t.Fatalf("spacing request %d = %v, want exactly %v", i, d, spacing)
		}
	}
}

// TestNotifyPending_BatchMidFailureContinuesToLaterRows is the load-bearing
// proof the plan's prohibition calls out: it does not stop at "the pass
// returned nil" (a regression that aborted after the first failure would
// also return nil) -- it positively confirms the row after the failed one
// was both attempted and delivered.
func TestNotifyPending_BatchMidFailureContinuesToLaterRows(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "midfail")
	id1 := insertPendingEventTitled(t, pool, artistID, "midfail-ext-1", "Row One")
	id2 := insertPendingEventTitled(t, pool, artistID, "midfail-ext-2", "Row Two")
	id3 := insertPendingEventTitled(t, pool, artistID, "midfail-ext-3", "Row Three")

	var callCount int32
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		atomic.AddInt32(&callCount, 1)
		if strings.Contains(embed.Title, "Row Two") {
			return errors.New("row two send exploded")
		}
		return nil
	}}
	n := notifier.New(q, sender, time.Millisecond)

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v, want nil (a mid-batch send failure must not abort the pass)", err)
	}

	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Fatalf("sender was called %d times, want 3 (all three rows must be attempted, not just the first)", got)
	}
	if !isNotified(t, pool, id1) {
		t.Fatal("row one's notified_at should be non-NULL")
	}
	if isNotified(t, pool, id2) {
		t.Fatal("row two's notified_at must remain NULL after its send failed")
	}
	if !isNotified(t, pool, id3) {
		t.Fatal("row three's notified_at should be non-NULL -- it must still be attempted and delivered after row two's failure")
	}
}

// TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows exercises
// the batch-level 429/Retry-After path (D-08) through a real discord.Client
// against one httptest.Server, since the retry handling lives inside
// discord.Client.sendAttempt, not the notifier's loop. Row two's first
// request gets a 429 with a small retry_after; its retry and rows one/three
// all get 204. Every row must still be delivered exactly once, in order.
func TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "retryafter")
	id1 := insertPendingEventTitled(t, pool, artistID, "retryafter-ext-1", "Retry Row One")
	id2 := insertPendingEventTitled(t, pool, artistID, "retryafter-ext-2", "Retry Row Two")
	id3 := insertPendingEventTitled(t, pool, artistID, "retryafter-ext-3", "Retry Row Three")

	type reqRecord struct {
		title string
		at    time.Time
	}
	var mu sync.Mutex
	var requests []reqRecord
	row2Attempts := 0

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Embeds []struct {
				Title string `json:"title"`
			} `json:"embeds"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		title := ""
		if len(payload.Embeds) > 0 {
			title = payload.Embeds[0].Title
		}

		mu.Lock()
		requests = append(requests, reqRecord{title: title, at: time.Now()})
		isRowTwo := strings.Contains(title, "Retry Row Two")
		if isRowTwo {
			row2Attempts++
		}
		attempt := row2Attempts
		mu.Unlock()

		if isRowTwo && attempt == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited","retry_after":0.05,"global":false}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := discord.NewClient(ts.URL, ts.Client())
	n := notifier.New(q, client, time.Millisecond)

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("NotifyPending: %v", err)
	}

	if !isNotified(t, pool, id1) || !isNotified(t, pool, id2) || !isNotified(t, pool, id3) {
		t.Fatal("all three rows should have non-NULL notified_at after the pass")
	}

	mu.Lock()
	got := append([]reqRecord(nil), requests...)
	mu.Unlock()
	if len(got) != 4 {
		t.Fatalf("server recorded %d requests, want 4 (three rows + one 429 retry)", len(got))
	}

	// Confirm row three's request arrived after row two's retry completed --
	// proof the retry wait actually happened without preventing row three
	// from being attempted afterward.
	var row2RetryAt, row3At time.Time
	for _, r := range got {
		switch {
		case strings.Contains(r.title, "Retry Row Two"):
			row2RetryAt = r.at // last write wins: the retry (second) request
		case strings.Contains(r.title, "Retry Row Three"):
			row3At = r.at
		}
	}
	if row2RetryAt.IsZero() || row3At.IsZero() {
		t.Fatalf("did not observe both row two's retry and row three's request: %+v", got)
	}
	if !row3At.After(row2RetryAt) {
		t.Fatalf("row three's request (%v) should arrive after row two's retry completed (%v)", row3At, row2RetryAt)
	}
}

// TestNotifyPending_CrossCycleRecoveryAfterOutage proves D-09's cross-cycle
// re-pickup with two real, separate NotifyPending calls: the first, against
// a webhook that always answers 500, must leave notified_at NULL; the
// second, wholly separate call -- standing in for the next poll cycle,
// after the webhook recovers -- must find and deliver the same row. No
// retry-count or give-up state may prevent this.
func TestNotifyPending_CrossCycleRecoveryAfterOutage(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	logger, _ := newTestLogger()

	artistID := insertTestArtist(t, pool, "crosscycle")
	eventID := insertPendingEvent(t, pool, artistID, "crosscycle-ext-1")

	var mu sync.Mutex
	healthy := false
	var successCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := healthy
		mu.Unlock()
		if !h {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		atomic.AddInt32(&successCount, 1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	client := discord.NewClient(ts.URL, ts.Client())
	n := notifier.New(q, client, time.Millisecond)

	// First call: the webhook is down for the whole pass.
	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("first NotifyPending: %v, want nil (D-09: a failed send must not fail the pass)", err)
	}
	if isNotified(t, pool, eventID) {
		t.Fatal("notified_at must still be NULL after a pass against a failing webhook")
	}

	// Recovery: a second, wholly separate NotifyPending call after the
	// webhook comes back up.
	mu.Lock()
	healthy = true
	mu.Unlock()

	if err := n.NotifyPending(context.Background(), logger); err != nil {
		t.Fatalf("second NotifyPending: %v", err)
	}
	if got := atomic.LoadInt32(&successCount); got != 1 {
		t.Fatalf("successful request count = %d, want 1", got)
	}
	if !isNotified(t, pool, eventID) {
		t.Fatal("notified_at should be non-NULL after the recovery pass")
	}
}
