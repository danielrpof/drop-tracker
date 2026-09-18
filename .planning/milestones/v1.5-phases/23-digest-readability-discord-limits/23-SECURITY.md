---
phase: "23"
slug: "digest-readability-discord-limits"
status: verified
# threats_open = count of OPEN threats at or above workflow.security_block_on severity (the blocking gate)
threats_open: 0
asvs_level: 1
created: "2026-09-18"
updated: "2026-09-18"
---

# Phase 23 — Security

> Per-phase security contract: threat register, accepted risks, and audit trail.

---

## Trust Boundaries

| Boundary | Description | Data Crossing |
|----------|-------------|---------------|
| Postgres `events` rows → Discord message body | `events.title`, `events.artist_name`, `events.watched_artist_name` originate from MusicBrainz/Deezer and are community-editable, semi-trusted text that crosses into a rendered Discord embed. | title, artist name, host credit |
| Postgres `events` rows → rendered chunk description | Community-editable artist names and titles flow through `digestLine` into chunk text this phase rearranges across message boundaries. | title, artist name |
| Chunk composition → Discord's 4096-rune Description ceiling | An over-budget Description is rejected with a 400, never acked, and retried forever with each attempt larger — a self-amplifying wedge, not a one-off failure. | chunk Description length |
| drop-tracker process → Discord webhook API | Outbound HTTPS POST carrying a secret-bearing URL; response status and body are attacker-influenceable by anything sitting on that path. | webhook URL, embed payload |
| Discord webhook API → drop-tracker process | Response status codes, headers, and bodies are attacker-influenceable by anything on that network path, including a misbehaving proxy or CDN error page. | HTTP status, response body |
| Scheduler tick → shared `notifying` CAS lock | The digest send holds the same lock `NotifyPending` takes (ADR-0002); time spent holding it is time real-time notification is dark. | lock hold duration |
| drop-tracker process → structured logs | An error value returned from `internal/discord` is logged by `internal/notifier`; anything embedded in it lands in log storage. The webhook URL's path is a live credential. | error strings, log fields |
| Process shutdown signal → in-flight digest | A deploy landing mid-digest races the drain deadline; what gets acked and what gets logged is decided here. | ack state, log output |

---

## Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation | Status |
|-----------|----------|-----------|----------|-------------|------------|--------|
| T-23-01 | Tampering | `lineLabel`/`digestLine` in `digest_format.go` | high | mitigate | `truncateRunes` applied *before* `escapeMarkdown` for both title (`digestTitleLimit`) and artist (`digestArtistLimit`) — an escape sequence can never be severed by the cap (`digest_format.go:153-156`) | closed |
| T-23-02 | Tampering | `digestWindowHeader` in `digest_chunk.go` | medium | mitigate | Composed only from a compile-time literal and `time.Time.Unix()`; never passed through `escapeMarkdown`; `TestDigestWindowHeader_NeverContainsBackslash` asserts no backslash in either branch's output (`digest_chunk.go:323-333`, `digest_chunk_test.go:40-46`) | closed |
| T-23-03 | Denial of Service | `chunkDigest`'s boundary loop | high | mitigate | An over-budget single line is emitted via the line-boundary fallback (`splitOversizedGroup`) rather than retried, so the loop provably terminates; `digestArtistLimit`/`digestTitleLimit` keep the practical worst case well under `chunkContentBudget` (`digest_chunk.go:177-181,219-290`) | closed |
| T-23-04 | Denial of Service | `SendDigestIfDue` chunk loop holding the `notifying` lock | high | mitigate | `ctx.Done()` observed at every chunk boundary so shutdown stops the loop between chunks, never mid-POST (`digest.go:161-167`); the chunk cap and whole-send budget (T-23-17) fully close the unbounded-worst-case tail | closed |
| T-23-05 | Repudiation | partial-digest failure path in `digest.go` | medium | mitigate | A chunk send failure logs a structured `logger.Error` naming chunk index, chunk count, sendable count, and a rate-limit discriminator before returning nil — never silent (`digest.go:188-195`) | closed |
| T-23-06 | Elevation of Privilege | `AckEventsOnly` query | medium | mitigate | Parameterised via sqlc-generated code (`sqlc.arg('ids')::bigint[]`), no string interpolation; `WHERE` clause can only narrow `events`, never touch `notification_settings` (`queries/notification_settings.sql:26-31`) | closed |
| T-23-07 | Information Disclosure | ack path under shutdown | low | accept | `context.WithoutCancel` deliberately lets an ack outlive cancellation so an accepted send always acks; accepted residual is an accepted-but-unacked duplicate on a rare timeout window — Phase 21's existing WR-03 posture, now one exposure window per chunk (ADR-0003) | closed |
| T-23-08 | Information Disclosure | 429-exhausted return in `sendAttempt` | high | mitigate | `ErrRateLimited` is a compile-time literal sentinel; the branch attaches only the integer status code. `TestSend_429Exhausted_ErrorNeverLeaksBodyOrToken` asserts neither the response body nor the webhook URL token segment appears in the returned error (`client.go:34-39,185`, `client_test.go:242`) | closed |
| T-23-09 | Information Disclosure | `httpClient.Do` error return (existing) | high | mitigate | Raw `*url.Error` never wrapped, preserved verbatim; `TestSend_TransportFailure_ErrorNeverLeaksHostOrToken` pins it so a future refactor cannot reintroduce the leak (`client_test.go:273`) | closed |
| T-23-10 | Denial of Service | `retry429Body` decoding and the `maxRetryAfter` clamp | medium | accept | Unchanged from the shipped implementation; the existing clamp already bounds a malformed or hostile `retry_after`. This phase adds no new parsing of attacker-controlled data | closed |
| T-23-11 | Spoofing | a proxy/CDN returning 429 on behalf of Discord | low | accept | A non-Discord intermediary forcing a 429 look-alike produces a correctly-handled partial digest that retries inside the grace window — same outcome as a genuine 429, no state advanced. Below ASVS L1 blocking threshold | closed |
| T-23-12 | Denial of Service | indicator stamping in `buildDigestChunks` | high | mitigate | `chunkOverheadReserve` (D-20) is sized *before* splitting and never re-measured after, so widening the indicator cannot change chunk count; `TestChunkInvariant1_EveryChunkAtMostDiscordLimit` asserts the 4096 ceiling over a 20+-chunk synthetic batch (`digest_chunk.go:30-46`, `digest_chunk_test.go:916`) | closed |
| T-23-13 | Denial of Service | group-preferred boundary loop in `chunkDigest` | high | mitigate | A group that doesn't fit an empty chunk is line-split into that chunk rather than deferred — the loop's own termination argument; pinned by the invariant property tests over a 700-event synthetic batch | closed |
| T-23-14 | Tampering | `continuationHeading`/`continuationNote` | medium | mitigate | Both render only compile-time literals plus a `digestHeadings`-sourced title — no event-derived text reaches either, confirmed the group `title` field is populated solely from the fixed `digestHeadings` set, never event data (`digest_chunk.go:113-128,301-321`) | closed |
| T-23-15 | Information Disclosure | synthetic test fixtures | low | mitigate | `syntheticEvents`/`insertPendingEventsForChunking` generate deterministic fabricated titles and ids in-test; no production data, credential, or webhook URL in any fixture; `gitleaks` pre-commit covers the staged test files (`digest_chunk_test.go:52`, `digest_test.go:150`) | closed |
| T-23-16 | Repudiation | content dropped without a marker | medium | mitigate | `(continued)` heading and trailing note make every mid-group cut visible; `TestChunkInvariant2_IDUnionExactlyOnce` proves no id is dropped from the uncapped output (`digest_chunk_test.go:926-935`) | closed |
| T-23-17 | Denial of Service | `SendDigestIfDue` holding the `notifying` lock | high | mitigate | `maxDigestChunks = 20` caps the burst and `digestSendBudget = 3 * time.Minute` caps the run, both stopping cleanly at a chunk boundary with the remainder pending instead of the ~14-minute unbounded worst case (`digest_chunk.go:53,77`) | closed |
| T-23-18 | Denial of Service | outbound burst through one Discord webhook | medium | mitigate | Inter-chunk spacing plus the 20-chunk cap keeps sustained outbound rate under the webhook ceiling; `sendAttempt` still honours one `Retry-After`, clamped by `maxRetryAfter` (`client.go:26-32,162-171`) | closed |
| T-23-19 | Tampering | ack decision on a capped or budgeted run | high | mitigate | The settings-advancing `AckDigestBatch` runs only on the final chunk with `deferred == 0`; every other case (including capped/budgeted stops) takes the narrower `ackEventsOnly` path. `TestSendDigestIfDue_Budget_ExceededAtBoundaryStopsPartialAckBothColumnsUnchanged` asserts both settings columns byte-identical to their pre-call snapshot after a capped/budgeted run (`digest.go:198-214`, `digest_test.go:566-628`) | closed |
| T-23-20 | Information Disclosure | new log fields at the chunk-failure site | medium | mitigate | Added fields are chunk index, counts, and an `errors.Is`-derived boolean discriminator — never the error's wrapped detail, response body, or webhook URL; the underlying error string's cleanliness is proven at the source by T-23-08/T-23-09 (`digest.go:188-195`) | closed |
| T-23-21 | Repudiation | shutdown drain outcome | medium | mitigate | The reworded drain line ("digest chunk loop stopped at a chunk boundary...") is emitted at Warn, visible at the default `LOG_LEVEL`, so a predictable correct outcome is never mislabelled a failure (`digest.go:167`) | closed |
| T-23-22 | Denial of Service | cancelling an in-flight POST via the budget | high | mitigate | Budget is checked at chunk boundaries only, never passed as the POST's context; `TestSendDigestIfDue_Budget_SendContextNeverCarriesBudgetDeadline` asserts the sender's context carries no budget-derived deadline (`digest.go:145,153`, `digest_test.go:677`) | closed |
| T-23-23 | Spoofing | an intermediary forging 429s to force partial digests | low | accept | Worst outcome is a correctly-handled partial digest that retries inside the grace window with nothing lost and no state advanced. Below the ASVS L1 blocking threshold | closed |
| T-23-SC (23-01) | Tampering | npm/pip/cargo installs | n/a | accept | No new Go module, npm package, or CLI tool added; `git diff --exit-code -- go.mod go.sum web/package.json` verified clean across the full 23-01..23-04 commit range | closed |
| T-23-SC (23-02) | Tampering | npm/pip/cargo installs | n/a | accept | Only `errors` from the standard library enters the import block; no dependency change | closed |
| T-23-SC (23-03) | Tampering | npm/pip/cargo installs | n/a | accept | Chunker uses only `strings`, `fmt`, `unicode/utf8`, and packages already imported by `digest_format.go`; no dependency change | closed |
| T-23-SC (23-04) | Tampering | npm/pip/cargo installs | n/a | accept | Clock seam is a package `var` over `time.Now`, not a third-party library; only new import is `errors` plus the already-present `internal/discord`; no dependency change | closed |

