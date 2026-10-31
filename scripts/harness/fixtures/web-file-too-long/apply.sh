#!/usr/bin/env bash
# Runs in the repo copy: a module of 301 lines, one past ESLint max-lines' threshold (300). Generated, so it
# stays past the threshold when the threshold goes down. Comment lines count (skipComments: false).
set -euo pipefail
{
  printf 'export const fixtureLines = 301;\n'
  for i in $(seq 300); do printf '// Line %d of a module that holds too much.\n' "$i"; done
} >apps/web/src/fixture.ts
