---
phase: 06
slug: frontend-release-history
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-10
mapped_to_plans: 2026-08-11
---

# Phase 06 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Backend: Go stdlib `testing` + `net/http/httptest` + `internal/testutil.NewTestPool` (real Postgres) — matches every prior phase. Frontend: none configured yet — greenfield (Wave 0 decision, see below). |
| **Config file** | Backend: none — see `Makefile` targets. Frontend: none yet. |
| **Quick run command** | `go test ./internal/httpserver/... -short -race -count=1` |
| **Full suite command** | `make test` (`test-integration` requires `make db-up` first — real Postgres) |
| **Estimated runtime** | ~30 seconds (quick, backend-only); full suite duration unverified this phase — mirrors Phase 05's ~2-3 minutes incl. Postgres integration |

---

## Sampling Rate

- **After every task commit:** `go test ./internal/httpserver/... -short -race -count=1` for backend changes; manual click-through in a running dev server (`react-router dev` + `go run ./cmd/server`) for frontend changes
- **After every plan wave:** `make test` (full backend suite against real Postgres)
- **Before `/gsd-verify-work`:** Full backend suite green + manual UAT walkthrough of all three ROADMAP success criteria
- **Max feedback latency:** 30 seconds (backend)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 06-01-T1 | 06-01 | 1 | — | T-06-SC | Package legitimacy gate clears all 16 npm packages before the first install; blocking human checkpoint, never auto-approvable | manual (blocking gate) | — | N/A — security checkpoint | ✅ green (approved by human pre-execution) |
| 06-01-T2 | 06-01 | 1 | HIST-01, UI-03 | T-06-01, T-06-02, T-06-04 | `GET /events` returns keyset-paginated (`id DESC`) results in the `{events, next_cursor}` shape; empty page encodes `[]` not `null`; a store error yields 500 with a fixed message and no raw DB text | integration (real Postgres) | `make db-up && TEST_DATABASE_URL=... go test ./internal/httpserver/ ./internal/events/ -run 'TestHandleListEvents\|TestListEvents' -count=1` | ✅ `internal/httpserver/events_test.go` | ✅ green (re-ran this session against real Postgres, all PASS) |
| 06-01-T2 | 06-01 | 1 | UI-03 | T-06-03, T-06-05 | The embedded SPA is served from the Go binary; an unregistered non-asset path falls back to `index.html`; `/health` and `/events` still reach their own handlers | integration (httptest) | same command, `-run TestSPA` | ✅ `internal/httpserver/spa_test.go` | ✅ green (re-ran this session, all PASS) |
| 06-01-T2 | 06-01 | 1 | UI-03 | — | React Router builds in SPA mode: `build/client` present, `build/server` absent | build gate | `cd web && pnpm run build && test -d build/client && test ! -d build/server` | ✅ verified | ✅ green (re-ran this session: `build/client` present, `build/server` absent) |
| 06-02-T1 | 06-02 | 2 | HIST-01 | T-06-06, T-06-07, T-06-08, T-06-10 | Invalid `limit`/`artist_id`/`cursor`/`event_type` rejected 400 in the `{"error": "..."}` shape with no raw DB text; an over-large `limit` is clamped server-side, not honoured | unit (stub store) | `go test ./internal/httpserver/ -run TestHandleListEvents_Validation -count=1` | ✅ verified | ✅ green (re-ran this session, all subtests PASS) |
| 06-02-T1 | 06-02 | 2 | HIST-01 | — | `ListEvents` excludes rows on the wrong side of the `id` cursor; `artist_id`/`event_type` filters apply independently and compose (both, either, neither) | integration (real Postgres) | `go test ./internal/httpserver/ -run TestListEvents_Filters -count=1` | ✅ verified | ✅ green (re-ran this session against real Postgres, all subtests PASS) |
| 06-02-T2 | 06-02 | 2 | UI-03 | T-06-09 | History cards render per-type detail; no raw-HTML injection anywhere under `web/app/`; type-check and build clean | build + source gate | `cd web && pnpm run build && pnpm exec tsc --noEmit` | ✅ verified (no frontend test runner this phase, by design — see Wave 0 Requirements) | ✅ green (re-ran this session: build clean, `tsc --noEmit` exit 0); visual card rendering confirmed by 06-VERIFICATION.md UAT step 5 |
| 06-03-T1 | 06-03 | 2 | UI-02 | T-06-12 | Watchlist list renders in the server's order with no client re-sort; all list states present; no raw-HTML injection | build + source gate | `cd web && pnpm run build && pnpm exec tsc --noEmit` | ✅ verified (no frontend test runner this phase) | ✅ green (build/tsc re-ran clean this session); live rendering confirmed by 06-VERIFICATION.md UAT approval |
| 06-03-T2 | 06-03 | 2 | UI-02 | T-06-13, T-06-14 | Preference toggle sends exactly one axis per PATCH and restores the prior value on failure; remove has no blocking dialog and an honestly-labelled Undo | build + source gate | `cd web && pnpm run build && pnpm exec tsc --noEmit` | ✅ verified (no frontend test runner this phase) | ✅ green (build/tsc re-ran clean this session); PATCH-failure rollback live-confirmed in 06-UAT.md (pass, 2026-08-11) closing the sole item 06-VERIFICATION.md had left as human-needed |
| 06-04-T1 | 06-04 | 3 | UI-01 | T-06-18, T-06-19, T-06-21, T-06-22 | Search is debounced and abortable; sources render as separate columns with no cross-source merge; 409 handled as backstop, not user-facing copy; no preference axis on the add body | build + source gate, plus full Go suite | `cd web && pnpm run build && pnpm exec tsc --noEmit` and `go test ./... -count=1` | ✅ verified (no frontend test runner this phase) | ✅ green (re-ran full frontend build/typecheck + `go test ./... -count=1` this session, all packages `ok`) |
| 06-04-T2 | 06-04 | 3 | UI-01, UI-02, UI-03 | — | Seven-step manual walkthrough of all three ROADMAP success criteria against the real `go:embed` binary | manual (UAT) | — | N/A — no frontend test framework this phase (see Wave 0 Requirements) | ✅ green (7-step UAT approved per 06-04-SUMMARY.md D11; 06-VERIFICATION.md scored 13/14 truths; remaining human-needed item closed pass in 06-UAT.md) |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Task IDs map to `06-{plan}-T{task}` in the four PLAN.md files created 2026-08-11.*

