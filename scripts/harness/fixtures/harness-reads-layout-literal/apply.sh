#!/usr/bin/env bash
# Runs in the repo copy: a .bin/harness source names a layout path as a literal instead of asking layout.go.
set -euo pipefail
printf 'package main\n\nconst fixtureDomainsDir = "apps/api/internal"\n' >apps/api/tools/harness/zz_layout_fixture.go
