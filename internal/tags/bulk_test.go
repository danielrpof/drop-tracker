package tags_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/tags"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedLinks inserts n tags named prefix-00.. and links each to artistID.
func seedLinks(t *testing.T, ctx context.Context, pool *pgxpool.Pool, artistID int64, n int, prefix string) {
	t.Helper()
	for i := 0; i < n; i++ {
		var tagID int64
		if err := pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ($1) RETURNING id", fmt.Sprintf("%s-%02d", prefix, i)).Scan(&tagID); err != nil {
			t.Fatalf("seed tag %d: %v", i, err)
		}
		if _, err := pool.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", artistID, tagID); err != nil {
			t.Fatalf("seed link %d: %v", i, err)
		}
	}
}

func countLinks(t *testing.T, ctx context.Context, pool *pgxpool.Pool, artistID int64) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags WHERE artist_id = $1", artistID).Scan(&n); err != nil {
		t.Fatalf("count links: %v", err)
	}
	return n
}

func countAllLinks(t *testing.T, ctx context.Context, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM artist_tags").Scan(&n); err != nil {
		t.Fatalf("count all links: %v", err)
	}
	return n
}

func deleteEntry(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entryID int64) {
	t.Helper()
	if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", entryID); err != nil {
		t.Fatalf("delete entry: %v", err)
	}
}

