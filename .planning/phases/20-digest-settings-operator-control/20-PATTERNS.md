# Phase 20: Digest Settings & Operator Control - Pattern Map

**Mapped:** 2026-09-11
**Files analyzed:** 9
**Analogs found:** 9 / 9

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/db/migrations/000008_notification_settings.up.sql` | migration | CRUD (schema) | `internal/db/migrations/000005_artists_art_match_attempted_at.up.sql` | role-match (additive ALTER, not CREATE — see note) |
| `internal/db/migrations/000008_notification_settings.down.sql` | migration | CRUD (schema) | `internal/db/migrations/000005_artists_art_match_attempted_at.down.sql` | role-match |
| `queries/notification_settings.sql` | model (sqlc query file) | CRUD | `queries/watchlist.sql` (`UpdateWatchlistPreferences`, `CountWatchlist`) | exact |
| `internal/settings/settings.go` | service (Store) | CRUD, request-response | `internal/watchlist/service.go` (sqlc-backed `Store`) shape; `internal/pollruns/pollruns.go` for narrow-seam doc-comment style | role-match (watchlist is the CRUD/sqlc analog; pollruns is the "narrow Get/Update seam" naming analog, but is in-memory, not sqlc-backed) |
| `internal/httpserver/settings.go` (new handler file) | controller | request-response | `internal/httpserver/watchlist.go` (`handleUpdateWatchlist`) + `internal/httpserver/status.go` (`handleStatus`, `StatusDeps`/`Option` wiring) | exact (combination of both) |
| `internal/httpserver/server.go` (edit: `registerDataRoutes`, `Server` struct, `Option`/`serverConfig`) | route/config | request-response | same file's existing `WithStatus`/`statusStore`/`watchlistCounter` wiring | exact |
| `cmd/server/main.go` (edit: composition root) | config | request-response | existing `watchlist.NewService(...)` / `httpserver.WithStatus(...)` wiring block | exact |
| `web/app/lib/api.ts` (edit: add types + wrappers) | service (API client) | request-response | `StatusResponse`/`getStatus()` and `updateWatchlistPreferences()` in same file | exact |
| `web/app/components/system/DigestSettings.tsx` (new) | component | request-response | `web/app/components/system/AboutInstance.tsx` | exact (same `dl` layout, same Card chrome) |
| `web/app/routes/system.tsx` (edit) | route/page | request-response | itself — extend existing `Promise.all` fetch, `SystemSkeleton`, keep-stale (`refreshError`) pattern | exact |

## Pattern Assignments

### `internal/db/migrations/000008_notification_settings.up.sql` (migration)

**Analog:** `internal/db/migrations/000005_artists_art_match_attempted_at.up.sql` (comment style / rationale-first convention) — but this migration is a `CREATE TABLE`, not an `ALTER`, so also check `internal/db/migrations/000002_watchlist.up.sql`'s inline-`CHECK` convention (`watchlist_release_types_valid` style) referenced directly in CONTEXT.md D-05.

**Comment convention** (from 000005 lines 1-8): a short rationale block above the DDL, one design-doc/decision reference (e.g. `D-05`), no multi-paragraph headers.

**Shape required by CONTEXT.md/ROADMAP (locked, not discretionary):**
```sql
CREATE TABLE notification_settings (
    id                  int PRIMARY KEY CHECK (id = 1),
    digest_enabled      boolean NOT NULL DEFAULT false,
    digest_cadence      text NOT NULL DEFAULT 'daily'
        CHECK (digest_cadence IN ('daily', 'weekly')),
    digest_last_sent_at timestamptz,
    updated_at          timestamptz NOT NULL DEFAULT now()
);

INSERT INTO notification_settings (id) VALUES (1);
```
Read `internal/db/migrations/README.md` before finalizing — it governs N-1 rollback safety and the `allow-destructive` annotation syntax; this migration (additive CREATE + seed INSERT) should need no such annotation, matching 000005's zero-finding precedent.

**Down migration** — mirror 000005's one-line down:
```sql
DROP TABLE notification_settings;
```

---

### `queries/notification_settings.sql` (model, CRUD)

**Analog:** `queries/watchlist.sql`

**Simple single-row read pattern** (mirrors `CountWatchlist`, lines 58-62):
```sql
-- name: GetNotificationSettings :one
SELECT * FROM notification_settings WHERE id = 1;
```

**Full-row update pattern** (mirrors `UpdateWatchlistPreferences`'s `:one` + `RETURNING`, lines 21-56, simplified — this phase's PUT is full-object, not partial, per the UI-SPEC Data Contract, so no CASE/boolean-flag partial-update machinery is needed):
```sql
-- name: UpdateNotificationSettings :one
UPDATE notification_settings
SET digest_enabled = $1,
    digest_cadence  = $2,
    updated_at      = now()
