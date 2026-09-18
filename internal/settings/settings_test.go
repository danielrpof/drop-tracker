package settings_test

// The settings row is a process-wide singleton (D-05), so every test in this
// file uses testutil.NewIsolatedTestPool rather than NewTestPool: an exact
// row-count or exact-default assertion against the shared fixture's default
// schema would be corrupted by any concurrently-running package or the live
// dev app -- the same hazard internal/notifier already resolved this way.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/settings"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// mustLoadNY is defined in slot_test.go (same package) and reused here.

func TestService_FreshSchemaSeedsOneRowWithDefaults(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_fresh")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool), mustLoadNY(t))

	got, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.DigestEnabled != false {
		t.Fatalf("DigestEnabled = %v, want false", got.DigestEnabled)
	}
	if got.DigestCadence != settings.CadenceDaily {
		t.Fatalf("DigestCadence = %q, want %q", got.DigestCadence, settings.CadenceDaily)
	}
	if got.DigestLastSentAt != nil {
		t.Fatalf("DigestLastSentAt = %v, want nil", *got.DigestLastSentAt)
	}
	if got.DigestLastSlotAt != nil {
		t.Fatalf("DigestLastSlotAt = %v, want nil", *got.DigestLastSlotAt)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notification_settings").Scan(&count); err != nil {
		t.Fatalf("query row count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestService_UpdateRoundTripsThroughPostgres(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_roundtrip")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool), mustLoadNY(t))

	if _, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceWeekly}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Read back through a second, independent sqlc.New(pool) instance, so
	// this assertion cannot be satisfied by in-process state on svc.
	reader := settings.NewService(sqlc.New(pool), mustLoadNY(t))
	got, err := reader.Get(ctx)
	if err != nil {
		t.Fatalf("Get (second instance): %v", err)
	}
	if got.DigestEnabled != true {
		t.Fatalf("DigestEnabled = %v, want true", got.DigestEnabled)
	}
	if got.DigestCadence != settings.CadenceWeekly {
		t.Fatalf("DigestCadence = %q, want %q", got.DigestCadence, settings.CadenceWeekly)
	}
}

func TestService_UpdateIsIdempotent(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_idempotent")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool), mustLoadNY(t))

	params := settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceWeekly}
	first, err := svc.Update(ctx, params)
	if err != nil {
		t.Fatalf("first Update: %v", err)
	}

	second, err := svc.Update(ctx, params)
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}

	if second.DigestEnabled != first.DigestEnabled || second.DigestCadence != first.DigestCadence {
		t.Fatalf("replayed Update changed observable state: first=%+v second=%+v", first, second)
	}
	if second.UpdatedAt.Before(first.UpdatedAt) {
		t.Fatalf("second UpdatedAt (%v) is before first (%v)", second.UpdatedAt, first.UpdatedAt)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notification_settings").Scan(&count); err != nil {
		t.Fatalf("query row count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1", count)
	}
}

func TestService_SecondRowRejectedByCheckConstraint(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_second_row")
	ctx := context.Background()

	_, err := pool.Exec(ctx, "INSERT INTO notification_settings (id) VALUES (2)")
	if err == nil {
		t.Fatal("insert with id=2 succeeded, want a check-constraint violation")
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM notification_settings").Scan(&count); err != nil {
		t.Fatalf("query row count: %v", err)
	}
	if count != 1 {
		t.Fatalf("row count = %d, want 1 (the rejected insert must not land)", count)
	}
}

func TestService_LastSentWatermarkUntouched(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_watermark")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool), mustLoadNY(t))

	if _, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceDaily}); err != nil {
		t.Fatalf("first Update: %v", err)
	}
	got, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: false, DigestCadence: settings.CadenceWeekly})
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}

	if got.DigestLastSentAt != nil {
		t.Fatalf("DigestLastSentAt = %v, want nil -- nothing in this phase writes the watermark", *got.DigestLastSentAt)
	}
}

// TestService_UpdateReanchorsSlot_EnablingFromOff pins D-14's first
// re-anchor trigger: enabling digest mode from off writes
// digest_last_slot_at to MostRecentSlot(clock(), cadence, loc), and never
// touches the separate digest_last_sent_at watermark.
func TestService_UpdateReanchorsSlot_EnablingFromOff(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_reanchor_enable")
	ctx := context.Background()
	loc := mustLoadNY(t)
	fixedNow := time.Date(2026, 6, 10, 9, 30, 0, 0, loc)
	svc := settings.NewService(sqlc.New(pool), loc, settings.WithClock(func() time.Time { return fixedNow }))

	got, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceDaily})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	wantSlot := settings.MostRecentSlot(fixedNow, settings.CadenceDaily, loc)
	if got.DigestLastSlotAt == nil {
		t.Fatal("DigestLastSlotAt = nil, want the re-anchored slot")
	}
	if !got.DigestLastSlotAt.Equal(wantSlot) {
		t.Fatalf("DigestLastSlotAt = %v, want %v", *got.DigestLastSlotAt, wantSlot)
	}
	if got.DigestLastSentAt != nil {
		t.Fatalf("DigestLastSentAt = %v, want nil -- enabling never writes the watermark", *got.DigestLastSentAt)
	}
}

