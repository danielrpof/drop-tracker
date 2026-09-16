---
phase: quick-260901-muu
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - web/app/lib/authStore.ts
  - internal/config/config.go
  - internal/db/pool.go
  - internal/detection/detector.go
  - internal/detection/musicbrainz.go
  - internal/notifier/notifier.go
autonomous: true
requirements: [QUICK-260901-MUU]

estimate:
  tokens: 70000
  raw_tokens: 70000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "All six files are materially shorter in comment lines and none of them carries a comment block longer than 8 consecutive comment lines — the structural definition of 'no essays'."
    - "Not one line of code, identifier, string literal, struct tag, or trailing `//nolint` directive changed in any of the six files — proven mechanically by a `git diff -G` gate, not by eyeball."
    - "Every design-doc identifier (`D-NN`, `G-NN-N`, `T-NN-NN`, `WR-NN`, `OQ-NN`, `DTCT-NN`, `GATE-NN`, `PERF-NN`, `DATA-NN`, `NTFY-NN`) and every document path (`*.md`, `quick/NNNNNN-xxx`) cited anywhere in the six files at HEAD is still cited at least once after the trim — the information is compressed, never deleted."
    - "Every exported Go symbol in the five Go files still carries a doc comment that starts with its own identifier name."
    - "`go build ./...`, `go vet ./...`, `golangci-lint run`, `make test`, `prettier --check`, and the web vitest suite all pass after the trim."
    - "No file outside the six is modified — in particular no test file, no `.planning/debug/` doc, and none of the other 28 comment-dense files from the density scan."
  artifacts:
    - web/app/lib/authStore.ts
    - internal/config/config.go
    - internal/db/pool.go
    - internal/detection/detector.go
    - internal/detection/musicbrainz.go
    - internal/notifier/notifier.go
  key_links:
    - "trailing `//nolint:gosec` directives (`internal/detection/musicbrainz.go` lines ~509/~510, `internal/detection/detector.go` line ~268) -> `golangci-lint run` (each sits on a CODE line, so the comment-only diff gate structurally forbids touching it; deleting one turns the lint job red)"
    - "ID-preservation gate -> `.planning/debug/resolved/notify-pass-hangs-forever.md` + `.planning/debug/resolved/backlog-songs-trigger-discord.md` (the surviving one-line pointers are the ONLY in-source link to the war stories being deleted)"
    - "`T-04-01`/`T-04-12` ASVS-V5 `range only` notes in `internal/detection/musicbrainz.go` -> Phase 04 SECURITY threat register (the comment IS the audit trail for the mitigation)"
    - "comment-only `git diff -G` gate -> the six files' runtime behavior (the sole proof that COMMENTS ONLY actually held)"
    - "`web/.prettierrc` (`printWidth: 80`, `endOfLine: lf`) + repo-root `.gitattributes` -> `web/app/lib/authStore.ts` (a rewritten comment must survive `prettier --check` with zero reformatting)"
---

<objective>
Trim essay-style comments out of the six most comment-dense source files in the
repo, bringing them in line with the "Comment discipline" rule added to
`.claude/CLAUDE.md` and `.planning/codebase/CONVENTIONS.md` in commit `5de010b`.

Purpose: these six files carry 999 comment lines between them (45-58% of each
file). The density makes both humans and agents work harder to find the code, and
`web/app/lib/authStore.ts` is already named in CONVENTIONS.md as the anti-pattern.
The comments re-argue decisions that already have a design doc, narrate their own
correction history, and reproduce war stories that already live in
`.planning/debug/resolved/`.

Output: six files with the same bytes of code and materially fewer bytes of
comment. Zero logic changes.

**Established at plan time (verified against the live tree — do not re-derive):**

| File | Lines | Comment lines | % | Longest comment run |
|---|---|---|---|---|
| `web/app/lib/authStore.ts` | 211 | 121 | 57% | **81** |
| `internal/notifier/notifier.go` | 341 | 192 | 56% | 29 |
| `internal/db/pool.go` | 199 | 101 | 50% | 19 |
| `internal/config/config.go` | 109 | 59 | 54% | 16 |
| `internal/detection/detector.go` | 284 | 167 | 58% | 30 |
| `internal/detection/musicbrainz.go` | 783 | 359 | 45% | **80** |

