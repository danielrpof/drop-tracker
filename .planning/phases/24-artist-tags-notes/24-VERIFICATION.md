---
phase: 24-artist-tags-notes
verified: 2026-10-05T22:21:47Z
status: human_needed
score: 5/5 roadmap success criteria verified; 24-13 gap-closure must-haves 7/7 truths verified, 5/5 prohibitions hold for task commits (G-24-5 closed in code)
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
  - internal/webassets/build/client/assets/root-Dg0NMU0m.css
  - internal/webassets/build/client/assets/watchlist-z6XpSwAW.js
  - internal/webassets/build/client/index.html
  - queries/tags.sql
  - queries/watchlist.sql
  - web/app/components/common/ConfirmDialog.test.tsx
  - web/app/components/common/ConfirmDialog.tsx
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
covered_digest: "v1:sha256:64a31ad6e4adbe6e3ff10e91f336476e233ce061850b0f495bfecd6ca9733979"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: "5/5 roadmap SCs; 24-12 must-haves 13/13; 8 human items, then UAT: 7 pass, 1 issue (G-24-5), G-24-4 resolved as not a defect"
  gaps_closed:
    - "G-24-5 (UAT test 5): merge confirm title overflowed sideways at 375px with two 32-char unspaced names. ConfirmDialog now passes className=\"wrap-anywhere\" to AlertDialogTitle and AlertDialogDescription. Two new tests pin the class; removing the class turns both red (mutation check). A headless-Chrome layout probe with the shipped CSS shows the title and description stay inside the 320px popup (scrollWidth 320). The no-fix control reproduces the overflow (scrollWidth 656)."
  gaps_remaining: []
  regressions: []
advisory:
  - finding: "WR-09 (carried): the invalidateLoad() calls in handleConfirmMerge and handleConfirmDelete in ManageTagsDialog.tsx have no committed regression test"
    category: other
    reason: "Behavior was confirmed correct last round with a throwaway test and a mutation check. This is test-coverage debt; commit the merge-mid-reopen test sketched in the 24-12 review."
    evidence_status: "mutation evidence of a coverage hole; no behavioral failure"
  - finding: "IN-18 (carried): a Manage tags load from an open that is never reopened stays current and can drop a just-created tag from suggestions"
    category: other
    reason: "Cosmetic. Create get-or-creates the same normalized tag, so the one-tag-per-name rule still holds."
    evidence_status: "code reasoning; no must-have affected"
  - finding: "24-REVIEW IN-02: wrap-anywhere needs Tailwind 4.1+, but web/package.json declares tailwindcss ^4"
    category: other
    reason: "The lockfile pins 4.3.0 and the shipped CSS contains .wrap-anywhere{overflow-wrap:anywhere}. A 4.0.x resolve would drop the rule silently while the class-level tests stay green. Latent; resolve by raising the floor to ^4.1."
    evidence_status: "no active failure; lockfile pins a compatible version"
human_verification:
  - test: "UAT test 5 re-run (24-13 Task 2 human-check): with the go:embed build at a 375px viewport, create two 32-char tags, 31 W + A and 31 W + B. Rename the first onto the second to open the merge confirm, then cancel. Open Delete on one of them. Finally, open a merge confirm between two short multi-word tags."
    expected: "The merge and delete titles wrap onto several lines inside the dialog: no horizontal scroll, no ellipsis, nothing past the dialog edge. Both full names are readable, the consequence sentence wraps the same way, and Cancel and Merge tags stay visible. Short multi-word titles break only at spaces."
    why_human: "jsdom computes no layout. The verifier's headless-Chrome probe used a hand-built copy of the dialog DOM with the shipped CSS, not the live app. The plan defers this end-of-phase check to UAT, and UAT.md still records G-24-5 as failed until it is re-run."
---

# Phase 24: Artist Tags & Notes Verification Report