// TestService_UpdateReanchorsSlot_CadenceChangeWhileOn pins D-14's second
// re-anchor trigger: changing cadence while digest mode is already on
// overwrites digest_last_slot_at with the new cadence's own
// MostRecentSlot(clock(), ...), computed at the moment of this second call.
func TestService_UpdateReanchorsSlot_CadenceChangeWhileOn(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_reanchor_cadence")
	ctx := context.Background()
	loc := mustLoadNY(t)
	firstNow := time.Date(2026, 6, 1, 9, 30, 0, 0, loc)
	secondNow := time.Date(2026, 6, 10, 9, 30, 0, 0, loc)
	now := firstNow
	svc := settings.NewService(sqlc.New(pool), loc, settings.WithClock(func() time.Time { return now }))

	first, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceDaily})
	if err != nil {
		t.Fatalf("first Update: %v", err)
	}
	if first.DigestLastSlotAt == nil {
		t.Fatal("first Update: DigestLastSlotAt = nil, want the re-anchored daily slot")
	}

	now = secondNow
	got, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceWeekly})
	if err != nil {
		t.Fatalf("second Update: %v", err)
	}

	wantSlot := settings.MostRecentSlot(secondNow, settings.CadenceWeekly, loc)
	if got.DigestLastSlotAt == nil {
		t.Fatal("second Update: DigestLastSlotAt = nil, want the re-anchored weekly slot")
	}
	if !got.DigestLastSlotAt.Equal(wantSlot) {
		t.Fatalf("DigestLastSlotAt = %v, want %v", *got.DigestLastSlotAt, wantSlot)
	}
	if got.DigestLastSlotAt.Equal(*first.DigestLastSlotAt) {
		t.Fatal("DigestLastSlotAt did not change across the cadence-changing Update")
	}
}

// TestService_UpdateLeavesSlotUntouched pins D-14's negative space: every
// update path that is neither "enable from off" nor "change cadence while
// already on" leaves digest_last_slot_at exactly as it was before the
// call -- a read-before/read-after equality check, not merely "no error".
func TestService_UpdateLeavesSlotUntouched(t *testing.T) {
	loc := mustLoadNY(t)
	now := time.Date(2026, 6, 10, 9, 30, 0, 0, loc)

	cases := []struct {
		name  string
		steps []settings.UpdateParams // every step but the last establishes the "before" state; the last is the update under test
	}{
		{
			name: "replay_same_cadence_while_on",
			steps: []settings.UpdateParams{
				{DigestEnabled: true, DigestCadence: settings.CadenceDaily},
				{DigestEnabled: true, DigestCadence: settings.CadenceDaily},
			},
		},
		{
			name: "turn_off_while_on",
			steps: []settings.UpdateParams{
				{DigestEnabled: true, DigestCadence: settings.CadenceDaily},
				{DigestEnabled: false, DigestCadence: settings.CadenceDaily},
			},
		},
		{
			name: "cadence_change_while_off",
			steps: []settings.UpdateParams{
				{DigestEnabled: false, DigestCadence: settings.CadenceDaily},
				{DigestEnabled: false, DigestCadence: settings.CadenceWeekly},
			},
		},
		{
			name: "replay_identical_while_off",
			steps: []settings.UpdateParams{
				{DigestEnabled: false, DigestCadence: settings.CadenceDaily},
				{DigestEnabled: false, DigestCadence: settings.CadenceDaily},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pool := testutil.NewIsolatedTestPool(t, "settings_store_test_untouched_"+tc.name)
			ctx := context.Background()
			svc := settings.NewService(sqlc.New(pool), loc, settings.WithClock(func() time.Time { return now }))

			var before settings.Settings
			var err error
			for _, p := range tc.steps[:len(tc.steps)-1] {
				before, err = svc.Update(ctx, p)
				if err != nil {
					t.Fatalf("setup Update(%+v): %v", p, err)
				}
			}

			after, err := svc.Update(ctx, tc.steps[len(tc.steps)-1])
			if err != nil {
				t.Fatalf("Update under test: %v", err)
			}

			switch {
			case before.DigestLastSlotAt == nil && after.DigestLastSlotAt == nil:
				// Both NULL: byte-identical.
			case before.DigestLastSlotAt != nil && after.DigestLastSlotAt != nil && before.DigestLastSlotAt.Equal(*after.DigestLastSlotAt):
				// Same instant: byte-identical.
			default:
				t.Fatalf("DigestLastSlotAt changed: before=%v after=%v", before.DigestLastSlotAt, after.DigestLastSlotAt)
			}
		})
	}
}

func TestService_UpdateRejectsUnknownCadence(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_bad_cadence")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool), mustLoadNY(t))

	before, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get (before): %v", err)
	}

	_, err = svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: "monthly"})
	if !errors.Is(err, settings.ErrInvalidCadence) {
		t.Fatalf("Update error = %v, want errors.Is(err, settings.ErrInvalidCadence)", err)
	}

	after, err := svc.Get(ctx)
	if err != nil {
		t.Fatalf("Get (after): %v", err)
	}
	if after != before {
		t.Fatalf("stored row changed after a rejected update: before=%+v after=%+v", before, after)
	}
}
