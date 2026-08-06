---
phase: 02
slug: watchlist-core
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-05
---

# Phase 02 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`), no third-party test framework |
| **Config file** | `Makefile` targets — no separate test-config file |
| **Quick run command** | `go test ./... -short -race -count=1` (skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` check) |
| **Full suite command** | `make test` → `db-up` then `TEST_DATABASE_URL=... go test ./... -race -count=1` |
| **Estimated runtime** | ~30 seconds (short), ~90 seconds (full, includes DB migration + integration tests) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -short -race -count=1`
- **After every plan wave:** Run `make test` (full suite against real Postgres)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 02-01-T1 | 02-01 | 1 | WLST-02 | T-02-01, T-02-02, T-02-03 | Add path decodes into a DTO with `DisallowUnknownFields`; no raw SQL in the service; no driver error text in any response body | unit + integration | `go test ./internal/httpserver/ -run 'TestWatchlist_Add' -count=1` | ✅ internal/httpserver/watchlist.go, internal/httpserver/watchlist_test.go | ✅ pass |
| 02-02-T1 | 02-02 | 2 | WLST-02 | T-02-14 | Duplicate add returns 409 via SQLSTATE 23505 on `watchlist_artist_id_key`, never a 500 and never a silent preferences overwrite | unit + integration | `go test ./internal/watchlist/... ./internal/httpserver/... -run 'Duplicate' -count=1` | ✅ internal/watchlist/service.go, internal/watchlist/service_test.go | ✅ pass |
| 02-02-T2 | 02-02 | 2 | WLST-05, WLST-06 | T-02-04, T-02-05, T-02-13 | Body capped at 64 KiB, `mbid`/`name` rune-capped, out-of-allow-list preference values rejected 400 before any write | unit + integration | `go test ./internal/watchlist/... ./internal/httpserver/... -count=1` | ✅ internal/watchlist/normalize_test.go, internal/httpserver/watchlist.go | ✅ pass |
| 02-03-T1 | 02-03 | 3 | WLST-04 | T-02-08, T-02-09 | Joined list query aliases every column so `watchlist.id` and `artists.id` never collapse; empty result encodes as `[]` | integration | `go test ./internal/watchlist/... ./internal/httpserver/... -run 'List' -count=1` | ✅ queries/watchlist.sql, internal/watchlist/service.go | ✅ pass |
| 02-03-T2 | 02-03 | 3 | WLST-03 | T-02-07, T-02-15 | Path `{id}` parsed and bounds-checked before any query; removing a missing id returns 404, not a 500; concurrent deletes yield exactly one 204 and one 404 | unit + integration | `go test ./internal/httpserver/ -run 'TestWatchlist_Delete' -count=5` | ✅ internal/httpserver/watchlist.go, internal/httpserver/server.go | ✅ pass |
| 02-04-T1 | 02-04 | 4 | WLST-05, WLST-06 | T-02-10, T-02-11, T-02-16 | Invalid release-type or mute-event value rejected 400 with the stored row untouched; PATCH DTO carries only the two preference axes | unit + integration | `go test ./internal/watchlist/... ./internal/httpserver/... -count=1` | ✅ queries/watchlist.sql, internal/watchlist/service.go, internal/httpserver/watchlist.go | ✅ pass |
| 02-04-T2 | 02-04 | 4 | WLST-02..WLST-06 | T-02-10, T-02-12 | DB `CHECK` constraints reject out-of-allow-list values written by raw SQL, independent of the Go layer; full lifecycle demonstrated end to end | integration | `go test ./... -count=1` | ✅ internal/watchlist/service_test.go, internal/httpserver/watchlist_test.go | ✅ pass |

*`-race` is deliberately absent from these commands: this dev machine's mingw64 toolchain cannot execute `cc1.exe`, documented in STATE.md from phases 01-02/01-03. The `-race` pass runs under WSL2 or in the Phase 7 CI job, matching the Phase 1 precedent.*

---

## Wave 0 Requirements

- [x] `internal/db/migrations/000002_watchlist.up.sql` / `.down.sql` — new schema (artists, watchlist tables)
- [x] `queries/artists.sql`, `queries/watchlist.sql` — new sqlc query sources
- [x] `sqlc.yaml` edits (`emit_interface: true`, `emit_pointers_for_null_types: true`) + `sqlc generate` regeneration
- [x] `internal/watchlist/service.go` + `service_test.go` — new package, no existing tests to build on
- [x] `internal/httpserver/watchlist.go` + `watchlist_test.go` — new handlers, following `health.go`/`health_test.go` shape
- [x] Updates to the **8** existing `httpserver.New(...)` call sites for the new constructor parameter, all in plan 02-01 task 1's single commit. 02-RESEARCH.md Pitfall 5 says 5 and undercounts: the verified set is `cmd/server/main.go:80`, `internal/httpserver/health_test.go:57,86,126,151`, `internal/httpserver/server_test.go:83,203`, `internal/httpserver/boot_e2e_test.go:50`

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification (no UI, no external API calls this phase).*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 90s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** confirmed 2026-08-06 — all 7 task rows pass with real automated commands pointing at files that exist; `wave_0_complete: true` and `nyquist_compliant: true` set. `TestWatchlist_FullLifecycle` demonstrates WLST-02 through WLST-06 end to end and `go test ./... -count=1` is green across every package.
