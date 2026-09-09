# Pitfalls Research

**Domain:** Operator observability (readiness probe + poll-cycle run records + status API + status UI) bolted onto a shipped Go single-binary poller
**Researched:** 2026-09-09
**Confidence:** HIGH for the codebase-specific mechanics (read directly from `internal/poller/poller.go`, `internal/db/migrate.go`, `internal/httpserver/server.go`, `cmd/migration-check/main.go`, `cmd/server/main.go`); MEDIUM for the golang-migrate `schema_migrations` semantics and Postgres prune concurrency (corroborated against upstream docs + pgsql-general threads).

Cost ranking of the critical pitfalls (highest first): **#1 `runCycle` counter aggregation**, **#2 prune-on-insert concurrency**, then #3–#4 (RunRecorder call-site/failure semantics), then #5–#7 (`/ready` correctness, `/status` leakage), then #8 (`events_recorded` seam), then the Phase 19 UI items #9–#11.

---

## Critical Pitfalls

### Pitfall 1: Data race aggregating `artists_errored` / `artists_checked` / `events_recorded` across the worker goroutines — and `-race` can't catch it here

**What goes wrong:**
`runCycle` (poller.go:270) fans every watchlist entry out across `go func(entry)` closures. Today those closures share only `sem` (channel) and `wg` (WaitGroup) — both already concurrency-safe — plus a `recover()` per worker. To record a per-cycle row you must count how many artists were checked, how many errored, and how many events were recorded, and those counts are produced *inside* `fetchAndRecord`, which runs on N worker goroutines at once (`p.mbWorkers` default 3, `p.dzWorkers` default 5). A plain `errored++` or a shared `[]string` append inside `fetchAndRecord` is an unsynchronized read-modify-write from multiple goroutines: a textbook data race that corrupts the count and is undefined behaviour.

**Why it happens:**
The obvious implementation — close over an `int` in `runCycle` and bump it in the `fetchAndRecord` closure — compiles, passes every existing test, and passes a new non-race test because the race window is tiny and the counts are usually right. The project's safety net for exactly this (`go test -race`) is **unavailable**: ThreadSanitizer fails to allocate under WSL2 on the dev machine (PROJECT.md Context, `.planning/WINDOWS.md`). CI does not run `-race` either. So the normal "the race detector will catch it" backstop is gone.

**How to avoid:**
- Aggregate with `sync/atomic` (the package is *already imported* in poller.go for `nextCycleID` and the `atomic.Bool` guards). Use `atomic.Int64` for `checked`, `errored`, `recorded`; increment inside the worker; read once after `wg.Wait()`.
- Or (cleaner, no shared mutable state) have each worker send a small result value on a buffered channel sized to `len(entries)`, close it after `wg.Wait()`, and fold the totals single-threaded in `runCycle`. This mirrors the "workers return, parent aggregates" shape and is trivially correct without `-race`.
- The panic path must also count: the worker `recover()` block (poller.go:337) must increment `errored` (a panicked artist is a failed artist), otherwise `checked` and `errored+ok` disagree whenever a worker panics.
- Add a non-race invariant test: run a cycle with a fake source where K of M artists error and P panic, assert `errored == K+P` and `checked == M` exactly, repeated ~1000× in a loop to shake out ordering bugs the race detector would otherwise have found.
- Reason about it explicitly in the plan's "concurrency correctness" note (the project's stated substitute for `-race`).

**Warning signs:**
Counts that don't reconcile (`checked != errored + succeeded`); test assertions on counts that are `>=`/`<=` instead of `==`; flaky count values between test runs; a `[]string` or `map` being appended to inside `fetchAndRecord`.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 2: Prune-on-insert races between the two sources — wrong scoping deletes the other source's history, or the two DELETEs deadlock

**What goes wrong:**
`poller.New` registers **two** cron entries on the *same* `@every <interval>` spec (poller.go:209–223), so the MusicBrainz and Deezer cycles start — and finish — at essentially the same instant every tick. Each cycle then does `INSERT INTO poll_runs ...` followed by "prune to last N rows per source." Failure modes:
- **Missing `source` scope:** `DELETE FROM poll_runs WHERE id NOT IN (SELECT id FROM poll_runs ORDER BY started_at DESC LIMIT N)` keeps N rows *total*, so whichever source prunes second deletes the other source's rows down to nothing. The prune predicate must be `WHERE source = $1 AND ...`.
- **`NOT IN (SELECT ...)` + NULL:** if any selected `id` is NULL the whole `NOT IN` goes false and the prune deletes nothing (silent unbounded growth) or everything, depending on shape. Use `NOT EXISTS` or a keyset cutoff, not `NOT IN`.
- **Cross-source deadlock:** two concurrent `DELETE`s on `poll_runs` from the two cycles take row/page locks; if they touch overlapping rows in different orders Postgres kills one with `deadlock detected`. Since the write is best-effort (see Pitfall 3) you'd lose that row silently.
- **Prune bug deletes too much:** `OFFSET N` vs `LIMIT N`, `ASC` vs `DESC`, or an off-by-one keeps N-1 or 0 rows. On a table that only ever holds ~50–100 rows per source this is easy to get subtly wrong and invisible until someone opens the System view and sees two entries.

