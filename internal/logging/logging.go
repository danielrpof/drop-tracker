// Package logging constructs the process-wide structured logger. It is
// intentionally decoupled from the HTTP router (see internal/httpserver) so
// the same constructor works for HTTP request logging today and for
// out-of-request contexts — such as Phase 3's poll-cycle logging — later,
// without changing any call site.
package logging

import (
	"io"
	"log/slog"
	"os"

	"github.com/danielrpof/drop-tracker/internal/config"
)

// NewWithWriter builds the process logger writing to w. It is the real
// constructor: New delegates to it with os.Stdout so tests can capture log
// output by injecting a different writer without changing any other call
// site.
func NewWithWriter(cfg *config.Config, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	opts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	switch cfg.LogFormat {
	case "text":
		handler = slog.NewTextHandler(w, opts)
	default:
		handler = slog.NewJSONHandler(w, opts)
	}

	return slog.New(handler).With(slog.String("service", "drop-tracker"))
}

// New builds the process logger writing to os.Stdout.
func New(cfg *config.Config) *slog.Logger {
	return NewWithWriter(cfg, os.Stdout)
}

// parseLevel maps a configured level name to a slog.Level, defaulting to
// Info for anything unrecognized rather than failing startup over a log
// verbosity typo.
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
