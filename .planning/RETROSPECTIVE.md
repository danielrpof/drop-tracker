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

## Milestone: v1.2 — Cleanup & Display Fixes

**Shipped:** 2026-08-24
**Phases:** 2 (12-13) | **Plans:** 6 | **Sessions:** not tracked

### What Was Built
- `CoverArt.tsx`'s stale-placeholder bug fixed via a `useEffect([src])` reset, fixing History, Watchlist, and search-result rows from one shared-component change
- Deezer fan-count-based search popularity ranking (`Client.SearchArtists`, stable descending sort) and a MusicBrainz `country`-code disambiguation fallback for search results, absorbing backlog Phase 999.1
- History cards for guest-feature and deluxe-change events now render a release date, sourced via a new per-recording MusicBrainz lookup with a precision-aware earliest-date rule and a per-cycle rate cap
- Guest-feature release cards render album art, matching new-release cards
- A new hand-rolled `internal/artistart` matcher (strict close-name equality + guarded shared-album-title tie-break, fail-closed on ambiguity) resolves MusicBrainz artist art from Deezer, wired into both add-time and a cooldown-bounded startup backfill sweep coordinated by a shared `ActivityGate` — absorbing backlog Phase 999.2

### What Worked
- Fixing a bug in one shared component (`CoverArt.tsx`) automatically fixed all three consumers (History, Watchlist, search) with zero call-site changes — no need to touch or re-test each caller
- Fail-closed design for the MusicBrainz→Deezer artist-art matcher (reject on any ambiguity rather than guess) traded a small number of missing photos for zero misattributed ones, matching the phase's own threat model
- Code-review warnings found during Phase 13's UAT verification (a date-parsing panic, a stats double-count, an `ActivityGate` leak on panic) were fixed in place immediately with regression tests instead of deferred to a follow-up phase, continuing the pattern v1.1 identified as worth doing more of

### What Was Inefficient
- Phases 12 and 13 ran as ad-hoc post-v1.1 cleanup with no REQUIREMENTS.md and no milestone version assigned until close time — `/gsd-complete-milestone` was first invoked with a stale "1.1" argument (already shipped/tagged) and had to be redirected to v1.2 mid-workflow. Assigning the next milestone version when this cleanup work started, rather than only at close, would have avoided the confusion.
- The milestone-close CLI (`gsd-tools.cjs query milestone.complete`) couldn't detect Phase 12/13 as belonging to a milestone (they weren't grouped under a `### v1.2 ...` heading in ROADMAP.md) and returned 0 phases/plans/accomplishments — the archive, MILESTONES.md entry, and phase-directory move all had to be done manually. Grouping ad-hoc phases under a milestone heading in ROADMAP.md as soon as a version is decided (not just at close) would let the automation work correctly.
- Windows dev-machine limitations (`go test -race` unusable, MusicBrainz TLS failure over WSL2) recurred again as a known, already-documented cost rather than something newly discovered.

### Patterns Established
- `useEffect([dep]) → reset` is the standard fix for a retained component whose failure/error state derives from a prop that can change without a remount
- `slices.SortStableFunc` (not `SortFunc`) is the standard choice for any popularity/relevance-style sort where ties are common and the pre-sort order carries meaning
- A shared `ActivityGate` priority-yielding primitive is the standard way to coordinate two independent consumers (an interactive path and a background sweep) against one external rate budget, instead of giving the background consumer its own budget
- Fail-closed strict-match + guarded-tie-break is the standard shape for any cross-source identity matching where a wrong match is worse than no match

### Key Lessons
1. Assign and record a milestone version (even provisionally) as soon as post-ship cleanup work starts, and group it under that version's heading in ROADMAP.md — waiting until `/gsd-complete-milestone` runs to decide the version number causes both human confusion and automation misdetection.
2. Fail-closed is worth the cost for any feature that attaches identity data (a photo, a name) from a second source with imperfect matching — a wrong result is worse than a missing one.
3. Closing code-review/UAT-surfaced warnings immediately, in the same phase, continues to beat deferring them — this is the second milestone running where that held true.

### Cost Observations
- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry captured for v1.2, consistent with v1.1

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | not tracked | 7 | Initial MVP; milestone never formally closed via `/gsd-complete-milestone` |
| v1.1 | not tracked | 5 (08-11.1) | First milestone closed via the full `/gsd-complete-milestone` workflow; added a dedicated tech-debt-closure phase pattern |
| v1.2 | not tracked | 2 (12-13) | Ad-hoc post-v1.1 cleanup phases (no REQUIREMENTS.md, version assigned only at close) closed via `/gsd-complete-milestone`; required manual archive/MILESTONES.md correction since the phases weren't pre-grouped under a milestone heading |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|--------------------|
| v1.0 | not measured | not enforced | — |
| v1.1 | backend 83.5%+, frontend 70%+ (both CI-enforced) | 80% backend / 70% frontend gate | buffered-channel semaphore (no worker-pool lib), hand-rolled accessible combobox (no UI lib) |
| v1.2 | backend/frontend suites extended, gates held at 80%/70% throughout | 80% backend / 70% frontend gate (unchanged) | `internal/artistart` fail-closed matcher + `ActivityGate` primitive (stdlib-only, no new deps) |

### Top Lessons (Verified Across Milestones)

1. Close out a milestone's archival step promptly after shipping — letting phase directories accumulate un-archived corrupts the next milestone's close (v1.1).
2. Assign a milestone version and group phases under it in ROADMAP.md as soon as post-ship work starts, not just when `/gsd-complete-milestone` runs — otherwise both humans and the close automation lose track of which phases belong to which version (v1.2).
3. Fixing code-review/UAT-surfaced warnings inline, in the same phase they're found, beats deferring them — held true across both v1.1 and v1.2.
