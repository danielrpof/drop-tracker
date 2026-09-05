package main

// Whitebox tests: package main so it can drive the unexported run function
// and the pure scan helpers directly (mirrors cmd/coverage-report/main_test.go).

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
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

// runCapture drives run() with an arbitrary argv, for scan-mode invocations
// that also need --prev-tag (Task 3).
func runCapture(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := run(args, &buf)
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
	// mixed_findings.sql carries three statements, one per finding class:
	// DROP COLUMN release_type (backward-incompatible, no cross-ref hit --
	// this stub's events.sql content never mentions release_type), ADD
	// COLUMN foo NOT NULL (unsafe-forward), and DROP COLUMN notified_at
	// (classCrossRef -- notified_at IS referenced below).
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": "-- name: ListSomething :many\nSELECT notified_at FROM events;\n",
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/mixed_findings.sql", "--prev-tag", "v1.7.0")
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
	if strings.Count(out, "internal/db/migrations/README.md") < 3 {
		t.Fatalf("golden output must cite internal/db/migrations/README.md once per finding class (3), got %d\n%s", strings.Count(out, "internal/db/migrations/README.md"), out)
	}
	if !strings.Contains(out, backwardIncompatibleMsg) || !strings.Contains(out, unsafeForwardMsg) {
		t.Fatal("golden output missing one of the backward-incompatible/unsafe-forward remediation paragraphs")
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatal("golden output missing the classCrossRef finding")
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

// ---- annotation parsing / suppression (Task 3, D-07/S4) ----

func TestScan_AnnotatedDropIsSuppressedAndEchoesReason(t *testing.T) {
	out, err := runScanCapture(t, "testdata/annotated_drop.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil (annotation suppresses the finding):\n%s", err, out)
	}
	if !strings.Contains(out, "v1.7.0") {
		t.Fatalf("output missing the annotation's expand-shipped-in tag value:\n%s", out)
	}
	if !strings.Contains(out, "events.release_type superseded by watched_artist_name") {
		t.Fatalf("output missing the annotation's reason text:\n%s", out)
	}
	if strings.Contains(out, "[backward-incompatible]") {
		t.Fatalf("suppressed finding must not still appear in output:\n%s", out)
	}
}

func TestScan_AnnotatedNotNullIsSuppressed(t *testing.T) {
	out, err := runScanCapture(t, "testdata/annotated_notnull.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil (annotation covers unsafe-forward too, D-08 revision):\n%s", err, out)
	}
	if strings.Contains(out, "[unsafe-forward]") {
		t.Fatalf("suppressed finding must not still appear in output:\n%s", out)
	}
}

func TestScan_AnnotatedSafeFileIsNotAnError(t *testing.T) {
	out, err := runScanCapture(t, "testdata/annotated_safe.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil (an annotation on a clean file is not itself an error):\n%s", err, out)
	}
	if !strings.Contains(out, "testdata/annotated_safe.sql") {
		t.Fatalf("scanned-file list missing annotated_safe.sql:\n%s", out)
	}
}

func TestScan_AnnotationMissingReasonIsHardErrorAndDoesNotSuppress(t *testing.T) {
	out, err := runScanCapture(t, "testdata/annotated_missing_reason.sql")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil for a half-written annotation:\n%s", out)
	}
	if !strings.Contains(err.Error(), `"reason"`) {
		t.Fatalf("error does not name the missing key \"reason\": %v", err)
	}
	if !strings.Contains(out, "[backward-incompatible]") {
		t.Fatalf("underlying DROP COLUMN finding must not be silently suppressed:\n%s", out)
	}
}

func TestScan_AnnotationMissingTagIsHardErrorAndDoesNotSuppress(t *testing.T) {
	out, err := runScanCapture(t, "testdata/annotated_missing_tag.sql")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil for a half-written annotation:\n%s", out)
	}
	if !strings.Contains(err.Error(), `"expand-shipped-in"`) {
		t.Fatalf("error does not name the missing key \"expand-shipped-in\": %v", err)
	}
	if !strings.Contains(out, "[backward-incompatible]") {
		t.Fatalf("underlying DROP COLUMN finding must not be silently suppressed:\n%s", out)
	}
}

func TestScan_AnnotatedOutputIsDeterministic(t *testing.T) {
	out1, err1 := runScanCapture(t, "testdata/annotated_drop.sql")
	out2, err2 := runScanCapture(t, "testdata/annotated_drop.sql")
	if (err1 == nil) != (err2 == nil) || out1 != out2 {
		t.Fatalf("repeated run() over an annotated file is not byte-identical:\n--- 1 ---\n%s\n--- 2 ---\n%s", out1, out2)
	}
}

