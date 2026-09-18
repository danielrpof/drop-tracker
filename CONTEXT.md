# drop-tracker

A release tracker for hip-hop, reggaeton, and R&B: it polls MusicBrainz and Deezer for watched artists, detects new releases/guest features/deluxe changes, and delivers them to a Discord webhook.

## Language

**Outbox**:
Every event that has been detected but not yet delivered or acknowledged — the single place both real-time delivery and digests draw from.
_Avoid_: queue, digest queue

**Pending event**:
One event in the outbox.
_Avoid_: queued event, held event, unnotified event

**Digest mode**:
The instance-wide, Postgres-persisted setting that switches notification delivery from real-time (one Discord message per event, as it's detected) to batched. Off by default.
_Avoid_: batch mode, digest toggle

**Flush**:
The first real-time delivery pass after digest mode is turned off, delivering the events that accumulated while it was on, one message each.
_Avoid_: drain, catch-up send

**Digest**:
One batched delivery produced while digest mode is on — the message(s) sent for a single cadence period, covering every event in that digest's window.
_Avoid_: batch, digest run (reserve "run" for poll-cycle runs, a distinct existing concept)

**Cadence**:
How often digest mode fires: daily or weekly. An instance-wide setting, not a specific time-of-day (v1.5 fixes the fire hour; it isn't operator-configurable).
_Avoid_: frequency, schedule, interval

**Digest window**:
The set of events one digest covers. Defined by outbox state (every event still pending since the last successful digest), never by a wall-clock time range — a late, skipped, or duplicated scheduler tick still converges on the correct set, because nothing is selected by *when* the tick fired.
_Avoid_: batch window, time window

**Watermark**:
The persisted timestamp of the last successful digest send. It is what the operator sees as "last digest sent" and supplies the "since <watermark>" label shown on the digest. It does not decide whether a digest is due (that's the slot record) and never selects which events are included (that's the digest window).
_Avoid_: cursor, last-sent timestamp, checkpoint

**Digest slot**:
A scheduled instant at which a digest is due: 00:05 America/New_York every day (daily cadence) or every Friday (weekly cadence). Slots are calendar times in that zone, not fixed intervals, so they stay put across daylight-saving changes.
_Avoid_: tick (a tick is one due-check, not the slot it checks), run

**Slot record**:
The persisted most recent digest slot that has been handled — by sending a digest, or by finding nothing to send. A digest is due only when a newer slot has passed. Turning digest mode on or changing cadence moves it to the most recent slot, so neither triggers an immediate digest.
_Avoid_: watermark, last run, checkpoint

**Grace window**:
How long after a digest slot a missed digest may still be sent late: 12 hours for daily cadence, 48 hours for weekly. Past it, the slot is abandoned and its pending events wait for the next slot.
_Avoid_: catch-up window, retry window

**Watched artist**:
The watchlist artist an event was detected for. On a guest feature this differs from the **credited artist** (the release's primary artist, whose release the watched artist appears on).
_Avoid_: featured artist (ambiguous — say watched or credited)
