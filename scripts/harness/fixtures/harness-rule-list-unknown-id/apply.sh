#!/usr/bin/env bash
# Runs in the repo copy: misspell a rule id in a review skill's Skip list.
set -euo pipefail
f=.claude/skills/review-modularity/SKILL.md
sed -i.bak 's/`go-layers`/`go-layer`/' "$f" && rm "$f.bak"
