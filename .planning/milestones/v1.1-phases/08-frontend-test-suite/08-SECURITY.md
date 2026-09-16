---
phase: 08
slug: frontend-test-suite
status: verified
threats_open: 0
asvs_level: 1
created: 2026-08-13
---

# Phase 08 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| npm registry -> `web/node_modules` | Five new third-party packages (vitest, jsdom, @testing-library/*) enter the local and CI test toolchain | package code |
| GitHub Actions marketplace -> CI runner | `pnpm/action-setup`, `actions/setup-node` execute with repository-scoped permissions on every push | action code, `contents: read` |
| Test process -> public internet | A test that fails to mock the API boundary would reach the real Go API and, transitively, MusicBrainz/Deezer, from every CI run | live network calls |
| Go API (`/watchlist`, `/events`, `/search`) -> React client | Artist names, event titles, and `event_type`/`external_id` values originate from MusicBrainz/Deezer and cross into the client with no client-side schema validation | third-party-sourced strings rendered into DOM, `aria-label`, and anchor `href` |

---

## Threat Register

| Threat ID | Plan | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|------|----------|-----------|----------|-------------|------------|--------|
| T-08-SC | 08-01 | Tampering | pnpm installs in `web/` (vitest, jsdom, @testing-library/*) | high | mitigate | Package Legitimacy Audit gated the three `[SUS]` candidates behind a blocking human checkpoint before install; versions exact-pinned; `web/pnpm-lock.yaml` committed (`--frozen-lockfile` in CI). Verified: lockfile present in repo, `web/package.json` pins exact versions. | closed |
| T-08-SC2 | 08-01 | Tampering | `pnpm/action-setup`, `actions/setup-node` in `full-pipeline.yml` | high | mitigate | Both pinned to full 40-char commit SHAs, re-verified against claimed tags before landing. Verified: `full-pipeline.yml` pins `pnpm/action-setup@ff378ebe6b225b0680b81c1ad4498ae0d1d3a5e3` and `actions/setup-node@820762786026740c76f36085b0efc47a31fe5020`. | closed |
| T-08-03 | 08-01 | Tampering | Test suite reaching live third-party APIs | low | mitigate | Every test file mocks `~/lib/api` bare (`vi.mock("~/lib/api")`, no passthrough factory). Verified: 4 test files all use the bare mock; zero `importActual`/`global(This).fetch` matches. | closed |
| T-08-04 | 08-01 | Information Disclosure | `frontend-test` CI job logs | low | accept | Job consumes no secrets, checks out from a committed lockfile, requests no `permissions:` beyond workflow-level `contents: read`. | closed |
| T-08-05 | 08-02 | Tampering | Artist name rendered into `WatchlistRow`'s `aria-label`/text | low | accept | Closed project-wide in Phase 6 — all API-sourced text renders as plain JSX text nodes; repo-wide `dangerouslySetInnerHTML` grep returns zero matches. This plan adds tests only, no new rendering surface. | closed |
| T-08-06 | 08-02 | Repudiation | Watchlist-remove test asserting call count only, not arguments | low | mitigate | Test asserts the specific entry id, not just that the mock was called. Verified: `expect(mockRemoveWatchlist).toHaveBeenCalledWith(42)` in `watchlist.test.tsx`. | closed |
| T-08-07 | 08-03 | Denial of Service | `SearchBox.runSearch` -> `/search` -> MusicBrainz/Deezer (uncancelled superseded search burns third-party rate-limit budget) | medium | mitigate | Per-search `AbortSignal` threaded into `searchArtists`, cancelling a superseded search at the request level. Verified: `searchArtists(query, signal?)` forwards `signal` into `apiFetch`'s init object in `web/app/lib/api.ts`. | closed |
| T-08-08 | 08-03/04/05 | Repudiation | RED-then-GREEN commit pair collapsed into one commit, hiding whether the test alone actually fails first | low | mitigate | Each folded-bug fix committed as a RED commit (test-file-only, suite red) then a GREEN commit (source-file-only, suite green). Verified via `git show --stat`: 43e1b40/14003dd, 1bf8cec/4f51937, df1344f/daee355 each isolate test-only vs. source-only changes. | closed |
| T-08-02 | 08-04 | Denial of Service | `EVENT_BADGE` lookup in `EventCard.tsx` — an out-of-union `event_type` throws at render, and the top-level error boundary blanks the whole History route | medium | mitigate | `UNKNOWN_EVENT_BADGE` fallback (`??`) on the lookup; an unrecognized `event_type` now renders a neutral badge instead of throwing. Verified: `EVENT_BADGE[event.event_type] ?? UNKNOWN_EVENT_BADGE` in `EventCard.tsx`; regression test in `EventCard.test.tsx` and green in CI (run 31663467596). | closed |
| T-08-09 | 08-04 | Tampering | Event `title`/`artist_name` rendered into the card | low | accept | Closed project-wide in Phase 6 (see T-08-05); this plan adds no new rendering surface. | closed |
| T-08-10 | 08-05 | Tampering | `guestFeatureHref` in `EventCard.tsx` — unescaped `external_id` interpolated into a URL path | low | mitigate | `encodeURIComponent(event.external_id)` at both interpolation sites (MusicBrainz, Deezer). Verified: exactly 2 real call sites in `EventCard.tsx` (a third grep hit is an explanatory comment, not code); green in CI. | closed |
| T-08-11 | 08-05 | Denial of Service | Over-broad fix escaping the whole URL template instead of just the id, breaking every guest-feature link | low | mitigate | Only `external_id` is wrapped, not the literal scheme/host/path segments. Verified: an ordinary UUID-shaped id is asserted unchanged (guard test) in addition to the two encoding-regression tests. | closed |
| T-08-12 | 08-05 | Elevation of Privilege | Anchor `href` scheme (e.g. a `javascript:`/`data:` URL) | low | accept | Scheme, host, and leading path segment are literals in the template — the untrusted value can only land in a later path segment. Anchor also carries `rel="noreferrer"` + `target="_blank"`. | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|--------------|------|
| R-08-01 | T-08-04 | `frontend-test` CI job carries no secrets and requests no elevated permissions — logs cannot leak credentials | Phase 08 plan (08-01) | 2026-08-12 |
| R-08-02 | T-08-05, T-08-09 | Third-party text-injection into the DOM already closed project-wide in Phase 6 (zero `dangerouslySetInnerHTML` in `web/app/`); this phase adds no new rendering surface | Phase 08 plan (08-02, 08-04) | 2026-08-12 |
| R-08-03 | T-08-12 | Anchor scheme/host are template literals, not attacker-controlled; `rel="noreferrer"` + `target="_blank"` already in place | Phase 08 plan (08-05) | 2026-08-12 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-13 | 13 | 13 | 0 | /gsd-secure-phase (L1 grep-depth, short-circuit — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-13
