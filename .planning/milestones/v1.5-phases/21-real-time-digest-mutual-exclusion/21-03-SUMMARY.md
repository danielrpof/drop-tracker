---
phase: 21-real-time-digest-mutual-exclusion
plan: 03

subsystem: ui
tags: [react, tsx, vitest, digest, copy, spa-bundle]

requires:
  - phase: 21-real-time-digest-mutual-exclusion
    provides: "Plans 21-01/21-02 implement the actual standdown/flush behavior in internal/notifier this copy documents; no code coupling, purely narrative sequencing"
provides:
  - "Always-visible operator-facing helper text under the Digest mode row in DigestSettings.tsx, stating the standdown (DGST-13) and toggle-off flush (DGST-14) in plain language"
  - "Refreshed internal/webassets/build/client/ committed SPA bundle carrying the new copy, so a Node-less clone's go:embed serves it"
affects: []

actuals:
  tokens: 1050
  tasks: 2
  commits: 2

tech-stack:
  added: []
  patterns:
    - "Static JSX string rendered via a single-line expression container ({\"...\"}) instead of a bare JSX text child, so prettier's 80-col wrap cannot split the sentence across source lines and defeat a literal grep"

key-files:
  created: []
  modified:
    - web/app/components/system/DigestSettings.tsx
    - web/app/components/system/DigestSettings.test.tsx
    - internal/webassets/build/client/index.html
    - internal/webassets/build/client/assets/system-C6s0fhiM.js
    - internal/webassets/build/client/assets/manifest-056e0136.js

key-decisions:
  - "Rendered the sentence as {\"While on, ...\"} (a single-line JSX expression container) rather than a bare multi-line JSX text child -- the sentence is 93 characters and cannot fit under the project's 80-column prettier printWidth as plain text, and the plan's own acceptance criterion (`grep -c '...' DigestSettings.tsx` must print 1) requires the literal string to survive on one source line. React renders both forms identically; no interpolation, no template literal, no settings value is injected either way."

patterns-established:
  - "Long static UI copy that must remain grep-matchable in source: wrap it in a JSX expression container as a single string literal rather than a bare text child, since prettier does not split inside string tokens the way it splits JSX text runs."

requirements-completed: [DGST-13, DGST-14]

coverage:
  - id: D1
    description: "The /system Digest notifications card shows the exact helper sentence under the Digest mode row, unconditionally (both toggle positions, before any interaction, not gated on SaveStatus)"
    requirement: DGST-14
    verification:
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#shows the digest helper text under the Digest mode row when digest mode is off"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#still shows the digest helper text when digest mode is on -- it is guidance, not a state indicator"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#renders the digest helper text immediately, before any interaction or save"
        status: pass
      - kind: unit
        ref: "web/app/components/system/DigestSettings.test.tsx#keeps the Digest mode switch reachable by its accessible name after the helper text is added"
        status: pass
    human_judgment: false
  - id: D2
    description: "The committed SPA bundle under internal/webassets/build/client carries the new copy, so a clone that has never run the Node toolchain serves a server whose embedded UI matches source"
    verification:
      - kind: other
        ref: "grep -rl 'switching back off delivers them individually' internal/webassets/build/client -> internal/webassets/build/client/assets/system-C6s0fhiM.js"
        status: pass
      - kind: other
        ref: "go build ./... && go vet ./... against the refreshed embed"
        status: pass
    human_judgment: false

duration: ~10min
completed: 2026-09-16
status: complete
---

# Phase 21 Plan 03: Digest Standdown Helper Text Summary

**Always-visible helper text under the `/system` Digest mode row explaining the standdown and toggle-off flush, plus a refreshed committed SPA bundle carrying the copy.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-09-16T16:47:00Z
- **Completed:** 2026-09-16T16:56:38Z
- **Tasks:** 2
- **Files modified:** 5 (2 source, 3 under the regenerated bundle tree)

## Accomplishments
- The `/system` Digest notifications card now states, in one sentence directly under the Digest mode switch, that turning digest mode on queues new events and turning it back off delivers them individually — the only place in the product that names the toggle-off flush behavior to an operator.
- The sentence renders unconditionally: present with digest mode off, present with digest mode on, present before any interaction, and untouched by the `SaveStatus` state machine.
- The Digest mode `<dd>` was restructured into a column (switch row + helper paragraph) while the `aria-labelledby="digest-mode-label"` association — and therefore `getByRole("switch", { name: "Digest mode" })` — survived unchanged.
- Four new tests pin the sentence via a single module-level `const`, covering both toggle positions, immediate presence, and switch accessibility post-restructure; the full 17-test file and the 229-test frontend suite are green.
- `internal/webassets/build/client/` was regenerated via `make web` so a clone that never runs the Node toolchain now embeds a server serving the new copy; `go build`/`go vet` are clean against the refreshed tree and `web/pnpm-lock.yaml`/`web/package.json` are untouched.

