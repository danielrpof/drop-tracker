---
phase: 02-watchlist-core
plan: 01
subsystem: watchlist-api
tags: [postgres, sqlc, chi, watchlist, migration]
dependency-graph:
  requires: []
  provides:
    - internal/watchlist.Store
    - internal/watchlist.Service
    - httpserver.New(db, store, logger) three-arg constructor
    - POST /watchlist endpoint
    - artists/watchlist Postgres schema
  affects:
    - internal/httpserver (constructor signature, all 8 call sites)
    - internal/db/sqlc (regenerated: Querier interface, Artist/Watchlist models)
tech-stack:
  added: []
  patterns:
    - "Narrow Store interface (Pinger-analog) as the second httpserver.New dependency"
    - "text[] + CHECK constraint instead of native Postgres enum for growable value sets"
    - "emit_interface + emit_pointers_for_null_types sqlc config for testable seams and clean JSON nulls"
key-files:
  created:
    - internal/db/migrations/000002_watchlist.up.sql
    - internal/db/migrations/000002_watchlist.down.sql
    - queries/artists.sql
    - queries/watchlist.sql
    - internal/db/sqlc/querier.go
    - internal/db/sqlc/artists.sql.go
    - internal/db/sqlc/watchlist.sql.go
    - internal/watchlist/service.go
    - internal/httpserver/watchlist.go
    - internal/httpserver/watchlist_test.go
  modified:
    - sqlc.yaml
    - internal/db/sqlc/models.go
    - internal/httpserver/server.go
    - internal/httpserver/health_test.go
    - internal/httpserver/server_test.go
    - internal/httpserver/boot_e2e_test.go
    - cmd/server/main.go
    - internal/db/migrate_test.go
decisions:
  - "internal/db/migrate_test.go's TestRunMigrations_AppliesFromScratch now resets by dropping and recreating the whole public schema, not just schema_migrations -- required once 000002 creates real domain tables, and stays correct for every future migration this project adds"
  - "TestWatchlist_AddEndToEnd deletes its own artists row (cascading to watchlist) before and after running, since testutil.NewTestPool only resets schema, not table contents -- keeps the test rerun-safe ahead of plan 02-02's duplicate-detection work"
  - "Tracer feedback gate: automated <verify> (build, vet, full suite, targeted watchlist tests, sqlc-check, constraint inspection, migration down/up) all passed; per this project's config (mode: yolo, human_verify_mode: end-of-phase) execution continued without an interactive per-plan checkpoint -- human verification is batched at phase end"
metrics:
  duration: 75m
  completed: 2026-08-06
status: complete
actuals:
  tokens: 8888
  tasks: 1
  commits: 1
---

# Phase 2 Plan 01: Watchlist Core End-to-End Tracer Summary

Proved the whole watchlist stack in one thin vertical slice: `POST /watchlist` with an mbid and
a name lands a real row in Postgres and comes back as JSON with its D-08 default preferences
(all four release types on, nothing muted).

## What Was Built

- **Schema (`internal/db/migrations/000002_watchlist.up.sql`/`.down.sql`):** `artists` (master
  data keyed on `mbid`, D-01/D-03) and `watchlist` (per-artist entry with `release_types` and
  `muted_event_types` as `text[]` + `CHECK` constraints, not native enums -- Pitfall 1). Both
  preference columns default to D-08's "everything on, nothing muted."
- **sqlc config + generated code:** `emit_interface: true` and
  `emit_pointers_for_null_types: true` added to `sqlc.yaml`. Regenerated `internal/db/sqlc/`
  now exports a `Querier` interface (`UpsertArtist`, `CreateWatchlistEntry`, plus the existing
  `Ping`) and nullable columns (`DeezerID`, `Disambiguation`, `ImageUrl`) as `*string`.
- **`internal/watchlist` (new package):** `Store` interface (`Add`, `List`, `UpdatePreferences`,
  `Remove`), `Service` implementing it against `sqlc.Querier`. `Add` is fully real: upserts the
  artist by mbid, then inserts the watchlist row (no transaction -- D-03 treats an artist row
  with no watchlist row as legitimate). `List`/`UpdatePreferences`/`Remove` are declared now so
  the interface never reshapes, but return `errNotImplemented` -- no route is registered against
  any of them yet, so nothing half-built is reachable over HTTP.
- **`internal/httpserver/watchlist.go`:** `handleAddWatchlist` decodes into a purpose-built DTO
  with `DisallowUnknownFields()` (rejects over-posted `id`/`artist_id`/etc. with 400 -- T-02-01),
  trims and validates `mbid`/`name` (D-04), and never lets a downstream error's raw text reach
  the response body (`writeError` + `httplog.SetAttrs` -- T-02-03/D-13).
