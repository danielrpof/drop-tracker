# Graph Report - drop-tracker  (2026-08-23)

## Corpus Check
- Large corpus: 412 files · ~713,144 words. Semantic extraction will be expensive (many Claude tokens). Consider running on a subfolder.

## Summary
- 1991 nodes · 5038 edges · 137 communities (83 shown, 54 thin omitted)
- Extraction: 92% EXTRACTED · 8% INFERRED · 0% AMBIGUOUS · INFERRED: 384 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- Events Store & Queries
- Cron Poller Core
- Events API Handler Tests
- Pgx Pool & Querier
- Watchlist List/Remove/PATCH Plans
- Event Retention Window
- Concurrency & Secret-Scan Decisions
- Server Boot & Graceful Shutdown
- Dockerfile & CI Hardening
- MusicBrainz Recordings Client Tests
- Coverage Gate Makefile & Plans
- Frontend Vitest Test Suite Plans
- Detection Engine Core Decisions
- Release History Frontend Plans
- SearchBox AbortController Fix
- CoverArt & Alert UI Components
- Artist Albums Client Tests
- Detection Functions & Logging
- DB Pinger & Store Stubs
- Docker Compose & Port Revert
- MusicBrainz/Deezer Search Clients
- Events Service List
- Cross-Cutting Architecture Decisions
- Guest-Feature Detection Sources
- TS Config Globals
- Frontend UI Dependencies
- Config Loader Tests
- Discord Embed Formatting
- Watchlist Core Decisions
- Frontend Dev Tooling Deps
- Config Surface & sqlc Wiring
- Bounded Concurrent Polling Design
- Events Insert & List
- Search Source Adapters
- shadcn/ui Components Config
- Health & Request-ID Config
- Phase 5 Discord Audit Reports
- Search Result Row Country Fallback
- EventCard Crash & Href Bugs
- Pgx Pool Config & DSN
- Deluxe-Change Baseline Detection
- Retry & Buffer Utilities
- Artist Albums Fetch Logic
- Phase 11.1 Audit & Filters UI
- Migrate Retry & DSN Redaction
- Release Detail Source Stubs
- Preference Toggle Checkboxes
- Frontend Tech-Debt Cleanup
- Discord Client Send Logic
- sqlc Generated Models
- HTTP Server & Router Setup
- Notification Message Formatting
- Notifier Hang Fix & Pool
- Guest-Feature & Deluxe Plans
- Phase 1 Foundation Decisions
- Phase 3 Search Client Code Review
- Pool MaxConns Sizing
- Flaky Test Stability Fixes
- Discord Notification Tracer Plans
- Frontend package.json
- Events Table Migrations
- Empty State & Button UI
- Toast & App Shell UI
- Phase 1 Code Review (DSN Leak)
- Loss-Window Signal & PoolConfig
- Backoff Retry Tests
- MusicBrainz Search Query Escaping
- CoverArt Reset Fix
- Phase 12 Popularity Ranking Scope
- Bounded Polling Worker Config
- Postgres Port Revert Decision
- Artist Upsert Query
- Idempotent Seen-Store Insert
- UpdatePreferences No-Op Guard Fix
- v1.1 Milestone Audit
- Phase 5 Closeout & v1.0 Audit
- Atomic CAS Baseline Pattern
- clsx Dependency
- Fontsource Inter Dependency
- Health Response Type
- Poller Overlap Guard
- next-themes Dependency
- Dockerfile Go Version Bump
- Prettier Tailwind Plugin
- Tailwind CSS Dependency
- Testing Library React
- React DOM Types
- TypeScript Dependency
- Web Frontend README
- Phase 1 Discussion Log
- Pitfall: pgxpool Silent Connect
- Pitfall: golangci-lint v1/v2 Confusion
- HTTP Server Timeout Gap
- Stale .gitignore Template
- Health Endpoint Info Disclosure Threat
- Health Endpoint DoS Threat
- Migration Retry DoS Threat
- .env Commit Threat
- Config Error Echo Threat
- SearchBox Comment Fix
- Per-Source Pool Config
- Cycle-End Log Field
- Concurrent Log Ordering
- Rate Limiter Burst Edge Case
- Baseline Tampering Threat
- Broken Windows Defect Ledger
- Recording Source Interface
- Release Detail Source Interface
- Discord Embed Struct
- Artist Credit Entry
- Recording Browse Limit
- Recording Struct
- Release Browse Limit
- Medium Struct
- Release Struct
- Discord NoOp Sink
- Notifier Sender Interface
- Notifier Sink Interface
- Repo Module Path
- Guest-Feature MusicBrainz-Only Decision
- Seed Cycle Pre-Notified Rows
- Implicit Seed Mode Detection
- Per-Source Independent Seeding
- Remove-Re-Add History Preservation
- Idempotency via Unique Constraint
- New-Release Event Detection
- Track Count Baseline Query
- HasAnyEvent Query
- ListExternalIDs Query
- SetGroupTrackCountBaseline Query

## God Nodes (most connected - your core abstractions)
1. `New()` - 111 edges
2. `NewTestPool()` - 107 edges
3. `New()` - 75 edges
4. `discardLogger()` - 58 edges
5. `newTestLogger()` - 58 edges
6. `New()` - 50 edges
7. `New()` - 49 edges
8. `newTestClient()` - 45 edges
9. `NewService()` - 44 edges
10. `insertTestArtist()` - 43 edges

## Surprising Connections (you probably didn't know these)
- `deluxeDetectionEnabled()` --semantically_similar_to--> `musicbrainz.ReleaseGroup`  [INFERRED] [semantically similar]
  internal/detection/filter.go → .planning/milestones/v1.0-phases/03-external-clients-search/03-03-PLAN.md
- `sendAttempt` --semantically_similar_to--> `coverArtURLForReleaseGroup()`  [INFERRED] [semantically similar]
  .planning/milestones/v1.0-phases/05-discord-notifications/05-01-PLAN.md → internal/detection/musicbrainz.go
- `Phase 2 Plan 02: Duplicate-Add & Preferences Plan` --references--> `redactedTarget()`  [AMBIGUOUS]
  .planning/milestones/v1.0-phases/02-watchlist-core/02-02-PLAN.md → internal/db/pool.go
- `Graceful shutdown via signal.NotifyContext` --references--> `run()`  [EXTRACTED]
  .planning/PROJECT.md → cmd/server/main.go
