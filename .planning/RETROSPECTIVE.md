# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.1 — Hardening & Scale Readiness

**Shipped:** 2026-08-17
**Phases:** 5 (08-11.1) | **Plans:** 22 | **Sessions:** not tracked

### What Was Built
- Vitest + React Testing Library component test suite for the watchlist, search, and history React surfaces, mocking the app's API boundary
- CI coverage gates blocking `build-scan`/`release` below 80% backend / 70% frontend, proven live in both the red and green direction on real GitHub Actions runs
- A configurable event-retention window (soft-delete/filter, default 90 days) that hides aged-out history while leaving every row and all detection state intact
- Bounded, env-configurable worker-pool polling for MusicBrainz and Deezer, replacing sequential per-artist iteration, plus an atomic `FOR UPDATE`-locked CTE closing a real lost-update race on shared deluxe-change baselines
- A dedicated tech-debt phase (11.1) closing every item the milestone audit flagged, including a real accessibility bug in the History filter UI

### What Worked
- Landing the highest-risk change (bounded concurrency, Phase 11) last, behind a working coverage harness (Phases 8-9), meant regressions in the concurrency rewrite were caught by tests rather than discovered live
- Using real GitHub Actions runs (not just local assertions) to close backstop-tier UAT truths for the coverage gates — directly observed red/green transitions on a scratch branch, never on `main`
- Folding related CI checks into an existing job (Prettier into `frontend-test`) instead of adding new pipeline surface for each small gate

### What Was Inefficient
- Tech debt accumulated across Phases 08-11 needed an entire extra phase (11.1) to close rather than being resolved inline phase-by-phase — worth watching whether review findings can be closed closer to when they're found
- v1.0 was never formally closed through `/gsd-complete-milestone` (no MILESTONES.md entry, no archived phase directories, no git tag) before v1.1 work started. Running the milestone-close workflow for v1.1 swept up all 12 phases (1-11.1) instead of just v1.1's 5, and required manual correction of the archive and MILESTONES.md entry after the fact
- Windows dev-machine limitations (`go test -race` unusable, MusicBrainz TLS failure over WSL2) recurred again this milestone as a known, already-documented cost rather than something newly discovered

### Patterns Established
- `FOR UPDATE`-locked `UPDATE...RETURNING` CTE is the standard fix for a check-then-act race once concurrency is introduced — used for the deluxe-change baseline, reusable for any future shared-mutable-state race
- A buffered-channel semaphore is sufficient for bounded per-cycle fan-out concurrency — no third-party worker-pool library needed
- New CI gates (coverage, formatting) get added to an existing job's `needs:`/steps rather than spawning a new job per gate, keeping the pipeline graph simple

### Key Lessons
1. Run the milestone-close archival step promptly when a milestone actually ships, even if the full ceremony (tag, retrospective) waits — deferring it let phase directories pile up un-archived and corrupted the next milestone's close.
2. A scoped tech-debt phase driven directly by a milestone audit's findings (13 locked decisions, each mapped to one audit item) is an effective, low-risk way to close review findings without reopening already-verified phases.
3. Concurrency-introducing phases benefit from landing last in a milestone, behind a working test/coverage harness that can catch what the rewrite breaks.

### Cost Observations
- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry was captured for v1.1 — worth deciding whether to start tracking this in v1.2 if cost visibility becomes valuable

---

## Milestone: v1.2 — Cleanup & Display Fixes

**Shipped:** 2026-08-24
**Phases:** 2 (12-13) | **Plans:** 6 | **Sessions:** not tracked

### What Was Built
- `CoverArt.tsx`'s stale-placeholder bug fixed via a `useEffect([src])` reset, fixing History, Watchlist, and search-result rows from one shared-component change
- Deezer fan-count-based search popularity ranking (`Client.SearchArtists`, stable descending sort) and a MusicBrainz `country`-code disambiguation fallback for search results, absorbing backlog Phase 999.1
- History cards for guest-feature and deluxe-change events now render a release date, sourced via a new per-recording MusicBrainz lookup with a precision-aware earliest-date rule and a per-cycle rate cap
- Guest-feature release cards render album art, matching new-release cards
- A new hand-rolled `internal/artistart` matcher (strict close-name equality + guarded shared-album-title tie-break, fail-closed on ambiguity) resolves MusicBrainz artist art from Deezer, wired into both add-time and a cooldown-bounded startup backfill sweep coordinated by a shared `ActivityGate` — absorbing backlog Phase 999.2

