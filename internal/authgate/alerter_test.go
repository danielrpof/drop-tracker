package authgate

// Whitebox tests for SelectAlerter's disabled-case gate (mirrors
// internal/notifier.Select): the empty-URL branch logs exactly one Info line
// and returns the inert no-op Alerter; a set URL returns the Discord-backed
// implementation. The Discord path is never actually sent here -- constructing
// the client with a fake URL and asserting the concrete type is enough.

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"testing"
)

func TestSelectAlerter_DisabledLogsOneInfoLineAndIsInert(t *testing.T) {
	buf := &syncBuf{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))

	a := SelectAlerter("", logger)
	if err := a.Alert(context.Background(), "ignored"); err != nil {
		t.Fatalf("no-op Alert returned %v, want nil", err)
	}

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("disabled SelectAlerter logged %d lines, want exactly 1:\n%s", len(lines), buf.String())
	}
	var f map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &f); err != nil {
		t.Fatalf("decode log line: %v", err)
	}
	if f["level"] != "INFO" {
		t.Fatalf("disabled-branch log line level = %v, want INFO", f["level"])
	}
	if _, ok := a.(noopAlerter); !ok {
		t.Fatalf("SelectAlerter(\"\") returned %T, want noopAlerter", a)
	}
}

func TestSelectAlerter_EnabledReturnsDiscordBacked(t *testing.T) {
	a := SelectAlerter(
		"https://discord.example/api/webhooks/1/secret-token",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)
	if _, ok := a.(discordAlerter); !ok {
		t.Fatalf("SelectAlerter(url) returned %T, want discordAlerter", a)
	}
}
