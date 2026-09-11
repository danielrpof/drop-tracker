# Project Research Summary

**Project:** drop-tracker v1.5 (Digest Notifications)
**Domain:** Single-operator release-tracker service, extending real-time Discord notifications with optional digest/batch delivery mode
**Researched:** 2026-09-11
**Confidence:** MEDIUM–HIGH (architecture grounded in existing codebase patterns; general digest/cron/Discord findings cross-checked across multiple sources)

## Executive Summary

The digest-notification feature for v1.5 is **fundamentally an extension of existing patterns**, not a new architecture. The project's real-time notifier already uses an outbox pattern (`events.notified_at IS NULL` as a queue) that digest mode can reuse unchanged — the feature is purely "change who drains the outbox (the poll cycle or a new scheduled job) and how many Discord embeds go in one message." This tight coupling to existing seams (`Notifier`, `discord.Client`, the `events` table) makes the implementation low-risk: no new third-party dependencies, no new schema concepts beyond one singleton-row settings table, and zero changes to the poller or existing real-time path unless they opt into the mode-check guard.

The main technical risk is **correctness under restart and mode-toggle scenarios**. The research identified seven concrete pitfalls: the most critical are ensuring the real-time drain is fully gated (not just overlapped by) digest mode, handling DST/timezone edge cases in scheduling, and ensuring digest chunking respects Discord's embed/message limits. All are preventable with thoughtful design, and most are already solved by existing project patterns (the outbox idempotency, the `sync.Mutex`-guarded store pattern from `pollruns`).

**Recommendation:** Implement digest mode as a time.Ticker-driven goroutine (mirroring `authgate.Manager.sweepLoop`, not a third `robfig/cron` entry), reading a fresh singleton `notification_settings` row on every check to support no-restart mode toggling. Reuse the existing `events` table's `notified_at IS NULL` outbox unchanged. Guard `NotifyPending` with a single mode-check seam so exactly one consumer (real-time or digest) drains per poll cycle. Phase structure is additive: migration → Go packages → HTTP routes → SPA → integration wiring.

---

## Key Findings

### Recommended Stack

**No new third-party dependencies.** Every technology needed already exists in `go.mod`: `robfig/cron` (reused for non-digest polls, not for digest scheduling), `sqlc` + `pgx/v5` (new queries for settings, same codegen pipeline), `golang-migrate` (one additive migration), and the hand-rolled `internal/discord` and `internal/notifier` (extended with batch sending).

**New in-repo components:**

- **`internal/settings` package** (~80 lines): wraps sqlc-generated queries for a singleton `notification_settings` row. Mirrors the existing `internal/pollruns.Store` shape — a mutex-guarded struct exposing narrow `Get`/`Update` seams. No cache invalidation needed; a single-row PK-indexed read is cheap and correct for this project's scale.

- **`notification_settings` table** (additive migration): singleton row with `digest_enabled BOOLEAN DEFAULT false`, `digest_cadence TEXT CHECK IN ('daily','weekly')`, `digest_last_sent_at TIMESTAMPTZ`, `updated_at TIMESTAMPTZ`. Follows the project's existing convention of CHECK constraints over free-form key-value tables.

- **Digest formatter in `internal/notifier`** (new methods, existing package): reuses `formatEmbed`, `truncateRunes`, and private helpers unchanged. New `SendDigest` method batches formatted embeds into ≤10-embed Discord messages and respects the existing 400ms inter-send spacing.

- **`internal/discord.Client.SendBatch`** (signature extension): generalizes the existing single-embed `sendAttempt` path to accept `[]Embed`, chunking to Discord's 10-embed cap.

### Expected Features

**Must have (P1, for MVP v1.5):**
- Instance-wide digest toggle (on/off) — SPA-configurable, Postgres-persisted, takes effect without redeploy
- Fixed wall-clock cadence: daily or weekly (not rolling windows, not minute-granularity)
- Skip-send on zero events (universal convention across every digest-system reference found)
- All three event types (new release, guest feature, deluxe change) batched into one message
- Real-time vs. digest mutual exclusion (prevents double-post)
- `last_digest_sent_at` watermark (the load-bearing state for window definition and toggle correctness)

