# Pitfalls Research

**Domain:** Hardening an already-shipped Go + React release tracker — retrofitting frontend tests, CI coverage gates, events-table retention, and concurrent polling onto working v1.0 code
**Researched:** 2026-08-12
**Confidence:** HIGH (poller/detector/events code read directly from `internal/poller`, `internal/detection`, `internal/events`, `internal/db/migrations/000003_events.up.sql`, `internal/db/pool.go`; ecosystem claims cross-checked against multiple 2025-2026 sources — see Sources)

## Critical Pitfalls

### Pitfall 1: TTL delete on `events` silently resets seed mode and re-floods old catalogue as "new"

**What goes wrong:**
`Detector.isSeedMode` (`internal/detection/detector.go:84`) has exactly one signal for "has this artist/source ever been polled before": `HasAnyEvent(artist_id, source)` — zero rows means seed mode. Seed mode changes behavior materially: `seedNotifiedAt` marks every row inserted that cycle as already-notified (no Discord alert), because a first-ever poll is a baseline snapshot, not "56 new releases just dropped." If a naive `DELETE FROM events WHERE created_at < now() - interval '90 days'` empties every row for an artist whose watch history is older than 90 days (a real case for any artist that has been on the watchlist since near project launch, or any artist with a sparse release cadence), the very next poll cycle sees zero rows again, `isSeedMode` returns `true`, and every one of that artist's already-known releases gets re-inserted (the unique constraint no longer blocks them, since the old rows are gone) *and* is treated as newly seeded. Depending on the exact interaction with `NotifyPending`'s query, this is either a silent history-loss event (rows insert with `notified_at` pre-set, no alert, but the events feed/History UI's older history has vanished) or, if any code path treats a re-seed as fresh, a Discord flood of years-old releases reported as new.

**Why it happens:**
Retention is usually designed against "how old is this row" without checking what the row's *absence* means to the code that reads it. `events` is not a pure audit log here — it is the detection engine's only "seen" store (D-09, per the schema comment at `000003_events.up.sql:1-4`). Deleting a row doesn't just shrink a table, it forgets that the artist/release was ever observed.

**How to avoid:**
- Never delete the *newest* row for a given `(artist_id, source)` pair, regardless of age — at minimum, retain one row per `(artist_id, source)` as a permanent seed-mode sentinel, or better: track "artist has been polled at least once" as separate state (e.g. a `first_polled_at` column on `artists`, decoupled from individual event rows) so seed-mode detection never depends on row survival.
- If keeping a sentinel row is the chosen approach, exclude it from the age-based delete explicitly (`WHERE created_at < cutoff AND id NOT IN (SELECT max(id) FROM events GROUP BY artist_id, source)` or equivalent), not just by trusting that "90 days is long enough that this won't happen" — it will happen for exactly the artists that were seeded near go-live plus any artist with an active, evenly-spaced release cadence older than 90 days on its oldest row.
- Add a regression test that asserts: after a retention run that empties `events` for an artist/source pair, the *next* detection cycle for that pair does not re-fire alerts for external IDs the artist already had (i.e. `isSeedMode` must not silently flip back to `true` as a side effect of retention).

**Warning signs:**
- Discord suddenly re-posts releases from a year+ ago for an artist that's been tracked the whole time.
- History feed for a long-tracked artist has a gap or an unexplained reset around the 90-day retention boundary.
- `isSeedMode` returns `true` for an artist you know has prior events, right after a retention job ran.

**Phase to address:**
Events retention phase — the retention design must be reviewed against `internal/detection/detector.go`'s `isSeedMode`/`HasAnyEvent` contract *before* the delete query is written, not discovered afterward.

---

### Pitfall 2: TTL delete destroys the deluxe-change track-count baseline, reintroducing 04-04's already-fixed false positive

**What goes wrong:**
`groupBaseline` (`internal/detection/detector.go:115-130`) reads the mutable `track_count` column off the release-group's *own* `new_release` event row — there is no separate baseline table. The whole point of `hasBaseline` (distinguishing "never measured" from "measured as zero") is to stop a first-ever comparison from reading as "the count increased from 0," which 04-RESEARCH.md's Pitfall #1 already identified and 04-04 fixed. A naive age-based delete removes the `new_release` row for any release-group older than 90 days — which is the *majority* of tracked groups in steady state — wiping out `track_count` along with the row. The next time that group's release detail is fetched (e.g. a deluxe/anniversary reissue drops for a 2-year-old album), `groupBaseline` returns `hasBaseline=false` again, because the row that held the baseline is gone. Per the documented switch statement (`internal/detection/musicbrainz.go:333-337`), a missing baseline is treated as "establish, don't alert" — so the deluxe_change event **silently does not fire** for exactly the reissue scenario this feature exists to catch. This is the inverse failure of the original bug (false negative instead of false positive), but it is the same root cause: baseline state riding on a row's mere existence, and retention deleting that existence out from under it.

**Why it happens:**
04-04's design (see `internal/db/migrations/000003_events.up.sql:18-22`) explicitly chose to store the mutable baseline on the display row rather than a separate table — reasonable for v1.0 where nothing ever deleted event rows. Retention is the first feature to introduce row deletion into a schema that was designed assuming rows are append-only/immutable-except-for-baseline-mutation.

**How to avoid:**
- Exclude `new_release` rows that are the current baseline holder for their `release_group_mbid` from the retention delete — i.e. never delete a row that a live `groupBaseline` lookup could still target. Concretely: `WHERE NOT (event_type = 'new_release' AND release_group_mbid IS NOT NULL)` is the safest first cut (never delete any `new_release` row that owns baseline state), even if that means `new_release` rows are retained indefinitely while `guest_feature`/`deluxe_change` rows are pruned on the 90-day TTL.
- If unbounded `new_release` retention is unacceptable, this milestone needs to first extract `track_count`/baseline into its own table (`release_group_baselines(release_group_mbid PK, track_count, updated_at)`) decoupled from the individual event row's lifecycle — then the display row can be pruned freely and the baseline persists independently. This is a real schema migration, not a delete-query tweak; treat it as in-scope design work for the retention phase, not an afterthought.
- Add a regression test mirroring 04-04's own: seed a group's baseline, force-expire/delete its `new_release` row via the retention path, then feed the same or higher track count through `detectDeluxeChanges` and assert whether a `deluxe_change` event fires (know explicitly which behavior you're choosing, don't let it be accidental).

**Warning signs:**
- `baseline_established_count` (logged by `detectDeluxeChanges`) unexpectedly climbs for release-groups older than 90 days that should already have a baseline.
- A known reissue/deluxe drop for an old album produces no Discord alert.

**Phase to address:**
Events retention phase — this is the single highest-risk item in the whole milestone because it silently reintroduces a bug the project already spent a phase fixing (04-04). Retention scope for `new_release` rows must be decided explicitly, not left to "same TTL as everything else."

---

### Pitfall 3: TTL delete removes the dedup key's basis, causing old catalogue items to re-insert and re-notify as new

**What goes wrong:**
Idempotency for every event type rests entirely on the `events_dedup_key` unique constraint over `(event_type, source, external_id)` plus `ON CONFLICT DO NOTHING` (D-20, `internal/detection/detector.go:58-68`). This is a purely presence-based guarantee — there is no separate "seen ids" ledger. If retention deletes an old `new_release`/`guest_feature` row and MusicBrainz or Deezer's catalogue API still returns that same item on a later poll (which happens routinely — catalogue browse/search results are not bounded to "recent" items, `ReleaseGroupsByArtist` and `RecordingsByArtist` return an artist's *entire* history every cycle), the unique constraint no longer blocks the re-insert, `InsertEvent` succeeds, `newly` is `true`, and (outside seed mode) `notified_at` is left `NULL` — meaning `NotifyPending` will pick it up and Discord will re-post a years-old release as if it just dropped.

