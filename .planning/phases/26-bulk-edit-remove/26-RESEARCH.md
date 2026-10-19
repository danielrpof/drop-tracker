# Phase 26: Bulk Edit & Remove - Research

**Researched:** 2026-10-15
**Domain:** Set-based transactional Go/Postgres endpoints (sqlc + pgx) plus React multi-select UI on an existing chi/React Router app
**Confidence:** HIGH (SQL and lock semantics were run against a throwaway postgres:16, the same major version CI and compose use; frontend shapes are read from the repo)

<user_constraints>
## User Constraints (from CONTEXT.md)

Post-discuss grill amendments (2026-10-11): grill decisions Q1–Q10 in `.planning/quick/261011-gq7-amend-phase-26-docs-with-grill-decisions/261011-gq7-DECISIONS.md` override this file where they differ. D-01, D-03, D-04, D-05, D-08, D-11 and D-17 are amended in place, D-19…D-22 are new, and D-22 settles the remove-tag id-versus-name discretion item. UI review amendments U1–U4 add to D-10, D-15 and D-16, and 26-UI-SPEC.md carries the detail.

### Locked Decisions

**Partial outcomes & API shape**
- **D-01:** Bulk tag add **applies wherever it fits**. The tag lands on every selected artist that has room. Artists already at 10 tags are skipped and **reported by name** (e.g. "Added to 8 artists — 2 already at 10 tags: X, Y"). They are never skipped silently (SC2).
  Mechanism (grill Q3), in one transaction: (1) `GetOrCreateTag`; (2) `SELECT id FROM artists WHERE id = ANY(...) ORDER BY id FOR NO KEY UPDATE`, which takes the cap trigger's lock early and in a fixed order, so there is no snapshot race and no deadlock; (3) count links per artist and insert only where the count is under 10 or the link already exists, with `ON CONFLICT DO NOTHING`. The ADR 0004 trigger stays as the backstop. If zero links were inserted, roll back, so a tag created on the fly never becomes an orphan (24 D-29). Tests and the ADR amendment: D-19.
- **D-02:** For the reversible actions (tag add, tag remove, and preferences), a selected id that **no longer exists** (removed concurrently, for example in another tab) is **skipped as `gone`**. The rest of the batch still applies, the response reports the skip, and the client refreshes. This matches the idempotent attach and detach from Phase 24 (D-21).
- **D-03:** Bulk **remove** is all-or-nothing (BULK-04) with respect to real failures. An id that is **already gone counts as already removed**, because the goal state already holds, and the batch still succeeds. The toast mentions any already-gone ids. It is one `DELETE FROM watchlist WHERE id = ANY($1) RETURNING id` with no explicit transaction (grill Q6). Nothing references `watchlist` by foreign key, so the single statement is all-or-nothing, and requested ids it does not return are already gone. Do not wrap it in a transaction for safety.
- **D-04:** The reversible bulk endpoints return **the outcome only** (grill Q2): `{applied: [ids], skipped: [{id, name?, reason: 'cap'|'gone'}]}`, plus the carried count for remove-tag. The response carries **no entries**, which avoids a third copy of the heavy `ListWatchlist` projection and keeps `tags` from importing `watchlist`. The client **always** calls `refresh()` after a bulk success and re-derives from it. Exact field names are the planner's call. Phase 27 confirms by external id with different skip reasons and may choose to reuse this shape, but nothing here commits it to (grill Q7).

**Bulk preference semantics**
- **D-05:** Preference changes are a **per-type delta**. The user changes individual types (for example "mute guest_feature" or "stop tracking deluxe"), and only those types change on every selected artist. Every untouched type keeps each artist's own value. Nothing overwrites a whole set. The request carries set and unset lists per axis (release types and muted event types), never full replacement sets. It can never touch notes (24 D-25). The server applies the delta as **one set-based statement** (grill Q8): a CTE locks the target ids `FOR UPDATE`, then updates only rows whose arrays actually change (`IS DISTINCT FROM`). `applied` is every locked (existing) id and `gone` is the requested ids minus the locked ones. Each axis is rebuilt in **canonical order** (the `ReleaseTypes`/`EventTypes` order `normalizeSet` uses) as current ∪ set minus unset.
- **D-06:** The bulk preferences editor uses **tri-state checkboxes**. A type is indeterminate when the selection is mixed, and clicking it cycles to on-for-all or off-for-all. Types left indeterminate are not sent. Use the vendored base-ui `Checkbox` (`web/app/components/ui/checkbox.tsx`), which supports `indeterminate`.
- **D-07:** Preferences open from the action bar as a **popover with an Apply button**. The user adjusts several types, then Apply sends one request and shows one toast. Nothing applies per click.
- **D-08:** Validation **mirrors single-artist `PATCH /watchlist/{id}`** for membership (`normalizeSet`), and an empty resulting release-type set is allowed. A delta therefore can't leave an artist invalid, and the only per-id skip for preferences is `gone`. The request-level 400 cases are D-21.

**Selection UX**
- **D-09:** Each Watchlist card has an **always-visible leading checkbox**, with no separate selection mode. The list toolbar gets a tri-state **"Select all visible"** checkbox next to "N of M artists".
- **D-10:** A **sticky bottom action bar** appears when at least one artist is selected: "N selected · Add tag · Remove tag · Preferences · Remove · Clear". It stays visible while scrolling, and the filter toolbar stays visible too. The selection count is always on screen (SC1). The bar is one row at phone width (icon-only actions), per UI review U3.
- **D-11:** **Selection is derived, not trimmed** (grill Q1). `selectedIds` stays intact. The effective selection is `selectedIds` ∩ visible, where visible is the same `selectVisible(entries, {...view, q: query}, stickyIds)` the list renders (`web/app/lib/watchlistView.ts`), sticky cards (25 G4) included. A selected artist that a search or filter hides stays selected but is inert, neither counted nor sent, until it shows again, so a bulk action can never hit an artist the user can't see. Any bulk action, and Clear, empty the entire selection, hidden artists included. The bar count, the "Select all visible" state, and the "{k} selected" announcement use the effective selection. Trimming the selection to the visible set was rejected because `query` is local state that changes per keystroke, so it would drop selections while the user types.
- **D-12:** Bulk add-tag **reuses `TagCombobox`**, including create-on-the-fly (`Create “x”`) and the lazy vocabulary load (24 D-30). It runs through Phase 24 normalization and get-or-create (`internal/tags`). Bulk remove-tag offers **only tags carried by at least one selected artist**.
- **D-13:** Bulk tag and preference edits **mark every affected card sticky**, the same rule in-card edits follow (25 G4). After bulk-removing the filtered tag, those cards stay visible and show the result. Stickiness clears on the next filter, search, or sort change, as defined in 25 G4. Removed artists always disappear.
- **D-14:** After any bulk action, the selection **clears** and the Watchlist updates in place with no reload (BULK-05/SC5). Re-derive state from the refreshed watchlist, not from the id list as it was before the action, so no checkbox is left pointing at a removed artist.

