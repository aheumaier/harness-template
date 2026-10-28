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
# Tool caches only in make check (scope `changed`, also when unset); any other scope, as in make check-all
# and CI (`full`), runs uncached (rules.md#harness-scope). Recursive (=): check-all sets the scope per target.
CACHED = $(filter changed,$(or $(HARNESS_SCOPE),changed))
GO_TEST_COUNT = $(if $(CACHED),,-count=1)
DEPCRUISE_CACHE = $(if $(CACHED),--cache node_modules/.cache/depcruise)
TURBO_FORCE = $(if $(CACHED),,--force)
# Load this worktree's ports/DB URL (written by `make setup`) into a recipe's shell.
WITH_ENV := [[ -f .env.local ]] || { echo "no .env.local: run make setup" >&2; exit 1; }; set -a; source .env.local; set +a;
API  := apps/api
# `$(call scoped,<go|web>,<everything>) <command on $$scope>`: in make check, runs the command on the packages or
# workspaces a change affects plus their dependents (scripts/harness/affected.sh), or skips it when there are
# none; on <everything> in make check-all, CI and whenever affected.sh cannot narrow it (rules.md#harness-scope).
scoped = scope="$$(scripts/harness/affected.sh $(1))" || scope="$(2)"; [[ -n "$$scope" ]] || \
	{ echo "$@: skipped, nothing it checks is affected (make check-all runs it)"; exit 0; };
GCL  := .bin/golangci-lint-custom
ANALYZERS_SRC := $(shell find $(API)/tools/analyzers -name '*.go' -o -name 'go.mod' -o -name 'go.sum')
HARNESS     := .bin/harness
HARNESS_SRC := $(shell find $(API)/tools/harness -name '*.go' -o -name 'go.mod' -o -name 'go.sum')

##@ Workflow
.PHONY: help setup check check-all harness-selftest harness-determinism new-domain rename dev db-up db-migrate db-reset

help: ## Show targets
	@awk 'BEGIN{FS=":.*##"} /^##@/{printf "\n%s\n",substr($$0,5)} /^[a-z0-9-]+:.*##/{printf "  %-18s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

setup: ## Tools, deps, this worktree's Postgres (own ports) + migrations, .env.local
	@scripts/setup-worktree.sh

new-domain: ## Scaffold a domain through every layer: make new-domain NAME=billing
	@scripts/new-domain.sh $(NAME)

rename: $(HARNESS) ## Rename the template once (dry run): make rename NAME=acme-shop MODULE=github.com/acme/shop OWNER=@acme [WRITE=1]
	@$(HARNESS) rename $(if $(filter 1,$(WRITE)),-write) . $(NAME) $(MODULE) $(OWNER)

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

check: $(HARNESS) ## Fast sensors (warm: <=10s without shell/harness changes, <=60s any: NFR-H3/H4): format, lint, arch, unit tests, secrets, gen-check
	@$(MAKE) -k -j4 lint-go arch-go test-go test-analyzers test-harness lint-sh fmt-check-sh test-sh lint-ci secrets schema-doc-fresh docs-links fixture-claims lint-web arch-web typecheck-web test-web
	@$(MAKE) gen-check

# CI skips the sensors that run in their own base-branch workflow (contract-gate, harness-integrity).
CHECK_ALL_SKIP ?=
check-all: export HARNESS_SCOPE := full
check-all: check ##Everything: check + integration, contract, migration, security, dead code
	@$(MAKE) -k -j4 $(filter-out $(CHECK_ALL_SKIP),vuln-go vuln-deps lint-migrations squawk migrations-append-only lint-api contract-check knip harness-integrity commits specs)
	@$(MAKE) test-integration

harness-selftest: $(HARNESS) ## Every fixture in scripts/harness/fixtures must be rejected by its sensor (FIXTURE=name for one)
	@scripts/harness/selftest.sh $(FIXTURE)

# NFR-R1, nightly in harness-nfr.yml. The selftest fixture swaps the target for one whose verdict alternates.
# Scope `full`: in `changed` scope run 1 would stamp gen-check and run 2 skip it, a difference by design.
harness-determinism: export HARNESS_SCOPE := full
DETERMINISM_TARGET ?= check
harness-determinism: ## Run make check twice on this tree: same exit status, findings and tree (NFR-R1)
	@$(RUN) determinism harness-deterministic -- scripts/harness/determinism.sh $(MAKE) $(DETERMINISM_TARGET)

