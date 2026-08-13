---
phase: 07-containerization-ci-cd-pipeline
reviewed: 2026-08-12T00:00:00Z
depth: standard
files_reviewed: 28
files_reviewed_list:
  - .dockerignore
  - .github/workflows/full-pipeline.yml
  - .golangci.yml
  - .pre-commit-config.yaml
  - docker-compose.yml
  - Dockerfile
  - go.mod
  - go.sum
  - internal/config/config_test.go
  - internal/db/migrate.go
  - internal/db/migrate_test.go
  - internal/db/pool_timeout_test.go
  - internal/deezer/albums.go
  - internal/deezer/search.go
  - internal/discord/client.go
  - internal/httpserver/boot_e2e_test.go
  - internal/httpserver/events_test.go
  - internal/httpserver/health_test.go
  - internal/httpserver/search.go
  - internal/httpserver/search_test.go
  - internal/httpserver/server_test.go
  - internal/httpserver/spa_test.go
  - internal/httpserver/watchlist_test.go
  - internal/musicbrainz/recordings.go
  - internal/musicbrainz/releasegroups.go
  - internal/musicbrainz/releases.go
  - internal/musicbrainz/search.go
  - internal/watchlist/service_test.go
  - README.md
findings:
  critical: 2
  warning: 4
  info: 2
  total: 8
status: issues_found
---

# Phase 07: Code Review Report

**Reviewed:** 2026-08-12T00:00:00Z
**Depth:** standard
**Files Reviewed:** 28
**Status:** issues_found

## Summary

Reviewed the genuinely new phase-07 artifacts (Dockerfile, `.dockerignore`, `docker-compose.yml`, `.golangci.yml`, `.pre-commit-config.yaml`, `.github/workflows/full-pipeline.yml`, `go.mod`/`go.sum`) plus the incidentally-touched pre-existing Go source (mechanical `errcheck` fixes in `internal/deezer`, `internal/discord`, `internal/musicbrainz`, `internal/db`) and the two `-race`-discovered fixes (`internal/httpserver/search.go`'s `httplog.SetAttrs` fan-out race, `internal/db/pool_timeout_test.go`'s `blackHoleAddr` cleanup-order race).

The two `-race` fixes are correct: `search.go` now collects per-source errors under a mutex and defers all `httplog.SetAttrs` calls to the single goroutine after `wg.Wait()` rejoins, and `pool_timeout_test.go`'s cleanup now closes the listener, waits for the accept goroutine to exit, and only then closes/drains the `held` channel — eliminating the close-vs-send race. The mechanical `errcheck` fixes across the client packages (`defer func() { _ = resp.Body.Close() }()` etc.) are all benign, well-precedented, and none silently swallow an error that the surrounding logic actually depended on.

The CI/CD pipeline itself (the actual point of this phase, per CLAUDE.md) has two structural gaps serious enough to undermine its stated guarantees: the `lint` job never installs a pinned Go toolchain (unlike every other Go-touching job), and the image Trivy scans is not the image that gets pushed to `ghcr.io` — the release job rebuilds it from scratch via a second, independent `docker/build-push-action` invocation against mutable, non-digest-pinned base image tags. Both are detailed below along with several lower-severity gaps versus the pipeline plan documented in CLAUDE.md.

## Critical Issues

### CR-01: `lint` job never installs a pinned Go toolchain, unlike every other Go job in the pipeline

**File:** `.github/workflows/full-pipeline.yml:27-35`
**Issue:** The `vet`, `test`, and `release` jobs all explicitly run `actions/setup-go` with `go-version-file: go.mod` before doing anything Go-related (lines 20-23, 42-45, 110-113). The `lint` job does not:

```yaml
lint:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@...v7.0.1
      - name: golangci-lint
        uses: golangci/golangci-lint-action@...v9.3.0
        with:
          version: v2.12.2
```

