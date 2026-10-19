---
status: awaiting_human_verify
trigger: "shift+click range selection selects only clicked card (phase 26 UAT test 2)"
created: 2026-10-19T00:00:00Z
updated: 2026-10-19T01:00:00Z
goal: find_and_fix
---

## Current Focus

bug_class: Mandelbug (input-timing dependent; green in jsdom, fails in a real browser when the OS auto-repeats the held Shift keydown)
hypothesis: CONFIRMED. WatchlistRow's label onKeyDownCapture resets shiftRef on ANY keydown, including the auto-repeating keydown of the held Shift key. After mousedown on the checkbox span focuses it (inside the target label), a repeated Shift keydown bubbles through the target label's capture handler and clears shiftRef before the click fires, so handleSelectChange gets shift=false and toggles only the clicked id.
reasoning_checkpoint:
  hypothesis: "Resetting shiftRef on every keydown (including modifier keydowns, notably the auto-repeating Shift keydown) loses the pointer-down Shift flag between pointerdown and click, so a Shift+click is handled as a plain toggle"
  confirming_evidence:
    - "Real Chrome via CDP, real route: with a repeated Shift keydown (repeat=true) between mousePressed and mouseReleased on the checkbox hit area, console showed 'keydown 5 Shift true' then 'checkedChange 5 true false' (shift lost) and selection stayed at 2 of the expected 4"
    - "Same run without the repeated keydown (A, B, C, E): 'pointerdown 5 true' -> 'checkedChange 5 true true' -> range of 4 selected; label-margin path and span path both work, anchor and applyRange are correct"
    - "Windows re-sends keydown for a held Shift (JDK bug write-up cited in search); user is on Windows 11; jsdom user-event never auto-repeats so the suite stayed green"
  falsification_test: "Fix the reset to ignore modifier keydowns; the same CDP scenario D must then select the 4-card range, and a jsdom test that fires a repeat Shift keydown between pointer press and release must go red->green"
  fix_rationale: "The keydown reset exists to drop a stale pointer flag before a keyboard toggle (Space/Enter). Modifier keys never activate the checkbox, so skipping Shift/Control/Alt/Meta keeps that protection and stops the held Shift from erasing its own flag"
  blind_spots: "Could not inject real OS typematic repeat (SendInput does not auto-repeat), so the repeat premise rests on the CDP-simulated autoRepeat keydown plus the Windows behavior reference; Firefox/Safari not exercised"
  candidate_causes:
    - "code: keydown reset clobbers the Shift flag (confirmed)"
    - "code: anchor lost / applyRange wrong (eliminated: anchor=3 and applyRange spans correctly in the non-repeat runs)"
    - "environment: label-synthesized click drops shiftKey (eliminated in Chrome: INPUT click carried shiftKey=true on both label and span paths)"
    - "data: visible order differs from id order (not a bug; sorted by name, range follows visible order)"
  and_gate: "no - a single condition (repeated Shift keydown reaching the target label while the pointer is down) is sufficient; the margin path is unaffected only because focus never lands inside the label"
next_action: user re-runs Phase 26 UAT test 2 in their real browser (hold Shift, click the card checkbox area three rows below an anchor); on "confirmed fixed" commit the two web files, archive this session, append the knowledge-base entry

## Symptoms

expected: Click one card checkbox, Shift+click a card three rows down on the 44px label area -> inclusive range in visible order is selected. After a sort change the anchor resets and only the clicked card toggles.
actual: Shift+click selects only the clicked box, not the boxes between. (Part B, retained prior selection after sort change, is by design.)
errors: none reported
timeline: found in phase 26 UAT (.planning/phases/26-*/26-UAT.md test 2); 26-04 backstop
reproduction: Watchlist page; click a card checkbox, Shift+click a card a few rows down via the label area

## Eliminated

- hypothesis: applyRange / anchor handling in handleSelectChange is wrong
  evidence: in real Chrome the non-repeat runs logged handleSelectChange(5, true, shift=true, anchor=3, sel="3") and selected the full 4-card range; anchorRef survives between clicks
  timestamp: 2026-10-19T00:25:00Z

- hypothesis: the label's synthesized click path (click on the 44px label margin) loses Shift
  evidence: CDP scenario A/C/E (mouse on label margin): pointerdown shift=true, checkedChange shift=true, range selected; the synthesized INPUT click also carried shiftKey=true in Chrome
  timestamp: 2026-10-19T00:25:00Z

- hypothesis: my first CDP "failure" (only 2 selected) was a product bug
  evidence: harness error - I picked the 4th artist by mock-array order, but the list is sorted by name, so Drake and Kendrick were adjacent; re-run using visible DOM order selected the 4-card range
  timestamp: 2026-10-19T00:22:00Z

