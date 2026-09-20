# Graph Report - drop-tracker  (2026-09-19)

## Corpus Check
- cluster-only mode -- file stats not available

## Summary
- 2635 nodes · 8208 edges · 149 communities (92 shown, 19 thin omitted)
- Extraction: 89% EXTRACTED · 11% INFERRED · 0% AMBIGUOUS · INFERRED: 931 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- About Instance & UI Primitives
- Watchlist App & Empty State UI
- Source History Panel UI
- Search Results UI Components
- Event Card & History UI
- Digest Settings & Status UI
- Digest Settings Frontend Logic
- Passphrase Screen & Session
- Watchlist Service Tests
- Poller Cron Cycle
- Error Redaction & SPA Embed
- Migration Rollback-Safety Rules
- v1.3 Requirements Archive
- MusicBrainz Artist Lookup Client
- v1.2 Roadmap
- Notify-Pending Timeout Tests
- SQL Statement Lexer
- Artist Matcher Tie-Break Tests
- GitHub Issue Tracker Guide
- GitHub Repo Reference
- v1.4 Requirements Archive
- Migration-Check CI Tool
- Domain Docs Guide
- Discord Webhook Client
- v1.3 Roadmap
- v1.4 Roadmap
- Shared HTTP Client & Rate Limiter
- Discord Send Retry Tests
- React Package
- React Router Dev Package
- Tailwind Vite Plugin
- Testing Library Jest-DOM
- Tailwind Animate CSS
- React Type Defs
- Detection Filter Rules
- Poll Activity Gate
- Watchlist Normalize & Preferences
- Cancelable Artist Searcher
- Server Boot & Main Entry
- Events Service List Tests
- Digest Scheduler Core
- Watchlist & Events Migrations
- Vitest Package
- Events & Health Handlers
- Login Audit & Rate Limits
- Baseline & Filter Tests
- /status Contract Doc
- Query Column Refs Tests
- SQL DDL Parser Tests
- MusicBrainz Recording Source
- ADR: Poll-Run Ring Buffer
- TypeScript Config
- Frontend Dependencies
- Config Loader Tests
- Discord Embed Formatting Tests
- MusicBrainz Release Groups
- Frontend Dev Tooling
- Digest Scheduler & Cadence
- Artist Art Backfill
- Raw SQL Statement Type
- Build Info Version
- Events SQL Queries
- Digest Chunk Rendering & Continuation
- Artist Matcher Core
- v1.5 Roadmap
- Alerter Selection (Discord/No-op)
- shadcn Component Config
- Schema Version & Boot E2E Tests
- v1.5 Requirements Archive
- Auth Session Window Tests
- Server Config & Log Redaction
- ADR: One Outbox, One Sender Lock
- ADR: Digest Ack Split
- Sync Buffer Test Helper
- Domain Context Doc
- DB Transaction Wrapper
- Migration: Notification Settings (up)
- Postgres Pool Config
- Auth Gate Session Fakes
- MusicBrainz Artist Albums
- Migrate CLI Tool
- Login/Logout Handlers
- Stale Release Suppression
- MusicBrainz Release Detail
- Settings Handler Tests
- Migrations README Test
- Deezer Artist Search
- Coverage Report CLI
- HTTP Server Core & Router
- DDL Alter/Create Actions
- Events Cursor Pagination
- Notifier & Digest Ack
- npm Scripts
- Digest Chunk Builder Tests
- SQL Column Reference Extraction
- Readiness Probe Tests
- HTTP Request/Response Types
- clsx Utility
- Health Response Type
- Deezer Detection Tests
- Artist Store & Upsert
- TypeScript Package
- MusicBrainz Lookup Tests
- Digest Chunk Formatting Tests
- Digest Line Formatting & Collation
- Broken Windows Defect Ledger
- v1.1 Frontend Test Requirements
- Pre-commit Hooks Config
- v1.1 Milestone Audit
- v1.0 Milestone Audit
- Frontend README & Workspace

## God Nodes (most connected - your core abstractions)
1. `New()` - 207 edges
2. `NewTestPool()` - 137 edges
3. `New()` - 101 edges
4. `NewIsolatedTestPool()` - 84 edges
5. `discardLogger()` - 82 edges
6. `New()` - 79 edges
7. `newTestLogger()` - 77 edges
8. `cn()` - 64 edges
9. `newTestLogger()` - 64 edges
10. `New()` - 60 edges

## Surprising Connections (you probably didn't know these)
- `Graceful shutdown via signal.NotifyContext` --references--> `run()`  [EXTRACTED]
  .planning/PROJECT.md → cmd/server/main.go
