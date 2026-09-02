// Command coverage-report parses a Go coverage profile and a Vitest coverage
// summary, reads cached baseline sidecars, and renders the single markdown
// table the drop-tracker PR coverage comment and job summary both display.
// After D-17 this is the one place a backend coverage percentage is computed.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// backendGate / frontendGate mirror COVERAGE_THRESHOLD_BACKEND in the Makefile
// and the coverage.thresholds literals in web/vitest.config.ts (D-09). This
// file must not become a second place the thresholds are decided.
const (
	backendGate  = 80.0
	frontendGate = 70.0
)

// unavailable is the fixed cell string for a row whose profile could not be read.
const unavailable = "unavailable"

// emDash is the fixed cell string for an absent delta or status.
const emDash = "—"

// nowUTC returns the render timestamp. It is a package var so the golden test
// can pin it; run() otherwise supplies the real UTC value.
var nowUTC = func() string { return time.Now().UTC().Format(time.RFC3339) }

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("coverage-report", flag.ContinueOnError)
	mode := fs.String("mode", "", "one of: total, sidecar, comment")
	profile := fs.String("profile", "", "path to the Go coverage profile")
	frontendSummary := fs.String("frontend-summary", "", "path to the Vitest coverage-summary.json")
	baselineBackend := fs.String("baseline-backend", "", "path to the backend baseline sidecar")
	baselineFrontend := fs.String("baseline-frontend", "", "path to the frontend baseline sidecar")
	headSHA := fs.String("head-sha", "", "PR head commit SHA")
	upstreamRed := fs.Bool("upstream-red", false, "an upstream CI job was red")
	out := fs.String("out", "", "output file path for comment/sidecar mode")
	sha := fs.String("sha", "", "commit SHA to stamp into the sidecar (sidecar mode)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch *mode {
	case "comment":
		return runComment(commentParams{
			profile:          *profile,
			frontendSummary:  *frontendSummary,
			baselineBackend:  *baselineBackend,
			baselineFrontend: *baselineFrontend,
			headSHA:          *headSHA,
			upstreamRed:      *upstreamRed,
			out:              *out,
		})
	case "total":
		return runTotal(*profile, stdout)
	case "sidecar":
		return runSidecar(*profile, *sha, *out)
	default:
		return fmt.Errorf("unrecognised --mode %q (want total, sidecar, or comment)", *mode)
	}
}

