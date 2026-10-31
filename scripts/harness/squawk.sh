#!/usr/bin/env bash
# squawk over every migration. Atlas wraps each file in a transaction unless the file starts with the
# `-- atlas:txmode none` directive (needed for CREATE INDEX CONCURRENTLY), so those files are checked
# with --no-assume-in-transaction and all others with .squawk.toml's assume_in_transaction = true.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 1
# shellcheck source=layout.sh
source scripts/harness/layout.sh || exit 2
tx=() notx=()
for f in "$MIGRATIONS_DIR"/*.sql; do
  if grep -qx -- '-- atlas:txmode none' "$f"; then notx+=("$f"); else tx+=("$f"); fi
done
status=0
((${#tx[@]} == 0)) || squawk --config .squawk.toml "${tx[@]}" || status=1
((${#notx[@]} == 0)) || squawk --config .squawk.toml --no-assume-in-transaction "${notx[@]}" || status=1
exit "$status"
