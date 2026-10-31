#!/usr/bin/env bash
# Harness self-test: proves every seed rule's sensor still rejects its violation with a remediation.
#
#   scripts/harness/selftest.sh [fixture-name...]     (make harness-selftest [FIXTURE=name])
#
# Copies the repo (tracked + untracked, not ignored; node_modules and .bin are linked, not copied) into a
# temp git repo, then for each scripts/harness/fixtures/<name>/:
#   base/      optional overlay committed on `main` first (e.g. a table a later migration drops)
#   files/     overlay committed on branch `fixture` (the violation)
#   apply.sh   optional script run in the copy root after files/ (edits that are not whole files)
#   fixture.conf  RULE (rules.md id), TARGET (make target = the sensor), EXPECT (ERE the tool output must
#                 match), NEEDS_DOCKER=1 (optional), SCOPE=changed|full (optional HARNESS_SCOPE of the run,
#                 default the caller's: a scoped sensor's fixture sets changed, so CI's full scope does not
#                 bypass the affected set it tests), STACK=web|db|... (optional: the groups of layout.env's STACK
#                 the fixture needs beyond its TARGET's, e.g. an apply.sh that edits apps/web),
#                 TEMPLATE_ONLY=1 (optional: skipped in a product, i.e. when scripts/tool-versions.env sets
#                 HARNESS_UPSTREAM (ADR 0026), as its template-only sensor is, ADR 0038)
# and asserts `make TARGET` fails, its output matches EXPECT, and run.sh printed RULE's remediation.
# A fixture whose TARGET is in the Makefile's STACK_OFF (sensors of the groups STACK leaves out) or whose STACK
# names such a group is skipped (ADR 0033); a STACK naming an unknown group is a bad fixture.conf, and a skip
# under the full stack is a failure.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FIXTURES="$ROOT/scripts/harness/fixtures"
cd "$ROOT"
# shellcheck source=../tool-versions.env
source scripts/tool-versions.env
# shellcheck source=layout.sh
source scripts/harness/layout.sh || exit 2
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$ROOT/node_modules/.bin:$PATH"
# The targets STACK leaves out, from the Makefile itself (make 3.81 has no --eval: a second makefile on stdin).
STACK_OFF="$(printf 'selftest-stack-off:\n\t@echo %s\n' "\$(STACK_OFF)" |
  make -s --no-print-directory -C "$ROOT" -f Makefile -f - selftest-stack-off)" || {
  echo "harness-selftest: could not read STACK_OFF from the Makefile" >&2
  exit 2
}
# Empty in the template, the template's URL in a product (ADR 0026); from the file, as adoption-probe.sh reads it.
upstream="$(sed -n 's/^HARNESS_UPSTREAM=//p' scripts/tool-versions.env | tail -n 1)"
full_stack=1
for group in $HARNESS_STACK_GROUPS; do harness_stack_has "$group" || full_stack=0; done

