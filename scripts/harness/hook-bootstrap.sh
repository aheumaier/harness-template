# shellcheck shell=bash
# Sourced by .claude/hooks/*.sh when .bin/harness is missing (a fresh clone or worktree before
# `make setup`), and by protect-harness.sh before every binary call, since the binary may be older than the
# hook (ADR 0045). Without the binary a hook cannot parse its input, so it fails closed (exit 2; a missing
# binary's exit 127 would not block), except for the commands that build it: blocking those would lock the
# repo (ADR 0012). The match is on the raw JSON, anchored on both quotes, so `make setup; rm -rf x` is not one.
#
#   harness_builds <input> [make-target-ERE]   the hook input runs exactly one of those commands
#   harness_missing <hook> [make-target-ERE]   reads the hook input on stdin; always exits

harness_builds() {
  local re="\"command\"[[:space:]]*:[[:space:]]*\"make (setup|\\.bin/harness${2:+|$2})\"[[:space:]]*[,}]"
  local key='"(command|tool_name)"[[:space:]]*:(.*)' rest="$1" n=0 nocase
  [[ "$1" =~ $re && "$1" =~ \"tool_name\"[[:space:]]*:[[:space:]]*\"Bash\" ]] || return 1
  # One command key and one tool_name key, so a JSON parser cannot read another command than this match. A
  # quote inside a JSON string is escaped, so only a key matches. Go's decoder also matches keys ignoring case
  # and decodes \u escapes in them, so keys are counted ignoring case and any \u fails closed (JSON.stringify
  # writes none for ASCII).
  [[ "$1" != *'\u'* ]] || return 1
  nocase="$(shopt -p nocasematch)"
  shopt -s nocasematch
  while [[ "$rest" =~ $key ]]; do
    rest="${BASH_REMATCH[2]}"
    ((++n))
  done
  eval "$nocase"
  ((n == 2))
}

harness_missing() {
  harness_builds "$(cat)" "${2:-}" && exit 0
  echo "$1: blocked: .bin/harness is missing; run \`make setup\` (it builds the hooks' parser)." >&2
  exit 2
}
