#!/usr/bin/env bash
# Template sync (rules.md#harness-upstream-in-sync, ADR 0026): lists the harness files that changed upstream
# since the template commit in .harness-version and that this repo has not taken yet. Never applies anything.
#
#   scripts/harness/sync-check.sh        (make harness-sync-check [HARNESS_UPSTREAM=<url>]; drift.yml, weekly)
#
# Upstream: HARNESS_UPSTREAM in scripts/tool-versions.env (written by make rename), or the make command line,
# which reaches this script as the environment variable. Empty: this repository is the template, nothing to
# sync. The upstream's default branch is fetched anonymously into a throwaway bare repo (the network: never in
# make check or check-all). Harness files are the globs in protected-paths.txt, this repo's and upstream's,
# except .harness-version itself. A path is reported when upstream's mode and content differ from both
# .harness-version's commit and this working tree; `# declined <path>: <why>` lines in .harness-version skip it.
#
# Exit 0: in sync (or the template). 1: behind, one line per file. 2: cannot check (a missing or malformed
# .harness-version, a refused URL, a failed or timed-out fetch, a marker commit not in upstream's history, any
# unexpected error): fails closed, never passes.
set -Eeuo pipefail

RULE=harness-upstream-in-sync
die() {
  echo "sync-check: $1 [$RULE]" >&2
  exit 2
}
trap 'die "unexpected failure at line $LINENO (exit $?)"' ERR

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
# The temp repo is addressed with --git-dir; none of the caller's repo overrides may leak into it.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

# --- .harness-version: the marker commit and the declined paths -------------------------------------------
[[ -f .harness-version ]] ||
  die ".harness-version is missing: write the template commit this repo was copied from"
base="$(grep -Ev '^[[:space:]]*(#|$)' .harness-version | head -n 1 || true)"
[[ "$base" =~ ^[0-9a-f]{40}$ ]] ||
  die ".harness-version: expected one 40-hex template commit, got '${base:0:60}'"
declined_paths=()
declined_whys=()
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ "$line" =~ ^#[[:space:]]*declined[[:space:]]+(.*)$ ]] || continue
  rest="${BASH_REMATCH[1]}"
  [[ "$rest" == *": "?* ]] || die ".harness-version: '$line': write '# declined <path>: <why>'"
  declined_paths+=("${rest%%: *}")
  declined_whys+=("${rest#*: }")
