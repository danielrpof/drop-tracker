# Phase 5: Discord Notifications - Pattern Map

**Mapped:** 2026-08-08
**Files analyzed:** 11 (new + modified)
**Analogs found:** 11 / 11

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|--------------------|------|-----------|-----------------|----------------|
| `internal/discord/client.go` (new) | service (external HTTP client) | request-response | `internal/musicbrainz/client.go` | exact (hand-rolled client shape, rate/User-Agent style `doRequest` funnel) |
| `internal/discord/client_test.go` (new) | test | request-response | `internal/musicbrainz/search_test.go` | exact (`httptest.Server`-backed) |
| `internal/notifier/notifier.go` (new) | service (orchestration/outbox drain) | event-driven / CRUD | `internal/detection/detector.go` | exact (narrow-seam struct wrapping `sqlc.Querier` + collaborator interfaces) |
| `internal/notifier/format.go` (new) | utility (transform) | transform | `internal/detection/musicbrainz.go` (`nullableString`, `coverArtURLForReleaseGroup`-style helpers) | role-match |
| `internal/notifier/notifier_test.go` (new) | test | event-driven / CRUD | `internal/detection/detector_test.go` (real-Postgres integration style) | exact |
| `internal/notifier/format_test.go` (new) | test | transform | `internal/musicbrainz/search_test.go` (table-driven unit style) | role-match |
| `internal/poller/poller.go` (modified) | controller/orchestrator | request-response (in-process call) | itself — extend `EventRecorder`-seam pattern with new `Notifier` seam | exact |
| `internal/detection/musicbrainz.go` (modified) | service | CRUD | itself — extend existing `insertEvent` call sites | exact |
| `internal/detection/deezer.go` (modified) | service | CRUD | itself — extend existing `insertEvent` call site | exact |
| `internal/db/migrations/000004_events_display_fields.{up,down}.sql` (new) | migration | batch/DDL | `internal/db/migrations/000003_events.{up,down}.sql` | exact |
| `queries/events.sql` (modified) + `internal/db/sqlc/events.sql.go` (regenerated) | model/query | CRUD | itself — extend with `MarkNotified`, widen `InsertEventParams` | exact |
| `cmd/server/main.go` (modified) | config/wiring | request-response | itself — extend existing `mbClient`/`dzClient`/`detector`/`poller.New` wiring block | exact |

## Pattern Assignments

### `internal/discord/client.go` (service, request-response)

**Analog:** `internal/musicbrainz/client.go` + `internal/musicbrainz/search.go`

**Imports pattern** (`internal/musicbrainz/client.go` lines 10-18):
```go
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)
```
Discord's client needs no rate limiter (D-07's spacing is the notifier's job, not the client's) — drop `golang.org/x/time/rate`, add `bytes`, `encoding/json`.

**Client struct + constructor pattern** (`internal/musicbrainz/client.go` lines 40-69):
```go
type Client struct {
	baseURL    string
	userAgent  string
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient(userAgent string, limiter *rate.Limiter, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{...}
}
```
Mirror the nil-`httpClient` default-injection idiom exactly; drop `baseURL`/`userAgent`/`limiter` fields, replace with `webhookURL string`.

**Single funnel method pattern** (`internal/musicbrainz/client.go` lines 93-135, `doRequest`): every outbound request goes through one method that sets headers then calls `httpClient.Do`. For `internal/discord`, `Send`/`sendAttempt` collapses `doRequest` + the one call site into one method (per RESEARCH.md Pattern 2) since Discord's client only ever does one thing (POST one embed).

**Error handling pattern — status-code check, no body echo** (`internal/musicbrainz/search.go` lines 89-93):
```go
if resp.StatusCode != http.StatusOK {
	// Never echo the response body -- only the status code
	return nil, fmt.Errorf("musicbrainz: search artists: unexpected status %d", resp.StatusCode)
}
```
Adapt for Discord's 204-success / 429-retry-once / other-status-error trichotomy (see RESEARCH.md Code Examples for the full `Send`/`sendAttempt` implementation — already vetted against this codebase's conventions).

**CRITICAL deviation from the musicbrainz/deezer client precedent (Pitfall 2):** do NOT wrap the raw `httpClient.Do` error the way `doRequest` does at line 131 (`fmt.Errorf("musicbrainz: do request: %w", err)`). MusicBrainz/Deezer URLs carry no secret; the Discord webhook URL's path IS the secret token (`/webhooks/{id}/{token}`), and Go's `*url.Error.Error()` embeds the full request URL. Return a fixed string instead: `fmt.Errorf("discord: send webhook: request failed")`.

