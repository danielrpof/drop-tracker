---
status: resolved
trigger: "when running the discord test notifications the status is left as time=2026-08-11T19:32:32.003-05:00 level=INFO msg=\"skipping notify pass: already in progress\" service=drop-tracker source=musicbrainz cycle_id=musicbrainz-78\ntime=2026-08-11T19:33:02.004-05:00 level=INFO msg=\"skipping notify pass: already in progress\" service=drop-tracker source=musicbrainz cycle_id=musicbrainz-80 . Ive tried restarting the container, setting the discord webhook url again, checking the psql, etc etc. The method request i sent to discord passed and notified correctly, meaning it is letting actual requests through, but these ones just dont pass for some reason. Ive tried a bunch of solutions in the context and history but none have helped"
created: 2026-08-11T19:35:00-05:00
updated: 2026-08-11T20:47:57-05:00
---

## Current Focus

bug_class: "Mandelbug (environment-triggered, non-deterministic onset; deterministic
  consequence once triggered). SBFL not applicable -- no failing test exists, the
  failure is a liveness (hang) failure, not a wrong-value failure."

hypothesis: |
  CONFIRMED BY SOURCE INSPECTION (experiment pending): the DB call path from
  NotifyPending has NO time bound of any kind at any layer, so a single wedged TCP
  socket blocks pgx forever and permanently wedges the notifier's CAS guard.

  Verified directly in the pinned pgx v5.10.0 source in the module cache:
  1. pgconn/config.go:385-394 -- ConnectTimeout / DialFunc timeout are set ONLY if the
     DSN carries connect_timeout=. This project's DSN (Makefile/docker-compose,
     postgres://...?sslmode=disable) does not, so makeDefaultDialer() returns a bare
     &net.Dialer{} and Config.ConnectTimeout stays 0.
  2. pgxpool/pool.go:19-24 -- with pgxpool.New(ctx, dsn) and zero custom Config (see
     internal/db/pool.go), the pool takes every default: MaxConnIdleTime 30m,
     MaxConnLifetime 1h, HealthCheckPeriod 1m, and PingTimeout **0** (no default at
     all, pool.go:98/171-173/247).
  3. pgxpool/pool.go:630-641 -- Acquire pings any connection idle >1s, and because
     pingTimeout==0 it passes the CALLER'S ctx straight through (`pingCtx := ctx`).
  4. pgconn/pgconn.go:344/379/1176+ -- pgx's ONLY cancellation mechanism is
     ctxwatch/DeadlineContextWatcherHandler, which sets a socket deadline only when
     ctx becomes Done. A ctx that is never Done means the socket read carries NO
     deadline whatsoever.
  5. poller.Start does runCtx = context.WithCancel(ctx) over
     signal.NotifyContext(context.Background(), ...) (main.go:82, poller.go:173) --
     never Done until SIGINT/SIGTERM. It is passed unchanged into NotifyPending ->
     ListUnnotified -> pgxpool.Query.

  Therefore: (1)+(2)+(3)+(4)+(5) compose into "a pgx read on a half-open socket blocks
  for the process's lifetime". Because NotifyPending holds the notifying CAS guard for
  its whole duration (the `defer n.notifying.Store(false)` IS correct on every
  *returning* path -- the function simply never returns), the guard is wedged forever
  and every later cycle logs "skipping notify pass: already in progress".

  This is the only hypothesis found that also explains the otherwise-contradictory
  pg_stat_activity evidence: a socket left ESTABLISHED by Docker Desktop's Windows
  userland port-proxy (which keeps ACKing TCP keepalives after the container-side
  connection is gone) means the Go process is blocked reading from a socket that
  Postgres has NO backend for -- exactly "zero app-owned rows in pg_stat_activity while
  the app is mid-query".
test: "Deterministic local reproduction, no user involvement and no Docker needed:
  stand up a net.Listener that Accepts and then never reads or writes (a TCP black
  hole -- the exact shape of a half-open proxied socket), point pgxpool at it, and call
  Query with a background (no-deadline) ctx. Then repeat with the bound applied."
expecting: "Unbounded variant: Query does not return within several seconds (proves the
  infinite block is a property of the current configuration, not of Discord/DNS/the
  webhook). Bounded variant (connect_timeout in the DSN and/or a ctx deadline): returns
  a real error in ~1s. Confirming both halves proves the mechanism AND proves the fix."
next_action: "RESOLVED -- nothing outstanding. Orchestrator investigated
  agent-ab48cbbed90194c80-postgres-1 directly (docker logs/inspect/psql) and confirmed
  it is another agent worktree's ACTIVE session with real accumulated watchlist data
  (traffic as recent as 21:41 UTC same day) -- NOT a stale leftover, so it was
  deliberately left untouched rather than removed. Fix instead: remapped THIS project's
  own docker-compose.yml Postgres publish port from 5432 to 5433 (avoids the collision
  without touching the other workspace). On the first retest after the remap, a SECOND,
  unrelated port collision surfaced on :8080 (\"bind: Only one usage of each socket
  address...\") -- diagnosed via netstat+tasklist as an orphaned server.exe (PID 24936,
  running from this project's own go-build temp dir) left over from an earlier
  make run / Ctrl+C cycle during this same investigation (a known go-run-on-Windows
  quirk: Ctrl+C does not always kill the compiled child process). Killed via taskkill,
  confirmed it was this project's own binary (not another workspace's) before doing so.
  User re-ran `make run` against the remapped port with the code fix in place and
  confirmed via UAT retest: \"yep, it works\" -- Discord delivery now succeeds.
  Final write-up completed: Resolution recorded below, both fixes committed, status
  resolved, session archived to .planning/debug/resolved/."
