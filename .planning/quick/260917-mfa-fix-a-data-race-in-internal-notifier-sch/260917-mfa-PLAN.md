---
phase: quick/260917-mfa
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [22-CI-RACE-01]

files_modified:
  - internal/notifier/scheduler_test.go

estimate:
  tokens: 40000
  raw_tokens: 25000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "`go test ./internal/notifier/... -race -run TestDigestScheduler_CheckError_LoggedAndLoopContinues` passes repeatedly (>= 50 consecutive iterations) with zero `WARNING: DATA RACE` output."
    - "No goroutine other than the test goroutine can be writing to the test's log buffer at the moment the test reads it: either the write is mutex-guarded, or the scheduler's loop goroutine has provably exited first."
    - "The test still proves both of the things it was written to prove: the due-check failure is logged at Error level, and the loop kept running after that failure (a second check happened)."
    - "`fakeSink`'s notify-before-`fn()` ordering, `waitForCallCount`, `callCount`, and `callAt` are unchanged, so the other seven `TestDigestScheduler_*` tests behave exactly as before."
    - "No file under `internal/notifier/` outside `scheduler_test.go` changes; no production code changes."
  artifacts:
    - internal/notifier/scheduler_test.go
  key_links:
    - "DigestScheduler.check() -> s.logger.Error (scheduler.go:119) -> shared test log buffer: the racing write"
    - "DigestScheduler.Stop() -> s.logger.Info (scheduler.go:134) -> same shared buffer: a SECOND racing write, from the test goroutine, issued before Stop waits on done"
    - "DigestScheduler.Stop() -> <-s.done <- close(s.done) in the loop goroutine: the only real happens-before edge the test can use to know the loop has stopped writing"
---

<objective>
CI run https://github.com/danielrpof/drop-tracker/actions/runs/35272810632 failed the `test`
job under `go test -race`, which skipped `build-scan` and blocked the Phase 22 UAT smoke
test. The failure is a genuine data race in this phase's own test code (added by 22-02,
commits b046d9f/14acffd), not upstream flakiness.

`fakeSink.SendDigestIfDue` pings its `notify` channel *before* calling `fn()`
(scheduler_test.go:45-57), so `waitForCallCount(sink, 2, ...)` unblocks while check #2 is
still running. `TestDigestScheduler_CheckError_LoggedAndLoopContinues` then reads
`buf.String()` (scheduler_test.go:260) while the scheduler's loop goroutine is writing
`"digest due-check failed"` into that same `*bytes.Buffer` from `check()` (scheduler.go:119).
`bytes.Buffer` is not goroutine-safe.

Reproduced locally under WSL2 before writing this plan — the race report names exactly
`bytes.(*Buffer).Write` <- `slog.(*Logger).Error` <- `DigestScheduler.check()` at
scheduler.go:119 racing the test goroutine's read.

Purpose: make the test's log-buffer access safe *and* deterministic, without touching
production code, without touching `fakeSink`'s notify ordering, and without weakening what
the test asserts.

Output: `internal/notifier/scheduler_test.go` gains a small mutex-guarded log buffer and
stops the scheduler before asserting; `go test -race` goes green locally and in CI.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/STATE.md
@internal/notifier/scheduler_test.go
@internal/notifier/scheduler.go
@internal/notifier/notifier_test.go
</context>

<design_contract>

## Why re-ordering alone is NOT enough (read this before implementing)

The obvious fix — move the `buf.String()` read behind a completed `sched.Stop()`, since
`Stop` waits on `<-s.done` and that is a real happens-before edge — is **necessary but not
sufficient**. `Stop` writes to the same buffer itself, on the test goroutine, *before* it
waits:

```go
func (s *DigestScheduler) Stop(ctx context.Context) error {
    s.logger.Info("digest scheduler stopping")   // scheduler.go:134 -- test goroutine writes buf
    ...
    s.stopOnce.Do(func() { close(s.stopCh) })
    select {
    case <-s.done:                                // only NOW is the loop goroutine known done
```

