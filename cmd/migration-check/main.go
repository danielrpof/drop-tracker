// Command migration-check scans branch-new migration SQL for patterns that
// would break rollback safety (N-1, D-08) or hazard a forward deploy on a
// populated table. See internal/db/migrations/README.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("migration-check", flag.ContinueOnError)
	mode := fs.String("mode", "", "one of: scan, changed-files")
	filesArg := fs.String("files", "", "space- or newline-separated list of migration file paths")
	eventName := fs.String("event-name", "", "GitHub Actions event name (pull_request or push)")
	before := fs.String("before", "", "GitHub Actions github.event.before (push only)")
	sha := fs.String("sha", "", "GitHub Actions github.sha")
	baseRef := fs.String("base-ref", "", "GitHub Actions github.base_ref (pull_request only)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch *mode {
	case "scan":
		return runScan(*filesArg, stdout)
	case "changed-files":
		return runChangedFiles(*eventName, *before, *sha, *baseRef, stdout)
	default:
		return fmt.Errorf("unrecognised --mode %q (want scan or changed-files)", *mode)
	}
}

// downMigrationSuffix gates which files runScan actually reads: down-migration
// files are never scanned (every one in this repo carries DROP TABLE/DROP
// COLUMN and the app never runs Down() -- RESEARCH.md Anti-Patterns Pitfall C).
// Everything else (real *.up.sql files and the fixture-named testdata SQL
// files this package's own tests drive) is scanned.
const downMigrationSuffix = ".down.sql"

func runScan(filesArg string, stdout io.Writer) error {
	files := splitFileList(filesArg)
	sort.Strings(files)

	var scanned, skipped []string
	var findings []finding
	var suppressed []suppressedFile
	var annotationErrs []error
	for _, path := range files {
		if strings.HasSuffix(path, downMigrationSuffix) {
			skipped = append(skipped, path)
			continue
		}
		scanned = append(scanned, path)
		// path is a CI-controlled --files argument (workflow input, not end-user
		// input); the repo-wide gosec G304 carve-out for this directory lands in
		// 16-02 Task 3 alongside the mirrored cmd/coverage-report entry.
		data, err := os.ReadFile(path) //nolint:gosec // G304: CI-controlled path, see above
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		raw := string(data)
		fileFindings := scanFile(path, raw)

		ann, hasAnn, annErr := parseAnnotation(raw)
		switch {
		case annErr != nil:
			// A half-written annotation is a hard error and never suppresses
			// anything -- the underlying finding still reports normally.
			annotationErrs = append(annotationErrs, fmt.Errorf("%s: %w", path, annErr))
			findings = append(findings, fileFindings...)
		case hasAnn && shouldSuppress(fileFindings):
			// Plan 03's D-15 previous-release cross-reference deliberately
			// bypasses this predicate: a dropped/renamed object still
			// referenced by the previous release stays red regardless.
			suppressed = append(suppressed, suppressedFile{path: path, ann: ann, findings: fileFindings})
		default:
			findings = append(findings, fileFindings...)
		}
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		return findings[i].line < findings[j].line
	})

	if _, err := fmt.Fprint(stdout, buildReport(scanned, skipped, findings, suppressed, annotationErrs)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if len(findings) > 0 || len(annotationErrs) > 0 {
		var parts []string
		if len(findings) > 0 {
			parts = append(parts, fmt.Sprintf("%d finding(s)", len(findings)))
		}
		for _, e := range annotationErrs {
			parts = append(parts, e.Error())
		}
		return fmt.Errorf("migration-check: %s", strings.Join(parts, "; "))
	}
	return nil
}

func splitFileList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// ---- changed-files mode: diff-base selection (D-16, S2, T-16-21/T-16-23) ----

const (
	allZeroSHA             = "0000000000000000000000000000000000000000"
	mergeBaseFallbackRange = "origin/main...HEAD"
	migrationsUpGlob       = "internal/db/migrations/*.up.sql"
)

var reBranchRef = regexp.MustCompile(`^[A-Za-z0-9._][A-Za-z0-9._/-]*$`)

