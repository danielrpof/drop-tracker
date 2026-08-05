// Package config parses drop-tracker's runtime configuration from environment
// variables only. There is no configuration file and no dotenv-style loader:
// the process environment is the single source of truth, and a missing or
// invalid required setting must fail startup immediately (D-05).
package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

// Config is the single source of truth for every setting drop-tracker reads,
// including settings later phases will consume (D-06/D-07). Fields are
// grouped by the phase that introduces them; later phases start reading a
// field here instead of adding one from scratch.
type Config struct {
	// Phase 1 — required, must never boot half-configured.
	DatabaseURL string `env:"DATABASE_URL,notEmpty"`
	HTTPPort    int    `env:"HTTP_PORT" envDefault:"8080"`
	LogLevel    string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat   string `env:"LOG_FORMAT" envDefault:"json"`

	// Stubbed for future phases (D-06/D-07) — optional, sane defaults, never
	// `notEmpty`/`required`. Plan 03 owns the full surface; these exist so the
	// struct is authoritative for later phases from day one.
	DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
	PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
	MusicBrainzUA     string        `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0"`
}

// Load parses Config from the process environment. On failure it returns
// env.Parse's aggregate error unchanged so the caller can report every
// missing or invalid variable at once, rather than one at a time.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
