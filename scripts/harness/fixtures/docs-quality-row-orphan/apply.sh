#!/usr/bin/env bash
# Runs in the repo copy: a well-formed docs/QUALITY.md row for a domain that does not exist.
set -euo pipefail
row='| [invoices](../apps/api/internal/invoices/AGENTS.md) | A | A | A | A | service, handler | AGENTS.md | 2026-01-01 | |'
ROW="$row" awk '/harness:quality-rows/ { print ENVIRON["ROW"] } { print }' docs/QUALITY.md >docs/QUALITY.md.new
mv docs/QUALITY.md.new docs/QUALITY.md