func TestService_BulkAttach(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	t.Run("mixed selection", func(t *testing.T) {
		a, _ := seedEntry(t, ctx, pool, "room")
		b, bArtist := seedEntry(t, ctx, pool, "capped")
		c, cArtist := seedEntry(t, ctx, pool, "carrier")
		d, _ := seedEntry(t, ctx, pool, "gone")
		seedLinks(t, ctx, pool, bArtist, 10, "mixed-cap")
		if _, err := svc.Attach(ctx, c, "mixed"); err != nil {
			t.Fatalf("pre-tag carrier: %v", err)
		}
		deleteEntry(t, ctx, pool, d)
		before := countLinks(t, ctx, pool, cArtist)

		got, err := svc.BulkAttach(ctx, []int64{d, c, b, a}, "mixed")
		if err != nil {
			t.Fatalf("BulkAttach: %v", err)
		}
		if want := []int64{a, c}; !reflect.DeepEqual(got.Applied, want) {
			t.Fatalf("Applied = %v, want %v", got.Applied, want)
		}
		want := []tags.BulkSkip{
			{ID: b, Name: "capped", Reason: tags.SkipReasonCap},
			{ID: d, Reason: tags.SkipReasonGone},
		}
		if !reflect.DeepEqual(got.Skipped, want) {
			t.Fatalf("Skipped = %+v, want %+v", got.Skipped, want)
		}
		if got.Tag.ID == 0 || got.Tag.Name != "mixed" {
			t.Fatalf("Tag = %+v, want a stored tag named mixed", got.Tag)
		}
		if after := countLinks(t, ctx, pool, cArtist); after != before {
			t.Fatalf("carrier links = %d, want unchanged %d", after, before)
		}
		if n := countLinks(t, ctx, pool, bArtist); n != 10 {
			t.Fatalf("capped artist links = %d, want 10", n)
		}
	})

	t.Run("nine links takes the tenth", func(t *testing.T) {
		nine, nineArtist := seedEntry(t, ctx, pool, "nine")
		seedLinks(t, ctx, pool, nineArtist, 9, "adj-nine")

		got, err := svc.BulkAttach(ctx, []int64{nine}, "adjacent")
		if err != nil {
			t.Fatalf("BulkAttach: %v", err)
		}
		if want := []int64{nine}; !reflect.DeepEqual(got.Applied, want) || len(got.Skipped) != 0 {
			t.Fatalf("result = %+v, want only %v applied", got, want)
		}
		if n := countLinks(t, ctx, pool, nineArtist); n != 10 {
			t.Fatalf("artist now has %d links, want 10", n)
		}
	})

	t.Run("ten links including the tag is applied not cap", func(t *testing.T) {
		ten, tenArtist := seedEntry(t, ctx, pool, "ten-with-tag")
		seedLinks(t, ctx, pool, tenArtist, 9, "adj-ten")
		if _, err := svc.Attach(ctx, ten, "carried"); err != nil {
			t.Fatalf("tag the ten-link artist: %v", err)
		}

		got, err := svc.BulkAttach(ctx, []int64{ten}, "carried")
		if err != nil {
			t.Fatalf("BulkAttach: %v", err)
		}
		if want := []int64{ten}; !reflect.DeepEqual(got.Applied, want) || len(got.Skipped) != 0 {
			t.Fatalf("result = %+v, want only %v applied", got, want)
		}
		if n := countLinks(t, ctx, pool, tenArtist); n != 10 {
			t.Fatalf("artist has %d links, want 10 unchanged", n)
		}
	})

	t.Run("idempotent", func(t *testing.T) {
		a, _ := seedEntry(t, ctx, pool, "idem-a")
		b, _ := seedEntry(t, ctx, pool, "idem-b")

		first, err := svc.BulkAttach(ctx, []int64{a, b}, "idem")
		if err != nil {
			t.Fatalf("first BulkAttach: %v", err)
		}
		total := countAllLinks(t, ctx, pool)
		second, err := svc.BulkAttach(ctx, []int64{a, b}, "idem")
		if err != nil {
			t.Fatalf("second BulkAttach: %v", err)
		}
		if !reflect.DeepEqual(first.Applied, second.Applied) {
			t.Fatalf("Applied differs: %v vs %v", first.Applied, second.Applied)
		}
		if after := countAllLinks(t, ctx, pool); after != total {
			t.Fatalf("total links = %d after repeat, want %d", after, total)
		}
	})

	t.Run("blank name", func(t *testing.T) {
		a, _ := seedEntry(t, ctx, pool, "blank")
		_, err := svc.BulkAttach(ctx, []int64{a}, "   ")
		if !errors.Is(err, tags.ErrNameRequired) {
			t.Fatalf("err = %v, want ErrNameRequired", err)
		}
		if n := countTagsByLowerName(t, ctx, pool, ""); n != 0 {
			t.Fatalf("tags with empty name = %d, want 0", n)
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		got, err := svc.BulkAttach(ctx, nil, "nothing")
		if err != nil {
			t.Fatalf("BulkAttach: %v", err)
		}
		if got.Tag != (tags.Tag{}) || got.Applied == nil || got.Skipped == nil || len(got.Applied)+len(got.Skipped) != 0 {
			t.Fatalf("result = %+v, want zero tag and empty non-nil slices", got)
		}
		if n := countTagsByLowerName(t, ctx, pool, "nothing"); n != 0 {
			t.Fatalf("tag rows = %d, want 0", n)
		}
	})
}

func TestService_BulkAttach_AllCappedLeavesNoTag(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, aArtist := seedEntry(t, ctx, pool, "full-a")
	b, bArtist := seedEntry(t, ctx, pool, "full-b")
	seedLinks(t, ctx, pool, aArtist, 10, "full-a")
	seedLinks(t, ctx, pool, bArtist, 10, "full-b")

	got, err := svc.BulkAttach(ctx, []int64{a, b}, "orphan")
	if err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(got.Applied) != 0 {
		t.Fatalf("Applied = %v, want empty", got.Applied)
	}
	want := []tags.BulkSkip{
		{ID: a, Name: "full-a", Reason: tags.SkipReasonCap},
		{ID: b, Name: "full-b", Reason: tags.SkipReasonCap},
	}
	if !reflect.DeepEqual(got.Skipped, want) {
		t.Fatalf("Skipped = %+v, want %+v", got.Skipped, want)
	}
	if got.Tag.ID != 0 {
		t.Fatalf("Tag.ID = %d, want 0 after rollback", got.Tag.ID)
	}
	if n := countTagsByLowerName(t, ctx, pool, "orphan"); n != 0 {
		t.Fatalf("tag rows = %d, want 0 (rolled back)", n)
	}
}

func TestService_BulkAttach_AllGoneLeavesNoTag(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, _ := seedEntry(t, ctx, pool, "vanish-a")
	b, _ := seedEntry(t, ctx, pool, "vanish-b")
	deleteEntry(t, ctx, pool, a)
	deleteEntry(t, ctx, pool, b)

	got, err := svc.BulkAttach(ctx, []int64{b, a}, "ghost")
	if err != nil {
		t.Fatalf("BulkAttach: %v", err)
	}
	if len(got.Applied) != 0 || got.Tag.ID != 0 {
		t.Fatalf("result = %+v, want nothing applied and tag id 0", got)
	}
	want := []tags.BulkSkip{{ID: a, Reason: tags.SkipReasonGone}, {ID: b, Reason: tags.SkipReasonGone}}
	if !reflect.DeepEqual(got.Skipped, want) {
		t.Fatalf("Skipped = %+v, want %+v", got.Skipped, want)
	}
	if n := countTagsByLowerName(t, ctx, pool, "ghost"); n != 0 {
		t.Fatalf("tag rows = %d, want 0", n)
	}
}

// Reversed id lists with distinct tags would deadlock without the ordered pre-lock (D-19).
func TestService_BulkAttach_ConcurrentOverlappingNoDeadlock(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	svc := tags.NewService(pool)

	const shared, rounds = 5, 4
	entries := make([]int64, shared)
	artists := make([]int64, shared)
	for i := range entries {
		entries[i], artists[i] = seedEntry(t, ctx, pool, fmt.Sprintf("shared-%d", i))
	}
	reversed := make([]int64, shared)
	for i, id := range entries {
		reversed[shared-1-i] = id
	}

	for round := 0; round < rounds; round++ {
		start := make(chan struct{})
		errs := make(chan error, 2)
		results := make(chan tags.BulkAttachResult, 2)
		var wg sync.WaitGroup
		for g, ids := range [][]int64{entries, reversed} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				res, err := svc.BulkAttach(ctx, ids, fmt.Sprintf("g%d-r%d", g, round))
				errs <- err
				results <- res
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		close(results)
		for err := range errs {
			if err != nil {
				if strings.Contains(err.Error(), "40P01") {
					t.Fatalf("round %d: deadlock between overlapping bulk attaches: %v", round, err)
				}
				t.Fatalf("round %d: BulkAttach: %v", round, err)
			}
		}
		for res := range results {
			if len(res.Applied) != shared {
				t.Fatalf("round %d: Applied = %v, want all %d", round, res.Applied, shared)
			}
		}
	}

	for _, artistID := range artists {
		if n := countLinks(t, ctx, pool, artistID); n != rounds*2 {
			t.Fatalf("artist %d links = %d, want %d", artistID, n, rounds*2)
		}
	}
}

// A bulk add queued behind another transaction's uncommitted 10th link must
// re-read the count after the lock is granted and skip that artist as cap.
func TestService_BulkAttach_WaitsOnConcurrentTenthLink(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	busy, busyArtist := seedEntry(t, ctx, pool, "busy")
	other, _ := seedEntry(t, ctx, pool, "other")
	seedLinks(t, ctx, pool, busyArtist, 9, "busy")

	var tenth int64
	if err := pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ('busy-tenth') RETURNING id").Scan(&tenth); err != nil {
		t.Fatalf("seed tenth tag: %v", err)
	}
	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin txA: %v", err)
	}
	defer func() { _ = txA.Rollback(ctx) }()
	if _, err := txA.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", busyArtist, tenth); err != nil {
		t.Fatalf("txA tenth link: %v", err)
	}
	pidA := txA.Conn().PgConn().PID()

	type outcome struct {
		res tags.BulkAttachResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := svc.BulkAttach(ctx, []int64{busy, other}, "late")
		done <- outcome{res, err}
	}()

	deadline := time.Now().Add(5 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		var n int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM pg_stat_activity WHERE $1 = ANY(pg_blocking_pids(pid))", int32(pidA)).Scan(&n); err != nil {
			t.Fatalf("poll pg_stat_activity: %v", err)
		}
		if n >= 1 {
			blocked = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("BulkAttach never blocked behind txA: the pre-lock did not serialize on the artist row")
	}

	if err := txA.Commit(ctx); err != nil {
		t.Fatalf("commit txA: %v", err)
	}
	out := <-done
	if out.err != nil {
		t.Fatalf("BulkAttach after txA commit: %v", out.err)
	}
	if want := []int64{other}; !reflect.DeepEqual(out.res.Applied, want) {
		t.Fatalf("Applied = %v, want %v", out.res.Applied, want)
	}
	want := []tags.BulkSkip{{ID: busy, Name: "busy", Reason: tags.SkipReasonCap}}
	if !reflect.DeepEqual(out.res.Skipped, want) {
		t.Fatalf("Skipped = %+v, want %+v", out.res.Skipped, want)
	}
	if n := countLinks(t, ctx, pool, busyArtist); n != 10 {
		t.Fatalf("busy artist links = %d, want exactly 10", n)
	}
}

