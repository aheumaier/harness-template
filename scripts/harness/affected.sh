#!/usr/bin/env bash
# Affected set for the scoped sensors of make check (rules.md#harness-scope, exec plan 0002 phase 2).
#
#   scripts/harness/affected.sh go    Go packages of API_DIR to lint and test, as ./dir relative to API_DIR
#   scripts/harness/affected.sh web   JS workspaces to lint and cruise, as dirs relative to the repo
#
# Exit 0: the affected set, one per line, sorted; empty means no input of these sensors changed. Exit 1: run on
# everything. Exit 2: usage. The change set is everything between the merge base (merge-base.sh) and the working
# tree plus untracked files. A changed package or workspace pulls in everything that depends on it: a changed
# type can break its importers, and the type-aware ESLint rules and the Go compiler judge the importer.
# Everything runs (exit 1) in any scope but `changed` (make check-all, CI), when a harness file, go.mod, go.sum,
# a package.json, the lockfile or the workspace config changed, on a changed non-Go file under API_DIR (an
# embedded file, a migration; with API_DIR=. any non-Go file), a changed .go file outside a current package (deleted or tagged out), a changed
# JS/TS/JSON file outside a workspace, and on any error (no merge base, `go list` or `turbo` failing).
set -uo pipefail

[[ $# == 1 && ("$1" == go || "$1" == web) ]] || {
  echo "usage: affected.sh go|web" >&2
  exit 2
}
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT" || exit 1
# shellcheck source=layout.sh
source scripts/harness/layout.sh || exit 1

scripts/harness/unchanged.sh '*go.mod' '*go.sum' '*package.json' pnpm-lock.yaml pnpm-workspace.yaml turbo.json ||
  exit 1
base="$(scripts/harness/merge-base.sh 2>/dev/null)" || exit 1
# --no-renames: a moved file counts at its old path too. Markdown feeds none of these sensors.
changed="$(git diff --name-only --no-renames "$base" -- && git ls-files -o --exclude-standard)" || exit 1
changed="$(printf '%s\n' "$changed" | grep -v '\.md$')"

go_packages() {
  local api mod
  # The changed files under API_DIR, relative to it; tools/: own modules.
  api="$(printf '%s\n' "$changed" | while IFS= read -r f; do
    [[ "$f" == "$API_PREFIX"* ]] && printf '%s\n' "${f#"$API_PREFIX"}"
  done | grep -v '^tools/')"
  [[ -n "$api" ]] || return 0
  ! grep -qv '\.go$' <<<"$api" || exit 1 # a here-string: grep -q cannot SIGPIPE a writer
  cd "$API_DIR" || exit 1
  mod="$(go list -m)" || exit 1
  # go list -test adds the test variants (`p [p.test]`, `p_test [p.test]`, `p.test`): their Deps carry the
  # imports of the tests. .Deps is transitive, so one pass finds every package that depends on a changed one.
  go list -test -f '{{.ImportPath}}|{{join .Deps "|"}}' ./... | CHANGED="$api" awk -F'|' -v mod="$mod" '
    function key(p) { sub(/ \[.*/, "", p); sub(/\.test$/, "", p); sub(/_test$/, "", p); return p }
    BEGIN {
      n = split(ENVIRON["CHANGED"], files, "\n")
      for (i = 1; i <= n; i++) {
        d = files[i]
        if (!sub(/\/[^\/]*$/, "", d)) d = ""
        dirs[d == "" ? mod : mod "/" d] = 1
      }
    }
    { lines[NR] = $0; known[key($1)] = 1 }
    END {
      for (d in dirs) if (!(d in known)) exit 3 # a changed .go file outside a current package
      for (i = 1; i <= NR; i++) {
        m = split(lines[i], f, "|")
        for (j = 1; j <= m; j++) if (key(f[j]) in dirs) { hit[key(f[1])] = 1; break }
      }
      for (p in hit) print (p == mod ? "." : "./" substr(p, length(mod) + 2))
    }' | LC_ALL=C sort
  ((PIPESTATUS[0] == 0 && PIPESTATUS[1] == 0)) || exit 1
}

web_workspaces() {
  local all f ws out hit=() filters=()
  all="$(turbo ls --output=json 2>/dev/null | sed -n 's/.*"path": *"\([^"]*\)".*/\1/p')" || exit 1
  [[ -n "$all" ]] || exit 1
  while read -r f; do
    [[ -n "$f" && ("$API_DIR" == . || "$f" != "$API_PREFIX"*) ]] || continue
    for ws in $all; do
      if [[ "$f" == "$ws"/* ]]; then
        [[ " ${hit[*]-} " == *" $ws "* ]] || hit+=("$ws")
        continue 2
      fi
    done
    [[ "$f" =~ \.(js|mjs|cjs|jsx|ts|tsx|mts|cts|json)$ ]] && exit 1 # read by ESLint or a tsconfig, no workspace
  done <<<"$changed"
  ((${#hit[@]} > 0)) || return 0
  for ws in "${hit[@]}"; do filters+=("--filter=...{./$ws}"); done # `...`: and its dependents
  out="$(turbo ls "${filters[@]}" --output=json 2>/dev/null | sed -n 's/.*"path": *"\([^"]*\)".*/\1/p')" || exit 1
  [[ -n "$out" ]] || exit 1
  printf '%s\n' "$out" | LC_ALL=C sort
}

if [[ "$1" == go ]]; then go_packages; else web_workspaces; fi
