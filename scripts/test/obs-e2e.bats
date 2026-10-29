#!/usr/bin/env bats
# scripts/harness/obs-e2e.sh (make test-obs, NFR-O1) fails closed, and removes only an obs stack it started.
# Docker, make and go are stubs here; the passing path needs Docker and the obs stack: make check-all runs it.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/obs-e2e.sh .bin/jq
  E2E="$SANDBOX/scripts/harness/obs-e2e.sh"
  STUB="$BATS_TEST_TMPDIR/stub"
  mkdir -p "$STUB"
  # docker answers `info` with $STUB/info-rc (default 0), `compose … ps` with $STUB/ps-out and ps-rc, and
  # records every other call; make records its calls and succeeds; go fails, so a run stops at the build, after
  # the stack.
  cat >"$STUB/docker" <<'STUB'
#!/usr/bin/env bash
d="$(dirname "$0")"
[[ "$1" == info ]] && exit "$(cat "$d/info-rc" 2>/dev/null || echo 0)"
if [[ "$1" == compose && " $* " == *" ps "* ]]; then
  cat "$d/ps-out" 2>/dev/null
  exit "$(cat "$d/ps-rc" 2>/dev/null || echo 0)"
fi
echo "$*" >>"$d/docker.calls"
STUB
  cat >"$STUB/make" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$(dirname "$0")/make.calls"
STUB
  printf '#!/usr/bin/env bash\necho "build failed" >&2\nexit 1\n' >"$STUB/go"
  chmod +x "$STUB/docker" "$STUB/make" "$STUB/go"
  export PATH="$STUB:$PATH"
  unset MAKE
  export COMPOSE_PROJECT_NAME=p OTLP_PORT=1 LOKI_PORT=2 TEMPO_PORT=3 PROMETHEUS_PORT=4
}

@test "obs-e2e: no docker on PATH fails with the fix" {
  local bin="$BATS_TEST_TMPDIR/nodocker"
  mkdir -p "$bin"
  ln -s "$(command -v bash)" "$bin/bash"
  ln -s "$(command -v dirname)" "$bin/dirname"
  run env PATH="$bin" "$E2E"
  [ "$status" -eq 2 ]
  has "no docker on PATH: install Docker"
}

@test "obs-e2e: Docker not running fails with the fix" {
  echo 1 >"$STUB/info-rc"
  run "$E2E"
  [ "$status" -eq 2 ]
  has "Docker is not running: start it"
  [ ! -e "$STUB/docker.calls" ]
  [ ! -e "$STUB/make.calls" ]
}

@test "obs-e2e: a worktree env without the obs ports fails with make setup" {
  unset OTLP_PORT
  run "$E2E"
  [ "$status" -eq 2 ]
  has "no OTLP_PORT: run make setup"
  [ ! -e "$STUB/docker.calls" ]
  [ ! -e "$STUB/make.calls" ]
}

@test "obs-e2e: a stack it started is removed when the run fails" {
  : >"$STUB/ps-out"
  run "$E2E"
  [ "$status" -eq 1 ]
  has "the API does not build"
  grep -q "obs-up" "$STUB/make.calls"
  grep -q "obs-down" "$STUB/make.calls"
}

@test "obs-e2e: a stack that was already up stays when the run fails" {
  echo 0123abcd >"$STUB/ps-out"
  run "$E2E"
  [ "$status" -eq 1 ]
  has "the API does not build"
  grep -q "obs-up" "$STUB/make.calls"
  run ! grep -q "obs-down" "$STUB/make.calls"
}

@test "obs-e2e: a failed stack probe fails and removes nothing" {
  echo 1 >"$STUB/ps-rc"
  run "$E2E"
  [ "$status" -eq 1 ]
  has "docker compose ps failed"
  run ! grep -q "obs-" "$STUB/make.calls"
}
