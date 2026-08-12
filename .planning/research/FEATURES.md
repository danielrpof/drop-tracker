# Feature Research

**Domain:** Software-engineering-practice hardening for an existing Go+React CI/CD portfolio project (drop-tracker v1.1) — NOT product/domain features. Scope: frontend test suite, CI coverage gating, events retention, bounded concurrent polling.
**Researched:** 2026-08-12
**Confidence:** MEDIUM (web-search-derived, cross-checked across 3+ independent sources per topic; no single authoritative "one true answer" exists for any of these four — they are engineering judgment calls with well-established typical ranges, not hard specs)

This file reinterprets the standard Feature Research categories for an engineering-practice milestone:

- **Table Stakes** = what a well-run Go+React project is expected to have for each capability — the credible minimum a reviewer/interviewer would look for.
- **Differentiators** = polish beyond the minimum that's genuinely worth the (small) extra effort for a portfolio piece, without materially growing scope.
- **Anti-Features** = practices that are legitimate at larger scale/org size but would be overkill here — flagged so the roadmap doesn't over-build.

All four capabilities are **modifications to existing, already-built systems**, not new subsystems — dependencies on the current codebase are called out explicitly per section, since that's what makes some "typical" practices inappropriate here and others mandatory.

## Feature Landscape

### Table Stakes (Expected Minimum Practice)

