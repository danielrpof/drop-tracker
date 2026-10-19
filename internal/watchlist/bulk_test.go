package watchlist_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/testutil"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
)

// seedBulkEntries adds n entries and returns their ids in insertion order.
func seedBulkEntries(t *testing.T, ctx context.Context, svc *watchlist.Service, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for i := range n {
		e, err := svc.Add(ctx, watchlist.AddParams{
			MBID: testMBID(t) + "-" + string(rune('a'+i)),
			Name: "Bulk Artist " + string(rune('A'+i)),
		})
		if err != nil {
			t.Fatalf("seed entry %d: %v", i, err)
		}
		ids = append(ids, e.ID)
	}
	return ids
}

func countRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func assertIDs(t *testing.T, label string, got, want []int64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want non-nil %v", label, want)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestService_BulkRemove(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	t.Run("removes every existing row", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		res, err := svc.BulkRemove(ctx, []int64{ids[2], ids[0], ids[1]})
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, ids)
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{})
		for _, id := range ids {
			if n := countRows(t, ctx, pool, "SELECT count(*) FROM watchlist WHERE id = $1", id); n != 0 {
				t.Fatalf("watchlist row %d still present", id)
			}
		}
	})

	t.Run("duplicates and order are normalized", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		a, c := ids[0], ids[2]
		res, err := svc.BulkRemove(ctx, []int64{c, a, 999999999, a})
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{a, c})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{999999999})
		if n := countRows(t, ctx, pool, "SELECT count(*) FROM watchlist WHERE id = $1", ids[1]); n != 1 {
			t.Fatalf("unselected row count = %d, want 1", n)
		}
	})

	t.Run("an id deleted elsewhere is already gone", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		if _, err := pool.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", ids[1]); err != nil {
			t.Fatalf("direct delete: %v", err)
		}
		res, err := svc.BulkRemove(ctx, ids)
		if err != nil {
			t.Fatalf("BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{ids[0], ids[2]})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{ids[1]})
	})

	t.Run("a repeat reports everything already gone", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 2)
		if _, err := svc.BulkRemove(ctx, ids); err != nil {
			t.Fatalf("first BulkRemove: %v", err)
		}
		res, err := svc.BulkRemove(ctx, ids)
		if err != nil {
			t.Fatalf("second BulkRemove: %v", err)
		}
		assertIDs(t, "Removed", res.Removed, []int64{})
		assertIDs(t, "AlreadyGone", res.AlreadyGone, ids)
	})

	t.Run("no ids returns empty non-nil slices", func(t *testing.T) {
		for _, in := range [][]int64{nil, {}} {
			res, err := svc.BulkRemove(ctx, in)
			if err != nil {
				t.Fatalf("BulkRemove(%v): %v", in, err)
			}
			assertIDs(t, "Removed", res.Removed, []int64{})
			assertIDs(t, "AlreadyGone", res.AlreadyGone, []int64{})
		}
	})
}

