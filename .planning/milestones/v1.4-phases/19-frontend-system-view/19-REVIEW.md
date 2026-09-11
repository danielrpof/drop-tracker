---
phase: 19-frontend-system-view
reviewed: 2026-09-10T00:00:00Z
depth: standard
files_reviewed: 19
files_reviewed_list:
  - web/app/app.css
  - web/app/components/system/AboutInstance.tsx
  - web/app/components/system/OutcomeBadge.test.tsx
  - web/app/components/system/OutcomeBadge.tsx
  - web/app/components/system/SourceHistoryTable.tsx
  - web/app/components/system/SourcePanel.tsx
  - web/app/components/ui/table.tsx
  - web/app/lib/api.test.ts
  - web/app/lib/api.ts
  - web/app/lib/format.test.ts
  - web/app/lib/format.ts
  - web/app/lib/sources.test.ts
  - web/app/lib/sources.ts
  - web/app/root.test.tsx
  - web/app/root.tsx
  - web/app/routes.ts
  - web/app/routes/system.test.tsx
  - web/app/routes/system.tsx
  - web/vitest.config.ts
findings:
  critical: 0
  warning: 2
  info: 1
  total: 3
status: issues_found
---

# Phase 19: Code Review Report

**Reviewed:** 2026-09-10T00:00:00Z
**Depth:** standard
**Files Reviewed:** 19
**Status:** issues_found

## Summary

Reviewed the System view feature (routes/system.tsx, its two presentational components, the OutcomeBadge classifier, format.ts/sources.ts helpers, the api.ts getStatus wrapper, and the supporting shadcn table primitive) plus root.tsx's logout/gate wiring, at standard depth. The implementation is unusually well-defended for edge cases — em-dash fallbacks for null/unparseable timestamps, an explicit re-entrancy guard on Refresh, a mounted-ref guard against post-unmount state writes, and a resilience-by-design default arm in `classifyOutcome` for unrecognized wire values. Traced every formatter's bucket-boundary arithmetic (relative-time minute/hour/day rounding, duration ms/s/m buckets, poll-interval pluralization) by hand against the accompanying tests and found the boundary math correct in every case checked.

Two gaps remain in the "must never throw / must never silently mislabel" guarantees this code explicitly claims for itself, both narrow but concrete. No security issues, no hardcoded secrets, no dangerous sinks (`eval`, `innerHTML`, `dangerouslySetInnerHTML`), no debug artifacts (`console.log`, `debugger`, `TODO`/`FIXME`), and no empty catch blocks were found across the reviewed files.

## Warnings

### WR-01: `classifyOutcome`'s "never throw" guarantee has a gap for a non-string `outcome`

**File:** `web/app/components/system/OutcomeBadge.tsx:20-23` (via `web/app/lib/api.ts:229`)
**Issue:** `getStatus()` resolves via `apiFetch<StatusResponse>`, whose generic core does `return (await res.json()) as T` (api.ts:229) — an unchecked type assertion with no runtime validation. `OutcomeBadge`'s own comment states the default arm "must degrade to a grey badge, never throw or echo the raw string," and `KnownOutcome` is deliberately widened to `KnownOutcome | (string & {})` specifically to accept an unrecognized *string* value from an N-1/N deploy. But `titleCase` only guards falsy values:
```ts
function titleCase(value: string): string {
  if (!value) return "Unknown"
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase()
}
```
If `run.outcome` is ever a truthy non-string (e.g. a JSON number or object — not preventable by TypeScript once a value has crossed the unchecked `as T` cast in api.ts), `value.charAt` throws a `TypeError` during render. That's a real crash for a code path whose entire stated purpose is to survive exactly this kind of wire drift without throwing. Realistically low-probability today (the Go handler marshals a string field), but the guard as written doesn't actually deliver on the comment's promise, and the cost of closing the gap is one line.
**Fix:**
```ts
function titleCase(value: unknown): string {
  if (typeof value !== "string" || !value) return "Unknown"
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase()
}
```

### WR-02: `SourceHistoryTable`'s cap caption silently mislabels an over-cap payload

**File:** `web/app/components/system/SourceHistoryTable.tsx:22,29-34`
**Issue:** `HISTORY_CAP = 50` is a frontend-local duplicate of a backend-owned invariant (the comment on line 124-125 of `api.ts` notes "history is allocated zero-length by the handler," implying the 50-entry cap is enforced server-side, not here). `captionText` branches on exact equality:
```ts
function captionText(count: number): string {
  if (count === HISTORY_CAP) {
    return "Showing the 50 most recent cycles for this source. Older history isn't retained — it lives only in the logs."
  }
  return `${count} cycle${count === 1 ? "" : "s"} recorded for this source since the last restart.`
}
```
If the backend cap is ever raised, lowered, or regresses to send more than 50 entries (a real risk precisely because there is no shared constant or type-level link between the two sides — the same class of "no compiler or runtime error on either side" drift the codebase's own `X-Instance-Gated` comment in api.ts explicitly worries about), `count === HISTORY_CAP` silently fails and the component prints the wrong claim: e.g. "53 cycles recorded for this source since the last restart" when in fact history *is* being truncated somewhere and the true count is unknown. This is an operator-trust surface (the whole point of D-09's "never lie to the operator" design principle applied elsewhere in this phase) presenting a confidently wrong number instead of degrading to the honest "not retained" copy.
**Fix:** Use `>=` so an unexpected overflow still degrades to the truncation-safe copy instead of a wrong exact count:
```ts
if (count >= HISTORY_CAP) {
  return "Showing the 50 most recent cycles for this source. Older history isn't retained — it lives only in the logs."
}
```

## Info

### IN-01: Inconsistent presence checks across `listEvents`' optional params

**File:** `web/app/lib/api.ts:246-248`
**Issue:**
```ts
if (params?.artistId != null) search.set("artist_id", String(params.artistId))
if (params?.eventType) search.set("event_type", params.eventType)
if (params?.cursor != null) search.set("cursor", params.cursor)
```
`artistId` and `cursor` use an explicit `!= null` check (correctly treating `0` as a valid artist id), but `eventType` uses a bare truthy check. Currently harmless since `""` is not a meaningful `eventType` value, but it's an inconsistent pattern in a file whose own comments elsewhere are careful about exactly this falsy-vs-null distinction (see the `artistId` case) — a future caller adding a new numeric or zero-like filter here is one copy-paste away from reintroducing the bug this file already fixed once.
**Fix:** Use the same `!= null` check for all three params for consistency, even though it's currently a no-op for `eventType`.

---

_Reviewed: 2026-09-10T00:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
