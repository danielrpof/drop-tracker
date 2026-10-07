package tags_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedEntry inserts an artist + watchlist row via raw SQL, deriving the mbid
// from t.Name() plus name so entries within one test never collide.
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

// --- List ---

func TestService_List_EmptyVocabularyReturnsNonNilEmptySlice(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	summaries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if summaries == nil {
		t.Fatal("List returned nil, want a non-nil empty slice")
	}
	if len(summaries) != 0 {
		t.Fatalf("List length = %d, want 0", len(summaries))
	}
}

// TestService_List_CarrierCountsIncludeZeroAndRemoved: counts are watched
// carriers only; zero-link and removed-artist-only tags still list (D-11).
func TestService_List_CarrierCountsIncludeZeroAndRemoved(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	watched1, _ := seedEntry(t, ctx, pool, "list-watched-1")
	watched2, _ := seedEntry(t, ctx, pool, "list-watched-2")
	removedEntry, _ := seedEntry(t, ctx, pool, "list-removed")

	if _, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", "Drill"); err != nil {
		t.Fatalf("seed zero-link tag: %v", err)
	}

	if _, err := svc.Attach(ctx, watched1, "reggaeton"); err != nil {
		t.Fatalf("attach reggaeton to watched1: %v", err)
	}
	if _, err := svc.Attach(ctx, watched2, "reggaeton"); err != nil {
		t.Fatalf("attach reggaeton to watched2: %v", err)
	}
	if _, err := svc.Attach(ctx, watched1, "latin"); err != nil {
		t.Fatalf("attach latin to watched1: %v", err)
	}
	if _, err := svc.Attach(ctx, removedEntry, "latin"); err != nil {
		t.Fatalf("attach latin to removedEntry: %v", err)
	}

	// Remove the watchlist row only -- artist_tags keys on artists.id
	// (TAG-07) and is untouched, so "latin" still carries a link to this
	// artist, just no longer a watched one.
	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", removedEntry); err != nil {
		t.Fatalf("remove watchlist entry: %v", err)
	}

	summaries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	byName := make(map[string]tags.Summary, len(summaries))
	for _, s := range summaries {
		byName[s.Name] = s
	}

	drill, ok := byName["Drill"]
	if !ok {
		t.Fatal("Drill missing from List (D-12: a zero-link tag must still surface)")
	}
	if drill.CarrierCount != 0 {
		t.Fatalf("Drill carrier_count = %d, want 0", drill.CarrierCount)
	}

	reggaeton, ok := byName["reggaeton"]
	if !ok {
		t.Fatal("reggaeton missing from List")
	}
	if reggaeton.CarrierCount != 2 {
		t.Fatalf("reggaeton carrier_count = %d, want 2", reggaeton.CarrierCount)
	}

	latin, ok := byName["latin"]
	if !ok {
		t.Fatal("latin missing from List (D-13: a tag only a removed artist carries must still surface)")
	}
	if latin.CarrierCount != 1 {
		t.Fatalf("latin carrier_count = %d, want 1 (removed artist not counted, D-11)", latin.CarrierCount)
	}
}

// TestService_List_OrderedByLowerNameThenID proves TAG-05's ordering: sorted
// by lower(name), not raw byte order (which would put uppercase-first names
// ahead of every lowercase name for the wrong reason).
func TestService_List_OrderedByLowerNameThenID(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "order-seed")

	for _, name := range []string{"Zouk", "reggaeton", "latin", "Drill", "apache"} {
		if _, err := svc.Attach(ctx, entryID, name); err != nil {
			t.Fatalf("attach %q: %v", name, err)
		}
	}

	summaries, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	got := make([]string, len(summaries))
	for i, s := range summaries {
		got[i] = s.Name
	}
	want := []string{"apache", "Drill", "latin", "reggaeton", "Zouk"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("List order = %v, want %v", got, want)
	}
}

// --- Rename (409 collision) and Delete ---

func countArtistTagsByTagID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tagID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE tag_id = $1", tagID).Scan(&n); err != nil {
		t.Fatalf("count artist_tags by tag id: %v", err)
	}
	return n
}

