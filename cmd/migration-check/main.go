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
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/danielrpof/drop-tracker/internal/sqlscan"
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
	prevTag := fs.String("prev-tag", "", "previous release tag for the D-15 cross-reference (empty = true bootstrap, D-04)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	switch *mode {
	case "scan":
		return runScan(*filesArg, *prevTag, stdout)
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

func runScan(filesArg, prevTag string, stdout io.Writer) error {
	files := splitFileList(filesArg)
	sort.Strings(files)

	// D-15 cross-reference setup (Task 3). An empty --prev-tag is the true
	// bootstrap case (D-04): the sub-check is skipped with a printed notice
	// and never affects the exit code. A supplied tag that cannot be read
	// is a hard error -- an unverifiable rollback must not be silently
	// masked (D-15/D-04).
	var refs *prevReleaseRefs
	var crossRefNotice string
	if prevTag == "" {
		crossRefNotice = "D-15 previous-release cross-reference: skipped -- no --prev-tag supplied (true bootstrap, D-04).\n"
	} else {
		var err error
		refs, err = buildPrevReleaseRefs(prevTag)
		if err != nil {
			return fmt.Errorf("D-15 cross-reference: %w", err)
		}
	}

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
		annValid := hasAnn && annErr == nil

		// D-15: pull out any cross-reference hit BEFORE the suppression
		// predicate is applied (16-02's shouldSuppress is deliberately
		// shaped for this bypass) -- a cross-reference finding is never
		// suppressible, annotation or not.
		var crossRefFindings, otherFindings []finding
		for _, ff := range fileFindings {
			if cf, ok := crossReferenceFinding(refs, ff, prevTag, ann, annValid); ok {
				crossRefFindings = append(crossRefFindings, cf)
				continue
			}
			otherFindings = append(otherFindings, ff)
		}

		switch {
		case annErr != nil:
			// A half-written annotation is a hard error and never suppresses
			// anything -- the underlying finding still reports normally.
			annotationErrs = append(annotationErrs, fmt.Errorf("%s: %w", path, annErr))
			findings = append(findings, otherFindings...)
		case hasAnn && shouldSuppress(otherFindings):
			suppressed = append(suppressed, suppressedFile{path: path, ann: ann, findings: otherFindings})
		default:
			findings = append(findings, otherFindings...)
		}
		findings = append(findings, crossRefFindings...)
	}

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].file != findings[j].file {
			return findings[i].file < findings[j].file
		}
		return findings[i].line < findings[j].line
	})

	report := crossRefNotice + buildReport(scanned, skipped, findings, suppressed, annotationErrs)
	if _, err := fmt.Fprint(stdout, report); err != nil {
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
	// classCrossRef is D-15's deterministic, non-overridable finding class
	// (Task 3): a DROP/RENAME COLUMN or DROP/RENAME TABLE whose object is
	// still referenced by the previous release's queries/*.sql. It bypasses
	// shouldSuppress entirely -- a well-formed allow-destructive annotation
	// documents intent but cannot make a live N-1 break safe.
	classCrossRef findingClass = "prev-release-reference"
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

	// classCrossRef-only fields (Task 3, D-15): which previous release and
	// which of its queries the dropped/renamed object is still referenced
	// from, plus the file's allow-destructive annotation (if any), echoed
	// into the message so it is visibly the thing that could not override
	// this finding.
	prevTag          string
	queryFile        string
	queryName        string
	annotationTag    string
	annotationReason string
}

func newFinding(file string, line int, class findingClass, kind, table, object string) finding {
	return finding{file: file, line: line, class: class, kind: kind, table: table, object: object}
}

