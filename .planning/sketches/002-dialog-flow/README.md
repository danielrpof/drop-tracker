---
sketch: 002
name: dialog-flow
question: "How do paste, resolving, review, and result feel as one flow, and where does progress live?"
winner: "B"
tags: [phase-27, flow, progress]
---

# Sketch 002: Dialog Flow

## Design Question
A 50-name paste takes roughly a minute to resolve at MusicBrainz's rate of about 1 request per second. Should the user watch a progress screen during that time, or start reviewing rows as they land? The review step reuses the sketch 001 winner (segmented tabs).

## How to View
open .planning/sketches/002-dialog-flow/index.html  (click "Add from list" and paste, or use the sample list; toggle "slow MB" in the toolbar for realistic pacing)

## Variants
- **A: Live review while resolving**: the review tabs appear immediately with a thin progress bar. A "Searching" tab holds pending rows, and finished rows drop into Needs review, Ready, or Skipped as they resolve. "Stop searching" stops the searches but keeps you on review. Add stays disabled until the searches finish or are stopped.
- **B: Progress screen, then review**: a dedicated progress screen shows the count, a bar, running High/Medium/Not found/Error tallies, and the names being searched right now. Review opens when the searches finish. "Stop and review" jumps straight to review, where unsearched lines are marked Not searched.

## What to Look For
- With slow MB on, which variant makes a minute of waiting feel shorter and more under control?
- In A, are tab counts and rows moving under your cursor distracting while you try to review?
- The paste step: does the live "32 names · 1 duplicate and 2 blank lines will be skipped" helper line make it clear what will be searched? Paste more than 100 names to see the over-cap error.
- Closing mid-flow brings up the "Discard this list?" confirm (D-12). "Edit list" goes back to the textarea with your text intact.
- After Done, the new artists appear on the Watchlist behind the dialog without a reload (SC5).
