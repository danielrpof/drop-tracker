# Phase 17: Automated VPS Deploy with Health-Gated Rollback - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-05
**Phase:** 17-automated-vps-deploy-with-health-gated-rollback
**Areas discussed:** Provisioning deliverable, Downtime posture + TLS proxy, SSH transport + on-box script model, Prune policy + operational guardrails

---

## Gray-area selection

All four presented areas were selected for discussion. The 4 keyword-matched pending
todos were reviewed; **none folded** (3 are Phase 16 `sqlscan`/`migration-check`
tooling debt, 1 is frontend-deps hygiene — none touch VPS deploy).

---

## Provisioning deliverable

| Option | Description | Selected |
|--------|-------------|----------|
| Wizard + thin runbook | Interactive `deploy/provision.sh` for the mechanical steps + a short doc for prerequisites and dashboard-click steps | ✓ |
| Prose runbook only | Single Markdown doc with copy-paste blocks | |
| Wizard only | Everything in the script including printed manual-step instructions | |

**User's choice:** Wizard + thin runbook
**Notes:** —

| Option | Description | Selected |
|--------|-------------|----------|
| Script on the box + documented | `deploy/pg-backup.sh` / `pg-restore.sh` on the VPS, operator-run, plus a runbook section on when to use them | ✓ |
| Documented commands only | Runbook lists the `pg_dump` / `pg_restore` invocations, no script committed | |

**User's choice:** Script on the box + documented
**Notes:** Scheduling / off-box retention stays OPS-04 (Future).

| Option | Description | Selected |
|--------|-------------|----------|
| Best-effort idempotent | Detect-and-skip already-done steps where cheap | ✓ |
| One-shot only | Assumes a clean VPS | |

**User's choice:** Best-effort idempotent

---

## Downtime posture + TLS proxy

| Option | Description | Selected |
|--------|-------------|----------|
| Accept + document the gap | Single `app` service, `pull` → `up -d app` → health-poll, ~2-10s of 502s cushioned by graceful SIGTERM, documented as an accepted tradeoff | ✓ |
| Blue/green cutover via Caddy | app-next alongside app-prev, poll then flip Caddy upstream, then stop app-prev | |

**User's choice:** Accept + document the gap
**Notes:** Blue/green stays DPLY-09 (Future).

| Option | Description | Selected |
|--------|-------------|----------|
| Caddy on-VPS | Automatic HTTPS via ACME, no third party in the request path, self-hosted ethos | ✓ |
| Cloudflare Tunnel | No inbound port, DDoS protection, but external dependency + Cloudflare in the path | |
| nginx / Traefik | More manual cert wiring or more moving parts | |

**User's choice:** Caddy on-VPS (confirms ROADMAP's recommendation)

| Option | Description | Selected |
|--------|-------------|----------|
| Compose service | `caddy` service in `deploy/compose.prod.yaml` + committed `Caddyfile` + named volumes; app port unpublished | ✓ |
| Host-level Caddy | `apt install caddy`, systemd-managed, config outside the repo | |

**User's choice:** Compose service

---

## SSH transport + on-box script model

| Option | Description | Selected |
|--------|-------------|----------|
| Raw ssh/rsync + forced-command rollout.sh on the box | Deploy key pinned `command="…/rollout.sh",no-port-forwarding,…`; runner pre-populates known_hosts from a pinned secret; `ssh vps <tag>` | ✓ |
| appleboy/ssh-action with fingerprint: | SHA-pinned action, `fingerprint:` input, inline `script:` | |

**User's choice:** Raw ssh/rsync + forced-command rollout.sh on the box

| Option | Description | Selected |
|--------|-------------|----------|
| Pass only the tag; sync scripts/compose on change | `.env` + tag files live only on the box, never rsync'd; deploy passes just the version string | ✓ |
| Full deploy/ dir rsync every run | Simpler mental model, `.env` excluded explicitly | |

**User's choice:** Pass only the tag; sync scripts/compose on change

| Option | Description | Selected |
|--------|-------------|----------|
| docker inspect on the box | Rollout script asserts running container image ref == handed tag; no app change, no version on `/health` | ✓ (both) |
| Enrich public /health with a version field | One small app-code change; bare version becomes public recon surface | |
| Gated /health/details with version | Version behind the passphrase; deploy can't read it without the secret | |

