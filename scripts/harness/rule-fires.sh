#!/usr/bin/env bash
# Rules that never fire (exec plan 0003 P4.4, ADR 0032, ci.md#rule-fires): a weekly drift report, never a gate.
#
#   scripts/harness/rule-fires.sh <ledger-in> <ledger-out> <retention-days> [<timings-dir>]    (weekly in drift.yml)
#
# run.sh writes each rule's finding count into its timings.jsonl row ("findings": lines naming the rule, at
# least 1 when the rule fired; only > 0 is used here). CI keeps those rows <retention-days> days (ci.yml's
# harness-timings-check-all artifact; drift.yml reads the number from ci.yml with ci-timings.sh retention), so
# the 90-day history lives in a ledger that each drift run carries to the next: <ledger-in> is the previous
# run's ("" starts tracking today), <ledger-out> the merged one. <timings-dir> holds one directory per
# artifact, <artifact-id>/timings.jsonl, as ci-timings.sh downloads them. Prints a Markdown section listing
# every rule of docs/harness/rules.md that ran in the window and had no finding in it, plus the rules with no
# run at all (not measured here); nothing while no rule has been tracked for the whole window. A human
# decides to keep or retire each listed rule; nothing here retires anything. One line on stderr says how many
# rows were merged from how many artifacts; the report says so too when new artifacts gave no row at all.
#
# Ledger (rule lines: one per rule of rules.md; a rule no longer there is dropped):
#   # rule-fires v2 merged_day=<day of this merge>
#   rule <rule-id> <since-day> <last-run-day|-> <last-fired-day|-> <runs-without-a-finding>
#   artifact <artifact-id> <merge-day>
# Days are days since the epoch (UTC). Each artifact is merged once: its id stays in the ledger until it has
# certainly expired (<retention-days> + 1 days after the merge), so overlapping downloads never count twice
# and an artifact uploaded after a merge (its run still going then) is merged the next week. Runs without a
# finding count the rows since the rule's last fire (or since tracking began). Rows come from PR runs and are
# untrusted: a row whose shape is not timing.sh's, or that starts after now (it would hide a rule into the
# future), is dropped; symlinks are never read.
#
# RULE_FIRES_NOW (epoch seconds; default now) and RULE_FIRES_DAYS (window; default 90) are for tests.
# Exits 2 on a usage error, an unreadable input, or a ledger or rules.md heading that does not parse.
set -euo pipefail

