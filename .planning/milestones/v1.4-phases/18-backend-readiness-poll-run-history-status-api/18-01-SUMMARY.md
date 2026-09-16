---
phase: 18-backend-readiness-poll-run-history-status-api
plan: 01
subsystem: api
tags: [readiness-probe, healthcheck, chi, pgx, golang-migrate, schema-version, functional-options]

requires:
  - phase: 16-rollback-safe-migrations
    provides: ahead-of-source guard (maxSourceVersion walk, applied-can-exceed-embedded)
  - phase: 14-instance-passphrase-gate
    provides: gated-vs-inert route branch structure, structural /health exemption
provides:
  - "GET /ready unauthenticated readiness probe: 200 {status,schema_applied,schema_expected} healthy; 503 with reason db_unreachable|schema_behind|schema_dirty|not_configured otherwise"
  - "db.ExpectedSchemaVersion() — embedded migration max, drift-proof, computed once at boot"
  - "db.SchemaVersion(ctx, RowQuerier) + db.SchemaVersionReader/NewSchemaVersionReader — the one hand-rolled schema_migrations read, shared with plan 18-04's /status"
  - "httpserver.SchemaVersioner seam + httpserver.WithReadiness(schema, expected) functional option"
  - "readyCheckTimeout (3s) bounding the ping + schema read together"
affects: [18-04 (/status instance block reuses db.SchemaVersion + Server.schema/expectedSchema fields), 19 (SPA readiness badge parses the D-03 body), 17 (deferred deploy gate polls /ready)]

actuals:
  tokens: 6400
  tasks: 3
  commits: 4

tech-stack:
  added: []
  patterns:
    - "Consumer-declared narrow seam mirroring Pinger (SchemaVersioner in httpserver, RowQuerier in db)"
    - "Adapter type (SchemaVersionReader) bridging a package function to a parameterless seam method"
    - "Reason vocabulary held as one shared const (reasonDBUnreachable) so two failure branches cannot drift"

key-files:
  created:
    - internal/db/schema_version.go
    - internal/db/schema_version_test.go
    - internal/httpserver/ready.go
    - internal/httpserver/ready_test.go
  modified:
    - internal/db/migrate.go
    - internal/httpserver/server.go
    - internal/httpserver/boot_e2e_test.go
    - cmd/server/main.go

key-decisions:
  - "Ready condition written as !dirty && applied >= expected (never ==) per D-01; dirty checked before the version comparison so a dirty-and-behind DB reports schema_dirty"
  - "A fourth reason value not_configured is emitted when WithReadiness is absent, purely for route-table parity; a cmd/server binary wires the option unconditionally so it is unreachable in production (planner deviation, pinned by TestReady_NotConfigured)"
  - "readyCheckTimeout is its own 3s const equal to healthPingTimeout but separate, so retuning readiness never retunes liveness (RDY-03)"
  - "db.SchemaVersion maps pgx.ErrNoRows to (0,false,nil); a non-negative-version guard was added (Rule 2) to satisfy gosec G115 on the int64->uint conversion"

patterns-established:
  - "Structural route exemption for /ready: registered on the root router immediately after /health, outside the gate branch — one registration serves both branches, never a path-string allowlist"
  - "Golden byte-for-byte pin of an unchanged contract (/health) added in the new test file rather than by editing the frozen health_test.go"

requirements-completed: [RDY-01, RDY-02, RDY-03]

