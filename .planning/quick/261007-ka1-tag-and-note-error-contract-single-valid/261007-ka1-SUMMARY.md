---
phase: quick/261007-ka1
plan: 01
subsystem: tags-notes-error-contract
tags: [go, httpserver, tags, watchlist, react, error-codes, limits]
requires: []
provides:
  - "Coded error envelope ({error, code}) for every tag and note domain error"
  - "Single limit constants in Go (tags.MaxNameRunes, tags.MaxTagsPerArtist, watchlist.MaxNoteRunes) and web (lib/limits.ts)"
  - "Code-point length checks in the SPA matching the server"
affects: [internal/httpserver, internal/tags, internal/watchlist, web/app/lib, web/app/components/watchlist, internal/webassets]
tech-stack:
  added: []
  patterns:
    - "One domainError table + writeDomainError per handler file, body text = matched sentinel's Error()"
    - "SPA branches on ApiError.code, never message text"
key-files:
  created:
    - web/app/lib/limits.ts
    - web/app/lib/limits.test.ts
  modified:
    - internal/tags/normalize.go
    - internal/tags/service.go
    - internal/watchlist/service.go
    - internal/httpserver/tags.go
    - internal/httpserver/watchlist.go
    - web/app/lib/api.ts
    - web/app/lib/tags.ts
    - web/app/components/watchlist/TagChips.tsx
    - web/app/components/watchlist/TagCombobox.tsx
    - web/app/components/watchlist/ManageTagsDialog.tsx
    - web/app/components/watchlist/ArtistNote.tsx
    - internal/webassets/build/client/**
decisions:
  - "UpdateNote's watchlist_note_not_blank branch dropped (unreachable: NormalizeNote maps blank to nil); a bypass-only violation now falls through to the wrapped 500 instead of the wrong-meaning ErrNoteInvalid"
  - "noteLength counts code points of the trimmed value WITHOUT NFC, mirroring watchlist.NormalizeNote; tagNameLength applies NFC + whitespace collapse, mirroring tags.NormalizeName (refines the brief's single NFC helper, which would undercount notes)"
  - "Rename collision 409 keeps the fixed literal text and gains code tag_name_taken; CollisionError.Error() embeds the stored tag name and is never echoed"
metrics:
  tasks: 3
  completed: 2026-10-07
status: complete
commits: 3
plan_head_before: 69f49c859fc86bb7021707954e01eda0b1f8eafe
actuals:
  tasks: 3
  commits: 3
---

# Phase quick/261007-ka1 Plan 01: Tag and note error contract, single validation point Summary

Tags and notes are now validated only in `tags.Service` / `watchlist.Service`; handlers map domain sentinels through one table per file to `{error: <sentinel text>, code: <stable code>}`, and the SPA branches on `code` and counts Unicode code points the way the server does.

## What changed

- **Go contract (998af72):** removed all handler-side tag-name/note pre-normalization, the name-message helper, and the dead nil guard in `handleListTags`. `errorResponse` gained `Code` (`omitempty`); `writeError` delegates to `writeErrorCode`, so uncoded responses are byte-identical. Limit sentinels are built with `fmt.Errorf` from the exported constants; constraint names are named consts, each literal once. Contract tests cover every sentinel on every route (status + code + text), the collision body, unknown error -> uncoded 500, real-Postgres 33-rune / 32-emoji tag names, and 501-rune notes through the real services.
- **Web (2046d34):** `ApiError.code`; `lib/limits.ts` (constants, `charCount`, `tagNameLength`, `noteLength`); `TagChips` / `ArtistNote` branch on code; native `maxLength` removed from tag, rename and note inputs; over-limit input disables submit / Enter, drops the Create option, and shows a destructive counter.
- **Embedded SPA (71d3bb1):** rebuilt with corepack from the changed `web/` source.

## Wire code list

Tags: `tag_name_required` (400), `tag_name_too_long` (400), `tag_name_invalid` (400), `tag_cap_reached` (409), `tag_name_taken` (409, rename collision), `tag_not_found` (404), `tag_merge_into_self` (400), `watchlist_entry_not_found` (404, `tags.ErrEntryNotFound`).
Notes: `note_too_long` (400), `note_invalid` (400).
Everything else (invalid id, invalid body, 503, 500, `watchlist.ErrNotFound`/`ErrDuplicate`/etc.) stays uncoded.

## Deviations from Plan

None - plan executed as written. The plan's chosen not-blank resolution (branch dropped) and the noteLength-without-NFC refinement are recorded under decisions above.

## Gate results (final tree)

- `go vet ./...`: clean
- `golangci-lint run`: 0 issues
- Go integration suite against Postgres (docker compose, started Docker Desktop): all 25 packages `ok`, no FAIL lines
- Backend coverage (`cmd/coverage-report --mode=total`): 90.18% (floor 80%)
- Web: `prettier --check` clean, `typecheck` clean, vitest 360/360 passing
- `make sqlc-check`: not required, no query changes
- Pre-commit hooks (gitleaks, golangci-lint, prettier) ran and passed on each commit; no `--no-verify`.

Environment notes (no code impact): `make` is not installed on this machine, so the Makefile steps were run by hand with the same flags; `-race` was dropped because there is no cgo/gcc (`go: -race requires cgo`); `go test -coverprofile` prints `go: no such tool "covdata"` for the test-less `internal/testutil` package and exits 1 even though every package reports `ok` and the profile is produced and measured.

## Known Stubs

None.

## Threat Flags

None. No new endpoints or trust-boundary surface; T-ka1-01..03 mitigations implemented as planned (body text is the matched sentinel's own text; collision keeps its fixed literal; services remain the validation point with DB CHECKs/trigger as backstop).

## Self-Check: PASSED

- web/app/lib/limits.ts and limits.test.ts exist
- Commits 998af72, 2046d34, 71d3bb1 present on gsd/phase-24-artist-tags-notes
- `git rev-list --count 69f49c8..HEAD` = 3, matching `commits: 3`
- Working tree clean for all touched paths
