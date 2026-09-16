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
