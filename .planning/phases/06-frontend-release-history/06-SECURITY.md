---
phase: 06
slug: frontend-release-history
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-11
---

# Phase 06 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| browser → Go API | Untrusted query strings, PATCH bodies, DELETE ids and search queries cross into `internal/httpserver` | query params, JSON bodies |
| embedded FS → browser | Request paths select files out of the embedded SPA build | static file bytes |
| npm registry → build host | Third-party package code is fetched and executed at build time | npm packages |
| Postgres → Go → browser | Externally-sourced text (MusicBrainz/Deezer titles, artist names) stored in earlier phases flows out to the DOM | artist/release text |
| external image host → browser | `cover_art_url` / `image_url` values are loaded as image sources | image bytes |
| Vite dev server → Go API | Local development only, SPA and API on different ports | proxied HTTP |
| MusicBrainz / Deezer → Go → browser DOM | Live third-party search results rendered without being persisted first | search result text |
| browser → third-party APIs (indirect) | Every search keystroke can drive outbound requests to two rate-limited external services | search queries |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-06-01 | Information Disclosure | `handleListEvents` error path | medium | mitigate | Raw error logged via `httplog.SetAttrs`; fixed `"internal error"` returned to client (`internal/httpserver/events.go:107-108`) | closed |
| T-06-02 | Denial of Service | `GET /events` result-set size | high | mitigate | `events.Service.List` clamps to `MaxPageSize` (100) regardless of caller input (`internal/events/service.go:19-101`) | closed |
| T-06-03 | Information Disclosure | `webassets.Handler` static file serving | medium | mitigate | Served exclusively from `fs.Sub` of an `embed.FS` — no parent directory to traverse into (`internal/webassets/embed.go:31`) | closed |
| T-06-04 | Tampering | SQL injection through events filters | high | mitigate | Single static sqlc-generated parameterized query; no string-assembled `WHERE` clauses (`internal/db/sqlc/events.sql.go`) | closed |
| T-06-05 | Spoofing | `r.NotFound` shadowing a real API route | medium | mitigate | chi matches explicitly registered routes before `NotFound`; asserted in `spa_test.go` | closed |
| T-06-SC | Tampering | pnpm/npm package installs (16 packages) | high | mitigate | `make web` uses `--frozen-lockfile`; committed lockfile prevents silent version drift (`Makefile`) | closed |
| T-06-06 | Denial of Service | `limit` query param on `GET /events` | high | mitigate | Clamped by `events.Service.List` at `MaxPageSize`; asserted by `TestHandleListEvents_Validation` (`internal/httpserver/events_test.go:331-332`) | closed |
| T-06-07 | Tampering | `event_type` query param | medium | mitigate | Membership-checked against `watchlist.EventTypes` via `slices.Contains` (`internal/httpserver/events.go:81`) | closed |
| T-06-08 | Tampering | `artist_id` / `cursor` query params | medium | mitigate | `strconv.ParseInt` with reject-below-1 rule before store call (`internal/httpserver/events.go:38`) | closed |
| T-06-09 | Tampering | Stored XSS via event `title` / `artist_name` in cards | high | mitigate | Plain JSX text nodes only; no `dangerouslySetInnerHTML` anywhere under `web/app/` (repo-wide grep, 0 matches) | closed |
| T-06-10 | Information Disclosure | Validation error messages | medium | mitigate | Every 400 message is a fixed operator-authored string; rejected value never echoed | closed |
| T-06-11 | Spoofing | Untrusted `cover_art_url` loaded as image source | low | accept | Browsers do not execute image sources as script; null-art placeholder fallback on failure (Phase 2 T-02-18) | accepted |
| T-06-12 | Tampering | Stored XSS via artist `name` / `disambiguation` in watchlist rows | high | mitigate | Plain JSX text nodes only; no `dangerouslySetInnerHTML` anywhere under `web/app/` (repo-wide grep, 0 matches) | closed |
| T-06-13 | Repudiation | Optimistic preference toggle after failed PATCH | medium | mitigate | Pre-toggle state restored on non-OK response with failure toast; verified live via UAT Test 1 (forced PATCH failure, checkbox reverted, toast shown) — `web/app/routes/watchlist.tsx`, `web/app/components/watchlist/PreferenceToggles.tsx` | closed |
| T-06-14 | Tampering | Preference values submitted from browser | medium | mitigate | `normalizeSet` re-validates server-side against `ReleaseTypes`/`EventTypes` allow-lists independent of client (`internal/watchlist/service.go:314`) | closed |
| T-06-15 | Information Disclosure | Dev-mode CORS configuration leaking into production | medium | mitigate | Cross-origin dev traffic handled entirely by Vite dev-server proxy; no CORS middleware exists anywhere in `internal/` (repo-wide grep, 0 matches) | closed |
| T-06-16 | Spoofing | Untrusted `image_url` loaded as image source | low | accept | Browsers do not execute image sources as script; null-art placeholder fallback (Phase 2 T-02-18) | accepted |
| T-06-17 | Denial of Service | Rapid repeated preference toggling | low | accept | Each toggle is one small PATCH against a local single-operator service; control disabled between click and response | accepted |
| T-06-18 | Tampering | XSS via search-result `name` / `disambiguation` from MusicBrainz/Deezer | high | mitigate | Plain JSX text nodes only; no `dangerouslySetInnerHTML` anywhere under `web/app/` (repo-wide grep, 0 matches) | closed |
| T-06-19 | Denial of Service | Search-as-you-type fanning every keystroke to two rate-limited third-party APIs | medium | mitigate | ~300ms debounce + `AbortController` cancellation (`web/app/components/watchlist/SearchBox.tsx`); server-side per-source rate limiter and `searchResultLimit` of 10 | closed |
| T-06-20 | Information Disclosure | Upstream failure detail reaching operator's screen | medium | mitigate | Fixed `"source unavailable"` string replaces raw upstream error text (`internal/httpserver/search.go:186`) | closed |
| T-06-21 | Spoofing | Duplicate add slipping past client-side "Already watching" check | low | accept | `POST /watchlist`'s 409 (Phase 2 D-09) remains the authoritative boundary; client check is UX affordance only | accepted |
| T-06-22 | Tampering | Over-posting server-owned fields on add body | low | mitigate | `addWatchlistRequest` omits server-owned fields; `DisallowUnknownFields` rejects extras (`internal/httpserver/watchlist.go:86`) | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| R-06-01 | T-06-11 | Untrusted `cover_art_url` as image src — browsers don't execute image sources as script; worst case is a broken/wrong image, already handled by the null-art placeholder fallback | Phase 2 (T-02-18), carried forward | 2026-08-11 |
| R-06-02 | T-06-16 | Untrusted `image_url` as image src — same rationale as R-06-01 | Phase 2 (T-02-18), carried forward | 2026-08-11 |
| R-06-03 | T-06-17 | Rapid repeated preference toggling — bounded to one in-flight PATCH per toggle by the disabled-during-request control; local single-operator service, no meaningful DoS surface | gap-closure plan (06-04) | 2026-08-11 |
| R-06-04 | T-06-21 | Client-side duplicate-add check is a UX affordance; server's existing 409 (Phase 2 D-09) remains the authoritative boundary and is handled as success, not an error | Phase 2 (D-09), carried forward | 2026-08-11 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-11 | 23 | 19 closed + 4 accepted | 0 | gsd-secure-phase (L1 grep-depth, orchestrator) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-11
