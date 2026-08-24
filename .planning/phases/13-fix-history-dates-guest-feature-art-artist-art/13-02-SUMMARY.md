---
phase: 13-fix-history-dates-guest-feature-art-artist-art
plan: 02
subsystem: database
tags: [go, sqlc, postgres, deezer, musicbrainz, artist-art, concurrency]

# Dependency graph
requires:
  - phase: 13-01
    provides: prior phase 13 plans (history dates / guest-feature art fixes) in the same phase
provides:
  - "internal/artistart package: Matcher.Match (D-08 close-name + shared-album-title tie-break, D-09 fail-closed)"
  - "ActivityGate (D-10): priority-yielding primitive between interactive adds and the backfill sweep"
  - "ListArtistsMissingImage + RecordArtMatchAttempt sqlc queries and the art_match_attempted_at migration (D-06/D-07/D-12)"
affects: [13-03, artist-art-backfill, watchlist-add]

# Actuals (#2632)
actuals:
  tokens: 12900
  tasks: 4
  commits: 8

tech-stack:
  added: []
  patterns:
    - "consumer-declared narrow interfaces (ArtistSearcher/AlbumLister/ReleaseGroupLister) mirroring detection.RecordingSource/ReleaseDetailSource"
    - "hand-rolled diacritic folding table instead of a new Unicode-normalization dependency"
    - "atomic.Int32 + sync.Once-guarded end closure for a double-call-safe activity counter"

key-files:
  created:
    - internal/artistart/match.go
    - internal/artistart/match_test.go
    - internal/artistart/activity.go
    - internal/artistart/activity_test.go
    - internal/db/migrations/000005_artists_art_match_attempted_at.up.sql
    - internal/db/migrations/000005_artists_art_match_attempted_at.down.sql
  modified:
    - queries/artists.sql
    - internal/db/sqlc/artists.sql.go
    - internal/db/sqlc/querier.go
    - internal/db/sqlc/models.go
    - internal/db/migrate_test.go
    - internal/watchlist/service_test.go

key-decisions:
  - "normalizeArtistName folds diacritics via a hand-rolled map[rune]rune table (no golang.org/x/text dependency), keeping the phase's zero-new-Go-module claim intact"
  - "titlesMatch's containment loosening (edition-suffix tolerance) is guarded by minTieBreakTitleLength=4 and only ever applies to the tie-break signal, never to D-08's primary name-equality check"
  - "tieBreak is bounded by maxTieBreakCandidates=5 to cap the synchronous MusicBrainz+Deezer fetch cost inside an HTTP add request"
  - "ActivityGate uses a plain atomic counter, not a second rate.Limiter -- a second budget was explicitly rejected (D-10 Q1(b)) since it would let total outbound traffic exceed MusicBrainz's real ~1 req/sec ceiling"
  - "RecordArtMatchAttempt is a separate minimal write from UpsertArtist -- D-09 already forbids writing image/deezer_id on a non-match, but the attempt timestamp still needs recording for the cooldown"

requirements-completed: [D-06, D-08, D-09, D-10, D-11, D-12]

coverage:
  - id: D1
    description: "Matcher.Match resolves a single close-name Deezer candidate (diacritic-folded name equality) to a Result carrying DeezerID/ImageURL"
    requirement: "D-08"
    verification:
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_SingleCloseNameCandidate"
        status: pass
    human_judgment: false
  - id: D2
    description: "Zero or ambiguous close-name candidates fail closed to a non-match with nil DeezerID/ImageURL and a nil error"
    requirement: "D-09"
    verification:
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_ZeroResultsFailsClosed"
        status: pass
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_TwoSameNamedCandidatesWithNoTieBreakDataFailsClosed"
        status: pass
    human_judgment: false
  - id: D3
    description: "Shared-album-title tie-break resolves same-named candidates via equality-or-guarded-containment title comparison, never by popularity"
    requirement: "D-08"
    verification:
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_TieBreakResolvesOnSharedAlbumTitle"
        status: pass
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_TieBreakDeluxeEditionSuffixResolves"
        status: pass
      - kind: unit
        ref: "internal/artistart/match_test.go#TestMatch_TieBreakShortSubstringDoesNotResolve"
        status: pass
    human_judgment: false
  - id: D4
    description: "ListArtistsMissingImage returns only watchlisted, NULL-image artists whose art_match_attempted_at is NULL or older than 24 hours"
    requirement: "D-06"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestListArtistsMissingImage_CooldownAndScope"
        status: pass
    human_judgment: false
  - id: D5
    description: "RecordArtMatchAttempt sets art_match_attempted_at to now for the given artist while leaving image_url untouched"
    requirement: "D-12"
    verification:
      - kind: integration
        ref: "internal/watchlist/service_test.go#TestRecordArtMatchAttempt_SetsTimestampLeavesImageUntouched"
        status: pass
    human_judgment: false
  - id: D6
    description: "ActivityGate reports active only while at least one Begin() call has no matching end(), and is safe against a double end() call"
    requirement: "D-10"
    verification:
      - kind: unit
        ref: "internal/artistart/activity_test.go#TestActivityGate_TwoConcurrentBeginsBothMustEnd"
        status: pass
      - kind: unit
        ref: "internal/artistart/activity_test.go#TestActivityGate_DoubleEndDoesNotCorruptState"
        status: pass
    human_judgment: false

