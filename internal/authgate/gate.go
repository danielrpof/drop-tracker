package authgate

import (
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Manager bundles everything the gate needs at request time: the
// passphrase-derived HMAC key, the SHA-256 digest of the passphrase for the
// constant-time login comparison, the brute-force Alerter seam, the logger for
// D-13 audit lines, and the plan 14-02 brute-force-defense state -- the per-IP
// limiter map (throttle), the process-wide failed-attempt counter, and the
// login-concurrency semaphore (loginSlots). It never retains the passphrase
// string itself beyond deriving those two digests in NewManager.
type Manager struct {
	key      [32]byte
	passHash [32]byte
	alerter  Alerter
	logger   *slog.Logger

	throttle   *loginThrottle
	counter    *globalCounter
	loginSlots chan struct{}

	sweepTicker *time.Ticker
	sweepDone   chan struct{}
	closeOnce   sync.Once
}

// NewManager derives the signing key and the passphrase digest once and
// returns a Manager ready to authenticate requests and handle /session. A
// nil logger falls back to slog.Default() so the gate can never panic on an
// audit line.
//
// It also builds the per-IP limiter map, the global counter and the
// loginSlots semaphore -- loginSlots is sized from the maxConcurrentLogins var
// at construction, so a test that shrinks it first gets the small buffer --
// and starts the limiter-map sweeper as a goroutine driven by a time.Ticker at
// limiterSweepInterval. Call Close to stop that goroutine.
func NewManager(passphrase string, alerter Alerter, logger *slog.Logger) *Manager {
	if alerter == nil {
		alerter = noopAlerter{}
	}
	if logger == nil {
		logger = slog.Default()
	}
	m := &Manager{
		key:         DeriveKey(passphrase),
		passHash:    sha256.Sum256([]byte(passphrase)),
		alerter:     alerter,
		logger:      logger,
		throttle:    newLoginThrottle(),
		counter:     &globalCounter{},
		loginSlots:  make(chan struct{}, maxConcurrentLogins),
		sweepTicker: time.NewTicker(limiterSweepInterval),
		sweepDone:   make(chan struct{}),
	}
	go m.sweepLoop()
	return m
}

// sweepLoop evicts idle per-IP limiter entries on every tick until Close.
func (m *Manager) sweepLoop() {
	for {
		select {
		case <-m.sweepTicker.C:
			m.throttle.sweep(time.Now())
		case <-m.sweepDone:
			return
		}
	}
}

// Close stops the limiter-map sweeper goroutine. It is idempotent. The process
// exit would reclaim the goroutine anyway; Close exists so the test suite is
// free of goroutines that outlive their server. cmd/server/main.go defers it
// via httpserver.Server.Close.
func (m *Manager) Close() {
	m.closeOnce.Do(func() {
		m.sweepTicker.Stop()
		close(m.sweepDone)
	})
}

// csrfHeaderName and csrfHeaderValue are a byte-for-byte contract with the
// SPA client: web/app/lib/api.ts (plan 14-03) attaches exactly
// "X-Requested-With: drop-tracker" to every non-GET request it makes. Changing
// either literal here without changing the client in the same commit silently
// breaks every state-changing request in the app -- the client keeps sending
// the old header, RequireCSRFHeader stops recognising it, and every write
// 403s. This works as a CSRF control because a cross-site attacker can force a
// form POST but cannot set a custom request header without a CORS preflight,
// and this server sends no CORS headers at all, so the preflight is denied
// (D-15). SameSite=Lax on the session cookie and this header are two
// independent controls that cover each other.
const (
	csrfHeaderName  = "X-Requested-With"
	csrfHeaderValue = "drop-tracker"
)

// instanceGatedHeaderName and instanceGatedHeaderValue are a byte-for-byte
// contract with the SPA client, shaped exactly like the csrfHeaderName /
// csrfHeaderValue block above. web/app/lib/api.ts (plan 14-07) reads
// "X-Instance-Gated" off every response apiFetch handles and, when it carries
// this value, calls authStore.markGateActive(). Changing either literal here
// without changing that client file in the same commit silently stops the
// Log out control from ever appearing on a gated instance -- the client keeps
// looking for the old header, never latches gateActive, and
// {gateActive && <LogoutButton />} in root.tsx never mounts -- with no
// compiler and no runtime error on either side.
//
// Semantics: the marker is set ONLY on responses that PASSED gate.Authenticate
// with a valid session cookie. Its ABSENCE proves nothing -- the exempt routes
// registered outside the protected group (/health, POST/DELETE /session)
// legitimately carry none on a gated instance -- which is why the client latch
// is one-way and never clears. The marker exists only on the gated path:
// Authenticate is registered solely inside httpserver's `gate != nil` branch,
// so an ungated instance emits no "X-Instance-Gated" and D-18's
// ungated-instance rule holds structurally rather than by the mere absence of
// a 401. This is what lets the SPA discover a gated instance from an ordinary
// authenticated 200 -- no 401, no typed login, no boot-time probe route
// (D-16 preserved).
const (
	instanceGatedHeaderName  = "X-Instance-Gated"
	instanceGatedHeaderValue = "1"
)

// hasCSRFHeader reports whether r carries the exact custom header the SPA
// client attaches to every non-GET request. See the csrfHeaderName /
// csrfHeaderValue contract note above.
func hasCSRFHeader(r *http.Request) bool {
	return r.Header.Get(csrfHeaderName) == csrfHeaderValue
}

// RequireCSRFHeader is a gate middleware, shaped like Authenticate. It passes
// straight through for the safe methods -- GET, HEAD and OPTIONS -- and for
// every other method requires csrfHeaderName to be present with
// csrfHeaderValue, rejecting otherwise with 403 and the fixed body
// {"error":"missing required header"} via the same writeJSONError helper the
// rest of the package uses.
//
// It is registered inside httpserver's protected chi Group via a second
// pr.Use, AFTER the Authenticate registration, so a request is authenticated
// before it is judged on the header (an unauthenticated non-GET still gets
// 401, not 403). Both registrations live inside the same gated conditional
// branch, so the inert path installs neither and no request is ever rejected
// with 403 for a missing header (GATE-07).
func (m *Manager) RequireCSRFHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !hasCSRFHeader(r) {
			writeJSONError(w, http.StatusForbidden, "missing required header")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Authenticate is the gate middleware, shaped exactly like httpserver's
// echoRequestID closure. It reads the session cookie by name, verifies it
// against time.Now(), and on any failure writes the fixed body
// {"error":"unauthenticated"} with status 401 and returns WITHOUT calling
// next -- the gate never falls open. On success, when the token is past its
// halfway mark, it re-issues a fresh 30-day cookie (IssuedAt copied
// unchanged, fresh nonce) on the response before calling next (D-06 sliding
// renewal).
//
// This middleware is registered via pr.Use inside httpserver's protected
// chi Group, never as a fifth top-level r.Use (D-05), so it always runs
// after RequestID -> echoRequestID -> httplog -> Recoverer and every
// rejected 401 is logged carrying a request_id.
func (m *Manager) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookieName)
		if err != nil || c.Value == "" {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		now := time.Now()
		tok, needsRenew, ok := Verify(m.key, c.Value, now)
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		// Mark every response that passes the gate so the SPA can discover the
		// instance is gated from an ordinary authenticated 200 (G-14-3). Staged
		// on w here, before next.ServeHTTP, because response headers are
		// flushed on the downstream handler's first write -- the neighbouring
		// setSessionCookie renewal call is the in-file precedent for a
		// middleware writing to w before next. Set only on this proven-valid-
		// cookie path, never on the two 401 returns above.
		w.Header().Set(instanceGatedHeaderName, instanceGatedHeaderValue)
		if needsRenew {
			renewed := Token{
				IssuedAt: tok.IssuedAt, // fixed across renewals (D-06 absolute cap)
				Expiry:   now.Add(sessionWindow),
				Nonce:    newNonce(),
			}
			setSessionCookie(w, Sign(m.key, renewed), int(sessionWindow.Seconds()))
		}
		next.ServeHTTP(w, r)
	})
}

// setSessionCookie writes the session cookie through http.SetCookie -- never
// a hand-built header string -- so SameSite, MaxAge (-1 renders Max-Age=0)
// and escaping are the stdlib's. Path=/, HttpOnly, Secure and SameSite=Lax
// are set unconditionally: Secure stays on even over http://localhost (D-09;
// Chrome and Firefox both send a non-prefixed Secure cookie to localhost).
func setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
}

// writeJSONError writes a fixed, operator-authored JSON body. It never
// embeds raw error text -- the passphrase, its digest and the derived key
// must never reach a response body (Pitfall 6, D-13).
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
