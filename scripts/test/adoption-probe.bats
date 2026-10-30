#!/usr/bin/env bats
# scripts/harness/adoption-probe.sh (make adoption-probe), its cheap paths only: the product skip (exit 0), what
# it cannot run on (exit 2) and the findings before any make (exit 1). A full run takes minutes: CI's
# adoption-probe job and the harness-adoption-stale-layout fixture cover it.
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/harness/adoption-probe.sh README.md scripts/harness/protected-paths.txt \
    scripts/tool-versions.env scripts/test/helpers.bash
  # The sandbox is always the template, whatever this repo is: each case that needs a product sets the value.
  edit scripts/tool-versions.env 's|^HARNESS_UPSTREAM=.*|HARNESS_UPSTREAM=|'
  sandbox_base
  SKEL="$BATS_TEST_TMPDIR/skel"
  cp -R "$REPO/scripts/templates/adoption-probe" "$SKEL"
  printf 'scripts/test/helpers.bash\n' >"$SKEL/drop.txt"
}

probe() { run "$@" "$SANDBOX/scripts/harness/adoption-probe.sh" "$SKEL"; }

# template_only: skips a case that reads README (b), which a product's README need not keep (ADR 0038), as the
# probe itself and the harness-adoption-stale-layout fixture do.
template_only() {
  [[ -z "$(sed -n 's/^HARNESS_UPSTREAM=//p' "$REPO/scripts/tool-versions.env" | tail -n 1)" ]] ||
    skip "template-only: HARNESS_UPSTREAM is set, this repo is a product (ADR 0038)"
}

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

@test "in a product (HARNESS_UPSTREAM set in tool-versions.env) it passes with a notice before reading anything" {
  rm -r "$SKEL/repo"
  sed -i.bak '/^## (b) Brownfield project/,$d' "$SANDBOX/README.md"
  edit scripts/tool-versions.env 's|^HARNESS_UPSTREAM=.*|HARNESS_UPSTREAM=https://example.com/template|'
  probe -0
  has "HARNESS_UPSTREAM is set in scripts/tool-versions.env: this repo is a product and the probe is template-only"
  [[ "$output" != *"[harness-adoption-probe]"* ]]
}

@test "the template (HARNESS_UPSTREAM empty in the file) runs the probe, whatever the environment says" {
  template_only
  sed -i.bak '/^## (b) Brownfield project/,$d' "$SANDBOX/README.md"
  probe -1 env HARNESS_UPSTREAM=https://example.com/template
  has 'step 3 no longer names `docs/harness/`, which the adoption probe still handles [harness-adoption-probe]'
  findings_well_formed
}

@test "cannot run without scripts/tool-versions.env" {
  rm "$SANDBOX/scripts/tool-versions.env"
  probe -2
  has "scripts/tool-versions.env is missing"
}

@test "cannot run without the skeleton's repo/" {
  template_only
  rm -r "$SKEL/repo"
  probe -2
  has "has no repo/, layout.env or drop.txt"
}

@test "cannot run on a skeleton layout.env without API_DIR" {
  template_only
  sed -i.bak '/^API_DIR=/d' "$SKEL/layout.env"
  probe -2
  has "missing API_DIR [harness-layout-valid]"
}

@test "cannot run when drop.txt names a path step 3 does not copy" {
  template_only
  printf 'scripts/test/nope.bats\n' >"$SKEL/drop.txt"
  probe -2
  has "drop.txt names scripts/test/nope.bats, which step 3 does not copy"
}

@test "a README step-3 list that differs from the probe's is a finding, both ways, before any step runs" {
  template_only
  sed -i.bak 's/`trivy.yaml`, `.shellcheckrc` and/`.shellcheckrc`, `.nvmrc` and/' "$SANDBOX/README.md"
  probe -1
  has 'step 3 names `.nvmrc`, which the adoption probe neither handles nor skips [harness-adoption-probe]'
  has 'step 3 no longer names `trivy.yaml`, which the adoption probe still handles [harness-adoption-probe]'
  findings_well_formed
  [[ "$output" != *"failed on"* ]]
}

@test "a README step-6 probe bullet that drops a path the probe does not is a finding" {
  template_only
  sed -i.bak 's/its budget in `scripts\/harness\/budgets.txt`/its budget in `scripts\/harness\/budgets.yaml`/' \
    "$SANDBOX/README.md"
  probe -1
  has 'step 6 names `scripts/harness/budgets.yaml`'
  has 'step 6 no longer names `scripts/harness/budgets.txt`'
  findings_well_formed
}

@test "a hand edit that changes nothing is README drift at its step" {
  template_only
  # The template's layout.env already says API_DIR=apps/api: step 2's edit matches nothing.
  sed -i.bak 's|^API_DIR=.*|API_DIR=apps/api|' "$SKEL/layout.env"
  probe -1
  has "step 2: hand edit s|^API_DIR=.*|API_DIR=apps/api| matched nothing in scripts/harness/layout.env [harness-adoption-probe]"
  has 'step 2 (copy the parser and set API_DIR) failed'
  findings_well_formed
}