## Task Commits

Each task was committed atomically:

1. **Task 1: Always-visible digest helper text under the Digest mode row** - `ac38883` (feat)
2. **Task 2: Refresh the committed SPA bundle so the embedded UI carries the new copy** - `ee5bb83` (docs)

**Plan metadata:** commit pending (docs: complete plan)

## Files Created/Modified
- `web/app/components/system/DigestSettings.tsx` - Digest mode `<dd>` restructured to `flex flex-col` (inner `flex items-center` div holds the Switch + On/Off span unchanged); added the unconditional helper `<p>` carrying the exact D-06 sentence as a single-line JSX expression container
- `web/app/components/system/DigestSettings.test.tsx` - `DIGEST_HELPER_TEXT` module-level const plus four new tests (off, on, pre-interaction, switch accessibility)
- `internal/webassets/build/client/index.html`, `internal/webassets/build/client/assets/system-C6s0fhiM.js`, `internal/webassets/build/client/assets/manifest-056e0136.js` - regenerated Vite output from `make web`, replacing the previous content-hashed `system-C5ZXJ29n.js`/`manifest-03347db6.js`

## Decisions Made
- Rendered the sentence via `{"While on, new events wait for the next digest; switching back off delivers them individually."}` — a single-line JSX expression container — rather than a bare JSX text child. The sentence is 93 characters and the project's prettier `printWidth` is 80, so a plain text child is always wrapped across two source lines by `prettier --write`. The plan's own acceptance criterion requires `grep -c '...' DigestSettings.tsx` to print `1`, which a wrapped text child cannot satisfy. Prettier does not split inside a string literal, so wrapping the same static string in an expression container keeps it on one source line while rendering identically in the DOM — no interpolation, no template literal, and no settings value crosses into the copy either way, so this satisfies the plan's underlying intent (T-21-11) even though the literal JSX form differs from the plan's exact phrasing ("plain JSX text child").

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Bare JSX text child would fail the plan's own grep acceptance criterion**
- **Found during:** Task 1, after running `prettier --write` per the action step
- **Issue:** The plan's action text specifies writing the sentence "as a plain JSX text child," but the 93-character sentence exceeds the project's 80-column prettier `printWidth` at the `<p>`'s indentation depth. `prettier --write` (mandated by the same task, and by CLAUDE.md's Definition of Done) wrapped the text across two lines, which made `grep -c 'switching back off delivers them individually' DigestSettings.tsx` print `0` instead of the `1` the plan's own acceptance criteria require.
- **Fix:** Wrapped the same static string literal in a JSX expression container (`{"..."}`) instead of a bare text child. Prettier does not split inside string tokens, so the sentence stays on one source line after formatting. No interpolation, no template literal, no `dangerouslySetInnerHTML` — the prohibitions the plan actually cares about are still satisfied; only the syntactic form of "plain text" changed.
- **Files modified:** web/app/components/system/DigestSettings.tsx
- **Verification:** `grep -c 'switching back off delivers them individually' web/app/components/system/DigestSettings.tsx` prints `1`; `prettier --check` clean; all 17 component tests and the full 229-test frontend suite pass.
- **Committed in:** `ac38883` (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug — a plan-instruction/tooling conflict, not a logic change)
**Impact on plan:** No scope change and no behavior difference; the rendered text, DOM structure, and all prohibitions (no interpolation, no dangerouslySetInnerHTML, no wording drift) are exactly as specified. Only the JSX syntax used to hold the static string changed, needed to make the plan's own automated grep verification pass under this project's prettier config.

## Issues Encountered
None beyond the deviation above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 21's three plans (gate, staleness/logging, operator copy) are all complete. Per the ROADMAP's "Deploy sequencing" note, this phase's branch is not merged to `main` until Phase 22 (the digest sender) is also ready — no PR opened from this plan.
- The committed SPA bundle is a faithful rebuild of `web/` as of this plan; Phase 7's Docker image will still regenerate it from source independently at image build time.
- No blockers.

## Self-Check: PASSED

Both modified source files and the three regenerated bundle files confirmed present on disk; both task commit hashes (`ac38883`, `ee5bb83`) confirmed present in `git log --oneline --all`.

---
*Phase: 21-real-time-digest-mutual-exclusion*
*Completed: 2026-09-16*
