#!/usr/bin/env bash
# Runs in the repo copy: a Go file of 206 lines, one past revive file-length-limit's threshold (205). Generated,
# so it stays past the threshold when the threshold goes down. Comment lines count (skipComments: false).
set -euo pipefail
mkdir -p apps/api/internal/fx/service
{
  printf '// Package service is a selftest fixture: one file over the length limit.\npackage service\n\n'
  for i in $(seq 203); do printf '// Line %d of a file that holds too much.\n' "$i"; done
} >apps/api/internal/fx/service/service.go
