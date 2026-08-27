# Phase 14: Instance Passphrase Gate - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-08-27
**Phase:** 14-instance-passphrase-gate
**Areas discussed:** Signing key source, SPA shell reachability, Session lifetime & renewal, Hardening depth (401 handling, logout, client IP, and CSRF also covered)

---

## Signing key source

| Option | Description | Selected |
|--------|-------------|----------|
| Derived from passphrase | `key = SHA256(INSTANCE_PASSPHRASE)`. One secret. Rotating the passphrase invalidates all sessions automatically. Research recommendation. | ✓ |
| Separate SESSION_SECRET | Independent 32-byte env var; revoke-all without changing the passphrase. Two secrets to provision. | |
| Derived now, key-version byte | Derive from passphrase but prepend a version byte for later force-invalidation without a passphrase change. | |

**User's choice:** Derived from passphrase
**Notes:** Rotation = revoke-all is acceptable and desirable at single-operator scale. No second secret anywhere in GitHub Actions or the VPS `.env`.

---

## SPA shell reachability

| Option | Description | Selected |
|--------|-------------|----------|
| Shell public, API gated | Static shell + bundle serve to anyone; SPA boots, gets 401, renders passphrase screen. Gate is a pure API concern. Research recommendation. | ✓ |
| Gate everything but /login | NotFound fallback moves inside the gated group; a minimal Go-served /login page + assets are exempt. Nothing visible unauthenticated; second static-serving path in Go. | |

**User's choice:** Shell public, API gated
**Notes:** Bundle has no secrets; watchlist data + Discord webhook config only move through the gated API.

### Follow-up: how the SPA detects the 401

| Option | Description | Selected |
|--------|-------------|----------|
| Global 401 interceptor | `apiFetch` catches any 401, flips shared auth state, app renders `<PassphraseGate>`. One chokepoint. Brief loading flash; mid-session expiry handled for free. | ✓ |
| Status check on boot | SPA calls `GET /session` before rendering routes. No flash; extra endpoint + root gating point. | |
| Both | Boot check for clean first paint + interceptor for expiry. Most polished, more code. | |

**User's choice:** Global 401 interceptor

---

## Session lifetime & renewal

| Option | Description | Selected |
|--------|-------------|----------|
| Sliding, 30d / 90d cap | Max-Age 30d; re-issue past the halfway mark; absolute ~90d cap from first login. Active devices never surprise-logged-out. | ✓ |
| Fixed 30 days | 30d, no renewal. Monthly re-login regardless of activity. Simplest. | |
| Fixed 7 days | Shorter window; a leaked cookie is only useful a week. More re-logins. | |

**User's choice:** Sliding, 30d / 90d cap

### Follow-up: logout semantics

| Option | Description | Selected |
|--------|-------------|----------|
| Client-local logout, accept it | `DELETE /session` clears the cookie on that browser; a copied cookie stays valid until TTL. Revoke-all = rotate the passphrase. Research recommendation. | ✓ |
| Client-local + document the rotation lever | Same behavior, plus an explicit runbook note about passphrase rotation as the revoke-all path. | |

**User's choice:** Client-local logout, accept it

### Follow-up: TTL config knob vs. hardcoded

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded constants | 30d/90d as consts in `internal/authgate`. One less config var. | ✓ |
| SESSION_TTL env var | Optional `SESSION_TTL time.Duration` (envDefault 720h), cap = 3× TTL. Operator-tunable. | |

**User's choice:** Hardcoded constants — `INSTANCE_PASSPHRASE` stays the only new config var.

---

## Hardening depth

### Boot-time passphrase strength

| Option | Description | Selected |
|--------|-------------|----------|
| Fail closed on weak/short/default | Process exits non-zero at boot on a short passphrase or a known-default match. Secure by construction. | |
| Warn only | Log a WARN if the passphrase looks weak, start anyway. Never risks a deploy failing on a passphrase-policy edge case. | ✓ |
| Minimum length only | Fail closed below a minimum length; skip the known-default blocklist. | |

**User's choice:** Warn only

### Brute-force defense beyond per-IP throttle

| Option | Description | Selected |
|--------|-------------|----------|
| Per-IP throttle only | `rate.Limiter` per IP (~5/min then 429) + fixed response delay + constant-time compare. Covers success criterion #4. No global state. | |
| Add global counter + Discord alert | Also a process-wide failed-attempt counter that fires a Discord alert via the existing webhook past a threshold, plus optional cooldown lock. Reuses the notifier sink; on-brand for the portfolio. Extra scope. | ✓ |
| Global counter, log only | Process-wide counter that escalates delay + emits a WARN, no Discord dependency in the auth path. | |

**User's choice:** Add global counter + Discord alert
**Notes:** Deliberate asymmetry with the "warn only" passphrase-strength choice — the user wants the observable/alerting DevOps signal but not a startup-blocking policy.

### Follow-up: client IP resolution (behind a Phase 17 reverse proxy)

| Option | Description | Selected |
|--------|-------------|----------|
| Trust X-Forwarded-For, document the invariant | `chi/middleware.RealIP`; safe only if the app is never reachable except through the proxy. Code comment + runbook note that the container port must not be published. Research's recommended path. | ✓ |
| Parse only the proxy's appended hop | Custom middleware trusting only the last XFF value. Safer if the port is ever exposed; more code + tests now. | |
| RemoteAddr only for now | Key on the direct peer; behind a proxy all traffic shares one bucket until Phase 17. Under-protects once deployed. | |

**User's choice:** Trust X-Forwarded-For, document the invariant

### Follow-up: CSRF

| Option | Description | Selected |
|--------|-------------|----------|
| SameSite=Lax + require custom header | Lax blocks classic form CSRF; middleware also rejects non-GET /api requests lacking a custom header that api.ts always sets. Defense-in-depth. CORS stays absent. | ✓ |
| SameSite=Lax only | Rely on Lax + same-origin alone. Simpler; thinner margin. | |

**User's choice:** SameSite=Lax + require custom header

---

## Claude's Discretion

- Exact endpoint paths/methods (`POST /session` + `DELETE /session` vs. `/auth/login` + `/auth/logout`)
- `401` JSON body shape, cookie name under the `__Host-` prefix
- Rate-limit constants (attempts/window, delay range, global threshold)
- Audit-log line format; the passphrase-weakness heuristic
- Whether the global brute-force counter also imposes a cooldown lock or only alerts
- `Referrer-Policy` header and `httplog` body/header scrubbing verification (required, wiring is discretion)

## Deferred Ideas

- Server-side session revocation / signing-key rotation without mass logout → future GATE-08
- `SESSION_TTL` env knob → rejected (D-07)
- Fail-closed passphrase-strength enforcement → rejected (D-11)
- Boot-time `GET /session` status check for flash-free first paint → rejected (D-16)
- "Don't publish the container port" topology + reverse proxy / TLS → Phase 17
- Two-tier `/health` (public-minimal + gated details) → Phase 17 decides what the health poll reads
