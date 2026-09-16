---
phase: quick-260817-cfu
plan: 01
type: execute
wave: 1
depends_on: []
files_modified:
  - Dockerfile
autonomous: true
requirements: [CICD-04]
user_setup: []

estimate:
  tokens: 36000
  raw_tokens: 24000
  tasks: 2
  confidence: low   # 0 calibration samples for this project; default factor applied

must_haves:
  truths:
    - "`docker build .` completes through the go-build stage against the new pinned builder image — the digest resolves against the live registry and is not fabricated."
    - "A Trivy image scan of the freshly built image with `--severity CRITICAL,HIGH --exit-code 1` (the exact gate CI applies) exits 0."
    - "All 8 previously-reported Go stdlib CVEs are absent from the scan output."
    - "The Go binary embedded in the final image reports a stdlib version of go1.26.6 or later."
    - "The web-build and runtime stages' pinned images are byte-for-byte unchanged."
    - "Exactly one `FROM golang:` line exists and it carries both a tag and an `@sha256:` digest — the pin form is preserved, not loosened to a bare tag."
  artifacts:
    - "Dockerfile line 41 updated to `FROM golang:1.26.6-alpine3.24@sha256:<index-digest> AS go-build`"
  key_links:
    - "Dockerfile go-build stage -> the compiled /usr/local/bin/server binary in the runtime stage: Trivy reads the Go stdlib version stamped into that binary's build info, so the builder image is the only lever that moves those CVEs."
    - "go.mod `go 1.26` directive -> builder image minor version: the builder must stay >= 1.26, which is why the 1.25.13 fix line is not an option."
    - ".github/workflows/full-pipeline.yml build-scan job -> this Dockerfile: CI builds the whole file and scans the final image, so a green local scan is the pre-flight for a green CI gate."
---

<objective>
Bump the Dockerfile's Go builder-stage base image from its current 1.26.5 pin to `golang:1.26.6-alpine3.24`, pinned by a real registry-resolved sha256 digest, clearing the 8 HIGH-severity Go stdlib CVEs that are failing the `build-scan` job's Trivy gate in CI.

Purpose: CI's `build-scan` job is red and blocking the pipeline. The failure is not application code — Trivy reads the Go stdlib version stamped into the compiled `/usr/local/bin/server` binary's build info, and that version is entirely determined by the builder-stage image. This restores requirement CICD-04's gate to green without weakening it.

Output: A one-line change to `Dockerfile`, verified by a real `docker build` and a real Trivy scan run locally under CI's exact severity gate.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@Dockerfile
@.github/workflows/full-pipeline.yml
</context>

<planning_findings>
Verified during planning — the executor should not re-derive these, only act on them:

- **Target version is `1.26.6-alpine3.24`.** Confirmed present on Docker Hub. It is the newest 1.26.x and fixes all 8 reported CVEs.
- **1.25.13 is not an option.** `go.mod` declares `go 1.26`; a 1.25.x toolchain cannot build this module.
- **1.27 is not an option.** Only `1.27rc3` / `1.27-rc` tags exist — release candidates, not appropriate for a pinned production build.
- **No other file needs changing.** All four `setup-go` steps in `full-pipeline.yml` use `go-version-file: go.mod`, which resolves `go 1.26` to the latest available 1.26.x. The Dockerfile digest pin is the only place a specific patch release is named, so there is no version-drift counterpart to update.
- **CI's exact gate** (`full-pipeline.yml` lines 148-153): `aquasecurity/trivy-action` against `image-ref: drop-tracker:scan` with `severity: CRITICAL,HIGH` and `exit-code: '1'`. The local scan must mirror these flags exactly — do not add `--ignore-unfixed` or narrow the scanners, or a locally-green scan will not predict CI.
- **`trivy` is not on PATH on this machine; Docker 29.6.2 is.** Run Trivy as a container.
</planning_findings>

<tasks>

<task type="auto">
  <name>Task 1: Resolve the real index digest for golang:1.26.6-alpine3.24 and repin the go-build stage</name>
  <files>Dockerfile</files>
  <precondition>The Docker daemon is running and can reach docker.io — `docker version` reports both a Client and a Server section.</precondition>
  <action>
Resolve the digest from the live registry first, then edit. Never hand-write, guess, or carry over a digest from any document — including this plan, which deliberately does not contain one. A wrong digest either fails every future build or silently pins an unintended image.

