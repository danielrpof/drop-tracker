# Phase 23: Digest Readability & Discord Limits - Pattern Map

**Mapped:** 2026-09-17
**Files analyzed:** 6 (2 modified, 1 new, 3 modified for ack/build-site changes)
**Analogs found:** 6 / 6 (all self-analogs — this phase mostly rewrites files that are their own best pattern source; `queries/notification_settings.sql`'s new query borrows `AckDigestBatch`'s own shape)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/notifier/digest_chunk.go` (new) | service/transform | batch/transform | `internal/notifier/digest_format.go` (`assembleDescription`, being deleted) | exact — direct ancestor, D-06/D-17 |
| `internal/notifier/digest_format.go` (modified: keep line/label/sort, drop `buildDigestEmbed`+`assembleDescription`) | service/transform | transform | itself (pre-phase version) | exact |
| `internal/notifier/digest.go` (modified: `SendDigestIfDue` send/ack loop) | service | request-response + event-driven (loop) | itself (pre-phase version) | exact |
| `internal/notifier/scheduler.go` | service (unchanged call site) | event-driven | itself — no change expected per CONTEXT | exact (no-op) |
| `internal/discord/client.go` (modified: sentinel 429-exhausted error) | service/client | request-response | itself (`sendAttempt`'s existing 429 branch) | exact |
| `queries/notification_settings.sql` (new `AckEventsOnly` query + generated sqlc) | model/migration-adjacent | CRUD | `AckDigestBatch` in the same file (lines 26-49) | exact — same file, same CTE shape, narrower |

## Pattern Assignments

### `internal/notifier/digest_chunk.go` (new — service/transform, batch)

**Analog:** `internal/notifier/digest_format.go`'s current `buildDigestEmbed` (lines 99-137) and `assembleDescription` (lines 173-198), which this file supersedes per D-17/D-19.

**Imports pattern** (from `digest_format.go` lines 16-27):
```go
import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	"github.com/danielrpof/drop-tracker/internal/db/sqlc"
	"github.com/danielrpof/drop-tracker/internal/discord"
)
```
`digest_chunk.go` will only need `strings`/`unicode/utf8`/`sqlc`/`discord` — grouping/sorting stays in `digest_format.go` and is called from here.

**Core pattern to copy — grouping loop** (`digest_format.go` lines 100-134): the `map[string][]sqlc.Event` grouping by `digestHeadings` fixed order, one `collate.Collator` per call (not package-level — concurrency note lines 105-108), `sortDigestGroup` per group, and the "leading `\n` only when not first" heading rule (lines 120-131) — D-17 flags this exact rule as needing correction once chunks exist (heading leads only when not first *in its chunk*, not not first overall).

**Core pattern to copy — budget-aware boundary logic** (`assembleDescription`, lines 173-198): the cumulative-rune-count array (`cum[]`) built once via `utf8.RuneCountInString`, then walked backward from the full set to find the largest prefix that fits `discordDescriptionLimit` — this is the direct ancestor of `chunkDigest`'s per-chunk boundary search (D-06/D-20), except:
- boundary must be `digestGroup`-preferred (D-06 amended), not `assembleDescription`'s "any segment"
- must carry `[]int64` ids alongside each `digestLine`/`digestEntry` (D-16), which `assembleDescription`'s `[]string segments` discards
- must reserve a named worst-case overhead constant (header + "(N/Total)" + trailing note) *before* splitting, not compute the note length after (D-20) — contrast with `assembleDescription`'s post-hoc `truncationNote(len(segments)-k)` call inside the loop (line 183), which is the exact fixed-point bug D-07/D-20 forbid reintroducing.

**Type shapes to introduce** (per D-16, no existing analog — new):
```go
type digestEntry struct {
	text string
	id   int64
}
type digestGroup struct {
	heading string
	lines   []digestEntry
}
type digestChunk struct {
	description string
	ids         []int64
}
```

**Constant-reserve pattern to copy** (`digest_format.go` lines 62-73 — `digestTitleLimit`, `discordDescriptionLimit`, `truncationNoteReserve`): declare `digestArtistLimit = 60` and the new worst-case overhead constant the same way — a named top-level `const`, each with a comment stating exactly what budget line it protects and why the number was chosen.

---

### `internal/notifier/digest_format.go` (modified — keep, trim, add `digestArtistLimit`)

**Analog:** itself.

**Keep unchanged:** `markdownEscaper`/`escapeMarkdown` (lines 36-54), `digestHeadings` (lines 82-86), `digestLine` (lines 143-162, but callers now attach `id`), `sortDigestGroup` (lines 219-230), `artistKey` (lines 239-244).

**Modify `lineLabel`** (lines 246-260) — add `digestArtistLimit = 60` cap (D-18) applied to both `watched` and `host` before escaping, mirroring how `title` is already capped on line 253:
```go
title := escapeMarkdown(truncateRunes(ev.Title, digestTitleLimit))
watched := escapeMarkdown(artistKey(ev))          // add truncateRunes(..., digestArtistLimit) here
...
host := escapeMarkdown(ev.ArtistName)             // add truncateRunes(..., digestArtistLimit) here
```
`truncateRunes` itself is `internal/notifier/format.go` lines 221-227 — shared helper, do not duplicate.

**Delete:** `buildDigestEmbed` (99-137), `assembleDescription` (173-198), `truncationNote` (200-210, "renamed to say so" per D-19 — keep only as the degraded-line floor, not the whole-Description truncator), `truncationNoteReserve`/`discordDescriptionLimit` move or stay depending on where `digest_chunk.go` needs them (co-locate the rune-budget constants with the chunker since that's now the sole consumer).

---

### `internal/notifier/digest.go` (modified — `SendDigestIfDue`)

**Analog:** itself, current version (lines 22-144).

**Structure to preserve verbatim:** CAS lock (lines 23-27), `n.loc` nil guard (32-35), settings read + fail-closed (37-44), slot due-check + grace window (46-65), `listUnnotified` + suppress partition (71-84), empty-sendable short-circuit using `ackDigestBatch(ctx, n.q, suppressedIDs, slot, nil)` (86-98) — **this exact branch is the "single-chunk / nothing-to-send" regression-safety path D-14 calls out; it should barely change**.

**Structure to rewrite** (lines 100-135, "build embed → re-check settings → send → ack" middle):
- replace `embed := buildDigestEmbed(sendable)` with `chunks := buildDigestChunks(sendable, cfg.DigestLastSentAt)`
- the settings re-check block (102-112) happens **once, before the first chunk only** (D-30) — keep its exact shape (read → fail-closed → `!cfg.DigestEnabled` bail) but move it to before the loop starts, not per-chunk
- turn the single `n.sender.Send` + single `ackDigestBatch` (114-135) into a loop: for each chunk but the last, `Send` then `AckEventsOnly` (new, narrow); on the final chunk, `Send` then `ackDigestBatch` (existing helper, unchanged shape) carrying `suppressedIDs` + `slot` + `sentAt` (D-14/D-15)
- a Send failure mid-loop must **not** call `ackDigestBatch` — leaves `digest_last_slot_at`/`digest_last_sent_at` untouched (D-11), matching the existing error-log-and-return-nil pattern already used at lines 114-119 for the single-embed case
- D-25/D-26: check wall-clock budget and `ctx.Done()` at each chunk-loop boundary — no existing analog in this file; nearest pattern is `NotifyPending`'s inter-send `select { case <-spacingWait(...): case <-ctx.Done(): return ctx.Err() }` (`notifier.go` lines 338-344), but D-23 uses a **digest-specific** 1s constant, not `n.spacing`/`spacingWait`

**`ackDigestBatch` helper stays as-is** (lines 152-164) — becomes the final-chunk-only ack. Copy its exact shape (`context.WithoutCancel` + `dbOpTimeout`, `sentAt *time.Time` nil-safe via generated COALESCE) for the new `ackEventsOnly` helper, minus the settings-row UPDATE.

**Error-handling pattern to copy** (lines 114-120, 132-135): `logger.Error` with structured fields then `return nil` (not an error) on a Send failure — a send failure is not fatal to the process, only to this digest attempt; contrast with `fmt.Errorf("notifier: ...: %w", err)` returned as a hard error only for DB failures (lines 72-74, 91-93, 133-135).

---

### `internal/notifier/scheduler.go`

No changes expected (`SendDigestIfDue`'s signature is unchanged: `(ctx, logger, now) error`). Confirms the call site at `scheduler.go` line 118 (`s.sink.SendDigestIfDue(s.runCtx, s.logger, s.now())`) needs no edit.

---

### `internal/discord/client.go` (modified — D-28 sentinel error)

**Analog:** itself, the existing 429-retry-exhausted path (lines 151-176).

**Pattern to copy:** `sendAttempt`'s existing branch structure — `if resp.StatusCode == http.StatusTooManyRequests && allowRetry` retries once (151-171); the fallthrough at the bottom (173-176) currently returns a generic `fmt.Errorf("discord: send webhook: unexpected status %d", resp.StatusCode)` for every non-204/non-retryable-429 case, indistinguishable from a 500. D-28 needs a **distinct sentinel** for "429 arrived on the retry attempt itself" (i.e., `resp.StatusCode == http.StatusTooManyRequests && !allowRetry`, which today falls through into the generic branch since `allowRetry` is only checked in the `&&` at line 151).

**Error convention to preserve:** no-body-echo (line 143 comment, line 173-175 comment) — the sentinel error must not include `resp.Body` content; and no wrapping of the raw URL error (lines 137-144) since the webhook path is a secret — these two conventions are load-bearing and must survive the new branch.

**Sentinel pattern (no existing analog in this file — closest Go-idiom in codebase):** declare as a package-level `var Err... = errors.New("...")` and use `errors.Is` at the call site per D-28's own wording. Search for existing sentinel-error conventions in the codebase:

---

### `queries/notification_settings.sql` (new `AckEventsOnly` query)

**Analog:** `AckDigestBatch` in the same file (lines 26-49) — D-14 explicitly narrows this into two queries.

**Pattern to copy exactly, minus the settings UPDATE:**
```sql
-- name: AckEventsOnly :exec
-- Phase 23 (D-14): acks a chunk's event ids only -- runs for every delivered
-- chunk except the last. Mirrors AckDigestBatch's idempotent predicate but
-- touches no notification_settings column; only the final chunk moves
-- instance state (see AckDigestBatch below). docs/adr/0003.
UPDATE events SET notified_at = now()
WHERE id = ANY(sqlc.arg('ids')::bigint[]) AND notified_at IS NULL;
```
Keep the `AND notified_at IS NULL` idempotence predicate verbatim (line 43's exact predicate) — this is the load-bearing invariant both queries share, called out explicitly in D-14.

**Placement:** Claude's Discretion (CONTEXT.md line 83) — beside `AckDigestBatch` in the same file is the lower-friction choice since both are read together by `digest.go` and the file is already small (49 lines); no existing multi-file query split precedent to break from.

**After editing:** run `sqlc generate` and commit the generated output under `internal/db/sqlc/` — `make sqlc-check` is local-only per D-14/CONTEXT's "Accepted Residual Risks", this phase's critical-path step.

## Shared Patterns

### DB call timeout + cancellation-detach
**Source:** `internal/notifier/digest.go` `ackDigestBatch` (lines 152-155) and `internal/notifier/notifier.go` `listUnnotified`/`markNotified`/`readSettings` (lines 190-213)
**Apply to:** the new `ackEventsOnly` helper — `context.WithTimeout(context.WithoutCancel(ctx), dbOpTimeout)` for the ack call (detach from shutdown so an accepted Discord send always acks), plain `context.WithTimeout(ctx, dbOpTimeout)` for reads.
```go
opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dbOpTimeout)
defer cancel()
```

### Fail-closed settings read
**Source:** `internal/notifier/notifier.go` `logSettingsReadFailure` (lines 219-226), called from both `digest.go` (37-41, 106-109) and `notifier.go` (265-269, 303-307)
**Apply to:** the single pre-first-chunk re-check in the rewritten `SendDigestIfDue` (D-30) — reuse this helper unchanged, do not write a new one.

### Rune-safe truncation
**Source:** `internal/notifier/format.go` `truncateRunes` (lines 221-227)
**Apply to:** `digestArtistLimit` capping in `lineLabel` (D-18) and any new chunk-header truncation — cut-on-rune-boundary discipline is the established convention (D-18/CONTEXT code_context section calls this out explicitly), not a new algorithm.

### No-body-echo / no-URL-wrap on Discord errors
**Source:** `internal/discord/client.go` lines 137-144, 173-175
**Apply to:** the new D-28 sentinel error path — must not attach `resp.Body` or the raw request URL to any returned/logged error.

### Structured slog field style
**Source:** throughout `digest.go`/`notifier.go` — e.g. `logger.Info("digest sent", slog.Int("sent_count", ...), slog.Int("suppressed_count", ...), slog.Time("slot", slot))` (digest.go lines 137-141)
**Apply to:** D-29's new chunk-count/pending-remainder fields on the "digest sent" summary line, and D-27's reworded drain-deadline log line — same `slog.Int`/`slog.String`/`slog.Time` key-value style, one line per outcome.

## No Analog Found

None — every file in scope is either a direct rewrite of its own pre-phase version, or (for the new `digest_chunk.go` and `AckEventsOnly` query) has a same-file/same-package direct ancestor explicitly named in CONTEXT.md's `code_context` section.

## Metadata

**Analog search scope:** `internal/notifier/`, `internal/discord/`, `queries/`
**Files read in full:** `internal/notifier/digest.go`, `internal/notifier/digest_format.go`, `internal/notifier/notifier.go`, `internal/notifier/scheduler.go`, `internal/discord/client.go`, `queries/notification_settings.sql`, `internal/notifier/format.go`
**Pattern extraction date:** 2026-09-17
