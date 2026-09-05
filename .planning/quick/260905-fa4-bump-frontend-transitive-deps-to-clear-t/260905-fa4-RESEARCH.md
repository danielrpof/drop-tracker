# Quick Task 260905-fa4 — Bump frontend transitive deps to clear trivy-fs HIGH CVEs

**Researched:** 2026-09-05
**Domain:** pnpm dependency resolution / Trivy filesystem scanning / CI unblock
**Confidence:** HIGH — the fix was applied in a scratch copy and the post-fix lockfile was scanned with the same Trivy version CI runs; result was 0 findings, exit 0.

## Summary

All six findings come from `web/pnpm-lock.yaml` alone, and both packages are pure
build/tooling transitives reachable from production only because `shadcn` (the CLI)
sits in `dependencies`. The repo already has the exact mechanism for this fix in place —
`web/pnpm-workspace.yaml` carries an `overrides:` block added by commit `f6647ec`
("fix(07): bump vulnerable frontend deps found by the new trivy-fs gate") for the same
class of problem. This task is two lines in that block plus a lockfile regen.

**Primary recommendation:** add `browserslist: '^4.28.7'` and `fast-uri: '^3.1.6'` to the
existing `overrides:` block in `web/pnpm-workspace.yaml`, then regenerate the lockfile with
**`corepack pnpm@11.8.0 install --lockfile-only`** (the version the Dockerfile pins). Use
caret ranges, not `>=`. Verified end-to-end below.

## Key Discovery — the fix already has a home in this repo

`web/pnpm-workspace.yaml` lines 7–12, read verbatim this session
[VERIFIED: web/pnpm-workspace.yaml:7-12]:

```yaml
# Force transitive deps to fixed versions past known HIGH CVEs (07-REVIEW.md
# WR-01 -- trivy-fs found these; neither package is a direct dependency, so
# a version bump in package.json alone wouldn't reach them).
overrides:
  nanoid: '>=3.3.17'
  postcss: '>=8.5.18'
```