- `caarlos0/env config parsing` --references--> `Load()`  [EXTRACTED]
  .claude/CLAUDE.md → internal/config/config.go

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **Full Pipeline CI/CD job set (lint, test, scan, sbom, release)** — github_workflows_full_pipeline_doc, tech_golangci_lint_v2, tech_trivy, tech_gitleaks, tech_svu, tech_syft_sbom [EXTRACTED 1.00]
- **Phase 12 scope: CoverArt reset plus search popularity/disambiguation work** — phase12_cleanup, coverart_reset_bug, search_popularity_disambiguation, deezer_fan_count_capture, musicbrainz_country_fallback [EXTRACTED 1.00]
- **Notifier hang bug AND-gate: two independent causes composing into one production incident** — notify_pass_hang_bug, internal_notifier_notifier_notifypending, internal_db_pool_newpool, postgres_port_remap [INFERRED 0.85]
- **Phase 1 Service Boot Sequence** — cmd_server_main_run, internal_config_config_load, internal_db_migrate_runmigrations, internal_httpserver_server_new [INFERRED 0.85]
- **Watchlist Add Validation Pipeline** — internal_httpserver_watchlist_handleaddwatchlist, internal_watchlist_service_add, internal_watchlist_service_normalizeset, _planning_milestones_v1_0_phases_02_watchlist_core_02_02_plan_t0213 [EXTRACTED 1.00]
- **DSN Redaction Hardening Effort** — internal_db_migrate_redactdsn, internal_db_pool_redactedtarget, _planning_milestones_v1_0_phases_01_foundation_data_layer_config_health_01_review_cr01, _planning_milestones_v1_0_phases_01_foundation_data_layer_config_health_01_security_t0101 [EXTRACTED 1.00]
- **Phase 02 design-decision documentation set (context, discussion log, research, patterns)** — _planning_milestones_v1_0_phases_02_watchlist_core_02_context, _planning_milestones_v1_0_phases_02_watchlist_core_02_discussion_log, _planning_milestones_v1_0_phases_02_watchlist_core_02_research, _planning_milestones_v1_0_phases_02_watchlist_core_02_patterns [INFERRED 0.85]
- **G-02-1 gap closure flow: UAT finding through plan 02-07 to re-verification** — _planning_milestones_v1_0_phases_02_watchlist_core_02_uat, _planning_milestones_v1_0_phases_02_watchlist_core_02_07_plan, _planning_milestones_v1_0_phases_02_watchlist_core_02_07_summary, concept_gap_g_02_1, _planning_milestones_v1_0_phases_02_watchlist_core_02_verification [EXTRACTED 0.90]
- **G-02-2/CR-01 gap closure flow: UAT finding through plan 02-08 to security register and re-verification** — _planning_milestones_v1_0_phases_02_watchlist_core_02_uat, _planning_milestones_v1_0_phases_02_watchlist_core_02_08_plan, _planning_milestones_v1_0_phases_02_watchlist_core_02_08_summary, concept_gap_g_02_2, _planning_milestones_v1_0_phases_02_watchlist_core_02_verification, _planning_milestones_v1_0_phases_02_watchlist_core_02_security [EXTRACTED 0.90]
- **Phase 3 External Clients & Search Plan Sequence** — _planning_milestones_v1_0_phases_03_external_clients_search_03_01_plan, _planning_milestones_v1_0_phases_03_external_clients_search_03_02_plan, _planning_milestones_v1_0_phases_03_external_clients_search_03_03_plan, _planning_milestones_v1_0_phases_03_external_clients_search_03_04_plan [EXTRACTED 1.00]
- **Narrow-Interface Consumer-Declared Seam Pattern** — internal_musicbrainz_client_artistsearcher, internal_deezer_client_artistsearcher, internal_httpserver_search_searchsource, internal_poller_poller_eventrecorder, internal_watchlist_service_store [INFERRED 0.85]
- **Phase 3 Quality-Gate Document Chain** — _planning_milestones_v1_0_phases_03_external_clients_search_03_review, _planning_milestones_v1_0_phases_03_external_clients_search_03_review_fix, _planning_milestones_v1_0_phases_03_external_clients_search_03_uat, _planning_milestones_v1_0_phases_03_external_clients_search_03_verification [EXTRACTED 1.00]
- **Guest-Feature Detection Pipeline (DTCT-03)** — internal_musicbrainz_recordings_recordingsbyartist, internal_detection_musicbrainz_isguestfeature, internal_detection_musicbrainz_detectguestfeatures, internal_detection_detector_recordingsource [INFERRED 0.85]
- **Deluxe-Change Baseline Pipeline (DTCT-02)** — internal_musicbrainz_releases_releasesbyreleasegroup, internal_detection_detector_groupbaseline, internal_detection_detector_setgroupbaseline, internal_detection_musicbrainz_detectdeluxechanges [INFERRED 0.85]
- **Discord Delivery Outbox Pipeline (NTFY-01..04)** — internal_notifier_notifier_notifypending, internal_notifier_format_formatembed, internal_discord_client_send, queries_events_marknotified [INFERRED 0.85]
- **Phase 5 Discord Notifications Sign-off Gate** — _planning_milestones_v1_0_phases_05_discord_notifications_05_review, _planning_milestones_v1_0_phases_05_discord_notifications_05_review_fix, _planning_milestones_v1_0_phases_05_discord_notifications_05_security, _planning_milestones_v1_0_phases_05_discord_notifications_05_validation, _planning_milestones_v1_0_phases_05_discord_notifications_05_verification, _planning_milestones_v1_0_phases_05_discord_notifications_05_uat [INFERRED 0.85]
- **Phase 6 Wave-Based Execution Plans** — _planning_milestones_v1_0_phases_06_frontend_release_history_06_01_plan, _planning_milestones_v1_0_phases_06_frontend_release_history_06_02_plan, _planning_milestones_v1_0_phases_06_frontend_release_history_06_03_plan, _planning_milestones_v1_0_phases_06_frontend_release_history_06_04_plan [INFERRED 0.90]
- **Phase 6 Shared Frontend Surface (api.ts, CoverArt, EmptyState)** — web_app_components_common_coverart_coverart, web_app_components_common_emptystate_emptystate, _planning_milestones_v1_0_phases_06_frontend_release_history_06_02_plan, _planning_milestones_v1_0_phases_06_frontend_release_history_06_03_plan, _planning_milestones_v1_0_phases_06_frontend_release_history_06_04_plan [INFERRED 0.85]
- **Vulnerability Gate Strictness Decision Group (D-07/D-08/D-09)** — concept_d07_trivy_critical_high_gate, concept_d08_trivyignore_escape_hatch, concept_d09_pr_build_scan_no_push [EXTRACTED 0.90]
- **docker-compose Dev-Loop Shape Decision Group (D-10/D-11/D-12)** — concept_d10_compose_build_local_image, concept_d11_compose_env_file, concept_d12_compose_database_url_override, concept_dockercompose_app_service [EXTRACTED 0.90]
- **Phase 07 Code Review Findings Closed by Post-Execution Hardening** — concept_cr01_lint_missing_setup_go, concept_cr02_scan_push_image_mismatch, concept_wr01_trivy_fs_scan_missing, concept_wr02_gosec_linter_missing, concept_wr03_sbom_output_destination, concept_wr04_concurrent_release_race [EXTRACTED 0.90]
- **RED-then-GREEN commit pair convention applied to all three folded bugs** — planning_milestones_v1_1_phases_08_frontend_test_suite_08_context_d07, planning_milestones_v1_1_phases_08_frontend_test_suite_08_context_folded_eventcard_crash, planning_milestones_v1_1_phases_08_frontend_test_suite_08_context_folded_searchbox_abortcontroller, planning_milestones_v1_1_phases_08_frontend_test_suite_08_context_folded_guestfeaturehref_encoding, planning_milestones_v1_1_phases_08_frontend_test_suite_08_03_plan, planning_milestones_v1_1_phases_08_frontend_test_suite_08_04_plan, planning_milestones_v1_1_phases_08_frontend_test_suite_08_05_plan [EXTRACTED 1.00]
- **Phase 8 pre-planning context/research package** — planning_milestones_v1_1_phases_08_frontend_test_suite_08_context, planning_milestones_v1_1_phases_08_frontend_test_suite_08_research, planning_milestones_v1_1_phases_08_frontend_test_suite_08_patterns, planning_milestones_v1_1_phases_08_frontend_test_suite_08_discussion_log [INFERRED 0.85]
- **Phase 8 post-execution quality-gate audit trail** — planning_milestones_v1_1_phases_08_frontend_test_suite_08_review, planning_milestones_v1_1_phases_08_frontend_test_suite_08_security, planning_milestones_v1_1_phases_08_frontend_test_suite_08_uat, planning_milestones_v1_1_phases_08_frontend_test_suite_08_validation, planning_milestones_v1_1_phases_08_frontend_test_suite_08_verification [INFERRED 0.80]
- **Backend coverage gate delivery chain (measure, close gap, wire into CI)** — _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_01_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_03_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_05_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_context_cicd_11 [INFERRED 0.85]
- **Frontend coverage gate delivery chain (measure, close gap, wire into CI)** — _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_02_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_04_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_05_plan_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_context_cicd_12 [INFERRED 0.85]
- **Phase 9 verification and audit trail (review, security, UAT, validation)** — _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_verification_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_security_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_uat_doc, _planning_milestones_v1_1_phases_09_ci_coverage_gates_09_validation_doc [INFERRED 0.80]
- **Detection-state queries that must stay permanently unfiltered by retention** — _planning_milestones_v1_1_phases_10_event_retention_window_10_01_plan_listexternalids_query, _planning_milestones_v1_1_phases_10_event_retention_window_10_01_plan_hasanyevent_query, _planning_milestones_v1_1_phases_10_event_retention_window_10_01_plan_listunnotified_query, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_03_plan_advancegrouptrackcountbaseline_query, _planning_milestones_v1_1_phases_10_event_retention_window_10_01_plan_testretention_detectionstatequeriesstayunfiltered [EXTRACTED 1.00]
- **Bounded worker-pool fan-out pattern mirrored across MusicBrainz and Deezer poll cycles** — _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_01_plan_poller_option_type, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_01_plan_withmusicbrainzworkers, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_02_plan_withdeezerworkers, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_01_plan_runmusicbrainzcycle_fanout, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_02_plan_rundeezercycle_fanout [EXTRACTED 1.00]
- **Flaky-test root-cause fixes required for a trustworthy PERF-04 race proof** — _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_04_plan_migrate_scratch_isolation, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_04_plan_spacing_seam, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_04_plan_flaky_test_todo, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_03_plan_perf_04 [EXTRACTED 1.00]
- **Phase 11 Gap G-11-1 Closure Flow** — _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_uat_gap_g11_1, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_05_plan_doc, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_05_summary_doc, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_verification_doc, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_validation_doc [EXTRACTED 1.00]
- **Phase 11.1 Backend Debt Closure (D-10/D-11/D-12)** — _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_review_wr01_doc_undersells_blast_radius, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_review_wr02_notification_loss_window, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_review_in01_identical_error_text, _planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_03_plan_doc, _planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_03_summary_doc [EXTRACTED 1.00]
- **Flaky Test Suite Stabilization Effort** — _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_context_folded_todo_flaky_tests, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_research_pitfall6_migrate_schema_drop, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_research_pitfall7_notifier_clock_injection, _planning_milestones_v1_1_phases_11_bounded_concurrent_polling_11_04_summary_doc [EXTRACTED 1.00]
- **Phase 11.1 GSD Planning-Execution-Verification Chain** — planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_discussion_log_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_research_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_patterns_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_05_plan_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_05_summary_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_validation_doc, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_verification_doc [EXTRACTED 1.00]
- **Phase 12 GSD Planning-Execution Chain** — planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_context_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_discussion_log_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_research_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_patterns_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_01_plan_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_02_plan_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_03_plan_doc, planning_phases_12_cleanup_coverart_reset_search_popularity_ranking_12_validation_doc [EXTRACTED 1.00]
- **Phase 11.1 Locked Tech-Debt Decisions (D-01..D-13)** — planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d01, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d02, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d03, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d04, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d05, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d06, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d07, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d08, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d09, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d10, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d11, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d12, planning_milestones_v1_1_phases_11_1_address_tech_debt_v1_1_cleanup_11_1_context_d13 [INFERRED 0.85]
- **Events Table Dual-Role Retention Pitfalls** — concept_pitfall_seed_mode_reset, concept_pitfall_baseline_loss, concept_pitfall_dedup_key_loss, internal_detection_detector [EXTRACTED 1.00]
- **Concurrent Polling Worker-Pool Risk Set** — concept_pitfall_baseline_race, concept_pitfall_rate_limiter_burst, concept_pitfall_unrecovered_panic, concept_pitfall_db_pool_sizing, internal_poller_poller [EXTRACTED 1.00]
- **Phase 5 Formal Closeout Flow** — planning_quick_260808_pt0_close_out_phase_5_commit_docs_cleanup_st_260808_pt0_plan, planning_quick_260808_pt0_close_out_phase_5_commit_docs_cleanup_st_260808_pt0_summary, planning_v1_0_milestone_audit [INFERRED 0.85]

