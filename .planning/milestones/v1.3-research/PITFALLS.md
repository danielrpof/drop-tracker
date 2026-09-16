# Pitfalls Research

**Domain:** Continuous deployment (SSH-to-VPS) + shared-passphrase auth gate + CI coverage-diff reporting, added to an existing single-binary Go/Postgres/GitHub-Actions app
**Researched:** 2026-08-27
**Confidence:** MEDIUM-HIGH (GitHub Actions fork/secret semantics, golang-migrate dirty-state, Go cookie/`crypto/subtle` behaviour verified against primary sources; VPS-compose specifics are well-established operational practice)

**Milestone under research:** v1.3 Continuous Deployment. Three feature areas, expected to become ~3 phases:

| Short name used below | Feature | Requirement |
|---|---|---|
| **Deploy phase** | GitHub Actions `deploy` job: SSH to VPS after `release`, `docker compose pull` + `up -d` pinned tag, poll `/health`, auto-rollback | DPLY-01 |
| **Gate phase** | chi passphrase-gate middleware: one env-var passphrase + session cookie, all routes except `/health` | v1.3 |
| **Coverage-comment phase** | PR comment with backend + frontend coverage delta vs. main baseline | CICD-13 |
| (cross-cutting) | Expand/contract migration discipline so `/health`-gated rollback is actually safe | DPLY-01 |

**Ordering note for the roadmap:** the Gate phase must land **before or in the same PR as** the first real public deploy. The moment DPLY-01 puts the app on a public URL, the watchlist API and the Discord webhook URL (returned/echoed in config surfaces and logs) are internet-exposed. Do not merge a deploy that reaches a public interface until the gate is enforced. If phases must be separate, gate the VPS at the network layer (firewall/allowlist, or bind to localhost + SSH tunnel) until the Gate phase ships.

---

## Critical Pitfalls

### Pitfall 1: Blind SSH host-key acceptance (`StrictHostKeyChecking=no` / no `fingerprint`)

**What goes wrong:**
The deploy step connects to the VPS without pinning the host key — either `ssh -o StrictHostKeyChecking=no`, an empty `known_hosts`, or `appleboy/ssh-action` without its `fingerprint:` input. The runner will connect to whatever answers on that IP/port. A DNS hijack, a re-provisioned VPS at a recycled IP, or an on-path attacker gets a shell session with your deploy key and every secret passed to that step.

**Why it happens:**
First-run friction: the runner has no `known_hosts` entry, the job fails with "Host key verification failed", and the fastest green-checkmark fix is to disable the check. Most `ssh-action` tutorials omit `fingerprint:`.

**How to avoid:**
- Capture the host key fingerprint once, out of band (`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` on the box via the hosting console), store it as a secret, and pass `fingerprint:` (appleboy) or pre-populate `~/.ssh/known_hosts` with the full host key line before any `ssh` call.
- Prefer ed25519 host keys; pin exactly one key type.
- STRIDE: this is the **Spoofing** mitigation for the deploy channel — call it out explicitly in the Deploy phase threat model.

**Warning signs:**
`StrictHostKeyChecking=no`, `-o UserKnownHostsFile=/dev/null`, `ssh-keyscan` piped straight into `known_hosts` inside the job (that trusts-on-first-use every run — no better than `no`), or no `fingerprint:` input on `appleboy/ssh-action`.

**Phase to address:** Deploy phase.

---

### Pitfall 2: Over-scoped deploy credentials (full-access key / root / broad PAT)

**What goes wrong:**
The deploy uses a passphraseless SSH key that lands in `~/.ssh/authorized_keys` for `root` (or a sudo-all user) with no `command=`/`from=` restriction, or a classic PAT with `repo`+`packages` scope where a fine-grained token would do. A leak of the Actions secret (see Pitfall 3) is then full VPS compromise, not just "can restart one compose project".

**Why it happens:**
Least-privilege on a single-operator box feels like ceremony. "It's my server" — so everything runs as root.

**How to avoid:**
- Dedicated unprivileged `deploy` user; membership in the `docker` group only (acknowledge in the threat model that `docker` group = root-equivalent, and that this is the accepted ceiling for a portfolio VPS).
- Restrict the key in `authorized_keys`: `from="<github-actions-egress-not-feasible>"` is impractical (GH ranges are broad), so instead pin `command="/opt/drop-tracker/deploy.sh",no-port-forwarding,no-agent-forwarding,no-pty` — the runner can invoke exactly one script and nothing else.
- Put the deploy logic in a checked-in `deploy.sh` on the box, not inline in the workflow YAML — keeps the forced command stable and reviewable.
- ghcr.io pull on the VPS: the package is already public (per PROJECT.md), so the VPS needs **no** registry credential at all. Do not add one.

**Warning signs:** key added to `root`'s `authorized_keys`; no `command=` restriction; workflow references `secrets.SSH_KEY` from a step that also runs arbitrary `script:` content; a PAT used where `GITHUB_TOKEN` would work.

**Phase to address:** Deploy phase.

---

### Pitfall 3: Secrets echoed into workflow logs / `set -x` in the deploy script

**What goes wrong:**
The passphrase, DB DSN, Discord webhook URL, or SSH key gets printed to the Actions log (public repo → world-readable). Common vectors: `set -x` / `bash -x` in `deploy.sh`, `docker compose config` (renders resolved env into stdout), `env |` dumps, `echo "DEBUG: $DATABASE_URL"`, or passing secrets as command-line args (visible in `ps` on the box and sometimes in error output). GitHub masks values registered as secrets, but **not** values derived from them (base64, URL-encoded, substring) and **not** anything transiting the SSH session's remote stdout unless independently masked.

**Why it happens:**
Debugging a broken first deploy. `set -x` is the reflexive move. `docker compose config` is the reflexive "is my env right?" check.

**How to avoid:**
- Never `set -x` in a script that has secrets in scope; if you must trace, `set -x` only around non-secret sections and `set +x` before touching env.
- Pass secrets to the container via `--env-file` / a root-owned `.env` on the box (mode 600), never as `-e KEY=value` on the command line, never via `docker compose config`.
- The remote `.env` is the source of truth on the VPS; the workflow should not transmit the full env every deploy — only push the file when it changes, over `scp` to a 600 path.
- gitleaks already gates commits/CI (CLAUDE.md) — extend the mental model: also scan the rendered workflow and `deploy.sh` for interpolated secrets.
- Add a post-deploy assertion in the workflow that greps its own captured remote output for known secret prefixes (`postgres://`, `https://discord.com/api/webhooks/`) and fails if found.

**Warning signs:** `set -x`/`bash -x` in deploy scripts; `docker compose config` anywhere in CI; secrets in `with: script:` inline; `run: echo` near a secret; masked `***` appearing mid-token (indicates a partial/derived value leaked).

**Phase to address:** Deploy phase. (Security-critical — foreground in the STRIDE **Information Disclosure** row.)

---

### Pitfall 4: Non-atomic compose swap → downtime window every deploy

**What goes wrong:**
`docker compose pull && docker compose up -d` on a single-container service stops the old container, then starts the new one. Between `stop` and a healthy new container (migrations run at boot — seconds to tens of seconds), every request 502s. If the new image is bad, you're down until rollback completes. There is no connection draining, so in-flight requests during the swap are killed (the app has graceful shutdown on SIGTERM per PROJECT.md, which helps, but the *gap* until the replacement is healthy is still hard-down).

**Why it happens:**
`up -d` "just works" locally where nobody notices a 10-second blip. Compose has no built-in rolling deploy for a single replica.

**How to avoid:**
- Accept a short controlled downtime (documented, portfolio-acceptable) **or** run two app services behind a tiny reverse proxy (Caddy/nginx/Traefik) and cut over: start `app-next`, poll its `/health`, then flip the proxy upstream, then stop `app-prev`. Traefik with Docker labels does this with health-gated load balancing.
- If accepting downtime: minimise it — `docker compose pull` (image already local before any stop), then `up -d --no-deps app`; keep migrations fast; make the deploy poll `/health` immediately.
- Set `stop_grace_period` in compose to comfortably exceed the app's shutdown timeout so graceful shutdown actually completes.
- Don't restart Postgres on an app-only deploy: `up -d --no-deps app` (or `up -d app`), never a bare `up -d` that may recreate the DB container.

**Warning signs:** bare `docker compose up -d` in the deploy script; no reverse proxy; `/health` poll starts immediately and passes because it's hitting the *old* container that hasn't stopped yet (false green — see Pitfall 6).

