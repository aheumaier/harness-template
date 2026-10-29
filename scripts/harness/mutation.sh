#!/usr/bin/env bash
# Mutation testing of the Go and TS source files changed recently (docs/adr/0023-mutation-testing-is-a-drift-report.md,
# docs/harness/ci.md#mutation-testing): gremlins for apps/api, Stryker for each JS workspace. A trend, never a
# gate: it fails only when it cannot measure (missing tool, failing tests, shallow history, unparseable input,
# a loosened config), never on a score. A report, not a sensor: no rule id, never in make check (slow, and the
# window reads the clock) nor in make check-all; drift.yml runs it weekly.
#
#   scripts/harness/mutation.sh files <since>
#       the files in scope, "<package>\t<path>" per line: the mutable source (below) that differs between HEAD
#       and the newest first-parent commit of HEAD at or before <since> (any date git rev-list --before takes;
#       no such commit: all of HEAD). A tree diff, not git log: a branch committed before <since> and merged
#       after it is in scope.
#   scripts/harness/mutation.sh collect <dir> <since>                   (make mutation)
#       mutates them and writes <dir>/mutants.tsv
#   scripts/harness/mutation.sh report [--scores] <tsv> <since>
#       Markdown: score per package, then every surviving mutant. --scores prints the score table only, without
#       the surviving mutants (make mutation-scores: drift.yml's weekly issue entry; the job summary has them).
#
# Mutable source:
#   - package apps/api: apps/api/**/*.go of the apps/api module (not a nested module such as tools/harness),
#     except *_test.go, testdata/ and files marked "Code generated ... DO NOT EDIT."; mutated by gremlins with
#     scripts/harness/gremlins.yaml (what is measured is fixed in protected config: no other config file and
#     no GREMLINS_* variable is read) and GOFLAGS=-short (the test set of make test-go and make coverage);
#   - package <workspace> (apps/*, packages/*): <workspace>/src/**/*.{ts,tsx}, except *.test.*, *.spec.*,
#     *.d.ts and src/gen/ (generated); mutated by the workspace's `mutation` script, run as `pnpm run mutation
#     stryker.config.json --mutate <files> --reporters json --cleanTempDir always` (Stryker; the report is read
#     from <workspace>/reports/mutation/mutation.json). A workspace with neither a `test` nor a `mutation`
#     script reads "no tests"; one with `test` only fails: its mutants would be missing, not survived. Its
#     stryker.config.json (protected) must set neither thresholds.break nor excludedMutations.
#   File names in scope are limited to letters, digits and _.@+$[]/- (they become regexes, globs and a
#   comma-separated list); any other name of a mutable source file fails the run.
#
# mutants.tsv, one mutant per line, sorted: "<package>\t<path>:<line>:<column>\t<mutator>\t<status>", status
# killed|timeout|survived|nocoverage|invalid (a mutant that does not compile or run: not counted) or, for a
# file in a workspace without tests, "<package>\t<path>\t-\tnotests".
#
# Exit 1 when tests fail, 2 on bad usage, a missing tool, shallow history, unparseable input or a loosened
# config.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
usage="usage: mutation.sh files <since> | collect <dir> <since> | report [--scores] <tsv> <since>"
API=apps/api
GREMLINS_CONFIG="$ROOT/scripts/harness/gremlins.yaml"
# An allowed file name, as an ERE (awk) and a JS regex.
NAME_ERE='^[]A-Za-z0-9_.@+$[/-]+$'
NAME_JS='^[\]A-Za-z0-9_.@+$[\/-]+$'

die() {
  echo "mutation: $1" >&2
  exit 2
}

# stryker_setup <workspace>: what a workspace with tests needs for Stryker.
stryker_setup() {
  printf '%s' "add \"mutation\": \"stryker run\" to $1/package.json, the devDependencies @stryker-mutator/core and" \
    " @stryker-mutator/vitest-runner at the versions apps/web/package.json pins (pnpm -C $1 add -D --save-exact ...)" \
    " and a stryker.config.json copied from apps/web (apps/web/AGENTS.md)"
}

