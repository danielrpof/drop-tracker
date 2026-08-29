---
phase: 14-instance-passphrase-gate
verified: 2026-08-29T00:00:00Z
status: human_needed
score: 7/7 roadmap success criteria verified (all plan must-have truths backed by wired code + passing tests)
behavior_unverified: 0
overrides_applied: 0
human_verification:
  - test: "Set INSTANCE_PASSPHRASE locally, run the server, open http://localhost:8080 in Chrome AND Firefox. Confirm the passphrase form renders, the correct passphrase unlocks and stays unlocked across a page refresh, a wrong passphrase shows the fixed inline message, and Log out returns to the form."
    expected: "The gate works end-to-end in a real browser; the bare-name dt_session cookie (option-a, no __Host- prefix) is accepted and replayed by both browsers over plain http://localhost."
    why_human: "Chrome vs Firefox cookie-prefix / Secure-over-localhost behaviour is not reproducible in httptest or jsdom; this is the explicit Manual-Only row in 14-VALIDATION.md."
  - test: "Visual conformance of PassphraseScreen to 14-UI-SPEC — glance at the running SPA: viewport-centred max-w-sm bg-card, gap-6 rhythm, indigo accent reserved to the Unlock fill + input focus ring, destructive colour reserved to error text, dark surface."
    expected: "Matches the approved UI-SPEC spacing / colour / typography pillars."
    why_human: "RTL asserts copy, structure, roles and behaviour but not rendered appearance; no Playwright/screenshot step exists. 14-03-SUMMARY marks coverage item D8 human_judgment: true, deferred to end-of-phase."
  - test: "docker compose up with no INSTANCE_PASSPHRASE configured."
    expected: "Stack starts, all seven v1.2 routes answer as before, no passphrase prompt, no new required variable."
    why_human: "Compose-stack runtime behaviour; not exercised by the Go unit suite (inert-path routing IS covered by TestInertPath_* but the compose wiring is not)."
  - test: "Set DISCORD_WEBHOOK_URL and drive >20 failed logins within 5 minutes against a running instance; observe the Discord channel."
    expected: "Exactly one brute-force alert embed arrives, carrying only a count and window (no passphrase, no fragment, no length); no further alert for 15 minutes."
    why_human: "The live webhook send path is never exercised by the test suite (SelectAlerter branches are unit-tested with a fake); real Discord embed rendering needs a human eye."
re_verification:
---

# Phase 14: Instance Passphrase Gate — Verification Report

**Phase Goal:** A drop-tracker instance on a public URL only exposes its watchlist data, search proxy, and event history to someone who knows the instance passphrase — while local dev, docker-compose, and the existing test suites keep working with no passphrase configured at all.

**Verified:** 2026-08-29
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (rolled up to the 7 roadmap Success Criteria; plan must-haves cross-referenced)