names=("$@")
if ((${#names[@]} == 0)); then
  for d in "$FIXTURES"/*/; do names+=("$(basename "$d")"); done
fi

tmp="$(mktemp -d -t harness-selftest.XXXXXX)"
trap 'rm -rf "$tmp"' EXIT
work="$tmp/repo"
mkdir -p "$work"

# --- repo copy ----------------------------------------------------------------------------------------
git ls-files -co --exclude-standard | while read -r f; do [[ -e "$f" ]] && printf '%s\0' "$f"; done |
  tar -cf - --null -T - | tar -xf - -C "$work"
ln -s "$ROOT/node_modules" "$work/node_modules"
ln -s "$ROOT/.bin" "$work/.bin"
if harness_stack_has web; then # pnpm workspace links are relative: copy them
  for ws in "${WEB_DIRS[@]}"; do
    for nm in "$ws"/*/node_modules; do
      [[ -d "$nm" ]] && cp -R "$nm" "$work/$nm"
    done
  done
fi
# The copy's layout.env carries the effective stack and its runs drop HARNESS_STACK (below), so a fixture that
# edits STACK is not overridden by the caller's HARNESS_STACK.
sed -i.bak '/^STACK=/d' "$work/scripts/harness/layout.env" && rm "$work/scripts/harness/layout.env.bak"
echo "STACK=$STACK" >>"$work/scripts/harness/layout.env"
g() { git -C "$work" -c core.hooksPath=/dev/null -c user.name=harness-selftest -c user.email=selftest@localhost "$@"; }
g init -q -b main
printf '/node_modules\n/.bin\n' >>"$work/.git/info/exclude"
g add -A && g commit -qm base
base="$(g rev-parse HEAD)"

overlay() { # dir: copy a fixture overlay into the copy
  [[ -d "$1" ]] || return 0
  cp -R "$1/." "$work/"
}
commit() { # message: rehash migrations if touched, then commit (must change something)
  if harness_stack_has db && [[ -n "$(g status --porcelain -- "$MIGRATIONS_DIR")" ]]; then
    (cd "$work/$API_DIR" && atlas migrate hash --env local) || return 1
  fi
  [[ -n "$(g status --porcelain)" ]] || {
    echo "fixture changed nothing" >&2
    return 1
  }
  g add -A && g commit -qm "$1"
}

docker_ok() { command -v docker >/dev/null && docker info >/dev/null 2>&1; }

pass=0 fail=0 skip=0
for name in "${names[@]}"; do
  dir="$FIXTURES/$name"
  [[ -f "$dir/fixture.conf" ]] || {
    echo "✗ $name: no $dir/fixture.conf" >&2
    fail=$((fail + 1))
    continue
  }
  RULE='' TARGET='' EXPECT='' NEEDS_DOCKER=0 SCOPE='' STACK_NEEDS='' TEMPLATE_ONLY=0
  # Parsed, not sourced: only these keys, values optionally single- or double-quoted.
  while IFS='=' read -r key val; do
    val="${val#[\'\"]}"
    val="${val%[\'\"]}"
    case "$key" in RULE) RULE="$val" ;; TARGET) TARGET="$val" ;; EXPECT) EXPECT="$val" ;; NEEDS_DOCKER) NEEDS_DOCKER="$val" ;; SCOPE) SCOPE="$val" ;; STACK) STACK_NEEDS="$val" ;; TEMPLATE_ONLY) TEMPLATE_ONLY="$val" ;; esac
  done < <(grep -E '^(RULE|TARGET|EXPECT|NEEDS_DOCKER|SCOPE|STACK|TEMPLATE_ONLY)=' "$dir/fixture.conf")
  bad=''
  [[ "$TARGET" =~ ^[a-z0-9-]+$ && -n "$RULE" && -n "$EXPECT" && "$SCOPE" =~ ^(changed|full)?$ &&
    "$STACK_NEEDS" =~ ^([a-z]+(:[a-z]+)*)?$ && "$TEMPLATE_ONLY" =~ ^[01]$ ]] || bad=' '
  for group in ${STACK_NEEDS//:/ }; do
    [[ " $HARNESS_STACK_GROUPS " == *" $group "* ]] || bad=" (STACK: unknown group $group; known: $HARNESS_STACK_GROUPS)"
  done
  if [[ -n "$bad" ]]; then
    echo "✗ $name: bad fixture.conf${bad% }" >&2
    fail=$((fail + 1))
    continue
  fi
  off=''
  [[ " $STACK_OFF " != *" $TARGET "* ]] || off="$TARGET"
  for group in ${STACK_NEEDS//:/ }; do harness_stack_has "$group" || off="$group"; done
  if [[ -n "$off" && "$full_stack" == 1 ]]; then
    echo "✗ $name: skipped for $off under the full STACK=$STACK (the Makefile's STACK_OFF and selftest.sh disagree)" >&2
    fail=$((fail + 1))
    continue
  elif [[ -n "$off" ]]; then
    echo "- $name: skipped (STACK=$STACK leaves out $off)"
    skip=$((skip + 1))
    continue
  fi
  if [[ "$TEMPLATE_ONLY" == 1 && -n "$upstream" ]]; then
    echo "- $name: skipped (template-only: HARNESS_UPSTREAM is set, this repo is a product)"
    skip=$((skip + 1))
    continue
  fi
  if [[ "$NEEDS_DOCKER" == 1 ]] && ! docker_ok; then
    if [[ "${SELFTEST_SKIP_DOCKER:-0}" == 1 ]]; then
      echo "- $name: skipped (no Docker)"
      skip=$((skip + 1))
      continue
    fi
    echo "✗ $name: needs Docker (or SELFTEST_SKIP_DOCKER=1)" >&2
    fail=$((fail + 1))
    continue
  fi

  # .tmp/ is gitignored, so clean keeps it: drop it too, or one fixture's scratch (harness-behind-upstream's
  # clone in .tmp/upstream) is linted and scanned by every later fixture under HARNESS_SCOPE=full. One command
  # per line: set -e ignores a failure inside an && list. Other ignored state (.cache/, .turbo/, the linked
  # .bin) still carries over: tech-debt TD-147.
  g checkout -qf -B main "$base"
  g clean -fdq
  rm -rf "$work/.tmp"
  g branch -qD fixture 2>/dev/null || true
  start=$SECONDS
  out="$tmp/$name.out"
  {
    overlay "$dir/base" && { [[ ! -d "$dir/base" ]] || commit "fixture base"; } &&
      g checkout -qb fixture && overlay "$dir/files" &&
      { [[ ! -x "$dir/apply.sh" ]] || (cd "$work" && "$dir/apply.sh"); } && commit "fixture $name"
  } >"$out" 2>&1 || {
    echo "✗ $name: could not apply fixture" >&2
    cat "$out" >&2
    fail=$((fail + 1))
    continue
  }

  status=0
  # A fresh make in the copy: no HARNESS_STACK, and no command-line variables of the caller's make (MAKEFLAGS).
  env -u HARNESS_STACK -u MAKEFLAGS -u MFLAGS -u MAKELEVEL ${SCOPE:+"HARNESS_SCOPE=$SCOPE"} make -C "$work" --no-print-directory "$TARGET" BASE_REF=main >"$out" 2>&1 || status=$?

  problems=()
  ((status != 0)) || problems+=("make $TARGET passed; expected it to fail")
  grep -Eq -- "$EXPECT" "$out" || problems+=("output does not match EXPECT /$EXPECT/")
  # run.sh prints "rule: <id>" followed by a non-empty "remediation: <text>" line.
  awk -v id="$RULE" '$1 == "rule:" && $2 == id { getline; if ($1 == "remediation:" && NF > 1) ok=1 } END { exit !ok }' "$out" ||
    problems+=("no remediation printed for rule $RULE")

  if ((${#problems[@]} == 0)); then
    echo "✓ $name → $TARGET [$RULE] ($((SECONDS - start))s)"
    pass=$((pass + 1))
  else
    {
      echo "✗ $name → $TARGET [$RULE]"
      printf '    %s\n' "${problems[@]}"
      sed 's/^/    | /' "$out" | head -n 60
    } >&2
    fail=$((fail + 1))
  fi
done

echo "harness-selftest: $pass passed, $fail failed, $skip skipped"
((fail == 0))
