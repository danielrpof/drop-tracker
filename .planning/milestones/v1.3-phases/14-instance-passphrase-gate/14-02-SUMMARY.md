---
phase: 14-instance-passphrase-gate
plan: 02
subsystem: auth
tags: [rate-limiting, brute-force, throttle, slog-audit, discord-alert, go]

requires:
  - phase: 14-instance-passphrase-gate
    plan: 01
    provides: authgate.Manager + HandleLogin/HandleLogout, authgate.Alerter seam, httpserver.WithAuthGate 3-arg option, middleware.RealIP conditional wiring
provides:
  - authgate.loginThrottle — per-IP token-bucket map with idle-eviction sweeper (GATE-04)
  - authgate fixed jittered login delay on the two comparison paths (204 / wrong-passphrase 401), undelayed 429
  - authgate loginSlots semaphore — bounded login concurrency, immediate 503 shed (D-12)
  - authgate.globalCounter — process-wide failed-comparison counter, one alert per cooldown (D-12)
  - authgate.discordAlerter + authgate.SelectAlerter — Discord-backed brute-force alert over the existing webhook sink
  - authgate.Manager.Close + httpserver.Server.Close — stop the limiter-map sweeper goroutine
  - structured slog auth audit lines (success / failure / throttle / logout) carrying source_ip (D-13)
affects: [14-03 frontend 401 handling, 14-04 CSRF + weak-passphrase WARN, 17 VPS deploy + TRUST_PROXY_HEADERS]

actuals:
  tokens: 12200
  tasks: 3
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Per-client rate limiting: golang.org/x/time/rate bucket-per-IP in a mutex-guarded map; only the map + ticker-driven idle eviction is ours (Pattern 4)"
    - "Undelayed reject / delayed compare: the fixed jittered delay wraps ONLY the two passphrase-comparison paths; 429/503/400 return immediately so a rejected request never parks a goroutine"
    - "Non-blocking semaphore shed: buffered chan struct{} sized at construction from a test-shrinkable var; select-default -> immediate 503, no token, no counter touch"
    - "Alert-only global counter: no endpoint cooldown lock (research A3) — per-IP limiter + delay carry the throughput bound, a global lock would lock the operator out mid-attack"
    - "Fire-and-forget alert dispatch: Alert runs on its own goroutine under context.WithTimeout; a slow/failing webhook never stalls or changes the login response"
    - "Secret-never-logged: alert path logs the outcome only, never the send error (webhook path is the token); enforced by an acceptance grep + a buffer-scan test (mirrors internal/db/migrate.go redactError discipline)"

key-files:
  created:
    - internal/authgate/login_test.go
    - internal/authgate/alerter_test.go
  modified:
    - internal/authgate/login.go
    - internal/authgate/gate.go
    - internal/authgate/alerter.go
    - internal/authgate/export_test.go
    - internal/authgate/gate_test.go
    - internal/httpserver/server.go
    - internal/httpserver/server_test.go
    - cmd/server/main.go

key-decisions:
  - "Chosen tunables: loginRate=rate.Every(12s) (~5/min sustained), loginBurst=5, loginDelayMin=250ms, loginDelayJitter=750ms (250ms–1s total), maxConcurrentLogins=32, limiterSweepInterval=10m, limiterIdleTTL=15m, globalWindow=5m, globalThreshold=20, alertCooldown=15m"
  - "gate.go and internal/httpserver/server.go were modified though the plan frontmatter files_modified omitted them — the plan's own <action> text mandates Manager/NewManager wiring (gate.go) and a cmd/server/main.go defer srv.Close() which needs httpserver.Server.Close() (server.go). Documented as Deviation 1."
  - "#nosec G404 on the jitter: math/rand/v2 is deliberate anti-hammer pacing, not a security primitive (14-RESEARCH.md §Standard Stack); crypto/rand stays reserved for the session nonce"
  - "Limiter-map sweeper is a real ticker-driven goroutine started in NewManager and stopped by Manager.Close(); Close is wired into main.go (defer) and both test helpers (t.Cleanup) so no goroutine outlives its server"
  - "loginSleep is a package-var seam over time.Sleep so the concurrency-shed test can block two logins in-flight without real wall-clock waits"

