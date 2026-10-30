#!/usr/bin/env bash
# Upstream spec drift (rules.md#api-upstream-no-breaking-change, ADR 0037): for each vendored upstream spec in
# UPSTREAM_SPECS (scripts/harness/layout.env), fetches the head of its source repo and runs oasdiff breaking with
# the vendored copy as base and the source's spec at head as revision. Never changes anything.
#
#   scripts/harness/upstream-drift.sh        (make upstream-drift; drift.yml, weekly)
#
# An entry is <vendored spec>@<source repo>@<spec path in the source>@<revision>: the source is a host path such
# as github.com/acme/rooms (fetched as https://github.com/acme/rooms) or an absolute local repo path (tests and
# fixtures only: CI has no such repo); the revision is the commit the copy was taken at, for people: the check
# never reads it. The entry checks below mirror layout.go's (make layout-check, in make check), since drift.yml
# runs this without make check; apps/api/tools/harness/testdata/upstream-spec-vectors.txt holds both to the same
# rule. Each source is fetched once, shallow, without a prompt, into a throwaway bare repo with sync-check.sh's
# limits, https or a local path only (the network: never in make check or check-all). Anonymous in CI; locally
# your git credential helpers apply.
#
# Exit 0: no entries, or no upstream spec breaks its vendored copy (silent). 1: drift, per spec a line
#   <vendored spec>: <n> breaking change(s) in <source>/<path> at <head> since vendored <revision> [api-upstream-no-breaking-change]
# then oasdiff's findings (the source's file as <source>@<head>:<path>), or a line for a spec gone upstream.
# 2: cannot check an entry (malformed or refused, a vendored copy that is not a file, a failed or timed-out
# fetch, a $ref that is not internal per `.bin/harness spec-refs`, a spec that does not parse or oasdiff cannot
# load) or any unexpected error; the other entries are still checked, and the run fails closed, never passes.
set -Eeuo pipefail

RULE=api-upstream-no-breaking-change
die() {
  echo "upstream-drift: $1 [$RULE]" >&2
  exit 2
}
trap 'die "unexpected failure at line $LINENO (exit $?)"' ERR