*Status: open · closed · open — below high threshold (non-blocking)*
*Severity: critical > high > medium > low — only open threats at or above workflow.security_block_on (high) count toward threats_open*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-23-01 | T-23-07 | `context.WithoutCancel` deliberately lets an ack outlive shutdown cancellation so an accepted send always acks; the alternative (letting shutdown cancel the ack) reintroduces a worse duplicate-send class. Phase 21's existing WR-03 posture, now scoped per chunk. | Daniel (plan-time) | 2026-09-18 |
| AR-23-02 | T-23-10 | `retry429Body`/`maxRetryAfter` clamp is unchanged, pre-existing behavior; this phase adds no new parsing of attacker-controlled data. | Daniel (plan-time) | 2026-09-18 |
| AR-23-03 | T-23-11, T-23-23 | An intermediary forging a 429 produces, at worst, a correctly-handled partial digest that retries inside the grace window — nothing lost, no state advanced. Below the ASVS L1 blocking threshold. | Daniel (plan-time) | 2026-09-18 |
| AR-23-04 | T-23-SC (23-01, 23-02, 23-03, 23-04) | No package-manager installs anywhere in Phase 23 — `go.mod`/`go.sum`/`web/package.json` untouched across the full commit range; the package-legitimacy gate does not apply. | Daniel (plan-time) | 2026-09-18 |

*No threat was blocked in this audit — all 23 numbered threats plus the four supply-chain checks verified closed at L1 grep-depth on the first pass.*

---

## Security Audit Trail

| Audit Date | Threats Total | Closed | Open | Run By |
|------------|---------------|--------|------|--------|
| 2026-09-18 | 27 | 27 | 0 | Orchestrator (Sonnet, ASVS L1, retroactive — SECURITY.md was not generated during phase execution; created at ship-time per `/gsd-secure-phase 23`'s State B path). Register built from all four plans' `<threat_model>` blocks (`register_authored_at_plan_time: true`); every mitigation claim verified directly against the shipped implementation and its test suite (`internal/notifier/digest.go`, `digest_chunk.go`, `digest_format.go`, `internal/discord/client.go`, `queries/notification_settings.sql`) rather than taken on the plans' own claims. `go build`/`go vet` clean; dependency-diff (`go.mod`/`go.sum`/`web/package.json`) confirmed unchanged across the full 23-01..23-04 range. |

---

## Sign-Off

- [x] All threats have a disposition (mitigate / accept / transfer)
- [x] Accepted risks documented in Accepted Risks Log
- [x] `threats_open: 0` confirmed
- [x] `status: verified` set in frontmatter

**Approval:** verified 2026-09-18 (retroactive — run at v1.5 ship-time via `/gsd-secure-phase 23`, closing the `ship:pre` security gate before the milestone PR).
