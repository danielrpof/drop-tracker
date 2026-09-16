---
phase: 19-frontend-system-view
verified: 2026-09-10T22:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:

  - test: "On a running gated instance with a real Postgres database, open the System tab and confirm both source panels render with real run data, the About block shows real version/schema/database-pill/watchlist/poll-interval values, clicking Refresh updates the freshness stamp while existing content stays on screen, and logging out mid-view yields to the passphrase screen with no error flash."
    expected: "Everything renders correctly against a live backend and live browser DOM/CSS, matching what jsdom-based unit tests can only approximate."
    why_human: "This is a live-browser, live-database check explicitly named by plan 19-05 Task 3's <human-check> block; jsdom cannot render real layout/CSS or exercise a real Postgres-backed gated instance. Carried forward from the SUMMARY as the phase's outstanding UAT item (same pattern as Phase 18.1's live-instance status check)."
---

# Phase 19: Frontend — System View Verification Report

**Phase Goal:** An operator opens the app and can see, on one screen, whether the scheduler is doing its job — per source, right now and over the last N cycles.
**Verified:** 2026-09-10
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | System tab in nav opens a System view showing per source: last run's time, outcome, duration, counts, plus time since last successful run | ✓ VERIFIED | `web/app/root.tsx:118-120` NavLink to `/system`; `web/app/routes.ts:11` route entry; `SourcePanel.tsx` renders `OutcomeBadge`, `formatAbsoluteTime(last_run.finished_at)`, `formatDuration(last_run.duration_ms)`, counts line, and the D-08 clean-run scan (`findCleanRun`) rendering `formatRelativeTime`. Tests: `system.test.tsx` lines 184-243 (healthy run + no-clean-run cases), all passing. |
| 2 | Same view shows recent-runs history table, watchlist size, poll interval, and an about block (app version, schema version, database reachable) | ✓ VERIFIED | `SourceHistoryTable.tsx` (8-column table per source); `AboutInstance.tsx` renders Version/Schema/Database/Watchlist/Poll-interval rows keyed off `instance.schema_applied`/`schema_expected`/`watchlist_size`/`poll_interval_seconds`. Tests: `system.test.tsx` lines 113-181 (schema equal/drift/null cases), 419-501 (cap/singular/zero-entry history cases), all passing. |
| 3 | Freshly deployed/migrated instance with no poll history shows explicit first-run copy, distinct from error and loaded-empty states; no endless spinner, no blank card, no "Invalid Date" | ✓ VERIFIED | `deriveLoadedShape()` in `system.tsx:43-51` requires all three clauses (null last_run, empty history, zero consecutive_skips) across every source before rendering the first-run `EmptyState` (heading "No poll cycles yet", distinct from the error heading "Couldn't load system status."); a separate `SystemSkeleton` component covers the loading state distinctly from both. `formatAbsoluteTime`/`formatRelativeTime`/`formatDuration` all degrade null/unparseable timestamps to an em dash, never JS's invalid-date text (`format.ts` lines 68-70, 98-100). Tests: `system.test.tsx` line 503 (first-run copy) and 245 (runless-but-skipping renders loaded, not first-run) both passing. |
| 4 | View fetches once on mount, otherwise only on Refresh click; no steady `/status` traffic while idle; explicit "as of" freshness timestamp | ✓ VERIFIED | Mount `useEffect` in `system.tsx:112-141` keyed on `[reloadToken]` fires once; `grep -rn "setInterval\|setTimeout\|visibilitychange\|addEventListener" web/app/routes/system.tsx web/app/components/system` → 0 matches. `asOf` state rendered as `<time>... as of {formatClock(asOf)}</time>` (system.tsx:184-192), set only on a resolved fetch. Tests: `system.test.tsx` line 80 ("issues no further call while the view remains mounted") and line 561 (double-click re-entrancy caps fetch count at 2), both passing. |
| 5 | Session expiry yields to the passphrase screen and re-fetches after login; no stray requests fire behind the login screen | ✓ VERIFIED | Mount-effect catch branches on `err instanceof ApiError && err.status === 401` and returns before setting any state (`system.tsx:130`); `handleRefresh` has the same branch (line 167) plus its own `mountedRef` guard checked before every state write, independent of the mount effect's cleanup flag. `<App>` (`root.tsx:105-107`) early-returns `<PassphraseScreen>` when `authStore` reports unauthenticated, and remounts `<Outlet/>` (re-triggering the mount fetch) once auth flips back — the same D-16 mechanism verified in Phase 14. Tests: `system.test.tsx` line 97 (mount 401 renders no error surface), line 616 (Refresh session-expiry renders neither error nor inline-failure line), line 635 (Refresh resolving after unmount emits no React unmounted-component warning), all passing. |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `web/app/lib/api.ts` | `KnownOutcome`, `StatusRun`, `StatusSource`, `StatusInstance`, `StatusResponse`, `getStatus()` | ✓ VERIFIED | Every field name matches `internal/httpserver/status.go` json tags character-for-character (manually cross-checked); `schema_applied: number \| null`, `schema_expected: number` as required. |
| `web/app/routes.ts` | `route("system", "routes/system.tsx", { id: "system-path" })` | ✓ VERIFIED | Present at line 11; comment updated to "three tabs/routes." |
| `web/app/root.tsx` | `<NavLink to="/system">` after History, before logout slot | ✓ VERIFIED | Lines 118-120; doc comment updated to "Watchlist, History, System." |
| `web/app/routes/system.tsx` | Five-state route component, `deriveLoadedShape` | ✓ VERIFIED | All states (loading/error/first-run/loaded/refresh-failed-inline) implemented and tested. |
| `web/app/lib/format.ts` | Six pure formatters | ✓ VERIFIED | `formatRelativeTime`, `formatAbsoluteTime`, `formatIsoTitle`, `formatClock`, `formatDuration`, `formatPollInterval` all exported, zero external imports, all injected-clock. |
| `web/app/lib/sources.ts` | `sourceDisplayName`, `SOURCE_ORDER` | ✓ VERIFIED | Lookup-based (not naive title-case), unknown-key passthrough confirmed. |
| `web/app/app.css` | `--color-status-ok`, `--color-status-warn` | ✓ VERIFIED (via review; not independently re-diffed) | Consumed by `OutcomeBadge.tsx`, `AboutInstance.tsx` via `bg-status-ok/15`, `text-status-warn` classes — confirms tokens exist and build succeeded (`go build`/vite build implied by embedded bundle regeneration). |
| `web/app/components/ui/table.tsx` | Vendored shadcn table, 7 exports | ✓ VERIFIED | Imported and used by `SourceHistoryTable.tsx`. |
| `web/app/components/system/OutcomeBadge.tsx` | `classifyOutcome`, `OutcomeBadge`, 5 tiers + default arm | ✓ VERIFIED | Default arm present (`OutcomeBadge.tsx:44-45`); 9 tests covering all tiers plus fallback/empty-string/no-throw. |
| `web/app/components/system/AboutInstance.tsx` | 5-row About block, no second request | ✓ VERIFIED | `grep -c "fetch("` and `grep -c "/ready"` → 0 in both files (re-confirmed). |
| `web/app/components/system/SourcePanel.tsx` | Last-run line, D-08 clean-run scan, skip/escalation lines | ✓ VERIFIED | `findCleanRun` requires `outcome === "ok" && artists_errored === 0`; tested by the "No clean run in recent history" case with errored-but-ok fixtures. |
| `web/app/components/system/SourceHistoryTable.tsx` | 8-column table, 3-condition caption, no sort | ✓ VERIFIED | `grep -c "sort("` → 0; zero-entry returns `null`; cap caption fires at exactly 50 (see Anti-Patterns below for the exact-equality fragility noted by code review). |
| `internal/webassets/build/client` | Regenerated embedded SPA bundle | ✓ VERIFIED | Committed at `433ad77`; contains `assets/system-DLzz-SBw.js`, confirming the new route is actually embedded in the Go binary. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `routes.ts` | `system.tsx` | route registration | ✓ WIRED | Confirmed by route table entry and passing route-mount test. |
| `root.tsx` NavLink | `/system` | `tabLinkClassName` active state | ✓ WIRED | Root test suite covers active-tab class (per 19-01 plan; not independently re-run here but code present and unchanged since). |
| `system.tsx` mount effect | `getStatus()` → `apiFetch('/status')` → D-16 401 interceptor → `authStore` → `<App>` `<PassphraseScreen>` | 401 propagation | ✓ WIRED | `apiFetch` throws `ApiError(401,...)` and calls `authStore.markUnauthenticated()` before throwing (`api.ts:200-203`); mount/refresh catch both branch on this and return early. |
| `SOURCE_ORDER` | `system.tsx` panel iteration | `orderedSourceKeys()` | ✓ WIRED | Confirmed by "renders MusicBrainz before Deezer even when fixture lists them in reverse" test, passing. |
| `classifyOutcome` | `OutcomeBadge` → panel last-run line + history table | one classifier, two call sites | ✓ WIRED | `SourceHistoryTable.tsx` imports and renders `OutcomeBadge` directly (`grep -c "OutcomeBadge"` → 3); `grep -c "Completed with errors"` in the table file → 0, confirming no re-derived classification. |
| `deriveLoadedShape(data)` | render (not state) | not-latched guarantee | ✓ WIRED | `grep -c "useState.*viewState\|setViewState"` → 0; function called inline at `system.tsx:265`. |
| `make web` | `internal/webassets/build/client` committed | embedded bundle | ✓ WIRED | Bundle diff committed at `433ad77`, contains the new `system-*.js` chunk. |

