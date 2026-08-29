package authgate

import "context"

// Alerter is the narrow seam the brute-force detector (plan 14-02) posts
// through, declared here in the consumer so a test can substitute a fake
// with no real HTTP client -- mirroring internal/notifier's Sender/Sink
// seam. This plan only establishes the interface so httpserver.WithAuthGate's
// signature is final; the Discord-backed implementation and SelectAlerter
// land in plan 14-02.
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

// NoOpAlerter returns an Alerter that does nothing. cmd/server/main.go passes
// it into httpserver.WithAuthGate until plan 14-02 replaces it with the
// Discord-backed selector -- so that wiring line is real code now rather than
// a nil placeholder.
func NoOpAlerter() Alerter { return noopAlerter{} }
