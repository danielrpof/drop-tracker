package logging

// Whitebox, same package as the code under test, so parseLevel -- the
// unexported level mapper -- is directly callable: there is no exported
// seam for it, and it carries the D-09 priority fallback branch in this
// package: an unrecognized log-level typo must silently resolve to Info
// rather than failing startup.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/danielrpof/drop-tracker/internal/config"
)

func TestParseLevel_RecognizedNames(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLevel(tc.input)
			if got != tc.want {
				t.Fatalf("parseLevel(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// TestParseLevel_UnrecognizedDefaultsToInfo is the D-09 priority case: a
// typo'd log level must resolve to Info rather than making the process fail
// to start.
func TestParseLevel_UnrecognizedDefaultsToInfo(t *testing.T) {
	got := parseLevel("not-a-real-level")
	if got != slog.LevelInfo {
		t.Fatalf("parseLevel(%q) = %v, want %v (unrecognized values must default to Info)", "not-a-real-level", got, slog.LevelInfo)
	}
}

func TestNewWithWriter_TextFormatRendersKeyValueNotJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{LogLevel: "info", LogFormat: "text"}
	logger := NewWithWriter(cfg, &buf)

	logger.Info("hello from text handler")

	out := buf.String()
	if !strings.Contains(out, "msg=") || !strings.Contains(out, "hello from text handler") {
		t.Fatalf("text handler output = %q, want key-value text rendering", out)
	}

	var asJSON map[string]any
	if err := json.Unmarshal(buf.Bytes(), &asJSON); err == nil {
		t.Fatalf("text handler output %q unexpectedly parsed as JSON -- want key-value text, not JSON", out)
	}
}

func TestNewWithWriter_JSONFormatRendersParsableJSON(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{LogLevel: "info", LogFormat: "json"}
	logger := NewWithWriter(cfg, &buf)

	logger.Info("hello from json handler")

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json handler output %q did not parse as JSON: %v", buf.String(), err)
	}
	if decoded["msg"] != "hello from json handler" {
		t.Fatalf("decoded msg = %v, want %q", decoded["msg"], "hello from json handler")
	}
}

// TestNewWithWriter_ServiceAttributeIsAttached proves every logger built by
// this constructor carries the project's service attribute, regardless of
// format.
func TestNewWithWriter_ServiceAttributeIsAttached(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{LogLevel: "info", LogFormat: "json"}
	logger := NewWithWriter(cfg, &buf)

	logger.Info("checking service attribute")

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if decoded["service"] != "drop-tracker" {
		t.Fatalf("service attribute = %v, want %q", decoded["service"], "drop-tracker")
	}
}

// TestNewWithWriter_LevelFilteringReachesTheHandler proves the parsed level
// actually bounds what the handler emits, rather than being computed and
// discarded: a logger configured at the highest severity must drop a
// lower-severity record entirely.
func TestNewWithWriter_LevelFilteringReachesTheHandler(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{LogLevel: "error", LogFormat: "json"}
	logger := NewWithWriter(cfg, &buf)

	logger.Info("this must be filtered out below error level")

	if buf.Len() != 0 {
		t.Fatalf("buffer = %q, want empty -- an info record must not pass a logger configured at error level", buf.String())
	}
}
