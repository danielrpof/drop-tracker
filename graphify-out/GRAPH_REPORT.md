# Graph Report - drop-tracker  (2026-10-19)

## Corpus Check
- 226 files · ~242,404 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1784 nodes · 3773 edges · 128 communities (75 shown, 17 thin omitted)
- Extraction: 94% EXTRACTED · 6% INFERRED · 0% AMBIGUOUS · INFERRED: 212 edges (avg confidence: 0.83)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- writeError
- 26 RESEARCH doc
- .HandleLogin
- watchlistView.ts
- main.go
- parse.go
- github.com/danielrpof/drop-tracker/internal/db/sqlc.Event
- 24 CONTEXT doc
- combobox.tsx
- deezer/client.go
- format.ts
- Phase 25 Research
- In-process poll-run ring buffer
- useBulkSelection.ts
- notifier.go
- DigestSettings.tsx
- migrate.go
- BulkPreferencesPopover.tsx
- match.go
- service.go
- TagChips.tsx
- history.tsx
- system.tsx
- api.ts
- 25 UI SPEC doc
- SearchResultsColumns.tsx
- root.tsx
- poller.go
- TESTING doc
- EventCard.tsx
- ManageTagsDialog
- service.go (2)
- ROADMAP doc
- TagChips.tsx (2)
- artist_tags
- full-pipeline.yml
- CONCERNS doc
- dialog.tsx
- alert-dialog.tsx
- server.go
- CONTEXT doc
- detector.go
- settings.go
- search.go
- STRUCTURE doc
- v1.0 MILESTONE AUDIT doc
- ROADMAP doc (2)
- musicbrainz.go
- REQUIREMENTS doc
- useBulkSelection.ts (2)
- INTEGRATIONS doc
- status.go
- musicbrainz.go (2)
- STATE doc
- STACK doc
- WatchlistBulkStore
- bulk.go
- tags.go
- recordings.go
- logging.go
- releases.go
- CONTEXT doc (2)
- artist_lookup.go
- terraform always on env doc
- activity.go
- backfill.go
- alerter.go
- CONTEXT doc (3)
- schema_version.go
- server.go (2)
- settings.go (2)
- releasegroups.go
- HistoryFilters.tsx
- pnpm workspace doc
- CONTEXT doc (4)
- CLAUDE doc
- CLAUDE doc (2)
- issue tracker doc
- health.go
- 000008_notification_settings.up.sql
- parse.go (2)
- MILESTONES doc
- 2026 09 08 feature module doc
- docker-compose.yml
- github.com/danielrpof/drop-tracker
- Feature/Module Ideas Post-v1.3
- Option C: Operator Status Panel +
- Option E: Watchlist Organization
- Phase 25 External API Coverage Declaration
- In-process poll-run ring buffer (2)
- Design Grilling Before Planning
- Don't Roadmap Phases Gated on Uncontrolled

## God Nodes (most connected - your core abstractions)
1. `cn()` - 115 edges
2. `run()` - 45 edges
3. `Watchlist()` - 36 edges
4. `Phase 26 Bulk Edit and Remove Research` - 30 edges
5. `Phase 26 UI Design Contract` - 28 edges
6. `WatchlistEntry` - 26 edges
7. `Phase 25 UI Design Contract` - 26 edges
8. `writeError()` - 24 edges
9. `Phase 24: Artist Tags & Notes` - 24 edges
10. `Testing Patterns` - 21 edges

## Surprising Connections (you probably didn't know these)
- `Comment Discipline Rule` --semantically_similar_to--> `Coding Conventions`  [INFERRED] [semantically similar]
  .claude/CLAUDE.md → .planning/codebase/CONVENTIONS.md
- `pre-commit config (gitleaks, golangci-lint --fix, prettier --write)` --semantically_similar_to--> `gitleaks job`  [EXTRACTED] [semantically similar]
  .pre-commit-config.yaml → .github/workflows/full-pipeline.yml
- `scanFile()` --calls--> `Parse()`  [EXTRACTED]
  cmd/migration-check/main.go → internal/sqlscan/parse.go
- `crossReferenceFinding()` --references--> `RefSet`  [EXTRACTED]
  cmd/migration-check/main.go → internal/sqlscan/queryrefs.go
