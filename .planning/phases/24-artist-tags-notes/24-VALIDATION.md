---
phase: "24"
slug: "artist-tags-notes"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: "2026-09-22"
---

# Phase 24 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + real-Postgres integration (`internal/testutil.NewTestPool`); Vitest 4.x + React Testing Library (jsdom) |
| **Config file** | none for Go (Makefile targets); `web/vitest.config.ts` |
| **Quick run command** | `go test ./internal/tags/... ./internal/watchlist/... ./internal/httpserver/... -short` |
| **Full suite command** | `make db-up && make test && make coverage-gate && make sqlc-check`; `corepack pnpm --dir web test` |
| **Estimated runtime** | ~90 seconds |

---

## Sampling Rate

- **After every task commit:** Run the quick command (or the touched Vitest file)
- **After every plan wave:** Run the full suite
- **Before `/gsd-verify-work`:** Full suite must be green, plus `make sqlc-check` (no CI counterpart)
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

Seeded from 24-RESEARCH.md §Validation Architecture; the planner assigns task IDs.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | TAG-01 | — | N/A | integration | `go test ./internal/tags/... -run TestService_Attach` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-01 | XSS | tag text rendered as plain JSX | component | `corepack pnpm --dir web test -- TagChips` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-02 | — | N/A | integration | `go test ./internal/tags/... -run TestService_Detach` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-03 | — | N/A | integration | `go test ./internal/tags/... -run TestService_Identity` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-04 | Tampering | DB refuses 11th tag / >32 chars when API bypassed | integration | `go test ./internal/httpserver/... -run TestTags_Attach_ConcurrentCapRace -count=25` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-04 | — | N/A | integration | `go test ./internal/tags/... -run TestTrigger_SkipExisting` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-05 | — | N/A | integration | `go test ./internal/tags/... -run 'TestService_Rename\|TestService_Merge_AtCap'` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-06 | — | N/A | integration | `go test ./internal/tags/... -run TestService_Delete` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | TAG-07 | — | N/A | integration | `go test ./internal/watchlist/... -run TestService_Remove_LeavesArtistTagsIntact` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | NOTE-01 | XSS | note rendered as plain JSX | integration | `go test ./internal/watchlist/... -run TestService_Note` | ❌ W0 | ⬜ pending |
| TBD | TBD | TBD | all routes | EoP / CSRF | 401 without session, 403 without CSRF header | integration | `go test ./internal/httpserver/... -run TestTags_` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/tags/service_test.go` — TAG-01…07 service behavior, ADR 0004 tests 2 and 3
- [ ] `internal/httpserver/tags_test.go` — HTTP mapping (400/404/409), auth/CSRF inheritance, ADR 0004 test 1 (concurrent cap race)
- [ ] `web/app/components/watchlist/TagChips.test.tsx`, `ArtistNote.test.tsx`, `ManageTagsDialog.test.tsx`, `web/app/components/common/ConfirmDialog.test.tsx`

*No framework installs needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Chip/× contrast and 24px hit areas | TAG-01/02 | Needs a rendered browser | Measure per 24-UI-SPEC Checker Sign-Off [R8] |
| Postgres collation folds `Ó`→`ó` | TAG-03 | Environment property (research A1) | `SHOW lc_collate; SHOW lc_ctype;` against dev and CI Postgres |
| Long-text/narrow-viewport layouts | TAG-01, TAG-05 | Visual (UI-SPEC 🧪 backstops) | 32-char tag at 375px; 30+ vocabulary popup; two 32-char names in merge confirm |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
