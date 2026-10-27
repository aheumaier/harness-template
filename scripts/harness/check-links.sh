#!/usr/bin/env bash
# Fails on relative Markdown links (and #anchors) in tracked *.md files that do not resolve, and on an
# AGENTS.md without a sibling CLAUDE.md that imports it (Claude Code reads CLAUDE.md, other agents AGENTS.md).
# External links (scheme:) are not checked: they are not deterministic. Anchors use GitHub's heading slugs.
set -euo pipefail
cd "$(dirname "$0")/../.."

slugs() { # file -> one GitHub heading slug per line
  awk '/^```/ { code = !code } !code && /^#+ / { sub(/^#+ +/, ""); print }' "$1" |
    tr '[:upper:]' '[:lower:]' | sed -e 's/[^a-z0-9 _-]//g' -e 's/ /-/g'
}

bad=0
while IFS= read -r md; do
  dir="$(dirname "$md")"
  # [text](target) outside fenced blocks and code spans; drop an optional "title".
  while IFS= read -r link; do
    target="${link%% *}"
    [[ "$target" =~ ^[a-z][a-z0-9+.-]*: ]] && continue
    path="${target%%#*}" anchor=""
    [[ "$target" == *#* ]] && anchor="${target#*#}"
    file="$md"
    if [[ -n "$path" ]]; then
      file="$dir/$path"
      [[ -e "$file" ]] || {
        echo "$md: broken link ($target): $file does not exist [docs-links-resolve]"
        bad=1
        continue
      }
    fi
    if [[ -n "$anchor" && "$file" == *.md && -f "$file" ]]; then
      slugs "$file" | grep -qxF -- "$anchor" || {
        echo "$md: broken anchor ($target): no heading #$anchor in $file [docs-links-resolve]"
        bad=1
      }
    fi
  done < <(awk '/^```/ { code = !code; next } !code' "$md" | sed -e 's/`[^`]*`//g' | grep -oE '\]\([^)]+\)' | sed -E 's/^\]\((.*)\)$/\1/' || true)
done < <(git ls-files -co --exclude-standard '*.md' | grep -vE '(^|/)(node_modules|fixtures)/')

while IFS= read -r agents; do
  claude="$(dirname "$agents")/CLAUDE.md"
  grep -qx '@AGENTS.md' "$claude" 2>/dev/null || {
    echo "$agents: no ${claude#./} with an '@AGENTS.md' line next to it [docs-claude-imports-agents]"
    bad=1
  }
done < <(git ls-files -co --exclude-standard '*AGENTS.md' | grep -E '(^|/)AGENTS\.md$' | grep -vE '(^|/)(node_modules|fixtures)/')
exit "$bad"
