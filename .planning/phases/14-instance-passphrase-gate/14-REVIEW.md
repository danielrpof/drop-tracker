---
phase: 14-instance-passphrase-gate
reviewed: 2026-08-31T00:00:00Z
depth: standard
files_reviewed: 4
files_reviewed_list:
  - cmd/server/main.go
  - cmd/server/main_test.go
  - docker-compose.yml
  - internal/config/config_test.go
findings:
  critical: 0
  warning: 4
  info: 2
  total: 6
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-08-31
**Depth:** standard
**Files Reviewed:** 4
**Status:** issues_found

## Summary

Reviewed the plan 14-05 (G-14-1 gap closure) changes: the `logInstanceGateStatus`
boot helper in `cmd/server/main.go`, its regression test in `cmd/server/main_test.go`,
the compose-wiring regression test in `internal/config/config_test.go`, and the
`docker-compose.yml` env forwarding.

The core secret-leakage concern is clean: `logInstanceGateStatus` takes the passphrase
as a parameter, never re-reads the environment, and on neither branch emits the value,
a substring, its length, or a hash. The GATE-07 inert path is unchanged — the only
functional edit to `main.go` is the added log call, which does not touch
`httpserver.WithAuthGate`.

However, the new `docker-compose.yml` `environment:` entries introduce a regression
path that can silently revert the gate to inert in a way the previous `env_file`-only
setup could not, and the three new regression tests each under-assert relative to the
invariant they claim to pin — most importantly, the `logInstanceGateStatus` guard test
does not lock out a future *hash* of the passphrase, which T-14G-01 explicitly forbids.

## Warnings

### WR-01: docker-compose `environment:` entry can clobber a valid `.env` passphrase with an empty string

**File:** `docker-compose.yml:72-73`
**Issue:** The added lines

```yaml
INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-}
TRUST_PROXY_HEADERS: ${TRUST_PROXY_HEADERS:-false}
```

sit in the `app` service `environment:` mapping, which outranks `env_file: .env`.
Compose variable interpolation resolves `${INSTANCE_PASSPHRASE}` from the host shell
and the project-directory `.env` — but that interpolation source is **not** always the
same file as `env_file: .env` (which is resolved relative to the compose file). When
`docker compose` is invoked from another directory, or with `--env-file`/
`--project-directory` pointing elsewhere, interpolation yields empty, and
`INSTANCE_PASSPHRASE:` is still emitted into the container as an empty string, which
overrides the correct value that `env_file: .env` would otherwise have loaded. Result:
an operator with a valid `INSTANCE_PASSPHRASE` in `.env` gets a silently **inert** gate.
The in-file comment (lines 58-62) only acknowledges the "shell exports empty string"
case, not this one. Before this change, `env_file: .env` alone would have delivered the
passphrase in that scenario. The new boot-status log is the only thing that surfaces it.
**Fix:** Either drop `INSTANCE_PASSPHRASE` from `environment:` and keep `env_file` as
the sole channel (host-shell forwarding can be documented via `.env` instead), or
document in the comment block that Compose interpolation reads the *project-directory*
`.env` and that running compose from any other directory reverts the gate to inert.
Prefer the former; the boot log already covers the "did the operator forget it" case
this line was added for.

### WR-02: compose-wiring test does not verify the env entries are scoped to the `app` service

**File:** `internal/config/config_test.go:415-463`
**Issue:** `TestDockerComposeWiresGateEnvVars` does a flat line scan for any non-comment
line beginning `INSTANCE_PASSPHRASE:` / `TRUST_PROXY_HEADERS:`. It never checks that the
match sits under `services.app.environment`. A future edit that relocates these lines
under `services.postgres`, under `build.args`, or to a top-level `x-` anchor would keep
the test green while breaking the container wiring the test's own docstring says it
guards ("in the app service's environment mapping"). The docstring overclaims what the
assertion enforces.
**Fix:** Track the current service/section during the scan (indentation-based, still no
YAML dep): only accept the match when the most recent `services:` child seen is `app:`
and the most recent second-level key is `environment:`. Alternatively, assert the line's
leading indentation matches the `DATABASE_URL:` line already known to be in that block.

### WR-03: `logInstanceGateStatus` guard test does not lock out a passphrase hash / added attributes

**File:** `cmd/server/main_test.go:77-99`
**Issue:** T-14G-01 forbids logging the passphrase, "a substring, or its length" — and
per 14-RESEARCH also a hash. The active-branch test asserts only (a) exactly one record,
(b) the record mentions `"active"`, (c) it does not contain the literal passphrase, and
(d) it does not contain the decimal rune count. A regression that added
`"fingerprint", sha256Hex(passphrase)` or `"prefix", passphrase[:4]` would pass every
one of these assertions (a hex digest is neither the literal value nor the rune-count
string). The test does not pin the record's attribute set.
**Fix:** After `delete(rec, "time")`, assert the key set exactly, e.g.:

```go
gotKeys := make([]string, 0, len(rec))
for k := range rec {
    gotKeys = append(gotKeys, k)
}
sort.Strings(gotKeys)
want := []string{"level", "msg", "service", "status"}
if !reflect.DeepEqual(gotKeys, want) {
    t.Errorf("active-branch record has unexpected attributes: got %v, want %v", gotKeys, want)
}
```

This makes any future added attribute (hash, prefix, entropy score, etc.) fail the test.

### WR-04: passphrase-line assertion accepts a hardcoded interpolation default

**File:** `internal/config/config_test.go:445-448`
**Issue:** The check is `strings.Contains(passphraseLine, "${INSTANCE_PASSPHRASE")`, which
also passes for `INSTANCE_PASSPHRASE: ${INSTANCE_PASSPHRASE:-hunter2}` — i.e. a shared
default passphrase baked into the compose file. The `TRUST_PROXY_HEADERS` assertion is
stricter (it pins `:-false}`), so coverage is asymmetric. A careless edit adding a
default secret here would ship a predictable gate credential; `authgate.IsWeakPassphrase`
only WARNs (never fails boot) and only catches its `knownDefaults` list.
**Fix:** Assert the passphrase interpolation carries no non-empty default, e.g. require
the entry to be exactly `${INSTANCE_PASSPHRASE}` or `${INSTANCE_PASSPHRASE:-}` (reject
anything matching `\$\{INSTANCE_PASSPHRASE:-\S`).

## Info

### IN-01: `recordMentions(rec, "active")` is a substring match that also matches "inactive"

**File:** `cmd/server/main_test.go:89`, helper at `53-63`
**Issue:** `recordMentions` uses `strings.Contains`. Asserting the active-branch record
"mentions `active`" would also be satisfied by a message reworded to "gate is inactive"
or "deactivated". The intent is to check the status token specifically.
**Fix:** Assert the structured field directly: `if rec["status"] != "active" { ... }`
(and `"inert"` for the other branch), which is exact and also documents the contract.

### IN-02: inert-branch test does not pin the remediation hint precisely

**File:** `cmd/server/main_test.go:113-118`
**Issue:** The inert-branch test checks the record "mentions `.env`", satisfied by any
value containing that substring (including an unrelated future attribute). It does not
verify the `hint` attribute specifically names the `.env` KEY=VALUE remediation channel
that T-14G-01's inert-path requirement calls for.
**Fix:** Read `rec["hint"]` and assert it contains both `INSTANCE_PASSPHRASE` and `.env`,
so the "name the channel that actually works" requirement is what's pinned.

---

_Reviewed: 2026-08-31_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
