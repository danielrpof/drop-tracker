---
phase: 20-digest-settings-operator-control
plan: "03"
subsystem: ui
tags: [react-router, base-ui, digest-settings, instant-apply, operator-control]

# Dependency graph
requires:
  - phase: 20-digest-settings-operator-control (plan 20-01)
    provides: the frozen GET/PUT /settings/notifications wire contract (settingsResponse/updateSettingsRequest)
  - phase: 19-frontend-system-view
    provides: the /system view's mount/Refresh fetch machinery, SystemSkeleton, and the refreshError inline-status pattern this plan extends
provides:
  - "NotificationSettings wire type, getDigestSettings(), updateDigestSettings() in web/app/lib/api.ts"
  - "DigestSettings card (Digest mode row) with instant-apply save, synchronous re-entrancy guard, and keep-stale-on-failure posture"
  - "/system view fetching status and digest settings together on mount and Refresh, rendering the card between AboutInstance and the empty-watchlist alert"
affects: [20-04-cadence-and-last-sent, 21-digest-mutual-exclusion]

# Actuals (#2632)
actuals:
  tokens: 6069
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "instant-apply-with-keep-stale-on-failure: a single optimistic overlay (pending | null) plus a status enum, cleared on both success and failure so the last-known-good value renders for free with no separate rollback bookkeeping"
    - "synchronous ref-based re-entrancy guard (savingRef) checked/set before the first await, distinct from the rendered disabled state -- mirrors system.tsx's own refreshingRef pattern applied to a write instead of a read"

key-files:
  created:
    - web/app/components/system/DigestSettings.tsx
    - web/app/components/system/DigestSettings.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/routes/system.tsx
    - web/app/routes/system.test.tsx

key-decisions:
  - "The base-ui Switch renders aria-disabled (not the native `disabled` attribute) on its role=\"switch\" element, so jest-dom's toBeDisabled() doesn't apply -- tests assert on getAttribute(\"aria-disabled\") === \"true\" instead."
  - "A standalone unit test of DigestSettings with a static `settings` prop cannot show the post-save rendered value, since the component never reads back its own write -- the save-success test uses a small ControlledDigestSettings wrapper that feeds onSaved's payload back in as the next `settings` prop, the way system.tsx's own setDigest does."

requirements-completed: [DGST-01, DGST-16]

coverage:
  - id: D1
    description: "An operator on /system sees the Digest notifications card, flips the digest-mode switch, and the change is persisted and confirmed inline (Saved.)"
    requirement: DGST-01
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#saves the toggle, calling updateDigestSettings once with the full payload, then shows On and Saved."
        status: pass
    human_judgment: false
  - id: D2
    description: "A failed save keeps the last-known-good value on screen and shows the locked failure copy"
    requirement: DGST-01
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#keeps Off on screen and shows the failure copy when the save rejects"
        status: pass
    human_judgment: false
  - id: D3
    description: "Two switch interactions dispatched in the same task start exactly one request, and the control is disabled while a save is in flight on both the resolve and reject paths"
    requirement: DGST-01
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#starts exactly one request when two interactions are dispatched in the same task"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#disables the switch while a save is in flight and re-enables it once it resolves"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#disables the switch while a save is in flight and re-enables it once it rejects"
        status: pass
    human_judgment: false
  - id: D4
    description: "Digest settings ride the same single mount/Refresh Promise.all fetch as the status payload, and a digest-settings-only failure produces the existing page-level error state, never a separate surface"
    requirement: DGST-01
    verification:
      - kind: unit
        ref: "web/app/routes/system.test.tsx#renders the Digest notifications card alongside the About block on a successful mount"
        status: pass
      - kind: unit
        ref: "web/app/routes/system.test.tsx#renders the existing page-level error state when the digest-settings fetch rejects with a non-401 error"
        status: pass
    human_judgment: false
  - id: D5
    description: "NotificationSettings wire type matches internal/httpserver/settings.go's json tags character-for-character, including digest_last_sent_at typed string | null"
    requirement: DGST-16
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#getDigestSettings() issues its request to /settings/notifications and resolves all four fields intact"
        status: pass
      - kind: other
        ref: "manual read-back: internal/httpserver/settings.go's settingsResponse fields vs. web/app/lib/api.ts's NotificationSettings interface"
        status: pass
    human_judgment: false
  - id: D6
    description: "updateDigestSettings() sends full-object PUT semantics carrying the centrally-injected CSRF header, and both wrappers propagate a real ApiError on 401/400"
    requirement: DGST-01
    verification:
      - kind: unit
        ref: "web/app/lib/api.test.ts#updateDigestSettings() PUTs both snake_case keys to /settings/notifications carrying the centrally-injected CSRF header"
        status: pass
      - kind: unit
        ref: "web/app/lib/api.test.ts#updateDigestSettings() rejects with an ApiError carrying the server's fixed error message on a 400"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-13
status: complete
---

# Phase 20 Plan 03: SPA Digest Mode Toggle Summary

**Digest mode toggle on /system, wired end to end through a new base-ui Switch card, instant-apply PUT, and a widened Promise.all mount/Refresh fetch — no cadence control or last-sent row yet (plan 20-04)**

## Performance

- **Duration:** ~40 min
- **Tasks:** 2
- **Files modified:** 6 (2 created, 4 modified)

