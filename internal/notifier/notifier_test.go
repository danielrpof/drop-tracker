package notifier_test

// This file follows internal/detection/detector_test.go's real-Postgres
// integration style (testutil.NewTestPool applies the embedded migrations)
// combined with internal/musicbrainz/search_test.go's httptest.Server style
// for the end-to-end drain case -- the tracer's proof that a pending events
// row travels poll cycle -> notifier -> Discord -> acked row.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	pool := testutil.NewTestPool(t)
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
	pool := testutil.NewTestPool(t)
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
	pool := testutil.NewTestPool(t)
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
	pool := testutil.NewTestPool(t)
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
