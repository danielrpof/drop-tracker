#!/usr/bin/env bash
# Comment gates for quick task 261007-lc4. Run from the repo root:
#   bash .planning/quick/261007-lc4-phase-24-comment-cleanup/check/comment-gates.sh [ast|blocks|narration|notes|all] [start-commit]
# ast       every file changed since start (working tree included) differs only in comments
# blocks    every Phase-24 comment block (vs merge-base with main) is <= 4 lines and cites <= 1 design-doc id
# narration no plan/task narration left in Phase-24 comments
# notes     the kept security/race notes are still present
# counts    before/after comment-line table for the SUMMARY (not a gate)
# start defaults to the parent of the first commit mentioning 261007-lc4, else HEAD.
set -u
MODE=${1:-all}
first=$(git log --reverse --format=%H --grep='261007-lc4' | head -1)
START=${2:-${first:+$first^}}
START=${START:-HEAD}
echo "start=$(git rev-parse --short "$START")"
HERE=$(dirname "$0")
BASE=$(git merge-base main HEAD)
SCOPE=(internal cmd queries web/app ':!internal/webassets' ':!internal/db/migrations' ':!web/app/components/ui')
fail=0

ast_gate() {
  local tmp f a b
  tmp=$(mktemp -d)
  while IFS= read -r f; do
    case "$f" in
      *.go)
        git show "$START:$f" | go run "$HERE/stripgo.go" - >"$tmp/a" 2>&1
        go run "$HERE/stripgo.go" - <"$f" >"$tmp/b" 2>&1 ;;
      *.ts|*.tsx)
        git show "$START:$f" | node "$HERE/strip-ts.cjs" - "$f" >"$tmp/a" 2>&1
        node "$HERE/strip-ts.cjs" "$f" "$f" >"$tmp/b" 2>&1 ;;
      *.sql)
        git show "$START:$f" | grep -vE '^[[:space:]]*(--.*)?$' | grep -v '^$' >"$tmp/a"
        grep -vE '^[[:space:]]*(--.*)?$' "$f" >"$tmp/b" ;;
      *) echo "AST FAIL $f: not a .go/.ts/.tsx/.sql file"; fail=1; continue ;;
    esac
    if ! cmp -s "$tmp/a" "$tmp/b"; then
      echo "AST FAIL $f: code differs, not just comments"; diff "$tmp/a" "$tmp/b" | head -20; fail=1
    fi
  done < <(git diff --name-only "$START" -- "${SCOPE[@]}"; git ls-files --others --exclude-standard -- "${SCOPE[@]}")
  rm -rf "$tmp"
  [ $fail -eq 0 ] && echo "ast: OK (comment-only since $START)"
}

