#!/usr/bin/env bats
# scripts/harness/check-specs.sh: spec-kit specs are well formed; feat changes to apps/ or api/ come with one.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/check-specs.sh scripts/harness/merge-base.sh scripts/harness/protected-paths.txt
  mkdir -p "$SANDBOX/apps/web" "$SANDBOX/docs"
  printf '%s\n' x | tee "$SANDBOX/apps/web/x" >"$SANDBOX/docs/x"
  sandbox_base
  git -C "$SANDBOX" checkout -qb work
}

check() { BASE_REF=main run "$@" "$SANDBOX/scripts/harness/check-specs.sh"; }
add() { mkdir -p "$(dirname "$SANDBOX/$1")" && printf '%s\n' "${@:2}" >>"$SANDBOX/$1"; }
commit() { # subject
  git -C "$SANDBOX" add -A
  git -C "$SANDBOX" -c user.name=test -c user.email=test@localhost -c commit.gpgsign=false commit -qm "$1"
}

@test "passes without specs" {
  check -0
}

@test "passes a complete spec" {
  add specs/001-user-signup/spec.md '# Spec'
  add specs/001-user-signup/plan.md '# Plan'
  add specs/001-user-signup/tasks.md '# Tasks'
  check -0
}

@test "flags a bad name, a duplicate number, a missing spec.md and tasks.md without plan.md" {
  add specs/Signup/spec.md '# Spec'
  add specs/001-a/spec.md '# Spec'
  add specs/001-b/tasks.md '# Tasks'
  check -1
  has 'specs/Signup: name is not NNN-kebab-slug'
  has 'specs/001-b: number 001 is used by another spec'
  has 'specs/001-b: no spec.md'
  has 'specs/001-b: tasks.md without plan.md'
  has '[specs-structure]'
}

@test "flags an open NEEDS CLARIFICATION with file and line" {
  add specs/002-x/spec.md '# Spec' '- FR-1: [NEEDS CLARIFICATION: which auth?]'
  add specs/002-x/plan.md '# Plan' '**Testing**: NEEDS CLARIFICATION'
  check -1
  has 'specs/002-x/spec.md:2: open NEEDS CLARIFICATION'
  has 'specs/002-x/plan.md:2: open NEEDS CLARIFICATION'
}

@test "the /speckit-specify checklist and a resolved research note are not open clarifications" {
  add specs/002-x/spec.md '# Spec'
  add specs/002-x/checklists/requirements.md '- [x] No [NEEDS CLARIFICATION] markers remain'
  add specs/002-x/research.md 'Resolves every NEEDS CLARIFICATION in plan.md.'
  check -0
}

@test "a feat commit touching apps/ needs a spec" {
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  check -1
  has '[specs-required-for-features]'
}

@test "product code is layout.env's PRODUCT_GLOBS" {
  sed -i.bak 's|^PRODUCT_GLOBS=.*|PRODUCT_GLOBS=services/*|' "$SANDBOX/scripts/harness/layout.env"
  rm "$SANDBOX/scripts/harness/layout.env.bak"
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  check -0
  add services/core/y 'y'
  commit 'feat(core): add thing'
  check -1
  has 'touches product code (services/*)'
}

@test "a feat commit touching apps/ passes with a spec, or with NO_SPEC_NEEDED=1" {
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  NO_SPEC_NEEDED=1 check -0
  add specs/003-banner/spec.md '# Spec'
  check -1 # CI sees only committed files
  has 'commit it'
  commit 'docs(specs): banner spec'
  check -0
}

@test "a feat touching api/ needs a spec; deleting a spec does not count" {
  add specs/001-old/spec.md '# Spec'
  commit 'docs(specs): old'
  git -C "$SANDBOX" branch -f main
  add api/openapi.yaml 'openapi: 3.1.0'
  git -C "$SANDBOX" rm -rq specs/001-old
  commit 'feat(api): add orders'
  check -1
  has '[specs-required-for-features]'
}

@test "reads the tree in HARNESS_ROOT, not the script's own checkout" {
  add apps/web/y 'y'
  commit 'feat(web): add banner'
  mkdir -p "$BATS_TEST_TMPDIR/base"
  cp -R "$SANDBOX/scripts" "$BATS_TEST_TMPDIR/base/"
  HARNESS_ROOT="$SANDBOX" BASE_REF=main run -1 "$BATS_TEST_TMPDIR/base/scripts/harness/check-specs.sh"
  has '[specs-required-for-features]'
}

@test "a feat PR title counts as a feat" {
  add apps/web/y 'y'
  commit 'fix(web): banner'
  check -0
  PR_TITLE='feat(web): banner' check -1
}

@test "feat outside apps/ and api/, and fix inside, need no spec" {
  add docs/y 'y'
  commit 'feat(docs): new guide'
  add apps/web/y 'y'
  commit 'fix(web): typo'
  check -0
}

@test "flags a symlinked spec dir and an OpenAPI document in a spec" {
  ln -s / "$SANDBOX/specs-link" && mkdir -p "$SANDBOX/specs" && mv "$SANDBOX/specs-link" "$SANDBOX/specs/004-x"
  add specs/005-y/spec.md '# Spec'
  add specs/005-y/contracts/api.yaml 'openapi: 3.1.0'
  add specs/005-y/contracts/notes.yaml 'operations: [listOrders]'
  check -1
  has 'specs/004-x: is a symlink'
  has 'specs/005-y/contracts/api.yaml: is an OpenAPI document'
  [[ "$output" != *notes.yaml* ]]
}

@test "a feat touching only harness files under apps/ needs no spec" {
  add apps/api/.golangci.yml 'linters: {}'
  add apps/api/tools/analyzers/x.go 'package x'
  commit 'feat(analyzers): add a linter'
  check -0
  add apps/web/y 'y'
  commit 'feat(web): banner'
  check -1
}

@test "exits 2 without a base" {
  BASE_REF=nope run -2 "$SANDBOX/scripts/harness/check-specs.sh"
}