## Communities (137 total, 54 thin omitted)

### Community 0 - "Events Store & Queries"
Cohesion: 0.08
Nodes (105): insertEventFailingQuerier, Queries, New(), TestSQLCPing(), insertBaselineNewRelease(), int32Ptr(), readBaselineTrackCount(), TestAdvanceGroupBaseline_ConcurrentRace() (+97 more)

### Community 1 - "Cron Poller Core"
Cohesion: 0.09
Nodes (77): cron.Cron, bytes.Buffer, New(), decodeLogRecords(), deezerID(), eightEntries(), failFirstThree(), newTestLogger() (+69 more)

### Community 2 - "Events API Handler Tests"
Cohesion: 0.10
Nodes (61): testing.T, errorBody, healthBody, watchlistEntryBody, TestHandleListEvents_EmptyReturnsEmptyArrayAndNullCursor(), TestHandleListEvents_HappyPathReturns200WithEnvelope(), TestHandleListEvents_NilEventsSliceStillEncodesAsEmptyArray(), TestHandleListEvents_StoreErrorReturns500WithFixedMessage() (+53 more)

### Community 3 - "Pgx Pool & Querier"
Cohesion: 0.09
Nodes (53): github.com/jackc/pgx/v5/pgxpool.Pool, sync/atomic.Bool, sync/atomic.Int32, time.Duration, Querier, NewClient(), TestSend_429RetryAfterClamped(), TestSend_429ThenSuccess_HonorsRetryAfter() (+45 more)

