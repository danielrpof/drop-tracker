# Phase 02 — API Coverage Declaration

No external API integration: this phase builds drop-tracker's own `/watchlist` CRUD resource over
its own Postgres schema, and calls no third-party service. The MusicBrainz identifier is supplied
by the client in the request body as an opaque string — 02-CONTEXT.md scopes live MusicBrainz and
Deezer search (WLST-01, CLNT-03) and scheduled polling (CLNT-01, CLNT-02) to Phase 3, and no HTTP
client, SDK, or outbound network call is introduced anywhere in phase 02's plans.

Confirmed with the deterministic detector against the phase scope (ROADMAP §"Phase 2: Watchlist
Core" + 02-CONTEXT.md): `{"detected": false, "signals": []}`.