// validCommitish reports whether s is a well-formed short or full commit SHA:
// 7 to 40 lowercase hexadecimal characters. Mirrors cmd/coverage-report's
// validSHA -- the only gate by which a SHA-shaped string may reach a git
// argv element (T-16-21).
func validCommitish(s string) bool {
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

// validBranchRef gates a branch/ref-shaped input before it can reach a git
// argv element (T-16-21): letters, digits, `.`, `_`, `/`, `-`, and never a
// `..` component (which git itself forbids in a ref and which could
// otherwise be used to construct a path-traversal-shaped remote ref).
func validBranchRef(s string) bool {
	if s == "" || strings.Contains(s, "..") {
		return false
	}
	return reBranchRef.MatchString(s)
}

// commitExists reports whether ref (already shape-validated) names a commit
// reachable in this repository. Package-level seam so diffRange's own tests
// never need a real git repo (mirrors gitShow's seam below).
var commitExists = func(ref string) bool {
	return exec.Command("git", "cat-file", "-e", ref+"^{commit}").Run() == nil
}

// diffRange computes the git revision range changed-files mode diffs for the
// given event shape, gating every ref-shaped input through an allowlist
// before it is ever assembled into a git argv element. Pure function -- no
// git repository is required to unit test it; commitExists is the only
// external seam it calls, and only on an already shape-validated value.
//
//   - pull_request: origin/<base-ref>...HEAD (three-dot merge-base).
//   - push: <before>..<sha> (two-dot literal range) when before is a
//     shape-valid, reachable commit.
//   - push with an all-zeroes or unreachable before (new branch, force-push,
//     Pitfall G): falls back to the merge-base range against origin/main, so
//     a destructive migration pushed straight to main is still scanned
//     (D-16/S2).
//   - any other event name: a hard error, never a silently-empty range.
func diffRange(event, before, sha, baseRef string) (string, error) {
	switch event {
	case "pull_request":
		if !validBranchRef(baseRef) {
			return "", fmt.Errorf("diffRange: rejected --base-ref %q", baseRef)
		}
		return "origin/" + baseRef + "...HEAD", nil
	case "push":
		if before == allZeroSHA {
			return mergeBaseFallbackRange, nil
		}
		if !validCommitish(before) {
			return "", fmt.Errorf("diffRange: rejected --before %q", before)
		}
		if !commitExists(before) {
			return mergeBaseFallbackRange, nil
		}
		if !validCommitish(sha) {
			return "", fmt.Errorf("diffRange: rejected --sha %q", sha)
		}
		return before + ".." + sha, nil
	default:
		return "", fmt.Errorf("diffRange: unrecognised --event-name %q (want pull_request or push)", event)
	}
}

// gitDiffNames is a package-level seam over
// `git diff --name-only --diff-filter=<filter> <rangeArg> -- <pathspec>`,
// argv-slice form, never sh -c. Stubbable in tests (mirrors commitExists and
// gitShow).
var gitDiffNames = func(filter, rangeArg string) ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", "--diff-filter="+filter, rangeArg, "--", migrationsUpGlob).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --diff-filter=%s %s: %w", filter, rangeArg, err)
	}
	return splitFileList(string(out)), nil
}

