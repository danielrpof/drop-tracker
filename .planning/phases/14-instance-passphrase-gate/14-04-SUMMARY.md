---
phase: 14-instance-passphrase-gate
plan: 04
subsystem: auth
tags: [csrf, referrer-policy, security-headers, weak-passphrase, chi, middleware, go]

requires:
  - phase: 14-instance-passphrase-gate
    plan: 01
    provides: "authgate.Manager + Authenticate + writeJSONError; protected chi Group + WithAuthGate option; config.InstancePassphrase; cmd/server boot order"
  - phase: 14-instance-passphrase-gate
    plan: 02
    provides: "HandleLogin/HandleLogout with per-IP throttle + global counter + D-13 audit lines; NewManager wiring"
  - phase: 14-instance-passphrase-gate
    plan: 03
    provides: "web/app/lib/api.ts injecting X-Requested-With: drop-tracker on every non-GET request (D-15 client half)"
provides:
  - "authgate.RequireCSRFHeader(next) middleware — 403 {\"error\":\"missing required header\"} on gated non-GET/HEAD/OPTIONS lacking X-Requested-With: drop-tracker"
  - "authgate.csrfHeaderName / csrfHeaderValue constants + hasCSRFHeader helper — the server half of the D-15 client contract"
  - "CSRF header check at the top of HandleLogin and HandleLogout (before throttle/body/comparison; no limiter token, no counter touch)"
  - "httpserver.securityResponseHeaders middleware — Referrer-Policy: no-referrer on every response, gated and inert"
  - "authgate.IsWeakPassphrase(p) (reason, weak) + knownDefaults denylist + envExamplePlaceholder constant (D-11)"
  - "cmd/server/main.go boot-time weak-passphrase WARN (one line, never blocks startup)"
  - ".env.example INSTANCE_PASSPHRASE + TRUST_PROXY_HEADERS documented with a self-warning placeholder"
affects: [17-vps-deploy]

actuals:
  tokens: 8400
  tasks: 3
  commits: 5

tech-stack:
  added: []
  patterns:
    - "Server/client CSRF contract as paired greppable literals: csrfHeaderName/csrfHeaderValue in gate.go must equal the X-Requested-With literal in web/app/lib/api.ts; a load-bearing comment on each records the coupling"
    - "Custom-header CSRF defence works because CORS is entirely absent — a cross-site attacker cannot set a custom request header without a preflight this server denies; SameSite=Lax and this header are two independent controls"
    - "Cross-cutting response headers (Referrer-Policy) as their own tiny middleware left of the existing chain, not folded into an unrelated closure, so the established r.Use lines stay byte-for-byte unchanged"
    - "Warn-only config heuristic: a pure function (no logging, no error) mirroring config.Load's manual-validation posture, consumed by exactly one WARN call site in main.go that always falls through"

key-files:
  created:
    - internal/authgate/weak.go
    - internal/authgate/weak_test.go
  modified:
    - internal/authgate/gate.go
    - internal/authgate/gate_test.go
    - internal/authgate/login.go
    - internal/authgate/login_test.go
    - internal/httpserver/server.go
    - internal/httpserver/server_test.go
    - cmd/server/main.go
    - .env.example

key-decisions:
  - "Referrer-Policy wired as a new securityResponseHeaders middleware registered as the FIRST r.Use (left of the conditional RealIP block and the four existing lines), NOT folded into echoRequestID — keeps the four established r.Use lines byte-for-byte unchanged and keeps a security header out of a request-ID closure"
  - "CSRF check sits at the very top of HandleLogin (before the semaphore acquire), so a rejected login consumes no concurrency slot, no limiter token and never touches the global counter"
  - "csrf_blocked emitted as a Warn audit line on both HandleLogin and HandleLogout, preserving the D-13 'one structured slog line per response path' invariant for the new path"
  - "envExamplePlaceholder constant = \"caliber\" (the operator changed the .env.example value from \"changeme\" to \"caliber\" mid-execution); both the constant and a literal \"changeme\" are on knownDefaults so the two never drift and the canonical default stays covered"
  - "Task 3 .env.example: the two bare keys 14-01 added were REPLACED with the fully-documented block (comments + assignments) rather than appended — the plan text assumed the keys were absent"

patterns-established:
  - "RequireCSRFHeader registered as a SECOND pr.Use after Authenticate inside the gated Group — auth (401) is decided before the header (403), and the inert path installs neither"
  - "New-package weak-check test file is package authgate (whitebox) so it can assert knownDefaults membership and reuse login_test.go's nonEmptyLines/wbLogger helpers"

