---
phase: 18-backend-readiness-poll-run-history-status-api
plan: 02
subsystem: api
tags: [ring-buffer, in-process-store, mutex, poller, runrecorder, functional-options, seam]

requires:
  - phase: 04-detection-engine
    provides: poller consumer-declared seam pattern (EventRecorder, Notifier) + fakeNotifier test-double shape
  - phase: 11-bounded-concurrent-polling
    provides: runCycle fan-out + per-source atomic overlap guards the RunRecorder will later instrument
provides:
  - "internal/pollruns package: Store (mutex-guarded ring buffer), RunResult DTO, SourceSnapshot, N=50, SourceMusicBrainz/SourceDeezer/KnownSources, OutcomeOK/OutcomeError/OutcomeCancelled, NewStore, RecordRun, RecordSkip, Snapshot"
  - "RecordRun composes Summary deterministically from counts + normalized outcome and discards any caller-supplied Summary (D-09); Outcome is normalized to a closed 3-value set"
  - "Snapshot returns a per-source deep copy (fresh slice, newest-first) so a /status response can never alias the live ring"
  - "poller.RunRecorder seam + noopRunRecorder never-nil default + WithRunRecorder(RunRecorder) Option + Poller.runs field — wired but never called from runCycle this phase"
affects: [18-04 (/status StatusStore reads pollruns.Snapshot; wires the shared NewStore instance as both RunRecorder and StatusStore), 18.1 (adds the RecordRun/RecordSkip calls inside runCycle + counter aggregation), 19 (SPA System view types against the /status body shaped from SourceSnapshot)]

actuals:
  tokens: 6100
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "In-process mutex-guarded ring buffer as a domain store (internal/pollruns), no DB table — ADR-0001"
    - "Single sync.Mutex covering the ring, last-skipped stamp and skip count; correctness argued by construction + a looped exact-equality test in place of the unavailable race detector"
    - "Consumer-declared write seam with a private no-op default (RunRecorder mirrors Notifier/notifier.NoOp) wired one phase before its first call site"
    - "Record-time recomposition of a user-facing string field (Summary) as a structural guarantee, not a convention"

key-files:
  created:
    - internal/pollruns/pollruns.go
    - internal/pollruns/pollruns_test.go
  modified:
    - internal/poller/poller.go
    - internal/poller/poller_test.go

key-decisions:
  - "RecordSkip was implemented in full in task 1 alongside RecordRun (both are ~4-line mutex ops sharing stateLocked) rather than stubbed for task 3 — the compile assertion var _ RunRecorder = (*pollruns.Store)(nil) that task 1 adds already forces the method to exist, and a correct-by-construction mutex discipline (orchestrator mandate) argues against a temporarily-wrong stub. Task 3 then wrote its skip tests against the finished method."
  - "Ring shape is a bounded slice + trim (append, then st.ring = st.ring[len-N:]), not a head/count ring or container/ring — three lines, inspectable by eye, per the plan's locked discretion call."
  - "sync.Mutex not RWMutex — one operator page view per refresh does not justify a second locking mode."
  - "NewStore pre-seeds musicbrainz and deezer so Snapshot always carries both keys on a fresh instance (stable /status body shape for 18-04)."
  - "Summary format string uses an em dash exactly as D-09 / 18-CONTEXT specify: '<outcome> — <checked> checked, <errored> errored, <events> events'."

patterns-established:
  - "internal/pollruns imports only the standard library — httpserver (18-04) can depend on it without pulling robfig/cron into its tree"
  - "poller imports internal/pollruns purely for the RunResult DTO, exactly as it already imports internal/musicbrainz and internal/deezer for their DTOs"
  - "TestStore_TwoSourceConcurrent: 1000-iteration, two-goroutine, exact-equality (not range) invariant test as the project's stated substitute for go test -race"

requirements-completed: [RUN-02, RUN-04]

