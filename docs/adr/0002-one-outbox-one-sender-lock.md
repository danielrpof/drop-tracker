---
status: accepted
---

# One outbox, one sender lock

## Context

Digest mode (v1.5) gives the outbox a second sender. The real-time notify
pass and the Phase 22 digest send both select pending events from the one
outbox, and each acks an event only after Discord accepts it. The ack's
`AND notified_at IS NULL` predicate makes it idempotent, but that prevents
a double ack, not a double send: the Discord POST comes first and the
ack's rows-affected count is discarded. So two senders listing the same
pending rows at the same time both deliver them. Toggling digest mode
mid-pass sets up exactly that overlap in both directions. Toggled on, a
real-time pass is still in flight when a digest fires. Toggled off, a
flush starts while a digest send is still running.

## Decision

Every outbox send serializes on the existing `Notifier.notifying`
atomic.Bool CAS guard. In Phase 22 the digest send becomes a method on
`Notifier`, not an independent goroutine with its own guard. Whichever
send finds the lock held skips (CAS-skip), which is safe because the
outbox is persistent: skipped work is picked up by the next pass or tick.
Inside the real-time pass, digest mode is read after acquiring the lock
and before listing pending events, then read again before each Discord
send. If a re-read shows digest mode on, the pass stops and the remaining
events stay pending. Stale-event suppression acks aren't re-checked,
since they make no Discord request.

## Considered options

- **A separate CAS guard per sender**, which is what the roadmap's Phase
  22 note originally said. Rejected: each guard only stops its own sender
  overlapping itself, so a real-time pass and a digest send can still
  list and send the same pending rows at the same time, in either toggle
  direction.
- **A per-send mode re-read with no shared lock.** Rejected: it narrows
  the window but can't close it, because the check and the send aren't
  atomic across two senders.
- **A Postgres advisory lock.** Rejected as unnecessary for a single
  binary running one instance (the same constraint ADR 0001 relies on).
  It would add a DB round trip and a lock that can outlive a wedged
  connection.
- **A shared in-process lock on Notifier.** Chosen.

## Consequences

- Phase 22's shape is constrained. The digest send must live on Notifier
  and take the notifying lock.
- A long flush or real-time backlog makes a due digest tick skip rather
  than race it. The due-check converges on a later tick, because events
  are selected by outbox state.
- Residual: when an operator turns digest mode on mid-pass, at most the
  one message already in flight still goes out in real time.
- Single-instance only. A second instance would need a DB-level lock,
  which is the same limit as ADR 0001.
