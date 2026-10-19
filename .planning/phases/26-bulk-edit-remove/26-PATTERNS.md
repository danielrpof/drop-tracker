# Phase 26: Bulk Edit & Remove - Pattern Map

**Mapped:** 2026-10-15
**Files analyzed:** 22 new/modified
**Analogs found:** 21 / 22 (all analog paths are git-tracked source)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|---|---|---|---|---|
| `queries/bulk.sql` (new) | query | batch / CRUD | `queries/tags.sql` + `queries/notification_settings.sql` (`AckEventsOnly`) | role-match; SQL bodies are pinned in 26-RESEARCH.md Examples 1-5 |
| `internal/db/sqlc/bulk.sql.go` (generated) | generated | - | `internal/db/sqlc/tags.sql.go` | exact (run `sqlc generate`, then `git add internal/db/sqlc/`) |
| `internal/tags/bulk.go` (new) | service | CRUD, transactional | `internal/tags/service.go` `Attach` / `Detach` | exact |
| `internal/tags/bulk_test.go` (new) | test | CRUD | `internal/tags/service_test.go` | exact |
| `internal/watchlist/bulk.go` (new) | service | batch | `internal/watchlist/service.go` `UpdatePreferences` / `Remove` / `normalizeSet` | exact |
| `internal/watchlist/bulk_test.go` (new) | test | batch | `internal/watchlist/service_test.go` | role-match |
| `internal/httpserver/bulk.go` (new) | controller | request-response | `internal/httpserver/tags.go` + `watchlist.go` | exact |
| `internal/httpserver/bulk_test.go` (new) | test | request-response | `internal/httpserver/tags_test.go` | exact |
| `internal/httpserver/tags.go` (mod: `TagStore`) | interface | request-response | itself, L56-63 | exact |
| `internal/httpserver/tags_test.go` (mod: `fakeTagStore`) | test double | - | itself, L594-620 | exact |
| `internal/httpserver/server.go` (mod) | config / route | - | `WithTags`, `WithSettings`, `registerDataRoutes` | exact |
| `cmd/server/main.go` (mod) | wiring | - | L310-313 | exact |
| `docs/adr/0004-per-artist-tag-cap-trigger.md` (mod) | doc | - | itself (amendment) | exact |
| `web/app/lib/bulkSelection.ts` (+test) (new) | utility | transform | `web/app/lib/watchlistView.ts` (+ `watchlistView.test.ts`) | exact |
| `web/app/lib/format.ts` (mod: `formatNameList`) | utility | transform | `web/app/lib/format.ts` (+ `format.test.ts`) | role-match |
| `web/app/lib/api.ts` (mod: 4 wrappers + types) | service | request-response | `attachTag`, `detachTag`, `updateWatchlistPreferences` | exact |
| `web/app/components/watchlist/BulkActionBar.tsx` (+test) (new) | component | event-driven | `TagCombobox.tsx`, `PreferenceToggles.tsx`, `ManageTagsDialog.tsx` | role-match |
| `web/app/components/watchlist/TagCombobox.tsx` (mod: generalize `entry` prop) | component | event-driven | itself | exact |
| `web/app/components/ui/popover.tsx` (vendored) | component | - | other vendored `ui/*` files; `shadcn add popover` | n/a |
| `web/app/components/ui/checkbox.tsx` (mod: indeterminate) | component | - | itself | exact |
| `web/app/components/watchlist/WatchlistRow.tsx` / `WatchlistToolbar.tsx` (mod) | component | event-driven | themselves | exact |
| `web/app/routes/watchlist.tsx` (mod) + `watchlist.bulk.test.tsx` (new) | route | event-driven | itself (`handleRemove`, `markSticky`, `refresh`); `ArtistNote.test.tsx` for mocks | exact |
| `web/app/root.tsx`, `web/app/app.css` (mod) | config | - | existing `<Toaster>` and `html` rule | exact |

## Pattern Assignments

### `internal/tags/bulk.go` (service, transactional CRUD)

**Analog:** `internal/tags/service.go`

