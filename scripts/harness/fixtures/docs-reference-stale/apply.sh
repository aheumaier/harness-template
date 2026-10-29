#!/usr/bin/env bash
# Runs in the repo copy: raises the sqlc pin without taking docs/references/sqlc-llms.txt again.
set -euo pipefail
sed -i.bak "s|^SQLC_VERSION=.*|SQLC_VERSION=v1.99.0|" scripts/tool-versions.env
rm -f scripts/tool-versions.env.bak
