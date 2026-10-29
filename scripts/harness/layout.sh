#!/usr/bin/env bash
# Reads scripts/harness/layout.env, the file next to this one: where the product's code lives (exec plan 0003,
# P4.6; ADR 0030). layout.env is parsed, never executed, sourced or included (the Makefile and CI read --print):
# only the known keys, each once, each value from a fixed charset, so it cannot run a command or set any other
# variable. The Go twin is
# apps/api/tools/harness/layout.go (`.bin/harness layout-check`); keep both parsers in step.
#
#   source scripts/harness/layout.sh || exit 2     # in a script
#   scripts/harness/layout.sh --print [<file>]     # validated KEY=value lines (Makefile, CI's $GITHUB_ENV)
#
# Sourced, it sets API_DIR, SPEC, MIGRATIONS_DIR, QUERIES_DIR, API_CLIENT_DIR and LAYER_GLOB as strings;
# WEB_GLOBS, GENERATED_GLOBS and PRODUCT_GLOBS as arrays (the `:`-separated entries); WEB_DIRS (each WEB_GLOBS
# entry without its `/*`); API_PREFIX, the prefix of a repo path inside API_DIR ("" for API_DIR=., else
# "$API_DIR/"); and DOMAINS_DIR, the repo path of the domains' parent (API_PREFIX + LAYER_GLOB without `/*/*`).
# Both forms fail closed (return 1, exit 2) with `<file>:<line>: <message> [harness-layout-valid]` on stderr
# when the file is missing, a line is not KEY=value, a key is unknown, repeated or missing, or a value is empty,
# holds a character outside [A-Za-z0-9_./*@+-] or an empty list entry.

HARNESS_LAYOUT_KEYS='API_DIR SPEC MIGRATIONS_DIR QUERIES_DIR API_CLIENT_DIR WEB_GLOBS GENERATED_GLOBS PRODUCT_GLOBS LAYER_GLOB'

# harness_layout_parse sets each key as a plain string; nothing else.
harness_layout_parse() { # <layout.env>
  local file="$1" line key value n=0 seen=' ' bad=''
  local re='^([A-Z_]+)=([A-Za-z0-9_./*@+-]+(:[A-Za-z0-9_./*@+-]+)*)$'
  [[ -r "$file" ]] || {
    echo "$file: missing or unreadable; restore scripts/harness/layout.env [harness-layout-valid]" >&2
    return 1
  }
  while IFS= read -r line || [[ -n "$line" ]]; do
    n=$((n + 1))
    [[ "$line" =~ ^[[:space:]]*(#|$) ]] && continue
    if [[ ! "$line" =~ $re ]]; then
      bad+="$file:$n: not KEY=value (no spaces, quotes, \$ or empty list entries) [harness-layout-valid]"$'\n'
      continue
    fi
    key="${BASH_REMATCH[1]}" value="${BASH_REMATCH[2]}"
    if [[ " $HARNESS_LAYOUT_KEYS " != *" $key "* ]]; then
      bad+="$file:$n: unknown key $key (known: $HARNESS_LAYOUT_KEYS) [harness-layout-valid]"$'\n'
    elif [[ "$seen" == *" $key "* ]]; then
      bad+="$file:$n: $key set twice [harness-layout-valid]"$'\n'
    else
      seen+="$key "
      printf -v "$key" '%s' "$value"
    fi
  done <"$file"
  for key in $HARNESS_LAYOUT_KEYS; do
    [[ "$seen" == *" $key "* ]] || bad+="$file: missing $key [harness-layout-valid]"$'\n'
  done
  if [[ -n "$bad" ]]; then
    printf '%s' "$bad" >&2
    return 1
  fi
}

# harness_layout_load parses, then splits the lists and derives the prefixes.
harness_layout_load() { # <layout.env>
  local value
  harness_layout_parse "$1" || return 1
  IFS=: read -ra WEB_GLOBS <<<"$WEB_GLOBS"
  IFS=: read -ra GENERATED_GLOBS <<<"$GENERATED_GLOBS"
  IFS=: read -ra PRODUCT_GLOBS <<<"$PRODUCT_GLOBS"
  WEB_DIRS=()
  for value in "${WEB_GLOBS[@]}"; do WEB_DIRS+=("${value%/\*}"); done
  API_PREFIX=''
  [[ "$API_DIR" == . ]] || API_PREFIX+="$API_DIR/"
  # shellcheck disable=SC2034 # reason: read by the scripts that source this file (rule: sh-shellcheck)
  DOMAINS_DIR="$API_PREFIX${LAYER_GLOB%/\*/\*}"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  set -euo pipefail
  [[ "${1:-}" == --print && $# -le 2 ]] || {
    echo "usage: $0 --print [<layout.env>]" >&2
    exit 2
  }
  harness_layout_parse "${2:-$(dirname "$0")/layout.env}" || exit 2
  for key in $HARNESS_LAYOUT_KEYS; do printf '%s=%s\n' "$key" "${!key}"; done
  exit 0
fi
harness_layout_load "$(dirname "${BASH_SOURCE[0]}")/layout.env"
