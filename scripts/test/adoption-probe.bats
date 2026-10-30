#!/usr/bin/env bats
# scripts/harness/adoption-probe.sh (make adoption-probe), its cheap paths only: what it cannot run on (exit 2)
# and the findings before any make (exit 1). A full run takes minutes: CI's adoption-probe job and the
# harness-adoption-stale-layout fixture cover it.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/adoption-probe.sh README.md scripts/harness/protected-paths.txt \
    scripts/tool-versions.env scripts/test/helpers.bash
  sandbox_base
  SKEL="$BATS_TEST_TMPDIR/skel"
  cp -R "$REPO/scripts/templates/adoption-probe" "$SKEL"
  printf 'scripts/test/helpers.bash\n' >"$SKEL/drop.txt"
}

probe() { run "$@" "$SANDBOX/scripts/harness/adoption-probe.sh" "$SKEL"; }

# Every finding line has the rule's format.
findings_well_formed() {
  local line n=0
  while IFS= read -r line; do
    [[ "$line" == *"[harness-adoption-probe]" ]] || continue
    [[ "$line" =~ ^README\.md:[0-9]+:\ step\ [0-9]+[\ :].*\ \[harness-adoption-probe\]$ ]] || {
      echo "malformed finding: $line" >&2
      return 1
    }
    n=$((n + 1))
  done <<<"$output"
  ((n > 0)) || {
    echo "no finding in: $output" >&2
    return 1
  }
}

@test "cannot run without the skeleton's repo/" {
  rm -r "$SKEL/repo"
  probe -2
  has "has no repo/, layout.env or drop.txt"
}

@test "cannot run on a skeleton layout.env without API_DIR" {
  sed -i.bak '/^API_DIR=/d' "$SKEL/layout.env"
  probe -2
  has "missing API_DIR [harness-layout-valid]"
}

@test "cannot run when drop.txt names a path step 3 does not copy" {
  printf 'scripts/test/nope.bats\n' >"$SKEL/drop.txt"
  probe -2
  has "drop.txt names scripts/test/nope.bats, which step 3 does not copy"
}

@test "a README step-3 list that differs from the probe's is a finding, both ways, before any step runs" {
  sed -i.bak 's/`trivy.yaml`, `.shellcheckrc` and/`.shellcheckrc`, `.nvmrc` and/' "$SANDBOX/README.md"
  probe -1
  has 'step 3 names `.nvmrc`, which the adoption probe neither handles nor skips [harness-adoption-probe]'
  has 'step 3 no longer names `trivy.yaml`, which the adoption probe still handles [harness-adoption-probe]'
  findings_well_formed
  [[ "$output" != *"failed on"* ]]
}

@test "a README step-6 probe bullet that drops a path the probe does not is a finding" {
  sed -i.bak 's/its budget in `scripts\/harness\/budgets.txt`/its budget in `scripts\/harness\/budgets.yaml`/' \
    "$SANDBOX/README.md"
  probe -1
  has 'step 6 names `scripts/harness/budgets.yaml`'
  has 'step 6 no longer names `scripts/harness/budgets.txt`'
  findings_well_formed
}

@test "a hand edit that changes nothing is README drift at its step" {
  # The template's layout.env already says API_DIR=apps/api: step 2's edit matches nothing.
  sed -i.bak 's|^API_DIR=.*|API_DIR=apps/api|' "$SKEL/layout.env"
  probe -1
  has "step 2: hand edit s|^API_DIR=.*|API_DIR=apps/api| matched nothing in scripts/harness/layout.env [harness-adoption-probe]"
  has 'step 2 (copy the parser and set API_DIR) failed'
  findings_well_formed
}
