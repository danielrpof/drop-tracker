package db_test

import (
	"context"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/db"
	"github.com/danielrpof/drop-tracker/internal/testutil"
)

// expectedSchemaVersionOnDisk is the highest migration number in
// internal/db/migrations/. It is a deliberate drift alarm: bump it in the
// same commit that adds a migration file, exactly as migrate_test.go's
// from-scratch assertion already does.
const expectedSchemaVersionOnDisk = 7

func TestExpectedSchemaVersion(t *testing.T) {
	got, err := db.ExpectedSchemaVersion()
	if err != nil {
		t.Fatalf("ExpectedSchemaVersion: %v", err)
	}
	if got != expectedSchemaVersionOnDisk {
		t.Fatalf("ExpectedSchemaVersion() = %d, want %d (bump the const when adding a migration)", got, expectedSchemaVersionOnDisk)
	}
}

func TestSchemaVersion_Integration(t *testing.T) {
	pool := testutil.NewTestPool(t)

	version, dirty, err := db.SchemaVersion(context.Background(), pool)
	if err != nil {
		t.Fatalf("SchemaVersion: %v", err)
	}
	if version == 0 {
		t.Fatal("SchemaVersion returned version 0 against a migrated fixture")
	}
	if dirty {
		t.Fatal("SchemaVersion reported the migrated fixture as dirty")
	}
}

func TestSchemaVersionReader_Delegates(t *testing.T) {
	pool := testutil.NewTestPool(t)
	ctx := context.Background()

	wantV, wantDirty, wantErr := db.SchemaVersion(ctx, pool)
	if wantErr != nil {
		t.Fatalf("SchemaVersion: %v", wantErr)
	}

	gotV, gotDirty, gotErr := db.NewSchemaVersionReader(pool).SchemaVersion(ctx)
	if gotErr != nil {
		t.Fatalf("reader SchemaVersion: %v", gotErr)
	}
	if gotV != wantV || gotDirty != wantDirty {
		t.Fatalf("reader returned (%d, %v), want (%d, %v)", gotV, gotDirty, wantV, wantDirty)
	}
}
