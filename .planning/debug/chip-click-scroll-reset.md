---
status: diagnosed
trigger: "UAT G-25-4: Clicking a chip label far down a 50+ artist watchlist resets window.scrollY to 0"
created: 2026-10-11T00:00:00Z
updated: 2026-10-11T00:45:00Z
goal: find_root_cause_only
---

## Current Focus

bug_class: Bohrbug (deterministic: same data + viewport -> same scrollY, reproduced on every run)
hypothesis: CONFIRMED (H2). No code scrolls the page. A chip click applies a tag filter that collapses the list, and the browser clamps scrollTop to the new, much shorter document.
next_action: none. Diagnose-only; return ROOT CAUSE FOUND to the orchestrator.
known_pattern_candidate: none (KB has no scroll/navigation entries)

reasoning_checkpoint:
  hypothesis: "scrollY drops to 0 because toggleFilterTag -> handleTagsChange filters `visible` down to the few cards carrying the tag. The document's scrollHeight falls to <= innerHeight (body is min-h-screen, so it floors at exactly innerHeight), and the browser clamps scrollTop to max(0, scrollHeight - innerHeight) = 0. React Router does not reset it: preventScrollReset is honored."
  confirming_evidence:
    - "CDP S1 (60 synthetic artists, chip on 2 cards): scrollY 8662 -> 0, scrollHeight 10021 -> 900 = innerHeight, ZERO window.scrollTo/scroll/scrollBy/scrollIntoView/focus calls"
    - "CDP S3 (same click, body min-height 20000px): scrollY 8662 -> 183. Changing only the document height removes the drop to 0, with zero programmatic calls"
    - "CDP S2 (chip on 40 cards): scrollY 8823 -> 5961 = exactly the new maxScroll. Clamped, not reset"
    - "CDP R1 (user's REAL 12-artist data, dembow chip): scrollY 1317 -> 0, scrollHeight 2435 -> 900, cards 12 -> 1, zero programmatic calls"
  falsification_test: "If RR or app code were resetting scroll, S3 would also end at 0 and the spy would log a window.scrollTo(0,0). Neither happened."
  fix_rationale: "(diagnose-only) The fix must address the list collapse, not the router: preventScrollReset is already correct and sufficient for what it covers."
  blind_spots: "Only Chromium tested (Chrome headless). Safari/Firefox clamp scrollTop the same way when content shrinks (no layout engine can scroll past content), but their scroll-anchoring behavior differs, which would only affect the partial-shrink case (S2/S3), not the drop to 0. The user's 50+ list was not available locally (local DB has 12 artists), so 60 synthetic rows stood in for it."
  candidate_causes:
    - "code: RR <ScrollRestoration> scrollTo(0,0) despite preventScrollReset. ELIMINATED (no scrollTo call; S3 non-zero)"
    - "code: programmatic focus()/scrollIntoView in TagChips/route effects. ELIMINATED (spies logged nothing)"
    - "code/design: filter collapses the list -> document shrinks below scrollY -> browser clamp. CONFIRMED"
    - "environment: browser scroll anchoring (overflow-anchor). ELIMINATED as cause (S5 with anchoring off still 0); it actually mitigates partial shrinks (S3)"
    - "data: each local tag is on exactly one artist, so any chip filter yields 1 card that fits in the viewport. CONTRIBUTING (decides 0 vs a partial drop)"
    - "spec: UI-SPEC D-12 / UAT truth models ScrollRestoration as the only scroll-to-top cause and treats preventScrollReset as sufficient. CONTRIBUTING design gap"
  and_gate: "yes, weakly. A drop to exactly 0 needs (1) no mechanism that keeps the clicked card's position when the list collapses (design gap) AND (2) a filtered result short enough to fit in the viewport (data). Without (2), the same mechanism gives a partial jump to the new bottom (S2: -2862px). (1) is the actionable root cause."

## Symptoms

expected: window.scrollY is preserved (not reset to 0) when clicking a tag chip label (button aria-label "Filter by {tag}" in web/app/components/watchlist/TagChips.tsx) on an artist card far down the Watchlist.
actual: User reported "it drops to 0 after clicking a tag" (window.scrollY becomes 0 after clicking a chip label). The `Has muted events` toggle was NOT reported as failing.
errors: none
reproduction: UAT test 4 in .planning/phases/25-find-filter-watchlist-and-history/25-UAT.md — scroll far down a 50+ artist watchlist, click a chip label.
started: discovered during phase 25 UAT

