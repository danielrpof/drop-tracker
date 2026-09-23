# Phase 24: Artist Tags & Notes - Pattern Map

**Mapped:** 2026-09-22
**Files analyzed:** 18 (new + modified)
**Analogs found:** 16 / 18

**Comment discipline reminder:** every analog below carries this repo's dense, decision-tagged
comment style (D-xx / T-xx / WR-xx references, multi-paragraph rationale blocks). Copy the *code
shape* — validation ordering, error-translation pattern, optimistic-update structure, sqlc
query idioms — never the comment density. New code in this phase follows `.claude/CLAUDE.md`:
1–3 line comments, why not what, one design-doc reference (e.g. one `D-xx` or the ADR) where it
earns its place.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `internal/db/migrations/000010_tags_and_notes.up.sql` / `.down.sql` | migration | batch (DDL) | `internal/db/migrations/000008_notification_settings.up.sql` (table shape); `000002_watchlist.up.sql` (CHECK + FK precedent) | role-match |
| `internal/tags/service.go` (new package) | service | CRUD + transactional | `internal/watchlist/service.go` | exact |
| `internal/tags/service_test.go` | test | CRUD | `internal/watchlist/service_test.go` (not read this session; same package convention) | role-match |
| `internal/watchlist/service.go` (MODIFIED: `Entry.Tags`/`Note`, zip logic, `Remove` tag-survival) | service | CRUD | itself (existing file, extend in place) | exact |
| `internal/httpserver/tags.go` (new file: GET/PATCH/DELETE /tags, POST /tags/{id}/merge, POST/DELETE /watchlist/{id}/tags) | controller | request-response | `internal/httpserver/watchlist.go` | exact |
| `internal/httpserver/watchlist.go` (MODIFIED: `PUT /watchlist/{id}/note`) | controller | request-response | `internal/httpserver/settings.go` (`handleUpdateSettings`, PUT-with-full-replace shape) | exact |
| `internal/httpserver/server.go` (MODIFIED: route registration) | config/route | request-response | itself, `registerDataRoutes` (lines 261-278) | exact |
| `queries/tags.sql` (new file) | model/query | CRUD | `queries/watchlist.sql` | exact |
| `queries/watchlist.sql` (MODIFIED: `ListWatchlist` enrichment, `CreateWatchlistEntry` note param) | model/query | CRUD | itself | exact |
| `web/app/components/watchlist/TagChips.tsx` (new) | component | request-response (optimistic) | `web/app/components/watchlist/PreferenceToggles.tsx` | exact (optimistic pattern) + `Badge` (chip base) |
| `web/app/components/watchlist/ArtistNote.tsx` (new, note display + inline edit) | component | request-response (optimistic-ish, keep-on-failure) | `PreferenceToggles.tsx` (structure only — note's failure mode diverges, see below) | role-match |
| `web/app/components/watchlist/ManageTagsDialog.tsx` (new) | component | CRUD (list/rename/merge/delete) | no existing dialog-driven CRUD list in this codebase — nearest shape is `web/app/routes/watchlist.tsx`'s list-state-plus-fetch pattern | no analog (new shadcn `Dialog`) |
| `web/app/components/common/ConfirmDialog.tsx` (new, reusable) | component | request-response | no existing AlertDialog wrapper — `web/app/components/ui/select.tsx` is the closest "wrap a base-ui primitive as a project component" precedent | role-match (wrapping convention only) |
| `web/app/components/watchlist/WatchlistRow.tsx` (MODIFIED: mount `TagChips` + `ArtistNote` in name column) | component | request-response | itself | exact |
| `web/app/routes/watchlist.tsx` (MODIFIED: header button, tag vocabulary route state, `addTag`/`removeTag` updaters) | route/provider | request-response | itself (not fully read this session — `handleEntryChange`/`refresh` referenced in RESEARCH.md/CONTEXT.md `code_context`) | exact |
| `web/app/lib/api.ts` (MODIFIED: `WatchlistEntry.tags`/`note`, tag endpoint wrappers, note PUT wrapper) | utility (API client) | request-response | itself, `apiFetch` core + `updateWatchlistPreferences`-style wrapper (referenced from `PreferenceToggles.tsx`) | exact |
| `web/app/components/ui/dialog.tsx`, `alert-dialog.tsx`, `combobox.tsx`, `input-group.tsx`, `textarea.tsx` (vendored via shadcn CLI, not hand-written) | component (vendored) | n/a | `web/app/components/ui/select.tsx` (most recent vendored base-ui wrapper, same "override built-in text size, swap `IconPlaceholder`" convention per Phase 20) | role-match |

## Pattern Assignments

### `internal/tags/service.go` (service, CRUD + transactional)

**Analog:** `internal/watchlist/service.go`

**Package doc + imports pattern** (lines 1-20):
```go
// Package watchlist implements the watchlist domain: ... It wraps the
// sqlc-generated Queries behind a narrow Store interface -- ... -- so
// handler tests can substitute a stub instead of a live Postgres
// connection.
package watchlist

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)
```
`internal/tags` mirrors this exactly: a narrow `Store` interface, sqlc `Querier` behind it, sentinel errors as package vars.

**Constraint-name error mapping pattern** (lines 271-276) — this is the exact shape ADR 0004 / D-18 / D-22 require for the cap trigger's `check_violation` and the `lower(name)` collision:
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "watchlist_artist_id_key" {
	return Entry{}, ErrDuplicate
}
return Entry{}, fmt.Errorf("create watchlist entry: %w", err)
```
Extend with a second branch for `pgErr.Code == pgerrcode.CheckViolation && pgErr.ConstraintName == "artist_tags_max_per_artist"` → `ErrTagCapReached`, and a `pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "<tags-lower-name-index>"` (name TBD at migration-write time, D-22/A2) → `ErrTagNameCollision{Target}`.

**`pgx.ErrNoRows` → sentinel translation pattern** (lines 366-372, `UpdatePreferences`):
```go
updated, err := s.q.UpdateWatchlistPreferences(ctx, params)
if err != nil {
	if errors.Is(err, pgx.ErrNoRows) {
		return Entry{}, ErrNotFound
	}
	return Entry{}, fmt.Errorf("update watchlist preferences: %w", err)
}
```
Reuse for `Rename` (target renamed/deleted mid-flight → 404, D-22).

**`:execrows` idempotent-delete pattern** (`Remove`, lines 398-407):
```go
func (s *Service) Remove(ctx context.Context, id int64) error {
	affected, err := s.q.DeleteWatchlistEntry(ctx, id)
	if err != nil {
		return fmt.Errorf("delete watchlist entry: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
```
Reuse verbatim-shape for `Detach` (D-21: idempotent, 204 even when the link doesn't exist — note Detach's *success* path differs from Remove's: a 0-affected detach is still success, not `ErrNotFound`, per D-21).

**Transaction pattern for `Merge`:** no existing multi-statement `pgx.Tx` precedent in this codebase (confirmed by RESEARCH.md's Code Examples section — flagged there as "new code following the repo's general pgx.Tx usage conventions, not a verified copy"). Use the `Service.Merge` shape given in RESEARCH.md's Code Examples section verbatim (`tx, err := s.pool.Begin(ctx)`, `defer tx.Rollback(ctx)`, `q := s.q.WithTx(tx)`, three ordered statements per D-19, then `tx.Commit(ctx)`).

**Error sentinel declaration pattern** (lines 41-58): declare `ErrTagNotFound`, `ErrTagCapReached`, `ErrTagNameCollision`, `ErrNoteTooLong` etc. the same way — a `var (...)` block of `errors.New(...)`, one-line doc comment per sentinel naming which requirement/decision it backs.

---

### `internal/watchlist/service.go` (MODIFIED — extend in place)

**Entry struct extension:** add `Tags []TagRef` and `Note *string` fields to the existing `Entry` struct (lines 61-73), following the existing field-ordering/json-tag convention (`json:"tags"`, `json:"note"`).

**Non-nil slice guarantee pattern** (`List`, lines 292-309):
```go
entries := make([]Entry, 0, len(rows))
for _, row := range rows {
	entries = append(entries, Entry{ ... })
}
return entries, nil
```
Apply the identical `make([]TagRef, 0, len(tagIds))` guarantee per-row for `Entry.Tags` (D-26: "tags is never null") — zip `row.TagIds`/`row.TagNames` (Pattern 5 in RESEARCH.md) inside this same loop.

**Tag-survival test precedent:** `TestService_Remove_LeavesArtistRowIntact` (referenced in RESEARCH.md/CONTEXT.md, not read this session — locate and mirror its structure for the new `TestService_Remove_LeavesArtistTagsIntact`).

---

### `internal/httpserver/tags.go` (new controller, request-response)

**Analog:** `internal/httpserver/watchlist.go`

**Shared helpers to reuse directly, no reimplementation:** `writeError`, `decodeJSONBody`, `trimAndCap`, `errorResponse` (all package-level in `watchlist.go`, lines 52-121) — `tags.go` imports/calls these as-is, it does not redeclare them.

**ID-param parsing pattern** (`parseWatchlistID`, lines 101-108):
```go
func parseWatchlistID(r *http.Request) (int64, error) {
	raw := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid watchlist id: %q", raw)
	}
	return id, nil
}
```
Write a `parseTagID` (and reuse `parseWatchlistID` unchanged for the `{id}` in `/watchlist/{id}/tags`) with the identical BIGSERIAL-floor validation.

**Full request/response handler shape** (`handleUpdateWatchlist`, lines 299-341 — the closest existing "id param + decode + service call + switch-on-sentinel-errors" handler):
```go
func (s *Server) handleUpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	id, err := parseWatchlistID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid watchlist id")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)
	var req updateWatchlistRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	entry, err := s.watchlist.UpdatePreferences(r.Context(), id, watchlist.PreferencesParams{...})
	switch {
	case errors.Is(err, watchlist.ErrNoPreferencesSupplied):
		writeError(w, http.StatusBadRequest, "no preferences supplied")
		return
	case errors.Is(err, watchlist.ErrNotFound):
		writeError(w, http.StatusNotFound, "watchlist entry not found")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(entry)
}
```
Every new tags handler (`handleAttachTag`, `handleDetachTag`, `handleListTags`, `handleRenameTag`, `handleMergeTag`, `handleDeleteTag`) follows this exact skeleton, switching on `internal/tags`'s new sentinels. `handleRenameTag`'s 409 branch is new shape (D-22): write `{target: {...}, carrier_count_after_merge}` via a dedicated response struct, same `json.NewEncoder(w).Encode(...)` tail.

**204-no-body pattern** (`handleRemoveWatchlist`, lines 365-367):
```go
// 204 carries no payload -- no Content-Type, no encoder call.
w.WriteHeader(http.StatusNoContent)
```
Reuse verbatim for `handleDetachTag`'s idempotent-204 path (D-21).

---

### `internal/httpserver/watchlist.go` (MODIFIED — add `handleUpdateNote`)

**Analog:** `internal/httpserver/settings.go`'s `handleUpdateSettings` (lines 78-115) — closest existing "PUT, full-value-replace, one field, trim + cap before store call" shape:
```go
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)
	var req updateSettingsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cadence, err := settings.ParseCadence(req.DigestCadence)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid digest cadence")
		return
	}
	updated, err := s.settingsStore.Update(r.Context(), settings.UpdateParams{...})
	switch { ... }
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toSettingsResponse(updated))
}
```
`handleUpdateNote` follows this shape: `parseWatchlistID`, decode `{note: string | null}`, trim via `trimAndCap`-style helper (D-25: empty trims to `null`), call `s.watchlist.UpdateNote(ctx, id, note)`, switch on `ErrNotFound`/note-too-long/generic, 200 with the full updated `Entry` (D-26).

---

### `internal/httpserver/server.go` (MODIFIED — route registration)

**Analog:** itself, `registerDataRoutes` (lines 261-278):
```go
func registerDataRoutes(r chi.Router, s *Server) {
	r.Get("/search", s.handleSearch)
	r.Post("/watchlist", s.handleAddWatchlist)
	r.Get("/watchlist", s.handleListWatchlist)
	r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
	r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
	...
	r.Get("/settings/notifications", s.handleGetSettings)
	r.Put("/settings/notifications", s.handleUpdateSettings)
}
```
Append the seven new routes here, exactly as RESEARCH.md's "Code Examples" section specifies (`r.Get("/tags", ...)`, `r.Patch("/tags/{id}", ...)`, `r.Delete("/tags/{id}", ...)`, `r.Post("/tags/{id}/merge", ...)`, `r.Post("/watchlist/{id}/tags", ...)`, `r.Delete("/watchlist/{id}/tags/{tag_id}", ...)`, `r.Put("/watchlist/{id}/note", ...)`). Never register outside this function — this is the entire access-control surface (PITFALLS.md #4).

---

### `queries/tags.sql` (new, CRUD)

**Analog:** `queries/watchlist.sql`'s `:one`/`:many`/`:execrows` idioms (whole file). Key patterns to copy:

**`:execrows` for idempotent existence-sensitive writes** (`DeleteWatchlistEntry`, lines 64-72) — reuse for `DetachTag`:
```sql
-- name: DeleteWatchlistEntry :execrows
DELETE FROM watchlist WHERE id = $1;
```

**Upsert-with-RETURNING pattern:** no `ON CONFLICT` precedent in `watchlist.sql` itself, but `internal/watchlist/service.go` references `UpsertArtist` (called at line 244) — locate `queries/artists.sql`'s `UpsertArtist` for the `ON CONFLICT ... DO UPDATE ... RETURNING` shape D-29's get-or-create statement must match syntactically (not read this session; grep `queries/artists.sql` at execution time if the exact upsert idiom is needed as a second analog).

**New queries needed** (from RESEARCH.md's Architecture Patterns, Pattern 4/5 and the Merge transaction Code Example): `GetOrCreateTag` (D-29's one-statement upsert), `AttachTag` (plain insert, trigger does the cap work), `DetachTag` (`:execrows`), `ListTags` (carrier-count `:many`, D-11's watched-only join), `RenameTag` (`:one`, collision surfaces as `PgError`), `DeleteDuplicateSourceLinks`/`RepointSourceLinks`/`DeleteTag`/`CountCarriers` (the four merge-transaction statements, D-19).

---

### `queries/watchlist.sql` (MODIFIED — `ListWatchlist` enrichment)

**Analog:** itself, the existing `ListWatchlist` query (lines 6-19) plus its `release_types text[]` precedent, extended per RESEARCH.md Pattern 5's exact SQL (already vetted against the sqlc/pgx `json_agg` bug — copy that block verbatim, not `json_agg`):
```sql
SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       w.release_types, w.muted_event_types, w.note, w.created_at, w.updated_at,
       COALESCE(
         (SELECT array_agg(t.id ORDER BY t.name)
          FROM artist_tags at JOIN tags t ON t.id = at.tag_id
          WHERE at.artist_id = a.id),
         '{}'
       ) AS tag_ids,
       COALESCE(
         (SELECT array_agg(t.name ORDER BY t.name)
          FROM artist_tags at JOIN tags t ON t.id = at.tag_id
          WHERE at.artist_id = a.id),
         '{}'
       ) AS tag_names
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
ORDER BY a.name ASC, a.id ASC;
```
`CreateWatchlistEntry` (lines 1-4) gains an optional `note` param (D-27, Undo re-adds the note) — extend its `INSERT`/`VALUES`/`RETURNING *` the same three-column way it already is.

---

### `internal/db/migrations/000010_tags_and_notes.up.sql` / `.down.sql` (migration)

**Analog:** `000008_notification_settings.up.sql` for plain `CREATE TABLE` + inline `CHECK` shape:
```sql
CREATE TABLE notification_settings (
    id                  int PRIMARY KEY CHECK (id = 1),
    digest_enabled      boolean NOT NULL DEFAULT false,
    digest_cadence      text NOT NULL DEFAULT 'daily'
        CHECK (digest_cadence IN ('daily', 'weekly')),
    ...
);
```
Apply the same inline-`CHECK`-in-`CREATE TABLE`/`ADD COLUMN` convention for `tags.name` (`CHECK (name = btrim(name))`, D-28) and `watchlist.note` (`ALTER TABLE watchlist ADD COLUMN note text CHECK (note IS NULL OR btrim(note) <> '')`, D-28) — both confirmed by RESEARCH.md Pitfall 2 as the shape that produces **no** `cmd/migration-check` finding, versus a later separate `ADD CONSTRAINT`.

**Trigger/function body:** copy ADR 0004's exact SQL (`docs/adr/0004-per-artist-tag-cap-trigger.md`, reproduced in RESEARCH.md Pattern 1) verbatim — it is already the locked design, not a pattern to re-derive.

**No existing `CREATE TRIGGER`/`CREATE FUNCTION` precedent in this codebase's migrations** — this is genuinely new SQL shape for the project; RESEARCH.md confirms `internal/sqlscan.Parse` and `sqlc` both already handle it without special-casing (no code change needed elsewhere).

**Naming convention to follow (Open Question #1 / A2):** name the `tags (lower(name))` unique index explicitly and predictably (RESEARCH.md suggests `tags_name_lower_idx`), and write the service-layer `ConstraintName` string literal in the same commit.

---

### `web/app/components/watchlist/TagChips.tsx` (new component)

**Analog:** `web/app/components/watchlist/PreferenceToggles.tsx` for the optimistic-update-then-rollback shape (imports, `onEntryChange` partial-patch signature, try/catch/finally with a toast on failure):
```tsx
import { useState } from "react"
import { toast } from "sonner"
import { type WatchlistEntry, updateWatchlistPreferences } from "~/lib/api"