func (f finding) render() string {
	if f.class == classCrossRef {
		return f.renderCrossRef()
	}
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

// renderCrossRef renders the classCrossRef message: names the previous
// release tag and the query file/name the object is referenced from, states
// plainly that the previously-released binary would fail against this
// schema, echoes the file's own allow-destructive annotation (if any) and
// says explicitly that it cannot override this finding, then points at the
// README like the other two classes (D-15 Task 3 action).
func (f finding) renderCrossRef() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d: [%s] %s\n", f.file, f.line, classCrossRef, f.describe())
	fmt.Fprintf(&b, "Still referenced by the previous release (%s): %s query %s. ", f.prevTag, f.queryFile, f.queryName)
	b.WriteString("The previously-released binary would fail against this schema after a rollback -- ")
	b.WriteString("this is a live N-1 break, not merely a documented one. ")
	if f.annotationTag != "" {
		fmt.Fprintf(&b, "The migration-check:allow-destructive annotation (expand-shipped-in=%s, reason=%s) cannot override a live reference. ", f.annotationTag, f.annotationReason)
	}
	b.WriteString("See internal/db/migrations/README.md for the expand -> backfill -> contract sequence.\n")
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

// ---- scan pipeline: sqlscan lexer -> classify ----

func scanFile(path, content string) []finding {
	stripped := sqlscan.StripComments(content)
	var out []finding
	for _, st := range sqlscan.SplitStatements(stripped) {
		out = append(out, classifyStatement(path, st)...)
	}
	return out
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

func classifyStatement(path string, st sqlscan.RawStatement) []finding {
	text := st.Text
	if m := reDropTable.FindStringSubmatch(text); m != nil {
		return []finding{newFinding(path, st.Line, classBackward, "drop_table", sqlscan.StripIdent(m[1]), "")}
	}
	if m := reAlterTable.FindStringSubmatch(text); m != nil {
		table := sqlscan.StripIdent(m[1])
		var out []finding
		for _, clause := range sqlscan.SplitTopLevelCommas(m[2]) {
			clause = strings.TrimSpace(clause)
			if clause == "" {
				continue
			}
			out = append(out, classifyAlterClause(path, st.Line, table, clause)...)
		}
		return out
	}
	return nil
}

func classifyAlterClause(path string, line int, table, clause string) []finding {
	switch {
	case reDropColumn.MatchString(clause):
		m := reDropColumn.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "drop_column", table, sqlscan.StripIdent(m[1]))}
	case reRenameTbl.MatchString(clause):
		m := reRenameTbl.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "rename_table", table, sqlscan.StripIdent(m[1]))}
	case reRenameCol.MatchString(clause):
		m := reRenameCol.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "rename_column", table, sqlscan.StripIdent(m[1])+" -> "+sqlscan.StripIdent(m[2]))}
	case reAlterType.MatchString(clause):
		m := reAlterType.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "alter_type", table, sqlscan.StripIdent(m[1]))}
	case reSetNotNull.MatchString(clause):
		m := reSetNotNull.FindStringSubmatch(clause)
		return []finding{newFinding(path, line, classBackward, "set_not_null", table, sqlscan.StripIdent(m[1]))}
	case reAddCheck.MatchString(clause):
		return []finding{newFinding(path, line, classBackward, "add_check", table, "")}
	case reAddColumn.MatchString(clause):
		if reNotNull.MatchString(clause) && !reDefault.MatchString(clause) {
			m := reAddColumn.FindStringSubmatch(clause)
			return []finding{newFinding(path, line, classUnsafeForward, "add_notnull_no_default", table, sqlscan.StripIdent(m[1]))}
		}
	}
	return nil
}

// ---- D-15: previous-release query cross-reference (Task 2) ----
//
// gitShow reads a file as it existed at a tag, behind readAtTag's shape/path
// gate, so the D-15 cross-reference can ask "does the previously-released
// binary's queries/*.sql still touch this column?" without ever executing
// anything -- it only reads and pattern-matches text (RESEARCH V5).

// gitShow is a package-level function variable over
// `git show <tag>:<path>`, built as an argv slice (never sh -c). Stubbable
// in tests via withStubGitShow so this package's own tests never need a
// real git repository or a real tag (T-16-20).
var gitShow = func(tag, path string) ([]byte, error) {
	out, err := exec.Command("git", "show", tag+":"+path).Output() //nolint:gosec // G204: tag/path gated by readAtTag before this is ever called
	if err != nil {
		return nil, fmt.Errorf("git show %s:%s: %w", tag, path, err)
	}
	return out, nil
}

