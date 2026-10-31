#!/usr/bin/env bash
# Runs in the repo copy: a make target without a `## ` help comment that no target, workflow, hook, skill,
# agent or guide calls.
set -euo pipefail
printf '\nforgotten-target:\n\t@echo "nobody calls me"\n' >>Makefile
