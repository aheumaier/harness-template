#!/usr/bin/env bash
# Runs in the repo copy: a unit test in a domain's service package (any layout whose domains are under PRODUCT_GLOBS, README (b)) that names
# 900/US1-AS1, the only test that does.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
dir="$DOMAINS_DIR/cart/service"
mkdir -p "$dir"
cat >"$dir/pay_test.go" <<'GO'
package service_test

import "testing"

func TestPay(t *testing.T) {
	t.Run("900/US1-AS1 gets a receipt", func(t *testing.T) {})
}
GO