- `caarlos0/env config parsing` --references--> `Load()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/config/config.go
- `handleHealth (internal/httpserver/health.go)` --shares_data_with--> `NewPool()`  [INFERRED]
  .planning/codebase/ARCHITECTURE.md → internal/db/pool.go
- `React (Vite) SPA embedded via go:embed` --references--> `Handler()`  [EXTRACTED]
  .planning/PROJECT.md → internal/webassets/embed.go
- `pgx/v5 Postgres driver` --references--> `NewPool()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/db/pool.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Full Pipeline CI/CD job set (lint, test, scan, sbom, release)** — github_workflows_full_pipeline_doc, tech_golangci_lint_v2, tech_trivy, tech_gitleaks, tech_svu, tech_syft_sbom [EXTRACTED 1.00]
- **Phase 12 scope: CoverArt reset plus search popularity/disambiguation work** — phase12_cleanup, coverart_reset_bug, search_popularity_disambiguation, deezer_fan_count_capture, musicbrainz_country_fallback [EXTRACTED 1.00]

## Communities (149 total, 19 thin omitted)

### Community 15 - "About Instance & UI Primitives"
Cohesion: 0.10
Nodes (36): AboutInstanceProps, Cadence, SaveStatus, StatusInstance, Avatar(), AvatarBadge(), AvatarFallback(), AvatarGroup() (+28 more)

### Community 234 - "Watchlist App & Empty State UI"
Cohesion: 0.15
Nodes (15): EmptyStateProps, EmptyState(), addWatchlist(), removeWatchlist(), renderRoute(), App(), renderAppAt(), renderUnderApp() (+7 more)

### Community 36 - "Source History Panel UI"
Cohesion: 0.13
Nodes (29): OutcomeTier, SourceHistoryTableProps, SourcePanelProps, StatusRun, StatusSource, classifyOutcome(), OutcomeBadge(), titleCase() (+21 more)

### Community 37 - "Search Results UI Components"
Cohesion: 0.08
Nodes (34): CoverArtProps, PreferenceTogglesProps, SearchResultRowProps, SearchResultsColumnsProps, SourceColumnProps, WatchlistRowProps, SearchArtist, SourceResult (+26 more)

### Community 43 - "Event Card & History UI"
Cohesion: 0.07
Nodes (29): EventCardProps, ComboboxOption, ComboboxProps, HistoryFiltersProps, HistoryFiltersValue, EventItem, EventCard(), GuestFeatureBody() (+21 more)

### Community 49 - "Digest Settings & Status UI"
Cohesion: 0.13
Nodes (18): DigestSettingsProps, NotificationSettings, StatusResponse, AboutInstance(), Alert(), AlertAction(), AlertDescription(), AlertTitle() (+10 more)

### Community 78 - "Digest Settings Frontend Logic"
Cohesion: 0.11
Nodes (19): SearchBoxProps, ApiError, EventsPage, KnownOutcome, SearchResponse, DigestSettings(), handleCadenceChange(), handleDigestModeChange() (+11 more)

### Community 14 - "Passphrase Screen & Session"
Cohesion: 0.08
Nodes (14): PassphraseScreen(), handleSubmit(), Button(), Input(), Toaster(), createSession(), deleteSession(), reimportStore() (+6 more)

### Community 0 - "Watchlist Service Tests"
Cohesion: 0.13
Nodes (65): New(), TestSQLCPing(), TestCountWatchlist_Integration(), TestWatchlist_Delete_ConcurrentSameIDYieldsOne204AndOne404(), TestWatchlist_FullLifecycle(), TestWatchlist_Patch_ConcurrentDifferentAxesBothSurvive(), TestWatchlist_Patch_ConcurrentWithDeleteNeverReturns500(), TestWatchlist_Patch_EmptyBodyStillRejectedEndToEnd() (+57 more)

### Community 1 - "Poller Cron Cycle"
Cohesion: 0.07
Nodes (99): cron.Cron, New(), decodeLogRecords(), deezerID(), eightEntries(), failFirstThree(), newTestLogger(), newTestPoller() (+91 more)

### Community 10 - "Error Redaction & SPA Embed"
Cohesion: 0.06
Nodes (50): redactError(), TestRedactError_KeepsDiagnosticContext(), TestRedactError_LeavesNonDSNTextAlone(), TestRedactError_NeverEchoesPassword(), wrapAsDriverError(), Handler(), firstJSAsset(), TestHandler_MissingAssetPathFallsBackInsteadOf404() (+42 more)

