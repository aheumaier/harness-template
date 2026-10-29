#!/usr/bin/env bats
# scripts/install-tools.sh claude: the CI-only Claude Code install (exec plan 0003 P3.2, ADR 0016) fails closed
# on a missing pin or a checksum mismatch. A stub curl serves fake bytes: no network.
# scripts/install-tools.sh browsers: make e2e's on-demand browser install (ADR 0025), against a stub playwright.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/install-tools.sh scripts/tool-versions.env
  STUB="$BATS_TEST_TMPDIR/stub"
  mkdir -p "$STUB"
  # curl -fsSL <url> -o <file>: logs the url, writes fake bytes.
  cat >"$STUB/curl" <<'EOF'
#!/usr/bin/env bash
while [[ $# -gt 0 ]]; do
  case $1 in
    -o) out=$2; shift ;;
    http*) echo "$1" >>"$BATS_TEST_TMPDIR/curl.log" ;;
  esac
  shift
done
printf 'not claude\n' >"$out"
EOF
  chmod +x "$STUB/curl"
  export PATH="$STUB:$PATH"
  I="$SANDBOX/scripts/install-tools.sh"
  V="$SANDBOX/scripts/tool-versions.env"
}

# pins <value>: every CLAUDE_CODE_SHA256_* pin set to value, or removed when value is empty.
pins() {
  grep -v '^CLAUDE_CODE_SHA256_' "$V" >"$V.new"
  if [[ -n $1 ]]; then
    for p in DARWIN_ARM64 DARWIN_AMD64 LINUX_ARM64 LINUX_AMD64; do echo "CLAUDE_CODE_SHA256_$p=$1" >>"$V.new"; done
  fi
  mv "$V.new" "$V"
}

@test "claude: no pin for this platform installs nothing and downloads nothing" {
  pins ''
  run -1 "$I" claude
  has 'no sha256 pin CLAUDE_CODE_SHA256_'
  [ ! -e "$SANDBOX/.bin/claude" ]
  [ ! -e "$BATS_TEST_TMPDIR/curl.log" ]
}

@test "claude: a checksum mismatch installs nothing and removes the download" {
  run -1 "$I" claude
  has 'sha256 mismatch for claude_code'
  grep -q '^https://downloads.claude.ai/claude-code-releases/' "$BATS_TEST_TMPDIR/curl.log"
  [ ! -e "$SANDBOX/.bin/claude" ]
  [ ! -e "$SANDBOX/.bin/.dl-claude_code" ]
}

@test "claude: a matching pin installs and stamps it, and installs nothing else" {
  pins "$(printf 'not claude\n' | { sha256sum 2>/dev/null || shasum -a 256; } | cut -d' ' -f1)"
  run -0 "$I" claude
  [ -x "$SANDBOX/.bin/claude" ]
  [ "$(find "$SANDBOX/.bin" -mindepth 1 -maxdepth 1 -exec basename {} \; | LC_ALL=C sort | tr '\n' ' ')" = '.claude.version claude ' ]
  rm "$BATS_TEST_TMPDIR/curl.log"
  run -0 "$I" claude
  [ ! -e "$BATS_TEST_TMPDIR/curl.log" ]
}

# fake_playwright: apps/web/node_modules/.bin/playwright in the sandbox; `install` logs its arguments, the
# browsers path and any download override it sees, and writes a marker there.
fake_playwright() {
  mkdir -p "$SANDBOX/apps/web/node_modules/.bin"
  cat >"$SANDBOX/apps/web/node_modules/.bin/playwright" <<'EOF'
#!/usr/bin/env bash
[[ "$1" != --version ]] || { echo "Version 1.64.1"; exit 0; }
echo "$* $PLAYWRIGHT_BROWSERS_PATH ${PLAYWRIGHT_DOWNLOAD_HOST:-}${npm_config_playwright_download_host:-}" >>"$BATS_TEST_TMPDIR/pw.log"
touch "$PLAYWRIGHT_BROWSERS_PATH/chromium_headless_shell"
EOF
  chmod +x "$SANDBOX/apps/web/node_modules/.bin/playwright"
}

@test "browsers: installs the locked headless shell once into PLAYWRIGHT_BROWSERS_PATH, ignoring download overrides" {
  fake_playwright
  local pw="$BATS_TEST_TMPDIR/browsers"
  run -0 env PLAYWRIGHT_BROWSERS_PATH="$pw" PLAYWRIGHT_DOWNLOAD_HOST=http://evil \
    npm_config_playwright_download_host=http://evil "$I" browsers
  has 'installing playwright chromium headless shell 1.64.1'
  [ "$(cat "$BATS_TEST_TMPDIR/pw.log")" = "install --only-shell chromium $pw " ]
  [ "$(cat "$pw/.version")" = 1.64.1 ]
  [ -e "$pw/chromium_headless_shell" ]
  # Stamped: a second run downloads nothing.
  run -0 env PLAYWRIGHT_BROWSERS_PATH="$pw" "$I" browsers
  [ -z "$output" ]
  [ "$(wc -l <"$BATS_TEST_TMPDIR/pw.log")" -eq 1 ]
}

@test "browsers: defaults to .bin/playwright; replaces an older stamped build, never deletes an unstamped directory" {
  fake_playwright
  mkdir -p "$SANDBOX/.bin/playwright"
  echo 1.0.0 >"$SANDBOX/.bin/playwright/.version"
  touch "$SANDBOX/.bin/playwright/old-build"
  run -0 env -u PLAYWRIGHT_BROWSERS_PATH "$I" browsers
  [ "$(cat "$SANDBOX/.bin/playwright/.version")" = 1.64.1 ]
  [ ! -e "$SANDBOX/.bin/playwright/old-build" ]
  local mine="$BATS_TEST_TMPDIR/mine"
  mkdir -p "$mine"
  touch "$mine/keep"
  run -0 env PLAYWRIGHT_BROWSERS_PATH="$mine" "$I" browsers
  [ -e "$mine/keep" ]
  [ -e "$mine/chromium_headless_shell" ]
}

@test "browsers: without node_modules it fails with the remediation and installs nothing" {
  run -1 env PLAYWRIGHT_BROWSERS_PATH="$BATS_TEST_TMPDIR/browsers" "$I" browsers
  has 'run make deps first'
  [ ! -e "$BATS_TEST_TMPDIR/browsers" ]
}
