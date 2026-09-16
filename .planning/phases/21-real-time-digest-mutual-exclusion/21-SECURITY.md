---
phase: "21"
slug: "real-time-digest-mutual-exclusion"
status: verified
threats_open: 0
asvs_level: 1
created: "2026-09-16"
---

# Phase 21 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator (gated SPA) → `notification_settings` row → notifier control flow | The singleton row is a control input to delivery, written only through the passphrase-gated, CSRF-protected `PUT /settings/notifications` (Phase 20); this phase makes it decide whether Discord messages go out at all | Digest on/off + cadence, no new fields |
| notifier → Discord webhook | Pre-existing outbound boundary; no new request, header, or payload field added by this phase | Event embeds (unchanged shape) |
| process ↔ Postgres | Pre-existing; one more read per pass over the same pool, no new credential or connection | Settings row read, event rows read/updated |
| event row (`created_at`) → delivery-side freshness gate | New in 21-02: a DB-supplied timestamp now decides suppression, written only by `DEFAULT now()` on insert from the poller's own detection path — never attacker-influenced | `created_at` timestamp |
| browser ← server-embedded SPA | 21-03's new content is a compile-time string literal in the component, not data from the API | Static helper text, no API round-trip |
| build toolchain → committed `internal/webassets/build/client/` | A generated artifact is committed to the repo and later embedded into the shipped binary via `go:embed` | Built JS/HTML bundle |