- `run()` --calls--> `NewActivityGate()`  [EXTRACTED]
  cmd/server/main.go → internal/artistart/activity.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Bulk tag add cap-safety mechanism** — docs_adr_0004_per_artist_tag_cap_trigger_cap_trigger, docs_adr_0004_per_artist_tag_cap_trigger_bulk_prelock, planning_phases_26_bulk_edit_remove_26_context_apply_where_fits [EXTRACTED 0.95]
- **Digest scheduling vocabulary** — context_digest_slot, context_slot_record, context_grace_window, context_digest_window, context_watermark [EXTRACTED 0.95]
- **Tag data model** — context_tag, context_tag_vocabulary, context_tag_link, context_carrier, context_merge [EXTRACTED 0.95]
- **Bulk selection and action UI** — planning_phases_26_bulk_edit_remove_26_ui_spec_derived_selection_effective_selection, planning_phases_26_bulk_edit_remove_26_ui_spec_select_all_visible_tri_state, planning_phases_26_bulk_edit_remove_26_ui_spec_sticky_bulk_action_bar, planning_phases_26_bulk_edit_remove_26_ui_spec_success_focus_order_and_fallback_chain [EXTRACTED 1.00]
- **Bulk tag add safe-insert flow** — planning_phases_26_bulk_edit_remove_26_research_bulk_tag_add_apply_where_it_fits_d_01, planning_phases_26_bulk_edit_remove_26_research_ordered_pre_lock_then_guarded_insert, planning_phases_26_bulk_edit_remove_26_research_adr_0004_tag_cap_trigger, planning_phases_26_bulk_edit_remove_26_research_tag_orphan_on_all_capped_create [EXTRACTED 1.00]
- **Release-path gate jobs feeding build-scan then release** — github_workflows_full_pipeline_lint, github_workflows_full_pipeline_test, github_workflows_full_pipeline_gitleaks, github_workflows_full_pipeline_trivy_fs, github_workflows_full_pipeline_frontend_test, github_workflows_full_pipeline_migration_check, github_workflows_full_pipeline_n1_boot, github_workflows_full_pipeline_build_scan, github_workflows_full_pipeline_release [EXTRACTED 1.00]
- **Phase 28 safety work: inert seam, escaping, fixture preconditions** — planning_roadmap_tagsreader, planning_roadmap_escapemd, planning_roadmap_chunkfixture, planning_roadmap_phase28 [EXTRACTED 1.00]
- **N-1 rollback safety enforcement** — internal_db_migrations_readme_n1_invariant, internal_db_migrations_readme_n1_boot_job, internal_db_migrations_readme_ahead_of_source_test, internal_db_migrations_readme_migration_check, internal_db_migrations_readme_expand_backfill_contract [EXTRACTED 1.00]
- **Single-instance in-process coordination decisions** — docs_adr_0001_in_process_ring_buffer_for_poll_run_history_ring_buffer, docs_adr_0002_one_outbox_one_sender_lock_notifier_notifying_lock, docs_adr_0001_in_process_ring_buffer_for_poll_run_history_single_instance_constraint, docs_adr_0002_one_outbox_one_sender_lock_postgres_advisory_lock [EXTRACTED 1.00]
- **v1.6 tags-first dependency chain (Phases 24-26)** — planning_roadmap_phase24, planning_roadmap_phase25, planning_roadmap_phase26, planning_roadmap_phase27, planning_roadmap_phase28 [EXTRACTED 1.00]
- **Watchlist filter toolbar composition** — planning_phases_25_find_filter_watchlist_and_history_25_ui_spec_name_filter, planning_phases_25_find_filter_watchlist_and_history_25_ui_spec_sort_select_six_options, planning_phases_25_find_filter_watchlist_and_history_25_ui_spec_tag_filter_any_of_combobox, planning_phases_25_find_filter_watchlist_and_history_25_ui_spec_preference_toggles_filter, planning_phases_25_find_filter_watchlist_and_history_25_ui_spec_clear_filters [EXTRACTED 1.00]
- **Local hooks mirroring CI gates** — pre_commit_config, golangci, claude_claude_definition_of_done, github_workflows_full_pipeline [INFERRED 0.85]
- **Watchlist find/filter composition (search, sort, tags, toggles)** — planning_phases_25_find_filter_watchlist_and_history_25_research_pattern_selectvisible [INFERRED 0.85]
- **Manage tags rename/merge/delete UI flow** — planning_phases_24_artist_tags_notes_24_ui_spec_managetagsdialog, planning_phases_24_artist_tags_notes_24_ui_spec_confirmdialog, planning_phases_24_artist_tags_notes_24_context_d_09_rename_merge_confirm, planning_phases_24_artist_tags_notes_24_context_d_16_reusable_alert_dialog, planning_phases_24_artist_tags_notes_24_context_d_22_vocabulary_api, planning_phases_24_artist_tags_notes_24_ui_spec_focus_management_r5 [INFERRED 0.85]
- **Per-artist 10-tag cap enforcement** — planning_phases_24_artist_tags_notes_24_context_d_18_cap_trigger, planning_phases_24_artist_tags_notes_24_context_d_19_merge_never_inserts, planning_phases_24_artist_tags_notes_24_context_adr_0004_cap_trigger, planning_phases_24_artist_tags_notes_24_research_before_insert_trigger_before_on_conflict, planning_phases_24_artist_tags_notes_24_research_for_no_key_update_lock [INFERRED 0.85]
- **Poll-run history to /status to SPA flow** — docs_adr_0001_in_process_ring_buffer_for_poll_run_history_runrecorder_seam, docs_adr_0001_in_process_ring_buffer_for_poll_run_history_ring_buffer, docs_api_status_contract_pollruns_snapshot, docs_api_status_contract_handlestatus, docs_api_status_contract_spa_api_client [INFERRED 0.85]
- **Tag integrity: normalization, unique index, cap trigger** — planning_roadmap_tagidentity, planning_roadmap_migration10, planning_roadmap_captrigger, planning_roadmap_migration1112 [INFERRED 0.85]

