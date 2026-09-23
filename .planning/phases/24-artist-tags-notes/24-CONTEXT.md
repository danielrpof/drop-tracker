# Phase 24: Artist Tags & Notes - Context

**Gathered:** 2026-09-22
**Status:** Ready for planning

<domain>
## Phase Boundary

Free-form tags and a plain-text note on each Watchlist card, plus a global tag vocabulary the user can rename, merge, and delete. Covers the migration (`tags`, `artist_tags`, `watchlist.notes`), the new `internal/tags` package, tag/notes API, `GET /watchlist` enrichment with `tags` + `notes`, and the card + "Manage tags" UI.

Requirements: TAG-01…07, NOTE-01. Not in this phase: filtering/sorting by tag (Phase 25), bulk tag edits (Phase 26), tags in Discord (Phase 28).

</domain>

<decisions>
## Implementation Decisions

### Card layout & editing
- **D-01:** Tag chips render under the artist name/disambiguation, wrapping inside the name column of `WatchlistRow`. Preference toggles and the remove button stay where they are.
- **D-02:** A compact "+ tag" button after the chips opens the autocomplete input in place; Esc or blur closes it. No always-visible input on every row.
- **D-03:** Chip add/remove is optimistic with rollback and an error toast, mirroring `PreferenceToggles`' optimistic-update-then-rollback pattern.
- **D-04:** Chips are plain labels with an × remove control. Clicking a chip does nothing in this phase (tag filtering arrives in Phase 25).
- **D-05:** The note shows inline under the chips as muted text, clamped to 2 lines with a "more" expander. An empty note shows only an "add note" affordance.
- **D-06:** The note is edited in place: textarea with a character counter and explicit Save / Cancel; Esc cancels. Clearing the text and saving clears the note.

### Tag management surface
- **D-07:** Global tag management is a "Manage tags" dialog opened from a button in the Watchlist header. No new route, no /system section.
- **D-08:** Each row shows the tag name and its watched-artist count (e.g. "reggaeton · 12 artists"), with Rename and Delete actions, sorted by name.
- **D-09:** Rename is inline in the list (name becomes an input with Save/Cancel). If the normalized new name collides with a different existing tag, a merge confirmation opens naming both tags; nothing merges without it. A case-only rename of the same tag is a plain rename, not a merge.

### Lifetimes & counts
- **D-10:** Notes live on the `watchlist` row: removing an artist deletes the note, and a re-add starts blank. Tags live on `artists.id` and survive remove/re-add (TAG-07). — **Reversibility:** one-way — moving notes to the artist later needs a migration plus a data backfill.
- **D-11:** Counts in Manage tags and in the delete confirmation include only artists currently on the watchlist. Deleting a tag still removes its links to removed artists too; they just aren't counted.
- **D-12:** A tag with zero carriers stays in the vocabulary (shown as "0 artists") until explicitly deleted. No auto-cleanup.
- **D-13:** Autocomplete suggests every tag in the vocabulary, including tags only removed artists carry — autocomplete and Manage tags show the same set.

### Limits & confirm UX
- **D-14:** At 10 tags the "+ tag" button is replaced by a muted "max 10 tags" hint; removing a chip restores it. API and DB still refuse an 11th tag.
- **D-15:** The tag input hard-stops at 32 characters (`maxLength`) with a small live counter that appears near the limit. API and DB still refuse >32.
- **D-16:** Merge and delete confirmations are modal alert dialogs built on `@base-ui/react`'s AlertDialog: title, one-sentence consequence naming the tag(s) and artist count, Cancel + primary/destructive action. Build it as a reusable component — Phase 26's bulk-remove confirm reuses it.
- **D-17:** After a successful merge/delete: a sonner toast (e.g. "Deleted “X” from 12 artists"), and both the Manage tags list and the Watchlist cards update without a page reload. No undo.

### Claude's Discretion
- **DB mechanism for the 10-tags-per-artist cap** (trigger vs. guarded insert that locks the artist row). Must hold under concurrent attaches and under Phase 26's set-based bulk attach. If a trigger is chosen, first confirm `cmd/migration-check`/`internal/sqlscan` tokenizes a dollar-quoted function body cleanly.
- API shape for rename-with-collision (e.g. 409 carrying target tag + resulting count, then an explicit merge call) and for tag attach/detach/notes endpoints.
- Where the counter threshold for D-15 starts, exact copy for hints/errors/toasts, and chip visual styling (the UI phase will pin these).
- `GET /watchlist` enrichment query form (correlated `array_agg` subquery recommended) — must stay one query, no per-artist follow-up.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone scope & requirements
- `.planning/ROADMAP.md` §Phase 24 — goal, success criteria, and "Notes for the phase planner" (schema, identity, rename-merge, lifetimes, security, UI stack)
- `.planning/REQUIREMENTS.md` — TAG-01…07, NOTE-01
- `.planning/PROJECT.md` §Current Milestone — v1.6 goal and constraints

