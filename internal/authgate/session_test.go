package authgate_test

// This file proves the stateless HMAC session codec (GATE-02, GATE-03,
// D-01/D-02/D-06): a signed token round-trips, a tampered MAC or a
// wrong-passphrase key is rejected, the 30-day window expires, the 90-day
// absolute cap fires before expiry, and a token past its halfway mark
// reports needsRenew. Verify takes an explicit now so every case is a pure
// assertion with no sleeping.

import (
	"testing"
	"time"

	"github.com/danielrpof/drop-tracker/internal/authgate"
)

func TestDeriveKey_DifferentPassphrasesDifferentKeys(t *testing.T) {
	a := authgate.DeriveKey("correct horse battery staple")
	b := authgate.DeriveKey("correct horse battery staplE")
	if a == b {
		t.Fatal("DeriveKey returned the same key for two different passphrases")
	}
	if a == (authgate.DeriveKey("")) {
		t.Fatal("DeriveKey of a real passphrase collided with DeriveKey(\"\")")
	}
}

func TestSignVerify_RoundTrip(t *testing.T) {
	key := authgate.DeriveKey("s3cret-passphrase-value-1234")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tok := authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)}

	raw := authgate.Sign(key, tok)

	got, needsRenew, ok := authgate.Verify(key, raw, now.Add(time.Hour))
	if !ok {
		t.Fatal("Verify(ok) = false for a freshly signed token one hour after issue")
	}
	if needsRenew {
		t.Fatal("needsRenew = true one hour into a 30-day window")
	}
	if !got.IssuedAt.Equal(tok.IssuedAt) || !got.Expiry.Equal(tok.Expiry) {
		t.Fatalf("round-tripped token = %+v, want IssuedAt=%v Expiry=%v", got, tok.IssuedAt, tok.Expiry)
	}
}

func TestVerify(t *testing.T) {
	const passphrase = "s3cret-passphrase-value-1234"
	key := authgate.DeriveKey(passphrase)
	otherKey := authgate.DeriveKey("a-different-passphrase-9876")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	fresh := authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)}

	cases := []struct {
		name      string
		tok       authgate.Token
		verifyKey [32]byte
		mutate    func(string) string
		at        time.Time
		wantOK    bool
		wantRenew bool
	}{
		{
			name: "round trips one hour after issue",
			tok:  fresh, verifyKey: key, at: now.Add(time.Hour),
			wantOK: true, wantRenew: false,
		},
		{
			name: "renew past the halfway mark",
			tok:  fresh, verifyKey: key, at: now.Add(20 * 24 * time.Hour),
			wantOK: true, wantRenew: true,
		},
		{
			name: "expired past the 30-day window",
			tok:  fresh, verifyKey: key, at: now.Add(31 * 24 * time.Hour),
			wantOK: false, wantRenew: false,
		},
		{
			name: "absolute cap fires even with an unexpired expiry",
			tok: authgate.Token{
				IssuedAt: now.Add(-91 * 24 * time.Hour),
				Expiry:   now.Add(24 * time.Hour),
			},
			verifyKey: key, at: now,
			wantOK: false, wantRenew: false,
		},
		{
			name: "tampered MAC (final two chars replaced)",
			tok:  fresh, verifyKey: key,
			mutate: func(s string) string { return s[:len(s)-2] + "xx" },
			at:     now.Add(time.Hour),
			wantOK: false, wantRenew: false,
		},
		{
			name: "tampered payload (first char of payload flipped)",
			tok:  fresh, verifyKey: key,
			mutate: func(s string) string {
				if s[0] == 'A' {
					return "B" + s[1:]
				}
				return "A" + s[1:]
			},
			at:     now.Add(time.Hour),
			wantOK: false, wantRenew: false,
		},
		{
			name: "key derived from a different passphrase",
			tok:  fresh, verifyKey: otherKey, at: now.Add(time.Hour),
			wantOK: false, wantRenew: false,
		},
		{
			name: "not a token at all (no separator)",
			tok:  fresh, verifyKey: key,
			mutate: func(string) string { return "garbage-without-a-dot" },
			at:     now.Add(time.Hour),
			wantOK: false, wantRenew: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := authgate.Sign(key, tc.tok)
			if tc.mutate != nil {
				raw = tc.mutate(raw)
			}
			got, needsRenew, ok := authgate.Verify(tc.verifyKey, raw, tc.at)
			if ok != tc.wantOK {
				t.Fatalf("Verify(ok) = %v, want %v", ok, tc.wantOK)
			}
			if needsRenew != tc.wantRenew {
				t.Fatalf("Verify(needsRenew) = %v, want %v", needsRenew, tc.wantRenew)
			}
			if !ok && got != (authgate.Token{}) {
				t.Fatalf("Verify returned a non-zero Token %+v on a failure path", got)
			}
		})
	}
}

// TestVerify_RenewalBoundaryPreservesIssuedAt is the guard against Pitfall 5:
// a token past halfway must report needsRenew AND still carry its original
// IssuedAt, so the caller re-signs with IssuedAt unchanged and the 90-day
// cap stays reachable.
func TestVerify_RenewalBoundaryPreservesIssuedAt(t *testing.T) {
	key := authgate.DeriveKey("s3cret-passphrase-value-1234")
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	tok := authgate.Token{IssuedAt: now, Expiry: now.Add(30 * 24 * time.Hour)}

	raw := authgate.Sign(key, tok)
	got, needsRenew, ok := authgate.Verify(key, raw, now.Add(16*24*time.Hour))
	if !ok || !needsRenew {
		t.Fatalf("ok=%v needsRenew=%v, want both true 16 days into a 30-day window", ok, needsRenew)
	}
	if !got.IssuedAt.Equal(now) {
		t.Fatalf("IssuedAt = %v, want it unchanged at %v", got.IssuedAt, now)
	}
}