func TestService_Rename_AppliesEverywhere(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	e1, _ := seedEntry(t, ctx, pool, "rename-everywhere-1")
	e2, _ := seedEntry(t, ctx, pool, "rename-everywhere-2")

	r1, err := svc.Attach(ctx, e1, "hiphop")
	if err != nil {
		t.Fatalf("attach 1: %v", err)
	}
	if _, err := svc.Attach(ctx, e2, "hiphop"); err != nil {
		t.Fatalf("attach 2: %v", err)
	}

	renamed, err := svc.Rename(ctx, r1.Tag.ID, "hip hop")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if renamed.Name != "hip hop" {
		t.Fatalf("renamed.Name = %q, want %q", renamed.Name, "hip hop")
	}

	if n := countTagsByLowerName(t, ctx, pool, "hip hop"); n != 1 {
		t.Fatalf("tags rows named %q = %d, want 1", "hip hop", n)
	}
	if n := countArtistTagsByTagID(t, ctx, pool, r1.Tag.ID); n != 2 {
		t.Fatalf("artist_tags rows for renamed tag = %d, want 2 (both carriers still linked)", n)
	}
}

func TestService_Rename_CaseOnlySameTagIsPlainRename(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "rename-case-only")
	result, err := svc.Attach(ctx, entryID, "Latin")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	renamed, err := svc.Rename(ctx, result.Tag.ID, "latin")
	if err != nil {
		t.Fatalf("case-only rename: %v", err)
	}
	if renamed.Name != "latin" {
		t.Fatalf("renamed.Name = %q, want %q", renamed.Name, "latin")
	}

	renamedAgain, err := svc.Rename(ctx, result.Tag.ID, "latin")
	if err != nil {
		t.Fatalf("rename to own current name: %v", err)
	}
	if renamedAgain.Name != "latin" {
		t.Fatalf("renamedAgain.Name = %q, want %q", renamedAgain.Name, "latin")
	}
}

// TestService_Rename_CollisionReturnsCollisionErrorAndChangesNothing: a rename
// onto another tag returns *CollisionError with the target's casing and the
// post-merge count, leaving the source unchanged (D-22).
func TestService_Rename_CollisionReturnsCollisionErrorAndChangesNothing(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, _ := seedEntry(t, ctx, pool, "collision-a")
	b, _ := seedEntry(t, ctx, pool, "collision-b")
	c, _ := seedEntry(t, ctx, pool, "collision-c")

	trap, err := svc.Attach(ctx, a, "trap")
	if err != nil {
		t.Fatalf("attach trap to a: %v", err)
	}
	if _, err := svc.Attach(ctx, b, "trap"); err != nil {
		t.Fatalf("attach trap to b: %v", err)
	}
	rap, err := svc.Attach(ctx, b, "rap")
	if err != nil {
		t.Fatalf("attach rap to b: %v", err)
	}
	if _, err := svc.Attach(ctx, c, "rap"); err != nil {
		t.Fatalf("attach rap to c: %v", err)
	}

	_, err = svc.Rename(ctx, rap.Tag.ID, " TRAP ")
	var collision *tags.CollisionError
	if !errors.As(err, &collision) {
		t.Fatalf("Rename: err = %v, want *tags.CollisionError", err)
	}
	if collision.Target.ID != trap.Tag.ID || collision.Target.Name != "trap" {
		t.Fatalf("collision.Target = %+v, want {%d trap}", collision.Target, trap.Tag.ID)
	}
	if collision.CarrierCountAfterMerge != 3 {
		t.Fatalf("collision.CarrierCountAfterMerge = %d, want 3 (a, b, c union)", collision.CarrierCountAfterMerge)
	}

	// Nothing about rap changed: name, id, and its two links are intact.
	if n := countTagsByLowerName(t, ctx, pool, "rap"); n != 1 {
		t.Fatalf("tags rows named %q after 409 = %d, want 1 (source untouched)", "rap", n)
	}
	if n := countArtistTagsByTagID(t, ctx, pool, rap.Tag.ID); n != 2 {
		t.Fatalf("artist_tags rows for rap after 409 = %d, want 2 (b, c unchanged)", n)
	}
}

func TestService_Rename_BlankNameReturnsErrNameRequired(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "rename-blank")
	result, err := svc.Attach(ctx, entryID, "some-tag")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if _, err := svc.Rename(ctx, result.Tag.ID, "   "); !errors.Is(err, tags.ErrNameRequired) {
		t.Fatalf("Rename to blank: err = %v, want ErrNameRequired", err)
	}
}

