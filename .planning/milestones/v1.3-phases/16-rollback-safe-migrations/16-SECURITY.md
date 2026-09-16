---
phase: "16"
slug: "rollback-safe-migrations"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-05"
---

# Phase 16 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.
> Register authored at plan time across `16-01`…`16-05` `<threat_model>` blocks; verified
> retroactively by `gsd-security-auditor` on 2026-09-05 (State B — created from artifacts).

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| rolled-back binary → live production schema | An older binary is handed a schema it has never seen; it must operate that schema, never reconcile it. | DDL / `schema_migrations` version |
| CI runner env → `cmd/migrate` | The DSN reaches the helper as an environment variable set by the workflow. | Postgres DSN (secret) |
| pull-request author → `cmd/migration-check` | Migration SQL and the `allow-destructive` annotation are attacker-influenceable text supplied by whoever opens the PR. | SQL, annotation grammar |
| pull-request author → `git show` / `git rev-parse` / `git diff` argv | The `expand-shipped-in` annotation value and `github.event.before` / `github.sha` / `github.base_ref` become subprocess arguments. | git revisions / refs |
| previous release tag → repository history read | The tool reads blobs at a tag; the readable set must be bounded. | `queries/*.sql`, `*.up.sql`, `sqlc/*.go` |
| ghcr.io registry → CI runner | The N-1 image is pulled from a public registry and executed on the runner. | container image |
| CI runner → throwaway Postgres | Ephemeral credentials on host networking for one job. | throwaway DSN, no prod data |
| workflow job graph → release path | Job dependency edges are the mechanism that blocks a merge; a wiring error silently removes the gate. | job status / skip propagation |
| documentation → operator behaviour | The README is the primary control preventing a rollback-breaking migration; the CI gate is the backstop. | schema-evolution policy |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-16-01 | Tampering | `runMigrationsOnce` ahead-of-source guard | high | mitigate | `internal/db/migrate.go:298-302` — guard fires only on `verr == nil && !dirty` then `ok && cur > smax`; dirty falls through to `m.Up()` and still surfaces `ErrDirty`. Pin: `migrate_ahead_test.go` `TestRunMigrationsWithSource_DirtyAheadStillErrors` (+4 siblings). | closed |
| T-16-02 | Tampering | ahead-of-source detection mechanism | medium | mitigate | `migrate.go:326-341` `maxSourceVersion` walk + numeric `cur > smax` at `:299`; zero `strings.Contains` / error-text matching in `migrate.go`. | closed |
| T-16-03 | Information disclosure | `cmd/migrate` DSN handling | medium | mitigate | `cmd/migrate/main.go:32-40` — DSN from env only, never printed, passed to `db.RunMigrations`; redaction at `migrate.go:223,231,254`. Pin: `migrate_test.go` `TestRunMigrations_NeverLogsDSN` (+ value-form variant). | closed |
| T-16-04 | Denial of service | boot path startup cost | low | accept | One `m.Version()` + in-memory `maxSourceVersion` walk, no extra network I/O (`migrate.go:298-299`). | closed (accepted) |
| T-16-05 | Elevation of privilege | migration advisory-lock contention between concurrent migrators | low | accept | `m.Version()` at `:298` is outside golang-migrate's advisory lock — sound only under the single-migrator assumption (D-17, PITFALLS Pitfall 10). Recorded as backstop truth. | closed (accepted) |
| T-16-10 | Repudiation | guard verdict with an empty input list | high | mitigate | `cmd/migration-check/main.go:332-340` — `"Scanned migration files:"` printed before any early return, `"  (none)"` on the empty case (D-07). Pin: `main_test.go:234`. | closed |
| T-16-11 | Tampering / Elevation | `expand-shipped-in` annotation value | high | mitigate | `main.go:497` `reTagShape` enforced at `:525` in `parseAnnotation` before the value is stored. Pin: `TestAnnotation_TagShapeIsValidated`. | closed |
| T-16-12 | Tampering | annotation used to wave through a live N-1 break | high | mitigate | `main.go:104-111` extracts cross-ref hits before the `switch` at `:113`; `shouldSuppress` (`:536`) only ever sees `otherFindings`. Closed by T-16-24. | closed |
| T-16-13 | Spoofing | destructive DDL hidden from the scanner | medium | mitigate | `main.go:555-603` `stripComments`; quote-aware `copySingleQuoted` / `dollarTagAt`; literal- and dollar-aware `splitStatements:677-733`; `^`-anchored classify regexes `:748-756`. Fixture `testdata/commented_out.sql`; pins `main_test.go:111,251,285`. | closed |
| T-16-14 | Denial of service | over-broad type-narrowing detection blocking a safe migration | low | accept | `reAlterType` (`main.go:753`) flags every `ALTER COLUMN … TYPE` with no widen/narrow analysis; `allow-destructive` annotation is the escape hatch (RESEARCH D-08). | closed (accepted) |
| T-16-15 | Information disclosure | `gosec` G304 carve-out scope | medium | mitigate | `.golangci.yml:49-51` — `path: '^cmd/migration-check/'` scoped to `gosec` only, mirroring the `cmd/coverage-report` entry. | closed |
| T-16-20 | Tampering / Elevation | `expand-shipped-in` → `git show` / `git rev-parse` argv | high | mitigate | Re-check before subprocess at `main.go:895` (`readAtTag` → `reTagShape`) guarding `gitShow:857`; all git calls argv slices (`:200,:248,:858`); zero `sh -c` in the package. Pins `main_test.go:371,688`. | closed |
| T-16-21 | Elevation | `--base-ref` / `--before` / `--sha` → `git diff` argv | high | mitigate | `main.go:217-241` `diffRange`; `validCommitish:172` (7-40 lowercase hex), `validBranchRef:189` (rejects `..`); rejection returns before argv assembly at `:248`. Pins `TestDiffRange`, `main_test.go:623`. | closed |
| T-16-22 | Information disclosure | arbitrary blob read at a release tag via `gitShow` | medium | mitigate | `main.go:868-872` fixed three-glob allowlist; `pathAllowedForGitShow:879` (`path.Match`, `*` never crosses `/`); gate at `:898` before `gitShow`. Pin: `main_test.go:668`. | closed |
| T-16-23 | Repudiation / gate bypass | guard scanning zero files because of a bad diff base | high | mitigate | `diffRange` branches for `pull_request:219`, `push:224`, all-zeroes `:225-226`, unreachable-before `:231-232` (both fall back to `origin/main...HEAD`); unknown event errors `:239`. Plus unconditional scanned-file echo (T-16-10). | closed |
| T-16-24 | Tampering | `allow-destructive` annotation waving through a live N-1 break | high | mitigate | `main.go:124` appends `crossRefFindings` outside the suppression switch; `crossReferenceFinding:1431-1468` with the CR-01 schema-strip fix at `:1439`. Pins `TestPrevReleaseCrossRef_AnnotationCannotOverride`, `…SchemaQualifiedDropTableIsRed`. | closed |
| T-16-25 | Denial of service | unrecoverable false red from an over-broad reference match | medium | mitigate | Only `hasHigh` / `hasHighAnyColumn` consulted (`main.go:1444,1450,1452`); no low-confidence path can red. Pin: `TestPrevReleaseCrossRef_LowConfidenceIsNotRed`. | closed |
| T-16-26 | Spoofing | unverifiable previous release masking a break | medium | mitigate | `main.go:1414-1417` read failure on any `queries/*.sql` → hard error propagated at `:73-75`; bootstrap skip printed at `:69`. Pins `main_test.go:1024,1034`. | closed |
| T-16-30 | Repudiation / gate bypass | job-level condition propagating a skip through `build-scan.needs:` | high | mitigate | `full-pipeline.yml` — `changes:302`, `migration-check:336`, `n1-boot:380` carry no job-level `if:`; `build-scan.needs:565` contains both. Live-confirmed on UAT run 33979094225 (build-scan ran). | closed |
| T-16-31 | Spoofing | `n1-boot` executing a tampered or unexpected image | high | mitigate | Image from `svu current` (`:401-404`) → immutable `:$PREV_TAG` at `:478,493`; zero `:latest` in the job block. Digest pinning deferred to Phase 17 (D-13). | closed |
| T-16-32 | Tampering | probe becoming a permanent false green because the instance gate is active | high | mitigate | `full-pipeline.yml:490-493` — container receives exactly `DATABASE_URL` + `HTTP_PORT`; `INSTANCE_PASSPHRASE` only in a comment at `:480`, never an env key. Phase 14 inert path applies → schema break surfaces as a 500. See UF-1. | closed |
| T-16-33 | Elevation | shell injection via `github.base_ref` / `github.event.before` on a fork PR | high | mitigate | Every `${{ }}` in lines 296-555 sits in an `outputs:` or `env:` position; zero interpolations inside any `run:` body; shell refs are `"$EVENT_NAME"` / `"$PREV_TAG"` etc. Plus Go-tool allowlist (T-16-21). | closed |
| T-16-34 | Elevation | over-privileged workflow token | medium | mitigate | `permissions: contents: read` at `:305-306,340-341,384-385`; `grep -c 'docker login'` over the file = 0; ghcr.io package public, no new secret. | closed |
| T-16-35 | Information disclosure | throwaway Postgres reachable from the runner | low | accept | `full-pipeline.yml:450-452` — throwaway `drop_tracker:drop_tracker` (identical to `Makefile:9`), runner localhost, no prod data, teardown at `:550-554` under `if: always()`. | closed (accepted) |
| T-16-36 | Denial of service | transient registry failure blocking an unrelated migration PR | medium | accept | Single bounded pull retry at `:479` (D-11 S7); D-12 step gating limits the pull to migration-touching pushes; mitigation is a manual re-run. | closed (accepted) |
| T-16-37 | Repudiation | true-bootstrap skip silently disabling the check on a repo that has tags | medium | mitigate | Bootstrap step `:408-419` fires only on empty `PREV_TAG`, emits `::notice::` at `:415`; `fetch-depth: 0` + `fetch-tags: true` at `:389-391` prevents a shallow checkout faking the empty case. Unreachable on this repo (`v1.x` tags exist). | closed |
| T-16-40 | Repudiation | documented rule silently drifting from what the tool enforces | medium | mitigate | `internal/db/migrations_readme_test.go:23-57` — 9 required-phrase subtests; `:61-72` 60-line floor defeats a phrase-stuffed stub. | closed |
| T-16-41 | Tampering | a second, divergent copy of the rule in `.claude/CLAUDE.md` | low | mitigate | `.claude/CLAUDE.md:9` — exactly one line, pointer only, rule not restated. | closed |
| T-16-42 | Information disclosure | README contents | low | accept | Credential / DSN / host scan over `internal/db/migrations/README.md`: 0 hits — schema-evolution policy plus already-public migration filenames only. | closed (accepted) |
| T-16-43 | Denial of service | README added inside the embedded migrations directory | medium | mitigate | `internal/db/migrate.go:22` `//go:embed migrations/*.sql` — markdown cannot be captured and cannot become a phantom migration version. | closed |
| T-16-SC | Tampering | npm/pip/cargo installs (supply chain) | high | accept | Verified, not assumed: zero `go.mod` / `go.sum` / `package.json` / `pnpm-lock` changes across all 35 phase-16 commits; `cmd/migration-check` is stdlib-only. `16-RESEARCH.md` "Package Legitimacy Audit" table present and empty by design. | closed (accepted) |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (high) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-16-01 | T-16-04 | Guard adds one `m.Version()` round trip + O(migration-count) in-memory walk, no network I/O; negligible against the existing bounded-retry budget. | Phase 16 plan (16-01) | 2026-09-05 |
| AR-16-02 | T-16-05 | Advisory-lock contention between concurrent migrators is out of scope per D-17 — single-instance deployment. `m.Version()` read outside the lock is safe only under that assumption; recorded as backstop truth. | Phase 16 plan (16-01) | 2026-09-05 |
| AR-16-03 | T-16-14 | Every `ALTER COLUMN … TYPE` is flagged including genuine widenings — a lexical widen-vs-narrow comparison is unreliable (RESEARCH D-08). `allow-destructive` is the escape hatch; the repo has done zero such migrations in seven releases. | Phase 16 plan (16-02) | 2026-09-05 |
| AR-16-04 | T-16-35 | Throwaway Postgres is ephemeral, bound to the runner's own localhost, uses the same throwaway credentials the `Makefile` already uses, holds no production data, and is torn down with the job. | Phase 16 plan (16-04) | 2026-09-05 |
| AR-16-05 | T-16-36 | Transient registry failure blocking a migration PR is accepted at D-11 S7 — bounded by one pull retry and by D-12 step gating; mitigation is a manual re-run. | Phase 16 plan (16-04) | 2026-09-05 |
| AR-16-06 | T-16-42 | The migrations README documents schema-evolution policy and cites already-public in-tree migration filenames; contains no credential, DSN, or host detail. | Phase 16 plan (16-05) | 2026-09-05 |
| AR-16-07 | T-16-SC | No package-manager install task exists in this phase; zero dependency-manifest changes across all 35 phase-16 commits. | Phase 16 plan (all) | 2026-09-05 |

