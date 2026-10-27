#!/usr/bin/env bash
# Scaffolds a backend domain wired through every layer, then regenerates code.
#
#   scripts/new-domain.sh <name>        (make new-domain NAME=<name>)
#
# Creates apps/api/internal/<name>/{model,store,service,api} (+ AGENTS.md, CLAUDE.md), db/queries/<name>.sql, an empty
# migration, an OpenAPI tag + stub operation (GET /<kebab>/status) + schema, a sqlc.yaml entry, the route
# registration in cmd/api/routes.go and a docs/QUALITY.md row; then runs `make gen fmt gen-schema-doc`.
# Insertions go before `harness:domain-*` / `harness:quality-rows` markers. Undo before committing:
#   git checkout -- . && git clean -fd -- apps/api api docs packages/api-client
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

# insert <file> <marker-regex>: stdin goes on the lines before the first line matching the marker.
insert() {
  local tmp
  tmp="$(mktemp)"
  BLOCK="$(cat)" awk -v marker="$2" '!done && $0 ~ marker { print ENVIRON["BLOCK"]; done = 1 } { print } END { exit !done }' "$1" >"$tmp" ||
    {
      rm -f "$tmp"
      die "marker /$2/ not found in $1"
    }
  mv "$tmp" "$1"
}

D="apps/api/internal/$name"
mkdir -p "$D/model" "$D/service" "$D/api" "$D/store" apps/api/db/queries

# --- Go layers ----------------------------------------------------------------------------------------
cat >"$D/AGENTS.md" <<EOF
# Domain: $name

