# Single entry point for humans, agents and CI. Every sensor runs through scripts/harness/run.sh:
# silent on success, actionable on failure. `make help` lists targets.
SHELL := bash
.SHELLFLAGS := -euo pipefail -c # make ≥3.82; macOS make 3.81 ignores it, recipes stay explicit
.DEFAULT_GOAL := help
MAKEFLAGS += --no-print-directory

include scripts/tool-versions.env
export GOTOOLCHAIN := $(GO_TOOLCHAIN)
export PATH := $(CURDIR)/.bin:$(CURDIR)/node_modules/.bin:$(PATH)

RUN  := scripts/harness/run.sh
# Load this worktree's ports/DB URL (written by `make setup`) into a recipe's shell.
WITH_ENV := [[ -f .env.local ]] || { echo "no .env.local: run make setup" >&2; exit 1; }; set -a; source .env.local; set +a;
API  := apps/api
GCL  := .bin/golangci-lint-custom
ANALYZERS_SRC := $(shell find $(API)/tools/analyzers -name '*.go' -o -name 'go.mod' -o -name 'go.sum')

##@ Workflow
.PHONY: help setup check check-all harness-selftest new-domain dev db-up db-migrate db-reset

help: ## Show targets
	@awk 'BEGIN{FS=":.*##"} /^##@/{printf "\n%s\n",substr($$0,5)} /^[a-z0-9-]+:.*##/{printf "  %-18s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

setup: ## Tools, deps, this worktree's Postgres (own ports) + migrations, .env.local
	@scripts/setup-worktree.sh

new-domain: ## Scaffold a domain through every layer: make new-domain NAME=billing
	@scripts/new-domain.sh $(NAME)

dev: db-up ## Run API + web for this worktree (Ctrl-C stops both)
	@$(WITH_ENV) trap 'kill 0' EXIT; \
	  echo "api http://$$HTTP_ADDR  web http://localhost:$$WEB_PORT"; \
	  (cd $(API) && go run ./cmd/api) & (cd apps/web && node_modules/.bin/vite) & wait

db-up: ## Start this worktree's Postgres
	@$(WITH_ENV) docker compose --progress quiet up -d --wait db

db-migrate: db-up ## Apply pending migrations to this worktree's Postgres
	@$(WITH_ENV) cd $(API) && atlas migrate apply --env local >/dev/null

db-reset: ## Drop this worktree's database volume, recreate, migrate
	@$(WITH_ENV) docker compose --progress quiet down -v; docker compose --progress quiet up -d --wait db
	@$(WITH_ENV) cd $(API) && atlas migrate apply --env local >/dev/null

check: ## Fast sensors (<60s warm): format, lint, arch, unit tests, gen-check
	@$(MAKE) -k -j4 lint-go arch-go test-go test-analyzers lint-sh fmt-check-sh test-sh lint-ci schema-doc-fresh docs-links lint-web arch-web typecheck-web test-web
	@$(MAKE) gen-check

# CI skips the sensors that run in their own base-branch workflow (contract-gate, harness-integrity).
CHECK_ALL_SKIP ?=
check-all: check ## Everything: check + integration, contract, migration, security, dead code
	@$(MAKE) -k -j4 $(filter-out $(CHECK_ALL_SKIP),vuln-go lint-migrations squawk migrations-append-only lint-api contract-check knip harness-integrity commits specs)
	@$(MAKE) test-integration

harness-selftest: ## Every fixture in scripts/harness/fixtures must be rejected by its sensor (FIXTURE=name for one)
	@scripts/harness/selftest.sh $(FIXTURE)

##@ Tools
.PHONY: tools deps
tools: $(GCL) ## Install pinned CLI tools into .bin
	@scripts/install-tools.sh

deps: ## Install JS dependencies from the lockfile
	@pnpm install --frozen-lockfile --silent

$(GCL): $(API)/.custom-gcl.yml $(ANALYZERS_SRC) scripts/tool-versions.env
	@scripts/install-tools.sh
	@grep -q "^version: $(GOLANGCI_LINT_VERSION)$$" $(API)/.custom-gcl.yml || { echo ".custom-gcl.yml version != GOLANGCI_LINT_VERSION ($(GOLANGCI_LINT_VERSION))" >&2; exit 1; }
	@cd $(API) && golangci-lint custom
	@touch $@

