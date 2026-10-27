#!/usr/bin/env bats
# scripts/new-domain.sh: name validation happens before any file is touched. The full generator run is
# covered by the CI probe (make new-domain NAME=zzz_probe && make check).
bats_require_minimum_version 1.5.0
load helpers

@test "rejects invalid and reserved names without changing the repo" {
  before="$(git -C "$REPO" status --porcelain)"
  for n in '' Billing 1x x a- 'a b' 'a/b'; do
    run -1 "$REPO/scripts/new-domain.sh" "$n"
    has 'NAME must match'
  done
  run -1 "$REPO/scripts/new-domain.sh" platform
  has 'reserved'
  [ "$(git -C "$REPO" status --porcelain)" = "$before" ]
}
