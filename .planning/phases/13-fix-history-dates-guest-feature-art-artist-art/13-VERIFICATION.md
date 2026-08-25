---
phase: 13-fix-history-dates-guest-feature-art-artist-art
verified: 2026-08-24T23:18:44Z
status: passed
score: 27/27 must-haves verified
behavior_unverified: 0
overrides_applied: 0
human_verification:

  - test: "Confirm the live musicbrainz.org ws/2/recording/{mbid}?inc=releases+release-groups response shape matches internal/musicbrainz/recording_lookup.go's [ASSUMED] RecordingRelease/recordingLookupResponse struct tags (releases[].date, releases[].release-group.id)."
    expected: "Field names/nesting match; a mismatch would decode silently to zero values (Go's encoding/json behavior) that look exactly like D-03's no-releases fallback, masking bug #2's fix."
    why_human: "This dev environment's WSL2 network path cannot reach musicbrainz.org (documented, waived Phase 3 blocker in STATE.md). 13-01-SUMMARY.md explicitly records this human-check as still outstanding — it was never performed against a live response in this phase."

  - test: "Decide whether to require fixes for the three WARNING-level findings in 13-REVIEW.md before shipping, or explicitly accept them as tracked follow-up debt: WR-01 (earlierDate can panic on a MusicBrainz date <4 chars), WR-02 (Backfill's Stats can double-increment Unmatched+Errored for the same artist, breaking its own documented invariant), WR-03 (Service.Add leaks the ActivityGate registration if Matcher.Match panics, since cancel()/end() are plain statements, not deferred)."
    expected: "A human decision: fix now (all three have concrete patches already written in 13-REVIEW.md) or accept and track as a follow-up issue."
    why_human: "These are code-quality/robustness judgment calls, not correctness failures of the phase's core must-haves — REVIEW.md classifies all three as 'warning' severity (0 critical) and each is reachable only via an edge case (malformed upstream data <4 chars, or a panic inside an HTTP/JSON-decoding call), not the phase's primary tested paths. Confirmed still present and unfixed in the current codebase by direct code reading during this verification."
---

# Phase 13: Fix History Dates, Guest-Feature Art & Artist Art Verification Report

**Phase Goal:** Resolve three outstanding display/data bugs users are still hitting after Phase 12: (1) History tab entries don't show a release date; (2) guest-feature release cards don't show album art; (3) artist art from MusicBrainz still doesn't render. Absorbs backlog Phase 999.2 (Deezer artist-art backfill).
**Verified:** 2026-08-24T23:18:44Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

All 27 `must_haves.truths` declared across the three phase plans (13-01, 13-02, 13-03) were checked against the actual codebase and, where automatable, against a live test run (not just SUMMARY claims). All 27 are VERIFIED.

**13-01 (guest-feature date/art, deluxe date rendering) — 8/8 verified**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | New `guest_feature` row gets non-null `release_date`/`cover_art_url` when lookup returns a dated release (D-01) | ✓ VERIFIED | `internal/detection/musicbrainz.go:233-274` wires `ReleasesForRecording`→`earliestReleaseDate`/`guestFeatureArt` into `InsertEventParams`. `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeatureStoresReleaseDateAndCoverArt` — PASS (ran live against temp Postgres DB) |
| 2 | Same-year precision-aware earliest-date rule (D-02, Q2) | ✓ VERIFIED | `earlierDate` (`musicbrainz.go:575-600`) implements year-first, then-prefix, then-lexicographic rule exactly as specified. `TestEarliestReleaseDate` (7 subtests incl. same-year prefix case) — PASS |
| 3 | Per-cycle lookup cap `maxNewGuestFeatureLookupsPerCycle = 20` (D-13) | ✓ VERIFIED | `musicbrainz.go:226-231` enforces the cap and skips remaining recordings without marking them seen. `TestDetectMusicBrainz_GuestFeature_PerCycleLookupCap` — PASS |
| 4 | No-releases lookup still inserts row with NULL date/art (D-03) | ✓ VERIFIED | `TestDetectMusicBrainz_GuestFeature_EmptyReleaseListInsertsWithNulls` — PASS |
| 5 | Per-recording lookup error isolated, siblings still process (OQ-02) | ✓ VERIFIED | `musicbrainz.go:237-248` logs and `continue`s. `TestDetectMusicBrainz_GuestFeature_PerRecordingLookupErrorIsolated` — PASS |
| 6 | `deluxe_change` card renders date ahead of track-count delta, falls back to "Release date unknown" (D-04/D-05) | ✓ VERIFIED | `EventCard.tsx:171-186` (`DeluxeChangeBody`). `npm test -- EventCard` — 13/13 PASS |
| 7 | `guest_feature` card renders a release-date line (OQ-01) | ✓ VERIFIED | `EventCard.tsx:139-163` (`GuestFeatureBody`), both linked/unlinked branches render `dateLabel`. Same test run — PASS |
| 8 | `ReleasesForRecording` uses `url.PathEscape` and reports non-OK status by code only (T-13-01/T-13-02) | ✓ VERIFIED | `recording_lookup.go:84,104-108`. `TestReleasesForRecording_RequestShape`, `_NonOKStatus` — PASS |

