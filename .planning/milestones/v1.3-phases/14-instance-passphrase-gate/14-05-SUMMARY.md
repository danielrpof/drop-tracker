---
phase: 14-instance-passphrase-gate
plan: 05
subsystem: auth
tags: [docker-compose, env-vars, slog, boot-logging, gap-closure, go, config-test]

# Dependency graph
requires:
  - phase: 14-instance-passphrase-gate
    plan: 01
    provides: "config.InstancePassphrase; httpserver.WithAuthGate; cmd/server boot order; internal/config repo-root file tests (repoRoot helper, envExampleKeys idiom)"
  - phase: 14-instance-passphrase-gate
    plan: 04
    provides: "cmd/server/main.go D-11 weak-passphrase boot WARN block (the anchor the new gate-status line is placed immediately after)"
provides:
  - "docker-compose.yml app.environment: forwards INSTANCE_PASSPHRASE and TRUST_PROXY_HEADERS as ${VAR:-default} interpolations, with env_file: .env still the primary channel"
  - "cmd/server/main.go logInstanceGateStatus(logger, passphrase) — one Info record per boot reporting the instance gate as active or inert, carrying no passphrase material or length"
  - "internal/config/config_test.go TestDockerComposeWiresGateEnvVars — CI regression guard: fails (naming the missing key) if either gate env entry is dropped from docker-compose.yml"
  - "cmd/server/main_test.go TestLogInstanceGateStatus — pins one Info record per branch and that the active branch leaks neither the passphrase nor its decimal rune count"
  - "14-UAT.md Test 1 precondition block — names the repo-root .env channel and the two channels that silently do nothing (.env.example, pre-14-05 host-shell export)"
  - "G-14-1 closed — operator reconciled the live repo-root .env; boot log reports the gate active; the browser shows the passphrase form"
affects: [17-vps-deploy, verify-work]

actuals:
  tokens: 3700
  tasks: 4
  commits: 6

tech-stack:
  added: []
  patterns:
    - "Compose env pass-through as documented interpolation: ${VAR:-default} entries in app.environment: with a load-bearing comment block recording precedence (environment: outranks env_file:), the one empty-string-export shadow case, and why NOTIFY_MAX_RELEASE_AGE_DAYS is deliberately excluded"
    - "Observability for an intentionally-inert security control: one Info boot line naming the state and, on the inert branch, the remediation channel — derived from the same in-memory config value the gate constructor receives, never a second os.Getenv"
    - "docker-compose.yml regression coverage via a bufio.Scanner line walk in internal/config/config_test.go (the envExampleKeys / TestDotEnvIsNotTracked idiom), never a YAML parser — the plan ships zero new module dependencies"
    - "A no-secrets-in-logs test that decodes the JSON record, deletes the time key, then asserts the passphrase value and its decimal rune count are absent from the remaining keys/values — avoids the flaky raw-buffer scan where a 2-digit length collides with the timestamp"

key-files:
  created: []
  modified:
    - docker-compose.yml
    - internal/config/config_test.go
    - cmd/server/main.go
    - cmd/server/main_test.go
    - .planning/phases/14-instance-passphrase-gate/14-UAT.md

key-decisions:
  - "logInstanceGateStatus takes the passphrase as a parameter and the single call site passes cfg.InstancePassphrase — the exact value that flows into httpserver.WithAuthGate — so the log line can never report active while the gate is inert (T-14G-06)"
  - "The gate-status line is placed in run() immediately after the D-11 weak-passphrase WARN and before db.RunMigrations, so gate status is visible even when a later boot step fails and all gate-related boot logging stays adjacent"
  - "TRUST_PROXY_HEADERS interpolation carries a false default (${TRUST_PROXY_HEADERS:-false}) and the test asserts the ':-false}' literal — D-14's fail-safe direction cannot be silently dropped by a future edit"
  - "NOTIFY_MAX_RELEASE_AGE_DAYS deliberately NOT added to docker-compose.yml — its Config field has envDefault 7 that already covers absence; an interpolation line would duplicate that default where it could drift"
  - "docker compose config is banned as a verification step everywhere in this plan — it inlines env_file contents and prints the real DISCORD_WEBHOOK_URL and INSTANCE_PASSPHRASE to stdout; a warning is written into both docker-compose.yml and the 14-UAT.md precondition, and a value-free GATE_ENV container check is the replacement (T-14G-02)"

