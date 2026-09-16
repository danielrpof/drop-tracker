# Architecture Research — Digest/Batch Notification Mode (v1.5)

**Domain:** Instance-wide notification delivery-mode toggle inside an existing single-binary Go service (poller + notifier + API, no microservices)
**Researched:** 2026-09-11
**Confidence:** HIGH (derived directly from the existing codebase — `internal/poller`, `internal/notifier`, `internal/discord`, `internal/pollruns`, `internal/authgate`, `queries/events.sql` — not from external ecosystem sources; this is an in-repo architecture-fit question, not a "what does the ecosystem look like" question)

This research answers the milestone's central integration question by extending four patterns the codebase already uses successfully, rather than introducing new mechanisms:

1. The **events table as outbox** (`notified_at IS NULL` = pending) — already the queue; digest mode reuses it unchanged, per the milestone's own "no new polling" constraint.
2. The **CAS overlap-guard + narrow consumer-declared seam** idiom from `poller.Poller` (D-08, D-11) — reused for the new digest scheduler.
3. The **ticker-driven background goroutine** idiom from `authgate.Manager.sweepLoop` — reused instead of a third `robfig/cron` entry, and instead of a fixed `"0 9 * * *"` cron spec.
4. The **inert-no-op-default option** idiom (`notifier.NoOp`, `poller.noopRunRecorder`) — reused so digest mode ships additive and off-by-default, matching D-10 and the milestone's "default stays real-time/off" requirement.

## Standard Architecture

