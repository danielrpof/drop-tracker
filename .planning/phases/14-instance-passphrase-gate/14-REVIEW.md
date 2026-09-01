---
phase: 14-instance-passphrase-gate
reviewed: 2026-08-31T00:00:00Z
depth: standard
files_reviewed: 3
files_reviewed_list:
  - web/app/lib/authStore.ts
  - web/app/lib/authStore.test.ts
  - web/app/root.test.tsx
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-08-31
**Depth:** standard
**Scope:** Gap-closure delta 14-06 (closes G-14-2) — `8fac9ae~1..HEAD`
**Files Reviewed:** 3
**Status:** issues_found

## Summary

The 14-06 change adds `sessionStorage`-backed persistence of the presentation-only
`gateActive` flag so a full document reload (valid cookie, no 401, no login) still
renders the **Log out** control. The design is sound and the checked invariants hold:

- `isGateActive` / `isAuthed` snapshot getters return the cached module boolean and
  do **not** re-read storage — `useSyncExternalStore`'s stable-snapshot contract is
  respected (authStore.ts:118-119, 145-148).
- `authed` is never written to storage; only `gateActive` is persisted, via the
  single `persistGateActive` writer called from both `mark*` functions.
- The Node SPA-prerender path (`react-router build`, `ssr: false`) cannot hit a
  `ReferenceError`: `typeof sessionStorage === "undefined"` on an *undeclared* global
  is safe in Node and returns `"undefined"`, so `readPersistedGateActive()` returns
  `false` at module init.
- `gateActive` seeded `true` while `authed` stays optimistically `true` does not
  leak a Log out button onto `<PassphraseScreen>` — `LogoutButton` renders only
  inside the `authed` nav branch (root.tsx:105-118).
- Test isolation is correct: `sessionStorage.clear()` is the first statement in both
  `beforeEach` hooks, ahead of `vi.resetModules()` and the dynamic re-import, and
  `afterEach` restores stubbed globals.

One incomplete guard is worth fixing (WR-01) plus two minor test-quality notes.

## Warnings

### WR-01: `typeof sessionStorage` access sits outside the try/catch and can throw at module load

**File:** `web/app/lib/authStore.ts:80-88` (and the same shape at `92-102`)
**Issue:**
`readPersistedGateActive()` runs unguarded at module-init time (line 107) and its
first statement is `if (typeof sessionStorage === "undefined")`. `typeof` only
suppresses errors for *unresolvable* references. When `window.sessionStorage` is a
present-but-throwing getter — Firefox with `dom.storage.enabled=false`, an
enterprise storage policy, or (most realistically) the SPA rendered inside an
`<iframe sandbox>` without `allow-same-origin` — the getter throws `SecurityError`
and `typeof` propagates it. Because the `try`/`catch` starts *after* this line, the
exception escapes `readPersistedGateActive`, escapes the module initializer, and
fails the `import` of `authStore` — which `root.tsx` imports at the top level,
white-screening the whole SPA.

This is exactly the failure mode the file's own header comment claims to prevent
("a browser can deny or throw on storage access (private mode, disabled storage);
an unguarded read at module scope would white-screen the whole SPA"). The current
code only covers the `getItem`/`setItem`-throws and quota-exceeded cases, not the
property-access-throws case named in that comment. Not a BLOCKER because default
browser configs — including standard incognito/private mode — expose a working
`sessionStorage`, so the common paths are covered.

**Fix:** wrap the whole body, including the `typeof` probe, in `try`/`catch`:
```ts
function readPersistedGateActive(): boolean {
  try {
    if (typeof sessionStorage === "undefined") return false
    return sessionStorage.getItem(GATE_ACTIVE_STORAGE_KEY) === GATE_ACTIVE_STORAGE_VALUE
  } catch {
    return false
  }
}

function persistGateActive(): void {
  try {
    if (typeof sessionStorage === "undefined") return
    sessionStorage.setItem(GATE_ACTIVE_STORAGE_KEY, GATE_ACTIVE_STORAGE_VALUE)
  } catch {
    // storage denied, full, or access throws — in-memory boolean still carries the gate
  }
}
```
Add a test that stubs `sessionStorage` with a *throwing getter* (via
`Object.defineProperty` on the stub or `vi.stubGlobal` with a getter that throws),
not just throwing `getItem`/`setItem`, and asserts module import + `mark*` do not throw.

## Info

### IN-01: No test pins the "snapshot must not re-read storage" contract

**File:** `web/app/lib/authStore.test.ts` (new `describe` block, lines 102-191)
**Issue:**
The review brief and the source comment (authStore.ts:145-148) both call out that
`isGateActive()` must return the cached module boolean and never re-read
`sessionStorage` on each call. Every new test either reloads the module or calls a
`mark*` function; none asserts the negative — that writing `dt_gate_active` to
`sessionStorage` *after* import leaves `isGateActive()` returning `false` until a
reload or `mark*`. A future refactor that made the getter re-read storage would
pass the entire suite.
**Fix:** add a case:
```ts
it("does not re-read the session store after module load", () => {
  sessionStorage.setItem(GATE_ACTIVE_STORAGE_KEY, GATE_ACTIVE_STORAGE_VALUE)
  expect(authStore.isGateActive()).toBe(false) // still the load-time value
})
```

### IN-02: `root.test.tsx` regression test hardcodes the storage key/value literals

**File:** `web/app/root.test.tsx:191`
**Issue:**
`sessionStorage.setItem("dt_gate_active", "1")` uses bare literals, while
`authStore.test.ts` deliberately introduced `GATE_ACTIVE_STORAGE_KEY` /
`GATE_ACTIVE_STORAGE_VALUE` constants "so a change to what the implementation
writes is caught here as well as there." The `root.tsx` test is now the one place
the contract can silently drift.
**Fix:** export the two constants from `authStore.ts` and import them in both test
files (or at minimum add a matching `const` with a comment mirroring
`authStore.test.ts`).

---

_Reviewed: 2026-08-31_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
