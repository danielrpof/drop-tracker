# Phase 24: Artist Tags & Notes - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-09-22
**Phase:** 24-artist-tags-notes
**Areas discussed:** Card layout & editing, Tag management surface, Lifetimes & counts, Limits & confirm UX

---

## Card layout & editing

| Option | Description | Selected |
|--------|-------------|----------|
| Under the name | Chips wrap below name inside the name column | ✓ |
| Full-width strip below row | Separate strip under the whole row | |
| You decide | Let the UI phase pick | |

| Option | Description | Selected |
|--------|-------------|----------|
| '+ tag' button | Opens autocomplete in place | ✓ |
| Always-visible input | Chip input on every row | |
| Edit mode per row | Edit button exposes chip input + note editor | |

| Option | Description | Selected |
|--------|-------------|----------|
| Inline, clamped to 2 lines | Muted text with "more" | ✓ |
| Icon + tooltip/popover | Hidden until hovered/clicked | |
| Full text always | Up to 500 chars visible | |

| Option | Description | Selected |
|--------|-------------|----------|
| In place, explicit Save/Cancel | Textarea + counter | ✓ |
| In place, save on blur | Fewer clicks, accidental saves | |
| Popover editor | Anchored popover | |

| Option | Description | Selected |
|--------|-------------|----------|
| Optimistic + rollback | Matches PreferenceToggles | ✓ |
| Wait for server | Pending state until confirmed | |

| Option | Description | Selected |
|--------|-------------|----------|
| Nothing yet | Filtering arrives in Phase 25 | ✓ |
| Filter by that tag | Pulls Phase 25 forward | |

---

## Tag management surface

| Option | Description | Selected |
|--------|-------------|----------|
| 'Manage tags' dialog on Watchlist | Button in Watchlist header | ✓ |
| Section on /system | Card next to Digest Settings | |
| New /tags route | Fourth top-level tab | |

| Option | Description | Selected |
|--------|-------------|----------|
| Name + artist count | Rename/Delete actions, sorted by name | ✓ |
| Name + count + artist names | Expandable artist list | |
| Name only | Count only in delete confirm | |

| Option | Description | Selected |
|--------|-------------|----------|
| Inline edit | Collision opens merge confirm | ✓ |
| Small rename dialog | Nested dialog | |

---

## Lifetimes & counts

| Option | Description | Selected |
|--------|-------------|----------|
| Note deleted with the watchlist row | Re-add starts blank | ✓ |
| Note kept like tags | Stored on the artist | |

| Option | Description | Selected |
|--------|-------------|----------|
| Watched artists only | Removed artists' links still deleted, not counted | ✓ |
| Watched + removed, split | "12 watched (+3 removed)" | |

| Option | Description | Selected |
|--------|-------------|----------|
| Stays until explicitly deleted | Shows "0 artists" | ✓ |
| Auto-removed when unused | Less clutter, surprising on re-add | |

| Option | Description | Selected |
|--------|-------------|----------|
| All tags in the vocabulary | Autocomplete = Manage tags list | ✓ |
| Only tags on watched artists | Current watchlist only | |

---

## Limits & confirm UX

| Option | Description | Selected |
|--------|-------------|----------|
| Hide '+ tag', show max hint | Restored when a chip is removed | ✓ |
| Keep '+ tag', error on submit | Discoverable but wasted work | |

| Option | Description | Selected |
|--------|-------------|----------|
| Hard stop + live counter | maxLength 32, counter near limit | ✓ |
| Allow typing, inline error | Blocks submit | |

| Option | Description | Selected |
|--------|-------------|----------|
| Modal alert dialog | base-ui AlertDialog, reusable in Phase 26 | ✓ |
| Inline confirm in the row | No modal-in-modal | |

| Option | Description | Selected |
|--------|-------------|----------|
| Toast + refresh cards | No undo | ✓ |
| Toast with Undo | Needs restore logic | |

---

## Claude's Discretion

- DB mechanism for the 10-tags-per-artist cap (trigger vs. row-locking guarded insert)
- Rename-collision API shape and tag/notes endpoint shapes
- Counter threshold, copy, and chip styling (UI phase)
- Enrichment query form for `GET /watchlist`

## Deferred Ideas

- Click a chip to filter the Watchlist — Phase 25
