---
phase: 13
slug: fix-history-dates-guest-feature-art-artist-art
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-24
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `httptest.Server` (client packages); real-Postgres integration tests via `testutil.NewTestPool` (detection package). Frontend: Vitest 4.1.10 + `@testing-library/react`. |
| **Config file** | `web/vitest.config.ts` (assumed present — not load-bearing for this phase's additions) |
| **Quick run command** | `go test ./internal/musicbrainz/... ./internal/detection/...` (backend) / `cd web && npm test -- EventCard` (frontend) |
| **Full suite command** | `make test` (Go) / `cd web && npm test` (frontend) |
| **Estimated runtime** | Not measured this session — existing suite, no new slow/integration-heavy fixtures introduced by this phase |

---

## Sampling Rate

- **After every task commit:** Run the quick-run command scoped to the package just touched (`go test ./internal/musicbrainz/...`, `./internal/detection/...`, or `npm test -- EventCard`)
- **After every plan wave:** Run `make test` (Go) and `cd web && npm test` (frontend)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** Not measured; per-task commands are scoped to a single package/component (seconds, existing suite — no new slow fixtures)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | D-01/D-02/D-03 | — | `ReleasesForRecording` escapes mbid via `url.PathEscape`; never echoes response body on non-OK status | unit (httptest) | `go test ./internal/musicbrainz/... -run TestReleasesForRecording` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-01/D-02/D-03 | — | N/A | unit (whitebox, real-Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-02 (amended, grilling round Q2) | — | `earliestReleaseDate` prefers the more precise date on a same-year prefix pair, not the lexicographically smaller string | unit | `go test ./internal/detection/... -run TestEarliestReleaseDate` | ❌ Wave 0 (part of the guest-feature Task 2 addition) | ⬜ pending |
| TBD | TBD | TBD | D-13 (grilling round Q5) | T-13-19 | One artist's seed cycle performs at most `maxNewGuestFeatureLookupsPerCycle` new lookups; excess recordings are retried next cycle, not lost | unit (whitebox, real-Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` | ❌ Wave 0 (same file, new case) | ⬜ pending |
| TBD | TBD | TBD | D-04/D-05 | — | N/A | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 (extends existing file) | ⬜ pending |
| TBD | TBD | TBD | GuestFeatureBody date rendering (Open Question 1 — treated in-scope) | — | N/A | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 (same file) | ⬜ pending |
| TBD | TBD | TBD | D-08/D-09 | — | Fail-closed on no confident match: never attach a wrong-artist photo | unit (stub Deezer searcher, no real HTTP) | `go test ./internal/artistart/...` (package name TBD by planner) | ❌ Wave 0 (new package) | ⬜ pending |
| TBD | TBD | TBD | D-08 amended (grilling round Q3) | — | `normalizeArtistName` folds common Latin diacritics, no new dependency | unit | `go test ./internal/artistart/... -run TestNormalizeArtistName` | ❌ Wave 0 (same package) | ⬜ pending |
| TBD | TBD | TBD | D-08 amended (grilling round Q6) | — | `titlesMatch` resolves a tie via guarded containment, not exact-only equality | unit | `go test ./internal/artistart/... -run TestTitlesMatch` | ❌ Wave 0 (same package) | ⬜ pending |
| TBD | TBD | TBD | D-10 (grilling round Q1) | T-13-20 | `ActivityGate` correctly reports in-flight state under concurrent use; `Backfill` yields to it with a bounded wait | unit, race-checked | `go test ./internal/artistart/... -race -run 'TestActivityGate|TestBackfill'` | ❌ Wave 0 (new file `activity.go`) | ⬜ pending |
| TBD | TBD | TBD | D-06/D-07/D-12 (grilling round Q4) | T-13-21 | Backfill sweeps `image_url IS NULL` rows not attempted within the last 24 hours; non-destructive to `deezer_id`/`disambiguation` via existing `UpsertArtist` COALESCE semantics; every visited artist gets an attempt recorded via `RecordArtMatchAttempt` regardless of outcome | integration (real-Postgres) | `go test ./internal/db/... ./internal/artistart/... -run 'ListArtistsMissingImage|RecordArtMatchAttempt|TestBackfill'` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | D-11 (grilling round Q3) | — | `Stats.MatchRatePercent()` computes correctly including the zero-`Visited` case, and appears in the sweep's summary log | unit | `go test ./internal/artistart/... -run TestBackfill` | ❌ Wave 0 (same file) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task ID/Plan/Wave are TBD — this file is created before PLAN.md exists; the planner should cross-reference these rows when assigning task IDs.*

---

## Wave 0 Requirements

- [ ] `internal/musicbrainz/recording_lookup_test.go` (or appended to `recordings_test.go`) — new file, no existing coverage for a lookup-by-path-segment MusicBrainz call
- [ ] `internal/detection/detector_test.go`'s `fakeRecordingSource` — needs a `ReleasesForRecording` stub method added (single-place edit)
- [ ] `internal/detection/musicbrainz_test.go` — needs the same-year prefix-pair and equal-precision `earliestReleaseDate` cases (grilling round Q2), and the per-cycle cap case (Q5)
- [ ] New package for D-08/D-09 match logic — no existing tests, no existing file
- [ ] `internal/artistart/match_test.go` — needs diacritic-fold cases (Q3) and `titlesMatch` containment cases (Q6)
- [ ] `internal/artistart/activity_test.go` — new file, no existing coverage for `ActivityGate` (Q1); must include a `-race` run
- [ ] `internal/db/migrations/000005_artists_art_match_attempted_at.{up,down}.sql` — new migration, no existing coverage (Q4); needs a real-Postgres round-trip test (up applies cleanly, down reverts cleanly)
- [ ] Backfill integration test — no existing coverage for a startup-time DB sweep; must now also cover the cooldown predicate (Q4) and the `RecordArtMatchAttempt` bookkeeping split across outcomes
- Framework install: none — Go `testing`/`httptest` and Vitest are both already fully set up in this repo

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Confirm the actual `GET /ws/2/recording/{mbid}?inc=releases+release-groups` JSON response shape (`releases[].release-group.id` nesting, `releases[].date` field) matches the `[ASSUMED]` struct tags in `ReleasesForRecording`'s response type | D-01 | This dev machine's WSL2 network path cannot reach `musicbrainz.org` (documented, waived blocker in STATE.md); CI intentionally makes no live external calls per project CLAUDE.md, so this can only be confirmed manually from a network path that can reach musicbrainz.org | From a machine that can reach musicbrainz.org: `curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"`, compare the returned shape against `RecordingRelease`/`recordingLookupResponse` struct tags, and adjust if the nesting differs before trusting the assumption in production |
| After the first production deploy carrying this phase, read the backfill sweep's summary log line and confirm the reported `match_rate_percent` is not implausibly low | D-11 (grilling round Q3) | The match rate depends on real MusicBrainz-vs-Deezer name divergence in this operator's actual watchlist, which cannot be simulated meaningfully in CI/unit tests — it's an operational health check, not a correctness assertion | After the first deploy post-phase, grep the process logs for the `Backfill` summary line; if `match_rate_percent` is persistently under ~40% (the informal threshold documented on `Stats.MatchRatePercent`), investigate `normalizeArtistName`'s folding rules before assuming the artists simply aren't on Deezer |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < N/A (see Sampling Rate — per-package scoped, not globally timed)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
