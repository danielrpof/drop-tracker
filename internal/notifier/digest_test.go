package notifier_test

// digest_test.go proves the tracer's end-to-end path (a due slot turns the
// outbox into one Discord embed and one atomic ack) plus every D-17 branch,
// against real Postgres -- mirroring notifier_test.go's real-Postgres style
// and reusing its fixture helpers (insertTestArtist, insertPendingEventTyped,
// insertPendingEventDated, isNotified, newTestLogger, stubSettings,
// erroringSettings) since this file shares the same notifier_test package.

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
	"github.com/danielrpof/drop-tracker/internal/notifier"
	"github.com/danielrpof/drop-tracker/internal/settings"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// digestSettingsReader returns a SettingsReader reporting a fixed
// DigestEnabled/DigestCadence/DigestLastSlotAt triple on every Get call --
// digestSettingsReader.
func digestSettingsReader(enabled bool, cadence settings.Cadence, lastSlot *time.Time) *fakeSettingsReader {
	return &fakeSettingsReader{fn: func(ctx context.Context) (settings.Settings, error) {
		return settings.Settings{DigestEnabled: enabled, DigestCadence: cadence, DigestLastSlotAt: lastSlot}, nil
	}}
}

// flippingDigestSettings mirrors flippingSettings' k-th-call mode flip
// (notifier_test.go), but reports the fuller Settings shape SendDigestIfDue
// reads: enabled for calls 1..k-1, disabled from call k onward, with a
// fixed cadence/lastSlot throughout -- the mid-pass re-read-turns-off case
// (D-17 step 5, T-22-12).
func flippingDigestSettings(k int, cadence settings.Cadence, lastSlot *time.Time) *fakeSettingsReader {
	r := &fakeSettingsReader{}
	r.fn = func(ctx context.Context) (settings.Settings, error) {
		enabled := r.Calls() < int32(k)
		return settings.Settings{DigestEnabled: enabled, DigestCadence: cadence, DigestLastSlotAt: lastSlot}, nil
	}
	return r
}

// settingsSnapshot is a comparable capture of the two digest-scheduling
// columns AckDigestBatch/UpdateNotificationSettings can write, so a test can
// assert "nothing changed" with one struct comparison.
type settingsSnapshot struct {
	slotValid bool
	slotTime  time.Time
	sentValid bool
	sentTime  time.Time
}

// snapshotSettings reads notification_settings back through q, per this
// plan's instruction to assert column state through sqlc.Querier directly.
func snapshotSettings(t *testing.T, q sqlc.Querier) settingsSnapshot {
	t.Helper()
	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	return settingsSnapshot{
		slotValid: row.DigestLastSlotAt.Valid,
		slotTime:  row.DigestLastSlotAt.Time,
		sentValid: row.DigestLastSentAt.Valid,
		sentTime:  row.DigestLastSentAt.Time,
	}
}

func (s settingsSnapshot) equal(other settingsSnapshot) bool {
	return s.slotValid == other.slotValid && s.slotTime.Equal(other.slotTime) &&
		s.sentValid == other.sentValid && s.sentTime.Equal(other.sentTime)
}

// TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns is
// the tracer's core proof: a due slot with one pending event of each type
// produces exactly one Send call carrying all three under headings, acks
// all three rows, and writes both settings columns.
func TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 15, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "all-three")
	newReleaseID := insertPendingEventTyped(t, pool, artistID, "new_release", "nr-all-three", "New Release Title")
	guestID := insertPendingEventTyped(t, pool, artistID, "guest_feature", "gf-all-three", "Guest Feature Title")
	deluxeID := insertPendingEventTyped(t, pool, artistID, "deluxe_change", "dc-all-three", "Deluxe Change Title")

	var gotEmbed discord.Embed
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		gotEmbed = embed
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1", got)
	}
	for _, want := range []string{"**New Releases**", "**Guest Features**", "**Deluxe Changes**"} {
		if !strings.Contains(gotEmbed.Description, want) {
			t.Fatalf("embed Description missing %q:\n%s", want, gotEmbed.Description)
		}
	}
	for _, id := range []int64{newReleaseID, guestID, deluxeID} {
		if !isNotified(t, pool, id) {
			t.Fatalf("event %d not marked notified", id)
		}
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
}

