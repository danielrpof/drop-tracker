# Phase 27: Paste-a-List Bulk Add - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-11-18
**Phase:** 27-paste-a-list-bulk-add
**Areas discussed:** Confidence rules, Art & Deezer link at confirm, Paste entry & limits, Review screen behavior

---

## Confidence rules

| Question | Options | Selected |
|----------|---------|----------|
| High tier | Unique exact match / Top result exact / Exact + MB score | Unique exact match |
| Deezer role | Ignore Deezer / Corroborate only / MB-only search param | Ignore Deezer |
| Medium tier | Any MB result = Medium / Fuzzy threshold | Any MB result = Medium |
| Search error | Distinct Error + retry / Fold into Not found | Distinct Error + retry |

## Art & Deezer link at confirm

| Question | Options | Selected |
|----------|---------|----------|
| Art match | Insert now, match in background / Skip, wait for startup backfill / Match inline, cap batch | Insert now, match in background |
| Deezer hint | No, server match only / Yes, as a hint | No, server match only |
| Pending art | Placeholder, no indicator / Toast hint only / Live refresh | Placeholder, no indicator |
| Match fail | Best-effort, backfill catches it / Durable retry | Best-effort |

## Paste entry & limits

| Question | Options | Selected |
|----------|---------|----------|
| Entry point | Dialog from Watchlist / Dedicated route / Inline panel | Dialog from Watchlist |
| Max lines | 100 / 50 / 200 | 100 |
| Close mid-flow | Confirm discard, then abort / Keep state until page leave / Close silently | Confirm discard, then abort |
| Cancel | Stop, review what's done / Stop and back to textarea | Stop, review what's done |

## Review screen behavior

**Notes:** The user paused the first round of questions to ask that ui-ux-pro-max be used for the UI design. The questions were then rebuilt on its guidance: no wide tables at phone width, no confidence shown by color alone, shadcn `Select`, and atomic status announcements.

| Question | Options | Selected |
|----------|---------|----------|
| Row layout | Stacked row card / Table on desktop, cards on phone | Stacked row card |
| Alternate | Select of fetched MB results / Select + re-search | Select of fetched MB results |
| Same artist | Flag, first one wins / Merge into one row | Flag, first one wins |
| Watched | Badge, unselectable / Hide them | Badge, unselectable |
| Organize | Group by status / Paste order + filter / Paste order only | Group by status |
| Result | Summary in dialog / Close + toast | Summary in dialog |

## Claude's Discretion

- Search concurrency, confirm endpoint path and shape, default preferences for new entries, copy, and visual tokens.

## Deferred Ideas

- Re-searching an edited name inside review. Applying tags at confirm.