duration: 25min
completed: 2026-08-24
status: complete
---

# Phase 13 Plan 02: Artist-Art Match Rule, Backfill Queries & ActivityGate Summary

**New `internal/artistart` package (D-08 close-name + shared-album-title tie-break match, D-09 fail-closed), the `ArtMatchAttemptedAt` cooldown migration + two sqlc queries the backfill sweep needs (D-06/D-07/D-12), and the `ActivityGate` concurrency primitive (D-10) -- all unit/integration-tested, no new Go module.**

## Performance

- **Duration:** 25 min
- **Started:** 2026-08-24T17:11:19-05:00
- **Completed:** 2026-08-24T17:25:26-05:00
- **Tasks:** 4
- **Files modified:** 12

## Accomplishments
- `internal/artistart.Matcher.Match` is the single, fully-tested entry point that resolves a Deezer artist candidate for a watched MusicBrainz artist: strict diacritic-folded name equality as the primary signal, a guarded shared-album-title tie-break as the only secondary signal, and a fail-closed non-match on every ambiguous or empty outcome.
- `ListArtistsMissingImage` (watchlist join + 24h cooldown, D-06/D-07/D-12) and `RecordArtMatchAttempt` (D-12) land on the generated `Querier` interface, backed by migration `000005_artists_art_match_attempted_at`.
- `ActivityGate` (D-10) is a race-safe, double-end-call-safe concurrency primitive the sibling plan will wire into both the add-time match and the backfill sweep so they yield to each other instead of contending for the same rate-limited clients.

## Task Commits

Each task was committed atomically (TDD tasks landed as separate RED/GREEN commits):

1. **Task 1: artistart package -- narrow seams, name normalization, primary close-name match**
   - `83a416f` (test) - RED: normalizeArtistName + Match behavior tests
   - `2e181c6` (feat) - GREEN: diacritic folding, primary close-name match
2. **Task 2: Shared-album-title tie-break for same-named Deezer candidates**
   - `f17e107` (test) - RED: titlesMatch + tie-break behavior tests
   - `dce507e` (feat) - GREEN: titlesMatch + tieBreak implementation
3. **Task 3: ListArtistsMissingImage + RecordArtMatchAttempt sqlc queries, migration**
   - `99affa5` (feat) - migration, queries, sqlc regen, real-Postgres tests, migrate_test.go schema-version fix
   - `f4a6f30` (fix) - avoided duplicate symbol-name occurrences in doc comments (grep-precision cleanup)
4. **Task 4: ActivityGate -- priority yielding (D-10)**
   - `b00fd58` (test) - RED: ActivityGate behavior tests
   - `4b0895d` (feat) - GREEN: atomic counter + sync.Once-guarded end closure

## Files Created/Modified
- `internal/artistart/match.go` - Matcher, Result, ArtistSearcher/AlbumLister/ReleaseGroupLister, normalizeArtistName, foldDiacritics, titlesMatch, tieBreak
- `internal/artistart/match_test.go` - full behavior matrix, stub-driven, no real HTTP
- `internal/artistart/activity.go` - ActivityGate
- `internal/artistart/activity_test.go` - concurrency + double-end-call tests
- `internal/db/migrations/000005_artists_art_match_attempted_at.{up,down}.sql` - nullable `art_match_attempted_at` column
- `queries/artists.sql` - ListArtistsMissingImage, RecordArtMatchAttempt
- `internal/db/sqlc/artists.sql.go`, `internal/db/sqlc/querier.go`, `internal/db/sqlc/models.go` - regenerated sqlc output
- `internal/db/migrate_test.go` - fixed hardcoded from-scratch schema version (4 -> 5)
- `internal/watchlist/service_test.go` - real-Postgres tests for both new queries