### Community 4 - "Watchlist List/Remove/PATCH Plans"
Cohesion: 0.08
Nodes (61): Phase 02 Plan 03: Watchlist List and Remove, Phase 02 Plan 03 Summary, Phase 02 Plan 04: PATCH Preferences Update, Phase 02 Plan 04 Summary, Phase 02 Plan 05: Close Gap G-02-2a, Phase 02 Plan 05 Summary, Phase 02 Plan 06: Close Gap G-02-2b, Phase 02 Plan 06 Summary (+53 more)

### Community 5 - "Event Retention Window"
Cohesion: 0.07
Nodes (60): Phase 10 Plan 01: Event Retention Window, DATA-01: Configurable event retention window, DATA-02: History/API exclude aged-out events, Config.EventRetentionDays, HasAnyEvent query (must stay unfiltered), ListEvents retention cutoff predicate, ListExternalIDs query (must stay unfiltered), ListUnnotified query (must stay unfiltered) (+52 more)

### Community 6 - "Concurrency & Secret-Scan Decisions"
Cohesion: 0.05
Nodes (43): D-10: dedup key excludes artist_id (global dedup), errgroup.SetLimit Bounded-Concurrency Pattern, Events Retention: Soft-Delete vs Hard-Delete Design Decision, Local Gitleaks Pre-commit Secret Scanning Hook, Local golangci-lint Pre-commit Hook (Changed-Files Only), Pitfall: TTL Delete Destroys Deluxe-Change Baseline, Pitfall: Concurrent Worker Pool TOCTOU Race on Shared Baseline, Pitfall: Coverage Gate Retrofit Breaks Build on Unmeasured Baseline (+35 more)

### Community 7 - "Server Boot & Graceful Shutdown"
Cohesion: 0.06
Nodes (36): D-06: Boot test selects on run()'s error channel instead of timing out, main(), run(), TestRun_BootServesHealthThenGracefulShutdownOnCancel(), TestRun_ConfigLoadFailureReturnsEarly(), net/http.Request, net/http.Response, net/http.ResponseWriter (+28 more)

### Community 8 - "Dockerfile & CI Hardening"
Cohesion: 0.09
Nodes (54): CICD-08: Third-Party Action Commit-SHA Pinning, CR-01: lint Job Missing Pinned Go Toolchain, CR-02: Scanned Image Never Equals Pushed Image, D-01: Alpine Final-Stage Base Image (Debuggability vs Distroless), D-02: Fixed Non-Root UID/GID 10001:10001, D-03: Dockerfile HEALTHCHECK Against /health, D-04: Seed v0.1.0, svu Computes Subsequent Versions, D-05: Public ghcr.io Image Package (+46 more)

### Community 9 - "MusicBrainz Recordings Client Tests"
Cohesion: 0.10
Nodes (50): net/http/httptest.Server, buildRecordingPageJSON(), pagedRecordingServer(), TestRecordingsByArtist_DecodesEnvelope(), TestRecordingsByArtist_EmptyMBID(), TestRecordingsByArtist_NonOKStatus(), TestRecordingsByArtist_Paginates(), TestRecordingsByArtist_RequestShape() (+42 more)

### Community 10 - "Coverage Gate Makefile & Plans"
Cohesion: 0.07
Nodes (53): Makefile COVER_PKGS variable (-coverpkg package-list exclusion), Makefile coverage-gate target, Makefile COVERAGE_THRESHOLD_BACKEND variable (default 80), Phase 9 Plan 01: Backend Coverage Measurement & Gate, Phase 9 Plan 01 Summary: Backend Coverage Measurement & Gate, Phase 9 Plan 02: Frontend Coverage Provider + Baseline, Phase 9 Plan 02 Summary: Frontend Coverage Provider + Baseline, Phase 9 Plan 03: Backend Coverage Gap Closure (+45 more)

### Community 11 - "Frontend Vitest Test Suite Plans"
Cohesion: 0.11
Nodes (48): Phase 8 Plan 01: Vitest Harness Tracer, frontend-test CI job (parallel tier), Phase 8 Plan 01 Summary, Phase 8 Plan 02: Router Stub + Watchlist Tests, Phase 8 Plan 02 Summary, Phase 8 Plan 03: Search Surface + AbortController Fix, Phase 8 Plan 03 Summary, Phase 8 Plan 04: EventCard Badge Fallback Fix (+40 more)

### Community 12 - "Detection Engine Core Decisions"
Cohesion: 0.06
Nodes (43): Phase 3 Plan 4 Summary, Phase 4 Plan 1: Thin End-to-End Detection Slice, D-09: events table as combined seen-store/event-log, D-12: event display snapshot columns (title/artist_name/release_date/cover_art_url), D-14: implicit seed mode, no seeded_at column, D-19: recovery is re-derivation, not resume state, D-20: never overwrite an event's display snapshot, T-04-01: range-only iteration over externally-supplied release-group slice (+35 more)

### Community 13 - "Release History Frontend Plans"
Cohesion: 0.09
Nodes (40): Phase 6 Plan 1: End-to-End Release History Slice, Phase 6 Plan 1 Summary, Phase 6 Plan 2: History Filters, Cards, Query-Param Validation, Phase 6 Plan 2 Summary, Phase 6 Plan 3: Watchlist Tab, Phase 6 Plan 3 Summary, Phase 6 Plan 4: Artist Search & Phase Sign-off, Phase 6 Plan 4 Summary (+32 more)

### Community 14 - "SearchBox AbortController Fix"
Cohesion: 0.10
Nodes (27): D-03: Fix SearchBox.test.tsx stale comment, D-05: Add watchlist/HistoryFilters test coverage, SearchBox AbortController Never Cancels Fetch (Todo), SearchBox(), handleChange(), runSearch(), SearchBoxProps, mockSearchArtists (+19 more)

