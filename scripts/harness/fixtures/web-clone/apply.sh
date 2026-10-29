#!/usr/bin/env bash
# Runs in the repo copy: one function pasted from apps/web into packages/domain, under the same file name, so the
# finding must name both files from the repo root to tell them apart. The clone is 18 tokens, just past minTokens 16
# in scripts/harness/jscpd.json, so the fixture stops reproducing if that threshold is raised past 18.
set -euo pipefail
body() { # name -> a function that normalises a name, the same body each time
  printf 'export function %s(name: string): string {\n' "$1"
  printf '  return name.trim() || name;\n}\n'
}
body normalizeName >apps/web/src/normalize.ts
body normalizeTag >packages/domain/src/normalize.ts
