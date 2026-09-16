# Pitfalls Research

**Domain:** Scheduled digest/batch notifications, added onto an existing real-time per-event Discord notification pipeline (drop-tracker v1.5)
**Researched:** 2026-09-11
**Confidence:** MEDIUM (general digest/cron/Discord findings cross-checked across multiple sources; drop-tracker-specific findings are HIGH — verified directly against this repo's schema and code)

## Codebase Grounding (read this before the pitfalls below)

The existing real-time notifier is **already an outbox/queue pattern**, not a fire-on-insert push:

- `events.notified_at TIMESTAMPTZ` (nullable) is the queue marker. `ListUnnotified` (`queries/events.sql`) is `SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC`.
- `MarkNotified` is `UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL` — a per-row atomic claim, called only after a confirmed Discord send (`internal/notifier/notifier.go`, D-06/D-07/D-10).
- `Notifier.NotifyPending` is invoked once at the end of **every** poll cycle (`internal/poller/poller.go:514`), for both the MusicBrainz and Deezer cycles independently, guarded by a single shared `notifying atomic.Bool` CAS flag so overlapping cycles never double-drain.
- `events_unnotified_idx` is a partial index on `notified_at IS NULL`, sized for "usually near-empty."

This matters enormously for digest design: **the queue already exists.** Digest mode is not "build a new buffer," it's "change who drains `notified_at IS NULL` rows, how often, and how many messages the drain produces." Most of the pitfalls below follow directly from that reframing — the dangerous move is building a *second*, parallel queueing mechanism (an in-memory buffer, a new table, a second timestamp column) instead of extending the one that's already race-tested and restart-safe.

---

## Critical Pitfalls

### Pitfall 1: Real-time drain stays wired in, digest mode just adds a second consumer on top

**What goes wrong:**
`poller.go` calls `p.notifier.NotifyPending(...)` unconditionally at the end of every poll cycle today. If digest mode is implemented as a new, separate scheduled job that also drains `notified_at IS NULL` rows, but the existing per-poll-cycle call to `NotifyPending` is left untouched, every event gets posted twice: once immediately (real-time path, still wired) and once again in the next digest batch — except `MarkNotified`'s `AND notified_at IS NULL` guard means the real-time path wins the race almost every time, so digest mode silently does nothing while looking "toggled on." Either failure mode (double-post, or digest mode that's a no-op) is easy to ship because both code paths compile and pass tests that only exercise one mode at a time.

**Why it happens:**
The toggle is a config value, not a structural change to the call graph. It's tempting to gate the *content* of what a drain sends (batch vs. individual) while forgetting to gate *whether the per-poll-cycle drain runs at all*.

**How to avoid:**
Make the digest toggle a single decision point that both `poller.go`'s per-cycle call site and the new scheduled digest job read from the same source: when digest mode is on, `NotifyPending`'s per-cycle invocation becomes a no-op (or is skipped entirely) and only the digest cron job drains `notified_at IS NULL`; when digest mode is off, the digest cron job's own tick is a no-op and the existing per-cycle drain resumes. One `Sink`-shaped seam deciding "who currently owns draining the outbox" is safer than two independent boolean checks that can drift out of sync.

**Warning signs:** A test that toggles digest mode on, inserts an event, runs both the poll cycle and the digest tick, and asserts Discord received exactly one message — if that test doesn't exist, this bug is very likely live.

**Phase to address:** The phase that wires the digest scheduler into the existing poll-cycle/notifier seam (not the phase that only builds the DB-persisted toggle and SPA control).

---

### Pitfall 2: Operator's "9am" isn't the container's "9am" — timezone and Alpine tzdata