At the moment the test calls `Stop`, check #2 may still be inside its
`s.logger.Error("digest due-check failed", ...)` write. So `Stop`'s own `Info` write races
that `Error` write — a Write/Write race that no amount of test re-ordering can remove,
because every exit door out of the loop goes through a `Stop` that logs.

Therefore the fix is two layers, and both are required:

1. **A mutex-guarded buffer** (structural): removes the Write/Write and Write/String race
   classes outright. This is the load-bearing half.
2. **A checked `Stop()` before the assertions** (determinism): guarantees the loop
   goroutine has exited and *both* failed checks have logged before the buffer is read, so
   the assertion no longer rests on the subtle argument that check #1's log was already
   flushed by the time `waitForCallCount(2)` returned.

## Shape (names and behavior fixed; internals are the executor's call)

```go
// internal/notifier/scheduler_test.go

// syncBuffer is a mutex-guarded log sink. scheduler_test.go is the only file
// in this package whose tests keep a background goroutine logging while the
// test goroutine reads -- DigestScheduler.check() and Stop() both write, from
// different goroutines, so newTestLogger's bare *bytes.Buffer races here.
type syncBuffer struct {
    mu  sync.Mutex
    buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error)   // locks, delegates
func (b *syncBuffer) String() string                // locks, delegates

// newSyncTestLogger mirrors newTestLogger (JSON handler, LevelDebug) but
// writes into a syncBuffer, for the scheduler tests that read log output
// while the loop goroutine is live.
func newSyncTestLogger() (*slog.Logger, *syncBuffer)
```

**Do not change `newTestLogger` in `notifier_test.go`.** It is called from 60+ sites across
four test files, two helpers (`decodeLogRecords`, `decodeModeTransitionRecords`) take its
`*bytes.Buffer` by type, and `digest_test.go:586` calls `buf.Len()`. Every one of those
call sites logs synchronously with no live background goroutine, so none of them races.
Widening the shared helper would be a large, unnecessary diff.

**Do not change `fakeSink`, `waitForCallCount`, `callCount`, or `callAt.`** Six other tests
in this file depend on the notify-before-`fn()` ordering (notably
`TestDigestScheduler_Stop_ReturnsNilAfterInFlightCheckFinishes`, which *requires* being
signalled mid-call so it can observe a blocked in-flight check).

## Local `-race` is Windows-blocked — use WSL2

Confirmed again today: `go test -race` on this Windows box fails to build with
`runtime/cgo: C:\Program Files\Go\pkg\tool\windows_amd64\cgo.exe: exit status 2` — the
pre-existing cgo toolchain break already recorded in STATE.md (Phase 11.1-04). Every
`-race` command in this plan therefore runs through WSL2 Ubuntu (Go 1.26.0, repo visible at
`/mnt/c/CodeProjects/drop-tracker`), which is the same route Phase 01 UAT used. Both were
verified working while writing this plan.

DB-backed tests in this package `t.Skip` cleanly when `TEST_DATABASE_URL`/`DATABASE_URL`
are unset (`internal/testutil/postgres.go:29-42`), so the WSL race runs deliberately unset
them and cover the pure-Go concurrency tests only. The DB half is covered separately by
`make test` on Windows in Task 2 — a skip there is not a pass.

</design_contract>

<tasks>

<task type="tracer">
  <name>Task 1: Close the scheduler_test log-buffer race with a guarded buffer and a checked Stop</name>
  <files>internal/notifier/scheduler_test.go</files>
  <read_first>internal/notifier/scheduler_test.go (whole file -- especially lines 45-89 for the fakeSink/waitForCallCount contract and 241-270 for the failing test), internal/notifier/scheduler.go (check/Stop, lines 88-148), internal/notifier/notifier_test.go lines 45-52 (newTestLogger, the helper being mirrored not modified)</read_first>
  <action>
First capture RED evidence, so the fix is demonstrably tied to the real failure rather than
to a narrowed timing window. Run the unmodified test under WSL2:

  wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker &amp;&amp; unset TEST_DATABASE_URL DATABASE_URL &amp;&amp; go test ./internal/notifier/... -race -count=5 -run TestDigestScheduler_CheckError_LoggedAndLoopContinues'

