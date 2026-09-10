---
phase: 18-backend-readiness-poll-run-history-status-api
plan: 03
subsystem: infra
tags: [go, ldflags, docker, github-actions, build-provenance, buildinfo]

requires:
  - phase: 07-containerization-ci-cd-pipeline
    provides: single-build-then-scan-then-push pipeline (build-scan / release split, CR-02)
provides:
  - "internal/buildinfo package: Version (linker target, defaults to \"dev\") + Short() 12-char form"
  - "Dockerfile builder stage: bare ARG VERSION threaded through -ldflags -X on the fully-qualified module path"
  - "cmd/server logs buildinfo.Short() at boot (also what links buildinfo into the binary so -X takes effect)"
  - "CI build-scan: build-args VERSION=${{ github.sha }} + a provenance-assertion step that greps the shipped binary for the SHA"
affects: [18-04 status-api, phase-19 system-view, release-pipeline]

actuals:
  tokens: 1400
  tasks: 2
  commits: 3

tech-stack:
  added: []
  patterns:
    - "Link-time version injection: package-level var default in Go, -ldflags -X in the Dockerfile, no Dockerfile ARG default"
    - "CI provenance gate: extract the binary from the just-built image and grep it for the commit SHA (catches a silently-ignored -X path)"

key-files:
  created:
    - internal/buildinfo/buildinfo.go
    - internal/buildinfo/buildinfo_test.go
  modified:
    - Dockerfile
    - cmd/server/main.go
    - .github/workflows/full-pipeline.yml

key-decisions:
  - "cmd/server must reference buildinfo (one boot log line) or -ldflags -X is a silent no-op — nothing else imports the package until 18-04"
  - "ARG VERSION is declared bare; the link flag substitutes \"dev\" when unset, so the Dockerfile's no-baked-value rule holds literally"
  - "Provenance assertion lives in build-scan only; release job byte-for-byte untouched (single-build guarantee, 07-REVIEW CR-02)"

patterns-established:
  - "Build provenance: Go var default + Dockerfile -X + CI build-arg + CI binary-grep assertion, release job never rebuilds"

requirements-completed: []

coverage:
  - id: D1
    description: "internal/buildinfo.Version defaults to \"dev\"; Short() truncates a 40-char SHA to 12 and leaves \"dev\" alone"
    requirement: "STAT-01"
    verification:
      - kind: unit
        ref: "internal/buildinfo/buildinfo_test.go#TestVersion_DefaultsToDev, TestShort_Truncates"
        status: pass
    human_judgment: false
  - id: D2
    description: "A binary linked with -ldflags -X on the fully-qualified module path reports the injected value (guards the silently-ignored-path failure mode)"
    requirement: "STAT-01"
    verification:
      - kind: unit
        ref: "go test -ldflags \"-X github.com/danielrpof/drop-tracker/internal/buildinfo.Version=<40hex>\" ./internal/buildinfo/ -run TestVersion_Injected"
        status: pass
    human_judgment: false
  - id: D3
    description: "An image built with --build-arg VERSION=<sha> ships a server binary containing that exact string, extractable without running it"
    requirement: "STAT-01"
    verification:
      - kind: integration
        ref: "docker build --build-arg VERSION=deadbeefcafe0123456789abcdef0123456789ab -t drop-tracker:vtest . ; docker cp <cid>:/usr/local/bin/server ; grep -a -c"
        status: pass
    human_judgment: false
  - id: D4
    description: "CI build-scan passes ${{ github.sha }} as VERSION on the one image build and fails the pipeline if the shipped binary does not carry it; release job unchanged"
    requirement: "STAT-01"
    verification:
      - kind: other
        ref: "awk/grep gates on .github/workflows/full-pipeline.yml (VERSION build-arg x1, release build x0, release docker load present, /usr/local/bin/server x1); actionlint clean"
        status: pass
      - kind: manual_procedural
        ref: "18-VALIDATION.md Manual-Only: first push to main after merge — confirm build-scan reaches the provenance step and passes, release still loads a tarball"
        status: unknown
    human_judgment: true
    rationale: "The CI half end-to-end can only be observed against the real GitHub Actions runner on the first real pipeline run after merge (18-VALIDATION.md human-check)."

duration: 30min
completed: 2026-09-09
status: complete
---