- Layers: \`model\` (types) → \`store\` (sqlc, generated) → \`service\` (logic) → \`api\` (generated strict server + handlers).
- Contract first: change the \`$name\` operations in \`api/openapi.yaml\`, run \`make gen\`, then implement in \`api/api.go\`.
- SQL lives in \`apps/api/db/queries/$name.sql\`; schema changes are new migrations (\`.claude/skills/add-migration\`).
- Other domains call \`service\` (or use \`model\`), never \`store\` or \`api\` (rules.md#go-domain-isolation).
- The generated \`GET /$kebab/status\` stub only proves the wiring: replace it with real operations.
- Done = \`make check\` green and the \`$name\` row in docs/QUALITY.md updated.
EOF
echo '@AGENTS.md' >"$D/CLAUDE.md" # rules.md#docs-claude-imports-agents

cat >"$D/model/model.go" <<EOF
// Package model holds the $name domain's types. It imports nothing from other layers.
package model
EOF

cat >apps/api/db/queries/"$name".sql <<EOF
-- Queries for the $name domain; \`make gen\` turns each into a method on store.Queries.
-- Ping is a generated stub that keeps the layers wired; replace it with real queries.

-- name: Ping :one
SELECT 1::int AS ok;
EOF

cat >"$D/service/service.go" <<EOF
// Package service holds the $name domain's business logic. It may import model and store, never api.
package service

import (
	"context"
	"fmt"

	"$MOD/internal/$name/store"
)

// Service is the $name domain's use-case API, called by its api package and by other domains.
type Service struct{ q *store.Queries }

// New returns a Service backed by the domain's generated queries.
func New(q *store.Queries) *Service { return &Service{q: q} }

// Ready reports whether the domain's storage answers. Generated stub: replace with real use cases.
func (s *Service) Ready(ctx context.Context) error {
	if _, err := s.q.Ping(ctx); err != nil {
		return fmt.Errorf("$name: ping: %w", err)
	}
	return nil
}
EOF

cat >"$D/api/oapi-codegen.yaml" <<EOF
# Generated strict server for OpenAPI tag "$name". Run via \`make gen\`.
package: api
output: api.gen.go
generate:
  std-http-server: true
  strict-server: true
  models: true
output-options:
  include-tags: [$name]
EOF

cat >"$D/api/api.go" <<EOF
// Package api serves the $name domain's HTTP operations (OpenAPI tag "$name").
// api.gen.go is generated from api/openapi.yaml by \`make gen\`; this file implements it.
package api

import (
	"context"
	"net/http"

	"$MOD/internal/$name/service"
	"$MOD/internal/platform/problem"
)

// Server implements StrictServerInterface by delegating to the service.
type Server struct{ svc *service.Service }

var _ StrictServerInterface = (*Server)(nil)

// Register mounts the generated routes for this tag on mux.
func Register(mux *http.ServeMux, svc *service.Service) {
	HandlerFromMux(NewStrictHandlerWithOptions(&Server{svc: svc}, nil, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  problem.RequestError,
		ResponseErrorHandlerFunc: problem.ResponseError,
	}), mux)
}

// Get${pascal}Status is the generated stub operation. Replace it with the domain's real operations.
func (s *Server) Get${pascal}Status(ctx context.Context, _ Get${pascal}StatusRequestObject) (Get${pascal}StatusResponseObject, error) {
	if err := s.svc.Ready(ctx); err != nil {
		return Get${pascal}Status503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse{
			Type: "about:blank", Title: "Service Unavailable", Status: http.StatusServiceUnavailable,
		}}, nil
	}
	return Get${pascal}Status200JSONResponse{Status: Ok}, nil
}
EOF

# --- wiring -------------------------------------------------------------------------------------------
insert "$ROUTES" '^[)]$' <<EOF
	${camel}API "$MOD/internal/$name/api"
	${camel}Service "$MOD/internal/$name/service"
	${camel}Store "$MOD/internal/$name/store"
EOF
insert "$ROUTES" 'harness:domain-routes' <<EOF
	${camel}API.Register(mux, ${camel}Service.New(${camel}Store.New(pool)))
EOF

insert "$SPEC" 'harness:domain-tags' <<EOF
  - name: $name
    description: The $name domain.
EOF
insert "$SPEC" 'harness:domain-paths' <<EOF
  /$kebab/status:
    get:
      operationId: get${pascal}Status
      summary: $name readiness (generated stub)
      description: Generated by \`make new-domain\` to prove the wiring; replace with the domain's real operations.
      tags: [$name]
      responses:
        '200':
          description: The domain's storage answers.
          content:
            application/json:
              schema:
                \$ref: '#/components/schemas/${pascal}Status'
        '4XX':
          \$ref: '#/components/responses/ClientError'
        '503':
          \$ref: '#/components/responses/ServiceUnavailable'
        '5XX':
          \$ref: '#/components/responses/ServerError'
EOF
insert "$SPEC" 'harness:domain-schemas' <<EOF
    ${pascal}Status:
      type: object
      required: [status]
      properties:
        status:
          type: string
          enum: [ok]
EOF

if grep -q '^sql: \[\]' "$SQLC"; then
  tmp="$(mktemp)"
  sed 's/^sql: \[\]$/sql:/' "$SQLC" >"$tmp" && mv "$tmp" "$SQLC"
fi
cat >>"$SQLC" <<EOF
  - engine: postgresql
    schema: db/migrations
    queries: db/queries/$name.sql
    gen:
      go:
        package: store
        out: internal/$name/store
        sql_package: pgx/v5
EOF

insert "$QUALITY" 'harness:quality-rows' <<EOF
| [$name](../apps/api/internal/$name/AGENTS.md) | – | none | AGENTS.md | generated stub |
EOF

# --- migration ----------------------------------------------------------------------------------------
(cd apps/api && atlas migrate new "$name" --env local)
migration="$(find apps/api/db/migrations -name "*_${name}.sql" | sort | tail -1)"
cat >"$migration" <<EOF
-- First migration of the $name domain: add its tables here (.claude/skills/add-migration).
SET lock_timeout = '2s';
SET statement_timeout = '30s';
EOF

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
EOF
