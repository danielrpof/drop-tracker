# Pitfalls Research

**Domain:** Go release-tracking service (external API polling + diff/notify) with a "Full Pipeline" GitHub Actions CI/CD practice goal
**Researched:** 2026-08-04
**Confidence:** MEDIUM (cross-checked against official docs, GitHub issues/discussions, and multiple independent write-ups; no single-source claims treated as authoritative)

## Critical Pitfalls

### Pitfall 1: MusicBrainz rate-limit violations get you fully blocked, not throttled

**What goes wrong:**
The scheduler fires MusicBrainz requests for every watchlist entry back-to-back (e.g. a `for` loop over N artists with no pacing), or fan-out happens across concurrent goroutines. MusicBrainz doesn't gracefully degrade — once you exceed ~1 request/second, it declines **100%** of your requests with HTTP 503 until your rate drops, not just the requests over the limit. A missing or generic `User-Agent` header (e.g. Go's default `Go-http-client/1.1`) can get the app throttled independently of rate, or rejected outright.

**Why it happens:**
Developers test with 1-2 watchlist artists locally where sequential polling never approaches the limit, then it breaks once real usage has 10+ artists and the cron tick tries to check them all in the same cycle.

**How to avoid:**
- Build a single rate-limited MusicBrainz client (e.g. `golang.org/x/time/rate` with `rate.NewLimiter(rate.Every(1*time.Second), 1)`) that every call — poller and search-proxy alike — goes through. Never let two code paths hit MusicBrainz independently.
- Set a descriptive `User-Agent` (`drop-tracker/0.1.0 (contact-url-or-email)`) as a package-level constant from day one, not an afterthought before shipping.
- Serialize per-cycle MusicBrainz calls (one artist at a time), don't parallelize them.

**Warning signs:** Intermittent 503s in logs that correlate with watchlist size growing; search-proxy endpoint (used by the UI) and the background poller racing for the same rate budget causes user-facing search to fail during a poll cycle.

**Phase to address:** Early — when the MusicBrainz client is first built (before the scheduler/poller phase), since the rate limiter needs to be a foundational property of the client, not bolted on later.

---

### Pitfall 2: robfig/cron double-fires jobs on slow ticks or restarts, causing duplicate poll cycles

