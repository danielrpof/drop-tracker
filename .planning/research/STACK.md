# Stack Research

**Domain:** CI/CD continuous deployment for a single-binary Go+React service (SSH-to-VPS deploy, passphrase auth gate, PR coverage-diff reporting)
**Researched:** 2026-08-27
**Confidence:** MEDIUM-HIGH (action/library versions verified against GitHub API + Go module proxy on 2026-08-27; integration patterns are well-established but not yet exercised in this repo)

> Scope note: this milestone (v1.3) adds to a shipped v1.2 app. The locked stack in `CLAUDE.md` (chi, pgx/v5, sqlc, golang-migrate boot-time, robfig/cron, React+Vite `go:embed`, caarlos0/env/v11, slog + go-chi/httplog, ghcr.io via ambient `GITHUB_TOKEN`) is **not** re-evaluated here. This document only covers what the three new features need.

---

## Recommended Stack

### Feature 1 — GitHub Actions VPS deploy job (DPLY-01)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `appleboy/ssh-action` | **v1.2.5** (commit `0ff4204d59e8e51228ff73bce53f80d53301dee2`) | Run the deploy+health+rollback shell script on the VPS over SSH from the `deploy` job | De-facto standard for "SSH in and run a script" in Actions. Bundles key setup, `known_hosts`/`fingerprint` host verification, `command_timeout`, and typed `envs:` passthrough — all the boilerplate you would otherwise hand-write around `ssh`. The deploy logic lives in **one greppable script**, not in action inputs, which matches this repo's "small, greppable config" preference. Auth stays secret-only (`SSH_HOST`, `SSH_USER`, `SSH_KEY`, `SSH_FINGERPRINT`, `GATE_PASSPHRASE` as GitHub Actions secrets) — no new registry credential, consistent with the ghcr.io/`GITHUB_TOKEN` constraint. |
| Docker Compose v2 (`docker compose`, VPS-side) | whatever ships with the VPS Docker Engine (v2.x plugin) | Pull the new pinned image tag and recreate the container | Already the documented local-dev path (`docker-compose.yml`). Re-using it on the VPS keeps one compose definition mental-model. The image tag is the **only** thing that changes between deploys. |
| `curl` or `wget` (VPS-side, already in the runtime image via busybox) | n/a | Poll `http://127.0.0.1:8080/health` after `up -d` | `/health` already returns 503 until Postgres + migrations are ready (see `Dockerfile` HEALTHCHECK + `internal/httpserver/health.go`). The deploy script reuses that exact signal as the promote/rollback gate. |

**No dedicated "deployment" action or tool** (no Kamal, no `docker rollout`, no Ansible, no Watchtower). PROJECT.md explicitly caps this milestone at "VPS SSH deploy" and rejects K8s/Helm/Terraform. A ~40-line bash script invoked through `appleboy/ssh-action` is the entire mechanism.

#### Compose-based image swap + rollback structure

The VPS holds a small deploy directory (provisioned once, out of band):

```
/opt/drop-tracker/
  docker-compose.yml      # app + postgres; app image = ghcr.io/danielrpof/drop-tracker:${DROP_TRACKER_TAG}
  .env                    # DROP_TRACKER_TAG=v1.2.3  + real runtime secrets (DATABASE_URL, DISCORD_WEBHOOK_URL, GATE_PASSPHRASE, GATE_SIGNING_KEY, ...)
  .env.previous           # written by the deploy script before each swap
```

`docker-compose.yml` on the VPS references the image as `ghcr.io/danielrpof/drop-tracker:${DROP_TRACKER_TAG}` and Compose interpolates `DROP_TRACKER_TAG` from `.env`.

Deploy script (runs on the VPS via `appleboy/ssh-action`, receives `NEW_TAG` through `envs:`):