**Phase to address:** Deploy phase.

---

### Pitfall 5: A "rollback" path that was never actually exercised and doesn't work

**What goes wrong:**
The workflow has an `if: failure()` step that "rolls back to the previous image", but: it was never triggered on a real bad deploy; it references `$PREVIOUS_TAG` that was computed wrong (or is empty on first deploy); it re-runs `docker compose up` with a stale `.env`; it doesn't roll back the database (see Pitfall 8); or the rollback itself isn't health-checked so a failed rollback reports success. First time it runs for real is during an actual incident, and it makes things worse.

**Why it happens:**
Rollback code is failure-path code — it's not on the happy path, so it's never executed in normal testing. Nobody deliberately ships a broken build to test it.

**How to avoid:**
- **Deliberately deploy a known-bad image as an acceptance test for the Deploy phase** (e.g. an image whose `/health` returns 500, or that exits on boot). Assert: rollback triggers, previous version comes back healthy, workflow ends red, alert fires. This is a UAT criterion, not optional.
- Record the currently-running image digest *before* pulling the new one (`docker inspect` the running container), pin the rollback target to that exact digest — not "the second-newest tag in ghcr.io" (which retention GC may have deleted — Pitfall 7).
- Rollback must poll `/health` after restoring, and fail loudly (non-zero exit + Discord/GitHub notification) if the rollback is *also* unhealthy — that's the "call a human" state.
- Keep the previous image pinned on the box (`docker tag` it `drop-tracker:rollback` locally) so rollback never needs the network.
- On first-ever deploy, `$PREVIOUS` is empty — handle explicitly (skip rollback, just report failure).

**Warning signs:** rollback step exists but no test artifact showing it ran; rollback target is a tag/`svu current` rather than a captured digest; no `/health` check inside the rollback branch; `set -e` missing so a failed rollback command is swallowed.

**Phase to address:** Deploy phase. Verification: forced-bad-deploy drill in phase UAT.

---

### Pitfall 6: `/health` gate gives a false green (checks the wrong thing / wrong container / too early)

**What goes wrong:**
The deploy's health poll passes but the new version is actually broken. Causes: (a) it polls before the old container stopped, hitting the old one; (b) it polls `localhost:8080` which the reverse proxy still routes to the old upstream; (c) `/health` only checks "process is up", not "migrations applied + DB reachable + this build's schema expectations met"; (d) it polls once instead of ret/timeout-looping, catching a lucky moment; (e) it accepts any HTTP response including the proxy's own 502.

**Why it happens:**
`/health` was built in Phase 1 for liveness. "Is it 200?" feels sufficient. The distinction between liveness and deploy-readiness isn't obvious until a bad deploy slips through.

**How to avoid:**
- Poll the **new** container directly (by container name on the compose network, or a temporary published port) before cutting traffic — not through the proxy, not the old one.
- `/health` should report readiness: DB `SELECT 1`, migration version matches the binary's embedded expectation, and ideally an app-version field. It already reports DB connectivity (PROJECT.md) — extend it to include applied migration version + build version.
- Poll with bounded retries and an explicit total timeout (e.g. 30 attempts × 2s = 60s); require N consecutive successes; treat connection-refused and non-200 identically as "not ready yet" until timeout, then fail.
- Assert the health response body contains the version/tag just deployed — the strongest "is this actually the new build" check.

**Warning signs:** single `curl -f /health` with no loop; health poll hits the same URL end users hit during the swap; `/health` handler has no DB call; deploy succeeds suspiciously fast.

**Phase to address:** Deploy phase (health-poll logic) + a small `/health` enrichment (version + migration id) that could sit in the Gate phase or Deploy phase.

---

### Pitfall 7: `docker compose pull` races ghcr.io image retention / GC → `manifest unknown`

**What goes wrong:**
A retention policy or cleanup action on ghcr.io deletes "untagged" or old versions. Two failure modes: (a) the **rollback target** tag/digest was GC'd, so rollback can't pull it; (b) multi-arch builds store the tagged index plus untagged per-platform child manifests — deleting untagged children breaks the *tagged* parent, producing `manifest unknown` on an image that still appears in the UI. The current pipeline builds single-platform (`load: true`, `drop-tracker:scan`), so (b) is low risk today, but any move to `buildx --platform` reintroduces it. Also: a deploy that starts right as GC runs can pull a half-deleted manifest.

**Why it happens:**
"Disk/quota is filling on ghcr, add a cleanup action" — done without protecting multi-arch children or the last-known-good tag. `container-retention-policy` older than v3.1.0 didn't protect multi-arch children.

