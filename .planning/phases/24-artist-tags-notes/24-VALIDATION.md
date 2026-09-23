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

Seeded from 24-RESEARCH.md §Validation Architecture; task IDs assigned by the planner (2026-09-23). DB-backed Go
commands need `TEST_DATABASE_URL` pointed at the `make db-up` fixture, or they skip — every plan's `<fails_when>` treats
`--- SKIP` as a failure. Single-file Vitest runs pass `--coverage.enabled=false` because the 70% threshold applies to the whole suite.

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 24-01-T1 | 24-01 | 1 | TAG-04, TAG-03, NOTE-01 | T-24-01 | DB refuses 11th link / >32 chars / untrimmed / case+accent dup / blank note with no API | integration (schema) | `go test ./internal/db/ -run 'TestSchema_\|TestTrigger_'` | ❌ W0 (created in task) | ⬜ pending |
| 24-01-T2 | 24-01 | 1 | TAG-01, TAG-03 | — | N/A | integration (HTTP e2e) + unit | `go test ./internal/httpserver/ -run TestTags_AttachEndToEnd`; `go test ./internal/tags/ -run TestNormalizeName` | ❌ W0 | ⬜ pending |
| 24-01-T3 | 24-01 | 1 | TAG-02, TAG-04, TAG-07 | T-24-02, T-24-03, T-24-04 | concurrent cap race; 401 without session; 403 without CSRF header | integration | `go test ./internal/tags/ ./internal/httpserver/ ./internal/watchlist/ -run 'TestService_Attach\|TestService_Detach\|TestService_Identity\|TestTags_\|TestService_Remove_LeavesArtistTagsIntact\|TestZipTags'` | ❌ W0 | ⬜ pending |
| 24-02-T1 | 24-02 | 2 | TAG-01 (vocabulary), TAG-06 | — | N/A | integration | `go test ./internal/tags/ ./internal/httpserver/ -run 'TestService_List\|TestTags_ListEndToEnd'` | ❌ W0 | ⬜ pending |
| 24-02-T2 | 24-02 | 2 | TAG-05, TAG-06 | T-24-12 | 409 collision changes nothing | integration | `go test ./internal/tags/ ./internal/httpserver/ -run 'TestService_Rename\|TestService_Delete\|TestTags_Rename\|TestTags_Delete'` | ❌ W0 | ⬜ pending |
| 24-02-T3 | 24-02 | 2 | TAG-05 | T-24-10, T-24-11, T-24-14 | merge never inserts; vocabulary routes gated | integration | `go test ./internal/tags/ ./internal/httpserver/ -run 'TestService_Merge\|TestTags_Merge\|TestTags_Vocabulary_'` | ❌ W0 | ⬜ pending |
| 24-03-T1 | 24-03 | 3 | NOTE-01 | T-24-18, T-24-19, T-24-20 | note route gated; note text never logged | integration | `go test ./internal/watchlist/ ./internal/httpserver/ -run 'TestNormalizeNote\|TestService_Note\|TestWatchlist_Note'` | ❌ W0 | ⬜ pending |
| 24-03-T2 | 24-03 | 3 | NOTE-01, TAG-07 | T-24-22 | PATCH cannot set a note | integration | `go test ./internal/watchlist/ ./internal/httpserver/ -run 'TestService_Add_\|TestService_ProjectionParity\|TestWatchlist_Patch_RejectsNoteKey'` | ❌ W0 | ⬜ pending |
| 24-04-T1 | 24-04 | 2 | TAG-02, TAG-01 | T-24-24, T-24-25 | tag text rendered as plain JSX; CSRF header on DELETE | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/TagChips.test.tsx app/lib/api.test.ts --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-04-T2 | 24-04 | 2 | TAG-02 | — | N/A | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/TagChips.test.tsx app/routes/watchlist.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-05-T1 | 24-05 | 3 | TAG-01 | T-24-28, T-24-29 | CSRF header on attach; vendoring leaves package.json unchanged | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/TagCombobox.test.tsx app/routes/watchlist.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-05-T2 | 24-05 | 3 | TAG-03 | — | N/A | unit | `corepack pnpm --dir web exec vitest run app/lib/tags.test.ts --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-05-T3 | 24-05 | 3 | TAG-04 | — | N/A | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/TagChips.test.tsx app/components/watchlist/TagCombobox.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-06-T1 | 24-06 | 4 | TAG-06 | T-24-32 | CSRF header on delete | component | `corepack pnpm --dir web exec vitest run app/components/common/ConfirmDialog.test.tsx app/components/watchlist/ManageTagsDialog.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-06-T2 | 24-06 | 4 | TAG-05 | — | N/A | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/ManageTagsDialog.test.tsx app/lib/api.test.ts --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-06-T3 | 24-06 | 4 | TAG-05 | T-24-33 | merge only after confirmation | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/ManageTagsDialog.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-07-T1 | 24-07 | 5 | NOTE-01 | T-24-37, T-24-38 | note rendered as plain JSX; CSRF header on PUT | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/ArtistNote.test.tsx app/lib/api.test.ts --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-07-T2 | 24-07 | 5 | NOTE-01 | — | N/A | component | `corepack pnpm --dir web exec vitest run app/components/watchlist/ArtistNote.test.tsx app/routes/watchlist.test.tsx --coverage.enabled=false` | ❌ W0 | ⬜ pending |
| 24-07-T3 | 24-07 | 5 | all | T-24-39, T-24-40 | blast radius unchanged; embedded bundle rebuilt | full gate | `make test && make coverage-gate && make sqlc-check && corepack pnpm --dir web test` | ✅ | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `internal/db/tags_schema_test.go` — DB-only invariants, ADR 0004 test 2, collation check (24-01-T1)
- [ ] `internal/tags/service_test.go` — TAG-01…07 service behavior, ADR 0004 test 3 (24-01-T3, 24-02)
- [ ] `internal/httpserver/tags_test.go` — HTTP mapping (400/404/409), auth/CSRF inheritance, ADR 0004 test 1 (concurrent cap race) (24-01, 24-02)
- [ ] `web/app/components/watchlist/TagChips.test.tsx`, `TagCombobox.test.tsx`, `ArtistNote.test.tsx`, `ManageTagsDialog.test.tsx`, `web/app/components/common/ConfirmDialog.test.tsx`, `web/app/lib/tags.test.ts`

Each is created by the task that first needs it (tests-first inside the task), so no separate Wave 0 plan exists.

*No framework installs needed.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Chip/× contrast and 24px hit areas | TAG-01/02 | Needs a rendered browser | Measure per 24-UI-SPEC Checker Sign-Off [R8] |
| Postgres collation folds `Ó`→`ó` | TAG-03 | Now automated in 24-01-T1 (`TestSchema_TagNameUniqueLower` reports `datcollate` on failure); Postgres 16 removed `SHOW lc_collate` | `SELECT datcollate FROM pg_database WHERE datname = current_database()` if the test fails |
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