### Community 15 - "CoverArt & Alert UI Components"
Cohesion: 0.10
Nodes (29): CoverArtProps, Alert(), AlertAction(), AlertDescription(), AlertTitle(), alertVariants, Avatar(), AvatarBadge() (+21 more)

### Community 16 - "Artist Albums Client Tests"
Cohesion: 0.14
Nodes (35): golang.org/x/time/rate.Limiter, net/http.Client, buildArtistAlbumsPageJSON(), pagedArtistAlbumsServer(), TestArtistAlbums_CancellationBetweenPagesAborts(), TestArtistAlbums_DecodesFixture(), TestArtistAlbums_EmptyArtistIDReturnsErrorWithZeroRequests(), TestArtistAlbums_MidFetchErrorStopsWithNoRetry() (+27 more)

### Community 17 - "Detection Functions & Logging"
Cohesion: 0.11
Nodes (18): log/slog.Logger, Detector, nullableString(), seedNotifiedAt(), eventTypeMuted(), coverArtURLForReleaseGroup(), Detector, releaseTypeForStorage() (+10 more)

### Community 18 - "DB Pinger & Store Stubs"
Cohesion: 0.13
Nodes (13): context.Context, noopPinger, stubPinger, stubStore, Queries, TestNormalizeSet(), AddParams, Entry (+5 more)

### Community 19 - "Docker Compose & Port Revert"
Cohesion: 0.11
Nodes (30): docker-compose.yml, .env.example, Makefile, Phase 11.1 Plan 05: Amend Audit & Reconcile Validation, Phase 11.1 Plan 05 Summary, D-01: Keep Postgres port 5432 revert, D-02: Update port rationale comments, D-04: Fix EventCard.tsx quotes + add CI prettier gate (+22 more)

### Community 20 - "MusicBrainz/Deezer Search Clients"
Cohesion: 0.10
Nodes (29): Phase 3 Plan 1: MusicBrainz Client and GET /search Proxy, T-03-04: MusicBrainz User-Agent spoofing mitigation, T-03-08: DoS via unauthenticated /search amplifying MusicBrainz traffic, Phase 3 Plan 1 Summary, Phase 3 Plan 2: Deezer Client and Two-Source Search, Phase 3 Plan 2 Summary, Phase 3 Plan 3: MusicBrainz Release-Groups Browse-by-Artist, T-03-12: unbounded release-group pagination DoS (+21 more)

### Community 21 - "Events Service List"
Cohesion: 0.21
Nodes (23): Service, eventsResponseBody, stubEventsStore, Event, ListParams, Page, NewService(), toEvent() (+15 more)

### Community 22 - "Cross-Cutting Architecture Decisions"
Cohesion: 0.12
Nodes (22): Bounded worker-pool concurrent polling, Atomic CAS overlap-guard pattern (poll cycles), CLAUDE.md (project instructions), CI coverage gates (80% backend / 70% frontend), FOR UPDATE-locked CTE fix for deluxe-change baseline race, full-pipeline.yml (GitHub Actions workflow), .golangci.yml (lint config), Graceful shutdown via signal.NotifyContext (+14 more)

### Community 23 - "Guest-Feature Detection Sources"
Cohesion: 0.11
Nodes (19): fakeRecordingSource, noRecordingSource, detectGuestFeatures, displayArtistName(), isGuestFeature(), creditFor(), TestIsGuestFeature_EmptyCredit(), TestIsGuestFeature_MissingArtistID() (+11 more)

### Community 24 - "TS Config Globals"
Cohesion: 0.08
Nodes (25): **/*, **/.client/**/*, DOM, DOM.Iterable, ES2022, node, .react-router/types/**/*, **/.server/**/* (+17 more)

### Community 25 - "Frontend UI Dependencies"
Cohesion: 0.08
Nodes (25): @base-ui/react, class-variance-authority, isbot, lucide-react, react, react-dom, react-router, @react-router/node (+17 more)

### Community 26 - "Config Loader Tests"
Cohesion: 0.19
Nodes (23): Load(), configEnvKeys(), envExampleKeys(), repoRoot(), setDiff(), setRequired(), TestDotEnvIsNotTracked(), TestEnvExampleCompleteness() (+15 more)

### Community 27 - "Discord Embed Formatting"
Cohesion: 0.16
Nodes (23): formatEmbed(), emojiPrefix(), i32Ptr(), strPtr(), TestFormatEmbed_AllNilOptionalFields_NoEmptyFieldsNoThumbnail(), TestFormatEmbed_DeluxeChange_BothCountsPresent(), TestFormatEmbed_DeluxeChange_NilBothCounts(), TestFormatEmbed_DeluxeChange_NilPreviousTrackCount() (+15 more)

### Community 28 - "Watchlist Core Decisions"
Cohesion: 0.13
Nodes (23): Phase 2 Plan 01: Watchlist Core Tracer Plan, D-08 (Phase 2): Adding an artist defaults to full visibility, D-09 (Phase 2): Duplicate add never implicitly updates preferences, D-11 (Phase 2): Optional initial preferences override defaults, D-13 (Phase 2): Error bodies never leak internals, D-14 (Phase 2): Flat routes, no /api prefix, T-02-01: Over-posting via handler body decode, T-02-02: SQL injection via handler-constructed queries (+15 more)

### Community 29 - "Frontend Dev Tooling Deps"
Cohesion: 0.09
Nodes (23): jsdom, prettier, @react-router/dev, @tailwindcss/vite, @testing-library/jest-dom, @testing-library/user-event, @types/node, @types/react (+15 more)

### Community 30 - "Config Surface & sqlc Wiring"
Cohesion: 0.12
Nodes (22): Phase 1 Plan 3: Complete Config Surface Plan, Phase 1 Plan 3: Complete Config Surface Summary, Phase 1 Plan 4: Wire sqlc End to End Plan, Phase 1 Plan 4: sqlc + Makefile Summary, Phase 1 Plan 5: Injectable Migrate Retry Policy Plan, Phase 1 Pattern Map, Phase 1 Foundation Research, OPS-01: /health reports service and DB connectivity (+14 more)

### Community 31 - "Bounded Concurrent Polling Design"
Cohesion: 0.13
Nodes (20): D-03: Partial results on source failure (HTTP 200), D-09: per-source in-process overlap guard, Pattern 1: Bounded concurrent fan-out with per-worker error isolation, Pitfall 2: Existing test hard-asserts sequential-only polling, Pitfall 3: errgroup.WithContext cancels siblings on first error, deezer.AlbumLister, deezer.ArtistSearcher, deezer.Client (internal/deezer/client.go) (+12 more)

