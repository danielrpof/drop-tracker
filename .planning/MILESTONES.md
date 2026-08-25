# Milestones

## v1.2 Cleanup & Display Fixes (Shipped: 2026-08-24)

**Phases completed:** 2 phases (12-13), 6 plans, 17 tasks

**Key accomplishments:**

- Fixed `CoverArt.tsx`'s stale-placeholder bug — the shared component now resets its failed-load state via `useEffect([src])` when `src` changes on a retained instance, fixing History, Watchlist, and search-result rows at once (Phase 12)
- Added Deezer fan-count-based popularity ranking to artist search (`Client.SearchArtists` sorts descending by `NbFan`, stable on ties) and a MusicBrainz `country`-code disambiguation fallback for when `disambiguation` is blank — absorbing backlog Phase 999.1 (Phase 12)
- History cards for guest-feature and deluxe-change events now show a release date, sourced via a new `internal/musicbrainz.ReleasesForRecording` per-recording lookup with a precision-aware earliest-date rule and a 20-lookup-per-cycle rate cap (Phase 13)
- Guest-feature release cards now show album art, matching the existing new-release card behavior (Phase 13)
- MusicBrainz artists now get real artist art via a new hand-rolled `internal/artistart` matcher (strict close-name + guarded shared-album-title tie-break, fail-closed on ambiguity) wired into both add-time resolution and a cooldown-bounded startup backfill sweep, coordinated by a shared `ActivityGate` so both stay within MusicBrainz's rate budget — absorbing backlog Phase 999.2 (Phase 13)
- Three code-review warnings surfaced during Phase 13 UAT (a date-parsing panic, a stats double-count, and an `ActivityGate` leak on panic) were fixed in place with regression tests rather than deferred

---

## v1.1 Hardening & Scale Readiness (Shipped: 2026-08-17)

**Phases completed:** 5 phases (08-11.1), 22 plans

**Key accomplishments:**

- Stood up a Vitest + React Testing Library test suite covering the watchlist, search, and history React surfaces, mocking the app's own API boundary rather than raw fetch (Phase 8)
- Wired CI coverage gates that block `build-scan`/`release` when backend Go coverage drops below 80% or frontend coverage drops below 70%, proven live on real GitHub Actions runs in both the red and green direction (Phase 9)
- Added a configurable event-retention window (default 90 days) that hides aged-out history from the UI/API while leaving every row and all detection state (dedup keys, deluxe-change baselines, seed-mode signal) fully intact — soft-delete only, zero hard deletes (Phase 10)
- Replaced sequential per-artist polling with a bounded, env-configurable worker pool for both MusicBrainz and Deezer, and closed a real lost-update race on shared deluxe-change baselines with an atomic `FOR UPDATE`-locked compare-and-set (Phase 11)
- Closed out the milestone's own tech debt: replaced a native `<select>` that failed accessibility/contrast on Windows Chromium with a hand-rolled `aria-activedescendant` combobox, added a blocking `prettier --check` CI gate, and reconciled Nyquist validation status across Phases 8-10 (Phase 11.1)

---