**Why it happens:**
The dedup design (D-20) was correct and sufficient for an append-only table. Retention breaks the assumption "once an external_id is in `events`, it stays there forever," which is exactly the assumption idempotency depends on.

**How to avoid:**
- This is the same root problem as Pitfalls 1 and 2 wearing a different hat: any retention scheme for this table must either (a) never delete the row that a future poll cycle's dedup check depends on, or (b) replace presence-based dedup with an explicit, retention-independent ledger (e.g. a lightweight `seen_external_ids(source, external_id, artist_id) ` table populated once and never pruned, separate from the richer/prunable display-oriented `events` rows).
- Cheapest correct option for this milestone: split "detection state" (never pruned — a narrow table of just the dedup keys and baseline) from "display history" (prunable on the 90-day TTL, feeds the History UI only). This reframes retention from "delete old rows" to "delete old *display* rows while detection state is retained separately" — a materially different, safer design than a blanket `DELETE ... WHERE created_at < cutoff`.
- If the team instead decides 90-day-old catalogue items genuinely should be forgotten (accepting the re-notify risk as intentional, e.g. "nobody cares about being re-alerted to an old song"), that must be a written, explicit decision in the phase plan — not a silent consequence of an unreviewed `DELETE` statement.

**Warning signs:**
- Discord posts a "new release" for a song/album that's been out for over 90 days.
- `inserted_count` in a `detection result` log line is non-zero for an artist with no actual new activity, shortly after a retention run.

**Phase to address:**
Events retention phase — same root cause as Pitfalls 1-2; solve all three with one design decision (what state is retention-exempt) rather than three separate patches.

---

### Pitfall 4: Concurrent worker pool reintroduces a check-then-act race on shared release-group baselines

