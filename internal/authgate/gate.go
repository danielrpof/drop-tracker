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
