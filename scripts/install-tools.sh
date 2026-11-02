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

# `install-tools.sh browsers` (make e2e, on demand; never make setup or make deps): the Chromium headless shell
# for make e2e, into $PLAYWRIGHT_BROWSERS_PATH (the Makefile exports .bin/playwright, also the default here),
# never the user's cache. Only where e2e runs downloads it (docs/adr/0025-browser-e2e-is-the-nfr-b2-sensor.md).
# Its build is pinned by the lockfile's @playwright/test (apps/web), so it needs node_modules. The version stamp
# lives inside the directory, which CI caches apart from the rest of .bin, keyed on that version
# (.github/actions/setup, input browser).
if [[ "${1:-}" == browsers ]]; then
  dir="${PLAYWRIGHT_BROWSERS_PATH:-$BIN/playwright}"
  pw="$ROOT/apps/web/node_modules/.bin/playwright"
  [[ -x "$pw" ]] || {
    echo "no $pw: run make deps first (Playwright's browser build comes from the lockfile)" >&2
    exit 1
  }
  ver="$("$pw" --version)" # "Version 1.64.1"
  ver="${ver##* }"
  [[ "$(cat "$dir/.version" 2>/dev/null)" == "$ver" ]] && exit 0
  echo "installing playwright chromium headless shell $ver into $dir" >&2
  # No sha256 pin for the browser (ADR 0025; TD-42), so at least the environment must not redirect its
  # download or swap its build: drop every override Playwright reads, also in npm's config spellings.
  for v in DOWNLOAD_HOST CHROMIUM_DOWNLOAD_HOST FIREFOX_DOWNLOAD_HOST WEBKIT_DOWNLOAD_HOST HOST_PLATFORM_OVERRIDE; do
    lower="$(echo "playwright_$v" | tr '[:upper:]' '[:lower:]')"
    unset "PLAYWRIGHT_$v" "npm_config_$lower" "npm_package_config_$lower"
  done
  # An older build is replaced, but only in a directory this script stamped: any other is installed into as is.
  [[ ! -f "$dir/.version" ]] || rm -rf "${dir:?}"
  mkdir -p "$dir"
  PLAYWRIGHT_BROWSERS_PATH="$dir" "$pw" install --only-shell chromium >/dev/null
  echo "$ver" >"$dir/.version"
  exit 0
fi

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
# Spellings some releases use: x64 for amd64 (claude, spectral, squawk), macos for darwin (jq, spectral).
x64=$arch && [[ $x64 == amd64 ]] && x64=x64
macos=$os && [[ $macos == darwin ]] && macos=macos

sha256() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

