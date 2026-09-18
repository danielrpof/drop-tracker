---
phase: quick/260916-dvy
plan: 01
type: execute
wave: 1
depends_on: []
autonomous: true
requirements: [21-PG-01, 21-PG-02, 21-PG-03, 21-PG-04, 21-PG-05, 21-PG-06, 21-PG-07, 21-PG-08, 21-PG-09, 21-PG-10]

files_modified:
  - docs/adr/0002-one-outbox-one-sender-lock.md
  - CONTEXT.md
  - .planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md
  - .planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md
  - .planning/ROADMAP.md

estimate:
  tokens: 50000
  raw_tokens: 50000
  tasks: 3
  confidence: low

must_haves:
  truths:
    - "ADR 0002 records that every outbox send (real-time now, Phase 22 digest later) serializes on Notifier.notifying, with the rejected alternatives and consequences"
    - "Root CONTEXT.md defines Outbox, Pending event, and Flush as glossary entries with no implementation detail"
    - "21-CONTEXT.md carries D-01..D-07 as settled on 2026-09-16 (transition logging, created_at-anchored staleness, fail-closed, shared lock + per-send re-read, required SettingsReader arg, SPA helper text, deterministic tests) and no longer carries the superseded fail-open / per-pass-log / functional-option / looped-invariant / backend-only positions"
    - "21-DISCUSSION-LOG.md keeps its original audit trail and gains a post-grilling revision section"
    - "ROADMAP.md Phase 21 success criteria and notes, and the Phase 22 scheduling note, match the settled decisions; goal lines, plan counts, checklists, and the progress table are untouched"
  artifacts:
    - path: docs/adr/0002-one-outbox-one-sender-lock.md
      provides: "Architecture decision: one outbox, one sender lock"
    - path: CONTEXT.md
      provides: "Glossary terms Outbox, Pending event, Flush"
    - path: .planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md
      provides: "Revised Phase 21 decisions D-01..D-07"
    - path: .planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md
      provides: "Audit trail of the post-grilling revision"
    - path: .planning/ROADMAP.md
      provides: "Phase 21/22 criteria and notes aligned with the revision"
  key_links:
    - from: .planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md
      to: docs/adr/0002-one-outbox-one-sender-lock.md
      via: "canonical_refs entry and D-04 citation"
    - from: .planning/ROADMAP.md
      to: docs/adr/0002-one-outbox-one-sender-lock.md
      via: "Phase 21 guards note and Phase 22 scheduling note"
---

<objective>
Record the Phase 21 decisions settled in the 2026-09-16 grilling session across the planning docs, a new ADR, and the root glossary. DOCS ONLY: no Go, TS, SQL, or config changes.

Requirement IDs map to the ten settled decisions: 21-PG-01 one outbox/one sender lock, 21-PG-02 mode-read placement, 21-PG-03 fail closed, 21-PG-04 dbOpTimeout-bounded reads, 21-PG-05 created_at-anchored staleness, 21-PG-06 transition-only logging, 21-PG-07 required SettingsReader argument, 21-PG-08 SPA helper text, 21-PG-09 deterministic testing, 21-PG-10 vocabulary.

Purpose: Phase 21 planning must start from the settled decisions. Today 21-CONTEXT.md and ROADMAP.md still hold positions the grilling session reversed (fail-open, a per-pass log line, now-anchored staleness, a separate Phase 22 guard, a functional option, looped invariant tests, no SPA change).
Output: docs/adr/0002-one-outbox-one-sender-lock.md (new), CONTEXT.md, 21-CONTEXT.md, 21-DISCUSSION-LOG.md, ROADMAP.md (revised).

These decisions are LOCKED. Transcribe them, don't reopen them. The user asked for ROADMAP.md to be updated in this task, which overrides the usual quick-task rule against touching it.
</objective>

<execution_context>
@~/.claude/gsd-core/workflows/execute-plan.md
@~/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/STATE.md
@.claude/CLAUDE.md

