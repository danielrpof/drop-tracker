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
	"os"
	"strings"
	"sync"
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

// TestSchema_TagCapTrigger_RawUpdateRefused closes gap 1 (24-VERIFICATION,
// review WR-03): migration 000011 extends the cap to UPDATE OF artist_id.
func TestSchema_TagCapTrigger_RawUpdateRefused(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	artistA := seedArtistNamed(t, ctx, pool, "a")
	artistB := seedArtistNamed(t, ctx, pool, "b")
	for i := 0; i < 10; i++ {
		tagID := insertTag(t, ctx, pool, fmt.Sprintf("upd-%02d", i))
		if err := linkTag(ctx, pool, artistA, tagID); err != nil {
			t.Fatalf("seed link %d on A: %v", i, err)
		}
	}
	tagB := insertTag(t, ctx, pool, "upd-b")
	if err := linkTag(ctx, pool, artistB, tagB); err != nil {
		t.Fatalf("seed link on B: %v", err)
	}

	_, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3", artistA, artistB, tagB)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("raw UPDATE moving a link onto a 10-link artist: got err %v, want *pgconn.PgError", err)
	}
	if pgErr.Code != pgerrcode.CheckViolation {
		t.Fatalf("raw UPDATE code = %q, want %q (%s)", pgErr.Code, pgerrcode.CheckViolation, pgErr.Message)
	}
	if pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("raw UPDATE constraint = %q, want artist_tags_max_per_artist", pgErr.ConstraintName)
	}

	var countA, countB int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistA).Scan(&countA); err != nil {
		t.Fatalf("count links on A: %v", err)
	}
	if countA != 10 {
		t.Fatalf("artist_tags count for A = %d, want 10 (refused move must not land)", countA)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistB).Scan(&countB); err != nil {
		t.Fatalf("count links on B: %v", err)
	}
	if countB != 1 {
		t.Fatalf("artist_tags count for B = %d, want 1 (link stays on its original artist)", countB)
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

type detachRaceResult struct {
	s3Err           error
	s3ReturnedEarly bool // S3 finished before S2 committed
	finalCount      int  // links on artist A at the end
	hasT, hasU      bool // A linked to T / U at the end
	otherCount      int  // links left on artist B (update mode only)
}

// runDetachRace forces the CR-01 interleaving (ADR 0004 amendment): S1 deletes
// link T uncommitted, S2 re-attaches or moves T, S3 attaches a different tag.
func runDetachRace(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string, viaUpdate bool) detachRaceResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	artistA := seedArtistNamed(t, ctx, pool, label+"-a")
	tagIDs := make([]int64, 10)
	for i := range tagIDs {
		tagIDs[i] = insertTag(t, ctx, pool, fmt.Sprintf("%s-%02d", label, i))
		if err := linkTag(ctx, pool, artistA, tagIDs[i]); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	tagT := tagIDs[0]
	tagU := insertTag(t, ctx, pool, label+"-u")

	s2SQL := `INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2) ON CONFLICT (artist_id, tag_id) DO NOTHING`
	s2Args := []any{artistA, tagT}
	var artistB int64
	if viaUpdate {
		artistB = seedArtistNamed(t, ctx, pool, label+"-b")
		if err := linkTag(ctx, pool, artistB, tagT); err != nil {
			t.Fatalf("seed link on B: %v", err)
		}
		s2SQL = `UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3`
		s2Args = []any{artistA, artistB, tagT}
	}

	var wg sync.WaitGroup
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseS2 := func() { releaseOnce.Do(func() { close(release) }) }

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin S1: %v", err)
	}
	// Every exit path frees S2 and S1 first, or pool.Close would hang on the blocked connections.
	defer func() {
		releaseS2()
		rbCtx, rbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rbCancel()
		_ = tx1.Rollback(rbCtx)
		wg.Wait()
	}()
	if _, err := tx1.Exec(ctx, "DELETE FROM artist_tags WHERE artist_id = $1 AND tag_id = $2", artistA, tagT); err != nil {
		t.Fatalf("S1 delete: %v", err)
	}

	waitingOnLock := func(pid uint32) bool {
		var waitEventType *string
		if err := pool.QueryRow(ctx, "SELECT wait_event_type FROM pg_stat_activity WHERE pid = $1", pid).Scan(&waitEventType); err != nil {
			t.Fatalf("poll pg_stat_activity: %v", err)
		}
		return waitEventType != nil && *waitEventType == "Lock"
	}

	pidCh := make(chan uint32, 1)
	stmtCh := make(chan error, 1)
	commitCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		tx, err := pool.Begin(ctx)
		if err != nil {
			stmtCh <- fmt.Errorf("begin S2: %w", err)
			return
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		pidCh <- tx.Conn().PgConn().PID()
		if _, err := tx.Exec(ctx, s2SQL, s2Args...); err != nil {
			stmtCh <- err
			return
		}
		stmtCh <- nil
		select {
		case <-release:
		case <-ctx.Done():
		}
		commitCh <- tx.Commit(ctx)
	}()

	var pidS2 uint32
	select {
	case pidS2 = <-pidCh:
	case err := <-stmtCh:
		t.Fatalf("S2 setup: %v", err)
	case <-ctx.Done():
		t.Fatal("S2 never started")
	}

	deadline := time.Now().Add(5 * time.Second)
	s2Waiting := false
	for time.Now().Before(deadline) {
		if waitingOnLock(pidS2) {
			s2Waiting = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !s2Waiting {
		t.Fatal("S2 never observed waiting on a Lock -- the race window was not opened")
	}

	if err := tx1.Commit(ctx); err != nil {
		t.Fatalf("commit S1: %v", err)
	}
	select {
	case err := <-stmtCh:
		if err != nil {
			t.Fatalf("S2 statement after S1 commit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("S2 statement never completed after S1 committed")
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire S3 connection: %v", err)
	}
	pidS3 := conn.Conn().PgConn().PID()
	s3Ch := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer conn.Release()
		_, err := conn.Exec(ctx, `INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2) ON CONFLICT (artist_id, tag_id) DO NOTHING`, artistA, tagU)
		s3Ch <- err
	}()

	var res detachRaceResult
	s3Settled := false
	deadline = time.Now().Add(5 * time.Second)
	for !s3Settled && time.Now().Before(deadline) {
		select {
		case res.s3Err = <-s3Ch:
			res.s3ReturnedEarly = true
			s3Settled = true
		default:
			if waitingOnLock(pidS3) {
				s3Settled = true
			} else {
				time.Sleep(50 * time.Millisecond)
			}
		}
	}
	if !s3Settled {
		t.Fatal("S3 neither returned nor blocked on a Lock -- the interleaving was not forced")
	}

	releaseS2()
	select {
	case err := <-commitCh:
		if err != nil {
			t.Fatalf("commit S2: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("S2 commit never completed")
	}
	if !res.s3ReturnedEarly {
		select {
		case res.s3Err = <-s3Ch:
		case <-ctx.Done():
			t.Fatal("S3 never completed after S2 committed")
		}
	}

	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistA).Scan(&res.finalCount); err != nil {
		t.Fatalf("count links on A: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $2)", artistA, tagT).Scan(&res.hasT); err != nil {
		t.Fatalf("check T link: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $2)", artistA, tagU).Scan(&res.hasU); err != nil {
		t.Fatalf("check U link: %v", err)
	}
	if viaUpdate {
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistB).Scan(&res.otherCount); err != nil {
			t.Fatalf("count links on B: %v", err)
		}
	}
	return res
}

// assertDetachRaceRefused checks S3 was held behind S2 and then refused at the cap.
func assertDetachRaceRefused(t *testing.T, res detachRaceResult) {
	t.Helper()
	if res.s3ReturnedEarly {
		t.Fatalf("S3 attached while S2 skipped the artist lock (CR-01): s3Err=%v finalCount=%d", res.s3Err, res.finalCount)
	}
	var pgErr *pgconn.PgError
	if !errors.As(res.s3Err, &pgErr) {
		t.Fatalf("S3: got err %v, want *pgconn.PgError", res.s3Err)
	}
	if pgErr.Code != pgerrcode.CheckViolation || pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("S3 code/constraint = %q/%q, want %q/artist_tags_max_per_artist (%s)", pgErr.Code, pgErr.ConstraintName, pgerrcode.CheckViolation, pgErr.Message)
	}
	if res.finalCount != 10 {
		t.Fatalf("artist_tags count = %d, want 10", res.finalCount)
	}
	if !res.hasT || res.hasU {
		t.Fatalf("links after race: T present=%v U present=%v, want T present and U absent", res.hasT, res.hasU)
	}
}

// TestSchema_TagCapTrigger_ConcurrentDetachRace pins CR-01 (ADR 0004
// amendment, migration 000012): a third attach racing a concurrent detach
// plus re-attach or move of the same link must be refused at 10.
func TestSchema_TagCapTrigger_ConcurrentDetachRace(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	t.Run("insert re-attach", func(t *testing.T) {
		assertDetachRaceRefused(t, runDetachRace(t, ctx, pool, "cdr-ins", false))
	})

	t.Run("update move", func(t *testing.T) {
		res := runDetachRace(t, ctx, pool, "cdr-upd", true)
		assertDetachRaceRefused(t, res)
		if res.otherCount != 0 {
			t.Fatalf("source artist B has %d links, want 0 (T moved to A)", res.otherCount)
		}
	})
}

// TestSchema_TagCapTrigger_DistinctTagConcurrentAt9 is the DB-level pin for
// ADR 0004 Consequences (review IN-09): of two concurrent attaches of
// different new tags at 9 links, the first wins and the second is refused.
func TestSchema_TagCapTrigger_DistinctTagConcurrentAt9(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	artistID := seedArtist(t, ctx, pool)
	for i := 0; i < 9; i++ {
		tagID := insertTag(t, ctx, pool, fmt.Sprintf("distinct-%02d", i))
		if err := linkTag(ctx, pool, artistID, tagID); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	tagX := insertTag(t, ctx, pool, "distinct-x")
	tagY := insertTag(t, ctx, pool, "distinct-y")

	const insertOne = `INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2) ON CONFLICT (artist_id, tag_id) DO NOTHING`

	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin txA: %v", err)
	}
	var wg sync.WaitGroup
	// Rolling back txA on every exit path frees the blocked attach, so pool.Close cannot hang.
	defer func() {
		rbCtx, rbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer rbCancel()
		_ = txA.Rollback(rbCtx)
		wg.Wait()
	}()
	if _, err := txA.Exec(ctx, insertOne, artistID, tagX); err != nil {
		t.Fatalf("txA insert X: %v", err)
	}

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire second connection: %v", err)
	}
	pidB := conn.Conn().PgConn().PID()
	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer conn.Release()
		_, err := conn.Exec(ctx, insertOne, artistID, tagY)
		errCh <- err
	}()

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
		t.Fatal("second attach never observed waiting on a Lock -- the artist-row lock did not serialize it")
	}

	if err := txA.Commit(ctx); err != nil {
		t.Fatalf("commit txA: %v", err)
	}

	var pgErr *pgconn.PgError
	select {
	case err := <-errCh:
		if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.CheckViolation || pgErr.ConstraintName != "artist_tags_max_per_artist" {
			t.Fatalf("second attach after first commit: got err %v, want artist_tags_max_per_artist", err)
		}
	case <-ctx.Done():
		t.Fatal("second attach never completed after txA committed")
	}

	var count int
	var hasX, hasY bool
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistID).Scan(&count); err != nil {
		t.Fatalf("count links: %v", err)
	}
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $2), EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $3)", artistID, tagX, tagY).Scan(&hasX, &hasY); err != nil {
		t.Fatalf("check X/Y links: %v", err)
	}
	if count != 10 || !hasX || hasY {
		t.Fatalf("after race: count=%d X linked=%v Y linked=%v, want 10, true, false", count, hasX, hasY)
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

// TestSchema_TagCapTrigger_UpdatePaths pins the UPDATE-path guarantees 000011
// must and must not disturb: merges still work, unchanged-artist updates are
// free, below-cap moves succeed, and cap-breaking or duplicate moves are
// refused (D-19, ADR 0004 amendment).
func TestSchema_TagCapTrigger_UpdatePaths(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	t.Run("merge-shape", func(t *testing.T) {
		artist := seedArtistNamed(t, ctx, pool, "merge-shape")
		source := insertTag(t, ctx, pool, "merge-shape-source")
		if err := linkTag(ctx, pool, artist, source); err != nil {
			t.Fatalf("link source: %v", err)
		}
		for i := 0; i < 9; i++ {
			tagID := insertTag(t, ctx, pool, fmt.Sprintf("merge-shape-filler-%02d", i))
			if err := linkTag(ctx, pool, artist, tagID); err != nil {
				t.Fatalf("seed filler %d: %v", i, err)
			}
		}
		target := insertTag(t, ctx, pool, "merge-shape-target")

		if _, err := pool.Exec(ctx, "UPDATE artist_tags SET tag_id = $1 WHERE tag_id = $2", target, source); err != nil {
			t.Fatalf("merge-shape UPDATE SET tag_id: %v", err)
		}

		var count, hasTarget, hasSource int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artist).Scan(&count); err != nil {
			t.Fatalf("count links: %v", err)
		}
		if count != 10 {
			t.Fatalf("artist_tags count = %d, want 10", count)
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1 AND tag_id = $2", artist, target).Scan(&hasTarget); err != nil {
			t.Fatalf("count target link: %v", err)
		}
		if hasTarget != 1 {
			t.Fatalf("target linked = %d, want 1", hasTarget)
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1 AND tag_id = $2", artist, source).Scan(&hasSource); err != nil {
			t.Fatalf("count source link: %v", err)
		}
		if hasSource != 0 {
			t.Fatalf("source linked = %d, want 0", hasSource)
		}
	})

	t.Run("unchanged-artist", func(t *testing.T) {
		artist := seedArtistNamed(t, ctx, pool, "unchanged")
		var old int64
		for i := 0; i < 10; i++ {
			tagID := insertTag(t, ctx, pool, fmt.Sprintf("unchanged-%02d", i))
			if err := linkTag(ctx, pool, artist, tagID); err != nil {
				t.Fatalf("seed link %d: %v", i, err)
			}
			if i == 0 {
				old = tagID
			}
		}

		// Changing tag_id too makes the TG_OP guard load-bearing (WR-07): the
		// fresh tag is not skip-existing, so only the guard keeps it from the cap.
		fresh := insertTag(t, ctx, pool, "unchanged-fresh")
		tag, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = artist_id, tag_id = $1 WHERE artist_id = $2 AND tag_id = $3", fresh, artist, old)
		if err != nil {
			t.Fatalf("unchanged-artist UPDATE: %v", err)
		}
		if tag.RowsAffected() != 1 {
			t.Fatalf("rows affected = %d, want 1", tag.RowsAffected())
		}

		var count int
		var hasFresh, hasOld bool
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artist).Scan(&count); err != nil {
			t.Fatalf("count links: %v", err)
		}
		if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $2), EXISTS (SELECT 1 FROM artist_tags WHERE artist_id = $1 AND tag_id = $3)", artist, fresh, old).Scan(&hasFresh, &hasOld); err != nil {
			t.Fatalf("check fresh/old links: %v", err)
		}
		if count != 10 || !hasFresh || hasOld {
			t.Fatalf("after update: count=%d fresh linked=%v old linked=%v, want 10, true, false", count, hasFresh, hasOld)
		}
	})

	t.Run("below-cap move", func(t *testing.T) {
		source := seedArtistNamed(t, ctx, pool, "below-cap-source")
		target := seedArtistNamed(t, ctx, pool, "below-cap-target")
		moving := insertTag(t, ctx, pool, "below-cap-moving")
		if err := linkTag(ctx, pool, source, moving); err != nil {
			t.Fatalf("link moving tag: %v", err)
		}
		for i := 0; i < 9; i++ {
			tagID := insertTag(t, ctx, pool, fmt.Sprintf("below-cap-filler-%02d", i))
			if err := linkTag(ctx, pool, target, tagID); err != nil {
				t.Fatalf("seed filler %d: %v", i, err)
			}
		}

		if _, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3", target, source, moving); err != nil {
			t.Fatalf("below-cap move UPDATE: %v", err)
		}

		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", target).Scan(&count); err != nil {
			t.Fatalf("count target links: %v", err)
		}
		if count != 10 {
			t.Fatalf("target artist_tags count = %d, want 10", count)
		}
	})

	t.Run("two-row move", func(t *testing.T) {
		source := seedArtistNamed(t, ctx, pool, "two-row-source")
		target := seedArtistNamed(t, ctx, pool, "two-row-target")
		moving1 := insertTag(t, ctx, pool, "two-row-moving-1")
		moving2 := insertTag(t, ctx, pool, "two-row-moving-2")
		if err := linkTag(ctx, pool, source, moving1); err != nil {
			t.Fatalf("link moving1: %v", err)
		}
		if err := linkTag(ctx, pool, source, moving2); err != nil {
			t.Fatalf("link moving2: %v", err)
		}
		for i := 0; i < 9; i++ {
			tagID := insertTag(t, ctx, pool, fmt.Sprintf("two-row-filler-%02d", i))
			if err := linkTag(ctx, pool, target, tagID); err != nil {
				t.Fatalf("seed filler %d: %v", i, err)
			}
		}

		_, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2", target, source)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.ConstraintName != "artist_tags_max_per_artist" {
			t.Fatalf("two-row move: got err %v, want artist_tags_max_per_artist", err)
		}

		var targetCount, sourceCount int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", target).Scan(&targetCount); err != nil {
			t.Fatalf("count target links: %v", err)
		}
		if targetCount != 9 {
			t.Fatalf("target artist_tags count = %d, want 9 (whole statement refused)", targetCount)
		}
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", source).Scan(&sourceCount); err != nil {
			t.Fatalf("count source links: %v", err)
		}
		if sourceCount != 2 {
			t.Fatalf("source artist_tags count = %d, want 2 (whole statement refused)", sourceCount)
		}
	})

	t.Run("duplicate move", func(t *testing.T) {
		source := seedArtistNamed(t, ctx, pool, "dup-source")
		target := seedArtistNamed(t, ctx, pool, "dup-target")
		shared := insertTag(t, ctx, pool, "dup-shared")
		if err := linkTag(ctx, pool, source, shared); err != nil {
			t.Fatalf("link shared on source: %v", err)
		}
		if err := linkTag(ctx, pool, target, shared); err != nil {
			t.Fatalf("link shared on target: %v", err)
		}
		for i := 0; i < 9; i++ {
			tagID := insertTag(t, ctx, pool, fmt.Sprintf("dup-filler-%02d", i))
			if err := linkTag(ctx, pool, target, tagID); err != nil {
				t.Fatalf("seed filler %d: %v", i, err)
			}
		}

		_, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3", target, source, shared)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation || pgErr.ConstraintName != "artist_tags_pkey" {
			t.Fatalf("duplicate move: got err %v, want unique violation on artist_tags_pkey", err)
		}

		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", target).Scan(&count); err != nil {
			t.Fatalf("count target links: %v", err)
		}
		if count != 10 {
			t.Fatalf("target artist_tags count = %d, want 10 (duplicate move refused)", count)
		}
	})
}

