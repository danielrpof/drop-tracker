# Roadmap: drop-tracker

## Overview

drop-tracker was built outward from the data layer: Postgres schema + config + health-checked skeleton, then a tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications, then the embedded React UI, then single-image containerization and the full GitHub Actions CI/CD pipeline that is the actual point of the project. v1.1–v1.2 hardened it (frontend tests, CI coverage gates, event retention, concurrent polling, display-bug cleanup). v1.3 delivered the deployment-readiness chain that needs no host — passphrase gate, PR coverage-diff comment, rollback-safe migrations — and deferred the actual VPS deploy until hardware exists.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- ✅ **v1.3 Continuous Deployment** — Phases 14-17 (shipped partial 2026-09-09; **Phase 17 deferred**)

Full phase-by-phase detail for every shipped milestone is archived under `.planning/milestones/v[X.Y]-ROADMAP.md`. Requirement archives: `.planning/milestones/v[X.Y]-REQUIREMENTS.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

**Deferred:** **Phase 17 — Automated VPS Deploy with Health-Gated Rollback** (DPLY-01…08). Blocked on a provisioned VPS + domain the developer does not have yet. `discuss-phase` context was already gathered — archived at `.planning/milestones/v1.3-phases/17-automated-vps-deploy-with-health-gated-rollback/` (`17-CONTEXT.md`, `17-DISCUSSION-LOG.md`). Un-defer it as its own milestone cycle once a box exists; the `/ready` probe from v1.4 is being built for its health-gate to consume.

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] Phase 1: Foundation — Data Layer, Config & Health
- [x] Phase 2: Watchlist Core
- [x] Phase 3: External Clients & Search
- [x] Phase 4: Detection Engine
- [x] Phase 5: Discord Notifications
- [x] Phase 6: Frontend & Release History
- [x] Phase 7: Containerization & CI/CD Pipeline

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] Phase 8: Frontend Test Suite
- [x] Phase 9: CI Coverage Gates
- [x] Phase 10: Event Retention Window
- [x] Phase 11: Bounded Concurrent Polling
- [x] Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)

</details>

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking
- [x] Phase 13: Fix History Dates, Guest-Feature Art & Artist Art

</details>

<details>
<summary>✅ v1.3 Continuous Deployment (Phases 14-17) — SHIPPED PARTIAL 2026-09-09</summary>

- [x] Phase 14: Instance Passphrase Gate (7/7 plans) — completed 2026-09-01
- [x] Phase 15: PR Coverage-Diff Comment (3/3 plans) — completed 2026-09-03
- [x] Phase 16: Rollback-Safe Migrations (5/5 plans) — completed 2026-09-05
- [ ] Phase 17: Automated VPS Deploy with Health-Gated Rollback — **DEFERRED** (no VPS); context archived, carries forward as its own milestone

</details>

## Progress

- **v1.0–v1.2:** shipped.
- **v1.3:** shipped partial — 3 of 4 phases (14, 15, 16). Phase 17 deferred, DPLY-01…08 carried forward.

## Backlog

*(none currently)*
