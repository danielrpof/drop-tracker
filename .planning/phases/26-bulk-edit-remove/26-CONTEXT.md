# Phase 26: Bulk Edit & Remove - Context

**Gathered:** 2026-10-11
**Status:** Ready for planning

**Post-discuss grill amendments (2026-10-11):** grill decisions Q1–Q10 in `.planning/quick/261011-gq7-amend-phase-26-docs-with-grill-decisions/261011-gq7-DECISIONS.md` override this file where they differ. D-01, D-03, D-04, D-05, D-08, D-11 and D-17 are amended in place, D-19…D-22 are new, and D-22 settles the remove-tag id-versus-name discretion item. UI review amendments U1–U4 in `.planning/quick/261011-moj-amend-phase-26-ui-spec-and-context-with-/261011-moj-DECISIONS.md` add to D-10, D-15 and D-16, and 26-UI-SPEC.md carries the detail.

<domain>
## Phase Boundary

The user selects artists on the Watchlist, either individually or with "select all visible" over Phase 25's filtered view. They can then add or remove a tag, change release-type and mute preferences, or remove the selected artists, all in one action. Each action is backed by one set-based, transactional endpoint (`= ANY($ids)`). Remove is all-or-nothing behind a confirmation that states the count. Covers BULK-01..05. Bulk add (paste-a-list) belongs to Phase 27, and bulk note editing is out of scope.

</domain>

<decisions>
## Implementation Decisions

### Partial outcomes & API shape
- **D-01:** Bulk tag add **applies wherever it fits**. The tag lands on every selected artist that has room. Artists already at 10 tags are skipped and **reported by name** (e.g. "Added to 8 artists — 2 already at 10 tags: X, Y"). They are never skipped silently (SC2).
  Mechanism (grill Q3), in one transaction: (1) `GetOrCreateTag`; (2) `SELECT id FROM artists WHERE id = ANY(...) ORDER BY id FOR NO KEY UPDATE`, which takes the cap trigger's lock early and in a fixed order, so there is no snapshot race and no deadlock; (3) count links per artist and insert only where the count is under 10 or the link already exists, with `ON CONFLICT DO NOTHING`. The ADR 0004 trigger stays as the backstop. If zero links were inserted, roll back, so a tag created on the fly never becomes an orphan (24 D-29). Tests and the ADR amendment: D-19.
- **D-02:** For the reversible actions (tag add, tag remove, and preferences), a selected id that **no longer exists** (removed concurrently, for example in another tab) is **skipped as `gone`**. The rest of the batch still applies, the response reports the skip, and the client refreshes. This matches the idempotent attach and detach from Phase 24 (D-21).
- **D-03:** Bulk **remove** is all-or-nothing (BULK-04) with respect to real failures. An id that is **already gone counts as already removed**, because the goal state already holds, and the batch still succeeds. The toast mentions any already-gone ids. It is one `DELETE FROM watchlist WHERE id = ANY($1) RETURNING id` with no explicit transaction (grill Q6). Nothing references `watchlist` by foreign key, so the single statement is all-or-nothing, and requested ids it does not return are already gone. Do not wrap it in a transaction for safety.
- **D-04:** The reversible bulk endpoints return **the outcome only** (grill Q2): `{applied: [ids], skipped: [{id, name?, reason: 'cap'|'gone'}]}`, plus the carried count for remove-tag. The response carries **no entries**, which avoids a third copy of the heavy `ListWatchlist` projection and keeps `tags` from importing `watchlist`. The client **always** calls `refresh()` after a bulk success and re-derives from it. Exact field names are the planner's call. Phase 27 confirms by external id with different skip reasons and may choose to reuse this shape, but nothing here commits it to (grill Q7).