**Phase Goal:** The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Verified:** 2026-10-05T22:21:47Z
**Status:** human_needed
**Re-verification:** Yes. This follows UAT and gap-closure plan 24-13 (G-24-5). The previous report was human_needed with no `gaps:`. UAT found G-24-5 and resolved G-24-4 as not a defect.

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it. Tags persist across reload and across remove + re-add | VERIFIED (regression) | No Go, SQL, or `internal/db` file changed since `0cbc334`. Full Go suite run once against live PG: 25 packages `ok`, no FAIL. `WatchlistDeleteKeepsTagLinks` passes. UAT test 1 (end-to-end in a real browser) passed. |
| 2 | `Reggaeton ` attaches the existing `reggaeton`. Over 32 chars or an 11th tag is refused from the UI or API, and the DB refuses it even when the API is bypassed | VERIFIED (regression) | Targeted `-v` run of `TagCap\|TagNameChecks\|NoteChecks\|WatchlistDeleteKeepsTagLinks\|Delete\|Merge\|Rename\|Normalize` across `internal/db`, `internal/tags`, `internal/httpserver`: 53 PASS, 0 SKIP, 0 FAIL. |
| 3 | Rename once, new name everywhere. Renaming onto an existing name asks for a confirmation naming both tags; nothing merges without it | VERIFIED | The 24-13 merge test renders the full `Merge “{32a}” into “{32b}”?` heading and asserts `mergeTag` is not called before confirmation. Both names are now readable at 375px (G-24-5, see below). The earlier rename/merge tests still pass (27/27 in the two files). |
| 4 | Delete a tag globally after a confirmation stating how many artists carry it; the tag disappears from every artist; the artists are untouched | VERIFIED | The delete confirm uses the same ConfirmDialog, so it inherits the wrap fix (layout probe case `fix-delete` stays inside the popup). Server delete and idempotency tests pass in the targeted run. |
| 5 | Add, edit, and clear a plain-text note up to 500 chars, shown after reload | VERIFIED (regression) | `NoteChecks` passes. ArtistNote tests are in the green web suite (343/343). |

**Score:** 5/5 roadmap truths verified (0 present-but-behavior-unverified).

### Gap G-24-5 (UAT test 5): CLOSED in code

**The code.** `web/app/components/common/ConfirmDialog.tsx:71-76` passes `className="wrap-anywhere"` to both `AlertDialogTitle` and `AlertDialogDescription`, with a 2-line why comment. The vendored `ui/alert-dialog.tsx` merges it through `cn(base, className)`. tailwind-merge keeps it next to the description's `text-balance`, since the two belong to different groups.

**Evidence. I did not rely on SUMMARY claims:**
1. **Tests:** `vitest run ConfirmDialog.test.tsx ManageTagsDialog.test.tsx --coverage.enabled=false` passes 27/27, including both new tests.
2. **Mutation check:** I stripped `className="wrap-anywhere"` from ConfirmDialog. Exactly the two new tests failed (25 pass, 2 fail), so the tests pin the fix. The file was restored and `git status web/` is clean.
3. **Shipped CSS:** `internal/webassets/build/client/assets/root-Dg0NMU0m.css` contains `.wrap-anywhere{overflow-wrap:anywhere}`. The class also appears in `watchlist-z6XpSwAW.js`, the chunk that contains ConfirmDialog.
4. **Embedded bundle is current:** I ran a fresh `pnpm run build`. `diff -rq web/build/client internal/webassets/build/client` reports them identical.
5. **Layout in a real engine (headless Chrome):** I loaded the shipped CSS into a page with the dialog's popup, header, title, and description class structure, copied from `ui/alert-dialog.tsx`. Below `sm`, the popup is capped at `max-w-xs` (320px), so 375px and the probe's viewport give the same dialog width.

| Case | popup scrollWidth / clientWidth | title right / popup right | title lines | overflow-wrap |
|------|------|------|------|------|
| fix, merge, 31W+A / 31W+B | 320 / 320 | 386 / 410 (inside the 24px padding) | ~7 | anywhere |
| fix, delete, 31W+A | 320 / 320 | 386 / 410 | ~4 | anywhere |
| **control (no class), merge** | **656 / 320** | **746 / 410 (overflows)** | ~4 | normal |
| fix, short multi-word merge | 320 / 320 | inside | 2 | anywhere |
| no class, short multi-word merge | 320 / 320 | inside | 2 | normal |

The control reproduces the UAT symptom, and the fix removes it. Short multi-word titles wrap to the same 2 lines with and without the class, so ordinary words are not split early.

The live-app visual check (UAT test 5 re-run) stays a human item. The plan defers it to end-of-phase UAT, and my probe is a hand-built copy of the dialog DOM, not the running app.

