---
phase: 04
slug: detection-engine
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-08
---

# Phase 04 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) — unchanged from Phases 1–3 |
| **Config file** | none — plain `go test ./...` |
| **Quick run command** | `go test -short ./...` (skips DB-backed tests via `testutil.RequirePostgresDSN`'s `testing.Short()` gate) |
| **Full suite command** | `TEST_DATABASE_URL=... go test ./...` (or `make test`) |
| **Estimated runtime** | ~30s (short), ~120–150s (full; adds `internal/detection` real-Postgres integration tests plus two new `internal/musicbrainz` httptest-backed fixture suites over Phase 3's baseline) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./...`
- **After every plan wave:** Run `TEST_DATABASE_URL=... go test ./...` (full suite)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120s (Phase 3 baseline of ~90s, extended by the new `internal/detection` package's real-Postgres integration tests)

---

## Per-Task Verification Map

*(Task IDs, plans, waves and threat refs assigned during planning on 2026-08-08. Status is refined during execution / `/gsd-validate-phase`.)*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 04-01 T2 | 04-01 | 1 | DTCT-01 | T-04-01, T-04-03 | New release-group creates a `new_release` event row end-to-end through a real poll cycle, with the D-12 snapshot populated | integration (real Postgres, `testutil.NewTestPool`) | `TEST_DATABASE_URL=... go test ./internal/detection/... ./internal/poller/... -run 'TestDetectMusicBrainz_NewRelease\|TestPoller_RunMusicBrainzCycle_RecordsNewRelease' -count=1` | ❌ Wave 0 — `internal/detection/detector_test.go` | ⬜ pending |
| 04-01 T3 | 04-01 | 1 | DTCT-04 | T-04-03 | `InsertEvent` returns 1 on first insert, 0 on duplicate dedup key; the snapshot is write-once (Pitfall #2, Assumption A4, D-20) | integration (real Postgres) | `TEST_DATABASE_URL=... go test ./internal/detection/... -run 'TestInsertEvent_' -count=1` | ❌ Wave 0 | ⬜ pending |
| 04-01 T3 | 04-01 | 1 | DTCT-05 | T-04-05 | A second cycle for the same source returns `ErrCycleInProgress` and writes nothing; the guard releases even when detection errors | integration | `TEST_DATABASE_URL=... go test ./internal/poller/... -run 'TestPoller_RunMusicBrainzCycle_' -count=1` | ⚠ extends existing Phase 3 coverage | ⬜ pending |
| 04-02 T1 | 04-02 | 2 | WLST-05/WLST-06 (D-17/D-18) | T-04-09 | Release-type / muted-event-type filters gate event creation before insert, including the `deluxe` pseudo-type case (Pitfall #7) | unit + integration | `go test ./internal/detection/... -run TestFilter -short -count=1` | ❌ Wave 0 — `internal/detection/filter_test.go` | ⬜ pending |
| 04-02 T2 | 04-02 | 2 | DTCT-04 (D-13..D-16) | T-04-07 | Seed mode pre-sets `notified_at` per (artist, source); a remove-then-re-add resumes rather than re-seeds | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetector_ -count=1` | ❌ Wave 0 | ⬜ pending |
| 04-02 T3 | 04-02 | 2 | DTCT-01 (Deezer half) | T-04-08 | Deezer albums become `new_release` rows in their own ID namespace; nil `deezer_id` still skips | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectDeezer_ -count=1` | ❌ Wave 0 — `internal/detection/deezer_test.go` | ⬜ pending |
| 04-03 T1 | 04-03 | 3 | DTCT-03 | T-04-14, T-04-15 | `RecordingsByArtist` decodes the assumed envelope, routes through the shared limiter/User-Agent helper, paginates bounded at maxPages=10 (Assumption A2) | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run TestRecordingsByArtist -short -count=1` | ❌ Wave 0 — `internal/musicbrainz/recordings_test.go` | ⬜ pending |
| 04-03 T1 | 04-03 | 3 | DTCT-03 | T-04-12 | Non-primary artist-credit recording fires `guest_feature`; primary-credit recordings never do (Pitfall #3) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature -count=1` | ❌ Wave 0 | ⬜ pending |
| 04-03 T2 | 04-03 | 3 | DTCT-03 | T-04-12, T-04-13, T-04-16 | Empty/malformed `artist-credit` never panics; a page-ceiling-truncated browse is visible as `page_ceiling_reached` in structured logs (Pitfall #4) | unit + integration | `go test ./internal/detection/... -run TestIsGuestFeature -short -count=1` | ❌ Wave 0 | ⬜ pending |
| 04-04 T1 | 04-04 | 4 | DTCT-02 | T-04-18, T-04-20 | `ReleasesByReleaseGroup` decodes the assumed shape, sums multi-disc `track-count`, routes through the shared helper (Assumption A1) | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run 'TestReleasesByReleaseGroup\|TestRelease_TrackCount' -short -count=1` | ❌ Wave 0 — `internal/musicbrainz/releases_test.go` | ⬜ pending |
| 04-04 T1 | 04-04 | 4 | DTCT-02 | T-04-22 | Expanded tracklist on an already-seen group fires `deluxe_change`; the first-ever measurement establishes a baseline and fires nothing (Pitfall #1) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_DeluxeChange -count=1` | ❌ Wave 0 | ⬜ pending |
| 04-04 T2 | 04-04 | 4 | DTCT-02 (D-04/D-17) | T-04-19, T-04-21 | A brand-new group triggers zero release-detail fetches; an artist without the `deluxe` preference costs zero requests; the page ceiling is bounded | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -count=1` | ❌ Wave 0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `internal/musicbrainz/releases_test.go` — httptest.Server fixture for `/ws/2/release?release-group=` using the `[ASSUMED]` shape from `04-RESEARCH.md` Pattern 2 (A1)
- [ ] `internal/musicbrainz/recordings_test.go` — httptest.Server fixture for `/ws/2/recording?artist=` using the `[ASSUMED]` shape from `04-RESEARCH.md` Pattern 3 (A2)
- [ ] `internal/db/migrations/000003_events.up.sql` + `.down.sql` — new `events` table (seen-store + event log, D-09)
- [ ] `queries/events.sql` — `InsertEvent`, `HasAnyEvent`, `ListExternalIDs`, a track-count baseline query (shape depends on the deluxe-change baseline design), `ListUnnotified` (D-11, Phase 5 groundwork)
- [ ] `internal/detection/` — new package, no existing tests
- [ ] A real-Postgres test proving `ON CONFLICT DO NOTHING`'s row-count semantics (Assumption A4) before any code relies on it

*(No new test framework install needed — `go test` is already the project's established tool.)*

---

## Manual-Only Verifications

*All phase behaviors have automated verification — MusicBrainz calls are mocked via `httptest.Server` per CLAUDE.md's no-live-external-calls-in-CI testing constraint. Live MusicBrainz reachability (for optional manual UAT re-verification of Assumptions A1/A2) is an acknowledged environmental gap this session — see `04-RESEARCH.md` Environment Availability and Phase 3's precedent in `03-VERIFICATION.md`.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
