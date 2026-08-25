---
status: resolved
trigger: "the ordering looks right but there doesnt seem to be the artist label when its a guest feature"
created: 2026-08-25T18:22:07.212Z
updated: 2026-08-25T19:10:00.000Z
---

## Current Focus
<!-- OVERWRITE on each update - always reflects NOW -->

hypothesis: CONFIRMED. Migration 000006 added `watched_artist_name` as a nullable column with no backfill, so all 1613 pre-existing rows carry NULL. `InsertEvent`'s `ON CONFLICT (event_type, source, external_id) DO NOTHING` then guarantees re-detection can never populate them. All 244 guest_feature rows are therefore permanently NULL, and `watchlistNote()`'s `if (!watched ...) return null` suppresses the label on every card.
test: Query the live dev Postgres for schema_migrations.version and the NULL-count of watched_artist_name per event_type.
expecting: version=6 (migration applied) AND count(watched_artist_name)=0 (no row ever populated) confirms the data-state cause and rules out both the stale-migration and stale-binary alternatives.
next_action: NONE — session closed. Human verification received 2026-08-25T19:10Z ("confirmed fixed": user hard-refreshed the History tab and confirmed guest_feature cards now render "— <watched artist> is on your watchlist"). Migrations 000007 up/down + migrate_test.go committed, session archived to .planning/debug/resolved/, knowledge-base entry appended.
bug_class: Bohrbug (deterministic — every guest_feature card is missing the label 100% of the time)
reasoning_checkpoint:
  hypothesis: "Migration 000006 added watched_artist_name without a backfill, and InsertEvent's ON CONFLICT DO NOTHING makes the column write-once, so all 244 pre-existing guest_feature rows are permanently NULL and the UI correctly suppresses the label."
  confirming_evidence:
    - "schema_migrations.version = 6, dirty = f — migration IS applied, column exists"
    - "SELECT count(watched_artist_name) FROM events = 0 across all 1613 rows — not one row was ever populated"
    - "Newest event created_at 17:56:46Z predates the current app container start 18:14:39Z — no detection cycle has run against the new binary"
    - "watchlistNote() returns null when `!watched`, so NULL deterministically suppresses the note"
  falsification_test: "If any row had a non-NULL watched_artist_name while its card still showed no label, the cause would be render/serialization, not data state. count()=0 rules that out."
  fix_rationale: "The value is exactly recoverable: detection sets ArtistID and watchedName from the same watchlist entry, so watched_artist_name === artists.name for the row's artist_id. A backfill migration restores precisely what the new insert path would have written — it repairs the data state rather than papering over it in the UI."
  blind_spots: "The forward insert path has not been observed writing a non-NULL value on live data (no detection cycle since restart); it is covered only by unit tests and code reading. Backfill is verified against current data only."
  candidate_causes:
    - "data: pre-existing rows NULL because ADD COLUMN did not backfill (CONFIRMED)"
    - "code: insert path fails to populate the field (REFUTED — watchedName := entry.Name at all 4 call sites, $14 positional binding correct, unit tests green)"
    - "environment: migration unapplied or stale binary/frontend build (REFUTED — schema_migrations=6 clean, container rebuilt 18:14)"
  and_gate: "YES — two conditions are simultaneously required. (1) 000006 added the column with no backfill, AND (2) InsertEvent uses ON CONFLICT DO NOTHING (write-once, deliberately not the COALESCE-refresh shape UpsertArtist uses). Either alone is survivable: a backfill would have populated history, and a refresh-on-conflict upsert would have healed rows on the next detection sweep. Only the combination makes the column permanently NULL."
tdd_checkpoint: null

## Symptoms
<!-- Written during gathering, then immutable -->