##@ Go sensors
.PHONY: fmt lint-go arch-go test-go test-analyzers vuln-go deadcode-go
fmt: $(GCL) ## Auto-format Go and shell code
	@cd $(API) && ../../$(GCL) fmt ./...
	@shfmt -w $(SH_FILES)

lint-go: $(GCL)
	@$(RUN) golangci-lint go-no-root-context,go-db-only-in-store,go-http-only-in-api,go-domain-isolation,go-format --dir $(API) -- ../../$(GCL) run ./...

arch-go:
	@$(RUN) go-arch-lint go-layers --dir $(API) -- go-arch-lint check --output-color=false

test-go:
	@$(RUN) go-test go-tests-pass --dir $(API) -- go test -race -count=1 -short ./...

test-analyzers:
	@$(RUN) analyzer-tests go-tests-pass --dir $(API)/tools/analyzers -- go test -count=1 ./...

vuln-go:
	@$(RUN) govulncheck go-no-known-vulns --dir $(API) -- govulncheck ./...

deadcode-go: ## Unreachable Go functions (drift.yml; deadcode exits 0, so fail on any output)
	@$(RUN) deadcode go-no-dead-code --dir $(API) -- bash -c 'out="$$(deadcode -test ./...)"; [[ -z "$$out" ]] || { echo "$$out"; exit 1; }'

##@ Web sensors (apps/web, packages/*)
.PHONY: lint-web arch-web typecheck-web test-web knip
lint-web:
	@$(RUN) eslint web-no-direct-http,web-domain-pure,web-lint -- eslint --max-warnings=0 .

arch-web:
	@$(RUN) dependency-cruiser web-domain-pure,web-no-cross-app -- depcruise apps packages --config .dependency-cruiser.cjs --output-type err

typecheck-web:
	@$(RUN) tsc web-typecheck -- turbo run typecheck --output-logs=errors-only

test-web:
	@$(RUN) vitest web-tests-pass -- turbo run test --output-logs=errors-only

knip: ## Unused files, exports and dependencies in the JS workspace
	@$(RUN) knip web-no-dead-code -- knip

##@ Contract (api/openapi.yaml)
.PHONY: lint-api contract-check
API_RULES := api-kebab-case-paths,api-camel-case-properties,api-operation-metadata,api-documented-errors,api-errors-problem-json

lint-api: ## Spectral style + error-contract rules on the OpenAPI spec
	@$(RUN) spectral $(API_RULES) -- spectral lint api/openapi.yaml --ruleset .spectral.yaml --fail-severity=warn --quiet

contract-check: ## oasdiff breaking-change check vs merge base with BASE_REF (contract-gate)
	@$(RUN) oasdiff api-no-breaking-change -- env BASE_REF="$(BASE_REF)" scripts/harness/contract-check.sh

##@ Database
.PHONY: lint-migrations squawk migrations-append-only gen-schema-doc schema-doc-fresh test-integration
BASE_REF ?= $(shell scripts/harness/merge-base.sh --ref)
ATLAS_LINT_SCOPE ?= --git-base "$(BASE_REF)"

lint-migrations: ## Atlas migrate lint (destructive + incompatible), needs Docker
	@$(RUN) atlas-lint db-no-destructive-migration,db-backward-compatible-migration --dir $(API) -- atlas migrate lint --env local $(ATLAS_LINT_SCOPE)

squawk: ## Postgres locking checks on migrations
	@$(RUN) squawk db-safe-locking -- scripts/harness/squawk.sh

migrations-append-only: ## Migrations on BASE_REF are never edited or deleted (migration-gate)
	@$(RUN) migrations db-append-only-migrations -- env BASE_REF="$(BASE_REF)" scripts/harness/migrations-append-only.sh

gen-schema-doc: ## Regenerate docs/generated/db-schema.md (needs Docker)
	@scripts/gen-schema-doc.sh