// allowedGitShowPaths is the fixed set of directories readAtTag may read at
// a tag (T-16-22). A path outside this set is a hard error -- the
// subprocess is never spawned for it.
var allowedGitShowPaths = []string{
	"queries/*.sql",
	migrationsUpGlob,
	"internal/db/sqlc/*.go",
}

// pathAllowedForGitShow reports whether p matches one of
// allowedGitShowPaths. path.Match (not filepath.Match) because git paths are
// always forward-slash regardless of host OS, and `*` must never cross a
// `/` -- that is exactly what keeps `queries/../../etc/passwd` from
// matching `queries/*.sql`.
func pathAllowedForGitShow(p string) bool {
	for _, pattern := range allowedGitShowPaths {
		if ok, _ := path.Match(pattern, p); ok {
			return true
		}
	}
	return false
}

// readAtTag reads path as it existed at tag, gating both arguments before
// gitShow (the stubbable seam) is ever invoked: tag against the same shape
// allowlist the allow-destructive annotation's expand-shipped-in value uses
// (reTagShape), and path against the fixed three-glob allowlist above. A
// rejected argument never reaches gitShow -- tests assert this by stubbing
// gitShow to fail the test if called (T-16-20/T-16-22).
func readAtTag(tag, path string) ([]byte, error) {
	if !reTagShape.MatchString(tag) {
		return nil, fmt.Errorf("readAtTag: rejected tag %q", tag)
	}
	if !pathAllowedForGitShow(path) {
		return nil, fmt.Errorf("readAtTag: path %q is outside the allowed read set", path)
	}
	return gitShow(tag, path)
}

// tableColumn is a normalised (table, column) identifier pair.
type tableColumn struct {
	table  string
	column string
}

// queryRef is a single high-confidence (table, column) reference plus the
// provenance Task 3's cross-reference message needs: which previous-release
// query file and sqlc query name it came from.
type queryRef struct {
	tc        tableColumn
	file      string
	queryName string
}

// prevReleaseRefs is the split high/low confidence reference set D-15 builds
// from the previous release's queries/*.sql. High and low stay separate
// deliberately -- Task 3 only reds on the high-confidence tier (RESEARCH
// Pitfall E); conflating them is the unrecoverable-false-red failure mode.
// params is the separate parameter-name bag (sqlc.arg/narg, @name) --
// collected but never asserted as a column of any specific table.
type prevReleaseRefs struct {
	high   []queryRef
	low    map[tableColumn]bool
	params map[string]bool
}

func newPrevReleaseRefs() *prevReleaseRefs {
	return &prevReleaseRefs{low: map[tableColumn]bool{}, params: map[string]bool{}}
}

func (r *prevReleaseRefs) addHigh(table, column, file, queryName string) {
	table, column = sqlscan.NormalizeIdent(table), sqlscan.NormalizeIdent(column)
	if table == "" || column == "" {
		return
	}
	r.high = append(r.high, queryRef{tc: tableColumn{table: table, column: column}, file: file, queryName: queryName})
}

func (r *prevReleaseRefs) addLow(table, column string) {
	table, column = sqlscan.NormalizeIdent(table), sqlscan.NormalizeIdent(column)
	if table == "" || column == "" {
		return
	}
	r.low[tableColumn{table: table, column: column}] = true
}

// hasHigh reports whether (table, column) appears in the high-confidence
// set and returns the first matching reference for message provenance.
func (r *prevReleaseRefs) hasHigh(table, column string) (queryRef, bool) {
	key := tableColumn{table: sqlscan.NormalizeIdent(table), column: sqlscan.NormalizeIdent(column)}
	for _, ref := range r.high {
		if ref.tc == key {
			return ref, true
		}
	}
	return queryRef{}, false
}

// hasLow reports whether (table, column) appears in the low-confidence set
// -- Task 3 never reds on this; it is at most an informational note.
func (r *prevReleaseRefs) hasLow(table, column string) bool {
	return r.low[tableColumn{table: sqlscan.NormalizeIdent(table), column: sqlscan.NormalizeIdent(column)}]
}