expected: On a guest_feature History card (e.g. a Lil Durk track featuring Lil Baby, where Lil Baby is the watchlisted artist), a line near "Featured on {title}" should name the watchlisted artist (Lil Baby) whenever it differs from the card's main artist_name (Lil Durk).
actual: The watched-artist label does not appear on guest_feature cards at all. User confirms the separate release-chronology ordering fix from the same quick task IS working correctly — only the label is missing.
errors: none reported by user
reproduction: Open the History tab in the running app and look at a guest_feature-type event card.
started: Immediately after quick task 260825-g6i shipped this session (commits f02f544..68a387b on main, merged just before this report).

## Eliminated
<!-- APPEND only - prevents re-investigating after /clear -->

## Evidence
<!-- APPEND only - facts discovered during investigation -->

- timestamp: 2026-08-25T18:22:07.212Z
  checked: Quick task 260825-g6i's own verification (executor SUMMARY.md) — full Go test suite green, frontend typecheck + 67/67 tests passing, go vet/golangci-lint clean.
  found: All automated tests pass, including EventCard.test.tsx and events_test.go which exercise watched_artist_name.
  implication: The bug is very unlikely to be in code logic that unit tests already cover directly — more likely an environment/data issue (migration not applied, stale server/build, or pre-existing rows without the new column populated), or a gap the unit tests didn't cover (e.g. real end-to-end wiring, actual DB state).

- timestamp: 2026-08-25T18:30:00.000Z
  checked: Full source round-trip for watched_artist_name — queries/events.sql ListEvents SELECT list, sqlc events.sql.go InsertEvent ($14 binding) and ListEvents scan, internal/events/service.go:41+197, web/app/lib/api.ts:21, EventCard.tsx watchlistNote()/GuestFeatureBody.
  found: Every layer is correct and consistent. ListEvents selects watched_artist_name; InsertEvent binds arg.WatchedArtistName positionally as $14 matching the column list; service.go maps row.WatchedArtistName into the JSON Event; api.ts types it; watchlistNote() is called only from GuestFeatureBody and returns null when the value is falsy or equals artist_name.
  implication: No code defect in the read or render path. Eliminates the "typo/comparison bug in EventCard" branch from the prior next_action.

- timestamp: 2026-08-25T18:32:00.000Z
  checked: Live dev Postgres (drop-tracker-postgres-1, host port 5432) — schema_migrations version, and per-event_type counts of populated watched_artist_name.
  found: version=6, dirty=f (migration 000006 IS applied). Counts: guest_feature 244 rows / 0 populated; new_release 1369 rows / 0 populated. Not a single row in the table has a non-NULL watched_artist_name.
  implication: ROOT CAUSE. The column exists but is universally NULL, so watchlistNote() correctly returns null on every card. Eliminates the "migration never applied" branch.