async function toggleReleaseType(type: string, next: boolean) {
  const previous = entry.release_types
  const optimistic = next ? [...previous, type] : previous.filter((t) => t !== type)
  onEntryChange(entry.id, { release_types: optimistic })
  setReleasePending(true)
  try {
    const updated = await updateWatchlistPreferences(entry.id, { releaseTypes: optimistic })
    onEntryChange(entry.id, { release_types: updated.release_types })
  } catch {
    onEntryChange(entry.id, { release_types: previous })
    toast.error("Couldn't update preferences — try again.")
  } finally {
    setReleasePending(false)
  }
}
```
**Diverges per D-24:** do NOT copy the whole-array-snapshot rollback shown above as-is. D-24 supersedes D-03 for chips specifically — use functional route-level updaters (`addTag(entryId, tag)` / `removeTag(entryId, tagId)`) and a per-row pending set keyed by a temporary id, not a `previous` snapshot restored wholesale. The *skeleton* (optimistic apply → async call → success reconciles → catch reverts → toast.error, disabled-state-while-pending) is what to copy; the *rollback granularity* is what to change, per D-24's explicit text.

**Chip base:** `web/app/components/ui/badge.tsx` (`Badge variant="secondary"`) — read directly at execution time for the exact `cva` variant shape; 24-UI-SPEC.md's Component Inventory table already locks the override classes (`h-6 max-w-full gap-1 pr-0 pl-2 text-label font-normal`).

---

### `web/app/components/watchlist/ArtistNote.tsx` (new component)

**Analog (structural only):** `PreferenceToggles.tsx`'s pending-state/try-catch shape. **Diverges on failure handling** — UI-SPEC [R2] explicitly rejects the toast-and-rollback pattern for notes ("this deliberately does not reuse the chip pattern of optimistic rollback plus toast"): on save failure, keep the textarea open with the user's text intact and show an inline `aria-live="polite"` error line instead. Follow UI-SPEC's Interaction Contract § Note editor verbatim for state transitions (open/save/cancel/error), not `PreferenceToggles`' revert-and-toast shape.

---

### `web/app/components/watchlist/ManageTagsDialog.tsx`, `web/app/components/common/ConfirmDialog.tsx` (new)

**No analog exists in this codebase** — this is the first `Dialog`/`AlertDialog` usage. Build directly from 24-UI-SPEC.md's Layout & Visual Hierarchy and Interaction Contract sections (already fully locked: markup skeletons, focus-management table, copy). Use `web/app/components/ui/select.tsx` only as the *wrapping convention* precedent (how a base-ui primitive is re-exported as a project `ui/` component with `data-slot`, `cn(...)` class merging, and prop pass-through) — not for interaction behavior, which the UI-SPEC fully specifies.

---

### `web/app/lib/api.ts` (MODIFIED)

**Analog:** itself — the existing wire-type + wrapper conventions:
```ts
export interface WatchlistEntry {
  id: number
  artist_id: number
  ...
  created_at: string
  updated_at: string
}
```
Add `tags: { id: number; name: string }[]` and `note: string | null` fields, matching the Data Contract block in 24-UI-SPEC.md exactly.

**`apiFetch` core** (lines 184-240) is the one path every new wrapper funnels through — no new fetch logic, no bypassing it (PITFALLS.md #4/CSRF). New wrappers (`attachTag`, `detachTag`, `listTags`, `renameTag`, `mergeTag`, `deleteTag`, `updateNote`) follow the same `apiFetch<T>(path, { method, body: JSON.stringify(...) })` shape as the (not-fully-read-this-session, but referenced) `updateWatchlistPreferences` wrapper used by `PreferenceToggles.tsx`.

**Error handling:** `ApiError` (lines 168-176) already carries `.status` — the 409 collision response's `{target, carrier_count_after_merge}` body needs a typed catch: `catch (e) { if (e instanceof ApiError && e.status === 409) { const body = ... } }`. Confirm at execution time whether `apiFetch` needs a variant that returns the parsed body on a non-2xx instead of just the message string (`ApiError` today only carries `message`, not the full JSON body) — this is a likely small addition to `apiFetch` or a dedicated `renameTag` implementation that does its own `fetch` for the 409 case, not a re-use of `apiFetch` unchanged. Flag for the planner as an integration point to confirm, not an assumption to lock silently.

## Shared Patterns

### Constraint-name error mapping (backend)
**Source:** `internal/watchlist/service.go` lines 271-276
**Apply to:** every `internal/tags` service method that can hit a Postgres constraint (`Attach` → cap trigger `check_violation`; `Rename`/`GetOrCreateTag` → `lower(name)` unique violation)
```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation && pgErr.ConstraintName == "<name>" {
	return ..., ErrSentinel
}
```

### Three-layer validation (fail-fast handler → non-bypassable service → DB CHECK/trigger)
**Source:** `internal/httpserver/watchlist.go` (`trimAndCap` + length checks before `s.watchlist.Add`) + `internal/watchlist/service.go` (`normalizeSet`) + migration `CHECK`s
**Apply to:** tag name (32 chars), tag count (10), note (500 chars) — every cap this phase introduces needs all three layers, per ROADMAP's established pattern and D-14/D-15/D-18's explicit "API and DB still refuse" language.

### Handler skeleton: parse id → bound body → decode → validate → service call → switch-on-sentinel → encode
**Source:** `internal/httpserver/watchlist.go` `handleUpdateWatchlist` (lines 299-341)
**Apply to:** every new `internal/httpserver/tags.go` handler.

### `:execrows` idempotent delete
**Source:** `queries/watchlist.sql` `DeleteWatchlistEntry` (lines 64-72), consumed by `Service.Remove` (lines 398-407)
**Apply to:** `DetachTag` — but note the *response* semantics diverge from `Remove`: D-21 makes 0-affected a success (204), not `ErrNotFound`.

### Optimistic UI update with rollback
**Source:** `web/app/components/watchlist/PreferenceToggles.tsx` (whole file)
**Apply to:** `TagChips.tsx` — skeleton only; D-24 changes the rollback granularity from whole-array snapshot to functional per-item updaters. `ArtistNote.tsx` explicitly does NOT use this pattern on failure (UI-SPEC [R2]).

### Route registration inside the protected group
**Source:** `internal/httpserver/server.go` `registerDataRoutes` (lines 261-278)
**Apply to:** all seven new routes — this is the entire authz surface (PITFALLS.md #4); never register elsewhere.

### `apiFetch` as the single client-side call path
**Source:** `web/app/lib/api.ts` lines 184-240
**Apply to:** every new tag/note wrapper — carries CSRF header and 401 interceptor automatically.

### Non-nil slice guarantee
**Source:** `internal/watchlist/service.go` `List` (`make([]Entry, 0, len(rows))`, lines 292-309)
**Apply to:** `Entry.Tags` zip (D-26: "tags is never null") and `internal/tags.Service.List`'s returned slice.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `web/app/components/watchlist/ManageTagsDialog.tsx` | component | CRUD (list/rename/merge/delete inside a dialog) | First `Dialog`-based CRUD surface in this codebase — build directly from 24-UI-SPEC.md, which fully specifies markup, states, and copy |
| `web/app/components/common/ConfirmDialog.tsx` | component | request-response | First `AlertDialog` usage — build directly from 24-UI-SPEC.md's Interaction Contract § "Manage tags: rename, merge, delete" and Focus management table |
| `internal/tags/service.go` `Merge` transaction | service (transactional) | batch (multi-statement tx) | No existing multi-statement `pgx.Tx` precedent in this codebase; RESEARCH.md's Code Examples section already provides the exact shape to use (flagged there as new, not a verified copy) |
| Migration trigger/function DDL (`CREATE FUNCTION ... CREATE TRIGGER`) | migration | batch (DDL) | No prior migration in this repo defines a Postgres trigger; ADR 0004 is the design source, not a codebase analog |

## Metadata

**Analog search scope:** `internal/watchlist/`, `internal/httpserver/` (`watchlist.go`, `settings.go`, `server.go`), `queries/`, `internal/db/migrations/`, `web/app/components/watchlist/`, `web/app/components/common/`, `web/app/components/ui/`, `web/app/lib/api.ts`
**Files scanned:** 12 read directly this session (full or targeted), plus `internal/httpserver/server.go`'s route table via grep
**Pattern extraction date:** 2026-09-22
