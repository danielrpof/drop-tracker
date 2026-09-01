# Phase 14: Instance Passphrase Gate - Research

**Researched:** 2026-08-27
**Domain:** Stateless HMAC session-cookie auth gate for a chi/Go API + React Router v7 SPA (stdlib crypto only)
**Confidence:** HIGH — every locked decision maps cleanly onto stdlib + already-present dependencies; one soundness flag on the `__Host-` cookie prefix (see A1).

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

Copied verbatim from `14-CONTEXT.md` `<decisions>`. Research answers **HOW**, not whether.

> **Amendments (2026-08-27 plan-hardening pass) — `14-CONTEXT.md` is authoritative where this list is now stale:**
> - **D-12** — the fixed delay applies only to the two paths that run the passphrase comparison (204 / wrong-passphrase 401); the `429` throttle rejection is **undelayed** (a delayed 429 parks a goroutine + connection). A `maxConcurrentLogins` (~32) semaphore bounds the handler; excess sheds with `503`. Global counter stays alert-only.
> - **D-14** — `middleware.RealIP` is wired **only** when a second new env var `TRUST_PROXY_HEADERS` (default `false`) is truthy. Unset (local dev, docker-compose, CI, any pre-proxy deploy) → throttle + audit key on `r.RemoteAddr`. The "exactly one new env var" goal (D-07) is relaxed to two. **The §Pattern 1 code sketch below is stale:** `WithAuthGate` takes three args — `WithAuthGate(passphrase string, trustProxyHeaders bool, alerter authgate.Alerter)` — and the `if cfg.gate != nil` branch gates `r.Use(middleware.RealIP)` behind a further `&& cfg.trustProxyHeaders`.
> - **D-06** — the 90-day absolute cap is measured per-authentication (fixed across sliding renewals, reset by a fresh passphrase entry), not a lifetime "first login ever" ceiling.
> - **D-18 (new)** — the SPA auth store carries a `gateActive` boolean; the Log out control renders only when it is true. This locks 14-UI-SPEC's one previously-unresolved item.
> - Rotating `INSTANCE_PASSPHRASE` (the revoke-all lever) must be documented in the Phase 17 runbook (D-10).

- **D-01:** HMAC-SHA256 signing key is **derived from the passphrase**: `key = SHA256(INSTANCE_PASSPHRASE)`. Exactly one new secret. Rotating `INSTANCE_PASSPHRASE` changes the derived key and invalidates every session — rotation *is* revoke-all. No separate `SESSION_SECRET`, no key-version byte. Reversibility: costly.
- **D-02:** Cookie payload is a signed "authenticated until T" — expiry timestamp + random nonce, HMAC'd with the derived key. No PII, no user identity. Issued-at / first-login timestamp is carried in the payload and does **not** move on renewal (needed for the D-06 absolute cap).
- **D-03:** The gate is a **pure API concern**. `/search`, `/watchlist` (incl. `POST`/`PATCH`/`DELETE`), `/events` are gated. Exempt (registered outside the protected `Group`): `GET /health` (exact path only, not a prefix), the session-login endpoint, and the SPA `NotFound` fallback (`webassets.Handler` — `index.html` + hashed JS/CSS under `/assets/`).
- **D-04:** The **static SPA shell serves publicly**. Unauthenticated visitor gets the bundle, SPA boots, calls the API, gets `401`, renders the passphrase screen. The stricter "gate everything but a Go-served `/login`" option was rejected.
- **D-05:** Gate middleware applied to the route `Group`, **after** all four existing middlewares (`RequestID` → `echoRequestID` → `httplog` → `Recoverer`), so rejected `401`s are logged with a request id. **Not** a fifth top-level `r.Use`. No in-middleware URL-path string matching — exemptions are "registered outside the group." Wired via `httpserver.WithAuthGate(passphrase)` functional option matching the `detection.With*` / `poller.With*` / `watchlist.With*` idiom; omitting it keeps every current test call site unchanged.
- **D-06:** **Sliding renewal.** Cookie `Max-Age` 30 days; when a request arrives past the halfway mark (cookie older than ~15 days), re-issue a fresh 30-day cookie. Absolute cap ~90 days from first login, after which re-auth is required regardless of activity.
- **D-07:** The 30-day window and 90-day cap are **hardcoded constants** in `internal/authgate`, not env vars. `INSTANCE_PASSPHRASE` stays the *only* new configuration variable. No `SESSION_TTL`.
- **D-08:** A container restart / redeploy does **not** log anyone out (stateless cookie).
- **D-09:** Cookie attributes: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`. Use the `__Host-` name prefix. `Secure` kept on even for local `http://localhost` — no dev toggle. **[SEE A1 — technically unsound as written for Chrome; recommendation below.]**
- **D-10:** Logout is **client-local only**. `DELETE /session` responds with `Set-Cookie … Max-Age=0`. A cookie copied elsewhere stays valid until TTL. Revoke-all = rotate `INSTANCE_PASSPHRASE`.
- **D-11:** **Warn only** — no boot-time strength enforcement. If `INSTANCE_PASSPHRASE` is set and looks weak (short, or a known-default like `changeme` / the `.env.example` value), log `WARN` at boot and start normally. Never refuse to start. `.env.example` recommends a 24+ char random value in its comment.
- **D-12:** **Per-IP throttle + global counter + Discord alert.** Per-IP: `golang.org/x/time/rate` keyed on client IP, ~5 attempts/min then `429`, plus a fixed ~250ms–1s delay on *every* login response. Global: process-wide failed-attempt counter; past a threshold in a window fires a Discord alert through `internal/discord`, and may apply a short endpoint cooldown. Passphrase comparison is `crypto/subtle.ConstantTimeCompare` on equal-length SHA-256 digests regardless of throttle state.
- **D-13:** Auth events (login success / failure / logout, with source IP) emitted as structured `slog` lines. Passphrase must never reach a log line (extends the Phase 01 `redactError` pattern; verify `httplog` logs no bodies).
- **D-14:** Use `chi/middleware.RealIP` for the per-IP throttle and the audit log. Safe **only** if the app is never reachable except through the reverse proxy. Phase 14 adds a load-bearing code comment + runbook note: *the container port must not be published on the VPS — proxy only.* Phase 17 enforces.
- **D-15:** `SameSite=Lax` **plus** a required custom header on all non-GET gated requests (e.g. `X-Requested-With`), set by the `web/app/lib/api.ts` fetch wrapper. Server rejects non-GET gated requests lacking it. Applies to `POST /session` too. CORS stays entirely absent.
- **D-16:** **Global 401 interceptor** in `apiFetch` (`web/app/lib/api.ts`): catches any `401`, flips a shared auth state, app renders the passphrase screen. No boot-time `GET /session` check. On successful login the SPA re-fetches the original data.
- **D-17:** On successful passphrase verification, issue a **brand-new** session cookie (new nonce, fresh expiry). Never carry a pre-auth identifier across the auth boundary.

### Claude's Discretion

- Exact endpoint paths / method shapes (`POST /session` + `DELETE /session` vs. `/auth/login` + `/auth/logout`).
- The `401` JSON body shape; the exact cookie name under the `__Host-` prefix.
- Rate-limit constants (attempts/window, delay range, global threshold + window); whether the global counter imposes a cooldown lock or only alerts.
- Audit-log line format; passphrase-weakness heuristic specifics.
- `Referrer-Policy: no-referrer` response header (Pitfall 14) and confirming `httplog` doesn't log bodies / scrubs the auth cookie — required hardening, exact wiring is discretion.
- How `internal/authgate` exposes its API (sign/verify types, middleware constructor, login/logout handlers) mirroring existing seam / functional-option conventions.
- `rate.Limiter`-per-IP lifecycle (map + eviction / `sync.Map` / small LRU).
- How the SPA passphrase screen + shared auth state fit React Router v7 SPA mode (dark-theme-only) with no UI-SPEC yet.
- Go + frontend test strategy.

### Deferred Ideas (OUT OF SCOPE)