Resolve the **multi-arch index (manifest list) digest**, not a platform-specific manifest digest. This is the load-bearing distinction: the two sibling pins already in this file (node, alpine) are index digests, a platform digest would silently drop multi-arch resolution, and the two forms are indistinguishable by inspection once written. Preferred command, which returns the index digest directly:

    docker buildx imagetools inspect golang:1.26.6-alpine3.24

Read the `Digest:` value from the top-level `Name:`/`MediaType:`/`Digest:` block — the one whose MediaType is an image *index*, not any of the per-platform `Manifests:` entries below it. Fallback if buildx is unavailable: `docker pull golang:1.26.6-alpine3.24` then `docker inspect --format='{{index .RepoDigests 0}}' golang:1.26.6-alpine3.24`, which also yields the index digest.

Sanity-check the resolved value before using it: it must start with `sha256:` followed by 64 hex characters, and it must differ from the digest currently on the FROM line.

Then edit the single `FROM golang:` line (currently line 41) so both the tag and the digest name the new image, keeping the `AS go-build` alias and the `tag@sha256:` pin form intact.

Change nothing else. The `node:26-alpine3.24` stage-1 pin, the `alpine:3.24` stage-3 pin, every comment, and every other instruction stay byte-for-byte identical — this is a one-line diff. Do not add a comment recording the old version; the git history already carries it.
  </action>
  <verify>
    <automated>grep -v '^#' Dockerfile | grep -c '^FROM golang:1.26.6-alpine3.24@sha256:[0-9a-f]\{64\} AS go-build$'</automated>
    <automated>test "$(grep -c '^FROM ' Dockerfile)" = "3" &amp;&amp; git diff --numstat -- Dockerfile</automated>
  </verify>
  <done>
The first automated check outputs `1` — exactly one FROM line names the new tag with a well-formed 64-hex digest and the `AS go-build` alias.
The Dockerfile still has exactly 3 `FROM` lines, and `git diff --numstat -- Dockerfile` reports `1	1	Dockerfile` — one line added, one removed, nothing else touched.
The digest was read from live registry output in this session, not copied from any file.
  </done>
</task>

<task type="auto">
  <name>Task 2: Prove the build works and the 8 CVEs are gone under CI's exact Trivy gate</name>
  <files>Dockerfile</files>
  <precondition>Task 1 is complete and the Docker daemon can pull from docker.io and ghcr.io (Trivy fetches its vulnerability DB on first run).</precondition>
  <action>
Build the full image the same way CI does, which exercises the go-build stage as a strict subset:

    docker build -t drop-tracker:scan .

A digest mismatch fails immediately at FROM resolution, so a successful build is itself proof the pin is real.

Then run the scan under CI's exact gate. `trivy` is not installed locally, so run it as a container against the local daemon:

    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
      aquasec/trivy:latest image --severity CRITICAL,HIGH --exit-code 1 drop-tracker:scan

On Git Bash for Windows, MSYS path conversion mangles the socket path — prefix the command with `MSYS_NO_PATHCONV=1`, or run it from PowerShell. If socket mounting still fails, fall back to the tarball route, which needs no socket:

    docker save drop-tracker:scan -o "$TMPDIR/dt-scan.tar"
    docker run --rm -v "$TMPDIR:/work" aquasec/trivy:latest image \
      --input /work/dt-scan.tar --severity CRITICAL,HIGH --exit-code 1

Write that tarball to a directory outside the repository working tree. A saved image tar is hundreds of megabytes and must never land in the repo or in a commit. Delete it once the scan finishes.

Keep the Trivy flags exactly as written. Do not add `--ignore-unfixed`, do not restrict `--scanners`, do not lower the severity set — the point of this check is to predict CI's result, and any narrowing makes a green local run meaningless.

Read the scan output rather than trusting the exit code alone. Confirm by eye that none of the 8 reported CVEs appear: CVE-2026-33818, CVE-2026-39821, CVE-2026-46600, CVE-2026-56853, CVE-2026-56858, CVE-2026-56859, CVE-2026-56860, CVE-2026-56862.

If the scan reports *different* CRITICAL/HIGH findings that the 1.26.5 build did not have — for example newly-disclosed alpine OS package CVEs surfacing in the refreshed layer — stop and report them rather than silently expanding this task's scope. That is a separate decision about the runtime stage, which this task is explicitly scoped not to touch.

As a final direct confirmation of the thing Trivy actually measures, read the stdlib version stamped into the shipped binary's build info:

    docker create --name dt-verify drop-tracker:scan
    docker cp dt-verify:/usr/local/bin/server ./dt-server-check
    docker rm dt-verify
    go version -m ./dt-server-check | head -3