## Communities (128 total, 17 thin omitted)

### Community 0 - "writeError"
Cohesion: 0.07
Nodes (51): Cursor, cursorWireForm, ListParams, Page, Service, github.com/danielrpof/drop-tracker/internal/db/sqlc.ListEventsRow, net/http.Request, net/http.ResponseWriter (+43 more)

### Community 1 - "26 RESEARCH doc"
Cohesion: 0.06
Nodes (66): ADR 0004 tag cap trigger, Bulk remove-tag by tag id (D-22), Bulk tag add apply-where-it-fits (D-01), ConfirmDialog close-order pitfall, Deadlock-free lock ordering, Derived selection (D-11), Grill amendments Q1-Q10, Id count and body bounds (+58 more)

### Community 2 - ".HandleLogin"
Cohesion: 0.07
Nodes (38): globalCounter, ipLimiter, loginRequest, loginThrottle, Token, sync.Mutex, sync.Once, time.Ticker (+30 more)

### Community 3 - "watchlistView.ts"
Cohesion: 0.07
Nodes (55): addWatchlist(), removeWatchlist(), sortTagsByName(), anchorCandidates(), useScrollAnchor(), rewriteTags(), byId(), canonicalWatchlistParams() (+47 more)

### Community 4 - "main.go"
Cohesion: 0.07
Nodes (54): appendFile(), backendTotalPct(), formatDelta(), frontendLinesPct(), main(), parseBlockLine(), readBackend(), readFrontend() (+46 more)

### Community 5 - "parse.go"
Cohesion: 0.07
Nodes (41): buildPrevReleaseRefs(), strings.Builder, NormalizeIdent(), StripIdent(), StripSchemaQualifier(), copyDollarQuoted(), copySingleQuoted(), dollarTagAt() (+33 more)

### Community 6 - "github.com/danielrpof/drop-tracker/internal/db/sqlc.Event"
Cohesion: 0.09
Nodes (43): discordAlerter, allowedMentions, EmbedImage, retry429Body, webhookPayload, github.com/danielrpof/drop-tracker/internal/db/sqlc.Event, golang.org/x/text/collate.Collator, Client (+35 more)