coverage:
  - id: D1
    description: "internal/pollruns.Store retains the last 50 run entries per source behind one mutex (N a compile-time constant, no env var); the 51st entry evicts the oldest and the snapshot stays exactly 50, newest-first, with LastRun == History[0]"
    requirement: "RUN-04"
    verification:
      - kind: unit
        ref: "internal/pollruns/pollruns_test.go#TestStore_RingBound,TestStore_NewestFirst,TestStore_EqualStartedAtKeepsInsertionOrder"
        status: pass
    human_judgment: false
  - id: D2
    description: "Snapshot on a fresh store returns a non-nil empty History and nil LastRun for both known sources; a slice from one snapshot is unaffected by later records (no aliasing of the ring)"
    requirement: "RUN-04"
    verification:
      - kind: unit
        ref: "internal/pollruns/pollruns_test.go#TestStore_EmptyStoreSnapshot,TestStore_SnapshotDoesNotAlias"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two sources recording near-simultaneously each end with exactly their own 50 entries and no torn RunResult — proven by a 1000-iteration exact-equality loop (the race-detector substitute)"
    requirement: "RUN-04"
    verification:
      - kind: unit
        ref: "internal/pollruns/pollruns_test.go#TestStore_TwoSourceConcurrent (also run -count=3)"
        status: pass
    human_judgment: false
  - id: D4
    description: "RecordSkip bumps a per-source consecutive-skip count and stamps last-skipped time without creating a run entry; RecordRun resets the count to 0 but never clears the timestamp; skip is per-source"
    requirement: "RUN-02"
    verification:
      - kind: unit
        ref: "internal/pollruns/pollruns_test.go#TestStore_Skip,TestStore_SkipIsPerSource"
        status: pass
    human_judgment: false
  - id: D5
    description: "RunResult carries no free-text error field; RecordRun recomposes Summary from counts + normalized outcome so a caller-supplied connection string never reaches a snapshot; Outcome is normalized to the closed set ok|error|cancelled"
    requirement: "RUN-02"
    verification:
      - kind: unit
        ref: "internal/pollruns/pollruns_test.go#TestStore_SummaryAlwaysComposed,TestStore_OutcomeNormalized"
        status: pass
    human_judgment: false
  - id: D6
    description: "poller.RunRecorder exists with a never-nil no-op default and a WithRunRecorder option; *pollruns.Store satisfies it (compile assertion); a full MusicBrainz + Deezer cycle leaves the recorder's call counters at zero — the seam is wired but inert until Phase 18.1"
    requirement: "RUN-04"
    verification:
      - kind: unit
        ref: "internal/poller/poller_test.go#TestWithRunRecorder_WiresRealStore,TestRunRecorderInertThisPhase; var _ RunRecorder = (*pollruns.Store)(nil)"
        status: pass
      - kind: other
        ref: "! grep -q 'p\\.runs\\.' internal/poller/poller.go; awk runCycle-body | grep -c p.runs == 0; grep -c 'groups []musicbrainz.ReleaseGroup) error' == 1; git status --porcelain internal/detection empty"
        status: pass
    human_judgment: false

duration: 18min
completed: 2026-09-09
status: complete
---

# Phase 18 Plan 02: Backend — Poll-Run History Store & RunRecorder Seam Summary

**`internal/pollruns.Store` — a mutex-guarded per-source ring of the last 50 `RunResult` values plus a scalar skip signal, with `RecordRun` owning summary composition and outcome normalization, and a wired-but-inert `poller.RunRecorder` seam ready for Phase 18.1's call site.**

## Performance

- **Duration:** ~18 min
- **Started:** 2026-09-09T20:12:00Z
- **Completed:** 2026-09-09T20:30:00Z
- **Tasks:** 3 (one tracer, two TDD)
- **Files modified:** 4 (2 created, 2 modified)

## Accomplishments