- Server-side session revocation / signing-key rotation without logging everyone out — future `GATE-08`.
- `SESSION_TTL` env knob — rejected (D-07).
- Boot-time fail-closed passphrase-strength enforcement — rejected (D-11).
- Boot-time `GET /session` status check for a flash-free first paint — rejected (D-16).
- "Don't publish the container port" topology + reverse proxy / TLS — Phase 17 (DPLY-08). Phase 14 only documents the invariant.
- Two-tier `/health` (`/health` public-minimal + `/health/details` gated) — Phase 17 decides.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description (from REQUIREMENTS.md §"Access Gate") | Research Support |
|----|--------------------------------------------------|------------------|
| GATE-01 | Passphrase configured → all data/API routes require a valid session cookie and return `401` without one; `/health`, session-login endpoint, static SPA shell stay public | Protected chi `Group` + exempt routes registered outside it (§Architecture Patterns · Pattern 1). Exact-path `/health` via chi `r.Get("/health", …)` — chi matches the registered literal, never a prefix. |
| GATE-02 | Authenticate once → signed stateless cookie keeps the browser authenticated across requests **and across restarts/redeploys** | HMAC-SHA256 token, no server store (§Standard Stack, §Pattern 2). Survives restart because nothing is held in process memory. |
| GATE-03 | Cookie is HMAC-signed, `HttpOnly`, `Secure`, `SameSite=Lax`, bounded lifetime; passphrase comparison constant-time | `crypto/hmac` + `crypto/sha256`; `http.Cookie` attributes (§Pattern 3); `subtle.ConstantTimeCompare` / `hmac.Equal` on 32-byte digests (§Don't Hand-Roll, §Pitfall 3). |
| GATE-04 | Login endpoint rate-limited per client IP | `golang.org/x/time/rate` limiter-per-IP map with eviction (§Pattern 4), `chi/middleware.RealIP` for the key (D-14). |
| GATE-05 | SPA detects `401`, shows a passphrase form, resumes normal operation after login | `apiFetch` global 401 interceptor → module-level `authStore` → `<App>` renders `<PassphraseScreen>` (§Pattern 5). Re-fetch is automatic via `<Outlet/>` remount. |
| GATE-06 | User can log out, invalidating the session on that browser | `DELETE /session` → `Set-Cookie … Max-Age=0` (D-10); client-local only, accepted. |
| GATE-07 | No passphrase configured → gate inert, every route behaves exactly as v1.2; local dev, docker-compose, existing test suite need no passphrase | `WithAuthGate` functional option not supplied → `New` registers routes flat, no `Group`, no `RealIP`, no `/session` (§Pattern 1, §Common Pitfalls · Pitfall 7). |
</phase_requirements>

## Summary

Every locked decision in this phase is implementable with the Go standard library plus dependencies already in `go.mod` — there is **nothing to install** (`crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `encoding/base64`, `net/http`, `math/rand/v2` from stdlib; `golang.org/x/time/rate v0.15.0`, `github.com/go-chi/chi/v5 v5.3.1` incl. `chi/middleware.RealIP`, `github.com/go-chi/httplog/v3 v3.4.0`, `github.com/danielrpof/drop-tracker/internal/discord` already present). This matches STACK.md's stdlib-only posture and the project's hand-roll-small-things ethos (Discord notifier, MB/Deezer clients).

The work is a new `internal/authgate` package (HMAC sign/verify + a `Manager` struct bundling the per-IP limiter map, the global failed-attempt counter, and a Discord `Alerter` seam), a protected chi `Group` plus `POST`/`DELETE /session` routes registered via a `httpserver.WithAuthGate(...)` functional option, one new `Config` field, one wiring line in `main.go` plus a boot-time weakness `WARN`, and on the frontend a module-level `authStore` that `apiFetch` pokes on `401`, an `<App>`-level `<PassphraseScreen>` branch, a logout control, and two new `api.ts` wrappers. The backend `401`/cookie contract and the SPA's `401` handling are one feature and must land together (PITFALLS #23).

**One soundness flag (A1):** D-09's combination of the `__Host-` cookie prefix + "`Secure` on for localhost, no dev toggle" is **not sound in Chrome** — Chrome rejects `__Host-`/`__Secure-` prefixed cookies on `http://localhost` because the scheme is not HTTPS (Firefox accepts them). Since the gate is inert by default in local dev (GATE-07), this only bites an operator who deliberately sets `INSTANCE_PASSPHRASE` locally and tests the login flow in Chrome over plain http — they would see an infinite login loop (server 200, cookie silently dropped). **Primary recommendation:** drop the `__Host-` prefix, use the bare name `dt_session` with explicit `Secure; HttpOnly; SameSite=Lax; Path=/` attributes (Chrome *does* accept a non-prefixed `Secure` cookie on `http://localhost`). The `__Host-` prefix's guarantees (no `Domain`, `Path=/`, `Secure`) are things this code sets and validates itself; the prefix mainly defends against a sibling subdomain app overwriting the cookie, which is not in this single-origin threat model. Take this to the planner / a quick discuss confirmation.

**Primary recommendation:** Build `internal/authgate` as a narrow `Manager` struct (mirrors `detection.Detector` / `notifier.Notifier`); wire it through `httpserver.WithAuthGate`; register `/session` + the protected `Group` only when a passphrase is configured; on the frontend use a non-React module `authStore` so `apiFetch` can flip auth state without a provider dependency, and let route components re-fetch for free by remounting under `<App>`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Passphrase verification, session mint/verify | API / Backend (`internal/authgate`) | — | The only secret-bearing check; stateless token is signed and validated server-side. |
| Enforce `401` on unauthenticated data routes | API / Backend (chi group middleware) | — | Enforcement is at the API boundary only (FEATURES.md "what protected should mean"). |
| Per-IP throttle + global brute-force counter | API / Backend (`authgate.Manager`) | Notifier (Discord alert) | Attack surface is the HTTP login endpoint; alert reuses the existing webhook sink. |
| Client-IP resolution | API / Backend (`chi/middleware.RealIP`) | Reverse proxy (Phase 17) | Proxy sets `X-Forwarded-For`; app trusts it *only* because the container port is unpublished. |
| CSRF defense (custom header + `SameSite=Lax`) | API / Backend (group middleware) + Browser/Client (`api.ts` sets header) | — | Split: server rejects, client always sends. |
| Passphrase form, shared auth state, logout control | Browser/Client (React SPA) | — | Pure UX; no enforcement value. SPA route "guards" are cosmetic. |
| Static SPA shell + `/assets/` bundle delivery | Frontend Server (Go `webassets.Handler` via `NotFound`) | — | Public by design (D-04) — bundle carries no secrets. |
| Audit log of auth events | API / Backend (`slog` via existing logger) | — | Consistent with Phase 01 structured-logging posture. |

## Standard Stack

### Core (all already present — nothing to install)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `crypto/hmac`, `crypto/sha256` | stdlib (Go 1.26) `[VERIFIED: go.mod line "go 1.26"]` | HMAC-SHA256 sign/verify of the session token; SHA-256 of the passphrase for the derived key and for the constant-time compare | STACK.md §"Feature 2" explicitly: "Pure stdlib crypto; no version coupling." A signed cookie is ~30 lines. |
| `crypto/subtle` | stdlib | `ConstantTimeCompare` on the two 32-byte SHA-256 digests (D-12, GATE-03) | Canonical timing-safe comparison. `hmac.Equal` (also constant-time) is equivalent for the MAC check. `[CITED: pkg.go.dev/crypto/subtle]` |
| `encoding/base64` | stdlib | `base64.RawURLEncoding` for the cookie value segments | Cookie-value safe, no padding. |
| `math/rand/v2` | stdlib | Jitter for the fixed 250ms–1s per-login delay (D-12) — not security-sensitive, only anti-timing/anti-hammer | Modern stdlib RNG; no need for `crypto/rand` here. Use `crypto/rand` **only** for the token nonce. |
| `crypto/rand` | stdlib | 16-byte session nonce (D-02, D-17) | Nonce must be unpredictable so each minted token is unique. |
| `github.com/go-chi/chi/v5` + `chi/middleware` | v5.3.1 `[VERIFIED: go.mod]` | `chi.Router.Group` for the protected sub-router; `middleware.RealIP` for the throttle/audit IP (D-14) | Already the router. `RealIP` is bundled, zero extra deps. Group middleware runs **after** parent `r.Use` middleware — chi guarantees this, so the "after RequestID→echoRequestID→httplog→Recoverer" ordering (D-05) is automatic. `[CITED: .planning/research/ARCHITECTURE.md §"Feature 2"]` |
| `golang.org/x/time/rate` | v0.15.0 `[VERIFIED: go.mod]` | Token-bucket `rate.Limiter` per client IP for login throttling (GATE-04, D-12) | Already a dependency (external-API limiters in `cmd/server/main.go`). |
| `github.com/go-chi/httplog/v3` | v3.4.0 `[VERIFIED: go.mod + go doc httplog/v3.Options this session]` | Existing access-log middleware; **already safe** for D-13 (see §Common Pitfalls · Pitfall 6) | Default `LogRequestHeaders` is `["Content-Type", "Origin"]`; `LogResponseHeaders` default is none; `LogRequestBody`/`LogResponseBody` are unset in `server.go`. Neither `Cookie` nor the POST body is logged. |
| `github.com/danielrpof/drop-tracker/internal/discord` | in-repo | Brute-force alert sink (D-12) via `discord.Client.Send` with a typed `Embed` | Reuse the existing hand-rolled client; mirror `notifier.Select`'s NoOp idiom for the unconfigured-webhook case. `[VERIFIED: internal/discord/client.go:98-112]` |

### Frontend (all already present)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `react-router` (v7, SPA mode `ssr: false`) | in `web/package.json` | The passphrase screen is a normal SPA view gated by shared state at `<App>` level; `<Outlet/>` remount drives the re-fetch | Phase 06 scaffold. `createRoutesStub` is the testing primitive (`web/app/lib/test/routeStub.tsx`). `[VERIFIED: web/app/root.tsx:1-9, web/app/routes.ts]` |
| Vitest 4 + `@testing-library/react` + `@testing-library/user-event` + `jest-dom` | present (Phase 08) | Unit/RTL tests for `authStore`, `<PassphraseScreen>`, the `<App>` gate branch, and an `apiFetch` 401 test | `web/vitest.config.ts` + `web/vitest.setup.ts` already wired; coverage gate at 70% all four axes. `[VERIFIED: web/vitest.config.ts:16-62]` |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled HMAC cookie | `gorilla/securecookie` | Only worth it for AEAD/rotation helpers, which D-01/D-07 explicitly don't want. STACK.md "What NOT to Use": gorilla toolkit archived/low-activity; "offers nothing over a hand-rolled signed cookie" here. |
| Stateless cookie | `alexedwards/scs` + `postgresstore` | Future-milestone tool if real accounts land (GATE-08). `memstore` is disqualified — wiped every redeploy. STACK.md flags this explicitly. |
| `__Host-dt_session` (D-09) | `dt_session`, no prefix, explicit attributes | **Recommended** — see A1 / Summary. `__Host-` breaks Chrome local-dev gate testing over `http://localhost`. |
| `chi/middleware.RealIP` | Parse only the proxy's last-appended XFF hop | D-14 already considered and rejected this. `RealIP` is fine given the unpublished-port invariant; the code comment + runbook note is the mitigation. |
| Separate `internal/authgate` middleware + separate handlers file | Everything in `internal/httpserver` | A dedicated package matches `internal/detection` / `internal/notifier` (narrow struct + seam, unit-testable without a router). CONTEXT.md canonical_refs and ARCHITECTURE.md both name `internal/authgate/`. |

**Installation:**
```bash
# Nothing. All crypto is stdlib; chi/middleware.RealIP, golang.org/x/time/rate,
# httplog/v3, and internal/discord are already in go.mod / the repo.
```

**Version verification (this session):**
- `go.mod`: `go 1.26`; `github.com/go-chi/chi/v5 v5.3.1`; `github.com/go-chi/httplog/v3 v3.4.0`; `golang.org/x/time v0.15.0`. `[VERIFIED: go.mod, read 2026-08-27]`
- `go doc github.com/go-chi/httplog/v3.Options` — confirmed default `LogRequestHeaders = ["Content-Type", "Origin"]`, `LogResponseHeaders` default none, body-logging predicates default nil. `[VERIFIED: go doc output, 2026-08-27]`

## Package Legitimacy Audit

**No external packages are installed by this phase.** Every dependency is either the Go standard library or already present in `go.mod` / the repo (`chi/v5`, `chi/middleware`, `golang.org/x/time/rate`, `httplog/v3`, `internal/discord`). The Package Legitimacy Gate is **not applicable**.

- Packages removed due to `[SLOP]` verdict: none.
- Packages flagged as suspicious `[SUS]`: none.

## Architecture Patterns

### System Architecture Diagram

```
                          ┌─────────────────────── unauthenticated ───────────────────────┐
                          │                                                               │
  Browser ──GET /──────────► r.NotFound → webassets.Handler ──► index.html + /assets/*.js  │ (D-04: public shell)
     │                                                                                     │
     │  SPA boots, apiFetch('/watchlist') ──► [chi chain] ──► r.Group{ Authenticate } ─────┘
     │        ▲                                                     │
     │        │  401 {"error":"unauthenticated"}                    │  no / bad / expired cookie
     │        └─────────────────────────────────────────────────────┘
     │
     │  apiFetch 401 interceptor ──► authStore.markUnauthenticated() ──► <App> renders <PassphraseScreen>
     │
     └──POST /session {passphrase} ──► [RealIP] ──► perIPLimiter.Allow()? ──no──► 429 (+ delay)
                                                        │ yes
                                                        ▼
                                   sha256(submitted) vs sha256(passphrase)  (subtle.ConstantTimeCompare)
                                          │ mismatch                    │ match
                                          ▼                             ▼
                        globalFailCounter++ ; slog(fail,ip)      mint Token{issued_at:=now, exp:=now+30d,
                        threshold? ──► discord.Alert             nonce:=rand16}  (D-17 always fresh)
                        401 (+ 250ms–1s delay)                   Set-Cookie: dt_session=b64(payload).b64(hmac)
                                                                 ; HttpOnly; Secure; SameSite=Lax; Path=/
                                                                 slog(success,ip) ; 204
     Browser  ──subsequent GET /events (Cookie: dt_session=…) ──► Authenticate:
                                        verify hmac.Equal ; now<exp ; now < issued_at+90d
                                        if now > exp-15d  ──► re-issue fresh 30d cookie (same issued_at)  (D-06)
                                        ──► next handler
              ──POST/PATCH/DELETE ──► RequireCSRFHeader: X-Requested-With present? ──no──► 403
     Browser  ──DELETE /session ──► Set-Cookie: dt_session=; Max-Age=0 ; 204  (D-10 client-local)

  GET /health  ──► handleHealth   (registered OUTSIDE the group — always public, exact path only)  (D-03)
```

### Recommended Project Structure

```
internal/authgate/
├── session.go        # DeriveKey, Token, Sign, Verify  — pure, no net/http
├── session_test.go   # table-driven: round-trip, tamper, expiry, absolute-cap, renewal boundary
├── gate.go           # Manager struct; Authenticate + RequireCSRFHeader middleware; renewal
├── gate_test.go      # httptest: 401 paths, exempt paths, cookie-flag assertions, CSRF header
├── login.go          # HandleLogin / HandleLogout; per-IP limiter map + eviction; global counter; delay
├── login_test.go     # wrong pass → 401, right pass → 204 + Set-Cookie, throttle → 429, alert fires
├── alerter.go        # Alerter interface + discordAlerter + noopAlerter (mirrors notifier.NoOp)
└── weak.go           # IsWeakPassphrase(string) (bool, reason) — used by main.go boot WARN

internal/httpserver/
├── server.go         # MOD: New(..., opts ...Option); Option/WithAuthGate; conditional Group + /session
└── server_test.go    # MOD: add gate-enabled construction helper; keep existing 5-arg call sites intact

internal/config/config.go   # MOD: + InstancePassphrase string `env:"INSTANCE_PASSPHRASE"`
cmd/server/main.go          # MOD: boot WARN + httpserver.WithAuthGate(cfg.InstancePassphrase, alerter)

web/app/lib/authStore.ts            # module-level pub/sub; markAuthenticated / markUnauthenticated / subscribe
web/app/lib/authStore.test.ts
web/app/lib/api.ts                  # MOD: 401 interceptor → authStore; X-Requested-With on non-GET; createSession/deleteSession
web/app/lib/api.test.ts             # NEW: real apiFetch, mocked global fetch → 401 flips authStore
web/app/components/auth/PassphraseScreen.tsx
web/app/components/auth/PassphraseScreen.test.tsx
web/app/root.tsx                    # MOD: <App> renders <PassphraseScreen> when !authed; logout button in nav
.env.example                        # MOD: INSTANCE_PASSPHRASE with a 24+ char recommendation comment
```

### Pattern 1: Conditional wiring via a trailing functional option (GATE-07)

**What:** `httpserver.New` gains a trailing `opts ...Option` param. `WithAuthGate(passphrase string, alerter authgate.Alerter)` is the only option. When it is **not** supplied (or the passphrase is empty), `New` registers the seven routes flat exactly as today and adds no middleware — the inert path has *zero* per-request cost and *zero* behavior change (success criterion 5).

**Why this over an always-present no-op middleware:** D-03 says the disabled case should be "a concrete no-op middleware, not a nil check." The *intent* — no per-request `if passphrase == ""` branching in the hot path — is fully preserved: when the gate is enabled the `Authenticate` middleware is unconditionally in the group chain; when disabled the group does not exist at all. Registering `/session` and a `Group` unconditionally would change v1.2 behavior for those paths (they currently fall through to the SPA shell). Conditional registration is the lower-risk reading of "every route behaves exactly as it did in v1.2." **Flag this interpretation for the planner** — it is a deliberate, testable deviation from the literal wording of D-03.

**Example (current → proposed):**
```go
// Source: internal/httpserver/server.go:62-92 (current, VERIFIED this session)
func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger) *Server {
    s := &Server{db: db, watchlist: store, events: eventsStore, sources: sources}
    r := chi.NewRouter()
    r.Use(middleware.RequestID)
    r.Use(echoRequestID)
    r.Use(httplog.RequestLogger(logger, &httplog.Options{ /* … */ }))
    r.Use(middleware.Recoverer)
    r.Get("/health", s.handleHealth)
    r.Get("/search", s.handleSearch)
    r.Post("/watchlist", s.handleAddWatchlist)
    // … r.Get/Patch/Delete/Get …
    r.NotFound(webassets.Handler().ServeHTTP)
    s.router = r
    return s
}
```
```go
// Proposed
type Option func(*serverConfig)
type serverConfig struct{ gate *authgate.Manager }

// WithAuthGate mirrors detection.With* / poller.With* / watchlist.With*.
func WithAuthGate(passphrase string, alerter authgate.Alerter) Option {
    return func(c *serverConfig) {
        if passphrase == "" { return }               // inert
        c.gate = authgate.NewManager(passphrase, alerter, /* logger injected in New */ nil)
    }
}

func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger, opts ...Option) *Server {
    var cfg serverConfig
    for _, o := range opts { o(&cfg) }
    s := &Server{db: db, watchlist: store, events: eventsStore, sources: sources, logger: logger}

    r := chi.NewRouter()
    if cfg.gate != nil {
        r.Use(middleware.RealIP)   // NEW, only when gated — before httplog so audit + access log see the client IP (D-14)
    }
    r.Use(middleware.RequestID)
    r.Use(echoRequestID)
    r.Use(httplog.RequestLogger(logger, &httplog.Options{ /* unchanged */ }))
    r.Use(middleware.Recoverer)

    r.Get("/health", s.handleHealth)   // exempt, exact path (D-03)

    if cfg.gate != nil {
        r.Post("/session", cfg.gate.HandleLogin)      // exempt: throttled + CSRF-checked inside the handler
        r.Delete("/session", cfg.gate.HandleLogout)   // exempt
        r.Group(func(pr chi.Router) {
            pr.Use(cfg.gate.Authenticate)             // 5th middleware — runs after the 4 above (chi: parent Use before group Use)
            pr.Use(cfg.gate.RequireCSRFHeader)        // non-GET without X-Requested-With → 403
            registerDataRoutes(pr, s)
        })
    } else {
        registerDataRoutes(r, s)                      // v1.2 shape, unchanged
    }
    r.NotFound(webassets.Handler().ServeHTTP)         // exempt SPA shell (D-04)
    s.router = r
    return s
}
```
> Note: `middleware.RealIP` order — chi's `RealIP` only rewrites `r.RemoteAddr` from `X-Forwarded-For`/`X-Real-IP`; it must sit *before* `httplog` and before the gate so both see the resolved IP. Placing it first is simplest. `[CITED: chi middleware docs / ARCHITECTURE.md §"Feature 2"]`

### Pattern 2: Stateless HMAC session token (D-01, D-02, D-06, D-08)

```go
// internal/authgate/session.go

const (
    sessionWindow = 30 * 24 * time.Hour   // D-06 Max-Age            (D-07: hardcoded, no env)
    renewAfter    = 15 * 24 * time.Hour    // re-issue past halfway
    absoluteCap   = 90 * 24 * time.Hour    // from first login
)

// DeriveKey — D-01. Domain-separated so a future key-version is possible
// without re-deriving; still exactly one secret.
func DeriveKey(passphrase string) [32]byte {
    return sha256.Sum256([]byte("drop-tracker/authgate/v1\x00" + passphrase))
}

type Token struct {
    IssuedAt time.Time   // first login — does NOT move on renewal (D-02, D-06)
    Expiry   time.Time
    Nonce    [16]byte     // crypto/rand — makes every minted token unique (D-17)
}

// wire format:  b64url(iat|exp|nonce)  "."  b64url(HMAC_SHA256(key, payload))
func Sign(key [32]byte, t Token) string { /* marshal payload, mac := hmac.New(sha256.New, key[:]); … */ }

// Verify returns (Token, needsRenew, ok). ok=false on any tamper / parse / expiry / absolute-cap failure.
func Verify(key [32]byte, raw string, now time.Time) (Token, bool, bool) {
    // 1. split on "."; b64-decode both halves
    // 2. recompute mac; hmac.Equal(got, want)          — constant-time
    // 3. now.Before(t.Expiry)
    // 4. now.Before(t.IssuedAt.Add(absoluteCap))         — D-06 cap
    // 5. needsRenew := now.After(t.Expiry.Add(-renewAfter))
}
```
`Authenticate` calls `Verify`; on `needsRenew` it re-signs with `IssuedAt` unchanged, `Expiry = now + sessionWindow`, fresh nonce, and writes a `Set-Cookie` on the response before calling `next`.

### Pattern 3: Cookie construction (D-09 — with the A1 adjustment)

```go
const sessionCookieName = "dt_session"   // A1: NOT "__Host-dt_session" (Chrome rejects prefixed cookies on http://localhost)

func setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
    http.SetCookie(w, &http.Cookie{
        Name:     sessionCookieName,
        Value:    value,
        Path:     "/",
        MaxAge:   maxAge,               // 30*24*3600 on mint/renew; -1 (→ Max-Age=0) on logout
        HttpOnly: true,
        Secure:   true,                 // Chrome & Firefox both send non-prefixed Secure cookies to http://localhost
        SameSite: http.SameSiteLaxMode,
    })
}
```
If discuss-phase insists on keeping `__Host-`: the fallback that is *not* a "dev toggle" is to choose the name from `r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"` — `__Host-dt_session` over HTTPS, `dt_session` over plain http. Adds a branch; the flat `dt_session` recommendation avoids it.

### Pattern 4: `rate.Limiter` per IP with bounded memory (GATE-04, D-12)

The canonical Go pattern (map keyed by IP + `sync.Mutex` + `lastSeen` + a sweeper goroutine). At this scale (one operator, low traffic) a small map is fine; the sweeper prevents unbounded growth from a scripted attack cycling source IPs.

```go
type ipLimiter struct{ lim *rate.Limiter; lastSeen time.Time }
type loginThrottle struct {
    mu   sync.Mutex
    m    map[string]*ipLimiter
    rate rate.Limit   // rate.Every(12 * time.Second)  → ~5/min sustained
    burst int          // 5
}
// getLimiter(ip): lock, upsert, touch lastSeen, return lim
// sweep(): every 10m, delete entries with lastSeen older than 3*sessionWindow… no — older than ~15min
```
Recommended constants (discretion): `rate.Every(12*time.Second)`, burst `5` → 5 immediate attempts then 1 per 12 s. Sweep every `10*time.Minute`, evict entries idle `> 15*time.Minute`. Start the sweeper in `NewManager`; stop it via a `context` or a `Close()` if the planner wants clean shutdown (optional — the process exits anyway).

**Global counter (D-12):** a `struct{ mu; count int; windowStart time.Time }`; on each *failure* increment (reset when `now - windowStart > 5*time.Minute`); when `count >= 20` within the window, fire one `alerter.Alert(ctx, "possible brute-force on this instance")` and set an `alertedAt` so alerts are rate-limited to one per ~15 min. **Recommendation: alert only, no endpoint cooldown lock** — the per-IP limiter + fixed delay already bound throughput, and a global cooldown risks locking the legitimate operator out during an attack. (CONTEXT explicitly leaves this to the planner.)

**Fixed delay (D-12):** `time.Sleep(250*time.Millisecond + time.Duration(rand.N(750))*time.Millisecond)` (`math/rand/v2`) before writing *every* `POST /session` response — success, `401`, and `429` alike.

### Pattern 5: SPA shared auth state without a provider (D-16, GATE-05)

`apiFetch` is not a React component and (in tests) the whole `~/lib/api` module is mocked — so the 401 interceptor must poke a **plain module** that both `apiFetch` and React can consume:

```ts
// web/app/lib/authStore.ts
let authed = true                       // optimistic — no boot-time check (D-16)
const listeners = new Set<() => void>()
export const authStore = {
  isAuthed: () => authed,
  markAuthenticated()   { authed = true;  listeners.forEach(l => l()) },
  markUnauthenticated() { authed = false; listeners.forEach(l => l()) },
  subscribe(l: () => void) { listeners.add(l); return () => listeners.delete(l) },
}
export function useAuthed() {
  return useSyncExternalStore(authStore.subscribe, authStore.isAuthed, () => true)
}
```
```ts
// api.ts — inside apiFetch, after `const res = await fetch(...)`
if (res.status === 401) { authStore.markUnauthenticated(); throw new ApiError(401, "unauthenticated") }
```
```tsx
// root.tsx — App()
const authed = useAuthed()
if (!authed) return <PassphraseScreen />   // full-screen, no nav tabs
return ( <div> <nav>… + <LogoutButton/></nav> <main><Outlet/></main> </div> )
```
On successful `createSession(passphrase)` the screen calls `authStore.markAuthenticated()` → `<App>` re-renders → `<Outlet/>` **remounts** → every route's `useEffect(() => refresh(), [])` fires fresh (`web/app/routes/watchlist.tsx:40-49`, `history.tsx`). **This is the D-16 "re-fetch the original data" behavior — no explicit retry-queue machinery needed.** The brief pre-first-401 flash is accepted (D-16).

`api.ts` also gains, for every non-GET call (D-15): `headers: { ...init.headers, "X-Requested-With": "drop-tracker" }`. Simplest is to always add it (harmless on GET). `DELETE` currently sends no headers (`web/app/lib/api.ts:204-206`) — this closes that gap.

### Anti-Patterns to Avoid

- **Gate as a top-level `r.Use` with in-middleware path matching** — D-05 forbids it; exemptions are structural (registered outside the group). A `strings.HasPrefix(r.URL.Path, "/health")` check is the exact bug PITFALLS #19 warns about (`/healthz-debug` leak).
- **`==` / `bytes.Equal` / `strings.EqualFold` on the passphrase** — PITFALLS #17. Always `subtle.ConstantTimeCompare(sha256(a), sha256(b))`.
- **Reading the passphrase from a query param or a `GET` login form** — PITFALLS #14. `POST` body only; the URL (with query) *is* logged by httplog.
- **In-memory session map** — PITFALLS #21. Redeploy on every merge to main would log everyone out.
- **`Secure: false` behind an `if dev {}` branch** — PITFALLS #15 and D-09. Not needed; see A1 for the correct fix.
- **Creating server-side session state before auth succeeds** — PITFALLS #16. Stateless-until-login; mint fresh on success (D-17).
- **Blocking the SPA shell / `/assets/` behind the gate** — the login form itself can't render (PITFALLS #23, D-04).

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Constant-time compare | A byte loop with `|=` | `crypto/subtle.ConstantTimeCompare` / `hmac.Equal` | Stdlib, audited, correct. `ConstantTimeCompare` returns 0 immediately on length mismatch — hashing both sides to 32 bytes first removes that leak (D-12). `[CITED: pkg.go.dev/crypto/subtle]` |
| HMAC verification | `computed == provided` on the MAC bytes | `hmac.Equal` | A plain `==` on the MAC reintroduces a timing side-channel on the signature check. |
| Client IP from `X-Forwarded-For` | Manual `strings.Split(xff, ",")` | `chi/middleware.RealIP` | Already bundled; D-14 accepted it. Manual parsing is where the spoofing bugs live (PITFALLS #22). |
| Per-client rate limiting | A custom sliding-window counter | `golang.org/x/time/rate` + a map | Already a dependency; token bucket is the right shape. Only the map+eviction is yours (Pattern 4). |
| Cookie attribute string building | `w.Header().Set("Set-Cookie", "…")` | `http.SetCookie(w, &http.Cookie{…})` | Correct escaping, `SameSite` enum, `MaxAge` semantics (`-1` → `Max-Age=0`). |
| Session library | `alexedwards/scs`, `gorilla/sessions` | Hand-rolled HMAC cookie (~30 lines) | FEATURES.md + STACK.md: one shared secret, no accounts, no server-side revoke → a session manager is strictly more moving parts for zero benefit, and its in-memory stores break under constant redeploy. |
| Discord alert delivery | A new webhook client | `internal/discord.Client` + a `noopAlerter` for the unset case | Mirror `notifier.Select` (`internal/notifier/notifier.go:147-159`) — one greppable disabled-case gate, `Send` never wraps the raw `*url.Error` (webhook path is the secret). `[VERIFIED: internal/discord/client.go:136-144]` |

**Key insight:** This domain is a minefield of subtle security bugs (timing, fixation, CSRF, XFF spoofing, cookie-flag omission) — every one of them has a one-line stdlib or already-present-dependency answer. The only genuinely new code is the ~30-line token sign/verify and the ~40-line limiter map. PITFALLS.md items 14–23 are the acceptance checklist.

## Common Pitfalls

### Pitfall 1: `__Host-` cookie prefix silently dropped by Chrome on `http://localhost`
**What goes wrong:** Operator sets `INSTANCE_PASSPHRASE` locally to test the gate, opens `http://localhost:8080` in Chrome, submits the correct passphrase, gets a `200`/`204` — but Chrome never stores the `__Host-`-prefixed cookie (scheme is not HTTPS). Next request has no cookie → `401` → passphrase screen again. Infinite loop, no error shown.
**Why it happens:** D-09 assumes "modern browsers send `Secure` cookies to localhost" — true for the `Secure` *attribute*, **false for the `__Host-`/`__Secure-` name *prefixes*** in Chrome. Firefox differs (accepts them). `[CITED: issues.chromium.org/issues/40202941, github.com/httpwg/http-extensions/issues/2605]`
**How to avoid:** Use the bare name `dt_session` with explicit `Secure; HttpOnly; SameSite=Lax; Path=/` (Pattern 3). Chrome *does* accept a non-prefixed `Secure` cookie on `http://localhost`. See A1.
**Warning signs:** manual gate test loops on the login form; `document.cookie` empty in devtools after a "successful" login; works in Firefox, not Chrome.

### Pitfall 2: `/health` swept into the gate → deploy health-poll gets `401`
**What goes wrong:** Phase 17's rollout script polls `/health`; if `/health` is inside the protected group it returns `401`, the deploy reads "unhealthy", auto-rollback fires on every deploy.
**Why it happens:** applying the gate at the router root instead of a sub-group.
**How to avoid:** `r.Get("/health", …)` registered *outside* the `Group` (Pattern 1). chi matches the registered literal `/health` — it is inherently exact, not a prefix. Unit-test all three: unauth `GET /health` → 200; unauth `GET /watchlist` → 401; unauth `GET /healthz` → 401 (falls to `NotFound` → SPA shell, which is fine).
**Warning signs:** deploy always rolls back; `curl localhost:8080/health` returns `401` with a passphrase set.

### Pitfall 3: Length leak in the constant-time compare
**What goes wrong:** `subtle.ConstantTimeCompare([]byte(submitted), []byte(expected))` returns 0 *immediately* when lengths differ — leaking passphrase length over many requests.
**How to avoid:** SHA-256 both sides first (D-12) — digests are always 32 bytes, so the compare always runs full-length. `[CITED: pkg.go.dev/crypto/subtle]`
**Warning signs:** `ConstantTimeCompare` called on raw `[]byte(passphrase)`; a fast-path `if len(a) != len(b)` before the compare.

### Pitfall 4: Session not rotated on login (fixation)
**What goes wrong:** an attacker-fixed pre-auth cookie value is "upgraded" to authenticated.
**How to avoid:** `HandleLogin` *always* calls `Sign` with a fresh `crypto/rand` nonce and `IssuedAt = now` and writes a new `Set-Cookie`, ignoring any inbound cookie entirely (D-17). With a stateless signed token a pre-auth cookie has no valid "authenticated" claim anyway, but the fresh mint makes it unconditional. Test: capture `Set-Cookie` from two logins → different values.
**Warning signs:** login handler reads the request cookie; same cookie value before and after authenticating in a test.

### Pitfall 5: Absolute cap not enforced because `IssuedAt` moves on renewal
**What goes wrong:** sliding renewal re-stamps `IssuedAt = now` each time → the 90-day cap (D-06) never triggers; an active session lives forever.
**How to avoid:** renewal copies `IssuedAt` from the verified token unchanged; only `Expiry` and `Nonce` change (Pattern 2). `Verify` rejects when `now >= IssuedAt + absoluteCap` *before* checking `Expiry`. Table-test the boundary: token with `IssuedAt = now-91d`, `Expiry = now+1d` → `ok=false`.
**Warning signs:** renewal path builds `Token{IssuedAt: now, …}`.

### Pitfall 6: Passphrase / session leaking into `httplog` output
**What goes wrong:** the secret shows up in the access log (public-repo CI logs, proxy logs).
**Current status: SAFE — verify with a test, don't re-configure.** `httplog/v3 v3.4.0` with the current `server.go` options logs neither request bodies (`LogRequestBody` nil) nor the `Cookie`/`Authorization` headers (default `LogRequestHeaders = ["Content-Type","Origin"]`, `LogResponseHeaders` default none). `[VERIFIED: go doc github.com/go-chi/httplog/v3.Options, this session]`
**How to avoid:** (1) never accept the passphrase in the URL/query (Pattern 2, Pitfall — it *does* log the URL+query); (2) never `logger.Info("…", "passphrase", …)` — audit lines log the *outcome* + IP only (D-13); (3) add a test that POSTs a known passphrase to `/session` and asserts the captured log buffer never contains it; (4) extend the Phase 01 `redactError` mindset — if any handler ever logs a decoded body, scrub it. `[VERIFIED: internal/db/migrate.go:190-193 redactError pattern]`
**Warning signs:** `r.URL.Query().Get("passphrase")`; a `slog` call with the raw submitted value; `LogRequestHeaders` widened to include `Cookie`.

### Pitfall 7: The "inert" path isn't actually inert
**What goes wrong:** adding `middleware.RealIP` / `/session` / the `Group` unconditionally changes v1.2 behavior — `make test-integration`, `pnpm test`, `docker compose up` were supposed to be untouched (success criterion 5, GATE-07).
**How to avoid:** everything gate-related is behind `if cfg.gate != nil` (Pattern 1). The ~40 existing `httpserver.New(...)` call sites pass 5 args and hit the flat-registration branch unchanged. Add exactly one gate-enabled test helper. `[VERIFIED: 40 call sites, e.g. internal/httpserver/search_test.go:93, health_test.go:57 — all 5-arg]`
**Warning signs:** a test in `internal/httpserver` or a route test in `web/` starts failing after the option is added; `docker-compose.yml` needs a passphrase to boot.

### Pitfall 8: Go `http.Client` cookiejar won't replay a `Secure` cookie to an `httptest.Server`
**What goes wrong:** an integration test does login → `client.Jar` stores the `Secure` cookie → next request over `http://127.0.0.1:port` → jar refuses to send a `Secure` cookie over plain http (Go's jar has **no** localhost exception, unlike browsers) → the "authenticated request" gets `401`, test looks broken.
**How to avoid:** in tests, extract the cookie from the login response (`resp.Cookies()`) and attach it to subsequent `http.Request`s manually — or use `httptest.NewTLSServer`. The manual-cookie approach is the common Go pattern and keeps the suite non-TLS.
**Warning signs:** gate integration test 401s right after a successful login; using `client.Jar` against an `http://` `httptest.Server`.

## Runtime State Inventory

Not a rename/refactor/migration phase — **section omitted**. (No stored data, no OS-registered state, no build artifacts affected. The one new env var `INSTANCE_PASSPHRASE` is documented in `## Environment Availability`.)

## Code Examples

### `Alerter` seam + disabled-case idiom (mirrors `notifier.Select`)
```go
// internal/authgate/alerter.go
type Alerter interface {
    Alert(ctx context.Context, message string) error
}
type noopAlerter struct{}
func (noopAlerter) Alert(context.Context, string) error { return nil }

type discordAlerter struct{ c *discord.Client }
func (d discordAlerter) Alert(ctx context.Context, msg string) error {
    return d.c.Send(ctx, discord.Embed{Title: "drop-tracker: possible brute-force", Description: msg, Color: 0xE53E3E})
}

// SelectAlerter mirrors internal/notifier/notifier.go:147-159's gate.
func SelectAlerter(webhookURL string, logger *slog.Logger) Alerter {
    if webhookURL == "" {
        logger.Info("authgate brute-force alerting disabled: DISCORD_WEBHOOK_URL not set")
        return noopAlerter{}
    }
    return discordAlerter{c: discord.NewClient(webhookURL, nil)}
}
```
> `main.go` wiring: `httpserver.WithAuthGate(cfg.InstancePassphrase, authgate.SelectAlerter(cfg.DiscordWebhookURL, logger))`.
> `[VERIFIED: internal/notifier/notifier.go:147-159 Select idiom; internal/discord/client.go:98 NewClient(url, nil)]`

### Config field (follows the grouped-by-phase convention)
```go
// internal/config/config.go — add to the Config struct
// Phase 14 — instance passphrase gate (GATE-01..07). Optional: empty = gate
// fully disabled, every route behaves exactly as v1.2 (GATE-07). Never
// `notEmpty`/`required`; no Load() validation beyond the boot-time weak
// heuristic WARN, which lives in cmd/server/main.go (WARN, never fail — D-11).
InstancePassphrase string `env:"INSTANCE_PASSPHRASE"`
```
> `[VERIFIED: internal/config/config.go:18-57 — fields grouped by introducing phase, optional fields never notEmpty, manual validation in Load() after env.Parse]`

### Boot-time weak WARN (main.go, right after `config.Load()`)
```go
// cmd/server/main.go — after `logger := logging.New(cfg)` (config → logging → migrations order is D-09)
if reason, weak := authgate.IsWeakPassphrase(cfg.InstancePassphrase); weak {
    logger.Warn("INSTANCE_PASSPHRASE looks weak; the instance gate is only as strong as this value", "reason", reason)
}
```
```go
// internal/authgate/weak.go — heuristic (discretion). Never logs the value.
var knownDefaults = []string{"changeme", "change-me", "password", "secret", "admin",
    "drop-tracker", "droptracker", "letmein", "instance-passphrase"}
func IsWeakPassphrase(p string) (reason string, weak bool) {
    if p == "" { return "", false }                                  // unset = gate off, not "weak"
    if utf8.RuneCountInString(p) < 16 { return "shorter than 16 characters", true }
    lower := strings.ToLower(strings.TrimSpace(p))
    for _, d := range knownDefaults { if lower == d { return "matches a known default value", true } }
    return "", false
}
```
> `[VERIFIED: cmd/server/main.go:88-98 boot order — config.Load then logging.New; WARN belongs here per CONTEXT code_context]`

### RTL test for the 401 → passphrase → re-fetch flow
```tsx
// web/app/lib/authStore.test.ts — direct unit test (module is not mocked here)
import { authStore } from "./authStore"
it("notifies subscribers on state change", () => {
  const seen: boolean[] = []
  const unsub = authStore.subscribe(() => seen.push(authStore.isAuthed()))
  authStore.markUnauthenticated(); authStore.markAuthenticated()
  unsub()
  expect(seen).toEqual([false, true])
})
```
```tsx
// web/app/lib/api.test.ts — real apiFetch, mocked global fetch (do NOT vi.mock("~/lib/api") here)
import { authStore } from "./authStore"
import { listWatchlist } from "./api"
it("flips authStore to unauthenticated on a 401", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(
    new Response(JSON.stringify({ error: "unauthenticated" }), { status: 401 })))
  authStore.markAuthenticated()
  await expect(listWatchlist()).rejects.toMatchObject({ status: 401 })
  expect(authStore.isAuthed()).toBe(false)
})
```
```tsx
// web/app/components/auth/PassphraseScreen.test.tsx — RTL, mock ~/lib/api (createSession)
it("calls createSession and marks authenticated on submit of the correct passphrase", async () => {
  mockCreateSession.mockResolvedValue(undefined)
  render(<PassphraseScreen />)
  await userEvent.type(screen.getByLabelText(/passphrase/i), "correct horse battery staple")
  await userEvent.click(screen.getByRole("button", { name: /unlock|sign in|enter/i }))
  expect(mockCreateSession).toHaveBeenCalledWith("correct horse battery staple")
  expect(authStore.isAuthed()).toBe(true)
})
```

### Go table-driven session test (matches the repo's individual-named-test lean, with `t.Run` subcases)
```go
// internal/authgate/session_test.go
func TestVerify(t *testing.T) {
    key := authgate.DeriveKey("s3cret-passphrase-value")
    now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
    fresh := authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)}
    cases := []struct {
        name         string
        tok          authgate.Token
        mutate       func(string) string
        at           time.Time
        wantOK       bool
        wantRenew    bool
    }{
        {"round trips", fresh, nil, now.Add(time.Hour), true, false},
        {"renew past halfway", fresh, nil, now.Add(20 * 24 * time.Hour), true, true},
        {"expired", fresh, nil, now.Add(31 * 24 * time.Hour), false, false},
        {"absolute cap", authgate.Token{IssuedAt: now.Add(-91 * 24 * time.Hour), Expiry: now.Add(24 * time.Hour)}, nil, now, false, false},
        {"tampered mac", fresh, func(s string) string { return s[:len(s)-2] + "xx" }, now.Add(time.Hour), false, false},
        {"wrong key", fresh, nil, now.Add(time.Hour), false, false}, // verify with a different key
    }
    // …
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `gorilla/sessions` as the default Go session store | Hand-rolled HMAC cookie for single-secret gates; `alexedwards/scs` for real multi-user | Gorilla toolkit archived 2022, partially revived | STACK.md "What NOT to Use" — no reason to pull it in here. |
| `SameSite` unset / `None` | `SameSite=Lax` default; custom-header double-check for SPAs | Browsers defaulted to `Lax` ~2020 | D-15's approach (Lax + `X-Requested-With`) is current best practice for a same-origin SPA with no CORS. |
| `math/rand` (v1) global source | `math/rand/v2` | Go 1.22 | Use `math/rand/v2` for the login delay jitter; `crypto/rand` for the nonce. |
| `crypto/subtle` raw-string compare | Hash-then-compare to hide length | long-standing ASVS guidance | D-12 already specifies this. |

**Deprecated/outdated:**
- STACK.md §"Feature 2" names `GATE_PASSPHRASE` + `GATE_SIGNING_KEY` and a `dt_gate` cookie — **superseded by CONTEXT.md D-01/D-07/D-09**: one var `INSTANCE_PASSPHRASE`, derived key, cookie name is planner's discretion. Follow CONTEXT, not STACK, where they differ.
- `.planning/codebase/TESTING.md` (dated 2026-08-12) says "No test framework configured in `web/package.json`" — **stale**; Phase 08 added Vitest 4 + RTL (`web/vitest.config.ts`, `web/vitest.setup.ts`, 9 `*.test.tsx` files). Use the Vitest setup.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | D-09's `__Host-` prefix + "Secure on localhost, no dev toggle" is unsound in Chrome (rejects prefixed cookies on `http://localhost`); **recommend the bare name `dt_session` with explicit attributes**. Cross-referenced Chromium tracker + httpwg issue + MDN via WebSearch (LOW-tier source), not tested against a live Chrome this session. | Summary, §Pattern 3, §Pitfall 1, Discretion | If planner keeps `__Host-`: operators can only manually test the *enabled* gate locally in Firefox or over HTTPS. Not a production risk (prod is HTTPS via Phase 17). Needs a one-line discuss-phase confirmation. |
| A2 | Recommended rate-limit constants (5 burst / `rate.Every(12s)` / 5-min global window / threshold 20 / one alert per 15 min / 250ms–1s delay) are engineering judgement, not derived from a stated requirement. CONTEXT explicitly leaves these to the planner. | §Pattern 4, D-12 | Too tight → operator locked out during their own testing; too loose → weaker brute-force bound. All are single greppable constants, trivially tunable. |
| A3 | Recommendation to **alert only, no global endpoint cooldown lock** (D-12 leaves this open). | §Pattern 4 | If a cooldown is wanted later it's an additive change to the global counter; no rework of the token or middleware. |
| A4 | Go's `net/http/cookiejar` has no `localhost` exception for `Secure` cookies (unlike browsers), so integration tests must attach the cookie manually or use TLS. Based on stdlib knowledge, not verified against Go 1.26 source this session. | §Pitfall 8, §Validation | If wrong, the manual-cookie test approach still works — it's strictly the safer pattern regardless. |
| A5 | `<Outlet/>` remount on the `<App>` auth-state flip re-runs each route's mount-effect fetch, satisfying D-16 "re-fetch original data" with no retry queue. Based on reading `watchlist.tsx`/`history.tsx` (both fetch in `useEffect([])`) + React Router v7 behavior. | §Pattern 5, GATE-05 | If a route ever fetches in a loader instead of `useEffect`, that route needs explicit revalidation. Currently none do. |

**If this table looks long:** A1 is the only one that could change a locked decision; the rest are tuning constants and test-mechanics notes the planner can adopt as-is.

## Open Questions

1. **Cookie name / `__Host-` prefix (A1).**
   - What we know: D-09 wants `__Host-`; Chrome rejects it on `http://localhost`; the gate is inert locally by default so impact is narrow.
   - Recommendation: drop the prefix → `dt_session` with explicit `Secure; HttpOnly; SameSite=Lax; Path=/`. Confirm with a one-line discuss note; otherwise plan the TLS-conditional name (Pattern 3 fallback).

2. **`401` body shape.**
   - Recommendation: `{"error":"unauthenticated"}` for the gate, `{"error":"invalid passphrase"}` for a wrong login, `{"error":"too many attempts"}` for `429`. Matches the existing `{"error": "..."}` convention that `ApiError` in `web/app/lib/api.ts:95-103` already parses.

3. **Login success status code.**
   - Recommendation: `204 No Content` + `Set-Cookie` (no body needed; SPA just flips state). `DELETE /session` → `204` + `Set-Cookie … Max-Age=0`. `apiFetch` already handles `204` (`web/app/lib/api.ts:114-116`).

4. **`Referrer-Policy` header (Pitfall 14 hardening).**
   - Recommendation: set `Referrer-Policy: no-referrer` globally (cheap, one `middleware` or a header set in `New`), not just on `/session`. CONTEXT marks the wiring as discretion.

5. **Sweeper goroutine lifecycle.**
   - Recommendation: start it in `NewManager`; give `Manager` a `Close()` that stops the ticker, called from a `defer` in `main.go` for tidiness. Not load-bearing (process exit reclaims it) — planner's call whether to bother.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build | ✓ | 1.26 (`go.mod`) | — |
| `golang.org/x/time/rate` | GATE-04 throttle | ✓ | v0.15.0 | — |
| `github.com/go-chi/chi/v5` (+ `middleware.RealIP`) | gate group, IP resolution | ✓ | v5.3.1 | — |
| `github.com/go-chi/httplog/v3` | audit log context | ✓ | v3.4.0 | — |
| `internal/discord` | D-12 brute-force alert | ✓ | in-repo | `noopAlerter` when `DISCORD_WEBHOOK_URL` unset (mirrors `notifier.NoOp`) |
| Vitest 4 + RTL + user-event + jest-dom | frontend tests | ✓ | in `web/package.json` (Phase 08) | — |
| `INSTANCE_PASSPHRASE` env var | the whole feature | operator-supplied | — | **unset = gate inert (GATE-07)** — this is the designed default for local dev, docker-compose, CI |
| Reverse proxy terminating TLS + unpublished container port | D-14 `RealIP` safety, `Secure` cookie in prod | ✗ (Phase 17) | — | Phase 14 ships a load-bearing code comment + runbook note only; ordering guarantees Phase 14 merges before Phase 17 exposes the instance |

**Missing dependencies with no fallback:** none — the phase is code-only against present dependencies.
**Missing dependencies with fallback:** the reverse proxy (Phase 17) — Phase 14 documents the invariant and does not depend on it at runtime.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go stdlib `testing` (no assertion lib); `httptest` for handlers `[VERIFIED: .planning/codebase/TESTING.md]` |
| Backend config file | none |
| Backend quick run | `make test-short` (`go test ./... -short -race -count=1`) |
| Backend full suite | `make test-integration` (`go test ./... -race -count=1 -p 1`, needs Docker Postgres on :5433) |
| Frontend framework | Vitest 4 (`v8` coverage) + `@testing-library/react` + `user-event` + `jest-dom` `[VERIFIED: web/vitest.config.ts]` |
| Frontend config file | `web/vitest.config.ts`, `web/vitest.setup.ts` |
| Frontend run command | `pnpm test` (in `web/`) — single unchanged `vitest run`; coverage + 70% gate on all four axes |

> Note: on this dev machine `-race` is documented as unusable (ThreadSanitizer allocation failure); substitute plain `go test` locally, `-race` runs in CI. `[VERIFIED: STATE.md Phase 11.1-04 entry]`

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| GATE-01 | unauth `GET /watchlist|/search|/events` → 401; unauth `GET /health` → 200; unauth `GET /healthz` → not-401 | unit (httptest) | `go test ./internal/authgate/ ./internal/httpserver/ -run Gate -short` | ❌ Wave 0 |
| GATE-01 | gate not configured → all 7 routes behave as v1.2 (no 401 anywhere) | unit | `go test ./internal/httpserver/ -run Inert -short` | ❌ Wave 0 |
| GATE-02 | `Sign`→`Verify` round-trip; token valid after a simulated "restart" (new `Manager`, same passphrase) | unit | `go test ./internal/authgate/ -run TestVerify -short` | ❌ Wave 0 |
| GATE-03 | `Set-Cookie` on login carries `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, bounded `Max-Age`; tampered cookie → 401; wrong-key cookie → 401 | unit (httptest, inspect `resp.Cookies()`) | `go test ./internal/authgate/ -run Cookie -short` | ❌ Wave 0 |
| GATE-03 | passphrase compare is constant-time (structural: assert `subtle.ConstantTimeCompare`/`hmac.Equal` on 32-byte digests; no `==` on the secret) | unit + grep | `go test ./internal/authgate/ -run Compare -short` + `! grep -n 'InstancePassphrase ==' internal/` | ❌ Wave 0 |
| GATE-04 | 6th login attempt from one IP within the window → 429; different IP unaffected; limiter map evicts idle entries | unit (fake clock or injected `rate.Limit`) | `go test ./internal/authgate/ -run Throttle -short` | ❌ Wave 0 |
| GATE-04 | fixed delay applied on every `/session` response (success/401/429) | unit (assert min elapsed with a shrunk delay `var`) | `go test ./internal/authgate/ -run Delay -short` | ❌ Wave 0 |
| GATE-04 | global counter past threshold → `Alerter.Alert` called once; alert re-armed after cooldown | unit (fake `Alerter`) | `go test ./internal/authgate/ -run GlobalAlert -short` | ❌ Wave 0 |
| GATE-05 | `apiFetch` 401 → `authStore` flips unauthenticated + throws `ApiError(401)` | unit (mock global `fetch`) | `pnpm test api` | ❌ Wave 0 (`web/app/lib/api.test.ts`) |
| GATE-05 | `<App>` renders `<PassphraseScreen>` when `!authed`, routed page when authed | RTL | `pnpm test root` | ⚠️ extend `web/app/root.test.tsx` |
| GATE-05 | `<PassphraseScreen>` submit → `createSession` + `authStore.markAuthenticated`; wrong passphrase surfaces an error, does not flip state | RTL | `pnpm test PassphraseScreen` | ❌ Wave 0 |
| GATE-05 | after login, a route re-fetches (mount effect fires on remount) | RTL (mock `~/lib/api`, assert `listWatchlist` called again post-auth) | `pnpm test watchlist` | ⚠️ extend `web/app/routes/watchlist.test.tsx` |
| GATE-06 | `DELETE /session` → 204 + `Set-Cookie … Max-Age=0`; logout button calls `deleteSession` + `authStore.markUnauthenticated` | unit + RTL | `go test ./internal/authgate/ -run Logout -short` ; `pnpm test root` | ❌ Wave 0 |
| GATE-07 | `make test-integration` + `pnpm test` both green with no passphrase set; `docker compose up` unchanged | full suite | `make test-integration && (cd web && pnpm test)` | existing suites must stay green |
| D-13 | `POST /session` with a known passphrase → captured `slog` buffer never contains it; success/fail/logout each emit one audit line with source IP | unit (capture `slog` via a buffer handler) | `go test ./internal/authgate/ -run Audit -short` | ❌ Wave 0 |
| D-17 | two consecutive logins produce different cookie values | unit | `go test ./internal/authgate/ -run Rotation -short` | ❌ Wave 0 |
| D-06 | request past halfway → response carries a fresh `Set-Cookie` with a later `Expiry`, same `IssuedAt`; past absolute cap → 401 even with unexpired `Expiry` | unit (injected clock) | `go test ./internal/authgate/ -run Renew -short` | ❌ Wave 0 |
| D-15 | gated `POST/PATCH/DELETE` without `X-Requested-With` → 403; with it → passes to handler; `api.ts` sends it on every non-GET | unit + RTL | `go test ./internal/authgate/ -run CSRF -short` ; `pnpm test api` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `make test-short` + (for `web/` changes) `pnpm test` for the touched files.
- **Per wave merge:** `make test-integration` + `cd web && pnpm test` (full, with coverage gate).
- **Phase gate:** full backend + frontend suites green, coverage gates (80% backend / 70% frontend, all four frontend axes) not regressed, before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/authgate/session_test.go` — GATE-02, GATE-03 (compare), D-06, D-17
- [ ] `internal/authgate/gate_test.go` — GATE-01, GATE-03 (cookie flags), D-06 (renewal), D-15 (CSRF)
- [ ] `internal/authgate/login_test.go` — GATE-04, D-12 (delay, global alert), D-13 (audit), D-17 (rotation)
- [ ] `internal/authgate/weak_test.go` — D-11 heuristic (short, known-default, empty→not-weak)
- [ ] `internal/httpserver/server_test.go` — a `newGatedServer(t, passphrase)` helper; an `Inert` test proving the 5-arg path is unchanged
- [ ] `web/app/lib/authStore.test.ts` — pub/sub
- [ ] `web/app/lib/api.test.ts` — **new file**, must NOT `vi.mock("~/lib/api")`; mocks global `fetch`; covers the 401 interceptor + `X-Requested-With`
- [ ] `web/app/components/auth/PassphraseScreen.test.tsx` — submit / error / state-flip
- [ ] extend `web/app/root.test.tsx` — `<App>` gate branch + logout button
- [ ] extend `web/app/routes/watchlist.test.tsx` — re-fetch after auth
- [ ] Framework installs: **none** — Go `testing` and Vitest+RTL are both present.

## Security Domain

`security_enforcement` is enabled (no `.planning/config.json` override found; this is the milestone's security-critical phase).

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | yes | Single shared operator secret; per-IP rate limit (`x/time/rate`) + global counter + fixed delay (D-12); constant-time compare on SHA-256 digests; boot-time weak-value WARN (D-11). No account lockout concept (no accounts) — the rate limit is its analog. |
| V3 Session Management | yes | HMAC-SHA256 stateless token; `HttpOnly` + `Secure` + `SameSite=Lax` + `Path=/` cookie; bounded `Max-Age` (30d) + absolute cap (90d, D-06); session rotation on authentication (D-17); logout clears the cookie (D-10). |
| V4 Access Control | yes | Default-deny: the protected chi `Group` gates every data route; exemptions are an explicit allowlist registered *outside* the group (`/health` exact, `/session`, SPA shell) — no path-string matching (D-05). |
| V5 Input Validation | yes | Passphrase arrives only in a `POST` JSON body (never URL/query — Pitfall 14); cap the login body size (reuse the `maxAddWatchlistBodyBytes`-style `http.MaxBytesReader` idiom); reject non-GET gated requests lacking `X-Requested-With` (D-15). |
| V6 Cryptography | yes | Stdlib only: `crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `crypto/rand` (nonce). No hand-rolled primitives, no JWT library, no KDF (env-var leak = passphrase leak anyway, so Argon2 buys nothing — FEATURES.md anti-feature). |
| V7 Error Handling & Logging | yes | Audit lines (login success/fail/logout + source IP) via existing `slog` (D-13); passphrase never logged — `httplog` already logs no bodies and not the `Cookie` header (Pitfall 6, VERIFIED); extend the Phase 01 `redactError` mindset; fixed operator-authored `{"error": "..."}` responses, never raw error text (`.planning/codebase/CONVENTIONS.md`). |
| V13 API & Web Service | yes | CSRF: `SameSite=Lax` + required custom header on non-GET (D-15); CORS entirely absent (same-origin SPA); `Referrer-Policy: no-referrer` (Open Q4). |
| V14 Configuration | yes | One new env var, optional, never `required`; secret via env only (CLAUDE.md); `.env.example` documents a 24+ char recommendation and must **not** ship a usable default (`.env.example` value is on the weak denylist). |

### Known Threat Patterns for {Go/chi API + embedded React SPA + shared passphrase}

| Pattern | STRIDE | Standard Mitigation | Pitfall ref |
|---------|--------|---------------------|-------------|
| Timing attack on `passphrase ==` | Information Disclosure | `subtle.ConstantTimeCompare(sha256(a), sha256(b))` | #17 |
| Passphrase in URL/query → logs, Referer, history | Information Disclosure | `POST` body only; `Referrer-Policy: no-referrer`; httplog logs no body | #14 |
| Session fixation (pre-auth cookie upgraded) | Spoofing | Mint a brand-new token on every successful login, ignore inbound cookie (D-17) | #16 |
| Cookie theft via XSS | Spoofing / Elevation | `HttpOnly` (+ existing zero-`dangerouslySetInnerHTML` posture from Phase 06) | #15 |
| Cookie sniffed over plaintext | Information Disclosure | `Secure`; prod is HTTPS-only (Phase 17); `SameSite=Lax` | #15, #22 |
| CSRF on `POST /watchlist` / `POST /session` | Tampering / Spoofing | `SameSite=Lax` + `X-Requested-With` required on non-GET; no CORS | #20 |
| Brute-force of the one shared secret | Elevation of Privilege | per-IP `rate.Limiter` → 429; global counter → Discord alert; fixed 250ms–1s delay | #18 |
| `X-Forwarded-For` spoofing to bypass the throttle | Tampering | `chi/middleware.RealIP` **+ the unpublished-container-port invariant** (load-bearing comment + runbook note, D-14; Phase 17 enforces) | #22 |
| `/health` behind the gate → deploy auto-rollback loop | Denial of Service (self-inflicted) | `/health` registered outside the group, exact path | #19, #2 |
| Verbose `/health` as recon when public | Information Disclosure | keep `/health` body to `{"status","db"}` — already minimal (`internal/httpserver/health.go:23-26`, VERIFIED); no version/DSN | #19 |
| Sessions wiped every redeploy → users hammer login | (availability / UX) | stateless signed cookie, survives restart (D-08) | #21 |
| SPA blank/broken on 401 instead of a login prompt | (UX / availability) | global `apiFetch` 401 interceptor → `<PassphraseScreen>` (D-16) | #23 |
| `__Host-` cookie silently dropped (Chrome/localhost) | (availability, local only) | bare `dt_session` name + explicit attributes (A1) | #1 |

## Sources

### Primary (HIGH confidence)
- `go.mod` (read this session) — `go 1.26`; `chi/v5 v5.3.1`; `httplog/v3 v3.4.0`; `golang.org/x/time v0.15.0`.
- `go doc github.com/go-chi/httplog/v3.Options` (run this session) — default logged headers/bodies.
- In-repo source, read this session: `internal/httpserver/server.go` (middleware chain, route registration, `New` signature), `internal/httpserver/health.go` (minimal health payload), `internal/config/config.go` (grouped-by-phase config convention, manual `Load()` validation), `cmd/server/main.go` (boot order, functional-option wiring), `internal/discord/client.go` (`NewClient`, no-raw-error-wrap), `internal/notifier/notifier.go` (`Select` / `NoOp` disabled-case idiom), `internal/db/migrate.go` (`redactError` pattern), `internal/poller/poller.go` (`Option`/`With*` shape), `web/app/lib/api.ts` (`apiFetch`, `ApiError`, 204 handling), `web/app/root.tsx` + `routes.ts` (SPA structure), `web/app/routes/watchlist.tsx` / `history.tsx` (mount-effect fetch), `web/vitest.config.ts` / `vitest.setup.ts` / `web/app/lib/test/routeStub.tsx` / `web/app/root.test.tsx` / `web/app/routes/watchlist.test.tsx` (test conventions), `.planning/codebase/CONVENTIONS.md` + `TESTING.md`.
- `.planning/research/ARCHITECTURE.md` §"Feature 2", `FEATURES.md` §"Feature 2", `STACK.md` §"Feature 2", `PITFALLS.md` items 14–23 — the v1.3 milestone research this phase builds on.
- `.planning/phases/14-instance-passphrase-gate/14-CONTEXT.md` — D-01…D-17, canonical refs.

### Secondary (MEDIUM confidence)
- `pkg.go.dev/crypto/subtle` `ConstantTimeCompare` semantics (length-mismatch early return) — stdlib canonical behavior, from training knowledge, not re-fetched this session.

### Tertiary (LOW confidence — flagged as A1)
- WebSearch (2026-08-27): "`__Host-` cookie prefix Secure http localhost" — surfaced `issues.chromium.org/issues/40202941` (Chrome rejects `__Host-` on `http://localhost`), `github.com/httpwg/http-extensions/issues/2605` (inconsistent browser behavior), MDN `Set-Cookie`. Not verified against a live browser this session; drives recommendation A1.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — nothing to install; every dependency verified in `go.mod` this session.
- Architecture / wiring: HIGH — every integration point read this session; the functional-option + `Group` pattern is already used verbatim in `poller`/`detection`/`watchlist` and spelled out in ARCHITECTURE.md.
- Pitfalls: HIGH — PITFALLS.md 14–23 is the checklist; each maps to a concrete stdlib control; httplog safety verified via `go doc`.
- Cookie prefix (A1): MEDIUM — the Chrome/`__Host-`/localhost behavior is well-attested across three independent sources but not live-tested; recommendation is conservative and low-cost.
- Rate-limit constants (A2): LOW — pure engineering judgement, explicitly the planner's to set; all single greppable literals.

**Research date:** 2026-08-27
**Valid until:** ~2026-09-26 (stable domain — stdlib crypto + a pinned dependency set; the only moving part is browser cookie-prefix behavior, which trends *toward* the recommendation, not away).
