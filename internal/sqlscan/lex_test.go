package sqlscan_test

import (
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/sqlscan"
)

func TestStripComments(t *testing.T) {
	t.Run("line comment stripped, line count preserved", func(t *testing.T) {
		in := "SELECT 1; -- drop stuff\nSELECT 2;"
		got := sqlscan.StripComments(in)
		if strings.Contains(got, "drop stuff") {
			t.Fatalf("StripComments left comment text in place: %q", got)
		}
		if strings.Count(got, "\n") != strings.Count(in, "\n") {
			t.Fatalf("StripComments changed the line count: got %q from %q", got, in)
		}
		if !strings.HasPrefix(got, "SELECT 1;") || !strings.HasSuffix(got, "SELECT 2;") {
			t.Fatalf("StripComments corrupted the surrounding statements: %q", got)
		}
	})

	t.Run("block comment stripped, line count preserved", func(t *testing.T) {
		in := "A/*\nB*/C"
		got := sqlscan.StripComments(in)
		if strings.Contains(got, "B") {
			t.Fatalf("StripComments left block-comment text in place: %q", got)
		}
		if strings.Count(got, "\n") != strings.Count(in, "\n") {
			t.Fatalf("StripComments changed the line count: got %q from %q", got, in)
		}
	})

	t.Run("string literal untouched", func(t *testing.T) {
		in := "DEFAULT '--not a comment'"
		if got := sqlscan.StripComments(in); got != in {
			t.Fatalf("StripComments(%q) = %q, want unchanged", in, got)
		}
	})

	t.Run("-- inside a dollar-quoted body is not stripped", func(t *testing.T) {
		in := "CREATE FUNCTION f() RETURNS int AS $body$ SELECT 1; -- keep me\n$body$ LANGUAGE sql;"
		got := sqlscan.StripComments(in)
		if !strings.Contains(got, "-- keep me") {
			t.Fatalf("StripComments stripped a comment sequence inside a dollar-quoted body: %q", got)
		}
	})
}

func TestSplitStatements_RespectsStringLiteralSemicolons(t *testing.T) {
	stmts := sqlscan.SplitStatements("ALTER TABLE t ADD COLUMN n text DEFAULT 'a;b';")
	if len(stmts) != 1 {
		t.Fatalf("SplitStatements produced %d statements, want 1 (semicolon inside a string literal must not split)", len(stmts))
	}
}

func TestSplitStatements_RespectsDollarQuotedSemicolons(t *testing.T) {
	stmts := sqlscan.SplitStatements("CREATE FUNCTION f() RETURNS int AS $$ BEGIN; SELECT 1; END; $$ LANGUAGE plpgsql;")
	if len(stmts) != 1 {
		t.Fatalf("SplitStatements produced %d statements, want 1 (semicolons inside $$...$$ must not split)", len(stmts))
	}
}

func TestSplitStatements_FirstLineIsOneBased(t *testing.T) {
	stmts := sqlscan.SplitStatements("\n\nDROP TABLE events;")
	if len(stmts) != 1 || stmts[0].Line != 3 {
		t.Fatalf("SplitStatements()[0].Line = %d, want 3 (1-based first non-blank line)", func() int {
			if len(stmts) == 0 {
				return -1
			}
			return stmts[0].Line
		}())
	}
}

func TestSplitTopLevelCommas_IgnoresCommasInsideParens(t *testing.T) {
	got := sqlscan.SplitTopLevelCommas("ADD COLUMN n numeric(10,2) NOT NULL DEFAULT 0, ADD COLUMN m text")
	if len(got) != 2 {
		t.Fatalf("SplitTopLevelCommas produced %d clauses, want 2: %#v", len(got), got)
	}
}
