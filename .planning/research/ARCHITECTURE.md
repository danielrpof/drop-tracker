# Architecture Research: v1.1 Hardening & Scale Readiness

**Domain:** Integration architecture for 4 hardening features into an existing single-binary Go + React Router SPA release tracker
**Researched:** 2026-08-12
**Confidence:** HIGH (all 4 findings verified directly against the current repo source, not general framework docs alone)

This is **not** greenfield ecosystem research — it is integration analysis. Each section below maps a target feature onto the exact existing files/patterns it touches, calls out what's genuinely new vs. what's a small extension of an existing pattern, and flags the concurrency/data-integrity risks specific to this codebase (not generic risks).

## System Overview (current, pre-milestone)

```
┌──────────────────────────────────────────────────────────────────────┐
│                     cmd/server/main.go (single process)               │
├───────────────────────────┬───────────────────────────┬──────────────┤
│   internal/httpserver      │    internal/poller          │ (embedded)   │
│   chi router, /health      │    robfig/cron, 2 entries:  │ internal/    │
│   /watchlist, /search,     │    mbRunning/dzRunning      │ webassets    │
│   /events (SPA fallback)   │    atomic.Bool CAS guards   │ (React SPA   │
│                             │    sequential per-artist    │ build/client │
│                             │    loop, one source at a    │ via go:embed)│
│                             │    time                     │              │
├───────────────────────────┴───────────────┬───────────────┴──────────┤
│         internal/detection (Detector)      │   internal/notifier      │
│   DetectMusicBrainz / DetectDeezer         │   drains ListUnnotified  │
│   per-artist seed-mode + seen-set,         │   → Discord webhook      │
│   captured once per artist per call        │                          │
├─────────────────────────────────────────────────────────────────────┤
│              internal/db/sqlc over pgx/v5 pool (Postgres)              │
│   watchlist, artists, events (seen-store + baseline column)           │
└─────────────────────────────────────────────────────────────────────┘
                    ▲                                    ▲
                    │ rate.Limiter (shared, per-source)   │
        internal/musicbrainz/client.go        internal/deezer/client.go
```

```
                     web/ (Vite, React Router v7, ssr:false)
web/app/
├── root.tsx, routes.ts            # route config (not fs-based routing)
├── routes/{watchlist,history}.tsx # route components
├── components/{watchlist,history,common,ui}/
└── lib/{api.ts,utils.ts}
                 │  `pnpm run build` → web/build/client
                 ▼
     internal/webassets/build/client (committed, go:embed all:build/client)
```

No test runner, no coverage tooling, and no CI coverage gate exist yet on either side. `web/package.json` has zero `vitest`/`@testing-library/*` deps today.

## Feature 1 — Vitest + React Testing Library suite

### Where it lives

Co-locate `*.test.tsx` files next to the component/route they test, inside `web/app/`, mirroring the Go side's own `_test.go`-beside-source convention already used throughout `internal/` (e.g. `internal/poller/poller_test.go` next to `poller.go`). Do **not** introduce a parallel `web/__tests__/` or `web/tests/` tree — that would be the one place this project's test-file convention diverges between Go and TS for no reason.

```
web/app/
├── lib/
│   ├── api.ts
│   └── api.test.ts                       # new
├── components/watchlist/
│   ├── WatchlistRow.tsx
│   └── WatchlistRow.test.tsx             # new
├── components/history/
│   ├── EventCard.tsx
│   └── EventCard.test.tsx                # new
├── routes/
│   ├── watchlist.tsx
│   └── watchlist.test.tsx                # new
└── test/
    └── setup.ts                          # new — jest-dom matchers, cleanup
```

`web/app/test/setup.ts` is the one genuinely new top-level thing: it registers `@testing-library/jest-dom`'s matchers and calls `cleanup()` after each test (Vitest doesn't do this automatically the way `@testing-library/react`'s Jest integration historically did).

### Vite config changes — this is the one real gotcha (HIGH confidence, verified against Remix/RR docs directly)

**Do not add a `test:` block to the existing `web/vite.config.ts`.** `@react-router/dev/vite`'s `reactRouter()` plugin is explicitly documented as being for the dev server and production build only — it is not designed to coexist with Vitest (or Storybook), which also read `vite.config.ts` by default. Loading it under Vitest produces the well-known "React Router Vite plugin can't detect preamble" / virtual-module-resolution failures other RR7 projects hit.

