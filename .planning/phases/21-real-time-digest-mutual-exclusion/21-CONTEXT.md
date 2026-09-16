# Phase 21: Real-Time ↔ Digest Mutual Exclusion - Context

**Gathered:** 2026-09-16
**Status:** Ready for planning (revised 2026-09-16 after a grilling session; see D-01..D-07 and docs/adr/0002-one-outbox-one-sender-lock.md)

<domain>
## Phase Boundary

Turning digest mode on makes the real-time notifier stand down cleanly — events queue instead of firing — and turning it back off delivers everything that queued, nothing lost, nothing duplicated.

Requirements: DGST-13, DGST-14 (`.planning/REQUIREMENTS.md`).

**In scope:**
- A digest-mode read at the top of `Notifier.NotifyPending` (`internal/notifier/notifier.go:155`), under the existing `notifying` lock and before `listUnnotified`, re-read before each Discord send (D-04)
- `SettingsReader` as a required constructor argument of `notifier.Select`/`notifier.New`, wired from `settingsStore` at `cmd/server/main.go:299` (D-05)
- Fail closed on a settings-read error, every read bounded by `dbOpTimeout` (D-03)
- Staleness cutoff anchored to the event's `created_at` (D-02)
- Mode-transition logging (D-01)
- SPA helper text under the Digest mode row (D-06)

**Not in this phase:**
- `internal/poller` — the mode switch is invisible to it; it should not need to change at all
- The digest scheduler / actual grouped digest send — Phase 22, which will make the digest send a `Notifier` method sharing the `notifying` lock (ADR 0002)
- Grouping, window header, Discord chunking — Phase 23
- A second "digest queue" table or `digest_pending` flag — explicitly rejected; there is exactly one outbox
- Any schema change: `events.created_at` already exists (D-02)
- A Postgres advisory lock: single binary, so the in-process lock is enough (ADR 0002)
</domain>

<decisions>
## Implementation Decisions