### Community 7 - "24 CONTEXT doc"
Cohesion: 0.06
Nodes (48): Phase 24 Context: Artist Tags & Notes, ADR 0004: Per-artist tag cap trigger, D-03: Optimistic chip add/remove with rollback, D-05: Note clamped to 2 lines with more expander, D-06: Note edited in place with Save/Cancel and counter, D-07: Manage tags dialog from Watchlist header, D-09: Inline rename; collision opens merge confirmation, D-10: Notes on watchlist row, tags on artists.id (+40 more)

### Community 8 - "combobox.tsx"
Cohesion: 0.08
Nodes (31): Button(), buttonVariants, ComboboxChip(), ComboboxChips(), ComboboxChipsInput(), ComboboxClear(), ComboboxContent(), ComboboxEmpty() (+23 more)

### Community 9 - "deezer/client.go"
Cohesion: 0.07
Nodes (28): Artist, AlbumLister, APIError, artistAlbumsResponse, errorProbe, context.CancelFunc, golang.org/x/time/rate.Limiter, io.ReadCloser (+20 more)

### Community 10 - "format.ts"
Cohesion: 0.10
Nodes (37): EventCard(), DigestSettings(), handleCadenceChange(), handleDigestModeChange(), save(), classifyOutcome(), OutcomeBadge(), OutcomeTier (+29 more)

### Community 11 - "Phase 25 Research"
Cohesion: 0.06
Nodes (38): Bulk outcome, Bulk selection, Effective selection, Filter, Search, ADR 0004: per-artist tag cap trigger, Ordered pre-lock for bulk attach, Locking skip-existing tag cap trigger (artist_tags) (+30 more)

### Community 12 - "In-process poll-run ring buffer"
Cohesion: 0.06
Nodes (40): ADR 0001: In-process ring buffer for poll-run history, poll_runs table (rejected option), In-process poll-run ring buffer (pollruns.Store), poller.RunRecorder seam, Single-instance constraint (no leader election in robfig/cron), httpserver.StatusStore seam, ADR 0002: One outbox, one sender lock, CAS-skip on held lock (+32 more)

### Community 13 - "useBulkSelection.ts"
Cohesion: 0.12
Nodes (35): BulkActionBarProps, RemoveTagPicker(), BulkPreferencesPopoverProps, DISMISS_REASONS, BulkAddTagOutcome, BulkAxisDelta, BulkOutcome, BulkRemoveTagOutcome (+27 more)

### Community 14 - "notifier.go"
Cohesion: 0.13
Nodes (24): Cadence, context.Context, github.com/danielrpof/drop-tracker/internal/db/sqlc.Querier, log/slog.Logger, NewClient(), ackDigestBatch(), ackEventsOnly(), Notifier (+16 more)

### Community 15 - "DigestSettings.tsx"
Cohesion: 0.12
Nodes (28): Cadence, CADENCE_LABELS, DigestSettingsProps, SaveStatus, Avatar(), AvatarBadge(), AvatarFallback(), AvatarGroup() (+20 more)

### Community 16 - "migrate.go"
Cohesion: 0.12
Nodes (30): main(), run(), retryConfig, RetryOption, github.com/golang-migrate/migrate/v4/source.Driver, github.com/jackc/pgx/v5/pgxpool.Config, github.com/jackc/pgx/v5/pgxpool.Pool, testing.T (+22 more)

### Community 17 - "BulkPreferencesPopover.tsx"
Cohesion: 0.10
Nodes (27): Popover(), PopoverContent(), PopoverDescription(), PopoverHeader(), PopoverTitle(), PopoverTrigger(), BulkPreferencesPopover(), changeOpen() (+19 more)

### Community 18 - "match.go"
Cohesion: 0.17
Nodes (20): AlbumLister, ArtistDetailLookup, ArtistFetcher, ArtistSearcher, Matcher, Option, ReleaseGroupLister, Result (+12 more)

### Community 19 - "service.go"
Cohesion: 0.11
Nodes (17): github.com/danielrpof/drop-tracker/internal/db/sqlc.DBTX, github.com/danielrpof/drop-tracker/internal/db/sqlc.Queries, tagCollisionResponse, BulkSkip, Service, goneIDs(), NormalizeName(), Service (+9 more)