### System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                         cmd/server/main.go                            │
│  (composition root — constructs every seam below, then Start()s both  │
│   schedulers)                                                         │
├──────────────────────────┬─────────────────────────┬──────────────────┤
│      internal/poller      │    internal/digest (NEW) │  internal/httpserver│
│  ┌──────────────────┐    │  ┌────────────────────┐  │  ┌─────────────┐ │
│  │ cron: MB cycle    │    │  │ time.Ticker loop   │  │  │ GET/PUT     │ │
│  │ cron: Deezer cycle│    │  │ (mirrors authgate's│  │  │ /settings/  │ │
│  │ (unchanged)       │    │  │  sweepLoop, NOT a   │  │  │ digest      │ │
│  │ end-of-cycle:      │    │  │  3rd cron entry)    │  │  │ (NEW)       │ │
│  │  notifier.NotifyPending│  │  each tick: read    │  │  └──────┬──────┘ │
│  └─────────┬──────────┘    │  settings fresh,      │  │         │        │
│            │                │  CAS-guard, if due:   │  │         │        │
│            │                │  Notifier.SendDigest  │  │         │        │
│            │                └──────────┬────────────┘  │         │        │
├────────────┴───────────────────────────┼────────────────┴─────────┼───────┤
│                     internal/notifier (extended)                          │
│  ┌───────────────────────────────────────────────────────────────────┐  │
│  │ NotifyPending(ctx,logger)  — checks SettingsReader first; if      │   │
│  │   digest ON, returns nil (rows stay pending) — else unchanged     │   │
│  │   real-time drain+send+mark loop (today's behavior)               │   │
│  │ SendDigest(ctx,logger)     — NEW: drains pending, reuses the SAME │   │
│  │   formatEmbed/suppresses/listUnnotified/markNotified helpers,     │   │
│  │   batches into <=10-embed Discord messages, sends via SendBatch   │   │
│  └───────────────────────────────────────────┬─────────────────────┘  │
├──────────────────────────────────────────────┼────────────────────────┤
│              internal/discord (extended: +SendBatch)   internal/settings │
│  ┌────────────────────────────┐              │        (NEW, thin)      │
│  │ sendAttempt(ctx, []Embed)   │◄─────────────┘        ┌──────────────┐│
│  │  Send()      = 1-embed call │                       │ Store.Get    ││
│  │  SendBatch() = N-embed call │                       │ Store.Update ││
│  └──────────────┬──────────────┘                       └──────┬───────┘│
├─────────────────┴──────────────────────────────────────────────┴───────┤
│                              Postgres (pgx/v5, sqlc)                    │
│  ┌──────────────┐          ┌─────────────────────────────┐             │
│  │ events table  │          │ notification_settings (NEW,  │             │
│  │ (unchanged —  │          │  singleton row: enabled,     │             │
│  │  the outbox)  │          │  cadence, last_sent_at)      │             │
│  └──────────────┘          └─────────────────────────────┘             │
└──────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | New or Modified |
|-----------|----------------|------------------|
| `notification_settings` table | Single-row instance config: digest on/off, cadence, last-sent bookkeeping | **New** — one additive migration |
| `internal/settings` | Thin sqlc-backed store: `Get(ctx)`/`Update(ctx, ...)`, typed `Settings` struct | **New** — small package, mirrors `internal/pollruns`'s "wrap sqlc, expose a narrow struct" shape |
| `internal/digest` | Ticker-driven scheduler: reads settings fresh every tick, decides "is a digest due," CAS-guards against overlap, calls `Notifier.SendDigest` when due | **New** — mirrors `authgate.Manager.sweepLoop`, not a third `robfig/cron` entry |
| `internal/notifier.Notifier` | Gains a `SettingsReader` seam + mode check at the top of `NotifyPending`; gains a `SendDigest` method reusing existing private helpers | **Modified** — additive, no signature break on `poller.Notifier`/`Sink` |
| `internal/discord.Client` | Gains `SendBatch(ctx, []Embed) error`; `Send` becomes a 1-embed wrapper over the same internal path | **Modified** — additive |
| `internal/httpserver` | Gains `GET/PUT /settings/digest` inside the existing protected route group | **New routes**, existing group |
| `internal/poller` | **Unchanged.** Still calls `notifier.NotifyPending` at end-of-cycle; digest mode's behavior change lives entirely inside `NotifyPending`, invisible to `poller` | **Unchanged** |
| `web/app` SPA | New settings control (toggle + cadence select), likely a new panel alongside the existing System view | **New** UI, existing page-shell patterns |

## Answering the Three Integration Questions Directly

### 1. New `robfig/cron` entry, or a different scheduling approach?

**Recommendation: a plain `time.Ticker`-driven goroutine (mirroring `authgate.Manager.sweepLoop`), not a third `robfig/cron` entry.**

Reasoning, grounded in what's already in the codebase:

- `poller.Poller` registers its two cron entries with a **fixed spec computed once at construction** (`spec := fmt.Sprintf("@every %s", interval.String())`, passed to `p.cron.AddFunc` inside `New`). robfig/cron has no supported way to change an already-registered entry's spec at runtime — the only path is `cron.Remove(id)` + a fresh `AddFunc` with a new spec, which is exactly the kind of runtime cron-internals reprogramming the milestone is trying to avoid needing every time an operator flips daily↔weekly in the SPA.
- A **literal wall-clock cron spec** (e.g. `"0 9 * * *"`) is fragile against exactly the failure mode a single-instance self-hosted tool is most likely to hit: the process being down, mid-restart, or mid-deploy at 09:00. robfig/cron does not replay a missed tick — if the process wasn't running at 09:00:00, that day's digest silently never fires.
- The project already has a proven, tested, in-repo precedent for "periodic background check against mutable state, stoppable via `Close()`": `authgate.Manager.sweepLoop`, a `time.NewTicker`-driven goroutine with a `sweepDone` channel and a `sync.Once`-guarded `Close`. The digest scheduler is structurally the same shape — periodic check, no cron-expression complexity needed — so reusing that idiom is lower-risk than introducing robfig/cron's expression parser for a feature that only ever uses `@every N`, which a raw ticker already does with less machinery.
- Design: register one goroutine that ticks at a short, fixed, **compile-time interval** (a new `DIGEST_CHECK_INTERVAL` env var, default e.g. `5m`, following the exact `env:"..." envDefault:"..."` pattern already used for `POLL_INTERVAL`/`EVENT_RETENTION_DAYS` in `internal/config/config.go`). On every tick: read `notification_settings` fresh (see Q2), and if `digest_enabled` is true and `now` has crossed the next due boundary for `digest_cadence` since `digest_last_sent_at`, call `Notifier.SendDigest`. A tick that fires late (process was down at the boundary) still fires — "due since X" is a durable, persisted fact (`digest_last_sent_at` in Postgres), not a missed cron tick — which is what makes this approach resilient to restarts/deploys in a way a fixed cron spec is not.
- Overlap guard: mirror `poller`'s CAS idiom exactly — a dedicated `atomic.Bool` (not shared with `notifier.Notifier.notifying`, matching D-08's "separate guards per independent concern" precedent from `mbRunning`/`dzRunning`) so a slow digest send can never collide with a concurrently-arriving tick.
- Fixed target hour: the milestone's locked scope is on/off + daily/weekly cadence only — no specific "9am" requirement is in `PROJECT.md`. Recommend a single compile-time target hour (UTC) for v1.5, exactly the same posture the project already took with `pollruns.N` ("a compile-time constant, deliberately not an environment variable" — ADR-0001 precedent for keeping something a constant until there's a real reason to expose it). If a later milestone wants an operator-configurable hour, it is an additive column on the same settings table — no architecture change.

