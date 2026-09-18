---
phase: 20-digest-settings-operator-control
verified: 2026-09-13T18:00:00Z
status: passed
score: 5/5 must-haves verified
behavior_unverified: 0
overrides_applied: 0
---

# Phase 20: Digest Settings & Operator Control Verification Report

**Phase Goal:** An operator can turn digest mode on and pick daily or weekly from inside the app, and that choice sticks across restarts — while notification behavior stays exactly what v1.4 shipped.
**Verified:** 2026-09-13
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Digest panel shows mode, cadence, last-sent (explicit "never sent yet"); operator changes mode/cadence and reload reflects it | ✓ VERIFIED | `web/app/components/system/DigestSettings.tsx` renders all three rows with locked copy; `web/app/routes/system.tsx` fetches via `Promise.all([getStatus(), getDigestSettings()])` on mount/Refresh; 40/40 frontend tests pass (`DigestSettings.test.tsx`, `system.test.tsx`), including the null-watermark "Never sent yet" case and the populated `<time>` case |
| 2 | Setting lives in Postgres (not env var/process memory); takes effect without rebuild/redeploy/restart; survives container restart | ✓ VERIFIED | `internal/settings/settings.go` holds no cache (`grep -cE "sync\.(Once\|RWMutex\|Mutex\|Map)"` = 0); every `Get`/`Update` is a fresh sqlc query; `TestSettings_RoundTrip` and `TestService_UpdateRoundTripsThroughPostgres` prove a value written by one `sqlc.New(pool)` instance is read back by an independent instance — the process-restart equivalent; no env var reads digest state anywhere in `cmd/server/main.go` |
| 3 | Fresh install reports digest off/default cadence; every notification path behaves exactly as v1.4 (one Discord message per event, same spacing, same mark-notified ack) | ✓ VERIFIED | Migration `000008_notification_settings.up.sql` defaults `digest_enabled=false`, `digest_cadence='daily'`, seeds `id=1`; `TestService_FreshSchemaSeedsOneRowWithDefaults` passes against a real freshly-migrated schema; `git log --oneline --all -- internal/notifier internal/poller internal/detection` shows the most recent commit touching those packages predates Phase 20 entirely (last touched in Phase 18.1) — confirmed independently, not just via SUMMARY claim |
| 4 | Settings routes sit behind the instance gate: 401 without session; PUT with bad cadence/malformed body rejected 4xx, never persisted | ✓ VERIFIED | `registerDataRoutes` registers both verbs only inside the gated `chi.Group` (`internal/httpserver/server.go`); `TestSettings_GetGated401NoCookie`, `TestSettings_PutGated401NoCookie`, `TestSettings_PutGatedForbiddenWithoutCSRFHeader`, and six `TestSettings_PutRejects*` cases (unknown cadence, omitted cadence, unknown field, wrong type, trailing JSON, oversize body) all pass and each rejection case re-GETs to confirm the stored row is unchanged |
| 5 | Exactly one settings row; no code path creates a second; concurrent writes cannot fork configuration | ✓ VERIFIED | `CHECK (id = 1)` in the migration plus a single seed `INSERT`; the only write query is `UPDATE ... WHERE id = 1` (`queries/notification_settings.sql`) — no `INSERT` code path exists anywhere in the application; `TestService_SecondRowRejectedByCheckConstraint` proves a manual `id=2` insert is rejected and the count stays 1. Concurrent-write serialization follows structurally from Postgres row-level locking on a single fixed-id `UPDATE` (no application code path can produce two rows to fork between) |