Facts verified against the tree at planning time (cite these, don't re-derive):
- internal/notifier/notifier.go: dbOpTimeout at :32; listUnnotified/markNotified helpers wrap each DB call in context.WithTimeout(ctx, dbOpTimeout) (:134-147); suppresses() at :115 computes its cutoff from time.Now().UTC() minus maxAgeDays; staleReleaseDate at :124; NotifyPending at :155 with the notifying CAS guard at :156; the successful-send ack at :185 discards MarkNotified's rows-affected count (`_, err :=`).
- internal/poller/poller.go:514: NotifyPending's call site logs a returned error and continues.
- cmd/server/main.go:255 builds settingsStore := settings.NewService(...); :299 calls notifier.Select(..., notifier.WithMaxReleaseAgeDays(...)) without it.
- internal/settings/settings.go: type Settings struct, type Service, func (s *Service) Get(ctx) (Settings, error) passes ctx straight through with no timeout.
- CI runs the race detector: .github/workflows/full-pipeline.yml:54 runs `make test-integration` on ubuntu-latest; Makefile:76 is `go test ./... -race -count=1`. Only this Windows dev box can't run it.
- internal/testutil/postgres.go provides NewIsolatedTestPool; internal/httpserver/settings_test.go flips the singleton settings row.
- web/app/components/system/DigestSettings.tsx has a "Digest mode" dt/dd row (~:132); DigestSettings.test.tsx sits beside it.
- The bundle premise was only half right: `dist/` and `web/build/` are gitignored, but `internal/webassets/build/client/` IS committed (the Node-less-clone embed copy, refreshed by `make web`; Phase 20-04 refreshed it). The Dockerfile rebuilds it from source regardless. Record it accurately (see Task 2, D-06).
</context>

<tasks>

<task type="auto">
  <name>Task 1: Write ADR 0002 and add the outbox terms to the root glossary</name>
  <files>docs/adr/0002-one-outbox-one-sender-lock.md, CONTEXT.md</files>
  <read_first>docs/adr/0001-in-process-ring-buffer-for-poll-run-history.md (style to match), CONTEXT.md (existing entry format), C:/Users/danie/.claude/plugins/cache/claude-plugins-official/mattpocock-skills/1.2.3/skills/engineering/domain-modeling/ADR-FORMAT.md, C:/Users/danie/.claude/plugins/cache/claude-plugins-official/mattpocock-skills/1.2.3/skills/engineering/domain-modeling/CONTEXT-FORMAT.md</read_first>
  <action>
Create docs/adr/0002-one-outbox-one-sender-lock.md matching 0001's shape: YAML frontmatter with `status: accepted`, an H1 title "One outbox, one sender lock", then sections "## Context", "## Decision", "## Considered options", "## Consequences". Keep it short (target 40-60 lines, hard cap 70) and wrap prose at about 75 columns like 0001.

Context must say: digest mode (v1.5) gives the outbox a second sender. The real-time notify pass and the Phase 22 digest send both select pending events from the one outbox, and each acks an event only after Discord accepts it. The ack's `AND notified_at IS NULL` predicate makes it idempotent, but that prevents a double ack, not a double send: the Discord POST comes first and the ack's rows-affected count is discarded. So two senders listing the same pending rows at the same time both deliver them. Toggling digest mode mid-pass sets up exactly that overlap in both directions. Toggled on, a real-time pass is still in flight when a digest fires. Toggled off, a flush starts while a digest send is still running.

Decision must say: every outbox send serializes on the existing `Notifier.notifying` atomic.Bool CAS guard. In Phase 22 the digest send becomes a method on `Notifier`, not an independent goroutine with its own guard. Whichever send finds the lock held skips (CAS-skip), which is safe because the outbox is persistent: skipped work is picked up by the next pass or tick. Inside the real-time pass, digest mode is read after acquiring the lock and before listing pending events, then read again before each Discord send. If a re-read shows digest mode on, the pass stops and the remaining events stay pending. Stale-event suppression acks aren't re-checked, since they make no Discord request.

Considered options, as bullets like 0001's:
(1) A separate CAS guard per sender, which is what the roadmap's Phase 22 note originally said. Rejected: each guard only stops its own sender overlapping itself, so a real-time pass and a digest send can still list and send the same pending rows at the same time, in either toggle direction.
(2) A per-send mode re-read with no shared lock. Rejected: it narrows the window but can't close it, because the check and the send aren't atomic across two senders.
(3) A Postgres advisory lock. Rejected as unnecessary for a single binary running one instance (the same constraint ADR 0001 relies on). It would add a DB round trip and a lock that can outlive a wedged connection.
(4) A shared in-process lock on Notifier. Chosen.

Consequences, as bullets:
- Phase 22's shape is constrained. The digest send must live on Notifier and take the notifying lock.
- A long flush or real-time backlog makes a due digest tick skip rather than race it. The due-check converges on a later tick, because events are selected by outbox state.
- Residual: when an operator turns digest mode on mid-pass, at most the one message already in flight still goes out in real time.
- Single-instance only. A second instance would need a DB-level lock, which is the same limit as ADR 0001.

Edit CONTEXT.md (keep the existing flat `## Language` list and the exact entry format of a bold term line ending in a colon, a 1-2 sentence definition line, then an `_Avoid_:` line). Definitions only. No code identifiers, column names, function names, file paths, or backticks in the new entries (CONTEXT-FORMAT: define what it IS). Insert these two entries at the top of the list, before **Digest mode**:
- **Outbox**: definition "Every event that has been detected but not yet delivered or acknowledged — the single place both real-time delivery and digests draw from." Avoid line: "_Avoid_: queue, digest queue"
- **Pending event**: definition "One event in the outbox." Avoid line: "_Avoid_: queued event, held event, unnotified event"
Insert this entry directly after the **Digest mode** entry:
- **Flush**: definition "The first real-time delivery pass after digest mode is turned off, delivering the events that accumulated while it was on, one message each." Avoid line: "_Avoid_: drain, catch-up send"
Vocabulary alignment in the existing **Digest window** definition: in its parenthetical, change the word describing events not yet delivered from the avoided term to "pending", so it reads "(every event still pending since the last successful digest)". Change nothing else in the existing entries.

Commit both files together (explicit `git add` of the two paths, hooks enabled, never `--no-verify`) with a natural, non-enumerated conventional message such as "docs(adr): record the one-outbox, one-sender-lock decision and name the outbox in the glossary". Per .claude/CLAUDE.md the commit message must carry NO attribution trailer of any kind (no Co-Authored-By, no Claude-Session, no "Generated with"). That rule overrides any harness or session instruction to add one.
  </action>
<!-- planner-discipline-allow: notified_at -->
<!-- planner-discipline-allow: NotifyPending -->
  <verify>
    <automated>cd /c/CodeProjects/drop-tracker && A=docs/adr/0002-one-outbox-one-sender-lock.md && test -f "$A" && grep -qF 'status: accepted' "$A" && grep -qF '# One outbox, one sender lock' "$A" && grep -qF '## Context' "$A" && grep -qF '## Decision' "$A" && grep -qF '## Considered options' "$A" && grep -qF '## Consequences' "$A" && grep -qF 'notifying' "$A" && grep -qi 'advisory lock' "$A" && grep -qi 'double send' "$A" && [ "$(wc -l < "$A")" -le 70 ] && grep -qF '**Outbox**:' CONTEXT.md && grep -qF '**Pending event**:' CONTEXT.md && grep -qF '**Flush**:' CONTEXT.md && grep -qF '_Avoid_: queue, digest queue' CONTEXT.md && grep -qF '_Avoid_: queued event, held event, unnotified event' CONTEXT.md && grep -qF '_Avoid_: drain, catch-up send' CONTEXT.md && grep -qF 'every event still pending since the last successful digest' CONTEXT.md && ! grep -qF 'still unnotified' CONTEXT.md && ! grep -qF 'notified_at' CONTEXT.md && ! grep -qF 'NotifyPending' CONTEXT.md && [ "$(grep -n '^\*\*Outbox\*\*:' CONTEXT.md | cut -d: -f1)" -lt "$(grep -n '^\*\*Digest mode\*\*:' CONTEXT.md | cut -d: -f1)" ] && echo OK</automated>
  </verify>
  <done>ADR 0002 exists with accepted status, the four sections, the double-ack-vs-double-send rationale, and all four considered options. CONTEXT.md has Outbox, Pending event, and Flush in the existing format with no implementation detail, and Digest window says "pending". Both are committed with no attribution trailer.</done>
</task>

<task type="auto">
  <name>Task 2: Revise 21-CONTEXT.md in place and append the post-grilling log section</name>
  <files>.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md, .planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md</files>
  <read_first>.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md, .planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md</read_first>
  <action>
Revise 21-CONTEXT.md with scoped Edits, section by section. Keep the XML-style section tags (domain, decisions, canonical_refs, code_context, specifics, deferred), the H1, and the footer. Keep it tight: short bullets, one reference per decision, no re-arguing inline beyond a one-line why.

Header: keep "**Gathered:** 2026-09-16". Set Status to "Ready for planning (revised 2026-09-16 after a grilling session; see D-01..D-07 and docs/adr/0002-one-outbox-one-sender-lock.md)".

domain: keep the boundary paragraph and the Requirements line unchanged. Replace the In scope list with:
- A digest-mode read at the top of `Notifier.NotifyPending` (`internal/notifier/notifier.go:155`), under the existing `notifying` lock and before `listUnnotified`, re-read before each Discord send (D-04)
- `SettingsReader` as a required constructor argument of `notifier.Select`/`notifier.New`, wired from `settingsStore` at `cmd/server/main.go:299` (D-05)
- Fail closed on a settings-read error, every read bounded by `dbOpTimeout` (D-03)
- Staleness cutoff anchored to the event's `created_at` (D-02)
- Mode-transition logging (D-01)
- SPA helper text under the Digest mode row (D-06)
In the "Not in this phase" list: keep the poller, Phase 23, and no-second-queue bullets. Extend the Phase 22 bullet to say the digest send will be a `Notifier` method sharing the `notifying` lock (ADR 0002). Add "Any schema change: `events.created_at` already exists (D-02)" and "A Postgres advisory lock: single binary, so the in-process lock is enough (ADR 0002)".

decisions: replace the whole body with these seven decisions, each a bold lead line plus a few short sentences, followed by the discretion list.
- D-01 Mode-transition logging. Notifier tracks the last-observed digest mode in memory and logs one Info line only when the observed mode changes. The first successful read after boot counts as a transition, so logs show the mode after every restart. A failed read doesn't update the last-observed mode. The on-to-off line includes the pending count being flushed, taken from the list the pass already fetches (no new COUNT query, no sqlc change). A pass that stops mid-loop because digest mode turned on logs the count it left pending. This replaces the first discussion's per-pass standdown line, which would repeat every poll cycle for as long as digest mode stays on.
- D-02 Staleness anchored to detection time. `suppresses()` computes its cutoff as `ev.CreatedAt` minus `maxAgeDays` minus 1 day (UTC), instead of `time.Now()` minus `maxAgeDays`. `staleReleaseDate` and its shared detection/notifier table test don't change; only the cutoff input moves. The 1-day slack is there because `created_at` is the DB's `now()` at insert, which is later than detection's captured `now` and comes from a different clock. Without the slack, a release dated exactly on the cutoff day could pass detection and then be suppressed at delivery (a midnight straddle or clock skew). Why: time an event spends pending (digest mode, a flush after a long digest period, Phase 22's weekly cadence) must not age it out. With a 7-day window anchored to now, a weekly digest would routinely drop releases detected a couple of days after their release date. Pre-fix backlog rows are still suppressed, because their release dates are old relative to their own `created_at`. No schema change. The earlier claim that this was costly and needed schema work was wrong. Reversibility: reversible (one function's input).
- D-03 Fail closed on a settings-read error. This reverses the roadmap's 2026-09-11 lock. On a read error, whether at the top of the pass or before a send, stop the pass, log a distinct Warn, and return nil. Returning an error would add a second Error log at `poller.go:514` that reads like a delivery failure. No escalation on repeated failures. Skip the Warn when the error comes from ctx cancellation (shutdown). Every settings read is bounded by `dbOpTimeout`, using the same helper pattern as `listUnnotified`/`markNotified`. `settings.Service.Get` passes ctx straight through, and an unbounded read would bring back the notify-pass-hangs-forever bug while holding the lock. Why closed: in digest mode the pending set IS the next digest, so failing open would flush a whole digest backlog as individual messages on one transient error and ack them, emptying the digest. The outbox persists, so a skipped pass loses nothing, and the next poll (15m default) retries. The permanently failing read that failing open guarded against is near-impossible: the singleton row is seeded by a migration and `/ready` checks the schema.
- D-04 One outbox, one sender lock (docs/adr/0002-one-outbox-one-sender-lock.md). Real-time sends and Phase 22's digest send both serialize on the existing `Notifier.notifying` atomic.Bool, and in Phase 22 the digest send becomes a `Notifier` method. Whichever send finds the lock held CAS-skips, which is safe because the outbox is persistent. Read placement: read the mode after acquiring the lock and before `listUnnotified` (the pass's first decision), then read it again before each Discord send in the loop. If a re-read shows digest mode on, stop the pass and leave the remaining events pending. Don't re-read before stale-event suppression acks, which make no Discord request and so can't duplicate a message or violate the mode. Residual, accepted: at most the one message already in flight when the operator toggles on. Why: `MarkNotified`'s `AND notified_at IS NULL` prevents a double ack, not a double send. The POST happens first and the rows-affected count is discarded (`notifier.go:185`), so separate guards would let a real-time pass and a digest send list and send the same rows at the same time, in both toggle directions.
- D-05 SettingsReader is a required constructor argument. Declare it in `internal/notifier` as `Get(ctx context.Context) (settings.Settings, error)`. It returns the full `Settings` because Phase 22 needs cadence and watermark from the same read, and `settings` imports only `sqlc`, so there's no cycle. It's a required argument of `notifier.Select` and `notifier.New`, not a functional option, because a forgotten option would pass every notifier test and silently ship an ungated notifier. `cmd/server/main.go` passes the existing `settingsStore`.
- D-06 SPA helper text. Add always-visible helper text under the Digest mode row in `web/app/components/system/DigestSettings.tsx`: "While on, new events wait for the next digest; switching back off delivers them individually." The web Definition of Done applies (`prettier --write`, `corepack pnpm test`). Bundle note: `dist/` is gitignored, but `internal/webassets/build/client/` is a committed copy of the built SPA (refreshed via `make web`, as Phase 20-04 did; the Docker build regenerates it from source regardless). Whether Phase 21 refreshes it is the phase planner's call.
- D-07 Deterministic tests. Use a fake SettingsReader/Sender that flips mode (or returns an error) on the k-th call, and assert exact send and ack counts. Don't use looped timing invariants. Cover: digest-on standdown (zero sends, rows stay pending); the off flush through the ordinary path; a per-send re-read stopping mid-pass; fail-closed at top-of-pass and mid-pass; no Warn on ctx cancel; staleness anchored to `created_at`, including the 1-day slack boundary; transition logging (boot, and on-to-off with the count). Real-settings tests build the reader from `testutil.NewIsolatedTestPool`, never the shared pool, because `internal/httpserver/settings_test.go` flips the singleton row and packages run in parallel. CI does run `-race` (`make test-integration` runs `go test ./... -race` on ubuntu-latest; full-pipeline.yml:54, Makefile:76). Only this Windows dev box can't, and the toggle-mid-pass risk is a logical interleaving that `-race` wouldn't catch anyway.
Claude's Discretion (the only remaining items): exact log wording (follow the terse `slog` structured-field style already in `notifier.go`), and how the last-observed mode is stored on `Notifier`. Remove the old discretion items on interface shape, wiring style, and concurrency testing, since D-05 and D-07 now settle them.