- New `internal/pollruns` package: `Store` holds a `map[string]*sourceState` behind one `sync.Mutex`; each state carries the entry slice (`ring`, newest-last, len ≤ `N`), `lastSkippedAt`, and `consecutiveSkips`. `N = 50` is a named const (~12h at the 15-minute default). `NewStore` pre-seeds `musicbrainz` and `deezer`.
- `RecordRun(_ context.Context, r RunResult) error` — normalizes `r.Outcome` to the closed set `ok|error|cancelled` (anything else → `error`), overwrites `r.Summary` with `"<outcome> — <checked> checked, <errored> errored, <events> events"`, appends, trims to the newest `N`, resets `consecutiveSkips` to 0. `ctx` is vestigial (seam symmetry, D-07).
- `RecordSkip(source string)` — bumps `consecutiveSkips`, stamps `lastSkippedAt`, creates no history entry (RUN-02, D-05 superseded).
- `Snapshot() map[string]SourceSnapshot` — under the lock, builds a freshly allocated newest-first slice per source, sets `LastRun` to a copy of the newest entry, sets `LastSkippedAt` only when the stamp is non-zero. No returned slice aliases the ring.
- `internal/poller` (additive only): `RunRecorder` interface next to `Notifier`, unexported `noopRunRecorder` default with a compile assertion, `runs RunRecorder` field defaulted in `New`'s struct literal, `WithRunRecorder` option (nil-guarded). `runCycle`, `RunMusicBrainzCycle`, `RunDeezerCycle`, and `EventRecorder` are byte-for-byte unchanged — the recorder is never called this phase.
- 10 new `pollruns` tests + 2 new `poller` tests + `fakeRunRecorder`; the two-source invariant runs a 1000-iteration exact-equality loop as the stated `-race` substitute.

## Task Commits

1. **Task 1: End-to-end tracer — a run recorded through the poller's seam is readable from the store** — `d7a26f5` (feat)
2. **Task 2: The bound and the two-source concurrency invariant** — `488c755` (test)
3. **Task 3: The skip signal, the closed outcome set, and the inert-cycle proof** — `ddb1eee` (test)

**Plan metadata:** _this commit_ (docs: complete plan)

_Tasks 2 and 3 are single `test` commits: the `Store` built by the tracer already satisfied every behavior those tasks specify, so their TDD cycle pinned already-correct code rather than driving new implementation. The one exception — `RecordSkip`'s body — was also written in task 1 (see Deviations)._

## Files Created/Modified

- `internal/pollruns/pollruns.go` — the package: `N`, source/outcome consts, `KnownSources`, `RunResult`, `sourceState`, `Store`, `NewStore`, `RecordRun`, `RecordSkip`, `stateLocked`, `normalizeOutcome`, `composeSummary`, `SourceSnapshot`, `Snapshot`
- `internal/pollruns/pollruns_test.go` — `TestStore_RingBound`, `_NewestFirst`, `_EmptyStoreSnapshot`, `_EqualStartedAtKeepsInsertionOrder`, `_SnapshotDoesNotAlias`, `_TwoSourceConcurrent`, `_Skip`, `_SkipIsPerSource`, `_SummaryAlwaysComposed`, `_OutcomeNormalized`
- `internal/poller/poller.go` — `RunRecorder` interface, `noopRunRecorder`, `WithRunRecorder`, `runs` field + literal default, `internal/pollruns` import
- `internal/poller/poller_test.go` — `var _ RunRecorder = (*pollruns.Store)(nil)`, `TestWithRunRecorder_WiresRealStore`, `fakeRunRecorder`, `TestRunRecorderInertThisPhase`

## Decisions Made

- **`RecordSkip` implemented in task 1, not deferred to task 3.** The plan suggested leaving it for task 3 "only if the compile requires it" — the compile *does* require it, because task 1 adds `var _ RunRecorder = (*pollruns.Store)(nil)` and `RunRecorder` declares `RecordSkip`. A no-op stub would satisfy the compiler but leave the mutex discipline temporarily incomplete, against the orchestrator's correct-by-construction mandate. Implemented fully; task 3 wrote the skip tests against it.
- **Bounded slice + trim** for the ring (not head/count, not `container/ring`) — per the plan's locked discretion call.
- **One `sync.Mutex`**, not `RWMutex` — the read path is one operator page view per refresh.
- **`NewStore` pre-seeds both known sources** so `/status` (18-04) has a stable body shape on a fresh instance.
- **Em dash in the composed `Summary`** — matches D-09 / 18-CONTEXT verbatim; `golangci-lint` (standard + gosec) is clean on it.

## Deviations from Plan

### Auto-fixed / consolidated