### What Worked
- Fixing a bug in one shared component (`CoverArt.tsx`) automatically fixed all three consumers (History, Watchlist, search) with zero call-site changes — no need to touch or re-test each caller
- Fail-closed design for the MusicBrainz→Deezer artist-art matcher (reject on any ambiguity rather than guess) traded a small number of missing photos for zero misattributed ones, matching the phase's own threat model
- Code-review warnings found during Phase 13's UAT verification (a date-parsing panic, a stats double-count, an `ActivityGate` leak on panic) were fixed in place immediately with regression tests instead of deferred to a follow-up phase, continuing the pattern v1.1 identified as worth doing more of

### What Was Inefficient
- Phases 12 and 13 ran as ad-hoc post-v1.1 cleanup with no REQUIREMENTS.md and no milestone version assigned until close time — `/gsd-complete-milestone` was first invoked with a stale "1.1" argument (already shipped/tagged) and had to be redirected to v1.2 mid-workflow. Assigning the next milestone version when this cleanup work started, rather than only at close, would have avoided the confusion.
- The milestone-close CLI (`gsd-tools.cjs query milestone.complete`) couldn't detect Phase 12/13 as belonging to a milestone (they weren't grouped under a `### v1.2 ...` heading in ROADMAP.md) and returned 0 phases/plans/accomplishments — the archive, MILESTONES.md entry, and phase-directory move all had to be done manually. Grouping ad-hoc phases under a milestone heading in ROADMAP.md as soon as a version is decided (not just at close) would let the automation work correctly.
- Windows dev-machine limitations (`go test -race` unusable, MusicBrainz TLS failure over WSL2) recurred again as a known, already-documented cost rather than something newly discovered.

### Patterns Established
- `useEffect([dep]) → reset` is the standard fix for a retained component whose failure/error state derives from a prop that can change without a remount
- `slices.SortStableFunc` (not `SortFunc`) is the standard choice for any popularity/relevance-style sort where ties are common and the pre-sort order carries meaning
- A shared `ActivityGate` priority-yielding primitive is the standard way to coordinate two independent consumers (an interactive path and a background sweep) against one external rate budget, instead of giving the background consumer its own budget
- Fail-closed strict-match + guarded-tie-break is the standard shape for any cross-source identity matching where a wrong match is worse than no match

### Key Lessons
1. Assign and record a milestone version (even provisionally) as soon as post-ship cleanup work starts, and group it under that version's heading in ROADMAP.md — waiting until `/gsd-complete-milestone` runs to decide the version number causes both human confusion and automation misdetection.
2. Fail-closed is worth the cost for any feature that attaches identity data (a photo, a name) from a second source with imperfect matching — a wrong result is worse than a missing one.
3. Closing code-review/UAT-surfaced warnings immediately, in the same phase, continues to beat deferring them — this is the second milestone running where that held true.

### Cost Observations
- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry captured for v1.2, consistent with v1.1

---

## Milestone: v1.3 — Continuous Deployment (partial)

**Shipped:** 2026-09-09 (partial — Phase 17 deferred)
**Phases:** 4 planned (14-17) | **Plans:** 15 | **Tasks:** 46 | **Sessions:** not tracked

### What Was Built

- Optional single-passphrase instance gate — `internal/authgate` (HMAC-SHA256 signed cookie, `Manager` middleware, `/session` handlers, `Alerter` seam), `httpserver.WithAuthGate` moving the six data routes behind a protected chi Group, per-IP login throttle + fixed comparison delay + alert-only brute-force counter, a framework-free SPA `authStore` + `PassphraseScreen` + gateActive-gated Log out control; fully inert when `INSTANCE_PASSPHRASE` is unset (Phase 14)
- `cmd/coverage-report` (stdlib-only) plus CI wiring: a report-only `coverage-comment` job that sticky-upserts one same-repo PR comment showing backend/frontend coverage and the pp delta vs. a SHA-keyed main baseline, sharing one measurement algorithm with `make coverage-gate` (D-17); never blocks a merge (Phase 15)
- Rollback-safe migrations — an ahead-of-source no-op guard in `internal/db/migrate.go` (`maxSourceVersion` + `runMigrationsWithSource`), `cmd/migration-check` (stdlib-only SQL tokenizer flagging backward-incompatible / unsafe-forward DDL with README-citing messages + a non-overridable previous-release query cross-reference), and unconditional `migration-check` / `n1-boot` CI jobs with step-gated expensive work; `internal/db/migrations/README.md` documents the expand/contract rule (Phase 16)
- `internal/sqlscan` extracted from `cmd/migration-check` as a reusable SQL lexing module