requirements-completed: []

coverage:
  - id: D-14-02-1
    description: "Per-IP login throttle (GATE-04): attempts 1–5 from one address answered normally, attempt 6 returns 429 {\"error\":\"too many attempts\"}, a second address in the same window is unaffected, and service is restored after the bucket refills"
    requirement: "GATE-04"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginThrottle_BoundaryAndSecondAddressUnaffected"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginThrottle_RefillRestoresService"
        status: pass
    human_judgment: false
  - id: D-14-02-2
    description: "Fixed jittered delay on exactly the two comparison paths (204 success, wrong-passphrase 401), and NOT on the 429 throttle rejection (D-12)"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginDelay_OnComparisonPathButNotOn429"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginDelay_AppliedOnSuccessPath"
        status: pass
      - kind: manual
        ref: "grep: loginDelay() invoked on exactly 2 call sites (login.go:251 wrong-passphrase 401, login.go:266 success); math/rand/v2 imported once"
        status: pass
    human_judgment: false
  - id: D-14-02-3
    description: "Login-concurrency semaphore: a request that would exceed maxConcurrentLogins sheds with an immediate 503 {\"error\":\"server busy\"}, no delay, no limiter token, no counter touch (D-12, T-14-02-11)"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginConcurrency_ShedsExcessWith503"
        status: pass
    human_judgment: false
  - id: D-14-02-4
    description: "Limiter map cannot grow without bound: entries idle beyond limiterIdleTTL are evicted by the sweeper (GATE-04)"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLimiterSweep_EvictsIdleEntries"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginThrottle_ParallelSameAddressConsistent (no panic / no map corruption under concurrent access)"
        status: pass
    human_judgment: false
    rationale: "The parallel test passes; -race is run in CI, not on this Windows dev machine (documented STATE.md ThreadSanitizer limitation)."
  - id: D-14-02-5
    description: "Global failed-attempt counter fires exactly one Discord alert per cooldown once globalThreshold is crossed inside globalWindow; a stale window restarts the count; throttled and malformed requests never feed it (D-12, T-14-02-10)"
    requirement: "GATE-04"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestGlobalCounter_ThresholdCooldownAndWindowReset"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestGlobalCounter_StaleWindowStartsFreshCount"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestGlobalAlert_FiresOncePerCooldown"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestGlobalAlert_ThrottledRequestsDoNotIncrementCounter"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestGlobalAlert_FailingAlerterDoesNotChangeLoginResponse"
        status: pass
    human_judgment: false
  - id: D-14-02-6
    description: "SelectAlerter mirrors notifier.Select: empty DISCORD_WEBHOOK_URL -> one Info line + inert no-op Alerter; set URL -> Discord-backed Alerter. The alert path never carries the webhook send error into a log call (T-14-02-02)"
    requirement: "GATE-04"
    verification:
      - kind: unit
        ref: "internal/authgate/alerter_test.go#TestSelectAlerter_DisabledLogsOneInfoLineAndIsInert"
        status: pass
      - kind: unit
        ref: "internal/authgate/alerter_test.go#TestSelectAlerter_EnabledReturnsDiscordBacked"
        status: pass
      - kind: manual
        ref: "grep gate: `grep -v '^\\s*//' internal/authgate/alerter.go | grep -cE '(Warn|Error|Info)\\(.*(err|%w|%v)'` = 0; `grep -c 'SelectAlerter' cmd/server/main.go` = 1"
        status: pass
    human_judgment: false
  - id: D-14-02-7
    description: "One structured slog audit line per response path (success / failure / throttle / logout), carrying the resolved source_ip, at Info for success+logout and Warn for failure+throttle; the passphrase reaches no log line on any path (D-13)"
    requirement: "GATE-04"
    verification:
      - kind: unit
        ref: "internal/authgate/login_test.go#TestAudit_NoPassphraseInLogsAcrossOutcomes"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestAudit_OneLinePerOutcomeWithSourceIP"
        status: pass
      - kind: manual
        ref: "git diff shows the httplog.Options block in internal/httpserver/server.go was not touched by this plan (grep -c = 0)"
        status: pass
    human_judgment: false

