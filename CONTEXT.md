# drop-tracker

A release tracker for hip-hop, reggaeton, and R&B: it polls MusicBrainz and Deezer for watched artists, detects new releases/guest features/deluxe changes, and delivers them to a Discord webhook.

## Language

**Digest mode**:
The instance-wide, Postgres-persisted setting that switches notification delivery from real-time (one Discord message per event, as it's detected) to batched. Off by default.
_Avoid_: batch mode, digest toggle

**Digest**:
One batched delivery produced while digest mode is on — the message(s) sent for a single cadence period, covering every event in that digest's window.
_Avoid_: batch, digest run (reserve "run" for poll-cycle runs, a distinct existing concept)

**Cadence**:
How often digest mode fires: daily or weekly. An instance-wide setting, not a specific time-of-day (v1.5 fixes the fire hour; it isn't operator-configurable).
_Avoid_: frequency, schedule, interval

**Digest window**:
The set of events one digest covers. Defined by outbox state (every event still unnotified since the last successful digest), never by a wall-clock time range — a late, skipped, or duplicated scheduler tick still converges on the correct set, because nothing is selected by *when* the tick fired.
_Avoid_: batch window, time window

**Watermark**:
The persisted timestamp of the last successful digest send (`digest_last_sent_at`). It decides whether a new digest is due and supplies the "since <watermark>" label shown on the digest — it never selects which events are included (that's the digest window's job, via outbox state).
_Avoid_: cursor, last-sent timestamp, checkpoint
