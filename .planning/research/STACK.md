# Technology Stack — v1.1 Hardening & Scale Readiness

**Project:** drop-tracker
**Milestone:** v1.1 (frontend tests, CI coverage gates, events retention, concurrent polling)
**Researched:** 2026-08-12

This is an **additions-only** stack doc. It does not repeat v1.0's validated stack (chi, sqlc/pgx, golang-migrate, robfig/cron, React Router v7/Vite, the full-pipeline.yml CI). Every recommendation below integrates with that existing stack rather than replacing any part of it.

## Recommended Stack

### 1. Frontend test suite (Vitest + React Testing Library)

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `vitest` | `^4.1.10` | Test runner | Vite-native (`web/vite.config.ts` is already on `vite: ^8`), and Vitest 4.1 is the first line with a `vite: ^8.0.0` peerDependency — anything earlier (Vitest ≤4.0) does not declare Vite 8 support and will emit a peer-dependency warning or fail resolution under pnpm's default strict peer behavior. Zero-config TS/JSX support matches the existing Vite build pipeline exactly (same transform, same `resolve.tsconfigPaths` behavior), so tests exercise the same module graph as `pnpm run build`. |
| `@vitest/coverage-v8` | `^4.1.10` (pin to the exact same version as `vitest`) | V8-native coverage provider | Uses Node's built-in V8 coverage counters (no source instrumentation step), which is materially faster than the Istanbul alternative and is Vitest's documented default provider. Must track the installed `vitest` version exactly — Vitest resolves this as a versioned companion package, not a loose semver range. |
| `@testing-library/react` | `^16.3.2` | Component rendering + queries | v16.3.1+ is the first line with a real (non-workaround) React 19 peer range — this project is already on `react: ^19.2.6`. Do not pin an older v16.x or any v14/v15 — those declare React 18 peers and will pnpm-resolve-error against React 19. |
| `@testing-library/dom` | `^10.4.1` | Underlying query engine | RTL v16 stopped bundling this transitively for some install topologies — install it explicitly as a direct devDependency so pnpm's strict resolution doesn't leave it as an unmet peer. |
| `@testing-library/jest-dom` | `^7.0.1` | DOM assertion matchers (`toBeInTheDocument`, etc.) | Import path for Vitest is `@testing-library/jest-dom/vitest` (not `/jest-globals`, which is the Jest-only entry point) — wire this into a `vitest.setup.ts` referenced from `test.setupFiles`. |
| `@testing-library/user-event` | `^14.6.1` | Realistic user interaction simulation (click/type/tab) | Standard companion to RTL; v14's async-by-default API (`await userEvent.click(...)`) is the current idiom — do not reach for the legacy v13 sync API in new tests. |
| `jsdom` | `^30.0.1` | DOM environment for Vitest | Recommended over `happy-dom` for this project: jsdom has stronger WHATWG spec fidelity (co-maintained by a WHATWG spec editor), which matters more for a correctness-focused portfolio test suite than happy-dom's ~5-10x raw speed edge on a component tree this small. `happy-dom` is a legitimate alternative if the suite later grows large enough that test-runtime becomes a CI bottleneck — RTL works unchanged under either. |

**Vite/Vitest config — do not extend the existing `vite.config.ts`.** `web/vite.config.ts` registers `reactRouter()` from `@react-router/dev/vite`, and the React Router Vite plugin is explicitly documented as incompatible with Vitest's test-mode transform (it expects a dev-server/build lifecycle Vitest doesn't provide). Create a **separate `web/vitest.config.ts`** that only registers `tailwindcss()` (harmless for tests, keeps class-name behavior consistent) and sets `test: { environment: 'jsdom', setupFiles: ['./vitest.setup.ts'], coverage: {...} }` — no `reactRouter()` plugin. `resolve.tsconfigPaths: true` still applies (Vite 8 native feature, not a plugin) so path aliases resolve identically to the app build.

**Route/component testing pattern:** use `createRoutesStub` from `react-router` for components that consume router hooks (`useNavigate`, `useLoaderData`, etc.) via a stubbed loader/action, per React Router's own testing guidance. It is explicitly *not* designed for full framework-mode `Route` module testing — for this project's SPA-mode app (`ssr: false`), that's the correct scope: test presentational/watchlist components in isolation with `render()` + RTL queries, and use `createRoutesStub` only for the handful of components that need router context to mount at all. Do not attempt to spin up the whole route tree end-to-end in Vitest — that's better served by not adding an E2E tool in this milestone (out of scope; Playwright/Cypress would be a separate, larger addition).

