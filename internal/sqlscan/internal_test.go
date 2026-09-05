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