### 2. Where does the digest setting live, and how is it read?

**Recommendation: a new dedicated singleton table (`notification_settings`), read fresh from Postgres on every check (no boot-time load, no in-process cache).**

Why a new table, not a column on an existing one:
- `events` is per-event-row data (the outbox), not instance config — bolting instance-wide settings onto it would mean every row redundantly carries (or a sentinel row fakes) config state; wrong shape.
- `watchlist`/`artists` are per-artist scope; digest settings are instance-wide, orthogonal to any single artist.
- There is no existing "one row per instance" table (unlike `authgate`, which is env-var-only and therefore has no DB row at all) — this milestone is explicitly the first instance-wide setting that needs to be **SPA-configurable without a redeploy**, which env vars structurally cannot satisfy. A new table is the correct, minimal-surface primitive.
- Precedent for the shape: `pollruns` deliberately rejected a new Postgres table for run-history because that data resets-on-restart by design and the table version carried real concurrency hazards (ADR-0001). Digest settings are the opposite case — they must **survive restart** (an operator's toggle should stick) and involve no concurrency hazard (one low-frequency, single-row read-modify-write from one admin action, not a hot per-artist path) — so a table is the right call here specifically, even though `pollruns` correctly avoided one for its own use case.

Schema sketch (new additive migration, following the existing migrations' comment-heavy style and the N-1 expand/contract rule in `internal/db/migrations/README.md`):

```sql
CREATE TABLE notification_settings (
    id                  INT PRIMARY KEY DEFAULT 1,
    digest_enabled      BOOLEAN NOT NULL DEFAULT false,
    digest_cadence      TEXT NOT NULL DEFAULT 'daily'
                          CHECK (digest_cadence IN ('daily', 'weekly')),
    digest_last_sent_at TIMESTAMPTZ,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT notification_settings_singleton CHECK (id = 1)
);

INSERT INTO notification_settings (id) VALUES (1);
```

The `CHECK (id = 1)` + a migration-time seed `INSERT` gives an always-exactly-one-row table with no application-level "ensure a row exists" logic and no upsert-race handling — `GetNotificationSettings` (`SELECT * ... WHERE id = 1`) and `UpdateNotificationSettings` (`UPDATE ... WHERE id = 1 RETURNING *`) are both trivial, single-row, index-backed (PK) lookups. This is the same "make invalid states unrepresentable via a CHECK constraint" instinct the project already applies elsewhere (`events_source_valid`, `events_event_type_valid`).

**Read timing — the load-bearing decision for "takes effect without a restart":**

Do **not** load settings once at boot into a struct field. Read fresh via a cheap single-row `SELECT` at two call sites:
1. Every digest-ticker tick (every `DIGEST_CHECK_INTERVAL`, e.g. 5 min) — decides whether a digest is due.
2. Every `Notifier.NotifyPending` invocation (i.e. at the end of every poll cycle, both MusicBrainz and Deezer) — decides whether to send real-time or leave rows pending for the digest job.

Both call sites already run at low, bounded frequency (poll cycles are ~15 min apart per source by default; the digest ticker is proposed at 5 min) — the query cost is the same class as `store.List()` already paid once per poll cycle. No cache/invalidation layer is needed: a PK-indexed single-row read is cheap enough that re-querying Postgres each time is simpler and strictly more correct than an in-process cache that would need an explicit invalidation hook wired to the new `PUT /settings/digest` handler. This mirrors the project's own stated bias against premature complexity (ADR-0001's rejection of a `poll_runs` table cited exactly this kind of unnecessary-machinery risk).

### 3. How does `internal/notifier` support both delivery modes without duplicating embed-building?

**Recommendation: keep `formatEmbed` (and its private helpers `appendField`, `truncateRunes`, `tracksFieldValue`, the URL builders) exactly as they are today — pure, unexported, package-private — and add the digest path as a second method on the same `Notifier` type, in the same package, so it can call those unexported functions directly with zero duplication.**

Concretely:

1. **`discord.Client` gains `SendBatch`, generalizing the existing single-request path.** `sendAttempt` already builds a `webhookPayload{Embeds: []Embed{embed}, ...}` from a single embed; widen its parameter to `embeds []Embed` and add a chunking caller:
   ```go
   func (c *Client) Send(ctx context.Context, embed Embed) error {
       return c.sendAttempt(ctx, []Embed{embed}, true)
   }
   func (c *Client) SendBatch(ctx context.Context, embeds []Embed) error {
       return c.sendAttempt(ctx, embeds, true) // caller chunks to Discord's 10-embed/message cap
   }
   ```
   This is a signature widening on an already-private helper, not new marshal/retry/429 logic — `sendAttempt`'s existing 429-retry-once and no-body-echo behavior (D-08, T-03-01) is inherited by both callers for free.

2. **`Notifier` gains a narrow, consumer-declared `SettingsReader` seam**, matching the project's established D-11 convention (seams declared where they're consumed, e.g. `poller.ReleaseGroupSource`, `poller.EventRecorder`):
   ```go
   type SettingsReader interface {
       DigestEnabled(ctx context.Context) (bool, error)
   }
   ```
   injected via a `WithSettingsReader` functional option, defaulting to an always-disabled no-op (mirroring `notifier.NoOp` and `poller.noopRunRecorder`) — so every existing call site and every existing test is unaffected until main.go opts in.

