# Phase 23: Digest Readability & Discord Limits - Context

**Gathered:** 2026-09-17
**Status:** Ready for planning

<domain>
## Phase Boundary

Every digest message states the window it covers ("since <timestamp>"), and a digest large enough to exceed Discord's per-message limits is delivered as multiple ordered messages instead of silently truncating content — with Phase 22's type/artist grouping preserved across the split, and events acked per delivered message so a partial failure never loses or re-sends anything.

This is message composition and delivery inside `internal/notifier` and `internal/discord`. No SPA work — the operator-facing surface is the Discord message itself. The grouping hierarchy (by type, then watched artist) is already built in Phase 22 and stays untouched; this phase only adds a window header and a split.

</domain>

<decisions>
## Implementation Decisions

### Window Header (DGST-11)
- **D-01:** Timestamp renders as a Discord relative markup token (`<t:unix:R>`, e.g. "2 hours ago") — live-updating in the viewer's own locale, zero server-side date formatting/i18n needed.
- **D-02:** The window header is the first line of the embed's `Description`, above the grouped event content — inside the same 4096-rune budget `assembleDescription` already caps. **Reversibility:** costly — moving it to a Footer/Title later means adding a field to `discord.Embed` and re-deriving the Description rune budget math in `digest_format.go`, which today assumes the header shares Description's space.
- **D-03:** The very first digest ever sent (`digest_last_sent_at` is `NULL`) reads "since digest mode was enabled" instead of a timestamp — names the real reason there's no prior watermark rather than a generic "first digest" label.
- **D-04:** The window header repeats on every chunk message (not just the first), matching success criterion 1's literal wording ("every digest message states the window it covers") — each message is self-contained if messages arrive out of order or one goes missing.

### Chunking Strategy (DGST-12)
- **D-05:** Chunking stays one-embed-per-message, repeated across multiple sends — **not** packing multiple embeds into fewer messages. `discord.Client.Send` keeps its existing `(ctx, Embed) error` signature; no widening to `[]Embed`, no change to its one existing call site pattern used by both real-time `NotifyPending` and the digest path. **Reversibility:** costly to reverse — switching to multi-embed messages later touches `discord.Client.Send`'s public signature and every caller.
- **D-06:** A chunk boundary can only fall between two `digestLine` segments (whole-line only), never mid-line — the same invariant `assembleDescription` already enforces for its single-embed truncation case, now used to split rather than drop. This means a group (e.g. Guest Features) **can** span two messages; see Continuation Marker below.
- **D-07:** Each chunk message shows a visible "(N/Total)" position indicator. Since Total is only known once every chunk is built, the builder needs a two-pass shape (build all chunks first, then stamp N/Total into each) rather than emitting messages as it goes.
- **D-08:** A multi-message digest is framed as a rare/synthetic-scale edge case per ROADMAP's own gap #3 note — correctness under a large synthetic batch matters, throughput tuning for a routinely-huge watchlist does not. Don't over-build for scale this instance doesn't have yet.

### Continuation Marker
- **D-09:** A group that splits across a chunk boundary re-prints its heading with "(continued)" at the top of the message where it resumes (e.g. "**Guest Features (continued)**"), then continues that group's remaining lines.
- **D-10:** The marker appears at both ends of the split: a trailing note at the bottom of the cut-off message (e.g. "…Guest Features continues in message 2/3") **and** the re-printed "(continued)" heading at the top of the next one.