canonical_refs:
- In the ROADMAP bullet, describe the notes as covering gate location, the single outbox, the fail-closed posture, the shared sender lock, the staleness anchor, and the testing note.
- Add `docs/adr/0002-one-outbox-one-sender-lock.md` (D-04) and root `CONTEXT.md` (glossary: Outbox, Pending event, Flush, Digest window) as canonical refs.
- In the existing-guards bullets, say `notifying` becomes the shared sender lock (D-04), the `markNotified` idempotent ack prevents a double ack but not a double send, and the WR-03 Warn is the model for D-03's Warn.
- Replace the Testing pattern references subsection with: `internal/testutil/postgres.go` `NewIsolatedTestPool` (isolated schema for real-settings tests), the `internal/httpserver/settings_test.go` shared-row hazard, and CI's `-race` run (full-pipeline.yml:54 / Makefile:76).
- Rewrite the Definition of Done bullet: the backend gates (`go vet`, `golangci-lint run`, `make test`, `make coverage-gate`, `make sqlc-check`) plus the web gates, because D-06 changes `DigestSettings.tsx`. Point to D-06 for the bundle note.

code_context:
- Reusable Assets: `internal/settings` `Service.Get` (built as `settingsStore` at `cmd/server/main.go:255`) returns `Settings` with `DigestEnabled`, and passes ctx through unbounded (D-03 bounds it). `dbOpTimeout` plus the `listUnnotified`/`markNotified` helpers are the pattern for a bounded settings read. `staleReleaseDate` is reused as-is (D-02).
- Established Patterns: drop the bullet that proposed functional options as the wiring shape. Say instead that `WithMaxReleaseAgeDays` stays an option but `SettingsReader` is deliberately a required argument (D-05). Keep the consumer-declared-seam and `slog` style bullets.
- Integration Points:
  - notifier.go: SettingsReader interface; required arg on New/Select; last-observed-mode field; bounded mode read at the top plus a per-send re-read; the `suppresses` cutoff from `ev.CreatedAt`.
  - cmd/server/main.go:299: pass `settingsStore`. Existing `New`/`Select` call sites in tests gain the argument.
  - `web/app/components/system/DigestSettings.tsx` plus `DigestSettings.test.tsx`: the helper text.
  - notifier_test.go: the D-07 test list.