## Evidence

- timestamp: 2026-10-19T00:05:00Z
  checked: web/app/lib/bulkSelection.ts applyRange + bulkSelection.test.ts
  found: applyRange is a correct pure inclusive-span helper; unit tests cover forward/backward/no-anchor/unknown-anchor
  implication: pure logic is not the fault; the shift flag or anchor is lost upstream of applyRange

- timestamp: 2026-10-19T00:06:00Z
  checked: web/app/routes/watchlist.tsx handleSelectChange (L529-548), anchorRef effects (L252-259), filterKeyOf (L92)
  found: shift -> applyRange(order, anchorRef.current, id, next, selectedIds); anchor set to id after every change; anchor reset only on filterKey/query change or empty selection; filterKeyOf returns a string so the effect is stable across renders
  implication: anchor handling looks right for the described flow; shift=false would produce exactly the reported symptom (only clicked id toggles)

- timestamp: 2026-10-19T00:07:00Z
  checked: web/app/components/watchlist/WatchlistRow.tsx L46-77
  found: shiftRef set on label onPointerDownCapture; label onKeyDownCapture sets shiftRef=false on ANY keydown; onCheckedChange reads+clears shiftRef
  implication: pressing/holding Shift is itself a keydown; auto-repeat of a held Shift fires keydown (repeat=true) on the focused element, which after mousedown is the target's own checkbox span inside the label

- timestamp: 2026-10-19T00:08:00Z
  checked: web/app/routes/watchlist.bulk.test.tsx shiftClick helper (L377-382)
  found: test clicks the role=checkbox span via user-event with {Shift>} held; user-event never emits auto-repeat keydown
  implication: jsdom test cannot see the repeat-keydown path, which explains why the suite is green while UAT fails

- timestamp: 2026-10-19T00:09:00Z
  checked: node_modules/@base-ui/react/checkbox/root/CheckboxRoot.js (v1.7.0)
  found: span onClick -> preventDefault + dispatchClickWithModifiers(input, event); input onChange fires from the native click, details.event = nativeEvent
  implication: the click that reaches onCheckedChange carries shiftKey in Chrome; the pointerdown ref is a belt-and-braces path

- timestamp: 2026-10-19T00:24:00Z
  checked: real Chrome 155 (headless, CDP) against vite dev server with CDP-mocked API (port 5199, real Go server on 8080 untouched), temporary console.log in WatchlistRow/handleSelectChange (reverted)
  found: scenario A (label margin), B (span hit area), C (repeat keydown before press), E (label margin + repeat mid-click) -> 4 selected. Scenario D (span hit area + repeat Shift keydown between press and release) -> logs 'pointerdown 5 true', 'keydown 5 Shift true' x2, 'checkedChange 5 true false', selected = 2 (anchor + clicked only)
  implication: root cause reproduced in a real browser; focus lands inside the target label only on the span path, so repeat keydowns reach the label's capture handler there

## Resolution

root_cause: WatchlistRow's label onKeyDownCapture clears shiftRef on every keydown, including the OS auto-repeat keydown of the held Shift key. After mousedown focuses the checkbox (inside the label), a repeated Shift keydown wipes the pointer-down Shift flag before click, so onCheckedChange reports shift=false and handleSelectChange toggles only the clicked card.
fix: WatchlistRow label onKeyDownCapture now clears shiftRef only for non-modifier keys (MODIFIER_KEYS = Shift/Control/Alt/Meta). Space/Enter still void a stale pointer flag; a held Shift no longer erases its own flag. Left uncommitted in the working tree.
verification: |
  guardrail (manual, no committed mutation tooling run):
  - red/green: new test 'keeps the range when the held Shift auto-repeats mid-click' failed before the fix ('3 selected' not found) and passes after
  - mutation guard: removing the keydown reset entirely makes 'treats a keyboard toggle as plain after an abandoned Shift press' fail, so the reset is still needed and still covered
  - real Chrome 155 via CDP, same 5 scenarios before/after: D (span hit area + repeat Shift keydown mid-click) 2 selected -> 4 selected; A/B/C/E unchanged at 4
  - prettier --check clean, vitest 34 files / 782 tests pass, tsc (pnpm typecheck) clean
  - not run: Go gates (no backend change), real OS typematic repeat (SendInput does not auto-repeat), Firefox/Safari
  guardrail_verdict: accepted pending human verify
  oracle_type: specified (UAT test 2 expected: inclusive range in visible order)
files_changed:
  - web/app/components/watchlist/WatchlistRow.tsx
  - web/app/routes/watchlist.bulk.test.tsx
