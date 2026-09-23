package db_test

// Schema-level proof for migration 000010 (D-18, ADR 0004): these tests
// exercise the database directly with raw SQL, before internal/tags exists,
// so the cap trigger, name/note CHECKs, and cascade behaviour are proven
// independent of any service-layer guard (SC2).

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedArtistNamed inserts a unique artist row, deriving its mbid from the
// running test's name plus suffix so multiple artists within one test never
// collide.
func seedArtistNamed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string) int64 {
	t.Helper()
	sum := sha256.Sum256([]byte(t.Name() + "|" + suffix))
	mbid := "tags-schema-" + hex.EncodeToString(sum[:])[:20]
	var id int64
	if err := pool.QueryRow(ctx, "INSERT INTO artists (mbid, name) VALUES ($1, $2) RETURNING id", mbid, "Tags Schema Test Artist").Scan(&id); err != nil {
		t.Fatalf("seed artist: %v", err)
	}
	return id
}

func seedArtist(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int64 {
	t.Helper()
	return seedArtistNamed(t, ctx, pool, "")
}

func insertTag(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) int64 {
	t.Helper()
	var id int64
	if err := pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ($1) RETURNING id", name).Scan(&id); err != nil {
		t.Fatalf("insert tag %q: %v", name, err)
	}
	return id
}

func linkTag(ctx context.Context, pool *pgxpool.Pool, artistID, tagID int64) error {
	_, err := pool.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", artistID, tagID)
	return err
}

// TestSchema_TagCapTrigger_RawInsertRefused is ADR 0004's SC2 proof: the
// database refuses an 11th link via a raw INSERT with no API code involved.
func TestSchema_TagCapTrigger_RawInsertRefused(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	artistID := seedArtist(t, ctx, pool)
	tagIDs := make([]int64, 11)
	for i := range tagIDs {
		tagIDs[i] = insertTag(t, ctx, pool, fmt.Sprintf("tag-%02d", i))
	}
	for i := 0; i < 10; i++ {
		if err := linkTag(ctx, pool, artistID, tagIDs[i]); err != nil {
			t.Fatalf("link tag %d: %v", i, err)
		}
	}

	err := linkTag(ctx, pool, artistID, tagIDs[10])
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("11th raw insert: got err %v, want *pgconn.PgError", err)
	}
	if pgErr.Code != pgerrcode.CheckViolation {
		t.Fatalf("11th raw insert code = %q, want %q (%s)", pgErr.Code, pgerrcode.CheckViolation, pgErr.Message)
	}
	if pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("11th raw insert constraint = %q, want artist_tags_max_per_artist", pgErr.ConstraintName)
	}
}

// TestTrigger_SkipExisting is ADR 0004 required test 2: a set-based
// ON CONFLICT DO NOTHING insert naming only already-linked tags is a true
// no-op at the cap; the same statement naming one new tag still refuses.
func TestTrigger_SkipExisting(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	artistID := seedArtist(t, ctx, pool)
	tagIDs := make([]int64, 10)
	for i := range tagIDs {
		tagIDs[i] = insertTag(t, ctx, pool, fmt.Sprintf("skip-%02d", i))
		if err := linkTag(ctx, pool, artistID, tagIDs[i]); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}

	const setInsert = `INSERT INTO artist_tags (artist_id, tag_id) SELECT $1, unnest($2::bigint[]) ON CONFLICT DO NOTHING`

	tag, err := pool.Exec(ctx, setInsert, artistID, []int64{tagIDs[0], tagIDs[1]})
	if err != nil {
		t.Fatalf("set-based insert over two already-linked tags: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Fatalf("rows affected = %d, want 0 (both tags already linked)", tag.RowsAffected())
	}

	newTagID := insertTag(t, ctx, pool, "skip-new")
	_, err = pool.Exec(ctx, setInsert, artistID, []int64{tagIDs[0], newTagID})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("set-based insert with one new tag at cap: got err %v, want artist_tags_max_per_artist", err)
	}
}