duration: 45min
completed: 2026-08-29
status: complete
---

# Phase 14 Plan 02: Instance Passphrase Gate — Brute-force Defense & Auditability Summary

**Per-IP `golang.org/x/time/rate` login throttle (burst 5, `rate.Every(12s)`, `429` on the sixth attempt), a fixed 250ms–1s jittered delay wrapping only the two passphrase-comparison paths, a `maxConcurrentLogins=32` semaphore that sheds excess with an undelayed `503`, an alert-only process-wide failed-attempt counter (20 within 5m → one Discord alert per 15m cooldown) posting through the existing webhook sink, and one structured `slog` audit line per auth outcome carrying `source_ip` — the passphrase reaches no log line on any path.**

## Chosen tunable values (recorded verbatim per plan `<output>`)

| Var | Value | Meaning |
|-----|-------|---------|
| `loginRate` | `rate.Every(12 * time.Second)` | ~5 attempts/min sustained per client IP after the burst |
| `loginBurst` | `5` | 5 immediate attempts per IP, then throttled |
| `loginDelayMin` | `250 * time.Millisecond` | floor delay on the 204 / wrong-passphrase 401 paths |
| `loginDelayJitter` | `750 * time.Millisecond` | added jitter → total 250ms–1s |
| `maxConcurrentLogins` | `32` | login-handler concurrency ceiling; excess → immediate 503 |
| `limiterSweepInterval` | `10 * time.Minute` | how often the per-IP map is swept |
| `limiterIdleTTL` | `15 * time.Minute` | evict a limiter entry not seen for this long |
| `globalWindow` | `5 * time.Minute` | rolling window for the process-wide failed-attempt count |
| `globalThreshold` | `20` | failed comparisons within the window that trigger an alert |
| `alertCooldown` | `15 * time.Minute` | minimum spacing between brute-force alerts |

All are single greppable literals in `internal/authgate/login.go`, package `var`s only so `export_test.go` can shrink them — never runtime-configurable (D-07). An operator locked out during their own testing retunes one line and rebuilds.

## Performance