---

### `internal/discord/client_test.go` (test)

**Analog:** `internal/musicbrainz/search_test.go` (httptest.Server pattern — read this file's structure directly before writing, same `httptest.NewServer(http.HandlerFunc(...))` + table-driven cases idiom used project-wide for external-client tests). Cover: 204 success, 429-then-204 (honors `Retry-After`), 429-then-429 (single retry only, per D-08), other non-204 status → error, and a case asserting the returned error string never contains the webhook URL/token (Pitfall 2 regression guard).

---

### `internal/notifier/notifier.go` (service, event-driven/CRUD)

**Analog:** `internal/detection/detector.go`

**Struct + constructor pattern** (`internal/detection/detector.go` lines 41-56):
```go
type Detector struct {
	q          sqlc.Querier
	recordings RecordingSource
	releases   ReleaseDetailSource
}

func New(q sqlc.Querier, recordings RecordingSource, releases ReleaseDetailSource) *Detector {
	return &Detector{q: q, recordings: recordings, releases: releases}
}
```
`Notifier` mirrors this exactly: `q sqlc.Querier`, `sender Sender` (narrow seam over `discord.Client.Send`), plus the `atomic.Bool` guard and `spacing time.Duration` field (see RESEARCH.md's `internal/notifier/notifier.go` code example — already fits this shape).

**Narrow-seam-in-consumer pattern** (`internal/detection/detector.go` lines 21-39, `RecordingSource`/`ReleaseDetailSource`): declare a `Sender` interface in `internal/notifier` (the consumer) over `discord.Client`, and declare `poller.Notifier` in `internal/poller` (the consumer) over `*notifier.Notifier`:
```go
// internal/poller/poller.go — add alongside EventRecorder, mirrors its exact shape (lines 64-73)
type Notifier interface {
	NotifyPending(ctx context.Context, logger *slog.Logger) error
}
```

**CAS-skip guard pattern** (`internal/poller/poller.go` lines 193-205, `mbRunning.CompareAndSwap`):
```go
if !p.mbRunning.CompareAndSwap(false, true) {
	p.logger.Warn("skipping poll cycle: previous cycle still in progress", ...)
	return ErrCycleInProgress
}
defer p.mbRunning.Store(false)
```
`Notifier.NotifyPending` copies this exact idiom with its own `notifying atomic.Bool` field — CAS-skip (not blocking mutex), released via `defer`, logged at `Warn`/`Info` on skip. This is D-06's locked mechanism; the analog is a byte-for-byte structural match already in the codebase.

**Error-continue-on-per-item-failure pattern** (`internal/poller/poller.go` lines 220-244, and `internal/detection/deezer.go` lines 61-91): a per-artist/per-album loop logs an error and `continue`s rather than aborting the whole cycle. `NotifyPending`'s per-event send loop mirrors this: log-and-`continue` on a failed send (D-09), but `return` on an unexpected `MarkNotified`/`ListUnnotified` DB error (mirrors `detector.go`'s `insertEvent`/`groupBaseline` treating DB errors as hard failures vs. treating "0 rows affected" as expected).

**insertEvent-style helper wrapping a sqlc call** (`internal/detection/detector.go` lines 58-68, `insertEvent`): mirror this shape for a `markNotified` helper wrapping `q.MarkNotified`, returning a bool/err pair if useful, though here MarkNotified's contract is simpler (always call after a confirmed send).

---

### `internal/notifier/format.go` (utility, transform)

**Analog:** `internal/detection/musicbrainz.go`'s small pure helpers (`nullableString` lines 108-113, `coverArtURLForReleaseGroup`) and the per-event-type field-population blocks at lines 92-122 (new_release insert) and 342-358 (deluxe_change insert).

**Pattern:** one pure function per concern, no I/O, operating on the `sqlc.Event` struct's already-populated display snapshot fields (`Title`, `ArtistName`, `ReleaseDate *string`, `CoverArtUrl *string`, `TrackCount *int32`, `PreviousTrackCount *int32` [new], `ReleaseType *string` [new]) — never a live re-fetch (D-05/D-12's locked constraint). Nil-check every `*string`/`*int32` field before dereferencing, exactly as `nullableString`'s inverse is used at insert time. See RESEARCH.md's embed color/emoji table (D-01) and field-content table (D-02/D-03) for the exact per-event-type mapping — already fully specified and ready to translate directly into a `switch event.EventType { case eventTypeNewRelease: ... }` structure mirroring `detection`'s own `eventType*` constants (`eventTypeNewRelease`, `eventTypeGuestFeature`, `eventTypeDeluxeChange` — reuse these constants from `internal/detection`, or mirror their string literals if `internal/notifier` should not import `internal/detection` — confirm during planning which direction avoids a package cycle; `internal/detection` does not currently import `internal/notifier` so importing detection's constants from notifier is safe. Prefer exporting `EventTypeNewRelease` etc. from `internal/detection` and importing here, OR defining independent constants in `internal/notifier` matching the DB CHECK constraint values verbatim, per Claude's Discretion on this point.)