**Tx scaffold** (`Attach`, L93-130). Copy it for `BulkAttach`: `NormalizeName`, Begin, deferred Rollback, `s.q.WithTx(tx)`, `GetOrCreateTag`, `mapTagError`, Commit. Per Research Pitfall B5, return without Commit when no selected artist ends up carrying the tag.
```go
tx, err := s.db.Begin(ctx)
if err != nil { return AttachResult{}, fmt.Errorf("begin attach tx: %w", err) }
defer func() { _ = tx.Rollback(ctx) }()
qtx := s.q.WithTx(tx)
...
tag, err := qtx.GetOrCreateTag(ctx, normalized)
if err != nil { return AttachResult{}, mapTagError(err) }
...
if err := tx.Commit(ctx); err != nil { return ..., fmt.Errorf("commit attach tx: %w", err) }
```

**Not-found mapping.** `BulkDetach` first calls `GetTagByID` (from `queries/bulk.sql`), and `pgx.ErrNoRows` becomes `ErrTagNotFound`, as in `Detach` (L134-147):
```go
if errors.Is(err, pgx.ErrNoRows) { return ErrEntryNotFound }
return fmt.Errorf("resolve watchlist artist: %w", err)
```

**Cap backstop.** `mapTagError` (L284-299) already turns the trigger's `check_violation` on `artist_tags_max_per_artist` into `ErrTagCapReached`. Reuse it. Do not add a second constraint literal.

**Imports** (L3-12): `context`, `errors`, `fmt`, `internal/db/sqlc`, `pgerrcode`, `pgx/v5`, `pgconn`. The `Service` struct has `db DB` and `q *sqlc.Queries`. Add new methods to it. Put result types (`BulkSkip`, `BulkAttachResult`, `BulkDetachResult`) beside `AttachResult` (L46-51). The result types carry `Tag` as `tags.Tag`, and `tags` must not import `watchlist`.

---

### `internal/watchlist/bulk.go` (service, batch)

**Analog:** `internal/watchlist/service.go`

**Validation to reuse** (`normalizeSet`, L537-559). Use it for the `set` and `unset` lists. It never returns nil, which also guards Research Pitfall B1:
```go
func normalizeSet(values []string, allowed []string, invalidErr error) ([]string, error)
// ReleaseTypes = []string{"album","single","ep","deluxe"} (L48); EventTypes likewise
```

**Sentinel and empty-request pattern** (`UpdatePreferences`, L441-443). Mirror it for D-21. Add `ErrBulkOverlap`, and reuse `ErrNoPreferencesSupplied` when all four lists are empty:
```go
if p.ReleaseTypes == nil && p.MutedEventTypes == nil { return Entry{}, ErrNoPreferencesSupplied }
```

**Param construction.** The existing code seeds non-nil empty slices (L447-451, `ReleaseTypes: []string{}`). Do the same for `BulkUpdateWatchlistPreferencesParams` and pass `ReleaseTypes` / `EventTypes` as `AllReleaseTypes` / `AllEventTypes`, which fixes the canonical order. Compute `gone` as the deduped requested ids minus the returned ids.

**Delete.** `Remove` (L492-501) is the single-row precedent. The bulk version calls `BulkDeleteWatchlist` with no transaction (D-03). `removed` is the returned ids and `already_gone` is the requested ids minus the returned ids. Both are non-nil.

Methods go on `*watchlist.Service` but NOT on `watchlist.Store` (D-20).

---

### `internal/httpserver/bulk.go` (controller, request-response)

**Analog:** `internal/httpserver/tags.go` (L18-130) plus `watchlist.go` (L73-128, `decodeJSONBody`).

**Option and 503 pattern** (`tags.go` L53-72, L98-102):
```go
type TagStore interface { Attach(...); Detach(...); ... }
func WithTags(store TagStore) Option { return func(c *serverConfig) { c.tags = store } }
...
if s.tags == nil { writeError(w, http.StatusServiceUnavailable, "tags not available"); return }
```
New `WatchlistBulkStore` and `WithWatchlistBulk` follow this exactly (bulk remove and bulk preferences routes). Bulk tag routes use `s.tags` and `TagStore`, which gains `BulkAttach` and `BulkDetach`.