WHERE id = 1
RETURNING *;
```
Comment discipline: one line explaining *why* it's a plain positional UPDATE (full-object PUT, D-05 singleton, no partial-update ambiguity to resolve) rather than watchlist's CASE-based merge — do not copy the CASE pattern, it solves a problem (partial update semantics) this phase does not have.

---

### `internal/settings/settings.go` (service)

**Analog A — sqlc-backed Store shape:** `internal/watchlist/service.go` lines 1-20 (imports), 95-104 (`Store` interface, narrow surface), 141-164 (`Service` struct + `NewService`, no functional options needed here since there's no optional dependency like `ArtistMatcher`).

**Imports pattern** (watchlist/service.go lines 8-20):
```go
import (
	"context"
	"fmt"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
)
```

**Store interface pattern** (watchlist/service.go lines 95-104, narrowed to this phase's two operations):
```go
// Store is the minimal surface internal/httpserver needs for the digest
// settings resource -- mirrors watchlist.Store / httpserver.Pinger.
type Store interface {
	Get(ctx context.Context) (Settings, error)
	Update(ctx context.Context, p UpdateParams) (Settings, error)
}
```

**Service struct + constructor** (watchlist/service.go lines 141-162, minus the `Option` machinery — no optional dependency exists for settings):
```go
type Service struct {
	q sqlc.Querier
}

func NewService(q sqlc.Querier) *Service {
	return &Service{q: q}
}

var _ Store = (*Service)(nil)
```

**Error wrapping convention** (watchlist/service.go line 275, 371): `fmt.Errorf("update notification settings: %w", err)` — never bare `err`.

**Analog B — narrow-seam doc-comment convention:** `internal/pollruns/pollruns.go` lines 1-3, 59-65 (package doc + "mirrors X" cross-reference style). Since D-05 already locked the single-row-no-cache design, `settings.Store`'s own package comment should state plainly (1-3 lines) that it is a trivial PK-indexed read/write with no cache, referencing `internal/pollruns.Store`'s narrow-seam shape as the pattern followed, per CONTEXT.md's own framing.

---

### `internal/httpserver/settings.go` (new handler file, controller)

**Analog A — PATCH-with-DTO handler shape:** `internal/httpserver/watchlist.go` lines 280-341 (`handleUpdateWatchlist`).

**Request DTO + decode + bounded body pattern** (watchlist.go lines 280-314, adapted — this phase's PUT sends both fields always, so no `*[]string` partial-optionality needed, just a straightforward DTO):
```go
type updateSettingsRequest struct {
	DigestEnabled bool   `json:"digest_enabled"`
	DigestCadence string `json:"digest_cadence"`
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes) // reuse or define a small const
	var req updateSettingsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// validate req.DigestCadence against the CHECK's allow-list before calling Update
	...
}
```

**Error-branch switch pattern** (watchlist.go lines 320-336): `errors.Is` cascade to specific 400s, fallback to `httplog.SetAttrs` + fixed 500 body — never echo raw `err.Error()` except for client-caused validation sentinels.

**Response encoding pattern** (watchlist.go lines 338-340):
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusOK)
_ = json.NewEncoder(w).Encode(entry)
```