**User's choice (free-text):** "Is it possible to include a small text that shows the
version in the application ui. Also implement the docker inspect on the box." →
Confirmed on follow-up: **both** — the `docker inspect` image-ref assertion in
`rollout.sh` (the automated SC #2 gate) **and** a build-stamped version string
rendered in the SPA UI via a small **public `GET /version`** endpoint (deliberate,
bare-version-only carve-out from Pitfall 19). `/health` itself stays minimal.

| Option | Description | Selected |
|--------|-------------|----------|
| Unprivileged deploy user, docker group only | Dedicated `deploy` user, docker group, no sudo; runbook accepts docker-group = root-equivalent as the portfolio ceiling | ✓ |
| Document a hardening path beyond docker-group | Also research/document rootless Docker or a sudo-scoped wrapper | |

**User's choice:** Unprivileged deploy user, docker group only

---

## Prune policy + operational guardrails

| Option | Description | Selected |
|--------|-------------|----------|
| Keep exactly current + rollback | Retag `:current` / `:rollback`, remove older; never `docker system prune -a` | ✓ |
| docker image prune -f after success | Simpler one-liner, dangling images only | |
| You decide | — | |

**User's choice:** Keep exactly current + rollback

| Option | Description | Selected |
|--------|-------------|----------|
| On-box flock in rollout.sh | `flock -n /run/drop-tracker-deploy.lock` wrapping the rollout | ✓ |
| Bounded restart policy | Prod compose `app` uses `on-failure` with max attempts, not `unless-stopped` | ✓ |
| 80%-disk Discord alert | Cron on the box posts to the existing webhook at 80% Docker-fs usage | ✓ |
| Re-assert n1-boot probe (Phase 16 UF-1) | Once a guard-carrying release is N-1, stop skip-greening `n1-boot` and alarm if the window stays open | ✓ |

**User's choice:** All four in scope for this phase

| Option | Description | Selected |
|--------|-------------|----------|
| workflow_dispatch input on the deploy job | Optional `force_bad_tag` input deploys a known-unhealthy tag; inert on normal merges | ✓ |
| Manual on-box procedure | Runbook documents pointing `current-tag` at a bad image by hand | |
| Scratch-branch / throwaway tag | Build + push a deliberately-broken image under a scratch tag | |

**User's choice:** workflow_dispatch input on the deploy job

---

## Claude's Discretion

- Exact `deploy/` file names and on-box directory layout, the `flock` lock path.
- Health-poll timing (attempts, interval, sustained-green count) for the rollout and
  the post-rollback re-verify.
- The "known-bad image" mechanism for the `force_bad_tag` drill.
- Where the version var lives (`main.version` vs. `internal/version`) and how the
  ldflags value threads through the Dockerfile build stage / `release` job.
- Whether the UF-1 re-assert (D-20) is an edit to the existing `guardcheck` step or a
  separate assertion, and whether it lands in the initial plans or a gap-closure.
- Whether the `production` environment gets an optional wait timer.
- SSH secret names; rsync-on-change vs. rsync-every-run for `deploy/` artifacts.
- Whether `rollout.sh` pins the rollback target to the image digest or the `:rollback`
  tag.
- Whether the runbook lives under `deploy/` or `docs/`.
- Whether `GET /version` takes a `WithVersion` option on `httpserver.New`.

## Deferred Ideas

- Blue/green / near-zero-downtime deploy — DPLY-09 (Future).
- Scheduled + off-box Postgres backups with retention — OPS-04 proper (Future).
- ghcr.io image-retention / cleanup job — OPS-05 (Future).
- Session signing-key rotation without full logout — GATE-08 (Future).
- Hardening the deploy user past docker-group (rootless Docker / sudo-scoped wrapper).
- Deeper `/health` readiness (migration version, build version in `/health`).
- Poller leader-election via Postgres advisory lock (scaling concern only).
- IaC / Terraform VPS provisioning.

### Reviewed Todos (not folded)
- Delete the stale tracked `web/package-lock.json` — frontend-deps hygiene.
- Resolve the D-15 previous-release schema/query files from `--prev-tag`, not CWD —
  Phase 16 `migration-check` tooling debt.
- Unify `sqlscan`'s two hand-rolled quote/dollar-quote state machines — Phase 16 debt.
- Move `shadcn` from dependencies to devDependencies — frontend-deps hygiene.
