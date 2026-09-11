---
phase: 18-backend-readiness-poll-run-history-status-api
verified: 2026-09-09T00:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: none
  previous_score: n/a
human_verification:

  - test: "On the first push to `main` after this phase merges, open the CI run and inspect the `build-scan` job."
    expected: "The 'Verify build provenance landed in the binary' step runs and passes (the extracted binary contains the commit SHA), and the `release` job shows a `docker load` of the scanned tarball with no `docker build` of its own."
    why_human: "Only observable against the real GitHub Actions runner; no unit or local test exercises the CI-built image path end to end (18-03 coverage D4, 18-VALIDATION Manual-Only)."
  - test: "`docker compose up --build` with `INSTANCE_PASSPHRASE` set. Log in through the SPA, then fetch `/status` in the same browser session; then fetch `/status` in a private window with no session."
    expected: "Authenticated: body matches `docs/api/status-contract.md` field for field — `sources` carries `musicbrainz` and `deezer` with `last_run` null and `history` `[]` on a fresh instance, `poll_interval_seconds` matches `POLL_INTERVAL`, `watchlist_size` matches the watchlist view, `instance.app_version` is `dev` for a local image. No session: `401`."
    why_human: "SPA login round-trip against a real running instance in a browser — automation covers the wiring against a real DB (boot_e2e) but not the cookie/session path through the SPA (18-04 coverage D10)."
---

# Phase 18: Backend — Readiness, Status Surface & App Version — Verification Report

**Phase Goal:** An operator (or a machine) can tell a live drop-tracker apart from a merely-running process, and the `/status` JSON contract Phase 19 types against exists and is frozen — all without touching the poll cycle's hot path.
**Verified:** 2026-09-09
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

The phase goal is a *contract-and-mechanism* goal, not a live-data goal: the poll cycle is
deliberately untouched (Phase 18.1 wires the recorder calls), so the truths concern the
existence, shape, wiring and safety of `/ready`, `/status`, the run-history store, the seam,
and the build-version injection — not runtime poll data flowing through them.

All five ROADMAP success criteria are backed by code that exists, is substantive, is wired
at the composition root, and is exercised by passing unit + real-Postgres integration tests.
Two end-to-end observations (the CI pipeline runner and the SPA login round-trip) remain
for a human — both were pre-identified as manual in the plans.

### Observable Truths

