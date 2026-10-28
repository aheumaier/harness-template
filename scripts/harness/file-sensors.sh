# shellcheck shell=bash
# Sourced. The per-file sensor list: which cheap sensor a single changed file gets. Shared by the pre-commit git
# hook (scripts/git-hooks/pre-commit) and meant for .claude/hooks/fast-check.sh, so both pick sensors from one
# table (exec plan 0003, Q2). Each caller decides what it runs per kind; this file only classifies.
#
#   harness_file_kind <repo-relative path>   prints one kind, or nothing when no per-file sensor applies:
#
#   skip            deliberate violations (fixture overlays) and vendored spec-kit (ADR 0011): never checked
#   generated       generated code: change its source and run `make gen` (rules.md#gen-up-to-date)
#   go-analyzers    Go in apps/api/tools/analyzers (own module)
#   go-harness      Go in apps/api/tools/harness (own module)
#   go              any other Go under apps/api
#   migration       apps/api/db/migrations/*.sql
#   sql             sqlc queries and config
#   openapi         api/openapi.yaml
#   web             TS/JS in apps/ and packages/
#   sh              shell, bats and git hooks (the Makefile's SH_FILES)
#   md              Markdown

harness_file_kind() {
  case "$1" in
    scripts/harness/fixtures/*/files/* | .specify/scripts/* | .specify/templates/*) echo skip ;;
    *.gen.go | apps/api/internal/*/store/*.go | packages/api-client/src/gen/* | docs/generated/*) echo generated ;;
    apps/api/tools/analyzers/*.go) echo go-analyzers ;;
    apps/api/tools/harness/*.go) echo go-harness ;;
    apps/api/*.go) echo go ;;
    apps/api/db/migrations/*.sql) echo migration ;;
    apps/api/db/queries/*.sql | apps/api/sqlc.yaml) echo sql ;;
    api/openapi.yaml) echo openapi ;;
    apps/*.ts | apps/*.tsx | packages/*.ts | packages/*.tsx | apps/*.js | apps/*.mjs | packages/*.js | packages/*.mjs) echo web ;;
    *.sh | *.bash | *.bats | scripts/git-hooks/*) echo sh ;;
    *.md) echo md ;;
  esac
}