reasoning_checkpoint:
  hypothesis: |
    NotifyPending blocks forever inside n.q.ListUnnotified(ctx) because NO layer in the
    path bounds the operation in time -- not the caller's context (poller.runCtx is
    derived from signal.NotifyContext and is never Done), not the pool
    (pgxpool.New(ctx, dsn) with zero custom Config leaves PingTimeout=0 and
    ConnectTimeout=0), and not pgx itself (its only cancellation mechanism sets a socket
    deadline when ctx becomes Done, which never happens). A single half-open socket
    therefore parks the goroutine for the process's lifetime, and because NotifyPending
    holds the notifying CAS guard for its whole duration, the guard is never released.
  confirming_evidence:
    - "Direct source read of pgx v5.10.0 in the module cache: ConnectTimeout/dial timeout
       set only when the DSN carries connect_timeout= (pgconn/config.go:385-394); the
       project's DSN does not carry it."
    - "Direct source read: pgxpool PingTimeout has no default (pool.go:98/171-173/247)
       and Acquire's idle>1s health-check ping passes the caller's ctx through unbounded
       (`pingCtx := ctx`, pool.go:630-641)."
    - "Executed experiment: pgxpool against a TCP black hole with the project's current
       (unconfigured) settings did not return after 5s; the same call returned an error
       in 1s once either a connect_timeout or a ctx deadline was added."
    - "The bounded run's error text is 'failed to receive message' -- TCP accepted, no
       Postgres response -- which is the only mechanism found that explains ZERO
       app-owned rows in pg_stat_activity while the app is blocked in a query."
    - "Static read of all production DB call sites (grep for Query/QueryRow/Exec/Acquire
       excluding tests) shows every access goes through sqlc, which always defer
       rows.Close() or Scan -- so pool-exhaustion-by-leaked-connection is ruled out, and
       an unbounded block is the only remaining way to never return without an error."
  falsification_test: |
    If the mechanism were wrong, then bounding the DB operations would NOT prevent the
    wedge: a fake Querier that blocks until ctx.Done() would still make NotifyPending
    hang, and the subsequent pass would still log 'skipping notify pass: already in
    progress'. The regression test asserts the opposite on both counts -- if it fails,
    this hypothesis is refuted.
  fix_rationale: |
    Addresses the root cause (absence of any time bound), not the symptom (the wedged
    guard). Deliberately NOT changing the CAS guard: `defer n.notifying.Store(false)` is
    already correct on every returning path -- releasing it some other way would be a
    symptom patch that left the goroutine leaked and the DB call still unbounded. Two
    layers, because the evidence shows two distinct unbounded call sites:
      (a) internal/db/pool.go -- set ConnectTimeout, PingTimeout and a MaxConnIdleTime
          shorter than a NAT/proxy idle-drop window, so establishment and the
          acquire-time health-check ping can never block forever, for EVERY caller in
          the process.
      (b) internal/notifier/notifier.go -- give each sqlc call its own deadline, so a
          query on an already-acquired connection that dies mid-flight also returns.
          Safe to wrap at the sqlc-call boundary specifically because the generated
          ListUnnotified fully drains and closes its rows before returning (verified in
          events.sql.go:257-291) and MarkNotified is an Exec.
    On timeout the error propagates through NotifyPending's existing error paths, the
    deferred guard release runs, poller logs 'notify pending failed', and the next cycle
    retries -- turning a permanent silent wedge into a logged, self-healing failure.
  blind_spots: |
    - The precise environmental trigger (why the socket dies within 1-2 poll cycles on
      this machine) is inferred, not directly observed; no packet capture or netstat was
      taken. This does not affect the fix: the code defect is that ANY such event is
      unrecoverable, and the fix makes it recoverable regardless of cause.
    - internal/poller's own store.List(ctx) and internal/detection's DB calls share the
      identical unbounded-context exposure and can wedge mbRunning/dzRunning the same
      way. Only partially covered by fix (a). Deliberately left out of this diff to keep
      the blast radius on a verified phase small -- recorded as follow-up.
    - A sqlc.DBTX timeout-decorator was considered as a one-place fix for all callers and
      rejected: Query/QueryRow return pgx.Rows/pgx.Row that are consumed AFTER the call
      returns, so cancelling on return would break row iteration.
  candidate_causes:
    - "config: pgxpool.New(ctx, dsn) with zero custom Config -> PingTimeout 0,
       MaxConnIdleTime 30m (pgxpool defaults)"
    - "config: DSN carries no connect_timeout -> pgx ConnectTimeout 0, bare net.Dialer"
    - "code: poller.runCtx has no deadline and is passed unchanged into every DB call"
    - "environment: Docker Desktop Windows port-proxy leaves the client socket
       ESTABLISHED (and keepalive-answering) after the container-side connection is gone"
    - "code (consequence amplifier, not a cause): NotifyPending holds the shared CAS
       guard for the entire unbounded duration"
  and_gate: |
    YES -- this failure genuinely requires more than one simultaneous condition, which is
    why single-category hypotheses (bad webhook, Postgres lock, proxy env vars, DNS) all
    failed to explain it. The environment condition alone is harmless: a bounded client
    would simply error and retry. The configuration/code condition alone is harmless: a
    healthy socket always answers. Only (unbounded pgx config AND unbounded caller ctx
    AND a half-open socket) produces an infinite block, and only when that block happens
    to land inside the CAS-guarded region does it escalate into the reported permanent
    'already in progress' wedge. root_cause is therefore recorded as a set, not one item.
