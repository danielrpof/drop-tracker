---
phase: 24-artist-tags-notes
plan: 05
subsystem: ui
tags: [react, base-ui-combobox, shadcn, tags, watchlist, accessibility]

requires:
  - phase: 24-02
    provides: "GET /tags -- whole vocabulary with watched-only carrier counts"
  - phase: 24-04
    provides: "TagChips base component, TagActions (addTag/removeTag), route-level announce()/status region"
provides:
  - "web/app/components/ui/combobox.tsx, input-group.tsx, textarea.tsx (vendored shadcn base-maia)"
  - "web/app/lib/api.ts: TagSummary, attachTag(entryId, name), listTags()"
  - "web/app/lib/tags.ts: normalizeTagKey, buildTagSuggestions, MAX_TAG_LENGTH, MAX_TAGS_PER_ARTIST"
  - "web/app/components/watchlist/TagCombobox.tsx: the '+ tag' autocomplete editor"
  - "TagActions gains vocabulary/loadVocabulary/rememberTag; TagChips gains the pending set and the '+ tag'/cap trailing slot"
affects: [24-06-manage-tags, 24-07-artist-note]

actuals:
  tokens: 17150
  tasks: 3
  commits: 5
  plan_head_before: 7b7d80e5f8fbd404901e4c8b4443f0f7295012ef

tech-stack:
  added: []
  patterns:
    - "Pure suggestion-ordering function (buildTagSuggestions) returning a closed state union (already-on | empty-vocabulary | items), so the component only renders what the function decided -- no ordering logic lives in JSX"
    - "Unified FocusRequest ref ({chip index} | {grow} | {add-button}) drives two layout effects (one keyed on entry.tags, one on [pending, editorOpen, entry.tags]) so chip-removal focus (24-04) and the new grow/add-button focus share one mechanism instead of parallel ad hoc refs"
    - "Reason-filtered onClose: base-ui's onOpenChange fires for many internal reasons (including a bare 'none' when Enter has nothing to select); only a whitelisted DISMISS_REASONS set (escape-key/outside-press/focus-out/input-blur) is treated as 'the user wants the editor gone'"

key-files:
  created:
    - web/app/components/ui/combobox.tsx
    - web/app/components/ui/input-group.tsx
    - web/app/components/ui/textarea.tsx
    - web/app/components/watchlist/TagCombobox.tsx
    - web/app/components/watchlist/TagCombobox.test.tsx
    - web/app/lib/tags.ts
    - web/app/lib/tags.test.ts
  modified:
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagChips.test.tsx
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx

key-decisions:
  - "The shadcn CLI's `combobox` add silently appended a `cn` npm package to package.json/pnpm-lock.yaml and imported `cn` from it in all three vendored files (instead of the project's `~/lib/utils`). Reverted the manifest/lockfile via `git checkout --` and hand-fixed the three imports -- matches the plan's explicit deviation note and Phase 20's precedent; `pnpm install`'s own prune then removed the orphaned package from node_modules on the next run."
  - "base-ui's Combobox popup closes with reason 'none' when Enter is pressed and nothing is selectable (the already-on state) -- left unfiltered, that closing would have propagated through onClose and collapsed the whole '+ tag' editor, contradicting UI-SPEC's literal 'Enter does nothing'. Fixed by controlling `open` locally and only accepting escape-key/outside-press/focus-out/input-blur as genuine dismiss reasons (found and fixed during Task 2's acceptance-criteria loop, before that task's commit)."
  - "base-ui also echoes the just-committed item's label back through onInputValueChange one call after a pick resolves (its own 'fill selected value into the input' behavior), silently re-filling the input TagCombobox had just cleared. Fixed with a one-shot suppression ref keyed on the committed label (found and fixed during Task 3's acceptance-criteria loop, before that task's commit)."
  - "TagCombobox's onClose now carries a reason ('escape' | 'blur') instead of firing bare, so TagChips can return focus to '+ tag' only for Escape -- UI-SPEC focus table row (a) explicitly leaves focus alone after a blur-close, since the user already moved it somewhere on purpose."

patterns-established:
  - "TagCombobox / lib/tags.ts is the seam Phase 25's filter bar and 24-06's Manage-tags rename input can reuse for name-normalization/counter conventions (MAX_TAG_LENGTH, the 25-char counter threshold)."