Expect a WARNING: DATA RACE naming bytes.(*Buffer).Write under slog from
DigestScheduler.check() at scheduler.go:119. Record the observed report in the SUMMARY.

Then add the syncBuffer type and newSyncTestLogger constructor from the design_contract
block near the top of scheduler_test.go, alongside the existing fakeSink helpers. Add
"bytes" to the import block ("sync" and "log/slog" are already imported). Keep the type's
surface at exactly Write and String -- scheduler_test.go needs nothing else, and an unused
method is lint noise.

Then rewrite TestDigestScheduler_CheckError_LoggedAndLoopContinues (currently lines
241-270) only:

- Build its logger with newSyncTestLogger instead of newTestLogger. This is the only test
  in the file that binds the buffer (the other seven discard it with `logger, _ :=`), so no
  other test needs touching.
- Keep Start, keep waitForCallCount(1), keep the single `tickCh &lt;- time.Now()` send, keep
  waitForCallCount(2) -- the tick send is unbuffered, so its completion already proves the
  loop received it, and the call-count wait keeps the intent legible.
- Move the stopCtx construction and the Stop call UP, ahead of both assertions, and check
  its error instead of discarding it with `_ =`. A Stop that returns non-nil means its
  drain deadline expired and the loop goroutine is still live and still writing, so the
  test must fail loudly there rather than read the buffer anyway. Fail with a message that
  says why nil matters, not just that it wasn't nil.
- Leave both assertions as they are, now running after Stop returned nil: the
  strings.Contains check for "digest due-check failed", and the callCount == 2 check with
  its existing "the loop kept going after the error" message. Only two checks are ever
  possible here (one immediate plus one tick), so callCount == 2 stays exact after Stop.

Add a short comment above the moved Stop -- 1-3 lines per CLAUDE.md's comment discipline --
saying that Stop's completed drain is what guarantees the loop goroutine is no longer
writing to the buffer, since the sink signals before fn() runs and a call-count wait alone
returns mid-check. Do not re-derive the whole race analysis inline; this PLAN and the
SUMMARY are the source of truth. Do the same for syncBuffer: one short why-comment on the
type, not a paragraph.

Do not touch scheduler.go or any other production file. Do not touch fakeSink,
waitForCallCount, callCount, callAt, neverFiringTickSource, or notifier_test.go's
newTestLogger.

Finally prove GREEN, in this order:
  1. the single test, verbose, once -- readable confirmation it passes;
  2. the same test at -count=50 -- confidence the race window is closed, not merely
     narrowed (the reproduction above fires within 5);
  3. the whole TestDigestScheduler_* family at -count=20 -- confirms the shared helpers
     were not disturbed;
  4. the full package under -race once -- confirms no sibling test reads a log buffer while
     a goroutine writes it. (Only scheduler_test.go:260 does today; this run is what proves
     that claim rather than asserting it.)

Also run go vet and golangci-lint natively on Windows in this task, since both are fast and
this task is where the file actually changes.
  </action>
  <verify>
    <automated>wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker &amp;&amp; unset TEST_DATABASE_URL DATABASE_URL &amp;&amp; go test ./internal/notifier/... -race -count=1 -run TestDigestScheduler_CheckError_LoggedAndLoopContinues -v'</automated>
    <automated>wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker &amp;&amp; unset TEST_DATABASE_URL DATABASE_URL &amp;&amp; go test ./internal/notifier/... -race -count=50 -run TestDigestScheduler_CheckError_LoggedAndLoopContinues'</automated>
    <automated>wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker &amp;&amp; unset TEST_DATABASE_URL DATABASE_URL &amp;&amp; go test ./internal/notifier/... -race -count=20 -run TestDigestScheduler'</automated>
    <automated>wsl.exe -d Ubuntu -e bash -lc 'cd /mnt/c/CodeProjects/drop-tracker &amp;&amp; unset TEST_DATABASE_URL DATABASE_URL &amp;&amp; go test ./internal/notifier/... -race -count=1'</automated>
    <automated>go vet ./... &amp;&amp; golangci-lint run</automated>
  </verify>
  <done>The RED reproduction was observed and recorded before the fix. After the fix, all four WSL2 race runs pass with zero `WARNING: DATA RACE` in the output, including 50 consecutive iterations of the previously-racing test and one full-package race pass. go vet and golangci-lint are clean. `git diff --stat` shows `internal/notifier/scheduler_test.go` as the only changed file, and `git diff` shows no edit to fakeSink, waitForCallCount, callCount, callAt, or newTestLogger.</done>
  <reversibility rating="reversible">Test-only change confined to one file; `git revert` restores the previous (racing) test with no production or API impact.</reversibility>