1. `cd /opt/drop-tracker`
2. Capture rollback target: `grep '^DROP_TRACKER_TAG=' .env > .env.previous` (or read current running image digest).
3. `echo "GHCR_TOKEN" | docker login ghcr.io -u <actor> --password-stdin` — **only needed if the ghcr package is private**; the existing package is public and pullable without auth, so this step can be omitted while it stays public. (If it is ever made private, pass a `packages:read` PAT or the job's `GITHUB_TOKEN` as a secret — the VPS cannot see the workflow's ambient token.)
4. Write new tag: `sed -i "s/^DROP_TRACKER_TAG=.*/DROP_TRACKER_TAG=${NEW_TAG}/" .env`
5. `docker compose pull app && docker compose up -d app`
6. Health poll: loop up to ~60–90s, `curl -fsS http://127.0.0.1:8080/health` until success.
7. **On success:** prune old images (`docker image prune -f`), exit 0.
8. **On failure:** restore `.env` from `.env.previous`, `docker compose up -d app` (old tag is still in the local image cache), re-poll `/health`, then `exit 1` so the Actions job goes red.

Key properties:
- **Pinned tag, never `:latest`** — rollback is deterministic because the previous tag's image is still in the VPS local cache and its exact identifier is recorded.
- **Boot-time migrations + expand/contract** (already a PROJECT.md constraint) make step 8 safe: the old binary must still run against the new schema. This is a *discipline* requirement on migration authoring, not a tooling choice.
- **Compose has no native rollback** — the `.env` tag-swap + `.env.previous` restore *is* the rollback. This is the standard minimal pattern; anything fancier (blue/green, two compose projects, `docker rollout`) is out of scope per PROJECT.md.
- Single-container-at-a-time recreate means a few seconds of downtime per deploy. Acceptable for a single-operator portfolio app; note it, don't engineer around it.

#### Workflow wiring

New `deploy` job appended after `release`:

```yaml
deploy:
  needs: [release]
  if: github.event_name == 'push' && github.ref == 'refs/heads/main'
  runs-on: ubuntu-latest
  concurrency:
    group: deploy-${{ github.ref }}
    cancel-in-progress: false        # never interleave two deploys
  steps:
    - uses: appleboy/ssh-action@0ff4204d59e8e51228ff73bce53f80d53301dee2  # v1.2.5
      with:
        host: ${{ secrets.SSH_HOST }}
        username: ${{ secrets.SSH_USER }}
        key: ${{ secrets.SSH_KEY }}
        fingerprint: ${{ secrets.SSH_FINGERPRINT }}   # pin host key, do NOT skip verification
        command_timeout: 5m
        envs: NEW_TAG
        script: |
          set -euo pipefail
          /opt/drop-tracker/deploy.sh
      env:
        NEW_TAG: ${{ needs.release.outputs.version }}
```

This requires `release` to **expose the computed version as a job output**. Today `release` only writes `VERSION` to `$GITHUB_ENV` (step-local). Add:
```yaml
release:
  outputs:
    version: ${{ steps.svu.outputs.version }}
```
and have the svu step do `echo "version=$NEXT" >> "$GITHUB_OUTPUT"`. Small, contained change to the existing job.

**Deploy script (`deploy.sh`) should be committed to the repo** and `scp`'d / `git pull`'d onto the VPS as part of provisioning, so it is version-controlled and reviewable rather than living only on the server.

---

### Feature 2 — Instance passphrase gate (chi middleware)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `encoding/base64`, `net/http` (**stdlib only**) | Go 1.26 | Stateless signed-cookie auth: `POST /login` compares the submitted passphrase with `subtle.ConstantTimeCompare`, on success sets an HMAC-SHA256-signed cookie carrying an expiry; middleware verifies the signature + expiry on every request except `/health` | **Zero new dependencies** — directly honors CLAUDE.md's "minimal dependency footprint" and matches this repo's established hand-roll pattern (`internal/discord`, `internal/artistart`, the channel-semaphore worker pool, the hand-rolled combobox). The cookie carries **no secret payload** (just `exp` + version), so signing (HMAC) is sufficient and encryption is unnecessary. Being **stateless** is the decisive advantage here: the app redeploys on *every merge to main*, so any in-memory server-side session store would log every user out on every deploy. |

