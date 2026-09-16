package sqlscan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/sqlscan"
)

// prevReleaseSchemaCols is a hand-constructed "all columns of table X" set
// for the fixtures in testdata/prevrelease -- avoids routing every case
// through SchemaColumns just to get a known column list.
var prevReleaseSchemaCols = map[string][]string{
	"artists": {
		"id", "mbid", "deezer_id", "name", "disambiguation", "image_url",
		"created_at", "updated_at", "art_match_attempted_at",
	},
	"events": {
		"id", "artist_id", "source", "event_type", "external_id", "release_group_mbid",
		"title", "artist_name", "release_date", "cover_art_url", "track_count",
		"notified_at", "created_at", "previous_track_count", "release_type", "watched_artist_name",
	},
	"watchlist": {
		"id", "artist_id", "release_types", "muted_event_types", "created_at", "updated_at",
	},
}

func readPrevReleaseFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "prevrelease", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func refsFor(t *testing.T, file string) sqlscan.RefSet {
	t.Helper()
	return sqlscan.QueryColumnRefs("queries/"+file, readPrevReleaseFixture(t, file), prevReleaseSchemaCols)
}

func mustHigh(t *testing.T, r sqlscan.RefSet, table, col string) {
	t.Helper()
	if _, ok := r.Lookup(table, col); !ok {
		t.Fatalf("expected (%s, %s) in the high-confidence set", table, col)
	}
}

func mustNotHigh(t *testing.T, r sqlscan.RefSet, table, col string) {
	t.Helper()
	if _, ok := r.Lookup(table, col); ok {
		t.Fatalf("did NOT expect (%s, %s) in the high-confidence set", table, col)
	}
}

func TestQueryColumnRefs(t *testing.T) {
	t.Run("InsertEvent's 14-column list is entirely high-confidence", func(t *testing.T) {
		r := refsFor(t, "events.sql")
		cols := []string{
			"artist_id", "source", "event_type", "external_id", "release_group_mbid",
			"title", "artist_name", "release_date", "cover_art_url", "track_count", "notified_at",
			"previous_track_count", "release_type", "watched_artist_name",
		}
		if len(cols) != 14 {
			t.Fatalf("test fixture error: %d columns, want 14", len(cols))
		}
		for _, c := range cols {
			mustHigh(t, r, "events", c)
		}
	})

	t.Run("ON CONFLICT target list is high-confidence", func(t *testing.T) {
		r := refsFor(t, "events.sql")
		for _, c := range []string{"event_type", "source", "external_id"} {
			mustHigh(t, r, "events", c)
		}
	})

	t.Run("UpsertArtist EXCLUDED and table-qualified columns are high-confidence", func(t *testing.T) {
		r := refsFor(t, "artists.sql")
		for _, c := range []string{"mbid", "name", "deezer_id"} {
			mustHigh(t, r, "artists", c)
		}
	})

	t.Run("alias-qualified star expands to every schema column, alias.col resolves via the alias map", func(t *testing.T) {
		r := refsFor(t, "artists.sql")
		mustHigh(t, r, "artists", "image_url")
		mustHigh(t, r, "watchlist", "artist_id")
		for _, c := range prevReleaseSchemaCols["artists"] {
			mustHigh(t, r, "artists", c)
		}
	})

	t.Run("single-table bare SELECT * expands to every schema column", func(t *testing.T) {
		r := refsFor(t, "events.sql")
		for _, c := range prevReleaseSchemaCols["events"] {
			mustHigh(t, r, "events", c)
		}
	})

	t.Run("CTE name is excluded from the real-table set; its inner bare column resolves to the real table", func(t *testing.T) {
		r := refsFor(t, "events.sql")
		mustHigh(t, r, "events", "track_count")
		mustNotHigh(t, r, "existing", "track_count")
	})

	t.Run("subquery is flattened: bare WHERE column inside EXISTS(...) resolves to the single real table", func(t *testing.T) {
		r := refsFor(t, "events.sql")
		mustHigh(t, r, "events", "artist_id")
	})

	t.Run("sqlc.arg/narg and @param names are collected as parameters, never as columns", func(t *testing.T) {
		r := refsFor(t, "params.sql")
		for _, p := range []string{"artist_id", "cutoff", "page_size", "set_release_types", "release_types", "id"} {
			if !r.Params[p] {
				t.Fatalf("expected parameter %q collected into Params", p)
			}
		}
		if r.HasLow("events", "page_size") {
			t.Fatal("page_size must never be asserted as a column, not even low-confidence")
		}
		mustNotHigh(t, r, "events", "page_size")
	})

	t.Run("bare unqualified column inside a two-table join is low-confidence, not high-confidence", func(t *testing.T) {
		r := refsFor(t, "low_confidence_join.sql")
		if !r.HasLow("widgets_a", "status") && !r.HasLow("widgets_b", "status") {
			t.Fatal("expected \"status\" in the low-confidence set for at least one joined table")
		}
		mustNotHigh(t, r, "widgets_a", "status")
		mustNotHigh(t, r, "widgets_b", "status")
	})

	t.Run("schema-qualified table name resolves to the bare table", func(t *testing.T) {
		r := refsFor(t, "schema_qualified.sql")
		mustHigh(t, r, "events", "title")
		mustHigh(t, r, "events", "notified_at")
		// A schema-qualified lookup argument must also resolve.
		if _, ok := r.Lookup("public.events", "title"); !ok {
			t.Fatal("expected Lookup(public.events, title) to resolve via StripSchemaQualifier")
		}
	})
}

