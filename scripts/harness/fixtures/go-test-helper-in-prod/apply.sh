#!/usr/bin/env bash
# Runs in the repo copy: a non-test file in platform/health imports the test-only approvaltest package.
set -euo pipefail
cat >apps/api/internal/platform/health/approved.go <<'EOF'
package health

import "github.com/aheumaier/harness-template/apps/api/internal/platform/approvaltest"

// approvedDir reuses a test helper from production code.
var approvedDir = approvaltest.Dir
EOF
