#!/usr/bin/env bash
# Runs in the repo copy: hand-edit the approved 200 response to a status value the spec does not declare.
set -euo pipefail
f=apps/api/internal/platform/health/testdata/approved/getHealth.200.json
sed -i.bak 's/"status": "ok"/"status": "fine"/' "$f"
rm "$f.bak"
