package config_test

import (
	"reflect"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
)

// setRequired sets only the one variable Load cannot proceed without,
// leaving every other variable exactly as the ambient test environment has
// it (t.Setenv auto-restores on test end, so no manual cleanup is needed).
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db?sslmode=disable")
}

func TestLoad_Defaults(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.HTTPPort != 8080 {
		t.Errorf("HTTPPort = %d, want 8080", cfg.HTTPPort)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogFormat != "json" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "json")
	}
	if cfg.DiscordWebhookURL != "" {
		t.Errorf("DiscordWebhookURL = %q, want empty string (no default)", cfg.DiscordWebhookURL)
	}
	if cfg.PollInterval.String() != "15m0s" {
		t.Errorf("PollInterval = %v, want 15m0s", cfg.PollInterval)
	}
	if cfg.MusicBrainzUserAgent == "" {
		t.Error("MusicBrainzUserAgent = \"\", must never default to empty (MusicBrainz throttles missing/default UAs)")
	}
	if cfg.MusicBrainzRateLimitPerSec != 1 {
		t.Errorf("MusicBrainzRateLimitPerSec = %v, want 1", cfg.MusicBrainzRateLimitPerSec)
	}
	if cfg.DeezerRateLimitPer5s != 50 {
		t.Errorf("DeezerRateLimitPer5s = %d, want 50", cfg.DeezerRateLimitPer5s)
	}
}

func TestLoad_ExplicitValueEqualToDefault(t *testing.T) {
	setRequired(t)
	unset, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with LOG_LEVEL unset returned unexpected error: %v", err)
	}

	t.Setenv("LOG_LEVEL", "info")
	explicit, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with LOG_LEVEL=info returned unexpected error: %v", err)
	}

	if !reflect.DeepEqual(unset, explicit) {
		t.Errorf("Config with LOG_LEVEL unset (%+v) differs from LOG_LEVEL=info explicit (%+v); "+
			"an explicitly-set value equal to the default must not conflict with the unset path", unset, explicit)
	}
}

func TestLoad_OptionalUnsetIsNotAnError(t *testing.T) {
	setRequired(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() with DISCORD_WEBHOOK_URL unset returned unexpected error: %v", err)
	}
	if cfg.DiscordWebhookURL != "" {
		t.Errorf("DiscordWebhookURL = %q, want empty string", cfg.DiscordWebhookURL)
	}
}