done <.harness-version
declined() { # path → prints the reason and succeeds when the path is declined
  local i
  for ((i = 0; i < ${#declined_paths[@]}; i++)); do
    [[ "${declined_paths[i]}" == "$1" ]] && echo "${declined_whys[i]}" && return 0
  done
  return 1
}

# --- the upstream URL -------------------------------------------------------------------------------------
url="${HARNESS_UPSTREAM:-}"
[[ -n "$url" ]] || url="$(sed -n 's/^HARNESS_UPSTREAM=//p' scripts/tool-versions.env 2>/dev/null | tail -n 1 || true)"
if [[ -z "$url" ]]; then
  echo "sync-check: HARNESS_UPSTREAM is empty: this repository is the template; nothing to sync"
  exit 0
fi
[[ "$url" != -* ]] || die "HARNESS_UPSTREAM starts with '-'"
if [[ "$url" =~ ://[^/@]*:[^/@]*@ || "$url" =~ ^https?://[^/@]+@ ]]; then
  die "HARNESS_UPSTREAM embeds credentials: remove them from the URL and use a git credential helper or a deploy key"
fi

# --- fetch upstream's default branch, anonymously and bounded ---------------------------------------------
tmp="$(mktemp -d "${TMPDIR:-/tmp}/harness-sync.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
up() { git --git-dir="$tmp" "$@"; }
up init -q --bare
# Read-only: no credential prompt, no transport that runs a command (ext::) or is unauthenticated plain text
# (git://, http://), and no hang: ssh in batch mode, a slow transfer aborts, and the whole fetch has 120 s
# where timeout(1) exists (stock macOS has none; the low-speed limit still applies).
limit=()
if command -v timeout >/dev/null 2>&1; then limit=(timeout 120); fi
status=0
env LC_ALL=C GIT_TERMINAL_PROMPT=0 GIT_ALLOW_PROTOCOL=file:https:ssh \
  GIT_HTTP_LOW_SPEED_LIMIT=1000 GIT_HTTP_LOW_SPEED_TIME=30 GIT_SSH_COMMAND='ssh -oBatchMode=yes -oConnectTimeout=15' \
  ${limit[@]+"${limit[@]}"} git --git-dir="$tmp" fetch -q --no-tags -- "$url" HEAD 2>"$tmp/fetch.err" || status=$?
((status != 124)) || die "fetching HEAD of $url timed out after 120 s"
((status == 0)) ||
  die "cannot fetch HEAD of $url: $(head -n 3 "$tmp/fetch.err" | tr '\n' ' ')(set HARNESS_UPSTREAM in scripts/tool-versions.env to the template's git URL; the fetch is anonymous, so check a private template locally with your credentials)"
tip="$(up rev-parse --verify -q FETCH_HEAD || true)"
[[ -n "$tip" ]] || die "upstream $url has no HEAD commit"
up merge-base --is-ancestor "$base" "$tip" 2>/dev/null ||
  die ".harness-version: $base is not in the history of $url (is HARNESS_UPSTREAM the template? set it in scripts/tool-versions.env)"

# --- harness paths that changed upstream since the marker -------------------------------------------------
specs=()
add_specs() { # protected-paths.txt on stdin → specs
  local spec rest
  while read -r spec rest || [[ -n "$spec" ]]; do
    [[ -z "$spec" || "$spec" == \#* ]] || specs+=("$spec")
  done
}
if [[ -f scripts/harness/protected-paths.txt ]]; then add_specs <scripts/harness/protected-paths.txt; fi
add_specs <<<"$(up show "$tip:scripts/harness/protected-paths.txt" 2>/dev/null || true)"
((${#specs[@]} > 0)) || die "no harness paths: scripts/harness/protected-paths.txt is missing here and upstream"
up diff -z --name-only --no-renames "$base" "$tip" -- "${specs[@]}" ':(exclude).harness-version' >"$tmp/changed"

entry() { # commit path → "mode blob", empty when absent
  local out
  out="$(GIT_LITERAL_PATHSPECS=1 up ls-tree -z "$1" -- "$2" | tr -d '\0')"
  out="${out%%$'\t'*}"
  [[ -z "$out" ]] || echo "${out%% *} ${out##* }"
}
local_entry() { # path → "mode blob" of the working tree, empty when absent
  if [[ -L "$1" ]]; then
    echo "120000 $(printf %s "$(readlink -- "$1")" | git hash-object --stdin)"
  elif [[ -f "$1" ]]; then
    if [[ -x "$1" ]]; then printf 100755; else printf 100644; fi
    echo " $(git hash-object -- "$1")"
  fi
}

behind=0
while IFS= read -r -d '' path; do
  theirs="$(entry "$tip" "$path")"
  ours="$(local_entry "$path")"
  [[ "$theirs" != "$ours" ]] || continue # already taken
  if why="$(declined "$path")"; then
    echo "$path: declined ($why), changed upstream since ${base:0:12}"
    continue
  fi
  was="$(entry "$base" "$path")"
  if [[ -z "$theirs" ]]; then
    what="deleted upstream"
  elif [[ -z "$was" ]]; then
    what="added upstream"
  else
    what="changed upstream"
  fi
  if [[ -z "$ours" && -n "$was" ]]; then
    what="$what, missing here"
  elif [[ -n "$ours" && -z "$was" ]]; then
    what="$what, present here too (merge by hand)"
  elif [[ -n "$ours" && "$ours" != "$was" ]]; then
    what="$what, changed here too (merge by hand)"
  fi
  echo "$path: $what since ${base:0:12} [$RULE]"
  behind=$((behind + 1))
done <"$tmp/changed"
((behind > 0)) || exit 0
echo "$behind harness file(s) behind $url at ${tip:0:12}: review with git fetch $url HEAD && git diff ${base:0:12} FETCH_HEAD -- <path>;" \
  "after porting them (or declining them in .harness-version) in a harness PR, write $tip to .harness-version"
exit 1
