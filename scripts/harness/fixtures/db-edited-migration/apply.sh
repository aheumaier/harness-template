#!/usr/bin/env bash
# Runs in the repo copy: edit a migration that exists on the base branch.
set -euo pipefail
echo '-- edited after it was applied' >>apps/api/db/migrations/20261027000000_init.sql