// filterMigrationUpFiles keeps only paths matching the migrations up-glob,
// defensively re-filtering the diff seam's output (T-16-23) -- a stubbed or
// unexpectedly broad diff result must never leak a non-migration path into
// migration_files. path.Match (not filepath.Match) because git always
// emits forward-slash paths regardless of the host OS.
func filterMigrationUpFiles(files []string) []string {
	var out []string
	for _, f := range files {
		if ok, _ := path.Match(migrationsUpGlob, f); ok {
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

// runChangedFiles implements --mode=changed-files: selects the diff base
// (diffRange), lists added and modified *.up.sql files across it, emits
// migrations_changed/migration_files as GitHub Actions key=value lines to
// $GITHUB_OUTPUT (when set) and mirrors them to stdout unconditionally, then
// hard-errors if any already-released migration file was modified --
// released migrations are immutable (RESEARCH Open Question 1).
func runChangedFiles(eventName, before, sha, baseRef string, stdout io.Writer) error {
	rangeArg, err := diffRange(eventName, before, sha, baseRef)
	if err != nil {
		return fmt.Errorf("changed-files: %w", err)
	}

	added, err := gitDiffNames("A", rangeArg)
	if err != nil {
		return fmt.Errorf("changed-files: diff added: %w", err)
	}
	modified, err := gitDiffNames("M", rangeArg)
	if err != nil {
		return fmt.Errorf("changed-files: diff modified: %w", err)
	}
	added = filterMigrationUpFiles(added)
	modified = filterMigrationUpFiles(modified)

	migrationsChanged := len(added) > 0
	migrationFiles := strings.Join(added, " ")

	if _, err := fmt.Fprintf(stdout, "migrations_changed=%t\nmigration_files=%s\n", migrationsChanged, migrationFiles); err != nil {
		return fmt.Errorf("changed-files: write report: %w", err)
	}

	if ghOut := os.Getenv("GITHUB_OUTPUT"); ghOut != "" {
		if err := appendGithubOutput(ghOut, migrationsChanged, migrationFiles); err != nil {
			return fmt.Errorf("changed-files: %w", err)
		}
	}

	if len(modified) > 0 {
		return fmt.Errorf("changed-files: released migration(s) modified -- released migrations are immutable: %s", strings.Join(modified, ", "))
	}
	return nil
}

// appendGithubOutput writes the two changed-files outputs as GitHub Actions
// key=value lines, appended to the file named by $GITHUB_OUTPUT.
func appendGithubOutput(outPath string, migrationsChanged bool, migrationFiles string) error {
	f, err := os.OpenFile(outPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open GITHUB_OUTPUT %s: %w", outPath, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintf(f, "migrations_changed=%t\nmigration_files=%s\n", migrationsChanged, migrationFiles); err != nil {
		return fmt.Errorf("write GITHUB_OUTPUT: %w", err)
	}
	return nil
}

// buildReport renders the scanned-file list unconditionally (including the
// empty case, D-07), then suppressed-file/annotation-error lines, then every
// finding in the caller-sorted order. Building into a strings.Builder first
// keeps every intermediate write error-free; run() checks the single write
// to stdout.
func buildReport(scanned, skipped []string, findings []finding, suppressed []suppressedFile, annotationErrs []error) string {
	var b strings.Builder
	b.WriteString("Scanned migration files:\n")
	if len(scanned) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, f := range scanned {
		fmt.Fprintf(&b, "  %s\n", f)
	}
	for _, f := range skipped {
		fmt.Fprintf(&b, "Skipped (down-migration file, never scanned): %s\n", f)
	}
	for _, s := range suppressed {
		fmt.Fprintf(&b, "%s: suppressed %d finding(s) via migration-check:allow-destructive (expand-shipped-in=%s, reason=%s)\n",
			s.path, len(s.findings), s.ann.tag, s.ann.reason)
	}
	for _, e := range annotationErrs {
		fmt.Fprintf(&b, "Annotation error: %s\n", e)
	}
	if len(findings) == 0 {
		b.WriteString("No destructive or unsafe-forward migration statements found.\n")
		return b.String()
	}
	b.WriteString("\n")
	for _, f := range findings {
		b.WriteString(f.render())
		b.WriteString("\n")
	}
	return b.String()
}

// ---- finding classes (D-08 / S4) ----

type findingClass string

const (
	classBackward      findingClass = "backward-incompatible"
	classUnsafeForward findingClass = "unsafe-forward"
)

// backwardIncompatibleMsg and unsafeForwardMsg are the two class-specific
// remediation paragraphs (S4): a backward-incompatible change breaks the
// N-1 rollback invariant, an unsafe-forward change breaks or locks the
// deploy itself. Both name internal/db/migrations/README.md so a red build
// teaches the rule instead of reporting opaque DDL (D-09, SC #2).
const backwardIncompatibleMsg = `Backward-incompatible change: this breaks the expand/contract rule and the N-1
rollback invariant -- the previously-released binary must still run correctly against
this schema after a rollback. Split the change across two releases: expand (add the
new shape) in one release, contract (remove the old shape) only once that release has
shipped and is no longer a rollback target. See internal/db/migrations/README.md.`

const unsafeForwardMsg = `Unsafe-forward change: adding a NOT NULL column with no DEFAULT fails outright, or
locks the table for a full rewrite, against any table that already has rows. Add a
DEFAULT in the same ADD COLUMN clause, or backfill in a separate migration before
tightening NOT NULL. See internal/db/migrations/README.md.`

type finding struct {
	file   string
	line   int
	class  findingClass
	kind   string
	table  string
	object string
}

func newFinding(file string, line int, class findingClass, kind, table, object string) finding {
	return finding{file: file, line: line, class: class, kind: kind, table: table, object: object}
}

func (f finding) render() string {
	label := string(f.class)
	msg := backwardIncompatibleMsg
	if f.class == classUnsafeForward {
		msg = unsafeForwardMsg
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d: [%s] %s\n", f.file, f.line, label, f.describe())
	b.WriteString(msg)
	b.WriteString("\n")
	return b.String()
}

func (f finding) describe() string {
	switch f.kind {
	case "drop_table":
		return fmt.Sprintf("DROP TABLE %s", f.table)
	case "drop_column":
		return fmt.Sprintf("DROP COLUMN %s on %s", f.object, f.table)
	case "rename_table":
		return fmt.Sprintf("RENAME TABLE %s TO %s", f.table, f.object)
	case "rename_column":
		return fmt.Sprintf("RENAME COLUMN %s on %s", f.object, f.table)
	case "alter_type":
		return fmt.Sprintf("ALTER COLUMN %s TYPE on %s", f.object, f.table)
	case "set_not_null":
		return fmt.Sprintf("SET NOT NULL on %s.%s", f.table, f.object)
	case "add_check":
		return fmt.Sprintf("ADD CHECK on %s", f.table)
	case "add_notnull_no_default":
		return fmt.Sprintf("ADD COLUMN %s NOT NULL (no DEFAULT) on %s", f.object, f.table)
	default:
		return f.kind
	}
}

// ---- allow-destructive annotation (D-07/S4, grammar locked at the 16-02 checkpoint) ----

// annotation is a parsed
// `-- migration-check:allow-destructive expand-shipped-in=<tag> reason=<text>`
// comment. Grammar is a one-way door (immutable *.up.sql files) -- do not
// change its shape once a released migration carries one.
type annotation struct {
	tag    string
	reason string
}

type suppressedFile struct {
	path     string
	ann      annotation
	findings []finding
}

var (
	reAnnotationLine = regexp.MustCompile(`(?m)^[ \t]*--[ \t]*migration-check:allow-destructive(.*)$`)
	reAnnoTag        = regexp.MustCompile(`\bexpand-shipped-in=(\S+)`)
	reAnnoReason     = regexp.MustCompile(`\breason=(.*)$`)
	reTagShape       = regexp.MustCompile(`^v?[0-9]+(\.[0-9]+){0,2}(-[0-9A-Za-z.-]+)?$`)
)

// parseAnnotation looks for an allow-destructive comment in raw (pre-strip)
// file text, since the annotation lives inside a `--` comment. ok reports
// whether the annotation prefix was found at all; a false ok with a nil err
// means the file simply carries no annotation. Both keys are required: a
// partial annotation returns a non-nil err naming the missing key and the
// zero annotation, so nothing is ever suppressed on a half-written contract.
// expand-shipped-in's value is shape-validated here (T-16-11) -- it is not
// yet stored or reachable by any subprocess (that lands in Plan 03).
func parseAnnotation(raw string) (ann annotation, ok bool, err error) {
	m := reAnnotationLine.FindStringSubmatch(raw)
	if m == nil {
		return annotation{}, false, nil
	}
	tail := m[1]

	tagM := reAnnoTag.FindStringSubmatch(tail)
	if tagM == nil {
		return annotation{}, true, errors.New(`migration-check:allow-destructive annotation missing required key "expand-shipped-in"`)
	}
	reasonM := reAnnoReason.FindStringSubmatch(tail)
	if reasonM == nil {
		return annotation{}, true, errors.New(`migration-check:allow-destructive annotation missing required key "reason"`)
	}

	tag := tagM[1]
	if !reTagShape.MatchString(tag) {
		return annotation{}, true, fmt.Errorf("migration-check:allow-destructive expand-shipped-in %q does not match the expected form vX.Y.Z", tag)
	}

	return annotation{tag: tag, reason: strings.TrimSpace(reasonM[1])}, true, nil
}

// shouldSuppress is the suppression predicate: a valid annotation suppresses
// every backward-incompatible and unsafe-forward finding in its file (D-08
// revision). Plan 03's D-15 previous-release query cross-reference bypasses
// this predicate deliberately -- it is a separate, non-overridable check.
func shouldSuppress(findings []finding) bool {
	return len(findings) > 0
}

// ---- scan pipeline: stripComments -> splitStatements -> classify ----

func scanFile(path, content string) []finding {
	stripped := stripComments(content)
	var out []finding
	for _, st := range splitStatements(stripped) {
		out = append(out, classifyStatement(path, st)...)
	}
	return out
}

// stripComments removes `--` line comments and /* */ block comments,
// replacing their bytes with spaces while preserving every newline so
// 1-based line numbers computed downstream stay accurate. String literals
// and $tag$...$tag$ dollar-quoted spans pass through untouched.
func stripComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		switch {
		case c == '-' && i+1 < n && src[i+1] == '-':
			for i < n && src[i] != '\n' {
				b.WriteByte(' ')
				i++
			}
		case c == '/' && i+1 < n && src[i+1] == '*':
			b.WriteByte(' ')
			b.WriteByte(' ')
			i += 2
			for i < n && (src[i] != '*' || i+1 >= n || src[i+1] != '/') {
				if src[i] == '\n' {
					b.WriteByte('\n')
				} else {
					b.WriteByte(' ')
				}
				i++
			}
			if i+1 < n {
				b.WriteByte(' ')
				b.WriteByte(' ')
				i += 2
			}
		case c == '\'':
			b.WriteByte(c)
			i++
			i = copySingleQuoted(&b, src, i)
		case c == '$':
			if tag, ok := dollarTagAt(src, i); ok {
				end := copyDollarQuoted(&b, src, i, tag)
				i = end
			} else {
				b.WriteByte(c)
				i++
			}
		default:
			b.WriteByte(c)
			i++
		}
	}
	return b.String()
}

// copySingleQuoted copies a '...' string literal body (with ” escapes)
// starting just after the opening quote, returning the index past the
// closing quote.
func copySingleQuoted(b *strings.Builder, src string, i int) int {
	n := len(src)
	for i < n {
		c := src[i]
		b.WriteByte(c)
		if c == '\'' {
			if i+1 < n && src[i+1] == '\'' {
				b.WriteByte(src[i+1])
				i += 2
				continue
			}
			return i + 1
		}
		i++
	}
	return i
}

// dollarTagAt reports whether src[i:] begins a $tag$ delimiter and returns
// the full delimiter (e.g. "$$" or "$body$").
func dollarTagAt(src string, i int) (string, bool) {
	rest := src[i+1:]
	end := strings.IndexByte(rest, '$')
	if end < 0 || !isValidDollarTag(rest[:end]) {
		return "", false
	}
	return src[i : i+1+end+1], true
}

func isValidDollarTag(tag string) bool {
	for _, r := range tag {
		if r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// copyDollarQuoted copies from the opening tag through the matching closing
// tag (inclusive), returning the index just past it. If no closing tag
// exists the rest of the source is copied verbatim.
func copyDollarQuoted(b *strings.Builder, src string, i int, tag string) int {
	b.WriteString(tag)
	i += len(tag)
	closeIdx := strings.Index(src[i:], tag)
	if closeIdx < 0 {
		b.WriteString(src[i:])
		return len(src)
	}
	b.WriteString(src[i : i+closeIdx+len(tag)])
	return i + closeIdx + len(tag)
}

type statement struct {
	text string
	line int
}

// splitStatements splits comment-stripped SQL on `;`, respecting '...'
// string literals and $tag$...$tag$ dollar-quoting so an embedded semicolon
// never ends a statement early.
func splitStatements(src string) []statement {
	var stmts []statement
	var cur strings.Builder
	line := 1
	startLine := 1
	started := false
	inSingle := false
	dollarTag := ""
	n := len(src)
	i := 0
	for i < n {
		c := src[i]
		if c == '\n' {
			line++
		}
		if !started && !isBlank(c) {
			started = true
			startLine = line
		}
		switch {
		case dollarTag != "":
			if strings.HasPrefix(src[i:], dollarTag) {
				cur.WriteString(dollarTag)
				i += len(dollarTag)
				dollarTag = ""
				continue
			}
			cur.WriteByte(c)
			i++
		case inSingle:
			cur.WriteByte(c)
			if c == '\'' {
				if i+1 < n && src[i+1] == '\'' {
					cur.WriteByte(src[i+1])
					i += 2
					continue
				}
				inSingle = false
			}
			i++
		case c == '\'':
			inSingle = true
			cur.WriteByte(c)
			i++
		case c == '$':
			if tag, ok := dollarTagAt(src, i); ok {
				dollarTag = tag
				cur.WriteString(tag)
				i += len(tag)
			} else {
				cur.WriteByte(c)
				i++
			}
		case c == ';':
			if text := strings.TrimSpace(cur.String()); text != "" {
				stmts = append(stmts, statement{text: text, line: startLine})
			}
			cur.Reset()
			started = false
			i++
		default:
			cur.WriteByte(c)
			i++
		}
	}
	if text := strings.TrimSpace(cur.String()); text != "" {
		stmts = append(stmts, statement{text: text, line: startLine})
	}
	return stmts
}

func isBlank(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

// ---- classify (D-08 reliably-detectable pattern set) ----

var (
	reDropTable  = regexp.MustCompile(`(?is)^DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	reAlterTable = regexp.MustCompile(`(?is)^ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(\S+)\s+(.*)$`)
	reDropColumn = regexp.MustCompile(`(?is)^DROP\s+COLUMN\s+(?:IF\s+EXISTS\s+)?(\S+)`)
	reRenameTbl  = regexp.MustCompile(`(?is)^RENAME\s+TO\s+(\S+)`)
	reRenameCol  = regexp.MustCompile(`(?is)^RENAME\s+(?:COLUMN\s+)?(\S+)\s+TO\s+(\S+)`)
	reAlterType  = regexp.MustCompile(`(?is)^ALTER\s+COLUMN\s+(\S+)\s+(?:SET\s+DATA\s+)?TYPE\b`)
	reSetNotNull = regexp.MustCompile(`(?is)^ALTER\s+COLUMN\s+(\S+)\s+SET\s+NOT\s+NULL\b`)
	reAddCheck   = regexp.MustCompile(`(?is)^ADD\s+(?:CONSTRAINT\s+\S+\s+)?CHECK\s*\(`)
	reAddColumn  = regexp.MustCompile(`(?is)^ADD\s+(?:COLUMN\s+)?(\S+)\s`)
	reNotNull    = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
	reDefault    = regexp.MustCompile(`(?i)\bDEFAULT\b`)
)

func classifyStatement(path string, st statement) []finding {
	text := st.text
	if m := reDropTable.FindStringSubmatch(text); m != nil {
		return []finding{newFinding(path, st.line, classBackward, "drop_table", stripIdent(m[1]), "")}
	}
	if m := reAlterTable.FindStringSubmatch(text); m != nil {
		table := stripIdent(m[1])
		var out []finding
		for _, clause := range splitTopLevelCommas(m[2]) {
			clause = strings.TrimSpace(clause)
			if clause == "" {
				continue
			}
			out = append(out, classifyAlterClause(path, st.line, table, clause)...)
		}
		return out
	}
	return nil
}

func classifyAlterClause(path string, line int, table, clause string) []finding {
	switch {
	case reDropColumn.MatchString(clause):
		m := reDropColumn.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "drop_column", table, stripIdent(m[1]))}
	case reRenameTbl.MatchString(clause):
		m := reRenameTbl.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "rename_table", table, stripIdent(m[1]))}
	case reRenameCol.MatchString(clause):
		m := reRenameCol.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "rename_column", table, stripIdent(m[1])+" -> "+stripIdent(m[2]))}
	case reAlterType.MatchString(clause):
		m := reAlterType.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "alter_type", table, stripIdent(m[1]))}
	case reSetNotNull.MatchString(clause):
		m := reSetNotNull.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "set_not_null", table, stripIdent(m[1]))}
	case reAddCheck.MatchString(clause):
		return []finding{newFinding(path, line, classBackward, "add_check", table, "")}
	case reAddColumn.MatchString(clause):
		if reNotNull.MatchString(clause) && !reDefault.MatchString(clause) {
			m := reAddColumn.FindStringSubmatch(clause)
			return []finding{newFinding(path, line, classUnsafeForward, "add_notnull_no_default", table, stripIdent(m[1]))}
		}
	}
	return nil
}

// splitTopLevelCommas splits an ALTER TABLE action list on commas that sit
// outside parens and string literals, so `numeric(10,2)` or a CHECK(...)
// clause's internal commas never split into a bogus extra clause.
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	inSingle := false
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inSingle:
			if c == '\'' {
				inSingle = false
			}
		case c == '\'':
			inSingle = true
		case c == '(':
			depth++
		case c == ')':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

// stripIdent trims surrounding double quotes and trailing punctuation a
// regex capture group may include at a clause boundary.
func stripIdent(s string) string {
	s = strings.Trim(s, `"`)
	return strings.TrimRight(s, ";,()")
}
