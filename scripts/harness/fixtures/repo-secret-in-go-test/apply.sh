#!/usr/bin/env bash
# Runs in the repo copy: plants a fake AWS access key in a Go test file, which Trivy's built-in path allow-list
# skips; scripts/harness/trivy-secret.yaml switches that allow-list off. Assembled here, so no key-shaped string
# is in the repo (see fixtures/repo-secret-planted).
set -euo pipefail
printf 'package fx\n\nconst awsKeyID = "%s%s"\n' AKIA QYLPMN5HHHFPZAM2 >fx_credentials_test.go