**Why it happens:**
"Retention by pruning to last N rows per source on insert (no new env var)" (PROJECT.md) reads as one line of SQL. The two-independent-cron-entries design (D-08) means the author testing one cycle in isolation never sees the concurrent-prune interaction.

**How to avoid:**
- Do the insert and the prune as **one atomic statement** via a CTE: `WITH ins AS (INSERT INTO poll_runs (...) VALUES (...) RETURNING id) DELETE FROM poll_runs WHERE source = $1 AND started_at < (SELECT started_at FROM poll_runs WHERE source = $1 ORDER BY started_at DESC, id DESC OFFSET $N_minus_1 LIMIT 1)`. One statement = one implicit transaction, prune failure fails the insert (which is logged), no separate round-trip.
- **Every** prune predicate scoped by `source`. Add a test that inserts M rows for `musicbrainz` and M for `deezer`, prunes each, and asserts both sources retain exactly N — the cross-source-deletion bug fails this immediately.
- Prefer a keyset cutoff (`started_at < <Nth newest>`) over `id NOT IN (SELECT ... LIMIT N)`; it locks fewer rows and sidesteps the NULL trap.
- Keep it in a sqlc query (`queries/pollruns.sql`), not a Postgres trigger / plpgsql function — a trigger drags `CREATE FUNCTION ... $$ ... $$` into a boot migration (dollar-quoting the stdlib `cmd/migration-check` tokenizer has to handle) and moves prune logic somewhere no Go test covers.
- **Prune-on-insert is the right call here, not a periodic sweep.** Volume is ~2 inserts per `POLL_INTERVAL` (default 15m) ≈ 192 rows/day total, table capped at N per source. A periodic sweep would need its own scheduler entry and a DB touch-point, and the poller is deliberately DB-connection-free (poller.go package doc, D-05) — a sweep reintroduces exactly what the `RunRecorder` seam exists to avoid.

**Warning signs:**
`deadlock detected` in logs around cycle-completion timestamps; System view history shorter than N right after both cycles run; `SELECT count(*) FROM poll_runs` climbing past `2*N`; a prune query with no `source =` in its `WHERE`.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 3: A `RunRecorder` write that fails or hangs turns a green poll cycle red — or delays it — or is skipped at shutdown

**What goes wrong:**
The recorder is called at the end of `runCycle`. Three ways to get it wrong:
- **Propagating the error:** returning the recorder's error from `runCycle` makes a successful poll cycle (artists checked, events detected, notifications sent) report as *failed* just because the observability row didn't persist. The cron closure would then log `"musicbrainz poll cycle failed"` (poller.go:211) for a cycle that did its job.
- **Blocking the overlap guard:** `runCycle`'s `defer running.Store(false)` (poller.go:282) releases the per-source guard. If the recorder call sits before that runs and blocks on a hung DB (a TCP-ESTABLISHED-but-unanswering Postgres — the exact failure `internal/db/pool.go` documents), the `running` flag stays `true`, every subsequent tick for that source logs `"skipping poll cycle: previous cycle still in progress"` and the source silently stops polling for the life of the process.
- **Skipped at shutdown:** if the recorder call uses the cycle's `ctx`, and shutdown cancelled it (`pollr.Stop` → `runCancel()` in main.go), the write is skipped and the final cycle before shutdown leaves no trace — or worse, blocks `Stop`'s bounded drain (`pollDrainTimeout` 10s) and makes the container slow to die.

**Why it happens:**
The existing seams give mixed signals: `EventRecorder` errors *are* logged-not-returned (poller.go:434), `notifier.NotifyPending` errors *are* logged-not-returned (poller.go:394) — but both are called with the cycle `ctx`. Copying that pattern verbatim inherits the shutdown-skip problem.

**How to avoid:**
- Model `RunRecorder` on the `Notifier` seam: a narrow consumer-declared interface in `internal/poller`, one method, returns `error`, and `runCycle` **logs and swallows** that error exactly like `NotifyPending` (`logger.Error("record poll run failed", ...)`), never returns it.
- Give the recorder call its **own bounded context**, derived so shutdown cancellation doesn't nuke it but a hang still can't wedge anything: `recCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)`. This records the last cycle even during graceful shutdown, and caps the delay to the overlap guard / drain at 5s.
- Put the recorder call *before* `defer running.Store(false)` in source order (so it runs while still "in progress", which is fine) but ensure its bounded context guarantees it returns promptly.
- Test: recorder returns an error → `runCycle` still returns `nil` and logs. Recorder blocks 30s → `runCycle` returns within ~5s.

**Warning signs:**
Poll-cycle error rate rises after wiring the recorder; container shutdown latency regresses; a source stops emitting `"poll cycle complete"` lines after a DB blip; the recorder call passes `ctx` straight through.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 4: Recording a row for a cycle that the overlap guard skipped

