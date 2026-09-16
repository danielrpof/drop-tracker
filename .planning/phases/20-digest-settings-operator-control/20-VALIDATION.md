---
phase: "20"
slug: "digest-settings-operator-control"
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-14"
---

# Phase 20 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | `go test` (real Postgres, `internal/testutil.NewIsolatedTestPool`) + `vitest` (SPA) |
| **Config file** | none new — existing `go.mod` build tags and `web/vitest.config.ts` |
| **Quick run command** | `go test ./internal/settings/... ./internal/httpserver/... -run TestSettings` / `corepack pnpm --dir web exec vitest run DigestSettings.test.tsx system.test.tsx api.test.ts` |
| **Full suite command** | `make test` (backend) + `corepack pnpm --dir web test` (frontend) |
| **Estimated runtime** | ~40s backend (real-Postgres suite), ~15s frontend (225 vitest cases) |

---

## Sampling Rate

- **After every task commit:** targeted `go test`/`vitest` run scoped to the touched package or component
- **After every plan wave:** full backend (`make test`) and, for UI plans, full frontend (`corepack pnpm --dir web test`) suite
- **Before `/gsd-verify-work`:** Full suite must be green — confirmed in 20-04's Definition-of-Done run (24/24 Go packages, 225/225 vitest cases, `coverage-gate` 90.72%)
- **Max feedback latency:** ~60s (backend suite is the long pole)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 20-01-01 | 01 | 1 | DGST-01, DGST-03, DGST-04 | T-20-01 / T-20-06 | Gated route wired inside `registerDataRoutes`; migration paired with a down file | integration | `go test ./internal/httpserver/... -run TestSettings_RoundTrip` | ✅ | ✅ green |
| 20-01-02 | 01 | 1 | DGST-01, DGST-02, DGST-03, DGST-04 | T-20-02 / T-20-03 | Singleton row enforced by `CHECK (id=1)`; idempotent update; watermark untouched | integration | `go test ./internal/settings/... -run TestService` | ✅ | ✅ green |
| 20-02-01 | 02 | 2 | DGST-02, DGST-04 | T-20-09 / T-20-11 | Malformed/oversize PUT bodies rejected 400/4xx, stored row unchanged | integration | `go test ./internal/httpserver/... -run TestSettings_PutRejects` | ✅ | ✅ green |
| 20-02-02 | 02 | 2 | DGST-01 | T-20-07 / T-20-08 / T-20-10 | 401 without session, 403 without CSRF header, 503 when unconfigured, no secret leak | integration | `go test ./internal/httpserver/... -run "TestSettings_(Get\|Put)Gated\|TestSettings_Ungated\|TestSettings_NotConfigured\|TestSettings_NoLeak"` | ✅ | ✅ green |
| 20-03-01 | 03 | 2 | DGST-01 | T-20-12 / T-20-16 | Switch toggle end-to-end with instant-apply save, re-entrancy guard | unit | `corepack pnpm --dir web exec vitest run DigestSettings.test.tsx` | ✅ | ✅ green |
| 20-03-02 | 03 | 2 | DGST-01, DGST-16 | T-20-13 / T-20-14 | Wrapper request shape, CSRF header, 401/400 propagation, shared mount/refresh fetch | unit | `corepack pnpm --dir web exec vitest run api.test.ts system.test.tsx` | ✅ | ✅ green |
| 20-04-01 | 04 | 3 | — (infra) | T-20-17 | Vendored `select.tsx` with no dependency-manifest drift | other | `git diff --name-only -- web/package.json web/pnpm-lock.yaml` (expect empty) | ✅ | ✅ green |
| 20-04-02 | 04 | 3 | DGST-02, DGST-16 | T-20-18 / T-20-19 | Cadence control (disabled-but-populated when off) + "Never sent yet"/`<time>` last-sent row | unit | `corepack pnpm --dir web exec vitest run DigestSettings.test.tsx` | ✅ | ✅ green |
| 20-04-03 | 04 | 3 | — (infra) | T-20-20 | Embedded SPA bundle rebuilt from phase source; full Definition of Done green | other | `make web && make test && make coverage-gate` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements — `internal/testutil.NewIsolatedTestPool` (real-Postgres isolation) and the project's existing Vitest setup were already in place from earlier phases; no new test scaffolding was needed.

---

## Manual-Only Verifications

All phase behaviors have automated verification. `20-VERIFICATION.md`'s "Human Verification Required" section independently confirmed this (re-ran the backend and frontend suites live rather than trusting SUMMARY claims); this audit additionally located `internal/settings/settings_test.go#TestService_UpdateRoundTripsThroughPostgres` — a two-independent-`Service`-instance read-back test that automates DGST-03 ("survives process restart, Postgres-backed") directly. That test exists and passes but was not tagged with a `coverage:` id in `20-01-SUMMARY.md`; noted here as a documentation completeness gap, not a test gap — no new test was needed.

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none found — no MISSING references)
- [x] No watch-mode flags
- [x] Feedback latency < 60s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-14
