---
sketch: 001
name: review-screen
question: "How do status groups, row cards, and the confirm footer hold up inside a dialog at 375px with 40 rows?"
winner: "C"
tags: [phase-27, review, dialog, mobile]
---

# Sketch 001: Review Screen

## Design Question
The Phase 27 review step shows 40 pasted lines: High, Medium, Not found, Error, Not searched, already watched, and duplicate rows. CONTEXT D-14..D-19 fix the row card and the grouping. This sketch tests how the groups should behave so the user's attention goes to the rows that need a decision.

## How to View
open .planning/sketches/001-review-screen/index.html  (starts at 375px; use the toolbar for 768 and 1280)

## Variants
- **A: Flat sections**: all three groups are expanded, and their headers stick while you scroll.
- **B: Ready collapsed**: only Needs review is open. Ready and Skipped show a one-line summary of names until you expand them.
- **C: Segmented tabs**: the three groups are switched with a segmented control, so only one is visible at a time.

## What to Look For
- At 375px, can you get through the Medium rows without losing track of the "N selected" count and the Add button?
- Does B's collapsed Ready group feel safe? You're adding 25 artists you never looked at.
- Does C hide too much? The Skipped tab is easy to never open.
- Things to try: line 1 ("ye") starts disabled as a duplicate of the High "Kanye West" row. Switch its pick to YE (Korean band) and watch it become selectable. Retry the Error rows. Pick an alternate on "Future" (line 8).
- Rows stay in their initial group while you edit them. Only the badges and notes change. Check that this doesn't feel stale.

## Deviation from CONTEXT D-16 (confirmed: selected row wins, D-16 amended)
The sketch only blocks a row as a duplicate when another **selected** row picks the same artist. D-16 says the first row in paste order wins. Under D-16 literally, an unselected Medium "ye" on line 1 that happens to pick Kanye West would disable the High "Kanye West" row further down. Here the selected row wins instead.

## Implementation notes
The candidate picker is a native `<select>` here. The real build uses the vendored base-ui `Select` (`web/app/components/ui/select.tsx`).
