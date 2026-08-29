---
phase: 14-instance-passphrase-gate
reviewed: 2026-08-29T00:00:00Z
depth: standard
files_reviewed: 12
files_reviewed_list:
  - internal/authgate/session.go
  - internal/authgate/gate.go
  - internal/authgate/login.go
  - internal/authgate/alerter.go
  - internal/authgate/weak.go
  - internal/config/config.go
  - internal/httpserver/server.go
  - cmd/server/main.go
  - web/app/lib/authStore.ts
  - web/app/lib/api.ts
  - web/app/components/auth/PassphraseScreen.tsx
  - web/app/root.tsx
findings:
  critical: 0
  warning: 4
  info: 5
  total: 9
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-08-29
**Depth:** standard
**Files Reviewed:** 12 (+ 8 test files and `internal/discord/client.go` read for cross-reference)
**Status:** issues_found

## Summary

The crypto core (`session.go`), the middleware wiring (`gate.go` / `server.go`),
and the brute-force layer (`login.go`) are carefully built and heavily tested.
The specific risk areas called out for this review hold up under scrutiny:

- **Gate never falls open** — the only path to `next.ServeHTTP` in `Authenticate`
  is `Verify(...) ok == true`, which runs after a constant-time `hmac.Equal`,
  the absolute cap, and the expiry check, in that order. `NewManager` has no
  panic path, `gate` is non-nil inside every `if gate != nil` branch, and
  `registerDataRoutes` is called exactly once. A panic in the renewal path
  (`newNonce`) is caught by `middleware.Recoverer` (registered outside the
  Group) and converted to 500 without calling `next`.
- **Absolute-cap-before-expiry ordering** is correct (`session.go:175-180`) and
  regression-pinned by `TestVerify` / `TestGate_AbsoluteCapRejectsUnexpiredToken`.
- **Constant-time comparison** — `subtle.ConstantTimeCompare` over two fixed
  32-byte SHA-256 digests (no length leak), and `hmac.Equal` for the MAC.
- **Session rotation on login** — `HandleLogin` ignores any inbound cookie and
  mints a fresh `IssuedAt`/nonce; `TestGate_TwoLoginsRotateCookieValue` pins it.
- **Secret never logged / returned** — audit lines carry only `outcome` +
  `source_ip`; `writeJSONError` uses fixed strings; `dispatchAlert` / `SelectAlerter`
  never format the webhook URL or error; `IsWeakPassphrase` returns fixed
  reason phrases. `TestAudit_NoPassphraseInLogsAcrossOutcomes` and
  `TestNoDSNInLogs` guard it.
- **Throttle keying** — `clientIP` uses `r.RemoteAddr` (direct peer) unless
  `middleware.RealIP` is wired, which only happens with `gate != nil &&
  trustProxyHeaders`. Default is fail-safe.
- **CSRF safe-method list** — `{GET, HEAD, OPTIONS}` pass through; everything
  else requires the exact `X-Requested-With: drop-tracker` header, matching the
  SPA client. `RequireCSRFHeader` is registered after `Authenticate`, so an
  unauthenticated non-GET still gets 401.
- **Inert path** — routing is byte-identical to v1.2 (`TestInertPath_*`); the one
  deliberate, tested deviation is the always-on `Referrer-Policy` header (see IN-03).

No blocking defects were found. The warnings below concern security hardening
depth and operational fragility rather than incorrect behavior in the default
configuration.

**Review gap:** `.env.example` is in a directory denied by the sandbox and could
not be read. `weak_test.go` pins that whatever it ships as `INSTANCE_PASSPHRASE`
equals `envExamplePlaceholder` (`"caliber"`) and is on `knownDefaults`, so the
contract is enforced by test even though the file itself was not directly
reviewed (IN-05).

## Warnings

### WR-01: `dt_session` cookie name drops the `__Host-` prefix that D-09 specified; the equivalence rationale is incomplete

**File:** `internal/authgate/session.go:25-39`, `internal/authgate/gate.go:183-193`
**Issue:** The package uses the bare cookie name `dt_session` instead of the
locked D-09 name `__Host-dt_session`. The in-code rationale argues the two are
equivalent because `setSessionCookie` sets `Secure`, `Path=/` and no `Domain`
explicitly. That reasoning covers only the attributes of *this server's own*
cookie. The `__Host-` prefix additionally makes the browser **reject any
`__Host-`-named cookie that is not `Secure` + `Path=/` + host-only** — which is
what defends against a *different party* injecting a `dt_session` cookie:

