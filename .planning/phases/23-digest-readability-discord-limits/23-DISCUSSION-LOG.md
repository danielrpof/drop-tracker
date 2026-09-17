# Phase 23: Digest Readability & Discord Limits - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-17
**Phase:** 23-digest-readability-discord-limits
**Areas discussed:** Window header wording, Chunking strategy, Continuation marker, Partial-failure retry behavior

---

## Window Header Wording

| Question | Options | Selected |
|---|---|---|
| Timestamp rendering | Discord relative `<t:unix:R>` / Absolute text in digest zone / Both | Discord relative `<t:unix:R>` ✓ |
| Placement | First line of Description / Embed Footer / Embed Title | First line of Description ✓ |
| First-ever digest (NULL last-sent) wording | "since digest mode was enabled" / "first digest" / Omit entirely | "since digest mode was enabled" ✓ |
| Repeats per chunk? | Every chunk message / First chunk only | Every chunk message ✓ |

**User's choice:** Relative Discord timestamp, first line of Description, explicit "since digest mode was enabled" for the first-ever case, repeated on every chunk.
**Notes:** "Every chunk message" ties directly to success criterion 1's literal wording that every digest message states its window.

---

## Chunking Strategy

| Question | Options | Selected |
|---|---|---|
| Packing approach | One embed per message (repeated sends) / Pack up to 10 embeds per message | One embed per message ✓ |
| Boundary rule | Whole-line/whole-segment only / Whole-group only | Whole-line/whole-segment only ✓ |
| Position indicator | Yes — "(N/Total)" / No indicator | Yes — "(N/Total)" ✓ |
| Realism framing | Rare edge case (ROADMAP's framing) / Plan for it being routine | Rare edge case ✓ |

**User's choice:** Keep `discord.Client.Send`'s existing one-embed signature (no interface change); allow a group to split mid-message rather than force whole-group boundaries; show "(N/Total)" per message; treat multi-message digests as a correctness-not-throughput concern.
**Notes:** Choosing whole-line boundaries (not whole-group) is what makes the Continuation Marker area necessary — a group can now legitimately span two messages. Choosing "(N/Total)" numbering means the builder needs a two-pass shape (all chunks built before Total is known).

---

## Continuation Marker

| Question | Options | Selected |
|---|---|---|
| Marker text | Repeat heading + "(continued)" / Plain note only | Repeat heading + "(continued)" ✓ |
| Placement | Both ends / Start of next message only | Both ends ✓ |

**User's choice:** A split group re-prints its heading with "(continued)" at the top of the resuming message, and the cut-off message gets a trailing note naming which message it continues in.

---

## Partial-Failure Retry Behavior

| Question | Options | Selected |
|---|---|---|
| Slot-record timing | Only after the final chunk succeeds / After every chunk that sends | Only after the final chunk succeeds ✓ |
| Retry shape | Rebuild fresh from remaining outbox / Track and resend the exact failed chunk | Rebuild fresh from remaining outbox ✓ |
| Numbering on retry | Recompute fresh each attempt / N/A (tied to retry-shape question) | Recompute fresh each attempt ✓ |

**User's choice:** Confirmed ROADMAP's own lean — the slot advances only once the whole digest (all chunks) has sent successfully, and a retry after partial failure is just an ordinary next scheduler check against whatever's still in the outbox, not a resume of a specific failed attempt.
**Notes:** This keeps the design consistent with the codebase's existing "outbox is the only source of truth" principle (ADR-0002) — no new persisted state for "which chunk failed."

---

## Claude's Discretion

- Exact Go implementation shapes (chunk struct, two-pass builder structure, helper names) — left to the planner.
- Precise phrasing of the trailing "continues in message N/Total" note beyond its stated intent — left to planner/executor within the rune budget.

## Deferred Ideas

None raised outside phase scope. One implementation nuance was flagged for the planner rather than deferred as a new capability: reconciling "ack per delivered chunk" (success criterion 3) with "the slot only advances on the final chunk" (D-11) — both events-acked and slot-advanced happen in the same `ackDigestBatch` call today, and a multi-chunk send needs to call it once per chunk while only the last call should carry the slot/sentAt write.
