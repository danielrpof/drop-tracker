---
phase: "14"
slug: "instance-passphrase-gate"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on (high) severity
threats_open: 0
asvs_level: 1
created: "2026-09-01"
---

# Phase 14 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
>
> Built from artifacts (State B): no prior SECURITY.md; register reconstructed from
> the `<threat_model>` blocks in plans 14-01…14-07 (all authored at plan time —
> `register_authored_at_plan_time: true`), cross-checked against 14-VERIFICATION.md
> (7/7 must-haves verified in code + tests), 14-REVIEW.md (0 critical, 2 warning,
> 4 info), and the passing Go + 125/125 frontend suites. ASVS L1, `block_on: high`
> → L1 grep-depth verification, no auditor spawn (short-circuit rule).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Public internet → chi router | Unauthenticated parties reach `/health`, `POST /session`, `DELETE /session`, the SPA shell. | Untrusted request bytes |
| Browser cookie jar → `Authenticate` middleware | The `dt_session` cookie is attacker-controllable bytes until `hmac.Equal` succeeds. | Session MAC |
| Reverse proxy → app (`X-Forwarded-For`) | Trusted only when `TRUST_PROXY_HEADERS=true` AND container port unpublished (D-14) — both Phase-17 runbook steps. Default (false) ignores the header. | Client IP for throttle/audit |
| Process environment → `internal/config` | `INSTANCE_PASSPHRASE` enters here; must never leave the process in observable output. | The instance secret |
| operator shell / `.env` → container process env | The config channel that failed in G-14-1; a value that does not cross here silently disables the gate. | The instance secret |
| App → Discord webhook | Outbound sink whose URL is itself a credential. | Brute-force alert embed (count + window only) |
| App → `slog` / stdout | Everything written here may reach a log aggregator or a public CI run. | Auth outcomes, source IP, gate status token |
| Server 401 / `X-Instance-Gated` header → SPA state | The 401 is the sole enforcement signal; the client auth/gate flags are presentation only. | Presentation hints |
| Browser web storage → SPA module state | `dt_gate_active` under the origin session store is user- and script-controllable. | One fixed presentation literal |
| Node prerender host → browser-only globals | `react-router build` evaluates `root.tsx` + `authStore.ts` in Node. | n/a (build-time) |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-14-01-01 | Spoofing | `authgate.Verify` MAC check | critical | mitigate | `hmac.Equal` over recomputed HMAC-SHA256; tampered-MAC / wrong-key tests | closed |
| T-14-01-02 | Info Disclosure | `HandleLogin` passphrase compare | high | mitigate | SHA-256 both sides → `subtle.ConstantTimeCompare` on 32-byte digests (no value/length timing) | closed |
| T-14-01-03 | Spoofing | Session fixation on login | high | mitigate | `HandleLogin` ignores inbound cookie, always mints fresh nonce + `IssuedAt`; rotation test | closed |
| T-14-01-04 | Elevation of Privilege | Route registration falls open | critical | mitigate | Data routes only via `registerDataRoutes` on gated sub-router; structural exemptions; inert + boundary tests | closed |
| T-14-01-05 | Info Disclosure | `/health` as prefix not exact | medium | mitigate | `r.Get("/health", …)` exact literal; `/healthz`, `/health/details` assert no payload | closed |
| T-14-01-06 | Info Disclosure | Cookie sniffed / script-read | high | mitigate | `Secure` + `HttpOnly` + `SameSite=Lax` + `Path=/`; asserted on raw header | closed |
| T-14-01-07 | Info Disclosure | Passphrase to log line / URL | high | mitigate | POST JSON body only; `httplog` logs no bodies / no Cookie header; buffer-grep test (14-02) | closed |
| T-14-01-08 | Elevation of Privilege | Session outlives cap on renewal | high | mitigate | Renewal copies `IssuedAt` unchanged; `Verify` checks cap before expiry; renewal + cap tests | closed |
| T-14-01-09 | Denial of Service | `/health` swept behind gate → deploy rollback loop | medium | mitigate | `/health` on root router in both branches; asserts 200 unauth on gated server | closed |
| T-14-01-10 | Tampering | `X-Forwarded-For` spoof via `middleware.RealIP` | medium | mitigate | `RealIP` wired only when `TRUST_PROXY_HEADERS=true` (D-14); false-path keys on `RemoteAddr`; test + load-bearing comment | closed |
| T-14-01-11 | Denial of Service | Unbounded `POST /session` body | medium | mitigate | `http.MaxBytesReader` at `maxSessionBodyBytes` (4096) before decode | closed |
| T-14-01-12 | Repudiation | No record of auth outcomes | low | mitigate | D-13 structured audit lines delivered in 14-02 (outcome + source IP) | closed |
| T-14-02-01 | Elevation of Privilege | Unbounded guessing of shared secret | high | mitigate | Per-IP `rate.Limiter` (burst 5, `rate.Every(12s)`) → 429 + fixed 250ms–1s delay on compare paths; boundary/refill tests | closed |
| T-14-02-02 | Info Disclosure | Discord webhook URL via logged send error | high | mitigate | Alert path logs outcome only, never the send error; grep gate in acceptance | closed |
| T-14-02-03 | Info Disclosure | Passphrase to a log line | high | mitigate | No `slog` attr carries value/digest; buffer-scan test over success/failure/throttle | closed |
| T-14-02-04 | Info Disclosure | Alert echoes submitted value | high | mitigate | Embed carries only count + window + instance name; no fragment/length | closed |
| T-14-02-05 | Denial of Service | Limiter map unbounded growth | medium | mitigate | Mutex-guarded map + `lastSeen` sweeper evicting >15min idle; ticker in `NewManager`, stopped by `Close` | closed |
| T-14-02-06 | Denial of Service | Slow/hung Discord webhook stalls login | medium | mitigate | Alert on own goroutine with bounded `context.WithTimeout`; failing Alert never changes login status (test) | closed |
| T-14-02-07 | Tampering | `X-Forwarded-For` rotates throttle key | medium | accept | Inherited from T-14-01-10 under D-14 (unpublished container port); global counter is the compensating control for distributed guessing | closed |
| T-14-02-08 | Denial of Service | Global cooldown locks out legit operator | medium | mitigate | Deliberately alert-only, no global endpoint lock; per-IP limiter + fixed delay carry the bound | closed |
| T-14-02-09 | Repudiation | Auth outcomes not attributable | low | mitigate | Exactly one structured line per outcome with resolved source address (Info success/logout, Warn failure/throttle) | closed |
| T-14-02-10 | Spoofing | Throttled request inflates global counter → alert fatigue | low | mitigate | `recordFailure` called only on genuine mismatch, never throttled/malformed; test | closed |
| T-14-03-01 | Info Disclosure | Passphrase to URL / history / Referer | high | mitigate | `createSession` sends value only in POST JSON body; acceptance grep + test | closed |
| T-14-03-02 | Info Disclosure | Passphrase echoed to error / toast / console / DOM | high | mitigate | Fixed operator-authored strings by status code; value only in component state + password input | closed |
| T-14-03-03 | Tampering | CSRF on state-changing API call | high | mitigate | `X-Requested-With` injected centrally in `apiFetch` for every non-GET; server enforcement in 14-04 | closed |
| T-14-03-04 | Elevation of Privilege | Client auth flag treated as access control | high | mitigate | Store gates presentation only; every data read still crosses the server gate; plan prohibition recorded | closed |
| T-14-03-05 | Denial of Service | Blank SPA on 401 instead of login prompt | medium | mitigate | Single `apiFetch` 401 branch + `App` early return guarantees rendered form on first 401 | closed |
| T-14-03-06 | Spoofing | Cookie theft via injected markup | medium | mitigate | Cookie `HttpOnly` (14-01); all copy plain JSX text nodes, no raw-HTML sink; acceptance grep | closed |
| T-14-03-07 | Denial of Service | Stale local flag strands user after failed logout | low | mitigate | Local state cleared whether `deleteSession` resolves or rejects + failure toast | closed |
| T-14-03-08 | Repudiation | Concurrent 401s → inconsistent client state | low | mitigate | `markUnauthenticated` idempotent + convergent; concurrency test → one state, one gate | closed |
| T-14-04-01 | Tampering | CSRF on gated POST/PATCH/DELETE | high | mitigate | `RequireCSRFHeader` on protected group; CORS entirely absent so preflight denied; tests | closed |
| T-14-04-02 | Spoofing | Login-CSRF forcing an authenticated session | high | mitigate | Same header check at top of `HandleLogin`, before throttle + compare; rejection consumes no token | closed |
| T-14-04-03 | Info Disclosure | Referrer leakage to third-party origin | medium | mitigate | `Referrer-Policy: no-referrer` on every response, gated or inert | closed |
| T-14-04-04 | Elevation of Privilege | Weak / default passphrase | high | mitigate | Boot-time length + known-default denylist heuristic → one WARN; `.env.example` placeholder is on the denylist | closed |
| T-14-04-05 | Denial of Service | Strength policy refuses to start | medium | mitigate | Warn-only by explicit decision (D-11); no error return / exit path; diff acceptance check | closed |
| T-14-04-06 | Info Disclosure | Passphrase via weak-check reason in boot log | high | mitigate | Fixed operator-authored reason phrases; test asserts input + buffer never contain the value | closed |
| T-14-04-07 | Info Disclosure | Real secret committed in `.env.example` | high | mitigate | Value is an obvious denylisted sentinel; gitleaks pre-commit + CI both scan it | closed |
| T-14-04-08 | Elevation of Privilege | CSRF middleware on the inert path changes v1.2 behaviour | medium | mitigate | Both group middlewares inside the gated conditional; inert-path test asserts no 403 for missing header | closed |
| T-14-04-09 | Denial of Service | CSRF header name drift server↔client | medium | mitigate | Server constant + client literal asserted equal by acceptance grep; contract comment each side | closed |
| T-14G-01 | Info Disclosure | `logInstanceGateStatus` in `cmd/server/main.go` | high | mitigate | Logs only a status token (+ inert remediation hint); test asserts value and its rune count absent | closed |
| T-14G-02 | Info Disclosure | `docker compose config` as a verify step | high | mitigate | Banned from every verify step (inlines env_file → leaks webhook URL + passphrase); warnings in compose file + UAT precondition; value-free `GATE_ENV` check supplied | closed |
| T-14G-03 | Tampering | `app.environment:` interpolation shadows a good `.env` value | medium | mitigate | Made observable: boot log reports inert on every affected start; documented in compose comment + UAT precondition | closed |
| T-14G-04 | Spoofing | Operator copies the `.env.example` placeholder verbatim | high | mitigate | Task-4 checkpoint requires a fresh 24+ char value, forbids the placeholder; D-11 boot WARN is the backstop | closed |
| T-14G-05 | Elevation of Privilege | GATE-07 inert path (empty passphrase → no gate) | high | accept | Intended, tested contract (Test 3) that keeps local dev / CI / test suites working; plan makes the state observable, not different | closed |
| T-14G-06 | Repudiation | Gate status from a second env read (false audit record) | medium | mitigate | Helper takes the passphrase as a parameter; single call site passes `cfg.InstancePassphrase`, same value the gate constructor gets | closed |
| T-14G2-01 | Info Disclosure | `persistGateActive` in `authStore.ts` | high | mitigate | One fixed literal under one key; `sessionStorage.setItem` appears exactly once (acceptance grep); no secret / cookie / row ever written | closed |
| T-14G2-02 | Elevation of Privilege | `gateActive` as a persisted signal | high | mitigate | Controls one button's visibility only; `authed` deliberately NOT persisted (acceptance grep); 14-03 prohibition carried; server 401 untouched | closed |
| T-14G2-03 | Tampering | Operator-set `dt_gate_active` in devtools | low | accept | Forging it yields a button calling an unregistered route (404/405); no data / server state / gated route reachable | closed |
| T-14G2-04 | Denial of Service | Module init in a storage-denying browser | high | mitigate | `typeof` guard + `try`/`catch` on both accessors; tests drive undefined + throwing store, assert `mark*` still complete | closed |
| T-14G2-05 | Denial of Service | `react-router build` Node prerender ReferenceError | high | mitigate | Same `typeof` guard; real production build run as an acceptance command | closed |
| T-14G2-06 | Tampering | Cross-session persistence contradicts D-18 ungated rule | medium | mitigate | Scoped to per-tab session store; `must_haves` prohibition; negative grep; UAT precondition to clear the entry | closed |
| T-14G3-01 | Elevation of Privilege | `markGateActive` resurrecting `authed` | high | mitigate | Writes `gateActive` only, never reads/writes `authed`; named test: `isAuthed()` stays false after `markUnauthenticated()` → `markGateActive()`; server 401 sole enforcement | closed |
| T-14G3-02 | Info Disclosure | Ungated instance emitting the marker | medium | mitigate | Only write site is inside `Authenticate`, registered solely in `server.go` `gate != nil` branch; Go negative cases + single-write-site grep + diff-scope criterion | closed |
| T-14G3-03 | Info Disclosure | The marker header itself | low | accept | Emitted only on already-gate-passed responses; one fixed literal (no secret/token/count/timing); discloses strictly less than the existing 401 | closed |
| T-14G3-04 | Spoofing | A forged marker on a response | low | accept | Anyone who can inject a header controls the response entirely; no CORS headers so cross-origin JS cannot read it; forging locally yields only a dead button | closed |
| T-14G3-05 | Denial of Service | Per-response notify + storage write | low | mitigate | One-shot early return; test proves two marker-carrying responses notify exactly once | closed |
| T-14G3-06 | Tampering | Operator-set gate-active entry in devtools | low | accept | Carry-forward of T-14G2-03 — dead button, nothing exposed or reachable | closed |
| T-14G3-07 | Denial of Service | Store init where the storage accessor throws (WR-01 residual) | high | mitigate | Both definedness probes moved inside their `try` blocks; jsdom case drives a throwing accessor | closed |
| T-14G3-08 | Tampering | Two-sided wire literal drift | medium | mitigate | Literal pinned independently on both sides + both test suites; contract comment each side naming the other file | closed |
| T-14-CACHE-01 | Info Disclosure | Gated authenticated 2xx (`GET /watchlist`, `/events`, `/search`) carry no `Cache-Control: no-store` / `Vary: Cookie` | medium | mitigate | **Below `high` threshold — non-blocking.** Surfaced by 14-REVIEW WR-01. A shared/intermediary cache or misconfigured proxy could store one session's authenticated response (body + `X-Instance-Gated` marker) and replay it to another client; the marker also causes the recipient to latch `gateActive`. Today's deployment is a single same-origin binary with no CDN and `SameSite`/`HttpOnly` cookies, and Phase 17 puts it behind an operator-controlled reverse proxy. **Recommendation:** set `Cache-Control: no-store` in the same middleware that stamps the marker (`internal/authgate/gate.go`) so the two cannot drift; optionally `Vary: Cookie`. | open — below high threshold |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

