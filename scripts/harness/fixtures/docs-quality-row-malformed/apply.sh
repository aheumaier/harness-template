#!/usr/bin/env bash
# Runs in the repo copy: a domain whose docs/QUALITY.md row writes "not graded" with a hyphen, not the en dash.
set -euo pipefail
mkdir -p apps/api/internal/billing
echo '# billing' >apps/api/internal/billing/AGENTS.md
row='| [billing](../apps/api/internal/billing/AGENTS.md) | A | B | B | - | service, handler | AGENTS.md | 2026-01-01 | |'
ROW="$row" awk '/harness:quality-rows/ { print ENVIRON["ROW"] } { print }' docs/QUALITY.md >docs/QUALITY.md.new
mv docs/QUALITY.md.new docs/QUALITY.md
