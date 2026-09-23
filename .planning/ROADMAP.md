# Roadmap: drop-tracker

## Overview

drop-tracker was built outward from the data layer: Postgres schema + config + health-checked skeleton, then a tested watchlist CRUD API, then rate-limited MusicBrainz/Deezer clients with live search, then the detection engine that diffs poll results into new-release/guest-feature/deluxe events, then Discord notifications, then the embedded React UI, then single-image containerization and the full GitHub Actions CI/CD pipeline that is the actual point of the project. v1.1–v1.2 hardened it, v1.3 delivered the deployment-readiness chain that needs no host, v1.4 made the running service legible (`/ready`, `/status`, a System view), and v1.5 added an operator-switchable daily/weekly Discord digest alongside real-time alerts.

v1.6 is about the watchlist itself, not detection or delivery. Past about 50 artists a flat list stops being manageable, so v1.6 lets the user label artists with free-form tags and a short note, find them by name, tag, or preference state, change or remove many at once, and add many at once from a pasted list. It then carries the tags into Discord alerts. There is no new background polling and no new npm package or Postgres extension. The work is one additive migration (`tags`, `artist_tags` keyed on `artists.id`, `watchlist.notes`), a new `internal/tags` package, set-based bulk endpoints, and a read-only `TagsReader` seam into the notifier. Paste-a-list reuses the existing `GET /search` one name at a time from the browser, so its only external traffic is interactive search the user starts and can watch.

## Milestones

- ✅ **v1.0 MVP** — Phases 1-7 (shipped 2026-08-12)
- ✅ **v1.1 Hardening & Scale Readiness** — Phases 8-11.1 (shipped 2026-08-17)
- ✅ **v1.2 Cleanup & Display Fixes** — Phases 12-13 (shipped 2026-08-24)
- ✅ **v1.3 Continuous Deployment** — Phases 14-17 (shipped partial 2026-09-09; **Phase 17 deferred**)
- ✅ **v1.4 Operator Observability** — Phases 18, 18.1, 19 (shipped 2026-09-11)
- ✅ **v1.5 Digest Notifications** — Phases 20-23 (shipped 2026-09-18)
- 🚧 **v1.6 Watchlist Organization** — Phases 24-28 (in progress)

Full phase-by-phase detail for every shipped milestone is archived under `.planning/milestones/v[X.Y]-ROADMAP.md`. Requirement archives: `.planning/milestones/v[X.Y]-REQUIREMENTS.md`. Accomplishment summaries: `.planning/MILESTONES.md`.

**Deferred:** **Phase 17 — Automated VPS Deploy with Health-Gated Rollback** (DPLY-01…08). Blocked on a provisioned VPS + domain the developer does not have yet. `discuss-phase` context was already gathered — archived at `.planning/milestones/v1.3-phases/17-automated-vps-deploy-with-health-gated-rollback/` (`17-CONTEXT.md`, `17-DISCUSSION-LOG.md`). Un-defer it as its own milestone cycle once a box exists; the `/ready` probe from v1.4 is built for its health-gate to consume.

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-7) — SHIPPED 2026-08-12</summary>

- [x] Phase 1: Foundation — Data Layer, Config & Health
- [x] Phase 2: Watchlist Core
- [x] Phase 3: External Clients & Search
- [x] Phase 4: Detection Engine
- [x] Phase 5: Discord Notifications
- [x] Phase 6: Frontend & Release History
- [x] Phase 7: Containerization & CI/CD Pipeline

</details>

<details>
<summary>✅ v1.1 Hardening & Scale Readiness (Phases 8-11.1) — SHIPPED 2026-08-17</summary>

- [x] Phase 8: Frontend Test Suite
- [x] Phase 9: CI Coverage Gates
- [x] Phase 10: Event Retention Window
- [x] Phase 11: Bounded Concurrent Polling
- [x] Phase 11.1: Address tech debt: v1.1 cleanup (INSERTED)

</details>

<details>
<summary>✅ v1.2 Cleanup & Display Fixes (Phases 12-13) — SHIPPED 2026-08-24</summary>

- [x] Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking
- [x] Phase 13: Fix History Dates, Guest-Feature Art & Artist Art

</details>

