---
phase: "25"
slug: "find-filter-watchlist-and-history"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-12"
---

# Phase 25 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Postgres events → GET /watchlist JSON | Detected release dates (from MusicBrainz/Deezer) in new response fields, behind the existing auth gate | Catalogue dates, low |
| Browser → GET /events?tag_id | Untrusted query param reaches a SQL predicate | Integer id, low |
| GET /watchlist, /tags, /events, /search → DOM | Release dates, user-authored tag names, `created_at`, third-party catalogue strings rendered in cards, chips, options | Display text, low |
| URL query string → Watchlist/History view state | `q`, `sort`, `tags`, `muted`, `custom`, `artist`, `type`, `tag` are attacker-craftable via a shared link | Untrusted strings, low |
| Browser → history API | Keystrokes can become navigations | UI state, low |
| Card chip click → filter vs server state | Adjacent filter-toggle and detach buttons | Mutation intent, medium |
| Package registry → `pnpm install --frozen-lockfile` | Build-time dependency resolution | Supply chain, high |
| Built bundle → go:embed → served SPA | The committed tree is what the binary serves | Shipped code, medium |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-25-01 | DoS | ListWatchlist laterals | medium | mitigate | Two `LIMIT 1` laterals in one statement (`queries/watchlist.sql:39-93`) | closed |
| T-25-02 | DoS | NULL scan | medium | mitigate | Plain-column laterals; null-not-500 HTTP test (`internal/httpserver` watchlist test ~L2023) | closed |
| T-25-03 | Info Disclosure | latest/next release fields | low | accept | See Accepted Risks AR-25-01 | closed |
| T-25-04 | Tampering | SQL construction | low | accept | See Accepted Risks AR-25-02 | closed |
| T-25-05 | Tampering | `tag_id` → ListEvents/HasOlderEvents | high | mitigate | `parseOptionalPositiveInt64` + `invalid tag_id` (`internal/httpserver/events.go:87-89`); bound `sqlc.narg('tag_id')::bigint` (`queries/events.sql:151,188`) | closed |
| T-25-06 | Info Disclosure | Retention bypass via tag filter | medium | mitigate | Predicate inside retention statements; `TestRetention_TagFilterComposesWithCutoff` (`internal/httpserver/events_test.go:1519`) | closed |
| T-25-07 | Info Disclosure | Error responses | low | mitigate | Fixed messages (`invalid tag_id`, `internal error`) | closed |
| T-25-08 | DoS | EXISTS over artist_tags | low | accept | See Accepted Risks AR-25-03 | closed |
| T-25-09 | Tampering (XSS) | WatchlistRow release lines | medium | mitigate | Plain JSX text; literal-`<b>` route test (`web/app/routes/watchlist.test.tsx`); no `dangerouslySetInnerHTML`/`innerHTML` in `web/app` | closed |
| T-25-10 | Tampering | Date display | low | mitigate | `formatReleaseDate` renders stored precision, no Date/timezone math | closed |
| T-25-11 | Tampering (XSS) | `q` param | medium | mitigate | Controlled input value + JSX text only | closed |
| T-25-12 | DoS | History flooding | low | mitigate | `Q_WRITE_DEBOUNCE_MS = 300` with replace (`web/app/routes/watchlist.tsx:59`) | closed |
| T-25-13 | DoS | Per-keystroke API traffic | low | mitigate | In-memory filter; test asserts `listTags`/`listEvents` not called (`watchlist.test.tsx:1067-1068`) | closed |
| T-25-14 | Repudiation | Invisible add under filter | low | mitigate | Sticky set keyed by POST response id; add + Undo route tests | closed |
| T-25-15 | Tampering | `sort` param | low | mitigate | `isSortKey` allow-list → `DEFAULT_SORT` (`web/app/lib/watchlistView.ts:44,81`) | closed |
| T-25-16 | DoS | Client sort cost | low | accept | See Accepted Risks AR-25-04 | closed |
| T-25-17 | Tampering (XSS) | Tag names in History combobox | medium | mitigate | Plain JSX text / React-escaped `title` | closed |
| T-25-18 | Tampering | `tagId` sent to API | low | mitigate | Ids only from GET /tags; server re-validates (T-25-05) | closed |
| T-25-19 | DoS | Failed GET /tags | low | mitigate | `.catch` → `status: "error"`, no retry (`web/app/routes/history.tsx:173-185`) | closed |
| T-25-20 | DoS | Oversized `tags=` | low | mitigate | `MAX_TAG_TOKENS = 200`, `^[1-9]\d*$` (`watchlistView.ts:63-70`) | closed |
| T-25-21 | Tampering (XSS) | Tag names in toolbar chips | medium | mitigate | JSX text / escaped attributes; no HTML-injection API in `web/app` | closed |
| T-25-22 | Tampering | Filter ↔ server state | low | mitigate | Filter never calls attach/detach/delete | closed |
| T-25-23 | Tampering | Chip label vs × | medium | mitigate | Label calls only `toggleFilterTag` (`TagChips.tsx:200`); tests assert attach/detach not called (`TagChips.test.tsx:554-560`) | closed |
| T-25-24 | Tampering | `muted`/`custom` params | low | mitigate | Only `"1"` honored (`watchlistView.ts:83-84`) | closed |
| T-25-25 | Tampering (XSS) | Chip `aria-label`/`title` | low | mitigate | React-escaped attributes | closed |
| T-25-26 | Tampering | History `artist`/`tag` params | low | mitigate | `ID_PATTERN = /^[1-9]\d*$/` (`web/app/lib/historyParams.ts:10`); server re-validates | closed |
| T-25-27 | Tampering | History `type` param | low | mitigate | Three-value allow-list (`historyParams.ts:5-7`); server `watchlist.EventTypes` (`internal/watchlist/service.go:49`) | closed |
| T-25-28 | Tampering (XSS) | Param values in DOM | low | mitigate | Params parsed to ids/tokens, never rendered | closed |
| T-25-SC | Tampering | npm installs during rebuilds | high | mitigate | No Phase 25 commit touches `web/package.json` or `web/pnpm-lock.yaml`; empty diff vs `461267f`, `8ccde67`, `9d3413f`. Lockfile diff vs `9030bfa` is Phase 24's `7eaa989` (security overrides, patch bumps), merged into the branch base before `627b159` | closed |
| T-25-29 | Tampering | Stale embedded bundle | medium | mitigate | Dedicated rebuild task with marker checks (25-10) | closed |
| T-25-30 | Elevation | Out-of-scope packages | low | mitigate | Empty diff over notifier/detection/discord/musicbrainz/deezer/poller/migrations since `9030bfa` | closed |
| T-25-31 | DoS (UI) | useScrollAnchor effect | low | mitigate | Anchor cleared before measuring; early returns | closed |
| T-25-32 | Tampering | Router scroll contract | low | mitigate | `preventScrollReset: true` on every write (`web/app/lib/useUrlParams.ts:5`); file only touched by 25-04 | closed |
| T-25-33 | Tampering (XSS) | EventCard Detected line | low | mitigate | `formatDetectedDate` → `<time>` JSX text (`EventCard.tsx:54,89`) | closed |
| T-25-34 | Info Disclosure | Retention semantics | low | mitigate | No query change; regression test | closed |
| T-25-35 | DoS (UI) | Malformed `created_at` | low | mitigate | `formatDetectedDate` returns `null` (`web/app/lib/format.ts:179`) | closed |
| T-25-36 | Tampering (XSS) | SearchResultRow spans | low | accept | See Accepted Risks AR-25-05 | closed |
| T-25-37 | Repudiation | Disabled Deezer button | low | mitigate | Button stays disabled; Deezer id never posted as mbid | closed |
| T-25-38 | Tampering | Stale embedded bundle | medium | mitigate | Marker-count rebuild check (25-14) | closed |
| T-25-39 | Elevation | Retention SQL / vendored ui | low | mitigate | Blast-radius diff (25-14) | closed |
| T-25-40 | Repudiation | UAT result tampering | low | mitigate | Append-only UAT/decision check (25-14) | closed |
| T-25-41 | DoS (UI) | Render-time `setSticky` reconcile | low | mitigate | `reconcileSticky` returns `prev` once converged; Tests A-E, I | closed |
| T-25-42 | Tampering | Sticky carry | low | mitigate | Sticky ids only from in-card edits/adds, never URL; unit tests F-H | closed |
| T-25-43 | Tampering | Router scroll contract | low | mitigate | `useUrlParams.ts` unchanged after 25-04 | closed |
| T-25-44 | Info Disclosure | Wrapper `title` | low | mitigate | Static `DEEZER_ADD_HINT` (`SearchResultsColumns.tsx:134,199`) | closed |
| T-25-45 | Tampering | Shared `ui/button.tsx` | low | mitigate | `web/app/components/ui` unchanged since `9d3413f` | closed |
| T-25-46 | DoS (UI) | useScrollAnchor candidates | low | mitigate | ≤3 candidates, ref cleared first, early return | closed |
| T-25-47 | Tampering | Scroll behaviour | low | mitigate | Only `window.scrollBy` in hooks/lib (`web/app/lib/useScrollAnchor.ts:56`) | closed |
| T-25-48 | Tampering | Stale embedded bundle | medium | mitigate | Marker-count rebuild check (25-18) | closed |
| T-25-49 | Elevation | Go/SQL/vendored/verification docs | low | mitigate | Blast-radius diff (25-18) | closed |
| T-25-50 | Repudiation | Spec/decision drift | low | mitigate | Exact UI-SPEC/CONTEXT diff check (25-18) | closed |
| T-25-51 | DoS | `anchorCandidates` sibling measuring | low | accept | See Accepted Risks AR-25-06 | closed |
| T-25-52 | Tampering | Route/chip/URL helper scope | low | mitigate | Unchanged-file verify (25-19); only `window.scrollBy` used | closed |
| T-25-53 | Tampering | `focusAfterCardLeaves` selector | low | mitigate | Constant `'[data-chip-label][aria-pressed="true"]'` (`watchlist.tsx:200`) | closed |
| T-25-54 | DoS (usability) | Focus moved against intent | low | mitigate | Keyboard-only (`detail === 0`) gate; hook + route tests | closed |
| T-25-55 | Tampering | URL helper, ui/, packages | low | mitigate | Empty diff since `9d3413f` | closed |
| T-25-56 | Tampering (XSS) | Deezer hint span | low | mitigate | Static constant child (`SearchResultsColumns.tsx:212`); id from `useId` (L161) | closed |
| T-25-57 | Tampering | Stale embedded bundle | medium | mitigate | Marker-count rebuild check (25-21) | closed |
| T-25-58 | Elevation | Go/SQL/vendored/decision docs | low | mitigate | Empty diff vs `9d3413f` over `*.go`, queries, internal/db, internal/events, ui/, useUrlParams.ts, root.tsx | closed |
| T-25-59 | Repudiation | Spec drift | low | mitigate | Cumulative UI-SPEC diff check (25-21) | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-25-01 | T-25-03 | Same rows History serves behind the same gate; G1 accepts Watchlist may show a date older than History's retention window | plan 25-01 | 2026-10-12 |
| AR-25-02 | T-25-04 | No new parameters reach the queries; sqlc bound statements only | plan 25-01 | 2026-10-12 |
| AR-25-03 | T-25-08 | Semi-join on `artist_tags_tag_id_idx`; page size clamped by `events.MaxPageSize` | plan 25-02 | 2026-10-12 |
| AR-25-04 | T-25-16 | In-memory sort of ~100 rows on change only; no network | plan 25-05 | 2026-10-12 |
| AR-25-05 | T-25-36 | Class/markup change only; still plain JSX text, no new data source | plan 25-13 | 2026-10-12 |
| AR-25-06 | T-25-51 | Back-to-back rect reads, one layout; bounded by the user's own watchlist; runs only on chip-label click | plan 25-19 | 2026-10-12 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-12 | 60 | 60 | 0 | /gsd-secure-phase (orchestrator, L1 grep verification; auditor skipped per short-circuit) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-12