### What Worked

- **Splitting Phase 16 (rollback safety) out of the deploy phase** so the cross-cutting migration rule landed *before* auto-rollback exists rather than as the last task inside the heaviest phase — the discipline is now in force for every future migration regardless of when Phase 17 lands
- **Verifying research assumptions against the actual pinned dependency during planning**, not just from docs — Phase 16 caught two false founding assumptions this way: `migrate.Up()` against an ahead-of-source schema returns a hard error (not `ErrNoChange`), and a skipped `needs:` job skips its dependents (not counts as success). Both would have been production or CI bugs
- **The report-only CI job shape** (real job, job-scoped write permission, in no `needs:` graph, `continue-on-error: true`) cleanly delivered CICD-13's "never-blocking" property — proven live when a deliberate coverage drop turned `frontend-test` red while `coverage-comment` stayed green
- Gap-closure plans (14-05/06/07) closed UAT gaps inside Phase 14 rather than deferring them, continuing the pattern v1.1/v1.2 identified

### What Was Inefficient

- **Phase 14 needed three separate gap-closure rounds.** G-14-1: the gate shipped *inert* (container booted with an empty `INSTANCE_PASSPHRASE`) and survived an entire UAT round unnoticed because the inert path was silent. G-14-2 and G-14-3: two more Log-out-control bugs, the deeper one (no gated-load signal for a session already holding a valid cookie) present since plan 14-03 but not found until a third UAT pass. A boot-status log line and a gated-load signal both belonged in the original plan.
- **Two Phase 14 debug sessions were fixed in gap-closure plans but never filed as resolved** — they surfaced as open-artifact noise at this milestone's close and had to be reconciled by hand (mirrors v1.1/v1.2's manual-correction findings, one layer down).
- **Phase 17 was roadmapped as a milestone phase despite depending on hardware the developer does not have.** v1.3 therefore cannot fully ship. It should have been its own milestone gated on "a VPS + domain exist", not a phase inside a milestone that was otherwise complete weeks earlier.
- Windows dev-machine limitations (`go test -race`, MusicBrainz TLS over WSL2) recurred again as a known, documented cost.

### Patterns Established

- **Verify a research finding against the pinned dependency's real behavior during planning** — a hermetic RED→GREEN proof against the actual library version, not a docs citation, for any assumption a phase's design rests on
- **stdlib-only Go CLI tools for CI logic** (`cmd/coverage-report`, `cmd/migration-check`, backed by `internal/sqlscan`) — unit-testable, zero new deps, greppable call sites; the default shape for new pipeline checks
- **Report-only CI job** = real job + job-scoped permission + no `needs:` membership + `continue-on-error: true`, for anything that must inform without ever blocking
- **A security feature's inert/disabled path must be observable** — emit one boot line stating active/inert, so "it does nothing" cannot pass a UAT round silently

### Key Lessons

1. Do not roadmap a phase whose critical dependency is external infrastructure you do not control. Make it its own milestone, gated on the prerequisite actually existing — otherwise the whole milestone hangs on it.
2. File a debug session as `resolved` (and move it to `debug/resolved/`) the moment its fix lands in a plan, not only when a debug cycle formally closes it. Orphaned `diagnosed` / `awaiting_human_verify` sessions become milestone-close noise.
3. An inert or disabled security control is invisible without an explicit status signal. The boot-status log line added in Phase 14 gap closure should have been in plan 14-01.

### Cost Observations

- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry captured for v1.3, consistent with v1.0–v1.2

---

## Milestone: v1.4 — Operator Observability

**Shipped:** 2026-09-11
**Phases:** 3 (18, 18.1, 19) | **Plans:** 12

### What Was Built

`GET /ready` distinct from `/health` liveness; an in-process, mutex-guarded poll-run ring buffer (`internal/pollruns.Store`, ADR-0001) replacing a speculative `poll_runs` table; `runCycle` instrumentation wiring the ring buffer live via a channel-fold aggregation; a gated `GET /status` JSON contract plus SHA-based app version; and a "System" tab in the SPA rendering it all as a five-state render machine with a keep-stale manual Refresh.