download() { # name url [var]: fetch into $BIN/.dl-<name>, verify against var (default <NAME>_SHA256_<OS>_<ARCH>) or fail closed
  local name=$1 url=$2 var="${3-}" want got
  [[ -n "$var" ]] || var="$(echo "${name}_SHA256_${os}_${arch}" | tr '[:lower:]' '[:upper:]')"
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

install_release() { # name version url: a single binary downloaded (and verified) into $BIN/<name>, stamped
  local name=$1 ver=$2
  stamp_ok "$name" "$ver" && return 0
  echo "installing $name $ver" >&2
  download "$name" "$3"
  mv "$BIN/.dl-$name" "$BIN/$name"
  chmod +x "$BIN/$name"
  stamp "$name" "$ver"
}

# Claude Code, for the CI reviewer only (.github/workflows/pr-review.yml, exec plan 0003 P3.2): installed by
# `install-tools.sh claude` alone, never by `make tools`, which every worktree runs. Its pins match the
# release's manifest.json. Its download is claude_code (its pins' name), its binary claude.
if [[ "${1:-}" == claude ]]; then
  if ! stamp_ok claude "$CLAUDE_CODE_VERSION"; then
    echo "installing claude $CLAUDE_CODE_VERSION" >&2
    download claude_code "https://downloads.claude.ai/claude-code-releases/${CLAUDE_CODE_VERSION}/${os}-${x64}/claude"
    mv "$BIN/.dl-claude_code" "$BIN/claude"
    chmod +x "$BIN/claude"
    stamp claude "$CLAUDE_CODE_VERSION"
  fi
  exit 0
fi

# XcodeGen, for make build-ios (ADR 0044): installed by `install-tools.sh xcodegen` alone, on demand where Xcode is
# selected (make build-ios, make setup), never by `make tools`: a Mac with only the command-line tools cannot use
# it. The zip's tree is kept whole in .bin/.xcodegen, since the binary reads its setting presets from
# ../share/xcodegen next to it (it follows the .bin/xcodegen link to find them).
if [[ "${1:-}" == xcodegen ]]; then
  [[ $os == darwin ]] || {
    echo "xcodegen runs on macOS only (ADR 0044); not installed" >&2
    exit 1
  }
  if ! stamp_ok xcodegen "$XCODEGEN_VERSION"; then
    echo "installing xcodegen $XCODEGEN_VERSION" >&2
    download xcodegen "https://github.com/yonaskolb/XcodeGen/releases/download/${XCODEGEN_VERSION}/xcodegen.zip" XCODEGEN_SHA256_DARWIN
    rm -rf "$BIN/.xcodegen"
    unzip -q "$BIN/.dl-xcodegen" -d "$BIN/.xcodegen" && [[ -x "$BIN/.xcodegen/xcodegen/bin/xcodegen" ]] || {
      rm -rf "$BIN/.dl-xcodegen" "$BIN/.xcodegen"
      echo "could not unpack xcodegen (no xcodegen/bin/xcodegen in the zip); not installed" >&2
      exit 1
    }
    rm -f "$BIN/.dl-xcodegen"
    ln -sfn .xcodegen/xcodegen/bin/xcodegen "$BIN/xcodegen"
    stamp xcodegen "$XCODEGEN_VERSION"
  fi
  exit 0
fi

# The stack (layout.env's STACK, or HARNESS_STACK; ADR 0033) picks the tools: go's always, db's and web's only
# with that group, ios's only with that group and on macOS.
# shellcheck source=harness/layout.sh
source "$ROOT/scripts/harness/layout.sh" || exit 2

goinstall golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"$GOLANGCI_LINT_VERSION"
goinstall go-arch-lint github.com/fe3dback/go-arch-lint@"$GO_ARCH_LINT_VERSION"
goinstall govulncheck golang.org/x/vuln/cmd/govulncheck@"$GOVULNCHECK_VERSION"
goinstall deadcode golang.org/x/tools/cmd/deadcode@"$DEADCODE_VERSION"
! harness_stack_has db || goinstall sqlc github.com/sqlc-dev/sqlc/cmd/sqlc@"$SQLC_VERSION"
goinstall oapi-codegen github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@"$OAPI_CODEGEN_VERSION"
goinstall oasdiff github.com/oasdiff/oasdiff@"$OASDIFF_VERSION"
goinstall shfmt mvdan.cc/sh/v3/cmd/shfmt@"$SHFMT_VERSION"
goinstall actionlint github.com/rhysd/actionlint/cmd/actionlint@"$ACTIONLINT_VERSION"
goinstall gremlins github.com/go-gremlins/gremlins/cmd/gremlins@"$GREMLINS_VERSION"

# Atlas community edition.
! harness_stack_has db || install_release atlas "$ATLAS_VERSION" "https://release.ariga.io/atlas/atlas-community-${os}-${arch}-${ATLAS_VERSION}"

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

install_release jq "$JQ_VERSION" "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/jq-${macos}-${arch}"
# Standalone builds: no Node needed.
install_release spectral "$SPECTRAL_VERSION" "https://github.com/stoplightio/spectral/releases/download/${SPECTRAL_VERSION}/spectral-${macos}-${x64}"
! harness_stack_has db || install_release squawk "$SQUAWK_VERSION" "https://github.com/sbdchd/squawk/releases/download/${SQUAWK_VERSION}/squawk-${os}-${x64}"

# bats is a bash program: the npm tarball's package/ goes to .bin/.bats, .bin/bats links its entry point.
if ! stamp_ok bats "$BATS_VERSION"; then
  echo "installing bats $BATS_VERSION" >&2
  download bats "https://registry.npmjs.org/bats/-/bats-${BATS_VERSION}.tgz" BATS_SHA256
  rm -rf "$BIN/.bats"
  mkdir -p "$BIN/.bats"
  tar -xzf "$BIN/.dl-bats" -C "$BIN/.bats" --strip-components=1 package
  rm -f "$BIN/.dl-bats"
  ln -sf .bats/bin/bats "$BIN/bats"
  stamp bats "$BATS_VERSION"
fi

# The ios group's linters (ADR 0043), on macOS only: owner decision Q4 of exec plan 0004.
if harness_stack_has ios && [[ $os == darwin ]]; then
  if ! stamp_ok swiftlint "$SWIFTLINT_VERSION"; then
    echo "installing swiftlint $SWIFTLINT_VERSION" >&2
    download swiftlint "https://github.com/realm/SwiftLint/releases/download/${SWIFTLINT_VERSION}/portable_swiftlint.zip" SWIFTLINT_SHA256_DARWIN
    unzip -o -q "$BIN/.dl-swiftlint" swiftlint -d "$BIN" || {
      rm -f "$BIN/.dl-swiftlint"
      echo "could not unpack swiftlint; not installed" >&2
      exit 1
    }
    rm -f "$BIN/.dl-swiftlint"
    stamp swiftlint "$SWIFTLINT_VERSION"
  fi
  # No release binaries: built once from the tag's commit, every dependency at its pinned revision (about 3 min).
  # The stamp names the commit and the pins too, so a change to either rebuilds it.
  resolved="$ROOT/scripts/swift-format/Package.resolved"
  [[ -f "$resolved" ]] || {
    echo "scripts/swift-format/Package.resolved is missing: restore it from the template (swift-format's dependency pins, ADR 0043)" >&2
    exit 1
  }
  pin="$SWIFT_FORMAT_VERSION $SWIFT_FORMAT_COMMIT $(sha256 "$resolved")"
  if ! stamp_ok swift-format "$pin"; then
    echo "building swift-format $SWIFT_FORMAT_VERSION from source (about 3 minutes, once)" >&2
    src="$BIN/.src-swift-format"
    not_installed() {
      rm -rf "${src:?}"
      echo "swift-format $SWIFT_FORMAT_VERSION $1; not installed" >&2
      exit 1
    }
    rm -rf "${src:?}"
    git -c advice.detachedHead=false clone -q --depth 1 --branch "$SWIFT_FORMAT_VERSION" \
      https://github.com/swiftlang/swift-format.git "$src" || not_installed "could not be cloned"
    [[ "$(git -C "$src" rev-parse HEAD)" == "$SWIFT_FORMAT_COMMIT" ]] ||
      not_installed "is not commit $SWIFT_FORMAT_COMMIT (SWIFT_FORMAT_COMMIT)"
    cp "$resolved" "$src/Package.resolved"
    swift build -q --package-path "$src" -c release --product swift-format --only-use-versions-from-resolved-file ||
      not_installed "did not build"
    mv "$(swift build --package-path "$src" -c release --show-bin-path)/swift-format" "$BIN/swift-format" ||
      not_installed "built, but could not be moved to .bin"
    rm -rf "${src:?}"
    stamp swift-format "$pin"
  fi
fi

harness_stack_has web || exit 0
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
