# Phase 5: Discord Notifications - Research

**Researched:** 2026-08-08
**Domain:** Discord webhook delivery (hand-rolled Go HTTP client) wired into an existing outbox-pattern poller
**Confidence:** MEDIUM — Discord's official docs confirm the request/response shape and rate-limit *headers*; the exact per-webhook numeric bucket size is not officially documented (community-corroborated only, matching this project's own PITFALLS.md). All codebase integration points were read directly this session.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

- **D-01:** Each event type gets a distinct embed side-bar color plus an emoji prefix in the message title (e.g. 🆕 new_release, 🎤 guest_feature, 💿 deluxe_change) — not color-only, and not separate channels/webhooks per type. One webhook URL handles all three types.
- **D-02:** A `guest_feature` notification shows the recording title, the primary credited artist (`artist_name` snapshot), and a link to the recording on MusicBrainz — no fetch of the full artist-credit list.
- **D-03:** A `deluxe_change` notification shows the release title, the old→new track-count delta (e.g. "12 → 18 tracks"), and a link to the release.
- **D-04:** A new nullable `previous_track_count` column on `events`, populated by Phase 4's `internal/detection/musicbrainz.go` deluxe-change insert path (capturing the pre-update `baseline` value before it's overwritten). Reversible — additive column.
- **D-05:** Notifications are sent inline, at the end of each poll cycle (`RunMusicBrainzCycle`/`RunDeezerCycle`), not via a separate cron job. No new `Config` field, no new cron entry.
- **D-06:** A single shared mutex (or equivalent atomic guard) wraps the whole "fetch pending → post → mark notified_at" sequence, called by both cycles, mirroring `mbRunning`/`dzRunning`. Reversible — a future `SELECT ... FOR UPDATE SKIP LOCKED` swap replaces this guard without touching calling cycles.
- **D-07:** Multiple pending events are sent as separate Discord messages, one event per message, serially with spacing (~350–500ms) — not batched into multi-embed messages.
- **D-08:** A 429's `Retry-After` must be honored before any retry within a single send attempt — exact retry/backoff shape left to research/planning.
- **D-09:** A send that ultimately fails leaves `notified_at` NULL and logs the failure — no retry-count column, no give-up-after-N-attempts logic. Next cycle's `ListUnnotified` re-picks the row automatically.
- **D-10:** `DISCORD_WEBHOOK_URL` stays optional (no `notEmpty`). Unset → app boots normally, notify step no-ops, one log line at startup.

### Claude's Discretion

- Exact embed color values (hex) and emoji choices for each event type.
- Exact spacing duration between serial sends (D-07) and the precise retry/backoff shape for a single send's 429 handling (D-08).
- Whether the shared notifier guard (D-06) is a `sync.Mutex` or an `atomic.Bool` CAS.
- Exact Go project layout for the new Discord client/notifier package (`internal/discord`) and where the shared mutex/guard lives (`Poller` struct vs. a new `Notifier` type it holds).
- Structured log field names for notifier log lines.

### Deferred Ideas (OUT OF SCOPE)

None — discussion stayed within phase scope. No scope-creep suggestions came up during discussion.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| NTFY-01 | Post a Discord webhook message for each new-release event, including title, artist, cover art, release date, and release type | Embed schema (below) gives the exact JSON shape; **Gap Finding G-1** below identifies that "release type" has no backing column today and must be added alongside D-04's column in the same migration |
| NTFY-02 | Post a visually distinct Discord webhook message for guest-feature events | D-02 + embed color/emoji scheme + `events.external_id` (recording MBID) gives the MusicBrainz recording link |
| NTFY-03 | Post a visually distinct Discord webhook message for deluxe/tracklist-change events | D-03 + D-04's `previous_track_count` gives the delta; `events.external_id` (release MBID, D-10 04-CONTEXT) gives the release link |
| NTFY-04 | Suppress notifications for muted artists/release-types | Already fully satisfied by Phase 4 D-17/D-18 (filtering happens before a row is ever inserted) — nothing to build here, only to confirm `ListUnnotified` never surfaces a muted event, which is true by construction since a muted event never becomes a row |
</phase_requirements>

## Summary

This phase wires actual HTTP delivery onto a transactional-outbox mechanism Phase 4 already built correctly: `events` rows with `notified_at IS NULL` are the queue, `ListUnnotified` is the dequeue, and marking `notified_at` after a successful POST is the ack. Nothing about the *detection* or *outbox* design needs to change — the work is (1) a small, additive schema change to capture two display fields Phase 4's detection code computes but never persists, (2) a hand-rolled `internal/discord` webhook client mirroring the existing `internal/musicbrainz`/`internal/deezer` client shape exactly, and (3) an orchestration layer (a new `internal/notifier` package is recommended) that fetches pending rows, formats them into per-event-type embeds, sends them serially with spacing, and marks each row notified on success.

Two concrete findings drove this research past what CONTEXT.md already locked. First, **Discord's webhook execute endpoint returns `204 No Content` on success, not `200`** — a client that checks for `200` will treat every successful send as a failure. Second, **NTFY-01's "release type" field has no backing column on `events` today** — `PrimaryType`/`RecordType` are used only as an in-memory filter predicate in Phase 4's detection code and are never written to the row (confirmed by reading `internal/detection/musicbrainz.go` and `internal/detection/deezer.go` in full). This must be added as a new nullable `release_type` column in the same migration as D-04's `previous_track_count`, populated at insert time from the exact same normalized value the filter already computes.