tdd_checkpoint: ""

## Symptoms

expected: |
  Test 1 of 05-UAT.md ("Live Discord rendering and mention-suppression check"): after
  seeding one pending events row per event_type (new_release, guest_feature,
  deluxe_change) with notified_at NULL, the next scheduled poll cycle's
  poller.RunMusicBrainzCycle/RunDeezerCycle calls notifier.NotifyPending, which should
  drain all 3 rows -- POST one Discord embed per row, mark each notified_at, and return
  -- within one pass (a couple seconds at most, well inside the 30s POLL_INTERVAL used
  for testing).
actual: |
  The very first NotifyPending pass that acquires the atomic.Bool "notifying" guard
  never returns. No embeds are ever delivered to Discord (channel stays empty). Every
  subsequent cron tick (both musicbrainz and deezer cycles, ~every 30s) logs only
  "skipping notify pass: already in progress" and nothing else, indefinitely (observed
  continuously for 19+ minutes in one run before the user gave up and restarted).
errors: |
  None. No "notify send failed", no "mark notified failed after a successful send", no
  panic, no process crash (the process keeps running and logging subsequent cron ticks
  normally -- store.List()-driven poll-cycle logging continues fine, only the notifier
  path is wedged). Temporary debug instrumentation (see Evidence) showed one run where a
  DIFFERENT cycle (musicbrainz-1) completed a full pass instantly with count=0 pending
  events, then the NEXT cycle (deezer-2) logged "about to list unnotified" and produced
  no further output -- i.e. that specific pass hung inside/around
  n.q.ListUnnotified(ctx) itself, not inside Send().
reproduction: |
  1. `make db-up` (docker compose postgres:16, healthy).
  2. In psql (via `docker compose exec postgres psql -U drop_tracker -d drop_tracker`):
     insert one artists row, then 3 events rows (event_type = new_release,
     guest_feature, deluxe_change respectively), all with notified_at left NULL/unset.
  3. Set env vars in the SAME terminal: DATABASE_URL (localhost:5432, matches
     docker-compose.yml), a real/valid DISCORD_WEBHOOK_URL (confirmed valid separately,
     see Evidence), POLL_INTERVAL=30s, HTTP_PORT=8080, LOG_LEVEL=info, LOG_FORMAT=text,
     MUSICBRAINZ_USER_AGENT=... Then `make run` (== `go run ./cmd/server`).
  4. Within 1-2 poll cycles (30-60s), the notify pass hangs and never recovers for the
     lifetime of the process. Reproduced on at least 3 separate fresh restarts,
     including immediately after a `docker compose restart postgres`.