<details>
<summary>✅ v1.3 Continuous Deployment (Phases 14-17) — SHIPPED PARTIAL 2026-09-09</summary>

- [x] Phase 14: Instance Passphrase Gate (7/7 plans) — completed 2026-09-01
- [x] Phase 15: PR Coverage-Diff Comment (3/3 plans) — completed 2026-09-03
- [x] Phase 16: Rollback-Safe Migrations (5/5 plans) — completed 2026-09-05
- [ ] Phase 17: Automated VPS Deploy with Health-Gated Rollback — **DEFERRED** (no VPS); context archived, carries forward as its own milestone

</details>

<details>
<summary>✅ v1.4 Operator Observability (Phases 18, 18.1, 19) — SHIPPED 2026-09-11</summary>

- [x] Phase 18: Backend — Readiness, Status Surface & App Version (4/4 plans) — completed 2026-09-09
- [x] Phase 18.1: Poll-Cycle Instrumentation (3/3 plans) — completed 2026-09-11
- [x] Phase 19: Frontend — System View (5/5 plans) — completed 2026-09-11

</details>

<details>
<summary>✅ v1.5 Digest Notifications (Phases 20-23) — SHIPPED 2026-09-18</summary>

- [x] Phase 20: Digest Settings & Operator Control (4/4 plans) — completed 2026-09-13
- [x] Phase 21: Real-Time ↔ Digest Mutual Exclusion (3/3 plans) — completed 2026-09-16
- [x] Phase 22: Scheduled Digest Send (4/4 plans) — completed 2026-09-17
- [x] Phase 23: Digest Readability & Discord Limits (4/4 plans) — completed 2026-09-18

</details>

### 🚧 v1.6 Watchlist Organization (Phases 24-28) — IN PROGRESS

**Milestone Goal:** Keep a 50+ artist watchlist manageable. The user can label, annotate, find, bulk-change, and bulk-add artists, with no new background API polling.

- [ ] **Phase 24: Artist Tags & Notes** - Free-form, case-insensitive tags with autocomplete and a plain-text note on each Watchlist card, global tag rename/merge/delete, and tags that belong to the artist so they survive a remove and re-add
- [ ] **Phase 25: Find & Filter — Watchlist and History** - Client-side name search, stable sorts (including latest release), and tag/preference filters over the fully loaded watchlist, plus a retention-aware tag filter on the History feed
- [ ] **Phase 26: Bulk Edit & Remove** - Multi-select on the Watchlist with one-action tag add/remove, preference changes, and a count-confirmed all-or-nothing remove, backed by set-based transactional endpoints
- [ ] **Phase 27: Paste-a-List Bulk Add** - Paste names one per line, watch them resolve one search at a time within the existing rate limits, review best matches by confidence, and add the confirmed set in one request
- [ ] **Phase 28: Tags on Discord Notifications** - Tags on real-time embeds and digest lines, with markdown escaping added to the real-time path and the digest's chunking guarantees re-proven against longer lines

> **Ordering rationale.** Tags are the shared dependency for five of the seven features (autocomplete, rename/delete, both filters, Discord display), so the tags-and-notes slice lands first and carries the milestone's only schema migration. Every length cap goes into that migration at creation, because `cmd/migration-check` treats tightening an already-shipped column as a full expand/contract cycle. Filtering comes before bulk edit because "select all visible" is defined by the filtered view. History-by-tag goes in with the Watchlist filters instead of standing alone. It is a single requirement, it uses the same tag vocabulary and combobox, and on its own it would get task-shaped success criteria. Paste-a-list follows bulk edit and reuses its set-based, per-item-outcome endpoint shape for the confirm step. Discord tag display is last on purpose. It is the only change that touches Phase 23's chunker and ack invariants, so it has the milestone's highest blast radius, and it is best tested once real tags exist. It depends only on Phase 24, so it could run in parallel with 25-27, but it should not merge before its escaping and fixture-precondition work is done.
>
> **Deviation from research.** Research (SUMMARY.md) proposed a backend-only "data foundation" phase followed by a Watchlist UI phase. Phase 24 is instead a vertical slice: migration, then `internal/tags`, then API, then card UI. A migration plus a package plus routes with no UI has task-shaped success criteria, which is the same reason v1.5 folded its inert foundation phase into the settings-panel phase. The `latest_release_date` enrichment moves to Phase 25, the phase that uses it.