**How to avoid:**
- Don't rely on ghcr.io as the rollback source at all — pin the previous good image **locally on the VPS** (Pitfall 5). ghcr GC then can't break rollback.
- If adding a retention/cleanup action: pin it by commit SHA (CLAUDE.md rule), use `container-retention-policy` ≥ v3.1.0 (or `actions/delete-package-versions`) with `keep-n-most-recent` generous (≥ 10), and explicitly exclude tagged versions from deletion.
- Keep builds single-platform unless multi-arch is a stated goal; if multi-arch, verify the cleanup tool walks and protects child digests.
- Retention SLA: keep every released tag for the life of the project (they're tiny; SBOM + tag history is part of the portfolio story). Only prune genuinely untagged build-cache blobs.

**Warning signs:** a cleanup/retention workflow with `untagged` deletion and no multi-arch protection; rollback pulls from ghcr rather than local; `manifest unknown` in deploy logs; ghcr package UI shows a tag but `docker pull` 404s.

**Phase to address:** Deploy phase (local rollback pin). A ghcr cleanup job, if wanted, is its own small slice — flag as **needs its own mini-threat-model** if added.

---

### Pitfall 8: Non-backward-compatible migration bricks the rollback

**What goes wrong:**
`v1.4` ships a migration that drops a column / renames a table / adds a `NOT NULL` without default / tightens a constraint. `/health` passes, deploy succeeds. Later a *different* bad deploy triggers auto-rollback to `v1.3` — whose binary now runs against a schema it doesn't understand (missing column → every query errors), or the forward migration already destroyed data `v1.3` needs. Auto-rollback of the *image* without rollback of the *schema* leaves the system in a state neither version supports. This is the single highest-cost pitfall in the milestone because it turns a routine rollback into data loss / extended outage.

**Why it happens:**
golang-migrate's `down` files are rarely written or tested. Boot-time migration (PROJECT.md) only ever runs `up`. The team thinks "rollback = redeploy old image" and forgets the DB moved forward. The `/health` gate catches a *broken* migration, not an *irreversible* one.

**How to avoid:**
- **Mandate expand/contract (parallel-change) for every migration in v1.3+** (PROJECT.md already states this intent — make it an enforced checklist item, not a hope):
  - Expand: additive only (new nullable columns, new tables, new indexes `CONCURRENTLY`). Old binary keeps working.
  - Deploy code that writes both old + new.
  - Backfill.
  - Contract (drop old column/table) only in a *later* release, after the prior version is provably not coming back.
- A migration PR checklist gate: "Can the currently-deployed version run against this schema? If no → split it."
- Keep N-1 rollback compatibility as an explicit invariant: the previous released tag must run against `HEAD`'s schema. Add a CI job that boots the *previous* image against a DB migrated to `HEAD` and checks `/health` + a smoke query.
- Never put a destructive statement (`DROP COLUMN`, `DROP TABLE`, `ALTER ... TYPE` that loses data) in the same release that first stops using it.
- Boot-time migration must be **idempotent and forward-only**; the rollback image will try to migrate too — ensure an older image seeing a *newer* `schema_migrations` version does nothing (golang-migrate: older binary has fewer migration files, sees DB version ahead, no-ops — verify this is the actual behaviour and not an error in your wiring).

**Warning signs:** a `down` migration that can't restore data (`-- irreversible` comments); `DROP`/`NOT NULL`/`RENAME` in a migration; no test that the prior image boots against new schema; `down/` files that are empty or `-- TODO`.

**Phase to address:** Cross-cutting — owned by the Deploy phase (because rollback safety is a DPLY-01 acceptance criterion) and enforced on every migration thereafter. Add the "previous image vs. HEAD schema" CI job in the Deploy phase.

---

### Pitfall 9: Boot-time migration failure → unhealthy container in a restart crash loop

**What goes wrong:**
A migration fails mid-run (bad SQL, lock timeout, disk full, a `CREATE INDEX` that conflicts). golang-migrate sets the `dirty` flag and refuses all further migrations until someone runs `force`. The container exits non-zero on boot; `restart: unless-stopped` / compose restarts it; it fails identically forever. `/health` never comes up, deploy gate fails, auto-rollback fires — but if the failed migration was partially applied and non-idempotent, even the old image may hit the `dirty` state and refuse to boot. Now *both* versions are down and it needs manual `migrate force` on the box.

**Why it happens:**
Migrations run at boot with a retry loop (PROJECT.md Phase 1: "bounded retry loop") that's designed for "DB not ready yet", not "migration is broken" — retrying a broken migration just loops. The `dirty` flag is golang-migrate working as designed but is a manual-recovery state.

**How to avoid:**
- Distinguish transient (DB unreachable → retry) from terminal (migration SQL error / `dirty` → do **not** retry, exit fast, emit a loud structured error).
- Wrap each migration in a transaction where the DB allows it (Postgres DDL is mostly transactional — a failed migration rolls back cleanly and doesn't leave `dirty`). Note: `CREATE INDEX CONCURRENTLY` **cannot** run in a transaction — either avoid it at boot or handle its partial-failure explicitly (drop the invalid index on retry).
- Test every migration's failure path: run it against a DB where it will fail, confirm the container exits with a clear message and does *not* wedge `dirty` in a way the previous image can't get past.
- Consider a pre-deploy migration dry-run: the deploy script runs `migrate up` in a throwaway step / against a snapshot before flipping traffic — catches the failure before the swap.
- Have a documented `migrate force <v>` runbook and the `migrate` CLI available on the box.
- Set a bounded restart policy (`restart: on-failure:3` semantics via a supervisor, or compose `deploy.restart_policy` with `max_attempts`) so a crash loop stops and stays visibly down rather than thrashing.

**Warning signs:** retry loop with no error classification; `CREATE INDEX CONCURRENTLY` in a boot migration; no test of a failing migration; `dirty` appears in logs; container `Restarting (1)` repeatedly.

**Phase to address:** Deploy phase (failure classification + crash-loop bound + runbook). The `CONCURRENTLY`-not-transactional gotcha should be in the migration checklist from Pitfall 8.

---

### Pitfall 10: Migration lock contention (two app instances / deploy overlap)

**What goes wrong:**
During a rolling cutover (Pitfall 4 mitigation) or a double-deploy (Pitfall 12), two app containers boot simultaneously and both try to migrate. golang-migrate's Postgres driver takes `pg_advisory_lock`, so the second blocks until the first finishes — usually fine, but if the first holds the lock a long time (big `CREATE INDEX`) the second's boot-retry/health window can expire, failing the deploy. Worse in clustered setups: reports exist of the second instance erroring with "dirty database" when locking races (golang-migrate issue #119).

**Why it happens:**
The single-binary design assumes one instance. Any deploy strategy that briefly runs two (blue/green, or an accidental concurrent deploy) breaks that assumption. robfig/cron double-polling is the same class of problem (CLAUDE.md already flags cron has no leader election).

**How to avoid:**
- Keep exactly one instance running migrations. In a blue/green cutover, run migrations as a **separate one-shot step** (`docker compose run --rm app migrate` or a dedicated `migrate` service with `restart: no`) *before* starting `app-next`, so no two long-lived containers race.
- Rely on `pg_advisory_lock` as the backstop (it's automatic) but don't design around holding it for minutes — use `CREATE INDEX CONCURRENTLY` *outside* the locked migration path, or schedule big index builds as manual ops.
- Enforce deploy concurrency = 1 (Pitfall 12) so overlap can't happen from the CI side.
- If the app ever scales past one instance: Postgres advisory lock for the *poller* leader too (CLAUDE.md's stated future fix) — same milestone is a good time to add the pattern.

**Warning signs:** blue/green plan with both containers running `up` on boot; migrations taking >10s; "dirty database" under concurrency; deploy flakiness that correlates with two runs close in time.

**Phase to address:** Deploy phase.

---

### Pitfall 11: The deploy job runs on PRs / forks where secrets are absent

**What goes wrong:**
The current workflow triggers on `push:` **and** `pull_request:` (bare, all branches). Adding a `deploy` job without a tight `if:` means: it attempts to run on every PR, on forks (where `secrets.SSH_KEY` etc. are empty → the step either errors confusingly or, worse, `ssh` with an empty key/host does something undefined), and it could attempt to deploy from an unmerged PR branch. On a public repo, a fork PR could also be crafted to exfiltrate anything the job touches if you reach for `pull_request_target` to "fix" the missing-secrets error.

**Why it happens:**
Copy the `release` job's structure but forget its `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` guard. Or hit "secret is empty on fork PR", google it, and land on `pull_request_target` — which runs untrusted PR code with secrets and write token.

**How to avoid:**
- `deploy` job guard: `needs: [release]` + `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` (mirror `release`). It should only ever run after a successful `release` on main.
- Never use `pull_request_target` for anything in this milestone. If a PR needs to *report* something (coverage comment), use the `workflow_run` pattern or `pull_request` + artifact hand-off (Pitfall 13), not `pull_request_target`.
- Scope `permissions:` at the job level, minimum needed (`deploy` needs almost nothing from `GITHUB_TOKEN` — maybe `contents: read`).
- Use a GitHub **Environment** (`production`) on the deploy job with required reviewers or a branch restriction — secrets scoped to the environment can't be read by any other job/trigger.
- Fork PRs: the existing gates (vet/lint/test/trivy) are fine to run; just ensure no new job assumes a secret is present. A job that needs a secret should `if:` itself off for forks (`github.event.pull_request.head.repo.full_name == github.repository`).

**Warning signs:** `deploy` job with no `if:`; `pull_request_target` anywhere; secrets referenced in a job that runs on `pull_request`; environment secrets not used; a fork PR run showing a deploy step attempted.

**Phase to address:** Deploy phase (guard) + Coverage-comment phase (fork handling).

---

### Pitfall 12: Two concurrent deploys clobber each other

**What goes wrong:**
Two merges to main land minutes apart. Two `deploy` jobs SSH to the box concurrently: both `docker compose pull` (different tags), both `up -d`, interleaved — you can end up running tag B's container with tag A's `.env`, or both trying to migrate (Pitfall 10), or the health-poll of run 1 seeing run 2's container. The `release` job already has `concurrency: group: release-... cancel-in-progress: false` (queue, don't cancel) for exactly this reason — `deploy` needs the same, plus a lock **on the box**.

**Why it happens:**
The workflow-level `concurrency` block (`cancel-in-progress: pull_request` only) doesn't serialize main pushes. Each job needs its own.

**How to avoid:**
- `deploy` job: `concurrency: { group: deploy-production, cancel-in-progress: false }` — queue, never cancel a deploy mid-flight.
- Belt-and-suspenders: a `flock` on the deploy script on the VPS (`flock -n /run/drop-tracker-deploy.lock -c ...`) so even a manual deploy + a CI deploy can't overlap.
- Because `deploy` `needs: [release]` and `release` is already serialized, the queue mostly falls out — but make it explicit; don't rely on the transitive property.

**Warning signs:** no `concurrency:` on the `deploy` job; no on-box lock; deploy failures that only happen when two PRs merge close together; container running with mismatched env.

**Phase to address:** Deploy phase.

---

### Pitfall 13: Disk fills on the VPS with old images / volumes / build cache

**What goes wrong:**
Every deploy pulls a new `~100-300MB` image; the old ones are never removed. `docker compose pull` doesn't prune. After ~20-50 deploys the VPS disk is full; Postgres can't write WAL; the app and the DB both fail; the *next* deploy's `pull` fails for lack of space; and now you can't even roll back. Dangling volumes and `type=gha` isn't on the box but local build cache (if the box ever builds) piles up too.

**Why it happens:**
Disk exhaustion is slow and invisible until it's catastrophic. Nobody watches a portfolio VPS's `df`.

**How to avoid:**
- After a *successful, health-verified* deploy, prune deliberately: `docker image prune -f` plus explicitly remove images older than the current + rollback pin. **Never** `docker system prune -a` in the deploy path (it would delete the rollback image and Postgres's image).
- Keep exactly two app images on the box: `current` and `rollback` (retag on each successful deploy, then remove the now-3rd-oldest).
- Monitor: a cheap cron on the box that posts to the Discord webhook when `df` on the Docker root exceeds 80%. (Reuses the notification sink already in the app.)
- Put Postgres data on a volume with known capacity; alert before it's full.
- Size the VPS with headroom for at least a few images + DB growth.

**Warning signs:** no prune step; `docker system prune -a` in CI; `df -h` never checked; deploy fails with "no space left on device"; Postgres logs "could not extend file".

**Phase to address:** Deploy phase.

---

### Pitfall 14: Passphrase accepted via URL query param → logged, in Referer, in history

**What goes wrong:**
The gate accepts `?passphrase=...` (or the login form uses `method=GET`). The secret then lands in: chi's `httplog` access logs (PROJECT.md: request logging middleware is wired), the reverse-proxy logs, browser history, and the `Referer` header sent to any third-party resource the SPA loads (fonts, album art from Deezer/MusicBrainz CDNs, error trackers). One shared passphrase for the whole instance — leaking it once compromises the instance until rotated.

**Why it happens:**
Easiest possible "auth": a magic query string. Or the SPA does `fetch('/api/x?key=' + passphrase)`. Or a "shareable unlock link".

**How to avoid:**
- Passphrase only ever arrives in a `POST` body over HTTPS, to a dedicated `POST /auth` (or similar). Never a query param, never a GET, never a path segment.
- `httplog` config: confirm it does not log request bodies, and scrub the `Authorization` header and any auth cookie from logged headers (PROJECT.md already established a DSN-redaction pattern for slog — extend it).
- Set `Referrer-Policy: no-referrer` (or `same-origin`) as a response header so even accidental in-URL secrets don't leak cross-origin.
- The session cookie — not the passphrase — is what's sent on every subsequent request.

**Warning signs:** `r.URL.Query().Get("passphrase")`; login `<form method="get">`; passphrase visible in `httplog` output during a test; `Referer` containing the secret in Deezer/MB CDN request logs.

**Phase to address:** Gate phase. (STRIDE **Information Disclosure**.)

---

### Pitfall 15: Session cookie missing `Secure` / `HttpOnly` / `SameSite`

**What goes wrong:**
- No `HttpOnly` → any XSS in the SPA (or a malicious dependency) reads the session cookie and exfiltrates instance access.
- No `Secure` → cookie sent over plaintext HTTP; on a misconfigured proxy or an `http://` fallback it's sniffable.
- No / `SameSite=None` without reason → CSRF surface on the passphrase POST and on state-changing watchlist endpoints (Pitfall 20).
- `Domain` set too broad, or `Path` wrong so `/health` gets the cookie unnecessarily.

**Why it happens:**
`http.SetCookie` defaults are all-permissive: `Secure=false`, `HttpOnly=false`, `SameSite=0` (which browsers now treat as Lax-ish but it's undefined-intent). Local dev is `http://localhost` so `Secure` "breaks testing" and gets dropped.

**How to avoid:**
- `http.Cookie{ HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode, Path: "/", Name: "__Host-dt_session" }`.
- Use the `__Host-` prefix (forces `Secure`, `Path=/`, no `Domain`) — a strong, cheap invariant.
- `SameSite=Lax` is right here (SPA + API same origin; Lax still allows top-level navigation). `Strict` is fine too since there's no cross-site flow. Never `None`.
- Local dev over `http://localhost`: `Secure` cookies **are** sent to `localhost` by modern browsers (localhost is treated as a secure context) — so `Secure: true` does not actually break local dev. Verify rather than assume it needs a dev toggle. If a toggle is truly needed, gate it on an explicit `DEV_INSECURE_COOKIES` env var that is impossible to enable in the prod image path.
- Set a sane `Max-Age` / `Expires` (e.g. 7-30 days) and a server-side absolute session lifetime.

**Warning signs:** `SetCookie` without `HttpOnly`/`Secure`; `SameSite` unset; cookie name without `__Host-`; a `if dev { secure = false }` branch; cookie visible to JS in devtools.

**Phase to address:** Gate phase. (STRIDE **Spoofing / Elevation**, ASVS L1 session-management.)

---

### Pitfall 16: Session fixation — cookie value not rotated on successful auth

**What goes wrong:**
The server issues a session ID to unauthenticated visitors (or accepts a client-supplied one), and on successful passphrase entry it just marks that same ID authenticated. An attacker who fixes a victim's session ID beforehand (via a link, a subdomain cookie inject, an earlier XSS) is then logged in as the victim once the victim authenticates.

**Why it happens:**
Naive session design: "generate ID on first request, flip a boolean on login". Rotating on privilege change is a step people don't know to add.

**How to avoid:**
- On successful passphrase verification, **generate a brand-new session ID**, set it, and invalidate the old one. Never carry a pre-auth identifier across the auth boundary.
- Don't create server-side session state at all until authentication succeeds (stateless-until-login).
- Session ID = ≥128 bits from `crypto/rand`. Never derived from the passphrase, IP, or timestamp.
- If sessions are signed cookies (stateless) rather than server-stored: the token must include an issued-at, be HMAC'd with a server secret, and auth state must be *in* the signed payload — a pre-auth token simply has no valid "authenticated" claim to forge without the key.

**Warning signs:** session ID issued before login; login handler doesn't call the "new session" path; same cookie value before and after authenticating in a test.

**Phase to address:** Gate phase. (STRIDE **Spoofing**, ASVS L1 3.2.1 "session token regeneration on authentication".)

---

### Pitfall 17: Timing-unsafe passphrase comparison

**What goes wrong:**
`if submitted == os.Getenv("INSTANCE_PASSPHRASE")` — Go's `==` on strings short-circuits at the first differing byte. Over many requests an attacker can measure response-time differences to recover the passphrase byte-by-byte. Amplified because it's one static shared secret (worth attacking) and there's no lockout by default (Pitfall 18).

**Why it happens:**
`==` is the obvious thing. The timing-attack risk feels theoretical over a network — but it's a documented, cheap ASVS finding and trivial to fix.

**How to avoid:**
- `crypto/subtle.ConstantTimeCompare([]byte(submitted), []byte(expected)) == 1`.
- Guard the length leak: `ConstantTimeCompare` returns 0 immediately (fast) if lengths differ. Hash both sides to a fixed length first — `sha256.Sum256` each, then `ConstantTimeCompare` the digests — so length isn't observable.
- Better: store a hash of the passphrase (even a fast SHA-256, or Argon2id if you want defense against an env-var leak) and compare hashes.

**Warning signs:** `==`, `strings.EqualFold`, `bytes.Equal` on the secret; `subtle` not imported; comparison in the hot path with variable-time early return.

**Phase to address:** Gate phase. (ASVS L1 crypto; STRIDE **Information Disclosure**.)

---

### Pitfall 18: No rate limiting / lockout on the passphrase endpoint → brute-forceable

**What goes wrong:**
One shared passphrase, public URL, unlimited guesses. A weak passphrase (or one that's leaked and rotated to something guessable) falls to a scripted attack in hours. No lockout, no delay, no alerting — the logs just show a lot of failed auths nobody watches.

**Why it happens:**
"It's behind a passphrase" feels done. Rate limiting is a separate chunk of work. The app has `golang.org/x/time/rate` already (for external APIs) but nobody wires it to auth.

**How to avoid:**
- Per-IP rate limit on the auth endpoint: `golang.org/x/time/rate` (already a dependency) — e.g. 5 attempts/minute/IP, then 429 with backoff. Keyed on the real client IP (parse `X-Forwarded-For` correctly given the reverse proxy — trust only the proxy's appended value, Pitfall 22).
- Global failure counter → after N total failures in a window, alert to Discord (reuse the sink) and optionally lock the endpoint for a cooldown.
- Add a small fixed delay (e.g. 200-500ms) to every auth response, success or fail, to blunt both timing analysis and rapid guessing.
- Require a strong passphrase: document a minimum length/entropy in `.env.example`, and consider refusing to start if `INSTANCE_PASSPHRASE` is shorter than N chars or equals a known-default.
- This is the "no account lockout" ASVS control adapted to a single-secret model.

**Warning signs:** no limiter on `/auth`; identical fast response time on every failed attempt; no metric/log/alert for failed-auth rate; `.env.example` shows a short example passphrase that someone might ship as-is.

**Phase to address:** Gate phase. (ASVS L1 authentication; STRIDE **Elevation of Privilege**.)

---

### Pitfall 19: `/health` on the wrong side of the gate

**What goes wrong (two opposite failure modes):**
- **`/health` behind the gate:** the deploy's health poll gets a 401/redirect-to-login, reads it as "unhealthy", and every deploy auto-rolls-back — or the poll is given the passphrase and now the deploy workflow holds the instance secret unnecessarily (extra leak surface, Pitfall 3).
- **`/health` fully open AND verbose:** to keep the deploy simple, `/health` is exempted from the gate (correct) but it returns build version, git SHA, migration version, DB host/name, dependency versions, uptime, env — all readable by anyone on the internet. That's free reconnaissance (exact version → known CVEs; DB details; internal hostnames).

**Why it happens:**
The gate middleware is applied to the router root; `/health` either gets swept in by accident, or gets a blanket exemption and its response body is never reviewed for what it discloses.

**How to avoid:**
- Exempt exactly `/health` (exact path match, not a prefix — `/health*` could expose `/healthz-debug`) from the gate middleware. Unit-test: unauthenticated `GET /health` → 200; unauthenticated `GET /anything-else` → 401; unauthenticated `GET /healthsomething` → 401.
- Two-tier health: a **public** `/health` that returns only `{"status":"ok"}` + HTTP 200/503 (enough for the deploy gate and uptime checks), and a **gated** `/health/details` (or `/debug/status`) behind the passphrase that has version, migration id, DB connectivity detail for operator use.
- The deploy gate polls the *public* `/health` — no secret needed in the workflow. The "is it the new version?" assertion (Pitfall 6) then needs the version somewhere public: either put a bare version string in public `/health`, or read the deployed tag from the container image label / `docker inspect` on the box instead of over HTTP.
- Don't leak DB DSN/host anywhere in either response (PROJECT.md redaction pattern applies).

**Warning signs:** gate applied as `r.Use(gate)` on the root router with no carve-out test; `/health` body contains `version`, `database`, `commit`, `env`; deploy workflow references the passphrase secret; `curl https://<public>/health` returns internals.

**Phase to address:** Gate phase (carve-out + response minimisation) — coordinate with Deploy phase (what the health poll reads).

---

### Pitfall 20: CSRF on the passphrase POST and on state-changing endpoints

**What goes wrong:**
Once authenticated, the session cookie is sent automatically by the browser. A malicious page can `POST` to `/api/watchlist` (add/remove artists) or to `/auth` (login-CSRF: log the victim into the attacker's session) using the victim's cookie. The watchlist write endpoints (Phase 2) and the passphrase POST are the targets.

**Why it happens:**
The gate adds a *cookie*, which reintroduces CSRF that a token-in-header scheme wouldn't have. Teams add cookie auth and forget CSRF is now in scope.

**How to avoid:**
- `SameSite=Lax` (Pitfall 15) already blocks the classic cross-site `POST` form CSRF for the state-changing endpoints — necessary baseline.
- For defense in depth on an SPA: require a custom header (e.g. `X-Requested-With` or a double-submit CSRF token) on all non-GET `/api/*` requests; a cross-site attacker can't set custom headers without CORS permission. Cheap to add in the chi middleware + the SPA's `fetch` wrapper.
- Strict CORS: no wildcard `Access-Control-Allow-Origin`, no `Allow-Credentials: true` with a permissive origin. Same-origin only (SPA is served from the same binary — CORS can be entirely absent/deny).
- Login-CSRF: on `POST /auth`, rotate the session (Pitfall 16) and consider a pre-flight token; `SameSite=Lax` + same-origin check on the auth POST covers most of it.

**Warning signs:** non-GET `/api/*` handlers with no CSRF/custom-header check; CORS middleware with `AllowOriginFunc` returning true; `Allow-Credentials: true`; `SameSite=None`.

**Phase to address:** Gate phase. (STRIDE **Tampering / Spoofing**, ASVS L1 CSRF.)

---

### Pitfall 21: In-memory sessions wiped on every deploy → forced re-login each ship

**What goes wrong:**
Sessions live in a Go map in process memory. Every deploy (frequent — on every merge to main) restarts the container, so every user is logged out on every deploy. Annoying for a single operator; actively bad UX for a demo ("why do I have to log in again?"). Tempts a move to sticky sessions or skipping the gate.

**Why it happens:**
In-memory map is the fastest session store to write. The deploy cadence making it painful isn't obvious until CD is live.

**How to avoid:**
- **Stateless signed-cookie sessions**: the cookie is `HMAC-SHA256(payload, serverKey)` where payload = `{authenticated:true, issued_at, nonce}`. No server storage; survives restarts; logout = clear cookie + (optionally) bump a key version. This fits the "minimal dependency footprint" constraint — ~30 lines with stdlib `crypto/hmac`.
- Signing key from an env var (`SESSION_SIGNING_KEY`, 32 bytes). If unset, generate one at boot **and** log a warning that sessions won't survive restart — but for CD, require it to be set (fail fast if missing in prod).
- If server-side sessions are preferred later: store them in the Postgres already present (a `sessions` table), not memory.
- Key rotation story: support two valid keys (current + previous) so rotating the signing key doesn't nuke all sessions instantly.

**Warning signs:** `map[string]Session` in a handler struct; sessions gone after `docker compose up`; no `SESSION_SIGNING_KEY` in `.env.example`; plan to add Redis "just for sessions" (over-engineering for one operator).

**Phase to address:** Gate phase.

---

### Pitfall 22: Reverse proxy / `X-Forwarded-For` handled wrong (IP spoofing, wrong scheme)

**What goes wrong:**
Behind the proxy, `r.RemoteAddr` is the proxy, and `X-Forwarded-For` is client-controlled unless the proxy overwrites/appends it. If the rate limiter (Pitfall 18) or any allowlist keys on a naively-parsed `X-Forwarded-For`, an attacker spoofs it to bypass rate limiting or forge audit logs. Separately, if the app doesn't trust `X-Forwarded-Proto`, it may think requests are `http` and refuse to set `Secure` cookies, or build wrong redirect URLs.

**Why it happens:**
`chi/middleware.RealIP` exists but trusts `X-Forwarded-For`/`X-Real-IP` unconditionally — fine *only* if the app is never reachable except through the proxy. If the container port is also published, clients hit it directly and spoof freely.

**How to avoid:**
- Bind the app container to the Docker network only; publish nothing except through the proxy. Then `RealIP` is safe. If the app port must be published, don't use `RealIP` / parse XFF from untrusted hops.
- Trust only the *last* value appended by your own proxy, or configure the proxy to replace `X-Forwarded-For` with the true client IP.
- Set `X-Forwarded-Proto` at the proxy and honor it for `Secure`-cookie / HTTPS decisions; ensure the proxy terminates TLS and the internal hop can be `http` without the app downgrading cookie flags.
- HSTS (`Strict-Transport-Security`) at the proxy or app once HTTPS is confirmed working.

**Warning signs:** app port published in compose on the VPS alongside a proxy; `middleware.RealIP` with a directly-reachable app; rate limiter keyed on raw XFF; `Secure` cookies not being set in prod because the app sees `http`.

**Phase to address:** Deploy phase (proxy/network topology) + Gate phase (IP handling in the limiter).

---

### Pitfall 23: SPA shows a broken/blank state on 401 instead of a login prompt

**What goes wrong:**
The gate returns `401` (or `403`) to `fetch` calls from the embedded SPA. The SPA's API layer (`web/app/lib/api.ts`) treats non-2xx as a generic error → the watchlist page renders an error toast or an infinite spinner or a blank screen. On first visit (no cookie) the user sees a broken app, not "enter passphrase". Also: after session expiry mid-session, in-flight requests 401 and the UI half-breaks.

**Why it happens:**
The gate is added server-side; the SPA's existing error handling wasn't written with an auth-challenge state in mind. Returning `401` with a JSON body the SPA doesn't special-case.

**How to avoid:**
- Define the contract: unauthenticated API request → `401` with a small JSON body `{"error":"unauthenticated"}`. The SPA's `api.ts` intercepts `401` globally → redirect to / render the `<PassphraseGate>` view, preserving intended destination.
- For the initial HTML document request (not XHR): the gate can either serve the SPA shell always (SPA renders the login view based on a `GET /auth/status` check) **or** redirect to a `/login` route. Serving the shell + a status endpoint is cleaner for an SPA and avoids a full-page redirect flash.
- Handle session expiry: a `401` on any call re-triggers the gate view without losing unsaved state where feasible; show "session expired, re-enter passphrase".
- Test (Vitest + RTL, per the existing suite): mock a `401` from `api.ts`, assert the passphrase view renders, assert a successful auth re-fetches the original data.
- `/health` and static assets (JS/CSS/favicon) must be reachable without auth or the login page itself won't load (Pitfall 19 — the carve-out must include the SPA's static bundle, or the login view can't render).

**Warning signs:** `401` from the API renders a generic error component; blank page on first visit; no `<PassphraseGate>` component; static asset requests 401 so the app shell won't boot; no RTL test for the unauthenticated path.

**Phase to address:** Gate phase (both the server contract and the SPA handling — they're one feature).

---

### Pitfall 24: Coverage-comment job — `GITHUB_TOKEN` missing `pull-requests: write`

**What goes wrong:**
The workflow's top-level `permissions: contents: read` (current state) means `GITHUB_TOKEN` cannot post a PR comment. The coverage job fails with `403 Resource not accessible by integration`, or silently no-ops depending on the action. Bumping the **top-level** permission instead of scoping it to the one job over-grants every other job (including `build-scan`, `release`, and any third-party action) write access to PRs.

**Why it happens:**
`permissions` is subtle: setting it at workflow level applies to all jobs; the fix looks like "add `pull-requests: write`" at the top.

**How to avoid:**
- Add `permissions: { contents: read, pull-requests: write }` **at the coverage-comment job level only**, leaving the workflow default and every other job untouched.
- Keep `release`'s existing job-level `permissions: { contents: write, packages: write }` as-is; don't merge concerns.
- Use a well-known pinned-by-SHA action (`marocchino/sticky-pull-request-comment`, or post via `gh pr comment` / the REST API in a `run:` step) — CLAUDE.md requires SHA pins for third-party actions.

**Warning signs:** `403 Resource not accessible by integration`; `pull-requests: write` at workflow level; comment step "succeeds" but no comment appears.

**Phase to address:** Coverage-comment phase.

---

### Pitfall 25: Coverage comment breaks / is useless on fork PRs

**What goes wrong:**
On a PR from a fork, `GITHUB_TOKEN` is read-only regardless of the `permissions:` block, and secrets are absent. The coverage-comment step hard-fails, turning the whole PR red for an external contributor over a *reporting* feature. The instinctive fix — `pull_request_target` — runs the fork's untrusted code (including its build/test scripts that produce the coverage numbers) with a write token and repo secrets: a classic exfiltration hole.

**Why it happens:**
`pull_request` + write permission works for same-repo branches (this repo's normal flow) and looks done, until an external PR arrives.

**How to avoid:**
- This is a solo portfolio repo — fork PRs are rare. Simplest correct approach: the coverage step runs under `pull_request`, computes the delta, and if it can't comment (fork), it writes the delta to the **job summary** (`$GITHUB_STEP_SUMMARY`) and to the check output instead of failing. `continue-on-error` on the comment step, or an `if:` guarding it to same-repo PRs only: `if: github.event.pull_request.head.repo.full_name == github.repository`.
- The robust pattern if fork comments are truly needed: `pull_request` job uploads `coverage.out` + the computed delta as an **artifact**; a separate `workflow_run`-triggered job (runs in the base repo's context, has write token, but checks out **no** untrusted code — only downloads the artifact) posts the comment. More machinery than this project needs unless external contribution becomes real.
- Never `pull_request_target` with a checkout of the PR head.

**Warning signs:** coverage job fails on a fork PR; `pull_request_target` with `actions/checkout` using `ref: ${{ github.event.pull_request.head.sha }}`; external contributor PRs blocked by a reporting job.

**Phase to address:** Coverage-comment phase. (Security — foreground the `pull_request_target` prohibition.)

---

### Pitfall 26: Comment spam — a new coverage comment on every push

**What goes wrong:**
Every push to the PR branch appends another coverage comment. A 15-commit PR has 15 near-identical comments burying the actual review discussion. Notifications fire each time.

**Why it happens:**
The naive "create comment" API call has no idea a previous one exists.

**How to avoid:**
- Use a **sticky / upsert** comment: find the bot's prior comment by a hidden marker (`<!-- coverage-report -->`) and edit it in place; create only if absent. `marocchino/sticky-pull-request-comment` does exactly this; or `gh` + a `--edit-last` / search-then-PATCH in a script.
- One comment per PR, always showing the latest delta.

**Warning signs:** multiple coverage comments on one PR; comment step always calls "create"; no hidden marker string.

**Phase to address:** Coverage-comment phase.

---

### Pitfall 27: Baseline fetch fails on first run / new branch / shallow checkout

**What goes wrong:**
The comment computes "delta vs. main baseline" by fetching a stored baseline (an artifact, a branch, a gist, or by checking out `main` and re-running coverage). First time the feature ships there is **no** stored baseline → the step errors or reports a nonsense `+80.0%` delta. Also: `actions/checkout` defaults to `fetch-depth: 1`, so `git diff origin/main` or checking out `main` for a baseline run fails ("unknown revision"). And re-running the full backend+frontend suite on `main` to get a baseline doubles CI time on every PR.

**Why it happens:**
The baseline is a chicken-and-egg dependency that only bites once, so it's easy to not plan for. Shallow checkout is the default and silently lacks history.

**How to avoid:**
- Treat a missing baseline as "delta unavailable — showing absolute coverage only", not an error. First-run and any fetch failure degrade gracefully.
- Establish the baseline on the **push to main** side: a job on `push: main` writes `coverage.out` / the summary number to a durable location — a dedicated `coverage-baseline` branch (orphan), a repo variable, or a long-retention artifact keyed by `main`. The PR job *reads* it, never recomputes `main`'s coverage.
- The existing `test` job already produces `coverage.out` (`make test-integration`) and `frontend-test` produces Vitest coverage — reuse those exact artifacts; don't add a second coverage run.
- If any git-history operation is needed (`git merge-base`, diff coverage), set `fetch-depth: 0` on that job's checkout (the `gitleaks` and `release` jobs already do this — consistent).
- Store both numbers (backend %, frontend %) and diff each independently — they have different thresholds (80 / 70).

**Warning signs:** first PR after the feature ships shows `+80%` or errors; `git diff origin/main` → "unknown revision"; baseline job recomputes main coverage on every PR (CI time doubles); `fetch-depth` not set where history is used.

**Phase to address:** Coverage-comment phase.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|---|---|---|---|
| Accept a few-second hard-down window on deploy instead of a reverse-proxy blue/green cutover | No proxy to run/configure; simpler script | Every deploy blips; can't demo zero-downtime; a bad deploy = outage until rollback | Acceptable for v1.3 portfolio if documented; revisit when "zero-downtime deploy" becomes a portfolio talking point |
| In-memory sessions | Fastest to write | Forced re-login on every deploy (frequent under CD); no multi-instance path | **Never** under CD — go straight to signed-cookie sessions |
| `StrictHostKeyChecking=no` to get the first deploy green | Unblocks the phase in 5 min | MITM/spoofing hole in the deploy channel forever; nobody comes back to fix it | **Never** |
| One shared passphrase, no rotation mechanism | Trivial auth for a single operator | Leak = full instance access until manual env change + redeploy; no per-user revocation | Acceptable as the v1.3 model (multi-user explicitly out of scope) *if* rate-limited, hashed, timing-safe, and rotation is a documented one-liner |
| Skip writing/testing `down` migrations | Migrations ship faster | Rollback silently can't restore schema/data; discovered during an incident | **Never** in a milestone whose headline feature is auto-rollback — expand/contract + N-1 compatibility test is mandatory |
| `docker system prune -a` in the deploy script to "keep disk clean" | One line, disk stays clean | Deletes the rollback image and Postgres image; next rollback fails; next DB restart re-pulls | **Never** — prune only app images older than current+rollback |
| Rollback target = "previous tag in ghcr.io" | No local state to manage | ghcr retention/GC can delete it; network dependency during an incident | **Never** — pin the previous image locally on the box |
| Coverage baseline = recompute `main` coverage on every PR | No baseline storage to build | Doubles CI minutes per PR; slower feedback | Acceptable very short-term; replace with a `push: main` baseline artifact within the same phase |
| `continue-on-error: true` on the whole deploy job to stop it blocking merges | Merges never blocked by deploy flakiness | Broken deploys go unnoticed; CD becomes theater | Only on the *notify* step, never the deploy/rollback steps |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|---|---|---|
| GitHub Actions → VPS (SSH) | `StrictHostKeyChecking=no`; secrets as CLI args; `set -x` with secrets in scope | Pin host key (`fingerprint:`/`known_hosts`); forced `command=`; `--env-file` on the box; no `-x` near secrets |
| GitHub Actions → ghcr.io (VPS pull) | Adding a registry login on the VPS; relying on ghcr as rollback source | Package is public → no creds; rollback image pinned locally |
| golang-migrate (boot) ↔ auto-rollback | Assuming image rollback = full rollback; irreversible migration in the same release it stops using a column | Expand/contract; N-1 schema compatibility CI job; destructive DDL only one release later |
| golang-migrate ↔ blue/green cutover | Two containers run `up` on boot, race the advisory lock, one goes `dirty` | Run migrations as a one-shot step before starting the new container; one migrator at a time |
| chi gate middleware ↔ `/health` | Prefix match sweeps in `/health*`; or blanket-exempt with a verbose body | Exact-path exempt `/health` only; public `/health` returns `{"status":"ok"}`, details endpoint is gated |
| chi gate ↔ embedded SPA static assets | Gate blocks the JS/CSS bundle → login page can't render | Carve out `/assets/*`, `index.html`, favicon alongside `/health` |
| chi `httplog` ↔ passphrase | Passphrase in query string or request body gets logged | POST body only; scrub auth header + cookie from logs; `Referrer-Policy: no-referrer` |
| Reverse proxy ↔ `Secure` cookies / RealIP | App sees internal `http` hop → won't set `Secure`; trusts spoofable `X-Forwarded-For` | Honor `X-Forwarded-Proto`; app not directly reachable; trust only the proxy's XFF value |
| GitHub `GITHUB_TOKEN` ↔ PR comment | `pull-requests: write` set workflow-wide; hard-fail on fork PRs | Job-scoped permission; degrade to job summary on forks; never `pull_request_target` |
| Vitest/Go coverage artifacts ↔ baseline diff | Recompute main coverage per PR; shallow checkout breaks `git` history ops | Reuse existing `coverage.out`/Vitest artifacts; baseline written on `push: main`; `fetch-depth: 0` where history is used |
| Discord webhook ↔ public deployment | Webhook URL exposed via a config/status endpoint once the app is public | Gate everything but `/health`; never echo the webhook URL in any response or log |

## Performance / Operational Traps

| Trap | Symptoms | Prevention | When It Breaks |
|---|---|---|---|
| VPS disk fills with old images | Deploy fails "no space left"; Postgres "could not extend file"; can't roll back | Prune to current+rollback after each success; Discord alert at 80% disk | ~20-50 deploys on a small VPS |
| Long `CREATE INDEX` in a boot migration holds the advisory lock | Deploy health-poll times out; second container blocked; occasional `dirty` | `CREATE INDEX CONCURRENTLY` outside the migration path, or as a manual op | First time the DB has non-trivial row counts |
| Migration retry loop spins on a broken migration | Container `Restarting (1)` forever; `/health` never green | Classify transient vs. terminal; exit fast on SQL error/`dirty`; bounded restart attempts | First genuinely broken migration |
| Full backend+frontend suite re-run for coverage baseline on every PR | PR CI time roughly doubles | Baseline from `push: main` artifact; PR only reads it | Immediately, every PR |
| Every-push coverage comments | Dozens of comments per PR; review buried | Sticky/upsert comment keyed on a marker | Any PR with more than ~3 pushes |
| No connection draining on compose swap | 502s + killed in-flight requests each deploy | `stop_grace_period` ≥ app shutdown timeout; blue/green if it matters | Every deploy; worse under real traffic |

## Security Mistakes

| Mistake | Risk | Prevention |
|---|---|---|
| Blind SSH host-key accept | Deploy channel MITM → shell on VPS with deploy key + secrets | Pin host key fingerprint / known_hosts |
| Deploy key on `root`, no `command=` restriction | Secret leak = full VPS compromise | Unprivileged `deploy` user, `docker` group only, forced command |
| `set -x` / `docker compose config` / secrets as CLI args | Passphrase, DSN, webhook URL in world-readable Actions logs | No `-x` near secrets; `--env-file`; post-run secret-grep assertion |
| Passphrase in URL query param or GET form | Secret in access logs, browser history, `Referer` to Deezer/MB CDNs | POST body only; `Referrer-Policy: no-referrer`; scrub logs |
| Cookie without `HttpOnly`/`Secure`/`SameSite` | XSS steals session; plaintext sniff; CSRF | `__Host-` prefix, `HttpOnly`, `Secure`, `SameSite=Lax` |
| Session ID not rotated on auth | Session fixation → attacker rides victim's authenticated session | New session ID on successful passphrase; no pre-auth state |
| `==` on the passphrase | Timing side-channel recovers the shared secret byte-by-byte | `subtle.ConstantTimeCompare` on fixed-length hashes; store a hash |
| No rate limit on `/auth` | One shared secret, unlimited guesses, no alerting | Per-IP `rate.Limiter`, global failure alert to Discord, fixed response delay, min-entropy check at boot |
| `/health` verbose and open | Version/CVE recon, DB host/name, internal hostnames leak to the internet | Public `/health` = `{"status":"ok"}` only; details behind the gate |
| CSRF on watchlist writes / login POST | Cross-site page mutates the watchlist or fixes a session using the victim's cookie | `SameSite=Lax` + custom-header requirement on non-GET `/api/*` + strict/absent CORS |
| `pull_request_target` to fix fork-PR secret access | Untrusted PR code runs with write token + secrets → exfiltration | Never; use `pull_request` + artifact + `workflow_run` if fork comments are needed |
| `pull-requests: write` at workflow scope | Every job + third-party action can now write to PRs | Scope the permission to the coverage-comment job only |
| Spoofable `X-Forwarded-For` feeding the auth rate limiter | Attacker rotates fake IPs to bypass the limit | App not directly reachable; trust only the proxy-appended XFF value |
| ghcr cleanup deletes multi-arch child manifests / last-good tag | `manifest unknown` on a live tag; rollback target gone | Local rollback pin; retention action ≥ v3.1.0 pinned by SHA; keep all released tags |

## "Looks Done But Isn't" Checklist

- [ ] **SSH deploy:** host key pinned (not TOFU, not `no`) — verify a deliberately-wrong host key fails the deploy.
- [ ] **Auto-rollback:** actually exercised against a known-bad image in phase UAT — verify previous version returns healthy and the run ends red with an alert.
- [ ] **Rollback target:** a locally-pinned image digest, not a ghcr tag — verify rollback works with the network to ghcr blocked.
- [ ] **Health gate:** polls the *new* container (not the old one, not through the proxy), loops with timeout, and asserts the deployed version.
- [ ] **Migrations:** every v1.3+ migration is additive-only (expand/contract); a CI job boots the *previous* image against `HEAD`'s schema and it stays healthy.
- [ ] **Boot migration failure:** terminal errors exit fast (no infinite retry); `dirty`-state runbook exists; bounded restart attempts.
- [ ] **Deploy triggers:** `deploy` job has `if: push && ref == main` + `needs: [release]` + its own `concurrency` group — verify it does not run on PRs or forks.
- [ ] **Disk:** post-deploy prune keeps only current+rollback; Discord alert wired at 80% disk.
- [ ] **Passphrase:** POST-only, `subtle.ConstantTimeCompare`, per-IP rate limit, min-entropy check at boot — verify none of these are missing.
- [ ] **Cookie:** `__Host-` prefix, `HttpOnly`, `Secure`, `SameSite=Lax`; session ID rotates on login (fixation test).
- [ ] **Sessions:** survive a container restart (signed cookie or Postgres-backed) — verify you stay logged in across a deploy.
- [ ] **`/health`:** exact-path exempt; unauthenticated `GET /health` → 200, `GET /healthz`/`/health/x` → 401; body contains no version/DB/env detail.
- [ ] **SPA static bundle + favicon:** reachable unauthenticated so the login view can render.
- [ ] **SPA 401 handling:** unauthenticated API call renders `<PassphraseGate>`, not a broken/blank screen — RTL test covers it.
- [ ] **CSRF:** non-GET `/api/*` requires a custom header / token; CORS is same-origin/absent.
- [ ] **Coverage comment:** `pull-requests: write` scoped to that job only; sticky/upsert (one comment per PR); degrades gracefully on missing baseline and on fork PRs; reuses existing coverage artifacts.
- [ ] **Logs:** passphrase, DSN, webhook URL confirmed absent from Actions logs and `httplog` output (grep assertion).

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---|---|---|
| Secret leaked into a public Actions log | MEDIUM | Rotate the secret (passphrase env + redeploy; new SSH key; regenerate Discord webhook; rotate DB password); delete the workflow-run logs; audit access; add the secret-grep assertion so it can't recur |
| Irreversible migration already shipped, rollback needed | HIGH | Restore DB from the most recent pre-deploy backup/snapshot; redeploy the old image against the restored schema; if no backup, hand-write a corrective migration — this is why backups + expand/contract are non-negotiable |
| `dirty` migration state, both images refuse to boot | HIGH | SSH in; `migrate` CLI on the box; inspect `schema_migrations`; manually finish/undo the partial DDL; `migrate force <last-good>`; boot old image; write the corrective migration |
| VPS disk full, can't deploy or roll back | MEDIUM | SSH in; `docker image prune` (not `-a`); remove all but current+rollback images; free space; verify Postgres recovered; then add the alert + prune step |
| Auto-rollback ran but rollback is also unhealthy | HIGH | Deploy is hard-down; SSH in; `docker compose` up the locally-pinned last-known-good digest; if DB moved, restore snapshot; post-incident: fix the rollback test gap |
| Passphrase brute-forced (no rate limit) | LOW-MEDIUM | Rotate `INSTANCE_PASSPHRASE` to a strong value + redeploy; add rate limiting + failure alerting; review access logs for what was touched |
| Session fixation exploited | MEDIUM | Rotate `SESSION_SIGNING_KEY` (invalidates all sessions); ship the session-regeneration fix; review logs |
| Coverage comment spamming / blocking fork PRs | LOW | Switch to sticky comment; add the same-repo `if:` guard / job-summary fallback; delete the noise comments |
| `deploy` job ran on a PR / fork | LOW-MEDIUM | Add the `if:` guard immediately; rotate any secret the misfired job could have exposed; move deploy secrets into a `production` Environment |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---|---|---|
| 1 Blind SSH host-key accept | Deploy | Deploy fails when host key doesn't match the pinned fingerprint |
| 2 Over-scoped deploy credentials | Deploy | Deploy user is unprivileged; key has a forced `command=`; no registry cred on the box |
| 3 Secrets in workflow logs | Deploy | Post-run grep of captured output for `postgres://` / `discord.com/api/webhooks` / key material fails the run if found; no `set -x` near secrets |
| 4 Non-atomic compose swap | Deploy | Measured downtime window documented; `stop_grace_period` ≥ shutdown timeout; Postgres not recreated on app deploy |
| 5 Untested rollback | Deploy | Forced-bad-image drill in phase UAT: rollback triggers, old version healthy, run red, alert fired |
| 6 False-green health gate | Deploy (+ `/health` enrich) | Health poll hits the new container directly, loops w/ timeout, asserts deployed version |
| 7 ghcr GC races the pull | Deploy | Rollback works with ghcr unreachable; released tags retained; any cleanup action SHA-pinned + multi-arch-safe |
| 8 Irreversible migration bricks rollback | Cross-cutting (Deploy owns the gate) | CI job boots previous image against `HEAD` schema, `/health` + smoke query pass; migration PR checklist enforced |
| 9 Boot-migration crash loop | Deploy | Failing-migration test: container exits fast with a clear message, no infinite retry, previous image can still boot; `force` runbook exists |
| 10 Migration lock contention | Deploy | Migrations run as a single one-shot step before the new container starts; no two migrators |
| 11 Deploy runs on PR/fork | Deploy (+ Coverage-comment) | `deploy` job proven not to run on a PR or fork; no `pull_request_target` anywhere |
| 12 Concurrent deploys | Deploy | `concurrency: deploy-production, cancel-in-progress: false` + on-box `flock`; two rapid merges serialize |
| 13 VPS disk fills | Deploy | Post-success prune keeps only current+rollback; 80%-disk Discord alert verified |
| 14 Passphrase in query param | Gate | `httplog` output contains no passphrase; only `POST /auth` accepts it; `Referrer-Policy` header set |
| 15 Cookie flags missing | Gate | Cookie is `__Host-` prefixed, `HttpOnly`, `Secure`, `SameSite=Lax` — asserted in a handler test |
| 16 Session fixation | Gate | Session ID differs before vs. after successful auth (test) |
| 17 Timing-unsafe compare | Gate | `subtle.ConstantTimeCompare` on hashed values; `==`/`bytes.Equal` on the secret absent (grep) |
| 18 No auth rate limiting | Gate | 6th attempt/minute/IP → 429; global failure alert to Discord; boot refuses a too-short passphrase |
| 19 `/health` wrong side of gate | Gate (+ Deploy coordination) | Unauth `GET /health` → 200 minimal body; `GET /health/x` → 401; details endpoint gated |
| 20 CSRF | Gate | Non-GET `/api/*` without the custom header → rejected; CORS same-origin/absent |
| 21 In-memory sessions wiped on deploy | Gate | Still authenticated after `docker compose up` (signed cookie or Postgres store) |
| 22 Proxy / XFF handling | Deploy (topology) + Gate (limiter key) | App not directly reachable; `Secure` cookies set in prod; limiter can't be XFF-spoofed |
| 23 SPA broken on 401 | Gate | RTL test: `401` from `api.ts` renders `<PassphraseGate>`; static bundle loads unauthenticated |
| 24 `GITHUB_TOKEN` PR-comment perms | Coverage-comment | `pull-requests: write` scoped to that job only; comment appears on a same-repo PR |
| 25 Fork PR coverage comment | Coverage-comment | Fork PR: step degrades to job summary, PR not blocked; no `pull_request_target` |
| 26 Comment spam | Coverage-comment | 3 pushes to a PR → exactly one (edited) coverage comment |
| 27 Baseline fetch fails first run | Coverage-comment | First PR after ship: "delta unavailable", absolute % shown, no error; baseline written on `push: main`; `fetch-depth: 0` where history used |

## Sources

- GitHub Actions — fork PR secret/token semantics: [community discussion #196886](https://github.com/orgs/community/discussions/196886), [2i2c blog: Action secrets only from non-forked repos](https://2i2c.org/blog/github-action-secrets-forked-repositories/), [check-spelling: pull_request gets a read-only GITHUB_TOKEN](https://github.com/check-spelling/check-spelling-docs/blob/gh-pages/Feature:-Support-pull_request_target.md), [external-secrets/ok-to-test](https://github.com/external-secrets/ok-to-test) — confidence HIGH
- `appleboy/ssh-action` host-key `fingerprint` / MITM: [README](https://github.com/appleboy/ssh-action/blob/master/README.md), [issue #275](https://github.com/appleboy/ssh-action/issues/275) — confidence HIGH
- golang-migrate dirty state, `pg_advisory_lock`, `force` recovery: [migrate FAQ.md](https://github.com/golang-migrate/migrate/blob/master/FAQ.md), [database package docs](https://pkg.go.dev/github.com/golang-migrate/migrate/v4/database), [issue #119 (dirty under clustered locking)](https://github.com/golang-migrate/migrate/issues/119), [issue #317](https://github.com/golang-migrate/migrate/issues/317) — confidence HIGH
- ghcr.io untagged/multi-arch manifest deletion, `manifest unknown`: [snok/container-retention-policy](https://github.com/snok/container-retention-policy), [container-retention-policy issue #43](https://github.com/snok/container-retention-policy/issues/43), [GoBlog issue #60](https://github.com/jlelse/GoBlog/issues/60) — confidence MEDIUM
- Go stdlib: `crypto/subtle.ConstantTimeCompare`, `net/http.Cookie` (`Secure`/`HttpOnly`/`SameSite` defaults), `__Host-` cookie prefix (RFC 6265bis / MDN), `SameSite=Lax` CSRF semantics, `crypto/hmac` for signed cookies — general knowledge, confidence HIGH
- OWASP ASVS L1: session token regeneration on auth (3.2.1), authentication rate limiting, timing-safe comparison — general knowledge, confidence HIGH
- Expand/contract (parallel-change) migration pattern — Sato/Fowler, well-established practice — confidence HIGH
- Project artifacts read: `PROJECT.md`, `.claude/CLAUDE.md`, `.github/workflows/full-pipeline.yml`, `docker-compose.yml`, `.planning/ROADMAP.md` — confidence HIGH

---
*Pitfalls research for: continuous deployment (SSH-to-VPS) + shared-passphrase gate + CI coverage-diff reporting, added to drop-tracker v1.3*
*Researched: 2026-08-27*
