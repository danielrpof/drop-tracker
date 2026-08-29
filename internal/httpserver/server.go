// Package httpserver wires the chi router: request-ID correlation, request
// logging, panic recovery, and route registration.
package httpserver

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/authgate"
	"github.com/danielrpof/drop-tracker/internal/events"
	"github.com/danielrpof/drop-tracker/internal/watchlist"
	"github.com/danielrpof/drop-tracker/internal/webassets"
)

// Pinger is the minimal surface Server needs from a database handle.
// *pgxpool.Pool satisfies it. Defining this seam (rather than depending on
// *pgxpool.Pool directly) lets tests exercise the database-down branch with
// a fake that never dials a real database, without changing Server's
// exported signature.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server holds the dependencies the router needs to answer requests.
type Server struct {
	db        Pinger
	watchlist watchlist.Store
	events    events.Store
	sources   []SearchSource
	router    http.Handler
}

// serverConfig collects the optional settings New applies before building
// the router. It is populated by Option closures; a zero serverConfig is the
// v1.2 shape -- no gate, no proxy trust.
type serverConfig struct {
	gatePassphrase    string
	gateAlerter       authgate.Alerter
	trustProxyHeaders bool
}

// Option customises New, mirroring internal/poller's and internal/notifier's
// functional-option shape so every existing 5-argument New call site stays a
// pure additive change (GATE-07 / success criterion 5).
type Option func(*serverConfig)

// WithAuthGate enables the instance passphrase gate (GATE-01..06). When
// passphrase is empty the option is completely inert: New registers the
// seven v1.2 routes flat with no gate middleware, no /session routes and no
// middleware.RealIP, so an unconfigured instance behaves exactly as it did
// before v1.3 (GATE-07).
//
// trustProxyHeaders gates middleware.RealIP per D-14: pass true ONLY when
// the app is reachable exclusively through a reverse proxy that sets
// X-Forwarded-For (the Phase 17 VPS topology, container port unpublished).
// Pass false for local dev, docker-compose, CI and any pre-proxy deploy so a
// spoofed X-Forwarded-For cannot bypass the login throttle or forge an audit
// line. alerter is the brute-force alert seam (plan 14-02 supplies the
// Discord-backed one; nil falls back to a no-op).
func WithAuthGate(passphrase string, trustProxyHeaders bool, alerter authgate.Alerter) Option {
	return func(c *serverConfig) {
		c.gatePassphrase = passphrase
		c.trustProxyHeaders = trustProxyHeaders
		c.gateAlerter = alerter
	}
}

