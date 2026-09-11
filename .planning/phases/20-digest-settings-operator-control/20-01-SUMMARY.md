---
phase: 20-digest-settings-operator-control
plan: "01"
subsystem: api
tags: [postgres, sqlc, chi, digest-settings, migrations]

# Dependency graph
requires:
  - phase: 18-backend-readiness-poll-run-history-status-api
    provides: consumer-declared narrow-seam pattern (StatusStore/WatchlistCounter), functional-option server wiring (WithStatus), gated registerDataRoutes convention
  - phase: 19-frontend-system-view
    provides: the /system view this phase's cadence/toggle data will surface in (plan 20-03)
provides:
  - a singleton notification_settings Postgres row (CHECK id=1 + seed INSERT, D-05)
  - internal/settings.Store — narrow Get/Update seam, no cache, sqlc-backed
  - gated GET/PUT /settings/notifications routes inside registerDataRoutes
affects: [20-02-http-hardening, 20-03-spa-panel, 21-digest-mutual-exclusion, 22-digest-scheduler]

# Actuals (#2632)
actuals:
  tokens: 7765
  tasks: 2
  commits: 3

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "singleton table via CHECK (id = 1) + migration-time seed INSERT, no upsert-on-read"
    - "narrow consumer-declared Store seam (settings.Store, httpserver.SettingsStore) mirroring internal/pollruns.Store"
    - "three-layer cadence validation: HTTP handler -> Service.Update re-validation -> DB CHECK backstop"

key-files:
  created:
    - internal/db/migrations/000008_notification_settings.up.sql
    - internal/db/migrations/000008_notification_settings.down.sql
    - queries/notification_settings.sql
    - internal/db/sqlc/notification_settings.sql.go
    - internal/settings/settings.go
    - internal/settings/settings_test.go
    - internal/httpserver/settings.go
    - internal/httpserver/settings_test.go
  modified:
    - internal/db/migrate_test.go
    - internal/db/schema_version_test.go
    - internal/db/sqlc/models.go
    - internal/db/sqlc/querier.go
    - internal/httpserver/server.go
    - cmd/server/main.go

key-decisions:
  - "digest_last_sent_at maps through sqlc as pgtype.Timestamptz, not *time.Time (emit_pointers_for_null_types only applies to types with no native null-representable pgtype); settings.Service.toSettings converts it to *time.Time by hand, matching the events.Service/NotifiedAt precedent already in this codebase."

patterns-established:
  - "Pattern 3: singleton-row schema (CHECK id = 1 + seed INSERT) as the enforcement mechanism instead of app-level ensure-a-row logic — reusable for any future instance-wide config table"

requirements-completed: [DGST-01, DGST-02, DGST-03, DGST-04]

coverage:
  - id: D1
    description: "A fresh migrated database carries exactly one notification_settings row (digest off, cadence daily, watermark NULL)"
    requirement: DGST-04
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_FreshSchemaSeedsOneRowWithDefaults"
        status: pass
    human_judgment: false
  - id: D2
    description: "GET/PUT /settings/notifications round-trip through Postgres, sitting inside the gated route group"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_RoundTrip"
        status: pass
    human_judgment: false
  - id: D3
    description: "Replaying an identical PUT is a no-op on observable state (idempotent singleton update)"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_UpdateIsIdempotent"
        status: pass
    human_judgment: false
  - id: D4
    description: "A second settings row (id=2) is rejected by the CHECK constraint; row count stays 1"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_SecondRowRejectedByCheckConstraint"
        status: pass
    human_judgment: false
  - id: D5
    description: "An unrecognised digest_cadence is rejected before touching Postgres and the stored row is unchanged"
    requirement: DGST-02
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_UpdateRejectsUnknownCadence"
        status: pass
    human_judgment: false
  - id: D6
    description: "digest_last_sent_at is never written by this phase — stays NULL across multiple updates"
    requirement: DGST-04
    verification:
      - kind: integration
        ref: "internal/settings/settings_test.go#TestService_LastSentWatermarkUntouched"
        status: pass
    human_judgment: false
  - id: D7
    description: "Real-time notification behavior (internal/notifier, internal/poller, internal/detection) is byte-for-byte unchanged"
    requirement: DGST-04
    verification:
      - kind: other
        ref: "git diff --name-only -- internal/notifier internal/poller internal/detection (prints nothing)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-09-11
status: complete
---

# Phase 20 Plan 01: Digest Settings Store & Gated Routes Summary

**Singleton `notification_settings` Postgres row (CHECK id=1 + seed row) reachable through gated GET/PUT /settings/notifications, backed by a narrow no-cache `internal/settings.Store`**

## Performance

- **Duration:** ~45 min
- **Tasks:** 2
- **Files modified:** 14 (8 created, 6 modified — includes generated sqlc output)

## Accomplishments
- Migration `000008_notification_settings` creates the singleton table (`CHECK (id = 1)`, `digest_cadence` inline `CHECK`, nullable `digest_last_sent_at`) plus the D-05 seed `INSERT`, with schema-version pins bumped from 7 to 8 in the same commit.
- `internal/settings.Store` — a narrow `Get`/`Update` seam over generated sqlc queries, holding no cache/memoised state, re-validating cadence server-side as the middle of three independent validation layers (handler → Service → DB CHECK).
- `GET`/`PUT /settings/notifications` wired inside `registerDataRoutes`, inheriting `gate.Authenticate` + `gate.RequireCSRFHeader` structurally, with no new middleware or path allowlist.
- Composition-root wiring in `cmd/server/main.go` (`settings.NewService(sqlc.New(pool))` → `httpserver.WithSettings(...)`).
- Tracer test (`TestSettings_RoundTrip`) proves the whole path against a real, freshly migrated Postgres schema: default GET, PUT update, second GET reads the change back from Postgres, not an in-process cache.
- Six real-Postgres tests in `internal/settings/settings_test.go` pin the singleton, idempotency, watermark, and cadence-bypass guarantees the plan's `must_haves.truths` required.