### What Worked

- **A design grilling before planning caught three concurrency hazards in the originally-proposed `poll_runs` table** — a prune-on-insert race, a cross-source deadlock, and a skip-row-eviction hazard — and replaced it with an in-process ring buffer (ADR-0001) before any code existed, eliminating a migration and a local-only `sqlc-check` drift gate along with the hazards.
- **Splitting Phase 18 into 18 (additive) and 18.1 (the `runCycle` edit)** isolated the milestone's single riskiest change — counter aggregation across worker goroutines — into its own phase, reviewable independently of the low-risk endpoint/store work.
- **Contract-freeze discipline held across the phase split**: Phase 18 froze `/status`'s JSON shape in `docs/api/status-contract.md` before Phase 19 started, so the frontend typed against a real response body rather than a guess.
- **Channel-fold aggregation plus a 1000-iteration exact-equality invariant test stood in for `go test -race`** (still unusable on this dev box, still absent from CI) — correctness by construction, verified at scale, not by detector.
- **Phase 19 extended the shipped `history.tsx` conventions** (fetch-on-mount, three-way empty/error/first-run copy, Retry) rather than inventing a new pattern for the System view.

### What Was Inefficient

- **Phase 18.1 was fully executed but never formally transitioned to complete in STATE.md** — caught and retroactively corrected at this same milestone's close (2026-09-11), one level down from v1.3's "file debug sessions as resolved when they close" lesson: this time it was a phase-completion flag, not a debug session.
- **No milestone audit (`/gsd-audit-milestone`) existed for v1.4** — only v1.0 has one on file. The close proceeded on ROADMAP.md/REQUIREMENTS.md self-report (100% phases, 12/12 requirements) rather than a structured cross-phase audit; accepted as sufficient given full requirement coverage, but the audit habit established at v1.0 didn't carry forward.

### Patterns Established

- **An ADR for a storage-shape decision that rejects a database table in favor of an in-process structure** (`docs/adr/0001`) — first ADR in the repo; the pattern is to write one whenever a design grilling overturns a REQUIREMENTS-level storage assumption.
- **Splitting a phase into a low-risk/high-risk pair after a design grilling**, so the riskiest concurrency or correctness change in a milestone gets its own single-purpose phase rather than riding along with additive work.
- **Channel-fold counter aggregation** (buffered-to-dispatched-count channel, one send per worker including the panic-recovery path, single-threaded fold after `wg.Wait()`) as the house pattern for concurrent counter correctness when `-race` is unavailable, backed by a high-iteration invariant test.

### Key Lessons

