# Milestones

## v1.1 Hardening & Scale Readiness (Shipped: 2026-08-17)

**Phases completed:** 5 phases (08-11.1), 22 plans

**Key accomplishments:**

- Stood up a Vitest + React Testing Library test suite covering the watchlist, search, and history React surfaces, mocking the app's own API boundary rather than raw fetch (Phase 8)
- Wired CI coverage gates that block `build-scan`/`release` when backend Go coverage drops below 80% or frontend coverage drops below 70%, proven live on real GitHub Actions runs in both the red and green direction (Phase 9)
- Added a configurable event-retention window (default 90 days) that hides aged-out history from the UI/API while leaving every row and all detection state (dedup keys, deluxe-change baselines, seed-mode signal) fully intact — soft-delete only, zero hard deletes (Phase 10)
- Replaced sequential per-artist polling with a bounded, env-configurable worker pool for both MusicBrainz and Deezer, and closed a real lost-update race on shared deluxe-change baselines with an atomic `FOR UPDATE`-locked compare-and-set (Phase 11)
- Closed out the milestone's own tech debt: replaced a native `<select>` that failed accessibility/contrast on Windows Chromium with a hand-rolled `aria-activedescendant` combobox, added a blocking `prettier --check` CI gate, and reconciled Nyquist validation status across Phases 8-10 (Phase 11.1)

---