## Task Commits

Each task was committed atomically:

1. **Task 1: End-to-end digest setting — Postgres singleton row through gated GET/PUT** - `44370ed` (feat)
2. **Task 2: Pin the singleton, idempotency and untouched-watermark guarantees at the store** - `9743acd` (test)
3. **Fix: drop literal route path from a `registerDataRoutes` comment** (see Deviations) - `97335b0` (fix)

## Files Created/Modified
- `internal/db/migrations/000008_notification_settings.up.sql` / `.down.sql` - the singleton table + seed row, paired down migration
- `internal/db/migrate_test.go`, `internal/db/schema_version_test.go` - schema-version pins bumped 7 → 8
- `queries/notification_settings.sql` - `GetNotificationSettings`/`UpdateNotificationSettings` sqlc queries
- `internal/db/sqlc/notification_settings.sql.go`, `models.go`, `querier.go` - regenerated sqlc output (committed, `make sqlc-check` clean)
- `internal/settings/settings.go` - `Cadence`, `ParseCadence`, `Settings`, `UpdateParams`, `Store`, `Service`
- `internal/settings/settings_test.go` - real-Postgres guarantee tests (Task 2)
- `internal/httpserver/settings.go` - `SettingsStore` seam, `settingsResponse`/`updateSettingsRequest`, `handleGetSettings`/`handleUpdateSettings`
- `internal/httpserver/settings_test.go` - end-to-end tracer test
- `internal/httpserver/server.go` - `settingsStore` field, `WithSettings` option, route registration in `registerDataRoutes`
- `cmd/server/main.go` - composition-root wiring

## Decisions Made
- `digest_last_sent_at` comes back from sqlc as `pgtype.Timestamptz` rather than `*time.Time` — `emit_pointers_for_null_types` only kicks in for column types with no native pgtype null representation (e.g. `text`/`int4`); `timestamptz` always gets the struct form. `settings.Service`'s `toSettings` converts it to `*time.Time` by hand (nil when `!Valid`), the same pattern `events.Service` already uses for `NotifiedAt`. Not a deviation from the plan's intent (the `Settings` struct still exposes `*time.Time`), just a correction to an incidental assumption in the plan text about which sqlc knob produces that shape.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `registerDataRoutes` comment tripped its own acceptance grep**
- **Found during:** Task 1, post-implementation acceptance-criteria check
- **Issue:** The route-registration comment spelled out the literal `/settings/notifications` path unquoted, so `awk '/^func registerDataRoutes/,/^}/' ... | grep -c '/settings/notifications'` counted 3 occurrences (the comment plus both `r.Get`/`r.Put` calls) instead of the required 2.
- **Fix:** Reworded the comment to describe "the digest settings resource" instead of repeating the literal path string.
- **Files modified:** `internal/httpserver/server.go`
- **Verification:** Both grep-based acceptance criteria (`"/settings/notifications"` quoted count = 2, unquoted in-function count = 2) now pass; full build/vet/lint/test suite reconfirmed green.
- **Committed in:** `97335b0`

---

**Total deviations:** 1 auto-fixed (Rule 1 — bug in a self-verifying comment)
**Impact on plan:** Cosmetic-only; no behavior change. No scope creep.

## Issues Encountered
- `internal/detection`'s `TestDetectDeezer_FiltersByRecordType` failed once during a full `go test ./...` run under default package-level parallelism, then passed both in isolation and on an immediate full-suite rerun. `internal/detection` is untouched by this plan (`git diff --name-only -- internal/notifier internal/poller internal/detection` prints nothing across both commits), and the project's own `Makefile`/STATE.md already document pre-existing test-suite flakiness under parallel execution for unrelated packages — treated as a known, unrelated flake, not a regression from this work.
- `go test -race` is unusable on this Windows dev box (documented pre-existing ThreadSanitizer/cgo limitation — see `.planning/WINDOWS.md`); `make test`'s `-race` flag was substituted with a plain `go test ./... -coverprofile=... -coverpkg=...` run using the same `COVER_PKGS` set the Makefile computes, matching the project's established substitution precedent (Phase 11.1, Phase 15).

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The wire contract (`digest_enabled`/`digest_cadence`/`digest_last_sent_at`/`updated_at`) is frozen for plan 20-03's `web/app/lib/api.ts` types.
- Plan 20-02 (HTTP hardening: 401/503/leak tests) can build directly on `SettingsStore`/`handleGetSettings`/`handleUpdateSettings` with no further backend changes needed.
- No blockers. Real-time notification behavior (`internal/notifier`, `internal/poller`, `internal/detection`) is confirmed untouched.

## Self-Check: PASSED

All 8 listed created files verified present on disk; all 3 commit hashes (`44370ed`, `9743acd`, `97335b0`) verified present in git log.

---
*Phase: 20-digest-settings-operator-control*
*Completed: 2026-09-11*