- **D-01 Mode-transition logging.** Notifier tracks the last-observed digest mode in memory and logs one Info line only when the observed mode changes. The first successful read after boot counts as a transition, so logs show the mode after every restart. A failed read doesn't update the last-observed mode. The on-to-off line includes the pending count being flushed, taken from the list the pass already fetches (no new COUNT query, no sqlc change). A pass that stops mid-loop because digest mode turned on logs the count it left pending. This replaces the first discussion's per-pass standdown line, which would repeat every poll cycle for as long as digest mode stays on.
- **D-02 Staleness anchored to detection time.** `suppresses()` computes its cutoff as `ev.CreatedAt` minus `maxAgeDays` minus 1 day (UTC), instead of `time.Now()` minus `maxAgeDays`. `staleReleaseDate` and its shared detection/notifier table test don't change; only the cutoff input moves. The 1-day slack is there because `created_at` is the DB's `now()` at insert, which is later than detection's captured `now` and comes from a different clock. Without the slack, a release dated exactly on the cutoff day could pass detection and then be suppressed at delivery (a midnight straddle or clock skew). Why: time an event spends pending (digest mode, a flush after a long digest period, Phase 22's weekly cadence) must not age it out. With a 7-day window anchored to now, a weekly digest would routinely drop releases detected a couple of days after their release date. Pre-fix backlog rows are still suppressed, because their release dates are old relative to their own `created_at`. No schema change. The earlier claim that this was costly and needed schema work was wrong. Reversibility: reversible (one function's input).
- **D-03 Fail closed on a settings-read error.** This reverses the roadmap's 2026-09-11 lock. On a read error, whether at the top of the pass or before a send, stop the pass, log a distinct Warn, and return nil. Returning an error would add a second Error log at `poller.go:514` that reads like a delivery failure. No escalation on repeated failures. Skip the Warn when the error comes from ctx cancellation (shutdown). Every settings read is bounded by `dbOpTimeout`, using the same helper pattern as `listUnnotified`/`markNotified`. `settings.Service.Get` passes ctx straight through, and an unbounded read would bring back the notify-pass-hangs-forever bug while holding the lock. Why closed: in digest mode the pending set IS the next digest, so failing open would flush a whole digest backlog as individual messages on one transient error and ack them, emptying the digest. The outbox persists, so a skipped pass loses nothing, and the next poll (15m default) retries. The permanently failing read that failing open guarded against is near-impossible: the singleton row is seeded by a migration and `/ready` checks the schema.
- **D-04 One outbox, one sender lock** (docs/adr/0002-one-outbox-one-sender-lock.md). Real-time sends and Phase 22's digest send both serialize on the existing `Notifier.notifying` atomic.Bool, and in Phase 22 the digest send becomes a `Notifier` method. Whichever send finds the lock held CAS-skips, which is safe because the outbox is persistent. Read placement: read the mode after acquiring the lock and before `listUnnotified` (the pass's first decision), then read it again before each Discord send in the loop. If a re-read shows digest mode on, stop the pass and leave the remaining events pending. Don't re-read before stale-event suppression acks, which make no Discord request and so can't duplicate a message or violate the mode. Residual, accepted: at most the one message already in flight when the operator toggles on. Why: `MarkNotified`'s `AND notified_at IS NULL` prevents a double ack, not a double send. The POST happens first and the rows-affected count is discarded (`notifier.go:185`), so separate guards would let a real-time pass and a digest send list and send the same rows at the same time, in both toggle directions.
- **D-05 SettingsReader is a required constructor argument.** Declare it in `internal/notifier` as `Get(ctx context.Context) (settings.Settings, error)`. It returns the full `Settings` because Phase 22 needs cadence and watermark from the same read, and `settings` imports only `sqlc`, so there's no cycle. It's a required argument of `notifier.Select` and `notifier.New`, not a functional option, because a forgotten option would pass every notifier test and silently ship an ungated notifier. `cmd/server/main.go` passes the existing `settingsStore`.
- **D-06 SPA helper text.** Add always-visible helper text under the Digest mode row in `web/app/components/system/DigestSettings.tsx`: "While on, new events wait for the next digest; switching back off delivers them individually." The web Definition of Done applies (`prettier --write`, `corepack pnpm test`). Bundle note: `dist/` is gitignored, but `internal/webassets/build/client/` is a committed copy of the built SPA (refreshed via `make web`, as Phase 20-04 did; the Docker build regenerates it from source regardless). Whether Phase 21 refreshes it is the phase planner's call.
- **D-07 Deterministic tests.** Use a fake SettingsReader/Sender that flips mode (or returns an error) on the k-th call, and assert exact send and ack counts. Don't use looped timing invariants. Cover: digest-on standdown (zero sends, rows stay pending); the off flush through the ordinary path; a per-send re-read stopping mid-pass; fail-closed at top-of-pass and mid-pass; no Warn on ctx cancel; staleness anchored to `created_at`, including the 1-day slack boundary; transition logging (boot, and on-to-off with the count). Real-settings tests build the reader from `testutil.NewIsolatedTestPool`, never the shared pool, because `internal/httpserver/settings_test.go` flips the singleton row and packages run in parallel. CI does run `-race` (`make test-integration` runs `go test ./... -race` on ubuntu-latest; full-pipeline.yml:54, Makefile:76). Only this Windows dev box can't, and the toggle-mid-pass risk is a logical interleaving that `-race` wouldn't catch anyway.

### Claude's Discretion

- Exact log wording — follow the terse `slog` structured-field style already in `notifier.go` (`slog.Int`, `slog.String("error", ...)`, etc.).
- How the last-observed mode is stored on `Notifier`.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — DGST-13, DGST-14 are this phase's contract.
- `.planning/ROADMAP.md` → "Phase 21: Real-Time ↔ Digest Mutual Exclusion" → "Notes for the phase planner" — the primary brief: gate location, the single outbox, the fail-closed posture, the shared sender lock, the staleness anchor, and the testing note.
- `.planning/ROADMAP.md` → "Ordering rationale" note (above Phase Details) — why this phase (the gate) lands before Phase 22 (the sender): an un-gated real-time drain empties the outbox every poll cycle, making the digest sender unverifiable until this gate exists.
- `.planning/ROADMAP.md` → "Deploy sequencing" note (locked 2026-09-11) — Phase 21 is not merged to `main` until Phase 22 is also ready; a release-sequencing rule, not a plan/dependency change.
- `docs/adr/0002-one-outbox-one-sender-lock.md` (D-04) — the shared sender lock.
- Root `CONTEXT.md` (glossary: Outbox, Pending event, Flush, Digest window).

