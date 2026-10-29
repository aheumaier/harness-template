#!/usr/bin/env bash
# Runs in the repo copy: drop `repo-no-orphan-scripts` from review-modularity's Skip list.
set -euo pipefail
f=.claude/skills/review-modularity/SKILL.md
sed -i.bak 's/ `repo-no-orphan-scripts`,//' "$f" && rm "$f.bak"