**What goes wrong:**
The current sequential `for _, entry := range entries` loop in `RunMusicBrainzCycle` (`internal/poller/poller.go:229-258`) guarantees only one artist's detection logic runs at a time within a cycle. `detectDeluxeChanges`'s baseline update (`internal/detection/detector.go:328-337` / `internal/detection/musicbrainz.go:328-373`) is a classic read-then-write: `groupBaseline` (SELECT) followed later by `setGroupBaseline` (UPDATE), with no transaction or row lock spanning the two. Sequential execution makes this safe by construction — no two artists' detection calls ever interleave. Converting the per-artist loop to a bounded worker pool removes that guarantee: if two watched artists happen to share a `release_group_mbid` (a various-artists compilation, a genuine collaboration release-group, or simply two watchlist entries both getting fed the same group via MusicBrainz's data model), two goroutines can now read the same baseline concurrently, both observe "count increased," both independently compute `previousTrackCount`, and race to `setGroupBaseline` — one goroutine's insert/update is silently clobbered by the other's, either double-firing a `deluxe_change` alert or losing one entirely, with no error surfaced (the `-race` flag will *not* catch this — it's a valid, non-racy sequence of independent DB round trips, just semantically wrong under concurrency).

**Why it happens:**
The baseline design was correct for the concurrency model that existed when 04-04 was written (strictly sequential per-cycle processing). Nothing in `groupBaseline`/`setGroupBaseline` was built with concurrent callers in mind — there is no `SELECT ... FOR UPDATE`, no advisory lock, no atomic compare-and-set at the SQL level (unlike `InsertEvent`'s `ON CONFLICT DO NOTHING`, which *is* concurrency-safe by design).

**How to avoid:**
- Do not assume "the worker pool is per-artist so this can't race" — verify whether two artists can legitimately share a `release_group_mbid` in this dataset before dismissing this pitfall (compilations/various-artists releases make it plausible, not hypothetical).
- Serialize baseline mutation per `release_group_mbid` regardless of worker-pool concurrency: either (a) make `setGroupBaseline` a single atomic SQL statement (`UPDATE events SET track_count = GREATEST(track_count, $1) WHERE external_id = $2 RETURNING track_count` and compare pre/post, or a `SELECT ... FOR UPDATE` wrapping both reads and the write in one transaction), or (b) key the worker pool's task partitioning so that no two in-flight workers can ever process the same release-group concurrently (harder to guarantee correctly, since grouping isn't known until after the fetch).
- Prefer (a): pushing the read-modify-write into one atomic SQL statement removes the race regardless of how many workers run concurrently, and is strictly more correct than trying to reason about scheduling.
- Add a concurrency-specific regression test: two goroutines calling `detectDeluxeChanges`-equivalent logic against the same `release_group_mbid` concurrently, run with `go test -race`, asserting the final baseline and event count are correct (not just "no data race detected" — the `-race` flag alone won't catch this logical race).

**Warning signs:**
- Occasional missing or duplicated `deluxe_change` Discord alerts that don't reproduce when the cycle is re-run sequentially.
- `baseline_established_count`/`inserted_count` metrics that don't add up across a cycle's log lines when cross-checked against direct DB queries.

**Phase to address:**
Concurrent polling phase — this must be resolved *before* the worker pool ships, not discovered via a flaky-alert bug report post-launch, since it will not show up under `-race` and may not show up in low-traffic testing at all (needs artists that actually share a release-group to trigger).

---

### Pitfall 5: Worker pool causes a rate-limiter thundering herd at the start of every cycle, changing external-API behavior even though the limiter itself is unchanged

**What goes wrong:**
`golang.org/x/time/rate.Limiter` is safe for concurrent use and *does* enforce the configured long-run rate correctly across goroutines — the limiter itself is not the bug. The behavioral change is at the burst boundary: today, one sequential goroutine calls `limiter.Wait(ctx)` once per artist, so requests are naturally spread out. With N concurrent workers (env-configurable, default 3-5), at the instant a cycle starts, up to N goroutines call `Wait` simultaneously. If the limiter's burst size (`rate.NewLimiter(rate.Limit(cfg.MusicBrainzRateLimitPerSec), burst)`) is anywhere near or above N, all N requests can leave in the same instant rather than being spread across the interval — which is fine for MusicBrainz/Deezer's rate ceiling in aggregate, but changes request *pacing* in a way that's more likely to trip upstream abuse heuristics that look at burstiness, not just average rate (MusicBrainz in particular is known to be sensitive to non-uniform request patterns, separate from the documented average-rate limit).

**Why it happens:**
`rate.Limiter` guarantees the long-run average; it does not by itself guarantee even spacing unless burst is tuned tightly (e.g. burst=1). Nobody needed to think about burst size when there was only ever one caller.

**How to avoid:**
- Explicitly (re-)check both clients' `rate.NewLimiter` burst parameter (`internal/musicbrainz/client.go`, `internal/deezer/client.go`) once concurrency is introduced — a burst of 1 with the existing per-second rate is the safest choice for preserving today's request pacing under N concurrent workers; don't leave it at whatever default was chosen when only one caller existed.
- Confirm in a test with `rate.NewLimiter` and N concurrent goroutines exactly what the observed request timestamps look like (a simple timestamp-collection test against a fake limiter target), rather than assuming "the limiter handles it."

**Warning signs:**
- MusicBrainz 503s become more frequent right after the worker pool ships, even though the configured requests/sec value didn't change.
- Deezer's per-5-second window budget gets exhausted in a sudden spike rather than spread across the window.

**Phase to address:**
Concurrent polling phase — verify burst configuration as an explicit checklist item, not an assumption.

---

### Pitfall 6: Unrecovered panic in a worker goroutine crashes the whole process instead of failing just one artist

**What goes wrong:**
The sequential loop's error handling contract is explicit and load-bearing: "[a] per-artist fetch or detection error is logged and the cycle continues to the next artist — one unreachable or misbehaving artist must not cost the rest of the cycle" (`internal/poller/poller.go:204-206`). That contract only covers `error` returns, not panics — but today a panic anywhere in the loop body crashes the same goroutine the whole cron job runs on, which was already true before concurrency. What changes with a worker pool is that a naive implementation (e.g. `go func() { processArtist(entry) }()` with no `recover()`) still crashes the entire process on any worker's panic — Go does not isolate panics to "just that goroutine" the way the phrase "worker pool" implies; an unrecovered panic in *any* goroutine takes down the whole binary, full stop. If the team's mental model going in is "concurrency isolates failures better," that's backwards unless recovery is added explicitly, and the existing per-artist error-isolation contract must be extended to also survive a panic, which it does not currently need to (single goroutine, one bad artist just logs and `continue`s).

**Why it happens:**
"Worker pool" sounds like it implies fault isolation; in Go it does not, by default. The existing code has never needed a `recover()` because a panic already meant "crash and let the container orchestrator restart" was acceptable — but concurrency raises the stakes (N artists' work lost simultaneously) and often the goal of adding a worker pool is precisely to *not* regress the current "one bad artist doesn't cost the rest" guarantee.

**How to avoid:**
- Add an explicit `defer func() { if r := recover(); r != nil { ... } }()` inside each worker's task function, logging the panic with the artist's mbid/name attached (mirroring the existing error-log shape) and continuing to the next task — do not rely on the pool library (if one is used) to do this for you; verify it explicitly, since not every worker-pool implementation recovers panics by default.
- Decide and document what happens to that artist's cycle iteration on panic recovery: skip it (same as an error) is the behavior that preserves parity with today's contract.
- Test this: a stub `ReleaseGroupSource`/`AlbumSource` that panics for one specific artist, verify the cycle still completes and processes the remaining artists, and the process does not exit.

**Warning signs:**
- Process restarts (container orchestrator-level) correlating with poll cycles, with no corresponding `poll cycle failed` log line (a caught error logs; a crash doesn't get the chance to).

**Phase to address:**
Concurrent polling phase — part of the same worker-pool implementation work, not a separate hardening pass.

---

### Pitfall 7: Worker pool count silently serializes on `pgxpool` if DB pool sizing isn't reconsidered alongside it

**What goes wrong:**
`internal/db/pool.go`'s `PoolConfig` sets `ConnectTimeout`, `PingTimeout`, and `MaxConnIdleTime` explicitly but never sets `MaxConns` — meaning the pool runs on whatever `pgxpool` defaults to, unreviewed. Each concurrent worker's detection pass makes several sequential DB round trips per artist (`HasAnyEvent`, `ListExternalIDs` ×2-3, `InsertEvent` ×N, `GroupTrackCountBaseline`/`SetGroupTrackCountBaseline` per group) — a worker pool of 3-5 doesn't just add 3-5x HTTP concurrency to MusicBrainz/Deezer, it adds 3-5x concurrent DB connection demand on top of whatever the HTTP API handlers (watchlist CRUD, search proxy, history feed) are already using from the same shared pool. If the default pool size is smaller than worker-count + concurrent HTTP handler load, workers silently block on `pool.Acquire` rather than erroring — the worker pool "works" but delivers none of the intended wall-clock speedup, and under real load could start timing out DB calls from the HTTP handlers instead (a regression nobody was trying to introduce).

**Why it happens:**
Nobody needed to think about pool sizing when the only DB consumer during a poll cycle was one goroutine at a time; the worker-pool phase's stated goal (bounded, env-configurable concurrency for HTTP fan-out to MusicBrainz/Deezer) doesn't obviously imply "also revisit database pool sizing," but the two are coupled once the poller's DB calls happen concurrently too.

**How to avoid:**
- Explicitly size (or at minimum measure and document) `pgxpool`'s `MaxConns` relative to the new worker-pool size plus expected concurrent HTTP handler load, rather than leaving it as an unreviewed default.
- Load-test (even a simple local `docker-compose` run with the worker pool at its max configured size and a few concurrent HTTP requests) to confirm the worker pool actually reduces cycle wall-clock time as intended, not just "runs without erroring."

**Warning signs:**
- Enabling the worker pool doesn't meaningfully shorten poll-cycle duration in logs/metrics despite a higher configured pool size.
- HTTP request latency (watchlist/search/history endpoints) degrades specifically during active poll cycles after the worker pool ships.

**Phase to address:**
Concurrent polling phase — sizing check belongs in the same phase's verification/UAT, not deferred.

---

### Pitfall 8: First-ever frontend test suite breaks on React Router 7's loader/context requirements, not on the component logic itself

**What goes wrong:**
This codebase uses React Router 7 in framework mode (`react-router.config.ts`-driven, `ssr: false`, route modules under `web/app/routes/`), which means components under test frequently depend on router context (loaders, `useLoaderData`, `Link`, navigation) that plain `render()` from React Testing Library does not provide out of the box. The single most common first-time-Vitest-adopter mistake in a router-dependent app is writing a component test that renders a route component directly and immediately hits "invariant expected app router to be mounted" or an equivalent context-missing error that has nothing to do with the component's actual behavior — then "fixing" it by mocking away the router entirely, which defeats the purpose of testing anything that navigates or reads loader data.

**Why it happens:**
There is zero existing frontend test infrastructure to copy a working pattern from (`.planning/codebase/TESTING.md` confirms "No test framework configured in `web/package.json`") — this genuinely is the first test in the repo, so there's no established local convention for wrapping components in the right providers.

**How to avoid:**
- Use React Router's own testing utilities (`createRoutesStub` in RR7, the modern replacement for hand-rolled `MemoryRouter` wrapping) to render route components with realistic loader/action data rather than mocking `useLoaderData` per-test — this is the officially supported pattern for RR7's data APIs and avoids re-inventing router context wiring per test file.
- Establish one shared test-render helper (`renderWithRouter`/similar) in the very first test file so every subsequent test reuses it — do not let each contributor re-solve router-context setup independently, since inconsistent approaches here are exactly what makes a fledgling frontend suite flaky.
- For components that don't need routing at all (e.g. `CoverArt`, `EmptyState`, `PreferenceToggles` if they take plain props), test them with plain `render()` — don't reflexively wrap everything in router context "just in case," since that adds noise and slows tests for no benefit.

**Warning signs:**
- New tests fail with router-context invariant errors unrelated to the assertion being written.
- Test files accumulate ad hoc, slightly-different router/provider wrapping boilerplate instead of one shared helper.

**Phase to address:**
Frontend test suite phase — the shared render helper should be one of the first things built, before writing component-specific tests, so later tests inherit a working pattern instead of each one improvising.

---

### Pitfall 9: Over-mocking `fetch`/API calls produces a green suite that doesn't actually verify integration with the real Go backend's response shapes

**What goes wrong:**
It's tempting, when writing the *first* frontend tests ever, to mock every network call at the component boundary (mock the hook/loader function itself, never touch `fetch`) — this makes tests fast and simple to write, but it means the test suite can pass 100% while the actual JSON shape the frontend expects has silently drifted from what `internal/httpserver` currently serializes (e.g. `events.Event`'s `PreviousTrackCount *int32`/`ReleaseType *string` fields, or the watchlist preference toggle shape) — a real, live risk on this project since the Go handlers and the TypeScript types are hand-maintained independently with no generated client/OpenAPI contract enforcing them in sync.

**Why it happens:**
Mocking at the highest possible boundary is the path of least resistance for a first test suite, especially under time pressure to hit a coverage number — and nothing in a coverage percentage distinguishes "this test exercises real serialization/deserialization" from "this test mocks the network entirely."

**How to avoid:**
- For at least the data-shape-sensitive components (History feed, watchlist preference toggles), mock at the `fetch`/HTTP boundary (e.g. MSW — Mock Service Worker, or a hand-rolled `fetch` stub returning realistic fixture JSON copied from an actual `internal/httpserver` response) rather than mocking the loader/hook itself, so a JSON shape change in the Go handler would actually break the parsing code the test exercises.
- Base frontend test fixtures on real backend response shapes (mirroring how the Go test suite already transcribes live-verified fixtures per `.planning/codebase/TESTING.md`'s "Fixtures" section) rather than hand-writing plausible-looking JSON from memory.
- Don't treat 100% mock coverage as a substitute for the handful of true integration checks (even one Playwright/E2E-style smoke test against a running server would catch drift that pure component tests, however well-written, cannot) — full E2E is explicitly out of scope for this milestone per PROJECT.md, so this is a documented gap to accept, not silently solve by over-trusting unit-level mocks.

**Warning signs:**
- Frontend tests all pass after a Go handler's JSON field is renamed/retyped, because every test mocked the shape rather than exercising real parsing.
- Coverage percentage rises without any test file referencing an actual API response fixture.

**Phase to address:**
Frontend test suite phase — decide the mocking boundary (fetch-level vs. hook-level) as an explicit convention up front, in the phase's design/discussion step, not per-file as tests accumulate.

---

### Pitfall 10: Coverage gate retrofit breaks the build on day one because 100% of the existing frontend and much of the Go codebase currently has 0%/partial coverage

**What goes wrong:**
Two distinct retrofit failure modes, one per language:
- **Frontend:** `web/package.json` has no test framework at all today. A CI gate requiring 70% coverage on a codebase with 0% existing frontend tests cannot be satisfied by "add the gate and let developers fill in tests incrementally" — it fails the very first PR after the gate lands, for every file, unless the gate is scoped (new/changed files only) or the test suite is written *before* the gate is turned on.
- **Backend:** Go tests already exist (`.planning/codebase/TESTING.md` confirms unit + integration coverage across most packages) but there is "No explicit coverage target or requirement... Coverage measurement: Not enforced by CI" today — meaning nobody has verified what the *actual current* aggregate coverage percentage is. Turning on a 70% gate without first measuring today's real number risks either (a) the gate failing immediately on merge if actual coverage is below 70% (e.g. thin coverage in `cmd/`, wiring code, or newer packages), or (b) the gate passing trivially if some already-tested-but-thin package inflates the aggregate while a genuinely under-tested package (e.g. the new retention or worker-pool code this same milestone adds) hides beneath the blended average.

**Why it happens:**
Coverage gates are usually designed for "coverage should not regress from here," which implicitly assumes someone already measured "here." Retrofitting onto an existing codebase without that baseline measurement step turns the gate into either an immediate, indiscriminate build-breaker or (worse) a false-confidence pass that doesn't actually protect the new code this milestone is adding.

**How to avoid:**
- Measure both languages' current aggregate coverage *before* choosing 70% as the number to enforce — if either is currently below 70%, the gate must either (a) be scoped to diff/patch coverage (only newly-changed lines must hit 70%, a git-diff-aware coverage tool, not the whole-repo aggregate) so existing untested code isn't retroactively required to reach the bar, or (b) the 70% target is deferred until a preparatory pass brings the baseline up first — don't silently redefine "70%" as "70% of files we happen to already cover" without writing that decision down.
- For the frontend specifically, since it starts at literally 0%, the only viable order is: write the initial Vitest+RTL suite first (this same milestone), measure what percentage it lands at, and only then decide whether 70% is achievable immediately or needs a short ratchet-up period — do not turn on a hard 70%-or-fail gate in the same PR that introduces the first test file with no prior measurement.
- Prefer a ratchet (gate can only get stricter over time, starting from whatever today's real number is, moving toward 70%) over a flat threshold if the two languages' actual starting points turn out to differ meaningfully — a uniform hard-coded 70% is a reasonable target but an unreviewed one is exactly the "unreachable threshold blocks every PR" failure mode.
- Watch for coverage-gaming once the gate is live: a developer facing a failing gate under time pressure will write the shortest possible test that exercises a line without asserting anything meaningful (e.g. calling a function and asserting only `err == nil`) purely to move the number — code review, not the coverage tool, is what catches this; the coverage percentage itself cannot distinguish a meaningful test from a gaming one.

**Warning signs:**
- The very first PR after the coverage-gate CI job lands fails on files nobody touched in that PR (whole-repo aggregate below threshold, not diff-based).
- New test files with high line coverage but assertions limited to "no error" / "no throw," added right after the gate started blocking merges.

**Phase to address:**
CI coverage gate phase — must come *after* the frontend test suite phase in the roadmap (can't gate what doesn't exist yet), and must include an explicit "measure current baseline" step before the threshold value is hard-coded into the workflow.

---

### Pitfall 11: `go test -race -p 1` and coverage merging interact badly if not verified together

**What goes wrong:**
The existing Go test suite is explicitly designed to run with `-p 1` (packages sequential, not parallel) for integration tests because they "share a single Docker Postgres instance... migrations run `DROP SCHEMA public CASCADE`" (`.planning/codebase/TESTING.md`). Adding `-coverprofile` to that same invocation is generally safe, but combining `-race` + `-coverprofile` + `-p 1` across many packages, then merging multiple `coverage.out` files (if CI runs `test-short`/`test-integration` as separate jobs, or matrices across Go versions) needs an explicit merge step (`gocovmerge` or `go tool covdata`) — a naive setup that just runs coverage once on `test-short` (unit tests, no DB) will under-report real coverage by excluding every DB-backed integration test's contribution, producing an artificially low number that then either fails the gate for the wrong reason or gets "fixed" by lowering the threshold rather than fixing the measurement.

**Why it happens:**
`make test-short` and `make test-integration` are two distinct existing invocations for a reason (DB availability), but a coverage gate naively wired to just one of them (typically the faster `test-short`, since CI wants quick feedback) silently coverage-measures only a subset of the actual test suite.

**How to avoid:**
- Decide explicitly whether the coverage gate measures `test-short` only, `test-integration` only, or a merge of both — and verify the chosen number against a manual `go tool cover -func=coverage.out` run locally before trusting the CI-computed percentage.
- If both are measured, merge the profiles (`go tool covdata merge` or run `-coverprofile` against the same output path across both invocations with `-coverpkg=./...` so cross-package coverage isn't lost) — don't let CI silently report whichever job ran last.

**Warning signs:**
- Locally-measured coverage (`go test ./... -race -count=1 -p 1 -coverprofile=coverage.out`) doesn't match what CI reports for the same commit.
- Coverage percentage swings noticeably depending on which `make` target's logs happen to include the coverage step.

**Phase to address:**
CI coverage gate phase — verify the merge/measurement approach as part of that phase's own testing, not assumed to "just work" because `-coverprofile` is a standard flag.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Ship the 90-day retention delete without first splitting detection state (dedup keys, baselines) from display history | Fastest path to "retention exists" | Silent false negatives (Pitfall 2), false positives (Pitfall 3), and seed-mode resets (Pitfall 1) — all three are correctness bugs in the core detection engine, the project's primary value proposition | Never — this is the one shortcut this milestone cannot afford to take; explicitly exclude baseline/dedup-critical rows from the delete at minimum, even if the "proper" separate-table refactor is deferred |
| Turn on the 70% coverage gate at the same time the first Vitest tests are written | One PR, one milestone item checked off | Gate fails on day one or is quietly scoped down to "70% of files we bothered to test," undermining its own purpose | Only if the gate is diff/patch-coverage-based from day one, or the baseline is measured and the threshold explicitly set below/at that number with a documented ratchet plan |
| Mock every frontend network call at the hook/loader boundary instead of the fetch boundary | Faster to write, fewer moving parts per test | Coverage number rises with zero protection against Go/TS response-shape drift (Pitfall 9) | Acceptable for components that genuinely have no data-shape risk (pure UI state, styling logic); not acceptable for History feed / watchlist components that directly consume `internal/httpserver` JSON |
| Size the worker pool without revisiting `pgxpool` `MaxConns` | One less thing to configure this milestone | Worker pool silently serializes on DB acquisition, delivering none of the intended speedup, discovered only under load (Pitfall 7) | Only if `MaxConns` is confirmed already comfortably larger than worker-pool-size + expected concurrent HTTP load — verify, don't assume |
| Leave `setGroupBaseline`/`groupBaseline` as two separate SQL statements under the worker pool | No detection-code changes needed for the concurrency phase | Non-deterministic baseline corruption for artists sharing a release-group, silent and rare enough to be hard to reproduce (Pitfall 4) | Never, once the poller is concurrent — this needs an atomic SQL statement or explicit locking regardless of how rare the trigger condition is, because "rare and silent" is worse than "common and loud" for a detection engine |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| `internal/detection` ↔ Postgres `events` table | Treating `events` as a pure append-only audit log safe for age-based pruning | Recognize `events` is simultaneously the seed-mode signal (Pitfall 1), the baseline store (Pitfall 2), and the dedup ledger (Pitfall 3) — retention design must account for all three roles, not just "row age" |
| `internal/poller` worker pool ↔ `golang.org/x/time/rate` | Assuming the limiter "just handles" concurrency because it's documented as goroutine-safe | Goroutine-safety guarantees correctness of the long-run rate, not request pacing/burstiness — explicitly review/tune the burst parameter once callers go from 1 to N (Pitfall 5) |
| `internal/poller` worker pool ↔ `internal/db` (`pgxpool`) | Adding worker-pool concurrency without revisiting the shared DB pool's `MaxConns` | Size or explicitly verify `MaxConns` against worker-pool-size + concurrent HTTP handler demand (Pitfall 7) |
| React Router 7 route components ↔ Vitest/RTL | Rendering route components with plain `render()`, hitting router-context invariants, then over-mocking the router away entirely | Use RR7's `createRoutesStub` (or equivalent officially-supported test utility) and one shared render helper from the first test file onward (Pitfall 8) |
| Frontend fetch calls ↔ Go JSON handlers | Mocking at the hook/loader boundary so response-shape drift between `internal/httpserver` and the TS types is invisible to tests | Mock at the `fetch`/HTTP boundary with fixtures transcribed from real backend responses for data-shape-sensitive components (Pitfall 9) |
| CI coverage tooling ↔ existing `-p 1`/`-race`/dual `make test-short`+`test-integration` split | Wiring the coverage gate to only one of the two existing test invocations, under-reporting real coverage | Explicitly decide and verify which invocation(s) feed the coverage number, merging profiles if both matter (Pitfall 11) |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Unbounded single-transaction `DELETE FROM events WHERE created_at < cutoff` | Retention job holds long-running locks, causes bloat/vacuum pressure, may time out or block concurrent `InsertEvent` writes from an in-flight poll cycle | Batch the delete (`DELETE ... WHERE id IN (SELECT id ... LIMIT N)` looped until zero rows affected), schedule during low-traffic windows, and run it under its own CAS overlap guard (mirroring `mbRunning`/`dzRunning`) so a slow retention run can't overlap itself on the next cron tick | Table sizes with a meaningful backlog of aged rows — likely modest for a portfolio-scale project today, but the *pattern* (single giant DELETE) is the trap regardless of current row count, and this milestone is explicitly about scale-readiness |
| Worker pool sized above what `pgxpool`'s default `MaxConns` and MusicBrainz/Deezer's real rate ceiling can usefully absorb | Configuring `WORKER_POOL_SIZE=20` "for speed" delivers no measurable improvement over 3-5, because the bottleneck was never goroutine count | Default (3-5) is already reasoned for this reason — treat any push toward a much higher configured value as needing the Pitfall 7 sizing check first, not as a free win | Any deployment where an operator cranks the env var up without re-verifying pool/rate assumptions |
| Whole-repo coverage recomputed on every CI run across two languages | CI pipeline duration creeps up as both suites grow, disproportionately from coverage instrumentation overhead (Go `-race` + coverage is measurably slower than either alone) | Keep `-race` + coverage scoped to what the gate actually needs (don't re-run full coverage on every job in a matrix if one canonical job can produce the number) | Once the frontend suite and Go coverage are both nontrivial in size — worth revisiting CI job structure at that point, not necessarily now |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Retention job's DELETE query built via string concatenation of a configurable retention-days value (if the env var is read directly into SQL rather than as a bound parameter) | SQL injection surface on a value that's normally "safe" (an integer from env), but any future path that lets a user-influenced value reach this query without parameterization is a real risk class for this codebase, which otherwise uses sqlc's parameterized queries throughout | Route the retention interval through sqlc like every other query in this codebase (`internal/db/sqlc`) — a bound `INTERVAL` parameter, never a formatted string — consistent with the project's existing "type-safe generated queries" convention |
| Frontend tests that hardcode a real-looking Discord webhook URL, API token, or DSN in a fixture "because it's just a test file" | Gitleaks (already enforced pre-commit and in CI per PROJECT.md constraints) may or may not flag a plausible-but-fake test credential depending on pattern specificity — teams sometimes disable/allowlist a pattern to unblock a commit, then the allowlist rule quietly weakens real-secret detection | Use obviously-fake, clearly-non-matching values in test fixtures (the existing Go test suite already does this — mirror that convention in the new frontend fixtures rather than inventing a new one) |
| Worker-pool concurrency exposing a previously-theoretical TOCTOU in a security-adjacent path (none currently known in the poller, but concurrency conversions are exactly where this class of bug tends to surface) | A future contributor adding logic to the concurrent poller assumes today's sequential-safe patterns are still safe under concurrency without re-auditing | Treat the concurrent-polling phase as warranting the project's existing security-review step (per CLAUDE.md's GSD workflow) specifically because concurrency changes threat models even when no new external input is introduced |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|------------------|
| Retention silently removes History feed entries older than 90 days with no UI indication | A user browsing history sees what looks like "nothing happened before this date" rather than "older history was intentionally pruned," which reads as a bug | Even a simple, non-blocking UI note ("showing releases from the last 90 days") sets correct expectations — low effort, avoids support-question-shaped confusion for a portfolio demo audience specifically (reviewers poking around the History feed) |
| Concurrent worker pool changes per-cycle log ordering (interleaved `poll result`/`detection result` lines across artists instead of one clean artist-at-a-time block) | Anyone reading logs to debug a specific artist's poll behavior now has to filter by `artist_mbid`/`cycle_id` instead of reading top-to-bottom | Confirm `cycle_id` (already present per-cycle) plus `artist_mbid` is sufficient to reconstruct one artist's story from interleaved logs — if not, this is a good moment to also add a per-artist correlation field, since concurrency is exactly what breaks "logs are naturally grouped by proximity" |

## "Looks Done But Isn't" Checklist

- [ ] **Frontend test suite:** Passing tests exist, but verify they actually wrap route components in real router context (Pitfall 8) rather than mocking the router away — a suite that passes because everything is mocked is not the same as a suite that would catch a real regression.
- [ ] **Frontend test suite:** Coverage number looks good, but verify at least the History/watchlist components have tests that exercise real fixture JSON shapes (Pitfall 9), not just mocked hook return values.
- [ ] **CI coverage gate:** Gate is green, but verify it's measuring the intended test invocation(s) (`test-short` vs `test-integration` vs merged, Pitfall 11) and check for gaming (assertion-free tests added right after the gate went live).
- [ ] **Events retention:** Old rows are being deleted on schedule, but verify with a targeted test: after retention runs, does the next poll cycle for an affected artist correctly *not* re-flag old external IDs as new, and does `groupBaseline` still return the correct `hasBaseline=true` for groups that predate the cutoff (Pitfalls 1-3)?
- [ ] **Events retention:** Retention job runs, but verify it's guarded against overlapping itself (mirrors the poller's `mbRunning`/`dzRunning` CAS pattern) and against racing an in-flight poll cycle's writes to the same table.
- [ ] **Concurrent polling:** Cycle completes faster with the worker pool enabled, but verify wall-clock improvement is real (Pitfall 7's pool-sizing check) and not masked by DB-pool serialization.
- [ ] **Concurrent polling:** No data races reported by `-race`, but verify with a targeted test for the *logical* race on shared release-group baselines (Pitfall 4) — `-race` does not catch this class of bug.
- [ ] **Concurrent polling:** One artist's fetch/detection error doesn't stop the cycle (existing contract), but verify one artist's *panic* also doesn't crash the whole process (Pitfall 6) — this is a new failure mode concurrency introduces that the current code has never needed to guard against explicitly.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|------------------|
| Retention already deleted rows and seed mode/baselines are now corrupted for some artists (Pitfalls 1-3) | HIGH | If MusicBrainz/Deezer catalogue data is still available upstream (it is — these are external sources of truth), a targeted re-seed pass can rebuild baselines by re-fetching each affected artist's current catalogue and re-establishing baselines via the existing `!hasBaseline` code path — but this cannot recover *which* releases were already notified pre-deletion, so a period of "no re-notification of stuff we've truly already told users about" needs manual verification or explicit user communication in a real deployment (less critical for a portfolio project, but worth noting the limitation) |
| Worker pool shipped without atomic baseline updates and has been silently corrupting shared-release-group baselines in production (Pitfall 4) | MEDIUM | Fix `setGroupBaseline`/`groupBaseline` to use one atomic statement, then run a one-off audit query comparing each release-group's stored baseline against a fresh MusicBrainz fetch of its actual max track count, correcting any drift found — bounded work since it only affects release-groups shared across multiple watched artists (a small subset) |
| Coverage gate merged and immediately started blocking unrelated PRs on pre-existing low-coverage files (Pitfall 10) | LOW | Switch the gate from whole-repo aggregate to diff/patch-coverage scoping (most CI coverage actions support this natively) — this is a workflow-config change, not a code change, and can ship same-day |
| Frontend tests all mock the network boundary and a real Go/TS shape mismatch ships undetected (Pitfall 9) | MEDIUM | Retrofit fetch-boundary mocking (MSW or fixture-based fetch stub) into the highest-risk existing test files (History feed, watchlist) using real captured response bodies from a running server — doesn't require rewriting every test, just the data-shape-sensitive subset |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Seed-mode reset on retention delete (1) | Events retention phase | Test: retention run on an artist with prior events, then next cycle does not re-flag known external IDs |
| Baseline loss on retention delete (2) | Events retention phase | Test: retention run on a release-group with an established baseline, then a higher fresh track count still correctly fires `deluxe_change` (not silently re-establishes) |
| Dedup key loss on retention delete (3) | Events retention phase | Test: retention run, then a poll cycle re-fetching a pruned external ID does not re-insert/re-notify it |
| TOCTOU on shared release-group baseline under concurrency (4) | Concurrent polling phase | Test: two goroutines racing `detectDeluxeChanges`-equivalent logic on the same release-group under `-race`, asserting final state correctness (not just absence of a data race) |
| Rate-limiter burst/pacing change under concurrency (5) | Concurrent polling phase | Test: N concurrent `Wait()` calls against the real limiter config, asserting observed request timestamps match intended pacing |
| Unrecovered panic crashes the process (6) | Concurrent polling phase | Test: one worker's task panics, cycle still completes for remaining artists, process does not exit |
| DB pool sizing vs. worker-pool concurrency (7) | Concurrent polling phase | Load test / measurement: confirm cycle wall-clock time improves with the worker pool enabled vs. disabled at the configured pool size |
| Router-context test setup (8) | Frontend test suite phase | First test file establishes a shared render helper using RR7's supported test utilities; later tests reuse it without reinventing router wrapping |
| Over-mocked network boundary (9) | Frontend test suite phase | Explicit convention documented: which components get fetch-boundary mocks with real fixture JSON vs. hook-boundary mocks |
| Coverage gate breaks on unmeasured baseline (10) | CI coverage gate phase | Baseline measured and recorded before the threshold is hard-coded into the CI workflow; gate scoped to diff coverage if baseline is below 70% |
| Coverage measurement/merge gaps across `-p 1`/dual test targets (11) | CI coverage gate phase | Manually cross-check CI-reported coverage percentage against a local `go tool cover -func` run on the same commit |

## Sources

- `internal/poller/poller.go`, `internal/detection/detector.go`, `internal/detection/musicbrainz.go`, `internal/db/migrations/000003_events.up.sql`, `internal/events/service.go`, `internal/db/pool.go`, `internal/config/config.go` — read directly from the drop-tracker codebase — confidence HIGH (this is the actual current implementation, not inferred)
- `.planning/PROJECT.md`, `.planning/codebase/TESTING.md` — project constraints and current testing state — confidence HIGH
- [React Testing Library + Vitest: The Mistakes That Bite](https://medium.com/@samueldeveloper/react-testing-library-vitest-the-mistakes-that-haunt-developers-and-how-to-fight-them-like-ca0a0cda2ef8) — confidence MEDIUM
- [How to Fix Flaky Vitest Tests: Common Causes and Proven Solutions](https://deflaky.com/blog/vitest-flaky-tests) — confidence MEDIUM
- [Switching from jest to vitest causes act(...) warnings — vitest-dev/vitest#1242](https://github.com/vitest-dev/vitest/issues/1242), [testing-library/react-testing-library#1413](https://github.com/testing-library/react-testing-library/issues/1413) — confidence MEDIUM (community-reported, corroborated across multiple independent issues)
- [Lightweight code coverage quality gate](https://medium.com/@darbj95/lightweight-code-coverage-quality-gate-bc595d18bf1), [Code Coverage: Measure, Improve, and Scale Quality in CI](https://www.harness.io/blog/code-coverage-measure-improve-and-scale-quality-in-ci) — coverage-gate retrofit and gaming pitfalls — confidence MEDIUM
- [`golang.org/x/time/rate` package docs](https://pkg.go.dev/golang.org/x/time/rate) — Limiter concurrency-safety and burst semantics — confidence HIGH (official package documentation)
- General idempotency/TTL-vs-deduplication tension (webhook/event-processing literature, multiple 2026-dated independent sources) corroborating that TTL-based cleanup of state that also serves as a dedup/idempotency ledger is a recognized failure class, not unique to this codebase — confidence MEDIUM

---
*Pitfalls research for: drop-tracker v1.1 Hardening & Scale Readiness milestone*
*Researched: 2026-08-12*
