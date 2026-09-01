# Phase 14 — Deferred Items

## Out-of-scope discoveries (not fixed)

### `tsc --noEmit` fails on a stale react-router typegen artifact

- **Found during:** Plan 14-06, Task 2 verification.
- **Symptom:** `cd web && node_modules/.bin/tsc --noEmit -p tsconfig.json` exits 2 with
  `app/root.tsx(19,28): error TS2307: Cannot find module './+types/root' or its corresponding type declarations.`
- **Pre-existing:** confirmed by stashing plan 14-06's `authStore.ts` change and re-running — the
  same single error remains. Not introduced by this plan (no `root.tsx` edit, no change to
  `authStore`'s exported type surface).
- **Why not fixed here:** outside plan 14-06's `files_modified` scope fence. `react-router build`
  (which runs `react-router typegen` itself) passes cleanly, so the CI/Docker build path is
  unaffected — only a bare `tsc` invocation without a prior typegen trips it.
- **Suggested fix:** a separate quick task to run `react-router typegen` as a pretest/pretypecheck
  step (or add `.react-router/types` regeneration to the `make`/CI typecheck target).
