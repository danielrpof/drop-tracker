---
phase: 25-find-filter-watchlist-and-history
plan: 16
subsystem: ui
tags: [react, tooltip, gap-closure, search-results, tdd]
status: complete
requires:
  - phase: 25-find-filter-watchlist-and-history
    provides: "G-25-9 capped and truncating Deezer pill from plan 25-13"
provides:
  - "DEEZER_ADD_HINT module constant in SearchResultsColumns.tsx"
  - "Deezer hint pill wrapped in a titled, width-capped span so the native tooltip is reachable on hover"
affects: [25-18 real-browser UAT test 9, gsd-verify-work]
actuals:
  tokens: 6000
  tasks: 1
  commits: 1
plan_head_before: 9744475c59f9ed85a5c42cc97f8d044fcd8f504b
tech-stack:
  added: []
  patterns:
    - "Put title on a pointer-receiving wrapper when the control is disabled (disabled:pointer-events-none swallows hover)"
key-files:
  created: []
  modified:
    - web/app/components/watchlist/SearchResultsColumns.tsx
    - web/app/components/watchlist/SearchResultsColumns.test.tsx
key-decisions:
  - "Per user decision 2026-10-11 (gap planning), WR-04: title and the max-w-[40%] shrink-0 lg:max-w-none cap sit on a wrapper span; the Button keeps disabled, gains w-full, and keeps its truncating inner span"
requirements-completed: []
coverage:
  - id: D1
    description: "The truncated Search MusicBrainz to add pill carries its full explanation on a wrapper that receives pointer events; cap and truncation (G-25-9) and the accessible name are unchanged"
    requirement: WLVW-01
    verification:
      - kind: test
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#caps the Deezer hint wrapper, titles it, and truncates the button label (G-25-9, WR-04)"
        status: pass
      - kind: test
        ref: "web/app/components/watchlist/SearchResultsColumns.test.tsx#disables the control with a MusicBrainz-only hint for a Deezer result"
        status: pass
    human_judgment: false
  - id: D2
    description: "Native tooltip actually appears on hover of the truncated pill at 375px and 640px, and the full label shows at 1024px"
    requirement: WLVW-01
    human_judgment: true
    rationale: "Native tooltips and real widths need a browser; joins UAT test 9 (human_verify_mode end-of-phase)"
duration: 4 min
completed: 2026-10-12
---

# Phase 25 Plan 16: Deezer Hint Wrapper Title Summary

**The truncated `Search MusicBrainz to add` pill on Deezer rows now explains itself on hover: `title` and the 40% cap moved to a wrapper span that receives pointer events (closes WR-04).**

## Accomplishments

- `DEEZER_ADD_HINT` module constant holds the hint copy verbatim; the Deezer branch renders `<span title={DEEZER_ADD_HINT} className="max-w-[40%] shrink-0 lg:max-w-none">` around the disabled Button.
- The Button keeps `variant="secondary"`, `size="sm"`, `disabled`, gains `w-full`, and keeps its single inner `<span className="truncate">`. Its accessible name is still exactly `Search MusicBrainz to add`.
- `Add to Watchlist`, `Already watching`, and `web/app/components/ui/button.tsx` are untouched.

## Task Commits

1. Task 1 (tracer): `ca1106b` fix(25-16): title the truncated Deezer hint pill through a pointer-receiving wrapper

Committed once after GREEN (plan commit discipline), so no RED-only commit exists.

## RED Evidence

Renamed test `caps the Deezer hint wrapper, titles it, and truncates the button label (G-25-9, WR-04)` against the pre-change component:

```
AssertionError: expected 'LI' to be 'SPAN'
Expected: "SPAN"
Received: "LI"
 ❯ SearchResultsColumns.test.tsx:187:29  expect(wrapper.tagName).toBe("SPAN")
```

The button's parent was the row `li` (no wrapper, no title). After the change: 11/11 tests in the file pass.

## Deviations from Plan

None - plan executed exactly as written.

## Verification

- `vitest run app/components/watchlist/SearchResultsColumns.test.tsx --coverage.enabled=false`: 11 passed.
- `corepack pnpm --dir web run typecheck`: clean. `grep -c 'title={DEEZER_ADD_HINT}'` prints 1. `git diff --name-only` for `ui/button.tsx`, `package.json`, `pnpm-lock.yaml`: empty.
- `prettier --check "**/*.{ts,tsx}"` clean; full `corepack pnpm --dir web test`: 31 files, 596 tests passed, coverage thresholds met.
- Hooks (gitleaks, prettier) passed on commit. Go gates and the bundle rebuild are deferred to 25-18 (no Go file changed).
- Requirement WLVW-01 is also declared by sibling plans without summaries yet, so the shared-ID gate left it unmarked.

## Known Stubs

None.

## Threat Flags

None. The title is a static constant (T-25-44); `ui/button.tsx` and package files unchanged (T-25-45, T-25-SC).

## Next

Ready for 25-17. The real-browser hover check joins UAT test 9.

## Self-Check: PASSED

- FOUND: web/app/components/watchlist/SearchResultsColumns.tsx, web/app/components/watchlist/SearchResultsColumns.test.tsx
- FOUND commit: ca1106b
