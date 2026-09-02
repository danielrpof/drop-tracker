package main

// Whitebox tests: this file is package main so it can drive the unexported
// run function and the pure parse/render helpers directly (mirrors
// cmd/server/main_test.go).

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

// execComment drives run() in comment mode with the given args plus a temp
// --out, asserts the step summary mirrors the body, and returns the body.
func execComment(t *testing.T, args []string) string {
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
	summary, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("read step summary: %v", err)
	}
	if string(summary) != string(got) {
		t.Fatalf("step summary != comment body\n--- summary ---\n%s\n--- body ---\n%s", summary, got)
	}
	return string(got)
}

// runComparingGolden drives comment mode and compares (or updates) the named
// golden file.
func runComparingGolden(t *testing.T, golden string, args []string) string {
	t.Helper()
	got := execComment(t, args)

	goldenPath := filepath.Join("testdata", golden)
	if *update {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", golden, err)
	}
	if got != string(want) {
		t.Fatalf("comment body mismatch for %s\n--- got ---\n%s\n--- want ---\n%s", golden, got, want)
	}
	return got
}

// deltaCell returns the trimmed "Δ vs main" cell of the named row from a
// rendered comment body.
func deltaCell(t *testing.T, body, rowLabel string) string {
	t.Helper()
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(ln, "| "+rowLabel+" ") {
			cols := strings.Split(ln, "|")
			if len(cols) < 4 {
				t.Fatalf("row %q has too few columns: %q", rowLabel, ln)
			}
			return strings.TrimSpace(cols[3])
		}
	}
	t.Fatalf("row %q not found in body:\n%s", rowLabel, body)
	return ""
}

// statusCell returns the trimmed "Status" cell of the named row.
func statusCell(t *testing.T, body, rowLabel string) string {
	t.Helper()
	for _, ln := range strings.Split(body, "\n") {
		if strings.HasPrefix(ln, "| "+rowLabel+" ") {
			cols := strings.Split(ln, "|")
			if len(cols) < 6 {
				t.Fatalf("row %q has too few columns: %q", rowLabel, ln)
			}
			return strings.TrimSpace(cols[5])
		}
	}
	t.Fatalf("row %q not found in body:\n%s", rowLabel, body)
	return ""
}

// writeSidecar writes a baseline sidecar with the given pct into a temp dir and
// returns its path.
func writeSidecar(t *testing.T, pct string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "baseline.json")
	body := `{"pct": ` + pct + `, "sha": "abcdef1234567890abcdef1234567890abcdef12", "generated_at": "2026-08-30T00:00:00Z"}`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write sidecar: %v", err)
	}
	return p
}

const goldenHeadSHA = "abc1234def5678abc1234def5678abc1234def56"

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