func TestService_Rename_TooLongNameReturnsErrNameTooLong(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "rename-too-long")
	result, err := svc.Attach(ctx, entryID, "some-tag-2")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	longName := ""
	for i := 0; i < 33; i++ {
		longName += "a"
	}
	if _, err := svc.Rename(ctx, result.Tag.ID, longName); !errors.Is(err, tags.ErrNameTooLong) {
		t.Fatalf("Rename to 33-rune name: err = %v, want ErrNameTooLong", err)
	}
}

func TestService_Rename_MissingIDReturnsErrTagNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	if _, err := svc.Rename(ctx, 987654321, "whatever"); !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("Rename on unknown id: err = %v, want ErrTagNotFound", err)
	}
}

// TestService_Delete_RemovesEverywhereAndReturnsWatchedCount: the watched count
// is reported, every link goes (removed artists' too), watchlist rows stay (TAG-06).
func TestService_Delete_RemovesEverywhereAndReturnsWatchedCount(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	watched1, _ := seedEntry(t, ctx, pool, "delete-watched-1")
	watched2, _ := seedEntry(t, ctx, pool, "delete-watched-2")
	removedEntry, _ := seedEntry(t, ctx, pool, "delete-removed")

	result, err := svc.Attach(ctx, watched1, "delete-me")
	if err != nil {
		t.Fatalf("attach to watched1: %v", err)
	}
	if _, err := svc.Attach(ctx, watched2, "delete-me"); err != nil {
		t.Fatalf("attach to watched2: %v", err)
	}
	if _, err := svc.Attach(ctx, removedEntry, "delete-me"); err != nil {
		t.Fatalf("attach to removedEntry: %v", err)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", removedEntry); err != nil {
		t.Fatalf("remove watchlist entry: %v", err)
	}

	var beforeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM watchlist WHERE id IN ($1, $2)", watched1, watched2).Scan(&beforeCount); err != nil {
		t.Fatalf("count watchlist rows before delete: %v", err)
	}

	count, err := svc.Delete(ctx, result.Tag.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if count != 2 {
		t.Fatalf("Delete carrier count = %d, want 2 (removed artist not counted, D-11)", count)
	}

	if n := countArtistTagsByTagID(t, ctx, pool, result.Tag.ID); n != 0 {
		t.Fatalf("artist_tags rows for deleted tag = %d, want 0 (all 3 links removed, including the removed artist's)", n)
	}
	if n := countTagsByLowerName(t, ctx, pool, "delete-me"); n != 0 {
		t.Fatalf("tags rows named %q after delete = %d, want 0", "delete-me", n)
	}

	var afterCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM watchlist WHERE id IN ($1, $2)", watched1, watched2).Scan(&afterCount); err != nil {
		t.Fatalf("count watchlist rows after delete: %v", err)
	}
	if afterCount != beforeCount {
		t.Fatalf("watchlist row count changed: before=%d after=%d, want unchanged (SC4)", beforeCount, afterCount)
	}
}

func TestService_Delete_UnknownIDReturnsErrTagNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	if _, err := svc.Delete(ctx, 987654321); !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("Delete on unknown id: err = %v, want ErrTagNotFound", err)
	}
}

func TestService_Delete_TwiceReturnsErrTagNotFoundSecondTime(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "delete-twice")
	result, err := svc.Attach(ctx, entryID, "delete-twice-tag")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if _, err := svc.Delete(ctx, result.Tag.ID); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if _, err := svc.Delete(ctx, result.Tag.ID); !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("second Delete: err = %v, want ErrTagNotFound", err)
	}
}

// --- Merge ---