- timestamp: 2026-08-25T18:34:00.000Z
  checked: Newest/oldest events.created_at vs. the app container's StartedAt, and the detection write path (internal/detection/musicbrainz.go:269-279, deezer.go:92).
  found: Newest event 2026-08-25 17:56:46Z; app container started 2026-08-25 18:14:39Z. No event has been inserted since the current binary started. Write path sets `watchedName := entry.Name` (the watchlist entry's own name) with `ArtistID: entry.ArtistID` from that same entry.
  implication: Every existing row was written by the pre-fix binary. Combined with InsertEvent's `ON CONFLICT (event_type, source, external_id) DO NOTHING`, re-detection can never backfill these rows — the NULL is permanent, not a "wait for the next poll" situation. Eliminates the "stale binary/build" branch.

- timestamp: 2026-08-25T18:36:00.000Z
  checked: Backfill recoverability and safety — JOIN events to artists on artist_id, comparing artist_name to artists.name per event_type, plus an orphan-row check.
  found: 0 events have no matching artist row. new_release: 1369/1369 have artist_name = artists.name. guest_feature: 244/244 have artist_name <> artists.name (e.g. artist_name "Chase B"/"Jeezy"/"Future", artists.name "Gunna").
  implication: watched_artist_name is exactly reconstructible as artists.name for the row's artist_id — identical to what the detection code writes, since ArtistID and watchedName come from the same watchlist entry. Backfill preserves 000006's stated invariant: equal to artist_name for new_release (note stays hidden), distinct for guest_feature (note renders).

## Resolution
<!-- OVERWRITE as understanding evolves -->

root_cause: |
  Two design decisions combined (AND-gate), neither sufficient alone:
  (1) Migration 000006 added events.watched_artist_name as a nullable column
      with no backfill, leaving all 1613 pre-existing rows NULL; AND
  (2) InsertEvent is ON CONFLICT (event_type, source, external_id) DO NOTHING
      (D-20's deliberate write-once snapshot semantics, explicitly not the
      COALESCE-refresh shape UpsertArtist uses), so re-detecting an
      already-seen recording updates nothing.
  Together these make the NULL permanent rather than self-healing: all 244
  guest_feature rows would never receive a value, and EventCard's
  watchlistNote() correctly returns null for a falsy watched_artist_name, so
  the "<X> is on your watchlist" note was suppressed on every card. The
  feature was not broken in code — it was correct code with no data to act on.
  The release-chronology ordering fix from the same quick task appeared to
  work because it is a pure read-side query change requiring no new column data.
fix: |
  Added migration 000007_backfill_events_watched_artist_name, a data repair
  that sets watched_artist_name from artists.name via events.artist_id for
  rows where it IS NULL. This is a reconstruction, not a guess: both detection
  call sites build the row from one watchlist entry, setting
  ArtistID: entry.ArtistID alongside watchedName := entry.Name, so
  watched_artist_name is by construction the name of the artist artist_id
  already points at — the migration writes exactly what the post-000006
  insert path would have written. Scoped to IS NULL so it is idempotent and
  can never overwrite a snapshot the live insert path produced. The down
  migration is a documented no-op (a populated value is indistinguishable
  from a detection-written one, and 000006's own down drops the column).
  Also updated migrate_test.go's from-scratch assertion from version 6 to 7.
verification: |
  guardrail_verdict: accepted
  signal_1_original_issue_resolved: PASS — same dev Postgres, before/after on
    identical data: guest_feature 244 rows / 0 populated -> 244/244 populated;
    live GET /events?event_type=guest_feature now returns e.g.
    artist_name "Hotboii" with watched_artist_name "Lil Baby" (the user's own
    reported scenario), satisfying watchlistNote()'s render condition.
  signal_2_mechanism_understood: PASS — NULL -> watchlistNote() early-returns
    null -> no note. Backfill supplies the value the render path already
    consumed correctly.
  signal_3_regression: PASS — full Go suite green (16/16 packages) via
    `go test ./... -p 1`; frontend vitest suite green; `npx tsc --noEmit`
    clean; `go vet ./...` clean. Invariant preserved in both directions:
    new_release 1369/1369 keep watched_artist_name = artist_name (note stays
    hidden, 0 render), guest_feature 244/244 differ (note renders).
  signal_4_not_deletion_only: PASS — adds a migration pair plus a test
    assertion update; no code or assertions removed.
  signal_5_flaky_signal_isolated: PASS — an initial parallel full-suite run
    showed TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier
    failing. Proven pre-existing and unrelated: it passes in isolation on the
    fixed tree, and a stashed pre-fix baseline run failed WORSE (602s hang in
    RunMigrations advisory-lock contention from internal/watchlist). Cause is
    shared-DB contention between parallel test packages plus the live app
    container's poller; serializing with -p 1 and stopping the app container
    yields a fully green suite on the fixed tree.
  environment_caveat: `-race` could not be used on this Windows host —
    ThreadSanitizer fails to allocate (error code 87). Suite was run without
    -race; CI still exercises the race detector.
  human_verification: CONFIRMED 2026-08-25T19:10Z — user hard-refreshed the
    History tab against the rebuilt app (schema_migrations=7) and confirmed
    guest_feature cards now display "— <watched artist> is on your watchlist".
oracle_type: derived — the expected value is not a hand-written literal but is
  derived from an existing invariant in the data model (watched_artist_name
  must equal artists.name for the row's artist_id, because detection sources
  both from a single watchlist entry). Verified as a contract over all 1613
  rows rather than a single spot-check, with boundary coverage on both
  equivalence classes (names equal -> note hidden; names differ -> note shown)
  and the pre-existing null case already covered by EventCard.test.tsx.
files_changed:
  - internal/db/migrations/000007_backfill_events_watched_artist_name.up.sql (new)
  - internal/db/migrations/000007_backfill_events_watched_artist_name.down.sql (new)
  - internal/db/migrate_test.go (from-scratch version assertion 6 -> 7)

## Prevention
<!-- Blameless postmortem — written at archive_session -->

### Branching 5-Whys (reuses the Phase 2A candidate_causes branches)

**Branch A — data (the confirmed branch):**
1. Why was the label missing? `watchlistNote()` received a falsy `watched_artist_name` and early-returned null.
2. Why was it falsy? All 244 guest_feature rows had the column NULL.
3. Why NULL? Migration 000006 added it as a nullable column with no `UPDATE` backfill.
4. Why no backfill? Adding a column and populating existing rows are two separate acts, and nothing in the workflow prompts for the second. The migration was written to make the *forward* path correct and the existing-rows question was never surfaced as a decision.
5. Actionable condition: **a schema-addition step with no checkpoint asking "what happens to rows that already exist?"**

**Branch B — code (the second AND-gate leg):**
1. Why didn't the NULL heal itself on the next detection sweep? `InsertEvent` is `ON CONFLICT ... DO NOTHING`.
2. Why that shape? D-20 deliberately chose write-once snapshot semantics for events (correct for its purpose — an event is a historical record, not a mutable projection), explicitly unlike `UpsertArtist`'s COALESCE-refresh.
3. Why did that interact badly? Write-once is a *correct* choice that silently converts "column added late" from a transient gap into a permanent one. The interaction is invisible at either site: the migration author sees no insert code, the insert author sees no migration.
4. Actionable condition: **write-once insert paths make every future nullable column addition a mandatory-backfill situation, and that coupling is undocumented.**

Note: neither branch is a defect on its own. 000006 without `DO NOTHING` would have healed on the next sweep; `DO NOTHING` without 000006 has no gap to expose. Only the conjunction produces the bug — which is why single-cause reasoning would have missed it.

**Blame-free framing:** the operative question is not "who forgot the backfill" but "why was omitting it possible and undetectable" — answered above as an absent workflow checkpoint plus an undocumented coupling between two independently-correct design decisions.

### Why wasn't this caught?

No gate existed for this class, and each existing gate was structurally incapable of catching it:

- **Unit tests (Go + `EventCard.test.tsx`):** construct fixtures with the column populated, exercising the render path against data that never exists in a migrated DB.
- **`TestRunMigrations_AppliesFromScratch`:** runs against an **empty** `events` table — a missing backfill is invisible by construction, since zero rows trivially satisfy any backfill.
- **`tsc` / `go vet` / `golangci-lint`:** cannot observe data state.
- **Verify/UAT for quick task 260825-g6i:** passed because the task's *other* change (release-chronology ordering) is a pure read-side query change requiring no new column data. Seeing the ordering work read as "the task shipped," masking the label regression.

### Recurrence guard

- **Primary (KB pattern, in place):** the entry in `.planning/debug/knowledge-base.md` records the diagnostic pattern *"nullable ADD COLUMN + write-once ON CONFLICT DO NOTHING = permanently NULL historical rows"*, so Phase-0 recall surfaces it on symptoms like "works for new records but not existing ones" or "UI element silently absent, no error."
- **Supporting (verified):** `internal/db/migrate_test.go:TestRunMigrations_AppliesFromScratch` asserts `schema_migrations` version 7, forcing any future migration author to consciously update the expected version — a review touchpoint. Guards migration-count drift only, **not** this bug class.
- **Open gap (recorded, not claimed as covered):** no automated gate for the class was added. The honest guard would be a migration-review checklist item, or a test/lint that flags an `ADD COLUMN` whose table's insert path is `DO NOTHING` without an accompanying backfill.