### Research (v1.6)
- `.planning/research/SUMMARY.md` — cross-cutting decisions (tags keyed on `artists.id`, `UNIQUE lower(name)`, no new deps)
- `.planning/research/ARCHITECTURE.md` — `internal/tags` package shape, normalization layering, enrichment query
- `.planning/research/STACK.md` — `@base-ui/react` Combobox (`multiple`, chips, creatable); no new npm packages or Postgres extensions
- `.planning/research/PITFALLS.md` — #4 CSRF/session on new routes, #5 tag scope, #7 case-folding/NFC, #10 caps in the initial migration
- `.planning/research/FEATURES.md` — tag normalization and rename-merge conventions

### Schema rules
- `internal/db/migrations/README.md` — expand/contract rule; every cap must ship in migration `000010`; `make sqlc-check` has no CI counterpart

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `web/app/components/watchlist/WatchlistRow.tsx`: the card to extend (chips + note go in its name column).
- `web/app/components/watchlist/PreferenceToggles.tsx`: optimistic-update-then-rollback pattern via `onEntryChange` partial patches — reuse for chip add/remove.
- `web/app/components/ui/*` (shadcn over `@base-ui/react`: `badge`, `button`, `input`, `select`, `checkbox`, `sonner`): chip = badge-style; no dialog/alert-dialog component exists yet (D-16 adds one).
- `web/app/components/common/EmptyState.tsx`: empty state for a vocabulary with no tags.
- `internal/httpserver/watchlist.go` `trimAndCap` and `internal/watchlist/service.go` `normalizeSet`: existing normalization idioms the new `internal/tags` normalization should mirror.

### Established Patterns
- Handler fail-fast validation + service-layer non-bypassable backstop + DB `CHECK` (three-layer validation, Phase 02/20).
- `watchlist.Service.Remove` hard-deletes only the watchlist row (`TestService_Remove_LeavesArtistRowIntact`) — add the tag-survival counterpart.
- `ListWatchlist` returns the full list in one query ordered by name then id; the route never re-sorts.
- Plain JSX text rendering only (Phase 06 XSS posture) — no `dangerouslySetInnerHTML` for tags/notes.

### Integration Points
- `internal/httpserver/server.go:261` `registerDataRoutes` — register every new route here to inherit `gate.Authenticate` and `RequireCSRFHeader`.
- `web/app/lib/api.ts:184` `apiFetch` and `WatchlistEntry` (line 61) — all new SPA calls go through `apiFetch`; `WatchlistEntry` gains `tags` and `notes`.
- `queries/watchlist.sql` + sqlc regen; new `queries/tags.sql`.
- `web/app/routes/watchlist.tsx` — header gets the "Manage tags" button; `handleEntryChange`/`refresh` drive card updates after dialog actions.

</code_context>

<specifics>
## Specific Ideas

- Confirmation copy should name the tags and the count, e.g. merge: 'Merge “Latin” into “latin”? 7 artists will carry “latin”.'; delete: 'Delete “X” from 12 artists?'.
- The alert dialog is intentionally built for reuse by Phase 26 bulk remove.

</specifics>

<deferred>
## Deferred Ideas

- Clicking a tag chip to filter the Watchlist — Phase 25.

### Reviewed Todos (not folded)
All six pending todos matched only on generic keywords and are unrelated to tags/notes:
- Move shadcn to devDependencies — tooling chore, not this phase.
- Resolve D-15 previous-release files from `--prev-tag` — `cmd/migration-check` tooling.
- Unify sqlscan quote state machines — tooling (relevant only if a trigger's dollar-quoted body trips `internal/sqlscan`; revisit then).
- Dead `resuming = true` in digest chunker — notifier chore.
- Pin chunk-count test fixtures — already assigned to Phase 28.
- Singular/plural remainder marker grammar — notifier chore.

</deferred>

---

*Phase: 24-artist-tags-notes*
*Context gathered: 2026-09-22*