// chunkForcingEventCount is a fixture size empirically confirmed (whitebox
// probe against chunkDigest, digest_chunk_test.go's syntheticEvents shape)
// to split into exactly 3 chunks across a wide stable margin (70-87 events
// all produce 3; 75 sits comfortably inside it) -- deterministic without
// this external test package reaching into unexported chunker internals.
const chunkForcingEventCount = 75

// insertPendingEventsForChunking inserts n new_release rows shaped like
// digest_chunk_test.go's syntheticEvents (same title/artist padding), so
// this package's real-Postgres SendDigestIfDue tests split into the same
// chunk count the whitebox chunker tests already pin.
func insertPendingEventsForChunking(t *testing.T, pool *pgxpool.Pool, artistID int64, n int) []int64 {
	t.Helper()
	ids := make([]int64, n)
	for i := 0; i < n; i++ {
		externalID := fmt.Sprintf("chunk-%04d", i)
		title := fmt.Sprintf("Album Title Number %04d With Extra Padding Text To Grow The Line", i)
		ids[i] = insertPendingEventTyped(t, pool, artistID, "new_release", externalID, title)
	}
	return ids
}

// capForcingEventCount is a fixture size empirically confirmed (whitebox
// probe against chunkDigest, digest_chunk_test.go's syntheticEvents shape)
// to split into 22 uncapped chunks -- comfortably over maxDigestChunks (20,
// D-23) so a real-Postgres SendDigestIfDue call actually exercises the cap
// rather than merely the multi-chunk path chunkForcingEventCount covers.
const capForcingEventCount = 600

// countNotified reports how many of ids currently have a non-NULL
// notified_at.
func countNotified(t *testing.T, pool *pgxpool.Pool, ids []int64) int {
	t.Helper()
	n := 0
	for _, id := range ids {
		if isNotified(t, pool, id) {
			n++
		}
	}
	return n
}

// TestSendDigestIfDue_MultiChunk_AllSucceed_ThreeSendsAllAckedBothColumnsAdvanceOnce
// is success criterion 2's proof: a sendable set too large for one message
// sends more than one Discord message, and every event acks exactly once,
// with both settings columns advanced exactly once by the final chunk's ack
// (D-14).
func TestSendDigestIfDue_MultiChunk_AllSucceed_ThreeSendsAllAckedBothColumnsAdvanceOnce(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 1, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "multi-chunk-all-succeed")
	ids := insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	var gotDescriptions []string
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		gotDescriptions = append(gotDescriptions, embed.Description)
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 3 {
		t.Fatalf("sender.calls = %d, want 3", got)
	}
	for i, desc := range gotDescriptions {
		if rc := utf8.RuneCountInString(desc); rc > 4096 {
			t.Fatalf("chunk %d description has %d runes, want <= 4096", i, rc)
		}
		if !strings.HasPrefix(desc, "Everything pending since digest mode was enabled") {
			t.Fatalf("chunk %d description does not open with the window header:\n%s", i, desc)
		}
	}
	if got := countNotified(t, pool, ids); got != len(ids) {
		t.Fatalf("notified count = %d, want %d (every event acked exactly once)", got, len(ids))
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
}