**Analog B — GET handler + Option/Deps wiring:** `internal/httpserver/status.go` lines 1-13 (imports), 31-40 (`StatusDeps` struct convention for a related-fields bundle), 86-99 (`handleStatus`'s nil-dependency 503 guard + single read + write). Follow the same "`if s.settingsStore == nil { 503 }`" defensive guard if settings is wired as an optional `Option`, matching `handleStatus`'s `s.statusStore == nil || s.watchlistCounter == nil` check (status.go line 87).

**errorResponse / writeError reuse** — do not redefine; both already live in `internal/httpserver/watchlist.go` lines 52-60 and are shared package-wide.

---

### `internal/httpserver/server.go` (edit)

**Analog:** the existing `WithStatus` `Option` + `Server`/`serverConfig` field wiring (lines 31-44, 55-59, 106-112, 161-165) and the `registerDataRoutes` route list (lines 244-258).

**Option pattern to copy** (server.go lines 106-112):
```go
func WithSettings(store settings.Store) Option {
	return func(c *serverConfig) {
		c.settingsStore = store
	}
}
```

**Route registration** — add inside `registerDataRoutes` (server.go line 248 onward), directly beside the existing `/watchlist` and `/status` lines:
```go
r.Get("/settings/notifications", s.handleGetSettings)
r.Put("/settings/notifications", s.handleUpdateSettings)
```
This inherits `gate.Authenticate`, `X-Instance-Gated`, and `RequireCSRFHeader` purely by virtue of being registered inside `registerDataRoutes` — no new middleware wiring needed, per ROADMAP's explicit instruction.

---

### `cmd/server/main.go` (edit)

**Analog:** the existing `watchlist.NewService(sqlc.New(pool), ...)` (line 230) → `httpserver.New(..., httpserver.WithStatus(httpserver.StatusDeps{...}))` (lines 260-273) composition sequence.

```go
settingsStore := settings.NewService(sqlc.New(pool))
...
srv := httpserver.New(pool, store, eventsStore, ...,
	httpserver.WithSettings(settingsStore),
	...
)
```

---

### `web/app/lib/api.ts` (edit)

**Analog A — wire type mirroring a Go response struct exactly:** `StatusResponse`/`StatusInstance` (lines 107-151) — same discipline: one field-for-field TS interface per Go JSON struct, with a comment naming the exact backing Go file.

```ts
export interface NotificationSettings {
  digest_enabled: boolean
  digest_cadence: "daily" | "weekly"
  digest_last_sent_at: string | null
  updated_at: string
}
```

**Analog B — simple GET wrapper:** `getStatus()` (lines 347-349):
```ts
export async function getDigestSettings(): Promise<NotificationSettings> {
  return apiFetch<NotificationSettings>("/settings/notifications")
}
```

**Analog C — write wrapper with JSON body, full-object semantics (not partial like `updateWatchlistPreferences`):** modeled on `addWatchlist` (lines 262-280) rather than `updateWatchlistPreferences` (lines 285-297), since both fields are always sent:
```ts
export async function updateDigestSettings(params: {
  digestEnabled: boolean
  digestCadence: "daily" | "weekly"
}): Promise<NotificationSettings> {
  return apiFetch<NotificationSettings>("/settings/notifications", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      digest_enabled: params.digestEnabled,
      digest_cadence: params.digestCadence,
    }),
  })
}
```
`apiFetch` already injects the `X-Requested-With` CSRF header on any non-GET (lines 181-185) and the 401 interceptor (lines 206-209) — no per-wrapper auth code needed, exactly as `updateWatchlistPreferences` needs none.

---

### `web/app/components/system/DigestSettings.tsx` (new component)

**Analog:** `web/app/components/system/AboutInstance.tsx` (entire file, 79 lines) — copy its `Card`/`CardHeader`/`CardContent` + `dl grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2` structure verbatim per UI-SPEC's explicit instruction ("Internal layout mirrors `AboutInstance`'s definition-list pattern exactly").

**Imports pattern** (AboutInstance.tsx lines 1-6):
```tsx
import { Card, CardContent, CardHeader } from "~/components/ui/card"
import { formatAbsoluteTime, formatIsoTitle } from "~/lib/format"
import type { NotificationSettings } from "~/lib/api"
```
Plus `Switch` (`~/components/ui/switch`) and the newly-vendored `Select`/`SelectTrigger`/`SelectValue`/`SelectContent`/`SelectItem` (`~/components/ui/select`), per UI-SPEC's Component Inventory.

**Row-per-`dt`/`dd` pattern** (AboutInstance.tsx lines 34-75) — reuse directly for the three rows (`Digest mode`, `Cadence`, `Last digest sent`), substituting `Switch`/`Select`/`<time>` for the value cells per UI-SPEC's Copywriting Contract (exact locked strings: `Digest notifications`, `Digest mode`, `Cadence`, `Last digest sent`, `Never sent yet`, `Saved.`, `Couldn't save — reverted to the previous value.`).

**Timestamp rendering** — reuse `formatAbsoluteTime`/`formatIsoTitle` (`web/app/lib/format.ts` lines 88-122) exactly as `SourcePanel` does for `finished_at`, per UI-SPEC's Copywriting Contract row for "Last digest sent". Do NOT use the em-dash fallback path for `digest_last_sent_at === null` — UI-SPEC explicitly overrides `format.ts`'s own EM_DASH convention with the literal string `Never sent yet` for this one field (see UI-SPEC "Rows" table note); the em-dash special case is `format.ts`'s internal null-handling and must be bypassed by checking `=== null` in the component before calling the formatter, not by modifying `format.ts`.

**Instant-apply + keep-stale-on-failure pattern:** no direct existing component analog (this is new to the codebase — first instant-apply-with-revert control), but the *posture* is `system.tsx`'s own `refreshError`/`handleRefresh` re-entrancy-guard pattern (see below) applied to a write instead of a read.

---

### `web/app/routes/system.tsx` (edit)

**Analog:** itself — extend existing patterns rather than introducing new ones.

**Fetch-on-mount extension** (system.tsx lines 112-141): change the mount effect's single `getStatus()` call to `Promise.all([getStatus(), getDigestSettings()])`, matching UI-SPEC's explicit instruction; both results land in state together so a single `loadError` still covers both.

**Re-entrancy guard pattern to copy for the save call** (system.tsx lines 105-110, 154-173 `handleRefresh`): use the same `useRef` re-entrancy boolean + `mountedRef` guard shape for the digest `PUT`, so a second toggle/dropdown change can't race an in-flight save — this is exactly what UI-SPEC's "A save is in flight → both controls disabled" requires.

**Keep-stale error line pattern** (system.tsx lines 215-224, `refreshError` block): copy this exact `role="status" aria-live="polite" text-label text-destructive` structure for `DigestSettings`'s own inline error line (D-03), with the locked copy `Couldn't save — reverted to the previous value.` instead of the refresh message.

**SystemSkeleton extension** (system.tsx lines 57-91): insert one more `<Card>` skeleton block (3 short bars, per UI-SPEC) directly after the About-block skeleton (lines 60-71) and before the two source-panel skeletons (lines 73-88).

**Card placement** (system.tsx lines 240-246): render `<DigestSettings ... />` directly after `<AboutInstance ... />` and before the `watchlist_size === 0` `<Alert>`, per UI-SPEC's Visual Hierarchy section.

## Shared Patterns

### Error response shape
**Source:** `internal/httpserver/watchlist.go` lines 52-60 (`errorResponse`, `writeError`)
**Apply to:** `handleGetSettings`/`handleUpdateSettings` — reuse the existing package-level helper, do not redefine.

### `httplog.SetAttrs` for internal-error logging without leaking to the client
**Source:** `internal/httpserver/watchlist.go` line 333, `internal/httpserver/status.go` line 96/104
**Apply to:** any unexpected DB error path in `handleUpdateSettings`/`handleGetSettings` — log the real error server-side, return the fixed `"internal error"` string to the client.

### Consumer-declared narrow Store seam
**Source:** `internal/watchlist/service.go` lines 95-104 (`Store` interface), `internal/httpserver/status.go` lines 15-29 (`WatchlistCounter`/`StatusStore` interfaces declared in the consumer package)
**Apply to:** `internal/settings.Store` interface declaration, and `internal/httpserver`'s own consumption of it (declare a local `SettingsStore` interface in `httpserver` mirroring `StatusStore`, per CONVENTIONS.md's "narrow consumer-declared seams" rule) rather than importing `settings.Store` directly into the handler if a narrower shape suffices.

