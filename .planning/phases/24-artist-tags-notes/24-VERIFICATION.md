---
phase: 24-artist-tags-notes
verified: 2026-09-23T12:00:00Z
status: gaps_found
score: 4/5 roadmap success criteria verified (SC2 partial — DB cap bypassable via raw UPDATE)
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
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/migrations/000010_tags_and_notes.down.sql
  - internal/db/migrations/000010_tags_and_notes.up.sql
  - internal/httpserver/server.go
  - internal/httpserver/tags.go
  - internal/httpserver/watchlist.go
  - internal/tags/normalize.go
  - internal/tags/service.go
  - internal/watchlist/service.go
  - queries/tags.sql
  - queries/watchlist.sql
  - web/app/components/common/ConfirmDialog.tsx
  - web/app/components/watchlist/ArtistNote.tsx
  - web/app/components/watchlist/ManageTagsDialog.tsx
  - web/app/components/watchlist/TagChips.tsx
  - web/app/components/watchlist/TagCombobox.tsx
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/lib/api.ts
  - web/app/lib/tags.ts
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:f6dc9e55beeb029189341aea4e2dd88bc1172a1cc48dba9acc2dd993c2447050"
behavior_unverified: 0
overrides_applied: 0
gaps:
  - truth: "SC2 / TAG-04: an 11th tag on one artist is refused ... and the database refuses it even when the API check is bypassed"
    status: partial
    reason: >-
      The cap trigger artist_tags_cap_trigger is BEFORE INSERT only. A raw
      `UPDATE artist_tags SET artist_id = <artist at 10 links>` bypasses it.
      Reproduced live against the dev Postgres in a rolled-back transaction:
      artist at 10 links -> UPDATE moves one link from another artist -> count 11,
      no error. The INSERT/COPY path, the 32-char CHECK, and the API 409 all hold
      (tests pass); only the UPDATE-of-artist_id path is open. ADR 0004 argues no
      code path does this today, but that is an application promise, not the
      DB-level guarantee SC2 states. (Code review WR-03.)
    artifacts:
      - path: "internal/db/migrations/000010_tags_and_notes.up.sql"
        issue: "Trigger declared `BEFORE INSERT ON artist_tags` only (line 64-67); no UPDATE OF artist_id coverage"
    missing:
      - "New additive migration 000011 (000010 is shipped and must not be edited): CREATE OR REPLACE check_artist_tags_max_per_artist() with an early `IF TG_OP = 'UPDATE' AND NEW.artist_id = OLD.artist_id THEN RETURN NEW; END IF;`, plus `CREATE TRIGGER artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id ON artist_tags FOR EACH ROW EXECUTE FUNCTION check_artist_tags_max_per_artist();` (merge's UPDATE ... SET tag_id must stay unaffected)"
      - "DB test in internal/db/tags_schema_test.go: raw UPDATE moving a link onto a 10-link artist fails with SQLSTATE 23514 / constraint artist_tags_max_per_artist; a merge on a 10-tag artist still succeeds"
      - "ADR 0004 amendment noting UPDATE coverage — OR, if the user deliberately accepts insert-only scope, an override (see report) plus an ADR amendment stating the guarantee covers inserts only"
  - truth: "Plan 24-06 key link: ManageTagsDialog onDeleted -> dropTagFromEntries + vocabulary updates -> every card and the '+ tag' autocomplete reflect the delete"
    status: partial
    reason: >-
      dropTagFromEntries (web/app/routes/watchlist.tsx:121-130) filters only
      `entries`; the route `vocabulary` state is never updated on delete (rename
      and merge both patch it). vocabularyStatus stays "loaded", so it is never
      refetched. After a delete, every row's "+ tag" autocomplete still offers
      the deleted tag as an existing suggestion, and picking it silently
      re-creates the tag. SC4's literal "disappears from every artist" holds;
      this is a warning-level wiring gap, not a blocker. (Code review WR-01.)
    artifacts:
      - path: "web/app/routes/watchlist.tsx"
        issue: "dropTagFromEntries does not call setVocabulary"
    missing:
      - "Add `setVocabulary((v) => (v ? v.filter((t) => t.id !== tagId) : v))` to dropTagFromEntries"
      - "Route test: delete a tag via Manage tags, open '+ tag' on a row, assert the deleted name is not offered as an existing suggestion"
---

