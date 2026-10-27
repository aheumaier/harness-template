#!/usr/bin/env bash
# PreToolUse hook (Edit|Write|MultiEdit|NotebookEdit|Bash): blocks agent edits to harness files, the
# globs in scripts/harness/protected-paths.txt (AGENTS.md golden principle 5). A human allows edits by
# starting the session with HARNESS_EDIT=1 in the environment.
# File tools are matched exactly. Bash is a heuristic: it blocks a protected path that is a redirection
# target, a `tee` argument, or an argument of a file-changing command (sed -i, perl -i, mv, cp, rm, ln,
# chmod, touch, truncate, git checkout/restore/rm/mv/apply). Writes from inside an interpreter
# (`python -c`, `node -e`) are not seen; CODEOWNERS and the harness-integrity check catch them in review.
# Exit 2 blocks the call; unparseable input is blocked too (fail closed).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
# shellcheck source=../../scripts/harness/timing.sh
source "$ROOT/scripts/harness/timing.sh" 2>/dev/null && harness_time_exit hook protect-harness
[[ "${HARNESS_EDIT:-}" == 1 ]] && exit 0

block() {
  [[ -n "$1" ]] || {
    echo "protect-harness: blocked: cannot parse the hook input (is node on PATH?)" >&2
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

input="$(node -e 'let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
  try {
    const j = JSON.parse(s), t = j.tool_input ?? {};
    const target = t.file_path ?? t.notebook_path ?? "";
    process.stdout.write(["ok", j.cwd ?? "", j.tool_name ?? "", target, (t.command ?? "") + "\n."].join("\n"));
  } catch {}
})')"
[[ "$input" == ok$'\n'*$'\n'*$'\n'*$'\n'* ]] || block ""
input="${input#ok$'\n'}" input="${input%$'\n.'}" # the "." keeps $(...) from eating trailing newlines
cwd="${input%%$'\n'*}" input="${input#*$'\n'}"
tool="${input%%$'\n'*}" input="${input#*$'\n'}"
target="${input%%$'\n'*}" cmd="${input#*$'\n'}"
cwd="$(cd "$cwd" 2>/dev/null && pwd -P)" || cwd="$PWD"

# protected <path>: path (absolute or relative to cwd) is inside the repo and matches a protected glob.
protected() {
  local p="$1" dir
  [[ "$p" == /* ]] || p="$cwd/$p"
  dir="$(cd "$(dirname "$p")" 2>/dev/null && pwd -P)" || dir="$(dirname "$p")"
  p="$dir/$(basename "$p")"
  [[ "$p" == "$ROOT"/* ]] || return 1
  p="${p#"$ROOT"/}"
  # Pathspec glob → anchored ERE: `.` literal, `*` any characters including `/`.
  awk -v p="$p" '!/^#/ && NF { g = $1; gsub(/\./, "\\.", g); gsub(/\*/, ".*", g); if (p ~ "^" g "$") found = 1 }
    END { exit !found }' "$ROOT/scripts/harness/protected-paths.txt"
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
