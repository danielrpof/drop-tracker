package db

// backoff_test.go is in package db (not db_test), like redact_test.go,
// because backoffDelay and newRetryConfig are unexported. It covers T-02-33:
// a caller-supplied WithMaxAttempts large enough that attempt-1 reaches 64
// overflowed the uint64 left shift in the pre-fix delay calculation and
// wrapped to a zero delay, which the "delay > cfg.maxDelay" clamp never
// caught (0 is never greater than a positive maxDelay) -- collapsing
// exponential backoff into a zero-wait retry storm. These tests call
// backoffDelay directly rather than driving RunMigrations through 65+ real
// retries, so the fix is proven in milliseconds rather than minutes.

import (
	"testing"
	"time"
)

func TestBackoffDelay_SaturatesRatherThanOverflowsToZero(t *testing.T) {
	cfg := retryConfig{maxAttempts: 200, baseDelay: 500 * time.Millisecond, maxDelay: 8 * time.Second}

	// attempt-1 = 64 is exactly where the pre-fix uint64 shift overflowed
	// and wrapped to 0. attempt = 1000 is far past it, proving the clamp
	// holds for arbitrarily large caller-supplied attempt counts, not just
	// the boundary value.
	for _, attempt := range []int{65, 100, 1000} {
		got := backoffDelay(cfg, attempt)
		if got <= 0 {
			t.Fatalf("backoffDelay(cfg, %d) = %v, want a positive delay (zero means the retry loop stops waiting between attempts)", attempt, got)
		}
		if got != cfg.maxDelay {
			t.Fatalf("backoffDelay(cfg, %d) = %v, want it saturated at maxDelay %v", attempt, got, cfg.maxDelay)
		}
	}
}

func TestBackoffDelay_GrowsExponentiallyBeforeSaturating(t *testing.T) {
	cfg := retryConfig{maxAttempts: 10, baseDelay: 1 * time.Millisecond, maxDelay: 8 * time.Second}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{attempt: 1, want: 1 * time.Millisecond},
		{attempt: 2, want: 2 * time.Millisecond},
		{attempt: 3, want: 4 * time.Millisecond},
		{attempt: 4, want: 8 * time.Millisecond},
	}
	for _, tt := range tests {
		if got := backoffDelay(cfg, tt.attempt); got != tt.want {
			t.Fatalf("backoffDelay(cfg, %d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestBackoffDelay_ClampsToMaxDelayOnceExceeded(t *testing.T) {
	cfg := retryConfig{maxAttempts: 10, baseDelay: 1 * time.Second, maxDelay: 5 * time.Second}

	// baseDelay * 2^(10-1) = 512s, far past maxDelay.
	if got := backoffDelay(cfg, 10); got != cfg.maxDelay {
		t.Fatalf("backoffDelay(cfg, 10) = %v, want it clamped to maxDelay %v", got, cfg.maxDelay)
	}
}

func TestNewRetryConfig_ClampsNonPositiveMaxAttemptsToOne(t *testing.T) {
	for _, n := range []int{0, -1, -100} {
		cfg := newRetryConfig(WithMaxAttempts(n))
		if cfg.maxAttempts != 1 {
			t.Fatalf("newRetryConfig(WithMaxAttempts(%d)).maxAttempts = %d, want 1 (RunMigrations must always try at least once)", n, cfg.maxAttempts)
		}
	}
}

func TestNewRetryConfig_PositiveMaxAttemptsUnchanged(t *testing.T) {
	cfg := newRetryConfig(WithMaxAttempts(6))
	if cfg.maxAttempts != 6 {
		t.Fatalf("newRetryConfig(WithMaxAttempts(6)).maxAttempts = %d, want 6", cfg.maxAttempts)
	}
}
