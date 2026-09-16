---
phase: 07
slug: containerization-ci-cd-pipeline
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-12
---

# Phase 07 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| repo working tree -> Docker build context | Untrusted-for-secrets: anything not excluded by `.dockerignore` becomes an image layer | source, config, potential secrets |
| container process -> host kernel | A compromised in-container process must not hold host-equivalent privileges | process privilege level |
| container process -> public internet (MusicBrainz, Deezer, Discord) | Outbound TLS; certificate validation is the only integrity control | API requests/responses |
| host developer -> committed compose values | Anything written into `docker-compose.yml` is public in a public repo | dev-only DSN |
| developer working tree -> git commit object | The pre-commit hook is the last point at which a secret or lint regression can be stopped before it becomes history | staged diff |
| pre-commit hook auto-fix -> committed code | `--fix` rewrites source; the rewrite must be visible to the developer, not applied invisibly | source rewrites |
| local lint config -> CI lint config | A version or config skew makes a locally-green commit fail CI (or worse, the reverse) | lint config |
| pull-request author -> CI runner | PR-authored code executes on the runner; the token that job holds is the blast radius | GITHUB_TOKEN scope |
| third-party GitHub Action -> workflow execution | An action runs arbitrary code with the job's token; a re-pointed mutable tag is a silent code substitution | job token, workflow env |
| upstream base image / Go module -> built image | Transitive OS and language-level packages enter the artifact without review | image layers |
| source tree -> public repository | A committed secret in a public repo is disclosed the moment it is pushed | secrets |
| workflow job -> ghcr.io registry | The publish credential is the ambient `GITHUB_TOKEN`; whichever job holds it can write packages | image push |
| workflow job -> git refs on origin | `contents: write` allows creating tags, which are the version line consumers rely on | git tags |
| ghcr.io -> anonymous puller | The package is public (D-05); anything in the image is world-readable | published image |
| conventional-commit message -> computed version | An unvalidated commit message silently drives the published version number | version string |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-07-01 | Information Disclosure | Docker build context / image layers | high | mitigate | `.dockerignore` excludes `.env`, `.env.*`, `.git/`, `.planning/`; Dockerfile carries no `ENV`/`ARG` config value | closed |
| T-07-02 | Elevation of Privilege | container runtime process | high | mitigate | Fixed non-root `USER 10001:10001`, explicit `addgroup -g 10001` / `adduser -D -u 10001` | closed |
| T-07-03 | Tampering | outbound HTTPS to MusicBrainz / Deezer / Discord | high | mitigate | `apk add --no-cache ca-certificates` in final stage; TLS verification never disabled | closed |
| T-07-04 | Tampering | base image drift | medium | mitigate | All three build stages pinned to exact digests (`node:26-alpine3.24@sha256:...`, `golang:1.26.5-alpine3.24@sha256:...`, `alpine:3.24@sha256:...`) | closed |
| T-07-05 | Information Disclosure | committed docker-compose values | medium | accept | Dev-only `drop_tracker:drop_tracker` DSN grants access only to a local throwaway container; real secrets stay in gitignored `.env` | closed |
| T-07-06 | Denial of Service | container health reporting | low | accept | HEALTHCHECK is a local-dev signal only; no external monitor depends on it in this phase | closed |
| T-07-07 | Tampering | golangci-lint pre-commit `--fix` pass | medium | accept | `--fix` rewrites land in the working tree for developer re-staging; `git diff --exit-code` verification gate confirms no silent rewrite | closed |
| T-07-08 | Repudiation | lint suppression | medium | mitigate | Every `//nolint` in the codebase names its linter and carries a same-line reason (verified: 5 occurrences, all `//nolint:gosec // <reason>`); `.golangci.yml` keeps `default: standard` | closed |
| T-07-09 | Tampering | pre-commit hook supply chain | medium | mitigate | Both hook repos pin exact released tags (`v8.30.1` gitleaks, `v2.12.2` golangci-lint) in `.pre-commit-config.yaml` | closed |
| T-07-10 | Elevation of Privilege | local hook bypass (`--no-verify`) | low | accept | CI gitleaks job is the non-bypassable backstop for the same tool | closed |
| T-07-11 | Tampering | third-party GitHub Actions | high | mitigate | Every `uses:` in `full-pipeline.yml` pinned to a 40-hex commit SHA (checkout, setup-go, golangci-lint-action, gitleaks-action, trivy-action, action-semantic-pull-request, setup-buildx-action, build-push-action, upload/download-artifact, login-action — verified, all SHA-pinned with version comments) | closed |
| T-07-12 | Elevation of Privilege | PR-triggered jobs | high | mitigate | Workflow-level `permissions: contents: read`; `build-scan` re-declares `contents: read`; only `pr-title` (scoped `pull-requests: read`) and `release` (guarded, main-only) hold any elevation | closed |
| T-07-13 | Tampering | vulnerable base image or transitive dependency | high | mitigate | Trivy `severity: CRITICAL,HIGH`, `exit-code: '1'` on both `trivy-fs` and the loaded image in `build-scan`, which `needs:` all four source jobs; a real HIGH finding (CVE-2026-56852) was fixed by dependency bump, not suppressed | closed |
| T-07-14 | Information Disclosure | committed secret reaching a public repo | high | mitigate | `gitleaks-action` v3 runs as its own job on every push/PR with `fetch-depth: 0` | closed |
| T-07-15 | Repudiation | silent vulnerability suppression | medium | mitigate | No `.trivyignore` exists in the repo — confirms it is created only when a finding has no upstream fix; the one real finding encountered was fixed upstream instead | closed |
| T-07-SC | Tampering | supply chain (package installs) | high | mitigate | No new entries added to `go.mod`/`web/package.json`/`web/pnpm-lock.yaml` by this phase; the two post-review commits (`1836652`, `f6647ec`) only bump existing, already-audited dependency versions to close Trivy CVE findings | closed |
| T-07-16 | Denial of Service | runner minutes consumed by duplicate runs | low | accept | Triggering on every push duplicates work when a branch push and its PR coexist; concurrency group cancels superseded PR runs | closed |
| T-07-17 | Elevation of Privilege | ghcr.io push credentials | high | mitigate | `packages: write` / `contents: write` scoped to the `release` job only, guarded by `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` — unreachable from PR events | closed |
| T-07-18 | Tampering | unscanned artifact reaching the registry | high | mitigate | `release` declares `needs: [build-scan]`; no alternate code path to the registry exists | closed |
| T-07-19 | Repudiation | version tag with no corresponding image | medium | mitigate | `git tag` / `git push origin` steps run last, after the image push and SBOM generation steps | closed |
| T-07-20 | Tampering | mutable published tag | medium | mitigate | Exactly one immutable `$VERSION` tag is pushed; no `latest` or rolling tag exists in the workflow | closed |
| T-07-21 | Spoofing | unvalidated commit message driving the version | medium | mitigate | `pr-title` job validates the PR title via `action-semantic-pull-request`; `svu next`/`svu current` step fails loudly (`exit 1`) when no version bump can be computed | closed |
| T-07-22 | Information Disclosure | public image contents | low | accept | Image contains only compiled output of an already-public repo; secrets are kept out of every layer by T-07-01's mitigation. Public visibility is the explicit intent of D-05 | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-07-01 | T-07-05 | Committed dev-only Postgres DSN (`drop_tracker:drop_tracker`) grants access only to a local throwaway container; identical credential already existed in the pre-phase `postgres:` compose block | Phase 07 plan (07-01) | 2026-08-12 |
| AR-07-02 | T-07-06 | HEALTHCHECK is a local-dev signal only; no external monitor consumes it in this phase | Phase 07 plan (07-01) | 2026-08-12 |
| AR-07-03 | T-07-07 | Upstream golangci-lint hook applies `--fix` by design; rewrite is working-tree-visible and gated by a `git diff --exit-code` verification step | Phase 07 plan (07-02) | 2026-08-12 |
| AR-07-04 | T-07-10 | `--no-verify` is a git-level escape no local hook can close; CI gitleaks is the non-bypassable backstop | Phase 07 plan (07-02) | 2026-08-12 |
| AR-07-05 | T-07-16 | Every-push trigger duplicates work when a branch push and its open PR coexist; concurrency group cancels superseded PR runs, so cost is bounded | Phase 07 plan (07-03) | 2026-08-12 |
| AR-07-06 | T-07-22 | Public image contents are the explicit intent of D-05 (public ghcr.io package); secrets are excluded by T-07-01's mitigation | Phase 07 plan (07-04) | 2026-08-12 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-12 | 23 | 23 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth verification against Dockerfile, .dockerignore, .pre-commit-config.yaml, .golangci.yml, .github/workflows/full-pipeline.yml) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-12
