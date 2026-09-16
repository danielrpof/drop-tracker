---
phase: 02
slug: watchlist-core
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-06
---

# Phase 02 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

This phase's threat register was authored at plan time across all 8 plans (02-01 through 02-08,
the last four being UAT gap-closure plans), so this run is a retroactive verification against the
implementation rather than a from-scratch STRIDE pass. `register_authored_at_plan_time: true`.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| client → chi router | Untrusted JSON request bodies and HTTP headers | POST/PATCH bodies, headers |
| client → `handleAddWatchlist` / `handleUpdateWatchlist` | Unbounded request body until `MaxBytesReader` binds it | JSON body |
| client → `{id}` path segment | Attacker-controlled string reaching a query parameter | path param |
| any intermediary (proxy, gateway, future WAF) → the Go server | Two parsers reading the same bytes; disagreement about where the body ends is independently exploitable | JSON body |
| handler → watchlist service | Decoded, still-unvalidated user data | domain params |
| `internal/httpserver` → `internal/watchlist` | The documented reusable domain surface future non-HTTP callers (Phase 3 search proxy, Phase 4 poller) will also use | function calls |
| watchlist service → Postgres | Domain params, preference arrays crossing into `CHECK`-constrained columns | SQL params |
| Go validation layer → schema constraints | Defense-in-depth seam: anything bypassing the service still meets the database | preference arrays |
| concurrent client → concurrent client | Two requests mutating one watchlist row with no coordination | row state |
| Postgres → `GET /watchlist` response | Every stored watchlist row crosses back to the client in one payload | JSON array |
| `artists.image_url` → Phase 5/6 (Discord embeds, React UI) | A stored, client-supplied URL a later phase will render | URL string |
| build → module cache | `go.mod` / `go.sum` dependency resolution | Go modules |
| `DATABASE_URL` env var → `internal/db` | The process's highest-value secret, entering as a raw string in either URL or keyword/value form | DSN string |
| `internal/db` → log sink | `RunMigrations`' retry `Warn` line reaches stdout and any log aggregator | log line |
| `internal/db` → returned error → `cmd/server/main.go` | Final failure message on the fatal-exit path | error string |
| third-party driver error text → `redactError` | Attacker-influenced, dependency-version-dependent text | error text |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-02-01 | Tampering | `handleAddWatchlist` body decode | high | mitigate | DTO decode with `DisallowUnknownFields()`; server-owned fields absent from DTO | closed |
| T-02-02 | Tampering | `internal/watchlist` → sqlc | high | mitigate | Every statement is a sqlc-generated parameterized call; no raw SQL | closed |
| T-02-03 | Information Disclosure | error responses | high | mitigate | `writeError` emits only fixed strings; driver text confined to structured logs | closed |
| T-02-04 | Denial of Service | request body reader | medium | mitigate | `http.MaxBytesReader(w, r.Body, 65536)` before decode | closed |
| T-02-05 | Denial of Service | `mbid`/`name` field lengths | medium | mitigate | Rune-counted 36/512 caps before any DB call | closed |
| T-02-06 | Spoofing | `/watchlist` has no authentication | medium | accept | Single-operator deployment, no multi-tenant boundary; revisit if multi-user promoted | closed |
| T-02-07 | Tampering | `{id}` path parameter | medium | mitigate | `parseWatchlistID` rejects parse failures and values < 1 before any service call | closed |
| T-02-08 | Information Disclosure | sequential `BIGSERIAL` ids | low | accept | No auth boundary to enumerate across in a single-operator deployment | closed |
| T-02-09 | Denial of Service | `GET /watchlist` returns entire table, no pagination | low | accept | Human-scale watchlist size; no defined page-size contract yet | closed |
| T-02-10 | Tampering | PATCH preference values | medium | mitigate | `normalizeSet` allow-list + CHECK constraint backstop | closed |
| T-02-11 | Tampering | PATCH mass assignment | medium | mitigate | `updateWatchlistRequest` declares only the two preference fields, `DisallowUnknownFields()` | closed |
| T-02-12 | Repudiation | no dedicated audit trail | low | accept | `httplog` structured request logging sufficient for single-operator v1 | closed |
| T-02-13 | Tampering | preference array values on add path | medium | mitigate | `normalizeSet` + CHECK constraint backstop | closed |
| T-02-14 | Tampering | duplicate-add implicit preferences overwrite | high | mitigate | SQLSTATE 23505 + constraint-name match → `ErrDuplicate`, no update/retry | closed |
| T-02-15 | Tampering | concurrent deletes of the same row | medium | mitigate | Single `:execrows` DELETE relies on Postgres row-level locking | closed |
| T-02-16 | Denial of Service | PATCH request body size | medium | mitigate | Same 64 KiB `MaxBytesReader` ceiling as the add path | closed |
| T-02-17 | Tampering | `UpsertArtist` conflict clause widened to overwrite metadata | medium | accept | No new principal/privilege; same no-auth-boundary rationale as T-02-06 | closed |
| T-02-18 | Tampering | `artists.image_url` stored client-supplied URL | medium | accept | No rendering surface in v1; flagged forward to Phase 5/6 to validate before use | closed |
| T-02-19 | Tampering | lost update between concurrent PATCH calls | high | mitigate | Single data-modifying CTE resolves untouched axis via `CASE...ELSE <column>` under the row lock the UPDATE itself takes | closed |
| T-02-20 | Information Disclosure | 500 on deleted-mid-write race | medium | mitigate | `pgx.ErrNoRows` translated to `ErrNotFound` → honest scrubbed 404 | closed |
| T-02-21 | Denial of Service | row-lock contention on single-statement update | low | accept | Lock held only for statement duration, no cross-round-trip transaction | closed |
| T-02-22 | Tampering | empty-preferences update silently no-ops | medium | mitigate | `ErrNoPreferencesSupplied` returned before any DB call | closed |
| T-02-23 | Repudiation | `updated_at` advances on a no-op write | low | mitigate | Same guard as T-02-22 prevents the write entirely | closed |
| T-02-24 | Tampering | JSON body smuggling (trailing concatenated value) | medium | mitigate | Shared `decodeJSONBody` asserts `io.EOF` after primary decode | closed |
| T-02-25 | Denial of Service | second decode pass over body remainder | low | accept | Body already bounded by `MaxBytesReader` before either decode runs | closed |
| T-02-26 | Information Disclosure | 400 bodies from new rejection paths | low | mitigate | Fixed operator-authored strings via `writeError`, never raw error text | closed |
| T-02-27 | Information Disclosure | `redactError` missed libpq keyword/value-form passwords | critical | mitigate | `kvPasswordPattern` applied after the existing userinfo strip | closed |
| T-02-28 | Information Disclosure | reliance on pgx's undocumented self-redaction | high | mitigate | `redactError` now meets its contract independently; doc comment records this | closed |
| T-02-29 | Information Disclosure | over-broad `kvPasswordPattern` swallowed trailing URL query params (`&sslmode=disable`) | medium | mitigate | Fixed 2026-08-06: unquoted branch changed from `\S+` to `[^\s&'"]+`; regression pinned by a `mustContain` assertion on the query-parameter fixture | closed |
| T-02-30 | Information Disclosure | realistic fixture secret committed by the fix's own tests | medium | mitigate | All fixtures built from the project's non-entropic `local-test-fixture-password` marker; gitleaks-gated | closed |
| T-02-31 | Repudiation | test claimed to guard CR-01 while only exercising a dial-failure path | medium | mitigate | Renamed to `..._DialFailurePath` with a corrected doc comment; real coverage lives in `redact_test.go` | closed |
| T-02-32 | Denial of Service | regex evaluation over attacker-influenced error text | low | accept | Go `regexp` is RE2 (linear time, no backtracking) | closed |
| T-02-33 | Denial of Service | retry backoff exponent overflow collapses to a zero-delay retry storm | low | mitigate | Fixed 2026-08-06: `backoffDelay` caps the shift at `maxBackoffShift` (32) before the left shift, plus a `delay <= 0` backstop; `newRetryConfig` also clamps a non-positive `WithMaxAttempts` to 1 | closed |
| T-02-34 | Denial of Service | `deezer_id`/`disambiguation`/`image_url` unbounded and untrimmed on the add path | low | mitigate | Fixed 2026-08-06: same `TrimSpace` + rune-cap treatment as `mbid`/`name` (64/512/2048), enforced before any DB call | closed |
| T-02-SC | Tampering | Go module supply chain | medium | mitigate | No `go get -u`; only Phase-1-vetted pins promoted indirect→direct; `go mod verify` clean; `git diff --exit-code -- go.mod go.sum` gated across every gap-closure plan | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-06 | No authentication on `/watchlist`; single-operator deployment with no multi-tenant boundary to spoof across. REQUIREMENTS.md explicitly places accounts/SSO out of scope for v1. | developer (plan 02-01) | 2026 |
| AR-02-02 | T-02-08 | Sequential `BIGSERIAL` ids disclose nothing an authorised caller could not already list, given no auth boundary exists. | developer (plan 02-03) | 2026 |
| AR-02-03 | T-02-09 | `GET /watchlist` has no pagination; a single operator's watchlist is human-scale (tens of rows). No page-size contract exists until Phase 6's UI does. | developer (plan 02-03) | 2026 |
| AR-02-04 | T-02-12 | No dedicated audit trail for preference mutations; Phase 1's `httplog` structured request logging is sufficient for a single-operator deployment. | developer (plan 02-04) | 2026 |
| AR-02-05 | T-02-17 | Widening `UpsertArtist`'s ON CONFLICT clause to overwrite `disambiguation`/`image_url` adds no new principal or privilege given no auth boundary exists (same rationale as AR-02-01). | developer (plan 02-05) | 2026 |
| AR-02-06 | T-02-18 | `artists.image_url` is stored client-supplied input with no rendering surface in v1 (only ever echoed as a JSON string field). Phase 5 (Discord embeds) and Phase 6 (React UI) are on notice via this entry to validate before rendering. | developer (plan 02-05) | 2026 |
| AR-02-07 | T-02-21 | Row-lock contention on the single-statement preferences update is bounded to one `UPDATE` on one row; no explicit transaction spans a client round trip. | developer (plan 02-06) | 2026 |
| AR-02-08 | T-02-25 | The second decode pass in `decodeJSONBody` reads at most the `MaxBytesReader`-bounded remainder once; no unbounded read is introduced. | developer (plan 02-07) | 2026 |
| AR-02-09 | T-02-32 | Regex DoS is structurally ruled out by Go's RE2 engine (linear time, no catastrophic backtracking), on a bounded-retry failure path. | developer (plan 02-08) | 2026 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-06 | 35 | 32 | 3 (all non-blocking, medium/low) | gsd-security-auditor (opus) |
| 2026-08-06 | 35 | 35 | 0 | developer fix pass — T-02-29 (regex over-redaction), T-02-33 (retry backoff overflow), T-02-34 (unbounded optional metadata) fixed same-day per developer's "fix now" disposition on all three findings |

