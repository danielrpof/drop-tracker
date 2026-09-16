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

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/settings"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

func TestService_FreshSchemaSeedsOneRowWithDefaults(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_fresh")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool))

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
	svc := settings.NewService(sqlc.New(pool))

	if _, err := svc.Update(ctx, settings.UpdateParams{DigestEnabled: true, DigestCadence: settings.CadenceWeekly}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// Read back through a second, independent sqlc.New(pool) instance, so
	// this assertion cannot be satisfied by in-process state on svc.
	reader := settings.NewService(sqlc.New(pool))
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
	svc := settings.NewService(sqlc.New(pool))

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
	svc := settings.NewService(sqlc.New(pool))

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

func TestService_UpdateRejectsUnknownCadence(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "settings_store_test_bad_cadence")
	ctx := context.Background()
	svc := settings.NewService(sqlc.New(pool))

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
