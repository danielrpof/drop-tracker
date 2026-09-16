---
phase: quick/260916-dvy
plan: 01
subsystem: docs
tags: [notifier, digest, adr, glossary, roadmap]

requires:
  - phase: 21-real-time-digest-mutual-exclusion
    provides: 21-CONTEXT.md's original gathered context (fail-open, per-pass log, now-anchored staleness, functional option, looped invariant tests, backend-only assumption) that this task supersedes
provides:
  - "docs/adr/0002-one-outbox-one-sender-lock.md recording the shared Notifier.notifying sender lock decision"
  - "root CONTEXT.md glossary entries for Outbox, Pending event, Flush"
  - "21-CONTEXT.md revised with D-01..D-07 as the phase's locked decisions"
  - "21-DISCUSSION-LOG.md post-grilling revision audit trail (9 items)"
  - "ROADMAP.md Phase 21/22 success criteria and notes aligned with the settled decisions"
affects: [22-scheduled-digest-send]

actuals:
  tokens: 12000
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Single shared CAS lock (Notifier.notifying) as the exclusion mechanism between real-time and digest sends, rather than per-sender guards"
    - "Staleness cutoff anchored to an event's created_at plus 1-day slack, rather than time.Now()"

key-files:
  created:
    - docs/adr/0002-one-outbox-one-sender-lock.md
  modified:
    - CONTEXT.md
    - .planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md
    - .planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md
    - .planning/ROADMAP.md

key-decisions:
  - "D-01: Mode-transition-only logging (not per-pass) with the flush count on the on-to-off transition"
  - "D-02: Staleness cutoff anchored to ev.CreatedAt minus maxAgeDays minus 1 day, not time.Now()"
  - "D-03: Fail CLOSED on a settings-read error (reverses the 2026-09-11 fail-open lock), Warn-logged, dbOpTimeout-bounded"
  - "D-04: One outbox, one sender lock — every send serializes on Notifier.notifying (ADR 0002)"
  - "D-05: SettingsReader is a required constructor argument, not a functional option"
  - "D-06: Always-visible SPA helper text under the Digest mode row"
  - "D-07: Deterministic k-th-call fakes replace looped timing invariants for concurrency proof"

patterns-established:
  - "ADR 0002 format matches ADR 0001: frontmatter status, Context/Decision/Considered options/Consequences, ~60 lines"

requirements-completed: [21-PG-01, 21-PG-02, 21-PG-03, 21-PG-04, 21-PG-05, 21-PG-06, 21-PG-07, 21-PG-08, 21-PG-09, 21-PG-10]

coverage:
  - id: D1
    description: "ADR 0002 exists with accepted status, all four sections, and is referenced from both 21-CONTEXT.md and ROADMAP.md"
    requirement: "21-PG-01"
    verification:
      - kind: other
        ref: "Task 1 automated verify command (grep-based structural and content checks)"
        status: pass
    human_judgment: false
  - id: D2
    description: "Root CONTEXT.md defines Outbox, Pending event, and Flush as glossary entries, and Digest window's wording aligns to 'pending'"
    requirement: "21-PG-10"
    verification:
      - kind: other
        ref: "Task 1 automated verify command (grep-based checks incl. entry ordering)"
        status: pass
    human_judgment: false
  - id: D3
    description: "21-CONTEXT.md carries D-01..D-07 as settled, with superseded phrasings (fail-open, per-pass log, functional option, looped invariant, backend-only) removed"
    requirement: "21-PG-02, 21-PG-03, 21-PG-04, 21-PG-05, 21-PG-06, 21-PG-07, 21-PG-08, 21-PG-09"
    verification:
      - kind: other
        ref: "Task 2 automated verify command (positive and negative grep checks)"
        status: pass
    human_judgment: false
  - id: D4
    description: "21-DISCUSSION-LOG.md keeps its original audit trail and gains a nine-item post-grilling revision section"
    verification:
      - kind: other
        ref: "Task 2 automated verify command (section-presence and User's-choice-count checks)"
        status: pass
    human_judgment: false
  - id: D5
    description: "ROADMAP.md Phase 21 success criteria and notes, and the Phase 22 scheduling/staleness notes, match the settled decisions; goal lines, plan counts, checklists, and the progress table are untouched"
    verification:
      - kind: other
        ref: "Task 3 automated verify command (content checks plus a git-diff guard against Goal/Plans/checklist/table line changes)"
        status: pass
    human_judgment: false

