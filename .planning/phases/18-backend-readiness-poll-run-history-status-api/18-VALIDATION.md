---
phase: "18"
slug: "backend-readiness-poll-run-history-status-api"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-09"
---

# Phase 18 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Seeded from `18-RESEARCH.md` → "Validation Architecture". Planner fleshes out the
> Per-Task Verification Map; validate-phase signs off.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (table-driven, `t.Fatalf`; no testify in `internal/httpserver` / `internal/poller`) |
| **Config file** | none — `go test` |
| **Quick run command** | `go test ./internal/httpserver/ ./internal/pollruns/ ./internal/db/ ./internal/poller/ ./internal/buildinfo/ -count=1` |
| **Full suite command** | `make db-up && make test && make coverage-gate && make sqlc-check` |
| **Estimated runtime** | ~60–90 seconds (quick); ~3–5 min (full incl. integration) |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/httpserver/ ./internal/pollruns/ ./internal/db/ ./internal/poller/ ./internal/buildinfo/ -count=1` + `go vet ./...` + `golangci-lint run`
- **After every plan wave:** `make db-up && make test && make coverage-gate && make sqlc-check`
- **Before `/gsd-verify-work`:** Full suite green + `make sqlc-check` clean + manual app-version E2E on a locally-built image
- **Max feedback latency:** ~90 seconds (quick subset)

*`corepack pnpm` frontend gates are NOT applicable — this phase touches no `web/` files.*

---

## Per-Task Verification Map

*Populated by the planner. Requirement → test mapping from 18-RESEARCH.md "Phase Requirements → Test Map":*

| Requirement | Behavior | Test Type | Automated Command | Wave 0 File |
|-------------|----------|-----------|-------------------|-------------|
| RDY-01 | `applied >= expected && !dirty` → 200; `<` → 503 `schema_behind`; dirty → 503 `schema_dirty`; ping fail → 503 `db_unreachable` | unit | `go test ./internal/httpserver/ -run TestReady` | `internal/httpserver/ready_test.go` |
| RDY-01 | rollback: `applied = expected + 1` → 200 `ready` (`>=` not `==`) | unit | `go test ./internal/httpserver/ -run TestReady_AheadOfSource` | ready_test.go |
| RDY-01 | 503 body carries no DSN / driver text / path | unit (golden) | `go test ./internal/httpserver/ -run TestReady_NoLeak` | ready_test.go |
| RDY-01 | `db.ExpectedSchemaVersion()` returns embedded max (7) | unit (embedded FS, no DB) | `go test ./internal/db/ -run TestExpectedSchemaVersion` | `internal/db/schema_version_test.go` |
| RDY-01 | `db.SchemaVersion` reads real `schema_migrations` (version + dirty) | integration (`make db-up`) | `go test ./internal/db/ -run TestSchemaVersion_Integration` | schema_version_test.go |
| RDY-02 | `/ready` non-401 on a `WithAuthGate` server with no cookie | unit | `go test ./internal/httpserver/ -run TestReady_GatedNo401` | ready_test.go |
| RDY-02 | `/ready` reachable in the inert branch too | unit | `go test ./internal/httpserver/ -run TestReady_InertBranch` | ready_test.go |
| RDY-02 | DB check bounded — blocked ping fails within ~3s, no hang | unit | `go test ./internal/httpserver/ -run TestReady_Timeout` | ready_test.go |
| RDY-02 | shared pool, no new conn, no side effects | manual: `grep -n 'sql.Open\|pgxpool.New\|migrate.New' internal/httpserver/ready.go` empty | — |
| RDY-03 | `/health` body/status/timeout unchanged | unit (existing `health_test.go` passes byte-for-byte) | `go test ./internal/httpserver/ -run TestHealth` | ✅ exists |
| RUN-02 (skip) | `RecordSkip` bumps `consecutive_skips` + stamps `last_skipped_at`; `RecordRun` resets to 0 | unit | `go test ./internal/pollruns/ -run TestStore_Skip` | `internal/pollruns/pollruns_test.go` |
| RUN-02 (skip) | skip signal surfaces in `/status` | unit (fake `StatusStore`) | `go test ./internal/httpserver/ -run TestStatus_SkipSignal` | `internal/httpserver/status_test.go` |
| RUN-04 | history bounded to N=50 per source, newest-first | unit | `go test ./internal/pollruns/ -run TestStore_RingBound` | pollruns_test.go |
| RUN-04 | correct under two sources near-simultaneously | unit (looped ~1000×, two goroutines) — the `-race` substitute | `go test ./internal/pollruns/ -run TestStore_TwoSourceConcurrent -count=1` | pollruns_test.go |
| RUN-04 | `poller.RunRecorder` seam exists, wired, inert (no `runCycle` call) | unit + `grep -n 'p.runs' internal/poller/poller.go` empty | `go test ./internal/poller/ -run TestRunRecorder_Wired` | poller_test.go (add `fakeRunRecorder`) |
| STAT-01 | gated `GET /status` returns runs/history/watchlist/interval/`instance` | unit (httptest + fakes) | `go test ./internal/httpserver/ -run TestStatus_Shape` | status_test.go |
| STAT-01 | `401` without a session | unit | `go test ./internal/httpserver/ -run TestStatus_Gated401` | status_test.go |
| STAT-01 | empty-history instance → 200 (not error) | unit (fresh `pollruns.NewStore()`) | `go test ./internal/httpserver/ -run TestStatus_EmptyHistory` | status_test.go |
| STAT-01 | `watchlist_size` reflects `CountWatchlist` | integration + unit wiring | `go test ./internal/db/ -run TestCountWatchlist_Integration` | schema_version_test.go / watchlist test |
| STAT-01 | `poll_interval_seconds` renders `cfg.PollInterval` as int (900) | unit | `go test ./internal/httpserver/ -run TestStatus_PollInterval` | status_test.go |
| STAT-02 | no DSN/webhook/path/driver text on any `/status` path — golden | unit | `go test ./internal/httpserver/ -run TestStatus_NoLeak` | status_test.go |
| STAT-02 | `RunResult` has no free-text error field | compile-time + review (`grep 'Error\|Detail\|Message'` on struct) | — |
| app version | `buildinfo.Version` defaults to `"dev"` | unit | `go test ./internal/buildinfo/` | `internal/buildinfo/buildinfo_test.go` |
| app version | injected `-ldflags -X` reaches `/status` `instance.app_version` | manual / recommended CI smoke | build image `--build-arg VERSION=deadbeef`, run, `curl -s :8080/status \| jq -r .instance.app_version` != `"dev"` | — |
| app version | `release` job pushes byte-identical scanned image | code review — `full-pipeline.yml` `release` job untouched | — |

*Status legend: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/httpserver/ready_test.go` — RDY-01, RDY-02 (gated-no-401, timeout, ahead-of-source, no-leak)
- [ ] `internal/httpserver/status_test.go` — STAT-01, STAT-02, RUN-02 skip-signal surfacing
- [ ] `internal/pollruns/pollruns_test.go` — RUN-04 (ring bound, two-source concurrent looped), RUN-02 (skip counter + reset)
- [ ] `internal/db/schema_version_test.go` — `ExpectedSchemaVersion` (unit) + `SchemaVersion` (integration)
- [ ] `internal/db/` — `CountWatchlist` integration test (may join an existing watchlist-query test file)
- [ ] `internal/poller/poller_test.go` — add `fakeRunRecorder` (mirror `fakeEventRecorder`/`fakeNotifier`) + wiring/inert test
- [ ] `internal/buildinfo/buildinfo_test.go` — default `"dev"`
- [ ] Fakes: `fakeSchemaVersioner`, `fakeStatusStore`, `fakeWatchlistCounter` in `internal/httpserver` (mirror `stubPinger`, `stubEventsStore`)
- [ ] Recommended: CI smoke assertion that a `build-scan`-built image reports non-`"dev"` `app_version`

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| `-ldflags -X` reaches `instance.app_version` end-to-end | app version | Needs a real Docker image build + run; no unit can prove the CI build-arg → binary path | `docker build --build-arg VERSION=deadbeefcafe -t dt:vtest .` → `docker run --rm -e … dt:vtest` → `curl -s localhost:8080/status \| jq -r .instance.app_version` must equal `deadbeefcafe` (or its 12-char short form), not `"dev"` |
| `release` job pushes the byte-identical scanned image | app version | Pipeline-structure invariant, not runtime behavior | Inspect the `.github/workflows/full-pipeline.yml` diff — the `release` job must be untouched; confirm it `docker load`s the scanned tar and only `tag`+`push` |
| `/ready` non-side-effecting / shared-pool | RDY-02 | Absence-of-code assertion | `grep -n 'sql.Open\|pgxpool.New\|migrate.New' internal/httpserver/ready.go` → empty |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