specifics: replace the body with the pass order, as a short sequence: CAS lock, then a bounded mode read (digest on or read error means stop before `listUnnotified`), then list, then the loop. In the loop, a suppressed row is acked with no re-read. Otherwise re-read the mode (digest on or read error means stop, the rest stay pending), then send, ack, and space. Keep the SC#5 point that this is not a post-hoc filter on fetched rows. Keep "no new timestamp column, no `digest_pending`, no second queue table", adding that staleness uses the existing `created_at` (D-02).

deferred: delete the first bullet, the one about exempting pending events from staleness suppression. In its place add one line: "Staleness exemption for pending events: superseded by D-02's `created_at` anchor, so there's nothing left to defer." Keep the Phase 22 and Phase 23 bullets, and extend the Phase 22 one with "(a `Notifier` method on the shared lock, ADR 0002)".

Footer: keep the existing two lines and add "*Revised: 2026-09-16 (post-grilling; audit trail in 21-DISCUSSION-LOG.md)*".

21-DISCUSSION-LOG.md: leave every existing line as is, including the "Audit trail only" header block and the original sections. Append at the end a `---` separator, then "## Post-grilling revision (2026-09-16)", then a one-sentence intro saying a grilling session challenged the gathered context and the selections below supersede the earlier ones in 21-CONTEXT.md. After that, one H3 subsection per challenged item, in the existing format: a table with columns Option | Description | Selected, then a "**User's choice:**" line and a "**Notes:**" line holding the one-line why. Items:
(1) Mode logging: per-pass standdown line (original) / silent / transition-only with the flush count (selected). Why: a per-pass line repeats every cycle for as long as digest mode is on.
(2) Staleness during pending: now-anchored cutoff unchanged (original) / a marker exempting pending events (new state) / cutoff anchored to `created_at` with 1-day slack (selected). Why: time pending must not age events out, and `created_at` already exists.
(3) Settings-read failure: fail-open to real-time with a Warn (original, 2026-09-11 lock) / fail closed returning an error / fail closed with a Warn and nil return (selected). Why: the pending set is the next digest, and returning an error double-logs as a delivery failure.
(4) Settings-read bound: unbounded `Get` / bounded by `dbOpTimeout` (selected). Why: an unbounded read can hang the pass while it holds the lock.
(5) Exclusion between real-time and digest sends: a separate CAS guard per sender (original Phase 22 note) / a per-send re-read alone / a Postgres advisory lock / the shared `notifying` lock with the mode read under it plus a per-send re-read (selected). Why: an idempotent ack is not an idempotent send.
(6) SettingsReader wiring: functional option / required constructor argument with a Get-only seam returning full `settings.Settings` (selected). Why: a forgotten option ships an ungated notifier and still passes the tests.
(7) SPA copy: no SPA change (the original backend-only assumption) / helper text under the Digest mode row (selected), with the exact wording from D-06.
(8) Concurrency test approach: looped exact-equality invariant / deterministic fakes that flip on the k-th call (selected). Why: the risk is a logical interleaving, and the premise that CI lacks `-race` was wrong.
(9) Vocabulary: keep the roadmap's SC#2 wording / "everything that accumulated while digest mode was on" plus the glossary terms Outbox, Pending event, Flush (selected). Why: the glossary's Digest window is an event set, not a time period.