*Accepted risks do not resurface in future audit runs.*

---

## Unregistered Flags

Surfaced by the auditor from a direct diff of post-plan work (no SUMMARY carries a `## Threat Flags` section). Both are **non-blocking**.

### UF-1 — `n1-boot` guard-adoption skip-green step (post-plan, closing UAT gap G-16-1)

`.github/workflows/full-pipeline.yml:425-445` (`id: guardcheck`, commit `865c162`, quick task 260905-et1). A second, currently always-active skip path: all 8 expensive `n1-boot` steps are ANDed on `steps.guardcheck.outputs.proceed == 'true'`, which is `false` today because `v1.7.0` predates `maxSourceVersion`. Not covered by any plan `<threat_model>`.

**Compensating controls (verified):** `::notice::` naming tag + G-16-1 (`:438,:441`); a hard `::error::` + `exit 1` if the guard token vanishes from HEAD (`:434-436`, closes the stale-token silent-skip hole); documentation at `internal/db/migrations/README.md:50-58` and the `16-CONTEXT.md` amendment (2026-09-05); self-clears with no workflow edit once a guard-carrying release becomes N-1.

**Residual:** nothing alarms if the guard-adoption window stays open — no check fails should a future release ship without the guard. **Recommended:** a Phase 17 follow-up that re-asserts the probe once a guard-carrying tag is N-1.