// New builds a Server backed by db, store, eventsStore, sources and logging
// through logger. The chi middleware stack runs in this order:
// middleware.RequestID first so the correlation ID exists in context for
// everything downstream, then echoRequestID so the client can see the same
// ID via the X-Request-Id response header, then httplog.RequestLogger so
// every request/response emits a structured JSON log line carrying that ID
// (via LogExtraAttrs, since httplog's own schema has no built-in
// request-ID field), then middleware.Recoverer so a panic in a handler is
// converted into a 500 instead of crashing the process.
//
// store is a second, separate dependency rather than a widened Pinger --
// widening Pinger's method set would break stubPinger, which today only
// implements Ping (health_test.go). eventsStore mirrors store's own
// narrow-Store-interface shape (Phase 6, HIST-01).
//
// sources is a slice rather than one parameter per source (D-01, D-02) so
// adding a source -- plan 03-02's Deezer -- is an append at the call site
// instead of a signature change across every test.
//
// Every explicit route below is registered before r.NotFound(webassets.
// Handler()): chi matches an explicitly registered route before falling
// through to NotFound, so registering the embedded SPA fallback last
// creates no ordering conflict with the API routes above it
// (06-RESEARCH.md Pattern 3, T-06-05) -- /health, /search, /watchlist and
// /events always reach their own handlers, never the SPA fallback.
//
// opts is variadic so every pre-v1.3 5-argument call site is unchanged
// (GATE-07). When WithAuthGate supplies a non-empty passphrase, the six data
// routes move behind a protected chi Group whose middleware is
// authgate.Manager.Authenticate; /health and the SPA fallback stay on the
// root router in BOTH branches (D-03/D-04). When no passphrase is
// configured the route table is byte-for-byte the v1.2 shape.
func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger, opts ...Option) *Server {
	s := &Server{db: db, watchlist: store, events: eventsStore, sources: sources}

	var cfg serverConfig
	for _, o := range opts {
		o(&cfg)
	}
	var gate *authgate.Manager
	if cfg.gatePassphrase != "" {
		gate = authgate.NewManager(cfg.gatePassphrase, cfg.gateAlerter, logger)
	}

	r := chi.NewRouter()

	// D-14: middleware.RealIP rewrites r.RemoteAddr from X-Forwarded-For /
	// X-Real-IP UNCONDITIONALLY -- it trusts the header with no verification.
	// It is wired ONLY when the gate is enabled AND the operator set
	// TRUST_PROXY_HEADERS=true, which is sound only because the Phase 17 VPS
	// topology never publishes the container port: the app is reachable
	// exclusively through the reverse proxy, so the only party that can set
	// X-Forwarded-For is that proxy. Phase 17's runbook enables this flag
	// together with that topology. When trustProxyHeaders is false (local
	// dev, docker-compose, CI, any pre-proxy deploy) RealIP stays off and the
	// login throttle and audit log key on r.RemoteAddr -- the direct peer,
	// which a client cannot spoof -- so a misconfigured deploy fails safe.
	if gate != nil && cfg.trustProxyHeaders {
		r.Use(middleware.RealIP)
	}

	r.Use(middleware.RequestID)
	r.Use(echoRequestID)
	r.Use(httplog.RequestLogger(logger, &httplog.Options{
		Level:         slog.LevelInfo,
		Schema:        httplog.SchemaECS,
		RecoverPanics: true,
		LogExtraAttrs: func(req *http.Request, _ string, _ int) []slog.Attr {
			if id := middleware.GetReqID(req.Context()); id != "" {
				return []slog.Attr{slog.String("request_id", id)}
			}
			return nil
		},
	}))
	r.Use(middleware.Recoverer)

	// /health is exempt as an exact registered path (D-03): chi matches the
	// literal "/health", never a prefix, so /healthz and /health/details fall
	// through to the SPA fallback and never see the health payload.
	r.Get("/health", s.handleHealth)

	if gate != nil {
		// /session is exempt (registered outside the Group): the login form
		// must be reachable without a session. Throttling + CSRF checks live
		// inside the handlers (plan 14-02 / 14-04).
		r.Post("/session", gate.HandleLogin)
		r.Delete("/session", gate.HandleLogout)
		r.Group(func(pr chi.Router) {
			pr.Use(gate.Authenticate)
			registerDataRoutes(pr, s)
		})
	} else {
		registerDataRoutes(r, s)
	}

	// D-04: the static SPA shell serves publicly in both branches -- an
	// unauthenticated visitor must receive index.html and the hashed assets
	// under /assets/ so the passphrase form can render at all (Pitfall 23).
	r.NotFound(webassets.Handler().ServeHTTP)

	s.router = r
	return s
}

// registerDataRoutes registers the six gated data routes on r. It is called
// on the protected sub-router when the gate is enabled and directly on the
// root router otherwise, so the route set is identical in both cases -- the
// only difference is whether authgate.Manager.Authenticate runs first.
func registerDataRoutes(r chi.Router, s *Server) {
	r.Get("/search", s.handleSearch)
	r.Post("/watchlist", s.handleAddWatchlist)
	r.Get("/watchlist", s.handleListWatchlist)
	r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
	r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
	r.Get("/events", s.handleListEvents)
}

// echoRequestID writes chi's per-request correlation ID (already stamped
// into context by middleware.RequestID) back to the client as the
// X-Request-Id response header, so a caller can correlate their request with
// server-side log lines.
func echoRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(middleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

// Router returns the wired http.Handler.
func (s *Server) Router() http.Handler {
	return s.router
}