func TestQueryColumnRefs_IdentifierCaseFolding(t *testing.T) {
	r := refsFor(t, "case_folding.sql")

	for _, lookup := range []string{"image_url", "IMAGE_URL", "Image_Url"} {
		if _, ok := r.Lookup("artists", lookup); !ok {
			t.Fatalf("expected an unquoted reference to fold: lookup %q against a stored image_url reference failed", lookup)
		}
	}

	if _, ok := r.Lookup("artists", `"Mixed"`); !ok {
		t.Fatal(`expected the byte-exact quoted lookup "Mixed" to match`)
	}
	if _, ok := r.Lookup("artists", "Mixed"); ok {
		t.Fatal(`unquoted lookup "Mixed" must NOT match the quoted reference "Mixed"`)
	}
	if _, ok := r.Lookup("artists", "mixed"); ok {
		t.Fatal(`lower-cased unquoted lookup "mixed" must NOT match the quoted reference "Mixed"`)
	}
}

func TestRefSet_MergeAndLookups(t *testing.T) {
	a := sqlscan.QueryColumnRefs("queries/events.sql", readPrevReleaseFixture(t, "events.sql"), prevReleaseSchemaCols)
	b := sqlscan.QueryColumnRefs("queries/low_confidence_join.sql", readPrevReleaseFixture(t, "low_confidence_join.sql"), prevReleaseSchemaCols)
	p := sqlscan.QueryColumnRefs("queries/params.sql", readPrevReleaseFixture(t, "params.sql"), prevReleaseSchemaCols)

	// Merge against a zero-value receiver.
	var merged sqlscan.RefSet
	merged.Merge(a)
	merged.Merge(b)
	merged.Merge(p)

	if _, ok := merged.Lookup("events", "notified_at"); !ok {
		t.Fatal("Merge lost a's high-confidence (events, notified_at)")
	}
	if !merged.HasLow("widgets_a", "status") && !merged.HasLow("widgets_b", "status") {
		t.Fatal("Merge lost b's low-confidence status reference")
	}
	if !merged.Params["cutoff"] {
		t.Fatal("Merge lost p's cutoff parameter")
	}

	// LookupAnyColumn: hit on any column of a referenced table, miss on an
	// unreferenced table.
	if _, ok := merged.LookupAnyColumn("events"); !ok {
		t.Fatal("LookupAnyColumn(events) = false, want true")
	}
	if _, ok := merged.LookupAnyColumn("nonexistent_table"); ok {
		t.Fatal("LookupAnyColumn(nonexistent_table) = true, want false")
	}

	// HasLow never promotes into a Lookup hit.
	if _, ok := merged.Lookup("widgets_a", "status"); ok {
		t.Fatal("a low-confidence reference must never satisfy Lookup")
	}
}
