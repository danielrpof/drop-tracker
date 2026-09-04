# Phase 16: Rollback-Safe Migrations - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-04
**Phase:** 16-rollback-safe-migrations
**Areas discussed:** HEAD-schema setup, Destructive-migration detection, Rule documentation home, CI job shape

---

## HEAD-schema setup

### Q1 — bringing the throwaway Postgres to the current branch's schema

| Option | Description | Selected |
|--------|-------------|----------|
| go-run RunMigrations helper | Tiny `go run` helper calls `internal/db.RunMigrations` against a Postgres service container — same embedded-FS migration code the app boots with, no Docker build, ~seconds | ✓ |
| Build & boot current image | `docker build` the branch, boot it once to self-migrate through the real entrypoint, then run the N-1 image against the same DB — highest fidelity, adds a full image build | |
| golang-migrate CLI | Run the CLI directly over `internal/db/migrations/*.sql` — simple, but a third migration code path the app never uses | |

**User's choice:** go-run RunMigrations helper
**Notes:** Fidelity of the current image's own boot is already covered by `build-scan` and the `test` job.

### Q2 — proving SC #4 (older binary no-ops against ahead-of-source schema)

| Option | Description | Selected |
|--------|-------------|----------|
| Dedicated Go test + CI job as bonus | Hermetic `internal/db` test (migrate to N+1 with full set, run `RunMigrations` with `iofs` truncated to 1..N, assert nil); runs every CI run; N-1 boot job is real-world confirmation on top | ✓ |
| CI boot job only, made deterministic | No separate test; boot job always applies a synthetic migration beyond HEAD so the N-1 image always boots ahead | |
| Go test only | The hermetic test is the whole answer; boot job doesn't specifically target ahead-of-source | |

**User's choice:** Dedicated Go test + CI job as bonus
**Notes:** The deterministic every-run guarantee needs to live in the Go test, not depend on whether a PR happens to add a migration.

### Q3 — what the N-1 boot check asserts as "starts and stays healthy"

| Option | Description | Selected |
|--------|-------------|----------|
| /health + real read endpoints | Poll `/health` to 200 sustained, then `GET /watchlist` + `GET /events` must 200 (handlers run representative sqlc queries) so schema incompatibility 500s | ✓ |
| /health only | Process-up + DB-reachable only; misses query-level schema incompatibility that doesn't crash startup | |
| Full seeded smoke flow | Seed a watchlist entry via API, verify reads across watchlist + events + a search proxy call; most thorough, needs fixtures + stubbed external API | |

**User's choice:** /health + real read endpoints
**Notes:** `/health` is a bare DB ping; the failure this phase catches is the old binary's queries breaking against a contracted schema.

### Q4 — behavior when the previous released image can't be fetched