## Eliminated

- hypothesis: "H1: React Router <ScrollRestoration> calls window.scrollTo(0,0) on the chip-click navigation (preventScrollReset missing or dropped)"
  evidence: "useUrlParams.write always passes preventScrollReset: true (shared by chip and muted paths); RR 7.18.2 setSearchParams forwards it verbatim and useScrollRestoration skips scrollTo(0,0) when it is true. At runtime the window.scrollTo spy logged zero calls in S1-S5/R1, and S3 (taller document, same navigation) ended at scrollY 183, not 0."
  timestamp: 2026-10-11T00:30:00Z

- hypothesis: "H3: a programmatic focus() or scrollIntoView() (TagChips focus effects, route effects) moves the viewport"
  evidence: "HTMLElement.focus, Element.scrollIntoView and Element.scrollTo spies logged nothing during any click. The chip receives focus natively from the mouse click, which did not scroll (S3 stayed at 183)."
  timestamp: 2026-10-11T00:30:00Z

- hypothesis: "Browser scroll anchoring (overflow-anchor: auto) pulls scrollY to 0"
  evidence: "S5 with overflow-anchor:none still went 8662 -> 0. With a tall document (S3), anchoring kept the clicked chip near its viewport position, which helps rather than hurts."
  timestamp: 2026-10-11T00:30:00Z

## Evidence

- timestamp: 2026-10-11T00:00:00Z
  checked: .planning/debug/knowledge-base.md
  found: no entries about scroll, navigation, React Router, or URL params
  implication: no known-pattern candidate; open investigation

- timestamp: 2026-10-11T00:05:00Z
  checked: web/app/components/watchlist/TagChips.tsx:188-193, web/app/routes/watchlist.tsx:439-462, web/app/lib/useUrlParams.ts
  found: chip label onClick -> actions.toggleFilterTag(tag) -> handleTagsChange(ids) -> write(mutate) (push, no replace). The muted toggle -> handleFlagChange -> write(mutate) (push). useUrlParams.write ALWAYS calls setParams(next, { replace, preventScrollReset: true }).
  implication: the chip path and the (not-failing) muted path share the exact same URL writer with preventScrollReset: true. A "missing preventScrollReset" hypothesis is unlikely unless RR ignores the flag; the difference must be elsewhere (e.g. what the filter does to page height).

- timestamp: 2026-10-11T00:10:00Z
  checked: web/node_modules/react-router/dist/development/chunk-62JRHF6Z.mjs (RR 7.18.2) useSearchParams (L10830-10862) and useScrollRestoration (L11036-11126); web/app/root.tsx (<ScrollRestoration /> in Layout); web/react-router.config.ts (ssr:false framework/SPA mode)
  found: setSearchParams forwards navigateOptions verbatim to navigate("?"+params, opts). ScrollRestoration's layout effect only calls window.scrollTo(0,0) when restoreScrollPosition is not a number, there is no hash target, AND preventScrollReset !== true. WatchlistRow/TagChips have no <a>/<form> wrapper; chip button is type="button".
  implication: by code reading, RR should not be the one scrolling to 0 on a chip click. Needs a runtime check (spy on window.scrollTo) to eliminate formally.

- timestamp: 2026-10-11T00:15:00Z
  checked: 25-UI-SPEC.md:399 ("Scroll reset (D-12)") and :639, 25-RESEARCH.md Pitfall 7, 25-08-PLAN human-check; web/app/components/watchlist/WatchlistToolbar.tsx
  found: the spec's only modelled cause of "jump to top" is RR ScrollRestoration; its only mitigation is preventScrollReset. Nothing in spec/plan/research addresses the list SHRINKING under the viewport when a filter is applied. The toolbar (incl. `Has muted events`) is a static, non-sticky flex group ABOVE the list, so it can only be clicked when scrolled near the top.
  implication: (a) a design gap: filtering a 50+ list down to a few cards makes the document shorter than the current scrollY, and the browser must clamp scrollTop to max(0, scrollHeight - innerHeight); (b) explains why the muted toggle "did not fail": it is only reachable at scrollY near 0, so there is nothing to lose.

