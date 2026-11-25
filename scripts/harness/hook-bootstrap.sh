# shellcheck shell=bash
# Sourced by .claude/hooks/*.sh when .bin/harness is missing (a fresh clone or worktree before
# `make setup`), and by protect-harness.sh and stop-gate.sh before every binary call, since the binary may be
# older than the hook (ADR 0045). Without the binary a hook cannot parse its input, so it fails closed (exit 2; a
# missing binary's exit 127 would not block; stop-gate.sh only once per turn, ADR 0048), except for the commands
# that build it: blocking those would lock the repo (ADR 0012). The match is on the raw JSON, anchored on both
# quotes, so `make setup; rm -rf x` is not one.
#
#   harness_builds <input> [make-target-ERE]   the hook input runs exactly one of those commands
#   harness_missing <hook> [make-target-ERE]   reads the hook input on stdin; always exits
#   harness_stale <hook> <exit> <output>       prints what to say when the binary exited non-zero

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

# The binary's own block, or that it is out of date or failed: a usage error (64, or the usage text of a binary
# from before that code) means it is older than the hook (ADR 0045); for any other failure its output says why. The
# block comes first: stop-gate's quotes the output of `make check`, which may hold that usage text.
harness_stale() {
  local out="$3"
  case $2 in
    64) out="" ;;
    2)
      if [[ "$out" == "$1:"* ]]; then
        printf '%s\n' "$out"
        return
      elif [[ $'\n'"$out" == *$'\n''usage: harness'* ]]; then
        out=""
      fi
      ;;
  esac
  [[ -z "$out" ]] || printf '%s\n' "$out"
  echo "$1: blocked: .bin/harness is out of date or failed (exit $2); rebuild it with \`make .bin/harness\`"
}
