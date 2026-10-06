---
phase: quick/261005-ezu
plan: 01
subsystem: planning-docs
tags: [ui-spec, phase-25, docs]
status: complete
requirements: [WLVW-03, WLVW-04, WLVW-06]
key-files:
  modified:
    - .planning/phases/25-find-filter-watchlist-and-history/25-UI-SPEC.md
    - .planning/REQUIREMENTS.md
actuals:
  tokens: 3000
  tasks: 2
  commits: 2
plan_head_before: c61be5075b83d5ad154207d542e54ccf18d4cf27
commits: 2
---

# Quick 261005-ezu: Apply Phase 25 UI-SPEC review fixes

Aligned 25-UI-SPEC with grill overrides G1-G7 and added router scroll-reset and sticky-state rules, so Phase 25 planning copies a contract that matches real router and refresh behavior.

## Commits

- `4e4acef` Task 1: banner and upstream contract name G1-G7 (G wins over D); Upcoming line covered in Scope, Typography, accent list, contrast row, date table, Data Contract; optional single-line layout recorded; WLVW-03 excludes upcoming dates.
- `2b448b5` Task 2: new "Router & sticky-state rules" section (shared `preventScrollReset` helper, explicit `stickyIds`, adds/Undo sticky, tag-id validation once); refresh/reload-pruning wording removed from three places; two W5 rows, tally 35/30/5/0; three UAT items.

## Deviations from Plan

None - plan executed as written. Both automated verify gates passed (TASK1_OK; Task 2 checks all green). Frontmatter `status: approved` and 25-CONTEXT.md unchanged; only `.planning/` files changed.

## Self-Check: PASSED

Commits 4e4acef and 2b448b5 exist; both edited files contain the required markers.
