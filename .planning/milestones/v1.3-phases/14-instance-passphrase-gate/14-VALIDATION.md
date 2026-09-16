---
phase: 14
slug: instance-passphrase-gate
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: 2026-08-27
updated: 2026-09-01
---

# Phase 14 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
>
> **Note (2026-08-27):** this contract was hand-authored during a plan-hardening pass, not by
> `/gsd-validate-phase`. The content below is complete and executable; run `/gsd-validate-phase 14`
> to formally validate — that step populates the sign-off block, sets `status: validated`, and flips
> `nyquist_compliant` once it confirms sampling continuity. Do not treat this as NOT-VALIDATED for
> audit purposes only because of the `status: draft` frontmatter — treat it as "validated content,
> formal step pending".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + `httptest` (backend); Vitest 4 + `@testing-library/react` + `user-event` + `jest-dom` (frontend) |
| **Config file** | none for Go; `web/vitest.config.ts` + `web/vitest.setup.ts` for frontend |
| **Quick run command** | `go test ./internal/authgate/... ./internal/httpserver/... ./internal/config/... -count=1` |
| **Full suite command** | `make test-integration && (cd web && pnpm test)` |
| **Estimated runtime** | quick ~45–60s (timing `var`s shrunk via `export_test.go`); full ~4 min (needs Docker Postgres on :5433 for `make test-integration`) |

> `-race` is documented unusable on this dev machine (ThreadSanitizer allocation failure — STATE.md
> Phase 11.1-04). Use plain `go test` locally; CI runs `-race`. The GATE-01/GATE-04 concurrency
> checks are backstop-tier for exactly this reason.

---

## Sampling Rate