### Community 32 - "Events Insert & List"
Cohesion: 0.13
Nodes (10): Event, recordingQuerier, HasOlderEventsParams, InsertEventParams, ListEventsParams, ListEventsRow, Queries, AdvanceGroupTrackCountBaselineParams (+2 more)

### Community 33 - "Search Source Adapters"
Cohesion: 0.16
Nodes (22): deezer.Artist struct, deezer.SearchArtists, deezerSource.SearchArtists adapter, musicBrainzSource.SearchArtists adapter, httpserver.SearchArtist wire struct, musicbrainz.Artist, Phase 03 Research (v1.0), Phase 06 Plan 04 Summary (v1.0) (+14 more)

### Community 34 - "shadcn/ui Components Config"
Cohesion: 0.09
Nodes (21): aliases, components, hooks, lib, ui, utils, iconLibrary, menuAccent (+13 more)

### Community 35 - "Health & Request-ID Config"
Cohesion: 0.14
Nodes (18): Phase 1 Plan 2 Summary: Health & Request-ID Test Coverage, io.Writer, log/slog.Level, Config, TestBootToHealth_EndToEnd(), handleHealth (internal/httpserver/health.go), New(), NewWithWriter() (+10 more)

### Community 36 - "Phase 5 Discord Audit Reports"
Cohesion: 0.15
Nodes (21): Phase 5 Discord Notifications Research, Phase 5 Code Review Report, Phase 5 Code Review Fix Report, Phase 5 Security Report, Phase 5 UAT Report, Phase 5 Validation Strategy, Phase 5 Verification Report, Phase 5 Discord Webhook API Coverage Matrix (+13 more)

### Community 37 - "Search Result Row Country Fallback"
Cohesion: 0.15
Nodes (16): D-10: Render country fallback in same UI slot, SearchResultRow(), SearchResultRowProps, SearchResultsColumnsProps, SOURCE_LABELS, SourceColumn(), SourceColumnProps, sourceLabel() (+8 more)

### Community 38 - "EventCard Crash & Href Bugs"
Cohesion: 0.14
Nodes (11): EventCard Crashes on Unrecognized Event Type (Todo), guestFeatureHref Missing encodeURIComponent (Todo), EVENT_BADGE, EventCardProps, GuestFeatureBody(), guestFeatureHref(), UNKNOWN_EVENT_BADGE, HistoryFiltersValue (+3 more)

### Community 39 - "Pgx Pool Config & DSN"
Cohesion: 0.19
Nodes (15): T-01-01: DSN leakage via logs/errors, D-11: Differentiate PoolConfig's two parse-failure error messages, Pitfall: Worker Pool Serializes on Unreviewed pgxpool MaxConns, github.com/jackc/pgx/v5/pgxpool.Config, dsnSetsMaxConns(), PoolConfig(), poolMaxConnsForWorkers(), redactedTarget() (+7 more)

### Community 40 - "Deluxe-Change Baseline Detection"
Cohesion: 0.12
Nodes (17): D-10: detectDeluxeChanges doc comment permanent-loss/delayed asymmetry, D-12: Static window=baseline_advanced_insert_failed log signal, D-04: Cycle-end duration_ms log field, Pitfall 1: Reordering baseline-advance and event-insert can swallow a notification, setGroupBaseline, detectDeluxeChanges, RecordingsByArtist, ReleasesByReleaseGroup (+9 more)

### Community 41 - "Retry & Buffer Utilities"
Cohesion: 0.26
Nodes (13): RetryOption, syncBuffer, closedPortDSN(), closedPortKeywordValueDSN(), countWarnLines(), newCapturingLogger(), TestRunMigrations_HonoursContextCancellation(), TestRunMigrations_NeverLogsDSN() (+5 more)

### Community 42 - "Artist Albums Fetch Logic"
Cohesion: 0.16
Nodes (9): artistAlbumsResponse, sync.Mutex, time.Time, syncBuffer, Album, Client, fakeAlbumSource, rateLimitedAlbumSource (+1 more)

### Community 43 - "Phase 11.1 Audit & Filters UI"
Cohesion: 0.13
Nodes (11): Phase 11.1 Code Review Report, Phase 11.1 Validation Strategy, Phase 11.1 Verification Report, ComboboxOption, ComboboxProps, EVENT_TYPE_OPTIONS, HistoryFilters(), HistoryFiltersProps (+3 more)

### Community 44 - "Migrate Retry & DSN Redaction"
Cohesion: 0.27
Nodes (13): Phase 1 Plan 5: Injectable Migrate Retry Policy Summary, DSN/secret redaction on every error path, github.com/golang-migrate/migrate/v4/source.Driver, redactDSN(), redactError(), RunMigrations(), runMigrationsOnce(), TestRedactDSN_NeverEchoesPassword() (+5 more)

### Community 45 - "Release Detail Source Stubs"
Cohesion: 0.18
Nodes (7): fakeReleaseDetailSource, noReleaseDetailSource, Client, Release, Medium, releaseEnvelope, fakeReleaseDetailSource

### Community 46 - "Preference Toggle Checkboxes"
Cohesion: 0.23
Nodes (11): Checkbox(), PreferenceToggles(), toggleMutedEventType(), toggleReleaseType(), PreferenceTogglesProps, entry, mockUpdateWatchlistPreferences, WatchlistRow() (+3 more)

### Community 47 - "Frontend Tech-Debt Cleanup"
Cohesion: 0.16
Nodes (13): D-05a: watchlist.test.tsx success-path row-removal coverage gap, D-05b: HistoryFilters.test.tsx eventType axis coverage gap, D-13: White-on-white History filter dropdown option text bug, Phase 11.1 Plan 01 — Frontend Tech-Debt Cleanup, Phase 11.1 Plan 01 Summary — Frontend Tech-Debt Cleanup, D-04: Whole-tree prettier formatting + blocking CI check, Phase 11.1 Plan 02 — Frontend Formatting + CI Gate, Phase 11.1 Plan 02 Summary — Frontend Formatting + CI Gate (+5 more)

### Community 48 - "Discord Client Send Logic"
Cohesion: 0.23
Nodes (9): allowedMentions, Client, EmbedImage, retry429Body, webhookPayload, Embed, EmbedField, fakeSender (+1 more)

### Community 49 - "sqlc Generated Models"
Cohesion: 0.21
Nodes (8): github.com/jackc/pgx/v5/pgtype.Timestamptz, Artist, Watchlist, Queries, CreateWatchlistEntryParams, ListWatchlistRow, UpdateWatchlistPreferencesParams, UpdateWatchlistPreferencesRow

### Community 50 - "HTTP Server & Router Setup"
Cohesion: 0.23
Nodes (11): net/http.Handler, Pinger, Store, echoRequestID(), Server, newCapturingServer(), requestIDsInLog(), TestNoDSNInLogs() (+3 more)