### UF-2 — `guardcheck` step shells `git show` directly (informational)

`full-pipeline.yml:437` runs `git show "$PREV_TAG:internal/db/migrate.go"` in a `run:` body, bypassing the Go tool's `readAtTag` path allowlist (T-16-22) and `reTagShape` gate (T-16-20). `PREV_TAG` derives from `svu current` over base-repo tags, is double-quoted, and there is no `eval` — not injectable — but it is a git-argv path no threat row covers and no test pins.

---

## Residual Notes on Otherwise-Closed Threats

- **T-16-20 pin gap:** the plan names an `exec.Command("sh"` count assertion as a pin; that string exists only in `16-03-PLAN.md:452`, not in the repo. The control is verified by direct enumeration (3 argv-form calls, 0 shell-form) but no durable regression test would catch a future shell-form invocation.
- **T-16-13:** single-quoted literals are preserved verbatim (not "stripped" as the plan says) by a quote-aware copier; the declared property still holds. Residual outside the declared vector: DDL executed from inside a dollar-quoted `DO $$ … EXECUTE '…' … $$` block would not be classified (statement begins with `DO`).
- **T-16-24:** code-review finding WR-01 (`main.go:791` / `:1449`) remains unfixed by design — the `rename_column` display string `"old -> new"` is re-parsed by `crossReferenceFinding`, so a future edit to the rendering could silently break the non-overridable D-15 lookup with no test signal. Tracked in `16-REVIEW.md`; no live bug.
- **T-16-03:** `internal/db/migrate.go:229` returns the underlying error unredacted on `context.Canceled` / `DeadlineExceeded`. Pre-existing path, not surface introduced by this phase.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-05 | 31 | 31 | 0 | gsd-security-auditor (opus), ASVS L1, block_on=high |

Independently re-ran `go test ./cmd/migration-check/... -count=1` → ok.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-05