## Accomplishments
- `NotificationSettings` wire type plus `getDigestSettings()`/`updateDigestSettings()` wrappers added to `web/app/lib/api.ts`, typed field-for-field against `internal/httpserver/settings.go`'s `settingsResponse`, with `updateDigestSettings()` sending full-object PUT semantics through `apiFetch`'s existing CSRF/401 machinery.
- New `DigestSettings` card (`web/app/components/system/DigestSettings.tsx`) copies `AboutInstance`'s `Card`/`dl` chrome verbatim, renders the single `Digest mode` row (vendored `Switch` + `On`/`Off` text), and implements the full instant-apply save machinery: a `pending` optimistic overlay, a synchronous `savingRef` re-entrancy guard, a `mountedRef` post-await guard, a 2-second self-clearing `Saved.` confirmation, and the locked `Couldn't save — reverted to the previous value.` failure line — all in muted/destructive text, never the reserved run-health palette.
- `web/app/routes/system.tsx`'s mount effect and `handleRefresh` both now fetch `[getStatus(), getDigestSettings()]` together, so a digest-settings failure produces the existing page-level `Couldn't load system status.` state rather than a separate surface, and the card can never drift stale relative to the rest of the page. `SystemSkeleton` gained one more 3-bar card in the correct position.
- 35 new/extended tests across `DigestSettings.test.tsx`, `system.test.tsx`, and `api.test.ts` pin the render, the toggle-success/failure paths, the double-interaction re-entrancy guard, the disabled-while-saving state on both settle paths, the shared mount/refresh fetch composition, and the wrapper-level request shape/CSRF header/401/400 propagation.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end "flip digest mode from /system" — one control only** - `7e1a032` (feat)
2. **Task 2: Pin the wrappers and the two guards the tracer crosses** - `9e88a6d` (test)

## Files Created/Modified
- `web/app/lib/api.ts` - `NotificationSettings` interface, `getDigestSettings()`, `updateDigestSettings()`
- `web/app/lib/api.test.ts` - wrapper request-shape, CSRF header, 401, and 400-message cases
- `web/app/components/system/DigestSettings.tsx` - the Digest mode card
- `web/app/components/system/DigestSettings.test.tsx` - render, save-success, save-failure, re-entrancy, disabled-while-saving (resolve + reject), and 401-swallow cases
- `web/app/routes/system.tsx` - widened mount/Refresh fetch, `digest` state, card render site, extended skeleton
- `web/app/routes/system.test.tsx` - default `getDigestSettings` mock, card-presence case, non-401 shared-error case

## Decisions Made
- The base-ui `Switch` renders `aria-disabled` on its `role="switch"` element rather than the native `disabled` attribute, so `jest-dom`'s `toBeDisabled()` doesn't recognize it as disabled. Tests assert `getAttribute("aria-disabled") === "true"` directly instead of relaxing the assertion.
- A save-success unit test rendering `DigestSettings` in isolation with a static `settings` prop cannot observe the post-save rendered value, because the component itself never reads back its own write — that's the parent's job (`onSaved`). Added a small `ControlledDigestSettings` test wrapper that feeds the resolved payload back in as the next `settings` prop, mirroring what `system.tsx`'s `setDigest` does in production, rather than weakening the test to only check the mock call.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] A code comment's literal string tripped its own acceptance grep**
- **Found during:** Task 1, post-implementation acceptance-criteria check
- **Issue:** Two comments spelled out the literal substrings the plan's own acceptance greps scan for — `--color-status-ok`/`--color-status-warn` in `DigestSettings.tsx` (tripping the `status-ok|status-warn|sonner` count-must-be-0 gate) and `Promise.all` in a `system.tsx` comment (inflating the `Promise.all` line count from 2 to 3).
- **Fix:** Reworded both comments to describe the same intent without repeating the literal grep-matched substrings ("the reserved run-health palette" instead of the token names; "one combined fetch" instead of "Promise.all").
- **Files modified:** `web/app/components/system/DigestSettings.tsx`, `web/app/routes/system.tsx`
- **Verification:** Both greps now report the exact required counts (0 and 2 respectively); full targeted test suite reconfirmed green.
- **Committed in:** `7e1a032` (part of Task 1's commit — caught before commit, not a separate fix commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 — a comment tripping its own acceptance grep, same failure class 20-01's SUMMARY already documented)
**Impact on plan:** Cosmetic-only; no behavior change. No scope creep.

**Task boundary note (not a deviation from behavior, but from task sequencing):** Task 1's commit already included `DigestSettings.test.tsx`'s two re-entrancy/disabled-while-saving guard cases (needed to verify the tracer's own acceptance criteria before committing), so Task 2's commit contains only the `api.ts` wrapper test additions — the component guard tests it was scoped to add were already present and green. All of Task 2's required behaviors are covered; nothing is missing, just committed one task earlier than the plan's task split anticipated.

## Issues Encountered
None beyond the self-corrected comment greps above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The `DigestSettings` component, `NotificationSettings` type, and the widened mount/Refresh fetch composition are frozen for plan 20-04, which adds the `Cadence` `Select` control and the `Last digest sent` row into the same card and `dl`.
- No blockers. `go vet ./...` and `golangci-lint run` both clean (no backend files touched by this plan); frontend Definition of Done green (`prettier --check` clean, `corepack pnpm --dir web test` 219/219 passing, all four coverage axes above 70%, no dependency changes).

## Self-Check: PASSED

All 2 listed created files verified present on disk; both commit hashes (`7e1a032`, `9e88a6d`) verified present in git log.

---
*Phase: 20-digest-settings-operator-control*
*Completed: 2026-09-13*
