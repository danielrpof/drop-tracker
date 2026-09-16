---
phase: 01-foundation-data-layer-config-health
plan: 03
subsystem: config
tags: [config, env, fail-fast, tdd]
dependency-graph:
  requires:
    - config.Load
  provides:
    - config.Config (full 9-field surface: DatabaseURL, HTTPPort, LogLevel, LogFormat,
      DiscordWebhookURL, PollInterval, MusicBrainzUserAgent, MusicBrainzRateLimitPerSec,
      DeezerRateLimitPer5s)
    - internal/config/config_test.go (8 tests covering defaults, fail-fast, and
      struct-to-.env.example parity)
  affects: []
tech-stack:
  added: []
  patterns:
    - "reflect.TypeOf over Config's env tags compared as a set against a line-scanned
      .env.example, reporting both directions of drift"
    - "t.Setenv + os.Unsetenv (never os.Setenv) to simulate an absent env var while keeping
      the auto-restore hook active"
key-files:
  created:
    - internal/config/config_test.go
  modified:
    - internal/config/config.go
    - .env.example
decisions:
  - "Renamed the stub field MusicBrainzUA to MusicBrainzUserAgent and changed its default to
    include a contact URL (drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)),
    matching the plan's exact field name/default and CLAUDE.md's requirement that MusicBrainz
    never see a missing/default User-Agent."
  - "TestLoad_AggregatesAllMissing asserts on the Go field name \"HTTPPort\", not the env tag
    \"HTTP_PORT\", for the type-conversion half of the aggregate error. Verified directly
    against caarlos0/env v11.4.1's ParseError.Error() (fmt.Sprintf using reflect.StructField.Name):
    a missing/empty notEmpty variable is named by its env tag, but a type-conversion failure is
    named by the Go struct field, not the tag. The test still proves the required behavior (one
    aggregate error naming every offending setting) with an assertion accurate to the library's
    real output."
  - "Followed a genuine RED->GREEN split for Task 1 despite implementing the struct extension
    before the test file: reverted config.go to its pre-plan state, confirmed the new tests
    failed to compile (undefined fields), committed that as the test(...) commit, then
    reapplied the struct extension as the feat(...) commit."
metrics:
  duration: "~45 min"
  completed: 2026-08-05
actuals:
  tokens: 2686
  tasks: 2
  commits: 3
status: complete
---

# Phase 1 Plan 3: Complete Config Surface and Prove Fail-Fast Summary

Extended `internal/config.Config` from four boot-path fields to the full nine-field surface
this application will need through Phase 5 (Discord webhook, poll interval, MusicBrainz
user-agent and rate limit, Deezer rate limit), documented every field in `.env.example`, and
proved the fail-fast contract end-to-end: unset, empty-string, and simultaneous multi-field
failures all produce a single aggregate error naming every offending variable, while
`.env.example` and `Config` are now held in lockstep by a reflection-based parity test.

## What Was Built

**Task 1** (`internal/config/config.go`, `.env.example`, `internal/config/config_test.go`):
Added `MusicBrainzRateLimitPerSec float64` (default `1`) and `DeezerRateLimitPer5s int`
(default `50`); renamed the existing stub `MusicBrainzUA` to `MusicBrainzUserAgent` with a
default that now embeds a contact URL, since MusicBrainz throttles requests carrying a
missing/default User-Agent. `.env.example` now documents all nine fields, grouped under
"Phase 1: required" and "Stubbed for future phases" headers, with the DSN matching
`docker-compose.yml`. `internal/config/config_test.go` (package `config_test`) added
`TestLoad_Defaults`, `TestLoad_ExplicitValueEqualToDefault`, and
`TestLoad_OptionalUnsetIsNotAnError`.

**Task 2** (`internal/config/config_test.go`): Added `TestLoad_MissingRequired`,
`TestLoad_EmptyRequired`, `TestLoad_AggregatesAllMissing`, `TestEnvExampleCompleteness`
(reflects `Config`'s `env` tags and line-scans `.env.example`, comparing both as sets and
reporting the symmetric difference in both directions on failure), and `TestDotEnvIsNotTracked`
(exact-line `.gitignore` match plus an empty `git ls-files .env`). All environment mutation
goes through `t.Setenv`; the "absent" cases use a `t.Setenv` sentinel followed immediately by
`os.Unsetenv` so the auto-restore hook still fires at test end.

## Verification Performed

- `go build ./... && go vet ./...`: both exit 0.
- `go test ./internal/config/ -short -v -count=1`: all 8 tests `--- PASS`.
- `go test ./... -short -count=1`: full module green (httpserver package's DB-backed tests
  skip cleanly under `-short`, as established in 01-01/01-02).
- Field count: `internal/config/config.go` has exactly 9 `env:"..."` tags (4 boot-path + 5
  later-phase); `.env.example` has exactly 9 lines matching `^[A-Z][A-Z0-9_]*=`.
- `grep -q 'DATABASE_URL,notEmpty'` passes; `grep -q 'DATABASE_URL,required'` does not match —
  the DSN tag is `notEmpty`, not merely `required`.