patterns-established:
  - "Gap-closure plan pattern: three surgical auto tasks (compose wiring + regression test, boot log + test, UAT precondition) plus one blocking-human checkpoint for the one step agent tooling cannot perform (.env is denied Read/Write in this sandbox)"

requirements-completed: [GATE-01, GATE-07]

coverage:
  - id: D1
    description: "A host-shell INSTANCE_PASSPHRASE / TRUST_PROXY_HEADERS now reaches the app container because docker-compose.yml app.environment: carries ${INSTANCE_PASSPHRASE:-} and ${TRUST_PROXY_HEADERS:-false}; env_file: .env stays the primary channel and is untouched; the postgres service is untouched"
    requirement: "GATE-01"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestDockerComposeWiresGateEnvVars"
        status: pass
    human_judgment: false
  - id: D2
    description: "Removing either gate env entry from docker-compose.yml fails an automated test that names the missing key (CI regression instead of a UAT)"
    requirement: "GATE-01"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestDockerComposeWiresGateEnvVars (asserts non-comment INSTANCE_PASSPHRASE: line referencing ${INSTANCE_PASSPHRASE...}, and TRUST_PROXY_HEADERS: line with ${TRUST_PROXY_HEADERS...} and a :-false} default)"
        status: pass
    human_judgment: false
  - id: D3
    description: "Every boot emits exactly one Info record stating whether the instance gate is active or inert; the inert record names the repo-root .env KEY=VALUE remediation channel; the active record contains neither the passphrase string nor its decimal rune count; never zero, never two records"
    requirement: "GATE-01"
    verification:
      - kind: unit
        ref: "cmd/server/main_test.go#TestLogInstanceGateStatus/active_passphrase (one INFO record, mentions 'active', passphrase and rune count absent after time key deleted)"
        status: pass
      - kind: unit
        ref: "cmd/server/main_test.go#TestLogInstanceGateStatus/empty_passphrase (one INFO record, mentions 'inert' and '.env')"
        status: pass
    human_judgment: false
  - id: D4
    description: "With no passphrase configured, the GATE-07 inert path is byte-for-byte unchanged apart from the added inert log line — the full go test ./... suite passes with INSTANCE_PASSPHRASE absent"
    requirement: "GATE-07"
    verification:
      - kind: integration
        ref: "go test ./... -count=1 (TEST_DATABASE_URL set, INSTANCE_PASSPHRASE absent) — every package ok"
        status: pass
    human_judgment: false
  - id: D5
    description: "14-UAT.md Test 1 carries a precondition block naming the working .env channel and both non-working channels (.env.example, pre-14-05 host-shell export), plus the boot-log check, a value-free GATE_ENV container fallback, and the docker-compose-config warning; Tests 2 and 4 note their unblock condition"
    requirement: "GATE-01"
    verification:
      - kind: manual_procedural
        ref: "grep -c '^precondition:' 14-UAT.md = 1; grep -c 'GATE_ENV' 14-UAT.md >= 1"
        status: pass
    human_judgment: false
  - id: D6
    description: "The operator's live repo-root .env carries INSTANCE_PASSPHRASE / TRUST_PROXY_HEADERS / NOTIFY_MAX_RELEASE_AGE_DAYS; docker compose up --build boots with the gate-status line reporting ACTIVE; the browser shows the passphrase form instead of the watchlist — G-14-1 closed"
    requirement: "GATE-01"
    verification:
      - kind: manual_procedural
        ref: "Task 4 checkpoint:human-action — operator reconciled .env, restarted the stack, confirmed the boot log reports the instance gate active and the browser shows the passphrase form. Operator response: 'all good now'."
        status: pass
    human_judgment: true
    rationale: "Agent tooling in this sandbox is denied Read/Write on .env* (Phase 11.1-04 / WINDOWS.md limitation); only the operator can edit the live repo-root .env and observe the running stack in a browser. This is the step that actually closes the gap and it is not automatable."

