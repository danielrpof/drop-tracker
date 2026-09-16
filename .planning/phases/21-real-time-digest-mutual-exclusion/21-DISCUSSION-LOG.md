# Phase 21: Real-Time ↔ Digest Mutual Exclusion - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-16
**Phase:** 21-real-time-digest-mutual-exclusion
**Areas discussed:** Standdown log visibility, Flush + staleness interaction, Fail-open failure visibility

---

## Standdown Log Visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Log a summary line per pass | Mirrors the existing 'suppressed' summary pattern in notifier.go: one Info line per pass, e.g. 'digest mode on: N events pending, standing down' — only when there's something pending. | ✓ |
| Silent — no log line | The gate is a deliberate, expected state, not an anomaly — the /system view already tells the operator. Adding a repeating log line every poll cycle (possibly for days) is noise. | |
| Log once on state transition only | Log when the gate first engages and once more when it disengages/flushes — not on every individual standing-down cycle. | |

**User's choice:** Log a summary line per pass (Recommended)
**Notes:** None — first option selected directly.

---

## Flush + Staleness Interaction

| Option | Description | Selected |
|--------|-------------|----------|
| Apply staleness suppression unchanged | The queued event flushes through the exact same real-time path, including the existing suppresses() check. Matches SC#2's literal wording ('through the ordinary real-time path'); needs zero new code. | ✓ |
| Exempt digest-queued events from staleness suppression | Track that an event queued because of the digest gate and skip the age check for those on flush. Requires new state distinguishing 'queued by the gate' from 'suppressed by staleness' — arguably a second queue in spirit. | |

**User's choice:** Apply staleness suppression unchanged (Recommended)
**Notes:** Locked as D-02 — no exemption, no new "queued at" clock; reaffirms the roadmap's single-outbox constraint.

---

## Fail-Open Failure Visibility

| Option | Description | Selected |
|--------|-------------|----------|
| Warn-level log line | Mirrors the notifier's existing WR-03 pattern (a distinct Warn line for a different silent-failure mode). A recurring settings-read failure is worth noticing even though the fallback is safe. | ✓ |
| Error-level log line | Settings reads should basically never fail on a healthy DB — treat a failure as a harder signal than WR-03's expected occasional race. | |
| Silent — no distinct log line | Fail-open to real-time is itself the safe, intended behavior — today's default. | |

**User's choice:** Warn-level log line (Recommended)
**Notes:** The fail-open behavior itself was already locked by the roadmap/grilling session (2026-09-11) — this discussion only settled the logging visibility around it.

---

## Claude's Discretion

- Exact `SettingsReader` interface shape (full `settings.Store` vs. narrower `Get`-only seam declared in `internal/notifier`).
- Exact wording of the standdown/fail-open log lines — follow the existing terse `slog` structured-field style.
- Composition-root wiring details in `cmd/server/main.go` (functional option vs. new constructor parameter).
- Concurrency proof technique — follow `TestRunCycle_CounterInvariant` / `TestStore_TwoSourceConcurrent`'s looped exact-equality invariant pattern (no `go test -race` available on this dev box or in CI).

## Deferred Ideas

- Digest-queue staleness exemption (a separate "queued while digest was on" clock so aged events are never silently suppressed) — explicitly rejected in favor of applying ordinary staleness suppression unchanged; would introduce new state and contradict the single-outbox design. Revisit only if ordinary suppression is found to actually lose meaningful events in practice.

---

## Post-grilling revision (2026-09-16)

A grilling session challenged the gathered context above, and the selections below supersede the earlier ones in 21-CONTEXT.md.

### Mode logging

| Option | Description | Selected |
|--------|-------------|----------|
| Per-pass standdown line | Log an Info summary line every pass while digest is on and events are pending, as originally selected above. | |
| Silent | No log line at all for the standdown state. | |
| Transition-only with the flush count | Log one Info line only when the observed mode changes; the on-to-off line includes the pending count being flushed. | ✓ |

**User's choice:** Transition-only with the flush count
**Notes:** A per-pass line repeats every cycle for as long as digest mode is on.

