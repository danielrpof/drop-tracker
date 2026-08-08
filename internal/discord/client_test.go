package discord

// This file is package discord (whitebox), mirroring
// internal/musicbrainz/search_test.go's httptest.Server-backed pattern --
// unlike musicbrainz.Client, discord.Client needs no unexported baseURL
// setter, since NewClient already takes the target webhook URL directly.

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSend_Success204(t *testing.T) {
	var reqCount int32
	var gotContentType string
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		gotContentType = r.Header.Get("Content-Type")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	embed := Embed{Title: "New Release", Color: 5763719}
	if err := c.Send(context.Background(), embed); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got := atomic.LoadInt32(&reqCount); got != 1 {
		t.Fatalf("request count = %d, want 1", got)
	}
	if gotContentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", gotContentType)
	}

	var payload webhookPayload
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if len(payload.Embeds) != 1 {
		t.Fatalf("len(payload.Embeds) = %d, want 1 (D-07: one event per message)", len(payload.Embeds))
	}
	if payload.Embeds[0].Title != "New Release" {
		t.Fatalf("Embeds[0].Title = %q, want %q", payload.Embeds[0].Title, "New Release")
	}
}

// TestSend_AllowedMentionsAlwaysSuppressed is the CR-01 regression guard:
// every outbound payload must carry an empty allowed_mentions.parse list so
// a community-editable ArtistName containing @everyone/@here/a role or user
// mention token can never trigger a live Discord ping.
func TestSend_AllowedMentionsAlwaysSuppressed(t *testing.T) {
	var gotBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = body
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	embed := Embed{Title: "New Release", Fields: []EmbedField{{Name: "Artist", Value: "@everyone"}}}
	if err := c.Send(context.Background(), embed); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if !strings.Contains(string(gotBody), `"allowed_mentions":{"parse":[]}`) {
		t.Fatalf("request body = %s, want it to contain \"allowed_mentions\":{\"parse\":[]}", gotBody)
	}

	var payload webhookPayload
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	if payload.AllowedMentions.Parse == nil || len(payload.AllowedMentions.Parse) != 0 {
		t.Fatalf("AllowedMentions.Parse = %v, want empty non-nil slice", payload.AllowedMentions.Parse)
	}
	// Fix belongs at the transport layer, not string-mangling display data:
	// the field value itself is expected to survive unescaped.
	if payload.Embeds[0].Fields[0].Value != "@everyone" {
		t.Fatalf("Embeds[0].Fields[0].Value = %q, want %q (unescaped -- suppression happens via allowed_mentions)", payload.Embeds[0].Fields[0].Value, "@everyone")
	}
}

func TestSend_429ThenSuccess_HonorsRetryAfter(t *testing.T) {
	var reqCount int32
	var firstTime, secondTime time.Time
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqCount, 1)
		if n == 1 {
			firstTime = time.Now()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"message":"rate limited","retry_after":0.05,"global":false}`))
			return
		}
		secondTime = time.Now()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	if err := c.Send(context.Background(), Embed{Title: "x"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if got := atomic.LoadInt32(&reqCount); got != 2 {
		t.Fatalf("request count = %d, want 2 (D-08: exactly one retry)", got)
	}
	if gap := secondTime.Sub(firstTime); gap < 50*time.Millisecond {
		t.Fatalf("gap between requests = %s, want >= 50ms (retry_after not honored)", gap)
	}
}

func TestSend_429Twice_ReturnsErrorAfterSingleRetry(t *testing.T) {
	var reqCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqCount, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"message":"rate limited","retry_after":0.01,"global":false}`))
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	err := c.Send(context.Background(), Embed{Title: "x"})
	if err == nil {
		t.Fatal("Send: expected error after the single retry is exhausted, got nil")
	}
	if got := atomic.LoadInt32(&reqCount); got != 2 {
		t.Fatalf("request count = %d, want 2 (one original + one retry, no third request)", got)
	}
}

// TestSend_429RetryAfterClamped is the WR-04 regression guard: an
// unexpectedly large retry_after value must be clamped to maxRetryAfter
// rather than blocking sendAttempt for an unbounded duration. maxRetryAfter
// is temporarily shrunk so this test stays fast and deterministic instead
// of waiting out a real 30s window.
func TestSend_429RetryAfterClamped(t *testing.T) {
	origMax := maxRetryAfter
	maxRetryAfter = 50 * time.Millisecond
	t.Cleanup(func() { maxRetryAfter = origMax })

	var reqCount int32
	var firstTime, secondTime time.Time
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqCount, 1)
		if n == 1 {
			firstTime = time.Now()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			// Absurdly large retry_after (1 hour) -- must be clamped down
			// to maxRetryAfter, not honored verbatim.
			_, _ = w.Write([]byte(`{"message":"rate limited","retry_after":3600,"global":false}`))
			return
		}
		secondTime = time.Now()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Send(ctx, Embed{Title: "x"}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	gap := secondTime.Sub(firstTime)
	if gap > 2*time.Second {
		t.Fatalf("gap between requests = %s, want it clamped near maxRetryAfter (50ms), not the unclamped 3600s retry_after", gap)
	}
}

func TestSend_UnexpectedStatus_ReturnsErrorNamingOnlyTheCode(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	c := NewClient(ts.URL, ts.Client())
	err := c.Send(context.Background(), Embed{Title: "x"})
	if err == nil {
		t.Fatal("Send: expected error for a 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error = %q, want it to name status 500", err.Error())
	}
}

// TestSend_TransportFailure_ErrorNeverLeaksHostOrToken is the Pitfall-2
// regression guard: httpClient.Do's failure is a *url.Error whose Error()
// embeds the full request URL, and a Discord webhook URL's path IS the
// secret token. sendAttempt must never wrap that raw error.
func TestSend_TransportFailure_ErrorNeverLeaksHostOrToken(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	// Nothing is listening on addr anymore -- the connection is refused
	// immediately rather than timing out, keeping this test fast.

	const token = "super-secret-webhook-token-fixture"
	webhookURL := "http://" + addr + "/api/webhooks/123456789012345678/" + token

	c := NewClient(webhookURL, &http.Client{Timeout: 2 * time.Second})
	err = c.Send(context.Background(), Embed{Title: "x"})
	if err == nil {
		t.Fatal("Send: expected error for an unroutable host")
	}

	msg := err.Error()
	if strings.Contains(msg, token) {
		t.Fatalf("error %q leaks the webhook token", msg)
	}
	if strings.Contains(msg, addr) {
		t.Fatalf("error %q leaks the webhook host:port", msg)
	}
	host, _, splitErr := net.SplitHostPort(addr)
	if splitErr == nil && host != "" && strings.Contains(msg, host) {
		t.Fatalf("error %q leaks the webhook host", msg)
	}
}
