# shellcheck shell=bash
# Sourced by .claude/hooks/*.sh only when .bin/harness is missing (a fresh clone or worktree before
# `make setup`). Without it a hook cannot parse its input, so it fails closed (exit 2; a missing binary's
# exit 127 would not block), except for the commands that build it: blocking those would lock the repo
# (ADR 0012). The match is on the raw JSON, anchored on both quotes, so `make setup; rm -rf x` is not one.
#
#   harness_missing <hook> [make-target-ERE]   reads the hook input on stdin; always exits

harness_missing() {
  local re="\"command\"[[:space:]]*:[[:space:]]*\"make (setup|\\.bin/harness${2:+|$2})\"[[:space:]]*[,}]"
  [[ "$(cat)" =~ $re ]] && exit 0
  echo "$1: blocked: .bin/harness is missing; run \`make setup\` (it builds the hooks' parser)." >&2
  exit 2
}
