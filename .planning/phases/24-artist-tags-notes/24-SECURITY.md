---
phase: "24"
slug: "artist-tags-notes"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-10-05"
---

# Phase 24 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Browser/API client -> POST/DELETE /watchlist/{id}/tags | Untrusted tag names and ids; operator session + CSRF header are the admission control | user-authored strings, low sensitivity |
| Browser -> GET/PATCH/DELETE /tags, POST /tags/{id}/merge | Untrusted ids and names; session + CSRF header gate every call | user-authored strings, low sensitivity |
| Browser -> PUT /watchlist/{id}/note, POST /watchlist | Untrusted free text (≤500 runes) stored verbatim | operator notes, low sensitivity |
| Go service -> Postgres | Normalized names/ids become bound SQL parameters | ids, names |
| Any SQL writer (API or raw psql) -> artist_tags | The 10-tag cap must hold for writers that skip the API | link rows |
| Service -> Postgres transaction | Merge rewrites many rows; lock ordering decides cap/deadlock safety | link rows |
| API JSON / Postgres -> React render | Tag names and notes render in the SPA | user-authored strings (XSS surface) |

---

## Threat Register

Built from the `<threat_model>` blocks of plans 24-01…24-14; `T-24-SC` (repeated per plan) is collapsed to one row. T-24-23 is transferred to the render layer and closed by T-24-24/27/34/37 (no `dangerouslySetInnerHTML` in `web/app`).

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-24-01 | Tampering | `artist_tags` cap | high | mitigate | `artist_tags_cap_trigger` enforces 10 links per artist in the database; `TestSchema_TagCapTrigger_RawInsertRefused` proves a raw INSERT is refused with no API code involved. | closed |
| T-24-02 | Tampering | concurrent attaches | high | mitigate | The trigger locks the artist row `FOR NO KEY UPDATE` before counting and re-checks existence after the lock; `TestTags_Attach_ConcurrentCapRace` (looped) and `TestSchema_TagCapTrigger_SameTagConcurrentAt9` prove both races. | closed |
| T-24-03 | Elevation of Privilege | new routes in `server.go` | high | mitigate | Both routes are registered inside `registerDataRoutes`, so they sit behind `gate.Authenticate`; `TestTags_Gated401NoCookie` proves 401 without a session. | closed |
| T-24-04 | Tampering (CSRF) | POST/DELETE tag routes | high | mitigate | Inherited `gate.RequireCSRFHeader`; `TestTags_GatedForbiddenWithoutCSRFHeader` proves 403 and zero store calls without X-Requested-With. | closed |
| T-24-05 | Denial of Service | attach request body | medium | mitigate | `http.MaxBytesReader(maxTagBodyBytes = 4096)` before `decodeJSONBody`; names are capped at 32 runes before any query. | closed |
| T-24-06 | Tampering (injection) | `queries/tags.sql` | medium | mitigate | Every statement is sqlc-generated with bound parameters; no string-built SQL exists in `internal/tags`. | closed |
| T-24-07 | Information Disclosure | error responses and logs | medium | mitigate | Error bodies are fixed strings via `writeError`; `httplog.SetAttrs` carries `err.Error()` only, never the submitted name (acceptance criterion greps this). | closed |
| T-24-08 | Spoofing | visually confusable tag names | low | accept | Zero-width or look-alike characters can make two tags that read the same; a single-operator app where the operator authors every tag, and D-31's NFC + lower() identity already folds the common cases. | closed |
| T-24-09 | Denial of Service | unbounded tag vocabulary | low | accept | D-12 keeps zero-carrier tags until deleted; one operator creates them by hand, and each name is capped at 32 runes. | closed |
| T-24-SC | Tampering | npm/pip/cargo installs | high | accept | No new package in any plan; frozen lockfile and dependency diff gates. | closed |
| T-24-10 | Elevation of Privilege | four vocabulary routes | high | mitigate | Registered inside `registerDataRoutes`; `TestTags_Vocabulary_Gated401NoCookie` proves 401 on all four. | closed |
| T-24-11 | Tampering (CSRF) | PATCH / POST merge / DELETE | high | mitigate | Inherited `gate.RequireCSRFHeader`; `TestTags_Vocabulary_GatedForbiddenWithoutCSRFHeader` proves 403 with zero store calls. | closed |
| T-24-12 | Tampering | silent merge on rename | high | mitigate | `Service.Rename` never moves links: a collision returns `*CollisionError` and the handler writes 409; a test reads the source tag and links back after the 409 and asserts no change. | closed |
| T-24-13 | Denial of Service | concurrent merges | medium | mitigate | `LockTagsForMerge ... | closed |
| T-24-14 | Tampering | merge pushing an artist past 10 | medium | mitigate | Delete-duplicates then UPDATE, never INSERT (D-19); `TestService_Merge_AtCap` proves a 10-tag artist merges. | closed |
| T-24-15 | Tampering | target renamed from a second tab during the modal merge confirm | low | accept | `{into: id}` cannot detect it; the merge lands under the target's current stored name (D-23). | closed |
| T-24-16 | Information Disclosure | 409 body | low | accept | The body names an existing tag and a count to the same authenticated operator who owns the vocabulary; nothing cross-tenant exists. | closed |
| T-24-17 | Denial of Service | delete/rename request bodies | low | mitigate | `http.MaxBytesReader(maxTagBodyBytes)` before decode; names capped at 32 runes before any query. | closed |
| T-24-18 | Elevation of Privilege | PUT /watchlist/{id}/note | high | mitigate | Registered inside `registerDataRoutes`; `TestWatchlist_Note_Gated401NoCookie` proves 401 without a session. | closed |
| T-24-19 | Tampering (CSRF) | PUT /watchlist/{id}/note | high | mitigate | Inherited `gate.RequireCSRFHeader`; `TestWatchlist_Note_GatedForbiddenWithoutCSRFHeader` proves 403 with zero store calls. | closed |
| T-24-20 | Information Disclosure | note text in logs/errors | medium | mitigate | Error bodies are fixed strings; `httplog.SetAttrs` receives `err.Error()` only — an acceptance criterion greps the handler. | closed |
| T-24-21 | Denial of Service | note request body | medium | mitigate | `http.MaxBytesReader(maxAddWatchlistBodyBytes)` before decode; 500-rune cap in the handler, the service, and the `watchlist_note_length` CHECK. | closed |
| T-24-22 | Tampering | PATCH setting a note | medium | mitigate | `updateWatchlistRequest` has no note field and `decodeJSONBody` disallows unknown keys; `TestWatchlist_Patch_RejectsNoteKey` proves the 400 (D-25). | closed |
| T-24-23 | Tampering (stored XSS) | note text rendered later | high | transfer | The API stores the operator's text verbatim by design; plan 24-07 renders it as a plain JSX text node only and asserts the raw-HTML prop never appears in `web/app`. | closed |
| T-24-24 | Tampering (stored XSS) | `TagChips` label | high | mitigate | Names render as plain JSX text nodes; a test renders an HTML-looking name literally, and `grep -rn "dangerouslySetInnerHTML" web/app` must print nothing. | closed |
| T-24-25 | Tampering (CSRF) | `detachTag` | high | mitigate | The wrapper goes through `apiFetch`, which injects `X-Requested-With` on every non-GET; `api.test.ts` asserts the header on the DELETE. | closed |
| T-24-26 | Repudiation / integrity of UI state | optimistic removal | medium | mitigate | A failed DELETE restores the chip at its index and toasts; a test asserts the restore (T-06-13 precedent). | closed |
| T-24-27 | Tampering (stored XSS) | combobox options, pending chips | high | mitigate | Option labels and chip labels are plain JSX text nodes; the phase-wide raw-HTML grep in plan 24-04/24-07 covers the new files. | closed |
| T-24-28 | Tampering (CSRF) | `attachTag` | high | mitigate | Goes through `apiFetch`, which injects `X-Requested-With`; `api.test.ts` asserts the header on the POST. | closed |
| T-24-29 | Tampering (supply chain) | shadcn vendoring | medium | mitigate | Uses the repo-pinned local shadcn 4.16.2 binary against the first-party `base-maia` registry (UI-SPEC Registry Safety); `git diff --exit-code` proves `package.json`, the lockfile, `button.tsx`, and `input.tsx` are unchanged; vendored files are reviewed for network calls and raw-HTML use before commit. | closed |
| T-24-30 | Denial of Service | vocabulary request volume | low | mitigate | `loadVocabulary` fires only from `idle`/`error`, so repeated `+ tag` opens do not refetch; the Watchlist mount never requests it (D-30). | closed |
| T-24-31 | Tampering | client-side cap bypass | low | accept | The UI hint is convenience only; the API and the database trigger (plan 24-01) are the enforcement. | closed |
| T-24-32 | Tampering (CSRF) | `renameTag`, `mergeTag`, `deleteTag` | high | mitigate | All three go through `apiFetch` (X-Requested-With injected); `api.test.ts` asserts the header on each; no direct `fetch(` is added. | closed |
| T-24-33 | Tampering | unconfirmed merge | high | mitigate | `mergeTag` is called only from the merge ConfirmDialog's `onConfirm`; a test asserts it is not called when the collision dialog opens or on Cancel (D-09). | closed |
| T-24-34 | Tampering (stored XSS) | tag names in rows, confirms, toasts | high | mitigate | Names render as plain JSX text; the phase-wide raw-HTML grep stays empty. | closed |
| T-24-35 | Tampering (supply chain) | dialog / alert-dialog vendoring | medium | mitigate | Pinned local shadcn CLI, first-party registry; `git diff --exit-code` on `package.json`, lockfile, and `button.tsx`. | closed |
| T-24-36 | Repudiation | irreversible delete/merge | low | accept | No undo by design (D-17); both require an explicit count-stating confirmation. | closed |
| T-24-37 | Tampering (stored XSS) | `ArtistNote` paragraph | high | mitigate | The note renders as a plain JSX text node with `whitespace-pre-line`; a test renders an HTML-looking note literally, and `! grep -rn "dangerouslySetInnerHTML" web/app` must exit 0. | closed |
| T-24-38 | Tampering (CSRF) | `updateNote`, `addWatchlist` | high | mitigate | Both go through `apiFetch` (X-Requested-With injected); `api.test.ts` asserts the header on the PUT. | closed |
| T-24-39 | Information Disclosure | note text leaving the instance | medium | mitigate | The blast-radius check proves no change under `internal/notifier`, `internal/discord`, or the external clients this phase. | closed |
| T-24-40 | Tampering | stale embedded bundle | medium | mitigate | `make web` output is committed and the verify step fails when the bundle did not change. | closed |
| T-24-41 | Tampering | `artist_tags` via raw `UPDATE ... SET artist_id` | medium | mitigate | 000011's `artist_tags_cap_update_trigger BEFORE UPDATE OF artist_id` runs the cap function. | closed |
| T-24-42 | Denial of Service | Artist-row lock on the new UPDATE path | low | mitigate | Same `FOR NO KEY UPDATE` as the INSERT path, so it never conflicts with the `KEY SHARE` FK locks from `watchlist`/`events`. | closed |
| T-24-43 | Tampering | Editing applied migration 000010 | medium | mitigate | 000010 stays unchanged, enforced by acceptance `git diff --exit-code 447baa0 -- 000010_*`. | closed |
| T-24-44 | Denial of Service | N-1 rollback against schema 11 | low | mitigate | 000011 is trigger/function DDL only, with no DROP/RENAME/CHECK/column. | closed |
| T-24-45 | Information Disclosure | Cap error text | low | accept | The trigger raises a fixed operator-authored message with no tag or artist names. | closed |
| T-24-46 | Tampering (integrity of user intent) | stale vocabulary in `dropTagFromEntries` | medium | mitigate | A functional `setVocabulary` filter on delete, plus a route test proving the deleted name is offered only as `Create “…”` and never as an existing item, with a single `listTags` call. | closed |
| T-24-47 | Tampering (supply chain) | `make web` frozen install | medium | mitigate | `pnpm install --frozen-lockfile` inside `make web`, and an acceptance diff requiring `web/package.json` and `web/pnpm-lock.yaml` unchanged against main. | closed |
| T-24-48 | Tampering (stored XSS) | tag names in suggestions | low | accept | No rendering path changes. | closed |
| T-24-49 | Tampering | `check_artist_tags_max_per_artist()` skip-existing check under a concurrent uncommitted detach (CR-01) | medium | mitigate | 000012 replaces both visibility-only checks with `PERFORM ... | closed |
| T-24-50 | Denial of Service | The new row lock on an existing link (waits, deadlock) | low | mitigate | Lock order is unchanged: link row, then artist row. | closed |
| T-24-51 | Tampering | Editing applied migrations 000010/000011 | medium | mitigate | Neither is edited. | closed |
| T-24-52 | Denial of Service | N-1 rollback against schema 12 | low | mitigate | 000012 is function-body DDL only, with no column, CHECK, DROP, or RENAME. | closed |
| T-24-53 | Information Disclosure | Cap error text | low | accept | The fixed operator-authored message carries no tag or artist names. | closed |
| T-24-54 | Tampering (integrity of user intent) | Unguarded `loadVocabulary` settle in `watchlist.tsx` (WR-06) | medium | mitigate | The `vocabGen` generation ref is bumped by every fresher vocabulary write, and a stale result is dropped. | closed |
| T-24-55 | Tampering (supply chain) | `make web` frozen install | medium | mitigate | `pnpm install --frozen-lockfile` inside `make web`, and an acceptance diff requiring `web/package.json` and `web/pnpm-lock.yaml` unchanged against main. | closed |
| T-24-56 | Tampering (stored XSS) | Tag names in suggestions | low | accept | No rendering path changes. | closed |
| T-24-57 | Repudiation (test integrity) | De-flaking the Esc-on-merge-confirm test | low | mitigate | Only a focus synchronization gate is added. | closed |
| T-24-58 | Tampering (integrity of user intent) | Unguarded `ManageTagsDialog.load()` settle feeding `handleTagsLoaded` (WR-08) | medium | mitigate | A `loadGen` generation ref: each load captures it, only the current generation applies `setTags`/`setStatus`/`onLoaded`, and `invalidateLoad()` bumps it on delete, rename, and merge success. | closed |
| T-24-59 | Denial of service (UI availability) | The generation bump dropping the only in-flight load, which would leave the dialog on its skeleton | low | mitigate | `invalidateLoad()` restarts `load()` while `loadInFlight.current` is true. | closed |
| T-24-60 | Repudiation (test integrity) | Pin tests that pass before Task 2's change could be vacuous | low | mitigate | A recorded mutation check: removing the `.then` or `.catch` early return makes the matching pins fail. | closed |
| T-24-61 | Tampering (supply chain) | `make web` frozen install | medium | mitigate | `pnpm install --frozen-lockfile` inside `make web`, plus an acceptance diff requiring `web/package.json` and `web/pnpm-lock.yaml` unchanged against main. | closed |
| T-24-62 | Tampering (stored XSS) | Tag names in the dialog and suggestions | low | accept | No rendering path changes. | closed |
| T-24-63 | Tampering (integrity of user intent) | Merge and delete ConfirmDialog titles overflowing sideways at narrow widths (G-24-5), hiding part of a tag name, so the user confirms a merge or delete without seeing both names (D-09) | low | mitigate | `wrap-anywhere` on `AlertDialogTitle` and `AlertDialogDescription` in ConfirmDialog. | closed |
| T-24-64 | Tampering (supply chain) | Frozen install inside the `web` recipe | medium | mitigate | `pnpm install --frozen-lockfile` before the build, plus the acceptance diff that requires `web/package.json` and `web/pnpm-lock.yaml` unchanged against main. | closed |
| T-24-65 | Tampering (stored XSS) | Tag names in the confirm title and description | low | accept | No rendering path changes. | closed |
| T-24-66 | Tampering (integrity of user intent) | ManageTagsDialog initial-focus hand-off re-arming on reload | low | mitigate | Hand-off armed only by `open` and consumed by the first successful load; pinned by the "reload ... leaves focus alone" test in `ManageTagsDialog.test.tsx`. | closed |
| T-24-67 | Denial of Service (accessibility) | ArtistNote 'less' toggle unmounting while focused (G-24-9) | low | mitigate | Overflow measured only while clamped; pinned by the fake-ResizeObserver test in `ArtistNote.test.tsx`. | closed |
| T-24-68 | Tampering (supply chain) | Frozen install inside the `web` recipe | medium | mitigate | `pnpm install --frozen-lockfile` in `make web`. | closed |
| T-24-69 | Tampering (stored XSS) | Note text and tag names in the changed components | low | accept | No rendering path changes; still plain JSX text (T-24-37). | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-08 | T-24-08 | Zero-width or look-alike characters can make two tags that read the same; a single-operator app where the operator authors every tag, and D-31's NFC + lower() identity already folds the common cases. | operator (plan-time register) | 2026-10-05 |
| AR-09 | T-24-09 | D-12 keeps zero-carrier tags until deleted; one operator creates them by hand, and each name is capped at 32 runes. | operator (plan-time register) | 2026-10-05 |
| AR-SC | T-24-SC | No new package in any plan; frozen lockfile and dependency diff gates. | operator (plan-time register) | 2026-10-05 |
| AR-15 | T-24-15 | `{into: id}` cannot detect it; the merge lands under the target's current stored name (D-23). | operator (plan-time register) | 2026-10-05 |
| AR-16 | T-24-16 | The body names an existing tag and a count to the same authenticated operator who owns the vocabulary; nothing cross-tenant exists. | operator (plan-time register) | 2026-10-05 |
| AR-31 | T-24-31 | The UI hint is convenience only; the API and the database trigger (plan 24-01) are the enforcement. | operator (plan-time register) | 2026-10-05 |
| AR-36 | T-24-36 | No undo by design (D-17); both require an explicit count-stating confirmation. | operator (plan-time register) | 2026-10-05 |
| AR-45 | T-24-45 | The trigger raises a fixed operator-authored message with no tag or artist names. | operator (plan-time register) | 2026-10-05 |
| AR-48 | T-24-48 | No rendering path changes. | operator (plan-time register) | 2026-10-05 |
| AR-53 | T-24-53 | The fixed operator-authored message carries no tag or artist names. | operator (plan-time register) | 2026-10-05 |
| AR-56 | T-24-56 | No rendering path changes. | operator (plan-time register) | 2026-10-05 |
| AR-62 | T-24-62 | No rendering path changes. | operator (plan-time register) | 2026-10-05 |
| AR-65 | T-24-65 | No rendering path changes. | operator (plan-time register) | 2026-10-05 |
| AR-69 | T-24-69 | No rendering path changes; still plain JSX text (T-24-37). | operator (plan-time register) | 2026-10-06 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-10-05 | 66 | 66 | 0 | gsd-secure-phase (orchestrator, ASVS L1 grep-depth) |
| 2026-10-06 | 70 | 70 | 0 | gsd-secure-phase (orchestrator, ASVS L1 grep-depth) |

L1 evidence: every named pin test exists (`internal/db`, `internal/tags`, `internal/httpserver`, `web/app`) and ran green during validate-phase on 2026-10-05; tag/note routes are registered inside `registerDataRoutes` (session + CSRF gate); the only `fetch(` in `web/app/lib/api.ts` is inside `apiFetch`; `mergeTag` is called only from `handleConfirmMerge` (ConfirmDialog `onConfirm`); zero `dangerouslySetInnerHTML` in `web/app`; zero string-built SQL in `internal/tags`; no diff vs main under `internal/notifier`, `internal/discord`, or the external clients; migrations 000010/000011 unchanged since `77de93d`.

## Security Audit 2026-10-05
| Metric | Count |
|--------|-------|
| Threats found | 66 |
| Closed | 66 |
| Open | 0 |

## Security Audit 2026-10-06
| Metric | Count |
|--------|-------|
| Threats found | 70 |
| Closed | 70 |
| Open | 0 |

Added plan 24-14's T-24-66…69 (its T-24-SC collapses into the existing row). L1 evidence re-checked: tag/note routes inside `registerDataRoutes`, sole `fetch(` inside `apiFetch`, zero `dangerouslySetInnerHTML` in `web/app`, `--frozen-lockfile` in `make web`. Post-phase, `43f00c1` changed `web/pnpm-lock.yaml` via `pnpm-workspace.yaml` overrides to clear new trivy-fs CVEs in transitive deps (no new package); CVE-2026-93687 (`braces`, unfixed, dev-only via shadcn CLI) is in `.trivyignore` until 2026-11-05.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-10-06
