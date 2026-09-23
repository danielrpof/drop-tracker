package tags_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedEntry inserts a fresh artist + watchlist row, deriving both the mbid
// and the test-name-scoped suffix from t.Name() plus name so multiple
// entries within one test never collide. Raw inserts only -- this package's
// own Service is what's under test.
func seedEntry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) (entryID, artistID int64) {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name() + "|" + name))
	mbid := "tags-svc-" + hex.EncodeToString(sum[:])[:20]

	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, name).Scan(&artistID); err != nil {
		t.Fatalf("seed artist: %v", err)
	}
	if err := pool.QueryRow(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1) RETURNING id", artistID).Scan(&entryID); err != nil {
		t.Fatalf("seed watchlist: %v", err)
	}
	return entryID, artistID
}

func countTagsByLowerName(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM tags WHERE lower(name) = lower($1)", name).Scan(&n); err != nil {
		t.Fatalf("count tags by lower name: %v", err)
	}
	return n
}

func TestService_Attach_CreatesAndLinksNewTag(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "attach-new")

	result, err := svc.Attach(ctx, entryID, "Reggaeton")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true for a brand-new link")
	}
	if result.Tag.Name != "Reggaeton" {
		t.Fatalf("Tag.Name = %q, want %q", result.Tag.Name, "Reggaeton")
	}
}

func TestService_Attach_ExistingLinkReturnsCreatedFalse(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "attach-existing")

	if _, err := svc.Attach(ctx, entryID, "rap"); err != nil {
		t.Fatalf("first Attach: %v", err)
	}
	result, err := svc.Attach(ctx, entryID, "RAP")
	if err != nil {
		t.Fatalf("second Attach: %v", err)
	}
	if result.Created {
		t.Fatal("Created = true, want false for an already-linked tag")
	}
	if result.Tag.Name != "rap" {
		t.Fatalf("Tag.Name = %q, want %q (first-entered casing, TAG-03)", result.Tag.Name, "rap")
	}
}

func TestService_Attach_UnknownEntryReturnsErrEntryNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	if _, err := svc.Attach(ctx, 987654321, "whatever"); !errors.Is(err, tags.ErrEntryNotFound) {
		t.Fatalf("Attach to unknown entry: err = %v, want ErrEntryNotFound", err)
	}
}

func TestService_Attach_AtCapReturnsErrTagCapReachedAndRollsBackOrphan(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "attach-at-cap")
	for i := 0; i < tags.MaxTagsPerArtist; i++ {
		if _, err := svc.Attach(ctx, entryID, fmt.Sprintf("cap-tag-%02d", i)); err != nil {
			t.Fatalf("seed attach %d: %v", i, err)
		}
	}

	const eleventh = "cap-tag-eleventh"
	_, err := svc.Attach(ctx, entryID, eleventh)
	if !errors.Is(err, tags.ErrTagCapReached) {
		t.Fatalf("11th Attach: err = %v, want ErrTagCapReached", err)
	}

	if n := countTagsByLowerName(t, ctx, pool, eleventh); n != 0 {
		t.Fatalf("orphan tags row count for %q = %d, want 0 (the refused link's transaction must roll back the get-or-create too)", eleventh, n)
	}

	// Re-attaching an existing link at the cap still succeeds with
	// Created == false -- the cap only blocks new links.
	result, err := svc.Attach(ctx, entryID, "cap-tag-00")
	if err != nil {
		t.Fatalf("re-attach at cap: %v", err)
	}
	if result.Created {
		t.Fatal("re-attach at cap: Created = true, want false")
	}
}

func TestService_Identity_ASCIICaseAndWhitespace(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	e1, _ := seedEntry(t, ctx, pool, "identity-ascii-1")
	e2, _ := seedEntry(t, ctx, pool, "identity-ascii-2")
	e3, _ := seedEntry(t, ctx, pool, "identity-ascii-3")

	r1, err := svc.Attach(ctx, e1, "Reggaeton ")
	if err != nil {
		t.Fatalf("attach 1: %v", err)
	}
	r2, err := svc.Attach(ctx, e2, "reggaeton")
	if err != nil {
		t.Fatalf("attach 2: %v", err)
	}
	r3, err := svc.Attach(ctx, e3, "REGGAETON")
	if err != nil {
		t.Fatalf("attach 3: %v", err)
	}

	if r1.Tag.ID != r2.Tag.ID || r2.Tag.ID != r3.Tag.ID {
		t.Fatalf("tag ids = %d, %d, %d, want all equal", r1.Tag.ID, r2.Tag.ID, r3.Tag.ID)
	}
	if r1.Tag.Name != "Reggaeton" || r2.Tag.Name != "Reggaeton" || r3.Tag.Name != "Reggaeton" {
		t.Fatalf("tag names = %q, %q, %q, want all %q (first-entered casing)", r1.Tag.Name, r2.Tag.Name, r3.Tag.Name, "Reggaeton")
	}
}

func TestService_Identity_NonASCIICaseFold(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	e1, _ := seedEntry(t, ctx, pool, "identity-nonascii-1")
	e2, _ := seedEntry(t, ctx, pool, "identity-nonascii-2")

	r1, err := svc.Attach(ctx, e1, "REGGAETÓN")
	if err != nil {
		t.Fatalf("attach 1: %v", err)
	}
	r2, err := svc.Attach(ctx, e2, "reggaetón")
	if err != nil {
		t.Fatalf("attach 2: %v", err)
	}
	if r1.Tag.ID != r2.Tag.ID {
		t.Fatalf("tag ids = %d, %d, want equal (D-31 non-ASCII fold)", r1.Tag.ID, r2.Tag.ID)
	}
}

func TestService_Detach_RemovesExistingLink(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "detach-existing")
	result, err := svc.Attach(ctx, entryID, "detach-me")
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}

	if err := svc.Detach(ctx, entryID, result.Tag.ID); err != nil {
		t.Fatalf("Detach: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE tag_id = $1", result.Tag.ID).Scan(&count); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if count != 0 {
		t.Fatalf("artist_tags count = %d, want 0", count)
	}
}

func TestService_Detach_MissingLinkIsIdempotentSuccess(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "detach-missing")

	if err := svc.Detach(ctx, entryID, 99999999); err != nil {
		t.Fatalf("Detach on never-linked tag: %v", err)
	}
}

func TestService_Detach_UnknownEntryReturnsErrEntryNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	if err := svc.Detach(ctx, 987654321, 1); !errors.Is(err, tags.ErrEntryNotFound) {
		t.Fatalf("Detach on unknown entry: err = %v, want ErrEntryNotFound", err)
	}
}
