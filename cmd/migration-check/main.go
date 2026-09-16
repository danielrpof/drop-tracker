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
	var refs *sqlscan.RefSet
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
	file  string
	line  int
	class findingClass
	kind  string
	// table/object hold the NORMALIZED identifiers (for object on a
	// rename_column, the OLD column name alone); rawTable/rawObject hold the
	// as-written spellings and are the only thing describe() renders, so
	// normalizing the parse layer moves no output byte.
	table     string
	object    string
	rawTable  string
	rawObject string

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

func newFinding(file string, line int, class findingClass, kind, table, rawTable, object, rawObject string) finding {
	return finding{
		file: file, line: line, class: class, kind: kind,
		table: table, rawTable: rawTable, object: object, rawObject: rawObject,
	}
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

// describe renders from the raw (as-written) identifier spellings only, so
// normalizing the parse layer changes no output byte.
func (f finding) describe() string {
	switch f.kind {
	case "drop_table":
		return fmt.Sprintf("DROP TABLE %s", f.rawTable)
	case "drop_column":
		return fmt.Sprintf("DROP COLUMN %s on %s", f.rawObject, f.rawTable)
	case "rename_table":
		return fmt.Sprintf("RENAME TABLE %s TO %s", f.rawTable, f.rawObject)
	case "rename_column":
		return fmt.Sprintf("RENAME COLUMN %s on %s", f.rawObject, f.rawTable)
	case "alter_type":
		return fmt.Sprintf("ALTER COLUMN %s TYPE on %s", f.rawObject, f.rawTable)
	case "set_not_null":
		return fmt.Sprintf("SET NOT NULL on %s.%s", f.rawTable, f.rawObject)
	case "add_check":
		return fmt.Sprintf("ADD CHECK on %s", f.rawTable)
	case "add_notnull_no_default":
		return fmt.Sprintf("ADD COLUMN %s NOT NULL (no DEFAULT) on %s", f.rawObject, f.rawTable)
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

// ---- scan pipeline: sqlscan.Parse -> Statement/Action type switch ----
//
// The two switches below ARE the rollback rules: which typed statement and
// which typed ALTER action count as a backward-incompatible or
// unsafe-forward change (D-08). sqlscan owns the parse; the policy stays
// here.

func scanFile(path, content string) []finding {
	var out []finding
	for _, st := range sqlscan.Parse(content) {
		switch s := st.(type) {
		case sqlscan.DropTable:
			out = append(out, newFinding(path, s.Line, classBackward, "drop_table", s.Name, s.RawName, "", ""))
		case sqlscan.AlterTable:
			for _, a := range s.Actions {
				if f, ok := classifyAction(path, s, a); ok {
					out = append(out, f)
				}
			}
		}
	}
	return out
}

func classifyAction(path string, at sqlscan.AlterTable, a sqlscan.Action) (finding, bool) {
	mk := func(class findingClass, kind, object, rawObject string) (finding, bool) {
		return newFinding(path, at.Line, class, kind, at.Name, at.RawName, object, rawObject), true
	}
	switch act := a.(type) {
	case sqlscan.DropColumn:
		return mk(classBackward, "drop_column", act.Column, act.RawColumn)
	case sqlscan.RenameTable:
		return mk(classBackward, "rename_table", act.To, act.RawTo)
	case sqlscan.RenameColumn:
		return mk(classBackward, "rename_column", act.From, act.RawFrom+" -> "+act.RawTo)
	case sqlscan.AlterColumnType:
		return mk(classBackward, "alter_type", act.Column, act.RawColumn)
	case sqlscan.SetNotNull:
		return mk(classBackward, "set_not_null", act.Column, act.RawColumn)
	case sqlscan.AddCheck:
		return mk(classBackward, "add_check", "", "")
	case sqlscan.AddColumn:
		if act.NotNull && !act.HasDefault {
			return mk(classUnsafeForward, "add_notnull_no_default", act.Column, act.RawColumn)
		}
	}
	return finding{}, false
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
func buildPrevReleaseRefs(tag string) (*sqlscan.RefSet, error) {
	schemaCols := map[string][]string{}
	if migFiles, err := filepath.Glob(filepath.Join("internal", "db", "migrations", "*.up.sql")); err == nil {
		for _, f := range migFiles {
			data, rerr := readAtTag(tag, filepath.ToSlash(f))
			if rerr != nil {
				continue
			}
			for t, cols := range sqlscan.SchemaColumns(sqlscan.Parse(string(data))) {
				schemaCols[t] = append(schemaCols[t], cols...)
			}
		}
	}

	refs := &sqlscan.RefSet{}
	for _, f := range prevReleaseQueryFiles {
		data, err := readAtTag(tag, f)
		if err != nil {
			return nil, fmt.Errorf("could not read %s at %s: %w", f, tag, err)
		}
		refs.Merge(sqlscan.QueryColumnRefs(f, string(data), schemaCols))
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
func crossReferenceFinding(refs *sqlscan.RefSet, f finding, prevTag string, ann annotation, annValid bool) (finding, bool) {
	if refs == nil {
		return finding{}, false
	}
	// f.table and f.object arrive already normalized + schema-stripped from
	// sqlscan (CR-01 bug class retired). For rename_column, f.object is the
	// OLD column name alone -- a typed RenameColumn.From, not a re-parsed
	// display string (WR-01 retired).
	var ref sqlscan.Ref
	var hit bool
	switch f.kind {
	case "drop_column", "rename_column":
		ref, hit = refs.Lookup(f.table, f.object)
	case "drop_table", "rename_table":
		ref, hit = refs.LookupAnyColumn(f.table)
	default:
		return finding{}, false
	}
	if !hit {
		return finding{}, false
	}
	out := f
	out.class = classCrossRef
	out.prevTag = prevTag
	out.queryFile = ref.QueryFile
	out.queryName = ref.QueryName
	if annValid {
		out.annotationTag = ann.tag
		out.annotationReason = ann.reason
	}
	return out, true
}
