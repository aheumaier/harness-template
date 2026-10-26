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
	@$(MAKE) -k -j4 lint-go arch-go test-go test-analyzers

check-all: check ## Everything: check + slow/networked sensors
	@$(MAKE) -k -j4 vuln-go

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