1. When `go test -race` is unavailable (dev box or CI), design concurrent aggregation so correctness is structural (channel fold, no shared mutable state) and prove it with a high-iteration exact-equality invariant test — don't substitute more code review for the missing detector.
2. Run a design grilling before planning a phase whose design rests on a speculative schema change — it can eliminate a migration, a class of races, and a drift gate before a line of code is written (mirrors v1.3's "verify research assumptions against the pinned dependency" lesson, one step earlier in the process).
3. Mark a phase's completion transition the moment execution finishes, not at the next convenient checkpoint — an executed-but-untransitioned phase becomes cleanup work at milestone close, the same failure shape as v1.3's orphaned debug sessions.

### Cost Observations

- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry captured for v1.4, consistent with v1.0–v1.3

---

## Milestone: v1.5 — Digest Notifications

**Shipped:** 2026-09-18
**Phases:** 4 (20-23) | **Plans:** 15

### What Was Built

An instance-wide, Postgres-persisted digest mode (on/off + daily/weekly cadence) with a gated SPA panel (Phase 20); real-time ↔ digest mutual exclusion built on the existing outbox with no second queue, fail-closed on a settings-read error (Phase 21); a slot-based `DigestScheduler` with calendar-math DST-safe fire times, a bounded restart grace window, and one grouped Discord embed per send (Phase 22); and window-stamped, group-preserving multi-message chunking with per-chunk acking so a partial send never loses or re-sends events (Phase 23).

### What Worked

- **Design grillings before planning kept catching correctness bugs the roadmap's first pass had locked in** — Phase 22's plan grilling reversed a "log-and-wait, no immediate catch-up" rule that actually contradicted its own success criterion, and replaced send-timestamp-plus-duration due math with calendar-based slot math after spotting the DST failure mode; Phase 23's grilling caught that the roadmap's "10 embeds / 25 fields" Discord limits were unreachable under the locked one-embed-per-message shape, redirecting the "10" into a chunks-per-run cap instead of dead code.
- **Deploy-sequencing as an explicit, written rule** (Phases 21-23 held to one release) closed a real production wedge: shipping the standdown gate (21) without the sender (22) would have gone dark with no ETA; shipping 21+22 without 23's chunking would have let an oversized digest get silently rejected and retried forever, growing each cycle.
- **Two ADRs written before the code that needed them** (`docs/adr/0002` one-outbox-one-sender-lock, `docs/adr/0003` digest-ack-splits-event-ack-from-completion) turned two non-obvious concurrency/idempotence decisions into citable source-of-truth instead of tribal knowledge re-derived per phase.
- **CI's `-race` gate on Linux caught a genuine data race in Phase 22's own test code** (quick task 260917-mfa) that this Windows dev box structurally cannot detect locally — direct, current-milestone evidence for the standing lesson that CI's own `test` job, not local dev-box testing, is the authoritative concurrency gate.
- **Keeping one outbox (no second "digest queue" table)** made DGST-14 (toggle-off flushes the backlog) correct by construction rather than by additional reconciliation logic — the same "outbox state decides what, a separate signal decides when" shape recurred cleanly from Phase 21 into Phase 22's slot record.

### What Was Inefficient

- **Phase 23's code review surfaced two Warnings and one Info-level nit that shipped unfixed and untracked** — a dead `resuming = true` assignment that never takes effect (a latent trap for a future edit), two chunk-count test fixtures pinned only by code comments rather than a precondition-asserting test, and a singular/plural grammar slip in an operator-facing message. None got a todo filed; they're now recorded in PROJECT.md's Context section at milestone close instead of at the point they were found — a whole milestone later than the todo/quick-task pattern established at v1.1-v1.4 would suggest.
- **No `/gsd-audit-milestone` run for v1.5**, continuing the pattern from v1.2-v1.4 (only v1.0 and v1.1 have one on file) — closed on ROADMAP.md/REQUIREMENTS.md self-report (16/16 requirements, 4/4 phases verified) instead, accepted given full coverage, but the audit habit still hasn't stuck project-wide.

### Patterns Established

- **A written deploy-sequencing rule spanning multiple phases** ("Phases N-M ship in one release") as a first-class roadmap artifact when an intermediate phase's shipped-alone state would be operationally unsafe, not just logically incomplete.
- **Three distinct persisted signals for one feature, each owning exactly one question**: outbox state (`notified_at IS NULL`) decides *what* to send, a slot record decides *whether a send is due*, and a separate last-sent watermark stays purely the operator-facing display value — reusable shape for any future scheduled/batched delivery feature.
- **Per-delivered-unit acking instead of per-run acking** whenever a run can partially fail and resume (`AckEventsOnly` vs. the final `AckDigestBatch`) — the same shape as Phase 21/22's outbox-state pattern, one level more granular.

### Key Lessons

1. Run a design/plan grilling on the phase that's actually novel (a scheduler with DST math, a hard delivery-size limit), even after roadmap-time research already covered it — RESEARCH's own assumptions got overturned twice more (grace-window rule, Discord limit shape) at planning time in this milestone alone.
2. When a milestone's phases have a real production-safety ordering constraint beyond their dependency graph (a gate shipped without its sender, a sender shipped without its overflow handling), write the deploy-sequencing rule down in the roadmap itself — don't rely on remembering it at merge time.
3. A code-review finding that ships unfixed needs a todo filed in the same session it's found, or it becomes a milestone-close paragraph instead of a quick-task — the discipline that held for warnings in v1.1-v1.3 lapsed here.

### Cost Observations

- Model mix: not tracked this milestone
- Sessions: not tracked
- Notable: no cost/efficiency telemetry captured for v1.5, consistent with v1.0–v1.4

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Sessions | Phases | Key Change |
|-----------|----------|--------|------------|
| v1.0 | not tracked | 7 | Initial MVP; milestone never formally closed via `/gsd-complete-milestone` |
| v1.1 | not tracked | 5 (08-11.1) | First milestone closed via the full `/gsd-complete-milestone` workflow; added a dedicated tech-debt-closure phase pattern |
| v1.2 | not tracked | 2 (12-13) | Ad-hoc post-v1.1 cleanup phases (no REQUIREMENTS.md, version assigned only at close) closed via `/gsd-complete-milestone`; required manual archive/MILESTONES.md correction since the phases weren't pre-grouped under a milestone heading |
| v1.3 | not tracked | 4 planned (14-17), 3 shipped | First **partial** milestone close — Phase 17 deferred for lack of a VPS; debug-session and todo backlog acknowledged and carried forward at close |
| v1.4 | not tracked | 3 (18, 18.1, 19) | First milestone with a design-grilling checkpoint before planning, splitting a phase into low-risk/high-risk halves and writing the repo's first ADR; full requirement coverage (12/12) but closed without a `/gsd-audit-milestone` run |
| v1.5 | not tracked | 4 (20-23) | Post-roadmap plan grillings overturned a locked rule twice (Phase 22 grace window, Phase 23 Discord-limit shape); first explicit multi-phase deploy-sequencing rule written into the roadmap; two more ADRs (0002, 0003); full requirement coverage (16/16) but no `/gsd-audit-milestone` run, and Phase 23's own code-review findings shipped unfixed and untracked |

### Cumulative Quality

| Milestone | Tests | Coverage | Zero-Dep Additions |
|-----------|-------|----------|--------------------|
| v1.0 | not measured | not enforced | — |
| v1.1 | backend 83.5%+, frontend 70%+ (both CI-enforced) | 80% backend / 70% frontend gate | buffered-channel semaphore (no worker-pool lib), hand-rolled accessible combobox (no UI lib) |
| v1.2 | backend/frontend suites extended, gates held at 80%/70% throughout | 80% backend / 70% frontend gate (unchanged) | `internal/artistart` fail-closed matcher + `ActivityGate` primitive (stdlib-only, no new deps) |
| v1.3 | suites extended for authgate + the two CI tools; gates held at 80%/70% (backend cutover margin measured 10pp above floor) | 80% backend / 70% frontend gate (unchanged) | `internal/authgate` signed-cookie gate, `cmd/coverage-report`, `cmd/migration-check`, `internal/sqlscan` — all stdlib-only, no new deps |
| v1.4 | suites extended for `/ready`, `/status`, `pollruns`, and the System view; gates held at 80%/70%; a 1000-iteration invariant test substitutes for `-race` on `runCycle` | 80% backend / 70% frontend gate (unchanged) | `internal/pollruns` ring buffer + `internal/buildinfo` — stdlib-only, no new deps; repo's first ADR (`docs/adr/0001`) |
| v1.5 | suites extended for `internal/settings`, digest gating/scheduling/chunking, and property-style invariant tests over a synthetic 700-event batch; gates held at 80%/70%; CI's `-race` caught a real data race in Phase 22 test code | 80% backend / 70% frontend gate (unchanged) | `internal/settings` slot math + `golang.org/x/text` promoted to a direct dependency — no new third-party runtime deps; two more ADRs (`docs/adr/0002`, `0003`) |

### Top Lessons (Verified Across Milestones)

1. Close out a milestone's archival step promptly after shipping — letting phase directories accumulate un-archived corrupts the next milestone's close (v1.1).
2. Assign a milestone version and group phases under it in ROADMAP.md as soon as post-ship work starts, not just when `/gsd-complete-milestone` runs — otherwise both humans and the close automation lose track of which phases belong to which version (v1.2).
3. Fixing code-review/UAT-surfaced warnings inline, in the same phase they're found, beats deferring them — held true across v1.1, v1.2, and v1.3; **broke at v1.5** (Phase 23's two Warnings + one Info shipped unfixed with no todo filed, only caught at milestone close) — the discipline needs a forcing function (e.g. a checklist item at phase-complete), not just intent.
4. Don't roadmap a phase gated on infrastructure you don't control — it holds the whole milestone hostage. Make it its own milestone gated on the prerequisite existing (v1.3 / Phase 17).
5. File debug sessions as resolved when the fix ships in a plan, not only when a debug cycle closes them, or they resurface as milestone-close noise (v1.3).
6. A phase-completion transition needs the same discipline as a debug-session resolution — flip it the moment execution finishes, or it becomes retroactive cleanup at the next milestone close (v1.4, same failure shape as lesson 5 one level up).
7. A design/plan grilling can still overturn a locked rule after roadmap-time research already covered the same ground — run one on the genuinely novel phase (new scheduling math, a hard external limit) even when the roadmap looks settled (v1.5, twice in one milestone).