**Design (all in a new `internal/authgate` package):**
- Config via existing `caarlos0/env/v11` struct: `GATE_PASSPHRASE` (required) and `GATE_SIGNING_KEY` (required, ≥32 bytes random). Keep the signing key **separate** from the passphrase so rotating the passphrase (e.g. after sharing it) does not silently depend on cookie-invalidation semantics — though deriving `key = SHA256("drop-tracker-gate-v1" || passphrase)` is an acceptable single-secret variant if you accept that a passphrase change logs everyone out.
- Cookie: `Name=dt_gate`, value = `base64url(exp_unix) + "." + base64url(HMAC_SHA256(key, "v1|" + exp_unix))`. Attributes: `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, `Max-Age` matching `exp` (suggest 30 days).
- Middleware: skip when `r.URL.Path == "/health"`; also allow-list the login page assets and `POST /login` itself. On missing/invalid/expired cookie → for SPA/API routes return `401`; for top-level navigations serve/redirect to a minimal login page (can be a tiny embedded HTML form or a React route).
- Verify with `hmac.Equal` (constant-time). Passphrase check with `subtle.ConstantTimeCompare`. Add a small fixed delay or rate-limit on `/login` failures to blunt brute force (a per-IP token bucket with the already-present `golang.org/x/time/rate` is enough; not strictly required for v1).
- Redaction: `GATE_PASSPHRASE` / `GATE_SIGNING_KEY` must be added to the existing slog secret-redaction helper's key list.

**Why not a session library:** see "What NOT to Use". The short version — for *one shared passphrase* with *no user accounts, no per-user data, no server-side logout-all requirement*, a session manager is strictly more moving parts (a store, GC, session IDs, serialization) for zero benefit, and the in-memory variants break under this project's constant-redeploy model.

---

### Feature 3 — PR coverage-diff comment (CICD-13)

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| `fgrosse/go-coverage-report` | **v1.3.1** (commit `e432de98ee94a276e8f666d25bfd76347665f75b`) | Post a per-package Go coverage-delta table as a sticky PR comment | Purpose-built, Go-only, no third-party service. Solves the annoying part — fetching the **main-branch baseline** `coverage.txt` from a prior workflow run's artifact — for you (`github-baseline-workflow-ref` input). Maps 1:1 onto the existing backend `make coverage-gate` side of the split coverage gate. |
| `davelosert/vitest-coverage-report-action` | **v2.12.2** (pin `@v2` per its own docs, or SHA `8b157684c6a6b259b97d45e72b44242865c0f6a5`) | Post the frontend Vitest coverage summary (+ optional diff) as a second sticky PR comment | The established Vitest equivalent. Reads `coverage-summary.json`; shows a trend/delta when handed a baseline via `json-summary-compare-path`. Maps 1:1 onto the existing frontend `vitest coverage.thresholds` side of the split gate. |
| `marocchino/sticky-pull-request-comment` | **v3.0.5** (commit `5770ad5eb8f42dd2c4f34da00c94c5381e49af88`) | *(only if you choose the custom-script route — see Alternatives)* create/update one idempotent PR comment from a hand-built markdown table | Tiny, single-responsibility, widely used. This is what `fgrosse/go-coverage-report` uses under the hood anyway. |

**Recommendation: the two single-language actions (`fgrosse` + `davelosert`), not an all-in-one.** Rationale:
- The repo *already* split coverage into two independent, single-language mechanisms in Phase 09 (Makefile gate for Go, `vitest.config.ts` thresholds for frontend) and explicitly rejected a third-party coverage-gating action in favor of "one greppable threshold literal per side." Two focused reporting actions preserve that exact shape.
- Each action is single-purpose and its config is a handful of lines in `full-pipeline.yml` — greppable, no new config file, no new persistent concept.
- Downside: **two PR comments** instead of one. Acceptable and arguably clearer (backend vs frontend are reviewed independently). If one comment is strongly preferred, use the custom-script + `sticky-pull-request-comment` route below.

**Baseline plumbing (the real work of this feature):**
- Add a step on the **push-to-main** path of `full-pipeline.yml` that uploads the backend `coverage.txt` (from `make test-integration`) and the frontend `coverage/coverage-summary.json` as artifacts (e.g. names `code-coverage` and `frontend-coverage`).
- `fgrosse/go-coverage-report` then pulls that `code-coverage` artifact from the latest successful main run automatically.
- For `davelosert`, on the PR run add a step that downloads the latest main `frontend-coverage` artifact (via `actions/download-artifact` with `run-id` + `github-token`, or `dawidd6/action-download-artifact`) and pass its path as `json-summary-compare-path`. If cross-run download is judged too fiddly, ship the frontend comment **without** a delta first (summary table only) and add the compare later — it is still useful.
- Vitest config change: add `'json-summary'` and `'json'` to `coverage.reporter` in `web/vitest.config.ts` (currently only threshold-checking reporters are needed).

**Permissions / fork limitation (must be documented in the plan):**
- The coverage-comment job needs `permissions: pull-requests: write` (plus `contents: read`).
- **Fork PRs get a read-only `GITHUB_TOKEN`** — the comment post will fail on PRs from forks. This repo is a single-operator portfolio project where PRs come from branches in the same repo (the `release`/`deploy` jobs already assume `push` to `main`), so this is acceptable. **Do not** work around it with `pull_request_target` (well-known security footgun: runs untrusted PR code with a write token). If fork support is ever needed, use the `pull_request` (test, upload artifact) + `workflow_run` (download artifact, comment) two-workflow split.
- Put the coverage-comment work in its **own job** gated `if: github.event_name == 'pull_request'`, `needs: [test, frontend-test]`. It must **not** be added to `build-scan.needs` — a flaky comment post must never block the merge/release path.

---

## Installation

```bash
# Feature 2 — passphrase gate: NOTHING to install. Stdlib only.
#   internal/authgate/ uses crypto/hmac, crypto/sha256, crypto/subtle, encoding/base64, net/http.
#   New env vars wired through the existing caarlos0/env/v11 Config struct.

