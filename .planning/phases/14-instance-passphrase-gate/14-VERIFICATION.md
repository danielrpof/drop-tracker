---
phase: 14-instance-passphrase-gate
verified: 2026-08-31T00:00:00Z
status: human_needed
score: 7/7 GATE requirement IDs backed by wired + test-covered code; G-14-1 (config reachability) closed and operator-confirmed
behavior_unverified: 0
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 7/7 roadmap success criteria (code + tests); blocked at UAT by G-14-1
  gaps_closed:
    - "G-14-1 — app container booted with an empty INSTANCE_PASSPHRASE so the gate never engaged. Plan 14-05: (a) docker-compose.yml app.environment: now forwards INSTANCE_PASSPHRASE / TRUST_PROXY_HEADERS as ${VAR:-default} interpolations with a regression test (TestDockerComposeWiresGateEnvVars); (b) cmd/server/main.go emits one secret-free Info boot line reporting the gate active/inert (logInstanceGateStatus + TestLogInstanceGateStatus); (c) 14-UAT.md Test 1 gained a precondition block naming the .env channel and the two non-channels; (d) operator reconciled the live repo-root .env, restarted the stack, confirmed the boot log reports the gate ACTIVE and the browser shows the passphrase form ('all good now')."
  gaps_remaining: []
  regressions:
    - "WR-01 (code review, WARNING not blocker): the new docker-compose.yml `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` entry sits in `environment:`, which outranks `env_file: .env`. If `docker compose` is invoked from another directory or with `--env-file`/`--project-directory` pointing elsewhere, interpolation yields empty and the empty string is still emitted into the container, clobbering the value `env_file: .env` would otherwise load — silently reverting the gate to inert. The pre-14-05 `env_file`-only setup could not do this. Mitigated by the new boot-status log line (surfaces inert on every affected start) and the in-file comment block; operator's standard `docker compose up --build` from the repo root is unaffected and was confirmed working."
human_verification:
  - test: "With INSTANCE_PASSPHRASE set (gate confirmed ACTIVE via boot log), open the app in Chrome AND Firefox over http://localhost. Enter the correct passphrase; confirm the watchlist/history UI loads. Refresh the page; confirm it stays unlocked. Enter a wrong passphrase; confirm the fixed inline error message. Click Log out; confirm it returns to the passphrase form."
    expected: "Correct passphrase unlocks and survives a refresh; the bare-name dt_session cookie (option-a, no __Host- prefix) is accepted and replayed by both browsers over plain http://localhost; wrong passphrase shows the fixed message; Log out returns to the form. (Operator has already confirmed the passphrase form renders instead of the watchlist and the boot log reports the gate ACTIVE — this item covers the remaining unlock / persistence / wrong-passphrase / logout / cross-browser steps of 14-UAT.md Test 1.)"
    why_human: "Chrome vs Firefox cookie-prefix / Secure-over-localhost behaviour is not reproducible in httptest or jsdom; this is the explicit Manual-Only row in 14-VALIDATION.md. 14-UAT.md Test 1 result field still reads `issue` from the pre-14-05 round and must be re-run now that G-14-1 is closed."
  - test: "Visual conformance of PassphraseScreen to 14-UI-SPEC — glance at the running SPA: viewport-centred max-w-sm bg-card card, gap-6 rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface."
    expected: "Matches the approved UI-SPEC spacing / colour / typography pillars."
    why_human: "RTL asserts copy, structure, roles and behaviour but not rendered appearance; no Playwright/screenshot step exists. 14-03-SUMMARY marks coverage item D8 human_judgment: true, deferred to end-of-phase. 14-UAT.md Test 2 is `blocked` on the (now-closed) G-14-1 and is unblocked."
  - test: "Set DISCORD_WEBHOOK_URL and drive >20 failed logins within 5 minutes against the running gated instance; observe the Discord channel."
    expected: "Exactly one brute-force alert embed arrives, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes."
    why_human: "The live webhook send path is never exercised by the test suite (SelectAlerter branches are unit-tested with a fake); real Discord embed rendering needs a human eye. 14-UAT.md Test 4 is `blocked` on the (now-closed) G-14-1 and is unblocked."
