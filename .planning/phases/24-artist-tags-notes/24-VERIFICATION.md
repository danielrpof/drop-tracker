---
phase: 24-artist-tags-notes
verified: 2026-10-06T20:15:00Z
status: passed
score: 5/5 roadmap success criteria verified; 24-14 gap-closure must-haves 7/7 truths verified, 5/5 prohibitions hold; G-24-9, G-24-10, G-24-11 closed in code
covered_files:
  - .planning/REQUIREMENTS.md
  - .planning/phases/24-artist-tags-notes/24-01-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-01-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-02-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-02-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-03-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-03-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-04-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-04-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-05-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-05-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-06-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-06-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-07-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-07-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-08-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-08-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-09-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-09-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-10-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-10-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-11-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-11-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-12-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-12-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-13-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-13-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-14-PLAN.md
  - .planning/phases/24-artist-tags-notes/24-14-SUMMARY.md
  - .planning/phases/24-artist-tags-notes/24-REVIEW.md
  - .planning/phases/24-artist-tags-notes/24-UAT.md
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/migrate_test.go
  - internal/db/migrations/000010_tags_and_notes.down.sql
  - internal/db/migrations/000010_tags_and_notes.up.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.down.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.up.sql
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.down.sql
  - internal/db/migrations/000012_artist_tags_cap_concurrent_detach.up.sql
  - internal/db/schema_version_test.go
  - internal/db/tags_schema_test.go
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/watchlist.go
  - internal/tags/normalize.go
  - internal/tags/service.go
  - internal/watchlist/service.go
  - internal/webassets/build/client/assets/root-BhE8j8TA.css
  - internal/webassets/build/client/assets/watchlist-Bo02kuoH.js
  - internal/webassets/build/client/index.html
  - queries/tags.sql
  - queries/watchlist.sql
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/watchlist/ArtistNote.test.tsx
  - web/app/components/watchlist/ArtistNote.tsx
  - web/app/components/watchlist/ManageTagsDialog.test.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.ts
  - web/app/lib/tags.ts
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:239caf70ea5c11a4318df8193fa79382276b44cab3facb753ed8dc1b726aba6e"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: "5/5 roadmap SCs; 24-13 must-haves 7/7; 1 human item (UAT test 5 re-run), since passed in 24-UAT.md"
  gaps_closed:
    - "G-24-9 (UAT test 9, blocker): the note 'less' toggle unmounted after expanding. The ArtistNote overflow layout effect now depends on `expanded` and returns early while expanded (ArtistNote.tsx:88-102), so the grown paragraph is never measured and the toggle stays mounted, focused, and the same element. The new fake-ResizeObserver test fails on pre-fix code and passes now."
    - "G-24-10 (UAT test 10, minor): Manage tags left initial focus on the close x. `initialFocusPendingRef` is armed by `open` and consumed by a `[status]` layout effect on the first successful load (ManageTagsDialog.tsx:123-162). The hand-off test fails on pre-fix code. The reload guard test fails when the pending flag is removed (re-arm mutation)."
    - "G-24-11 (UAT test 11, minor): the note textarea was 14px on mobile. className is now `max-h-40 overflow-y-auto md:text-label` (ArtistNote.tsx:179); the rendered textarea carries `text-base` and not the bare `text-label`. The test fails on pre-fix code."
    - "Prior human item (UAT test 5 re-run for G-24-5) is closed: 24-UAT.md records test 5 as pass."
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "24-REVIEW IN-01: from md up, `md:text-sm` (vendored Textarea base) is emitted after `md:text-label` in the same media block and wins font-size and line-height. Both are 14px, so G-24-11 holds; only the line-height differs from the label token (1.25rem vs 1.5)."
    category: other
    reason: "Confirmed in root-BhE8j8TA.css. Cosmetic. The plan flagged this as an accepted assumption. Fix via extendTailwindMerge (IN-02)."
    evidence_status: "CSS inspected; no must-have affected"
  - finding: "24-REVIEW IN-02: tailwind-merge drops `text-label` from the 'add note' button (ArtistNote.tsx:229) because it groups custom size tokens with text colours"
    category: other
    reason: "Predates 24-14; not in any must-have. Register the custom font-size tokens with extendTailwindMerge."
    evidence_status: "code reasoning plus reviewer's local twMerge run"
  - finding: "24-REVIEW IN-03 / IN-04: no committed tests for observer re-attach after collapse, empty-vocabulary focus, error-then-Retry hand-off, or close/reopen re-arm"
    category: other
    reason: "Coverage debt only. The verifier ran throwaway probes for all four and each behaved as specified (see Behavioral Spot-Checks). Commit them to pin the behavior."
    evidence_status: "behavior confirmed by throwaway tests; no failure"
  - finding: "WR-09 and IN-18 (carried from earlier rounds): invalidateLoad() on merge/delete success has no committed regression test; a never-reopened stale load can drop a just-created tag from suggestions"
    category: other
    reason: "Unchanged since the previous verification. Coverage debt and cosmetic."
    evidence_status: "carried; no behavioral failure"
