---
status: accepted
---

# In-process ring buffer for poll-run history

## Context

v1.4 (Operator Observability) needs a record of what each poll cycle did —
per source, timings, work counters, an outcome — surfaced through a gated
`GET /status` endpoint and a SPA "System" view. The obvious implementation,
and the one the v1.4 requirements and research originally assumed, is a
`poll_runs` Postgres table written through a seam at the end of every cycle
and pruned to the last N rows per source.

## Decision

Store poll-run history in an **in-process ring buffer** — a mutex-guarded
slice of the last N `RunResult` values per source, held in
`internal/pollruns.Store` — not a database table. The `poller.RunRecorder`
seam and the `httpserver.StatusStore` seam both point at this one in-memory
store, wired at the composition root. History does not survive a process
restart.

## Considered options

- **`poll_runs` table + sqlc + prune-on-insert.** Rejected. It pulls a
  schema migration, an `n1-boot` surface, and the local-only `make
  sqlc-check` drift gate into the milestone, and — more importantly — it
  introduces two genuine concurrency hazards on a table written by two
  cron entries that fire on the same `@every` spec: a prune that deletes
  the other source's rows if its `WHERE` is mis-scoped, and a cross-source
  `DELETE` deadlock. A burst of overlap-skipped ticks during one slow
  cycle can also evict real history through the prune. None of these exist
  without a table.
- **Structured logs only.** Rejected. The per-cycle counters already go to
  the `cycle_id`-correlated logs, but logs roll and there is no queryable
  "last N cycles per source" surface for the `/status` contract the System
  view types against.
- **In-process ring buffer.** Chosen.

## Consequences

- No migration, no sqlc codegen, no prune SQL. The `poll_runs` prune-race
  and cross-source-deadlock pitfalls are designed out rather than guarded
  against. The recorder write is a mutex append that cannot hang on a
  stalled database, so the shutdown-survival `context.WithoutCancel`
  handling the table design needed collapses away.
- **History resets on restart** — which is also every deploy. Phase 19's
  System view must render an explicit first-run empty state regardless, so
  the post-deploy "no cycles yet" window is already handled.
- Single-instance only. `robfig/cron` has no leader election; a second
  instance would keep its own separate buffer. This matches the project's
  single-binary / single-instance constraint and the existing per-process
  overlap guard.
- Reversing this later (moving to a table) costs a migration plus
  accepting that the new table starts empty — there is no historical data
  to back-fill.
- v1.4 requirements RUN-01/RUN-03/RUN-04 were reworded from
  `poll_runs`-table language to storage-agnostic wording to match.