**Bulk remove**
- **D-15:** **Confirm only, with no Undo toast.** The confirmation that states the count is the safety net. Undo would need a bulk re-add that restores notes and preferences, which is Phase 27-shaped work. Tags survive anyway (TAG-07). Bulk remove-tag offers Undo (re-add by name to `applied` via bulk add-tag); this does not reverse D-15, which covers artist removal only (UI review U4).
- **D-16:** The confirm dialog reuses `ConfirmDialog` (`web/app/components/common/ConfirmDialog.tsx`, 24 D-16). It states the **count plus the first few names**, e.g. "Remove 14 artists? Drake, Bad Bunny, SZA and 11 more." The dialog is hosted at route level in `web/app/routes/watchlist.tsx`, not inside the action bar, whose `Remove` only opens it (UI review U1).
- **D-17:** The confirm carries **one plain sentence** on consequences, pending alerts, and the seed-mode re-add gap, e.g. "Their notes are deleted; tags are kept. Alerts already found for them may still be sent. If you add them back later, releases from the meantime may notify." (REQUIREMENTS Out of Scope, PITFALLS #6). Removing an artist does **not** withdraw its pending events (grill Q4): the outbox query (`ListUnnotified` in `queries/events.sql`) has no watchlist join, so alerts already detected are still delivered by the real-time flush or the next digest. Phase 26 keeps this behaviour and discloses it. The UI phase pins the exact wording.
- **D-18:** Bulk remove deletes `watchlist` rows only. Artist-scoped tags persist, and notes go with the row (24 D-10).

**Post-grill additions (2026-10-11)**
- **D-19:** Bulk add-tag tests pin two cases (grill Q3): concurrent overlapping bulk attaches don't deadlock, and a new tag where every selected artist is at the cap leaves no tag row. Amending `docs/adr/0004-per-artist-tag-cap-trigger.md` with the ordered pre-lock for bulk insert is an implementation task of this phase.
- **D-20:** A new narrow `httpserver.WatchlistBulkStore` interface, implemented by `*watchlist.Service` and wired by a `WithWatchlistBulk` option, backs bulk remove and bulk preferences. Its routes return 503 when the option is absent, as `WithTags` and `WithSettings` do. Bulk tag add and remove are methods on `tags.Service`, exposed through `TagStore`. `watchlist.Store` is **not** extended, so the poller and the handler stubs stay untouched (grill Q5).
- **D-21:** Bulk preferences returns **400** when one type is in both `set` and `unset` on one axis, when a type is unknown, or when all four lists are empty, mirroring `ErrNoPreferencesSupplied` (grill Q8).
- **D-22:** Bulk remove-tag targets a **tag id**. A tag that no longer exists (deleted or merged concurrently) returns **404** `tag not found`, and the client shows the failure toast and calls `refresh()` (grill Q10).

### Claude's Discretion
- Endpoint paths and verbs. Candidates are `POST /watchlist/bulk/tags` (add/remove), `PATCH /watchlist/bulk/preferences`, and `POST /watchlist/bulk/delete`. The constraint: one set-based transactional endpoint per action, following the `AckEventsOnly`/`AckDigestBatch` `= ANY($ids)` precedent (`queries/notification_settings.sql`), and never N client calls.
- The maximum id count per request and the request body size limit, which must be bounded (roadmap security note). Size the limit generously above a realistic watchlist, e.g. 500–1000.
- Toast copy, how many names to list before "and N more", action-bar layout at phone width, and keyboard and shift-click range selection. The UI phase (`/gsd-ui-phase 26`) pins these, and the `ui-ux-pro-max` skill applies to visual choices.

### Deferred Ideas (OUT OF SCOPE)
- Undo for bulk remove (a bulk re-add restoring notes and preferences). Revisit once Phase 27's bulk-add endpoint exists.
- Withdraw pending events on remove (single and bulk), so a removed artist's already-detected alerts are not sent (grill Q4). Tracked in `.planning/todos/pending/2026-10-11-withdraw-pending-events-on-remove.md`.
- Reviewed todos (not folded): the todos matched on keywords only (D-15 prev-release query files, the digest chunker `resuming` assignment, chunk-count fixture pinning, unifying History filters on base-ui, sqlscan quote state machines, digest plural grammar). None of them touch bulk edit. Chunker and fixture pinning belong to Phase 28.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BULK-01 | Select multiple artists on the Watchlist, including "select all visible" | Derived selection `selectedIds ∩ selectVisible(...)` (D-11); pure `bulkSelection.ts` helpers; checkbox + range-select mechanics (Pitfalls F2, F5); heading-row select-all per UI-SPEC |
| BULK-02 | Add or remove a tag on all selected artists in one action | Verified ordered pre-lock + count-guarded `BulkAttachTag` SQL (Code Examples 1–2); `BulkDetachTag` SQL (Example 3); `tags.Service.BulkAttach/BulkDetach`; cap skips reported by name |
| BULK-03 | Set release-type filters and muted event types on all selected artists in one action | Verified delta CTE `BulkUpdateWatchlistPreferences` (Example 4) with canonical-order rebuild, `IS DISTINCT FROM`, nil-slice COALESCE guard; tri-state popover state machine |
| BULK-04 | Remove all selected artists after a count-stating confirmation; all-or-nothing | Single-statement `DELETE ... = ANY RETURNING id` (Example 5); route-hosted `ConfirmDialog` close-order mechanics (Pitfall F3); disclosure copy already pinned in UI-SPEC |
| BULK-05 | Selection clears and the watchlist reflects the result without a reload | Outcome-only responses + unconditional `refresh()`, `markStickyMany`, `pendingFocus` effect; optimistic removal filter for BULK-04; no skeleton flash (existing `refresh` keeps entries) |
</phase_requirements>

## Project Constraints (from `.claude/CLAUDE.md`)

- Definition of Done before every commit: `corepack pnpm --dir web exec prettier --write "**/*.{ts,tsx}"` first for any `web/` change (tailwind class order cannot be hand-formatted); then `go vet ./...`, `golangci-lint run`, `make test` (run `make db-up` first), `make coverage-gate` (80% backend floor), `make sqlc-check` (local only, no CI counterpart), and for web `prettier --check` + `corepack pnpm test` (70% gate on all four axes, `web/vitest.config.ts`).
- Never `git commit --no-verify`. No AI attribution trailers in commits or PRs.
- Comments: 1–3 lines of intent, one design-doc reference at most. No multi-paragraph headers (anti-pattern: `authStore.ts`).
- Migration work needs `internal/db/migrations/README.md` first. **No migration is needed for this phase** (verified: every bulk statement runs against the existing schema, see Code Examples).
- Stack is locked: chi v5, sqlc + pgx/v5, golang-migrate, React + Vite, no new npm packages. All SPA calls go through `apiFetch`. Names and tags render as plain JSX text only.
- `make sqlc-check` ends in `git diff --exit-code -- internal/db/sqlc/`, which only sees tracked files. After `sqlc generate`, `git add internal/db/sqlc/` (including the new untracked `bulk.sql.go`) before running it, or it can pass while hiding drift.
- Work must go through a GSD workflow (this research is part of `/gsd-plan-phase`).

## Summary

Phase 26 needs four server operations and one client selection model. Every server operation fits the existing seams without new packages, migrations, or changes to `watchlist.Store`. I wrote each candidate statement and ran it against a throwaway `postgres:16` (the same image `docker-compose.yml` and CI use), ran it through `sqlc generate` v1.31.1 in a scratch copy of the repo, and drove the preferences query through pgx from Go. The statements, results, and four non-obvious findings are in Code Examples and Common Pitfalls.

The four findings that change how the planner should write tasks: (1) **a nil `[]string` parameter becomes SQL NULL, and without a `COALESCE` the preferences delta wipes every selected artist's release types** (reproduced); (2) **sqlc cannot resolve `RETURNING cte.col` from a `DELETE ... USING <cte>`, and types `unnest()` output as `interface{}`**, so the SQL in this document is already rewritten around both; (3) the D-01 pre-lock is not optional: without it the count guard reads a stale snapshot and a concurrent 10th link aborts the whole batch with `artist_tags_max_per_artist` (reproduced), and with it the batch waits and then correctly skips that artist (reproduced); (4) `Watchlist` (the route function) renders `TagVocabularyProvider`, so it cannot call `useTagVocabulary()` itself, which constrains where the bulk add-tag handler lives.

**Primary recommendation:** Add `queries/bulk.sql` with the five statements below (already sqlc-verified), expose `BulkAttach`/`BulkDetach` on `tags.Service` and `BulkUpdatePreferences`/`BulkRemove` on `watchlist.Service`, register four static routes under `/watchlist/bulk/...` inside `registerDataRoutes` with a shared 1000-id / 64 KiB bound, and build the client around a pure `bulkSelection.ts` plus a `BulkActionBar` that lives inside the `TagVocabularyProvider`.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Selection state, effective selection (∩ visible), shift-range | Browser / Client | — | Pure UI state; derived from `selectVisible` over entries already loaded (D-11) |
| Per-type delta computation from tri-state boxes | Browser / Client | — | Initial/current state compare yields `set`/`unset` lists; server never sees full sets (D-05) |
| Tag add (normalize, get-or-create, cap, pre-lock) | API / Backend (`tags.Service`) | Database | Cap truth is the ADR 0004 trigger; service owns tx + ordering |
| Tag remove, preferences delta, delete | API / Backend | Database | Single set-based statements; Postgres row locks give atomicity |
| 10-tag cap backstop | Database (trigger) | — | Unchanged; the new pre-lock only makes the bulk path never trip it |
| Id-count / body bounds, dedupe, auth, CSRF | API / Backend (`httpserver`) | — | Registered inside `registerDataRoutes`, so the gate group applies |
| Canonical array order | API / Backend (Go `ReleaseTypes`/`EventTypes` passed as params) | Database | Single Go source of truth (D-05); SQL only applies the order it is given |
| Result display, toasts, focus, announcements | Browser / Client | — | Outcome-only responses; `refresh()` re-derives |

## Standard Stack

### Core (all already in the repo; no installs)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/go-chi/chi/v5` | v5.3.1 | Routing | `[VERIFIED: go.mod:7]` Static `/watchlist/bulk/...` segments beat `/watchlist/{id}` (probe below) |
| `github.com/jackc/pgx/v5` | in go.mod | Driver; `pgx.Tx` for the add-tag transaction | Existing `tags.DB` interface already exposes `Begin` |
| sqlc | v1.31.1 | Codegen | `[VERIFIED: sqlc version in ~/go/bin, matches Makefile SQLC_VERSION]` |
| `@base-ui/react` | 1.7.0 | Checkbox (`indeterminate`), Popover, Combobox | `[VERIFIED: web/node_modules/@base-ui/react/package.json]` |
| `sonner` | 2.0.8 | Toasts incl. `toast.warning`, `Toaster offset/mobileOffset` | `[VERIFIED: web/node_modules/sonner/dist/index.d.ts:112-113]` |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `shadcn` CLI | ^4.16.2 (devDep/dep in package.json) | Vendors the official `popover` component | Once: `corepack pnpm --dir web exec shadcn add popover` (needs network; answer **no** to any overwrite prompt) |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Pre-lock statement + guarded INSERT (D-01) | Single guarded INSERT | Reproduced failing: snapshot race aborts the batch on a concurrent 10th link |
| Delta CTE (D-05) | Read-modify-write in Go | Reintroduces the lost-update window `UpdateWatchlistPreferences` was built to avoid (G-02-2b) |
| Static `/tags/add` + `/tags/remove` paths | One `POST /watchlist/bulk/tags` with an `action` field | The two bodies differ (`name` vs `tag_id`); separate routes keep each DTO strict under `DisallowUnknownFields` |

**Installation:** nothing to `npm install` or `go get`. One vendored shadcn file (`web/app/components/ui/popover.tsx`) wraps the already-installed `@base-ui/react/popover` `[VERIFIED: web/node_modules/@base-ui/react/popover exists]`.

## Package Legitimacy Audit

No external package is installed or added to `go.mod`/`package.json` by this phase, so the `package-legitimacy` gate has nothing to evaluate. The only new third-party-derived code is the official shadcn `popover` source, copied from the project's pinned first-party registry (`web/components.json` has `registries: {}`; UI-SPEC "Registry Safety" records the 2026-10-11 review: wraps `@base-ui/react/popover` with `cn` only). `[CITED: 26-UI-SPEC.md Registry Safety]`

**Packages removed due to [SLOP] verdict:** none
**Packages flagged as suspicious [SUS]:** none

## Architecture Patterns

### System Architecture Diagram

```
Browser (watchlist.tsx)
  selectedIds ──► effective = selectedIds ∩ selectVisible(entries, view+query, stickyIds)
        │
        ├─ Add tag  ──► BulkActionBar (inside TagVocabularyProvider) ─► apiFetch POST /watchlist/bulk/tags/add
        ├─ Remove tag ─────────────────────────────────────────────────► POST /watchlist/bulk/tags/remove
        ├─ Preferences (delta from tri-state) ─────────────────────────► PATCH /watchlist/bulk/preferences
        └─ Remove ─► route-hosted ConfirmDialog ───────────────────────► POST /watchlist/bulk/delete
                                   │
        401 / 403 gate ◄── gate.Authenticate ► gate.RequireCSRFHeader (registerDataRoutes group)
                                   │
        handler: decodeJSONBody (DisallowUnknownFields) ► dedupe+bound ids (1..1000, each >=1)
                                   │
   ┌───────────────┬───────────────┴───────────────┬─────────────────┐
   ▼               ▼                               ▼                 ▼
 tags.BulkAttach  tags.BulkDetach        watchlist.BulkUpdatePrefs  watchlist.BulkRemove
 tx{GetOrCreate   (1) tag exists?        1 stmt: locked CTE         1 stmt: DELETE
  ► lock artists  (2) 1 stmt DELETE      FOR UPDATE ORDER BY id     = ANY RETURNING id
    ORDER BY id     links of existing    ► computed (canonical)
  ► guarded INSERT  entries ► carriers   ► UPDATE IS DISTINCT FROM
  ► rollback if                                   │                 │
    nobody carries}                               ▼                 ▼
        └───────────── Postgres: artist_tags / watchlist / tags ───┘
                                   │
   {applied, skipped[{id,name?,reason}], (tag | carried | already_gone)}  (no entries)
                                   ▼
   client: close surface ► clear selectedIds ► markStickyMany(applied) ► refresh() ► pendingFocus effect ► toast
```

### Recommended Project Structure
```
queries/bulk.sql                         # new: 5 statements (Examples 1-5) + GetTagByID
internal/db/sqlc/bulk.sql.go             # generated; commit it
internal/tags/bulk.go (+ bulk_test.go)   # BulkAttach, BulkDetach, result/skip types
internal/watchlist/bulk.go (+ _test.go)  # BulkUpdatePreferences, BulkRemove, sentinel errors
internal/httpserver/bulk.go (+ _test.go) # 4 handlers, WatchlistBulkStore, WithWatchlistBulk, DTOs, id bound
internal/httpserver/tags.go              # TagStore gains BulkAttach/BulkDetach (update fakeTagStore)
internal/httpserver/server.go            # serverConfig + Server field, 4 routes in registerDataRoutes
cmd/server/main.go                       # + httpserver.WithWatchlistBulk(store)
docs/adr/0004-per-artist-tag-cap-trigger.md   # amendment: ordered pre-lock for bulk insert
web/app/lib/bulkSelection.ts (+ test)    # pure: effective selection, range, select/deselect-all, delta, tri-state
web/app/lib/format.ts                    # + formatNameList(names, max = 3)
web/app/lib/api.ts                       # + BulkOutcome types and 4 wrappers
web/app/components/watchlist/BulkActionBar.tsx (+ test)
web/app/components/ui/popover.tsx        # vendored; checkbox.tsx indeterminate edit
web/app/routes/watchlist.tsx             # selection state, handlers, ConfirmDialog host, markStickyMany
```

### Pattern 1: Pre-lock, then guarded insert (D-01, verified)
**What:** inside one `pgx.Tx`: `GetOrCreateTag` → lock the target artists `ORDER BY a.id FOR NO KEY UPDATE OF a` → one guarded `INSERT ... ON CONFLICT DO NOTHING` that also reports per-artist `had_tag`/`inserted` → roll back if nobody ends up carrying the tag.
**Why it works:** Postgres applies `ORDER BY` before the row-lock step, so rows are locked in sorted order `[CITED: postgresql.org/docs/16/sql-select.html Caution: "ORDER BY is applied first. The command sorts the result, but might then block trying to obtain a lock on one or more of the rows."]`, confirmed by `EXPLAIN`: `LockRows -> Sort (Sort Key: a.id)` `[VERIFIED: postgres:16 probe, this session]`. The cap trigger takes the same lock mode, `FOR NO KEY UPDATE` on the artist row (`migrations/000012...up.sql`: `PERFORM 1 FROM artists WHERE id = NEW.artist_id FOR NO KEY UPDATE;`), and "a transaction never conflicts with itself" `[CITED: postgresql.org/docs/16/explicit-locking.html]`, so the trigger's own lock is free once the batch holds it. Because the INSERT is a **separate statement**, it takes a fresh READ COMMITTED snapshot after the locks are held, so the count guard sees every link committed before the lock was granted.
**Reproduced:** with a concurrent tx holding an uncommitted 10th link on artist 1, the pre-lock batch waited ~2s, then inserted artist 2 and skipped artist 1 (`inserted = f`) with no error. The control (no pre-lock, count guard under the statement snapshot) failed the whole statement: `ERROR: artist already has the maximum of 10 tags ... check_artist_tags_max_per_artist() line 27`. `[VERIFIED: postgres:16 probe, this session]`

### Pattern 2: Locked-CTE delta update (D-05, verified)
`locked` (`MATERIALIZED`, `ORDER BY id FOR UPDATE`) → `computed` (canonical-order array rebuild) → `updated` (`UPDATE ... FROM computed ... WHERE ... IS DISTINCT FROM`) → final `SELECT id FROM locked`. The final SELECT reads all of `locked`, so every row is locked even when no row needs an update (a locked-`SELECT` CTE is only run as far as demanded `[CITED: postgresql.org/docs/16/queries-with.html]`). The doc caveat that matters: "these clauses do not apply to `WITH` queries referenced by the primary query. If you want row locking to occur within a `WITH` query, specify a locking clause within the `WITH` query" `[CITED: postgresql.org/docs/16/sql-select.html]` — which is why `FOR UPDATE` is inside `locked`, not on the outer SELECT.
**Reproduced:** a concurrent writer had committed `muted_event_types='{new_release}'` on row 6 while the bulk statement waited; the bulk result for row 6 was `{new_release,guest_feature}` (nothing lost), matching Read Committed semantics where "it is the updated version of the row that is locked and returned" `[CITED: postgresql.org/docs/16/transaction-iso.html]`. A concurrent uncommitted DELETE of row 6 made it simply absent from `locked` (it becomes `gone`). `[VERIFIED: postgres:16 probe, this session]`

### Pattern 3: Outcome-only handler with non-nil slices
Every response slice must be non-nil (`make([]int64, 0, n)`), because Go encodes a nil slice as `null` and the client does `.length`/`.includes` on them. `json:"name,omitempty"` on skip names, since only `cap` skips carry one.

### Anti-Patterns to Avoid
- **Do not call `useTagVocabulary()` from `Watchlist`.** The route function renders `<TagVocabularyProvider>`, so the hook has no provider above it. Put the add-tag commit wrapper (which needs `remember`/`ensureLoaded`) in `BulkActionBar` or another child. `[VERIFIED: watchlist.tsx:538 renders the provider; TagChips.tsx:17,253 consume it as a child]`
- **Do not clear the selection inside `ConfirmDialog.onConfirm`.** `handleConfirm` calls `onOpenChange(false)` only in `finally` after `onConfirm` settles `[VERIFIED: ConfirmDialog.tsx:43-53]`. Record success in a ref/state inside `onConfirm`, then clear selection and set `pendingFocus` from the dialog's `onOpenChange(false)` (UI-SPEC U1 order).
- **Do not loop `Attach`/`Detach`/`UpdatePreferences` per id.** That is N requests or N transactions; the whole phase exists to avoid it.
- **Do not trim `selectedIds` to the visible set** (grill Q1). Pruning ids that no longer exist in `entries` (not visibility) is safe and optional.
- **Do not extend `watchlist.Store`** (grill Q5): `stubStore` in `watchlist_test.go` and the poller would break.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Tag name normalization / identity | New trim/NFC/lowercase | `tags.NormalizeName` + `GetOrCreateTag` (`INSERT ... ON CONFLICT ((lower(name)))`) | Go and Postgres identity must agree (PITFALLS #7); already used by `Attach` |
| Cap enforcement | Count check as the only guard | Keep the ADR 0004 trigger; the guarded INSERT is an optimization that produces named skips | A raw writer can still bypass app SQL; the trigger is the guarantee |
| Preference set validation + canonical order | New allow-list | `normalizeSet(values, ReleaseTypes/EventTypes, ErrInvalid*)` for `set`/`unset`, then pass `ReleaseTypes`/`EventTypes` as the order arrays | One Go source of truth `[VERIFIED: internal/watchlist/service.go:48-49]` |
| Request decode | `json.Unmarshal` | `decodeJSONBody` (rejects unknown fields and trailing values) + `http.MaxBytesReader` | `[VERIFIED: internal/httpserver/watchlist.go:118-128]` |
| Error → status table | Per-handler switch | Extend the `domainError` tables (`tagErrors` already maps `ErrTagNotFound` → 404) | One mapping per package |
| Confirm dialog, tag picker, checkbox | New components | `ConfirmDialog`, `TagCombobox` (generalize its `entry` prop), vendored `Checkbox` (+ indeterminate edit) | UI-SPEC |
| Toast stacking above the bar | Manual offsets per toast | `Toaster offset/mobileOffset` with `--bulk-bar-clearance` | UI-SPEC |

**Key insight:** every hard correctness property here (atomic delete, no lost update, no cap race, canonical order) is already provided by Postgres row locks plus one carefully shaped statement. The risk is in statement shape and parameter nullability, not in missing infrastructure.

## Runtime State Inventory

Omitted: this is not a rename/refactor/migration phase. No stored key, service config, OS registration, secret, or build artifact embeds a renamed string. (Verified: no new table, column, or migration.)

## Common Pitfalls

### Pitfall B1: nil slice = SQL NULL wipes release types (reproduced)
**What goes wrong:** pgx encodes a nil `[]string` as NULL. In the delta, `NOT (u.t = ANY(NULL))` is NULL, so the WHERE excludes every type and `computed.release_types` becomes `{}` for **every** selected artist. Reproduced: with `UnsetReleaseTypes` nil and `SetReleaseTypes = {ep}`, the post-state of three artists was `[[] [] []]`. `[VERIFIED: pgx probe against postgres:16, this session]`
**Why it happens:** the existing single-row query avoids it by initializing `ReleaseTypes: []string{}` (`UpdatePreferences`), but a request that omits `unset` decodes to a nil slice.
**How to avoid:** both layers. Wrap each array param in `COALESCE(@x::text[], '{}')` (done in Example 4; with it the same call produced the correct `[[album ep deluxe] [ep] [single ep]]`) **and** build the params with non-nil empty slices via `normalizeSet`, which never returns nil `[VERIFIED: service.go:553 make([]string, 0, len(allowed))]`.
**Warning signs:** a test that sends only `set` lists and asserts untouched types survive. Make it a required test.

### Pitfall B2: sqlc limitations hit by the natural SQL
**What goes wrong:** `DELETE FROM artist_tags l USING existing e ... RETURNING e.id` fails `sqlc generate` with `column "id" does not exist`; `SELECT unnest(@ids::bigint[]) AS artist_id` is typed `interface{}`. `[VERIFIED: sqlc 1.31.1 scratch run, this session]`
**How to avoid:** use the rewritten forms in Examples 3 and 2 (`WHERE l.artist_id IN (SELECT artist_id FROM existing) ... RETURNING l.artist_id`, and `t.artist_id::bigint AS artist_id`).

### Pitfall B3: `POST /watchlist/bulk/tags` is swallowed by `/watchlist/{id}/tags`
**What goes wrong:** with only the `{id}` routes registered, `POST /watchlist/bulk/tags` reaches the single-attach handler with `id="bulk"` and answers 400 `invalid watchlist id`; a typo in a bulk path fails the same confusing way. `[VERIFIED: chi v5.3.1 probe, this session]`
**How to avoid:** register the bulk routes as literal paths (chi gives static segments precedence: `/watchlist/bulk/tags/add`, `.../tags/remove`, `.../preferences`, `.../delete` all reached their own handlers). Add one test per route asserting it reaches the bulk handler (store call count 1) and not the `{id}` handler.

### Pitfall B4: overlapping writers and lock order
**What goes wrong:** two bulk tag adds locking artists in different orders can deadlock.
**Why it doesn't here:** every acquisition is in artist-id order (Pattern 1). Different-tag adds serialize only on shared artists; same-tag adds also serialize on the tag row, because `GetOrCreateTag`'s `DO UPDATE` locks it until commit. So the deadlock regression test must use **different tags** with **reversed id lists**. I could not force the unordered control into a deadlock with a single statement (the planner scans physical order either way), so this is a regression guard, not a proof; the proof is the documented lock order plus the `EXPLAIN`. `[ASSUMED: residual risk of two overlapping bulk DELETEs/link-DELETEs locking in different orders is negligible for a single-user app; the plain DELETE per grill Q6 was kept]`
**Warning signs:** `40P01 deadlock detected` in tests or logs.

### Pitfall B5: the tag orphan on all-capped create
**What goes wrong:** `GetOrCreateTag` inserts the tag, then every artist is capped, leaving an unattached tag (24 D-29).
**How to avoid:** roll back the transaction when no selected artist carries the tag after the guarded INSERT (`len(applied) == 0`). If the tag already existed, the rollback is harmless. If some artists already had it, commit (the tag cannot be an orphan). Pin with `countTagsByLowerName(...) == 0` after an all-capped bulk (the helper exists in `tags/service_test.go:33`).

### Pitfall B6: duplicate / malformed ids and non-null arrays
Dedupe ids (preserve first occurrence), reject `< 1`, reject empty, reject over the cap. `ANY(array)` tolerates duplicates, but `applied`/`gone` math and counts do not. Compute `gone = deduped requested − found`. Return `[]` not `null` for empty `applied`/`skipped`/`already_gone`.

### Pitfall B7: extending `TagStore` breaks the existing test double
`var _ httpserver.TagStore = fakeTagStore{}` at `tags_test.go:665` stops compiling the moment `TagStore` gains methods. Update `fakeTagStore` in the same task as the interface change, with call counters like `attachCalls`, so the 403-never-reaches-store assertions work for bulk.

### Pitfall F1: `vi.mock("sonner")` in the route test has no `warning`
`watchlist.test.tsx:41` mocks `toast` as `{ error, success }`. The bulk flows call `toast.warning`. New bulk tests need `{ error, success, warning }` (and `info` if used). Put bulk route tests in a new file (`watchlist.bulk.test.tsx`); the existing file is already 2601 lines.

### Pitfall F2: bare `vi.mock("~/lib/api")` erases `ApiError`
The route test automocks `~/lib/api`, so `err instanceof ApiError` and `err.code/.body` are meaningless. The bulk tests need the partial-mock pattern used by `ArtistNote.test.tsx`/`ManageTagsDialog.test.tsx`: `vi.mock("~/lib/api", async (importOriginal) => ({ ...(await importOriginal()), <wrappers>: vi.fn() }))`. This is required to test 404 `tag_not_found` and the too-many-ids toast.

### Pitfall F3: ConfirmDialog close order (U1)
See Anti-Patterns. Also: `ConfirmDialog.finalFocus` is typed `RefObject<HTMLElement | null>`, and base-ui reads `.current` when the dialog closes; assign the ref (bar `Remove` on cancel/failure, fallback target on success) **before** calling the close path, or widen the prop to the function form base-ui accepts (`finalFocus?: ... | ((closeType) => boolean | HTMLElement | null | void)`). `[VERIFIED: base-ui DialogPopup.d.ts:34]`

### Pitfall F4: `disabled` elements cannot receive focus
Select-all and card checkboxes are `disabled` while a request is in flight (UI-SPEC). The success-path focus move must run in an effect after in-flight ends **and** the bar has unmounted (`pendingFocus` flag consumed by `useEffect`), or `.focus()` silently no-ops.

### Pitfall F5: shift-click detection
base-ui fires `onCheckedChange(next, details)` from the hidden input's `onChange`, with `details.event = event.nativeEvent` `[VERIFIED: CheckboxRoot.js:214-215]`. Reading `details.event.shiftKey` works for a direct click on the input, but the UI-SPEC wraps the box in a `<label>`, and whether the label-synthesized click carries `shiftKey` is browser behaviour I could not verify here `[ASSUMED]`. Robust approach: capture `e.shiftKey` in the **label's** `onClick` (it fires before the change) into a ref, and read the ref in `onCheckedChange`. Keep range logic in a pure helper so it is unit-testable with no DOM.

### Pitfall F6: `ghost` ids and double counting after refresh
`refresh()` can drop an id (removed in another tab). Derived selection ignores ghosts, but the bar must show `effective.length`, never `selectedIds.size`. For BULK-04, additionally `setEntries(rows => rows.filter(r => !removedSet.has(r.id)))` on server success (it is confirmed state) so the cards vanish immediately, then `refresh()` resyncs.

## Code Examples

All five statements below were executed against `postgres:16` with the repo's migrations 000001–000012 applied, then compiled by `sqlc generate` v1.31.1 in a scratch copy of the repo (generated Go: `Lock...Row{EntryID, ArtistID, Name}`, `BulkAttachTagRow{ArtistID int64, HadTag, Inserted bool}`, `BulkDetachTagRow{EntryID int64, Removed bool}`, `BulkUpdateWatchlistPreferencesParams{Ids, AllReleaseTypes, SetReleaseTypes, UnsetReleaseTypes, AllEventTypes, SetEventTypes, UnsetEventTypes}` returning `[]int64`, `BulkDeleteWatchlist` returning `[]int64`). `[VERIFIED: postgres:16 + sqlc 1.31.1 scratch run, this session]` Put them in `queries/bulk.sql` with 1–3 line intent comments (comment discipline).

Schema facts these rely on, read this session: `watchlist.release_types`/`muted_event_types` are `text[] NOT NULL`, with CHECKs `release_types <@ ARRAY['album','single','ep','deluxe']::text[]` and `muted_event_types <@ ARRAY['new_release','guest_feature','deluxe_change']::text[]` `[VERIFIED: migrations/000002_watchlist.up.sql]`; `watchlist_artist_id_key UNIQUE (artist_id)` (one entry per artist, so entry↔artist is 1:1); no table `REFERENCES watchlist` (grep over `internal/db/migrations` returned nothing), so D-03's "no FK references" holds.

### Example 1: lock and resolve (tags tx, statement 2)
```sql
-- name: LockWatchlistArtistsForBulkTag :many
-- Locks the cap trigger's artist rows in id order before any insert (ADR 0004 bulk amendment).
SELECT w.id AS entry_id, a.id AS artist_id, a.name
FROM watchlist w JOIN artists a ON a.id = w.artist_id
WHERE w.id = ANY(@entry_ids::bigint[])
ORDER BY a.id
FOR NO KEY UPDATE OF a;
```
Rows returned = existing entries; `gone` = requested − returned. `FOR NO KEY UPDATE OF a` locks only `artists` rows, never `watchlist` (so it cannot interleave with bulk delete's `watchlist` row locks), and, being `NO KEY UPDATE`, does not block the FK `KEY SHARE` locks that `events`/`watchlist` inserts take `[CITED: docs/adr/0004 + postgresql.org/docs/16/explicit-locking.html]`.

### Example 2: count-guarded insert with per-artist outcome (tags tx, statement 3)
```sql
-- name: BulkAttachTag :many
-- Inserts only where the artist has room; reports who already had the tag and who was linked now.
WITH targets AS (
  SELECT unnest(@artist_ids::bigint[]) AS artist_id
), had AS (
  SELECT l.artist_id FROM artist_tags l
  WHERE l.tag_id = @tag_id::bigint AND l.artist_id = ANY(@artist_ids::bigint[])
), ins AS (
  INSERT INTO artist_tags (artist_id, tag_id)
  SELECT t.artist_id, @tag_id::bigint FROM targets t
  WHERE t.artist_id NOT IN (SELECT artist_id FROM had)
    AND (SELECT count(*) FROM artist_tags l WHERE l.artist_id = t.artist_id) < 10
  ON CONFLICT (artist_id, tag_id) DO NOTHING
  RETURNING artist_id
)
SELECT t.artist_id::bigint AS artist_id,
       (t.artist_id IN (SELECT artist_id FROM had))::boolean AS had_tag,
       (t.artist_id IN (SELECT artist_id FROM ins))::boolean AS inserted
FROM targets t ORDER BY t.artist_id;
```
Reading the result: carrier afterwards = `had_tag OR inserted`; cap skip = neither. Executed result for artists {1: room, 2: at cap, 3: already had, 4: room}: `(1,f,t) (2,f,f) (3,t,f) (4,f,t)`, and artist 2 stayed at 10 links. The literal `10` mirrors `MaxTagsPerArtist = 10` (`internal/tags/service.go:38`); the trigger remains the backstop and a trigger `check_violation` still maps to `ErrTagCapReached` via `mapTagError`.

### Example 3: bulk detach with carrier report
```sql
-- name: BulkDetachTag :many
-- Deletes the tag's links on the selected entries' artists; removed=true marks entries that carried it.
WITH existing AS (
  SELECT w.id, w.artist_id FROM watchlist w WHERE w.id = ANY(@entry_ids::bigint[])
), removed AS (
  DELETE FROM artist_tags l
  WHERE l.tag_id = @tag_id::bigint AND l.artist_id IN (SELECT artist_id FROM existing)
  RETURNING l.artist_id
)
SELECT e.id AS entry_id,
       (e.artist_id IN (SELECT artist_id FROM removed))::boolean AS removed
FROM existing e ORDER BY e.id;

-- name: GetTagByID :one
SELECT id, name FROM tags WHERE id = $1;
```
Run `GetTagByID` first; `pgx.ErrNoRows` → `tags.ErrTagNotFound` (404 `tag_not_found`, D-22). No transaction: one statement is atomic, and a tag deleted in the gap yields zero carriers, which is a harmless success. Entries returned with `removed=false` are unaffected (not reported); requested ids not returned are `gone`. Executed: ids `{1,2,9,500}` with only 1 and 2 carrying → `(1,t) (2,t) (9,f)`, 500 absent, links gone.

### Example 4: preference delta (single statement, D-05)
```sql
-- name: BulkUpdateWatchlistPreferences :many
-- Locks targets in id order, rebuilds each axis as (current ∪ set) − unset in canonical order, updates only changed rows.
WITH locked AS MATERIALIZED (
  SELECT id, release_types, muted_event_types FROM watchlist
  WHERE id = ANY(@ids::bigint[]) ORDER BY id FOR UPDATE
), computed AS (
  SELECT l.id,
    ARRAY(SELECT u.t FROM unnest(@all_release_types::text[]) WITH ORDINALITY AS u(t, ord)
          WHERE (u.t = ANY(l.release_types) OR u.t = ANY(COALESCE(@set_release_types::text[], '{}')))
            AND NOT (u.t = ANY(COALESCE(@unset_release_types::text[], '{}')))
          ORDER BY u.ord)::text[] AS release_types,
    ARRAY(SELECT u.t FROM unnest(@all_event_types::text[]) WITH ORDINALITY AS u(t, ord)
          WHERE (u.t = ANY(l.muted_event_types) OR u.t = ANY(COALESCE(@set_event_types::text[], '{}')))
            AND NOT (u.t = ANY(COALESCE(@unset_event_types::text[], '{}')))
          ORDER BY u.ord)::text[] AS muted_event_types
  FROM locked l
), updated AS (
  UPDATE watchlist w
  SET release_types = c.release_types, muted_event_types = c.muted_event_types, updated_at = now()
  FROM computed c
  WHERE w.id = c.id
    AND (w.release_types IS DISTINCT FROM c.release_types
         OR w.muted_event_types IS DISTINCT FROM c.muted_event_types)
  RETURNING w.id
)
SELECT l.id AS id FROM locked l ORDER BY l.id;
```
Executed results `[VERIFIED: postgres:16, this session]`:
- ids `{1,2,3,4,999}`, set `ep` / unset `deluxe` on release, set `guest_feature` on mute: artist 1 `{deluxe,album}` became `{album,ep}` (canonical order restored, not insertion order), artist 3 `{single,ep}` kept `ep`, artist 4's mute set grew to `{guest_feature}`, 999 absent from the result (gone).
- Re-running the identical delta changed nothing: an instrumented variant reported `changed = f` for all rows, so `IS DISTINCT FROM` skips no-op rows and `updated_at` is not bumped for them.
- Unsetting every release type left `release_types = {}` (D-08: an empty set is allowed; the CHECK `<@` accepts it).
- Concurrency and deletion cases: see Pattern 2.
- The pass-through `all_*` params are Go's `watchlist.ReleaseTypes`/`EventTypes`; `set`/`unset` come from `normalizeSet`.
- `AS MATERIALIZED` is required in spirit, not syntax-critical: `locked` is referenced twice and would be materialized by default `[CITED: postgresql.org/docs/16/queries-with.html]`; spelling it out documents that the lock must not be folded away. Postgres 16 in compose and CI supports it.

### Example 5: bulk delete (D-03, grill Q6)
```sql
-- name: BulkDeleteWatchlist :many
-- One statement is all-or-nothing; ids it does not return were already gone.
DELETE FROM watchlist WHERE id = ANY(@ids::bigint[]) RETURNING id;
```
Executed: ids `{11,12,500,11}` returned `11,12`. Keep this exactly as the decision locks it (no transaction, no pre-lock).

### Example 6: Go service shapes (sketch; names are recommendations)
```go
// internal/tags: BulkAttach mirrors Attach's tx scaffold (Service.db.Begin, qtx := s.q.WithTx(tx)).
type BulkSkip struct {
	ID     int64
	Name   string // artist name, set for "cap" only
	Reason string // "cap" | "gone"
}
type BulkAttachResult struct {
	Tag     Tag
	Applied []int64 // entry ids that carry the tag afterwards (new + already had), ascending
	Skipped []BulkSkip
}
func (s *Service) BulkAttach(ctx context.Context, entryIDs []int64, name string) (BulkAttachResult, error)

type BulkDetachResult struct {
	Applied []int64 // entry ids whose link was deleted (== carried); this is what Undo re-adds
	Carried int
	Skipped []BulkSkip // "gone" only
}
func (s *Service) BulkDetach(ctx context.Context, entryIDs []int64, tagID int64) (BulkDetachResult, error)
```
`BulkAttach` flow: `NormalizeName` → `Begin` → `GetOrCreateTag` → lock (Example 1) → map `gone` → `BulkAttach` (Example 2) → if no carrier, return result **without** `Commit` (the deferred `Rollback` runs) → else `Commit`. `mapTagError` already turns a trigger violation into `ErrTagCapReached` (`tagCapConstraint = "artist_tags_max_per_artist"`, `service.go:16`).

### Example 7: handler contract (sketch)
```go
// internal/httpserver/bulk.go
const (
	maxBulkIDs       = 1000  // Claude's discretion: 500-1000
	maxBulkBodyBytes = 65536 // 1000 ids x <=20 bytes ~ 20 KB
	codeBulkIDsRequired = "bulk_ids_required"
	codeBulkTooManyIDs  = "bulk_too_many_ids"
)
type bulkIDsRequest struct{ IDs []int64 `json:"ids"` }          // embed per route
type bulkTooManyResponse struct {
	Error string `json:"error"`; Code string `json:"code"`; Max int `json:"max"`
} // follows tagCollisionResponse precedent so the SPA reads err.body.max
```
Wire shapes (recommended; satisfy the UI-SPEC Data Contract): add-tag `{"tag":{"id","name"},"applied":[],"skipped":[{"id","name","reason"}]}`; remove-tag `{"applied":[],"carried":N,"skipped":[{"id","reason":"gone"}]}`; preferences `{"applied":[],"skipped":[{"id","reason":"gone"}]}`; delete `{"removed":[],"already_gone":[]}`. Preferences request: `{"ids":[],"release_types":{"set":[],"unset":[]},"muted_event_types":{"set":[],"unset":[]}}` decoded with `DisallowUnknownFields`; there is deliberately no `note` field, so over-posting one fails with 400 (24 D-25).

### Example 8: client pure helpers (sketch, in `web/app/lib/bulkSelection.ts`, no runtime imports from `~/lib/api`)
```ts
export function effectiveSelection(visible: WatchlistEntry[], selected: ReadonlySet<number>) {
  return visible.filter((e) => selected.has(e.id))
}
// Inclusive range in current visible order; `next` is the clicked box's new state.
export function applyRange(order: number[], anchor: number, clicked: number, next: boolean,
                           selected: ReadonlySet<number>): Set<number> { /* ... */ }
// initial/current tri-state per type -> { set: string[]; unset: string[] } (only changed types)
export function buildPrefDelta(initial: Record<string, TriState>, current: Record<string, TriState>) { /* ... */ }
```
The file header in `watchlistView.ts` already warns: "never import runtime values from ~/lib/api here, route tests automock that module" (`ApiError` is a runtime value).

## State of the Art

| Old Approach | Current Approach | Impact |
|--------------|------------------|--------|
| N client calls, one per artist | One set-based endpoint per action (`= ANY($ids)`) | Atomic, one round trip, one toast (`AckDigestBatch` precedent) |
| Check-then-insert with a count read | Lock first, then guard, with the trigger as backstop | No snapshot race (reproduced control failure) |
| Whole-set preference overwrite | Per-type delta with canonical rebuild | Mixed selections keep each artist's untouched types |

**Deprecated/outdated:** nothing relevant. `middleware.RealIP` is already a documented, gated exception in `server.go`; not touched here.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | A `<label>`-synthesized click carries `shiftKey` to the checkbox's change event in all target browsers | Pitfall F5 | Shift-range silently does nothing if the label route is used without the `onClick` capture; the recommended capture-in-label approach avoids depending on it |
| A2 | Residual deadlock risk between two overlapping plain bulk `DELETE`s or two bulk link deletes (different physical lock order) is negligible in a single-user app | Pitfall B4 | A rare `40P01` surfaces as a 500 and the "failure, nothing removed" toast; retry succeeds. Hardening (not recommended, contradicts grill Q6) would be a locking subselect |
| A3 | Bulk add-tag `applied` = every selected entry that carries the tag afterwards (new + already had), and the toast's `{n}` is `applied.length` | Example 6 / Open Question 1 | If the owner wants "Added to 7" excluding already-tagged artists, the response needs an additional `added` count; trivial server change but a copy/contract decision |
| A4 | `maxBulkIDs = 1000` and a 64 KiB body cap are "generous above a realistic watchlist" | Example 7 | Too low frustrates a very large "select all visible"; too high only costs a bigger `ANY` array. UI-SPEC says the server 4xx is the source of truth for `{max}` |
| A5 | `shadcn add popover` yields a file matching the UI-SPEC overrides (`PopoverTitle`, `w-80`) after the documented edits | Standard Stack | Executor may need a manual fix-up; the registry could have changed since the 2026-10-11 review |

## Open Questions

1. **What does `{n}` in "Added “x” to {n} artists" count?**
   - What we know: UI-SPEC does not say; remove-tag explicitly uses the carried count. With post-state semantics (A3), a selection where 3 of 10 already had the tag reads "Added to 10".
   - What's unclear: whether the owner prefers new-links-only.
   - Recommendation: ship post-state `applied` (also gives correct sticky marking and avoids a wrong "every artist at cap" toast when all already had the tag); add an optional `added` count only if the owner wants the stricter copy.
2. **Remove-tag `applied` vs `carried` redundancy.** The UI-SPEC says the Undo re-adds to `applied`, and the toast uses `carried`. If `applied` were "all existing selected ids", Undo would add the tag to artists that never had it. Recommendation: `applied` = entries whose link was deleted, so `carried == len(applied)`; keep `carried` in the payload for the contract.
3. **`PATCH` vs `POST` for preferences.** Either routes fine (verified). Recommendation: keep CONTEXT's `PATCH /watchlist/bulk/preferences`, the other three as `POST`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Docker Desktop | `make db-up`, all DB-backed tests | ✓ (was stopped at session start; I started it) | 29.1.3 | start Docker Desktop; tests skip without `TEST_DATABASE_URL` and a skip is not a pass |
| Postgres 16 | integration tests, sqlc schema source | ✓ via compose (`postgres:16`, same as CI) | 16 | — |
| Go | build/test | ✓ | 1.26.0 | — |
| sqlc | `make sqlc`, `sqlc-check` | ✓ at `~/go/bin/sqlc.exe`, **not on Git Bash PATH** | 1.31.1 | `export PATH="$HOME/go/bin:$PATH"` |
| golangci-lint | DoD | ✓ | 2.13.2 | — |
| Node / corepack pnpm | web tests, prettier | ✓ | 24.16.0 / 11.23.0 | — |
| Network (shadcn registry) | `shadcn add popover` | not tested | — | hand-write `popover.tsx` against `@base-ui/react/popover` using the UI-SPEC overrides |

Probe container `gsd26-research-pg` (port 55432) was removed after use; the project's own database and the working tree were not touched (`git status` shows only the pre-existing `graphify-out/` changes). Docker Desktop is still running.

## Validation Architecture

> Nyquist validation is enabled (`workflow.nyquist_validation: true`).

### Test Framework
| Property | Value |
|----------|-------|
| Backend framework | Go `testing`, real Postgres via `testutil.NewIsolatedTestPool(t, "<unique_schema>")` |
| Backend config | `Makefile` (`test-integration`), `TEST_DATABASE_URL` default `postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable` |
| Frontend framework | Vitest 4.1.10 + Testing Library + user-event, jsdom, `mockReset: true` |
| Frontend config | `web/vitest.config.ts` (coverage enabled, 70% on statements/branches/functions/lines) |
| Quick run (Go) | `TEST_DATABASE_URL=... go test ./internal/tags ./internal/watchlist ./internal/httpserver -run 'Bulk' -race -count=1` |
| Quick run (web) | `corepack pnpm --dir web exec vitest run app/lib/bulkSelection.test.ts app/components/watchlist/BulkActionBar.test.tsx app/routes/watchlist.bulk.test.tsx` |
| Full suite | `make test` (then `make coverage-gate`), `corepack pnpm --dir web test` |

Unused isolated-schema names today include `tags_service_test`, `tags_schema_test`, `tags_http_*`, `watchlist_*`, `settings_*`, `notifier_test`; use new unique ones such as `tags_bulk_test`, `watchlist_bulk_test`, `bulk_http_test` `[VERIFIED: grep of NewIsolatedTestPool callers]`.

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BULK-01 | Effective selection = selected ∩ visible incl. sticky; select-all-visible selects exactly visible; deselect clears only visible-selected; range select inclusive in visible order, anchor reset | unit | `vitest run app/lib/bulkSelection.test.ts` | ❌ Wave 0 |
| BULK-01 | Bar count and select-all tri-state use effective selection; hidden selection stays inert and returns when shown | component/route | `vitest run app/routes/watchlist.bulk.test.tsx` | ❌ Wave 0 |
| BULK-02 | Add: links all with room, existing link unaffected, at-cap artist skipped **by name**, gone id reported | integration | `go test ./internal/tags -run TestService_BulkAttach -race -count=1` | ❌ Wave 0 |
| BULK-02 | New tag + every artist capped leaves **no** tag row; cap/ADR trigger still refuses a raw insert | integration | `go test ./internal/tags -run TestService_BulkAttach_AllCappedLeavesNoTag` | ❌ Wave 0 |
| BULK-02 | Two overlapping bulk adds (different tags, reversed id lists) do not deadlock; a batch waiting on a concurrent 10th link skips that artist with no error (use the `pg_stat_activity` wait-for-Lock pattern from `TestSchema_TagCapTrigger_SameTagConcurrentAt9`) | integration | `go test ./internal/tags -run TestService_BulkAttach_Concurrent -race -count=1` | ❌ Wave 0 |
| BULK-02 | Remove tag: only carriers affected, `applied == carried`, unknown tag → `ErrTagNotFound` → 404 `tag_not_found`, idempotent second call | integration + http | `go test ./internal/tags ./internal/httpserver -run 'BulkDetach\|BulkTagRemove'` | ❌ Wave 0 |
| BULK-03 | Delta set/unset per axis; untouched types keep each artist's own value; canonical order; no-op rows keep `updated_at`; empty release set allowed; note untouched | integration | `go test ./internal/watchlist -run TestService_BulkUpdatePreferences` | ❌ Wave 0 |
| BULK-03 | nil `set`/`unset` slices never wipe release types (B1); same type in set and unset, unknown type, all four empty → 400 | integration + http | same package + `-run BulkPreferences` | ❌ Wave 0 |
| BULK-03 | Concurrent single-axis write is not lost (hold a tx update, then run bulk; mirror `TestService_UpdatePreferences_ConcurrentAxisWriteIsNotLost`); concurrent delete → gone | integration | `go test ./internal/watchlist -run BulkUpdatePreferences_Concurrent` | ❌ Wave 0 |
| BULK-03 | Tri-state popover: initial states, cycle order, only changed types sent, Apply disabled until change, failure keeps popover open | component | `vitest run app/components/watchlist/BulkActionBar.test.tsx` | ❌ Wave 0 |
| BULK-04 | All selected rows deleted; already-gone ids reported, batch succeeds; artists row and tags survive; notes go | integration | `go test ./internal/watchlist -run TestService_BulkRemove` | ❌ Wave 0 |
| BULK-04 | Confirm states count + first 3 names + disclosure sentence; confirm issues one request; failure keeps selection and says nothing removed | route | `vitest run app/routes/watchlist.bulk.test.tsx` | ❌ Wave 0 |
| BULK-05 | After any success: selection cleared, `refresh()` called once, applied ids sticky, no skeleton flash, no checked box for a removed artist, focus on fallback target | route | same | ❌ Wave 0 |
| All | 401 without session and 403 without `X-Requested-With`, **store never called**, for all four routes (Pitfall 4, Phase 20-02 style); 503 when option absent; each bulk path reaches its own handler | http | `go test ./internal/httpserver -run 'Bulk' -race -count=1` | ❌ Wave 0 |
| All | Over-limit ids → 400 `bulk_too_many_ids` with `max`; empty/invalid/duplicate ids; oversize body → 400; unknown JSON field → 400; non-null arrays in every response | http | same | ❌ Wave 0 |
| All | `sqlc generate` is clean and committed | static | `export PATH=$HOME/go/bin:$PATH; git add internal/db/sqlc; make sqlc-check` | n/a |

### Sampling Rate
- **Per task commit:** the matching quick-run line above, plus `go vet ./...`/`golangci-lint run` for Go tasks and `prettier --write` then the touched vitest files for web tasks.
- **Per wave merge:** `make test` (needs `make db-up`), `make coverage-gate`, `make sqlc-check`, `corepack pnpm --dir web test`.
- **Phase gate:** full suite green, plus the UI-SPEC "Verification items to carry into UAT" (network panel shows exactly one request per action; hidden-selection behavior; focus never behind the bar; 375px layout; contrast).

### Wave 0 Gaps
- [ ] `queries/bulk.sql` + `sqlc generate` (everything else compiles against it)
- [ ] `internal/tags/bulk_test.go`, `internal/watchlist/bulk_test.go`, `internal/httpserver/bulk_test.go` (extend `fakeTagStore`; add a `fakeBulkStore` with call counters)
- [ ] `web/app/lib/bulkSelection.test.ts`, `format.test.ts` additions for `formatNameList` (1, 2, 3, 4+ per UI-SPEC), `api.test.ts` additions for the four wrappers
- [ ] `BulkActionBar.test.tsx`, `watchlist.bulk.test.tsx` (partial `~/lib/api` mock + `sonner` mock with `warning`)
- [ ] Update `TagCombobox.test.tsx`/`TagChips` call sites when `entry` becomes `inputLabel` + `alreadyOnMessage`
- [ ] UI-SPEC 🧪 backstop rows (375px bar, popover, long names/tags, shift-click manual check) go to human UAT; they have no automated evidence here

## Security Domain

`security_enforcement` is enabled (ASVS level 1, block on high).

### Applicable ASVS Categories
| ASVS Category | Applies | Standard Control |
|---------------|---------|------------------|
| V2 Authentication | inherited | `gate.Authenticate` via `registerDataRoutes`; no new auth code |
| V3 Session Management | inherited | gate cookie; unchanged |
| V4 Access Control | yes | Register all four routes inside `registerDataRoutes` (`server.go:264-293`) so they sit in the `pr.Use(gate.Authenticate)` / `pr.Use(gate.RequireCSRFHeader)` group; pin 401 and 403 tests with the store-call counter |
| V5 Input Validation | yes | `decodeJSONBody` + `http.MaxBytesReader` (64 KiB), id count 1..1000, each id `>= 1`, dedupe, `normalizeSet` allow-lists, `tags.NormalizeName`; parameterized sqlc statements only |
| V6 Cryptography | no | — |
| V7 Error Handling | yes | Fixed error text via `writeDomainError`; never echo downstream error text; no tag/artist names in log attrs beyond existing patterns |
| V13 API | yes | State-changing verbs only (POST/PATCH); custom `X-Requested-With` header enforced by the gate for non-GET |

### Known Threat Patterns for this stack
| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unauthenticated or cross-site bulk delete (mass data loss) | Spoofing / Tampering / EoP | Gated group + CSRF header; tests prove 401/403 and zero store calls |
| Oversized `ids` array or body (resource exhaustion) | DoS | 1000-id cap, 64 KiB body cap, rejected before any DB call; `ANY` over ≤1000 bigints is cheap |
| Over-posting (`note`, `artist_id`, timestamps via bulk preferences) | Tampering | Dedicated DTO without those fields + `DisallowUnknownFields` |
| Lock contention holding artist/watchlist rows | DoS (self) | Locks live for one short transaction/statement; ordered acquisition prevents deadlock |
| Stored XSS via tag or artist names in toasts/confirm text | Tampering | Render as plain JSX text only; toast strings are template strings passed to sonner as text (Phase 06 posture) |
| Info disclosure via `skipped` names | Info disclosure | Names belong to the same single-tenant owner and are already returned by `GET /watchlist` |

## Sources

### Primary (HIGH confidence)
- Repo files read this session (cited inline with paths): `internal/tags/service.go`, `queries/tags.sql`, `queries/watchlist.sql`, `queries/notification_settings.sql`, `internal/watchlist/service.go`, `internal/httpserver/{server,tags,watchlist}.go`, `internal/httpserver/tags_test.go`, `internal/testutil/postgres.go`, `internal/db/tags_schema_test.go`, `internal/db/migrations/{000002,000010,000011,000012}`, `docs/adr/0004-per-artist-tag-cap-trigger.md`, `sqlc.yaml`, `Makefile`, `web/app/routes/watchlist.tsx`, `web/app/lib/{api,watchlistView,limits,tags,useTagVocabulary}.ts(x)`, `web/app/components/{common/ConfirmDialog,ui/checkbox,watchlist/*}.tsx`, `web/vitest.config.ts`, `@base-ui/react` Checkbox/Popover sources and type defs.
- Local experiments on `postgres:16` (migrations 000001–000012), `sqlc generate` 1.31.1 in a scratch copy of the repo, pgx probe from Go, chi v5.3.1 routing probe, all this session.
- PostgreSQL 16 docs: `postgresql.org/docs/16/sql-select.html` (ORDER BY + locking caution; locking clauses do not apply to WITH queries; `OF table_name`), `.../explicit-locking.html` (row-lock modes and conflict table; "a transaction never conflicts with itself"; consistent lock ordering), `.../transaction-iso.html` (Read Committed re-evaluation, "updated version of the row ... is locked and returned"), `.../queries-with.html` (data-modifying CTE execution; MATERIALIZED).

### Secondary (MEDIUM confidence)
- `26-UI-SPEC.md` (approved contract), `26-CONTEXT.md`, `261011-gq7-DECISIONS.md`, `.planning/research/PITFALLS.md` #4/#6.

### Tertiary (LOW confidence)
- None load-bearing. Browser label/shiftKey behavior is the only unverified claim (A1).

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH, no new dependencies; versions read from the repo and node_modules.
- Architecture / SQL: HIGH, every statement executed against postgres:16, compiled by sqlc, and the key concurrency properties reproduced (plus a failing control).
- Pitfalls: HIGH for B1–B3, B5, F1–F3 (reproduced or read from source); MEDIUM for B4 and F5 (reasoned / partly browser-dependent).
- Frontend structure: MEDIUM-HIGH, derived from reading the route and components; the UI-SPEC supplies the interaction detail.

**Research date:** 2026-10-15
**Valid until:** 2026-11-14 (stable stack; re-check only if `postgres`, `sqlc`, or `@base-ui/react` majors change)