human_verification:
  - test: "UAT test 9 re-run (24-14 Task 3 human-check a): with the go:embed build, give an artist a note long enough to wrap past two lines. Click 'more', then 'less'. Repeat from the keyboard: Tab to 'more', Enter, Enter."
    expected: "'more' expands the full note; the same control then reads 'less', stays visible, and keeps its focus ring. 'less' collapses to two lines; the control reads 'more' with focus still on it. Focus never drops to the page."
    why_human: "jsdom has no layout and no real ResizeObserver. The committed test uses a fake observer and stubbed heights; only a real browser shows the toggle surviving the paragraph's actual growth."
  - test: "UAT test 10 re-run (24-14 Task 3 human-check b): with at least two tags, open Manage tags under DevTools Slow 3G and watch the focus ring when rows appear. Do a delete or rename afterwards. Then delete every tag and reopen."
    expected: "While loading, focus is on the close x. When rows load it moves once to row 1's Rename and does not jump again after later actions. With no tags, focus stays on the close x."
    why_human: "Real ordering against base-ui FloatingFocusManager's rAF-queued initial focus cannot be observed in jsdom."
  - test: "UAT test 11 re-run (24-14 Task 3 human-check c): at a 375px viewport open a note editor and read the textarea's computed font-size; widen to 768px+ and read it again. On a real iPhone, tap into the textarea."
    expected: "16px at 375px, 14px at 768px+. iOS does not zoom on focus."
    why_human: "Breakpoint-dependent computed style and iOS zoom need a real browser/device."
  - test: "Decision on 24-REVIEW WR-01 (reproduced by the verifier): open Manage tags, click Rename on row 1, close the dialog with the x, reopen."
    expected: "Decide whether to fix before phase close (reset renameTarget/renameValue/cancelFocusIndexRef in the open effect and target Rename by a data attribute) or log it as a follow-up. Observed today: row 1 reopens in rename mode with the old text, and the hand-off focuses that row's Cancel button instead of a Rename."
    why_human: "Judgment call on severity. Classified as WARNING, not BLOCKER: it needs a pre-existing stale-state leak (closing mid-rename, present since 24-06), row 1 has no Rename button in that state, focus stays inside the correct row, and pressing Cancel or Enter mutates nothing."
---

# Phase 24: Artist Tags & Notes Verification Report

**Phase Goal:** The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Verified:** 2026-10-06T20:15:00Z
**Status:** human_needed
**Re-verification:** Yes. This follows gap-closure plan 24-14 (G-24-9, G-24-10, G-24-11 from the UI audit). The previous report had no `gaps:`; its one human item (UAT test 5) has since passed in 24-UAT.md.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it. Tags persist across reload and across remove + re-add | VERIFIED (regression) | No Go, SQL, migration, or route file changed since the previous verification (`git diff aaec06e..HEAD` outside `.planning` touches only the 4 planned web files and the embedded bundle). Targeted Go run against live PG: 140 PASS, 0 FAIL, 0 SKIP, including `WatchlistDeleteKeepsTagLinks`. UAT test 1 passed. |
| 2 | `Reggaeton ` attaches the existing `reggaeton`. Over 32 chars or an 11th tag is refused from UI or API, and the DB refuses it with the API bypassed | VERIFIED (regression) | Same targeted run (`TagCap`, `TagNameChecks`, `Normalize`) green; backend untouched since last verification. |
| 3 | Rename once, new name everywhere; renaming onto an existing name asks for confirmation naming both tags; nothing merges without it | VERIFIED (regression) | ManageTagsDialog rename/merge tests (24 in the file) all pass with the new hand-off effect. UAT test 5 passed. 24-14 did not touch the rename/merge handlers or the tags-keyed FocusRequest effect. |
| 4 | Delete a tag globally after a confirmation stating how many artists carry it | VERIFIED (regression) | Delete tests pass; the new guard test drives a failed delete and its reload. |
| 5 | Add, edit, and clear a plain-text note up to 500 chars, shown after reload | VERIFIED | `NoteChecks` passes. ArtistNote: all tests pass, plus the two new G-24-9/G-24-11 tests. Expand/collapse now works (see below). |