### Gap-closure plan 24-13 must-haves

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | At 375px the 32+32 merge title wraps inside the dialog, never overflows or truncates, and both names stay readable | VERIFIED | Layout probe (table above); full-name heading match in the merge test. Live-app re-run is a human item. |
| 2 | The title (`h2`) and description (`p`) carry `wrap-anywhere` and not `truncate`; merge, delete, and future bulk remove inherit it | VERIFIED | Code at ConfirmDialog.tsx:73-74. There is a single shared component, and both call sites use it. |
| 3 | The merge confirm with two 32-char names renders the full heading and consequence sentence, both wrapping; `mergeTag` is not called | VERIFIED | New ManageTagsDialog test passes; mutation turns it red |
| 4 | Titles made of ordinary words still break only at spaces | VERIFIED | Probe: same 2-line layout with and without the class |
| 5 | The vendored `ui/alert-dialog.tsx` is unchanged | VERIFIED | `git log 0cbc334..HEAD -- web/app/components/ui/alert-dialog.tsx` is empty |
| 6 | The embedded SPA is rebuilt and its CSS contains `overflow-wrap:anywhere` | VERIFIED | grep match, and identical to a fresh build |
| 7 | Existing tests unchanged and every DoD gate green | VERIFIED | The diff to the test files is additions only. Gates are listed in the spot-checks below. |

**Prohibitions:**
- **`ui/alert-dialog.tsx` not edited:** holds.
- **Title and description never truncated, clamped, or clipped:** holds. ConfirmDialog adds no `truncate`, `line-clamp`, or `overflow-hidden`.
- **No `package.json` or lockfile change:** holds. The diff against `main` for deps and out-of-scope packages is empty.
- **No assertions weakened:** holds. The test diffs are additions only.
- **No task edits `REQUIREMENTS.md` or `24-UAT.md`:** holds for the task commits `05140ae` and `845a876`. The orchestrator's completion commit `d2b6db4` did edit `REQUIREMENTS.md`, marking TAG-05 and TAG-06 `[x]` / Complete (see the traceability note under Requirements Coverage).

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `web/app/components/common/ConfirmDialog.tsx` | VERIFIED | Both elements carry `wrap-anywhere`, wired through `cn()`, and render in both confirms |
| `web/app/components/common/ConfirmDialog.test.tsx` | VERIFIED | New class-pin test; fails when the fix is removed |
| `web/app/components/watchlist/ManageTagsDialog.test.tsx` | VERIFIED | New two-32-char merge test drives the real collision path |
| `internal/webassets/build/client` | VERIFIED | Byte-identical to a fresh build; the CSS rule is present |
| Backend artifacts (migrations 000010-000012, tags/watchlist services, httpserver, ADR 0004) | VERIFIED (regression) | Unchanged since `0cbc334`; suites green |
| ManageTagsDialog / route vocabulary guard (24-11, 24-12) | VERIFIED (regression) | Unchanged; route and dialog tests pass |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| ConfirmDialog `className="wrap-anywhere"` | rendered `h2` / `p` | vendored `cn(base, className)` | WIRED (the tests read the class off the rendered elements) |
| ManageTagsDialog `collisionTarget` | ConfirmDialog title `Merge “…” into “…”?` | props | WIRED (the merge test finds the heading by full name) |
| Tailwind 4.3.0 `wrap-anywhere` | `root-Dg0NMU0m.css` → go:embed | build + copy | WIRED (CSS rule present; embed tree matches a fresh build) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| ConfirmDialog (merge) | title, description | ManageTagsDialog collision result from `renameTag` (`PATCH /tags/{id}`) and the tag list from `GET /tags` | yes | FLOWING |
| TagCombobox / TagChips / ManageTagsDialog / ArtistNote | vocabulary, entries, tags, note | `GET /tags`, `GET /watchlist` | yes | FLOWING (unchanged) |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| 24-13 tests | `vitest run ConfirmDialog.test.tsx ManageTagsDialog.test.tsx --coverage.enabled=false` | 27/27 | PASS |
| Mutation (class removed) | same command | 2 fail (exactly the new tests), 25 pass | PASS (the tests pin the fix) |
| Layout at a 320px popup | headless Chrome + shipped CSS | fix contained; control overflows | PASS |
| Embedded bundle | fresh build + `diff -rq` | identical | PASS |
| Prettier / typecheck | `prettier --check`; `pnpm run typecheck` | clean / clean | PASS |
| Web suite (once) | `pnpm --dir web test` | 23 files, 343/343; coverage 92.32/85.49/91.81/94.34 (floor 70) | PASS |
| go vet / golangci-lint | `go vet ./...`; `golangci-lint run` | clean / 0 issues | PASS |
| sqlc drift | `sqlc generate && git diff --exit-code internal/db/sqlc/` | clean | PASS |
| Full Go suite + coverage (once) | `go test ./... -count=1 -p 1 -coverpkg=<COVER_PKGS>` against live PG (no `-race` on Windows) | 25 packages `ok`, no FAIL; total 89.27% (floor 80). The exit code is 1 only because of `go: no such tool "covdata"` on `internal/testutil`, a toolchain quirk the SUMMARY also notes. | PASS |
| Targeted TAG/NOTE Go tests | `-v -run 'TagCap\|...\|Normalize'` | 53 PASS, 0 SKIP | PASS |

