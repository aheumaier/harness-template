#!/usr/bin/env bash
# Runs in the repo copy: the API module moves to the repo root. scripts/harness/fixtures/go.mod (shipped in the
# template) keeps the root module's ./... from building the fixtures' deliberate violations.
set -euo pipefail
sed -i.bak 's|^API_DIR=.*|API_DIR=.|' scripts/harness/layout.env && rm scripts/harness/layout.env.bak