## Decisions Made
- Diacritic folding is a hand-rolled `map[rune]rune` table, not `golang.org/x/text/unicode/norm` -- keeps the phase's zero-new-Go-module claim (13-RESEARCH.md Package Legitimacy Audit) true.
- `titlesMatch`'s equality-or-containment loosening only ever applies to the tie-break signal (guarded by `minTieBreakTitleLength=4`), never to `Match`'s primary name-equality check -- D-09's fail-closed guarantee is preserved.
- `tieBreak` is bounded by `maxTieBreakCandidates=5` since it runs synchronously inside an HTTP add request in the sibling plan.
- `ActivityGate` is a plain `atomic.Int32` counter, not a second `rate.Limiter` -- a second budget was explicitly considered and rejected (D-10, grilling round Q1(b)) since it would let total outbound traffic exceed MusicBrainz's real external ~1 req/sec ceiling rather than respect it.
- `RecordArtMatchAttempt` is a deliberately separate, minimal write from `UpsertArtist`: D-09 already forbids writing `image_url`/`deezer_id` on a `Matched: false` outcome, but the attempt itself still needs recording so the cooldown predicate has something to check.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed migrate_test.go's hardcoded from-scratch schema version**
- **Found during:** Task 3 (migration + sqlc queries)
- **Issue:** `TestRunMigrations_AppliesFromScratch` asserted the from-scratch migration count equals 4 (Phase 5's value), which the new migration 000005 broke.
- **Fix:** Updated the expected version to 5 and its explanatory comment.
- **Files modified:** `internal/db/migrate_test.go`
- **Verification:** `go test ./internal/db/... -run TestRunMigrations` passes.
- **Committed in:** `99affa5` (Task 3 commit)

**2. [Rule 1 - Bug] Reworded artists.sql doc comments to avoid duplicate symbol-name grep matches**
- **Found during:** Task 3, post-implementation acceptance-criteria verification
- **Issue:** `ListArtistsMissingImage` and `RecordArtMatchAttempt`'s doc comments cross-referenced each other by literal name, making each symbol's occurrence count 2 in the regenerated `querier.go` instead of the plan's exact-1 acceptance criterion.
- **Fix:** Reworded the cross-references to describe the other query's role instead of naming it; regenerated sqlc output.
- **Files modified:** `queries/artists.sql`, `internal/db/sqlc/artists.sql.go`, `internal/db/sqlc/querier.go`
- **Verification:** `grep -c` for both symbol names in `querier.go` now returns exactly 1 each; `make sqlc-check` clean.
- **Committed in:** `f4a6f30`

**3. [Rule 1 - Bug] Similarly reworded two other doc-comment literal-string collisions**
- **Found during:** Task 1 and Task 3, post-implementation acceptance-criteria verification
- **Issue:** `foldDiacritics`'s doc comment named `golang.org/x/text/unicode/norm` literally (colliding with the plan's zero-new-module grep check), and `ListArtistsMissingImage`'s own doc comment repeated the literal string `image_url IS NULL` (colliding with the plan's exact-1 grep check).
- **Fix:** Reworded both comments to preserve the same explanation without the literal matched substring.
- **Files modified:** `internal/artistart/match.go`, `queries/artists.sql`
- **Verification:** Both `grep -c` acceptance criteria now match exactly as specified.
- **Committed in:** `2e181c6`, `99affa5`

---

**Total deviations:** 3 auto-fixed (all Rule 1 -- pre-existing/self-caused test and grep-precision breakage)
**Impact on plan:** All fixes were direct, in-scope consequences of this plan's own changes (a new migration, and doc comments colliding with the plan's own acceptance-criteria greps). No scope creep, no architectural changes.

## Issues Encountered
- `go test -race` is unusable on this Windows dev machine (pre-existing ThreadSanitizer allocation-failure limitation, already documented in STATE.md from Phase 11.1-04). `ActivityGate`'s concurrency and double-end-call tests were verified with plain `go test` instead; `TestActivityGate_ConcurrentUse` still exercises 50 goroutines x 200 cycles concurrently with a separate `Active()`-polling goroutine, so the race-shaped behavior is exercised even without the detector attached.
- `git status` intermittently flagged four unrelated sqlc-generated files (`db.go`, `events.sql.go`, `health.sql.go`, `watchlist.sql.go`) as modified after each `sqlc generate` run, with zero actual diff content (confirmed via `git diff`/`git diff --numstat`) -- a Windows CRLF-normalization artifact, not a real change. These were left unstaged in every commit.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `internal/artistart` (Matcher, ActivityGate) and the two new sqlc queries are ready for sibling plan 13-03 to wire into `watchlist.Service.Add` (interactive match) and a new `Backfill` sweep (startup match), per this plan's key_links.
- No blockers. `internal/artistart` imports neither `internal/watchlist` nor `cmd/server`, confirmed by grep -- the dependency direction stays one-way as required.

---
*Phase: 13-fix-history-dates-guest-feature-art-artist-art*
*Completed: 2026-08-24*

## Self-Check: PASSED

All 7 created files verified present on disk; all 8 task/RED-GREEN commit hashes and the docs commit verified present in `git log`.
