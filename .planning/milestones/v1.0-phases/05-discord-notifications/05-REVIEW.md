---
phase: 05-discord-notifications
reviewed: 2026-08-08T21:27:25Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/server/main.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000004_events_display_fields.down.sql
  - internal/db/migrations/000004_events_display_fields.up.sql
  - internal/db/sqlc/events.sql.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/querier.go
  - internal/detection/deezer.go
  - internal/detection/deezer_test.go
  - internal/detection/detector_test.go
  - internal/detection/musicbrainz.go
  - internal/detection/musicbrainz_test.go
  - internal/discord/client.go
  - internal/discord/client_test.go
  - internal/notifier/format.go
  - internal/notifier/format_test.go
  - internal/notifier/notifier.go
  - internal/notifier/notifier_test.go
  - internal/poller/poller.go
  - internal/poller/poller_test.go
  - queries/events.sql
findings:
  critical: 1
  warning: 4
  info: 3
  total: 8
status: issues_found
---

# Phase 05: Code Review Report

**Reviewed:** 2026-08-08T21:27:25Z
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

This phase wires the Discord notification delivery path: `internal/discord` (webhook
client), `internal/notifier` (outbox drain + embed formatting), the `previous_track_count`
/ `release_type` display-snapshot columns (migration 000004), and their wiring into
`internal/detection` and `cmd/server/main.go`. The core outbox/ack/CAS-guard/retry
mechanics are well covered by tests and largely correct — dedup, idempotent
`MarkNotified`, per-row error isolation, and rune-safe truncation all hold up under
review.

The one finding that must block shipping is a genuine security gap: embed field values
(specifically `ArtistName`, sourced from MusicBrainz/Deezer — both explicitly documented
elsewhere in this codebase as "community-editable, semi-trusted data") are sent to
Discord's webhook API without an `allowed_mentions` suppression, so a vandalized or
maliciously-registered artist name containing `@everyone`, `@here`, or a role/user mention
token can trigger a live mass-ping in the destination Discord server/channel. This is a
well-known Discord webhook gotcha with a trivial, low-risk fix (send
`"allowed_mentions":{"parse":[]}` on every request), and the codebase's own comments show
the authors were already alert to MusicBrainz/Deezer data being untrusted for other
purposes (URL-building, length limits) but missed this one.

The remaining findings are warnings/info around rate-limit pacing edge cases, a
documented-but-real duplicate-notification risk on a rare DB failure window, and minor
observability gaps — none of which block shipping but should be tracked.

## Critical Issues

### CR-01: Discord webhook payload never suppresses mention parsing — community-editable artist/title data can trigger @everyone/@here pings

**File:** `internal/discord/client.go:53-55` (payload shape), consumed via `internal/notifier/format.go:84,106,118` (Field population)
**Issue:**

`webhookPayload` only ever carries `Embeds`:

```go
type webhookPayload struct {
	Embeds []Embed `json:"embeds"`
}
```

No `allowed_mentions` field is ever set. Discord's webhook-execute endpoint's documented
default behavior, absent an explicit `allowed_mentions` object, is to parse mentions found
in the message content **and in embed description/field values** — and convert
`@everyone`, `@here`, `<@userID>`, and `<@&roleID>` tokens into real, notification-triggering
pings in the destination channel.

`internal/notifier/format.go` populates exactly that vulnerable surface with external,
community-editable data on every event type:

```go
embed.Fields = appendField(embed.Fields, "Artist", ev.ArtistName)   // formatNewRelease, formatGuestFeature
```

`ev.ArtistName` originates from MusicBrainz's artist-credit name or a release's display
name — both are explicitly called out elsewhere in this same codebase as
"community-editable, semi-trusted data" (e.g. `internal/detection/musicbrainz.go`'s
`isGuestFeature`/`displayArtistName` doc comments), i.e. anyone with a MusicBrainz account
can rename an artist-credit entry to `@everyone` or embed a raw role/user mention token,
and the very next poll cycle that detects a "new" recording/release credited to that name
will render it verbatim into a `Fields[].Value` and POST it to the configured webhook —
paging every member of the destination Discord server.

This directly undermines the operational goal of this phase (quiet, scannable release
alerts) by turning an upstream data-quality/vandalism issue into a live spam/social-
engineering vector against the operator's own Discord server, and it is exactly the class
of gap ASVS output-encoding requirements (already invoked elsewhere in this codebase, e.g.
T-05-06 for URL building) exist to catch.

**Fix:** Add an `AllowedMentions` field to the payload and always send an empty parse list
— this project has no legitimate use case for a notification ever pinging anyone:

```go
type allowedMentions struct {
	Parse []string `json:"parse"` // deliberately empty: never resolve any mention type
}

type webhookPayload struct {
	Embeds          []Embed         `json:"embeds"`
	AllowedMentions allowedMentions `json:"allowed_mentions"`
}
```

and set it unconditionally in `sendAttempt`:

```go
body, err := json.Marshal(webhookPayload{
	Embeds:          []Embed{embed},
	AllowedMentions: allowedMentions{Parse: []string{}},
})
```

Add a regression test asserting the marshalled request body always contains
`"allowed_mentions":{"parse":[]}`, and/or a test that an `ArtistName` of `"@everyone"`
survives formatting unescaped into the Field value (expected — the fix belongs at the
transport layer, not string-mangling display data) but the outbound payload still
disables mention resolution.

## Warnings

### WR-01: A failed `Send` skips the inter-send spacing entirely, defeating pacing exactly when it matters most

**File:** `internal/notifier/notifier.go:126-147`
**Issue:** `NotifyPending`'s loop only waits `n.spacing` after a **successful** send/ack:

```go
for i, ev := range events {
    embed := formatEmbed(ev)
    if err := n.sender.Send(ctx, embed); err != nil {
        logger.Error("notify send failed", ...)
        continue          // <-- no spacing wait on this path
    }
    if _, err := n.q.MarkNotified(ctx, ev.ID); err != nil {
        return fmt.Errorf("notifier: mark notified: %w", err)
    }
    if i < len(events)-1 {
        select {
        case <-time.After(n.spacing):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

`D-07`'s whole purpose is to keep this project's outbound rate under Discord's per-webhook
ceiling. A `continue` on a `Send` error (network error, 5xx, or a second 429 after the
client's single built-in retry is exhausted) skips the spacing wait, so a backlog of N
pending rows during a Discord outage or sustained rate-limit condition will fire N requests
back-to-back with zero pacing between them — the exact scenario where hammering the
upstream is most likely to make things worse (deepening a rate-limit ban, or generating a
burst of near-simultaneous failures that all get logged/retried on the very next pass).

**Fix:** Apply the spacing wait unconditionally between iterations (move it out of the
success-only path), or at minimum apply it on the error path too:

```go
for i, ev := range events {
    embed := formatEmbed(ev)
    sendErr := n.sender.Send(ctx, embed)
    if sendErr == nil {
        if _, err := n.q.MarkNotified(ctx, ev.ID); err != nil {
            return fmt.Errorf("notifier: mark notified: %w", err)
        }
    } else {
        logger.Error("notify send failed", ...)
    }
    if i < len(events)-1 {
        select {
        case <-time.After(n.spacing):
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

### WR-02: `defaultSpacing`'s own doc comment doesn't reconcile with the stated Discord rate-limit ceiling

**File:** `internal/notifier/notifier.go:24-27`
**Issue:**

```go
// defaultSpacing is the inter-send pause between consecutive events in one
// NotifyPending pass (D-07): inside the decision's stated 350-500ms band,
// comfortably under Discord's ~30-per-minute per-webhook ceiling.
const defaultSpacing = 400 * time.Millisecond
```

400ms spacing yields roughly 2.5 sends/second sustained, i.e. ~150 sends/minute — five
times the "~30-per-minute" ceiling the same comment cites as the reason this spacing is
safe. Either the cited ceiling is stale/wrong (Discord's actual per-webhook limit is
commonly reported closer to 5 requests/2 seconds ≈ 150/min, which *would* make 400ms
correctly sized) or the spacing constant is under-sized relative to the documented
assumption. As written, the comment and the constant contradict each other, which makes
this load-bearing constant hard to trust on a future edit.

**Fix:** Re-verify Discord's current documented per-webhook rate limit against the
execute-webhook endpoint's response headers (`X-RateLimit-Limit`/`X-RateLimit-Reset-After`)
or current developer docs, and correct whichever of the two (the comment's stated ceiling,
or `defaultSpacing`) is wrong so they agree.

### WR-03: A `MarkNotified` failure after a successful `Send` can cause a duplicate Discord post on the next pass

**File:** `internal/notifier/notifier.go:126-139`
**Issue:** If `n.sender.Send` succeeds (Discord has durably accepted and will display the
message) but the subsequent `n.q.MarkNotified` call fails (e.g. a transient DB connection
error), `NotifyPending` returns the error immediately and the row's `notified_at` stays
`NULL`. The very next pass (the next poll cycle, seconds to minutes later per
`PollInterval`) will re-select this same row via `ListUnnotified` and send it to Discord
again, producing a user-visible duplicate notification for the same event.

The code's own comment acknowledges this is an intentional trade-off ("there is no
well-defined 'keep going' outcome for a row Discord has already accepted but this process
failed to acknowledge in the database"), so this is a known limitation rather than an
oversight — but it's worth surfacing explicitly since there is no idempotency safeguard on
the Discord side (e.g., no dedup key embedded in the message) to catch the resend, and the
window (a DB error landing in the narrow gap between a successful `Send` and its
`MarkNotified` call) will eventually be hit in production over a long enough run.

**Fix:** No change required to ship, but consider: (a) logging at `Warn` (not just
returning the error) so this specific failure mode is distinguishable in production logs
from a generic `ListUnnotified`-level DB outage, and/or (b) documenting the acceptable
duplicate-notification risk in an operator-facing doc so a duplicate alert during a DB
blip isn't mistaken for a detection bug.

### WR-04: Unbounded/unclamped `retry_after` value from a 429 response can produce an unbounded wait

**File:** `internal/discord/client.go:127-133`
**Issue:**

```go
if resp.StatusCode == http.StatusTooManyRequests && allowRetry {
    var rl retry429Body
    _ = json.NewDecoder(resp.Body).Decode(&rl)
    wait := time.Duration(rl.RetryAfter * float64(time.Second))
    if wait <= 0 {
        wait = time.Second
    }
    select {
    case <-time.After(wait):
    case <-ctx.Done():
        return ctx.Err()
    }
    return c.sendAttempt(ctx, embed, false)
}
```

`rl.RetryAfter` is only floor-guarded against `<= 0`; there is no upper clamp. A malformed
or unexpectedly large `retry_after` value (e.g. a proxy/CDN error page mis-decoded as JSON,
or a future Discord API change) converted through `float64(time.Second)` into a
`time.Duration` has no ceiling, so `sendAttempt` could block for an extremely long or (on
float64→int64 overflow) effectively undefined duration, bounded only by whatever `ctx`
the caller happens to pass in (which, from `notifier.NotifyPending`, is the ambient poll
cycle's context — itself unbounded unless the process is shutting down).

**Fix:** Clamp `wait` to a sane maximum (e.g. 30s–60s) in addition to the existing
floor:

```go
const maxRetryAfter = 30 * time.Second
wait := time.Duration(rl.RetryAfter * float64(time.Second))
if wait <= 0 {
    wait = time.Second
} else if wait > maxRetryAfter {
    wait = maxRetryAfter
}
```

## Info

### IN-01: `MarkNotified`'s affected-row count is discarded

**File:** `internal/notifier/notifier.go:137`
**Issue:** `if _, err := n.q.MarkNotified(ctx, ev.ID); err != nil { ... }` ignores the
returned row count. Given the CAS guard, this should always be `1`; a `0` would silently
indicate a same-row race or a logic bug elsewhere (e.g. the row was already marked
notified by something else) and currently goes completely unobserved.
**Fix:** Log at `Warn` (not treated as an error) when the affected count is `0`, to make an
otherwise-invisible invariant violation visible in production logs.

### IN-02: `EmbedField.Inline` is defined but never set to `true` anywhere

**File:** `internal/discord/client.go:40-44`, `internal/notifier/format.go`
**Issue:** `appendField` never sets `Inline`, so every field always renders full-width. Not
a bug, but the field exists in the type with no call site ever using it — either intended
for future use or dead capability worth a one-line comment noting it's deliberately unused
for now.
**Fix:** No functional change needed; optionally add a short comment on `EmbedField.Inline`
noting it's currently unused, so a future reader doesn't wonder if it's a bug.

### IN-03: `formatEmbed`'s unreachable default branch would silently produce a colorless, linkless embed if ever hit

**File:** `internal/notifier/format.go:66-72`
**Issue:** The `default` case is documented as unreachable given the DB's
`events_event_type_valid` CHECK constraint, and returns a safe-but-uninformative
`discord.Embed{Title: ...}` with no color, URL, or fields. If this branch is ever actually
reached (e.g. a future migration adds a new `event_type` value that this switch isn't
updated for), the failure mode is a silently degraded, unbranded Discord message rather
than a loud signal that formatting fell behind the schema.
**Fix:** Consider having the default branch also log a warning (it currently has access to
neither a logger nor the calling context, since `formatEmbed` is pure — this would require
threading a logger through, or having the caller check `ev.EventType` against the known set
before calling `formatEmbed` and logging there). Low priority given the CHECK constraint
backstop.

---

_Reviewed: 2026-08-08T21:27:25Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