**Score:** 5/5 roadmap truths verified (0 present-but-behavior-unverified).

### Gap-closure plan 24-14 must-haves

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | G-24-9: after 'more', a re-measure of the grown paragraph leaves the toggle mounted as 'less', aria-expanded=true, same element, focused; 'less' restores the clamp with focus kept | VERIFIED | New test asserts identity (`toBe(toggle)`), aria-expanded, focus, and the clamp both ways. Mutation: with pre-fix ArtistNote.tsx the test fails. |
| 2 | Overflow measured only on the clamped paragraph; deps include `expanded`; no measure or observer while expanded; collapse re-measures before paint; a short note shows no toggle | VERIFIED | ArtistNote.tsx:88-102, deps `[editing, entry.note, expanded]`, early return at :93. Throwaway probe: observers go 1 → 0 on expand → 1 after collapse, and 'more' keeps focus after the post-collapse re-measure. "renders no more/less toggle when the note fits in two lines" still passes. |
| 3 | G-24-11: no unprefixed `text-label`; `text-base` below md; `md:text-label` from md up | VERIFIED | ArtistNote.tsx:179. Test asserts `text-base` and `md:text-label` present, bare `text-label` absent; fails on pre-fix code. Shipped CSS has `.text-base{font-size:var(--text-base)}` (1rem). From md up `md:text-sm` wins over `md:text-label` by source order; both are 14px (IN-01, advisory). |
| 4 | G-24-10: first GET /tags of an open resolving with tags moves focus from close to row 1's Rename | VERIFIED (with WR-01 warning) | ManageTagsDialog.tsx:144-162. Hand-off test (unsorted input, asserts sorted row 1 "Drill") fails on pre-fix code. Edge case WR-01 reproduced; see below. |
| 5 | Hand-off at most once per open; only from popup/close/body/outside; later reloads never pull focus; empty vocabulary stays on close; error then Retry still hands off | VERIFIED | Guard test passes, and fails when the pending-flag check is removed (re-arm mutation), so it is not vacuous. Throwaway probes: empty vocabulary keeps Close focused; reject → Retry → load focuses "Rename tag Drill"; close + reopen re-arms and hands off again. |
| 6 | Embedded SPA rebuilt; JS contains `max-h-40 overflow-y-auto md:text-label` | VERIFIED | `watchlist-Bo02kuoH.js` contains it; no asset contains `overflow-y-auto text-label`. A fresh `pnpm run build` is byte-identical to `internal/webassets/build/client` (`diff -rq`). `go:embed all:build/client` serves this tree. |
| 7 | Existing tests unchanged; every DoD gate green | VERIFIED | Test diffs since `0d7b36b` remove only two import lines (expanded to add `act`, `onTestFinished`); every other change is an addition. Gates in the spot-check table. |

**Prohibitions (all hold):**

- `ui/textarea.tsx` and `ui/dialog.tsx` unchanged: `git diff --name-only 0d7b36b..HEAD -- web/app/components/ui/` is empty.
- No copy change: the 'more'/'less' strings, aria-labels, and locked strings are unchanged in the diff.
- No `web/package.json`, `web/pnpm-lock.yaml`, `go.mod`, or `go.sum` change against `main`.
- No assertion removed or weakened.
- `REQUIREMENTS.md` and `24-UAT.md` untouched since `0d7b36b`.

### WR-01 assessment (24-REVIEW warning)

**Reproduced.** I used a throwaway test, since deleted. The steps: open, load `[Drill, latin]`, click "Rename tag Drill", `setOpen(false)`, reopen, then resolve the load. Result: `document.activeElement` is the **Cancel** button, and the "New name for Drill" input is rendered again with stale state. Against pre-fix ManageTagsDialog.tsx, the same script leaves focus on **Close** and the stale rename row is also present. So the state leak predates 24-14. The new positional `li button` selector turns it into a misdirected focus move.

**Classification: WARNING, not BLOCKER.**

- Truth 4 is stated for an open where row 1 has a Rename. In the WR-01 state, row 1 is in rename mode and has no Rename button at all.
- The prohibitions hold: the hand-off never steals focus from a control the user moved to.
- Focus lands inside the correct row, and Cancel/Enter mutates nothing.
- The scenario requires closing the dialog mid-rename.
- No roadmap SC depends on it.