**What goes wrong:**
The Docker image is a multi-stage build on `alpine` (per this project's Dockerfile/stack decisions). Alpine's minimal base does **not** ship the IANA timezone database — Go's `time.LoadLocation("America/New_York")` (or whatever zone the operator picks in the SPA) will fail at runtime with `unknown time zone` unless `tzdata` is installed in the final stage, *or* Go's embeddable `time/tzdata` package is blank-imported so the zoneinfo is baked into the binary itself. Separately, `robfig/cron` defaults to the **host's local timezone** unless `cron.WithLocation(...)` is passed explicitly — in a container that's almost always UTC, not the operator's timezone, so a schedule entered as "fire at 9am" silently fires at 9am UTC instead.

**Why it happens:**
This class of bug is invisible in local dev (a developer's own machine has full tzdata and a "real" local timezone that happens to match their expectations) and only surfaces in the actual deployed container, which is a different environment than where it was written and tested.

**How to avoid:**
1. Store the operator's chosen IANA zone name (e.g. `"America/Chicago"`) in Postgres alongside the digest config, not a UTC offset — offsets don't carry DST information.
2. Blank-import `time/tzdata` in `main.go` (`_ "time/tzdata"`) so the zoneinfo database is compiled into the binary and independent of the base image — cheaper and more reliable than relying on an Alpine package staying installed across image rebuilds.
3. Pass `cron.WithLocation(loc)` explicitly when constructing the digest cron entry, using the operator's stored zone, not the process default.
4. Never store or compute the digest fire time in UTC and then "convert for display only" — compute the next fire time using `time.Date(..., loc)` in the operator's zone so DST arithmetic is correct by construction.

**Warning signs:** Any code path that treats the digest time as a bare `HH:MM` string without an accompanying zone; any `time.Now()` call in the digest scheduler with no explicit `.In(loc)`.

**Phase to address:** The phase that builds the digest scheduler itself — this must be settled before any cron-registration code is written, and needs a Dockerfile check (or CI smoke test) that the built image can actually `time.LoadLocation` a real zone name.

---

### Pitfall 3: DST transitions skip or double-fire the digest

**What goes wrong:**
`robfig/cron` (confirmed on the maintained v3 line, which this project already depends on) has documented, still-open gaps around spring-forward and fall-back: a job scheduled inside the skipped "spring forward" hour (e.g. 2:30am when clocks jump from 2:00 to 3:00) fires immediately when the clock jumps rather than being silently lost, but the fall-back case — where the local hour repeats — has ambiguous, under-documented behavior on whether the job fires once or twice. A digest scheduled for a fixed local wall-clock time (e.g. "9:00am daily") will hit this twice a year in any timezone that observes DST.

**Why it happens:**
Cron libraries reason in wall-clock time; DST transitions are precisely the two days a year wall-clock time is not a monotonic, one-to-one mapping onto real time. This is a known, structural limitation of wall-clock cron scheduling, not a bug specific to this project.

**How to avoid:**
- Pick a fire time unlikely to fall in a transition window where possible (DST transitions in the US happen at 2am local, not 9am, so a mid-morning/evening digest time mostly sidesteps the ambiguous-hour case — but don't assume this holds for every timezone an operator might pick).
- Make the digest job **idempotent on the send side**, not just the schedule side: the window-selection query (see Pitfall 4) should derive its event set from `notified_at IS NULL`, not from "did cron tick." A double-fire on a DST fall-back day then finds nothing new to send on the second tick (empty digest → suppressed, see Pitfall 7) rather than sending duplicate content.
- Log every digest cron fire (scheduled time, actual fire time, event count) so a DST-week anomaly is visible in `/status` or logs rather than silently causing a missed or duplicate operator-facing message.

**Warning signs:** No log line distinguishing "cron ticked, N events found" from "cron ticked, 0 events found" — without that, a DST-week skip/double-fire looks identical to "just a quiet day" in the logs.

**Phase to address:** Same phase as Pitfall 2 (scheduler construction) — add a test that advances a fake clock across both DST boundaries and asserts the digest fires exactly once per calendar day either side of the transition.

---

### Pitfall 4: Window-boundary events are neither lost nor double-sent, but land in a non-obvious cycle

**What goes wrong:**
If the digest job selects events by a time-range query (`created_at BETWEEN window_start AND window_end`), an event whose `created_at` lands within milliseconds of the boundary can end up in either window depending on exact commit timing versus query-snapshot timing — not "double-sent" (Postgres snapshot isolation prevents that), but attributed to a different day's digest than an operator watching a clock would expect. Worse, if the window boundaries are computed independently each run (e.g. "now minus 24h") rather than anchored to the last successful send, a delayed or skipped cron tick (Pitfall 3, Pitfall 5) silently shifts or narrows the window, and events can fall into the **gap** between two windows and never get selected by either.

**Why it happens:**
Time-range windowing assumes cron fires exactly on schedule every time. Combined with Pitfall 5 (restarts) and Pitfall 3 (DST), that assumption doesn't hold.

**How to avoid:**
Don't window by time range at all — window by **outbox state**, matching the existing real-time pattern. The digest query should be `SELECT * FROM events WHERE notified_at IS NULL ORDER BY created_at ASC, id ASC` (the exact `ListUnnotified` query that already exists), with no time-range predicate. "The digest window" becomes "everything that accumulated since the last successful digest send," which is self-correcting: a late, skipped, or double-fired cron tick changes *when* the digest goes out, never *whether* an event gets included exactly once. This also means an event detected in the last second before the digest job's query runs is safely included (it's just an unclaimed row), and an event detected one second after is safely deferred to the next cycle — no boundary ambiguity, no lost row.

**Warning signs:** Any digest query with a `created_at >= $1 AND created_at < $2` predicate instead of `notified_at IS NULL` — that's the tell that windowing is being done by wall-clock time instead of by outbox state.

**Phase to address:** The phase that writes the digest event-selection query — should reuse/extend `ListUnnotified`, not introduce a parallel time-ranged query.

---

### Pitfall 5: Process restart near the fire time silently skips that cycle (schedule, not data)

**What goes wrong:**
`robfig/cron`'s schedule lives entirely in process memory — it computes each entry's next fire time from `Now()` at `cron.Start()`, with no persisted "last fired at" or "missed run" catch-up. If the container restarts (a deploy — this app restarts on every merge-to-main release per its existing CI/CD pipeline) in the minute the digest was due to fire, that fire is simply gone: on restart, cron recomputes the *next* scheduled time from the new `Now()`, which for a daily digest is tomorrow, and for a weekly digest is up to six days later.

**Why it happens:**
This is a direct consequence of using an in-process scheduler with no persistence — which is the correct, already-validated choice for this project's poll cycles (ADR-0001 made the same call for poll-run history: single-instance, restart-reset-by-design is acceptable there). But a digest's failure mode is more visible to the operator than a poll-run history entry: "I configured a weekly digest and didn't get one this week" is a much louder signal than "the ring buffer reset."

**How to avoid:**
- This is *not* a data-loss risk, thanks to Pitfall 4's design: skipped events stay `notified_at IS NULL` and simply roll into the next successful cycle. Make sure this stays true — do not let any digest-adjacent code mark events notified before a confirmed Discord send.
- Store `last_digest_sent_at` in Postgres (not memory) so that on boot, the scheduler can detect "the last successful send was more than one full cadence period ago" and log a visible warning (or optionally fire an immediate catch-up send) rather than silently waiting for the next natural cron tick.
- Surface `last_digest_sent_at` in `/status` (this project already has a "System" observability surface from v1.4 — extending it, rather than inventing a new diagnostic path, is the lower-risk move) so a missed cycle is operator-visible instead of only discoverable by an empty Discord channel.

**Warning signs:** No persisted "last sent" timestamp anywhere outside cron's in-memory state; a digest feature with no `/status`-visible field showing when it last actually ran.

**Phase to address:** Scheduler-construction phase for the persisted cursor; the observability-surfacing part can ride along with whatever phase touches the SPA digest config screen, since that's already the natural place an operator checks "is this working."

---

### Pitfall 6: Toggling digest → real-time mid-window orphans the queued events

**What goes wrong:**
Say digest mode is on, three events have accumulated (`notified_at IS NULL`, waiting for Friday's digest), and the operator switches the toggle to real-time on Wednesday. If the per-poll-cycle `NotifyPending` call is simply re-enabled going forward, it will pick up those three already-queued events on the very next poll cycle and post them individually — which may be exactly right (nothing is lost, they just arrive as three separate real-time messages instead of one digest) or may be jarring if the operator's mental model was "those are gone, digest mode ate them." Conversely, if the implementation instead moves a "digest queue" into some other bucket when digest mode is on and doesn't reconcile it on toggle-off, those events never get sent at all — a genuine drop.

**Why it happens:**
This is the direct consequence of *not* following Pitfall 1's guidance (single shared outbox, single active consumer). If the digest feature is built with its own queue separate from `notified_at IS NULL`, toggling modes mid-window becomes a data-migration problem instead of a no-op.

**How to avoid:**
Keep exactly one outbox (`notified_at IS NULL`) and exactly one active consumer determined by the current mode at drain time, decided fresh on every drain attempt (poll-cycle tick or digest cron tick) rather than cached at toggle time. With that design, toggling digest → real-time mid-window has an automatic, correct, and easily-explained behavior: whatever's still unclaimed gets swept up by the next real-time poll cycle and sent individually, with no special-case flush code needed. Document this behavior explicitly in the SPA copy near the toggle ("switching to real-time will immediately send any events that built up while digest mode was on") so it's a stated contract, not an accidental side effect an operator discovers by surprise.

**Warning signs:** Any new column/table (`digest_queue`, `pending_digest_events`, a second `*_at` timestamp) introduced specifically for digest mode — that's a sign a second, parallel outbox is being built instead of reusing the one that exists.

**Phase to address:** Same phase as Pitfall 1 — this is really one design decision (single outbox, mode-selected consumer) with two observable consequences.

---

### Pitfall 7: Discord embed/message limits silently truncate or drop events in a busy digest

**What goes wrong:**
A Discord embed is capped at 25 fields, 6000 total characters across all embeds in one message, and a single message can carry at most 10 embeds; per-webhook send rate is roughly 5 requests per 2 seconds. A digest that naively tries to pack every event from a busy day/week into one embed's fields will either get rejected outright by Discord's API once a limit is crossed, or — worse, if the code truncates the field list to "the first 25" without any further handling — silently drops the rest with no operator-visible signal and no corresponding `MarkNotified` skipped, meaning those events are *not* marked notified and will confusingly reappear in the *next* digest (partially mitigating data loss, but producing a duplicate-looking entry days later).

**Why it happens:**
The existing real-time notifier only ever formats one event per embed, so there's no existing code path in this codebase that has ever had to chunk N events across multiple embeds/messages — this is genuinely new surface area, not an extension of a pattern that's already been battle-tested here.

**How to avoid:**
- Chunk events into multiple embeds (up to 25 fields each) and multiple messages (up to 10 embeds each) as needed, rather than assuming one message suffices.
- Only call `MarkNotified` for events actually included in a message that received a confirmed 2xx from Discord — matching the existing real-time contract of "mark after confirmed send," applied per-chunk rather than per-event-in-a-loop.
- Respect the existing 400ms inter-send spacing constant (`defaultSpacing` in `internal/notifier/notifier.go`, already tuned to Discord's 5-req/2s ceiling) between chunked digest messages, the same way the real-time path already does between individual sends — a digest firing 5 chunked messages back-to-back with no spacing can trip the same rate limit the real-time path was built to avoid.
- For a single-operator, modest-watchlist project, this is a low-probability-but-not-impossible edge case (a very active week across many watched artists) — it doesn't need to be over-engineered, but it must degrade gracefully (multiple messages) rather than silently (dropped fields) when it does happen.

**Warning signs:** A digest formatter that builds one `discord.Embed` and appends fields in an unbounded loop with no length/count check before sending.

**Phase to address:** The phase that implements digest message formatting/sending — should extend `internal/discord`'s existing embed-building code and `internal/notifier`'s spacing constant rather than hand-rolling a new send path.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|-----------------|
| Separate in-memory buffer for "events pending digest," built alongside the existing `notified_at`-based outbox | Feels like a clean, isolated feature module | Reintroduces the exact restart-data-loss risk the DB-backed outbox already solved; requires new reconciliation logic for mode toggles (Pitfall 6) | Never — reuse `notified_at IS NULL` |
| Fixed UTC-offset digest time instead of an IANA zone name | Simpler config, no tzdata dependency | Breaks twice a year at DST transitions with no natural fix | Never, once an operator-facing "pick your local time" control exists |
| Digest window computed as `now() - 24h` / `now() - 7d` instead of outbox-state-based | Simple, intuitive-sounding query | Boundary/gap bugs under any schedule drift (restart, DST, late cron tick) | Only acceptable for a throwaway prototype/demo, never for the shipped feature |
| Single unbounded embed with all events appended as fields, no chunking | Fastest to implement, works fine at current watchlist scale | Silent drop or hard API rejection the first time a digest crosses 25 events | Acceptable temporarily behind an explicit `TODO` + a hard cap that logs a warning, not acceptable as the final shipped behavior |
| Reading the digest on/off + cadence config once at process boot instead of per-tick | Simpler code, no need for a config-watch mechanism | Contradicts the milestone's explicit goal ("changeable without a redeploy") — a toggle flip in the SPA would silently do nothing until the next restart | Never, given the stated requirement |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|--------------|------------------|--------------------|
| Discord webhooks (digest message) | Packing unlimited events into one embed's fields | Chunk at 25 fields/embed, 10 embeds/message, 6000 total chars/message; send multiple messages if needed |
| Discord webhooks (digest message) | Firing several chunked messages back-to-back with no spacing | Reuse the existing 400ms inter-send spacing (`defaultSpacing`) between chunks, same as real-time sends |
| Discord webhooks (digest message) | Treating a 429 on a digest send the same as a single dropped event | A 429 mid-digest should retry the whole failed chunk (or back off and retry the batch), not silently mark some events notified and lose others — keep the per-chunk "mark only on confirmed send" discipline |
| `robfig/cron` (already a project dependency) | Constructing the digest cron entry with the library's default (host-local) timezone | Pass `cron.WithLocation(operatorZone)` explicitly, sourced from the Postgres-persisted config, not process default |
| `robfig/cron` (already a project dependency) | Registering the cron schedule once at boot and expecting a Postgres config change to take effect | Support re-registering the cron entry (remove + re-add, or use a dynamically-computed `Schedule`) when the operator changes cadence/time via the SPA, without requiring a restart |
| Postgres-persisted config (new for this milestone — everything else in the app is env-var-only per CLAUDE.md) | Treating this like the rest of the app's env-var config (read once, cached forever) | This is intentionally a runtime-mutable exception to the "env vars only" convention — design the read path (cache invalidation or per-tick read) accordingly, and call this out explicitly since it's a deliberate deviation from an established project convention |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|-----------|-------------|-----------------|
| Unbounded digest event count read into memory in one query | Fine at current single-operator watchlist scale | `ListUnnotified` already has no `LIMIT`; acceptable given this project's scale, but worth a sanity cap (e.g. log a warning past a few hundred pending events) so a stuck/misconfigured digest doesn't silently build an unbounded backlog | Not a near-term concern for this project's single-operator, modest-watchlist scope — flag only, don't build infrastructure for it |
| Digest formatter re-fetching artist/cover-art data per event synchronously before sending | Slow digest send on a busy day, blocking the cron goroutine | Reuse whatever display-field caching the existing per-event notifier already does; the events table already stores denormalized display fields (title, artist_name, cover_art_url) precisely so this isn't needed | Only relevant if a future digest redesign starts re-querying MusicBrainz/Deezer at send time, which nothing here requires |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| No indication of "last digest sent" anywhere in the SPA | Operator can't tell whether digest mode is actually working or silently stuck (Pitfall 5) | Surface `last_digest_sent_at` and next-scheduled-fire time in the System view alongside the digest toggle |
| Empty digest sent when zero events accumulated | A blank/near-empty Discord message every day erodes trust in the feature and trains the operator to ignore it | Suppress the send entirely when the outbox is empty at fire time; log the no-op tick instead |
| Toggling digest → real-time with no explanation of what happens to queued events | Operator surprised by a burst of "old" individual messages, or worse, silently loses them if Pitfall 6 wasn't handled correctly | State the flush behavior explicitly in the SPA UI copy near the toggle (see Pitfall 6) |
| Digest time picker that accepts a bare `HH:MM` with no timezone selector | Operator has no way to correctly express intent; falls back to server default (likely UTC), producing the exact bug in Pitfall 2 | Timezone selector (or auto-detected browser timezone as the default, explicitly confirmed/overridable) alongside the time picker |

## "Looks Done But Isn't" Checklist

- [ ] **Digest toggle wired end-to-end:** Verify the *existing* per-poll-cycle `NotifyPending` call is actually gated off when digest mode is on — not just that a new digest job exists alongside it (Pitfall 1).
- [ ] **Timezone correctness:** Verify the built container image (Alpine-based) can actually `time.LoadLocation` a real IANA zone name, not just that the code compiles locally where tzdata is already present (Pitfall 2).
- [ ] **DST coverage:** Verify a test exists that advances a fake clock across both a spring-forward and a fall-back boundary and asserts exactly one digest fires per calendar day (Pitfall 3).
- [ ] **Restart resilience:** Verify killing the process a few seconds before a scheduled digest fire, then restarting, still results in that day's events reaching Discord on the next tick — not silently lost (Pitfall 4, Pitfall 5).
- [ ] **Toggle-mid-window behavior:** Verify switching digest → real-time with events already queued either flushes them via the next real-time poll cycle or explicitly documents/tests the chosen behavior — not left unspecified (Pitfall 6).
- [ ] **Discord limit handling:** Verify a digest with more events than fit in one embed/message actually sends multiple chunked messages instead of erroring or truncating silently (Pitfall 7).
- [ ] **Cadence change without redeploy:** Verify changing the digest time/cadence via the SPA takes effect on the *next* scheduled fire without a container restart — this is an explicit milestone goal, easy to accidentally regress to "read config at boot only."

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|-----------------|-------------------|
| Double-send from an un-gated real-time drain running alongside digest mode (Pitfall 1) | LOW | Since `MarkNotified` is idempotent per-row, no DB cleanup is needed; fix the gating logic and ship — no data corruption occurred, only duplicate Discord messages |
| Wrong timezone causing digests to fire at an unexpected hour (Pitfall 2) | LOW | Backfill `time/tzdata` import / operator zone config, redeploy; no data lost since events remain queued via `notified_at IS NULL` regardless of when the digest fires |
| A missed digest cycle from a restart or DST edge case (Pitfall 4, Pitfall 5) | LOW | No recovery action needed if outbox-state windowing (Pitfall 4) was followed — the missed cycle's events are still `notified_at IS NULL` and go out on the next successful tick automatically |
| Orphaned queued events after a mode toggle, if a separate digest-only queue was built instead of reusing `notified_at` (Pitfall 6) | MEDIUM | Requires a one-off manual query/backfill to reconcile the separate queue's state back into `notified_at`, plus a follow-up fix to collapse to a single outbox going forward |
| Silently dropped events from an un-chunked oversized digest embed (Pitfall 7) | MEDIUM | If `MarkNotified` was (incorrectly) called before confirming the send succeeded, affected events must be identified and manually reset (`notified_at = NULL`) to re-queue them; if the "mark only on confirmed send" discipline was followed correctly, no recovery is needed — they simply remain queued |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|--------------------|-----------------|
| Un-gated real-time drain running alongside digest mode (1) | Scheduler/outbox-integration phase | Test: toggle digest on, insert event, run both a poll cycle and a digest tick, assert exactly one Discord send |
| Timezone / Alpine tzdata (2) | Scheduler-construction phase | CI or image-smoke-test step that `time.LoadLocation`s a real zone inside the built container; `cron.WithLocation` unit test |
| DST transitions (3) | Scheduler-construction phase | Fake-clock test crossing both DST boundaries, asserting exactly one fire per day |
| Window-boundary ambiguity (4) | Event-selection query phase | Test asserting an event inserted mid-drain is included exactly once across two consecutive digest ticks, never zero or twice |
| Restart near fire time (5) | Scheduler-construction phase + System-view extension | Kill/restart test around a scheduled fire time; `/status` shows `last_digest_sent_at` |
| Toggle mid-window orphaning events (6) | Same phase as (1) | Test: queue events under digest mode, toggle to real-time, assert they're sent on the next poll cycle |
| Discord embed/message limits (7) | Digest message-formatting phase | Test with an event count exceeding 25, asserting multiple embeds/messages sent and all events end up `notified_at IS NOT NULL` |
| Cadence change requires restart (Technical Debt row) | Postgres-config-read phase | Test: change cadence via API/SPA path, assert next fire uses new cadence with no process restart |

## Sources

- `internal/db/migrations/000003_events.up.sql`, `queries/events.sql`, `internal/notifier/notifier.go`, `internal/poller/poller.go` (this repository) — existing outbox/queue design (`notified_at`, `ListUnnotified`, `MarkNotified`, per-poll-cycle `NotifyPending` call site, 400ms spacing constant) — confidence HIGH (primary source, read directly)
- `.planning/PROJECT.md` (this repository) — milestone goal (Postgres-persisted, no-redeploy-required toggle), Alpine-based Dockerfile decision, single-instance/restart-on-deploy deployment model — confidence HIGH
- [github.com/robfig/cron](https://github.com/robfig/cron), [Enhancement: UTC · Issue #180](https://github.com/robfig/cron/issues/180), [Set Timezone for Scheduler · Issue #132](https://github.com/robfig/cron/issues/132), [pkg.go.dev/github.com/robfig/cron/v3](https://pkg.go.dev/github.com/robfig/cron/v3) — DST spring-forward/fall-back behavior, default-to-host-timezone behavior, `cron.WithLocation` — confidence MEDIUM (project's own dependency's issue tracker, cross-checked across multiple pages)
- [docs.discord.com/developers/topics/rate-limits](https://docs.discord.com/developers/topics/rate-limits), [Discord Embed Limits Cheat Sheet](https://discord-webhook.com/en/blog/discord-webhook-embed-limits/), [discord.com/safety/using-webhooks-and-embeds](https://discord.com/safety/using-webhooks-and-embeds) — 25 fields/embed, 6000 chars/message, 10 embeds/message, ~5 requests/2s per webhook, global 50 req/s — confidence MEDIUM (official Discord docs plus independent corroborating sources)
- [Wawandco: Go's Locations & Alpine Docker image](https://wawand.co/blog/posts/go-time-default-locations/), [A story about Go, Docker and time zones](https://lalatron.hashnode.dev/a-story-about-go-docker-and-time-zones) — Alpine missing tzdata, `time.LoadLocation` failure mode, `time/tzdata` blank-import fix — confidence MEDIUM (independent, corroborating sources; well-known Go/Alpine interaction)
- [Knock: Building a batched notification engine](https://knock.app/blog/building-a-batched-notification-engine), [SuprSend: How Notification Batching and Digests Actually Work](https://www.suprsend.com/post/notification-batching-and-digest), [techinterview.org: Digest Scheduler Low-Level Design](https://www.techinterview.org/post/3233470550/lld-digest-scheduler/) — outbox-state vs. time-range windowing, idempotency-key dedup pattern, empty-digest suppression — confidence MEDIUM (industry vendor engineering blogs, corroborating on the same core patterns)
- Kubernetes CronJob `startingDeadlineSeconds` / missed-schedule documentation (general cron catch-up pattern references) — confidence MEDIUM, used only as a general illustration of the "missed schedule window" problem class, not as a direct implementation recommendation for this project (robfig/cron has no equivalent option; the outbox-state approach in Pitfall 4 is the recommended substitute)

---
*Pitfalls research for: digest/batch notification mode, drop-tracker v1.5*
*Researched: 2026-09-11*