func TestAnnotation_TagShapeIsValidated(t *testing.T) {
	cases := []struct {
		name string
		tail string
	}{
		{"shell metacharacter", "expand-shipped-in=v1.7.0;rm-rf reason=bad"},
		{"path separator", "expand-shipped-in=v1.7.0/../etc reason=bad"},
		{"shell substitution", "expand-shipped-in=$(whoami) reason=bad"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := "-- migration-check:allow-destructive " + tc.tail + "\nALTER TABLE events DROP COLUMN release_type;\n"
			ann, ok, err := parseAnnotation(raw)
			if err == nil {
				t.Fatalf("parseAnnotation(%q) error = nil, want non-nil", tc.tail)
			}
			if !ok {
				t.Fatalf("parseAnnotation(%q) ok = false, want true (prefix was present)", tc.tail)
			}
			if ann.tag != "" {
				t.Fatalf("parseAnnotation(%q) stored tag %q on a rejected value, want zero value", tc.tail, ann.tag)
			}
		})
	}
}

func TestParseAnnotation_NoAnnotationIsNotAnError(t *testing.T) {
	ann, ok, err := parseAnnotation("ALTER TABLE events ADD COLUMN foo text;\n")
	if err != nil || ok || ann != (annotation{}) {
		t.Fatalf("parseAnnotation(no annotation) = %+v, %v, %v; want zero value, false, nil", ann, ok, err)
	}
}

// ---- changed-files mode: diff-base selection (Task 1, D-16/S2) ----

// withStubCommitExists swaps the commitExists seam for a test, restoring it
// in t.Cleanup -- mirrors coverage-report's withFixedClock pattern.
func withStubCommitExists(t *testing.T, fn func(ref string) bool) {
	t.Helper()
	prev := commitExists
	commitExists = fn
	t.Cleanup(func() { commitExists = prev })
}

// withStubGitDiffNames swaps the gitDiffNames seam for a test.
func withStubGitDiffNames(t *testing.T, fn func(filter, rangeArg string) ([]string, error)) {
	t.Helper()
	prev := gitDiffNames
	gitDiffNames = fn
	t.Cleanup(func() { gitDiffNames = prev })
}

func TestDiffRange(t *testing.T) {
	const reachableSHA = "abc1234abc1234abc1234abc1234abc1234abc1"
	const targetSHA = "def5678def5678def5678def5678def5678def5"

	t.Run("pull_request uses three-dot merge-base against base-ref", func(t *testing.T) {
		withStubCommitExists(t, func(string) bool { return true })
		got, err := diffRange("pull_request", "", "", "main")
		if err != nil {
			t.Fatalf("diffRange() error = %v, want nil", err)
		}
		if want := "origin/main...HEAD"; got != want {
			t.Fatalf("diffRange() = %q, want %q", got, want)
		}
	})

	t.Run("push with a reachable before uses the two-dot literal range", func(t *testing.T) {
		withStubCommitExists(t, func(ref string) bool { return ref == reachableSHA })
		got, err := diffRange("push", reachableSHA, targetSHA, "")
		if err != nil {
			t.Fatalf("diffRange() error = %v, want nil", err)
		}
		if want := reachableSHA + ".." + targetSHA; got != want {
			t.Fatalf("diffRange() = %q, want %q", got, want)
		}
	})

	t.Run("push with all-zeroes before falls back to merge-base against origin/main", func(t *testing.T) {
		withStubCommitExists(t, func(string) bool {
			t.Fatal("commitExists must not be called for the all-zeroes before")
			return false
		})
		got, err := diffRange("push", allZeroSHA, targetSHA, "")
		if err != nil {
			t.Fatalf("diffRange() error = %v, want nil", err)
		}
		if got != mergeBaseFallbackRange {
			t.Fatalf("diffRange() = %q, want the merge-base fallback %q", got, mergeBaseFallbackRange)
		}
	})

	t.Run("push with an unreachable before falls back to the same merge-base range", func(t *testing.T) {
		withStubCommitExists(t, func(string) bool { return false })
		got, err := diffRange("push", reachableSHA, targetSHA, "")
		if err != nil {
			t.Fatalf("diffRange() error = %v, want nil", err)
		}
		if got != mergeBaseFallbackRange {
			t.Fatalf("diffRange() = %q, want the merge-base fallback %q", got, mergeBaseFallbackRange)
		}
	})

	t.Run("unknown event name is a hard error, never a silently-empty range", func(t *testing.T) {
		got, err := diffRange("workflow_dispatch", "", "", "")
		if err == nil {
			t.Fatalf("diffRange() error = nil, want non-nil for an unrecognised event")
		}
		if got != "" {
			t.Fatalf("diffRange() = %q, want empty range alongside the error", got)
		}
	})

	rejectCases := []struct {
		name                        string
		event, before, sha, baseRef string
	}{
		{"base-ref shell metacharacter", "pull_request", "", "", "main;rm -rf /"},
		{"base-ref path traversal", "pull_request", "", "", "../../etc"},
		{"before shell metacharacter", "push", "abc; echo pwned", targetSHA, ""},
		{"sha with a newline", "push", reachableSHA, "abc1234\nrm -rf /", ""},
	}
	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			withStubCommitExists(t, func(string) bool { return true })
			got, err := diffRange(tc.event, tc.before, tc.sha, tc.baseRef)
			if err == nil {
				t.Fatalf("diffRange() error = nil, want non-nil")
			}
			if got != "" {
				t.Fatalf("diffRange() = %q, want empty range alongside the error", got)
			}
			var rejected string
			switch {
			case tc.baseRef != "" && !validBranchRef(tc.baseRef):
				rejected = tc.baseRef
			case tc.event == "push" && !validCommitish(tc.before):
				rejected = tc.before
			default:
				rejected = tc.sha
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%q", rejected)) {
				t.Fatalf("diffRange() error = %q, want it to name the rejected value %q", err.Error(), rejected)
			}
		})
	}
}