// TestSendDigestIfDue_MultiChunk_FailureAtChunk2_PartialAckBothColumnsUnchanged
// is success criterion 3's proof and D-14/D-15's load-bearing assertion: a
// sender failure on the second of three chunks leaves the remainder
// (including a suppressed id, D-15) pending and neither settings column
// advances -- the digest stays due for the next check.
func TestSendDigestIfDue_MultiChunk_FailureAtChunk2_PartialAckBothColumnsUnchanged(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 2, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "multi-chunk-fail-at-2")
	ids := insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)
	// releaseDate == "" inserts SQL NULL -- suppressed, never sent, must
	// still stay pending after a mid-run failure (D-15).
	suppressedID := insertPendingEventDated(t, pool, artistID, "chunk-suppressed", "Suppressed Title", "")

	before := snapshotSettings(t, q)

	var sender *fakeSender
	sender = &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		if atomic.LoadInt32(&sender.calls) == 2 {
			return errors.New("discord unavailable")
		}
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 2 {
		t.Fatalf("sender.calls = %d, want 2", got)
	}
	if !strings.Contains(buf.String(), "digest send failed") {
		t.Fatalf("expected send-failure Error log, got: %s", buf.String())
	}

	notified := countNotified(t, pool, ids)
	if notified == 0 || notified == len(ids) {
		t.Fatalf("notified count = %d, want strictly between 0 and %d (chunk 1 acked, chunks 2-3 still pending)", notified, len(ids))
	}
	if isNotified(t, pool, suppressedID) {
		t.Fatalf("suppressed event %d unexpectedly acked after a mid-run failure -- suppressed ids ride the final ack only (D-15)", suppressedID)
	}

	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed after a partial failure: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_MultiChunk_RetryAfterFailureSendsRemainder is D-12's
// proof: a second SendDigestIfDue call after a partial failure is an
// ordinary call that rebuilds fresh from whatever is still un-acked and
// delivers it -- no state tracks which chunk failed.
func TestSendDigestIfDue_MultiChunk_RetryAfterFailureSendsRemainder(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 3, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "multi-chunk-retry")
	ids := insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	var failingSender *fakeSender
	failingSender = &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		if atomic.LoadInt32(&failingSender.calls) == 2 {
			return errors.New("discord unavailable")
		}
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n1 := notifier.New(q, failingSender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n1.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("first SendDigestIfDue: %v, want nil", err)
	}
	if got := countNotified(t, pool, ids); got == len(ids) {
		t.Fatalf("notified count = %d after the first (partial-failure) call, want < %d", got, len(ids))
	}

	succeedingSender := &fakeSender{}
	n2 := notifier.New(q, succeedingSender, reader, time.Millisecond, notifier.WithLocation(loc))
	if err := n2.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("second SendDigestIfDue: %v, want nil", err)
	}

	if got := countNotified(t, pool, ids); got != len(ids) {
		t.Fatalf("notified count = %d after the retry, want %d -- the retry must deliver the remainder", got, len(ids))
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
}

// TestSendDigestIfDue_MultiChunk_SpacingRequestedLenMinus1Times proves
// consecutive chunk sends request digestChunkSpacing through the injectable
// seam exactly len(chunks)-1 times (D-23).
func TestSendDigestIfDue_MultiChunk_SpacingRequestedLenMinus1Times(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 4, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "multi-chunk-spacing")
	insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	var mu sync.Mutex
	var requested []time.Duration
	fired := make(chan time.Time)
	close(fired)
	notifier.SetDigestChunkWaitForTest(t, func(d time.Duration) <-chan time.Time {
		mu.Lock()
		requested = append(requested, d)
		mu.Unlock()
		return fired
	})

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	calls := atomic.LoadInt32(&sender.calls)
	if calls < 2 {
		t.Fatalf("sender.calls = %d, want >= 2 to exercise inter-chunk spacing", calls)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requested) != int(calls)-1 {
		t.Fatalf("digestChunkWait requested %d times, want %d (len(chunks)-1)", len(requested), calls-1)
	}
	for _, d := range requested {
		if d != time.Second {
			t.Fatalf("digestChunkWait requested duration = %v, want the digest-specific 1s constant (D-23)", d)
		}
	}
}

