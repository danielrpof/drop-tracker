---
status: complete
phase: 16-rollback-safe-migrations
source: [16-VERIFICATION.md]
started: 2026-09-04T00:00:00Z
updated: 2026-09-05T17:10:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Live scratch-branch CI run — destructive migration, docs-only push, additive migration
expected: |
  (a) both guard jobs red, `build-scan` not reached; (b) `build-scan` runs despite
  `migration-check`/`n1-boot` being trivially green (confirms Finding 2's skip-propagation fix
  holds live); (c) `n1-boot` genuinely pulls the ghcr.io image at the previous release tag and
  passes health/read/write probes.
result: pass
reported: |
  Ran on scratch/16-n1-boot-live-check (deleted after). Three pushes:

  (a) destructive `ALTER TABLE events DROP COLUMN release_type` —
      run https://github.com/danielrpof/drop-tracker/actions/runs/33974299591
      - migration-check: RED, correct. Message: "[prev-release-reference] DROP COLUMN
        release_type on events / Still referenced by the previous release (v1.7.0):
        queries/events.sql query InsertEvent ... this is a live N-1 break ... See
        internal/db/migrations/README.md." D-15 cross-reference names tag + query as designed.
      - n1-boot: RED. "N-1 boot failure -- explain the rule" step fired with the correct
        message. BUT the underlying failure was NOT the dropped column — the v1.7.0 container
        never applied the DROP. It died on boot with: "run migrations: migrations failed after
        6 attempts ... apply migrations: no migration found for version 8: read down for
        version 8 migrations: file does not exist" (ahead-of-source).
      - build-scan / release: not reached. Correct.

  (b) docs-only commit (README touch) —
      run https://github.com/danielrpof/drop-tracker/actions/runs/33974944661
      - changes: reports no migration change. Correct.
      - migration-check: GREEN, 22s (scan ran with empty file list). SUCCESS, not skipped.
      - n1-boot: GREEN, 10s (all expensive steps skipped via step-level if:). SUCCESS, not skipped.
      - build-scan: skipped — but because `trivy-fs` (an unrelated needs: entry) FAILED, not
        because either guard job skipped. Could not directly observe "build-scan runs" live.
        The core of RESEARCH Finding 2 still holds: both guards finish as success, not skipped,
        so they do not propagate a skip to build-scan.
      - trivy-fs: RED — 6 HIGH CVEs in frontend transitive deps (browserslist, fast-uri),
        all CVE-2026-xxxxx disclosed after this code was written. Pre-existing on main.
        Unrelated to phase 16.

  (c) additive nullable `ALTER TABLE events ADD COLUMN scratch_probe TEXT` (safe / expand-only) —
      run https://github.com/danielrpof/drop-tracker/actions/runs/33975084671
      - migration-check: GREEN, 24s. Correct.
      - n1-boot: RED. Identical ahead-of-source failure: "apply migrations: no migration found
        for version 8: read down for version 8 migrations: file does not exist", 6 retries,
        "N-1 image never reached a sustained-healthy state". The purely additive, rollback-safe
        migration reds n1-boot exactly the same as the destructive one in (a).

  Pre-flight: `docker manifest inspect ghcr.io/danielrpof/drop-tracker:v1.7.0` returned a
  manifest — the image exists, so the redness is not a missing-image artifact. `svu current`
  resolves to v1.7.0.

  Conclusion: n1-boot cannot distinguish a safe migration from an unsafe one — it reds on
  every migration-touching branch. The image it boots (v1.7.0) predates the ahead-of-source
  no-op guard (`maxSourceVersion` in internal/db/migrate.go) that Phase 16 itself introduced,
  so that binary always fails golang-migrate's "DB version not present in source" check.
  Contradicts Truth #1 ("passes only if that older binary starts and stays healthy").

  --- RESOLVED 2026-09-05 (quick task 260905-et1, confirmed live) ---
  Fix: new gated `guardcheck` step in n1-boot (commit 865c162) that statically inspects the
  resolved N-1 tag's internal/db/migrate.go for the guard token; if absent, skip-greens with
  a ::notice:: naming the tag and G-16-1, ANDed into all 8 expensive steps. Self-clears once
  a guard-carrying release becomes N-1. Also fixed the blocking trivy-fs CVEs (quick task
  260905-fa4, commit 18c36db) that had prevented a clean build-scan observation.

  Live confirmation on scratch/verify-n1boot-trivy-fixes (deleted after):
  - run 33978945980 (additive migration 000008): n1-boot GREEN — guardcheck fired, emitted
    "the previously-released image v1.7.0 predates the ahead-of-source migration guard ...
    (G-16-1). This self-clears ... once a guard-carrying release becomes the N-1 rollback
    target.", all probe steps skipped. migration-check GREEN. trivy-fs GREEN (was 6 HIGH).
  - run 33979094225 (test-assertion bump only): trivy-fs GREEN, test GREEN, n1-boot GREEN
    (7s, migrations_changed=false), **build-scan RAN (1m20s, produced a dockerbuild
    artifact)** — the skip-propagation fix and build-scan-runs both confirmed. release
    skipped (scratch branch, expected).
  Together the runs prove: guardcheck skip path works, both guards finish `success` not
  `skipped`, build-scan runs when its needs are green, and trivy-fs is clean.
