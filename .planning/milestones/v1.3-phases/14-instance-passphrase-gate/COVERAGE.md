# Phase 14 — External API Coverage

No external API integration: this phase adds no new external API, SDK, or service — every
outbound surface it touches is already integrated, and all crypto is Go stdlib.

## Re-decision on the D-12 brute-force alert (required by the api-coverage capability)

D-12 fires a Discord alert when the global failed-login counter crosses its threshold. That
alert is posted through `internal/discord.Client.Send` with a `discord.Embed` — the **same
client, same webhook URL (`DISCORD_WEBHOOK_URL`), same `Send` method, and same
single-embed payload shape** that `internal/notifier` has used since Phase 05.

Verdict: **not a new external integration surface.** It is one additional message type on an
already-integrated sink. Concretely:

- No new endpoint is called (Discord's execute-webhook route, already exercised).
- No new auth scheme, credential, or env var (the existing `DISCORD_WEBHOOK_URL`).
- No new request/response shape (`discord.Embed` → 204, already handled including the 429
  Retry-After path in `internal/discord/client.go`).
- No new failure mode to enumerate (the unset-webhook case reuses the `notifier.Select` /
  `NoOp` disabled-case idiom as `authgate.SelectAlerter` / `noopAlerter`).

A capability matrix would therefore restate `internal/discord`'s existing coverage rather than
document anything new. The one genuinely new obligation — that the alert path must never
leak the webhook URL or the submitted passphrase into a log line — is carried as threat
`T-14-02-02` in `14-02-PLAN.md` (the plan that owns the brute-force alert wiring, Task 2) and as
two `must_haves.prohibitions` entries in that same plan, not as a coverage row. It is additionally
enforced by an acceptance-criteria grep gate on `internal/authgate/alerter.go` asserting no log call
on the alert path carries the send error.

## Everything else in this phase

| Surface | New? | Note |
|---------|------|------|
| `crypto/hmac`, `crypto/sha256`, `crypto/subtle`, `crypto/rand`, `encoding/base64` | no | Go stdlib |
| `math/rand/v2` | no | Go stdlib (login-delay jitter only, not security-sensitive) |
| `github.com/go-chi/chi/v5` + `chi/middleware.RealIP` | no | already in `go.mod` (v5.3.1) |
| `golang.org/x/time/rate` | no | already in `go.mod` (v0.15.0) |
| `github.com/go-chi/httplog/v3` | no | already in `go.mod` (v3.4.0), config unchanged |
| `internal/discord` | no | in-repo, integrated Phase 05 (see above) |
| React Router v7 / Vitest / RTL | no | in `web/package.json` since Phase 06/08 |

Nothing is installed by this phase (`14-RESEARCH.md` §Standard Stack: "Installation: Nothing").
The Package Legitimacy Gate is not applicable — see `14-RESEARCH.md` §Package Legitimacy Audit.