// TestSchema_TagCapTrigger_SameTagConcurrentAt9 is ADR 0004 required test 1:
// two concurrent attaches of the same new tag at 9 links both succeed and
// leave exactly one link, because the trigger re-checks existence after the
// artist-row lock. The interleaving is forced, not hoped for: txB's backend
// is observed waiting on the lock via pg_stat_activity before txA commits.
func TestSchema_TagCapTrigger_SameTagConcurrentAt9(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	artistID := seedArtist(t, ctx, pool)
	for i := 0; i < 9; i++ {
		tagID := insertTag(t, ctx, pool, fmt.Sprintf("race-%02d", i))
		if err := linkTag(ctx, pool, artistID, tagID); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	tagX := insertTag(t, ctx, pool, "race-shared")

	const insertOne = `INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2) ON CONFLICT (artist_id, tag_id) DO NOTHING`

	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin txA: %v", err)
	}
	defer func() { _ = txA.Rollback(ctx) }()
	if _, err := txA.Exec(ctx, insertOne, artistID, tagX); err != nil {
		t.Fatalf("txA insert: %v", err)
	}

	pidCh := make(chan uint32, 1)
	errCh := make(chan error, 1)
	go func() {
		txB, err := pool.Begin(ctx)
		if err != nil {
			errCh <- fmt.Errorf("begin txB: %w", err)
			return
		}
		defer func() { _ = txB.Rollback(ctx) }()
		pidCh <- txB.Conn().PgConn().PID()
		if _, err := txB.Exec(ctx, insertOne, artistID, tagX); err != nil {
			errCh <- err
			return
		}
		errCh <- txB.Commit(ctx)
	}()

	pidB := <-pidCh

	deadline := time.Now().Add(5 * time.Second)
	waiting := false
	for time.Now().Before(deadline) {
		var waitEventType *string
		if err := pool.QueryRow(ctx, "SELECT wait_event_type FROM pg_stat_activity WHERE pid = $1", pidB).Scan(&waitEventType); err != nil {
			t.Fatalf("poll pg_stat_activity: %v", err)
		}
		if waitEventType != nil && *waitEventType == "Lock" {
			waiting = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !waiting {
		t.Fatal("txB never observed waiting on a Lock -- the artist-row lock did not serialize the concurrent same-tag attach")
	}

	if err := txA.Commit(ctx); err != nil {
		t.Fatalf("commit txA: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("txB after txA commit: %v", err)
	}

	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if count != 10 {
		t.Fatalf("artist_tags count = %d, want 10", count)
	}
}

// TestSchema_TagNameChecks proves tags_name_length and tags_name_trimmed
// (D-28): a too-long or blank name is rejected, an untrimmed name is
// rejected, and a 32-rune multi-byte name is accepted -- matching Postgres
// char_length, which TAG-04's encoding probe requires.
func TestSchema_TagNameChecks(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	cases := []struct {
		name           string
		value          string
		wantConstraint string
	}{
		{"too long", strings.Repeat("a", 33), "tags_name_length"},
		{"empty", "", "tags_name_length"},
		{"leading space", " rap", "tags_name_trimmed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", tc.value)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.ConstraintName != tc.wantConstraint {
				t.Fatalf("insert %q: got err %v, want constraint %q", tc.value, err, tc.wantConstraint)
			}
		})
	}

	multiByte32 := strings.Repeat("é", 32) // 32 code points of 'é'
	if _, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", multiByte32); err != nil {
		t.Fatalf("32-rune multi-byte name rejected: %v", err)
	}
}