- **Duration:** ~45 min
- **Tasks:** 3 (all TDD-style; RED/GREEN combined per commit for Task 1 as the three tasks share `login.go`, mirroring 14-01's precedent)
- **Files:** 2 created, 8 modified

## Accomplishments

- **Per-IP throttle** — `loginThrottle`: a `sync.Mutex`-guarded `map[string]*ipLimiter`, `getLimiter(ip, now)` upserting a `rate.NewLimiter(loginRate, loginBurst)` and refreshing `lastSeen`, `sweep(now)` deleting entries idle past `limiterIdleTTL`. A rejected request gets an **immediate, undelayed 429** `{"error":"too many attempts"}` and never touches the comparison or the global counter.
- **Fixed jittered delay** — `loginDelay()` sleeps `loginDelayMin` + a `math/rand/v2` jitter (`#nosec G404` — anti-hammer pacing, not a security primitive). Invoked on **exactly two call sites**: the 204 success and the wrong-passphrase 401. Never on 429 / 503 / 400. `loginSleep` is a `time.Sleep` seam for the concurrency test.
- **Concurrency semaphore** — `Manager.loginSlots chan struct{}` sized `make(chan struct{}, maxConcurrentLogins)` in `NewManager` (reads the var at construction). `HandleLogin` does a non-blocking `select` acquire first; a would-block request gets an immediate `503 {"error":"server busy"}` with no delay, no limiter token, no counter touch.
- **Global counter + Discord alert** — `globalCounter.recordFailure(now) bool` resets on a stale window, increments, and returns `true` only on the threshold-crossing transition outside the cooldown (restamping `alertedAt`). Fed **only** by a genuine comparison mismatch. On `true`, `dispatchAlert()` posts on its own goroutine under `context.WithTimeout(10s)` so a slow/failing webhook never stalls the response; a delivery failure logs the outcome only — never the send error.
- **`discordAlerter` + `SelectAlerter`** — mirrors `notifier.Select`: empty `DISCORD_WEBHOOK_URL` → one Info line + `noopAlerter{}`; set URL → `discordAlerter` over `discord.NewClient(url, nil)`. The embed carries only a count and a window — no submitted value, prefix, suffix or length. `cmd/server/main.go` now passes `authgate.SelectAlerter(cfg.DiscordWebhookURL, logger)` as `WithAuthGate`'s third argument.
- **Audit lines (D-13)** — exactly one structured `slog` line per response path in `HandleLogin` / `HandleLogout`: `"authgate login" | "authgate logout"` + `outcome` (`success` / `failure` / `throttled` / `logout`) + `source_ip` (the same resolved client address the limiter keys on). Info for success + logout, Warn for failure + throttle. No attribute carries the passphrase, its digest, the derived key or the cookie.
- **Lifecycle** — `Manager.Close()` (idempotent, `sync.Once`) stops the ticker-driven sweeper goroutine; `httpserver.Server.Close()` delegates. `cmd/server/main.go` defers it; both `newGatedServer` test helpers wire it into `t.Cleanup`.
- **Test suite** — 16 new tests across `login_test.go` (whitebox `package authgate`) and `alerter_test.go`: throttle boundary + refill, delay on/off the two paths, 503 shed with a blocked `loginSleep`, sweeper eviction, 40-goroutine parallel-same-address consistency, counter threshold/cooldown/window-reset, one-alert-per-cooldown across a 10-failure burst, 429s excluded from the counter, failing-Alerter leaves the 401 unchanged, both `SelectAlerter` branches, and the two D-13 audit guards.

## Task Commits

1. **Task 1 (throttle + delay + concurrency shed + audit lines + counter hook):** `9fac2ab` — `feat(14-02): per-IP login throttle, fixed delay, concurrency shed (GATE-04)`
2. **Task 2 (Discord alerter + SelectAlerter wiring):** `9fde033` — `feat(14-02): Discord brute-force alerter + SelectAlerter wiring (D-12)`
3. **Task 3 (D-13 audit-line enforcing tests):** `0c113c9` — `test(14-02): structured auth audit lines + secret-never-logged guard (D-13)`
4. **Plan metadata:** _(this commit)_

_TDD note: `internal/authgate/login.go` is edited by all three tasks. The throttle rewrite of `HandleLogin` in commit 1 necessarily contains the audit-line calls (Task 3) and the `recordFailure` hook (Task 2) because they are the same method body — splitting them would have produced non-building intermediate states. Each commit is atomic and leaves the suite green. Task 3's dedicated commit is test-only because its production surface landed cohesively in commit 1._

## Files Created/Modified

- `internal/authgate/login.go` — `loginThrottle`/`ipLimiter`, `globalCounter`, `clientIP`, `loginDelay`/`loginSleep`, the brute-force tunable `var` block, rewritten `HandleLogin` (semaphore → per-IP 429 → body → compare → delay → counter → audit), `dispatchAlert`, `HandleLogout` now audited
- `internal/authgate/gate.go` — `Manager` gains `throttle`/`counter`/`loginSlots`/`sweepTicker`/`sweepDone`/`closeOnce`; `NewManager` builds them + starts `sweepLoop`; `Close()`
- `internal/authgate/alerter.go` — `discordAlerter` + `Alert` + `SelectAlerter` + `var _ Alerter = discordAlerter{}`
- `internal/authgate/export_test.go` — setters for all 10 tunables + the `loginSleep` seam (save-swap-restore)
- `internal/authgate/gate_test.go` — `newGatedServer` helper wires `t.Cleanup(srv.Close)`
- `internal/httpserver/server.go` — `Server.gate` field, `s.gate = gate`, `Server.Close()`
- `internal/httpserver/server_test.go` — `newGatedServer` + the RealIP-wiring test helper wire `t.Cleanup(srv.Close)`
- `cmd/server/main.go` — `WithAuthGate` third arg → `authgate.SelectAlerter(cfg.DiscordWebhookURL, logger)`; `defer srv.Close()`
- `internal/authgate/login_test.go` (NEW) — whitebox throttle / delay / semaphore / sweeper / counter / alert / audit tests + shared helpers
- `internal/authgate/alerter_test.go` (NEW) — whitebox `SelectAlerter` branch tests

## Decisions Made

- **Tunable values** — recorded in the table above; engineering judgement per 14-RESEARCH.md §Pattern 4 recommended constants (assumption A2), which 14-CONTEXT.md leaves to the planner.
- **`gate.go` + `internal/httpserver/server.go` modified despite not being in the plan frontmatter `files_modified`** — the plan's `<action>` text explicitly requires `Manager`/`NewManager` wiring and a `cmd/server/main.go` `defer srv.Close()`; `Manager` lives in `gate.go`, and `main.go` has no handle to the internally-constructed `Manager` so `httpserver.Server.Close()` is the delegation point. See Deviation 1.
- **`#nosec G404`** on the jitter line — `math/rand/v2` is the documented choice for non-security pacing (14-RESEARCH.md); the annotation keeps the CI `gosec` gate green with a stated rationale.
- **Real ticker-driven sweeper + `Close()`** (not opportunistic in-`getLimiter` sweeping) — follows the plan's `<action>` verbatim; the goroutine is cleanly stopped everywhere it is started.
- **Alert message** — `fmt.Sprintf` of `globalThreshold` + `globalWindow` only ("...the failed-login threshold (20 within 5m0s) was crossed"). No instance name (none is configured); no submitted value, prefix, suffix or length (T-14-02-04).

## Deviations from Plan

### 1. [Rule 3 — Blocking] `internal/authgate/gate.go` and `internal/httpserver/server.go` modified (not in frontmatter `files_modified`)

- **Found during:** Task 1
- **Issue:** The plan's `files_modified` frontmatter lists only `login.go`, `login_test.go`, `alerter.go`, `alerter_test.go`, `export_test.go`, `cmd/server/main.go`. But the Task 1 `<action>` says *"Give `Manager` (or the `loginThrottle`) a `loginSlots chan struct{}` field, made in `NewManager`"*, *"Construct the throttle in `NewManager` and start the sweeper there"*, and *"Give `Manager` a `Close()` method … call it from a `defer` in `cmd/server/main.go`"*. `Manager` and `NewManager` are defined in `gate.go`. `cmd/server/main.go` receives a `*httpserver.Server`, not the `Manager` (built privately inside `httpserver.New`), so `httpserver.Server.Close()` is required as the delegation point.
- **Fix:** Added the brute-force-defense fields + `sweepLoop` + `Close()` to `gate.go`'s `Manager`/`NewManager`; added `Server.gate` + `Server.Close()` to `server.go`; wired `defer srv.Close()` in `main.go` and `t.Cleanup(srv.Close)` in both `newGatedServer` helpers.
- **Verification:** `go build ./...`, `go vet ./...`, `go test ./... -short` all exit 0; the plan's `<artifacts_this_phase_produces>` narrative already anticipated `NewManager` wiring, so this is a frontmatter omission, not a scope change.
- **Committed in:** `9fac2ab` (gate.go, server.go), `9fde033` (main.go).

### 2. [Process] RED/GREEN combined for Task 1

`internal/authgate/login.go`'s `HandleLogin` is rewritten as one cohesive method by all three tasks. Splitting the throttle, the counter hook and the audit lines into separate building commits was not practical. Each commit is atomic and green. Not a scope change — every task's behavior and acceptance criteria are implemented and pass.

**Total deviations:** 1 blocking (frontmatter omission, resolved), 1 process note.
**Impact on plan:** None on the delivered surface. Every `<behavior>`, `<acceptance_criteria>` and `<success_criteria>` item is implemented and test-proven.

## Authentication Gates

None — this plan touches no external service requiring credentials. The Discord webhook path is inert unless `DISCORD_WEBHOOK_URL` is set (and is never exercised live by the test suite).

## Issues Encountered

- **`-race` unavailable on this Windows dev machine** (documented STATE.md ThreadSanitizer limitation). The concurrency tests (`TestLoginThrottle_ParallelSameAddressConsistent`, `TestLoginConcurrency_ShedsExcessWith503`) pass under plain `go test`; CI runs the same suite under `-race`. Consistent with 14-01's handling of `TestGate_Concurrency`.

## Plan Verification Results

| Check | Result |
|-------|--------|
| `go build ./... && go vet ./...` | exit 0 |
| `go test ./internal/authgate/ -count=1` | exit 0, ~6.5s (well under 60s with shrunk timings) |
| `go test ./... -short -count=1` | exit 0 (all packages ok), ~13s |
| `go test ./internal/authgate/ ./internal/httpserver/ ./cmd/... -count=1` with `INSTANCE_PASSPHRASE` absent | exit 0 |
| `cd web && pnpm test` | not run — `git diff --name-only 705bda5..HEAD` shows zero `web/` paths; unchanged by construction |
| Task 1 acceptance greps | `math/rand/v2` ×1, `loginSlots` ×6 (≥3), `sync.Mutex` in login.go ×2, `loginDelay()` on 2 call sites, `maxConcurrentLogins` setter present |
| Task 2 acceptance greps | `SelectAlerter` in main.go ×1, `discord.NewClient` in alerter.go ×1, alert-path log-with-error ×0 |
| Task 3 acceptance | `httplog.Options` untouched in the plan diff (×0) |

## Threat Flags

None. Every file changed operates inside the trust boundaries already enumerated in `14-02-PLAN.md`'s `<threat_model>`. No new network endpoint, auth path, file-access pattern or schema change was introduced — the throttle, counter and audit lines all sit on the existing `POST /session` surface, and the alert reuses the already-integrated Discord webhook sink (COVERAGE.md).

## Next Phase Readiness

- **14-03 (Wave 2, frontend, GATE-05):** ready. The server contract is unchanged from 14-01 for the frontend's purposes — `401 {"error":"unauthenticated"}`, `POST /session {passphrase}` → 204 + `Set-Cookie`, `DELETE /session` → 204 — plus the new `429 {"error":"too many attempts"}` and `503 {"error":"server busy"}` shapes the SPA's `apiFetch` should tolerate gracefully (both are `ApiError` with an `error` field, same as the others).
- **14-04 (Wave 3, CSRF + weak-passphrase WARN):** unblocked. `Manager.RequireCSRFHeader` and `authgate/weak.go` remain unbuilt (explicitly 14-04's). `HandleLogin`'s clean insertion points are intact — the CSRF header check would sit just after the semaphore acquire / before the per-IP limiter.
- **Phase 17 (VPS deploy):** the per-IP limiter and audit log key on `clientIP(r)` (host of `r.RemoteAddr`), which `middleware.RealIP` rewrites from the proxy headers only when `TRUST_PROXY_HEADERS=true` (D-14, wired in 14-01). The Phase 17 runbook must set that flag together with the unpublished-container-port topology.
- **Requirements:** GATE-04 stays Pending — it is also declared by 14-03/14-04 and the shared-ID gate holds it until the last declaring plan produces its SUMMARY.

## Self-Check: PASSED

- All 2 created files + 8 modified files present on disk (verified with `[ -f ]` / `git show`).
- All 3 task commits present in `git log` (`9fac2ab`, `9fde033`, `0c113c9`).
- Plan `<verification>` re-run: `go build`/`go vet` exit 0; `go test ./internal/authgate/` exit 0 in ~6.5s; `go test ./... -short` exit 0.
- Every task's `<acceptance_criteria>` re-checked green (grep gates + `-run` filters logged in the table above).
- No new stubs, skipped tests, or unrun verifies — nothing appended to `.planning/WINDOWS.md`.

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-08-29*