</task>

<task type="auto">
  <name>Task 2: Run the Definition of Done, commit, and push so CI re-runs the pipeline</name>
  <files>internal/notifier/scheduler_test.go</files>
  <read_first>.claude/CLAUDE.md (Definition of Done section, and the no-AI-attribution rule)</read_first>
  <action>
Run CLAUDE.md's Definition of Done on Windows, in order, and do not proceed past a red gate:

  1. go vet ./...
  2. golangci-lint run
  3. make db-up, then make test   (integration suite -- this is what covers the DB-backed
     internal/notifier tests that the WSL race runs deliberately skipped)
  4. make coverage-gate           (80% backend floor; a test-only change should not move it,
     but record the measured percentage in the SUMMARY)
  5. make sqlc-check              (local-only, no CI counterpart -- cheap, and it is the only
     guard against generated-code drift leaking in from an earlier session)

The web/ prettier + vitest arm does not apply: no file under web/ changes.

Then commit the single changed file. Do not use --no-verify; let the gitleaks and
golangci-lint hooks run. Write the message in the repo's natural, non-enumerated style,
scoped like the phase's other quick-task commits:

  test(quick-260917-mfa): stop scheduler_test racing its own log buffer

with a short body naming the CI run that caught it and the two-layer fix (guarded buffer +
Stop-before-assert). Per .claude/CLAUDE.md this repo forbids AI-attribution trailers --
no Co-Authored-By, no Claude-Session, no Generated with -- and that rule explicitly
overrides session defaults, so the message ends with the body.

Then push to the branch already checked out -- do NOT create a new branch:

  git push origin gsd/phase-22-scheduled-digest-send

Confirm CI actually re-ran and that the previously-failing job is now green, rather than
assuming the push was enough:

  gh run list --branch gsd/phase-22-scheduled-digest-send --limit 3
  gh run watch &lt;run-id&gt;   (or poll `gh run view &lt;run-id&gt;`)

The success condition is the `test` job green and `build-scan` no longer skipped -- that
skip is what blocked the Phase 22 UAT smoke test. If `test` fails for an unrelated reason,
capture the failure in the SUMMARY and stop; do not start fixing a second, different
problem inside this quick task.
  </action>
  <verify>
    <automated>go vet ./... &amp;&amp; golangci-lint run</automated>
    <automated>make db-up &amp;&amp; make test</automated>
    <automated>make coverage-gate</automated>
    <automated>make sqlc-check</automated>
    <automated>git status --porcelain &amp;&amp; git log --oneline -1</automated>
    <automated>gh run list --branch gsd/phase-22-scheduled-digest-send --limit 3</automated>
  </verify>
  <done>All five Definition of Done gates are green locally; one commit exists on gsd/phase-22-scheduled-digest-send touching only internal/notifier/scheduler_test.go, with no AI-attribution trailer and no --no-verify; the branch is pushed; and a fresh CI run on that branch shows the `test` job passing with `build-scan` no longer skipped.</done>
  <reversibility rating="reversible">A pushed commit on a feature branch; revertable with `git revert` plus a follow-up push, no released artifact involved.</reversibility>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| CI `test` job -> `build-scan` / `release` jobs | a red or falsely-green `test` job is the gate controlling whether an image is built, scanned, and published |
