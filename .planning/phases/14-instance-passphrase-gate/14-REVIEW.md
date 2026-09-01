---
phase: 14-instance-passphrase-gate
reviewed: 2026-09-01T00:00:00Z
depth: standard
files_reviewed: 7
files_reviewed_list:
  - internal/authgate/gate.go
  - internal/authgate/gate_test.go
  - web/app/lib/api.ts
  - web/app/lib/api.test.ts
  - web/app/lib/authStore.ts
  - web/app/lib/authStore.test.ts
  - web/app/root.test.tsx
findings:
  critical: 0
  warning: 2
  info: 4
  total: 6
status: issues_found
---

# Phase 14: Code Review Report

**Reviewed:** 2026-09-01
**Depth:** standard
**Files Reviewed:** 7
**Status:** issues_found

## Summary

Plan 14-07 adds a self-identifying `X-Instance-Gated: 1` response marker in
`gate.Authenticate` and a monotonic one-shot client latch (`markGateActive`)
that `apiFetch` fires on the first marker-carrying response. The core
correctness properties hold up under scrutiny:

- **Monotonic / one-shot:** `markGateActive` early-returns on `gateActive`,
  so it flips state and notifies at most once per browser session. Verified
  against `authStore.test.ts` and `api.test.ts`.
- **Never resurrects a session after a 401:** `markGateActive` never reads or
  writes `authed`. In every interleaving of a stale marker-bearing 200 and a
  401 (either order, concurrent), `authed` ends up `false` and stays there.
  The marker is never emitted on the two 401 return paths in `Authenticate`.
- **sessionStorage throw-safety (WR-01 residual):** both `readPersistedGateActive`
  and `persistGateActive` now put the `typeof` probe inside the `try`, so a
  single `catch` covers the undeclared-identifier, throwing-method, and
  throwing-accessor cases. `authStore.test.ts` exercises all three.
- **Ungated instance never emits the marker:** confirmed structurally --
  `server.go:173` registers `gate.Authenticate` only inside the
  `WithAuthGate` branch, and the marker is set only inside that middleware.

Two WARNING-level gaps concern caching/replay of the new header and one
latch durability edge. Nothing rises to BLOCKER.

## Warnings

### WR-01: Gated responses carry the new marker with no `Cache-Control: no-store`

**File:** `internal/authgate/gate.go:200` (and absent server-wide -- `grep`
finds zero `Cache-Control` / `Vary` in any `.go` file)

**Issue:** `Authenticate` now stamps `X-Instance-Gated: 1` onto every
cookie-authenticated 2xx/4xx (including `GET /watchlist`, `GET /events`,
`GET /search`), but no gated response sets `Cache-Control: no-store` (or
`private`) and none sets `Vary: Cookie`. A shared/intermediary cache or a
misconfigured reverse proxy/CDN in front of the single binary can store one
user's authenticated `GET /watchlist` 200 -- body and the marker header --
and replay it to a different client. The pre-existing risk is the watchlist
body leak; 14-07 compounds it by also causing the recipient to latch
`gateActive` (persisted to their `sessionStorage` for the session),
permanently showing a "Log out" control they never earned. Because the marker
is designed to be discovered from "an ordinary authenticated 200", these
responses are exactly the ones most likely to look cacheable.

**Fix:** Set `no-store` on the gated path, ideally in the same middleware that
sets the marker so the two cannot drift:

```go
w.Header().Set(instanceGatedHeaderName, instanceGatedHeaderValue)
w.Header().Set("Cache-Control", "no-store")
```

Or add a small middleware on the protected `chi.Group` in `server.go`
alongside `pr.Use(gate.Authenticate)`.

### WR-02: `markGateActive` never retries a failed `persistGateActive`

**File:** `web/app/lib/authStore.ts:171-178`

