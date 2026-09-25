---
phase: 24-artist-tags-notes
verified: 2026-09-25T03:24:53Z
status: gaps_found
score: 4/5 roadmap success criteria verified (SC2 still partial — DB cap bypassable under a concurrent detach, reproduced)
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
  - cmd/server/main.go
  - docs/adr/0004-per-artist-tag-cap-trigger.md
  - internal/db/migrate_test.go
  - internal/db/migrations/000010_tags_and_notes.down.sql
  - internal/db/migrations/000010_tags_and_notes.up.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.down.sql
  - internal/db/migrations/000011_artist_tags_cap_on_update.up.sql
  - internal/db/schema_version_test.go
  - internal/db/tags_schema_test.go
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
  - web/app/routes/watchlist.test.tsx
  - web/app/routes/watchlist.tsx
covered_digest: "v1:sha256:c2c3c08e9d9889b8da482426e8d6f15fa5eff27c6cdcb71baf77cc5490bc2269"
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: gaps_found
  previous_score: 4/5
  gaps_closed:
    - "Raw `UPDATE artist_tags SET artist_id = <10-link artist>` is now refused (000011 BEFORE UPDATE OF artist_id trigger; TestSchema_TagCapTrigger_RawUpdateRefused PASS)"
    - "Deleting a tag in Manage tags now removes it from the route vocabulary / '+ tag' autocomplete (dropTagFromEntries calls setVocabulary; route test PASS)"
  gaps_remaining:
    - "SC2 / TAG-04 database-level cap: the same truth still fails, now through a different root cause (concurrent-detach race in the shared trigger function, CR-01)"
  regressions: []
