---
phase: 22-scheduled-digest-send
reviewed: 2026-09-16T00:00:00Z
depth: standard
files_reviewed: 26
files_reviewed_list:
  - .github/workflows/full-pipeline.yml
  - CONTEXT.md
  - cmd/server/main.go
  - go.mod
  - internal/db/migrate_test.go
  - internal/db/migrations/000009_digest_last_slot_at.down.sql
  - internal/db/migrations/000009_digest_last_slot_at.up.sql
  - internal/db/schema_version_test.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/notification_settings.sql.go
  - internal/db/sqlc/querier.go
  - internal/httpserver/settings_test.go
  - internal/notifier/digest.go
  - internal/notifier/digest_format.go
  - internal/notifier/digest_format_test.go
  - internal/notifier/digest_schedule_test.go
  - internal/notifier/digest_test.go
  - internal/notifier/format.go
  - internal/notifier/notifier.go
  - internal/notifier/notifier_test.go
  - internal/notifier/scheduler.go
  - internal/notifier/scheduler_test.go
  - internal/settings/settings.go
  - internal/settings/settings_test.go
  - internal/settings/slot.go
  - internal/settings/slot_test.go
  - queries/notification_settings.sql
findings:
  critical: 1
  warning: 3
  info: 0
  total: 4
status: issues_found
---

# Phase 22: Code Review Report

**Reviewed:** 2026-09-16
**Depth:** standard
**Files Reviewed:** 26
**Status:** issues_found

## Summary

This phase adds calendar/DST-correct digest slot math (`internal/settings/slot.go`), the fixed digest-send sequence (`internal/notifier/digest.go`), single-embed digest rendering with markdown-escaping (`internal/notifier/digest_format.go`), the `DigestScheduler` lifecycle (`internal/notifier/scheduler.go`), the `digest_last_slot_at` migration/query layer, and the CI zone-resolution boot proof. The DST/grace-window/slot-record logic is unusually well tested (four-transition sweeps for both cadences, inclusive/exclusive grace boundaries, mid-pass mode-flip races) and I did not find a correctness bug in that core math.

The findings below are: one genuine, currently-unmitigated risk in the digest-send path that the phase's own design docs (`22-CONTEXT.md` D-19, `ROADMAP.md`) acknowledge but do not enforce anywhere in code or CI; and three narrower defects in the scheduler wiring, logging, and embed-building code introduced by this phase.

## Critical Issues

### CR-01: Oversized digest Description has no interim safety valve, and nothing in this phase or CI stops it shipping alone

**File:** `internal/notifier/digest_format.go:51-56`, `internal/notifier/digest.go:100-120`
**Issue:**
`buildDigestEmbed` caps only each line's *title* at 100 runes (`digestTitleLimit`); the watched-artist name, the guest-feature host credit, and the per-line URL are all uncapped, and — more importantly — there is no cap anywhere on the *whole* `Embed.Description` string. Discord rejects a `description` over 4096 characters outright (a non-204 response), which `discord.Client.sendAttempt` turns into `discord: send webhook: unexpected status 400`. In `SendDigestIfDue`, that error is logged (`"digest send failed"`) and the function returns `nil` **without acking anything** (`internal/notifier/digest.go:114-120`) — every event in that batch stays pending.