func TestService_BulkDetach(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	t.Run("removes from carriers only and keeps the tag", func(t *testing.T) {
		a, aArtist := seedEntry(t, ctx, pool, "carrier-a")
		b, bArtist := seedEntry(t, ctx, pool, "non-carrier")
		c, _ := seedEntry(t, ctx, pool, "carrier-c")
		d, dArtist := seedEntry(t, ctx, pool, "unselected")
		var tagID int64
		for _, id := range []int64{a, c, d} {
			res, err := svc.Attach(ctx, id, "detach-keep")
			if err != nil {
				t.Fatalf("attach: %v", err)
			}
			tagID = res.Tag.ID
		}
		if _, err := svc.Attach(ctx, b, "detach-other"); err != nil {
			t.Fatalf("attach other: %v", err)
		}
		bBefore := countLinks(t, ctx, pool, bArtist)
		dBefore := countLinks(t, ctx, pool, dArtist)

		got, err := svc.BulkDetach(ctx, []int64{c, b, a}, tagID)
		if err != nil {
			t.Fatalf("BulkDetach: %v", err)
		}
		if want := []int64{a, c}; !reflect.DeepEqual(got.Applied, want) {
			t.Fatalf("Applied = %v, want %v", got.Applied, want)
		}
		if len(got.Gone) != 0 {
			t.Fatalf("Gone = %v, want empty", got.Gone)
		}
		if n := countLinks(t, ctx, pool, aArtist); n != 0 {
			t.Fatalf("carrier A links = %d, want 0", n)
		}
		if n := countLinks(t, ctx, pool, bArtist); n != bBefore {
			t.Fatalf("non-carrier links = %d, want unchanged %d", n, bBefore)
		}
		if n := countLinks(t, ctx, pool, dArtist); n != dBefore {
			t.Fatalf("unselected carrier links = %d, want unchanged %d", n, dBefore)
		}
		if n := countTagsByLowerName(t, ctx, pool, "detach-keep"); n != 1 {
			t.Fatalf("tag rows = %d, want the tag kept", n)
		}
	})

	t.Run("repeat and non-carriers apply nothing", func(t *testing.T) {
		a, _ := seedEntry(t, ctx, pool, "repeat-a")
		b, _ := seedEntry(t, ctx, pool, "repeat-b")
		res, err := svc.Attach(ctx, a, "detach-repeat")
		if err != nil {
			t.Fatalf("attach: %v", err)
		}
		if got, err := svc.BulkDetach(ctx, []int64{a}, res.Tag.ID); err != nil || !reflect.DeepEqual(got.Applied, []int64{a}) {
			t.Fatalf("first = %+v, %v; want applied [%d]", got, err, a)
		}
		got, err := svc.BulkDetach(ctx, []int64{a}, res.Tag.ID)
		if err != nil {
			t.Fatalf("second BulkDetach: %v", err)
		}
		if got.Applied == nil || len(got.Applied) != 0 {
			t.Fatalf("second Applied = %#v, want empty non-nil", got.Applied)
		}
		got, err = svc.BulkDetach(ctx, []int64{b}, res.Tag.ID)
		if err != nil || len(got.Applied) != 0 {
			t.Fatalf("non-carrier = %+v, %v; want nothing applied", got, err)
		}
	})

	t.Run("gone ids are reported", func(t *testing.T) {
		a, _ := seedEntry(t, ctx, pool, "gone-a")
		g, _ := seedEntry(t, ctx, pool, "gone-g")
		res, err := svc.Attach(ctx, a, "detach-gone")
		if err != nil {
			t.Fatalf("attach: %v", err)
		}
		deleteEntry(t, ctx, pool, g)

		got, err := svc.BulkDetach(ctx, []int64{g, a, g}, res.Tag.ID)
		if err != nil {
			t.Fatalf("BulkDetach: %v", err)
		}
		if !reflect.DeepEqual(got.Applied, []int64{a}) || !reflect.DeepEqual(got.Gone, []int64{g}) {
			t.Fatalf("result = %+v, want applied [%d] gone [%d]", got, a, g)
		}
	})

	t.Run("empty ids", func(t *testing.T) {
		got, err := svc.BulkDetach(ctx, nil, 1)
		if err != nil {
			t.Fatalf("BulkDetach: %v", err)
		}
		if got.Applied == nil || got.Gone == nil || len(got.Applied)+len(got.Gone) != 0 {
			t.Fatalf("result = %#v, want empty non-nil slices", got)
		}
	})
}

func TestService_BulkDetach_UnknownTag(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "tags_bulk_test")
	ctx := context.Background()
	svc := tags.NewService(pool)

	a, _ := seedEntry(t, ctx, pool, "unknown-tag")
	if _, err := svc.Attach(ctx, a, "stays"); err != nil {
		t.Fatalf("attach: %v", err)
	}
	before := countAllLinks(t, ctx, pool)

	_, err := svc.BulkDetach(ctx, []int64{a}, 999999999)
	if !errors.Is(err, tags.ErrTagNotFound) {
		t.Fatalf("err = %v, want ErrTagNotFound", err)
	}
	if after := countAllLinks(t, ctx, pool); after != before {
		t.Fatalf("links = %d, want unchanged %d", after, before)
	}
}
