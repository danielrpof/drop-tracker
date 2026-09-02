package main

// Whitebox tests: this file is package main so it can drive the unexported
// run function and the pure parse/render helpers directly (mirrors
// cmd/server/main_test.go).

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from current output")

// withFixedClock pins nowUTC for a test so golden output is stable.
func withFixedClock(t *testing.T, ts string) {
	t.Helper()
	prev := nowUTC
	nowUTC = func() string { return ts }
	t.Cleanup(func() { nowUTC = prev })
}

// runComparingGolden drives run() in comment mode with the given args plus a
// temp --out, then compares (or updates) the named golden file.
func runComparingGolden(t *testing.T, golden string, args []string) string {
	t.Helper()
	outPath := filepath.Join(t.TempDir(), "comment.md")
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	full := append([]string{}, args...)
	full = append(full, "--out", outPath)
	if err := run(full, os.Stdout); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read comment output: %v", err)
	}

	goldenPath := filepath.Join("testdata", golden)
	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", golden, err)
	}
	if string(got) != string(want) {
		t.Fatalf("comment body mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
	}

	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read step summary: %v", err)
	}
	if string(summary) != string(got) {
		t.Fatalf("step summary != comment body\n--- summary ---\n%s\n--- body ---\n%s", summary, got)
	}
	return string(got)
}

func TestRenderComment_Golden(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	runComparingGolden(t, "comment-normal.golden.md", []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--baseline-backend", "testdata/baseline-metrics-backend.json",
		"--baseline-frontend", "testdata/baseline-metrics-frontend.json",
		"--head-sha", "abc1234def5678abc1234def5678abc1234def56",
		"--upstream-red=false",
	})
}

func TestBackendTotalPct(t *testing.T) {
	f, err := os.Open("testdata/backend-profile.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	got, err := backendTotalPct(f)
	if err != nil {
		t.Fatalf("backendTotalPct error = %v", err)
	}
	// 33 covered statements / 38 total * 100.
	if round2(got) != 86.84 {
		t.Fatalf("round2(backendTotalPct) = %v, want 86.84", round2(got))
	}
}

func TestFrontendLinesPct(t *testing.T) {
	f, err := os.Open("testdata/coverage-summary.json")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	got, err := frontendLinesPct(f)
	if err != nil {
		t.Fatalf("frontendLinesPct error = %v", err)
	}
	// total.lines.pct, not total.statements.pct (70.91).
	if got != 72.3 {
		t.Fatalf("frontendLinesPct = %v, want 72.3", got)
	}
}

func TestParseBlockLine_LastColonSplit(t *testing.T) {
	file, numStmts, count, err := parseBlockLine(
		"github.com/danielrpof/drop-tracker/internal/x:special/y.go:2.2,4.4 2 1")
	if err != nil {
		t.Fatalf("parseBlockLine error = %v", err)
	}
	if file != "github.com/danielrpof/drop-tracker/internal/x:special/y.go" {
		t.Fatalf("file = %q, want the path including its embedded colon", file)
	}
	if numStmts != 2 || count != 1 {
		t.Fatalf("numStmts,count = %d,%d, want 2,1", numStmts, count)
	}
}

func TestRenderComment_GoldenHasFixedShape(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "comment-normal.golden.md"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.HasPrefix(s, "## Coverage\n") {
		t.Errorf("golden does not open with the ## Coverage heading")
	}
	backendIdx := strings.Index(s, "\n| Backend ")
	frontendIdx := strings.Index(s, "\n| Frontend ")
	if backendIdx < 0 || frontendIdx < 0 || backendIdx > frontendIdx {
		t.Errorf("rows not present in fixed Backend-then-Frontend order")
	}
	if !strings.Contains(s, "baseline: main@") {
		t.Errorf("golden footer has no baseline provenance marker")
	}
}