### Functional-option server wiring
**Source:** `internal/httpserver/server.go` `WithStatus` (lines 106-112) / `WithArtistArt` in `internal/watchlist/service.go` (lines 130-139)
**Apply to:** the new `httpserver.WithSettings(store)` option and its `cmd/server/main.go` call site.

### apiFetch / ApiError / 401 interceptor / CSRF header
**Source:** `web/app/lib/api.ts` lines 174-230
**Apply to:** `getDigestSettings`/`updateDigestSettings` — call through `apiFetch`, add zero custom auth/error-handling code, exactly as every existing wrapper does.

### Explicit-copy-never-blank convention
**Source:** `web/app/lib/format.ts` (`EM_DASH` fallback throughout) and `web/app/routes/system.tsx`'s `deriveLoadedShape`/first-run copy
**Apply to:** `DigestSettings`'s `Never sent yet` literal — an intentional *deviation* from the em-dash convention (per UI-SPEC), not a bug; document this in a 1-line comment in the component so a future reader doesn't "fix" it to match `format.ts`.

## No Analog Found

None — every file in scope has at least a role-match analog in the current codebase.

## Metadata

**Analog search scope:** `internal/pollruns/`, `internal/watchlist/`, `internal/httpserver/`, `internal/db/migrations/`, `queries/`, `web/app/lib/`, `web/app/components/system/`, `web/app/routes/`
**Files scanned:** ~15 (read fully or in targeted ranges)
**Pattern extraction date:** 2026-09-11
