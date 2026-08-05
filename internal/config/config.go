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

	// Phase 3-5 — optional, sane defaults, never `notEmpty`/`required`. Real
	// fields per D-06/D-07: the struct is authoritative for later phases from
	// day one, so Phase 3 starts by reading these rather than introducing them.
	DiscordWebhookURL string        `env:"DISCORD_WEBHOOK_URL"`
	PollInterval      time.Duration `env:"POLL_INTERVAL" envDefault:"15m"`
	// MusicBrainzUserAgent must never default to empty: MusicBrainz throttles
	// requests carrying a missing/default User-Agent, and the failure surfaces
	// as intermittent 503s rather than an auth error (CLAUDE.md).
	MusicBrainzUserAgent       string  `env:"MUSICBRAINZ_USER_AGENT" envDefault:"drop-tracker/0.1.0 (+https://github.com/danielrpof/drop-tracker)"`
	MusicBrainzRateLimitPerSec float64 `env:"MUSICBRAINZ_RATE_LIMIT_PER_SEC" envDefault:"1"`
	DeezerRateLimitPer5s       int     `env:"DEEZER_RATE_LIMIT_PER_5S" envDefault:"50"`
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