started: |
  First observed during this UAT session (2026-08-11), testing Phase 5's Discord
  notifications for the first time with a real webhook configured. STATE.md already
  flagged this exact manual test ("Live Discord rendering + mention-suppression check")
  as open/human-required (05-VERIFICATION.md Human Verification Required #1) -- this is
  the first attempt at actually running it.

## Eliminated

- hypothesis: DISCORD_WEBHOOK_URL was never set / app silently using notifier.NoOp (D-10)
  reasoning: |
    Ruled out directly -- the server's own startup log never printed
    "discord notifications disabled: DISCORD_WEBHOOK_URL not set" (notifier.go's Select,
    the only place that log line is emitted), across every restart in this
    investigation, meaning cfg.DiscordWebhookURL was consistently non-empty when
    notifier.Select ran.

- hypothesis: The webhook URL/token itself is invalid, causing Send() to fail (not hang)
  reasoning: |
    Partially true at one point (an earlier copy of the URL genuinely returned HTTP 401
    "Invalid Webhook Token" / code 50027 when POSTed manually) but not the actual root
    cause of the hang: after the user re-copied a fresh, confirmed-valid webhook URL
    directly from Discord's webhook settings UI, the exact same indefinite-hang symptom
    reproduced again immediately on the very next `make run`. A bad/rejected token also
    can't explain the symptom mechanically -- Discord responds to a bad token in under a
    second (confirmed both via manual PowerShell Invoke-RestMethod and a standalone Go
    program, see Evidence), which would produce a fast "notify send failed" log line,
    not a silent multi-minute hang.

- hypothesis: Postgres has a stuck/blocking query or lock (idle-in-transaction, deadlock)
    holding the notifier's DB call
  reasoning: |
    Checked `pg_stat_activity` directly while the process was actively hung, twice, on
    two separate hang reproductions (once filtering out idle sessions, once showing ALL
    sessions regardless of state). Both times: zero rows belonging to the app process --
    only the psql session running the diagnostic query itself. No active query, no
    idle-in-transaction session, no lock wait. The app was not holding any Postgres
    backend connection at all during the hang (not even an idle pooled one), which rules
    out a conventional Postgres-side lock/blocking-query explanation.

- hypothesis: A large backlog of pre-existing pending (notified_at IS NULL) events from
    earlier phases (when Discord was disabled) is what's actually being slowly ground
    through
  reasoning: |
    Checked directly: `SELECT count(*) FROM events WHERE notified_at IS NULL` returned
    exactly 3 (only the seeded UAT rows) at the time of the hang. No backlog.

- hypothesis: Corporate/VPN proxy environment variables (HTTP_PROXY/HTTPS_PROXY) are
    causing Go's http.Client to hang connecting to Discord through a broken/unreachable
    proxy
  reasoning: |
    Checked in the exact terminal window running the hung `make run` process: both
    `set HTTP_PROXY` and `set HTTPS_PROXY` returned "Environment variable ... not
    defined". Neither var is set.

- hypothesis: Go's networking stack (DNS/TLS/HTTP) is broadly broken or unusually slow
    to Discord on this machine/Windows environment
  reasoning: |
    Directly refuted: a standalone throwaway Go program (`scratch/main.go`, written and
    run once during this investigation, since deleted -- not part of the repo), using
    the exact same client construction pattern as internal/discord/client.go
    (`&http.Client{Timeout: 10*time.Second}`, plain client.Post), POSTed to the real,
    confirmed-valid webhook URL and got `status: 204` back in `491.7ms`. Discord, DNS,
    TLS, and Go's own HTTP transport all work correctly and fast on this exact machine,
    using the exact same webhook URL the app itself was configured with.

- hypothesis: Docker Desktop's Postgres container itself is unhealthy / needs a restart
  reasoning: |
    `docker compose ps` showed the container `Up ... (healthy)` throughout,
    `docker compose logs postgres` showed no restarts, no errors beyond the user's own
    earlier SQL syntax mistakes (harmless, self-contained). A deliberate
    `docker compose restart postgres`, followed immediately by a fresh `make run`, did
    NOT fix the hang -- it reproduced again immediately on the very next process start
    (first or second poll cycle).

## Evidence

- timestamp: 2026-08-11T19:20:00-05:00 (approx, prior to formal debug session)
  checked: |
    Added temporary slog.Info instrumentation directly inside
    internal/notifier/notifier.go's NotifyPending (before/after ListUnnotified, before
    each Send, after Send returns), rebuilt (`go build ./...` succeeded clean), had the
    user restart `make run` and paste the resulting log output. Change was reverted via
    `git checkout -- internal/notifier/notifier.go` immediately after capturing the
    result below -- it is NOT present in the current working tree.
  found: |
    time=...18:51:32.008 msg="DEBUG: notify pass: about to list unnotified" source=musicbrainz cycle_id=musicbrainz-1
    time=...18:51:32.014 msg="DEBUG: notify pass: listed unnotified" source=musicbrainz cycle_id=musicbrainz-1 count=0
    time=...18:51:32.035 msg="DEBUG: notify pass: about to list unnotified" source=deezer cycle_id=deezer-2
    (no further DEBUG lines ever printed for deezer-2 -- the process was left running
    for several more minutes with no further output before the user moved on)
  implication: |
    musicbrainz-1's pass ran and returned in ~6ms with 0 pending rows (this was BEFORE
    the 3 UAT rows were (re-)confirmed present -- count=0 here is suspicious given a
    `SELECT count(*) FROM events WHERE notified_at IS NULL` run minutes later, in a
    different hang reproduction, did show 3). deezer-2's pass got as far as logging
    "about to list unnotified" and then never logged "listed unnotified" -- meaning
    execution is stuck somewhere at or inside the `n.q.ListUnnotified(ctx)` call
    (sqlc-generated query in internal/db/sqlc, exact generated function not yet
    inspected in this investigation) on that specific pass. This is the most concrete
    localization obtained so far and should be the debugger's starting point --
    specifically worth checking sqlc's generated ListUnnotified/MarkNotified code and
    how internal/db/pool.go's pgxpool is configured (pool size/timeouts) since
    `pgxpool.New` is used with zero custom Config (see internal/db/pool.go) -- default
    MinConns=0, so every acquire after a brief idle period may need to establish a new
    connection.

- timestamp: 2026-08-11T19:19:00-05:00 (approx)
  checked: |
    `SELECT pid, state, wait_event_type, wait_event, query, backend_start, state_change
    FROM pg_stat_activity WHERE datname = 'drop_tracker' ORDER BY backend_start;` run
    live against the Postgres container while the app was in the hung state (via
    `docker compose exec postgres psql`).
  found: |
    Exactly 1 row: the psql session running this very query. No row for the Go app at
    all -- not active, not idle, not idle-in-transaction. Zero app-owned Postgres
    backend connections existed at the moment of the hang.
  implication: |
    The app's pgxpool currently holds NO connection to Postgres during the hang -- not
    even an idle pooled one left over from a prior successful query moments earlier.
    Whatever is blocking is happening entirely client-side (in the Go process, before a
    connection is even established at the Postgres wire-protocol level), OR the hang is
    not DB-related at all and this snapshot simply caught a moment between two different
    (also-stuck) attempts. Either way, standard Postgres-side lock/query diagnostics
    (pg_locks, pg_stat_activity) cannot see this particular hang -- it requires
    process-level introspection (goroutine dump) instead.

- timestamp: 2026-08-11T19:10:00-05:00 (approx)
  checked: |
    Manual PowerShell `Invoke-RestMethod -Uri '<webhook>' -Method Post -ContentType
    'application/json' -Body '{"content":"manual test"}'` against the exact webhook URL
    later confirmed to be the one set as DISCORD_WEBHOOK_URL.
  found: |
    Returned near-instantly with an HTTP error body `{"message": "Invalid Webhook
    Token", "code": 50027}` (this was against a stale/mistyped copy of the URL). After
    the user re-copied a fresh URL directly from Discord's webhook UI ("Copy Webhook
    URL" button) and re-tested with a standalone Go program (next entry), the fresh URL
    returned 204 fine -- but the app's own hang reproduced again anyway with the fresh
    URL in place.
  implication: |
    Confirms Discord's webhook endpoint itself responds fast regardless of outcome
    (success or rejection) -- there is no scenario found so far where hitting this exact
    URL/endpoint is itself slow. Also ruled out "the app never even had a valid webhook
    URL" as the whole story, since the hang persisted after a confirmed-valid URL was in
    place.

