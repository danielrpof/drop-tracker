# Phase 25: Find & Filter - Pattern Map

**Mapped:** 2026-10-05
**Files analyzed:** 22 (new/modified)
**Analogs found:** 20 / 22 (2 have no code analog, use RESEARCH.md)

All analog paths below were verified git-tracked (`git ls-files`). G1-G7 in CONTEXT.md override D-xx where they conflict (no cutoff on ListWatchlist; two LATERALs; sticky cards; History URL state as its own plan).

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match |
|---|---|---|---|---|
| `queries/watchlist.sql` (ListWatchlist + GetWatchlistEntry) | query | CRUD/read | itself, the `tag_ids`/`tag_names` ARRAY enrichment (lines 26-60) | exact |
| `queries/events.sql` (ListEvents + HasOlderEvents) | query | request-response | itself, `artist_id`/`event_type` narg filters (148-149, 184-185) | exact |
| `internal/db/sqlc/*.go` | generated | n/a | regenerate, never hand-edit | n/a |
| `internal/watchlist/service.go` (Entry, entryFromRow) | service | CRUD | itself, lines 82-96, 345-365 | exact |
| `internal/events/service.go` (ListParams.TagID) | service | request-response | itself, lines 143-167 | exact |
| `internal/httpserver/events.go` (tag_id parse) | controller | request-response | itself, `artist_id` parse lines 81-85 | exact |
| `internal/httpserver/events_test.go` (tag + retention tests) | test | request-response | `TestListEvents_RetentionExcludesAgedOutRows` (745-800), `TestListEvents_HasOlderEventsRespectsFilters` (1287) | exact |
| `internal/watchlist/service_test.go` (latest/next release, parity) | test | CRUD | `TestService_ProjectionParity` (2358) | exact |
| `web/app/lib/api.ts` | utility | request-response | itself, `WatchlistEntry` (77-94), `listEvents` (278-289) | exact |
| `web/app/lib/format.ts` (`formatReleaseDate`) | utility | transform | itself (pure, React-free formatters) | role-match |
| `web/app/lib/watchlistView.ts` (NEW) | utility | transform | `web/app/lib/tags.ts` (`buildTagSuggestions`, pure helper kept out of api.ts) | role-match |
| `web/app/lib/useUrlParams.ts` (NEW) | hook | event-driven | none (see No Analog) | none |
| `web/app/lib/test/fixtures.ts` (NEW) | test util | n/a | `web/app/lib/test/routeStub.tsx` | role-match |
| `web/app/components/watchlist/WatchlistToolbar.tsx` (NEW) | component | event-driven | `web/app/components/system/DigestSettings.tsx` (Select) + `HistoryFilters.tsx` (label layout) | role-match |
| `web/app/components/watchlist/WatchlistTagFilter.tsx` (NEW) | component | event-driven | `web/app/components/watchlist/TagCombobox.tsx` + `ui/combobox.tsx` | role-match |
| `web/app/components/watchlist/TagChips.tsx` | component | event-driven | itself (194-217) | exact |
| `web/app/components/watchlist/WatchlistRow.tsx` | component | request-response | itself (36-73) | exact |
| `web/app/components/ui/combobox.tsx` (ComboboxChip edit) | component (vendored) | n/a | itself (235-265) | exact |
| `web/app/components/history/HistoryFilters.tsx` | component | event-driven | itself, `Combobox<T>` (58-184) | exact |
| `web/app/routes/watchlist.tsx` | route | CRUD | itself | exact |
| `web/app/routes/history.tsx` | route | request-response | itself | exact |
| `internal/webassets/build/client/` | build artifact | n/a | Phase 24 rebuild commit; `make web` | n/a |

## Pattern Assignments

### `queries/watchlist.sql` (query, read)

**Analog:** same file. Both ListWatchlist (lines 26-39) and GetWatchlistEntry (47-60) must stay byte-identical in select list and joins, because `Service.get` converts with `sqlc.ListWatchlistRow(row)` (service.go:371-379).

**Existing enrichment shape to extend** (lines 26-39):
```sql
SELECT w.id AS id, a.id AS artist_id, a.mbid, a.name, a.deezer_id,
       a.disambiguation, a.image_url,
       w.release_types, w.muted_event_types, w.note, w.created_at, w.updated_at,
       ARRAY( ... )::bigint[] AS tag_ids,
       ARRAY( ... )::text[] AS tag_names
FROM watchlist w
JOIN artists a ON a.id = w.artist_id
ORDER BY a.name ASC, a.id ASC;
```
**Add** (verified in RESEARCH Pattern 1): after `tag_names`, append `latest.release_date AS latest_release_date, upcoming.release_date AS next_release_date`, then two plain-column `LEFT JOIN LATERAL (... ORDER BY e.release_date DESC|ASC LIMIT 1) ... ON true` between the `JOIN artists` and `ORDER BY`. Use `left(to_char((now() AT TIME ZONE 'UTC')::date,'YYYY-MM-DD'), length(e.release_date))` for the same-precision upcoming test (`<=` for latest, `>` for upcoming). Do NOT use `max() FILTER` (sqlc emits `interface{}`/non-null `string`). No cutoff arg (G1). Positional `$1` in GetWatchlistEntry stays. Header comments: keep short (CLAUDE.md comment discipline).

