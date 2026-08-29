# Requirements: drop-tracker

**Defined:** 2026-08-27
**Core Value:** A single Go binary that reliably detects and notifies on new releases for watched artists, built and shipped through a CI/CD pipeline rigorous enough to demonstrate real DevOps practice.

Milestones v1.0–v1.2 are shipped; their requirements are archived under `.planning/milestones/`. This file scopes **v1.3 Continuous Deployment**.

---

## v1.3 Requirements — Continuous Deployment

Automate shipping the app to a self-hosted VPS on every merge to main, put a passphrase gate in front of the public instance, keep rollback safe, and close the last CI reporting gap. Research: `.planning/research/SUMMARY.md` (+ STACK / FEATURES / ARCHITECTURE / PITFALLS).

### Deployment

- [ ] **DPLY-01**: On merge to main, after the release job publishes the versioned image to ghcr.io, a deploy job connects to the VPS over SSH and rolls the running container forward to that image tag
- [ ] **DPLY-02**: The deploy verifies the new container by polling `/health`, and on failure (pull error, crash loop, or non-200 within the retry budget) automatically restores the previously-running image and re-verifies
- [ ] **DPLY-03**: Concurrent deploys are serialized — a deploy triggered while another is in progress queues rather than cancelling or overlapping the in-flight one
- [ ] **DPLY-04**: The production compose file and rollout script are versioned in the repo; the real secrets (`.env`) and the pinned image tag live only on the VPS and are never committed
- [ ] **DPLY-05**: The deploy verifies the SSH host key against a pinned fingerprint (no blind accept); all deploy secrets are supplied via GitHub Actions secrets scoped to a `production` environment
- [ ] **DPLY-06**: The deploy job runs only on push to main — never on pull requests, and never on forks
- [ ] **DPLY-07**: A documented, repeatable VPS provisioning runbook/wizard covers Docker install, directory layout, prod compose + `.env` placement, SSH key + host-fingerprint capture, GitHub secrets, the TLS reverse proxy, a firewall allowlist during provisioning (so the instance is never public-and-pre-gate), setting `TRUST_PROXY_HEADERS=true` only once the proxy is live and the container port is unpublished, the passphrase-rotation revoke-all procedure, and a Postgres backup + restore procedure (recovery path when an image rollback also needs the schema restored)
- [ ] **DPLY-08**: The deployed instance is reachable over HTTPS via a reverse proxy on the VPS

### Access Gate