Supply-chain threats T-14-01-SC, T-14-02-SC, T-14-03-SC, T-14-04-SC, T-14G-SC,
T-14G2-SC, T-14G3-SC are all disposition `accept`: every plan in this phase
installs zero packages (no npm/pnpm/pip/cargo/`go get`, no `go.mod` /
`web/package.json` / lockfile change). All crypto is Go stdlib; every other
dependency touched was already present. `14-RESEARCH.md` §Package Legitimacy
Audit and `COVERAGE.md` both record zero new external integration surface. See
the Accepted Risks Log.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-14-01 | T-14-01-SC / T-14-02-SC / T-14-03-SC / T-14-04-SC / T-14G-SC / T-14G2-SC / T-14G3-SC | Phase installs no packages and touches no lockfile; all crypto is Go stdlib; RESEARCH §Package Legitimacy Audit + COVERAGE.md record zero new integration surface. No `[ASSUMED]`/`[SUS]`/`[SLOP]` package to gate. | plan-time threat model (14-01…14-07) | 2026-09-01 |
| AR-14-02 | T-14-02-07 | `X-Forwarded-For` throttle-key rotation is accepted under D-14: `RealIP` is only trusted when the container port is unpublished (Phase-17 topology). The global brute-force counter is the compensating control for distributed guessing, which per-IP throttling structurally cannot bound. | plan-time threat model (14-02) | 2026-09-01 |
| AR-14-03 | T-14G-05 | GATE-07 inert path (empty `INSTANCE_PASSPHRASE` → no gate) is the intended, tested contract that keeps local dev, CI, and the test suites working. Test 3 passed on it. Plan 14-05 makes the state observable (boot log) rather than changing it. | operator (UAT Test 3 PASS) | 2026-09-01 |
| AR-14-04 | T-14G3-03 / T-14G3-04 / T-14G2-03 / T-14G3-06 | The `X-Instance-Gated` marker and the `dt_gate_active` storage entry are presentation-only by construction. Forging either yields a "Log out" button whose route is unregistered (404/405); no data is exposed, no server state changes, no gated route becomes reachable. The server 401 is the sole enforcement. | plan-time threat model (14-06, 14-07) | 2026-09-01 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-01 | 67 | 66 | 1 (below `high` threshold — non-blocking) | /gsd-secure-phase (State B, L1, no auditor per short-circuit) |

Notes:
- `register_authored_at_plan_time: true` (all 7 plans carry `<threat_model>`), `asvs_level: 1`, `block_on: high` → short-circuit rule: no auditor spawn, L1 grep-depth verification.
- Mitigation evidence cross-checked against 14-VERIFICATION.md (7/7 must-haves verified in code + tests; GATE-01…GATE-07; G-14-1/G-14-2/G-14-3 closed) and the passing Go + 125/125 frontend suites.
- 14-REVIEW.md: 0 critical, 2 warning (WR-01 → tracked here as T-14-CACHE-01; WR-02 `markGateActive` no persist retry — cosmetic, documented as a known limitation), 4 info (IN-01 load-bearing D-18 wiring, IN-02 cross-origin latch no-op, both no-change-required today).
- `threats_open: 0` — no OPEN threat at or above `high`. T-14-CACHE-01 is medium and non-blocking.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed (no open threat at or above `high`)
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-01