duration: 20min
completed: 2026-09-16
status: complete
---

# Quick Task 260916-dvy: Record Post-Grilling Phase 21 Decisions Summary

**Recorded ten grilling-session decisions across a new ADR, the root glossary, 21-CONTEXT.md, its discussion log, and ROADMAP.md — the phase's fail-open posture flips to fail-closed, staleness anchors to `created_at`, and every outbox send now serializes on one shared sender lock (ADR 0002).**

## Performance

- **Duration:** ~20 min
- **Tasks:** 3
- **Files modified:** 5 (1 created, 4 modified)

## Accomplishments

- Wrote `docs/adr/0002-one-outbox-one-sender-lock.md`, documenting why an idempotent ack (`AND notified_at IS NULL`) doesn't prevent a double send, and why the fix is one shared `Notifier.notifying` lock rather than per-sender guards or a Postgres advisory lock.
- Added `Outbox`, `Pending event`, and `Flush` to the root `CONTEXT.md` glossary, and aligned `Digest window`'s wording from "unnotified" to "pending".
- Rewrote `21-CONTEXT.md`'s decisions section as D-01..D-07, replacing the original gathered context's fail-open posture, per-pass log line, now-anchored staleness, functional-option wiring, looped-invariant testing, and backend-only assumption.
- Appended a nine-item post-grilling revision section to `21-DISCUSSION-LOG.md`, preserving its original "Audit trail only" content unchanged.
- Aligned ROADMAP.md's Phase 21 success criteria 2 and 5, its "Notes for the phase planner" bullets (gate location, guards, fail-closed, staleness, testing, operator copy), and Phase 22's scheduling/staleness notes — without touching any Goal line, checklist, plan count, or the progress table.

## Task Commits

Each task was committed atomically:

1. **Task 1: Write ADR 0002 and add the outbox terms to the root glossary** - `295d797` (docs)
2. **Task 2: Revise 21-CONTEXT.md in place and append the post-grilling log section** - `6bf1dd5` (docs)
3. **Task 3: Align ROADMAP.md Phase 21 and Phase 22 with the settled decisions** - `478f5eb` (docs)

_No TDD tasks; all three were docs-only plan tasks._

## Files Created/Modified

- `docs/adr/0002-one-outbox-one-sender-lock.md` - New ADR recording the one-outbox, one-sender-lock decision
- `CONTEXT.md` - Added Outbox, Pending event, Flush glossary entries; aligned Digest window wording
- `.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md` - Replaced original decisions (D-01..D-03) with the settled D-01..D-07, updated domain/canonical_refs/code_context/specifics/deferred sections and footer
- `.planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md` - Appended the post-grilling revision section (9 challenged items) after the original audit trail
- `.planning/ROADMAP.md` - Updated Phase 21 SC#2/SC#5 and planner notes, Phase 22 scheduling/staleness notes, and vocabulary in the Ordering rationale / Deploy sequencing prose

## Decisions Made

All ten decisions were pre-settled by the user's grilling session and transcribed verbatim per the plan — no new decisions were made during execution. See `key-decisions` in frontmatter for the seven implementation decisions (D-01..D-07); the three remaining requirement IDs (21-PG-01, 21-PG-04, 21-PG-10) cover the ADR record, the dbOpTimeout-bounded reads detail, and the vocabulary alignment respectively, all captured within D-01..D-07 and the glossary changes above.

## Deviations from Plan

None - plan executed exactly as written. All three automated verify commands passed on the first attempt with no rework.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

Phase 21 planning can now proceed from `21-CONTEXT.md`'s settled D-01..D-07 and ROADMAP.md's aligned success criteria. `docs/adr/0002-one-outbox-one-sender-lock.md` is referenced from both and is the canonical source for the shared-sender-lock design that Phase 22's digest send will also need to implement against.

---
*Phase: quick/260916-dvy*
*Completed: 2026-09-16*

## Self-Check: PASSED

All five plan-listed files and the SUMMARY.md exist on disk; all three task commits (295d797, 6bf1dd5, 478f5eb) are found in git history.