- timestamp: 2026-08-11T19:12:00-05:00 (approx)
  checked: |
    Standalone `scratch/main.go` (deleted after use, not committed), built and run via
    `go run ./scratch` in the SAME repo/module, using
    `&http.Client{Timeout: 10*time.Second}` and `client.Post(<webhook>, "application/json", body)`
    -- deliberately mirroring internal/discord/client.go's NewClient default-timeout
    pattern.
  found: "Output: `elapsed: 491.7027ms` / `status: 204`."
  implication: |
    Definitively rules out any machine-wide Go networking/DNS/TLS/proxy problem, and
    rules out the specific webhook URL as currently invalid. The app's own
    internal/discord.Client, using the identical http.Client pattern, should behave
    identically fast IF it is actually reached with the same inputs -- strengthens the
    hypothesis that execution never actually reaches Send() at all on the hung pass (see
    the ListUnnotified localization above), rather than Send() itself hanging.

- timestamp: 2026-08-11T20:05:00-05:00
  checked: |
    Read the pinned dependency source directly out of the Go module cache
    (C:/Users/danie/go/pkg/mod/github.com/jackc/pgx/v5@v5.10.0) rather than relying on
    recollection of pgx's defaults:
      - pgconn/config.go:385-394 and :978-1001
      - pgxpool/pool.go:19-24 (defaults), :98/:171-173/:247 (pingTimeout),
        :262-267 (default ShouldPing), :598-660 (Acquire body)
      - pgconn/pgconn.go:253-258, :344-345, :379-381 (ctxwatch/deadline handling)
  found: |
    - ConnectTimeout and a dialer timeout are applied ONLY when the DSN carries
      connect_timeout=. This project's DSN (docker-compose/Makefile:
      postgres://drop_tracker:drop_tracker@localhost:5432/drop_tracker?sslmode=disable)
      does not, so makeDefaultDialer() returns a bare &net.Dialer{} and
      Config.ConnectTimeout == 0.
    - pgxpool defaults: MaxConns 4(/NumCPU), MinConns 0, MaxConnLifetime 1h,
      MaxConnIdleTime 30m, HealthCheckPeriod 1m -- and PingTimeout has NO default
      constant at all, so it stays 0.
    - Pool.Acquire pings any connection idle >1s (default ShouldPing), and because
      pingTimeout==0 it passes the caller's ctx straight through: `pingCtx := ctx`.
    - pgx's ONLY cancellation mechanism is ctxwatch + DeadlineContextWatcherHandler,
      which sets a socket deadline when ctx becomes Done. A ctx that is never Done means
      the socket read has no deadline at all.
  implication: |
    Every layer between poller.runCtx and the Postgres socket is unbounded in the
    current configuration. internal/db/pool.go's existing 5s bound applies ONLY to the
    one-off startup Ping (pool.go:31-37) -- it does not configure the pool, so every
    acquire, dial, health-check ping and query issued after startup is unbounded.

- timestamp: 2026-08-11T19:55:00-05:00
  checked: |
    `docker compose up -d --wait postgres` (to run the integration suite) failed with
    "Bind for 0.0.0.0:5432 failed: port is already allocated". Followed that thread:
    `docker ps -a`, `netstat -ano | grep :5432`, Get-Process on the listening PIDs,
    `docker inspect` StartedAt on each postgres container, and finally a direct query
    against the container that actually owns the published port.
  found: |
    SEVEN postgres:16 containers exist on this machine, from multiple compose projects:
      drop-tracker-postgres-1              Created (not running)
      agent-a47c49dcb71f81383-postgres-1   Created (not running)
      agent-a7318021850d3befe-postgres-1   Created (not running)
      agent-a31f8fe9cfd337074-postgres-1   Up 9 hours (healthy)   5432/tcp (unpublished)
      agent-ab48cbbed90194c80-postgres-1   Up 10 hours (healthy)  0.0.0.0:5432->5432/tcp
    `agent-ab48cbbed90194c80-postgres-1` has held host port 5432 continuously since
    StartedAt 2026-08-11T14:57:29Z (= 09:57 local), i.e. since roughly ten hours BEFORE
    the 19:32 hang. Listening PIDs: 14904 = com.docker.backend (0.0.0.0:5432 and
    [::]:5432), 22648 = wslrelay.exe ([::1]:5432).

    Querying the container that actually owns the port:
      docker exec agent-ab48cbbed90194c80-postgres-1 psql -U drop_tracker -d drop_tracker
      -> events: total 0, pending 0
      -> artists: 0
      -> watchlist: 0
    The drop_tracker schema EXISTS there (the app's own migrations created it) but the
    database is completely empty.

    A pgx handshake probe (temporary internal/db/zz_probe_test.go, run once then
    deleted) against localhost:5432, 127.0.0.1:5432 and [::1]:5432 connected in
    9-31ms on all three and returned events=0 from all three -- so all loopback spellings
    currently reach this same empty instance, and no black-hole listener is present
    right now.
  implication: |
    SECOND ROOT CAUSE FOUND, and it is the one that explains why no Discord embed was
    ever delivered. `make run` connected to `agent-ab48cbbed90194c80-postgres-1` -- a
    leftover Postgres from an unrelated agent workspace that had already claimed host
    port 5432 -- while `docker compose exec postgres psql` execs into
    `drop-tracker-postgres-1`, a DIFFERENT instance. The three seeded UAT events were
    inserted into the container the app was never talking to.

    This retro-explains, exactly and without strain, the three observations that
    previously looked contradictory:
      - "musicbrainz-1 ... count=0" -- correct: the app's real database has 0 events.
      - "SELECT count(*) FROM events WHERE notified_at IS NULL returned exactly 3" --
        also correct, in the other container.
      - "zero app-owned rows in pg_stat_activity" -- correct: the app held no connection
        to the inspected container, because it was connected to the other one.
      - No "poll result" log lines -- correct: that watchlist has 0 entries.

    It also explains why `docker compose restart postgres` never helped (wrong
    container) and why re-copying the webhook URL never helped (Send is never reached
    when there are zero pending rows).

    It further supplies the missing trigger for the FIRST root cause: repeated
    `make db-up` / `docker compose restart postgres` against a port already owned by
    another project churns Docker Desktop's port-forward (com.docker.backend on IPv4,
    wslrelay on IPv6) underneath a pgxpool that is holding pooled connections through
    it. That is precisely how a socket ends up TCP-ESTABLISHED on the Windows side with
    no live backend behind it -- the half-open state the black-hole experiment below
    reproduces.

