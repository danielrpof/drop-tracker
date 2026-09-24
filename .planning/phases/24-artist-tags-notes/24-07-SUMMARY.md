---
phase: 24-artist-tags-notes
plan: 07
subsystem: ui
tags: [react, vitest, watchlist, notes, tdd, embedded-spa]

requires:
  - phase: 24-03
    provides: "PUT /watchlist/{id}/note (trim/cap/clear), POST /watchlist's optional note, the shared tags+note projection"
  - phase: 24-04
    provides: "route-level announce/status region"
  - phase: 24-05
    provides: "the vendored Textarea component"
  - phase: 24-06
    provides: "Manage tags dialog (unrelated surface, same phase)"
provides:
  - "ArtistNote: the per-card note display/editor (NOTE-01) -- add, edit, clear, save through PUT, never PATCH"
  - "The remove toast's Undo now restores the entry's note (D-27)"
  - "internal/webassets/build/client rebuilt so a Go-only build serves this phase's SPA"
affects: []

actuals:
  tokens: 8306
  tasks: 3
  commits: 4
  plan_head_before: 4f43e6aa6c77276d6e8b7300a615b0a744a2fa87

tech-stack:
  added: []
  patterns:
    - "ArtistNote's FocusRequest ref pattern (mirrors TagChips' FocusRequest): a save-success focus target is computed from the fresh updateNote response, not the entry prop, since the parent's re-render carrying the new note may not have landed by the time the post-close layout effect runs."
    - "Overflow measurement (the note's more/less toggle) via useLayoutEffect + optional ResizeObserver, with scrollHeight/clientHeight stubbed directly in tests since jsdom performs no layout."

key-files:
  created:
    - web/app/components/watchlist/ArtistNote.tsx
    - web/app/components/watchlist/ArtistNote.test.tsx
  modified:
    - web/app/lib/api.ts
    - web/app/lib/api.test.ts
    - web/app/components/watchlist/WatchlistRow.tsx
    - web/app/routes/watchlist.tsx
    - web/app/routes/watchlist.test.tsx
    - internal/webassets/build/client

key-decisions:
  - "Task 1's tracer scope stayed deliberately narrow (no Saving state, no focus management, no announce calls, no overflow toggle) so Task 2's TDD RED phase had genuine failing assertions to drive rather than pre-implementing the behavior TDD exists to prove."
  - "Reworded a source comment that spelled out the raw-HTML rendering API name literally, since it tripped the phase's own `! grep -rn dangerouslySetInnerHTML web/app` gate even though no code used it -- only prose referenced it."
  - "actuals.tokens counts only the human/AI-authored diff under web/app, excluding the machine-generated internal/webassets/build/client rebuild, to keep the number meaningful against the plan's estimate."

patterns-established:
  - "Save-focus-after-async-update: when a component's own visual state (which trigger to focus) depends on the result of an async call rather than a prop the parent may not have re-rendered with yet, stash that result in a ref-based FocusRequest and consume it in a layout effect keyed on the local state transition, not on the prop."

requirements-completed: [NOTE-01]

coverage:
  - id: D1
    description: "A card with a note shows it clamped to 2 lines with a pencil; a card without shows only 'add note'; opening the editor pre-fills the textarea, focuses it with the caret at the end, and Save (through PUT /watchlist/{id}/note, never PATCH) is disabled while the trimmed text equals the current note; saving empty/whitespace clears the note."
    requirement: NOTE-01
    verification:
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#renders note text literally, never as HTML (T-24-37)"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#Save calls updateNote with the trimmed text and patches the entry"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#Save of whitespace-only text calls updateNote with null (clears the note)"
        status: pass
      - kind: unit
        ref: "app/lib/api.test.ts#updateNote PUTs /watchlist/{entryId}/note with {note} and the CSRF header"
        status: pass
    human_judgment: false
  - id: D2
    description: "A failed save keeps the textarea open with the typed text and an inline error (generic, or the 400 length backstop); while saving, Save reads 'Saving…', the textarea is readOnly+aria-busy, Cancel is disabled and Esc is ignored; Ctrl/Cmd+Enter saves; a successful save focuses the pencil or 'add note' and announces the route status region; a hidden region announces the 450/500 counter thresholds; 'more'/'less' appears only when the note actually overflows."
    requirement: NOTE-01
    verification:
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#while saving: Save reads 'Saving…' ... Esc does nothing"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#a rejected save keeps the textarea open ... re-enables Save"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#a 400 length rejection shows the server-backstop copy"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#a successful save focuses the pencil and announces 'Note saved for {artist}.'"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#a successful save focuses 'add note' and announces 'Note cleared for {artist}.'"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#the hidden counter region announces threshold crossings at 450 and 500"
        status: pass
      - kind: unit
        ref: "app/components/watchlist/ArtistNote.test.tsx#shows the 'more' toggle only when the note paragraph actually overflows"
        status: pass
    human_judgment: false
  - id: D3
    description: "The remove toast's Undo re-adds the artist with its note (D-27); a plain re-add from search still starts blank."
    requirement: NOTE-01
    verification:
      - kind: unit
        ref: "app/routes/watchlist.test.tsx#Undo re-adds the artist with its note"
        status: pass
      - kind: unit
        ref: "app/routes/watchlist.test.tsx#Undo for an entry with no note sends no note value"
        status: pass
    human_judgment: false
  - id: D4
    description: "The embedded SPA (internal/webassets/build/client) is rebuilt with this phase's UI, and the whole-phase backend Definition of Done (go vet, golangci-lint, go test ./... with coverage, sqlc-check) is green with no blast-radius or dependency drift."
    requirement: NOTE-01
    verification:
      - kind: other
        ref: "go vet ./..."
        status: pass
      - kind: other
        ref: "golangci-lint run (via pre-commit's pinned v2.13.2, --all-files)"
        status: pass
      - kind: integration
        ref: "go test ./... -count=1 -coverprofile=coverage.out (25 packages, 0 failures)"
        status: pass
      - kind: other
        ref: "cmd/coverage-report --mode=total: 89.27% (floor 80%)"
        status: pass
      - kind: other
        ref: "sqlc generate (docker sqlc/sqlc:1.31.1) -- git diff --exit-code internal/db/sqlc/ clean"
        status: pass
      - kind: other
        ref: "git diff --name-only main -- internal/notifier internal/detection internal/discord internal/musicbrainz internal/deezer go.mod go.sum web/package.json web/pnpm-lock.yaml -- empty"
        status: pass
    human_judgment: true
    rationale: "The phase-level human-check (add/rename/merge/delete tags, add/edit/clear a note, remove+Undo, reload, plus UI-SPEC [R8] contrast/hit-area/narrow-viewport checks with a real browser) needs a rendered browser jsdom cannot provide. Per workflow.human_verify_mode: end-of-phase, this is harvested into the phase-level UAT rather than gated here."