| # | Truth (ROADMAP success criterion) | Status | Evidence |
|---|-----------------------------------|--------|----------|
| 1 | Unauthenticated `GET /ready` → `200` with `schema_applied`/`schema_expected` when healthy; `503` with `db_unreachable`/`schema_behind`/`schema_dirty` when Postgres down / schema behind / dirty; no DSN/driver/path in body; never `401` on a gated instance with no cookie | ✓ VERIFIED | `internal/httpserver/ready.go` implements exactly the D-01/D-02/D-03 shape (`!dirty && applied >= expected`, dirty checked first, raw cause only to `httplog.SetAttrs`). Tests pass: `TestReady_Healthy/DBUnreachable/SchemaBehind/SchemaDirty/AheadOfSource/EmptySchemaMigrations/NoLeak/GatedNo401/InertBranch/Timeout/ExactPathOnly` (11) + `TestBootToReady_EndToEnd` against real migrated Postgres. Registered on the root router (`server.go:215`) outside the `if gate != nil` branch. |
| 2 | `/health` answers exactly as in v1.3 — same code, body, timeout | ✓ VERIFIED | `internal/httpserver/health.go` and `health_test.go` are absent from the phase diff (`git diff 9e764d5 HEAD`). New `TestHealth_ContractUnchanged` golden byte-pin added in `ready_test.go` (not by editing the frozen file) and passes. |
| 3 | Gated `GET /status` returns last run per source, last N runs per source, per-source `last_skipped_at` + `consecutive_skips`, `watchlist_size`, `poll_interval_seconds`, `instance{app_version,schema_applied,schema_expected}`; no session → `401`; fresh instance → empty run lists (not error); no DSN/webhook/path/driver error on any path | ✓ VERIFIED | `internal/httpserver/status.go` envelope matches the frozen contract. Tests pass: `TestStatus_EmptyHistory/PollInterval/SkipSignal/HistoryNewestFirst/HistoryBounded/WatchlistSize/Gated401/Ungated200/NoLeak/EmptyErrorMessage/SchemaErrorStillTwoHundred/NotConfigured` (12) + `TestBootToStatus_EndToEnd` against real Postgres. No `Error`/`Detail`/`Message`/`LastError` field on any response struct. `watchlist_size` from `CountWatchlist` `count(*)` (`TestCountWatchlist_Integration`). |
| 4 | `app_version` is the short commit SHA on a CI image and `dev` on a flagless local build; the `release` job pushes the byte-identical scanned image | ✓ VERIFIED (CI runtime → human check) | `internal/buildinfo` (`Version="dev"` default, `Short()` → 12 chars). `Dockerfile` `ARG VERSION` (bare) + `-ldflags -X github.com/danielrpof/drop-tracker/internal/buildinfo.Version=${VERSION:-dev}` (fully-qualified path). `cmd/server/main.go` links `buildinfo` via a boot log line so `-X` is not a no-op. CI `build-scan` passes `VERSION=${{ github.sha }}` + a provenance-assertion step that greps the extracted binary; `release` job has 0 build steps and a `docker load`. `TestVersion_DefaultsToDev`, `TestShort_Truncates`, `TestVersion_Injected` (passes under `go test -ldflags -X`). The first-real-pipeline-run observation is a documented human check. |
| 5 | `pollruns.Store` holds the last N (=50) run entries per source behind a mutex; `poller.RunRecorder` seam exists and is wired to the real store, inert only because `runCycle` does not call it; `StatusStore` reads the buffer for `/status` | ✓ VERIFIED | `internal/pollruns/pollruns.go`: `const N = 50`, one `sync.Mutex` covering ring + skip stamp + skip count, bounded-slice-plus-trim ring. `internal/poller/poller.go`: `RunRecorder` interface, `noopRunRecorder` never-nil default, `WithRunRecorder`, `runs` field. `cmd/server/main.go:248`: one `pollruns.NewStore()` passed to both `httpserver.WithStatus` and `poller.WithRunRecorder(runs)`. `runCycle`, `RunMusicBrainzCycle`, `RunDeezerCycle` reference `p.runs` zero times; `EventRecorder` not widened; `internal/detection` untouched. Tests pass: `TestStore_RingBound/NewestFirst/EmptyStoreSnapshot/EqualStartedAtKeepsInsertionOrder/SnapshotDoesNotAlias/TwoSourceConcurrent (1000×)/Skip/SkipIsPerSource/SummaryAlwaysComposed/OutcomeNormalized`, `TestWithRunRecorder_WiresRealStore`, `TestRunRecorderInertThisPhase`. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Behavior-dependent truths — behavioral evidence confirmed