### Community 100 - "Migration Rollback-Safety Rules"
Cohesion: 0.29
Nodes (6): Before you merge a migration, Expand, backfill, contract: a walkthrough already in this tree, Migrations: rollback-safety rules, The N-1 invariant, The two rules, What `cmd/migration-check` enforces

### Community 108 - "v1.3 Requirements Archive"
Cohesion: 0.14
Nodes (13): Access Gate, Access Gate, CI/CD Pipeline, CI/CD Pipeline, Deployment, Deployment / Operations, Future Requirements, Migration Safety (+5 more)

### Community 110 - "MusicBrainz Artist Lookup Client"
Cohesion: 0.33
Nodes (6): stubArtistDetailLookup, ArtistDetail, Client, ArtistAlias, ArtistRelation, ArtistRelationURL

### Community 112 - "v1.2 Roadmap"
Cohesion: 0.22
Nodes (8): Milestone Summary, Overview, Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking, Phase 13: Fix History Dates, Guest-Feature Art & Artist Art, Phase Details, Phases, Progress, Roadmap: drop-tracker

### Community 113 - "Notify-Pending Timeout Tests"
Cohesion: 0.10
Nodes (23): stubDigestOff, sync/atomic.Int32, fakeSettingsStore, callNotifyPending(), discardLogger(), shrinkDBOpTimeout(), TestNotifyPending_ParentCancellationStillPropagates(), TestNotifyPending_RecoversAfterUnresponsiveDatabase() (+15 more)

### Community 118 - "SQL Statement Lexer"
Cohesion: 0.18
Nodes (16): strings.Builder, TestCopyDollarQuoted_NoClosingTagCopiesRemainder(), TestDollarTagAt(), TestFindFromJoinTables_AdjacentFromJoinBothFound(), copyDollarQuoted(), copySingleQuoted(), dollarTagAt(), RawStatement (+8 more)

### Community 12 - "Artist Matcher Tie-Break Tests"
Cohesion: 0.13
Nodes (37): stubSearcherFunc, NewMatcher(), TestMatch_EmptyNameFailsClosedWithoutOutboundCall(), TestMatch_MatchedCandidateWithEmptyPictureYieldsNilImageURL(), TestMatch_NoCloseNameCandidateFailsClosed(), TestMatch_SearchErrorSurfaces(), TestMatch_SingleCandidateIssuesNoTieBreakFetch(), TestMatch_SingleCloseNameCandidate() (+29 more)

### Community 120 - "GitHub Issue Tracker Guide"
Cohesion: 0.29
Nodes (6): Conventions, Issue tracker: GitHub, Pull requests as a triage surface, Wayfinding operations, When a skill says "fetch the relevant ticket", When a skill says "publish to the issue tracker"

### Community 129 - "v1.4 Requirements Archive"
Cohesion: 0.15
Nodes (12): Future Requirements, Observability, Operations / Deployment (carried from v1.3), Out of Scope, Poll Run History, Readiness, Requirements Archive: v1.4 Operator Observability, Requirements: drop-tracker (+4 more)

### Community 13 - "Migration-Check CI Tool"
Cohesion: 0.06
Nodes (70): appendGithubOutput(), buildPrevReleaseRefs(), buildReport(), classifyAction(), crossReferenceFinding(), diffRange(), filterMigrationUpFiles(), main() (+62 more)

### Community 130 - "Domain Docs Guide"
Cohesion: 0.33
Nodes (5): Before exploring, read these, Domain Docs, File structure, Flag ADR conflicts, Use the glossary's vocabulary

### Community 131 - "Discord Webhook Client"
Cohesion: 0.29
Nodes (8): allowedMentions, EmbedImage, retry429Body, webhookPayload, Client, Embed, EmbedField, fakeSender

### Community 141 - "v1.3 Roadmap"
Cohesion: 0.17
Nodes (11): Backlog, Milestones, Overview, Phase 14: Instance Passphrase Gate, Phase 15: PR Coverage-Diff Comment, Phase 16: Rollback-Safe Migrations, Phase 17: Automated VPS Deploy with Health-Gated Rollback, Phase Details (+3 more)

### Community 146 - "v1.4 Roadmap"
Cohesion: 0.17
Nodes (11): Backlog, Milestones, Overview, Phase 18.1: Poll-Cycle Instrumentation, Phase 18: Backend — Readiness, Status Surface & App Version, Phase 19: Frontend — System View, Phase Details, Phases (+3 more)

