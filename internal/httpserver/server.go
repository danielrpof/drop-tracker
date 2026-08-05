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
	db     Pinger
	router http.Handler
}

// New builds a Server backed by db and logging through logger. The chi
// middleware stack runs in this order: middleware.RequestID first so the
// correlation ID exists in context for everything downstream, then
// echoRequestID so the client can see the same ID via the X-Request-Id
// response header, then httplog.RequestLogger so every request/response
// emits a structured JSON log line carrying that ID (via LogExtraAttrs,
// since httplog's own schema has no built-in request-ID field), then
// middleware.Recoverer so a panic in a handler is converted into a 500
// instead of crashing the process.
func New(db Pinger, logger *slog.Logger) *Server {
	s := &Server{db: db}

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