### Prior phase context (established patterns this phase follows)
- `.planning/phases/20-digest-settings-operator-control/20-CONTEXT.md` — the `internal/settings.Store` interface (`Get`/`Update`) this phase's `SettingsReader` seam reads from; D-05's no-cache single-row-read rationale is exactly what makes "re-read on every notify pass, no restart needed" (SC#3) true by construction.

### Existing safety guards this phase must preserve, not rework
- `internal/notifier/notifier.go` — the `notifying atomic.Bool` CAS-skip guard becomes the shared sender lock every outbox send serializes on (D-04); `markNotified`'s idempotent `AND notified_at IS NULL` ack prevents a double ack, not a double send; the WR-03 log pattern for "Discord sent but DB ack failed" is the model D-03's fail-closed Warn follows.
- `internal/poller/poller.go` (~line 514) — `NotifyPending`'s call site: a returned error is logged and swallowed, never turns a successful detection cycle into a failed one. This contract must not change.

### Testing pattern references
- `internal/testutil/postgres.go` → `NewIsolatedTestPool` — isolated schema for real-settings tests.
- `internal/httpserver/settings_test.go` — flips the singleton settings row; the shared-row hazard real-settings tests must avoid.
- CI's `-race` run (`full-pipeline.yml:54` / `Makefile:76`) — `make test-integration` runs `go test ./... -race` on ubuntu-latest.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" — the backend gates (`go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check`, local-only) plus the web gates (`prettier --write`, `corepack pnpm test`), since D-06 changes `DigestSettings.tsx`. See D-06 for the committed-bundle note.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/settings.Store` (`internal/settings/settings.go`) — already built in Phase 20: narrow `Get(ctx) (Settings, error)` / `Update(ctx, UpdateParams) (Settings, error)`. `Settings.DigestEnabled bool` is exactly what the gate needs to read; `Service.Get` passes ctx through unbounded (D-03 bounds it).
- `internal/notifier/notifier.go` — the whole file is the integration point. `Notifier.NotifyPending` (line ~155) is where the gate goes, first thing in the function body, before `listUnnotified` is called. `dbOpTimeout` plus the `listUnnotified`/`markNotified` helpers are the pattern for a bounded settings read.
- Existing consumer-declared-seam pattern in the same file (`Sender`, `Sink` interfaces) — the model for adding a `SettingsReader` interface declared in `notifier` itself, not imported from `settings`.
- `staleReleaseDate` is reused as-is (D-02).

### Established Patterns
- `WithMaxReleaseAgeDays` stays a functional option, but `SettingsReader` is deliberately a required constructor argument, not an option (D-05).
- `notifier.Select` in `cmd/server/main.go` (line 299) is the composition-root call site that will need the new dependency (`settingsStore` is already constructed at line 255 — it's just not yet passed into `notifier.Select`/`notifier.New`).
- Structured `slog` logging style already established in `notifier.go`: `logger.Info(...)`/`logger.Warn(...)` with `slog.Int64`, `slog.String("error", ...)` fields, one line per pass (not per row) for summary-style events (mirrors the `suppressed` count line D-01 is modeled on).

### Integration Points
- `internal/notifier/notifier.go` — `SettingsReader` interface; required arg on `New`/`Select`; last-observed-mode field; bounded mode read at the top plus a per-send re-read; the `suppresses` cutoff from `ev.CreatedAt`.
- `cmd/server/main.go:299` — pass `settingsStore`. Existing `New`/`Select` call sites in tests gain the argument.
- `web/app/components/system/DigestSettings.tsx` plus `DigestSettings.test.tsx` — the helper text.
- `notifier_test.go` — the D-07 test list.

</code_context>

<specifics>
## Specific Ideas

Pass order: CAS lock, then a bounded mode read (digest on or a read error means stop before `listUnnotified`), then list, then the loop. In the loop, a suppressed row is acked with no re-read. Otherwise re-read the mode (digest on or a read error means stop, the rest stay pending), then send, ack, and space. This is not a post-hoc filter on fetched rows (SC#5).

No new timestamp column, no `digest_pending` flag, no second queue table — staleness uses the existing `created_at` (D-02).

</specifics>

<deferred>
## Deferred Ideas

- Staleness exemption for pending events: superseded by D-02's `created_at` anchor, so there's nothing left to defer.
- The digest scheduler and actual grouped send (cadence fire times, missed-tick catch-up, DST/tzdata) — Phase 22 (a `Notifier` method on the shared lock, ADR 0002).
- Digest window header, per-type/per-artist grouping, Discord chunking — Phase 23.

</deferred>

---

*Phase: 21-real-time-digest-mutual-exclusion*
*Context gathered: 2026-09-16*
*Revised: 2026-09-16 (post-grilling; audit trail in 21-DISCUSSION-LOG.md)*
