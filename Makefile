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
API  := apps/api
GCL  := .bin/golangci-lint-custom
ANALYZERS_SRC := $(shell find $(API)/tools/analyzers -name '*.go' -o -name 'go.mod' -o -name 'go.sum')

##@ Workflow
.PHONY: help setup check check-all

help: ## Show targets
	@awk 'BEGIN{FS=":.*##"} /^##@/{printf "\n%s\n",substr($$0,5)} /^[a-z0-9-]+:.*##/{printf "  %-18s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

setup: tools ## Install pinned tools and dependencies

check: ## Fast sensors (<60s warm): format, lint, arch, unit tests, gen-check
	@$(MAKE) -k -j4 lint-go arch-go test-go test-analyzers schema-doc-fresh
	@$(MAKE) gen-check

check-all: check ## Everything: check + integration, contract, migration, security, dead code
	@$(MAKE) -k -j4 vuln-go lint-migrations squawk lint-api contract-check
	@$(MAKE) test-integration

##@ Tools
.PHONY: tools
tools: $(GCL) ## Install pinned CLI tools into .bin
	@scripts/install-tools.sh

$(GCL): $(API)/.custom-gcl.yml $(ANALYZERS_SRC) scripts/tool-versions.env
	@scripts/install-tools.sh
	@grep -q "^version: $(GOLANGCI_LINT_VERSION)$$" $(API)/.custom-gcl.yml || { echo ".custom-gcl.yml version != GOLANGCI_LINT_VERSION ($(GOLANGCI_LINT_VERSION))" >&2; exit 1; }
	@cd $(API) && golangci-lint custom
	@touch $@

##@ Go sensors
.PHONY: fmt lint-go arch-go test-go test-analyzers vuln-go deadcode-go
fmt: $(GCL) ## Auto-format Go code
	@cd $(API) && ../../$(GCL) fmt ./...

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

##@ Contract (api/openapi.yaml)
.PHONY: lint-api contract-check
API_RULES := api-kebab-case-paths,api-camel-case-properties,api-operation-metadata,api-documented-errors,api-errors-problem-json

lint-api: ## Spectral style + error-contract rules on the OpenAPI spec
	@$(RUN) spectral $(API_RULES) -- spectral lint api/openapi.yaml --ruleset .spectral.yaml --fail-severity=warn --quiet

contract-check: ## oasdiff breaking-change check vs merge base with BASE_REF (contract-gate)
	@$(RUN) oasdiff api-no-breaking-change -- env BASE_REF=$(BASE_REF) scripts/harness/contract-check.sh

##@ Database
.PHONY: lint-migrations squawk gen-schema-doc schema-doc-fresh test-integration
BASE_REF ?= $(shell git rev-parse -q --verify origin/main >/dev/null && echo origin/main || echo main)
ATLAS_LINT_SCOPE ?= --git-base $(BASE_REF)

lint-migrations: ## Atlas migrate lint (destructive + incompatible), needs Docker
	@$(RUN) atlas-lint db-no-destructive-migration,db-backward-compatible-migration --dir $(API) -- atlas migrate lint --env local $(ATLAS_LINT_SCOPE)

squawk: ## Postgres locking checks on migrations
	@$(RUN) squawk db-safe-locking -- squawk --config .squawk.toml $(API)/db/migrations/*.sql

gen-schema-doc: ## Regenerate docs/generated/db-schema.md (needs Docker)
	@scripts/gen-schema-doc.sh

schema-doc-fresh:
	@$(RUN) schema-doc db-schema-doc-fresh -- scripts/gen-schema-doc.sh --check

test-integration: ## Integration tests against the worktree's Postgres (.env.local)
	@$(RUN) go-integration go-tests-pass --dir $(API) -- bash -c 'set -a; source ../../.env.local; set +a; go test -race -count=1 -tags=integration ./...'

##@ Generation
.PHONY: gen gen-check gen-sqlc gen-atlas-sum gen-oapi
gen: gen-oapi gen-sqlc gen-atlas-sum ## Regenerate all generated code (then commit it)

gen-oapi: # one strict server per OpenAPI tag: every apps/api/internal/**/oapi-codegen.yaml
	@for cfg in $$(find $(API)/internal -name oapi-codegen.yaml | sort); do \
	  (cd $$(dirname $$cfg) && oapi-codegen -config oapi-codegen.yaml $(CURDIR)/api/openapi.yaml) || exit 1; \
	done

gen-sqlc:
	@cd $(API) && if grep -q -- '- engine:' sqlc.yaml; then sqlc generate; fi

gen-atlas-sum:
	@cd $(API) && atlas migrate hash --env local

gen-check: ## gen + fail if anything changed (generated code must be committed and untouched)
	@$(RUN) gen-check gen-up-to-date -- scripts/harness/gen-check.sh
