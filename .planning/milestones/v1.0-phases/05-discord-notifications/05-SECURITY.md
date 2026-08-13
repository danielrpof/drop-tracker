---
phase: 05
slug: discord-notifications
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: 2026-08-08
---

# Phase 05 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| drop-tracker process -> Discord API | Outbound HTTPS carrying a bearer-style secret embedded in the request URL path | Webhook URL/token, embed JSON payload |
| Postgres `events` rows -> Discord embed | Community-editable MusicBrainz/Deezer content (title, artist_name, release_type) crosses into a rendered message | Free-text release/artist metadata |
| Operator environment -> process config | `DISCORD_WEBHOOK_URL` is operator-supplied, never attacker-supplied (OPS-03) | Webhook URL/token via env var |
| `events.external_id` -> embed URL | A stored external id is interpolated into a link the user will click | External id (MBID / Deezer numeric id) |
| Two concurrent poll-cycle goroutines -> shared `events` table | Both `RunMusicBrainzCycle` and `RunDeezerCycle` can call `NotifyPending` around the same wall-clock moment | Shared pending-row dequeue state |
| Muted-preference row -> Discord channel | A muted artist's suppressed event must never become visible content in a channel other watchers can see | Suppressed event content |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-05-01 | Information Disclosure | `internal/discord/client.go` `sendAttempt` | high | mitigate | Transport-failure error is a fixed string (`"discord: send webhook: request failed"`) with no wrapped `*url.Error` cause, so the webhook token embedded in the URL path never reaches a log. Verified at `internal/discord/client.go:143`; covered by `client_test.go`'s leak-regression case. | closed |
| T-05-05 | Repudiation | `MarkNotified` ack in `queries/events.sql` | medium | mitigate | `UPDATE ... WHERE id = $1 AND notified_at IS NULL` makes the ack idempotent — a repeat ack of an already-delivered row affects zero rows. Verified at `queries/events.sql:67-72`. | closed |
| T-05-02 | Tampering | `internal/notifier/format.go` field population | medium | mitigate | Every community-sourced string is rune-truncated (`truncateRunes`) to Discord's documented limits (title 256, field value 1024) before marshaling. Verified at `internal/notifier/format.go:203`; covered by `format_test.go` multi-byte truncation cases. | closed |
| T-05-06 | Tampering | `internal/notifier/format.go` link construction | medium | mitigate | Embed URLs are built by escaping `external_id` into a fixed path template with `url.PathEscape`; `Title`/`ArtistName` are never interpolated into a URL. Verified at `internal/notifier/format.go:163-184`; covered by a URL-unsafe-external-id test. | closed |
| T-05-09 | Repudiation / Tampering | `internal/notifier/notifier.go` `notifying atomic.Bool` guard | medium | mitigate | CAS guard (`notifying.CompareAndSwap`) mirrors `poller.Poller`'s `mbRunning`/`dzRunning` idiom, releasing via defer. Proven under a genuine two-goroutine race by `TestNotifyPending_ConcurrentCallsNoDoublePost`. Verified at `internal/notifier/notifier.go:119` and `notifier_test.go`. | closed |
| T-05-10 | Denial of Service | `internal/notifier/notifier.go` per-row send loop | medium | mitigate | A per-event `Send` error is logged and the loop continues (falls through to the spacing wait rather than aborting); the row keeps `notified_at` NULL and the next pass re-selects it. Proven by `TestNotifyPending_BatchMidFailureContinuesToLaterRows`, which positively confirms the row after a mid-batch failure is both attempted and delivered. Verified at `internal/notifier/notifier.go:130-152` and `notifier_test.go:493`. | closed |
| T-05-12 | Information Disclosure | Muted event content reaching `internal/notifier/format.go`/Discord (NTFY-04) | medium | mitigate | Phase 4's D-17/D-18 filter runs before a row is ever inserted, so a muted event never reaches `ListUnnotified`. Proven end-to-end through a real `NotifyPending` call by `TestDetectMusicBrainz_GuestFeature_Muted_NeverDeliveredByNotifier`. Verified at `internal/detection/detector_test.go`. | closed |
| T-05-03 | Spoofing | `cmd/server/main.go` / `DISCORD_WEBHOOK_URL` | low | accept | Operator-supplied config (OPS-03), not attacker-controlled input. `http.NewRequestWithContext` rejects a malformed URL before any request is sent. | closed |
| T-05-04 | Denial of Service | `NotifyPending` pending-event backlog (D-09's retry-forever contract) | low | accept | Bounded in practice by `ListUnnotified`'s result-set size and D-07's per-send spacing. Residual cost is a longer cycle, not unbounded growth — mirrors Phase 4's accepted `AR-04-05`. | closed |
| T-05-SC | Tampering | npm/pip/cargo/go installs | high | accept | No package-manager install task in this phase — 05-RESEARCH.md's Package Legitimacy Audit records "not applicable — stdlib only, zero new `go.mod` entries." | closed |
| T-05-07 | Spoofing | Misleading embed link text | low | accept | A community-editable title next to a legitimate link can read misleadingly, but Discord embeds do not parse `@mentions` or auto-unfurl nested links — blast radius is a visually odd field. Accepted exactly as Phase 4 accepted the same class (`AR-04-01`/`AR-04-04`). | closed |
| T-05-08 | Information Disclosure | `release_type` normalization drift | low | accept | Storing the filter's own normalized value keeps stored data inside the `detectableReleaseTypes` vocabulary; drift renders as an unexpected label, never an authorization decision. | closed |
| T-05-11 | Information Disclosure | `internal/discord/client.go` `sendAttempt`'s 429 retry path, exercised repeatedly within one batch | low | accept | T-05-01's mitigation is per-call, not per-batch, so repeated exercise within one pass introduces no new leak surface. `TestNotifyPending_BatchHonorsRetryAfterWithoutDroppingOtherRows` exercises the path multiple times per pass without incident. | closed |

*Status: open · closed · open — below {block_on} threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-05-01 | T-05-03 | Webhook URL is operator-supplied config (OPS-03), not attacker input; malformed URLs are rejected before any request is sent. | plan 05-01 threat model | 2026-08-08 |
| AR-05-02 | T-05-04 | Retry-forever backlog is bounded in practice by per-send spacing and result-set size; mirrors Phase 4's `AR-04-05`. | plan 05-01 threat model | 2026-08-08 |
| AR-05-03 | T-05-SC | No package-manager installs in this phase — stdlib only, zero new go.mod entries. | plan 05-01 threat model | 2026-08-08 |
| AR-05-04 | T-05-07 | Discord embeds don't parse mentions or auto-unfurl nested links; blast radius is cosmetic. Mirrors Phase 4's `AR-04-01`/`AR-04-04`. | plan 05-02 threat model | 2026-08-08 |
| AR-05-05 | T-05-08 | `release_type` is display-only in this phase; drift produces a mislabeled field, never an authorization decision. | plan 05-02 threat model | 2026-08-08 |
| AR-05-06 | T-05-11 | 429-retry leak mitigation (T-05-01) is per-call; repeated exercise within a batch introduces no new surface, confirmed by batch-level test coverage. | plan 05-03 threat model | 2026-08-08 |

*Accepted risks do not resurface in future audit runs.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-08-08 | 13 | 13 | 0 | /gsd-secure-phase (orchestrator, L1 grep-depth verification; short-circuit per ASVS level 1 with plan-time threat register) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-08-08
