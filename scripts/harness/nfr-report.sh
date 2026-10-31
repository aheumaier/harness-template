#!/usr/bin/env bash
# Latency trend (rules.md#harness-latency-budget, ADR 0013): p95 (nearest rank) per sensor and hook over
# timings.jsonl files written by scripts/harness/timing.sh, against scripts/harness/budgets.txt.
#
#   scripts/harness/nfr-report.sh <timings.jsonl>...     (nightly in harness-nfr.yml, over 7 days of CI)
#
# Prints a Markdown table of the names whose p95 is over budget and nothing when all are within. Exits 2 when
# the input has no timing rows: a broken timings pipeline must not read as "everything is fast".
# HARNESS_BUDGETS overrides the budgets file, as in timing.sh.
set -euo pipefail

(($# > 0)) || {
  echo "usage: nfr-report.sh <timings.jsonl>..." >&2
  exit 2
}
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
budgets="${HARNESS_BUDGETS:-$ROOT/scripts/harness/budgets.txt}"

# timing.sh writes one fixed-format JSON object per line; rows from PR runs are untrusted, so only ids made of
# [A-Za-z0-9_.-] are kept. Output: "<name> <ms>", sorted by name, then by time.
rows="$(cat "$@" | sed -nE 's/^\{"sensor":"([A-Za-z0-9_.-]+)","start_ms":[0-9]+,"ms":([0-9]+),.*$/\1 \2/p' | LC_ALL=C sort -k1,1 -k2,2n)"
[[ -n "$rows" ]] || {
  echo "nfr-report: no timing rows in $*" >&2
  exit 2
}

LC_ALL=C awk -v budgets="$budgets" '
  function sec(ms, s) { s = sprintf("%.3f", ms / 1000); sub(/0+$/, "", s); sub(/\.$/, "", s); return s }
  function flush(k, p, b, id) {
    if (name == "") return
    k = int((95 * n + 99) / 100) # ceil(0.95 n)
    p = ms[k]
    b = (name in budget) ? budget[name] : budget["default"]
    id = (name in budget) ? nfr[name] : nfr["default"]
    if (b != "" && p > b) out = out sprintf("| %s | %d | %s | %s | %s |\n", name, n, sec(p), sec(b), id)
  }
  BEGIN {
    while ((getline line < budgets) > 0) {
      if (split(line, f, " ") >= 3 && f[2] ~ /^[0-9]+$/) { budget[f[1]] = f[2]; nfr[f[1]] = f[3] }
    }
  }
  $1 != name { flush(); name = $1; n = 0 }
  { ms[++n] = $2 }
  END {
    flush()
    if (out != "") printf "| Sensor or hook | Runs | p95 (s) | Budget (s) | NFR |\n|---|---|---|---|---|\n%s", out
  }
' <<<"$rows"
