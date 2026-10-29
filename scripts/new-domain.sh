#!/usr/bin/env bash
# Scaffolds a backend domain wired through every layer, then regenerates code.
#
#   scripts/new-domain.sh <name>        (make new-domain NAME=<name>)
#
# Creates apps/api/internal/<name>/{model,store,service,api} (+ AGENTS.md, CLAUDE.md), db/queries/<name>.sql, an empty
# migration, an OpenAPI tag + stub operation (GET /<kebab>/status) + schema, a sqlc.yaml entry, the route
# registration in cmd/api/routes.go and a docs/QUALITY.md row; then runs `make gen fmt gen-schema-doc`.
# Each domain starts with three passing tests (NFR-B1): service (fake store), handler (strict server +
# httptest) and integration (-tags=integration, real Postgres). Every generated file and insertion is a
# template in scripts/templates/new-domain/. Insertions go before `harness:domain-*` / `harness:quality-rows`
# markers. Undo before committing (restores only what it wrote; printed with the real paths at the end):
#   git checkout -- apps/api/cmd/api/routes.go api/openapi.yaml apps/api/sqlc.yaml docs/QUALITY.md \
#     apps/api/db/migrations/atlas.sum docs/generated packages/api-client/src/gen
#   git clean -fd -- apps/api/internal/<name> apps/api/db/queries/<name>.sql \
#     'apps/api/db/migrations/*_<name>.sql' packages/api-client/src/gen
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
# shellcheck source=tool-versions.env
source scripts/tool-versions.env
export GOTOOLCHAIN="$GO_TOOLCHAIN" PATH="$ROOT/.bin:$ROOT/node_modules/.bin:$PATH"

die() {
  echo "new-domain: $*" >&2
  exit 1
}

name="${1:-}"
[[ "$name" =~ ^[a-z][a-z0-9_]*[a-z0-9]$ ]] || die "NAME must match ^[a-z][a-z0-9_]*[a-z0-9]\$ (got '${name}'), e.g. make new-domain NAME=billing"
[[ "$name" != platform ]] || die "'platform' is reserved for plumbing"
[[ ! -e "apps/api/internal/$name" ]] || die "apps/api/internal/$name already exists"
grep -q "^  - name: $name$" api/openapi.yaml && die "tag '$name' already exists in api/openapi.yaml"

MOD="$(cd apps/api && go list -m)"
kebab="${name//_/-}"
pascal="$(awk -F_ '{ for (i = 1; i <= NF; i++) printf "%s%s", toupper(substr($i, 1, 1)), substr($i, 2) }' <<<"$name")"
camel="$(tr '[:upper:]' '[:lower:]' <<<"${pascal:0:1}")${pascal:1}"

ROUTES=apps/api/cmd/api/routes.go SPEC=api/openapi.yaml SQLC=apps/api/sqlc.yaml QUALITY=docs/QUALITY.md
dirty="$(git status --porcelain -- "$ROUTES" "$SPEC" "$SQLC" "$QUALITY")"
[[ -z "$dirty" || "${FORCE:-0}" == 1 ]] || die "uncommitted changes in files it edits (FORCE=1 to proceed):
$dirty"
for f in "$ROUTES" "$SPEC" "$QUALITY"; do grep -q 'harness:' "$f" || die "generator marker missing in $f"; done

TEMPLATES=scripts/templates/new-domain PLACEHOLDER='__[A-Z][A-Z0-9_]*__'
[[ -d "$TEMPLATES" ]] || die "$TEMPLATES missing"

# render <template>: the template on stdout with __NAME__, __KEBAB__, __PASCAL__, __CAMEL__ and __MOD__ filled
# in. Fails on a missing template or a placeholder it does not know (fail closed).
render() {
  local t="$TEMPLATES/$1" out
  [[ -f "$t" ]] || die "template $t missing"
  out="$(sed -e "s|__NAME__|$name|g" -e "s|__KEBAB__|$kebab|g" -e "s|__PASCAL__|$pascal|g" \
    -e "s|__CAMEL__|$camel|g" -e "s|__MOD__|$MOD|g" "$t")" || die "cannot render $t"
  ! grep -qE "$PLACEHOLDER" <<<"$out" || die "unknown placeholder in $t: $(grep -oE "$PLACEHOLDER" <<<"$out" | head -1)"
  printf '%s\n' "$out"
}