**Should have (P2, fast-follow once P1 lands):**
- Group-by-artist or group-by-event-type within digest body — pure presentation-layer change, lowers scanning effort
- "Since [timestamp]" header/footer in digest message — builds operator trust, directly solves mode-switch ambiguity
- Discord multi-message chunking for large digests — only matters at scale, but must degrade gracefully

**Defer to v2+ (P3, per milestone scope):**
- Per-event-type digest overrides (single-operator instance doesn't need preference-center complexity)
- Multi-channel digests (email, RSS, webhook) — orthogonal to batching logic, separate milestone
- Rolling/event-driven windows — not documented at day/week granularity

### Architecture Approach

Digest mode is **invisible to the poller** — it lives entirely inside the notifier as a mode branch on an existing seam. Two-path architecture:

1. **Real-time mode** (default, unchanged): `poller.runCycle` → `notifier.NotifyPending` → `SettingsReader` gate → if digest off, drain `notified_at IS NULL` events, send each as 1-embed Discord message.

2. **Digest mode** (new, opt-in): same poll-cycle call becomes no-op (events accumulate); separate `internal/digest` goroutine (time.Ticker, mirrors `authgate.sweepLoop`) ticks ~5 minutes, reads fresh settings, calls `Notifier.SendDigest` to batch-drain into ≤10-embed chunked Discord messages.

**Key integration points:**
- `events` table: outbox unchanged (`notified_at IS NULL`)
- `notification_settings` table: singleton instance config (new, 1 migration)
- `internal/settings.Store`: wrap sqlc queries, expose narrow seam (new package)
- `internal/notifier.Notifier`: gains `SettingsReader` seam + `SendDigest` method (modified, additive)
- `internal/discord.Client`: gains `SendBatch` for multi-embed messages (modified, generalization)
- `internal/digest`: ticker-driven scheduler, mirrors `authgate.sweepLoop` (new package)
- `internal/poller`: **unchanged** — mode-switch is invisible
- HTTP routes: new `GET/PUT /settings/digest` (inside existing protected group)

### Critical Pitfalls

1. **Un-gated real-time drain alongside digest** — Both poll-cycle `NotifyPending` and new digest job drain same outbox, risking double-post. **Prevention:** Gate at top of `NotifyPending` via fresh settings read; when digest is on, per-cycle call becomes no-op. **Test:** Toggle on, insert event, run poll cycle and digest tick, assert exactly one Discord send.

2. **Timezone and Alpine tzdata failures** — Operator configures "9am," but Alpine lacks tzdata and robfig/cron defaults to UTC container time. **Prevention:** (1) Blank-import `time/tzdata` in main.go; (2) store operator's IANA zone name, not UTC offset; (3) pass `cron.WithLocation(operatorZone)` explicitly. **CI check:** smoke test that built image can `time.LoadLocation("America/New_York")`.

3. **DST transitions skip or double-fire** — robfig/cron has documented gaps on spring-forward and fall-back. **Prevention:** (1) Fire times outside 2am DST window (9am safe); (2) make job idempotent so double-fire on fall-back finds zero new events and suppresses; (3) log every tick so anomalies are visible. **Test:** advance fake clock across both DST boundaries, assert exactly one digest per calendar day.

4. **Window-boundary events lost or re-sent** — Time-range queries can drop events at boundaries or create gaps. **Prevention:** Don't window by wall-clock time — window by outbox state. Use `ListUnnotified` (SELECT WHERE notified_at IS NULL) with no time-range predicate. "The digest window" = "everything since last successful send," self-correcting for late/skipped ticks.

5. **Process restart near fire time skips cycle** — robfig/cron resets on restart; deploy mid-fire loses that fire (though data safe via outbox). **Prevention:** Store `last_digest_sent_at` in Postgres so on boot scheduler detects gap and logs warning. Surface in SPA System view so missed cycles are operator-visible.

6. **Mid-window toggle orphans queued events** — Toggling digest→real-time mid-window risks losing queued events. **Prevention:** Keep exactly one outbox (no second "digest queue" table), so toggle-off has automatic, correct behavior: next real-time poll cycle picks up queued events and sends individually. Document in SPA UI.

7. **Discord embed/message limits silently truncate** — Unbounded digest formatter hits 6000-char message limit or 10-embed cap. **Prevention:** Chunk into ≤10-embed groups and ≤25-field embeds (or use Description list lines, which scale further); send multiple messages with existing 400ms spacing; only call `MarkNotified` for events in confirmed 2xx Discord responses. **Test:** run digest with >25 events, assert multiple embeds/messages sent, all events reach notified state.

---

## Implications for Roadmap

### Phase 20: Digest Foundation (Postgres + Go packages, inert)

**Rationale:** Additive, no dependents yet. Enables parallel SPA design while backend under review.

**Delivers:**
- `notification_settings` table + sqlc queries
- `internal/settings.Store` package (mirrors `pollruns.Store`)
- `internal/notifier` gains `SettingsReader` seam + `SendDigest` method (wired but inert)
- `internal/discord.Client.SendBatch` method
- Updated `make sqlc-check` passes

**Addresses:** Pitfall 1 (seam in place), Pitfall 4 (windowing uses `ListUnnotified` unchanged)

**Research flags:** None — entirely additive, follows established patterns

### Phase 21: Digest Scheduler (time.Ticker-driven goroutine)

**Rationale:** Depends on Phase 20. Isolates scheduling correctness (DST, restart, timezone).

**Delivers:**
- `internal/digest` ticker-driven scheduler (mirrors `authgate.sweepLoop`)
- ENV config for `DIGEST_CHECK_INTERVAL` (e.g., 5m)
- CAS overlap guard via separate `atomic.Bool`
- DST coverage test (fake-clock across spring-forward and fall-back)
- Timezone handling test (`time.LoadLocation` on operator zone)
- Restart-resilience test (kill/restart near fire, verify catch-up next tick)

**Addresses:** Pitfalls 2, 3, 5 (timezone, DST, restart safety)

**Research flags:**
- **Timezone scope for v1.5:** Research assumes "daily/weekly cadence only, UTC fire time" based on scope. If operator-configurable local fire time is in scope, settings schema and Phase 21 expand (IANA zone picker, DST arithmetic). Clarify during Phase 21 planning.

### Phase 22: HTTP Settings Routes + SPA UI

**Rationale:** Depends on Phases 20–21. Enables operator-facing toggle and observability.

**Delivers:**
- `GET/PUT /settings/digest` routes (inside existing protected group, gated by authgate)
- SPA digest control: toggle, cadence selector (daily/weekly), "last sent" timestamp display
- `/status` System view extended to show `last_digest_sent_at` and next-scheduled-fire time
- Toggle-mid-window behavior documented in UI copy

**Addresses:** Pitfall 5 (last-sent visibility), Pitfall 6 (documented toggle behavior)

**Research flags:** None — follows existing patterns

### Phase 23: Mutual-Exclusion Integration (real-time ↔ digest gating)

**Rationale:** Final integration — wires mode-check seam into `poller.runCycle`'s end-of-cycle call.

**Delivers:**
- Poller gains optional `SettingsReader` seam (or `NotifyPending` extended with settings-gating callback)
- Updated `cmd/server/main.go` composition root: construct `settings.Store`, pass to `notifier.New`, construct and `.Start()` digest scheduler
- Double-post prevention test: toggle digest on, poll both sources, assert exactly one Discord send (Pitfall 1 test)
- Mode-toggle test: queue events under digest, toggle to real-time, assert flushed via next poll cycle (Pitfall 6 test)

**Addresses:** Critical Pitfall 1 (un-gated real-time drain) — this phase enforces the mode decision

**Research flags:**
- **Discord chunking under load:** If digest ever produces >10 embeds, multiple messages sent. Verify spacing (`defaultSpacing`) respected between chunks and all chunks within one `SendDigest` run use same updated `last_digest_sent_at`.

### Phase 24: Discord Chunking + UX Polish (P2 fast-follow)

**Rationale:** Ship Phase 23 first; Phase 24 refines once digest mode is live and behavior validated.

**Delivers:**
- Group-by-event-type or artist presentation within digest body (reorder embeds, use Description list lines for density)
- "Since [timestamp]" header/footer in digest message (cosmetic, adds trust)
- Defensive +N-more truncation when event list exceeds Discord limits (logged as warning)

**Addresses:** Pitfall 7 (graceful degradation at high volume)

**Research flags:** None — pure presentation-layer refinement

### Phase Ordering Rationale

- Phase 20 → 21 → 22 → 23 is strict dependency chain
- Phase 24 can start in parallel with Phases 22–23, pure UI-only after Phase 21
- Phase 20 first: additive migration + seams, reviewable independently
- Phase 21 before UI: scheduler correctness (DST, timezone, restart) complex enough for own phase
- Phase 23 separate integration phase: mutual-exclusion gating is the riskiest behavior; isolation ensures it doesn't slide or get overlooked
- Phase 24 P2 fast-follow: MVP (Phases 20–23) delivers core goal; grouping and cosmetics validated additions once base behavior proven

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| **Stack** | HIGH | No new third-party deps; all reuses from existing go.mod. Confirmed against internal/notifier, internal/poller, internal/discord via direct codebase read. |
| **Features** | MEDIUM | MVP features well-defined and corroborated by commercial digest-vendor consensus (SuprSend, Novu, NotificationAPI, Knock). P2 features straightforward presentation changes. No novel feature class. |
| **Architecture** | HIGH | Entirely grounded in existing codebase patterns (pollruns.Store, D-11 seams, D-08 CAS, authgate.sweepLoop). Mode-switch is single boolean gate on existing seam — low architecture risk. |
| **Pitfalls** | MEDIUM–HIGH | Pitfalls 1, 4, 6 derived directly from existing outbox-pattern design (HIGH). Pitfalls 2, 3 are well-documented robfig/cron + Alpine/Go interactions (MEDIUM, needs verification). Pitfalls 5, 7 standard cron/Discord limits (MEDIUM, industry-standard mitigations). |

**Overall confidence:** MEDIUM–HIGH

### Gaps to Address

1. **Operator timezone handling scope for v1.5:** Research assumes "daily/weekly cadence only, UTC fire time." If operator-configurable local fire time is in scope, settings schema and Phase 21 scheduler design expand significantly. **Action:** Clarify with PO during Phase 21 planning.

2. **Digest event grouping strategy:** Research recommends list-style embeds (compact per-event lines in Description) over field-per-event for better scaling. Exact UI hierarchy (group by artist? by event type? both?) is design decision, not research finding. **Action:** Capture during Phase 24 design; Phase 23 MVP can use flat chronological, add hierarchy as Phase 24 polish.

3. **Discord chunking edge case:** Research assumes multi-message chunking acceptable (tested Phase 24). Verify operator's actual watchlist size makes this realistic scenario vs. theoretical edge case. **Action:** Capture watchlist metrics at v1.5 launch to inform Phase 24 prioritization.

4. **Restart-catch-up behavior:** Research recommends storing `last_digest_sent_at` and logging warning on boot if last send >24h ago. Whether to also fire immediate catch-up send (closing gap) vs. just logging (let next natural tick pick up) is UX choice. **Action:** Decide during Phase 21 design; either way, outbox-state windowing means nothing lost.

---

## Sources

- Project codebase (direct read): `internal/notifier/`, `internal/poller/`, `internal/discord/`, `internal/pollruns/`, `internal/authgate/`, `queries/events.sql`, `.planning/PROJECT.md`
- `.planning/research/STACK.md`, `FEATURES.md`, `ARCHITECTURE.md`, `PITFALLS.md` (this milestone's own research files)
- robfig/cron GitHub source + issue tracker (DST/timezone behavior)
- Discord developer documentation + independent corroborating guides (embed/message/rate limits)
- Commercial digest-notification vendor docs (SuprSend, Novu, NotificationAPI, Knock) — general digest-system design conventions