- **`httpserver.New` widened** to `New(db Pinger, store watchlist.Store, logger *slog.Logger) *Server`.
  `Pinger` itself was left untouched (still one method) so `stubPinger` in `health_test.go` stays
  valid. All eight existing call sites were updated in this same commit: `cmd/server/main.go`,
  `boot_e2e_test.go` (both wired to a real `watchlist.NewService(sqlc.New(pool))`), and six test
  call sites across `health_test.go`/`server_test.go` (wired to a zero-value `stubStore{}`, since
  those tests never touch watchlist routes).

## Verification

All of this plan's `<verify>` and `<acceptance_criteria>` commands were run directly (no `make`
available in this shell session -- ran the equivalent raw commands instead):

- `docker compose up -d --wait postgres` -- healthy
- `sqlc generate` -- clean regeneration, `git diff` after commit shows the committed generated
  code matches
- `go build ./...` and `go vet ./...` -- both exit 0
- `TEST_DATABASE_URL=... go test ./... -count=1` -- full suite green, including the four
  new watchlist tests and every pre-existing Phase 1 test (health, request-ID, boot e2e, config,
  db migration tests)
- `go test ./internal/httpserver/ -run 'TestWatchlist_Add' -v` -- all four pass, run twice in a
  row to confirm the end-to-end test is rerun-safe
- `psql ... \d watchlist` -- confirmed `watchlist_artist_id_key`, `watchlist_release_types_valid`,
  `watchlist_muted_event_types_valid` constraints all present
- Migration reversibility exercised directly via `go run .../migrate/v4/cmd/migrate down 1` then
  `up` -- `artists`/`watchlist` cleanly dropped and recreated
- `go mod verify` -- ok; `go mod tidy` made no changes (the `jackc/pgerrcode` vetted pin stays
  indirect, untouched, per this task's explicit prohibition on `go get -u`-ing it)
- Repository-wide `httpserver.New(` grep confirmed all eight call sites (02-RESEARCH.md's
  Pitfall 5 undercounted at five; `server_test.go`'s two call sites bring it to eight, matching
  02-VALIDATION.md's corrected count)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `TestRunMigrations_AppliesFromScratch` broke once 000002 created real tables**
- **Found during:** first full-suite run after adding the migration
- **Issue:** The test reset itself by dropping only `schema_migrations` before rerunning
  `RunMigrations`, which was safe when migration 000001 was a no-op. With 000002 now creating
  `artists`/`watchlist`, a rerun against a database that already had those tables (from an
  earlier test run) failed with "Dirty database version 2. Fix and force version."
- **Fix:** Changed the reset to `DROP SCHEMA public CASCADE; CREATE SCHEMA public` so the test
  proves a true from-scratch apply regardless of what any current or future migration creates.
  Also updated the test's hardcoded expected post-migration version from 1 to 2.
- **Files modified:** `internal/db/migrate_test.go`
- **Commit:** 965ccc4

**2. [Rule 1 - Bug] `TestWatchlist_AddEndToEnd` was not rerun-safe**
- **Found during:** running the targeted watchlist test twice in a row to confirm stability
- **Issue:** `testutil.NewTestPool` only re-applies migrations (schema), it does not reset table
  contents between test runs. A second run against the same Postgres instance hit the
  `watchlist_artist_id_key` unique constraint on the artist inserted by the first run, and since
  duplicate-error translation to a 409 is explicitly deferred to plan 02-02, the raw pg error
  fell through to a 500 -- an unrelated test failure that would have looked like a regression.
- **Fix:** The test now deletes its own `artists` row for the fixed mbid (cascading to its
  watchlist row) both before and after running via `t.Cleanup`.
- **Files modified:** `internal/httpserver/watchlist_test.go`
- **Commit:** 965ccc4

No other deviations -- the schema, sqlc config, service, and handler were built exactly as
02-RESEARCH.md's Code Examples and 02-PATTERNS.md's analogs specified.

## Known Stubs

`internal/watchlist/service.go`'s `List`, `UpdatePreferences`, and `Remove` methods return a
package-level `errNotImplemented` sentinel. This is intentional and plan-documented, not a
silent gap: no HTTP route is registered against any of the three, so nothing reachable over the
API depends on them. Plan 02-03 fills `List`/`Remove`; plan 02-04 fills `UpdatePreferences` and
carries the gate proving none of these bodies survive to the end of the phase.

## Threat Flags

None -- every new trust-boundary-crossing surface this task introduces (`POST /watchlist`,
`internal/watchlist` -> Postgres) was already registered in this plan's own `<threat_model>`
(T-02-01, T-02-02, T-02-03, T-02-06, T-02-SC) and mitigated as described in "What Was Built."

## Self-Check: PASSED

- `internal/db/migrations/000002_watchlist.up.sql` -- FOUND
- `internal/db/migrations/000002_watchlist.down.sql` -- FOUND
- `internal/watchlist/service.go` -- FOUND
- `internal/httpserver/watchlist.go` -- FOUND
- `internal/httpserver/watchlist_test.go` -- FOUND
- `queries/artists.sql` -- FOUND
- `queries/watchlist.sql` -- FOUND
- Commit `965ccc4` -- FOUND in `git log --oneline --all`