`golangci-lint-action` does not install a Go toolchain itself — it requires one already on `PATH` (that's why every other job in this same file sets one up). Without `actions/setup-go`, the action falls back to whichever Go version happens to be preinstalled on the `ubuntu-latest` runner image. `go.mod` pins `go 1.26` (line 3) — a very recent toolchain version — so if the runner's preinstalled Go is older, `golangci-lint` will fail outright (`go.mod requires go >= 1.26`) or, worse, silently type-check/lint the codebase against a different Go version than `vet`/`test`/`release` use, producing lint results that don't match what actually builds and ships. This is a real, currently-latent CI defect: it either breaks the `lint` job or makes it non-authoritative.
**Fix:**
```yaml
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - name: Set up Go
        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: golangci-lint
        uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: v2.12.2
```

### CR-02: The image Trivy scans is never the image pushed to `ghcr.io`

**File:** `.github/workflows/full-pipeline.yml:72-96` (scan) and `:134-146` (push)
**Issue:** `build-scan` builds an image locally (`push: false, load: true, tags: drop-tracker:scan`, lines 82-90) and scans that local image with Trivy (lines 91-96), gated `exit-code: '1'`. The `release` job, which `needs: [build-scan]`, then performs a **second, completely independent** `docker/build-push-action` invocation (lines 134-141) against the same `context: .` and pushes *that* build's output to `ghcr.io`. Nothing ties the two builds to the same image digest — there is no `docker save`/`load` handoff, no digest pinning, and no re-tag-and-push of the already-scanned image.

Every base image in the Dockerfile is referenced by a mutable tag, not a digest (`node:26-alpine3.24`, `golang:1.26.5-alpine3.24`, `alpine:3.24` — see `Dockerfile:22,41,64`), and stage 3 runs `apk add --no-cache ca-certificates` (`Dockerfile:72`), which re-resolves the Alpine package index at build time. Two separate `docker build` invocations minutes apart in the same CI run are *likely* to produce equivalent output, but there is no guarantee: an upstream tag repoint or package index update between the two builds means the artifact that actually reaches `ghcr.io` was never the one Trivy exited 0 on. This defeats the entire purpose of gating on `severity: CRITICAL,HIGH` / `exit-code: 1` — the gate can pass while a materially different (and potentially vulnerable) image ships.
**Fix:** Build once, load it locally, scan it, then push that exact image (or push by digest) instead of rebuilding:
```yaml
  build-scan:
    outputs:
      digest: ${{ steps.build.outputs.digest }}
    steps:
      - id: build
        uses: docker/build-push-action@...
        with:
          context: .
          push: false
          load: true
          tags: drop-tracker:scan
      - uses: aquasecurity/trivy-action@...
        with:
          image-ref: drop-tracker:scan
          severity: CRITICAL,HIGH
          exit-code: '1'
      - run: docker save drop-tracker:scan -o image.tar
      - uses: actions/upload-artifact@...
        with: { name: scanned-image, path: image.tar }

  release:
    needs: [build-scan]
    steps:
      - uses: actions/download-artifact@...
        with: { name: scanned-image }
      - run: |
          docker load -i image.tar
          docker tag drop-tracker:scan ghcr.io/danielrpof/drop-tracker:${{ env.VERSION }}
          docker push ghcr.io/danielrpof/drop-tracker:${{ env.VERSION }}
```

## Warnings

### WR-01: Trivy only scans the built image, not the filesystem (`go.sum`), as the project's own CI/CD plan specifies

**File:** `.github/workflows/full-pipeline.yml:91-96`
**Issue:** CLAUDE.md's Technology Stack section is explicit: "Run twice in CI: `scan-type: fs` against the repo (catches vulnerable `go.sum` deps) and `image-ref:` against the built image (catches OS package + base-image CVEs)." Only the `image-ref` scan is implemented. There is no fail-fast `fs` scan against the repo before the (comparatively expensive) Docker build stage runs, and dependency vulnerabilities that don't surface through Trivy's Go-binary language analyzer inside the image (e.g. a vulnerable module that gets built out via dead-code elimination but still sits in `go.sum`, or a vuln class the binary scanner doesn't cover) go undetected.
**Fix:** Add an `fs`-mode Trivy step (e.g. in the `vet` or a new fast-fail job) scanning `.` before `build-scan` runs, matching the documented two-scan plan.

### WR-02: `.golangci.yml` enables no security-focused linter