- a related-domain / sibling-subdomain attacker (e.g. XSS on another app under a
  shared parent domain) can set a domain-scoped `dt_session` cookie;
- a network attacker on any plain-HTTP hop (before Phase 17 TLS) can inject a
  `Set-Cookie: dt_session=...` for the parent domain.

`r.Cookie(sessionCookieName)` returns the *first* matching cookie, so an injected
value can shadow the legitimate one (session-fixation / cookie-tossing). The
practical risk is low for a single-domain deployment that terminates TLS, but
this is a real reduction in defense vs. the locked decision, and the summary that
"approved" it did so on an incomplete equivalence claim.
**Fix:** Restore `__Host-dt_session` and branch the name only if local-over-HTTP
testing in Chrome actually proves necessary:
```go
// pick the prefixed name in production; the bare name is a localhost-only escape hatch
func sessionCookieName() string {
    if devInsecureCookies { // gated by an explicit env var, default false
        return "dt_session"
    }
    return "__Host-dt_session"
}
```
At minimum, document the residual cookie-injection risk in the phase security
notes rather than asserting full equivalence.

### WR-02: passphrase→key derivation is a single unsalted SHA-256 — one captured cookie enables an offline passphrase brute-force

**File:** `internal/authgate/session.go:87-93`
**Issue:** `DeriveKey` = `SHA256(prefix || passphrase)`. A session cookie is
`base64(payload) "." base64(HMAC-SHA256(key, base64(payload)))` — the payload is
fully known plaintext. An attacker who obtains **one** valid cookie (pasted by a
user for support, captured from a proxy log, browser-extension exfil, etc.) can
run an offline dictionary/brute-force attack at roughly two hash operations per
guess: `HMAC(SHA256(prefix+guess), knownPayload) == knownMAC`. None of the
online defenses (per-IP throttle, jittered delay, concurrency shed, global alert)
apply to an offline attack, and the D-11 weak-passphrase check only **warns** —
the process still boots with a 1-character passphrase. So a short or
human-memorable `INSTANCE_PASSPHRASE` is realistically recoverable.
**Fix:** Derive the key with a deliberately slow, salted KDF:
```go
// golang.org/x/crypto/argon2
key := argon2.IDKey([]byte(passphrase), []byte(keyDomainSalt), 3, 64*1024, 4, 32)
```
The salt can be a fixed package constant (it still defeats rainbow tables and
raises per-guess cost by orders of magnitude). This revisits locked decision
D-01; if D-01 is kept, the phase docs and `.env.example` must state in the
strongest terms that `INSTANCE_PASSPHRASE` MUST be a ≥24-char random value, and
`IsWeakPassphrase` should arguably raise `minPassphraseRunes`.

### WR-03: `middleware.RealIP` trusts `X-Forwarded-For` with no trusted-proxy allowlist — a single misconfigured deploy fully bypasses the login throttle and forges audit IPs

**File:** `internal/httpserver/server.go:121-134`, `internal/authgate/login.go:169-179`
**Issue:** When `TRUST_PROXY_HEADERS=true`, `middleware.RealIP` unconditionally
rewrites `r.RemoteAddr` from `True-Client-IP` / `X-Real-IP` / `X-Forwarded-For`
with no verification that the request actually arrived from a known proxy. If
that flag is ever set while the container port is reachable directly (a common
misconfiguration: port left published, proxy added later, a second ingress path,
docker-compose port mapping), an attacker sends a fresh `X-Forwarded-For` per
request and:
- the per-IP token bucket in `getLimiter(ip, ...)` never throttles (every request
  is a "new" IP), defeating GATE-04 entirely;
- the D-13 audit `source_ip` is fully attacker-controlled, so the audit trail and
  the Discord alert become useless / misleading during an attack.

The default (`false`) is safe and tested, and the comments are thorough, but the
blast radius of the misconfiguration is total and there is no in-process
guardrail.
**Fix:** Use a proxy-aware extractor with an explicit trusted-CIDR list (e.g.
parse `X-Forwarded-For` right-to-left, skipping trusted hops), configured via a
new `TRUSTED_PROXY_CIDRS` var; or add a boot-time assertion/log that
`TRUST_PROXY_HEADERS=true` is only meaningful behind a proxy and log the resolved
client IP source on the first N requests so a misconfig is visible in ops logs.

### WR-04: fire-and-forget alert goroutines are not accounted for at shutdown