# Phase 24: Artist Tags & Notes Verification Report

**Phase Goal:** The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Verified:** 2026-09-23
**Status:** gaps_found (one narrow DB-guarantee gap plus one warning-level wiring gap)
**Re-verification:** No (initial verification)

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it; tags persist across reload and across remove + re-add | VERIFIED | TagChips/TagCombobox wired in WatchlistRow; attachTag/detachTag go through apiFetch to POST/DELETE `/watchlist/{id}/tags`, registered in `registerDataRoutes` (server.go:288-289); `tags.NewService(pool)` wired in main.go:313. GET /watchlist projects `tag_ids`/`tag_names` in its single query. `artist_tags` keys on `artists.id`, and `Remove` deletes only the watchlist row, with no `DELETE FROM artists` anywhere. Ran `TestSchema_WatchlistDeleteKeepsTagLinks` and `TestService_Remove_LeavesArtistTagsIntact`: PASS. |
| 2 | `Reggaeton ` attaches existing `reggaeton` (first casing kept, no near-duplicate); >32 chars or 11th tag refused with a clear message from UI or API; **DB refuses even when API bypassed** | FAILED (partial) | Identity: `NormalizeName` (NFC, trim, collapse) + `ON CONFLICT ((lower(name))) DO UPDATE SET name = tags.name` keeps the first casing. Client `normalizeTagKey` pins the exact match. Length: API returns 400, `tags_name_length` CHECK exists, and the UI enforces `maxLength` plus a toast. Cap: API returns 409, the UI shows the "max 10 tags" hint and toast, and a BEFORE INSERT trigger exists. Ran the DB tests for raw insert refused, skip-existing, same-tag concurrency at 9, name checks, and lower-unique: PASS. **Gap:** a raw `UPDATE artist_tags SET artist_id` gave an artist 11 tags with no error (reproduced live, rolled back). |
| 3 | Rename once, new name everywhere; rename onto existing name asks for confirmation naming both tags; nothing merges without it | VERIFIED | `Service.Rename` detects collisions from the unique violation and returns `CollisionError`, and the handler returns 409 with `target` + `carrier_count_after_merge`. `mergeTag` has exactly one caller, `handleConfirmMerge` (ManageTagsDialog.tsx:199), which runs only from the ConfirmDialog's `onConfirm`. The ConfirmDialog title names both tags. `renameTagInEntries` updates every card. Backend tests (`TestService_Rename_*`, `TestService_Merge_*`, `TestTags_MergeEndToEnd`) and ManageTagsDialog vitest: PASS. |
| 4 | Delete a tag globally after a confirmation stating how many artists carry it; tag disappears from every artist; artists untouched | VERIFIED | `DeleteTagCountingCarriers` does a CTE count, then deletes, and the FK cascade removes the links. The ConfirmDialog says `Delete “{name}” from {n} artist{s}?`, and `dropTagFromEntries` removes the chip from every card. `TestTags_DeleteEndToEnd_LeavesWatchlistUntouched` PASS. Warning: the stale autocomplete vocabulary (gap 2) does not violate this SC as written. |
| 5 | Add, edit, clear a plain-text note up to 500 chars; shows on the card after reload | VERIFIED | ArtistNote → `updateNote` (PUT `/watchlist/{id}/note`, server.go:272) → `onEntryChange`. `watchlist.note` carries a `char_length <= 500` + not-blank CHECK. GET /watchlist projects `w.note`. It renders as plain JSX (no `dangerouslySetInnerHTML` anywhere in web/app). Note tests in httpserver/watchlist and ArtistNote vitest: PASS. |

**Score:** 4/5 roadmap truths verified. 0 truths are present but behavior-unverified.