duration: 17min
completed: 2026-09-24
status: complete
---

# Phase 24 Plan 07: Watchlist Note Editor + Embedded SPA Rebuild Summary

**A plain-text note per Watchlist card (add/edit/clear, saved through PUT /watchlist/{id}/note), with full failure/in-flight/focus/announcement/overflow behavior built TDD-first, Undo restoring the note on re-add, and the embedded SPA bundle refreshed to close out the phase.**

## Performance

- **Duration:** 17 min
- **Started:** 2026-09-24T03:11:04Z
- **Completed:** 2026-09-24T03:27:52Z
- **Tasks:** 3 completed
- **Files modified:** 6 source files + the refreshed embedded SPA bundle (25 asset files)

## Accomplishments

- `ArtistNote` renders a note clamped to 2 lines with an edit pencil, or `add note` when there is none, and an in-place `Textarea` editor with a live `{n}/500` counter -- `Save` goes through the new `updateNote(entryId, note)` wrapper (`PUT /watchlist/{id}/note`, never `PATCH`, D-25) and is disabled while the trimmed text equals the current note.
- Task 2 built the remaining UI-SPEC contract test-first: a `Saving…` in-flight state (`readOnly` + `aria-busy` textarea, disabled Cancel, Esc ignored), a failed save that keeps the typed text and shows the generic or 400-length-backstop copy inline ([R2]), `Ctrl`/`Cmd`+`Enter` to save, save-success focus management (pencil vs `add note`, sourced from the fresh server response rather than a possibly-stale prop) with route-status announcements, a hidden live region for the 450/500 counter thresholds, and a `more`/`less` toggle gated on real paragraph overflow.
- `watchlist.tsx`'s remove-toast Undo now passes `note: entry.note ?? undefined` to `addWatchlist`, so Undo never silently drops a note (D-27); a plain re-add from search still starts blank.
- `internal/webassets/build/client` was rebuilt via `make web`'s recipe and committed, so a Go-only build serves this phase's tags/notes UI instead of the pre-phase SPA.
- The whole-phase backend Definition of Done is green: `go vet`, `golangci-lint` (pinned v2.13.2 via pre-commit, `--all-files`), `go test ./...` (25 packages, coverage 89.27% against an 80% floor), and `sqlc-check` (via the pinned `sqlc/sqlc:1.31.1` Docker image, no drift). Blast-radius and dependency diffs against `main` are both empty.

## Task Commits

Each task was committed atomically:

1. **Task 1: Add a note on a card and see it saved through PUT /watchlist/{id}/note (tracer)** - `290056e` (feat)
2. **Task 2: Failure-keeps-text, in-flight state, focus, announcements, overflow toggle, Undo (RED)** - `0784437` (test)
2. **Task 2: Failure-keeps-text, in-flight state, focus, announcements, overflow toggle, Undo (GREEN)** - `a50c6c0` (feat)
3. **Task 3: Rebuild the embedded SPA and run the whole-phase Definition of Done** - `5636677` (feat)

**Plan metadata:** commit follows this SUMMARY.

_TDD Task 2 produced no REFACTOR commit -- the GREEN implementation needed no follow-up cleanup._

## Files Created/Modified