### Community 16 - "Shared HTTP Client & Rate Limiter"
Cohesion: 0.05
Nodes (65): AlbumLister, APIError, errorProbe, golang.org/x/time/rate.Limiter, net/http.Client, net/http.Response, net/http.RoundTripper, capturingRoundTripper (+57 more)

### Community 162 - "Discord Send Retry Tests"
Cohesion: 0.30
Nodes (11): NewClient(), TestSend_400_ReturnsErrorNotMatchingSentinel(), TestSend_429Exhausted_ErrorNeverLeaksBodyOrToken(), TestSend_429RetryAfterClamped(), TestSend_429ThenSuccess_HonorsRetryAfter(), TestSend_429Twice_ReturnsErrorAfterSingleRetry(), TestSend_AllowedMentionsAlwaysSuppressed(), TestSend_ErrorPaths_NeverLeakTokenOrBody() (+3 more)

### Community 17 - "Detection Filter Rules"
Cohesion: 0.13
Nodes (24): Detector, newNotifyGate(), nullableString(), deluxeDetectionEnabled(), eventTypeMuted(), releaseTypeAllowed(), TestFilter_DeluxeIsAGateNotAType(), coverArtURLForReleaseGroup() (+16 more)

### Community 177 - "Poll Activity Gate"
Cohesion: 0.27
Nodes (7): ActivityGate, NewActivityGate(), TestActivityGate_ActiveWhileBegunNotEnded(), TestActivityGate_ConcurrentUse(), TestActivityGate_DoubleEndDoesNotCorruptState(), TestActivityGate_FreshGateIsNotActive(), TestActivityGate_TwoConcurrentBeginsBothMustEnd()

### Community 18 - "Watchlist Normalize & Preferences"
Cohesion: 0.13
Nodes (12): stubStore, TestNormalizeSet(), AddParams, Entry, PreferencesParams, normalizeSet(), toEntry(), fakeEventRecorder (+4 more)

### Community 180 - "Cancelable Artist Searcher"
Cohesion: 0.22
Nodes (6): cancelingSearcher, recordingTimeSearcher, ArtistSearcher, context.CancelFunc, io.ReadCloser, cancelReadCloser

### Community 183 - "Server Boot & Main Entry"
Cohesion: 0.07
Nodes (33): logInstanceGateStatus(), main(), run(), decodeRecord(), nonEmptyLines(), recordMentions(), TestLogInstanceGateStatus(), TestRun_BootServesHealthThenGracefulShutdownOnCancel() (+25 more)

### Community 184 - "Events Service List Tests"
Cohesion: 0.30
Nodes (21): NewService(), datePtr(), insertTestArtist(), insertTestEvent(), insertTestEventAt(), insertTestEventTyped(), insertTestEventWithDate(), TestHandleListEvents_CursorRoundTripsThroughHTTP() (+13 more)

### Community 19 - "Digest Scheduler Core"
Cohesion: 0.14
Nodes (26): globalCounter, sync.Mutex, time.Time, defaultTickSource(), DigestScheduler, NewDigestScheduler(), neverFiringTickSource(), newFakeSink() (+18 more)

### Community 194 - "Watchlist & Events Migrations"
Cohesion: 0.50
Nodes (3): artists, watchlist, events

### Community 2 - "Events & Health Handlers"
Cohesion: 0.08
Nodes (60): errorBody, healthBody, watchlistEntryBody, TestHandleListEvents_CursorRejection(), TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(), TestHandleListEvents_HappyPathReturns200WithEnvelope(), TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray(), TestHandleListEvents_StoreErrorReturns500WithFixedMessage() (+52 more)

### Community 20 - "Login Audit & Rate Limits"
Cohesion: 0.29
Nodes (24): recordingAlerter, golang.org/x/time/rate.Limit, SetLoginBurstForTest(), SetLoginDelayForTest(), SetLoginRateForTest(), NewManager(), loginReq(), noCSRFLoginReq() (+16 more)

### Community 21 - "Baseline & Filter Tests"
Cohesion: 0.20
Nodes (21): insertBaselineNewRelease(), int32Ptr(), readBaselineTrackCount(), TestAdvanceGroupBaseline_ConcurrentRace(), TestAdvanceGroupBaseline_SingleCallerContract(), filterTestLogger(), filterTestMBID(), insertFilterTestArtist() (+13 more)

### Community 215 - "/status Contract Doc"
Cohesion: 0.25
Nodes (7): Authentication and status codes, Changing this contract, Envelope, Example — fresh instance (no cycles recorded), Example — instance with history, `GET /status` — frozen response contract, Run object