---

### `queries/events.sql` (query, request-response)

**Analog:** same file. Append the tag predicate in BOTH queries, mirroring the existing narg pattern.

**ListEvents filters** (lines 148-149) / **HasOlderEvents** (184-185):
```sql
WHERE (sqlc.narg('artist_id')::bigint IS NULL OR artist_id = sqlc.narg('artist_id')::bigint)
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type')::text)
```
**Add after event_type, before cursor / created_at lines:**
```sql
  AND (sqlc.narg('tag_id')::bigint IS NULL OR EXISTS (
        SELECT 1 FROM artist_tags link
        WHERE link.artist_id = events.artist_id AND link.tag_id = sqlc.narg('tag_id')::bigint))
```
In HasOlderEvents the inner `FROM events` is un-aliased, so `events.artist_id` works the same. Retention stays `created_at >= cutoff` (ListEvents:162) / `created_at < cutoff` (HasOlderEvents:186), in the same query (PITFALLS #9). Then `sqlc generate`, `git add internal/db/sqlc`, `make sqlc-check`.

---

### `internal/watchlist/service.go` (service)

**Entry struct** (lines 82-96): add `LatestReleaseDate *string \`json:"latest_release_date"\`` and `NextReleaseDate *string \`json:"next_release_date"\``.
**entryFromRow** (345-365): add `LatestReleaseDate: row.LatestReleaseDate, NextReleaseDate: row.NextReleaseDate,` after `Note`. Assert generated type is `*string` (nullability trap). `watchlist.NewService` gets no retention arg (G1).

---

### `internal/events/service.go` + `internal/httpserver/events.go`

**Handler analog** (events.go:81-85, 118-123):
```go
artistID, ok := parseOptionalPositiveInt64(r, "artist_id")
if !ok {
	writeError(w, http.StatusBadRequest, "invalid artist_id")
	return
}
...
page, err := s.events.List(r.Context(), events.ListParams{ArtistID: artistID, EventType: eventType, Cursor: cursor, PageSize: pageSize})
```
Copy for `tag_id` -> `invalid tag_id`; add `TagID: tagID`. Update the `handleListEvents` doc comment ("four optional query params" becomes five).

**Service analog** (events.go service 143-167): add `TagID: p.TagID` to BOTH `sqlc.ListEventsParams{...}` and `sqlc.HasOlderEventsParams{...}`; add `TagID *int64` to `ListParams`.

---

### `internal/httpserver/events_test.go` (test)

**Analog:** `TestListEvents_RetentionExcludesAgedOutRows` (745-800). Copy the skeleton for a `TestRetention_TagFilterComposesWithCutoff`:
```go
pool := testutil.NewTestPool(t)
mbid := testMBID(t)
t.Cleanup(func() { pool.Exec(..., "DELETE FROM artists WHERE mbid = $1", mbid) })
artistID := insertTestArtist(t, pool, mbid)
now := time.Now()
agedOutID := insertTestEventAt(t, pool, artistID, mbid+"-old", now.AddDate(0, 0, -120))
recentID := insertTestEventAt(t, pool, artistID, mbid+"-new", now.AddDate(0, 0, -1))
svc := events.NewService(sqlc.New(pool), 90)
srv := httpserver.New(pool, stubStore{}, svc, nil, discardLogger())
ts := httptest.NewServer(srv.Router())
```
Seed tag + `artist_tags` link (no `watchlist` row = removed-artist shape). Assert `events == [recentID]` and `has_older_events == true`, a second tag on another artist is absent, and `tag_id=abc|0|-1` returns 400 (extend `TestHandleListEvents_Validation`, line 222). Also mirror `TestListEvents_HasOlderEventsRespectsFilters` (1287).

### `internal/watchlist/service_test.go`
**Analog:** `TestService_ProjectionParity` (2358). Extend with seeded events so `Add`/`UpdatePreferences`/`UpdateNote` equal the `List` element including both new fields; add an artist with zero events asserting JSON `null` and HTTP 200. Use fixed far dates (1999/2999) per RESEARCH Pitfall 13.

---

### `web/app/lib/api.ts` (types and wrappers only)

**WatchlistEntry** (77-94): add after `note`:
```ts
latest_release_date: string | null
next_release_date: string | null
```
**listEvents** (278-289): add `tagId?: number` and `if (params?.tagId != null) search.set("tag_id", String(params.tagId))`, mirroring the `artistId` line. No helpers here: every test file does a bare `vi.mock("~/lib/api")` (history.test.tsx:15), which automocks them.

### `web/app/lib/format.ts` and `web/app/lib/watchlistView.ts`
**Analog:** `format.ts` (pure, React-free, no `Date` for the new helper) and `lib/tags.ts` (`buildTagSuggestions`, a pure helper imported by `TagCombobox.tsx`). Put `formatReleaseDate` in `format.ts` (code in RESEARCH §Code Examples). Put `foldName`, `matchesName`, param parse/serialize, comparators, `selectVisible`, and the tag-union/carrier-count helpers in `watchlistView.ts` (RESEARCH Patterns 4, 7). It must not import from `~/lib/api` at runtime (types only).

### `web/app/lib/test/fixtures.ts`
**Analog:** `web/app/lib/test/routeStub.tsx` (shared test seam, excluded from coverage). Provide `makeEntry(overrides)` so the 8 fixture files stay type-correct.

---

### `web/app/components/watchlist/WatchlistToolbar.tsx` (component)

**Select pattern** (copy from `DigestSettings.tsx:165-179`):
```tsx
<Select value={displayed.digest_cadence} onValueChange={handleCadenceChange}>
  <SelectTrigger aria-labelledby="digest-cadence-label">
    <SelectValue>{(value: Cadence) => CADENCE_LABELS[value]}</SelectValue>
  </SelectTrigger>
  <SelectContent>
    <SelectItem value="daily">Daily</SelectItem>
```
Use six items, `aria-labelledby="watchlist-sort-label"`.

**Labelled-control layout** (copy from `HistoryFilters.tsx:227-237`): `<label className="flex flex-col gap-1 text-label text-muted-foreground">Text <Control/></label>`. Name filter uses `InputGroup` from `ui/input-group.tsx`; preference toggles are `Button` with `aria-pressed`. Empty state uses `EmptyState` (`~/components/common/EmptyState`) as at watchlist.tsx:357-375.

### `web/app/components/watchlist/WatchlistTagFilter.tsx` (component)

**Analog:** `TagCombobox.tsx` imports (5-18) and generic usage `Combobox<TagSuggestion> items value onValueChange inputValue ... itemToStringLabel` (111-120), built from `~/components/ui/combobox` parts: `Combobox, ComboboxChips, ComboboxChip, ComboboxChipsInput, ComboboxContent, ComboboxList, ComboboxItem, ComboboxEmpty, useComboboxAnchor`. It is a different shape (multiple, controlled `TagRef[]`, not creatable), so do not reuse `TagCombobox` itself. Keep the popup open via `onOpenChange={(open, d) => { if (!open && d.reason === "item-press") d.cancel() }}`, and prove it in a jsdom test (RESEARCH A2). Note `TagCombobox`'s `DISMISS_REASONS` (39-50) and `suppressEchoRef` (73) as precedent for base-ui close reasons and input echo handling.

### `web/app/components/ui/combobox.tsx` (vendored edit)
`ComboboxChip` (235-265): line 256 `className="-ml-1 opacity-50 hover:opacity-100"` becomes `text-muted-foreground hover:text-foreground`; add an optional `removeLabel` prop forwarded as the remove button `aria-label`. Chip className override per UI-SPEC: `h-6 text-label font-normal bg-secondary`.

### `web/app/components/watchlist/TagChips.tsx` (D-08)

**Current chip** (195-217): `Badge` containing `<span className="min-w-0 truncate ..." title={tag.name}>{tag.name}</span>` then the × `Button`. Replace the span with the UI-SPEC label `<button type="button" aria-pressed aria-label={\`Filter by ${tag.name}\`}>` (keep the `group-focus-within/chip` un-truncate classes). Pending chips (218-243) stay plain text.

**Trap** (lines 94-96): focus logic `containerRef.current?.querySelectorAll<HTMLButtonElement>("button")` then `buttons?.[index]?.focus()` assumes one button per chip. With two, select only remove buttons, e.g. add `data-chip-remove` to the × Button and query `[data-chip-remove]`. Re-run `TagChips.test.tsx` first. Extend `TagActions` (28-34) with `filterTagIds: ReadonlySet<number>` and `toggleFilterTag(tag)`.

### `web/app/components/watchlist/WatchlistRow.tsx`
**Analog:** itself. In the name column (40-58) insert, between the disambiguation span (47-51) and `<TagChips>` (52):
```tsx
<span className="text-label text-muted-foreground">...Latest release: {formatReleaseDate(...)} | No releases yet</span>
{entry.next_release_date && <span ...>Upcoming: ...</span>}
```
Plain JSX text only (XSS posture noted in the file's header comment).

### `web/app/components/history/HistoryFilters.tsx`
**Value shape** (7-10): add `tagId: number | null`. Add a third `<label>` after Event type, copying 239-247. Tag options come from `listTags()` fetched in the effect alongside `listWatchlist()` (195-210, same `cancelled` guard and `.catch` degrade). `Combobox<T>` hardening: `selectedOption` fallback at line 73 silently shows "All ..." for an unknown value, so add a `triggerLabel` override prop; add `max-h-72 overflow-y-auto` to the `<ul>` (162); on ArrowDown/Up call `el?.scrollIntoView?.({ block: "nearest" })` (optional-call, jsdom lacks it); add `max-w-64` to the trigger (36-37) and `truncate` plus `title` on the label span (151).

### `web/app/routes/history.tsx`
**Analog:** itself. Plan A (HIST-02): add `tagId` to `fetchHistoryPage` (62-68: `tagId: filters.tagId ?? undefined`), the effect deps (125: `[filters.artistId, filters.eventType, filters.tagId, reloadToken]`), `isFiltered` (157), and a tag branch in `emptyStateCopy` (40-57, order: hasOlderEvents, then tag, then isFiltered, then default). Plan B (G6, separate): swap `useState<HistoryFiltersValue>` (75-78) for `useSearchParams` via the shared writer.

### `web/app/routes/watchlist.tsx`
**Analog:** itself.
- Header comment (26-32) cites Phase 06 D-03/D-04 "never re-sorts client-side"; rewrite to 1-3 lines.
- Functional-updater helpers `handleEntryChange` (76), `addTag` (89), `removeTag` (108): each also adds the id to `stickyIds` state (G4).
- Manage-tags hook `applyTagChange` (the route's TagVocabularyProvider `onChange`, quick 261007-kt1): its `deleted` / `merged` branches also update the URL `tags` param through the ref-based writer with `replace` (D-13).
- `handleAddSearchResult` (249) and the Undo `.then(refresh)` (310): both need the new artist id added to `stickyIds`; `addWatchlist` response currently discarded at line 258.
- Render (377-396): wrap in `<section aria-labelledby="watchlist-artists-heading">` with heading row, toolbar, and `visible.map` or the filtered-to-zero `EmptyState`. Reuse `announce` (220) and the status div (333).
- The empty-entries state at 373-375 stays untouched.

## Shared Patterns

### Nullable columns via plain-column LATERAL
**Source:** RESEARCH Pattern 1 and Pitfall 1 (verified with sqlc v1.31.1). Apply to both watchlist queries. Assert `*string`.

### sqlc regeneration flow
`sqlc generate`, then `git add internal/db/sqlc`, then `make sqlc-check` (it diffs against the index, so unstaged output fails). `make` and `sqlc` may not be on the bash PATH (`~/go/bin/sqlc.exe`).

### Optional filter parsing
**Source:** `internal/httpserver/events.go:41-51` `parseOptionalPositiveInt64`. Apply to `tag_id`.

### Test mocking boundary
**Source:** `web/app/routes/history.test.tsx:15-22`: bare `vi.mock("~/lib/api")` automocks everything, so every History test must set `mockListTags.mockResolvedValue([])`. Route tests render via `renderRoute(Component, path)` (`lib/test/routeStub.tsx:12-15`), which supports initial query strings via the path.

### Copy and security
Tag/artist names render as plain JSX text. Copy strings are locked in UI-SPEC "Copywriting Contract", so copy verbatim. Comments: short, why-only (CLAUDE.md).

### Definition of Done
Prettier write on `web/` first, `go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check`, web typecheck/tests, and finally rebuild and commit the embedded bundle in `internal/webassets/build/client/` (Pitfall 12).

## No Analog Found

| File | Role | Data Flow | Reason |
|---|---|---|---|
| `web/app/lib/useUrlParams.ts` | hook | event-driven | No `useSearchParams` usage exists yet in the app. Use RESEARCH Pattern 3 (ref-based writer, always `preventScrollReset: true`) |
| `selectVisible` sticky-set logic | utility/state | transform | No existing client-side filter/sort pipeline. Use RESEARCH Patterns 4 and 7 |

## Metadata

**Analog search scope:** `queries/`, `internal/{watchlist,events,httpserver}`, `web/app/{lib,routes,components/{watchlist,history,system,ui,common}}`
**Files read:** about 14
**Pattern extraction date:** 2026-10-05