**What goes wrong:**
`runCycle` returns `ErrCycleInProgress` at the CAS check (poller.go:278–281) — *before* `cycleStart` is set, before the watchlist is read, before any work. If the recorder is wired in the wrong place (a `defer` at the very top of `RunMusicBrainzCycle`, or in the cron closure that receives the error), every skipped tick writes a junk `poll_runs` row: zero duration, zero artists, an ambiguous outcome. During one slow 20-minute cycle you'd get a burst of bogus rows and the prune would evict the *real* history to make room for them.

**Why it happens:**
`ErrCycleInProgress` is a normal, expected control-flow value (the cron closures explicitly `errors.Is(err, ErrCycleInProgress)` and stay quiet — poller.go:210). It's easy to forget it's also a path through whatever wraps the cycle.

**How to avoid:**
- Instrument *inside* `runCycle`, after `running.CompareAndSwap` succeeds. Capture `startedAt` right after the CAS, and record in a `defer` that runs on every exit path from that point (normal, error, panic) — so a skipped cycle, which returns before the defer is registered, is never recorded.
- The recorded `outcome` must distinguish: `ok` (completed, no artist errors), `partial` (completed, ≥1 artist errored), `cancelled` (`cycleErr != nil` from context cancellation — poller.go:375–377), `error` (watchlist `List` failed — poller.go:289). Never record `ErrCycleInProgress` as an outcome; it's the *absence* of a run.

**Warning signs:**
`poll_runs` row count grows faster than 2 per `POLL_INTERVAL`; rows where `started_at == finished_at` or `artists_checked = 0` on a non-empty watchlist; a "skipped" outcome value appearing in the table.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 5: `/ready` requires the schema version to *equal* the binary's expected version — so an intentionally-rolled-back instance reports not-ready forever and a future deploy gate flap-loops

**What goes wrong:**
"Returns 200 only when the DB is reachable and the schema is at the expected migration version" (PROJECT.md) invites a strict equality check: `db.schema_migrations.version == <max embedded migration> AND NOT dirty`. That is wrong for the rollback scenario Phase 16 deliberately supports:
- Deploy `v_new` → it applies migration 8. Roll back to `v_old` (embedded max = 7). Phase 16's ahead-of-source guard (migrate.go:298–302) lets `v_old` **boot cleanly** against schema version 8 — that's the whole N-1 guarantee, and the expand/contract rule (migrations README) means schema 8 is additive-only so `v_old` runs fine against it.
- But a strict-equality `/ready` on `v_old` sees `db.version (8) > binary.expectedMax (7)` → **503 forever**. The instance is up, serving traffic correctly, `/health` green — and `/ready` says not-ready.
- When Phase 17's health-gated auto-rollback is un-deferred, it polls `/ready` after a deploy. A rolled-back-and-healthy instance that reports 503 makes the gate conclude the rollback failed and… roll back again / never converge. The probe built to make deploys safe makes them loop.

**Why it happens:**
"At the expected version" is naturally read as `==`. The interaction with Phase 16's intentional ahead-of-source tolerance is non-obvious and lives in a different subsystem.

**How to avoid:**
- Ready condition: **`NOT dirty AND db.version >= binary.expectedMax`**. Equal-or-newer additive schema = ready (that is precisely the N-1 invariant the migrations README states). Only `db.version < expectedMax` (migrations genuinely not yet applied) or `dirty = true` is not-ready.
- Get `expectedMax` from the embedded source, not a hand-typed constant that drifts: expose a `db.ExpectedSchemaVersion()` that reuses the existing `maxSourceVersion` walk (migrate.go:326) over `migrationsFS`.
- Read `schema_migrations` via one sqlc query (`SELECT version, dirty FROM schema_migrations`) on the shared pool — it's a single-row table golang-migrate maintains (`version bigint`, `dirty boolean`).
- Document the `>=` choice and its Phase-16 rationale in the handler comment, the way `health.go` documents its own contract.

**Warning signs:**
`/ready` returns 503 on an instance whose `/health` is 200 and whose logs show `"ahead-of-source"` no-op at boot; an equality (`==`) comparison on the version; `expectedMax` written as a literal.

**Phase to address:** Phase 18 (backend). Flag for the (deferred) Phase 17: the deploy gate consumer must debounce — require consecutive successes and tolerate transient 503s, because a single DB blip through `/ready` should never trigger a production rollback.

---

### Pitfall 6: `/ready` ends up auth-gated, CSRF-wrapped, expensive, or flapping

