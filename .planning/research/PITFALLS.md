# Pitfalls Research: v1.6 Watchlist Organization

**Domain:** Adding tags/notes/search/sort/filter/bulk-ops to an existing Go+chi+sqlc+Postgres+React watchlist app with a Discord notifier
**Researched:** 2026-09-22
**Confidence:** HIGH (all findings grounded in direct reads of this repo's code, not general web research — this is an integration-pitfalls review, not an ecosystem survey)

## Critical Pitfalls

### Pitfall 1: Real-time Discord embeds have no markdown escaping — tags would be the first injectable field

**What goes wrong:**
`internal/notifier/format.go` (the real-time embed path used by `formatNewRelease`/`formatGuestFeature`/`formatDeluxeChange`) never calls any markdown-escaping function. It truncates `ev.Title`/`ev.ArtistName` with `truncateRunes` and puts them straight into `discord.Embed` fields. Markdown escaping (`escapeMarkdown`/`markdownEscaper` in `internal/notifier/digest_format.go`) exists **only** on the digest path, added in Phase 22/23 specifically because community-editable MusicBrainz/Deezer text could "terminate or retarget a masked link" (D-21/T-22-07). If a developer follows the nearest precedent when wiring "tags on real-time embeds" — copying `format.go`'s pattern (`appendField(embed.Fields, "Tags", tagString)`) rather than `digest_format.go`'s pattern — user-typed, autocomplete-suggested-but-freely-editable tag text reaches Discord unescaped. A tag like `] (https://evil.example` or a tag containing backticks/asterisks can break embed field rendering or, worse, retarget a masked link if tags are ever rendered inside one.

**Why it happens:**
Two call sites for "the same kind of text" (community/user text → Discord) with two different escaping postures already coexist in this codebase, and nothing enforces which one a new call site should follow. Tags are new, fully user-authored free text — a materially higher-trust-risk input than MusicBrainz/Deezer titles, since the *user themselves* controls every character with no upstream moderation at all.

**How to avoid:**
- Route every tag string that reaches a Discord payload (real-time embed field AND digest line) through `escapeMarkdown` (or a shared helper promoted out of `digest_format.go` into a location both `format.go` and `digest_format.go` import) before it is written into any `discord.Embed` field or digest line.
- Add a table-driven test mirroring `format_test.go`'s style asserting a tag containing every `markdownEscaper` metacharacter (`` \*_~`|>#[]() ``) renders escaped in the real-time embed, matching the existing digest-path test coverage.
- Treat this as an opportunity to close the pre-existing gap on `ev.Title`/`ev.ArtistName` in `format.go` too (they are already unescaped today) — not required for v1.6 scope, but flag it since the same phase is already touching this file.

**Warning signs:**
- A new `formatEmbed`/`appendField` call for tags with no `escapeMarkdown` in the diff.
- Manual Discord UAT where a tag like `**PRIORITY**` renders bold instead of literal.

**Phase to address:**
Phase implementing "tags on Discord real-time embeds and digest lines" — this is a hard blocker for that phase's own UAT, not a follow-up.

---

### Pitfall 2: Digest chunker's pinned chunk-count fixtures will silently drift once tags lengthen every line

**What goes wrong:**
`internal/notifier/digest_test.go` pins `chunkForcingEventCount = 75` and `capForcingEventCount = 600` as fixture sizes "empirically confirmed... to split into exactly 3 chunks" and "22 uncapped chunks" respectively, given the *current* per-line rune cost from `digestLine`/`lineLabel` (title capped at 100 runes + artist capped at 60 runes + fixed markdown wrapper). `chunkContentBudget` is a hard 3,796-rune ceiling (`discordDescriptionLimit` 4096 minus `chunkOverheadReserve` 300). If digest lines grow to include a tag suffix (e.g. ` [tag1, tag2]`), every line's rune count increases, which shifts the exact number of lines that fit `chunkContentBudget` — meaning 75 events might now split into 4 chunks instead of 3, and 600 events might now cross `maxDigestChunks` (20) well before all 600 are considered. These two constants are comments-pinned, not precondition-asserted (already flagged as known tech debt at v1.5 close) — a test using them will either fail confusingly (wrong chunk count) or, worse, keep passing against a re-tuned magic number that nobody re-derives against the real budget math.

**Why it happens:**
The fixture sizes were reverse-engineered against a fixed line-rendering function; any future change to that function (which digest-tag-display is) invalidates them without any compiler or lint signal, because the numbers are free-floating integers in a test file, not derived from `chunkContentBudget` or `digestLine`'s actual output.

**How to avoid:**
- Before adding tag rendering to `digestLine`, add a precondition assertion at the top of the affected tests (or a small helper) that computes the actual chunk count for `chunkForcingEventCount`/`capForcingEventCount` synthetic events and fails loudly with a clear message if it no longer matches the hardcoded expectation — turning silent drift into an explicit, actionable test failure that names the new right-hand-side number.
- Decide up front whether tags append to `lineLabel`'s output (raising `digestArtistLimit`/`digestTitleLimit`-style per-line cost) or are excluded from digest lines and only shown on real-time embeds — the milestone spec says both, so budget the worst case (an artist with many tags) explicitly, the same way `digestArtistLimit`/`digestTitleLimit` already cap other fields to keep a single line bounded.
- Add a cap on the number of tags (or total tag-text runes) rendered per digest line, mirroring the existing `digestTitleLimit`/`digestArtistLimit` pattern, so one heavily-tagged artist cannot blow a single line past `chunkContentBudget` and trigger the `oversizedLineNote` truncation fallback on an otherwise-ordinary release.
- Re-run `digest_chunk_test.go`'s property tests (the "three invariant property tests over a synthetic 700-event batch") after the change — they test structural invariants, not fixed counts, and are the correct regression net; the two named constants are the fragile part.

**Warning signs:**
- `TestSendDigestIfDue_MultiChunk*` tests failing with an unexpected chunk count after adding tag rendering.
- `TestSendDigestIfDue_Cap*` tests failing with the cap triggering at a different point than before.

**Phase to address:**
Phase implementing "tags on Discord real-time embeds and digest lines" — the same phase that touches `digestLine` must re-derive or defensively assert these two constants.

---

### Pitfall 3: Bulk add's 100-line paste cannot run as one synchronous HTTP request — 15s server WriteTimeout vs. 1 req/sec MusicBrainz limit

**What goes wrong:**
`cmd/server/main.go` sets `writeTimeout = 15 * time.Second` on the `http.Server` (alongside a 15s `ReadTimeout`), and `MUSICBRAINZ_RATE_LIMIT_PER_SEC` defaults to `1` (`internal/config/config.go`). A "review screen" backed by a single backend endpoint that loops over N pasted artist names, calling the MusicBrainz search client once per name to get "best match + alternates," takes roughly N seconds at N=100+ just from the shared `rate.Limiter` wait — 6-7x past the server's own `WriteTimeout`. Go's `net/http` server silently truncates/aborts a response once `WriteTimeout` fires; the client (and the passphrase-gated SPA sitting on top of it) would see a broken connection partway through, with no clean "still working" signal and no partial-results contract, on every paste over roughly 15 names.

**Why it happens:**
Every existing MusicBrainz-touching endpoint in this codebase (`GET /search`) is a single fast lookup, so the 15s `WriteTimeout` has never before collided with the 1 req/sec limiter's cumulative wait. Paste-a-list is the first feature whose natural implementation is "many sequential rate-limited external calls behind one user action."

**How to avoid:**
- Do not build this as one backend request that blocks until every name is resolved. Two viable shapes, both consistent with "single Go binary, no new background polling" (PROJECT.md's explicit v1.6 non-goal):
  - **Client-orchestrated:** the SPA calls the existing (or a near-identical) per-name search endpoint once per pasted line, sequentially or with bounded concurrency, rendering each row's match as it resolves — the *client*, not the server, absorbs the 1-2 minutes for a 100-line paste, and each individual HTTP request stays well under 15s.
  - **Server-side async job:** a POST kicks off a background resolve loop (in-process, keyed by a job id) and the SPA polls a status endpoint — more machinery, and closer to introducing "background work" the milestone goal says isn't needed; prefer the client-orchestrated shape unless a UX reason forces otherwise.
- Either way, add an explicit per-name rate-limit-respecting delay budget in the UI (a visible "resolving 34/100…" progress state) so users are not staring at a spinner for 100+ seconds with no feedback — this is also a UX requirement, not just a technical one.
- If a server-side loop is used for any part of this, wrap it in its own generous `context.WithTimeout` independent of the request's `r.Context()` lifetime expectations, and document why `writeTimeout` doesn't apply (streaming/chunked response, or a job-based design) rather than silently exceeding it.

**Warning signs:**
- A `POST /watchlist/bulk-add/resolve`-shaped handler with a `for _, name := range names { mbClient.SearchArtists(...) }` loop and no chunking/pagination in the request/response contract.
- Manual UAT with a real 50+ line paste hanging or erroring around the 15s mark.

**Phase to address:**
The phase implementing paste-a-list bulk add — this is an architecture decision that must be locked at plan time, not discovered during implementation.

---

### Pitfall 4: Bulk add/remove endpoints need the CSRF header and session-renewal contract wired identically to every other write route

**What goes wrong:**
`internal/authgate.RequireCSRFHeader` requires `X-Requested-With: drop-tracker` on every non-GET request, matched against `web/app/lib/api.ts`'s `apiFetch` helper, and `Authenticate` re-issues a sliding session cookie past the halfway mark of `sessionWindow` (30 days) on every gate-passing response. New bulk endpoints (`POST /watchlist/bulk`, `POST /watchlist/bulk-add`, etc.) must go through the same `pr.Use(gate.Authenticate)` / `pr.Use(gate.RequireCSRFHeader)` protected group as every existing write route, and any client-side bulk-add orchestration (Pitfall 3) that fires many sequential requests must use the same `apiFetch` wrapper — a hand-rolled `fetch()` call for the bulk-add loop (e.g. to add custom per-name progress tracking) is the likely place a developer forgets the CSRF header and gets a silent-looking wall of 403s when the instance is gated (`INSTANCE_PASSPHRASE` set).

**Why it happens:**
Bulk endpoints are new call shapes (multiple round trips per user action, or a bulk payload structurally different from the single-entity CRUD the CSRF/session code was proven against) — exactly the kind of "structurally different enough to feel like it needs its own client code" feature that tempts bypassing the shared `apiFetch` helper.

**How to avoid:**
- Route every new write call — both the per-name resolve calls in bulk-add and the actual bulk-edit/bulk-remove/bulk-add-confirm POST — through the existing `apiFetch` in `web/app/lib/api.ts`, not a parallel fetch path.
- Register every new bulk route inside the existing protected chi `Group`, never as a new top-level route (mirrors the `Authenticate`/`RequireCSRFHeader` registration discipline already documented in `gate.go`).
- Since a 100-name resolve loop can span minutes, verify the session's sliding-renewal window comfortably covers a single bulk-add session (30 days trivially does; flag this only if a shorter session lifetime is ever introduced).

**Warning signs:**
- New fetch/XHR calls in the bulk-add review-screen component that don't import `apiFetch`.
- 403 `{"error":"missing required header"}` responses appearing only from bulk-flow network calls during gated-instance UAT.

**Phase to address:**
Any phase adding a new write endpoint for bulk edit or bulk add — verify against `internal/authgate`'s existing test patterns (`gate_test.go`) rather than assuming the middleware "just applies."

---

### Pitfall 5: Where tags/notes are stored decides whether History tag-filtering survives a removed artist — get the FK wrong and bulk remove silently breaks it

**What goes wrong:**
This schema already has two tables with very different lifetimes: `artists` is master data (`internal/db/migrations/000002_watchlist.up.sql`'s own header: "identity keyed on MusicBrainz's mbid, independent of whether the artist is currently on anyone's watchlist"), and `watchlist` is the *membership* row, hard-deleted on remove (`Service.Remove`, `internal/watchlist/service.go:389-407`: "no status column, no soft-delete timestamp... the row is gone"). `events` references `artist_id`, not `watchlist_id`, specifically so event history survives watchlist removal (`ON DELETE CASCADE` runs `artists → watchlist` and `artists → events`, never `watchlist → events`). The milestone requires "History feed filterable by tag" — but History is explicitly about *past events*, which by design outlive watchlist membership. If tags are modeled as `watchlist_id`-scoped (the seemingly obvious place, since tags are described as a watchlist-entry feature), then `DELETE FROM watchlist` (confirmed-bulk-remove, or any single remove) cascades tags away with it, and every historical event for that artist becomes permanently untaggable/unfilterable in History from that point forward — even though the events themselves are untouched and still visible.

**Why it happens:**
"Tags per watchlist entry" reads naturally as "tag belongs to the watchlist row," but the feature list explicitly wants tags to reach a second, independent-lifetime surface (History) that this codebase already deliberately decoupled from watchlist membership via the `artists`/`events` FK design (documented rationale in the `000002` migration header and `Service.Remove`'s comment).

**How to avoid:**
- Model the tag-assignment join table (and any `artist_tags`/`tag_assignments`-shaped table) as `artist_id`-scoped (`REFERENCES artists(id)`), not `watchlist_id`-scoped, so tags persist for History filtering regardless of current watchlist membership — matching the existing `events.artist_id` precedent exactly.
- Decide explicitly (and document as a decision, since it affects bulk-remove's confirmation copy) whether "confirmed bulk remove" also strips tags from the artist (i.e., does removal delete the `artist_tags` rows, or only the `watchlist` row?) — if tags are meant to survive for History filtering, the remove path must NOT cascade through `artist_id`-scoped tag rows, meaning tags need their own explicit lifecycle decision independent of `ON DELETE CASCADE artists → watchlist`.
- Keep per-artist **notes** scoped to `watchlist_id` (per the milestone's own spec: "edited and shown on the Watchlist card only," i.e., not a History concern) — notes disappearing on remove is correct and matches existing hard-delete semantics; don't conflate the two features' storage design just because they're built in the same phase.
- Write the analogous test to `TestService_Remove_LeavesArtistRowIntact`/`TestService_Remove_ThenReAddSucceeds` for tags: "remove watchlist entry, assert artist's tags (for History) still queryable" and "remove watchlist entry, assert notes are gone."

**Warning signs:**
- A migration that adds `watchlist_id BIGINT REFERENCES watchlist(id) ON DELETE CASCADE` to a tags/tag-assignments table.
- History tag-filter UI returning empty results for events belonging to a since-removed artist, discovered only in UAT.

**Phase to address:**
The phase that designs the tags data model — this is a schema decision that's expensive to reverse once bulk remove ships against it (a migration + backfill, not a one-line fix).

---

### Pitfall 6: Re-adding a previously-removed artist silently skips seed-mode suppression — recent backlog can fire immediately instead of seeding quietly

**What goes wrong:**
Confirmed via `internal/detection/detector.go`'s `isSeedMode`: seed mode is `!HasAnyEvent(artist_id, source)` — purely "does this artist+source have zero rows in `events`, ever." Because `Service.Remove` only deletes the `watchlist` row and the `artists` row (plus its `events` rows) survive untouched, re-adding a previously-removed artist (Pitfall 5's re-add path, or the "paste-a-list" bulk-add matching against an existing MusicBrainz id) reuses the same `artist_id` and its pre-existing `events` rows — so `isSeedMode` returns `false` on the very next poll cycle, even though from the *user's* perspective this is a brand-new watch. Per `notifyGate.notifiedAt` (`internal/detection/detector.go:129-135`), non-seed-mode + release within the age cutoff means the event is queued for **real, immediate** Discord delivery — not silently seeded. Concretely: user removes Artist X, Artist X drops an EP two weeks later, user re-adds Artist X a month after that (well within typical `maxAgeDays` cutoffs) — the next poll cycle detects that EP as a brand-new external_id (nothing in `events` for it yet) and fires a live Discord notification for a month-old release the user has no context for, rather than treating it as backlog to seed quietly the way a genuinely brand-new artist's history would be.

**Why it happens:**
Seed mode's implicit "zero rows = first time" definition (a deliberate, documented v1 design choice — D-14) was correct for "artist never watched before," but bulk remove/re-add is a genuinely new user workflow this milestone introduces that breaks that assumption: an artist can now legitimately re-enter "first cycle since (re-)watching" state while `events` already has rows for it.

**How to avoid:**
- This is very likely acceptable/out-of-scope behavior to explicitly document rather than fix (re-deriving seed-mode-on-re-add would need a new signal — e.g., a `watchlist.created_at`-relative cutoff, or tracking a `first_seen_at` per (artist, watchlist-membership) pair — real scope creep for a v1.6 whose goal is explicitly "no new background API polling," i.e. no detection-engine changes).
- At minimum: surface this as a known-and-accepted behavior in the bulk-remove confirmation UI copy or in this milestone's PROJECT.md context notes (mirroring how the existing MusicBrainz TLS/`-race` limitations are documented as accepted, not silently left for someone to rediscover via a debug report) — "removing and re-adding an artist may trigger notifications for releases missed while off your watchlist" is a one-sentence warning that prevents a support/confusion cycle.
- Do **not** attempt to fix this by having bulk remove `DELETE FROM artists` (cascading to `events`) instead of only `watchlist` — that would resurrect the exact problem `Service.Remove`'s own doc comment says the design deliberately avoids (destroying detection state), and would also destroy the History-survives-removal property Pitfall 5 depends on.

**Warning signs:**
- A UAT report of "I removed and re-added an artist and got notified about an old release out of nowhere."
- Confirmed-bulk-remove's UI copy or the paste-a-list "artist already on watchlist, re-add?" review-screen path saying nothing about this.

**Phase to address:**
The phase implementing confirmed bulk remove (and separately, the paste-a-list review screen's "this artist was previously removed" case, if such matching is in scope) — document as a known limitation, don't silently ship it undocumented.

---

### Pitfall 7: Tag rename/dedup needs explicit case-folding and Unicode-normalization rules, or autocomplete + global rename silently fork the same tag

**What goes wrong:**
"Free-form multi-tags... autocomplete from existing tags... global tag rename/delete" implies tags are compared for equality/uniqueness somewhere (at minimum, autocomplete suggests existing tags; a global rename presumably means "rename this tag everywhere it's used," which requires identifying all rows that share "the same" tag). Without an explicit case-folding rule, a user typing `Hip-Hop` and later `hip-hop` produces two visually-identical but distinct tag values if uniqueness is a naive `=` comparison — autocomplete then either shows both as separate suggestions (confusing) or silently prefers one (surprising). Unicode adds a second axis: combining-vs-precomposed forms of the same visible string (e.g., `é` as U+00E9 vs. `e`+U+0301) are byte-distinct but visually and semantically identical, and would independently fork a tag with no visible difference to the user at all.

**Why it happens:**
Free-form text fields default to "store exactly what was typed, compare with `=`" unless a normalization step is deliberately inserted — and this codebase has no existing precedent for free-form user-authored *identity* fields (artist names/notes are free text but are never deduplicated/compared for identity; only MusicBrainz ids are).

**How to avoid:**
- Pick one explicit case-folding rule for tag *identity* (e.g., store the user's original casing for display, but enforce uniqueness and do autocomplete matching against a `citext` column or a generated lowercase column with a `UNIQUE` constraint) — Postgres's `citext` extension or a plain `LOWER(name)` unique index are both simple, proven options; pick one and document it in the migration's header comment (this repo's migration-comment convention, per every existing `.up.sql` file).
- Normalize to NFC (`golang.org/x/text/unicode/norm`) before the case-fold, at the point tags are created/renamed — do this once, server-side, not per-comparison, so stored data is already canonical and every future read is a trivial `=`.
- For sort/display ordering (not identity), reuse the collation pattern already proven in this codebase: `internal/notifier/digest_format.go` already imports `golang.org/x/text/collate` and builds a `collate.New(language.Und, collate.IgnoreCase)` per call (never package-level, since `collate.Collator` isn't concurrency-safe) — the same library is the natural fit for locale-aware, case-insensitive tag-list sorting in the Watchlist UI's tag filter/autocomplete, keeping the two features on the same normalization primitive rather than inventing a second one.
- Global rename should be a single `UPDATE` against the canonical tag row (if tags are a first-class `tags` table with a join table) rather than a fan-out `UPDATE` per assignment row referencing the old string value — this also makes rename atomic and trivially avoids the "renamed to a value that already exists as a different tag" merge case, which needs its own explicit decision (reject vs. merge assignments).

**Warning signs:**
- A tags table with a plain `TEXT UNIQUE` column and no case-insensitive index.
- Autocomplete implemented as a client-side `.filter(t => t.startsWith(query))` with no normalization, producing visually duplicate suggestions in manual testing.

**Phase to address:**
The phase that designs and implements the tags data model — the case-folding/normalization rule is a schema-and-query decision, not a UI nicety to patch in later.

---

### Pitfall 8: "Sort by latest release" is an easy N+1 (or a full-table-scan aggregate) bolted onto a query that today does zero joins into `events`

**What goes wrong:**
`ListWatchlist` (`queries/watchlist.sql`) is a plain two-table join (`watchlist` + `artists`) with no reference to `events` at all — "latest release" for an artist doesn't exist anywhere in the watchlist query today. The naive implementation path is: fetch the watchlist list, then for each artist, issue a separate `SELECT MAX(created_at) FROM events WHERE artist_id = $1` (or reuse the release-date field) — a textbook N+1 that's invisible at the 5-10 artist scale used in dev/tests but directly costs one extra round trip per artist at the "50+ artist watchlist" scale this milestone's own goal names as the target.

**Why it happens:**
The existing codebase's query-per-artist habits (search proxy does one round trip per source, not per artist) don't have a "fetch related data for N rows" precedent to imitate, and hand-writing a per-row loop in Go is the path of least resistance when a developer is focused on getting sorting to "work" first.

**How to avoid:**
- Compute latest-release-per-artist as a single query: either a `LEFT JOIN LATERAL (SELECT created_at FROM events WHERE events.artist_id = a.id ORDER BY created_at DESC LIMIT 1) e ON true`, or precompute it via a `GROUP BY artist_id` subquery/CTE joined once — either shape is one round trip regardless of watchlist size, matching this codebase's existing single-round-trip query style (`ListWatchlist`, `AdvanceGroupTrackCountBaseline`'s CTE, etc.).
- Add `events_artist_source_idx` isn't quite the right index for this (it's `(artist_id, source)`); a sort-by-latest-release query benefits from `(artist_id, created_at DESC)` — check `EXPLAIN ANALYZE` on this query once written, and add a migration for a supporting index if the planner isn't already using `events_artist_source_idx` efficiently for it.
- Decide whether "latest release" should respect `EVENT_RETENTION_DAYS` (Pitfall 9) — almost certainly yes, so a retention-aged-out release doesn't appear as "latest" in the sort while being invisible everywhere else in the UI, which would look like a bug (Watchlist says "latest: Jan 2025" but History shows nothing that recent).

**Warning signs:**
- A Go-side `for _, entry := range watchlist { latest := s.q.GetLatestEventForArtist(ctx, entry.ArtistID) }` loop.
- Watchlist page load time scaling visibly with watchlist size once past ~30-50 artists.

**Phase to address:**
The phase implementing Watchlist sort — write the single-query version from the start; this is cheap to do right and expensive to retrofit once N+1 code and its tests exist.

---

### Pitfall 9: `EVENT_RETENTION_DAYS` and the new History tag filter must compose, not diverge — two independent filters on the same query is where the seam usually leaks

**What goes wrong:**
History's retention filtering already lives as its own deliberately-scoped concern: `internal/httpserver/events.go`'s `ListEvents`/`Service.List` applies the `EVENT_RETENTION_DAYS` window (hiding aged-out rows from `GET /events`/History), while the codebase's own test suite (`events_test.go`) explicitly guards against a "consistency pass" that accidentally *adds* the retention predicate to detection-state queries (`ListExternalIDs`, `HasAnyEvent`) that must never have it — i.e., this codebase already has a documented history (Phase 10) of retention filtering nearly leaking into the wrong query. Adding a tag filter to History introduces a second predicate on the same `GET /events` path; if it's implemented as a second, independently-constructed query (rather than composed into the same retention-aware query/Service.List call), it's easy to either (a) apply the tag filter to a query that bypasses retention (showing tag-filtered aged-out rows the un-tag-filtered view correctly hides — an inconsistency a user would notice immediately when a tag filter shows "more history" than no filter does) or (b) apply retention twice/inconsistently across the two filter dimensions.

**Why it happens:**
Retention filtering and the new tag filter are naturally implemented by different people/PRs at different times touching the same handler; the existing regression tests (`TestRetention_DetectionStateQueriesStayUnfiltered` et al.) guard the boundary that already exists, but nothing yet guards the *new* boundary a tag-filtered History query introduces.

**How to avoid:**
- Add the tag filter as another `WHERE` clause/parameter inside the same retention-aware query (or the same `Service.List` composition point), never as a separate code path that reconstructs its own `events` query from scratch.
- Write the analogous regression test to the existing retention suite: seed an aged-out (>retention window) tagged event and a within-window tagged event, filter History by that tag, and assert only the within-window one is visible — mirroring `TestRetention_...`'s existing pattern exactly (`events_test.go` is already the right file and already has the pinned aged-row fixture helpers to reuse).
- If tags are `artist_id`-scoped per Pitfall 5, the tag-filter join is `events.artist_id = artist_tags.artist_id AND artist_tags.tag_id = $N` — composed with the existing retention predicate in one query, not two round trips.

**Warning signs:**
- A tag-filtered History view showing rows the unfiltered view hides (or vice versa) for the same time range, caught only by careful UAT rather than a test.
- Two separate SQL query functions in `queries/events.sql` for "list events" and "list events by tag" that duplicate the retention `WHERE` clause instead of sharing it.

**Phase to address:**
The phase implementing "History feed filterable by tag."

---

### Pitfall 10: A migration adding `NOT NULL` tag/note columns without a default trips `cmd/migration-check`'s unsafe-forward guard — and a same-release tag-column rename/type-narrow trips the backward-incompatible guard

**What goes wrong:**
`cmd/migration-check` (documented in `internal/db/migrations/README.md`) hard-fails CI on two finding classes: `ADD COLUMN ... NOT NULL` with no `DEFAULT` in the same clause (unsafe-forward), and any `DROP`/`RENAME`/type-narrowing `ALTER COLUMN` against an *existing* column in the same release the code stops relying on the old shape (backward-incompatible, cross-referenced against the N-1 release's `queries/*.sql`). A tags/notes migration is mostly pure-additive (new tables — generally safe), but two shapes commonly appear in a real implementation and would trip the guard: (a) a `notes` column added as `TEXT NOT NULL DEFAULT ''` is fine, but a well-intentioned `CHECK (length(notes) <= 500)` constraint added to an *existing* column in a later cleanup migration is a backward-incompatible finding if the N-1 binary's queries still write to that column unconstrained; (b) if the tags feature is prototyped first as a `TEXT[]` column directly on `watchlist` (simpler than a join table) and then migrated to a proper `tags`/`artist_tags` join-table design within the same release cycle, the column-drop half of that migration is a same-release expand+contract violation — the README's own worked example (`000006`/`000007` add+backfill, contract "not yet done") is the pattern to follow: ship the join-table addition and backfill in one release, defer dropping the interim `TEXT[]` column to a later release once the join-table release is no longer N-1.

**Why it happens:**
Tags/notes length caps ("length-capped" per the milestone's own spec) are exactly the kind of constraint that gets added as a `CHECK` on an existing column in a "let's tighten this up" follow-up commit — a natural sequence that happens to collide with the N-1 rollback-safety rule this repo enforces automatically.

**How to avoid:**
- Set any length cap as a `CHECK` constraint in the *same* migration that first creates the column (not retrofitted onto an already-shipped column) — this sidesteps the backward-incompatible classification entirely, since there's no "old, less-constrained shape" a rollback binary could have depended on.
- If a `TEXT[]` tags column is ever considered as a stepping stone before a proper `tags` table (for a fast MVP), treat that as a real design decision requiring the same expand/backfill/contract discipline the README documents, not a "we'll clean it up later in the same release" shortcut — the `cmd/migration-check` cross-reference will catch a same-release drop of a still-queried column regardless of intent, and the annotation escape hatch (`migration-check:allow-destructive`) requires a real, already-shipped `expand-shipped-in` release tag, which won't exist yet for a same-milestone prototype-then-replace sequence.
- Read the checklist in `internal/db/migrations/README.md` before writing the tags/notes migration — it's short and this milestone's migrations are exactly the kind of new-domain-table work it was written for.

**Warning signs:**
- CI's `migration-check` job going red on a tags/notes migration PR with an "unsafe-forward" or "backward-incompatible" finding.
- A local `git log` showing a `TEXT[]` tags column added and then dropped within the same milestone's commit range.

**Phase to address:**
The phase that designs the tags/notes schema — get the join-table shape and constraint placement right in the first migration, since this repo's CI is specifically built to make a wrong second attempt expensive (an N-1 boot failure or a red `migration-check`), not merely inconvenient.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Store tags as a `TEXT[]` column directly on `watchlist` instead of a `tags`/`artist_tags` join table | Faster to ship, no join table, no rename plumbing | Global rename becomes an `UPDATE ... SET tags = array_replace(...)` across every row (no single source of truth for a tag's canonical spelling), autocomplete has no dedicated lookup table to query distinctly, and it collides with Pitfall 5 (tags need to be `artist_id`-scoped, and `watchlist`-column storage makes that harder to retrofit) | Never for this feature set — global rename and cross-artist autocomplete are explicit v1.6 requirements that a plain array column actively works against |
| Skip case-folding on tag identity for v1 ("we'll clean it up if it becomes a problem") | One less migration decision, ships faster | Every existing tag becomes a silent duplicate-fork risk the moment two users (or one user on two sessions) type the same tag with different casing; retrofitting normalization later requires a data migration to merge already-diverged tag rows, which is much more invasive than deciding the rule up front | Only if tags are scoped to a single operator who is warned and disciplined about consistent casing — risky for a "portfolio piece meant to look production-grade" project |
| Implement bulk-add's per-name resolve loop as one blocking backend request "for now, optimize later" | Simplest possible backend code, no new async/polling machinery | Directly collides with the 15s `writeTimeout` (Pitfall 3) — this isn't a performance nicety to defer, it's a correctness bug that manifests on any paste over ~15 names | Never — this must be decided architecturally before the first line of the endpoint is written |
| Leave real-time embed markdown-escaping as-is ("digest already handles it, real-time predates tags, low risk") | Zero extra code in the phase that's already touching a lot of surface area | Ships a known Discord-formatting-injection vector into a *new, explicitly user-facing* feature (tags), on the exact code path the milestone's own spec calls out for Discord display | Never for tags; the pre-existing `ev.Title`/`ev.ArtistName` gap in `format.go` is lower-priority tech debt (MusicBrainz/Deezer text, not free-typed by the app's own user) and can be deferred separately |

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|-------------------|
| MusicBrainz search (paste-a-list bulk add) | Treat the existing `GET /search` endpoint/rate limiter as capable of absorbing N sequential calls inside one HTTP request | Client-orchestrated sequential/bounded-concurrency calls to the existing per-name search endpoint, each request independently under the 15s `writeTimeout`, with visible per-name progress in the UI |
| Discord webhook (tags on embeds/digest lines) | Copy `format.go`'s no-escaping pattern for a new user-authored field | Route tags through the same `escapeMarkdown` used by `digest_format.go`, on both the real-time and digest paths |
| Postgres schema (tags/notes tables) | Add a `CHECK` length constraint onto an already-shipped column in a follow-up migration | Bake the length cap into the same migration that creates the column; treat any later tightening as a full expand/backfill/contract cycle |
| `golang-migrate`-embedded migrations + `cmd/migration-check` | Assume "it's just a new table, additive, no risk" without running the README's pre-merge checklist | Read `internal/db/migrations/README.md` before writing the tags/notes migration; run `cmd/migration-check` locally (`make sqlc-check`-adjacent target, or via CI on a scratch branch) before opening the PR |

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Per-artist N+1 for "latest release" sort | Watchlist page load time grows linearly with watchlist size | Single `LEFT JOIN LATERAL` or `GROUP BY` query, matching this codebase's existing one-round-trip query style | Noticeable past ~30-50 artists; the milestone's own goal names "50+ artist watchlist" as the target scale |
| Tag-filter autocomplete querying all tag-assignment rows client-side | Autocomplete input lag grows with total tag-assignment count, not distinct-tag count | A dedicated `tags` table queried for distinct tag names (with an index), never a client-side filter over every assignment row | Once total tag assignments exceed a few hundred across a 50+ artist watchlist with multiple tags each |
| Bulk-add per-name resolve loop with no client-side batching/backoff visibility | UI appears frozen for 60-100+ seconds on a large paste with no incremental feedback | Bounded-concurrency or strictly sequential client-orchestrated calls with a live progress indicator (Pitfall 3) | Any paste over roughly 15-20 names given the 1 req/sec MusicBrainz limiter |

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Unescaped tag text reaching a Discord embed field or digest line | Markdown injection: a tag can retarget a masked link's URL, forge bold/heading formatting, or otherwise misrepresent a notification's content in a channel other users trust | Route every tag string through `escapeMarkdown` on both notifier paths (Pitfall 1) |
| New bulk-edit/bulk-remove/bulk-add-confirm endpoints registered outside the existing protected chi `Group`, or client code bypassing `apiFetch` | On a gated instance, a mis-registered route either skips authentication entirely (data exposure) or breaks CSRF protection (state-changing request forgeable cross-site) | Register every new write route inside the existing `pr.Use(gate.Authenticate)`/`pr.Use(gate.RequireCSRFHeader)` group; route every new client call through `apiFetch` (Pitfall 4) |
| Tag/note free text rendered into the SPA via anything other than a plain JSX text node | Frontend XSS — this codebase's existing discipline (`CONCERNS.md`: "no `dangerouslySetInnerHTML` anywhere") must extend to two brand-new free-text surfaces | Render tags and notes as plain JSX text exactly like existing event titles/artist names; do not introduce any HTML-rendering path for user-typed content |

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-------------------|
| Stale multi-select state surviving a bulk operation | After a bulk tag-add/remove/mute-set/remove completes, the selection checkboxes still show the just-processed (and, for bulk-remove, now-deleted) rows as selected; a second accidental action (e.g. hitting "bulk remove" again) targets stale/gone ids | Clear the selection set immediately after any bulk action's confirmed success (and on any error that leaves the operation's outcome ambiguous, since a partial-failure state — see below — makes "which rows are still valid to re-select" genuinely unclear); re-derive selection validity from the freshly re-fetched watchlist rather than trusting the pre-action id list |
| No partial-failure contract for bulk edit | If a bulk tag-add spans 20 selected artists and row 14 fails (e.g. a concurrent delete of that watchlist entry), a naive implementation either aborts with no indication of which of the first 13 already succeeded, or silently continues and reports blanket "success" | Mirror this codebase's existing partial-failure precedent (`GET /search`'s D-03 contract: "a source failing never fails the whole request") — return a per-id result list (succeeded/failed/reason) from bulk endpoints, and render it distinctly in the UI rather than one boolean toast |
| Paste-a-list review screen auto-adding on a "good enough" match | A pasted list with ambiguous names (e.g. an artist with several same-named entries disambiguated only by country/genre) silently picks the wrong candidate, and the user doesn't notice until a wrong artist's releases start appearing | The milestone's own spec already requires "nothing added until confirmed" — enforce this at the API layer too (a separate resolve/confirm step, not a single add-with-best-guess call), and default an ambiguous name's review-row selection to "no selection" rather than pre-picking the top search result, so silence never means acceptance |
| Bulk-remove confirmation copy with no mention of the re-add/seed-mode surprise (Pitfall 6) | A user re-adding a removed artist gets an unexplained Discord notification for an old release days or weeks later, with no context connecting it back to the removal | Add a short, factual line to the bulk-remove confirmation and/or the paste-a-list re-add case: releases missed while off the watchlist may notify once re-added |

## "Looks Done But Isn't" Checklist

- [ ] **Tags on Discord:** Often missing markdown escaping on the real-time embed path specifically (digest path already has it) — verify with a tag containing `*_~`` `|>#[]()` renders literally in a real Discord message, not just in a unit test.
- [ ] **Global tag rename:** Often missing a defined behavior for "rename to a name that already exists as another tag" — verify the UI/API explicitly rejects, merges, or otherwise handles this rather than producing a silent constraint-violation 500.
- [ ] **Watchlist sort by latest release:** Often missing retention-window awareness — verify an artist whose only release is older than `EVENT_RETENTION_DAYS` sorts consistently with what History shows for it (not "latest: <aged-out date>" while History shows nothing).
- [ ] **History filter by tag:** Often missing a test proving it composes with (not bypasses) `EVENT_RETENTION_DAYS` — verify with a seeded aged-out tagged event and a within-window tagged event, per Pitfall 9.
- [ ] **Bulk remove:** Often missing an explicit test proving tags survive when scoped to `artist_id` and notes are correctly gone (mirroring `TestService_Remove_LeavesArtistRowIntact`) — verify both halves, not just one.
- [ ] **Paste-a-list bulk add:** Often missing a hard verification that the 15s `writeTimeout` is never hit by the chosen implementation shape — verify with a real 50-100 line paste against a rate-limited (not `rate.Inf`) MusicBrainz client, not just a mocked instant-response test.
- [ ] **Frontend coverage gate (70%):** Often missing coverage on the review-screen's partial-failure and re-selection-after-bulk-op states specifically (the "unhappy path" branches), even when the happy path is well-tested — verify the coverage report, not just that tests exist.

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|----------------|-----------------|
| Tags stored `watchlist_id`-scoped, discovered after ship | HIGH | New migration: `artist_tags` (or equivalent) table keyed on `artist_id`; backfill by joining current `watchlist_id`-scoped rows through `watchlist.artist_id`; ship as its own expand+backfill release per the README's pattern; drop the old table only once that release is no longer N-1 |
| Chunk-count fixture drift discovered via a red CI run after tags land in digest lines | LOW | Re-derive `chunkForcingEventCount`/`capForcingEventCount` empirically against the new `digestLine` output (same whitebox-probe method the existing comment describes), update the two constants and their comments in one PR |
| Unescaped tags shipped to real-time embeds, discovered post-release | MEDIUM | Add `escapeMarkdown` to `format.go`'s tag-rendering call site; no data migration needed (formatting is computed at send time from live tag data, not stored pre-rendered) — but any already-sent malformed Discord messages in a live channel cannot be retroactively fixed, only prevented going forward |
| Bulk-add implemented as one blocking request, timing out in production | MEDIUM | Refactor to client-orchestrated sequential calls against the existing per-name search endpoint; no schema change needed since "nothing added until confirmed" means no partial-add data exists to reconcile |

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|-------------------|----------------|
| 1. Unescaped tags on real-time Discord embeds | Phase: tags on Discord embeds/digest lines | Table-driven test asserting a metacharacter-laden tag renders escaped on both the real-time and digest paths |
| 2. Digest chunker fixture drift | Phase: tags on Discord embeds/digest lines | `TestSendDigestIfDue_MultiChunk*`/`*Cap*` still pass, or fixture constants are explicitly re-derived and documented |
| 3. Bulk-add 15s WriteTimeout collision | Phase: paste-a-list bulk add | Manual UAT with a real 50-100 line paste against a genuinely rate-limited MusicBrainz client (not `rate.Inf`), confirming no request exceeds `writeTimeout` |
| 4. CSRF/session contract on new bulk endpoints | Phase: multi-select bulk edit; Phase: paste-a-list bulk add | New endpoints registered in the existing protected `Group`; new client calls route through `apiFetch`; gated-instance UAT shows no unexpected 403s |
| 5. Tag storage scope (artist_id vs watchlist_id) | Phase: tags data model / free-form multi-tags | Test proving tags survive `Service.Remove` when scoped to `artist_id`, mirroring `TestService_Remove_LeavesArtistRowIntact` |
| 6. Re-add bypassing seed mode | Phase: multi-select bulk edit (confirmed bulk remove) | Documented as an accepted limitation in bulk-remove confirmation copy; no code fix required unless scope changes |
| 7. Tag case-folding/Unicode normalization | Phase: tags data model / free-form multi-tags | Migration uses a case-insensitive uniqueness mechanism (`citext` or `LOWER()` index); NFC-normalize before storing |
| 8. Sort-by-latest-release N+1 | Phase: Watchlist search/sort/filter | `EXPLAIN ANALYZE` on the chosen query shows one round trip regardless of watchlist size; no per-artist Go-side loop in the diff |
| 9. Retention/tag-filter composition on History | Phase: History filter by tag | New test mirroring `TestRetention_...`'s pattern, seeding an aged-out tagged event and a within-window tagged event |
| 10. Migration-check guard on tags/notes DDL | Phase: tags data model / free-form multi-tags | `cmd/migration-check` (CI job `migration-check`) passes green on the first attempt, without needing a `migration-check:allow-destructive` annotation |

## Sources

- `C:\CodeProjects\drop-tracker\internal\notifier\format.go` — real-time embed formatting, confirmed no markdown escaping (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\notifier\digest_format.go`, `digest_chunk.go` — digest markdown escaping, chunk budget math, pinned fixture constants (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\notifier\digest_test.go` — `chunkForcingEventCount`/`capForcingEventCount` fixture definitions and their own "empirically confirmed" comments (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\authgate\gate.go` — CSRF header contract, session renewal, route registration discipline (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\db\migrations\000002_watchlist.up.sql`, `000003_events.up.sql` — `artists`/`watchlist`/`events` FK/cascade design and its documented rationale (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\watchlist\service.go` (`Service.Remove`) plus `service_test.go` (`TestService_Remove_LeavesArtistRowIntact`, `TestService_Remove_ThenReAddSucceeds`) — confirmed hard-delete-watchlist-only-not-artist behavior (confidence: HIGH, direct read + test names)
- `C:\CodeProjects\drop-tracker\internal\detection\detector.go` (`isSeedMode`, `notifyGate`) — confirmed per-`(artist_id, source)` implicit seed-mode definition (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\queries\watchlist.sql`, `queries\events.sql` — existing query shapes (no N+1 precedent, no tag/latest-release query yet) (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\cmd\server\main.go` (`writeTimeout`, `readTimeout` constants), `internal\config\config.go` (`MusicBrainzRateLimitPerSec` default `1`) — confirmed the exact numbers behind the bulk-add timeout collision (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\db\migrations\README.md` — N-1 rollback-safety rules, `cmd/migration-check` enforcement scope, expand/backfill/contract worked example (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\internal\httpserver\events.go`, `events_test.go` — retention filtering scope and its existing regression-test discipline (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\.planning\codebase\CONCERNS.md` — existing known tech debt/fragile areas informing which patterns to reuse vs. avoid (confidence: HIGH, direct read)
- `C:\CodeProjects\drop-tracker\.planning\PROJECT.md` — v1.6 milestone scope, target features, "no new background API polling" constraint (confidence: HIGH, direct read)

---
*Pitfalls research for: drop-tracker v1.6 Watchlist Organization*
*Researched: 2026-09-22*
