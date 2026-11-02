#!/usr/bin/env bash
# Adoption probe (rules.md#harness-adoption-probe, ADR 0038): README.md "(b) Brownfield project", followed step
# by step on a minimal brownfield repo, ends green. Without it the procedure drifts from the harness it adopts.
#
#   scripts/harness/adoption-probe.sh [skeleton-dir]     (make adoption-probe; CI's adoption-probe job)
#
# skeleton-dir (default scripts/templates/adoption-probe) holds repo/, the brownfield repo (a root go.mod,
# internal/<domain>/<layer>, an OpenAPI contract, no db, no web); layout.env, the layout step 5 writes for it;
# drop.txt, the paths step 6 drops; and fixtures/, the fixtures step 6 rewrites. Every file under repo/ and
# fixtures/, and any directory a sensor matches by path, carries a .tmpl suffix that the copy strips, so no sensor
# of this repo reads them. Before any heavy step the probe checks that README (b) still names exactly the paths
# its step 3 copies or skips and its step 6 drops (a finding when not). Then it commits repo/ into a temp git
# repo and adopts this checkout's harness into it:
#   1  branch harness/adopt, .harness-version
#   2  tool-versions.env, layout.env, layout.sh; API_DIR from the skeleton's layout.env; tools/harness;
#      the Makefile; make .bin/harness
#   3  copy the harness; merge AGENTS.md/CLAUDE.md, .claude/, .specify/, .gitignore (none exist yet)
#   4  git add, make rename (dry run, then WRITE=1)
#   5  layout.env, make layout-check, re-point the files of step 5's table that exist in this layout
#   6  drop what the layout lacks (web and db links, make new-domain, drop.txt) and this probe's own files and
#      rule, declined in .harness-version (its CI job, target and required check stay); port fixtures
#   7  make setup check harness-selftest, as README writes them (the skeleton's layout.env has STACK=go)
# Each hand edit is one the README asks for and must change its file; one that matches nothing is README drift.
# .bin is seeded with links to this checkout's pinned tools (their version stamps match, so install-tools.sh
# skips them); harness and golangci-lint-custom are built in the copy. Prints nothing on success; on failure, the
# step, its output and `README.md:<line>: step <n> … [harness-adoption-probe]`. Exit 1 on a finding, 2 when the
# probe cannot run (no skeleton, an invalid skeleton layout.env, a drop.txt line naming a path step 3 does not
# copy). ADOPTION_PROBE_KEEP=1 keeps the temp repo and prints its path. In a product (HARNESS_UPSTREAM set in
# scripts/tool-versions.env) it prints a notice and exits 0 before anything else.
set -euo pipefail

RULE=harness-adoption-probe
ROOT="$(cd "$(dirname "$0")/../.." && pwd -P)"
SKEL="${1:-$ROOT/scripts/templates/adoption-probe}"
exec 3>&2 # the probe's own stderr, for cannot() inside a step whose output goes to the log
tmp=''

# cannot <message>: the probe cannot run (exit 2), even from inside a step.
cannot() {
  echo "adoption-probe: $*" >&3
  [[ -z "$tmp" ]] || : >"$tmp/cannot"
  exit 2
}

# Template-only (ADR 0038): a non-empty HARNESS_UPSTREAM means this repo is a product (ADR 0026; make rename
# WRITE=1 writes it), whose README need not keep (b). Read from the file only, which is protected: a value in the
# environment (CI's setup action exports the key) never switches this required check off.
[[ -f "$ROOT/scripts/tool-versions.env" ]] || cannot "scripts/tool-versions.env is missing (copy it from the template)"
if [[ -n "$(sed -n 's/^HARNESS_UPSTREAM=//p' "$ROOT/scripts/tool-versions.env" | tail -n 1)" ]]; then
  echo "adoption-probe: HARNESS_UPSTREAM is set in scripts/tool-versions.env: this repo is a product and the probe is template-only (ADR 0038); nothing to run"
  exit 0
