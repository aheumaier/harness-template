#!/usr/bin/env bash
# PreToolUse hook (Bash): blocks force pushes, pushes to main and recursive/forced rm outside the repo.
# Backs up the permissions.deny patterns in .claude/settings.json, which only match command text
# (e.g. a bare `git push` while on main, or `rm -rf "$HOME/x"`, slip past them). Heuristic, not a sandbox:
# it tokenizes with shell word splitting and never evaluates the command. Exit 2 blocks the call.
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
input="$(node -e 'let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
  try { const j = JSON.parse(s); process.stdout.write((j.cwd ?? "") + "\n" + (j.tool_input?.command ?? "")); } catch {}
})')"
cwd="${input%%$'\n'*}" cmd="${input#*$'\n'}"
[[ -n "$cmd" && "$input" == *$'\n'* ]] || exit 0
[[ -d "$cwd" ]] || cwd="$PWD"

block() {
  echo "guard-bash: blocked: $*" >&2
  exit 2
}

# Prints one simple command per line: splits on ; & | and drops heredoc bodies, which are data
# (commit messages, file contents), not commands.
simple_commands() {
  awk '
    delim != "" { line = $0; if (tabs) sub(/^\t+/, "", line); if (line == delim) delim = ""; next }
    { line = $0; gsub(/[;&|]/, "\n", line); print line
      if (match($0, /<<-?[ \t]*["\047]?[A-Za-z_][A-Za-z0-9_]*["\047]?/)) {
        d = substr($0, RSTART, RLENGTH); tabs = (d ~ /^<<-/)
        sub(/^<<-?[ \t]*/, "", d); gsub(/["\047]/, "", d); delim = d
      } }'
}

# Inspect each simple command's words.
while IFS= read -r part; do
  read -ra w <<<"$part" || true
  ((${#w[@]})) || continue
  # Skip leading env assignments (FOO=1 git push).
  i=0
  while ((i < ${#w[@]})) && [[ "${w[i]}" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]]; do i=$((i + 1)); done
  w=("${w[@]:i}")
  ((${#w[@]})) || continue

  if [[ "${w[0]}" == git ]]; then
    # git [-C dir] [-c k=v] push ...
    j=1
    while ((j < ${#w[@]})) && [[ "${w[j]}" == -* ]]; do
      [[ "${w[j]}" == -[Cc] ]] && j=$((j + 1))
      j=$((j + 1))
    done
    [[ "${w[j]:-}" == push ]] || continue
    args=("${w[@]:j+1}")
    positional=() # bash 3.2: empty arrays are "unbound" under set -u, hence ${a[@]+...}
    for a in ${args[@]+"${args[@]}"}; do
      case "$a" in
        -f | --force | --force-with-lease* | --force-if-includes | --mirror | --delete | -d) block "force/destructive push ($a). Push a normal branch and open a PR." ;;
        -*) ;;
        +*) block "force refspec ($a). Push a normal branch and open a PR." ;;
        *) positional+=("$a") ;;
      esac
    done
    for ref in ${positional[@]+"${positional[@]:1}"}; do
      [[ "${ref##*:}" =~ ^(refs/heads/)?(main|master)$ ]] && block "push to ${ref##*:}. Push a feature branch and open a PR."
    done
    if ((${#positional[@]} <= 1)); then # no refspec: pushes the current branch
      branch="$(git -C "$cwd" symbolic-ref --short -q HEAD || true)"
      [[ "$branch" =~ ^(main|master)$ ]] && block "\`git push\` from $branch pushes main. Create a branch first."
    fi
  elif [[ "${w[0]}" == rm || "${w[0]}" == */rm ]]; then
    recursive=0
    for a in "${w[@]:1}"; do [[ "$a" =~ ^-[a-zA-Z]*[rRf] || "$a" == --recursive || "$a" == --force ]] && recursive=1; done
    ((recursive)) || continue
    for a in "${w[@]:1}"; do
      [[ "$a" == -* ]] && continue
      a="${a%\"}"
      a="${a#\"}"
      a="${a%\'}"
      a="${a#\'}"
      [[ "$a" == *'$'* || "$a" == '~'* || "$a" == *'`'* ]] && block "rm -r/-f with an unexpanded path ($a). Use a literal path inside the repo."
      [[ "$a" == /* ]] && abs="$a" || abs="$cwd/$a"
      # Resolve .. lexically (the target may not exist).
      norm="$(awk -v p="$abs" 'BEGIN { n = split(p, s, "/"); k = 0
        for (i = 1; i <= n; i++) { if (s[i] == "" || s[i] == ".") continue; if (s[i] == "..") { if (k) k--; continue } o[++k] = s[i] }
        r = ""; for (i = 1; i <= k; i++) r = r "/" o[i]; print (r == "" ? "/" : r) }')"
      [[ "$norm" == "$ROOT"/* ]] || block "rm -r/-f outside the repository ($a → $norm). Only delete paths inside $ROOT."
    done
  fi
done < <(simple_commands <<<"$cmd")
exit 0
