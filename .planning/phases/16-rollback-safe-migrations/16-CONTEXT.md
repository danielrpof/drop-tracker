# Phase 16: Rollback-Safe Migrations - Context

**Gathered:** 2026-09-04
**Status:** Ready for planning

<domain>
## Phase Boundary

A **CI-only** guardrail plus a written rule. No VPS, no deploy job, no change to the
application's runtime behavior (the only Go added is CI-helper `cmd/` tooling and test
code). Delivers:

1. **MGRT-01** - a CI check that boots the **previously-released** image against a database
   migrated to the **current branch's** schema and passes only if that older binary starts
   and stays healthy (N-1 backward-compatibility check).
2. A **static SQL guard** that turns a branch red when a new migration carries a
   rollback-breaking destructive change, with a failure message naming the expand/contract
   rule (SC #2).
3. A hermetic test closing the research's one MEDIUM-confidence assumption: the older
   binary's boot migration **no-ops** against an ahead-of-source `schema_migrations`
   version rather than erroring (SC #4).
4. **MGRT-02** - the expand/contract rule written down where a migration author will meet
   it (SC #3).

**In scope:** MGRT-01, MGRT-02. Edits `.github/workflows/full-pipeline.yml`; adds
`cmd/migration-check/`, a `go run` migration helper, an `internal/db` test, and
`internal/db/migrations/README.md`; small pointer edits to `CLAUDE.md`.

**Out of scope:**
- The VPS deploy job and auto-rollback control flow (Phase 17, DPLY-01).
- The Postgres backup + restore procedure (ROADMAP flags it for Phase 16 *or* 17 - deferred
  to Phase 17's provisioning runbook; this phase is CI-only and never touches a real DB).
- Enforcing "no blocking DDL in boot migrations" as a machine check - it stays a written
  rule only (see D-08).
- Writing/testing `down` migrations as a rollback mechanism - the app only ever runs `Up()`;
  rollback is image-swap + N-1 schema compatibility, not `migrate down`.
- Any `docs/adr/` convention (not introduced here - see D-09).

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
- **D-03:** The N-1 boot check asserts "starts and stays healthy" as: poll `/health` to a
  200 for a short **sustained** window, **then** `GET /watchlist` and `GET /events` must
  each return 200. Those handlers run representative sqlc queries, so a contracting
  migration that breaks the old binary's read paths surfaces as a 500 rather than passing
  silently. `/health` alone only pings the DB and would miss query-level schema
  incompatibility. The N-1 image runs with **no `INSTANCE_PASSPHRASE`** so those routes are
  reachable unauthenticated (Phase 14 GATE-07 inert path).
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
- **D-12:** The checks run on **every push and every PR** - a natural consequence of
  sitting in `build-scan.needs:` (which itself runs on `push` and `pull_request`). The
  redundant post-merge run on `main` is cheap insurance and is the only gate if someone
  pushes directly to `main`. The static guard diffs against the `main` merge-base in all
  cases (feature-branch push, PR, and post-merge).
- **D-13:** The previous released image is resolved with **`svu current`** on a
  `fetch-depth: 0` checkout -> `ghcr.io/danielrpof/drop-tracker:<tag>`. Same tool the
  `release` job uses; on a PR this tag is exactly what a rollback would land on. Not a
  ghcr.io API query (extra token handling, can disagree with git tags on a half-failed
  push). Not a committed rollback-floor file (manual step that drifts).
- **D-14:** Boot wiring: a **GitHub Actions `services: postgres`** container on the runner;
  the D-01 `go run` helper applies HEAD schema against `localhost:5432`; then
  `docker run --network host` the N-1 image with `DATABASE_URL` + `PORT` (and no
  `INSTANCE_PASSPHRASE`) in its env; `curl localhost:$PORT` for the D-03 assertions. No
  docker-compose file or image-parameterization is added now (that's Phase 17's prod
  compose work).

### Claude's Discretion
- Exact names for the new `cmd/` tool (`cmd/migration-check` is the working name) and the
  `go run` migration helper (could be a `cmd/` package or a `//go:build tools`-style
  helper or a test-mode flag on an existing binary).
- The precise `allow-destructive` annotation grammar (D-07) and whether `expand-shipped-in`
  is tag-validated.
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
  context-aware `Up` so it runs on a goroutine). D-01's helper reuses this; D-02's test
  exercises the `ErrNoChange` path against a truncated source
- `internal/db/migrations/*.sql` - 7 migrations today (`000001`..`000007`); `000007` is the
  expand-style backfill precedent D-10 cites; `*.down.sql` files exist but the app never
  runs them
- `internal/httpserver/health.go` - `GET /health` pings the DB only (3s timeout), returns
  200 `{status:ok,db:up}` or 503 - does **not** check migrations-applied or schema shape
  (why D-03 adds real read endpoints)
- `internal/httpserver` watchlist + events handlers - the `GET /watchlist` / `GET /events`
  read paths D-03 exercises (planner: confirm both return 200 on an empty DB)
- `.github/workflows/full-pipeline.yml` - `build-scan.needs: [vet, lint, test, gitleaks,
  trivy-fs, frontend-test]` (D-11 appends to this), `release` job's `svu` usage +
  `fetch-depth: 0` checkout (D-13 mirrors), the SHA-pinned-actions + trailing-version-comment
  convention, `test` job's `services`/`make db-up` Postgres pattern
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
- `.github/workflows/full-pipeline.yml` - two new jobs (or one combined), both appended to
  `build-scan.needs:` (D-11); each is `contents: read` only; a `services: postgres` block
  on the boot job.
- `cmd/migration-check/` - new Go `main` + `_test.go`; likely `.golangci.yml` `gosec`
  carve-out; likely `COVER_PKGS` exclusion.
- A `go run` migration helper (name/shape at planner discretion) invoked from the boot job.
- `internal/db/*_test.go` - new test for D-02 (SC #4).
- `internal/db/migrations/README.md` - new file (D-09/D-10).
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
  contract-phase migration - it should force the author to state *which release the expand
  half shipped in*, so a reviewer can confirm the rollback floor has moved past it.
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