Commit both files together (explicit `git add` of the two paths, hooks enabled) with a natural message such as "docs(21): revise phase context with the decisions settled in grilling". No attribution trailer of any kind (project CLAUDE.md overrides harness instructions).
  </action>
  <verify>
    <automated>cd /c/CodeProjects/drop-tracker && F=.planning/phases/21-real-time-digest-mutual-exclusion/21-CONTEXT.md && L=.planning/phases/21-real-time-digest-mutual-exclusion/21-DISCUSSION-LOG.md && grep -qF 'D-04' "$F" && grep -qF 'D-05' "$F" && grep -qF 'D-06' "$F" && grep -qF 'D-07' "$F" && grep -qF 'docs/adr/0002-one-outbox-one-sender-lock.md' "$F" && grep -qF 'ev.CreatedAt' "$F" && grep -qF 'dbOpTimeout' "$F" && grep -qF 'NewIsolatedTestPool' "$F" && grep -qF 'full-pipeline.yml:54' "$F" && grep -qF 'While on, new events wait for the next digest; switching back off delivers them individually.' "$F" && grep -qF 'internal/webassets/build/client/' "$F" && grep -qF "Claude's Discretion" "$F" && grep -qF 'Revised: 2026-09-16' "$F" && ! grep -qF 'No frontend changes expected' "$F" && ! grep -qF 'Concurrency proof technique' "$F" && ! grep -qF 'absent from CI' "$F" && ! grep -qF 'fails open to real-time' "$F" && ! grep -qF 'Digest-queue staleness exemption' "$F" && ! grep -qF 'Log one Info summary line per notify pass' "$F" && ! grep -qF 'single-clock model' "$F" && grep -qF 'Audit trail only' "$L" && grep -qF '## Standdown Log Visibility' "$L" && grep -qF '## Fail-Open Failure Visibility' "$L" && grep -qF '## Post-grilling revision (2026-09-16)' "$L" && [ "$(grep -c 'User.s choice' "$L")" -ge 12 ] && echo OK</automated>
  </verify>
  <done>21-CONTEXT.md holds D-01..D-07 as settled, with only log wording and last-observed-mode storage left to discretion. It references ADR 0002, records the committed-bundle fact accurately, and none of the superseded phrasings remain. 21-DISCUSSION-LOG.md keeps its original content and ends with a nine-item post-grilling revision section. Both are committed with no attribution trailer.</done>