##@ Reports
.PHONY: coverage coverage-fell
# A trend, never a gate (ADR 0020): not in check or check-all; coverage.yml puts it in its job summary,
# drift.yml compares it with the last scheduled run's. COVERAGE_PREVIOUS=<coverage.tsv> adds the change column.
COVERAGE_DIR := .cache/coverage
COVERAGE_TSV := $(COVERAGE_DIR)/coverage.tsv
COVERAGE_PREVIOUS ?=
coverage: ## Unit-test coverage per Go package and JS workspace, as Markdown (.cache/coverage/coverage.tsv)
	@scripts/harness/coverage.sh collect $(COVERAGE_DIR)
	@scripts/harness/coverage.sh report $(COVERAGE_TSV) "$(COVERAGE_PREVIOUS)"
coverage-fell: ## After make coverage: only the packages that fell since COVERAGE_PREVIOUS=<coverage.tsv>
	@scripts/harness/coverage.sh report --fell $(COVERAGE_TSV) "$(COVERAGE_PREVIOUS)"

##@ Tools
.PHONY: tools deps
tools: $(GCL) $(HARNESS) ## Install pinned CLI tools into .bin
	@scripts/install-tools.sh

deps: ## Install JS dependencies from the lockfile
	@pnpm install --frozen-lockfile --silent

$(GCL): $(API)/.custom-gcl.yml $(ANALYZERS_SRC) scripts/tool-versions.env
	@scripts/install-tools.sh
	@grep -q "^version: $(GOLANGCI_LINT_VERSION)$$" $(API)/.custom-gcl.yml || { echo ".custom-gcl.yml version != GOLANGCI_LINT_VERSION ($(GOLANGCI_LINT_VERSION))" >&2; exit 1; }
	@cd $(API) && golangci-lint custom
	@touch $@

# The hooks' parser (ADR 0012); every hook fails closed without it.
$(HARNESS): $(HARNESS_SRC) scripts/tool-versions.env
	@cd $(API)/tools/harness && go build -o $(CURDIR)/$@ .

##@ Go sensors
.PHONY: fmt lint-go arch-go test-go test-analyzers test-harness vuln-go deadcode-go
fmt: $(GCL) ## Auto-format Go and shell code
	@cd $(API) && ../../$(GCL) fmt ./...
	@shfmt -w $(SH_FILES)

lint-go: $(GCL)
	@$(call scoped,go,./...) $(RUN) golangci-lint go-no-root-context,go-db-only-in-store,go-http-only-in-api,go-domain-isolation,go-structured-logging,go-sec,go-format --dir $(API) -- ../../$(GCL) run $$scope

arch-go:
	@$(RUN) go-arch-lint go-layers --dir $(API) -- go-arch-lint check --output-color=false

test-go:
	@$(call scoped,go,./...) $(RUN) go-test go-tests-pass --dir $(API) -- go test -race $(GO_TEST_COUNT) -short $$scope

test-analyzers:
	@$(RUN) analyzer-tests go-tests-pass --dir $(API)/tools/analyzers -- go test $(GO_TEST_COUNT) ./...

test-harness:
	@$(RUN) harness-tests go-tests-pass --dir $(API)/tools/harness -- go test $(GO_TEST_COUNT) ./...

vuln-go:
	@$(RUN) govulncheck go-no-known-vulns --dir $(API) -- govulncheck ./...

deadcode-go: ## Unreachable Go functions (drift.yml; deadcode exits 0, so fail on any output)
	@$(RUN) deadcode go-no-dead-code --dir $(API) -- bash -c 'out="$$(deadcode -test ./...)"; [[ -z "$$out" ]] || { echo "$$out"; exit 1; }'

##@ Web sensors (apps/web, packages/*)
.PHONY: lint-web arch-web typecheck-web test-web knip
lint-web:
	@$(call scoped,web,.) $(RUN) eslint web-no-direct-http,web-domain-pure,web-lint -- eslint --max-warnings=0 $$scope

arch-web:
	@$(call scoped,web,apps packages) $(RUN) dependency-cruiser web-domain-pure,web-no-cross-app -- depcruise $$scope --config .dependency-cruiser.cjs --output-type err $(DEPCRUISE_CACHE)

typecheck-web:
	@$(RUN) tsc web-typecheck -- turbo run typecheck --output-logs=errors-only $(TURBO_FORCE)

