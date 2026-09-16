# Phase 21: Real-Time ↔ Digest Mutual Exclusion - Context

**Gathered:** 2026-09-16
**Status:** Ready for planning

<domain>
## Phase Boundary

Turning digest mode on makes the real-time notifier stand down cleanly — events queue instead of firing — and turning it back off delivers everything that queued, nothing lost, nothing duplicated.

Requirements: DGST-13, DGST-14 (`.planning/REQUIREMENTS.md`).

**In scope:**
- One gate at the top of `Notifier.NotifyPending` (`internal/notifier/notifier.go:155`) reading a `SettingsReader` seam
- Composition-root wiring at `cmd/server/main.go` to supply the real `settings.Store` (already built by Phase 20) to the notifier
- Standdown observability: a per-pass log line when digest is on and events are pending
- Fail-open behavior on a settings-read error, with a distinct log line for operator visibility

**Not in this phase:**
- `internal/poller` — the mode switch is invisible to it; it should not need to change at all
- The digest scheduler / actual grouped digest send — Phase 22
- Grouping, window header, Discord chunking — Phase 23
- A second "digest queue" table or `digest_pending` flag — explicitly rejected; there is exactly one outbox
</domain>

<decisions>
## Implementation Decisions

### Standdown Log Visibility

- **D-01: Log one Info summary line per notify pass while digest is on and events are pending** — e.g. `"digest mode on: N events pending, standing down"`. Mirrors the existing `suppressed`-count summary pattern already in `notifier.go` (log only when there's something to report, not on every empty-queue pass). Gives operators the same poll-cycle-level visibility Phase 18/19 built the System view around, rather than a silent gate that's invisible outside the `/system` toggle state.

### Flush + Staleness Interaction

- **D-02: The existing `suppresses()` staleness check applies unchanged to flushed events — no exemption for digest-queued events.** An event that queued while digest was on and aged past `maxReleaseAgeDays` by the time digest toggles off is silently acked (no Discord message) exactly as it would be for any other event hitting that check today. This is a deliberate reading of DGST-14/SC#2's "through the ordinary real-time path" — the ordinary path already includes staleness suppression, so nothing new is being introduced by the gate. Do NOT add a second notion of "queued at" timestamp or age exemption; that would be schema/state creep on top of the explicitly-rejected second-queue idea. — **Reversibility:** costly — introducing an exemption later means threading a "queued while digest was on" marker through the outbox schema and rewriting `suppresses()`'s single-clock model; not a local change.

### Fail-Open Failure Visibility

- **D-03: A `SettingsReader` read error inside the gate fails open to real-time (already locked by the roadmap/grilling session) AND logs a distinct Warn-level line**, mirroring the notifier's existing WR-03 pattern (a named, identifiable log signature for a specific failure mode, not a generic error). This is a deliberate branch, not an implicit `false` zero value on error — the settings read failing must never wedge or error out the poll cycle itself (`NotifyPending`'s error contract at the `poller.go` call site is log-and-continue; preserve it).

### Claude's Discretion

- **Exact `SettingsReader` interface shape** — whether it's the full `settings.Store` interface or a narrower `Get`-only seam declared in `internal/notifier` (consumer-declared-seam convention). Follows the existing pattern (`Sender`, `Sink` are both declared in `notifier.go` itself); not re-litigated here.
- **Exact wording of the standdown/fail-open log lines** — small, not worth locking string-for-string; follow the existing terse `slog` structured-field style already in `notifier.go` (`slog.Int`, `slog.String("error", ...)`, etc.).
- **Composition-root wiring details in `cmd/server/main.go`** (e.g. whether `notifier.Select`/`notifier.New` takes a new parameter or a new `Option`) — mirrors the existing `WithMaxReleaseAgeDays` functional-option pattern; planner's call.
- **Concurrency proof technique** — `go test -race` is unavailable on this dev box and absent from CI (WINDOWS.md). Follow the established substitute already used at `TestRunCycle_CounterInvariant` (`internal/poller/poller_test.go`) and `TestStore_TwoSourceConcurrent` (`internal/pollruns/pollruns_test.go`): a looped exact-equality invariant test standing in for `-race` coverage of the toggle-mid-pass scenario.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — DGST-13, DGST-14 are this phase's contract.
- `.planning/ROADMAP.md` → "Phase 21: Real-Time ↔ Digest Mutual Exclusion" → "Notes for the phase planner" — the primary brief: gate location (`internal/notifier/notifier.go:155`), single-outbox constraint, locked fail-open posture, existing guards to preserve, testing substitute for `-race`.
- `.planning/ROADMAP.md` → "Ordering rationale" note (above Phase Details) — why this phase (the gate) lands before Phase 22 (the sender): an un-gated real-time drain empties the outbox every poll cycle, making the digest sender unverifiable until this gate exists.
- `.planning/ROADMAP.md` → "Deploy sequencing" note (locked 2026-09-11) — Phase 21 is not merged to `main` until Phase 22 is also ready; a release-sequencing rule, not a plan/dependency change.