A fix is small (reset rename state in the open effect; target Rename by a data attribute). It is listed as a human decision item: fix now or log as a follow-up. Nothing in Phases 25-28 covers it, so it is not deferred.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `web/app/components/watchlist/ArtistNote.tsx` | VERIFIED | Overflow latch + textarea className; 3-line comment |
| `web/app/components/watchlist/ArtistNote.test.tsx` | VERIFIED | Two new tests, both RED on pre-fix code |
| `web/app/components/watchlist/ManageTagsDialog.tsx` | VERIFIED | `initialFocusPendingRef` + `[status]` layout effect; 2-line comment |
| `web/app/components/watchlist/ManageTagsDialog.test.tsx` | VERIFIED | Hand-off test (RED on pre-fix) and guard test (RED under re-arm mutation) |
| `internal/webassets/build/client` | VERIFIED | Identical to a fresh build; carries all three fixes |
| Backend, ConfirmDialog, TagChips/TagCombobox, route (24-01..24-13) | VERIFIED (regression) | Unchanged since previous verification; suites green |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| ArtistNote `expanded` | overflow `useLayoutEffect` | deps `[editing, entry.note, expanded]` + early return | WIRED |
| overflow state | `{overflow && <button>}` toggle | stays true while expanded | WIRED (test asserts same element survives) |
| `<Textarea className>` | rendered textarea classes | vendored `cn(base, className)` | WIRED (test reads `text-base`, `md:text-label` off the DOM) |
| `[open]` effect arms flag | `[status]` layout effect → first `li button` | `initialFocusPendingRef` | WIRED (positional selector; WR-01 edge) |
| `web/build/client` | `internal/webassets/build/client` → `go:embed all:build/client` | `web` recipe | WIRED (byte-identical) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| ArtistNote | `entry.note` | `GET /watchlist` → route state; `PUT /watchlist/{id}/note` | yes | FLOWING (unchanged) |
| ManageTagsDialog | `tags`, `status` | `listTags()` → `GET /tags` | yes | FLOWING (unchanged) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 24-14 tests + existing ArtistNote/ManageTagsDialog | `vitest run ArtistNote.test.tsx ManageTagsDialog.test.tsx --coverage.enabled=false` | 41/41 | PASS |
| Fail-first (pre-fix components from `0d7b36b`, current tests) | same | exactly 3 fail: G-24-11, G-24-9, G-24-10 hand-off; 38 pass | PASS |
| Guard non-vacuity (remove pending-flag check) | `vitest run ManageTagsDialog.test.tsx` | exactly the guard test fails | PASS |
| Throwaway probes (deleted afterwards; `git status web/` clean) | empty vocab / error→Retry / reopen re-arm / observer re-attach | all behave as specified | PASS |
| WR-01 repro | throwaway probe | focus on stale row's Cancel (pre-fix: Close) | WARNING |
| Embedded bundle | fresh `pnpm run build` + `diff -rq` | identical | PASS |
| Prettier / typecheck | `prettier --check`; `pnpm run typecheck` | clean / clean | PASS |
| Web suite (once) | `pnpm --dir web test` | 23 files, 347/347; coverage 92.95/85.92/92.14/94.9 (floor 70) | PASS |
| go build / vet / lint | `go build ./...`; `go vet ./...`; `golangci-lint run` | OK / OK / 0 issues | PASS |
| sqlc drift | `sqlc generate && git diff --exit-code internal/db/sqlc/` | clean | PASS |
| Targeted Go tag/note suite | `go test ./internal/{db,tags,httpserver,watchlist}/... -run 'Tag\|Note\|Delete\|Merge\|Rename\|Normalize\|...'` against live PG | 140 PASS, 4 packages `ok`, no FAIL/SKIP | PASS |
| Full Go suite + 80% gate | not re-run | no Go change since previous verification (89.27%) and the 24-14 executor run (89.27%) | CARRIED |

### Probe Execution