- timestamp: 2026-08-11T20:15:00-05:00
  checked: |
    Deterministic local experiment (temporary internal/db/zz_hang_experiment_test.go,
    run once then deleted -- NOT in the working tree). A net.Listener that Accepts TCP
    connections and then never reads or writes them (a TCP black hole -- the exact shape
    of a socket left ESTABLISHED by a userland port-proxy whose backend is gone), with
    pgxpool pointed at it, calling pool.Exec(ctx, "SELECT 1") three ways.
  found: |
    === RUN   TestExperimentUnboundedBlocks/current-config-no-bounds
        STILL BLOCKED after 5s -- unbounded
    === RUN   TestExperimentUnboundedBlocks/with-connect-timeout-in-dsn
        RETURNED after 1s: err=failed to connect to `user=u database=db`:
          127.0.0.1:57558 (127.0.0.1): failed to receive message: timeout:
          context deadline exceeded
    === RUN   TestExperimentUnboundedBlocks/with-ctx-deadline
        RETURNED after 1s: err=context deadline exceeded
  implication: |
    ROOT CAUSE CONFIRMED, and both halves of the fix proven in the same run. Against a
    socket that is TCP-ESTABLISHED but never answers, the project's current pgx
    configuration blocks forever with no error; adding EITHER a connect-level bound OR a
    context deadline converts that permanent silent hang into a fast, ordinary error.

    Note the error text on the bounded run: "failed to receive message" -- the TCP
    connection was ACCEPTED and only the Postgres wire-protocol response never arrived.
    That is precisely the state that produces the previously-contradictory
    pg_stat_activity evidence: the Go process is blocked reading from a socket for which
    Postgres has no backend at all, so `SELECT ... FROM pg_stat_activity` legitimately
    returns zero app-owned rows while the app is mid-query. This was the one observation
    no earlier hypothesis could account for, and it is the signature of Docker Desktop's
    Windows userland port-proxy holding a half-open connection (it keeps ACKing TCP
    keepalives after the container-side connection is gone, so Go's default keepalives
    never detect the break either).

## Resolution

root_cause: |
  TWO independent contributing causes, both confirmed. The AND-gate fired: neither alone
  produces the reported symptom set, which is why every single-cause hypothesis tried
  earlier (bad webhook, Postgres lock, proxy env vars, broken DNS, unhealthy container)
  was eliminated by evidence.

  CAUSE 1 -- environment/data: the app was talking to the wrong Postgres instance.
  `agent-ab48cbbed90194c80-postgres-1`, a leftover container from an unrelated agent
  workspace, has owned host port 5432 since 09:57 local -- roughly ten hours before the
  hang. `make run` therefore connected there (the app's own migrations created the
  schema, so nothing failed), while `docker compose exec postgres psql` inspected
  `drop-tracker-postgres-1`, a different instance. The three seeded UAT events lived in
  the container the app never touched. Verified: the app's actual database holds 0
  events, 0 artists, 0 watchlist rows. This is why no embed was ever delivered, why
  ListUnnotified legitimately returned count=0, and why pg_stat_activity showed zero
  app-owned backends.

  CAUSE 2 -- code: no layer between the poll cycle and the Postgres socket bounded any
  operation in time, so a single half-open socket blocked the process permanently.
  Confirmed against pinned pgx v5.10.0 source and reproduced with a TCP black-hole
  experiment: (a) pgx sets ConnectTimeout only when the DSN carries connect_timeout=,
  which this project's does not; (b) pgxpool.Config supplies a default for every field
  EXCEPT PingTimeout, so the liveness ping pgxpool issues at acquire time for any
  connection idle >1s inherits the caller's context unbounded; (c) pgx's only
  cancellation mechanism sets a socket deadline when ctx becomes Done, and the poll
  cycles run under a context derived from signal.NotifyContext that is not Done until
  shutdown. Because NotifyPending holds the notifying CAS guard for its entire duration,
  one such block wedged the guard for the process's lifetime -- every later cycle logged
  "skipping notify pass: already in progress". The trigger for the half-open socket was
  supplied by Cause 1: repeated `make db-up` / `docker compose restart postgres` against
  a port owned by another project churns Docker Desktop's port-forward
  (com.docker.backend on IPv4, wslrelay on IPv6) underneath a pool holding connections
  through it.

  NOT a cause, explicitly checked and ruled out: the `defer n.notifying.Store(false)`
  guard release is correct on every returning path, and every production DB access goes
  through sqlc with proper defer rows.Close()/Scan, so connection leakage and
  pool exhaustion are both ruled out.

