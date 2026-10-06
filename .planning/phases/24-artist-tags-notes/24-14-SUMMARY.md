---
phase: 24-artist-tags-notes
plan: 14
subsystem: ui
tags: [react, focus-management, resize-observer, tailwind, gap-closure, embedded-spa, g-24-9, g-24-10, g-24-11]
status: complete

requires:
  - phase: 24-artist-tags-notes
    provides: ArtistNote (24-07), ManageTagsDialog (24-05..24-09), embedded SPA recipe (24-13)
provides:
  - "ArtistNote overflow effect latches while expanded: the 'less' toggle stays mounted, is the same focused element, and collapses the note again (G-24-9, D-05)"
  - "ManageTagsDialog hands focus from the close x to row 1's Rename on the first load of each open, never later and never from a control the user moved to (G-24-10, D-07, D-08)"
  - "Note textarea keeps the built-in 16px text-base below md and uses md:text-label from md up, so iOS Safari does not zoom on focus (G-24-11, D-06)"
  - Refreshed embedded SPA bundle carrying all three fixes
affects: [24-verification, 24-uat-reverify]

actuals:
  tokens: 2500
  tasks: 3
  commits: 5
plan_head_before: 9aad187739ba6714434c161bd34126ff7f59b449

tech-stack:
  added: []
  patterns:
    - "Overflow measured only against the clamped paragraph: the layout effect depends on `expanded` and returns early (no measure, no observer) while expanded"
    - "One-shot focus hand-off ref armed by `open` and consumed by the first successful load; moves focus only from the popup chrome/body/outside"

key-files:
  created: []
  modified:
    - web/app/components/watchlist/ArtistNote.tsx
    - web/app/components/watchlist/ArtistNote.test.tsx
    - web/app/components/watchlist/ManageTagsDialog.tsx
    - web/app/components/watchlist/ManageTagsDialog.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "G-24-9: dependency-plus-early-return in the overflow layout effect rather than OR-ing with a ref-held expanded flag, so the observer never runs when it cannot give a meaningful answer."
  - "G-24-10: the hand-off flag is consumed only by a successful load, so an error followed by Retry still hands off."

requirements-completed: [NOTE-01, TAG-05, TAG-06]

coverage:
  - id: G-24-9
    description: "An expanded note keeps its 'less' toggle (same element, aria-expanded=true, focused) after the grown paragraph re-measures, and 'less' restores line-clamp-2 with focus kept"
    requirement: "NOTE-01"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ArtistNote.test.tsx#an expanded note keeps its 'less' toggle mounted and focused after the grown paragraph re-measures, and 'less' collapses it again"
        status: pass
    human_judgment: false
  - id: G-24-10
    description: "Manage tags opened with tags moves focus from the close button to row 1's Rename once the list loads, once per open; later reloads leave focus alone"
    requirement: "TAG-05"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ManageTagsDialog.test.tsx#opening with tags hands focus from the close button to row 1's Rename once the list loads"
        status: pass
      - kind: unit
        ref: "web/app/components/watchlist/ManageTagsDialog.test.tsx#a reload later in the same open session leaves focus alone instead of pulling it to row 1's Rename"
        status: pass
    human_judgment: false
  - id: G-24-11
    description: "Note textarea has text-base and md:text-label and no unprefixed text-label"
    requirement: "NOTE-01"
    verification:
      - kind: unit
        ref: "web/app/components/watchlist/ArtistNote.test.tsx#the note textarea keeps the 16px base size below md and switches to the label size only from md up"
        status: pass
    human_judgment: false
  - id: G-24-real-browser
    description: "In a real browser: more/less keeps focus through the paragraph's growth, the Manage tags focus ring lands on row 1's Rename, and the textarea computes 16px at 375px and 14px at 768px+"
    requirement: "NOTE-01"
    human_judgment: true
    rationale: "jsdom computes no layout and fires no real ResizeObserver or animation-frame focus; this is the Task 3 human check, deferred to phase-level UAT (human_verify_mode: end-of-phase)."
  - id: E-embed
    description: "Embedded bundle rebuilt; the watchlist chunk contains `max-h-40 overflow-y-auto md:text-label` and no pre-fix className"
    requirement: "TAG-06"
---

# Phase 24 Plan 14: Note toggle, Manage tags focus, and textarea size Summary

ArtistNote's overflow measurement now latches while expanded so the 'less' toggle survives the paragraph growing, ManageTagsDialog hands focus to row 1's Rename once per open, and the note textarea keeps 16px below md; the embedded SPA is rebuilt with all three.

**Duration:** about 20 min | **Completed:** 2026-10-06 | **Tasks:** 3 | **Files:** 4 web source files plus the embedded bundle

## Accomplishments

