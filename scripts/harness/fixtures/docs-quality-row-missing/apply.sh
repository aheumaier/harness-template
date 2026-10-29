#!/usr/bin/env bash
# Runs in the repo copy: a domain under apps/api/internal/ without a docs/QUALITY.md row.
set -euo pipefail
mkdir -p apps/api/internal/billing
echo '# billing' >apps/api/internal/billing/AGENTS.md
