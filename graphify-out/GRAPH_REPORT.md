# Graph Report - drop-tracker  (2026-09-05)

## Corpus Check
- 263 files · ~445,475 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2816 nodes · 6576 edges · 212 communities (160 shown, 24 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 618 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `563fa51e`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- New
- poller_test.go
- testing.T
- New
- gate_test.go
- coverage-report/main_test.go
- Album
- net/http.Request
- Revisions - 2026-09-04 (post-context grill)
- newTestClient
- codebase/ARCHITECTURE.md
- Phase 8: Frontend Test Suite
- NewMatcher
- migration-check/main_test.go
- authStore.ts
- cn
- newTestClient
- .DetectMusicBrainz
- context.Context
- migration-check/main.go
- time.Duration
- prevReleaseRefs
- Phase 15: PR Coverage-Diff Comment - Discussion Log
- Recording
- compilerOptions
- dependencies
- config_test.go
- formatEmbed
- Pattern Assignments
- devDependencies
- Backfill
- PROJECT.md
- github.com/jackc/pgx/v5/pgtype.Timestamptz
- match.go
- components.json
- NewWithWriter
- Pattern Assignments
- watchlist.tsx
- EventCard.tsx
- PoolConfig
- Implementation Decisions
- RunMigrations
- time.Time
- history.tsx
- Implementation Decisions
- Release
- WatchlistEntry
- Phase 16: Rollback-Safe Migrations - Discussion Log
- Artist
- Artist
- Server
- Pattern Assignments
- Phase 15 Plan 01: cmd/coverage-report Go tool Summary
- Warnings
- Phase 15 Plan 02: Wire the coverage tool into the build Summary
- Phase 15 Plan 03: Wire the coverage tool into CI Summary
- Goal Achievement
- Phase 14 Plan 04: Instance Passphrase Gate — CSRF, Referrer-Policy & Weak-Passphrase WARN Summary
- Phase 16: Rollback-Safe Migrations - Research
- scripts
- Phase 14 Plan 02: Instance Passphrase Gate — Brute-force Defense & Auditability Summary
- Phase 14 — UI Design Contract
- root.tsx
- Phase 14 Plan 05: Passphrase Gate Config Reachability (G-14-1 Closure) Summary
- Phase 16 Plan 03: cmd/migration-check changed-files mode, gitShow seam, and D-15 cross-reference Summary
- Phase 14 Plan 01: Instance Passphrase Gate Summary
- Artist
- Phase 14 Plan 03: SPA Instance Passphrase Gate Summary
- Phase 16 Plan 04: CI Wiring — the changes prelude, migration-check guard, and n1-boot job Summary
- Phase 14: Instance Passphrase Gate - Discussion Log
- Phase 16 Plan 01: Ahead-of-Source Boot Guard & HEAD-Schema Helper Summary
- Phase 16 Plan 02: cmd/migration-check Static DDL Guard Summary
- Goal Achievement
- Phase 14 Plan 07: Close G-14-3 — Self-Identifying Gated Responses Summary
- Milestone v1.1 Audit Report
- v1.0 Milestone Audit Report
- Phase 16 Plan 05: internal/db/migrations/README.md — the expand/contract rule as a standing constraint Summary
- clsx
- api.ts
- healthResponse
- Goal Achievement
- runChangedFiles
- runCapture
- Phase 15: PR Coverage-Diff Comment - Research
- Phase 16 — Security
- watchlist.sql.go
- Common Pitfalls
- typescript
- web/pnpm-workspace.yaml
- Phase 14: Instance Passphrase Gate - Research
- Requirements: drop-tracker
- Tests
- Common Pitfalls
- NewMusicBrainzSource
- Mechanism Verification (the core of this research)
- Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary
- Phase 15 — Validation Strategy
- Phase 16 — Validation Strategy
- Info
- Phase 16: Code Review Report
- Migrations: rollback-safety rules
- Repo Reconnaissance — exact anchors for the 20 decisions
- Phase 15 — Security
- 16-UAT.md
- withStubGitShow
- Broken Windows Ledger cross-phase defect register
- Validation Architecture
- Tests
- ArtistDetail
- Architecture Patterns
- Roadmap: drop-tracker
- Architecture Patterns
- Common Pitfalls
- Phase 14 — Validation Strategy
- GitHub Actions Wiring (D-14, D-16, D-13, D-04)
- Finding 1 (CRITICAL): `migrate.Up()` against an ahead-of-source schema ERRORS — it does not `ErrNoChange`
- stripComments
- Code Examples
- Issue tracker: GitHub
- github.com/danielrpof/drop-tracker
- D-15: extracting the previous release's referenced `(table, column)` set from `queries/*.sql`
- D-08: static destructive-DDL detection in `*.up.sql`
- Validation Architecture
- findFromJoinTables
- 15-01-PLAN.md
- 15-02-PLAN.md
- 15-03-PLAN.md
- Phase 14 — Security
- Domain Docs
- 14-07-PLAN.md
- Code Examples
- Standard Stack
- User Constraints (from 15-CONTEXT.md)
- 14-05-PLAN.md
- 14-06-PLAN.md
- Validation Architecture
- .pre-commit-config.yaml
- Sources
- Code Examples
- User Constraints (from CONTEXT.md)
- Sources
- migrations_readme_test.go
- Standard Stack
- User Constraints (from CONTEXT.md)
- Sources
- Phase 14 — External API Coverage
- Out-of-scope discoveries (not fixed)
- Security Domain
- 16-01-PLAN.md
- 16-02-PLAN.md
- 16-03-PLAN.md
- 16-04-PLAN.md
- 14-01-PLAN.md
- 14-02-PLAN.md
- 14-03-PLAN.md
- 14-04-PLAN.md
- Security Domain
- react
- @react-router/dev
- @tailwindcss/vite
- @testing-library/jest-dom
- tw-animate-css
- @types/react
- 16-05-PLAN.md
- Standard Stack
- Security Domain
- NewActivityGate
- Finding 2 (CRITICAL): a skipped `needs:` job SKIPS its dependents — the D-12/S6 "counts as success" premise is wrong
- ReleaseGroup
- cancelingSearcher
- .HandleLogin
- httpserver/search.go
- run
- events/service.go
- EncodeCursor
- ROADMAP.md
- detector.go
- Handler
- full-pipeline.yml (GitHub Actions workflow)
- IsWeakPassphrase
- redactError
- .handleListEvents
- artists
- vitest

## God Nodes (most connected - your core abstractions)
1. `New()` - 141 edges
2. `NewTestPool()` - 134 edges
3. `New()` - 81 edges
4. `discardLogger()` - 64 edges
5. `New()` - 60 edges
6. `newTestLogger()` - 58 edges
7. `newTestClient()` - 57 edges
8. `NewService()` - 56 edges
9. `insertTestArtist()` - 51 edges
10. `unlimitedLimiter()` - 51 edges

## Surprising Connections (you probably didn't know these)
- `caarlos0/env config parsing` --references--> `Load()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/config/config.go
- `handleHealth (internal/httpserver/health.go)` --shares_data_with--> `NewPool()`  [INFERRED]
  .planning/codebase/ARCHITECTURE.md → internal/db/pool.go
- `Graceful shutdown via signal.NotifyContext` --references--> `run()`  [EXTRACTED]
  .planning/PROJECT.md → cmd/server/main.go
- `.golangci.yml (lint config)` --references--> `redactError()`  [EXTRACTED]
  .golangci.yml → internal/db/migrate.go
- `golang-migrate schema migrations` --references--> `RunMigrations()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/db/migrate.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Full Pipeline CI/CD job set (lint, test, scan, sbom, release)** — github_workflows_full_pipeline_doc, tech_golangci_lint_v2, tech_trivy, tech_gitleaks, tech_svu, tech_syft_sbom [EXTRACTED 1.00]
- **Phase 12 scope: CoverArt reset plus search popularity/disambiguation work** — phase12_cleanup, coverart_reset_bug, search_popularity_disambiguation, deezer_fan_count_capture, musicbrainz_country_fallback [EXTRACTED 1.00]

## Communities (212 total, 24 thin omitted)

### Community 0 - "New"
Cohesion: 0.06
Nodes (150): fakeReleaseDetailSource, insertEventFailingQuerier, Queries, New(), TestSQLCPing(), TestDetectDeezer_FiltersByRecordType(), TestDetectDeezer_NeverProducesDeluxeChange(), TestDetectDeezer_NewRelease() (+142 more)

### Community 1 - "poller_test.go"
Cohesion: 0.08
Nodes (78): cron.Cron, bytes.Buffer, sync/atomic.Bool, New(), decodeLogRecords(), deezerID(), eightEntries(), failFirstThree() (+70 more)

### Community 2 - "testing.T"
Cohesion: 0.10
Nodes (59): testing.T, errorBody, healthBody, watchlistEntryBody, TestHandleListEvents_CursorRejection(), TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(), TestHandleListEvents_HappyPathReturns200WithEnvelope(), TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray() (+51 more)

### Community 3 - "New"
Cohesion: 0.06
Nodes (79): github.com/jackc/pgx/v5/pgxpool.Pool, sync/atomic.Int32, Querier, insertBaselineNewRelease(), int32Ptr(), readBaselineTrackCount(), TestAdvanceGroupBaseline_ConcurrentRace(), TestAdvanceGroupBaseline_SingleCallerContract() (+71 more)

### Community 4 - "gate_test.go"
Cohesion: 0.10
Nodes (46): fakeAlerter, stubPinger, stubStore, syncBuffer, Token, net/http.Cookie, net/http/httptest.Server, net/http.Response (+38 more)

### Community 5 - "coverage-report/main_test.go"
Cohesion: 0.08
Nodes (55): appendFile(), backendTotalPct(), formatDelta(), frontendLinesPct(), main(), parseBlockLine(), readBackend(), readFrontend() (+47 more)

### Community 6 - "Album"
Cohesion: 0.31
Nodes (5): stubAlbumLister, artistAlbumsResponse, Album, Client, fakeAlbumSource

### Community 7 - "net/http.Request"
Cohesion: 0.22
Nodes (12): net/http.Request, net/http.ResponseWriter, addWatchlistRequest, errorResponse, updateWatchlistRequest, Server, Server, decodeJSONBody() (+4 more)

### Community 8 - "Revisions - 2026-09-04 (post-context grill)"
Cohesion: 0.06
Nodes (30): Amendment 2026-09-05 (post-UAT, G-16-1) - n1-boot inert during the guard-adoption window -> D-04/D-19a and D-11 amended, Canonical References, CI job shape, Claude's Discretion, Deferred Ideas, Destructive-migration detection, Established Patterns, Existing Code Insights (+22 more)

### Community 9 - "newTestClient"
Cohesion: 0.08
Nodes (62): TestLookupArtist_DecodesFixture(), TestLookupArtist_EmptyMBID(), TestLookupArtist_MalformedJSON(), TestLookupArtist_NonOKStatus(), TestLookupArtist_NoRelationsNoAliasesYieldsNonNilZeroLengthSlices(), TestLookupArtist_RequestShape(), TestReleasesForRecording_DecodesFixture(), TestReleasesForRecording_EmptyMBID() (+54 more)

### Community 10 - "codebase/ARCHITECTURE.md"
Cohesion: 0.22
Nodes (13): CLAUDE.md (project instructions), DSN/secret redaction on every error path, .golangci.yml (lint config), deezer.Client (internal/deezer/client.go), Detector (internal/detection/detector.go), discord.Client (internal/discord/client.go), events.Service.List (internal/events/service.go), handleSearch (internal/httpserver/search.go) (+5 more)

### Community 11 - "Phase 8: Frontend Test Suite"
Cohesion: 0.47
Nodes (6): v1.1 Requirements Archive, TEST-01 Requirement, TEST-02 Requirement, v1.1 Roadmap, Phase 8: Frontend Test Suite, Phase 9: CI Coverage Gates

### Community 12 - "NewMatcher"
Cohesion: 0.13
Nodes (37): stubSearcherFunc, NewMatcher(), TestMatch_EmptyNameFailsClosedWithoutOutboundCall(), TestMatch_MatchedCandidateWithEmptyPictureYieldsNilImageURL(), TestMatch_NoCloseNameCandidateFailsClosed(), TestMatch_SearchErrorSurfaces(), TestMatch_SingleCandidateIssuesNoTieBreakFetch(), TestMatch_SingleCloseNameCandidate() (+29 more)

### Community 13 - "migration-check/main_test.go"
Cohesion: 0.14
Nodes (27): main(), run(), runScanCapture(), TestChangedFiles_AddedAndModified(), TestChangedFiles_NothingUnderGlobIsFalseAndEmpty(), TestChangedFiles_WritesGithubOutputFile(), TestModifiedReleasedMigration(), TestPrevReleaseCrossRef_NoPriorTagSkips() (+19 more)

### Community 14 - "authStore.ts"
Cohesion: 0.11
Nodes (8): PassphraseScreen(), handleSubmit(), Input(), createSession(), authStore, gateActive, listeners, reimportStore()

### Community 15 - "cn"
Cohesion: 0.11
Nodes (28): Alert(), AlertAction(), AlertDescription(), AlertTitle(), alertVariants, Avatar(), AvatarBadge(), AvatarFallback() (+20 more)

### Community 16 - "newTestClient"
Cohesion: 0.07
Nodes (56): AlbumLister, APIError, errorProbe, golang.org/x/time/rate.Limiter, net/http.Client, buildArtistAlbumsPageJSON(), pagedArtistAlbumsServer(), TestArtistAlbums_CancellationBetweenPagesAborts() (+48 more)

### Community 17 - ".DetectMusicBrainz"
Cohesion: 0.13
Nodes (24): Detector, newNotifyGate(), nullableString(), deluxeDetectionEnabled(), eventTypeMuted(), releaseTypeAllowed(), TestFilter_DeluxeIsAGateNotAType(), coverArtURLForReleaseGroup() (+16 more)

### Community 18 - "context.Context"
Cohesion: 0.11
Nodes (17): context.Context, log/slog.Logger, noopPinger, stubPinger, stubStore, Queries, TestNormalizeSet(), AddParams (+9 more)

### Community 19 - "migration-check/main.go"
Cohesion: 0.17
Nodes (23): buildReport(), classifyAlterClause(), classifyStatement(), dollarTagAt(), isBlank(), isValidDollarTag(), newFinding(), parseAnnotation() (+15 more)

### Community 20 - "time.Duration"
Cohesion: 0.09
Nodes (49): discordAlerter, noopAlerter, recordingAlerter, syncBuf, golang.org/x/time/rate.Limit, time.Duration, serverConfig, Alerter (+41 more)

### Community 21 - "prevReleaseRefs"
Cohesion: 0.17
Nodes (19): buildPrevReleaseRefs(), classifyBareColumn(), crossReferenceFinding(), expandStar(), extractBlockReferences(), extractParams(), extractReferences(), newPrevReleaseRefs() (+11 more)

### Community 22 - "Phase 15: PR Coverage-Diff Comment - Discussion Log"
Cohesion: 0.08
Nodes (23): Baseline publish trigger, Baseline storage, Body layout, Cache payload, Checkout depth on the comment job, Claude's Discretion, Comment content, Deferred Ideas (+15 more)

### Community 23 - "Recording"
Cohesion: 0.11
Nodes (13): datedRecordingSource, erroringRecordingSource, fakeRecordingSource, noRecordingSource, Client, RecordingRelease, ArtistCreditEntry, Client (+5 more)

### Community 24 - "compilerOptions"
Cohesion: 0.08
Nodes (25): **/*, **/.client/**/*, DOM, DOM.Iterable, ES2022, node, .react-router/types/**/*, **/.server/**/* (+17 more)

### Community 25 - "dependencies"
Cohesion: 0.08
Nodes (25): @base-ui/react, class-variance-authority, @fontsource-variable/inter, isbot, lucide-react, next-themes, react-dom, react-router (+17 more)

### Community 26 - "config_test.go"
Cohesion: 0.18
Nodes (24): Load(), configEnvKeys(), envExampleKeys(), repoRoot(), setDiff(), setRequired(), TestDockerComposeWiresGateEnvVars(), TestDotEnvIsNotTracked() (+16 more)

### Community 27 - "formatEmbed"
Cohesion: 0.09
Nodes (40): allowedMentions, EmbedImage, retry429Body, webhookPayload, Event, Client, Embed, EmbedField (+32 more)

### Community 28 - "Pattern Assignments"
Cohesion: 0.08
Nodes (23): Anchored `grep -vE` / scoped linter carve-out for a CI helper package, CI: SHA-pinned actions + `# vX.Y.Z` comment; `contents: read`; env not interpolation, `.claude/CLAUDE.md` (MODIFY — one-line pointer), `cmd/migrate/main.go` (NEW — `go run` HEAD-schema helper; name at discretion), `cmd/migration-check/main.go` (NEW — stdlib CI guard), `cmd/migration-check/main_test.go` (NEW — `package main` whitebox), Error wrapping (Go), File Classification (+15 more)

### Community 29 - "devDependencies"
Cohesion: 0.09
Nodes (23): jsdom, prettier, prettier-plugin-tailwindcss, tailwindcss, @testing-library/dom, @testing-library/react, @testing-library/user-event, @types/node (+15 more)

### Community 30 - "Backfill"
Cohesion: 0.26
Nodes (15): Stats, Store, Backfill(), matchingCandidate(), TestBackfill_ActivityGate_DelaysThenProceeds(), TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach(), TestBackfill_ContextCancelledPartway_StopsPromptly(), TestBackfill_ListArtistsMissingImageErrors_ReturnsErrNoWrites() (+7 more)

### Community 31 - "PROJECT.md"
Cohesion: 0.19
Nodes (12): Bounded worker-pool concurrent polling, Atomic CAS overlap-guard pattern (poll cycles), FOR UPDATE-locked CTE fix for deluxe-change baseline race, MusicBrainz TLS handshake failure (environmental, WSL2), WINDOWS.md (Broken Windows Ledger), Single Go binary/service architecture, caarlos0/env config parsing, chi router (go-chi/chi/v5) (+4 more)

### Community 32 - "github.com/jackc/pgx/v5/pgtype.Timestamptz"
Cohesion: 0.12
Nodes (13): notifyGate, Event, github.com/jackc/pgx/v5/pgtype.Timestamptz, recordingQuerier, HasOlderEventsParams, InsertEventParams, ListEventsParams, ListEventsRow (+5 more)

### Community 33 - "match.go"
Cohesion: 0.15
Nodes (20): AlbumLister, ArtistDetailLookup, ArtistFetcher, ArtistSearcher, Matcher, Option, ReleaseGroupLister, aliasQueryNames() (+12 more)

### Community 34 - "components.json"
Cohesion: 0.09
Nodes (21): aliases, components, hooks, lib, ui, utils, iconLibrary, menuAccent (+13 more)

### Community 35 - "NewWithWriter"
Cohesion: 0.21
Nodes (12): log/slog.Level, Config, TestNoDSNInLogs(), New(), NewWithWriter(), parseLevel(), TestNewWithWriter_JSONFormatRendersParsableJSON(), TestNewWithWriter_LevelFilteringReachesTheHandler() (+4 more)

### Community 36 - "Pattern Assignments"
Cohesion: 0.07
Nodes (28): `cmd/server/main.go` (MOD), Disabled-case seam (never a nil check in the request path), File Classification, Fixed JSON error body, never raw error text, Functional-option constructor, `internal/authgate/alerter.go` (service seam — the strongest analog in this phase), `internal/authgate/gate.go` (middleware), `internal/authgate/login.go` (login/logout handlers + throttle + global counter) (+20 more)

### Community 37 - "watchlist.tsx"
Cohesion: 0.13
Nodes (21): Button(), buttonVariants, SearchResultRow(), SearchResultRowProps, SearchResultsColumns(), SearchResultsColumnsProps, SOURCE_LABELS, SourceColumn() (+13 more)

### Community 38 - "EventCard.tsx"
Cohesion: 0.15
Nodes (10): CoverArt(), CoverArtProps, EVENT_BADGE, EventCard(), GuestFeatureBody(), guestFeatureHref(), UNKNOWN_EVENT_BADGE, watchlistNote() (+2 more)

### Community 39 - "PoolConfig"
Cohesion: 0.23
Nodes (14): github.com/jackc/pgx/v5/pgxpool.Config, dsnSetsMaxConns(), NewPool(), PoolConfig(), poolMaxConnsForWorkers(), redactedTarget(), blackHoleAddr(), TestPoolConfig_AppliesExplicitBounds() (+6 more)

### Community 40 - "Implementation Decisions"
Cohesion: 0.09
Nodes (22): Baseline publish trigger, Baseline storage, Canonical References, Claude's Discretion, Comment content, Decided by grill review (no longer discretionary), Deferred Ideas, Established Patterns (+14 more)

### Community 41 - "RunMigrations"
Cohesion: 0.08
Nodes (48): main(), run(), highestMigrationVersion(), scratchSchemaDSN(), TestRun_AppliesHeadSchema(), TestRun_MissingDatabaseURL(), retryConfig, RetryOption (+40 more)

### Community 42 - "time.Time"
Cohesion: 0.14
Nodes (11): globalCounter, ipLimiter, loginRequest, loginThrottle, net/http.RoundTripper, sync.Mutex, time.Time, capturingRoundTripper (+3 more)

### Community 43 - "history.tsx"
Cohesion: 0.08
Nodes (24): EmptyState(), EmptyStateProps, EventCardProps, Combobox(), commitSelection(), handleTriggerKeyDown(), openAt(), ComboboxOption (+16 more)

### Community 44 - "Implementation Decisions"
Cohesion: 0.08
Nodes (25): 401 handling in the SPA, Brute-force defense, Canonical References, Claude's Discretion, Client IP resolution, CSRF, Deferred Ideas, Established Patterns (+17 more)

### Community 45 - "Release"
Cohesion: 0.21
Nodes (6): noReleaseDetailSource, Client, Release, Medium, releaseEnvelope, fakeReleaseDetailSource

### Community 46 - "WatchlistEntry"
Cohesion: 0.27
Nodes (10): PreferenceToggles(), toggleMutedEventType(), toggleReleaseType(), PreferenceTogglesProps, entry, mockUpdateWatchlistPreferences, WatchlistRow(), WatchlistRowProps (+2 more)

### Community 47 - "Phase 16: Rollback-Safe Migrations - Discussion Log"
Cohesion: 0.09
Nodes (22): CI job shape, Claude's Discretion, Deferred Ideas, Destructive-migration detection, HEAD-schema setup, Phase 16: Rollback-Safe Migrations - Discussion Log, Q1 — bringing the throwaway Postgres to the current branch's schema, Q1 — do the checks block a merge? (+14 more)

### Community 48 - "Artist"
Cohesion: 0.19
Nodes (8): perArtistOutcome, perArtistSearcher, stubArtistFetcher, stubSearcher, stubSearcherByQuery, artistSearchResponse, stubDeezerArtistSearcher, Artist

### Community 49 - "Artist"
Cohesion: 0.18
Nodes (7): stubStore, Artist, Queries, UpsertArtistParams, Artist, Watchlist, toEntry()

### Community 50 - "Server"
Cohesion: 0.14
Nodes (19): chi.Router, net/http.Handler, Option, Pinger, syncBuffer, echoRequestID(), Server, registerDataRoutes() (+11 more)

### Community 51 - "Pattern Assignments"
Cohesion: 0.10
Nodes (20): CI action pinning, `cmd/coverage-report/main.go` (CLI tool, file-I/O + transform), `cmd/coverage-report/main_test.go` (test, `package main` whitebox), `cmd/coverage-report/testdata/`, Comment discipline, continue-on-error for report-only surfaces, Error wrapping (Go), File Classification (+12 more)

### Community 52 - "Phase 15 Plan 01: cmd/coverage-report Go tool Summary"
Cohesion: 0.12
Nodes (16): Accomplishments, Auto-fixed Issues, CLI surface, Decisions Made, Deviations from Plan, Issues Encountered, Next Phase Readiness, Performance (+8 more)

### Community 53 - "Warnings"
Cohesion: 0.12
Nodes (15): IN-01: `.golangci.yml` comment cites the wrong linter version, IN-02: `run` shadows the imported `io/fs` package, IN-03: `sidecar.GeneratedAt` is decoded but never read, IN-04: `run`'s `stdout` parameter is ignored by comment and sidecar modes, IN-05: `coverage-comment` restores profile blobs it never uses, Info, Phase 15: Code Review Report, Summary (+7 more)

### Community 54 - "Phase 15 Plan 02: Wire the coverage tool into the build Summary"
Cohesion: 0.14
Nodes (13): Accomplishments, Auto-fixed Issues, Cutover measurement (D-17 planner-must-check evidence), Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness (+5 more)

### Community 55 - "Phase 15 Plan 03: Wire the coverage tool into CI Summary"
Cohesion: 0.14
Nodes (13): Accomplishments, Auto-fixed Issues, Decisions Made, Deviations from Plan, Files Created/Modified, Final wiring strings (as actually written), Issues Encountered, Next Phase Readiness (+5 more)

### Community 56 - "Goal Achievement"
Cohesion: 0.14
Nodes (13): Anti-Patterns Found, Behavioral Spot-Checks, Data-Flow Trace (Level 4), Gaps Summary, Goal Achievement, Human Verification Required, Key Link Verification, Observable Truths (+5 more)

### Community 57 - "Phase 14 Plan 04: Instance Passphrase Gate — CSRF, Referrer-Policy & Weak-Passphrase WARN Summary"
Cohesion: 0.11
Nodes (18): 1. [Rule 3 — Blocking] `internal/authgate/login_test.go` + `internal/authgate/gate_test.go` shared helpers updated (beyond frontmatter `files_modified` intent), 2. [Rule 1 — Bug] CSRF counter-isolation test tripped a real brute-force alert, 3. [Rule 3 — Blocking] `.env.example` writable; placeholder changed by operator mid-run, Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered (+10 more)

### Community 58 - "Phase 16: Rollback-Safe Migrations - Research"
Cohesion: 0.14
Nodes (13): Architectural Responsibility Map, Assumptions Log, Don't Hand-Roll, Environment Availability, Metadata, Open Questions, Package Legitimacy Audit, Phase 16: Rollback-Safe Migrations - Research (+5 more)

### Community 59 - "scripts"
Cohesion: 0.18
Nodes (10): name, private, scripts, build, dev, format, test, test:watch (+2 more)

### Community 60 - "Phase 14 Plan 02: Instance Passphrase Gate — Brute-force Defense & Auditability Summary"
Cohesion: 0.12
Nodes (16): 1. [Rule 3 — Blocking] `internal/authgate/gate.go` and `internal/httpserver/server.go` modified (not in frontmatter `files_modified`), 2. [Process] RED/GREEN combined for Task 1, Accomplishments, Authentication Gates, Chosen tunable values (recorded verbatim per plan `<output>`), Decisions Made, Deviations from Plan, Files Created/Modified (+8 more)

### Community 61 - "Phase 14 — UI Design Contract"
Cohesion: 0.12
Nodes (16): Checker Sign-Off, Color, Copywriting Contract, Design System, E1 — `<PassphraseScreen>` (full-screen gate), E2 — Passphrase input, E3 — Unlock submit button, E4 — Inline error slot (+8 more)

### Community 62 - "root.tsx"
Cohesion: 0.14
Nodes (12): Toaster(), deleteSession(), App(), ErrorBoundary(), LogoutButton(), handleLogout(), renderAppAt(), entry (+4 more)

### Community 63 - "Phase 14 Plan 05: Passphrase Gate Config Reachability (G-14-1 Closure) Summary"
Cohesion: 0.12
Nodes (15): Accomplishments, Auth / Checkpoint Gates, Continuation note, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness (+7 more)

### Community 64 - "Phase 16 Plan 03: cmd/migration-check changed-files mode, gitShow seam, and D-15 cross-reference Summary"
Cohesion: 0.15
Nodes (12): Accomplishments, Auto-fixed Issues, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance (+4 more)

### Community 65 - "Phase 14 Plan 01: Instance Passphrase Gate Summary"
Cohesion: 0.13
Nodes (14): 1. [Rule 3 — Blocking] `.env.example` keys added by the operator (not the agent), 2. [Process] RED/GREEN combined per commit, Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness (+6 more)

### Community 66 - "Artist"
Cohesion: 0.31
Nodes (6): stubMusicBrainzArtistSearcher, clampLimit(), escapeLucene(), Artist, Client, artistSearchResponse

### Community 67 - "Phase 14 Plan 03: SPA Instance Passphrase Gate Summary"
Cohesion: 0.13
Nodes (14): Accomplishments, Adjustments within plan latitude (not deviations), Auto-fixed Issues, D-18 (`gateActive`) confirmation — per the plan `<output>`, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered (+6 more)

### Community 68 - "Phase 16 Plan 04: CI Wiring — the changes prelude, migration-check guard, and n1-boot job Summary"
Cohesion: 0.15
Nodes (12): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 16 Plan 04: CI Wiring — the changes prelude, migration-check guard, and n1-boot job Summary (+4 more)

### Community 69 - "Phase 14: Instance Passphrase Gate - Discussion Log"
Cohesion: 0.13
Nodes (14): Boot-time passphrase strength, Brute-force defense beyond per-IP throttle, Claude's Discretion, Deferred Ideas, Follow-up: client IP resolution (behind a Phase 17 reverse proxy), Follow-up: CSRF, Follow-up: how the SPA detects the 401, Follow-up: logout semantics (+6 more)

### Community 70 - "Phase 16 Plan 01: Ahead-of-Source Boot Guard & HEAD-Schema Helper Summary"
Cohesion: 0.17
Nodes (11): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 16 Plan 01: Ahead-of-Source Boot Guard & HEAD-Schema Helper Summary (+3 more)

### Community 71 - "Phase 16 Plan 02: cmd/migration-check Static DDL Guard Summary"
Cohesion: 0.17
Nodes (11): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 16 Plan 02: cmd/migration-check Static DDL Guard Summary (+3 more)

### Community 72 - "Goal Achievement"
Cohesion: 0.13
Nodes (14): Anti-Patterns Found, Behavioral Spot-Checks, Data-Flow Trace (Level 4), Gaps Summary, Goal Achievement, Human Verification Required, Key Link Verification, Observable Truths (+6 more)

### Community 73 - "Phase 14 Plan 07: Close G-14-3 — Self-Identifying Gated Responses Summary"
Cohesion: 0.14
Nodes (13): Accomplishments, Auto-fixed Issues, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Output confirmations (per plan `<output>`) (+5 more)

### Community 74 - "Milestone v1.1 Audit Report"
Cohesion: 0.67
Nodes (3): CoverArt.tsx Stale Image-Load-Error State Bug, v1.1 Milestone Nyquist Compliance (All 5 Phases), Milestone v1.1 Audit Report

### Community 76 - "Phase 16 Plan 05: internal/db/migrations/README.md — the expand/contract rule as a standing constraint Summary"
Cohesion: 0.17
Nodes (11): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 16 Plan 05: internal/db/migrations/README.md — the expand/contract rule as a standing constraint Summary (+3 more)

### Community 78 - "api.ts"
Cohesion: 0.17
Nodes (15): SearchBox(), handleChange(), runSearch(), SearchBoxProps, mockSearchArtists, searchResponse, addWatchlist(), ApiError (+7 more)

### Community 80 - "Goal Achievement"
Cohesion: 0.17
Nodes (11): 1. Live scratch-branch CI run — skip-propagation, registry state, and n1-boot behavior, Anti-Patterns Found, Behavioral Spot-Checks, Gaps Summary, Goal Achievement, Human Verification Required, Key Link Verification, Observable Truths (Roadmap Success Criteria) (+3 more)

### Community 81 - "runChangedFiles"
Cohesion: 0.27
Nodes (10): appendGithubOutput(), diffRange(), filterMigrationUpFiles(), runChangedFiles(), TestDiffRange(), TestFilterMigrationUpFiles_KeepsOnlyTheGlob(), TestValidCommitishAndValidBranchRef(), withStubCommitExists() (+2 more)

### Community 82 - "runCapture"
Cohesion: 0.42
Nodes (11): readPrevReleaseFixture(), runCapture(), stubQueriesGitShow(), TestPrevReleaseCrossRef_AnnotationCannotOverride(), TestPrevReleaseCrossRef_DropTableIsRed(), TestPrevReleaseCrossRef_LowConfidenceIsNotRed(), TestPrevReleaseCrossRef_NoReferenceStillPlainFindingSuppressedByAnnotation(), TestPrevReleaseCrossRef_RenameColumnIsRed() (+3 more)

### Community 83 - "Phase 15: PR Coverage-Diff Comment - Research"
Cohesion: 0.18
Nodes (10): Architectural Responsibility Map, Assumptions Log, Don't Hand-Roll, Metadata, Open Questions, Package Legitimacy Audit, Phase 15: PR Coverage-Diff Comment - Research, Phase Requirements (+2 more)

### Community 84 - "Phase 16 — Security"
Cohesion: 0.18
Nodes (10): Accepted Risks Log, Phase 16 — Security, Residual Notes on Otherwise-Closed Threats, Security Audit Trail, Sign-Off, Threat Register, Trust Boundaries, UF-1 — `n1-boot` guard-adoption skip-green step (post-plan, closing UAT gap G-16-1) (+2 more)

### Community 85 - "watchlist.sql.go"
Cohesion: 0.27
Nodes (5): Queries, CreateWatchlistEntryParams, ListWatchlistRow, UpdateWatchlistPreferencesParams, UpdateWatchlistPreferencesRow

### Community 86 - "Common Pitfalls"
Cohesion: 0.20
Nodes (10): Common Pitfalls, Pitfall 1: Detecting "no baseline" via `cache-hit` (D-20), Pitfall 2: `download-artifact` hard-fails when the profile is missing (D-18), Pitfall 3: `pull-requests: write` at workflow scope (Pitfall 24), Pitfall 4: Fork PR turns red over a reporting feature (Pitfall 25), Pitfall 5: `cache/save` reddening a green `main` (D-04), Pitfall 6: Saving a sub-threshold `main` as the baseline (D-14), Pitfall 7: D-17 rounding cutover moves the enforced number (D-17) (+2 more)

### Community 89 - "Phase 14: Instance Passphrase Gate - Research"
Cohesion: 0.15
Nodes (12): Architectural Responsibility Map, Assumptions Log, Don't Hand-Roll, Environment Availability, Metadata, Open Questions, Package Legitimacy Audit, Phase 14: Instance Passphrase Gate - Research (+4 more)

### Community 90 - "Requirements: drop-tracker"
Cohesion: 0.15
Nodes (12): Access Gate, Access Gate, CI/CD Pipeline, CI/CD Pipeline, Deployment, Deployment / Operations, Future Requirements, Migration Safety (+4 more)

### Community 91 - "Tests"
Cohesion: 0.20
Nodes (9): 1. Same-repo PR produces exactly one coverage comment with both rows and deltas (SC #1), 2. Three pushes leave one comment, edited in place (SC #2), 3. No-baseline CI run posts absolute numbers + delta-unavailable footer, job green (SC #3, CI path), 4. Coverage drop shows in the comment with a warning glyph; PR stays mergeable (SC #4, merge-button confirmation), 5. Merge publishes the baseline; next PR diffs against it; no PR job recomputes main's coverage (SC #5), Current Test, Gaps, Summary (+1 more)

### Community 92 - "Common Pitfalls"
Cohesion: 0.20
Nodes (10): Common Pitfalls, Existing milestone pitfalls (PITFALLS.md 8, 9, 10), Pitfall A: assuming `migrate.Up()` no-ops on an ahead-of-source schema, Pitfall B: job-level `if:` on a job in `build-scan.needs:`, Pitfall C: scanning `*.down.sql`, Pitfall D: `services: postgres` starting on every push, Pitfall E: D-15 false-red with no override, Pitfall F: `dirty` schema_migrations masquerading as "ahead" (+2 more)

### Community 93 - "NewMusicBrainzSource"
Cohesion: 0.36
Nodes (6): SearchSource, stubSearchSource, NewMusicBrainzSource(), TestNewDeezerSource_MapsFields(), TestNewMusicBrainzSource_MapsFields(), TestNewMusicBrainzSource_PreservesOrder()

### Community 94 - "Mechanism Verification (the core of this research)"
Cohesion: 0.22
Nodes (9): Mechanism 1 — `actions/cache` save/restore, Mechanism 2 — `marocchino/sticky-pull-request-comment` v3.0.5, Mechanism 3 — Go `coverage.out` profile format, Mechanism 4 — Vitest `json-summary` reporter, Mechanism 5 — GitHub Actions artifact hand-off, Mechanism 6 — `if:` / `needs:` / `permissions` / `concurrency` semantics, Mechanism 7 — `gosec` G304 carve-out in golangci-lint v2 (D-19), Mechanism 8 — `make coverage-report` / gate refactor (D-17) (+1 more)

### Community 95 - "Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary"
Cohesion: 0.17
Nodes (11): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary (+3 more)

### Community 96 - "Phase 15 — Validation Strategy"
Cohesion: 0.22
Nodes (8): Manual-Only Verifications, Per-Task Verification Map, Phase 15 — Validation Strategy, Sampling Rate, Test Infrastructure, Validation Audit 2026-09-03, Validation Sign-Off, Wave 0 Requirements

### Community 97 - "Phase 16 — Validation Strategy"
Cohesion: 0.22
Nodes (8): Manual-Only Verifications, Per-Task Verification Map, Phase 16 — Validation Strategy, Sampling Rate, Test Infrastructure, Validation Audit 2026-09-04, Validation Sign-Off, Wave 0 Requirements

### Community 98 - "Info"
Cohesion: 0.18
Nodes (10): IN-01: Structural "ungated instance" guarantee is asserted only in prose + tests, IN-02: Latch silently no-ops if the API is ever served cross-origin, IN-03: Test matrix gaps on the marker's negative space, IN-04: `apiFetch` reads a header on every response for a session-lifetime one-shot, Info, Phase 14: Code Review Report, Summary, Warnings (+2 more)

### Community 99 - "Phase 16: Code Review Report"
Cohesion: 0.25
Nodes (7): CR-01: Schema-qualified table names bypass the D-15 non-overridable cross-reference check, Critical Issues, Phase 16: Code Review Report, Post-Review Outcome, Summary, Warnings, WR-01: `rename_column` findings encode two identifiers into one display string that is later re-parsed

### Community 100 - "Migrations: rollback-safety rules"
Cohesion: 0.29
Nodes (6): Before you merge a migration, Expand, backfill, contract: a walkthrough already in this tree, Migrations: rollback-safety rules, The N-1 invariant, The two rules, What `cmd/migration-check` enforces

### Community 101 - "Repo Reconnaissance — exact anchors for the 20 decisions"
Cohesion: 0.29
Nodes (7): `cmd/` layout, `.github/workflows/full-pipeline.yml`, `.gitignore`, `.golangci.yml`, `Makefile`, Repo Reconnaissance — exact anchors for the 20 decisions, `web/vitest.config.ts`

### Community 102 - "Phase 15 — Security"
Cohesion: 0.29
Nodes (6): Accepted Risks Log, Phase 15 — Security, Security Audit Trail, Sign-Off, Threat Register, Trust Boundaries

### Community 103 - "16-UAT.md"
Cohesion: 0.29
Nodes (6): 1. Live scratch-branch CI run — destructive migration, docs-only push, additive migration, Current Test, Gaps, Notes (not gaps), Summary, Tests

### Community 104 - "withStubGitShow"
Cohesion: 0.40
Nodes (6): pathAllowedForGitShow(), readAtTag(), TestGitShow_RejectsMalformedTag(), TestGitShow_RejectsPathOutsideAllowlist(), TestPrevReleaseCrossRef_GitShowFailureIsRed(), withStubGitShow()

### Community 108 - "Validation Architecture"
Cohesion: 0.33
Nodes (6): Phase Requirements → Test Map, Sampling Rate, Test Framework, Validation Architecture, Wave 0 Gaps, Workflow wiring — what is verifiable how

### Community 109 - "Tests"
Cohesion: 0.20
Nodes (9): 1. Real-browser cookie behaviour (Chrome + Firefox, http://localhost), 2. PassphraseScreen visual conformance to 14-UI-SPEC, 3. docker compose up with no INSTANCE_PASSPHRASE configured, 4. Live Discord brute-force alert, 5. Log out control persists across a page reload while logged in, Current Test, Gaps, Summary (+1 more)

### Community 110 - "ArtistDetail"
Cohesion: 0.33
Nodes (6): stubArtistDetailLookup, ArtistDetail, Client, ArtistAlias, ArtistRelation, ArtistRelationURL

### Community 111 - "Architecture Patterns"
Cohesion: 0.33
Nodes (6): Anti-Patterns to Avoid, Architecture Patterns, Pattern 1: the D-18 source seam, Pattern 2: `cmd/migration-check` structure (mirror `cmd/coverage-report`), Pattern 3: `internal/db` real-Postgres test (mirror `migrate_test.go`), System flow — the N-1 boot check

### Community 112 - "Roadmap: drop-tracker"
Cohesion: 0.22
Nodes (8): Milestone Summary, Overview, Phase 12: Cleanup: CoverArt Reset & Search Popularity Ranking, Phase 13: Fix History Dates, Guest-Feature Art & Artist Art, Phase Details, Phases, Progress, Roadmap: drop-tracker

### Community 113 - "Architecture Patterns"
Cohesion: 0.22
Nodes (9): Anti-Patterns to Avoid, Architecture Patterns, Pattern 1: Conditional wiring via a trailing functional option (GATE-07), Pattern 2: Stateless HMAC session token (D-01, D-02, D-06, D-08), Pattern 3: Cookie construction (D-09 — with the A1 adjustment), Pattern 4: `rate.Limiter` per IP with bounded memory (GATE-04, D-12), Pattern 5: SPA shared auth state without a provider (D-16, GATE-05), Recommended Project Structure (+1 more)

### Community 114 - "Common Pitfalls"
Cohesion: 0.22
Nodes (9): Common Pitfalls, Pitfall 1: `__Host-` cookie prefix silently dropped by Chrome on `http://localhost`, Pitfall 2: `/health` swept into the gate → deploy health-poll gets `401`, Pitfall 3: Length leak in the constant-time compare, Pitfall 4: Session not rotated on login (fixation), Pitfall 5: Absolute cap not enforced because `IssuedAt` moves on renewal, Pitfall 6: Passphrase / session leaking into `httplog` output, Pitfall 7: The "inert" path isn't actually inert (+1 more)

### Community 115 - "Phase 14 — Validation Strategy"
Cohesion: 0.22
Nodes (8): Manual-Only Verifications, Per-Task Verification Map, Phase 14 — Validation Strategy, Sampling Rate, Test Infrastructure, Validation Audit 2026-09-01, Validation Sign-Off, Wave 0 Requirements

### Community 116 - "GitHub Actions Wiring (D-14, D-16, D-13, D-04)"
Cohesion: 0.33
Nodes (6): `build-scan.needs:` (D-11), `changes` prelude job (D-16), Conventions to follow, Fetch-depth / tags, GitHub Actions Wiring (D-14, D-16, D-13, D-04), `n1-boot` job (D-13 / D-14 / D-04)

### Community 117 - "Finding 1 (CRITICAL): `migrate.Up()` against an ahead-of-source schema ERRORS — it does not `ErrNoChange`"
Cohesion: 0.33
Nodes (6): Falsification test (executed this session), Finding 1 (CRITICAL): `migrate.Up()` against an ahead-of-source schema ERRORS — it does not `ErrNoChange`, Impact if left unaddressed, `iofs` over a truncated / synthetic source — CONFIRMED, Mechanism (read from the pinned module source), Recommended fix (prototype verified this session)

### Community 118 - "stripComments"
Cohesion: 0.50
Nodes (5): copyDollarQuoted(), copySingleQuoted(), stripComments(), TestStripComments(), strings.Builder

### Community 119 - "Code Examples"
Cohesion: 0.40
Nodes (5): `actions/cache/save` on push→main (D-13 / D-04), Code Examples, `coverage.out` hand-parse (D-06) — reference shape, marocchino step (D-05 / D-08), Round half-up to 2 dp (D-06 / D-12)

### Community 120 - "Issue tracker: GitHub"
Cohesion: 0.29
Nodes (6): Conventions, Issue tracker: GitHub, Pull requests as a triage surface, Wayfinding operations, When a skill says "fetch the relevant ticket", When a skill says "publish to the issue tracker"

### Community 122 - "D-15: extracting the previous release's referenced `(table, column)` set from `queries/*.sql`"
Cohesion: 0.40
Nodes (5): Blind spots (enumerate — these are the boot job's responsibility, not the guard's), Conservatism tuning (false-red vs. false-green), D-15: extracting the previous release's referenced `(table, column)` set from `queries/*.sql`, Recommended extraction approach (stdlib Go), What the previous-release queries actually look like

### Community 123 - "D-08: static destructive-DDL detection in `*.up.sql`"
Cohesion: 0.40
Nodes (5): D-08: static destructive-DDL detection in `*.up.sql`, Hard — flag broadly, require the annotation (blind spots), Reliably detectable (regex / token scan), Scanner mechanics (stdlib, hand-rolled — the on-brand choice), Two finding classes (S4) — message content

### Community 124 - "Validation Architecture"
Cohesion: 0.40
Nodes (5): Phase Requirements → Test Map, Sampling Rate, Test Framework, Validation Architecture, Wave 0 Gaps

### Community 125 - "findFromJoinTables"
Cohesion: 0.50
Nodes (4): findFromJoinTables(), nextToken(), TestFindFromJoinTables_AdjacentFromJoinBothFound(), fromJoinTable

### Community 126 - "15-01-PLAN.md"
Cohesion: 0.50
Nodes (3): Artifacts this phase produces (plan 15-01 share), STRIDE Threat Register, Trust Boundaries

### Community 127 - "15-02-PLAN.md"
Cohesion: 0.50
Nodes (3): Artifacts this phase produces (plan 15-02 share), STRIDE Threat Register, Trust Boundaries

### Community 128 - "15-03-PLAN.md"
Cohesion: 0.50
Nodes (3): Artifacts this phase produces (plan 15-03 share), STRIDE Threat Register, Trust Boundaries

### Community 129 - "Phase 14 — Security"
Cohesion: 0.29
Nodes (6): Accepted Risks Log, Phase 14 — Security, Security Audit Trail, Sign-Off, Threat Register, Trust Boundaries

### Community 130 - "Domain Docs"
Cohesion: 0.33
Nodes (5): Before exploring, read these, Domain Docs, File structure, Flag ADR conflicts, Use the glossary's vocabulary

### Community 131 - "14-07-PLAN.md"
Cohesion: 0.33
Nodes (5): Artifacts this plan produces, Deliberate scoping decisions, Source coverage audit, STRIDE Threat Register, Trust Boundaries

### Community 132 - "Code Examples"
Cohesion: 0.33
Nodes (6): `Alerter` seam + disabled-case idiom (mirrors `notifier.Select`), Boot-time weak WARN (main.go, right after `config.Load()`), Code Examples, Config field (follows the grouped-by-phase convention), Go table-driven session test (matches the repo's individual-named-test lean, with `t.Run` subcases), RTL test for the 401 → passphrase → re-fetch flow

### Community 137 - "Standard Stack"
Cohesion: 0.50
Nodes (4): Actions / tools this phase adds or touches, Alternatives Considered (all rejected by CONTEXT — do not revisit), No new dependencies, Standard Stack

### Community 138 - "User Constraints (from 15-CONTEXT.md)"
Cohesion: 0.50
Nodes (4): Claude's Discretion, Deferred Ideas (OUT OF SCOPE), Locked Decisions (condensed — full text in 15-CONTEXT.md), User Constraints (from 15-CONTEXT.md)

### Community 139 - "14-05-PLAN.md"
Cohesion: 0.40
Nodes (4): Compose semantics — already verified empirically, do not re-derive, Constraints, STRIDE Threat Register, Trust Boundaries

### Community 140 - "14-06-PLAN.md"
Cohesion: 0.40
Nodes (4): Artifacts this phase produces, Deliberate scoping decisions, STRIDE Threat Register, Trust Boundaries

### Community 141 - "Validation Architecture"
Cohesion: 0.40
Nodes (5): Phase Requirements → Test Map, Sampling Rate, Test Framework, Validation Architecture, Wave 0 Gaps

### Community 142 - ".pre-commit-config.yaml"
Cohesion: 0.50
Nodes (3): Local Gitleaks Pre-commit Secret Scanning Hook, Local golangci-lint Pre-commit Hook (Changed-Files Only), drop-tracker README

### Community 143 - "Sources"
Cohesion: 0.50
Nodes (4): Primary (HIGH confidence), Secondary (MEDIUM confidence), Sources, Tertiary (LOW confidence)

### Community 144 - "Code Examples"
Cohesion: 0.50
Nodes (4): Ahead-of-source guard (verified prototype — see Finding 1), `changes` diff-base (see GitHub Actions Wiring for the full step), Code Examples, D-02 hermetic test skeleton

### Community 145 - "User Constraints (from CONTEXT.md)"
Cohesion: 0.50
Nodes (4): Claude's Discretion, Deferred Ideas (OUT OF SCOPE), Locked Decisions, User Constraints (from CONTEXT.md)

### Community 146 - "Sources"
Cohesion: 0.50
Nodes (4): Primary (HIGH confidence), Secondary (MEDIUM confidence), Sources, Tertiary (LOW confidence)

### Community 148 - "Standard Stack"
Cohesion: 0.50
Nodes (4): Alternatives Considered, Core (all already present — nothing to install), Frontend (all already present), Standard Stack

### Community 149 - "User Constraints (from CONTEXT.md)"
Cohesion: 0.50
Nodes (4): Claude's Discretion, Deferred Ideas (OUT OF SCOPE), Locked Decisions, User Constraints (from CONTEXT.md)

### Community 150 - "Sources"
Cohesion: 0.50
Nodes (4): Primary (HIGH confidence), Secondary (MEDIUM confidence), Sources, Tertiary (LOW confidence — flagged as A1)

### Community 151 - "Phase 14 — External API Coverage"
Cohesion: 0.50
Nodes (3): Everything else in this phase, Phase 14 — External API Coverage, Re-decision on the D-12 brute-force alert (required by the api-coverage capability)

### Community 152 - "Out-of-scope discoveries (not fixed)"
Cohesion: 0.50
Nodes (3): Out-of-scope discoveries (not fixed), Phase 14 — Deferred Items, `tsc --noEmit` fails on a stale react-router typegen artifact

### Community 153 - "Security Domain"
Cohesion: 0.67
Nodes (3): Applicable ASVS categories, Security Domain, Threat patterns for this stack

### Community 162 - "Security Domain"
Cohesion: 0.67
Nodes (3): Applicable ASVS Categories, Known Threat Patterns for {Go/chi API + embedded React SPA + shared passphrase}, Security Domain

### Community 170 - "Standard Stack"
Cohesion: 0.67
Nodes (3): Alternatives Considered, Core (no new dependencies), Standard Stack

### Community 171 - "Security Domain"
Cohesion: 0.67
Nodes (3): Applicable ASVS Categories, Known Threat Patterns for this phase, Security Domain

### Community 177 - "NewActivityGate"
Cohesion: 0.27
Nodes (7): ActivityGate, NewActivityGate(), TestActivityGate_ActiveWhileBegunNotEnded(), TestActivityGate_ConcurrentUse(), TestActivityGate_DoubleEndDoesNotCorruptState(), TestActivityGate_FreshGateIsNotActive(), TestActivityGate_TwoConcurrentBeginsBothMustEnd()

### Community 178 - "Finding 2 (CRITICAL): a skipped `needs:` job SKIPS its dependents — the D-12/S6 "counts as success" premise is wrong"
Cohesion: 0.67
Nodes (3): Finding 2 (CRITICAL): a skipped `needs:` job SKIPS its dependents — the D-12/S6 "counts as success" premise is wrong, Impact if implemented as D-12 describes, Recommended design

### Community 179 - "ReleaseGroup"
Cohesion: 0.31
Nodes (5): stubGroupLister, Client, ReleaseGroup, releaseGroupEnvelope, fakeReleaseGroupSource

### Community 180 - "cancelingSearcher"
Cohesion: 0.22
Nodes (6): cancelingSearcher, recordingTimeSearcher, ArtistSearcher, context.CancelFunc, io.ReadCloser, cancelReadCloser

### Community 181 - ".HandleLogin"
Cohesion: 0.22
Nodes (8): sync.Once, time.Ticker, Manager, hasCSRFHeader(), setSessionCookie(), writeJSONError(), clientIP(), Manager

### Community 182 - "httpserver/search.go"
Cohesion: 0.21
Nodes (10): deezerSource, musicBrainzSource, searchResponse, searchResponseBody, sourceResult, ArtistSearcher, SearchArtist, NewDeezerSource() (+2 more)

### Community 183 - "run"
Cohesion: 0.24
Nodes (10): logInstanceGateStatus(), main(), run(), decodeRecord(), nonEmptyLines(), recordMentions(), TestLogInstanceGateStatus(), TestRun_BootServesHealthThenGracefulShutdownOnCancel() (+2 more)

### Community 184 - "events/service.go"
Cohesion: 0.24
Nodes (9): stubEventsStore, Service, eventsResponse, eventsResponseBody, stubEventsStore, Event, ListParams, Page (+1 more)

### Community 185 - "EncodeCursor"
Cohesion: 0.36
Nodes (9): Cursor, cursorWireForm, DecodeCursor(), EncodeCursor(), TestDecodeCursor_NeverPanics(), TestDecodeCursor_RejectsInvalidInput(), TestEncodeCursor_ProducesURLSafeToken(), TestEncodeDecodeCursor_RoundTripsWithNilReleaseDate() (+1 more)

### Community 187 - "ROADMAP.md"
Cohesion: 0.31
Nodes (6): CoverArt.tsx image-load-error state never resets on src change, Deezer fan-count capture and popularity sort, MusicBrainz country fallback for disambiguation, Phase 12: CoverArt Reset & Search Popularity Ranking, Search-result popularity ranking / same-name artist disambiguation, Soft-delete/filter event retention (not hard delete)

### Community 188 - "detector.go"
Cohesion: 0.31
Nodes (6): Option, RecordingSource, ReleaseDetailSource, Detector, onOrAfterCutoff(), WithNotifyMaxReleaseAgeDays()

### Community 189 - "Handler"
Cohesion: 0.33
Nodes (7): Handler(), firstJSAsset(), TestHandler_MissingAssetPathFallsBackInsteadOf404(), TestHandler_RootPathServesIndexHTML(), TestHandler_ServesRealEmbeddedAssetDirectly(), TestHandler_UnknownPathFallsBackToIndexHTML(), React (Vite) SPA embedded via go:embed

### Community 190 - "full-pipeline.yml (GitHub Actions workflow)"
Cohesion: 0.33
Nodes (6): CI coverage gates (80% backend / 70% frontend), full-pipeline.yml (GitHub Actions workflow), gitleaks secret scanning, svu semantic version calculation, syft/anchore SBOM generation, Vitest + React Testing Library frontend test suite

### Community 191 - "IsWeakPassphrase"
Cohesion: 0.47
Nodes (4): IsWeakPassphrase(), TestIsWeakPassphrase(), TestWeakPassphraseBootWarn_EmptyOrStrongLogsNothing(), TestWeakPassphraseBootWarn_OneWarnLineNeverContainsValue()

### Community 192 - "redactError"
Cohesion: 0.60
Nodes (5): redactError(), TestRedactError_KeepsDiagnosticContext(), TestRedactError_LeavesNonDSNTextAlone(), TestRedactError_NeverEchoesPassword(), wrapAsDriverError()

### Community 193 - ".handleListEvents"
Cohesion: 0.50
Nodes (3): Server, parseOptionalPageSize(), parseOptionalPositiveInt64()

### Community 194 - "artists"
Cohesion: 0.50
Nodes (3): artists, watchlist, events

## Knowledge Gaps
- **850 isolated node(s):** `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest`, `Queries`, `Client` (+845 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 1001 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **24 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `run` to `New`, `poller_test.go`, `testing.T`, `New`, `NewMatcher`, `newTestClient`, `context.Context`, `time.Duration`, `config_test.go`, `Backfill`, `match.go`, `NewWithWriter`, `PoolConfig`, `RunMigrations`, `NewActivityGate`, `Server`, `httpserver/search.go`, `detector.go`, `IsWeakPassphrase`, `NewMusicBrainzSource`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Why does `New()` connect `testing.T` to `New`, `poller_test.go`, `NewWithWriter`, `gate_test.go`, `codebase/ARCHITECTURE.md`, `Server`, `context.Context`, `time.Duration`, `Handler`, `run`, `events/service.go`, `NewMusicBrainzSource`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Why does `callNotifyPending()` connect `New` to `context.Context`, `testing.T`, `time.Duration`?**
  _High betweenness centrality (0.011) - this node is a cross-community bridge._
- **What connects `vitestSummary`, `github.com/danielrpof/drop-tracker`, `loginRequest` to the rest of the system?**
  _850 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `New` be split into smaller, more focused modules?**
  _Cohesion score 0.06030225946431243 - nodes in this community are weakly interconnected._
- **Should `poller_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08372093023255814 - nodes in this community are weakly interconnected._
- **Should `testing.T` be split into smaller, more focused modules?**
  _Cohesion score 0.10496671786994367 - nodes in this community are weakly interconnected._