requirements-completed: [TAG-01, TAG-03, TAG-04]

coverage:
  - id: D1
    description: "From a card, '+ tag' opens the combobox in place; typing a new name or picking a vocabulary tag and pressing Enter attaches it through the real POST /watchlist/{id}/tags and the chip lands with the server's stored casing, with the vocabulary fetched lazily on first use (never at Watchlist mount) and kept in route state (TAG-01, D-30)."
    requirement: TAG-01
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#typing a new name and pressing Enter commits the typed text"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#picking a vocabulary option commits its stored name"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#never calls listTags at mount, and calls it exactly once across two rows' '+ tag' opens"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#typing a name and pressing Enter attaches it and shows the server's resolved casing"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#still lets Enter attach via Create when listTags rejects"
        status: pass
      - kind: unit
        ref: "app/lib/api.test.ts#attachTag POSTs /watchlist/{entryId}/tags with the name body, the CSRF header, and resolves the server's tag"
        status: pass
      - kind: unit
        ref: "app/lib/api.test.ts#listTags GETs /tags and resolves the vocabulary array with no CSRF header"
        status: pass
    human_judgment: false
  - id: D2
    description: "Suggestion ordering is deterministic: an exact match (NFC-and-whitespace-normalized) is pinned first with no Create row; otherwise Create is pinned first, then contains-matches sorted by name; a query matching a tag already on the artist (including a pending chip) shows the non-selectable already-on line and Enter does nothing; empty/not-yet-loaded vocabulary falls back to Create-only (TAG-01, TAG-03, D-13, D-30, D-31)."
    requirement: TAG-03
    verification:
      - kind: unit
        ref: "app/lib/tags.test.ts (9 tests covering normalizeTagKey NFC/whitespace identity, exact-match pinning, Create-first ordering, already-on for artist and pending names, empty/null vocabulary, and the already-on exclusion from partial matches)"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#pins an exact match and suppresses Create, so Enter attaches the existing tag under its stored casing (TAG-03)"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#shows the non-selectable already-on line and does nothing on Enter when the query matches a tag already on the artist"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#shows 'Type a name to create a tag' for an empty, loaded vocabulary with no query / before the vocabulary has loaded"
        status: pass
    human_judgment: false
  - id: D3
    description: "At 10 tags (server + pending) the '+ tag' button is replaced by the plain-text 'max 10 tags' hint; the pick that reaches 10 closes the editor and focuses the new chip's x, or restores and focuses '+ tag' if that attach fails; removing a chip from a 10-tag artist restores '+ tag' (D-14, TAG-04 UI backstop)."
    requirement: TAG-04
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#shows 'max 10 tags' (not a button) at 10 tags, with no '+ tag'; removing a chip restores it"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#a pick that brings a 9-tag artist to 10 closes the editor, focuses the new chip's x, and announces the max-reached variant"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#a failed 10th attach restores '+ tag' and focuses it"
        status: pass
      - kind: integration
        ref: "app/routes/watchlist.test.tsx#a pick that brings the artist to 10 tags shows the 'max 10 tags' hint through the real route wiring (D-14)"
        status: pass
    human_judgment: false
  - id: D4
    description: "The tag-name input stops at 32 characters; a '{n}/32' counter appears from 25 characters, switching to text-foreground at 32; a hidden polite region announces only the 25 and 32 threshold crossings, once each, with nothing on other keystrokes (D-15)."
    requirement: TAG-04
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#shows the muted 25/32 counter at 25 characters and switches to text-foreground at 32"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#does not show a counter below 25 characters"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#announces the 25 and 32 threshold crossings exactly once each, and nothing on other keystrokes"
        status: pass
    human_judgment: false
  - id: D5
    description: "A rejected attach maps to its exact UI-SPEC toast: 409 -> '{artist} already has 10 tags -- remove one first.'; the 32-character 400 -> 'Tags can be at most 32 characters.'; anything else -> the generic 'Couldn't add...' copy. Only the failed pending chip disappears (TAG-04 API backstop in the UI)."
    requirement: TAG-04
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#maps a 409 refusal to the cap toast"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#maps the exact 400 length message to the length toast"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#maps any other failure to the generic toast"
        status: pass
    human_judgment: false
  - id: D6
    description: "Focus follows UI-SPEC table (a)/(b): Esc closes the popup and editor together in one press and focuses '+ tag'; a blur-close leaves focus wherever the user already moved it; a successful pick under 10 tags leaves focus in the cleared input; removing the only chip focuses '+ tag' (completing 24-04's fallback)."
    verification:
      - kind: unit
        ref: "app/components/watchlist/TagCombobox.test.tsx#calls onClose('escape') and closes the popup when Esc is pressed / calls onClose('blur') when focus leaves the combobox via an outside click"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#Esc closes the editor in one press and focuses '+ tag', without attaching the typed text"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#blur closes the editor without pulling focus back, and does not attach the typed text"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#a successful pick under 10 tags leaves focus in the cleared input and announces the plain message"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/TagChips.test.tsx#focuses '+ tag' when the only chip is removed"
        status: pass
    human_judgment: false
  - id: D7
    description: "[UI E2 overflow + long-text backstop] With 30+ vocabulary tags and a 32-character name in a narrow viewport, the suggestion popup scrolls inside its own bounded list and option text fits or wraps without clipping."
    verification: []
    human_judgment: true
    rationale: "The plan's own must_haves flags this as verification: backstop -- jsdom does not compute layout/scroll, so no automated test can assert real clipping/wrapping behavior at a given viewport width. The vendored ComboboxList carries the registry's own bounded-scroll classes (no-scrollbar max-h-[...] overflow-y-auto) unmodified; a human/visual check is needed to confirm the rendered result."