Concretely:
1. Add a **new, separate `web/vitest.config.ts`** (not a merge into `vite.config.ts`) that only carries `resolve: { tsconfigPaths: true }` (needed for the `~/` import alias already used across `web/app/`) plus `@vitejs/plugin-react` (a new devDependency — `reactRouter()` bundles React fast-refresh but Vitest needs the plain React plugin for JSX transform, since `reactRouter()` itself is excluded) and the `test` block (`environment: "jsdom"`, `setupFiles: ["./app/test/setup.ts"]`, `css: true` — several existing components pull in `app.css`/Tailwind classes, and `css: true` avoids import-time failures on those).
2. New devDependencies: `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `jsdom`, `@vitejs/plugin-react`.
3. `web/package.json` gets a `"test": "vitest run"` script (and optionally `"test:watch": "vitest"`), matching the existing `build`/`dev`/`typecheck` script naming.
4. `routes.ts`'s config-based routing (not filesystem-based) means route components under test still import real loaders/actions from `~/lib/api.ts` — for route-level tests that need to avoid hitting the network, mock `~/lib/api.ts`'s exported functions with `vi.mock`, not a router-level fixture; there is no `createRoutesStub()` need here since routes.ts doesn't use loaders/actions today (a plain fetch-in-`useEffect`/handler pattern per `lib/api.ts`).

**New vs. modified:** everything under "Where it lives" is new. `web/vite.config.ts` is untouched. `web/package.json` gets new deps + one new script (modified, additive only).

## Feature 2 — CI coverage gating

### Where it slots into `full-pipeline.yml`

The existing job graph is: `vet`, `lint`, `test` (Go), `gitleaks`, `trivy-fs` run in parallel → `build-scan` (`needs: [vet, lint, test, gitleaks, trivy-fs]`) → `release`. Coverage gating is a **correctness gate**, same tier as `test`, not a security/supply-chain gate — it belongs alongside `test`, feeding into the same `build-scan` `needs:` array, not before or after it as a separate serial stage.

Two options, and the right one is **extend the existing `test` job, don't add new jobs**:

- **Backend:** `test` job's `make test-integration` step already runs `go test ./... -race -count=1 -p 1`. Add `-coverprofile=coverage.out -covermode=atomic` to that invocation (atomic mode is required, not `count` or `set`, because `-race` is already on and atomic is the only mode safe under the race detector's instrumentation). Follow with a `go tool cover -func=coverage.out` step that parses the `total:` line and fails (`exit 1`) below the 70% threshold. This is a **new step inside the existing `test` job**, not a new job — `test-integration` already requires the `db-up` Postgres service container, and duplicating that setup in a second job just to compute coverage is wasted CI minutes for zero benefit.
- **Frontend:** needs a **new job**, e.g. `test-web`, added to the top-level parallel tier (alongside `vet`/`lint`/`test`/`gitleaks`/`trivy-fs`) and added to `build-scan`'s `needs:` array. It runs `pnpm install --frozen-lockfile` (mirrors the `Makefile web:` target's flag) then `pnpm run test -- --coverage` with `vitest.config.ts`'s `coverage.thresholds` block set to 70 for lines/statements/functions/branches — Vitest's own threshold enforcement (`vitest run --coverage`, provider `v8`) exits non-zero on breach, so this CI step needs no separate coverage-parsing logic the way the Go side does; the gate is enforced by Vitest itself, the CI step just has to not swallow its exit code.

```
vet ─┐
lint ─┤
test (Go, now also gates 70% coverage) ─┤
test-web (NEW: pnpm test --coverage, gates 70%) ─┤→ build-scan → release
gitleaks ─┤
trivy-fs ─┘
```

**New vs. modified:**
- `test` job: modified (one new coverage-check step appended after the existing test run).
- `test-web` job: entirely new, added to the parallel tier and to `build-scan`'s `needs:`.
- No change to `build-scan` or `release` job bodies — only their implicit dependency surface widens by one job (`test-web`).

**Sequencing dependency on Feature 1:** `test-web`'s coverage step is meaningless (0% or a hard failure with no test files) until Feature 1's Vitest suite exists with real assertions. Land Feature 1 first, then Feature 2's frontend half; the backend half of Feature 2 has no such dependency and can land independently/first since `go test` already exists.

## Feature 3 — Events retention (90-day hard delete)

### Placement: a third robfig/cron entry inside the existing in-process scheduler — not pg_cron, not a separate process

`internal/poller.Poller` already owns exactly one `*cron.Cron` instance with two `AddFunc` entries (MusicBrainz, Deezer), both wired through `poller.New` and started/stopped by `cmd/server/main.go`'s existing `pollr.Start(ctx)` / deferred `pollr.Stop(drainCtx)` lifecycle. Retention is architecturally identical in shape to those two entries — a periodic, idempotent, single-purpose job that needs the same graceful-shutdown draining `Stop` already provides — so it should be:

- **A third `cron.AddFunc` entry**, registered either directly in `poller.New` (simplest: same file, same `cron` instance, same lifecycle) or in a small new `internal/retention` package with its own `Register(cron *cron.Cron, ...)` exposed to `poller.New`, mirroring how `detection.EventRecorder` and `notifier.Notifier` are already narrow seams the poller depends on rather than owns. **Prefer the new-package option** — retention has nothing to do with polling an external API, and folding it into `poller` blurs a package whose doc comment already scopes it tightly to "runs the scheduled polling cycles." A new `internal/retention` package (own `Service`, own `DeleteEventsOlderThan` sqlc query call, own logger correlation id like `cycleID` does) keeps the same architectural shape (own CAS guard is unnecessary here — an `execrows`-affecting `DELETE ... WHERE created_at < now() - interval` is naturally idempotent/re-entrant even if two ticks somehow overlapped, unlike the poll cycles which must not overlap because they hold a live rate-limited HTTP conversation) while staying a separate concern.
- Interval: a new `RETENTION_INTERVAL` config field (e.g. default `24h`, run once a day) is independent of `POLL_INTERVAL` — reuse the `@every <duration>` cron spec pattern `poller.New` already uses, and a new `RetentionDays` config field (default `90`) rather than hardcoding the threshold, following `config.go`'s existing "every knob is a `caarlos0/env` struct field with an `envDefault`" convention.
- **Rejected: pg_cron.** This project has no existing pg_cron/extension-management story (`db/migrations` are plain golang-migrate SQL, no `CREATE EXTENSION pg_cron`), it would require enabling and trusting a Postgres extension not available on every managed Postgres offering, and it moves scheduling authority outside the single-binary architecture PROJECT.md explicitly locks ("API, scheduler, notifier all in one process"). A cron entry inside the already-running `*cron.Cron` instance is strictly less new surface area than standing up a second scheduling mechanism.
- **Rejected: separate process/sidecar.** Directly contradicts the locked single-binary constraint for zero benefit — retention is a lightweight periodic `DELETE`, not a workload that needs isolation.

### The real risk: retention interacts destructively with the detection engine's baseline/seen-store logic — this is not a hypothetical, it's structural

Verified directly against `internal/detection/detector.go` and `musicbrainz.go`: the events table is simultaneously (1) the seen-store, (2) the deluxe-change baseline store, and (3) the seed-mode signal — a naive `DELETE FROM events WHERE created_at < now() - interval '90 days'` breaks all three:

1. **Seed-mode reset (`isSeedMode` / `HasAnyEvent`).** `HasAnyEvent(artist_id, source)` returning `false` means "seed mode" (D-14): every currently-fetched item for that artist+source is inserted silently with a shared `notified_at`, **not surfaced as a new Discord notification** (`seedNotifiedAt`). If retention deletes *every* event row for a long-tracked, low-release-frequency artist (all their events older than 90 days, none newer), the artist silently flips back into seed mode. The next poll cycle then re-inserts their entire back catalogue as "seen" with no notification — this specific failure mode is silent (no error, no alert) and only observable as "this artist's history disappeared from the UI and no one got notified about it."
2. **Seen-set collapse (`ListExternalIDs` / `seenExternalIDs`).** A deleted `new_release` row's dedup key (`event_type, source, external_id`) unique constraint no longer exists in the table — the next poll cycle's `InsertEvent` for that same release will succeed as "newly detected" (since `ON CONFLICT DO NOTHING` has nothing to conflict with), re-firing a Discord notification for a release that already fired one 91+ days ago. **This is the most user-visible bug retention could introduce if built naively: duplicate re-notification of old releases.**
3. **Baseline loss (`GroupTrackCountBaseline` / `SetGroupTrackCountBaseline`).** The deluxe-change baseline (`track_count`) lives on the group's own `new_release` event row (04-01's "option-a" decision, not a separate table). Deleting that row deletes the baseline. The next cycle's `detectDeluxeChanges` sees `hasBaseline = false` and silently re-establishes the baseline at whatever the *current* track count is (`baselineEstablishedCount++`, no event fired) — a real deluxe expansion that happened between the deletion and the next poll is swallowed instead of alerted.

**Prevention (architectural, to design into the retention job, not just note as a caveat):**
- The safest correct behavior given the current schema is to **exclude each artist+source's most recent `new_release` row per release-group from hard-delete**, or more simply: **retention should delete by `created_at` age only from a narrower, explicitly-eligible subset** — e.g. never delete a `new_release` row that is the *only* row (or the newest row) for its `(artist_id, source)` pair, and never delete a `new_release` row whose `track_count` is non-NULL (i.e., it's currently serving as an active deluxe baseline) unless a newer `new_release` row for a different release-group already exists for that artist+source (proving seed-mode can't be re-triggered).
- Simpler and more honest given time constraints: **retention should target `guest_feature` events only for the 90-day window in v1**, since those have no baseline/seed-mode load-bearing role beyond their own seen-set entry, or **retention should be presented to the user as a display-only concern (soft-delete / hide-from-`ListEvents`-after-90-days) rather than a hard `DELETE`**, keeping the row (and its dedup key, seed-mode signal, and baseline) intact while satisfying the "90-day retention" UX goal via a `WHERE created_at > now() - interval '90 days'` filter on `ListEvents` instead of row removal. **This is the recommended approach** — it fully satisfies "events older than 90 days don't show in the UI" without touching detection correctness at all, and is a strictly smaller, safer change (`ListEvents` query filter, no new destructive cron job, no baseline-migration logic needed).
- If a genuine hard-delete is required (e.g. for storage/compliance reasons rather than UX), it must ship together with a **baseline-migration step**: before deleting a `new_release` row that currently holds a group's `track_count` baseline, that baseline value must be relocated (e.g. a dedicated `release_group_baselines` table, decoupled from any individual event row's lifecycle) — this is real new schema, not a cron-job-only change, and should be scoped and estimated separately from "add a delete job."

**New vs. modified:**
- New: `internal/retention` package (or a function inside `internal/poller`), a `RetentionDays`/`RetentionInterval` config fields, a new sqlc query (`DeleteEventsOlderThan` or, if the soft-delete/filter approach is taken instead, a `created_at` bound added to the existing `ListEvents` query).
- Modified (if hard-delete path chosen): `internal/db/sqlc/events.sql.go`-generating `.sql` query file, `cmd/server/main.go` wiring, `poller.New`'s cron registration (or the new package's own `Register`).
- **This feature has the highest design risk of the four** — the roadmap should treat "decide soft-filter vs. hard-delete-with-baseline-migration" as its own upfront decision, not an implementation detail discovered mid-build.

## Feature 4 — Bounded worker-pool concurrent polling

### What changes, precisely

`poller.RunMusicBrainzCycle` / `RunDeezerCycle` each do: `entries, _ := p.store.List(ctx)` once, then a `for _, entry := range entries` loop, sequentially calling the source client, then `p.events.DetectMusicBrainz(...)`, per artist. The worker-pool conversion replaces that `for` loop's body with N concurrent workers pulling from `entries`, bounded by a new `POLL_WORKER_POOL_SIZE` config field (default 3-5 per PROJECT.md).

**Recommended primitive: `golang.org/x/sync/errgroup` with `SetLimit(n)`**, not a hand-rolled channel+`sync.WaitGroup` pool. `errgroup.WithContext(ctx)` + `g.SetLimit(n)` + `g.Go(func() error {...})` per entry is the idiomatic bounded-concurrency pattern for "N independent items, cap concurrency at K" in current Go, and it's a single new dependency-free-of-new-dependencies choice since `golang.org/x/sync` is already an indirect dependency of this module graph (pulled in by `golang.org/x/time` and/or Go's own toolchain modules) or a trivial one-line `go get` if not.

**Critical correctness note specific to this codebase — errgroup's default error semantics must be neutralized:** `errgroup.Group`'s contract is: the *first* non-nil error returned by any `g.Go` closure cancels the shared context and is what `g.Wait()` returns. The current sequential loop's contract is the opposite — a per-artist fetch/detection error is **logged and the loop continues to the next artist** (`RunMusicBrainzCycle`'s existing comment: "one unreachable or misbehaving artist must not cost the rest of the cycle"). If each worker's closure is written to `return err` on a fetch/detection failure the way it's tempting to translate 1:1, converting to `errgroup` would silently change behavior: the *first* artist to fail cancels every other in-flight worker's context, turning "log and skip" into "abort the whole cycle." **Every worker closure must keep swallowing per-artist errors internally (log, then `return nil`) exactly as the sequential version does** — `errgroup`'s error-return path should be reserved solely for `ctx.Err()` (the existing `if err := ctx.Err(); err != nil { return err }` check, which today aborts the *whole* sequential cycle on shutdown and should still do so under the pool).

**Interaction with `rate.Limiter` (no change needed, by design).** `mbLimiter`/`dzLimiter` are already shared, package-level `*rate.Limiter` instances passed into `musicbrainz.Client`/`deezer.Client` and reused across search traffic and poll traffic (D-07, verified in `cmd/server/main.go`). `rate.Limiter.Wait(ctx)` (called internally by each client before every outbound request) is already goroutine-safe — a `rate.Limiter` is explicitly designed for concurrent callers contending on the same token bucket. Going from 1 to N concurrent callers against the *same* limiter instance does not change the enforced rate; it only changes how many goroutines are simultaneously *blocked waiting* for a token when the pool outpaces the configured rate. **No rate-limiter code changes are required** — this is the one piece of the existing architecture that was already built assuming eventual concurrent callers.

**Interaction with `mbRunning`/`dzRunning` CAS guards (no change needed, by design).** These guard *whole-cycle* overlap (a new tick arriving while the previous cycle, sequential or pooled, is still draining) — they say nothing about intra-cycle concurrency. The CAS-and-defer-release pattern around the entire `RunMusicBrainzCycle`/`RunDeezerCycle` body is orthogonal to what happens inside the loop; converting the loop body to a worker pool changes nothing about when `mbRunning`/`dzRunning` flip. One thing does change and must be re-verified: today, a single hung artist blocks the whole sequential cycle proportionally to 1 slow artist; under a pool of size N, up to N artists can be concurrently slow before the cycle-completion time is affected — this makes the cycle *faster to drain* on `Stop`'s `pollDrainTimeout`, not slower, so no timeout tuning is implied.

**Interaction with the detection engine's per-artist state capture — this is the real new risk, not the limiter or CAS guard.** Verified in `internal/detection/musicbrainz.go`: `isSeedMode` and `preCycleSeenGroups` are captured **once per artist, per `DetectMusicBrainz` call** (not once for the whole cycle across all artists) — so running `DetectMusicBrainz` concurrently for two *different* artists is safe with respect to seed-mode and the new_release seen-set, since both are scoped by `artist_id` (`HasAnyEvent(artist_id, source)`, `ListExternalIDs(artist_id, source, event_type)`). **The one place this scoping breaks down is `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline`, which are scoped only by `release_group_mbid` (verified in `querier.go` — no `artist_id` parameter on either query).** If two different watched artists are both credited on the same collaborative release-group (a real, if uncommon, case in this domain — a feature-heavy reggaeton/hip-hop collab album), and the worker pool processes both artists' `detectDeluxeChanges` concurrently, there is a genuine **read-then-write race on that shared baseline row**: both workers can read the same pre-update baseline, both compute "count increased," both call `SetGroupTrackCountBaseline` and `insertEvent` — the `events_dedup_key` unique constraint (`event_type, source, external_id`) prevents a literal duplicate row (the second `InsertEvent` no-ops via `ON CONFLICT DO NOTHING`), but the two `SetGroupTrackCountBaseline` calls can still interleave in a way that isn't strictly worse than sequential (last-write-wins on the same final value, since both computed the same `maxCount` from the same upstream data) — **this is a narrow, low-probability, self-correcting race** (not implicated in a real user-visible bug: worst case is one duplicate-suppressed insert attempt) — but it is worth a one-line mitigation: wrap `groupBaseline` + `setGroupBaseline`'s read-then-write in a single `UPDATE events SET track_count = GREATEST(track_count, $new) WHERE external_id = $group_mbid AND ... RETURNING track_count` style atomic compare-and-set at the SQL level instead of the current Go-level read-then-write, removing the race entirely rather than merely bounding its blast radius. This is a small, contained fix scoped to `internal/detection`, not the poller.

**New vs. modified:**
- Modified: `internal/poller/poller.go`'s `RunMusicBrainzCycle`/`RunDeezerCycle` loop bodies (sequential `for` → `errgroup`-bounded `g.Go` per entry); `internal/config/config.go` gets one new `PollWorkerPoolSize int` field (`envDefault:"4"`, mid-range of PROJECT.md's stated 3-5 default).
- New dependency: `golang.org/x/sync/errgroup` (likely already present transitively; confirm via `go.sum` before assuming a new direct dependency line is needed).
- Recommended companion fix (not strictly required for the milestone's stated scope, but the analysis above surfaces it as the one real correctness gap the conversion exposes): an atomic SQL-level `UPDATE ... RETURNING` for the baseline compare-and-set in `internal/detection/detector.go`, replacing the current `groupBaseline` read + `setGroupBaseline` write pair used inside `detectDeluxeChanges`.
- **Test impact:** `internal/poller/poller_test.go`'s existing fakes (`ReleaseGroupSource`/`AlbumSource`/`EventRecorder` stubs) must become concurrency-safe (guard any shared call-count/recorded-args state with a mutex) once the loop under test dispatches calls from multiple goroutines — this is a near-certain source of new flaky-test failures if the existing test doubles assume single-goroutine call ordering, and should be budgeted as part of this feature's own work, not discovered as CI flakiness after Feature 2's coverage gate is already in place.

## Build Order — dependencies across the 4 features

```
Feature 2 (backend half)     Feature 1 (Vitest+RTL suite)
   [independent, land           │
    anytime — go test           ▼
    already exists]        Feature 2 (frontend half)
                            [needs Feature 1's test files
                             to exist for coverage % to
                             mean anything]

