#!/usr/bin/env bats
# scripts/harness/check-links.sh: relative Markdown links and #anchors resolve.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/check-links.sh
}

md() { # md <path> <line...>
  mkdir -p "$SANDBOX/$(dirname "$1")"
  printf '%s\n' "${@:2}" >"$SANDBOX/$1"
}

@test "passes when links and anchors resolve" {
  md docs/a.md '# Title' '## Some Section' 'See [b](../b.md#other-heading "title") and [self](#some-section).'
  md b.md '## Other Heading'
  run -0 "$SANDBOX/scripts/harness/check-links.sh"
}

@test "reports missing files and missing anchors" {
  md docs/a.md 'See [gone](missing.md) and [b](../b.md#nope).'
  md b.md '## Real'
  run -1 "$SANDBOX/scripts/harness/check-links.sh"
  has 'docs/a.md: broken link (missing.md)'
  has 'docs/a.md: broken anchor (../b.md#nope)'
}

@test "ignores external links, code spans and fenced blocks" {
  md a.md 'Visit [x](https://example.com/nope) and `[y](gone.md)`.' '```' '[z](gone.md)' '```'
  run -0 "$SANDBOX/scripts/harness/check-links.sh"
}