Expect `go1.26.6` or later. Remove `dt-server-check` afterward so no binary is left in the working tree. If a local `go` toolchain is unavailable, skip this confirmation — the Trivy result is the authoritative check.
  </action>
  <verify>
    <automated>docker build -t drop-tracker:scan . &amp;&amp; MSYS_NO_PATHCONV=1 docker run --rm -v /var/run/docker.sock:/var/run/docker.sock aquasec/trivy:latest image --severity CRITICAL,HIGH --exit-code 1 drop-tracker:scan</automated>
    <automated>git status --porcelain</automated>
    <human-check>After committing and pushing, confirm the GitHub Actions "Full Pipeline" run reaches a green `build-scan` job. CI is the authoritative gate; the local scan is the pre-flight.</human-check>
  </verify>
  <done>
`docker build` completes all three stages successfully.
The Trivy scan exits 0 with zero CRITICAL and zero HIGH findings, and none of the 8 named CVEs appear in its output.
`git status --porcelain` shows only the expected `Dockerfile` modification — no image tarball, no extracted binary, no stray artifact left behind.
The shipped binary reports a Go stdlib version of 1.26.6 or later (or this check was skipped for lack of a local toolchain, and noted as skipped).
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Docker Hub -> build stage | An external, third-party-controlled image becomes the toolchain that compiles the shipped binary. Everything it contains is inherited by the artifact. |
| Dockerfile -> CI build-scan gate | This file is the single build authority; CI builds it verbatim and gates publishing on the result. |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-CFU-01 | Tampering | Dockerfile `FROM golang:` digest | medium | mitigate | Digest is resolved from the live registry via `docker buildx imagetools inspect` in Task 1 and never hand-written; this plan intentionally contains no digest to copy. A successful `docker build` in Task 2 is the proof it resolves. |
| T-CFU-02 | Tampering | Base-image pin form | high | mitigate | The `tag@sha256:` pin form is preserved, not loosened to a bare tag. Task 1's grep gate requires a 64-hex digest on the line, so a tag-only reference — which would let the upstream image be silently substituted between builds — cannot pass. |
| T-CFU-03 | Elevation of Privilege | Stage-3 runtime image / non-root user | low | accept | Out of scope by constraint: the `alpine:3.24` pin, the `USER 10001:10001` directive, and the fixed UID/GID (07-CONTEXT.md D-02) are untouched. Task 1's 3-FROM-line and 1-line-diff gates enforce this. |
| T-CFU-04 | Information Disclosure | `docker save` tarball / extracted binary | low | mitigate | Task 2 writes the tarball outside the repo tree and deletes both it and the extracted binary; `git status --porcelain` is an acceptance gate specifically to catch a stray multi-hundred-megabyte artifact before commit. |
| T-CFU-SC | Tampering | Upstream base-image supply chain | high | mitigate | The new upstream image is scanned with CI's exact `CRITICAL,HIGH` / `exit-code 1` gate *before* the change is committed, so a bump that trades 8 known CVEs for a different set is caught locally rather than in CI. Task 2 explicitly requires stopping and reporting rather than absorbing unrelated new findings. |
</threat_model>

<verification>
1. `grep -v '^#' Dockerfile | grep -c '^FROM golang:1.26.6-alpine3.24@sha256:[0-9a-f]\{64\} AS go-build$'` outputs `1`.
2. `git diff --numstat -- Dockerfile` reports exactly `1	1	Dockerfile`.
3. `docker build -t drop-tracker:scan .` succeeds through all three stages.
4. Trivy image scan at `--severity CRITICAL,HIGH --exit-code 1` exits 0.
5. None of the 8 named CVEs appear in the scan output.
6. `git status --porcelain` shows only the `Dockerfile` modification.
7. (Human) The next GitHub Actions "Full Pipeline" run shows `build-scan` green.
</verification>

<success_criteria>
- The Dockerfile's go-build stage is pinned to `golang:1.26.6-alpine3.24` by a genuine registry-resolved index digest.
- The change is a strict one-line diff; the web-build and runtime stages are untouched.
- A local Trivy scan under CI's exact gate passes with zero CRITICAL/HIGH findings.
- Requirement CICD-04's gate is restored to a passing state without being weakened, narrowed, or bypassed.
</success_criteria>

<output>
Create `.planning/quick/260817-cfu-bump-the-dockerfile-s-go-builder-stage-b/260817-cfu-SUMMARY.md` when done.
</output>