- timestamp: 2026-10-11T00:30:00Z
  checked: headless Chrome (CDP) against the running build at localhost:8080, viewport 1280x900, GET /watchlist intercepted with 60 synthetic artists (tag "rare" on cards 3+55, "common" on 40 cards, 2 muted). Spies on window.scrollTo/scroll/scrollBy, Element.scrollIntoView/scrollTo, HTMLElement.focus plus scroll events. Real mouse clicks via Input.dispatchMouseEvent. Script: scratchpad/cdp-scroll.mjs
  found: |
    S1 rare chip on card 55: scrollY 8662 -> 0; scrollHeight 10021 -> 900 (= innerHeight, body is min-h-screen); cards 60 -> 2; URL ?tags=901; programmatic scroll/focus calls: NONE; one scroll event at y=0.
    S2 common chip on card 56: scrollY 8823 -> 5961 = new maxScroll exactly (scrollHeight 10021 -> 6861); cards 60 -> 40; calls: NONE. Clamped to the new bottom, NOT reset to 0.
    S3 = S1 + body min-height 20000px: scrollY 8662 -> 183 (not 0); calls: NONE. With a tall document Chrome scroll-anchoring keeps the clicked chip near its viewport position.
    S5 = S1 + overflow-anchor:none: still 8662 -> 0; calls: NONE. Anchoring is not the cause; the height clamp is.
    S4 `Has muted events` at the deepest scroll where it is still visible: scrollY 285 -> 0 (cards 60 -> 2); calls: NONE. Same clamp, but from a tiny offset because the toolbar is not sticky.
    history.scrollRestoration = "manual" in all runs (ScrollRestoration mounted and active).
  implication: H2 CONFIRMED and H1/H3 eliminated. preventScrollReset works and nothing calls scrollTo or focus. scrollY goes to 0 because filtering shrinks the document to <= innerHeight and the browser clamps scrollTop. Changing ONLY the document height (S3) turns the same click and navigation into a non-zero scrollY.

- timestamp: 2026-10-11T00:38:00Z
  checked: same CDP harness, NO interception, the user's real data from localhost:8080/watchlist (12 artists; tags Hip-Hop, Melodic Rap, dembow each on exactly 1 artist; 0 muted). Clicked "Filter by dembow" on Sech's card scrolled into view. URL-only change in a throwaway headless profile; no API writes.
  found: scrollY 1317 -> 0; scrollHeight 2435 -> 900; cards 12 -> 1; URL ?tags=81; programmatic scroll/focus calls NONE.
  implication: reproduces the exact UAT report on real data. With every tag on one artist, any chip click leaves one card, which fits in any desktop viewport, so the drop to 0 happens every time.

- timestamp: 2026-10-11T00:40:00Z
  checked: grep -i scroll in web/app/lib/useUrlParams.test.tsx, web/app/routes/watchlist.test.tsx, web/app/components/watchlist/TagChips.test.tsx
  found: the only scroll assertion is useUrlParams.test.tsx:10 "always prevents scroll reset and defaults to push", which pins the OPTION value (preventScrollReset: true), not a scroll outcome.
  implication: why not caught. The guard verifies the router contract only; jsdom has no layout, so the collapse-and-clamp class is invisible to the suite and was left to UAT.

## Resolution

root_cause: "Filtering collapses the watchlist under the viewport, and nothing keeps the clicked card's position. A chip click (TagChips.tsx:193 -> watchlist.tsx toggleFilterTag/handleTagsChange) narrows `visible` from 50+ cards to the few carrying that tag. The document's scrollHeight drops to <= innerHeight (body is min-h-screen, so it floors at exactly innerHeight), and the browser clamps scrollTop to max(0, scrollHeight - innerHeight) = 0. React Router is NOT resetting scroll: preventScrollReset: true (useUrlParams.ts) is honored and no code calls scrollTo/scrollIntoView/focus. The design (UI-SPEC D-12, line 399) modelled <ScrollRestoration> as the only cause of a jump to the top, so preventScrollReset was the whole mitigation and the UAT truth assumed it was sufficient. `Has muted events` uses the same writer and clamps the same way (S4), but the toolbar is not sticky and only reachable near scrollY 0, so there is nothing visible to lose."
fix: (diagnose-only, not applied)
verification: (diagnose-only)
files_changed: []