func TestChangedFiles_AddedAndModified(t *testing.T) {
	withStubGitDiffNames(t, func(filter, rangeArg string) ([]string, error) {
		switch filter {
		case "A":
			return []string{"internal/db/migrations/000008_x.up.sql", "web/app/root.tsx"}, nil
		case "M":
			return nil, nil
		}
		t.Fatalf("unexpected diff-filter %q", filter)
		return nil, nil
	})
	var buf bytes.Buffer
	err := run([]string{"--mode", "changed-files", "--event-name", "pull_request", "--base-ref", "main"}, &buf)
	if err != nil {
		t.Fatalf("run() error = %v, want nil:\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "migrations_changed=true") {
		t.Fatalf("output missing migrations_changed=true:\n%s", out)
	}
	if !strings.Contains(out, "migration_files=internal/db/migrations/000008_x.up.sql") {
		t.Fatalf("output missing the .up.sql migration_files entry:\n%s", out)
	}
	if strings.Contains(out, "web/app/root.tsx") {
		t.Fatalf("output leaked a non-migration path into migration_files:\n%s", out)
	}
}

func TestChangedFiles_NothingUnderGlobIsFalseAndEmpty(t *testing.T) {
	withStubGitDiffNames(t, func(filter, rangeArg string) ([]string, error) {
		return nil, nil
	})
	var buf bytes.Buffer
	err := run([]string{"--mode", "changed-files", "--event-name", "pull_request", "--base-ref", "main"}, &buf)
	if err != nil {
		t.Fatalf("run() error = %v, want nil:\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, "migrations_changed=false") {
		t.Fatalf("output missing migrations_changed=false:\n%s", out)
	}
	if !strings.Contains(out, "migration_files=\n") {
		t.Fatalf("output missing an empty migration_files line:\n%s", out)
	}
}

func TestChangedFiles_WritesGithubOutputFile(t *testing.T) {
	withStubGitDiffNames(t, func(filter, rangeArg string) ([]string, error) {
		if filter == "A" {
			return []string{"internal/db/migrations/000009_y.up.sql"}, nil
		}
		return nil, nil
	})
	outPath := filepath.Join(t.TempDir(), "github_output.txt")
	t.Setenv("GITHUB_OUTPUT", outPath)

	var buf bytes.Buffer
	if err := run([]string{"--mode", "changed-files", "--event-name", "pull_request", "--base-ref", "main"}, &buf); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read GITHUB_OUTPUT file: %v", err)
	}
	if !strings.Contains(string(got), "migrations_changed=true") || !strings.Contains(string(got), "migration_files=internal/db/migrations/000009_y.up.sql") {
		t.Fatalf("GITHUB_OUTPUT file missing expected key=value lines:\n%s", got)
	}
}

func TestModifiedReleasedMigration(t *testing.T) {
	withStubGitDiffNames(t, func(filter, rangeArg string) ([]string, error) {
		switch filter {
		case "A":
			return nil, nil
		case "M":
			return []string{"internal/db/migrations/000006_events_watched_artist_name.up.sql"}, nil
		}
		t.Fatalf("unexpected diff-filter %q", filter)
		return nil, nil
	})
	var buf bytes.Buffer
	err := run([]string{"--mode", "changed-files", "--event-name", "pull_request", "--base-ref", "main"}, &buf)
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil for a modified already-released migration:\n%s", buf.String())
	}
	if !strings.Contains(err.Error(), "000006_events_watched_artist_name.up.sql") {
		t.Fatalf("error = %q, want it to name the modified file", err.Error())
	}
	if !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("error = %q, want it to name the immutability rule", err.Error())
	}
}

