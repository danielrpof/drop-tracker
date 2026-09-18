---
phase: "20"
slug: "digest-settings-operator-control"
status: verified
threats_open: 0
asvs_level: 1
created: "2026-09-14"
---

# Phase 20 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| browser → gated API | `GET`/`PUT /settings/notifications` cross `gate.Authenticate`; the PUT additionally crosses `gate.RequireCSRFHeader` | operator session cookie, digest mode/cadence JSON |
| untrusted JSON → Postgres | a client-supplied cadence string reaches a `CHECK`-constrained column | `digest_cadence` value |
| application → schema | a new boot migration (000008) runs against a live database that a previous release's binary may be rolled back onto | schema DDL |
| unauthenticated browser → gated API | a caller with no session cookie reaching either settings verb | none (rejected 401) |
| cross-site page → gated API | a forged `PUT` issued from another origin against an authenticated browser | forged request (rejected 403 without `X-Requested-With`) |
| store failure text → HTTP response | a database error whose message can embed a DSN or webhook URL | error text (redacted server-side) |
| settings JSON → React DOM | server-authored values (cadence, timestamp) rendered into the page | `digest_cadence`, `digest_last_sent_at` |
| external registry → repo source tree | the shadcn CLI fetches a component definition over the network and writes it into `web/app/components/ui/` | vendored `select.tsx` source |
| SPA source → embedded binary | `internal/webassets/build/client` is committed and embedded at Go build time | built JS/CSS bundle |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-20-01 | Elevation of privilege | `/settings/notifications` route registration | high | mitigate | Both verbs registered only inside `registerDataRoutes`; proven behaviorally by `TestSettings_GetGated401NoCookie`/`PutGated401NoCookie` | closed |
| T-20-02 | Tampering | `PUT` cadence value | medium | mitigate | Three independent layers: handler `ParseCadence` → `Service.Update` re-validation → DB `CHECK`; pinned by `TestService_UpdateRejectsUnknownCadence` | closed |
| T-20-03 | Tampering | settings row multiplicity | medium | mitigate | `CHECK (id = 1)` + seed row, no `INSERT` code path; pinned by `TestService_SecondRowRejectedByCheckConstraint` | closed |
| T-20-04 | Information disclosure | error responses from both handlers | medium | mitigate | Fixed `internal error` body, server-side-only `httplog.SetAttrs`; pinned by `TestSettings_NoLeak` | closed |
| T-20-05 | Denial of service | `PUT` request body | low | mitigate | `http.MaxBytesReader` at the shared 64 KiB ceiling; pinned by `TestSettings_PutRejectsOversizeBody` | closed |
| T-20-06 | Tampering | migration 000008 vs. N-1 rollback | medium | mitigate | Additive `CREATE TABLE` + paired `.down.sql`; `cmd/migration-check --mode=scan` reports zero findings | closed |
| T-20-07 | Spoofing | settings routes on a gated instance | high | mitigate | `gate.Authenticate` inherited from `registerDataRoutes`; pinned live by `TestSettings_GetGated401NoCookie`/`PutGated401NoCookie` (401, no leaked `settings` key) | closed |
| T-20-08 | Tampering (CSRF) | `PUT /settings/notifications` | high | mitigate | `gate.RequireCSRFHeader`; pinned live by `TestSettings_PutGatedForbiddenWithoutCSRFHeader`/`PutGatedSucceedsWithCookieAndHeader` via a real minted session | closed |
| T-20-09 | Tampering | cadence allow-list | medium | mitigate | Bogus/omitted/wrong-typed cadence all rejected 4xx, row unchanged; pinned by `TestSettings_PutRejects*` (6 cases) | closed |
| T-20-10 | Information disclosure | 500 path on either verb | medium | mitigate | No-leak assertion on the raw response body; pinned by `TestSettings_NoLeak` | closed |
| T-20-11 | Denial of service | oversize `PUT` body | low | mitigate | `http.MaxBytesReader` trips before decode; pinned by `TestSettings_PutRejectsOversizeBody` | closed |
| T-20-12 | Tampering (CSRF) | `updateDigestSettings()` | high | mitigate | `apiFetch` injects `X-Requested-With` on every non-GET centrally; pinned by `api.test.ts`'s CSRF-header assertion, server half proven in plan 20-02 | closed |
| T-20-13 | Information disclosure | mount 401 handling | medium | mitigate | `apiFetch`'s 401 interceptor flips `authStore`; view/card both early-return before rendering settings-derived text; pinned by `system.test.tsx` | closed |
| T-20-14 | Information disclosure | failure copy | low | mitigate | Only the two locked operator-authored strings render; `ApiError.message` never interpolated; matches `system.tsx`'s existing fixed-string catch | closed |
| T-20-15 | Tampering (XSS) | rendering settings values | low | mitigate | Every value is a JSX text node (React-escaped); repo-wide zero raw-HTML-injection props under `web/app` | closed |
| T-20-16 | Denial of service | repeated saves | low | mitigate | Synchronous re-entrancy ref + disabled control bound to one in-flight PUT; pinned by `DigestSettings.test.tsx` | closed |
| T-20-17 | Tampering | shadcn registry add of `select` | medium | mitigate | Pre-recorded first-party provenance (`20-UI-SPEC.md`); acceptance gate proves no `package.json`/`pnpm-lock.yaml` diff; confirmed empty in `20-VERIFICATION.md` | closed |
| T-20-18 | Tampering (XSS) | cadence and timestamp rendering | low | mitigate | Cadence rendered from a closed two-value option list, never echoed from the server string; timestamp via shared formatters + `<time>` as a JSX text node | closed |
| T-20-19 | Tampering | full-object PUT from the cadence control | medium | mitigate | Shared `save()` helper always sends both fields; a cadence change cannot silently clear digest mode; pinned by `DigestSettings.test.tsx` | closed |
| T-20-20 | Tampering | embedded bundle drift | medium | mitigate | `make web` rebuilds the committed tree in the final commit, gated on the tree actually changing; confirmed non-empty-then-committed in `20-VERIFICATION.md` | closed |
| T-20-SC | Tampering | npm/pnpm installs (supply chain) | medium | mitigate | Manifest/lockfile byte-identical after the shadcn registry action; confirmed empty diff (`git diff --name-only -- web/package.json web/pnpm-lock.yaml`) in `20-VERIFICATION.md` | closed |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (`high`) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

No accepted risks.

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-14 | 21 | 21 | 0 | Claude (gsd-secure-phase, register authored at plan time — L1 short-circuit, ASVS level 1) |

All four `high`-severity threats (T-20-01, T-20-07, T-20-08, T-20-12) were spot-verified directly against the live test files during this audit (`internal/httpserver/settings_test.go`, `web/app/lib/api.test.ts`), not merely accepted from PLAN.md's mitigation claim. The remaining 17 medium/low threats were cross-checked against `20-VERIFICATION.md`'s independently re-run behavioral spot-checks and `20-REVIEW.md`'s code review (0 critical, 2 non-blocking warnings unrelated to any threat above).

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log (none)
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-14