### Partial-Failure Retry Behavior
- **D-11:** `digest_last_slot_at` is written only after the **final** chunk of a multi-message digest sends successfully. If chunk 2 of 3 fails, the slot stays un-advanced, so the whole digest is still "due" and the scheduler's next check retries within the grace window — this is what makes success criterion 3 ("a failure partway through leaves the undelivered remainder pending for the next digest") true.
- **D-12:** A retry after partial failure is not a resume of the specific failed attempt — it's an ordinary `SendDigestIfDue` call that lists whatever's still un-acked (already-sent chunks' events are already acked and gone from the outbox) and rebuilds chunks fresh from that remaining set. No new state tracks "which chunk failed" — consistent with this feature's existing one-outbox-is-truth design (`docs/adr/0002-one-outbox-one-sender-lock.md`; no `digest_pending` flag, no second queue, per the v1.5 Roadmap's locked decision).
- **D-13:** Because retries rebuild fresh (D-12), the "(N/Total)" numbering (D-07) is recomputed from scratch each attempt — a retry with fewer remaining events may need fewer messages than the original failed attempt did.

### Claude's Discretion
- Exact Go shapes (a `chunk` struct, where the two-pass builder lives, helper/function names) are left to the planner — nothing here locks an implementation shape, only the observable behavior above.
- The exact wording of the trailing "continues in message N/Total" note (D-10) beyond its intent is left to the planner/executor to phrase naturally within the rune budget.

### Reviewed Todos (not folded)
Three pending todos matched Phase 23 by keyword coincidence (`todo.match-phase`, scores 0.2–0.6) but are unrelated tooling cleanup, not digest/Discord work — reviewed and explicitly not folded:
- `2026-09-05-resolve-d15-prev-release-query-files-from-prev-tag.md` — `cmd/migration-check` tooling, unrelated.
- `2026-09-05-unify-sqlscan-quote-state-machines.md` — `internal/sqlscan` cleanup, unrelated.
- `2026-09-05-move-shadcn-out-of-frontend-dependencies.md` — frontend dependency hygiene, unrelated.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` §"Phase 23: Digest Readability & Discord Limits" — goal, success criteria, and planner notes (Discord's named limits: 10 embeds/message, 25 fields/embed, ~6000-char total budget; grouping hierarchy already exists; chunk boundaries must not split a markdown-escaped line).
- `.planning/ROADMAP.md` §"Phase 22: Scheduled Digest Send" planner notes — the send sequence, ack shape, and slot/watermark distinction this phase builds on (CAS lock → read settings → list+partition outbox → build → re-check settings → send → ack).
- `.planning/REQUIREMENTS.md` — DGST-11 (window header), DGST-12 (no-truncation multi-message split).

### Design this phase builds on
- `.planning/phases/22-scheduled-digest-send/22-CONTEXT.md` (D-11–D-26) — the grilling-session source of truth for the slot/watermark/outbox three-role split, the send sequence, and the grouping hierarchy this phase must preserve across a split.
- `docs/adr/0002-one-outbox-one-sender-lock.md` — the single-outbox, single-`notifying`-lock design D-12 above depends on (no second queue, no `digest_pending` flag).

### No external specs beyond the above — requirements fully captured in decisions above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/notifier/digest.go` — `SendDigestIfDue`: the full send sequence (CAS lock, settings read/re-read, slot due-check, outbox list+partition, `ackDigestBatch`). This phase's split changes the "build embed → send → ack" middle of this function into "build chunks → send each → ack each"; the CAS lock, settings reads, and slot due-check stay as-is.
- `internal/notifier/digest_format.go` — `buildDigestEmbed`/`assembleDescription`/`digestLine`/`sortDigestGroup`/`digestHeadings`. `assembleDescription`'s whole-segment-boundary truncation logic is the direct ancestor of this phase's chunk-splitting logic (D-06) — same invariant, applied repeatedly instead of once with a drop.
- `internal/notifier/notifier.go` — `n.spacing` (`defaultSpacing = 400 * time.Millisecond`) is the existing inter-send pacing already used between real-time sends; ROADMAP's planner notes call for reusing this same spacing between a multi-message digest's chunk sends.
- `internal/notifier/digest.go:ackDigestBatch` — the existing single-CTE batch ack (`sqlc.AckDigestBatchParams{Slot, Ids, SentAt}`). Per-chunk acking (success criterion 3) calls this once per delivered chunk; D-11 means only the final successful call carries a `Slot`/`SentAt` write that actually advances state — earlier chunks in the same successful run still need their event ids acked so they don't re-send, but should not prematurely advance the slot if a later chunk in the same attempt could still fail. Worth flagging precisely to the planner: this is the one place D-11's "only the final chunk" rule and "ack per delivered message" (success criterion 3) intersect and need reconciling.
- `internal/discord/client.go` — `Embed{Title, Description, URL, Color, Fields, Thumbnail, Timestamp}`. `Title` and `Timestamp` (RFC3339, distinct from D-01's Discord relative-markup approach) already exist and are unused by the digest path today.

### Established Patterns
- `truncateRunes` (`internal/notifier/format.go`) — the shared rune-safe truncation helper used everywhere text is capped; not directly reused for chunk splitting (that's whole-line, not rune-count truncation) but the file's cut-on-a-rune-boundary discipline is the convention to match.
- `discord.Client.sendAttempt`'s single-funnel, no-body-echo, no-URL-wrapping conventions apply unchanged — this phase calls `Send` multiple times, it doesn't touch `sendAttempt`.

### Integration Points
- `internal/notifier/digest.go`'s `SendDigestIfDue` is the sole call site that needs to change from one `buildDigestEmbed` + one `n.sender.Send` + one `ackDigestBatch` into a loop. `internal/notifier/scheduler.go`'s `DigestScheduler` calls `SendDigestIfDue` unchanged — no scheduler-level change expected.

</code_context>

<specifics>
## Specific Ideas

- Window header wording for the first-ever digest: "since digest mode was enabled" (D-03) — specific phrase locked, not just "some explanatory text."
- Continuation heading format: "**Guest Features (continued)**" — same bold-heading style `digestHeadings` already uses, with the literal suffix "(continued)".
- Position indicator format: "(N/Total)" — parenthesized N-of-Total, exact placement (window header line vs. elsewhere) left to the planner.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. (The one item flagged above — reconciling per-chunk acking with "only the final chunk advances the slot" — is a design detail for the planner to resolve, not scope creep; captured under Reusable Assets since it points at exact code, not a new capability.)

</deferred>

---

*Phase: 23-digest-readability-discord-limits*
*Context gathered: 2026-09-17*
