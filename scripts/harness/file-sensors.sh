# shellcheck shell=bash
# Sourced. The per-file sensor list: which cheap sensor a single changed file gets. Shared by the pre-commit git
# hook (scripts/git-hooks/pre-commit) and meant for .claude/hooks/fast-check.sh, so both pick sensors from one
# table (exec plan 0003, Q2). Each caller decides what it runs per kind; this file only classifies. The paths
# come from scripts/harness/layout.env, read here through layout.sh.
#
#   harness_file_kind <repo-relative path>   prints one kind, or nothing when no per-file sensor applies:
#
#   skip            deliberate violations (fixture overlays) and vendored spec-kit (ADR 0011): never checked
#   generated       generated code (GENERATED_GLOBS): change its source and run `make gen` (rules.md#gen-up-to-date)
#   go-analyzers    Go in API_DIR/tools/analyzers (own module)
#   go-harness      Go in API_DIR/tools/harness (own module)
#   go              any other Go under API_DIR
#   migration       MIGRATIONS_DIR/*.sql
#   sql             sqlc queries (QUERIES_DIR) and config
#   openapi         SPEC
#   web             TS/JS in the WEB_GLOBS workspaces
#   sh              shell, bats and git hooks (the Makefile's SH_FILES)
#   md              Markdown

# shellcheck source=layout.sh
source "$(dirname "${BASH_SOURCE[0]}")/layout.sh" || return 2

harness_file_kind() {
  local g d
  case "$1" in
    scripts/harness/fixtures/*/files/* | .specify/scripts/* | .specify/templates/*) echo skip && return ;;
    .specify/*.sh | .specify/*.bash | .specify/*.bats) echo skip && return ;; # the Makefile's SH_FILES excludes .specify/
  esac
  for g in "${GENERATED_GLOBS[@]}"; do
    # shellcheck disable=SC2254 # reason: $g is a glob from layout.env, matched as one (rule: sh-shellcheck)
    case "$1" in $g) echo generated && return ;; esac
  done
  case "$1" in
    "$API_PREFIX"tools/analyzers/*.go) echo go-analyzers ;;
    "$API_PREFIX"tools/harness/*.go) echo go-harness ;;
    "$API_PREFIX"*.go) echo go ;;
    "$MIGRATIONS_DIR"/*.sql) echo migration ;;
    "$QUERIES_DIR"/*.sql | "$API_PREFIX"sqlc.yaml) echo sql ;;
    "$SPEC") echo openapi ;;
    *.ts | *.tsx | *.js | *.mjs)
      for d in "${WEB_DIRS[@]}"; do
        [[ "$1" != "$d"/* ]] || { echo web && return; }
      done
      ;;
    *.sh | *.bash | *.bats | scripts/git-hooks/*) echo sh ;;
    *.md) echo md ;;
  esac
}
