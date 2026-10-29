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

# Some findings cannot carry their rule id: golangci ends a line with the linter's name ("(funlen)"), ESLint's
# stylish format with its rule's name ("max-params"). This maps each such finding to the rule it belongs to, so
# only that rule's remediation prints (ADR 0024: each size measure prints its own split recipe).
finding_pattern() { # rule-id -> ERE matching a tool finding of that rule, or nothing
  case "$1" in
    code-function-length) echo '\(funlen\)$|[[:space:]]max-lines-per-function$' ;;
    code-complexity) echo '\(gocognit\)$|[[:space:]]complexity$' ;;
    code-parameters) echo ': argument-limit: .*\(revive\)$|[[:space:]]max-params$' ;;
    code-file-length) echo ': file-length-limit: .*\(revive\)$|[[:space:]]max-lines$' ;;
    go-sec) echo '\(gosec\)$' ;;
    go-structured-logging) echo '\(sloglint\)$' ;;
    go-format) echo '\((gofmt|goimports)\)$' ;;
    web-lint) echo '^[[:space:]]+[0-9]+:[0-9]+[[:space:]]+(error|warning)[[:space:]]' ;; # any ESLint finding
  esac
}
catch_all() { [[ "$1" == web-lint ]]; } # owns the mapped findings that no other given rule names or maps

named() { # rule-id -> true if the output names the rule or holds one of its mapped findings
  local id="$1" pattern found other
  grep -q -- "$id" "$log" && return 0
  pattern="$(finding_pattern "$id")"
  [[ -n "$pattern" ]] || return 1
  found="$(grep -E -- "$pattern" "$log")" || return 1
  if catch_all "$id"; then
    for other in "${ids[@]}"; do
      [[ "$other" == "$id" ]] && continue
      found="$(grep -vF -- "$other" <<<"$found")"
      pattern="$(finding_pattern "$other")"
      if [[ -n "$pattern" ]]; then found="$(grep -Ev -- "$pattern" <<<"$found")"; fi
    done
  fi
  [[ -n "$found" ]]
}

general() { [[ -z "$(finding_pattern "$1")" ]] || catch_all "$1"; } # not a rule for one kind of finding

# Show the rules named in the output; if the tool names none of them, show the general ones (so a split recipe
# prints only when its finding is there), or every rule when none is general (e.g. go-format alone).
shown() { # rule-id -> true if its remediation prints
  named "$1" && return 0
  ((any)) && return 1
  ((! general_given)) || general "$1"
}

IFS=',' read -ra ids <<<"$rules"
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
  any=0 general_given=0
  for id in "${ids[@]}"; do
    named "$id" && any=1
    general "$id" && general_given=1
  done
  for id in "${ids[@]}"; do
    if shown "$id"; then
      echo "rule:        $id"
      echo "remediation: $(remediation "$id")"
      echo "docs:        docs/harness/rules.md#$id"
    fi
  done
} >&2
[[ $lines -le $MAX_LINES ]] && rm -f "$log"
exit "$status"
