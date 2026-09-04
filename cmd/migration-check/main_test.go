package main

// Whitebox tests: package main so it can drive the unexported run function
// and the pure scan helpers directly (mirrors cmd/coverage-report/main_test.go).

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from current output")

func runScanCapture(t *testing.T, filesArg string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run([]string{"--mode", "scan", "--files", filesArg}, &buf)
	return buf.String(), err
}

func TestScan_PatternDetection(t *testing.T) {
	cases := []struct {
		name      string
		file      string
		wantClass string
	}{
		{"drop column", "testdata/drop_column.sql", "backward-incompatible"},
		{"drop table", "testdata/drop_table.sql", "backward-incompatible"},
		{"rename table", "testdata/rename_table.sql", "backward-incompatible"},
		{"rename column", "testdata/rename_column.sql", "backward-incompatible"},
		{"alter type", "testdata/alter_type.sql", "backward-incompatible"},
		{"set not null", "testdata/set_not_null.sql", "backward-incompatible"},
		{"add check", "testdata/add_check.sql", "backward-incompatible"},
		{"add notnull no default", "testdata/add_notnull_no_default.sql", "unsafe-forward"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runScanCapture(t, tc.file)
			if err == nil {
				t.Fatalf("run() error = nil, want non-nil for %s:\n%s", tc.file, out)
			}
			got := strings.Count(out, "["+tc.wantClass+"]")
			if got != 1 {
				t.Fatalf("finding count for class %s = %d, want 1\n%s", tc.wantClass, got, out)
			}
			total := strings.Count(out, "[backward-incompatible]") + strings.Count(out, "[unsafe-forward]")
			if total != 1 {
				t.Fatalf("total finding count = %d, want exactly 1\n%s", total, out)
			}
			if !strings.Contains(out, tc.file+":") {
				t.Fatalf("output does not cite %s with a line number:\n%s", tc.file, out)
			}
		})
	}
}

func TestScan_UnsafeForwardMessageDiffersFromBackwardIncompatible(t *testing.T) {
	backOut, err := runScanCapture(t, "testdata/drop_column.sql")
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}
	forwardOut, err := runScanCapture(t, "testdata/add_notnull_no_default.sql")
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}
	if backOut == forwardOut {
		t.Fatal("backward-incompatible and unsafe-forward reports must not be identical")
	}
	if !strings.Contains(forwardOut, unsafeForwardMsg) {
		t.Fatalf("unsafe-forward output missing its remediation paragraph:\n%s", forwardOut)
	}
	if strings.Contains(forwardOut, backwardIncompatibleMsg) {
		t.Fatalf("unsafe-forward output must not contain the backward-incompatible paragraph:\n%s", forwardOut)
	}
	if !strings.Contains(backOut, backwardIncompatibleMsg) {
		t.Fatalf("backward-incompatible output missing its remediation paragraph:\n%s", backOut)
	}
}

func TestScan_AddNotNullWithDefaultProducesNoFindings(t *testing.T) {
	out, err := runScanCapture(t, "testdata/add_notnull_with_default.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil:\n%s", err, out)
	}
}

func TestScan_SafeAdditiveProducesNoFindings(t *testing.T) {
	out, err := runScanCapture(t, "testdata/safe_additive.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil:\n%s", err, out)
	}
	if !strings.Contains(out, "testdata/safe_additive.sql") {
		t.Fatalf("scanned-file list missing safe_additive.sql:\n%s", out)
	}
}

func TestScan_CommentedOutDestructiveSQLIsIgnored(t *testing.T) {
	out, err := runScanCapture(t, "testdata/commented_out.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil:\n%s", err, out)
	}
}

func TestScan_RealRepoMigrationsAreClean(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "internal", "db", "migrations", "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 7 {
		t.Fatalf("glob matched %d files, want at least 7 (an empty glob must not pass vacuously)", len(files))
	}
	out, err := runScanCapture(t, strings.Join(files, "\n"))
	if err != nil {
		t.Fatalf("run() error = %v, want nil against the repo's real up-migrations:\n%s", err, out)
	}
	for _, f := range files {
		if !strings.Contains(out, filepath.ToSlash(f)) && !strings.Contains(out, f) {
			t.Fatalf("scanned-file list missing %s:\n%s", f, out)
		}
	}
}

func TestScan_DownFilesProduceNoFindings(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "internal", "db", "migrations", "*.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 7 {
		t.Fatalf("glob matched %d files, want at least 7 (an empty glob must not pass vacuously)", len(files))
	}
	out, err := runScanCapture(t, strings.Join(files, "\n"))
	if err != nil {
		t.Fatalf("run() error = %v, want nil for down-only files:\n%s", err, out)
	}
	for _, f := range files {
		if !strings.Contains(out, f) {
			t.Fatalf("output does not mention skipped down file %s:\n%s", f, out)
		}
	}
}

