# Feature Research

**Domain:** Digest/batch notification mode for a single-operator self-hosted release tracker (drop-tracker v1.5)
**Researched:** 2026-09-11
**Confidence:** MEDIUM (cross-checked against multiple independent digest-system vendors/guides; no single canonical spec exists for this feature class, and the closest sibling OSS domain — Sonarr/Radarr — has no first-party implementation to benchmark against directly)

## Scope Note

This is subsequent-milestone research scoped to one feature area, not a full domain survey. drop-tracker already has: real-time per-event Discord notifications, a Postgres `events` table with 90-day soft-delete retention (`created_at`-based, dedup keys/deluxe-baselines preserved unfiltered), a watchlist, `robfig/cron` scheduling, and a gated `GET /status` operator panel. Findings below assume that substrate and do not re-litigate stack/architecture choices already locked in PROJECT.md.

## Feature Landscape

### Table Stakes (Users Expect These)

Features an operator turning on "digest mode" would assume exist. Missing these makes the toggle feel half-built.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| On/off toggle, persisted server-side (Postgres, not env var) | Milestone spec explicitly requires SPA-configurable, no-redeploy setting — matches how every commercial digest feature (SuprSend, Novu, NotificationAPI) exposes it as a live preference, not a build-time flag | LOW | One settings row/table; already decided in PROJECT.md scope — reinforced, not new research |
| Fixed wall-clock cadence (daily / weekly), not a rolling window | Every scheduled-summary product (Sonarr/Radarr third-party digest add-ons, RSS-to-email tools like Digest/Mailbrew, generic digest vendors) uses "fires at a fixed time" for day/week-granularity digests. Rolling/event-driven windows (start on first event, collect for N minutes) are the *other* pattern in the literature, but only ever seen at minute/hour granularity for "batch comments together" use cases — never for daily/weekly cadences. Daily-or-weekly is squarely fixed-schedule territory | LOW–MEDIUM | Pick one fixed time-of-day (e.g. configurable hour, default reasonable UTC hour) for daily; one day-of-week + hour for weekly. `robfig/cron` already expresses this natively as a cron expression — no new scheduling primitive needed |
| Skip-send on zero events (no empty digest) | Universal, unquestioned convention across every source found (Knock's alert-digest template, generic alert-digest guidance, and a concrete reference implementation's `if events: send_digest()` gate). No vendor treats "send an empty digest" as a real option | LOW | A `COUNT(*)` guard before posting to Discord; cheapest correctness win in the whole feature |
| Chronological ordering within the digest as the baseline | The one consistent finding across RSS-digest and alert-digest sources: absent a stronger signal, order by time (`created_at`), preserving insertion order. This is the "safe default" every source falls back to when no fancier grouping is implemented | LOW | `ORDER BY created_at` on the query already used for `GET /events`/retention filtering — no new index needed beyond what retention already relies on |
| Digest reflects all three existing event types together in one message | Milestone spec explicitly requires new-release, guest-feature, and deluxe-change events to batch into the *same* scheduled message, not three separate digests | LOW–MEDIUM | The `events` table already carries a type discriminator (used today to render distinct Discord embeds per type in real-time mode); digest mode reuses the same read path, just batches the write |
| Default stays real-time/off | Matches today's behavior; also matches the general digest-vendor pattern of "opt-in batching, not opt-in real-time" — batching is the deviation from default, not the other way around | LOW | Already locked in PROJECT.md scope |

### Differentiators (Competitive Advantage)

