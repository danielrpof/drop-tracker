package authgate

import (
	"context"
	"log/slog"

	"github.com/danielrpof/drop-tracker/internal/discord"
)

// Alerter is the narrow seam the brute-force detector posts through, declared
// here in the consumer so a test can substitute a fake with no real HTTP
// client -- mirroring internal/notifier's Sender/Sink seam.
type Alerter interface {
	Alert(ctx context.Context, message string) error
}

// noopAlerter is the inert Alerter: used when DISCORD_WEBHOOK_URL is unset
// (plan 14-02 wires SelectAlerter), so the login handler's alert path is
// always non-nil and never nil-checked -- exactly as notifier.NoOp works.
type noopAlerter struct{}

// Alert on noopAlerter issues no request.
func (noopAlerter) Alert(context.Context, string) error { return nil }

var _ Alerter = noopAlerter{}

// NoOpAlerter returns an Alerter that does nothing. Retained for callers and
// tests that want an explicit inert Alerter without going through SelectAlerter.
func NoOpAlerter() Alerter { return noopAlerter{} }

// discordAlerter posts the D-12 brute-force alert through the same
// internal/discord client, webhook URL and single-embed Send path that
// internal/notifier has used since Phase 05 -- one more message type on an
// already-integrated sink, not a new integration surface (COVERAGE.md).
type discordAlerter struct {
	c *discord.Client
}

// Alert sends a fixed-title embed whose description carries only the fact of
// the observation -- a count and a window. It never carries a submitted value,
// any prefix or suffix of one, or a length (T-14-02-04). It returns whatever
// the client returns; the caller logs the outcome only, never the error text
// (the webhook path is a credential).
func (d discordAlerter) Alert(ctx context.Context, message string) error {
	return d.c.Send(ctx, discord.Embed{
		Title:       "drop-tracker: possible brute-force on the instance gate",
		Description: message,
		Color:       0xE53E3E,
	})
}

var _ Alerter = discordAlerter{}

// SelectAlerter mirrors internal/notifier.Select's disabled-case gate: an
// empty webhookURL logs exactly one Info line and returns the inert no-op
// Alerter; otherwise it returns the Discord-backed Alerter over a fresh
// client for that URL (a nil httpClient self-defaults its timeout). The
// webhook URL is never written to a log line here or on the send path.
func SelectAlerter(webhookURL string, logger *slog.Logger) Alerter {
	if logger == nil {
		logger = slog.Default()
	}
	if webhookURL == "" {
		logger.Info("authgate brute-force alerting disabled: DISCORD_WEBHOOK_URL not set")
		return noopAlerter{}
	}
	return discordAlerter{c: discord.NewClient(webhookURL, nil)}
}