func TestFilterMigrationUpFiles_KeepsOnlyTheGlob(t *testing.T) {
	got := filterMigrationUpFiles([]string{
		"internal/db/migrations/000010_z.up.sql",
		"internal/db/migrations/000010_z.down.sql",
		"web/app/root.tsx",
	})
	if len(got) != 1 || got[0] != "internal/db/migrations/000010_z.up.sql" {
		t.Fatalf("filterMigrationUpFiles() = %#v, want only the .up.sql migration path", got)
	}
}

func TestValidCommitishAndValidBranchRef(t *testing.T) {
	if !validCommitish("abc1234") {
		t.Fatal("validCommitish(7-char hex) = false, want true")
	}
	if validCommitish("abc123") {
		t.Fatal("validCommitish(6-char hex) = true, want false")
	}
	if validCommitish("abc123g") {
		t.Fatal("validCommitish(non-hex) = true, want false")
	}
	if !validBranchRef("main") || !validBranchRef("feature/foo-bar.1") {
		t.Fatal("validBranchRef rejected a well-formed branch ref")
	}
	if validBranchRef("../../etc") || validBranchRef("main;rm -rf /") || validBranchRef("") {
		t.Fatal("validBranchRef accepted a malformed or path-traversal-shaped ref")
	}
}

// ---- D-15 previous-release query cross-reference (Task 2) ----

// withStubGitShow swaps the gitShow seam for a test, restoring it in
// t.Cleanup -- mirrors coverage-report's withFixedClock pattern.
func withStubGitShow(t *testing.T, fn func(tag, path string) ([]byte, error)) {
	t.Helper()
	prev := gitShow
	gitShow = fn
	t.Cleanup(func() { gitShow = prev })
}

// stubQueriesGitShow stubs gitShow so buildPrevReleaseRefs's four
// prevReleaseQueryFiles reads resolve to fixture content (Task 3): byFile
// maps a path (e.g. "queries/events.sql") to its content; any
// prevReleaseQueryFiles entry not present in byFile gets trivial content
// (a bare `SELECT 1`, no table refs) rather than an error, so a test only
// needs to supply the one file its scenario actually cares about.
func stubQueriesGitShow(t *testing.T, byFile map[string]string) {
	t.Helper()
	withStubGitShow(t, func(tag, path string) ([]byte, error) {
		if content, ok := byFile[path]; ok {
			return []byte(content), nil
		}
		return []byte("-- name: Ping :one\nSELECT 1;\n"), nil
	})
}

func TestGitShow_RejectsPathOutsideAllowlist(t *testing.T) {
	cases := []string{"../../etc/passwd", ".github/workflows/full-pipeline.yml"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			called := false
			withStubGitShow(t, func(tag, p string) ([]byte, error) {
				called = true
				return nil, nil
			})
			_, err := readAtTag("v1.7.0", path)
			if err == nil {
				t.Fatalf("readAtTag(%q) error = nil, want non-nil", path)
			}
			if called {
				t.Fatalf("readAtTag(%q) invoked the gitShow stub, want it never invoked for a rejected path", path)
			}
		})
	}
}

func TestGitShow_RejectsMalformedTag(t *testing.T) {
	called := false
	withStubGitShow(t, func(tag, p string) ([]byte, error) {
		called = true
		return nil, nil
	})
	_, err := readAtTag("v1.7.0;rm -rf /", "queries/events.sql")
	if err == nil {
		t.Fatal("readAtTag(malformed tag) error = nil, want non-nil")
	}
	if called {
		t.Fatal("readAtTag(malformed tag) invoked the gitShow stub, want it never invoked")
	}
}

func readPrevReleaseFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "prevrelease", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// ---- D-15 cross-reference wired into the scan path (Task 3) ----

func TestPrevReleaseCrossRef_AnnotationCannotOverride(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "events.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/annotated_drop.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil (annotation cannot override a live reference):\n%s", out)
	}
	if !strings.Contains(out, "events.release_type superseded by watched_artist_name") {
		t.Fatalf("output missing the echoed annotation reason:\n%s", out)
	}
	if !strings.Contains(out, "v1.7.0") {
		t.Fatalf("output missing the echoed annotation tag:\n%s", out)
	}
	if !strings.Contains(out, "cannot override") {
		t.Fatalf("output missing the message stating the annotation cannot override a live reference:\n%s", out)
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output missing the classCrossRef finding class:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_RenameColumnIsRed(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/artists.sql": readPrevReleaseFixture(t, "artists.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/prevref_rename_column.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil (RENAME COLUMN artists.image_url is still referenced):\n%s", out)
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output missing the classCrossRef finding class:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_DropTableIsRed(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "events.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/drop_table.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil (DROP TABLE events is still queried):\n%s", out)
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output missing the classCrossRef finding class:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_RenameTableIsRed(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "events.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/rename_table.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil (RENAME TABLE events is still queried):\n%s", out)
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output missing the classCrossRef finding class:\n%s", out)
	}
}

// CR-01 regression: a schema-qualified table name (e.g. "public.events") must
// not bypass the D-15 cross-reference — the scan side's raw f.table and the
// query-reference side's schema-stripped keys must normalize the same way.
func TestPrevReleaseCrossRef_SchemaQualifiedDropTableIsRed(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "events.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/drop_table_schema_qualified.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatalf("run() error = nil, want non-nil (DROP TABLE public.events is still queried):\n%s", out)
	}
	if !strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output missing the classCrossRef finding class for a schema-qualified table:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_NoReferenceStillPlainFindingSuppressedByAnnotation(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "events.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/prevref_drop_column_no_ref_annotated.sql", "--prev-tag", "v1.7.0")
	if err != nil {
		t.Fatalf("run() error = %v, want nil (no cross-reference hit; the annotation still suppresses the plain finding):\n%s", err, out)
	}
	if strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output must not contain a classCrossRef finding for an unreferenced column:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_LowConfidenceIsNotRed(t *testing.T) {
	stubQueriesGitShow(t, map[string]string{
		"queries/events.sql": readPrevReleaseFixture(t, "low_confidence_join.sql"),
	})
	out, err := runCapture(t, "--mode", "scan", "--files", "testdata/prevref_low_confidence_annotated.sql", "--prev-tag", "v1.7.0")
	if err != nil {
		t.Fatalf("run() error = %v, want nil (low-confidence reference must not cross-reference red; annotation suppresses the plain finding):\n%s", err, out)
	}
	if strings.Contains(out, string(classCrossRef)) {
		t.Fatalf("output must not contain a classCrossRef finding for a low-confidence-only reference:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_NoPriorTagSkips(t *testing.T) {
	out, err := runScanCapture(t, "testdata/safe_additive.sql")
	if err != nil {
		t.Fatalf("run() error = %v, want nil for a true bootstrap (no --prev-tag):\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "skip") {
		t.Fatalf("output missing a notice naming the D-15 cross-reference skip:\n%s", out)
	}
}

func TestPrevReleaseCrossRef_GitShowFailureIsRed(t *testing.T) {
	withStubGitShow(t, func(tag, path string) ([]byte, error) {
		if path == "queries/events.sql" {
			return nil, errors.New("simulated git show failure")
		}
		return []byte("-- name: Ping :one\nSELECT 1;\n"), nil
	})
	_, err := runCapture(t, "--mode", "scan", "--files", "testdata/safe_additive.sql", "--prev-tag", "v1.7.0")
	if err == nil {
		t.Fatal("run() error = nil, want non-nil when a supplied --prev-tag cannot be read")
	}
	if !strings.Contains(err.Error(), "v1.7.0") {
		t.Fatalf("error = %q, want it to name the tag", err.Error())
	}
	if !strings.Contains(err.Error(), "queries/events.sql") {
		t.Fatalf("error = %q, want it to name the unreadable query file", err.Error())
	}
}