### Community 216 - "Query Column Refs Tests"
Cohesion: 0.46
Nodes (7): mustHigh(), mustNotHigh(), readPrevReleaseFixture(), refsFor(), TestQueryColumnRefs(), TestQueryColumnRefs_IdentifierCaseFolding(), TestRefSet_MergeAndLookups()

### Community 22 - "SQL DDL Parser Tests"
Cohesion: 0.23
Nodes (14): StripIdent(), SplitTopLevelCommas(), TestSplitTopLevelCommas_IgnoresCommasInsideParens(), createTableColumns(), Parse(), TestParse_AddCheckNeverAddColumn(), TestParse_AddColumnNotNullAndDefaultAreIndependent(), TestParse_AlterTableActionsInClauseOrder() (+6 more)

### Community 23 - "MusicBrainz Recording Source"
Cohesion: 0.06
Nodes (24): datedRecordingSource, erroringRecordingSource, fakeRecordingSource, noRecordingSource, context.Context, fakeSchemaVersioner, fakeWatchlistCounter, noopPinger (+16 more)

### Community 233 - "ADR: Poll-Run Ring Buffer"
Cohesion: 0.33
Nodes (5): Consequences, Considered options, Context, Decision, In-process ring buffer for poll-run history

### Community 24 - "TypeScript Config"
Cohesion: 0.08
Nodes (25): compilerOptions, esModuleInterop, jsx, lib, module, moduleResolution, noEmit, paths (+17 more)

### Community 25 - "Frontend Dependencies"
Cohesion: 0.08
Nodes (25): dependencies, @base-ui/react, class-variance-authority, @fontsource-variable/inter, isbot, lucide-react, next-themes, react-dom (+17 more)

### Community 26 - "Config Loader Tests"
Cohesion: 0.18
Nodes (24): Load(), configEnvKeys(), envExampleKeys(), repoRoot(), setDiff(), setRequired(), TestDockerComposeWiresGateEnvVars(), TestDotEnvIsNotTracked() (+16 more)

### Community 27 - "Discord Embed Formatting Tests"
Cohesion: 0.22
Nodes (17): formatEmbed(), emojiPrefix(), i32Ptr(), TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail(), TestFormatEmbed_DeluxeChange_BothCountsPresent(), TestFormatEmbed_DeluxeChange_NilBothCounts(), TestFormatEmbed_DeluxeChange_NilPreviousTrackCount(), TestFormatEmbed_ExternalIDIsPercentEscapedInURL() (+9 more)

### Community 28 - "MusicBrainz Release Groups"
Cohesion: 0.27
Nodes (5): stubGroupLister, Client, ReleaseGroup, releaseGroupEnvelope, fakeReleaseGroupSource

### Community 29 - "Frontend Dev Tooling"
Cohesion: 0.09
Nodes (23): devDependencies, jsdom, prettier, prettier-plugin-tailwindcss, tailwindcss, @testing-library/dom, @testing-library/react, @testing-library/user-event (+15 more)

### Community 3 - "Digest Scheduler & Cadence"
Cohesion: 0.07
Nodes (133): Cadence, bytes.Buffer, github.com/jackc/pgx/v5/pgxpool.Pool, time.Location, insertPendingEventTypedRaw(), mustLoadDigestZone(), newManualTickSource(), newMutableClock() (+125 more)

### Community 30 - "Artist Art Backfill"
Cohesion: 0.26
Nodes (15): Stats, Store, Backfill(), matchingCandidate(), TestBackfill_ActivityGate_DelaysThenProceeds(), TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach(), TestBackfill_ContextCancelledPartway_StopsPromptly(), TestBackfill_ListArtistsMissingImageErrors_ReturnsErrNoWrites() (+7 more)

### Community 31 - "Build Info Version"
Cohesion: 0.33
Nodes (4): Short(), TestShort_Truncates(), TestVersion_DefaultsToDev(), TestVersion_Injected()

### Community 32 - "Events SQL Queries"
Cohesion: 0.13
Nodes (10): Event, recordingQuerier, HasOlderEventsParams, InsertEventParams, ListEventsParams, ListEventsRow, Queries, AdvanceGroupTrackCountBaselineParams (+2 more)

### Community 320 - "Digest Chunk Rendering & Continuation"
Cohesion: 0.21
Nodes (13): continuationHeading(), continuationNote(), positionIndicator(), renderGroup(), splitOversizedGroup(), TestContinuationHeading_ExactWording(), TestContinuationNote_NoDigit(), TestPositionIndicator_EmptyAtTotalOne() (+5 more)

