#!/usr/bin/env bash
# Test coverage per Go package and JS workspace (exec plan 0003 P2.5, docs/harness/ci.md#coverage). A trend,
# never a gate (docs/adr/0020-test-coverage-is-a-trend.md): it fails only when it cannot measure (missing
# tool, failing tests, unparseable input), never on a number. A gate on a number invites tests written for
# the number. A report, not a sensor: no rule id, and no required check runs it.
#
#   scripts/harness/coverage.sh collect <dir>                      (make coverage)
#       runs the unit tests with coverage and writes <dir>/coverage.tsv
#   scripts/harness/coverage.sh report [--fell] <tsv> [<previous tsv>]
#       Markdown table on stdout; with a previous tsv, a change column in percentage points. --fell prints
#       only the rows whose coverage fell since the previous tsv, and nothing when none did
#       (make coverage-fell, drift.yml).
#
# coverage.tsv, one row per line, sorted: "<package>\t<covered statements>\t<statements>"
#   - every Go package of apps/api (`go test -short`: unit tests, as make test-go; files marked
#     "Code generated ... DO NOT EDIT." are excluded), and their sum as "apps/api/...";
#   - every JS workspace (apps/*, packages/*) with a `coverage` script, run as `pnpm run coverage
#     --coverage.provider=v8 --coverage.reporter=json-summary --coverage.reportsDirectory=<dir>` (the
#     measurement is fixed here, not in the workspace's config; apps/web/AGENTS.md); "<workspace>\t-\t-"
#     for a workspace with neither a `test` nor a `coverage` script. One with tests but no `coverage`
#     script fails: its numbers would be missing, not zero.
#
# Exit 1 when tests fail, 2 on bad usage, a missing tool or unparseable input.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
usage="usage: coverage.sh collect <dir> | coverage.sh report [--fell] <tsv> [<previous tsv>]"

die() {
  echo "coverage: $1" >&2
  exit 2
}

# One Go profile block per line: "<import path>/<file>.go:<l>.<c>,<l>.<c> <statements> <count>".
go_rows() {
  local profile="$1" module="$2" generated="$3"
  LC_ALL=C awk -v module="$module/" -v generated="$generated" '
    BEGIN { while ((getline f < generated) > 0) skip[f] = 1 }
    NR == 1 { if ($0 !~ /^mode: (set|count|atomic)$/) bad = 1; next }
    $0 !~ /^[^ ]+\.go:[0-9]+\.[0-9]+,[0-9]+\.[0-9]+ [0-9]+ [0-9]+$/ { bad = 1; next }
    {
      block = $1
      file = substr(block, 1, index(block, ".go:") + 2)
      if (index(file, module) != 1) { bad = 1; next }
      rel = "apps/api/" substr(file, length(module) + 1)
      if (rel in skip) next
      pkg = rel
      sub(/\/[^\/]*$/, "", pkg)
      # A block listed twice (one per test binary that covers it) counts once, covered if any run hit it.
      if (!(block in stmts)) { stmts[block] = $2; owner[block] = pkg }
      if ($3 > 0) hit[block] = 1
    }
    END {
      if (bad || NR == 0) exit 3
      for (b in stmts) {
        p = owner[b]; total[p] += stmts[b]; all += stmts[b]
        if (b in hit) { covered[p] += stmts[b]; allc += stmts[b] }
      }
      for (p in total) printf "%s\t%d\t%d\n", p, covered[p], total[p]
      if (all > 0) printf "apps/api/...\t%d\t%d\n", allc, all
    }
  ' "$profile"
}