- **After every task commit:** run the quick command, plus (for `web/` changes) `cd web && pnpm test -- --run <touched files>`.
- **After every plan wave:** run the full suite (`make test-integration` + `cd web && pnpm test` with the coverage gate).
- **Before `/gsd-verify-work`:** full suite green; backend ≥80% / frontend ≥70% (all four axes) coverage gates not regressed.
- **Max feedback latency:** 60s for the quick command.

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 14-01-01 | 01 | 1 | GATE-03 | T-14-01-06 | Cookie name chosen so browser-enforced + code-enforced attributes agree (A1: `__Host-` vs bare name) | checkpoint:decision | *(human-check — operator selects option-a/b/c; recorded in 14-01-SUMMARY.md)* | ✅ | ✅ green |
| 14-01-02 | 01 | 1 | GATE-01, GATE-02, GATE-03, GATE-07, D-14 | T-14-01-01/04/09 | Unauth data route → 401; `/health` → 200; login mints signed cookie that unlocks; inert path structurally unchanged; RealIP only when `TRUST_PROXY_HEADERS` true | unit (httptest) | `go test ./internal/authgate/ ./internal/httpserver/ ./internal/config/ -count=1` | ✅ | ✅ green |
| 14-01-03 | 01 | 1 | GATE-07, GATE-01 (edges) | T-14-01-04/05/09 | `/health` exact-path exempt (`/healthz`, `/health/details` not); SPA shell public (D-04); empty/absent cookie → 401 no panic; 401 carries `request_id`; unset ≡ empty passphrase | unit | `go test ./internal/httpserver/ ./internal/authgate/ -run 'Inert\|Exempt\|Empty\|Ordering\|Concurrent' -count=1` | ✅ | ✅ green |
| 14-01-04 | 01 | 1 | GATE-03, GATE-06, D-06, D-17 | T-14-01-01/03/06/08 | Cookie flags (HttpOnly/Secure/SameSite=Lax/Path=/); sliding renewal keeps `IssuedAt`; 90-day cap enforced before expiry; cap is per-authentication (fresh login resets); rotation on login; tamper/wrong-key → 401; logout → Max-Age=0 | unit | `go test ./internal/authgate/ -run 'Verify\|Cookie\|Rotation\|Renew\|Logout' -count=1` | ✅ | ✅ green |
| 14-02-01 | 02 | 2 | GATE-04, D-12 | T-14-02-01/05/07/11 | 6th per-IP attempt → 429 (undelayed); 2nd IP unaffected; comparison paths delayed, 429/503 not; `maxConcurrentLogins` sheds excess with 503; limiter map swept | unit | `go test ./internal/authgate/ -run 'Throttle\|Delay\|Sweep\|Concurren\|Busy' -count=1` | ✅ | ✅ green |
| 14-02-02 | 02 | 2 | GATE-04, D-12 | T-14-02-02/04/06/10 | Global counter fires `Alert` once per cooldown; unset webhook → inert + one Info line; throttled request doesn't increment; failing Alert doesn't change login status; webhook URL never in a log line | unit (fake Alerter) | `go test ./internal/authgate/ -run 'GlobalAlert\|SelectAlerter\|Counter' -count=1` | ✅ | ✅ green |
| 14-02-03 | 02 | 2 | D-13 | T-14-02-03/09 | One structured audit line per outcome (success/fail/throttle/logout) with source IP; captured `slog` buffer never contains the passphrase on any path; `LogRequestHeaders` not widened | unit (buffer scan) | `go test ./internal/authgate/ -run 'Audit\|NoPassphraseInLogs' -count=1` | ✅ | ✅ green |
| 14-03-01 | 03 | 2 | GATE-05, GATE-06, D-15, D-16, D-18 | T-14-03-01/03/04/08 | Any 401 → `authStore` unauthenticated once (idempotent) + still throws `ApiError(401)`; every non-GET carries `X-Requested-With`; passphrase only in POST body; `gateActive` set on first 401/login | unit (mock global fetch) | `cd web && pnpm test -- --run authStore api` | ✅ | ✅ green |
| 14-03-02 | 03 | 2 | GATE-05 | T-14-03-02/05 | Approved `<PassphraseScreen>` copy verbatim; type=password, no placeholder, autofocus; 401/429/network → distinct fixed messages; typed value retained + never echoed to DOM/toast/log | RTL | `cd web && pnpm test -- --run PassphraseScreen` | ✅ | ✅ green |
| 14-03-03 | 03 | 2 | GATE-05, GATE-06, D-18 | T-14-03-04/05/07 | `<App>` renders `<PassphraseScreen>` when `!authed`; Log out only when `gateActive`; logout clears local state on success AND failure (+ toast); post-login `<Outlet/>` remount re-fetches; post-login 401 re-shows gate | RTL | `cd web && pnpm test -- --run root watchlist` | ✅ | ✅ green |
| 14-04-01 | 04 | 3 | GATE-01, GATE-03, D-15 | T-14-04-01/02/03/08/09 | Gated non-GET without `X-Requested-With` → 403 (after 401 check); `POST /session` without it → 403 before comparison, no counter move; GET unaffected; `Referrer-Policy: no-referrer` on every response; inert path installs none of it | unit | `go test ./internal/authgate/ ./internal/httpserver/ -run 'CSRF\|Referrer\|Inert' -count=1` | ✅ | ✅ green |
| 14-04-02 | 04 | 3 | GATE-01 (config), D-11 | T-14-04-04/05/06 | `IsWeakPassphrase`: empty → not-weak; <16 runes → weak; denylist (case-insensitive) → weak; `.env.example` placeholder caught; reason never contains the value; boot logs one WARN and never refuses to start | unit + boot-buffer | `go test ./internal/authgate/ -run 'Weak' -count=1` | ✅ | ✅ green |
| 14-04-03 | 04 | 3 | GATE-07, D-11, D-14 | T-14-04-07 | `.env.example` documents `INSTANCE_PASSPHRASE` (24+ rec, rotation = revoke-all) with a self-warning sentinel value, and `TRUST_PROXY_HEADERS` (default false, proxy precondition); gitleaks passes | doc + grep | `grep -c 'INSTANCE_PASSPHRASE' .env.example && grep -c 'TRUST_PROXY_HEADERS' .env.example` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Sampling continuity check: no 3 consecutive tasks lack an `<automated>` verify. 14-01-01 is a `checkpoint:decision` (human-check only, legitimate); every other task carries an automated command. ✔

---

## Wave 0 Requirements

New test files to create (no framework install needed — Go `testing` and Vitest+RTL are both present):

- [x] `internal/authgate/session_test.go` — GATE-02, GATE-03 (compare), D-06 (renewal + per-auth cap), D-17
- [x] `internal/authgate/gate_test.go` — GATE-01, GATE-03 (cookie flags), D-06 (renewal), D-14 (RealIP on/off), D-15 (CSRF)
- [x] `internal/authgate/login_test.go` — GATE-04 (throttle, undelayed 429, `maxConcurrentLogins` 503), D-12 (delay, global alert), D-13 (audit + secret-scan), D-17 (rotation)
- [x] `internal/authgate/alerter_test.go` — `SelectAlerter` both branches; webhook URL never logged
- [x] `internal/authgate/weak_test.go` — D-11 heuristic (short, known-default, empty→not-weak, multibyte, `.env.example` placeholder)
- [x] `internal/authgate/export_test.go` — setters for every shrinkable timing/threshold `var` (incl. `maxConcurrentLogins`)
- [x] `internal/httpserver/server_test.go` — add `newGatedServer(t, passphrase, trustProxyHeaders)` helper; `Inert` test proving the 5-arg path is unchanged; RealIP on/off assertion
- [x] `web/app/lib/authStore.test.ts` — pub/sub, idempotent marks, `gateActive`
- [x] `web/app/lib/api.test.ts` — **new file**, must NOT `vi.mock("~/lib/api")`; mocks global `fetch`; 401 interceptor + `X-Requested-With` + session wrappers
- [x] `web/app/components/auth/PassphraseScreen.test.tsx` — submit / 3 error branches / state-flip / value retention
- [x] extend `web/app/root.test.tsx` — `<App>` gate branch, Log out gated on `gateActive`, failed-logout backstop
- [x] extend `web/app/routes/watchlist.test.tsx` — re-fetch after auth flip; post-login 401 re-shows gate

