#!/usr/bin/env bats
# scripts/new-domain.sh: name validation happens before any file is touched. Runs on a sandbox copy, so a
# regression cannot scaffold into the real repo. The full generator run is covered by the CI probe
# (make new-domain NAME=zzz_probe && make check).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/new-domain.sh scripts/tool-versions.env
}

@test "rejects invalid and reserved names without changing anything" {
  sandbox_base
  for n in '' Billing 1x x a- 'a b' 'a/b'; do
    run -1 "$SANDBOX/scripts/new-domain.sh" "$n"
    has 'NAME must match'
  done
  run -1 "$SANDBOX/scripts/new-domain.sh" platform
  has 'reserved'
  [ -z "$(git -C "$SANDBOX" status --porcelain)" ]
}