### Bulk preference semantics
- **D-05:** Preference changes are a **per-type delta**. The user changes individual types (for example "mute guest_feature" or "stop tracking deluxe"), and only those types change on every selected artist. Every untouched type keeps each artist's own value. Nothing overwrites a whole set. The request carries set and unset lists per axis (release types and muted event types), never full replacement sets. It can never touch notes (24 D-25). The server applies the delta as **one set-based statement** (grill Q8): a CTE locks the target ids `FOR UPDATE`, then updates only rows whose arrays actually change (`IS DISTINCT FROM`). `applied` is every locked (existing) id and `gone` is the requested ids minus the locked ones. Each axis is rebuilt in **canonical order** (the `ReleaseTypes`/`EventTypes` order `normalizeSet` uses) as current ∪ set minus unset.
- **D-06:** The bulk preferences editor uses **tri-state checkboxes**. A type is indeterminate when the selection is mixed, and clicking it cycles to on-for-all or off-for-all. Types left indeterminate are not sent. Use the vendored base-ui `Checkbox` (`web/app/components/ui/checkbox.tsx`), which supports `indeterminate`.
- **D-07:** Preferences open from the action bar as a **popover with an Apply button**. The user adjusts several types, then Apply sends one request and shows one toast. Nothing applies per click.
- **D-08:** Validation **mirrors single-artist `PATCH /watchlist/{id}`** for membership (`normalizeSet`), and an empty resulting release-type set is allowed. A delta therefore can't leave an artist invalid, and the only per-id skip for preferences is `gone`. The request-level 400 cases are D-21.

### Selection UX
- **D-09:** Each Watchlist card has an **always-visible leading checkbox**, with no separate selection mode. The list toolbar gets a tri-state **"Select all visible"** checkbox next to "N of M artists".
- **D-10:** A **sticky bottom action bar** appears when at least one artist is selected: "N selected · Add tag · Remove tag · Preferences · Remove · Clear". It stays visible while scrolling, and the filter toolbar stays visible too. The selection count is always on screen (SC1). The bar is one row at phone width (icon-only actions), per UI review U3.
- **D-11:** **Selection is derived, not trimmed** (grill Q1). `selectedIds` stays intact. The effective selection is `selectedIds` ∩ visible, where visible is the same `selectVisible(entries, {...view, q: query}, stickyIds)` the list renders (`web/app/lib/watchlistView.ts`), sticky cards (25 G4) included. A selected artist that a search or filter hides stays selected but is inert, neither counted nor sent, until it shows again, so a bulk action can never hit an artist the user can't see. Any bulk action, and Clear, empty the entire selection, hidden artists included. The bar count, the "Select all visible" state, and the "{k} selected" announcement use the effective selection. Trimming the selection to the visible set was rejected because `query` is local state that changes per keystroke, so it would drop selections while the user types.
- **D-12:** Bulk add-tag **reuses `TagCombobox`**, including create-on-the-fly (`Create “x”`) and the lazy vocabulary load (24 D-30). It runs through Phase 24 normalization and get-or-create (`internal/tags`). Bulk remove-tag offers **only tags carried by at least one selected artist**.
- **D-13:** Bulk tag and preference edits **mark every affected card sticky**, the same rule in-card edits follow (25 G4). After bulk-removing the filtered tag, those cards stay visible and show the result. Stickiness clears on the next filter, search, or sort change, as defined in 25 G4. Removed artists always disappear.
- **D-14:** After any bulk action, the selection **clears** and the Watchlist updates in place with no reload (BULK-05/SC5). Re-derive state from the refreshed watchlist, not from the id list as it was before the action, so no checkbox is left pointing at a removed artist.