### Community 33 - "Artist Matcher Core"
Cohesion: 0.15
Nodes (20): AlbumLister, ArtistDetailLookup, ArtistFetcher, ArtistSearcher, Matcher, Option, ReleaseGroupLister, aliasQueryNames() (+12 more)

### Community 333 - "v1.5 Roadmap"
Cohesion: 0.15
Nodes (12): Backlog, Milestones, Overview, Phase 20: Digest Settings & Operator Control, Phase 21: Real-Time ↔ Digest Mutual Exclusion, Phase 22: Scheduled Digest Send, Phase 23: Digest Readability & Discord Limits, Phase Details (+4 more)

### Community 334 - "Alerter Selection (Discord/No-op)"
Cohesion: 0.21
Nodes (8): discordAlerter, noopAlerter, Alerter, NoOpAlerter(), SelectAlerter(), TestSelectAlerter_DisabledLogsOneInfoLineAndIsInert(), TestSelectAlerter_EnabledReturnsDiscordBacked(), nonEmptyLines()

### Community 34 - "shadcn Component Config"
Cohesion: 0.09
Nodes (21): aliases, components, hooks, lib, ui, utils, iconLibrary, menuAccent (+13 more)

### Community 340 - "Schema Version & Boot E2E Tests"
Cohesion: 0.17
Nodes (12): ExpectedSchemaVersion(), NewPool(), TestExpectedSchemaVersion(), TestSchemaVersion_Integration(), TestSchemaVersionReader_Delegates(), TestBootToHealth_EndToEnd(), TestBootToHealth_MigrationsAreIdempotent(), TestBootToReady_EndToEnd() (+4 more)

### Community 343 - "v1.5 Requirements Archive"
Cohesion: 0.18
Nodes (10): Digest Configuration, Digest Delivery, Digest Scheduling, Operator Visibility, Out of Scope, Real-Time / Digest Interop, Requirements Archive: v1.5 Digest Notifications, Requirements: drop-tracker (+2 more)

### Community 345 - "Auth Session Window Tests"
Cohesion: 0.24
Nodes (12): time.Duration, SessionWindow(), SetAbsoluteCapForTest(), SetGlobalCounterForTest(), SetLimiterIdleTTLForTest(), SetLimiterSweepIntervalForTest(), SetLoginSleepForTest(), SetMaxConcurrentLoginsForTest() (+4 more)

### Community 35 - "Server Config & Log Redaction"
Cohesion: 0.11
Nodes (23): log/slog.Level, syncBuffer, Config, assertInert(), newCapturingServer(), requestIDsInLog(), TestGatedServer_TrustProxyHeaders_RealIPWiring(), TestInertPath_EmptyPassphraseIsIndistinguishable() (+15 more)

### Community 358 - "ADR: One Outbox, One Sender Lock"
Cohesion: 0.33
Nodes (5): Consequences, Considered options, Context, Decision, One outbox, one sender lock

### Community 359 - "ADR: Digest Ack Split"
Cohesion: 0.33
Nodes (5): Consequences, Considered options, Context, Decision, The digest ack splits event-ack from completion

### Community 38 - "DB Transaction Wrapper"
Cohesion: 0.50
Nodes (3): Queries, pgx.Tx, DBTX

### Community 39 - "Postgres Pool Config"
Cohesion: 0.25
Nodes (12): github.com/jackc/pgx/v5/pgxpool.Config, dsnSetsMaxConns(), PoolConfig(), poolMaxConnsForWorkers(), redactedTarget(), blackHoleAddr(), TestPoolConfig_AppliesExplicitBounds(), TestPoolConfig_ComputesMaxConnsFromPollWorkers() (+4 more)

### Community 4 - "Auth Gate Session Fakes"
Cohesion: 0.10
Nodes (43): fakeAlerter, stubPinger, stubStore, syncBuffer, Token, net/http.Cookie, deleteSession(), discardLogger() (+35 more)

### Community 40 - "MusicBrainz Artist Albums"
Cohesion: 0.27
Nodes (5): stubAlbumLister, artistAlbumsResponse, Album, Client, fakeAlbumSource

### Community 41 - "Migrate CLI Tool"
Cohesion: 0.08
Nodes (48): main(), run(), highestMigrationVersion(), scratchSchemaDSN(), TestRun_AppliesHeadSchema(), TestRun_MissingDatabaseURL(), retryConfig, RetryOption (+40 more)

### Community 42 - "Login/Logout Handlers"
Cohesion: 0.12
Nodes (15): ipLimiter, loginRequest, loginThrottle, sync.Once, time.Ticker, Manager, hasCSRFHeader(), setSessionCookie() (+7 more)