- `web/app/components/watchlist/ArtistNote.tsx` - new component: note display, `add note`/pencil triggers, and the in-place editor (Save/Cancel/counter/error/overflow toggle)
- `web/app/components/watchlist/ArtistNote.test.tsx` - full behavior coverage: rendering, save/clear, disabled-when-unchanged, Esc, in-flight state, failure copy (generic and 400), Ctrl/Cmd+Enter, save-success focus+announce, counter thresholds, overflow toggle
- `web/app/lib/api.ts` - `updateNote(entryId, note)`; `addWatchlist` gains an optional `note` param
- `web/app/lib/api.test.ts` - tests for both, plus the "omits the key when undefined" case
- `web/app/components/watchlist/WatchlistRow.tsx` - renders `ArtistNote` after the chip row in the name column
- `web/app/routes/watchlist.tsx` - Undo now passes the removed entry's note to `addWatchlist`
- `web/app/routes/watchlist.test.tsx` - two Undo tests (with note, without) via a mocked `sonner` toast
- `internal/webassets/build/client` - refreshed SPA bundle

## Decisions Made

- Task 1 (the tracer) was kept deliberately minimal -- save/cancel/disabled-when-unchanged only, no in-flight/focus/announce/overflow -- so Task 2's TDD RED phase had real failing assertions to drive rather than tests that trivially passed against pre-built behavior.
- A source comment in `ArtistNote.tsx` originally spelled out the raw-HTML rendering API name literally; it tripped Task 3's own `! grep -rn dangerouslySetInnerHTML web/app` acceptance check even though no code used it. Reworded to describe the behavior instead of naming the API.
- The counter-threshold test switched from `userEvent.type`'s per-keystroke simulation (which timed out building a 449+ character string) to `fireEvent.change` for the bulk value jumps -- still exercises the real `onChange` handler, just without ~450 individually-timed key events.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test performance: counter-threshold test timed out under `userEvent.type`**
- **Found during:** Task 2 GREEN verification
- **Issue:** `userEvent.type(textarea, "a".repeat(449))` simulates one timed key event per character; building a 449+ character value this way exceeded Vitest's 5s default test timeout.
- **Fix:** Switched the bulk value jumps to `fireEvent.change`, which still fires a real `onChange` event through the same handler, without the per-keystroke simulation cost.
- **Files modified:** `web/app/components/watchlist/ArtistNote.test.tsx`
- **Verification:** Full test file passes in ~7s (was timing out at 5s per case).
- **Committed in:** `a50c6c0` (Task 2 GREEN commit)

**2. [Rule 3 - Blocking] Doc comment tripped the phase's own XSS-surface grep gate**
- **Found during:** Task 3's acceptance-criteria verification (`! grep -rn "dangerouslySetInnerHTML" web/app`)
- **Issue:** `ArtistNote.tsx`'s top doc comment named the raw-HTML rendering API literally to explain what the component does *not* do, which the phase's own literal grep check (correctly) cannot distinguish from real usage.
- **Fix:** Reworded to "renders as a plain JSX text node only -- never raw HTML," matching the convention every other file in the codebase already follows (none of them name the API literally either).
- **Files modified:** `web/app/components/watchlist/ArtistNote.tsx`
- **Verification:** `! grep -rn "dangerouslySetInnerHTML" web/app` exits 0.
- **Committed in:** `5636677` (Task 3 commit)

---

**Total deviations:** 2 auto-fixed (1 test-performance bug, 1 blocking acceptance-gate fix)
**Impact on plan:** Both fixes were necessary to get the plan's own gates green; neither changed shipped behavior. No scope creep.

## Issues Encountered

- `sqlc` and `golangci-lint` are not on this box's bare `PATH` (they live inside `pre-commit`'s per-hook cache and a local Docker image respectively). Ran `golangci-lint` via `python -m pre_commit run golangci-lint --all-files` (resolves the pinned v2.13.2 binary from `.pre-commit-config.yaml`) and `sqlc` via `docker run --rm -v "$(pwd):/src" -w /src sqlc/sqlc:1.31.1 generate` (`MSYS_NO_PATHCONV=1` needed on this Git Bash to stop the `-w /src` flag from being mangled into a Windows path). Both resolved to the project's pinned versions; no workaround was needed beyond locating them.
- `go test ./... -race` is unusable on this Windows dev box (documented pre-existing limitation, STATE.md); ran without `-race` per the environment notes, with CI's Linux job as the authoritative race-detector gate.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- NOTE-01 is now fully closed: the note API (24-03) and the SPA editor (this plan) both land, and Undo restores the note (D-27).
- Phase 24 (Artist Tags & Notes) is complete: all 7 plans have SUMMARYs, the embedded SPA reflects the whole phase, and the backend Definition of Done is green.
- The phase-level human-check (tags CRUD, note CRUD, Undo, reload, and the UI-SPEC [R8] contrast/hit-area/narrow-viewport items) is recorded in this plan's `<verify><human-check>` block for harvesting into the phase UAT (`workflow.human_verify_mode: end-of-phase`) -- not yet run in a real browser.
- No blockers.

---

*Phase: 24-artist-tags-notes*
*Completed: 2026-09-24*
