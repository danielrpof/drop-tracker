package authgate

// Whitebox tests for the D-11 boot-time weak-passphrase heuristic: the pure
// IsWeakPassphrase function and the exact WARN snippet cmd/server/main.go runs
// right after the logger is constructed. These live in package authgate so
// they can reference knownDefaults directly.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestIsWeakPassphrase(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		weak   bool
		reason string
	}{
		{"empty is not weak (gate disabled)", "", false, ""},
		{"short ascii value", "short-pass", true, "shorter than 16 characters"},
		{"exactly 15 runes is short", strings.Repeat("a", 15), true, "shorter than 16 characters"},
		{"16 runes, not a default, is safe", strings.Repeat("a", 16), false, ""},
		{"long random value is safe", "a-perfectly-fine-long-random-instance-passphrase", false, ""},
		{"known default, lowercase", "instance-passphrase", true, "matches a known default value"},
		{"known default, mixed casing", "InStAnCe-PaSsPhRaSe", true, "matches a known default value"},
		{"known default padded past the length floor with whitespace", "  instance-passphrase          ", true, "matches a known default value"},
		{"16 multi-byte runes is not reported short", strings.Repeat("é", 16), false, ""},
		{"15 multi-byte runes is reported short by rune count", strings.Repeat("é", 15), true, "shorter than 16 characters"},
		{"short distinctive value: reason names length, never the value", "Zq9-improbabl-x", true, "shorter than 16 characters"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, weak := IsWeakPassphrase(tc.in)
			if weak != tc.weak {
				t.Fatalf("weak = %v, want %v", weak, tc.weak)
			}
			if reason != tc.reason {
				t.Fatalf("reason = %q, want %q", reason, tc.reason)
			}
			if tc.in != "" && strings.Contains(reason, tc.in) {
				t.Fatalf("reason %q contains the input value %q", reason, tc.in)
			}
		})
	}
}

// TestWeakPassphraseBootWarn_OneWarnLineNeverContainsValue exercises the exact
// snippet cmd/server/main.go runs: a weak value logs exactly one WARN line and
// the value never appears in the buffer.
func TestWeakPassphraseBootWarn_OneWarnLineNeverContainsValue(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	const weakValue = "  instance-passphrase   " // a denylist match, long enough to pass the length check
	if reason, weak := IsWeakPassphrase(weakValue); weak {
		logger.Warn("INSTANCE_PASSPHRASE looks weak; the instance gate is only as strong as this value", "reason", reason)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("got %d log lines, want exactly 1 WARN:\n%s", len(lines), buf.String())
	}
	var f map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &f); err != nil {
		t.Fatalf("decode WARN line: %v", err)
	}
	if f["level"] != "WARN" {
		t.Fatalf("level = %v, want WARN", f["level"])
	}
	if strings.Contains(buf.String(), "instance-passphrase") {
		t.Fatalf("log buffer contains the passphrase value:\n%s", buf.String())
	}
}

// TestWeakPassphraseBootWarn_EmptyOrStrongLogsNothing: an unset or strong
// passphrase produces no boot WARN.
func TestWeakPassphraseBootWarn_EmptyOrStrongLogsNothing(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	for _, v := range []string{"", "a-strong-random-instance-passphrase-value"} {
		if reason, weak := IsWeakPassphrase(v); weak {
			logger.Warn("weak", "reason", reason)
		}
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no log output for an empty or strong passphrase, got:\n%s", buf.String())
	}
}
