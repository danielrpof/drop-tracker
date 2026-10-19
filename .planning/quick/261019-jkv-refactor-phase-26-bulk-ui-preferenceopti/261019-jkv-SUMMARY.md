---
status: complete
---
# Quick 261019-jkv: phase 26 bulk UI cleanup

Four refactor commits on refactor/phase-26-bulk-cleanup, no behavior change; all 793 web tests pass, existing integration tests unedited.

- preferenceOptions.ts shared by the card and bulk bar; PreferencesDelta defined once in api.ts.
- BulkPreferencesPopover and useBarClearance extracted (BulkActionBar 585 -> 330 lines).
- bulkOutcomeMessages.ts: pure toast data with table tests.
- useBulkSelection.ts: selection, remove confirm, handlers, Undo, focus (watchlist.tsx 1065 -> 711 lines).
- CONTEXT.md: bulk selection, effective selection, bulk outcome.

Not done: embedded SPA bundle (internal/webassets) not rebuilt; integration cases duplicated by the new pure tests were kept, not deleted.
