#!/usr/bin/env bash
# Runs in the repo copy: plants a fake AWS access key in a Markdown file, which Trivy's built-in path allow-list
# skips; scripts/harness/trivy-secret.yaml switches that allow-list off. Assembled here, so no key-shaped string
# is in the repo (see fixtures/repo-secret-planted).
set -euo pipefail
printf '# Setup notes\n\naws_access_key_id = %s%s\n' AKIA QYLPMN5HHHFPZAM2 >fx-credentials.md