# Metrics
duration: ~20min
completed: 2026-09-01
status: complete
---

# Phase 14 Plan 05: Passphrase Gate Config Reachability (G-14-1 Closure) Summary

**`docker-compose.yml` now forwards `INSTANCE_PASSPHRASE` / `TRUST_PROXY_HEADERS` as `${VAR:-default}` interpolations (with `env_file: .env` still primary), `cmd/server` emits one secret-free Info line per boot stating whether the instance gate is active or inert, `TestDockerComposeWiresGateEnvVars` turns a dropped gate env entry into a CI failure instead of a UAT surprise, and the operator reconciled the live `.env` — the passphrase gate that silently did nothing through a whole Phase 14 UAT round is now engaged and observable.**

## Performance

- **Duration:** ~20 min automated work (Tasks 1-3 + continuation verification); Task 4 was an operator checkpoint
- **Started:** 2026-08-31T19:21:30-05:00 (first task commit)
- **Completed:** 2026-09-01T02:16:55Z (continuation verification + plan close)
- **Tasks:** 4 (Tasks 1 & 2 TDD; Task 3 doc; Task 4 checkpoint:human-action)
- **Files modified:** 5

## Continuation note

This plan was executed across two agent sessions. A prior agent completed Tasks 1-3 (commits `fabf105`, `215d5fb`, `2a3cff7`, `cd966aa`, `2599980`) and stopped at the Task 4 `checkpoint:human-action` gate. The operator then reconciled the repo-root `.env`, ran `docker compose up --build`, confirmed the boot log reports the instance gate **ACTIVE**, and confirmed the browser shows the passphrase form ("all good now"). This continuation session verified the prior work is intact, re-ran the plan's full `<verification>` section, and closed the plan.

## Accomplishments

- **`docker-compose.yml` compose-half of the fix** — the `app` service `environment:` mapping now carries `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` and `TRUST_PROXY_HEADERS: ${TRUST_PROXY_HEADERS:-false}`, added after `DATABASE_URL`. Before this, a host-shell `export INSTANCE_PASSPHRASE=...` reached nothing — Compose forwards no host variable it is not told about, and this file referenced none. A ~20-line comment block above the two entries records: (a) why they exist, (b) that `${VAR:-}` resolves host-shell → project `.env` and that `env_file: .env` IS that same file so an operator `.env` value still arrives, (c) that `environment:` outranks `env_file:` so an empty-string shell export is the one shadow case (made visible by the boot log line), (d) why `NOTIFY_MAX_RELEASE_AGE_DAYS` is deliberately excluded, and a `docker compose config` secret-leak warning. `env_file: .env` and the `postgres` service are untouched.
- **`cmd/server/main.go` observability-half of the fix** — new unexported `logInstanceGateStatus(logger *slog.Logger, passphrase string)`: empty passphrase → one Info record reporting the gate **inert** with a `hint` attribute naming the repo-root `.env` `KEY=VALUE` channel; non-empty → one Info record reporting the gate **active**. Never logs the passphrase, a substring, its length, or a hash — safe to leave on in production. Called from `run()` immediately after the D-11 weak-passphrase WARN and before `db.RunMigrations`, passing `cfg.InstancePassphrase` (the same value `httpserver.WithAuthGate` receives — no second environment read).
- **`internal/config/config_test.go` regression guard** — `TestDockerComposeWiresGateEnvVars` scans `docker-compose.yml` with a `bufio.Scanner` line walk (skipping blank and `#` lines), asserts a non-comment `INSTANCE_PASSPHRASE:` line referencing `${INSTANCE_PASSPHRASE...}`, and a `TRUST_PROXY_HEADERS:` line referencing `${TRUST_PROXY_HEADERS...}` **and** carrying a `:-false}` default. Fails naming the missing key. No YAML parser — zero new module dependencies.
- **`cmd/server/main_test.go` no-secrets test** — `TestLogInstanceGateStatus` (two subtests). Builds a capturing JSON logger via `logging.NewWithWriter` into a `bytes.Buffer`, counts non-empty lines to prove exactly one record per branch, decodes the record, deletes the `time` key, and asserts the passphrase fixture (`correct-horse-battery-staple-9times`) and its decimal rune count are absent from the remaining keys/values. The active branch asserts `"active"`; the inert branch asserts `"inert"` and `".env"`.
- **`14-UAT.md` Test 1 precondition** — a YAML block-scalar `precondition:` inserted after the Test 1 heading: set `INSTANCE_PASSPHRASE` as a `KEY=VALUE` line in the repo-root `.env`; editing `.env.example` does nothing (Compose never reads it — the G-14-1 mistake); a host-shell export now also works but only because 14-05 added the interpolation entry; confirm via the boot log line; value-free `docker compose run --rm --entrypoint sh app -c '...'` GATE_ENV fallback; do NOT use `docker compose config`. Tests 2 and 4 get a one-line unblock note. No `result`/`status`/`severity`/`summary` field and no Gaps entry was touched.
- **G-14-1 closed** — operator reconciled the live `.env`, restarted the stack, boot log reports the gate active, browser shows the passphrase form.