**`-race` note:** the commands above deliberately omit `-race`, which the Makefile's own `test-short`/`test-integration` targets do use. This dev box's mingw64 cgo toolchain cannot build race-instrumented binaries (documented in STATE.md from Phase 01-02/01-03, and worked around by running `-race` under WSL2). Race coverage stays at the wave-merge gate (`make test`) and in Phase 07's CI, not at the per-task gate where fast feedback matters more.

---

## Wave 0 Requirements

Both backend Wave 0 gaps are closed **inside the tracer task (06-01-T2)** rather than by a separate Wave 0 plan — the tracer's own `<verify>` needs them, so they cannot lag behind it:

- [x] `internal/httpserver/events_test.go` — created by 06-01-T2, mirroring `watchlist_test.go`'s stub-store + real-Postgres dual pattern (`stubEventsStore` with a func field per method, plus `var _ events.Store = stubEventsStore{}`). Extended by 06-02-T1 with `TestHandleListEvents_Validation` and `TestListEvents_Filters`.
- [x] `ListEvents` query-level coverage — **resolved location:** co-located in `internal/httpserver/events_test.go`, not a separate `internal/db` test file. This matches the existing convention: no query in `queries/events.sql` has a dedicated query-level test today; each is exercised through the handler or service that consumes it (`detector_test.go`, `deezer_test.go`, `watchlist_test.go`). Adding a lone query-test file for `ListEvents` would be the only one of its kind in the repo.
- [x] **Decision taken (planner discretion): no frontend test framework this phase.** Following RESEARCH.md's recommendation — standing up Vitest + `@testing-library/react` would be the single largest scope addition in the phase, and CLAUDE.md's testing philosophy is backend/pipeline-focused. The frontend half is gated instead by (a) `pnpm run build` plus `pnpm exec tsc --noEmit` on every frontend task, (b) source-level negative greps in each task's acceptance criteria (no raw-HTML injection, no cross-source merge, no client-side re-sort, no both-axes PATCH body, no blocking confirm dialog), and (c) the seven-step manual UAT at 06-04-T2.

  **The honest cost of that decision:** three `must_haves` items across the phase are marked `verification: backstop` precisely because no automated frontend assertion can confirm them — the loading skeletons, the long-text truncation behaviour, and the mid-flight PATCH-failure/concurrent-remove interaction. At verify time these abstain to human review rather than passing silently. If a later milestone adds a frontend test runner, those three are the first things to wire up.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full search → add → manage → view-history flow through the actual web UI | UI-01, UI-02, UI-03 | No frontend test framework exists yet; CLAUDE.md's testing convention targets backend/API correctness via `httptest.Server`-mocked unit tests, and this project's stated Core Value is CI/CD pipeline maturity, not UI test coverage. Standing up Playwright/Cypress for a 2-tab MVP frontend is disproportionate to phase scope. | Run the app locally (`react-router dev` + `go run ./cmd/server`, or the built `go:embed` binary), then: (1) search for an artist, add it, confirm it appears in the watchlist; (2) toggle a release-type/mute preference inline, confirm it persists on reload; (3) remove an artist; (4) open the History tab, confirm events render with type-specific detail (D-08), filter by artist and event type, and "Load more" pages correctly. |
| "Already watching" disabled state (D-11) | UI-01 | Depends on client-side timing (whether `GET /watchlist` has resolved before a search result renders) — a DOM/visual assertion, not a natural fit for the backend-only test suite in place this phase. | Add an artist, then search for it again; confirm the result shows "Already watching" (disabled) rather than an active add button. |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references
- [x] No watch-mode flags
- [x] Feedback latency < 30s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** plan-checker verified 2026-08-11 — all 5 sign-off criteria met at planning time; task IDs are still `06-TBD`→`06-{plan}-T{task}` pre-execution mappings, to be confirmed green during/after execution (final validate-phase pass still owns `status: validated`).

---

## Validation Audit 2026-08-11

Post-execution audit (State A: existing VALIDATION.md re-checked against SUMMARY/VERIFICATION/UAT artifacts and re-run this session).

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 (none needed — all 12 Per-Task rows already had automated or human-confirmed coverage) |
| Escalated | 0 |

Re-ran this session and confirmed green: `go build ./...`, `go vet ./...`, `go test ./internal/httpserver/... -count=1` (all phase-06 tests incl. real-Postgres `TestListEvents_*`/`TestHandleListEvents_*`/`TestSPA_*`), `go test ./... -count=1` (full suite, all packages `ok`), `cd web && pnpm run build` (`build/client` present, `build/server` absent), `pnpm exec tsc --noEmit` (exit 0). The sole item 06-VERIFICATION.md had left open (PATCH-failure preference rollback, human-needed) is confirmed closed `pass` in `06-UAT.md` (2026-08-11). Per-Task Verification Map statuses updated from `⬜ pending` to `✅ green` to reflect this; `status: validated` set in frontmatter.
