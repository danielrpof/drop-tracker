---
phase: 03
slug: external-clients-search
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-07
---

# Phase 03 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (`go test`) — no third-party test framework, matches Phase 1/2 convention |
| **Config file** | none — plain `go test ./...` (Makefile targets available per Phase 2 precedent) |
| **Quick run command** | `go test -short ./...` (skips DB-backed tests per `testutil.RequirePostgresDSN`'s `testing.Short()` gate) |
| **Full suite command** | `TEST_DATABASE_URL=... go test ./...` (or `make test`) |
| **Estimated runtime** | ~30s (short), ~90–120s (full; adds 4 new httptest-backed packages over Phase 2's baseline) |

---

## Sampling Rate

- **After every task commit:** Run `go test -short ./...`
- **After every plan wave:** Run `TEST_DATABASE_URL=... go test ./...` (full suite)
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~90s (Phase 2 baseline; full suite gains `internal/musicbrainz`, `internal/deezer`, `internal/poller`, and `internal/httpserver` search-endpoint tests)

---

## Per-Task Verification Map

*(Task ID / Plan / Wave / Threat Ref are assigned once PLAN.md tasks and the planner's threat model exist — this draft maps phase requirements to the test commands from `03-RESEARCH.md`'s Validation Architecture. Refined to real Task IDs during execution / `/gsd-validate-phase`.)*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | TBD | TBD | CLNT-03 | TBD | `GET /search?q=` returns combined MusicBrainz + Deezer results | unit (httptest.Server-backed fakes for both upstreams) | `go test ./internal/httpserver/... -run TestSearch -short` | ❌ Wave 0 — `internal/httpserver/search_test.go` | ⬜ pending |
| TBD | TBD | TBD | CLNT-03 (D-03) | TBD | One upstream failing/timing out still returns the other source's results + a per-source error flag | unit | `go test ./internal/httpserver/... -run TestSearch_PartialFailure -short` | ❌ Wave 0 | ⬜ pending |
| TBD | TBD | TBD | WLST-01 | TBD | MusicBrainz artist search returns MBID, name, disambiguation, type (D-11) | unit (httptest.Server fake) | `go test ./internal/musicbrainz/... -short` | ❌ Wave 0 — `internal/musicbrainz/search_test.go` | ⬜ pending |
| TBD | TBD | TBD | CLNT-01 | TBD | MusicBrainz release-groups browse-by-artist, self-rate-limited (D-10) | unit (httptest.Server fake + fake clock or short-interval limiter) | `go test ./internal/musicbrainz/... -run TestReleaseGroups -short` | ❌ Wave 0 — `internal/musicbrainz/releasegroups_test.go` | ⬜ pending |
| TBD | TBD | TBD | CLNT-02 | TBD | Deezer artist-albums fetch, self-rate-limited (D-12) | unit (httptest.Server fake) | `go test ./internal/deezer/... -short` | ❌ Wave 0 — `internal/deezer/albums_test.go` | ⬜ pending |
| TBD | TBD | TBD | CLNT-01, CLNT-02 | TBD | Two independent poll cycles never share a limiter/goroutine; per-source overlap guard skips an overlapping tick (D-08, D-09) | unit (fake clients + manual trigger, no real cron ticks) | `go test ./internal/poller/... -short` | ❌ Wave 0 — `internal/poller/poller_test.go` | ⬜ pending |
| TBD | TBD | TBD | D-06 | TBD | Deezer poll cycle skips artists with nil `deezer_id` without erroring | unit | `go test ./internal/poller/... -run TestDeezerPoll_SkipsNilDeezerID -short` | ❌ Wave 0 | ⬜ pending |

---

## Wave 0 Requirements

- [ ] `internal/musicbrainz/search_test.go`, `internal/musicbrainz/releasegroups_test.go` — httptest.Server fakes for `/ws/2/artist` and `/ws/2/release-group` responses (use live-verified JSON shapes from `03-RESEARCH.md` as fixtures)
- [ ] `internal/deezer/search_test.go`, `internal/deezer/albums_test.go` — httptest.Server fakes for `/search/artist` and `/artist/{id}/albums`, including a quota-error fixture (Assumption A1)
- [ ] `internal/httpserver/search_test.go` — combined-endpoint test doubles for both `ArtistSearcher` interfaces (D-02 no-dedup, D-03 partial-results)
- [ ] `internal/poller/poller_test.go` — overlap-guard and independent-cycle tests; no framework install needed, all stdlib `testing`

*(No test framework install needed — `go test` is already the project's established tool.)*

---

## Manual-Only Verifications

*All phase behaviors have automated verification — MusicBrainz/Deezer calls are mocked via `httptest.Server` per CLAUDE.md's no-live-external-calls-in-CI testing constraint.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