**Decode and bound** (L110-115, plus `decodeJSONBody` in `watchlist.go` L118-128):
```go
r.Body = http.MaxBytesReader(w, r.Body, maxTagBodyBytes)
var req attachTagRequest
if err := decodeJSONBody(r, &req); err != nil {
    writeError(w, http.StatusBadRequest, "invalid request body"); return
}
```
Use `maxBulkBodyBytes = 65536`. Add ids validation (non-empty, each >= 1, at most `maxBulkIDs = 1000`, deduped in first-occurrence order). Over the limit, return 400 with `code: bulk_too_many_ids` and `max`, following the `tagCollisionResponse` shape (L186-191).

**Error table.** Extend the `tagErrors` pattern (L34-42, `domainError{err, status, code}`) with `writeDomainError` (`watchlist.go` L82-90). `ErrTagNotFound` already maps to 404 `tag_not_found` (D-22). Unmapped errors do:
```go
httplog.SetAttrs(r.Context(), slog.String("tags_error", err.Error()))
writeError(w, http.StatusInternalServerError, "internal error")
```

**Response.** Every slice is non-nil. Use `w.Header().Set("Content-Type","application/json")`, `WriteHeader(200)`, `json.NewEncoder(w).Encode(...)`. There is no `note` field in the preferences DTO, so over-posting fails under `DisallowUnknownFields` (24 D-25).

---

### `internal/httpserver/server.go` (route and option wiring)

**Analog:** itself.
- Add a `watchlistBulk WatchlistBulkStore` field to `Server` (L31-46) and `serverConfig` (L51-63), next to `tags TagStore`.
- Register routes in `registerDataRoutes` (L264-293) as literal paths, next to the tag routes (L286-287). Static segments beat `{id}` in chi (Research Pitfall B3):
```go
r.Post("/watchlist/bulk/tags/add", s.handleBulkAddTag)
r.Post("/watchlist/bulk/tags/remove", s.handleBulkRemoveTag)
r.Patch("/watchlist/bulk/preferences", s.handleBulkPreferences)
r.Post("/watchlist/bulk/delete", s.handleBulkDelete)
```
- `cmd/server/main.go` L310-313: add `httpserver.WithWatchlistBulk(store)` beside `httpserver.WithTags(tags.NewService(pool))`. `store` is the `watchlist.NewService(...)` from L233.

---

### `internal/httpserver/bulk_test.go` (+ `fakeTagStore` in `tags_test.go`)

**Analog:** `internal/httpserver/tags_test.go`

**End-to-end** (L50-60): `testutil.NewIsolatedTestPool(t, "..._http_test")`, `watchlist.NewService(sqlc.New(pool))`, `tags.NewService(pool)`, `httpserver.New(pool, watchlistSvc, stubEventsStore{}, nil, discardLogger(), httpserver.WithTags(tagsSvc))`, `httptest.NewServer(srv.Router())`.

**Gated 401 and 403 pin** (L936-1005). Replicate for all four bulk routes. The 403 test asserts that the store call counters stay 0:
```go
ts := newTagsServer(t, tagsServerOpts{passphrase: settingsTestPassphrase})
// no cookie -> 401 (Authenticate runs before RequireCSRFHeader)
cookie := loginForSettings(t, ts)
// cookie but no X-Requested-With -> 403, atomic.LoadInt32(&calls) == 0
```

**Test double** (L594-620). `fakeTagStore` uses per-method `xxxFunc` fields plus `*int32` call counters. Add `bulkAttachFunc` / `bulkAttachCalls` and the same for detach, in the same task that extends `TagStore` (Pitfall B7, `var _ httpserver.TagStore = fakeTagStore{}` at about L665). Add one test per route proving it hits the bulk handler (store call count 1) and not `/watchlist/{id}/tags`.

---

### `internal/tags/bulk_test.go`

**Analog:** `internal/tags/service_test.go`. Reuse `seedEntry` (L19-31, returns `entryID, artistID`) and `countTagsByLowerName` (L33-40). Use `testutil.NewIsolatedTestPool(t, "tags_service_test")`. Required cases (D-19, Pitfalls B1, B4, B5):
- Concurrent overlapping bulk attaches with different tags and reversed id lists do not deadlock.
- Every selected artist at the cap leaves `countTagsByLowerName(...) == 0`.
- A cap skip is reported by name.
- `gone` ids are skipped.
- The preferences delta with only `set` lists leaves untouched types intact (nil-slice guard).