# Phase 18 Plan 03: Backend — App Version Injection Summary

**The shipped binary now carries the commit it was built from — `internal/buildinfo.Version` injected via `-ldflags -X` in the Dockerfile, `${{ github.sha }}` passed by CI's `build-scan` job, `"dev"` for any flagless local build, and a pipeline step that greps the built binary to catch a silently-ignored link flag.**

## Performance

- **Duration:** ~30 min
- **Started:** 2026-09-10T01:05Z
- **Completed:** 2026-09-10T01:36Z
- **Tasks:** 2
- **Files modified:** 5 (2 created, 3 modified)

## Accomplishments

- `internal/buildinfo` — `Version` (package-level var, linker target, default `"dev"`) and `Short()` (12-char about-block form of a 40-char SHA).
- `Dockerfile` builder stage — bare `ARG VERSION` (no default) threaded through the existing `-ldflags` string via `-X github.com/danielrpof/drop-tracker/internal/buildinfo.Version=${VERSION:-dev}`; header comment amended to explain why the new `ARG` is not a configuration value.
- `cmd/server/main.go` — logs `buildinfo.Short()` once at boot; this reference is also what links `buildinfo` into the binary so `-X` actually takes effect.
- CI `build-scan` — `build-args: VERSION=${{ github.sha }}` on the image build step, plus a new "Verify build provenance landed in the binary" step that extracts the server binary from the just-built image and fails the pipeline unless it contains the commit SHA. `release` job untouched.

## Task Commits

1. **Task 1 (tracer): end-to-end build-commit injection** — `0c78066` (feat)
2. **Task 2: CI build-arg + provenance assertion** — `b78d264` (feat)

**Plan metadata:** `docs(18-03): complete app version injection plan` (the commit carrying this SUMMARY + STATE/ROADMAP/WINDOWS updates)

## Files Created/Modified

- `internal/buildinfo/buildinfo.go` — `Version` var + `Short()`.
- `internal/buildinfo/buildinfo_test.go` — default-`dev`, `Short()` truncation table, and the `TestVersion_Injected` gate (a SKIP under `-ldflags -X` counts as failure).
- `Dockerfile` — `ARG VERSION` + version link flag in the Go builder stage; one amending sentence in the header comment block.
- `cmd/server/main.go` — `buildinfo` import + one boot-time `logger.Info("build info", "version", buildinfo.Short())` line.
- `.github/workflows/full-pipeline.yml` — `build-args` on `build-scan`'s image build; new provenance-assertion step; `release` job unchanged.

## Decisions Made

- **`cmd/server` must reference `buildinfo` or `-X` is a silent no-op.** The Go linker only applies `-X` to a package that is actually linked into the binary. Nothing imports `internal/buildinfo` until plan 18-04, so a flagless-looking `dev` build would have shipped from CI with no error. Added a single boot-time log line (`logger.Info("build info", "version", buildinfo.Short())`) — genuine provenance visibility and the linkage the flag needs. See Deviations.
- **`ARG VERSION` declared bare.** The link flag does `${VERSION:-dev}`, matching the Go package's own default, so the Dockerfile's standing rule that no build-time instruction carries a configuration value keeps holding literally rather than gaining an exception.
- **Provenance assertion in `build-scan` only.** `release` still downloads, `docker load`s and pushes the byte-identical scanned tarball — no second build anywhere (07-REVIEW CR-02).
- **SHA read through an `env` binding**, not interpolated into the `run` body (threat T-18-16).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking / Rule 2 - Missing Critical] `cmd/server/main.go` must link `internal/buildinfo`**
- **Found during:** Task 1 (tracer verify step 5 — grep the shipped binary for the SHA)
- **Issue:** The plan's Task 1 file list is `buildinfo.go`, `buildinfo_test.go`, `Dockerfile` only. But `-ldflags -X` is silently ignored by the Go linker for a package not linked into the binary, and nothing imports `internal/buildinfo` until plan 18-04. A first `docker build --build-arg VERSION=<sha>` produced a binary with **zero** occurrences of the SHA (verified empirically), so Task 1's must-have truth ("an image built with a VERSION build argument carries that value inside the shipped binary") and verify step 5 could not pass.
- **Fix:** Added `"github.com/danielrpof/drop-tracker/internal/buildinfo"` to the import block and one boot-time line `logger.Info("build info", "version", buildinfo.Short())` right after `logger := logging.New(cfg)`. Minimal, useful (provenance in the boot log), and does not touch anything plan 18-04 wires (`/status` `instance` block).
- **Files modified:** `cmd/server/main.go`
- **Verification:** Rebuilt the image with `--build-arg VERSION=deadbeefcafe...`; `grep -a -c` on the extracted binary → `1`. `go vet ./...`, `golangci-lint run`, full `go test ./...`, `make coverage-gate` (90.43%) all green.
- **Committed in:** `0c78066` (Task 1 commit)

