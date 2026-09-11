---
phase: "19"
slug: "frontend-system-view"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-11"
---

# Phase 19 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| gated instance -> browser | `GET /status` crosses `gate.Authenticate`; a missing/expired session cookie yields 401, never a partial payload | server-authored run/instance status |
| `/status` wire values -> formatter output -> DOM | server-authored timestamps, durations, and the configured poll interval pass through pure string builders | timestamps, durations, integers |
| external registry -> repo source tree | the shadcn CLI fetches a component definition over the network and writes it into `web/app/components/ui/` | vendored component source |
| gated instance -> browser (manual Refresh) | a manual Refresh re-crosses `gate.Authenticate`; the session may have expired since mount | server-authored run/instance status |
| source tree -> embedded binary | `internal/webassets/build/client` is committed and embedded into the Go binary at build time | built SPA bundle |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-19-01 | Information disclosure | `system.tsx` mount effect / `getStatus()` | medium | mitigate | Routed solely through `apiFetch`; the D-16 401 interceptor (`web/app/lib/api.ts:206-208`) flips `authStore.markUnauthenticated()` and the mount catch returns early on `ApiError` 401. Verified: interceptor present, pinned by the mount-401 test. | closed |
| T-19-02 | Tampering | `web/app/lib/api.ts` wire types | low | mitigate | Types restricted to fields in `docs/api/status-contract.md`; `outcome` typed as the known union widened with a `string` arm. Verified in `api.ts`. | closed |
| T-19-03 | Information disclosure | `system.tsx` error copy | low | mitigate | Error state renders fixed operator-authored copy; `err`/`err.message` never interpolated into rendered output. Verified: only `err.status === 401` branch checks, no message interpolation. | closed |
| T-19-04 | Tampering (XSS) | `system.tsx` rendering `instance.app_version` | low | mitigate | Rendered as a JSX text node; React escapes it. Verified: zero `dangerouslySetInnerHTML` matches repo-wide. | closed |
| T-19-SC (01) | Tampering | npm/pnpm installs | low | accept | Plan 19-01 runs no package install; `web/package.json`/`web/pnpm-lock.yaml` untouched. | closed |
| T-19-05 | Information disclosure | `formatAbsoluteTime`/`formatRelativeTime` malformed-input path | low | mitigate | Unparseable/null timestamp returns a fixed em-dash placeholder instead of the runtime's own invalid-date text. Pinned by a dedicated test case (`format.test.ts`). | closed |
| T-19-06 | Tampering (XSS) | formatter return values | low | mitigate | Every formatter returns a plain string from numbers/literals via `Intl`; rendered as JSX text/attribute, both escaped by React. | closed |
| T-19-07 | Denial of service | `formatDuration`/`formatPollInterval` with extreme integer | low | accept | Constant-time arithmetic over a bounded server-composed int; no loop or proportional allocation. | closed |
| T-19-SC (02) | Tampering | npm/pnpm installs | low | accept | Plan 19-02 runs no package install; native `Intl` used instead of a date library. | closed |
| T-19-SC (03) | Tampering | shadcn registry add of `table` | medium | mitigate | First-party shadcn official registry; single dependency `cn` already vendored; registry JSON curl-verified 2026-09-10 (`19-RESEARCH.md` Package Legitimacy Audit, verdict OK). Verified: `web/app/components/ui/table.tsx` imports only `React` and `cn`; no diff to `package.json`/`pnpm-lock.yaml`. | closed |
| T-19-08 | Tampering | `web/app/components/ui/table.tsx` content | low | mitigate | File committed as readable source, only imports `React` and `cn`. Verified directly. | closed |
| T-19-09 | Information disclosure | `web/app/app.css` theme tokens | low | accept | Two hex color literals; no secret, env var, or URL; already public in the committed SPA bundle. | closed |
| T-19-10 | Spoofing | `sourceDisplayName` unknown-key passthrough | low | accept | Unknown key renders verbatim as a panel title; key originates from compile-time `internal/pollruns` source identifiers, not user input; React escapes the text node. | closed |
| T-19-11 | Information disclosure | `SourcePanel` rendering `run.summary` verbatim (D-06) | medium | mitigate | `summary` composed server-side by `internal/pollruns` from integer counts + a closed three-value outcome set; contract declares no error/detail/message key in the success shape. Verified: `SourcePanel.tsx:64` renders `{last_run.summary}` as a plain JSX text node, no other free-text field. | closed |
| T-19-12 | Tampering (XSS) | every payload value rendered by `AboutInstance` and `SourcePanel` | medium | mitigate | Every value rendered as a JSX text node or escaped attribute; no `dangerouslySetInnerHTML` anywhere under `web/app` (repo-wide zero matches, re-asserted by the phase-gate grep). No payload field builds an `href`. | closed |
| T-19-13 | Tampering | unrecognised `outcome` from an N-1/N deploy | medium | mitigate | `classifyOutcome`'s mandatory `default` arm renders a grey badge with a title-cased fallback label. Verified: `OutcomeBadge.tsx:44` has a `default:` case; pinned by a dedicated test. | closed |
| T-19-14 | Information disclosure | a future `/status` field leaking an internal string | low | mitigate | View renders only the fields enumerated in the plan 19-01 wire types; no generic unknown-field/object-dump renderer exists. | closed |
| T-19-15 | Spoofing | a source key present in the payload but absent from `SOURCE_ORDER` | low | accept | Renders appended after known panels using its key as title; keys are compile-time `internal/pollruns` identifiers, not user input; React escapes the text. | closed |
| T-19-SC (04) | Tampering | npm/pnpm installs | low | accept | Plan 19-04 runs no package install; `web/package.json`/`web/pnpm-lock.yaml` untouched. | closed |
| T-19-16 | Information disclosure / session confusion | `handleRefresh` resolving after a session expiry | medium | mitigate | Handler carries its own mounted guard checked before every state write, plus an early return on session-expiry rejection — a second guard independent of the mount effect's cleanup flag. Pinned by the resolve-after-unmount test case. | closed |
| T-19-17 | Information disclosure | inline refresh-failure line and error state | low | mitigate | Both render fixed operator-authored copy; error object/message never interpolated. A session expiry renders neither surface. Verified alongside T-19-03. | closed |
| T-19-18 | Tampering (XSS) | table cells rendering `cycle_id`, counts, timestamps | low | mitigate | Every cell is a JSX text node or escaped attribute; `cycle_id` is a source name plus a server-composed integer. Phase-gate grep re-asserts the zero-match invariant. | closed |
| T-19-19 | Denial of service | recent-runs table at the buffer cap | low | accept | Server caps each source's history at fifty entries by contract; rendered row count bounded at one hundred across both sources regardless of uptime. | closed |
| T-19-20 | Denial of service / abuse | operator-driven refresh traffic | low | mitigate | No timer, listener, or automatic refetch; a grep gate enforces this. Verified: zero matches for `setInterval`/`setTimeout`/`visibilitychange`/`addEventListener` in `system.tsx`/`components/system`. Re-entrancy guard prevents a double-click issuing a third request. | closed |
| T-19-21 | Tampering | the committed `internal/webassets/build/client` bundle | low | mitigate | Tree regenerated by `make web` from audited source in this repo and committed as a reviewable diff, not fetched at build time; the gate asserts the rebuild actually produced a change. | closed |
| T-19-SC (05) | Tampering | npm/pnpm installs | low | accept | Only install is the frozen-lockfile install inside `make web`, resolving the already-committed lockfile; adds nothing new. Acceptance gate confirms no file outside the frontend and the bundle changed. | closed |