Do **not** add `pnpm.overrides` to `web/package.json` — `web/package.json` has no `pnpm`
key today [VERIFIED: web/package.json:1-49], pnpm 10+ reads `overrides` from
`pnpm-workspace.yaml` [CITED: pnpm.io/settings — "All other settings … must be configured
in `pnpm-workspace.yaml` or the global `~/.config/pnpm/config.yaml`"], and the repo
precedent settles it. Extend the existing block; a second `overrides:` key in the same
YAML file is a duplicate-key error.

## 1. Fixed versions — verified

Queried OSV (`api.osv.dev/v1/query`) and the npm registry live on 2026-09-05.

| Package | CVE / GHSA | Introduced | Fixed | Verdict |
|---|---|---|---|---|
| `browserslist` | CVE-2026-73088 / GHSA-73wf-gq98-2v4g | 0 | **4.28.7** | todo's claim correct |
| `browserslist` | CVE-2026-73089 / GHSA-c83g-rgw3-j3cx | 0 | **4.28.7** | todo's claim correct |
| `fast-uri` | CVE-2026-75899 / GHSA-fph4-wmhf-6fwf | 2.4.1 / 3.1.2 / 4.0.0 | 2.4.5, **3.1.6**, 4.1.3 | todo's claim correct |
| `fast-uri` | CVE-2026-75931 / GHSA-5jgf-p345-68v8 | 2.4.2 / 3.1.3 / 4.0.1 | 2.4.5, **3.1.6**, 4.1.3 | todo's claim correct |
| `fast-uri` | CVE-2026-75975 / GHSA-f65p-4m7j-42xc | 2.3.1 / 3.0.0 / 4.0.0 | 2.4.5, **3.1.6**, 4.1.3 | todo's claim correct |
| `fast-uri` | CVE-2026-76172 / GHSA-jqff-g426-hqxp | 2.3.1 / 3.0.0 / 4.0.0 | 2.4.5, **3.1.6**, 4.1.3 | todo's claim correct |

[VERIFIED: OSV API query, 2026-09-05] — every fixed-version claim in the todo checks out.

**Latest safe versions in the wanted semver line** [VERIFIED: npm registry, `npm view`]:

- `browserslist` — latest is **4.28.9** (`time.modified = '2026-09-04T13:39:02.600Z'`).
  OSV query against `browserslist@4.28.9` returns **no vulns**. There is no 5.x.
- `fast-uri` — 3.x line ends at **3.1.7** (`dist-tags.three = "3.1.7"`); `latest` is
  `4.1.4`. OSV query against `fast-uri@3.1.7` returns **no vulns**.

**Does `ajv@8.20.0` already permit a fixed `fast-uri`?** Yes — a lockfile-only bump suffices:

```
$ npm view ajv@8.20.0 dependencies --json
{ "fast-deep-equal": "^3.1.3", "fast-uri": "^3.0.1", ... }
```

[VERIFIED: npm registry] `^3.0.1` admits `3.1.7`, so no override is strictly required to
reach a fixed version — but pnpm will not re-resolve an already-locked transitive without
being told to, so the override is still the mechanism that forces it. Same for
`browserslist`: `shadcn@4.16.2` declares `"browserslist": "^4.26.2"`
[VERIFIED: `npm view shadcn@4.16.2 dependencies`], which admits `4.28.9`.

## 2. Trivy dev-dependency behavior — this is load-bearing

`aquasecurity/trivy-action@v0.36.0` runs Trivy 0.70.0. For `pnpm-lock.yaml` at
**lockfileVersion `'9.0'`** [VERIFIED: web/pnpm-lock.yaml:1 — `lockfileVersion: '9.0'`],
Trivy **excludes devDependencies by default** [CITED: trivy.dev/latest/docs/coverage/language/nodejs/
— "By default, Trivy doesn't report development dependencies"; pnpm row of the feature
table reads "Excluded (lock file v9 version)"]. The opt-in knob is `--include-dev-deps`;
the CI job at `.github/workflows/full-pipeline.yml:109-115` does not set it.

**Consequence:** these six only fire because `shadcn` is in `dependencies`
[VERIFIED: web/package.json:25 — `"shadcn": "^4.16.2",`]. Reclassifying it would make
them vanish *without fixing anything*. I confirmed this empirically (see §3b).

## 3. Candidate fixes — all four tested or traced

Baseline reproduced locally with the exact CI scanner, scanning only the lockfile
[VERIFIED: `docker run aquasec/trivy:0.70.0 fs --scanners vuln --severity CRITICAL,HIGH`]:

```
pnpm-lock.yaml (pnpm)
Total: 6 (HIGH: 6, CRITICAL: 0)
browserslist  CVE-2026-73088/73089           4.28.2  -> 4.28.7
fast-uri      CVE-2026-75899/75931/75975/76172  3.1.5 -> 2.4.5, 3.1.6, 4.1.3
```

### (a) `overrides` in `web/pnpm-workspace.yaml` — **RECOMMENDED**

Tested: added `browserslist: '^4.28.7'` + `fast-uri: '^3.1.6'` to the existing block,
ran `corepack pnpm@11.8.0 install --lockfile-only`, rescanned.

- **Result: 0 vulnerabilities, Trivy exit code 0.** [VERIFIED: local Trivy 0.70.0 run]
- Resolves to `browserslist@4.28.9`, `fast-uri@3.1.7` — both OSV-clean.
- Blast radius: **66 changed lines**, confined to the browserslist family
  (`baseline-browser-mapping` 2.10.32→2.11.21, `caniuse-lite` 1.0.30001793→1.0.30001810,
  `electron-to-chromium` 1.5.362→1.5.422, `node-releases` 2.0.46→2.0.54,
  `update-browserslist-db` 1.2.3→1.3.2) plus the two target packages. No other package moved.
- `corepack pnpm install --frozen-lockfile --lockfile-only` against the new lockfile
  exits 0 [VERIFIED: local run], so CI's `frontend-test` install step will accept it.
- Correctness: actually removes the vulnerable code. Reversible: revert two YAML lines
  and the lockfile. Matches repo precedent `f6647ec`.

**Use `^`, not `>=`.** I first tested `fast-uri: '>=3.1.6'` and pnpm resolved it to
**`fast-uri@4.1.4`** — an unconstrained `>=` crosses the major boundary, and an override
bypasses the parent's declared range, so `ajv@8.20.0` (which asks for `^3.0.1`) would
silently receive a major-version-incompatible `fast-uri`. `^3.1.6` yields `3.1.7` and stays
inside ajv's declared range. [VERIFIED: two local `pnpm install --lockfile-only` runs]
(The existing `nanoid`/`postcss` overrides use `>=` safely only because no newer major
exists for either.)

### (b) Move `shadcn` to `devDependencies` — clears the gate, but does not fix anything

Tested: moved `shadcn` to `devDependencies` in a scratch `package.json`, re-resolved,
rescanned. **Result: 0 vulnerabilities** — while the lockfile still pins
`browserslist@4.28.2` and `fast-uri@3.1.5`. [VERIFIED: local Trivy 0.70.0 run]

This is Trivy's dev-dep exclusion doing the work. As the *only* fix it is security theater
on a job whose entire purpose is to gate the release path. It is, separately, a legitimate
correctness cleanup — `shadcn` is a CLI (`bin: dist/index.js`), invoked as
`npx shadcn@latest add button` per `web/README.md`, and is not imported by any app code.
**Recommendation: do it as a follow-up todo, not as this fix, and never instead of (a).**

### (c) Remove `shadcn` from the manifest entirely — out of scope

No tracked script, Makefile target, Dockerfile stage, or CI job invokes `shadcn`
[VERIFIED: `git ls-files | xargs grep -n shadcn` — hits only in `web/README.md`,
`web/components.json`, comments in `web/app/app.css` / `web/app/components/history/HistoryFilters.tsx`
/ `web/vitest.config.ts`, and the manifests/lockfiles]. So removal would work
(`pnpm dlx shadcn@latest add …` on demand) and would delete `fast-uri` from the tree
outright. But it drops the pinned CLI version and is a workflow change, not a CI unblock.
Defer.

### (d) Change the `trivy-action` config — reject

Adding an ignore file or `--skip-dirs web` weakens the gate globally for a fix that
takes two lines. Note there is a **second** `trivy-action@v0.36.0` at
`.github/workflows/full-pipeline.yml:583-589` scanning `image-ref: drop-tracker:scan`;
it is unaffected either way (the final Alpine stage carries no lockfile or `node_modules`).

## 4. pnpm overrides mechanics

**Exact edit** — extend the block at `web/pnpm-workspace.yaml:10-12`:

```yaml
overrides:
  nanoid: '>=3.3.17'
  postcss: '>=8.5.18'
  browserslist: '^4.28.7'
  fast-uri: '^3.1.6'
```

Bare package names are correct here; no nested `parent>child` selector is needed since
each package resolves to a single version in this tree
[VERIFIED: `grep '^  browserslist@\|^  fast-uri@' web/pnpm-lock.yaml` — one entry each].
pnpm supports `"foo": "^1.0.0"`, `"bar@^2.1.0": "3.0.0"`, and `"qar@1>zoo": "2"` selector
forms [CITED: pnpm.io/settings/dependency-resolution#overrides]; the bare form matches the
existing two entries.

**Regen command:** `corepack pnpm@11.8.0 install --lockfile-only`, run from `web/`.

- `--lockfile-only` updates `pnpm-lock.yaml` without touching `node_modules` — the whole
  edit takes ~4s and leaves the working `node_modules` alone.
- `pnpm dedupe` is the wrong tool: it collapses duplicate versions, it does not apply a
  new override.
- **Pin the pnpm version to 11.8.0.** The Dockerfile installs `pnpm@11.8.0`
  [VERIFIED: Dockerfile:29 — `RUN npm install -g pnpm@11.8.0`] and runs
  `pnpm install --frozen-lockfile` at Dockerfile:35 on `web/package.json web/pnpm-lock.yaml
  web/pnpm-workspace.yaml` (Dockerfile:34). Regenerating with the locally-installed
  **11.25.0** instead produced a **240-line** diff carrying **86 new
  `(supports-color@7.2.0)` peer-suffix annotations** on unrelated packages
  (`debug`, `express`, `body-parser`, `finalhandler`, …) — pure resolution-style drift.
  With `pnpm@11.8.0` the diff is 66 lines and the churn is **zero**.
  [VERIFIED: two local `install --lockfile-only` runs, diffed against the committed lockfile]
- CI's `frontend-test` uses `pnpm/action-setup version: 11`
  (`.github/workflows/full-pipeline.yml:126-137`) then `pnpm install --frozen-lockfile`.
  A lockfile written by 11.8.0 passes under any 11.x; the reverse is the risky direction.

## 5. Pitfalls

**Peer-dep breakage: none observed.** `--frozen-lockfile` validation passes, and
`update-browserslist-db` auto-bumped 1.2.3→1.3.2 to satisfy its own new peer range
(`browserslist: ^4.28.7`, replacing `>= 4.21.0`). pnpm 11's supply-chain policy check
(`✓ Lockfile passes supply-chain policies (565 entries)`) also passes on the new lockfile.

**`caniuse-lite` needs no companion bump.** It rides along as a browserslist transitive:
1.0.30001793 → 1.0.30001810, automatically. Same for `update-browserslist-db`.

**`internal/webassets/build/client/` may need regeneration.** Precedent commit `f6647ec`
re-ran `make web` after its dep bump because the content-hashed chunk filenames changed.
A `browserslist`/`caniuse-lite` bump can shift transpile targets, so the hashes may move
again. This is **not** CI-blocking: the Dockerfile "deliberately builds `web/` itself
rather than trusting the committed `internal/webassets/build/client/` tree (07-CONTEXT.md
D-10)" [VERIFIED: Dockerfile:7-9], no workflow job diffs that tree, and its sole purpose is
letting a Node-less clone `go build`. Still — run `make web` and commit the result if it
changes, to match precedent and keep the Node-less path honest. `fast-uri` is
shadcn-CLI-only and cannot affect the build output.

**Prettier and vitest are untouched.** The Definition of Done's frontend format gate is
`prettier --write/check "**/*.{ts,tsx}"` — YAML and JSON manifests are outside that glob,
so no reformat is required for this change. Nothing in vitest 4's runtime path imports
`browserslist` or `fast-uri`; `corepack pnpm --dir web test` should be unaffected, but run
it anyway per DoD.

**CRLF / `.gitattributes` (quick task 260901-lvn).** `web/pnpm-workspace.yaml` currently has
CRLF terminators in the working tree (`file` reports "ASCII text, with CRLF line
terminators") while `.gitattributes` declares `* text=auto eol=lf`
[VERIFIED: `git check-attr text eol -- web/pnpm-workspace.yaml` → `text: auto`, `eol: lf`].
`git status --short web/` is currently clean, because `text=auto` normalizes CRLF→LF on
compare. Editing the file with an LF-writing tool yields mixed endings on disk but a clean
2-line `git diff` — harmless. **Verify with `git diff --stat web/pnpm-workspace.yaml` after
editing: expect 2 insertions, not a whole-file rewrite.** If it shows a full-file diff,
rewrite the file preserving its existing CRLF terminators.

**Stale `web/package-lock.json` is tracked and Trivy scans it too.** It carries
`browserslist@4.28.8` and `fast-uri@3.1.6` [VERIFIED: web/package-lock.json:3237-3239,
4283-4285] — already past the fixed versions, so it contributes **zero** of the six
findings and needs no change here. But it is a second, drifting npm lockfile living beside
the authoritative pnpm one, and it is a live source of future Trivy findings nobody is
maintaining. **File a separate todo to delete it.**

## Verification plan for the executor

1. Edit `web/pnpm-workspace.yaml` — two lines into the existing `overrides:` block.
2. `cd web && corepack pnpm@11.8.0 install --lockfile-only` — expect a ~66-line lockfile diff.
3. `git diff --stat web/` — confirm only `pnpm-workspace.yaml` (+2) and `pnpm-lock.yaml`.
4. `cd web && corepack pnpm install --frozen-lockfile` (full install, refresh `node_modules`).
5. `cd web && corepack pnpm test` and `corepack pnpm exec prettier --check "**/*.{ts,tsx}"`.
6. `make web`; if `internal/webassets/build/client/` changed, stage it.
7. Local Trivy re-check (docker is available on this machine):
   `docker run --rm -v <dir-with-lockfile>:/scan aquasec/trivy:0.70.0 fs --scanners vuln
   --severity CRITICAL,HIGH --exit-code 1 /scan` → expect **0** findings, exit 0.
8. Backend DoD gates (`go vet`, `golangci-lint run`, `make test`, `make coverage-gate`,
   `make sqlc-check`) — unchanged by this edit but required before commit.
9. Scratch-branch push to confirm `trivy-fs` goes green and `build-scan` runs.

## Assumptions Log

| # | Claim | Risk if wrong |
|---|---|---|
| A1 | `make web` output may change under the browserslist/caniuse-lite bump | Only affects the Node-less-clone convenience tree; not CI-blocking (Dockerfile:7-9). Step 6 catches it either way. |
| A2 | pnpm 11.8.0-generated lockfile is accepted by CI's `pnpm/action-setup version: 11` (latest 11.x) | `frontend-test` fails on `--frozen-lockfile`; step 9 catches it before merge. Verified in the reverse direction (11.25.0 accepts the file). |

## Sources

**Primary (HIGH):**
- OSV API (`api.osv.dev/v1/query`) — all 6 CVE ranges and fixed versions, plus clean checks on `browserslist@4.28.9` and `fast-uri@3.1.7`
- npm registry via `npm view` — `browserslist` 4.28.9, `fast-uri` dist-tags, `ajv@8.20.0` and `shadcn@4.16.2` dependency ranges
- Local `aquasec/trivy:0.70.0` runs — baseline (6 HIGH) and post-fix (0, exit 0) on the real lockfile
- Local `corepack pnpm@11.8.0` / `pnpm@11.25.0 install --lockfile-only` runs — resolution and diff blast radius
- Repo files read this session: `web/pnpm-workspace.yaml`, `web/package.json`, `web/pnpm-lock.yaml`, `web/package-lock.json`, `Dockerfile`, `Makefile`, `.github/workflows/full-pipeline.yml`, `.gitattributes`; commit `f6647ec`

**Secondary (MEDIUM):**
- trivy.dev/latest/docs/coverage/language/nodejs/ — dev-dependency exclusion default (independently confirmed by the §3b experiment)
- pnpm.io/settings and pnpm.io/settings/dependency-resolution — overrides location and selector syntax
