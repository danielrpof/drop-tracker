# Phase 16: Rollback-Safe Migrations - Context

**Gathered:** 2026-09-04
**Revised:** 2026-09-04 (post-context grill — findings S1..S8, see `<revisions>`)
**Status:** Ready for planning

<domain>
## Phase Boundary

A **CI-only** guardrail plus a written rule. No VPS, no deploy job, no change to the
application's runtime **behavior** — the Go added is CI-helper `cmd/` tooling and test
code, plus one behavior-preserving refactor of `internal/db/migrate.go` (extract an
unexported source seam, D-18, so the SC #4 test can drive the real boot path).
Delivers:

1. **MGRT-01** - a CI check that boots the **previously-released** image against a database
   migrated to the **current branch's** schema and passes only if that older binary starts
   and stays healthy - exercising a **write** path (`POST /watchlist`) as well as reads, so
   a contract that breaks the old binary's INSERTs is caught (N-1 backward-compatibility
   check; S1).
2. A **static SQL guard** that turns a branch red when a new migration carries a
   backward-incompatible change (N-1 break) **or** an unsafe-forward change (deploy
   hazard), with class-specific failure messages, and that **cross-references the previous
   release's `queries/*.sql`** so a drop/rename of a column the old binary still queries is
   caught deterministically every run (SC #2; S1/S4/S5).
3. A hermetic test closing the research's one MEDIUM-confidence assumption: the older
   binary's boot migration **no-ops** against an ahead-of-source `schema_migrations`
   version rather than erroring - calling the same code path `RunMigrations` uses, via an
   unexported source seam (SC #4; S3).
4. **MGRT-02** - the expand/contract rule written down where a migration author will meet
   it (SC #3).

**In scope:** MGRT-01, MGRT-02. Edits `.github/workflows/full-pipeline.yml` (a `changes`
prelude job + the guard job + the N-1 boot job); adds `cmd/migration-check/`, a `go run`
migration helper, an unexported `runMigrationsWithSource` seam in `internal/db/migrate.go`,
an `internal/db` test, and `internal/db/migrations/README.md`; small pointer edits to
`CLAUDE.md`. Reads `queries/*.sql` at previous-release tags via `git show` (read-only).

**Out of scope:**
- The VPS deploy job and auto-rollback control flow (Phase 17, DPLY-01).
- The Postgres backup + restore procedure (ROADMAP flags it for Phase 16 *or* 17 - deferred
  to Phase 17's provisioning runbook; this phase is CI-only and never touches a real DB).
- Enforcing "no blocking DDL in boot migrations" as a machine check - it stays a written
  rule only (see D-08).
- Writing/testing `down` migrations as a rollback mechanism - the app only ever runs `Up()`;
  rollback is image-swap + N-1 schema compatibility, not `migrate down`.
- Any `docs/adr/` convention (not introduced here - see D-09).
- Implementing multi-hop rollback safety - this phase verifies exactly **one** hop (N-1).
  Phase 17's auto-rollback must therefore be constrained to strictly N-1 (D-17); that
  constraint is recorded here but locked in Phase 17's discuss/spec.
- A `dirty`-`schema_migrations` recovery path - a half-applied forward migration blocks the
  old binary too; that is Phase 17's Postgres restore-from-backup territory, noted in D-02.

</domain>

<decisions>
## Implementation Decisions

### HEAD-schema setup (the N-1 boot check)
- **D-01:** CI brings the throwaway Postgres to the current branch's schema with a small
  `go run` helper that calls `internal/db.RunMigrations` (the exact embedded-`iofs` +
  bounded-retry code the app runs at boot) against a Postgres service container - **not** a
  full `docker build` of the current image, and **not** the `golang-migrate` CLI (a third
  code path the app never uses). Fidelity of the *current* image's own boot is already
  covered by `build-scan` and the `test` job.
- **D-02:** SC #4 is proven by a **dedicated hermetic Go test in `internal/db`** that runs
  on every CI run: migrate a DB to version N+1 with the full embedded set, then run
  `RunMigrations` (or an equivalent `migrate.Up()`) with an `iofs` source **truncated to
  1..N** and assert `nil` (the `ErrNoChange` path). The N-1 boot job is real-world
  confirmation *on top* when a PR actually adds a migration, but the deterministic
  every-run guarantee lives in the Go test. Planner: confirm an `iofs` source can be built
  over a subset of the embedded FS (e.g. a test-only `embed.FS` or a filtered
  `fs.Sub`/temp dir) without exporting `migrationsFS`.
  — **REVISED 2026-09-04 (S3):** the test calls the new unexported `runMigrationsWithSource`
  seam (D-18) with a synthetic truncated source, so it exercises the real boot code path,
  not raw `migrate.Up()`. Run it **first** in the phase plan — it pins the milestone's one
  MEDIUM-confidence assumption and there is no plan B if it fails. See `<revisions>`.
- **D-03:** The N-1 boot check asserts "starts and stays healthy" as: poll `/health` to a
  200 for a short **sustained** window, **then** `GET /watchlist` and `GET /events` must
  each return 200. Those handlers run representative sqlc queries, so a contracting
  migration that breaks the old binary's read paths surfaces as a 500 rather than passing
  silently. `/health` alone only pings the DB and would miss query-level schema
  incompatibility. The N-1 image runs with **no `INSTANCE_PASSPHRASE`** so those routes are
  reachable unauthenticated (Phase 14 GATE-07 inert path).
  — **REVISED 2026-09-04 (S1):** GET-only misses the poller's write paths (the app's core
  job). The boot job additionally does `POST /watchlist` against the N-1 image and asserts
  the row reads back, exercising the old binary's `watchlist.Add` / `UpsertArtist` INSERTs.
  `GET /watchlist` (`200 []`) and `GET /events` (`200 {events:[]}`) on an empty DB are
  confirmed — the prior planner TODO is closed. Env var is `HTTP_PORT`, not `PORT` (S8).
  See `<revisions>`.
- **D-04:** When the previous released image cannot be fetched: **skip-green** with a
  logged notice **only if `svu current` finds no prior release tag at all** (genuine
  bootstrap). If a prior tag exists but the pull fails (eviction, ghcr.io outage), the
  check goes **red** - an unverifiable rollback must not be silently masked. Add one pull
  retry before declaring failure.

### Destructive-migration detection
- **D-05:** SC #2 is answered by a **static SQL guard**, distinct from the N-1 boot job.
  The guard scans the branch's new migration SQL, fails fast with the expand/contract rule
  text plus the offending file and line. The N-1 boot job stays the MGRT-01 mechanism
  (catches subtler real incompatibility the pattern list can't see). The two are
  complementary; both are required (see D-10).
- **D-06:** The guard is a **small, stdlib-only, unit-tested Go tool at
  `cmd/migration-check/`** - mirrors the `cmd/coverage-report` precedent from Phase 15
  (on-brand for the portfolio, no new dependency). Not `squawk` (adds a dev-tool dep + a
  rule-config file, diverges from the repo's hand-roll pattern). Not inline bash/grep in
  YAML (untested brittle regex - the anti-pattern Phase 15 moved away from).
  — **Reversibility:** reversible - self-contained `cmd/` package + one workflow step.
- **D-07:** Guard scope is **diff-scoped**: it inspects only migration files **added on
  this branch vs. the `main` merge-base**, so a destructive statement that shipped in an
  earlier release never re-reddens the build. A destructive statement in a branch-new
  migration is **red unless** the file carries an explicit annotation comment - shape
  roughly `-- migration-check:allow-destructive expand-shipped-in=vX.Y.0 reason=<text>` -
  which the tool **requires** (both `expand-shipped-in` and `reason`) and echoes into its
  output. The annotation is self-documenting and travels with the migration. Not a
  separate allowlist file (approval reason lives away from the migration, list grows
  forever). Planner: define the exact annotation grammar and whether `expand-shipped-in`
  is validated against existing tags or just recorded.
  — **REVISED 2026-09-04 (S2/S5):** diff base is computed once by the `changes` prelude job
  (D-16), not per-job — `github.event.before..github.sha` for `push` (fixes the
  direct-to-`main` no-op), merge-base for `pull_request`. `expand-shipped-in` **is**
  validated: must be a real tag, warn if older than `svu current` (D-13/D-17). See
  `<revisions>`.
- **D-08:** The guard flags **only the rollback-breaking destructive set**: `DROP COLUMN`,
  `DROP TABLE`, `RENAME` (column or table), type-narrowing `ALTER COLUMN ... TYPE`, and
  `ADD COLUMN ... NOT NULL` without a `DEFAULT`. It does **not** enforce "no blocking DDL"
  (non-`CONCURRENTLY` `CREATE INDEX`, table-rewriting `ALTER`) - that stays a written
  MGRT-02 rule only, given drop-tracker's tiny single-operator tables and the
  golang-migrate-wraps-each-migration-in-a-transaction vs. `CONCURRENTLY` wrinkle.
  Planner/researcher: type-narrowing is the hardest pattern to detect reliably
  (`varchar(255)->varchar(100)`, `int->smallint`, added `CHECK`, tightened `NOT NULL` on an
  existing column) - RESEARCH.md should scope what's feasible and what the known blind
  spots are; the N-1 boot job is the backstop for what the pattern match misses.
  — **REVISED 2026-09-04 (S4):** the set splits into two finding **classes** with distinct
  messages — *backward-incompatible* (N-1 break: DROP/RENAME/type-narrowing) and
  *unsafe-forward* (deploy hazard: `ADD COLUMN ... NOT NULL` no `DEFAULT`, which never
  breaks the old binary but fails/locks a non-empty table). D-15 adds the previous-release
  query cross-reference. See `<revisions>`.

### Rule documentation home (MGRT-02 / SC #3)
- **D-09:** Primary home is **`internal/db/migrations/README.md`**, next to the `.sql`
  files - unmissable when adding a migration. One-line pointers from **CLAUDE.md's
  "Definition of Done"** section and from the **`cmd/migration-check` failure message**
  drive people to it. No `docs/adr/` directory is introduced (the repo has deliberately
  deferred that convention).
- **D-10:** README contents: the expand/contract rule and the N-1 invariant stated plainly;
  a concrete **expand -> backfill -> contract** walkthrough citing the existing
  `000007_backfill_events_watched_artist_name` migration as the in-tree precedent; a
  copy-paste **"before you merge a migration" checklist**; and a section documenting what
  `cmd/migration-check` enforces and the `allow-destructive` annotation syntax. Not a
  full migration-authoring guide (golang-migrate mechanics, naming, local testing) - that
  breadth isn't needed for SC #3.

### CI job shape
- **D-11:** **Both checks block a merge.** The `cmd/migration-check` guard and the N-1 boot
  job are both added to **`build-scan`'s `needs:`** array, gating the release path exactly
  like `vet` / `lint` / `test` / `frontend-test`. This matches the milestone rationale - a
  non-backward-compatible migration must not be able to merge once auto-rollback exists.
  Not the `coverage-comment` report-only pattern. Flakiness is bounded by D-04
  (skip-green only on true bootstrap) + the D-04 pull retry.
  — **Reversibility:** reversible - removing an entry from `build-scan.needs:` is a
  one-line workflow edit, but ROADMAP SC #2 ("turns that check red") expects the blocking
  behavior.
  — **REVISED 2026-09-04 (S6/S7):** both checks stay blocking. Flakiness is now also
  bounded by the D-12 path filter (the image pull only happens on migration PRs). Residual
  risk explicitly accepted: a transient ghcr.io failure on a migration PR blocks that one
  PR — mitigation is the D-04 retry + manual re-run; not worth a report-only interim.
- **D-12:** The checks run on **every push and every PR** - a natural consequence of
  sitting in `build-scan.needs:` (which itself runs on `push` and `pull_request`). The
  redundant post-merge run on `main` is cheap insurance and is the only gate if someone
  pushes directly to `main`. The static guard diffs against the `main` merge-base in all
  cases (feature-branch push, PR, and post-merge).
  — **REVISED 2026-09-04 (S2/S6):** the "merge-base in all cases" line is **withdrawn** —
  on a push to `main` the merge-base is `HEAD` and the diff is empty, so the guard could
  never catch a direct-to-`main` destructive migration (the case this decision cited). Diff
  base now comes from the `changes` prelude job (D-16): `github.event.before..github.sha`
  for `push`, merge-base for `pull_request`. The **`changes` job and the static guard run
  unconditionally** (cheap); the **N-1 boot job runs its real steps only when a migration
  file changed** (`if: needs.changes.outputs.migrations_changed == 'true'`) — a skipped job
  in `needs:` counts as success, so the gate is intact. See `<revisions>`.
- **D-13:** The previous released image is resolved with **`svu current`** on a
  `fetch-depth: 0` checkout -> `ghcr.io/danielrpof/drop-tracker:<tag>`. Same tool the
  `release` job uses; on a PR this tag is exactly what a rollback would land on. Not a
  ghcr.io API query (extra token handling, can disagree with git tags on a half-failed
  push). Not a committed rollback-floor file (manual step that drifts).
  — **REVISED 2026-09-04 (S5):** that single resolved tag is the **only** rollback target —
  Phase 17 auto-rollback is strictly N-1 (D-17). The same tag drives D-15's query
  cross-reference. Every new job that resolves a tag or reads history checks out
  `fetch-depth: 0` **and fetches tags** (hard requirement, not implied).
- **D-14:** Boot wiring: a **GitHub Actions `services: postgres`** container on the runner;
  the D-01 `go run` helper applies HEAD schema against `localhost:5432`; then
  `docker run --network host` the N-1 image with `DATABASE_URL` + `PORT` (and no
  `INSTANCE_PASSPHRASE`) in its env; `curl localhost:$PORT` for the D-03 assertions. No
  docker-compose file or image-parameterization is added now (that's Phase 17's prod
  compose work).
  — **REVISED 2026-09-04 (S8):** the env var is **`HTTP_PORT`** (`internal/config`,
  `envDefault:"8080"`), not `PORT`. The N-1 image boots with just `DATABASE_URL` +
  `HTTP_PORT`; `DISCORD_WEBHOOK_URL` is optional and the 15m poll interval never fires
  inside the health window. Boot job also issues the D-03 `POST /watchlist` write assertion
  (S1).

### Claude's Discretion
- Exact names for the new `cmd/` tool (`cmd/migration-check` is the working name) and the
  `go run` migration helper (could be a `cmd/` package or a `//go:build tools`-style
  helper or a test-mode flag on an existing binary).
- The precise `allow-destructive` annotation grammar (D-07). Tag-validation of
  `expand-shipped-in` is now decided (yes — D-07 revision); the grammar itself is still open.
- Whether the `changes` prelude, guard, and N-1 boot job are three jobs or the guard folds
  into `changes` (D-16). The N-1 boot job stays separate (it carries `services: postgres`).
- The `changes` job's diff mechanism (`dorny/paths-filter` SHA-pinned vs. a plain
  `git diff --name-only` step) (D-16).
- How D-15 extracts referenced `(table, column)` identifiers from `queries/*.sql` and where
  its blind spots are (RESEARCH.md scopes this).
- Health-poll timing (attempt count, interval, sustained-green window) for the N-1 boot
  check (D-03).
- Whether `cmd/migration-check` needs a `.golangci.yml` `gosec` carve-out for its file
  reads (mirrors the `cmd/coverage-report` G304 carve-out from Phase 15 D-19 - likely, but
  confirm).
- Whether `cmd/migration-check` is excluded from the Makefile `COVER_PKGS` denominator like
  `cmd/coverage-report` is (Phase 15 D-07 precedent).
- Exact failure-message wording (must name the expand/contract rule and point at the
  README).
- Whether the two new jobs are one combined job or two separate jobs in the workflow.

</decisions>

<revisions>
## Revisions - 2026-09-04 (post-context grill)

A `/grill-with-docs` pass surfaced eight findings (S1-S8). These **amend** the decisions
they cite; where a revision conflicts with the original wording, the revision wins. New
decisions D-15..D-19 carry mechanisms that did not exist in the original context.

### S1 - N-1 boot check was read-only; poller write paths unverified -> D-03 amended, **D-15 (new)**
The app's core job is the **poller** writing to `events` (`queries/events.sql` `InsertEvent`
- 14 named columns, plus `SELECT *` / `RETURNING *` reads) and `artists`
(`queries/artists.sql` `UpsertArtist`, `RecordArtMatchAttempt`). A contract migration that
drops/renames a column only the poller's INSERT names would pass a GET-only check and then
fail silently in a background goroutine after rollback.

- **D-03 amended:** after the `/health` + `GET /watchlist` + `GET /events` assertions, the
  boot job does **`POST /watchlist`** (minimal body) against the N-1 image, asserts 2xx,
  then re-reads `GET /watchlist` and asserts the row is present - exercising the old
  binary's `watchlist.Add` + `UpsertArtist` writes with no fixtures. Planner: confirm the
  add path does not require a live MusicBrainz/Deezer call; if it does, rely on D-15 alone
  for write coverage and note it.
- **D-15 (new) - static guard cross-references the previous release's query SQL.**
  `cmd/migration-check`, in addition to the D-08 pattern scan, resolves the previous
  release tag (the same `svu current` value as D-13/D-17), reads that tag's `queries/*.sql`
  via `git show <tag>:queries/<f>.sql`, and builds the set of `(table, column)` identifiers
  those queries reference - treating `SELECT *` / `RETURNING *` from a table as "references
  every column". For every `DROP COLUMN`, `RENAME COLUMN`, `DROP TABLE`, `RENAME TABLE` in a
  branch-new migration, the guard goes **red if the object is still referenced by the
  previous release's queries - regardless of the `allow-destructive` annotation** (the
  annotation documents intent; it cannot make a live N-1 break safe). This is the
  deterministic every-run counterpart to the boot job's behavioural check and it covers the
  poller's `events`/`artists` writes because those live in `queries/*.sql`. No prior tag
  (true bootstrap, D-04) -> sub-check skipped. Prior tag exists but `git show` fails -> red.
  `sqlc generate` already rejects a *current-branch* query referencing a dropped column, so
  this check is strictly about the **previous** release.
  - Scope note: largest single addition to `cmd/migration-check`. RESEARCH.md scopes the
    identifier extraction (`INSERT (...)` column lists, `SELECT col, a.col`, `RETURNING *`,
    `sqlc.narg('col')`) and its blind spots. Type-narrowing / `SET NOT NULL` on an existing
    column stays the boot job's backstop (D-08).

### S2 - "direct push to main" guard was a no-op -> D-07/D-12 amended, **D-16 (new)**
Diffing new migrations against the `main` merge-base "in all cases" yields an empty diff on
a push *to* `main` (`origin/main` already contains the commit), so the guard could never
catch a destructive migration pushed straight to `main` - the case D-12 cited to justify
running on every push.

- **D-16 (new) - a `changes` prelude job computes the diff base once.** A tiny `changes`
  job (no services, `fetch-depth: 0`, fetch tags) outputs the **new-migration file list**
  and a boolean `migrations_changed`:
  - `pull_request` -> diff `origin/${{ github.base_ref }}...HEAD` (merge-base).
  - `push` -> diff `${{ github.event.before }} ${{ github.sha }}` (the actual pushed range;
    correct for direct-to-`main`). `github.event.before` all-zeroes (new branch) -> fall
    back to merge-base against `origin/main`.
- **D-07/D-12 amended:** guard and boot job consume `changes` outputs; neither recomputes a
  diff base. "Merge-base in all cases" withdrawn.

### S3 - `RunMigrations` could not be tested against an ahead-of-source schema -> D-02 amended, **D-18 (new)**
`RunMigrations` hard-codes `iofs.New(migrationsFS, "migrations")`, so "run `RunMigrations`
with a truncated source" was impossible and the test would have exercised raw `migrate.Up()`
(library behaviour), not our boot path.

- **D-18 (new) - unexported source seam.** Add
  `func runMigrationsWithSource(ctx context.Context, dsn string, logger *slog.Logger, src source.Driver, opts ...RetryOption) error`
  holding the current `RunMigrations` body from the `newRetryConfig` line onward.
  `RunMigrations` keeps its exact exported signature and becomes: build the embedded `iofs`
  source, `return runMigrationsWithSource(...)`. No new exported symbol; `migrationsFS`
  stays unexported. (The `iofs.New` error path moves into `RunMigrations`'s wrapper.)
- **D-02 amended:** the SC #4 test (`package db`, in-package) builds a truncated source over
  a **synthetic** migration set (nothing exists past `000007`: write `1..N` and `1..N+1`
  into two `fstest.MapFS`/temp dirs), migrates a real throwaway DB to `N+1` with the full
  set, then calls **`runMigrationsWithSource` with the `1..N` source** and asserts `nil`.
  It now guards our code, not just golang-migrate.
- **D-02 caveat:** this is the research's one MEDIUM-confidence item. **Plan it as task 1
  and treat it as a checkpoint** - if `iofs`/`migrate.Up()` errors rather than returning
  `ErrNoChange` against an ahead-of-source DB, stop and escalate: the rollback-safety model
  and Phase 17's auto-rollback depend on it and there is no plan B here. Related: a
  `dirty`-`schema_migrations` state (half-applied forward migration) also blocks the old
  binary - out of scope, Phase 17 restore territory, noted so the "no-ops cleanly" framing
  is not over-trusted.

### S4 - `ADD COLUMN ... NOT NULL` (no default) is not an N-1 problem -> D-08/D-10 amended
Keeping the check (per direction), but it is a **forward-migration hazard** (fails or
table-rewrites on a non-empty table - Pitfall 9), not an N-1 break: the old binary never
selects an unknown column.

- **D-08 amended:** two finding **classes**, different messages:
  1. **backward-incompatible (N-1 break):** `DROP COLUMN`, `DROP TABLE`, `RENAME`
     (column/table), type-narrowing `ALTER COLUMN ... TYPE`. Message names the
     expand/contract / N-1 rule + README. Overridable by `allow-destructive` **and**
     subject to the D-15 cross-reference (which the annotation cannot override).
  2. **unsafe-forward (deploy hazard):** `ADD COLUMN ... NOT NULL` without `DEFAULT`.
     Message explains the non-empty-table failure/lock + README's "adding a NOT NULL
     column" note. Overridable by the same annotation (author asserts table empty / lock
     acceptable).
- **D-10 amended:** README states the two rules separately - *backward-incompatible*
  (breaks rollback) vs *unsafe-forward* (breaks/locks the deploy) - not one umbrella.

### S5 - only one rollback hop verified; annotation was self-reported -> D-13 amended, **D-17 (new)**
- **D-17 (new) - rollback is strictly N-1.** Phase 17's auto-rollback MUST target exactly
  the immediately-previous release tag, never N-2 or earlier. Recorded here as the
  invariant Phase 16's checks assume; **locked in Phase 17's discuss/spec.** Given it,
  "expand shipped in the immediately-previous release" is a sufficient safety condition and
  D-15's cross-reference against that one tag is exact.
- **D-13 amended:** `expand-shipped-in=vX.Y.0` is **validated** - the guard rejects a value
  that is not an existing tag, and warns (not red) if it is older than `svu current`.

### S6 - N-1 boot job cost/signal asymmetry -> D-11/D-12 amended
- **D-12 amended:** the **boot job** runs its real steps only when
  `needs.changes.outputs.migrations_changed == 'true'` (job-level `if:`). It stays in
  `build-scan.needs:`; skipped == success, gate intact. No-op on the ~95% of pushes that
  touch no migration. `changes` + static guard stay unconditional (cheap).
- **D-11 amended:** blocking unchanged; flakiness now also bounded by the path filter.

### S7 - blocking gate ships one phase before the capability it guards -> D-11 amended (accepted risk)
Both checks stay blocking from Phase 16 (per direction + milestone rationale + the repo
already shipped the "non-blocking for a whole phase then forgot to wire it" defect once,
Phase 8->9). Residual risk explicitly accepted: a transient ghcr.io failure on a
migration-changing PR (rare post-S6) blocks that one PR; mitigation is the D-04 retry + a
manual re-run. No report-only interim.

### S8 - factual slips / unstated requirements -> D-13/D-14 amended, **D-19 (new)**
- **D-14 amended:** env var is **`HTTP_PORT`** (`envDefault:"8080"`), not `PORT`. Only
  `DATABASE_URL` is `notEmpty` in `internal/config`, so the N-1 image boots with just
  `DATABASE_URL` + `HTTP_PORT` + no `INSTANCE_PASSPHRASE`; `DISCORD_WEBHOOK_URL` optional.
- **D-13/D-16 amended:** every new job resolving a tag or diffing history checks out
  `fetch-depth: 0` **and fetches tags** - hard requirement.
- **D-19 (new) - housekeeping.** (a) D-04's skip-green-on-true-bootstrap is an **in-job
  step** (detect "no prior tag", exit 0 with a notice), not a job-level `if:`; it is
  unreachable on this repo (v1.x tags exist) - accepted defensive dead code, no
  `workflow_dispatch` override. (b) `cmd/migration-check` gets the `gosec` G304 carve-out
  and `COVER_PKGS` exclusion (Phase 15 D-19/D-07 precedent) - confirmed, not "likely".
  (c) `GET /watchlist` `200 []` and `GET /events` `200 {events:[]}` on an empty DB are
  verified in code - the D-03 / canonical-refs planner TODO is closed.

</revisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Requirements & roadmap
- `.planning/ROADMAP.md` §"Phase 16: Rollback-Safe Migrations" - goal, the 4 success
  criteria, and the _Notes_ (split-from-deploy rationale, "CI-only, no VPS required",
  pitfalls 8/9/10)
- `.planning/ROADMAP.md` §"Phase 17" _Notes_ - the "Postgres backup + restore procedure ...
  Schedule this as part of Phase 16 or 17" line (this phase defers it to 17), and the
  N-1-rollback dependency Phase 17 places on this phase
- `.planning/REQUIREMENTS.md` - MGRT-01, MGRT-02 (locked text)

### v1.3 research (this milestone)
- `.planning/research/PITFALLS.md` **Pitfall 8** (non-backward-compatible migration bricks
  the rollback - the milestone's highest-cost failure; the "How to avoid" list is the
  checklist this phase implements), **Pitfall 9** (boot-migration failure -> crash loop),
  **Pitfall 10** (migration lock contention / advisory lock)
- `.planning/research/ARCHITECTURE.md` §"Boot-time migrations x rollback - the
  expand/contract constraint" - the golang-migrate `Up()`-against-ahead-schema behavior
  (source returns `os.ErrNotExist` -> `ErrNoChange`, MEDIUM confidence, the thing D-02
  pins down), the additive-only rule, the `000007` precedent, and the recommended phase
  test ("apply migrations 1..N+1, then call the N-migration build's `RunMigrations` and
  assert `nil`")
- `.planning/research/ARCHITECTURE.md` §"NEW vs. MODIFIED - Feature 1" and §"Boot-time
  migration must be idempotent and forward-only"
- `.planning/research/FEATURES.md` §"Feature 1" - the "Backward-compatible (expand/contract)
  migrations" table row (MEDIUM complexity, "discipline constraint on every future
  migration, not code. Add a checklist / ADR")
- `.planning/research/STACK.md` §"Feature 1" - deploy/rollback structure context (why N-1
  schema compatibility matters), the `:latest` vs pinned-tag reasoning

### Prior-phase decisions this phase mirrors
- `.planning/phases/15-pr-coverage-diff-comment/15-CONTEXT.md` - D-05/D-06 (a small tested
  Go `cmd/` tool instead of an inline shell blob or a third-party action), D-07
  (`COVER_PKGS` exclusion for a CI helper), D-19 (`gosec` G304 carve-out for a tool that
  reads file paths from argv), and the report-only-job pattern (which D-11 deliberately
  does *not* follow - these checks block)
- `.planning/phases/14-instance-passphrase-gate/14-CONTEXT.md` - GATE-07 inert path
  (`INSTANCE_PASSPHRASE` unset => routes flat, no auth) - why the N-1 boot check can hit
  `/watchlist` and `/events` unauthenticated (D-03/D-14)

### Existing code this phase modifies or depends on
- `internal/db/migrate.go` - `RunMigrations` (embedded `//go:embed migrations/*.sql` +
  `iofs`, bounded retry, `migrate.ErrNoChange` treated as success, `Up()` only, no
  context-aware `Up` so it runs on a goroutine). D-01's helper reuses this; D-18 extracts
  an unexported `runMigrationsWithSource` seam from it; D-02's test drives that seam with a
  truncated source
- `internal/db/migrations/*.sql` - 7 migrations today (`000001`..`000007`); `000007` is the
  expand-style backfill precedent D-10 cites; `*.down.sql` files exist but the app never
  runs them
- `queries/*.sql` - `artists.sql`, `events.sql`, `health.sql`, `watchlist.sql` (repo root;
  `sqlc.yaml` `queries:`). The source of truth for which columns the shipped binary
  reads/writes - **D-15 reads these at the previous release tag** via `git show`.
  `events.sql` `InsertEvent` names 14 columns + `SELECT *`/`RETURNING *`; `artists.sql`
  uses `RETURNING *` / `SELECT a.*`
- `internal/httpserver/health.go` - `GET /health` pings the DB only (3s timeout), returns
  200 `{status:ok,db:up}` or 503 - does **not** check migrations-applied or schema shape
  (why D-03 adds real read + write endpoints)
- `internal/httpserver` watchlist + events handlers - `GET /watchlist` (`200 []` on empty),
  `GET /events` (`200 {events:[]}` on empty) and `POST /watchlist` (the write path D-03
  adds per S1) - all verified in `watchlist.go` / `events.go`
- `internal/config/config.go` - only `DATABASE_URL` is `notEmpty`; `HTTP_PORT`
  (`envDefault:"8080"`) is the port var (not `PORT`); `DISCORD_WEBHOOK_URL` optional -
  governs the N-1 image's env in D-14
- `.github/workflows/full-pipeline.yml` - `build-scan.needs: [vet, lint, test, gitleaks,
  trivy-fs, frontend-test]` (D-11 appends the guard + boot job), `release` job's `svu` usage
  + `fetch-depth: 0` checkout (D-13/D-16 mirror), `concurrency` block with
  `cancel-in-progress` only on PRs, the SHA-pinned-actions + trailing-version-comment
  convention, `test` job's `services`/`make db-up` Postgres pattern. `github.event.before`
  is the push diff base D-16 uses
- `Makefile` - `test-integration` -> `db-up` (`docker compose up -d --wait postgres`),
  `TEST_DATABASE_URL` default, `COVER_PKGS` anchored `grep -vE` (D-07 precedent),
  `coverage-gate`
- `CLAUDE.md` §"Definition of Done - run before every commit" - where D-09's one-line
  pointer to the migrations README goes
- `docker-compose.yml` - the `postgres` service definition (referenced, not modified - D-14
  uses a GH Actions service instead)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`internal/db.RunMigrations`** - directly reused by D-01's `go run` helper to apply
  HEAD schema; the same function under test in D-02. Exported, takes `(ctx, dsn, logger,
  ...RetryOption)`.
- **`cmd/coverage-report/`** (Phase 15) - the structural template for `cmd/migration-check`:
  a small `main` package, unit-tested, kept out of the product coverage denominator, with
  a scoped `gosec` carve-out for reading file paths from argv/env.
- **`release` job's `fetch-depth: 0` + `go install .../svu` + `svu current`** - copy this
  for D-13's N-1 tag resolution.
- **`test` job's Postgres bring-up** (`make db-up` / a service) and `TEST_DATABASE_URL`
  wiring - the model for D-14's `services: postgres`.
- **`000007_backfill_events_watched_artist_name`** - a real in-tree expand/backfill
  migration D-10's README walkthrough is built around.

### Established Patterns
- **CI tooling is small tested Go `cmd/` packages, not inline shell** (Phase 15 D-06) -
  `cmd/migration-check` follows.
- **Third-party actions are SHA-pinned with a trailing `# vX.Y.Z` comment** - any new
  `uses:` (e.g. a Postgres service is a container image, not an action, but if `svu` or a
  curl-wait helper action is added, pin it).
- **Top-level `permissions: contents: read`; jobs opt into more** - the new jobs need only
  `contents: read` (public ghcr.io package, no registry auth, no PR write).
- **CI helper packages are excluded from `COVER_PKGS`** (Phase 15 D-07) but still compile
  under `vet`/`lint` and run their own `_test.go`.
- **Migrations are forward-only, additive, embedded** - this phase codifies the discipline
  that was already the informal practice (PROJECT.md "migrations must stay
  backward-compatible (expand/contract)").

### Integration Points
- `.github/workflows/full-pipeline.yml` - a `changes` prelude job (D-16), a guard job, and
  an N-1 boot job. Guard + boot appended to `build-scan.needs:` (D-11); each is
  `contents: read` only; `fetch-depth: 0` + fetch tags on every job that resolves a tag or
  diffs history; `services: postgres` on the boot job; boot job carries
  `if: needs.changes.outputs.migrations_changed == 'true'` (D-12/S6).
- `cmd/migration-check/` - new Go `main` + `_test.go`; `.golangci.yml` `gosec` G304
  carve-out and `COVER_PKGS` exclusion (D-19, confirmed). Does the D-08 pattern scan **and**
  the D-15 previous-release query cross-reference (shells `git show`).
- `internal/db/migrate.go` - **modified**: extract unexported `runMigrationsWithSource`
  seam (D-18); `RunMigrations` signature unchanged.
- A `go run` migration helper (name/shape at planner discretion) invoked from the boot job.
- `internal/db/*_test.go` - new test for D-02 (SC #4), drives the D-18 seam with a
  synthetic truncated source.
- `internal/db/migrations/README.md` - new file (D-09/D-10), two-rule structure (S4).
- `CLAUDE.md` - one-line pointer added to the Definition-of-Done section (D-09).

</code_context>

<specifics>
## Specific Ideas

- The N-1 boot check must exercise **real read endpoints**, not just `/health` - the
  operator's mental model is "rollback = redeploy old image" and the failure this phase
  exists to catch is the old binary's *queries* breaking against a contracted schema,
  which `/health` (a bare DB ping) cannot see.
- The static guard's failure message is a teaching moment - it must **name the
  expand/contract rule and point at `internal/db/migrations/README.md`**, not just say
  "destructive DDL found".
- The `allow-destructive` annotation is the deliberate, reviewable way to ship a genuine
  contract-phase migration - it forces the author to state *which release the expand half
  shipped in*. Post-S5 this is **machine-verified**: the guard validates the tag exists and
  (D-15) reds the build anyway if that previous release's `queries/*.sql` still touch the
  column, so the annotation cannot wave through a live N-1 break.
- Keep the new surface minimal, matching Phase 15's restraint: no new secret, no new
  registry auth, no docker-compose artifact, no `docs/adr/` convention - one new `cmd/`
  tool, one helper, one test, one README, workflow edits.

</specifics>

<deferred>
## Deferred Ideas

- **Postgres backup + restore procedure** (ROADMAP Phase 17 _Notes_ folds in OPS-04
  basics; flagged for "Phase 16 or 17"). Deferred to Phase 17's provisioning runbook -
  it's the recovery path when an image rollback *also* needs the schema restored, and it
  belongs with the VPS work, not this CI-only phase.
- **Machine-enforced "no blocking DDL"** (non-`CONCURRENTLY` `CREATE INDEX`,
  table-rewriting `ALTER`) - D-08 keeps this a written rule only. Revisit if the app ever
  grows a large table or scales past one instance (Pitfall 10).
- **`down` migration authoring + a `migrate down` rollback path** - out of scope; the app
  is forward-only by design and rollback is image-swap + N-1 compatibility. Only reconsider
  if a true schema-rollback capability is ever wanted.
- **A `docs/adr/` ADR for the expand/contract decision** - considered as the doc home,
  rejected (D-09) to avoid introducing the ADR convention now. If `docs/adr/` is
  established later for another reason, cross-link the migrations README to it.
- **Poller leader-election via Postgres advisory lock** (Pitfall 10 / CLAUDE.md's noted
  future fix) - same problem class as migration lock contention, but a scaling concern the
  single-instance design doesn't have yet.

### Reviewed Todos (not folded)
None - `todo.match-phase 16` returned no matches.

</deferred>

---

*Phase: 16-rollback-safe-migrations*
*Context gathered: 2026-09-04*
*Context revised: 2026-09-04 (grill findings S1-S8; new decisions D-15..D-19)*
