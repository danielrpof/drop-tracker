# Phase 17: Automated VPS Deploy with Health-Gated Rollback - Context

**Gathered:** 2026-09-05
**Status:** Ready for planning

<domain>
## Phase Boundary

Merging a PR to `main` puts that exact released ghcr.io image live on a public HTTPS
URL within minutes with **no human action**, and a release whose `/health` never comes
up restores the previous image by itself. This is the deployment step the whole
pipeline was built pointing at (DPLY-01..DPLY-08).

**Delivers:**

1. A **`deploy` job** in `.github/workflows/full-pipeline.yml`: `needs: [release]`,
   `if: github.event_name == 'push' && github.ref == 'refs/heads/main'`, its own
   `concurrency` group with `cancel-in-progress: false`, `environment: production`
   holding the SSH secrets. Also a `workflow_dispatch` path with an optional
   `force_bad_tag` input for the SC #3 rollback drill (inert on normal merges).
2. A small **`release`-job change** to expose `outputs.version` (currently `VERSION`
   is only written to `$GITHUB_ENV`, job-local).
3. **Versioned repo artifacts** under `deploy/`: `compose.prod.yaml` (image-based, no
   `build:`, adds a `caddy` service, bounded restart policy, no published app port),
   `Caddyfile`, `rollout.sh` (the health-gated rollout + rollback script invoked as a
   forced SSH command), `pg-backup.sh` / `pg-restore.sh`, and an interactive
   `provision.sh` wizard.
4. A **thin prose provisioning runbook** covering prerequisites, the dashboard-click
   steps the wizard can't do, the passphrase-rotation = revoke-all procedure, the
   `TRUST_PROXY_HEADERS` timing rule, and when to use the backup/restore scripts.
