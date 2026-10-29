#!/usr/bin/env bash
# Runs in the repo copy: the web-clone pair plus a root .jscpd.json that would ignore every file and raise
# minTokens. clones-web names the protected config with --config, so jscpd must not load this one and the pair
# must still be reported.
set -euo pipefail
bash scripts/harness/fixtures/web-clone/apply.sh
printf '{ "minTokens": 1000, "ignore": ["**"] }\n' >.jscpd.json