### Community 44 - "Stale Release Suppression"
Cohesion: 0.60
Nodes (4): fmtDate(), TestNewDefaultsMaxReleaseAgeDays(), TestNotifierSuppresses_WiresMaxAgeDays(), TestStaleReleaseDate()

### Community 45 - "MusicBrainz Release Detail"
Cohesion: 0.21
Nodes (6): noReleaseDetailSource, Client, Release, Medium, releaseEnvelope, fakeReleaseDetailSource

### Community 46 - "Settings Handler Tests"
Cohesion: 0.23
Nodes (23): net/http/httptest.Server, settingsBody, settingsServerOpts, assertSettingsDefaults(), getSettingsRaw(), loginForSettings(), mustLoadNYForSettings(), newSettingsRejectionServer() (+15 more)

### Community 48 - "Deezer Artist Search"
Cohesion: 0.19
Nodes (8): perArtistOutcome, perArtistSearcher, stubArtistFetcher, stubSearcher, stubSearcherByQuery, artistSearchResponse, stubDeezerArtistSearcher, Artist

### Community 5 - "Coverage Report CLI"
Cohesion: 0.08
Nodes (54): appendFile(), backendTotalPct(), formatDelta(), frontendLinesPct(), main(), parseBlockLine(), readBackend(), readFrontend() (+46 more)

### Community 50 - "HTTP Server Core & Router"
Cohesion: 0.06
Nodes (55): chi.Router, net/http.Handler, fakeStatusStore, Option, Pinger, readyResponse, serverConfig, settingsResponse (+47 more)

### Community 53 - "DDL Alter/Create Actions"
Cohesion: 0.09
Nodes (14): Action, AlterTable, parseAlterAction(), SchemaColumns(), AddCheck, AddColumn, AlterColumnType, CreateTable (+6 more)

### Community 55 - "Events Cursor Pagination"
Cohesion: 0.17
Nodes (17): stubEventsStore, Cursor, cursorWireForm, Service, eventsResponseBody, stubEventsStore, DecodeCursor(), EncodeCursor() (+9 more)

### Community 58 - "Notifier & Digest Ack"
Cohesion: 0.10
Nodes (21): log/slog.Logger, sync/atomic.Bool, Querier, ackDigestBatch(), ackEventsOnly(), Notifier, Notifier, Sink (+13 more)

### Community 59 - "npm Scripts"
Cohesion: 0.18
Nodes (10): name, private, scripts, build, dev, format, test, test:watch (+2 more)

### Community 6 - "Digest Chunk Builder Tests"
Cohesion: 0.16
Nodes (32): buildDigestGroups(), chunkDigest(), containsID(), makeSyntheticGroup(), stripChunkMarkers(), syntheticInvariantBatch(), TestBuildDigestChunks_CapTotalReflectsKeptChunksNotUncapped(), TestBuildDigestChunks_CapTruncatesAndReportsDeferred() (+24 more)

### Community 60 - "SQL Column Reference Extraction"
Cohesion: 0.21
Nodes (15): NormalizeIdent(), StripSchemaQualifier(), classifyBareColumn(), expandStar(), extractBlockReferences(), extractParams(), findFromJoinTables(), RefSet (+7 more)

### Community 69 - "Readiness Probe Tests"
Cohesion: 0.33
Nodes (17): readyBody, getReady(), schemaAt(), TestReady_AheadOfSource(), TestReady_DBUnreachable(), TestReady_EmptySchemaMigrations(), TestReady_ExactPathOnly(), TestReady_GatedNo401() (+9 more)

### Community 7 - "HTTP Request/Response Types"
Cohesion: 0.13
Nodes (19): net/http.Request, net/http.ResponseWriter, addWatchlistRequest, errorResponse, eventsResponse, updateWatchlistRequest, Server, parseOptionalPageSize() (+11 more)

### Community 8 - "Deezer Detection Tests"
Cohesion: 0.15
Nodes (59): fakeReleaseDetailSource, insertEventFailingQuerier, TestDetectDeezer_FiltersByRecordType(), TestDetectDeezer_NeverProducesDeluxeChange(), TestDetectDeezer_NewRelease(), TestDetectDeezer_ReDetectionInsertsNothing(), TestDetectDeezer_SameIDDifferentSourceCoexist(), TestDetectDeezer_SeedsIndependentlyOfMusicBrainz() (+51 more)

### Community 85 - "Artist Store & Upsert"
Cohesion: 0.07
Nodes (21): stubStore, notifyGate, Option, RecordingSource, ReleaseDetailSource, github.com/jackc/pgx/v5/pgtype.Timestamptz, UpsertArtistParams, Artist (+13 more)