**13-02 (artistart match rule, ActivityGate, backfill queries) — 9/9 verified**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Single close-name Deezer candidate → matched Result with DeezerID/ImageURL (D-08 primary) | ✓ VERIFIED | `match.go:155-193`. `TestMatch_SingleCloseNameCandidate` — PASS |
| 2 | Zero close-name candidates → non-match, nil pointers, nil error (D-09) | ✓ VERIFIED | `TestMatch_ZeroResultsFailsClosed` — PASS |
| 3 | Diacritic folding satisfies D-08 exact-equality (Q3) | ✓ VERIFIED | `diacriticFolds` map + `foldDiacritics` (`match.go:78-112`). `TestNormalizeArtistName/acute_accent`, `/tilde` — PASS |
| 4 | Tie-break resolves via equality-or-guarded-containment title match (Q6) | ✓ VERIFIED | `titlesMatch` (`match.go:229-244`), `tieBreak` (`match.go:249-305`). `TestMatch_TieBreakDeluxeEditionSuffixResolves`, `TestTitlesMatch` (4 subtests) — PASS |
| 5 | Zero or ≥2 tie-break winners → non-match, popularity never decides (D-08/D-09) | ✓ VERIFIED | `match.go:299-304`; `grep -c NbFan match.go` (executable code) = 0. `TestMatch_TwoSameNamedCandidatesWithNoTieBreakDataFailsClosed`, `TestMatch_TieBreakBothShareTitleFailsClosed` — PASS |
| 6 | Exactly one exported match entry point | ✓ VERIFIED | `grep -c 'func (m \*Matcher) Match' match.go` = 1 |
| 7 | `ActivityGate.Active()` true only while ≥1 unmatched `Begin()`, double-`end()`-safe (D-10) | ✓ VERIFIED | `activity.go` — atomic counter + `sync.Once`-guarded closure. `TestActivityGate_TwoConcurrentBeginsBothMustEnd`, `_DoubleEndDoesNotCorruptState`, `_ConcurrentUse` — PASS |
| 8 | `ListArtistsMissingImage` scope + 24h cooldown predicate (D-06/D-07/D-12, Q4) | ✓ VERIFIED | `queries/artists.sql:22-44`. `TestListArtistsMissingImage_CooldownAndScope` — PASS (ran live against temp Postgres DB) |
| 9 | `RecordArtMatchAttempt` sets timestamp regardless of outcome (D-12) | ✓ VERIFIED | `queries/artists.sql:46-55`. `TestRecordArtMatchAttempt_SetsTimestampLeavesImageUntouched` — PASS |

