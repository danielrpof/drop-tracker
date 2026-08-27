// Package config parses drop-tracker's runtime configuration from environment
// variables only. There is no configuration file and no dotenv-style loader:
// the process environment is the single source of truth, and a missing or
// invalid required setting must fail startup immediately (D-05).
package config

import (
	"fmt"
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
	NotifyMaxReleaseAgeDays    int     `env:"NOTIFY_MAX_RELEASE_AGE_DAYS" envDefault:"7"`

	// Phase 10 — how many days of events history GET /events shows (DATA-01,
	// D-01/D-04). A plain int day-count, deliberately not the time.Duration
	// shape PollInterval above uses: DATA-01's own requirement text says
	// "defaulting to 90 days," and an integer day-count is the most
	// operator-friendly unit for this specific setting. This is a per-field
	// judgment call, not a project-wide "durations must be time.Duration"
	// rule -- do not generalize it.
	EventRetentionDays int `env:"EVENT_RETENTION_DAYS" envDefault:"90"`

	// Phase 11 — per-source bounded-concurrency pool sizes for the poll
	// cycle worker fan-out (PERF-01, D-01/D-02/D-03). Independent per-source
	// fields, not one shared pool-size setting -- mirrors the codebase's
	// existing fully independent per-source rate limiters
	// (MusicBrainzRateLimitPerSec/DeezerRateLimitPer5s) and cycle-overlap
	// guards (mbRunning/dzRunning), since MusicBrainz's tight 1 req/sec
	// limiter and Deezer's faster ~10 req/sec limiter behave very
	// differently under the same pool size.
	MusicBrainzPollWorkers int `env:"MUSICBRAINZ_POLL_WORKERS" envDefault:"3"`
	DeezerPollWorkers      int `env:"DEEZER_POLL_WORKERS" envDefault:"5"`
}

// Load parses Config from the process environment. On failure it returns
// env.Parse's aggregate error unchanged so the caller can report every
// missing or invalid variable at once, rather than one at a time.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	// caarlos0/env/v11 has no numeric-minimum struct tag, so a non-positive
	// EVENT_RETENTION_DAYS (D-03) is checked manually here, after
	// env.Parse's own aggregate-error return -- placing it after preserves
	// TestLoad_AggregatesAllMissing's existing aggregate-error behavior. A
	// non-positive value must fail fast at boot rather than be silently
	// reinterpreted as "show everything" or "hide everything".
	if cfg.EventRetentionDays <= 0 {
		return nil, fmt.Errorf("EVENT_RETENTION_DAYS must be a positive integer, got %d", cfg.EventRetentionDays)
	}
	// Same rationale as the EVENT_RETENTION_DAYS check above: caarlos0/env/v11
	// has no numeric-minimum struct tag, so a non-positive worker count must
	// be rejected manually here rather than silently sized to zero workers
	// (which would mean the poll cycle never dispatches anything).
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