func TestScan_OutputIsDeterministic(t *testing.T) {
	shuffledA := "testdata/drop_column.sql\ntestdata/add_notnull_no_default.sql\ntestdata/safe_additive.sql"
	shuffledB := "testdata/safe_additive.sql\ntestdata/add_notnull_no_default.sql\ntestdata/drop_column.sql"

	outA, errA := runScanCapture(t, shuffledA)
	outB, errB := runScanCapture(t, shuffledB)
	if (errA == nil) != (errB == nil) {
		t.Fatalf("errors differ across input orderings: %v vs %v", errA, errB)
	}
	if outA != outB {
		t.Fatalf("stdout differs across input orderings:\n--- A ---\n%s\n--- B ---\n%s", outA, outB)
	}

	idxAdd := strings.Index(outA, "testdata/add_notnull_no_default.sql:")
	idxDrop := strings.Index(outA, "testdata/drop_column.sql:")
	if idxAdd < 0 || idxDrop < 0 {
		t.Fatalf("expected findings for both files:\n%s", outA)
	}
	if idxAdd > idxDrop {
		t.Fatalf("findings not sorted ascending by path (add_notnull_no_default.sql should sort before drop_column.sql):\n%s", outA)
	}

	// Re-running the exact same input must be byte-identical.
	outA2, errA2 := runScanCapture(t, shuffledA)
	if (errA == nil) != (errA2 == nil) || outA != outA2 {
		t.Fatalf("repeated run() over identical input is not byte-identical:\n--- 1 ---\n%s\n--- 2 ---\n%s", outA, outA2)
	}
}

func TestScan_GoldenFailureMessage(t *testing.T) {
	out, err := runScanCapture(t, "testdata/mixed_findings.sql")
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}

	goldenPath := filepath.Join("testdata", "mixed_findings.golden.txt")
	if *update {
		if werr := os.WriteFile(goldenPath, []byte(out), 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if out != string(want) {
		t.Fatalf("golden mismatch\n--- got ---\n%s\n--- want ---\n%s", out, want)
	}
	if !strings.Contains(out, "internal/db/migrations/README.md") {
		t.Fatal("golden output missing the internal/db/migrations/README.md pointer")
	}
	if !strings.Contains(out, backwardIncompatibleMsg) || !strings.Contains(out, unsafeForwardMsg) {
		t.Fatal("golden output missing one of the two class-specific remediation paragraphs")
	}
}

func TestRun_UnrecognisedMode(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"--mode", "bogus"}, &buf)
	if err == nil {
		t.Fatal("run() error = nil, want non-nil for an unrecognised --mode")
	}
	if !strings.Contains(err.Error(), `unrecognised --mode "bogus"`) {
		t.Fatalf("error = %q, want it to name the bad mode", err.Error())
	}
}

func TestScan_EmptyFileListPrintsScannedNoneAndExitsZero(t *testing.T) {
	out, err := runScanCapture(t, "")
	if err != nil {
		t.Fatalf("run() error = %v, want nil for an empty file list:\n%s", err, out)
	}
	if !strings.Contains(out, "(none)") {
		t.Fatalf("empty scan output does not report a visibly empty scanned-file set:\n%s", out)
	}
}

func TestScan_MissingFileIsAHardError(t *testing.T) {
	_, err := runScanCapture(t, "testdata/does-not-exist.up.sql")
	if err == nil {
		t.Fatal("run() error = nil, want non-nil for a missing migration file")
	}
}

func TestStripComments(t *testing.T) {
	t.Run("line comment stripped, line count preserved", func(t *testing.T) {
		in := "SELECT 1; -- drop stuff\nSELECT 2;"
		got := stripComments(in)
		if strings.Contains(got, "drop stuff") {
			t.Fatalf("stripComments left comment text in place: %q", got)
		}
		if strings.Count(got, "\n") != strings.Count(in, "\n") {
			t.Fatalf("stripComments changed the line count: got %q from %q", got, in)
		}
		if !strings.HasPrefix(got, "SELECT 1;") || !strings.HasSuffix(got, "SELECT 2;") {
			t.Fatalf("stripComments corrupted the surrounding statements: %q", got)
		}
	})

	t.Run("block comment stripped, line count preserved", func(t *testing.T) {
		in := "A/*\nB*/C"
		got := stripComments(in)
		if strings.Contains(got, "B") {
			t.Fatalf("stripComments left block-comment text in place: %q", got)
		}
		if strings.Count(got, "\n") != strings.Count(in, "\n") {
			t.Fatalf("stripComments changed the line count: got %q from %q", got, in)
		}
	})

	t.Run("string literal untouched", func(t *testing.T) {
		in := "DEFAULT '--not a comment'"
		if got := stripComments(in); got != in {
			t.Fatalf("stripComments(%q) = %q, want unchanged", in, got)
		}
	})
}

func TestSplitStatements_RespectsStringLiteralSemicolons(t *testing.T) {
	stmts := splitStatements("ALTER TABLE t ADD COLUMN n text DEFAULT 'a;b';")
	if len(stmts) != 1 {
		t.Fatalf("splitStatements produced %d statements, want 1 (semicolon inside a string literal must not split)", len(stmts))
	}
}

func TestSplitTopLevelCommas_IgnoresCommasInsideParens(t *testing.T) {
	got := splitTopLevelCommas("ADD COLUMN n numeric(10,2) NOT NULL DEFAULT 0, ADD COLUMN m text")
	if len(got) != 2 {
		t.Fatalf("splitTopLevelCommas produced %d clauses, want 2: %#v", len(got), got)
	}
}