func TestDelta(t *testing.T) {
	cases := []struct {
		name string
		in   float64
		want string
	}{
		{"positive gain", 0.53, "+0.53pp"},
		{"negative drop", -1.2, "-1.20pp"},
		{"exactly unchanged", 0, "±0.00pp"},
		{"rounds half up to two dp", 0.005, "+0.01pp"},
		{"tiny negative rounds to zero form", -0.001, "±0.00pp"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatDelta(tc.in); got != tc.want {
				t.Fatalf("formatDelta(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	t.Run("no baseline renders the em-dash, not a nonsense delta", func(t *testing.T) {
		row := coverageRow{label: "Backend", gate: backendGate, value: 86.84, available: true, baselineKnown: false}
		got := renderRow(row)
		if !strings.Contains(got, "| "+emDash+" |") {
			t.Fatalf("renderRow without baseline = %q, want an em-dash delta cell", got)
		}
		if strings.Contains(got, "pp") {
			t.Fatalf("renderRow without baseline = %q, want no pp delta", got)
		}
	})
}

func TestRenderComment_NoBaseline(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	body := runComparingGolden(t, "comment-no-baseline.golden.md", []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
	})
	if strings.Contains(body, "baseline: main@") {
		t.Fatalf("no-baseline body carries a provenance line:\n%s", body)
	}
	if deltaCell(t, body, "Backend") != emDash || deltaCell(t, body, "Frontend") != emDash {
		t.Fatalf("no-baseline body has non-em-dash delta cells:\n%s", body)
	}
}

func TestRenderComment_Unchanged(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	baselineBackend := writeSidecar(t, "80.00")
	baselineFrontend := writeSidecar(t, "70.00")
	body := runComparingGolden(t, "comment-unchanged.golden.md", []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile-boundary.txt",
		"--frontend-summary", "testdata/coverage-summary-boundary.json",
		"--baseline-backend", baselineBackend,
		"--baseline-frontend", baselineFrontend,
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
	})

	unchangedDelta := deltaCell(t, body, "Backend")
	if unchangedDelta != "±0.00pp" {
		t.Fatalf("unchanged Backend delta = %q, want ±0.00pp", unchangedDelta)
	}

	// Edge-probe adjacency: the zero-delta form and the no-baseline em-dash must
	// be visibly different strings (D-12).
	noBaseline, err := os.ReadFile(filepath.Join("testdata", "comment-no-baseline.golden.md"))
	if err != nil {
		t.Fatal(err)
	}
	noBaselineDelta := deltaCell(t, string(noBaseline), "Backend")
	if unchangedDelta == noBaselineDelta {
		t.Fatalf("unchanged delta %q equals no-baseline delta %q — they must differ", unchangedDelta, noBaselineDelta)
	}
}

func TestRenderComment_MissingProfile(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	outPath := filepath.Join(t.TempDir(), "comment.md")
	summaryPath := filepath.Join(t.TempDir(), "summary.md")
	t.Setenv("GITHUB_STEP_SUMMARY", summaryPath)

	err := run([]string{
		"--mode", "comment",
		"--profile", "testdata/does-not-exist-backend.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--baseline-backend", "testdata/baseline-metrics-backend.json",
		"--baseline-frontend", "testdata/baseline-metrics-frontend.json",
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
		"--out", outPath,
	}, os.Stdout)
	if err != nil {
		t.Fatalf("run() error = %v, want nil (comment mode always exits 0)", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	body := string(got)
	if *update {
		if werr := os.WriteFile(filepath.Join("testdata", "comment-unavailable.golden.md"), got, 0o644); werr != nil {
			t.Fatal(werr)
		}
	}
	want, err := os.ReadFile(filepath.Join("testdata", "comment-unavailable.golden.md"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if body != string(want) {
		t.Fatalf("comment body mismatch\n--- got ---\n%s\n--- want ---\n%s", body, want)
	}
	if !strings.Contains(body, "| Backend | "+unavailable+" | "+emDash+" | 80% | "+emDash+" |") {
		t.Fatalf("Backend row is not fully degraded:\n%s", body)
	}
	if !strings.Contains(body, "| Frontend | 72.30% |") {
		t.Fatalf("Frontend row lost its real percentage:\n%s", body)
	}
}

func TestStatusMark_AtGateBoundary(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	boundary := execComment(t, []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile-boundary.txt",
		"--frontend-summary", "testdata/coverage-summary-boundary.json",
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
	})
	passing := execComment(t, []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
	})

	for _, row := range []string{"Backend", "Frontend"} {
		if got, want := statusCell(t, boundary, row), statusCell(t, passing, row); got != want {
			t.Fatalf("%s status at gate boundary = %q, want the passing glyph %q", row, got, want)
		}
	}
}

func TestModeTotal_PrintsOnlyNumber(t *testing.T) {
	var buf bytes.Buffer
	if err := run([]string{"--mode", "total", "--profile", "testdata/backend-profile.txt"}, &buf); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
	if !regexp.MustCompile(`\A[0-9]+\.[0-9]{2}\n\z`).MatchString(buf.String()) {
		t.Fatalf("total stdout = %q, want exactly a 2-decimal number and one newline", buf.String())
	}
}

func TestModeTotal_MissingProfile(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"--mode", "total", "--profile", "testdata/does-not-exist.txt"}, &buf)
	if err == nil {
		t.Fatal("run() error = nil, want non-nil for a missing profile")
	}
	if buf.Len() != 0 {
		t.Fatalf("total stdout = %q, want empty on error", buf.String())
	}
}