// TestSchema_TagNameUniqueLower proves the tags_name_lower_idx expression
// index (D-29's ON CONFLICT target, TAG-03) including the non-ASCII pair
// D-31 requires. A C/POSIX collation would fail only the second assertion,
// so its failure message reports the database's actual collation.
func TestSchema_TagNameUniqueLower(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	insertTag(t, ctx, pool, "reggaeton")
	_, err := pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", "Reggaeton")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation || pgErr.ConstraintName != "tags_name_lower_idx" {
		t.Fatalf("insert %q after %q: got err %v, want unique violation on tags_name_lower_idx", "Reggaeton", "reggaeton", err)
	}

	insertTag(t, ctx, pool, "reggaetón")
	_, err = pool.Exec(ctx, "INSERT INTO tags (name) VALUES ($1)", "REGGAETÓN")
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation || pgErr.ConstraintName != "tags_name_lower_idx" {
		var collate string
		_ = pool.QueryRow(ctx, "SELECT datcollate FROM pg_database WHERE datname = current_database()").Scan(&collate)
		t.Fatalf("insert %q after %q: got err %v, want unique violation on tags_name_lower_idx (database collation: %s)", "REGGAETÓN", "reggaetón", err, collate)
	}
}

// TestSchema_NoteChecks proves watchlist_note_length and
// watchlist_note_not_blank (D-28): a 501-char or blank note is rejected,
// NULL and a 500-char note both succeed.
func TestSchema_NoteChecks(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()
	artistID := seedArtist(t, ctx, pool)

	cases := []struct {
		name           string
		note           string
		wantConstraint string
	}{
		{"too long", strings.Repeat("a", 501), "watchlist_note_length"},
		{"blank", "   ", "watchlist_note_not_blank"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id, note) VALUES ($1, $2)", artistID, tc.note)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.ConstraintName != tc.wantConstraint {
				t.Fatalf("insert note %q: got err %v, want constraint %q", tc.note, err, tc.wantConstraint)
			}
		})
	}

	nullArtist := seedArtistNamed(t, ctx, pool, "null-note")
	if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id, note) VALUES ($1, $2)", nullArtist, nil); err != nil {
		t.Fatalf("insert NULL note: %v", err)
	}

	fullArtist := seedArtistNamed(t, ctx, pool, "full-note")
	if _, err := pool.Exec(ctx, "INSERT INTO watchlist (artist_id, note) VALUES ($1, $2)", fullArtist, strings.Repeat("a", 500)); err != nil {
		t.Fatalf("insert 500-char note: %v", err)
	}
}

// TestSchema_WatchlistDeleteKeepsTagLinks proves TAG-07 at the database
// level: deleting the watchlist row leaves artist_tags intact; deleting the
// artists row cascades them away; deleting a tags row cascades its links.
func TestSchema_WatchlistDeleteKeepsTagLinks(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	artistID := seedArtist(t, ctx, pool)
	var watchlistID int64
	if err := pool.QueryRow(ctx, "INSERT INTO watchlist (artist_id) VALUES ($1) RETURNING id", artistID).Scan(&watchlistID); err != nil {
		t.Fatalf("seed watchlist: %v", err)
	}
	tagID := insertTag(t, ctx, pool, "survive-test")
	if err := linkTag(ctx, pool, artistID, tagID); err != nil {
		t.Fatalf("link tag: %v", err)
	}

	var count int
	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", watchlistID); err != nil {
		t.Fatalf("delete watchlist row: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count links after watchlist delete: %v", err)
	}
	if count != 1 {
		t.Fatalf("artist_tags count after watchlist delete = %d, want 1 (TAG-07: links survive)", count)
	}

	if _, err := pool.Exec(ctx, "DELETE FROM artists WHERE id = $1", artistID); err != nil {
		t.Fatalf("delete artists row: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count links after artist delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("artist_tags count after artist delete = %d, want 0 (cascade)", count)
	}

	artistID2 := seedArtistNamed(t, ctx, pool, "cascade-tag")
	tagID2 := insertTag(t, ctx, pool, "cascade-from-tag")
	if err := linkTag(ctx, pool, artistID2, tagID2); err != nil {
		t.Fatalf("link tag 2: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM tags WHERE id = $1", tagID2); err != nil {
		t.Fatalf("delete tags row: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE tag_id = $1", tagID2).Scan(&count); err != nil {
		t.Fatalf("count links after tag delete: %v", err)
	}
	if count != 0 {
		t.Fatalf("artist_tags count after tag delete = %d, want 0 (cascade)", count)
	}
}
