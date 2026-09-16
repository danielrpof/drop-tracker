---
phase: "19"
slug: "frontend-system-view"
# status lifecycle: draft (seeded by plan-phase) → validated (set by validate-phase §6)
# audit-milestone §5.5 distinguishes NOT-VALIDATED (draft) from PARTIAL (validated + nyquist_compliant: false) (#2117)
status: validated
nyquist_compliant: true
wave_0_complete: true
created: "2026-09-10"
---

# Phase 19 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | vitest 4.1.x (frontend), go test (backend, no backend files changed this phase) |
| **Config file** | `web/vitest.config.ts` |
| **Quick run command** | `corepack pnpm --dir web exec vitest run --coverage.enabled=false <file>` |
| **Full suite command** | `corepack pnpm --dir web test` |
| **Estimated runtime** | ~19s (205 tests / 16 files) |

---

## Sampling Rate

- **After every task commit:** Run `corepack pnpm --dir web exec vitest run --coverage.enabled=false <file>`
- **After every plan wave:** Run `corepack pnpm --dir web test`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~20 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 19-01-01 | 01 | 1 | SYS-01, SYS-03 | unit/route | `vitest run app/routes/system.test.tsx` | ✅ | ✅ green |
| 19-01-02 | 01 | 1 | SYS-01, SYS-03 | unit | `vitest run app/lib/api.test.ts app/root.test.tsx` | ✅ | ✅ green |
| 19-02-01 | 02 | 2 | SYS-01, SYS-02 | unit | `vitest run app/lib/format.test.ts` | ✅ | ✅ green |
| 19-02-02 | 02 | 2 | SYS-01, SYS-02 | unit | `vitest run app/lib/format.test.ts` | ✅ | ✅ green |
| 19-03-01 | 03 | 2 | SYS-01, SYS-02 | unit | `vitest run app/lib/sources.test.ts` | ✅ | ✅ green |
| 19-03-02 | 03 | 2 | SYS-01, SYS-02 | visual/build | `vite build --mode production` | ✅ | ✅ green |
| 19-03-03 | 03 | 2 | SYS-01, SYS-02 | component | `test -f web/app/components/ui/table.tsx` | ✅ | ✅ green |
| 19-04-01 | 04 | 3 | SYS-01, SYS-02, SYS-03 | unit | `vitest run app/components/system/OutcomeBadge.test.tsx` | ✅ | ✅ green |
| 19-04-02 | 04 | 3 | SYS-01, SYS-02, SYS-03 | unit/route | `vitest run app/routes/system.test.tsx` | ✅ | ✅ green |
| 19-04-03 | 04 | 3 | SYS-01, SYS-02, SYS-03 | unit/route | `vitest run app/routes/system.test.tsx app/components/system/OutcomeBadge.test.tsx` | ✅ | ✅ green |
| 19-05-01 | 05 | 4 | SYS-02, SYS-03 | unit/route | `vitest run app/routes/system.test.tsx` | ✅ | ✅ green |
| 19-05-02 | 05 | 4 | SYS-02, SYS-03 | unit/route | `vitest run app/routes/system.test.tsx` | ✅ | ✅ green |
| 19-05-03 | 05 | 4 | SYS-02, SYS-03 | phase gate | `pnpm test && go build ./... && go vet ./... && golangci-lint run && make coverage-gate && make sqlc-check && make web` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

Confirmed at audit time: `corepack pnpm --dir web test` → 16 test files passed, 205 tests passed, 0 failed. `app/routes/system.tsx` at 90.9% line coverage, `app/components/system/*` at 100% line coverage.

---

## Wave 0 Requirements

*Existing infrastructure (vitest + Testing Library, already established in prior phases) covers all phase requirements — no Wave 0 scaffolding was needed.*

---

## Manual-Only Verifications

*None — every task carries an automated `<verify>` command and its referenced test file exists and passes. The one behavior that automated tests structurally cannot cover (real MusicBrainz/Deezer data, a real Postgres instance, and the passphrase gate's cookie/logout flow in an actual browser) is covered separately by `19-UAT.md` Test 1 ("Live gated-instance smoke test"), which passed.*

---

## Validation Sign-Off

- [x] All tasks have `<automated>` verify or Wave 0 dependencies
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none were missing)
- [x] No watch-mode flags
- [x] Feedback latency < 20s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-09-11

## Validation Audit 2026-09-11

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