blocks_gate() {
  # Phase-24-added comment blocks: consecutive added comment lines in the
  # working tree vs the merge-base. sqlc "-- name:" annotations are not comments.
  git diff -U0 "$BASE" -- "${SCOPE[@]}" ':!internal/db/sqlc' | awk '
    function flush() {
      if (run > 0) {
        n = 0; ids = ""
        for (k in seen) { n++; ids = ids " [" k "]" }
        if (run > 4) { printf "BLOCK FAIL %s:%d-%d %d lines\n", file, start, prev, run; bad = 1 }
        else if (run == 4) printf "BLOCK WARN %s:%d-%d 4 lines (justify in SUMMARY)\n", file, start, prev
        if (n > 1) { printf "IDS FAIL %s:%d-%d %d ids:%s\n", file, start, prev, n, ids; bad = 1 }
      }
      run = 0; split("", seen)
    }
    /^\+\+\+ / { flush(); file = substr($0, 7); next }
    /^@@/ { flush(); match($0, /\+[0-9]+/); ln = substr($0, RSTART + 1, RLENGTH - 1) + 0; next }
    /^\+/ {
      s = substr($0, 2); sub(/^[ \t]+/, "", s)
      isc = (s ~ /^\/\// || s ~ /^--/ || s ~ /^\/\*/ || s ~ /^\*/ || s ~ /^\{\/\*/) && s !~ /^-- name:/
      if (!isc || (run > 0 && ln != prev + 1)) flush()
      if (isc) {
        if (run == 0) start = ln
        run++; prev = ln
        t = " " s " "
        while (match(t, /[^A-Za-z0-9](D-[0-9]+|TAG-[0-9]+|NOTE-[0-9]+|WLST-[0-9]+|DGST-[0-9]+|G-[0-9]+-[0-9]+|T-[0-9]+-[0-9]+|WR-[0-9]+|CR-[0-9]+|IN-[0-9]+|SC[0-9]+|ADR [0-9]+|UI-SPEC|[0-9]+-RESEARCH\.md|Pitfall [0-9]+|Pattern [0-9]+)/)) {
          tok = substr(t, RSTART + 1, RLENGTH - 1)
          if (tok ~ /RESEARCH/ && substr(t, RSTART + RLENGTH) ~ /^ (Pitfall|Pattern) [0-9]+/) {
            t = substr(t, RSTART + RLENGTH); sub(/^ (Pitfall|Pattern) [0-9]+/, "", t)
          } else t = substr(t, RSTART + RLENGTH)
          seen[tok] = 1
        }
      }
      ln++; next
    }
    END { flush(); exit bad }
  ' || fail=1
  [ $fail -eq 0 ] && echo "blocks: OK"
}

narration_gate() {
  local files
  files=$(git diff --name-only "$BASE" -- "${SCOPE[@]}" ':!internal/db/sqlc')
  # Shared files whose untouched pre-Phase-24 comments carry older plan narration.
  files=$(echo "$files" | grep -vE '^(internal/authgate/gate_test.go|web/app/lib/api.test.ts)$')
  if grep -nE 'Task [0-9]|[Pp]lan 24-[0-9]|24-0[0-9] Task|plan objective|24-01 confirmed|\(24-01\)' $files; then
    echo "narration: FAIL"; fail=1
  else echo "narration: OK"; fi
}

notes_gate() {
  local spec f pat
  for spec in \
    'internal/httpserver/watchlist.go|never reach a log' \
    'internal/httpserver/tags.go|CollisionError.Error()' \
    'queries/tags.sql|deadlock' \
    'queries/watchlist.sql|mispair' \
    'internal/watchlist/service.go|mispair' \
    'web/app/components/watchlist/TagChips.tsx|raw HTML' \
    'web/app/components/watchlist/ArtistNote.tsx|raw HTML'; do
    f=${spec%%|*}; pat=${spec#*|}
    grep -qF "$pat" "$f" || { echo "notes FAIL: '$pat' missing from $f"; fail=1; }
  done
  [ $fail -eq 0 ] && echo "notes: OK"
}

counts_report() {
  # Comment lines before (start) and after (working tree) for every changed file.
  local f cnt='{ s=$0; sub(/^[ \t]+/,"",s); if ((s ~ /^\/\// || s ~ /^--/ || s ~ /^\/\*/ || s ~ /^\*/ || s ~ /^\{\/\*/) && s !~ /^-- name:/) c++ } END { print c+0 }'
  echo "| File | Comment lines before | after |"
  echo "|------|---------------------|-------|"
  while IFS= read -r f; do
    echo "| $f | $(git show "$START:$f" 2>/dev/null | awk "$cnt") | $(awk "$cnt" "$f") |"
  done < <(git diff --name-only "$START" -- "${SCOPE[@]}" ':!internal/db/sqlc')
}

case "$MODE" in
  counts) counts_report ;;
  ast) ast_gate ;;
  blocks) blocks_gate ;;
  narration) narration_gate ;;
  notes) notes_gate ;;
  all) ast_gate; blocks_gate; narration_gate; notes_gate ;;
  *) echo "unknown mode $MODE"; exit 2 ;;
esac
exit $fail
