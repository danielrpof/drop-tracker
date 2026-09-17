# Roadmap: drop-tracker

## Overview

drop-tracker was built outward from the data layer: Postgres schema + config + health-checked skeleton, then a tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications, then the embedded React UI, then single-image containerization and the full GitHub Actions CI/CD pipeline that is the actual point of the project. v1.1–v1.2 hardened it, v1.3 delivered the deployment-readiness chain that needs no host, and v1.4 made the running service legible (`/ready`, an in-process poll-run ring buffer, a gated `/status`, and a System view).

v1.5 changes *how* the app talks, not *what* it detects. Today every detected event fires its own Discord message the moment the poll cycle notices it. v1.5 adds an instance-wide, Postgres-persisted digest mode: flip it on, and everything accumulated since the last digest arrives as one scheduled daily-or-weekly message instead. Real-time stays the default and the unchanged path. No new polling, no new external traffic, no new third-party dependency — the existing `events` table's `notified_at IS NULL` outbox is the queue, and the only question the milestone answers is who drains it and how many embeds go out at once.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- ✅ **v1.3 Continuous Deployment** — Phases 14-17 (shipped partial 2026-09-09; **Phase 17 deferred**)
- ✅ **v1.4 Operator Observability** — Phases 18, 18.1, 19 (shipped 2026-09-11)
- 🔄 **v1.5 Digest Notifications** — Phases 20-23 (in progress)