**What changed in the fix pass:**
- `internal/db/migrate.go`: `kvPasswordPattern`'s unquoted branch narrowed from `\S+` to `[^\s&'"]+` so a URL query-parameter password no longer swallows trailing sibling parameters (`&sslmode=disable`); `redact_test.go` gained a `mustContain` assertion pinning the regression.
- `internal/db/migrate.go`: backoff delay calculation extracted to `backoffDelay`, exponent capped at `maxBackoffShift` (32) plus a `delay <= 0` backstop, preventing a uint64 shift overflow from collapsing exponential backoff to a zero-wait retry storm at very large attempt counts; `newRetryConfig` clamps a non-positive `WithMaxAttempts` to 1. New `internal/db/backoff_test.go` covers both deterministically (no real-time waiting).
- `internal/httpserver/watchlist.go`: `deezer_id`, `disambiguation`, `image_url` on the add path now receive the same `TrimSpace` + rune-cap treatment `mbid`/`name` already had (T-02-05's original scope missed these three). New tests in `watchlist_test.go` cover rejection and trimming.
- All three fixes verified: `go build ./... && go vet ./...` clean, full suite green against the fixture Postgres (`internal/db`, `internal/httpserver`, `internal/watchlist` all `ok`), `pre-commit run gitleaks --all-files` passed, `git diff --exit-code -- go.mod go.sum` clean (no dependency movement).

**Auditor findings not carried into the register as new blocking threats:** none — T-02-29, T-02-33 and T-02-34 were the only gaps found and all three are now closed.

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-06