3. **`NotifyPending` gets one new guard at the top**, everything else unchanged:
   ```go
   if enabled, err := n.settings.DigestEnabled(ctx); err != nil {
       logger.Warn("digest settings read failed: falling back to real-time delivery", ...)
       // fall through — fail OPEN to real-time, not fail-closed-to-silence
   } else if enabled {
       return nil // rows stay pending; the digest scheduler owns delivery
   }
   ```
   Failing open (real-time) on a settings-read error is the deliberate choice: a transient DB hiccup should never silently suppress all notifications — real-time was already the safe, validated default (D-10), so an error path should degrade *toward* it, not away from it.

4. **A new `SendDigest(ctx, logger) error` method on `*Notifier`** reuses the existing private plumbing verbatim:
   - `listUnnotified(ctx, n.q)` — same query, same function, unchanged.
   - `n.suppresses(ev)` / `staleReleaseDate` — same staleness gate applied per-event before batching, so a digest can't resurrect a stale backlog any more than real-time can.
   - `formatEmbed(ev)` — the exact same pure transform real-time uses; **this is the whole point of co-locating `SendDigest` in the `notifier` package** rather than a separate top-level package — `formatEmbed` and its helpers are intentionally unexported (per format.go's own comment: kept private "to avoid a compile-time dep on internal/detection"), so any digest logic living outside this package would be forced to either duplicate the formatting logic or force those helpers to become exported API surface they were deliberately kept out of.
   - New: chunk the formatted `[]discord.Embed` into groups of ≤10 (Discord's per-message embed cap) and call `sender.SendBatch(ctx, chunk)` per chunk (mirroring the existing `defaultSpacing` inter-send pacing between chunks, reusing `spacingWait`/`dbOpTimeout` exactly as `NotifyPending` does).
   - On each chunk's successful send, `markNotified` every event in that chunk (same function, same per-row idempotent `WHERE ... AND notified_at IS NULL` semantics as today). A failed chunk is logged and left pending — mirroring `NotifyPending`'s existing per-event Send-error handling (D-09: a later pass, here the next digest tick or a mode switch back to real-time, retries it) — rather than treating one bad chunk as a hard failure of the whole run.
   - After a fully successful (or partially successful — "at least one chunk sent") run, update `digest_last_sent_at` via the settings store, so the ticker's due-check advances even under partial delivery.

5. **`poller` package needs zero changes.** It still depends only on the existing `Notifier` interface (`NotifyPending(ctx, logger) error`) — the mode switch is entirely internal to `notifier.Notifier`, invisible to `poller.runCycle`. This preserves `poller.go`'s own documented boundary ("this package still performs no diffing... it only calls the seam") and avoids widening `poller`'s dependency surface for a concern (delivery mode) it has no reason to know about.

This gives exactly one embed-building code path (`formatEmbed`, unchanged), exactly one Discord-payload-construction code path (`sendAttempt`, widened not duplicated), and exactly one outbox-draining query pair (`listUnnotified`/`markNotified`, reused verbatim) — real-time and digest differ only in *how many embeds go in one Discord request* and *whether the caller is the poll cycle or the digest ticker*.

## Data Flow

### Real-time mode (today, and the default under v1.5)

```
poll cycle (MB or Deezer) completes
    ↓
runCycle → p.notifier.NotifyPending(ctx, logger)
    ↓
NotifyPending: SettingsReader.DigestEnabled → false
    ↓
listUnnotified → for each event: suppresses? → formatEmbed → sender.Send (1 embed/request)
    ↓
markNotified (per event, spaced defaultSpacing apart)
```

### Digest mode (new, opt-in)

```
poll cycle (MB or Deezer) completes
    ↓
runCycle → p.notifier.NotifyPending(ctx, logger)
    ↓
NotifyPending: SettingsReader.DigestEnabled → true → return nil (no dequeue, no send)
    ↓                                                        (events accumulate,
    ↓                                                         notified_at stays NULL)
digest ticker (independent goroutine, every DIGEST_CHECK_INTERVAL)
    ↓
read notification_settings fresh → enabled? cadence? due since last_sent_at?
    ↓ (due)
CAS guard → Notifier.SendDigest(ctx, logger)
    ↓
listUnnotified → suppresses? filter → formatEmbed (same fn as real-time) per event
    ↓
chunk into ≤10-embed groups → sender.SendBatch per chunk (spaced)
    ↓
markNotified per successfully-sent event → settings.Store.Update(digest_last_sent_at = now)
```

### SPA settings change (no restart required)

```
Operator toggles digest ON / picks "weekly" in SPA
    ↓
PUT /settings/digest {enabled, cadence}  (protected route, behind authgate when active)
    ↓
httpserver handler → settings.Store.Update → notification_settings row updated
    ↓
Next poll-cycle-end NotifyPending call reads the new value (within ≤ one poll interval)
Next digest-ticker tick reads the new value (within ≤ DIGEST_CHECK_INTERVAL)
    ↓
No process restart, no cron re-registration, no cache to invalidate
```

## Scaling Considerations

This is a single-operator, single-instance self-hosted tool (explicitly out of scope: multi-user, horizontal scale) — scaling considerations here are about **data volume within one instance**, not concurrent users.

| Scale | Approach |
|-------|----------|
| Small watchlist, low event volume (the actual target use case) | As designed above: single-row settings read per tick, single Discord message per digest run (events fit in ≤10 embeds) |
| A digest accumulates more than 10 events between sends (e.g. a long outage, or weekly cadence over an active watchlist) | Already handled by the chunking design — multiple Discord messages, spaced, in one `SendDigest` run; no redesign needed |
| Digest check interval tension | A shorter `DIGEST_CHECK_INTERVAL` gives tighter "UI change takes effect" latency and tighter wall-clock-hour accuracy, at the cost of more low-cost PK-indexed reads; 5 min is a reasonable default matching the existing `authgate` sweep-interval order of magnitude |

No path here requires a second Postgres connection pool, a message queue, or a distributed lock — `robfig/cron`'s own documented limitation (no leader election, relevant only if the project ever runs multiple instances of one poller) is explicitly listed as an existing, already-accepted constraint in STACK.md and is unchanged by this milestone, since the digest scheduler is exactly as single-instance-only as the existing poll cycles.

## Anti-Patterns to Avoid

### Anti-Pattern 1: A second, independent "digest queue" table

**What people do:** Introduce a `digest_queue` table that mirrors/duplicates rows out of `events` for batching purposes.
**Why it's wrong:** The milestone's own constraint is "uses only the existing `events` table as its data source." `notified_at IS NULL` already *is* a queue — duplicating it invites the exact dedup/consistency bugs the existing outbox design (D-06, D-09, D-20) was built to avoid, and doubles the surface `internal/db/migrations/README.md`'s N-1 safety rule has to reason about.
**Do this instead:** Reuse `ListUnnotified`/`MarkNotified` unchanged; digest mode is purely "who drains the outbox and how many embeds per request," not a different queue.

### Anti-Pattern 2: A fixed cron spec (`"0 9 * * *"`) per cadence, swapped via `cron.Remove`+`cron.AddFunc` on every settings change

**What people do:** Try to keep the "real" schedule inside `robfig/cron` and reprogram it live when the operator changes cadence in the SPA.
**Why it's wrong:** Couples the HTTP settings-update handler to cron-internals mutation (must find the right `cron.EntryID`, remove it, re-add it, handle the case where a tick was mid-flight during the swap), and still doesn't solve the "process was down at 09:00" resilience gap — a missed tick with `robfig/cron` is just gone.
**Do this instead:** A ticker that checks a persisted `digest_last_sent_at` against `now` every few minutes — the "schedule" lives in the database, not in cron's in-memory entry table, so an HTTP update is just an ordinary row update with no coupling to the scheduler's internals at all.

### Anti-Pattern 3: Caching settings in-process at boot or on first read

**What people do:** Load `notification_settings` once into a `Config`-shaped struct at startup (matching the existing env-var `config.Config` pattern) for performance.
**Why it's wrong:** Directly defeats the milestone's explicit requirement ("changeable without a redeploy") — env-var config is deliberately boot-time-only in this codebase (that's the whole reason the milestone calls out digest as different from `authgate`'s env-var gate), and a cache reintroduces the exact "SPA change didn't take effect" bug class the milestone exists to avoid.
**Do this instead:** Read-through on every check, as above — the query is cheap and infrequent enough that a cache buys nothing but risk.

## Integration Points

### External Services

| Service | Integration Pattern | Notes |
|---------|---------------------|-------|
| Discord webhook | Existing `discord.Client`, extended with `SendBatch` | Discord's execute-webhook route accepts up to 10 embeds per request and a ~6000-character total-embed-content budget per message — `SendDigest`'s chunking must respect the 10-embed cap; the existing per-field `fieldValueLimit`/`titleLimit` truncation already bounds individual embed size, so the main new constraint is chunk *count*, not per-embed size |

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `poller` ↔ `notifier` | Existing `Notifier` interface (`NotifyPending`), unchanged signature | Digest mode is invisible to `poller`; only `notifier`'s internals branch on mode |
| `internal/digest` ↔ `notifier` | New: calls `Notifier.SendDigest(ctx, logger)` directly (not through the `poller.Notifier`/`Sink` interface, since the digest scheduler is a different caller with a different trigger) | Mirrors how `httpserver.handleStatus` calls `pollruns.Store` directly rather than through `poller`'s own seam — a second, independent consumer of the same underlying type is already an established pattern in this codebase |
| `internal/digest` ↔ `internal/settings` | New: reads `Settings` every tick | Narrow interface, consumer-declared, matching D-11 |
| `notifier` ↔ `internal/settings` | New: `SettingsReader` seam, consumer-declared | Same D-11 convention |
| `internal/httpserver` ↔ `internal/settings` | New: `GET/PUT /settings/digest` handlers call `settings.Store` directly | Registered inside the existing protected route group (same group as `/watchlist`, `/events`) so it is gated by `authgate` exactly like every other mutating endpoint when the passphrase gate is active, and ungated identically when it's not (GATE-07's inert-path guarantee extends automatically — no new gate logic needed) |
| `internal/discord` ↔ `notifier` | Widened `Sender`-shaped interface (`Send` + `SendBatch`) | `*discord.Client` already implements both once `SendBatch` is added; no second client type needed |

## Suggested Build Order

Matches the project's own established sequencing convention (additive migration → Go seams bottom-up → HTTP surface → SPA → composition-root wiring last), and mirrors how Phase 18/18.1/19 sequenced `pollruns` (land the seam inert, then wire it, then give it a UI):

1. **Migration**: `notification_settings` table (additive, singleton-row, `CHECK (id = 1)`) + sqlc queries (`GetNotificationSettings`, `UpdateNotificationSettings`). No dependents yet blocked; can be reviewed in isolation against the N-1 expand/contract rule.
2. **`internal/settings` package**: typed `Settings` struct + `Store.Get`/`Store.Update` wrapping the sqlc queries from step 1. Small, mirrors `internal/pollruns`'s "wrap sqlc, expose a narrow struct" shape.
3. **`discord.Client.SendBatch`**: generalize `sendAttempt`'s signature, add the public method. Independent of steps 1–2; can build in parallel.
4. **`internal/notifier` changes**: `SettingsReader` seam + `WithSettingsReader` option + the mode-check guard in `NotifyPending` + the new `SendDigest` method. Depends on step 2 (interface shape) and step 3 (`BatchSender`/`SendBatch`).
5. **`internal/digest` package**: ticker scheduler (mirroring `authgate.sweepLoop`) wrapping `Notifier.SendDigest`, reading `settings.Store` each tick for the due-check. Depends on steps 2 and 4. Land it wired but perhaps gated similarly to how `pollruns`' seam landed inert first, if the team wants to split "scheduler exists" from "scheduler is live" into separate reviewable changes.
6. **`internal/httpserver` routes**: `GET/PUT /settings/digest` inside the existing protected group. Depends on step 2 only — can be built in parallel with steps 3–5.
7. **SPA settings UI**: toggle + cadence control calling the new endpoints. Depends on step 6.
8. **`cmd/server/main.go` composition-root wiring**: construct `settings.Store`, pass it into `notifier.Select`/`New` via the new option, construct and `Start()` the `internal/digest` scheduler alongside the existing `poller.Start()`/`Stop()` lifecycle (same `signal.NotifyContext`-bounded shutdown pattern already used for the poller). Final integration step — depends on everything above.

## Sources

- `C:/CodeProjects/drop-tracker/.planning/PROJECT.md` — milestone scope, locked architecture constraints, ADR-0001 precedent, N-1 migration rule — confidence HIGH (project's own source of truth)
- `internal/notifier/notifier.go`, `internal/notifier/format.go` — existing real-time delivery loop, outbox drain, embed-building — confidence HIGH (read directly)
- `internal/discord/client.go` — existing webhook client, single-embed send path, 429 handling — confidence HIGH (read directly)
- `internal/poller/poller.go` — existing `robfig/cron` registration pattern, CAS overlap guard (D-08, D-09), consumer-declared seam convention (D-11), `RunRecorder` no-op-default idiom — confidence HIGH (read directly)
- `internal/pollruns/pollruns.go` — precedent for a small typed store wrapping mutable instance state, and ADR-0001's table-vs-in-process reasoning — confidence HIGH (read directly)
- `internal/authgate/gate.go` — `time.Ticker`-driven background-goroutine precedent (`sweepLoop`/`Close`), instance-wide-toggle precedent (env-var-only, contrasted against this milestone's DB-configurable requirement) — confidence HIGH (read directly)
- `queries/events.sql`, `internal/db/migrations/000003_events.up.sql`, `000004_events_display_fields.up.sql` — outbox schema, `ListUnnotified`/`MarkNotified` query shape, dedup/idempotency constraints (D-20) — confidence HIGH (read directly)
- `internal/config/config.go` — env-var naming/default convention (`env:"..." envDefault:"..."`) referenced for the proposed `DIGEST_CHECK_INTERVAL` — confidence HIGH (read directly)
- Discord webhook embed limits (10 embeds/message, ~6000-char total budget) — confidence MEDIUM (general Discord API documentation knowledge, not re-verified live against Discord's docs in this research pass — worth a quick confirmation during phase planning/discuss, same caveat the project's own `05-RESEARCH.md` already flagged for embed limits)

---
*Architecture research for: drop-tracker v1.5 Digest Notifications*
*Researched: 2026-09-11*
