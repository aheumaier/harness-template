#!/usr/bin/env bash
# CI timing rows (ADR 0013, ADR 0032): ci.yml's harness-timings-check-all artifacts, for the nightly latency
# report (harness-nfr.yml, nfr-report.sh) and the weekly rule-fires ledger (drift.yml, rule-fires.sh).
#
#   scripts/harness/ci-timings.sh retention                the artifact's retention-days in ci.yml (one source)
#   scripts/harness/ci-timings.sh download <dir> <days>    every unexpired artifact created in the last <days>
#                                                          days, as <dir>/<artifact-id>/timings.jsonl
#
# download lists the artifacts once (gh api, paginated) and downloads each by id: no call per ci.yml run, and
# an artifact id names one upload, so rule-fires.sh merges each once. It needs gh with a token that can read
# actions (GH_TOKEN). Prints "downloaded N of M" on stderr and exits 1 when the list or any download failed,
# so the caller can say in its report that the rows are incomplete; exits 2 on a usage error.
set -euo pipefail

usage() {
  echo "usage: ci-timings.sh retention | download <dir> <days>" >&2
  exit 2
}
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
name=harness-timings-check-all

case "${1-}" in
  retention)
    (($# == 1)) || usage
    days="$(awk -v want="name: $name" '
    { line = $0; gsub(/^[ \t]+|[ \t]+$/, "", line) }
    line == want { found = 1; next }
    found && line ~ /^retention-days:/ { sub(/^retention-days:[ \t]*/, "", line); sub(/[ \t]*#.*$/, "", line); print line; exit }
    found && line ~ /^- / { exit }
  ' "$ROOT/.github/workflows/ci.yml")"
    [[ "$days" =~ ^[0-9]+$ ]] || {
      echo "ci-timings: no retention-days for $name in .github/workflows/ci.yml" >&2
      exit 2
    }
    echo "$days"
    ;;
  download)
    (($# == 3)) && [[ "$3" =~ ^[0-9]+$ ]] || usage
    dir="$2"
    mkdir -p "$dir"
    since=$(($(date -u +%s) - $3 * 86400))
    if ! ids="$(gh api --paginate "repos/{owner}/{repo}/actions/artifacts?name=$name&per_page=100" \
      --jq ".artifacts[] | select(.expired | not) | select((.created_at | fromdateiso8601) >= $since) | .id")"; then
      echo "ci-timings: listing the $name artifacts failed" >&2
      exit 1
    fi
    total=0 ok=0
    for id in $ids; do
      [[ "$id" =~ ^[0-9]+$ ]] || continue
      total=$((total + 1))
      if gh api "repos/{owner}/{repo}/actions/artifacts/$id/zip" >"$dir/$id.zip" &&
        unzip -q -o "$dir/$id.zip" -d "$dir/$id"; then
        ok=$((ok + 1))
      else
        echo "ci-timings: downloading artifact $id failed" >&2
      fi
      rm -f "$dir/$id.zip"
    done
    echo "ci-timings: downloaded $ok of $total $name artifacts" >&2
    ((ok == total)) || exit 1
    ;;
  *) usage ;;
esac
