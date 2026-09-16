# Phase 5: Discord Notifications - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-08
**Phase:** 5-Discord Notifications
**Areas discussed:** Visual distinction & content, Notification cadence/trigger, Burst handling & failure behavior, Webhook-unset behavior

---

## Visual distinction & content

**Q: How should the three event types be visually distinguished in Discord?**

| Option | Description | Selected |
|--------|-------------|----------|
| Color-coded embeds + emoji title prefix | Distinct embed side-bar color per type plus emoji in title | ✓ |
| Color only, no emoji | Just the embed color-bar distinguishes type | |
| Separate channels/webhooks per type | Route each event type to a different Discord channel | |

**Q: What fields should a guest_feature notification show?**

| Option | Description | Selected |
|--------|-------------|----------|
| Recording title + primary artist + link | Uses existing event snapshot data, no extra fetch | ✓ |
| Recording title + full artist-credit list | More complete but snapshot doesn't store full credit list | |
| Minimal: just recording title | Loses "featured with" context | |

**Q: What fields should a deluxe_change notification show?**

| Option | Description | Selected |
|--------|-------------|----------|
| Release title + old → new track count + link | Uses detector's computed count jump | ✓ |
| Release title + link only | Loses "what changed" detail | |
| Release title + full new tracklist | Needs new fetch at notify time, adds latency | |

**Q: The old track count isn't persisted anywhere today — how should the notifier get it to show the '12 → 18' delta?**

| Option | Description | Selected |
|--------|-------------|----------|
| Add a previous_track_count column | New migration, Phase 4 detection code writes it at insert time | ✓ |
| Bake the delta into the title text at insert time | No schema change, but mangles clean release title | |
| Drop the delta, show only the new count | Simplest, loses the growth signal | |

**Notes:** This question emerged mid-discussion after tracing `internal/detection/musicbrainz.go`'s deluxe-change insert path — the `track_count` column is mutable baseline state (overwritten on each change), not a delta record. Confirmed with user before locking D-03's field choice.

---

## Notification cadence/trigger

**Q: When should pending events actually get posted to Discord?**

| Option | Description | Selected |
|--------|-------------|----------|
| Inline at end of each poll cycle | Reuses existing per-source cron, no new config | ✓ |
| Dedicated notifier cron on its own interval | Lower latency, but more moving parts and new config | |

**Q: How should double-posting be prevented given both cycles could call the notifier concurrently against the same global ListUnnotified query?**

| Option | Description | Selected |
|--------|-------------|----------|
| One shared notifier mutex, called by both cycles | Mirrors existing mbRunning/dzRunning pattern | ✓ |
| Row-level locking via SELECT ... FOR UPDATE SKIP LOCKED | Also correct for multi-instance, but new SQL pattern | |

**Notes:** This race was identified by cross-referencing `ListUnnotified`'s unscoped query shape against Phase 3 D-08's independent-concurrent-cycles decision — not raised by the user unprompted, but confirmed as a real risk worth guarding against once surfaced.

---

## Burst handling & failure behavior

**Q: How should multiple pending events be sent when several are unnotified at once (e.g. a busy Friday)?**

| Option | Description | Selected |
|--------|-------------|----------|
| One message per event, serial with spacing | Matches PITFALLS.md Pitfall 5 guidance directly | ✓ |
| Batch up to 10 events into one message | Fewer API calls, but complicates visual distinction | |

**Q: When a Discord send fails (network error, 5xx, or a 429 that exhausts retries), what should happen to that event?**

| Option | Description | Selected |
|--------|-------------|----------|
| Leave notified_at NULL, log error, retry next cycle | Matches existing outbox design, no new state | ✓ |
| Mark notified_at anyway after N failed attempts | Needs new retry-count column, risks silent drop | |

---

## Webhook-unset behavior

**Q: DISCORD_WEBHOOK_URL is optional (no notEmpty constraint) — what should happen when it's unset?**

| Option | Description | Selected |
|--------|-------------|----------|
| Boot normally, notifier disabled, log once at startup | Matches existing optional field design | ✓ |
| Require it — fail fast at startup if unset | Reverses Phase 1's D-06/D-07 decision | |

---

## Claude's Discretion

- Exact embed color values (hex) and emoji choices per event type
- Exact spacing duration between serial sends, and precise 429/backoff shape within one send
- Whether the shared notifier guard is a `sync.Mutex` or `atomic.Bool`
- Go project layout for the new Discord client/notifier package and where the shared guard lives
- Structured log field names for notifier log lines

## Deferred Ideas

None — discussion stayed within phase scope.
