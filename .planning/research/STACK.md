# Stack Research

**Domain:** Digest/batch notification mode for drop-tracker (milestone v1.5) — a scheduled Discord digest built on the existing robfig/cron + internal/notifier stack, gated by a Postgres-persisted, SPA-configurable instance setting.
**Researched:** 2026-09-11
**Confidence:** MEDIUM (Discord API limits and robfig/cron API surface cross-checked across multiple independent sources; the settings-table/cache pattern is corroborated general Go practice, not a single canonical spec)

## Scope note

This is a subsequent-milestone research pass, not a greenfield stack pick. Go 1.23+, chi, sqlc + pgx/v5, golang-migrate, robfig/cron/v3, and the hand-rolled `internal/discord`/`internal/notifier` packages are already validated (see PROJECT.md Key Decisions) and are **not** re-evaluated here. The only question is what, if anything, needs to be *added* for the digest capability, and how it should integrate with what already exists.

## Recommended Stack

### Core Technologies (no additions — reuse as-is)

| Technology | Version (pinned in go.mod) | Purpose for this milestone | Why no new library is needed |
|------------|---------|---------|-----------------|
| `github.com/robfig/cron/v3` | v3.0.1 | Drives the digest's periodic "is it time to send?" tick | Already the project's locked scheduler (D-08 pattern: independent `cron.AddFunc` entries, one per concern). A digest job is a third independent entry alongside the existing MusicBrainz/Deezer ones — no dynamic-scheduling library needed (see Pattern below). |
| `github.com/sqlc-dev/sqlc` (CLI) + `github.com/jackc/pgx/v5` | sqlc v1.31.1 / pgx v5.10.0 | Generated queries for the new settings table and the digest's `events` batch read | Same codegen pipeline as every other table in the project; a new `queries/instance_settings.sql` + `sqlc generate` is all that's needed, no new driver or ORM. |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 | One additive migration (`000008_instance_settings`) | Plain `CREATE TABLE`, no destructive DDL — safe under the project's own N-1 migration-safety gate (`cmd/migration-check`). |
| `internal/discord` (hand-rolled) | n/a (in-repo) | POSTs the digest embed(s) | Already handles the 429/Retry-After dance, `allowed_mentions` suppression, and the DSN/URL-redaction discipline. The digest reuses `discord.Client.Send` unchanged — it needs a *new caller* (a digest formatter), not a new client. |

### New additions (small, in-repo — no new third-party dependency)

| Addition | Purpose | Why in-repo, not a library |
|----------|---------|-----------------------------|
| `internal/instancesettings` (or similarly named) package | Postgres-backed, mutex-guarded, SPA-configurable instance setting (`digest_enabled`, `digest_cadence`, `digest_last_sent_at`) | Mirrors `internal/pollruns.Store`'s existing shape almost exactly: one struct, one `sync.RWMutex` (or `sync.Mutex`, matching pollruns), a `Get`/`Set` seam. This is ~80 lines given the precedent already in the codebase — pulling in a config/feature-flag library (`spf13/viper`, LaunchDarkly SDK, etc.) would be pure overkill for one boolean + one enum + one timestamp, and directly contradicts the project's own "What NOT to Use" stance against `viper` for exactly this reason (STACK.md: "pulls in a large dependency tree… for a task that's just reading structured config"). |
| A digest-specific embed formatter (new function(s) in `internal/notifier`, not a new package) | Batches N `events` rows into 1–3 Discord embeds instead of one embed per event | `internal/notifier/format.go`'s existing `formatEmbed`/`formatNewRelease`/etc. are deliberately one-event-per-embed (D-07's "one embed per message" contract) and cannot be reused unmodified for a digest without blowing embed/field budgets on any real accumulation window — see Discord limits below. This needs new logic, not a new dependency. |

## Discord embed limits (verified, cross-checked — directly constrains the digest formatter)

The existing `internal/notifier/format.go` already encodes the two limits it needs (`titleLimit = 256`, `fieldValueLimit = 1024`) plus a `truncateRunes` helper. A digest formatter needs the **rest** of the limit surface, since it deliberately violates D-07's one-event-per-embed assumption:

| Limit | Value | Digest implication |
|-------|-------|---------------------|
| Fields per embed | 25 max | A field-per-event layout (mirroring the current per-event embeds) caps out at 25 events per embed — too low for a weekly digest on an active watchlist. |
| Field value | 1,024 chars | Same constraint that already exists today; irrelevant if fields aren't used for line items. |
| Embed description | 4,096 chars | Large enough to hold a compact multi-line list ("• Artist — Title (date)" per line) for dozens of events in one embed. |
| **Total embed character budget** | **6,000 chars, summed across every field/title/description/footer/author in *all* embeds of one message** | This is the binding constraint, not the per-field or per-embed limits individually — a digest with 3 embeds (one per event type) each using a list-style description must still sum to ≤6,000 chars total. |
| Embeds per message | 10 max | Plenty of headroom for a 3-embed-max (one per event type: new release / guest feature / deluxe change) layout. |
| Webhook rate limit | ~30 req/60s per webhook URL; 5 req/5s shared per channel; 50 req/s global | Irrelevant at digest volume (1 message per cadence tick) — this only mattered for the existing per-event notifier's burst behavior, not the digest. |

**Recommended digest embed shape:** one embed per event type present in the batch (max 3: new_release, guest_feature, deluxe_change), each using `Description` as a truncated, newline-joined list of compact per-event lines (not `Fields` — `Fields` is the wrong shape here since 25/embed is the real ceiling on a busy digest, while `Description`'s 4,096-char budget comfortably fits far more compact list lines). Cap each embed's description via the existing `truncateRunes` helper plus an explicit "+N more" trailing line when truncated, so a pathological accumulation (many watched artists, weekly cadence) degrades to a readable summary instead of a rejected or silently-truncated-mid-line request. This keeps the total 3-embed message safely under the 6,000-char cross-embed sum for any realistic personal-watchlist volume, while the explicit "+N more" guard handles the unbounded case defensively — this shape is a phase-planning decision, not a library choice, so it's flagged here for the roadmap rather than fully specified.

## Pattern: scheduling "daily/weekly at a stable time" without a new library

**No new scheduling library is needed.** robfig/cron/v3's own API — `AddFunc`/`Schedule` returning an `EntryID`, `Remove(id)` — supports two designs:

1. **Remove-and-re-add on cadence change** (literal cron spec per cadence, e.g. `30 9 * * *` for daily 09:30, `30 9 * * 1` for weekly Monday): correct, but requires the SPA's settings-write handler to reach into the running `*cron.Cron` and call `Remove` + re-`AddFunc` every time an operator flips the toggle or cadence — adds a live-mutation seam between the HTTP handler and the scheduler that the project's poller doesn't currently have anywhere (`poller.New` registers its two entries once, at construction, and never touches them again).
2. **Fixed frequent tick + DB-driven decision** (recommended): register one more `cron.AddFunc("@every 15m", ...)` entry at construction time — same `@every` idiom `poller.New` already uses for both existing entries — whose job body reads the cached instance setting and does nothing unless `digest_enabled && time.Since(last_sent_at) >= cadence duration (24h or 7*24h)`. Toggling on/off or changing cadence from the SPA only ever writes a row; the cron entry itself is never added, removed, or touched again after `Start()`. This is the same shape the project already chose for the poll-run history problem (an always-registered, cheap, idempotent check rather than dynamic scheduler mutation) and sidesteps `robfig/cron`'s lack of an in-place "update this entry's schedule" API entirely, plus DST/timezone edge cases a literal `HH:MM` cron field would otherwise need to reason about.

This also means: if a future milestone wants a literal fixed local send time (e.g., "always at 9am, not just once-per-24h-ish"), that is a distinct, larger design decision (timezone handling, DST, `cron.WithLocation`) that the current milestone's stated scope — on/off + daily/weekly cadence only, no time-of-day picker — does not require. Flagging this as an open scope question for phase planning rather than deciding it here.

## Pattern: settings table + read-through cache

**Table:** a single-row (fixed-id, `CHECK (id = 1)`-style) `instance_settings` table — mirrors the project's existing convention of CHECK-constrained enum columns (`events_event_type_valid`, `events_source_valid`) rather than a free-form key-value table. Minimal columns: `digest_enabled boolean not null default false`, `digest_cadence text not null default 'daily'` with a CHECK `IN ('daily','weekly')`, `digest_last_sent_at timestamptz`, `updated_at timestamptz not null default now()`. Defaults preserve "default stays real-time/off" per the milestone goal with no application-level fallback logic needed.

**Cache:** a `sync.RWMutex`-guarded struct in the new `internal/instancesettings` package, read by both the digest cron tick and the `GET`/`PUT` HTTP handlers backing the SPA toggle — this directly mirrors `internal/pollruns.Store`'s existing mutex-guarded-struct shape (already proven in this codebase, Phase 18). Because drop-tracker is explicitly a **single-instance** architecture (PROJECT.md Constraints: "single Go binary/service… not split microservices"), a synchronous invalidate-on-write is correct and sufficient — the `PUT` handler writes to Postgres and then updates the in-process cache in the same call, with no TTL, no polling, and no cross-instance cache-coherency concern (that concern only exists once there are multiple replicas, which is explicitly out of this project's architecture). This also means **no Postgres `LISTEN`/`NOTIFY`, no Redis, and no distributed cache library** are needed — those solve a multi-process cache-invalidation problem this single-binary architecture doesn't have.