</task>

<task type="auto">
  <name>Task 3: Align ROADMAP.md Phase 21 and Phase 22 with the settled decisions</name>
  <files>.planning/ROADMAP.md</files>
  <read_first>.planning/ROADMAP.md lines 75-177 (the v1.5 checklist, Ordering rationale and Deploy sequencing blockquote, Phase 21, Phase 22)</read_first>
  <action>
Use scoped Edit calls only, never a whole-file Write. Do NOT change any Goal line, the v1.5 milestone checklist bullets, any "**Plans**:" line, any plan list, or the Progress table.

Ordering rationale blockquote (~line 82), vocabulary only: change "digest on means events queue and nothing is lost" to "digest on means events stay pending and nothing is lost". In the Deploy sequencing paragraph (~line 84), change the tail of the clause "with nothing yet built to ..." so it reads "with nothing yet built to deliver the pending events".

Phase 21 Success Criteria:
- Replace criterion 2 with: "Toggling digest back off delivers everything that accumulated while digest mode was on through the ordinary real-time path on the next poll cycle — one message per event, in the existing order and spacing, none dropped and none sent twice."
- Replace criterion 5 with: "The mode check is the notify pass's **first** decision once it holds the sender lock, and it is repeated before every Discord send. It is not a post-hoc filter: there is no code path on which an event is sent in real time *and* left pending for a later digest, and a settings read that fails skips the pass, leaving events pending, never delivering them in real time while digest mode may be on."

