package httpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/pollruns"
)

// WatchlistCounter is the minimal surface handleStatus needs for
// watchlist_size. The generated *sqlc.Queries satisfies it directly -- the
// method is named CountWatchlist so no adapter is needed at the composition
// root. Mirrors Pinger: a consumer-declared seam so the handler is testable
// with a fake and no live database.
type WatchlistCounter interface {
	CountWatchlist(ctx context.Context) (int64, error)
}

// StatusStore is the minimal surface handleStatus needs from the poll-run
// history. *pollruns.Store satisfies it. The same instance is wired as the
// poller's RunRecorder at the composition root (RUN-04).
type StatusStore interface {
	Snapshot() map[string]pollruns.SourceSnapshot
}

// StatusDeps carries the status-only dependencies WithStatus installs. The
// schema seam and the expected schema version are deliberately absent: they
// are WithReadiness's to own, so /ready and /status report one
// schema-version concept, not two.
type StatusDeps struct {
	Store        StatusStore
	Counter      WatchlistCounter
	AppVersion   string
	PollInterval time.Duration
}

// statusResponse is the frozen Phase 18 GET /status contract. Phase 19 types
// the SPA API client (web/app/lib/api.ts) against these exact key names; see
// docs/api/status-contract.md. A rename here without a paired update there is
// a broken published contract, not a refactor.
type statusResponse struct {
	PollIntervalSeconds int                     `json:"poll_interval_seconds"`
	WatchlistSize       int64                   `json:"watchlist_size"`
	Instance            statusInstance          `json:"instance"`
	Sources             map[string]statusSource `json:"sources"`
}

type statusInstance struct {
	AppVersion     string `json:"app_version"`
	SchemaApplied  *uint  `json:"schema_applied"`
	SchemaExpected uint   `json:"schema_expected"`
}

type statusSource struct {
	LastRun          *statusRun  `json:"last_run"`
	History          []statusRun `json:"history"`
	LastSkippedAt    *time.Time  `json:"last_skipped_at"`
	ConsecutiveSkips int         `json:"consecutive_skips"`
}

type statusRun struct {
	CycleID        string    `json:"cycle_id"`
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	DurationMS     int64     `json:"duration_ms"`
	ArtistsChecked int       `json:"artists_checked"`
	ArtistsSkipped int       `json:"artists_skipped"`
	ArtistsErrored int       `json:"artists_errored"`
	EventsRecorded int       `json:"events_recorded"`
	Outcome        string    `json:"outcome"`
	Summary        string    `json:"summary"`
}

// handleStatus implements the gated GET /status operator surface (STAT-01,
// STAT-02). The watchlist count failing is a 500 with the shared fixed body,
// mirroring handleListEvents. The schema read failing is deliberately not
// symmetric: it logs and proceeds with a null applied version and a 200,
// because the run history and the counts are this payload's point and /ready
// is the endpoint that turns red on a database blip. No value derived from
// an error reaches the response at any depth.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if s.statusStore == nil || s.watchlistCounter == nil {
		writeError(w, http.StatusServiceUnavailable, "status not available")
		return
	}

	ctx := r.Context()

	count, err := s.watchlistCounter.CountWatchlist(ctx)
	if err != nil {
		httplog.SetAttrs(ctx, slog.String("status_watchlist_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var schemaApplied *uint
	if s.schema != nil {
		if applied, _, serr := s.schema.SchemaVersion(ctx); serr != nil {
			httplog.SetAttrs(ctx, slog.String("status_schema_error", serr.Error()))
		} else {
			a := applied
			schemaApplied = &a
		}
	}

	snap := s.statusStore.Snapshot()
	sources := make(map[string]statusSource, len(snap))
	for name, src := range snap {
		sources[name] = toStatusSource(src)
	}

	resp := statusResponse{
		PollIntervalSeconds: int(s.pollInterval / time.Second),
		WatchlistSize:       count,
		Instance: statusInstance{
			AppVersion:     s.appVersion,
			SchemaApplied:  schemaApplied,
			SchemaExpected: s.expectedSchema,
		},
		Sources: sources,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// toStatusSource maps one source's snapshot onto the frozen wire shape. The
// history slice is allocated zero-length so an empty source encodes as [],
// never null -- Phase 19 maps over it directly.
func toStatusSource(src pollruns.SourceSnapshot) statusSource {
	history := make([]statusRun, 0, len(src.History))
	for _, run := range src.History {
		history = append(history, toStatusRun(run))
	}

	out := statusSource{
		History:          history,
		LastSkippedAt:    src.LastSkippedAt,
		ConsecutiveSkips: src.ConsecutiveSkips,
	}
	if src.LastRun != nil {
		lr := toStatusRun(*src.LastRun)
		out.LastRun = &lr
	}
	return out
}

func toStatusRun(r pollruns.RunResult) statusRun {
	return statusRun{
		CycleID:        r.CycleID,
		StartedAt:      r.StartedAt,
		FinishedAt:     r.FinishedAt,
		DurationMS:     r.DurationMS,
		ArtistsChecked: r.ArtistsChecked,
		ArtistsSkipped: r.ArtistsSkipped,
		ArtistsErrored: r.ArtistsErrored,
		EventsRecorded: r.EventsRecorded,
		Outcome:        r.Outcome,
		Summary:        r.Summary,
	}
}