---

# Phase 14: Instance Passphrase Gate — Verification Report (re-verification after G-14-1 closure)

**Phase Goal:** A drop-tracker instance on a public URL only exposes its watchlist data, search proxy, and event history to someone who knows the instance passphrase — while local dev, docker-compose, and the existing test suites keep working with no passphrase configured at all. This must land before anything in this milestone makes the app publicly reachable, so the instance is never briefly public-and-open.

**Verified:** 2026-08-31
**Status:** human_needed
**Re-verification:** Yes — after G-14-1 gap closure (plan 14-05)

## Re-verification Summary

The 14-01…14-04 verification round found all seven GATE requirement IDs implemented, wired, and test-covered, but routed the phase to `human_needed` for four browser/runtime UAT items. The subsequent UAT round hit a **blocker (G-14-1)**: the operator set a passphrase, ran `docker compose up --build`, and the watchlist loaded with no passphrase form — the gate was inert. Root cause was a **configuration-reachability gap, not a code defect**: the `app` container received `INSTANCE_PASSPHRASE` only through `env_file: .env`, the live repo-root `.env` had no such line, and neither a host-shell export nor editing `.env.example` reached the container.

**Plan 14-05 closed G-14-1:**

| Fix | Evidence | Status |
|-----|----------|--------|
| `docker-compose.yml` `app.environment:` forwards `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` and `TRUST_PROXY_HEADERS: ${TRUST_PROXY_HEADERS:-false}`, `env_file: .env` still primary, load-bearing comment block on precedence + shadow case + `docker compose config` secret-leak warning | `docker-compose.yml:72-73` + comment `:47-71`; `postgres` service and `env_file` untouched | ✓ VERIFIED |
| One secret-free Info boot line reporting gate active/inert | `cmd/server/main.go:73-81` `logInstanceGateStatus` — logs only `status` + (inert) `hint`; never the passphrase, a fragment, its length, or a hash. Called at `main.go:135` with `cfg.InstancePassphrase` (same value `httpserver.WithAuthGate` receives at `:238`), between the D-11 WARN and `db.RunMigrations` | ✓ VERIFIED |
| Compose-wiring regression test | `internal/config/config_test.go` `TestDockerComposeWiresGateEnvVars` — `bufio.Scanner` line walk, no YAML dep; asserts both keys present as interpolations, `TRUST_PROXY_HEADERS` pins `:-false}`. **PASS** | ✓ VERIFIED |
| Boot-log no-secrets test | `cmd/server/main_test.go` `TestLogInstanceGateStatus` (active + empty subtests) — decodes JSON record, deletes `time`, asserts passphrase fixture and its decimal rune count absent; active→`active`, empty→`inert` + `.env`. **PASS** | ✓ VERIFIED |
| 14-UAT.md Test 1 precondition | `precondition:` block present (`grep -c '^precondition:'` = 1, `GATE_ENV` ×2); names the `.env` channel, the two non-channels, the boot-log check, the value-free container fallback, and the `docker compose config` warning; Tests 2 & 4 carry unblock notes | ✓ VERIFIED |
| Operator `.env` reconcile (checkpoint:human-action) | 14-05-SUMMARY coverage D6 + continuation note: operator added `INSTANCE_PASSPHRASE` (fresh 24+ char), `TRUST_PROXY_HEADERS=false`, `NOTIFY_MAX_RELEASE_AGE_DAYS=7`; `docker compose up --build`; boot log reports gate **ACTIVE**; browser shows the passphrase form. Operator: "all good now" | ✓ CONFIRMED (operator) |

The phase-goal-blocking condition ("instance is public-and-open despite a passphrase being set") is resolved: the gate now engages, and the failure mode is now observable at boot instead of silent.

## Goal Achievement