schema-doc-fresh:
	@$(RUN) schema-doc db-schema-doc-fresh -- scripts/gen-schema-doc.sh --check

test-integration: ## Integration tests against the worktree's Postgres (.env.local)
	@$(WITH_ENV) $(RUN) go-integration go-tests-pass --dir $(API) -- go test -race -count=1 -tags=integration ./...

##@ Shell sensors (scripts/, .claude/hooks/)
.PHONY: lint-sh fmt-check-sh test-sh
# Every shell script and bats file, except fixture overlays (deliberate violations).
# .specify/ is spec-kit's vendored code (ADR 0011): replaced on upgrade, not edited.
SH_FILES = $(shell git ls-files -co --exclude-standard '*.sh' '*.bash' '*.bats' 'scripts/git-hooks/*' ':!:scripts/harness/fixtures/*/files/**' ':!:.specify/**')

lint-sh: ## shellcheck (.shellcheckrc) on every shell script
	@$(RUN) shellcheck sh-shellcheck -- shellcheck -f gcc $(SH_FILES)

fmt-check-sh: ## shfmt (style from .editorconfig) on every shell script
	@$(RUN) shfmt sh-format -- shfmt -d $(SH_FILES)

test-sh: ## bats tests for scripts/ and .claude/hooks/ (scripts/test)
	@$(RUN) bats sh-tests-pass -- bats scripts/test

##@ CI and harness integrity (.github/, protected harness files)
.PHONY: lint-ci harness-integrity commits specs
lint-ci: ## actionlint (+ shellcheck on run: blocks) on .github/workflows
	@$(RUN) actionlint ci-workflows-valid -- actionlint -no-color

harness-integrity: ## Added suppressions carry a reason; harness configs are not loosened (vs BASE_REF)
	@$(RUN) integrity harness-suppression-reason,harness-config-not-loosened -- env BASE_REF="$(BASE_REF)" scripts/harness/integrity-check.sh

# PR_TITLE reaches these from the pull_request_target job as an env var (ADR 0011): never write $(PR_TITLE)
# in a recipe, make would expand a title like `$(shell …)` in a job that holds the base branch's checkout.
commits: ## Commit subjects (and PR_TITLE) since BASE_REF follow Conventional Commits
	@$(RUN) commits git-conventional-commits -- env BASE_REF="$(BASE_REF)" scripts/harness/check-commits.sh

specs: ## spec-kit specs/ are well formed; feat changes to apps/ or api/ come with a spec (vs BASE_REF)
	@$(RUN) specs specs-structure,specs-required-for-features -- env BASE_REF="$(BASE_REF)" scripts/harness/check-specs.sh

##@ Docs
.PHONY: docs-links
docs-links: ## Relative links and anchors in Markdown resolve; every AGENTS.md has a CLAUDE.md importing it
	@$(RUN) check-links docs-links-resolve,docs-claude-imports-agents -- scripts/harness/check-links.sh

##@ Generation
.PHONY: gen gen-check gen-sqlc gen-atlas-sum gen-oapi gen-api-client
gen: gen-oapi gen-api-client gen-sqlc gen-atlas-sum ## Regenerate all generated code (then commit it)

gen-oapi: # one strict server per OpenAPI tag: every apps/api/internal/**/oapi-codegen.yaml
	@for cfg in $$(find $(API)/internal -name oapi-codegen.yaml | sort); do \
	  (cd $$(dirname $$cfg) && oapi-codegen -config oapi-codegen.yaml $(CURDIR)/api/openapi.yaml) || exit 1; \
	done

gen-api-client: # orval: TanStack Query hooks + zod schemas into packages/api-client/src/gen
	@cd packages/api-client && node_modules/.bin/orval --config orval.config.ts >/dev/null

gen-sqlc:
	@cd $(API) && if grep -q -- '- engine:' sqlc.yaml; then sqlc generate; fi

gen-atlas-sum:
	@cd $(API) && atlas migrate hash --env local

gen-check: ## gen + fail if anything changed (generated code must be committed and untouched)
	@$(RUN) gen-check gen-up-to-date -- scripts/harness/gen-check.sh