Not required to ship a working toggle, but meaningfully better than the bare minimum — and cheap relative to the existing codebase's design bar (e.g. Phase 13's fail-closed artist-art matching, Phase 15's dual-purpose coverage tool).

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Group-by-artist (or group-by-event-type) within the digest body, chronological *within* each group | The more sophisticated digest tools (email-digest best-practice guidance, cross-source RSS mergers) move past flat chronological lists toward "clear hierarchy" — grouping reduces scanning effort in a message that could contain a week's worth of releases across many watched artists. Sonarr/Radarr's own third-party digest add-ons (Bettarr-Notifications) exist specifically because flat unstructured batches feel worse than grouped ones | LOW–MEDIUM | Pure presentation-layer change over the same query — group in Go before building the Discord payload, no schema change |
| A visible "since [last digest timestamp]" header/footer in the digest message | No vendor explicitly documents this, but it directly solves the ambiguity every source flags around mode-switching and window boundaries — telling the operator exactly what the digest covers builds trust that nothing was silently dropped or double-counted | LOW | Requires storing `last_digest_sent_at` (see Dependencies) — already needed for the query itself, so surfacing it in the message body is nearly free |
| Discord embed chunking for large digests | Discord hard-caps a single message at 6000 total embed characters and 10 embeds per message (existing real-time notifier already respects Discord's per-embed field limits per the stack decisions). A watchlist with many artists + a weekly cadence could plausibly accumulate more events than one message can hold | MEDIUM | Not optional once volume is realistic — split into multiple sequential webhook POSTs if the batch exceeds Discord's limits. This is the one piece of real new complexity in the feature; flag for phase-level research/design (rate-limit-aware multi-message send, ordering preserved across the split) |

### Anti-Features (Commonly Requested, Often Problematic)

Explicitly already fenced off by the milestone's own "Out of scope this cycle" list — corroborated here by what the wider ecosystem treats as scope creep for a v1 digest feature.

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|------------------|-------------|
| Per-event-type digest overrides (e.g. "batch releases but keep guest-features real-time") | Commercial digest platforms (per-category digest frequency in a preference center) support this because they serve many users with different tolerances | Single-operator instance = one taste, not many; the milestone's own scoping already rejected this (Out of scope). Preference-center-style per-category logic also multiplies the window/watermark bookkeeping this research flags as the trickiest part of even the single-toggle version | One instance-wide toggle + cadence, as scoped |
| Rolling/event-driven window ("batch for N minutes after first event") instead of fixed daily/weekly schedule | Seems more "responsive" than a fixed clock time | Wrong granularity — rolling windows are documented for minute/hour-scale grouping (e.g. Slack-notification-style debouncing), not day/week digests; adds a stateful "is a window currently open" tracking problem the fixed-schedule model avoids entirely | Fixed wall-clock cron schedule, as scoped |
| Multi-channel digest sinks (RSS/webhook/email) | "Since we're batching anyway, why not offer other delivery channels" | Explicitly parked (Option A) in the milestone scope; orthogonal to the batching logic itself — conflating them risks scope creep into the digest phase | Discord-only for this milestone; multi-channel is a separable future milestone |
| Send an empty "no news this week" digest | Feels reassuring ("the bot is alive") | Contradicts the universal skip-send convention found across every source; also duplicates what `/status`'s existing per-source last-run summary (shipped v1.4) already tells the operator — an empty digest would be redundant liveness signaling wearing a notification's clothes | Rely on the existing `/status` panel for "is this thing still running"; digest stays strictly content-gated |
| Retroactively re-including events that already fired as real-time notifications before a mid-window toggle to digest mode | Feels like "don't lose anything" | Would duplicate notifications the operator already saw — no source condones this; the one clear signal found is that digest-frequency settings are read "at batch-open time," i.e. prospectively | Toggle takes effect for the *next* window only; the query building each digest should key off `last_digest_sent_at` (see Dependencies), and toggling from real-time→digest should set that watermark to "now" at toggle time so nothing already-notified reappears |

## Feature Dependencies

```
[Instance-wide digest toggle + cadence setting] (Postgres, SPA-editable)
    └──requires──> [Settings persistence layer distinct from existing env-var config]
                       (new: no existing Postgres-backed, live-editable setting exists today —
                        everything else in the app is env-var/compile-time per PROJECT.md)

[Scheduled digest send] (robfig/cron, fixed wall-clock)
    └──requires──> [last_digest_sent_at watermark]
                       └──requires──> [events table already has created_at + soft-delete filtering]
                                          (Phase 10 retention machinery — reused, not rebuilt)
    └──requires──> [Digest toggle is ON at cron-fire time] (read fresh each fire, not cached)

[Skip-send on empty window] ──requires──> [last_digest_sent_at watermark]
    (same watermark powers both "what's in this digest" and "was it empty")

[Discord embed chunking] ──enhances──> [Scheduled digest send]
    (only triggers once accumulated-event count crosses Discord's per-message embed/char limits)

[Real-time notification path] ──conflicts──> [Scheduled digest send]
    (mutually exclusive at any given moment — the toggle selects exactly one active path;
     both paths must read the same toggle state to avoid double-notifying)
```

### Dependency Notes

- **Digest toggle requires a new settings persistence layer:** every existing piece of drop-tracker config is env-var-only (per CLAUDE.md/PROJECT.md constraints); this is explicitly called out in the milestone goal as new ("changeable without a redeploy, unlike the env-var-only config used elsewhere"). This is new schema/plumbing, not a reuse of an existing pattern — flag for phase-level design.
- **Scheduled send requires a `last_digest_sent_at` watermark:** this is the load-bearing piece of state for three separate behaviors — defining the query window (`WHERE created_at > last_digest_sent_at`), deciding skip-vs-send (row count from that same query), and correctly handling a mid-cycle toggle (set the watermark to "now" the moment digest mode is switched on, so already-real-time-notified events never reappear). One column, three jobs — get its update timing right and most of the feature's correctness follows.
- **Real-time conflicts with scheduled send:** the two paths must never both fire for the same event. Concretely, this likely means the existing real-time notifier call becomes conditional on the toggle's current state (checked per poll cycle, not cached at boot) — the toggle needs to be read live, since it's now a runtime Postgres value rather than a boot-time env var.
- **Discord embed chunking only matters at scale:** with a modest watchlist and daily cadence this may never trigger; with a large watchlist and weekly cadence it's likely to. Worth a complexity flag for phase research rather than building it defensively on day one — but the query/grouping logic should be shaped so chunking can be added without a rework (e.g. build a list of embed objects up front, chunk at send time).

## MVP Definition

### Launch With (v1.5, this milestone)

- [ ] Instance-wide digest toggle (off/daily/weekly) — Postgres-persisted, SPA-editable — essential per milestone goal
- [ ] Fixed wall-clock cron schedule for daily and weekly cadences — matches the milestone's stated cadence granularity and every relevant precedent found
- [ ] `last_digest_sent_at` watermark, updated at send time (or at toggle-on time) — the one piece of state everything else depends on
- [ ] Skip-send when the watermark-to-now window has zero events — universal convention, cheap, prevents empty-digest noise
- [ ] All three event types (release/guest-feature/deluxe-change) combined into one digest message, chronologically ordered — core of the milestone goal
- [ ] Toggle applies prospectively — flipping to digest mode sets/resets the watermark so nothing already real-time-notified is repeated

### Add After Validation (v1.5.x or a fast-follow)

- [ ] Group-by-artist (or by event-type) presentation within the digest body — pure formatting improvement once the base send path works
- [ ] "Since [timestamp]" header/footer in the digest for operator trust/debuggability
- [ ] Discord multi-message chunking for digests that exceed embed limits — build once real usage shows whether this is ever actually hit

### Future Consideration (v2+, or a separate milestone)

- [ ] Per-event-type digest overrides — explicitly out of scope this cycle; would need the per-category preference-center pattern this research flags as meaningfully more complex
- [ ] Multi-channel digest delivery (RSS/email/webhook) — parked Option A, orthogonal to the batching engine itself
- [ ] Configurable/rolling digest windows — no clear precedent at daily/weekly granularity; only relevant if the product ever wants sub-hour batching for a different use case

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|----------------------|----------|
| Digest toggle + cadence, Postgres-persisted, SPA-editable | HIGH | MEDIUM (new settings-persistence pattern for this codebase) | P1 |
| Fixed wall-clock cron schedule (daily/weekly) | HIGH | LOW (robfig/cron already in stack) | P1 |
| `last_digest_sent_at` watermark + skip-on-empty | HIGH | LOW | P1 |
| Combined multi-event-type digest, chronological | HIGH | LOW–MEDIUM (reuses existing events query/retention path) | P1 |
| Real-time/digest mutual-exclusion at send time | HIGH (correctness-critical — this is the difference between "digest mode" and "digest mode plus duplicate real-time pings") | MEDIUM | P1 |
| Group-by-artist/event-type presentation | MEDIUM | LOW | P2 |
| "Since [timestamp]" digest header | LOW–MEDIUM | LOW | P2 |
| Discord multi-message chunking | MEDIUM (only matters at scale) | MEDIUM | P2 |
| Per-event-type digest overrides | LOW (single-operator instance) | HIGH | P3 (deferred) |
| Multi-channel digest sinks | LOW this cycle | HIGH | P3 (deferred) |

**Priority key:**
- P1: Must have for the milestone to deliver its stated goal
- P2: Should have, straightforward fast-follow once P1 lands
- P3: Explicitly deferred per milestone scope

## Comparable-Product Feature Analysis

No canonical single competitor exists for "self-hosted music-release Discord tracker with a digest toggle" — the closest sibling domain (media-release *arr stack) and the general digest-vendor space were both surveyed instead.

| Feature | Sonarr/Radarr (`*arr` ecosystem) | Commercial digest platforms (SuprSend/Novu/NotificationAPI) | drop-tracker's approach |
|---------|-----------------------------------|----------------------------------------------------------------|--------------------------|
| Native digest/batch mode | None — real-time-only by design; digesting only exists via third-party middleware sitting between the app and Discord | Core product feature, often per-category and multi-window | First-party, instance-wide (not per-category — single-operator doesn't need it) |
| Window model | N/A (no native digest) | Both fixed-schedule and rolling/event-driven, chosen per use case | Fixed wall-clock only (daily/weekly) — matches the requested cadence granularity |
| Empty-window behavior | N/A | Skip-send (universal convention) | Skip-send |
| Event grouping in a batch | Ad-hoc, tool-dependent (Bettarr-Notifications etc.) | Chronological baseline; hierarchy/grouping in more mature tools | Chronological at launch; grouping as a fast-follow |

## Sources

- SuprSend — "How Notification Batching and Digests Actually Work" (https://www.suprsend.com/post/notification-batching-and-digest) — fixed-schedule vs rolling-window distinction, batch-on-read/batch-on-write — MEDIUM confidence (cross-checked)
- Novu — "Best Practices – How to Not Over Notify Your Users" (https://novu.co/blog/digest-notifications-best-practices-example/) — per-category digest frequency, batch-open-time preference reads — MEDIUM confidence
- NotificationAPI — "Batching & Digest" docs (https://www.notificationapi.com/docs/features/digest) — window/schedule mechanics — MEDIUM confidence
- Knock — "Build alert digest notifications" template library (https://knock.app/template-library/workflows/alert-digest) — skip-send-on-empty convention — MEDIUM confidence
- OneUptime — "How to Build a Notification Digest System with Redis" (https://oneuptime.com/blog/post/2026-03-31-redis-notification-digest-system/view) — concrete reference implementation: chronological ordering, `if events: send()` gate, schema shape for a Postgres-backed equivalent — MEDIUM confidence
- Readless / Digest / Mailbrew coverage of RSS-to-email digest tools (https://www.readless.app/blog/best-email-digest-services-2026, https://usedigest.com/features/rss-to-email-digest/) — chronological default + hierarchy/grouping in mature tools, cross-source dedup merging — MEDIUM confidence
- Sonarr/Radarr Discord notification ecosystem survey (GitHub: NiNiyas/Bettarr-Notifications, samwiseg0/better-discord-notifications; hotio.dev Arr Discord Notifier; notifiarr.wiki) — confirms no native digest mode in the closest sibling OSS domain, digesting is third-party middleware only — MEDIUM confidence
- General digest-vendor guidance on mode-switching/preference-read timing (Novu best-practices post; no single authoritative source addresses drop-tracker's exact mid-cycle real-time→digest toggle scenario) — LOW confidence, treated as directional only, not prescriptive

---
*Feature research for: digest/batch notification mode, drop-tracker v1.5*
*Researched: 2026-09-11*
