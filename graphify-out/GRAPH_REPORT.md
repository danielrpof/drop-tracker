# Graph Report - drop-tracker  (2026-09-01)

## Corpus Check
- 225 files · ~314,801 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 2088 nodes · 5504 edges · 137 communities (91 shown, 19 thin omitted)
- Extraction: 90% EXTRACTED · 10% INFERRED · 0% AMBIGUOUS · INFERRED: 568 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `9ee4aa61`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- New
- poller_test.go
- New
- New
- gate_test.go
- TestDetectGuestFeatures_NonSeedCycle_FreshFeatureStillDelivered
- Album
- net/http.Request
- releases_test.go
- newTestClient
- golang.org/x/time/rate.Limiter
- Phase 8: Frontend Test Suite
- NewMatcher
- .HandleLogin
- authStore.ts
- cn
- testing.T
- log/slog.Logger
- context.Context
- sync.Mutex
- time.Duration
- events_test.go
- EncodeCursor
- Recording
- compilerOptions
- dependencies
- config_test.go
- formatEmbed
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
- run
- RunMigrations
- time.Time
- history.tsx
- Implementation Decisions
- Release
- WatchlistEntry
- Artist
- Artist
- Server
- Phase 14 Plan 04: Instance Passphrase Gate — CSRF, Referrer-Policy & Weak-Passphrase WARN Summary
- scripts
- Phase 14 Plan 02: Instance Passphrase Gate — Brute-force Defense & Auditability Summary
- Phase 14 — UI Design Contract
- root.tsx
- Phase 14 Plan 05: Passphrase Gate Config Reachability (G-14-1 Closure) Summary
- Phase 14 Plan 01: Instance Passphrase Gate Summary
- Artist
- Phase 14 Plan 03: SPA Instance Passphrase Gate Summary
- Phase 14: Instance Passphrase Gate - Discussion Log
- Goal Achievement
- Phase 14 Plan 07: Close G-14-3 — Self-Identifying Gated Responses Summary
- Milestone v1.1 Audit Report
- v1.0 Milestone Audit Report
- clsx
- api.ts
- healthResponse
- typescript
- web/pnpm-workspace.yaml
- Phase 14: Instance Passphrase Gate - Research
- Requirements: drop-tracker
- Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary
- events/service.go
- Info
- Broken Windows Ledger cross-phase defect register
- Tests
- ArtistDetail
- Roadmap: drop-tracker
- Architecture Patterns
- Common Pitfalls
- Phase 14 — Validation Strategy
- Issue tracker: GitHub
- github.com/danielrpof/drop-tracker
- Phase 14 — Security
- Domain Docs
- 14-07-PLAN.md
- Code Examples
- 14-05-PLAN.md
- 14-06-PLAN.md
- Validation Architecture
- .pre-commit-config.yaml
- Standard Stack
- User Constraints (from CONTEXT.md)
- Sources
- Phase 14 — External API Coverage
- Out-of-scope discoveries (not fixed)
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
- vitest
- NewActivityGate
- .fetchReleaseGroupPage
- cancelingSearcher
- artists

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
- `Graceful shutdown via signal.NotifyContext` --references--> `run()`  [EXTRACTED]
  .planning/PROJECT.md → cmd/server/main.go