- [ ] **GATE-01**: When a passphrase is configured, all data/API routes require a valid session cookie and return `401` without one; `/health`, the session-login endpoint, and the static SPA shell stay publicly reachable
- [x] **GATE-02**: A user authenticates by submitting the correct passphrase once; a signed, stateless session cookie then keeps that browser authenticated across requests and across app restarts/redeploys
- [ ] **GATE-03**: The session cookie is HMAC-signed, `HttpOnly`, `Secure`, `SameSite=Lax`, and has a bounded lifetime; passphrase comparison is constant-time
- [x] **GATE-04**: The login endpoint is rate-limited per client IP to bound brute-force attempts (client IP comes from `X-Forwarded-For` only when `TRUST_PROXY_HEADERS` is set — the Phase 17 topology; otherwise the direct peer address, so a pre-proxy deploy can't be spoofed). The throttle rejection is undelayed and login-handler concurrency is bounded so a distributed flood can't exhaust goroutines/connections
- [ ] **GATE-05**: The SPA detects a `401` from the API, presents a passphrase login form, and resumes normal operation after a successful login
- [ ] **GATE-06**: A user can log out, invalidating the session on that browser
- [ ] **GATE-07**: When no passphrase is configured the gate is inert — every route behaves exactly as it did before v1.3, so local dev, docker-compose, and the existing test suite need no passphrase

### Migration Safety

- [ ] **MGRT-01**: A CI check boots the previously-released image against the current branch's schema and fails the build if the older binary cannot start and stay healthy (backward-compatibility / N-1 check)
- [ ] **MGRT-02**: The repo documents the expand/contract migration rule (additive-only per release, destructive changes split across releases, no blocking DDL in boot migrations) as a standing constraint

### CI/CD Pipeline

- [ ] **CICD-13**: On a pull request from a same-repo branch, CI posts and updates in place a single comment reporting backend and frontend coverage totals plus their delta versus the main-branch baseline; the comment is report-only and never blocks the merge
- [ ] **CICD-14**: Main-branch pipeline runs publish their coverage results as the baseline the PR comment diffs against, and the comment degrades gracefully (absolute numbers only) when no baseline is available

## Future Requirements

Deferred, tracked, not in the v1.3 roadmap.

### Deployment / Operations

- **OPS-04**: Automated, scheduled Postgres backups with off-box retention for the VPS. The *basic* backup + restore procedure is folded into DPLY-07 (Phase 17) as the recovery path for an irreversible migration; OPS-04 remains for scheduling, off-box copies, and retention policy beyond a manual runbook
- **OPS-05**: ghcr.io image-retention / cleanup job so old tags don't accumulate
- **DPLY-09**: Near-zero-downtime deploy (reverse-proxy connection draining or a second short-lived container) instead of the accepted brief swap gap

### Access Gate

- **GATE-08**: Session signing-key rotation / key-versioning without logging every browser out

### CI/CD Pipeline

- **CICD-15**: Patch/diff-level coverage in the PR comment (uncovered new lines), not just file/total deltas

## Out of Scope

| Feature | Reason |
|---------|--------|
| Multi-user auth / accounts / per-user profiles / real-time cross-device sync | Evaluated in v1.3 scoping and rejected; single-operator self-host + server-side Postgres already covers isolation and multi-device access, and OAuth/session/RBAC is an orthogonal spike that doesn't showcase the CI/CD goal |
| Blue-green / K8s / Swarm / multi-host orchestration | One container on one host; a 2–5s swap gap cushioned by graceful SIGTERM shutdown is acceptable at this scale |
| Terraform / IaC-provisioned VPS | The provisioning runbook/wizard (DPLY-07) is manual-but-documented; full IaC stays past the current DevOps ceiling |
| `pull_request_target` for coverage comments on fork PRs | Known security anti-pattern; fork PRs simply skip the comment (same-repo branches only) |
| Codecov / other coverage SaaS | Against the project's self-hosted, minimal-external-dependency ethos; baseline is a workflow artifact/cache |
| Server-side session store (Postgres `sessions` table) | Stateless signed cookie fits the constant-redeploy model and adds zero dependencies; revisit only if real multi-user accounts ever land |

## Traceability

Mapped during roadmap creation (2026-08-27). See `.planning/ROADMAP.md` for each phase's goal and success criteria.

| Requirement | Phase | Status |
|-------------|-------|--------|
| GATE-01 | Phase 14 | Pending |
| GATE-02 | Phase 14 | Complete |
| GATE-03 | Phase 14 | Pending |
| GATE-04 | Phase 14 | Complete |
| GATE-05 | Phase 14 | Pending |
| GATE-06 | Phase 14 | Pending |
| GATE-07 | Phase 14 | Pending |
| CICD-13 | Phase 15 | Pending |
| CICD-14 | Phase 15 | Pending |
| MGRT-01 | Phase 16 | Pending |
| MGRT-02 | Phase 16 | Pending |
| DPLY-01 | Phase 17 | Pending |
| DPLY-02 | Phase 17 | Pending |
| DPLY-03 | Phase 17 | Pending |
| DPLY-04 | Phase 17 | Pending |
| DPLY-05 | Phase 17 | Pending |
| DPLY-06 | Phase 17 | Pending |
| DPLY-07 | Phase 17 | Pending |
| DPLY-08 | Phase 17 | Pending |

**Coverage:**

- v1.3 requirements: 21 total
- Mapped to phases: 21 ✓
- Unmapped: 0 ✓

**By phase:**

| Phase | Requirements | Count |
|-------|--------------|-------|
| 14 — Instance Passphrase Gate | GATE-01 … GATE-07 | 7 |
| 15 — PR Coverage-Diff Comment | CICD-13, CICD-14 | 2 |
| 16 — Rollback-Safe Migrations | MGRT-01, MGRT-02 | 2 |
| 17 — Automated VPS Deploy with Health-Gated Rollback | DPLY-01 … DPLY-08 | 8 |

No orphaned requirements; no requirement mapped to more than one phase.

---
*Requirements defined: 2026-08-27*
*Last updated: 2026-08-27 after v1.3 roadmap creation (traceability populated)*