requirements-completed: [GATE-01, GATE-03, GATE-07]

coverage:
  - id: D1
    description: "A gated POST/PATCH/DELETE with a valid session cookie but no X-Requested-With header is rejected 403 {\"error\":\"missing required header\"}; the same request WITH the header reaches its handler; an unauthenticated non-GET still gets 401 not 403 (RequireCSRFHeader runs after Authenticate)"
    requirement: "GATE-01"
    verification:
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_RequireCSRFHeader_StateChangingMethods (9 subcases: POST/PATCH/DELETE x without-header/with-header/unauthenticated)"
        status: pass
    human_judgment: false
  - id: D2
    description: "A gated GET and HEAD carrying a valid cookie and no custom header reach their handler — the CSRF check applies to state-changing methods only"
    requirement: "GATE-01"
    verification:
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_RequireCSRFHeader_SafeMethods"
        status: pass
    human_judgment: false
  - id: D3
    description: "POST /session without the custom header returns 403 before the passphrase comparison runs, sets no cookie despite the correct passphrase, spends no per-IP limiter token and does not move the global failure counter; DELETE /session without it is 403"
    requirement: "GATE-01"
    verification:
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_CSRFHeaderOnLogin"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginCSRF_RejectedBeforeComparisonAndCounter"
        status: pass
      - kind: unit
        ref: "internal/authgate/login_test.go#TestLoginCSRF_HeaderPresentProceeds / TestLogoutCSRF_MissingHeaderRejected"
        status: pass
    human_judgment: false
  - id: D4
    description: "Every response from a gated server AND an inert server carries Referrer-Policy: no-referrer"
    requirement: "GATE-03"
    verification:
      - kind: integration
        ref: "internal/httpserver/server_test.go#TestReferrerPolicy_OnEveryResponse (inert + gated subcases)"
        status: pass
      - kind: integration
        ref: "internal/authgate/gate_test.go#TestGate_ReferrerPolicyOnGatedResponse"
        status: pass
    human_judgment: false
  - id: D5
    description: "With the gate inert, no CSRF middleware is installed — no route returns 403 for a missing header — and Referrer-Policy is still set (GATE-07: v1.2 behaviour byte-for-byte intact)"
    requirement: "GATE-07"
    verification:
      - kind: integration
        ref: "internal/httpserver/server_test.go#TestInertPath_NoCSRFRejection"
        status: pass
      - kind: integration
        ref: "internal/httpserver/server_test.go#TestInertPath_FiveArgConstructor / TestInertPath_EmptyPassphraseIsIndistinguishable (still green after the header + CSRF changes)"
        status: pass
    human_judgment: false
  - id: D6
    description: "IsWeakPassphrase: empty -> not weak (gate disabled); <16 runes -> weak with a length reason; case-insensitive + whitespace-trimmed match against knownDefaults -> weak with a known-default reason; rune count not byte count; reason never contains the input"
    requirement: "GATE-01"
    verification:
      - kind: unit
        ref: "internal/authgate/weak_test.go#TestIsWeakPassphrase (12 table cases incl. empty, 15/16 ascii, 15/16 multi-byte, mixed-case default, whitespace-padded default, distinctive value)"
        status: pass
      - kind: unit
        ref: "internal/authgate/weak_test.go#TestWeakPassphrase_EnvExamplePlaceholderOnDenylist"
        status: pass
    human_judgment: false
  - id: D7
    description: "A weak configured passphrase logs exactly one boot WARN carrying a reason attribute and no part of the value; an empty or strong passphrase logs nothing; the process never refuses to start (no os.Exit, no error return added to main.go)"
    requirement: "GATE-01"
    verification:
      - kind: unit
        ref: "internal/authgate/weak_test.go#TestWeakPassphraseBootWarn_OneWarnLineNeverContainsValue / TestWeakPassphraseBootWarn_EmptyOrStrongLogsNothing"
        status: pass
      - kind: manual
        ref: "grep: `grep -c 'IsWeakPassphrase' cmd/server/main.go` = 1; `git diff cmd/server/main.go` adds no os.Exit / error return"
        status: pass
    human_judgment: false
  - id: D8
    description: ".env.example documents INSTANCE_PASSPHRASE (24+ char recommendation, rotation = revoke-all lever) and TRUST_PROXY_HEADERS (default false, reverse-proxy + unpublished-port precondition), with a placeholder value that is itself on the weak denylist and below the length floor"
    requirement: "GATE-07"
    verification:
      - kind: unit
        ref: "internal/config/config_test.go#TestEnvExampleCompleteness (struct <-> file key parity holds)"
        status: pass
      - kind: manual
        ref: "grep: one INSTANCE_PASSPHRASE + one TRUST_PROXY_HEADERS assignment line; '24' appears on the INSTANCE_PASSPHRASE comment; placeholder 'caliber' in knownDefaults"
        status: pass
    human_judgment: false