| Capability | What's Expected | Complexity | Notes |
|---|---|---|---|
| Frontend test suite: presentational + interactive component coverage | Test the app's actual interaction surface: `routes/watchlist.tsx` (list render, empty/loading/error states — `EmptyState` already exists as a component to exercise), `WatchlistRow` (per-row actions), `PreferenceToggles` (toggle → correct API call/handler invoked), `SearchBox` + `SearchResultsColumns` (debounce/typing → results render, no-results state), `history.tsx` + `EventCard` + `HistoryFilters` (filter interaction → list updates) | MEDIUM | ~8 components/routes already exist under `web/app/components` and `web/app/routes` (confirmed via repo scan) — this is a fixed, enumerable surface, not open-ended. RTL's guidance (query by role/label, assert on rendered output and callback calls, not internal state) is the accepted default. |
| Test against the app's existing API abstraction, not raw `fetch`/network | `web/app/lib/api.ts` already centralizes all backend calls — table stakes here is `vi.mock('~/lib/api')` (or equivalent module mock) per test, asserting the component calls the right api.ts function with the right args and renders correctly for success/error/loading responses | LOW | Because the codebase already isolated API calls behind one module (a good existing pattern), tests don't need to intercept HTTP at the network layer to be "properly tested" — mocking the module boundary is the natural, minimal-effort match for this architecture. |
| Vitest + React Testing Library as the toolchain | Exactly what PROJECT.md already locked | LOW | Frontend has zero test tooling installed today (`web/package.json` has no `vitest`/`@testing-library/*` — confirmed) — this is greenfield setup: `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `@testing-library/user-event`, `jsdom` (or `happy-dom`) as the environment. React 19 + Vite 8 + React Router 7 (framework/SPA mode) are all current-generation and Vitest-compatible. |
| CI coverage: fail-the-build, not warn-only | Non-zero exit on threshold miss is the norm — "warn and let it merge anyway" defeats the purpose of a gate | LOW | Matches PROJECT.md's already-decided approach. Backend: `go test -coverprofile=coverage.out ./...` + a threshold check (script or `go-test-coverage` action) wired into the existing `full-pipeline.yml`. Frontend: `vitest run --coverage` with `coverage.thresholds` in `vite.config.ts` — Vitest's v8 coverage provider fails the process natively on a miss, no extra tooling required. |
| CI coverage: single whole-repo aggregate threshold | One number per side (backend, frontend), not per-package/per-file tiers | LOW | Per-package thresholds are an org-scale pattern (see Anti-Features) — a single-module Go backend and single React app both get one aggregate number each. 70% (PROJECT.md's chosen value) sits squarely inside the commonly-cited 60–85% range across both ecosystems' tooling defaults and community guidance — validated as reasonable, not a wildcard choice. |
| Events retention: scheduled deletion job, reusing existing scheduler | A cron-driven job that deletes `events` rows older than the retention window | LOW-MEDIUM | The project already runs `robfig/cron` in-process for polling (`internal/poller`) — adding a third cron entry (or a lightweight ticker) that calls a new sqlc query (`DELETE FROM events WHERE created_at < now() - interval '90 days'`) is the natural fit. No new infrastructure (e.g. Postgres `pg_cron` extension) is needed or typical for a project already embedding a Go-side scheduler. |
| Events retention: hard delete for a non-compliance operational table | Straight `DELETE`, not soft-delete-then-purge | LOW | Soft-delete (`deleted_at`/`is_deleted` + a later purge) is the norm when recovery/undo or an audit/compliance trail is a real requirement. `events` here is operational/display data (release history) with no such requirement — hard delete is both the simpler and the typical choice for this table's actual purpose. Matches PROJECT.md's already-decided approach. |
| Events retention: 90-day window is a reasonable, industry-typical default | Not something that needs re-litigating | LOW | 90 days is the *default* audit/event retention period across multiple major platforms independently (Oracle Cloud audit logs, Microsoft 365 audit logs, ManageEngine EventLog Analyzer) and satisfies PCI-DSS's "3 months immediately available" floor. Cross-checked across 3+ independent sources — MEDIUM confidence this is a safe, unremarkable choice for a personal-scale app. |
| Bounded concurrent polling: fan-out-and-wait per poll cycle, not a persistent worker pool | Replace the current sequential `for _, entry := range entries { ... }` loop in `RunMusicBrainzCycle`/`RunDeezerCycle` (`internal/poller/poller.go`) with a bounded-concurrency fan-out over that same per-cycle artist list | LOW-MEDIUM | This is one poll cycle processing a bounded, known-size list (the watchlist), not a continuous stream — `golang.org/x/sync/errgroup` with `g.SetLimit(n)` is the idiomatic, minimal-dependency match (a fixed-goroutine-plus-jobs-channel "worker pool" library is built for long-lived streaming consumers, a different shape of problem). `golang.org/x/sync` is **already an indirect dependency** in `go.mod` — promoting it to direct via `errgroup` adds zero new external dependencies. |
| Bounded concurrent polling: per-source concurrency limit, not one shared/global limit | MusicBrainz's pool and Deezer's pool are sized and limited independently | LOW | The codebase already treats the two sources as fully independent at the cron/overlap-guard level (`mbRunning`/`dzRunning` as separate `atomic.Bool`s, explicitly "never one shared mutex... a shared guard would reintroduce exactly the cross-source coupling D-08 rejects"). A shared global concurrency cap across both sources would reintroduce that exact coupling one level up — the existing per-source separation principle extends directly to worker-pool/errgroup sizing. |
| Bounded concurrent polling: concurrency cap composes with, doesn't replace, the existing per-source rate limiters | `golang.org/x/time/rate.Limiter` (already in place per source, per STACK.md) keeps pacing requests; the new concurrency cap (env-configurable, default 3-5 per PROJECT.md) bounds how many goroutines/DB connections/in-flight HTTP requests exist at once | LOW | These are complementary controls, not redundant ones: the limiter paces *rate*, the cap bounds *concurrency* (memory, connection-pool pressure, file descriptors). Removing either changes behavior; keeping both is the documented norm. |

### Differentiators (Worth the Small Extra Effort)

| Capability | Value Proposition | Complexity | Notes |
|---|---|---|---|
| MSW (Mock Service Worker) for a handful of integration-style tests | Realistic network-boundary testing (request/response shape, not just "was the mock called") is a widely-recognized RTL-ecosystem best practice and a nice CI/DevOps-practice demo point | LOW-MEDIUM | Not needed for every test — reserve for 1-2 higher-value flows (e.g. the search-to-add flow end-to-end at the component boundary) while the bulk of tests use the simpler `vi.mock('~/lib/api')` approach (table stakes). Adding MSW wholesale for every test would be disproportionate setup for this app's size. |
| PR coverage-diff/report comments (`davelosert/vitest-coverage-report-action` for FE; an equivalent Go coverage-comment action for BE) | Visible, reviewable coverage feedback directly on PRs — a genuine CI/CD-maturity showcase point, consistent with the project's existing "Full Pipeline" polish (SBOM, Trivy, gitleaks already produce visible CI artifacts/gates) | LOW | Additive to the coverage gate, not a replacement — the gate (fail-the-build) is table stakes; the report/comment is presentation polish on top of it. |
| Structured log line on each retention run (rows deleted, duration) | Cheap observability win, consistent with the project's existing `log/slog` structured-logging discipline (every other subsystem already logs cycle results this way) | LOW | One `logger.Info("events retention run", slog.Int64("rows_deleted", n), ...)` call — trivial to add, meaningfully improves operability for zero real cost. |
| Idempotent, safe-to-rerun retention job | Matches the project's existing idempotency ethos (the `events` table's own dedup key, `ON CONFLICT DO NOTHING` pattern) | LOW | A `DELETE ... WHERE created_at < cutoff` is naturally idempotent (re-running it after a crash mid-run just deletes fewer or zero additional rows) — worth stating explicitly as a design property, not extra code. |
| Per-cycle fan-out timing/goroutine-count in poll-cycle logs | Cheap observability differentiator, consistent with the project's per-cycle structured logging (`cycle_id`, `item_count`, etc. already logged today) | LOW | e.g. log total cycle duration and configured pool size alongside the existing per-cycle summary log line — demonstrates the concurrency change actually changed cycle throughput, useful evidence for a portfolio writeup. |
| Documenting the concurrency-sizing heuristic in code comments / README | The codebase already has a strong convention of explaining *why* (see extensive doc comments in `poller.go`, `detector.go`) — stating "pool size is bounded by the external API's rate limit, not CPU count" continues that pattern | LOW | Zero implementation cost, meaningfully improves the code's self-documentation for a reviewer/interviewer reading it later. |

### Anti-Features (Typical at Larger Scale, Overkill Here)

| Practice | Why It's Commonly Seen | Why It's Overkill for This Project | Alternative |
|---|---|---|---|
| Full E2E test suite (Playwright/Cypress) | Standard at larger orgs; catches integration issues unit/component tests miss | PROJECT.md's Active requirement is explicitly "Vitest + RTL unit/component test suite," not E2E — adding a second, heavier test framework and browser-automation CI job is real added CI runtime/complexity for a milestone scoped as a coverage gap, not a new testing tier | Component tests at the RTL level cover the enumerated surface; defer E2E to a future milestone if ever needed |
| Chasing 100% coverage | Feels rigorous | Every source consulted explicitly warns against this — coverage percentage is gameable (trivial assertion-free tests inflate it) and 100% typically means testing getters/boilerplate, not real behavior | Target the 70% aggregate threshold already chosen; prioritize meaningful assertions on the enumerated component surface over chasing the last few percent |
| Per-package/per-file coverage threshold tiers | Common in large, multi-service orgs with clearly distinct high-risk vs low-risk modules | This is a single small Go module and a single small React app — there's no clearly distinct "critical vs low-risk" module split yet that would justify differentiated thresholds; added CI-config complexity with no corresponding benefit at this size | One aggregate threshold per side (backend, frontend) |
| Diff/patch coverage gating (new/changed lines must hit a higher bar than the aggregate) | A genuinely good practice at scale — catches regressions on new code even when legacy code drags the aggregate down | Requires base-branch diff tooling and more CI-config surface than a whole-repo aggregate threshold; disproportionate for a project with no large legacy low-coverage codebase to work around | A single aggregate threshold is sufficient — there's no legacy coverage debt this project needs to route around |
| Mutation testing (`go-mutesting`, Stryker for the FE side) | Genuinely more rigorous than line coverage at large scale | Meaningfully increases CI runtime and tooling surface for marginal signal at this project's size; not what "properly tested" means for a portfolio-scale coverage milestone | Line/branch/function coverage thresholds are the accepted level of rigor here |
| Table partitioning (e.g. monthly range partitions) for retention via cheap partition-drop | The textbook "right" pattern once an events/log table reaches large volume — dropping a partition is a metadata-only operation vs row-by-row `DELETE` | `events` here is a personal watchlist's release history — realistic row volume is small (a handful of artists × a few releases/month); partitioning adds migration complexity (partition-aware schema, new-partition-creation job) with no measurable performance benefit at this scale | A single unbatched (or lightly batched) `DELETE ... WHERE created_at < cutoff` run daily/weekly is more than sufficient |
| Soft-delete + separate purge job (two-phase retention) | Standard where an undo window or audit trail is a real requirement | No such requirement exists for this table (see Table Stakes) — soft-delete adds a `deleted_at` filter to every existing query plus a second scheduled job, pure added complexity with no compensating benefit here | Straight hard delete on the retention cutoff |
| `pg_cron` (Postgres extension) for retention scheduling | Common when the deletion logic should live DB-side, independent of the application process | Requires extension installation/superuser privileges, which many managed Postgres providers restrict or gate behind extra setup — the project already has an in-process Go scheduler (`robfig/cron`) doing exactly this job for polling; reusing it for retention needs zero new infrastructure | A third `robfig/cron` entry (or a simple ticker) in the existing Go process, calling a sqlc-generated delete query |
| A general-purpose worker-pool library (`gammazero/workerpool`, `pond`, etc.) with a persistent jobs channel | Fits long-lived, continuously-arriving-work scenarios (queue consumers, streaming ingestion) | Each poll cycle is a bounded, one-shot fan-out over a known-size list (the watchlist at that moment), then the cycle ends — this is exactly the shape `errgroup.SetLimit` (stdlib-adjacent, already an indirect dependency) is built for; a persistent pool/jobs-channel abstraction adds a new dependency and a lifecycle-management concern (start/stop the pool) for no benefit over the simpler per-cycle fan-out | `golang.org/x/sync/errgroup` with `SetLimit(n)`, one `errgroup.Group` created fresh per poll cycle |
| Dynamic/adaptive concurrency (auto-tune pool size from observed latency/429 responses) | Legitimate at scale, where traffic patterns and API behavior justify the engineering investment | A personal watchlist of a handful of artists against two well-documented, low-traffic external APIs doesn't produce the kind of variable load that adaptive tuning is built to handle; the effort is disproportionate to any realistic benefit | A single env-configurable static pool size (default 3-5, per PROJECT.md) is sufficient and simpler to reason about/test |
| Distributed rate limiting / cross-instance worker coordination (e.g. Redis-backed limiter) | Necessary once a service runs as multiple horizontally-scaled instances | PROJECT.md explicitly locks a single Go binary/service architecture with no horizontal scaling in scope — the existing in-process `rate.Limiter` and the new in-process concurrency cap are correct and sufficient for a single-instance deployment | In-process limiter + in-process concurrency cap (both already the plan) |

## Feature Dependencies

```
[Existing React components: WatchlistRow, PreferenceToggles, SearchBox,
 SearchResultsColumns, EventCard, HistoryFilters, EmptyState, CoverArt,
 routes/watchlist.tsx, routes/history.tsx]  (built Phase 06)
    └──requires (as test subjects)──> [Vitest + RTL toolchain setup]
    └──requires (as mock boundary)──> [web/app/lib/api.ts]  (already exists — the mockable seam)

