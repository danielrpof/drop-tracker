---
phase: quick/260916-wao
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [22-REVIEW-CR-01, 22-SECURITY-T-22-15]

files_modified:
  - internal/notifier/digest_format.go
  - internal/notifier/digest_format_test.go
  - internal/notifier/digest.go
  - internal/notifier/digest_test.go

estimate:
  tokens: 65000
  raw_tokens: 45000
  tasks: 2
  confidence: low

must_haves:
  truths:
    - "Every discord.Embed handed to Sender.Send by SendDigestIfDue has a Description of at most 4096 runes, regardless of how many sendable events a digest slot accumulates."
    - "When buildDigestEmbed truncates, it cuts on a full digest-line boundary (never mid-line) and appends a human-readable \"N more events\" note so the operator can see the digest is incomplete."
    - "Every event id in a digest batch -- both the ones whose line survived truncation and the ones dropped -- is acked via ackDigestBatch in the same call once Send succeeds, so digest_last_slot_at/digest_last_sent_at advance and a persistently oversized backlog cannot retry the same batch forever."
    - "For any digest whose full content already fits under 4096 runes, buildDigestEmbed's output is byte-identical to before this change: the 100-rune title cap, markdown escaping (T-22-07), fixed heading order/grouping, and collated sort are all unaffected."
  artifacts:
    - internal/notifier/digest_format.go
    - internal/notifier/digest_format_test.go
    - internal/notifier/digest.go
    - internal/notifier/digest_test.go
  key_links:
    - "buildDigestEmbed -> assembleDescription/truncationNote -> discord.Embed.Description (the new whole-limit enforcement point, T-22-15)"
    - "SendDigestIfDue -> ackDigestBatch -> sentIDs (already unconditional over the full `sendable` slice; this is the existing property that makes truncation safe once Send stops 400-ing)"
---

<objective>
Close T-22-15 (22-SECURITY.md, blocking) / CR-01 (22-REVIEW.md): `buildDigestEmbed` has no
cap on the whole `discord.Embed.Description`, only on each line's title. Once a digest
slot's accumulated events render past Discord's 4096-character limit, `Sender.Send`
returns a 400, `SendDigestIfDue`'s failure branch acks nothing, and the same (or a larger)
batch retries every 5 minutes until grace expires and then carries forward again --
a self-sustaining failure loop with no automatic recovery.

Purpose: add the interim safety valve the review sketched -- truncate the Description at
the last full line boundary at/under the limit, append a "... N more events" note, and
keep acking every event id in the batch (rendered or not) so the loop cannot start. This
is explicitly an interim guard, not DGST-12's full multi-message split, which stays
Phase 23's job.

Output: `internal/notifier/digest_format.go` enforces a whole-Description cap;
`internal/notifier/digest.go` gets a short comment documenting why its existing ack list
already covers truncated-out events; both gain test coverage proving the fix end to end.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.claude/CLAUDE.md
@.planning/STATE.md
@.planning/phases/22-scheduled-digest-send/22-REVIEW.md
@.planning/phases/22-scheduled-digest-send/22-SECURITY.md
@internal/notifier/digest.go
@internal/notifier/digest_format.go
@internal/notifier/format.go
@internal/notifier/digest_format_test.go
@internal/notifier/digest_test.go
@internal/notifier/notifier_test.go
</context>

<design_contract>

The shape below is settled for Task 1. Names and behavior are fixed; internal loop
mechanics are the executor's judgment call.

```go
// internal/notifier/digest_format.go

const discordDescriptionLimit = 4096   // Discord's documented Description ceiling
const truncationNoteReserve = 40       // runes withheld for the trailing note

// digestLine renders exactly one event's line -- extracted verbatim from
// buildDigestEmbed's existing per-item loop body ("- [label](url)" plus the
// deluxe track-count suffix). Output must not change from what is on disk today.
func digestLine(eventType string, ev sqlc.Event) string

// assembleDescription joins pre-rendered segments (one per event; a group's
// first segment also carries that group's leading separator + "**Heading**\n",
// exactly as buildDigestEmbed writes them today) into the final Description.
// When the full join already fits discordDescriptionLimit runes, the result is
// byte-identical to today's output -- no note, no truncation. Otherwise it
// keeps the largest whole-segment prefix whose rune count plus the resulting
// truncationNote's rune count stays within discordDescriptionLimit, and
// appends that note. Never cuts inside a segment.
func assembleDescription(segments []string) string

// truncationNote reports how many sendable events assembleDescription
// dropped. SendDigestIfDue's ack step does not consult Description content --
// every dropped event's id is still acked as delivered once Send succeeds --
// so this note exists only so the operator can see the digest is incomplete.
func truncationNote(omitted int) string
```