## Phase Details

### Phase 24: Artist Tags & Notes

**Goal**: The user can label any watchlist artist with free-form tags and a short note right on its Watchlist card, and manage the tag vocabulary globally. There is one tag per name regardless of casing or stray whitespace, and tags stay with the artist across a remove and re-add.
**Depends on**: Nothing (builds on shipped v1.5 code)
**Requirements**: TAG-01, TAG-02, TAG-03, TAG-04, TAG-05, TAG-06, TAG-07, NOTE-01
**Success Criteria** (what must be TRUE):

  1. On a Watchlist card, the user can type a tag, pick an existing one from autocomplete or create a new one on the fly, see it as a chip, and remove it again. The tags are still there after a reload, and still there after the artist is removed from the watchlist and added back.
  2. Entering `Reggaeton ` when `reggaeton` already exists attaches the existing tag, so no near-duplicate ever appears in autocomplete or on a card, and the first-entered casing is the one displayed. A tag over 32 characters, or an 11th tag on one artist, is refused with a clear message whether it comes from the UI or straight at the API, and the database refuses it even when the API check is bypassed.
  3. The user can rename a tag once and see the new name on every artist carrying it. Renaming onto a name that already exists asks for confirmation naming both tags before merging them, and nothing merges without that confirmation.
  4. The user can delete a tag globally after a confirmation stating how many artists carry it. The tag disappears from every artist, and the artists themselves stay on the watchlist untouched.
  5. The user can add, edit, and clear a plain-text note of up to 500 characters on an artist, and it shows on that artist's Watchlist card after a reload.

**Plans:** 2/7 plans executed

Plans:
**Wave 1**

- [x] 24-01-PLAN.md — Migration 000010 (tags, artist_tags, cap trigger, watchlist.note) and attach/detach through the API, enforced by the DB (wave 1)

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 24-02-PLAN.md — Tag vocabulary API: GET /tags with watched-only counts, rename with 409 collision, merge, delete (wave 2)
- [ ] 24-04-PLAN.md — Card chips: tags render under the artist name and × removes one (wave 2)

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 24-03-PLAN.md — Notes API: PUT /watchlist/{id}/note, POST note for Undo, shared tags+note projection on POST/PATCH (wave 3)
- [ ] 24-05-PLAN.md — Card "+ tag" autocomplete: create or pick, caps, lazy vocabulary (wave 3)

**Wave 4** *(blocked on Wave 3 completion)*

- [ ] 24-06-PLAN.md — Manage tags dialog: rename, confirmed merge, delete, reusable ConfirmDialog (wave 4)

**Wave 5** *(blocked on Wave 4 completion)*

- [ ] 24-07-PLAN.md — Card note display/editor, Undo restores the note, embedded bundle + full gate (wave 5)

**UI hint**: yes

**Notes for the phase planner**