[Frontend test suite]
    └──enables──> [Frontend CI coverage gate]  (no coverage number to gate on without tests existing first)

[Existing Go unit tests (httptest.Server-mocked, built Phases 01-07)]
    └──enables──> [Backend CI coverage gate]  (coverage measurement wires into tests that already exist)

[Frontend CI coverage gate] + [Backend CI coverage gate]
    └──requires──> [Existing "Full Pipeline" GitHub Actions workflow]  (Phase 07 — new gate steps slot into it, not a new workflow)

[events table]  (built Phase 04 — doubles as the seen/dedup store AND the
                 frontend release-history data source, per
                 internal/db/migrations/000003_events.up.sql)
    └──requires (for retention)──> [Existing robfig/cron scheduler]  (internal/poller — reused, not duplicated)
    └──requires (for retention)──> [New sqlc delete query]

[Bounded concurrent polling]
    └──requires──> [Existing sequential per-artist loop]  (internal/poller/poller.go — RunMusicBrainzCycle/RunDeezerCycle, the code being refactored)
    └──requires──> [Existing per-source rate.Limiter]  (internal/musicbrainz, internal/deezer — must remain intact, not replaced)
    └──requires──> [Existing per-source overlap guard]  (mbRunning/dzRunning atomic.Bool — concurrency happens *within* one cycle, guard still prevents overlapping *cycles*)
    └──enables──> [Faster poll cycles, same rate-limit compliance]