**1. [Rule 3 - Blocking sequencing] `RecordSkip` body written in task 1 instead of task 3**
- **Found during:** Task 1 (step 5 adds the `*pollruns.Store` compile assertion against `RunRecorder`, which declares `RecordSkip`)
- **Issue:** The interface assertion forces `(*Store).RecordSkip` to exist for task 1 to build; the plan's "declare it returning without doing work only if the compile requires it" would have shipped a knowingly-incorrect method for one commit.
- **Fix:** Implemented `RecordSkip` fully in task 1 (4 lines, shares `stateLocked` with `RecordRun`). Task 3 then authored `TestStore_Skip` / `TestStore_SkipIsPerSource` against the finished method — they passed on first run.
- **Files modified:** `internal/pollruns/pollruns.go` (in `d7a26f5`)
- **Verification:** `TestStore_Skip`, `TestStore_SkipIsPerSource` green; `RecordSkip` appends to no history slice (asserted).

**2. [Verify-command imprecision, not a code change] task 1 deps check**
- **Found during:** Task 1 `<verify>`
- **Issue:** `go list -deps ./internal/pollruns/ | grep -c 'github.com/danielrpof/drop-tracker'` prints `1`, not `0` — `go list -deps` always lists the target package itself.
- **Resolution:** The real criterion ("`internal/pollruns` imports nothing from this repo") holds: `go list -deps` shows only `github.com/danielrpof/drop-tracker/internal/pollruns` and no other repo path, and the package's imports are `context`, `fmt`, `sync`, `time` only. No code changed.

---

**Total deviations:** 1 sequencing consolidation, 1 verify-command note. No scope creep, no behavior beyond the plan.

## Issues Encountered

- **`make test` / `make test-short` carry `-race`, unusable on this dev box** (ThreadSanitizer allocation failure — a standing STATE.md blocker, `-race` also absent from CI). Substituted the DoD suite gate with a plain `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=<COVER_PKGS>` run, then `make coverage-gate` unchanged. This matches the precedent set in Phases 11.1, 15, and 18-01. Full suite green; backend coverage **90.41%** (floor 80%). `TestStore_TwoSourceConcurrent` is the deliberate compensating control for the missing detector on this plan's concurrency surface.
- **Pre-commit `golangci-lint` hook lints the whole repo** and can exceed a 2-minute command timeout; commits ran under an extended timeout. No hook bypassed (`--no-verify` never used).
- `make sqlc-check` — not applicable (no `queries/` change this plan).

## Definition of Done

- `go build ./...` — clean
- `go vet ./...` — clean
- `golangci-lint run` — 0 issues
- Full `go test ./...` (race substituted) — all packages `ok`, no FAIL
- `make coverage-gate` — 90.41% (required 80%) PASS
- Scope-fence gates — `! grep -q 'p\.runs\.' internal/poller/poller.go` ✓; `runCycle` body references `p.runs` 0 times ✓; `EventRecorder` unwidened (`groups []musicbrainz.ReleaseGroup) error` count 1) ✓; `git status --porcelain internal/detection` empty ✓
- `TestStore_TwoSourceConcurrent -count=3` — green

## Next Phase Readiness

- **Plan 18-04 (`/status`)** can build its `StatusStore` seam directly against `pollruns.Snapshot() map[string]SourceSnapshot`, and wire one shared `pollruns.NewStore()` as both `poller.WithRunRecorder(runs)` and `httpserver.WithStatus(runs, ...)` at the composition root. The DTO shape (`SourceSnapshot{LastRun *RunResult, History []RunResult, LastSkippedAt *time.Time, ConsecutiveSkips int}`) is what its response envelope maps to.
- **Phase 18.1** has a pure `runCycle` edit ahead of it — the `runs` field, the option, and the no-op default are all in place; it adds the `RecordRun` / `RecordSkip` calls and the per-cycle counter aggregation, and must invert `TestRunRecorderInertThisPhase`.
- RUN-02 and RUN-04 remain **Pending** in REQUIREMENTS.md — their `/status` surfacing is 18-04 and the `cancelled` run entry is 18.1. This plan delivers the storage half only.
- No new blockers. The `-race` limitation continues to apply to Phase 18.1's counter aggregation, where the looped invariant test remains the required substitute.

## Self-Check

- Created files present: `internal/pollruns/pollruns.go`, `internal/pollruns/pollruns_test.go` — both found
- Commits present: `d7a26f5`, `488c755`, `ddb1eee` — all in `git log`
- `runCycle` / `RunMusicBrainzCycle` / `RunDeezerCycle` / `EventRecorder` unchanged since `f009dc7`

---
*Phase: 18-backend-readiness-poll-run-history-status-api*
*Completed: 2026-09-09*