### Community 51 - "Notification Message Formatting"
Cohesion: 0.31
Nodes (13): Event, appendField(), formatDeluxeChange(), formatGuestFeature(), formatNewRelease(), musicBrainzRecordingURL(), musicBrainzReleaseGroupURL(), musicBrainzReleaseURL() (+5 more)

### Community 52 - "Notifier Hang Fix & Pool"
Cohesion: 0.24
Nodes (11): NewPool(), sqlc.Querier (interface), NotifyPending (internal/notifier/notifier.go), Notifier NotifyPending permanent hang (resolved), Debug: notify pass hangs forever, WR-01: Un-muting new_release after a mute period floods a notification burst, D-06: Shared CAS guard prevents double-posting across cycles, D-07: Serial sends with spacing between events (+3 more)

### Community 53 - "Guest-Feature & Deluxe Plans"
Cohesion: 0.28
Nodes (13): Phase 4 Plan 3: Guest-Feature Detection, Phase 4 Plan 3 Summary: Guest-Feature Detection, Phase 4 Plan 4: Deluxe/Tracklist-Change Detection, Phase 4 Plan 4 Summary: Deluxe/Tracklist-Change Detection, Phase 4 Context: Detection Engine, Phase 4 Discussion Log: Detection Engine, Phase 4 Pattern Map: Detection Engine, Phase 4 Research: Detection Engine (+5 more)

### Community 54 - "Phase 1 Foundation Decisions"
Cohesion: 0.17
Nodes (12): Phase 1 Foundation Context, D-01: Bookkeeping-only initial schema, D-02: Wire sqlc in Phase 1 with drift check, D-03: /health DB ping only, no readiness split, D-04: 503 on DB ping failure, 200 when healthy, D-05: Fail-fast config validation, exit(1) on error, D-06: Stub future-phase settings in .env.example now, D-07: Stubbed settings are real Config struct fields with defaults (+4 more)

### Community 55 - "Phase 3 Search Client Code Review"
Cohesion: 0.26
Nodes (12): D-07: one rate.Limiter per client, sequential polling, Phase 3 Code Review Report, Phase 3 Code Review Fix Report, WR-01: unescaped Lucene special characters in MusicBrainz search query, WR-02: deezer.ArtistAlbums lacked pagination unlike musicbrainz ReleaseGroupsByArtist, Phase 3 UAT, Phase 3 Verification Report, deezer.ArtistAlbums (+4 more)

### Community 56 - "Pool MaxConns Sizing"
Cohesion: 0.21
Nodes (12): Phase 11 Plan 05 — Pool MaxConns Sizing Plan, Phase 11 Plan 05 Summary — Pool MaxConns Sizing, PERF-01: Bounded env-configurable worker pool per source, PERF-02: Concurrency preserves rate limiter and overlap guard, PERF-03: Per-artist error isolation under concurrency, PERF-04: Atomic deluxe-change baseline compare-and-set, Phase 11 External API Coverage, Pitfall 4: pgxpool default MaxConns can become the new bottleneck (+4 more)

### Community 57 - "Flaky Test Stability Fixes"
Cohesion: 0.25
Nodes (9): Phase 11 Plan 04 Summary — Suite-Stability Fixes, Folded todo: fix flaky tests under parallel go test, Pitfall 6: flaky-test root cause is a raw schema-drop, not a migrate race, Pitfall 7: notifier flaky tests need clock injection, not DB isolation, scratchSchemaDSN(), TestRunMigrations_AppliesFromScratch(), TestRunMigrations_IsIdempotent(), TestBootToHealth_MigrationsAreIdempotent() (+1 more)

### Community 58 - "Discord Notification Tracer Plans"
Cohesion: 0.24
Nodes (11): Phase 5 Plan 1: Discord Notification Tracer, Phase 5 Plan 1 Summary: Discord Notification Tracer, Phase 5 Plan 2: Complete Embed Rendering, Phase 5 Plan 2 Summary: Complete Embed Rendering, Phase 5 Plan 3: Adversarial Notifier Concurrency Tests, Phase 5 Plan 3 Summary: Adversarial Notifier Concurrency Tests, Phase 5 Context: Discord Notifications, NTFY-01: new_release notification carries title/artist/cover/date/type (+3 more)

### Community 59 - "Frontend package.json"
Cohesion: 0.18
Nodes (10): name, private, scripts, build, dev, format, test, test:watch (+2 more)

### Community 60 - "Events Table Migrations"
Cohesion: 0.20
Nodes (10): events table (migration 000003), events.previous_track_count / events.release_type columns (migration 000004), sqlc.Event (model), groupBaseline, D-09: One combined events table serves as seen store and event log, D-10: Dedup key is a per-event-type external ID, D-11: Nullable notified_at column added now for Phase 5, Pitfall 1: Deluxe-change baseline false positive on first real comparison (+2 more)

### Community 61 - "Empty State & Button UI"
Cohesion: 0.29
Nodes (6): EmptyStateProps, Button(), buttonVariants, emptyStateCopy(), fetchHistoryPage(), History()

### Community 62 - "Toast & App Shell UI"
Cohesion: 0.29
Nodes (4): Toaster(), App(), ErrorBoundary(), renderAppAt()

### Community 63 - "Phase 1 Code Review (DSN Leak)"
Cohesion: 0.31
Nodes (9): OPS-02: Structured JSON logs with request-ID correlation, Phase 1 Code Review Report, CR-01: redactDSN leaks password in libpq keyword/value DSN form, Phase 1 Code Review Fix Report, WR-01: Migration attempt not context-aware, WR-03: No graceful shutdown, Phase 1 Security Contract, Phase 1 UAT Report (+1 more)

### Community 64 - "Loss-Window Signal & PoolConfig"
Cohesion: 0.22
Nodes (9): Phase 11.1 Plan 03 — Detection Loss-Window Signal + PoolConfig Error Differentiation, insertEventFailingQuerier test-only sqlc.Querier decorator, Phase 11.1 Plan 03 Summary, Phase 11 Code Review Report, IN-01: PoolConfig's two parse failures share identical error text, WR-01: detectDeluxeChanges doc undersells InsertEvent-failure blast radius, WR-02: advance-then-insert ordering is a narrow notification-loss window, Phase 11 Security Report (+1 more)

### Community 65 - "Backoff Retry Tests"
Cohesion: 0.33
Nodes (8): retryConfig, TestBackoffDelay_ClampsToMaxDelayOnceExceeded(), TestBackoffDelay_GrowsExponentiallyBeforeSaturating(), TestBackoffDelay_SaturatesRatherThanOverflowsToZero(), TestNewRetryConfig_ClampsNonPositiveMaxAttemptsToOne(), TestNewRetryConfig_PositiveMaxAttemptsUnchanged(), backoffDelay(), newRetryConfig()