## Task Commits

1. **Task 1 (RED): failing docker-compose gate env wiring regression test** — `fabf105` (test)
2. **Task 1 (GREEN): forward gate env vars through docker-compose app service** — `215d5fb` (feat)
3. **Task 2 (RED): failing test for instance gate boot status log line** — `2a3cff7` (test)
4. **Task 2 (GREEN): emit one secret-free instance gate status line at boot** — `cd966aa` (feat)
5. **Task 3: give 14-UAT Test 1 an explicit configuration precondition** — `2599980` (docs)
6. **Task 4: checkpoint:human-action** — operator reconciled the repo-root `.env` (not agent-visible), restarted the stack, confirmed the gate reports active and the passphrase form renders. No commit (operator-only file).

**Plan metadata:** _(next commit)_ — `docs(14-05): complete passphrase gate config reachability plan`

## Files Created/Modified

- `docker-compose.yml` — `app.environment:` now carries `INSTANCE_PASSPHRASE` / `TRUST_PROXY_HEADERS` interpolations + precedence/shadow-case/`compose config` warning comment block
- `internal/config/config_test.go` — NEW `TestDockerComposeWiresGateEnvVars` (bufio.Scanner line walk, no YAML dep)
- `cmd/server/main.go` — NEW `logInstanceGateStatus` helper + call site in `run()` between the D-11 WARN and `db.RunMigrations`; adds `log/slog` import
- `cmd/server/main_test.go` — NEW `TestLogInstanceGateStatus` + `nonEmptyLines` / `decodeRecord` / `recordMentions` helpers; adds `bytes`, `encoding/json`, `fmt`, `log/slog`, `strconv`, `config`, `logging` imports
- `.planning/phases/14-instance-passphrase-gate/14-UAT.md` — Test 1 `precondition:` block; Tests 2 & 4 unblock notes

## Decisions Made

- Recorded in frontmatter `key-decisions`. In brief: the log line is sourced from the same in-memory `cfg.InstancePassphrase` the gate constructor receives (never a second `os.Getenv`, per T-14G-06); the status line sits between the D-11 WARN and migrations so it survives a later boot failure; `TRUST_PROXY_HEADERS` interpolation keeps a `false` default and the test pins it; `NOTIFY_MAX_RELEASE_AGE_DAYS` is not added to compose (envDefault 7 already covers it); `docker compose config` is banned as a verification step and warned against in two places.

## Deviations from Plan

None — plan executed exactly as written. Tasks 1-3 landed as specified (RED→GREEN for the two TDD tasks, doc-only for Task 3). No auto-fix deviations (Rules 1-3) were triggered; `go build` / `go vet` / the full test suite were clean throughout. No `go.mod` / `go.sum` change.

## Auth / Checkpoint Gates

