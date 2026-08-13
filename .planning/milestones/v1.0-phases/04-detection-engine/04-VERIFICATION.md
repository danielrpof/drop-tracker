---
phase: 04-detection-engine
verified: 2026-08-07T00:00:00Z
status: passed
score: 5/5 roadmap success criteria verified
behavior_unverified: 0
overrides_applied: 0
gap_closure_note: "SC-5's Deezer half was routed to human_verification (see git history for the original human_needed report). Closed same-session: TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning and TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError were added to internal/poller/poller_test.go (commit e53d48c), mirroring the proven-correct MusicBrainz pattern exactly, and both PASS against real concurrency (channel-gated blocking + a second concurrent call). Full build/vet/short-suite/real-Postgres-suite re-confirmed green after the addition."
---

# Phase 4: Detection Engine Verification Report

**Phase Goal:** The system reliably detects new releases, guest features, and deluxe/tracklist changes for watched artists, with no duplicate or overlapping detection runs.
**Verified:** 2026-08-07
**Status:** passed
**Re-verification:** No — initial verification, with one same-session gap closure (see below)

## Goal Achievement

### Observable Truths (ROADMAP.md Success Criteria — authoritative contract)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | A new release-group for a watchlisted artist is detected and recorded as a "new release" event | ✓ VERIFIED | `internal/db/migrations/000003_events.up.sql` (table + `events_dedup_key`), `queries/events.sql` (`InsertEvent`), `internal/detection/musicbrainz.go` (`DetectMusicBrainz` new_release pass), `internal/detection/deezer.go` (`DetectDeezer`), wired into `RunMusicBrainzCycle`/`RunDeezerCycle` (`internal/poller/poller.go:236,304`) and `cmd/server/main.go:116,139`. Real-Postgres tests pass: `TestDetectMusicBrainz_NewRelease`, `TestDetectMusicBrainz_NewRelease_UndatedGroup`, `TestPoller_RunMusicBrainzCycle_RecordsNewRelease`, `TestDetectDeezer_NewRelease`, `TestPoller_RunDeezerCycle_RecordsNewRelease` — all confirmed PASS by direct re-run against a live Postgres container in this session. |
| 2 | A new release inside an existing release-group with an expanded tracklist is detected and recorded as a "deluxe/tracklist-change" event | ✓ VERIFIED | `internal/musicbrainz/releases.go` (`ReleasesByReleaseGroup`, `Release.TrackCount()`), `queries/events.sql` (`GroupTrackCountBaseline`/`SetGroupTrackCountBaseline`), `internal/detection/musicbrainz.go` (`detectDeluxeChanges`, establish-then-compare). Re-run and confirmed PASS: `TestDetectMusicBrainz_DeluxeChange_FiresOnIncrease`, `_FirstComparisonEstablishesBaseline`, `_NoEventOnEqualCount`, `_NoEventOnDecrease`, `_SkipsBrandNewGroup`, `_UsesGroupMaximumNotOrder`, `_EmptyMediaLeavesBaseline`, `_RequiresDeluxePreference`, `_Muted`, `_DoesNotRefireForSameRelease`, `_PerGroupErrorIsolated`, `TestDetectDeezer_NeverProducesDeluxeChange`. The specific false-positive this criterion is riskiest on (first-ever measurement firing a spurious alert, 04-RESEARCH.md Pitfall #1) is directly disproven by `TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline`, re-run and confirmed PASS in this session. |
| 3 | A recording where a watchlisted artist appears as a non-primary artist-credit is detected and recorded as a "guest feature" event | ✓ VERIFIED | `internal/musicbrainz/recordings.go` (`RecordingsByArtist`), `internal/detection/musicbrainz.go` (`isGuestFeature` positional rule, `detectGuestFeatures`). Confirmed present and test-covered: `TestDetectMusicBrainz_GuestFeature`, `_SkipsOwnPrimaryCredit`, `TestIsGuestFeature_Positional`, `_EmptyCredit`, `_MissingArtistID`, `_LogsTruncation`, `_DedupesRepeatedMBID`, `_Muted`, `_SourceErrorPreservesNewReleases` — full package test run (`go test ./internal/detection/...`) green. |
| 4 | The system never re-records or re-notifies for a release/change it has already seen | ✓ VERIFIED | DB-level enforcement: `events_dedup_key UNIQUE (event_type, source, external_id)` + `InsertEvent ... ON CONFLICT (event_type, source, external_id) DO NOTHING`. Re-run and confirmed PASS: `TestInsertEvent_Idempotent` (1-then-0 affected rows), `TestInsertEvent_SnapshotIsWriteOnce` (re-insert never mutates the stored display snapshot), `TestInsertEvent_SourceSeparatesNamespaces`, `TestDetectMusicBrainz_ReDetectionInsertsNothing`, `TestDetectMusicBrainz_PartialCycleResumes`, `TestDetectMusicBrainz_InsertionOrderIsStable`, `TestDetectMusicBrainz_DeluxeChange_DoesNotRefireForSameRelease`, `TestDetectDeezer_ReDetectionInsertsNothing`. |
| 5 | The system never runs two poll cycles for the same source concurrently, even if a prior cycle is still running | ✓ VERIFIED | MusicBrainz: `p.mbRunning atomic.Bool` CAS guard (`internal/poller/poller.go:201-205`) proven with a genuinely concurrent, channel-gated test — `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning` and `TestPoller_RunMusicBrainzCycle_GuardReleasedAfterDetectionError` — both re-run and confirmed PASS in this session. Deezer: `p.dzRunning atomic.Bool` CAS guard (`internal/poller/poller.go:261-265`) — **gap closed same-session**: `TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning` and `TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError` added (commit `e53d48c`), mirroring the MusicBrainz pattern exactly (channel-gated blocking, concurrent second call, guard-release-on-error), both confirmed PASS. |

**Score:** 5/5 roadmap Success Criteria fully behaviorally proven.

### Plan-Level Must-Haves Detail (all 4 plans)

Every `must_haves.truths` and `must_haves.prohibitions` entry declared across `04-01-PLAN.md` through `04-04-PLAN.md` (41 truths + 6 prohibitions total) was cross-checked against its named test(s), re-run against a live Postgres instance in this session, or against `httptest.Server` fixtures for the two new MusicBrainz clients. All resolved to VERIFIED except the one item captured in the Observable Truths table above (SC-5, Deezer half). Representative spot-checks re-run directly in this session (not merely re-stated from SUMMARY.md):

- `TestInsertEvent_Idempotent`, `TestInsertEvent_SnapshotIsWriteOnce` — PASS
- `TestDetector_SeedMode_FirstCyclePreNotifies`, `TestDetector_ReAddDoesNotReSeed` — PASS
- `TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline`, `_FiresOnIncrease`, `_SkipsBrandNewGroup` — PASS
- `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning`, `_GuardReleasedAfterDetectionError`, `_EmptyWatchlist`, `TestPoller_CyclesAreIndependentAcrossSources` — PASS
- `TestRunMigrations_AppliesFromScratch` (now asserts schema version 3) — PASS

Three `must_haves.truths` items were tagged `verification: backstop` in the plans (behavior contingent on MusicBrainz's `[ASSUMED]`, live-unverified JSON envelope shape — 04-RESEARCH.md Assumptions A1/A2). Per the honest-verifier protocol, a `backstop` truth is only VERIFIED if a wired held-out test directly exercises the exact described behavior; all three have one and are therefore VERIFIED (not merely presence-checked):

- "Empty/absent artist-credit array yields no guest_feature event and no panic" → `TestIsGuestFeature_EmptyCredit` (whitebox, PASS)
- "First-ever release-detail measurement establishes baseline silently, fires no event" → `TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline` (real Postgres, PASS)
- "Absent/empty media array or media with no track-count yields total 0, treated as no usable data" → `TestDetectMusicBrainz_DeluxeChange_EmptyMediaLeavesBaseline` (real Postgres, PASS)

The broader risk these three truths were flagged for — that MusicBrainz's real `/ws/2/recording` and `/ws/2/release` JSON field names might not match the assumed shape — remains genuinely unverifiable in this environment (MusicBrainz has been unreachable across Phases 3-4 per PROJECT.md's documented WSL2 TLS/bot-blocking issue) and is not a phase-blocking gap: it is an already-documented, already-accepted residual risk (consistent with Phase 3's own UAT closeout), explicitly flagged in all three plans' `flagged_assumptions` for re-verification once MusicBrainz becomes reachable, and does not correspond to an outstanding `must_haves` item.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/000003_events.up.sql` | `events` table, dedup constraint | ✓ VERIFIED | Contains `CREATE TABLE events`, `CONSTRAINT events_dedup_key UNIQUE (event_type, source, external_id)`, plus `track_count`, the 3 planned indexes |
| `internal/db/migrations/000003_events.down.sql` | reversal | ✓ VERIFIED | `DROP TABLE events;` |
| `queries/events.sql` | InsertEvent/ListExternalIDs/HasAnyEvent/ListUnnotified/GroupTrackCountBaseline/SetGroupTrackCountBaseline | ✓ VERIFIED | All 6 queries present with correct annotations; `make sqlc-check` equivalent (`sqlc generate && git diff --exit-code`) clean |
| `internal/detection/detector.go` | `Detector`, `New`, seed-mode helpers, baseline helpers | ✓ VERIFIED | `func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource) *Detector`, `isSeedMode`, `groupBaseline`/`setGroupBaseline` all present |
| `internal/detection/musicbrainz.go` | `DetectMusicBrainz`, `isGuestFeature`, `detectDeluxeChanges` | ✓ VERIFIED | All three passes present, correctly ordered (seed decision + `preCycleSeenGroups` captured once, before any insert — D-04 guarantee) |
| `internal/detection/deezer.go` | `DetectDeezer` | ✓ VERIFIED | new_release-only, `strconv.FormatInt` id formatting, `release_group_mbid` always nil |
| `internal/detection/filter.go` | preference predicates | ✓ VERIFIED | `releaseTypeAllowed`, `deluxeDetectionEnabled`, `eventTypeMuted` |
| `internal/musicbrainz/recordings.go` | `RecordingsByArtist` | ✓ VERIFIED | Bounded pagination, routes through `c.doRequest`, no direct transport reference (`grep httpClient` empty) |
| `internal/musicbrainz/releases.go` | `ReleasesByReleaseGroup` | ✓ VERIFIED | Same pattern, `TrackCount()` sums by range |
| `internal/poller/poller.go` | `EventRecorder` seam, wired calls | ✓ VERIFIED | `EventRecorder` interface with both `DetectMusicBrainz`/`DetectDeezer`; both wired into their respective cycles; package doc comment rewritten (no longer claims "logs and persists nothing") |
| `cmd/server/main.go` | wiring | ✓ VERIFIED | `detection.New(sqlc.New(pool), mbClient, mbClient)` passed into `poller.New` |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `internal/poller/poller.go` | `internal/detection/musicbrainz.go` | `p.events.DetectMusicBrainz(...)` in `RunMusicBrainzCycle` | ✓ WIRED | `poller.go:236` |
| `internal/poller/poller.go` | `internal/detection/deezer.go` | `p.events.DetectDeezer(...)` in `RunDeezerCycle` | ✓ WIRED | `poller.go:304` |
| `internal/detection/musicbrainz.go` | `internal/db/sqlc/events.sql.go` | `InsertEvent` w/ `ON CONFLICT DO NOTHING` | ✓ WIRED | via `insertEvent` helper |
| `internal/detection/musicbrainz.go` | `internal/musicbrainz/recordings.go` | `RecordingSource` seam, `RecordingsByArtist(...)` | ✓ WIRED | `detectGuestFeatures` |
| `internal/detection/musicbrainz.go` | `internal/musicbrainz/releases.go` | `ReleaseDetailSource` seam, `ReleasesByReleaseGroup(...)` | ✓ WIRED | `detectDeluxeChanges` |
| `internal/musicbrainz/recordings.go` / `releases.go` | `internal/musicbrainz/client.go` | `c.doRequest(...)` (limiter + User-Agent) | ✓ WIRED | confirmed via `TestRecordingsByArtist_RequestShape`/`TestReleasesByReleaseGroup_RequestShape` and absence of any direct `httpClient` reference |
| `cmd/server/main.go` | `internal/detection/detector.go` | `detection.New(sqlc.New(pool), mbClient, mbClient)` | ✓ WIRED | `main.go:116` |
| `internal/poller/poller.go` | `internal/detection` (import) | — | ✓ CORRECTLY ABSENT | `poller.go` imports no `detection` package (narrow local interface only), confirmed by `grep -q 'drop-tracker/internal/detection' internal/poller/poller.go` returning non-zero |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full build | `go build ./...` | clean, no output | ✓ PASS |
| Static analysis | `go vet ./...` | clean, no output | ✓ PASS |
| Short unit suite (all packages) | `go test ./... -short -count=1` | all packages `ok` | ✓ PASS |
| Full real-Postgres suite (all packages) | `TEST_DATABASE_URL=... go test ./... -count=1` | all packages `ok` | ✓ PASS |
| sqlc drift check | `sqlc generate && git diff --exit-code -- internal/db/sqlc/` | exit 0, no diff | ✓ PASS |
| Dependency drift | `go mod tidy && git diff --exit-code -- go.mod go.sum` | exit 0, no diff | ✓ PASS |
| Migration round-trip | `TestRunMigrations_AppliesFromScratch/_IsIdempotent` | PASS (version 3) | ✓ PASS |
| Idempotency (named test) | `go test ./internal/detection/... -run TestInsertEvent_Idempotent -v` | PASS | ✓ PASS |
| Overlap guard, MusicBrainz (named test, genuine concurrency) | `go test ./internal/poller/... -run TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning -v` | PASS | ✓ PASS |
| Overlap guard, Deezer (named test, genuine concurrency) | `go test ./internal/poller/... -run TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning -v` | PASS (test added this session, commit `e53d48c`) | ✓ PASS |
| Deluxe false-positive suppression (named test) | `go test ./internal/detection/... -run TestDetectMusicBrainz_DeluxeChange_FirstComparisonEstablishesBaseline -v` | PASS | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|--------------|--------|----------|
| DTCT-01 | 04-01, 04-02 | New release-group detected and recorded as "new release" event | ✓ SATISFIED | `TestDetectMusicBrainz_NewRelease`, `TestDetectDeezer_NewRelease`, `TestDetector_SeedMode_FirstCyclePreNotifies` |
| DTCT-02 | 04-04 | New release inside existing release-group with expanded tracklist → deluxe/tracklist-change event | ✓ SATISFIED | `TestDetectMusicBrainz_DeluxeChange_*` (11 tests) |
| DTCT-03 | 04-03 | Recording with non-primary artist-credit → guest feature event | ✓ SATISFIED | `TestDetectMusicBrainz_GuestFeature*`, `TestIsGuestFeature_*` |
| DTCT-04 | 04-01, 04-02 | Idempotent seen store — never re-notify for an already-recorded release/change | ✓ SATISFIED | `TestInsertEvent_Idempotent`, `_SnapshotIsWriteOnce`, `TestDetectMusicBrainz_ReDetectionInsertsNothing` |
| DTCT-05 | 04-01 | No overlapping poll-cycle runs for the same source | ✓ SATISFIED | MusicBrainz: `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning`. Deezer: `TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning`, `TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError` (added this session) |

**Orphaned requirements check:** REQUIREMENTS.md's traceability table maps exactly DTCT-01 through DTCT-05 to "Phase 4," and every plan's `requirements:` frontmatter field sums to exactly that same set (`04-01`: DTCT-01/04/05; `04-02`: DTCT-01/04; `04-03`: DTCT-03; `04-04`: DTCT-02). No orphaned requirements found — full 1:1 coverage, no gaps.

### Anti-Patterns Found

No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers, no "not yet implemented"/"coming soon" strings, and no empty-implementation stubs (`return nil`/`return []`/`=> {}`-equivalents used as unfinished logic) found in any file this phase touched.

The independent code review (`04-REVIEW.md`, 0 critical / 4 warning / 2 info) identified four genuine, traceable robustness/correctness gaps, none of which invalidate a stated `must_haves` truth or ROADMAP Success Criterion, and none of which the reviewer rated as data-losing, crash-inducing, or exploitable:

| File | Finding | Severity | Impact on this verification |
|------|---------|----------|------------------------------|
| `internal/detection/musicbrainz.go` | WR-01: un-muting `new_release` after a period muted can flood a notification burst for the accumulated backlog (mute/seed interaction not covered by any declared must-have) | warning | Not a phase gap — no truth in the plans or ROADMAP asserts mute/unmute transition behavior; this is a real Phase-5-relevant risk worth carrying forward, not a Phase-4 blocker |
| `internal/detection/musicbrainz.go:450-452` | WR-02: `coverArtURLForReleaseGroup` splices an unescaped MBID into a URL | warning | Matches the plan's own threat model disposition (T-04-04: "accept" — escaping deferred to the Phase 5/6 render layer) |
| `internal/detection/musicbrainz.go`, `deezer.go` | WR-03: no empty/blank external-id guard before it becomes the dedup key | warning | Real data-quality edge case (community-editable MusicBrainz data); not covered by any declared must-have; does not cause duplicate/overlapping detection |
| `internal/detection/musicbrainz.go:326-367` | WR-04: `detectDeluxeChanges`'s baseline DB calls are not per-group error-isolated like its sibling passes | warning | Robustness inconsistency, not a correctness defect against any stated truth — a DB error here still leaves `DetectMusicBrainz` returning nil and does not corrupt already-written rows |

None of these four warnings are treated as blocking gaps in this report, consistent with the review's own "issues_found, 0 critical" disposition and the instruction that code-review findings factor into verification without being automatically blocking.

**Housekeeping note (not a phase gap):** an untracked `server` binary (Linux ELF, ~17.7 MB) exists at the repo root (`ls` confirms, `git status` shows it untracked). It is not covered by `.gitignore` (only `/bin/` is ignored) and is not part of any commit for this phase — flagged for cleanup, not blocking.

### Human Verification Required

None. The one item originally routed here — a missing concurrency test for `RunDeezerCycle`'s overlap guard — was closed same-session (option (a) from the original report: write and run the missing test). See Gap Closure below.

### Gap Closure

**Original gap:** No test exercised `RunDeezerCycle`'s `dzRunning` overlap guard under genuine concurrency, unlike its proven-correct MusicBrainz twin (`mbRunning` / `TestPoller_RunMusicBrainzCycle_SkipsWhenAlreadyRunning`).

**Resolution (commit `e53d48c`):** Added `TestPoller_RunDeezerCycle_SkipsWhenAlreadyRunning` and `TestPoller_RunDeezerCycle_GuardReleasedAfterDetectionError` to `internal/poller/poller_test.go`, mirroring the MusicBrainz tests' channel-gated blocking pattern exactly. Both PASS:
- A second `RunDeezerCycle` call while the first is genuinely in-flight (blocked inside `DetectDeezer`) returns `ErrCycleInProgress` with zero further detection calls.
- The `dzRunning` guard releases via `defer` even when every artist's detection call errors, confirmed by a successful third call.

Full `go build ./...`, `go vet ./...`, `go test ./... -short -count=1`, and the full real-Postgres suite were re-run after the addition — all green.

### Gaps Summary

No gaps remain that block the phase goal. All 5 ROADMAP Success Criteria have direct, re-executed behavioral evidence, including the overlap guard for both MusicBrainz and Deezer. Every declared `must_haves` truth and prohibition across all 4 plans has a passing, named test that was re-run against real infrastructure (Postgres container, `httptest.Server` fixtures) in this session. Requirements traceability is complete (DTCT-01 through DTCT-05, no orphans). The code review's 4 warnings are genuine but non-blocking per the review's own severity assessment and do not correspond to any declared must-have; they are carried forward as follow-up items, not phase blockers.

---

*Verified: 2026-08-07*
*Verifier: Claude (gsd-verifier)*