### Observable Truths (ROADMAP success criteria; GATE-* requirement IDs cross-referenced)

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | **SC1 / GATE-01** — with a passphrase set, `/search` `/watchlist` `/events` return `401` without a valid cookie; `GET /health` (exact path), `POST/DELETE /session`, and the SPA shell + static bundle stay public | ✓ VERIFIED | `internal/httpserver/server.go:164-187` — `r.Get("/health")` exact path; `r.Post/Delete("/session")` outside the Group; data routes registered only inside `r.Group{ pr.Use(gate.Authenticate); pr.Use(gate.RequireCSRFHeader) }`; `r.NotFound(webassets.Handler)` on the root router. Exemption is structural (no `strings.HasPrefix`). Tests: `TestGate_EndToEnd_401_Login_200_Logout`, `TestGate_ExemptionBoundary_HealthIsExactPathOnly`, `TestGate_PublicSPAShell`, `TestGate_RejectedRequestIsLoggedWithRequestID` — pass. Live `401` observation → human (item 1). |
| 2 | **SC2 / GATE-05** — opening the instance shows a passphrase form (not blank/spinner/error); correct passphrase restores the watchlist + history UI, wrong does not | ⚠️ PARTIALLY CONFIRMED → human | Code: `web/app/lib/api.ts:133-136` single 401 → `authStore.markUnauthenticated()`; `web/app/root.tsx:105-107` early return `<PassphraseScreen/>` on `!authed`; `PassphraseScreen.tsx` calls `createSession` then `markAuthenticated()` only on resolve, distinct fixed copy for 401/429/network. 101/101 web tests pass. **Operator confirmed the passphrase form renders instead of the watchlist** with the gate active. Remaining: correct-passphrase unlock, wrong-passphrase rejection, cross-browser → human (item 1). |
| 3 | **SC3 / GATE-02, GATE-06** — the browser stays authenticated across a container restart/redeploy; Log out immediately returns it to the form | ✓ VERIFIED (code) → human (browser) | `internal/authgate/session.go` — `DeriveKey = SHA256(prefix‖passphrase)`, `Sign`/`Verify` pure HMAC-SHA256, no server-side store. `TestGate_MintedCookieSurvivesNewManager`, `TestSignVerify_RoundTrip` pass. `login.go:310-322` `HandleLogout` → `setSessionCookie("", -1)`, 204. `root.tsx` `LogoutButton` clears local state in `finally`. Tests: `TestGate_LogoutClearsCookie`, `TestLogoutCSRF_MissingHeaderRejected`, web `root.test.tsx` logout cases. Cross-restart + Log-out browser behaviour → human (item 1). |
| 4 | **SC4 / GATE-03, GATE-04** — cookie carries `HttpOnly`, `Secure`, `SameSite=Lax`, bounded lifetime; repeated wrong-passphrase attempts from one client are throttled with `429`; constant-time compare | ✓ VERIFIED | `gate.go` `setSessionCookie` via `http.SetCookie` (Path=/, HttpOnly, Secure, SameSiteLax, MaxAge). `login.go` `sha256` both sides → `subtle.ConstantTimeCompare`; `session.go` `hmac.Equal`; absolute-cap-before-expiry ordering. `loginThrottle` = mutex-guarded `map[string]*ipLimiter` (`x/time/rate`, burst 5, `rate.Every(12s)`), undelayed 429, non-blocking `loginSlots` semaphore (503 shed). `server.go:132` wires `middleware.RealIP` only when `gate != nil && cfg.trustProxyHeaders`. Tests: `TestGate_LoginCookieAttributes` (Max-Age=2592000), `TestVerify` (8 subcases), `TestGate_AbsoluteCapRejectsUnexpiredToken`, `TestLoginThrottle_BoundaryAndSecondAddressUnaffected`, `TestLoginDelay_OnComparisonPathButNotOn429`, `TestLoginConcurrency_ShedsExcessWith503`, `TestGatedServer_TrustProxyHeaders_RealIPWiring` — pass. Devtools inspection of live cookie → human (item 1). |
| 5 | **SC5 / GATE-07** — with no passphrase configured, every route behaves exactly as v1.2; `make test-integration`, `pnpm test`, `docker compose up` all pass with no passphrase anywhere | ✓ VERIFIED | `server.go:180-182` else-branch calls `registerDataRoutes(r, s)` directly — no `/session`, no `Authenticate`, no `RequireCSRFHeader`, no `RealIP`. `config.go:74-75` both fields optional, absent from `Load()` validation. Tests: `TestInertPath_FiveArgConstructor`, `TestInertPath_EmptyPassphraseIsIndistinguishable`, `TestGate_NoOptionsIsUngated`, `TestLogInstanceGateStatus/empty_passphrase` (reports inert, does not fail boot). Full `go test ./...` passes with `INSTANCE_PASSPHRASE` unset (every package `ok`). **14-UAT.md Test 3 (`docker compose up` with no passphrase) = PASS.** Documented deliberate exception: every response — inert included — carries `Referrer-Policy: no-referrer` (`TestReferrerPolicy_OnEveryResponse`). |