### Community 20 - "TagChips.tsx"
Cohesion: 0.12
Nodes (23): Textarea(), ArtistNote(), handleChange(), handleSave(), openEditor(), ArtistNoteProps, FocusRequest, saveErrorMessage() (+15 more)

### Community 21 - "history.tsx"
Cohesion: 0.13
Nodes (22): EmptyState(), EmptyStateProps, ComboboxOption, ComboboxProps, EVENT_TYPE_OPTIONS, HistoryFilters(), HistoryFiltersProps, HistoryFiltersValue (+14 more)

### Community 22 - "system.tsx"
Cohesion: 0.13
Nodes (23): AboutInstance(), AboutInstanceProps, Alert(), AlertAction(), AlertDescription(), AlertTitle(), alertVariants, Card() (+15 more)

### Community 23 - "api.ts"
Cohesion: 0.13
Nodes (22): ApiError, apiFetch(), BulkRemoveOutcome, bulkRemoveTag(), bulkRemoveWatchlist(), BulkSkip, bulkUpdatePreferences(), deleteTag() (+14 more)

### Community 24 - "25 UI SPEC doc"
Cohesion: 0.15
Nodes (26): 24px dense target exception, Accent and case folding match, Clear filters, Click-to-filter tag chips, Client-side filtering over single payload, Combobox hardening max-h-72, ComboboxChip vendored edit, Dark-only design system base-maia (+18 more)

### Community 25 - "SearchResultsColumns.tsx"
Cohesion: 0.12
Nodes (21): SearchBox(), handleChange(), runSearch(), SearchBoxProps, DEEZER_ADD_HINT, SearchResultRow(), SearchResultRowProps, SearchResultsColumns() (+13 more)

### Community 26 - "root.tsx"
Cohesion: 0.10
Nodes (10): PassphraseScreen(), handleSubmit(), Toaster(), createSession(), deleteSession(), authStore, gateActive, listeners (+2 more)

### Community 27 - "poller.go"
Cohesion: 0.19
Nodes (16): cron.Cron, sync/atomic.Bool, New(), runOneArtist(), WithDeezerWorkers(), WithMusicBrainzWorkers(), WithRunRecorder(), AlbumSource (+8 more)

### Community 28 - "TESTING doc"
Cohesion: 0.14
Nodes (22): Co-located _test package convention, Concurrent testing with WaitGroup, Error sanitization test, Frontend test framework absent, Func-field stubs, Go stdlib testing runner, httptest.Server API mocking, Inline string-constant fixtures (+14 more)

### Community 29 - "EventCard.tsx"
Cohesion: 0.13
Nodes (14): CoverArt(), CoverArtProps, EVENT_BADGE, EventCardProps, GuestFeatureBody(), guestFeatureHref(), UNKNOWN_EVENT_BADGE, watchlistNote() (+6 more)

### Community 30 - "ManageTagsDialog"
Cohesion: 0.13
Nodes (14): BulkActionBar(), commitAddTag(), commitRemoveTag(), exitInline(), openAddTag(), ManageTagsDialog(), handleConfirmDelete(), handleConfirmMerge() (+6 more)

### Community 31 - "service.go (2)"
Cohesion: 0.21
Nodes (14): github.com/danielrpof/drop-tracker/internal/db/sqlc.ListWatchlistRow, entryFromRow(), Entry, Service, NewService(), NormalizeNote(), normalizeSet(), WithArtistArt() (+6 more)

### Community 32 - "ROADMAP doc"
Cohesion: 0.14
Nodes (20): Notes requirement NOTE-01, Bulk endpoints (delete, tags add/remove, preferences), Per-artist 10-tag cap trigger, Chunk fixture precondition todo, Deviation from research: vertical Phase 24 slice, latest_release_date lateral join, Migrations 000011/000012 tag cap trigger fixes, Confirm-step timeout trap (matchTimeout 8s) (+12 more)

### Community 33 - "TagChips.tsx (2)"
Cohesion: 0.13
Nodes (16): nextPendingId(), TagActions, TagChips(), attachErrorMessage(), handleCommit(), handleRemove(), TagComboboxProps, WatchlistRowProps (+8 more)