- **`golangci-lint` does NOT enforce doc-comment form.** `.golangci.yml` is
  `linters.default: standard` + `gosec`. The standard set is errcheck, govet,
  ineffassign, staticcheck, unused. `revive` is not enabled, `godot` is not
  enabled, and golangci-lint v2's `staticcheck` disables ST1000/ST1020/ST1021/
  ST1022 (the "comment on exported X should be of the form..." checks) by default.
  So doc comments are governed by house style (CONVENTIONS.md "Comments"), not by
  a linter. **Keep the Go convention anyway** — every exported symbol keeps a
  concise doc comment starting with its own name.
- **All six files use `//` line comments exclusively.** There is not one `/* */`
  block comment in any of them. The comment-only diff gate below depends on that;
  do not introduce one.
- **Three trailing `//nolint:gosec` directives are load-bearing** and sit on CODE
  lines (`musicbrainz.go` ~509/~510, `detector.go` ~268). The comment-only gate
  treats those lines as code, so touching one fails the gate. Leave them alone.
- **`internal/notifier/format.go` and `internal/detection/filter.go` are NOT
  targets** but are cross-referenced from target comments (`notifier.go:35` points
  at `format.go`'s reasoning; `notifier.go:203` points at `suppresses`' own doc a
  few lines up). Keep those pointers working — do not edit the referenced files.
- **`internal/authgate/gate.go:109` names `authStore.markGateActive()`** in its own
  comment. That is an identifier reference, not a comment reference; nothing in
  Task 1 renames anything, so it stays valid. `gate.go` is out of scope.
- `make test` runs `go test ./... -race`. **`-race` is a documented pre-existing
  failure on this Windows dev machine** (ThreadSanitizer allocation failure, STATE.md
  Phase 11.1-04). The fallback is spelled out in Task 2's action.

**MUTABLE SCOPE:** the tables and line numbers above are orientation only. The exact
comment lines to cut are authorized from a live read of each file at execution time.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/codebase/CONVENTIONS.md
@.golangci.yml
</context>

<tasks>

<task type="tracer">
  <name>Task 1: Trim web/app/lib/authStore.ts and prove the comment-only gate apparatus works end-to-end</name>
  <precondition>`git status --porcelain --untracked-files=no` prints nothing. Every gate in this plan compares the working tree against `HEAD`, so a pre-existing uncommitted modification to a tracked file would be swept into the measurement and into the commit. If it prints anything, HALT and report rather than stashing.</precondition>
  <files>web/app/lib/authStore.ts</files>
  <action>
This is the tracer: one file, carried through the complete gate suite, proving the
comment-only apparatus works before it is applied to five more files in Task 2. It is
also the priority file — CONVENTIONS.md names it by path as the density anti-pattern.

Read the whole file first. It is 211 lines, 121 of them comment, with a single
unbroken 81-line narrative header block (lines 1-81) before the first `import`.

**Rewrite the comments. Do not touch anything else.** Specifically:

1. **The 81-line header block** collapses to a short module summary of no more than
   6 lines: what `authStore` is (a framework-free pub/sub module holding the SPA's
   two auth signals so `~/lib/api` can import it without pulling in React), what
   `authed` means (optimistic-true, volatile, never persisted — cite `D-16` once),
   and what `gateActive` means (per-browser-session, monotonic, presentation-only,
   sessionStorage-backed — cite `D-18` and `G-14-2`/`G-14-3` once). Do NOT reproduce
   the three-trigger enumeration, the localStorage-vs-sessionStorage argument, or the
   "presentation-only, never access control" paragraph as prose — the decision IDs
   carry a reader to the doc that owns them.

2. **The sessionStorage guard commentary** (the three-failure-mode bullet list, ~18
   lines) collapses to at most 3 lines on `readPersistedGateActive` /
   `persistGateActive` combined. The load-bearing FACT is that the `typeof` probe
   must stay INSIDE the `try` because a present-but-throwing accessor is not
   suppressed by `typeof` — state that once, in one line, and keep the `WR-01`
   citation. Do not enumerate the three failure modes as separate paragraphs.

