#!/usr/bin/env bash
# Runs in the repo copy: the health handler words its 503 detail differently; the approved file is unchanged.
set -euo pipefail
f=apps/api/internal/platform/health/health.go
sed -i.bak 's/ptr("database unreachable")/ptr("database down")/' "$f"
rm "$f.bak"