| # | Truth (requirement) | Status | Evidence |
|---|---------------------|--------|----------|
| 1 | GATE-01 — with a passphrase set, data/API routes 401 without a valid cookie; `/health`, `POST/DELETE /session`, and the static SPA shell stay public | ✓ VERIFIED | `internal/httpserver/server.go:164-188` — data routes registered only inside `r.Group{ pr.Use(gate.Authenticate); pr.Use(gate.RequireCSRFHeader) }`; `/health` exact path, `/session`, and `r.NotFound(webassets.Handler)` on the root router in both branches. Tests: `TestGate_EndToEnd_401_Login_200_Logout`, `TestGate_ExemptionBoundary_HealthIsExactPathOnly`, `TestGate_PublicSPAShell`, `TestGate_EmptyCookieInputs`, `TestGate_RejectedRequestIsLoggedWithRequestID` — all pass. Exemption is structural (no `strings.HasPrefix(r.URL.Path`). |
| 2 | GATE-02 — authenticate once; a signed stateless cookie persists across requests and across restarts/redeploys | ✓ VERIFIED | `session.go` — `DeriveKey = SHA256(prefix‖passphrase)`, `Sign`/`Verify` are pure HMAC-SHA256 with no server-side store. `TestGate_MintedCookieSurvivesNewManager` (cookie from one Manager verifies against a second built from the same passphrase), `TestSignVerify_RoundTrip`, `TestGate_EndToEnd…` pass. |
| 3 | GATE-03 — cookie is HMAC-signed, HttpOnly, Secure, SameSite=Lax, bounded lifetime; passphrase comparison constant-time | ✓ VERIFIED | `gate.go:183-193` `setSessionCookie` via `http.SetCookie` (Path=/, HttpOnly, Secure, SameSiteLax, MaxAge). `login.go:260-261` `sha256` both sides → `subtle.ConstantTimeCompare` over 32-byte digests. `session.go:168` `hmac.Equal`; absolute-cap-before-expiry ordering at `:175-180`. Tests: `TestGate_LoginCookieAttributes` (parsed + raw header, Max-Age=2592000), `TestVerify` (8 subcases incl. tamper + wrong key + absolute cap), `TestGate_SlidingRenewal[_ShrunkDurations]`, `TestGate_AbsoluteCapRejectsUnexpiredToken`, `TestGate_TwoLoginsRotateCookieValue`. **Accepted deviation:** cookie name is bare `dt_session`, not D-09's `__Host-` prefix (operator chose option-a at the 14-01 checkpoint; recorded in 14-01-SUMMARY; residual cookie-injection risk noted by code review WR-01). |
| 4 | GATE-04 — login rate-limited per client IP; client IP from XFF only when `TRUST_PROXY_HEADERS`; undelayed 429; bounded login concurrency | ✓ VERIFIED | `login.go` — `loginThrottle` mutex-guarded `map[string]*ipLimiter` (`x/time/rate`, burst 5, `rate.Every(12s)`), ticker-driven `sweep`, non-blocking `loginSlots` semaphore (503 shed), undelayed 429, `loginDelay()` only on the 204 / wrong-passphrase-401 paths. `clientIP` = host of `r.RemoteAddr`; `server.go:132` wires `middleware.RealIP` only when `gate != nil && cfg.trustProxyHeaders`. Tests: `TestLoginThrottle_BoundaryAndSecondAddressUnaffected`, `TestLoginThrottle_RefillRestoresService`, `TestLoginDelay_OnComparisonPathButNotOn429`, `TestLoginConcurrency_ShedsExcessWith503`, `TestLimiterSweep_EvictsIdleEntries`, `TestGatedServer_TrustProxyHeaders_RealIPWiring` (2 subcases) — pass. |
| 5 | GATE-05 — SPA detects a 401, shows a passphrase form, resumes after a successful login | ✓ VERIFIED | `web/app/lib/api.ts:133-136` single `apiFetch` 401 branch → `authStore.markUnauthenticated()` + rethrow `ApiError(401)`. `web/app/root.tsx:105-107` early return `<PassphraseScreen/>` on `!authed` (remounts `<Outlet/>` on the flip → route mount effects re-fetch). `PassphraseScreen.tsx` calls `createSession(value)` then `markAuthenticated()` only on resolve; 401/429/network map to distinct fixed copy. 101 web tests pass incl. `api.test.ts`, `PassphraseScreen.test.tsx`, `root.test.tsx`, `watchlist.test.tsx` gate cases. |
| 6 | GATE-06 — user can log out, invalidating the session on that browser | ✓ VERIFIED | `login.go:310-322` `HandleLogout` → `setSessionCookie("", -1)` (Max-Age=0), 204, one audit line. `api.ts` `deleteSession()`; `root.tsx` `LogoutButton` clears local state in `finally` on success or failure + toast. Client-local per D-10. Tests: `TestGate_LogoutClearsCookie`, `TestLogoutCSRF_MissingHeaderRejected`, web `root.test.tsx` logout success/failure/double-click. |
| 7 | GATE-07 — with no passphrase, the gate is inert; every route behaves as pre-v1.3; local dev / docker-compose / tests need no passphrase | ✓ VERIFIED | `server.go:180-182` else-branch calls `registerDataRoutes(r, s)` directly; no `/session`, no `Authenticate`, no `RequireCSRFHeader`, no `RealIP`. `config.go:74-75` both fields optional, absent from `Load()` validation. Tests: `TestInertPath_FiveArgConstructor`, `TestInertPath_EmptyPassphraseIsIndistinguishable`, `TestInertPath_NoCSRFRejection`, `TestGate_NoOptionsIsUngated`. Full `go test ./... -short` passes with `INSTANCE_PASSPHRASE` unset. **Documented exception (code review IN-03, 14-04-SUMMARY):** every response — inert included — now carries `Referrer-Policy: no-referrer`; deliberate hardening, asserted by `TestReferrerPolicy_OnEveryResponse`. |

