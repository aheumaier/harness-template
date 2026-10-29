#!/usr/bin/env bash
# Sensor wrapper: silent on success, verbose and actionable on failure.
#
#   run.sh <sensor-id> <rule-id>[,<rule-id>...] [--dir <dir>] -- <command...>
#
# On failure prints the sensor, the tool output (file/line/rule as the tool reports it) and, for each
# rule whose id appears in the output (directly, or as a tool finding mapped to it by finding_pattern; if none
# does, every rule given without a specific finding_pattern, or all if each has one), the remediation and its anchor in
# docs/harness/rules.md. Remediation text lives ONLY in rules.md ("- **Remediation:** ..." line under
# "### <rule-id>"), so guides and sensors cannot drift apart.
#
# Every run is timed by scripts/harness/timing.sh: one row in .cache/harness/timings.jsonl and, over the
# sensor's budget in scripts/harness/budgets.txt, one warning line (rules.md#harness-latency-budget).
# Timing never changes the exit status: if it cannot be measured or written, it is skipped.
#
# The row also holds each given rule's finding count ("findings"), for the drift report of rules that never
# fire (scripts/harness/rule-fires.sh, ADR 0032): 0 on success; on failure, for a rule whose remediation
# prints, its output lines (those naming it or holding one of its mapped findings), at least 1, so a rule
# whose tool names no rule (a failing test, a gofmt diff) still counts as fired; 0 for the others. The number
# is lines naming the rule, not findings: only "more than 0" (fired) is used.
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
IFS=',' read -ra ids <<<"$rules"
counts=() # finding count per rule id, by index; unset = 0

findings_json() { # -> {"<id>":<count>,...}
  local i json=""
  for i in "${!ids[@]}"; do json+="${json:+,}\"${ids[i]}\":${counts[i]:-0}"; done
  echo "{$json}"
}

log="$(mktemp -t "harness-${sensor}.XXXXXX")"
start="$(harness_now_ms)"
(cd "$dir" && "$@") >"$log" 2>&1
status=$?
end="$(harness_now_ms)"

if [[ $status -eq 0 ]]; then
  harness_record sensor "$sensor" "$start" "$status" "$(findings_json)" "$end"
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

# Some findings cannot carry their rule id: golangci ends a line with the linter's name ("(funlen)"), ESLint's
# stylish format with its rule's name ("max-params"). This maps each such finding to the rule it belongs to, so
# only that rule's remediation prints (ADR 0024: each size measure prints its own split recipe).
finding_pattern() { # rule-id -> ERE matching a tool finding of that rule, or nothing
  case "$1" in
    code-function-length) echo '\(funlen\)$|[[:space:]]max-lines-per-function$' ;;
    code-complexity) echo '\(gocognit\)$|[[:space:]]complexity$' ;;
    code-parameters) echo ': argument-limit: .*\(revive\)$|[[:space:]]max-params$' ;;
    code-file-length) echo ': file-length-limit: .*\(revive\)$|[[:space:]]max-lines$' ;;
    repo-no-clones) echo '\(dupl\)$' ;;
    go-sec) echo '\(gosec\)$' ;;
    go-structured-logging) echo '\(sloglint\)$' ;;
    go-format) echo '\((gofmt|goimports)\)$' ;;
    web-lint) echo '^[[:space:]]+[0-9]+:[0-9]+[[:space:]]+(error|warning)[[:space:]]' ;; # any ESLint finding
  esac
}
catch_all() { [[ "$1" == web-lint ]]; } # owns the mapped findings that no other given rule names or maps

findings_of() { # rule-id -> the output lines that name the rule or hold one of its mapped findings
  local id="$1" pattern found other
  grep -- "$id" "$log"
  pattern="$(finding_pattern "$id")"
  [[ -n "$pattern" ]] || return 0
  found="$(grep -v -- "$id" "$log" | grep -E -- "$pattern")" || return 0
  if catch_all "$id"; then
    for other in "${ids[@]}"; do
      [[ "$other" == "$id" ]] && continue
      found="$(grep -vF -- "$other" <<<"$found")"
      pattern="$(finding_pattern "$other")"
      if [[ -n "$pattern" ]]; then found="$(grep -Ev -- "$pattern" <<<"$found")"; fi
    done
  fi
  [[ -z "$found" ]] || printf '%s\n' "$found"
}

general() { [[ -z "$(finding_pattern "$1")" ]] || catch_all "$1"; } # not a rule for one kind of finding

# Show the rules named in the output; if the tool names none of them, show the general ones (so a split recipe
# prints only when its finding is there), or every rule when none is general (e.g. go-format alone). A shown
# rule fired: its count is its findings, at least 1.
matched=() # output lines per rule id, by index
any=0 general_given=0
for i in "${!ids[@]}"; do
  matched[i]="$(findings_of "${ids[i]}" | wc -l | tr -d ' ')"
  ((matched[i] > 0)) && any=1
  general "${ids[i]}" && general_given=1
done
for i in "${!ids[@]}"; do
  counts[i]=0
  if ((matched[i] > 0)); then
    counts[i]="${matched[i]}"
  elif ((! any)) && { ((! general_given)) || general "${ids[i]}"; }; then
    counts[i]=1
  fi
done
harness_record sensor "$sensor" "$start" "$status" "$(findings_json)" "$end"

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
  for i in "${!ids[@]}"; do
    if ((counts[i] > 0)); then
      echo "rule:        ${ids[i]}"
      echo "remediation: $(remediation "${ids[i]}")"
      echo "docs:        docs/harness/rules.md#${ids[i]}"
    fi
  done
} >&2
[[ $lines -le $MAX_LINES ]] && rm -f "$log"
exit "$status"