fix: |
  Both causes fixed in the repo. CAUSE 1 was initially going to be handed back to the
  user as an operator-side cleanup ("stop the other container"), and that turned out to
  be the wrong call: investigating agent-ab48cbbed90194c80-postgres-1 directly
  (docker logs / inspect / psql) showed it is another agent worktree's ACTIVE session
  holding real accumulated watchlist data, with traffic as recent as 21:41 UTC the same
  day -- not a stale leftover. Deleting it would have destroyed another workspace's
  data to work around this project's own hardcoded port. So CAUSE 1 is fixed here
  instead, by moving THIS project off the contended port.

  0. docker-compose.yml -- the published Postgres port moves from "5432:5432" to
     "5433:5432", with a comment recording why it must not drift back. Binding the
     default 5432 on a shared dev box is a silent-failure design: compose refuses to
     start, the app connects to whichever unrelated instance already owns the port, the
     app's own migrations create a schema there that looks entirely correct, and every
     subsequent query runs against the wrong database with no error anywhere. Nothing in
     the system reports a problem -- which is exactly why this cost a full session.

     Two collaborators of that change had to move in lockstep or the fix would have
     reintroduced the same trap through a different door:
       - Makefile's TEST_DATABASE_URL default, which still pointed at 5432. Left alone,
         `make test-integration` would have run migrations and written test rows into
         the other workspace's live database -- strictly worse than the original bug.
       - .env.example, which still ships localhost:5432. NOT updated here: this
         environment denies access to .env* paths. It is a one-line change and is
         recorded as the single outstanding follow-up.

  1. internal/db/pool.go -- NewPool now builds the pool from a new exported PoolConfig
     that applies three explicit bounds pgx does not apply on its own:
     ConnConfig.ConnectTimeout = 5s (bounds dial + TLS + Postgres startup handshake;
     left untouched when the operator set connect_timeout in the DSN themselves),
     PingTimeout = 2s (bounds the acquire-time liveness ping that otherwise inherits an
     unbounded context -- the check meant to detect a dead connection was itself what
     hung on one), and MaxConnIdleTime = 1m (pgxpool's 30-minute default outlives every
     common NAT/port-proxy idle window, which is what made half-open sockets the normal
     case rather than a rare one).

  2. internal/notifier/notifier.go -- ListUnnotified and MarkNotified now run under a
     dbOpTimeout (10s) deadline derived from the caller's ctx, via two small helpers.
     Bounded per database call rather than per pass on purpose: a large backlog
     legitimately takes len(events)*spacing to drain, so a whole-pass deadline would
     abort healthy work. Safe to wrap at the sqlc-call boundary specifically because the
     generated ListUnnotified fully drains and closes its rows before returning.

  Net effect: a wedged connection now surfaces as an ordinary error, NotifyPending
  returns, the deferred guard release runs, the poller logs "notify pending failed", and
  the next cycle retries -- a permanent silent wedge becomes a logged, self-healing
  failure.

  Deliberately NOT changed: the CAS guard itself. Releasing it some other way would be a
  symptom patch leaving the goroutine leaked and the DB call still unbounded.