No probes are declared for this phase; Step 7c does not apply.

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | Unchanged; suites green; UAT 1 passed |
| TAG-02 | 24-01, 04 | SATISFIED | Unchanged; UAT 1 passed |
| TAG-03 | 24-01, 02, 05 | SATISFIED | Normalize tests pass |
| TAG-04 | 24-01, 05, 08, 10 | SATISFIED | Caps at API, UI, DB (000010-000012) |
| TAG-05 | 24-02, 06, 11, 12, 13, 14 | SATISFIED | Rename/merge flows tested; Manage tags now opens onto row 1's Rename (WR-01 edge noted) |
| TAG-06 | 24-02, 06, 09, 11, 12, 13, 14 | SATISFIED | Delete count confirm; guard test exercises failed-delete reload |
| TAG-07 | 24-01, 03, 04 | SATISFIED | `WatchlistDeleteKeepsTagLinks` passes |
| NOTE-01 | 24-03, 07, 14 | SATISFIED | `NoteChecks`; expand/collapse fixed (G-24-9); 16px mobile textarea (G-24-11) |

All 8 IDs are claimed by plans; none orphaned.

**Traceability note for the orchestrator:** `REQUIREMENTS.md` still shows TAG-01..04, TAG-07, NOTE-01 as `[ ]` / "Gaps Found", and only TAG-05/06 as Complete. All 8 are satisfied with no open gap. After the human items are signed off, mark all 8 Complete and set G-24-9..11 in `24-UAT.md` to resolved by 24-14 (tests 9-11 to pass after the re-run).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| ManageTagsDialog.tsx | 150 | Positional `li button` selector assumes row 1's first button is Rename (WR-01) | Warning | Misdirected focus after close-mid-rename + reopen; human decision item |
| ManageTagsDialog.tsx | 123-127 | Rename state not reset on open (root of WR-01; predates 24-14) | Warning | Stale rename row with old text reappears on reopen |
| ArtistNote.tsx | 179 | `md:text-label` shadowed by `md:text-sm` (IN-01) | Info | Same 14px; line-height differs |
| ArtistNote.tsx | 229 | tailwind-merge drops `text-label` on "add note" (IN-02, predates 24-14) | Info | 12px instead of 14px |
| ArtistNote.test.tsx | 400-410 | prototype height stubs not restored (IN-03) | Info | Test-order fragility |
| 24-14 files | — | TBD/FIXME/XXX/TODO/HACK | none | — |

### Human Verification Required

### 1. UAT test 9 re-run: expand and collapse a long note

**Test:** With the go:embed build, add a note that wraps past two lines. Click 'more', then 'less'. Repeat with Tab and Enter.
**Expected:** 'less' stays visible and focused after expanding, collapses to two lines, and the control reads 'more' with focus kept. Focus never drops to the page.
**Why human:** jsdom has no layout or real ResizeObserver; the committed test uses a fake.

### 2. UAT test 10 re-run: Manage tags initial focus

**Test:** Open Manage tags with at least two tags under Slow 3G; then act on a row; then delete every tag and reopen.
**Expected:** Focus moves once from the close x to row 1's Rename when rows load, never jumps again, and stays on the close x when the vocabulary is empty.
**Why human:** Real ordering against base-ui's rAF-queued initial focus.

### 3. UAT test 11 re-run: note textarea font size

**Test:** Read the textarea's computed font-size at 375px and at 768px+; tap into it on an iPhone if available.
**Expected:** 16px / 14px; no iOS focus zoom.
**Why human:** Computed styles at breakpoints and iOS zoom need a real browser/device.

### 4. Decide on WR-01

**Test:** Open Manage tags, click Rename on row 1, close with the x, reopen.
**Expected:** A decision: fix before close (reset rename state on open, target Rename by attribute, add a regression test) or log as a follow-up. Today the reopen shows row 1 still in rename mode, and focus lands on its Cancel.
**Why human:** Severity judgment. The verifier rates it WARNING (see the WR-01 assessment).

### Gaps Summary

No blocking gaps. Plan 24-14 closes G-24-9, G-24-10, and G-24-11 in code:

- Each fix is pinned by a test that fails on the pre-fix component, and the guard test fails under a re-arm mutation.
- The embedded bundle matches a fresh build and carries all three fixes.
- All five roadmap success criteria still hold, and all 8 requirement IDs are satisfied.
- Every gate I re-ran is green. The full Go suite and coverage gate were not re-run, because no Go code changed since they last passed.

The status is `human_needed`. UAT tests 9-11 need a real-browser re-run, and the developer needs to decide on WR-01, a reproduced, minor focus misdirection after closing mid-rename that rests on a stale-state leak older than this plan. IN-01..IN-04, WR-09, and IN-18 are advisory.

---

_Verified: 2026-10-06T20:15:00Z_
_Verifier: Claude (gsd-verifier)_