duration: ~75min
completed: 2026-09-23
status: complete
---

# Phase 24 Plan 05: Tag Autocomplete (Pick, Create, Cap, Errors) Summary

**A single-value, creatable base-ui `Combobox` behind "+ tag" lets a user pick or create a tag through the real `POST /watchlist/{id}/tags`, with deterministic NFC-aware suggestion ordering, a lazily-loaded vocabulary, the 10-tag/32-character caps enforced in the UI, and every server refusal mapped to its exact toast.**

## Performance

- **Duration:** ~75 min
- **Tasks:** 3 (Task 1 tracer: pick/create end to end; Task 2 TDD: deterministic suggestion rules; Task 3 TDD: cap hint, counter, toasts, focus table (a))
- **Files modified:** 13 (7 created, 6 modified)

## Accomplishments

- Vendored `combobox`/`input-group`/`textarea` from the pinned local shadcn `base-maia` registry via the repo's own `shadcn` binary (routed through a `corepack pnpm` shim so the CLI's internal `pnpm add` subprocess could resolve on this Windows/Git-Bash box); `button.tsx`/`input.tsx` and `package.json`/`pnpm-lock.yaml` stayed byte-identical (`git diff --exit-code` gate).
- `web/app/lib/tags.ts`: `normalizeTagKey` (trim, collapse whitespace, `.normalize("NFC")`, lower-case, matching the server's collation-driven identity) and the pure `buildTagSuggestions`, returning `already-on | empty-vocabulary | items` -- exact matches pin first with no Create row, otherwise Create pins first followed by contains-matches sorted by name, and tags already on the artist (server or pending) are never suggested. 9 unit tests cover every `<behavior>` line, including a real NFC-decomposed-vs-precomposed identity pair.
- `TagCombobox.tsx`: builds its item list straight from `buildTagSuggestions`, renders the already-on/empty-vocabulary states through `ComboboxEmpty` with the locked copy, and shows a `{n}/32` counter (muted below 32, `text-foreground` at 32) plus a hidden polite region announcing only the 25/32 threshold crossings.
- `TagChips.tsx`: the chip row now always renders (`+ tag` is the floor, never an empty gap); pending chips (typed casing, `aria-disabled` inert x) live in a local set keyed by a temp id so a concurrent `refresh()` never wipes them; the pick that reaches 10 tags closes the editor immediately and focuses the new chip's x once it lands (or restores and focuses `+ tag` if the attach fails); a unified `FocusRequest` ref now drives chip-removal, chip-grow, and add-button focus from two layout effects.
- `attachErrorMessage` maps `ApiError(409)` / the exact 32-character `ApiError(400)` message to their locked toasts, everything else to the generic "try again" copy.
- Route (`watchlist.tsx`): `vocabulary`/`vocabularyStatus` state, `loadVocabulary()` (fires only from `idle`/`error`, so a failed load retries on the next open and repeated opens never refetch) and `rememberTag()`; the mount effect still calls only `listWatchlist()` -- opening the Watchlist stays one request.
- Two real bugs in base-ui's own Combobox were found and fixed during the TDD acceptance loop, before either task's commit: (1) it closes its popup with reason `"none"` when Enter is pressed with nothing selectable, which would have collapsed the whole `+ tag` editor on the already-on state; (2) it echoes the just-picked label back through `onInputValueChange` one call after a commit, silently re-filling the input TagCombobox had just cleared. Both are documented as Deviations below.

## Task Commits

Each task was committed atomically (Tasks 2 and 3 followed TDD RED->GREEN):

1. **Task 1 (tracer): pick or create one tag from '+ tag' through the real attach route** — `6a30190` (feat)
2. **Task 2 RED: failing tests for deterministic suggestion rules** — `25458eb` (test)
3. **Task 2 GREEN: NFC pinning, already-on, empty states** — `2a216a9` (feat)
4. **Task 3 RED: failing tests for cap hint, counter, refusal toasts, and focus** — `4216971` (test)
5. **Task 3 GREEN: cap hint, length counter, refusal toasts, and focus table (a)** — `2cc256d` (feat)

**Plan metadata:** commit follows this SUMMARY.

## Files Created/Modified

- `web/app/components/ui/combobox.tsx`, `input-group.tsx`, `textarea.tsx` (vendored) — the `cn` import fixed to `~/lib/utils`, no other change from the registry source
- `web/app/lib/tags.ts` (new), `tags.test.ts` (new) — `normalizeTagKey`, `buildTagSuggestions`, `MAX_TAG_LENGTH`, `MAX_TAGS_PER_ARTIST`
- `web/app/components/watchlist/TagCombobox.tsx` (new), `TagCombobox.test.tsx` (new) — the "+ tag" editor: suggestions, counter, live region, close-reason plumbing
- `web/app/components/watchlist/TagChips.tsx`, `TagChips.test.tsx` — pending set, "+ tag"/cap trailing slot, `FocusRequest` ref, `attachErrorMessage`
- `web/app/lib/api.ts`, `api.test.ts` — `TagSummary`, `attachTag`, `listTags`
- `web/app/routes/watchlist.tsx`, `watchlist.test.tsx` — lazy `vocabulary`/`loadVocabulary`/`rememberTag`, `tagActions` wiring, one route-level cap test

## Decisions Made

See `key-decisions` in the frontmatter: the shadcn CLI's stray `cn` package add (reverted), the `onOpenChange` reason `"none"` fix (Enter-with-nothing-selectable must not close the editor), the `onInputValueChange` echo-suppression fix (the input must stay cleared after a commit), and `onClose(reason)` distinguishing Escape from blur.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] base-ui closed the whole "+ tag" editor when Enter was pressed with nothing selectable**
- **Found during:** Task 2's acceptance-criteria loop (the already-on state's "Enter does nothing" requirement)
- **Issue:** base-ui's `Combobox` fires `onOpenChange(false, { reason: "none" })` when Enter is pressed and no item is highlighted (e.g. the already-on or empty-vocabulary states). The original `onOpenChange={(open) => { if (!open) onClose() }}` treated every close the same way, so this internal, non-user-driven close collapsed the entire editor back to "+ tag" -- contradicting the UI-SPEC's literal "Enter does nothing."
- **Fix:** `open` is now controlled locally (`popupOpen`); `onOpenChange` only calls `onClose()` for a whitelisted `DISMISS_REASONS` set (`escape-key`, `outside-press`, `focus-out`, `input-blur`) -- reason `"none"` and any other internal reason leave the editor open.
- **Files modified:** `web/app/components/watchlist/TagCombobox.tsx`
- **Verification:** `app/components/watchlist/TagCombobox.test.tsx#does not call onClose when Enter is pressed with nothing selectable (already-on state)`
- **Committed in:** `2a216a9` (Task 2 GREEN)