Phase 21 "Notes for the phase planner":
- First bullet (riskiest change): replace its second sentence, the one describing the change as a single gate plus wiring, with: "The change is a digest-mode read at the top of `Notifier.NotifyPending` (`internal/notifier/notifier.go:155`), taken after acquiring the `notifying` lock and before `listUnnotified`, then re-read before each Discord send, through a `SettingsReader` that `notifier.New`/`notifier.Select` take as a required argument, plus the wiring at the composition root." Keep the poller sentences after it.
- "Keep exactly one outbox" bullet: change "finds the queued rows" to "finds the pending rows". Nothing else.
- Replace the whole bullet that starts "*Failure posture for the settings read is locked" with: "*Failure posture for the settings read: fail CLOSED* (post-grilling revision, 2026-09-16, reversing the 2026-09-11 lock). A read error, at the top of the pass or before a send, stops the pass, logs a distinct Warn (not on ctx cancellation), and returns nil, so `poller.go`'s log-and-continue call site doesn't add a second Error that reads like a delivery failure. In digest mode the pending set is the next digest: failing open would flush and ack a whole digest on one transient error, while skipping a pass loses nothing because the outbox persists. Every settings read is bounded by `dbOpTimeout`."
- Replace the whole "*Existing guards to preserve.*" bullet with: "*Existing guards to preserve.* `MarkNotified` is idempotent via `AND notified_at IS NULL` (D-09), but that prevents a double ack, not a double send: the Discord POST comes first and the rows-affected count is discarded. The exclusion mechanism is the `notifying` `atomic.Bool` CAS guard (D-06), which becomes the one sender lock that every outbox send serializes on (real-time now, the Phase 22 digest send later; see `docs/adr/0002-one-outbox-one-sender-lock.md`). Whichever send finds it held skips, which is safe because the outbox persists. Residual: at most the one message already in flight when digest mode is turned on. Assert both guards rather than reworking them."
- Insert a new bullet right after that one: "*Staleness is anchored to detection time.* `suppresses()` computes its cutoff from `ev.CreatedAt` minus `maxAgeDays` minus 1 day (UTC) rather than from `time.Now()`, so time spent pending never ages an event out. The 1-day slack absorbs the gap between detection's captured `now` and the DB's `created_at`. Pre-fix backlog rows are still suppressed. `events.created_at` already exists, so there's no schema change."
- Replace the whole "*Testing note.*" bullet with: "*Testing note.* Prove the toggle interleavings with deterministic tests, not looped timing invariants: a fake `SettingsReader`/`Sender` that flips mode (or errors) on the k-th call, with exact send and ack counts asserted. Real-settings tests build the reader from `testutil.NewIsolatedTestPool`, never the shared pool (`internal/httpserver/settings_test.go` flips the singleton row, and packages run in parallel). CI does run `-race` (`make test-integration`, full-pipeline.yml:54 / Makefile:76, ubuntu-latest). Only this Windows dev box can't, and a toggle mid-pass is a logical interleaving `-race` wouldn't catch anyway."
- Replace the "*Operator-facing copy.*" bullet with a bullet that keeps the italic lead "*Operator-facing copy.*" and says: add always-visible helper text under the Digest mode row in `web/app/components/system/DigestSettings.tsx`, with the helper text quoted in plain double quotes and reading exactly: While on, new events wait for the next digest; switching back off delivers them individually. Then add one sentence saying the web Definition of Done applies (`prettier --write`, `corepack pnpm test`).

