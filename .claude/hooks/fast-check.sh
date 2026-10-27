#!/usr/bin/env bash
# PostToolUse hook (Edit/Write/MultiEdit): runs the cheapest sensor for the file just changed.
#
#   .claude/hooks/fast-check.sh [file]     (no argument: reads the hook JSON on stdin, tool_input.file_path)
#
# Silent and exit 0 when the file is fine or no sensor applies. On failure prints the sensor output and the
# rule's remediation (scripts/harness/run.sh) to stderr and exits 2, which Claude Code feeds back to the
# agent. `make check` remains the full gate; this only shortens the feedback loop.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook fast-check
cd "$ROOT" || exit 0
# shellcheck source=../../scripts/tool-versions.env
source scripts/tool-versions.env
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$ROOT/node_modules/.bin:$PATH"
export HARNESS_SCOPE=fast-check # tags the timings of the sensors below
RUN=scripts/harness/run.sh

file="${1:-}"
if [[ -z "$file" ]]; then
  file="$(node -e 'let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
    try { const i = JSON.parse(s).tool_input ?? {}; process.stdout.write(i.file_path ?? i.notebook_path ?? ""); } catch {}
  })')"
fi
[[ -n "$file" ]] || exit 0
file="${file#"$ROOT"/}"
[[ "$file" != /* && -f "$file" ]] || exit 0 # outside the repo, or deleted

fail() {
  echo "fast-check: $*" >&2
  exit 2
}
check() { "$@" || exit 2; } # run.sh already printed the details to stderr

case "$file" in
  scripts/harness/fixtures/*/files/*) ;;        # deliberate violations
  .specify/scripts/* | .specify/templates/*) ;; # vendored spec-kit (ADR 0011): replaced on upgrade, not linted

  *.gen.go | apps/api/internal/*/store/*.go | packages/api-client/src/gen/* | docs/generated/*)
    fail "$file is generated (rules.md#gen-up-to-date). Revert the edit, change the source (api/openapi.yaml, apps/api/db/queries, apps/api/db/migrations) and run \`make gen\`."
    ;;

  apps/api/tools/analyzers/*.go)
    check $RUN analyzer-tests go-tests-pass --dir apps/api/tools/analyzers -- go vet ./...
    ;;

  apps/api/*.go)
    pkg="./$(dirname "${file#apps/api/}")/"
    check $RUN golangci-lint go-no-root-context,go-db-only-in-store,go-http-only-in-api,go-domain-isolation,go-format \
      --dir apps/api -- ../../.bin/golangci-lint-custom run "$pkg"
    ;;

  apps/api/db/migrations/*.sql)
    # Rehashing atlas.sum after editing an applied migration would hide the edit from gen-check.
    check $RUN migrations db-append-only-migrations -- scripts/harness/migrations-append-only.sh
    (cd apps/api && atlas migrate hash --env local) || fail "atlas migrate hash failed"
    check $RUN squawk db-safe-locking -- scripts/harness/squawk.sh
    if docker info >/dev/null 2>&1; then
      check $RUN atlas-lint db-no-destructive-migration,db-backward-compatible-migration --dir apps/api -- \
        atlas migrate lint --env local --latest 1
    fi
    ;;

  apps/api/db/queries/*.sql | apps/api/sqlc.yaml)
    check $RUN sqlc gen-up-to-date --dir apps/api -- sqlc compile
    ;;

  api/openapi.yaml)
    check $RUN spectral api-kebab-case-paths,api-camel-case-properties,api-operation-metadata,api-documented-errors,api-errors-problem-json \
      -- spectral lint api/openapi.yaml --ruleset .spectral.yaml --fail-severity=warn --quiet
    ;;

  apps/*.ts | apps/*.tsx | packages/*.ts | packages/*.tsx | apps/*.js | apps/*.mjs | packages/*.js | packages/*.mjs)
    check $RUN eslint web-no-direct-http,web-domain-pure,web-lint -- eslint --max-warnings=0 "$file"
    pkg="$(cut -d/ -f1-2 <<<"$file")"
    [[ -f "$pkg/tsconfig.json" ]] && check $RUN tsc web-typecheck --dir "$pkg" -- tsc --noEmit
    ;;

  *.sh | *.bash | *.bats)
    check $RUN shellcheck sh-shellcheck -- shellcheck -f gcc "$file"
    check $RUN shfmt sh-format -- shfmt -d "$file"
    ;;

  *.md)
    check $RUN check-links docs-links-resolve -- scripts/harness/check-links.sh
    ;;
esac
exit 0
