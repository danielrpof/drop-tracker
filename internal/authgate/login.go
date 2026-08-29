package authgate

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

// maxSessionBodyBytes bounds the POST /session request body before JSON
// decoding (ASVS V5): a login body is a single short passphrase field, so
// 4 KiB is generous, and http.MaxBytesReader turns anything larger into a
// decode error rather than an unbounded read.
const maxSessionBodyBytes = 4096

// loginRequest is the POST /session body shape: exactly one field. A
// passphrase is only ever accepted here, in a POST JSON body -- never from
// the URL, the query string or a GET form (Pitfall 14: httplog logs the URL
// and query, not the body).
type loginRequest struct {
	Passphrase string `json:"passphrase"`
}

// HandleLogin verifies the submitted passphrase in constant time and, on a
// match, mints a brand-new session token -- fresh crypto/rand nonce,
// IssuedAt = now, Expiry = now + sessionWindow -- ignoring any inbound
// cookie entirely (D-17 session rotation, Pitfall 4 fixation). It responds
// 204 with no body and the Set-Cookie. On a mismatch it responds 401 with
// {"error":"invalid passphrase"} and sets no cookie. A malformed or
// oversized body is 400.
//
// The comparison hashes BOTH the submitted value and the configured value
// with SHA-256 and passes the two 32-byte digests to
// subtle.ConstantTimeCompare, so neither the value nor its length is
// timing-observable (Pitfall 3/17).
//
// Per-IP throttling, the fixed response delay, the login-concurrency
// semaphore, the global brute-force counter and the D-13 audit lines arrive
// in plan 14-02; the clean insertion point is the top of this method.
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxSessionBodyBytes)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	submitted := sha256.Sum256([]byte(req.Passphrase))
	if subtle.ConstantTimeCompare(submitted[:], m.passHash[:]) != 1 {
		writeJSONError(w, http.StatusUnauthorized, "invalid passphrase")
		return
	}

	now := time.Now()
	tok := Token{
		IssuedAt: now,
		Expiry:   now.Add(sessionWindow),
		Nonce:    newNonce(),
	}
	setSessionCookie(w, Sign(m.key, tok), int(sessionWindow.Seconds()))
	w.WriteHeader(http.StatusNoContent)
}

// HandleLogout clears the session cookie on the calling browser and returns
// 204. Logout is client-local only (GATE-06, D-10): a cookie already copied
// elsewhere stays valid until its own expiry -- the revoke-all lever is
// rotating INSTANCE_PASSPHRASE. setSessionCookie with MaxAge -1 renders as
// Max-Age=0, which is what net/http emits for a delete.
func (m *Manager) HandleLogout(w http.ResponseWriter, _ *http.Request) {
	setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}
