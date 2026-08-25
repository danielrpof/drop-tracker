---
phase: 12
slug: cleanup-coverart-reset-search-popularity-ranking
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-08-18
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `net/http/httptest` (backend) / Vitest 4.1.10 (frontend) |
| **Config file** | `web/vitest.config.ts` (frontend); none — stdlib (backend) |
| **Quick run command** | `go test ./internal/deezer/... ./internal/musicbrainz/... ./internal/httpserver/... -short -race -count=1` (backend); `pnpm test -- <name>` scoped run (frontend) |
| **Full suite command** | `make test` (backend, requires `db-up`); `pnpm test` (frontend, coverage always enabled) |
| **Estimated runtime** | ~30-60s scoped backend run; ~10-20s scoped frontend run |

---

## Sampling Rate

- **After every task commit:** backend — `go test ./internal/deezer/... ./internal/musicbrainz/... ./internal/httpserver/... -short -race -count=1`; frontend — `pnpm test -- CoverArt` / `pnpm test -- SearchResultsColumns` (scoped run)
- **After every plan wave:** backend — `make test` (full, real-Postgres integration suite); frontend — `pnpm test` (full suite, 70% coverage gate per `vitest.config.ts:56-61`)
- **Before `/gsd-verify-work`:** Full suite must be green — `make coverage-gate` (backend 80% threshold) and `pnpm test` (frontend 70% thresholds)
- **Max feedback latency:** 60 seconds

---

## Per-Task Verification Map

| Task ID | Decision | Requirement | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|----------|-------------|-----------------|-----------|-------------------|-------------|--------|
| 12-01-xx | D-01/D-02 | CoverArt clears `failed` when `src` changes | N/A | unit (RTL) | `pnpm test -- CoverArt` | ❌ Wave 0 | ⬜ pending |
| 12-02-xx | D-03 | `deezer.Artist.NbFan` decodes `nb_fan` | N/A | unit | `go test ./internal/deezer/... -run TestSearchArtists_DecodesFixture` | ✅ existing, extend | ⬜ pending |
| 12-02-xx | D-04 | Deezer results sort by `NbFan` descending | N/A | unit | `go test ./internal/deezer/... -run TestSearchArtists_Sort` | ❌ Wave 0 (replaces `TestSearchArtists_PreservesUpstreamOrderNoSorting`) | ⬜ pending |
| 12-02-xx | D-05 | `SearchArtist` wire shape unchanged (no `nb_fan` leak) | N/A | unit | `go test ./internal/httpserver/... -run TestNewDeezerSource_MapsFields` | ✅ existing, extend | ⬜ pending |
| 12-03-xx | D-06/D-07 | MusicBrainz pipeline order preserved end-to-end | N/A | unit | `go test ./internal/httpserver/... -run TestNewMusicBrainzSource` | ❌ Wave 0 | ⬜ pending |
| 12-03-xx | D-09 | `musicbrainz.Artist.Country` decodes `country` | N/A | unit | `go test ./internal/musicbrainz/... -run TestSearchArtists_DecodesFixture` | ✅ existing, extend | ⬜ pending |
| 12-03-xx | D-10 | `SearchArtist.Country` populated; `SearchResultRow` renders disambiguation-or-country fallback | N/A (plain text node, no `dangerouslySetInnerHTML`) | unit (Go) + unit (RTL) | `go test ./internal/httpserver/... -run TestNewMusicBrainzSource_MapsFields` && `pnpm test -- SearchResultsColumns` | ❌ Wave 0 (both sides) | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `web/app/components/common/CoverArt.test.tsx` — new file, covers D-01/D-02 (no existing test file for this component at all)
- [ ] `internal/deezer/search_test.go` — extend/rename `TestSearchArtists_PreservesUpstreamOrderNoSorting`, covers D-04
- [ ] `internal/httpserver/search_test.go` — new Country-mapping assertion + new order-preservation test, covers D-07/D-09/D-10 (Go side)
- [ ] `web/app/components/watchlist/SearchResultsColumns.test.tsx` — extend with a disambiguation-blank/country-present fixture case, covers D-10 (frontend side)

---

## Manual-Only Verifications

*None — all phase behaviors have automated verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 60s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
