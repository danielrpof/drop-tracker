# Project Research Summary

**Project:** drop-tracker v1.3 Continuous Deployment
**Researched:** 2026-08-27
**Confidence:** HIGH

## Executive Summary

v1.3 delivers three integrated capabilities: automated SSH-based VPS deployment with health-gated auto-rollback (DPLY-01), a shared-passphrase instance gate, and PR coverage-delta reporting (CICD-13). The passphrase gate must land before or with the first public deploy; coverage-diff is independent and low-risk (report-only); deploy is last and heaviest.

**Key recommendation:** mandate expand/contract migration discipline. Once auto-rollback exists, every future migration must be backward-compatible for at least one release.

---

## Key Findings

### Recommended Stack

v1.3 adds zero new application dependencies. All are existing GitHub Actions and stdlib crypto.

**Feature 1 (Deploy):**
- appleboy/ssh-action v1.2.5
- Docker Compose v2 (VPS-side)
- /health endpoint + curl

**Feature 2 (Gate):**
- Stdlib only: crypto/hmac, crypto/sha256, crypto/subtle, encoding/base64
- Signed stateless session cookies
- Existing: caarlos0/env, golang.org/x/time/rate, chi middleware

**Feature 3 (Coverage):**
- fgrosse/go-coverage-report v1.3.1
- davelosert/vitest-coverage-report-action v2.12.2
- marocchino/sticky-pull-request-comment v3.0.5

## Implications for Roadmap

### Phase 1: Passphrase Instance Gate

**Rationale:** No external infra; testable locally. Must exist before first public deploy.
**Delivers:** authgate package, chi middleware, login/logout, SPA route guard, rate limiting
**Complexity:** MEDIUM
**Decisions:** Signing key separate or derived? Session TTL? Passphrase strength checks?

### Phase 2: PR Coverage-Diff Comment

**Rationale:** Independent, report-only, low risk.
**Delivers:** Baseline (artifact/cache), PR diffs, sticky comment
**Complexity:** LOW-MEDIUM
**Decisions:** Baseline storage (cache vs branch)? Patch coverage v1.3?

### Phase 3: VPS Deployment with Auto-Rollback

**Rationale:** Last — requires provisioned VPS + proxy + Gate merged.
**Delivers:** deploy job, SSH rollout, health gate, migration CI gate
**Complexity:** HIGH
**Must Decide First:** TLS/proxy choice (Caddy vs Cloudflare Tunnel) blocks first deploy.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versions verified 2026-08-27 |
| Features | HIGH | Verified against codebase and PROJECT.md |
| Architecture | HIGH | Verified directly; recommend phase regression test |
| Pitfalls | MEDIUM-HIGH | Well-documented; enforcement is main uncertainty |

**Overall: HIGH**

---

*Research completed: 2026-08-27*
*Ready for roadmap: yes*
