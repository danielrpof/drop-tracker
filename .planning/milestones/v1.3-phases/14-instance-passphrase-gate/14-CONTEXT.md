# Phase 14: Instance Passphrase Gate - Context

**Gathered:** 2026-08-27
**Status:** Ready for planning

<domain>
## Phase Boundary

A single shared operator-set passphrase (`INSTANCE_PASSPHRASE` env var) puts drop-tracker's data API behind a signed, stateless session cookie so a public deployment only exposes the watchlist, search proxy, and event history to someone who knows the passphrase. Delivered as:

- A Go `internal/authgate` package (HMAC sign/verify + gate middleware) and a protected chi route `Group` in `internal/httpserver/server.go`
- `POST` / `DELETE` session endpoints (login / logout), registered outside the gated group
- Per-IP login throttling + a bounded login-concurrency semaphore + a global brute-force counter that alerts to Discord (D-12)
- The React SPA's global 401 interception → passphrase screen, plus a `gateActive`-gated logout control (D-18)

**Inert when unconfigured:** with `INSTANCE_PASSPHRASE` unset, every route behaves exactly as it did in v1.2 — `make test-integration`, `pnpm test`, and `docker compose up` all pass untouched, and none of the ~8 `httpserver.New(...)` test call sites need a passphrase.

**In scope:** GATE-01 … GATE-07 (see `.planning/REQUIREMENTS.md`). Joint backend + frontend slice — splitting the server contract from the SPA handling would leave a half-phase that renders a broken app.

**Out of scope:** multi-user auth / accounts / OAuth / RBAC (rejected in PROJECT.md); server-side session store; signing-key rotation without logging everyone out (future GATE-08); the reverse proxy / TLS termination and the "don't publish the container port" topology enforcement (Phase 17).

</domain>

<decisions>
## Implementation Decisions

### Session signing key
- **D-01:** The HMAC-SHA256 signing key is **derived from the passphrase**: `key = SHA256(INSTANCE_PASSPHRASE)`. Exactly one new secret exists. Rotating `INSTANCE_PASSPHRASE` changes the derived key and therefore invalidates every existing session — rotation *is* revoke-all. No separate `SESSION_SECRET`, no key-version byte. — **Reversibility:** costly — moving to a separate secret later means adding + provisioning a new env var in GitHub Actions and the VPS `.env`, and every session issued under the derived key is invalidated at the switchover.
- **D-02:** Session cookie payload is a signed "authenticated until T" — an expiry timestamp plus a random nonce, HMAC'd with the derived key. No PII, no user identity (there are no users). Issued-at / first-login timestamp is carried in the payload and does **not** move on renewal (needed for the absolute cap in D-06).

### What is gated vs. public
- **D-03:** The gate is a **pure API concern**. The data API (`/search`, `/watchlist` incl. `POST`/`PATCH`/`DELETE`, `/events`) is gated. Exempt (registered outside the protected `Group`): `GET /health` (exact path only, not a prefix), the session-login endpoint, and the SPA `NotFound` fallback (`webassets.Handler` — `index.html` + hashed JS/CSS under `/assets/`).
- **D-04:** The **static SPA shell serves publicly**. An unauthenticated visitor gets the bundle, the SPA boots, calls the API, receives `401`, and renders the passphrase screen. Rationale: the bundle is open client code with no secrets; the watchlist data and Discord webhook config only ever move through the gated API; keeps a single static-serving path in Go. The stricter "gate everything but a Go-served `/login`" option was considered and rejected.
- **D-05:** Gate middleware is applied to the route `Group`, **after** all four existing middlewares (`RequestID` → `echoRequestID` → `httplog` → `Recoverer`), so rejected `401`s are logged with a request id. It is **not** a fifth top-level `r.Use`. No in-middleware URL-path string matching — exemptions are "registered outside the group." Wired via a `httpserver.WithAuthGate(passphrase)` functional option matching the existing `detection.With*` / `poller.With*` / `watchlist.With*` idiom, so omitting it keeps every current test call site unchanged.