3. **The `markGateActive` block** (~20 lines) collapses to at most 4 lines. Its two
   real facts are: it never reads or writes `authed` (`D-16`), and its early return
   is scoped to this method only because it runs on every API response. Keep both,
   drop the numbered-list framing and the closing recap paragraph.

4. **The `GATE_ACTIVE_STORAGE_KEY` / `useAuthed` / `useGateActive` comments** each
   drop to 1-2 lines. The `useSyncExternalStore` note keeps exactly one fact: the
   snapshot getters must keep returning the cached module boolean and must never
   re-read the store.

5. **The `persistGateActive` catch-block comment** (2 lines) is already inside the
   1-3 line budget — leave it or shorten it, your call.

Constraints that bind every edit:
- Change zero code lines. No identifier, no string literal (`"dt_gate_active"`,
  `"1"`), no import, no export, no blank-line-inside-code reshuffle.
- Every `D-NN` / `G-NN-N` / `GATE-NN` / `WR-NN` token present in the file today must
  still appear at least once. At HEAD those are exactly: `D-16`, `D-18`, `G-14-2`,
  `G-14-3`, `GATE-05`, `WR-01`. The gate below enforces this — it is what stops a
  trim from becoming a deletion.
- Keep every comment line under 80 columns so `prettier --check` stays green
  (prettier does not reflow `//` comments; a long one just sits there, but the house
  print width is 80 and hand-wrapping is on you).
- Target: **at most 40 comment lines** in the finished file (121 today), and **no
  comment block longer than 8 consecutive comment lines**. Land under the ceiling —
  but never delete a fact to hit a number. If a fact genuinely needs the space, keep
  it and say so in the SUMMARY.

Then run the gate suite in `<verify>`. If the comment-only gate flags the file,
`git --no-pager diff HEAD -- web/app/lib/authStore.ts` and find the code line you
moved; revert that line and re-run. Do not weaken the gate.

Commit with a `docs(quick-260901-muu):` subject. Do not use `--no-verify` — the
prettier-web and gitleaks hooks must run.
  </action>
  <verify>
    <automated>F=web/app/lib/authStore.ts; RX='\b(D|G|T|WR|OQ|AR)-[0-9]+(-[0-9]+)?\b|\b[A-Z]{3,6}-[0-9]{2}\b|[A-Za-z0-9_./-]+\.md|quick/[0-9]{6}-[a-z0-9]+'; test "$(git --no-pager diff HEAD --name-only)" = "$F" && test -z "$(git --no-pager diff HEAD --name-only -G'^[[:space:]]*[^[:space:]/]' -- $F)" && test -z "$(comm -23 <(git show HEAD:$F | grep -oE "$RX" | sort -u) <(grep -oE "$RX" $F | sort -u))" && test "$(awk 'BEGIN{r=0;m=0} /^[ \t]*\/\//{r++; if(r>m)m=r; next} {r=0} END{print m}' $F)" -le 8 && test "$(grep -cE '^[[:space:]]*//' $F)" -le 40 && corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}" && corepack pnpm --dir web test && echo TASK1_GATES_OK</automated>
  </verify>
  <done>`web/app/lib/authStore.ts` is the ONLY tracked file modified. Its diff against HEAD contains zero added/removed lines whose first non-whitespace character is anything but `/` (comment-only, proven by `git diff -G`). All six design-doc identifiers present at HEAD (`D-16`, `D-18`, `G-14-2`, `G-14-3`, `GATE-05`, `WR-01`) still appear. Longest consecutive comment run is <= 8 lines (was 81). Total comment lines <= 40 (was 121). `prettier --check` and the web vitest suite both pass. Committed.</done>
</task>