// TestSendDigestIfDue_MultiChunk_ContextCancelledAtBoundary_StopsCleanly
// proves a cancelled context at a chunk boundary stops the loop between
// chunks: the sender receives no further call and no ack advances the slot
// (D-26).
func TestSendDigestIfDue_MultiChunk_ContextCancelledAtBoundary_StopsCleanly(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 5, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "multi-chunk-ctx-cancel")
	insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	before := snapshotSettings(t, q)

	// A channel that never fires: only ctx.Done() can win the inter-chunk
	// select, making the cancellation deterministic rather than a race.
	neverFires := make(chan time.Time)
	notifier.SetDigestChunkWaitForTest(t, func(time.Duration) <-chan time.Time { return neverFires })

	ctx, cancel := context.WithCancel(context.Background())
	var sender *fakeSender
	sender = &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		if atomic.LoadInt32(&sender.calls) == 1 {
			cancel()
		}
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(ctx, logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("sender.calls = %d, want 1 (no further call after the boundary cancellation)", got)
	}
	if !strings.Contains(buf.String(), "chunk loop stopped at a chunk boundary") {
		t.Fatalf("expected the boundary-cancellation Warn log, got: %s", buf.String())
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed after a boundary cancellation: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_Cap_ExactlyMaxChunksSentBothColumnsUnchangedRemainderPending
// is D-23/D-24's proof: a batch producing more than maxDigestChunks chunks
// sends exactly 20 messages, leaves the deferred remainder (including a
// suppressed id, D-15) pending, stamps the last delivered chunk with D-22's
// remainder marker and a Total = maxDigestChunks indicator, and leaves both
// settings columns byte-identical to their pre-call snapshot -- the digest
// stays due for the next check.
func TestSendDigestIfDue_Cap_ExactlyMaxChunksSentBothColumnsUnchangedRemainderPending(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 10, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "cap-exact")
	ids := insertPendingEventsForChunking(t, pool, artistID, capForcingEventCount)
	// releaseDate == "" inserts SQL NULL -- suppressed, must still stay
	// pending after a capped run (D-15).
	suppressedID := insertPendingEventDated(t, pool, artistID, "cap-suppressed", "Suppressed Title", "")

	before := snapshotSettings(t, q)

	fired := make(chan time.Time)
	close(fired)
	notifier.SetDigestChunkWaitForTest(t, func(time.Duration) <-chan time.Time { return fired })

	var gotDescriptions []string
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		gotDescriptions = append(gotDescriptions, embed.Description)
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	const wantCappedSends = 20 // mirrors notifier's unexported maxDigestChunks (D-23)
	if got := atomic.LoadInt32(&sender.calls); got != wantCappedSends {
		t.Fatalf("sender.calls = %d, want %d", got, wantCappedSends)
	}

	notified := countNotified(t, pool, ids)
	if notified == 0 || notified == len(ids) {
		t.Fatalf("notified count = %d, want strictly between 0 and %d -- a capped run delivers some but not all", notified, len(ids))
	}
	if isNotified(t, pool, suppressedID) {
		t.Fatalf("suppressed event %d unexpectedly acked after a capped run -- suppressed ids ride the final ack only (D-15)", suppressedID)
	}

	last := gotDescriptions[len(gotDescriptions)-1]
	if !strings.Contains(last, "still pending, continuing in the next digest") {
		t.Fatalf("last delivered chunk description = %q, want it to carry the remainder marker (D-22)", last)
	}
	if !strings.Contains(last, "(20/20)") {
		t.Fatalf("last delivered chunk description = %q, want the position indicator to read Total = maxDigestChunks (D-22)", last)
	}

	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed after a capped run: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_Cap_SecondCallDrainsRemainderBothColumnsAdvanceOnce is
// D-12/D-24's self-drain proof: a second SendDigestIfDue call at a later
// now still inside the grace window delivers the remainder from a capped
// first run, and on the run that finally exhausts the outbox both settings
// columns advance exactly once.
func TestSendDigestIfDue_Cap_SecondCallDrainsRemainderBothColumnsAdvanceOnce(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 11, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "cap-drain")
	ids := insertPendingEventsForChunking(t, pool, artistID, capForcingEventCount)
	suppressedID := insertPendingEventDated(t, pool, artistID, "cap-drain-suppressed", "Suppressed Title", "")

	before := snapshotSettings(t, q)

	fired := make(chan time.Time)
	close(fired)
	notifier.SetDigestChunkWaitForTest(t, func(time.Duration) <-chan time.Time { return fired })

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("first SendDigestIfDue: %v, want nil", err)
	}
	if got := countNotified(t, pool, ids); got == len(ids) {
		t.Fatalf("notified count = %d after the first (capped) call, want < %d", got, len(ids))
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed after the first (capped) call: before=%+v after=%+v", before, after)
	}

	// Same slot is still due (D-24) -- an ordinary second call rebuilds
	// fresh from whatever is still un-acked (D-12), which now fits well
	// under the cap.
	now2 := now.Add(time.Minute)
	if err := n.SendDigestIfDue(context.Background(), logger, now2); err != nil {
		t.Fatalf("second SendDigestIfDue: %v, want nil", err)
	}

	if got := countNotified(t, pool, ids); got != len(ids) {
		t.Fatalf("notified count = %d after the second call, want %d -- the remainder must fully drain", got, len(ids))
	}
	if !isNotified(t, pool, suppressedID) {
		t.Fatalf("suppressed event %d not acked by the completing run (D-15)", suppressedID)
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
}

