// Package authgate implements drop-tracker's instance passphrase gate: a
// single shared operator secret (INSTANCE_PASSPHRASE) put in front of the
// data API as a signed, stateless session cookie. Nothing here is a user
// account -- the cookie payload carries only an "authenticated until T"
// claim plus a random nonce (D-02). There is no server-side session store:
// a container restart or redeploy does not log anyone out because the token
// is verified purely from the passphrase-derived key (D-01, D-08).
//
// session.go is the pure crypto core -- DeriveKey, Token, Sign, Verify --
// with no net/http dependency so it is table-testable against an explicit
// clock. gate.go adds the chi middleware, login.go the /session handlers,
// alerter.go the brute-force alert seam.
package authgate

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"strings"
	"time"
)

// sessionCookieName is the literal cookie name, decided by the operator in
// plan 14-01 Task 1 as option-a: the bare name "dt_session", NOT the
// "__Host-dt_session" prefix that locked decision D-09 specified verbatim.
//
// Rationale (recorded in 14-01-SUMMARY.md): every guarantee the __Host-
// prefix would browser-enforce -- Secure, Path=/, no Domain -- this package
// sets explicitly in setSessionCookie and asserts in gate_test.go. Chrome
// rejects __Host-/__Secure- prefixed cookies on an http://localhost origin
// (the scheme is not HTTPS), so an operator testing the enabled gate locally
// in Chrome over plain http would loop on the login form with no error
// shown (14-RESEARCH.md finding A1, Pitfall 1). Production terminates TLS in
// Phase 17 and is unaffected either way. One constant, no scheme-conditional
// branching. This is a deliberate, operator-approved deviation from the
// literal wording of D-09.
const sessionCookieName = "dt_session"

// keyDomainPrefix domain-separates the passphrase before it is hashed into
// the HMAC key (D-01): the key is SHA256(prefix || passphrase), so the same
// passphrase could later derive a differently-scoped key without colliding
// with this one. Still exactly one secret -- no separate SESSION_SECRET, no
// key-version byte.
const keyDomainPrefix = "drop-tracker/authgate/v1\x00"

// Session lifetime bounds (D-06). These are package-level vars ONLY so the
// test binary can shrink them via export_test.go (mirroring
// internal/notifier's dbOpTimeout / spacingWait idiom) -- they are NOT
// configurable: hardcoded per D-07, INSTANCE_PASSPHRASE stays the only new
// secret this phase adds and there is no SESSION_TTL env var. Production
// code never reassigns them.
var (
	// sessionWindow is the cookie Max-Age: a freshly minted or renewed token
	// is valid for 30 days.
	sessionWindow = 30 * 24 * time.Hour
	// renewAfter: once a request arrives with less than this remaining before
	// Expiry (i.e. the token is past its halfway mark), Authenticate
	// re-issues a fresh 30-day cookie with IssuedAt copied unchanged.
	renewAfter = 15 * 24 * time.Hour
	// absoluteCap: a token is rejected once now is at or past
	// IssuedAt+absoluteCap, regardless of Expiry. Because IssuedAt is fixed
	// across sliding renewals (D-02, D-06), an idle-but-still-renewing
	// session dies at 90 days; a fresh passphrase entry (D-17) mints a new
	// IssuedAt and starts a new 90-day cap. The cap is per-authentication,
	// not a lifetime ceiling.
	absoluteCap = 90 * 24 * time.Hour
)

// payloadLen is the fixed wire length of a marshalled Token payload:
// 8 bytes IssuedAt (unix nanos, big-endian) + 8 bytes Expiry + 16 bytes
// Nonce. Fixed-width and order-stable so Sign is deterministic for a given
// Token and Verify can reject a wrong-length payload outright.
const payloadLen = 8 + 8 + 16

// Token is the decoded session claim. IssuedAt is the authentication that
// minted this session and does NOT move on renewal (needed for the D-06
// absolute cap). Nonce is 16 bytes from crypto/rand so every minted token is
// unique even if IssuedAt and Expiry collide (D-17 rotation).
type Token struct {
	IssuedAt time.Time
	Expiry   time.Time
	Nonce    [16]byte
}

// DeriveKey returns the HMAC-SHA256 signing key for passphrase (D-01):
// SHA256(keyDomainPrefix || passphrase). Rotating INSTANCE_PASSPHRASE
// changes this key and therefore invalidates every existing session --
// rotation is revoke-all (D-10).
func DeriveKey(passphrase string) [32]byte {
	return sha256.Sum256([]byte(keyDomainPrefix + passphrase))
}