coverage:
  - id: D1
    description: "GET /ready returns 200 with schema_applied/schema_expected on a healthy instance and 503 with exactly one reason (db_unreachable|schema_behind|schema_dirty|not_configured) otherwise; ahead-of-source binary is ready; no 503 body carries a DSN, webhook URL, or raw error text"
    requirement: "RDY-01"
    verification:
      - kind: unit
        ref: "internal/httpserver/ready_test.go#TestReady_Healthy,TestReady_AheadOfSource,TestReady_DBUnreachable,TestReady_SchemaReadError,TestReady_SchemaBehind,TestReady_SchemaDirty,TestReady_EmptySchemaMigrations,TestReady_NotConfigured,TestReady_NoLeak"
        status: pass
      - kind: integration
        ref: "internal/httpserver/boot_e2e_test.go#TestBootToReady_EndToEnd (real migrated Postgres)"
        status: pass
    human_judgment: false
  - id: D2
    description: "db.ExpectedSchemaVersion reads the embedded migration max (7); db.SchemaVersion + SchemaVersionReader read the real schema_migrations row"
    requirement: "RDY-01"
    verification:
      - kind: unit
        ref: "internal/db/schema_version_test.go#TestExpectedSchemaVersion"
        status: pass
      - kind: integration
        ref: "internal/db/schema_version_test.go#TestSchemaVersion_Integration,TestSchemaVersionReader_Delegates"
        status: pass
    human_judgment: false
  - id: D3
    description: "/ready is non-401 on a passphrase-gated instance with no cookie and on an inert instance, bounded under a hung ping (503 db_unreachable), exact-path-only, opens no database handle of its own; /health response bytes unchanged"
    requirement: "RDY-02"
    verification:
      - kind: unit
        ref: "internal/httpserver/ready_test.go#TestReady_GatedNo401,TestReady_InertBranch,TestReady_Timeout,TestReady_ExactPathOnly,TestHealth_ContractUnchanged"
        status: pass
      - kind: other
        ref: "grep -E 'sql.Open|pgxpool.New|migrate.New' internal/httpserver/ready.go -> empty"
        status: pass
      - kind: other
        ref: "git diff --exit-code 9e764d5 HEAD -- internal/httpserver/health_test.go -> clean (RDY-03)"
        status: pass
    human_judgment: false

duration: 35min
completed: 2026-09-09
status: complete
---

# Phase 18 Plan 01: Backend — GET /ready Readiness Probe Summary

**`GET /ready` — an unauthenticated, 3s-bounded readiness probe that tells a live drop-tracker apart from a merely-running process via a machine reason enum, added beside `/health` without changing liveness in any observable way.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-09-09T19:35:00Z
- **Completed:** 2026-09-09T20:09:00Z
- **Tasks:** 3 (one tracer, one TDD, one auto)
- **Files modified:** 8 (4 created, 4 modified)

## Accomplishments

- `GET /ready` ships: `200 {"status":"ready","schema_applied":N,"schema_expected":N}` when the DB is reachable and the applied schema is at-or-above the binary's expected version and not dirty; `503` with exactly one of `db_unreachable`, `schema_behind`, `schema_dirty`, `not_configured` otherwise. No response body on any path carries a DSN, webhook URL, driver text, or raw error string — the raw cause reaches `httplog.SetAttrs` under `ready_db_error` / `ready_schema_error` only.
- `db.ExpectedSchemaVersion()` reads the embedded migration source (returns 7 today) so the expected number can never drift from what `RunMigrations` would apply. `cmd/server` computes it once at boot.
- `db.SchemaVersion` + `db.SchemaVersionReader` are the single hand-rolled `SELECT version, dirty FROM schema_migrations` read; plan 18-04's `/status` instance block will call the same helper (D-15).
- `/ready` inherits `/health`'s exact structural exemption — registered on the root router immediately after `/health`, outside the gate branch — so it is never `401` on a gated instance and one registration serves both branches.
- `/health` is untouched: `health_test.go` is byte-for-byte unchanged and still passes, and a new golden test pins its exact response bytes for both the up and down cases.

## Task Commits

1. **Task 1: End-to-end healthy path (tracer)** — `52635f9` (feat)
2. **Task 2: Every not-ready branch, leak-free (TDD)** — `d831afa` (test, RED) → `0dac74c` (feat, GREEN)
3. **Task 3: Registration, bound, liveness contract, db helpers** — `f009dc7` (test)

**Plan metadata:** _this commit_ (docs: complete plan)

## Files Created/Modified