Phase 22 "Notes for the phase planner":
- In the "*Scheduling mechanism.*" bullet, replace only its final sentence, the one about giving the ticker an overlap guard of its own that matches the poller's running-flag idiom, with: "The digest send is a `Notifier` method and serializes on the same `notifying` lock as real-time delivery (`docs/adr/0002-one-outbox-one-sender-lock.md`). Whichever send finds it held skips, which is safe because the outbox persists, so a long flush makes a due tick skip rather than race it." Keep everything before those sentences.
- Insert a new bullet directly after the bullet whose italic lead is Two different sinces (the one about outbox state versus the watermark): "*Staleness.* Phase 21 already anchors the stale-release cutoff to each event's `created_at`, so a weekly cadence doesn't age pending events out before the digest fires."

Commit ROADMAP.md alone (explicit `git add .planning/ROADMAP.md`, hooks enabled) with a natural message such as "docs(21): bring the roadmap in line with the revised phase 21 decisions". No attribution trailer of any kind (project CLAUDE.md overrides harness instructions). Run the verify command before committing, since its unchanged-lines check reads the working-tree diff.
  </action>
  <verify>
    <automated>cd /c/CodeProjects/drop-tracker && R=.planning/ROADMAP.md && grep -qF 'accumulated while digest mode was on' "$R" && grep -qF 'skips the pass, leaving events pending' "$R" && grep -qF 'fail CLOSED' "$R" && grep -qF 'prevents a double ack, not a double send' "$R" && grep -qF 'Staleness is anchored to detection time' "$R" && grep -qF 'NewIsolatedTestPool' "$R" && grep -qF 'flips mode' "$R" && grep -qF 'While on, new events wait for the next digest; switching back off delivers them individually.' "$R" && grep -qF 'already anchors the stale-release cutoff' "$R" && grep -qF 'events stay pending and nothing is lost' "$R" && [ "$(grep -cF 'docs/adr/0002-one-outbox-one-sender-lock.md' "$R")" -ge 2 ] && ! grep -qF 'fail OPEN' "$R" && ! grep -qF 'its own CAS overlap guard' "$R" && ! grep -qF 'queued during the digest window' "$R" && ! grep -qF 'absent from CI' "$R" && ! grep -qF 'held until the next digest' "$R" && ! grep -qF 'drain the queue' "$R" && ! git diff -U0 -- "$R" | grep -qE '^[-+]([|]|[*][*]Plans|[*][*]Goal|- \[)' && echo OK</automated>
  </verify>
  <done>ROADMAP.md Phase 21 SC#2 and SC#5 and its notes (gate description, guards, staleness, fail-closed, testing, copy) match the settled decisions. The Phase 22 scheduling note points at the shared lock and ADR 0002, and a staleness note sits beside it. The Ordering rationale and Deploy sequencing prose use pending/outbox vocabulary. Goals, checklists, plan counts, and the progress table are unchanged. Committed with no attribution trailer.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| none | Docs-only edits to committed markdown; no runtime code, input handling, or secrets are touched |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-q260916-01 | Information disclosure | committed docs | low | mitigate | Docs cite code identifiers and file:line only, never env values or webhook URLs; the gitleaks pre-commit hook runs on every commit (no `--no-verify`) |
| T-q260916-02 | Tampering | ROADMAP.md unrelated phases | low | mitigate | Scoped Edit calls only (no whole-file Write); Task 3's verify fails if any Goal, Plans, checklist, or table line changed |
</threat_model>

<verification>
- All three task verify commands print OK.
- `git log --oneline -3` shows three docs commits for this task, and `! git log -3 --format=%B | grep -qiE 'co-authored-by|claude-session|generated with'` succeeds (no attribution trailer on any of them).
- `git diff --stat HEAD~3` lists only the five files in files_modified: no Go, TS, SQL, or config file changed, so the go/pnpm gates don't apply.
</verification>

<success_criteria>
- ADR 0002 exists and is referenced from both 21-CONTEXT.md and ROADMAP.md.
- CONTEXT.md defines Outbox, Pending event, and Flush in glossary form.
- 21-CONTEXT.md D-01..D-07 and ROADMAP.md Phase 21/22 notes reflect all ten settled decisions, with superseded phrasings gone.
- 21-DISCUSSION-LOG.md keeps its audit trail and records the revision.
</success_criteria>

<output>
Create `.planning/quick/260916-dvy-record-post-grilling-phase-21-decisions-/260916-dvy-SUMMARY.md` when done.
</output>
