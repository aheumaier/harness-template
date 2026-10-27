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
#                 match), NEEDS_DOCKER=1 (optional)
# and asserts `make TARGET` fails, its output matches EXPECT, and run.sh printed RULE's remediation.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
FIXTURES="$ROOT/scripts/harness/fixtures"
cd "$ROOT"
# shellcheck source=../tool-versions.env
source scripts/tool-versions.env
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$ROOT/node_modules/.bin:$PATH"

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
for nm in apps/*/node_modules packages/*/node_modules; do # pnpm workspace links are relative: copy them
  [[ -d "$nm" ]] && cp -R "$nm" "$work/$nm"
done
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
  if [[ -n "$(g status --porcelain -- apps/api/db/migrations)" ]]; then
    (cd "$work/apps/api" && atlas migrate hash --env local) || return 1
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
  RULE='' TARGET='' EXPECT='' NEEDS_DOCKER=0
  # Parsed, not sourced: only these keys, values optionally single- or double-quoted.
  while IFS='=' read -r key val; do
    val="${val#[\'\"]}"
    val="${val%[\'\"]}"
    case "$key" in RULE) RULE="$val" ;; TARGET) TARGET="$val" ;; EXPECT) EXPECT="$val" ;; NEEDS_DOCKER) NEEDS_DOCKER="$val" ;; esac
  done < <(grep -E '^(RULE|TARGET|EXPECT|NEEDS_DOCKER)=' "$dir/fixture.conf")
  [[ "$TARGET" =~ ^[a-z0-9-]+$ && -n "$RULE" && -n "$EXPECT" ]] || {
    echo "✗ $name: bad fixture.conf" >&2
    fail=$((fail + 1))
    continue
  }
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

  g checkout -qf -B main "$base" && g clean -fdq
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
  make -C "$work" --no-print-directory "$TARGET" BASE_REF=main >"$out" 2>&1 || status=$?

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
