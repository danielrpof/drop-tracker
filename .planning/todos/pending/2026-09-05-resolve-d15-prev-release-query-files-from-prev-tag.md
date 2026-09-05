---
created: 2026-09-05T18:31:00.000Z
title: Resolve the D-15 previous-release schema and query files from --prev-tag, not the process CWD
area: tooling
severity: minor
files:
  - cmd/migration-check/main.go
---

## Problem

This is candidate 5 of the Phase 16 architecture review.

`buildPrevReleaseRefs` resolves the previous release's schema columns (for
`SELECT *` / `RETURNING *` star expansion) via `filepath.Glob` of
`internal/db/migrations/*.up.sql` **in the process working directory**. So the
D-15 verdict silently depends on where the binary is invoked from: it works from
the repo root (as CI runs it) and degrades to no star expansion under `go test`'s
package-directory CWD.

Separately, `prevReleaseQueryFiles` is a hardcoded four-entry list hand-synced to
`sqlc.yaml`. When a new `queries/*.sql` file is added and this list is not
updated, the cross-reference silently under-scans — a live N-1 break routed only
through the new file would not be caught.

## Solution

Resolve both the migration glob and the query-file list from the `--prev-tag`
through the existing `readAtTag` / `gitShow` seam, which already gates paths
against `allowedGitShowPaths` (`queries/*.sql`, `internal/db/migrations/*.up.sql`,
`internal/db/sqlc/*.go`). List the previous tag's tree under those globs via
`git ls-tree <tag> -- <glob>` behind the same shape/path gate, then read each
entry with `readAtTag`. This kills the CWD dependency and the hand-synced list in
one change.

The seam and its allowlist already exist, so this is a wiring change, not new
attack surface. Verify the golden file stays byte-identical and
`TestPrevReleaseCrossRef_*` stays green.

Handle via `/gsd-quick`.
