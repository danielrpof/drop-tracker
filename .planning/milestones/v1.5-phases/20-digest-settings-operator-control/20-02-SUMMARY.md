---
phase: 20-digest-settings-operator-control
plan: "02"
subsystem: api
tags: [httpserver, testing, digest-settings, authgate, csrf]

# Dependency graph
requires:
  - phase: 20-digest-settings-operator-control
    plan: "01"
    provides: SettingsStore seam, handleGetSettings/handleUpdateSettings, gated GET/PUT /settings/notifications routes
provides:
  - a fully pinned rejection/gate/CSRF/no-leak contract for GET/PUT /settings/notifications, safe for Phase 22's scheduler and the 20-03 SPA panel to build against without re-deriving edge behavior
affects: [20-03-spa-panel, 21-digest-mutual-exclusion, 22-digest-scheduler]

# Actuals (#2632)
actuals:
  tokens: 4770
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "settings-route test double: fakeSettingsStore (func-field + atomic call counter), mirroring fakeStatusStore/fakeWatchlistCounter"
    - "real-session-via-real-login test pattern for gated route CSRF/auth cases (POST /session, lift dt_session cookie, attach explicitly with a nil-Jar client) instead of hand-forged tokens"

key-files:
  created: []
  modified:
    - internal/httpserver/settings_test.go

key-decisions:
  - "No production code changed in this plan: every one of the 13 new test cases (6 rejection-path, 7 gate/CSRF/503/no-leak) passed against plan 20-01's unmodified handler on first run, confirming the plan's own prediction that this would land as a pinning-test-only commit pair."

requirements-completed: [DGST-01, DGST-02, DGST-04]

coverage:
  - id: D8
    description: "A PUT carrying an unrecognised, omitted, or wrong-typed cadence -- or an extra unknown field, or a trailing concatenated JSON value -- is rejected 400 with the stored row unchanged"
    requirement: DGST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_PutRejectsUnknownCadence, #TestSettings_PutRejectsOmittedCadence, #TestSettings_PutRejectsUnknownFields, #TestSettings_PutRejectsWrongType, #TestSettings_PutRejectsTrailingJSONValue"
        status: pass
    human_judgment: false
  - id: D9
    description: "An oversize PUT body (past the shared 64 KiB ceiling) is rejected 4xx, never 5xx, and never persists"
    requirement: DGST-02
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_PutRejectsOversizeBody"
        status: pass
    human_judgment: false
  - id: D10
    description: "Digest defaults (disabled/daily) survive every rejected write on a fresh schema"
    requirement: DGST-04
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#assertSettingsDefaults (invoked at the end of every rejection case)"
        status: pass
    human_judgment: false
  - id: D11
    description: "A gated instance with no session cookie answers 401 (not 403) on both verbs, and the 401 body carries no settings key"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_GetGated401NoCookie, #TestSettings_PutGated401NoCookie"
        status: pass
    human_judgment: false
  - id: D12
    description: "A gated PUT with a real session but no X-Requested-With header is refused 403 with the store never called; with the header it succeeds 200 with the store called exactly once"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_PutGatedForbiddenWithoutCSRFHeader, #TestSettings_PutGatedSucceedsWithCookieAndHeader"
        status: pass
    human_judgment: false
  - id: D13
    description: "An inert (ungated) server never answers 401 on either verb; a server built without WithSettings answers 503 with the shared fixed body on both verbs"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_UngatedNeverAnswers401, #TestSettings_NotConfiguredAnswers503"
        status: pass
    human_judgment: false
  - id: D14
    description: "A store error whose text embeds a DSN password and a webhook token never reaches the raw response body on either verb"
    requirement: DGST-01
    verification:
      - kind: integration
        ref: "internal/httpserver/settings_test.go#TestSettings_NoLeak"
        status: pass
    human_judgment: false
  - id: D15
    description: "Real-time notification behavior (internal/notifier, internal/poller, internal/detection) is byte-for-byte unchanged"
    requirement: DGST-04
    verification:
      - kind: other
        ref: "git diff --name-only -- internal/notifier internal/poller internal/detection (prints nothing)"
        status: pass
    human_judgment: false

duration: 40min
completed: 2026-09-13
status: complete
---

# Phase 20 Plan 02: HTTP Hardening Summary

**Thirteen new real-request test cases pin every rejection, gate, CSRF, 503, and no-leak guarantee on GET/PUT /settings/notifications -- all pass against plan 20-01's handler unmodified, so this plan is a pure pinning-test commit pair with zero production-code change.**

