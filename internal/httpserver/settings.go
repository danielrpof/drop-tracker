package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/httplog/v3"

	"github.com/danielrpof/drop-tracker/internal/settings"
)

// SettingsStore is the minimal surface handleGetSettings/handleUpdateSettings
// need from the digest settings resource. *settings.Service satisfies it.
// Mirrors StatusStore/WatchlistCounter: a consumer-declared narrow seam
// rather than depending on settings.Store directly.
type SettingsStore interface {
	Get(ctx context.Context) (settings.Settings, error)
	Update(ctx context.Context, p settings.UpdateParams) (settings.Settings, error)
}

// settingsResponse is the GET/PUT /settings/notifications wire contract
// (DGST-01, DGST-03). digest_last_sent_at is *time.Time so a never-sent
// instance encodes as JSON null, never a zero timestamp.
type settingsResponse struct {
	DigestEnabled    bool       `json:"digest_enabled"`
	DigestCadence    string     `json:"digest_cadence"`
	DigestLastSentAt *time.Time `json:"digest_last_sent_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// updateSettingsRequest is the PUT /settings/notifications request DTO. It
// carries only the two client-settable fields -- digest_last_sent_at and
// updated_at are server-owned and deliberately absent, so an attempt to set
// either is rejected by decodeJSONBody's DisallowUnknownFields.
type updateSettingsRequest struct {
	DigestEnabled bool   `json:"digest_enabled"`
	DigestCadence string `json:"digest_cadence"`
}

func toSettingsResponse(s settings.Settings) settingsResponse {
	return settingsResponse{
		DigestEnabled:    s.DigestEnabled,
		DigestCadence:    string(s.DigestCadence),
		DigestLastSentAt: s.DigestLastSentAt,
		UpdatedAt:        s.UpdatedAt,
	}
}

// handleGetSettings implements GET /settings/notifications (DGST-01,
// DGST-03). Mirrors handleStatus's nil-dependency 503 guard and fixed-body
// error posture -- store errors are logged server-side only, never echoed.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	if s.settingsStore == nil {
		writeError(w, http.StatusServiceUnavailable, "settings not available")
		return
	}

	got, err := s.settingsStore.Get(r.Context())
	if err != nil {
		httplog.SetAttrs(r.Context(), slog.String("settings_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toSettingsResponse(got))
}

// handleUpdateSettings implements PUT /settings/notifications (DGST-01,
// DGST-02). The cadence is resolved through settings.ParseCadence and
// rejected with 400 before any store call -- the middle of three
// independent validation layers, none able to bypass the next (T-20-02).
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	if s.settingsStore == nil {
		writeError(w, http.StatusServiceUnavailable, "settings not available")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAddWatchlistBodyBytes)

	var req updateSettingsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	cadence, err := settings.ParseCadence(req.DigestCadence)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid digest cadence")
		return
	}

	updated, err := s.settingsStore.Update(r.Context(), settings.UpdateParams{
		DigestEnabled: req.DigestEnabled,
		DigestCadence: cadence,
	})
	switch {
	case errors.Is(err, settings.ErrInvalidCadence):
		writeError(w, http.StatusBadRequest, "invalid digest cadence")
		return
	case err != nil:
		httplog.SetAttrs(r.Context(), slog.String("settings_error", err.Error()))
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(toSettingsResponse(updated))
}
