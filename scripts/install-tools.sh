#!/usr/bin/env bash
# Installs pinned tools into ./.bin. Idempotent: each tool is stamped with its version.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# shellcheck source=tool-versions.env
source "$ROOT/scripts/tool-versions.env"
BIN="$ROOT/.bin"
mkdir -p "$BIN"
export GOBIN="$BIN" GOTOOLCHAIN="$GO_TOOLCHAIN" GOFLAGS=-mod=mod

stamp_ok() { [[ -x "$BIN/$1" && "$(cat "$BIN/.$1.version" 2>/dev/null)" == "$2" ]]; }
stamp() { echo "$2" >"$BIN/.$1.version"; }

goinstall() { # name module@version
  local name=$1 pkg=$2 ver=${2##*@}
  stamp_ok "$name" "$ver" && return 0
  echo "installing $name $ver" >&2
  go install "$pkg"
  stamp "$name" "$ver"
}

# Downloaded binaries: os darwin|linux, arch amd64|arm64; anything else has no sha256 pin.
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
[[ $arch == x86_64 ]] && arch=amd64
[[ $arch == aarch64 ]] && arch=arm64

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

download() { # name url: fetch into $BIN/.dl-<name>, verify against <NAME>_SHA256_<OS>_<ARCH> or fail closed
  local name=$1 url=$2 var want got
  var="$(echo "${name}_SHA256_${os}_${arch}" | tr '[:lower:]' '[:upper:]')"
  want="${!var:-}"
  [[ -n "$want" ]] || {
    echo "no sha256 pin $var in scripts/tool-versions.env for $name on ${os}/${arch}" >&2
    exit 1
  }
  # Every failure path removes the partial download (set -e exits before a RETURN trap would run).
  curl -fsSL "$url" -o "$BIN/.dl-$name" || {
    rm -f "$BIN/.dl-$name"
    echo "download failed for $name ($url); not installed" >&2
    exit 1
  }
  got="$(sha256 "$BIN/.dl-$name")" || {
    rm -f "$BIN/.dl-$name"
    echo "could not hash $BIN/.dl-$name for $name; not installed" >&2
    exit 1
  }
  [[ "$got" == "$want" ]] || {
    rm -f "$BIN/.dl-$name"
    echo "sha256 mismatch for $name ($url): got $got, pinned $want ($var); not installed" >&2
    exit 1
  }
}

# Claude Code, for the CI reviewer only (.github/workflows/pr-review.yml, exec plan 0003 P3.2): installed by
# `install-tools.sh claude` alone, never by `make tools`, which every worktree runs. Its pins match the
# release's manifest.json. Its platform names say x64 where the other tools say amd64.
if [[ "${1:-}" == claude ]]; then
  if ! stamp_ok claude "$CLAUDE_CODE_VERSION"; then
    echo "installing claude $CLAUDE_CODE_VERSION" >&2
    a=$arch && [[ $a == amd64 ]] && a=x64
    download claude_code "https://downloads.claude.ai/claude-code-releases/${CLAUDE_CODE_VERSION}/${os}-${a}/claude"
    mv "$BIN/.dl-claude_code" "$BIN/claude"
    chmod +x "$BIN/claude"
    stamp claude "$CLAUDE_CODE_VERSION"
  fi
  exit 0
fi

goinstall golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"$GOLANGCI_LINT_VERSION"
goinstall go-arch-lint github.com/fe3dback/go-arch-lint@"$GO_ARCH_LINT_VERSION"
goinstall govulncheck golang.org/x/vuln/cmd/govulncheck@"$GOVULNCHECK_VERSION"
goinstall deadcode golang.org/x/tools/cmd/deadcode@"$DEADCODE_VERSION"
goinstall sqlc github.com/sqlc-dev/sqlc/cmd/sqlc@"$SQLC_VERSION"
goinstall oapi-codegen github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@"$OAPI_CODEGEN_VERSION"
goinstall oasdiff github.com/oasdiff/oasdiff@"$OASDIFF_VERSION"
goinstall shfmt mvdan.cc/sh/v3/cmd/shfmt@"$SHFMT_VERSION"
goinstall actionlint github.com/rhysd/actionlint/cmd/actionlint@"$ACTIONLINT_VERSION"
goinstall gremlins github.com/go-gremlins/gremlins/cmd/gremlins@"$GREMLINS_VERSION"

if ! stamp_ok atlas "$ATLAS_VERSION"; then
  echo "installing atlas (community edition) $ATLAS_VERSION" >&2
  download atlas "https://release.ariga.io/atlas/atlas-community-${os}-${arch}-${ATLAS_VERSION}"
  mv "$BIN/.dl-atlas" "$BIN/atlas"
  chmod +x "$BIN/atlas"
  stamp atlas "$ATLAS_VERSION"
fi

if ! stamp_ok shellcheck "$SHELLCHECK_VERSION"; then
  echo "installing shellcheck $SHELLCHECK_VERSION" >&2
  a=$arch && [[ $a == arm64 ]] && a=aarch64
  [[ $a == amd64 ]] && a=x86_64
  download shellcheck "https://github.com/koalaman/shellcheck/releases/download/${SHELLCHECK_VERSION}/shellcheck-${SHELLCHECK_VERSION}.${os}.${a}.tar.gz"
  tar -xzf "$BIN/.dl-shellcheck" -C "$BIN" --strip-components=1 "shellcheck-${SHELLCHECK_VERSION}/shellcheck"
  rm -f "$BIN/.dl-shellcheck"
  stamp shellcheck "$SHELLCHECK_VERSION"
fi

# Release binary only (exec plan 0003, Q1: CVE-2026-33634): never the Docker image or trivy-action.
if ! stamp_ok trivy "$TRIVY_VERSION"; then
  echo "installing trivy $TRIVY_VERSION" >&2
  o=Linux && [[ $os == darwin ]] && o=macOS
  a=64bit && [[ $arch == arm64 ]] && a=ARM64
  download trivy "https://github.com/aquasecurity/trivy/releases/download/${TRIVY_VERSION}/trivy_${TRIVY_VERSION#v}_${o}-${a}.tar.gz"
  tar -xzf "$BIN/.dl-trivy" -C "$BIN" trivy
  rm -f "$BIN/.dl-trivy"
  stamp trivy "$TRIVY_VERSION"
fi

if ! stamp_ok jq "$JQ_VERSION"; then
  echo "installing jq $JQ_VERSION" >&2
  o=$os && [[ $o == darwin ]] && o=macos
  download jq "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-${o}-${arch}"
  mv "$BIN/.dl-jq" "$BIN/jq"
  chmod +x "$BIN/jq"
  stamp jq "$JQ_VERSION"
fi

# pnpm >=11 ships a native binary via optionalDependencies; older pnpm's self-switching cannot install it,
# so pin it here. Must equal "packageManager" in package.json.
if ! stamp_ok pnpm "$PNPM_VERSION"; then
  echo "installing pnpm $PNPM_VERSION" >&2
  rm -rf "$BIN/.pnpm"
  npm install --silent --no-audit --no-fund --prefix "$BIN/.pnpm" "pnpm@$PNPM_VERSION"
  ln -sf "$BIN/.pnpm/node_modules/.bin/pnpm" "$BIN/pnpm"
  stamp pnpm "$PNPM_VERSION"
fi
grep -q "\"packageManager\": \"pnpm@$PNPM_VERSION\"" "$ROOT/package.json" ||
  {
    echo "package.json packageManager must be pnpm@$PNPM_VERSION (scripts/tool-versions.env)" >&2
    exit 1
  }