<task type="auto">
  <name>Task 2: Trim the five Go files, expanding the proven gate to the Go toolchain</name>
  <precondition>Task 1 is committed and `git status --porcelain --untracked-files=no` prints nothing. Docker is running and `make db-up` can bring up the Postgres service container — `make test` depends on it and will not run without a reachable database.</precondition>
  <files>internal/config/config.go, internal/db/pool.go, internal/detection/detector.go, internal/detection/musicbrainz.go, internal/notifier/notifier.go</files>
  <action>
Same rules as Task 1, expanded across five Go files. Read each file in full before
editing it; work one file at a time and re-run the per-file gate after each so a
failure is localized rather than discovered at the end.

**Per-file guidance (orientation — authorize the exact cuts from the live read):**

- **`internal/config/config.go`** (59 comment lines -> ceiling **26**). The
  `EventRetentionDays` field comment ends with a sentence of meta-commentary
  instructing the reader not to extend the judgment call project-wide — delete that
  sentence entirely; it is commentary about the rule rather than about the code. The
  `MusicBrainzPollWorkers`/`DeezerPollWorkers` block re-derives the per-source
  rate-limiter analogy at length — reduce to one line plus the `PERF-01` and
  `D-01`/`D-02`/`D-03` citations. The Phase 14 block re-argues `TRUST_PROXY_HEADERS`
  over eight lines — reduce to two, keeping `D-14`, `D-11`, and `GATE-07`. The two
  in-body validation comments in `Load()` collapse to one line each.

- **`internal/db/pool.go`** (101 -> ceiling **42**). The multi-paragraph war-story
  block above the timeout const group reproduces a root-cause narrative that already
  lives at `.planning/debug/resolved/notify-pass-hangs-forever.md`. Reduce it to 2-3
  lines stating the fact (pgx applies none of these bounds itself; a TCP-ESTABLISHED
  but unanswering socket hangs the process forever without them) **and keep the
  existing `.planning/debug/resolved/notify-pass-hangs-forever.md` path reference** —
  that path is the pointer the prose is being traded for. Each of `connectTimeout`,
  `pingHealthTimeout`, `maxConnIdleTime` keeps 1-2 lines naming what it bounds and
  why the value is what it is. `poolMaxConnsForWorkers`, `dsnSetsMaxConns`,
  `PoolConfig` and the package doc all shrink; `PoolConfig`'s five-paragraph doc goes
  to at most 5 lines, keeping `G-11-1` and the "an explicitly set `pool_max_conns`
  survives untouched" fact.

- **`internal/notifier/notifier.go`** (192 -> ceiling **75**). `defaultSpacing`'s doc
  narrates its own correction history — delete the self-correcting parenthetical and
  simply state the current correct rate limit and why 400ms was chosen (`D-07`).
  `defaultMaxReleaseAgeDays`' 9-line doc goes to 2-3, keeping the "deliberately not
  imported from `internal/detection`" fact and the `format.go` pointer. `dbOpTimeout`
  (24 lines) goes to at most 4: per-call not per-pass, why, plus the
  `notify-pass-hangs-forever.md` path. `suppresses` (29 lines) goes to at most 6,
  keeping both jobs (drain the pre-existing backlog; defence in depth) and the
  `backlog-songs-trigger-discord.md` path and the "undated suppresses" rule.
  `NotifyPending`'s 22-line doc goes to at most 6 (`D-06`, `D-09`, and the three
  error dispositions). The four in-body block comments each drop to 2-3 lines,
  keeping `WR-01` and `WR-03`.

- **`internal/detection/detector.go`** (167 -> ceiling **65**). The repeated
  "declared here, in the consumer, mirroring ..." boilerplate on `RecordingSource`
  and `ReleaseDetailSource` states the same seam convention twice — state it once,
  briefly, on each (one line each is enough; the convention is already documented in
  CONVENTIONS.md "Function Design"). `DefaultNotifyMaxReleaseAgeDays`' 27-line essay
  on why not zero goes to at most 5, keeping the three-lags fact and
  `04-RESEARCH.md`. `notifyGate`'s 29-line doc goes to at most 6, keeping the
  `backlog-songs-trigger-discord.md` path and the "stable per-row property, not a
  temporal latch" fact. `isSeedMode`, `onOrAfterCutoff` and `advanceGroupBaseline`
  each go to at most 5, keeping their accepted-edge facts, `D-10`/`D-14`,
  `04-RESEARCH.md` Pitfall #1 and `PERF-04`/`11-RESEARCH.md`.

