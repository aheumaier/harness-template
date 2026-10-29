#!/usr/bin/env bash
# Runs in the repo copy: a function of 36 statements, one past funlen's threshold (35). Generated, so it stays
# past the threshold when the threshold goes down. Straight-line code: no other size measure fires.
set -euo pipefail
mkdir -p apps/api/internal/fx/service
{
  printf '// Package service is a selftest fixture: one function over the statement limit.\npackage service\n\n'
  printf '// FixtureTooLong does one thing 34 times.\nfunc FixtureTooLong() int {\n\tx := 0\n'
  for _ in $(seq 34); do printf '\tx++\n'; done
  printf '\treturn x\n}\n'
} >apps/api/internal/fx/service/service.go