HERE="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=layout.sh
source "$HERE/layout.sh" || exit 2 # UPSTREAM_SPECS
cd "$HERE/../.."
((${#UPSTREAM_SPECS[@]} > 0)) || exit 0 # vendors nothing: no network at all
command -v oasdiff >/dev/null 2>&1 || die "oasdiff not found: run make setup (it installs the pinned one into .bin/)"
HARNESS="${HARNESS:-$PWD/.bin/harness}" # spec-refs; make upstream-drift builds it
[[ -x "$HARNESS" ]] || die "$HARNESS not found: run make setup (or make .bin/harness)"
# The temp repos are addressed with --git-dir; none of the caller's repo overrides may leak into them.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR

tmp="$(mktemp -d "${TMPDIR:-/tmp}/upstream-drift.XXXXXX")"
trap 'rm -rf "$tmp"' EXIT
limit=()
if command -v timeout >/dev/null 2>&1; then limit=(timeout 120); fi

# Sources fetched so far (bash 3.2: no associative arrays): sources[i] was fetched into $tmp/git$i, head heads[i]
# ("" when the fetch failed, its reason in fetch_errors[i]).
sources=()
heads=()
fetch_errors=()
fetch() { # <source> → sets i to its index, fetching it first if new
  local url="$1" status=0
  for ((i = 0; i < ${#sources[@]}; i++)); do [[ "${sources[i]}" != "$1" ]] || return 0; done
  sources+=("$1") heads+=('') fetch_errors+=('')
  [[ "$url" == /* ]] || url="https://$url"
  git --git-dir="$tmp/git$i" init -q --bare
  # Read-only: no credential prompt, only the two transports built above (https, a local path), and no hang (as
  # in sync-check.sh: a slow transfer aborts, 120 s in all where timeout(1) exists).
  env LC_ALL=C GIT_TERMINAL_PROMPT=0 GIT_ALLOW_PROTOCOL=file:https \
    GIT_HTTP_LOW_SPEED_LIMIT=1000 GIT_HTTP_LOW_SPEED_TIME=30 \
    ${limit[@]+"${limit[@]}"} git --git-dir="$tmp/git$i" fetch -q --depth 1 --no-tags -- "$url" HEAD \
    2>"$tmp/fetch$i.err" || status=$?
  if ((status == 124)); then
    fetch_errors[i]="fetching HEAD of $url timed out after 120 s"
  elif ((status != 0)); then
    fetch_errors[i]="cannot fetch HEAD of $url: $(head -n 3 "$tmp/fetch$i.err" | tr '\n' ' ')(no prompt, anonymous in CI: check a private source locally, where your credential helpers apply)"
  else
    heads[i]="$(git --git-dir="$tmp/git$i" rev-parse --verify -q FETCH_HEAD || true)"
    [[ -n "${heads[i]}" ]] || fetch_errors[i]="$url has no HEAD commit"
  fi
}

# entry_problem <entry>: why the entry is refused, or nothing (layout.go's upstreamEntryProblem: keep in step).
entry_problem() {
  local ats="${1//[!@]/}" vendored="${1%%@*}" rest="${1#*@}" src path revision
  src="${rest%%@*}" rest="${rest#*@}"
  path="${rest%%@*}" revision="${rest#*@}"
  if [[ ${#ats} -ne 3 || -z "$vendored" || -z "$src" || -z "$path" || -z "$revision" ]]; then
    echo "not <vendored spec>@<source repo>@<spec path in the source>@<revision>"
  elif [[ "$vendored" == /* || "/$vendored/" == */../* ]]; then
    echo "vendored spec $vendored is not a path inside this repo"
  elif [[ "$src" != /* && ! "$src" =~ ^[A-Za-z0-9][A-Za-z0-9.-]*(/[A-Za-z0-9_.+-]+)+$ ]] || [[ "/$src/" == */../* ]]; then
    echo "source $src is neither a host path (github.com/<owner>/<repo>) nor an absolute local path"
  elif [[ "$path" == /* || "/$path/" == */../* || "/$path/" == */./* || "/$path/" == *//* ]]; then
    echo "spec path $path is not a plain path inside the source"
  elif [[ ! "$revision" =~ ^[0-9a-f]{7,40}$ ]]; then
    echo "revision $revision is not a 7 to 40 digit hex commit"
  fi
}

# refs_problem <status of harness spec-refs>: why the spec cannot be compared.
refs_problem() {
  if (($1 == 1)); then echo 'a $ref that is not internal (above): oasdiff would follow it'; else echo 'does not parse (above)'; fi
}

cannot=0 drift=0 n=0
skip() { # <entry> <why>
  echo "UPSTREAM_SPECS: $1: cannot check: $2 [$RULE]" >&2
  cannot=1
}
for entry in "${UPSTREAM_SPECS[@]}"; do
  n=$((n + 1))
  problem="$(entry_problem "$entry")"
  if [[ -n "$problem" ]]; then
    skip "$entry" "$problem"
    continue
  fi
  vendored="${entry%%@*}" rest="${entry#*@}"
  src="${rest%%@*}" rest="${rest#*@}"
  path="${rest%%@*}" revision="${rest#*@}"
  if [[ ! -f "$vendored" ]]; then
    skip "$entry" "no such file $vendored (the vendored copy)"
    continue
  fi
  status=0
  "$HARNESS" spec-refs "$vendored" >&2 || status=$?
  if ((status != 0)); then
    skip "$entry" "$vendored: $(refs_problem "$status")"
    continue
  fi

  fetch "$src"
  head="${heads[i]}"
  if [[ -z "$head" ]]; then
    skip "$entry" "${fetch_errors[i]}"
    continue
  fi
  if ! git --git-dir="$tmp/git$i" cat-file -e "$head:$path" 2>/dev/null; then
    echo "$vendored: $path is gone from $src at ${head:0:12} (vendored at ${revision:0:12}): find its new home or drop the entry [$RULE]"
    drift=1
    continue
  fi
  theirs="$tmp/spec$n/$path" # oasdiff picks the parser by extension; the name is mapped back below
  mkdir -p "$(dirname "$theirs")"
  git --git-dir="$tmp/git$i" show "$head:$path" >"$theirs"
  theirs_dir="$tmp/spec$n/" theirs_name="$src@${head:0:12}:"
  status=0
  "$HARNESS" spec-refs "$theirs" >"$tmp/refs" 2>&1 || status=$?
  if ((status != 0)); then
    refs="$(cat "$tmp/refs")"
    printf '%s\n' "${refs//"$theirs_dir"/$theirs_name}" >&2
    skip "$entry" "$src's $path: $(refs_problem "$status")"
    continue
  fi
  status=0
  oasdiff breaking "$vendored" "$theirs" --fail-on ERR --format singleline >"$tmp/out" 2>&1 || status=$?
  case "$status" in
    0) ;;
    1)
      out="$(grep -v '^[[:space:]]*$' "$tmp/out" || true)"
      out="${out//"$theirs_dir"/$theirs_name}"
      echo "$vendored: $(grep -c '^error ' <<<"$out" || true) breaking change(s) in $src/$path at ${head:0:12} since vendored ${revision:0:12}: migrate, then refresh the copy and its revision [$RULE]"
      printf '%s\n' "$out"
      drift=1
      ;;
    *)
      skip "$entry" "oasdiff cannot compare (exit $status): $(head -n 3 "$tmp/out" | tr '\n' ' ')"
      ;;
  esac
done
((cannot == 0)) || exit 2
((drift == 0)) || exit 1
exit 0