- **`internal/detection/musicbrainz.go`** (359 -> ceiling **155**, longest run 80).
  Biggest file, biggest win. `deluxeRecheckWindowDays` (18 lines) -> at most 5.
  `DetectMusicBrainz`'s 42-line doc -> at most 8. `detectGuestFeatures`' 20-line doc
  -> at most 6. `detectDeluxeChanges`' **80-line** doc -> at most 8: it currently
  carries the D-04 rule, the `quick/260826-gj8` window gate, both preference gates,
  a three-case bullet list, the accepted crash-window residual, AND a paragraph on
  loop-abort asymmetry. Keep one line per distinct fact and every ID
  (`D-01`,`D-02`,`D-04`,`D-10`,`PERF-04`,`quick/260826-gj8`,`04-RESEARCH.md`,
  `11-RESEARCH.md`); drop the argumentation. The `KNOWN NARROWING` in-body block
  (~17 lines) -> at most 4, keeping the fact that the 90-day group window and the
  7-day notify window disagree and that this is accepted. `earliestReleaseDate`'s
  19-line doc -> at most 6 (the three precision rules, `WR-01`, `13-REVIEW.md`).
  `withinDeluxeRecheckWindow`'s 32-line doc -> at most 8 (every ambiguous case
  resolves to "still check"; comparison truncates to the shorter operand; `>=` is
  inclusive). `isGuestFeature`, `displayArtistName`, `guestFeatureArt`,
  `releaseTypeForStorage`, `coverArtURLForReleaseGroup` each -> 2-4 lines.
  **Keep the `range only -- ... (T-04-01, ASVS V5)` / `(T-04-12, ASVS V5)` notes** —
  they are the in-source audit trail for a registered threat mitigation. Shorten them
  if you like; do not delete the `T-04-01`/`T-04-12` tokens.

**Constraints binding every edit in this task:**
- Change zero code lines. The three trailing `//nolint:gosec` directives sit on code
  lines and must not be touched — the gate will catch you if they are.
- Every exported symbol (`RecordingSource`, `ReleaseDetailSource`,
  `DefaultNotifyMaxReleaseAgeDays`, `Detector`, `Option`,
  `WithNotifyMaxReleaseAgeDays`, `New`, `DetectMusicBrainz`, `NewPool`,
  `PoolConfig`, `Config`, `Load`, `Sender`, `Sink`, `NoOp`, `NotifyPending`,
  `Notifier`, `WithMaxReleaseAgeDays`, `Select`) keeps a doc comment that starts with
  its own identifier name. Package doc comments (`// Package db ...`,
  `// Package config ...`, `// Package detection ...`, `// Package notifier ...`) all
  stay, shortened to 2-4 lines each.
- Do not touch any `_test.go` file, `internal/notifier/format.go`,
  `internal/detection/filter.go`, `internal/detection/deezer.go`, or anything under
  `.planning/debug/`.
- No comment block longer than 8 consecutive comment lines, in any of the five files.

**Verification notes:**
- `make test` runs `go test ./... -race`, which fails on this Windows dev machine
  with a ThreadSanitizer allocation failure (documented pre-existing limitation,
  STATE.md Phase 11.1-04). If and only if you hit that exact failure, substitute
  `make db-up` followed by `TEST_DATABASE_URL=<the value the Makefile passes>
  go test ./... -count=1` and record the substitution in the SUMMARY. A test
  FAILURE is different from the ThreadSanitizer allocation error — a real failure
  means you changed code and must stop.
- Skip `make coverage-gate` if you had to fall back to a non-`-coverprofile` run;
  a stale `coverage.out` would make it meaningless. Comment-only edits cannot move
  statement coverage. Say which path you took in the SUMMARY.
- `make sqlc-check` is not needed: no `.sql` file and no generated file is touched.

