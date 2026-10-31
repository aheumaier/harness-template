#!/usr/bin/env bash
# Runs in the repo copy: add a rule under rules.md's Duplication section without adding it to any Skip list.
set -euo pipefail
f=docs/harness/rules.md
while IFS= read -r l; do
  printf '%s\n' "$l"
  if [[ "$l" == '## Duplication'* ]]; then
    printf '\n### repo-no-near-clones\n- **Rule:** a second duplication rule.\n'
  fi
done <"$f" >"$f.new"
mv "$f.new" "$f"
