---
phase: 20-digest-settings-operator-control
reviewed: 2026-09-13T00:00:00Z
depth: standard
files_reviewed: 20
files_reviewed_list:
  - cmd/server/main.go
  - internal/db/migrate_test.go
  - internal/db/migrations/000008_notification_settings.down.sql
  - internal/db/migrations/000008_notification_settings.up.sql
  - internal/db/schema_version_test.go
  - internal/db/sqlc/models.go
  - internal/db/sqlc/notification_settings.sql.go
  - internal/db/sqlc/querier.go
  - internal/httpserver/server.go
  - internal/httpserver/settings.go
  - internal/httpserver/settings_test.go
  - internal/settings/settings.go
  - internal/settings/settings_test.go
  - queries/notification_settings.sql
  - web/app/components/system/DigestSettings.test.tsx
  - web/app/components/system/DigestSettings.tsx
  - web/app/components/ui/select.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/routes/system.test.tsx
  - web/app/routes/system.tsx
findings:
  critical: 0
  warning: 2
  info: 2
  total: 4
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-09-13
**Depth:** standard
**Files Reviewed:** 20
**Status:** issues_found

## Summary

This phase adds a singleton digest-notification-settings resource end to end: migration 000008, sqlc query/model/querier additions, the `internal/settings` service, the `GET/PUT /settings/notifications` HTTP handlers wired through `httpserver.WithSettings`, and the `DigestSettings` React component plus its `System` route integration. I read every listed file, cross-referenced the SQL/Go/TS layers against each other, ran `go vet`, `go build`, `golangci-lint run` (v2.12.2, this repo's pinned config), `gofmt -l`, `sqlc generate` (diffed against the committed generated code), the Go test suite for the touched packages against a live Postgres fixture, and the frontend Vitest suite plus `prettier --check` for the touched web files.

Functionally this phase is solid: the singleton-row invariant is enforced at the DB layer (`CHECK (id = 1)`) and re-verified at the service layer; validation for `digest_cadence` is independently enforced at three layers (HTTP handler, service, DB CHECK) with tests pinning each; the CSRF/gate wiring for the new routes is exercised end to end; the frontend's optimistic-update/re-entrancy-guard/keep-stale-on-failure logic is well covered by tests; `sqlc generate` produces byte-identical output to what's committed (no drift); and `go vet`/`golangci-lint run`/`go build` all pass clean. I found no correctness bugs, no injection or auth-bypass vectors, and no data-loss risk in the code this phase added. The findings below are quality/maintainability items only.

## Warnings

### WR-01: `Server` struct is not `gofmt`-clean after this phase's edit

**File:** `internal/httpserver/server.go:31-45`
**Issue:** `gofmt -l internal/httpserver/server.go` reports the file as needing reformatting. The `Server` struct's field alignment is split into two inconsistent column groups (the pre-existing `db`/`watchlist`/`events`/`sources`/`router` fields align to one tab stop, while `gate`/`schema`/.../`settingsStore` align to a wider one) instead of one gofmt-normalized block:
```
db             Pinger
watchlist      watchlist.Store
events         events.Store
sources        []SearchSource
router         http.Handler
gate             *authgate.Manager   // wider column starts here
schema           SchemaVersioner
...
settingsStore    SettingsStore       // this phase's added field
```
This misalignment predates this phase (present in the parent commit before `settingsStore` was added), but this phase touched the same struct and added a new field to it without correcting the existing drift, so the file the phase leaves behind is still not `gofmt`-clean. `golangci-lint run` against this repo's `.golangci.yml` does not catch it (the config's `standard` linter set has no `formatters:`/`gofmt` entry), so it will not be caught by CI or the pre-commit hook as configured today — but it is a genuine, provable formatting defect any contributor running plain `gofmt -l .` or an editor with format-on-save will immediately see as diff noise on their next unrelated edit to this file.
**Fix:** Run `gofmt -w internal/httpserver/server.go` (or `goimports -w`) and commit the result.

### WR-02: Settings PUT body-size constant is misleadingly named for a non-watchlist route

**File:** `internal/httpserver/settings.go:84`
**Issue:** `handleUpdateSettings` bounds the request body with `http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)`. `maxAddWatchlistBodyBytes` is declared and documented in `internal/httpserver/watchlist.go:21-24` specifically as "bounds the POST /watchlist request body" — its name and doc comment describe a watchlist-only concern, but it is now the shared body-size ceiling for four different routes (`POST /watchlist`, `PATCH /watchlist/{id}`, and now both `GET`'s absence-of-body aside, `PUT /settings/notifications`). This predates this phase's other two watchlist call sites but this phase adds a fourth, unrelated call site to a constant whose name and doc comment actively describe the wrong route.
**Fix:** Rename to a generic name (e.g. `maxJSONBodyBytes`) and update its doc comment to describe it as the shared per-request body ceiling, or introduce a settings-specific constant if a different ceiling is ever wanted. This is a one-line rename plus a doc-comment edit; no behavior change.

## Info

### IN-01: The 2-second "Saved." auto-dismiss and the timer-cancel-on-rapid-resave path are untested

**File:** `web/app/components/system/DigestSettings.tsx:74-93`
**Issue:** Vitest coverage for this file shows lines 75-76 (`clearTimeout(clearTimerRef.current); clearTimerRef.current = null`) and line 92 (`if (mountedRef.current) setStatus("idle")`, the `setTimeout` callback body) as uncovered. No test in `DigestSettings.test.tsx` exercises: (a) a second save starting while an earlier successful save's 2-second "Saved." timer is still pending (to prove the stale timer is cleared and doesn't clobber the second save's own status), or (b) the timer actually firing and reverting `status` from `"saved"` back to `"idle"`. Both branches read as intentional, sensible behavior from the source, but neither is currently pinned by a test, so a regression in either (e.g. a leaked timer flipping status on an unrelated later render) would not be caught.
**Fix:** Add a fake-timers test that triggers two saves in quick succession (second starting before the first's timer fires) and asserts only one status transition sequence occurs, plus a test that advances time past 2000ms after a successful save and asserts `status` reads `"idle"` again (and the "Saved." text is gone).

### IN-02: `GetNotificationSettings` returns a generic 500 with no distinguishing signal if the singleton row is ever missing

**File:** `internal/settings/settings.go:87-93`, `queries/notification_settings.sql:1-2`
**Issue:** `Get` wraps any error from `GetNotificationSettings` (including `pgx.ErrNoRows`, should the seeded `id = 1` row ever be missing — e.g., a hand-run `DELETE` bypassing the app, or a partial/corrupted migration state) into a generic `"get notification settings: %w"` error, which `handleGetSettings` turns into an opaque `500 internal error`. This matches the documented design ("on a from-scratch database this is D-05's seeded default row, never pgx.ErrNoRows") and is not a bug given the invariants this phase establishes (no app-level delete path, `CHECK (id = 1)` blocking any second row), but there is no explicit `errors.Is(err, pgx.ErrNoRows)` branch to produce a clearer log signal (e.g. "singleton row missing — was it manually deleted?") if that invariant is ever violated out-of-band.
**Fix:** Optional hardening only — not required by this phase's stated scope. If desired, add an explicit `pgx.ErrNoRows` check in `Service.Get`/`Service.Update` that logs a distinct message before falling through to the same generic wrapped error, so an operator debugging a 500 on this route doesn't have to guess between "DB down" and "someone deleted the singleton row."

---

_Reviewed: 2026-09-13_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
