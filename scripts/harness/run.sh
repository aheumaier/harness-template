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
# Every run appends {sensor, ms, exit, scope, sha, os} to .cache/harness/timings.jsonl and, when the
# sensor took longer than its budget in scripts/harness/budgets.txt, prints one warning line to stderr
# (rules.md#harness-latency-budget). Timing never changes the exit status: if it cannot be measured or
# written, it is skipped.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RULES="$ROOT/docs/harness/rules.md"
BUDGETS="$ROOT/scripts/harness/budgets.txt"
TIMINGS="$ROOT/.cache/harness/timings.jsonl"
MAX_LINES="${HARNESS_MAX_LINES:-80}"

now_ms() { # wall clock in ms: EPOCHREALTIME on bash ≥ 5, perl on macOS /bin/bash 3.2 (no %N in BSD date)
  if [[ -n "${EPOCHREALTIME:-}" ]]; then
    local t="${EPOCHREALTIME/[.,]/}" # the radix follows the locale
    echo $((t / 1000))
  else
    perl -MTime::HiRes=time -e 'printf "%d\n", time * 1000' 2>/dev/null
  fi
}

record_timing() { # sensor start-ms exit-status: append to TIMINGS, warn when over budget
  local end ms sha
  end="$(now_ms)"
  [[ "$2" =~ ^[0-9]+$ && "$end" =~ ^[0-9]+$ ]] || return 0
  ms=$((end - $2))
  sha="$(git -C "$ROOT" rev-parse --short HEAD 2>/dev/null)"
  { [[ -d "${TIMINGS%/*}" ]] || mkdir -p "${TIMINGS%/*}" 2>/dev/null; } &&
    printf '{"sensor":"%s","start_ms":%s,"ms":%s,"exit":%s,"scope":"%s","sha":"%s","os":"%s"}\n' \
      "$1" "$2" "$ms" "$3" "${HARNESS_SCOPE:-changed}" "$sha" "$OSTYPE" >>"$TIMINGS" 2>/dev/null
  # budgets.txt: "<sensor> <budget-ms> <nfr-id>"; the "default" row applies to sensors without a row.
  # LC_ALL=C: a decimal-comma locale would print "0,1 s".
  [[ -f "$BUDGETS" ]] && LC_ALL=C awk -v s="$1" -v ms="$ms" '
    /^[[:space:]]*(#|$)/ { next }
    $1 == s { b = $2; n = $3; found = 1 }
    $1 == "default" && !found { b = $2; n = $3 }
    END { if (b != "" && ms > b + 0) printf "harness: sensor '\''%s'\'' took %.1f s (budget %g s, %s)\n", s, ms / 1000, b / 1000, n }
  ' "$BUDGETS" >&2
  return 0
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
start="$(now_ms)"
(cd "$dir" && "$@") >"$log" 2>&1
status=$?
record_timing "$sensor" "$start" "$status"

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
