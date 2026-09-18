---
phase: 22-scheduled-digest-send
plan: 03
subsystem: notifications
tags: [discord, digest, markdown-escaping, collation, notifier, golang.org/x/text]

# Dependency graph
requires:
  - phase: 22-02
    provides: buildDigestEmbed's signature, its call site in digest.go, and the single-embed Description shape -- unchanged by this plan
provides:
  - "escapeMarkdown / markdownEscaper (digest_format.go) -- backslash-escapes Discord's eleven markdown metacharacters, closes T-22-07"
  - "eventURL (format.go) -- the one shared per-event-type URL switch, replacing three duplicated call sites and formatEmbed's dedicated helper's previous callers, plus the digest builder's own local switch"
  - "artistKey / lineLabel / sortDigestGroup (digest_format.go) -- collated sort by watched artist (title/id tie-break), guest-feature host credit, deluxe track-count suffix"
  - "golang.org/x/text promoted from indirect to direct in go.mod, same version, no go.sum churn"
affects: [22-04]

# Actuals (#2632)
actuals:
  tokens: 7257
  tasks: 2
  commits: 2

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "strings.NewReplacer for markdown escaping in one pass -- a single NewReplacer.Replace call never rescans its own replacement output, so pair ordering (backslash listed first for reviewer clarity) cannot cause a double-escape regardless of order, unlike a naive sequence of strings.ReplaceAll calls."
    - "One collate.Collator created per buildDigestEmbed call, never a package-level value -- collate.Collator is not safe for concurrent use, and two schedulers or a scheduler plus a test could otherwise share one."
    - "Total sort ordering via a three-key tie-break chain (collated artist -> collated title -> unique event id) makes rendered output fully determined by the input set, independent of ListUnnotified's arrival order -- proven directly with a shuffle-then-compare test rather than only individually-ordered fixtures."

key-files:
  created:
    - internal/notifier/digest_format_test.go
  modified:
    - internal/notifier/digest_format.go
    - internal/notifier/format.go
    - go.mod

key-decisions:
  - "eventURL lives in format.go (not digest_format.go) since it is the shared switch formatEmbed's three formatters and the digest builder both call -- format.go already owned the three musicBrainz*URL/newReleaseURL leaf helpers it composes."
  - "digestTitleLimit's cap is applied via truncateRunes before escapeMarkdown, not after -- the cap counts operator-visible characters, not the backslashes escaping inserts, matching the plan's explicit ordering requirement."
  - "lineLabel branches only on eventTypeGuestFeature for the host-credit form; every other type (including a hypothetical future one) falls through to the plain 'Artist — Title' shape rather than needing its own case."

requirements-completed: [DGST-08, DGST-10]

coverage:
  - id: D1
    description: "escapeMarkdown backslash-escapes all eleven Discord markdown metacharacters before an artist name, host credit, or title reaches the digest Description, closing T-22-07; a title carrying brackets/parens can no longer terminate or retarget the masked link's href"
    requirement: DGST-08
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestEscapeMarkdown"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestEscapeMarkdown_AllElevenMetacharactersInOnePass"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestDigestLine_TitleWithBracketsCannotRetargetLink"
        status: pass
    human_judgment: false
  - id: D2
    description: "Each digest title is capped to 100 runes via truncateRunes before escaping is applied -- a 100-rune multi-byte title survives uncapped, a 150-rune title is cut on a rune boundary"
    requirement: DGST-08
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestDigestTitleLimit_CapsBeforeEscaping"
        status: pass
    human_judgment: false
  - id: D3
    description: "eventURL is the single switch every embed formatter (formatNewRelease/formatGuestFeature/formatDeluxeChange) and the digest builder call for a link -- formatEmbed's rendered output is byte-identical to before the extraction"
    requirement: DGST-08
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestEventURL"
        status: pass
      - kind: unit
        ref: "internal/notifier/format_test.go#TestFormatEmbed_URLsMatchExpectedHostAndPath"
        status: pass
    human_judgment: false
  - id: D4
    description: "Each event-type group sorts by golang.org/x/text/collate-collated watched artist (case- and accent-insensitive), tie-broken by title then ascending event id -- a total order proven invariant under input shuffling"
    requirement: DGST-10
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_OrderingIsCollatedCaseAndAccentInsensitive"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_TieBreaksByTitleThenID"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_ShuffleInvariant"
        status: pass
    human_judgment: false
  - id: D5
    description: "A guest_feature digest line names both the watched artist and the host credit ('Watched on Host — Title'), sorted and grouped under the watched artist, not artist_name"
    requirement: DGST-10
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestLineLabel"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_GuestFeatureGroupedAndSortedByWatchedArtist"
        status: pass
    human_judgment: false
  - id: D6
    description: "A deluxe_change line carries tracksFieldValue's suffix after the link's closing parenthesis when known, and no suffix (never a bare '()') when both counts are unknown"
    requirement: DGST-08
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_DeluxeTrackCountSuffix"
        status: pass
    human_judgment: false
  - id: D7
    description: "The artist sort/display key falls back from watched_artist_name to artist_name on both NULL and empty-string, for the label and the sort key alike"
    requirement: DGST-10
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestArtistKey"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_WatchedArtistNameFallback"
        status: pass
    human_judgment: false
  - id: D8
    description: "Group headings render in the fixed New Releases / Guest Features / Deluxe Changes order regardless of input order, and a zero-event group's heading is omitted entirely -- no empty heading, no placeholder line"
    requirement: DGST-10
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_GroupOrderFixedRegardlessOfInputOrder"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_EmptyGroupOmitsHeadingEntirely"
        status: pass
    human_judgment: false
  - id: D9
    description: "Two events for the same release from two sources, or two events for the same watched artist in one group, always render as two separate lines -- the builder never deduplicates (D-26)"
    requirement: DGST-08
    verification:
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_DuplicateSourcesRenderAsTwoLines"
        status: pass
      - kind: unit
        ref: "internal/notifier/digest_format_test.go#TestBuildDigestEmbed_SameArtistTwoEventsRenderAsTwoLines"
        status: pass
    human_judgment: false
  - id: D10
    description: "golang.org/x/text is a direct go.mod requirement at the unchanged v0.39.0, with no go.sum module additions or removals -- no install ran, the module was already pinned as a transitive dependency"
    verification:
      - kind: other
        ref: "grep -n 'golang.org/x/text' go.mod"
        status: pass
      - kind: other
        ref: "git diff -- go.sum"
        status: pass
    human_judgment: false

