#!/usr/bin/env bash
# Runs in the repo copy: drops the '4XX' response (and its $ref line) from every operation.
set -euo pipefail
awk '/^ *'"'"'4XX'"'"':$/ { skip = 1; next } skip { skip = 0; next } { print }' api/openapi.yaml >api/openapi.yaml.new
mv api/openapi.yaml.new api/openapi.yaml
