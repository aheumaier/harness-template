#!/usr/bin/env bash
# Runs in the repo copy: makes a local upstream repo (in the gitignored .tmp/, so no network) whose spec this repo
# vendors at api/upstream/openapi.yaml, lists it in UPSTREAM_SPECS, then breaks the spec upstream (renames its
# first path, /healthz in the template, so the vendored path is removed without deprecation).
# The first path, not a named one, and UPSTREAM_SPECS appended, not replaced, so the fixture holds in an adopted
# repo (README (b)) whose contract has other paths and whose layout.env has no UPSTREAM_SPECS line.
set -euo pipefail
up="$PWD/.tmp/upstream-spec" # its own: harness-behind-upstream clones into .tmp/upstream
rm -rf "$up"
git init -q "$up"
mkdir -p "$up/spec" api/upstream
cp api/openapi.yaml "$up/spec/openapi.yaml"
cp api/openapi.yaml api/upstream/openapi.yaml
commit() {
  git -C "$up" add -A
  git -C "$up" -c core.hooksPath=/dev/null -c user.name=fixture -c user.email=fixture@localhost \
    -c commit.gpgsign=false commit -qm "$1"
}
commit "upstream spec"
rev="$(git -C "$up" rev-parse HEAD)"
# UPSTREAM_SPECS is optional (layout.sh): an adopted layout.env may not have the line, so drop it and append it.
sed -i.bak '/^UPSTREAM_SPECS=/d' scripts/harness/layout.env
rm -f scripts/harness/layout.env.bak
echo "UPSTREAM_SPECS=api/upstream/openapi.yaml@$up@spec/openapi.yaml@$rev" >>scripts/harness/layout.env
# Path keys are the only two-space-indented keys that start with "/"; the range stops after the first one.
sed -i.bak '1,/^  \//s|^  \(/[^:]*\):|  \1-v2:|' "$up/spec/openapi.yaml"
if cmp -s "$up/spec/openapi.yaml" "$up/spec/openapi.yaml.bak"; then
  echo "api-upstream-breaking: api/openapi.yaml has no path to rename" >&2
  exit 1
fi
rm -f "$up/spec/openapi.yaml.bak"
commit "upstream breaks its contract"