No new attacker-facing surface across the phase: no new endpoint, no new user input, no new parser, no new dependency. The change is internal control flow in `internal/notifier` plus one static UI string.

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-21-01 | Tampering | `readSettings` failure path in `NotifyPending` | medium | mitigate | Fail closed (D-03): a read error stops the pass and returns nil without listing or sending. Verified: every error/early-return path in `NotifyPending` flows through `defer n.notifying.Store(false)`; no path lists or sends after a failed read. | closed |
| T-21-02 | Denial of Service | `readSettings` inside the `notifying` CAS-held region | medium | mitigate | Every settings read bounded by `dbOpTimeout` via `context.WithTimeout`, same shape as `listUnnotified`/`markNotified`. Verified: `internal/notifier/notifier.go` — `readSettings` wraps `r.Get` in `dbOpTimeout` at both call sites (top-of-pass, line ~246; mid-pass, line ~284). | closed |
| T-21-03 | Tampering | outbox drain concurrency (`Notifier.notifying`) | medium | mitigate | One outbox, one sender lock (ADR 0002): every outbox send serializes on the existing `notifying` CAS guard. Verified: no second lock or bypass path introduced; guard shape unchanged from pre-phase. | closed |
| T-21-04 | Information Disclosure | fail-closed Warn's `error` field | low | accept | Logged text is a pgx/sqlc error over a fixed singleton-row query with no user-supplied input; logs are operator-only. Consistent with the file's pre-existing WR-03 Warn. | closed |
| T-21-05 | Elevation of Privilege | control-input path to the gate | low | accept | Only writer of `digest_enabled` is Phase 20's `PUT /settings/notifications`, registered inside `registerDataRoutes` (gated group). Verified: `internal/httpserver/server.go` — settings routes registered alongside `/watchlist`, `/events`, `/status` inside `registerDataRoutes`, called once behind `gate.Authenticate` and once ungated only when no passphrase is configured (GATE-07 structural exemption, unchanged from Phase 14/20 posture). | closed |
| T-21-06 | Tampering | per-send re-read path (21-02) | medium | mitigate | Re-read placed between the suppression branch and `n.sender.Send`; fail-closed identical to top-of-pass. Verified: statement ordering in `notifier.go` confirms re-read precedes every `Send` call, never follows one in the same iteration. | closed |
| T-21-07 | Denial of Service | per-send read amplification (21-02) | low | mitigate | One settings read per fresh row, none per suppressed row (D-04); each read `dbOpTimeout`-bounded (T-21-02); sends already rate-limited to 1/400ms. Verified: suppression-ack branch continues without invoking `readSettings`. | closed |
| T-21-08 | Repudiation | mode-transition logging (21-02) | low | mitigate | `observeDigestMode` is the sole emitter, wired at exactly three call sites, fires only on change; a failed read never updates `lastDigestMode`. Verified: no other call site logs a transition line; failed-read path returns before touching `lastDigestMode`/`lastDigestModeSet`. | closed |
| T-21-09 | Tampering | `created_at`-anchored freshness cutoff (21-02) | medium | mitigate | Cutoff re-anchored to `ev.CreatedAt` + 1 day slack; must not re-open the backlog-flood regression from `.planning/debug/resolved/backlog-songs-trigger-discord.md`. Verified: `suppress_test.go`'s boundary-focused table test pins pre-gate backlog rows as still-suppressed (old relative to their own `created_at`); `staleReleaseDate`/shared truth table with `internal/detection` untouched. | closed |
| T-21-10 | Information Disclosure | `pending_count` in the transition log (21-02) | low | accept | Integer count of undelivered events in operator-only logs; reveals no artist/title/identifier. Same file already logs `suppressed_count`/`pending_count` per-pass. | closed |
| T-21-11 | Tampering | helper text rendering (21-03) | low | mitigate | Static JSX text (rendered as a single-line expression container), React auto-escapes, no interpolation, no settings value injected. Verified: `grep -rn "dangerouslySetInnerHTML\|innerHTML" web/app/components/system/DigestSettings.tsx` returns no matches. | closed |
| T-21-12 | Tampering | committed `internal/webassets/build/client/` tree (21-03) | medium | mitigate | Tree only produced by `make web`; task verifies new copy actually present in regenerated bundle; Phase 7's Docker image rebuilds SPA from source in its own Node stage, so the shipped container never trusts this committed tree. Verified: old-hash asset files cleanly renamed away, no orphans, new bundle's minified text contains the exact new helper string (confirmed by 21-REVIEW.md's independent check). | closed |
| T-21-13 | Spoofing | dependency resolution during the refresh (21-03) | medium | mitigate | `make web` uses `pnpm install --frozen-lockfile`. Verified: `Makefile` line 140 — `cd web && pnpm install --frozen-lockfile`; `web/pnpm-lock.yaml`/`web/package.json` unchanged in this phase's diff. | closed |
| T-21-14 | Information Disclosure | helper text content (21-03) | low | accept | Describes a product behavior already visible in the toggle; names no host, credential, schedule, or identifier. | closed |
| T-21-SC (21-01) | Tampering | npm/pip/cargo/go-module installs | high | mitigate | No package installed in 21-01; enforced by task-1 gate over `go.mod`/`go.sum`. Verified: `git diff --stat` across the phase's commit range shows zero changes to `go.mod`/`go.sum`. | closed |
| T-21-SC (21-02) | Tampering | npm/pip/cargo/go-module installs | high | mitigate | No package installed in 21-02; enforced by gates over `queries`, `internal/db/sqlc`, `internal/db/migrations`, `go.mod`/`go.sum`. Verified: zero diff across the phase's commit range for all four paths. | closed |
| T-21-SC (21-03) | Tampering | npm/pip/cargo installs | high | mitigate | No package installed in 21-03; enforced by gate over `web/pnpm-lock.yaml`/`web/package.json`. Verified: zero diff across the phase's commit range for both files. | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-21-01 | T-21-04 | Fail-closed Warn logs the raw pgx/sqlc error text over a fixed singleton-row query with no user-supplied input; operator-only logs; consistent with the file's pre-existing WR-03 precedent. | Phase 21 plan (21-01) | 2026-09-16 |
| AR-21-02 | T-21-05 | Only writer of the control input is the already-gated Phase 20 `PUT /settings/notifications` route; this phase adds no second writer and no override path. | Phase 21 plan (21-01) | 2026-09-16 |
| AR-21-03 | T-21-10 | `pending_count` is an integer with no identifying content, already logged elsewhere in the same file. | Phase 21 plan (21-02) | 2026-09-16 |
| AR-21-04 | T-21-14 | Helper text describes visible product behavior; no host, credential, schedule, or identifier disclosed. | Phase 21 plan (21-03) | 2026-09-16 |

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-16 | 17 | 17 | 0 | gsd-secure-phase orchestrator (L1 grep-depth, short-circuit path — register authored at plan time, ASVS L1) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-16
