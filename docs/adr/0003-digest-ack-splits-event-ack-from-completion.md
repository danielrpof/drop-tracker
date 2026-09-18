---
status: accepted
---

# The digest ack splits event-ack from completion

## Context

Phase 22 acks a digest with one data-modifying CTE statement
(`AckDigestBatch`): it marks every sent id notified and writes both
`digest_last_slot_at` (always) and `digest_last_sent_at` (when something
was sent) atomically. Phase 22's own context note assumed Phase 23 could
"reuse the same statement per delivered chunk."

It cannot. The statement writes the slot unconditionally -- there is no
nil path for it, only for `sent_at`. Reusing it per chunk means chunk 1
advances the slot, so when chunk 2 of 3 fails the next due-check computes
the same slot, finds `digest_last_slot_at` already at or past it, logs
"digest not due", and returns. The undelivered remainder then waits for
the *next* slot: tomorrow on daily cadence, up to seven days on weekly.
That silently defeats Phase 23's requirement that a partially failed
digest retries inside the grace window.

The two writes serve different questions. Acking events answers "has this
event been delivered?" and is true the moment Discord accepts the chunk
carrying it. Writing the slot and watermark answers "did this digest
complete?" and is only true after the last chunk lands.

## Decision

Two queries. `AckEventsOnly` acks a chunk's event ids and touches no
settings column; it runs for every delivered chunk except the last.
`AckDigestBatch` keeps its existing shape and runs exactly once, on the
final chunk, carrying the suppressed ids as well -- it is the only write
in the phase that moves instance state. A single-chunk digest therefore
takes precisely Phase 22's existing path.

A run that stops early -- a failed send, the chunk cap, the whole-send
time budget, or shutdown -- never reaches the final ack, so the slot
stays put, the digest stays due, and the next due-check continues from
whatever is still in the outbox.

Both queries keep the `AND notified_at IS NULL` idempotence predicate and
run under `context.WithoutCancel` bounded by `dbOpTimeout`, so a shutdown
landing after Discord's 2xx still acks.

## Considered options

- **Make `slot` a `sqlc.narg` + `COALESCE` in `AckDigestBatch`**, so one
  statement covers both roles. Rejected: it edits a statement Phase 22's
  tests pin, and it hides "this write completes the digest" inside a
  COALESCE rather than showing it at the call site.
- **Pass the previous slot value back on intermediate chunks.** Rejected:
  on a first-ever digest that value is NULL, and writing NULL resets the
  slot record, discarding the handled-slot history that stops an
  off-schedule digest firing.
- **Accept that a partial failure defers the remainder to the next slot.**
  Rejected: on weekly cadence that is a week-long hole, and it makes the
  grace window -- which exists precisely to recover missed sends --
  unreachable for the most likely failure mode.
- **Two queries.** Chosen.

## Consequences

- Phase 23 touches `queries/notification_settings.sql` and the generated
  sqlc output, despite being framed as a message-composition phase.
  `make sqlc-check` has no CI counterpart, so a forgotten regenerate
  ships a drifted tree green.
- "Delivered" and "digest completed" become separately observable, which
  is what lets a capped or time-budgeted run drain across several ticks
  without losing or re-sending anything.
- Residual: a cancel or timeout landing after Discord's 2xx but before
  that chunk's ack re-sends the chunk's content in a later digest. This
  is Phase 21's existing WR-03 posture, now with roughly one exposure
  window per chunk instead of one per digest.