### Community 66 - "MusicBrainz Search Query Escaping"
Cohesion: 0.31
Nodes (6): stubMusicBrainzArtistSearcher, clampLimit(), escapeLucene(), Artist, Client, artistSearchResponse

### Community 67 - "CoverArt Reset Fix"
Cohesion: 0.36
Nodes (8): Phase 12 Plan 01: CoverArt Reset Fix, Phase 12 Plan 01 Summary, D-01: CoverArt useEffect reset on src change, D-02: CoverArt regression test, Phase 12 Pattern Map, Phase 12 Research, CoverArt(), CoverArt stale-failure-state bug (WR-02)

### Community 68 - "Phase 12 Popularity Ranking Scope"
Cohesion: 0.32
Nodes (6): CoverArt.tsx image-load-error state never resets on src change, Deezer fan-count capture and popularity sort, MusicBrainz country fallback for disambiguation, Phase 12: CoverArt Reset & Search Popularity Ranking, Search-result popularity ranking / same-name artist disambiguation, Soft-delete/filter event retention (not hard delete)

### Community 69 - "Bounded Polling Worker Config"
Cohesion: 0.29
Nodes (7): D-02: Default pool sizes MB=3, Deezer=5, D-03: MUSICBRAINZ_POLL_WORKERS/DEEZER_POLL_WORKERS naming, Phase 11 Context — Bounded Concurrent Polling, Phase 11 Discussion Log, Phase 11 Pattern Map, Phase 11 Research, Phase 11 Validation Strategy

### Community 70 - "Postgres Port Revert Decision"
Cohesion: 0.50
Nodes (5): D-01: Revert Postgres port 5433 back to 5432, committed, D-02: Preserve full 5433-to-5432 incident history in comments, Phase 11.1 Plan 04 — Boot Test, Coverage Filter, Postgres Port Revert, Phase 11.1 Plan 04 Summary, IN-02: .env.example could not be reviewed — sandboxed by tool permissions

### Community 71 - "Artist Upsert Query"
Cohesion: 0.40
Nodes (3): Artist, Queries, UpsertArtistParams

### Community 72 - "Idempotent Seen-Store Insert"
Cohesion: 0.40
Nodes (5): sqlc.InsertEventParams, D-20: ON CONFLICT DO NOTHING preserves the original snapshot, DTCT-04: Idempotent seen store, never re-notify, Pitfall 2: ON CONFLICT DO NOTHING row-count semantics must be tested, not assumed, InsertEvent (sqlc query)

### Community 73 - "UpdatePreferences No-Op Guard Fix"
Cohesion: 0.80
Nodes (5): handleUpdateWatchlist (internal/httpserver/watchlist.go), Service.UpdatePreferences (internal/watchlist/service.go), JSON trailing-data rejection (WR-02), Debug: watchlist UpdatePreferences no-op guard and JSON trailing data, UpdatePreferences no-op domain-boundary guard (WR-01)

### Community 74 - "v1.1 Milestone Audit"
Cohesion: 0.67
Nodes (3): CoverArt.tsx Stale Image-Load-Error State Bug, v1.1 Milestone Nyquist Compliance (All 5 Phases), Milestone v1.1 Audit Report

### Community 75 - "Phase 5 Closeout & v1.0 Audit"
Cohesion: 1.00
Nodes (3): Close Out Phase 5 Plan, Close Out Phase 5 Summary, v1.0 Milestone Audit Report

## Ambiguous Edges - Review These
- `redactedTarget()` → `Phase 2 Plan 02: Duplicate-Add & Preferences Plan`  [AMBIGUOUS]
  .planning/milestones/v1.0-phases/02-watchlist-core/02-02-PLAN.md · relation: references
- `Phase 3 Plan 1 Summary` → `Phase 3 Plan 4 Summary`  [AMBIGUOUS]
  .planning/milestones/v1.0-phases/03-external-clients-search/03-04-SUMMARY.md · relation: references
- `IN-02: .env.example could not be reviewed — sandboxed by tool permissions` → `D-02: Preserve full 5433-to-5432 incident history in comments`  [AMBIGUOUS]
  .planning/milestones/v1.1-phases/11.1-address-tech-debt-v1-1-cleanup/11.1-04-PLAN.md · relation: conceptually_related_to
- `Phase 12 Plan 02: Deezer Popularity Ranking` → `deezerSource.SearchArtists adapter`  [AMBIGUOUS]
  .planning/phases/12-cleanup-coverart-reset-search-popularity-ranking/12-02-PLAN.md · relation: conceptually_related_to

## Knowledge Gaps
- **252 isolated node(s):** `github.com/danielrpof/drop-tracker`, `Queries`, `Queries`, `AlbumLister`, `Client` (+247 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **54 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **What is the exact relationship between `redactedTarget()` and `Phase 2 Plan 02: Duplicate-Add & Preferences Plan`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **What is the exact relationship between `Phase 3 Plan 1 Summary` and `Phase 3 Plan 4 Summary`?**
  _Edge tagged AMBIGUOUS (relation: references) - confidence is low._
- **What is the exact relationship between `IN-02: .env.example could not be reviewed — sandboxed by tool permissions` and `D-02: Preserve full 5433-to-5432 incident history in comments`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **What is the exact relationship between `Phase 12 Plan 02: Deezer Popularity Ranking` and `deezerSource.SearchArtists adapter`?**
  _Edge tagged AMBIGUOUS (relation: conceptually_related_to) - confidence is low._
- **Why does `Handler()` connect `Release History Frontend Plans` to `Events API Handler Tests`, `HTTP Server & Router Setup`, `Bounded Concurrent Polling Design`?**
  _High betweenness centrality (0.076) - this node is a cross-community bridge._
- **Why does `New()` connect `Events API Handler Tests` to `Cron Poller Core`, `Health & Request-ID Config`, `Server Boot & Graceful Shutdown`, `Release History Frontend Plans`, `Detection Functions & Logging`, `HTTP Server & Router Setup`, `Events Service List`, `Watchlist Core Decisions`, `Bounded Concurrent Polling Design`?**
  _High betweenness centrality (0.071) - this node is a cross-community bridge._
- **Why does `NewPool()` connect `Notifier Hang Fix & Pool` to `Events Store & Queries`, `Pgx Pool & Querier`, `Health & Request-ID Config`, `Pgx Pool Config & DSN`, `Server Boot & Graceful Shutdown`, `DB Pinger & Store Stubs`, `Cross-Cutting Architecture Decisions`, `Pool MaxConns Sizing`?**
  _High betweenness centrality (0.055) - this node is a cross-community bridge._