duration: 11min
completed: 2026-08-29
status: complete
---

# Phase 14 Plan 04: Instance Passphrase Gate — CSRF, Referrer-Policy & Weak-Passphrase WARN Summary

**`authgate.RequireCSRFHeader` on the protected Group and at the top of `HandleLogin`/`HandleLogout` (403 `{"error":"missing required header"}` on any gated state-changing request lacking `X-Requested-With: drop-tracker`), a `securityResponseHeaders` middleware setting `Referrer-Policy: no-referrer` on every response, and `authgate.IsWeakPassphrase` feeding a single non-blocking boot WARN — the phase's last hardening layer, landing after the SPA already sends the header so nothing breaks.**

## Performance

- **Duration:** ~11 min
- **Started:** 2026-08-29T17:09:52Z
- **Completed:** 2026-08-29T17:21:00Z
- **Tasks:** 3 (Tasks 1 & 2 TDD; Task 3 doc)
- **Files:** 2 created, 8 modified

## Where Referrer-Policy was wired, and why (plan `<output>` requirement)

`Referrer-Policy: no-referrer` is set by a **new `securityResponseHeaders` middleware** in `internal/httpserver/server.go`, registered as the **first `r.Use`** — left of the conditional `middleware.RealIP` block and left of the four established middleware (`RequestID` → `echoRequestID` → `httplog` → `Recoverer`).

Rationale for this placement over the two options the plan offered:

1. **A dedicated middleware, not folded into `echoRequestID`'s closure.** The plan allowed setting the header inside `echoRequestID`, but that buries a security-response-header concern inside a function whose name and doc say "request-ID correlation." A one-line named middleware is self-documenting and is the obvious home for the next such header.
2. **Registered first, so it runs for every response on every path** — gated, inert, `/health`, the SPA `NotFound` fallback, and even a panic path (the header is written to the header map before `next` is called, so `middleware.Recoverer` downstream still emits it on the 500).
3. **The four existing `r.Use` lines are byte-for-byte unchanged** — the plan's stated constraint. Only a new line was added above them.

It is set unconditionally (gated and inert alike) because it is free and defence-in-depth: the passphrase already never enters a URL (POST body only, Pitfall 14), so this only closes the residual risk of any *other* app URL leaking to a third-party origin through a `Referer` header.

## Operator manual step — .env.example

**No operator action is required this run.** `.env.example` was writable in this execution (the Phase 11.1-04 tool-denial did not reproduce), so Task 3 was completed directly and committed in `cdc4717`. WINDOWS.md entry #4 (opened by 14-01 for this exact deferral) has been marked **fixed**.

For the record, the block now in `.env.example` (replacing the two bare `INSTANCE_PASSPHRASE=` / `TRUST_PROXY_HEADERS=false` lines 14-01 added) is:

```dotenv
# --- Phase 14: instance passphrase gate (GATE-01..07) ---
# Set this to a non-empty value to enable the instance passphrase gate: the SPA
# shows a full-screen unlock form and every data API route requires a signed
# session cookie. Leave it unset/empty to keep the gate fully inert -- local
# dev, docker-compose and the test suites need no passphrase and behave exactly
# as before v1.3. Use a random value of at least 24 characters. Rotating this
# value (change it and redeploy) invalidates every existing session -- it is
# the instance's only revoke-all lever, because logout is client-local (D-10).
# The value below is an obvious placeholder the process flags with a boot-time
# WARN; replace it (or clear it) before running a reachable instance.
INSTANCE_PASSPHRASE=caliber
# Set the flag below to true ONLY when the app is reachable exclusively through
# a reverse proxy that sets X-Forwarded-For AND the container port is not
# published (the Phase 17 VPS topology). Keep it false for local dev,
# docker-compose, CI and any pre-proxy deploy, or a spoofed X-Forwarded-For
# could bypass the login throttle or forge an audit line (D-14).
TRUST_PROXY_HEADERS=false
```

