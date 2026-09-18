# Phase 22: Scheduled Digest Send - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-16
**Phase:** 22-scheduled-digest-send
**Areas discussed:** Digest message layout, Fire schedule (time + zone), Scheduler check interval

---

## Digest message layout

| Option | Description | Selected |
|--------|-------------|----------|
| By event type, then artist | Three fixed headings (New Releases / Guest Features / Deluxe Changes), artists listed under each | ✓ |
| By artist, then event type | One heading per watched artist with mixed events underneath | |
| Flat by artist, type inline | No headings, one line per event, alphabetical by artist, type shown via emoji prefix | |

**User's choice:** By event type, then artist.

| Option | Description | Selected |
|--------|-------------|----------|
| One embed, markdown headings in Description | Single embed, Description holds markdown headings + compact lines — most headroom before hitting Discord's character limit | ✓ |
| One embed per event type (up to 3 embeds, one message) | Three embeds in one message payload, each a heading + fields | |
| Field-per-event, one embed | Single embed, one field per event — caps at 25 fields, hits ceiling before Description's budget would | |

**User's choice:** One embed, markdown headings in Description.

| Option | Description | Selected |
|--------|-------------|----------|
| Alphabetical by artist | Deterministic, scannable regardless of detection order | ✓ |
| Chronological (detection order) | Matches `ListUnnotified`'s existing row order, no extra sort step | |

**User's choice:** Alphabetical by artist.

| Option | Description | Selected |
|--------|-------------|----------|
| Artist — Title, linked | Clickable line reusing existing link-building helpers | ✓ |
| Artist — Title, plain text | No link | |
| Artist — Title + deluxe track-count delta | Linked, plus inline track-count delta on deluxe lines | |

**User's choice:** Artist — Title, linked.
**Notes:** Follow-up raised: this choice drops the track-count delta (12→15) that real-time notifications show for Deluxe Change events today. Asked whether to accept the loss or restore it as a suffix.

| Option | Description | Selected |
|--------|-------------|----------|
| Drop it — link only | Deluxe lines look identical to other event types | |
| Add it back as a suffix | Deluxe lines only carry `(12→15 tracks)` after the link | ✓ |

**User's choice:** Add it back as a suffix — deluxe track-count delta is preserved.

---

## Fire schedule (time + zone)

| Option | Description | Selected |
|--------|-------------|----------|
| UTC | Zero DST ambiguity | (initially selected, then reversed) |
| America/New_York | Real DST-observing zone, but is the whole point of DGST-06 | |
| Let me specify another zone | Free text | |

**User's choice:** UTC, initially.
**Notes:** Flagged as a conflict: ROADMAP's own success criterion #4 requires proving the shipped Alpine image "resolves the zone it schedules against rather than silently falling back to UTC" — unobservable if the scheduled zone literally is UTC. DGST-06's DST-transition correctness also becomes vacuously true under UTC. Re-asked with this framing.

| Option | Description | Selected |
|--------|-------------|----------|
| Switch to America/New_York | Makes DGST-06/DGST-07 actually testable | ✓ |
| Keep UTC anyway | Accept both criteria become vacuous, documented explicitly | |

**User's choice:** Switch to America/New_York.

| Option | Description | Selected |
|--------|-------------|----------|
| 09:00 (morning) | Clear of the 2am DST window | |
| 18:00 (evening) | Clear of the 2am DST window | |
| Let me specify another hour | Free text | ✓ |

**User's choice:** Free text — 00:05.
**Notes:** Confirmed 00:05 is clear of both the spring-forward skipped hour (02:00–03:00) and the fall-back repeated hour (01:00–01:59).

| Option | Description | Selected |
|--------|-------------|----------|
| Sunday | Week-recap framing, new week starting | |
| Monday | Same framing, one day later | |
| Let me specify another day | Free text | ✓ |

**User's choice:** Free text — Friday.

---

## Scheduler check interval

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded constant | No operator-facing config for check cadence | ✓ |
| Env-configurable | New env var matching existing poll-tunable pattern | |

**User's choice:** Hardcoded constant.

| Option | Description | Selected |
|--------|-------------|----------|
| 5 minutes | ROADMAP's own suggestion | ✓ |
| 1 minute | Tighter catch-up, 12x the read volume | |

**User's choice:** 5 minutes.

| Option | Description | Selected |
|--------|-------------|----------|
| Immediately on start | Mirrors `authgate.sweepLoop`'s restart-safety intent | ✓ |
| Wait for the first tick | Pure `time.Ticker` semantics | |

**User's choice:** Immediately on start.

---

## Claude's Discretion

- Exact Go identifiers/file layout for the new scheduler goroutine and digest-send method.
- Shape of the new `internal/settings` watermark-write query/method.
- Exact `slog` field names/wording for new log lines.
- Whether the digest-send method reuses `NotifyPending`'s helpers directly or wraps them.

## Deferred Ideas

- The "since &lt;timestamp&gt;" window header and multi-message chunking — Phase 23.
- Operator-configurable fire time/timezone picker — out of scope for v1.5.
- Reviewed-but-not-folded todos (unrelated tooling): D-15 prev-release query files, `internal/sqlscan` quote-state-machine unification, `shadcn` dependency move — all weak keyword matches already dismissed identically in Phase 20's discussion.