**Score:** 7/7 roadmap success criteria verified. All ~50 plan-frontmatter must-have truths across 14-01…14-04 trace to wired code and a passing named test (spot-checked above and enumerated in each SUMMARY's `coverage:` block).

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/authgate/session.go` | HMAC-SHA256 stateless session codec | ✓ VERIFIED | `DeriveKey`/`Token`/`Sign`/`Verify`, D-06 duration vars, `sessionCookieName="dt_session"`, absolute-cap-before-expiry ordering. |
| `internal/authgate/gate.go` | `Manager`, `Authenticate`, `RequireCSRFHeader`, cookie/JSON helpers | ✓ VERIFIED | Gate never falls open — only path to `next` is `Verify(...) ok==true`. `RequireCSRFHeader` after `Authenticate`. `NewManager` builds throttle/counter/semaphore + sweeper; `Close()` idempotent. |
| `internal/authgate/login.go` | `HandleLogin`/`HandleLogout`, throttle, delay, semaphore, global counter, audit lines | ✓ VERIFIED | CSRF check first (no token/counter touch), semaphore acquire, per-IP 429 undelayed, 4 KiB body cap, constant-time compare, `dispatchAlert` fire-and-forget under timeout, one slog line per path with `source_ip` only. |
| `internal/authgate/alerter.go` | `Alerter` seam + `SelectAlerter` + Discord impl | ✓ VERIFIED | Mirrors `notifier.Select`; empty webhook → one Info line + noop; send error never logged. |
| `internal/authgate/weak.go` | `IsWeakPassphrase` + `knownDefaults` | ✓ VERIFIED | empty→not weak; `<16` runes (`utf8.RuneCountInString`); case-insensitive trimmed denylist incl. `envExamplePlaceholder="caliber"`; reason never embeds input. |
| `internal/config/config.go` | `InstancePassphrase`, `TrustProxyHeaders` | ✓ VERIFIED | Both optional, `env:` tags, no `Load()` validation entry. |
| `internal/httpserver/server.go` | `Option`, `WithAuthGate`, protected Group, `securityResponseHeaders` | ✓ VERIFIED | Conditional wiring; four existing `r.Use` lines unchanged; `RealIP` behind `trustProxyHeaders`; `Close()` delegates to gate. |
| `cmd/server/main.go` | gate wiring + boot WARN | ✓ VERIFIED | `httpserver.WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(cfg.DiscordWebhookURL, logger))`; `IsWeakPassphrase` WARN between `logging.New` and migrations (no exit/error added); `defer srv.Close()`. |
| `web/app/lib/authStore.ts` | framework-free pub/sub + hooks | ✓ VERIFIED | `authed` optimistic true, `gateActive` false→true on first 401/login (D-18), idempotent marks, `useSyncExternalStore` hooks. |
| `web/app/lib/api.ts` | 401 interceptor, central `X-Requested-With`, session wrappers | ✓ VERIFIED | Single 401 flip point; header on every non-GET; passphrase only in POST body. |
| `web/app/components/auth/PassphraseScreen.tsx` | approved full-screen form | ✓ VERIFIED | Verbatim copy, `type=password` no placeholder autofocus, 401/429/network distinct fixed messages, value retained + never rendered/logged, button lock-until-edit after 401/429. |
| `web/app/root.tsx` | `<App>` gate branch + `LogoutButton` | ✓ VERIFIED | Early return on `!authed`; `LogoutButton` gated on `useGateActive()`; clears state in `finally`. |
| `.env.example` | documented `INSTANCE_PASSPHRASE` + `TRUST_PROXY_HEADERS` | ✓ VERIFIED | Verified via `git show HEAD:.env.example` — 24-char recommendation, revoke-all note, proxy/unpublished-port precondition, self-warning `caliber` placeholder. `TestEnvExampleCompleteness` (config pkg) green. |

### Key Link Verification

| From | To | Via | Status |
|------|----|----|--------|
| `cmd/server/main.go` | `httpserver.New` | `WithAuthGate(cfg.InstancePassphrase, cfg.TrustProxyHeaders, authgate.SelectAlerter(...))` | ✓ WIRED — line present at `main.go:210`; gate engages in production. |
| `httpserver.New` | protected `r.Group` | `if cfg.gatePassphrase != "" { gate = NewManager(...) }` then `r.Group{ pr.Use(gate.Authenticate) }` | ✓ WIRED — single branch; makes GATE-07 true. |
| `Manager.Authenticate` | protected sub-router only | `pr.Use` inside the Group, never a 5th top-level `r.Use` | ✓ WIRED — confirmed at `server.go:173`; `TestGate_RejectedRequestIsLoggedWithRequestID` proves ordering. |
| `web/app/lib/api.ts` `X-Requested-With: drop-tracker` | `internal/authgate/gate.go` `csrfHeaderName/Value` | identical string literals in both files | ✓ WIRED — `"X-Requested-With"` / `"drop-tracker"` match byte-for-byte. |
| `api.ts` 401 | `authStore` → `<App>` | `markUnauthenticated()` → `useAuthed()` false → `<PassphraseScreen/>` | ✓ WIRED — early return at `root.tsx:105`. |
| `main.go` boot | `IsWeakPassphrase` | one WARN call site, after logger, before migrations | ✓ WIRED — `main.go:105-107`. |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Full Go suite (short) | `go test ./... -short -count=1` | every package `ok` | ✓ PASS |
| `go build` / `go vet` | `go build ./... && go vet ./...` | exit 0 | ✓ PASS |
| authgate + httpserver named tests | `go test ./internal/authgate/ ./internal/httpserver/ -v` | 46+ authgate/httpserver tests PASS, 0 FAIL | ✓ PASS |
| Frontend suite + coverage | `web/node_modules/.bin/vitest run` | 101 passed / 101; coverage 87.55 / 77.73 / 86.02 / 88.74 (all > 70) | ✓ PASS |
| `.env.example` content | `git show HEAD:.env.example` | Phase 14 block present with 24-char rec, revoke-all note, proxy precondition, `caliber` placeholder | ✓ PASS |
| Debt markers in changed files | grep TODO/FIXME/XXX/HACK across `internal/authgate` + `web/app` | none | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plans | Description | Status | Evidence |
|-------------|--------------|-------------|--------|----------|
| GATE-01 | 14-01, 14-04 | Passphrase-gated data routes, public health/session/shell | ✓ SATISFIED | Truth 1 |
| GATE-02 | 14-01 | Authenticate once, stateless cookie survives restart | ✓ SATISFIED | Truth 2 |
| GATE-03 | 14-01, 14-04 | Signed HttpOnly/Secure/SameSite=Lax bounded cookie, constant-time compare | ✓ SATISFIED | Truth 3 |
| GATE-04 | 14-02 | Per-IP login throttle, undelayed 429, bounded concurrency, proxy-trust gating | ✓ SATISFIED | Truth 4 |
| GATE-05 | 14-03 | SPA 401 → passphrase form → resume | ✓ SATISFIED | Truth 5 |
| GATE-06 | 14-01, 14-03 | Client-local logout | ✓ SATISFIED | Truth 6 |
| GATE-07 | 14-01, 14-04 | Inert when unconfigured | ✓ SATISFIED | Truth 7 |

All 7 phase requirement IDs are claimed by a plan and implemented. No orphaned requirements (REQUIREMENTS.md maps exactly GATE-01…GATE-07 to Phase 14).

### Anti-Patterns Found

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| — | none | — | No debt markers, no stubs, no hollow props, no hardcoded-empty data paths in the phase surface. |

### Residual Risks (not blockers)

| Item | Source | Disposition |
|------|--------|-------------|
| `TestGate_Concurrency`, `TestLoginThrottle_ParallelSameAddressConsistent`, `TestLoginConcurrency_ShedsExcessWith503` pass but `-race` is CI-only on this dev machine (ThreadSanitizer limitation, STATE.md) | 14-01/14-02 SUMMARY, VALIDATION.md | Accepted — the plans classified GATE-01/GATE-04 concurrency as backstop-tier for exactly this reason; CI runs the same suite under `-race`. Behavioral tests exist and pass. |
| Cookie name lacks `__Host-` prefix (D-09 literal deviation) | Code review WR-01 | Accepted locked decision (operator option-a); residual sibling-subdomain / plaintext-hop cookie-injection risk before Phase 17 TLS. Recommend recording the residual risk in phase security notes. |
| `DeriveKey` = single unsalted SHA-256 → one captured cookie enables offline passphrase brute-force | Code review WR-02 | Accepted locked decision D-01; mitigated by the 24+ char `.env.example` recommendation and the weak-passphrase WARN. `IsWeakPassphrase` only warns — a 1-char passphrase still boots. |
| `middleware.RealIP` trusts XFF with no trusted-proxy CIDR allowlist | Code review WR-03 | Accepted locked decision D-14 (fail-safe default false); Phase 17 runbook owns the unpublished-port coupling. |
| Fire-and-forget alert goroutines not drained at shutdown | Code review WR-04 | Info-level nit; `discord` client's own ctx bounds lifetime to ~10s. |
| `PassphraseScreen` collapses 403 and 5xx into the connection message | Code review IN-04 | Cosmetic; a corporate proxy stripping `X-Requested-With` would show a misleading message. Consider a follow-up. |

### Human Verification Required

1. **Real-browser cookie behaviour (Chrome + Firefox, http://localhost)** — set `INSTANCE_PASSPHRASE`, run the server, confirm the form renders, correct passphrase unlocks and survives a refresh, wrong passphrase shows the fixed message, Log out returns to the form. *Why human:* browser cookie/Secure-over-localhost behaviour is not reproducible in httptest/jsdom (14-VALIDATION.md Manual-Only).
2. **PassphraseScreen visual conformance to 14-UI-SPEC** — spacing / colour / typography pillars on the running SPA. *Why human:* no screenshot/Playwright step; 14-03-SUMMARY coverage item D8 is `human_judgment: true`, deferred to end-of-phase.
3. **`docker compose up` with no passphrase** — all seven routes answer as v1.2, no prompt, no new required var. *Why human:* compose-stack runtime, not in the unit suite.
4. **Live Discord brute-force alert** — trip the threshold against a running instance with `DISCORD_WEBHOOK_URL` set; confirm one embed, count+window only, 15-min cooldown. *Why human:* the live webhook send is never exercised by tests.

### Gaps Summary

No blocking gaps. All seven roadmap success criteria are implemented, wired end-to-end (config → `internal/authgate` codec → chi Group middleware → `/session` handlers → `cmd/server/main.go`, and SPA `apiFetch` → `authStore` → `<App>` → `PassphraseScreen`), and covered by a green backend suite (`go test ./... -short`, every package `ok`) and a green frontend suite (101/101, coverage > 70 on all axes). The inert path is structurally absent when `INSTANCE_PASSPHRASE` is unset, with the one documented, deliberate exception of an always-on `Referrer-Policy` header.

Status is `human_needed` solely because four verification items — real-browser cookie behaviour, UI-SPEC visual conformance, the compose-stack inert check, and live Discord alert rendering — cannot be confirmed programmatically and are explicitly routed to a human by the phase's own validation contract and 14-03 code-review notes.

---

_Verified: 2026-08-29_
_Verifier: Claude (gsd-verifier)_
