package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v3"
)

// healthPingTimeout bounds the DB ping issued by GET /health. Without this
// bound, a hung network path to Postgres would hang the health request
// itself and accumulate blocked goroutines under a monitor polling every
// few seconds (D-03/D-04).
const healthPingTimeout = 3 * time.Second

// healthResponse is the exact D-03/D-04 response contract. It carries only
// these two fields; the underlying ping error, if any, is attached to the
// request's structured log entry, never to the response body, so database
// host and credential detail cannot cross the client boundary.
type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
	defer cancel()

	resp := healthResponse{Status: "ok", DB: "up"}
	status := http.StatusOK

	if err := s.db.Ping(ctx); err != nil {
		resp.Status, resp.DB = "degraded", "down"
		status = http.StatusServiceUnavailable
		httplog.SetAttrs(r.Context(), slog.String("db_error", err.Error()))
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
