---
phase: 09
slug: ci-coverage-gates
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-13
---

# Phase 9 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Developer/CI push → pipeline gating decision | The coverage gate is the only thing standing between an under-tested commit and the build/scan/publish chain | pass/fail verdict |
| `coverage.out` on disk → gate verdict | An untrusted, absent, truncated, or stale profile file is the gate's sole input | coverage percentage |
| npm registry → `web/node_modules` and the CI runner | A new devDependency (`@vitest/coverage-v8`) is executed by every developer and every CI run of the frontend job | package code |
| Coverage denominator → gate verdict | The set of files counted is what turns a percentage into a meaningful claim | file inclusion/exclusion scope |
| Test process → fixture Postgres | The boot test connects to and runs migrations against the shared docker-compose test database | DSN, migration DDL |
| Test process → local TCP listener | The boot test binds a real HTTP listener on a loopback port for the duration of the test | none (loopback only) |
| Test process → public internet | A frontend test that does not mock the API boundary would reach the real Go API and, transitively, MusicBrainz/Deezer | HTTP requests |
| Stubbed global fetch → the rest of the suite | The API unit test replaces a runtime global; a stub that outlives its test would alter other test files | test isolation |
| Pipeline gating graph → the container registry | `build-scan`'s `needs` array is the only thing deciding whether an under-tested commit can become a published image at `ghcr.io` | container image |
| Workflow YAML → CI runner shell | Every `run:` step's body executes on the runner with the workflow's token permissions | shell commands |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-09-01 | Tampering | `Makefile` `coverage-gate` recipe | medium | mitigate | Fails closed on missing/empty/unparseable profile; re-executed live in this session — `Backend coverage: 87.1% (required: 80%) / PASS` | closed |
| T-09-02 | Tampering | `COVER_PKGS` measurement scope | medium | mitigate | `go tool cover -func=coverage.out` confirmed live: `cmd/server` rows present (2), sqlc rows absent (0) | closed |
| T-09-03 | Repudiation | Threshold value | medium | mitigate | `COVERAGE_THRESHOLD_BACKEND ?= 80` confirmed as single greppable literal (grep count 1) | closed |
| T-09-04 | Information disclosure | `make test-integration` echoing `TEST_DATABASE_URL` | low | accept | Pre-existing docker-compose fixture credential, no external exposure; unchanged by this phase | closed |
| T-09-05 | Denial of service | `test` job runtime under `-coverpkg` instrumentation | low | mitigate | `timeout-minutes: 15` confirmed in `full-pipeline.yml`; live runs in this session completed in ~1m45s–1m49s | closed |
| T-09-06 | Tampering | `coverage.include`/`coverage.exclude` scope | high | mitigate | `web/vitest.config.ts` include/exclude/thresholds confirmed live; VERIFICATION.md confirms `history.tsx`/`api.ts` rows present in coverage table | closed |
| T-09-07 | Information disclosure | Coverage report artifacts in working tree | low | mitigate | Text-only reporter, no artifact written to disk | closed |
| T-09-08 | Tampering | CI YAML string interpolation (frontend coverage step) | low | accept | No `${{ github.event.* }}` interpolation added; report is log-only | closed |
| T-09-09 | Tampering | Gap-closing tests written to move a percentage | high | mitigate | `09-REVIEW.md` ran (0 critical/2 warning/2 info); tests exist and independently re-passed in this session and in VERIFICATION.md | closed |
| T-09-10 | Tampering | Threshold or measured-set drift during gap closure | high | mitigate | Threshold literal, `cmd/server` profile presence, and sqlc absence all re-confirmed live post gap-closure | closed |
| T-09-11 | Denial of service | Boot test binding a fixed/colliding TCP port | low | mitigate | `cmd/server/main_test.go:50` confirmed: `net.Listen("tcp", "127.0.0.1:0")` — ephemeral port, not hardcoded | closed |
| T-09-12 | Tampering | Boot test running migrations against a developer's real database | medium | mitigate | `cmd/server/main_test.go:44` confirmed: `testutil.RequirePostgresDSN(t)` — shared helper, not a hand-rolled DSN | closed |
| T-09-13 | Elevation of privilege | Signal-handling change in `main` | low | mitigate | `cmd/server/main.go:65` confirmed: single `signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)` registration in `main` | closed |
| T-09-14 | Tampering | `web/app/lib/api.test.ts` stubbing the runtime's fetch | medium | mitigate | Stub installed per-test, removed via unstub-all in `afterEach`; no other file in the component/route tree stubs fetch (per plan's acceptance criteria and REVIEW.md) | closed |
| T-09-15 | Tampering | Frontend gap-closing tests written to move a percentage | high | mitigate | Mutation-style acceptance criteria enforced per 09-04-SUMMARY.md; REVIEW.md found 0 critical issues | closed |
| T-09-16 | Tampering | Threshold or denominator drift during gap closure | high | mitigate | Include glob and 3-entry exclude array confirmed unchanged live in `web/vitest.config.ts` | closed |
| T-09-17 | Repudiation | An ignored threshold reported as an enforced one | medium | mitigate | Directly proven live in this session: raising all four axes to 99 caused `frontend-test` to fail in CI (run 31724744670), then restoring to 70 passed (run 31724954534) | closed |
| T-09-18 | Tampering | Test-only attributes added to production components | low | mitigate | Per plan acceptance criteria, `git diff --stat` for components under test was empty at close | closed |
| T-09-19 | Elevation of privilege | `build-scan` `needs` graph | high | mitigate | Confirmed live in `full-pipeline.yml`: `needs: [vet, lint, test, gitleaks, trivy-fs, frontend-test]`; **directly observed in this session** — `build-scan` conclusion=skipped on both the backend-red run (31724487315) and frontend-red run (31724744670), and ran to completion on the green run (31724954534) | closed |
| T-09-20 | Tampering | Gate escape hatches (`continue-on-error`, conditional `if`, swallowed exit codes) | high | mitigate | Confirmed live: `grep -n "continue-on-error" full-pipeline.yml` returns zero matches | closed |
| T-09-21 | Tampering | Supply chain via a new GitHub Action | medium | mitigate | No new action introduced; existing commit-SHA-pinning obligation (CICD-08) unaffected | closed |
| T-09-22 | Tampering | Workflow YAML expression injection | low | accept | No `${{ ... }}` interpolation added; report is log-only with no PR comment | closed |
| T-09-23 | Denial of service | `test` job timeout under coverage instrumentation | medium | mitigate | `timeout-minutes: 15` confirmed live, set from measured wall-clock data per 09-01/09-05 | closed |
| T-09-24 | Repudiation | Human-check experiments run on the default branch | medium | mitigate | **Directly executed in this session**: all three parts of the human verification ran on scratch branch `test/coverage-gate-ci-check`, never `main`; branch deleted (local + remote) after; `main` untouched throughout | closed |
| T-09-SC | Tampering | Package-manager installs (all 5 plans) | low | mitigate | Only new dependency (`@vitest/coverage-v8@4.1.10`) audited OK in 09-RESEARCH.md — official `vitest-dev` monorepo, pinned exact version, no `postinstall` script, lockfile committed with `--frozen-lockfile` in CI | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-09-01 | T-09-04 | Test-fixture DB credential in CI log output grants nothing outside a throwaway docker-compose container; pre-existing, unchanged by this phase | Plan 09-01 author | 2026-08-13 |
| AR-09-02 | T-09-08 | No `${{ github.event.* }}` interpolation added by this phase's frontend coverage step | Plan 09-02 author | 2026-08-13 |
| AR-09-03 | T-09-22 | No `${{ ... }}` interpolation added by this phase's CI wiring; report stays log-only | Plan 09-05 author | 2026-08-13 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-13 | 25 | 25 | 0 | Claude (gsd-secure-phase, L1 grep-depth + live CI verification) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-13