### Community 34 - "artist_tags"
Cohesion: 0.12
Nodes (13): artists, watchlist, events, artist_tags, artist_tags_cap_trigger, check_artist_tags_max_per_artist(), check_artist_tags_max_per_artist, tags (+5 more)

### Community 35 - "full-pipeline.yml"
Cohesion: 0.14
Nodes (18): Definition of Done (local CI gates), No AI Attribution in Commits/PRs, Recommended Tech Stack (chi, sqlc, pgx, golang-migrate, robfig/cron, caarlos0/env, slog, httplog), Full Pipeline GitHub Actions Workflow, build-scan job (image build, provenance check, tz boot check, Trivy image scan), coverage-comment job (report-only sticky PR comment), frontend-test job (prettier --check + Vitest), gitleaks job (+10 more)

### Community 36 - "CONCERNS doc"
Cohesion: 0.13
Nodes (18): docker-compose vs .env.example port drift, CONCERNS.md (codebase concerns), DSN password redaction, decodeJSONBody helper fragility, Migration ordering and idempotency, Non-root container user UID 10001, Notifier DB timeout bounds (fixed in 479c781), Poll cycle hangs without upstream timeouts (+10 more)

### Community 37 - "dialog.tsx"
Cohesion: 0.15
Nodes (12): Dialog(), DialogContent(), DialogDescription(), DialogFooter(), DialogHeader(), DialogOverlay(), DialogTitle(), Skeleton() (+4 more)

### Community 38 - "alert-dialog.tsx"
Cohesion: 0.18
Nodes (12): ConfirmDialog(), ConfirmDialogProps, AlertDialog(), AlertDialogAction(), AlertDialogCancel(), AlertDialogContent(), AlertDialogDescription(), AlertDialogFooter() (+4 more)

### Community 39 - "server.go"
Cohesion: 0.23
Nodes (12): chi.Router, net/http.Handler, Pinger, SearchSource, Store, echoRequestID(), Server, New() (+4 more)

### Community 40 - "CONTEXT doc"
Cohesion: 0.13
Nodes (15): Digest, Digest mode, Digest window, Flush, Outbox, Pending event, Detection engine (internal/detection), Idempotent event recording (ON CONFLICT DO NOTHING) (+7 more)

### Community 41 - "detector.go"
Cohesion: 0.21
Nodes (10): notifyGate, Option, RecordingSource, ReleaseDetailSource, github.com/danielrpof/drop-tracker/internal/db/sqlc.InsertEventParams, github.com/jackc/pgx/v5/pgtype.Timestamptz, Detector, New() (+2 more)

### Community 42 - "settings.go"
Cohesion: 0.27
Nodes (12): github.com/danielrpof/drop-tracker/internal/db/sqlc.NotificationSetting, time.Location, Settings, NewService(), ParseCadence(), toSettings(), WithClock(), Cadence (+4 more)

### Community 43 - "search.go"
Cohesion: 0.22
Nodes (10): deezerSource, musicBrainzSource, SearchArtist, searchResponse, sourceResult, ArtistSearcher, NewDeezerSource(), NewMusicBrainzSource() (+2 more)

### Community 44 - "STRUCTURE doc"
Cohesion: 0.16
Nodes (15): Poller CAS guard release fragility, Where to add new code guide, cmd/server entry point, STRUCTURE.md (directory layout), internal/ package layout, internal/db/migrations (immutable once committed), Naming conventions, Narrow interface pattern (+7 more)

### Community 45 - "v1.0 MILESTONE AUDIT doc"
Cohesion: 0.20
Nodes (15): Discord embed formatting constraints, internal/notifier and discord packages, Markdown escaping for real-time embeds (NTFY-07), Phase 1 debt: SIGTERM and race human verification, Phase 2 debt: review warnings WR-01..05, Phase 3 debt: Lucene injection, Deezer pagination, MusicBrainz unreachable, Phase 4 debt: unmute backlog flood, blank external id guard, Phase 5 debt: live Discord rendering and embed budget checks (+7 more)