**Issue:** `markGateActive` early-returns whenever `gateActive` is already
`true`, so it calls `persistGateActive()` exactly once -- on the transition.
If that single `setItem` throws (private-mode quota, transient policy denial)
the failure is swallowed and never retried, even though `markGateActive` runs
on every subsequent API response and would be the natural place to re-attempt
the write. By contrast `markAuthenticated` / `markUnauthenticated` call
`persistGateActive()` unconditionally and therefore self-heal. The result:
for a pure cookie-session user (no 401, no typed login) on a browser where
`getItem` works but `setItem` intermittently throws, the "Log out" control
disappears on every full document reload until the first API response
re-latches it. Low severity (narrow condition, cosmetic), but it is a real
regression from the always-retry behaviour of the other two `mark*` paths.

**Fix:** Attempt the persist even when the in-memory flag is already set, and
keep the guard only around `notify()` and the state assignment. Alternatively
accept the edge and document it explicitly as a known limitation next to the
method.

## Info

### IN-01: Structural "ungated instance" guarantee is asserted only in prose + tests

**File:** `internal/authgate/gate.go:118-130`, `:200`

**Issue:** `Authenticate` sets the marker unconditionally whenever it runs.
D-18's "an ungated instance emits no `X-Instance-Gated`" depends entirely on
`server.go:173` keeping `pr.Use(gate.Authenticate)` inside the `WithAuthGate`
branch. There is no type-level or assertion-level coupling; a future
middleware-registration refactor that hoists `Authenticate` (or copies the
`w.Header().Set`) would silently break D-18 with no compile or runtime error.
`gate_test.go`'s `TestGate_InstanceGatedMarker_AbsentOnUnauthenticatedAndUngated`
covers today's behaviour, which is the mitigation -- noted so a reviewer of a
future refactor knows the guarantee is load-bearing.

**Fix:** No change required now. If the middleware wiring is ever touched,
keep the D-18 sub-test green and treat it as a gate.

### IN-02: Latch silently no-ops if the API is ever served cross-origin

**File:** `web/app/lib/api.ts:146`

**Issue:** `res.headers.get("X-Instance-Gated")` returns `null` for a
cross-origin response unless the server also sends
`Access-Control-Expose-Headers: X-Instance-Gated`. This is fine today (single
binary, same origin, no CORS per CLAUDE.md), but if the API is ever split out
the latch fails closed -- no "Log out" control, no error. Worth a one-line
comment next to the marker constant so a future split doesn't lose the
behaviour quietly.

**Fix:** Add a note to the `INSTANCE_GATED_HEADER` comment block:
"same-origin only -- a cross-origin deployment must add
`Access-Control-Expose-Headers: X-Instance-Gated`."

### IN-03: Test matrix gaps on the marker's negative space

**File:** `internal/authgate/gate_test.go:775-825`

**Issue:** The negative matrix covers gated-401, gated-exempt-`/health`, and
ungated. Two behaviours are unpinned:
1. `POST` / `DELETE /session` (exempt, but non-GET) -- not asserted to omit
   the marker.
2. A CSRF 403 from `RequireCSRFHeader` does carry the marker (it runs after
   `Authenticate`, which already stamped `w`). This is arguably correct -- the
   caller is authenticated -- but no test documents the intended behaviour
   either way, and on the client that 403 runs `markGateActive` with no 401
   path.

**Fix:** Add a sub-test asserting the marker is absent on `POST /session` /
`DELETE /session`, and one asserting or documenting whether a post-Authenticate
403 carries it.

### IN-04: `apiFetch` reads a header on every response for a session-lifetime one-shot

**File:** `web/app/lib/api.ts:146-148`

**Issue:** Once `gateActive` latches, every subsequent `apiFetch` still does
`res.headers.get(...)` + string compare + a call into `markGateActive` that
early-returns. Negligible cost and out of v1 perf scope -- noted only because a
`if (!authStore.isGateActive())` short-circuit at the call site would make the
one-shot intent obvious at the point of use rather than only inside the store.

**Fix:** Optional:
```ts
if (!authStore.isGateActive() && res.headers.get(INSTANCE_GATED_HEADER) === INSTANCE_GATED_VALUE) {
  authStore.markGateActive()
}
```

---

_Reviewed: 2026-09-01_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