test-web:
	@$(RUN) vitest web-tests-pass -- turbo run test --output-logs=errors-only $(TURBO_FORCE)

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
# $(call sh_sensor,<command>): in make check, runs only when a shell, bats or harness file changed
# (rules.md#harness-scope); make check-all and CI always run it.
sh_sensor = if scripts/harness/unchanged.sh '*.sh' '*.bash' '*.bats' 'scripts/*'; then \
	echo "$@: skipped, no shell or harness file changed (make check-all runs it)"; else $(1); fi

lint-sh: ## shellcheck (.shellcheckrc) on every shell script
	@$(call sh_sensor,$(RUN) shellcheck sh-shellcheck -- shellcheck -f gcc $(SH_FILES))

fmt-check-sh: ## shfmt (style from .editorconfig) on every shell script
	@$(call sh_sensor,$(RUN) shfmt sh-format -- shfmt -d $(SH_FILES))

test-sh: $(HARNESS) ## bats tests for scripts/ and .claude/hooks/ (scripts/test)
	@$(call sh_sensor,$(RUN) bats sh-tests-pass -- bats scripts/test)

##@ Secrets and dependency vulnerabilities (Trivy: trivy.yaml, scripts/harness/trivy.sh)
.PHONY: secrets vuln-deps
# The sensor id is the rule id: budgets.txt row repo-no-secrets (NFR-H9). $(GCL) runs install-tools.sh
# (Trivy too) once tool-versions.env changes, before the scan, not racing it under -j4.
secrets: $(GCL) ## Offline secret scan of the git-known files (in make check, only the changed ones)
	@$(RUN) repo-no-secrets repo-no-secrets -- scripts/harness/trivy.sh secret

vuln-deps: $(GCL) ## Known vulns in pnpm-lock.yaml and every go.mod (fetches the Trivy DB into .cache/trivy)
	@$(RUN) trivy-vuln deps-no-known-vulns -- scripts/harness/trivy.sh vuln

##@ CI and harness integrity (.github/, protected harness files)
.PHONY: lint-ci harness-integrity commits specs
lint-ci: ## actionlint (+ shellcheck on run: blocks) on .github/workflows
	@$(RUN) actionlint ci-workflows-valid -- actionlint -no-color

harness-integrity: $(HARNESS) ## Added suppressions carry a reason; harness configs are not loosened (vs BASE_REF)
	@$(RUN) integrity harness-suppression-reason,harness-config-not-loosened -- env BASE_REF="$(BASE_REF)" $(HARNESS) integrity-check .

# PR_TITLE reaches these from the pull_request_target job as an env var (ADR 0011): never write $(PR_TITLE)
# in a recipe, make would expand a title like `$(shell …)` in a job that holds the base branch's checkout.
commits: ## Commit subjects (and PR_TITLE) since BASE_REF follow Conventional Commits
	@$(RUN) commits git-conventional-commits -- env BASE_REF="$(BASE_REF)" scripts/harness/check-commits.sh

specs: ## spec-kit specs/ are well formed; feat changes to apps/ or api/ come with a spec (vs BASE_REF)
	@$(RUN) specs specs-structure,specs-required-for-features -- env BASE_REF="$(BASE_REF)" scripts/harness/check-specs.sh

##@ Docs
.PHONY: docs-links fixture-claims
docs-links: $(HARNESS) ## Relative links and anchors in Markdown resolve; every AGENTS.md has a CLAUDE.md importing it
	@$(RUN) check-links docs-links-resolve,docs-claude-imports-agents -- $(HARNESS) check-links .

# In make check, runs only when rules.md, a fixture or another harness file changed (rules.md#harness-scope).
fixture-claims: $(HARNESS) ## Every rules.md Fixture line names fixtures that exist and test that rule, or none (<reason>)
	@if scripts/harness/unchanged.sh docs/harness/rules.md 'scripts/harness/fixtures/*'; then \
	  echo "$@: skipped, neither rules.md nor a fixture changed (make check-all runs it)"; \
	else $(RUN) fixture-claims rules-fixture-claims -- $(HARNESS) fixture-claims .; fi

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
	@if scripts/harness/gen-check.sh --unchanged; then \
	  echo "$@: skipped, generator inputs and outputs unchanged since the last green run (make check-all runs it)"; \
	else $(RUN) gen-check gen-up-to-date -- scripts/harness/gen-check.sh; fi
