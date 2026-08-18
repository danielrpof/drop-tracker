# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — Hardening & Scale Readiness

**Shipped:** 2026-08-17
**Phases:** 5 (08-11.1) | **Plans:** 22 | **Sessions:** not tracked

### What Was Built
- Vitest + React Testing Library component test suite for the watchlist, search, and history React surfaces, mocking the app's API boundary
- CI coverage gates blocking `build-scan`/`release` below 80% backend / 70% frontend, proven live in both the red and green direction on real GitHub Actions runs
- A configurable event-retention window (soft-delete/filter, default 90 days) that hides aged-out history while leaving every row and all detection state intact
- Bounded, env-configurable worker-pool polling for MusicBrainz and Deezer, replacing sequential per-artist iteration, plus an atomic `FOR UPDATE`-locked CTE closing a real lost-update race on shared deluxe-change baselines
- A dedicated tech-debt phase (11.1) closing every item the milestone audit flagged, including a real accessibility bug in the History filter UI

### What Worked
- Landing the highest-risk change (bounded concurrency, Phase 11) last, behind a working coverage harness (Phases 8-9), meant regressions in the concurrency rewrite were caught by tests rather than discovered live
- Using real GitHub Actions runs (not just local assertions) to close backstop-tier UAT truths for the coverage gates — directly observed red/green transitions on a scratch branch, never on `main`
- Folding related CI checks into an existing job (Prettier into `frontend-test`) instead of adding new pipeline surface for each small gate

### What Was Inefficient
- Tech debt accumulated across Phases 08-11 needed an entire extra phase (11.1) to close rather than being resolved inline phase-by-phase — worth watching whether review findings can be closed closer to when they're found
- v1.0 was never formally closed through `/gsd-complete-milestone` (no MILESTONES.md entry, no archived phase directories, no git tag) before v1.1 work started. Running the milestone-close workflow for v1.1 swept up all 12 phases (1-11.1) instead of just v1.1's 5, and required manual correction of the archive and MILESTONES.md entry after the fact
- Windows dev-machine limitations (`go test -race` unusable, MusicBrainz TLS failure over WSL2) recurred again this milestone as a known, already-documented cost rather than something newly discovered

### Patterns Established
- `FOR UPDATE`-locked `UPDATE...RETURNING` CTE is the standard fix for a check-then-act race once concurrency is introduced — used for the deluxe-change baseline, reusable for any future shared-mutable-state race
- A buffered-channel semaphore is sufficient for bounded per-cycle fan-out concurrency — no third-party worker-pool library needed
- New CI gates (coverage, formatting) get added to an existing job's `needs:`/steps rather than spawning a new job per gate, keeping the pipeline graph simple

### Key Lessons
1. Run the milestone-close archival step promptly when a milestone actually ships, even if the full ceremony (tag, retrospective) waits — deferring it let phase directories pile up un-archived and corrupted the next milestone's close.
2. A scoped tech-debt phase driven directly by a milestone audit's findings (13 locked decisions, each mapped to one audit item) is an effective, low-risk way to close review findings without reopening already-verified phases.
3. Concurrency-introducing phases benefit from landing last in a milestone, behind a working test/coverage harness that can catch what the rewrite breaks.

### Cost Observations
- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry was captured for v1.1 — worth deciding whether to start tracking this in v1.2 if cost visibility becomes valuable

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | not tracked | 7 | Initial MVP; milestone never formally closed via `/gsd-complete-milestone` |
| v1.1 | not tracked | 5 (08-11.1) | First milestone closed via the full `/gsd-complete-milestone` workflow; added a dedicated tech-debt-closure phase pattern |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|--------------------|
| v1.0 | not measured | not enforced | — |
| v1.1 | backend 83.5%+, frontend 70%+ (both CI-enforced) | 80% backend / 70% frontend gate | buffered-channel semaphore (no worker-pool lib), hand-rolled accessible combobox (no UI lib) |

### Top Lessons (Verified Across Milestones)

1. Close out a milestone's archival step promptly after shipping — letting phase directories accumulate un-archived corrupts the next milestone's close (v1.1).
