# Phase 22: Scheduled Digest Send - Context

**Gathered:** 2026-09-16
**Revised:** 2026-09-16 — post-discussion grilling session (D-11–D-26 added; the ROADMAP "log-and-wait" catch-up lock superseded by D-12; D-10 corrected)
**Status:** Ready for planning

<domain>
## Phase Boundary

With digest mode on, everything accumulated in the outbox since the last digest arrives as one grouped Discord message at a predictable, fixed time — and that schedule survives restarts, DST transitions, and the shipped Alpine image's minimal timezone database.

Requirements: DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15 (`.planning/REQUIREMENTS.md`).

**In scope:**
- The digest due-check scheduler (a `time.Ticker`-driven goroutine with `Start`/`Stop` modeled on `internal/poller.Poller`, not a third `robfig/cron` entry) and the digest send itself as a `Notifier` method sharing the existing `notifying` sender lock
- Batching + grouping: one grouped Discord message built from the outbox
- The **slot record** (`digest_last_slot_at`, new migration 000009) that decides *whether a send is due*, kept distinct from the **watermark** (`digest_last_sent_at`, the last successful send: the displayed timestamp and Phase 23's window label) and from the outbox-state row selection (`ListUnnotified`) that decides *what* is sent
- Re-anchoring the slot record when digest mode is turned on or the cadence changes (a change to Phase 20's `UpdateNotificationSettings` query)
- A single-statement batch ack (events + both settings columns)
- Grace-window catch-up, DST correctness, Alpine `tzdata` resolution with a fail-fast boot check and a new `build-scan` CI step that boots the current image
- Markdown escaping of community-editable text in the digest body
- Zero-event digest slots sending nothing (silent skip, logged, slot still recorded)

**Not in this phase:**
- The digest-mode gate / real-time↔digest mutual exclusion — done, Phase 21
- The "since &lt;timestamp&gt;" window header text and multi-message chunking for an oversized digest — Phase 23 (which now ships in the same release, D-19)
- An operator-configurable fire time/timezone picker — explicitly out of scope per REQUIREMENTS.md; the fire time is a fixed, documented constant
- Any change to `internal/poller` — invisible to the mode switch, per Phase 21
- A second "digest queue" table or `digest_pending` flag — there is exactly one outbox (ADR-0002). `digest_last_slot_at` is schedule bookkeeping on the settings row, not a queue.
- Merging duplicate digest lines for the same release seen from two sources (D-26)

</domain>

<decisions>
## Implementation Decisions

### Carried forward from ROADMAP.md (still binding)

- Scheduling mechanism is a `time.Ticker`-driven due-check goroutine, not a third `robfig/cron` entry. (Its structural model is `Poller.Start`/`Stop`, not `authgate.Manager.sweepLoop` — see D-18.)
- The digest send is a `Notifier` method, serializing on the existing `notifying` `atomic.Bool` CAS lock (ADR-0002) — a send that finds the lock held skips, safely, because the outbox persists.
- The outbox (`ListUnnotified`, no time predicate) decides *what* to send. Timestamps decide only *whether* a send is due (the slot record, D-13) and what window label Phase 23 renders (the watermark). Keep these roles separate in code and tests.
- Events are acked only after a confirmed 2xx from Discord — a send failure leaves the batch pending.
- `time/tzdata` must be blank-imported in `cmd/server` (Alpine ships no zoneinfo).

**Superseded:** the ROADMAP lock "restart catch-up is log-and-wait, no immediate catch-up send" contradicted success criterion 3 and D-10. It is replaced by D-12's grace window.

### Fire Schedule

- **D-01: Fixed timezone is `America/New_York`.** The first pick during discussion was UTC, but that was flagged and reversed: ROADMAP's own success criterion #4 requires proving "the shipped Alpine image resolves the zone it schedules against rather than silently falling back to UTC" — a test that's unobservable if the scheduled zone literally is UTC (a silent fallback and a correct resolution look identical). DGST-06's DST-transition correctness would also be vacuously true under UTC (no transition exists to mishandle). `America/New_York` makes both criteria actually testable.
- **D-02 — Daily cadence fires at 00:05 America/New_York.** Chosen specifically to sit outside the ~02:00 local transition window on both the spring-forward and fall-back transitions — 00:05 is unaffected by either the skipped 02:00–03:00 hour or the repeated 01:00–01:59 hour.
- **D-03 — Weekly cadence fires on Friday, same 00:05 America/New_York time.**
- **D-11: The due rule is slot-based calendar math, never "watermark + cadence".** A **digest slot** is a scheduled fire instant: 00:05 America/New_York each day (daily) or each Friday (weekly), computed with `time.Date(..., loc)` so DST days of 23h/25h resolve correctly. A digest is due when **all** hold: digest mode is on; `digest_last_slot_at < MostRecentSlot(now, cadence, loc)`; and `now − slot ≤ grace(cadence)` (D-12). Adding a duration to a send timestamp is rejected: a late catch-up would permanently shift the fire time, and `+24h` in absolute time lands at 23:05/01:05 local across a DST transition.
- **D-12: Catch-up uses a grace window — 12h (daily), 48h (weekly).** A missed slot still inside its grace window is sent on the next due-check (including the immediate first check on boot, D-10). Past the window, the check logs a Warn and writes nothing; the slot simply stops being due and the pending events go out at the next slot — nothing is lost, because the outbox persists. Named constants with a one-line why-comment. Rationale: a brief outage still delivers promptly (criterion 3, DGST-05); a day-long outage doesn't produce a surprise mid-afternoon digest.
- **D-14: Turning digest mode on, or changing cadence while it is on, re-anchors the slot record** to `MostRecentSlot(now)` of the (new) cadence, so enabling never triggers an immediate send and a weekly→daily switch mid-week doesn't read today's slot as missed. Done atomically inside `UpdateNotificationSettings`: `digest_last_slot_at = CASE WHEN $enabled AND (NOT digest_enabled OR digest_cadence <> $cadence) THEN $slot ELSE digest_last_slot_at END` (the right-hand `digest_enabled`/`digest_cadence` are the old values). `settings.UpdateParams` and the HTTP contract are unchanged; `Service.Update` computes `$slot` itself.
- **D-15: Slot math lives in `internal/settings`** — `MostRecentSlot`, the grace constants, and the zone name — because `settings` already owns `Cadence` and both `settings` (D-14) and `notifier` (D-11) need it; a leaf package would force moving `Cadence`, and `notifier` can't host it (import cycle). `Service` gets an injectable clock for tests.
- **D-25 — Operator-facing copy for weekly reads "Friday 00:05 America/New_York, covering through Thursday night"** — in code comments and any docs/copy that state the fire time, since REQUIREMENTS.md requires the fixed time be documented.

### Slot Record & Ack

- **D-13: A new nullable `digest_last_slot_at TIMESTAMPTZ` column (migration 000009) is the slot record** — "the most recent slot handled" — written on a successful send *and* on an empty/suppressed-only skip. The watermark (`digest_last_sent_at`) keeps meaning "last successful send" (what the SPA already displays, DGST-16). One column cannot carry both meanings: advancing the watermark on an empty skip makes the SPA report sends that never happened, while not recording the empty slot makes the first event polled in later that day trigger an off-schedule digest. The slot column stores the Go-computed slot instant, not DB `now()`, so app/DB clock skew can't make a handled slot look due. Additive nullable column: clean under `cmd/migration-check` and N-1 boot (sqlc expands `SELECT *` at generate time, so v1.12.0's scan is unaffected). Read `internal/db/migrations/README.md` first.
- **D-16: The ack is one data-modifying CTE statement, not a Go transaction.** It acks `WHERE id = ANY($ids) AND notified_at IS NULL` and updates the singleton row's `digest_last_slot_at` (always) and `digest_last_sent_at` (only when something was actually sent — e.g. a nullable param with `COALESCE`) atomically. It runs through the existing `sqlc.Querier` seam — no production code opens a transaction today, and `WithTx` lives only on concrete `*Queries` — under `context.WithoutCancel(ctx)` bounded by `dbOpTimeout`, so a shutdown landing after Discord's 2xx still acks instead of re-sending a whole batch. The watermark write therefore goes through `Querier` directly, not through `settings.Service`. Phase 23 can reuse the same statement per delivered chunk.
- **D-17 — The digest send sequence is fixed:**
  1. CAS the `notifying` lock; held → skip (Info).
  2. Read settings (bounded by `dbOpTimeout`, fail closed as in Phase 21). Digest off, slot not due, or grace expired → return (grace expired logs Warn, writes nothing).
  3. `ListUnnotified`; partition out events `suppresses()` rejects.
  4. Nothing left to send → ack the suppressed ids and record the slot only (D-16), log the empty skip.
  5. Build the message; **re-read settings immediately before the POST** — digest now off → abort, write nothing (real-time flushes on its next pass).
  6. POST. 2xx → D-16 ack of sent + suppressed ids, both columns.
  7. Send failure → write nothing; the next due-check retries within the grace window, after which events carry to the next slot.

### Digest Message Layout

- **D-04: Top-level grouping is by event type, then artist.** Three fixed headings — New Releases / Guest Features / Deluxe Changes — each listing that type's artists underneath. Matches the existing `formatNewRelease`/`formatGuestFeature`/`formatDeluxeChange` split in `internal/notifier/format.go`.
- **D-05: Rendering shape is one embed, with bold headings inside its Description** — not multiple embeds, not a field-per-event. Shape: `**New Releases**\n- [Artist — Title](url)\n**Guest Features**\n...`. Note Description is capped at 4096 characters and URLs count toward it (~30–35 lines); the overflow is Phase 23's chunking, which is why D-19 ships them together. `discord.Client.Send(ctx, Embed)` needs no interface change.
- **D-06: Sort order within each event-type group is alphabetical by watched artist** (D-20), deterministic regardless of detection order.
- **D-22: Sorting uses `golang.org/x/text/collate` with `IgnoreCase`** (promote `x/text` from indirect to direct in `go.mod` — no new module), tie-broken by title then event id. A byte sort would put lowercase-stylized and accented names (`Ñengo Flow`) after Z.
- **D-07: Per-event line is `[Artist — Title](url)`, linked**, reusing the URL helpers `formatEmbed` already uses (`newReleaseURL`, `musicBrainzRecordingURL`, `musicBrainzReleaseURL`) — likely via a small per-event-type URL extraction from `formatEmbed`'s switch rather than duplicating it.
- **D-20: The artist key is `watched_artist_name`** (falling back to `artist_name` when NULL — the column is `*string`, though migration 000007 backfilled every row). **Guest-feature lines also name the host credit:** `[Rauw Alejandro on Drake — Title](url)`. For `guest_feature` rows `artist_name` is the host's primary credit, so keying on it would file the line under an artist the operator doesn't watch and hide which watched artist is on it. Matches the History UI's existing watched-artist note.
- **D-26: Deluxe Change lines carry the track-count suffix `(<tracksFieldValue>)`**, e.g. `(12 → 15 tracks)`, omitted entirely when `tracksFieldValue` returns `""` (never `()`). Duplicate lines for one release seen from both sources are **not** merged — one event, one line, one ack.
- **D-21: Community-editable text is escaped.** Backslash-escape Discord markdown metacharacters (`\ * _ ~ ` | > # [ ] ( )`) in artist names and titles, and cap each title at ~100 runes with `…`. Titles routinely contain `[Deluxe]` / `(Remix)`, which break masked links, and `*`/`_` bleed formatting into headings. Table-test `[Deluxe]`, `(Remix)`, `*`, `_`, `\`. Record as a threat-model entry alongside T-05-06 (`allowed_mentions` already neutralizes pings).

### Scheduler Mechanics

- **D-08: The due-check interval is a hardcoded Go constant, not an env-configurable knob** — there's no legitimate operator reason to want a different check cadence.
- **D-09: The interval value is 5 minutes** — a missed 00:05 fire is caught within minutes, at negligible background cost.
- **D-10: The first due-check runs immediately on process start**, then every 5 minutes thereafter — not pure `time.Ticker` semantics waiting out the first full interval. Together with D-12 this is what delivers criterion 3's restart catch-up. (Corrected: `authgate.Manager.sweepLoop` does *not* run immediately; this is a deliberate addition, not something inherited from it.)
- **D-18: Lifecycle and wiring.** `notifier.Sink` gains `SendDigestIfDue(ctx, logger, now time.Time) error`; `NoOp` returns nil, so `notifier.Select`'s signature and its disabled-webhook inertness are unchanged, and `poller.Notifier` is untouched. A `notifier.DigestScheduler` built from the `Sink` exposes `Start(ctx)` / `Stop(drainCtx)` modeled on `Poller.Start`/`Stop` (retained child ctx, drain bounded by ctx, cancel on drain timeout), with an injected `now func() time.Time` and tick source so fake-clock DST tests drive it directly. `cmd/server/main.go` defers `Stop` beside the poller's drain — after `defer pool.Close()` — so an in-flight digest never races the pool closing.
- **D-24: Logging levels.** Not-due ticks log at Debug; every state-changing decision logs at Info/Warn: due, sent (with counts), empty skip, grace expired (Warn), lock held, settings read failure (Warn, Phase 21's shared literal). `LOG_LEVEL` defaults to info, so production shows every decision without 288 idle lines/day.

### Timezone Resolution & Release

- **D-23: Zone resolution is fail-fast and CI-verified.** `cmd/server` loads `America/New_York` once at startup; a load error exits the process (no fallback to UTC anywhere), and success logs one Info line carrying the zone name and current offset. A new `build-scan` step boots the just-built image against a throwaway Postgres (the `n1-boot` job's pattern) and asserts that log line — no existing job boots the *current* image, and a unit test can't prove Alpine behavior because Go consults host zoneinfo before embedded `time/tzdata`.
- **D-19 [informational]: Phases 21, 22, and 23 ship in one release.** A single-embed digest over Description's 4096-character budget is rejected by Discord, never acked, and retried every due-check — each later digest larger than the last. Holding 22 until 23's chunking lands closes that wedge with no interim cap code. Release-sequencing rationale, not a phase-22 implementation decision — nothing in this phase's code changes because of it (22-03's threat register cites it only as the accepted-risk rationale for shipping without chunking).

### Claude's Discretion

- Exact Go identifiers/file layout (constant names, `DigestScheduler` file placement, the per-type URL helper extraction).
- Exact sqlc query names and parameter shapes for D-14's revised update and D-16's ack statement (e.g. how "only advance the watermark when something was sent" is expressed), and whether the ack bumps `updated_at` (lean: no — `updated_at` tracks operator writes).
- Exact `slog` field names/wording, following the terse structured style in `internal/notifier/notifier.go`.
- The exact shape of the `build-scan` boot step (reusing `n1-boot`'s Postgres bring-up is the lean).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15 are this phase's contract (DGST-05/15 reworded 2026-09-16 to match D-11–D-13); the "Out of Scope" table binds (no operator-configurable fire time/timezone picker).
- `.planning/ROADMAP.md` → "Phase 22: Scheduled Digest Send" → "Notes for the phase planner" — updated 2026-09-16 to match this file.
- `.planning/ROADMAP.md` → "Ordering rationale" / "Deploy sequencing" — why Phase 21 lands before this phase, and why 21+22+23 release together (D-19).
- `docs/adr/0002-one-outbox-one-sender-lock.md` — the shared sender lock the digest send serializes on.

### Prior phase context (established patterns this phase follows)
- `.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md` — the `SettingsReader` seam, the `notifying` CAS lock as the shared sender lock, fail-closed settings reads, the `created_at`-anchored staleness cutoff.
- `.planning/phases/20-digest-settings-operator-control/20-CONTEXT.md` — the `internal/settings.Store` narrow Get/Update seam; the singleton-row `CHECK (id = 1)` every new query must respect.

### Existing code this phase extends (read before writing the digest-send path)
- `internal/notifier/notifier.go` — `Notifier`, `NotifyPending`, `notifying`, `Sink`/`NoOp`/`Select`, `SettingsReader`, `readSettings`/`logSettingsReadFailure`, `dbOpTimeout`, `suppresses`/`staleReleaseDate`.
- `internal/notifier/format.go` — per-type formatters and URL helpers, `truncateRunes`, `tracksFieldValue`.
- `internal/discord/client.go` — `Client.Send(ctx, Embed)`, `Embed.Description`, `allowed_mentions` (unchanged).
- `internal/poller/poller.go` (`Start`/`Stop`, ~line 264–295) — the lifecycle model D-18 follows.
- `internal/settings/settings.go` — `Store`, `Service.Get`/`Update`, `Cadence`, `Settings.DigestLastSentAt`; gains slot math (D-15) and the re-anchor (D-14).
- `queries/notification_settings.sql` — `GetNotificationSettings`, `UpdateNotificationSettings` (revised by D-14); D-16's ack statement is new.
- `queries/events.sql` — `ListUnnotified`, `MarkNotified` (the per-event ack D-16 does not reuse).
- `internal/db/migrations/README.md` + `000008_notification_settings.up.sql` — rules and prior shape for migration 000009 (D-13); `000007_backfill_events_watched_artist_name.up.sql` — coverage of `watched_artist_name` (D-20).
- `internal/db/sqlc/db.go` — `WithTx` exists only on `*Queries`, why D-16 is a single statement.
- `cmd/server/main.go` (~line 255 `settingsStore`, ~line 302 `notifier.Select`, ~line 322 poller drain defer) — composition root: `time/tzdata` import, zone fail-fast (D-23), scheduler `Start` + drain defer (D-18).
- `.github/workflows/full-pipeline.yml` → `build-scan` (~line 556) and `n1-boot` (~line 380) — where D-23's image boot step goes and the pattern it reuses.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" — `go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check` (local-only, no CI counterpart — this phase changes and adds sqlc queries and a migration).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/notifier/format.go`'s URL builders and `tracksFieldValue` — the digest line builder reuses these rather than new link logic.
- `internal/poller.Poller`'s `Start`/`Stop` — retained child ctx, bounded drain, cancel on timeout: the shape D-18's scheduler needs.
- `internal/notifier.Notifier`'s `notifying` CAS lock, `dbOpTimeout`, `readSettings`, `listUnnotified`, `suppresses` — reused by the digest send. The per-event `markNotified` is *not* reused (D-16).

### Established Patterns
- Consumer-declared narrow seams (`Sender`, `SettingsReader`, `Sink`) — D-18 extends `Sink` rather than adding a type assertion.
- `slog` structured summary lines, not per-row lines (D-24).
- Functional options for per-environment tunables — deliberately *not* used for the check interval (D-08).
- One stateless `sqlc.New(pool)` per consumer at the composition root.

### Integration Points
- `internal/notifier/` — `SendDigestIfDue` on `Notifier`/`NoOp`, `DigestScheduler`, digest message builder with escaping + collation.
- `internal/settings/` — slot math, grace constants, zone name, clock injection, re-anchor in `Update`.
- `internal/db/migrations/000009_*` + `queries/notification_settings.sql` + regenerated sqlc — slot column, revised update, ack statement.
- `cmd/server/main.go` — `time/tzdata`, zone fail-fast + log line, scheduler start + drain defer.
- `.github/workflows/full-pipeline.yml` — `build-scan` image boot step.
- `go.mod` — `golang.org/x/text` indirect → direct.

</code_context>

<specifics>
## Specific Ideas

- Digest Description text shape: `**New Releases**\n- [Artist — Title](url)\n**Guest Features**\n- [Watched on Host — Title](url)\n**Deluxe Changes**\n- [Artist — Title](url) (12 → 15 tracks)\n...` — a group heading is omitted entirely when that type has zero sendable events.
- Fire times: daily 00:05 America/New_York; weekly Friday 00:05 America/New_York (covering through Thursday night). Grace: 12h daily / 48h weekly.
- DST tests: drive `DigestScheduler` with a fake clock in 5-minute steps across both 2026/2027 transitions for both cadences and assert exactly one send per slot; include a poll-inserted event between the would-be double fires to prove the slot record, not an empty outbox, prevents the second send.

</specifics>

<deferred>
## Deferred Ideas

- The "since &lt;timestamp&gt;" window header and multi-message chunking — Phase 23 (same release, D-19). Its label reads `digest_last_sent_at`, which is NULL for the first digest after enabling — Phase 23 must render that case.
- Operator-configurable fire time / timezone picker — out of scope for v1.5 per REQUIREMENTS.md.
- Merging duplicate lines for one release detected from both MusicBrainz and Deezer — unverified how common; revisit only if observed (D-26).

### Reviewed Todos (not folded)
- *Resolve the D-15 previous-release schema/query files from `--prev-tag`* — `cmd/migration-check` tooling, unrelated to the digest scheduler; weak keyword match only (score 0.6). Already reviewed-and-deferred identically in Phase 20's discussion.
- *Unify `internal/sqlscan`'s two quote/dollar-quote state machines* — backend tooling cleanup, unrelated; weak keyword match only (score 0.6).
- *Move `shadcn` from dependencies to devDependencies in `web/package.json`* — frontend tooling cleanup, unrelated; weak keyword match only (score 0.4).

</deferred>

---

*Phase: 22-scheduled-digest-send*
*Context gathered: 2026-09-16 · revised 2026-09-16 after grilling session*