# Metrics
duration: 45min
completed: 2026-09-17
status: complete
---

# Phase 22 Plan 3: Digest Escaping, Collation, and Message Content Rules Summary

**Digest lines are now markdown-escaped and rune-capped (closing T-22-07), sorted by collated watched artist with a total title/id tie-break, and carry the guest-feature host credit and the deluxe track-count suffix -- via one shared `eventURL` switch and `golang.org/x/text` promoted to a direct dependency.**

## Performance

- **Duration:** 45 min
- **Started:** 2026-09-16T~23:46:00Z (approx.)
- **Completed:** 2026-09-17T02:31:07Z
- **Tasks:** 2
- **Files modified:** 4 (1 created, 3 modified)

## Accomplishments
- `escapeMarkdown`/`markdownEscaper` backslash-escape all eleven Discord markdown metacharacters (`\ * _ ~ \` | > # [ ] ( )`) in a digest line's artist name, host credit, and title before they reach the Description -- closes threat T-22-07, the phase's one open threat carried over from plan 22-02
- `eventURL` (format.go) is now the single switch deciding every event's outbound link -- `formatNewRelease`, `formatGuestFeature`, `formatDeluxeChange`, and the digest builder all call it, replacing three separate URL call sites and the digest builder's own duplicated switch; `formatEmbed`'s rendered output is unchanged (proven by the existing, unmodified `format_test.go` suite passing as-is)
- `digestTitleLimit` (100 runes) caps each title via the existing `truncateRunes` before escaping, so the escape backslashes never count against the visible-character budget
- Each event-type group sorts with a fresh `collate.New(language.Und, collate.IgnoreCase)` `Collator` (created per `buildDigestEmbed` call, never package-level, since `Collator` isn't concurrency-safe), keyed on `artistKey` and tie-broken by title then ascending event id -- a total order, proven shuffle-invariant
- `artistKey`/`lineLabel` replace the earlier `digestArtistName`: a guest-feature line now names both the watched artist and the host credit (`Rauw Alejandro on Drake — Title`), escaping the host credit too since it's the same community-editable `artist_name` column
- Deluxe-change lines append `tracksFieldValue`'s suffix (e.g. `(12 → 15 tracks)`) after the link's closing parenthesis when known, and nothing (never a bare `()`) when both counts are unknown
- `golang.org/x/text` promoted from indirect to direct in `go.mod` via `go mod tidy` -- same `v0.39.0`, zero `go.sum` churn, since the module was already resolvable in the local module cache as an existing transitive dependency (no install ran)

## Task Commits

Each task was committed atomically:

1. **Task 1: Escape and cap community-editable text, and extract the per-event-type URL helper** - `eaec279` (feat)
2. **Task 2: Group by type then collated artist, with the guest host credit and the deluxe track-count suffix** - `9f56eeb` (feat)

**Plan metadata:** commit pending (this SUMMARY + STATE.md + ROADMAP.md)

_Note: both tasks were plan-annotated `tdd="true"`. Rather than a strict RED-then-GREEN commit split, each task's implementation and its full test coverage (table tests hitting every behavior-block case) landed together in one `feat` commit -- the same TDD-annotation-without-split pattern 22-01-SUMMARY.md and 22-02-SUMMARY.md's Task 3/Task 1 already documented for this phase. Every acceptance criterion and `<verify>` command was run and confirmed passing before each commit, matching this phase's established precedent._

## Files Created/Modified
- `internal/notifier/digest_format.go` - `escapeMarkdown`/`markdownEscaper`, `digestTitleLimit`, `artistKey`, `lineLabel`, `sortDigestGroup`, and `buildDigestEmbed` rewritten to sort, escape, and append the deluxe suffix
- `internal/notifier/format.go` - new `eventURL` shared switch; `formatNewRelease`/`formatGuestFeature`/`formatDeluxeChange` now call it instead of their own dedicated URL helper
- `internal/notifier/digest_format_test.go` - new file: table tests for escaping (all eleven metacharacters + empty string), the rune cap, `eventURL`, `artistKey`, `lineLabel`, collated ordering, the title/id tie-break, shuffle invariance, the host credit, the track suffix's both-known/both-nil cases, the `watched_artist_name` fallback, fixed group order, empty-group heading omission, and non-merging duplicate-source/duplicate-artist lines
- `go.mod` - `golang.org/x/text v0.39.0` moved from the indirect block to a direct requirement

## Decisions Made
- `eventURL` was placed in `format.go` rather than `digest_format.go` -- it composes the three `musicBrainz*URL`/`newReleaseURL` leaf helpers that already live there, and `formatEmbed`'s three formatters are its primary callers alongside the digest builder.
- `truncateRunes(title, digestTitleLimit)` is applied strictly before `escapeMarkdown` in both `lineLabel` and the earlier Task 1 assembly point, per the plan's explicit ordering requirement -- the cap counts operator-visible characters, not the backslashes escaping inserts.
- `lineLabel` special-cases only `eventTypeGuestFeature` for the host-credit form; every other type falls through the same "Artist — Title" shape, keeping the function a two-branch `if` rather than a three-way switch mirroring `digestHeadings`.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed two `staticcheck` De Morgan's-law findings in test code, before the Task 2 commit**
- **Found during:** Task 2's pre-commit `golangci-lint run` (mandatory per CLAUDE.md's Definition of Done, and per the plan's own `<verify>` `go vet ./... && golangci-lint run` step)
- **Issue:** Two ordering-assertion tests (`TestBuildDigestEmbed_OrderingIsCollatedCaseAndAccentInsensitive`, `TestBuildDigestEmbed_GroupOrderFixedRegardlessOfInputOrder`) wrote `if !(a < b && b < c)`, which `staticcheck`'s `QF1001` flags as more clearly expressed via De Morgan's law
- **Fix:** Rewrote both conditions as `if a >= b || b >= c` -- same truth table, no `staticcheck` finding
- **Files modified:** `internal/notifier/digest_format_test.go`
- **Verification:** `golangci-lint run` reports `0 issues`; both tests still pass unchanged in behavior
- **Committed in:** `9f56eeb` (Task 2 commit -- caught and fixed before the commit was made, so no separate commit exists)

---

**Total deviations:** 1 auto-fixed (Rule 3, a lint-gate blocker in newly-written test code, fixed before commit).
**Impact on plan:** Purely a style fix in this plan's own new test code; no behavior change, no scope creep.

## Issues Encountered
None. `make test`'s `-race` flag remains unusable on this Windows dev box (pre-existing, documented, waived limitation -- see `.planning/WINDOWS.md` and every prior phase's SUMMARY since 05-UAT). Substituted `go test ./... -count=1 -coverprofile=coverage.out -coverpkg=$(COVER_PKGS)` (the same command `test-integration` runs, minus `-race`) for `make coverage-gate`'s input -- full suite green, `make coverage-gate` reported 91.20%, well above the 80% floor.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- `buildDigestEmbed(events []sqlc.Event) discord.Embed` (internal/notifier/digest_format.go) is unchanged in signature and call site -- plan 22-04 can drive it through the scheduler exactly as 22-02-SUMMARY.md recorded.
- Threat T-22-07 is closed: community-editable artist/title text (and the guest-feature host credit) is escaped and rune-capped before interpolation into the digest Description's markdown-parsed link labels.
- `eventURL` (format.go) is the one shared per-event-type URL switch now used by both `formatEmbed`'s three formatters and the digest builder -- any future event type only needs one new case.
- No blockers. `go build ./...`, `go vet ./...`, `golangci-lint run` (0 issues), `go test ./... -count=1` (full suite, `-race` substituted per the documented Windows limitation), `make coverage-gate` (91.20%), and `make sqlc-check` all pass clean on the final commit. `git diff -- go.sum` is empty; `git diff --exit-code main...HEAD -- internal/poller web/ internal/discord` still shows only Phase 21's pre-existing `DigestSettings.tsx` helper-text commit (`c3e5c14`), already documented as not a regression in 22-02-SUMMARY.md and confirmed here via `git log` to predate this plan.

---
*Phase: 22-scheduled-digest-send*
*Completed: 2026-09-17*

## Self-Check: PASSED
- Created file verified present: `internal/notifier/digest_format_test.go`.
- Commit hashes verified in `git log`: `eaec279`, `9f56eeb`.
- All task-level `<acceptance_criteria>` re-verified against the final commit (grep checks, `go build`/`go vet`, `golangci-lint run`, `go test` per-package and full-suite, `make coverage-gate`, `make sqlc-check`) -- all pass.