- **G-24-9 (blocker).** The overflow `useLayoutEffect` now has deps `[editing, entry.note, expanded]` and returns early while `expanded`: no measure, no ResizeObserver. Expanding disconnects the observer, so the unclamped paragraph is never measured, `overflow` stays true, and the toggle stays mounted (same DOM element, focus kept). Collapsing re-measures the clamped paragraph before paint. The 4-line comment became 3 lines.
- **G-24-10 (minor).** `initialFocusPendingRef` is armed by the `[open]` effect (`= open`) and consumed by a `useLayoutEffect` keyed on `[status]` once `status === "loaded"`. It focuses the first `li`'s first button (row 1's Rename) only when focus is on the popup, its close button, the body, or outside the popup. An empty vocabulary has no row, so focus stays on close. An error does not consume the flag, so Retry then load still hands off. The `tags`-keyed FocusRequest effect and rename-cancel effect are untouched.
- **G-24-11 (minor).** The note Textarea className is now `max-h-40 overflow-y-auto md:text-label`; the vendored base `text-base` (16px) applies below md.
- **Embedded SPA** rebuilt via the `web` recipe steps; the new `watchlist-Bo02kuoH.js` chunk contains the new className and no chunk contains the pre-fix one.

## Task Commits

| Task | Commit | Scope |
| ---- | ------ | ----- |
| 1 RED | 2fa4310 | test(24-14): ArtistNote toggle-survival and textarea size tests |
| 1 GREEN | b122403 | fix(24-14): overflow latch + textarea className |
| 2 RED | f321d91 | test(24-14): Manage tags hand-off and reload guard tests |
| 2 GREEN | 5be6e33 | fix(24-14): one-shot focus hand-off |
| 3 | 63c38b6 | chore(24-14): rebuilt embedded SPA |

## TDD Fail-First Evidence

On pre-fix code (tests committed before the fixes):

- G-24-9 test FAILED at `screen.getByRole("button", { name: "less" })` after the observer fired: "Unable to find an accessible element with the role button and name less" (toggle had unmounted).
- G-24-11 test FAILED on `expect(textarea).not.toHaveClass("text-label")`; received class list ended `... max-h-40 overflow-y-auto text-label md:text-label`.
- G-24-10 hand-off test FAILED on `expect(Rename tag Drill).toHaveFocus()` (focus stayed on Close).
- G-24-10 reload guard test PASSED before the fix and PASSES after it. It is a guard (fails for any implementation that re-arms the hand-off on every load), not RED evidence.

All four new tests pass after the fixes. Existing ArtistNote and ManageTagsDialog tests are unchanged and green (41 passed across the two files).

## Deviations from Plan

None - plan executed exactly as written.

Two small notes, neither a deviation: Task 1's test initially tripped `tsc` (`onTestFinished(() => vi.unstubAllGlobals())` returns `VitestUtils`, not `Awaitable<void>`); wrapped in a block body before the commit. RED and GREEN were split into separate per-file commits (test first, then fix) per tdd.md rather than a single task commit.

## Definition of Done results

| Gate | Result |
| ---- | ------ |
| `go vet ./...` | pass |
| `golangci-lint run` | 0 issues |
| Full integration suite (Postgres via `docker compose up -d --wait postgres`) | 25 `ok` packages, no FAIL/panic lines, coverage profile written |
| Coverage gate | 89.27% (floor 80) |
| sqlc generate + `git diff --exit-code internal/db/sqlc/` | clean |
| `prettier --check "**/*.{ts,tsx}"` | pass (ran `--write` before staging) |
| `typecheck` | pass |
| `pnpm --dir web test` | 23 files, 347 tests pass; coverage 92.95 / 85.92 / 92.14 / 94.9 (stmts/branches/funcs/lines, floor 70) |

Blast radius verified: no diff against `main` in `internal/notifier|detection|discord|musicbrainz|deezer`, `go.mod`, `go.sum`, `web/package.json`, `web/pnpm-lock.yaml`. Since `0d7b36b`, the only `web/app` changes are the four planned files; `ui/textarea.tsx`, `ui/dialog.tsx`, `REQUIREMENTS.md`, and `24-UAT.md` are untouched.

**Environment notes:** `make` is not on PATH, so the recipes were run directly with `~/go/bin` prepended (the `web` recipe steps, `go test ./... -coverprofile -coverpkg` with the Makefile's COVER_PKGS filter, the coverage-report total, and `sqlc generate` plus the git diff). `-race` was omitted (CI's Linux job is the race gate). The Docker daemon was not running at the start; Docker Desktop was launched to bring up the Postgres fixture. The Go gate was judged on the log (no FAIL/panic, `ok` lines present) rather than the exit code, per the known `covdata` quirk.

## Known Stubs

None.

## Threat Flags

None. No new routes, request shapes, or rendering paths. T-24-66 (focus hijack) and T-24-67 (stranded keyboard user) are mitigated and pinned by the reload guard test and the fake-ResizeObserver test.

## Deferred to phase-level UAT (human check, `human_verify_mode: end-of-phase`)

Re-run UAT tests 9, 10, 11 against the go:embed build: (a) expand/collapse a long note with mouse and keyboard, focus never lost; (b) Manage tags focus lands on row 1's Rename once, stays on close for an empty vocabulary; (c) textarea computed font-size is 16px at 375px and 14px at 768px or wider.

## Self-Check: PASSED

- FOUND: web/app/components/watchlist/ArtistNote.tsx, ArtistNote.test.tsx, ManageTagsDialog.tsx, ManageTagsDialog.test.tsx
- FOUND: internal/webassets/build/client/assets/watchlist-Bo02kuoH.js containing `max-h-40 overflow-y-auto md:text-label`
- FOUND commits: 2fa4310, b122403, f321d91, 5be6e33, 63c38b6
