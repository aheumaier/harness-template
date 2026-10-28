#!/usr/bin/env bash
# Runs in the repo copy: problemOf returns a Promise (lint-clean in api-client itself).
set -euo pipefail
perl -0pi -e 's/export function problemOf\(error: unknown\): ProblemOutput \| undefined \{/export function problemOf(error: unknown): Promise<ProblemOutput | undefined> {\n  return Promise.resolve(problemOfSync(error));\n}\n\nfunction problemOfSync(error: unknown): ProblemOutput | undefined {/' packages/api-client/src/index.ts
grep -q problemOfSync packages/api-client/src/index.ts
