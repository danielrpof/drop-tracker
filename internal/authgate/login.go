package authgate

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// maxSessionBodyBytes bounds the POST /session request body before JSON
// decoding (ASVS V5): a login body is a single short passphrase field, so
// 4 KiB is generous, and http.MaxBytesReader turns anything larger into a
// decode error rather than an unbounded read.
const maxSessionBodyBytes = 4096

// alertDispatchTimeout bounds the Discord alert send: Alert runs on its own
// goroutine under this deadline so a slow or hung webhook can never stall a
// login response (D-12, T-14-02-06).
const alertDispatchTimeout = 10 * time.Second

// Brute-force defense + audit tunables (GATE-04, D-12). These are package-level
// vars ONLY so the test binary can shrink them via export_test.go, mirroring
// internal/notifier's dbOpTimeout / spacingWait idiom -- they are NOT
// runtime-configurable (D-07: INSTANCE_PASSPHRASE stays the only knob). The
// exact numbers are engineering judgement per 14-RESEARCH.md assumption A2;
// 14-CONTEXT.md leaves them to the planner. Each is a single greppable literal
// on purpose: an operator who locks themselves out during their own testing
// retunes one line and rebuilds.
var (
	// loginRate + loginBurst: 5 immediate attempts per client address, then
	// one more every 12 seconds (~5/min sustained).
	loginRate  = rate.Every(12 * time.Second)
	loginBurst = 5

	// loginDelayMin + loginDelayJitter: every response that runs the passphrase
	// comparison (204 success / wrong-passphrase 401) is paced by at least
	// loginDelayMin plus up to loginDelayJitter of extra sleep. The
	// 429/503/400 paths are never paced.
	loginDelayMin    = 250 * time.Millisecond
	loginDelayJitter = 750 * time.Millisecond

	// limiterSweepInterval + limiterIdleTTL: the per-IP limiter map is swept
	// every 10 minutes and entries idle beyond 15 minutes are evicted, so an
	// attack cycling source addresses cannot grow it without bound.
	limiterSweepInterval = 10 * time.Minute
	limiterIdleTTL       = 15 * time.Minute

	// maxConcurrentLogins bounds how many goroutines can be parked in the fixed
	// delay at once; excess login requests shed with an immediate 503.
	maxConcurrentLogins = 32

	// globalWindow + globalThreshold + alertCooldown: once globalThreshold
	// failed comparisons occur process-wide inside globalWindow, one Discord
	// alert fires; further failures inside alertCooldown do not re-alert.
	globalWindow    = 5 * time.Minute
	globalThreshold = 20
	alertCooldown   = 15 * time.Minute
)

// loginSleep is time.Sleep behind a seam so a test can substitute a blocking
// implementation (holding a login in-flight for the maxConcurrentLogins shed
// test) without depending on real wall-clock time. Production always uses
// time.Sleep.
var loginSleep = time.Sleep

// loginRequest is the POST /session body shape: exactly one field. A
// passphrase is only ever accepted here, in a POST JSON body -- never from
// the URL, the query string or a GET form (Pitfall 14: httplog logs the URL
// and query, not the body).
type loginRequest struct {
	Passphrase string `json:"passphrase"`
}

// ipLimiter is one client address's token bucket plus the last time it was
// seen, so the sweeper can evict idle entries.
type ipLimiter struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// loginThrottle is the per-IP login rate limiter (GATE-04, Pattern 4): a
// mutex-guarded map of client address -> token bucket, built lazily on first
// sight of an address and swept for idle entries. This is the canonical Go
// limiter-per-client shape -- only the map + eviction is ours, the bucket is
// golang.org/x/time/rate.
type loginThrottle struct {
	mu sync.Mutex
	m  map[string]*ipLimiter
}

func newLoginThrottle() *loginThrottle {
	return &loginThrottle{m: make(map[string]*ipLimiter)}
}

