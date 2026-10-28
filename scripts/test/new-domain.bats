#!/usr/bin/env bats
# scripts/new-domain.sh: name and template validation happen before any file is touched, and every template
# renders. Runs on a sandbox copy, so a regression cannot scaffold into the real repo. The full generator run
# is covered by the CI probe (make new-domain NAME=zzz_probe && make check && make test-integration).
bats_require_minimum_version 1.5.0
load helpers

setup() {
  sandbox scripts/new-domain.sh scripts/tool-versions.env
}

@test "rejects invalid and reserved names without changing anything" {
  sandbox_base
  for n in '' Billing 1x x a- 'a b' 'a/b'; do
    run -1 "$SANDBOX/scripts/new-domain.sh" "$n"
    has 'NAME must match'
  done
  run -1 "$SANDBOX/scripts/new-domain.sh" platform
  has 'reserved'
  [ -z "$(git -C "$SANDBOX" status --porcelain)" ]
}

@test "fails closed on a template placeholder it does not know, before writing anything" {
  local templates
  templates="$(cd "$REPO" && echo scripts/templates/new-domain/*.tmpl)"
  # shellcheck disable=SC2086 # reason: one word per template path, none has a space (rule: sh-shellcheck)
  sandbox apps/api/go.mod api/openapi.yaml apps/api/cmd/api/routes.go apps/api/sqlc.yaml docs/QUALITY.md \
    $templates # copies: templates are not executable, so not hardlinked
  # The last template the generator uses: by then a check per write has wired routes, spec and sqlc.
  echo '| __OWNER_2__ |' >>"$SANDBOX/scripts/templates/new-domain/quality-row.md.tmpl"
  sandbox_base
  run -1 "$SANDBOX/scripts/new-domain.sh" billing
  has 'unknown placeholder in scripts/templates/new-domain/quality-row.md.tmpl: __OWNER_2__'
  [ -z "$(git -C "$SANDBOX" status --porcelain --untracked-files=all)" ]
  [ ! -e "$SANDBOX/apps/api/internal/billing" ]
}

# Rendered with the generator's placeholders (keep in step with render() in new-domain.sh): every template
# is used, fills in completely, and its Go parses and is gofmt-clean. The CI generator probe covers the rest.
@test "every template is used, fills in completely and renders valid Go" {
  local t f gofmt script="$REPO/scripts/new-domain.sh" dir="$REPO/scripts/templates/new-domain"
  local rendered="$BATS_TEST_TMPDIR/rendered"
  gofmt="$(go env GOROOT)/bin/gofmt"
  mkdir -p "$rendered"
  while read -r f; do
    [ -f "$dir/$f" ] || {
      echo "new-domain.sh uses missing template $f" >&2
      return 1
    }
  done < <(grep -oE '[A-Za-z0-9_.-]+\.tmpl' "$script" | sort -u)
  for t in "$dir"/*.tmpl; do
    f="${t##*/}"
    grep -qF "$f" "$script" || {
      echo "template $f is not used by new-domain.sh" >&2
      return 1
    }
    sed -e 's|__NAME__|billing_run|g' -e 's|__KEBAB__|billing-run|g' -e 's|__PASCAL__|BillingRun|g' \
      -e 's|__CAMEL__|billingRun|g' -e 's|__MOD__|example.com/app|g' "$t" >"$rendered/${f%.tmpl}"
    ! grep -nE '__[A-Z][A-Z0-9_]*__' "$rendered/${f%.tmpl}" || {
      echo "unfilled placeholder in $f" >&2
      return 1
    }
    # Whole Go files only; the routes.go fragments (insert) are parsed when the generator runs make fmt.
    grep -qE "^render $f >" "$script" || rm -f "$rendered/${f%.tmpl}"
  done
  ls "$rendered"/*.go >/dev/null
  run "$gofmt" -e -l "$rendered"
  [ "$status" -eq 0 ] && [ -z "$output" ] || {
    echo "rendered Go does not parse or is not gofmt-clean: $output" >&2
    return 1
  }
}