**Score:** 7/7 GATE requirement IDs (GATE-01…GATE-07) implemented, wired end-to-end, and covered by a passing named test. G-14-1 (the config-reachability gap that made the gate inert in the compose stack) is closed and operator-confirmed. Three browser/runtime UAT items remain for human sign-off.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authgate/session.go` | HMAC-SHA256 stateless session codec | ✓ VERIFIED | `DeriveKey`/`Token`/`Sign`/`Verify`, D-06 duration vars, `sessionCookieName="dt_session"`, absolute-cap-before-expiry ordering. |
| `internal/authgate/gate.go` | `Manager`, `Authenticate`, `RequireCSRFHeader`, cookie/JSON helpers | ✓ VERIFIED | Only path to `next` is `Verify(...) ok==true`. `RequireCSRFHeader` after `Authenticate`. `Close()` idempotent. |
| `internal/authgate/login.go` | `HandleLogin`/`HandleLogout`, throttle, delay, semaphore, global counter, audit lines | ✓ VERIFIED | CSRF check first, semaphore acquire, per-IP 429 undelayed, 4 KiB body cap, constant-time compare, fire-and-forget alert under timeout, one slog line per path with `source_ip` only. |
| `internal/authgate/alerter.go` | `Alerter` seam + `SelectAlerter` + Discord impl | ✓ VERIFIED | Mirrors `notifier.Select`; empty webhook → one Info line + noop; send error never logged. |
| `internal/authgate/weak.go` | `IsWeakPassphrase` + `knownDefaults` | ✓ VERIFIED | empty→not weak; `<16` runes; case-insensitive trimmed denylist incl. `caliber`; reason never embeds input. |
| `internal/config/config.go` | `InstancePassphrase`, `TrustProxyHeaders` | ✓ VERIFIED | `config.go:74-75` both optional `env:` tags, no `Load()` validation entry. |
| `internal/httpserver/server.go` | `Option`, `WithAuthGate`, protected Group, `securityResponseHeaders` | ✓ VERIFIED | Conditional wiring; `RealIP` behind `trustProxyHeaders`; `Close()` delegates to gate. |
| `cmd/server/main.go` | gate wiring + boot WARN + **boot gate-status line (14-05)** | ✓ VERIFIED | `httpserver.WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(...))` at `:238`; `IsWeakPassphrase` WARN at `:124`; `logInstanceGateStatus(logger, cfg.InstancePassphrase)` at `:135`; `defer srv.Close()`. |
| `cmd/server/main_test.go` | `TestLogInstanceGateStatus` (14-05) | ✓ VERIFIED | Two subtests pass; active branch proves passphrase + rune count absent from the record. |
| `internal/config/config_test.go` | `TestDockerComposeWiresGateEnvVars` (14-05) | ✓ VERIFIED | Passes; fails naming the missing key if either entry is dropped. |
| `docker-compose.yml` | `app.environment:` carries both gate env vars as interpolations, `env_file` primary | ✓ VERIFIED | Lines present at `:72-73`; comment block `:47-71`; `postgres` + `env_file` untouched. |
| `web/app/lib/authStore.ts` / `api.ts` / `components/auth/PassphraseScreen.tsx` / `root.tsx` | SPA 401 interceptor + gate branch + logout | ✓ VERIFIED | Single 401 flip point; header on every non-GET; passphrase only in POST body; early return on `!authed`; `LogoutButton` gated on `useGateActive()`. 101/101 web tests. |
| `.env.example` | documented `INSTANCE_PASSPHRASE` + `TRUST_PROXY_HEADERS` | ✓ VERIFIED | `git show HEAD:.env.example` — both keys present (`INSTANCE_PASSPHRASE=caliber` placeholder, `TRUST_PROXY_HEADERS=false`). `TestEnvExampleCompleteness` green. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| operator `.env` `INSTANCE_PASSPHRASE` line | `app` container env | `env_file: .env` (primary) + `environment: ${INSTANCE_PASSPHRASE:-}` (additive) | ✓ WIRED — operator confirmed the container now sees a non-empty value (boot log ACTIVE). |
| `cmd/server/main.go` | `httpserver.New` | `WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(...))` | ✓ WIRED — `main.go:238`. |
| `cfg.InstancePassphrase` | `logInstanceGateStatus` AND `httpserver.WithAuthGate` | same in-memory value, no second `os.Getenv` | ✓ WIRED — `main.go:135` and `:238` both read `cfg.InstancePassphrase`; the log line cannot report active while the gate is inert. |
| `httpserver.New` | protected `r.Group` | `if cfg.gatePassphrase != "" { gate = NewManager(...) }` then `pr.Use(gate.Authenticate)` inside the Group | ✓ WIRED — `server.go:114-178`; single branch; makes GATE-07 true. |
| `web/app/lib/api.ts` `X-Requested-With: drop-tracker` | `internal/authgate/gate.go` `csrfHeaderName/Value` | identical string literals | ✓ WIRED — byte-for-byte match. |
| `api.ts` 401 | `authStore` → `<App>` → `<PassphraseScreen/>` | `markUnauthenticated()` → `useAuthed()` false → early return | ✓ WIRED — `root.tsx:105`; operator observed the form render. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Build + vet | `go build ./...` ; `go vet ./cmd/server ./internal/config ./internal/httpserver ./internal/authgate` | exit 0 | ✓ PASS |
| authgate + config + cmd/server suites | `go test ./cmd/server/ ./internal/config/ ./internal/authgate/ -count=1` | all `ok` | ✓ PASS |
| httpserver suite | `go test ./internal/httpserver/ -count=1` | `ok` | ✓ PASS |
| 14-05 regression tests | `go test ./cmd/server/ -run TestLogInstanceGateStatus -v` ; `go test ./internal/config/ -run TestDockerComposeWiresGateEnvVars -v` | `--- PASS` (3 subtests) | ✓ PASS |
| Frontend suite + coverage | `web/node_modules/.bin/vitest run` | 12 files / 101 tests passed; coverage 87.55 / 77.73 / 86.02 / 88.74 (all > 70) | ✓ PASS |
| Compose gate wiring present | inspect `docker-compose.yml:72-73` | `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}` / `TRUST_PROXY_HEADERS: ${TRUST_PROXY_HEADERS:-false}` under `app.environment:` | ✓ PASS |
| Debt markers in 14-05 files | grep `TODO/FIXME/XXX/HACK/TBD` in `cmd/server/main.go`, `main_test.go`, `config_test.go`, `docker-compose.yml` | none | ✓ PASS |
| `go.mod` / `go.sum` drift | `git diff da5c8fb..HEAD -- go.mod go.sum` | empty | ✓ PASS |
| Live browser gate behaviour | — | not runnable here (server + real browsers) | ? SKIP → human (item 1) |
| Live Discord alert | — | not runnable here (external service) | ? SKIP → human (item 3) |

### Requirements Coverage

| Requirement | Source Plans | Status | Evidence |
|-------------|--------------|--------|----------|
| GATE-01 | 14-01, 14-04, **14-05** | ✓ SATISFIED | Truth 1; 14-05 made the gate reachable in the compose stack (was inert → now ACTIVE). |
| GATE-02 | 14-01 | ✓ SATISFIED | Truth 3 (stateless HMAC cookie survives a new Manager). |
| GATE-03 | 14-01, 14-04 | ✓ SATISFIED | Truth 4 (cookie attributes + constant-time compare). |
| GATE-04 | 14-02 | ✓ SATISFIED | Truth 4 (per-IP throttle, undelayed 429, bounded concurrency, proxy-trust gating). |
| GATE-05 | 14-03 | ✓ SATISFIED (code + partial operator confirmation) | Truth 2; full unlock/wrong-passphrase flow → human. |
| GATE-06 | 14-01, 14-03 | ✓ SATISFIED | Truth 3 (client-local logout). |
| GATE-07 | 14-01, 14-04, **14-05** | ✓ SATISFIED | Truth 5; 14-05 added the inert-branch boot log without changing the inert route table; UAT Test 3 PASS. |

All 7 phase requirement IDs (GATE-01…GATE-07) are claimed by a plan and implemented. REQUIREMENTS.md maps exactly `GATE-01…GATE-07` to Phase 14 (line 110: "GATE-01 … GATE-07 | 7") — no orphaned requirements. Every plan's `requirements:` frontmatter (14-01: GATE-01/02/03/06/07; 14-02: GATE-04; 14-03: GATE-05/06; 14-04: GATE-01/03/07; 14-05: GATE-01/07) resolves to an ID in that set. GATE-08 (key rotation) is explicitly out of scope in REQUIREMENTS.md (line 57).

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| — | none | — | No debt markers, stubs, hollow props, or hardcoded-empty data paths in the phase surface, including the 14-05 files. |

### Residual Risks (not blockers)

| Item | Source | Disposition |
|------|--------|-------------|
| **WR-01 — compose `environment:` entry can clobber a valid `.env` passphrase with an empty string when `docker compose` is run from another directory / with an external `--env-file`** | 14-REVIEW.md WR-01 | WARNING. New regression path introduced by the 14-05 fix. Mitigated by the boot-status log (surfaces inert every affected start) and the in-file comment. Operator's standard `docker compose up --build` from repo root is unaffected and confirmed working. Recommend the reviewer's fix (drop `INSTANCE_PASSPHRASE` from `environment:` and keep `env_file` sole, or expand the comment) be tracked for Phase 17 runbook work. |
| WR-02 — `TestDockerComposeWiresGateEnvVars` does a flat line scan, not scoped to `services.app.environment` | 14-REVIEW.md WR-02 | Info. Test under-asserts vs its docstring; the wiring itself is correct. |
| WR-03 — `TestLogInstanceGateStatus` active branch does not lock out a future passphrase *hash* attribute | 14-REVIEW.md WR-03 | Info. The implementation (`main.go:73-81`) logs only `status` + `hint` — verified by direct read, no leak. Test robustness gap only. |
| WR-04 — passphrase interpolation assertion accepts a hardcoded `${INSTANCE_PASSPHRASE:-default}` | 14-REVIEW.md WR-04 | Info. Asymmetric with the stricter `TRUST_PROXY_HEADERS` check; current compose file has no default. |
| Cookie name lacks `__Host-` prefix (D-09 literal deviation, operator option-a) | 14-01-SUMMARY, code review WR-01 (prior round) | Accepted locked decision; residual sibling-subdomain / plaintext-hop cookie-injection risk before Phase 17 TLS. |
| `DeriveKey` = single unsalted SHA-256 → one captured cookie enables offline passphrase brute-force | Code review WR-02 (prior round) | Accepted locked decision D-01; mitigated by the 24+ char `.env.example` recommendation and the weak-passphrase WARN (warn-only). |
| `middleware.RealIP` trusts XFF with no trusted-proxy CIDR allowlist | Code review WR-03 (prior round) | Accepted locked decision D-14 (fail-safe default false); Phase 17 runbook owns the unpublished-port coupling. |
| `go test -race` / `make test` unusable on this Windows dev machine (ThreadSanitizer allocation failure, STATE.md) | 14-01/14-02/14-05 SUMMARY | Accepted — concurrency truths classified backstop-tier for this reason; CI runs the same suite under `-race`. Behavioral tests exist and pass without `-race`. |

### Human Verification Required

1. **Complete 14-UAT.md Test 1 in real browsers (Chrome + Firefox, http://localhost)** — the gate is now ACTIVE and the operator has confirmed the passphrase form renders instead of the watchlist. Remaining: enter the correct passphrase and confirm the watchlist/history UI loads; refresh and confirm it stays unlocked; enter a wrong passphrase and confirm the fixed inline error; click Log out and confirm return to the form; repeat in both browsers. *Why human:* browser cookie / Secure-over-localhost behaviour is not reproducible in httptest/jsdom (14-VALIDATION.md Manual-Only). 14-UAT.md Test 1 still records `result: issue` from the pre-14-05 round and must be re-run.
2. **PassphraseScreen visual conformance to 14-UI-SPEC** — spacing / colour / typography pillars on the running SPA (viewport-centred `max-w-sm` `bg-card` card, `gap-6` rhythm, indigo accent on Unlock fill + focus ring, destructive colour on error text only, dark surface). *Why human:* no screenshot/Playwright step; 14-03-SUMMARY coverage item D8 is `human_judgment: true`. 14-UAT.md Test 2 was `blocked` on G-14-1 and is now unblocked.
3. **Live Discord brute-force alert** — with `DISCORD_WEBHOOK_URL` set, drive >20 failed logins within 5 minutes against the running gated instance; confirm exactly one alert embed carrying only a count + window (no passphrase, no fragment, no length) and no further alert for 15 minutes. *Why human:* the live webhook send is never exercised by the test suite. 14-UAT.md Test 4 was `blocked` on G-14-1 and is now unblocked.

**Already satisfied (no action needed):** 14-UAT.md Test 3 (`docker compose up` with no passphrase → all routes behave as v1.2) — PASS. Operator confirmation that the boot log reports the gate ACTIVE and the browser shows the passphrase form instead of the watchlist — the G-14-1 blocker — is done.

### Gaps Summary

No blocking gaps. All seven GATE requirement IDs are implemented, wired end-to-end (operator `.env` → `env_file`/`environment:` → `cfg.InstancePassphrase` → `httpserver.WithAuthGate` → chi Group middleware → `/session` handlers, and SPA `apiFetch` → `authStore` → `<App>` → `PassphraseScreen`), and covered by a green backend suite (`go test ./...`, every package `ok`) and a green frontend suite (101/101, coverage > 70 on all axes). **G-14-1 — the configuration-reachability gap that left the gate inert in the docker-compose stack through the first UAT round — is closed:** the compose file now forwards the gate env vars with a regression test, the boot emits a secret-free active/inert status line with its own test, the UAT precondition names the working channel, and the operator has reconciled the live `.env` and confirmed the gate reports ACTIVE with the passphrase form rendering.

Status is `human_needed` because three UAT items — the full real-browser unlock/persistence/wrong-passphrase/logout flow across Chrome + Firefox, PassphraseScreen visual conformance, and live Discord alert rendering — cannot be confirmed programmatically and are explicitly routed to a human by the phase's own validation contract. One new WARNING-level residual risk (WR-01: the compose `environment:` entry can emit an empty string that clobbers a valid `.env` value when compose is invoked from outside the repo root) is recorded for Phase 17 runbook follow-up; it does not block the phase goal and is now observable at boot.

---

_Verified: 2026-08-31_
_Verifier: Claude (gsd-verifier)_
_Re-verification after G-14-1 closure (plan 14-05)_