**Score:** 5/5 truths verified (0 present, behavior-unverified)

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/db/migrations/000008_notification_settings.up.sql` / `.down.sql` | Singleton table + seed, paired down migration | ✓ VERIFIED | Exists, `CHECK (id = 1)`, `CHECK (digest_cadence IN (...))`, seed INSERT present; down file drops the table; `migration-check --mode=scan` reports zero findings |
| `queries/notification_settings.sql` | Get/Update queries | ✓ VERIFIED | Both named queries present, `UPDATE ... WHERE id = 1` confirmed |
| `internal/db/sqlc/notification_settings.sql.go` (+models/querier) | Generated code | ✓ VERIFIED | `make sqlc-check` exits 0, no diff |
| `internal/settings/settings.go` + `settings_test.go` | Store/Service, real-Postgres tests | ✓ VERIFIED | 6 test functions, all pass against live Postgres; no cache fields |
| `internal/httpserver/settings.go` + `settings_test.go` | Handlers, full rejection/gate/CSRF/leak test suite | ✓ VERIFIED | 14 test functions, all pass; fixed-body error responses confirmed in source |
| `cmd/server/main.go` wiring | `settings.NewService` → `httpserver.WithSettings` | ✓ VERIFIED | Both call sites present and connected |
| `web/app/lib/api.ts` | `NotificationSettings`, `getDigestSettings`, `updateDigestSettings` | ✓ VERIFIED | Field-for-field match with Go `settingsResponse`; full-object PUT confirmed |
| `web/app/components/system/DigestSettings.tsx` | Mode/cadence/last-sent card | ✓ VERIFIED | All three rows present, locked copy strings present, no `<Button>`, no run-health palette tokens |
| `web/app/components/ui/select.tsx` | Vendored shadcn Select | ✓ VERIFIED | Wraps `@base-ui/react/select`; `web/package.json`/`pnpm-lock.yaml` unmodified |
| `web/app/routes/system.tsx` | Mount/Refresh integration | ✓ VERIFIED | `Promise.all` appears twice (mount + refresh); `DigestSettings` rendered between `AboutInstance` and the empty-watchlist alert |
| `internal/webassets/build/client` | Rebuilt embedded bundle | ✓ VERIFIED | `grep -rl "Digest notifications"` finds the string inside the committed built JS asset; last commit touching this path is `0beceb6` (Phase 20's final commit) |

### Key Link Verification

| From | To | Via | Status | Details |
|------|-----|-----|--------|---------|
| migration 000008 seed INSERT | `GetNotificationSettings` | fresh-schema read | ✓ WIRED | `TestService_FreshSchemaSeedsOneRowWithDefaults` passes |
| `registerDataRoutes` | `gate.Authenticate` + `gate.RequireCSRFHeader` | chi Group middleware | ✓ WIRED | Both verbs registered exclusively inside the gated group in `server.go`; live 401/403 tests pass |
| `cmd/server/main.go` | `httpserver.WithSettings` | composition root | ✓ WIRED | `settingsStore := settings.NewService(sqlc.New(pool))` then `httpserver.WithSettings(settingsStore)` passed into `httpserver.New(...)` |
| handler cadence allow-list | `Service.Update` re-validation | `settings.ParseCadence` called twice | ✓ WIRED | Confirmed in both `internal/httpserver/settings.go` and `internal/settings/settings.go`; DB CHECK is the third layer |
| `system.tsx` mount effect | `Promise.all([getStatus(), getDigestSettings()])` | shared `loadError` | ✓ WIRED | Both calls in one `Promise.all`, confirmed by grep count = 2, and by `system.test.tsx`'s non-401 shared-error case passing |
| `DigestSettings` switch/select | `updateDigestSettings` → `apiFetch` PUT | `save()` helper | ✓ WIRED | Single shared `save()` used by both controls, always sends both fields; confirmed by cadence full-object-PUT test |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|---------------------|--------|
| `DigestSettings` card | `digest_enabled`/`digest_cadence`/`digest_last_sent_at` | `GET /settings/notifications` → `settings.Service.Get` → sqlc query → Postgres row | Yes | ✓ FLOWING |
| PUT save path | new mode/cadence | `updateDigestSettings` → `PUT /settings/notifications` → `Service.Update` → `UPDATE ... RETURNING *` | Yes | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Backend settings package tests | `go test ./internal/settings/... -run TestService -v` (real Postgres) | 6/6 PASS | ✓ PASS |
| Backend HTTP settings tests | `go test ./internal/httpserver/... -run TestSettings -v` (real Postgres) | 14/14 PASS | ✓ PASS |
| Frontend digest/system tests | `pnpm exec vitest run DigestSettings.test.tsx system.test.tsx` | 40/40 PASS | ✓ PASS |
| Migration scan | `go run ./cmd/migration-check --mode=scan --files=...000008...` | "No destructive or unsafe-forward migration statements found." | ✓ PASS |
| sqlc drift check | `make sqlc-check` | exit 0, no diff | ✓ PASS |
| Go build/vet | `go build ./... && go vet ./...` | clean | ✓ PASS |
| golangci-lint (settings/httpserver) | `golangci-lint run ./internal/settings/... ./internal/httpserver/...` | "0 issues." | ✓ PASS |
| Real-time code untouched | `git log --oneline --all -- internal/notifier internal/poller internal/detection` | most recent commit predates Phase 20 (Phase 18.1) | ✓ PASS |
| Embedded bundle contains new UI | `grep -rl "Digest notifications" internal/webassets/build/client/` | found in built JS asset | ✓ PASS |
| No dependency drift | `git diff --name-only -- web/package.json web/pnpm-lock.yaml` | empty | ✓ PASS |
| No forbidden frontend patterns | `grep -rn "setInterval\|visibilitychange\|dangerouslySetInnerHTML" web/app` | 0 matches each | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|-------------|-------------|--------|----------|
| DGST-01 | 20-01, 20-02, 20-03 | Toggle digest mode from SPA, Postgres-persisted, no restart | ✓ SATISFIED | Switch control, `updateDigestSettings`, round-trip tests |
| DGST-02 | 20-01, 20-02, 20-04 | Choose daily/weekly cadence from SPA | ✓ SATISFIED | Cadence `Select` control, three-layer validation, full-object PUT tests |
| DGST-03 | 20-01 | Settings survive process restart (Postgres-backed) | ✓ SATISFIED | No cache in `Service`; independent-instance round-trip test |
| DGST-04 | 20-01, 20-02 | Default digest off; real-time notifications unchanged | ✓ SATISFIED | Migration defaults; `internal/notifier`/`poller`/`detection` diff empty since Phase 18.1 |
| DGST-16 | 20-03, 20-04 | SPA shows last digest send time and mode/cadence | ✓ SATISFIED | `Last digest sent` row, "Never sent yet" / `<time>` branches, tested |

No orphaned requirements: REQUIREMENTS.md maps exactly DGST-01, 02, 03, 04, 16 to Phase 20, and all five are claimed and covered across the four plans.

### Anti-Patterns Found

None blocking. Code review (`20-REVIEW.md`, 2026-09-13) found 0 critical, 2 warning (WR-01: `gofmt` misalignment predating this phase in `server.go`; WR-02: shared body-size constant misleadingly named for a non-watchlist route), 2 info (untested timer-cancel branch; generic 500 on a theoretically-impossible missing-singleton-row case). All four are cosmetic/hardening items, not gaps in the phase's must-haves, and none touch the observable truths above. No `TBD`/`FIXME`/`XXX`/`TODO`/`HACK`/`PLACEHOLDER` markers found in any phase-touched file.

### Human Verification Required

None. All must-haves are backed by passing automated tests re-run independently during this verification (backend against live Postgres, frontend via Vitest), plus direct source inspection of wiring and prohibitions. No plan deferred any `<human-check>` item to end-of-phase.

### Gaps Summary

No gaps. All five ROADMAP success criteria, all five requirement IDs, and every must-have truth/artifact/key-link declared across the four plans were independently verified against the actual codebase and a live Postgres instance — not merely accepted from SUMMARY.md claims. `go build`, `go vet`, `golangci-lint`, `make sqlc-check`, and the targeted Go/Vitest test suites were all re-run in this verification session and passed. The two review warnings (gofmt drift, a misleadingly-named shared constant) and two info items are optional hardening, explicitly non-blocking per `20-REVIEW.md`.

---

_Verified: 2026-09-13_
_Verifier: Claude (gsd-verifier)_
