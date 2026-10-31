#!/usr/bin/env bats
# .github/CODEOWNERS mirrors scripts/harness/protected-paths.txt: every harness file needs an owner's review.
# The list names the harness's own workflows and actions, so a product's workflow beside them is not protected
# (ADR 0035), and a new harness workflow is protected only once it is listed, in CODEOWNERS and the list alike.
bats_require_minimum_version 1.5.0
load helpers

# protected_paths [mark]: the globs in protected-paths.txt, or only those marked with <mark>.
protected_paths() {
  local glob mark
  while read -r glob mark _; do
    if [[ -z ${1-} || ${mark-} == "$1" ]]; then echo "$glob"; fi
  done < <(grep -Ev '^[[:space:]]*(#|$)' "$REPO/scripts/harness/protected-paths.txt")
}

# owned_paths: every CODEOWNERS pattern without its leading and trailing slash.
owned_paths() {
  awk '!/^#/ && NF { p = $1; sub(/^\//, "", p); sub(/\/$/, "", p); print p }' "$REPO/.github/CODEOWNERS"
}

@test "every protected path has a code owner" {
  owners="$(owned_paths)"
  missing=()
  while read -r glob; do
    p="${glob#\*}" p="${p%/\*}"
    covered=0
    while IFS= read -r o; do
      [[ "$p" == "$o" || "$p" == "$o"/* ]] && covered=1
    done <<<"$owners"
    ((covered)) || missing+=("$glob")
  done < <(protected_paths)
  output="${missing[*]-}"
  [[ -z "$output" ]] || {
    echo "no CODEOWNERS line covers: $output" >&2
    return 1
  }
}

@test "a workflow or action is a harness config exactly when its first line says it is a harness file" {
  configs=()
  while read -r glob; do configs+=("$glob"); done < <(protected_paths config)
  cd "$REPO"
  listed="$(git ls-files --cached --others --exclude-standard -- "${configs[@]}" | grep -E '^\.github/(workflows|actions)/' | sort -u || true)"
  headed="$(git ls-files --cached --others --exclude-standard -- .github/workflows .github/actions | while IFS= read -r f; do
    [[ -f $f ]] && head -n1 "$f" | grep -q '^# Harness file (protected)' && echo "$f"
  done | sort -u || true)"
  [[ "$listed" == "$headed" ]] || {
    echo "protected-paths.txt configs (<) and workflows or actions headed '# Harness file (protected)' (>) differ:" >&2
    echo "fix: drop the '# Harness file (protected)' line from a product workflow, or list it in" \
      "protected-paths.txt and .github/CODEOWNERS in a harness PR" >&2
    diff <(echo "$listed") <(echo "$headed") >&2
    return 1
  }
}

@test "the .github/ harness files are tracked and named one by one in CODEOWNERS and protected-paths.txt alike" {
  cd "$REPO"
  listed="$(protected_paths | grep '^\.github/' | sed 's|/\*$||' | sort -u)"
  # The workflows and actions directories are owned for the owner's eye (ADR 0035), not as harness files.
  owned="$(owned_paths | grep '^\.github/' | grep -Ev '^\.github/(workflows|actions)$' | sort -u)"
  [[ "$listed" == "$owned" ]] || {
    echo "protected-paths.txt (<) and .github/CODEOWNERS (>) name different .github/ harness files:" >&2
    diff <(echo "$listed") <(echo "$owned") >&2
    return 1
  }
  untracked=()
  while read -r p; do
    [[ $p == *'*'* ]] || git ls-files --error-unmatch -- "$p" >/dev/null 2>&1 || untracked+=("$p")
  done < <(protected_paths | grep '^\.github/')
  output="${untracked[*]-}"
  [[ -z "$output" ]] || {
    echo "protected-paths.txt names .github/ files git does not track (renamed or deleted?): $output" >&2
    return 1
  }
}
