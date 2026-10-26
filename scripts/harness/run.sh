#!/usr/bin/env bash
# Sensor wrapper: silent on success, verbose and actionable on failure.
#
#   run.sh <sensor-id> <rule-id>[,<rule-id>...] [--dir <dir>] -- <command...>
#
# On failure prints the sensor, the tool output (file/line/rule as the tool reports it) and, for each
# rule whose id appears in the output (or the only rule given), the remediation and its anchor in
# docs/harness/rules.md. Remediation text lives ONLY in rules.md ("- **Remediation:** ..." line under
# "### <rule-id>"), so guides and sensors cannot drift apart.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RULES="$ROOT/docs/harness/rules.md"
MAX_LINES="${HARNESS_MAX_LINES:-80}"

sensor="$1"; rules="$2"; shift 2
dir="$ROOT"
if [[ "${1:-}" == "--dir" ]]; then dir="$ROOT/$2"; shift 2; fi
[[ "${1:-}" == "--" ]] && shift

log="$(mktemp -t "harness-${sensor}.XXXXXX")"
( cd "$dir" && "$@" ) >"$log" 2>&1
status=$?

if [[ $status -eq 0 ]]; then
  rm -f "$log"
  exit 0
fi

# Strip ANSI colour codes so output is legible to agents and in CI logs.
sed -i.bak $'s/\x1b\\[[0-9;]*m//g' "$log" && rm -f "$log.bak"

remediation() { # rule-id -> remediation line from rules.md
  awk -v id="$1" '
    $0 ~ "^### " id "$" { found=1; next }
    found && /^### / { exit }
    found && /\*\*Remediation:\*\*/ { sub(/^.*\*\*Remediation:\*\* */, ""); print; exit }
  ' "$RULES"
}

{
  echo "✗ sensor '${sensor}' failed (exit ${status}) — cwd: ${dir#"$ROOT"/}"
  echo "  command: $*"
  echo "────────────────────────────────────────────────────────────"
  lines=$(wc -l <"$log" | tr -d ' ')
  head -n "$MAX_LINES" "$log"
  if (( lines > MAX_LINES )); then echo "… $((lines - MAX_LINES)) more lines in $log"; fi
  echo "────────────────────────────────────────────────────────────"
  IFS=',' read -ra ids <<<"$rules"
  for id in "${ids[@]}"; do
    if (( ${#ids[@]} == 1 )) || grep -q -- "$id" "$log"; then
      echo "rule:        $id"
      echo "remediation: $(remediation "$id")"
      echo "docs:        docs/harness/rules.md#$id"
    fi
  done
} >&2
[[ $lines -le $MAX_LINES ]] && rm -f "$log"
exit "$status"