---

### `queries/bulk.sql` (sqlc)

**Analogs:** `queries/tags.sql` L5-20 (`GetOrCreateTag`, `AttachTag :execrows`, `DetachTag`) and `queries/notification_settings.sql` L26-32 (`= ANY(sqlc.arg('ids')::bigint[])` precedent):
```sql
UPDATE events SET notified_at = now()
WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND notified_at IS NULL;
```
Use the five already-verified statements from 26-RESEARCH.md Examples 1-5. These rewrites matter: the `IN (SELECT ...)` form instead of `DELETE ... USING ... RETURNING cte.col`, `t.artist_id::bigint` for the unnest column, and `COALESCE(@x::text[], '{}')` around array params. Keep comments to 1-3 lines. `sqlc.yaml` uses `pgx/v5`, `emit_json_tags`, `emit_interface`.

---

### `web/app/lib/api.ts` (wrappers)

**Analog:** `attachTag` (L377-386) and `detachTag` (L369-373):
```ts
export async function attachTag(entryId: number, name: string): Promise<TagRef> {
  return apiFetch<TagRef>(`/watchlist/${entryId}/tags`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  })
}
```
Add `BulkOutcome` (per the UI-SPEC Data Contract) and four wrappers that use the same headers and body shape. `apiFetch` injects `X-Requested-With`, and `ApiError` carries `status`, `body` and `code` (L192-203). Use the PATCH verb for preferences. snake_case wire fields go in the body, as in `updateDigestSettings` (L510-523).

---

### `web/app/lib/bulkSelection.ts` (utility, transform)

**Analog:** `web/app/lib/watchlistView.ts`. It is pure, and its header forbids runtime imports from `~/lib/api` because route tests automock it. Use `import type` only. Build `effectiveSelection(visible, selectedIds)` on `selectVisible` (L223-238), `applyRange`, `buildPrefDelta`, and a tri-state helper. Colocate `bulkSelection.test.ts` as `watchlistView.test.ts` does.

---

### `web/app/components/watchlist/BulkActionBar.tsx`

**Analogs:**
- `TagCombobox.tsx` L17-35. Props are `entry`, `vocabulary`, `onArtist`, `pendingNames`, `onCommit`, `onClose(reason)`. Generalize `entry` to `inputLabel` plus `alreadyOnMessage` (aria-label currently `Add tag to ${entry.name}` at L134).
- `PreferenceToggles.tsx` for the option labels and aria-labels (UI-SPEC: same as the card).
- `ConfirmDialog.tsx` L11-22 for the props the route passes; it is hosted at route level, not in the bar.

Constraint: the bar must render inside `TagVocabularyProvider`. `Watchlist` renders the provider (watchlist.tsx L538), so it cannot call `useTagVocabulary()` itself (Research Anti-Patterns). Names render as plain JSX text only.

---

### `web/app/components/ui/checkbox.tsx` (mod)

**Analog:** itself. Root class has `data-checked:border-primary data-checked:bg-primary data-checked:text-primary-foreground`. The `Indicator` always renders `<CheckIcon />`. Add the `data-indeterminate:*` mirror, a `group/checkbox` class, and a `MinusIcon` swap (UI-SPEC "Vendored-file edit").

---

### `web/app/routes/watchlist.tsx` (mod) and `watchlist.bulk.test.tsx` (new)

**Analog:** itself.

**Sticky helpers** (L109-119). Add `markStickyMany(ids)` with the same functional-updater shape:
```ts
function markSticky(id: number) {
  setSticky((prev) => prev.ids.has(id) ? prev : { ...prev, ids: new Set(prev.ids).add(id) })
}
```

**Visible and effective selection** (L479-483):
```ts
const visible = entries ? selectVisible(entries, { ...view, q: query }, stickyIds) : []
```
Derive `effective = visible.filter(e => selectedIds.has(e.id))` from it. Never trim `selectedIds` (D-11).

