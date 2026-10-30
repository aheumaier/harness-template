#!/usr/bin/env bash
# PreToolUse hook (Edit|Write|MultiEdit|NotebookEdit|Bash): blocks agent edits to harness files, the
# globs in scripts/harness/protected-paths.txt (AGENTS.md golden principle 5). A human allows edits by
# starting the session with HARNESS_EDIT=1 in the environment.
# File tools are matched exactly. Bash is a heuristic: it blocks a protected path that is a redirection
# target, a `tee` argument, or an argument of a file-changing command (sed -i, perl -i, mv, cp, rm, ln,
# chmod, touch, truncate, git checkout/restore/rm/mv/apply). Writes from inside an interpreter
# (`python -c`, `node -e`) are not seen; CODEOWNERS and the harness-integrity check catch them in review.
# Paths are judged inside every worktree of this repository (git worktree list), not only this checkout: a
# session rooted in one worktree that edits a sibling lane worktree or a nested `.claude/worktrees/<agent>`
# meets the same globs, from this checkout's list (a worktree cannot loosen its own copy).
# Exit 2 blocks the call; unparseable input is blocked too (fail closed).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook protect-harness
[[ "${HARNESS_EDIT:-}" == 1 ]] && exit 0
[[ -x "$ROOT/.bin/harness" ]] || {
  # shellcheck source=../../scripts/harness/hook-bootstrap.sh
  source "$ROOT/scripts/harness/hook-bootstrap.sh" && harness_missing protect-harness
  exit 2
}

block() {
  [[ -n "$1" ]] || {
    echo "protect-harness: blocked: cannot parse the hook input" >&2
    exit 2
  }
  cat >&2 <<EOF
protect-harness: blocked: $1 is a harness file (AGENTS.md golden principle 5).
Linter configs, sensors, hooks and CI define what "good" means here; changing them to get a green check
hides the problem instead of fixing it. Fix the code the sensor points at. If the rule itself is wrong,
do not edit it: propose the change in the PR description (Harness change: Y, the file, the diff, why).
EOF
  exit 2
}

# The "." keeps $(...) from eating trailing newlines of the command.
input="$("$ROOT/.bin/harness" hook-input cwd tool_name target command 2>/dev/null && echo .)" || block ""
input="${input%.}"
cwd="${input%%$'\n'*}" input="${input#*$'\n'}"
tool="${input%%$'\n'*}" input="${input#*$'\n'}"
target="${input%%$'\n'*}" cmd="${input#*$'\n'}"
cwd="$(cd "$cwd" 2>/dev/null && pwd -P)" || cwd="$PWD"

# roots: this checkout and the worktrees of its repository (physical paths), asked of git once, only when a
# path needs them.
roots=() listed=""
list_roots() {
  [[ -z "$listed" ]] || return 0
  listed=1
  local out line
  # Fail closed (NFR-R2): without the list a sibling worktree would be unguarded.
  out="$(env -u GIT_DIR -u GIT_WORK_TREE -u GIT_COMMON_DIR git -C "$ROOT" worktree list --porcelain 2>/dev/null)" || {
    echo "protect-harness: blocked: cannot list the worktrees of $ROOT (git worktree list failed)" >&2
    exit 2
  }
  roots=("$ROOT")
  # Physical paths via the cd builtin, not one subshell per worktree (NFR-H1).
  while IFS= read -r line; do
    [[ "$line" == "worktree "* ]] && cd -P "${line#worktree }" 2>/dev/null && roots+=("$PWD")
  done <<<"$out"
  cd "$ROOT" || exit 2
}

# listed_root <dir>: dir is one of roots.
listed_root() {
  local r
  for r in "${roots[@]}"; do [[ "$r" == "$1" ]] && return 0; done
  return 1
}

# globbed <repo-relative path>: matches a glob of this checkout's protected-paths.txt.
globbed() {
  # Pathspec glob → anchored ERE: `.` literal, `*` any characters including `/`.
  awk -v p="$1" '!/^#/ && NF { g = $1; gsub(/\./, "\\.", g); gsub(/\*/, ".*", g); if (p ~ "^" g "$") found = 1 }
    END { exit !found }' "$ROOT/scripts/harness/protected-paths.txt"
}

# protected <path>: path (absolute or relative to cwd) is inside a worktree of the repo and, relative to any
# worktree that holds it, matches a protected glob. Every holder counts, so registering a fake worktree inside
# this one hides nothing; only an agent worktree nested at .claude/worktrees/<name> is judged as itself rather
# than as files under .claude/.
protected() {
  local p="$1" dir d r rel n
  [[ "$p" == /* ]] || p="$cwd/$p"
  dir="$(cd "$(dirname "$p")" 2>/dev/null && pwd -P)" || dir="$(dirname "$p")"
  p="$dir/$(basename "$p")"
  # The nearest .git above the path: none (this checkout still counts with its .git moved away) or this
  # checkout's own means only this checkout, without a git call (NFR-H1); anything else asks for every worktree.
  d="${p%/*}"
  while [[ -n "$d" && ! -e "$d/.git" ]]; do d="${d%/*}"; done
  if [[ -z "$d" || "$d" == "$ROOT" ]]; then
    [[ "$p" == "$ROOT"/* ]] && globbed "${p#"$ROOT"/}"
    return
  fi
  list_roots
  for r in "${roots[@]}"; do
    [[ "$p" == "$r"/* ]] || continue
    rel="${p#"$r"/}"
    if [[ "$rel" == .claude/worktrees/*/* ]]; then
      n="${rel#.claude/worktrees/}"
      listed_root "$r/.claude/worktrees/${n%%/*}" && continue
    fi
    globbed "$rel" && return 0
  done
  return 1
}

if [[ "$tool" != Bash ]]; then
  [[ -n "$target" ]] && protected "$target" && block "${target#"$ROOT"/}"
  exit 0
fi

# Bash: one simple command per line (split on ; & | and newlines), then look at its words.
while IFS= read -r part; do
  read -ra w <<<"$part" || true
  ((${#w[@]})) || continue
  # writer: 1 = every argument is written, 2 = only the last one (the destination).
  writer=0
  case "${w[0]}" in
    cp | ln | install) writer=2 ;;
    mv | rm | chmod | touch | truncate | tee) writer=1 ;;
    sed | perl) [[ " ${w[*]} " == *" -i"* ]] && writer=1 ;;
    git) [[ " ${w[*]} " =~ \ (checkout|restore|rm|mv|apply)\  ]] && writer=1 ;;
  esac
  ((writer == 2)) && w=("${w[0]}" "${w[${#w[@]} - 1]}")
  prev=""
  for a in "${w[@]:1}"; do
    a="${a//[\"\']/}"
    if [[ "$a" == *\>?* ]]; then # >file, >>file, 2>file
      protected "${a##*>}" && block "${a##*>}"
    elif [[ "$prev" == *\> ]] || { ((writer)) && [[ "$a" != -* ]]; }; then
      protected "$a" && block "$a"
    fi
    prev="$a"
  done
done < <(printf '%s\n' "$cmd" | tr ';&|' '\n')
exit 0