**Primary recommendation:** Add both missing columns in one migration `000004_events_display_fields` (mirroring the zero-padded numeric-prefix convention already established); build `internal/discord` as a pure webhook client (typed `Embed`, `Send` method, 204-success check, 429-retry-once-honoring-`Retry-After`); build a new `internal/notifier` package that owns the shared guard, the fetch/format/send/mark loop, and implements a `poller.Notifier` seam declared in `poller.go` exactly like `EventRecorder` is.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Fetch pending events (`ListUnnotified`) | API / Backend (`internal/notifier`) | Database / Storage | Read-side of the outbox pattern; a single-process background job, not exposed via HTTP |
| Format event → Discord embed | API / Backend (`internal/discord` or `internal/notifier`) | — | Pure in-process transformation, no external call |
| POST to Discord webhook | API / Backend (`internal/discord`) | External service (Discord) | Outbound HTTP call from the single Go binary — this project has no separate notifier microservice (CLAUDE.md: single-binary architecture) |
| Mark `notified_at` | Database / Storage | API / Backend | Write-side of the outbox pattern; must only happen after a 204 confirms delivery (D-09) |
| Shared cross-cycle guard (D-06) | API / Backend (`internal/notifier` or `Poller`) | — | In-process coordination between two goroutines in the same binary; no distributed-lock concern at v1 scale (single instance) |
| Trigger cadence | API / Backend (`internal/poller`) | — | Reuses the existing cron-driven cycle machinery (D-05); no new scheduler surface |

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `net/http` (stdlib) | Go 1.26 (this module's pinned version, `go.mod:3`) | Webhook POST | CLAUDE.md mandates hand-rolled `internal/discord`, no webhook-client library — a single `POST` with a JSON body needs nothing else |
| `encoding/json` (stdlib) | Go 1.26 | Embed payload marshaling, 429 body decoding | No third-party JSON library used anywhere else in this codebase either |
| `log/slog` (stdlib) | Go 1.26 | Structured notifier log lines | Matches every other package in this codebase (`internal/poller`, `internal/detection`) |

No new third-party Go module is required for this phase — confirmed no existing import of a webhook/Discord SDK anywhere in `go.mod` `[VERIFIED: C:/CodeProjects/drop-tracker/go.mod:1-22]` (full file read this session; only `caarlos0/env`, `chi`, `httplog`, `golang-migrate`, `pgerrcode`, `pgx`, `cron`, `golang.org/x/time` are direct deps).

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `time` (stdlib) | Go 1.26 | Serial-send spacing (D-07), 429 retry sleep (D-08) | `time.Sleep`/`time.After` bounded by ctx cancellation — see Code Examples |
| `sync`/`sync/atomic` (stdlib) | Go 1.26 | Shared notifier guard (D-06) | Either `sync.Mutex` or `atomic.Bool`; see Architecture Patterns for the recommendation |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled `internal/discord` | A Discord webhook Go module (e.g. community `discord-webhook` packages) | CLAUDE.md explicitly forbids this — a webhook is one `POST`, and most such libraries are full bot SDKs (gateway, slash commands) far beyond what's needed |
| `atomic.Bool` CAS-skip guard (D-06) | `sync.Mutex` blocking guard | A blocking mutex guarantees the *current* cycle's own newly-detected events get sent this cycle, but risks the faster Deezer cycle blocking briefly behind a slow/bursty MusicBrainz-triggered send batch — see Architecture Patterns for the full tradeoff and recommendation |

**Installation:**
No `go get`/`npm install` needed — every dependency used is already in `go.mod` or the Go standard library.

**Version verification:** N/A — no new package versions to verify (stdlib only).

## Package Legitimacy Audit

**Not applicable this phase.** No external packages are installed — `internal/discord` and `internal/notifier` are hand-rolled per CLAUDE.md, using only Go's standard library and packages already present in `go.mod` (confirmed by reading `go.mod` in full this session, `[VERIFIED: go.mod:1-22]`). The planner does not need a `checkpoint:human-verify` task for any package install in this phase.

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
                 ┌────────────────────────────────────────────────────────┐
                 │                     Poller (existing)                   │
                 │                                                          │
  cron tick ───▶ │  RunMusicBrainzCycle          RunDeezerCycle             │
                 │  (per-artist fetch+detect)    (per-artist fetch+detect)  │
                 │           │                          │                  │
                 │           └──────────┬───────────────┘                  │
                 │                      ▼ (end of cycle, D-05)              │
                 │            notifier.NotifyPending(ctx, logger)          │
                 └──────────────────────┬───────────────────────────────────┘
                                        │ CAS guard (D-06): skip if the
                                        │ other cycle is already notifying
                                        ▼
                          ┌─────────────────────────────┐
                          │   internal/notifier          │
                          │  1. ListUnnotified (sqlc)     │──▶ Postgres events table
                          │  2. for each row, serially:    │       (notified_at IS NULL)
                          │     format → discord.Embed     │
                          │     discord.Send(embed)         │──▶ POST /webhooks/{id}/{token}
                          │     on 204: MarkNotified (sqlc)  │       (Discord API)
                          │     on failure: log, leave NULL   │
                          │     sleep ~400ms before next row   │
                          └─────────────────────────────┘
```

A reader can trace one event end to end: a poll cycle detects and inserts a row with `notified_at = NULL` → the cycle's own trailing call into `internal/notifier` dequeues it via `ListUnnotified` → `internal/discord` POSTs one embed per row → a `204` response is the only signal that flips `notified_at` to non-NULL. A failure at any point (network error, non-204, exhausted 429 retry) leaves the row exactly as it was, and the *next* cycle's `NotifyPending` call picks it back up with no separate retry bookkeeping (D-09).

### Recommended Project Structure

```
internal/
├── discord/                # New. Hand-rolled webhook client (mirrors internal/musicbrainz, internal/deezer shape)
│   ├── client.go            # Client, NewClient, Embed/EmbedField types, Send method, 204 check, 429 retry
│   └── client_test.go       # httptest.Server-backed tests (mirrors search_test.go's pattern)
├── notifier/                # New. Orchestration: fetch pending → format → send → mark
│   ├── notifier.go           # Notifier struct, NewNotifier, NotifyPending, the shared CAS guard
│   ├── format.go              # event → discord.Embed per event_type (color/emoji table, link construction)
│   └── notifier_test.go        # Real-Postgres integration tests (mirrors detector_test.go's pattern)
├── poller/
│   └── poller.go              # Widened: new Notifier seam declared here (mirrors EventRecorder), called at end of both cycles
├── detection/
│   └── musicbrainz.go          # Modified: deluxe-change insert path gains PreviousTrackCount; new_release loop gains ReleaseType
│   └── deezer.go                # Modified: new_release insert gains ReleaseType (a.RecordType, already lowercase)
└── db/
    └── migrations/
        └── 000004_events_display_fields.{up,down}.sql   # New. Adds previous_track_count + release_type, both nullable
```

### Pattern 1: Narrow seam declared in the consumer (`poller.Notifier`)

**What:** `poller` package declares its own interface for whatever it needs from the notifier, exactly like it already does for `ReleaseGroupSource`, `AlbumSource`, and `EventRecorder` — never importing a concrete `*notifier.Notifier` type directly into its field type.

**When to use:** Always, in this codebase — it is the established convention across all four prior phases (`[VERIFIED: internal/poller/poller.go:45-73]`, `[VERIFIED: internal/detection/detector.go:21-39]`).

**Example:**
```go
// internal/poller/poller.go — add alongside EventRecorder
type Notifier interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}
```

### Pattern 2: Hand-rolled client `doRequest` funnel (mirror `internal/musicbrainz`/`internal/deezer`)

**What:** Every outbound Discord request goes through one unexported `doRequest`-style method so a shared concern (here: setting `Content-Type`, and — unlike MusicBrainz/Deezer — reading a 429 body and honoring `Retry-After`) can never be bypassed by a second call site.

**When to use:** `internal/discord.Client.Send` — this package only ever needs one outbound call shape (`POST` with a JSON body), so `doRequest` and `Send` can reasonably collapse into one method, unlike the read-heavy MusicBrainz/Deezer clients which separate them.

**Example (based on the confirmed request/response shape — see Sources):**
```go
// internal/discord/client.go
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embed is Discord's webhook embed object -- the fields this project uses.
// Color is a decimal RGB int, never a hex string (Discord ignores hex).
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Thumbnail   *EmbedImage  `json:"thumbnail,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"` // RFC3339
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedImage struct {
	URL string `json:"url"`
}