Full phase-by-phase detail for every shipped milestone is archived under `.planning/milestones/v[X.Y]-ROADMAP.md`. Requirement archives: `.planning/milestones/v[X.Y]-REQUIREMENTS.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

**Deferred:** **Phase 17 — Automated VPS Deploy with Health-Gated Rollback** (DPLY-01…08). Blocked on a provisioned VPS + domain the developer does not have yet. `discuss-phase` context was already gathered — archived at `.planning/milestones/v1.3-phases/17-automated-vps-deploy-with-health-gated-rollback/` (`17-CONTEXT.md`, `17-DISCUSSION-LOG.md`). Un-defer it as its own milestone cycle once a box exists; the `/ready` probe from v1.4 is built for its health-gate to consume.

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] Phase 1: Foundation — Data Layer, Config & Health
- [x] Phase 2: Watchlist Core
- [x] Phase 3: External Clients & Search
- [x] Phase 4: Detection Engine
- [x] Phase 5: Discord Notifications
- [x] Phase 6: Frontend & Release History
- [x] Phase 7: Containerization & CI/CD Pipeline

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] Phase 8: Frontend Test Suite
- [x] Phase 9: CI Coverage Gates
- [x] Phase 10: Event Retention Window
- [x] Phase 11: Bounded Concurrent Polling
- [x] Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)

</details>

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking
- [x] Phase 13: Fix History Dates, Guest-Feature Art & Artist Art

</details>

<details>
<summary>✅ v1.3 Continuous Deployment (Phases 14-17) — SHIPPED PARTIAL 2026-09-09</summary>

- [x] Phase 14: Instance Passphrase Gate (7/7 plans) — completed 2026-09-01
- [x] Phase 15: PR Coverage-Diff Comment (3/3 plans) — completed 2026-09-03
- [x] Phase 16: Rollback-Safe Migrations (5/5 plans) — completed 2026-09-05
- [ ] Phase 17: Automated VPS Deploy with Health-Gated Rollback — **DEFERRED** (no VPS); context archived, carries forward as its own milestone

</details>

<details>
<summary>✅ v1.4 Operator Observability (Phases 18, 18.1, 19) — SHIPPED 2026-09-11</summary>

- [x] Phase 18: Backend — Readiness, Status Surface & App Version (4/4 plans) — completed 2026-09-09
- [x] Phase 18.1: Poll-Cycle Instrumentation (3/3 plans) — completed 2026-09-11
- [x] Phase 19: Frontend — System View (5/5 plans) — completed 2026-09-11

</details>

### 🔄 v1.5 Digest Notifications (Phases 20-23) — IN PROGRESS

- [x] **Phase 20: Digest Settings & Operator Control** - Postgres-backed instance setting (on/off + daily/weekly), gated `GET`/`PUT` routes, and the SPA panel that drives it — notification behavior itself unchanged (completed 2026-09-13)
- [x] **Phase 21: Real-Time ↔ Digest Mutual Exclusion** - digest mode makes the real-time notify pass stand down; events queue instead of firing, and toggling back off flushes them (completed 2026-09-16)
- [ ] **Phase 22: Scheduled Digest Send** - the digest scheduler and batched, grouped send: cadence fire slots, grace-window catch-up, DST/tzdata correctness, the slot record and watermark, and the event-grouping hierarchy (by type, then watched artist)
- [ ] **Phase 23: Digest Readability & Discord Limits** - the "since <timestamp>" window header and multi-message chunking that never truncates, preserving Phase 22's grouping across the split

> **Ordering rationale.** Phase 21 (the gate) lands *before* Phase 22 (the sender) deliberately. With the real-time drain un-gated, a poll cycle empties the `notified_at IS NULL` outbox every interval, so a digest would always find zero events and silently skip — the sender is not verifiable until the gate exists. Landing the gate first also makes the in-between state safe by that phase's own success criteria: digest on means events stay pending and nothing is lost. Same posture as v1.3's Phase 16 (build the safety precondition before the thing it protects).
>
> **Deploy sequencing (locked during a post-roadmap grilling session, 2026-09-11).** Phase 21 and Phase 22 ship in the same release — Phase 21 is not merged to `main` (and therefore not auto-deployed, per this project's continuous-deploy pipeline) until Phase 22 is also ready. Landing Phase 21 alone would put a gate into production with no sender behind it: an operator flipping digest mode on would make real-time notifications stand down with nothing yet built to deliver the pending events, going dark with no ETA until Phase 22 lands. This is a release-sequencing rule, not a plan/dependency change — Phase 22 still depends on Phase 21 exactly as before.
>
> **Extended to Phase 23 (grilling session on the Phase 22 plan, 2026-09-16).** Phases 21, 22, and 23 ship in one release. Phase 22's digest is a single embed whose Description is capped at 4096 characters (URLs count, so roughly 30–35 lines). Shipped without Phase 23's chunking, an oversized digest is rejected by Discord, never acked, and retried on every due-check, with each later digest larger than the last. Holding the release until chunking lands closes that wedge without writing interim cap code that Phase 23 would replace.

## Phase Details

### Phase 20: Digest Settings & Operator Control

**Goal**: An operator can turn digest mode on and pick daily or weekly from inside the app, and that choice sticks across restarts — while notification behavior stays exactly what v1.4 shipped.
**Depends on**: Nothing (builds on shipped v1.4 code)
**Requirements**: DGST-01, DGST-02, DGST-03, DGST-04, DGST-16
**Success Criteria** (what must be TRUE):

  1. A digest panel in the SPA shows the current mode (on/off), the current cadence (daily/weekly), and the last digest send time — rendered as explicit "never sent yet" copy on an instance that has never sent one — and an operator can change mode and cadence from that panel and see the new values after a reload.
  2. The setting lives in Postgres, not in an env var and not in process memory: changing it takes effect without a rebuild, redeploy, or restart, and the same values are still there after the container is restarted.
  3. A fresh install with migrations applied and nothing touched reports digest **off** with a default cadence, and every notification path behaves exactly as it did in v1.4 — one Discord message per detected event, same spacing, same mark-notified ack.
  4. The digest settings routes sit behind the existing instance gate: without a session they answer `401` like every other data route, and a `PUT` carrying an unrecognised cadence or malformed body is rejected with a 4xx instead of persisting a value the scheduler would later have to interpret.
  5. There is exactly one settings row and no code path can create a second one — a read on a brand-new database returns defaults rather than "not found", and concurrent writes cannot fork the instance's configuration.

**Plans**: 4/4 plans executed

Plans:
**Wave 1**

- [x] 20-01-PLAN.md — Tracer: singleton `notification_settings` row, `internal/settings.Store`, gated `GET`/`PUT /settings/notifications` end to end

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 20-02-PLAN.md — HTTP contract hardening: rejection paths, gate 401, CSRF refusal, unconfigured 503, no-leak
- [x] 20-03-PLAN.md — Tracer: `DigestSettings` card on `/system` with the instant-apply digest-mode toggle and keep-stale failure posture

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 20-04-PLAN.md — Cadence control, last-sent row, vendored `select`, embedded SPA bundle rebuild

**UI hint**: yes

**Notes for the phase planner**

- **Run `/gsd-ui-phase 20` first** — this phase touches the SPA and needs its own UI-SPEC before planning.
- *Schema.* One additive migration, next free number is `000008` (the `poll_runs` table sketched during v1.4 was rejected in favour of an in-process ring buffer — see `docs/adr/0001` — so nothing occupies 000008). Research's shape: `notification_settings` singleton with `digest_enabled BOOLEAN NOT NULL DEFAULT false`, `digest_cadence TEXT NOT NULL DEFAULT 'daily' CHECK (digest_cadence IN ('daily','weekly'))`, `digest_last_sent_at TIMESTAMPTZ NULL`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`. `digest_last_sent_at` ships here even though nothing writes it until Phase 22 — the panel needs the column to render "never sent yet", and one migration beats two. **Singleton enforcement is locked** (ratified during a post-roadmap grilling session, 2026-09-11, over research's own schema sketch in ARCHITECTURE.md): a `CHECK (id = 1)` single-row constraint plus a migration-time seed `INSERT INTO notification_settings (id) VALUES (1)` — not upsert-on-read. This needs no app-level "ensure a row exists" race handling and matches the project's existing inline-CHECK convention.
- *Migration safety is enforced by CI, not by memory.* Read `internal/db/migrations/README.md` before writing it — `cmd/migration-check` reds `DROP`/`RENAME`/type-narrowing/`ADD COLUMN NOT NULL`, and `n1-boot` boots the previous release against this schema. A bare `CREATE TABLE` with inline CHECKs plus a paired `.down.sql` produces zero findings and satisfies N-1 automatically (the N-1 binary never queries the new table). `make sqlc-check` has **no CI counterpart** — regenerate and commit the `queries/notification_settings.sql` codegen locally or CI stays green over a drifted tree.
- *Go side.* `internal/settings.Store` wrapping the sqlc queries, shaped like `internal/pollruns.Store` (narrow `Get`/`Update` seam, no cache). No caching layer: a single PK-indexed row read per notify pass is cheap, and it is what makes DGST-01's "no restart" property true by construction rather than by invalidation logic.
- *HTTP.* Register inside `registerDataRoutes` (`internal/httpserver/server.go:248`) so the routes inherit `gate.Authenticate`, the `X-Instance-Gated` marker, and the 401 contract for free. A `PUT`/`PATCH` is state-changing, so it also inherits `authgate.RequireCSRFHeader` — the SPA's `apiFetch` already injects `X-Requested-With: drop-tracker` on every non-GET, so no client change is needed for that, but the planner should assert it.
- *SPA.* The natural home is the existing `/system` view (`web/app/routes/system.tsx`), which already fetches a gated JSON endpoint on mount, already has the five-state render machine, and already carries an About block — DGST-16 explicitly allows either that view or a new panel. Reuse `web/app/lib/format.ts`'s timestamp formatters for the last-sent stamp; type the new wire shape in `web/app/lib/api.ts` against the real Go response body, per the existing discipline. Phase 22 will make the last-sent field non-null; the empty state has to be built here regardless.
- *Definition of Done:* `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging, then `corepack pnpm test`. Hand-formatted TSX fails CI's `frontend-test` job.

### Phase 21: Real-Time ↔ Digest Mutual Exclusion

**Goal**: Turning digest mode on makes the real-time notifier stand down cleanly — events queue instead of firing — and turning it back off delivers everything that queued, nothing lost, nothing duplicated.
**Depends on**: Phase 20 (the settings row and `settings.Store` are what the notifier reads)
**Requirements**: DGST-13, DGST-14
**Success Criteria** (what must be TRUE):

  1. With digest mode on, a poll cycle that detects new releases, guest features, and deluxe/tracklist changes sends **zero** Discord messages, and every one of those events is still pending afterwards (`notified_at IS NULL`) rather than marked delivered.
  2. Toggling digest back off delivers everything that accumulated while digest mode was on through the ordinary real-time path on the next poll cycle — one message per event, in the existing order and spacing, none dropped and none sent twice.
  3. The mode is re-read on every notify pass, so flipping the toggle changes behavior on the next poll cycle of a running process — no restart, no cached boolean that survives the change.
  4. With digest mode off, the notify path is indistinguishable from v1.4: same message per event, same 400ms inter-send spacing, same idempotent `MarkNotified` ack, same error handling on a failed send.
  5. The mode check is the notify pass's **first** decision once it holds the sender lock, and it is repeated before every Discord send. It is not a post-hoc filter: there is no code path on which an event is sent in real time *and* left pending for a later digest, and a settings read that fails skips the pass, leaving events pending, never delivering them in real time while digest mode may be on.

**Plans**: 3/3 plans executed

Plans:
**Wave 1**

- [x] 21-01-PLAN.md — Tracer: `SettingsReader` seam, required constructor argument, top-of-pass digest gate, fail-closed bounded read, composition-root wiring
- [x] 21-03-PLAN.md — SPA helper text under the Digest mode row, plus the committed-bundle refresh

**Wave 2** *(blocked on 21-01)*

- [x] 21-02-PLAN.md — Per-send mode re-read, mode-transition logging, and the `created_at`-anchored stale-release cutoff

**Notes for the phase planner**

- *This is the milestone's riskiest behavioral change* (RESEARCH Pitfall 1: un-gated real-time drain alongside digest). Isolate it. The change is a digest-mode read at the top of `Notifier.NotifyPending` (`internal/notifier/notifier.go:155`), taken after acquiring the `notifying` lock and before `listUnnotified`, then re-read before each Discord send, through a `SettingsReader` that `notifier.New`/`notifier.Select` take as a required argument, plus the wiring at the composition root. `internal/poller` should not need to change at all — the mode switch is invisible to it, which is the property that keeps this phase small.
- *Keep exactly one outbox.* Do **not** add a second "digest queue" table or a `digest_pending` flag. DGST-14 is correct for free when there is one queue: toggling off means the next real-time pass finds the pending rows and sends them individually. A second queue is what would make toggle-off lose or re-batch events. This also means "the digest window" is defined by outbox state, never by a wall-clock `created_at BETWEEN` predicate (Pitfall 4).
- *Failure posture for the settings read: fail CLOSED* (post-grilling revision, 2026-09-16, reversing the 2026-09-11 lock). A read error, at the top of the pass or before a send, stops the pass, logs a distinct Warn (not on ctx cancellation), and returns nil, so `poller.go`'s log-and-continue call site doesn't add a second Error that reads like a delivery failure. In digest mode the pending set is the next digest: failing open would flush and ack a whole digest on one transient error, while skipping a pass loses nothing because the outbox persists. Every settings read is bounded by `dbOpTimeout`.
- *Existing guards to preserve.* `MarkNotified` is idempotent via `AND notified_at IS NULL` (D-09), but that prevents a double ack, not a double send: the Discord POST comes first and the rows-affected count is discarded. The exclusion mechanism is the `notifying` `atomic.Bool` CAS guard (D-06), which becomes the one sender lock that every outbox send serializes on (real-time now, the Phase 22 digest send later; see `docs/adr/0002-one-outbox-one-sender-lock.md`). Whichever send finds it held skips, which is safe because the outbox persists. Residual: at most the one message already in flight when digest mode is turned on. Assert both guards rather than reworking them.
- *Staleness is anchored to detection time.* `suppresses()` computes its cutoff from `ev.CreatedAt` minus `maxAgeDays` minus 1 day (UTC) rather than from `time.Now()`, so time spent pending never ages an event out. The 1-day slack absorbs the gap between detection's captured `now` and the DB's `created_at`. Pre-fix backlog rows are still suppressed. `events.created_at` already exists, so there's no schema change.
- *Testing note.* Prove the toggle interleavings with deterministic tests, not looped timing invariants: a fake `SettingsReader`/`Sender` that flips mode (or errors) on the k-th call, with exact send and ack counts asserted. Real-settings tests build the reader from `testutil.NewIsolatedTestPool`, never the shared pool (`internal/httpserver/settings_test.go` flips the singleton row, and packages run in parallel). CI does run `-race` (`make test-integration`, full-pipeline.yml:54 / Makefile:76, ubuntu-latest). Only this Windows dev box can't, and a toggle mid-pass is a logical interleaving `-race` wouldn't catch anyway.
- *Operator-facing copy.* Add always-visible helper text under the Digest mode row in `web/app/components/system/DigestSettings.tsx`, reading exactly: "While on, new events wait for the next digest; switching back off delivers them individually." The web Definition of Done applies (`prettier --write`, `corepack pnpm test`).

### Phase 22: Scheduled Digest Send

**Goal**: With digest mode on, everything accumulated since the last digest arrives as one Discord message at a predictable time — and the schedule survives restarts, DST transitions, and the container's minimal timezone database.
**Depends on**: Phase 21 (the real-time path must stand down, or the digest never has anything to send)
**Requirements**: DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15
**Success Criteria** (what must be TRUE):

  1. With digest mode on and events of all three types pending, one Discord message arrives at the next scheduled fire carrying all of them grouped under headings (by event type and/or artist) rather than as an undifferentiated flat list, and every event that message covered is marked delivered afterwards.
  2. A digest window that accumulated zero events sends nothing at all — no empty message, no placeholder embed — and the skipped send is visible in the structured logs rather than being indistinguishable from a scheduler that stopped running.
  3. The process being down at the exact fire time does not lose that digest: after a restart past a missed fire that is still within its grace window (12h daily, 48h weekly), the next check detects the gap and sends what was owed instead of waiting out a full cadence; past the grace window no late digest is sent, a warning is logged, and the pending events go out at the next scheduled fire.
  4. Across both a spring-forward and a fall-back transition, exactly one digest is sent per calendar day (daily) or per week (weekly) — no skip, no double-send — and the shipped Alpine image resolves the zone it schedules against rather than silently falling back to UTC.
  5. The slot record (last handled scheduled fire) decides whether a digest is due and outbox state decides what it carries: a late tick, a duplicated tick, a skipped tick, and a zero-event fire followed by newly detected events later that day all converge on one digest per scheduled fire, with none dropped and none sent twice. Turning digest mode on, or changing cadence, never triggers an immediate send.

**Plans**: 3/4 plans executed

Plans:
**Wave 1**

- [x] 22-01-PLAN.md — Migration 000009's slot-record column, the D-14 re-anchor and D-16 ack queries, `internal/settings` slot math, and `cmd/server`'s fail-fast zone resolution

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 22-02-PLAN.md — Tracer: a due slot turns the outbox into one Discord embed and one atomic ack, driven by a 5-minute `DigestScheduler` wired into the composition root

**Wave 3** *(blocked on Wave 2 completion)*

- [x] 22-03-PLAN.md — Digest body: markdown escaping and rune cap, collated sort by watched artist, guest host credit, deluxe track-count suffix

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 22-04-PLAN.md — DST and grace-window behavioural matrix, plus a `build-scan` step booting the just-built image to prove it resolves `America/New_York`

**Notes for the phase planner**

*Revised 2026-09-16 after a grilling session on this phase's plan. `.planning/phases/22-scheduled-digest-send/22-CONTEXT.md` (D-11–D-26) is the detailed source of truth; the notes below are updated to match it.*

- *Scheduling mechanism.* A `time.Ticker`-driven due-check goroutine (5m, first check immediately on start) rather than a third `robfig/cron` entry: a cron entry fires or it doesn't, whereas a due-check converges. Its lifecycle is modeled on `poller.Poller`'s `Start(ctx)`/`Stop(drainCtx)`, not `authgate.Manager.sweepLoop`, which has no ctx, no drain, and no injectable clock. `notifier.Sink` gains `SendDigestIfDue` (inert on `NoOp`), a `DigestScheduler` drives it with an injected clock, and `main.go` defers its drain before the pool closes. The digest send is a `Notifier` method and serializes on the same `notifying` lock as real-time delivery (`docs/adr/0002-one-outbox-one-sender-lock.md`). Whichever send finds it held skips, which is safe because the outbox persists.
- *Due rule: digest slots, not "watermark + cadence".* A digest slot is 00:05 America/New_York daily, or Friday 00:05 (covering through Thursday night) for weekly, computed with calendar math in that zone. A digest is due when digest mode is on, the slot record (`digest_last_slot_at`) is older than the most recent slot, and the slot is still within its grace window. Adding a duration to a send timestamp was rejected: a late catch-up would permanently shift the fire time, and `+24h` lands an hour off across DST. Slot math lives in `internal/settings`, which owns `Cadence`.
- *Three roles, kept distinct in code and tests.* **Outbox state** (`ListUnnotified`, no time predicate) decides *what* is sent; that is Pitfall 4's prevention, already true from Phase 21. The **slot record** (`digest_last_slot_at`, new nullable column in migration 000009) decides *whether a send is due*. It is written on a send and on an empty skip, so a zero-event 00:05 fire doesn't make the first event detected later that day trigger an off-schedule digest. The **watermark** (`digest_last_sent_at`, last successful send) stays the operator-facing timestamp (DGST-16) and supplies Phase 23's window label. Turning digest mode on, or changing cadence, re-anchors the slot record to the most recent slot inside `UpdateNotificationSettings`, so enabling never sends immediately.
- *Staleness.* Phase 21 already anchors the stale-release cutoff to each event's `created_at`, so a weekly cadence doesn't age pending events out before the digest fires.
- *Timezone scope is already settled — do not re-open it.* REQUIREMENTS.md Out of Scope excludes an operator-configurable time-of-day and IANA zone picker for v1.5: the fire time is fixed and documented. RESEARCH's gap #1 ("clarify with PO whether local fire time is in scope") is therefore closed as **no**. What still must happen: blank-import `time/tzdata` in `cmd/server` (Alpine ships no zoneinfo, DGST-07), and pass the zone explicitly wherever a fire time is computed rather than inheriting container-local time. The zone is loaded once at startup and a load failure exits the process, with no fallback to UTC anywhere. Success logs one line with the zone and its current offset. A new `build-scan` step boots the just-built image against a throwaway Postgres (reusing `n1-boot`'s pattern) and asserts that line. No existing job boots the *current* image, and a unit test can't prove Alpine behavior because Go consults host zoneinfo before embedded `time/tzdata`.
- *DST (DGST-06).* The 00:05 fire sits outside the 02:00 transition window, and slot math in the named zone handles 23h/25h days. The slot record is what prevents a double send. An empty outbox is not enough: a poll cycle can insert events between two would-be fires. Cover both transitions for both cadences with a fake clock, including an event inserted between the would-be fires. Not-due ticks log at Debug; every state-changing decision (due, sent, empty skip, grace expired, lock held) logs at Info/Warn, so an anomaly is observable at the default `LOG_LEVEL`.
- *Restart catch-up: bounded grace window* (revised 2026-09-16, **superseding** the 2026-09-11 "log-and-wait, no immediate catch-up send" lock). That lock contradicted success criterion 3 and the immediate first check. "Wait for the next natural check" meant either ≤5 minutes, which still sends at an arbitrary restart time, or the next slot, which leaves a weekly digest up to 7 days late. The new rule: a missed slot still within 12h (daily) or 48h (weekly) is sent on the next check, including the first check on boot. Past the window, a Warn is logged, nothing is written, and the pending events go out at the next slot. A brief outage still delivers; a day-long outage doesn't produce a surprise mid-afternoon digest. Outbox-state windowing loses nothing either way.
- *Batching and grouping* (DGST-08, and DGST-10 pulled forward from Phase 23, ratified 2026-09-11). One embed per digest, with bold per-type headings (New Releases / Guest Features / Deluxe Changes) and compact linked lines in its Description. `discord.Client.Send(ctx, Embed)` needs no interface change. Within each type, lines sort by watched artist using `x/text/collate` (IgnoreCase; tie-break title, then id). Guest-feature lines name both the watched artist and the host credit, because `artist_name` on those rows is the host. Deluxe lines keep the track-count suffix, omitted when unknown. Community-editable artist names and titles are markdown-escaped and title-capped, since `[Deluxe]` and `(Remix)` break masked links; record this in the threat model. Oversize handling is Phase 23's chunking, which ships in the same release (see "Deploy sequencing").
- *Send sequence and ack.* The send runs in this order. (1) CAS the lock. (2) Read settings, failing closed; stop if digest is off, the slot isn't due, or the grace window has expired. (3) List pending events and partition out `suppresses()` rejects. (4) If nothing is sendable, ack the suppressed ids and record the slot only. (5) Otherwise build the message and re-read settings immediately before the POST, aborting with no writes if digest was turned off. (6) POST; on a confirmed 2xx, ack. (7) On failure write nothing, so the next check retries within grace. The ack is **one data-modifying CTE statement** through the existing `sqlc.Querier`: it marks `id = ANY($ids) AND notified_at IS NULL` and updates the slot record (always) and the watermark (only if something was sent) atomically, under `context.WithoutCancel` bounded by `dbOpTimeout`, so a shutdown after the 2xx still acks instead of re-sending a whole batch. No production code opens a transaction today, and `WithTx` is only on concrete `*Queries`, so the single statement avoids new tx plumbing.

### Phase 23: Digest Readability & Discord Limits

**Goal**: A digest states the window it covers and stays complete — with Phase 22's grouping intact — even when it is large enough to exceed what one Discord message can hold.
**Depends on**: Phase 22 (there has to be a grouped digest before it can be window-stamped or chunked)
**Requirements**: DGST-11, DGST-12
**Success Criteria** (what must be TRUE):

  1. Every digest message states the window it covers ("since <timestamp>"), so a digest arriving after a late, skipped, or caught-up tick is unambiguous about what it includes.
  2. A digest large enough to exceed Discord's per-message embed count or character budget is delivered as multiple ordered messages, spaced by the existing inter-send delay — with no event silently dropped, no content truncated away without a visible marker, and Phase 22's grouping preserved across the split (a group is never silently broken across messages without a continuation marker).
  3. Events are acked per delivered message, not per digest run: a failure partway through a multi-message digest leaves the undelivered remainder pending for the next digest instead of losing it or re-sending what already went out.

**Plans**: TBD

**Notes for the phase planner**

- *Grouping hierarchy already exists* — locked and built in Phase 22 (RESEARCH gap #2 closed there; DGST-10 moved to Phase 22 during a post-roadmap grilling session, 2026-09-11, to avoid building a flat digest here only to discard it). This phase's job is to split a grouped digest across multiple messages without silently breaking a group across the boundary, not to invent the hierarchy.
- *Discord's real limits* are the constraint to encode explicitly as named constants with a source comment: 10 embeds per message, 25 fields per embed, and a ~6000-character total message budget. The existing `truncateRunes` helper already handles per-field truncation; what is new is the message-level budget and the split.
- *Chunking realism* (RESEARCH gap #3): with the operator's actual watchlist size, a >10-embed digest may be theoretical rather than routine. That does not make it optional — DGST-12 requires graceful degradation — but it does mean the bar is "provably correct under a synthetic large digest", not "tuned for throughput". A `+N more` marker is acceptable degradation only if it is logged as a warning and visible in the message.
- *No SPA work in this phase.* This is Discord message composition inside `internal/notifier` and `internal/discord`; the operator-facing surface is the message itself.
- *Built on Phase 22's slot/ack design* (added 2026-09-16). Ships in the same release as Phases 21 and 22. Per-message acking can reuse Phase 22's single-statement batch ack once per delivered chunk. Still open for this phase's own discussion: which chunk records the slot. Recording it only on the final chunk leaves a partially failed digest due, so the remainder retries within the grace window. The "since <timestamp>" label reads `digest_last_sent_at`, which is NULL for the first digest after digest mode is enabled (Phase 22 re-anchors the slot record, not last sent), so render that case explicitly. Chunk boundaries must not split a markdown-escaped line.

## Progress

- **v1.0–v1.2:** shipped.
- **v1.3:** shipped partial — 3 of 4 phases (14, 15, 16). Phase 17 deferred, DPLY-01…08 carried forward.
- **v1.4:** shipped — Phases 18, 18.1, 19.
- **v1.5:** in progress.

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 20. Digest Settings & Operator Control | 4/4 | Complete    | 2026-09-13 |
| 21. Real-Time ↔ Digest Mutual Exclusion | 3/3 | Complete    | 2026-09-16 |
| 22. Scheduled Digest Send | 3/4 | In Progress|  |
| 23. Digest Readability & Discord Limits | 0/? | Not started | - |

## Backlog

*(none currently)*
