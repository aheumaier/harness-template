#!/usr/bin/env bash
# The git pre-commit hook's checks (scripts/git-hooks/pre-commit runs this; exec plan 0003, Q2): checks what is
# about to be committed, the staged files as they are in the index (never the working tree), with the cheap
# sensors only:
#
#   every staged file   repo-no-secrets   scripts/harness/trivy.sh secret --staged (offline)
#   Go                  go-format         golangci-lint fmt (gofmt + goimports, apps/api/.golangci.yml)
#   shell, bats, hooks  sh-shellcheck, sh-format
#   Markdown            docs-links-resolve, docs-claude-imports-agents (on the whole staged tree)
#
# Which file gets which sensor: scripts/harness/file-sensors.sh. No eslint or tsc (up to 1 s per file, NFR-H2)
# and never the full `make check` (golden principle 6). Budget 2 s for a typical commit, a warning only
# (budgets.txt row pre-commit, NFR-H8). `git commit --no-verify` skips the hook; CI runs every sensor anyway.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source=timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook pre-commit
cd "$ROOT" || exit 1
# shellcheck source=../tool-versions.env
source scripts/tool-versions.env
# shellcheck source=file-sensors.sh
source scripts/harness/file-sensors.sh
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$PATH"
export HARNESS_SCOPE=pre-commit # tags the timings of the sensors below
RUN="$ROOT/scripts/harness/run.sh"
# `git commit <paths>` and `-a` hand the hook a temporary index; keep using it after a cd.
if [[ -n "${GIT_INDEX_FILE:-}" && "$GIT_INDEX_FILE" != /* ]]; then export GIT_INDEX_FILE="$ROOT/$GIT_INDEX_FILE"; fi

fail() { # the hook cannot check: exit 2, blocking the commit (fail closed)
  echo "pre-commit: $*" >&2
  exit 2
}
need() { # fail closed on a missing tool
  local t
  for t in "$@"; do [[ -x ".bin/$t" ]] || fail ".bin/$t is missing: run make setup"; done
}

check_staged() ( # a subshell: its EXIT trap removes the staged tree, the parent's records the hook's time
  status=0
  # Outside the repo, like trivy.sh: a copy inside it would be walked by make check's sensors.
  tmp="${TMPDIR:-/tmp}"
  tree="$(mktemp -d "${tmp%/}/pre-commit.XXXXXX")" || fail "mktemp failed"
  trap 'rm -rf "$tree" "$tree.list"' EXIT
  git diff -z --cached --name-only --no-renames --diff-filter=ACMRT -- >"$tree.list" ||
    fail "git could not list the staged files"
  [[ -s "$tree.list" ]] || exit 0

  go=() sh=() md=0
  while IFS= read -r -d '' f; do
    case "$(harness_file_kind "$f")" in
      go | go-analyzers | go-harness) go+=("$f") ;;
      sh) sh+=("$f") ;;
      md) md=1 ;;
    esac
  done <"$tree.list"
  ((${#go[@]})) && need golangci-lint-custom
  ((${#sh[@]})) && need shellcheck shfmt
  ((md)) && need harness

  "$RUN" secrets-staged repo-no-secrets -- scripts/harness/trivy.sh secret --staged || status=1
  ((${#go[@]} + ${#sh[@]} + md)) || exit "$status"

  # The whole index, so that `source` (shellcheck), go.mod (goimports), configs and link targets resolve as
  # they will be committed.
  git checkout-index -a --prefix="$tree/" || fail "git could not write the staged files to $tree"
  in_tree() { # <sensor-id> <rule-ids> <command...>: run.sh with the staged tree as working directory
    local id="$1" rules="$2"
    shift 2
    "$RUN" "$id" "$rules" -- bash -c 'cd "$1" || exit 2; shift; exec "$@"' in-tree "$tree" "$@" || status=1
  }
  if ((${#go[@]})); then
    in_tree golangci-fmt go-format golangci-lint-custom fmt --diff --config apps/api/.golangci.yml "${go[@]}"
  fi
  if ((${#sh[@]})); then
    in_tree shellcheck sh-shellcheck shellcheck -f gcc "${sh[@]}"
    in_tree shfmt sh-format shfmt -d "${sh[@]}"
  fi
  if ((md)); then # check-links lists files with git: the repo's index, with the staged tree as working tree
    GIT_DIR="$(git rev-parse --absolute-git-dir)" GIT_WORK_TREE="$tree" \
      in_tree check-links docs-links-resolve,docs-claude-imports-agents harness check-links .
  fi
  exit "$status"
)

check_staged
status=$?
if ((status == 1)); then
  echo "pre-commit: commit blocked. The hook checks the index, not the working tree: fix the files above, \`git add\` them and commit again." >&2
fi
exit "$status"