### Community 9 - "MusicBrainz Lookup Tests"
Cohesion: 0.10
Nodes (63): testing.T, TestLookupArtist_DecodesFixture(), TestLookupArtist_EmptyMBID(), TestLookupArtist_MalformedJSON(), TestLookupArtist_NonOKStatus(), TestLookupArtist_NoRelationsNoAliasesYieldsNonNilZeroLengthSlices(), TestLookupArtist_RequestShape(), TestReleasesForRecording_DecodesFixture() (+55 more)

### Community 93 - "Digest Chunk Formatting Tests"
Cohesion: 0.15
Nodes (22): buildDigestChunks(), digestWindowHeader(), remainderMarker(), syntheticEvents(), TestBuildDigestChunks_CapLastChunkCarriesRemainderMarker(), TestBuildDigestChunks_DeluxeTrackCountSuffix(), TestBuildDigestChunks_DuplicateSourcesRenderAsTwoLines(), TestBuildDigestChunks_EmptyGroupOmitsHeadingEntirely() (+14 more)

### Community 98 - "Digest Line Formatting & Collation"
Cohesion: 0.14
Nodes (29): golang.org/x/text/collate.Collator, Event, artistKey(), digestLine(), escapeMarkdown(), lineLabel(), sortDigestGroup(), TestArtistKey() (+21 more)

### Community 11 - "v1.1 Frontend Test Requirements"
Cohesion: 0.47
Nodes (6): TEST-01 Requirement, TEST-02 Requirement, Phase 8: Frontend Test Suite, Phase 9: CI Coverage Gates, v1.1 Requirements Archive, v1.1 Roadmap

### Community 142 - "Pre-commit Hooks Config"
Cohesion: 0.50
Nodes (3): drop-tracker README, Local Gitleaks Pre-commit Secret Scanning Hook, Local golangci-lint Pre-commit Hook (Changed-Files Only)

### Community 74 - "v1.1 Milestone Audit"
Cohesion: 0.67
Nodes (3): CoverArt.tsx Stale Image-Load-Error State Bug, v1.1 Milestone Nyquist Compliance (All 5 Phases), Milestone v1.1 Audit Report

## Knowledge Gaps
- **257 isolated node(s):** `Cadence`, `SaveStatus`, `EmptyStateProps`, `OutcomeTier`, `CoverArtProps` (+252 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 406 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **19 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `Server Boot & Main Entry` to `Watchlist Service Tests`, `Poller Cron Cycle`, `Events & Health Handlers`, `Digest Scheduler & Cadence`, `Deezer Detection Tests`, `Error Redaction & SPA Embed`, `Artist Matcher Tie-Break Tests`, `Shared HTTP Client & Rate Limiter`, `Digest Scheduler Core`, `MusicBrainz Recording Source`, `Config Loader Tests`, `Artist Art Backfill`, `Build Info Version`, `Artist Matcher Core`, `Server Config & Log Redaction`, `Migrate CLI Tool`, `Poll Activity Gate`, `HTTP Server Core & Router`, `Events Service List Tests`, `Notifier & Digest Ack`, `Readiness Probe Tests`, `Alerter Selection (Discord/No-op)`, `Schema Version & Boot E2E Tests`, `Artist Store & Upsert`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Why does `unnotifiedForArtist()` connect `Deezer Detection Tests` to `MusicBrainz Lookup Tests`, `Digest Line Formatting & Collation`, `Notifier & Digest Ack`, `MusicBrainz Recording Source`?**
  _High betweenness centrality (0.019) - this node is a cross-community bridge._
- **Why does `Parse()` connect `SQL DDL Parser Tests` to `DDL Alter/Create Actions`, `SQL Column Reference Extraction`, `Migration-Check CI Tool`, `SQL Statement Lexer`?**
  _High betweenness centrality (0.016) - this node is a cross-community bridge._
- **What connects `Cadence`, `SaveStatus`, `EmptyStateProps` to the rest of the system?**
  _257 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `About Instance & UI Primitives` be split into smaller, more focused modules?**
  _Cohesion score 0.0966183574879227 - nodes in this community are weakly interconnected._
- **Should `Watchlist App & Empty State UI` be split into smaller, more focused modules?**
  _Cohesion score 0.14736842105263157 - nodes in this community are weakly interconnected._
- **Should `Source History Panel UI` be split into smaller, more focused modules?**
  _Cohesion score 0.1253968253968254 - nodes in this community are weakly interconnected._