**2. [Rule 1 - Bug] The cleared input silently re-filled with the just-committed label**
- **Found during:** Task 3's acceptance-criteria loop (verifying focus stays in a cleared input after a successful pick)
- **Issue:** base-ui echoes the picked item's label back through `onInputValueChange` one call after `onValueChange` fires (its own "reflect the selection in the input" behavior). `commit()` was clearing `query` synchronously, but the trailing echo call overwrote it back to the typed text on the very next render.
- **Fix:** a one-shot `suppressEchoRef` records the label just committed; `handleInputValueChange` ignores exactly one incoming value that matches it, then resumes normal (counter-tracking) handling.
- **Files modified:** `web/app/components/watchlist/TagCombobox.tsx`
- **Verification:** `app/components/watchlist/TagChips.test.tsx#a successful pick under 10 tags leaves focus in the cleared input and announces the plain message`
- **Committed in:** `2cc256d` (Task 3 GREEN)

**3. [Rule 3 - Blocking] shadcn CLI's internal `pnpm add` could not resolve `pnpm` on PATH**
- **Found during:** Task 1's vendoring step
- **Issue:** This Windows/Git-Bash dev box has `pnpm` available only via `corepack pnpm`, not as a bare binary on `PATH`. The shadcn CLI shells out to a bare `pnpm add` internally, which failed with `'pnpm' is not recognized`.
- **Fix:** created a throwaway `pnpm.cmd` shim (forwarding to `corepack pnpm %*`) in the session scratchpad and prepended it to `PATH` for the vendoring command only -- not committed, not part of the repo.
- **Files modified:** none (environment-only, session-scoped)
- **Verification:** the vendoring command completed and produced the expected 3 files
- **Committed in:** n/a (no repo change)