| Invariant | Test | Result |
|-----------|------|--------|
| 51st entry evicts oldest; length stays exactly 50, newest-first | `TestStore_RingBound`, `TestStore_NewestFirst` | pass |
| `RecordSkip` bumps count + stamps time, no history entry; `RecordRun` resets count to 0 but never clears the stamp | `TestStore_Skip` (explicit `LastSkippedAt.Equal(stampBefore)` assertion after the reset) | pass |
| `Snapshot` returns a fresh non-aliasing slice | `TestStore_SnapshotDoesNotAlias` | pass |
| Two sources recording near-simultaneously each keep exactly their own 50 | `TestStore_TwoSourceConcurrent` (1000 iterations, exact-equality — the accepted `-race` substitute on this box) | pass (`-count=3` in plan) |
| Ready = `!dirty && applied >= expected`, never equality | `TestReady_AheadOfSource` (applied = expected + 1 → 200) | pass |
| `/ready` bounded under a hung ping | `TestReady_Timeout` | pass |

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/httpserver/ready.go` | `handleReady`, `SchemaVersioner`, `readyResponse`, `readyCheckTimeout` | ✓ VERIFIED | 82 lines; wired via `WithReadiness` + root-router registration; used by `main.go` |
| `internal/db/schema_version.go` | `RowQuerier`, `SchemaVersion`, `SchemaVersionReader`, `NewSchemaVersionReader` | ✓ VERIFIED | hand-rolled `SELECT version, dirty FROM schema_migrations`; `ErrNoRows`→(0,false,nil); negative-version guard (gosec G115). Used by both `/ready` and `/status`. |
| `internal/db/migrate.go` `ExpectedSchemaVersion` | embedded-source max, reuses `maxSourceVersion` | ✓ VERIFIED | called once in `main.go:151`; `TestExpectedSchemaVersion` pins it to 7 |
| `internal/pollruns/pollruns.go` | `Store`, `RunResult`, `SourceSnapshot`, `N`, source/outcome consts, `NewStore`, `RecordRun`, `RecordSkip`, `Snapshot` | ✓ VERIFIED | stdlib-only imports; `RecordRun` recomposes `Summary` + normalizes `Outcome`; wired at composition root |
| `internal/poller/poller.go` RunRecorder seam | interface + no-op default + `WithRunRecorder` + `runs` field | ✓ VERIFIED | wired, inert; `var _ RunRecorder = (*pollruns.Store)(nil)` compiles |
| `internal/buildinfo/buildinfo.go` | `Version` var (default `dev`), `Short()` | ✓ VERIFIED | linked into `cmd/server` binary; `-X` path matches `go.mod` module path |
| `internal/httpserver/status.go` | `WatchlistCounter`, `StatusStore`, `StatusDeps`, `WithStatus`, `handleStatus`, response types | ✓ VERIFIED | registered in `registerDataRoutes` (gated); no error-derived response field |
| `docs/api/status-contract.md` | frozen contract, every handler json key present | ✓ VERIFIED | 158 lines; all 21 keys align with `status.go` json tags; auth table, examples, "changing this contract" section |
| `queries/watchlist.sql` `CountWatchlist` + regenerated sqlc | `count(*)` query + `Querier` method | ✓ VERIFIED | `make sqlc-check` clean (no diff); `CountWatchlist(ctx) (int64, error)` on `Querier` |
| `Dockerfile` | bare `ARG VERSION` + version link flag | ✓ VERIFIED | one `ARG VERSION`, fully-qualified `-X` path, header comment amended |
| `.github/workflows/full-pipeline.yml` | `build-args VERSION=${{ github.sha }}` + provenance step in `build-scan` only | ✓ VERIFIED | `release` job: 0 builds, `docker load` present |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `cmd/server/main.go` | `httpserver.New` | `WithReadiness(db.NewSchemaVersionReader(pool), expectedSchema)` | ✓ WIRED |
| `cmd/server/main.go` | `httpserver.New` + `poller.New` | one `runs := pollruns.NewStore()` → `WithStatus{Store: runs}` and `poller.WithRunRecorder(runs)` | ✓ WIRED (single instance, grep count 1) |
| `server.go` root router | `handleReady` | `r.Get("/ready", s.handleReady)` immediately after `/health`, outside gate branch | ✓ WIRED |
| `registerDataRoutes` | `handleStatus` | `r.Get("/status", s.handleStatus)` inside the gated group | ✓ WIRED (7 routes total) |
| `handleStatus` | `pollruns.Store` | `StatusStore.Snapshot()` | ✓ WIRED |
| `handleStatus` / `handleReady` | `schema_migrations` | `SchemaVersioner` seam → `db.SchemaVersion` (shared helper, D-15) | ✓ WIRED |
| `Dockerfile -ldflags -X` | `internal/buildinfo.Version` | fully-qualified module path; `cmd/server` imports the package | ✓ WIRED |

### Data-Flow Trace

| Value | Source | Real data? | Status |
|-------|--------|-----------|--------|
| `/status` `watchlist_size` | `CountWatchlist` `SELECT count(*) FROM watchlist` on shared pool | yes — `TestCountWatchlist_Integration` tracks row-for-row incl. after insert | ✓ FLOWING |
| `/status` `instance.schema_applied` | live `db.SchemaVersion` read on shared pool | yes — `TestBootToStatus_EndToEnd` asserts real migrated value (7) | ✓ FLOWING |
| `/status` `poll_interval_seconds` | `int(cfg.PollInterval / time.Second)` | yes — from config | ✓ FLOWING |
| `/status` `sources.*` runs/skips | `pollruns.Store.Snapshot()` | store is real + wired; **no production caller records yet (Phase 18.1)** — by design, fresh-instance empty state is the frozen contract | ✓ FLOWING (empty by design this phase) |
| `/status` `instance.app_version` | `buildinfo.Short()` | `dev` locally (proven); CI-injected SHA proven via `go test -ldflags` + `docker build` in plan verify | ✓ FLOWING (CI runtime → human check) |
| `/ready` schema integers | `db.ExpectedSchemaVersion()` + live `db.SchemaVersion` | yes — `TestBootToReady_EndToEnd` | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build | `go build ./...` | exit 0 | ✓ PASS |
| Vet | `go vet ./...` | exit 0 | ✓ PASS |
| Phase package tests | `go test ./internal/httpserver ./internal/pollruns ./internal/poller ./internal/buildinfo ./internal/db -count=1` | all `ok` | ✓ PASS |
| Readiness/status/store tests (verbose) | `-run 'TestReady|TestStatus|TestHealth|TestStore_|TestRunRecorder|TestWithRunRecorder'` | 40 PASS, 0 FAIL, 2 SKIP (`TestHealth_Up`, `TestHealth_Concurrent` — pre-existing, DB-gated, unchanged) | ✓ PASS |
| Integration (real Postgres) | `TEST_DATABASE_URL=... go test -run 'TestBootToReady_EndToEnd|TestBootToStatus_EndToEnd|TestSchemaVersion_Integration|TestSchemaVersionReader_Delegates|TestCountWatchlist_Integration|TestExpectedSchemaVersion'` | 6 PASS, 0 SKIP | ✓ PASS |
| sqlc drift | `make sqlc-check` | no diff, exit 0 | ✓ PASS |
| Scope fence | `awk runCycle-body | grep -c p.runs` → 0; `EventRecorder` return still `error`; `git status internal/detection` empty | — | ✓ PASS |
| Debt-marker scan | grep `TODO|FIXME|XXX|HACK|PLACEHOLDER` over phase source | no matches | ✓ PASS |

*`go test -race` unavailable on this box (standing STATE.md blocker); plain `go test` used per the accepted substitute. `golangci-lint` not re-run by the verifier (slow full-repo pre-commit hook); the four SUMMARYs consistently report it clean and `go vet` is green.*

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|----------------|-------------|--------|----------|
| RDY-01 | 18-01 | `/ready` 200/503 with schema check, no secrets in body | ✓ SATISFIED | Truth 1; `TestReady_*` + `TestBootToReady_EndToEnd` |
| RDY-02 | 18-01 | `/ready` unauth both modes, bounded check, shared pool, no side effects | ✓ SATISFIED | `TestReady_GatedNo401/InertBranch/Timeout`; `ready.go` opens no DB handle (grep gate) |
| RDY-03 | 18-01 | `/health` contract unchanged | ✓ SATISFIED | Truth 2; `health*.go` absent from diff |
| RUN-02 (skip signal only) | 18-02, 18-04 | Skipped cycle recorded as per-source signal, surfaced in `/status`, not a run entry | ✓ SATISFIED (Phase 18 half) | `TestStore_Skip/SkipIsPerSource`, `TestStatus_SkipSignal`. The `cancelled`-outcome-on-shutdown half is explicitly Phase 18.1; the traceability row records the split (`Phase 18 (skip signal) + Phase 18.1 (cancelled entry)`). |
| RUN-04 | 18-02, 18-04 | History bounded to N=50 per source, correct under two-source concurrency | ✓ SATISFIED | Truth 5; `TestStore_RingBound/TwoSourceConcurrent` |
| STAT-01 | 18-01, 18-03, 18-04 | Gated `/status` with runs/history/watchlist size/poll interval/instance | ✓ SATISFIED | Truth 3; Truth 4 (app_version); `TestStatus_*` + `TestBootToStatus_EndToEnd` |
| STAT-02 | 18-04 | `/status` exposes counts/timestamps/enums only, never secrets | ✓ SATISFIED | `TestStatus_NoLeak/EmptyErrorMessage`; no free-text error field (grep gate) |

No orphaned requirements: every ID mapped to Phase 18 in `REQUIREMENTS.md` is claimed by a plan and verified.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/httpserver/status.go` | ~92 | `handleStatus` runs `CountWatchlist` and `SchemaVersion` against `r.Context()` with **no** `context.WithTimeout`, unlike `/health` and `/ready` which both bound their DB calls | ⚠️ Warning | Pre-existing review finding WR-01, not yet addressed. `/status` is the endpoint Phase 19 polls; a hung Postgres path parks the handler goroutine until the driver returns (`WriteTimeout` fails the write but does not cancel the call). Not a phase-goal failure (no success criterion requires it; `/status` is gated), but worth closing before Phase 19 wires polling. |
| `internal/httpserver/status.go` | ~101-123 | `instance.schema_expected` emitted unconditionally even when `s.schema == nil`; a binary wiring `WithStatus` without `WithReadiness` returns a plausible `{"schema_applied":null,"schema_expected":0}` | ℹ️ Info | Review finding IN-01. `cmd/server` always wires both, so unreachable in production. |
| `internal/httpserver/status_test.go` | ~333 | `cycleID` test helper produces non-ASCII runes past index 25 | ℹ️ Info | Review finding IN-02. Test-only; assertion is on `len`, so it passes. |
| `.planning/REQUIREMENTS.md` | 23, 84 | RUN-02 marked `[x]` / "Complete" while its `cancelled`-outcome-on-shutdown clause is Phase 18.1 (Pending) | ℹ️ Info | Documentation inaccuracy only. The traceability row *does* annotate the split; the Phase 18 deliverable (skip signal) is complete. Consider "Partial" until 18.1 lands. |

