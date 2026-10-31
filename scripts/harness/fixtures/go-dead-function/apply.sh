#!/usr/bin/env bash
# Runs in the repo copy: an exported function in apps/api that nothing, not even a test, calls.
set -euo pipefail
cat >apps/api/internal/platform/problem/fxdead.go <<'GO'
package problem

// FixtureUnused is a selftest fixture: nothing calls it.
func FixtureUnused() string { return "unused" }
GO
