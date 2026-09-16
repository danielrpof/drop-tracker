---
phase: 05-discord-notifications
plan: 01
subsystem: notifier
tags: [discord, webhook, outbox, poller, sqlc, migration]
dependency-graph:
  requires:
    - "internal/db/sqlc.Querier (ListUnnotified, InsertEvent) — Phase 4"
    - "internal/poller.Poller cycle machinery (RunMusicBrainzCycle/RunDeezerCycle) — Phase 3/4"
  provides:
    - "internal/discord.Client — hand-rolled Discord webhook client"
    - "internal/notifier.Notifier / notifier.Select / notifier.NoOp — outbox drain + D-10 gate"
    - "poller.Notifier seam, wired into both poll cycles"
    - "sqlc.Querier.MarkNotified"
    - "events.previous_track_count / events.release_type columns"
  affects:
    - "internal/poller/poller.go (New's signature widened)"
    - "cmd/server/main.go (notifier wiring block)"
tech-stack:
  added: []
  patterns:
    - "Narrow interface seam declared in the consumer (poller.Notifier, notifier.Sender/Sink)"
    - "CAS-skip overlap guard (atomic.Bool), never a blocking mutex"
    - "Fixed-string error on transport failure to avoid *url.Error leaking a secret URL"
key-files:
  created:
    - internal/db/migrations/000004_events_display_fields.up.sql
    - internal/db/migrations/000004_events_display_fields.down.sql
    - internal/discord/client.go
    - internal/discord/client_test.go
    - internal/notifier/notifier.go
    - internal/notifier/format.go
    - internal/notifier/notifier_test.go
  modified:
    - queries/events.sql
    - internal/db/sqlc/events.sql.go
    - internal/db/sqlc/models.go
    - internal/db/sqlc/querier.go
    - internal/db/migrate_test.go
    - internal/poller/poller.go
    - internal/poller/poller_test.go
    - cmd/server/main.go
decisions:
  - "Task 1's migration bumped 'from scratch' schema version from 3 to 4; fixed the hardcoded assertion in migrate_test.go as a direct, in-scope consequence (Rule 1)."
  - "-race is environmentally broken on this Windows dev box for every package (ThreadSanitizer allocation failure), not specific to this plan's changes -- verified against an unrelated package (internal/watchlist) before concluding this. Ran the full verify suite without -race instead; matches the pre-existing documented cgo/mingw toolchain limitation from Phase 1."
  - "go test ./... shows one flaky failure (TestDetectMusicBrainz_FiltersByReleaseType, 'relation events does not exist') caused by a pre-existing cross-package race: internal/db's TestRunMigrations_AppliesFromScratch drops and recreates the whole public schema while other packages' test binaries run concurrently against the same shared Postgres instance under go test ./.... Confirmed pre-existing and out of scope by re-running internal/detection alone, which passes cleanly. Not fixed (scope boundary)."
metrics:
  duration: 55min
  completed: 2026-08-08
actuals:
  tokens: 14730
  tasks: 2
  commits: 3
status: complete
---

# Phase 5 Plan 1: Discord Notification Tracer Summary

Delivers the complete Discord delivery path end-to-end on the thinnest possible slice: a real `events` row with `notified_at IS NULL` becomes an HTTP POST to a webhook endpoint and gets acknowledged in the database, wired into the live poll cycles and the process entrypoint via a hand-rolled `internal/discord` client and a new `internal/notifier` outbox-drain package.

## What Was Built

