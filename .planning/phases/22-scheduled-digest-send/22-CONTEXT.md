# Phase 22: Scheduled Digest Send - Context

**Gathered:** 2026-09-16
**Status:** Ready for planning

<domain>
## Phase Boundary

With digest mode on, everything accumulated in the outbox since the last digest arrives as one grouped Discord message at a predictable, fixed time — and that schedule survives restarts, DST transitions, and the shipped Alpine image's minimal timezone database.

Requirements: DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15 (`.planning/REQUIREMENTS.md`).

**In scope:**
- The digest due-check scheduler (a `time.Ticker`-driven goroutine, not a third `robfig/cron` entry) and the digest send itself as a `Notifier` method sharing the existing `notifying` sender lock
- Batching + grouping: generalizing Discord delivery beyond one-embed-per-event into one grouped message
- The watermark (`digest_last_sent_at`) as what defines "since the last digest," decided independently from the outbox-state row selection (`ListUnnotified`)
- Missed-tick / restart catch-up, DST correctness, Alpine `tzdata` resolution
- Zero-event digest windows sending nothing (silent skip, logged)

**Not in this phase:**
- The digest-mode gate / real-time↔digest mutual exclusion — done, Phase 21
- The "since &lt;timestamp&gt;" window header text and multi-message chunking for an oversized digest — Phase 23
- An operator-configurable fire time/timezone picker — explicitly out of scope per REQUIREMENTS.md; the fire time is a fixed, documented constant
- Any change to `internal/poller` — invisible to the mode switch, per Phase 21
- A second "digest queue" table or `digest_pending` flag — there is exactly one outbox (ADR-0002)

</domain>

<decisions>
## Implementation Decisions

### Locked already by ROADMAP.md (carried forward, not re-litigated)

These were settled during the 2026-09-11 roadmap grilling session and are binding on the planner as-is — see `.planning/ROADMAP.md` → "Phase 22: Scheduled Digest Send" → "Notes for the phase planner" for the full rationale:

- Scheduling mechanism is a `time.Ticker`-driven goroutine modeled on `internal/authgate.Manager.sweepLoop`, not a third `robfig/cron` entry.
- The digest send is a `Notifier` method, serializing on the existing `notifying` `atomic.Bool` CAS lock (ADR-0002) — a send that finds the lock held skips, safely, because the outbox persists.
- Two distinct "since"es: the outbox (`ListUnnotified`, no time predicate) decides *what* to send; the watermark (`digest_last_sent_at`) decides *whether a send is due* and supplies Phase 23's window label. Keep these roles separate in code and tests.
- Restart/missed-tick catch-up is **log-and-wait, no immediate catch-up send** — a detected gap on boot logs a visible warning and waits for the next natural check rather than firing a surprise digest at an arbitrary restart time.
- Events are acked (`MarkNotified`) only after a confirmed 2xx from Discord — a send failure leaves the batch pending for the next digest.
- `time/tzdata` must be blank-imported in `cmd/server` (Alpine ships no zoneinfo).

### Fire Schedule