| Option | Description | Selected |
|--------|-------------|----------|
| Skip-green only if no prior tag exists; fail otherwise | No prior release tag at all → skip with a logged notice, green. Prior tag exists but pull fails → red (don't silently mask an unverifiable rollback) | ✓ |
| Always skip-green on any fetch failure | Mirror Phase 15's no-baseline handling; never blocks on image-availability problems | |
| Always red if N-1 image unavailable | Treat inability to verify as failure including the first release (needs a one-time override) | |

**User's choice:** Skip-green only if no prior tag exists; fail otherwise

---

## Destructive-migration detection

### Q1 — how to cover SC #2 (RED check naming the N-1 rule)

| Option | Description | Selected |
|--------|-------------|----------|
| Static SQL guard as SC #2's answer + boot job for MGRT-01 | Dedicated static scan of changed `*.up.sql` fails fast with the rule text + file/line; N-1 boot job stays the MGRT-01 mechanism; complementary | ✓ |
| Behavioral boot job only | No static scan; wrap any N-1 boot failure in a rule-pointing message; misses destructive-but-non-crashing migrations | |
| Static guard only | Pattern scan is the whole phase; no N-1 image booted; weakens MGRT-01 / SC #1 | |

**User's choice:** Static SQL guard as SC #2's answer + boot job for MGRT-01

### Q2 — how the static guard is implemented

| Option | Description | Selected |
|--------|-------------|----------|
| Small tested Go tool (cmd/migration-check) | Stdlib-only, unit-tested, no new dependency, mirrors `cmd/coverage-report` | ✓ |
| squawk | Off-the-shelf Postgres migration linter; more coverage but adds a dev-tool dependency + config, diverges from hand-roll pattern | |
| Inline bash + grep in the workflow | Zero new files; untested brittle regex in YAML — the anti-pattern Phase 15 moved away from | |

**User's choice:** Small tested Go tool (cmd/migration-check)

### Q3 — guard scope + escape hatch for legitimate contract migrations

| Option | Description | Selected |
|--------|-------------|----------|
| Diff-scoped + inline annotation | Scan only migrations added vs the main merge-base; destructive → RED unless an explicit `-- migration-check:allow-destructive expand-shipped-in=… reason=…` annotation is present; self-documenting | ✓ |
| Diff-scoped, no escape hatch | Any destructive statement always RED; a real contract migration needs an out-of-band override | |
| Scan-all + allowlist file | Scan every `*.up.sql`; approved-destructive migrations listed by filename; approval reason lives away from the migration, list grows forever | |

**User's choice:** Diff-scoped + inline annotation

### Q4 — also enforce "no blocking DDL" from SC #3's rule?

| Option | Description | Selected |
|--------|-------------|----------|
| Destructive set only (RED); blocking-DDL stays documented | Guard flags exactly DROP COLUMN/TABLE, RENAME, type-narrowing ALTER, ADD COLUMN NOT NULL without DEFAULT; "no blocking DDL" remains a written rule (low risk at drop-tracker's data scale) | ✓ |
| Also flag blocking DDL as RED | Add non-CONCURRENTLY CREATE INDEX + table-rewriting ALTERs; more false positives + the CONCURRENTLY-vs-transaction wrinkle | |
| Destructive = RED, blocking DDL = WARN | Destructive fails; blocking-DDL prints a non-failing warning | |

**User's choice:** Destructive set only (RED); blocking-DDL stays documented

---

## Rule documentation home

### Q1 — primary home for the expand/contract rule

| Option | Description | Selected |
|--------|-------------|----------|
| internal/db/migrations/README.md (+ pointers) | Rule lives next to the `.sql` files; one-line pointers from CLAUDE.md's Definition-of-Done and the guard's failure message; no `docs/adr/` introduced | ✓ |
| New docs/adr/ ADR (+ pointers) | Establish `docs/adr/` with this as ADR-0001; more ceremony; introduces the deferred ADR convention | |
| CLAUDE.md Definition-of-Done block | Add expand/contract as a commit gate; already dense; the rule is DDL-authoring-specific, not every-commit | |

**User's choice:** internal/db/migrations/README.md (+ pointers)

### Q2 — README depth

| Option | Description | Selected |
|--------|-------------|----------|
| Rule + worked example + checklist + guard reference | Rule stated plainly; expand→backfill→contract walkthrough citing the `000007` precedent; copy-paste pre-merge checklist; section on what `cmd/migration-check` enforces + the annotation syntax | ✓ |
| Concise rule + links out | Tight rule statement linking to PITFALLS.md / ARCHITECTURE.md for rationale | |
| Full migration authoring guide | Everything above plus golang-migrate mechanics, naming conventions, local testing, down-file policy | |

**User's choice:** Rule + worked example + checklist + guard reference

---

## CI job shape

### Q1 — do the checks block a merge?

| Option | Description | Selected |
|--------|-------------|----------|
| Both blocking (in build-scan needs:) | Static guard + N-1 boot job both gate the release path like vet/lint/test/frontend-test; matches the milestone rationale; flakiness bounded by skip-green-only-if-no-prior-tag + a pull retry | ✓ |
| Static guard blocks; boot job advisory | Guard gates; boot job goes red visibly but isn't in any needs: graph (coverage-comment pattern); keeps registry flakiness from blocking merges, costs MGRT-01's teeth | |
| Both advisory | Neither gates; rely on the red X + reviewer discipline; contradicts SC #2 | |

**User's choice:** Both blocking (in build-scan needs:)

### Q2 — when do the checks run?

| Option | Description | Selected |
|--------|-------------|----------|
| Every push + PR (same as other gates) | Both checks sit in `build-scan.needs:` and run wherever build-scan runs; redundant post-merge run is cheap insurance and catches a direct-to-main push | ✓ |
| PR only | `if: github.event_name == 'pull_request'`; lighter, but can't be a plain `build-scan.needs:` gate then | |
| PR + push to main | Runs on PRs and post-merge pushes, not feature-branch pushes; slightly more complex `if:` | |

**User's choice:** Every push + PR (same as other gates)

### Q3 — how the previous released image is resolved

| Option | Description | Selected |
|--------|-------------|----------|
| svu current on a full-history checkout | `fetch-depth: 0`, `svu current` → latest release tag → pull that ghcr.io tag; same tool the release job uses; no prior tag → skip-green | ✓ |
| Query ghcr.io for the newest tag | Registry/packages API call; adds token handling; can disagree with git tags on a half-failed push | |
| Committed rollback-floor file | A repo file names the minimum-supported rollback version, bumped deliberately; manual step that drifts | |

**User's choice:** svu current on a full-history checkout

### Q4 — how the N-1 boot job wires up Postgres and runs the old image

| Option | Description | Selected |
|--------|-------------|----------|
| GH Actions services: postgres + docker run --network host | Postgres service on the runner; go-run helper applies HEAD schema at localhost:5432; `docker run --network host` the N-1 image with DATABASE_URL + PORT; curl localhost:PORT; no compose changes | ✓ |
| Dedicated compose file for the check | `deploy/compose.rollback-check.yaml` with a parameterized app image + postgres + healthcheck; reuses compose semantics, foreshadows Phase 17, adds an artifact now | |
| Reuse make db-up compose postgres | Bring up just the compose postgres, then `docker run` the N-1 image on that network; mixes compose + raw docker run | |

**User's choice:** GH Actions services: postgres + docker run --network host

---

## Claude's Discretion

- Exact names for `cmd/migration-check` and the `go run` migration helper, and the helper's shape (`cmd/` package vs. build-tagged helper vs. test-mode flag).
- The precise `allow-destructive` annotation grammar and whether `expand-shipped-in` is validated against existing tags.
- Health-poll timing (attempts, interval, sustained-green window) for the N-1 boot check.
- Whether `cmd/migration-check` needs a `.golangci.yml` `gosec` G304 carve-out (Phase 15 D-19 precedent) and whether it's excluded from `COVER_PKGS` (Phase 15 D-07 precedent).
- Exact failure-message wording (must name the expand/contract rule and point at the README).
- Whether the two new checks are one combined job or two separate jobs.

## Deferred Ideas

- Postgres backup + restore procedure — deferred to Phase 17's provisioning runbook (CI-only phase, no real DB).
- Machine-enforced "no blocking DDL" — stays a written rule; revisit at larger data scale.
- `down` migration authoring / a `migrate down` rollback path — out of scope; app is forward-only by design.
- A `docs/adr/` ADR for the expand/contract decision — rejected to avoid introducing the ADR convention now.
- Poller leader-election via Postgres advisory lock (Pitfall 10 / CLAUDE.md future fix) — same problem class, a scaling concern the single-instance design doesn't have yet.
