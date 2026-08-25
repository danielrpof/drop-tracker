# Roadmap: drop-tracker

## Overview

drop-tracker starts from an empty repo and builds outward from the data layer: a Postgres schema, config, and health-checked service skeleton first, then a fully tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications for those events, then the React UI that ties watchlist management and release history together, and finally the single-image containerization and full GitHub Actions CI/CD pipeline (lint, test, security scan, SBOM, semantic versioning, ghcr.io publish) that is the actual point of the project. Each phase produces something a user (or operator) can directly observe working before the next phase builds on it.

v1.1 picked up from a shipped, working v1.0 and closed four peer-reviewed gaps without changing what the app does for its user: the React frontend gained the component test suite it never had, the Full Pipeline started enforcing coverage floors on both languages instead of merely running tests, the events table gained a retention window that hides stale history from display while leaving every detection-critical row in place, and the poller stopped walking the watchlist one artist at a time. v1.2 then closed the two items left in the backlog plus a round of History-tab display bugs found in everyday use — search popularity ranking, artist-art backfill, and missing release dates/album art on History cards — without adding new capability.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)

Full phase-by-phase detail for all three is archived at `.planning/milestones/v1.0-ROADMAP.md`, `.planning/milestones/v1.1-ROADMAP.md`, and `.planning/milestones/v1.2-ROADMAP.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

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

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] **Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking** - Fixed the shared `CoverArt` component's stale-placeholder bug and added Deezer-fan-count popularity ranking plus a MusicBrainz country-code disambiguation fallback to artist search (completed 2026-08-19)
- [x] **Phase 13: Fix History Dates, Guest-Feature Art & Artist Art** - History cards now show release dates and guest-feature album art, and MusicBrainz artists get real artist art via a fail-closed MusicBrainz→Deezer matcher wired into add-time and a startup backfill sweep (completed 2026-08-24)

</details>

## Progress

| Milestone | Phases | Status | Completed |
| --------- | ------ | ------ | --------- |
| v1.0 MVP | 1-7 | Complete | 2026-08-12 |
| v1.1 Hardening & Scale Readiness | 8-11.1 | Complete | 2026-08-17 |
| v1.2 Cleanup & Display Fixes | 12-13 | Complete | 2026-08-24 |

## Backlog

*(none currently)*