func TestModeSidecar_Roundtrip(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	out := filepath.Join(t.TempDir(), "baseline-metrics-backend.json")
	if err := run([]string{
		"--mode", "sidecar",
		"--profile", "testdata/backend-profile.txt",
		"--sha", goldenHeadSHA,
		"--out", out,
	}, io.Discard); err != nil {
		t.Fatalf("sidecar run() error = %v", err)
	}

	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("sidecar is not valid JSON: %v (%s)", err, raw)
	}
	if len(m) != 3 {
		t.Fatalf("sidecar key set = %v, want exactly pct/sha/generated_at", m)
	}
	for _, k := range []string{"pct", "sha", "generated_at"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("sidecar missing key %q (%s)", k, raw)
		}
	}

	var total bytes.Buffer
	if err := run([]string{"--mode", "total", "--profile", "testdata/backend-profile.txt"}, &total); err != nil {
		t.Fatalf("total run() error = %v", err)
	}
	if got, want := string(m["pct"]), strings.TrimSpace(total.String()); got != want {
		t.Fatalf("sidecar pct = %q, want byte-identical to total-mode output %q", got, want)
	}
}

func TestSHAValidation(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"40 lowercase hex", "abc1234def5678abc1234def5678abc1234def56", true},
		{"7 lowercase hex", "abc1234", true},
		{"6 hex is too short", "abc123", false},
		{"41 hex is too long", "abc1234def5678abc1234def5678abc1234def567", false},
		{"uppercase hex rejected", "ABC1234", false},
		{"non-hex character rejected", "abcg123", false},
		{"empty rejected", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validSHA(tc.in); got != tc.want {
				t.Fatalf("validSHA(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSidecar_RejectsBadSHA(t *testing.T) {
	out := filepath.Join(t.TempDir(), "sc.json")
	err := run([]string{
		"--mode", "sidecar",
		"--profile", "testdata/backend-profile.txt",
		"--sha", "NOThex",
		"--out", out,
	}, io.Discard)
	if err == nil {
		t.Fatal("sidecar run() error = nil, want a rejection for a non-hex sha")
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Fatal("sidecar file was written despite an invalid sha")
	}
}

func TestModeComment_RejectsBadHeadSHA(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	body := execComment(t, []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--head-sha", "GARBAGE-not-a-sha",
		"--upstream-red=false",
	})
	if strings.Contains(body, "GARBAGE") {
		t.Fatalf("comment body echoed an invalid head-sha argument:\n%s", body)
	}
	if strings.Contains(body, "head ") {
		t.Fatalf("comment body kept a head-sha line for an invalid argument:\n%s", body)
	}
}

func TestRenderComment_NoUntrustedInterpolation(t *testing.T) {
	withFixedClock(t, "2026-09-02T12:00:00Z")
	body := execComment(t, []string{
		"--mode", "comment",
		"--profile", "testdata/backend-profile-hostile-paths.txt",
		"--frontend-summary", "testdata/coverage-summary.json",
		"--head-sha", goldenHeadSHA,
		"--upstream-red=false",
	})

	raw, err := os.ReadFile("testdata/backend-profile-hostile-paths.txt")
	if err != nil {
		t.Fatal(err)
	}
	var forbidden []string
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "mode:") {
			continue
		}
		file, _, _, perr := parseBlockLine(ln)
		if perr != nil {
			t.Fatalf("hostile fixture line unparseable: %q: %v", ln, perr)
		}
		forbidden = append(forbidden, file)
	}
	if len(forbidden) == 0 {
		t.Fatal("derived no forbidden path strings from the hostile fixture")
	}
	for _, f := range forbidden {
		if strings.Contains(body, f) {
			t.Errorf("comment body interpolated a coverage-file path: %q\n%s", f, body)
		}
	}
	// 15 covered / 20 total statements.
	if !strings.Contains(body, "| Backend | 75.00% |") {
		t.Fatalf("backend percentage wrong or missing from hostile-path render:\n%s", body)
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