**File:** `internal/authgate/login.go:283-302`, `internal/authgate/gate.go:66-87`
**Issue:** `dispatchAlert` spawns a goroutine bound only to
`context.Background()` + a 10s timeout. `Manager.Close()` stops the sweep ticker
but does not wait for (or cancel) in-flight alert goroutines, and
`cmd/server/main.go`'s graceful-shutdown path (`shutdownTimeout = 10s`) has no
knowledge of them. During a brute-force burst at the moment of a SIGTERM, an
alert goroutine can outlive `run()` and be killed mid-request by `os.Exit`,
losing the alert with no log. The `discord` client's own ctx-bounded 429 wait
caps the lifetime at ~10s, so this is a correctness/observability nit rather than
a leak, but it is inconsistent with the care taken to drain the poller and
backfill.
**Fix:** Give `Manager` a `context.Context` (or a `sync.WaitGroup`) that
`Close()` cancels/waits, and derive the alert dispatch context from it so
shutdown either flushes or explicitly abandons the alert with a log line.

## Info

### IN-01: `httpserver.New` doc comment no longer matches middleware order

**File:** `internal/httpserver/server.go:74-82`
**Issue:** The comment says "The chi middleware stack runs in this order:
`middleware.RequestID` first ...". As of this phase, `securityResponseHeaders`
runs before `RequestID`, and `middleware.RealIP` runs before *that* when
`trustProxyHeaders` is set. A reader relying on the comment would misjudge what
context is available in early middleware.
**Fix:** Update the comment to list `RealIP?` → `securityResponseHeaders` →
`RequestID` → `echoRequestID` → `httplog` → `Recoverer`.

### IN-02: `apiFetch` header merge assumes `init.headers` is a plain object

**File:** `web/app/lib/api.ts:120-126`
**Issue:** `{ ...init?.headers, "X-Requested-With": "drop-tracker" }` silently
produces `{}` (dropping all caller headers, including `Content-Type`) if a caller
ever passes a `Headers` instance or a `[string,string][]`. Every current caller
passes an object literal, so this is latent, but it is a foot-gun in a function
whose whole job is to guarantee the CSRF header is present.
**Fix:** Normalize first: `const merged = new Headers(init?.headers); if (method
!== "GET") merged.set("X-Requested-With", "drop-tracker");` or constrain the
wrapper signatures to `Record<string, string>`.

### IN-03: the inert path is no longer byte-for-byte v1.2 — every response gains `Referrer-Policy: no-referrer`

**File:** `internal/httpserver/server.go:136-144`, `206-216`
**Issue:** `securityResponseHeaders` is registered on the root router in **both**
branches, so an unconfigured (no-passphrase) instance now emits a response header
it did not emit in v1.2. `TestReferrerPolicy_OnEveryResponse` asserts this is
intentional, and it is a reasonable hardening, but it is a behavioral change to
the "inert = v1.2" contract that GATE-07 otherwise guards.
**Fix:** No code change required if this was an approved scope addition — record
it explicitly in the phase summary as a deliberate exception to "byte-identical
inert path" so a future audit does not flag it as a regression.

### IN-04: `PassphraseScreen` collapses 403 and 5xx into "Couldn't reach the server"

**File:** `web/app/components/auth/PassphraseScreen.tsx:63-76`
**Issue:** Only `401` and `429` get specific copy; everything else (including a
`403` from `RequireCSRFHeader` when a corporate proxy strips `X-Requested-With`,
and any `5xx`) shows `ERROR_CONNECTION` ("Check your connection"). A CSRF-header
strip would loop the operator on a misleading message with no path to diagnosis.
**Fix:** Add a distinct branch for `403` ("This request was blocked. Reload the
page and try again.") and keep `5xx` on a "server error" message distinct from a
network failure.

### IN-05: `.env.example` not directly reviewable in this environment

**File:** `.env.example`
**Issue:** The file is in a sandbox-denied directory; the placeholder value,
comments, and any recommended-entropy guidance for `INSTANCE_PASSPHRASE` /
`TRUST_PROXY_HEADERS` could not be inspected. `weak_test.go` enforces the
placeholder-on-denylist contract, but the human-facing guidance text (which is
the real mitigation for WR-02) was not verified.
**Fix:** Manually confirm `.env.example` states `INSTANCE_PASSPHRASE` must be a
long random value (not a passphrase a human picks), documents that
`TRUST_PROXY_HEADERS=true` is only safe when the container port is unpublished,
and that leaving `INSTANCE_PASSPHRASE` empty disables the gate.

---

_Reviewed: 2026-08-29_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