# Feature 1 + 3 — GitHub Actions (pinned by commit SHA, matching this repo's existing convention):
#   appleboy/ssh-action@0ff4204d59e8e51228ff73bce53f80d53301dee2          # v1.2.5
#   fgrosse/go-coverage-report@e432de98ee94a276e8f666d25bfd76347665f75b   # v1.3.1
#   davelosert/vitest-coverage-report-action@8b157684c6a6b259b97d45e72b44242865c0f6a5  # v2.12.2
#   marocchino/sticky-pull-request-comment@5770ad5eb8f42dd2c4f34da00c94c5381e49af88    # v3.0.5 (only if custom-script route)

# web/vitest.config.ts — add reporters:
#   coverage: { reporter: ['text', 'json', 'json-summary', ...existing] }
```

No new `go.mod` entries. No new npm dependencies (Vitest v8 coverage provider is already present for the thresholds gate).

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `appleboy/ssh-action` (run script over SSH) | **Docker context over SSH** (`docker context create --docker host=ssh://… ` then run `docker compose` from the runner) | When you want the compose file + deploy logic to live and execute *on the CI runner* rather than the VPS, or you are orchestrating multiple hosts. Costs: the runner needs the compose file and all runtime secrets materialized locally, SSH multiplexing quirks, and `docker compose pull` auth happens runner-side. For one host with the compose file already on it, running the script server-side is simpler and keeps secrets on the VPS. |
| `appleboy/ssh-action` | **Raw `ssh` + `webfactory/ssh-agent` + manual `known_hosts`** | If you want to avoid a Docker-based third-party action in the pipeline on principle. Fully doable; you re-implement key loading, host-key pinning, timeout, and env passthrough yourself (~15–20 lines). Reasonable for a DevOps portfolio piece that wants to *show* the primitives — note this as a legitimate variant. |
| `appleboy/ssh-action` | `appleboy/scp-action` **v1.0.0** (`ff85246acaad7bdce478db94a363cd2bf7c90345`) | Complementary, not a replacement — use it to copy `deploy.sh` / an updated `docker-compose.yml` to the VPS during the deploy job if you do not manage those via `git pull` on the server. |
| Stateless HMAC-signed cookie (hand-rolled) | `alexedwards/scs` **v2.9.0** + `alexedwards/scs/postgresstore` | If the gate later grows into real multi-user auth (accounts, per-user watchlists, server-side "log out everywhere", CSRF-tokened sessions). PROJECT.md explicitly rejects multi-user for v1.3, so this is a *future-milestone* tool. `postgresstore` (not `memstore`) would be mandatory to survive the constant redeploys — which means a new migration. |
| Stateless HMAC-signed cookie (hand-rolled) | `gorilla/securecookie` **v1.1.2** | If you want a vetted encode/decode-with-rotation-keys implementation instead of ~30 lines of your own HMAC, and are willing to add one small dependency. It is genuinely tiny and stable. The hand-rolled version is preferred *only* because of the explicit minimal-footprint constraint and the repo's consistent hand-roll pattern — `securecookie` is a defensible pick if the team would rather not own crypto glue code. |
| `fgrosse` + `davelosert` (two actions) | **Custom script + `marocchino/sticky-pull-request-comment` v3.0.5** | If a **single** unified PR comment is important. You write a ~50-line script that reads `go tool cover -func` total + `coverage-summary.json`, diffs both against baseline numbers pulled from a `code-coverage`/`frontend-coverage` artifact, renders one markdown table, and posts it with one `sticky-pull-request-comment` step. Most control, one comment, still greppable — but you own the diff logic for two formats. |
| `fgrosse` + `davelosert` (two actions) | **`k1LoW/octocov`** (`octocov-action` v1.5.2 / octocov v0.75.12) | If you want *one* tool + *one* `.octocov.yml` covering both Go and LCOV, with a single PR comment, and you are comfortable introducing a **datastore** concept (a git branch, or GitHub Actions Artifacts, holding historical coverage) for the baseline. It is a single Go binary and genuinely capable. Rejected as the primary because it is an all-in-one that adds a new persistent concept, where the repo has deliberately kept coverage as two independent single-language mechanisms with literal thresholds. Reconsider if coverage reporting needs grow (trends over time, code-to-test ratio, test-execution-time tracking — octocov does all three). |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| `:latest` (or any moving tag) for the VPS image | Rollback becomes non-deterministic — you cannot name the previous image, and `docker compose pull` on a re-run silently changes what "current" means. | The `svu`-computed semver tag (`v1.2.3`), interpolated into `docker-compose.yml` from `.env`; record the prior value in `.env.previous`. |
| `pull_request_target` to make the coverage comment work on fork PRs | Runs untrusted PR head code in a context that has a **write** `GITHUB_TOKEN` and access to secrets — a well-documented supply-chain footgun. | Accept the fork limitation (this repo's PRs are same-repo branches), or use the `pull_request` + `workflow_run` two-workflow split. |
| Adding the coverage-comment job to `build-scan.needs` / `release.needs` | A GitHub API hiccup posting a comment would block image build and deploy. Reporting must never gate shipping. | Standalone job, `if: github.event_name == 'pull_request'`, not referenced by any `needs:` on the release path. |
| `alexedwards/scs` with the default `memstore` | In-memory sessions are wiped on every process restart; this app restarts on **every merge to main**, so every user re-enters the passphrase after every deploy. | Stateless signed cookie (survives restarts by design), or `scs` + `postgresstore` if you truly need server-side sessions. |
| `gorilla/sessions` as a "just use the standard thing" default | The Gorilla toolkit was archived and only partially revived; low activity, and its cookie store is more machinery than a one-passphrase gate needs. For this use case it offers nothing over a hand-rolled signed cookie. | Hand-rolled HMAC cookie, or `gorilla/securecookie` alone if you want the encoding helper. |
| Watchtower / auto-pulling agents on the VPS | Pulls and restarts on registry change with no `/health` gate and no rollback — the opposite of the controlled, verifiable deploy this milestone is about. | The explicit `deploy.sh` (pull → `up -d` → poll `/health` → rollback-on-fail). |
| GoReleaser / Kamal / `docker rollout` / Ansible for the deploy | Out of scope per PROJECT.md ("VPS SSH deploy is the current ceiling"; K8s/Helm/Terraform explicitly deferred). Adds a tool and a mental model for capability this milestone does not want yet. | ~40-line `deploy.sh` invoked via `appleboy/ssh-action`. |
| A second GitHub secret for ghcr.io pull on the VPS (while the package is public) | Unnecessary credential surface; the package is already public and pullable unauthenticated, consistent with the "no extra registry secret" constraint. | No login step while public; add a `packages:read` PAT only if/when the package is made private. |
| Skipping SSH host-key verification (`appleboy/ssh-action` without `fingerprint`) | Silently accepts any host key → MITM exposure of the deploy channel and the passphrase secret passed through it. | Set the `fingerprint` input from an `SSH_FINGERPRINT` secret captured once during provisioning. |

---

## Stack Patterns by Variant

**If the ghcr.io package is made private:**
- Add a `packages:read`-scoped PAT (or fine-grained token) as an `SSH`-side secret and `docker login ghcr.io` inside `deploy.sh` before `compose pull`.
- Because the VPS has no access to the workflow's ambient `GITHUB_TOKEN`.

**If a single unified coverage PR comment becomes a hard requirement:**
- Switch Feature 3 to the custom-script + `marocchino/sticky-pull-request-comment@v3.0.5` route, or adopt `k1LoW/octocov` with an `artifact://` datastore.
- Because `fgrosse` and `davelosert` each own their own comment with incompatible formats.

**If the passphrase gate grows toward real accounts:**
- Move to `alexedwards/scs@v2.9.0` + `scs/postgresstore` (new migration), keep chi middleware shape.
- Because stateless single-secret cookies do not model per-user identity, revocation, or CSRF-tokened mutating requests.

**If zero-downtime deploys are wanted later:**
- Introduce a reverse proxy (Caddy/Traefik) and a two-slot (blue/green) compose project, or adopt `docker rollout`.
- Because the recommended single-container recreate has a few seconds of downtime per deploy.

**If deploys need to fan out to more than one VPS:**
- Switch to `docker context` over SSH driven from the runner, or a matrix over hosts in `appleboy/ssh-action`.
- Because a single server-side script does not coordinate multi-host rollout.

---

## Version Compatibility

| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `appleboy/ssh-action@v1.2.5` | GitHub-hosted `ubuntu-latest` runner | Docker-based action; runs fine on the standard runner. `command_timeout` default is 10m — set it explicitly (e.g. `5m`) so a hung deploy fails the job promptly. |
| `fgrosse/go-coverage-report@v1.3.1` | `actions/upload-artifact@v7` / `download-artifact@v8` (already in this repo) | Baseline artifacts expire after 90 days — fine here (main gets a run at least every merge). Needs `permissions: pull-requests: write`. |
| `davelosert/vitest-coverage-report-action@v2.12.2` | Vitest v8 coverage provider (already present) | Requires `json-summary` reporter (and `json` for file-level detail) added to `web/vitest.config.ts` `coverage.reporter`. Pin `@v2` per its docs or the SHA above. |
| `marocchino/sticky-pull-request-comment@v3.0.5` | Node 20 action runtime | `header:` input distinguishes multiple sticky comments if used for both languages. |
| Hand-rolled `internal/authgate` | Go 1.26 (repo toolchain), `caarlos0/env/v11`, chi v5 middleware chain | Pure stdlib crypto; no version coupling. Register the middleware **after** `middleware.RequestID`/`Recoverer`/`httplog` and **before** the API + SPA-fallback routes; explicitly exclude `/health`. |
| `release` job output `version` | new `deploy` job input | Requires adding `outputs:` to the `release` job and `>> "$GITHUB_OUTPUT"` in the svu step — the value is currently only in `$GITHUB_ENV` (step-local). |

---

## Integration Points Into `full-pipeline.yml` (summary)

1. **`release` job:** add `outputs.version`; svu step also writes `version=$NEXT >> $GITHUB_OUTPUT`.
2. **New `deploy` job:** `needs: [release]`, same `if:` guard as `release`, `concurrency: deploy-<ref> / cancel-in-progress: false`, one `appleboy/ssh-action` step running `/opt/drop-tracker/deploy.sh` with `NEW_TAG` passed via `envs:`.
3. **`test` job (push to main only):** add a step uploading `coverage.txt` as artifact `code-coverage`.
4. **`frontend-test` job (push to main only):** add a step uploading `coverage/coverage-summary.json` as artifact `frontend-coverage`; add `json`/`json-summary` reporters to `web/vitest.config.ts`.
5. **New `coverage-comment` job:** `if: github.event_name == 'pull_request'`, `needs: [test, frontend-test]`, `permissions: { contents: read, pull-requests: write }`, runs `fgrosse/go-coverage-report` + `davelosert/vitest-coverage-report-action`. **Not** in any release-path `needs:`.
6. **New GitHub Actions secrets:** `SSH_HOST`, `SSH_USER`, `SSH_KEY`, `SSH_FINGERPRINT`. **New VPS `.env` values:** `GATE_PASSPHRASE`, `GATE_SIGNING_KEY` (plus existing runtime secrets).
7. **Redaction:** add `GATE_PASSPHRASE`, `GATE_SIGNING_KEY` to the slog redaction key list.

---

## Sources

- GitHub REST API (`/repos/{repo}/releases/latest`, `/git/ref/tags/{tag}`) — retrieved 2026-08-27 — exact latest versions + commit SHAs for `appleboy/ssh-action` v1.2.5, `appleboy/scp-action` v1.0.0, `fgrosse/go-coverage-report` v1.3.1, `davelosert/vitest-coverage-report-action` v2.12.2, `marocchino/sticky-pull-request-comment` v3.0.5, `k1LoW/octocov` v0.75.12, `k1LoW/octocov-action` v1.5.2 — confidence HIGH
- `proxy.golang.org/.../@latest` — retrieved 2026-08-27 — `github.com/alexedwards/scs/v2` v2.9.0, `github.com/gorilla/securecookie` v1.1.2, `github.com/gorilla/sessions` v1.4.0 — confidence HIGH
- `github.com/fgrosse/go-coverage-report` README (WebFetch, 2026-08-27) — baseline-artifact mechanism, Go-only scope, fork-PR limitation, `v1.3.1` in usage example — confidence MEDIUM
- `github.com/davelosert/vitest-coverage-report-action` README (WebFetch, 2026-08-27) — `json-summary`/`json` reporter requirement, `json-summary-compare-path` for diffs (no auto baseline), `pull-requests: write`, fork `pull_request`+`workflow_run` split — confidence MEDIUM
- `github.com/k1LoW/octocov` README (WebFetch, 2026-08-27) — multi-format (Go + LCOV) support, datastore requirement for diff baseline, single-binary, `.octocov.yml` — confidence MEDIUM
- `github.com/appleboy/ssh-action` v1.2.5 README (raw, 2026-08-27) — `fingerprint` host verification, `envs:` passthrough, `command_timeout` — confidence MEDIUM
- WebSearch "gorilla/sessions vs alexedwards/scs maintenance status" (2026-08-27) — gorilla toolkit archival/partial revival, scs stable+maintained — confidence MEDIUM
- Project `CLAUDE.md` + `.planning/PROJECT.md` + `.github/workflows/full-pipeline.yml` + `docker-compose.yml` + `Dockerfile` — locked stack, existing `needs:` graph, ghcr.io/`GITHUB_TOKEN` constraint, boot-time migrations, `/health` semantics — confidence HIGH

---
*Stack research for: CI/CD continuous deployment (SSH-to-VPS deploy, passphrase gate, PR coverage-diff comment)*
*Researched: 2026-08-27*