*Status: open · closed · open — below `high` threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above `workflow.security_block_on` (`high`) count toward `threats_open`*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

No threat in this phase reached the configured `high` block-on severity at plan time, and no
`mitigate`-disposition threat's claimed control was found absent on independent (L1 grep-depth)
re-verification at audit time. All threats above are closed.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-19-01 | T-19-SC (all 5 plans) | No package installs across the phase except a frozen-lockfile no-op inside `make web`; every plan's `19-RESEARCH.md` Package Legitimacy Audit records zero additions. | plan-time author | 2026-09-10 |
| AR-19-02 | T-19-07 | `formatDuration`/`formatPollInterval` are constant-time over a bounded server-composed int; a nonsensical value renders a harmless nonsensical string. | plan-time author | 2026-09-10 |
| AR-19-03 | T-19-09 | Theme color hex literals in `app.css` carry no secret and are already public in the committed SPA bundle. | plan-time author | 2026-09-10 |
| AR-19-04 | T-19-10, T-19-15 | Unknown source keys originate from compile-time `internal/pollruns` identifiers, not user input; rendering them verbatim (React-escaped) is preferable to silently dropping a source. | plan-time author | 2026-09-10 |
| AR-19-05 | T-19-19 | Recent-runs table row count is server-bounded at 50/source (100 total); pagination beyond the cap is a deferred follow-up, not a phase-19 requirement. | plan-time author | 2026-09-10 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-11 | 25 | 25 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth re-verification, ASVS level 1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-11