// TestSchema_Migration000011_DownUpRoundTrip proves the 000011 pair
// round-trips: down restores INSERT-only behavior (UPDATE opens, INSERT cap
// stays), and up re-closes the UPDATE path (ADR 0004 amendment).
func TestSchema_Migration000011_DownUpRoundTrip(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	downSQL, err := os.ReadFile("migrations/000011_artist_tags_cap_on_update.down.sql")
	if err != nil {
		t.Fatalf("read down.sql: %v", err)
	}
	upSQL, err := os.ReadFile("migrations/000011_artist_tags_cap_on_update.up.sql")
	if err != nil {
		t.Fatalf("read up.sql: %v", err)
	}

	triggerCount := func() int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_trigger WHERE tgrelid = 'artist_tags'::regclass AND tgname = 'artist_tags_cap_update_trigger'").Scan(&n); err != nil {
			t.Fatalf("count update trigger: %v", err)
		}
		return n
	}

	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("exec down.sql: %v", err)
	}
	if n := triggerCount(); n != 0 {
		t.Fatalf("update trigger count after down = %d, want 0", n)
	}

	artistDown := seedArtistNamed(t, ctx, pool, "roundtrip-down")
	sourceDown := seedArtistNamed(t, ctx, pool, "roundtrip-down-source")
	for i := 0; i < 10; i++ {
		tagID := insertTag(t, ctx, pool, fmt.Sprintf("roundtrip-down-%02d", i))
		if err := linkTag(ctx, pool, artistDown, tagID); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	movingTag := insertTag(t, ctx, pool, "roundtrip-down-moving")
	if err := linkTag(ctx, pool, sourceDown, movingTag); err != nil {
		t.Fatalf("link moving tag: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3", artistDown, sourceDown, movingTag); err != nil {
		t.Fatalf("UPDATE onto 10-link artist after down: %v", err)
	}

	extraTag := insertTag(t, ctx, pool, "roundtrip-down-extra")
	_, err = pool.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", artistDown, extraTag)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("raw INSERT after down: got err %v, want artist_tags_max_per_artist (INSERT trigger unaffected)", err)
	}

	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("exec up.sql: %v", err)
	}
	if n := triggerCount(); n != 1 {
		t.Fatalf("update trigger count after up = %d, want 1", n)
	}

	artistUp := seedArtistNamed(t, ctx, pool, "roundtrip-up")
	sourceUp := seedArtistNamed(t, ctx, pool, "roundtrip-up-source")
	for i := 0; i < 10; i++ {
		tagID := insertTag(t, ctx, pool, fmt.Sprintf("roundtrip-up-%02d", i))
		if err := linkTag(ctx, pool, artistUp, tagID); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
	movingTagUp := insertTag(t, ctx, pool, "roundtrip-up-moving")
	if err := linkTag(ctx, pool, sourceUp, movingTagUp); err != nil {
		t.Fatalf("link moving tag: %v", err)
	}
	_, err = pool.Exec(ctx, "UPDATE artist_tags SET artist_id = $1 WHERE artist_id = $2 AND tag_id = $3", artistUp, sourceUp, movingTagUp)
	if !errors.As(err, &pgErr) || pgErr.ConstraintName != "artist_tags_max_per_artist" {
		t.Fatalf("UPDATE onto 10-link artist after up: got err %v, want artist_tags_max_per_artist", err)
	}
}