### Human Verification Required

1. **CI provenance step, first `main` push after merge**
   - **Test:** Open the CI run; inspect the `build-scan` job.
   - **Expected:** "Verify build provenance landed in the binary" step passes; `release` job shows `docker load` of the scanned tarball and no `docker build`.
   - **Why human:** Only observable against the real GitHub Actions runner.

2. **SPA `/status` round-trip against a real instance**
   - **Test:** `docker compose up --build` with `INSTANCE_PASSPHRASE` set; log in via the SPA, `GET /status` in-session; `GET /status` in a private window.
   - **Expected:** In-session body matches `docs/api/status-contract.md` field for field (`sources` = musicbrainz+deezer, `last_run` null, `history` `[]`, `poll_interval_seconds` = configured, `watchlist_size` = watchlist view count, `app_version` = `dev`). Private window → `401`.
   - **Why human:** SPA cookie/session round-trip in a browser; `boot_e2e` covers the DB wiring but not the login path.

### Gaps Summary

No gaps. All five ROADMAP success criteria are verified in the codebase with passing unit
and real-Postgres integration tests, the scope fence (no `runCycle` / `internal/detection` /
`EventRecorder` change, no second image build, `/health` untouched) holds, and the `/status`
contract is frozen and drift-gated against the handler. Status is `human_needed` solely
because two end-to-end observations — the CI pipeline runner and the SPA login round-trip —
require a human and were pre-identified as manual in plans 18-03 and 18-04. Three
pre-existing review findings (WR-01 `/status` timeout, IN-01, IN-02) and one REQUIREMENTS.md
status-label inaccuracy are recorded above as non-blocking follow-ups.

---

_Verified: 2026-09-09_
_Verifier: Claude (gsd-verifier)_
