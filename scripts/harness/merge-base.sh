#!/usr/bin/env bash
# Base commit for the diff-based sensors (contract-check, migrations-append-only, integrity-check).
#
#   scripts/harness/merge-base.sh --ref   prints the default BASE_REF: origin/main if it exists, else main
#   scripts/harness/merge-base.sh         prints the merge base of HEAD with $BASE_REF (default: --ref)
#
# Fails closed (exit 2): a sensor that cannot find its base would compare nothing and pass.
set -euo pipefail

default_ref() {
  if git rev-parse -q --verify origin/main >/dev/null; then echo origin/main; else echo main; fi
}
if [[ "${1:-}" == --ref ]]; then
  default_ref
  exit 0
fi

ref="${BASE_REF:-$(default_ref)}"
git rev-parse -q --verify "$ref^{commit}" >/dev/null || {
  echo "BASE_REF '$ref' is not a commit in this clone: fetch it (git fetch origin main) or pass BASE_REF=<branch|sha>." >&2
  exit 2
}
git merge-base HEAD "$ref" || {
  echo "HEAD and BASE_REF '$ref' have no merge base: fetch full history (git fetch --unshallow; CI: actions/checkout with fetch-depth: 0)." >&2
  exit 2
}