Feature 3 (retention)        Feature 4 (worker pool)
[independent of 1/2/4;       [independent of 1/2/3, but
 land the soft-filter         highest concurrency-correctness
 design before any hard-      risk of the four — land LAST,
 delete implementation]       after Features 1-2's coverage
                               gates exist, so the pool-
                               conversion's own test changes
                               are caught by a working CI
                               coverage/test harness rather
                               than being the harness's own
                               first real workout]
```

**Recommended sequencing:**
1. **Feature 2, backend half** (trivial, zero dependencies — one flag on an existing `go test` invocation plus a threshold-check step).
2. **Feature 1** (Vitest + RTL suite) — must exist before Feature 2's frontend half is meaningful.
3. **Feature 2, frontend half** (depends on #2).
4. **Feature 3** (retention) — independent of 1/2/4, but its own internal decision (soft-filter vs. hard-delete-with-baseline-migration) should be resolved as a design checkpoint before implementation starts, given the correctness risk documented above.
5. **Feature 4** (worker pool) — land last: it's the highest-risk change (concurrency correctness in `internal/detection`'s baseline path), and having Features 1-2's coverage/test infrastructure already in place means its own test-double concurrency-safety fixes and any regression are caught by CI rather than discovered manually.

Features 1+2 and 3 have no interdependency and can run as parallel workstreams; Feature 4 should not start until at least the backend coverage gate (step 1) exists, since it is the change most likely to introduce a subtle regression the existing `-race` flag (already in `make test-integration`) is specifically positioned to catch.

## Sources

- Direct repo inspection (HIGH confidence — primary source, not third-party docs): `cmd/server/main.go`, `internal/poller/poller.go`, `internal/detection/detector.go`, `internal/detection/musicbrainz.go`, `internal/db/sqlc/querier.go`, `internal/db/migrations/000003_events.up.sql`, `internal/events/service.go`, `internal/config/config.go`, `Makefile`, `.github/workflows/full-pipeline.yml`, `web/package.json`, `web/vite.config.ts`
- `vitest.dev/config/coverage` — Vitest coverage provider/threshold configuration shape — confidence MEDIUM (web search corroborated, not directly fetched)
- Remix/React Router v7 community discussions (`remix-run/react-router` GitHub Discussions #12655, #13353; multiple independent 2026-dated setup guides) — "the React Router Vite plugin is not designed for use with Vitest/Storybook, use a separate config" — confidence MEDIUM-HIGH (corroborated across multiple independent sources, matches the exact symptom class documented for other RR7 projects)
- `pkg.go.dev/golang.org/x/sync/errgroup` — `SetLimit` bounded-concurrency semantics, first-error cancels shared context — confidence HIGH (official godoc)
- `pkg.go.dev/golang.org/x/time/rate` (referenced, not re-fetched this pass — already verified in-repo usage at `internal/musicbrainz/client.go`/`internal/deezer/client.go`) — `rate.Limiter` is safe for concurrent use by multiple goroutines — confidence HIGH (well-established stdlib-adjacent package behavior, consistent with the package's documented design intent)

---
*Architecture research for: drop-tracker v1.1 Hardening & Scale Readiness milestone*
*Researched: 2026-08-12*
