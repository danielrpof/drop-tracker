// Package discord is a hand-rolled webhook client -- a thin net/http
// wrapper rather than a generated or community Discord SDK, per CLAUDE.md's
// "hand-rolled clients" constraint (a webhook POST needs nothing a bot SDK's
// gateway/slash-command machinery would add). It mirrors
// internal/musicbrainz/client.go's Client/NewClient shape, with two
// deliberate divergences: no rate limiter (pacing between sends is
// internal/notifier's job, D-07, not this client's), and no wrapping of a
// failed httpClient.Do's error (the webhook URL's own path IS the secret
// token -- see sendAttempt).
package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// defaultTimeout bounds every outbound request via httpClient.Timeout when
// the caller supplies no client, mirroring musicbrainz.defaultTimeout.
const defaultTimeout = 10 * time.Second

// Embed is Discord's webhook embed object -- the fields this project uses.
// Color is a decimal RGB int, never a hex string; Discord ignores a string
// color. Fields carry omitempty except where a zero value is meaningful (no
// field on this project's embeds is ever meaningfully zero).
type Embed struct {
	Title       string       `json:"title,omitempty"`
	Description string       `json:"description,omitempty"`
	URL         string       `json:"url,omitempty"`
	Color       int          `json:"color,omitempty"`
	Fields      []EmbedField `json:"fields,omitempty"`
	Thumbnail   *EmbedImage  `json:"thumbnail,omitempty"`
	Timestamp   string       `json:"timestamp,omitempty"` // RFC3339
}

// EmbedField is one row in an Embed's Fields list.
type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// EmbedImage backs Embed.Thumbnail.
type EmbedImage struct {
	URL string `json:"url"`
}

// webhookPayload is the execute-webhook request body shape: a single-embed
// slice per send, matching D-07's one-event-per-message contract.
type webhookPayload struct {
	Embeds []Embed `json:"embeds"`
}

// retry429Body is Discord's documented 429 JSON body shape.
type retry429Body struct {
	Message    string  `json:"message"`
	RetryAfter float64 `json:"retry_after"` // seconds, may carry ms precision
	Global     bool    `json:"global"`
}

// Client POSTs a single embed at a time to one Discord webhook URL. Unlike
// internal/musicbrainz.Client, it carries no baseURL/userAgent/limiter
// fields -- webhookURL IS the full request target (Discord webhooks have no
// separate resource path to build), and pacing between sends is
// internal/notifier's concern, not this client's.
type Client struct {
	webhookURL string
	httpClient *http.Client
}

// NewClient builds a Client that POSTs to webhookURL. httpClient defaults to
// a client with a defaultTimeout timeout when nil, mirroring
// musicbrainz.NewClient's nil-httpClient default-injection idiom.
func NewClient(webhookURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{webhookURL: webhookURL, httpClient: httpClient}
}

// Send POSTs embed as a single-embed message (D-07: one event per message).
// Discord's execute-webhook route returns 204 No Content on success -- this
// project never passes ?wait=true, so 200 is never the success path here
// (checking for 200 instead would treat every real delivery as a failure
// and re-send the same event forever).
func (c *Client) Send(ctx context.Context, embed Embed) error {
	return c.sendAttempt(ctx, embed, true)
}

// sendAttempt is the single request path for this package -- every outbound
// call, including the one 429 retry, funnels through here (mirrors
// musicbrainz.Client.doRequest's single-funnel pattern, collapsed with the
// one call site since this package only ever does one thing: POST one
// embed). allowRetry gates the 429 retry recursion so a second 429 cannot
// retry again -- D-08 is honor-Retry-After-once, not a retry loop.
func (c *Client) sendAttempt(ctx context.Context, embed Embed, allowRetry bool) error {
	body, err := json.Marshal(webhookPayload{Embeds: []Embed{embed}})
	if err != nil {
		return fmt.Errorf("discord: marshal embed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("discord: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Deliberately NOT wrapping the raw *url.Error here, unlike
		// musicbrainz.Client.doRequest's fmt.Errorf("...: %w", err) -- Go's
		// *url.Error.Error() embeds the full request URL, and a Discord
		// webhook URL's path (/webhooks/{id}/{token}) IS the secret token.
		// Wrapping it would write a live credential into structured logs
		// (05-RESEARCH.md Pitfall 2, T-05-01).
		return fmt.Errorf("discord: send webhook: request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests && allowRetry {
		var rl retry429Body
		_ = json.NewDecoder(resp.Body).Decode(&rl)
		wait := time.Duration(rl.RetryAfter * float64(time.Second))
		if wait <= 0 {
			wait = time.Second
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
		return c.sendAttempt(ctx, embed, false)
	}

	// Never echo the response body -- only the status code (mirrors
	// musicbrainz/search.go's "no body echo" convention, T-03-01/V13).
	return fmt.Errorf("discord: send webhook: unexpected status %d", resp.StatusCode)
}