---

### `internal/notifier/notifier_test.go` / `format_test.go` (test)

**Analog:** `internal/detection/detector_test.go` (real-Postgres integration style, per RESEARCH.md's own Wave-0 gap list) for `notifier_test.go`'s `NotifyPending` fetch/format/send/mark loop and D-06 genuine-concurrency test (mirror the recent commit `e53d48c`'s `RunDeezerCycle` overlap-guard-under-genuine-concurrency test style — read that test directly during planning for the concurrency-harness idiom). `format_test.go` is a pure table-driven unit test, no DB, mirroring `internal/musicbrainz/search_test.go`'s table-driven case style.

---

### `internal/poller/poller.go` (modified — controller/orchestrator)

**Analog:** itself. Add a `Notifier` field to the `Poller` struct (alongside `store`/`mb`/`dz`/`events`, lines 86-109) and a `notifier Notifier` constructor param to `New` (lines 118-152), following the exact pattern `events EventRecorder` already established. Call `p.notifier.NotifyPending(ctx, logger)` at the end of both `RunMusicBrainzCycle` (after line 244's loop) and `RunDeezerCycle` (after line 312's loop), per D-05 — log-and-continue on error, do not fail the whole cycle (mirrors how a per-artist detection error at lines 236-243 is logged, not fatal).

**D-10 gating:** the wiring in `cmd/server/main.go` (not `poller.go` itself) decides whether to construct a real `notifier.Notifier` or a no-op — see `cmd/server/main.go` pattern below. `poller.go`'s `Notifier` interface has no "disabled" concept; it always gets called, and a no-op implementation satisfies D-10 without adding a nil-check branch inside the cycle methods (mirrors how `EventRecorder` has no disabled-state concept either — the seam is unconditionally called).

---

### `internal/detection/musicbrainz.go` / `deezer.go` (modified — service, CRUD)

**Analog:** itself, three exact call sites already read this session:

1. New-release insert, MusicBrainz (`internal/detection/musicbrainz.go` lines 92-122) — add `releaseType := strings.ToLower(strings.TrimSpace(g.PrimaryType))` and `ReleaseType: &releaseType` to the `sqlc.InsertEventParams{}` literal at line 103.
2. New-release insert, Deezer (`internal/detection/deezer.go` lines 61-91) — add `recordType := a.RecordType` and `ReleaseType: &recordType` to the `sqlc.InsertEventParams{}` literal at line 72.
3. Deluxe-change insert, MusicBrainz (`internal/detection/musicbrainz.go` lines 326-364) — capture `previousTrackCount := int32(baseline)` immediately after line 326's `groupBaseline` call (before `setGroupBaseline` overwrites it at line 333/365), add `PreviousTrackCount: &previousTrackCount` to the `sqlc.InsertEventParams{}` literal at line 346.

All three are pure field-literal additions to existing struct literals — no new control flow, no new error paths. `strings` import already present in `musicbrainz.go`? Verify at implementation time; add if missing.

---

### `internal/db/migrations/000004_events_display_fields.{up,down}.sql` (new — migration)

**Analog:** `internal/db/migrations/000003_events.up.sql`/`.down.sql` (full file read this session).

**Convention to copy exactly:**
- Zero-padded numeric prefix (`000004_`), descriptive snake_case name.
- Up-migration comment header explaining *why*, referencing the phase/decision IDs (D-04, Pitfall 3), mirroring `000003`'s header style (lines 1-26 of that file).
- Plain `ALTER TABLE events ADD COLUMN <name> <TYPE>;` — no `NOT NULL`, no `DEFAULT` (both new columns are nullable per D-04/Pitfall 3), matching `000003`'s own nullable-column style (`release_group_mbid TEXT`, `release_date TEXT` — no constraint) rather than its `NOT NULL` columns (`title`, `artist_name`).
- Down-migration drops columns in reverse order of the up-migration's adds (`000003.down.sql` pattern — read it directly if unsure of exact reverse-drop convention, it was not fully quoted this session but the up/down pairing convention is `DROP TABLE`/`CREATE TABLE` symmetry at the file level; for column-level ALTER, reverse column order is the safe convention).

```sql
-- internal/db/migrations/000004_events_display_fields.up.sql
ALTER TABLE events ADD COLUMN previous_track_count INT;
ALTER TABLE events ADD COLUMN release_type TEXT;
```
```sql
-- internal/db/migrations/000004_events_display_fields.down.sql
ALTER TABLE events DROP COLUMN release_type;
ALTER TABLE events DROP COLUMN previous_track_count;
```

---

### `queries/events.sql` (modified) + `internal/db/sqlc/events.sql.go` (regenerated)

**Analog:** itself — existing `InsertEvent`/`ListUnnotified` queries in `queries/events.sql` (lines 1-14, 29-35) and their generated counterparts in `internal/db/sqlc/events.sql.go` (lines 62-110, 145-186).

**Widen `InsertEvent`:** add `previous_track_count`, `release_type` to the column list and `$12`, `$13` placeholders (comment style at lines 1-7 of `queries/events.sql` — explain *why* `DO NOTHING` with no `SET` clause is preserved unchanged, D-20 still applies to the two new columns exactly as to the existing snapshot columns).

**Add `MarkNotified` query**, exact shape already validated in RESEARCH.md:
```sql
-- name: MarkNotified :execrows
-- Flips notified_at from NULL to now() after Client.Send confirms a 204
-- (D-09) -- never called before a send succeeds.
UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL;
```
Comment style matches `SetGroupTrackCountBaseline`'s existing header (lines 52-57 of `queries/events.sql`) — state the precondition and the decision ID it implements.

**Regenerate via `sqlc generate`** (per CLAUDE.md/Makefile convention) rather than hand-editing `events.sql.go` — `InsertEventParams` gains `PreviousTrackCount *int32` and `ReleaseType *string` fields following the exact `*int32`/`*string` nullable-pointer convention already used for `TrackCount`/`ReleaseGroupMbid` (lines 72-84 of `events.sql.go`).

---

### `cmd/server/main.go` (modified — wiring)

**Analog:** itself — the existing `mbClient`/`dzClient`/`detector`/`pollr` construction block (lines 93-143).

**Pattern to copy:** construct `discordClient` conditionally on `cfg.DiscordWebhookURL != ""` (D-10), mirroring the existing style of building one shared instance per external dependency and passing it into `poller.New`. When unset, log one line and pass a no-op `notifier.Notifier` implementation (or a `nil`-safe wrapper) so `poller.New`'s new param is always non-nil — follow the same "always call the seam, let the seam itself be inert" idiom evident in how `EventRecorder`/`AlbumSource` are always real, never nil-checked inside `poller.go`. Suggested insertion point: immediately after the `dzClient` construction (line 126), before `srv := httpserver.New(...)` (line 128), since `pollr, err := poller.New(...)` (line 139) needs the resulting notifier as a new argument.

```go
// Pattern sketch, following main.go's existing comment density and structure:
var notif poller.Notifier
if cfg.DiscordWebhookURL == "" {
	logger.Info("discord notifications disabled: DISCORD_WEBHOOK_URL not set")
	notif = notifier.NoOp{} // or equivalent inert implementation
} else {
	discordClient := discord.NewClient(cfg.DiscordWebhookURL, nil)
	notif = notifier.New(sqlc.New(pool), discordClient, notifierSpacing)
}

pollr, err := poller.New(store, mbClient, dzClient, detector, notif, cfg.PollInterval, logger)
```

## Shared Patterns

### Narrow interface seam declared in the consumer
**Source:** `internal/poller/poller.go` lines 45-73 (`ReleaseGroupSource`, `AlbumSource`, `EventRecorder`); `internal/detection/detector.go` lines 21-39 (`RecordingSource`, `ReleaseDetailSource`)
**Apply to:** `poller.Notifier` (declared in `poller.go`), `notifier.Sender` (declared in `notifier.go`) — every new cross-package dependency in this phase must be a small interface declared in the importing package, never a concrete type reference, with a `var _ Interface = (*ConcreteType)(nil)` compile-time assertion (see `internal/poller/poller.go` lines 54, 62 for the assertion idiom).

### CAS-skip overlap guard (never a blocking mutex)
**Source:** `internal/poller/poller.go` lines 81-109, 193-205, 258-265 (`mbRunning`/`dzRunning atomic.Bool`, `CompareAndSwap(false, true)` + `defer ... Store(false)`)
**Apply to:** `internal/notifier.Notifier`'s shared D-06 guard — same field type, same CAS pattern, same defer-release, same "skip and log, don't queue" semantics. This is a hard architectural precedent in this codebase (explicitly documented at lines 29-34 and repeated in the comment at 194-200) — do not substitute a `sync.Mutex` here.

### Structured slog logging with cycle/source correlation
**Source:** `internal/poller/poller.go` lines 207-208, 267-268 (`logger := p.logger.With(slog.String("source", ...), slog.String("cycle_id", cycleID))`)
**Apply to:** `NotifyPending` should accept the same `*slog.Logger` the calling cycle already built (already carrying `source`/`cycle_id`), not construct its own — per CONTEXT.md's explicit note ("notifier log lines should extend the same logger passed into the cycle, not start a new one").

### Per-item error-continue, not whole-batch failure
**Source:** `internal/poller/poller.go` lines 220-244 (per-artist fetch/detect error → log + `continue`); `internal/detection/deezer.go` lines 61-91 (per-album loop, same idiom)
**Apply to:** `NotifyPending`'s per-event send loop — a single event's send failure must not abort the rest of the pending batch (D-09's "leave notified_at NULL, keep going" contract already matches this codebase's established per-item resilience pattern exactly).

### Never wrap/log raw transport errors that could leak secrets
**Source:** New pattern for this phase — deliberately diverges from `internal/musicbrainz/client.go` line 131's `fmt.Errorf("musicbrainz: do request: %w", err)`, and from `internal/db/migrate.go`'s `redactDSN`/`redactError` helpers (referenced in RESEARCH.md, not read this session — read directly during implementation if a redaction helper is preferred over a fixed string).
**Apply to:** `internal/discord/client.go`'s `sendAttempt` — return `fmt.Errorf("discord: send webhook: request failed")` on transport error, never `%w`-wrap the raw `*url.Error`.

### Nullable-pointer column convention
**Source:** `internal/db/sqlc/events.sql.go` lines 72-84 (`InsertEventParams` — `ReleaseGroupMbid *string`, `ReleaseDate *string`, `CoverArtUrl *string`, `TrackCount *int32`); `internal/detection/detector.go` lines 104-113 (`nullableString` helper)
**Apply to:** the two new columns (`PreviousTrackCount *int32`, `ReleaseType *string`) — same pointer-for-nullable convention, populated via the same `nullableString`-style pattern where applicable (or direct `&x` for values already known non-empty, matching `TrackCount: &trackCount`'s existing style at line 356 of `musicbrainz.go`).

## No Analog Found

None — every file in this phase's scope has a direct, previously-read analog in the existing codebase. This is expected: the phase is explicitly "wire delivery onto an already-designed outbox," not new architecture.

## Metadata

**Analog search scope:** `internal/poller/`, `internal/detection/`, `internal/musicbrainz/`, `internal/deezer/`, `internal/db/`, `queries/`, `internal/config/`, `cmd/server/` — full read of all files directly relevant to this phase's integration points, per CONTEXT.md's own "Existing code (Phase 1-4)" canonical-refs list.
**Files scanned:** 13 read in full this session (`poller.go`, `musicbrainz/client.go`, `musicbrainz/search.go`, `detection/detector.go`, `detection/deezer.go`, `detection/musicbrainz.go` [3 targeted ranges], `db/sqlc/events.sql.go`, `queries/events.sql`, `db/migrations/000003_events.up.sql`, `config/config.go`, `cmd/server/main.go`) plus RESEARCH.md's own already-verified `[VERIFIED: ...]`-tagged excerpts, cross-checked against direct reads where line-numbers overlapped.
**Pattern extraction date:** 2026-08-08