### Probe Execution

No probes are declared for this phase, so Step 7c does not apply.

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | Unchanged; suites green; UAT 1 passed |
| TAG-02 | 24-01, 04 | SATISFIED | Unchanged; UAT 1 and 4 passed |
| TAG-03 | 24-01, 02, 05 | SATISFIED | Normalize tests pass |
| TAG-04 | 24-01, 05, 08, 10 | SATISFIED | Caps at API, UI, and DB, including concurrent detach (000012) |
| TAG-05 | 24-02, 06, 11, 12, 13 | SATISFIED | Rename/merge flows tested. The merge confirm now shows both full names at narrow widths. |
| TAG-06 | 24-02, 06, 09, 11, 12, 13 | SATISFIED | Delete count confirm wraps; idempotency tests pass |
| TAG-07 | 24-01, 03, 04 | SATISFIED | `WatchlistDeleteKeepsTagLinks` passes |
| NOTE-01 | 24-03, 07 | SATISFIED | `NoteChecks` and ArtistNote tests pass |

All 8 IDs are claimed by plans, and none are orphaned.

**Traceability note for the orchestrator:** `REQUIREMENTS.md` is inconsistent right now. `d2b6db4` marked only TAG-05 and TAG-06 `[x]` / Complete, from 24-13's `requirements-completed`. TAG-01..04, TAG-07, and NOTE-01 are still `[ ]` / "Gaps Found", although all are satisfied and none has an open gap. After the UAT test 5 re-run is signed off, move all 8 to Complete, and set G-24-5 in `24-UAT.md` to resolved.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| ConfirmDialog.test.tsx | 52-63 | Test name claims long-name coverage but uses the short `Delete “rap”?` title; the `not.toHaveClass("truncate")` assertion can never fail (24-REVIEW IN-01) | Info | Test naming and strength only. The ManageTagsDialog test covers the real long-name case. |
| web/package.json | 44 | `tailwindcss: ^4` allows 4.0.x, which lacks `wrap-anywhere` (IN-02) | Info / advisory | Latent; the lockfile pins 4.3.0 |
| ConfirmDialog.tsx | 26-34 | 9-line header comment against the 1-3 line rule (IN-03) | Info | Predates 24-13 |
| ManageTagsDialog.tsx | merge/delete success | `invalidateLoad()` not pinned by a test (WR-09, carried) | Warning / advisory | Coverage debt |
| files changed by 24-13 | — | TBD/FIXME/XXX/TODO/HACK | none | — |

### Human Verification Required

UAT tests 1-4 and 6-8 passed in `24-UAT.md` and drop out of this list. One planner-deferred item remains, harvested from 24-13 Task 2 `<human-check>`:

### 1. UAT test 5 re-run: merge/delete confirm wrap at 375px

**Test:** Run the go:embed build at a 375px viewport and create two 32-char tags, 31 `W` + `A` and 31 `W` + `B`. Rename the first onto the second to open the merge confirm, then cancel. Open Delete on one of them. Finally, open a merge confirm between two short multi-word tags.
**Expected:** The titles and consequence sentences wrap inside the dialog: no horizontal scroll, no ellipsis, no clipping. Both names are fully readable, and the buttons stay visible. Short multi-word titles break only at spaces.
**Why human:** jsdom computes no layout. My headless-Chrome probe used a hand-built copy of the dialog DOM with the shipped CSS, not the running app. UAT.md still shows G-24-5 as failed until this is re-run.

### Gaps Summary

There are no gaps. Plan 24-13 closes G-24-5 in code:
- ConfirmDialog's title and description carry `overflow-wrap: anywhere`, pinned by two tests that fail without it.
- The shipped CSS contains the rule, and the embedded bundle matches a fresh build.
- A headless-Chrome layout probe shows the 32+32 merge and delete titles stay inside the 320px popup. The no-fix control reproduces the reported overflow.
- All five roadmap success criteria still hold, all 8 requirement IDs are satisfied, and every Definition of Done gate is green: vet, lint, sqlc, the full Go suite at 89.27%, prettier, typecheck, and the web suite at 343/343.

The status is `human_needed` because of one remaining item: re-run UAT test 5 against the live app. Advisory debt (WR-09, IN-02, IN-18) does not block. Nothing is deferred to a later phase.

---

_Verified: 2026-10-05T22:21:47Z_
_Verifier: Claude (gsd-verifier)_