func TestService_Merge_UnionsMembershipsAndDeletesSource(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, _ := seedEntry(t, ctx, pool, "merge-union-a")
	b, _ := seedEntry(t, ctx, pool, "merge-union-b")
	c, _ := seedEntry(t, ctx, pool, "merge-union-c")

	trap, err := svc.Attach(ctx, a, "trap")
	if err != nil {
		t.Fatalf("attach trap to a: %v", err)
	}
	if _, err := svc.Attach(ctx, b, "trap"); err != nil {
		t.Fatalf("attach trap to b: %v", err)
	}
	rap, err := svc.Attach(ctx, b, "rap")
	if err != nil {
		t.Fatalf("attach rap to b: %v", err)
	}
	if _, err := svc.Attach(ctx, c, "rap"); err != nil {
		t.Fatalf("attach rap to c: %v", err)
	}

	summary, err := svc.Merge(ctx, rap.Tag.ID, trap.Tag.ID)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if summary.ID != trap.Tag.ID || summary.Name != "trap" || summary.CarrierCount != 3 {
		t.Fatalf("summary = %+v, want {%d trap 3}", summary, trap.Tag.ID)
	}

	if n := countTagsByLowerName(t, ctx, pool, "rap"); n != 0 {
		t.Fatalf("rap tag rows after merge = %d, want 0 (source deleted)", n)
	}
	if n := countArtistTagsByTagID(t, ctx, pool, trap.Tag.ID); n != 3 {
		t.Fatalf("trap link count after merge = %d, want 3 (a, b, c)", n)
	}

	// b carried both; after the merge it has exactly one link, to trap.
	var bLinks int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags at JOIN watchlist w ON w.artist_id = at.artist_id WHERE w.id = $1", b).Scan(&bLinks); err != nil {
		t.Fatalf("count b's links: %v", err)
	}
	if bLinks != 1 {
		t.Fatalf("b's total link count after merge = %d, want 1 (duplicate collapsed)", bLinks)
	}
}

// TestService_Merge_AtCap is ADR 0004 required test 3: a 10-tag artist that
// carries the source but not the target still merges successfully and ends
// with 10 links, one of them the target.
func TestService_Merge_AtCap(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-at-cap")

	rap, err := svc.Attach(ctx, entryID, "at-cap-rap")
	if err != nil {
		t.Fatalf("attach rap: %v", err)
	}
	for i := 0; i < tags.MaxTagsPerArtist-1; i++ {
		if _, err := svc.Attach(ctx, entryID, fmt.Sprintf("at-cap-filler-%02d", i)); err != nil {
			t.Fatalf("seed filler %d: %v", i, err)
		}
	}
	trapEntry, _ := seedEntry(t, ctx, pool, "merge-at-cap-target-seed")
	trap, err := svc.Attach(ctx, trapEntry, "at-cap-trap")
	if err != nil {
		t.Fatalf("attach trap: %v", err)
	}

	summary, err := svc.Merge(ctx, rap.Tag.ID, trap.Tag.ID)
	if err != nil {
		t.Fatalf("Merge at cap: %v", err)
	}
	if summary.ID != trap.Tag.ID {
		t.Fatalf("summary.ID = %d, want %d", summary.ID, trap.Tag.ID)
	}

	var linkCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags at JOIN watchlist w ON w.artist_id = at.artist_id WHERE w.id = $1", entryID).Scan(&linkCount); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if linkCount != tags.MaxTagsPerArtist {
		t.Fatalf("link count = %d, want %d", linkCount, tags.MaxTagsPerArtist)
	}
	if n := countArtistTagsByTagID(t, ctx, pool, trap.Tag.ID); n < 1 {
		t.Fatalf("trap link count = %d, want at least 1", n)
	}
}

// TestService_Merge_AtCapBothSourceAndTarget covers the companion case: an
// artist already carrying both tags loses one link on merge (10 -> 9).
func TestService_Merge_AtCapBothSourceAndTarget(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-at-cap-both")

	rap, err := svc.Attach(ctx, entryID, "both-rap")
	if err != nil {
		t.Fatalf("attach rap: %v", err)
	}
	trap, err := svc.Attach(ctx, entryID, "both-trap")
	if err != nil {
		t.Fatalf("attach trap: %v", err)
	}
	for i := 0; i < tags.MaxTagsPerArtist-2; i++ {
		if _, err := svc.Attach(ctx, entryID, fmt.Sprintf("both-filler-%02d", i)); err != nil {
			t.Fatalf("seed filler %d: %v", i, err)
		}
	}

	if _, err := svc.Merge(ctx, rap.Tag.ID, trap.Tag.ID); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	var linkCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags at JOIN watchlist w ON w.artist_id = at.artist_id WHERE w.id = $1", entryID).Scan(&linkCount); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if linkCount != tags.MaxTagsPerArtist-1 {
		t.Fatalf("link count = %d, want %d", linkCount, tags.MaxTagsPerArtist-1)
	}
}

