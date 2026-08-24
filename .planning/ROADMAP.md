# Roadmap: drop-tracker

## Overview

drop-tracker starts from an empty repo and builds outward from the data layer: a Postgres schema, config, and health-checked service skeleton first, then a fully tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications for those events, then the React UI that ties watchlist management and release history together, and finally the single-image containerization and full GitHub Actions CI/CD pipeline (lint, test, security scan, SBOM, semantic versioning, ghcr.io publish) that is the actual point of the project. Each phase produces something a user (or operator) can directly observe working before the next phase builds on it.

v1.1 picked up from a shipped, working v1.0 and closed four peer-reviewed gaps without changing what the app does for its user: the React frontend gained the component test suite it never had, the Full Pipeline started enforcing coverage floors on both languages instead of merely running tests, the events table gained a retention window that hides stale history from display while leaving every detection-critical row in place, and the poller stopped walking the watchlist one artist at a time.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)

Full phase-by-phase detail for both is archived at `.planning/milestones/v1.0-ROADMAP.md` and `.planning/milestones/v1.1-ROADMAP.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

## Phases

**Phase Numbering:**

- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] **Phase 1: Foundation — Data Layer, Config & Health** - Postgres schema/migrations, sqlc, env-based config, structured logging, and a `/health` endpoint the rest of the app is built on (completed 2026-08-05)
- [x] **Phase 2: Watchlist Core** - Users can add, remove, list, and configure per-artist alert preferences through a tested watchlist API (completed 2026-08-06)
- [x] **Phase 3: External Clients & Search** - Rate-limited MusicBrainz/Deezer clients power a live search-proxy and scheduled polling (completed 2026-08-07)
- [x] **Phase 4: Detection Engine** - Poll results are diffed against a "seen" store to reliably detect new releases, guest features, and deluxe/tracklist changes without duplicates or overlapping runs (completed 2026-08-08)
- [x] **Phase 5: Discord Notifications** - Detected events are posted to Discord with distinct formatting per event type, honoring mute preferences (completed 2026-08-08)
- [x] **Phase 6: Frontend & Release History** - Users manage their watchlist and browse detected release history entirely through a web UI (completed 2026-08-11)
- [x] **Phase 7: Containerization & CI/CD Pipeline** - The app ships as a single scanned, versioned, non-root Docker image via an automated GitHub Actions pipeline, with docker-compose for local dev (completed 2026-08-12)

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] **Phase 8: Frontend Test Suite** - The watchlist, search, and history React surfaces get a Vitest + React Testing Library suite that mocks the app's own API boundary (completed 2026-08-12)
- [x] **Phase 9: CI Coverage Gates** - The Full Pipeline blocks the build when Go coverage drops below 80% or frontend coverage drops below 70% (completed 2026-08-13)
- [x] **Phase 10: Event Retention Window** - History and API hide events older than a configurable window (default 90 days) while every row and all detection state stay intact (completed 2026-08-13)
- [x] **Phase 11: Bounded Concurrent Polling** - Each source polls several artists at a time through a bounded worker pool, without breaking rate limits, overlap guards, or baseline correctness (completed 2026-08-17)
- [x] **Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)** - Closed the milestone audit's non-blocking tech debt: frontend coverage gaps, a real History filter accessibility bug, a Prettier CI gate, notification-loss observability, and Nyquist validation reconciliation (completed 2026-08-17)

</details>

### Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking

**Goal:** Close two loose ends left after v1.1 closes: (1) `CoverArt.tsx`'s image-load-error state never resets when `src` changes on a retained component instance, so a component that once failed to load keeps showing the placeholder forever even if a later `src` would succeed — flagged in `.planning/v1.1-MILESTONE-AUDIT.md` as pre-existing, non-blocking tech debt; (2) promoted from backlog Phase 999.1 — search results aren't ranked by popularity and same-named artists (e.g. multiple "Drake"s) are hard to disambiguate, since MusicBrainz's search API doesn't rank by popularity and its `disambiguation` field is often blank.
**Requirements**: TBD — no REQ-IDs mapped; `12-CONTEXT.md`'s locked decisions D-01 through D-10 are the authoritative scope and are traced through the plans' `requirements` fields
**Depends on:** Phase 11
**Plans:** 3/3 plans complete

Context:

- CoverArt fix: affects both History and Watchlist rows (shared component). Reset the error state on `src` change, likely via a `useEffect` keyed on `src` or a `key` prop forcing remount.
- Popularity/disambiguation (ex-999.1, captured during Phase 6 UAT 06-04): the Watchlist search UI already renders `disambiguation` when present (`SearchResultsColumns.tsx`) — the gap is upstream ranking, not the UI. Likely needs a popularity signal (Deezer search results carry fan-count data not currently captured by `internal/deezer`) and/or better MusicBrainz result ranking in `internal/httpserver/search.go`.

Plans:
**Wave 1**

- [x] 12-01-PLAN.md — CoverArt error-state reset on `src` change plus its regression test (D-01, D-02) — wave 1
- [x] 12-02-PLAN.md — Deezer fan-count capture and stable descending popularity sort inside the client (D-03, D-04) — wave 1

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 12-03-PLAN.md — MusicBrainz country fallback end-to-end, plus preserved-order and no-fan-count-on-the-wire guardrails (D-05 through D-10) — wave 2

## Progress

| Milestone | Phases | Status | Completed |
| --------- | ------ | ------ | --------- |
| v1.0 MVP | 1-7 | Complete | 2026-08-12 |
| v1.1 Hardening & Scale Readiness | 8-11.1 | Complete | 2026-08-17 |

### Phase 13: Fix History Dates, Guest-Feature Art & Artist Art

**Goal:** Resolve three outstanding display/data bugs users are still hitting after Phase 12: (1) History tab entries (single/feature/deluxe) don't show a release date next to each item; (2) guest-feature release cards don't show album art even though new-release cards do; (3) artist art from MusicBrainz still doesn't render, despite Deezer artists being linkable to MusicBrainz artists so Deezer pictures could be used. Absorbs backlog Phase 999.2 (Deezer artist-art backfill) where it overlaps with bug 3. Must actually resolve these — no repeat phases for the same unfixed behavior.
**Requirements**: D-01 through D-09 (13-CONTEXT.md locked decisions — no REQUIREMENTS.md IDs are mapped to this phase)
**Depends on:** Phase 12
**Plans:** 2/3 plans executed

Plans:
**Wave 1**

- [x] 13-01-PLAN.md — Guest-feature release date & cover art, end-to-end through the History card (D-01–D-05, plus guest-feature date rendering and per-recording lookup error isolation)
- [x] 13-02-PLAN.md — artistart matcher package: close-name match, shared-album-title tie-break, fail-closed policy, and the ListArtistsMissingImage query (D-06, D-08, D-09)

**Wave 2** *(blocked on Wave 1 completion)*

- [ ] 13-03-PLAN.md — Add-time artist-art wiring and the one-time startup backfill sweep (D-06, D-07, D-09)

## Backlog

*(none currently — Phase 999.2 was absorbed into Phase 13)*
