---
phase: "22"
slug: "scheduled-digest-send"
status: draft
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 1
asvs_level: 1
created: "2026-09-16"
---

# Phase 22 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| operator SPA → `PUT /settings/notifications` → Postgres | operator-controlled cadence/enabled values cross into scheduling state | digest_enabled, digest_cadence |
| container runtime → Go `time.LoadLocation` / host & image zoneinfo | the image's embedded or host tzdata determines every computed fire instant; Alpine ships no zoneinfo by default | IANA zone name → resolved `*time.Location` |
| app clock → Postgres clock | two independent clocks; the slot value must come from exactly one | slot instant written to `digest_last_slot_at` |
| Postgres `events` rows / MusicBrainz / Deezer catalogue text → Discord embed Description | community-editable artist names and titles cross into markdown-parsed text Discord renders as links and emphasis | artist name, title, host credit |
| drop-tracker process → Discord webhook | outbound HTTPS carrying the webhook path, which is itself the secret | webhook URL, embed payload |
| two in-process senders → one outbox | `NotifyPending` (real-time) and `SendDigestIfDue` (digest) contend for the same rows and the same lock | `events.notified_at`, digest watermark columns |
| operator SPA toggle → an in-flight send | digest mode can flip between the outbox read and the POST | digest_enabled |
| `events.external_id` → link href | the only id-namespace input a URL is built from | external_id → Discord embed link |
| CI runner → throwaway Postgres | a short-lived unauthenticated database on the runner's host network | fixed dev credentials, no prod data |
| CI runner → the built image under test | the artifact `build-scan` boots is the same one `release` later pushes, never a `ghcr.io/` pull | `drop-tracker:scan` local image |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-22-01 | Spoofing | `cmd/server` zone resolution | medium | mitigate | `time.LoadLocation(settings.ZoneName)` failure returns a wrapped error from `run()`; no fallback branch (`main.go:262-269`) | closed |
| T-22-02 | Tampering | `UpdateNotificationSettings` re-anchor CASE | medium | mitigate | CASE reads pre-UPDATE column values inside one statement; `WHERE id = 1` singleton write (`notification_settings.sql:14-24`) | closed |
| T-22-03 | Tampering | slot value source | medium | mitigate | `digest_last_slot_at` stores the Go-computed slot instant, never `now()` (`settings.go:136`) | closed |
| T-22-04 | Repudiation | `AckDigestBatch` watermark write | medium | mitigate | `digest_last_sent_at` written only via `COALESCE(nullable_param, digest_last_sent_at)` (`notification_settings.sql:47-48`) | closed |
| T-22-05 | Denial of Service | migration 000009 vs populated table | low | accept | Nullable `ADD COLUMN`, no `DEFAULT` — catalog-only change; single-row table | closed |
| T-22-06 | Elevation of Privilege | HTTP settings contract | low | mitigate | `updateSettingsRequest`/`settingsResponse` carry only the wire fields; `DisallowUnknownFields()`; new column server-owned (`settings.go:28-42`) | closed |
| T-22-07 | Tampering | digest Description markdown escaping | high | mitigate | `markdownEscaper` backslash-escapes Discord's markdown metacharacters before interpolation, backslash pair first (`digest_format.go:31-49,169-177`) | closed |
| T-22-08 | Denial of Service | `SendDigestIfDue` holding the notifying lock | high | mitigate | Every DB call bounded by `dbOpTimeout` via `readSettings`/`listUnnotified`/`ackDigestBatch` | closed |
| T-22-09 | Repudiation | post-2xx ack | high | mitigate | `ackDigestBatch` wraps `context.WithoutCancel(ctx)` so a shutdown between Discord's 2xx and the ack still records delivery (`digest.go:149`) | closed |
| T-22-10 | Repudiation | empty-skip watermark | medium | mitigate | Empty/suppressed-only branch acks with nil sent-at; SQL `COALESCE` leaves `digest_last_sent_at` untouched (`digest.go:86-97`) | closed |
| T-22-11 | Information Disclosure | send-failure logging | high | accept | `discord.Client.sendAttempt` refuses to wrap raw `*url.Error` (which embeds the webhook path); `digest.go` adds no new HTTP-error wrapping | closed |
| T-22-12 | Tampering | mode flip mid-send | medium | mitigate | Second settings read immediately before POST aborts with no writes if digest mode turned off after outbox read (`digest.go:102-112`) | closed |
| T-22-13 | Spoofing | unconfigured digest zone | medium | mitigate | `n.loc == nil` guard logs Warn and returns before any settings read (`digest.go:32-35`) | closed |
| T-22-14 | Spoofing | link href construction | high | mitigate | Every href built via `eventURL(ev)` from `external_id` through `url.PathEscape`; `Title`/`ArtistName` never reach a URL (`format.go:160-171`) | closed |
| T-22-15 | Denial of Service | Description length budget | **critical** *(reclassified from plan's self-rated medium)* | mitigate *(required — not accepted)* | **No mitigation present.** `buildDigestEmbed` caps only per-line title (100 runes); no cap exists on the whole `Embed.Description` against Discord's 4096-char limit. A failed oversized send (`digest.go:114-120`) returns without acking, so the same or a growing batch retries every 5 minutes until grace expires, then carries forward to the next slot in the same oversized state — a self-sustaining loop with no automatic recovery. The accepted rationale (D-19: Phases 21+22+23 ship in one release) is a process promise; `full-pipeline.yml`'s `release` job (`if: push && ref == main`) has no gate checking for Phase 23's chunking commits. | **open — BLOCKING** |
| T-22-16 | Tampering | emphasis bleeding into headings | medium | mitigate | `*`, `_`, `~`, `` ` `` all present in `markdownEscaper` (`digest_format.go:31-44`) | closed |
| T-22-17 | Repudiation | duplicate-line merging | low | accept | Deliberate design choice (D-26); two same-release events from two sources render as two lines, ack separately | closed |
| T-22-18 | Spoofing | shipped image zone resolution | high | mitigate | CI boot step polls `docker logs` for `"msg":"digest zone resolved"` + `"zone":"America/New_York"` (`full-pipeline.yml:629-653`) | closed |
| T-22-19 | Tampering | artifact under test | high | mitigate | Boots `drop-tracker:scan` (locally built image), no `ghcr.io/` reference in the new steps (`full-pipeline.yml:632-635`) | closed |
| T-22-20 | Information Disclosure | CI throwaway Postgres | low | accept | Same throwaway `drop_tracker`/`drop_tracker` credential pattern as existing `n1-boot` job; ephemeral runner, no prod data | closed |
| T-22-21 | Denial of Service | leaked CI containers | low | mitigate | `if: always()` cleanup step force-removes both containers, names distinct from `n1-boot`'s (`full-pipeline.yml:654-656`) | closed |
| T-22-22 | Information Disclosure | boot-log dump on failure | medium | mitigate | Failure path dumps `docker logs` from a container booted with only `DATABASE_URL`/`HTTP_PORT` set — `DISCORD_WEBHOOK_URL`/`INSTANCE_PASSPHRASE` deliberately absent (`full-pipeline.yml:625-635`) | closed |
| T-22-23 | Repudiation | grace-expired skip | medium | mitigate | Grace tests assert Warn count of exactly 1 and unchanged settings columns (`digest_schedule_test.go:518-529,589-598`) | closed |
| T-22-SC-01 (22-01) | Tampering | npm/pip/cargo installs | n/a | accept | No installs; `go.mod`/`go.sum` untouched | closed |
| T-22-SC-02 (22-02) | Tampering | npm/pip/cargo installs | n/a | accept | No installs; `go.mod`/`go.sum` untouched | closed |
| T-22-SC-03 (22-03) | Tampering | npm/pip/cargo installs | n/a | mitigate | `golang.org/x/text` promoted indirect→direct; `go.sum` unchanged (no new hash lines) | closed |
| T-22-SC-04 (22-04) | Tampering | npm/pip/cargo installs | n/a | accept | No installs, no new `uses:` action; `go.mod`, `go.sum`, action pin set untouched | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-22-01 | T-22-05 | Nullable `ADD COLUMN` with no `DEFAULT` is a catalog-only change in Postgres 11+; table is a single row regardless — no table rewrite, no long lock. | Daniel (plan-time) | 2026-09-16 |
| AR-22-02 | T-22-11 | `discord.Client` already refuses to wrap raw `*url.Error` (which embeds the full webhook path); this phase logs only `Sender.Send`'s error value and adds no new HTTP-error wrapping — existing control from T-05-01 carries forward. | Daniel (plan-time) | 2026-09-16 |
| AR-22-03 | T-22-17 | Not merging duplicate-source lines is the deliberate, transparent choice (D-26): an operator sees both detections rather than silently losing one. | Daniel (plan-time) | 2026-09-16 |
| AR-22-04 | T-22-20 | CI throwaway Postgres reuses the existing `n1-boot` job's dev-credential pattern on an ephemeral runner with no production data; accepted on the same terms as that existing job. | Daniel (plan-time) | 2026-09-16 |
| AR-22-05 | T-22-SC-01, T-22-SC-02, T-22-SC-04 | No package installs in plans 22-01/22-02/22-04 — `go.mod`/`go.sum` untouched, so no package-legitimacy checkpoint applies. | Daniel (plan-time) | 2026-09-16 |

*T-22-15 is explicitly NOT in this log — the user chose to block rather than accept it (2026-09-16). See Threat Register above.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-16 | 27 | 26 | 1 | gsd-security-auditor (Sonnet, ASVS L1, escalated to L2/L3 depth for T-22-15) |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [ ] `threats_open: 0` confirmed — **1 threat open (T-22-15), blocking by user decision**
- [ ] `status: verified` set in frontmatter — held at `draft` until T-22-15 closes

**Approval:** pending — re-run `/gsd-secure-phase 22` after T-22-15's interim guard (truncate-and-ack) or a CI/release gate enforcing D-19's Phase 23 sequencing lands.