### Session lifetime & renewal
- **D-06:** **Sliding renewal.** Cookie `Max-Age` 30 days; when a request arrives past the halfway mark (cookie older than ~15 days), re-issue a fresh 30-day cookie. Absolute cap ~90 days (3× the window) measured **from the authentication that minted the session**, not from a lifetime "first login ever": the `IssuedAt` stamp is fixed across sliding renewals, so an idle-but-still-renewing session dies at 90 days — but a fresh passphrase entry (D-17) mints a new token with a new `IssuedAt` and starts a new 90-day cap. There is deliberately no hard ceiling across re-authentications. Active devices never get surprise-logged-out; abandoned sessions still expire.
- **D-07:** The 30-day window and 90-day cap are **hardcoded constants** in `internal/authgate`, not an env var. `INSTANCE_PASSPHRASE` stays the *only* new configuration variable this phase adds. No `SESSION_TTL`.
- **D-08:** A container restart / redeploy does **not** log anyone out (stateless cookie — this is the whole point of D-01/D-02 over an in-memory session map).
- **D-09:** Cookie attributes: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`. Use the `__Host-` name prefix (forces `Secure` + `Path=/` + no `Domain`). `Secure` is kept on even for local `http://localhost` (modern browsers send `Secure` cookies to localhost) — no dev toggle.