---

**Total deviations:** 3 auto-fixed (2 Rule 1 bugs in a third-party library's interaction with our controlled state, 1 Rule 3 local-environment blocker).
**Impact on plan:** All three were necessary for correctness (the two base-ui interaction bugs) or for the vendoring step to run at all (the PATH shim). No scope creep -- the plan's own deviation notes already anticipated needing to work around this registry's rough edges.

## Issues Encountered

None beyond the two base-ui interaction bugs documented above, which were found and resolved as part of each task's own TDD acceptance loop rather than treated as separate defects.

## User Setup Required

None — no external service configuration required.

## jsdom Setup

No stub was added to `web/vitest.setup.ts`. base-ui's Combobox rendered cleanly in jsdom across all three tasks with no missing-browser-API errors (no `ResizeObserver`/`Element.prototype.scrollIntoView` polyfill was needed).

## Next Phase Readiness

- `web/app/lib/tags.ts` (`normalizeTagKey`, `MAX_TAG_LENGTH`) and the vendored `combobox`/`input-group` are ready for 24-06's Manage-tags rename input (same counter/length conventions) and Phase 25's filter bar.
- `TagActions.vocabulary`/`loadVocabulary`/`rememberTag` are the exact seam 24-06's Manage-tags dialog needs to open with an already-warm vocabulary when the user has already used "+ tag" on the same visit.
- The held-out backstop truth (D7: combobox popup scroll/wrap at 30+ tags in a narrow viewport) is flagged `human_judgment: true` for `/gsd-verify-work` and needs a real-browser check, not an assumption to close here.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-23*

## Self-Check: PASSED

- All 10 key files verified present on disk (`FOUND` for every entry in `files_modified`/`created`), plus this SUMMARY.
- All 5 task commit hashes verified present in `git log --oneline --all` (6a30190, 25458eb, 2a216a9, 4216971, 2cc256d).
- All acceptance criteria across Tasks 1-3 re-run and passing (vitest target suites per task, `typecheck`, the vendoring diff gate).
- Plan-level `<verification>` re-run: `prettier --check` clean, `typecheck` clean, `corepack pnpm --dir web test` green -- 282 tests passed, coverage 91.29%/84.36%/90.16%/93.19% (stmts/branch/funcs/lines, all >= 70% gate), `git diff --exit-code -- web/package.json web/pnpm-lock.yaml web/app/components/ui/button.tsx web/app/components/ui/input.tsx` clean (no dependency or vendored-file drift), `! grep -rn "dangerouslySetInnerHTML" web/app` exits 0 (no matches).