func TestService_Merge_ZeroLinkSourceSucceeds(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-zero-link")
	target, err := svc.Attach(ctx, entryID, "zero-link-target")
	if err != nil {
		t.Fatalf("attach target: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", "zero-link-source"); err != nil {
		t.Fatalf("seed zero-link source tag: %v", err)
	}
	var sourceID int64
	if err := pool.QueryRow(ctx, "SELECT id FROM tags WHERE name = $1", "zero-link-source").Scan(&sourceID); err != nil {
		t.Fatalf("read source id: %v", err)
	}

	summary, err := svc.Merge(ctx, sourceID, target.Tag.ID)
	if err != nil {
		t.Fatalf("Merge zero-link source: %v", err)
	}
	if summary.CarrierCount != 1 {
		t.Fatalf("summary.CarrierCount = %d, want 1 (unchanged)", summary.CarrierCount)
	}
	if n := countTagsByLowerName(t, ctx, pool, "zero-link-source"); n != 0 {
		t.Fatalf("source tag rows after merge = %d, want 0", n)
	}
}

func TestService_Merge_IntoSelfReturnsErrMergeIntoSelf(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-self")
	result, err := svc.Attach(ctx, entryID, "self-tag")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	if _, err := svc.Merge(ctx, result.Tag.ID, result.Tag.ID); !errors.Is(err, tags.ErrMergeIntoSelf) {
		t.Fatalf("Merge into self: err = %v, want ErrMergeIntoSelf", err)
	}
}

func TestService_Merge_MissingSourceReturnsErrTagNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-missing-source")
	target, err := svc.Attach(ctx, entryID, "missing-source-target")
	if err != nil {
		t.Fatalf("attach target: %v", err)
	}

	if _, err := svc.Merge(ctx, 987654321, target.Tag.ID); !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("Merge missing source: err = %v, want ErrTagNotFound", err)
	}
}

func TestService_Merge_MissingTargetReturnsErrTagNotFound(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	entryID, _ := seedEntry(t, ctx, pool, "merge-missing-target")
	source, err := svc.Attach(ctx, entryID, "missing-target-source")
	if err != nil {
		t.Fatalf("attach source: %v", err)
	}

	if _, err := svc.Merge(ctx, source.Tag.ID, 987654321); !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("Merge missing target: err = %v, want ErrTagNotFound", err)
	}
}

// TestService_Merge_MatchesPriorCollisionCount proves a 409 rename
// collision's carrier_count_after_merge equals what a subsequent confirmed
// Merge actually returns for the identical pair.
func TestService_Merge_MatchesPriorCollisionCount(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_service_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, _ := seedEntry(t, ctx, pool, "merge-matches-a")
	b, _ := seedEntry(t, ctx, pool, "merge-matches-b")
	c, _ := seedEntry(t, ctx, pool, "merge-matches-c")

	trap, err := svc.Attach(ctx, a, "matches-trap")
	if err != nil {
		t.Fatalf("attach trap to a: %v", err)
	}
	if _, err := svc.Attach(ctx, b, "matches-trap"); err != nil {
		t.Fatalf("attach trap to b: %v", err)
	}
	rap, err := svc.Attach(ctx, b, "matches-rap")
	if err != nil {
		t.Fatalf("attach rap to b: %v", err)
	}
	if _, err := svc.Attach(ctx, c, "matches-rap"); err != nil {
		t.Fatalf("attach rap to c: %v", err)
	}

	_, renameErr := svc.Rename(ctx, rap.Tag.ID, "matches-trap")
	var collision *tags.CollisionError
	if !errors.As(renameErr, &collision) {
		t.Fatalf("Rename: err = %v, want *tags.CollisionError", renameErr)
	}

	summary, err := svc.Merge(ctx, rap.Tag.ID, trap.Tag.ID)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if summary.CarrierCount != collision.CarrierCountAfterMerge {
		t.Fatalf("Merge carrier_count = %d, want %d (matching prior collision)", summary.CarrierCount, collision.CarrierCountAfterMerge)
	}
}
