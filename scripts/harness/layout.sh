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
# STACK, the sensor groups this product has (ADR 0033), is optional: a layout.env without it (one written before
# the key existed, such as a base branch's copy that migrations-append-only.sh reads) has go:db:web, the groups
# that existed then (ios came later, ADR 0041).
# The environment may override it for one run: HARNESS_STACK=go,db (`,` or `:` separated) replaces it in both
# forms. Either way it must name each of go, db, web and ios at most once, and go always; harness_stack_has <group>
# tests it.
#
# UPSTREAM_SPECS and CONTRACT_CONSUMERS (cross-repo contracts, ADR 0037) are optional too, and the only keys whose
# value may be empty: a layout.env without them, or with `KEY=`, has none.
#
# Sourced, it sets API_DIR, SPEC, MIGRATIONS_DIR, QUERIES_DIR, API_CLIENT_DIR, LAYER_GLOB and STACK as strings;
# WEB_GLOBS, GENERATED_GLOBS, PRODUCT_GLOBS, UPSTREAM_SPECS and CONTRACT_CONSUMERS as arrays (the `:`-separated
# entries; the last two may be empty); WEB_DIRS (each WEB_GLOBS
# entry without its `/*`); API_PREFIX, the prefix of a repo path inside API_DIR ("" for API_DIR=., else
# "$API_DIR/"); and DOMAINS_DIR, the repo path of the domains' parent (API_PREFIX + LAYER_GLOB without `/*/*`).
# Both forms fail closed (return 1, exit 2) with `<file>:<line>: <message> [harness-layout-valid]` on stderr
# when the file is missing, a line is not KEY=value, a key is unknown, repeated or missing (the optional ones
# aside), or a value is empty (the two above aside),
# holds a character outside [A-Za-z0-9_./*@+-] or an empty list entry, or when STACK (after HARNESS_STACK) is
# not a valid stack.

HARNESS_LAYOUT_KEYS='API_DIR SPEC MIGRATIONS_DIR QUERIES_DIR API_CLIENT_DIR WEB_GLOBS GENERATED_GLOBS PRODUCT_GLOBS LAYER_GLOB STACK UPSTREAM_SPECS CONTRACT_CONSUMERS'
# Optional lists, empty when unset (never inherited from the environment, where CI's setup action exports them).
HARNESS_LAYOUT_EMPTY_OK='UPSTREAM_SPECS CONTRACT_CONSUMERS'
HARNESS_STACK_GROUPS='go db web ios'
HARNESS_STACK_DEFAULT='go:db:web'

# harness_layout_parse sets each key as a plain string; nothing else.
harness_layout_parse() { # <layout.env>
  local file="$1" line key value n=0 seen=' ' bad='' stack_line=''
  local re='^([A-Z_]+)=(([A-Za-z0-9_./*@+-]+(:[A-Za-z0-9_./*@+-]+)*)?)$'
  for key in $HARNESS_LAYOUT_EMPTY_OK; do printf -v "$key" '%s' ''; done
  [[ -r "$file" ]] || {
    echo "$file: missing or unreadable; restore scripts/harness/layout.env [harness-layout-valid]" >&2
    return 1
  }
  while IFS= read -r line || [[ -n "$line" ]]; do
    n=$((n + 1))
    [[ "$line" =~ ^[[:space:]]*(#|$) ]] && continue
    if [[ ! "$line" =~ $re ]] || [[ -z "${BASH_REMATCH[2]}" && " $HARNESS_LAYOUT_EMPTY_OK " != *" ${BASH_REMATCH[1]} "* ]]; then
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
      [[ "$key" != STACK ]] || stack_line=$n
      printf -v "$key" '%s' "$value"
    fi
  done <"$file"
  for key in $HARNESS_LAYOUT_KEYS; do
    [[ "$seen" == *" $key "* || "$key" == STACK || " $HARNESS_LAYOUT_EMPTY_OK " == *" $key "* ]] || bad+="$file: missing $key [harness-layout-valid]"$'\n'
  done
  if [[ -n "$bad" ]]; then
    printf '%s' "$bad" >&2
    return 1
  fi
  if [[ -z "$stack_line" ]]; then
    STACK="$HARNESS_STACK_DEFAULT"
    harness_stack_apply "$file: STACK unset, so $STACK"
  else
    harness_stack_apply "$file:$stack_line: STACK=$STACK"
  fi
}

# harness_stack_apply replaces STACK with HARNESS_STACK when that is set (`,` read as `:`), then validates it:
# each finding on stderr, prefixed with where (the STACK line), return 1 when there is one.
harness_stack_apply() { # <where>
  local where="$1" group seen=' ' bad=''
  if [[ -n "${HARNESS_STACK+set}" ]]; then
    STACK="${HARNESS_STACK//,/:}" where="HARNESS_STACK=$HARNESS_STACK"
  fi
  if [[ ! "$STACK" =~ ^[a-z]+(:[a-z]+)*$ ]]; then
    echo "$where: not a list of groups such as go:db:web (HARNESS_STACK may use , instead of :) [harness-layout-valid]" >&2
    return 1
  fi
  for group in ${STACK//:/ }; do
    if [[ " $HARNESS_STACK_GROUPS " != *" $group "* ]]; then
      bad+="$where: unknown group $group (known: $HARNESS_STACK_GROUPS) [harness-layout-valid]"$'\n'
    elif [[ "$seen" == *" $group "* ]]; then
      bad+="$where: $group named twice [harness-layout-valid]"$'\n'
    fi
    seen+="$group "
  done
  [[ "$seen" == *" go "* ]] || bad+="$where: no go (every stack has the Go API and the harness) [harness-layout-valid]"$'\n'
  [[ -z "$bad" ]] || {
    printf '%s' "$bad" >&2
    return 1
  }
}

# harness_stack_has <group>: STACK names group (db, web, ios; go always).
harness_stack_has() { [[ ":$STACK:" == *":$1:"* ]]; }

# harness_layout_load parses, then splits the lists and derives the prefixes.
harness_layout_load() { # <layout.env>
  local value
  harness_layout_parse "$1" || return 1
  IFS=: read -ra WEB_GLOBS <<<"$WEB_GLOBS"
  IFS=: read -ra GENERATED_GLOBS <<<"$GENERATED_GLOBS"
  IFS=: read -ra PRODUCT_GLOBS <<<"$PRODUCT_GLOBS"
  IFS=: read -ra UPSTREAM_SPECS <<<"$UPSTREAM_SPECS"
  IFS=: read -ra CONTRACT_CONSUMERS <<<"$CONTRACT_CONSUMERS"
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