`buildDigestEmbed`'s own exported signature, `func buildDigestEmbed(events []sqlc.Event)
discord.Embed`, does NOT change -- the file's existing header comment already locks this,
and 15+ call sites across `digest_format_test.go` plus `digest.go`'s one call site depend
on it.

</design_contract>

<tasks>

<task type="tracer" tdd="true">
  <name>Task 1: Cap buildDigestEmbed's whole Description at Discord's 4096-rune limit</name>
  <files>internal/notifier/digest_format.go, internal/notifier/digest_format_test.go</files>
  <read_first>internal/notifier/digest_format.go, internal/notifier/format.go (truncateRunes), internal/notifier/digest_format_test.go</read_first>
  <behavior>
    - assembleDescription(segments) returns strings.Join(segments, "") unchanged, with no
      note, when the segments' combined rune count is <= 4096 -- this is what keeps every
      pre-existing digest_format_test.go assertion (16 test funcs) passing byte-for-byte.
    - assembleDescription(segments) whose combined rune count exceeds 4096 returns a
      result that (a) is itself <= 4096 runes, (b) consists of a prefix of whole segments
      -- never a partial segment/line -- followed by truncationNote's output, and (c) the
      note's reported count, added to the number of segments actually kept, equals
      len(segments).
    - truncationNote(1) reads as singular ("1 more event", not "events"); truncationNote(n)
      for n > 1 reads as plural.
    - buildDigestEmbed on a small ordinary batch (one event per heading, well under the
      limit) produces no truncation note -- pins the "unaffected when under budget"
      contract explicitly, on top of what the existing suite already implies.
    - buildDigestEmbed on a synthetic oversized batch (enough loop-generated events that
      concatenating every rendered line would exceed 4096 runes) yields a Description
      where utf8.RuneCountInString(...) <= 4096, the text contains "more event", and
      strings.Count(..., "- [") (rendered lines) plus the note's parsed-back count equals
      len(events).
  </behavior>
  <action>
Per the design_contract block above: extract the existing per-item line construction out
of buildDigestEmbed's inner loop into digestLine(eventType string, ev sqlc.Event) string
-- same "- [" + lineLabel(ev) + "](" + eventURL(ev) + ")" plus the deluxe-suffix branch,
byte-for-byte unchanged, ending in "\n".

Restructure buildDigestEmbed to build one segment string per event instead of writing
straight into a shared strings.Builder: for each digestHeadings group (unchanged sort via
sortDigestGroup), the group's first event's segment is prefixed with that group's leading
separator ("\n" only when this is not the very first segment overall) plus
"**"+h.title+"**\n" -- the same rule the current code expresses via `b.Len() > 0`, now
keyed off `len(segments) > 0`. Collect every segment into a []string in the existing fixed
group order, then return discord.Embed{Description: assembleDescription(segments)}.

Implement assembleDescription per its documented contract: sum each segment's
utf8.RuneCountInString; if the total is already <= discordDescriptionLimit, return
strings.Join(segments, "") immediately (this early return is what keeps every existing
test byte-identical). Otherwise find the largest whole-segment prefix whose rune count,
plus the rune count of truncationNote(len(segments)-prefixLen), stays within
discordDescriptionLimit -- a cumulative-rune-count prefix array scanned from the full
length downward is the simplest correct approach, since dropping a segment only reduces
the prefix's rune count while the note's length changes by at most a couple characters
per order of magnitude of the omitted count. Append the note directly after the joined
prefix (the last kept segment already ends in "\n", so no extra separator is needed).
truncationNoteReserve exists only as a generous (40-rune) sanity margin if useful for the
scan; it is not a hard second limit -- the real invariant is prefix-runes + note-runes <=
discordDescriptionLimit, checked exactly, not estimated.

Add truncationNote(omitted int) string returning the "... N more event(s)" text
(singular for omitted==1), using the same "..." ellipsis character convention as the rest
of this file's prose.

Update digest_format.go's file header comment and the digestTitleLimit comment with 1-2
lines noting the whole-Description cap is now actually enforced (closes T-22-15/CR-01)
rather than only aspirational -- keep additions to 1-3 lines per CLAUDE.md's comment
discipline; do not re-argue the fix's rationale inline, this PLAN and 22-SECURITY.md are
the source of truth.

Then add the two behavior-block test cases to digest_format_test.go: a small ordinary
batch asserting no truncation note appears, and an oversized synthetic batch (loop-built
sqlc.Event values, unique titles/external IDs, single event type is fine) asserting the
three properties in the behavior block above. Run the full pre-existing
digest_format_test.go suite alongside the new tests to confirm zero regressions.
  </action>
  <verify>
    <automated>go build ./... &amp;&amp; go vet ./... &amp;&amp; go test ./internal/notifier/ -run 'TestBuildDigestEmbed|TestDigestLine|TestAssembleDescription|TestTruncationNote|TestEscapeMarkdown|TestArtistKey|TestLineLabel|TestEventURL|TestDigestTitleLimit' -v</automated>
    <automated>golangci-lint run ./internal/notifier/...</automated>
  </verify>
  <done>buildDigestEmbed's exported signature is unchanged; every pre-existing digest_format_test.go assertion still passes; a new oversized-batch test proves the Description never exceeds 4096 runes, truncates only on a full line boundary, and reports an omitted count that exactly reconciles with the rendered line count.</done>
  <reversibility rating="reversible">Internal implementation of one already-tested pure function; a `git revert` restores the prior (unsafe) behavior with no external contract change.</reversibility>
</task>

<task type="auto" tdd="true">
  <name>Task 2: Prove SendDigestIfDue acks every event in an oversized batch</name>
  <files>internal/notifier/digest.go, internal/notifier/digest_test.go</files>
  <read_first>internal/notifier/digest.go, internal/notifier/digest_test.go (TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns for fixture conventions), internal/notifier/notifier_test.go (insertTestArtist/insertPendingEventTyped/isNotified)</read_first>
  <behavior>
    - SendDigestIfDue on a due slot whose sendable batch is large enough to trigger
      digest_format.go's truncation calls Sender.Send exactly once, with an embed whose
      Description is <= 4096 runes and contains a "more event" note (proving the fixture
      genuinely exceeded the limit, not merely that the code path was reachable).
    - Every event id inserted for that batch -- not only the ones whose line survived
      truncation -- has notified_at set (isNotified == true) after the call returns.
    - notification_settings.digest_last_slot_at and digest_last_sent_at both advance,
      matching the existing all-three-types test's assertions -- proving the backlog
      cannot retry: the slot record is what stops the next due-check from re-firing.
  </behavior>
  <action>
Add TestSendDigestIfDue_OversizedBatch_TruncatesDescriptionAndAcksEveryEvent to
digest_test.go, mirroring TestSendDigestIfDue_DueSlotAllThreeTypes_OneSendThreeAcksBothColumns's
structure: an isolated pool, one test artist via insertTestArtist, a fakeSender capturing
the sent embed, digestSettingsReader(true, settings.CadenceDaily, nil). Insert enough
pending new_release events under that one artist via a loop over insertPendingEventTyped
(unique external_id/title per iteration -- err generous, e.g. 80-100 events) that the
rendered digest would exceed 4096 runes before truncation. Do not hardcode an assumed
omitted count: assert the Description contains a truncation note as proof the fixture was
actually large enough, failing loudly if it is not, rather than silently under-sizing it.

Call SendDigestIfDue once, then assert: sender.calls == 1; utf8.RuneCountInString of the
captured embed's Description is <= 4096 (re-proves digest_format_test.go's guarantee
through the real send path, not only the pure function); the Description contains "more
event"; every one of the inserted event ids returns isNotified == true in a loop over the
full id slice -- this is the load-bearing assertion, since it is what proves T-22-15's
retry loop cannot happen (no truncated-out event is left pending); and
GetNotificationSettings shows both DigestLastSlotAt.Valid and DigestLastSentAt.Valid.

Add a 1-3 line comment in digest.go at the point sentIDs is built from sendable (ahead of
the ackDigestBatch call), noting that this list is the full sendable slice regardless of
what digest_format.go actually rendered into the Description -- so a
digest_format.go truncation never leaves an event stuck pending (closes T-22-15). Do not
otherwise change digest.go's logic: run the new test first against Task 1's changes with
zero digest.go edits -- SendDigestIfDue's ack step already acks every id in `sendable`
unconditionally once Send succeeds, and Task 1 is what stops Send from 400-ing on an
oversized payload in the first place, so the test is expected to pass with only the
comment added. If it does not, that reveals a genuine gap in the existing ack logic and
only then should digest.go's behavior itself change.

Then run the CLAUDE.md Definition of Done for this change: go vet ./..., golangci-lint
run, make test (after make db-up), make coverage-gate. Skip make sqlc-check -- no
migration or query file is touched by this fix. No web/ files change, so the frontend
prettier/vitest arm does not apply.
  </action>
  <verify>
    <automated>make db-up &amp;&amp; go test ./internal/notifier/ -run TestSendDigestIfDue -v</automated>
    <automated>go vet ./... &amp;&amp; golangci-lint run</automated>
    <automated>make test</automated>
    <automated>make coverage-gate</automated>
  </verify>
  <done>The new oversized-batch test passes against real Postgres and proves every event id in the batch is acked in exactly one send with no note-driven pending remainder; full go vet/golangci-lint/make test/make coverage-gate are clean; digest.go carries a short comment documenting why its ack list already covers truncated-out events.</done>
  <reversibility rating="reversible">Test-only addition plus a doc comment; no production logic changes in this task.</reversibility>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Postgres `events` accumulation -> Discord's per-request payload ceiling | an unbounded outbox rendered into one embed is the DoS surface T-22-15 names |
| Postgres `events` rows (community-editable artist/title text) -> Discord embed Description | unchanged from 22-SECURITY.md's existing boundary; this fix's note text is program-computed only, never interpolated event text |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-WAO-01 (closes T-22-15) | Denial of Service | `buildDigestEmbed` / `SendDigestIfDue` | critical | mitigate | Whole-Description cap (`discordDescriptionLimit`) with line-boundary truncation + note; `SendDigestIfDue`'s existing unconditional ack over the full `sendable` slice (unchanged) means a truncated batch is fully acked on the first successful send instead of retrying forever. |
| T-WAO-02 | Tampering | `truncationNote` output | low | mitigate | The note is built from a program-computed integer only (`fmt.Sprintf`-style formatting of a count) -- no community-editable text is interpolated into it, so it cannot retarget a link or inject markdown, consistent with T-22-07's existing escaping guarantee for the rest of the Description. |
| T-WAO-SC | Tampering | npm/pip/cargo installs | n/a | accept | No package installs; `go.mod`/`go.sum` untouched by this change. |

*Severity: critical > high > medium > low*
*Disposition: mitigate (implementation required) · accept (documented risk) · transfer (third-party)*

This plan closes 22-SECURITY.md's T-22-15 (currently `open — BLOCKING`, `threats_open: 1`).
Re-run `/gsd-secure-phase 22` after this lands to flip `22-SECURITY.md`'s frontmatter to
`threats_open: 0` / `status: verified` -- that re-verification is outside this plan's scope.
</threat_model>

<verification>
1. `go test ./internal/notifier/ -run 'TestBuildDigestEmbed|TestDigestLine|TestAssembleDescription|TestTruncationNote'` -- all green, including the new oversized-batch case.
2. `go test ./internal/notifier/ -run TestSendDigestIfDue -v` (real Postgres, `make db-up` first) -- all green, including the new `..._OversizedBatch_...` case.
3. Every pre-existing digest_format_test.go and digest_test.go assertion still passes unmodified.
4. `go vet ./...`, `golangci-lint run`, `make test`, `make coverage-gate` all clean.
5. `buildDigestEmbed`'s exported signature is byte-identical to before this change.
</verification>

<success_criteria>
- A digest whose accumulated events would render past 4096 runes now truncates at a full
  line boundary, carries a truncation note, and is accepted by Discord (no more 400 on
  send for this reason).
- Every event id in an oversized batch -- rendered or truncated-out -- is acked in the
  same successful send, so `digest_last_slot_at`/`digest_last_sent_at` advance and the
  backlog cannot retry the same batch forever.
- No regression to the 100-rune title cap, markdown escaping, heading grouping/ordering,
  or collated sort for any digest that already fit under the limit.
- Full Definition of Done (go vet, golangci-lint, make test, make coverage-gate) is clean.
</success_criteria>

<output>
Create `.planning/quick/260916-wao-add-an-interim-truncate-and-ack-guard-fo/260916-wao-SUMMARY.md` when done.
Record in it: the actual loop-count/fixture size used to trigger truncation in the new
tests, the measured coverage-gate percentage, whether the `-race` fallback was needed, and
an explicit note that `22-SECURITY.md`'s T-22-15 and `threats_open` count still need a
`/gsd-secure-phase 22` re-run to be marked closed in that document (out of this plan's
scope, but the next obvious follow-up).
</output>