# insert <file> <marker-regex> <template>: the rendered template goes on the lines before the first line
# matching the marker.
insert() {
  local block tmp
  block="$(render "$3")"
  tmp="$(mktemp)"
  BLOCK="$block" awk -v marker="$2" '!done && $0 ~ marker { print ENVIRON["BLOCK"]; done = 1 } { print } END { exit !done }' "$1" >"$tmp" ||
    {
      rm -f "$tmp"
      die "marker /$2/ not found in $1"
    }
  mv "$tmp" "$1"
}

# Render every template once before writing anything, so a broken one leaves the repo untouched.
for t in "$TEMPLATES"/*.tmpl; do render "${t##*/}" >/dev/null; done

D="apps/api/internal/$name"
mkdir -p "$D/model" "$D/service" "$D/api" "$D/store" apps/api/db/queries

# --- Go layers and their tests (NFR-B1) ---------------------------------------------------------------
render AGENTS.md.tmpl >"$D/AGENTS.md"
echo '@AGENTS.md' >"$D/CLAUDE.md" # rules.md#docs-claude-imports-agents
render model.go.tmpl >"$D/model/model.go"
render queries.sql.tmpl >apps/api/db/queries/"$name".sql
render service.go.tmpl >"$D/service/service.go"
render service_test.go.tmpl >"$D/service/service_test.go"
render oapi-codegen.yaml.tmpl >"$D/api/oapi-codegen.yaml"
render api.go.tmpl >"$D/api/api.go"
render api_test.go.tmpl >"$D/api/api_test.go"
render api_integration_test.go.tmpl >"$D/api/api_integration_test.go"

# --- wiring -------------------------------------------------------------------------------------------
insert "$ROUTES" '^[)]$' routes-imports.go.tmpl
insert "$ROUTES" 'harness:domain-routes' routes-register.go.tmpl
insert "$SPEC" 'harness:domain-tags' openapi-tag.yaml.tmpl
insert "$SPEC" 'harness:domain-paths' openapi-path.yaml.tmpl
insert "$SPEC" 'harness:domain-schemas' openapi-schema.yaml.tmpl

if grep -q '^sql: \[\]' "$SQLC"; then
  tmp="$(mktemp)"
  sed 's/^sql: \[\]$/sql:/' "$SQLC" >"$tmp" && mv "$tmp" "$SQLC"
fi
render sqlc.yaml.tmpl >>"$SQLC"

insert "$QUALITY" 'harness:quality-rows' quality-row.md.tmpl

# --- migration ----------------------------------------------------------------------------------------
(cd apps/api && atlas migrate new "$name" --env local)
migration="$(find apps/api/db/migrations -name "*_${name}.sql" | sort | tail -1)"
render migration.sql.tmpl >"$migration"

# --- regenerate ---------------------------------------------------------------------------------------
make --no-print-directory gen fmt
if command -v docker >/dev/null && docker info >/dev/null 2>&1; then
  make --no-print-directory gen-schema-doc
else
  echo "new-domain: Docker not running; run \`make gen-schema-doc\` before \`make check\`." >&2
fi

cat <<EOF
new-domain: created domain '$name' (tag $name, GET /$kebab/status, hook useGet${pascal}Status).
Next: model the domain in api/openapi.yaml and $migration, then \`make gen\`, implement, \`make check\`.
Undo (before committing): git checkout -- $ROUTES $SPEC $SQLC $QUALITY apps/api/db/migrations/atlas.sum docs/generated packages/api-client/src/gen && git clean -fd -- $D apps/api/db/queries/$name.sql $migration packages/api-client/src/gen
EOF