- **Run `/gsd-ui-phase 24` first.** This phase introduces the chip combobox, the notes editor, and a tag-management surface.
- *Schema.* One additive migration; the next free number is `000010`. `tags(id, name, created_at)` with `CREATE UNIQUE INDEX ... ON tags (lower(name))` (not `citext`) and an inline `CHECK (char_length(name) BETWEEN 1 AND 32)`. `artist_tags(artist_id → artists ON DELETE CASCADE, tag_id → tags ON DELETE CASCADE, PRIMARY KEY (artist_id, tag_id))`, plus an index on `tag_id`. `watchlist.note TEXT NULL CHECK (char_length(note) <= 500)` (singular; see 24-CONTEXT D-28 for the added `btrim` checks). **Every cap goes in this migration.** A `CHECK` added later to a shipped column is a backward-incompatible finding (PITFALLS #10). Read `internal/db/migrations/README.md` first. `make sqlc-check` has no CI counterpart, so regenerate and commit locally.
- *Per-artist cap at the DB layer.* A `CHECK` can't count rows, so TAG-04's "enforced by DB" for 10-per-artist needs a trigger or a guarded insert that locks the artist row. It must hold under concurrent attaches and under Phase 26's set-based bulk attach. Pick the mechanism in discuss. If it is a trigger, first confirm that `cmd/migration-check`'s tokenizer (`internal/sqlscan`) accepts a dollar-quoted function body cleanly.
- *Identity.* Normalize in Go (trim, collapse internal whitespace, NFC via `golang.org/x/text/unicode/norm`, already a direct dependency), then compare through `lower(name)` in SQL so Go and the DB agree on identity (PITFALLS #7). Get-or-create is one atomic statement (`INSERT ... ON CONFLICT (lower(name)) DO NOTHING RETURNING id`, then a fallback `SELECT`), never check-then-insert. A rename that differs only in casing normalizes to the same tag and must be a plain rename, not a merge prompt.
- *Rename-merge.* The API has to detect the collision before committing, for example a 409 carrying the target tag and the merged artist count, then an explicit merge confirmation. The exact shape is the planner's call. A merge unions memberships and deletes the source tag. It can never push an artist over the 10-tag cap, because an artist carrying both tags collapses to one.
- *Lifetimes.* Tags key on `artists.id`, never `watchlist.id` (TAG-07, PITFALLS #5). `watchlist.Service.Remove` hard-deletes only the watchlist row, so artist-scoped tags survive and Phase 25's History-by-tag keeps working for removed artists. Add the tag version of `TestService_Remove_LeavesArtistRowIntact`. Notes live on the `watchlist` row (research's recommendation), so a note disappears on remove while tags survive. REQUIREMENTS.md doesn't state a note lifetime, so confirm it in discuss.
- *Open for discuss.* Should TAG-06's "how many artists carry it" count only watched artists, or also removed artists whose tags persist? Does a tag with zero carriers stay in the autocomplete vocabulary until it is explicitly deleted?
- *Enrichment.* `GET /watchlist` gains `tags` and `notes` inside its existing single query (correlated `array_agg` subquery), with no per-artist follow-up. Phase 25 adds `latest_release_date` to the same query.
- *Security.* Register every new route inside `registerDataRoutes` so it inherits `gate.Authenticate` and `RequireCSRFHeader`, and route every SPA call through `apiFetch`. Tags and notes render only as plain JSX text (no `dangerouslySetInnerHTML`, per the Phase 06 XSS posture).
- *UI stack.* `@base-ui/react` Combobox (`multiple`, chips, creatable pattern). No new npm packages (STACK.md). Web Definition of Done: `prettier --write`, then `corepack pnpm test` against the 70% frontend gate.

### Phase 25: Find & Filter — Watchlist and History

**Goal**: With a 50+ artist watchlist, the user can find any artist or group of artists in seconds, by name, tag, or preference state and in the order they choose, and can narrow the History feed to the same tags.
**Depends on**: Phase 24 (tags to filter by, plus the enriched `GET /watchlist` payload this phase extends)
**Requirements**: WLVW-01, WLVW-02, WLVW-03, WLVW-04, WLVW-05, WLVW-06, HIST-02
**Success Criteria** (what must be TRUE):

  1. Typing part of an artist's name narrows the Watchlist as the user types, and the view always shows "N of M artists".
  2. The user can sort by name (A–Z / Z–A), date added (newest / oldest), or latest release (the artist's newest own new-release date). Artists with equal values keep a stable order across re-sorts, and artists with no new release sit at the bottom in both directions.
  3. The user can filter by one or more tags, and to artists with muted event types or non-default release-type filters. Search, sort, and all filters combine, and a combination that matches nothing shows an empty state with one clear-filters action that restores the full list.
  4. The user can filter History by a tag alongside the existing filters. It lists events for artists carrying that tag, including artists since removed from the watchlist, and it never shows an event older than the retention window, either on the first page or via "load older".
  5. Opening the Watchlist makes one request regardless of watchlist size. Searching, sorting, and filtering then happen instantly with no further requests.

**Plans**: TBD
**UI hint**: yes

**Notes for the phase planner**

- **Run `/gsd-ui-phase 25` first.**
- *Latest release.* Add `latest_release_date` to `GET /watchlist`'s single query with one `LEFT JOIN LATERAL` over that artist's `event_type = 'new_release'` events. Never issue a per-artist query (PITFALLS #8). `release_date` is text of mixed precision (`2024`, `2024-05`, `2024-05-17`) that sorts lexicographically, the same property `ListEvents` already relies on, so decide how partial and NULL dates are treated. Check `EXPLAIN`; if an index is needed, it is a pure-additive migration.
- *Open for discuss.* Should latest release respect `EVENT_RETENTION_DAYS`? PITFALLS #8 recommends yes, so the Watchlist never reports a latest release that History can't show. Is the multi-tag filter any-of (OR) or all-of (AND)? Label the control to match. Does search/sort/filter state persist in URL query params across reload and back-navigation, or reset?
- *Client-side only.* REQUIREMENTS.md Out of Scope rules out server-side paging and sorting. Name sort should be locale-aware and case-insensitive (`Intl.Collator`), mirroring the digest's collated sort. Every sort mode tie-breaks on a stable key such as the artist id.
- *"Non-default release-type filters"* means `release_types` differs from the default, which is the full set `album, single, ep, deluxe` (`internal/watchlist/service.go:37`, migration 000002).
- *History by tag.* Add a third `sqlc.narg('tag_id')` `EXISTS` predicate over `artist_tags` inside the existing `ListEvents` **and** `HasOlderEvents` queries, never as a separate query (PITFALLS #9). Parse `tag_id` with the existing `parseOptionalPositiveInt64`. Add a retention regression test in the `TestRetention_*` style: seed an aged-out tagged event and an in-window tagged event, filter by the tag, and assert only the in-window one is returned.
- *Components.* Reuse the hand-rolled accessible combobox pattern in `web/app/components/history/HistoryFilters.tsx` (Phase 11.1, `aria-activedescendant`) for tag pickers instead of building another. **Decide in discuss:** Phase 24 vendors base-ui's `Combobox` (`web/app/components/ui/combobox`) for the card tag input, so the app will have two combobox implementations. Pick one for the filter pickers on purpose. base-ui's `multiple` mode fits a multi-tag filter. Keep three empty states distinct: no tags exist yet, the filter matched nothing, and no History events for this tag.

### Phase 26: Bulk Edit & Remove

**Goal**: The user can change many artists at once (tags, release-type filters, mutes, or removal) by selecting them on the Watchlist. Every change applies in one step, and removal requires confirming an explicit count.
**Depends on**: Phase 25 ("select all visible" is defined by its filtered view) and Phase 24 (bulk tag changes)
**Requirements**: BULK-01, BULK-02, BULK-03, BULK-04, BULK-05
**Success Criteria** (what must be TRUE):

  1. The user can select individual artists and use "select all visible", which selects exactly what the current search and filters show and never hidden artists. The selection count is always on screen.
  2. With artists selected, the user can add or remove a tag on all of them in one action. Artists that already have (or lack) the tag are unaffected, and an artist the addition would push past the 10-tag cap is reported by name instead of being silently skipped.
  3. The user can set release-type filters and muted event types on all selected artists in one action, and every selected artist shows the new settings afterwards.
  4. Removing the selection first asks for confirmation stating how many artists will be removed. Confirming removes all of them, or none of them if any part fails.
  5. After any bulk action the selection clears and the Watchlist shows the result immediately, with no page reload and no checkbox left pointing at a removed artist.

**Plans**: TBD
**UI hint**: yes

**Notes for the phase planner**

- **Run `/gsd-ui-phase 26` first.**
- *Endpoints.* Use one set-based, transactional endpoint per action (`= ANY($ids)`), following the `AckEventsOnly`/`AckDigestBatch` precedent, never N client calls. Bulk tag add reuses Phase 24's `internal/tags` normalization, get-or-create, and per-artist cap enforcement, and the cap must hold under the set-based insert, not only the single attach.
- *Open for discuss.* What are the partial outcomes for the reversible actions (tags, preferences)? Either the whole batch is atomic with a per-artist rejection list, or it applies what fits and returns a per-id outcome. BULK-04 fixes remove as all-or-nothing. Also decide what all-or-nothing means when a selected id was removed concurrently: fail the batch, or treat it as already gone. Finally, decide whether changing filters prunes selections that become hidden.
- *Remove semantics.* Bulk remove deletes `watchlist` rows only. Artist-scoped tags persist (TAG-07), and notes go with the row.
- *Seed-mode copy.* REQUIREMENTS.md Out of Scope commits to documenting the re-add gap in UI copy: a removed-then-re-added artist skips seed mode, so releases missed in the meantime may notify. The bulk-remove confirmation is the natural place for it (PITFALLS #6).
- *Security.* Register inside `registerDataRoutes` (gate + CSRF) and call through `apiFetch`. Pin the 401-without-session and 403-without-CSRF-header cases with tests in the style of Phase 20-02 (PITFALLS #4). Bound the request's id count and body size.
- *UX.* Reversible edits apply immediately with a toast, and only remove gets a confirm modal (FEATURES.md). Re-derive the selection against the refreshed watchlist, not the pre-action id list.

### Phase 27: Paste-a-List Bulk Add

**Goal**: The user can paste a list of artist names and add the right artists in one reviewed batch. They see what each line matched and can fix or skip the doubtful ones, and nothing is added until they confirm.
**Depends on**: Phase 26 (the confirm step reuses its bulk-endpoint conventions: protected route group, per-item outcomes, refresh without reload)
**Requirements**: IMPT-01, IMPT-02, IMPT-03, IMPT-04, IMPT-05
**Success Criteria** (what must be TRUE):

  1. Pasting names one per line starts one search per distinct name, with blank lines and repeated names dropped first. Progress is visible (for example "12 of 40"), and a cancel action stops any further searches.
  2. A 50-name paste resolves to completion without any request timing out and without exceeding the existing MusicBrainz and Deezer rate limits.
  3. The review screen shows each line's best match with a High, Medium, or Not found confidence. Only High rows start selected, and the user can pick an alternate candidate or skip any row.
  4. Artists already on the watchlist are flagged on the review screen, matched by their MusicBrainz/Deezer identity rather than by name, and are not added again.
  5. Nothing reaches the watchlist until the user confirms. Confirming adds every selected artist in one request and reports how many were added, skipped, and failed, and the new artists appear on the Watchlist without a reload.

**Plans**: TBD
**UI hint**: yes

**Notes for the phase planner**

- **Research flag: run `/gsd-sketch` on the review screen before `/gsd-ui-phase 27`.** It is the least precedented UI surface in the milestone.
- *Resolve is client-orchestrated (locked by REQUIREMENTS.md Out of Scope).* The SPA calls the existing `GET /search` once per name through `apiFetch`, with bounded concurrency (research suggests 3–5; MusicBrainz's ~1 req/s limiter is shared with the poller) and an `AbortController` for cancel. There is no server-side resolve endpoint, because the 15s `writeTimeout` fails any single blocking request past about 15 names (PITFALLS #3). Run UAT against a genuinely rate-limited client, not `rate.Inf`.
- *The confirm step has its own timeout trap.* `watchlist.Service.Add` runs an artist-art match bounded by an 8s `matchTimeout` whenever no image URL is supplied (`internal/watchlist/service.go:22-30`), so looping `Add` over a large batch brings back the same 15s write-timeout collision. Pass the search result's image URL through where one exists, and skip or defer matching for the rest. The existing artist-art backfill sweep only runs at startup, so decide in discuss whether art for bulk-added artists can wait until the next restart.
- *Open for discuss.* What concrete rules define the confidence tiers? One option: High = the top result's normalized name equals the pasted line; Medium = plausible but not exact, or several close candidates; Not found = no usable result. Don't pre-strip "Artist - extra" noise (Out of Scope). Also set a maximum paste size and a bound on the confirm body.
- *Already watched.* Compare external ids (`mbid` / `deezer_id`) against the loaded watchlist, and also flag two pasted lines that resolve to the same artist. In the confirm endpoint, an already-watched artist is a non-fatal "skipped", never a batch failure.
- *Carry-overs.* The re-add seed-mode copy from Phase 26 also applies to previously removed artists here. Applying tags at confirm time is deferred (REQUIREMENTS.md Future).

### Phase 28: Tags on Discord Notifications

**Goal**: Discord alerts show which of the user's groupings an artist belongs to, with tags on real-time embeds and digest lines, without opening a markdown-injection hole or weakening the digest's never-truncate guarantees.
**Depends on**: Phase 24 (tag data). Sequenced last on purpose; see the ordering rationale above.
**Requirements**: NTFY-05, NTFY-06, NTFY-07
**Success Criteria** (what must be TRUE):

  1. A real-time Discord alert for a tagged artist shows its tags. An artist with no tags gets the same embed it gets today, with no empty tags field.
  2. User- and artist-supplied text on real-time embeds renders literally in Discord. A tag like `**PRIORITY**` or `[x](https://example.com)`, or a release title containing `*`, `_`, or `[`, shows as typed and is never bold, italic, or a link, matching what the digest path already does.
  3. Each digest line shows its artist's tags while grouping stays by event type, and a heavily tagged artist's line stays within a fixed per-line cap.
  4. A large digest full of maximally tagged artists still splits into ordered messages within Discord's 4096-character limit, with no event dropped and nothing truncated without a visible marker. Phase 23's chunking tests still pass, and the fixture sizes they depend on are asserted rather than assumed.
  5. Tags are read at send time. A tag renamed or deleted before a pending event goes out shows up renamed, or not at all, in that alert or digest line.

**Plans**: TBD

**Notes for the phase planner**

- *Highest-risk phase of the milestone.* It touches Phase 23's chunker and ack invariants. Land the `TagsReader` seam inert first: a required constructor argument on `notifier.New`/`notifier.Select` like `SettingsReader` (D-05), an empty map, and every existing notifier test passing unchanged. Render tags only after that.
- *Batch, then render.* Fetch tags once per `NotifyPending` pass and once per `SendDigestIfDue` run for the distinct `artist_id`s, never per event. Tags render into `formatEmbed`/`digestLine` **before** `buildDigestGroups`/`chunkDigest` measure runes. Never decorate chunks afterward (ARCHITECTURE.md anti-pattern 2).
- *Whose tags.* Tags come from `events.artist_id`, the watched artist. On guest-feature rows the displayed `artist_name` is the host, so confirm that the tags shown belong to the watched artist the line is keyed by.
- *NTFY-07.* Move `escapeMarkdown` out of `digest_format.go` into a helper both paths share, and apply it on the real-time path to tags, titles, and artist names. Verify live which embed parts Discord actually parses as markdown: description and field values do, but check title and author, because escaping a part Discord renders raw shows literal backslashes. Add a table-driven test over every `markdownEscaper` metacharacter on both paths, plus one live Discord check.
- *Line budget.* Add a per-line tag cap (a `digestTagsLimit` alongside `digestArtistLimit`/`digestTitleLimit`) so one artist's tags can't produce an oversized line and trigger the `oversizedLineNote` fallback. Re-check whether the comment on `chunkOverheadReserve` (300 runes) needs to account for tags.
- *Fixture drift.* Before touching `digestLine`, add precondition assertions for `chunkForcingEventCount` (75) and `capForcingEventCount` (600) in `digest_test.go` so drift fails loudly (PITFALLS #2). This closes the pending todo `2026-09-18-pin-chunk-count-test-fixture-constants-with-a-precondition-t`. The other two Phase 23 review todos (dead `resuming = true` in `flushMidGroup`, and "1 events still pending") are in the same code and can be folded in.
- *Open for discuss: tag-read failure posture.* Recommendation: tags are decoration, so send without them and log a Warn, rather than failing closed the way the settings read does. Confirm this for the digest path, which acks per chunk.
- *Backend only.* There is no SPA work; the operator-facing surface is the Discord message itself.

## Progress

- **v1.0–v1.2:** shipped.
- **v1.3:** shipped partial — 3 of 4 phases (14, 15, 16). Phase 17 deferred, DPLY-01…08 carried forward.
- **v1.4:** shipped — Phases 18, 18.1, 19.
- **v1.5:** shipped — Phases 20-23.
- **v1.6:** in progress.

**Execution Order:**
Phases execute in numeric order: 24 → 25 → 26 → 27 → 28

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 24. Artist Tags & Notes | 2/7 | In Progress|  |
| 25. Find & Filter — Watchlist and History | 0/? | Not started | - |
| 26. Bulk Edit & Remove | 0/? | Not started | - |
| 27. Paste-a-List Bulk Add | 0/? | Not started | - |
| 28. Tags on Discord Notifications | 0/? | Not started | - |

## Backlog

*(none currently)*