// hasHighAnyColumn reports whether ANY column of table appears in the
// high-confidence set -- used for DROP TABLE / RENAME TABLE, where "still
// referenced" means the previous release touches the table at all (a table
// itself is never a column reference).
func (r *prevReleaseRefs) hasHighAnyColumn(table string) (queryRef, bool) {
	table = sqlscan.NormalizeIdent(table)
	for _, ref := range r.high {
		if ref.tc.table == table {
			return ref, true
		}
	}
	return queryRef{}, false
}

// ---- schema column set: "all columns of table X" (RESEARCH D-15) ----

var reCreateTable = regexp.MustCompile(`(?is)^CREATE\s+TABLE\s+(\S+)\s*\((.*)\)\s*$`)

// tableDefKeywords are ALTER/CREATE TABLE clause prefixes that are not
// column definitions (constraints), so parseSchemaColumns does not
// misinterpret e.g. `CONSTRAINT foo UNIQUE (a, b)` as a column named
// CONSTRAINT.
var tableDefKeywords = []string{"CONSTRAINT", "PRIMARY KEY", "UNIQUE", "CHECK", "FOREIGN KEY"}

// parseSchemaColumns parses CREATE TABLE and ALTER TABLE ... ADD COLUMN
// statements out of migration SQL (read at the previous release tag via
// readAtTag) to build the "all columns of table X" set that a bare
// `SELECT *` / `RETURNING *` over a single table expands to.
func parseSchemaColumns(sql string) map[string][]string {
	stripped := sqlscan.StripComments(sql)
	cols := map[string][]string{}
	for _, st := range sqlscan.SplitStatements(stripped) {
		text := strings.TrimSpace(st.Text)
		if m := reCreateTable.FindStringSubmatch(text); m != nil {
			table := sqlscan.NormalizeIdent(sqlscan.StripSchemaQualifier(sqlscan.StripIdent(m[1])))
			for _, colDef := range sqlscan.SplitTopLevelCommas(m[2]) {
				colDef = strings.TrimSpace(colDef)
				if colDef == "" {
					continue
				}
				upper := strings.ToUpper(colDef)
				isConstraint := false
				for _, kw := range tableDefKeywords {
					if strings.HasPrefix(upper, kw) {
						isConstraint = true
						break
					}
				}
				if isConstraint {
					continue
				}
				fields := strings.Fields(colDef)
				if len(fields) == 0 {
					continue
				}
				cols[table] = append(cols[table], sqlscan.NormalizeIdent(sqlscan.StripIdent(fields[0])))
			}
			continue
		}
		if m := reAlterTable.FindStringSubmatch(text); m != nil {
			table := sqlscan.NormalizeIdent(sqlscan.StripSchemaQualifier(sqlscan.StripIdent(m[1])))
			for _, clause := range sqlscan.SplitTopLevelCommas(m[2]) {
				clause = strings.TrimSpace(clause)
				if reAddColumn.MatchString(clause) {
					cm := reAddColumn.FindStringSubmatch(clause)
					cols[table] = append(cols[table], sqlscan.NormalizeIdent(sqlscan.StripIdent(cm[1])))
				}
			}
		}
	}
	return cols
}

// ---- query block extraction (D-15, Task 2) ----

// reNameMarker splits a queries/*.sql file into sqlc query blocks on its own
// `-- name: X :kind` marker.
var reNameMarker = regexp.MustCompile(`(?im)^--\s*name:\s*(\S+)\s*:\S+\s*$`)

type queryBlock struct {
	name string
	body string
}

func splitQueryBlocks(raw string) []queryBlock {
	locs := reNameMarker.FindAllStringSubmatchIndex(raw, -1)
	if len(locs) == 0 {
		return nil
	}
	var out []queryBlock
	for i, loc := range locs {
		name := raw[loc[2]:loc[3]]
		start := loc[1]
		end := len(raw)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		out = append(out, queryBlock{name: name, body: raw[start:end]})
	}
	return out
}