## Performance

- **Duration:** ~40 min
- **Tasks:** 2
- **Files modified:** 1 (`internal/httpserver/settings_test.go`, both tasks)

## Accomplishments
- Task 1 extended `settings_test.go` with six real-Postgres rejection cases (`TestSettings_PutRejects*`): unrecognised cadence, omitted cadence (full-object PUT semantics), unknown field (`digest_last_sent_at`, proving the watermark is unsettable through the route), wrong JSON type, a trailing concatenated JSON value, and an oversize body past the shared 64 KiB ceiling. Every case ends with a GET proving the pre-request defaults (`digest_enabled=false`, `digest_cadence=daily`) survived.
- A local `newSettingsRejectionServer` helper factors the real-Postgres/isolated-schema server wiring `TestSettings_RoundTrip` established, following `newStatusServer`'s setup-helper precedent without adding anything to the production package.
- Task 2 extended the file with seven handler-/router-level cases driven by an in-file `fakeSettingsStore` (func-field double with an atomic call counter, mirroring `fakeStatusStore`/`fakeWatchlistCounter`): gated 401-not-403 on both verbs with no cookie, a real session (minted via a genuine `POST /session` login, never hand-forged) refused 403 with zero store calls when `X-Requested-With` is missing, the same session succeeding 200 with exactly one store call when the header is present, an inert server never answering 401, a server built without `WithSettings` answering 503 with the shared fixed body on both verbs, and a store error embedding a DSN password and a Discord webhook token never reaching the raw response body on either verb (`TestSettings_NoLeak`, table-driven get/update-error cases).
- Both `high`-severity threats registered in the plan (T-20-07 spoofing on the gated routes, T-20-08 CSRF tampering on the write verb) are now proven by live behavioral tests rather than resting on the structural claim that `registerDataRoutes` inherits the gate.

## Task Commits

Each task was committed atomically:

1. **Task 1: Pin every PUT rejection path -- and prove nothing persisted** - `0837754` (test)
2. **Task 2: Pin the gate, the CSRF requirement, the unconfigured 503, and the no-leak guarantee** - `42001e6` (test)

## Files Created/Modified
- `internal/httpserver/settings_test.go` - both tasks' additions: `newSettingsRejectionServer`, `putSettingsRaw`/`getSettingsRaw`/`assertSettingsDefaults` helpers, six rejection-path tests (Task 1); `fakeSettingsStore`, `newSettingsServer`, `loginForSettings`, seven gate/CSRF/503/no-leak tests (Task 2)

## Decisions Made
- No production code changed in either task -- every new case passed against plan 20-01's unmodified `internal/httpserver/settings.go` on the first run. The plan explicitly called this out as the expected outcome ("If every case passes immediately, that is the expected outcome... this task is a pinning-test-only commit"), and both tasks landed exactly that way.

## Deviations from Plan
None - plan executed exactly as written. Both tasks' `<action>` sections anticipated a pinning-test-only outcome as the likely (not fallback) result, and that is what happened.

## Issues Encountered
None. `go test -race` remains unusable on this Windows dev box (documented pre-existing limitation, see `.planning/WINDOWS.md`); the full-suite verification substituted a plain `go test ./... -p 1 -count=1 -coverprofile=coverage.out -coverpkg=$COVER_PKGS` run using the same `COVER_PKGS` set the Makefile computes, matching the established substitution precedent (Phases 11.1, 15, 18, 20-01). Backend coverage measured 90.72% (floor 80%). `make sqlc-check` reported no drift.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- The full HTTP contract for `GET`/`PUT /settings/notifications` (happy path from 20-01, every rejection/gate/CSRF/503/no-leak path from this plan) is now frozen and test-pinned. Plan 20-03 (SPA panel) and Phase 22 (digest scheduler) can build against it with no further backend changes anticipated.
- No blockers. Real-time notification behavior (`internal/notifier`, `internal/poller`, `internal/detection`) confirmed untouched (`git diff --name-only` against those three packages prints nothing across both commits).

## Self-Check: PASSED

`internal/httpserver/settings_test.go` verified present on disk with both tasks' additions; commit hashes `0837754` and `42001e6` verified present in `git log --oneline --all`.

---
*Phase: 20-digest-settings-operator-control*
*Completed: 2026-09-13*
