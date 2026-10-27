#!/usr/bin/env bash
# PreToolUse hook (Bash): blocks force pushes, pushes to main, recursive rm outside the repo and commands
# that name a .env file. Backs up the permissions.deny patterns in .claude/settings.json, which only match
# command text (e.g. a bare `git push` while on main, or `rm -rf "$HOME/x"`, slip past them).
# Heuristic, not a sandbox: it tokenizes with shell word splitting and never evaluates the command. It errs
# towards blocking: `;&|` split even inside quotes, any unexpanded rm path is refused, temp dirs outside
# the repo are not deletable. Exit 2 blocks the call; unparseable input is blocked too (fail closed).
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"

block() {
  echo "guard-bash: blocked: $*" >&2
  exit 2
}

input="$(node -e 'let s = ""; process.stdin.on("data", (d) => (s += d)).on("end", () => {
  try { const j = JSON.parse(s); process.stdout.write("ok\n" + (j.cwd ?? "") + "\n" + (j.tool_input?.command ?? "")); } catch {}
})')"
[[ "$input" == ok$'\n'*$'\n'* ]] || block "cannot parse the hook input (is node on PATH?)"
input="${input#ok$'\n'}"
cwd="${input%%$'\n'*}" cmd="${input#*$'\n'}"
[[ -n "$cmd" ]] || exit 0
cwd="$(cd "$cwd" 2>/dev/null && pwd -P)" || cwd="$PWD"
cmd="${cmd//\\$'\n'/ }" # join line continuations

# Prints one simple command per line: splits on ; & | ` $( and drops heredoc bodies, which are data
# (commit messages, file contents), not commands. A "heredoc" that never ends (e.g. `<<EOF` inside quotes)
# was not one: its lines are checked after all. `<<<` (here-string) does not start a heredoc.
simple_commands() {
  awk '
    function emit(s) { gsub(/[;&|`]|\$\(/, "\n", s); print s }
    delim != "" { body[++n] = $0; line = $0; if (tabs) sub(/^\t+/, "", line); if (line == delim) { delim = ""; n = 0 }; next }
    { emit($0)
      if (match($0, /(^|[^<])<<-?[ \t]*["\047]?[A-Za-z_][A-Za-z0-9_]*["\047]?/)) {
        d = substr($0, RSTART, RLENGTH); sub(/^[^<]/, "", d); tabs = (d ~ /^<<-/)
        sub(/^<<-?[ \t]*/, "", d); gsub(/["\047]/, "", d); delim = d
      } }
    END { for (i = 1; i <= n; i++) emit(body[i]) }'
}

# Words that only prefix the real command (`sudo rm`, `then rm`, `xargs rm`); their options are skipped too.
PREFIX=' if then else elif do while until time sudo env command exec nohup nice xargs ! '

while IFS= read -r part; do
  read -ra raw <<<"$part" || true
  # w: words without quoting and grouping ("rm", \rm, (rm, { rm, $(rm, `rm, /x) and /x`); raw keeps them
  # as typed, to spot unexpanded paths.
  w=()
  for a in ${raw[@]+"${raw[@]}"}; do
    while [[ "$a" == [\"\'\\\(\{\`]* || "$a" == '$('* ]]; do [[ "$a" == '$('* ]] && a="${a:2}" || a="${a:1}"; done
    while [[ "$a" == *[\"\'\)\}\`] ]]; do a="${a%?}"; done
    w+=("$a")
  done
  # Skip env assignments (FOO=1 git push), prefix words and their options.
  i=0 prefixed=0
  while ((i < ${#w[@]})); do
    if [[ -z "${w[i]}" || "${w[i]}" =~ ^[A-Za-z_][A-Za-z0-9_]*= || "$PREFIX" == *" ${w[i]} "* ]]; then
      [[ "$PREFIX" == *" ${w[i]} "* ]] && prefixed=1
    elif ! ((prefixed)) || [[ "${w[i]}" != -* ]]; then
      break
    fi
    i=$((i + 1))
  done
  ((i < ${#w[@]})) || continue
  w=("${w[@]:i}") raw=("${raw[@]:i}")

  for a in "${w[@]}"; do
    a="${a##*[<>]}"
    [[ "$a" =~ (^|/)\.env(\.[A-Za-z0-9_.-]+)?$ && "$a" != *.env.example ]] &&
      block "$a: .env files hold this worktree's secrets; never read, copy or print them. Use \`make\` targets, which load them."
  done

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
        --force | --force-with-lease* | --force-if-includes | --mirror | --delete) block "force/destructive push ($a). Push a normal branch and open a PR." ;;
        --*) ;;
        -*[fd]*) block "force/destructive push ($a). Push a normal branch and open a PR." ;;
        -*) ;;
        +*) block "force refspec ($a). Push a normal branch and open a PR." ;;
        *) positional+=("$a") ;;
      esac
    done
    branch="$(git -C "$cwd" symbolic-ref --short -q HEAD || true)"
    for ref in ${positional[@]+"${positional[@]:1}"}; do
      dst="${ref##*:}"
      [[ "$dst" == HEAD || "$dst" == @ ]] && dst="$branch"
      [[ "$dst" =~ ^(refs/heads/)?(main|master)$ ]] && block "push to $dst. Push a feature branch and open a PR."
    done
    if ((${#positional[@]} <= 1)); then # no refspec: pushes the current branch
      [[ "$branch" =~ ^(main|master)$ ]] && block "\`git push\` from $branch pushes main. Create a branch first."
    fi
  elif [[ "${w[0]}" == rm || "${w[0]}" == */rm ]]; then
    recursive=0
    for a in "${w[@]:1}"; do [[ "$a" =~ ^-[a-zA-Z]*[rR] || "$a" == --recursive ]] && recursive=1; done
    ((recursive)) || continue
    paths=0
    for ((k = 1; k < ${#w[@]}; k++)); do
      a="${w[k]}"
      [[ "$a" == -* || -z "$a" ]] && continue
      paths=$((paths + 1))
      [[ "${raw[k]}" == *[\$\`]* || "$a" == '~'* || "$a" == *'.*'* ]] &&
        block "rm -r with an unexpanded path (${raw[k]}). Use a literal path inside the repo."
      [[ "$a" == /* ]] && abs="$a" || abs="$cwd/$a"
      # Resolve .. lexically (the target may not exist).
      norm="$(awk -v p="$abs" 'BEGIN { n = split(p, s, "/"); k = 0
        for (i = 1; i <= n; i++) { if (s[i] == "" || s[i] == ".") continue; if (s[i] == "..") { if (k) k--; continue } o[++k] = s[i] }
        r = ""; for (i = 1; i <= k; i++) r = r "/" o[i]; print (r == "" ? "/" : r) }')"
      [[ "$norm" == "$ROOT"/* ]] || block "rm -r outside the repository ($a → $norm). Only delete paths inside $ROOT."
    done
    ((paths)) || block "rm -r without a literal path (paths from xargs or stdin). Name the paths inside the repo."
  fi
done < <(simple_commands <<<"$cmd")
exit 0
