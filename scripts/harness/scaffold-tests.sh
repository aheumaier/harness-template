#!/usr/bin/env bash
# The tests make new-domain scaffolded actually ran (NFR-B1, rule go-tests-pass): CI's generator probe
# (.github/workflows/ci.yml). A scaffold whose tests compile but skip, or whose test functions vanished, would
# still pass make test-integration; this fails unless every Test* function of the templates ran and passed and
# no test of the domain was skipped or failed.
#
#   scripts/harness/scaffold-tests.sh <name>          (make test-scaffold NAME=<name>, after make new-domain)
#
# Expected tests: each `func Test...` of scripts/templates/new-domain/*_test.go.tmpl with __PASCAL__ rendered as
# new-domain.sh renders it. Runs `go test -json -race -count=1 -tags=integration ./internal/<name>/...` in
# apps/api (-tags=integration builds the untagged unit tests too) against this worktree's Postgres: the caller
# loads .env.local (make test-scaffold does).
#
# Silent on success. Exit 1 when a test fails, is skipped or did not run, 2 on bad usage, a missing domain or
# tool, or unparseable go test output.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"
# shellcheck source=../tool-versions.env
source scripts/tool-versions.env
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$PATH"
TEMPLATES=scripts/templates/new-domain

die() {
  echo "scaffold-tests: $1" >&2
  exit 2
}
fail() {
  printf 'scaffold-tests: %s\n' "$1" >&2
  echo "Fix scripts/templates/new-domain/ so that every scaffolded test runs and passes, then make new-domain" \
    "again (rule go-tests-pass, NFR-B1; docs/harness/rules.md#go-tests-pass)." >&2
  exit 1
}

name="${1:-}"
[[ "$name" =~ ^[a-z][a-z0-9_]*[a-z0-9]$ ]] || die "usage: scaffold-tests.sh <name> (make test-scaffold NAME=<name>), got '$name'"
[[ -d "apps/api/internal/$name" ]] || die "no apps/api/internal/$name: run make new-domain NAME=$name first"
command -v jq >/dev/null || die "no jq in .bin: run make tools"

pascal="$(awk -F_ '{ for (i = 1; i <= NF; i++) printf "%s%s", toupper(substr($i, 1, 1)), substr($i, 2) }' <<<"$name")"
expected="$(sed -n 's/^func \(Test[A-Za-z0-9_]*\)(.*/\1/p' "$TEMPLATES"/*_test.go.tmpl | sed "s/__PASCAL__/$pascal/g" | LC_ALL=C sort -u)"
[[ -n "$expected" ]] || die "no func Test... in $TEMPLATES/*_test.go.tmpl"

out="$(mktemp)" err="$(mktemp)"
trap 'rm -f "$out" "$err"' EXIT
if ! (cd apps/api && go test -json -race -count=1 -tags=integration "./internal/$name/...") >"$out" 2>"$err"; then
  cat "$err" >&2
  # The test and build output without the JSON envelope; lines that are not JSON as they are.
  jq -jR '. as $l | (try fromjson catch null) as $j
    | if ($j | type) != "object" then $l + "\n" elif $j.Output then $j.Output else empty end' "$out" >&2 ||
    cat "$out" >&2
  fail "go test of apps/api/internal/$name failed (output above)"
fi

# One "<action>\t<test>" line per test-level pass, fail or skip event (subtests included).
results="$(jq -r 'select(.Test != null and (.Action == "pass" or .Action == "fail" or .Action == "skip")) | "\(.Action)\t\(.Test)"' "$out")" ||
  die "unparseable go test -json output of apps/api/internal/$name"

bad="$(awk -F'\t' 'NF && $1 != "pass" { print "  " $1 ": " $2 }' <<<"$results")"
[[ -z "$bad" ]] || fail "tests of apps/api/internal/$name were skipped or failed:
$bad"
missing="$(LC_ALL=C comm -23 <(printf '%s\n' "$expected") <(awk -F'\t' '{ print $2 }' <<<"$results" | LC_ALL=C sort -u))"
[[ -z "$missing" ]] || fail "scaffolded tests of apps/api/internal/$name did not run: $(tr '\n' ' ' <<<"$missing")"
