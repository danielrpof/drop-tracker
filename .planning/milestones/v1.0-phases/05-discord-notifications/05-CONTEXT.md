# Phase 5: Discord Notifications - Context

**Gathered:** 2026-08-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Events already recorded and preference-filtered by Phase 4 (mute/release-type filtering happens at detection time, D-17/D-18 — a muted event never becomes a row) get posted to a Discord webhook, formatted distinctly per event type (new release / guest feature / deluxe change), and marked `notified_at` on success. The `events` table already carries the outbox shape this needs (`notified_at IS NULL` = pending, `ListUnnotified` query already exists) — this phase wires actual Discord delivery on top of an already-designed mechanism, plus the concurrency-safety and burst/failure handling that delivery requires. No new detection logic, no UI (Phase 6).

</domain>

<decisions>
## Implementation Decisions

### Visual distinction & content (NTFY-01/02/03)
- **D-01:** Each event type gets a distinct embed side-bar color plus an emoji prefix in the message title (e.g. 🆕 new_release, 🎤 guest_feature, 💿 deluxe_change) — not color-only, and not separate channels/webhooks per type. One webhook URL handles all three types; the color+emoji combination is what makes them scannable at a glance in a busy channel.
- **D-02:** A `guest_feature` notification shows the recording title, the primary credited artist (from the event row's existing `artist_name` snapshot), and a link to the recording on MusicBrainz — no fetch of the full artist-credit list (the event snapshot doesn't store it, and DTCT-03 already only cares about the watched artist's non-primary position).
- **D-03:** A `deluxe_change` notification shows the release title, the old→new track-count delta (e.g. "12 → 18 tracks"), and a link to the release.
- **D-04:** Because the `events` table's `track_count` column only ever holds the *current* baseline (overwritten on each deluxe_change, per `setGroupBaseline`) — not what it was before — showing the D-03 delta requires a new nullable `previous_track_count` column, populated by Phase 4's detection code (`internal/detection/musicbrainz.go`'s deluxe-change insert path) at the same point it currently calls `setGroupBaseline`, capturing the pre-update `baseline` value into the new event row before it's overwritten. — **Reversibility:** reversible — additive column, no existing data affected, no existing query changes shape.

### Notification cadence/trigger
- **D-05:** Notifications are sent inline, at the end of each poll cycle (`RunMusicBrainzCycle`/`RunDeezerCycle` in `internal/poller/poller.go`) rather than via a separate, independently-scheduled notifier cron job. No new `Config` field, no new cron entry — reuses the existing `PollInterval`-driven cadence and cycle machinery. Detect-to-notify latency is effectively the remainder of the current cycle.
- **D-06:** `ListUnnotified` is a global query (no source/artist scoping), and MusicBrainz/Deezer cycles run as independent, potentially-concurrent goroutines (Phase 3 D-08) — without a guard, both cycles' inline notify calls could race on the same pending rows and double-post the same event. A single shared mutex (or equivalent atomic guard) wraps the whole "fetch pending → post → mark notified_at" sequence, called by both cycles, mirroring the existing `mbRunning`/`dzRunning` overlap-guard pattern already in `poller.go` — just one shared guard instead of two per-source ones. — **Reversibility:** reversible — a future `SELECT ... FOR UPDATE SKIP LOCKED` swap (needed only if the app ever runs multiple instances) replaces this guard without changing the calling cycles.

### Burst handling & failure behavior
- **D-07:** Multiple pending events are sent as separate Discord messages, one event per message, serially with spacing between sends (per PITFALLS.md Pitfall 5's guidance — e.g. ~350–500ms) to stay under Discord's ~30/min webhook rate limit — not batched into multi-embed messages. Keeps a clean 1:1 event→embed mapping and avoids mixing colors/emoji from D-01 within one message.
- **D-08:** A 429 response's `Retry-After` must be honored before any retry within a single send attempt (per PITFALLS.md Pitfall 5) — the exact retry/backoff shape within one send is left to research/planning.
- **D-09:** When a send ultimately fails (network error, 5xx, or a 429 that exhausts in-attempt retries), the event row is left with `notified_at` still NULL and the failure is logged — no retry-count column, no give-up-after-N-attempts logic. The next poll cycle's notify pass picks the row back up automatically, since `ListUnnotified` already re-selects any row with `notified_at IS NULL`. A permanently broken webhook just keeps retrying and logging every cycle, which is visible in logs without new state.

### Webhook-unset behavior
- **D-10:** `DISCORD_WEBHOOK_URL` stays optional (no `notEmpty` constraint — do not change Phase 1's D-06/D-07 decision). When unset, the app boots normally and the notify step no-ops, with a single log line at startup noting Discord notifications are disabled. Useful for local dev/testing without a real webhook configured.

### Claude's Discretion
- Exact embed color values (hex) and emoji choices for each event type — D-01 locks the *mechanism* (color + emoji), not the specific palette.
- Exact spacing duration between serial sends (D-07) and the precise retry/backoff shape for a single send's 429 handling (D-08) — implementation detail within the stated constraints (stay under ~30/min, honor `Retry-After`).
- Whether the shared notifier guard (D-06) is a `sync.Mutex` or an `atomic.Bool` CAS — mirrors the existing `mbRunning`/`dzRunning` implementation choice, not discussed further.
- Exact Go project layout for the new Discord client/notifier package (`internal/discord`, per CLAUDE.md's "Supporting Libraries" table) and where the shared mutex/guard lives (`Poller` struct vs. a new `Notifier` type it holds) — architecture detail, follows existing seam patterns (`ReleaseGroupSource`, `AlbumSource`, `EventRecorder`) either way.
- Structured log field names for notifier log lines — follow existing `slog`/`request_id`/`cycle_id` conventions from Phases 1–4.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & Roadmap
- `.planning/REQUIREMENTS.md` — NTFY-01 through NTFY-04 (this phase's requirement set); note NTFY-04 (mute suppression) is already satisfied by Phase 4 D-18 — nothing new to build for it in this phase, only to confirm it stays satisfied.
- `.planning/ROADMAP.md` §"Phase 5: Discord Notifications" — goal and success criteria.
- `.planning/PROJECT.md` — constraints and Key Decisions table.

### Tech stack / library choices (already locked)
- `.claude/CLAUDE.md` — hand-rolled Discord notifier (`internal/discord`, plain `net/http` POST, typed `Embed` struct), no webhook-client library.

### Research already done for this phase
- `.planning/research/PITFALLS.md` Pitfall 3 ("Diff-against-seen-store race conditions produce duplicate or missing notifications") — the transactional-outbox pattern it recommends is already what Phase 4's `events` table + `notified_at` column implements; D-09 depends on this being correct (never mark `notified_at` before a send actually succeeds).
- `.planning/research/PITFALLS.md` Pitfall 5 ("Discord webhook rate limits and message-size limits break notifications during release bursts") — directly shapes D-07/D-08 (serial spacing, honor `Retry-After`, use embeds not raw content strings).

### Prior phase decisions this phase builds directly on
- `.planning/phases/04-detection-engine/04-CONTEXT.md` — D-11 (nullable `notified_at` column, `SELECT WHERE notified_at IS NULL` is Phase 5's job — already built), D-12 (event rows carry an inline display snapshot so Phase 5 needs no live re-fetch), D-17/D-18 (release-type filter and mute preference both applied at detection time — Phase 5 inherits NTFY-04 for free), D-20 (`ON CONFLICT DO NOTHING` — a re-detected event's original snapshot is never overwritten, relevant background for why D-04's new column must be written at insert time, not patched in later).
- `.planning/phases/03-external-clients-search/03-CONTEXT.md` — D-08 (MusicBrainz/Deezer run as independent, potentially-concurrent poll cycles — the reason D-06's shared guard is needed at all), D-09 (the existing `mbRunning`/`dzRunning` overlap-guard pattern D-06's shared notifier guard mirrors).

### Existing code (Phase 1–4)
- `internal/poller/poller.go` — `RunMusicBrainzCycle`/`RunDeezerCycle`, where the inline notify call (D-05) is added at the end of each cycle's per-artist loop; `mbRunning`/`dzRunning` atomic-guard pattern (D-06 mirrors this).
- `internal/detection/musicbrainz.go` (deluxe-change insert path, around the `setGroupBaseline`/`InsertEvent` calls) — where D-04's `previous_track_count` capture must be added, using the `baseline` value already computed just before it's overwritten.
- `internal/db/migrations/000003_events.up.sql`, `queries/events.sql`, `internal/db/sqlc/events.sql.go` — current `events` table shape, `ListUnnotified`/`InsertEvent` queries; D-04's new column is a new migration (`000004_*`) following this file's exact conventions (explicit constraints, indexed where needed).
- `internal/config/config.go` — `DiscordWebhookURL string \`env:"DISCORD_WEBHOOK_URL"\`` already exists, intentionally optional (D-10 keeps it that way).
- `internal/db/sqlc/models.go`, `internal/db/sqlc/events.sql.go` — sqlc-generated `Event` struct and `ListUnnotified`/`InsertEvent` methods this phase's notifier reads from and writes back to (marking `notified_at`).

No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `queries/events.sql`'s `ListUnnotified` query — already selects exactly what the notifier needs (`notified_at IS NULL ORDER BY created_at ASC, id ASC`), no new query needed for the read side.
- `internal/db/sqlc.Querier` interface — the notifier should depend on this narrow interface (or an even narrower slice of it), mirroring `Detector`'s own dependency on `sqlc.Querier` rather than a concrete `*sqlc.Queries`.
- `internal/poller.Poller`'s existing `mbRunning`/`dzRunning atomic.Bool` overlap-guard fields — the template for D-06's shared notifier guard.

### Established Patterns
- Narrow-interface seams declared in the consumer, not the producer (`poller.ReleaseGroupSource`, `poller.AlbumSource`, `poller.EventRecorder`, `detection.RecordingSource`, `detection.ReleaseDetailSource`) — a new `poller.Notifier`-style seam (or similar) should follow the same convention.
- Migrations are plain up/down `.sql` under `internal/db/migrations/`, embedded via `go:embed`, sequentially numbered — D-04's new column is `000004_*`.
- Hand-rolled external HTTP clients live in their own `internal/<name>` package (`internal/musicbrainz`, `internal/deezer`) — `internal/discord` follows the same shape per CLAUDE.md.
- Structured `slog` logging with `cycle_id`/`source` correlation is already established per-source in the poller — notifier log lines should extend the same logger passed into the cycle, not start a new one.

### Integration Points
- New `internal/discord` package (webhook client + typed `Embed`/formatting) is called from `internal/poller/poller.go`'s cycle methods (D-05), gated by the shared guard (D-06).
- New migration `000004_*` adds `previous_track_count` to `events` (D-04); `internal/detection/musicbrainz.go`'s deluxe-change path and `queries/events.sql`'s `InsertEvent`/related queries need updating to populate it.
- `cmd/server/main.go` (or wherever `poller.New` is currently called) wires the new Discord client/notifier into the `Poller` constructor, following the same pattern `mb`/`dz`/`events` params already use.

</code_context>

<specifics>
## Specific Ideas

No particular visual mockup or exact copy was specified — the recurring theme was "make it obviously distinguishable at a glance" (color + emoji) without over-building (no per-type channels, no batched multi-embed messages, no retry-count bookkeeping). The one concrete technical requirement that emerged mid-discussion — persisting `previous_track_count` — came from tracing through the actual `events` schema and Phase 4's `setGroupBaseline` code, not from a prior spec; it's now a locked decision (D-04) precisely because the notifier can't show what Phase 4 never captured.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.

</deferred>

---

*Phase: 5-Discord Notifications*
*Context gathered: 2026-08-08*
