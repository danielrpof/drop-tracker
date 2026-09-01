// Package config parses drop-tracker's runtime configuration from environment
// variables only — no config file, no dotenv loader. A missing or invalid
// required setting fails startup immediately (D-05).
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the single source of truth for every setting drop-tracker reads
// (D-06/D-07), grouped by the phase that introduces each field.
type Config struct {
	// Phase 1 — required, must never boot half-configured.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	HTTPPort    int    `env:"HTTP_PORT" envDefault:"8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat   string `env:"LOG_FORMAT" envDefault:"json"`

	// Phase 3-5 — optional, sane defaults, never notEmpty/required (D-06/D-07).
	DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
	PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
	// MusicBrainzUserAgent must never be empty: MusicBrainz throttles a missing
	// User-Agent as intermittent 503s, not an auth error (CLAUDE.md).
	MusicBrainzUserAgent       string  `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)"`
	MusicBrainzRateLimitPerSec float64 `env:"MUSICBRAINZ_RATE_LIMIT_PER_SEC" envDefault:"1"`
	DeezerRateLimitPer5s       int     `env:"DEEZER_RATE_LIMIT_PER_5S" envDefault:"50"`
	NotifyMaxReleaseAgeDays    int     `env:"NOTIFY_MAX_RELEASE_AGE_DAYS" envDefault:"7"`

	// Phase 10 — days of events history GET /events shows (DATA-01, D-01/D-04).
	// Plain int day-count (DATA-01: "defaulting to 90 days"), not time.Duration.
	EventRetentionDays int `env:"EVENT_RETENTION_DAYS" envDefault:"90"`

	// Phase 11 — per-source bounded-concurrency pool sizes for the poll-cycle
	// worker fan-out (PERF-01, D-01/D-02/D-03). Independent per-source, like the
	// existing per-source rate limiters.
	MusicBrainzPollWorkers int `env:"MUSICBRAINZ_POLL_WORKERS" envDefault:"3"`
	DeezerPollWorkers      int `env:"DEEZER_POLL_WORKERS" envDefault:"5"`

	// Phase 14 — instance passphrase gate (GATE-01..07). Both optional, NOT in
	// Load()'s validation: an empty InstancePassphrase fully disables the gate
	// (GATE-07); the weak-passphrase WARN lives in cmd/server/main.go (D-11).
	// TrustProxyHeaders gates chi middleware.RealIP (D-14) — true ONLY behind a
	// reverse proxy that sets X-Forwarded-For, else a spoofed header bypasses the
	// login throttle. Hence two env vars, not the one D-07 targeted.
	InstancePassphrase string `env:"INSTANCE_PASSPHRASE"`
	TrustProxyHeaders  bool   `env:"TRUST_PROXY_HEADERS"`
}

// Load parses Config from the process environment, returning env.Parse's
// aggregate error unchanged so every missing/invalid var is reported at once.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	// caarlos0/env has no numeric-minimum tag, so a non-positive
	// EventRetentionDays (D-03) is rejected here, after env.Parse, and must
	// fail fast rather than mean "show everything" or "hide everything".
	if cfg.EventRetentionDays <= 0 {
		return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be a positive integer, got %d", cfg.EventRetentionDays)
	}
	// Same: a non-positive worker count must fail fast, not silently size to zero.
	if cfg.MusicBrainzPollWorkers <= 0 {
		return nil, fmt.Errorf("MUSICBRAINZ_POLL_WORKERS must be a positive integer, got %d", cfg.MusicBrainzPollWorkers)
	}
	if cfg.DeezerPollWorkers <= 0 {
		return nil, fmt.Errorf("DEEZER_POLL_WORKERS must be a positive integer, got %d", cfg.DeezerPollWorkers)
	}
	if cfg.NotifyMaxReleaseAgeDays < 0 {
		return nil, fmt.Errorf("NOTIFY_MAX_RELEASE_AGE_DAYS must be non-negative, got %d", cfg.NotifyMaxReleaseAgeDays)
	}
	return cfg, nil
}