// TestSendDigestIfDue_Budget_ExceededAtBoundaryStopsPartialAckBothColumnsUnchanged
// is D-25's proof: the wall-clock budget check at a chunk boundary stops
// the loop before the next send when the injectable clock has advanced
// past the deadline, leaving the remainder pending and both settings
// columns byte-identical to their pre-call snapshot.
func TestSendDigestIfDue_Budget_ExceededAtBoundaryStopsPartialAckBothColumnsUnchanged(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 20, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "budget-exceeded")
	ids := insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	before := snapshotSettings(t, q)

	fired := make(chan time.Time)
	close(fired)
	notifier.SetDigestChunkWaitForTest(t, func(time.Duration) <-chan time.Time { return fired })

	// The clock seam reports now (not past the deadline) until the second
	// send completes, at which point it jumps well past digestSendBudget --
	// simulating "budget exceeded between chunk 2 and chunk 3" without
	// waiting out a real ~3-minute window.
	var pastDeadline atomic.Bool
	notifier.SetDigestNowForTest(t, func() time.Time {
		if pastDeadline.Load() {
			return now.Add(4 * time.Minute)
		}
		return now
	})

	var sender *fakeSender
	sender = &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		if atomic.LoadInt32(&sender.calls) == 2 {
			pastDeadline.Store(true)
		}
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}

	if got := atomic.LoadInt32(&sender.calls); got != 2 {
		t.Fatalf("sender.calls = %d, want 2 (the budget stops the loop before a third send)", got)
	}
	if !strings.Contains(buf.String(), "digest send budget exceeded") {
		t.Fatalf("expected budget-exceeded Warn log, got: %s", buf.String())
	}

	notified := countNotified(t, pool, ids)
	if notified == 0 || notified == len(ids) {
		t.Fatalf("notified count = %d, want strictly between 0 and %d (chunks 1-2 acked, the remainder pending)", notified, len(ids))
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed after a budget stop: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_Budget_NotExceededNormalRunCompletes proves the
// budget check never fires on the common path -- a fast test never
// advances real wall-clock time anywhere near digestSendBudget, so the
// default digestNow seam (real time.Now) lets a multi-chunk run complete
// normally.
func TestSendDigestIfDue_Budget_NotExceededNormalRunCompletes(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 21, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "budget-not-exceeded")
	ids := insertPendingEventsForChunking(t, pool, artistID, chunkForcingEventCount)

	fired := make(chan time.Time)
	close(fired)
	notifier.SetDigestChunkWaitForTest(t, func(time.Duration) <-chan time.Time { return fired })

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if strings.Contains(buf.String(), "digest send budget exceeded") {
		t.Fatalf("unexpected budget-exceeded Warn log on a normal run: %s", buf.String())
	}
	if got := countNotified(t, pool, ids); got != len(ids) {
		t.Fatalf("notified count = %d, want %d", got, len(ids))
	}
	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(wantSlot) {
		t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, wantSlot)
	}
}