### Community 46 - "ROADMAP doc (2)"
Cohesion: 0.14
Nodes (14): Project overview: built outward from the data layer, Phase 17 Automated VPS Deploy (deferred), Backlog Phase 999.1 Terraform + Azure ephemeral deploy, Milestone v1.0 MVP (Phases 1-7), Milestone v1.1 Hardening & Scale Readiness (Phases 8-11.1), Milestone v1.2 Cleanup & Display Fixes (Phases 12-13), Milestone v1.3 Continuous Deployment (Phases 14-17, partial), Milestone v1.4 Operator Observability (Phases 18, 18.1, 19) (+6 more)

### Community 47 - "musicbrainz.go"
Cohesion: 0.30
Nodes (8): Detector, newNotifyGate(), nullableString(), deluxeDetectionEnabled(), eventTypeMuted(), releaseTypeAllowed(), Detector, releaseTypeForStorage()

### Community 48 - "REQUIREMENTS doc"
Cohesion: 0.23
Nodes (14): Bulk Edit requirements BULK-01..05, Client-side sort/filter decision, REQUIREMENTS.md (v1.6 Watchlist Organization), Future Requirements (deferred), History tag filter HIST-02, Bulk Add requirements IMPT-01..05, Notification requirements NTFY-05..07, Out of Scope decisions (+6 more)

### Community 49 - "useBulkSelection.ts (2)"
Cohesion: 0.30
Nodes (12): bulkAddTag(), showToast(), useBulkSelection(), addTag(), applyPreferences(), clearSelection(), finishSuccess(), onRemoveDialogOpenChange() (+4 more)

### Community 50 - "INTEGRATIONS doc"
Cohesion: 0.17
Nodes (13): MusicBrainz TLS handshake failure (environmental), GitHub Actions CI/CD pipeline, INTEGRATIONS.md (external integrations), Deezer API integration, Three-stage Dockerfile image, MusicBrainz API integration, No user authentication, Logging and observability (+5 more)

### Community 51 - "status.go"
Cohesion: 0.30
Nodes (11): serverConfig, StatusDeps, statusInstance, statusResponse, statusRun, statusSource, StatusStore, WatchlistCounter (+3 more)

### Community 52 - "musicbrainz.go (2)"
Cohesion: 0.24
Nodes (9): coverArtURLForReleaseGroup(), earlierDate(), earliestReleaseDate(), guestFeatureArt(), withinDeluxeRecheckWindow(), Client, RecordingRelease, recordingLookupResponse (+1 more)

### Community 53 - "STATE doc"
Cohesion: 0.21
Nodes (12): Event history disk growth, -race testing broken on Windows dev machine, Frontend XSS via untrusted data, Phase 15 coverage-report tool and PR comment, Accumulated decision log, v1.5 digest notifications design, Grilling sessions override earlier decisions, Phase 18.1 poll-cycle instrumentation (+4 more)

### Community 54 - "STACK doc"
Cohesion: 0.21
Nodes (12): Environment configuration variables, go-chi/chi v5 router, caarlos0/env v11 config, STACK.md (technology stack), React 19 + React Router 7 + Vite + Tailwind 4 SPA, Go 1.26 backend, golang-migrate v4, pgx/v5 and sqlc codegen (+4 more)

### Community 55 - "WatchlistBulkStore"
Cohesion: 0.22
Nodes (8): logInstanceGateStatus(), main(), run(), WatchlistBulkStore, IsWeakPassphrase(), Short(), Option, WithWatchlistBulk()

### Community 56 - "bulk.go"
Cohesion: 0.33
Nodes (8): axesOverlap(), Service, missingIDs(), normalizeAxis(), AxisDelta, BulkRemoveResult, BulkUpdateResult, PreferencesDelta

### Community 57 - "tags.go"
Cohesion: 0.25
Nodes (8): attachTagRequest, deleteTagResponse, mergeTagRequest, renameTagRequest, tagResponse, TagStore, Option, WithTags()

### Community 58 - "recordings.go"
Cohesion: 0.33
Nodes (6): displayArtistName(), isGuestFeature(), Client, Recording, ArtistCreditEntry, recordingEnvelope

### Community 59 - "logging.go"
Cohesion: 0.39
Nodes (6): log/slog.Level, Config, Load(), New(), NewWithWriter(), parseLevel()

### Community 60 - "releases.go"
Cohesion: 0.39
Nodes (4): Client, Medium, Release, releaseEnvelope

