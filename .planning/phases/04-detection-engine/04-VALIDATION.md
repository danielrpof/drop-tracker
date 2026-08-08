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

*(Task ID / Plan / Wave / Threat Ref are assigned once PLAN.md tasks and the planner's threat model exist — this draft maps phase requirements to the test commands from `04-RESEARCH.md`'s Validation Architecture. Refined to real Task IDs during execution / `/gsd-validate-phase`.)*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | DTCT-01 | TBD | New release-group creates a `new_release` event row; a re-detected group does not duplicate it | integration (real Postgres, `testutil.NewTestPool`) | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_NewRelease` | ❌ Wave 0 — `internal/detection/musicbrainz_test.go` | ⬜ pending |
| TBD | TBD | TBD | DTCT-02 | TBD | Expanded tracklist on an already-seen group fires `deluxe_change`; first-ever comparison cycle does NOT false-positive (baseline-tracking gap, RESEARCH.md Pitfall #1) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_DeluxeChange` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | DTCT-02 | TBD | `ReleasesByReleaseGroup` decodes the assumed JSON shape correctly, sums multi-disc `track-count` (Assumption A1) | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run TestReleasesByReleaseGroup -short` | ❌ Wave 0 — `internal/musicbrainz/releases_test.go` | ⬜ pending |
| TBD | TBD | TBD | DTCT-03 | TBD | Non-primary artist-credit recording fires `guest_feature`; primary-credit recordings never do (Pitfall #3) | unit (httptest fixture) + integration | `go test ./internal/detection/... -run TestIsGuestFeature -short` then `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetectMusicBrainz_GuestFeature` | ❌ Wave 0 — `internal/detection/musicbrainz_test.go` | ⬜ pending |
| TBD | TBD | TBD | DTCT-03 | TBD | `RecordingsByArtist` decodes the assumed JSON shape, paginates bounded at maxPages=10 (Assumption A2) | unit (httptest.Server fixture) | `go test ./internal/musicbrainz/... -run TestRecordingsByArtist -short` | ❌ Wave 0 — `internal/musicbrainz/recordings_test.go` | ⬜ pending |
| TBD | TBD | TBD | DTCT-04 | TBD | `InsertEvent` returns 1 on first insert, 0 on duplicate dedup key (Pitfall #2, Assumption A4) | integration (real Postgres) | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestInsertEvent_Idempotent` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | DTCT-04 (D-13/D-16) | TBD | Seed-mode: first cycle for a new artist pre-sets `notified_at`; removing/re-adding an artist does not re-seed (event rows survive watchlist row deletion via `artist_id`, not watchlist id) | integration | `TEST_DATABASE_URL=... go test ./internal/detection/... -run TestDetector_SeedMode` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | DTCT-05 | TBD | Already covered — no new test needed | — | (existing `internal/poller/poller_test.go` coverage from Phase 3) | ✓ already exists | ⬜ pending |
| TBD | TBD | TBD | WLST-05/WLST-06 (D-17/D-18) | TBD | Release-type / muted-event-type filters gate event creation before insert, including the `deluxe` pseudo-type case (Pitfall #7) | unit | `go test ./internal/detection/... -run TestFilter -short` | ❌ Wave 0 — `internal/detection/filter_test.go` | ⬜ pending |

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