**Failure and success handling** (`handleRemove`, L423-461): optimistic filter via `setEntries((rows) => rows ? rows.filter(...) : rows)`, a `try/catch` with `toast.error(...)` plus `refresh()`, and `toast.success(msg, { duration, action: { label: "Undo", onClick } })`. The remove-tag Undo reuses this action shape (U4). `refresh` (L238-244) is `useCallback`, keeps entries, and resolves to rows or null. Always call it after bulk success.

**Announcements:** reuse `announce()` (L356) and `scheduleCountAnnouncement` (L154-164) for the `{k} selected.` suffix. Focus fallback follows `headingRef` and `focusAfterCardLeaves` (L195-203). Make `<h1>` `tabIndex={-1}`.

**ConfirmDialog close order (Research F3):** `handleConfirm` calls `onOpenChange(false)` only in `finally`. Record success in a ref inside `onConfirm`, then clear the selection and set `pendingFocus` from `onOpenChange(false)`.

**Test mocks** (`ArtistNote.test.tsx` L12-17): use the partial-mock pattern so `ApiError` survives:
```ts
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  updateNote: vi.fn(),
}))
```
The sonner mock needs `{ error, success, warning }` (Pitfall F1). Put the bulk tests in a new file, because `watchlist.test.tsx` is already about 2600 lines.

---

### `web/app/components/watchlist/WatchlistRow.tsx` and `WatchlistToolbar.tsx` (mod)

**Analog:** themselves. The row's `<li className="flex flex-col gap-4 rounded-md bg-card p-4 sm:flex-row sm:items-center">` (L35) gets a leading `div` with the 44px label and `Checkbox` before `CoverArt`, plus the selected ring. The props interface (L10-16) gains `selected`, `onSelectChange`, `disabled`. The heading-row select-all lives in `watchlist.tsx` per the UI-SPEC. `WatchlistToolbar` is unchanged unless the planner moves select-all there (CONTEXT says toolbar, UI-SPEC says heading row; the UI-SPEC wins).

---

## Shared Patterns

### Auth, CSRF and gating
**Source:** `internal/httpserver/server.go` `registerDataRoutes` L264-293. **Apply to:** all four bulk routes. Register them inside this function and they inherit `gate.Authenticate` and `RequireCSRFHeader` with no new middleware. Pin the 401 and 403 tests in the style of `tags_test.go` L936-1005.

### Error to status table
**Source:** `internal/httpserver/tags.go` L34-51 with `writeDomainError` in `watchlist.go` L82-90. **Apply to:** bulk handlers. One `[]domainError` table per package, with a fallback of `httplog.SetAttrs` plus a 500 "internal error". The response body must never carry a raw error.

### Strict decode with a bounded body
**Source:** `watchlist.go` `decodeJSONBody` L118-128, with `http.MaxBytesReader` at `tags.go` L110. **Apply to:** all bulk handlers. Unknown fields and trailing values give 400.

### Postgres errors by constraint name
**Source:** `tags/service.go` `mapTagError` L284-299. Never pre-check; map `pgerrcode` plus `ConstraintName`.

### Optional-store 503
**Source:** `tags.go` L99-102. **Apply to:** `WithWatchlistBulk` routes.

### Frontend API and state conventions
`apiFetch` is the only fetch path. The sticky set is marked after edits. `refresh()` follows every bulk success. Functional state updaters are used throughout. Comments are 1-3 lines. Run `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` before staging.

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `web/app/components/ui/popover.tsx` | component | - | Vendored via `corepack pnpm --dir web exec shadcn add popover` (answer no to overwrites). Override `PopoverTitle` with `text-heading font-semibold` and `PopoverContent` with `w-80`. |
| Set-based `FOR NO KEY UPDATE ... ORDER BY` pre-lock and delta CTE | SQL | batch | No existing bulk row-locking query. Use the verified SQL in 26-RESEARCH.md Examples 1-4 verbatim. |

## Metadata

**Analog search scope:** `internal/tags`, `internal/watchlist`, `internal/httpserver`, `queries`, `cmd/server`, `web/app/{routes,lib,components}`
**Files scanned:** about 25
**Tracked-source check:** all named paths are repository source (`queries/`, `internal/`, `web/app/`). No `.gsd` mirror paths are named.
**Pattern extraction date:** 2026-10-15