Plan-level must_haves (about 100 truths across 7 plans) were spot-checked against code and named tests. Nearly all plan truths are backed by passing named tests. The two exceptions are the gaps above and the `verification: backstop` visual items routed to human verification below.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/db/migrations/000010_tags_and_notes.{up,down}.sql` | VERIFIED (with gap) | tags, artist_tags, lower() unique index, cap trigger (INSERT only), watchlist.note with inline CHECKs |
| `internal/tags/{normalize,service}.go` | VERIFIED | Attach/Detach/List/Rename/Delete/Merge; merge never inserts |
| `internal/httpserver/tags.go` | VERIFIED | 6 handlers, fixed error bodies, 409 collision body |
| `queries/tags.sql`, `queries/watchlist.sql` | VERIFIED | single-query tag projection; note update |
| `internal/watchlist/service.go` | VERIFIED | NormalizeNote, UpdateNote, Add with note, shared projection |
| `web/app/components/watchlist/TagChips.tsx`, `TagCombobox.tsx` | VERIFIED | wired in WatchlistRow |
| `web/app/components/watchlist/ManageTagsDialog.tsx`, `common/ConfirmDialog.tsx` | VERIFIED | wired in route header |
| `web/app/components/watchlist/ArtistNote.tsx` | VERIFIED | wired in WatchlistRow |
| `internal/webassets/build/client` | VERIFIED | `assets/watchlist-DQRWM9Da.js` contains "Manage tags" and "add note" |
| `docs/adr/0004-per-artist-tag-cap-trigger.md` | VERIFIED | Documents insert-only scope, which is the source of gap 1 |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| trigger RAISE `artist_tags_max_per_artist` | `mapTagError` → `ErrTagCapReached` → 409 | constraint-name literal | WIRED |
| `tags_name_lower_idx` | `GetOrCreateTag ON CONFLICT ((lower(name)))` | expression index inference | WIRED |
| `registerDataRoutes` | all 7 new routes (tags + note) | inherits gate.Authenticate + CSRF | WIRED (gated 401/403 tests pass) |
| main.go | `httpserver.WithTags(tags.NewService(pool))` | option | WIRED |
| Rename 409 body | merge ConfirmDialog → `mergeTag` | `RenameTagResult.collision` | WIRED |
| ManageTagsDialog onRenamed/onMerged | entries + vocabulary | route updaters | WIRED |
| ManageTagsDialog onDeleted | entries + **vocabulary** | `dropTagFromEntries` | PARTIAL (entries only; gap 2) |
| Undo | `addWatchlist({... note: entry.note})` | POST /watchlist optional note | WIRED |
| ArtistNote save | `updateNote` (PUT) → `onEntryChange` | apiFetch | WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| TagChips | `entry.tags` | ListWatchlist ARRAY subqueries over artist_tags/tags | yes | FLOWING |
| TagCombobox | `vocabulary` | GET /tags (`ListTags`), lazy | yes | FLOWING (stale after delete, gap 2) |
| ManageTagsDialog | tag rows + carrier_count | GET /tags every open | yes | FLOWING |
| ArtistNote | `entry.note` | `w.note` in ListWatchlist | yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| DB schema/trigger tests | `go test ./internal/db -run 'TestSchema_Tag…|TestTrigger_SkipExisting|TestSchema_NoteChecks|TestSchema_WatchlistDeleteKeepsTagLinks'` (live PG) | 7/7 PASS | PASS |
| Tags survive remove | `go test ./internal/watchlist -run TestService_Remove_LeavesArtistTagsIntact` | PASS | PASS |
| tags package | `go test ./internal/tags` (live PG) | ok, no skips | PASS |
| HTTP tag + note routes | `go test ./internal/httpserver -run 'TestTags_|Note'` | ok, 45 PASS | PASS |
| Frontend tag/note units | `vitest run ManageTagsDialog TagCombobox ArtistNote lib/tags` | 55/55 | PASS |
| DB cap under raw UPDATE | psql, `BEGIN; … UPDATE artist_tags SET artist_id=<10-link artist> …; ROLLBACK` | count went 10 → 11, no error | FAIL (gap 1) |

### Probe Execution

No probes declared for this phase (`scripts/*/tests/probe-*.sh` not referenced). Step 7c is not applicable.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | attach + autocomplete + create on the fly |
| TAG-02 | 24-01, 04 | SATISFIED | idempotent detach, chip × with rollback |
| TAG-03 | 24-01, 02, 05 | SATISFIED | NFC/trim/collapse + lower() identity, first casing kept |
| TAG-04 | 24-01, 05 | PARTIAL | API and UI enforce both caps. The DB enforces length fully but the count cap only on INSERT (gap 1). |
| TAG-05 | 24-02, 06 | SATISFIED | rename, 409 collision, confirmed merge |
| TAG-06 | 24-02, 06 | SATISFIED | delete with watched-carrier count confirmation |
| TAG-07 | 24-01, 03, 04 | SATISFIED | artist-keyed links survive remove/re-add |
| NOTE-01 | 24-03, 07 | SATISFIED | PUT note, ≤500, clear, shown on card |

All 8 phase requirement IDs are claimed by at least one plan. No requirements are orphaned. REQUIREMENTS.md marks all 8 as Complete, but TAG-04's "enforced by both API and DB" is only partially true (gap 1).

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | TBD/FIXME/XXX/TODO/HACK/placeholder | none found | — |
| web/app/routes/watchlist.tsx | 121-130 | delete does not update vocabulary | Warning | gap 2 (WR-01) |
| web/app/routes/watchlist.tsx / TagChips.tsx | 53-58 | wholesale `setEntries` on refresh can undo a just-settled chip add/remove | Warning | WR-02. Not a stated must-have; the chip self-corrects on the next refresh. |
| TagCombobox/ArtistNote/ManageTagsDialog | various | `maxLength`/`.length` count UTF-16 units | Warning | WR-04: over-restricts emoji/astral input. It never lets over-cap input through. |
| internal/httpserver/tags_test.go | 861-916 | `t.Fatalf` from goroutines | Info | WR-05: test hygiene only |
| migration 000010 | 14 | comment says "accent-insensitive" | Info | IN-01 |

### Human Verification Required

These are browser-only checks, still open once the gaps are closed. The phase goes to `human_needed` after gap closure until they are done.

1. **End-to-end tags + note CRUD in a real browser.** Add a tag via "+ tag" (create and pick), remove a chip, reload, remove the artist, re-add it from search, and confirm the tags return. Then add, edit, and clear a note and reload. Expected: the card state matches after each reload. Why human: full SPA-to-server flow with real focus and toast behavior.
2. **Combobox popup overflow (24-05 backstop).** With 30+ vocabulary tags and a 32-char name in a narrow viewport, the popup should scroll inside its own bounded list, with no clipped option text. Why human: layout.
3. **Long tag chip at 375px (24-04 backstop).** A 32-char unbroken tag should truncate in its chip, expose the full name via `title` and the × aria-label, and un-truncate while × has focus. Why human: layout.
4. **Merge title wrap at 375px (24-06 backstop).** A merge ConfirmDialog naming two 32-char tags should wrap and never truncate. Why human: layout.
5. **Manage tags row truncation at 375px (24-06 backstop).** A 32-char name should truncate with `title`, and `· {n} artists` should stay visible. Why human: layout.
6. **UI-SPEC contrast/hit-area checks** for chips, the ×, "+ tag", and the note pencil. Why human: visual/a11y audit.
7. **Delete-vs-attach race (24-02 `verification: backstop`).** No `artist_tags` row should ever reference a deleted tag. The FK `ON DELETE CASCADE` guarantees this structurally, but no forced-race test exists. This is low risk; confirm or accept.

### Gaps Summary

The phase delivers the goal from the user's side. Tags, autocomplete, chips, case/whitespace identity, global rename/merge/delete, notes, and remove/re-add persistence are all implemented, wired end to end, and backed by passing tests against live Postgres.

Two gaps remain:

1. **DB cap is INSERT-only (blocking against SC2 as written).** SC2 explicitly says the database refuses an 11th tag "even when the API check is bypassed". A plain `UPDATE artist_tags SET artist_id = …` is such a bypass, and it was reproduced producing 11 links. The fix is one small additive migration (000011) plus one DB test. The ADR chose insert-only scope on purpose. If the user accepts that scope as the intended meaning of SC2, record an override instead:

   ```yaml
   overrides:
     - must_have: "the database refuses an 11th tag on one artist even when the API check is bypassed"
       reason: "ADR 0004 scopes the DB guarantee to link creation (INSERT/COPY); no code path moves links between artists, and merge uses UPDATE of tag_id only. Amend ADR 0004 to state the insert-only scope."
       accepted_by: "<name>"
       accepted_at: "<ISO timestamp>"
   ```

2. **Deleted tag lingers in the "+ tag" autocomplete (warning).** This is the plan 24-06 key link "onDeleted → … + vocabulary updates", which is only half wired. The fix is a one-line `setVocabulary` filter plus a route test. It is cheap to close in the same gap plan.

Neither gap is covered by a later phase (Phases 25-28 do not touch the cap trigger or the delete→vocabulary path), so neither is deferred.

---

_Verified: 2026-09-23_
_Verifier: Claude (gsd-verifier)_