- `internal/db/schema_version.go` — `RowQuerier` seam, `SchemaVersion(ctx, RowQuerier)`, `SchemaVersionReader` + `NewSchemaVersionReader`
- `internal/db/schema_version_test.go` — `TestExpectedSchemaVersion` (drift alarm), `TestSchemaVersion_Integration`, `TestSchemaVersionReader_Delegates`
- `internal/httpserver/ready.go` — `SchemaVersioner` seam, `readyCheckTimeout`, `readyResponse`, `handleReady`
- `internal/httpserver/ready_test.go` — 13 tests + a shared `fakeSchemaVersioner` double
- `internal/db/migrate.go` — `+ ExpectedSchemaVersion()`
- `internal/httpserver/server.go` — `Server.schema`/`expectedSchema` fields, `serverConfig` additions, `WithReadiness` option, `/ready` root-router registration
- `internal/httpserver/boot_e2e_test.go` — `+ TestBootToReady_EndToEnd`
- `cmd/server/main.go` — compute `expectedSchema` after migrations, append `WithReadiness(db.NewSchemaVersionReader(pool), expectedSchema)`

## Decisions Made

- Ready condition is `!dirty && applied >= expected`, never `==` (D-01) — a cleanly rolled-back binary serving a newer additive schema is ready (Phase 16 ahead-of-source guard). Dirty is checked before the version comparison.
- `not_configured` fourth reason kept for byte-identical route-table behaviour with and without `WithReadiness`; unreachable from a `cmd/server` binary and pinned by a test so the distinction stays deliberate.
- `readyCheckTimeout` is a separate `const` deliberately equal to `healthPingTimeout` (RDY-03).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing Critical] Negative-version guard in `db.SchemaVersion`**
- **Found during:** Task 1 (golangci-lint run)
- **Issue:** `uint(v)` on the `int64` scanned from `schema_migrations` tripped gosec G115 (integer-overflow conversion); the RESEARCH worked example had the same unguarded cast.
- **Fix:** Added an explicit `if v < 0` guard returning a wrapped error before the conversion — golang-migrate only ever writes a non-negative version, so a negative value means out-of-band tampering.
- **Files modified:** `internal/db/schema_version.go`
- **Verification:** `golangci-lint run` clean; `TestSchemaVersion_Integration` still passes.
- **Committed in:** `52635f9` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 missing-critical / lint-driven)
**Impact on plan:** Necessary for the Definition-of-Done lint gate. No scope creep.

## Issues Encountered

- **`make test` carries `-race`, which is unusable on this dev box** (ThreadSanitizer allocation failure — a documented Phase 18 blocker in STATE.md, and `-race` is absent from CI). Substituted a plain `go test ./... -count=1 -coverprofile=... -coverpkg=...` run for the DoD suite gate, then ran `make coverage-gate` unchanged. Full suite green; backend coverage **90.35%** (floor 80%). This matches the substitution precedent set in Phases 11.1 and 15.
- The pre-commit `golangci-lint` hook lints the whole repo and takes longer than a 2-minute command timeout; commits were run with an extended timeout. No hook was bypassed (`--no-verify` never used).

## Definition of Done

- `go vet ./...` — clean
- `golangci-lint run` — 0 issues
- Full `go test ./...` (race substituted) — all packages ok, no FAIL
- `make coverage-gate` — 90.35% (required 80%) PASS
- `make sqlc-check` — not applicable (no `queries/` change this plan)
- `git diff --exit-code -- internal/httpserver/health_test.go` — clean

## Self-Check: PASSED

- Created files present: `internal/db/schema_version.go`, `internal/db/schema_version_test.go`, `internal/httpserver/ready.go`, `internal/httpserver/ready_test.go` — all found
- Commits present: `52635f9`, `d831afa`, `0dac74c`, `f009dc7` — all in `git log`
- `internal/httpserver/health_test.go` unchanged since `9e764d5`

## Next Phase Readiness

- Plan 18-04 (`/status`) can reuse `db.SchemaVersion` and the `Server.schema` / `Server.expectedSchema` fields for its `instance` block directly — the seam and the boot wiring are in place.
- Plan 19 (SPA) can parse the frozen D-03 `/ready` body: `status`, `schema_applied` (int or null), `schema_expected` (int), optional `reason`.
- No blockers introduced. The `-race` limitation continues to apply to plan 18-02's `pollruns` concurrency work (its looped invariant test is the stated substitute).

---
*Phase: 18-backend-readiness-poll-run-history-status-api*
*Completed: 2026-09-09*
