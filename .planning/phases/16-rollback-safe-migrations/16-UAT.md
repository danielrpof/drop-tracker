---
status: testing
phase: 16-rollback-safe-migrations
source: [16-VERIFICATION.md]
started: 2026-09-04T00:00:00Z
updated: 2026-09-04T00:00:00Z
---

## Current Test

number: 1
name: Live scratch-branch CI run — destructive migration, docs-only push, additive migration
expected: |
  (a) On a scratch branch (never `main`, delete after — Phase 09 precedent): add a throwaway
  migration `DROP`-ping a column the previous release's queries still reference, and push.
  Expect `migration-check` **red** (D-15 cross-reference message naming the tag/query) and
  `n1-boot` **red** (N-1 rule message naming `internal/db/migrations/README.md`), `build-scan`
  **not reached**.

  (b) Remove the dummy migration, push a docs-only commit. Expect `changes` reports no
  migration change, `migration-check` and `n1-boot` both **green in seconds** with their
  expensive steps shown as skipped, and `build-scan` **runs** (not "Skipped"). This is the
  single highest-value observation — it confirms RESEARCH.md Finding 2's skip-propagation fix
  actually holds on a real GitHub Actions run, not just structurally in the YAML.

  (c) Push a purely additive nullable `ADD COLUMN` migration. Expect both checks **green**,
  with `n1-boot` having genuinely pulled and booted the previous release image from ghcr.io
  and passed its health/read/write probes.

  Before starting: confirm a recent `release` workflow run actually pushed
  `ghcr.io/danielrpof/drop-tracker` at the tag `svu current` currently resolves to (a
  half-failed release could leave a tag with no image, which would redden `n1-boot` for a
  reason unrelated to migrations).

  Record all three run URLs and the observed per-job statuses, then delete the scratch branch
  locally and remotely.
awaiting: user response

## Tests

### 1. Live scratch-branch CI run — destructive migration, docs-only push, additive migration
expected: |
  (a) both guard jobs red, `build-scan` not reached; (b) `build-scan` runs despite
  `migration-check`/`n1-boot` being trivially green (confirms Finding 2's skip-propagation fix
  holds live); (c) `n1-boot` genuinely pulls the ghcr.io image at the previous release tag and
  passes health/read/write probes.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
