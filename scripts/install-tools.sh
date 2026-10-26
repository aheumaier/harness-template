#!/usr/bin/env bash
# Installs pinned tools into ./.bin. Idempotent: each tool is stamped with its version.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=tool-versions.env
source "$ROOT/scripts/tool-versions.env"
BIN="$ROOT/.bin"; mkdir -p "$BIN"
export GOBIN="$BIN" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS=-mod=mod

stamp_ok() { [[ -x "$BIN/$1" && "$(cat "$BIN/.$1.version" 2>/dev/null)" == "$2" ]]; }
stamp()    { echo "$2" > "$BIN/.$1.version"; }

goinstall() { # name module@version
  local name=$1 pkg=$2 ver=${2##*@}
  stamp_ok "$name" "$ver" && return 0
  echo "installing $name $ver" >&2
  go install "$pkg"; stamp "$name" "$ver"
}

goinstall golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"$GOLANGCI_LINT_VERSION"
goinstall go-arch-lint  github.com/fe3dback/go-arch-lint@"$GO_ARCH_LINT_VERSION"
goinstall govulncheck   golang.org/x/vuln/cmd/govulncheck@"$GOVULNCHECK_VERSION"
goinstall deadcode      golang.org/x/tools/cmd/deadcode@"$DEADCODE_VERSION"
goinstall sqlc          github.com/sqlc-dev/sqlc/cmd/sqlc@"$SQLC_VERSION"
goinstall oapi-codegen  github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@"$OAPI_CODEGEN_VERSION"
goinstall oasdiff       github.com/oasdiff/oasdiff@"$OASDIFF_VERSION"

if ! stamp_ok atlas "$ATLAS_VERSION"; then
  echo "installing atlas (community edition) $ATLAS_VERSION" >&2
  os=$(uname -s | tr '[:upper:]' '[:lower:]'); arch=$(uname -m); [[ $arch == x86_64 ]] && arch=amd64; [[ $arch == aarch64 ]] && arch=arm64
  curl -fsSL "https://release.ariga.io/atlas/atlas-community-${os}-${arch}-${ATLAS_VERSION}" -o "$BIN/atlas"
  chmod +x "$BIN/atlas"; stamp atlas "$ATLAS_VERSION"
fi

# pnpm >=11 ships a native binary via optionalDependencies; older pnpm's self-switching cannot install it,
# so pin it here. Must equal "packageManager" in package.json.
if ! stamp_ok pnpm "$PNPM_VERSION"; then
  echo "installing pnpm $PNPM_VERSION" >&2
  rm -rf "$BIN/.pnpm"
  npm install --silent --no-audit --no-fund --prefix "$BIN/.pnpm" "pnpm@$PNPM_VERSION"
  ln -sf "$BIN/.pnpm/node_modules/.bin/pnpm" "$BIN/pnpm"; stamp pnpm "$PNPM_VERSION"
fi
grep -q "\"packageManager\": \"pnpm@$PNPM_VERSION\"" "$ROOT/package.json" \
  || { echo "package.json packageManager must be pnpm@$PNPM_VERSION (scripts/tool-versions.env)" >&2; exit 1; }