### 2. CI coverage gating (~70%, Go + Vitest)

**Go: no new dependency — use stdlib `go test -cover` / `go tool cover`, wired into the existing `test` job.**

This is the correct call for this project specifically, not just "simplest wins": CLAUDE.md's own stack doc already establishes a hand-rolled-over-dependency pattern for exactly this class of problem (MusicBrainz/Deezer/Discord clients all hand-rolled rather than pulling in a wrapper library "for zero real benefit"). A coverage-threshold check is the same shape of problem — one arithmetic comparison against `go tool cover`'s own summary line — and doesn't justify a third-party binary (`vladopajic/go-test-coverage` et al. exist and are reasonable if this project later wants per-package or diff-only coverage gating, but that's more than "~70% overall" requires).

Concretely, extend `make test-integration` (already runs against the real Postgres service, `-race -count=1 -p 1`) to also emit a coverage profile, and add threshold enforcement as a distinct CI step so a coverage-percentage failure is visually distinguishable in the Actions UI from a test failure:

```makefile
test-integration: db-up
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./... -race -count=1 -p 1 -covermode=atomic -coverprofile=coverage.out
```

`-covermode=atomic` (not the default `set`) is required, not optional, once `-race` is also passed — `-race`'s instrumentation and the default `set`/`count` coverage counters are not safe to combine; `atomic` is the mode Go's own docs specify for this combination.

CI threshold step (new, after `test-integration`, same job or a following one that downloads the `coverage.out` artifact):

```yaml
- name: Enforce coverage threshold
  run: |
    set -e
    make test-integration
    PCT=$(go tool cover -func=coverage.out | tail -1 | awk '{print substr($3, 1, length($3)-1)}')
    echo "Total coverage: ${PCT}%"
    awk -v pct="$PCT" 'BEGIN { exit !(pct >= 70) }'
```

**Frontend: `@vitest/coverage-v8` with a built-in `coverage.thresholds` config — no separate CI script needed.** Vitest fails its own process (non-zero exit) when configured thresholds aren't met, so `vitest run --coverage` alone is the gate:

```ts
// web/vitest.config.ts
test: {
  coverage: {
    provider: 'v8',
    thresholds: { lines: 70, statements: 70, functions: 70, branches: 70 },
  },
}
```

Add a new `frontend-test` job to `.github/workflows/full-pipeline.yml`, parallel to the existing `vet`/`lint`/`test`/`gitleaks`/`trivy-fs` jobs, and add it to `build-scan`'s `needs:` array so a coverage regression blocks the image build exactly like a Go test failure does today. Use `pnpm` (already the project's package manager per the `Makefile`'s `web` target) with `--frozen-lockfile`, matching the existing frontend-build convention:

```yaml
frontend-test:
  runs-on: ubuntu-latest
  timeout-minutes: 10
  steps:
    - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
    - uses: pnpm/action-setup@v4
    - uses: actions/setup-node@v5
      with: { node-version: 22, cache: 'pnpm', cache-dependency-path: web/pnpm-lock.yaml }
    - run: cd web && pnpm install --frozen-lockfile
    - run: cd web && pnpm vitest run --coverage
```