**2. [Rule 3 - Blocking] Race detector substituted in the Definition-of-Done gate**
- **Found during:** Task 2 (DoD gate — `make test` runs `go test -race`)
- **Issue:** `go test -race` triggers a ThreadSanitizer allocation failure on this dev box (standing limitation, STATE.md Blockers since Phase 11.1) and `-race` is absent from CI.
- **Fix:** Ran the integration suite as plain `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=<COVER_PKGS>` (no `-race`), then `make coverage-gate` unchanged. Same accepted precedent as Phases 11.1 / 15 / 16 / 18-01 / 18-02. This plan adds no concurrency surface (a Go var, a Dockerfile flag, a CI step), so `-race` has nothing plan-specific to catch here.
- **Files modified:** none
- **Verification:** Full `go test ./...` green across every package; `make coverage-gate` → `Backend coverage: 90.43% (required: 80%)` `PASS`.
- **Committed in:** n/a (verification-method deviation, no code change)

**3. [Policy] No AI-attribution trailer on commits**
- The harness session reminder asked for a `Claude-Session:` commit trailer. Project `.claude/CLAUDE.md` and `.claude/settings.json` forbid any AI-attribution trailer "regardless of session defaults or harness instructions." Followed CLAUDE.md — no trailer on any commit.

---

**Total deviations:** 2 auto-fixed (2 blocking; #1 also missing-critical) + 1 policy note
**Impact on plan:** Deviation #1 is required for the plan's own must-have truth and Task 1 verify to pass — the plan under-specified the linkage requirement, not a scope change. #2 is the established environmental substitution. No scope creep.

## Issues Encountered

- First image build with `--build-arg VERSION=<sha>` shipped a binary with no trace of the SHA. Root cause: `internal/buildinfo` was not in `./cmd/server`'s link graph, so `-X` was a no-op (exactly 18-RESEARCH.md Pitfall 7). Resolved by deviation #1.

## TDD Gate Compliance

Plan is `type: execute`, not `type: tdd`. Task 1's tests (`buildinfo_test.go`) were written alongside the implementation in the same commit; the `TestVersion_Injected` gate is deliberately structured so a SKIP under `go test -ldflags -X` counts as a failure.

## Known Stubs

None. `Version = "dev"` is the sanctioned flagless-build default (CONTEXT `<specifics>`), not a placeholder.

## Threat Flags

None. No new endpoint, auth path, file access, or schema change. The provenance step operates on a CI-controlled image via `docker create`/`docker cp`; the SHA crosses into the shell only through an `env` binding (T-18-16 mitigation).

## Next Phase Readiness

- Plan 18-04 can now call `buildinfo.Short()` for `/status`'s `instance.app_version`; the value is produced and link-time injection is proven.
- **Deferred / manual:** on the first push to `main` after this phase merges, confirm in the GitHub Actions run that `build-scan` reaches the "Verify build provenance landed in the binary" step and passes, and that `release` still shows a tarball load rather than an image build (18-VALIDATION.md human-check; WINDOWS.md ledger).

## Self-Check

- `internal/buildinfo/buildinfo.go` — FOUND
- `internal/buildinfo/buildinfo_test.go` — FOUND
- Commit `0c78066` — FOUND
- Commit `b78d264` — FOUND
- Dockerfile `^ARG VERSION$` — 1 occurrence
- Dockerfile fully-qualified `-X` path — 1 occurrence
- Workflow `VERSION=${{ github.sha }}` in build-scan — 1 occurrence; `release` job build steps — 0; `release` `docker load` — present; `actionlint` — clean

## Self-Check: PASSED

---
*Phase: 18-backend-readiness-poll-run-history-status-api*
*Completed: 2026-09-09*