// TestSendDigestIfDue_Budget_SendContextNeverCarriesBudgetDeadline pins
// D-25's central prohibition: the context handed to Sender.Send is never
// wrapped with a deadline derived from digestSendBudget -- cancelling an
// in-flight POST would create an accepted-but-unacked duplicate, so the
// budget is checked only at chunk boundaries, never against the send
// itself.
func TestSendDigestIfDue_Budget_SendContextNeverCarriesBudgetDeadline(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 7, 22, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "budget-no-ctx-deadline")
	insertPendingEventTyped(t, pool, artistID, "new_release", "budget-nctx", "Budget No Ctx Deadline")

	var gotCtx context.Context
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		gotCtx = ctx
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if gotCtx == nil {
		t.Fatal("sender.Send was never called")
	}
	if _, ok := gotCtx.Deadline(); ok {
		t.Fatalf("sender received a context with a deadline, want none -- the budget (D-25) must never wrap the POST's context")
	}
}

// TestSendDigestIfDue_DueSlotOnlyNewRelease_OtherHeadingsOmitted proves a
// heading is omitted entirely when its group has zero sendable events.
func TestSendDigestIfDue_DueSlotOnlyNewRelease_OtherHeadingsOmitted(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 16, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "only-new-release")
	insertPendingEventTyped(t, pool, artistID, "new_release", "nr-only", "Solo New Release")

	var gotEmbed discord.Embed
	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		gotEmbed = embed
		return nil
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if !strings.Contains(gotEmbed.Description, "**New Releases**") {
		t.Fatalf("embed missing New Releases heading:\n%s", gotEmbed.Description)
	}
	for _, unwanted := range []string{"**Guest Features**", "**Deluxe Changes**"} {
		if strings.Contains(gotEmbed.Description, unwanted) {
			t.Fatalf("embed unexpectedly contains %q:\n%s", unwanted, gotEmbed.Description)
		}
	}
}

// TestSendDigestIfDue_EmptyOutbox_ZeroSendSlotAdvancesSentAtStaysNull covers
// DGST-09's zero-event skip: a due slot with an empty outbox sends nothing,
// still advances the slot record, and leaves the watermark untouched.
func TestSendDigestIfDue_EmptyOutbox_ZeroSendSlotAdvancesSentAtStaysNull(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 17, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if !strings.Contains(buf.String(), "digest slot had nothing to send") {
		t.Fatalf("expected empty-skip Info log, got: %s", buf.String())
	}

	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(wantSlot) {
		t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, wantSlot)
	}
	if row.DigestLastSentAt.Valid {
		t.Fatalf("digest_last_sent_at = %+v, want NULL", row.DigestLastSentAt)
	}
}

// TestSendDigestIfDue_AllSuppressed_ZeroSendAckedSlotAdvancesSentAtStaysNull
// covers DGST-09's "zero accumulated events" encoding: ListUnnotified
// returns a row, but suppresses() rejects it -- still counts as an empty
// digest.
func TestSendDigestIfDue_AllSuppressed_ZeroSendAckedSlotAdvancesSentAtStaysNull(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 18, 1, 5, 0, 0, time.UTC)
	wantSlot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	artistID := insertTestArtist(t, pool, "all-suppressed")
	// releaseDate == "" inserts SQL NULL -- staleReleaseDate treats an
	// undated row as suppressed (conservative by design).
	suppressedID := insertPendingEventDated(t, pool, artistID, "sup-1", "Suppressed Title", "")

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if !isNotified(t, pool, suppressedID) {
		t.Fatalf("suppressed event %d not acked", suppressedID)
	}
	if !strings.Contains(buf.String(), "digest slot had nothing to send") {
		t.Fatalf("expected empty-skip Info log, got: %s", buf.String())
	}

	row, err := q.GetNotificationSettings(context.Background())
	if err != nil {
		t.Fatalf("get notification settings: %v", err)
	}
	if !row.DigestLastSlotAt.Valid || !row.DigestLastSlotAt.Time.Equal(wantSlot) {
		t.Fatalf("digest_last_slot_at = %+v, want %v", row.DigestLastSlotAt, wantSlot)
	}
	if row.DigestLastSentAt.Valid {
		t.Fatalf("digest_last_sent_at = %+v, want NULL", row.DigestLastSentAt)
	}
}