(Pin the two new third-party actions — `pnpm/action-setup`, `actions/setup-node` — to a commit SHA at implementation time, matching this workflow's existing SHA-pinning convention for every other action.)

### 3. Events retention (90-day hard delete)

**Recommendation: a third `robfig/cron` job inside the existing `internal/poller` Go process, not `pg_cron` and not a migration script.**

| Option | Verdict | Why |
|--------|---------|-----|
| `pg_cron` extension | Rejected | `docker-compose.yml` runs stock `postgres:16` (not `citusdata/pg_cron`/a custom image). Adopting it means: (a) swapping the base image, (b) setting `shared_preload_libraries = pg_cron` in `postgresql.conf` — which requires a full Postgres **restart**, not a hot reload — and (c) running `CREATE EXTENSION pg_cron` as superuser. That's real infrastructure surface added for a single daily `DELETE`, in a project whose PROJECT.md explicitly defers Kubernetes/Terraform-grade infra and caps the near-term deploy target at a plain VPS SSH deploy. Also fights the "single Go binary owns all scheduling" architecture decision already validated in Phase 3 (D-08: two independent in-process cron entries). |
| Plain `DELETE` shipped as a `golang-migrate` migration file | Rejected | Migrations in this project are schema state, applied once and never re-run (`golang-migrate` tracks applied versions in `schema_migrations` and will never re-execute `000005_retention.up.sql` on a second deploy). A recurring data-maintenance job is not a migration — encoding it as one either does nothing after the first deploy, or requires an anti-pattern (a migration that reschedules itself, which golang-migrate has no primitive for). |
| New `robfig/cron` entry in `internal/poller` (or a small sibling package, e.g. `internal/retention`) | **Recommended** | Matches the exact pattern already validated for the MusicBrainz/Deezer poll cycles: an independent `cron.AddFunc("@daily", ...)` entry, its own CAS overlap guard (mirroring `mbRunning`/`dzRunning`, D-09's "skip, don't queue" rule), and a `sqlc`-generated query (`DeleteEventsOlderThan90Days` or parameterized `DeleteEventsBefore($1)`) executed through the existing `pgxpool`. No new dependency, no new infra, no new deploy step — the same binary and the same `robfig/cron.Cron` scheduling idiom the project already has two of. |

Implementation shape:
- New sqlc query in `internal/db/sqlc/queries/events.sql`: `DELETE FROM events WHERE created_at < $1;` (pass `time.Now().AddDate(0, 0, -90)` from Go — keeps the retention window a Go-side constant/env var rather than a hardcoded SQL literal, so it's the same "config over magic numbers" pattern as `POLL_INTERVAL`).
- Schedule at `@daily` (or reuse the existing `interval`-based `@every` idiom if a coarser cadence is preferred) — daily is more than sufficient for a 90-day window; there is no correctness reason to run this more often than once per day.
- No new env var is strictly required (90 days is a fixed product decision per PROJECT.md), but if it should be tunable, add `EventsRetentionDays int `env:"EVENTS_RETENTION_DAYS" envDefault:"90"`` to the existing `config.Config` struct, following the exact pattern already used for `PollInterval`/`MusicBrainzRateLimitPerSec`.
- `notified_at IS NULL` events younger than the poll cadence should never be at risk here — 90 days is far longer than any realistic notification-delivery delay, so no additional guard against deleting not-yet-notified rows is needed, but the query should be reviewed at implementation time to confirm it doesn't accidentally race the `events_unnotified_idx` partial index's use case.

### 4. Bounded concurrent polling (worker pool, 3-5 workers/source)

**Recommendation: `golang.org/x/sync/errgroup` with `Group.SetLimit(n)` — already an indirect dependency, promote to direct. No new module needed.**

`go.mod` already pulls in `golang.org/x/sync v0.21.0` as an indirect dependency (transitively, likely via `golang-migrate` or `pgx`). `errgroup.Group.SetLimit(n)` (added in `x/sync` v0.7.0, well below the already-pinned v0.21.0) is the idiomatic modern replacement for a hand-rolled buffered-channel semaphore + `sync.WaitGroup` — it gives bounded concurrency, first-error propagation, and context cancellation in one primitive, which is a better fit here than adding `golang.org/x/sync/semaphore` (lower-level, more boilerplate for the same result) or a third-party worker-pool library (unnecessary — this is exactly the kind of small, stdlib-adjacent problem the project's existing "hand-roll it" convention already favors).

Shape of the change in `internal/poller/poller.go`'s `RunMusicBrainzCycle`/`RunDeezerCycle` per-entry loops:

```go
g, gctx := errgroup.WithContext(ctx)
g.SetLimit(p.workerPoolSize) // env-configurable, default 4
for _, entry := range entries {
    g.Go(func() error {
        groups, err := p.mb.ReleaseGroupsByArtist(gctx, entry.MBID)
        // ...same per-artist error handling as today, logged not returned...
        return nil // never propagate a per-artist error to g — see below
    })
}
_ = g.Wait()
```

Critical integration points with the *existing* architecture — these are why this is a moderate change, not a one-line swap:

- **The per-source `rate.Limiter` is untouched and still the true throughput cap.** `musicbrainz.Client`/`deezer.Client` already own their own `*rate.Limiter` (injected at construction, confirmed in `internal/musicbrainz/client.go` and `internal/deezer/client.go`) and presumably call `limiter.Wait(ctx)` before each outbound request. Bounding the *goroutine* count with `SetLimit` is orthogonal to bounding *request rate* with the limiter — N=4 concurrent goroutines all calling into the same client just means up to 4 requests can be in-flight/queued-on-the-limiter at once instead of 1; the limiter still serializes actual dispatch to MusicBrainz's ~1 req/sec and Deezer's ~50-req/5s ceilings exactly as today. The worker pool's only real effect is overlapping goroutine scheduling + connection setup + response parsing for artists whose fetch is latency-bound, not request-rate-bound.
- **Never `return err` from inside `g.Go`'s closure for a per-artist failure.** The current sequential loop already treats a single artist's fetch/detection error as non-fatal (`logger.Error(...); continue`) — that behavior must be preserved. If a per-artist error is returned from the closure, `errgroup` cancels `gctx` and aborts every other in-flight goroutine in the batch, silently turning "one artist is unreachable" into "the whole cycle stops early" — a regression from current behavior, not a feature. Keep returning `nil` from the closure after logging, exactly as the sequential version does with `continue`.
- **`p.events.DetectMusicBrainz`/`DetectDeezer` must be safe under concurrent calls.** `pgxpool.Pool` (already the project's driver) is designed for concurrent use, and `slog.Logger` is documented safe for concurrent use — so the DB and logging seams need no new synchronization. Confirm at implementation time that `internal/detect` (the `EventRecorder` implementation) does no unguarded shared mutable state across calls (it shouldn't, given each call is scoped to one `entry`).
- **Keep `p.notifier.NotifyPending` called once, after the pool drains (`g.Wait()`), not from inside the pool.** This is unchanged from today's D-05 ordering — draining the notification outbox is a cycle-level step, not a per-artist one, and concurrent `NotifyPending` calls were never the intent.
- **Go 1.22+ per-iteration loop variables make the classic `for _, entry := range entries { go func() { use(entry) } }` capture bug a non-issue here** — `go.mod` already declares `go 1.26`, so each `entry` in the loop above is already a fresh binding per iteration; no `entry := entry` shadow is needed (call this out in code review so nobody "fixes" it with a needless shadow copy from outdated Go idiom).
- **Config:** add `PollerWorkerPoolSize int `env:"POLLER_WORKER_POOL_SIZE" envDefault:"4"`` to the existing `config.Config` struct (same file/pattern as `MusicBrainzRateLimitPerSec`), threaded into `poller.New(...)` as a new parameter. Default 4 sits in the requested 3-5 range; document in `.env.example` alongside the other poll-tuning vars.
- **D-08 (source independence) is unaffected.** The worker pool is scoped *within* one cycle's per-entry loop — `mbRunning`/`dzRunning`'s separate CAS guards and the two independent `cron.AddFunc` registrations are untouched, so MusicBrainz's pool and Deezer's pool remain fully independent of each other, exactly as today.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Frontend coverage provider | `@vitest/coverage-v8` | `@vitest/coverage-istanbul` | v8 is Vitest's documented default, avoids a source-instrumentation pass, and is faster; Istanbul is only preferable if a future need arises for coverage output formats V8 doesn't produce well (e.g. certain legacy CI dashboards) — not a concern here. |
| Frontend DOM environment | `jsdom` | `happy-dom` | happy-dom is faster (5-10x) and is Vitest's own recommended default for new projects, but jsdom's stronger spec fidelity is the better tradeoff for a small, correctness-focused suite where raw runtime isn't yet a bottleneck. Revisit if the suite grows large enough that CI wall-clock time on `frontend-test` becomes a real cost. |
| Go coverage gating | stdlib `go test -cover` + `go tool cover -func` + a shell threshold check | `vladopajic/go-test-coverage` (or similar) | A dedicated tool adds per-package thresholds, diff-only (changed-lines) coverage, and PR comment/badge generation — genuinely nice, but more than "~70% overall" requires, and it's a new third-party CLI dependency in a project whose established pattern (per CLAUDE.md) is to hand-roll rather than add a wrapper for a small, well-bounded problem. Reasonable to revisit if the team later wants diff-coverage enforcement specifically. |
| Events retention scheduling | `robfig/cron` entry inside the existing Go process | `pg_cron` Postgres extension | Requires swapping the `postgres:16` base image, editing `shared_preload_libraries` (restart-only setting), and running `CREATE EXTENSION` as superuser — real infra surface for a single daily `DELETE`, and inconsistent with the project's "single Go binary owns scheduling" architecture (already validated for the two poll cycles). |
| Events retention scheduling | `robfig/cron` entry | External OS-level `cron` + a small CLI/script | Would require either a second entrypoint in the Docker image or a host-level cron job outside the container — breaks the "single deployable image" constraint (PROJECT.md) for no benefit over an in-process job the app already has the scheduling library for. |
| Worker pool primitive | `errgroup.Group.SetLimit(n)` | Hand-rolled buffered-channel semaphore + `sync.WaitGroup` | Functionally equivalent, but `errgroup` (already an indirect dependency, so no new module) is fewer lines, gives context-cancellation propagation for free, and is the idiom the wider Go ecosystem has converged on since `SetLimit` landed — less code to review/maintain than reimplementing the same bounded-concurrency primitive by hand. |
| Worker pool primitive | `errgroup.Group.SetLimit(n)` | `golang.org/x/sync/semaphore.Weighted` | Lower-level (manual `Acquire`/`Release` bookkeeping, no built-in error aggregation) — `errgroup` sits on top of the same package and is the better fit unless weighted (non-uniform-cost) concurrency is needed, which per-artist polling is not. |

## Installation

```bash
# Frontend test suite (run from web/)
cd web
pnpm add -D vitest @vitest/coverage-v8 @testing-library/react @testing-library/dom \
  @testing-library/jest-dom @testing-library/user-event jsdom

# Backend: no new go.mod dependency for coverage gating (stdlib go test -cover).
# Backend: no new go.mod dependency for the worker pool -- promote the existing
# indirect entry to direct by importing it:
#   import "golang.org/x/sync/errgroup"
# then run:
go mod tidy
```

No new CI-tool installs beyond what's already pinned in `full-pipeline.yml` (`golangci-lint-action`, `trivy-action`, `gitleaks-action`, `sbom-action`) — the new `frontend-test` job only needs `pnpm/action-setup` and `actions/setup-node`, both to be added.

## Sources

- Vitest 4.1 release notes / `vitest.dev` — Vite 8 peer-dependency support (`vite: ^6.0.0 || ^7.0.0 || ^8.0.0`) — confidence MEDIUM (cross-checked across multiple independent search results, not a single primary-source fetch)
- `@testing-library/react` npm listing / GitHub releases — v16.3.1+ React 19 peer support — confidence MEDIUM
- React Router GitHub discussions (`remix-run/react-router` #13032, #12454) and community guides — `createRoutesStub` scope/limitations, React Router Vite plugin incompatibility with Vitest — confidence MEDIUM
- Vite 8.0 release notes — native `resolve.tsconfigPaths` (already in use in this repo's `vite.config.ts`, confirming the repo is genuinely on Vite 8) — confidence MEDIUM
- `go.dev/doc/build-cover` and community coverage-tooling roundups — `-covermode=atomic` requirement alongside `-race`, `go tool cover -func` output shape — confidence MEDIUM
- `@testing-library/jest-dom` npm/GitHub — Vitest-specific import path (`/vitest` entry point) — confidence MEDIUM
- jsdom vs happy-dom 2026 comparison sources (multiple independent) — confidence LOW-MEDIUM (no single canonical source; corroborated directionally across several community write-ups, treat the speed/fidelity tradeoff framing as directionally right rather than precisely benchmarked)
- Direct repo inspection (`go.mod`, `web/package.json`, `web/vite.config.ts`, `internal/poller/poller.go`, `internal/musicbrainz/client.go`, `internal/deezer/client.go`, `internal/config/config.go`, `internal/db/migrations/000003_events.up.sql`, `.github/workflows/full-pipeline.yml`, `Makefile`, `docker-compose.yml`) — confidence HIGH (ground truth, not web research)
- `golang.org/x/sync` module history — `errgroup.Group.SetLimit` availability well below the already-pinned `v0.21.0` — confidence MEDIUM (based on general knowledge of the `x/sync` API surface, not a freshly fetched changelog in this session)