**File:** `.golangci.yml:17-18`
**Issue:** `linters: default: standard` enables the standard low-noise subset (`errcheck`, `govet`, `staticcheck`, `unused`, `ineffassign`, etc.) but not `gosec` or any equivalent security linter. This project's own domain has several places where static security analysis has direct value — DSN handling and redaction (`internal/db/migrate.go`), webhook URL handling that must never be logged (`internal/discord/client.go`), user-supplied query strings escaped into Lucene syntax (`internal/musicbrainz/search.go`). `gitleaks` catches literal committed secrets, not misuse patterns (e.g. an accidental `fmt.Errorf("%w", err)` around a `*url.Error` that would leak the webhook token — the exact class of bug `discord/client.go:137-143`'s comment says was deliberately avoided by hand). Nothing in CI would catch a regression of that kind today.
**Fix:** Add `gosec` (or the subset of its rules relevant to this codebase) to `.golangci.yml`'s enabled linters, with documented exclusions for any false positives.

### WR-03: SBOM generation step has no verified output destination

**File:** `.github/workflows/full-pipeline.yml:142-146`
**Issue:**
```yaml
- name: Generate SBOM
  uses: anchore/sbom-action@...v0.24.0
  with:
    image: ghcr.io/danielrpof/drop-tracker:${{ env.VERSION }}
    format: spdx-json
```
No `output-file` is set, and there's no follow-up `actions/upload-artifact` step. CLAUDE.md's Technology Stack table notes the action "Auto-attaches as a release asset when the workflow runs on a release/tag event" — but this workflow is triggered on `push` to `main` (not a GitHub `release` event), and nothing in this pipeline ever creates a GitHub Release object (only a bare `git tag` via `git push origin "$VERSION"`, line 150-151). If the SBOM step's persistence relies purely on the action's own default `upload-artifact` behavior, that's undocumented here and worth making explicit — as written, a reviewer cannot tell from the workflow file whether the generated SBOM is retrievable after the run or silently discarded with the runner.
**Fix:** Set `output-file: sbom.spdx.json` explicitly and add an `actions/upload-artifact` step (or push the SBOM to the registry alongside the image via `anchore/sbom-action`'s registry-push mode) so the artifact's destination is explicit and auditable.

### WR-04: Two near-simultaneous pushes to `main` can race on `svu next` and tag/push

**File:** `.github/workflows/full-pipeline.yml:10-12,114-125,147-151`
**Issue:** `concurrency.cancel-in-progress` is deliberately `false` for `push` events (line 12), so two pushes to `main` in quick succession both run their `release` job to completion rather than the second cancelling the first. `svu next` (lines 118-120) computes the next version from the tag history visible at checkout time; if both runs check out before either has pushed a tag, both compute the *same* next version, both build and push an image tagged with that version, and both attempt `git tag "$VERSION"` / `git push origin "$VERSION"` (lines 149-151). The second run's tag push will fail (tag already exists) — but its image push to `ghcr.io` happens *before* the tag step and will have already succeeded, leaving an image in the registry tagged with a version that has no corresponding git tag, and a failed (red) `release` job for a push that otherwise built and shipped successfully.
**Fix:** Either scope concurrency more narrowly for the `release` job specifically (e.g. a dedicated concurrency group that queues releases rather than running them in parallel), or check `git tag` existence for `$VERSION` immediately after computing it and skip/fail fast before any build/push work happens.

## Info

### IN-01: Base images are pinned by tag, not digest

**File:** `Dockerfile:22,41,64`
**Issue:** `node:26-alpine3.24`, `golang:1.26.5-alpine3.24`, and `alpine:3.24` are all mutable tags. Every GitHub Actions step in `full-pipeline.yml` is pinned to a full commit SHA for exactly this reason (supply-chain reproducibility), but the same discipline isn't applied to the Dockerfile's `FROM` lines. This is the direct mechanism that makes CR-02 possible (two builds of "the same" Dockerfile at different times may not be byte-identical).
**Fix:** Pin base images by digest (`FROM golang:1.26.5-alpine3.24@sha256:...`), updated deliberately alongside dependency bumps, mirroring the Actions SHA-pinning already practiced elsewhere in this phase.

### IN-02: No `timeout-minutes` on any job

**File:** `.github/workflows/full-pipeline.yml` (all jobs)
**Issue:** None of the jobs set `timeout-minutes`. A hang in `make test-integration` (e.g. `docker compose up -d --wait postgres` never reaching healthy) would run until GitHub's default 360-minute job ceiling before failing, burning Actions minutes on every occurrence rather than failing fast.
**Fix:** Set a conservative `timeout-minutes` (e.g. 15-20) on each job.

---

_Reviewed: 2026-08-12T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