// getLimiter returns ip's token bucket, creating it from the current
// loginRate/loginBurst vars on first sight and refreshing its lastSeen.
func (t *loginThrottle) getLimiter(ip string, now time.Time) *rate.Limiter {
	t.mu.Lock()
	defer t.mu.Unlock()
	e, ok := t.m[ip]
	if !ok {
		e = &ipLimiter{lim: rate.NewLimiter(loginRate, loginBurst)}
		t.m[ip] = e
	}
	e.lastSeen = now
	return e.lim
}

// sweep deletes every entry not seen within limiterIdleTTL of now.
func (t *loginThrottle) sweep(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for ip, e := range t.m {
		if now.Sub(e.lastSeen) > limiterIdleTTL {
			delete(t.m, ip)
		}
	}
}

// size reports the current entry count.
func (t *loginThrottle) size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.m)
}

// globalCounter is the process-wide failed-comparison counter behind the D-12
// brute-force alert. It is deliberately alert-only: it imposes no global
// endpoint lock (research assumption A3) -- the per-IP limiter and the fixed
// delay already bound throughput, and a global lock would lock the legitimate
// operator out during an attack. Only a genuine comparison mismatch feeds it;
// a throttled or malformed request never does, so a throttle storm cannot
// manufacture alert fatigue (T-14-02-10).
type globalCounter struct {
	mu          sync.Mutex
	count       int
	windowStart time.Time
	alertedAt   time.Time
}

// recordFailure records one failed comparison at now and reports whether this
// is the transition that should fire an alert: the count has reached
// globalThreshold within the current globalWindow and at least alertCooldown
// has passed since the last alert (restamping alertedAt on that return).
func (c *globalCounter) recordFailure(now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if now.Sub(c.windowStart) > globalWindow {
		c.count = 0
		c.windowStart = now
	}
	c.count++
	if c.count >= globalThreshold && now.Sub(c.alertedAt) >= alertCooldown {
		c.alertedAt = now
		return true
	}
	return false
}

// clientIP is the throttle key and the audit source address: the host portion
// of r.RemoteAddr, or the raw value when it carries no port. When
// TRUST_PROXY_HEADERS is set, middleware.RealIP has already rewritten
// RemoteAddr from the proxy headers by this point (D-14); otherwise it is the
// direct peer, which is the correct key and cannot be spoofed.
func clientIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// loginDelay sleeps loginDelayMin plus a jitter bounded by loginDelayJitter.
// This is anti-hammer pacing to mask the passphrase comparison, NOT a security
// primitive -- it uses the non-crypto rand family deliberately; crypto/rand is
// reserved for the session nonce (session.go). It is called on exactly the two
// comparison paths (204 success, wrong-passphrase 401) and nowhere else (D-12).
func loginDelay() {
	d := loginDelayMin
	if loginDelayJitter > 0 {
		// #nosec G404 -- the jitter is anti-hammer pacing, not a security
		// primitive; a predictable value here weakens nothing. crypto/rand is
		// reserved for the session nonce (session.go).
		d += time.Duration(rand.Int64N(int64(loginDelayJitter)))
	}
	loginSleep(d)
}