verification: |
  guardrail_verdict: accepted

  signal_1_regression_test_kills_the_bug: PASS (mutation-tested, not merely assumed).
    Mutating listUnnotified back to passing the unbounded ctx makes both new notifier
    tests fail with the exact production symptom:
      "NotifyPending did not return within 5s: it is blocked on an unbounded database
       call, which wedges the notifying guard for the process's lifetime"
    Mutating away `cfg.PingTimeout = pingHealthTimeout` makes
    TestPoolConfig_AppliesExplicitBounds fail ("PingTimeout = 0s, want 2s"). Both
    mutants killed; both reverted.

  signal_2_full_suite: PASS. `go test ./... -short -count=1` green across all 11
    packages with tests.

  signal_3_diff_shape: PASS. Additive -- three bounds, two helpers, one exported
    PoolConfig, two new test files. Nothing deleted, no assertion weakened, no test
    skipped.

  signal_4_build_and_static: PASS. `go build ./...` and `go vet ./...` clean.

  signal_5_bug_returns_on_revert: PASS -- demonstrated by signal 1's mutation, which is
    a strictly stronger form of the revert check.

  signal_6_original_symptom_gone_in_production: PASS. This is the one that actually
    closes the report, because every signal above tests CAUSE 2 in isolation and the
    user-visible failure needed both causes. After the port remap the user re-ran
    `make run` against the project's own Postgres with the code fix in place and
    confirmed 05-UAT.md Test 1: "yep, it works" -- embeds delivered to Discord, and no
    "skipping notify pass: already in progress" wedge.

    Two environmental obstacles surfaced during that retest and were diagnosed rather
    than worked around, since either could have been mistaken for the bug recurring:
      - A second port collision, on :8080 ("bind: Only one usage of each socket
        address..."), traced via netstat+tasklist to an orphaned server.exe (PID 24936)
        running from THIS project's own go-build temp directory -- a leftover from an
        earlier make run / Ctrl+C during this same investigation (on Windows, Ctrl+C on
        `go run` does not reliably kill the compiled child). Ownership was confirmed
        before killing it, specifically so another workspace's process could not be
        killed by mistake.
      - The other workspace's container was left running and untouched throughout.

  signal_7_integration_suite: PASS, and this closes a gap the earlier write-up had to
    leave open. With the project's own Postgres finally reachable on 5433, the full
    DB-backed suite runs green:
      TEST_DATABASE_URL=...localhost:5433... go test ./... -count=1 -p 1
    all 11 packages ok.

    `-p 1` is load-bearing and its absence is a PRE-EXISTING defect this run exposed,
    unrelated to the hang. Without it the suite fails with misleading errors
    (`relation "artists" does not exist`; notifier recording 4 sends instead of 3),
    because Go runs package binaries in parallel while every DB-backed package shares
    one database and internal/db's migration test deliberately
    `DROP SCHEMA public CASCADE` to prove migrations apply from scratch -- that drop
    lands underneath whichever package is mid-test. Verified by direct comparison: same
    command, same database, fails without -p 1 and passes with it. Makefile's
    test-integration target was missing the flag and now carries it, so `make test`
    stops failing for reasons that have nothing to do with the code under test.

  known_gaps (documented technical-debt escapes, both environmental, neither
  code-related):
    - `-race` could not be run: ThreadSanitizer cannot allocate its shadow memory in
      this Windows environment ("ThreadSanitizer failed to allocate 0x4ef0000 bytes ...
      error code: 87"), which reproduces on every package including untouched ones.
    - golangci-lint is not installed locally. `gofmt -l` reports files, but `gofmt -d`
      shows the diffs are pure CRLF line-ending artifacts and they cover untouched files
      too (migrate.go, generated sqlc output, cmd/server/main.go), so this is
      pre-existing repo/environment state, not introduced by this change.
    - RESOLVED (was: "integration tests could not be run against the project's own
      Postgres, because CAUSE 1's port conflict prevents drop-tracker-postgres-1 from
      starting at all"). The port remap fixed exactly that: the container now starts on
      5433 and the full suite runs green -- see signal 7.

  outstanding_followups:
    - .env.example still ships DATABASE_URL=...localhost:5432... and must move to 5433
      to match docker-compose.yml. Not applied: this environment denies access to .env*
      paths. One line, and the only thing left; until it is done, anyone who copies
      .env.example verbatim reproduces CAUSE 1 exactly.
    - internal/poller's store.List(ctx) and internal/detection's DB calls share the
      identical unbounded-context exposure and can wedge mbRunning/dzRunning the same
      way NotifyPending was wedged. Fix (a)'s pool-level bounds cover establishment and
      the acquire-time ping for them, but not a query that dies mid-flight on an
      already-acquired connection. Deliberately out of scope here to keep the diff on a
      verified surface; carried forward from blind_spots.

  oracle_type: derived (contract). The assertions encode NotifyPending's liveness
    contract -- it must return, and it must leave the notifying guard released -- rather
    than an implicit crash oracle. Boundary neighbours included: parent-context
    cancellation must still propagate (must not be swallowed by the new deadline), and
    an operator-supplied connect_timeout in the DSN must not be overridden.

files_changed:
  - docker-compose.yml: CAUSE 1 -- publish Postgres on 5433 instead of the contended 5432, with a comment recording why it must not drift back
  - Makefile: TEST_DATABASE_URL default moved to 5433 in lockstep (otherwise `make test-integration` would write into another workspace's live database); test-integration gains -p 1 to stop the shared-database migration test from dropping the schema underneath parallel packages
  - internal/db/pool.go: added connectTimeout/pingHealthTimeout/maxConnIdleTime bounds and exported PoolConfig; NewPool now uses pgxpool.NewWithConfig
  - internal/notifier/notifier.go: added dbOpTimeout plus listUnnotified/markNotified helpers that bound each database call
  - internal/db/pool_timeout_test.go: new -- pins the three bounds, and proves a black-hole server errors instead of hanging
  - internal/notifier/timeout_test.go: new -- proves NotifyPending returns on an unresponsive database, releases its guard so the next pass runs, and still honours parent cancellation

prevention: |
  why not caught: none -- no gate existed for either class. Both causes are invisible to
  every gate this project runs. `go build`, `go vet` and the unit suite cannot see an
  absent timeout (the code is well-formed and every test used a fast in-memory fake), and
  nothing anywhere asserted which Postgres instance the app was actually talking to --
  connecting to the wrong database succeeded at every layer, because the app's own
  migrations built a correct-looking schema there.

  guard (CAUSE 2): internal/db/pool_timeout_test.go and internal/notifier/timeout_test.go
  now encode the liveness contract directly, and both were mutation-tested rather than
  assumed -- reverting either bound makes them fail with the production symptom.

  guard (CAUSE 1): partial and honestly so. The 5433 remap removes today's collision and
  the comments in docker-compose.yml and Makefile explain why the value must not drift
  back, but no automated check asserts that the app is connected to the intended
  instance. The durable version of that guard is a startup log line naming the resolved
  host:port and database, so "wrong instance" is visible in the first second of a run
  instead of after an hour of debugging -- deliberately not added here (it belongs with
  the config/logging surface, not this diff), and worth carrying into the observability
  work rather than losing.

  process note: the single most expensive move in this session was assuming
  `docker compose exec postgres psql` and `make run` reach the same database. Every
  contradictory piece of evidence -- count=0 pending rows, zero rows in pg_stat_activity,
  restarts changing nothing -- was consistent with that assumption being false, but it
  was never itself checked until late. When observations from two tools disagree, verify
  they are pointed at the same target before theorising about the target's behaviour.