Commit with a `docs(quick-260901-muu):` subject. Do not use `--no-verify`.
  </action>
  <verify>
    <automated>RX='\b(D|G|T|WR|OQ|AR)-[0-9]+(-[0-9]+)?\b|\b[A-Z]{3,6}-[0-9]{2}\b|[A-Za-z0-9_./-]+\.md|quick/[0-9]{6}-[a-z0-9]+'; GO_FILES="internal/config/config.go internal/db/pool.go internal/detection/detector.go internal/detection/musicbrainz.go internal/notifier/notifier.go"; test "$(git --no-pager diff HEAD --name-only | tr '\n' ' ')" = "internal/config/config.go internal/db/pool.go internal/detection/detector.go internal/detection/musicbrainz.go internal/notifier/notifier.go " && test -z "$(git --no-pager diff HEAD --name-only -G'^[[:space:]]*[^[:space:]/]' -- $GO_FILES)" && for f in $GO_FILES; do test -z "$(comm -23 <(git show HEAD:$f | grep -oE "$RX" | sort -u) <(grep -oE "$RX" $f | sort -u))" || { echo "DROPPED IDS in $f"; exit 1; }; test "$(awk 'BEGIN{r=0;m=0} /^[ \t]*\/\//{r++; if(r>m)m=r; next} {r=0} END{print m}' $f)" -le 8 || { echo "COMMENT RUN > 8 in $f"; exit 1; }; done && test "$(grep -cE '^[[:space:]]*//' internal/config/config.go)" -le 26 && test "$(grep -cE '^[[:space:]]*//' internal/db/pool.go)" -le 42 && test "$(grep -cE '^[[:space:]]*//' internal/detection/detector.go)" -le 65 && test "$(grep -cE '^[[:space:]]*//' internal/detection/musicbrainz.go)" -le 155 && test "$(grep -cE '^[[:space:]]*//' internal/notifier/notifier.go)" -le 75 && go build ./... && go vet ./... && golangci-lint run && make test && echo TASK2_GATES_OK</automated>
  </verify>
  <done>Exactly the five named Go files are modified. Their combined diff against HEAD contains zero added/removed lines whose first non-whitespace character is anything but `/`. Every design-doc identifier and document path present in each file at HEAD still appears in that file. No file has a comment block longer than 8 consecutive lines. Comment-line counts are at or under 26/42/65/155/75 respectively (from 59/101/167/359/192). `go build ./...`, `go vet ./...`, `golangci-lint run` and the integration suite all pass. Committed.</done>
</task>

</tasks>

<!-- planner-discipline-allow: none required -->
<!-- Rationale: no acceptance criterion in this plan negative-greps for a literal
     phrase. Every gate is structural (a diff regex, an identifier set-difference, a
     consecutive-run count, a comment-line ceiling), so no `<action>` body can
     self-invalidate a gate by containing text the gate greps for. The phrases being
     deleted are described positionally in the actions rather than quoted. -->

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| editor -> six source files | A "comments only" edit is enforced by discipline, not by the type system — an accidental code change crosses into runtime behavior with no compiler complaint if the change still compiles. |
| deleted comment -> external design/security doc | Rationale that exists ONLY in a comment is destroyed by deletion; rationale that exists in a doc survives if and only if the in-source pointer survives. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-MUU-01 | Tampering | all six target files | high | mitigate | `git diff HEAD -G'^[[:space:]]*[^[:space:]/]'` must return no files: every added/removed line's first non-whitespace character is `/`. Backed by `go build ./...`, `go vet ./...`, `golangci-lint run`, `make test`, `prettier --check`, and the web vitest suite. |
| T-MUU-02 | Tampering | 3 trailing `//nolint:gosec` directives (`musicbrainz.go` ~509/~510, `detector.go` ~268) | high | mitigate | Each directive is appended to a CODE line, so removing or altering it produces a non-`/`-leading diff line and fails T-MUU-01's gate. `golangci-lint run` is the independent backstop (gosec G115 would fire). |
| T-MUU-03 | Repudiation | in-source security audit trail (`T-04-01`, `T-04-12` ASVS V5 `range only` notes) and design-decision pointers | high | mitigate | Identifier-preservation gate: for each file, `comm -23` of the ID/doc-path token set at HEAD against the token set after the trim must be empty. Deleting the last mention of any `D-NN`/`T-NN-NN`/`WR-NN`/`*.md`/`quick/...` token fails the task. |
| T-MUU-04 | Tampering | files outside the six (test files, `.planning/debug/` docs, the other 28 comment-dense files) | medium | mitigate | Each task's gate asserts `git diff HEAD --name-only` equals its own exact file list, so any stray edit anywhere in the tree fails the task before commit. |
| T-MUU-05 | Denial of service | `web/app/lib/authStore.ts` formatting | low | mitigate | `corepack pnpm --dir web exec prettier --check` runs inside the gate, so a hand-written comment that violates the pinned prettier config cannot reach CI's `frontend-test` job. |
| T-MUU-06 | Information disclosure | comment text | low | accept | No comment in any target file contains a secret, credential, or internal hostname (verified at plan time by full read of all six). Deleting comment text cannot leak; the `gitleaks` pre-commit hook remains the backstop. |
| T-MUU-SC | Tampering | npm/pip/cargo installs | high | accept | No package is added, removed, or upgraded. `go.mod`, `go.sum`, `web/package.json`, and `web/pnpm-lock.yaml` are absent from `files_modified`, and T-MUU-04's exact-file-list gate fails the task if any of them changes. No package-legitimacy audit is required because no install command runs. |
</threat_model>