func TestService_BulkRemove_KeepsArtistTagsAndPendingEvents(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	mbid := testMBID(t)
	entry, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Keeper"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	note := "remember me"
	if _, err := svc.UpdateNote(ctx, entry.ID, &note); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	var tagID int64
	if err := pool.QueryRow(ctx, "INSERT INTO tags (name) VALUES ($1) RETURNING id", "bulk-keeper").Scan(&tagID); err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO artist_tags (artist_id, tag_id) VALUES ($1, $2)", entry.ArtistID, tagID); err != nil {
		t.Fatalf("seed artist_tags: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO events (artist_id, source, event_type, external_id, title, artist_name)
		VALUES ($1, 'musicbrainz', 'new_release', $2, 'Title', 'Keeper')`, entry.ArtistID, "bulk-keeps-"+mbid); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	res, err := svc.BulkRemove(ctx, []int64{entry.ID})
	if err != nil {
		t.Fatalf("BulkRemove: %v", err)
	}
	assertIDs(t, "Removed", res.Removed, []int64{entry.ID})

	if n := countRows(t, ctx, pool, "SELECT count(*) FROM artists WHERE id = $1", entry.ArtistID); n != 1 {
		t.Fatalf("artists rows = %d, want 1", n)
	}
	if n := countRows(t, ctx, pool, "SELECT count(*) FROM artist_tags WHERE artist_id = $1 AND tag_id = $2", entry.ArtistID, tagID); n != 1 {
		t.Fatalf("artist_tags links = %d, want 1", n)
	}
	if n := countRows(t, ctx, pool, "SELECT count(*) FROM events WHERE artist_id = $1 AND notified_at IS NULL", entry.ArtistID); n != 1 {
		t.Fatalf("unnotified events = %d, want 1", n)
	}

	again, err := svc.Add(ctx, watchlist.AddParams{MBID: mbid, Name: "Keeper"})
	if err != nil {
		t.Fatalf("re-Add: %v", err)
	}
	if again.Note != nil {
		t.Fatalf("Note after re-add = %q, want nil", *again.Note)
	}
}

type storedPrefs struct {
	release, mute []string
	note          *string
	updatedAt     time.Time
}

func readPrefs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64) storedPrefs {
	t.Helper()
	var p storedPrefs
	row := pool.QueryRow(ctx, "SELECT release_types, muted_event_types, note, updated_at FROM watchlist WHERE id = $1", id)
	if err := row.Scan(&p.release, &p.mute, &p.note, &p.updatedAt); err != nil {
		t.Fatalf("read prefs for %d: %v", id, err)
	}
	return p
}

func setRawPrefs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id int64, release, mute []string) {
	t.Helper()
	if _, err := pool.Exec(ctx, "UPDATE watchlist SET release_types = $2::text[], muted_event_types = $3::text[] WHERE id = $1", id, release, mute); err != nil {
		t.Fatalf("set raw prefs for %d: %v", id, err)
	}
}

func assertStrings(t *testing.T, label string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", label, got, want)
	}
}

func TestService_BulkUpdatePreferences(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))
	set := func(rel, mute []string) watchlist.PreferencesDelta {
		return watchlist.PreferencesDelta{
			ReleaseTypes:    watchlist.AxisDelta{Set: rel},
			MutedEventTypes: watchlist.AxisDelta{Set: mute},
		}
	}

	t.Run("set adds a type and keeps each artist's own others", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 3)
		setRawPrefs(t, ctx, pool, ids[0], []string{"album"}, []string{})
		setRawPrefs(t, ctx, pool, ids[1], []string{"single", "ep"}, []string{"new_release"})
		// ids[2] keeps the defaults, which already include ep.
		res, err := svc.BulkUpdatePreferences(ctx, ids, set([]string{"ep"}, nil))
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		assertIDs(t, "applied", res.Applied, ids)
		assertStrings(t, "A release", readPrefs(t, ctx, pool, ids[0]).release, []string{"album", "ep"})
		assertStrings(t, "B release", readPrefs(t, ctx, pool, ids[1]).release, []string{"single", "ep"})
		assertStrings(t, "B mute", readPrefs(t, ctx, pool, ids[1]).mute, []string{"new_release"})
		assertStrings(t, "C release", readPrefs(t, ctx, pool, ids[2]).release, []string{"album", "single", "ep", "deluxe"})
	})

	t.Run("canonical order regardless of stored or requested order", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 1)
		setRawPrefs(t, ctx, pool, ids[0], []string{"deluxe", "album"}, []string{"deluxe_change", "new_release"})
		_, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
			ReleaseTypes:    watchlist.AxisDelta{Set: []string{"ep"}, Unset: []string{"deluxe"}},
			MutedEventTypes: watchlist.AxisDelta{Set: []string{"guest_feature"}},
		})
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		p := readPrefs(t, ctx, pool, ids[0])
		assertStrings(t, "release", p.release, []string{"album", "ep"})
		assertStrings(t, "mute", p.mute, []string{"new_release", "guest_feature", "deluxe_change"})
	})

	t.Run("no-op delta leaves the row and updated_at unchanged", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 2)
		setRawPrefs(t, ctx, pool, ids[0], []string{"album", "single"}, []string{})
		setRawPrefs(t, ctx, pool, ids[1], []string{"album"}, []string{})
		beforeNoop := readPrefs(t, ctx, pool, ids[0])
		beforeChanged := readPrefs(t, ctx, pool, ids[1])

		// A already has single and lacks ep (unset no-op); B lacks single.
		res, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
			ReleaseTypes: watchlist.AxisDelta{Set: []string{"single"}, Unset: []string{"ep"}},
		})
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		assertIDs(t, "applied", res.Applied, ids)
		afterNoop := readPrefs(t, ctx, pool, ids[0])
		if !afterNoop.updatedAt.Equal(beforeNoop.updatedAt) {
			t.Fatalf("no-op updated_at moved: %v -> %v", beforeNoop.updatedAt, afterNoop.updatedAt)
		}
		assertStrings(t, "no-op release", afterNoop.release, []string{"album", "single"})
		afterChanged := readPrefs(t, ctx, pool, ids[1])
		if !afterChanged.updatedAt.After(beforeChanged.updatedAt) {
			t.Fatalf("changed updated_at did not advance: %v -> %v", beforeChanged.updatedAt, afterChanged.updatedAt)
		}
		assertStrings(t, "changed release", afterChanged.release, []string{"album", "single"})
	})

	t.Run("unsetting every release type stores an empty set", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 1)
		_, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
			ReleaseTypes: watchlist.AxisDelta{Unset: []string{"album", "single", "ep", "deluxe"}},
		})
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		if p := readPrefs(t, ctx, pool, ids[0]); p.release == nil || len(p.release) != 0 {
			t.Fatalf("release = %#v, want empty non-nil", p.release)
		}
	})

	t.Run("note is untouched", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 1)
		if _, err := svc.UpdateNote(ctx, ids[0], new("remember me")); err != nil {
			t.Fatalf("UpdateNote: %v", err)
		}
		if _, err := svc.BulkUpdatePreferences(ctx, ids, set([]string{"ep"}, []string{"guest_feature"})); err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		p := readPrefs(t, ctx, pool, ids[0])
		if p.note == nil || *p.note != "remember me" {
			t.Fatalf("note = %v, want remember me", p.note)
		}
	})

	t.Run("gone and duplicate ids are reported once", func(t *testing.T) {
		ids := seedBulkEntries(t, ctx, svc, 1)
		a := ids[0]
		res, err := svc.BulkUpdatePreferences(ctx, []int64{a, 999999999, a}, set(nil, []string{"guest_feature"}))
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		assertIDs(t, "applied", res.Applied, []int64{a})
		assertIDs(t, "gone", res.Gone, []int64{999999999})
	})
}

// Pitfall B1: a nil Unset/Set slice must never reach SQL as NULL and wipe types.
func TestService_BulkUpdatePreferences_NilSlicesNeverWipe(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))

	ids := seedBulkEntries(t, ctx, svc, 3)
	setRawPrefs(t, ctx, pool, ids[0], []string{"album", "deluxe"}, []string{"new_release"})
	setRawPrefs(t, ctx, pool, ids[1], []string{"ep"}, []string{})
	setRawPrefs(t, ctx, pool, ids[2], []string{"single"}, []string{"deluxe_change"})

	_, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
		ReleaseTypes:    watchlist.AxisDelta{Set: []string{"ep"}, Unset: nil},
		MutedEventTypes: watchlist.AxisDelta{Set: nil, Unset: nil},
	})
	if err != nil {
		t.Fatalf("BulkUpdatePreferences: %v", err)
	}
	assertStrings(t, "A release", readPrefs(t, ctx, pool, ids[0]).release, []string{"album", "ep", "deluxe"})
	assertStrings(t, "B release", readPrefs(t, ctx, pool, ids[1]).release, []string{"ep"})
	assertStrings(t, "C release", readPrefs(t, ctx, pool, ids[2]).release, []string{"single", "ep"})
	assertStrings(t, "A mute", readPrefs(t, ctx, pool, ids[0]).mute, []string{"new_release"})
	assertStrings(t, "C mute", readPrefs(t, ctx, pool, ids[2]).mute, []string{"deluxe_change"})
}

func TestService_BulkUpdatePreferences_Validation(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx := context.Background()
	svc := watchlist.NewService(sqlc.New(pool))
	ids := seedBulkEntries(t, ctx, svc, 1)
	before := readPrefs(t, ctx, pool, ids[0])

	cases := []struct {
		name string
		d    watchlist.PreferencesDelta
		want error
	}{
		{"set and unset overlap on release", watchlist.PreferencesDelta{ReleaseTypes: watchlist.AxisDelta{Set: []string{"ep"}, Unset: []string{"ep"}}}, watchlist.ErrPreferenceConflict},
		{"set and unset overlap on mute", watchlist.PreferencesDelta{MutedEventTypes: watchlist.AxisDelta{Set: []string{"new_release"}, Unset: []string{"new_release"}}}, watchlist.ErrPreferenceConflict},
		{"unknown release type", watchlist.PreferencesDelta{ReleaseTypes: watchlist.AxisDelta{Set: []string{"vinyl"}}}, watchlist.ErrInvalidReleaseType},
		{"unknown event type", watchlist.PreferencesDelta{MutedEventTypes: watchlist.AxisDelta{Unset: []string{"tweet"}}}, watchlist.ErrInvalidEventType},
		{"all lists empty", watchlist.PreferencesDelta{
			ReleaseTypes:    watchlist.AxisDelta{Set: []string{}, Unset: []string{}},
			MutedEventTypes: watchlist.AxisDelta{Set: []string{}, Unset: []string{}},
		}, watchlist.ErrNoPreferencesSupplied},
		{"all lists nil", watchlist.PreferencesDelta{}, watchlist.ErrNoPreferencesSupplied},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.BulkUpdatePreferences(ctx, ids, tc.d)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	after := readPrefs(t, ctx, pool, ids[0])
	assertStrings(t, "release unchanged", after.release, before.release)
	if !after.updatedAt.Equal(before.updatedAt) {
		t.Fatalf("updated_at moved after rejected deltas")
	}

	t.Run("validation outranks empty ids", func(t *testing.T) {
		_, err := svc.BulkUpdatePreferences(ctx, nil, watchlist.PreferencesDelta{})
		if !errors.Is(err, watchlist.ErrNoPreferencesSupplied) {
			t.Fatalf("err = %v, want ErrNoPreferencesSupplied", err)
		}
	})
	t.Run("valid delta with no ids is empty and non-nil", func(t *testing.T) {
		res, err := svc.BulkUpdatePreferences(ctx, nil, watchlist.PreferencesDelta{ReleaseTypes: watchlist.AxisDelta{Set: []string{"ep"}}})
		if err != nil {
			t.Fatalf("BulkUpdatePreferences: %v", err)
		}
		assertIDs(t, "applied", res.Applied, []int64{})
		assertIDs(t, "gone", res.Gone, []int64{})
	})
}

// A mute written by a concurrent transaction must survive a bulk release delta
// that was blocked on the row lock (RESEARCH Pattern 2).
func TestService_BulkUpdatePreferences_ConcurrentAxisWriteIsNotLost(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc := watchlist.NewService(sqlc.New(pool))
	ids := seedBulkEntries(t, ctx, svc, 1)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "UPDATE watchlist SET muted_event_types = ARRAY['deluxe_change']::text[] WHERE id = $1", ids[0]); err != nil {
		t.Fatalf("held-lock update: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
			ReleaseTypes: watchlist.AxisDelta{Set: []string{"ep"}, Unset: []string{"album", "single", "deluxe"}},
		})
		errCh <- err
	}()

	select {
	case <-errCh:
		t.Fatal("BulkUpdatePreferences finished without waiting for the held row lock")
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("BulkUpdatePreferences: %v", err)
	}

	p := readPrefs(t, ctx, pool, ids[0])
	assertStrings(t, "release", p.release, []string{"ep"})
	assertStrings(t, "mute", p.mute, []string{"deluxe_change"})
}

func TestService_BulkUpdatePreferences_ConcurrentDeleteIsGone(t *testing.T) {
	pool := testutil.NewIsolatedTestPool(t, "watchlist_bulk_test")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	svc := watchlist.NewService(sqlc.New(pool))
	ids := seedBulkEntries(t, ctx, svc, 2)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "DELETE FROM watchlist WHERE id = $1", ids[0]); err != nil {
		t.Fatalf("held-lock delete: %v", err)
	}

	type outcome struct {
		res watchlist.BulkUpdateResult
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := svc.BulkUpdatePreferences(ctx, ids, watchlist.PreferencesDelta{
			MutedEventTypes: watchlist.AxisDelta{Set: []string{"guest_feature"}},
		})
		done <- outcome{res, err}
	}()

	select {
	case <-done:
		t.Fatal("BulkUpdatePreferences finished without waiting for the held delete")
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit tx: %v", err)
	}
	out := <-done
	if out.err != nil {
		t.Fatalf("BulkUpdatePreferences: %v", out.err)
	}
	assertIDs(t, "applied", out.res.Applied, []int64{ids[1]})
	assertIDs(t, "gone", out.res.Gone, []int64{ids[0]})
}