### Staleness during pending

| Option | Description | Selected |
|--------|-------------|----------|
| Now-anchored cutoff, unchanged | `suppresses()` keeps computing its cutoff from `time.Now()`, as originally selected above. | |
| A marker exempting pending events | Track that an event queued because of the gate and skip the age check for those on flush. | |
| Cutoff anchored to `created_at` with 1-day slack | `suppresses()` computes its cutoff from `ev.CreatedAt` minus `maxAgeDays` minus 1 day. | ✓ |

**User's choice:** Cutoff anchored to `created_at` with 1-day slack
**Notes:** Time pending must not age events out, and `created_at` already exists.

### Settings-read failure

| Option | Description | Selected |
|--------|-------------|----------|
| Fail open to real-time with a Warn | The original 2026-09-11 lock, reaffirmed above. | |
| Fail closed, returning an error | Stop the pass and return the error to the caller. | |
| Fail closed with a Warn and a nil return | Stop the pass, log a distinct Warn, return nil. | ✓ |

**User's choice:** Fail closed with a Warn and a nil return
**Notes:** The pending set is the next digest, and returning an error double-logs as a delivery failure.

### Settings-read bound

| Option | Description | Selected |
|--------|-------------|----------|
| Unbounded `Get` | No timeout on the settings read, as the original context assumed. | |
| Bounded by `dbOpTimeout` | Wrap the read in the same bounded-timeout helper pattern as `listUnnotified`/`markNotified`. | ✓ |

**User's choice:** Bounded by `dbOpTimeout`
**Notes:** An unbounded read can hang the pass while it holds the lock.

### Exclusion between real-time and digest sends

| Option | Description | Selected |
|--------|-------------|----------|
| A separate CAS guard per sender | The original Phase 22 roadmap note: each sender gets its own overlap guard. | |
| A per-send re-read alone | Re-read the mode before each send, with no shared lock. | |
| A Postgres advisory lock | A DB-level lock shared across senders. | |
| The shared `notifying` lock with the mode read under it plus a per-send re-read | Every outbox send serializes on `Notifier.notifying`. | ✓ |

**User's choice:** The shared `notifying` lock with the mode read under it plus a per-send re-read
**Notes:** An idempotent ack is not an idempotent send.

### SettingsReader wiring

| Option | Description | Selected |
|--------|-------------|----------|
| Functional option | `SettingsReader` wired via a `WithSettingsReader`-style option, mirroring `WithMaxReleaseAgeDays`. | |
| Required constructor argument | A Get-only seam returning the full `settings.Settings`, required on `New`/`Select`. | ✓ |

**User's choice:** Required constructor argument
**Notes:** A forgotten option ships an ungated notifier and still passes the tests.

### SPA copy

| Option | Description | Selected |
|--------|-------------|----------|
| No SPA change | The original backend-only assumption: no frontend changes expected in this phase. | |
| Helper text under the Digest mode row | Always-visible helper text: "While on, new events wait for the next digest; switching back off delivers them individually." | ✓ |

**User's choice:** Helper text under the Digest mode row
**Notes:** The exact wording is D-06.

### Concurrency test approach

| Option | Description | Selected |
|--------|-------------|----------|
| Looped exact-equality invariant | The original substitute for `-race`, as selected above. | |
| Deterministic fakes that flip on the k-th call | A fake `SettingsReader`/`Sender` that flips mode (or errors) on the k-th call, with exact send/ack counts asserted. | ✓ |

**User's choice:** Deterministic fakes that flip on the k-th call
**Notes:** The risk is a logical interleaving, and the premise that CI lacks `-race` was wrong.

### Vocabulary

| Option | Description | Selected |
|--------|-------------|----------|
| Keep the roadmap's SC#2 wording | "everything that queued during the digest window" and similar queue-based phrasing. | |
| "Accumulated" plus the glossary terms | "everything that accumulated while digest mode was on", plus the glossary terms Outbox, Pending event, Flush. | ✓ |

**User's choice:** "Accumulated" plus the glossary terms
**Notes:** The glossary's Digest window is an event set, not a time period.