```

### Dependency Notes

- **Frontend tests must exist before the frontend coverage gate is meaningful** — there is currently zero frontend test tooling in `web/package.json`, so this is greenfield setup, not incremental addition. Sequence: install Vitest/RTL → write tests for the enumerated component surface → wire `coverage.thresholds` → gate in CI. Skipping straight to a coverage gate with no tests would either fail immediately (correct) or require a 0%-permissive threshold that defeats the purpose.
- **Backend coverage gate has no equivalent bootstrap problem** — Phase 01-07 already built a substantial `httptest.Server`-mocked test suite; this capability is "measure and gate what already exists," not "write tests from scratch."
- **Events retention has a real interaction with the events table's dual role as seen-store, not just history log** — `internal/db/migrations/000003_events.up.sql` and `internal/detection/detector.go` (`isSeedMode`, `HasAnyEvent`, `groupBaseline`) show the `events` table is simultaneously: (a) the frontend's release-history display data, (b) the dedup key (`UNIQUE (event_type, source, external_id)`, `ON CONFLICT DO NOTHING`) that makes idempotent detection work, and (c) the per-release-group `track_count` baseline used to detect deluxe/tracklist changes. **A naive hard-delete of rows older than 90 days can re-trigger detection for a still-catalogued release**: if a purged row's `external_id` is still returned by MusicBrainz/Deezer on a later poll (which it always will be — catalog data doesn't disappear) and that artist has *other*, newer event rows for the same source (so `isSeedMode` returns false), the diff engine has no record of having seen it before and will insert it as genuinely new — triggering a real Discord notification for a 90+-day-old release. This is a concrete, code-grounded edge case (not a generic "be careful with deletes" caveat) that the phase implementing retention needs to explicitly accept, mitigate (e.g. exclude a release-group's most-recent event per artist+source from the retention sweep, or scope retention to `notified_at`-populated rows only), or document as a known/waived limitation — the project already has precedent for documenting an accepted limitation this way (the MusicBrainz TLS/WSL2 issue in PROJECT.md's Context section). This finding belongs in this milestone's pitfalls/planning research, not just this file, but it directly affects what "table stakes" retention implementation actually requires beyond "just add a DELETE statement."
- **Bounded concurrent polling must preserve, not replace, two existing safety mechanisms**: the per-source `rate.Limiter` (paces request rate) and the per-source overlap guard (`mbRunning`/`dzRunning`, prevents two overlapping *cycles* of the same source). The new concurrency bound operates *within* a single cycle (how many of that cycle's artists are fetched at once), which is an orthogonal axis to both — none of the three should be removed or merged into one.
- **Per-source separation is a pattern already established at the cron/overlap-guard layer** (two independent cron entries, two independent atomic guards, explicitly documented as intentional to prevent MusicBrainz's slower pace from blocking Deezer) — extending that same per-source independence to concurrency-pool sizing is following the codebase's existing convention, not introducing a new one.

## MVP Definition

### Launch With (this milestone, v1.1)

Exactly PROJECT.md's Active requirements — no more, no less.

- [ ] Vitest + RTL test suite covering the enumerated component/route surface (`WatchlistRow`, `PreferenceToggles`, `SearchBox`, `SearchResultsColumns`, `EventCard`, `HistoryFilters`, `routes/watchlist.tsx`, `routes/history.tsx`), mocking through `web/app/lib/api.ts`
- [ ] CI coverage gates (70% threshold, fail-the-build) for both backend (`go test -coverprofile` + threshold check) and frontend (`vitest run --coverage` + `coverage.thresholds`), wired into the existing "Full Pipeline" workflow
- [ ] Events table retention: scheduled hard-delete of rows older than 90 days, via a new `robfig/cron`-scheduled job reusing the existing in-process scheduler — **with the seen-store/dedup interaction above explicitly considered during planning, not silently ignored**
- [ ] Bounded worker-pool concurrent per-artist polling in `RunMusicBrainzCycle`/`RunDeezerCycle`, via `golang.org/x/sync/errgroup` + `SetLimit(n)`, env-configurable pool size (default 3-5), per-source (not global), still respecting the existing per-source `rate.Limiter`

### Add After Validation (not this milestone, only if the pattern proves valuable)

- [ ] PR coverage-diff/report comment action (FE and/or BE) — trigger: once the coverage gate itself is proven stable and not flaky, this is pure presentation polish worth adding opportunistically
- [ ] Retention-run structured logging (rows deleted, duration) — trigger: near-zero cost, reasonable to fold directly into the retention job's initial implementation rather than defer, but listed here in case it's cut for time

### Future Consideration (explicitly out of scope for this milestone)

- [ ] E2E test suite (Playwright/Cypress) — defer: different testing tier than what this milestone scopes ("unit/component"); revisit only if a future milestone specifically calls for integration/E2E coverage
- [ ] Table partitioning for events retention — defer: no realistic data volume at this project's scale justifies it; revisit only if the events table ever grows by orders of magnitude
- [ ] Adaptive/dynamic concurrency tuning — defer: static env-configured pool size is correct for this project's traffic profile; revisit only if polling ever needs to scale to many more watched artists

## Feature Prioritization Matrix

| Capability | Value (to milestone goal) | Implementation Cost | Priority |
|---|---|---|---|
| Frontend test suite (Vitest + RTL, enumerated surface) | HIGH | MEDIUM | P1 |
| Backend + frontend CI coverage gates (70%, fail-the-build) | HIGH | LOW-MEDIUM | P1 |
| Events retention (90-day hard delete) | HIGH | LOW-MEDIUM (given the seen-store interaction to design around) | P1 |
| Bounded concurrent polling (errgroup + SetLimit, per-source) | HIGH | LOW-MEDIUM | P1 |
| MSW for 1-2 higher-value integration-style FE tests | LOW-MEDIUM | LOW | P2 |
| PR coverage-diff/report comment action | LOW-MEDIUM | LOW | P2 |
| Retention-run structured logging | LOW (but nearly free) | LOW | P2 |
| Per-cycle concurrency/timing metrics in poll logs | LOW (but nearly free) | LOW | P2 |
| E2E test suite | LOW (out of this milestone's scope) | HIGH | Anti-feature (this milestone) |
| Table partitioning for retention | LOW (no scale justification) | MEDIUM-HIGH | Anti-feature |
| Diff/patch coverage gating | LOW (no legacy-debt problem to solve) | MEDIUM | Anti-feature |
| Mutation testing | LOW (disproportionate rigor for scale) | HIGH | Anti-feature |
| `pg_cron` extension for retention | LOW (existing in-process scheduler already covers it) | MEDIUM (infra dependency) | Anti-feature |
| Adaptive/dynamic concurrency tuning | LOW (no traffic profile that needs it) | HIGH | Anti-feature |

**Priority key:**
- P1: Matches PROJECT.md's four Active requirements for this milestone exactly
- P2: Cheap, additive polish worth doing if time allows, not required for milestone completion
- Anti-feature: Legitimate practice at larger scale, disproportionate for this project's size/goals

## Sources

- [Unit Testing a React Application with Vitest — Medium](https://medium.com/@mtandimartin/unit-testing-a-react-application-with-vitest-5d63f7d75507)
- [Component Testing — Vitest official guide](https://vitest.dev/guide/browser/component-testing)
- [How to Unit Test React Components with Vitest and React Testing Library](https://oneuptime.com/blog/post/2026-01-15-unit-test-react-vitest-testing-library/view)
- [How to set up a Test Coverage threshold in Go and Github — Medium/Synechron](https://medium.com/synechron/how-to-set-up-a-test-coverage-threshold-in-go-and-github-167f69b940dc)
- [go-test-coverage (GitHub Action)](https://github.com/marketplace/actions/go-test-coverage)
- [go-coverage-threshold (GitHub)](https://github.com/jokeyrhyme/go-coverage-threshold)
- [Vitest Coverage Thresholds: Fail CI on Low Coverage — Nerd Level Tech](https://nerdleveltech.com/vitest-coverage-thresholds-fail-ci-tutorial)
- [davelosert/vitest-coverage-report-action (GitHub)](https://github.com/davelosert/vitest-coverage-report-action)
- [ci(coverage): add coverage thresholds to frontend vitest configuration — microsoft/edge-ai#140](https://github.com/microsoft/edge-ai/issues/140)
- [Code Coverage: Benchmarks, Targets & Best Practices](https://www.em-tools.io/engineering-metrics/code-coverage)
- [How much code coverage is enough? — Graphite](https://graphite.com/guides/code-coverage-best-practices)
- [Time-based retention strategies in Postgres — Sequin blog](https://blog.sequinstream.com/time-based-retention-strategies-in-postgres/)
- [Automatic deletion of older records in Postgres — Nicola Iarocci](https://nicolaiarocci.com/automatic-deletion-of-older-records-in-postgres/)
- [PostgreSQL soft-delete strategies — DEV Community](https://dev.to/oddcoder/postgresql-soft-delete-strategies-balancing-data-retention-50lo)
- [pg_ttl_index — PGXN](https://pgxn.org/dist/pg_ttl_index/)
- [Manage audit log retention policies — Microsoft Learn](https://learn.microsoft.com/en-us/purview/audit-log-retention-policies)
- [Audit Log Retention Period — Oracle Cloud docs](https://docs.cloud.oracle.com/en-us/Content/Audit/Tasks/settingretentionperiod.htm)
- [Retention Settings — EventLog Analyzer](https://www.manageengine.com/products/eventlog/help/StandaloneManagedServer-UserGuide/AdminSettings/db-storage-settings.html)
- [Go Concurrency Control: Worker Pools vs Semaphores — Medium](https://phu09032000.medium.com/go-concurrency-control-worker-pools-vs-semaphores-069e90bc3a03)
- [Bounded Concurrency in Go: Worker Pools, Semaphores, errgroup, and the Pitfalls — levelup.gitconnected](https://levelup.gitconnected.com/bounded-concurrency-in-go-worker-pools-semaphores-errgroup-and-the-pitfalls-that-hurt-in-5192eff95e86)
- [Goroutine Pool Patterns in Go: errgroup & Backpressure](https://tanhdev.com/posts/golang-goroutine-pool-errgroup-worker/)
- [Goroutine Worker Pools — Go Optimization Guide](https://goperf.dev/01-common-patterns/worker-pool/)
- [Worker Pool — Go Patterns](https://go-patterns.dev/parallel-computing/worker-pool)
- Repo scan: `web/app/components/`, `web/app/routes/`, `web/package.json`, `internal/poller/poller.go`, `internal/detection/detector.go`, `internal/db/migrations/000003_events.up.sql`, `go.mod` (confirmed component surface, existing tooling gaps, existing scheduler/rate-limiter/dedup implementation directly from source — HIGH confidence, primary source)

---
*Feature research for: Hardening milestone (frontend tests, CI coverage gates, events retention, bounded concurrent polling) — drop-tracker v1.1*
*Researched: 2026-08-12*
