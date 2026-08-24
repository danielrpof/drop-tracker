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
| TBD | TBD | TBD | D-04/D-05 | — | N/A | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 (extends existing file) | ⬜ pending |
| TBD | TBD | TBD | GuestFeatureBody date rendering (Open Question 1 — treated in-scope) | — | N/A | unit (RTL) | `cd web && npm test -- EventCard` | ❌ Wave 0 (same file) | ⬜ pending |
| TBD | TBD | TBD | D-08/D-09 | — | Fail-closed on no confident match: never attach a wrong-artist photo | unit (stub Deezer searcher, no real HTTP) | `go test ./internal/artistart/...` (package name TBD by planner) | ❌ Wave 0 (new package) | ⬜ pending |
| TBD | TBD | TBD | D-06/D-07 | — | Backfill sweeps only `image_url IS NULL` rows; non-destructive to `deezer_id`/`disambiguation` via existing `UpsertArtist` COALESCE semantics | integration (real-Postgres) | `go test ./cmd/server/... -run TestBackfill` (or an `internal/artistart` integration test using `sqlc.New(pool)`) | ❌ Wave 0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

*Task ID/Plan/Wave are TBD — this file is created before PLAN.md exists; the planner should cross-reference these rows when assigning task IDs.*

---

## Wave 0 Requirements

- [ ] `internal/musicbrainz/recording_lookup_test.go` (or appended to `recordings_test.go`) — new file, no existing coverage for a lookup-by-path-segment MusicBrainz call
- [ ] `internal/detection/detector_test.go`'s `fakeRecordingSource` — needs a `ReleasesForRecording` stub method added (single-place edit)
- [ ] New package for D-08/D-09 match logic — no existing tests, no existing file
- [ ] Backfill integration test — no existing coverage for a startup-time DB sweep
- Framework install: none — Go `testing`/`httptest` and Vitest are both already fully set up in this repo

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Confirm the actual `GET /ws/2/recording/{mbid}?inc=releases+release-groups` JSON response shape (`releases[].release-group.id` nesting, `releases[].date` field) matches the `[ASSUMED]` struct tags in `ReleasesForRecording`'s response type | D-01 | This dev machine's WSL2 network path cannot reach `musicbrainz.org` (documented, waived blocker in STATE.md); CI intentionally makes no live external calls per project CLAUDE.md, so this can only be confirmed manually from a network path that can reach musicbrainz.org | From a machine that can reach musicbrainz.org: `curl "https://musicbrainz.org/ws/2/recording/<a-real-recording-mbid>?inc=releases+release-groups&fmt=json"`, compare the returned shape against `RecordingRelease`/`recordingLookupResponse` struct tags, and adjust if the nesting differs before trusting the assumption in production |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < N/A (see Sampling Rate — per-package scoped, not globally timed)
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