### Bulk remove
- **D-15:** **Confirm only, with no Undo toast.** The confirmation that states the count is the safety net. Undo would need a bulk re-add that restores notes and preferences, which is Phase 27-shaped work. Tags survive anyway (TAG-07). Bulk remove-tag offers Undo (re-add by name to `applied` via bulk add-tag); this does not reverse D-15, which covers artist removal only (UI review U4).
- **D-16:** The confirm dialog reuses `ConfirmDialog` (`web/app/components/common/ConfirmDialog.tsx`, 24 D-16). It states the **count plus the first few names**, e.g. "Remove 14 artists? Drake, Bad Bunny, SZA and 11 more." The dialog is hosted at route level in `web/app/routes/watchlist.tsx`, not inside the action bar, whose `Remove` only opens it (UI review U1).
- **D-17:** The confirm carries **one plain sentence** on consequences, pending alerts, and the seed-mode re-add gap, e.g. "Their notes are deleted; tags are kept. Alerts already found for them may still be sent. If you add them back later, releases from the meantime may notify." (REQUIREMENTS Out of Scope, PITFALLS #6). Removing an artist does **not** withdraw its pending events (grill Q4): the outbox query (`ListUnnotified` in `queries/events.sql`) has no watchlist join, so alerts already detected are still delivered by the real-time flush or the next digest. Phase 26 keeps this behaviour and discloses it. The UI phase pins the exact wording.
- **D-18:** Bulk remove deletes `watchlist` rows only. Artist-scoped tags persist, and notes go with the row (24 D-10).

### Post-grill additions (2026-10-11)
- **D-19:** Bulk add-tag tests pin two cases (grill Q3): concurrent overlapping bulk attaches don't deadlock, and a new tag where every selected artist is at the cap leaves no tag row. Amending `docs/adr/0004-per-artist-tag-cap-trigger.md` with the ordered pre-lock for bulk insert is an implementation task of this phase.
- **D-20:** A new narrow `httpserver.WatchlistBulkStore` interface, implemented by `*watchlist.Service` and wired by a `WithWatchlistBulk` option, backs bulk remove and bulk preferences. Its routes return 503 when the option is absent, as `WithTags` and `WithSettings` do. Bulk tag add and remove are methods on `tags.Service`, exposed through `TagStore`. `watchlist.Store` is **not** extended, so the poller and the handler stubs stay untouched (grill Q5).
- **D-21:** Bulk preferences returns **400** when one type is in both `set` and `unset` on one axis, when a type is unknown, or when all four lists are empty, mirroring `ErrNoPreferencesSupplied` (grill Q8).
- **D-22:** Bulk remove-tag targets a **tag id**. A tag that no longer exists (deleted or merged concurrently) returns **404** `tag not found`, and the client shows the failure toast and calls `refresh()` (grill Q10).

### Claude's Discretion
- Endpoint paths and verbs. Candidates are `POST /watchlist/bulk/tags` (add/remove), `PATCH /watchlist/bulk/preferences`, and `POST /watchlist/bulk/delete`. The constraint: one set-based transactional endpoint per action, following the `AckEventsOnly`/`AckDigestBatch` `= ANY($ids)` precedent (`queries/notification_settings.sql`), and never N client calls.
- The maximum id count per request and the request body size limit, which must be bounded (roadmap security note). Size the limit generously above a realistic watchlist, e.g. 500–1000.
- Toast copy, how many names to list before "and N more", action-bar layout at phone width, and keyboard and shift-click range selection. The UI phase (`/gsd-ui-phase 26`) pins these, and the `ui-ux-pro-max` skill applies to visual choices.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` § Phase 26 — goal, success criteria, and the "Notes for the phase planner" (endpoints, security, seed-mode copy, UX)
- `.planning/REQUIREMENTS.md` — BULK-01..05, plus the Out of Scope entry on documenting the re-add seed-mode gap
- `.planning/quick/261011-gq7-amend-phase-26-docs-with-grill-decisions/261011-gq7-DECISIONS.md` — post-discuss grill Q1–Q10, which override this file where they differ

### Prior decisions this phase builds on
- `.planning/phases/24-artist-tags-notes/24-CONTEXT.md` — D-16 (reusable confirm dialog), D-18 (cap trigger), D-20/D-21 (entry-scoped, idempotent attach/detach), D-24 (functional optimistic updaters), D-25 (preferences request never sets notes), D-26 (shared projection), D-30 (lazy tag vocabulary)
- `.planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md` — D-10/D-12 (filters in URL), G4 (sticky cards; "N of M" counts visible)
- `docs/adr/0004-per-artist-tag-cap-trigger.md` — how the cap trigger behaves, and why it must hold under set-based inserts; amended in this phase with the ordered pre-lock for bulk insert (D-19)

### Research
- `.planning/research/PITFALLS.md` — #4 (auth/CSRF test pinning), #6 (re-add seed-mode gap)
- `.planning/research/FEATURES.md` — bulk-action UX norms (reversible edits use a toast, removal uses a confirm modal)

### DB conventions
- `internal/db/migrations/README.md` — read first only if a migration turns out to be needed. None is expected.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/components/common/ConfirmDialog.tsx` — reusable AlertDialog confirm (24 D-16), used by bulk remove
- `web/app/components/watchlist/TagCombobox.tsx` — tag picker with create-on-the-fly, used by bulk add-tag
- `web/app/components/ui/checkbox.tsx` — base-ui Checkbox (supports `indeterminate`), used for card checkboxes, select-all-visible, and the tri-state preferences
- `web/app/lib/watchlistView.ts` `selectVisible(entries, view, stickyIds)` — the single source of visibility for the list, select-all, and the effective selection (D-11)
- `web/app/routes/watchlist.tsx` — `stickyIds`, `markSticky`, `clearSticky`, `refresh()`, and the sonner toasts. Bulk handlers live here alongside the per-card handlers.
- `internal/tags/service.go` — `Attach` (normalization plus get-or-create), `Detach`. Bulk variants belong in this package.
- `internal/watchlist/service.go` — `UpdatePreferences` (single-round-trip merge, `normalizeSet`), `Remove`, `ReleaseTypes`/`EventTypes`
- `queries/notification_settings.sql` — `AckDigestBatch`/`AckEventsOnly` show the `= ANY($ids)` set-based precedent

### Established Patterns
- Postgres errors map by constraint name (`artist_tags_max_per_artist`, SQLSTATE plus index name), never by checking first.
- Every SPA call goes through `apiFetch`. Tags and names render as plain JSX text only.
- Optimistic updates use functional updaters with per-item rollback (24 D-24). Bulk actions can wait for the server response instead, because they report per-id outcomes.

### Integration Points
- New routes go inside `registerDataRoutes` (`internal/httpserver/server.go`, ~L265–292) so they inherit `gate.Authenticate` and `RequireCSRFHeader`. Pin the 401-without-session and 403-without-CSRF tests in the style of `internal/httpserver/tags_test.go` and `settings_test.go` (Phase 20-02).
- `WatchlistToolbar.tsx` gets the select-all-visible checkbox, `WatchlistRow.tsx` gets the per-card checkbox, and a new action-bar component goes under `web/app/components/watchlist/`.
- Bulk remove and bulk preferences reach the handler through the new `WatchlistBulkStore` (D-20), not `watchlist.Store`; bulk tag routes use `TagStore`.

</code_context>

<specifics>
## Specific Ideas

- Example toast for a partial apply: "Added to 8 artists — 2 already at 10 tags: X, Y".
- Example confirm: "Remove 14 artists? Drake, Bad Bunny, SZA and 11 more." followed by the notes, tags, pending-alert, and seed-mode sentence.
- The action bar follows the Gmail/Linear convention: a sticky bottom bar that shows the count.

</specifics>

<deferred>
## Deferred Ideas

- Undo for bulk remove (a bulk re-add restoring notes and preferences). Revisit once Phase 27's bulk-add endpoint exists.
- Withdraw pending events on remove (single and bulk), so a removed artist's already-detected alerts are not sent (grill Q4). Tracked in `.planning/todos/pending/2026-10-11-withdraw-pending-events-on-remove.md`.

### Reviewed Todos (not folded)
- The todos matched on keywords only (D-15 prev-release query files, the digest chunker `resuming` assignment, chunk-count fixture pinning, unifying History filters on base-ui, sqlscan quote state machines, digest plural grammar). None of them touch bulk edit, so they stay in the backlog. Chunker and fixture pinning belong to Phase 28.

</deferred>

---

*Phase: 26-bulk-edit-remove*
*Context gathered: 2026-10-11*