// HandleLogin verifies the submitted passphrase in constant time and, on a
// match, mints a brand-new session token -- fresh crypto/rand nonce,
// IssuedAt = now, Expiry = now + sessionWindow -- ignoring any inbound cookie
// entirely (D-17 session rotation, Pitfall 4 fixation). It responds 204 with
// no body and the Set-Cookie. On a mismatch it responds 401 with
// {"error":"invalid passphrase"} and sets no cookie.
//
// Brute-force defense wraps the comparison (D-12, GATE-04):
//   - a non-blocking acquire on the maxConcurrentLogins semaphore first; a
//     request that would block gets an immediate 503, undelayed, with no
//     limiter token and no counter touch (T-14-02-11);
//   - a per-IP token bucket next; a rejected request gets an immediate,
//     UNDELAYED 429 -- it never reaches the comparison the fixed delay exists
//     to mask, and parking a goroutine on a rejected request amplifies a DoS;
//   - the fixed jittered delay on exactly the two comparison paths (204 / 401);
//   - the process-wide failed-attempt counter, fed ONLY by a genuine mismatch,
//     raising one Discord alert per cooldown when it crosses its threshold.
//
// D-13 audit: exactly one structured slog line per response path -- outcome +
// resolved source address only. No attribute ever carries the submitted value,
// the configured value, either SHA-256 digest, the derived key or the cookie
// (the standing precedent is internal/db/migrate.go's redactError helpers).
func (m *Manager) HandleLogin(w http.ResponseWriter, r *http.Request) {
	// D-15 login-CSRF: reject a POST that lacks the SPA client's custom header
	// before the semaphore acquire, the per-IP limiter, the body read and the
	// comparison. SameSite=Lax alone does not cover this case -- a login POST
	// is exactly what an attacker wants to force. A rejection here consumes no
	// limiter token and never touches the global failed-attempt counter.
	if !hasCSRFHeader(r) {
		m.logger.Warn("authgate login", "outcome", "csrf_blocked", "source_ip", clientIP(r))
		writeJSONError(w, http.StatusForbidden, "missing required header")
		return
	}

	select {
	case m.loginSlots <- struct{}{}:
		defer func() { <-m.loginSlots }()
	default:
		writeJSONError(w, http.StatusServiceUnavailable, "server busy")
		return
	}

	ip := clientIP(r)

	if !m.throttle.getLimiter(ip, time.Now()).Allow() {
		m.logger.Warn("authgate login", "outcome", "throttled", "source_ip", ip)
		writeJSONError(w, http.StatusTooManyRequests, "too many attempts")
		return
	}

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
		loginDelay()
		m.logger.Warn("authgate login", "outcome", "failure", "source_ip", ip)
		if m.counter.recordFailure(time.Now()) {
			m.dispatchAlert()
		}
		writeJSONError(w, http.StatusUnauthorized, "invalid passphrase")
		return
	}

	now := time.Now()
	tok := Token{
		IssuedAt: now,
		Expiry:   now.Add(sessionWindow),
		Nonce:    newNonce(),
	}
	loginDelay()
	setSessionCookie(w, Sign(m.key, tok), int(sessionWindow.Seconds()))
	m.logger.Info("authgate login", "outcome", "success", "source_ip", ip)
	w.WriteHeader(http.StatusNoContent)
}

// dispatchAlert posts the brute-force alert on its own goroutine under a
// bounded timeout so a slow or failing webhook can never stall the login
// response (T-14-02-06). On failure it logs the OUTCOME ONLY: the error
// returned by discord.Client.Send must never be wrapped or formatted into a
// log call in any form -- net/http's transport error text embeds the full
// request URL and a Discord webhook path is the secret token, the same rule
// internal/discord/client.go already enforces at its send site (T-14-02-02).
func (m *Manager) dispatchAlert() {
	msg := fmt.Sprintf(
		"possible brute-force on the instance passphrase gate: the failed-login threshold (%d within %s) was crossed",
		globalThreshold, globalWindow,
	)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), alertDispatchTimeout)
		defer cancel()
		if err := m.alerter.Alert(ctx, msg); err != nil {
			m.logger.Warn("authgate brute-force alert delivery failed")
		}
	}()
}

// HandleLogout clears the session cookie on the calling browser and returns
// 204. Logout is client-local only (GATE-06, D-10): a cookie already copied
// elsewhere stays valid until its own expiry -- the revoke-all lever is
// rotating INSTANCE_PASSPHRASE. setSessionCookie with MaxAge -1 renders as
// Max-Age=0, which is what net/http emits for a delete. It emits one D-13
// audit line carrying the resolved source address.
func (m *Manager) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// D-15: the same custom-header check as HandleLogin, for symmetry -- a
	// forced cross-site DELETE /session would log a victim out, a minor
	// nuisance rather than a real attack, but the check is free here.
	if !hasCSRFHeader(r) {
		m.logger.Warn("authgate logout", "outcome", "csrf_blocked", "source_ip", clientIP(r))
		writeJSONError(w, http.StatusForbidden, "missing required header")
		return
	}
	setSessionCookie(w, "", -1)
	m.logger.Info("authgate logout", "outcome", "logout", "source_ip", clientIP(r))
	w.WriteHeader(http.StatusNoContent)
}
