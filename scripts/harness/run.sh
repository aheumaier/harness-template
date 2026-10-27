#!/usr/bin/env bash
# Sensor wrapper: silent on success, verbose and actionable on failure.
#
#   run.sh <sensor-id> <rule-id>[,<rule-id>...] [--dir <dir>] -- <command...>
#
# On failure prints the sensor, the tool output (file/line/rule as the tool reports it) and, for each
# rule whose id appears in the output (or the only rule given), the remediation and its anchor in
# docs/harness/rules.md. Remediation text lives ONLY in rules.md ("- **Remediation:** ..." line under
# "### <rule-id>"), so guides and sensors cannot drift apart.
#
# Every run is timed by scripts/harness/timing.sh: one row in .cache/harness/timings.jsonl and, over the
# sensor's budget in scripts/harness/budgets.txt, one warning line (rules.md#harness-latency-budget).
# Timing never changes the exit status: if it cannot be measured or written, it is skipped.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RULES="$ROOT/docs/harness/rules.md"
MAX_LINES="${HARNESS_MAX_LINES:-80}"
# shellcheck source=timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null || {
  harness_now_ms() { :; }
  harness_record() { :; }
}

sensor="$1"
rules="$2"
shift 2
dir="$ROOT"
if [[ "${1:-}" == "--dir" ]]; then
  dir="$ROOT/$2"
  shift 2
fi
[[ "${1:-}" == "--" ]] && shift

log="$(mktemp -t "harness-${sensor}.XXXXXX")"
start="$(harness_now_ms)"
(cd "$dir" && "$@") >"$log" 2>&1
status=$?
harness_record sensor "$sensor" "$start" "$status"

if [[ $status -eq 0 ]]; then
  rm -f "$log"
  exit 0
fi

# Strip ANSI colour codes and OSC-8 hyperlinks so output is legible to agents and in CI logs.
sed -i.bak -e $'s/\x1b\\[[0-9;]*m//g' -e $'s/\x1b]8;;[^\x1b]*\x1b\\\\//g' "$log" && rm -f "$log.bak"

remediation() { # rule-id -> remediation line from rules.md
  awk -v id="$1" '
    $0 ~ "^### " id "$" { found=1; next }
    found && /^### / { exit }
    found && /\*\*Remediation:\*\*/ { sub(/^.*\*\*Remediation:\*\* */, ""); print; exit }
  ' "$RULES"
}

{
  rel="${dir#"$ROOT"}"
  rel="${rel#/}"
  echo "✗ sensor '${sensor}' failed (exit ${status}) — cwd: ${rel:-.}"
  echo "  command: $*"
  echo "────────────────────────────────────────────────────────────"
  lines=$(wc -l <"$log" | tr -d ' ')
  head -n "$MAX_LINES" "$log"
  if ((lines > MAX_LINES)); then echo "… $((lines - MAX_LINES)) more lines in $log"; fi
  echo "────────────────────────────────────────────────────────────"
  IFS=',' read -ra ids <<<"$rules"
  # Show the rules named in the output; if the tool names none of them, show them all.
  any=0
  for id in "${ids[@]}"; do grep -q -- "$id" "$log" && any=1; done
  for id in "${ids[@]}"; do
    if ((! any)) || grep -q -- "$id" "$log"; then
      echo "rule:        $id"
      echo "remediation: $(remediation "$id")"
      echo "docs:        docs/harness/rules.md#$id"
    fi
  done
} >&2
[[ $lines -le $MAX_LINES ]] && rm -f "$log"
exit "$status"