severity: major
resolution: |
  Gap G-16-1 closed by quick task 260905-et1 and confirmed live (runs 33978945980 +
  33979094225). Truth #1 now holds within the guard-adoption window via the documented
  skip-green path (internal/db/migrations/README.md + 16-CONTEXT.md amendment 2026-09-05).

## Summary

total: 1
passed: 1
issues: 0
pending: 0
skipped: 0
blocked: 0

## Gaps

- gap_id: G-16-1
  truth: "A CI check boots the previously-released image against a database migrated to the current branch's schema and passes only if that older binary starts and stays healthy (n1-boot); a safe additive migration must leave it green."
  status: resolved
  resolved_by: "quick task 260905-et1 (commits 865c162, 9876c4e) — new n1-boot guardcheck step"
  resolved_at: 2026-09-05
  confirmed_live: "runs 33978945980 (guardcheck fires + notice) and 33979094225 (build-scan runs)"
  reason: |
    User reported: n1-boot reds on EVERY migration-touching branch regardless of migration
    safety. Both a destructive DROP COLUMN (run 33974299591) and a purely additive nullable
    ADD COLUMN (run 33975084671) fail identically with golang-migrate "no migration found for
    version 8: read down for version 8 migrations: file does not exist". The v1.7.0 image
    n1-boot pulls was built before Phase 16's ahead-of-source no-op guard
    (maxSourceVersion / runMigrationsWithSource in internal/db/migrate.go), so that binary
    cannot no-op against a schema ahead of its embedded migration set — it errors. n1-boot
    therefore provides no discriminating signal until a release containing the guard becomes
    the N-1 rollback target. The phase's True-bootstrap guard step only covers "no prior tag
    at all", not "prior tag's image predates the guard".
  severity: major
  test: 1
  artifacts: []
  missing: []

## Notes (not gaps)

- `trivy-fs` was red on `main` (6 HIGH CVEs: browserslist CVE-2026-73088/73089, fast-uri
  CVE-2026-75899/75931/75975/76172) — frontend transitive deps, new disclosures, unrelated
  to Phase 16. Fixed out-of-band by quick task 260905-fa4 (commit 18c36db: caret `overrides:`
  in web/pnpm-workspace.yaml → browserslist 4.28.9 / fast-uri 3.1.7); confirmed green on
  runs 33978945980 / 33979094225. Two follow-up hygiene todos filed (delete stale
  web/package-lock.json; reclassify `shadcn` out of dependencies).
- Parts of test 1 DID pass as designed and should not be re-verified:
  migration-check's two-class DDL detection, the D-15 previous-release query cross-reference
  (names tag + query), the "explain the rule" failure messages on both jobs, the
  unconditional-job / gated-step shape (both guards finish `success` not `skipped` on a
  no-migration push), and the `changes` diff-base computation for a new branch.