The placeholder value **`caliber`** (the operator changed it from `changeme` mid-run) is on the `knownDefaults` denylist in `internal/authgate/weak.go` via the `envExamplePlaceholder` constant, and at 7 runes is also below the 16-rune length floor — so an operator who copies the file verbatim trips the boot WARN by either path. The canonical `changeme` remains a separate literal on the denylist.

> **Comment wording note:** the acceptance grep requires `grep -c 'INSTANCE_PASSPHRASE' .env.example` == 1 and the same for `TRUST_PROXY_HEADERS`, so the comment lines say "Set this to…" / "Set the flag below to…" rather than repeating the variable names.

## Accomplishments

- **`authgate.RequireCSRFHeader`** — a `*Manager` method shaped like `Authenticate`. Passes `GET`/`HEAD`/`OPTIONS` straight through; every other method must carry `X-Requested-With: drop-tracker` or gets `403 {"error":"missing required header"}` via the existing `writeJSONError` helper. `csrfHeaderName`/`csrfHeaderValue` are declared with a load-bearing comment naming `web/app/lib/api.ts` as the other half of the contract. Registered as a **second `pr.Use` after `gate.Authenticate`** inside the protected Group, so an unauthenticated non-GET still gets 401, and both middlewares live inside the same `if gate != nil` branch (inert path installs neither).
- **Login-CSRF** — `HandleLogin` and `HandleLogout` reject a missing header at the very top, before the semaphore acquire, the per-IP limiter, the body read and the comparison. A rejection consumes no concurrency slot, no limiter token, never feeds the global counter, and emits one `csrf_blocked` Warn audit line (D-13 invariant preserved).
- **`securityResponseHeaders`** — `Referrer-Policy: no-referrer` on every response (see the section above).
- **`internal/authgate/weak.go`** — `IsWeakPassphrase(p) (reason string, weak bool)`: empty → not weak (gate disabled); `< 16` runes (`utf8.RuneCountInString`) → weak, length reason; lower-cased + `TrimSpace`d match against the unexported `knownDefaults` slice → weak, known-default reason. Reasons are fixed operator-authored phrases and never embed the input. `envExamplePlaceholder` is a named constant so `weak_test.go` pins that the `.env.example` value stays on the denylist.
- **Boot WARN** — `cmd/server/main.go` calls `IsWeakPassphrase(cfg.InstancePassphrase)` immediately after `logging.New` and before `db.RunMigrations`, logging one `Warn` line with a `reason` attribute when weak, then falling through unconditionally. No error return, no `os.Exit`, no branch that can skip the rest of boot.
- **`.env.example`** — documented (see above).
- **Test suite** — 8 new backend tests / test groups: CSRF on POST/PATCH/DELETE (with/without header/unauthenticated), safe-method pass-through, login + logout CSRF isolation (no counter/limiter movement), Referrer-Policy on gated + inert, inert-path no-403, the 12-case `IsWeakPassphrase` table + denylist-membership pin + two boot-WARN buffer-scan tests.

## Task Commits

1. **Task 1 (RED): failing CSRF + Referrer-Policy tests** — `1ad38e7` (test)
2. **Task 1 (GREEN): CSRF header enforcement + no-referrer policy** — `bc11b2b` (feat)
3. **Task 1 (fixup): raise global-counter threshold in the CSRF isolation test** — `7390e16` (test)
4. **Task 2: weak-passphrase heuristic + boot WARN** — `dc1f146` (feat, RED+GREEN combined per the 14-01 phase precedent — new package, whitebox test file would not compile as a test-only commit)
5. **Task 3: document INSTANCE_PASSPHRASE + TRUST_PROXY_HEADERS in .env.example** — `cdc4717` (docs)

**Plan metadata:** _(next commit)_ — `docs(14-04): complete instance passphrase gate hardening plan`

## Files Created/Modified

- `internal/authgate/weak.go` — NEW: `IsWeakPassphrase`, `knownDefaults`, `minPassphraseRunes`, `envExamplePlaceholder`
- `internal/authgate/weak_test.go` — NEW: heuristic table + denylist pin + boot-WARN buffer scans
- `internal/authgate/gate.go` — `csrfHeaderName`/`csrfHeaderValue` constants, `hasCSRFHeader`, `RequireCSRFHeader`
- `internal/authgate/gate_test.go` — `login()`/`deleteSession()` helpers now send the header; new CSRF + Referrer-Policy tests
- `internal/authgate/login.go` — CSRF check + `csrf_blocked` audit line at the top of `HandleLogin` and `HandleLogout`
- `internal/authgate/login_test.go` — `loginReq` helper sends the header; `noCSRFLoginReq` + 3 CSRF isolation tests
- `internal/httpserver/server.go` — `securityResponseHeaders` middleware (first `r.Use`); `pr.Use(gate.RequireCSRFHeader)` inside the gated Group
- `internal/httpserver/server_test.go` — `TestInertPath_NoCSRFRejection`, `TestReferrerPolicy_OnEveryResponse`
- `cmd/server/main.go` — boot-time weak-passphrase WARN between `logging.New` and `db.RunMigrations`
- `.env.example` — documented `INSTANCE_PASSPHRASE` + `TRUST_PROXY_HEADERS` block