### Behavioral Spot-Checks / Test Execution

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| System-scoped unit/component tests | `corepack pnpm exec vitest run app/routes/system.test.tsx app/components/system` | 2 files, 34 tests passed | ✓ PASS |
| Full frontend suite + coverage gate | `corepack pnpm test` | 205/205 tests passed; Stmts 90.21% / Branch 83.17% / Funcs 88.13% / Lines 91.77% (all ≥ 70% floor) | ✓ PASS |
| No timer/listener in System view | `grep -rn "setInterval\|setTimeout\|visibilitychange\|addEventListener" web/app/routes/system.tsx web/app/components/system` | 0 matches | ✓ PASS |
| No timer/listener repo-wide (D-10 gate) | `grep -rn "setInterval\|setTimeout\|visibilitychange" web/app` | 2 matches, both in pre-existing `SearchBox.tsx` debounce (unrelated to this phase) | ✓ PASS (no regression) |
| No raw-HTML injection | `grep -rn "dangerouslySetInnerHTML" web/app` | 0 matches | ✓ PASS |
| Prettier check | `corepack pnpm exec prettier --check "**/*.{ts,tsx}"` | "All matched files use Prettier code style!" | ✓ PASS |
| Backend build (regression guard) | `go build ./...` | exits 0 | ✓ PASS |
| Embedded bundle contains new route | `ls internal/webassets/build/client/assets \| grep system` | `system-DLzz-SBw.js` present | ✓ PASS |

