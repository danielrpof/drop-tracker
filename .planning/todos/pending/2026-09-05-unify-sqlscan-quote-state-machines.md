---
created: 2026-09-05T18:30:00.000Z
title: Unify sqlscan's two hand-rolled quote/dollar-quote state machines
area: tooling
severity: minor
files:
  - internal/sqlscan/lex.go
---

## Problem

`StripComments` and `SplitStatements` in `internal/sqlscan/lex.go` each hand-roll
their own scanner over the same grammar: single-quoted string literals (with `''`
escapes) and `$tag$ ... $tag$` dollar-quoted spans. The two implementations are
near-duplicates that must stay in lockstep — a fix applied to one (say, a
dollar-tag edge case, or nested-quote handling) silently leaves the other wrong,
and the divergence would not surface until a specific migration or query file hit
the untouched path.

They were moved as-is out of `cmd/migration-check/main.go` during the
`internal/sqlscan` extraction (quick task 260905-kfv) specifically to keep that
change behavior-preserving. Unifying them there would have made the extraction a
behavior change rather than a move.

## Solution

Now that the module boundary exists, fold both into a single lexer pass — one
scanner that yields the same comment-stripped text and the same statement split
from one place. Prove equivalence against:

1. The existing blackbox lexer suite in `internal/sqlscan/lex_test.go` (including
   the dollar-quote cases).
2. The `cmd/migration-check` golden file
   `cmd/migration-check/testdata/mixed_findings.golden.txt`, which must stay
   byte-identical.
3. `go test ./cmd/migration-check/ ./internal/sqlscan/ -count=1` and
   `make coverage-gate`.

Handle via `/gsd-quick`.