- **Task 4 — `checkpoint:human-action` (`gate="blocking-human"`), SATISFIED.** Agent tooling in this sandbox is denied Read/Write on `.env*` (documented Phase 11.1-04 / WINDOWS.md limitation), so the one step that actually closes G-14-1 — reconciling the live repo-root `.env` — could not be automated. Everything automatable was delivered in Tasks 1-3. The operator added `INSTANCE_PASSPHRASE` (fresh 24+ char random value, not the `caliber` placeholder), `TRUST_PROXY_HEADERS=false`, and `NOTIFY_MAX_RELEASE_AGE_DAYS=7` to the `.env`, ran `docker compose up --build`, confirmed the boot log line reports the instance gate **ACTIVE**, and confirmed the browser shows the passphrase form instead of the watchlist. Operator confirmation: "all good now". This is expected flow for this plan, not a deviation.

## Issues Encountered

- **`go test -race` and `make test` unusable on this Windows dev machine** (ThreadSanitizer allocation failure under memory pressure — pre-existing documented limitation, STATE.md). Substituted `go test ./... -count=1` (no `-race`) with `TEST_DATABASE_URL` pointed at the running docker-compose Postgres; every package reports `ok`. `-race` runs in CI.

## Plan Verification Results

| Check | Result |
|-------|--------|
| `go build ./...` | exit 0 (`BUILD_OK`) |
| `go vet ./...` | exit 0 (`VET_OK`) |
| `go test ./internal/config/ ./cmd/server/ -count=1` | exit 0 — `TestDockerComposeWiresGateEnvVars` PASS, `TestLogInstanceGateStatus` (active + empty) PASS |
| `go test ./... -count=1` (TEST_DATABASE_URL set, `INSTANCE_PASSPHRASE` absent) | exit 0 — every package `ok`; GATE-07 inert path unchanged |
| `git diff da5c8fb..HEAD -- go.mod go.sum` | empty — no new module dependency |
| `grep -c '^precondition:' 14-UAT.md` | 1 |
| `grep -c 'GATE_ENV' 14-UAT.md` | 2 (≥ 1) |
| `git diff da5c8fb..HEAD --stat` | 5 files, +242 lines, 0 deletions |
| No agent tool read or wrote `.env` / `.env.example` | confirmed — `.env*` never touched by this session |

## Threat Flags

None. Every change operates inside the trust boundaries `14-05-PLAN.md`'s `<threat_model>` already enumerates (operator shell/`.env` → container env; process → stdout log; repo working tree → terminal stdout). `logInstanceGateStatus` adds no endpoint or trust boundary and is a pure logger call. The compose interpolation adds no new service surface. No new package — `log/slog`, `bytes`, `encoding/json`, `fmt`, `strconv` are stdlib; `config` / `logging` were already imported by `cmd/server`.

## Next Phase Readiness

- **Phase 14 gap G-14-1 is closed.** `/gsd-verify-work` on resume should reconcile the gap via this plan's `gap_ids: [G-14-1]` frontmatter and re-run 14-UAT.md Test 1 (now unblocked), which in turn unblocks Tests 2 and 4.
- **Phase 17 (VPS deploy):** the compose interpolation comment and the `.env.example` block (from 14-04) together document that `TRUST_PROXY_HEADERS=true` belongs only with the reverse-proxy + unpublished-port topology, and that a real 24+ char random `INSTANCE_PASSPHRASE` must be set on the VPS. The `docker compose config` secret-leak warning is now in-repo for the Phase 17 runbook author.

## Self-Check

- No files were created by this plan; all 5 modified files verified present with the expected changes via `git diff da5c8fb..HEAD`.
- All 5 prior task commits present in `git log` (`fabf105`, `215d5fb`, `2a3cff7`, `cd966aa`, `2599980`).
- Plan `<verification>` re-run this session: `go build` / `go vet` exit 0; `go test ./internal/config/ ./cmd/server/` PASS incl. both new tests; full `go test ./...` exit 0 with `INSTANCE_PASSPHRASE` absent; no `go.mod` entry added; `.env*` untouched.
- Task acceptance greps re-checked: `^precondition:` ×1, `GATE_ENV` ×2 in 14-UAT.md.

## Self-Check: PASSED

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-09-01*