// TestSendDigestIfDue_NotDue_ZeroSendZeroWriteNoInfoWarnLog proves the
// not-due branch is silent at Info/Warn -- Debug only.
func TestSendDigestIfDue_NotDue_ZeroSendZeroWriteNoInfoWarnLog(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 19, 1, 5, 0, 0, time.UTC)
	slot := settings.MostRecentSlot(now, settings.CadenceDaily, loc)

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, &slot)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	for _, level := range []string{`"level":"INFO"`, `"level":"WARN"`, `"level":"ERROR"`} {
		if strings.Contains(buf.String(), level) {
			t.Fatalf("expected no %s record, got: %s", level, buf.String())
		}
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_GraceExpired_ZeroSendZeroWriteOneWarn proves a slot
// past its grace window writes nothing and logs exactly one Warn.
func TestSendDigestIfDue_GraceExpired_ZeroSendZeroWriteOneWarn(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 20, 13, 5, 0, 0, time.UTC) // 13h after the day's 00:05 slot; GraceDaily is 12h

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if !strings.Contains(buf.String(), "digest slot past its grace window") {
		t.Fatalf("expected grace-expired Warn log, got: %s", buf.String())
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_DigestModeOff_ZeroSendZeroWrite proves digest-off
// returns immediately with no send and no column write.
func TestSendDigestIfDue_DigestModeOff_ZeroSendZeroWrite(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := digestSettingsReader(false, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(time.UTC))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, time.Now()); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_LockHeld_ZeroSettingsReadsZeroSendOneInfo proves a
// SendDigestIfDue call that finds the shared notifying lock already held by
// an in-flight NotifyPending returns immediately, with no settings read of
// its own and one Info record.
func TestSendDigestIfDue_LockHeld_ZeroSettingsReadsZeroSendOneInfo(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)

	artistID := insertTestArtist(t, pool, "lock-held")
	insertPendingEventTyped(t, pool, artistID, "new_release", "lock-held-ext", "Lock Held Title")

	started := make(chan struct{})
	release := make(chan struct{})
	blockingSender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		close(started)
		<-release
		return nil
	}}

	notifySettings := stubSettings(false) // digest off: NotifyPending proceeds to Send
	n := notifier.New(q, blockingSender, notifySettings, time.Millisecond, notifier.WithLocation(time.UTC))

	notifyLogger, _ := newTestLogger()
	notifyDone := make(chan error, 1)
	go func() {
		notifyDone <- n.NotifyPending(context.Background(), notifyLogger)
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for NotifyPending to reach the blocking Send call")
	}

	callsBefore := notifySettings.Calls()

	digestLogger, buf := newTestLogger()
	if err := n.SendDigestIfDue(context.Background(), digestLogger, time.Now()); err != nil {
		t.Fatalf("SendDigestIfDue while lock held: %v, want nil", err)
	}
	if callsAfter := notifySettings.Calls(); callsAfter != callsBefore {
		t.Fatalf("shared settingsReader.Get calls = %d, want %d (no read while lock held)", callsAfter, callsBefore)
	}
	if !strings.Contains(buf.String(), "already in progress") {
		t.Fatalf("expected lock-held Info log, got: %s", buf.String())
	}

	close(release)
	if err := <-notifyDone; err != nil {
		t.Fatalf("NotifyPending: %v", err)
	}
}