fi
# readme_step_line <n>: the line of README (b)'s step n, or of the (b) heading.
readme_step_line() {
  N="$1" awk '/^## \(b\) Brownfield project/ { b = NR } b && $0 ~ "^" ENVIRON["N"] "\\. \\*\\*" { print NR; f = 1; exit }
    END { if (!f) print (b ? b : 1) }' "$ROOT/README.md"
}
# finding <README line> <message>
finding() { echo "README.md:$1: $2 [$RULE]"; }
# readme_spans <from ERE> <to ERE> [bold]: "<line> <span>" for each path-like code span of README (b), from the
# line matching <from> to the line before the next one matching <to>; with bold, only in a bullet's bold lead.
readme_spans() {
  FROM="$1" TO="$2" BOLD="${3:-}" awk '/^## \(b\) Brownfield project/ { b = 1 }
    !b { next }
    r && $0 ~ ENVIRON["TO"] { exit }
    !r && $0 ~ ENVIRON["FROM"] { r = 1 }
    !r { next }
    { s = $0
      if (ENVIRON["BOLD"] != "") { if (s !~ /^   - \*\*/) next; s = substr(s, 8); sub(/\*\*.*/, "", s) }
      while (match(s, /`[^`]+`/)) {
        t = substr(s, RSTART + 1, RLENGTH - 2); s = substr(s, RSTART + RLENGTH)
        if (t ~ /^[A-Za-z0-9_.*<>\/-]+$/ && (t ~ /[.\/]/ || t == "Makefile")) print NR, t
      } }' "$ROOT/README.md"
}
in_list() {
  local x="$1" y
  shift
  for y in "$@"; do [[ "$x" == "$y" ]] && return 0; done
  return 1
}
# check_list <n> <spans file> <token>...: README (b) step n names exactly these paths, in both directions.
check_list() {
  local n="$1" spans="$2" line t bad=0
  shift 2
  while read -r line t; do
    in_list "$t" "$@" || {
      finding "$line" "step $n names \`$t\`, which the adoption probe neither handles nor skips"
      bad=1
    }
  done <"$spans"
  for t in "$@"; do
    cut -d' ' -f2 "$spans" | grep -qxF -- "$t" || {
      finding "$(readme_step_line "$n")" "step $n no longer names \`$t\`, which the adoption probe still handles"
      bad=1
    }
  done
  return "$bad"
}

in_dst() { (cd "$DST" && env "${unset_env[@]}" "$@"); }
g() { in_dst git -c core.hooksPath=/dev/null -c user.name=adoption-probe -c user.email=probe@localhost "$@"; }
# run_logged <command...>: the command under set -e with its output in $LOG; its exit status.
run_logged() {
  local rc
  set +e
  (
    set -e
    "$@"
  ) >"$LOG" 2>&1
  rc=$?
  set -e
  [[ ! -e "$tmp/cannot" ]] || exit 2
  return "$rc"
}
# prep <what> <command...>: the probe's own setup, no README step; a failure means it cannot run.
prep() {
  local what="$1"
  shift
  run_logged "$@" || {
    sed 's/^/  | /' "$LOG" | tail -n 30 >&3
    cannot "$what failed"
  }
}
# step <n> <what> <command...>: README (b) step n's work in the adopted repo, silent unless it fails.
step() {
  STEP_N="$1"
  local what="$2"
  shift 2
  run_logged "$@" && return 0
  echo "README.md \"(b) Brownfield project\" step $STEP_N ($what) failed on $SKEL:"
  { grep -v "\[$RULE\]\$" "$LOG" || true; } | sed 's/^/  | /' | tail -n 60
  grep "\[$RULE\]\$" "$LOG" ||
    finding "$(readme_step_line "$STEP_N")" "step $STEP_N ($what) no longer adopts the harness into a root-go.mod repo"
  exit 1
}
# put <src> <dst>: one file, its directory made first.
put() { mkdir -p "$(dirname "$2")" && cp "$1" "$2"; }
# copy_to <src> [<dst> [<exclude pathspec>...]]: this checkout's git-known files under src, at the same path in
# the adopted repo, or with src replaced by dst. A finding when src matches no file.
copy_to() {
  local src="$1" dst="${2:-}" f n=0
  shift
  [[ $# -eq 0 ]] || shift
  while IFS= read -r f; do
    [[ -e "$ROOT/$f" ]] || continue # deleted in the working tree
    if [[ -n "$dst" ]]; then put "$ROOT/$f" "$DST/$dst${f#"$src"}"; else put "$ROOT/$f" "$DST/$f"; fi
    n=$((n + 1))
  done < <(git -C "$ROOT" ls-files -co --exclude-standard -- "$src" "$@")
  ((n > 0)) || {
    finding "$(readme_step_line "$STEP_N")" "step $STEP_N: \`$src\` matches no file in the template"
    return 1
  }
}
# copy_tmpl <dir> <dst>: the skeleton's *.tmpl files under dir to dst, every .tmpl in the path stripped (a
# directory may carry one: testdata.tmpl/approved stays out of approved-check). An overlay's fixture.conf.tmpl
# is merged instead (merge_expect).
copy_tmpl() {
  local f
  while IFS= read -r f; do
    f="${f#"$1"/}"
    put "$1/$f" "$2/${f//.tmpl/}"
  done < <(find "$1" -type f -name '*.tmpl' ! -name fixture.conf.tmpl)
}
# hand_edit <file> <sed -E expression>...: one hand edit README (b) asks for, in the adopted repo. A finding when
# it changes nothing: the README no longer matches the file.
hand_edit() {
  local file="$1" e
  local args=()
  shift
  for e in "$@"; do args+=(-e "$e"); done
  cp "$DST/$file" "$tmp/before"
  sed -E -i.bak "${args[@]}" "$DST/$file"
  rm "$DST/$file.bak"
  cmp -s "$DST/$file" "$tmp/before" || return 0
  finding "$(readme_step_line "$STEP_N")" "step $STEP_N: hand edit $* matched nothing in $file"
  return 1
}

# --- what README (b) names ----------------------------------------------------------------------------------
# Step 3, as README writes the paths: each copied or merged by copy_step3...
step3_copy=(docs/harness/ docs/adr/ docs/exec-plans/ scripts/harness/ scripts/git-hooks/ scripts/test/
  scripts/install-tools.sh scripts/swift-format/Package.resolved scripts/setup-worktree.sh scripts/github-branch-protection.sh
  .github/CODEOWNERS .github/pull_request_template.md trivy.yaml .shellcheckrc .editorconfig .swiftlint.yml
  .swift-format ARCHITECTURE.md docs/design-docs/
  docs/QUALITY.md docs/SECURITY.md docs/RELIABILITY.md docs/generated/ docs/references/ AGENTS.md CLAUDE.md
  apps/api/internal/ .spectral.yaml apps/api/.golangci.yml apps/api/.custom-gcl.yml apps/api/.go-arch-lint.yml
  apps/api/tools/analyzers/ .claude/ .specify/ .gitignore)
# ...or not copied here: step 2 did (layout.env, Makefile), a destination (<API_DIR>/...), the db stack
# (compose.yaml, .squawk.toml), named twice (setup-worktree.sh), or the protected-paths.txt list instead
# (.github/, .github/workflows/).
step3_skip=(layout.env Makefile '<API_DIR>/internal/' '<API_DIR>/' '<API_DIR>/tools/analyzers/' compose.yaml
  .squawk.toml setup-worktree.sh .github/ .github/workflows/)
# Step 6's probe bullet: what the adopted repo drops of the probe (drop_probe), .harness-version last...
step6_probe=(scripts/templates/adoption-probe/ scripts/harness/adoption-probe.sh
  scripts/harness/fixtures/harness-adoption-stale-layout/ scripts/test/adoption-probe.bats docs/harness/rules.md
  .harness-version)
# ...and the synced harness files it leaves as they are: the CI job reads the decline line instead (ADR 0038).
step6_keep=(.github/workflows/ci.yml Makefile scripts/github-branch-protection.sh docs/harness/ci.md
  scripts/harness/budgets.txt scripts/harness/protected-paths.txt .github/CODEOWNERS)

# copy_step3 <path as README step 3 writes it>
copy_step3() {
  case "$1" in
    scripts/harness/) copy_to "$1" "" ':(exclude)scripts/harness/layout.env' ;;
    # the root pair, and the area pairs of the stack kept: api/ (apps/api/db/ goes with db, apps/web/ with web)
    AGENTS.md | CLAUDE.md) copy_to "$1" && copy_to "api/$1" ;;
    apps/api/internal/) copy_to "$1AGENTS.md" "${API_PREFIX}internal/AGENTS.md" &&
      copy_to "$1CLAUDE.md" "${API_PREFIX}internal/CLAUDE.md" ;;
    apps/api/*) copy_to "$1" "$API_PREFIX${1#apps/api/}" ;;
    # hook scripts, agents and skills; new-domain goes with make new-domain (step 5's table), add-migration with db
    .claude/) copy_to "$1" "" ':(exclude).claude/skills/new-domain/' ':(exclude).claude/skills/add-migration/' ;;
    *) copy_to "$1" ;;
  esac
}

# --- preflight: nothing heavy before the inputs are known to fit --------------------------------------------
[[ -d "$SKEL/repo" && -f "$SKEL/layout.env" && -f "$SKEL/drop.txt" ]] ||
  cannot "$SKEL has no repo/, layout.env or drop.txt"
SKEL="$(cd "$SKEL" && pwd -P)"
# The skeleton's layout through the validating parser. The adopted repo's make, git and tools get none of this
# checkout's make or harness variables, nor what CI's setup action exports from this checkout's layout.env and
# tool-versions.env.
out="$("$ROOT/scripts/harness/layout.sh" --print "$SKEL/layout.env" 2>&1)" || cannot "$out"
unset_env=(-u HARNESS_STACK -u HARNESS_SCOPE -u BASE_REF -u MAKEFLAGS -u MFLAGS -u MAKELEVEL)
while IFS='=' read -r key _; do unset_env+=(-u "$key"); done < <(
  "$ROOT/scripts/harness/layout.sh" --print
  grep -E '^[A-Z_]+=' "$ROOT/scripts/tool-versions.env"
)
# shellcheck source=scripts/harness/layout.sh
source "$ROOT/scripts/harness/layout.sh" || exit 2
harness_layout_load "$SKEL/layout.env" || exit 2 # API_DIR, API_PREFIX of the adopted repo

tmp="$(mktemp -d -t adoption-probe.XXXXXX)"
if [[ "${ADOPTION_PROBE_KEEP:-0}" == 1 ]]; then
  echo "adoption-probe: keeping $tmp" >&2
else
  trap 'rm -rf "$tmp"' EXIT
fi
DST="$tmp/repo"
LOG="$tmp/step.log"
STEP_N=3

readme_spans '^3\. \*\*Copy and merge the harness' '^   Merge these into' >"$tmp/step3"
readme_spans '^   Merge these into' '^4\. ' bold >>"$tmp/step3"
readme_spans "^   - The adoption probe is the template's own" '^   - |^[0-9]+\. ' >"$tmp/step6"
bad=0
check_list 3 "$tmp/step3" "${step3_copy[@]}" "${step3_skip[@]}" || bad=1
check_list 6 "$tmp/step6" "${step6_probe[@]}" "${step6_keep[@]}" || bad=1
# The harness's own workflows and action, as protected-paths.txt (which step 3 names) lists them.
github=()
while read -r glob kind _; do
  [[ "$kind" != config || "$glob" != .github/* ]] || github+=("$glob")
done < <(grep -Ev '^[[:space:]]*(#|$)' "$ROOT/scripts/harness/protected-paths.txt")
((${#github[@]} > 0)) || {
  finding "$(readme_step_line 3)" "step 3: scripts/harness/protected-paths.txt names no .github config to copy"
  bad=1
}
((bad == 0)) || exit 1
drop=()
while read -r p; do
  hit=''
  for t in "${step3_copy[@]}"; do
    [[ "$t" == apps/api/* || ! ("$p" == "$t" || ("$t" == */ && "$p" == "$t"*)) ]] || hit=1
  done
  [[ -n "$hit" && -n "$(git -C "$ROOT" ls-files -co --exclude-standard -- "$p")" ]] ||
    cannot "$SKEL/drop.txt names $p, which step 3 does not copy: remove the line"
  drop+=("$p")
done < <(grep -Ev '^[[:space:]]*(#|$)' "$SKEL/drop.txt")

# --- the brownfield repo ------------------------------------------------------------------------------------
make_repo() {
  copy_tmpl "$SKEL/repo" "$DST"
  g init -q -b main
  g add -A
  g commit -qm "feat: hello service"
}
mkdir -p "$DST"
prep "committing the brownfield repo" make_repo

# --- 1 branch -----------------------------------------------------------------------------------------------
step1() {
  g checkout -qb harness/adopt
  git -C "$ROOT" rev-parse HEAD >"$DST/.harness-version"
}
step 1 "branch and .harness-version" step1

# --- 2 the hooks' parser ------------------------------------------------------------------------------------
step2() {
  copy_to scripts/tool-versions.env
  copy_to scripts/harness/layout.env
  copy_to scripts/harness/layout.sh
  hand_edit scripts/harness/layout.env "s|^API_DIR=.*|API_DIR=$API_DIR|"
  copy_to apps/api/tools/harness/ "${API_PREFIX}tools/harness/"
  copy_to Makefile
}
step 2 "copy the parser and set API_DIR" step2
step 2 "make .bin/harness" in_dst make .bin/harness

# --- 3 copy and merge ---------------------------------------------------------------------------------------
step3() {
  local t
  for t in "${step3_copy[@]}"; do copy_step3 "$t"; done
  for t in "${github[@]}"; do copy_to "$t"; done
}
step 3 "copy and merge the harness" step3

# --- 4 rename -----------------------------------------------------------------------------------------------
module="$(sed -n 's/^module //p' "$SKEL/repo/go.mod.tmpl")"
rename() { in_dst make rename NAME=hello MODULE="$module" OWNER=@example "$@"; }
step 4 "git add the copied files" g add -A
step 4 "make rename (dry run)" rename
step 4 "make rename WRITE=1" rename WRITE=1
step 4 "commit" g commit -qm "chore(harness): adopt the harness"

# --- 5 layout and re-pointed files --------------------------------------------------------------------------
step 5 "write layout.env" cp "$SKEL/layout.env" "$DST/scripts/harness/layout.env"
step 5 "make layout-check" in_dst make layout-check
up="" # the repo root seen from API_DIR: one ../ per directory
IFS=/ read -ra parts <<<"${API_PREFIX%/}"
for _ in ${parts[@]+"${parts[@]}"}; do up="$up../"; done
step5() {
  hand_edit scripts/harness/protected-paths.txt "s|^apps/api/tools/|${API_PREFIX}tools/|" \
    '/^apps\/api\/internal\/platform\/approvaltest\//d' '/^apps\/web\//d'
  hand_edit .github/CODEOWNERS "s|^/apps/api/tools/|/${API_PREFIX}tools/|" \
    '/^\/apps\/api\/internal\/platform\/approvaltest\//d' '/^\/apps\/web\//d' '/^\/apps\/api\/db\//d'
  hand_edit "${API_PREFIX}.custom-gcl.yml" "s|^destination: .*|destination: ${up}.bin|"
  hand_edit "${API_PREFIX}internal/AGENTS.md" "s|\(\.\./\.\./\.\./docs/|(${up}../docs/|g"
  hand_edit docs/harness/rules.md "s|apps/api/tools/|${API_PREFIX}tools/|g"
}
step 5 "re-point the files that cannot read layout.env" step5

# --- 6 rules ------------------------------------------------------------------------------------------------
# STACK=go drops the web and db sets; their area links, the orval reference and the skills step 3 left out go
# by hand. make new-domain writes the template layout: its target, CI probe job and required check go too.
drop_sets() {
  hand_edit AGENTS.md 's|, \[apps/api/db/\]\(apps/api/db/AGENTS.md\)||' 's|, \[apps/web/\]\(apps/web/AGENTS.md\)||' \
    's|, \[orval\]\(docs/references/orval-llms.txt\)||' "s|\(apps/api/internal/AGENTS.md\)|(${API_PREFIX}internal/AGENTS.md)|"
  g rm -q docs/references/orval-llms.txt
  hand_edit .claude/agents/test-writer.md 's|\[apps/web/AGENTS\.md\]\(\.\./\.\./apps/web/AGENTS\.md\)|`apps/web/AGENTS.md`|'
  hand_edit AGENTS.md 's| \(\[\.claude/skills/add-migration\]\(\.claude/skills/add-migration/SKILL\.md\)\)||' \
    's|\[new-domain\]\(\.claude/skills/new-domain/SKILL\.md\), ||' \
    's|, \[add-migration\]\(\.claude/skills/add-migration/SKILL\.md\)||'
  hand_edit .specify/memory/constitution.md 's| \(\[add-migration\]\(\.\./\.\./\.claude/skills/add-migration/SKILL\.md\)\)||'
  hand_edit Makefile '/^new-domain:/,/^$/d' '/^\.PHONY:/s/ new-domain( |$)/\1/'
  hand_edit .github/workflows/ci.yml '/^  generator-probe:$/,/^$/d'
  hand_edit scripts/github-branch-protection.sh 's/ generator-probe([ )])/\1/' 's/\(generator-probe /(/'
  hand_edit docs/harness/ci.md '/`generator-probe`/d' 's/the eight required checks/the seven required checks/'
}
step 6 "drop the web and db sets and make new-domain" drop_sets
# The skeleton's drop.txt: what this layout drops by path. A dropped test leaves its rule's Fixture line.
drop_paths() {
  local p
  for p in ${drop[@]+"${drop[@]}"}; do
    g rm -rq "$p"
    if [[ "$p" == scripts/test/*.bats ]] && grep -qF "\`$p\`" "$DST/docs/harness/rules.md"; then
      hand_edit docs/harness/rules.md "s|, \`$p\`||g" "s|; \`$p\`, |; |g" "s|; \`$p\` ·| ·|g"
    fi
  done
}
step 6 "drop drop.txt's paths" drop_paths
# Port the kept fixtures: the template's apps/api/ becomes API_DIR in overlays, scripts, EXPECT patterns and the
# depguard import path. Fixtures that edit the template's own product code get the skeleton's fixtures/ copy:
# its files replace the template's, its fixture.conf.tmpl replaces only the EXPECT line (RULE, TARGET and the
# rest stay the template's).
merge_expect() {
  local conf name orig
  for conf in "$SKEL"/fixtures/*/fixture.conf.tmpl; do
    [[ -e "$conf" ]] || continue
    name="$(basename "$(dirname "$conf")")"
    orig="$DST/scripts/harness/fixtures/$name/fixture.conf"
    [[ -f "$orig" ]] || cannot "$conf: the template has no fixture $name"
    grep -q '^EXPECT=' "$conf" || cannot "$conf: no EXPECT line"
    {
      grep -v '^EXPECT=' "$orig" || true
      grep '^EXPECT=' "$conf"
    } >"$tmp/conf"
    mv "$tmp/conf" "$orig"
  done
}
port_fixtures() {
  local d sub c f
  if [[ "$API_DIR" != apps/api ]]; then
    for d in "$DST"/scripts/harness/fixtures/*/; do
      d="${d#"$DST/"}"
      for sub in files base; do
        [[ -d "$DST/$d$sub/apps/api" ]] || continue
        mkdir -p "$DST/$d$sub/$API_PREFIX"
        for c in "$DST/$d$sub/apps/api"/*; do g mv "$d$sub/apps/api/${c##*/}" "$d$sub/$API_PREFIX"; done
      done
    done
    while IFS= read -r f; do
      hand_edit "${f#"$DST/"}" "s|apps/api/|${API_PREFIX}|g"
    done < <(grep -rlF apps/api/ "$DST/scripts/harness/fixtures" "$DST/${API_PREFIX}.golangci.yml")
  fi
  [[ -d "$SKEL/fixtures" ]] || return 0
  for d in "$SKEL"/fixtures/*/; do
    [[ -d "$DST/scripts/harness/fixtures/$(basename "$d")" ]] || cannot "$d: the template has no such fixture"
  done
  copy_tmpl "$SKEL/fixtures" "$DST/scripts/harness/fixtures"
  merge_expect
}
step 6 "port the fixtures" port_fixtures
# The adoption probe's own files and rule go, and they are declined for template sync; step6_keep stays as is.
drop_probe() {
  local p only=()
  for p in "${step6_probe[@]}"; do
    case "$p" in
      scripts/templates/adoption-probe/) only+=("$p") ;; # step 3 never copied it
      scripts/harness/adoption-probe.sh | scripts/harness/fixtures/* | scripts/test/adoption-probe.bats)
        g rm -rq "$p"
        only+=("$p")
        ;;
      docs/harness/rules.md) hand_edit "$p" '/^### harness-adoption-probe$/,/^$/d' ;;
      .harness-version)
        git -C "$ROOT" ls-files -co --exclude-standard -- "${only[@]}" |
          sed 's/$/: template-only/; s/^/# declined /' >>"$DST/$p"
        ;;
      *) cannot "drop_probe: no case for $p" ;;
    esac
  done
}
step 6 "drop the adoption probe" drop_probe

# --- 7 setup and check --------------------------------------------------------------------------------------
seed_bin() {
  local stamp tool f
  mkdir -p "$DST/.bin"
  for stamp in "$ROOT"/.bin/.*.version; do
    [[ -e "$stamp" ]] || continue
    tool="$(basename "$stamp" .version)"
    tool="${tool#.}"
    cp "$stamp" "$DST/.bin/"
    for f in ".$tool" "$tool"; do # .bats and .pnpm are the installs bats and pnpm link into
      [[ ! -e "$ROOT/.bin/$f" ]] || ln -s "$ROOT/.bin/$f" "$DST/.bin/$f"
    done
  done
}
prep "linking this checkout's pinned tools" seed_bin
step 7 "make setup" in_dst make setup
step 7 "make check" in_dst make check
step 7 "make harness-selftest" in_dst make harness-selftest