usage() {
  echo "usage: rule-fires.sh <ledger-in|\"\"> <ledger-out> <retention-days> [<timings-dir>]" >&2
  exit 2
}
(($# == 3 || $# == 4)) || usage
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
rules="$ROOT/docs/harness/rules.md"
ledger_in="$1" ledger_out="$2" retention="$3" dir="${4-}"
now="${RULE_FIRES_NOW-}"
[[ -n "$now" ]] || now="$(date -u +%s)"
days="${RULE_FIRES_DAYS-90}"
[[ "$now" =~ ^[0-9]+$ && "$days" =~ ^[0-9]+$ && "$retention" =~ ^[0-9]+$ ]] || usage
for f in "$rules" ${ledger_in:+"$ledger_in"}; do
  [[ -r "$f" && -f "$f" ]] || {
    echo "rule-fires: cannot read $f" >&2
    exit 2
  }
done
[[ -z "$dir" || -d "$dir" ]] || {
  echo "rule-fires: cannot read $dir" >&2
  exit 2
}
files=()
if [[ -n "$dir" ]]; then
  for f in "$dir"/*/timings.jsonl; do
    id="${f%/timings.jsonl}"
    id="${id##*/}"
    [[ "$id" =~ ^[0-9]+$ && -f "$f" && ! -L "$f" && ! -L "${f%/timings.jsonl}" ]] || continue
    files+=("$f")
  done
fi

LC_ALL=C awk -v now="$now" -v days="$days" -v retention="$retention" -v out="$ledger_out" \
  -v rules="$rules" -v ledger="$ledger_in" '
  function fail(msg) { print "rule-fires: " msg > "/dev/stderr"; failed = 1; exit 2 }
  function ago(d) { return (d == "-") ? "never" : (today - d == 0 ? "today" : (today - d) " days ago") }
  BEGIN {
    today = int(now / 86400); limit = now * 1000 + 999 # now has whole seconds; rows have ms
    # Same headings as fixtureclaims.go and integritycheck.go read; one that is not a rule id fails closed.
    while ((getline line < rules) > 0) {
      if (line !~ /^### /) continue
      id = substr(line, 5); gsub(/^[ \t]+|[ \t]+$/, "", id)
      if (id !~ /^[a-z0-9][a-z0-9-]*$/) fail(rules ": not a rule id in heading: " line)
      if (!(id in known)) { known[id] = 1; order[++n] = id }
    }
    close(rules)
    if (n == 0) fail("no rule ids (### headings) in " rules)
    merged = ""
    if (ledger != "") {
      ln = 0
      while ((getline line < ledger) > 0) {
        if (++ln == 1) {
          if (line !~ /^# rule-fires v2 merged_day=[0-9]+$/) fail(ledger ":1: not a rule-fires v2 ledger header")
          merged = substr(line, index(line, "=") + 1) + 0
          continue
        }
        if (line ~ /^artifact [0-9]+ [0-9]+$/) {
          split(line, f, " ")
          if (today - f[3] <= retention + 1) done[f[2]] = f[3]
          continue
        }
        if (line !~ /^rule [a-z0-9][a-z0-9-]* [0-9]+ ([0-9]+|-) ([0-9]+|-) [0-9]+$/) fail(ledger ":" ln ": not rule <rule-id> <since> <last-run> <last-fired> <runs> or artifact <id> <day>")
        split(line, f, " ")
        if (f[2] in known) { since[f[2]] = f[3]; lastrun[f[2]] = f[4]; lastfired[f[2]] = f[5]; quiet[f[2]] = f[6] }
      }
      close(ledger)
      if (ln == 0) fail(ledger ": empty ledger")
    }
  }
  FNR == 1 {
    art = FILENAME; sub(/\/timings\.jsonl$/, "", art); sub(/^.*\//, "", art)
    skip = (art in done)
    if (skip) old++; else { done[art] = today; fresh++ }
  }
  skip { next }
  # timing.sh rows that carry findings; anything else (hook rows, old rows, forged ones) is skipped.
  /^\{"sensor":"[A-Za-z0-9_.-]+","start_ms":[0-9]+,"ms":[0-9]+,"exit":[0-9]+,.*,"findings":\{[^{}]*\}\}$/ {
    s = $0; sub(/^\{"sensor":"[A-Za-z0-9_.-]+","start_ms":/, "", s); sub(/,.*$/, "", s)
    if (length(s) > 15 || s + 0 > limit) next
    body = $0; sub(/^.*,"findings":\{/, "", body); sub(/\}\}$/, "", body)
    k = split(body, kv, ",")
    for (i = 1; i <= k; i++) if (kv[i] !~ /^"[a-z0-9][a-z0-9-]*":[0-9]+$/ || length(kv[i]) > 120) next
    day = int((s + 0) / 86400000)
    rows++
    for (i = 1; i <= k; i++) {
      split(kv[i], p, ":"); id = p[1]; gsub(/"/, "", id)
      if (!(id in known)) continue
      if (!(id in lastrun) || lastrun[id] == "-" || day > lastrun[id]) lastrun[id] = day
      b = ++nb[id]; bday[id, b] = day; bfire[id, b] = (p[2] + 0 > 0)
      if (bfire[id, b] && (!(id in newfire) || day > newfire[id])) newfire[id] = day
    }
  }
  END {
    if (failed) exit 2
    printf "rule-fires: merged %d rows from %d new artifacts (%d already merged)\n", rows, fresh, old > "/dev/stderr"
    cutoff = today - days
    printf "# rule-fires v2 merged_day=%d\n", today > out
    for (i = 1; i <= n; i++) {
      id = order[i]
      if (!(id in since)) since[id] = today
      if (!(id in lastrun)) lastrun[id] = "-"
      if (!(id in lastfired)) lastfired[id] = "-"
      if (!(id in quiet)) quiet[id] = 0
      # A fire newer than the ledger one resets the count; then the new rows after the last fire add to it.
      from = lastfired[id]
      if ((id in newfire) && (from == "-" || newfire[id] > from)) { from = newfire[id]; lastfired[id] = from; quiet[id] = 0 }
      for (b = 1; b <= nb[id]; b++) if (from == "-" || bday[id, b] > from) quiet[id]++
      printf "rule %s %d %s %s %d\n", id, since[id], lastrun[id], lastfired[id], quiet[id] > out
      if (since[id] > cutoff) continue # not tracked for the whole window yet
      if (lastrun[id] == "-" || lastrun[id] <= cutoff) { unmeasured = unmeasured (unmeasured == "" ? "" : ", ") "`" id "`"; continue }
      if (lastfired[id] != "-" && lastfired[id] > cutoff) continue
      table = table sprintf("| `%s` | %s | %d | %s |\n", id, ago(lastfired[id]), quiet[id], ago(lastrun[id]))
    }
    for (a in done) printf "artifact %s %d\n", a, done[a] > out
    close(out)
    if (merged != "" && today - merged > retention)
      notes = notes sprintf("The previous ledger is from %d days ago, longer than the %d days ci.yml keeps timings: rows in between are lost, so a rule may show here that fired then.\n\n", today - merged, retention)
    if (fresh > 0 && rows == 0)
      notes = notes sprintf("No row merged from %d new ci.yml timing artifacts: every row was dropped (run.sh writes rows rule-fires.sh does not read, or they are not timing.sh rows), so this report is not current.\n\n", fresh)
    if (table == "" && unmeasured == "" && notes == "") exit 0
    printf "### Rules that never fired in %d days\n\n", days
    printf "%s", notes
    if (table != "") {
      printf "No finding in the ci.yml check-all runs of the last %d days (scripts/harness/rule-fires.sh, ADR 0032). For each one a human decides: keep it (it guards a rare mistake, or one agents stopped making because it exists) or retire it in a harness PR (rules.md entry, sensor, fixture). Never automated.\n\n", days
      printf "| Rule | Last fired | Runs without a finding | Last ran |\n|---|---|---|---|\n%s\n", table
    }
    if (unmeasured != "")
      printf "Not measured: no run in the ci.yml check-all timings of the last %d days (a hook, its own workflow, local only, or a convention), so no verdict: %s.\n\n", days, unmeasured
  }
' ${files[@]+"${files[@]}"} </dev/null