- **D-01: Fixed timezone is `America/New_York`.** The first pick during discussion was UTC, but that was flagged and reversed: ROADMAP's own success criterion #4 requires proving "the shipped Alpine image resolves the zone it schedules against rather than silently falling back to UTC" — a test that's unobservable if the scheduled zone literally is UTC (a silent fallback and a correct resolution look identical). DGST-06's DST-transition correctness would also be vacuously true under UTC (no transition exists to mishandle). `America/New_York` makes both criteria actually testable.
- **D-02: Daily cadence fires at 00:05 America/New_York.** Chosen specifically to sit outside the ~02:00 local transition window on both the spring-forward and fall-back transitions (research/ROADMAP's own constraint) — 00:05 is unaffected by either the skipped 02:00–03:00 hour or the repeated 01:00–01:59 hour.
- **D-03: Weekly cadence fires on Friday, same 00:05 America/New_York time.**

### Digest Message Layout

- **D-04: Top-level grouping is by event type, then artist.** Three fixed headings — New Releases / Guest Features / Deluxe Changes — each listing that type's artists underneath. Matches the milestone's own framing ("batches into one scheduled message" per event type) and the existing `formatNewRelease`/`formatGuestFeature`/`formatDeluxeChange` split in `internal/notifier/format.go`, which already segments by type.
- **D-05: Rendering shape is one embed, with markdown headings inside its Description** — not multiple embeds, not a field-per-event. Research's own lean: Description text scales much further before hitting Discord's per-embed character limit than a field-per-event layout does (embeds cap at 25 fields; a big digest would hit that ceiling long before Description's budget). Shape: `**New Releases**\n- [Artist — Title](url)\n**Guest Features**\n...`.
- **D-06: Sort order within each event-type group is alphabetical by artist** — deterministic and scannable regardless of detection order, prioritized over the zero-extra-work chronological/`ListUnnotified`-order alternative.
- **D-07: Per-event line is `[Artist — Title](url)`, linked**, reusing the same link-building helpers `formatEmbed` already has (`newReleaseURL`/`musicBrainzReleaseGroupURL`-style). Deluxe Change lines specifically carry an added track-count suffix: `- [Artist — Title](url) (12→15 tracks)`, reusing `tracksFieldValue`'s existing formatting — this detail was flagged as being silently dropped by the plain-linked-line choice and the user chose to restore it rather than lose it in the digest.

### Scheduler Mechanics

- **D-08: The due-check interval is a hardcoded Go constant, not an env-configurable knob** — unlike `MUSICBRAINZ_POLL_WORKERS`-style tunables elsewhere, there's no legitimate operator reason to want a different check cadence (only the fire time/zone would matter to them, and that's already fixed by D-01–D-03).
- **D-09: The interval value is 5 minutes**, matching ROADMAP's own suggestion — a missed 00:05 fire is caught within minutes, at negligible background cost alongside the existing poll/notify cycles.
- **D-10: The first due-check runs immediately on process start**, then every 5 minutes thereafter — not pure `time.Ticker` semantics waiting out the first full interval. This directly demonstrates DGST-05's missed-tick catch-up: a restart shortly after a missed fire doesn't sit idle for up to 5 minutes before checking. Mirrors the restart-safety intent of `authgate.Manager.sweepLoop`.

### Claude's Discretion

- Exact Go identifiers/file layout for the new scheduler goroutine and the digest-send method (e.g. package-level constant names, whether the ticker lives on `Notifier` itself or a small wrapping type) — implementation detail, not re-litigated here.
- How the watermark write (`digest_last_sent_at`) is added to `internal/settings` — a new sqlc query/method distinct from the existing `UpdateNotificationSettings` (which only ever sets `digest_enabled`/`digest_cadence`) is needed; exact naming/shape is the planner's call.
- Exact `slog` field names/wording for the new due-check and skip-empty-window log lines — follow the terse structured style already established in `internal/notifier/notifier.go`.
- Whether the digest-send method reuses `NotifyPending`'s existing `listUnnotified`/`markNotified`/`dbOpTimeout` helpers directly or wraps them — implementation detail.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` — DGST-05, DGST-06, DGST-07, DGST-08, DGST-09, DGST-10, DGST-15 are this phase's contract; the "Out of Scope" table binds (no operator-configurable fire time/timezone picker — D-01–D-03 above are the fixed values that close that gap).
- `.planning/ROADMAP.md` → "Phase 22: Scheduled Digest Send" → "Notes for the phase planner" — the primary brief: scheduling mechanism, the two-"since"es distinction, restart catch-up posture, DST/tzdata requirements, batching/grouping rationale, ack ordering. Every "Locked already by ROADMAP.md" item above is sourced from this section.
- `.planning/ROADMAP.md` → "Ordering rationale" note (above Phase Details) — why Phase 21 (the gate) had to land before this phase: an un-gated real-time drain would empty the outbox every poll cycle, making the digest sender unverifiable.
- `docs/adr/0002-one-outbox-one-sender-lock.md` — the shared sender lock the digest send serializes on.

### Prior phase context (established patterns this phase follows)
- `.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md` — the `SettingsReader` seam (D-05 there), the `notifying` CAS lock now becoming the shared sender lock (D-04 there / ADR-0002), the staleness-anchor fix (D-02 there) that makes a weekly cadence not age pending events out.
- `.planning/phases/20-digest-settings-operator-control/20-CONTEXT.md` — the `internal/settings.Store` narrow Get/Update seam this phase's watermark write extends; the singleton-row enforcement (`CHECK (id = 1)`) this phase's new query must respect.

### Existing code this phase extends (read before writing the digest-send path)
- `internal/notifier/notifier.go` — `Notifier`, `NotifyPending`, the `notifying atomic.Bool` guard, `SettingsReader`, `listUnnotified`/`markNotified`/`dbOpTimeout` helpers, `suppresses`/`staleReleaseDate` (D-02 from Phase 21).
- `internal/notifier/format.go` — `formatEmbed`/`formatNewRelease`/`formatGuestFeature`/`formatDeluxeChange`, `truncateRunes`, `tracksFieldValue`, the `emojiNewRelease`/`emojiGuestFeature`/`emojiDeluxeChange` constants, the URL-building helpers (`newReleaseURL`, `musicBrainzReleaseGroupURL`, etc.) — D-04/D-07 above reuse these rather than duplicating link/format logic.
- `internal/discord/client.go` — `Client.Send(ctx, Embed)`, the `Embed`/`EmbedField` shapes, `defaultTimeout`. D-05's one-embed batch needs the embed's `Description` field populated with the multi-line grouped text.
- `internal/authgate/gate.go` (`Manager.sweepLoop`, `NewManager`, `Close`) — the exact ticker-goroutine-with-Close-for-tests pattern D-10's scheduler follows structurally.
- `internal/settings/settings.go` — `Store` interface, `Service.Get`/`Update`, `Settings.DigestCadence`/`DigestLastSentAt`, `ParseCadence`. No existing method writes `DigestLastSentAt`; this phase adds one.
- `queries/notification_settings.sql` — `GetNotificationSettings`, `UpdateNotificationSettings` (enabled+cadence only, `id = 1` predicate). A new query for the watermark write goes here.
- `cmd/server/main.go` (~line 255, ~line 299) — composition root: `settingsStore` is already constructed and passed into `notifier.Select`/`New` (Phase 21); this phase's scheduler goroutine gets started from here too, alongside the `time/tzdata` blank import.

### Definition of Done
- `C:\CodeProjects\drop-tracker\.claude\CLAUDE.md` "Definition of Done" — `go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check` (local-only, no CI counterpart) since this phase adds a sqlc query.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/notifier/format.go`'s per-type formatters, emoji constants, `truncateRunes`, `tracksFieldValue`, and URL builders — D-04/D-05/D-07's grouped-Description text is built from these, not new formatting logic.
- `internal/authgate/gate.go`'s `sweepLoop`/`NewManager`/`Close` pattern — a `time.Ticker` + `done` channel + `sync.Once`-guarded `Close`, exactly the shape D-08–D-10's scheduler goroutine needs (including being closeable in tests, mirroring `cmd/server`'s existing `defer gate.Close()`-style cleanup).
- `internal/notifier.Notifier`'s existing `notifying` CAS lock, `dbOpTimeout`, `listUnnotified`/`markNotified` helpers — the digest send reuses these rather than inventing a parallel set.

### Established Patterns
- Consumer-declared narrow seams (`Sender`, `SettingsReader`, `Sink` in `notifier.go`) — any new seam this phase needs follows the same declared-in-the-consumer convention.
- `slog` structured logging: `logger.Info`/`logger.Warn` with typed fields, summary lines not per-row lines — the new due-check/skip-empty-window logs should match.
- Functional options (`Option func(*Notifier)`) for tunables that vary per environment — D-08 deliberately does NOT use this pattern for the check interval, since it's not meant to vary.

### Integration Points
- `internal/notifier/notifier.go` — new digest-send method + scheduler goroutine, sharing `notifying`.
- `internal/settings/` + `queries/notification_settings.sql` + regenerated sqlc output — new watermark-write query/method.
- `cmd/server/main.go` — start the scheduler goroutine, blank-import `time/tzdata`.
- `internal/discord/client.go` / `Embed` — no interface change expected, D-05 reuses `Send(ctx, Embed)` with one richly-populated `Description`.

</code_context>

<specifics>
## Specific Ideas

- Digest Description text shape (D-05): `**New Releases**\n- [Artist — Title](url)\n- [Artist — Title](url)\n**Guest Features**\n- ...\n**Deluxe Changes**\n- [Artist — Title](url) (12→15 tracks)\n...` — a group heading is omitted entirely when that type has zero events in the window (never an empty heading with nothing under it).
- 00:05 America/New_York (daily) / Friday 00:05 America/New_York (weekly) are the exact literal fire times — document them plainly in code comments and in any operator-facing copy Phase 23 or this phase adds, since REQUIREMENTS.md requires the fixed time be documented (not just implemented).

</specifics>

<deferred>
## Deferred Ideas

- The "since &lt;timestamp&gt;" window header and multi-message chunking for an oversized digest — Phase 23, by design (RESEARCH gap already closed: DGST-10 grouping was deliberately pulled forward into this phase so Phase 23 doesn't have to retrofit chunking onto an ungrouped flat digest).
- Operator-configurable fire time / timezone picker — out of scope for v1.5 per REQUIREMENTS.md; D-01–D-03's fixed values are the answer for this milestone.

### Reviewed Todos (not folded)
- *Resolve the D-15 previous-release schema/query files from `--prev-tag`* — `cmd/migration-check` tooling, unrelated to the digest scheduler; weak keyword match only (score 0.6). Already reviewed-and-deferred identically in Phase 20's discussion.
- *Unify `internal/sqlscan`'s two quote/dollar-quote state machines* — backend tooling cleanup, unrelated; weak keyword match only (score 0.6).
- *Move `shadcn` from dependencies to devDependencies in `web/package.json`* — frontend tooling cleanup, unrelated; weak keyword match only (score 0.4).

</deferred>

---

*Phase: 22-scheduled-digest-send*
*Context gathered: 2026-09-16*