gaps:
  - truth: "SC2 / TAG-04: an 11th tag on one artist is refused ... and the database refuses it even when the API check is bypassed"
    status: failed
    severity: blocker
    reason: >-
      The shared trigger function check_artist_tags_max_per_artist() (000010,
      re-created unchanged in logic by 000011) short-circuits with a plain
      `IF EXISTS (SELECT 1 FROM artist_tags WHERE artist_id=NEW.artist_id AND
      tag_id=NEW.tag_id)`. Under MVCC that SELECT still sees a link another
      transaction has deleted but not yet committed, so the trigger returns
      NEW without taking the artist lock or counting. The INSERT's ON CONFLICT
      arbiter (or the UPDATE's PK check) then waits for the deleter, the
      deleter commits, the row is written, and the BEFORE ROW trigger is not
      re-run. A third transaction attaching a different tag in that window
      counts 9 committed links and succeeds. Independently reproduced by this
      verifier in a throwaway database (postgres:16 container) using the exact
      000011 function body and both triggers: artist at 10 links ended at 11
      for BOTH the `INSERT ... ON CONFLICT DO NOTHING` variant and the
      `UPDATE artist_tags SET artist_id` variant. Sequential control at 10 was
      refused as expected. Reachable through the API too (AttachTag is the
      only cap guard; there is no API-level count), though only with a
      microsecond window since Detach is a single autocommit statement.
    artifacts:
      - path: "internal/db/migrations/000011_artist_tags_cap_on_update.up.sql"
        issue: "Lines 13-27: both skip-existing checks are plain IF EXISTS, which a concurrent uncommitted DELETE of the same link defeats"
      - path: "internal/db/migrations/000010_tags_and_notes.up.sql"
        issue: "Lines 38-52: same logic (shipped; must not be edited)"
      - path: "docs/adr/0004-per-artist-tag-cap-trigger.md"
        issue: "Amendment claims the DB guarantee covers every way a link can reach an artist; not true under a concurrent detach"
    missing:
      - "New migration 000012 (dev DB is already at 11|f, so editing 000011 in place would never re-run there) that re-creates check_artist_tags_max_per_artist() with both skip-existing checks as `PERFORM 1 FROM artist_tags WHERE artist_id = NEW.artist_id AND tag_id = NEW.tag_id FOR KEY SHARE; IF FOUND THEN RETURN NEW; END IF;` — keep the TG_OP='UPDATE' early return and the FOR NO KEY UPDATE artist lock. This verifier ran that exact shape against the same three-session race: the third attach is refused with artist_tags_max_per_artist and the artist stays at 10 for both INSERT and UPDATE variants"
      - "Paired 000012 down file restoring the 000011 function body; bump expectedSchemaVersionOnDisk and the from-scratch (12, false) pin"
      - "Forced, pg_stat_activity-gated DB test (style of TestSchema_TagCapTrigger_SameTagConcurrentAt9): S1 deletes link T uncommitted, S2 re-attaches T (and a variant moving T by UPDATE) and blocks, S1 commits, S3 attaches U; assert S3 fails 23514/artist_tags_max_per_artist and the artist ends at 10"
      - "Confirm TestSchema_TagCapTrigger_SameTagConcurrentAt9 and TestTrigger_SkipExisting still pass (uncommitted inserts are invisible, so FOR KEY SHARE does not change them)"
      - "ADR 0004 amendment covering the concurrent-detach case — OR, if the user deliberately accepts the race window, an override (template in the report) plus an ADR note stating the guarantee is serial-only against a concurrent detach of the same link"
  - truth: "24-09 prohibition: picking a suggestion must never silently re-create a tag the user just deleted (TAG-06 / SC4 autocomplete)"
    status: partial
    severity: warning
    reason: >-
      The normal flow is fixed and tested. Residual (review WR-06, confirmed by
      reading web/app/routes/watchlist.tsx:188-197): loadVocabulary writes its
      listTags() result unconditionally on settle. If the '+ tag' vocabulary
      request is still in flight when the user opens Manage tags and deletes a
      tag, the late response overwrites the filtered vocabulary with a list that
      still contains the deleted tag and sets status 'loaded' (never refetched);
      picking it re-creates the tag. Same overwrite can undo a rename/merge. Not
      reproduced (needs a stalled request); non-blocking on its own, cheap to
      close in the same gap plan.
    artifacts:
      - path: "web/app/routes/watchlist.tsx"
        issue: "loadVocabulary has no stale-response guard; handleTagsLoaded/drop/rename/merge do not invalidate an in-flight load"
    missing:
      - "Generation ref bumped in handleTagsLoaded, dropTagFromEntries, renameTagInEntries, mergeTagInEntries; loadVocabulary applies its result (and error status) only if the generation is unchanged"
      - "Route test with a deferred listTags promise: open '+ tag', open Manage tags and delete, then resolve the first listTags with the old list; assert the deleted tag is not offered as existing"
---

# Phase 24: Artist Tags & Notes Verification Report

**Phase Goal:** The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Verified:** 2026-09-25T03:24:53Z
**Status:** gaps_found
**Re-verification:** Yes, after gap-closure plans 24-08 and 24-09

## Goal Achievement

### Observable Truths (ROADMAP success criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Type a tag on a card, pick from autocomplete or create on the fly, see a chip, remove it. Tags persist across reload and across remove + re-add | VERIFIED (regression check) | No regression. `internal/tags`, `internal/watchlist` (`TestService_Remove_LeavesArtistTagsIntact`) and `internal/httpserver` (`TestTags_*`) pass against live PG. `TestSchema_WatchlistDeleteKeepsTagLinks` passes. The full web suite passes 334/334. |
| 2 | `Reggaeton ` attaches existing `reggaeton`. Over 32 chars or an 11th tag is refused from the UI or the API, **and the DB refuses it even when the API check is bypassed** | FAILED (partial), BLOCKER | The UPDATE-path gap is closed: `TestSchema_TagCapTrigger_RawUpdateRefused` and the 5 `UpdatePaths` subtests pass, the dev DB reads `11\|f`, and both triggers are present. **But the DB cap is still bypassable.** A concurrent, uncommitted DELETE of the same link makes the plain `IF EXISTS` skip-existing check return early, with no lock and no count. I reproduced this myself and got 11 links on both the INSERT and UPDATE paths (see Spot-Checks). Identity, length, and the API/UI cap all still hold. |
| 3 | Rename once and the new name shows everywhere. Renaming onto an existing name asks for confirmation naming both tags, and nothing merges without it | VERIFIED (regression check) | Rename/merge files are unchanged since the prior verification. `Merge`-filtered tests in tags/httpserver pass. The ManageTagsDialog vitest passes under the project's `pnpm test` config (see Info note on one timing-sensitive test). |
| 4 | Delete a tag globally after a confirmation stating the carrier count. The tag disappears from every artist, and the artists stay untouched | VERIFIED | `dropTagFromEntries` (watchlist.tsx:117-129) now filters `entries` and `vocabulary`. The route test "deleting a tag in Manage tags removes it from the '+ tag' autocomplete, with no extra listTags call" passes (watchlist.test.tsx:329). The rebuilt bundle `watchlist-CWaO3Pt5.js` replaces `DQRWM9Da`. A residual stale-response race remains (WR-06, warning gap 2). It does not negate the SC as written. |
| 5 | Add, edit, and clear a plain-text note up to 500 chars, shown on the card after reload | VERIFIED (regression check) | Unchanged since the prior verification. `TestSchema_NoteChecks` passes. ArtistNote vitest passes. |

**Score:** 4/5 roadmap truths verified. 0 truths are present but behavior-unverified.

**24-08 plan must-haves:** 8 of 9 hold as stated. The UPDATE refusal, merge-shape, unchanged-artist, below-cap and two-row moves, duplicate-move PK violation, INSERT-path guarantees, 000011 round-trip, version pins/migration-check, and the ADR amendment are all present. Two failures:
- The plan's stated purpose ("the database refuses an 11th link whether it arrives by INSERT or by moving an existing link") and the ADR amendment's "covers every way a link can reach an artist" are false under concurrency (gap 1).
- The "unchanged-artist" truth holds behaviorally, but its subtest does not pin the new `TG_OP='UPDATE'` guard (WR-07, confirmed). `SET artist_id = artist_id` hits the first EXISTS on its own row and returns before the guard matters.

**24-09 plan must-haves:** all 6 truths are verified. The no-extra-GET, the non-deleted tags still offered, the existing route tests, the rebuilt bundle, and the green web suite were all re-run by me. The prohibition "never silently re-create a just-deleted tag" is flagged partial because of WR-06 (gap 2).

### CR-01 independent assessment

I did not take the reviewer's claim on trust. My reasoning and evidence:

- **Postgres semantics.** Under READ COMMITTED, each statement in the plpgsql function takes a fresh snapshot. A row whose `xmax` belongs to an in-progress deleter is still *visible*, so `EXISTS` returns true. The unique/arbiter check (`ON CONFLICT` speculative insert, or the PK index insert for the UPDATE's new tuple version) waits on the deleter's xid. After the deleter commits, the conflict is gone and the write proceeds. BEFORE ROW triggers run once, before that retry loop, and are not re-fired. `DELETE FROM artist_tags` takes no lock on `artists`, and the waiting writer's FK `KEY SHARE` does not conflict with a third session's `FOR NO KEY UPDATE`. So the third session's count sees 9 and passes.
- **Reproduction.** I created a throwaway DB `verify_cr01_scratch` on the project's `postgres:16` container, loaded the exact 000011 up file plus the INSERT trigger, seeded artist 1 at 10 links and artist 2 carrying tag 1, and ran three psql sessions:
  - S1 `DELETE (1,1)` then sleep, then commit.
  - S2 `INSERT (1,1) ON CONFLICT DO NOTHING` blocked until S1 committed, returned at +0.3 ms, then slept before committing.
  - S3 `INSERT (1,11)` ran after S1's commit and succeeded while S2 was still open.
  - Final count for artist 1 was **11**. The UPDATE variant (`SET artist_id=1 WHERE artist_id=2 AND tag_id=1`) also ended at **11**. A sequential control at 10 was refused. The scratch DB was dropped, and the dev DB was not touched.
- **Fix check.** I swapped in the reviewer's `PERFORM ... FOR KEY SHARE; IF FOUND` shape and re-ran both variants. S3 was refused with `artist_tags_max_per_artist`, and the artist stayed at 10 in both cases.
- **Classification: BLOCKER against SC2/TAG-04.** SC2's clause is specifically about the database refusing the 11th link when the API is bypassed. Raw concurrent SQL is such a bypass, and it produces 11 links deterministically. This is the same standard the prior verification applied to the raw-UPDATE hole. The practical risk through the API is very low: it needs a detach of T, a re-attach of T, and an attach of U, all concurrent on one artist, inside a microsecond autocommit window, in a single-user app. If you judge that acceptable, use the override template below instead of a fix.

### WR-06 assessment (against TAG-06)

Confirmed by code reading. `loadVocabulary` (watchlist.tsx:188-197) has no generation/stale guard, and nothing invalidates an in-flight load. TAG-06 itself (delete globally with a count confirmation) is satisfied. The residual only violates the 24-09 prohibition, and only when the first `GET /tags` outlives both the Manage-tags fetch and a confirmed delete. I rate it **WARNING**, not a blocker, and list it as gap 2 so it gets closed in the same plan.

### Required Artifacts

| Artifact | Status | Details |
|----------|--------|---------|
| `internal/db/migrations/000011_artist_tags_cap_on_update.{up,down}.sql` | VERIFIED (with gap) | `artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id` exists. The down body is byte-identical to 000010's function (diffed). The race is inherited in the up body. |
| `internal/db/migrations/000010_*` | VERIFIED | Unchanged since 43c1d56, as the prohibition requires. |
| `internal/db/tags_schema_test.go` | VERIFIED | `RawUpdateRefused`, `UpdatePaths` (5 subtests), and `Migration000011_DownUpRoundTrip` all pass. There is no test for the concurrent-detach interleaving. |
| `internal/db/{schema_version,migrate}_test.go` | VERIFIED | `expectedSchemaVersionOnDisk = 11`. From-scratch expects `(11, false)`. |
| `docs/adr/0004-per-artist-tag-cap-trigger.md` | VERIFIED (claim overstated) | The amendment is present, but "covers every way a link can reach an artist" does not hold under concurrency. |
| `web/app/routes/watchlist.tsx` / `.test.tsx` | VERIFIED | Vocabulary filter on delete, plus a new route test. |
| `internal/webassets/build/client` | VERIFIED | Rebuilt in 54dab01 (new watchlist and manifest hashes). The bundle contains the Manage tags / delete strings. |
| All other phase-24 artifacts | VERIFIED (regression) | Unchanged since the prior verification. Their tests pass. |

### Key Link Verification

| From | To | Via | Status |
|------|----|-----|--------|
| 000011 update trigger | `check_artist_tags_max_per_artist()` → `CONSTRAINT = 'artist_tags_max_per_artist'` → `ErrTagCapReached` | same literal | WIRED |
| merge `UPDATE ... SET tag_id` | does not fire `UPDATE OF artist_id` | column-scoped trigger | WIRED (merge-shape subtest, `Merge` tests pass) |
| 000011 down | 000010 function body | byte-identical | WIRED (diff clean, round-trip test passes) |
| ManageTagsDialog `onDeleted` | `dropTagFromEntries` → `setEntries` + `setVocabulary` | route updater | WIRED (was PARTIAL) |
| `loadVocabulary` | route `vocabulary` | unguarded async set | WIRED, with a stale-overwrite hazard (gap 2) |
| trigger skip-existing check | artist lock + count | plain `IF EXISTS` | BROKEN under concurrent delete (gap 1) |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real data | Status |
|----------|------|--------|-----------|--------|
| TagCombobox | `vocabulary` | GET /tags (lazy), patched locally on create, rename, merge, and now delete | yes | FLOWING (a stale in-flight response can clobber it, gap 2) |
| TagChips / ManageTagsDialog / ArtistNote | unchanged | unchanged | yes | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| DB tag/cap/note schema tests | `go test ./internal/db -run 'TestSchema_Tag\|TestTrigger\|TestSchema_Migration000011\|TestSchema_NoteChecks\|TestSchema_WatchlistDeleteKeepsTagLinks\|TestExpectedSchemaVersion'` (live PG) | 11/11 PASS, including the 5 UpdatePaths subtests | PASS |
| Merge/cap/HTTP regression | `go test ./internal/tags ./internal/watchlist ./internal/httpserver -run 'Merge\|Remove_LeavesArtistTagsIntact\|TestTags_\|Cap'` | ok ×3 | PASS |
| Dev DB state | `select version,dirty from schema_migrations`, `pg_trigger` | `11\|f`, both cap triggers present | PASS |
| migration-check | `go run ./cmd/migration-check --mode scan` | no findings | PASS |
| go vet | `go vet ./...` | exit 0 | PASS |
| Full web suite | `corepack pnpm test` | 23 files, 334/334. Coverage 92/85/92/94 | PASS |
| Delete clears autocomplete | watchlist.test.tsx:329 (in the suite above) | PASS | PASS |
| **DB cap under concurrent detach (INSERT)** | 3 psql sessions, scratch DB, exact 000011 body | artist 10 → **11**, no error | **FAIL (gap 1)** |
| **DB cap under concurrent detach (UPDATE)** | same, S2 = `UPDATE ... SET artist_id` | artist 10 → **11**, no error | **FAIL (gap 1)** |
| Proposed FOR KEY SHARE fix | same race, patched function | S3 refused, artist stays 10 (both variants) | PASS (fix validated) |

golangci-lint was not on PATH in this session, so I did not re-run it. The 24-09 SUMMARY claims it passed after a reinstall.

### Probe Execution

No probes are declared for this phase, so Step 7c is not applicable.

### Requirements Coverage

| Requirement | Source Plan | Status | Evidence |
|-------------|-------------|--------|----------|
| TAG-01 | 24-01, 02, 04, 05 | SATISFIED | unchanged; tests pass |
| TAG-02 | 24-01, 04 | SATISFIED | unchanged |
| TAG-03 | 24-01, 02, 05 | SATISFIED | unchanged |
| TAG-04 | 24-01, 05, 08 | **BLOCKED (partial)** | The API and UI enforce both caps, and the DB enforces length. The DB count cap now covers INSERT and UPDATE OF artist_id, but a concurrent detach defeats it (gap 1). |
| TAG-05 | 24-02, 06 | SATISFIED | unchanged |
| TAG-06 | 24-02, 06, 09 | SATISFIED | delete + count confirm; vocabulary now synced (WR-06 residual is a warning) |
| TAG-07 | 24-01, 03, 04 | SATISFIED | unchanged |
| NOTE-01 | 24-03, 07 | SATISFIED | unchanged |

All 8 IDs are claimed by at least one plan, and none are orphaned. **REQUIREMENTS.md is inconsistent:** it marks TAG-04 `[x]` / Complete (lines 14 and 87), which is premature given gap 1. The other six are shown as "Gaps Found". The orchestrator should reconcile these marks with this verdict.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| migrations 000011 (and 000010) | 13-27 (38-52) | visibility-only `IF EXISTS` used as a concurrency guard | Blocker | gap 1 (CR-01) |
| web/app/routes/watchlist.tsx | 188-197 | unguarded async state write | Warning | gap 2 (WR-06) |
| internal/db/tags_schema_test.go | 445-461 | subtest passes without the guard it claims to pin | Warning | WR-07. The `TG_OP='UPDATE'` early return is untested. Changing `tag_id` too would pin it. |
| web/app/components/watchlist/ManageTagsDialog.test.tsx | 366-379 | timing-sensitive `waitFor` (default 1000 ms) | Info | The "Esc on the merge confirm..." test fails 3/3 when the file runs with `--coverage.enabled=false`, passes alone, and passes under the project's `pnpm test`. The file is unchanged since 24-06. This is flaky-test hygiene, not a phase gap. |
| queries/tags.sql:18, internal/tags/service.go:14,35 | — | comments cite only 000010 for the trigger | Info | IN-11 |
| — | — | TBD/FIXME/XXX/TODO/HACK in files changed by 24-08/09 | none | — |

Carried warnings WR-02, WR-04, and WR-05 are unchanged and are not must-haves (see the prior report).

### Human Verification Required

These items are carried forward and still open once the gaps close. The phase moves to `human_needed` after gap closure until they are done.

1. **End-to-end tags + note CRUD in a real browser.** Create, pick, and remove chips. Reload. Remove the artist and re-add it: the tags should return. Add, edit, and clear a note, then reload. Delete a tag in Manage tags, then open "+ tag": the deleted tag should not be offered as existing.
2. **Combobox popup overflow (24-05 backstop).** With 30+ tags, a 32-char name, and a narrow viewport, the popup should scroll inside its own bounds and no option should be clipped.
3. **Long tag chip at 375px (24-04 backstop).** The chip truncates, and `title` plus the × aria-label carry the full name.
4. **Merge title wrap at 375px (24-06 backstop).** Two 32-char names should wrap, not truncate.
5. **Manage tags row truncation at 375px (24-06 backstop).** `· {n} artists` should stay visible.
6. **UI-SPEC contrast/hit-area audit** of the chips, the ×, "+ tag", and the note pencil.
7. **Delete-vs-attach race (24-02 backstop).** No `artist_tags` row should reference a deleted tag. The FK cascade guarantees this structurally. Confirm or accept.

### Gaps Summary

The gap-closure plans did what they said:
- Raw `UPDATE ... SET artist_id` is now refused.
- Merges are unaffected.
- The migration pair round-trips.
- Deleting a tag now clears it from the autocomplete.

All of this is backed by passing tests I re-ran.

SC2's DB clause still fails, though, for a root cause neither the plan nor the prior verification caught. The trigger's skip-existing checks rely on visibility, not locking, so a concurrent uncommitted detach of the same link lets a writer skip the artist lock, and a third writer then counts only 9. I reproduced 11 links on both the INSERT and UPDATE paths, and confirmed that the `FOR KEY SHARE` rewrite closes the hole. The fix belongs in a new migration 000012, because the dev DB is already at version 11. Add a forced interleaving test and an ADR note alongside it. WR-06 (stale vocabulary response) and WR-07 (test that does not pin the guard) are cheap to fold into the same plan.

**If you accept the concurrency window rather than fixing it**, add this to the frontmatter and amend ADR 0004:

```yaml
overrides:
  - must_have: "the database refuses an 11th tag on one artist even when the API check is bypassed"
    reason: "Cap holds for all serial writes and concurrent attaches; the only bypass needs a concurrent uncommitted detach of the same link plus a third attach in the window. Accepted as out of scope for a single-user app; ADR 0004 documents the limitation."
    accepted_by: "<name>"
    accepted_at: "<ISO timestamp>"
```

**Deferred:** none. Phase 26 SC2 reuses the per-artist cap for bulk add but does not commit to fixing this race. This gap stays in Phase 24.

---

_Verified: 2026-09-25T03:24:53Z_
_Verifier: Claude (gsd-verifier)_