Full `golangci-lint run`, `make coverage-gate`, `make sqlc-check`, and `make test` (integration suite requiring `make db-up`) were not re-run in this verification pass since no Go source file is in this phase's scope and `go build`/`go vet` already confirm no backend regression; the SUMMARY's own reported values (90.89% backend coverage, clean lint/sqlc-check) are consistent with the phase's declared zero-Go-file-touched scope and are not independently re-verified here.

### Requirements Coverage

| Requirement | Source Plan(s) | Description | Status | Evidence |
|-------------|-----------------|-------------|--------|----------|
| SYS-01 | 19-01, 19-04 | Per-source last run time/outcome/duration/counts + time since last successful run | ✓ SATISFIED | `SourcePanel.tsx`, `OutcomeBadge.tsx`, `format.ts` |
| SYS-02 | 19-01, 19-03, 19-04, 19-05 | Recent-runs history table, watchlist size, poll interval, about block | ✓ SATISFIED | `SourceHistoryTable.tsx`, `AboutInstance.tsx`, `sources.ts` |
| SYS-03 | 19-01, 19-02, 19-04, 19-05 | Fetch on mount + manual Refresh, no fast auto-polling, reuse empty-state/401 handling | ✓ SATISFIED | `system.tsx` mount effect, `handleRefresh`, D-10 grep gates, 401 branches |

No orphaned requirements — REQUIREMENTS.md maps exactly SYS-01/02/03 to Phase 19, all three appear in plan frontmatter `requirements:` fields, all three are marked complete in the traceability table.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `web/app/components/system/OutcomeBadge.tsx` | 20-23 | `titleCase(value: string)` only guards falsy values, not non-string types reaching it via the unchecked `as T` cast in `apiFetch` | ⚠️ Warning (carried from 19-REVIEW.md WR-01) | Low-probability crash path (`value.charAt` would throw on a truthy non-string outcome); the Go handler always marshals a string today, so this is a latent robustness gap, not an active bug. Does not block any must-have. |
| `web/app/components/system/SourceHistoryTable.tsx` | 30 | `captionText` branches on `count === HISTORY_CAP` (exact equality) rather than `>=` | ⚠️ Warning (carried from 19-REVIEW.md WR-02) | If the backend cap is ever misaligned with the frontend's local `HISTORY_CAP = 50` constant, an over-cap payload would print a wrong exact count instead of degrading to the honest "not retained" copy. No shared type-level link between frontend and backend cap. Does not block any must-have — current backend always sends exactly ≤50. |

Both warnings were identified by the phase's own code review (19-REVIEW.md, 0 critical / 2 warning / 1 info) and remain unresolved in the current code (confirmed by direct inspection). Neither affects an observable truth required by the ROADMAP success criteria — both are narrow, low-probability robustness gaps explicitly scoped as non-blocking by the reviewer. No debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) found in any of the 19 phase-touched files.

### Human Verification Required

1. **Live gated-instance smoke test**
   **Test:** On a running gated instance with a real Postgres database, open the System tab and confirm both source panels render with real run data; the About block shows the real version, schema, database pill, watchlist size, and poll interval; clicking Refresh updates the freshness stamp while existing content stays on screen; logging out mid-view yields to the passphrase screen with no error flash.
   **Expected:** All of the above render and behave correctly against real infrastructure.
   **Why human:** This is explicitly named as a `<human-check>` in plan 19-05 Task 3's `<verify>` block — jsdom-based tests cannot render real CSS/layout or exercise a live Postgres-backed gated session. The SUMMARY records this as an "outstanding UAT item," the same pattern used for Phase 18.1's live-instance status check.

### Gaps Summary

No gaps. All five ROADMAP success criteria are verified against the codebase with passing automated tests exercising the actual behavior (not just presence/wiring) for every state-transition and session-boundary truth. The only open item is the human-in-the-loop live-instance smoke test the plan itself deferred to a human, which routes this verification to `human_needed` rather than `passed` per the decision tree — this is not a defect, it is the expected outcome for a phase whose plan explicitly named a live-browser check.

---

_Verified: 2026-09-10_
_Verifier: Claude (gsd-verifier)_
