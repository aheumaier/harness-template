#!/usr/bin/env bash
# Runs in the repo copy: a docs/QUALITY.md row last graded on 2020-01-01 (the age check reads no domain).
set -euo pipefail
row='| [billing](../apps/api/internal/billing/AGENTS.md) | A | B | B | A | service, handler | AGENTS.md | 2020-01-01 | |'
ROW="$row" awk '/harness:quality-rows/ { print ENVIRON["ROW"] } { print }' docs/QUALITY.md >docs/QUALITY.md.new
mv docs/QUALITY.md.new docs/QUALITY.md
