# Graph Report - drop-tracker  (2026-10-06)

## Corpus Check
- 332 files · ~451,884 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 3075 nodes · 9590 edges · 173 communities (110 shown, 25 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 1060 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Watchlist & Detector DB Tests
- Poller Tests
- Watchlist HTTP Tests
- HTTP Handlers & JSON Helpers (3)
- shadcn UI Primitives
- Frontend API Client & Watchlist Route
- Status Endpoint & Server
- MusicBrainz Client Tests
- DB Migration Runner
- System Page & Source Panels
- Coverage-Report Tool
- Login Throttle Tests (11)
- Notifier Integration Tests (12)
- Auth Gate Tests
- History Route & Event Cards
- Tags sqlc Queries
- Tag Chips & Combobox UI
- MusicBrainz Detection
- Tags HTTP Tests
- Shared Test Fakes & Types
- Migration-Check Tests
- Notifier Integration Tests (21)
- Artist Matcher Tests
- sqlc Models & Watchlist Queries
- Deezer Client Tests (24)
- Dialogs, Auth Store & App Shell (25)
- Artist Note & Preference Toggles
- Watchlist Service
- Notifier Integration Tests (28)
- Digest Chunking Tests (29)
- SQL Schema Parser
- HTTP Server Wiring Tests
- Notifier Integration Tests (32)
- Discord Digest Formatting
- Tags Service
- Deezer-MusicBrainz Matcher
- Digest Scheduler Tests
- MusicBrainz Recordings
- CI Pipeline & Local Gates
- Settings HTTP Tests
- Notifier Core
- Query Column Reference Analysis
- Search UI Components
- Web Runtime Dependencies (43)
- Migration-Check Scanner
- Config Loading Tests
- Notifier Integration Tests (46)
- Events Service Tests (47)
- Dialogs, Auth Store & App Shell (48)
- Web Dev Dependencies (49)
- Events sqlc Queries
- Digest Chunking Tests (51)
- Tag Schema Tests
- SQL Lexer (Dollar Quoting)
- Readiness Endpoint Tests
- Discord Embed Formatting Tests
- ConfirmDialog & AlertDialog
- HTTP Handlers & JSON Helpers (57)
- Tags & Notes Migration
- Artist Backfill
- Notifier DB Timeout Tests
- Phase 25 Tag Filter Plans
- Deezer Client Tests (62)
- Notification Settings Service
- DB Pool Config
- Phase 24 Optimistic UI & Notes
- Phase 25 Release-Date Enrichment
- Artist Search Stubs
- Digest Chunk Builder
- Phase 24 UI Contract Concepts
- Events Service Tests (70)
- MusicBrainz Releases
- Phase 25 URL State & Name Filter
- Server Entrypoint
- v1.6 Phases & Requirements (74)
- Discord Webhook Client
- Discord Client Retry Tests
- Activity Gate
- v1.6 Phases & Requirements (78)
- Phase 24 Pattern Map Concepts
- Phase 24 Postgres Research Findings
- Phase 24 Review, UAT & Verification
- Phase 25 History Tag Filter Plans
- Web package.json Scripts
- Search Cancellation Stubs
- Migration-Check Git Diff
- Codebase Architecture Map
- MusicBrainz Artist Lookup
- MusicBrainz Artist Search
- Per-Artist Tag Cap Decisions
- Phase 24 Security Threats
- Release Detector
- Single-Instance ADRs (92)
- Query Refs Tests
- SPA Embed Handler
- End-to-End Boot Tests
- Single-Instance ADRs (96)
- Status Snapshot & Ring Buffer (97)
- Digest Ack Split
- Tag Cap Locking & Merge
- Migration Rollback Rules
- Status Snapshot & Ring Buffer (101)
- Web Template & Status Client
- Login Throttle Tests (103)
- Build Info
- Combobox Interaction
- Digest Mode Concepts
- Digest Slot & Watermark
- Deezer Client
- v1.4 Observability Milestone (109)
- Comment Discipline Convention
- Graph-First & GSD Conventions
- clsx Dependency
- Migration Safety & Passphrase Gate
- Issue Tracker Conventions
- Health Endpoint
- Notification Settings Migration
- Raw SQL Statement
- Milestone Log & Retrospective
- Feature Options (Sinks, Digest)
- v1.4 Observability Milestone (121)
- React Dependency
- React Router Dev
- Tailwind Vite Plugin
- Web Dev Dependencies (125)
- Web Runtime Dependencies (126)
- React Types
- Web Dev Dependencies (128)
- Vitest Dependency
- Domain Glossary
- Watched Artist vs Credited Artist
- github.com/danielrpof/drop-tracker
- 2026-09-08-feature-module-ideas-post-v1.3.md
- Soft-delete Event Retention
- v1.0 Milestone Audit

## God Nodes (most connected - your core abstractions)
1. `New()` - 228 edges
2. `NewIsolatedTestPool()` - 146 edges
3. `NewTestPool()` - 137 edges
4. `New()` - 116 edges
5. `cn()` - 104 edges
6. `discardLogger()` - 97 edges
7. `New()` - 79 edges
8. `NewService()` - 78 edges
9. `newTestLogger()` - 77 edges
10. `newTestLogger()` - 64 edges

## Surprising Connections (you probably didn't know these)
- `handleHealth (internal/httpserver/health.go)` --shares_data_with--> `NewPool()`  [INFERRED]
  .planning/codebase/ARCHITECTURE.md → internal/db/pool.go
- `Comment Discipline Rule` --semantically_similar_to--> `Coding Conventions`  [INFERRED] [semantically similar]
  .claude/CLAUDE.md → .planning/codebase/CONVENTIONS.md
- `Tag merge via UPDATE tag_id (never insert)` --semantically_similar_to--> `Expand / backfill / contract pattern (000006, 000007)`  [INFERRED] [semantically similar]
  docs/adr/0004-per-artist-tag-cap-trigger.md → internal/db/migrations/README.md
- `G7: keep both comboboxes` --references--> `Combobox()`  [EXTRACTED]
  .planning/phases/25-find-filter-watchlist-and-history/25-CONTEXT.md → web/app/components/history/HistoryFilters.tsx
- `pre-commit config (gitleaks, golangci-lint --fix, prettier --write)` --semantically_similar_to--> `gitleaks job`  [EXTRACTED] [semantically similar]
  .pre-commit-config.yaml → .github/workflows/full-pipeline.yml

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Release-path gate jobs feeding build-scan then release** — github_workflows_full_pipeline_lint, github_workflows_full_pipeline_test, github_workflows_full_pipeline_gitleaks, github_workflows_full_pipeline_trivy_fs, github_workflows_full_pipeline_frontend_test, github_workflows_full_pipeline_migration_check, github_workflows_full_pipeline_n1_boot, github_workflows_full_pipeline_build_scan, github_workflows_full_pipeline_release [EXTRACTED 1.00]
- **Digest scheduling domain model** — context_digest_mode, context_outbox, context_digest_window, context_digest_slot, context_slot_record, context_watermark, context_grace_window, context_flush [EXTRACTED 1.00]
- **N-1 rollback safety enforcement** — internal_db_migrations_readme_n1_invariant, internal_db_migrations_readme_n1_boot_job, internal_db_migrations_readme_ahead_of_source_test, internal_db_migrations_readme_migration_check, internal_db_migrations_readme_expand_backfill_contract [EXTRACTED 1.00]
- **History tag filter (HIST-02) end to end** — planning_phases_25_find_filter_watchlist_and_history_25_02_plan, planning_phases_25_find_filter_watchlist_and_history_25_06_plan, planning_phases_25_find_filter_watchlist_and_history_25_09_plan, planning_phases_25_find_filter_watchlist_and_history_25_context_d11_history_single_tag, planning_phases_25_find_filter_watchlist_and_history_25_research_pattern_tag_filter_retention [EXTRACTED 1.00]
- **Single-query latest/next release enrichment** — planning_phases_25_find_filter_watchlist_and_history_25_01_plan, planning_phases_25_find_filter_watchlist_and_history_25_context_latest_release_date, planning_phases_25_find_filter_watchlist_and_history_25_context_next_release_date, planning_phases_25_find_filter_watchlist_and_history_25_context_g1_no_retention, planning_phases_25_find_filter_watchlist_and_history_25_context_g2_upcoming_utc, planning_phases_25_find_filter_watchlist_and_history_25_research_pattern_lateral_enrichment [EXTRACTED 1.00]
- **Single-instance in-process coordination decisions** — docs_adr_0001_in_process_ring_buffer_for_poll_run_history_ring_buffer, docs_adr_0002_one_outbox_one_sender_lock_notifier_notifying_lock, docs_adr_0001_in_process_ring_buffer_for_poll_run_history_single_instance_constraint, docs_adr_0002_one_outbox_one_sender_lock_postgres_advisory_lock [EXTRACTED 1.00]
- **Local hooks mirroring CI gates** — pre_commit_config, golangci, claude_claude_definition_of_done, github_workflows_full_pipeline [INFERRED 0.85]
- **Watchlist find/filter composition (search, sort, tags, toggles)** — planning_phases_25_find_filter_watchlist_and_history_25_04_plan, planning_phases_25_find_filter_watchlist_and_history_25_05_plan, planning_phases_25_find_filter_watchlist_and_history_25_07_plan, planning_phases_25_find_filter_watchlist_and_history_25_08_plan, planning_phases_25_find_filter_watchlist_and_history_25_research_pattern_selectvisible, planning_phases_25_find_filter_watchlist_and_history_25_context_d10_three_empty_states [INFERRED 0.85]
- **Manage tags rename/merge/delete UI flow** — planning_phases_24_artist_tags_notes_24_ui_spec_managetagsdialog, planning_phases_24_artist_tags_notes_24_ui_spec_confirmdialog, planning_phases_24_artist_tags_notes_24_context_d_09_rename_merge_confirm, planning_phases_24_artist_tags_notes_24_context_d_16_reusable_alert_dialog, planning_phases_24_artist_tags_notes_24_context_d_22_vocabulary_api, planning_phases_24_artist_tags_notes_24_ui_spec_focus_management_r5 [INFERRED 0.85]
- **Per-artist 10-tag cap enforcement** — planning_phases_24_artist_tags_notes_24_context_d_18_cap_trigger, planning_phases_24_artist_tags_notes_24_context_d_19_merge_never_inserts, planning_phases_24_artist_tags_notes_24_context_adr_0004_cap_trigger, planning_phases_24_artist_tags_notes_24_research_before_insert_trigger_before_on_conflict, planning_phases_24_artist_tags_notes_24_research_for_no_key_update_lock, planning_phases_24_artist_tags_notes_24_security_t_24_01_02_cap_tampering, planning_phases_24_artist_tags_notes_24_security_migrations_000011_000012 [INFERRED 0.85]
- **Poll-run history to /status to SPA flow** — docs_adr_0001_in_process_ring_buffer_for_poll_run_history_runrecorder_seam, docs_adr_0001_in_process_ring_buffer_for_poll_run_history_ring_buffer, docs_api_status_contract_pollruns_snapshot, docs_api_status_contract_handlestatus, docs_api_status_contract_spa_api_client [INFERRED 0.85]

## Communities (173 total, 25 thin omitted)

### Community 0 - "Watchlist & Detector DB Tests"
Cohesion: 0.05
Nodes (160): fakeReleaseDetailSource, insertEventFailingQuerier, ExpectedSchemaVersion(), TestExpectedSchemaVersion(), TestSchemaVersion_Integration(), TestSchemaVersionReader_Delegates(), New(), TestSQLCPing() (+152 more)

### Community 1 - "Poller Tests"
Cohesion: 0.06
Nodes (103): cron.Cron, sync/atomic.Bool, New(), runOneArtist(), decodeLogRecords(), deezerID(), eightEntries(), failFirstThree() (+95 more)

### Community 2 - "Watchlist HTTP Tests"
Cohesion: 0.06
Nodes (82): errorBody, healthBody, noteEntryBody, SearchSource, watchlistEntryBody, TestHandleListEvents_CursorRejection(), TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(), TestHandleListEvents_HappyPathReturns200WithEnvelope() (+74 more)

### Community 3 - "HTTP Handlers & JSON Helpers (3)"
Cohesion: 0.05
Nodes (47): globalCounter, ipLimiter, loginRequest, loginThrottle, Cursor, cursorWireForm, net/http.Request, net/http.ResponseWriter (+39 more)

### Community 4 - "shadcn UI Primitives"
Cohesion: 0.05
Nodes (64): AboutInstanceProps, Cadence, CADENCE_LABELS, SaveStatus, Alert(), AlertAction(), AlertDescription(), AlertTitle() (+56 more)

### Community 5 - "Frontend API Client & Watchlist Route"
Cohesion: 0.06
Nodes (47): Phase 25 Pattern Map, mockDeleteTag, mockListTags, mockMergeTag, mockRenameTag, openCollisionConfirm(), renderDialog(), SearchBox() (+39 more)

### Community 6 - "Status Endpoint & Server"
Cohesion: 0.06
Nodes (56): chi.Router, net/http.Handler, attachTagRequest, deleteTagResponse, fakeStatusStore, mergeTagRequest, Option, Pinger (+48 more)

### Community 7 - "MusicBrainz Client Tests"
Cohesion: 0.08
Nodes (62): TestLookupArtist_DecodesFixture(), TestLookupArtist_EmptyMBID(), TestLookupArtist_MalformedJSON(), TestLookupArtist_NonOKStatus(), TestLookupArtist_NoRelationsNoAliasesYieldsNonNilZeroLengthSlices(), TestLookupArtist_RequestShape(), TestReleasesForRecording_DecodesFixture(), TestReleasesForRecording_EmptyMBID() (+54 more)

### Community 8 - "DB Migration Runner"
Cohesion: 0.07
Nodes (54): Atomic CAS overlap-guard pattern (poll cycles), main(), run(), highestMigrationVersion(), scratchSchemaDSN(), TestRun_AppliesHeadSchema(), TestRun_MissingDatabaseURL(), retryConfig (+46 more)

### Community 9 - "System Page & Source Panels"
Cohesion: 0.07
Nodes (47): AboutInstance(), DigestSettings(), handleCadenceChange(), handleDigestModeChange(), save(), DigestSettingsProps, mockUpdateDigestSettings, classifyOutcome() (+39 more)

### Community 10 - "Coverage-Report Tool"
Cohesion: 0.08
Nodes (56): appendFile(), backendTotalPct(), formatDelta(), frontendLinesPct(), main(), parseBlockLine(), readBackend(), readFrontend() (+48 more)

### Community 11 - "Login Throttle Tests (11)"
Cohesion: 0.11
Nodes (46): discordAlerter, noopAlerter, recordingAlerter, golang.org/x/time/rate.Limit, time.Duration, Alerter, NoOpAlerter(), SelectAlerter() (+38 more)

### Community 12 - "Notifier Integration Tests (12)"
Cohesion: 0.15
Nodes (50): bytes.Buffer, github.com/jackc/pgx/v5/pgxpool.Pool, decodeLogRecords(), decodeModeTransitionRecords(), erroringFrom(), erroringSettings(), flippingSettings(), insertPendingEvent() (+42 more)

### Community 13 - "Auth Gate Tests"
Cohesion: 0.11
Nodes (43): fakeAlerter, stubPinger, syncBuffer, Token, net/http.Cookie, net/http.Response, deleteSession(), discardLogger() (+35 more)

### Community 14 - "History Route & Event Cards"
Cohesion: 0.06
Nodes (29): CoverArt(), CoverArtProps, EmptyState(), EmptyStateProps, EVENT_BADGE, EventCard(), EventCardProps, GuestFeatureBody() (+21 more)

### Community 15 - "Tags sqlc Queries"
Cohesion: 0.08
Nodes (20): context.Context, fakeWatchlistCounter, noopPinger, stubPinger, Artist, Queries, Queries, Queries (+12 more)

### Community 16 - "Tag Chips & Combobox UI"
Cohesion: 0.07
Nodes (30): CollisionTarget, ManageTagsDialogProps, FocusRequest, nextPendingId(), PendingTag, TagActions, TagChips(), attachErrorMessage() (+22 more)

### Community 17 - "MusicBrainz Detection"
Cohesion: 0.08
Nodes (30): log/slog.Logger, Detector, newNotifyGate(), nullableString(), deluxeDetectionEnabled(), eventTypeMuted(), releaseTypeAllowed(), TestFilter_DeluxeIsAGateNotAType() (+22 more)

### Community 18 - "Tags HTTP Tests"
Cohesion: 0.12
Nodes (41): testing.T, tagsServerOpts, tagSummaryWire, tagWire, watchlistTagsEntryBody, TestMigrationsReadme_ContainsRequiredPhrases(), TestMigrationsReadme_IsNonTrivial(), TestSearchArtist_WireShapeKeySet() (+33 more)

### Community 19 - "Shared Test Fakes & Types"
Cohesion: 0.07
Nodes (22): stubAlbumLister, stubGroupLister, syncBuf, artistAlbumsResponse, net/http.RoundTripper, sync.Mutex, capturingRoundTripper, Album (+14 more)

### Community 20 - "Migration-Check Tests"
Cohesion: 0.11
Nodes (41): main(), run(), readPrevReleaseFixture(), runCapture(), runScanCapture(), stubQueriesGitShow(), TestChangedFiles_AddedAndModified(), TestChangedFiles_NothingUnderGlobIsFalseAndEmpty() (+33 more)

### Community 21 - "Notifier Integration Tests (21)"
Cohesion: 0.20
Nodes (40): TestDigestScheduler_NotDue_Quiet_NoLogsNoWrites(), countNotified(), digestSettingsReader(), flippingDigestSettings(), insertPendingEventsForChunking(), snapshotSettings(), TestNoOp_SendDigestIfDue_ReturnsNilTouchesNothing(), TestSendDigestIfDue_AllSuppressed_ZeroSendAckedSlotAdvancesSentAtStaysNull() (+32 more)

### Community 22 - "Artist Matcher Tests"
Cohesion: 0.13
Nodes (37): stubSearcherFunc, NewMatcher(), TestMatch_EmptyNameFailsClosedWithoutOutboundCall(), TestMatch_MatchedCandidateWithEmptyPictureYieldsNilImageURL(), TestMatch_NoCloseNameCandidateFailsClosed(), TestMatch_SearchErrorSurfaces(), TestMatch_SingleCandidateIssuesNoTieBreakFetch(), TestMatch_SingleCloseNameCandidate() (+29 more)

### Community 23 - "sqlc Models & Watchlist Queries"
Cohesion: 0.08
Nodes (19): stubStore, notifyGate, github.com/jackc/pgx/v5/pgtype.Timestamptz, UpsertArtistParams, Artist, NotificationSetting, Queries, Queries (+11 more)

### Community 24 - "Deezer Client Tests (24)"
Cohesion: 0.14
Nodes (35): buildArtistAlbumsPageJSON(), pagedArtistAlbumsServer(), TestArtistAlbums_CancellationBetweenPagesAborts(), TestArtistAlbums_DecodesFixture(), TestArtistAlbums_EmptyArtistIDReturnsErrorWithZeroRequests(), TestArtistAlbums_MidFetchErrorStopsWithNoRetry(), TestArtistAlbums_NonexistentArtistReturnsEmptyNonNilNoError(), TestArtistAlbums_PacingAppliesAcrossPages() (+27 more)

### Community 25 - "Dialogs, Auth Store & App Shell (25)"
Cohesion: 0.08
Nodes (16): PassphraseScreen(), handleSubmit(), Input(), Toaster(), createSession(), deleteSession(), authStore, gateActive (+8 more)

### Community 26 - "Artist Note & Preference Toggles"
Cohesion: 0.09
Nodes (21): Button(), buttonVariants, Checkbox(), ArtistNote(), handleSave(), ArtistNoteProps, FocusRequest, saveErrorMessage() (+13 more)

### Community 27 - "Watchlist Service"
Cohesion: 0.11
Nodes (17): stubStore, fullEntryBody, stubStore, ListWatchlistRow, TestNormalizeSet(), entryFromRow(), AddParams, Entry (+9 more)

### Community 28 - "Notifier Integration Tests (28)"
Cohesion: 0.21
Nodes (34): NewService(), countArtistTagsByTagID(), countTagsByLowerName(), seedEntry(), TestService_Attach_AtCapReturnsErrTagCapReachedAndRollsBackOrphan(), TestService_Attach_CreatesAndLinksNewTag(), TestService_Attach_ExistingLinkReturnsCreatedFalse(), TestService_Attach_UnknownEntryReturnsErrEntryNotFound() (+26 more)

### Community 29 - "Digest Chunking Tests (29)"
Cohesion: 0.16
Nodes (32): buildDigestGroups(), chunkDigest(), containsID(), makeSyntheticGroup(), stripChunkMarkers(), syntheticInvariantBatch(), TestBuildDigestChunks_CapTotalReflectsKeptChunksNotUncapped(), TestBuildDigestChunks_CapTruncatesAndReportsDeferred() (+24 more)

### Community 30 - "SQL Schema Parser"
Cohesion: 0.08
Nodes (21): Parse(), SchemaColumns(), TestParse_AddCheckNeverAddColumn(), TestParse_AddColumnNotNullAndDefaultAreIndependent(), TestParse_AlterTableActionsInClauseOrder(), TestParse_CreateTableColumnsExcludeConstraintClauses(), TestParse_DropTableNormalizesAndKeepsRaw(), TestParse_EachActionTypeFromItsClauseForm() (+13 more)

### Community 31 - "HTTP Server Wiring Tests"
Cohesion: 0.11
Nodes (25): log/slog.Level, syncBuffer, Config, assertInert(), newCapturingServer(), newGatedServer(), requestIDsInLog(), TestGatedServer_TrustProxyHeaders_RealIPWiring() (+17 more)

### Community 32 - "Notifier Integration Tests (32)"
Cohesion: 0.18
Nodes (26): time.Time, insertPendingEventTypedRaw(), mustLoadDigestZone(), newManualTickSource(), newMutableClock(), newSignalingSink(), seedNotificationSettings(), sweep() (+18 more)

### Community 33 - "Discord Digest Formatting"
Cohesion: 0.15
Nodes (27): golang.org/x/text/collate.Collator, Event, artistKey(), digestLine(), escapeMarkdown(), lineLabel(), sortDigestGroup(), TestArtistKey() (+19 more)

### Community 34 - "Tags Service"
Cohesion: 0.11
Nodes (13): fakeTagStore, DBTX, Queries, NormalizeName(), TestNormalizeName(), AttachResult, Summary, Tag (+5 more)

### Community 35 - "Deezer-MusicBrainz Matcher"
Cohesion: 0.15
Nodes (20): AlbumLister, ArtistDetailLookup, ArtistFetcher, ArtistSearcher, Matcher, Option, ReleaseGroupLister, aliasQueryNames() (+12 more)

### Community 36 - "Digest Scheduler Tests"
Cohesion: 0.17
Nodes (20): defaultTickSource(), DigestScheduler, NewDigestScheduler(), newFakeSink(), newSyncTestLogger(), TestDigestScheduler_CheckError_LoggedAndLoopContinues(), TestDigestScheduler_ChecksReceiveInjectedClockValue(), TestDigestScheduler_ContextCancelled_LoopExitsWithoutStop() (+12 more)

### Community 37 - "MusicBrainz Recordings"
Cohesion: 0.11
Nodes (13): datedRecordingSource, erroringRecordingSource, fakeRecordingSource, noRecordingSource, Client, RecordingRelease, ArtistCreditEntry, Client (+5 more)

### Community 38 - "CI Pipeline & Local Gates"
Cohesion: 0.09
Nodes (27): Definition of Done (local CI gates), No AI Attribution in Commits/PRs, Recommended Tech Stack (chi, sqlc, pgx, golang-migrate, robfig/cron, caarlos0/env, slog, httplog), Full Pipeline GitHub Actions Workflow, build-scan job (image build, provenance check, tz boot check, Trivy image scan), coverage-comment job (report-only sticky PR comment), frontend-test job (prettier --check + Vitest), gitleaks job (+19 more)

### Community 39 - "Settings HTTP Tests"
Cohesion: 0.20
Nodes (26): net/http/httptest.Server, settingsBody, settingsServerOpts, WithSettings(), SettingsStore, assertSettingsDefaults(), getSettingsRaw(), loginForSettings() (+18 more)

### Community 40 - "Notifier Core"
Cohesion: 0.13
Nodes (18): Querier, ackDigestBatch(), ackEventsOnly(), Notifier, Notifier, listUnnotified(), logSettingsReadFailure(), markNotified() (+10 more)

### Community 41 - "Query Column Reference Analysis"
Cohesion: 0.18
Nodes (19): NormalizeIdent(), StripIdent(), StripSchemaQualifier(), SplitTopLevelCommas(), createTableColumns(), parseAlterAction(), classifyBareColumn(), expandStar() (+11 more)

### Community 42 - "Search UI Components"
Cohesion: 0.12
Nodes (21): SearchResultRow(), SearchResultRowProps, SearchResultsColumns(), SearchResultsColumnsProps, SOURCE_LABELS, SourceColumn(), SourceColumnProps, sourceLabel() (+13 more)

### Community 43 - "Web Runtime Dependencies (43)"
Cohesion: 0.08
Nodes (25): @base-ui/react, class-variance-authority, @fontsource-variable/inter, isbot, lucide-react, next-themes, react-dom, react-router (+17 more)

### Community 44 - "Migration-Check Scanner"
Cohesion: 0.18
Nodes (20): buildPrevReleaseRefs(), buildReport(), classifyAction(), crossReferenceFinding(), newFinding(), parseAnnotation(), pathAllowedForGitShow(), readAtTag() (+12 more)

### Community 45 - "Config Loading Tests"
Cohesion: 0.18
Nodes (24): Load(), configEnvKeys(), envExampleKeys(), repoRoot(), setDiff(), setRequired(), TestDockerComposeWiresGateEnvVars(), TestDotEnvIsNotTracked() (+16 more)

### Community 46 - "Notifier Integration Tests (46)"
Cohesion: 0.17
Nodes (21): Cadence, time.Location, NewService(), TestService_FreshSchemaSeedsOneRowWithDefaults(), TestService_LastSentWatermarkUntouched(), TestService_SecondRowRejectedByCheckConstraint(), TestService_UpdateIsIdempotent(), TestService_UpdateLeavesSlotUntouched() (+13 more)

### Community 47 - "Events Service Tests (47)"
Cohesion: 0.28
Nodes (22): NewService(), datePtr(), insertTestArtist(), insertTestEvent(), insertTestEventAt(), insertTestEventTyped(), insertTestEventWithDate(), TestHandleListEvents_CursorRoundTripsThroughHTTP() (+14 more)

### Community 48 - "Dialogs, Auth Store & App Shell (48)"
Cohesion: 0.14
Nodes (17): Dialog(), DialogContent(), DialogDescription(), DialogFooter(), DialogHeader(), DialogOverlay(), DialogTitle(), FocusRequest (+9 more)

### Community 49 - "Web Dev Dependencies (49)"
Cohesion: 0.09
Nodes (23): jsdom, prettier, prettier-plugin-tailwindcss, tailwindcss, @testing-library/dom, @testing-library/react, @testing-library/user-event, @types/node (+15 more)

### Community 50 - "Events sqlc Queries"
Cohesion: 0.13
Nodes (10): Event, recordingQuerier, HasOlderEventsParams, InsertEventParams, ListEventsParams, ListEventsRow, Queries, AdvanceGroupTrackCountBaselineParams (+2 more)

### Community 51 - "Digest Chunking Tests (51)"
Cohesion: 0.15
Nodes (22): buildDigestChunks(), digestWindowHeader(), remainderMarker(), syntheticEvents(), TestBuildDigestChunks_CapLastChunkCarriesRemainderMarker(), TestBuildDigestChunks_DeluxeTrackCountSuffix(), TestBuildDigestChunks_DuplicateSourcesRenderAsTwoLines(), TestBuildDigestChunks_EmptyGroupOmitsHeadingEntirely() (+14 more)

### Community 52 - "Tag Schema Tests"
Cohesion: 0.28
Nodes (20): detachRaceResult, assertDetachRaceRefused(), insertTag(), linkTag(), runDetachRace(), seedArtist(), seedArtistNamed(), TestSchema_Migration000011_DownUpRoundTrip() (+12 more)

### Community 53 - "SQL Lexer (Dollar Quoting)"
Cohesion: 0.16
Nodes (17): strings.Builder, TestCopyDollarQuoted_NoClosingTagCopiesRemainder(), TestDollarTagAt(), TestFindFromJoinTables_AdjacentFromJoinBothFound(), copyDollarQuoted(), copySingleQuoted(), dollarTagAt(), RawStatement (+9 more)

### Community 54 - "Readiness Endpoint Tests"
Cohesion: 0.28
Nodes (18): fakeSchemaVersioner, readyBody, getReady(), schemaAt(), TestReady_AheadOfSource(), TestReady_DBUnreachable(), TestReady_EmptySchemaMigrations(), TestReady_ExactPathOnly() (+10 more)

### Community 55 - "Discord Embed Formatting Tests"
Cohesion: 0.19
Nodes (19): TestDigestLine_RendersLabelURLAndDeluxeSuffix(), formatEmbed(), emojiPrefix(), i32Ptr(), TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail(), TestFormatEmbed_DeluxeChange_BothCountsPresent(), TestFormatEmbed_DeluxeChange_NilBothCounts(), TestFormatEmbed_DeluxeChange_NilPreviousTrackCount() (+11 more)

### Community 56 - "ConfirmDialog & AlertDialog"
Cohesion: 0.15
Nodes (12): ConfirmDialog(), ConfirmDialogProps, AlertDialog(), AlertDialogAction(), AlertDialogCancel(), AlertDialogContent(), AlertDialogDescription(), AlertDialogFooter() (+4 more)

### Community 57 - "HTTP Handlers & JSON Helpers (57)"
Cohesion: 0.17
Nodes (12): deezerSource, musicBrainzSource, searchResponse, searchResponseBody, sourceResult, stubSearchSource, ArtistSearcher, SearchArtist (+4 more)

### Community 58 - "Tags & Notes Migration"
Cohesion: 0.12
Nodes (13): artists, watchlist, events, artist_tags, artist_tags_cap_trigger, check_artist_tags_max_per_artist(), check_artist_tags_max_per_artist, tags (+5 more)

### Community 59 - "Artist Backfill"
Cohesion: 0.26
Nodes (15): Stats, Store, Backfill(), matchingCandidate(), TestBackfill_ActivityGate_DelaysThenProceeds(), TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach(), TestBackfill_ContextCancelledPartway_StopsPromptly(), TestBackfill_ListArtistsMissingImageErrors_ReturnsErrNoWrites() (+7 more)

### Community 60 - "Notifier DB Timeout Tests"
Cohesion: 0.17
Nodes (13): sync/atomic.Int32, callNotifyPending(), discardLogger(), shrinkDBOpTimeout(), TestNotifyPending_ParentCancellationStillPropagates(), TestNotifyPending_RecoversAfterUnresponsiveDatabase(), TestNotifyPending_SettingsReadUnresponsive_ReturnsInsteadOfWedgingLock(), TestNotifyPending_UnresponsiveDatabase_ReturnsInsteadOfWedging() (+5 more)

### Community 61 - "Phase 25 Tag Filter Plans"
Cohesion: 0.22
Nodes (18): Plan 25-05: Watchlist sorts with stable ties, Plan 25-07: Watchlist multi-tag any-of filter, Plan 25-08: preference toggles and chip click-to-filter, Phase 25 Context: Find & Filter, D-05: multi-tag filter is any-of (OR), D-07: Watchlist tag options derived from payload, D-08: card tag chip toggles filter, D-09: Has muted events / Custom release types toggles (+10 more)

### Community 62 - "Deezer Client Tests (62)"
Cohesion: 0.24
Nodes (13): golang.org/x/time/rate.Limiter, net/http.Client, Client, NewClient(), Do(), TestDo_CloseCancelsTheDerivedContext(), TestDo_LimiterWaitErrorOnCancelledContext(), TestDo_Success() (+5 more)

### Community 63 - "Notification Settings Service"
Cohesion: 0.23
Nodes (10): stubDigestOff, fakeSettingsStore, Cadence, Settings, UpdateParams, ParseCadence(), toSettings(), Option (+2 more)

### Community 64 - "DB Pool Config"
Cohesion: 0.23
Nodes (14): github.com/jackc/pgx/v5/pgxpool.Config, dsnSetsMaxConns(), NewPool(), PoolConfig(), poolMaxConnsForWorkers(), redactedTarget(), blackHoleAddr(), TestPoolConfig_AppliesExplicitBounds() (+6 more)

### Community 65 - "Phase 24 Optimistic UI & Notes"
Cohesion: 0.16
Nodes (16): D-03: Optimistic chip add/remove with rollback, D-05: Note clamped to 2 lines with more expander, D-06: Note edited in place with Save/Cancel and counter, D-10: Notes on watchlist row, tags on artists.id, D-11: Counts include only watched artists (carriers), D-20: Attach/detach routes under /watchlist/{id}/tags, D-22: GET/PATCH/DELETE /tags and POST /tags/{id}/merge; 409 on collision, D-24: Functional addTag/removeTag updaters with per-row pending set (+8 more)

### Community 66 - "Phase 25 Release-Date Enrichment"
Cohesion: 0.19
Nodes (15): Plan 25-01: latest/next release dates on GET /watchlist, Plan 25-03: card shows latest and upcoming release, G1: latest_release_date ignores retention, G2: upcoming = strictly after UTC today at own precision, latest_release_date field, next_release_date field (G3), WLVW-03 Sort by latest release, Phase 25 Research (+7 more)

### Community 67 - "Artist Search Stubs"
Cohesion: 0.19
Nodes (8): perArtistOutcome, perArtistSearcher, stubArtistFetcher, stubSearcher, stubSearcherByQuery, artistSearchResponse, stubDeezerArtistSearcher, Artist

### Community 68 - "Digest Chunk Builder"
Cohesion: 0.21
Nodes (13): continuationHeading(), continuationNote(), positionIndicator(), renderGroup(), splitOversizedGroup(), TestContinuationHeading_ExactWording(), TestContinuationNote_NoDigit(), TestPositionIndicator_EmptyAtTotalOne() (+5 more)

### Community 69 - "Phase 24 UI Contract Concepts"
Cohesion: 0.16
Nodes (14): D-07: Manage tags dialog from Watchlist header, D-16: Reusable AlertDialog confirm for merge/delete, G-24-4: Long chip truncation (fixture issue, not a defect), Phase 24 UI Review (6-pillar audit), 6-pillar UI audit: 17/24, Phase 24 UI Design Contract, Tag chip treatment distinct from status/event badges [R1], ConfirmDialog (reusable AlertDialog wrapper) (+6 more)

### Community 70 - "Events Service Tests (70)"
Cohesion: 0.24
Nodes (10): stubEventsStore, Service, eventsResponse, eventsResponseBody, stubEventsStore, Event, ListParams, Page (+2 more)

### Community 71 - "MusicBrainz Releases"
Cohesion: 0.21
Nodes (6): noReleaseDetailSource, Client, Release, Medium, releaseEnvelope, fakeReleaseDetailSource

### Community 72 - "Phase 25 URL State & Name Filter"
Cohesion: 0.21
Nodes (13): Plan 25-04: name filter, count, URL state, sticky cards, D-04: accent-insensitive name substring search, D-10: three distinct empty states + Clear filters, D-12: filter state in URL query params, G4: sticky cards, G5: name fold table ($ to s, o-slash to o, ae, ss), useUrlParams hook (no code analog), foldName() helper (+5 more)

### Community 73 - "Server Entrypoint"
Cohesion: 0.27
Nodes (9): logInstanceGateStatus(), main(), run(), decodeRecord(), nonEmptyLines(), recordMentions(), TestLogInstanceGateStatus(), TestRun_BootServesHealthThenGracefulShutdownOnCancel() (+1 more)

### Community 74 - "v1.6 Phases & Requirements (74)"
Cohesion: 0.17
Nodes (12): Filter vs Search, Latest Release, Upcoming Release, Option D: Upcoming Releases Calendar, BULK-01..05 Bulk Edit Requirements, IMPT-01..05 Paste-a-List Bulk Add Requirements, WLVW-01..06 + HIST-02 Find & Filter Requirements, ROADMAP.md (v1.6 Phases 24-28) (+4 more)

### Community 75 - "Discord Webhook Client"
Cohesion: 0.29
Nodes (8): allowedMentions, EmbedImage, retry429Body, webhookPayload, Client, Embed, EmbedField, fakeSender

### Community 76 - "Discord Client Retry Tests"
Cohesion: 0.30
Nodes (11): NewClient(), TestSend_400_ReturnsErrorNotMatchingSentinel(), TestSend_429Exhausted_ErrorNeverLeaksBodyOrToken(), TestSend_429RetryAfterClamped(), TestSend_429ThenSuccess_HonorsRetryAfter(), TestSend_429Twice_ReturnsErrorAfterSingleRetry(), TestSend_AllowedMentionsAlwaysSuppressed(), TestSend_ErrorPaths_NeverLeakTokenOrBody() (+3 more)

### Community 77 - "Activity Gate"
Cohesion: 0.27
Nodes (7): ActivityGate, NewActivityGate(), TestActivityGate_ActiveWhileBegunNotEnded(), TestActivityGate_ConcurrentUse(), TestActivityGate_DoubleEndDoesNotCorruptState(), TestActivityGate_FreshGateIsNotActive(), TestActivityGate_TwoConcurrentBeginsBothMustEnd()

### Community 78 - "v1.6 Phases & Requirements (78)"
Cohesion: 0.18
Nodes (11): Carrier, Tag Merge, Note (watchlist entry annotation), Tag, Tag Link (belongs to artist), Per-artist 10-tag Cap via Locking Trigger (ADR 0004), NOTE-01 Artist Note, NTFY-05..07 Tags on Discord Notifications (+3 more)

### Community 79 - "Phase 24 Pattern Map Concepts"
Cohesion: 0.20
Nodes (11): D-21: Idempotent attach (200) and detach (204), D-26: POST/PATCH /watchlist return tags+note via same enrichment; tags never null, Three-layer validation (handler, service, DB CHECK), Phase 24 Pattern Map, apiFetch as single client call path, :execrows idempotent delete, Handler skeleton: parse id, bound body, decode, validate, service, switch sentinel, Non-nil slice guarantee (+3 more)

### Community 80 - "Phase 24 Postgres Research Findings"
Cohesion: 0.22
Nodes (11): D-29: One-statement get-or-create via ON CONFLICT (lower(name)) DO UPDATE, D-31: Non-ASCII case identity tests and NFC normalization, Constraint-name error mapping (pgErr.ConstraintName), Phase 24 Research, BEFORE INSERT trigger fires before ON CONFLICT arbitration, Postgres lower() non-ASCII folding depends on collation (assumption), Expression-index ON CONFLICT target and RAISE USING CONSTRAINT, Parallel array_agg tag enrichment, not json_agg (+3 more)

### Community 81 - "Phase 24 Review, UAT & Verification"
Cohesion: 0.18
Nodes (11): Requirements TAG-01..07, NOTE-01, Phase 24 Code Review (24-14 increment), IN-01/02: tailwind-merge misclassifies text-label as colour, IN-03/04: Test hygiene and missing G-24-10 branch tests, WR-01: Hand-off lands on stale rename row's Cancel, Phase 24 UAT, G-24-14: Textarea 14px reading from stale go:embed build, Phase 24 Validation Strategy (+3 more)

### Community 82 - "Phase 25 History Tag Filter Plans"
Cohesion: 0.35
Nodes (10): Plan 25-02: GET /events tag_id filter with retention, Plan 25-06: History Tag control, Plan 25-09: History filters move to URL params, Plan 25-10: rebuild embedded SPA and phase DoD gate, D-11: History single-select tag via scalar tag_id narg, G6: History URL state as its own plan, HIST-02 History tag filter, Pattern 2: tag_id narg EXISTS over artist_tags in retention-aware ListEvents/HasOlderEvents (+2 more)

### Community 83 - "Web package.json Scripts"
Cohesion: 0.18
Nodes (10): name, private, scripts, build, dev, format, test, test:watch (+2 more)

### Community 84 - "Search Cancellation Stubs"
Cohesion: 0.22
Nodes (6): cancelingSearcher, recordingTimeSearcher, ArtistSearcher, context.CancelFunc, io.ReadCloser, cancelReadCloser

### Community 85 - "Migration-Check Git Diff"
Cohesion: 0.27
Nodes (10): appendGithubOutput(), diffRange(), filterMigrationUpFiles(), runChangedFiles(), TestDiffRange(), TestFilterMigrationUpFiles_KeepsOnlyTheGlob(), TestValidCommitishAndValidBranchRef(), withStubCommitExists() (+2 more)

### Community 86 - "Codebase Architecture Map"
Cohesion: 0.36
Nodes (8): deezer.Client (internal/deezer/client.go), Detector (internal/detection/detector.go), discord.Client (internal/discord/client.go), events.Service.List (internal/events/service.go), handleSearch (internal/httpserver/search.go), musicbrainz.Client (internal/musicbrainz/client.go), RunDeezerCycle (internal/poller/poller.go), RunMusicBrainzCycle (internal/poller/poller.go)

### Community 87 - "MusicBrainz Artist Lookup"
Cohesion: 0.33
Nodes (6): stubArtistDetailLookup, ArtistDetail, Client, ArtistAlias, ArtistRelation, ArtistRelationURL

### Community 88 - "MusicBrainz Artist Search"
Cohesion: 0.31
Nodes (6): stubMusicBrainzArtistSearcher, clampLimit(), escapeLucene(), Artist, Client, artistSearchResponse

### Community 89 - "Per-Artist Tag Cap Decisions"
Cohesion: 0.28
Nodes (9): Phase 24 Context: Artist Tags & Notes, ADR 0004: Per-artist tag cap trigger, D-18: 10-per-artist cap as BEFORE INSERT trigger, D-19: Merge deletes duplicates then UPDATEs, never inserts, Phase 24 Discussion Log, Claude's discretion: cap mechanism, collision API shape, enrichment form, FOR NO KEY UPDATE does not block FK FOR KEY SHARE, Row visibility within multi-row INSERT (Phase 26 bulk attach) (+1 more)

### Community 90 - "Phase 24 Security Threats"
Cohesion: 0.22
Nodes (9): D-09: Inline rename; collision opens merge confirmation, D-30: Tag vocabulary loads on first use, Phase 24 Security Threat Register, Generation-guarded loads (vocabGen, loadGen) T-24-54/58, Migrations 000011/000012: cap trigger UPDATE path and CR-01 lock fix, Stored XSS mitigated by plain JSX text (no dangerouslySetInnerHTML), T-24-12: Silent merge on rename prevented by 409 CollisionError, T-24-63: Confirm titles overflowing hide tag names (+1 more)

### Community 91 - "Release Detector"
Cohesion: 0.36
Nodes (5): Option, RecordingSource, ReleaseDetailSource, Detector, WithNotifyMaxReleaseAgeDays()

### Community 92 - "Single-Instance ADRs (92)"
Cohesion: 0.25
Nodes (8): Single-instance constraint (no leader election in robfig/cron), ADR 0002: One outbox, one sender lock, CAS-skip on held lock, Digest mode (v1.5), Notification outbox (pending events), Notifier.notifying shared CAS sender lock, Postgres advisory lock (rejected option), ADR 0003: Digest ack splits event-ack from completion

### Community 93 - "Query Refs Tests"
Cohesion: 0.46
Nodes (7): mustHigh(), mustNotHigh(), readPrevReleaseFixture(), refsFor(), TestQueryColumnRefs(), TestQueryColumnRefs_IdentifierCaseFolding(), TestRefSet_MergeAndLookups()

### Community 94 - "SPA Embed Handler"
Cohesion: 0.39
Nodes (6): Handler(), firstJSAsset(), TestHandler_MissingAssetPathFallsBackInsteadOf404(), TestHandler_RootPathServesIndexHTML(), TestHandler_ServesRealEmbeddedAssetDirectly(), TestHandler_UnknownPathFallsBackToIndexHTML()

### Community 95 - "End-to-End Boot Tests"
Cohesion: 0.60
Nodes (4): RowQuerier, SchemaVersionReader, NewSchemaVersionReader(), SchemaVersion()

### Community 96 - "Single-Instance ADRs (96)"
Cohesion: 0.33
Nodes (6): ADR 0001: In-process ring buffer for poll-run history, poll_runs table (rejected option), CONTEXT.md glossary vocabulary, Domain Docs agent guide, Flag ADR conflicts rule, n1-boot CI job

### Community 97 - "Status Snapshot & Ring Buffer (97)"
Cohesion: 0.40
Nodes (6): In-process poll-run ring buffer (pollruns.Store), poller.RunRecorder seam, httpserver.StatusStore seam, handleStatus (internal/httpserver/status.go), gate.Authenticate passphrase gate (X-Instance-Gated), pollruns.Snapshot()

### Community 98 - "Digest Ack Split"
Cohesion: 0.40
Nodes (6): AckDigestBatch query, AckEventsOnly query, digest_last_slot_at / digest_last_sent_at, AND notified_at IS NULL idempotence predicate, Split event-ack from digest completion, context.WithoutCancel ack bounded by dbOpTimeout

### Community 99 - "Tag Cap Locking & Merge"
Cohesion: 0.33
Nodes (6): ADR 0004: Per-artist tag cap locking trigger, artist_tags_max_per_artist constraint name, Migration 000011 (cap trigger on UPDATE OF artist_id), Migration 000012 (FOR KEY SHARE vs concurrent detach), TAG-04 (10 tags per artist cap), artist_tags cap trigger (locking, skip-existing)

### Community 100 - "Migration Rollback Rules"
Cohesion: 0.40
Nodes (6): internal/sqlscan dollar-quoted body handling, migration-check:allow-destructive annotation, Rule 1: backward-incompatible changes break rollback, cmd/migration-check CI guard, Migrations rollback-safety rules, Rule 2: unsafe-forward changes break or lock the deploy

### Community 101 - "Status Snapshot & Ring Buffer (101)"
Cohesion: 0.33
Nodes (6): Tag merge via UPDATE tag_id (never insert), schema_applied / schema_expected version seam, internal/db ahead-of-source no-op test, Expand / backfill / contract pattern (000006, 000007), N-1 rollback invariant, RunMigrations (boot-time embedded migrations)

### Community 102 - "Web Template & Status Client"
Cohesion: 0.33
Nodes (6): /status run object, SPA API client (web/app/lib/api.ts), GET /status frozen response contract, pnpm allowBuilds (esbuild yes, msw no), pnpm transitive CVE overrides (nanoid, postcss, browserslist, fast-uri, js-yaml), React Router + shadcn/ui web template

### Community 103 - "Login Throttle Tests (103)"
Cohesion: 0.47
Nodes (4): IsWeakPassphrase(), TestIsWeakPassphrase(), TestWeakPassphraseBootWarn_EmptyOrStrongLogsNothing(), TestWeakPassphraseBootWarn_OneWarnLineNeverContainsValue()

### Community 104 - "Build Info"
Cohesion: 0.33
Nodes (4): Short(), TestShort_Truncates(), TestVersion_DefaultsToDev(), TestVersion_Injected()

### Community 105 - "Combobox Interaction"
Cohesion: 0.47
Nodes (5): G7: keep both comboboxes, Combobox(), commitSelection(), handleTriggerKeyDown(), openAt()

### Community 106 - "Digest Mode Concepts"
Cohesion: 0.40
Nodes (5): Digest Mode, Digest Window, Flush, Outbox, SettingsReader Required Constructor Arg, Fail-closed Gate

### Community 107 - "Digest Slot & Watermark"
Cohesion: 0.40
Nodes (5): Digest Slot (00:05 America/New_York), Grace Window (12h daily / 48h weekly), Slot Record (digest_last_slot_at), Watermark (last digest sent), Group-preferred Digest Chunking with Per-chunk Ack

### Community 108 - "Deezer Client"
Cohesion: 0.50
Nodes (3): AlbumLister, APIError, errorProbe

### Community 109 - "v1.4 Observability Milestone (109)"
Cohesion: 0.67
Nodes (3): Option C: Operator Status Panel + /ready, Don't Roadmap Phases Gated on Uncontrolled Infra, Phase 17: Automated VPS Deploy (deferred)

## Knowledge Gaps
- **208 isolated node(s):** `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest`, `notification_settings`, `Queries` (+203 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 417 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **25 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `Phase 25 Pattern Map` connect `Frontend API Client & Watchlist Route` to `Phase 25 Release-Date Enrichment`, `shadcn UI Primitives`, `Phase 25 URL State & Name Filter`, `History Route & Event Cards`, `Tag Chips & Combobox UI`, `Phase 25 History Tag Filter Plans`, `Artist Note & Preference Toggles`, `Watchlist Service`, `Phase 25 Tag Filter Plans`?**
  _High betweenness centrality (0.219) - this node is a cross-community bridge._
- **Why does `NewService()` connect `Watchlist & Detector DB Tests` to `Notifier Core`, `Server Entrypoint`, `Watchlist HTTP Tests`, `Watchlist Service`?**
  _High betweenness centrality (0.159) - this node is a cross-community bridge._
- **Why does `Phase 25 Context: Find & Filter` connect `Phase 25 Tag Filter Plans` to `Phase 25 Release-Date Enrichment`, `HTTP Handlers & JSON Helpers (3)`, `Frontend API Client & Watchlist Route`, `Phase 25 URL State & Name Filter`, `Combobox Interaction`, `Phase 25 History Tag Filter Plans`, `Per-Artist Tag Cap Decisions`, `Watchlist Service`?**
  _High betweenness centrality (0.115) - this node is a cross-community bridge._
- **What connects `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest` to the rest of the system?**
  _208 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Watchlist & Detector DB Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.053376623376623376 - nodes in this community are weakly interconnected._
- **Should `Poller Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.06086956521739131 - nodes in this community are weakly interconnected._
- **Should `Watchlist HTTP Tests` be split into smaller, more focused modules?**
  _Cohesion score 0.05929989550679206 - nodes in this community are weakly interconnected._