**What goes wrong:**
robfig/cron has no built-in overlap protection — if a poll cycle for a large watchlist takes longer than the configured interval (e.g. MusicBrainz's 1 req/sec limit makes a 30-artist cycle take 30+ seconds), the next scheduled tick can start a second poll cycle concurrently with the first, doubling API load and creating race conditions in the diff logic (see Pitfall 3).

**Why it happens:**
The interval is chosen for "how often should we check," not for "how long does a full cycle actually take" — these get conflated until watchlist size grows or an external API slows down.

**How to avoid:**
- Wrap the poll job with `cron.SkipIfStillRunning` (or `DelayIfStillRunning`) middleware from `robfig/cron/v3` — never use raw `AddFunc` without it.
- Since this is a single-binary, single-instance deployment (per the locked architecture), distributed locking is not needed for v1 — but if a future phase adds horizontal scaling, note that robfig/cron has no multi-instance coordination and would double-fire across replicas without an external lock (Redis SETNX, Postgres advisory lock).
- Schedule in UTC explicitly; avoid scheduling right at DST transition hours if any user-facing "at this local time" scheduling is ever added (not currently a requirement, but worth noting for the config layer).

**Warning signs:** Duplicate log lines for the same artist within seconds of each other; Discord notifications arriving in pairs; MusicBrainz 503s that correlate with poll-cycle duration approaching the interval.

**Phase to address:** Early/mid — when the scheduler is wired up (same phase as Pitfall 1), because the overlap-guard middleware is a one-line addition at construction time but very easy to forget and hard to retrofit once diff logic assumes single-flight execution.

---

### Pitfall 3: Diff-against-seen-store race conditions produce duplicate or missing notifications

**What goes wrong:**
Two variants:
1. **Duplicate notifications:** the poll cycle reads the "seen" row, compares to the fetched release, decides it's new, sends the Discord notification, *then* writes the "seen" row — if the process crashes or a second cycle overlaps between the notify and the write, the same release gets notified again on the next cycle.
2. **Missed updates:** the diff only checks "does this release ID exist in the seen store," so an edited release (e.g. MusicBrainz release-group gets a deluxe edition added, or a track's guest-feature credit is corrected after initial ingestion) is silently ignored because the ID already exists — even though the *content* changed and that's exactly the kind of change (tracklist/feature changes) this app is supposed to catch.

**Why it happens:** "Have I seen this ID before" is the naive first implementation, and reordering "notify" before "commit as seen" feels natural procedurally but isn't crash-safe or idempotent.

**How to avoid:**
- Store a content hash or a `last_seen_payload`/version marker (e.g. hash of tracklist + release date + title) per tracked entity, not just existence. Diff against the hash, not just presence, to catch edits.
- Make the write-then-notify ordering atomic and ordered correctly: write the new "seen" state (or an "outbox" row representing the pending notification) inside the same DB transaction as detecting the change, and only mark the outbox entry "delivered" after the Discord webhook call succeeds. This is the transactional-outbox pattern — it decouples detection (which must be exactly-once against the DB) from delivery (which can safely retry).
- Give each detected change a stable idempotency key (e.g. `sha256(artist_id + release_id + change_type + content_hash)`) so that even if the notifier retries, it can no-op against an already-delivered outbox row instead of re-sending.

**Warning signs:** Same release notified twice in Discord after a crash/restart during a poll cycle; a known tracklist edit (deluxe edition added) on MusicBrainz never triggers a "deluxe/tracklist changed" alert even though the release existed already.

**Phase to address:** Mid — this is core to the diff engine phase; get the outbox/idempotency-key design right before the notifier phase is built on top of it, since retrofitting an outbox after the notifier already assumes "detect and notify in one step" means redesigning both.

---

### Pitfall 4: go:embed ships a stale or dev-only frontend build inside the Go binary

**What goes wrong:**
`go:embed` embeds whatever is on disk in the `dist/` (or equivalent) directory *at Go build time*. If CI builds the Go binary without first running `npm run build` (or runs it against a stale/cached `dist/`), the binary silently embeds an old or empty frontend — the app still compiles and runs, it just serves outdated or broken UI, and nothing in `go build` will catch this. A related but separate mistake: forgetting the SPA fallback route (serving `index.html` for any unmatched path so React Router can handle client-side routes) means deep-linking or a browser refresh on a non-root route 404s in production even though it worked in dev.

**Why it happens:** The Go build and the frontend build are two separate toolchains with no dependency graph between them — `go build` has no idea `dist/` needs regenerating, and local dev typically runs the Vite dev server directly (bypassing embed entirely), so the embed path is only exercised at the very end, often first noticed in CI or in the shipped image.

**How to avoid:**
- CI pipeline order must be explicit and enforced: `npm ci && npm run build` (produces `dist/`) **before** `go build`/`docker build` — never let the Dockerfile's Go build stage run without a preceding, verified frontend build stage in the same multi-stage Dockerfile (so there's no "which ran last" ambiguity locally either).
- Add a build-time guard: fail the build if `dist/index.html` doesn't exist before the `go:embed` directive is compiled (a simple `Makefile`/script check), so a missing frontend build fails loudly instead of embedding an empty/stale directory.
- Implement the catch-all SPA route (serve `index.html` for any path not matched by the API prefix and not an existing embedded file) in the Go router from the start, and test it with an actual non-root-path request in CI, not just `/`.

**Warning signs:** Local `docker build` producing a working image while a clean CI checkout produces a broken/blank UI (stale `dist/` reused locally); refreshing the browser on any route other than `/` returns 404 in the built image but works in `npm run dev`.

**Phase to address:** Mid — when the go:embed wiring and Dockerfile are built (UI integration phase), before the CI pipeline phase locks in build ordering.

---

### Pitfall 5: Discord webhook rate limits and message-size limits break notifications during release bursts

**What goes wrong:**
Album drop days (e.g. a Friday with many watched artists releasing simultaneously) can produce a burst of notifications in a short window. Discord webhooks allow roughly 30 requests/60s per webhook and 5 requests/5s per channel (shared across all webhooks posting to that channel) — a naive loop that posts one webhook call per detected change with no pacing will start hitting 429s, and mishandled 429s (retrying immediately, or retrying without honoring `Retry-After`) can escalate into a global rate-limit ban on that IP/token for up to 10 minutes, during which *all* notifications silently fail. Separately, Discord message content is capped at 2000 characters — a notification that includes a full tracklist for a large deluxe edition can exceed this and get rejected outright unless truncated or split into an embed.

**Why it happens:** Works fine in manual testing (one notification at a time); breaks under the exact "many releases detected in one poll cycle" scenario that's the app's core value proposition.

**How to avoid:**
- Queue detected notifications and send them serially with spacing (e.g. 400ms+ apart) rather than firing concurrently per detected change.
- Always read and honor the `Retry-After` header/`retry_after` field on a 429 response before retrying; back off further on repeated 429s rather than retrying immediately.
- Use a Discord embed (title/fields) instead of a raw content string for release details, and truncate/paginate tracklists that could approach the 2000-character content limit or embed field limits.
- Treat notification delivery as retryable-but-idempotent (ties back to Pitfall 3's outbox key) so a failed/rate-limited send can be retried later without risk of double-posting once it succeeds.

**Warning signs:** Notifications missing for some artists on high-release-volume days (Fridays) while working fine on quiet days; Discord API logs showing 429s with increasing frequency during bursts.

**Phase to address:** Mid — when the notifier is built, before it's exercised at any real watchlist scale.

---

### Pitfall 6: The "Full Pipeline" GitHub Actions workflow is built in the wrong order, hiding failures or wasting CI minutes

**What goes wrong:**
A common anti-pattern is running expensive/slow steps (Trivy image scan, SBOM generation) before cheap/fast steps (lint, unit tests), so a trivial lint failure isn't caught until minutes into the run. Another is running the security/secret scan (gitleaks) *after* the image has already been pushed to ghcr.io, meaning a leaked secret is already public in a registry layer by the time it's detected. A third is not caching Go module downloads or Docker layers at all, making every CI run rebuild everything from scratch — turning a pipeline that should take 2-3 minutes into 10+.

**Why it happens:** The pipeline is often built by appending steps in the order features get built (test first, then "let's add scanning," then "let's add SBOM," then "let's add publish") rather than being deliberately ordered by cost and blast-radius.

**How to avoid:**
- Order stages: lint/vet → unit tests → gitleaks secret scan → build → Trivy scan of the built artifact/image → SBOM generation → semantic-release (version/tag) → push to ghcr.io. Fail fast on cheap checks before spending minutes on Docker builds and scans.
- Never push to ghcr.io before both gitleaks and Trivy have passed on that exact build artifact — the registry push should be the last step, gated on everything else green.
- Cache Go build/module cache (`actions/setup-go`'s built-in caching, or explicit `actions/cache` on `~/go/pkg/mod` and `~/.cache/go-build`) and Docker layers (`docker/build-push-action` with `cache-from`/`cache-to: type=gha`) — note that BuildKit `--mount=type=cache` (used for `go mod download` inside the Dockerfile) is a *separate* cache mechanism from the GHA layer cache and needs its own cache action, or builds silently stop benefiting from module caching even though layer caching looks "on."
- Pin third-party actions (`aquasecurity/trivy-action`, `gitleaks/gitleaks-action`, etc.) to full commit SHAs, not version tags — in March 2026 a real supply-chain compromise force-pushed malicious commits onto Trivy's own action tags, exfiltrating CI secrets from pipelines that used tag-pinned versions while appearing to run normally. Tags are mutable; SHAs are not.

**Warning signs:** CI runs taking 8-10+ minutes for a one-line change; a scan step failing on a build that was already pushed to the registry in an earlier step; `git log` on a pinned action tag showing a different commit than what was originally reviewed.

**Phase to address:** Early-to-mid — the pipeline should be scaffolded with correct stage ordering and caching from the first CI phase, since reordering stages later means rewriting job dependencies (`needs:`) across the whole workflow file.

---

### Pitfall 7: semantic-release misconfiguration in a Go repo (no package.json) silently no-ops or fails the release step

**What goes wrong:**
semantic-release defaults to the `@semantic-release/npm` plugin, which expects a `package.json` to bump and (optionally) publish. In a pure Go repo with no `package.json`, this either errors out immediately or, if someone adds a placeholder `package.json` just to satisfy it, risks semantic-release trying to `npm publish` a meaningless package or misreading the "version" field as the source of truth instead of git tags. Related: forgetting `GITHUB_TOKEN` permissions (`contents: write`, and `packages: write` if the release step also needs to push to ghcr.io) causes the tag/release-creation step to fail with an opaque 403 rather than a clear "missing permission" message.

**Why it happens:** Every semantic-release tutorial defaults to a Node.js/npm project; adapting it to a Go repo requires actively removing/replacing the default plugin, which is easy to skip if the setup is copy-pasted from a Node example.

**How to avoid:**
- Use a `.releaserc`/`release.config.js` that explicitly lists only the plugins needed: `@semantic-release/commit-analyzer`, `@semantic-release/release-notes-generator`, `@semantic-release/github` (for GitHub releases/tags), and skip `@semantic-release/npm` entirely — do not add a placeholder `package.json` to work around it.
- If the pipeline needs semantic-release to determine the version used to *tag the Docker image* pushed to ghcr.io, have the release step output the computed version (via `@semantic-release/exec` or reading the created git tag) and feed it into the `docker/build-push-action` tag, rather than trying to make semantic-release itself aware of Docker/Go.
- Explicitly set `permissions: contents: write` (and `packages: write` if applicable) on the release job in the workflow YAML — don't rely on repo-level default token permissions, which may be read-only depending on org settings.

**Warning signs:** semantic-release step failing with `ENOPKGJSON`/"no package.json found" type errors; a release job that appears to succeed but no git tag or GitHub release is actually created; ghcr.io image tags not matching the semantic-release-computed version.

**Phase to address:** Mid — when the release/versioning step is added to the pipeline, after lint/test/scan are already working, since it depends on the rest of the pipeline being stable first (semantic-release should be the last "gate" before publish).

---

### Pitfall 8: Multi-stage Dockerfile non-root user is set at the wrong stage, or bloats the final image

**What goes wrong:**
Two related mistakes: (1) switching `USER nonroot` in the final stage *before* copying files that need root to place (or before an entrypoint script needs execute permission set) causes "permission denied" at container start rather than at build time, which is a worse debugging experience; (2) using a full `golang:1.x` image (not `-alpine` or a multi-stage split) as the *final* runtime stage instead of just the *builder* stage, shipping the entire Go toolchain, or copying the frontend `node_modules`/build tooling into the final image instead of only the built `dist/` output — both massively bloat the final image and expand the attack surface Trivy will flag against.

**Why it happens:** Copy-pasted Dockerfile examples often don't clearly separate "what needs root" (installing packages, chown-ing directories) from "what should run as non-root" (the actual server process), and it's easy to reuse the builder's base image for the runtime stage out of inertia rather than switching to a minimal one.

**How to avoid:**
- Use a true multi-stage build: stage 1 builds the frontend (`node:xx` → produces `dist/`), stage 2 builds the Go binary (`golang:1.x` → `CGO_ENABLED=0 go build`), stage 3 is a minimal runtime base (`gcr.io/distroless/static-debian12:nonroot` or `alpine` + explicit non-root user) that only `COPY --from=` the compiled binary and embedded assets — never the toolchains.
- Perform any `chown`/directory-creation as root *before* the `USER nonroot` instruction; if using distroless nonroot (UID 65532), remember it can't `chown` at runtime since there's no shell — any writable directories must be prepared and owned correctly during the build stage with `COPY --chown=`.
- Since the frontend is embedded via `go:embed` (not served by Node at runtime), the final image needs zero Node.js presence at all — confirm the final stage's `FROM` line never derives from a Node or full Go SDK base.
- Order Dockerfile instructions so rarely-changing layers (base image, `go.mod`/`go.sum` download) come before frequently-changing ones (application source) to maximize layer cache hits between builds.

**Warning signs:** Final image size in the hundreds of MB instead of tens of MB (`docker images` sanity check); container crash-looping with "permission denied" only in the built image, not in local `go run`; Trivy flagging OS-package CVEs that only exist because the final image still has a full Alpine/Debian package manager present.

**Phase to address:** Mid — Dockerfile phase, ideally verified with an image-size and non-root smoke test before the CI pipeline phase wires Trivy scanning against it (so the scan phase starts against an already-lean, correctly-permissioned image rather than debugging both at once).

---

### Pitfall 9: golang-migrate and sqlc drift apart — schema and generated queries silently mismatch

**What goes wrong:**
`golang-migrate` orders migration files by numeric version prefix; if `sqlc` is configured to read the same migration directory as its schema source, it parses files in lexicographic (string) order instead. If numeric and lexicographic ordering ever disagree (e.g. `2_add_column.sql` sorts after `10_add_table.sql` lexicographically but before it numerically), `sqlc` can generate Go query code against an incorrect intermediate schema state — this doesn't fail loudly; it produces code that compiles but silently expects the wrong columns/types, and the mismatch only surfaces at runtime against a real database (or worse, only in production if CI's test DB happened to apply migrations in an order that masked it). Separately, if a migration is written but `sqlc generate` isn't re-run (or vice versa — `sqlc` code changed without a matching migration), CI can pass on stale generated code that no longer matches the actual applied schema.

**Why it happens:** golang-migrate and sqlc are independent tools with independent (and different) file-ordering assumptions; nothing forces them to be re-synced, and a small team/solo project easily forgets to re-run `sqlc generate` after every migration change.

**How to avoid:**
- Always zero-pad migration version prefixes (`0001_`, `0002_`, ... `0010_`) so numeric and lexicographic ordering are always identical — this fully eliminates the ordering-drift class of bug and costs nothing.
- Add a CI step that runs migrations against a fresh ephemeral Postgres (matching the actual `golang-migrate` CLI/library, not sqlc's own SQL parsing) and then runs `sqlc generate --file sqlc.yaml` and fails the build with a diff if the generated code doesn't match what's committed (`git diff --exit-code` after generate) — this catches "forgot to regenerate" drift directly, and running actual migrations (not just sqlc's schema parser) also catches migration syntax errors that only manifest against a real database engine.
- Keep migrations and `sqlc` query files reviewed together in the same PR/commit; never let a migration merge without the corresponding `sqlc generate` output committed alongside it.

**Warning signs:** `sqlc generate` succeeding locally but CI integration tests failing with "column does not exist"; generated query code referencing a column that was renamed/dropped in a later migration than the one sqlc actually parsed; migration file names that aren't consistently zero-padded (a code-review smell to catch early).

**Phase to address:** Early — establish the zero-padded migration naming convention and the CI "migrate then generate then diff" check as soon as the first migration and sqlc query are written, since renaming existing migrations later is disruptive (breaks `schema_migrations` version history in any environment that already applied them).

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|-----------------|------------------|
| Skipping the outbox/idempotency-key pattern, notifying directly from the diff step | Faster to build v1 notifier | Duplicate/missed notifications on any crash or overlap (Pitfall 3) | Never past the first working demo — retrofit before any real watchlist usage |
| Using version-tag pinning for GitHub Actions instead of SHA pinning | Simpler, human-readable workflow YAML, easier to bump | Exposed to supply-chain tag-rewrite attacks (real incident, Pitfall 6) | Only for first-party/GitHub-owned actions (`actions/checkout`, etc.) with a documented risk acceptance; never for smaller third-party security-tooling actions |
| Committing a placeholder `package.json` to make semantic-release "just work" | Avoids configuring `.releaserc` plugins properly | Confusing dual-source-of-truth for versioning, risk of accidental `npm publish` attempts | Never |
| Running `golang:1.x` (full SDK) as the final Docker runtime stage during early development | One Dockerfile to maintain, easier debugging (has a shell) | Bloated image, larger Trivy CVE surface, slower registry push | Acceptable only in local dev `docker-compose`, never in the image built/scanned/pushed by CI |
| Hardcoding a single fixed poll interval with no jitter across all watchlist entries | Simple `cron.New()` call, easy to reason about | All entries poll in lockstep, worsening MusicBrainz rate-limit contention as watchlist grows | Acceptable at very small watchlist sizes (<5 artists); revisit once search-proxy live-lookups and poller are competing for the same rate budget |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|-----------------|-------------------|
| MusicBrainz | No rate limiting, or per-goroutine limiting instead of a single shared limiter | One shared `rate.Limiter` (1 req/sec) used by every code path that calls MusicBrainz, including the search-proxy |
| MusicBrainz | Generic/default Go `User-Agent` | Set an explicit, descriptive `User-Agent` constant at client construction |
| Deezer | Assuming Deezer's rate limits/error semantics mirror MusicBrainz's | Handle Deezer's own "Quota limit exceeded" (error code 4) response distinctly; Deezer is not officially documented for rate-limit numbers, so build in defensive backoff even without a published number |
| Deezer | Searching with no query parameter, expecting filter-only search | Deezer search requires a non-empty query string; validate before calling |
| Discord webhooks | Firing all detected notifications concurrently in a burst | Serial queue with spacing, honoring `Retry-After` |
| GitHub Container Registry | Assuming `GITHUB_TOKEN` has `packages: write` by default | Explicitly set `permissions: packages: write` on the publish job |
| golang-migrate + sqlc | Trusting sqlc's schema parse order matches golang-migrate's applied order | Zero-pad migration filenames so both orderings agree |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|-----------------|
| Sequential, unbounded per-artist MusicBrainz polling in a single cron tick | Poll cycle duration grows linearly with watchlist size, eventually exceeding the poll interval | Rate-limited client + cycle-duration monitoring + `SkipIfStillRunning` guard | Somewhere around 60+ watched artists at a 1-minute interval with MusicBrainz's 1 req/sec cap |
| No caching in CI (Go modules, Docker layers) | Every PR's CI run takes the full cold-build time | `actions/setup-go` caching + `docker/build-push-action` with `cache-from`/`cache-to: type=gha` | Immediately — this is a day-one cost, not a scale threshold |
| GitHub Actions cache silently exceeding the 10GB per-repo cap | Cache hit rate quietly drops, builds slow down with no code change | Monitor cache size in Actions UI; prune stale cache keys | Once accumulated Docker layer + Go module + node_modules caches cross ~10GB combined |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Using `pull_request_target` with a checkout of the PR head to run tests/build | Fork PR can exfiltrate `GITHUB_TOKEN`/secrets via modified test/build code | Use plain `pull_request` for untrusted-code build/test (no secrets by default); never check out fork code under `pull_request_target` |
| Pinning security-tooling GitHub Actions (Trivy, gitleaks) by version tag | Tag can be force-pushed to malicious commit (real March 2026 incident) — secrets exfiltrated while pipeline appears healthy | Pin to full commit SHA; Dependabot can still track/bump SHA pins |
| Pushing to ghcr.io before gitleaks/Trivy gates pass | A leaked secret or known-critical CVE ends up published in a registry layer, which is very hard to fully scrub afterward | Order the pipeline so publish is strictly the last, fully-gated step |
| Loosening Trivy severity threshold or adding blanket `--ignore-unfixed` globally to "make the pipeline green" | Real vulnerabilities silently stop blocking builds | Block only on CRITICAL/HIGH with `--exit-code 1`, use a documented, per-CVE `.trivyignore` with expiry/comment rather than a global threshold change |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| No SPA fallback route for client-side React Router paths | Refreshing the browser on any non-root route 404s in production | Serve `index.html` for any unmatched non-API path from the Go router |
| Search-proxy endpoint blocked/slow because it shares the same MusicBrainz rate budget as the background poller | Users see slow/failed search-to-add while a poll cycle is running | Either prioritize interactive search requests over background poll requests in the shared limiter, or accept and surface latency in the UI rather than silently timing out |
| Silent notification loss when Discord rate-limits are hit during a release burst | Users miss real release alerts on exactly the days they'd want them most (Fridays) | Persist detected-but-undelivered changes (outbox) and retry with backoff rather than fire-and-forget |

## "Looks Done But Isn't" Checklist

- [ ] **MusicBrainz/Deezer clients:** Often missing a shared rate limiter across *all* call sites (poller + search-proxy) — verify by load-testing with a watchlist of 20+ artists, not 1-2
- [ ] **Diff engine:** Often missing edit-detection (only checks ID existence, not content hash) — verify by manually editing a tracked release's tracklist source data and confirming an alert fires
- [ ] **go:embed build:** Often ships whatever `dist/` happened to be on disk at Go-build time — verify by running a truly clean CI checkout (`git clean -fdx` locally) and confirming the frontend build step runs before `go build`
- [ ] **Notifier:** Often fire-and-forget with no retry/backoff — verify by intentionally triggering a burst of 10+ simultaneous detected changes and confirming none silently drop
- [ ] **CI pipeline ordering:** Often has scan/publish steps in whatever order they were added, not cost/risk order — verify by checking that a deliberately-introduced lint failure or leaked test secret fails fast, before any Docker build/push runs
- [ ] **Dockerfile non-root:** Often "works on my machine" locally (root) but permission-denied only in CI/prod — verify with `docker run --rm <image> whoami` confirming non-root, and an actual container start (not just `docker build` success)
- [ ] **sqlc/migrate sync:** Often has generated code that compiles but doesn't match the latest migration — verify with a CI step that runs `sqlc generate` fresh and diffs against committed output

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|-----------------|-------------------|
| Duplicate Discord notifications already sent to users | LOW | Add idempotency key + outbox table retroactively; going forward, dedupe is enforced; past duplicates are cosmetic only |
| Bloated Docker image already in ghcr.io | LOW | Rebuild final stage from distroless/minimal base, push a new tag; old bloated tags can be deleted from the registry |
| Tag-pinned GitHub Action later found compromised | MEDIUM | Rotate any secrets that action had access to, re-pin to a verified-clean SHA, audit recent workflow run logs for exfiltration indicators |
| sqlc-generated code drifted from actual schema (found late) | MEDIUM | Regenerate against current schema, diff against runtime queries, add integration tests that exercise every generated query against a real migrated DB before merging the fix |
| Migration files not zero-padded, ordering already ambiguous in production | HIGH | Cannot safely rename already-applied migration files (breaks `schema_migrations` history); must add new, correctly-padded migrations going forward and document the historical ordering gap |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| MusicBrainz rate-limit violations (P1) | Early — MusicBrainz client phase | Load test with 20+ watchlist entries, confirm zero 503s |
| robfig/cron overlap (P2) | Early/mid — scheduler phase | Inject an artificially slow poll cycle, confirm the next tick skips/delays instead of overlapping |
| Diff/notify race conditions (P3) | Mid — diff engine phase | Kill the process mid-cycle in a test, confirm no duplicate notification on restart; edit a tracked release's data, confirm an alert fires |
| go:embed stale build (P4) | Mid — UI integration phase | Clean CI checkout builds and serves current frontend; non-root-path refresh returns 200, not 404 |
| Discord rate limits (P5) | Mid — notifier phase | Burst-test with 10+ simultaneous notifications, confirm all eventually deliver with no ban |
| CI pipeline ordering/caching (P6) | Early/mid — first CI pipeline phase | Deliberately failing lint/test fails the run in under a minute; Trivy/gitleaks gate registry push |
| semantic-release Go misconfig (P7) | Mid — release/versioning phase | A merged conventional commit produces a real git tag/GitHub release with no `package.json`-related errors |
| Dockerfile non-root/bloat (P8) | Mid — Dockerfile phase | `docker images` shows a lean final size; `docker run --rm <image> whoami` is non-root |
| sqlc/migrate drift (P9) | Early — first migration + first sqlc query | CI step running migrate-then-generate-then-diff passes cleanly |

## Sources

- [MusicBrainz API / Rate Limiting](https://musicbrainz.org/doc/MusicBrainz_API/Rate_Limiting) — official docs
- [MusicBrainz API/Rate Limiting - Wiki](https://wiki.musicbrainz.org/MusicBrainz_API/Rate_Limiting) — official wiki
- [Deezer FAQs For Developers](https://support.deezer.com/hc/en-gb/articles/360011538897-Deezer-FAQs-For-Developers) — official support
- [Deezer API Rate limit issue discussion](https://github.com/BackInBash/DeezerSync/issues/6) — community, corroborating unofficial rate behavior
- [robfig/cron package docs](https://pkg.go.dev/github.com/robfig/cron) — official godoc
- [Build a Simple Dynamic Scheduler in Go with robfig/cron](https://medium.com/@gauravpaudel2013/build-a-simple-dynamic-scheduler-in-go-with-robfig-cron-87c42600e21b)
- [Idempotent Consumer - Handling Duplicate Messages](https://www.milanjovanovic.tech/blog/idempotent-consumer-handling-duplicate-messages)
- [Sending Reliable Event Notifications with Transactional Outbox Pattern](https://medium.com/event-driven-utopia/sending-reliable-event-notifications-with-transactional-outbox-pattern-7a7c69158d1b)
- [Developing and Compiling Webapps with Vite and Go](https://matteogassend.com/articles/go-webapp-vite)
- [Embed Vite React in Golang binary with live reload](https://dev.to/danhawkins/embed-vite-react-in-golang-binary-with-live-reload-1k4d)
- [Discord Webhook Rate Limits Explained (429, Retry-After, Best Practices)](https://discord-webhook.com/en/blog/discord-webhook-rate-limits/)
- [Discord Rate Limits — official docs](https://discord.com/developers/docs/topics/rate-limits)
- [Trivy Supply Chain Incident: GitHub Actions Compromise Breakdown](https://www.upwind.io/feed/trivy-supply-chain-incident-github-actions-compromise-breakdown)
- [Trivy ecosystem supply chain temporarily compromised — official advisory](https://github.com/aquasecurity/trivy/security/advisories/GHSA-69fq-xp46-6x23)
- [The Trivy Attack: Why SHA Pinning Fails GitHub Actions (nuance/counterpoint on SHA pinning)](https://dev.to/ameer-pk/the-trivy-attack-why-sha-pinning-fails-github-actions-14if)
- [Securely using pull_request_target — GitHub official docs](https://docs.github.com/en/actions/reference/security/securely-using-pull_request_target)
- [Keeping your GitHub Actions and workflows secure Part 1: Preventing pwn requests — GitHub Security Lab](https://securitylab.github.com/resources/github-actions-preventing-pwn-requests/)
- [semantic-release Configuration docs](https://semantic-release.org/usage/configuration/) — official
- [semantic-release configuration file .releaserc precedence issue](https://github.com/semantic-release/semantic-release/issues/729)
- [How to Configure Trivy Severity Filtering](https://oneuptime.com/blog/post/2026-01-28-trivy-severity-filtering/view)
- [Vulnerability — Trivy official docs](https://trivy.dev/docs/v0.52/scanner/vulnerability/)
- [distroless/static:nonroot permission denied issue](https://github.com/GoogleContainerTools/distroless/issues/718)
- [How to add a directory where non-root user can write — distroless issue](https://github.com/GoogleContainerTools/distroless/issues/427)
- [Modifying the database schema — sqlc official docs](https://docs.sqlc.dev/en/latest/howto/ddl.html)
- [Database migrations in Go with golang-migrate — Better Stack](https://betterstack.com/community/guides/scaling-go/golang-migrate/)
- [Syntax error on migration — golang-migrate issue](https://github.com/golang-migrate/migrate/issues/573)
- [Cache management with GitHub Actions — Docker official docs](https://docs.docker.com/build/ci/github-actions/cache/)
- [How to Cache Docker Images in GitHub Actions](https://www.dash0.com/faq/cache-docker-images-github-actions)

---
*Pitfalls research for: Go release-tracking service with CI/CD-focused portfolio goal (drop-tracker)*
*Researched: 2026-08-04*
