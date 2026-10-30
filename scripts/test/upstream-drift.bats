#!/usr/bin/env bats
# scripts/harness/upstream-drift.sh: oasdiff breaking from each vendored upstream spec (UPSTREAM_SPECS) to its
# source's head (rules.md#api-upstream-no-breaking-change). Sources here are local repos: no network.
bats_require_minimum_version 1.5.0
load helpers
# API_PREFIX: where README "(b) Brownfield project" puts the harness tool and its testdata in this repo.
# shellcheck source=scripts/harness/layout.sh
source "$REPO/scripts/harness/layout.sh"

setup() {
  sandbox scripts/harness/upstream-drift.sh
  export HARNESS="$REPO/.bin/harness" # spec-refs; the sandbox has no .bin
  UP="$BATS_TEST_TMPDIR/up"
  git init -q "$UP"
  mkdir -p "$SANDBOX/api/upstream"
  spec "$UP/spec/openapi.yaml" /a /b
  REV="$(commit "$UP")"
  cp "$UP/spec/openapi.yaml" "$SANDBOX/api/upstream/up.yaml"
  vendor "api/upstream/up.yaml@$UP@spec/openapi.yaml@$REV"
}

# commit <repo>: commits everything, prints the commit.
commit() {
  commit_all "$1" up
  git -C "$1" rev-parse HEAD
}

# vendor <entry...>: sets UPSTREAM_SPECS in the sandbox's layout.env, whether or not the optional key is there.
vendor() {
  local IFS=:
  edit scripts/harness/layout.env '/^UPSTREAM_SPECS=/d'
  echo "UPSTREAM_SPECS=$*" >>"$SANDBOX/scripts/harness/layout.env"
}

drift() { run "$@" "$SANDBOX/scripts/harness/upstream-drift.sh"; }

@test "no UPSTREAM_SPECS: silent, nothing fetched" {
  vendor ''
  drift -0
  [ -z "$output" ]
}

@test "an unchanged or additive upstream passes silently" {
  drift -0
  [ -z "$output" ]
  spec "$UP/spec/openapi.yaml" /a /b /c
  commit "$UP"
  drift -0
  [ -z "$output" ]
}

@test "a breaking upstream change is drift, named at the source's head" {
  spec "$UP/spec/openapi.yaml" /a
  head="$(commit "$UP")"
  drift -1
  has "api/upstream/up.yaml: 1 breaking change(s) in $UP/spec/openapi.yaml at ${head:0:12} since vendored ${REV:0:12}"
  has "[api-upstream-no-breaking-change]"
  has "api path removed without deprecation [api-path-removed-without-deprecation]"
}

@test "a spec gone upstream is drift" {
  git -C "$UP" rm -q spec/openapi.yaml
  commit "$UP"
  drift -1
  has "api/upstream/up.yaml: spec/openapi.yaml is gone from $UP at"
}

@test "a malformed entry cannot be checked (exit 2), and the others still are" {
  spec "$UP/spec/openapi.yaml" /a
  commit "$UP"
  vendor "api/upstream/up.yaml@$UP@spec/openapi.yaml" "api/upstream/missing.yaml@$UP@spec/openapi.yaml@$REV" \
    "api/upstream/up.yaml@$UP@spec/openapi.yaml@HEAD" "api/upstream/up.yaml@-oProxy@spec/openapi.yaml@$REV" \
    "api/upstream/up.yaml@$UP@../openapi.yaml@$REV" "api/upstream/up.yaml@$UP@spec/openapi.yaml@$REV"
  drift -2 --separate-stderr
  grep -qF "spec/openapi.yaml: cannot check: not <vendored spec>@<source repo>" <<<"$stderr"
  grep -qF "cannot check: no such file api/upstream/missing.yaml" <<<"$stderr"
  grep -qF "cannot check: revision HEAD is not a 7 to 40 digit hex commit" <<<"$stderr"
  grep -qF "cannot check: source -oProxy is neither" <<<"$stderr"
  grep -qF "cannot check: spec path ../openapi.yaml is not a plain path" <<<"$stderr"
  has "api/upstream/up.yaml: 1 breaking change(s)"
}

@test "entries: the vectors layout.go's test shares (testdata/upstream-spec-vectors.txt)" {
  # A refused entry names one of entry_problem's reasons; an accepted one gets past them (to the missing copy).
  local refused='cannot check: (not <|vendored spec .* is not a path|source .* is neither|spec path .* is not a plain|revision .* is not a 7)'
  local entry want got n=0 bad=''
  while read -r entry want; do
    [[ -n "$entry" && "$entry" != "#"* ]] || continue
    n=$((n + 1))
    vendor "$entry"
    run "$SANDBOX/scripts/harness/upstream-drift.sh"
    grep -qE "$refused" <<<"$output" && got=reject || got=ok
    [[ "$status" -eq 2 && "$got" == "$want" ]] || bad+="$entry: $got (exit $status), want $want"$'\n'
  done <"$REPO/${API_PREFIX}tools/harness/testdata/upstream-spec-vectors.txt"
  [ "$n" -gt 0 ]
  [ -z "$bad" ] || {
    printf '%s' "$bad" >&2
    return 1
  }
}

@test "a source that cannot be fetched fails closed" {
  vendor "api/upstream/up.yaml@$BATS_TEST_TMPDIR/nope@spec/openapi.yaml@$REV"
  drift -2
  has "cannot check: cannot fetch HEAD of $BATS_TEST_TMPDIR/nope"
}

@test "a source with a .. segment is refused (exit 2), never fetched" {
  vendor "api/upstream/up.yaml@github.com/acme/../up@spec/openapi.yaml@$REV"
  drift -2
  has "cannot check: source github.com/acme/../up is neither"
}

@test "an external \$ref, here or upstream, fails closed: oasdiff would follow it" {
  printf '%s\n' 'components:' '  schemas:' '    X: { $ref: "https://example.com/x.yaml" }' >>"$UP/spec/openapi.yaml"
  head="$(commit "$UP")"
  drift -2
  has "$UP@${head:0:12}:spec/openapi.yaml:16: \$ref \"https://example.com/x.yaml\" points outside this file"
  has "$UP's spec/openapi.yaml: a \$ref that is not internal (above)"
  cp "$UP/spec/openapi.yaml" "$SANDBOX/api/upstream/up.yaml"
  drift -2
  has "api/upstream/up.yaml:16: \$ref \"https://example.com/x.yaml\" points outside this file"
  has "api/upstream/up.yaml: a \$ref that is not internal (above)"
}

@test "an external \$ref that a line grep misses fails closed: key quoted or spaced, value on the next line" {
  printf '%s\n' 'components:' '  schemas:' '    A:' '      $ref:' '        other.yaml' '    B:' '      $ref : other.yaml' \
    '    C: { "$ref": "other.yaml" }' >>"$UP/spec/openapi.yaml"
  commit "$UP"
  drift -2
  has 'spec/openapi.yaml:17: $ref "other.yaml" points outside this file'
  has 'spec/openapi.yaml:20: $ref "other.yaml" points outside this file'
  has 'spec/openapi.yaml:21: $ref "other.yaml" points outside this file'
}

@test "a spec that does not parse, or that oasdiff cannot load, fails closed" {
  printf 'openapi: [\n' >"$UP/spec/openapi.yaml"
  commit "$UP"
  drift -2
  has "$UP's spec/openapi.yaml: does not parse (above)"
  printf 'openapi: 3.0.3\ninfo: { title: t, version: "1" }\npaths: 7\n' >"$UP/spec/openapi.yaml"
  commit "$UP"
  drift -2
  has "cannot check: oasdiff cannot compare"
}
