#!/usr/bin/env bash
# Runs in the repo copy: append a hand edit to a generated file.
set -euo pipefail
echo '// hand edit' >>packages/api-client/src/gen/model/health.zod.ts
