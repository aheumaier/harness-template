#!/usr/bin/env bats
# scripts/harness/file-sensors.sh: which per-file sensor kind a path gets (the pre-commit git hook's table).
bats_require_minimum_version 1.5.0
load helpers

@test "classifies paths by kind; deliberate violations, vendored and generated files are never format-checked" {
  # shellcheck source=../harness/file-sensors.sh
  source "$REPO/scripts/harness/file-sensors.sh"
  local path want got
  while read -r path want; do
    got="$(harness_file_kind "$path")"
    [[ "$got" == "${want#-}" ]] || {
      echo "$path: got '$got', want '${want#-}'" >&2
      return 1
    }
  done <<'TABLE'
scripts/harness/fixtures/sh-unquoted-variable/files/scripts/clean.sh skip
.specify/scripts/bash/common.sh skip
.specify/extensions/x/run.sh skip
apps/api/internal/platform/health/api/api.gen.go generated
apps/api/internal/billing/store/db.go generated
packages/api-client/src/gen/x.ts generated
apps/api/tools/analyzers/noctxroot/noctxroot.go go-analyzers
apps/api/tools/harness/main.go go-harness
apps/api/cmd/api/main.go go
apps/api/db/migrations/20260101000000_x.sql migration
apps/api/db/queries/x.sql sql
api/openapi.yaml openapi
apps/web/src/main.tsx web
apps/ios/Sources/AppCore/Greeting.swift swift
packages/kit/Sources/K.swift swift
scripts/tools/x.swift -
scripts/harness/run.sh sh
scripts/test/run.bats sh
scripts/git-hooks/pre-commit sh
docs/harness/rules.md md
Makefile -
TABLE
}

@test "the paths come from layout.env: a Go module at the repo root, the spec elsewhere" {
  sandbox scripts/harness/file-sensors.sh
  sed -i.bak -e 's|^API_DIR=.*|API_DIR=.|' -e 's|^SPEC=.*|SPEC=contract/v1.yaml|' "$SANDBOX/scripts/harness/layout.env"
  # shellcheck source=../harness/file-sensors.sh
  source "$SANDBOX/scripts/harness/file-sensors.sh"
  [[ "$(harness_file_kind cmd/api/main.go)" == go ]]
  [[ "$(harness_file_kind tools/harness/main.go)" == go-harness ]]
  [[ "$(harness_file_kind contract/v1.yaml)" == openapi ]]
  [[ "$(harness_file_kind api/openapi.yaml)" == "" ]]
}