type webhookPayload struct {
	Embeds []Embed `json:"embeds"`
}

// retry429Body is Discord's documented 429 JSON body shape.
type retry429Body struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"` // seconds, may carry ms precision
	Global     bool    `json:"global"`
}

type Client struct {
	webhookURL string
	httpClient *http.Client
}

func NewClient(webhookURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{webhookURL: webhookURL, httpClient: httpClient}
}

// Send POSTs one embed as a single-embed message (D-07: one event per
// message). On a 429, it sleeps for retry_after (honoring ctx cancellation)
// and retries exactly once (D-08) -- a second 429 or any other failure
// returns an error and does not retry further, leaving D-09's "leave
// notified_at NULL, next cycle retries" contract as the outer safety net.
//
// Discord's execute-webhook endpoint returns 204 No Content on success
// (no ?wait=true is used, so 200 is never the success path here).
func (c *Client) Send(ctx context.Context, embed Embed) error {
	return c.sendAttempt(ctx, embed, true)
}

func (c *Client) sendAttempt(ctx context.Context, embed Embed, allowRetry bool) error {
	body, err := json.Marshal(webhookPayload{Embeds: []Embed{embed}})
	if err != nil {
		return fmt.Errorf("discord: marshal embed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Deliberately NOT wrapping the raw *url.Error here (see Pitfall
		// below) -- it embeds c.webhookURL, which carries the webhook's
		// secret token in its path.
		return fmt.Errorf("discord: send webhook: request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

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

	return fmt.Errorf("discord: send webhook: unexpected status %d", resp.StatusCode)
}
```

### Pattern 3: CAS-skip guard for the shared notify step (D-06)

**Recommendation:** `atomic.Bool` CAS-skip, NOT a blocking `sync.Mutex`.

**Rationale:** This codebase has an established, deliberate idiom of *never blocking or queueing* across the two poll cycles — `mbRunning`/`dzRunning` skip a whole cycle rather than wait for one (`[VERIFIED: internal/poller/poller.go:81-85]`, comment: "a mutex would serialise ticks into a backlog and eventually run every missed cycle back to back"). A blocking `sync.Mutex` on the notify step would reintroduce exactly that coupling in miniature: a slow, rate-limited Discord send burst triggered by the MusicBrainz cycle would make the Deezer cycle's own `RunDeezerCycle` call sit blocked waiting for the mutex, deepening its own cycle duration for a reason unrelated to Deezer's own API. A CAS-skip guard costs nothing here because `ListUnnotified` is global and cycle-independent (D-06's own justification) — if cycle B's `NotifyPending` call is skipped because cycle A is mid-send, cycle B's own newly-inserted rows are simply picked up by whichever cycle's `NotifyPending` runs next (mirrors D-09's "next cycle picks it up" contract one level up).

```go
// internal/notifier/notifier.go
type Notifier struct {
	q       sqlc.Querier
	sender  Sender
	notifying atomic.Bool
	spacing time.Duration
}

func (n *Notifier) NotifyPending(ctx context.Context, logger *slog.Logger) error {
	if !n.notifying.CompareAndSwap(false, true) {
		logger.Info("skipping notify pass: already in progress")
		return nil
	}
	defer n.notifying.Store(false)

	events, err := n.q.ListUnnotified(ctx)
	if err != nil {
		return fmt.Errorf("notifier: list unnotified: %w", err)
	}
	for i, ev := range events {
		embed := formatEmbed(ev)
		if err := n.sender.Send(ctx, embed); err != nil {
			logger.Error("notify send failed", slog.Int64("event_id", ev.ID), slog.String("error", err.Error()))
			continue // D-09: leave notified_at NULL, next cycle retries
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
	return nil
}
```

### Anti-Patterns to Avoid

- **Checking for HTTP 200 on a successful webhook POST:** Discord's execute-webhook endpoint returns `204 No Content` by default; `200` only appears when the caller opts in via `?wait=true`, which this project has no reason to use (no downstream code needs the created message's own id).
- **Logging the raw error from `httpClient.Do`:** Go's `net/http` wraps transport errors in `*url.Error`, whose `Error()` string embeds the full request URL — and the Discord webhook URL's path *is* the secret token (`https://discord.com/api/webhooks/{id}/{token}`). Logging that error verbatim leaks the webhook token into structured logs. See Common Pitfalls below.
- **Batching multiple events into one multi-embed message:** Locked out by D-07 already, but worth restating as an anti-pattern for this domain — it breaks the clean 1:1 color/emoji mapping D-01 relies on for scannability.
- **Sleeping a fixed interval instead of reading `retry_after` on a 429:** PITFALLS.md Pitfall 5 and D-08 both call this out; a fixed guess can either wait too little (immediate re-429) or needlessly too long.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Webhook delivery client | A generic "webhook client" abstraction, or a full Discord bot SDK | The ~80-line `internal/discord.Client` above | CLAUDE.md forbids a dependency here; the surface area (one POST, one embed shape) is smaller than the abstraction any library would impose |
| Outbox / idempotency mechanism | A new dedup or delivery-tracking table | Existing `events.notified_at` + `events_dedup_key` unique constraint (Phase 4) | Already correctly implements PITFALLS.md Pitfall 3's transactional-outbox recommendation — re-building it here would duplicate Phase 4's work and risk a second, inconsistent notion of "already seen" |
| Rate-limit backoff | A generic retry/circuit-breaker library | The single in-attempt retry in `Client.sendAttempt` above, backstopped by D-09's cross-cycle re-pickup | D-08/D-09 together already define the full retry contract; a library would add configuration surface for a two-line `time.Sleep` |

**Key insight:** Everything hard about "reliable webhook delivery" — dedup, at-least-once semantics, crash safety — was already solved correctly by Phase 4's outbox design. This phase's entire job is the last mile: turn a `NULL`-`notified_at` row into one HTTP POST and flip a boolean-shaped column on success.

## Common Pitfalls

### Pitfall 1: Checking for 200 instead of 204 on a successful send

**What goes wrong:** A client that only accepts `http.StatusOK` treats every real success as a failure, endlessly re-sending the same events every cycle (D-09's re-pickup would recreate the same duplicate message every poll interval, since MarkNotified would never fire).

**Why it happens:** Most REST APIs use `200` for a successful `POST`; Discord's default execute-webhook path deliberately returns an empty `204` unless `?wait=true` is passed.

**How to avoid:** Check for `http.StatusNoContent` (204) explicitly; do not use `?wait=true` unless a future feature needs the created message's id.

**Warning signs:** Discord messages actually arriving (visible in the channel) while logs show repeated "send failed" or the same event gets posted multiple times across successive poll cycles.

### Pitfall 2: Logging the webhook URL leaks the secret token

**What goes wrong:** A Discord webhook URL's *path* is `/webhooks/{id}/{token}` — the token is not a header or query parameter, it's part of the URL itself. Go's `net/http.Client.Do` returns transport-level failures as `*url.Error`, and `(*url.Error).Error()` includes the full request URL in its string form `[CITED: github.com/golang/go issue #44819 — "net/http.Client should allow omitting URLs from *url.Error"]`. A naive `fmt.Errorf("discord: send: %w", err)` followed by `logger.Error(..., slog.String("error", err.Error()))` therefore writes the live webhook token straight into structured logs.

**Why it happens:** Every other client in this codebase (`internal/musicbrainz`, `internal/deezer`) wraps `httpClient.Do` errors the same way, and it's safe there — MusicBrainz/Deezer URLs carry no secret. Copy-pasting that same wrapping pattern for Discord is the natural (but wrong) move.

**How to avoid:** Never include the underlying `err` from `httpClient.Do` in a Discord send-error message; return a fixed string (`"discord: send webhook: request failed"`, as in the Code Examples above) or apply a redaction helper mirroring `internal/db/migrate.go`'s existing `redactDSN`/`redactError` pattern (`[VERIFIED: internal/db/migrate.go:167,190]`) before logging anything derived from the request.

**Warning signs:** A grep for `DISCORD_WEBHOOK_URL`'s value (or the literal string `/webhooks/`) turning up in log output during code review.

### Pitfall 3: NTFY-01's "release type" field silently has no data

**What goes wrong:** A planner implements the new_release embed by reading `event.Title`/`event.ArtistName`/`event.ReleaseDate`/`event.CoverArtUrl` — all of which exist on the row today — and never notices "release type" has no matching column, so the embed either omits it (failing NTFY-01's literal acceptance criterion) or a lazy re-fetch is added that defeats D-05's whole "use the D-12 display snapshot, no live re-fetch" design.

**Why it happens:** `PrimaryType` (MusicBrainz) and `RecordType` (Deezer) are read by Phase 4's detection code, but only as an in-memory filter predicate — `releaseTypeAllowed(entry, g.PrimaryType)` (`[VERIFIED: internal/detection/filter.go:39-50]`, uses the exact string `primaryType is a raw MusicBrainz value such as "Album", capitalised`) and `releaseTypeAllowed(entry, a.RecordType)` (`[VERIFIED: internal/detection/deezer.go:62]`) — never assigned into `sqlc.InsertEventParams`. Confirmed by reading the full `InsertEventParams` struct (`[VERIFIED: internal/db/sqlc/events.sql.go:72-84]`): its fields are exactly `ArtistID, Source, EventType, ExternalID, ReleaseGroupMbid, Title, ArtistName, ReleaseDate, CoverArtUrl, TrackCount, NotifiedAt` — no release-type field exists.

**How to avoid:** Add a nullable `release_type TEXT` column in the same migration as D-04's `previous_track_count` (both are Phase-5-driven additive columns on the same table). Populate it in `DetectMusicBrainz`'s new_release loop using the *already-computed normalized* value (`strings.ToLower(strings.TrimSpace(g.PrimaryType))` — reuse `releaseTypeAllowed`'s internal `normalized` local, or recompute identically) and in `DetectDeezer`'s loop using `a.RecordType` directly (Deezer's live-observed value is already lowercase, `[VERIFIED: internal/deezer/albums.go:21-25]`, comment: "RecordType is carried as an opaque string ... only 'album' was observed live"). Leave it `nil` for `guest_feature` and `deluxe_change` inserts — NTFY-02/03 never ask for a release type.

**Warning signs:** A code-review or UAT pass on the new_release embed showing a blank/missing release-type field, or the planner's task list omitting `internal/detection/musicbrainz.go`/`deezer.go` from `files_modified` despite this migration existing.

### Pitfall 4: Serial-send loop with no ctx-cancellation check blocks graceful shutdown

**What goes wrong:** `Poller.Stop` cancels `runCtx` so an in-flight cycle unwinds promptly (`[VERIFIED: internal/poller/poller.go:164-185]`). If the notify loop's spacing sleep (D-07) uses a bare `time.Sleep(spacing)` instead of a ctx-aware wait, a burst of pending events (e.g. a Friday release day) can keep the process alive for several seconds past `Stop` being called, one 400ms sleep at a time, even though nothing about the sleep itself needs the database or network.

**How to avoid:** Use `select { case <-time.After(spacing): case <-ctx.Done(): return ctx.Err() }` for both the inter-send spacing and the 429 retry wait (both shown in the Code Examples above) — mirrors the ctx-respecting style already used throughout `internal/musicbrainz`/`internal/deezer`'s `doRequest`.

**Warning signs:** `docker stop`/SIGTERM taking noticeably longer to actually exit on a poll cycle that detected many new events versus one that detected none.

## Code Examples

### Migration: additive display columns

```sql
-- internal/db/migrations/000004_events_display_fields.up.sql
-- Phase 5: two nullable, additive columns Phase 4's detection code already
-- computes in memory but never persists (D-04's previous_track_count for
-- NTFY-03's delta display; release_type for NTFY-01's literal requirement,
-- a gap this research surfaced -- see 05-RESEARCH.md Pitfall 3).
ALTER TABLE events ADD COLUMN previous_track_count INT;
ALTER TABLE events ADD COLUMN release_type TEXT;
```

```sql
-- internal/db/migrations/000004_events_display_fields.down.sql
ALTER TABLE events DROP COLUMN release_type;
ALTER TABLE events DROP COLUMN previous_track_count;
```

### New sqlc query: mark a row notified

```sql
-- queries/events.sql -- new query, append after ListUnnotified
-- name: MarkNotified :execrows
-- Flips notified_at from NULL to now() after Client.Send confirms a 204
-- (D-09) -- never called before a send succeeds.
UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL;
```

### Deluxe-change insert path gains `PreviousTrackCount` (D-04)

The exact call site, read this session — `[VERIFIED: internal/detection/musicbrainz.go:326,342-358]`:
```go
baseline, hasBaseline, err := d.groupBaseline(ctx, g.MBID)
// ...
case maxCount > baseline:
	groupMBID := g.MBID
	coverArt := coverArtURLForReleaseGroup(groupMBID)
	trackCount := int32(maxCount)
	previousTrackCount := int32(baseline) // D-04: capture pre-update baseline before setGroupBaseline overwrites it
	newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
		ArtistID:           entry.ArtistID,
		Source:             sourceMusicBrainz,
		EventType:          eventTypeDeluxeChange,
		ExternalID:         winner.MBID,
		ReleaseGroupMbid:   &groupMBID,
		Title:              winner.Title,
		ArtistName:         entry.Name,
		ReleaseDate:        nullableString(winner.Date),
		CoverArtUrl:        &coverArt,
		TrackCount:         &trackCount,
		PreviousTrackCount: &previousTrackCount, // new field
		NotifiedAt:         notifiedAt,
	})
```

### New-release insert path gains `ReleaseType` (Pitfall 3 fix, MusicBrainz)

The exact call site, read this session — `[VERIFIED: internal/detection/musicbrainz.go:92-115]`:
```go
for _, g := range groups {
	if !releaseTypeAllowed(entry, g.PrimaryType) {
		filtered++
		continue
	}
	// ...
	mbid := g.MBID
	coverArt := coverArtURLForReleaseGroup(mbid)
	releaseType := strings.ToLower(strings.TrimSpace(g.PrimaryType)) // same normalization releaseTypeAllowed already applies
	newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
		ArtistID:         entry.ArtistID,
		Source:           sourceMusicBrainz,
		EventType:        eventTypeNewRelease,
		ExternalID:       mbid,
		ReleaseGroupMbid: &mbid,
		Title:            g.Title,
		ArtistName:       entry.Name,
		ReleaseDate:      nullableString(g.FirstReleaseDate),
		CoverArtUrl:      &coverArt,
		TrackCount:       nil,
		ReleaseType:      &releaseType, // new field
		NotifiedAt:       notifiedAt,
	})
```

### New-release insert path gains `ReleaseType` (Deezer)

The exact call site, read this session — `[VERIFIED: internal/detection/deezer.go:61-84]`:
```go
for _, a := range albums {
	if !releaseTypeAllowed(entry, a.RecordType) {
		filtered++
		continue
	}
	// ...
	recordType := a.RecordType // already lowercase per live-observed Deezer data
	newly, err := d.insertEvent(ctx, sqlc.InsertEventParams{
		ArtistID:         entry.ArtistID,
		Source:           sourceDeezer,
		EventType:        eventTypeNewRelease,
		ExternalID:       externalID,
		ReleaseGroupMbid: nil,
		Title:            a.Title,
		ArtistName:       entry.Name,
		ReleaseDate:      nullableString(a.ReleaseDate),
		CoverArtUrl:      nullableString(a.Cover),
		TrackCount:       nil,
		ReleaseType:      &recordType, // new field
		NotifiedAt:       notifiedAt,
	})
```

### Per-event-type embed formatting table (D-01, D-02, D-03)

Colors are Discord's own published brand palette (hex → decimal is a direct base-16 conversion; verify with `strconv.ParseInt(hex, 16, 32)` at implementation time if in doubt):

| Event type | Emoji | Hex | Decimal `color` | Link construction |
|---|---|---|---|---|
| `new_release` | 🆕 | `#57F287` (green) | 5763719 | MusicBrainz source: `https://musicbrainz.org/release-group/{external_id}` (external_id IS the release-group MBID, `[VERIFIED: internal/detection/musicbrainz.go:101,107-108]`, `ExternalID: mbid` / `ReleaseGroupMbid: &mbid`). Deezer source: `https://www.deezer.com/album/{external_id}` |
| `guest_feature` | 🎤 | `#FEE75C` (yellow) | 16705372 | `https://musicbrainz.org/recording/{external_id}` (external_id is the recording MBID, `[VERIFIED: internal/detection/musicbrainz.go:205]`, `ExternalID: rec.MBID`) |
| `deluxe_change` | 💿 | `#EB459E` (fuchsia) | 15418782 | `https://musicbrainz.org/release/{external_id}` (external_id is the winning *release*'s own MBID, `[VERIFIED: internal/detection/musicbrainz.go:350]`, `ExternalID: winner.MBID`) |

Field content per type (D-02/D-03, and NTFY-01's fields for `new_release`):
- `new_release`: title = `event.Title`, one field "Artist" = `event.ArtistName`, one field "Release Date" = `event.ReleaseDate` (if non-nil), one field "Type" = `event.ReleaseType` (if non-nil, capitalize for display), thumbnail = `event.CoverArtUrl` (if non-nil).
- `guest_feature`: title = `event.Title`, one field "Artist" = `event.ArtistName` (D-02's "primary credited artist" — already the value stored, `[VERIFIED: internal/detection/musicbrainz.go:207]`, `ArtistName: displayArtistName(rec, entry.Name)`).
- `deluxe_change`: title = `event.Title`, one field "Tracks" = `fmt.Sprintf("%d → %d tracks", *event.PreviousTrackCount, *event.TrackCount)` (D-03), thumbnail = `event.CoverArtUrl` (if non-nil).

All string fields must be truncated to Discord's documented limits before marshaling (title ≤ 256 chars, field value ≤ 1024 chars) — see Sources for the exact limit table; MusicBrainz/Deezer titles are community-editable data with no length guarantee (Phase 4's `[accepted risk] AR-04-01`/`AR-04-04` already flagged this class of data as untrusted at the render layer).

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Embed `description` capped at 2048 chars | Capped at 4096 chars | Documented in current community references, not independently reconfirmed against a dated official changelog this session | Low impact for this phase — none of the three event types here need more than a title + 1-2 short fields, well under either limit |

**Deprecated/outdated:** None relevant — Discord's webhook execute endpoint and rate-limit header set have been stable for years; no migration/deprecation applies here.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Per-webhook rate limit is ~30 requests/60s (not officially documented by Discord; community-corroborated, matches this project's own PITFALLS.md Pitfall 5) | Common Pitfalls / D-07 context | If the true bucket is smaller, the chosen ~400ms spacing could still occasionally 429 — mitigated by D-08's honor-`Retry-After` retry, so a wrong assumption here degrades to "one extra sleep," not data loss |
| A2 | Discord's brand-palette hex values (`#57F287`, `#FEE75C`, `#EB459E`) are still current | Code Examples (embed formatting table) | Purely cosmetic — a stale color value would still render a valid, distinct embed; no functional impact |
| A3 | Deezer's `RecordType` values are lowercase for all record types, not just the live-observed `"album"` | Pitfall 3 / Code Examples | If `single`/`ep`/`compilation` come back capitalized, `release_type` would display inconsistently (e.g. "Album" vs "Single") — cosmetic only, and Phase 4's own `[VERIFIED: internal/deezer/albums.go:21-25]` comment already flags this as an open assumption (A2 in 03-RESEARCH.md), not something this phase introduces |

## Open Questions

1. **Should `release_type` be normalized/title-cased for display, or shown raw?**
   - What we know: The stored value will be lowercase (`"album"`, `"single"`, `"ep"`) to match the existing filter vocabulary.
   - What's unclear: Whether the embed should show `"Album"` (title-cased) or `"album"` (raw) — purely a cosmetic choice CONTEXT.md left to discretion implicitly (it's covered by "exact emoji/color... not discussed further" in spirit, but display casing wasn't explicitly named).
   - Recommendation: Title-case at format time (`internal/notifier/format.go`), keep the stored column itself lowercase for consistency with `detectableReleaseTypes`.

2. **Does `internal/notifier` need its own package, or can this logic live directly on `Poller`?**
   - What we know: CONTEXT.md explicitly leaves this to Claude's Discretion ("Poller struct vs. a new Notifier type it holds").
   - What's unclear: No strong technical forcing function either way at this scale.
   - Recommendation: A separate `internal/notifier` package, mirroring `internal/detection`'s separation from `internal/poller` — keeps `poller.go` from growing a third responsibility (it already owns cycle scheduling + overlap guards for two sources) and gives the notify logic its own test file with real-Postgres integration tests, matching `detector_test.go`'s existing pattern.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Postgres (local/dev) | `internal/notifier` integration tests, `MarkNotified`/`ListUnnotified` | Assumed available via existing `make db-up` / `testutil.NewTestPool` (used by all prior phases' integration tests) | matches existing project setup | Tests requiring Postgres already skip gracefully under `-short` via `internal/testutil/postgres.go`'s existing pattern |
| Discord webhook URL (real) | End-to-end manual verification of actual message delivery | Not available in CI/automated tests (D-10 keeps it optional; no real webhook secret should ever be committed) | — | `httptest.Server`-backed unit tests cover `internal/discord.Client` behavior (204/429/other-status paths) without a real Discord endpoint; a real webhook is only needed for a human `checkpoint:human-verify`-style UAT step |

**Missing dependencies with no fallback:** none — a real Discord webhook is inherently a manual/human-verification concern (D-10), not a blocker for the automated build.

**Missing dependencies with fallback:** Real Discord delivery — falls back to `httptest.Server` fixtures for all automated tests; a human checkpoint is the correct gate for confirming a real message renders as expected in an actual Discord channel.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (`go test`), matching every prior phase |
| Config file | none — see `Makefile` targets |
| Quick run command | `go test ./internal/discord/... ./internal/notifier/... -short -race -count=1` |
| Full suite command | `make test` (`test-integration`, requires `make db-up` first — real Postgres, matches `[VERIFIED: Makefile:25-31]`) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| NTFY-01 | new_release embed carries title/artist/cover/date/release-type, POSTs successfully on 204 | unit (httptest.Server) | `go test ./internal/discord/... -run TestClient_Send -short` | ❌ Wave 0 |
| NTFY-01 | `previous_track_count`/`release_type` columns round-trip through `InsertEvent`/`ListUnnotified` | integration (real Postgres) | `go test ./internal/detection/... -run TestDetectMusicBrainz -race` | ❌ Wave 0 (existing `detector_test.go` needs new assertions, not a new file) |
| NTFY-02 | guest_feature embed is visually distinct (different color/emoji) and links to the recording | unit | `go test ./internal/notifier/... -run TestFormatEmbed_GuestFeature -short` | ❌ Wave 0 |
| NTFY-03 | deluxe_change embed shows old→new track delta | unit | `go test ./internal/notifier/... -run TestFormatEmbed_DeluxeChange -short` | ❌ Wave 0 |
| NTFY-04 | A muted event never reaches `ListUnnotified` | already covered | Existing Phase 4 tests (`detector_test.go` mute-axis tests) — confirm still green, no new test needed | ✓ (Phase 4) |
| D-06 | Concurrent MB/Deezer cycles never double-post the same pending event | integration (real Postgres, genuine concurrency) | `go test ./internal/notifier/... -run TestNotifyPending_ConcurrentCyclesNoDoublePost -race` | ❌ Wave 0 — mirrors the existing genuine-concurrency test style already used for `RunDeezerCycle`'s overlap guard (per recent commit history, `e53d48c`) |
| D-08/D-09 | A 429 honors `Retry-After` once, then a persistent failure leaves `notified_at` NULL for next-cycle pickup | unit (httptest.Server returning 429 then 429, or 429 then 204) | `go test ./internal/discord/... -run TestClient_Send_HonorsRetryAfter -short` | ❌ Wave 0 |
| D-10 | Empty `DISCORD_WEBHOOK_URL` boots normally, notify no-ops, one startup log line | unit | `go test ./cmd/server/... -run TestMain_DiscordDisabled -short` (or equivalent wiring test) | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/discord/... ./internal/notifier/... -short -race -count=1`
- **Per wave merge:** `make test` (full integration suite against real Postgres)
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps

- [ ] `internal/discord/client_test.go` — new file, covers `Client.Send`'s 204/429/other-status paths, mirroring `internal/musicbrainz/search_test.go`'s `httptest.Server` pattern
- [ ] `internal/notifier/notifier_test.go` — new file, covers `NotifyPending`'s fetch/format/send/mark loop and the D-06 concurrency guard, mirroring `internal/detection/detector_test.go`'s real-Postgres integration style
- [ ] `internal/notifier/format_test.go` — new file, covers per-event-type embed formatting (color/emoji/link construction/truncation)
- [ ] Existing `internal/detection/musicbrainz_test.go`/`deezer_test.go` — extend with assertions that `PreviousTrackCount`/`ReleaseType` are populated correctly on insert (not a new file, new test cases within it)
- [ ] No new framework install needed — `go test` and `httptest` are already in use project-wide

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | This phase has no inbound auth surface — it is an outbound-only background job |
| V3 Session Management | no | N/A |
| V4 Access Control | no | N/A |
| V5 Input Validation | yes | Truncate all MusicBrainz/Deezer-sourced strings (title, artist_name, release_type) to Discord's documented embed field limits before marshaling — both a correctness requirement (oversized fields cause Discord to reject the whole request) and a defensive one (Phase 4's `[accepted risk]` AR-04-01/AR-04-04 explicitly deferred this class of untrusted-content handling to this phase's render layer) |
| V6 Cryptography | no | No cryptographic operation in this phase; the webhook URL is a bearer-style secret, not a cryptographic key — see V9/V7 below for its handling |
| V7 Error Handling and Logging | yes | Never log the raw `error` returned from `httpClient.Do` inside `internal/discord` (leaks the webhook token via `*url.Error`, see Pitfall 2) — mirror the existing `redactDSN`/`redactError` discipline from `internal/db/migrate.go` |
| V9 Communications | yes | All outbound calls are HTTPS to `discord.com` (TLS handled by `net/http`'s default transport, same as `internal/musicbrainz`/`internal/deezer` — no custom TLS config needed) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Webhook token (secret) embedded in a URL, leaking via generic error-wrapping | Information Disclosure | Never wrap/log the raw `httpClient.Do` error; return/log a fixed message instead (Code Examples, Pitfall 2) |
| Community-editable title/artist-name content rendered into a Discord embed (markdown injection: bold/link markdown, not code execution — Discord embeds don't parse `@mentions`, only the `content` field does) | Tampering | Truncate to field limits (functional necessity); accept the residual "misleading link text" risk exactly as Phase 4 already did for the storage layer (`AR-04-01`/`AR-04-04`) — Discord embeds cannot trigger a mention ping or auto-unfurl a nested link, so the blast radius is a visually odd/misleading field, not an active exploit |
| A malformed/hostile `DISCORD_WEBHOOK_URL` env var (e.g. pointing at an internal address) | Spoofing / SSRF-adjacent | Low residual risk — this value is operator-supplied config (OPS-03, env-var-only secrets), not attacker-controlled input; no additional validation beyond what `http.NewRequestWithContext` already enforces (a malformed URL fails to parse and returns an error before any request is sent) |
| Unbounded pending-event backlog on a long-broken webhook (D-09's "keeps retrying every cycle forever") | Denial of Service | Bounded in practice by `ListUnnotified`'s existing result set size and the per-send spacing (D-07) — worth noting for the planner's threat model as an `accept`ed risk (mirrors Phase 4's own `AR-04-05` "residual cost is a longer cycle, not unbounded growth" pattern) rather than something this phase needs to newly mitigate |

## Project Constraints (from CLAUDE.md)

- Hand-rolled `internal/discord` package using plain `net/http` + `encoding/json` — no webhook-client library, no Discord bot SDK.
- `DISCORD_WEBHOOK_URL` supplied via environment variable only (`caarlos0/env`-parsed `Config` struct); already present and intentionally optional (`internal/config/config.go:27`, `[VERIFIED: internal/config/config.go:27]`, `DiscordWebhookURL string \`env:"DISCORD_WEBHOOK_URL"\``) — D-10 keeps it that way, no `notEmpty`/`required` tag.
- sqlc for the new `MarkNotified` query and the widened `InsertEventParams`; golang-migrate for the new `000004_*` migration — both via the existing `make sqlc` / migration-file conventions, zero-padded numeric prefix (PITFALLS.md Pitfall 9).
- Single Go binary/service architecture — the notifier is an in-process package called from the existing poller, not a separate service or container.
- All secrets via environment variables only; nothing real ever committed — no test fixture should contain a real-looking Discord webhook URL/token (gitleaks pre-commit hook already enforces this project-wide, per `260806-hfn`).
- Unit tests use `httptest.Server` to mock external calls, no live external calls in CI — applies directly to `internal/discord`'s test suite (no real Discord webhook is ever hit in CI).

## Sources

### Primary (HIGH confidence)

None — no `context7`/`ref` (official-docs-integrated) provider was available this session (all of `exa_search`/`brave_search`/`firecrawl`/`tavily_search`/`ref_search`/`jina`/`perplexity` are `false` in `.planning/config.json`, `[VERIFIED: .planning/config.json:7-12]`); the built-in `WebSearch`/`WebFetch` tools were used instead, capped at MEDIUM per this project's `classify-confidence` seam.

### Secondary (MEDIUM confidence)

- [Webhook Resource — Discord Developer Docs](https://docs.discord.com/developers/resources/webhook) — confirmed via `WebFetch` this session: execute-webhook route, method, `application/json` content-type, 204-default/200-with-`wait=true` response codes, embed field restrictions
- [Rate Limits — Discord Developer Docs](https://docs.discord.com/developers/topics/rate-limits) — confirmed via `WebFetch` this session: `X-RateLimit-Limit`/`-Remaining`/`-Reset`/`-Reset-After`/`-Bucket` headers, 429 JSON body shape (`message`, `retry_after` float seconds, `global`, optional `code`); explicitly does not publish a per-webhook numeric bucket
- [Discord Webhooks Guide — fields](https://birdie0.github.io/discord-webhooks-guide/structure/embed/fields.html), [Discord Webhook Guide 2026](https://discord-webhook.com/en/discord-webhook-guide/), [Discord Embed Limits Cheat Sheet](https://discord-webhook.com/en/blog/discord-webhook-embed-limits/) — corroborate exact embed field character limits (title 256, description 4096, field name 256/value 1024, footer 2048, author name 256, 6000-char total, 25 fields, 10 embeds/message)
- [Discord Webhook Rate Limits Explained](https://discord-webhook.com/en/blog/discord-webhook-rate-limits/) — corroborates the community-observed ~30/60s-per-webhook figure already cited in this project's own PITFALLS.md
- [golang/go issue #44819 — net/http.Client should allow omitting URLs from *url.Error](https://github.com/golang/go/issues/44819) — confirms `*url.Error.Error()` embeds the full request URL, the basis for Pitfall 2
- `.planning/research/PITFALLS.md` Pitfall 3 (transactional outbox), Pitfall 5 (Discord rate limits/message-size) — this project's own prior research, already locked as canonical background per 05-CONTEXT.md

### Tertiary (LOW confidence)

None recorded as authoritative in this document — all web-sourced numeric claims above were cross-checked against at least the official Discord docs page where possible and are tagged `[CITED]`/MEDIUM accordingly; anything not independently corroborated is called out explicitly in the Assumptions Log.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — stdlib-only, directly read from this repo's own `go.mod` and existing client packages this session
- Architecture: HIGH — every seam/pattern recommendation mirrors code read directly this session (`poller.go`, `detector.go`, `musicbrainz.go`, `deezer.go`, `events.sql.go`, `models.go`)
- Discord API shape (embed schema, rate-limit headers, 204/429 behavior): MEDIUM — official docs confirmed the structural facts; exact per-webhook numeric rate-limit bucket remains community-sourced, not officially published
- Pitfalls: HIGH for the two codebase-specific findings (204-vs-200, missing release_type column) — both confirmed by reading source directly, not inferred

**Research date:** 2026-08-08
**Valid until:** 2026-09-07 (30 days — Discord's webhook API surface is stable; re-verify only if Discord ships a documented breaking change to the execute-webhook endpoint or rate-limit scheme)
