# Quick 261019-jkv: phase 26 bulk UI cleanup

Source: architecture review + grilling (candidates 1 and 2). Branch refactor/phase-26-bulk-cleanup.
Each step is one commit; gate = prettier --write/--check, pnpm test (web only, no Go changes).

1. lib/preferenceOptions.ts: shared option lists + EVENT_TYPE_ORDER; PreferencesDelta defined once in api.ts.
2. BulkPreferencesPopover.tsx + lib/useBarClearance.ts extracted from BulkActionBar.tsx.
3. lib/bulkOutcomeMessages.ts: pure toast data + table tests; route switched over.
4. lib/useBulkSelection.ts: selection, remove-confirm, handlers, Undo, focus out of routes/watchlist.tsx.
Comments in touched files: 1-3 lines, one design-doc ref, no opaque tags.
CONTEXT.md: add bulk selection, effective selection, bulk outcome.
Existing integration tests stay unedited and green.
