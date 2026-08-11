---
phase: 06
slug: frontend-release-history
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-10
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
| 06-TBD-01 | TBD | TBD | HIST-01 | V5 | `GET /events` returns keyset-paginated (`id DESC`), artist/event-type-filtered results in the documented `{events, next_cursor}` shape | integration (real Postgres) | `go test ./internal/httpserver/... -run TestHandleListEvents -race -count=1` | ❌ W0 — new `internal/httpserver/events_test.go` | ⬜ pending |
| 06-TBD-02 | TBD | TBD | HIST-01 | — | `ListEvents` sqlc query excludes rows on the wrong side of the `id` cursor; `artist_id`/`event_type` filters apply independently and compose (both, either, neither) | integration (real Postgres) | `go test ./internal/db/... -run TestListEvents -race -count=1` | ❌ W0 | ⬜ pending |
| 06-TBD-03 | TBD | TBD | HIST-01 | V5 | Unbounded/invalid `limit`, non-numeric `artist_id`/`cursor`, and out-of-allow-list `event_type` are rejected with `errorResponse{Error}` (400), never echo raw DB error text | unit (httptest.Server) | `go test ./internal/httpserver/... -run TestHandleListEvents_Validation -race -count=1` | ❌ W0 | ⬜ pending |
| 06-TBD-04 | TBD | TBD | UI-01, UI-02, UI-03 | — | Search → add → manage (preferences, remove) → view history end-to-end via the web UI | manual (UAT) | — | N/A — no frontend test framework this phase (see Wave 0 Requirements) | ⬜ pending |
| 06-TBD-05 | TBD | TBD | UI-01 (D-11) | — | "Already watching" client-side cross-reference disables add on already-watchlisted search results | manual (UAT) | — | N/A | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky. Task IDs are `06-TBD-*` placeholders — the planner assigns real plan/task IDs; re-map this table against the actual PLAN.md files once planning completes (see Phase 05's precedent: this draft table is speculative and superseded by execution).*

---

## Wave 0 Requirements

- [ ] `internal/httpserver/events_test.go` — stubs for HIST-01 (new handler test file, mirrors `watchlist_test.go`'s stub-store + real-Postgres dual pattern)
- [ ] New sqlc query-level test for `ListEvents` — co-located per this repo's established convention (planner should confirm the right location: package-level test vs. a dedicated query test file, since existing `events.sql` queries don't have one separate from handler/service tests)
- [ ] **Decision needed (planner discretion):** whether to introduce a frontend test framework (Vitest + `@testing-library/react`, the standard Vite-ecosystem pairing) for D-11's "Already watching" logic and the optimistic-rollback logic (D-12), or accept manual-only UAT coverage for the entire frontend. **Recommendation (from RESEARCH.md):** skip a frontend test framework this phase — it would be the single largest scope addition in the phase, and CLAUDE.md's testing philosophy is backend/pipeline-focused. Rely on manual UAT for the frontend half.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Full search → add → manage → view-history flow through the actual web UI | UI-01, UI-02, UI-03 | No frontend test framework exists yet; CLAUDE.md's testing convention targets backend/API correctness via `httptest.Server`-mocked unit tests, and this project's stated Core Value is CI/CD pipeline maturity, not UI test coverage. Standing up Playwright/Cypress for a 2-tab MVP frontend is disproportionate to phase scope. | Run the app locally (`react-router dev` + `go run ./cmd/server`, or the built `go:embed` binary), then: (1) search for an artist, add it, confirm it appears in the watchlist; (2) toggle a release-type/mute preference inline, confirm it persists on reload; (3) remove an artist; (4) open the History tab, confirm events render with type-specific detail (D-08), filter by artist and event type, and "Load more" pages correctly. |
| "Already watching" disabled state (D-11) | UI-01 | Depends on client-side timing (whether `GET /watchlist` has resolved before a search result renders) — a DOM/visual assertion, not a natural fit for the backend-only test suite in place this phase. | Add an artist, then search for it again; confirm the result shows "Already watching" (disabled) rather than an active add button. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