Given the read volume here (an SPA settings panel + one cron tick every 15 minutes), a cache is a nice-to-have for consistency with `pollruns.Store`'s precedent more than a load-bearing performance need — a direct single-row `SELECT` on every read call site would also be simple and correct. Either is acceptable; the cache is recommended primarily for pattern consistency with the codebase's existing seam style (`Store` structs with narrow `Get`/interfaces, injected at the composition root in `cmd/server/main.go`, exactly like `watchlist.Store`, `pollruns.Store`).

## Installation

No new `go.mod` entries. Everything above is either an existing dependency or new in-repo code:

```bash
# No `go get` needed — robfig/cron, sqlc, pgx, golang-migrate all already present.
# New work is: one migration file, one sqlc query file + `sqlc generate`,
# one new internal/ package, and new functions in internal/notifier.
```

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|--------------------------|
| Fixed `@every 15m` tick + DB-driven "is it due" check (robfig/cron, existing dependency) | Remove/re-add literal `HH:MM` cron entries on every settings change | If a future milestone adds a genuine operator-facing time-of-day picker ("send my digest at exactly 9am") rather than just a cadence — at that point the remove/re-add pattern (and `cron.WithLocation` for timezone correctness) becomes necessary; today's on/off + daily/weekly-only scope doesn't need it. |
| In-process `sync.RWMutex`-guarded settings cache (mirrors `pollruns.Store`) | Direct Postgres `SELECT` on every read, no cache | Equally valid for this traffic volume — pick this if the team prefers one fewer seam/package over consistency with the `pollruns.Store` precedent. Either is fine; neither needs a library. |
| Single-row `instance_settings` table with CHECK-constrained columns | A generic `key text, value jsonb` settings table | If more instance-wide settings are expected soon (this milestone's own "Out of scope" list already parks per-event-type overrides and other config) — a KV table avoids a new migration per future setting, at the cost of losing column-level typing/CHECK constraints. Given the project's stated preference for typed, sqlc-generated columns everywhere else, the fixed-column table is the better fit *today*; revisit if settings sprawl. |
| Description-based list embeds (1 per event type, max 3) | Field-based embeds (1 field per event, matching today's per-event style) | Field-based only works up to 25 events/embed before requiring pagination logic (multiple digest messages) — acceptable if digest volume is known to always stay low, but the description-list shape scales further before any pagination is needed and is the safer default for an unbounded accumulation window (weekly cadence on a growing watchlist). |

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| A third-party job-queue/scheduler library (`asynq`, `river`, `gocron`, etc.) | This is one more periodic in-process check on top of an already-validated `robfig/cron` scheduler — adding a second scheduling paradigm (often backed by Redis or its own Postgres tables) for a single boolean-gated tick is the same category of over-engineering the project already rejected for poll-run history (`docs/adr/0001`) and the worker pool (hand-rolled semaphore, not a pool library) | `robfig/cron/v3`, already a dependency — one more `AddFunc("@every ...")` entry |
| `spf13/viper` or any config/feature-flag SDK for the settings toggle | The project's own STACK.md already rejects `viper` for pure env-var config as unnecessary weight; a DB-persisted, SPA-editable boolean+enum+timestamp is even further from what a feature-flag SDK is built for (multi-environment rollout, targeting rules, remote evaluation) | A single-row Postgres table + sqlc queries + the in-process cache pattern above |
| Reusing `formatEmbed`/`formatNewRelease`/etc. unmodified, looped once per event, to build the digest | These are deliberately one-event-per-embed (D-07); looping them into a multi-embed digest message hits the 10-embeds-per-message ceiling after 10 events and risks the 6,000-char cross-embed sum well before that on any richly-populated week | A dedicated digest formatter using compact `Description` list lines per event type (see Discord limits section) |
| Postgres `LISTEN`/`NOTIFY`, Redis, or any distributed cache/pubsub for settings-cache invalidation | Solves a multi-instance cache-coherency problem; drop-tracker is explicitly single-binary/single-instance (PROJECT.md Constraints) | Synchronous invalidate-on-write inside the same HTTP handler that persists the change, exactly as the read-through cache pattern above describes |
| Literal per-operator time-of-day cron scheduling (dynamic `Remove`/re-`AddFunc` on every settings change) for *this* milestone | Out of the milestone's stated scope (on/off + daily/weekly cadence only, no time picker) — building the more complex remove/re-add + timezone-aware path now is speculative scope creep | The fixed-tick + elapsed-time-since-`last_sent_at` check described above |

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|------------------|-------|
| `robfig/cron/v3` v3.0.1 | Go 1.23+ | No version bump needed; already the pinned version project-wide. |
| `sqlc` v1.31.1 codegen for `instance_settings` | `pgx/v5` v5.10.0 | Same `sql_package: "pgx/v5"` setting already in `sqlc.yaml` applies unchanged — no new sqlc config needed for a new table in the same schema. |
| New migration `000008_instance_settings` | `cmd/migration-check` (Phase 16) | Purely additive `CREATE TABLE` — no `DROP`/`RENAME`/type-narrowing/`NOT NULL ADD COLUMN`, so it should pass the existing N-1 safety guard with no `allow-destructive` annotation needed. Confirm at plan time per `internal/db/migrations/README.md`. |

## Sources

- WebSearch, cross-checked across multiple independent sources (klartext-tools.com, discord-webhook.com embed-limits guide, docs.discord.com rate-limits page referenced in results) — Discord embed/field/message/rate limits — confidence MEDIUM (verified/cross-checked)
- WebSearch, cross-checked against pkg.go.dev (`github.com/robfig/cron/v3`) and the `robfig/cron` GitHub source (`cron.go`) — `AddFunc`/`Remove`/`EntryID` API shape, `@every`/`@daily`/`@weekly` descriptors, `cron.WithLocation` — confidence MEDIUM (verified/cross-checked)
- WebSearch, general Go/Postgres settings-cache pattern discussion (RWMutex config-store idiom, GitLab's application-settings-cache TTL precedent as a comparison point) — confidence MEDIUM (corroborated pattern, not a single canonical spec)
- In-repo: `C:/CodeProjects/drop-tracker/internal/poller/poller.go`, `internal/pollruns/pollruns.go`, `internal/discord/client.go`, `internal/notifier/format.go`, `internal/config/config.go`, `go.mod` — read directly to confirm existing pinned versions and the precedent patterns this research recommends mirroring (D-08 independent cron entries, `pollruns.Store`'s mutex-guarded-struct shape, `truncateRunes`/`titleLimit`/`fieldValueLimit` already in `internal/notifier`) — confidence HIGH (primary source, current codebase)

---
*Stack research for: drop-tracker v1.5 Digest Notifications*
*Researched: 2026-09-11*
