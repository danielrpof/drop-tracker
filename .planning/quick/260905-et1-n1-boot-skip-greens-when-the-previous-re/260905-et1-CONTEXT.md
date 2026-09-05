# Quick Task 260905-et1: n1-boot skip-greens when the previous release image predates the ahead-of-source migration guard (Phase 16 gap G-16-1) - Context

**Gathered:** 2026-09-05
**Status:** Ready for planning

<domain>
## Task Boundary

Fix Phase 16 UAT gap G-16-1. The `n1-boot` job in `.github/workflows/full-pipeline.yml`
reds on *every* migration-touching branch — a destructive `DROP COLUMN` and a purely
additive nullable `ADD COLUMN` fail it identically — because it boots
`ghcr.io/danielrpof/drop-tracker:$PREV_TAG` (currently `v1.7.0`) and runs *that* binary's
embedded `RunMigrations` against the HEAD schema. `v1.7.0` was built before Phase 16's
ahead-of-source no-op guard (`maxSourceVersion` / `runMigrationsWithSource` in
`internal/db/migrate.go:213-330`), so golang-migrate hard-errors
`no migration found for version 8: read down for version 8 migrations: file does not exist`
regardless of migration safety. `n1-boot` therefore has no discriminating signal until a
guard-containing release becomes the N-1 rollback target.

Live evidence (scratch branch `scratch/16-n1-boot-live-check`, now deleted):
- destructive: https://github.com/danielrpof/drop-tracker/actions/runs/33974299591
- docs-only:  https://github.com/danielrpof/drop-tracker/actions/runs/33974944661
- additive:   https://github.com/danielrpof/drop-tracker/actions/runs/33975084671

Full detail: `.planning/phases/16-rollback-safe-migrations/16-UAT.md` § Gaps (G-16-1).

In scope: edit the `n1-boot` job in `.github/workflows/full-pipeline.yml`; add a note to
`internal/db/migrations/README.md`; a short amendment note in
`.planning/phases/16-rollback-safe-migrations/16-CONTEXT.md` recording that n1-boot's
designed behavior is amended for the guard-adoption window.

Out of scope: the unrelated `trivy-fs` failure (frontend transitive-dep CVEs) — its own
quick task. Any change to `internal/db/migrate.go` itself (the guard is correct; the
problem is only that released images don't have it yet).
</domain>

<decisions>
## Implementation Decisions

### Skip vs. red
- When the N-1 image predates the guard, `n1-boot` **skip-greens** and emits a GitHub
  `::notice::` explaining that the previously-released image predates the ahead-of-source
  migration guard, so rollback-boot cannot be verified this cycle. This mirrors the
  existing `True-bootstrap guard` step's D-04/D-19a skip-green-on-no-prior-tag behavior
  exactly (`echo "proceed=false" >> "$GITHUB_OUTPUT"`). It self-clears once a
  guard-containing release becomes N-1 (i.e. after this branch's work ships as v1.8.0 and
  one more migration PR lands).

### Detection
- **Static grep of the previous release's `internal/db/migrate.go`.** The new step runs
  `git show "$PREV_TAG:internal/db/migrate.go"` and checks for the guard's identifying
  token (`maxSourceVersion` — the ahead-of-source predicate at
  `internal/db/migrate.go:299`; `runMigrationsWithSource` is an acceptable equivalent
  token). Absent → pre-guard → skip. This mirrors how `migration-check` already reads
  `git show "$TAG:queries/*.sql"` for the D-15 cross-reference: deterministic, no
  container side effects, no brittle error-string match. If the `git show` itself fails
  (tag gone, file renamed at that tag), treat as pre-guard and skip (fail-safe, same
  direction as D-04's empty-tag handling) with a distinct notice.

### Step placement
- **A new dedicated step** with its own `id` (e.g. `guardcheck`) and its own `::notice::`,
  placed immediately after `True-bootstrap guard`, gated on the same
  `needs.changes.outputs.migrations_changed == 'true'` condition. Its output is ANDed into
  every downstream expensive step's `if:` alongside `steps.bootstrap.outputs.proceed`.
  Keeps "no prior tag" and "prior tag predates the guard" as independently readable skip
  reasons in the run log — consistent with the phase's stated preference for transparent,
  self-describing job runs.

### Claude's Discretion
- Exact token to grep for and exact step `id`/name wording.
- Exact phrasing of the `::notice::` messages.
- Whether the ANDed gate condition is refactored for readability (e.g. a single composed
  boolean output) or each downstream `if:` simply gains `&& steps.<id>.outputs.proceed == 'true'`.
- Exact placement and wording of the `README.md` "guard-adoption window" note and the
  `16-CONTEXT.md` amendment note.
- Whether any lightweight test/assertion is worth adding (the phase's guard tests live in
  `internal/db/migrate_ahead_test.go`; this change is workflow-YAML-only and GitHub's
  `if:` semantics aren't unit-testable from the repo — a shape assertion in a workflow
  lint test is optional, not required).
</decisions>

<specifics>
## Specific Ideas

- The existing `True-bootstrap guard` step (`.github/workflows/full-pipeline.yml`, in the
  `n1-boot` job) is the exact pattern to follow:
  ```yaml
  - name: True-bootstrap guard
    id: bootstrap
    if: needs.changes.outputs.migrations_changed == 'true'
    env:
      PREV_TAG: ${{ steps.prevtag.outputs.tag }}
    run: |
      if [ -z "$PREV_TAG" ]; then
        echo "::notice::no prior release tag found -- true bootstrap, N-1 check not applicable (D-04/D-19a)"
        echo "proceed=false" >> "$GITHUB_OUTPUT"
      else
        echo "proceed=true" >> "$GITHUB_OUTPUT"
      fi
  ```
- Downstream steps are gated:
  `if: needs.changes.outputs.migrations_changed == 'true' && steps.bootstrap.outputs.proceed == 'true'`
  — there are ~9 such steps plus the `failure()` explainer and the `always()` teardown.
- Guard token location: `internal/db/migrate.go:299` — `if smax, ok := maxSourceVersion(src); ok && cur > smax`.
</specifics>

<canonical_refs>
## Canonical References

- `.planning/phases/16-rollback-safe-migrations/16-UAT.md` — gap G-16-1 (source of truth for the failure)
- `.planning/phases/16-rollback-safe-migrations/16-CONTEXT.md` — D-04, D-19a (true-bootstrap skip precedent), n1-boot design
- `internal/db/migrations/README.md` — the N-1 invariant doc that gets the guard-adoption-window note
- `internal/db/migrate.go:213-330` — `runMigrationsWithSource` / `maxSourceVersion` (the guard whose absence is being detected)
</canonical_refs>