// TestSendDigestIfDue_SettingsReadFails_ZeroSendZeroWriteOneWarn proves a
// first-read settings failure fails closed with the shared Phase 21 literal.
func TestSendDigestIfDue_SettingsReadFails_ZeroSendZeroWriteOneWarn(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := erroringSettings(errors.New("boom"))
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(time.UTC))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, time.Now()); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if !strings.Contains(buf.String(), "digest settings read failed") {
		t.Fatalf("expected settings-read-failure Warn log, got: %s", buf.String())
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_ModeFlipsOffBeforePost_ZeroSendZeroWrite proves D-17
// step 5's re-check: digest mode reported on at the first read and off at
// the immediately-pre-POST second read aborts with no send and no write.
func TestSendDigestIfDue_ModeFlipsOffBeforePost_ZeroSendZeroWrite(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 21, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "mode-flip")
	eventID := insertPendingEventTyped(t, pool, artistID, "new_release", "mode-flip-ext", "Mode Flip Title")

	before := snapshotSettings(t, q)

	sender := &fakeSender{}
	reader := flippingDigestSettings(2, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, _ := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if isNotified(t, pool, eventID) {
		t.Fatalf("event %d unexpectedly acked", eventID)
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_SenderError_ZeroWriteRowsStillPending is the
// send-failure test: a Sender.Send error leaves every event row pending and
// writes neither settings column, so the next due-check retries the batch.
func TestSendDigestIfDue_SenderError_ZeroWriteRowsStillPending(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)
	loc := time.UTC
	now := time.Date(2026, 6, 22, 1, 5, 0, 0, time.UTC)

	artistID := insertTestArtist(t, pool, "send-error")
	newReleaseID := insertPendingEventTyped(t, pool, artistID, "new_release", "send-err-nr", "Send Error New Release")
	guestID := insertPendingEventTyped(t, pool, artistID, "guest_feature", "send-err-gf", "Send Error Guest Feature")
	deluxeID := insertPendingEventTyped(t, pool, artistID, "deluxe_change", "send-err-dc", "Send Error Deluxe Change")

	before := snapshotSettings(t, q)

	sender := &fakeSender{fn: func(ctx context.Context, embed discord.Embed) error {
		return errors.New("discord unavailable")
	}}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond, notifier.WithLocation(loc))
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, now); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if !strings.Contains(buf.String(), "digest send failed") {
		t.Fatalf("expected send-failure Error log, got: %s", buf.String())
	}
	for _, id := range []int64{newReleaseID, guestID, deluxeID} {
		if isNotified(t, pool, id) {
			t.Fatalf("event %d unexpectedly acked after send failure", id)
		}
	}
	if after := snapshotSettings(t, q); !after.equal(before) {
		t.Fatalf("notification_settings changed: before=%+v after=%+v", before, after)
	}
}

// TestSendDigestIfDue_NoLocation_ZeroSendOneWarn proves a Notifier built
// without WithLocation refuses to send and never reads settings.
func TestSendDigestIfDue_NoLocation_ZeroSendOneWarn(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "notifier_test")
	q := sqlc.New(pool)

	sender := &fakeSender{}
	reader := digestSettingsReader(true, settings.CadenceDaily, nil)
	n := notifier.New(q, sender, reader, time.Millisecond) // no WithLocation
	logger, buf := newTestLogger()

	if err := n.SendDigestIfDue(context.Background(), logger, time.Now()); err != nil {
		t.Fatalf("SendDigestIfDue: %v, want nil", err)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Fatalf("sender.calls = %d, want 0", got)
	}
	if !strings.Contains(buf.String(), "digest zone not configured") {
		t.Fatalf("expected zone-not-configured Warn log, got: %s", buf.String())
	}
	if got := reader.Calls(); got != 0 {
		t.Fatalf("settings reads = %d, want 0", got)
	}
}

// TestNoOp_SendDigestIfDue_ReturnsNilTouchesNothing proves NoOp's
// implementation is a true no-op, mirroring NoOp.NotifyPending.
func TestNoOp_SendDigestIfDue_ReturnsNilTouchesNothing(t *testing.T) {
	logger, buf := newTestLogger()
	if err := (notifier.NoOp{}).SendDigestIfDue(context.Background(), logger, time.Now()); err != nil {
		t.Fatalf("NoOp.SendDigestIfDue: %v, want nil", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output, got: %s", buf.String())
	}
}