<verification>
Run from the repo root after both tasks are committed. `BASE` is the commit
immediately before this quick task's first commit.

```
BASE=$(git rev-parse HEAD~2)
FILES="web/app/lib/authStore.ts internal/config/config.go internal/db/pool.go internal/detection/detector.go internal/detection/musicbrainz.go internal/notifier/notifier.go"
```

1. **Comments only, across both commits.** `git --no-pager diff $BASE..HEAD --name-only -G'^[[:space:]]*[^[:space:]/]' -- $FILES` prints nothing.
2. **Scope held.** `git --no-pager diff $BASE..HEAD --name-only` lists exactly the six paths and nothing else.
3. **No information deleted.** For each file, the ID/doc-path token set at `$BASE` is a subset of the token set at `HEAD`.
4. **No essays remain.** No file has a run of more than 8 consecutive comment lines (was 81 and 80 in the two worst).
5. **Density down.** Total comment lines across the six drop from 999 to at most 403 (>= 59% reduction).
6. **Go toolchain green.** `go build ./...`, `go vet ./...`, `golangci-lint run`, `make test` all pass.
7. **Frontend green.** `corepack pnpm --dir web exec prettier --check "**/*.{ts,tsx}"` and `corepack pnpm --dir web test` both pass.
8. **Exported symbols documented.** Spot-check that each exported Go symbol listed in Task 2's action still has a doc comment beginning with its own name.
</verification>

<success_criteria>
- Six files modified, zero files outside the six touched.
- Zero code, identifier, string-literal, struct-tag, or `//nolint` changes — proven by the `-G` diff gate over the full `$BASE..HEAD` range, not by inspection.
- Comment lines: 999 -> <= 403 across the six (authStore.ts 121 -> <= 40, config.go 59 -> <= 26, pool.go 101 -> <= 42, detector.go 167 -> <= 65, musicbrainz.go 359 -> <= 155, notifier.go 192 -> <= 75).
- Longest consecutive comment run in any of the six: <= 8 lines.
- Every design-doc identifier and document path cited at `$BASE` still cited at `HEAD`, per file.
- `go build`, `go vet`, `golangci-lint run`, `make test`, `prettier --check`, and the web vitest suite all pass.
- Two commits, both with `docs(quick-260901-muu):` subjects, neither made with `--no-verify`.
</success_criteria>

<output>
Create `.planning/quick/260901-muu-trim-essay-style-code-comments-in-6-file/260901-muu-SUMMARY.md` when done.

Record in it: the before/after comment-line count per file, the before/after longest
comment run per file, whether the `-race` fallback was needed for `make test`, and any
comment block that was deliberately left longer than the guidance because the fact it
carries genuinely needed the space.
</output>
