package sqlscan

// Whitebox tests -- the single package sqlscan test file the design allows,
// so the lexer/extractor internals can be exercised without exporting them.

import (
	"strings"
	"testing"
)

func TestDollarTagAt(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
		ok   bool
	}{
		{"bare $$", "$$body$$", "$$", true},
		{"named tag", "$body$ x $body$", "$body$", true},
		{"no closing dollar", "$ x", "", false},
		{"space in tag", "$a b$", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := dollarTagAt(tc.src, 0)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("dollarTagAt(%q) = (%q, %v), want (%q, %v)", tc.src, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestCopyDollarQuoted_NoClosingTagCopiesRemainder(t *testing.T) {
	var b strings.Builder
	src := "$$ unterminated body ; more"
	end := copyDollarQuoted(&b, src, 0, "$$")
	if end != len(src) {
		t.Fatalf("copyDollarQuoted end = %d, want %d (whole remainder consumed)", end, len(src))
	}
	if b.String() != src {
		t.Fatalf("copyDollarQuoted copied %q, want the verbatim remainder %q", b.String(), src)
	}
}

// TestFindFromJoinTables_AdjacentFromJoinBothFound pins the fix for a
// regex-consumption bug: a single combined "FROM/JOIN + table + optional
// trailing alias" regex let the alias group swallow the following clause's
// own FROM/JOIN keyword, making FindAllStringSubmatch skip the real second
// table. findFromJoinTables must find both.
func TestFindFromJoinTables_AdjacentFromJoinBothFound(t *testing.T) {
	got := findFromJoinTables("SELECT id\nFROM widgets_a\nJOIN widgets_b ON widgets_a.widget_id = widgets_b.id\nWHERE status = 'ok'")
	if len(got) != 2 {
		t.Fatalf("findFromJoinTables() = %#v, want exactly 2 entries (FROM widgets_a, JOIN widgets_b)", got)
	}
	if got[0].table != "widgets_a" || got[0].alias != "" {
		t.Fatalf("findFromJoinTables()[0] = %+v, want table widgets_a with no alias", got[0])
	}
	if got[1].table != "widgets_b" || got[1].alias != "" {
		t.Fatalf("findFromJoinTables()[1] = %+v, want table widgets_b with no alias", got[1])
	}
}