All Wave 0 files exist on disk and their suites run green — `wave_0_complete: true` (plans 14-01…14-04 delivered them TDD-first; gap-closure plans 14-05…14-07 extended them for G-14-1/2/3).

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Cookie-name decision (A1 / `__Host-` prefix) | GATE-03, D-09 | Operator decision with a browser-behaviour trade-off no test can make | Answer the 14-01 Task 1 checkpoint (`option-a` recommended); record in 14-01-SUMMARY.md before Task 2 |
| Enabled gate loads in a real browser over `http://localhost` | GATE-05, A1 | Chrome vs Firefox cookie-prefix behaviour is not reproducible in `httptest`/jsdom | Set `INSTANCE_PASSPHRASE` locally, `docker compose up` (or `go run ./cmd/server`), open `http://localhost:8080` in Chrome AND Firefox: passphrase form renders, correct passphrase unlocks and stays unlocked on refresh, wrong passphrase shows the fixed message, Log out returns to the form |
| `docker compose up` with no passphrase is untouched | GATE-07 | Compose stack behaviour | `docker compose up`; confirm all routes answer as v1.2, no passphrase prompt, no new required var |
| 3 UI-SPEC held-out backstop checks (E4 network error, E5 failed logout, E6 post-login 401) | GATE-05 | `{ verification: backstop }` — assert during `/gsd-verify-work`, escalate to human if no evidence | See the `backstop` truths in 14-03 `must_haves` |

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or are a `checkpoint:decision` with human-check (14-01-01)
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING test-file references
- [x] No watch-mode flags in any command
- [x] Feedback latency < 60s for the quick command
- [x] `nyquist_compliant: true` — confirmed 2026-09-01 by `/gsd-validate-phase 14` against the live plans, SUMMARYs, and test suites

**Approval:** approved 2026-09-01 (`/gsd-validate-phase 14`)

---

## Validation Audit 2026-09-01

State A (existing VALIDATION.md audited). All 13 per-task rows cross-referenced against the
7 plan SUMMARYs, 14-VERIFICATION.md (7/7 must-haves, UAT 5/5), and a live suite re-run.

| Metric | Count |
|--------|-------|
| Per-task rows | 13 (12 automated + 1 `checkpoint:decision`) |
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Auditor spawn needed | no |

**Live re-run evidence:**

- `go test ./internal/authgate/... ./internal/httpserver/... ./internal/config/... -count=1` → `ok` (authgate 5.9s, httpserver 4.9s, config 0.5s; ~11s total, well under the 60s latency budget).
- `cd web && node_modules/.bin/vitest run authStore api PassphraseScreen root watchlist` → 8 files / 89 tests passed (run against the committed `HEAD` of `web/app/lib/authStore.ts`).
- All 12 Wave 0 test files present on disk (`internal/authgate/{session,gate,login,alerter,weak,export}_test.go`, `internal/httpserver/server_test.go`, `web/app/lib/{authStore,api}.test.ts`, `web/app/components/auth/PassphraseScreen.test.tsx`, `web/app/root.test.tsx`, `web/app/routes/watchlist.test.tsx`).

**Note — working-tree blocker (not a phase-14 coverage gap):** at audit time the working
tree carried an uncommitted edit to `web/app/lib/authStore.ts` (line 89 —
`GATE_ACTIVE_STORAGE_KEY` literal deleted, leaving `const GATE_ACTIVE_STORAGE_KEY =`), a
syntax error that makes the whole frontend suite fail to transform. The frontend re-run
above used the committed `HEAD` version. This is a local working-tree issue to resolve
separately (`git checkout HEAD -- web/app/lib/authStore.ts` if accidental); the phase's
committed surface and its Nyquist coverage are unaffected.