Because nothing was acked, `digest_last_slot_at` is not advanced, so the *same* oversized (or larger, since new events keep accumulating) batch is retried on every 5-minute due-check for the rest of the grace window, then silently carried forward to the next slot once grace expires (`internal/notifier/digest.go:54-65`) — where it is, at best, the same size or larger again. For a watchlist that generates enough events per cadence period (weekly cadence + a reasonably active watchlist gets there well under the ~30–35 line budget the phase's own comments cite), this is a self-sustaining failure loop: the digest can never be delivered again until an operator manually disables digest mode (which flushes the backlog as individual real-time messages) or intervenes in the database.

This is a *known and explicitly documented* risk: `22-CONTEXT.md` D-19 and `ROADMAP.md`'s "Deploy sequencing" note state Phases 21/22/23 must ship together, with Phase 23 providing the chunking that closes this gap, and the phase-22 threat register accepts it (T-22-15, "accept"). The problem is that this acceptance is a *process* commitment only — nothing in this codebase or in `.github/workflows/full-pipeline.yml` enforces it. `full-pipeline.yml`'s `release` job auto-tags and pushes a new `ghcr.io` image and git tag on every green push to `main` (`svu next` + `docker push` + `git tag`, unconditional on any manual gate). If this phase's branch is merged to `main` by itself — which is exactly what a phase-scoped `/gsd-code-review` + merge workflow does — the very next push auto-releases a build carrying this defect live, with no code-level backstop.
**Fix:** Either (a) do not merge/release this phase to `main` until Phase 23's chunking lands in the same release window (a process control, not a code fix, and currently unenforced anywhere machine-checkable), or (b) add a cheap interim guard in this phase so an oversized digest degrades safely instead of looping forever, e.g.:
```go
// buildDigestEmbed, after composing b: if the Description would exceed
// Discord's 4096-char ceiling, truncate at the last full line and note the
// remainder instead of building an oversized payload. digest.go's caller
// can then log the truncation instead of retrying a doomed send forever.
const discordDescriptionLimit = 4096
if b.Len() > discordDescriptionLimit {
    // truncate to the last full "\n" boundary at/under the limit, append
    // "\n… N more events" so nothing is silently invisible, and — critically —
    // still ack every included AND excluded id via the existing ackDigestBatch
    // call so a persistently oversized backlog cannot loop forever.
}
```
At minimum, add a CI or release-process check that fails if this phase's commit range reaches `main` without Phase 23's chunking commits present, so D-19's sequencing constraint has a machine-enforced backstop instead of relying on manual discipline.

## Warnings

### WR-01: `digestSched.Start` runs before the fallible `poller.New` call, so an early failure never calls `Stop`

**File:** `cmd/server/main.go:337-350`
**Issue:** `digestSched.Start(ctx)` is called at line 338, but `poller.New(...)` (line 346) can still fail and `return fmt.Errorf("build poller: %w", err)` at line 348 — before the `defer` that calls `digestSched.Stop(...)` is ever registered (that defer is added at line 379, after `pollr.Start(ctx)` at line 350, which never executes on this path). On this path, `run()` returns immediately; `defer pool.Close()` (registered much earlier, line 167) fires as the function unwinds, while the digest scheduler's background goroutine (launched by `Start`, already running its immediate first `SendDigestIfDue` check per D-10) is still alive and not drained. It will eventually exit once `main`'s `stop()` cancels the parent context, but there is a real window where that goroutine can be mid-DB-call against a pool that is concurrently closing — exactly the race class this file's own extensive comments (e.g. lines 351-364, 366-385) go out of their way to prevent for the poller and backfill goroutines.
**Fix:** Move `digestSched.Start(ctx)` to after `poller.New` succeeds (immediately before `pollr.Start(ctx)`), or register a `defer digestSched.Stop(...)` immediately after `Start` is called rather than after the poller's own `Start`/defer block, so every return path — including `poller.New`'s error path — drains the scheduler before `pool.Close()` runs.

### WR-02: Grace-expired Warn re-logs on every 5-minute tick for the rest of the missed period, with no dedup

**File:** `internal/notifier/digest.go:54-65`
**Issue:** Once a slot's grace window expires without a successful ack, `digest_last_slot_at` is never advanced, so `MostRecentSlot(now, cadence, loc)` keeps returning the same past slot on every subsequent due-check until the *next* real slot instant arrives. Each of those checks re-evaluates `age > grace` as true and logs `"digest slot past its grace window: not sending late"` at Warn — unconditionally, every 5 minutes. For a weekly cadence during a multi-day Discord outage (or during CR-01's failure loop, which never recovers on its own), this can produce well over a thousand duplicate Warn lines before the next slot arrives. `Notifier` already has a proven pattern for exactly this kind of "log once per state, not once per tick" concern (`observeDigestMode`'s `lastDigestMode`/`lastDigestModeSet` fields in `internal/notifier/notifier.go:99-107,235-246`), but `SendDigestIfDue` does not reuse or mirror it for the grace-expired transition.
**Fix:** Track the last slot instant a grace-expired Warn was already logged for (a single `time.Time` field on `Notifier`, set-once-per-slot like `lastDigestMode`), and only log again when the slot value changes — mirroring `observeDigestMode`'s existing shape.

### WR-03: `buildDigestEmbed` silently drops (but still acks as delivered) any event whose type isn't one of the three known headings

**File:** `internal/notifier/digest_format.go:81-127`, `internal/notifier/digest.go:76-100`
**Issue:** `digest.go`'s `sendable` slice is built from every non-suppressed row `listUnnotified` returns, regardless of `EventType`, and every id in `sendable` is acked as sent once `n.sender.Send` succeeds (`internal/notifier/digest.go:122-131`). But `buildDigestEmbed` only ever renders the three headings enumerated in `digestHeadings` (`new_release`, `guest_feature`, `deluxe_change`) — any event whose `EventType` doesn't match one of those three is added to the `grouped` map but then never visited by the `for _, h := range digestHeadings` loop, so it never appears anywhere in the rendered `Description`. The net effect for such a row: it is marked `notified_at` as if delivered, but the operator never sees it in the digest — a silent drop, not a fail-safe (contrast `format.go`'s `formatEmbed` default branch, which still renders *something* rather than nothing, for the same "unrecognized type" case on the real-time path).

This is unreachable today because the `events_event_type_valid` DB CHECK constraint only permits those three values, so it is not exploitable now — but it is a latent trap for whoever adds a fourth event type in a future phase and forgets to add a corresponding entry to `digestHeadings`: the real-time path (`formatEmbed`) would visibly degrade to a bare-title embed, while the digest path would just quietly eat the row.
**Fix:** Either assert (via a bounds/consistency check, or a comment cross-referencing `formatEmbed`'s default-branch precedent) that this must be updated in lockstep with `eventTypeNewRelease`/`eventTypeGuestFeature`/`eventTypeDeluxeChange`, or have `buildDigestEmbed` render an "Other" fallback group (mirroring `formatEmbed`'s degrade-rather-than-drop posture) for any `EventType` not covered by `digestHeadings`.

---

_Reviewed: 2026-09-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