// TestSchema_Migration000012_DownUpRoundTrip proves the 000012 pair reopens
// and re-closes CR-01: down restores 000011's visibility-only check (the race
// reaches 11), up locks the existing link again (the race is refused at 10).
func TestSchema_Migration000012_DownUpRoundTrip(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_schema_test")
	ctx := context.Background()

	downSQL, err := os.ReadFile("migrations/000012_artist_tags_cap_concurrent_detach.down.sql")
	if err != nil {
		t.Fatalf("read down.sql: %v", err)
	}
	upSQL, err := os.ReadFile("migrations/000012_artist_tags_cap_concurrent_detach.up.sql")
	if err != nil {
		t.Fatalf("read up.sql: %v", err)
	}

	if _, err := pool.Exec(ctx, string(downSQL)); err != nil {
		t.Fatalf("exec down.sql: %v", err)
	}
	down := runDetachRace(t, ctx, pool, "rt-down", false)
	if down.s3Err != nil || !down.s3ReturnedEarly || down.finalCount != 11 {
		t.Fatalf("after down: s3Err=%v returnedEarly=%v finalCount=%d, want nil, true, 11 (000011 body leaves the hole open)", down.s3Err, down.s3ReturnedEarly, down.finalCount)
	}

	if _, err := pool.Exec(ctx, string(upSQL)); err != nil {
		t.Fatalf("exec up.sql: %v", err)
	}
	assertDetachRaceRefused(t, runDetachRace(t, ctx, pool, "rt-up", false))
}