collect() {
  local out="$1" log module generated pjs pj ws kind name summary row rows=""
  command -v go >/dev/null || die "go not found: run make setup"
  command -v pnpm >/dev/null || die "pnpm not found: run make setup"
  command -v node >/dev/null || die "node not found: run make setup"
  mkdir -p "$out"
  out="$(cd "$out" && pwd)"
  rm -rf "${out:?}/go.out" "$out/coverage.tsv" "$out/ws"
  log="$out/test.log"

  if ! (cd "$ROOT/apps/api" && go test -short -count=1 -coverprofile="$out/go.out" ./...) >"$log" 2>&1; then
    cat "$log" >&2
    echo "coverage: go test failed in apps/api (make test-go shows the same failure)" >&2
    exit 1
  fi
  module="$(cd "$ROOT/apps/api" && go list -m)" || die "go list -m failed in apps/api: run make setup"
  generated="$out/generated.txt"
  # git grep exits 1 when nothing matches; anything above that is a git error.
  git -C "$ROOT" grep -l --untracked -E '^// Code generated .* DO NOT EDIT\.$' -- 'apps/api/*.go' >"$generated" ||
    (($? == 1)) || die "git grep failed listing generated Go files"
  rows="$(go_rows "$out/go.out" "$module" "$generated")" || die "unparseable Go coverage profile $out/go.out"
  [[ -n "$rows" ]] || die "no Go packages in $out/go.out"

  pjs="$(git -C "$ROOT" ls-files -co --exclude-standard -- 'apps/*/package.json' 'packages/*/package.json')" ||
    die "git ls-files failed listing JS workspaces"
  for pj in $pjs; do
    ws="$(dirname "$pj")"
    # 0: a coverage script; 1: neither test nor coverage; 4: a test script only.
    node -e 'const s = require(process.argv[1]).scripts ?? {}; process.exit(s.coverage ? 0 : s.test ? 4 : 1)' \
      "$ROOT/$pj" && kind=0 || kind=$?
    case "$kind" in
      0) ;;
      1)
        rows="$rows"$'\n'"$ws"$'\t-\t-'
        continue
        ;;
      4) die "$ws has a test script but no coverage script: add \"coverage\": \"vitest run --coverage\" to $pj (apps/web/AGENTS.md)" ;;
      *) die "cannot read the scripts of $pj" ;;
    esac
    name="$out/ws/$(echo "$ws" | tr / _)"
    if ! (cd "$ROOT/$ws" && pnpm run --silent coverage --coverage.provider=v8 --coverage.reporter=json-summary \
      --coverage.reportsDirectory="$name") >"$log" 2>&1; then
      cat "$log" >&2
      echo "coverage: pnpm run coverage failed in $ws (make test-web shows a test failure; otherwise run make setup)" >&2
      exit 1
    fi
    summary="$name/coverage-summary.json"
    row="$(node -e '
      const s = require(process.argv[1]).total.statements;
      if (!Number.isInteger(s.covered) || !Number.isInteger(s.total)) process.exit(3);
      process.stdout.write(`${s.covered}\t${s.total}`);
    ' "$summary" 2>/dev/null)" || die "no istanbul json-summary at $summary (the coverage script of $ws must write one)"
    rows="$rows"$'\n'"$ws"$'\t'"$row"
  done

  printf '%s\n' "$rows" | LC_ALL=C sort >"$out/coverage.tsv"
}

# valid <tsv>: every row is a package name and two counts (or "-\t-"); the previous tsv comes from an
# artifact of an earlier run, so it is parsed, not trusted.
valid() {
  [[ -s "$1" ]] || die "no coverage rows in $1"
  LC_ALL=C awk -F'\t' '
    NF != 3 || $1 !~ /^[A-Za-z0-9_.@\/-]+$/ { exit 1 }
    !(($2 ~ /^[0-9]+$/ && $3 ~ /^[0-9]+$/ && $2 + 0 <= $3 + 0) || ($2 == "-" && $3 == "-")) { exit 1 }
  ' "$1" || die "unparseable coverage file $1 (expected <package>\\t<covered>\\t<statements> per line)"
}

report() {
  local fell=0 cur prev
  if [[ "${1:-}" == --fell ]]; then
    fell=1
    shift
  fi
  (($# >= 1 && $# <= 2)) || die "$usage"
  cur="$1"
  prev="${2:-}"
  ((fell == 0)) || [[ -n "$prev" ]] || die "report --fell needs a previous tsv"
  valid "$cur"
  [[ -z "$prev" ]] || valid "$prev"
  LC_ALL=C awk -F'\t' -v fell="$fell" -v hasprev="$([[ -n "$prev" ]] && echo 1 || echo 0)" '
    function pct(c, t) { return t > 0 ? sprintf("%.1f", 100 * c / t) : "" }
    FNR == 1 { file++ }
    hasprev && file == 1 { if ($3 != "-") before[$1] = pct($2, $3); next }
    {
      now = $3 == "-" ? "" : pct($2, $3)
      shown = $3 == "-" ? "no tests" : (now == "" ? "no statements" : now "%")
      change = ""
      if (hasprev && now != "") change = ($1 in before && before[$1] != "") ? sprintf("%+.1f", now - before[$1]) : "new"
      # A workspace that had coverage and now has no tests fell all the way.
      else if (hasprev && $3 == "-" && ($1 in before) && before[$1] != "") change = sprintf("%+.1f", -before[$1])
      if (change == "-0.0" || change == "+0.0") change = "0.0"
      if (fell && !(change ~ /^-/)) next
      row = sprintf("| %s | %s | %s |", $1, $3 == "-" ? "" : $3, shown)
      rows = rows (hasprev ? row " " change " |" : row) "\n"
    }
    END {
      if (rows == "") exit
      if (fell) print "### Coverage fell since the last drift run (unit tests, statements)\n"
      else print "### Coverage per package (unit tests, statements; a trend, not a gate)\n"
      print hasprev ? "| Package | Statements | Coverage | Change (pp) |\n|---|---:|---:|---:|" : "| Package | Statements | Coverage |\n|---|---:|---:|"
      printf "%s", rows
    }
  ' ${prev:+"$prev"} "$cur"
}

case "${1:-}" in
  collect)
    (($# == 2)) || die "$usage"
    collect "$2"
    ;;
  report)
    shift
    report "$@"
    ;;
  *) die "$usage" ;;
esac
