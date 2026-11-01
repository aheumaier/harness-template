#!/usr/bin/env bash
# Runs in the repo copy: IOS_DIR names a directory without the iOS app, as after the app moved and layout.env
# did not follow. The directory has a file, so only the missing Package.swift or project.yml is the finding.
set -euo pipefail
# shellcheck source=scripts/harness/layout.sh
source scripts/harness/layout.sh || exit 2
dir="${PRODUCT_GLOBS[0]%/\*}/moved-ios"
mkdir -p "$dir"
echo '# not the app' >"$dir/README.md"
sed -i.bak '/^IOS_DIR=/d' scripts/harness/layout.env
rm scripts/harness/layout.env.bak
echo "IOS_DIR=$dir" >>scripts/harness/layout.env