## Decisions Made

- **Referrer-Policy placement** — dedicated first-position middleware, recorded in full above.
- **CSRF check before the semaphore in `HandleLogin`** — so a forced cross-site login consumes zero server resources; the plan said "before the throttle," this is one step earlier and strictly safer.
- **`csrf_blocked` audit line** — added on both handlers to keep D-13's "one structured slog line per response path" true for the new rejection path. Not required by the plan; consistent with the phase's audit posture.
- **`envExamplePlaceholder = "caliber"`** — the operator edited `.env.example` from `changeme` to `caliber` while this plan was executing. Per the harness guidance to treat the on-disk file as authoritative, the constant was aligned to `caliber` and `caliber` added to `knownDefaults`; a literal `"changeme"` was kept on the denylist so the canonical default stays covered. Both paths (denylist + `<16` length) flag `caliber`.
- **Task 3 replaced rather than appended** — `.env.example` already carried bare `INSTANCE_PASSPHRASE=` / `TRUST_PROXY_HEADERS=false` from 14-01; the documented block replaces those two lines. `git diff .env.example` shows the two bare lines removed and the block added (technically a modified region, not a pure append — the plan text predated 14-01's stopgap).

## Deviations from Plan

### 1. [Rule 3 — Blocking] `internal/authgate/login_test.go` + `internal/authgate/gate_test.go` shared helpers updated (beyond frontmatter `files_modified` intent)

- **Found during:** Task 1
- **Issue:** Enforcing `X-Requested-With` at the top of `HandleLogin`/`HandleLogout` breaks every existing test that drives `/session` — `login_test.go`'s `loginReq` helper and `gate_test.go`'s `login()` / inline `DELETE /session` calls all built header-less requests. `files_modified` lists both test files, so this is in scope; noting the breadth.
- **Fix:** `loginReq` and `login()`/`deleteSession()` now attach the header like the real client; dedicated `noCSRFLoginReq` + explicit missing-header tests cover the rejection path.
- **Verification:** `go test ./internal/authgate/ -count=1` green; the full pre-existing 14-01/14-02 suite still passes.
- **Committed in:** `1ad38e7`, `bc11b2b`, `7390e16`.

### 2. [Rule 1 — Bug] CSRF counter-isolation test tripped a real brute-force alert

- **Found during:** Task 1 GREEN
- **Issue:** `TestLoginCSRF_RejectedBeforeComparisonAndCounter` follows the CSRF reject with 5 header-carrying wrong-passphrase attempts to prove no limiter token was pre-consumed; with the shrunk `globalThreshold=3` those legitimate failures fired a real alert, failing the "no alert off a CSRF-blocked request" assertion.
- **Fix:** raised the test's threshold to 100 so the follow-up attempts cannot trip it — the test now isolates only the CSRF path.
- **Committed in:** `7390e16`.

### 3. [Rule 3 — Blocking] `.env.example` writable; placeholder changed by operator mid-run

- **Found during:** Task 3
- **Issue:** (a) The plan's `<precondition>` / env-boundary anticipated `.env.example` being denied to agent tools; it was writable this run. (b) The operator changed the placeholder from `changeme` to `caliber` on disk during execution.
- **Fix:** Task 3 completed directly (commit `cdc4717`); `envExamplePlaceholder` constant + `knownDefaults` aligned to `caliber` with `changeme` retained. WINDOWS.md entry #4 marked `fixed`.
- **Verification:** `TestEnvExampleCompleteness` green; `TestWeakPassphrase_EnvExamplePlaceholderOnDenylist` green.
- **Committed in:** `cdc4717` (`.env.example`, `weak.go`).

---

**Total deviations:** 3 (2 blocking resolved, 1 test bug auto-fixed). **Impact:** none on the delivered surface — every `<behavior>`, `<acceptance_criteria>` and `<success_criteria>` item is implemented and test-proven. No scope creep; `caliber` vs `changeme` is cosmetic and both are covered.

## Issues Encountered

- **No `pnpm`, `make`, `-race`, or pre-commit hook on this dev machine** (documented STATE.md / prior-phase limitation). Substitutes used: `node_modules/.bin/vitest run` (101 tests pass) and `node_modules/.bin/react-router build` (green) for the frontend; `go test ./... -short` + targeted non-short runs for the backend. `make test-integration` (Docker Postgres) and `go test -race` run in CI only. This plan touches no `web/` file, so the frontend suite is unchanged by construction.
- **gitleaks pre-commit hook not installed locally** — `caliber` / `changeme` are obvious non-credential sentinels; the CI `gitleaks-action` is the backstop and will scan `cdc4717`.

## Plan Verification Results

| Check | Result |
|-------|--------|
| `go build ./... && go vet ./...` | exit 0 |
| `go test ./... -short -count=1` | exit 0 — every package `ok` |
| `go test ./internal/authgate/ ./internal/httpserver/ -run 'CSRF\|Referrer\|Inert' -count=1` | exit 0 |
| `go test ./internal/authgate/ -run 'Weak' -count=1` | exit 0 |
| `go test ./internal/authgate/ ./internal/httpserver/ ./internal/config/ ./cmd/... -count=1` (non-short, `INSTANCE_PASSPHRASE` absent) | exit 0 |
| `node_modules/.bin/vitest run` (web, = `pnpm test`) | exit 0 — 101 tests, 12 files; coverage 87.55 / 77.73 / 86.02 / 88.74 |
| `node_modules/.bin/react-router build` (= `pnpm build`) | exit 0 |
| `make test-integration` | not run — no `make`/Docker on this machine; CI-only per VALIDATION.md |
| `grep -c 'RequireCSRFHeader' internal/httpserver/server.go` | 1, inside the `if gate != nil` branch |
| `X-Requested-With` in `internal/authgate/gate.go` and `web/app/lib/api.ts` | both `"X-Requested-With"` / `"drop-tracker"` — identical |
| `grep -c 'utf8.RuneCountInString' internal/authgate/weak.go` | 1 |
| `grep -c 'IsWeakPassphrase' cmd/server/main.go` | 1; `git diff` adds no `os.Exit` / error return |
| `.env.example` keys | one `INSTANCE_PASSPHRASE` + one `TRUST_PROXY_HEADERS` assignment; `24` on the passphrase comment |

## Threat Flags

None. Every change operates inside the trust boundaries `14-04-PLAN.md`'s `<threat_model>` already enumerates. `RequireCSRFHeader` and `securityResponseHeaders` add no new endpoint or trust boundary — they harden the existing gated surface. `IsWeakPassphrase` is a pure function with no I/O. No new package (`unicode/utf8`, `strings`, `net/http` are stdlib; everything else was already in `go.mod`).

## Next Phase Readiness

- **Phase 14 is code-complete.** All three waves delivered; the orchestrator's code review + phase verification are the next step.
- **Phase 17 (VPS deploy):** `.env.example` now documents the `TRUST_PROXY_HEADERS=true` precondition (reverse proxy sets `X-Forwarded-For`, container port unpublished). The Phase 17 runbook must set that flag together with that topology, and should set a real 24+ char random `INSTANCE_PASSPHRASE` (the committed `caliber` placeholder self-warns at boot but must not ship).
- **Requirements:** GATE-01, GATE-03, GATE-07 are this plan's declared set and — 14-04 being the last plan in the phase — the shared-ID gate now releases them along with any still-Pending IDs from 14-01/02/03. Marked complete via `requirements.ready-ids` in the state step below.

## Self-Check

- Both created files present on disk (`internal/authgate/weak.go`, `internal/authgate/weak_test.go`).
- All 5 task commits present in `git log` (`1ad38e7`, `bc11b2b`, `7390e16`, `dc1f146`, `cdc4717`).
- Plan `<verification>` re-run: `go build`/`go vet` exit 0; `go test ./... -short` exit 0; targeted `-run` filters exit 0; web vitest + build green.
- Task acceptance greps re-checked (table above): `RequireCSRFHeader` ×1 in server.go inside the gated branch, header literal identical in gate.go and api.ts, `utf8.RuneCountInString` ×1, `IsWeakPassphrase` ×1 in main.go with no new exit path, `.env.example` key parity.

## Self-Check: PASSED

---
*Phase: 14-instance-passphrase-gate*
*Completed: 2026-08-29*
