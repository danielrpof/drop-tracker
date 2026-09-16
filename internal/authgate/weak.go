package authgate

import (
	"strings"
	"unicode/utf8"
)

// minPassphraseRunes is the boot-time weak-check length floor (D-11). A
// configured passphrase shorter than this many runes earns a WARN at boot --
// never a refusal to start. It is deliberately a plain heuristic, not a
// policy: 16 is comfortably above "obviously too short" while still well below
// the 24-character random value .env.example recommends.
const minPassphraseRunes = 16

// knownDefaults is the case-insensitive denylist of obvious placeholder values
// that must never guard a real instance. Compared against a lower-cased,
// whitespace-trimmed copy of the configured value (D-11).
var knownDefaults = []string{
	"changeme",
	"change-me",
	"change_me",
	"changeit",
	"password",
	"passphrase",
	"secret",
	"admin",
	"default",
	"drop-tracker",
	"droptracker",
	"letmein",
	"instance-passphrase",
	"replace-me",
	"replaceme",
}

// IsWeakPassphrase reports whether a configured INSTANCE_PASSPHRASE looks weak
// enough to warn an operator about at boot, plus a fixed reason phrase when it
// does. Its posture mirrors the manual validation block in config.Load -- a
// plain function, no side effects, no logging of its own -- with one
// deliberate difference recorded here: it feeds a WARN, never an error. Per
// D-11 the process must never refuse to start over a passphrase-policy edge
// case; fail-closed enforcement and minimum-length-only were both considered
// and rejected.
//
// An empty string is NOT weak: unset means the gate is disabled entirely
// (GATE-07), a configuration state rather than a weak secret.
//
// Length is counted in runes, not bytes, so a short multi-byte value is not
// mis-reported and a 16-rune multi-byte value is not flagged. The returned
// reason is always one of a small set of fixed,
// operator-authored phrases -- it never embeds p or any substring of it, so
// the passphrase cannot reach a log line through this function any more than
// through the audit path (D-11, D-13).
func IsWeakPassphrase(p string) (reason string, weak bool) {
	if p == "" {
		return "", false
	}
	if utf8.RuneCountInString(p) < minPassphraseRunes {
		return "shorter than 16 characters", true
	}
	normalized := strings.ToLower(strings.TrimSpace(p))
	for _, d := range knownDefaults {
		if normalized == d {
			return "matches a known default value", true
		}
	}
	return "", false
}