- `caarlos0/env config parsing` --references--> `Load()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/config/config.go
- `handleHealth (internal/httpserver/health.go)` --shares_data_with--> `NewPool()`  [INFERRED]
  .planning/codebase/ARCHITECTURE.md → internal/db/pool.go
- `golang-migrate schema migrations` --references--> `RunMigrations()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/db/migrate.go
- `pgx/v5 Postgres driver` --references--> `NewPool()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/db/pool.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Full Pipeline CI/CD job set (lint, test, scan, sbom, release)** — github_workflows_full_pipeline_doc, tech_golangci_lint_v2, tech_trivy, tech_gitleaks, tech_svu, tech_syft_sbom [EXTRACTED 1.00]
- **Phase 12 scope: CoverArt reset plus search popularity/disambiguation work** — phase12_cleanup, coverart_reset_bug, search_popularity_disambiguation, deezer_fan_count_capture, musicbrainz_country_fallback [EXTRACTED 1.00]

## Communities (137 total, 19 thin omitted)

### Community 0 - "New"
Cohesion: 0.07
Nodes (125): fakeReleaseDetailSource, insertEventFailingQuerier, Queries, New(), Querier, TestSQLCPing(), TestDetectDeezer_FiltersByRecordType(), TestDetectDeezer_NeverProducesDeluxeChange() (+117 more)

### Community 1 - "poller_test.go"
Cohesion: 0.08
Nodes (78): cron.Cron, bytes.Buffer, sync/atomic.Bool, New(), decodeLogRecords(), deezerID(), eightEntries(), failFirstThree() (+70 more)

### Community 2 - "New"
Cohesion: 0.08
Nodes (59): errorBody, healthBody, watchlistEntryBody, TestHandleListEvents_CursorRejection(), TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(), TestHandleListEvents_HappyPathReturns200WithEnvelope(), TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray(), TestHandleListEvents_StoreErrorReturns500WithFixedMessage() (+51 more)

### Community 3 - "New"
Cohesion: 0.09
Nodes (53): github.com/jackc/pgx/v5/pgxpool.Pool, NewClient(), TestSend_429RetryAfterClamped(), TestSend_429ThenSuccess_HonorsRetryAfter(), TestSend_429Twice_ReturnsErrorAfterSingleRetry(), TestSend_AllowedMentionsAlwaysSuppressed(), TestSend_Success204(), TestSend_TransportFailure_ErrorNeverLeaksHostOrToken() (+45 more)

### Community 4 - "gate_test.go"
Cohesion: 0.12
Nodes (42): fakeAlerter, stubPinger, Token, net/http.Cookie, net/http.Response, deleteSession(), discardLogger(), login() (+34 more)

### Community 5 - "TestDetectGuestFeatures_NonSeedCycle_FreshFeatureStillDelivered"
Cohesion: 0.20
Nodes (21): insertBaselineNewRelease(), int32Ptr(), readBaselineTrackCount(), TestAdvanceGroupBaseline_ConcurrentRace(), TestAdvanceGroupBaseline_SingleCallerContract(), newNotifyGate(), filterTestLogger(), filterTestMBID() (+13 more)

### Community 6 - "Album"
Cohesion: 0.11
Nodes (14): stubAlbumLister, AlbumLister, APIError, artistAlbumsResponse, errorProbe, Album, Client, Artist (+6 more)

### Community 7 - "net/http.Request"
Cohesion: 0.17
Nodes (15): net/http.Request, net/http.ResponseWriter, addWatchlistRequest, errorResponse, updateWatchlistRequest, Server, parseOptionalPageSize(), parseOptionalPositiveInt64() (+7 more)

### Community 8 - "releases_test.go"
Cohesion: 0.12
Nodes (19): net/http/httptest.Server, buildRecordingPageJSON(), pagedRecordingServer(), TestRecordingsByArtist_DecodesEnvelope(), TestRecordingsByArtist_EmptyMBID(), TestRecordingsByArtist_NonOKStatus(), TestRecordingsByArtist_Paginates(), TestRecordingsByArtist_RequestShape() (+11 more)

### Community 9 - "newTestClient"
Cohesion: 0.11
Nodes (44): TestLookupArtist_DecodesFixture(), TestLookupArtist_EmptyMBID(), TestLookupArtist_MalformedJSON(), TestLookupArtist_NonOKStatus(), TestLookupArtist_NoRelationsNoAliasesYieldsNonNilZeroLengthSlices(), TestLookupArtist_RequestShape(), TestReleasesForRecording_DecodesFixture(), TestReleasesForRecording_EmptyMBID() (+36 more)

### Community 10 - "golang.org/x/time/rate.Limiter"
Cohesion: 0.22
Nodes (14): golang.org/x/time/rate.Limiter, net/http.Client, Client, NewClient(), Do(), TestDo_CloseCancelsTheDerivedContext(), TestDo_LimiterWaitErrorOnCancelledContext(), TestDo_Success() (+6 more)

### Community 11 - "Phase 8: Frontend Test Suite"
Cohesion: 0.47
Nodes (6): v1.1 Requirements Archive, TEST-01 Requirement, TEST-02 Requirement, v1.1 Roadmap, Phase 8: Frontend Test Suite, Phase 9: CI Coverage Gates

### Community 12 - "NewMatcher"
Cohesion: 0.13
Nodes (37): stubSearcherFunc, NewMatcher(), TestMatch_EmptyNameFailsClosedWithoutOutboundCall(), TestMatch_MatchedCandidateWithEmptyPictureYieldsNilImageURL(), TestMatch_NoCloseNameCandidateFailsClosed(), TestMatch_SearchErrorSurfaces(), TestMatch_SingleCandidateIssuesNoTieBreakFetch(), TestMatch_SingleCloseNameCandidate() (+29 more)

### Community 13 - ".HandleLogin"
Cohesion: 0.21
Nodes (9): sync.Once, time.Ticker, Manager, hasCSRFHeader(), setSessionCookie(), writeJSONError(), clientIP(), Manager (+1 more)

### Community 14 - "authStore.ts"
Cohesion: 0.11
Nodes (8): PassphraseScreen(), handleSubmit(), Input(), createSession(), authStore, gateActive, listeners, reimportStore()

### Community 15 - "cn"
Cohesion: 0.11
Nodes (28): Alert(), AlertAction(), AlertDescription(), AlertTitle(), alertVariants, Avatar(), AvatarBadge(), AvatarFallback() (+20 more)

### Community 16 - "testing.T"
Cohesion: 0.18
Nodes (36): testing.T, buildArtistAlbumsPageJSON(), pagedArtistAlbumsServer(), TestArtistAlbums_CancellationBetweenPagesAborts(), TestArtistAlbums_DecodesFixture(), TestArtistAlbums_EmptyArtistIDReturnsErrorWithZeroRequests(), TestArtistAlbums_MidFetchErrorStopsWithNoRetry(), TestArtistAlbums_NonexistentArtistReturnsEmptyNonNilNoError() (+28 more)

### Community 17 - "log/slog.Logger"
Cohesion: 0.11
Nodes (25): stubGroupLister, log/slog.Logger, Detector, nullableString(), deluxeDetectionEnabled(), eventTypeMuted(), releaseTypeAllowed(), TestFilter_DeluxeIsAGateNotAType() (+17 more)

### Community 18 - "context.Context"
Cohesion: 0.09
Nodes (16): stubStore, context.Context, noopPinger, stubPinger, stubStore, Queries, Client, TestNormalizeSet() (+8 more)

### Community 19 - "sync.Mutex"
Cohesion: 0.15
Nodes (6): syncBuf, syncBuffer, net/http.RoundTripper, sync.Mutex, capturingRoundTripper, rateLimitedReleaseGroupSource

### Community 20 - "time.Duration"
Cohesion: 0.11
Nodes (47): discordAlerter, noopAlerter, recordingAlerter, golang.org/x/time/rate.Limit, time.Duration, serverConfig, Alerter, NoOpAlerter() (+39 more)

### Community 21 - "events_test.go"
Cohesion: 0.28
Nodes (22): NewService(), datePtr(), insertTestArtist(), insertTestEvent(), insertTestEventAt(), insertTestEventTyped(), insertTestEventWithDate(), TestHandleListEvents_CursorRoundTripsThroughHTTP() (+14 more)

### Community 22 - "EncodeCursor"
Cohesion: 0.36
Nodes (9): Cursor, cursorWireForm, DecodeCursor(), EncodeCursor(), TestDecodeCursor_NeverPanics(), TestDecodeCursor_RejectsInvalidInput(), TestEncodeCursor_ProducesURLSafeToken(), TestEncodeDecodeCursor_RoundTripsWithNilReleaseDate() (+1 more)

### Community 23 - "Recording"
Cohesion: 0.09
Nodes (20): datedRecordingSource, erroringRecordingSource, fakeRecordingSource, noRecordingSource, isGuestFeature(), creditFor(), TestGuestFeatureArt(), TestIsGuestFeature_EmptyCredit() (+12 more)

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

### Community 29 - "devDependencies"
Cohesion: 0.09
Nodes (23): jsdom, prettier, prettier-plugin-tailwindcss, tailwindcss, @testing-library/dom, @testing-library/react, @testing-library/user-event, @types/node (+15 more)

### Community 30 - "Backfill"
Cohesion: 0.26
Nodes (15): Stats, Store, Backfill(), matchingCandidate(), TestBackfill_ActivityGate_DelaysThenProceeds(), TestBackfill_AllMatch_WritesUpsertAndRecordsAttemptForEach(), TestBackfill_ContextCancelledPartway_StopsPromptly(), TestBackfill_ListArtistsMissingImageErrors_ReturnsErrNoWrites() (+7 more)

### Community 31 - "PROJECT.md"
Cohesion: 0.05
Nodes (51): Bounded worker-pool concurrent polling, Atomic CAS overlap-guard pattern (poll cycles), CLAUDE.md (project instructions), CI coverage gates (80% backend / 70% frontend), CoverArt.tsx image-load-error state never resets on src change, Deezer fan-count capture and popularity sort, DSN/secret redaction on every error path, FOR UPDATE-locked CTE fix for deluxe-change baseline race (+43 more)

### Community 32 - "github.com/jackc/pgx/v5/pgtype.Timestamptz"
Cohesion: 0.09
Nodes (18): notifyGate, Option, RecordingSource, ReleaseDetailSource, Event, github.com/jackc/pgx/v5/pgtype.Timestamptz, recordingQuerier, HasOlderEventsParams (+10 more)

### Community 33 - "match.go"
Cohesion: 0.15
Nodes (20): AlbumLister, ArtistDetailLookup, ArtistFetcher, ArtistSearcher, Matcher, Option, ReleaseGroupLister, aliasQueryNames() (+12 more)

### Community 34 - "components.json"
Cohesion: 0.09
Nodes (21): aliases, components, hooks, lib, ui, utils, iconLibrary, menuAccent (+13 more)

### Community 35 - "NewWithWriter"
Cohesion: 0.19
Nodes (13): io.Writer, log/slog.Level, Config, TestNoDSNInLogs(), New(), NewWithWriter(), parseLevel(), TestNewWithWriter_JSONFormatRendersParsableJSON() (+5 more)

### Community 36 - "Pattern Assignments"
Cohesion: 0.07
Nodes (28): `cmd/server/main.go` (MOD), Disabled-case seam (never a nil check in the request path), File Classification, Fixed JSON error body, never raw error text, Functional-option constructor, `internal/authgate/alerter.go` (service seam — the strongest analog in this phase), `internal/authgate/gate.go` (middleware), `internal/authgate/login.go` (login/logout handlers + throttle + global counter) (+20 more)

### Community 37 - "watchlist.tsx"
Cohesion: 0.13
Nodes (21): Button(), buttonVariants, SearchResultRow(), SearchResultRowProps, SearchResultsColumns(), SearchResultsColumnsProps, SOURCE_LABELS, SourceColumn() (+13 more)

### Community 38 - "EventCard.tsx"
Cohesion: 0.15
Nodes (10): CoverArt(), CoverArtProps, EVENT_BADGE, EventCard(), GuestFeatureBody(), guestFeatureHref(), UNKNOWN_EVENT_BADGE, watchlistNote() (+2 more)

### Community 39 - "run"
Cohesion: 0.06
Nodes (43): logInstanceGateStatus(), main(), run(), decodeRecord(), nonEmptyLines(), recordMentions(), TestLogInstanceGateStatus(), TestRun_BootServesHealthThenGracefulShutdownOnCancel() (+35 more)

### Community 41 - "RunMigrations"
Cohesion: 0.13
Nodes (28): retryConfig, RetryOption, syncBuffer, github.com/golang-migrate/migrate/v4/source.Driver, TestBackoffDelay_ClampsToMaxDelayOnceExceeded(), TestBackoffDelay_GrowsExponentiallyBeforeSaturating(), TestBackoffDelay_SaturatesRatherThanOverflowsToZero(), TestNewRetryConfig_ClampsNonPositiveMaxAttemptsToOne() (+20 more)

### Community 42 - "time.Time"
Cohesion: 0.22
Nodes (7): globalCounter, ipLimiter, loginRequest, loginThrottle, time.Time, loginDelay(), SetSpacingWaitForTest()

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

### Community 48 - "Artist"
Cohesion: 0.19
Nodes (8): perArtistOutcome, perArtistSearcher, stubArtistFetcher, stubSearcher, stubSearcherByQuery, artistSearchResponse, stubDeezerArtistSearcher, Artist

### Community 49 - "Artist"
Cohesion: 0.11
Nodes (12): stubStore, Artist, Queries, UpsertArtistParams, Artist, Watchlist, Queries, toEntry() (+4 more)

### Community 50 - "Server"
Cohesion: 0.12
Nodes (22): chi.Router, net/http.Handler, Option, Pinger, syncBuffer, echoRequestID(), Server, registerDataRoutes() (+14 more)

### Community 57 - "Phase 14 Plan 04: Instance Passphrase Gate — CSRF, Referrer-Policy & Weak-Passphrase WARN Summary"
Cohesion: 0.11
Nodes (18): 1. [Rule 3 — Blocking] `internal/authgate/login_test.go` + `internal/authgate/gate_test.go` shared helpers updated (beyond frontmatter `files_modified` intent), 2. [Rule 1 — Bug] CSRF counter-isolation test tripped a real brute-force alert, 3. [Rule 3 — Blocking] `.env.example` writable; placeholder changed by operator mid-run, Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered (+10 more)

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

### Community 65 - "Phase 14 Plan 01: Instance Passphrase Gate Summary"
Cohesion: 0.13
Nodes (14): 1. [Rule 3 — Blocking] `.env.example` keys added by the operator (not the agent), 2. [Process] RED/GREEN combined per commit, Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness (+6 more)

### Community 66 - "Artist"
Cohesion: 0.36
Nodes (5): stubMusicBrainzArtistSearcher, escapeLucene(), Artist, Client, artistSearchResponse

### Community 67 - "Phase 14 Plan 03: SPA Instance Passphrase Gate Summary"
Cohesion: 0.13
Nodes (14): Accomplishments, Adjustments within plan latitude (not deviations), Auto-fixed Issues, D-18 (`gateActive`) confirmation — per the plan `<output>`, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered (+6 more)

### Community 69 - "Phase 14: Instance Passphrase Gate - Discussion Log"
Cohesion: 0.13
Nodes (14): Boot-time passphrase strength, Brute-force defense beyond per-IP throttle, Claude's Discretion, Deferred Ideas, Follow-up: client IP resolution (behind a Phase 17 reverse proxy), Follow-up: CSRF, Follow-up: how the SPA detects the 401, Follow-up: logout semantics (+6 more)

### Community 72 - "Goal Achievement"
Cohesion: 0.13
Nodes (14): Anti-Patterns Found, Behavioral Spot-Checks, Data-Flow Trace (Level 4), Gaps Summary, Goal Achievement, Human Verification Required, Key Link Verification, Observable Truths (+6 more)

### Community 73 - "Phase 14 Plan 07: Close G-14-3 — Self-Identifying Gated Responses Summary"
Cohesion: 0.14
Nodes (13): Accomplishments, Auto-fixed Issues, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Output confirmations (per plan `<output>`) (+5 more)

### Community 74 - "Milestone v1.1 Audit Report"
Cohesion: 0.67
Nodes (3): CoverArt.tsx Stale Image-Load-Error State Bug, v1.1 Milestone Nyquist Compliance (All 5 Phases), Milestone v1.1 Audit Report

### Community 78 - "api.ts"
Cohesion: 0.17
Nodes (15): SearchBox(), handleChange(), runSearch(), SearchBoxProps, mockSearchArtists, searchResponse, addWatchlist(), ApiError (+7 more)

### Community 89 - "Phase 14: Instance Passphrase Gate - Research"
Cohesion: 0.15
Nodes (12): Architectural Responsibility Map, Assumptions Log, Don't Hand-Roll, Environment Availability, Metadata, Open Questions, Package Legitimacy Audit, Phase 14: Instance Passphrase Gate - Research (+4 more)

### Community 90 - "Requirements: drop-tracker"
Cohesion: 0.15
Nodes (12): Access Gate, Access Gate, CI/CD Pipeline, CI/CD Pipeline, Deployment, Deployment / Operations, Future Requirements, Migration Safety (+4 more)

### Community 95 - "Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary"
Cohesion: 0.17
Nodes (11): Accomplishments, Decisions Made, Deviations from Plan, Files Created/Modified, Issues Encountered, Next Phase Readiness, Performance, Phase 14 Plan 06: Close G-14-2 — persist gateActive for the browser session Summary (+3 more)

### Community 96 - "events/service.go"
Cohesion: 0.24
Nodes (10): stubEventsStore, Service, eventsResponse, eventsResponseBody, stubEventsStore, Event, ListParams, Page (+2 more)

### Community 98 - "Info"
Cohesion: 0.18
Nodes (10): IN-01: Structural "ungated instance" guarantee is asserted only in prose + tests, IN-02: Latch silently no-ops if the API is ever served cross-origin, IN-03: Test matrix gaps on the marker's negative space, IN-04: `apiFetch` reads a header on every response for a session-lifetime one-shot, Info, Phase 14: Code Review Report, Summary, Warnings (+2 more)

### Community 109 - "Tests"
Cohesion: 0.20
Nodes (9): 1. Real-browser cookie behaviour (Chrome + Firefox, http://localhost), 2. PassphraseScreen visual conformance to 14-UI-SPEC, 3. docker compose up with no INSTANCE_PASSPHRASE configured, 4. Live Discord brute-force alert, 5. Log out control persists across a page reload while logged in, Current Test, Gaps, Summary (+1 more)

### Community 110 - "ArtistDetail"
Cohesion: 0.48
Nodes (5): stubArtistDetailLookup, ArtistDetail, ArtistAlias, ArtistRelation, ArtistRelationURL

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

### Community 120 - "Issue tracker: GitHub"
Cohesion: 0.29
Nodes (6): Conventions, Issue tracker: GitHub, Pull requests as a triage surface, Wayfinding operations, When a skill says "fetch the relevant ticket", When a skill says "publish to the issue tracker"

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

### Community 162 - "Security Domain"
Cohesion: 0.67
Nodes (3): Applicable ASVS Categories, Known Threat Patterns for {Go/chi API + embedded React SPA + shared passphrase}, Security Domain

### Community 177 - "NewActivityGate"
Cohesion: 0.20
Nodes (9): sync/atomic.Int32, ActivityGate, NewActivityGate(), TestActivityGate_ActiveWhileBegunNotEnded(), TestActivityGate_ConcurrentUse(), TestActivityGate_DoubleEndDoesNotCorruptState(), TestActivityGate_FreshGateIsNotActive(), TestActivityGate_TwoConcurrentBeginsBothMustEnd() (+1 more)

### Community 180 - "cancelingSearcher"
Cohesion: 0.22
Nodes (6): cancelingSearcher, recordingTimeSearcher, ArtistSearcher, context.CancelFunc, io.ReadCloser, cancelReadCloser

### Community 183 - "artists"
Cohesion: 0.50
Nodes (3): artists, watchlist, events

## Knowledge Gaps
- **434 isolated node(s):** `github.com/danielrpof/drop-tracker`, `loginRequest`, `Queries`, `Client`, `AlbumLister` (+429 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 558 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **19 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `run()` connect `run` to `New`, `poller_test.go`, `New`, `New`, `gate_test.go`, `golang.org/x/time/rate.Limiter`, `NewMatcher`, `context.Context`, `time.Duration`, `events_test.go`, `config_test.go`, `Backfill`, `PROJECT.md`, `github.com/jackc/pgx/v5/pgtype.Timestamptz`, `match.go`, `NewWithWriter`, `RunMigrations`, `NewActivityGate`, `Server`?**
  _High betweenness centrality (0.034) - this node is a cross-community bridge._
- **Why does `New()` connect `New` to `poller_test.go`, `New`, `New`, `TestDetectGuestFeatures_NonSeedCycle_FreshFeatureStillDelivered`, `run`, `events_test.go`?**
  _High betweenness centrality (0.018) - this node is a cross-community bridge._
- **Why does `unnotifiedForArtist()` connect `New` to `testing.T`, `context.Context`, `formatEmbed`?**
  _High betweenness centrality (0.017) - this node is a cross-community bridge._
- **What connects `github.com/danielrpof/drop-tracker`, `loginRequest`, `Queries` to the rest of the system?**
  _434 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `New` be split into smaller, more focused modules?**
  _Cohesion score 0.07278112768433302 - nodes in this community are weakly interconnected._
- **Should `poller_test.go` be split into smaller, more focused modules?**
  _Cohesion score 0.08372093023255814 - nodes in this community are weakly interconnected._
- **Should `New` be split into smaller, more focused modules?**
  _Cohesion score 0.08141321044546851 - nodes in this community are weakly interconnected._