### Community 61 - "CONTEXT doc (2)"
Cohesion: 0.29
Nodes (7): Carrier, Merge (tags), Note, Tag, Tag link, Tag vocabulary, Watched artist

### Community 62 - "artist_lookup.go"
Cohesion: 0.43
Nodes (5): ArtistDetail, Client, ArtistAlias, ArtistRelation, ArtistRelationURL

### Community 63 - "terraform always on env doc"
Cohesion: 0.52
Nodes (7): Always-on B1ms Flexible Server cost, Drift detection via scheduled plan, Long-lived Terraform environment seed, Provider and version upgrades, Seed: Promote Ephemeral Terraform Env, Separate prod-like workspace, State-recovery drill

### Community 64 - "activity.go"
Cohesion: 0.40
Nodes (3): sync/atomic.Int32, ActivityGate, NewActivityGate()

### Community 65 - "backfill.go"
Cohesion: 0.60
Nodes (4): Stats, Store, Backfill(), waitForActivityGate()

### Community 66 - "alerter.go"
Cohesion: 0.47
Nodes (4): noopAlerter, Alerter, NoOpAlerter(), SelectAlerter()

### Community 67 - "CONTEXT doc (3)"
Cohesion: 0.33
Nodes (6): Cadence, Digest slot, Grace window, Slot record, Watermark, Calendar-based slot math over duration math

### Community 68 - "schema_version.go"
Cohesion: 0.60
Nodes (4): RowQuerier, SchemaVersionReader, NewSchemaVersionReader(), SchemaVersion()

### Community 69 - "server.go (2)"
Cohesion: 0.33
Nodes (5): Option, readyResponse, SchemaVersioner, WithAuthGate(), WithReadiness()

### Community 70 - "settings.go (2)"
Cohesion: 0.40
Nodes (5): settingsResponse, SettingsStore, updateSettingsRequest, WithSettings(), toSettingsResponse()

### Community 71 - "releasegroups.go"
Cohesion: 0.53
Nodes (3): Client, ReleaseGroup, releaseGroupEnvelope

### Community 72 - "HistoryFilters.tsx"
Cohesion: 0.60
Nodes (4): Combobox(), commitSelection(), handleTriggerKeyDown(), openAt()

### Community 73 - "pnpm workspace doc"
Cohesion: 0.70
Nodes (4): allowBuilds esbuild and msw, pnpm workspace config, Transitive CVE overrides, Trivy fs HIGH CVE findings

### Community 74 - "CONTEXT doc (4)"
Cohesion: 0.67
Nodes (4): Latest release, Upcoming release, Option D: Upcoming Releases Calendar, latest_release_date ignores retention (G1)

## Knowledge Gaps
- **175 isolated node(s):** `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest`, `notification_settings`, `Client` (+170 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 336 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **17 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `WatchlistBulkStore` to `writeError`, `.HandleLogin`, `deezer/client.go`, `notifier.go`, `migrate.go`, `match.go`, `service.go`, `poller.go`, `service.go (2)`, `server.go`, `detector.go`, `settings.go`, `search.go`, `status.go`, `tags.go`, `logging.go`, `activity.go`, `backfill.go`, `alerter.go`, `schema_version.go`, `server.go (2)`, `settings.go (2)`?**
  _High betweenness centrality (0.046) - this node is a cross-community bridge._
- **Why does `NewWithWriter()` connect `logging.go` to `main.go`, `notifier.go`?**
  _High betweenness centrality (0.036) - this node is a cross-community bridge._
- **Why does `runScan()` connect `main.go` to `parse.go`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Are the 6 inferred relationships involving `Watchlist()` (e.g. with `addTag()` and `announce()`) actually correct?**
  _`Watchlist()` has 6 INFERRED edges - model-reasoned connections that need verification._
- **What connects `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest` to the rest of the system?**
  _175 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `writeError` be split into smaller, more focused modules?**
  _Cohesion score 0.06621226874391431 - nodes in this community are weakly interconnected._
- **Should `26 RESEARCH doc` be split into smaller, more focused modules?**
  _Cohesion score 0.05608322026232474 - nodes in this community are weakly interconnected._