| test goroutine <-> DigestScheduler loop goroutine (shared `*bytes.Buffer`) | the concurrency boundary this fix exists to make safe; test-only, never reached by production code |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-MFA-01 | Denial of Service | CI `test` job / `internal/notifier` race gate | high | mitigate | The racing test blocks every downstream job (`build-scan` skipped), stalling the Phase 22 UAT smoke test and any release from this branch. Fixed at the source: mutex-guarded log buffer plus a drain-checked `Stop`, proven by 50 consecutive `-race` iterations rather than one lucky pass. |
| T-MFA-02 | Tampering | test-suite integrity | medium | mitigate | A race "fixed" by only narrowing the timing window would leave a suite that passes CI while no longer detecting the defect. Guarded against by requiring the RED reproduction first, by fixing the race structurally (a lock, not a sleep or a re-order), and by a full-package `-race` pass. Explicitly forbidden: `time.Sleep` sychronisation, `-race` removal, or `t.Skip`. |
| T-MFA-03 | Tampering | shared test helpers (`fakeSink`, `waitForCallCount`, `newTestLogger`) | medium | mitigate | Changing the notify-before-`fn()` ordering would silently break `TestDigestScheduler_Stop_ReturnsNilAfterInFlightCheckFinishes`, which depends on being signalled mid-call. Mitigated by scoping the new helper to `scheduler_test.go`, leaving `newTestLogger` and all 60+ of its call sites untouched, and by a `git diff` check in Task 1's acceptance criteria. |
| T-MFA-04 | Information Disclosure | commit + push to a remote branch | low | mitigate | Pre-commit gitleaks hook runs (no `--no-verify`); the diff is one test file containing no credentials, and the pushed branch is an existing feature branch, not `main`. |
| T-MFA-SC | Tampering | npm/pip/cargo installs | n/a | accept | No package installs. `go.mod`/`go.sum` are untouched; the only new imports are stdlib (`bytes`). |

*Severity: critical > high > medium > low*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*
</threat_model>

<verification>
1. RED reproduced before the fix: `-race -count=5` on the single test reports `WARNING: DATA
   RACE` at `scheduler.go:119` (`check()` -> `slog.Error` -> `bytes.Buffer.Write`) racing the
   test goroutine's `buf.String()`.
2. GREEN after the fix: the same command, plus `-count=50` on that test, `-count=20` on the
   whole `TestDigestScheduler` family, and one full-package `-race -count=1` — all with zero
   `WARNING: DATA RACE`.
3. `git diff --stat` lists exactly one file: `internal/notifier/scheduler_test.go`.
4. `git diff` contains no change to `fakeSink`, `waitForCallCount`, `callCount`, `callAt`,
   or `newTestLogger`, and no change to any production `.go` file.
5. `go vet ./...`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check`
   all clean on Windows.
6. A fresh CI run on `gsd/phase-22-scheduled-digest-send` shows `test` green and
   `build-scan` no longer skipped.
</verification>

<success_criteria>
- `TestDigestScheduler_CheckError_LoggedAndLoopContinues` survives 50 consecutive `-race`
  iterations with no race report, where 5 iterations reliably reproduced the failure before.
- The race is closed structurally (a guarded buffer plus a drain-checked `Stop`), not by a
  sleep, a re-order that merely shrinks the window, or by weakening an assertion.
- The test still asserts both of its original properties: the failure is logged, and the
  loop kept going (exactly 2 checks).
- No production code and no shared test helper changed; the other seven `TestDigestScheduler_*`
  tests pass untouched.
- CI's `test` job is green on `gsd/phase-22-scheduled-digest-send`, unblocking `build-scan`
  and the Phase 22 UAT smoke test.
</success_criteria>

<output>
Create `.planning/quick/260917-mfa-fix-a-data-race-in-internal-notifier-sch/260917-mfa-SUMMARY.md` when done.
Record in it: the verbatim RED race report header (goroutine ids and the two conflicting
stacks), how many `-race` iterations were run clean, the measured `make coverage-gate`
percentage, confirmation that Windows-native `-race` is still cgo-blocked and WSL2 was the
route used, and the CI run URL showing `test` green with `build-scan` no longer skipped.
</output>