// newNonce fills a 16-byte nonce from crypto/rand. A failure of the OS CSPRNG
// is not a recoverable condition for a security gate -- minting a token with
// a predictable or zero nonce would be worse than refusing -- so it panics,
// which chi's Recoverer converts to a 500 rather than crashing the process.
func newNonce() [16]byte {
	var n [16]byte
	if _, err := rand.Read(n[:]); err != nil {
		panic("authgate: crypto/rand read failed: " + err.Error())
	}
	return n
}

// marshalPayload renders t to its fixed payloadLen wire form.
func marshalPayload(t Token) []byte {
	b := make([]byte, payloadLen)
	binary.BigEndian.PutUint64(b[0:8], uint64(t.IssuedAt.UnixNano()))
	binary.BigEndian.PutUint64(b[8:16], uint64(t.Expiry.UnixNano()))
	copy(b[16:32], t.Nonce[:])
	return b
}

// unmarshalPayload is marshalPayload's inverse. ok is false for any payload
// that is not exactly payloadLen bytes.
func unmarshalPayload(b []byte) (Token, bool) {
	if len(b) != payloadLen {
		return Token{}, false
	}
	var t Token
	t.IssuedAt = time.Unix(0, int64(binary.BigEndian.Uint64(b[0:8]))).UTC() //nolint:gosec // G115: exact inverse of marshalPayload's uint64(t.IssuedAt.UnixNano()) write; both sides are 64 bits so the two's-complement round-trip is total, not narrowing -- no value is unrepresentable and no truncation is possible. A forged/corrupted payload only reaches here after Verify's constant-time MAC comparison has passed, and the worst case is a nonsense timestamp that Verify then rejects via its absolute-cap and expiry checks. A signed decode is what the fixed-width wire format documented at payloadLen intends.
	t.Expiry = time.Unix(0, int64(binary.BigEndian.Uint64(b[8:16]))).UTC()  //nolint:gosec // G115: exact inverse of marshalPayload's uint64(t.Expiry.UnixNano()) write; both sides are 64 bits so the two's-complement round-trip is total, not narrowing -- no value is unrepresentable and no truncation is possible. A forged/corrupted payload only reaches here after Verify's constant-time MAC comparison has passed, and the worst case is a nonsense timestamp that Verify then rejects via its absolute-cap and expiry checks. A signed decode is what the fixed-width wire format documented at payloadLen intends.
	copy(t.Nonce[:], b[16:32])
	return t, true
}

// Sign returns the cookie value for t signed with key. Wire format:
//
//	base64url(payload) "." base64url(HMAC_SHA256(key, base64url(payload)))
//
// The MAC is computed over the already-encoded payload so verification never
// has to re-marshal a decoded Token to recheck the signature.
func Sign(key [32]byte, t Token) string {
	encPayload := base64.RawURLEncoding.EncodeToString(marshalPayload(t))
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(encPayload))
	encMAC := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encPayload + "." + encMAC
}

// Verify checks raw against key at time now and returns (token, needsRenew,
// ok). Every failure path -- malformed, bad base64, forged MAC, expired,
// past the absolute cap -- returns the zero Token and ok false, never a
// distinguishing error, so a caller cannot probe why a cookie was rejected.
//
// Order of checks matters: the MAC is verified with hmac.Equal (constant
// time) before any field is trusted; the absolute cap (D-06) is checked
// before Expiry so a token cannot outlive its bound by riding sliding
// renewals (Pitfall 5). needsRenew is true once now is past
// Expiry-renewAfter, i.e. the token is past its halfway mark.
func Verify(key [32]byte, raw string, now time.Time) (Token, bool, bool) {
	encPayload, encMAC, found := strings.Cut(raw, ".")
	if !found {
		return Token{}, false, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(encPayload)
	if err != nil {
		return Token{}, false, false
	}
	gotMAC, err := base64.RawURLEncoding.DecodeString(encMAC)
	if err != nil {
		return Token{}, false, false
	}
	mac := hmac.New(sha256.New, key[:])
	mac.Write([]byte(encPayload))
	if !hmac.Equal(gotMAC, mac.Sum(nil)) {
		return Token{}, false, false
	}
	t, ok := unmarshalPayload(payload)
	if !ok {
		return Token{}, false, false
	}
	if !now.Before(t.IssuedAt.Add(absoluteCap)) {
		return Token{}, false, false
	}
	if !now.Before(t.Expiry) {
		return Token{}, false, false
	}
	needsRenew := now.After(t.Expiry.Add(-renewAfter))
	return t, needsRenew, true
}