### Logout
- **D-10:** Logout is **client-local only** and that is accepted. `DELETE /session` responds with `Set-Cookie … Max-Age=0`, clearing the cookie on that browser. A cookie already copied elsewhere stays valid until its TTL expires. The revoke-all lever at this scale is rotating `INSTANCE_PASSPHRASE` (D-01). No server-side revocation list, no key-version bump. **Because this is the *only* revoke-all path and logout does not provide one, the Phase 17 provisioning runbook must document it explicitly:** "rotate `INSTANCE_PASSPHRASE` and redeploy — the derived key (D-01) changes and every session is invalidated." (Earlier this was offered and declined for minimalism; a leaked-cookie recovery path that lives only in an agent's head is not acceptable for a public instance.)

### Passphrase strength
- **D-11:** **Warn only** — no boot-time strength enforcement. If `INSTANCE_PASSPHRASE` is set and looks weak (short, or a known-default like `changeme` / the `.env.example` value), log a `WARN` at boot and start normally. The process never refuses to start over a passphrase-policy edge case. `.env.example` should still recommend a 24+ char random value in its comment. (Fail-closed and minimum-length-only were both considered and rejected.)

### Brute-force defense
- **D-12:** **Per-IP throttle + global counter + Discord alert + a concurrency bound.**
  - Per-IP: `golang.org/x/time/rate` keyed on client IP, ~5 attempts/min then `429`.
  - Fixed delay: a fixed ~250ms–1s delay is applied to every login response **that actually runs the passphrase comparison** — the `204` success and the wrong-passphrase `401` — to blunt timing analysis and distributed guessing. The fast `429` throttle rejection is **not** delayed: it never reaches the comparison the delay exists to mask, and holding a goroutine + connection open for up to a second on a rejected request turns a distributed flood into a resource-exhaustion amplifier. The `503` over-capacity path (below) and a malformed-body `400` are also undelayed.
  - Concurrency bound: the login handler acquires a slot from a process-wide buffered channel (`maxConcurrentLogins`, ~32) before the per-IP check; if the acquire would block, respond `503` immediately. This caps how many goroutines can be parked in the fixed delay at once — the same distributed-guess flood the global counter *detects* is now also *contained*.
  - Global: a process-wide failed-attempt counter; past a threshold in a window it fires a Discord alert through the existing `internal/discord` webhook path ("possible brute-force on <instance>"). Alert-only — no global endpoint cooldown lock (a global lock would lock the legitimate operator out during an attack; the per-IP limiter, the delay, and the concurrency bound carry the throughput ceiling). The alert dispatch is fire-and-forget on its own goroutine with a bounded timeout — never in the login response path. Reuses the notifier sink (on-brand for the DevOps portfolio). — **Reversibility:** reversible — a small self-contained counter + one wiring point.
  - Passphrase comparison is `crypto/subtle.ConstantTimeCompare` on equal-length digests (SHA-256 each side first, so length isn't observable) regardless of throttle state.
- **D-13:** Auth events (login success / failure / logout, with source IP) are emitted as structured `slog` lines — "who/when" visibility, consistent with D-12's observable posture. The passphrase itself must never reach a log line (extends the Phase 01 slog-redaction pattern; `httplog` logs metadata not bodies — verify no handler logs the decoded request body).

### Client IP resolution
- **D-14:** Client IP for the per-IP throttle and the audit log comes from `chi/middleware.RealIP` (reads `X-Forwarded-For` / `X-Real-IP`) **only when a second new env var, `TRUST_PROXY_HEADERS`, is truthy** (default `false`). With it unset — every context that exists before Phase 17: local dev, docker-compose, CI, and any manual pre-proxy deploy — `RealIP` is not wired and both the throttle and the audit log key on `r.RemoteAddr` (the direct peer), which a client cannot spoof. Phase 17's runbook sets `TRUST_PROXY_HEADERS=true` on the VPS *together with* the unpublished-container-port topology (the app reachable only through the proxy). This deliberately breaks D-07's "exactly one new config variable" goal: a load-bearing code comment is not an enforcement mechanism, and "accept the XFF-spoof risk" is only sound once the compensating control (the proxy topology) actually exists. A load-bearing comment still records the topology requirement. The "parse only the proxy's appended hop" and "`RemoteAddr` only, always" alternatives were considered and rejected.

### CSRF
- **D-15:** `SameSite=Lax` **plus** a required custom header on all non-GET `/api` requests. The gate (or an adjacent middleware) rejects any non-GET request to a gated route that lacks a custom header (e.g. `X-Requested-With`), which the `web/app/lib/api.ts` fetch wrapper always sets on `POST`/`PATCH`/`DELETE`. A cross-site attacker cannot set custom headers without a CORS preflight the server denies. CORS stays entirely absent (same-origin SPA served from the same binary). Applies to `POST /session` too (login-CSRF), alongside session rotation on successful auth.

### 401 handling in the SPA
- **D-16:** **Global 401 interceptor.** `apiFetch` (the single fetch funnel in `web/app/lib/api.ts`) catches any `401`, flips a shared auth state, and the app renders the passphrase screen instead of the routed page. No boot-time `GET /session` status check — the brief loading flash before the first `401` is acceptable, and mid-session expiry is handled for free by the same path. On successful login the SPA re-fetches the original data.

### Session rotation on auth
- **D-17:** On successful passphrase verification, issue a **brand-new** session cookie (new nonce, fresh expiry). Never carry any pre-auth identifier across the auth boundary — with stateless signed cookies a pre-auth token simply has no valid "authenticated" claim, but the login handler still always mints a fresh token rather than "upgrading" anything.

### Log out control visibility (resolves the UI-SPEC open item)
- **D-18:** The SPA auth store carries a second boolean, `gateActive`, initialised `false` and set `true` the first time the app observes a `401` **or** completes a login in this browser session. The **Log out** control renders only when `gateActive` is true, so an ungated instance (no `/session` route registered) never shows a control that would `404`. `gateActive` is presentation-only — never an access-control signal (the server `401` remains the sole enforcement). This resolves 14-UI-SPEC's one "unresolved — planner must treat as assumption" item as a **locked decision**; it is no longer a UAT confirmation.

### Claude's Discretion
- Exact endpoint paths and method shapes (`POST /session` + `DELETE /session` vs. `/auth/login` + `/auth/logout`), the `401` JSON body shape (`{"error":"unauthenticated"}` or similar), the exact cookie name under the `__Host-` prefix, rate-limit constants (attempts/window, delay range, global threshold), the audit-log line format, and the passphrase-weakness heuristic — all standard, left to research + planning.
- Whether the global brute-force counter also imposes a cooldown lock or only alerts — planner's call based on complexity.
- The `Referrer-Policy: no-referrer` response header (Pitfall 14) and confirming `httplog` doesn't log bodies / scrubs the auth cookie from logged headers — treat as required hardening, exact wiring is discretion.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/REQUIREMENTS.md` §"Access Gate" — GATE-01 … GATE-07, the locked requirement text
- `.planning/ROADMAP.md` §"Phase 14: Instance Passphrase Gate" — goal, 5 success criteria, plan-splitting note

### v1.3 research (this milestone)
- `.planning/research/PITFALLS.md` items **14–23** — the security checklist this phase is judged against: passphrase-in-URL (14), cookie flags (15), session fixation (16), timing-unsafe compare (17), no rate limiting (18), `/health` on the wrong side of the gate (19), CSRF (20), in-memory sessions wiped on deploy (21), reverse-proxy / `X-Forwarded-For` (22), SPA broken-state on 401 (23)
- `.planning/research/ARCHITECTURE.md` §"Feature 2 — Passphrase gate middleware" — the middleware-ordering diagram, `Group`-not-`r.Use` rationale, exemption list, derived-key recommendation, config-disable behavior, file-touch table
- `.planning/research/FEATURES.md` §"Feature 2 — Instance Passphrase Gate" — expected UX (custom login form, not Basic Auth), table-stakes list, "what protected should mean" table, anti-features
- `.planning/research/STACK.md` — stdlib-only crypto (`crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `encoding/base64`); zero new application dependencies
- `.planning/research/SUMMARY.md` — milestone framing (gate must land before the first public deploy)

### Existing code this phase modifies or mirrors
- `internal/httpserver/server.go` — chi router + 4-middleware chain + route registration; add the protected `Group` and `WithAuthGate` option here
- `internal/httpserver/health.go` — `GET /health` stays exempt; keep its payload minimal (no version/build/DSN leakage)
- `internal/config/config.go` — add `InstancePassphrase string \`env:"INSTANCE_PASSPHRASE"\`` (empty = gate disabled) and `TrustProxyHeaders bool \`env:"TRUST_PROXY_HEADERS"\`` (default false, gates `middleware.RealIP` per D-14); follow the existing grouped-by-phase struct convention, both optional, no `Load()` validation
- `cmd/server/main.go` — one line: pass `httpserver.WithAuthGate(cfg.InstancePassphrase)` into `httpserver.New`
- `internal/discord/client.go` — reused by D-12's brute-force alert (see `internal/notifier` for the existing Send path / `notifier.Select` disabled-case idiom)
- `web/app/lib/api.ts` — `apiFetch` is the single fetch funnel; add the global 401 interceptor + custom header on non-GET here
- `web/app/root.tsx` / `web/app/routes.ts` — React Router v7 SPA mode, dark-theme-only; the passphrase screen is a normal SPA view gated by the shared auth state
- `.env.example` — document `INSTANCE_PASSPHRASE` (24+ char recommendation; note that rotating it is the revoke-all lever, D-10) and `TRUST_PROXY_HEADERS` (default false; true only behind a trusted proxy with an unpublished container port)
- Phase 01 slog DSN-redaction pattern (`internal/db/migrate.go` `redactError`) — the model for "secret never reaches a log line"

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`golang.org/x/time/rate`** — already a dependency (external API rate limiters); reuse a `rate.Limiter` per client IP for login throttling.
- **`internal/discord` webhook client + `notifier.Select` disabled-case idiom** — the brute-force alert (D-12) posts through this; the `NoOp` pattern shows how to handle "webhook URL not configured."
- **Functional-option constructors** (`detection.With*`, `poller.With*`, `watchlist.With*`) — `httpserver.WithAuthGate(passphrase)` follows this exact shape, keeping test call sites unchanged when omitted.
- **`chi/middleware.RealIP`** — bundled with chi, gives the client IP once the proxy invariant (D-14) holds.
- **`web/app/lib/api.ts` `apiFetch`** — every endpoint wrapper funnels through this one function; a single `401` branch covers the whole app. `ApiError` already carries `status`.
- **Phase 01 `redactError` / `redactDSN` helpers** — the template for never logging the passphrase.

### Established Patterns
- **chi middleware chain is fixed at 4** (`RequestID` → `echoRequestID` → `httplog` → `Recoverer`); the gate is group-scoped middleware #5, and chi runs parent `Use` before group `Use` so ordering is automatic.
- **Config struct is grouped by introducing phase**, optional fields never `notEmpty`/`required`, non-positive-value validation done manually in `config.Load()` after `env.Parse`. `INSTANCE_PASSPHRASE` is optional (empty = disabled) — no `Load()` validation beyond the boot `WARN`.
- **Seam interfaces are always non-nil at runtime** (see ARCHITECTURE.md anti-pattern) — the gate's "disabled" case is a concrete no-op middleware wired at construction, not a nil check in the request path.
- **Idempotent / fail-fast boot** — `cmd/server/main.go` orders config → logging → migrations → pool → wiring; the passphrase `WARN` belongs right after config load.
- **Tests mock at seams with `httptest.Server` / stubs, no live calls**; the gate and session sign/verify get unit tests in `internal/authgate`, and the SPA gets a Vitest + RTL test (mock a `401` from `api.ts`, assert the passphrase view renders, assert successful auth re-fetches).

### Integration Points
- `internal/httpserver/server.go` `New(...)` — new `WithAuthGate` option; new protected `Group`; new session route registration.
- `cmd/server/main.go` — one wiring line.
- `internal/config/config.go` — one struct field.
- `web/app/lib/api.ts` — 401 interceptor + custom request header.
- `web/app/` root — shared auth state + passphrase screen + logout control.
- `.env.example` — one documented variable.

</code_context>

<specifics>
## Specific Ideas

- **Custom login form, single password field, served by the SPA** — explicitly preferred over HTTP Basic Auth (ugly native prompt, no logout, credentials replayed every request, poor mobile UX). Styled in the app's existing dark-only theme.
- The brute-force alert should read like a real ops signal in the same Discord channel that already gets release alerts — the user chose the "observable / alerting" option deliberately for portfolio value, while explicitly *not* wanting a startup-blocking passphrase policy (D-11 vs D-12 asymmetry is intentional).
- Keep the new configuration surface minimal: session timings stay hardcoded (D-07). The phase adds exactly **two** env vars — `INSTANCE_PASSPHRASE` (the gate itself) and `TRUST_PROXY_HEADERS` (default `false`, gates `middleware.RealIP` per D-14). The original "exactly one variable" goal was relaxed once it became clear a code comment cannot enforce the reverse-proxy invariant the throttle depends on. No `SESSION_TTL`, no signing-key var.

</specifics>

<deferred>
## Deferred Ideas

- **Server-side session revocation / signing-key rotation without logging everyone out** — future `GATE-08`; out of scope here (D-10 accepts client-local logout).
- **`SESSION_TTL` env knob** — considered and rejected (D-07); revisit only if an operator actually needs it.
- **Boot-time fail-closed passphrase-strength enforcement** — considered and rejected (D-11, warn-only); could revisit as a hardening pass.
- **Boot-time `GET /session` status check for a flash-free first paint** — considered and rejected (D-16, global interceptor only); a polish item if the loading flash proves annoying.
- **"Don't publish the container port" topology + reverse proxy / TLS** — Phase 17 (DPLY-08); Phase 14 only documents the invariant that D-14's `RealIP` use depends on.
- **Two-tier `/health` (`/health` public-minimal + `/health/details` gated)** — Pitfall 19 raises it; the deploy phase (17) decides what the health poll reads. Phase 14 just keeps `/health` exempt and minimal.

</deferred>

---

*Phase: 14-instance-passphrase-gate*
*Context gathered: 2026-08-27*
