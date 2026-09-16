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
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