**13-03 (add-time wiring, backfill sweep, boot wiring) — 10/10 verified**

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Add with no `ImageURL` triggers exactly one match attempt; confident match persists (D-06) | ✓ VERIFIED | `service.go:198-243`. `TestService_Add_ArtistArt_CallsMatchWhenImageURLNil`, `_MatchedPersistsDeezerIDAndImageURL` — PASS (live DB) |
| 2 | Add still succeeds (201, complete Entry) on match error/timeout/non-match (D-06/D-09) | ✓ VERIFIED | `TestService_Add_ArtistArt_MatchErrorStillSucceeds`, `_UnmatchedLeavesPreviousImageUnchanged` — PASS |
| 3 | Add with a supplied `ImageURL` triggers zero match attempts | ✓ VERIFIED | `TestService_Add_ArtistArt_SkipsMatchWhenImageURLProvided` — PASS |
| 4 | Add-time match registers on shared `ActivityGate` for the match's duration (D-10) | ✓ VERIFIED (normal path only — see WR-03 below) | `TestService_Add_ArtistArt_ActivityGateActiveDuringMatch` — PASS. Code-read confirms `end()`/`cancel()` are plain (non-`defer`) statements after `Match` returns (`service.go:215-219`) — a panic inside `Match` would leak the gate permanently active. Flagged as WR-03 (13-REVIEW.md); routed to human decision below, not scored as a gap since the tested/claimed happy-path behavior is correct. |
| 5 | Cooldown-eligible watchlisted artist visited; matched artist written via `UpsertArtist`, attempt always recorded (D-06/D-07/D-12) | ✓ VERIFIED | `backfill.go:136-223`. `TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach` — PASS |
| 6 | Fail-closed backfill match writes nothing via `UpsertArtist`, still records attempt (D-09/D-12) | ✓ VERIFIED | `backfill.go:162-178`. `TestBackfill_UnmatchedArtist_NoUpsertButRecordsAttempt` — PASS |
| 7 | One artist's match error doesn't stop the sweep | ✓ VERIFIED | `TestBackfill_MatchError_NoUpsertNoRecordAttempt_ContinuesOthers` — PASS |
| 8 | Sweep yields to active `ActivityGate` with a bounded poll, then proceeds regardless (D-10) | ✓ VERIFIED | `waitForActivityGate` (`backfill.go:89-108`). `TestBackfill_ActivityGate_DelaysThenProceeds` — PASS |
| 9 | `Stats` exposes a computed match rate; summary log includes it (D-11) | ✓ VERIFIED | `MatchRatePercent` (`backfill.go:74-79`), logged at `backfill.go:225-231`. `TestStats_MatchRatePercent` — PASS. Note: `Stats`' own doc-comment invariant ("Matched+Unmatched+Errored sum consistently against Visited") can be violated in one narrow edge case — see WR-02 below; `MatchRatePercent` itself (Matched/Visited) is unaffected and correct. |
| 10 | Backfill never blocks the HTTP listener; drained before pool closes | ✓ VERIFIED | `cmd/server/main.go`: backfill goroutine started after `pollr.Start(ctx)` (line ~233); drain defer registered after `defer pool.Close()` (line 109 < drain defer), confirming LIFO drain-before-close ordering by direct grep/read. |