// validSHA reports whether s is a well-formed short or full commit SHA:
// 7 to 40 lowercase hexadecimal characters. It is the only gate by which a
// SHA string from a file or a CLI argument may reach the comment body (V5).
func validSHA(s string) bool {
	if len(s) < 7 || len(s) > 40 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// ---- total mode (D-17) ----

// runTotal writes only the bare 2-decimal backend percentage plus a newline to
// stdout; a parse failure is a hard error so `make coverage-gate` sees an empty
// stdout in plan 15-02 (RESEARCH Pitfall 9).
func runTotal(profilePath string, stdout io.Writer) error {
	pct, err := readBackend(profilePath)
	if err != nil {
		return fmt.Errorf("total mode: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "%.2f\n", round2(pct)); err != nil {
		return fmt.Errorf("total mode: write: %w", err)
	}
	return nil
}

// ---- sidecar mode (D-02) ----

// sidecarOut is the flat baseline object the backend and frontend publish steps
// both emit (D-02); one decoder (readSidecar) reads both. pct is a RawMessage so
// it is byte-identical to what total mode prints for the same profile (D-17).
type sidecarOut struct {
	Pct         json.RawMessage `json:"pct"`
	SHA         string          `json:"sha"`
	GeneratedAt string          `json:"generated_at"`
}

func runSidecar(profilePath, sha, outPath string) error {
	if !validSHA(sha) {
		return fmt.Errorf("sidecar mode: --sha %q is not 7-40 lowercase hex", sha)
	}
	if outPath == "" {
		return errors.New("sidecar mode: --out is required")
	}
	pct, err := readBackend(profilePath)
	if err != nil {
		return fmt.Errorf("sidecar mode: %w", err)
	}
	body, err := json.Marshal(sidecarOut{
		Pct:         json.RawMessage(fmt.Sprintf("%.2f", round2(pct))),
		SHA:         sha,
		GeneratedAt: nowUTC(),
	})
	if err != nil {
		return fmt.Errorf("sidecar mode: marshal: %w", err)
	}
	if err := os.WriteFile(outPath, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("sidecar mode: write %s: %w", outPath, err)
	}
	return nil
}

// round2 rounds half-up to 2 decimals (D-06) -- the same rule make coverage-gate
// compares against after D-17.
func round2(x float64) float64 { return math.Floor(x*100+0.5) / 100 }

// ---- backend profile parse (D-06) ----

// backendTotalPct returns the statement-weighted coverage percentage of a Go
// coverage profile: sum(numStmts where count>0) / sum(numStmts) * 100.
func backendTotalPct(r io.Reader) (float64, error) {
	sc := bufio.NewScanner(r)
	var covered, total int64
	seenHeader := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !seenHeader {
			if !strings.HasPrefix(line, "mode:") {
				return 0, fmt.Errorf("first line %q is not a coverage mode header", line)
			}
			seenHeader = true
			continue
		}
		_, numStmts, count, err := parseBlockLine(line)
		if err != nil {
			return 0, err
		}
		total += numStmts
		if count > 0 {
			covered += numStmts
		}
	}
	if err := sc.Err(); err != nil {
		return 0, fmt.Errorf("scan profile: %w", err)
	}
	if !seenHeader {
		return 0, errors.New("empty coverage profile")
	}
	if total == 0 {
		return 0, errors.New("coverage profile has zero statements")
	}
	return float64(covered) / float64(total) * 100, nil
}

// parseBlockLine splits "<path>:<sL>.<sC>,<eL>.<eC> <numStmts> <count>". The
// file field is split on the LAST colon so a path containing a colon cannot
// corrupt the parse.
func parseBlockLine(line string) (file string, numStmts, count int64, err error) {
	lastSpace := strings.LastIndexByte(line, ' ')
	if lastSpace < 0 {
		return "", 0, 0, fmt.Errorf("malformed profile line %q", line)
	}
	count, err = strconv.ParseInt(line[lastSpace+1:], 10, 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse execution count in %q: %w", line, err)
	}
	rest := line[:lastSpace]
	prevSpace := strings.LastIndexByte(rest, ' ')
	if prevSpace < 0 {
		return "", 0, 0, fmt.Errorf("malformed profile line %q", line)
	}
	numStmts, err = strconv.ParseInt(rest[prevSpace+1:], 10, 64)
	if err != nil {
		return "", 0, 0, fmt.Errorf("parse statement count in %q: %w", line, err)
	}
	posPart := rest[:prevSpace]
	lastColon := strings.LastIndexByte(posPart, ':')
	if lastColon <= 0 {
		return "", 0, 0, fmt.Errorf("malformed profile line %q", line)
	}
	return posPart[:lastColon], numStmts, count, nil
}

// ---- frontend summary parse (D-10) ----

type vitestSummary struct {
	Total struct {
		Lines struct {
			Pct *float64 `json:"pct"`
		} `json:"lines"`
	} `json:"total"`
}

// frontendLinesPct returns total.lines.pct from a Vitest json-summary report.
func frontendLinesPct(r io.Reader) (float64, error) {
	var s vitestSummary
	if err := json.NewDecoder(r).Decode(&s); err != nil {
		return 0, fmt.Errorf("decode vitest summary: %w", err)
	}
	if s.Total.Lines.Pct == nil {
		return 0, errors.New("vitest summary has no total.lines.pct")
	}
	return *s.Total.Lines.Pct, nil
}

// ---- baseline sidecar (D-02) ----

type sidecar struct {
	Pct         *float64 `json:"pct"`
	SHA         string   `json:"sha"`
	GeneratedAt string   `json:"generated_at"`
}

// readSidecar decodes the flat pct/sha/generated_at object. An empty path or a
// missing file is the absent-baseline state (D-11, D-20), not an error. D-20
// wants both the workflow-side matched-key check and this on-disk check.
func readSidecar(path string) (s sidecar, present bool, err error) {
	if path == "" {
		return sidecar{}, false, nil
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return sidecar{}, false, nil
	}
	if err != nil {
		return sidecar{}, false, fmt.Errorf("read baseline sidecar: %w", err)
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return sidecar{}, false, fmt.Errorf("decode baseline sidecar: %w", err)
	}
	return s, true, nil
}

// ---- render (D-09) ----

type coverageRow struct {
	label         string
	gate          float64
	value         float64
	available     bool
	baseline      float64
	baselineKnown bool
}

type commentData struct {
	rows        []coverageRow // fixed order: backend, then frontend
	headSHA     string        // short SHA, or "" to omit
	baselineSHA string        // short SHA, or "" to omit the provenance line
	noBaseline  bool          // no sidecar read at all
	upstreamRed bool
	timestamp   string
}

func render(d commentData) string {
	var b strings.Builder
	b.WriteString("## Coverage\n\n")
	b.WriteString("| Area | Coverage | Δ vs main | Gate | Status |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, r := range d.rows {
		b.WriteString(renderRow(r))
	}
	b.WriteString("\n")
	if d.headSHA != "" {
		fmt.Fprintf(&b, "head %s · %s\n", d.headSHA, d.timestamp)
	} else {
		fmt.Fprintf(&b, "%s\n", d.timestamp)
	}
	switch {
	case d.baselineSHA != "":
		fmt.Fprintf(&b, "baseline: main@%s\n", d.baselineSHA)
	case d.noBaseline:
		// Footer wording avoids the literal "unavailable" token so it does not
		// collide with the per-row unavailable cell string (D-11).
		b.WriteString("Delta not available yet — no main baseline cached (first run or evicted). Absolute coverage shown.\n")
	}
	if d.upstreamRed {
		b.WriteString("Note: an upstream CI job was red; a coverage row may be unavailable.\n")
	}
	return b.String()
}

func renderRow(r coverageRow) string {
	gate := strconv.FormatFloat(r.gate, 'f', -1, 64) + "%"
	if !r.available {
		return fmt.Sprintf("| %s | %s | %s | %s | %s |\n", r.label, unavailable, emDash, gate, emDash)
	}
	value := round2(r.value)
	cov := fmt.Sprintf("%.2f%%", value)
	delta := emDash
	if r.baselineKnown {
		delta = formatDelta(value - round2(r.baseline))
	}
	status := "⚠️"
	if value >= r.gate {
		status = "✅"
	}
	return fmt.Sprintf("| %s | %s | %s | %s | %s |\n", r.label, cov, delta, gate, status)
}

// formatDelta prints a signed percentage-point delta at 2 decimals. An
// unchanged value renders as the plus-or-minus-zero form so "no change" is
// visually distinct from "no baseline" (D-12).
func formatDelta(d float64) string {
	d = round2(d)
	switch {
	case d == 0:
		return "±0.00pp"
	case d > 0:
		return fmt.Sprintf("+%.2fpp", d)
	default:
		return fmt.Sprintf("-%.2fpp", -d)
	}
}

// shortSHA returns the 7-char prefix of a format-valid commit SHA, or "" for
// anything else. This is the V5 control: a rejected SHA is simply omitted from
// the footer, never echoed into the body (D-04).
func shortSHA(s string) string {
	if !validSHA(s) {
		return ""
	}
	return s[:7]
}

// ---- comment mode ----

type commentParams struct {
	profile          string
	frontendSummary  string
	baselineBackend  string
	baselineFrontend string
	headSHA          string
	upstreamRed      bool
	out              string
}

func runComment(p commentParams) error {
	backend := coverageRow{label: "Backend", gate: backendGate}
	if v, err := readBackend(p.profile); err != nil {
		fmt.Fprintf(os.Stderr, "backend coverage unavailable: %v\n", err)
	} else {
		backend.value, backend.available = v, true
	}

	frontend := coverageRow{label: "Frontend", gate: frontendGate}
	if v, err := readFrontend(p.frontendSummary); err != nil {
		fmt.Fprintf(os.Stderr, "frontend coverage unavailable: %v\n", err)
	} else {
		frontend.value, frontend.available = v, true
	}

	bSide, bPresent := sidecarOrNil(p.baselineBackend)
	fSide, fPresent := sidecarOrNil(p.baselineFrontend)

	if bPresent && bSide.Pct != nil {
		backend.baseline, backend.baselineKnown = *bSide.Pct, true
	}
	if fPresent && fSide.Pct != nil {
		frontend.baseline, frontend.baselineKnown = *fSide.Pct, true
	}

	baselineSHA := ""
	if bPresent {
		baselineSHA = shortSHA(bSide.SHA)
	}
	if baselineSHA == "" && fPresent {
		baselineSHA = shortSHA(fSide.SHA)
	}

	body := render(commentData{
		rows:        []coverageRow{backend, frontend},
		headSHA:     shortSHA(p.headSHA),
		baselineSHA: baselineSHA,
		noBaseline:  !bPresent && !fPresent,
		upstreamRed: p.upstreamRed,
		timestamp:   nowUTC(),
	})

	if p.out != "" {
		if err := os.WriteFile(p.out, []byte(body), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write comment output: %v\n", err)
		}
	}
	if sum := os.Getenv("GITHUB_STEP_SUMMARY"); sum != "" {
		if err := appendFile(sum, body); err != nil {
			fmt.Fprintf(os.Stderr, "append step summary: %v\n", err)
		}
	}
	return nil
}

func readBackend(path string) (float64, error) {
	if path == "" {
		return 0, errors.New("no backend profile path")
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open backend profile: %w", err)
	}
	defer func() { _ = f.Close() }()
	return backendTotalPct(f)
}

func readFrontend(path string) (float64, error) {
	if path == "" {
		return 0, errors.New("no frontend summary path")
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open frontend summary: %w", err)
	}
	defer func() { _ = f.Close() }()
	return frontendLinesPct(f)
}

// sidecarOrNil reads a sidecar, swallowing any error to a diagnostic line so
// comment mode never fails on a bad baseline file (D-04).
func sidecarOrNil(path string) (sidecar, bool) {
	s, present, err := readSidecar(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "baseline sidecar ignored: %v\n", err)
		return sidecar{}, false
	}
	return s, present
}

func appendFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