var (
	reSqlcNamedParam  = regexp.MustCompile(`sqlc\.(?:arg|narg)\('([^']+)'\)`)
	reAtParam         = regexp.MustCompile(`@([A-Za-z_][A-Za-z0-9_]*)`)
	reFromJoinKeyword = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\b`)
	reIdentWithDot    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*`)
	reIdentSimple     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	reWithCTEName     = regexp.MustCompile(`(?i)(?:\bWITH\b|,)\s*([A-Za-z_][A-Za-z0-9_]*)\s+AS\s*\(`)
	reInsertIntoCols  = regexp.MustCompile(`(?is)\bINSERT\s+INTO\s+([A-Za-z_][A-Za-z0-9_.]*)\s*\(([^)]*)\)`)
	reOnConflictCols  = regexp.MustCompile(`(?is)\bON\s+CONFLICT\s*\(([^)]*)\)`)
	reSelectSeg       = regexp.MustCompile(`(?is)\bSELECT\b(.*?)\bFROM\b`)
	reQualifiedRef    = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\b`)
	reQualifiedQuoted = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\."([^"]*)"`)
	reStarQualified   = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\.\*`)
	reBareStarSelect  = regexp.MustCompile(`(?i)\bSELECT\s+\*\s+FROM\b`)
	reBareStarReturn  = regexp.MustCompile(`(?i)\bRETURNING\s+\*`)
	reWhereBareCol    = regexp.MustCompile(`(?i)\b(?:WHERE|AND|OR)\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|<>|!=|<=|>=|<|>|IS\b|IN\s*\()`)
	reBareSelectItem  = regexp.MustCompile(`(?i)^([A-Za-z_][A-Za-z0-9_]*)(?:\s+AS\s+\S+)?$`)
)

// sqlKeywords is a denylist so the bare-identifier passes (WHERE/SELECT-list
// scanning) never mistake a keyword for a column reference. Not
// exhaustive -- only what this repo's queries and the test fixtures use --
// deliberately, since an over-broad bare-column match here would produce an
// unrecoverable D-15 false-red (RESEARCH Pitfall E).
var sqlKeywords = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true, "not": true,
	"null": true, "is": true, "in": true, "as": true, "on": true, "join": true,
	"left": true, "right": true, "inner": true, "outer": true, "with": true,
	"insert": true, "into": true, "values": true, "update": true, "set": true,
	"delete": true, "returning": true, "order": true, "by": true, "group": true,
	"having": true, "limit": true, "asc": true, "desc": true, "distinct": true,
	"conflict": true, "do": true, "nothing": true, "exists": true, "case": true,
	"when": true, "then": true, "else": true, "end": true, "nulls": true,
	"first": true, "last": true, "for": true, "true": true, "false": true,
	"excluded": true,
}

// nextToken skips leading whitespace and returns the next simple
// identifier-shaped token (no dot) plus the remainder of s after it.
func nextToken(s string) (tok, rest string) {
	s = strings.TrimLeft(s, " \t\n\r")
	tok = reIdentSimple.FindString(s)
	return tok, s[len(tok):]
}

// fromJoinTable is one FROM/JOIN occurrence's table (possibly
// schema-qualified) and its resolved alias, if any.
type fromJoinTable struct {
	table string
	alias string
}

// findFromJoinTables scans stripped for every `FROM`/`JOIN` keyword and
// tokenizes what follows by hand, rather than a single combined regex --
// a combined "keyword + table + optional trailing alias" regex would let
// the alias group's match consume the NEXT clause's own FROM/JOIN keyword
// (e.g. matching "JOIN" itself as the alias of the table in a preceding
// `FROM t\nJOIN` pair), which then makes FindAllStringSubmatch skip the
// real second occurrence entirely -- silently dropping a joined table (and
// with it, any column referenced only through that table) from the
// real-table set. Locating just the bare keyword first side-steps that
// match-consumption trap: the keyword regex matches only "FROM"/"JOIN"
// itself, so two adjacent clauses are always found as two separate hits
// regardless of what token follows either one.
func findFromJoinTables(stripped string) []fromJoinTable {
	var out []fromJoinTable
	for _, loc := range reFromJoinKeyword.FindAllStringIndex(stripped, -1) {
		rest := strings.TrimLeft(stripped[loc[1]:], " \t\n\r")
		table := reIdentWithDot.FindString(rest)
		if table == "" {
			continue
		}
		rest = rest[len(table):]
		tok1, rest2 := nextToken(rest)
		alias := ""
		switch {
		case strings.EqualFold(tok1, "AS"):
			if tok2, _ := nextToken(rest2); tok2 != "" && !sqlKeywords[strings.ToLower(tok2)] {
				alias = tok2
			}
		case tok1 != "" && !sqlKeywords[strings.ToLower(tok1)]:
			alias = tok1
		}
		out = append(out, fromJoinTable{table: table, alias: alias})
	}
	return out
}

// extractParams replaces sqlc.arg('x')/sqlc.narg('x') and @x occurrences
// with an inert placeholder (so later regex passes -- especially the
// alias.col qualified-reference scan -- never mistake `sqlc.arg` for a
// qualified column reference) and collects the parameter names into a
// separate bag. Parameter names are never asserted as columns of any table
// (RESEARCH D-15 step 10).
func extractParams(body string) (cleaned string, params map[string]bool) {
	params = map[string]bool{}
	body = reSqlcNamedParam.ReplaceAllStringFunc(body, func(m string) string {
		sub := reSqlcNamedParam.FindStringSubmatch(m)
		params[sub[1]] = true
		return " __param_" + sub[1] + " "
	})
	body = reAtParam.ReplaceAllStringFunc(body, func(m string) string {
		sub := reAtParam.FindStringSubmatch(m)
		params[sub[1]] = true
		return " __param_" + sub[1] + " "
	})
	return body, params
}

// extractReferences parses every sqlc query block in one previous-release
// queries/*.sql file's raw text into refs' high/low confidence (table,
// column) sets (D-15). schemaCols is the previous release's "all columns of
// table X" set (parseSchemaColumns), used to expand a bare `SELECT *` /
// `RETURNING *` over a single real table.
func extractReferences(file, content string, schemaCols map[string][]string, refs *prevReleaseRefs) {
	for _, qb := range splitQueryBlocks(content) {
		extractBlockReferences(file, qb, schemaCols, refs)
	}
}

func extractBlockReferences(file string, qb queryBlock, schemaCols map[string][]string, refs *prevReleaseRefs) {
	body, params := extractParams(qb.body)
	for p := range params {
		refs.params[p] = true
	}
	stripped := sqlscan.StripComments(body)

	cteNames := map[string]bool{}
	for _, m := range reWithCTEName.FindAllStringSubmatch(stripped, -1) {
		cteNames[sqlscan.NormalizeIdent(m[1])] = true
	}

	// aliasMap: normalised alias/table-name -> normalised real table, or ""
	// for a CTE alias (deliberately excluded from the real-table set, per
	// the must_haves truth: `FROM existing`/`FROM updated u` never resolve
	// to a migratable object).
	aliasMap := map[string]string{}
	realTables := map[string]bool{}
	for _, fj := range findFromJoinTables(stripped) {
		table := sqlscan.NormalizeIdent(sqlscan.StripSchemaQualifier(fj.table))
		alias := sqlscan.NormalizeIdent(fj.alias)
		if cteNames[table] {
			if alias != "" {
				aliasMap[alias] = ""
			}
			continue
		}
		realTables[table] = true
		aliasMap[table] = table
		if alias != "" {
			aliasMap[alias] = table
		}
	}

	insertTarget := ""
	if m := reInsertIntoCols.FindStringSubmatch(stripped); m != nil {
		insertTarget = sqlscan.NormalizeIdent(sqlscan.StripSchemaQualifier(m[1]))
		aliasMap[insertTarget] = insertTarget
		aliasMap["excluded"] = insertTarget
		for _, col := range strings.Split(m[2], ",") {
			col = strings.TrimSpace(col)
			if col == "" {
				continue
			}
			refs.addHigh(insertTarget, sqlscan.StripIdent(col), file, qb.name)
		}
		if m := reOnConflictCols.FindStringSubmatch(stripped); m != nil {
			for _, col := range strings.Split(m[1], ",") {
				col = strings.TrimSpace(col)
				if col == "" {
					continue
				}
				refs.addHigh(insertTarget, sqlscan.StripIdent(col), file, qb.name)
			}
		}
	}

	// Qualified alias.col / table.col references (INSERT ... EXCLUDED.col,
	// DO UPDATE SET table.col, RETURNING alias.col, SELECT alias.col, WHERE
	// alias.col, ...): high confidence whenever the alias resolves to a
	// real table.
	for _, m := range reQualifiedRef.FindAllStringSubmatch(stripped, -1) {
		alias := sqlscan.NormalizeIdent(m[1])
		table, ok := aliasMap[alias]
		if !ok || table == "" {
			continue
		}
		refs.addHigh(table, m[2], file, qb.name)
	}
	// Same pass, quoted-column form (`a."Mixed"`) -- reQualifiedRef's
	// unquoted-only character class cannot match a double-quoted column, so
	// this is a separate regex; the quotes are re-added before normalizing
	// so the byte-exact quoted-identifier rule applies (D-15 case folding).
	for _, m := range reQualifiedQuoted.FindAllStringSubmatch(stripped, -1) {
		alias := sqlscan.NormalizeIdent(m[1])
		table, ok := aliasMap[alias]
		if !ok || table == "" {
			continue
		}
		refs.addHigh(table, `"`+m[2]+`"`, file, qb.name)
	}

	// Star expansion: alias.* (SELECT/RETURNING) and bare * -- bare SELECT *
	// only for a single real table; bare RETURNING * always resolves to the
	// INSERT target (RETURNING can only ever return the acted-on table's
	// columns).
	for _, m := range reStarQualified.FindAllStringSubmatch(stripped, -1) {
		alias := sqlscan.NormalizeIdent(m[1])
		if table, ok := aliasMap[alias]; ok && table != "" {
			expandStar(refs, schemaCols, table, file, qb.name)
		}
	}
	if reBareStarSelect.MatchString(stripped) && len(realTables) == 1 {
		for t := range realTables {
			expandStar(refs, schemaCols, t, file, qb.name)
		}
	}
	if reBareStarReturn.MatchString(stripped) && insertTarget != "" {
		expandStar(refs, schemaCols, insertTarget, file, qb.name)
	}

	// Bare unqualified columns in WHERE/AND/OR position (RESEARCH D-15 step
	// 8; also flattens a subquery's own WHERE clause -- B4).
	for _, m := range reWhereBareCol.FindAllStringSubmatch(stripped, -1) {
		classifyBareColumn(refs, realTables, m[1], file, qb.name)
	}

	// Bare unqualified explicit SELECT list items.
	if m := reSelectSeg.FindStringSubmatch(stripped); m != nil {
		for _, item := range sqlscan.SplitTopLevelCommas(m[1]) {
			item = strings.TrimSpace(item)
			if item == "" || item == "*" || strings.Contains(item, ".") {
				continue
			}
			if bm := reBareSelectItem.FindStringSubmatch(item); bm != nil {
				classifyBareColumn(refs, realTables, bm[1], file, qb.name)
			}
		}
	}
	// Note: qualified/star RETURNING items are already covered by the
	// whole-block qualified-ref and star-expansion passes above (flatten,
	// per RESEARCH D-15) -- no separate RETURNING-segment pass needed.
}

// expandStar adds every column of table (per schemaCols) as a high-
// confidence reference. A table missing from schemaCols (schema unknown)
// is silently skipped -- no high-confidence claim can be made without it.
func expandStar(refs *prevReleaseRefs, schemaCols map[string][]string, table, file, queryName string) {
	cols, ok := schemaCols[table]
	if !ok {
		return
	}
	for _, c := range cols {
		refs.addHigh(table, c, file, queryName)
	}
}

// classifyBareColumn attributes a bare (unqualified) column reference: high
// confidence if the query has exactly one real table (unambiguous), low
// confidence (every real table) otherwise -- the RESEARCH D-15 conservatism
// split (Pitfall E). A SQL keyword is never treated as a column reference.
func classifyBareColumn(refs *prevReleaseRefs, realTables map[string]bool, col, file, queryName string) {
	if sqlKeywords[strings.ToLower(col)] {
		return
	}
	if len(realTables) == 1 {
		for t := range realTables {
			refs.addHigh(t, col, file, queryName)
		}
		return
	}
	for t := range realTables {
		refs.addLow(t, col)
	}
}

// ---- D-15 cross-reference wiring into the scan path (Task 3) ----

// prevReleaseQueryFiles is the small, human-curated set of queries/*.sql
// files sqlc.yaml points its codegen at -- hardcoded rather than discovered
// via a local filesystem glob for two reasons: it gives readAtTag's own
// tests a specific, stable path to target with a failing stub
// (TestPrevReleaseCrossRef_GitShowFailureIsRed), and it means path
// discovery never depends on the process's working directory, which
// differs between `go run` at the repo root (production, per this plan's
// own verify commands) and `go test`'s package-directory convention.
var prevReleaseQueryFiles = []string{
	"queries/artists.sql",
	"queries/events.sql",
	"queries/health.sql",
	"queries/watchlist.sql",
}

// buildPrevReleaseRefs resolves the previous release's high/low confidence
// (table, column) reference set (D-15) by reading every
// prevReleaseQueryFiles entry at tag via readAtTag. A read failure for any
// one of them is a hard error -- D-15's own rule: "prior tag exists but
// git show fails for a queries file -> red," never a silent skip, since an
// unverifiable rollback must not be masked.
//
// Schema-column data for SELECT */RETURNING * expansion is gathered
// best-effort from the local internal/db/migrations/*.up.sql glob (present
// only when running from the repo root, as CI does); a glob miss simply
// means no star expansion happens -- none of this guard's deterministic-red
// positions require it, so degrading gracefully here is safe.
func buildPrevReleaseRefs(tag string) (*prevReleaseRefs, error) {
	schemaCols := map[string][]string{}
	if migFiles, err := filepath.Glob(filepath.Join("internal", "db", "migrations", "*.up.sql")); err == nil {
		for _, f := range migFiles {
			data, rerr := readAtTag(tag, filepath.ToSlash(f))
			if rerr != nil {
				continue
			}
			for t, cols := range parseSchemaColumns(string(data)) {
				schemaCols[t] = append(schemaCols[t], cols...)
			}
		}
	}

	refs := newPrevReleaseRefs()
	for _, f := range prevReleaseQueryFiles {
		data, err := readAtTag(tag, f)
		if err != nil {
			return nil, fmt.Errorf("could not read %s at %s: %w", f, tag, err)
		}
		extractReferences(f, string(data), schemaCols, refs)
	}
	return refs, nil
}

// crossReferenceFinding checks a backward-incompatible finding's (table,
// object) against refs' high-confidence set. Only DROP COLUMN, RENAME
// COLUMN, DROP TABLE, and RENAME TABLE are eligible; every other kind
// (unsafe-forward, alter_type, set_not_null, add_check) is out of D-15's
// scope by design (RESEARCH "Conservatism tuning"). A hit produces a
// distinct classCrossRef finding that echoes the file's own annotation (if
// ann is valid) so the message states plainly that it could not override
// this finding.
func crossReferenceFinding(refs *prevReleaseRefs, f finding, prevTag string, ann annotation, annValid bool) (finding, bool) {
	if refs == nil {
		return finding{}, false
	}
	// prevReleaseRefs keys are always schema-stripped (extractBlockReferences,
	// parseSchemaColumns); f.table is raw from the scanner and may carry a
	// schema qualifier (e.g. "public.events") that would otherwise miss the
	// lookup and let a live D-15 reference slip past the annotation override.
	table := sqlscan.StripSchemaQualifier(f.table)
	var ref queryRef
	var hit bool
	switch f.kind {
	case "drop_column":
		ref, hit = refs.hasHigh(table, f.object)
	case "rename_column":
		// f.object is the combined "old -> new" display string
		// (classifyAlterClause); the previous release could only ever have
		// referenced the OLD name.
		old, _, _ := strings.Cut(f.object, " -> ")
		ref, hit = refs.hasHigh(table, old)
	case "drop_table", "rename_table":
		ref, hit = refs.hasHighAnyColumn(table)
	default:
		return finding{}, false
	}
	if !hit {
		return finding{}, false
	}
	out := f
	out.class = classCrossRef
	out.prevTag = prevTag
	out.queryFile = ref.file
	out.queryName = ref.queryName
	if annValid {
		out.annotationTag = ann.tag
		out.annotationReason = ann.reason
	}
	return out, true
}