5. A **GitHub `production` Environment** + SSH secrets (host, user, key, known_hosts).
6. A small **app-code change**: build-time version stamp + a minimal public
   `GET /version` endpoint + the SPA rendering that version string unobtrusively, so a
   human can eyeball the live release at the public URL (SC #1/#2 support).
7. **Re-asserting the Phase 16 `n1-boot` probe** (UF-1) once the first v1.3
   guard-carrying release becomes the N-1 rollback target.

**In scope:** DPLY-01, DPLY-02, DPLY-03, DPLY-04, DPLY-05, DPLY-06, DPLY-07, DPLY-08.
Edits `.github/workflows/full-pipeline.yml`; adds `deploy/`; adds a `version` package
or `main.version` ldflags var, a `GET /version` route, a small SPA footer component;
edits `Dockerfile`/`Makefile` for the version stamp; edits `.env.example` and the
provisioning runbook. Provisions and drills against a real VPS.

**Out of scope:**
- **Blue/green / near-zero-downtime deploy** (connection draining, second container) -
  the ~2-10s single-container swap gap is accepted and documented. Stays DPLY-09
  (Future).
- **Scheduled / off-box Postgres backups + retention policy** - only the *basic*
  operator-run backup + restore scripts + procedure land here (OPS-04 basics). OPS-04
  proper (scheduling, off-box copies, retention) stays Future.
- **ghcr.io image-retention / cleanup job** - OPS-05 (Future). This phase's rollback
  never depends on ghcr, so ghcr GC cannot break it (Pitfall 7).
- **IaC / Terraform-provisioned VPS** - the `provision.sh` wizard + runbook is
  manual-but-documented; full IaC stays past the current DevOps ceiling.
- **Multi-hop rollback** - auto-rollback targets strictly N-1 (Phase 16 D-17, locked
  here as D-14). N-2 or earlier is never a target.
- **`down` migrations / `migrate down` as a rollback mechanism** - the app is
  forward-only; rollback is image-swap + N-1 schema compatibility (already enforced by
  Phase 16).
- **Enriching `/health` beyond the current DB-ping** - `/health` stays the minimal
  `{"status":"ok"}` / `503` gate. The version check is a separate `GET /version` plus
  `docker inspect` on the box, not a `/health` field (Pitfall 19).
- **Hardening the deploy user past docker-group** - rootless Docker / sudo-scoped
  wrapper is noted as a known ceiling, not implemented.
- **Rootless / non-privileged migration one-shot step** - migrations stay boot-time,
  in-process, single instance (research: fine for one replica; Pitfall 10 is a
  scaling concern this design doesn't have).

</domain>

<decisions>
## Implementation Decisions

### Provisioning deliverable (DPLY-07)

- **D-01:** The provisioning deliverable is an **interactive `deploy/provision.sh`
  wizard** plus a **thin prose runbook**. The wizard walks the mechanical steps
  (Docker presence check, `deploy` user + docker group, SSH key generation,
  host-key fingerprint capture, `.env` scaffold, Caddy/compose bring-up, first manual
  `compose up`, seed `current-tag`) and pauses for the parts only a human can do
  (paste the pubkey into the VPS provider console, add the GitHub Environment
  secrets). The runbook covers prerequisites, those dashboard-click steps, and the
  standing procedures (below). SC #1 is "following the wizard/runbook from a fresh
  VPS produces a running HTTPS instance that asks for the passphrase".
  — **Reversibility:** reversible — self-contained files under `deploy/`.
- **D-02:** The wizard is **best-effort idempotent** — detect-and-skip already-done
  steps where cheap (user exists, key exists, Caddyfile in place) so a half-finished
  run resumes and a box rebuild is less painful. Not required to be bulletproof.
- **D-03:** Postgres backup + restore (OPS-04 basics, folded in per ROADMAP) is
  delivered as **`deploy/pg-backup.sh`** (`pg_dump` to a local timestamped file) and
  **`deploy/pg-restore.sh`**, placed on the box during provisioning, **operator-run
  and unscheduled**, plus a runbook section explaining *when* to use them (an image
  rollback that outran an irreversible migration — Pitfall 8, the milestone's
  highest-cost failure). Scheduling / off-box retention stays OPS-04 (Future).

### Downtime posture + TLS reverse proxy (DPLY-08)

- **D-04:** The deploy **accepts and documents the swap-gap downtime** rather than
  mitigating it. Single `app` service: `docker compose pull app` (image local before
  any stop), `docker compose up -d app`, then poll `/health`. ~2-10s of 502s during
  boot + migrations, cushioned by the app's graceful SIGTERM shutdown. The runbook
  records the measured window. `stop_grace_period` in the prod compose comfortably
  exceeds the app's shutdown timeout so graceful shutdown completes (Pitfall 4).
  Postgres is **never** recreated on an app deploy (`up -d app`, never a bare
  `up -d`). Blue/green health-gated cutover stays **DPLY-09 (Future)**.
  — **Reversibility:** reversible, but revisiting means real rollout-script + compose
  rework; DPLY-09 is the tracked path.
- **D-05:** The TLS reverse proxy is **Caddy on the VPS** (confirmed — ROADMAP's
  recommendation over Cloudflare Tunnel). Automatic HTTPS via ACME, no third party in
  the request path, keeps the self-hosted / minimal-external-dependency ethos.
  — **Reversibility:** costly — the public origin, cert automation, and the
  `X-Forwarded-*` / `TRUST_PROXY_HEADERS` topology are all built around it; swapping
  proxies later touches provisioning, compose, and the runbook.
- **D-06:** Caddy runs as a **`caddy` service in `deploy/compose.prod.yaml`** with a
  committed **`deploy/Caddyfile`** and named volumes for certs/config. Caddy publishes
  `80`/`443`; the `app` container port is **not published** (only Caddy reaches it on
  the Docker network). One `compose up` brings the whole stack. This is the topology
  that makes `TRUST_PROXY_HEADERS=true` safe (Phase 14 D-14): app unreachable except
  through the proxy, so `middleware.RealIP` can trust the proxy-set `X-Forwarded-For`.

### SSH transport + on-box rollout/rollback model (DPLY-04, DPLY-05)

- **D-07:** The runner reaches the VPS with **raw `ssh` / `rsync`**, not
  `appleboy/ssh-action`. `deploy/rollout.sh` lives **permanently on the box**; the
  deploy key in `authorized_keys` is pinned with a forced command:
  `command="/opt/drop-tracker/rollout.sh",no-port-forwarding,no-agent-forwarding,no-pty`
  (Pitfall 2). The runner can invoke exactly that one script and nothing else. The
  runner pre-populates `~/.ssh/known_hosts` from a **pinned host-key secret** before
  any `ssh` call — no TOFU, no `ssh-keyscan` at runtime, no `StrictHostKeyChecking=no`
  (Pitfall 1, SC #5). A deliberately-wrong host key must fail the deploy.
  — **Reversibility:** reversible — the forced-command line and the workflow step are
  each a small edit.
- **D-08:** The deploy **passes only the new version string** as the forced-command
  argument. `compose.prod.yaml`, `Caddyfile`, `rollout.sh`, and the backup scripts are
  rsync'd to the box **only when they change** (or every run — the operation is
  idempotent). The real `.env` and the `current-tag` / `previous-tag` files live
  **only on the box** and are never touched by rsync (Pitfall 3; ARCHITECTURE.md
  anti-pattern "storing the pinned tag inside the secret `.env`" — the tag files carry
  nothing sensitive so the rollback rewrite is low-blast-radius).
- **D-09:** The pinned tag is stored in its **own one-line file** (`current-tag`),
  separate from the secret `.env`. `previous-tag` (and the previous container's
  resolved image digest via `docker inspect`) is written by `rollout.sh` before the
  swap. Tag-file updates use write-to-temp-then-atomic-`mv` so a script death before
  the `mv` leaves the running container and the old tag untouched.
- **D-10:** Rollback restores from the **image already in the local Docker store**
  (the outgoing `:current` / a locally-pinned digest), never a re-pull from ghcr.io
  (Pitfall 7). `rollout.sh` then re-polls `/health` after restoring and **fails loudly
  with a non-zero exit + a Discord alert** if the rollback is *also* unhealthy — the
  "call a human" state (Pitfall 5). First-ever deploy: `previous-tag` empty → skip
  rollback, just `exit 1` on health failure.
- **D-11:** Version-live confirmation is **both**:
  1. **`docker inspect` on the box** — after `up -d app`, `rollout.sh` asserts the
     running `app` container's image ref matches the tag it was handed. This is the
     automated SC #2 gate. No app-code, no version on `/health`.
  2. **A build-stamped version string in the SPA UI** — the Go binary carries its
     version via `-ldflags "-X <pkg>.version=<tag>"` set at image build time (the
     `release` job / Dockerfile already knows the tag); a **minimal public
     `GET /version`** returns `{"version":"vX.Y.Z"}`; the SPA fetches it once and
     renders it small and unobtrusive (footer / bottom corner), visible whether or
     not the viewer is past the passphrase gate.
  — **Reversibility:** the `GET /version` route + SPA footer are reversible; the
  ldflags wiring in the Dockerfile is a small standing change.
  — **Pitfall 19 note:** exposing a bare version string publicly is a **deliberate,
  scoped carve-out** from "keep `/health`/public surface minimal". Accepted because
  the image tag is already public on ghcr.io and the operator explicitly wants the
  live version visible at the URL. `GET /version` returns the version string and
  nothing else — no DB host/name, no env, no dependency versions, no build host.
- **D-12:** Health polling happens **inside the SSH session against `127.0.0.1`**
  (`rollout.sh`), never from the GitHub runner — the app port is firewalled to
  localhost + Caddy (Pitfall 6, ARCHITECTURE.md anti-pattern). The poll loops with
  bounded retries and an explicit total timeout and requires a **sustained** green
  window (N consecutive 200s), so a container that starts then dies fails the gate
  rather than passing on one lucky early 200. Exact attempt count / interval /
  sustained-window is Claude's discretion (mirror Phase 16's `n1-boot` 8×@2s shape).
- **D-13:** The **`deploy` user is unprivileged** — dedicated `deploy` account, member
  of the `docker` group only, no sudo. The runbook **explicitly acknowledges
  docker-group = root-equivalent** and accepts it as the portfolio-VPS ceiling
  (Pitfall 2). The forced command (D-07) further limits what the key can do. No
  registry credential on the box — the package is public (Pitfall 2).

### Prune policy + operational guardrails (DPLY-03, and Pitfalls 9/12/13)

- **D-14 (rollback is strictly N-1 — locked here, assumed by Phase 16):** Phase 17's
  auto-rollback targets **exactly the immediately-previous release tag**, never N-2 or
  earlier (Phase 16 D-17, recorded there, locked here). Phase 16's `migration-check`
  cross-reference and `n1-boot` probe both assume this single hop.
  — **Reversibility:** one-way in spirit — Phase 16's safety checks only verify one
  hop; widening rollback to N-2 would need Phase 16 rework.
- **D-15:** Post-deploy image prune keeps **exactly `:current` + `:rollback`**. After
  a successful, health-verified deploy `rollout.sh` retags the new image `:current`,
  the outgoing one `:rollback`, and removes any older `drop-tracker` image. **Never
  `docker system prune -a`** in the deploy path (it would delete the `:rollback` image
  and the Postgres image — Pitfall 13, ARCHITECTURE.md anti-pattern). Bounded disk
  footprint regardless of deploy count.
- **D-16:** Concurrency is **belt-and-suspenders**: the CI `deploy` job's own
  `concurrency: { group: deploy-production, cancel-in-progress: false }` (queue, never
  cancel) **plus** an on-box `flock -n /run/drop-tracker-deploy.lock` wrapping
  `rollout.sh`, so a manual deploy and a CI deploy can't overlap even though CI
  already serializes and `deploy needs: [release]` (which is itself serialized). Two
  rapid merges deploy strictly one after the other (DPLY-03, SC #4).
- **D-17:** `deploy/compose.prod.yaml`'s `app` service uses a **bounded restart
  policy** (e.g. `restart: on-failure` with a max-attempts cap via
  `deploy.restart_policy`, not `unless-stopped`), so a bad-migration crash-loop
  **stops and stays visibly down** instead of thrashing while the deploy health poll
  waits out its timeout (Pitfall 9). Postgres keeps `unless-stopped`.
- **D-18:** The wizard installs a tiny **cron on the box that posts to the existing
  Discord webhook when the Docker filesystem exceeds 80%** (Pitfall 13). Reuses the
  notification sink already in the app's `.env`; no new secret.
- **D-19:** The SC #3 **rollback drill is triggered by `workflow_dispatch`** on the
  deploy job with an optional `force_bad_tag` input: when set, the deploy targets a
  known-unhealthy tag instead of the released one. Inert on every normal merge
  (`workflow_dispatch` only), cannot fire by accident, and is documented as the UAT
  step for SC #3 (the drill runs against the **live box** — verify rollback triggers,
  the previous version comes back healthy, the run ends **red**, and the Discord alert
  fires). The "known-bad image" shape (e.g. a tag whose `/health` never returns 200)
  is Claude's discretion.
- **D-20 (Phase 16 UF-1 follow-up):** Once the first v1.3 release — the first
  guard-carrying image (`internal/db/migrate.go` has `func maxSourceVersion(`) —
  becomes the N-1 rollback target, **re-assert the `n1-boot` probe**: the `guardcheck`
  skip-green path (quick task 260905-et1) should stop masking the boot check, and the
  workflow should surface an alarm (`::warning::` / notice) if the guard-adoption
  window is still open when it shouldn't be. Planner: decide whether this is a small
  edit to the existing `guardcheck` step or a follow-up assertion; it is genuinely
  Phase 17 work because the first v1.3 release is produced by this phase's own deploy
  path.

### Locked implementation requirements (not gray areas — carried from research/prior phases)

- **D-21:** `release` job gains `outputs.version` (emit `version=$NEXT` to
  `$GITHUB_OUTPUT` in the svu step); `deploy` pins `needs.release.outputs.version`.
- **D-22:** `deploy` job `permissions:` is minimal — `contents: read` only (checks out
  the repo for the `deploy/` artifacts); **no** `packages:` (public image), **no**
  `pull-requests:` write. Never `pull_request_target` anywhere (Pitfall 11).
- **D-23:** SSH secrets live in the **`production` GitHub Environment**, not repo
  secrets — scoped so no other job/trigger can read them. Environment protection:
  **branch restriction to `main`, no required reviewer** (DPLY-06 demands no human
  action). A wait timer is optional and Claude's discretion (likely omitted).
- **D-24:** The workflow includes a **post-run secret-grep assertion** — after the SSH
  session, grep the captured remote output for known secret prefixes (`postgres://`,
  `https://discord.com/api/webhooks/`, key material) and **fail the run if any are
  found** (Pitfall 3, SC #5). No `set -x` / `bash -x` anywhere secrets are in scope;
  no `docker compose config` in the deploy path (it inlines `env_file` secrets to
  stdout).
- **D-25:** Migrations stay **boot-time, in-process** (the existing `migrate.Up()`
  path). No separate one-shot migration step — single instance, so advisory-lock
  contention (Pitfall 10) isn't a concern. The Phase 16 ahead-of-source guard already
  makes the N-1 image's boot migration a clean no-op against a newer schema.
- **D-26:** `.env.example` gains the prod-relevant vars and guidance (no real values):
  the deploy topology's `TRUST_PROXY_HEADERS=true` note already exists (Phase 14); add
  any prod-only var the compose file introduces. The real `.env` on the box carries
  `DATABASE_URL` (real password), `DISCORD_WEBHOOK_URL`, `INSTANCE_PASSPHRASE`,
  `MUSICBRAINZ_USER_AGENT` (real contact) — `chmod 600`, owned by `deploy`.

### Claude's Discretion

- Exact `deploy/` file names and layout (`compose.prod.yaml`, `rollout.sh`,
  `provision.sh`, `Caddyfile`, `pg-backup.sh`, `pg-restore.sh` are working names).
- Health-poll timing for the rollout (attempts, interval, sustained-green count) and
  for the post-rollback re-verify.
- The "known-bad image" mechanism for the `force_bad_tag` drill.
- Where the version var lives (`main.version` vs. an `internal/version` package) and
  how the ldflags value is threaded through the Dockerfile build stage / `release`
  job.
- Whether D-20 (UF-1 re-assert) is an edit to the existing `guardcheck` step or a
  separate assertion; whether it lands in this phase's initial plans or a gap-closure
  once the first v1.3 tag exists.
- On-box directory layout (`/opt/drop-tracker/...`), the deploy-user home, and the
  `flock` lock path.
- Whether the `production` environment gets an optional wait timer (D-23).
- SSH secret names (`VPS_SSH_HOST` / `VPS_SSH_USER` / `VPS_SSH_KEY` /
  `VPS_SSH_KNOWN_HOSTS` are the research's working names).
- rsync-on-change detection vs. rsync-every-run for the `deploy/` artifacts (D-08).
- Whether `rollout.sh` pins the rollback target to the resolved image **digest** or
  the `:rollback` tag (research prefers digest; tag is acceptable since `release` only
  pushes immutable `vX.Y.Z`).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/ROADMAP.md` §"Phase 17: Automated VPS Deploy with Health-Gated Rollback" —
  goal, the 5 success criteria, the _Notes_ (heaviest phase, 17/27 pitfalls,
  `release` `outputs.version`, `needs: [release]` + push/main guard + own
  `concurrency` `cancel-in-progress: false` + `production` Environment, rollback
  targets the **locally-pinned** previous image, `/health` poll inside the SSH
  session against `127.0.0.1`), the **"Provisioning runbook (DPLY-07) must now also
  cover"** list (firewall allowlist during provisioning, `TRUST_PROXY_HEADERS=true`
  only after proxy live + port unpublished, passphrase-rotation = revoke-all, Postgres
  backup + restore), and the **"TLS reverse-proxy choice — recommended: Caddy
  on-VPS"** paragraph
- `.planning/REQUIREMENTS.md` — DPLY-01..DPLY-08 (locked text); Future section OPS-04 /
  OPS-05 / DPLY-09 / GATE-08 (what stays deferred); Out of Scope table (blue-green,
  IaC, `pull_request_target`)

### v1.3 research (this milestone)
- `.planning/research/PITFALLS.md` — **Pitfalls 1-13** (SSH host-key pinning,
  least-privilege deploy creds, secrets in logs, non-atomic swap, untested rollback,
  false-green health gate, ghcr GC vs. rollback, irreversible migration, boot-migration
  crash loop, migration lock contention, deploy on PR/fork, concurrent deploys, VPS
  disk fill), **Pitfall 19** (`/health` on the wrong side of the gate / verbose
  `/health`), **Pitfall 22** (reverse proxy / `X-Forwarded-For` handling); the
  **"Looks Done But Isn't" checklist**, the **"Technical Debt Patterns"** table
  (which shortcuts are "Never"), **"Recovery Strategies"**, and the
  **"Pitfall-to-Phase Mapping"** rows for 1-13/19/22
- `.planning/research/ARCHITECTURE.md` §"Feature 1 — VPS SSH deploy job with
  `/health`-gated rollback" — the whole section: `needs:` graph position, the
  `production` Environment rationale, `permissions: {}` / `contents: read`, the
  `release` `outputs.version` change, the **"What lives ON the VPS vs. in the repo"**
  table, the **atomic tag-file write-then-rename**, the **rollback flow with a single
  compose service**, the **"Boot-time migrations × rollback — the expand/contract
  constraint"** subsection, and the **"Anti-Patterns to avoid"** list (tag in secret
  `.env`, poll from runner, over-trust `/health` `200`, gate before `httplog`, in-mem
  sessions, positional `New()` param, re-run suite for baseline)
- `.planning/research/SUMMARY.md` — Phase 3 (VPS deployment) rationale; "TLS/proxy
  choice blocks first deploy"; expand/contract mandate
- `.planning/research/STACK.md` §"Feature 1" — appleboy/ssh-action, Docker Compose v2,
  `:latest` vs pinned-tag reasoning, deploy/rollback structure
- `.planning/research/FEATURES.md` §"Feature 1" — the deploy/rollback feature
  breakdown and the "Backward-compatible (expand/contract) migrations" discipline row

### Prior-phase decisions this phase depends on / locks
- `.planning/phases/16-rollback-safe-migrations/16-CONTEXT.md` — **D-17** (rollback is
  strictly N-1 — recorded there, *locked here* as D-14), **D-13** (`svu current` on
  `fetch-depth: 0` + fetch-tags resolves the N-1 tag), **D-15** (previous-release
  `queries/*.sql` cross-reference), and the **2026-09-05 amendment** (`n1-boot`
  `guardcheck` skip-green during the guard-adoption window — the UF-1 follow-up this
  phase re-asserts, D-20)
- `.planning/phases/16-rollback-safe-migrations/16-SECURITY.md` — **UF-1** (re-assert
  the `n1-boot` probe once a guard-carrying tag is N-1; alarm if the window stays
  open) and **UF-2** context
- `.planning/phases/14-instance-passphrase-gate/14-CONTEXT.md` — **D-10**
  (passphrase-rotation = revoke-all, "Phase 17's runbook documents it"), **D-14**
  (`TRUST_PROXY_HEADERS` gates `middleware.RealIP`; set `true` only once the proxy is
  live and the container port is unpublished — the D-06 topology), **GATE-07** inert
  path (unset `INSTANCE_PASSPHRASE` ⇒ routes flat), and the **`X-Instance-Gated` /
  `dt_session` cookie `Secure` requirement** (why the public origin must be HTTPS)
- `.planning/PROJECT.md` §"Key Decisions" (the passphrase-gate and rollback-safe
  rows), §"Deferred Items" (OPS-04, OPS-05, DPLY-09, GATE-08, CICD-15), §"Out of
  Scope" (multi-user auth, K8s/Swarm, IaC, `pull_request_target`)

### Existing code / config this phase modifies or depends on
- `.github/workflows/full-pipeline.yml` — `release` job (add `outputs.version`; svu
  step; `fetch-depth: 0`; `concurrency: release-<ref>` `cancel-in-progress: false`;
  `permissions: contents: write / packages: write`; the scanned-image artifact
  hand-off), top-level `permissions: contents: read`, top-level `concurrency`, the
  `changes` / `migration-check` / `n1-boot` jobs and `build-scan.needs:` (Phase 16 —
  the `deploy` job is **downstream of `release`**, not in `build-scan.needs:`), the
  SHA-pinned-actions + trailing `# vX.Y.Z` comment convention
- `docker-compose.yml` — the **dev-only** compose (`build: .`, published `8080:8080`,
  published `5432:5432`, `env_file: .env` + `environment:` `${INSTANCE_PASSPHRASE:-}`
  / `${TRUST_PROXY_HEADERS:-false}`). `deploy/compose.prod.yaml` **diverges**:
  `image:` not `build:`, adds `caddy`, no published app/Postgres port, bounded
  `app` restart policy, named Postgres volume, `stop_grace_period`. The
  "do NOT `docker compose config`" comment is load-bearing (Pitfall 3)
- `internal/httpserver/server.go` — chi router + 4-middleware chain + route
  registration; where `GET /version` is registered (public, outside the auth Group —
  same exemption class as `/health` and the SPA shell)
- `internal/httpserver/health.go` — `GET /health` pings the DB only (3s timeout),
  `200 {"status":"ok","db":"up"}` / `503`. **Unchanged** — the version check does not
  touch it (D-11, Pitfall 19)
- `internal/config/config.go` — env-only (`caarlos0/env`); only `DATABASE_URL` is
  `notEmpty`; `HTTP_PORT` (`envDefault:"8080"`) is the port var; `DISCORD_WEBHOOK_URL`
  optional; `INSTANCE_PASSPHRASE` / `TRUST_PROXY_HEADERS` (Phase 14)
- `internal/db/migrate.go` — boot-time `RunMigrations` (embedded `iofs`, bounded
  retry, `ErrNoChange` = success, `Up()` only) + the Phase 16 **ahead-of-source guard**
  (`maxSourceVersion` / `runMigrationsWithSource`). Rollback correctness rests on this
  — **unchanged** here, but the `n1-boot` UF-1 re-assert (D-20) keys on the
  `func maxSourceVersion(` token
- `internal/db/migrations/README.md` (Phase 16) — the expand/contract + N-1 rule the
  runbook cross-links, not re-states
- `cmd/server/main.go` — boot sequencing (`config.Load` → `logging` →
  `db.RunMigrations` → pool → services → `httpserver.New` + `poller.New` → SIGTERM
  graceful shutdown); where the `version` var is read for `GET /version`
- `Dockerfile` — three stages (`node:26-alpine3.24` SPA build → `golang:1.26.x-alpine`
  Go build with `go:embed` → `alpine:3.24` runtime, non-root UID 10001, `HEALTHCHECK`
  `wget .../health`). The Go build stage is where `-ldflags "-X ...version=<tag>"` is
  threaded; the build already receives the version context in the `release` job
- `Makefile` — `build` / `run` targets (local ldflags), `test-integration`,
  `coverage-gate`; a `make` target may print/stamp the dev version
- `web/app/` — React Router SPA; `web/app/lib/api.ts` (the `apiFetch` wrapper, Phase
  14's `401` interceptor + `X-Instance-Gated` latch); where a small `<VersionFooter>`
  fetches `GET /version` and renders. `web/app/lib/authStore.ts` context for gated vs.
  ungated rendering
- `.env.example` — already documents `INSTANCE_PASSPHRASE` + `TRUST_PROXY_HEADERS`
  with the Phase 17 topology note; add any prod-only var the compose file introduces
- git tags — `v1.8.1` is current `HEAD`; `svu current` / `svu next` drive
  `release.outputs.version`; the first v1.3 tag from *this* phase's deploy is the one
  D-20 waits for

### Standing procedures the runbook must document (from ROADMAP DPLY-07 additions)
- **VPS firewall allowlist during provisioning** — the instance is never both publicly
  reachable *and* pre-gate/pre-proxy (belt-and-suspenders alongside the passphrase
  gate; the app gate is the load-bearing control)
- **`TRUST_PROXY_HEADERS=true` only after** Caddy is confirmed live and the container
  port is unpublished (Phase 14 D-14) — unset/false at every earlier step
- **Passphrase rotation = revoke-all** (Phase 14 D-10): rotate `INSTANCE_PASSPHRASE`,
  redeploy — every session invalidated
- **Postgres backup + restore** (D-03): the recovery path when an image rollback also
  needs the schema restored (Pitfall 8)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`release` job's `svu` + `fetch-depth: 0` pattern** — `deploy` reads
  `needs.release.outputs.version` (after D-21 exposes it); no re-resolution needed.
- **`release` job's `concurrency: { group: ..., cancel-in-progress: false }`** — copy
  the exact shape for `deploy` (D-16).
- **Phase 16 `n1-boot` job** — the model for the deploy's rollout+health-poll shell
  (sustained-green loop, `::error::` teaching message on failure, teardown on
  `always()`), and the `guardcheck` step D-20 re-asserts.
- **`internal/httpserver` route-exemption pattern** (Phase 14) — `/health` and the SPA
  shell are registered outside the auth `Group`; `GET /version` joins that exempt set.
- **The Discord webhook already in the app's `.env`** — reused by the 80%-disk alert
  cron (D-18) and the rollback-failed alert (D-10); no new secret.
- **`cmd/coverage-report` / `cmd/migration-check`** — the precedent that CI helpers are
  small tested Go, not inline shell. `rollout.sh` / `provision.sh` are shell by
  necessity (they run on the VPS), but keep them small and lint them (`shellcheck` if
  cheap to add).
- **Phase 14 `logInstanceGateStatus`** — the "one secret-free boot-status line" idiom;
  a similar one-line "deployed version X" log on boot is cheap and useful.

### Established Patterns
- **SHA-pinned third-party actions with a trailing `# vX.Y.Z` comment** — any new
  `uses:` in the `deploy` job.
- **Top-level `permissions: contents: read`; jobs opt into more** — `deploy` needs
  only `contents: read` (D-22).
- **`if: github.event_name == 'push' && github.ref == 'refs/heads/main'`** — the
  `release` guard `deploy` mirrors exactly (Pitfall 11).
- **Functional options on constructors** (`With*`) — if `GET /version` needs the
  version passed into `httpserver.New`, use a `WithVersion(...)` option, not a
  positional param (~8 test call sites — ARCHITECTURE.md anti-pattern).
- **Boot-time embedded migrations, forward-only, additive** — Phase 16 codified this;
  D-25 keeps it.
- **Secret redaction on every error path** (`redactDSN` / `redactError`) — the
  rollout/deploy scripts extend the same mental model (D-24).

### Integration Points
- `.github/workflows/full-pipeline.yml` — new `deploy` job (`needs: [release]`,
  push/main guard + `workflow_dispatch`, `environment: production`, dedicated
  `concurrency`, `permissions: contents: read`), `release` gains `outputs.version`
  (D-21). The `deploy` job is **not** added to any existing `needs:` array — it is a
  leaf downstream of `release`.
- `deploy/` (new repo dir) — `compose.prod.yaml`, `Caddyfile`, `rollout.sh`,
  `provision.sh`, `pg-backup.sh`, `pg-restore.sh`, and the runbook (or the runbook
  under `docs/` — planner's call).
- `internal/httpserver/server.go` — register `GET /version` outside the auth Group.
- `internal/version` (or `main.version`) — new; read from ldflags.
- `Dockerfile` + `Makefile` — thread the version into `-ldflags`.
- `web/app/` — small version-footer component + its test; `web/app/lib/api.ts` (or a
  direct fetch) for `GET /version`.
- VPS host (new) — deploy user, SSH key + pinned host key, `.env` (chmod 600),
  `current-tag` seed, Caddy, firewall, disk-alert cron.
- GitHub — `production` Environment + SSH secrets (D-23).

</code_context>

<specifics>
## Specific Ideas

- **The rollback path is failure-path code** — Pitfall 5 says it's the #1 place deploys
  go wrong precisely because it's never exercised. The `workflow_dispatch` +
  `force_bad_tag` drill (D-19) is a **required UAT step against the live box**, not
  optional: assert rollback triggers, the previous version comes back healthy, the run
  ends **red**, and the Discord alert fires (SC #3).
- **A wrong SSH host key must fail the deploy** — SC #5 calls this out; the pinned
  `known_hosts` from a secret (D-07) is what makes it fail rather than TOFU-accept.
- **Logs must contain no passphrase, DSN, or Discord webhook URL** — SC #5; the
  post-run secret-grep assertion (D-24) is the automated backstop, on top of "no
  `set -x` near secrets" and "no `docker compose config`".
- **Two merges minutes apart deploy strictly one after the other** — SC #4; D-16's
  CI `concurrency` group + on-box `flock`.
- **The operator wants the live version visible at the URL** — not just a CI-internal
  check. `GET /version` + a small SPA footer string (D-11); the `docker inspect`
  assertion is the machine gate, the footer is the human one.
- **Keep the new surface minimal**, matching Phase 15/16 restraint — no new
  application dependency, no registry auth on the box, no server-side session store,
  `/health` untouched. The only app-code additions are the version stamp + `GET
  /version` + the SPA footer.

</specifics>

<deferred>
## Deferred Ideas

- **Blue/green / near-zero-downtime deploy** (Caddy health-gated upstream cutover, or
  connection draining) — DPLY-09 (Future). The ~2-10s swap gap is accepted and
  documented for v1.3 (D-04).
- **Scheduled + off-box Postgres backups with retention policy** — OPS-04 proper
  (Future). Only the basic operator-run scripts + procedure land here (D-03).
- **ghcr.io image-retention / cleanup job** — OPS-05 (Future). Not needed for rollback
  safety (rollback is local-pin only).
- **Session signing-key rotation without logging everyone out** — GATE-08 (Future).
  Passphrase rotation = revoke-all stays the only lever (Phase 14 D-10).
- **Hardening the deploy user past docker-group** (rootless Docker, sudo-scoped
  compose wrapper) — noted as a known ceiling in the runbook, not implemented.
- **Deeper `/health` readiness** (migration version, build version *in* `/health`,
  dependency checks) — the version check is a separate `GET /version` + `docker
  inspect`; a richer readiness endpoint is a later refinement, not this phase.
- **Poller leader-election via Postgres advisory lock** — a scaling concern the
  single-instance design doesn't have (Pitfall 10 / CLAUDE.md's noted future fix).
- **IaC / Terraform VPS provisioning** — past the current DevOps ceiling; the
  `provision.sh` wizard + runbook is the manual-but-documented approach.

### Reviewed Todos (not folded)
- **"Delete the stale tracked `web/package-lock.json`"** — frontend-deps hygiene,
  unrelated to VPS deploy. Stays a standalone quick task.
- **"Resolve the D-15 previous-release schema and query files from `--prev-tag`, not
  the process CWD"** — `migration-check` correctness that underpins N-1 rollback
  safety, but it is Phase 16 tooling debt; cleaner as its own quick task than folded
  into the deploy phase.
- **"Unify sqlscan's two hand-rolled quote/dollar-quote state machines"** — Phase 16
  `internal/sqlscan` refactor debt, unrelated to deploy.
- **"Move `shadcn` from dependencies to devDependencies in `web/package.json`"** —
  frontend-deps hygiene, unrelated to deploy.

</deferred>

---

*Phase: 17-automated-vps-deploy-with-health-gated-rollback*
*Context gathered: 2026-09-05*
