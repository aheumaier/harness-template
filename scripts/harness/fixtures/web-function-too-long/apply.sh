#!/usr/bin/env bash
# Runs in the repo copy: a function of 51 lines, one past ESLint max-lines-per-function's threshold (50).
# Generated, so it stays past the threshold when the threshold goes down. Straight-line code: no other size
# measure fires.
set -euo pipefail
{
  printf 'export function fixtureTooLong(): number {\n  let x = 0;\n'
  for _ in $(seq 47); do printf '  x += 1;\n'; done
  printf '  return x;\n}\n'
} >apps/web/src/fixture.ts