# files <since>: see the header. File names come from history, so they are checked, not trusted.
files() {
  local since="$1" shallow base changed nested generated
  shallow="$(git -C "$ROOT" rev-parse --is-shallow-repository)" || die "git rev-parse failed in $ROOT"
  [[ "$shallow" == false ]] ||
    die "shallow clone: the history before $since is incomplete (CI: actions/checkout with fetch-depth: 0; locally: git fetch --unshallow)"
  base="$(git -C "$ROOT" rev-list -1 --first-parent --before="$since" HEAD)" || die "git rev-list --before=\"$since\" failed"
  # No commit that old: every file of HEAD is new.
  [[ -n "$base" ]] || base="$(git -C "$ROOT" hash-object -t tree /dev/null)" || die "git hash-object failed"
  changed="$(git -C "$ROOT" -c core.quotePath=false diff --name-only --no-renames --diff-filter=d "$base" HEAD)" ||
    die "git diff $base HEAD failed"
  nested="" generated=""
  # git ls-files --others and git grep --untracked warn about a pathspec directory that does not exist.
  if [[ -d "$ROOT/$API" ]]; then
    # Nested Go modules below apps/api (tools/analyzers, tools/harness) are not the apps/api module.
    nested="$(git -C "$ROOT" ls-files --cached --others --exclude-standard -- "$API/*/go.mod")" ||
      die "git ls-files failed listing nested Go modules"
    generated="$(git -C "$ROOT" -c core.quotePath=false grep -l --untracked -E '^// Code generated .* DO NOT EDIT\.$' -- "$API/*.go")" ||
      (($? == 1)) || die "git grep failed listing generated Go files"
  fi
  {
    printf '%s\n' "$nested" | sed 's/^/N /; s/go\.mod$//'
    printf '%s\n' "$generated" | sed 's/^/G /'
    printf '%s\n' "$changed" | sed 's/^/C /'
  } | LC_ALL=C awk -v api="$API/" -v name="$NAME_ERE" '
    # The whole rest of the line, not $2: a name with a space is one name.
    { t = substr($0, 1, 1); f = substr($0, 3) }
    f == "" { next }
    t == "N" { nested[f] = 1; next }
    t == "G" { generated[f] = 1; next }
    {
      # git still quotes a name with a control character or a double quote: filter on what is inside.
      g = f
      if (g ~ /^".*"$/) g = substr(g, 2, length(g) - 2)
      if (index(g, api) == 1 && g ~ /\.go$/) {
        if (g ~ /_test\.go$/ || g ~ /\/testdata\// || (g in generated)) next
        for (n in nested) if (index(g, n) == 1) next
        pkg = substr(api, 1, length(api) - 1)
      } else if (g ~ /^(apps|packages)\/[^\/]+\/src\/.+\.tsx?$/) {
        if (g ~ /\.(test|spec)\.tsx?$/ || g ~ /\.d\.ts$/ || g ~ /^[^\/]+\/[^\/]+\/src\/gen\//) next
        split(g, p, "/")
        pkg = p[1] "/" p[2]
      } else next
      if (f !~ name) { bad = f; exit }
      print pkg "\t" f
    }
    END {
      if (bad == "") exit
      print "mutation: unexpected file name " bad ": rename it to letters, digits and _.@+$[]/- only (gremlins and Stryker get it as a regex, a glob and a comma-separated list)" > "/dev/stderr"
      exit 3
    }
  ' | LC_ALL=C sort || exit 2
}

# gremlins_rows <gremlins.json> <selection>: its mutants as mutants.tsv rows; file names are relative to
# apps/api and must be in the selection ("<path>" per line).
gremlins_rows() {
  node -e '
    const [file, sel, name] = process.argv.slice(1);
    const fail = (m) => { console.error(`mutation: ${m} in ${file}`); process.exit(3); };
    const j = require(file);
    const allowed = new Set(sel.split("\n"));
    const status = { KILLED: "killed", "TIMED OUT": "timeout", LIVED: "survived", "NOT COVERED": "nocoverage",
      "NOT VIABLE": "invalid", SKIPPED: "invalid" };
    if (!Array.isArray(j.files)) fail("no files array");
    for (const f of j.files) {
      const path = `apps/api/${f.file_name}`;
      if (typeof f.file_name !== "string" || !new RegExp(name).test(f.file_name) || !path.endsWith(".go")) fail(`unexpected file_name ${f.file_name}`);
      if (!allowed.has(path)) fail(`a file outside the selection: ${path}`);
      for (const m of f.mutations ?? []) {
        if (!(m.status in status) || !/^[A-Z_]+$/.test(m.type) || !Number.isInteger(m.line) || !Number.isInteger(m.column)) fail(`unexpected mutation ${JSON.stringify(m)}`);
        console.log(`apps/api\t${path}:${m.line}:${m.column}\t${m.type}\t${status[m.status]}`);
      }
    }
  ' "$1" "$2" "$NAME_JS"
}

# stryker_rows <workspace> <mutation.json> <selection>: its mutants as mutants.tsv rows
# (mutation-testing-report-schema); file names are relative to the workspace and must be in the selection.
stryker_rows() {
  node -e '
    const [ws, file, sel, name] = process.argv.slice(1);
    const fail = (m) => { console.error(`mutation: ${m} in ${file}`); process.exit(3); };
    const j = require(file);
    const allowed = new Set(sel.split("\n"));
    const status = { Killed: "killed", Timeout: "timeout", Survived: "survived", NoCoverage: "nocoverage",
      CompileError: "invalid", RuntimeError: "invalid", Ignored: "invalid" };
    if (typeof j.files !== "object" || j.files === null) fail("no files object");
    for (const [n, f] of Object.entries(j.files)) {
      const path = `${ws}/${n}`;
      if (!new RegExp(name).test(n) || !Array.isArray(f.mutants)) fail(`unexpected file ${n}`);
      if (!allowed.has(path)) fail(`a file outside the selection: ${path}`);
      for (const m of f.mutants) {
        const s = m.location?.start;
        if (!(m.status in status) || !/^[A-Za-z]+$/.test(m.mutatorName) || !Number.isInteger(s?.line) || !Number.isInteger(s?.column)) fail(`unexpected mutant ${JSON.stringify(m)}`);
        console.log(`${ws}\t${path}:${s.line}:${s.column}\t${m.mutatorName}\t${status[m.status]}`);
      }
    }
  ' "$1" "$2" "$3" "$NAME_JS"
}

# loosened <stryker.config.json>: the settings it must not have, comma-separated; exit 3 when unparseable.
loosened() {
  node -e '
    let c;
    try { c = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8")); } catch { process.exit(3); }
    const set = [];
    if (c.thresholds?.break != null) set.push("thresholds.break");
    if (c.excludedMutations != null) set.push("excludedMutations");
    if (c.mutator?.excludedMutations != null) set.push("mutator.excludedMutations");
    console.log(set.join(", "));
  ' "$1"
}

collect() {
  local out="$1" since="$2" log sel ws f v kind report rel rows="" mutate gofiles mine bad
  local -a exclude
  command -v gremlins >/dev/null || die "gremlins not found: run make setup"
  command -v pnpm >/dev/null || die "pnpm not found: run make setup"
  command -v node >/dev/null || die "node not found: run make setup"
  sel="$(files "$since")"
  mkdir -p "$out"
  out="$(cd "$out" && pwd)"
  rm -rf "${out:?}/mutants.tsv" "$out/gremlins.json" "$out/ws"
  log="$out/run.log"

  mine="$(printf '%s\n' "$sel" | LC_ALL=C awk -F'\t' -v api="$API" '$1 == api { print $2 }')"
  if [[ -n "$mine" ]]; then
    [[ -f "$GREMLINS_CONFIG" ]] || die "no $GREMLINS_CONFIG: restore it (git checkout -- scripts/harness/gremlins.yaml)"
    # gremlins takes a module, not a file list: exclude every other Go file of the module.
    gofiles="$(git -C "$ROOT/$API" ls-files --cached --others --exclude-standard -- '*.go')" ||
      die "git ls-files failed listing the Go files of $API"
    exclude=()
    while IFS= read -r f; do
      [[ -n "$f" ]] || continue
      grep -qxF "$API/$f" <<<"$mine" && continue
      exclude+=(-E "^$(printf '%s' "$f" | sed 's/[][\.*^$+?(){}|]/\\&/g')\$")
    done <<<"$gofiles"
    if ! (
      cd "$ROOT/$API" &&
        for v in $(compgen -e | grep '^GREMLINS_' || true); do unset "$v"; done &&
        GOFLAGS=-short gremlins unleash --config "$GREMLINS_CONFIG" --output "$out/gremlins.json" \
          ${exclude[@]+"${exclude[@]}"} .
    ) >"$log" 2>&1; then
      cat "$log" >&2
      echo "mutation: gremlins failed in $API (cd $API && go test -short ./... shows a test failure; otherwise run make setup)" >&2
      exit 1
    fi
    rows="$(gremlins_rows "$out/gremlins.json" "$mine")" || die "unparseable gremlins report $out/gremlins.json"
  fi

  for ws in $(printf '%s\n' "$sel" | LC_ALL=C awk -F'\t' -v api="$API" 'NF && $1 != api { print $1 }' | LC_ALL=C sort -u); do
    # 10: a mutation script; 11: neither test nor mutation; 12: a test script only; anything else: unreadable.
    kind=0
    node -e '
      let s;
      try { s = require(process.argv[1]).scripts ?? {}; } catch { process.exit(2); }
      process.exit(s.mutation ? 10 : s.test ? 12 : 11);
    ' "$ROOT/$ws/package.json" || kind=$?
    case "$kind" in
      10) ;;
      11)
        rows="$rows"$'\n'"$(printf '%s\n' "$sel" | LC_ALL=C awk -F'\t' -v ws="$ws" '$1 == ws { print ws "\t" $2 "\t-\tnotests" }')"
        continue
        ;;
      12) die "$ws has a test script but no mutation script: $(stryker_setup "$ws")" ;;
      *) die "cannot read $ws/package.json: fix its JSON" ;;
    esac
    [[ -f "$ROOT/$ws/stryker.config.json" ]] || die "$ws has no stryker.config.json: $(stryker_setup "$ws")"
    bad="$(loosened "$ROOT/$ws/stryker.config.json")" || die "unparseable $ws/stryker.config.json: fix its JSON"
    [[ -z "$bad" ]] ||
      die "$ws/stryker.config.json sets $bad: mutation testing measures every mutant and gates nothing (ADR 0023); remove it"
    # Stryker reads --mutate as globs: [ and ] are escaped.
    mutate="$(printf '%s\n' "$sel" | LC_ALL=C awk -F'\t' -v ws="$ws/" '$1 "/" == ws { f = substr($2, length(ws) + 1); gsub(/[][]/, "\\\\&", f); print f }' | paste -sd, -)"
    report="$ROOT/$ws/reports/mutation/mutation.json"
    rm -f "$report"
    if ! (cd "$ROOT/$ws" && pnpm run --silent mutation stryker.config.json --mutate "$mutate" --reporters json \
      --cleanTempDir always) >"$log" 2>&1; then
      cat "$log" >&2
      LC_ALL=C grep -Eq 'stryker: (command )?not found' "$log" && die "$ws cannot run stryker: $(stryker_setup "$ws")"
      echo "mutation: pnpm run mutation failed in $ws (make test-web shows a test failure; otherwise run make setup)" >&2
      exit 1
    fi
    [[ -f "$report" ]] || die "no Stryker json report at $report (the mutation script of $ws must be stryker run)"
    mkdir -p "$out/ws"
    rel="$out/ws/$(echo "$ws" | tr / _).json"
    mv "$report" "$rel"
    rmdir "$ROOT/$ws/reports/mutation" "$ROOT/$ws/reports" 2>/dev/null || true
    rows="$rows"$'\n'"$(stryker_rows "$ws" "$rel" "$(printf '%s\n' "$sel" | LC_ALL=C awk -F'\t' -v ws="$ws" '$1 == ws { print $2 }')")" ||
      die "unparseable Stryker report $rel"
  done

  printf '%s\n' "$rows" | LC_ALL=C awk 'NF' | LC_ALL=C sort >"$out/mutants.tsv"
}

report() {
  local scores=0
  if [[ "${1:-}" == --scores ]]; then
    scores=1
    shift
  fi
  (($# == 2)) || die "$usage"
  local tsv="$1" since="$2"
  [[ -f "$tsv" ]] || die "no $tsv: run make mutation"
  LC_ALL=C awk -F'\t' -v name="$NAME_ERE" '
    { file = $2; sub(/:[0-9]+:[0-9]+$/, "", file) }
    NF != 4 || $1 !~ /^[A-Za-z0-9_.@\/-]+$/ || file !~ name { exit 1 }
    $3 !~ /^([A-Za-z_]+|-)$/ || $4 !~ /^(killed|timeout|survived|nocoverage|invalid|notests)$/ { exit 1 }
  ' "$tsv" || die "unparseable mutants file $tsv (expected <package>\\t<path:line:column>\\t<mutator>\\t<status> per line)"
  LC_ALL=C awk -F'\t' -v since="$since" -v scores="$scores" '
    {
      p = $1; file = $2; sub(/:[0-9]+:[0-9]+$/, "", file)
      if (!(p in order)) { order[p] = ++n; name[n] = p }
      if (!((p, file) in seen)) { seen[p, file] = 1; files[p]++ }
      count[p, $4]++
      if ($4 == "survived") { live = live "- `" $2 "`: " $3 "\n"; lived++ }
    }
    END {
      printf "### Mutation testing of the files changed since %s (a trend, not a gate)\n\n", since
      if (n == 0) { print "No mutable Go or TypeScript source file changed."; exit }
      print "Score: killed and timed-out mutants over all valid mutants (survived and not covered ones count against it)."
      print ""
      print "| Package | Files with mutants | Mutants | Killed | Timed out | Survived | No coverage | Score |"
      print "|---|---:|---:|---:|---:|---:|---:|---:|"
      for (i = 1; i <= n; i++) {
        p = name[i]
        if (count[p, "notests"]) { printf "| %s |  |  |  |  |  |  | no tests (%d files) |\n", p, files[p]; continue }
        det = count[p, "killed"] + count[p, "timeout"]
        all = det + count[p, "survived"] + count[p, "nocoverage"]
        printf "| %s | %d | %d | %d | %d | %d | %d | %s |\n", p, files[p], all, count[p, "killed"] + 0,
          count[p, "timeout"] + 0, count[p, "survived"] + 0, count[p, "nocoverage"] + 0,
          (all > 0 ? sprintf("%.1f%%", 100 * det / all) : "no mutants")
      }
      if (lived > 0 && !scores) printf "\n#### Surviving mutants (%d): a test covers the line but none fails when it changes\n\n%s", lived, live
    }
  ' "$tsv"
}

case "${1:-}" in
  files)
    (($# == 2)) || die "$usage"
    files "$2"
    ;;
  collect)
    (($# == 3)) || die "$usage"
    collect "$2" "$3"
    ;;
  report)
    shift
    report "$@"
    ;;
  *) die "$usage" ;;
esac