- `go list -deps ./internal/config | grep godotenv`: no match — no dotenv-file dependency.
- `git diff --stat cmd/server/main.go internal/logging/logging.go`: empty — extending the
  struct changed no consumer.
- `DATABASE_URL= go run ./cmd/server`: exits 1, stderr is
  `load config: env: environment variable "DATABASE_URL" should not be empty`.
- Mutation checks: deleting the `HTTP_PORT=` line from `.env.example` failed
  `TestEnvExampleCompleteness` naming `HTTP_PORT` as the drifted key (then restored via
  `git checkout -- .env.example`); the same test fails symmetrically if a new `Config` field is
  added without a matching `.env.example` line.
- `git ls-files .env` prints nothing; `.gitignore` contains the exact line `.env` (line 151).

### Known environment limitation: `-race` could not be run locally

Same pre-existing, code-independent Windows toolchain issue documented in 01-02's SUMMARY:
`runtime/cgo: cgo.exe: exit status 2` with this Go 1.26.5 + MSYS2 GCC 15.2.0 combination,
reproducible on a trivial program with no application code involved. `go test
./internal/config/ -short -v -count=1` (no `-race`) is green for all 8 tests; this module's CI
(Phase 7, GitHub Actions on Linux) runs a standard toolchain where `-race` is expected to
succeed. All new tests are race-safe by construction (no shared mutable state across
goroutines — this package has no concurrency).

## TDD Gate Compliance

**Task 1** followed a genuine RED -> GREEN cycle: `internal/config/config.go` was reverted to
its pre-plan (four-field-plus-three-stub) state, `internal/config/config_test.go` was added
referencing the not-yet-existing `MusicBrainzUserAgent`, `MusicBrainzRateLimitPerSec`, and
`DeezerRateLimitPer5s` fields, and `go test` failed to compile (`cfg.MusicBrainzUserAgent
undefined`, etc.) — committed as `test(01-03): add failing tests for full Config field surface`
(`5946d56`). The struct extension was then reapplied and all three tests passed — committed as
`feat(01-03): complete Config struct and .env.example to the full app surface` (`242df5a`).

**Task 2** added its five tests against already-correct `Load()` behavior (the fail-fast
mechanics were already implemented; nothing new needed to be built to make them pass) — a
single `test(...)` commit (`bb1af10`), consistent with 01-02's precedent for tasks whose
production behavior already exists.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected `TestLoad_AggregatesAllMissing`'s field-name assertion**
- **Found during:** Task 2, first test run.
- **Issue:** The plan's `<behavior>` describes the aggregate error as naming both
  `DATABASE_URL` and `HTTP_PORT`. Written literally, the test failed:
  `caarlos0/env` v11.4.1's `ParseError.Error()` (`error.go:71`,
  `fmt.Sprintf("parse error on field %q ...", e.Name, ...)`) names a type-conversion failure by
  the Go struct field (`"HTTPPort"`), not the `env` struct tag (`"HTTP_PORT"`) — confirmed by
  reading the vendored module source directly (`env_test.go` examples corroborate this for
  every built-in type).
- **Fix:** Asserted on `"HTTPPort"` instead of `"HTTP_PORT"`, with an inline comment explaining
  the library's actual (tag-name-for-missing, field-name-for-parse-error) behavior. The
  underlying contract this task specifies — one aggregate error naming every offending
  setting, not only the first — is unaffected and still fully proven; only the literal
  substring checked for the type-conversion half changed to match verified reality.
- **Files modified:** `internal/config/config_test.go`.
- **Commit:** `bb1af10`.

## Known Stubs

None — every field added is a real, typed `Config` field with a locked default per D-07, not a
commented-out placeholder. `DiscordWebhookURL`, `PollInterval`, `MusicBrainzUserAgent`,
`MusicBrainzRateLimitPerSec`, and `DeezerRateLimitPer5s` are unused by any consumer yet
(Phase 3/5's job), which is the explicitly intended and documented state per D-06/D-07 — not an
unresolved stub.

## Threat Flags

None beyond what the plan's own `<threat_model>` already covers. `TestDotEnvIsNotTracked`
directly exercises T-01-08 (`.env` accidental commit); `TestLoad_AggregatesAllMissing` and
`TestLoad_MissingRequired`/`TestLoad_EmptyRequired` exercise T-01-09 (fail-fast error message
never echoes a supplied value — assertions check variable names only, never a value) and
T-01-11 (malformed typed settings terminate boot rather than silently zero-valuing). No new
network endpoints, auth paths, or schema changes were introduced.

## Self-Check: PASSED

- FOUND: internal/config/config.go
- FOUND: internal/config/config_test.go
- FOUND: .env.example
- FOUND: commit 5946d56 (test(01-03): add failing tests for full Config field surface)
- FOUND: commit 242df5a (feat(01-03): complete Config struct and .env.example to the full app surface)
- FOUND: commit bb1af10 (test(01-03): prove fail-fast rejection and struct-to-example parity)
