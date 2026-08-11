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
func New(db Pinger, store watchlist.Store, eventsStore events.Store, sources []SearchSource, logger *slog.Logger) *Server {
	s := &Server{db: db, watchlist: store, events: eventsStore, sources: sources}

	r := chi.NewRouter()
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

	r.Get("/health", s.handleHealth)
	r.Get("/search", s.handleSearch)
	r.Post("/watchlist", s.handleAddWatchlist)
	r.Get("/watchlist", s.handleListWatchlist)
	r.Patch("/watchlist/{id}", s.handleUpdateWatchlist)
	r.Delete("/watchlist/{id}", s.handleRemoveWatchlist)
	r.Get("/events", s.handleListEvents)
	r.NotFound(webassets.Handler().ServeHTTP)

	s.router = r
	return s
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