**What goes wrong:**
- **Gated:** if `/ready` is registered inside the `r.Group` that `gate.Authenticate` + `RequireCSRFHeader` protect (server.go:172–179), a gated production instance returns 401 to every uptime monitor and to Phase 17's deploy gate (neither sends a passphrase). `/health` is exempt because it's registered as an exact path on the root router *before* the group (server.go:164, D-03) — `/ready` must be registered the same way, in **both** the gated and inert branches.
- **Too expensive:** implementing the schema check via `migrate.NewWithInstance(...).Version()` opens a *fresh* `database/sql` connection every hit (that's what `runMigrationsOnce` does — migrate.go:272). Under a monitor polling every few seconds plus a deploy gate, that's needless connection churn. Use the shared `pgxpool` via a plain sqlc query.
- **Unbounded:** no timeout on the DB call means a hung network path hangs the probe and stacks goroutines — the exact reasoning behind `healthPingTimeout` (health.go:13). Bound `/ready`'s DB work with the same 3s.
- **Flapping / over-sensitive:** adding ret/hysteresis logic *inside* `/ready` (e.g. "503 only after 3 consecutive bad checks") makes it lie about the current instant and hides real problems. Keep `/ready` a pure point-in-time check; debouncing belongs in the consumer (Phase 17).
- **Deploy consequence of a wrong probe:** false-ready → Phase 17 shifts traffic to a broken release and never rolls back = outage. False-not-ready → deploy never completes / rollback loop.

**Why it happens:**
The gate refactor (Phase 14) makes "which router does this route go on" a real decision with an inert branch and a gated branch; it's easy to add the route in one place. And `internal/db` already *has* a version-reading path (`m.Version()`), so reusing it looks DRY.

**How to avoid:**
- Register `/ready` exactly like `/health`: `r.Get("/ready", s.handleReady)` on the root router, outside `if gate != nil`, so it's identical in both branches. Add a test asserting `/ready` returns non-401 on a gated server with no cookie.
- New `Server` dependency for the schema query (a narrow `SchemaVersioner` interface mirroring `Pinger`), or widen the existing DB seam minimally — don't reach into `internal/db`'s migrate path.
- `context.WithTimeout(r.Context(), 3*time.Second)` around the query; on error/timeout return 503 with a fixed body, log the raw error to `httplog.SetAttrs` (health.go:38 pattern).
- Keep the handler branchless and stateless.

**Warning signs:**
`/ready` in the `registerDataRoutes` set or inside the `r.Group`; a 401 from `/ready` in a gated-instance test; `sql.Open` / `migrate.New*` in the ready path; no `context.WithTimeout`.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 7: `/status` leaks a DSN, the Discord webhook URL, or an internal path through an error string in the JSON

**What goes wrong:**
`poll_runs` naturally grows a "what went wrong" field. If the cycle stores and `/status` returns a free-text error/detail string, it can carry:
- a Postgres DSN **with password** — pgx / driver connection errors embed it verbatim (the entire reason `redactDSN`/`redactError` exist — migrate.go:114–194, `internal/db/pool.go` `redactedTarget`);
- the `DISCORD_WEBHOOK_URL` (a secret) — notifier errors can include the request URL;
- internal filesystem paths / stack fragments.
`/status` is gated, but a gated instance with a weak passphrase (the D-11 WARN path — main.go:124) still must not hand out credentials, and "operator observability" is not a reason to relax the Phase 1 redaction decision.

**Why it happens:**
The existing redaction helpers (`redactDSN`, `redactError`) are **unexported** in `internal/db` — a new `internal/pollruns` or the `internal/httpserver` status handler cannot call them. The path of least resistance is to `err.Error()` into a string column and echo it.

**How to avoid:**
- **Preferred: store no free-text error at all.** `poll_runs` carries counts (`artists_checked`, `artists_errored`, `events_recorded`) and an `outcome` enum (`ok` / `partial` / `cancelled` / `error`). Per-artist error *detail* already goes to the structured log with the `cycle_id` correlation attribute (poller.go:285, 420) — that's where an operator debugging a specific failure looks. The System view shows counts + outcome + a "view logs" hint.
- If a message is genuinely required: promote redaction to an exported shared package (`internal/redact` with `Error(err) string` / `DSN(string) string`), move `internal/db`'s two helpers behind it, run **every** string through it at the write boundary, and add a golden test in the spirit of `TestRunMigrations_NeverLogsDSN` (asserted at migrate.go / redact_test.go) that feeds a DSN-bearing and webhook-bearing error through the status path and asserts neither survives.
- `/status`'s own DB-failure path mirrors every other handler: log raw error to `httplog.SetAttrs`, return the fixed `"internal error"` body (events.go:124–127), never raw driver text.
- `poll interval` and `watchlist size` are non-sensitive — fine to return as-is.

**Warning signs:**
Any `error`, `message`, `detail`, `last_error` string field in the `/status` response type; `err.Error()` written into a `poll_runs` column; no redaction test covering the status path; the string `://` or `password=` appearing in a `/status` fixture.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 8: `events_recorded` can't be counted without giving the poller a DB connection or widening the `EventRecorder` seam

**What goes wrong:**
The poller does no diffing and holds no DB connection *by design* (poller.go package doc, D-05). Detection happens behind `EventRecorder.DetectMusicBrainz` / `DetectDeezer`, which return **only `error`** (poller.go:86–89). So `runCycle` has no idea how many event rows a cycle produced. Getting `events_recorded` means either:
- widening `EventRecorder` to return `(int, error)` — touches the interface, `internal/detection`'s implementation, and every test fake in `internal/poller` and `internal/httpserver`; or
- having the `RunRecorder` count rows in the `events` table — which hands the poller (or its seam) a DB read it's architecturally not supposed to have; or
- omitting / approximating `events_recorded`.

**Why it happens:**
"events recorded" is listed alongside "artists checked / errored" as if all three are equally available to the poller. Two of them are; this one isn't.

**How to avoid:**
- Decide explicitly at plan time. Cleanest that preserves the DB-free poller: widen the two `EventRecorder` methods to return `(int, error)` — the count flows back through the same seam the poller already depends on, no new DB access, and the aggregation is subject to Pitfall 1 (use atomics / channel fold).
- If that ripple is judged too large for this milestone, record `events_recorded` as nullable and populate it later, or drop it from the row and surface "new events this cycle" in the UI from the existing `/events` feed instead. Do not give the poller a `sqlc.Queries`.

**Warning signs:**
`events_recorded` hard-coded to 0; a `sqlc.New(pool)` appearing in `poller.New`'s call site for the recorder; the `EventRecorder` fakes gaining a DB.

**Phase to address:** Phase 18 (backend).

---

### Pitfall 9: The System view polls `/status` far faster than the thing it's observing

**What goes wrong:**
A status panel invites `setInterval(fetchStatus, 5000)`. Each poll is a gated request that runs several DB queries (last run per source, run history, `SELECT count(*)` on the watchlist). The poll cycle it's reporting on runs every **15 minutes** by default. Polling every 5s is ~180× more backend/DB load than the subsystem under observation, and every open System tab adds a steady connection-pool draw (`MaxConns` is sized for poll workers + headroom — `internal/db/pool.go`, not for UI polling).

**Why it happens:**
"Live status panel" reads as "real-time." The rest of this SPA has **no polling anywhere** — `history.tsx` and `watchlist.tsx` fetch once on mount (history.tsx:98) — so there's no existing pattern to copy and the author invents one.

**How to avoid:**
- Fetch once on mount (the established pattern) + an explicit "Refresh" button.
- If auto-refresh is wanted, no faster than 30–60s, and **pause when the tab is hidden** (`document.visibilityState` / `visibilitychange`), resuming (with an immediate fetch) on re-show.
- Clear the interval in the `useEffect` cleanup (history.tsx's `cancelled` flag is the reference pattern).

**Warning signs:**
Network tab shows steady `/status` traffic with the System tab focused; pool acquisition waits climb when someone leaves the tab open; a sub-10s interval literal; no `visibilitychange` handling.

**Phase to address:** Phase 19 (UI).

---

### Pitfall 10: Empty state before the first poll cycle — the normal state for the first 15 minutes of every deploy — renders as "Loading…" forever or a broken date

**What goes wrong:**
A freshly deployed (or freshly migrated) instance has an empty `poll_runs` table. `/status` returns `last run per source = null`, empty history. A UI that assumes a run always exists renders a spinner that never resolves, the string "Invalid Date" from `new Date(null)`, or a blank card. This is not an edge case — it's every deploy's first ~`POLL_INTERVAL`, and every instance with an empty watchlist (poll cycles run but do nothing).

**Why it happens:**
Development always has poll history within minutes; the "never run yet" window is easy to never see locally.

**How to avoid:**
- Explicit first-run empty state, distinct from an error state and from a loaded-but-empty state — mirror `history.tsx`'s three-way `emptyStateCopy` (history.tsx:40) and the `EmptyState` component. Copy like: "No poll cycle has run yet — the first is scheduled within your poll interval."
- Handle each source independently (one may have a run while the other doesn't — though both cron entries fire together, a source's first cycle can still be mid-flight).
- Guard every timestamp render against `null`.
- Handle watchlist size 0 explicitly ("Add an artist to start tracking").

**Warning signs:**
`new Date(status.lastRun)` with no null check; one loading boolean covering both "fetching" and "no data"; no dedicated first-run copy; QA only ever run against a populated dev DB.

**Phase to address:** Phase 19 (UI).

---

### Pitfall 11: Auth expiry while the System view is polling — stale interval keeps firing 401s after the passphrase screen mounts

**What goes wrong:**
`apiFetch`'s global 401 interceptor calls `authStore.markUnauthenticated()` (api.ts:155), which makes `<App>` early-return `<PassphraseScreen>` (root.tsx:105) and unmount the routed content. If the System route started a `setInterval` and doesn't clear it on unmount, the interval keeps firing `/status` fetches behind the login screen, each returning 401, each re-calling `markUnauthenticated` — harmless but noisy, and it holds a fetch in flight against a gated server on every tick.

**Why it happens:**
`setInterval` in a component without a matching `clearInterval` in the effect cleanup is one of the most common React mistakes, and this SPA has no prior polling code to have established the discipline.

**How to avoid:**
- `useEffect` that returns `() => clearInterval(id)`; never start a second interval without clearing the first (guard on filter/dependency changes).
- Rely on the existing 401 → `PassphraseScreen` swap for re-auth; after login, `markAuthenticated` remounts `<Outlet/>` and the System route's mount effect re-fetches (root.tsx:97–100 documents this as the whole re-fetch mechanism). No retry queue needed.
- A poll firing during the login race is fine — the 401 just re-asserts unauthenticated.

**Warning signs:**
Repeated 401s in the console after logout/expiry; `/status` requests continuing with the passphrase screen visible; a `setInterval` with no `clearInterval` in the same effect.

**Phase to address:** Phase 19 (UI).

---

## The `poll_runs` migration vs the CI guards (confirmed safe, with conditions)

**`cmd/migration-check` — a pure `CREATE TABLE` passes.** `scanFile` (migration-check main.go:560) only switches on `sqlscan.DropTable` and `sqlscan.AlterTable`; a `CREATE TABLE` statement produces zero findings, and `CREATE INDEX` isn't matched either. The D-15 previous-release cross-reference only fires on `DROP`/`RENAME` of an object the N-1 release's `queries/*.sql` still touches — irrelevant for a brand-new table. Conditions to keep it clean:
- Put any `CHECK` constraint (e.g. on the `outcome` enum) **inline in the `CREATE TABLE`**, not as a later `ALTER TABLE ... ADD CHECK` — `classifyAction` flags `AddCheck` as `backward-incompatible` (migration-check main.go:592), and there's no reason to split it since the table is new.
- No `CREATE INDEX CONCURRENTLY` — golang-migrate wraps each file in a transaction and `CONCURRENTLY` can't run inside one (migrations README checklist). A brand-new empty table has no rows to lock, so a plain `CREATE INDEX` in the same file is fine.
- No trigger / `CREATE FUNCTION ... $$ ... $$` for the prune (see Pitfall 2) — keep prune logic in a sqlc query.
- Ship the `.down.sql` (`DROP TABLE poll_runs`) as the pair even though the app never runs `Down()`; `migration-check` skips `*.down.sql` files (main.go:57).
- Number it `000008_poll_runs.up.sql`, strictly ascending; never renumber or edit a released migration (`runChangedFiles` hard-errors on a modified released migration — main.go:309).

**N-1 boot is safe.** A bare additive `CREATE TABLE poll_runs` that the previous release's binary never references satisfies the N-1 invariant automatically — the old binary boots against the new schema, ignores the new table, reads/writes everything else unchanged. The `n1-boot` job exercises this. The guard-adoption skip-green window (migrations README) is orthogonal and self-clearing.

**Not caught by CI — the local-only `sqlc` gate.** Adding `queries/pollruns.sql` requires `sqlc generate` + committing the generated code; `make sqlc-check` (CLAUDE.md Definition of Done, no CI counterpart) is the only thing that catches drift. Easy to forget because CI stays green.

**Phase to address:** Phase 18 (backend).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Plain `int` counters in the worker closures, "we'll test it works" | One less concept than atomics | Data race with no `-race` safety net; corrupted counts that look plausible; the bug ships | **Never** — atomics are already imported; the cost is zero |
| `err.Error()` into a `poll_runs.last_error` text column | Rich detail in the System view | DSN/webhook/path leakage through a gated-but-not-secret endpoint; needs the unexported redaction helpers promoted anyway | **Never** — use counts + `outcome` enum; detail lives in the correlated logs |
| Prune with `id NOT IN (SELECT id ... LIMIT N)`, no `source` scope | Shortest SQL | Cross-source history deletion; NULL trap; extra locks feeding cross-source deadlock | **Never** — scope by source, use a keyset cutoff |
| `/ready` = strict `version == expectedMax` | "Obviously correct" | Rolled-back (intentionally-behind) instance reports not-ready forever; Phase 17 flap-loop | **Never** — use `>=` and document the Phase 16 rationale |
| `events_recorded` hard-coded to 0 for now | Avoids widening `EventRecorder` this milestone | A visible metric that's always wrong; erodes trust in the whole panel | Only if the field is omitted from the UI too, not shown as "0" |
| `setInterval` polling `/status`, no visibility gating | "Live" panel with 3 lines of code | 180× load multiplier vs the observed subsystem; pool pressure per open tab; stale-interval 401 spam | MVP only if interval ≥ 60s and cleared on unmount; visibility gating is cheap, do it |
| Reading `schema_migrations` by spinning up `migrate.NewWithInstance().Version()` | Reuses existing code | Fresh `database/sql` connection per `/ready` hit | **Never** — one sqlc query on the shared pool |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| golang-migrate `schema_migrations` | Assuming multiple rows / a history; parsing `version` as a string | Single-row table: `version bigint`, `dirty boolean`. `SELECT version, dirty FROM schema_migrations`. `ErrNilVersion` equivalent = table empty / no row (fresh DB) → not ready |
| Phase 16 ahead-of-source guard | `/ready` treating "DB newer than binary" as an error | It's the supported rollback state (migrate.go:298). `db.version >= expectedMax && !dirty` = ready |
| chi gate routing (Phase 14) | Registering `/ready` once, inside or outside the gate group | Register on the root router in **both** branches, exact path, before `r.NotFound` — mirror `/health` (server.go:164) |
| `EventRecorder` seam | Expecting it to report event counts | Returns `error` only; widen to `(int, error)` if `events_recorded` is required |
| `notifier.NotifyPending` pattern | Copying "log, don't return" *including* passing the cycle `ctx` | Log-don't-return is right; the `ctx` is not — use `context.WithoutCancel` + timeout for the recorder write |
| sqlc | Adding `queries/pollruns.sql` and relying on CI to catch drift | `make sqlc-check` is local-only; regenerate and commit before pushing |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| `/status` polled every few seconds by the UI | Steady gated-request + DB traffic; pool acquisition waits | Fetch-on-mount + Refresh button; ≥60s interval; pause on hidden tab | Immediately with one open tab; worse per concurrent operator |
| Two concurrent per-source `DELETE` prunes on `poll_runs` | `deadlock detected` in logs at cycle-completion times | Insert+prune as one CTE statement; keyset cutoff; source-scoped | Every tick once both sources have >N rows |
| `/status` "run history" as a query per source in a loop | N queries where 1 would do | Single `LIMIT`-bounded query, clamp page size in the domain layer (events.go pattern) | Trivial now; still wrong shape |
| `/ready` opening a fresh DB connection per hit | Connection churn under monitor + deploy-gate polling | sqlc query on the shared `pgxpool` | Under Phase 17's frequent polling |
| `poll_runs` unbounded growth if a swallowed prune keeps failing | `count(*)` >> `2*N`; slow System view | Atomic insert+prune (prune failure fails the logged insert); sanity assertion in a test | Slowly — ~192 rows/day, tiny rows, but no monitoring |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Free-text error string in `poll_runs` → `/status` JSON | DSN-with-password / Discord webhook URL / internal path exfiltrated through a gated-but-not-secret endpoint | Store counts + `outcome` enum only; detail to correlated logs; if a message is unavoidable, promote `redactDSN`/`redactError` to an exported `internal/redact` and golden-test the status path |
| `/ready` or `/status` returning raw driver error text on DB failure | Same leakage class as above, via the error path | Log raw to `httplog.SetAttrs`, return fixed `"internal error"` / a bare 503 body (health.go / events.go pattern) |
| `/ready` registered behind the gate | Not a leak, but breaks uptime monitoring and the future deploy gate — pushing operators toward disabling the gate | Root-router exact path, both branches |
| Assuming "gated" means "safe to expose secrets to" | A weak passphrase (D-11 WARN path) still gets in | Redaction is unconditional, independent of the gate |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| No first-run empty state | Operator sees a spinner or "Invalid Date" for 15 min after every deploy and thinks it's broken | Dedicated "no cycle has run yet" copy, distinct from error and from loaded-empty (history.tsx three-way pattern) |
| `artists_checked` counting all watchlist entries for Deezer | Deezer always shows the same count as MusicBrainz even when most entries have no `deezer_id` (skipped pre-dispatch, poller.go:466) | Count dispatched artists (post-`shouldDispatch`); optionally a separate `artists_skipped` |
| Showing `events_recorded: 0` when the number is actually unknown | Operator distrusts the panel | Omit the field until it's real, or wire the count through the seam |
| Status panel that looks stale with no "as of" timestamp | Operator can't tell if the panel itself is live | Show "last refreshed" and the cycle `finished_at`, both explicitly |
| Outcome shown as a raw enum (`partial`) | Ambiguous | Human phrasing: "Completed with 2 artist errors" |

## "Looks Done But Isn't" Checklist

- [ ] **`/ready`:** often missing the `>=` (not `==`) version comparison — verify a binary with a *lower* embedded max than the DB still reports ready (simulate rollback)
- [ ] **`/ready`:** often missing the gated-instance test — verify it returns non-401 with no session cookie on a server built with `WithAuthGate`
- [ ] **`/ready`:** often missing a DB-call timeout — verify a blocked ping fails the probe within ~3s, doesn't hang
- [ ] **`runCycle` counters:** often missing `==` (exact) assertions — verify `checked == errored + succeeded` under a fake with errors *and* panics, looped 1000×
- [ ] **Worker panic:** often missing from the errored count — verify a panicking artist increments `artists_errored`
- [ ] **RunRecorder:** often missing the "swallow the error" path — verify a recorder returning an error leaves `runCycle` returning `nil`
- [ ] **RunRecorder:** often missing the shutdown case — verify the last cycle before `Stop()` still writes a row; verify a 30s-hanging recorder doesn't extend shutdown past ~5s
- [ ] **Overlap-guard skip:** verify an `ErrCycleInProgress` tick writes **no** `poll_runs` row
- [ ] **Prune:** often missing source scoping — verify inserting >N rows for each source and pruning leaves exactly N *per source*
- [ ] **Prune:** verify insert+prune is atomic (one statement) so a prune failure surfaces
- [ ] **`/status` redaction:** verify a DSN-bearing and webhook-bearing error fed through the status path appears nowhere in the response
- [ ] **Migration:** `migration-check` green, `n1-boot` green, `make sqlc-check` green locally, `.down.sql` present, `CHECK` inline in `CREATE TABLE`
- [ ] **System view:** verify the fetch interval (if any) is cleared on unmount and paused on `visibilitychange`
- [ ] **System view:** verify null `last_run` timestamps render as first-run copy, not "Invalid Date"

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Counter data race shipped | MEDIUM | Switch to `atomic.Int64` or channel-fold; add looped exact-assertion test; the historical rows are just slightly wrong, no data loss |
| Prune deleted the other source's history | MEDIUM | History is gone (not recoverable), but fix is a one-line `WHERE source =` + test; re-accumulates within N cycles |
| Cross-source prune deadlock | LOW | Collapse insert+prune into one CTE statement; deadlock disappears |
| `/ready` strict-equality shipped, deploy gate loops | LOW (before Phase 17) / HIGH (after) | Change `==` to `>=`; before Phase 17 there's no consumer so it's harmless; after, a bad deploy could have flapped |
| `/status` leaked a secret | HIGH | Rotate the exposed credential (DB password / Discord webhook), then remove the field + add the golden test |
| RunRecorder wedged the overlap guard | MEDIUM | Bound the recorder context; until fixed, a restart clears the stuck `running` flag |
| `poll_runs` grew unbounded | LOW | One-off `DELETE` down to N per source; fix the atomic prune |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| #1 Counter aggregation data race | Phase 18 | Looped exact-count test with errors + panics; atomics/channel fold in code review |
| #2 Prune-on-insert concurrency | Phase 18 | Per-source retention test; single-statement CTE; no `deadlock detected` in a two-source stress test |
| #3 RunRecorder failure/hang semantics | Phase 18 | Recorder-error → cycle nil; hanging-recorder → bounded; last-cycle-at-shutdown recorded |
| #4 Recording an overlap-skipped cycle | Phase 18 | `ErrCycleInProgress` tick writes no row; row count == 2 per interval |
| #5 `/ready` version `>=` not `==` | Phase 18 | Simulated-rollback binary reports ready; `ExpectedSchemaVersion` derived from embedded FS |
| #6 `/ready` gating / cost / timeout | Phase 18 | Non-401 on gated server; shared-pool query; 3s timeout test |
| #7 `/status` secret leakage | Phase 18 | Golden redaction test on the status path; no free-text error field |
| #8 `events_recorded` seam | Phase 18 | Decision recorded; if kept, `EventRecorder` returns `(int, error)` and poller stays DB-free |
| #9 Over-aggressive `/status` polling | Phase 19 | Interval ≥60s + `visibilitychange` pause, or fetch-on-mount + Refresh |
| #10 First-run empty state | Phase 19 | Three-way empty/error/first-run copy; null-timestamp guards |
| #11 Auth-expiry / stale interval | Phase 19 | `clearInterval` in effect cleanup; 401 → PassphraseScreen swap re-fetches on re-auth |
| `poll_runs` migration vs CI guards | Phase 18 | `migration-check` + `n1-boot` + `make sqlc-check` all green; inline CHECK; `.down.sql` present |
| `artists_checked` semantics (Deezer skip) | Phase 18 | Deezer count reflects dispatched artists, not `len(entries)` |

## Sources

- Codebase (authoritative for all mechanics): `internal/poller/poller.go`, `internal/db/migrate.go`, `internal/db/pool.go`, `internal/httpserver/server.go`, `internal/httpserver/health.go`, `internal/httpserver/events.go`, `cmd/server/main.go`, `cmd/migration-check/main.go`, `internal/config/config.go`, `internal/db/migrations/README.md`, `internal/watchlist/service.go`, `web/app/lib/api.ts`, `web/app/lib/authStore.ts`, `web/app/root.tsx`, `web/app/routes/history.tsx` — read 2026-09-09
- `.planning/PROJECT.md` (v1.4 milestone scope, Phase 16 decisions D-16/D-17, `-race` unavailability), `.planning/WINDOWS.md` (Broken Windows Ledger — ThreadSanitizer waiver)
- [golang-migrate/migrate pkg.go.dev](https://pkg.go.dev/github.com/golang-migrate/migrate/v4) and [Better Stack: Database migrations in Go with golang-migrate](https://betterstack.com/community/guides/scaling-go/golang-migrate/) — `schema_migrations` table shape (`version` bigint, `dirty` boolean), `Version()` semantics, `ErrNilVersion` — MEDIUM
- [Checking migration status with golang-migrate — Jamie Tanna](https://www.jvt.me/posts/2023/06/19/golang-migrate-status/) — reading applied version — MEDIUM
- [pgsql-general: "How to keep at-most N rows per group?"](https://www.postgresql.org/message-id/20080109182115.6E39C2E3239%40postgresql.org) and [Nicola Iarocci: Automatic deletion of older records in Postgres](https://nicolaiarocci.com/automatic-deletion-of-older-records-in-postgres/) — trigger vs periodic-sweep tradeoffs, prune-on-insert lock overhead — MEDIUM

---
*Pitfalls research for: operator observability features on drop-tracker v1.4*
*Researched: 2026-09-09*
