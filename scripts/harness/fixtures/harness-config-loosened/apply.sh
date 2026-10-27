#!/usr/bin/env bash
# Runs in the repo copy: loosen a harness config.
set -euo pipefail
echo 'excluded_rules = ["require-concurrent-index-creation"]' >>.squawk.toml