**Score:** 27/27 truths verified (0 present-but-behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/musicbrainz/recording_lookup.go` | `ReleasesForRecording` single-entity lookup | ✓ VERIFIED | Exists, substantive, wired into `detection.RecordingSource` |
| `internal/musicbrainz/recording_lookup_test.go` | httptest coverage | ✓ VERIFIED | 6 tests, all pass |
| `internal/detection/detector.go` | Widened `RecordingSource` interface | ✓ VERIFIED | `ReleasesForRecording` added; `detection.New` signature unchanged (confirmed via grep) |
| `internal/detection/musicbrainz.go` | `detectGuestFeatures` wiring, helpers | ✓ VERIFIED | `earliestReleaseDate`, `earlierDate`, `guestFeatureArt`, cap const all present and wired |
| `web/app/components/history/EventCard.tsx` | Date rendering on 2 bodies | ✓ VERIFIED | `GuestFeatureBody`/`DeluxeChangeBody` both render `dateLabel` |
| `internal/artistart/match.go` | `Matcher`, `Result`, seams, normalize/tie-break | ✓ VERIFIED | All present, one exported `Match` entry point |
| `internal/artistart/activity.go` | `ActivityGate` | ✓ VERIFIED | Present, race-tested (non-`-race` on this Windows box per known limitation, plain concurrent test used instead) |
| `internal/artistart/backfill.go` | `Store`, `Stats`, `Backfill`, `waitForActivityGate` | ✓ VERIFIED | All present and wired |
| `internal/db/migrations/000005_artists_art_match_attempted_at.{up,down}.sql` | New nullable column migration | ✓ VERIFIED | Exists; `up` adds nullable `timestamptz`, `down` drops it — exactly one new migration pair, confirmed via `ls` |
| `queries/artists.sql` + generated sqlc | `ListArtistsMissingImage`, `RecordArtMatchAttempt` | ✓ VERIFIED | Both on `Querier` interface. `sqlc generate && git diff --exit-code -- internal/db/sqlc/` — zero content drift (only pre-existing Windows CRLF warnings, no diff) |
| `internal/watchlist/service.go` | Variadic `Option`, `WithArtistArt`, `Add` wiring | ✓ VERIFIED | `NewService(q sqlc.Querier, opts ...Option)`; existing call sites compile unchanged |
| `cmd/server/main.go` | Client reorder, shared matcher/gate, backfill goroutine + drain | ✓ VERIFIED | `dzClient`/`mbClient` before `store`; one `artistart.NewMatcher`, one `artistart.NewActivityGate`; backfill goroutine + LIFO drain confirmed by line-number grep |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| `musicbrainz.Client.ReleasesForRecording` | `detection.RecordingSource` | interface satisfaction | ✓ WIRED | `detection.New`'s signature and `cmd/server/main.go`'s wiring line unchanged (confirmed via grep) |
| `detectGuestFeatures` | `d.insertEvent` | `ReleaseDate`/`ReleaseGroupMbid`/`CoverArtUrl` params | ✓ WIRED | Reuses `coverArtURLForReleaseGroup`/`nullableString`, no parallel helpers |
| `events.release_date` | `EventCard.tsx` bodies | `EventItem.release_date` (no `api.ts` change) | ✓ WIRED | Confirmed `api.ts` untouched by this phase (per 13-01-SUMMARY, no listed changes to `api.ts`) |
| `artistart` package | `*deezer.Client`/`*musicbrainz.Client` | narrow `ArtistSearcher`/`AlbumLister`/`ReleaseGroupLister` interfaces | ✓ WIRED | `grep -c 'deezer.Client' match.go` (executable code) = 0 — depends only on its own interfaces |
| `watchlist.Service.Add` | `artistart.Matcher.Match` | `ArtistMatcher` interface via `WithArtistArt` | ✓ WIRED | `service.go:203-243` |
| `cmd/server/main.go` | `watchlist.WithArtistArt` + `artistart.Backfill` | shared `artMatcher`/`artActivityGate` instances | ✓ WIRED | Both call sites receive the same two instances (confirmed via grep) |
| `internal/artistart` | `internal/watchlist` / `cmd/server` | (must NOT import) | ✓ CONFIRMED ONE-WAY | `grep -c 'drop-tracker/internal/watchlist' internal/artistart/*.go` (full path match) = 0 |

### Data-Flow Trace (Level 4)

Not applicable in the dashboard-hollow-prop sense — this phase's frontend changes are pure derived-string rendering (`event.release_date ?? fallback`) from an already-fetched `EventItem`, not a new data source. Traced anyway: `release_date`/`cover_art_url` originate from real MusicBrainz HTTP responses decoded into `RecordingRelease` (not static/hardcoded), flow through `InsertEventParams` into a real `INSERT ... events`, and are read back by the existing `GET /events` handler into `EventItem` — confirmed by the passing real-Postgres test `TestDetectMusicBrainz_GuestFeatureStoresReleaseDateAndCoverArt`, which asserts the actual persisted row values. FLOWING, not static.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Guest-feature date/art lookup, precision rule, cap, error isolation (Go, DB-backed) | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` against a temporary Postgres DB (`verify_13_test`, dropped after use) | 12/12 subtests PASS | ✓ PASS |
| artistart match rule + ActivityGate + backfill (Go, no DB) | `go test ./internal/musicbrainz/... ./internal/artistart/... -v` | 33/33 tests PASS | ✓ PASS |
| watchlist add-time wiring + new queries (Go, DB-backed) | `go test ./internal/watchlist/... -v` against the same temp DB | 40/40 tests PASS | ✓ PASS |
| History card date rendering (frontend) | `cd web && npm test -- EventCard` | 13/13 tests PASS | ✓ PASS |
| Full repo build/vet/lint | `go build ./...`, `go vet ./...`, `golangci-lint run ./...` | clean, 0 issues | ✓ PASS |
| sqlc drift check | `sqlc generate && git diff --exit-code -- internal/db/sqlc/` | zero content diff (pre-existing CRLF-only warning noise, matches 13-02-SUMMARY's documented note) | ✓ PASS |

### Probe Execution

Not applicable — this phase has no `scripts/*/tests/probe-*.sh` probes and none are referenced by the plans/SUMMARYs.

### Requirements Coverage

No `REQUIREMENTS.md` exists in this project. Per the task instructions, traceability is checked against `13-CONTEXT.md`'s locked decisions D-01 through D-13 instead.

| Decision | Description | Status | Evidence |
|----------|-------------|--------|----------|
| D-01 | Per-recording MusicBrainz lookup for cover art + date | ✓ SATISFIED | `ReleasesForRecording` wired into `detectGuestFeatures` |
| D-02 (amended Q2) | Precision-aware earliest-date rule | ✓ SATISFIED | `earlierDate`, tested; see WR-01 caveat for malformed-input robustness |
| D-03 | No-releases fallback | ✓ SATISFIED | Tested |
| D-04 | `DeluxeChangeBody` renders date | ✓ SATISFIED | Tested |
| D-05 | Shared "Release date unknown" fallback | ✓ SATISFIED | Tested, verbatim expression reused |
| D-06 | Artist-art matching applies to new adds AND backfill | ✓ SATISFIED | `WithArtistArt` + `Backfill`, both wired in `cmd/server/main.go` |
| D-07 (amended Q4) | One-time startup sweep, cooldown-bounded | ✓ SATISFIED | Async goroutine after `pollr.Start`; 24h cooldown via `art_match_attempted_at` |
| D-08 (amended Q3/Q6) | Close-name primary signal + guarded-containment tie-break, diacritic fold | ✓ SATISFIED | Tested extensively |
| D-09 | Fail closed on no confident match | ✓ SATISFIED | Tested; `NbFan` never referenced in executable code |
| D-10 (Q1) | `ActivityGate` priority-yielding, no second rate budget | ✓ SATISFIED (with a caveat) | `ActivityGate` itself correct and tested; its *caller* in `Service.Add` leaks registration on a `Match` panic (WR-03) — a narrow, documented, non-blocking gap |
| D-11 (Q3) | `MatchRatePercent` + summary log | ✓ SATISFIED (with a caveat) | Match rate itself correct; `Stats`' own doc-invariant can be violated in one edge case (WR-02) — does not affect the match-rate signal D-11 requires |
| D-12 (Q4) | `art_match_attempted_at` cooldown column + `RecordArtMatchAttempt` | ✓ SATISFIED | Migration + queries + tests all present |
| D-13 (Q5) | Per-cycle lookup cap of 20 | ✓ SATISFIED | Tested |

No orphaned decisions — every D-01 through D-13 in 13-CONTEXT.md maps to at least one plan's `requirements` frontmatter and has implementation + test evidence.

### Anti-Patterns Found

No debt markers (`TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER`) in any file this phase modified. However, `13-REVIEW.md` (the phase's own code review, run prior to this verification) surfaced three WARNING-severity findings, independently confirmed still present and unfixed by direct code reading during this verification:

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| `internal/detection/musicbrainz.go` | 570-600 (`earlierDate`) | Slice `a[:4]`/`b[:4]` on a MusicBrainz date whose length is not length-checked before use (only empty-string is filtered) | ⚠️ WARNING | A community-edited date shorter than 4 characters panics; recovered by the poller's per-artist `recover()`, but silently aborts that artist's remaining detection work for the cycle. No test exercises a <4-char date. |
| `internal/artistart/backfill.go` | 162-178 | `Stats.Unmatched` and `Stats.Errored` can both increment for the same artist when `RecordArtMatchAttempt` fails after a `Matched: false` outcome | ⚠️ WARNING | Breaks `Stats`' own documented "sums consistently against Visited" invariant; inflates the `errored`/`unmatched` counts in the D-11 summary log. `MatchRatePercent` itself (Matched/Visited) is unaffected. Untested (`stubStore.attemptErrByMbid` exists but no test populates it for this branch). |
| `internal/watchlist/service.go` | 203-219 | `cancel()`/`end()` are plain (non-`defer`) statements after the blocking `s.matcher.Match(...)` call | ⚠️ WARNING | If `Match` panics, `end()` never runs and `ActivityGate.count` stays permanently incremented — `artistart.Backfill`'s `waitForActivityGate` would then wait the full 5s bound before every subsequent artist for the rest of the process's life. Untested (no panic-path test exists). |

All three have concrete, already-drafted fixes in `13-REVIEW.md`. None invalidate the phase's core, tested, happy-path must-haves — they are edge-case robustness gaps, not missing functionality.

### Human Verification Required

1. **MusicBrainz recording-lookup response shape** (outstanding since 13-01)
   **Test:** From a machine that can reach musicbrainz.org, run `curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"` and compare the returned nesting against `RecordingRelease`/`recordingLookupResponse`'s struct tags in `internal/musicbrainz/recording_lookup.go`.
   **Expected:** Field names/nesting match (`releases[].date`, `releases[].release-group.id`).
   **Why human:** This dev environment's WSL2 network path cannot reach musicbrainz.org (documented, waived Phase 3 blocker in STATE.md). A mismatch would decode silently to zero values that look exactly like D-03's no-releases fallback — the one assumption in the phase that literally cannot be checked in CI, and it was never performed against a live response in either plan 13-01's execution or this verification.

2. **Accept-or-fix decision on the three code-review warnings**
   **Test:** Review WR-01/WR-02/WR-03 in `13-REVIEW.md` (all confirmed still present by this verification) and decide whether to apply the drafted fixes now or explicitly accept them as tracked follow-up debt.
   **Expected:** A recorded decision — either a follow-up commit applying the three small patches, or an explicit acceptance (e.g., a GSD override entry or a tracked issue).
   **Why human:** These are severity/risk-acceptance judgment calls (all three are edge-case, narrow-impact, non-blocking per the review's own classification), not something an automated verifier should decide unilaterally.

### Gaps Summary

No gaps. All 27 must-have truths across the three phase plans are VERIFIED against live test runs (not SUMMARY claims), all required artifacts exist/are substantive/are wired, all key links are wired, and D-01 through D-13 all have implementation + passing-test evidence. `go build`, `go vet`, `golangci-lint run`, and `sqlc-check`-equivalent are all clean. All three user-facing bugs (missing History dates, missing guest-feature art, missing artist art) are demonstrably fixed end-to-end with real Postgres and frontend test coverage, not placeholders.

Status is `human_needed` rather than `passed` solely because of two items that genuinely require a human: (1) the one MusicBrainz response-shape assumption that cannot be checked from this environment, explicitly flagged as outstanding since 13-01 and never closed, and (2) a risk-acceptance decision on three already-identified, already-patched-in-review, non-blocking code-quality warnings.

---

_Verified: 2026-08-24T23:18:44Z_
_Verifier: Claude (gsd-verifier)_