### Prior phase context (established patterns this phase follows)
- `.planning/phases/20-digest-settings-operator-control/20-CONTEXT.md` — the `internal/settings.Store` interface (`Get`/`Update`) this phase's `SettingsReader` seam reads from; D-05's no-cache single-row-read rationale is exactly what makes "re-read on every notify pass, no restart needed" (SC#3) true by construction.

### Existing safety guards this phase must preserve, not rework
- `internal/notifier/notifier.go` — `notifying atomic.Bool` CAS-skip guard (D-06, cross-source overlap guard); `markNotified`'s idempotent `AND notified_at IS NULL` semantics (D-09); the WR-03 log pattern for "Discord sent but DB ack failed" (the model D-03 above follows for the fail-open case).
- `internal/poller/poller.go` (~line 514) — `NotifyPending`'s call site: a returned error is logged and swallowed, never turns a successful detection cycle into a failed one. This contract must not change.

### Testing pattern references
- `internal/poller/poller_test.go` → `TestRunCycle_CounterInvariant` — looped exact-equality invariant pattern.
- `internal/pollruns/pollruns_test.go` → `TestStore_TwoSourceConcurrent` — same pattern, applied to a two-source concurrent store.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" — `go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check` (local-only, no CI counterpart). No frontend changes expected in this phase (backend-only gate), so the prettier/pnpm steps likely don't apply — confirm during planning if any `/system` copy changes are needed for the standdown state.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/settings.Store` (`internal/settings/settings.go`) — already built in Phase 20: narrow `Get(ctx) (Settings, error)` / `Update(ctx, UpdateParams) (Settings, error)`. `Settings.DigestEnabled bool` is exactly what the gate needs to read.
- `internal/notifier/notifier.go` — the whole file is the integration point. `Notifier.NotifyPending` (line ~155) is where the gate goes, first thing in the function body, before `listUnnotified` is called.
- Existing consumer-declared-seam pattern in the same file (`Sender`, `Sink` interfaces) — the model for adding a `SettingsReader` interface declared in `notifier` itself, not imported from `settings`.

### Established Patterns
- Functional options (`Option func(*Notifier)`, `WithMaxReleaseAgeDays`) — the likely shape for wiring a `SettingsReader` into `Notifier` without breaking `New`'s existing signature, mirroring how `maxAgeDays` was added.
- `notifier.Select` in `cmd/server/main.go` (line 299) is the composition-root call site that will need the new dependency (`settingsStore` is already constructed at line 255 — it's just not yet passed into `notifier.Select`/`notifier.New`).
- Structured `slog` logging style already established in `notifier.go`: `logger.Info(...)`/`logger.Warn(...)` with `slog.Int64`, `slog.String("error", ...)` fields, one line per pass (not per row) for summary-style events (mirrors the `suppressed` count line D-01 is modeled on).

### Integration Points
- `internal/notifier/notifier.go` — new `SettingsReader` interface, gate logic at top of `NotifyPending`, new field on `Notifier` struct, new functional option.
- `cmd/server/main.go` (~line 299) — pass `settingsStore` into the notifier construction call.
- `internal/notifier/notifier_test.go` — new test coverage for: digest-on standdown (zero sends, events stay pending), digest-off flush (queued events send through ordinary path including staleness suppression), fail-open on a settings-read error, toggle-mid-pass invariant test.

</code_context>

<specifics>
## Specific Ideas

- The gate must be the **first** decision in `NotifyPending`, before `listUnnotified` is even called — not a post-hoc filter on already-fetched rows (SC#5). When digest is on, the function should log (per D-01, only if there's a nonzero pending count worth mentioning — the count itself needs a query or can be inferred from the pre-gate state) and return `nil` without touching the outbox at all.
- No new "queued at" timestamp, no `digest_pending` column, no second queue table (already explicitly rejected in the roadmap notes — reaffirmed by D-02 above).

</specifics>

<deferred>
## Deferred Ideas

- Digest-queue staleness exemption (tracking a separate "queued while digest was on" clock so aged events are never silently suppressed) — considered and explicitly rejected in D-02; would require new state and contradicts the single-outbox design. Revisit only if a future phase finds "ordinary staleness suppression" is actually losing meaningful events in practice.
- The digest scheduler and actual grouped send (cadence fire times, missed-tick catch-up, DST/tzdata) — Phase 22.
- Digest window header, per-type/per-artist grouping, Discord chunking — Phase 23.

</deferred>

---

*Phase: 21-real-time-digest-mutual-exclusion*
*Context gathered: 2026-09-16*
