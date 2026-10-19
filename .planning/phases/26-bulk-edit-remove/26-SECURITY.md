---
phase: "26"
slug: "bulk-edit-remove"
status: verified
threats_open: 0
asvs_level: 1
created: "2026-10-19"
---

# Phase 26 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser → bulk routes (`/watchlist/bulk/*`) | Untrusted ids, tag name/id, per-axis preference deltas behind session cookie + CSRF header | Watchlist ids, tag names |
| httpserver → services → Postgres | Validated, deduped ids into parameterized single statements / one transaction | Watchlist, tag, preference rows |
| Server data → DOM | Artist and tag names in dialogs, toasts, combobox | Display strings |
| Route state → assistive tech / document root | Count-only announcements; one CSS custom property | Counts, pixel value |
| Package registry / shadcn → bundle → `go:embed` | Build-time deps and the committed bundle | Source/bundle |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-26-01 | Spoofing / Elevation | bulk delete route | high | mitigate | `registerDataRoutes` + `gate.Authenticate` (server.go:297); `TestBulk_Gated401NoCookie` | closed |
| T-26-02 | CSRF | bulk delete | high | mitigate | `RequireCSRFHeader` (server.go:247); 403 test | closed |
| T-26-03 | DoS | id array / body size | medium | mitigate | `maxBulkIDs=1000`, 64 KiB `MaxBytesReader` (bulk.go:18-19,54,109) | closed |
| T-26-04 | Tampering | malformed / over-posted ids | medium | mitigate | `decodeJSONBody` strict, ids ≥ 1, dedupe (bulk.go:57-68) | closed |
| T-26-05 | Info Disclosure | error bodies | low | mitigate | Fixed messages; raw error in log only; no-leak test | closed |
| T-26-06 | Integrity | partial delete | medium | mitigate | Single `DELETE … ANY … RETURNING` (bulk.sql:1-5); no FK to `watchlist` | closed |
| T-26-07 | DoS (self) | overlapping deletes | low | accept | See Accepted Risks Log | closed |
| T-26-08 | Tampering | 10-tag cap bypass | high | mitigate | Ordered `FOR NO KEY UPDATE`, count-guarded insert, trigger backstop | closed |
| T-26-09 | DoS | bulk-add deadlock | medium | mitigate | `ORDER BY a.id` lock order; concurrency test | closed |
| T-26-10 | Integrity | orphan tag rows | low | mitigate | No commit when `Applied` empty; test | closed |
| T-26-11 | CSRF / Spoofing | bulk add-tag route | high | mitigate | Gated group; 401/403 tests | closed |
| T-26-12 | Tampering | tag name input | medium | mitigate | `NormalizeName`, DB CHECKs, parameterized sqlc | closed |
| T-26-13 | Info Disclosure | names in cap skips | low | accept | See Accepted Risks Log | closed |
| T-26-14 | DoS | id/body size, lock hold | medium | mitigate | `validateBulkIDs`, 64 KiB cap, one short tx | closed |
| T-26-15 | XSS | names in confirm/toasts | medium | mitigate | JSX text only; no HTML sinks in `web/app` | closed |
| T-26-16 | CSRF | SPA bulk delete | high | mitigate | `apiFetch` adds `X-Requested-With`; api.test.ts | closed |
| T-26-17 | Safety | acting on hidden artists | medium | mitigate | `effectiveSelection(visible, …)` snapshot; route test | closed |
| T-26-18 | Integrity | accidental mass removal | high | mitigate | Count-stating destructive confirm; Cancel first; ghost Remove | closed |
| T-26-19 | Tampering | preference over-posting | medium | mitigate | Dedicated DTO + `DisallowUnknownFields` | closed |
| T-26-20 | Tampering | unknown preference types | medium | mitigate | `normalizeSet` allow-list + CHECK constraints | closed |
| T-26-21 | Integrity | nil slice wipes types | high | mitigate | `COALESCE` on arrays; non-nil `normalizeSet`; tests | closed |
| T-26-22 | Integrity | lost update | medium | mitigate | `MATERIALIZED … FOR UPDATE` CTE; concurrency test | closed |
| T-26-23 | CSRF / Spoofing | bulk preferences route | high | mitigate | Gated group; `TestBulk_PreferencesGated` | closed |
| T-26-24 | DoS | id/body size, lock hold | medium | mitigate | `validateBulkIDs`, body cap, single statement | closed |
| T-26-25 | DoS (usability) | focus loss / hidden by bar | low | mitigate | pendingFocus, finalFocus, `scroll-padding-bottom` | closed |
| T-26-26 | Info Disclosure | status announcements | low | accept | See Accepted Risks Log | closed |
| T-26-27 | Tampering | `--bulk-bar-clearance` | low | accept | See Accepted Risks Log | closed |
| T-26-28 | CSRF / Spoofing | bulk remove-tag route | high | mitigate | Gated group; `TestBulkTags_RemoveGated` | closed |
| T-26-29 | Tampering | tag id input | low | mitigate | `tag_id ≥ 1`; `GetTagByID` 404 before delete | closed |
| T-26-30 | Integrity | removing tag from unselected | medium | mitigate | DELETE scoped to selected entries' artists; e2e test | closed |
| T-26-31 | DoS | id/body size | medium | mitigate | `validateBulkIDs` + 64 KiB cap | closed |
| T-26-32 | XSS | tag/artist names in toasts | medium | mitigate | JSX text / string toasts only | closed |
| T-26-33 | CSRF | SPA bulk add-tag | high | mitigate | `apiFetch`; api.test.ts | closed |
| T-26-34 | Safety | tagging hidden artists | medium | mitigate | Effective-selection ids; order test | closed |
| T-26-35 | Tampering | over-long tag names | low | mitigate | UI limit; server `NormalizeName` authoritative | closed |
| T-26-36 | CSRF | SPA bulk preferences | high | mitigate | `apiFetch`; api.test.ts | closed |
| T-26-37 | Integrity | overwriting untouched types/notes | high | mitigate | `buildAxisDelta` changed-only; exact-body tests | closed |
| T-26-38 | Safety | hidden artists in preferences | medium | mitigate | Effective-selection snapshot | closed |
| T-26-39 | XSS | tag names in options/toasts | medium | mitigate | JSX text / string toasts only | closed |
| T-26-40 | CSRF | remove-tag and Undo calls | high | mitigate | Both via `apiFetch` | closed |
| T-26-41 | Integrity | Undo on never-tagged artists | medium | mitigate | Undo sends only `out.applied`; test | closed |
| T-26-42 | DoS (usability) | stale tag id | low | mitigate | 404 → toast + `refresh()`, selection kept | closed |
| T-26-43 | Tampering | stale embedded bundle | medium | mitigate | Bundle rebuilt at e1aa421 and carries phase strings, but `f3105f1` (Shift fix) changed `WatchlistRow.tsx` without a rebuild; committed chunk still has the old handler | closed (bundle rebuilt after f3105f1) |
| T-26-44 | Scope creep | out-of-scope packages/files | low | mitigate | Blast-radius diff vs `f793ffc` empty | closed |
| T-26-45 | Safety | Shift range reaching hidden ids | low | mitigate | `applyRange` over `visible` ids only; anchor resets; `f3105f1` does not weaken it | closed |
| T-26-46 | XSS | combobox label strings | medium | mitigate | `aria-label` / JSX text only | closed |
| T-26-47 | Integrity (usability) | Esc clears selection | low | mitigate | Escape guard on `mode === "idle"`; test | closed |
| T-26-SC | Tampering | shadcn vendoring | medium | mitigate | `registries: {}`; package.json/lockfile/components.json unchanged | closed |
| T-26-SC2 | Tampering | npm installs in `make web` | high | mitigate | `--frozen-lockfile` (Makefile, Dockerfile, CI); lockfile unchanged | closed |

*Status: open · closed*
*Severity: critical > high > medium > low — only open threats at or above `high` count toward threats_open*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-26-01 | T-26-07 | Single-user app, single statement; a rare 40P01 is a retryable 500 | Plan 26-01 (RESEARCH A2) | 2026-10-19 |
| AR-26-02 | T-26-13 | Single-tenant owner; same names already served by GET /watchlist behind the same gate | Plan 26-02 | 2026-10-19 |
| AR-26-03 | T-26-26 | Announcements carry counts only, never names; region local to gated page | Plan 26-06 | 2026-10-19 |
| AR-26-04 | T-26-27 | Value derived only from the bar's measured box; removed on unmount | Plan 26-06 | 2026-10-19 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-19 | 49 | 49 | 0 | gsd-security-auditor |

**T-26-43:** closed after the bundle was rebuilt with `f3105f1` (new chunk carries the modifier-key set; old handler absent).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed (no open threat at or above `high`)
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-19 
