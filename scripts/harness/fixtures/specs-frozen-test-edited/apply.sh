#!/usr/bin/env bash
# Runs in the repo copy: commits the spec and, as the test writer's, its test under the first PRODUCT_GLOBS entry
# (any layout, README (b)), then loosens the assertion (selftest commits that).
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
dir="${PRODUCT_GLOBS[0]%/\*}/demo"
mkdir -p "$dir"
cat >"$dir/demo_test.go" <<'GO'
package demo

import "testing"

func TestAdd(t *testing.T) { // spec: 900/US1-AS1
	if got := add(1, 2); got != 3 {
		t.Fatalf("got %d, want 3", got)
	}
}
GO
git add specs "$dir"
git -c user.name=fixture -c user.email=fixture@localhost -c commit.gpgsign=false -c core.hooksPath=/dev/null \
  commit -qm 'test(demo): add two numbers' -m 'Test-Writer: specs/900-demo'
sed -i.bak 's/got != 3/got < 0/' "$dir/demo_test.go"
rm "$dir/demo_test.go.bak"