**Task 1 — Schema:** Migration `000004_events_display_fields` adds nullable `previous_track_count INT` and `release_type TEXT` to `events` (D-04, NTFY-01's display fields Phase 4 computed but never persisted). `queries/events.sql` widens `InsertEvent` to `$12`/`$13` for the two new columns (D-20's write-once contract preserved) and adds `MarkNotified` (`UPDATE events SET notified_at = now() WHERE id = $1 AND notified_at IS NULL`, D-09's idempotent ack). Regenerated via `sqlc generate` (v1.31.1, matches `SQLC_VERSION` pin) — `InsertEventParams`/`Event` gained `PreviousTrackCount *int32`/`ReleaseType *string`, `Querier` gained `MarkNotified`. Proved against a live Postgres instance (`docker exec ... psql \d events`, confirms both columns nullable) and via `go test ./internal/db/... ./internal/detection/...`.

**Task 2 — Tracer (TDD: RED then GREEN):**
- `internal/discord.Client` (`NewClient`, `Send`) — mirrors `internal/musicbrainz.Client`'s shape, minus the rate limiter (pacing is the notifier's job, D-07). Checks for HTTP 204 (not 200 — Discord's real success code), retries exactly once on 429 honoring `Retry-After` (D-08), and — critically — never wraps the raw `httpClient.Do` error, since Go's `*url.Error.Error()` embeds the full request URL and a Discord webhook's path *is* its secret token (T-05-01).
- `internal/notifier.Notifier` — `NotifyPending` drains `ListUnnotified` under an `atomic.Bool` CAS-skip guard (D-06, mirrors `mbRunning`/`dzRunning`), sends each event serially with 400ms spacing (D-07), marks success via `MarkNotified`, and logs-and-continues on a per-event send failure (D-09: row stays NULL, next pass re-picks it up). `notifier.Select` owns D-10's gate — an empty webhook URL returns `NoOp{}` with one disabled-startup log line, so `poller.New`'s notifier argument is always non-nil.
- `internal/notifier/format.go` — pure `formatEmbed` switch on `event_type`, establishing the color/emoji shape for all three types (new_release green/🆕, guest_feature yellow/🎤, deluxe_change fuchsia/💿); only `new_release`'s fields are fully populated in this tracer, per the plan's explicit thin-formatting scope (plan 05-03 fills in the rest).
- `internal/poller.Poller` gained a `Notifier` seam, called at the end of both `RunMusicBrainzCycle` and `RunDeezerCycle` (D-05) — a delivery failure is logged, not returned, so it never turns a successful detection cycle into a failed one.
- `cmd/server/main.go` wires `notifier.Select(cfg.DiscordWebhookURL, sqlc.New(pool), nil, logger)` into `poller.New`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed hardcoded schema-version assertion in `internal/db/migrate_test.go`**
- **Found during:** Task 1 verification (`go test ./internal/db/...`)
- **Issue:** `TestRunMigrations_AppliesFromScratch` asserted `version == 3` for an apply-from-scratch run — a value that predates this plan's migration 000004.
- **Fix:** Updated the assertion (and its explanatory comment) to `version == 4`, matching the new migration count.
- **Files modified:** `internal/db/migrate_test.go`
- **Commit:** 0428396

No other deviations — plan executed as written, including the tracer feedback gate (auto mode: re-ran the full `internal/discord`/`internal/notifier`/`internal/poller` suite after the GREEN commit; all green, proceeded without a checkpoint).

## Known Environment Limitations (not deviations, not fixed)

- **`-race` fails on this Windows dev box for every package**, not just this plan's new code (`ThreadSanitizer failed to allocate ... (error code: 87)`) — confirmed by running `-race` against an unrelated pre-existing package (`internal/watchlist`) and observing the identical failure. This matches Phase 1's already-documented cgo/mingw toolchain limitation on this machine (STATE.md: "same pre-existing cgo toolchain break already documented for -race in 01-02/01-03"). All verification in this plan was run without `-race` instead; the plan's specified test commands otherwise pass cleanly.
- **One flaky cross-package test observed under `go test ./...`**: `TestDetectMusicBrainz_FiltersByReleaseType` failed once with `relation "events" does not exist`, caused by `internal/db`'s `TestRunMigrations_AppliesFromScratch` dropping and recreating the entire `public` schema while `internal/detection`'s own test binary ran concurrently against the same shared Postgres instance (Go runs separate packages' test binaries in parallel by default). Re-running `internal/detection` alone passed cleanly, confirming this is a pre-existing test-isolation hazard unrelated to this plan's changes — out of scope per the deviation rules' scope boundary (not this task's file, not this task's change).

## Self-Check: PASSED

- `internal/db/migrations/000004_events_display_fields.up.sql` — FOUND
- `internal/db/migrations/000004_events_display_fields.down.sql` — FOUND
- `internal/discord/client.go` — FOUND
- `internal/discord/client_test.go` — FOUND
- `internal/notifier/notifier.go` — FOUND
- `internal/notifier/format.go` — FOUND
- `internal/notifier/notifier_test.go` — FOUND
- Commit 0428396 — FOUND (`git log --oneline --all`)
- Commit 02bb80d — FOUND
- Commit 67e9dea — FOUND
