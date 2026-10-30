#!/usr/bin/env bash
# Runs in the repo copy: makes a local upstream repo (in the gitignored .tmp/, so no network) whose spec this repo
# vendors at api/upstream/openapi.yaml, lists it in UPSTREAM_SPECS, then breaks the spec upstream (renames
# /healthz, so the vendored path is removed without deprecation).
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
sed -i.bak "s|^UPSTREAM_SPECS=.*|UPSTREAM_SPECS=api/upstream/openapi.yaml@$up@spec/openapi.yaml@$rev|" \
  scripts/harness/layout.env
rm -f scripts/harness/layout.env.bak
sed -i.bak 's|^  /healthz:|  /healthz-v2:|' "$up/spec/openapi.yaml"
rm -f "$up/spec/openapi.yaml.bak"
commit "upstream breaks its contract"
