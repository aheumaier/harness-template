# Single entry point for humans, agents and CI. Every sensor runs through scripts/harness/run.sh:
# silent on success, actionable on failure. `make help` lists targets.
SHELL := bash
.SHELLFLAGS := -euo pipefail -c # make ≥3.82; macOS make 3.81 ignores it, recipes stay explicit
.DEFAULT_GOAL := help
MAKEFLAGS += --no-print-directory

include scripts/tool-versions.env
# Where the code lives (API_DIR, SPEC, ...), one file for every sensor (ADR 0030); its lists separate entries
# with `:`. Read through the validating parser, never included as make syntax: layout.sh prints only its known
# keys with plain values, one KEY=value word each, or nothing and its findings on stderr. HARNESS_STACK, from the
# environment or the command line, overrides its STACK (ADR 0033); make 3.81's $(shell) sees only the former.
# The keys are set with `override`, and so are STACK_OFF and HOST_OFF below, so neither a command-line variable nor one from
# the environment under make -e (CI's setup action exports the keys) can beat layout.env and silently drop the
# db and web sensors past layout-check's guard (it reads only HARNESS_STACK). A command-line STACK, STACK_OFF,
# HOST_OFF or group target list (DB_TARGETS, WEB_TARGETS, IOS_TARGETS and its IOS_SWIFT_TARGETS and IOS_XCODE_TARGETS, which feed the last two) is refused outright, since the one asking for it expects it to win; $(origin) of a command-line variable is
# two words, which @ joins, so make -e's `environment override` passes here and override beats it below.
from_command_line = $(filter command@line override,$(subst $() ,@,$(origin $(1))))
ifneq ($(call from_command_line,STACK),)
$(error STACK comes from scripts/harness/layout.env; for one run use HARNESS_STACK=… [harness-layout-valid])
endif
ifneq ($(call from_command_line,STACK_OFF),)
$(error STACK_OFF follows STACK in scripts/harness/layout.env; for one run use HARNESS_STACK=… [harness-layout-valid])
endif
ifneq ($(call from_command_line,HOST_OFF),)
$(error HOST_OFF follows the host's OS (the iOS sensors need macOS); for one run use HARNESS_STACK=… [harness-layout-valid])
endif
$(foreach v,DB_TARGETS WEB_TARGETS IOS_TARGETS IOS_SWIFT_TARGETS IOS_XCODE_TARGETS,$(if $(call from_command_line,$(v)),$(error $(v) is the Makefile's list of a sensor group's targets; for one run use HARNESS_STACK=… [harness-layout-valid])))
LAYOUT :=$(shell $(if $(filter command line,$(origin HARNESS_STACK)),HARNESS_STACK='$(HARNESS_STACK)') scripts/harness/layout.sh --print)
ifeq ($(LAYOUT),)
$(error scripts/harness/layout.env or HARNESS_STACK is missing or invalid (see above): fix it, then run make layout-check [harness-layout-valid])
endif
$(foreach kv,$(LAYOUT),$(eval override $(kv)))
# The sensor groups (ADR 0033): go is every target not listed here and always runs; a db or web target leaves
# check, check-all, gen and make harness-selftest when STACK lacks its group. e2e needs both (make dev starts
# Postgres and the web app). An ios target also needs a macOS host (Swift with Apple's SDKs): elsewhere it is in
# HOST_OFF and leaves them by name (ADR 0042). The ios targets that need only Swift (the command-line tools are
# enough) run in check; those that need Xcode itself only in check-ios (ADR 0043). `$(call stack_has,<group>)` is non-empty when STACK names it;
# `$(call stacked,<targets>)` drops the targets STACK or the host leaves out; check says once on stderr which ones.
DB_TARGETS  := schema-doc-fresh lint-migrations squawk migrations-append-only test-integration test-obs e2e gen-sqlc gen-atlas-sum
WEB_TARGETS := lint-web arch-web typecheck-web test-web knip clones-web e2e gen-api-client
IOS_SWIFT_TARGETS := test-ios lint-ios fmt-check-ios
IOS_XCODE_TARGETS := xcode-pinned build-ios
IOS_TARGETS := $(IOS_SWIFT_TARGETS) $(IOS_XCODE_TARGETS)
override stack_has = $(filter $(1),$(subst :, ,$(STACK)))
override STACK_OFF := $(strip $(if $(call stack_has,db),,$(DB_TARGETS)) $(if $(call stack_has,web),,$(WEB_TARGETS)) $(if $(call stack_has,ios),,$(IOS_TARGETS)))
override HOST_OFF  := $(if $(filter Darwin,$(shell uname -s)),,$(IOS_TARGETS))
override stacked   = $(filter-out $(STACK_OFF) $(HOST_OFF),$(1))
export GOTOOLCHAIN := $(GO_TOOLCHAIN)
export OBS_IMAGE # compose.yaml interpolates the obs service even when its profile is off
export PATH := $(CURDIR)/.bin:$(CURDIR)/node_modules/.bin:$(PATH)
# Playwright's browser (make e2e) lives in .bin like every tool, installed on demand by make e2e only (ADR 0025).
export PLAYWRIGHT_BROWSERS_PATH := $(CURDIR)/.bin/playwright

RUN  := scripts/harness/run.sh
# Tool caches only in make check (scope `changed`, also when unset); any other scope, as in make check-all
# and CI (`full`), runs uncached (rules.md#harness-scope). Recursive (=): check-all sets the scope per target.
CACHED = $(filter changed,$(or $(HARNESS_SCOPE),changed))
GO_TEST_COUNT = $(if $(CACHED),,-count=1)
DEPCRUISE_CACHE = $(if $(CACHED),--cache node_modules/.cache/depcruise)
TURBO_FORCE = $(if $(CACHED),,--force)
# Load this worktree's ports/DB URL (written by `make setup`) into a recipe's shell.
WITH_ENV := [[ -f .env.local ]] || { echo "no .env.local: run make setup" >&2; exit 1; }; set -a; source .env.local; set +a;
# The JS workspaces' parent directories: each WEB_GLOBS entry without its /*.
WEB_DIRS := $(patsubst %/*,%,$(subst :, ,$(WEB_GLOBS)))
# The domains' parent directory (each domain is <DOMAINS_DIR>/<name>): LAYER_GLOB without its /*/*, in API_DIR.
DOMAINS_DIR := $(patsubst ./%,%,$(API_DIR)/$(patsubst %/*/*,%,$(LAYER_GLOB)))
# `$(call scoped,<go|web>,<everything>) <command on $$scope>`: in make check, runs the command on the packages or
# workspaces a change affects plus their dependents (scripts/harness/affected.sh), or skips it when there are
# none; on <everything> in make check-all, CI and whenever affected.sh cannot narrow it (rules.md#harness-scope).
scoped = scope="$$(scripts/harness/affected.sh $(1))" || scope="$(2)"; [[ -n "$$scope" ]] || \
	{ echo "$@: skipped, nothing it checks is affected (make check-all runs it)"; exit 0; };
GCL  := .bin/golangci-lint-custom
# ADR 0024's size and complexity limits, one rule id per measure, checked by lint-go, lint-web and lint-ios. The linters'
# messages cannot carry the id: run.sh maps each finding to its id, so only that measure's split recipe prints.
SIZE_RULES := code-function-length,code-complexity,code-parameters,code-file-length
ANALYZERS_SRC := $(shell find $(API_DIR)/tools/analyzers -name '*.go' -o -name 'go.mod' -o -name 'go.sum')
HARNESS     := .bin/harness
HARNESS_SRC := $(shell find $(API_DIR)/tools/harness -name '*.go' -o -name 'go.mod' -o -name 'go.sum')
# install-tools.sh's stamp (TD-9): a target that needs a pinned tool from .bin depends on this, never on $(GCL).
TOOLS_STAMP := .bin/.tools-stamp

##@ Workflow
.PHONY: help setup check check-all harness-selftest harness-determinism new-domain rename dev db-up db-migrate db-reset

help: ## Show targets
	@awk 'BEGIN{FS=":.*##"} /^##@/{printf "\n%s\n",substr($$0,5)} /^[a-z0-9-]+:.*##/{printf "  %-18s %s\n",$$1,$$2}' $(MAKEFILE_LIST)

setup: ## Tools, deps, this worktree's Postgres (own ports) + migrations, .env.local
	@scripts/setup-worktree.sh

new-domain: ## Scaffold a domain through every layer: make new-domain NAME=billing
	@scripts/new-domain.sh $(NAME)

rename: $(HARNESS) ## Rename the template once (dry run): make rename NAME=acme-shop MODULE=github.com/acme/shop|shop|skip OWNER=@acme [WRITE=1]
	@$(HARNESS) rename $(if $(filter 1,$(WRITE)),-write) . $(NAME) $(MODULE) $(OWNER)

# Each process's output (the API's JSON log lines, Vite's text) also goes to .tmp/dev/<name>.log, emptied on start,
# so an agent can read this run without the obs stack.
dev: db-up ## Run API + web for this worktree (Ctrl-C stops both), output also in .tmp/dev/{api,web}.log; telemetry goes to make obs-up's stack if it runs
	@$(WITH_ENV) off=; \
	  if [[ -z "$${OTLP_PORT:-}" ]]; then off="no OTLP_PORT in this worktree's env: run make setup"; \
	  elif ! (: <"/dev/tcp/127.0.0.1/$$OTLP_PORT") 2>/dev/null; then \
	    off="no obs stack on :$$OTLP_PORT (make obs-up, then restart make dev)"; fi; \
	  if [[ -n "$$off" ]]; then echo "telemetry off: $$off"; \
	    export OTEL_TRACES_EXPORTER=none OTEL_METRICS_EXPORTER=none OTEL_LOGS_EXPORTER=none; fi; \
	  for p in "$$API_PORT" "$$WEB_PORT"; do \
	    if (: <"/dev/tcp/localhost/$$p") 2>/dev/null; then \
	      echo "make dev: port $$p is already in use, likely by this worktree's running make dev; not starting, so" \
	        ".tmp/dev/api.log and .tmp/dev/web.log keep its output. Read them, or stop it first." >&2; exit 1; fi; \
	  done; \
	  trap 'kill 0' EXIT; mkdir -p .tmp/dev; \
	  echo "api http://$$HTTP_ADDR  web http://localhost:$$WEB_PORT  logs .tmp/dev/api.log .tmp/dev/web.log"; \
	  (cd $(API_DIR) && go run ./cmd/api) 2>&1 | tee .tmp/dev/api.log & \
	  (cd apps/web && node_modules/.bin/vite) 2>&1 | tee .tmp/dev/web.log & wait

db-up: ## Start this worktree's Postgres
	@$(WITH_ENV) docker compose --progress quiet up -d --wait db

db-migrate: db-up ## Apply pending migrations to this worktree's Postgres
	@$(WITH_ENV) cd $(API_DIR) && atlas migrate apply --env local >/dev/null

# Two sub-makes, not prerequisites: down must finish before db-migrate starts, also under -j.
db-reset: ## Drop this worktree's database volume (and the obs stack), recreate, migrate
	@$(MAKE) down
	@$(MAKE) db-migrate

check: $(HARNESS) ## Fast sensors (warm: <=10s without shell/harness changes, <=60s any: NFR-H3/H4): format, lint, arch, unit tests, secrets, gen-check
	@$(if $(STACK_OFF),echo "STACK=$(STACK) leaves out: $(sort $(STACK_OFF)) (ADR 0033)" >&2,true)
	@$(if $(and $(call stack_has,ios),$(HOST_OFF)),echo "this host (not macOS) leaves out: $(HOST_OFF) (ADR 0042)" >&2,true)
	@$(MAKE) -k -j4 $(call stacked,layout-check lint-go arch-go test-go test-analyzers test-harness lint-sh fmt-check-sh test-sh lint-ci secrets schema-doc-fresh docs-links references-pinned swift-resolved fixture-claims rule-lists approved-check specs-tested lint-web arch-web typecheck-web test-web $(IOS_SWIFT_TARGETS))
	@$(MAKE) gen-check

# CI skips the sensors that run in their own base-branch workflow (contract-gate, harness-integrity).
CHECK_ALL_SKIP ?=
check-all: export HARNESS_SCOPE := full
check-all: check ## Everything: check + integration, contract, migration, security, dead code, orphan scripts, clones, e2e
	@$(MAKE) -k -j4 $(call stacked,$(filter-out $(CHECK_ALL_SKIP),vuln-go vuln-deps lint-migrations squawk migrations-append-only lint-api contract-check knip deadcode-go clones-web docs-quality-shape harness-integrity orphans commits specs))
	@$(if $(call stacked,test-integration test-obs e2e),$(MAKE) -k $(call stacked,test-integration test-obs e2e),true)

# A fixture whose target or STACK= tag STACK leaves out is skipped (selftest.sh asks make for STACK_OFF, HOST_OFF and
# each group's targets).
harness-selftest: $(HARNESS) ## Every fixture in scripts/harness/fixtures must be rejected by its sensor (FIXTURE=name for one)
	@scripts/harness/selftest.sh $(FIXTURE)

# NFR-R1, nightly in harness-nfr.yml. The selftest fixture swaps the target for one whose verdict alternates.
# Scope `full`: in `changed` scope run 1 would stamp gen-check and run 2 skip it, a difference by design.
harness-determinism: export HARNESS_SCOPE := full
DETERMINISM_TARGET ?= check
harness-determinism: ## Run make check twice on this tree: same exit status, findings and tree (NFR-R1)
	@$(RUN) determinism harness-deterministic -- scripts/harness/determinism.sh $(MAKE) $(DETERMINISM_TARGET)

##@ Runtime: this worktree's logs, metrics and traces (compose profile obs, ADR 0017)
.PHONY: obs-up obs-down down logs metrics traces test-obs
# The query targets read the stack's HTTP APIs through scripts/obs-query.sh; their inputs (REQUEST_ID, QUERY,
# SINCE, LIMIT, ID) reach it as environment variables, never spliced into the recipe. Make expands `$` in a
# command-line value (QUERY='x$y' arrives as 'x'): write `$$`, or pass it from the environment (QUERY=… make logs).
OBS_QUERY := scripts/obs-query.sh
obs-up: ## Start this worktree's OTel collector, Loki, Prometheus, Tempo, Grafana (then restart make dev)
	@$(WITH_ENV) docker compose --progress quiet --profile obs up -d --wait obs && \
	  echo "obs up: make logs, make metrics, make traces; grafana http://127.0.0.1:$$GRAFANA_PORT"

obs-down: ## Stop the obs stack and drop its telemetry (Postgres keeps running)
	@$(WITH_ENV) docker compose --progress quiet --profile obs rm -fsv obs >/dev/null

down: ## Remove all of this worktree's containers and volumes, Postgres data too (make setup recreates)
	@$(WITH_ENV) docker compose --progress quiet --profile obs down -v

logs: ## Log records as JSON lines: make logs REQUEST_ID=<id> | QUERY='<LogQL>' [SINCE=1h] [LIMIT=100] (`$$` for `$`)
	@$(WITH_ENV) $(OBS_QUERY) logs

metrics: ## PromQL instant query, one series per line: make metrics QUERY='<PromQL>' (`$$` for `$`)
	@$(WITH_ENV) $(OBS_QUERY) metrics

traces: ## Spans of one trace as JSON lines, in start order: make traces ID=<trace_id>
	@$(WITH_ENV) $(OBS_QUERY) traces

# NFR-O1 end to end, in make check-all: the API on a free port with its exporters on, one request, read back
# through the targets above. Starts Postgres and, if it is down, the obs stack, which it then removes; a stack
# that was already up stays. OBS_IMAGE (2.58 GB) is pulled first, as its own sensor, so the pull stays out of
# obs-e2e's latency budget. MAKE: the script's own make calls use this make (macOS ships 3.81).
test-obs: ## One request's access line, trace and p95 latency through make logs, traces and metrics (Docker)
	@$(WITH_ENV) $(RUN) obs-pull obs-request-queryable -- scripts/harness/obs-e2e.sh pull
	@$(WITH_ENV) $(RUN) obs-e2e obs-request-queryable -- env MAKE="$(MAKE)" scripts/harness/obs-e2e.sh

##@ Behaviour: browser journeys against this worktree's make dev (exec plan 0003 P2.3, NFR-B2)
.PHONY: e2e e2e-deps
# Playwright (apps/web/playwright.config.ts) attaches to a running make dev, else starts one and stops it after;
# in make check-all (HARNESS_SCOPE=full) it always starts its own and fails while another make dev runs.
# E2E_GREP (a title regex; not GREP, which a shell may already export): make exports it, and the recipe's shell
# reads it as "$E2E_GREP", so make never splices it in.
# Failure artifacts (screenshot, trace) go to .tmp/e2e; make dev's own output to .tmp/dev/{api,web}.log.
# The browser is installed here, on demand and outside the e2e latency budget, so only a job or worktree that
# runs e2e downloads it (ADR 0025).
e2e: ## Browser journeys (Playwright) against make dev, started if not running: make e2e [E2E_GREP=<title regex>] (`$$` for `$`; Docker)
	@scripts/install-tools.sh browsers
	@$(WITH_ENV) $(RUN) e2e web-e2e-journeys --dir apps/web -- env MAKE="$(MAKE)" \
	  node_modules/.bin/playwright test $${E2E_GREP:+--grep "$$E2E_GREP"}

# Linux only: apt through sudo, unpinned (ADR 0025, TD-42). CI's check-all job runs it before make check-all.
e2e-deps: ## System libraries of make e2e's Chromium headless shell (Linux, sudo)
	@[[ -x apps/web/node_modules/.bin/playwright ]] || { echo "no apps/web/node_modules/.bin/playwright: run make deps" >&2; exit 1; }
	@apps/web/node_modules/.bin/playwright install-deps chromium-headless-shell

##@ Reports
.PHONY: coverage coverage-fell mutation mutation-scores
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
# A trend, never a gate (ADR 0023): not in check (slow; the window reads the clock) or check-all;
# drift.yml runs it weekly. MUTATION_SINCE takes any date `git rev-list --before` does.
MUTATION_DIR := .cache/mutation
MUTATION_SINCE ?= 7 days ago
mutation: ## Mutation score and surviving mutants (gremlins, Stryker) of the files changed since MUTATION_SINCE (7 days ago)
	@scripts/harness/mutation.sh collect $(MUTATION_DIR) "$(MUTATION_SINCE)"
	@scripts/harness/mutation.sh report $(MUTATION_DIR)/mutants.tsv "$(MUTATION_SINCE)"
mutation-scores: ## After make mutation: only the score per package, without the surviving mutants (drift.yml's issue)
	@scripts/harness/mutation.sh report --scores $(MUTATION_DIR)/mutants.tsv "$(MUTATION_SINCE)"
# Advisory (ADR 0019): drift.yml feeds this prompt to a read-only agent weekly; locally it is the dry run.
.PHONY: review-prompt
review-prompt: ## Prompt of a scheduled review, as drift.yml sends it: make review-prompt REVIEW=security|data-handling|modularity
	@scripts/harness/scheduled-review.sh prompt "$(REVIEW)"

##@ Tools
.PHONY: tools deps
# Always runs install-tools.sh (once), so it also repairs a deleted .bin tool the stamp cannot see.
tools: ## Install pinned CLI tools into .bin
	@scripts/install-tools.sh
	@touch $(TOOLS_STAMP)
	@$(MAKE) -s $(GCL) $(HARNESS)

deps: ## Install JS dependencies from the lockfile (make e2e installs its browser itself); none without web in STACK
	@$(if $(call stack_has,web),pnpm install --frozen-lockfile --silent,true)

$(TOOLS_STAMP): scripts/tool-versions.env scripts/install-tools.sh $(wildcard scripts/swift-format/Package.resolved)
	@scripts/install-tools.sh
	@touch $@

# Order-only on the stamp: CI restores .bin from cache and touches $(GCL) (.github/actions/setup), and a
# checkout's newer tool-versions.env remakes the stamp; a normal prerequisite would then rebuild $(GCL) too.
$(GCL): $(API_DIR)/.custom-gcl.yml $(ANALYZERS_SRC) scripts/tool-versions.env | $(TOOLS_STAMP)
	@grep -q "^version: $(GOLANGCI_LINT_VERSION)$$" $(API_DIR)/.custom-gcl.yml || { echo ".custom-gcl.yml version != GOLANGCI_LINT_VERSION ($(GOLANGCI_LINT_VERSION))" >&2; exit 1; }
	@cd $(API_DIR) && golangci-lint custom
	@touch $@

# The hooks' parser (ADR 0012); every hook fails closed without it.
$(HARNESS): $(HARNESS_SRC) scripts/tool-versions.env
	@cd $(API_DIR)/tools/harness && go build -o $(CURDIR)/$@ .

##@ Go sensors
.PHONY: fmt lint-go arch-go test-go test-analyzers test-harness vuln-go deadcode-go
fmt: $(GCL) ## Auto-format Go, shell and (ios in STACK, on macOS) Swift code
	@cd $(API_DIR) && $(CURDIR)/$(GCL) fmt ./...
	@shfmt -w $(SH_FILES)
	@$(if $(call stacked,fmt-check-ios),export HARNESS_SCOPE=full; $(call swift_files) swift-format format -i --follow-symlinks --configuration .swift-format "$${files[@]}",true)

lint-go: $(GCL)
	@$(call scoped,go,./...) $(RUN) golangci-lint go-no-root-context,go-db-only-in-store,go-http-only-in-api,go-test-helpers-only-in-tests,go-domain-isolation,go-structured-logging,go-sec,go-format,$(SIZE_RULES),repo-no-clones --dir $(API_DIR) -- $(CURDIR)/$(GCL) run $$scope

arch-go:
	@$(RUN) go-arch-lint go-layers --dir $(API_DIR) -- go-arch-lint check --output-color=false

test-go:
	@$(call scoped,go,./...) $(RUN) go-test go-tests-pass,api-responses-approved --dir $(API_DIR) -- go test -race $(GO_TEST_COUNT) -short $$scope

test-analyzers:
	@$(RUN) analyzer-tests go-tests-pass --dir $(API_DIR)/tools/analyzers -- go test $(GO_TEST_COUNT) ./...

test-harness:
	@$(RUN) harness-tests go-tests-pass,harness-reads-layout --dir $(API_DIR)/tools/harness -- go test $(GO_TEST_COUNT) ./...

vuln-go:
	@$(RUN) govulncheck go-no-known-vulns --dir $(API_DIR) -- govulncheck ./...

# tools/analyzers is left out: deadcode cannot see golangci-lint's module-plugin entry points (TD-70).
deadcode-go: ## Unreachable Go functions in apps/api and tools/harness (check-all, drift.yml; deadcode exits 0, so fail on any output)
	@$(RUN) deadcode go-no-dead-code --dir $(API_DIR) -- bash -c 'set -o pipefail; out="$$(deadcode -test ./... && cd tools/harness && deadcode -test ./... | sed "s|^|tools/harness/|")" || exit 2; [[ -z "$$out" ]] || { echo "$$out"; exit 1; }'

##@ Web sensors (the workspaces of WEB_GLOBS)
.PHONY: lint-web arch-web typecheck-web test-web knip clones-web
lint-web:
	@$(call scoped,web,.) $(RUN) eslint web-no-direct-http,web-domain-pure,web-lint,$(SIZE_RULES) -- eslint --max-warnings=0 $$scope

arch-web:
	@$(call scoped,web,$(WEB_DIRS)) $(RUN) dependency-cruiser web-domain-pure,web-no-cross-app -- depcruise $$scope --config .dependency-cruiser.cjs --output-type err $(DEPCRUISE_CACHE)

typecheck-web:
	@$(RUN) tsc web-typecheck -- turbo run typecheck --output-logs=errors-only $(TURBO_FORCE)

test-web:
	@$(RUN) vitest web-tests-pass -- turbo run test --output-logs=errors-only $(TURBO_FORCE)

knip: ## Unused files, exports and dependencies in the JS workspace
	@$(RUN) knip web-no-dead-code -- knip

# rules.md#repo-no-clones: jscpd reads every file .gitignore does not exclude, in every app and package (as arch-web).
# Its settings are in the protected scripts/harness/jscpd.json; naming it with --config also stops jscpd from loading
# a .jscpd.json, .config/jscpd.json or package.json "jscpd" key it would otherwise find. Generated code is listed
# there by path, as in eslint.config.mjs (ignores); golangci finds it by its header (exclusions.generated): keep the
# copies in step. jscpd names each file relative to the root it was given, so it gets absolute paths and the root is
# cut back off. jscpd's exit 1 is findings only when it printed a clone line; any other failure (a bad config, no
# files: --fail-on-empty) is exit 2, so a broken sensor never reads as a clone (scripts/test/clones-web.bats).
clones-web: ## Copy-pasted code in the JS workspaces (jscpd)
	@$(RUN) jscpd repo-no-clones -- bash -c 'root="$$(pwd -P)"; out="$$(jscpd --config "$$root/scripts/harness/jscpd.json" --reporters xcode --absolute --exit-code 1 --fail-on-empty --no-colors --no-tips $(foreach d,$(WEB_DIRS),"$$root/$(d)"))" && s=0 || s=$$?; echo "$${out//"$$root"\//}"; ((s == 0)) || { ((s == 1)) && grep -Eq ": warning: Found [0-9]+ lines " <<<"$$out" && s=1 || s=2; }; exit $$s'

##@ iOS sensors (the Swift package in IOS_DIR, macOS only: ADR 0042; the Swift linters and Xcode: ADR 0043)
.PHONY: check-ios test-ios lint-ios fmt-check-ios xcode-pinned build-ios ios-project
check-ios: ## Every iOS sensor, those needing Xcode too (the macOS CI job)
	@$(if $(call stacked,$(IOS_TARGETS)),$(MAKE) -k $(call stacked,$(IOS_TARGETS)),echo "$@: skipped, $(if $(call stack_has,ios),this host is not macOS (ADR 0042),STACK=$(STACK) has no ios (ADR 0033))")

# With only the command-line tools selected (xcode-select -p is .../CommandLineTools), swift test finds Swift
# Testing only through the tools' frameworks directory, passed as -F and -rpath; XCTest needs Xcode selected. An app without
# a Swift package (only an XcodeGen project.yml) and without Swift tests in Tests/ has nothing for swift test, which
# would otherwise search the parent folders; with tests there but no package, nothing would run them, so it fails.
# swift test compiles only the test targets the manifest declares: `harness swift-tests` fails on a Swift file in
# Tests/ that none of them compiles (ADR 0044), and swift test still runs, so both kinds of finding show at once.
# .bin/harness is built only past the skips, so a skip by name needs no Go. In scope full (CI, check-all) SwiftPM
# may use only the versions in Package.resolved, so a lockfile left stale by a manifest change fails swift test
# (reported as ios-tests-pass) rather than resolving fresh; locally a new dependency resolves as usual.
test-ios: ## Swift unit tests of the package in IOS_DIR (swift test)
	@[[ -n "$(IOS_DIR)" ]] || { echo "$@: skipped, IOS_DIR is empty in scripts/harness/layout.env (no iOS app)"; exit 0; }; \
	if scripts/harness/unchanged.sh '*.swift' 'Package.resolved' '*/Package.resolved' '$(IOS_DIR)'; then \
	  echo "$@: skipped, no Swift file or file in IOS_DIR changed (make check-all runs it)"; exit 0; fi; \
	if [[ ! -f "$(IOS_DIR)/Package.swift" ]]; then \
	  tests="$$(git ls-files -co --exclude-standard -- '$(IOS_DIR)/Tests/*.swift')" || exit 2; \
	  if [[ -z "$$tests" ]]; then echo "$@: skipped, no Package.swift and no Swift file in Tests/ in IOS_DIR=$(IOS_DIR) (ADR 0042)"; exit 0; fi; \
	  $(RUN) swift-test ios-tests-pass -- bash -c 'echo "$@: Swift tests in $(IOS_DIR)/Tests but no Package.swift there, so nothing runs them: restore the package"; exit 1'; exit; fi; \
	dev="$$(xcode-select -p)" || { echo "$@: no Xcode or command-line tools: run xcode-select --install [ios-tests-pass]" >&2; exit 2; }; fw=(); pin=(); \
	[[ "$(HARNESS_SCOPE)" != full ]] || pin=(--only-use-versions-from-resolved-file); \
	if [[ "$$dev" == */CommandLineTools ]]; then \
	  fw=(-Xswiftc -F -Xswiftc "$$dev/Library/Developer/Frameworks" -Xlinker -rpath -Xlinker "$$dev/Library/Developer/Frameworks"); fi; \
	$(MAKE) -s --no-print-directory $(HARNESS) || exit 2; \
	$(RUN) swift-tests-declared ios-tests-pass -- bash -c 'set -o pipefail; d="$$1"; swift package --package-path "$$d" describe --type json | $(HARNESS) swift-tests . "$$d"' _ "$(IOS_DIR)" && s=0 || s=$$?; \
	$(RUN) swift-test ios-tests-pass --dir "$(IOS_DIR)" -- swift test $${pin[@]+"$${pin[@]}"} $${fw[@]+"$${fw[@]}"} || exit; exit $$s

# `$(call swift_files) <command on "$${files[@]}">`: the Swift files the linters read (owner decision Q8 of exec
# plan 0004), each git-known *.swift under PRODUCT_GLOBS or WEB_GLOBS (layout.env) and not under GENERATED_GLOBS.
# Skips the target by name when there is none, or in make check when no Swift file changed (their configs are
# harness files, which unchanged.sh always counts). The configs are named, so a nested one never applies. -z: git
# quotes a name with a non-ASCII byte, `"`, `\` or a tab, which would drop out (a newline in a name still splits it).
# SwiftPM compiles the Swift behind a link to a directory, which git lists as one file: those files are added, but
# none under a hidden directory (.build, .swiftpm: dependency checkouts and build output, which git ignores).
swift_files = set -o pipefail; \
	if scripts/harness/unchanged.sh '*.swift'; then echo "$@: skipped, no Swift file changed (make check-all runs it)"; exit 0; fi; \
	list="$$(git ls-files -z -co --exclude-standard -- $(foreach g,$(sort $(subst :, ,$(PRODUCT_GLOBS) $(WEB_GLOBS))),'$(g)') \
	  $(foreach g,$(subst :, ,$(GENERATED_GLOBS)),':(exclude)$(g)') | tr '\0' '\n')" || exit 2; files=(); \
	while IFS= read -r f; do \
	  if [[ -L "$$f" && -d "$$f" ]]; then linked="$$(find -L "$$f" -mindepth 1 -name '.*' -prune -o -name '*.swift' -type f -print)" || exit 2; \
	    while IFS= read -r g; do [[ -z "$$g" ]] || files+=("$$g"); done <<<"$$linked"; \
	  elif [[ "$$f" == *.swift && -f "$$f" ]]; then files+=("$$f"); fi; \
	done <<<"$$list"; \
	(($${\#files[@]})) || { echo "$@: skipped, no Swift file under PRODUCT_GLOBS or WEB_GLOBS"; exit 0; };

# $(TOOLS_STAMP): after a bump, the tools are installed (swift-format's build takes minutes) before they run.
# The wrappers (swiftlint.sh, swift-format.sh) exit 1 only for a finding, 2 for a broken tool or config.
lint-ios: $(TOOLS_STAMP) ## SwiftLint on the Swift files (rules.md#ios-lint, the size rules)
	@$(call swift_files) $(RUN) swiftlint ios-lint,$(SIZE_RULES) -- scripts/harness/swiftlint.sh "$${files[@]}"

fmt-check-ios: $(TOOLS_STAMP) ## swift-format lint on the Swift files (rules.md#ios-format; make fmt fixes)
	@$(call swift_files) $(RUN) swift-format ios-format -- scripts/harness/swift-format.sh "$${files[@]}"

# `$(call xcode_selected,<rule id>) <command>`: fails (exit 2) without even the command-line tools, and skips the
# target by name while only they are selected (xcodebuild fails: no Xcode, the third host class of ADR 0043).
xcode_selected = dev="$$(xcode-select -p)" || { echo "$@: no Xcode or command-line tools: run xcode-select --install [$(1)]" >&2; exit 2; }; \
	if [[ "$$dev" == */CommandLineTools ]] && ! xcodebuild -version >/dev/null 2>&1; then \
	  echo "$@: skipped, only the command-line tools are selected, no Xcode (ADR 0043)"; exit 0; fi;

# rules.md#ios-xcode-pinned: xcodebuild -version's first line is "Xcode <version>". With only the command-line
# tools selected, xcodebuild fails: there is no Xcode to compare (the third host class, ADR 0043), skipped by name.
xcode-pinned: ## The selected Xcode is XCODE_VERSION (tool-versions.env)
	@$(call xcode_selected,ios-xcode-pinned) \
	$(RUN) xcode-pinned ios-xcode-pinned -- bash -c 'set -o pipefail; v="$$(xcodebuild -version | sed -n "1s/^Xcode //p")" || exit 2; [[ -n "$$v" ]] || exit 2; \
	  [[ "$$v" == "$(XCODE_VERSION)" ]] || { echo "the selected Xcode is $$v, scripts/tool-versions.env pins XCODE_VERSION=$(XCODE_VERSION)"; exit 1; }'

XCODEGEN = .bin/xcodegen generate --quiet --spec "$(IOS_DIR)/project.yml" --project "$(IOS_DIR)"

# make setup runs it where Xcode is selected, so Xcode can open the app; build-ios generates the project itself.
ios-project: ## The iOS app's Xcode project from IOS_DIR/project.yml (XcodeGen, git-ignored; needs macOS)
	@scripts/install-tools.sh xcodegen && $(XCODEGEN)

# rules.md#ios-app-builds: XcodeGen (installed on demand, as the browsers for e2e) generates the app's Xcode project
# from IOS_DIR/project.yml into IOS_DIR, where git ignores it (ADR 0044), and xcodebuild builds its scheme App for
# the iOS Simulator, unsigned, into .cache/xcode. Needs Xcode selected; skipped by name without it, as xcode-pinned.
build-ios: ## The iOS app builds for the simulator: XcodeGen, then xcodebuild (needs Xcode)
	@[[ -n "$(IOS_DIR)" && -f "$(IOS_DIR)/project.yml" ]] || { echo "$@: skipped, no project.yml in IOS_DIR=$(IOS_DIR) (no app target yet)"; exit 0; }; \
	$(call xcode_selected,ios-app-builds) \
	scripts/install-tools.sh xcodegen || exit 2; \
	$(RUN) xcodegen ios-app-builds -- $(XCODEGEN) || exit; \
	projects=("$(IOS_DIR)"/*.xcodeproj); [[ -d "$${projects[0]}" ]] || { echo "$@: XcodeGen wrote no *.xcodeproj in $(IOS_DIR) [ios-app-builds]" >&2; exit 2; }; \
	[[ $${#projects[@]} == 1 ]] || { echo "$@: want one generated *.xcodeproj in $(IOS_DIR), found: $${projects[*]}; delete the stale ones, they are generated [ios-app-builds]" >&2; exit 2; }; \
	$(RUN) xcodebuild ios-app-builds -- xcodebuild -quiet -project "$${projects[0]}" -scheme App -destination 'generic/platform=iOS Simulator' \
	  -derivedDataPath .cache/xcode CODE_SIGNING_ALLOWED=NO build

##@ Contract (SPEC)
.PHONY: lint-api contract-check upstream-drift approve approved-check
API_RULES := api-kebab-case-paths,api-camel-case-properties,api-operation-metadata,api-documented-errors,api-errors-problem-json

lint-api: ## Spectral style + error-contract rules on the OpenAPI spec
	@$(RUN) spectral $(API_RULES) -- spectral lint $(SPEC) --ruleset .spectral.yaml --fail-severity=warn --quiet

approve: $(HARNESS) ## Rewrite the approved responses from the handler tests, then check them; review the diff
	@$(RUN) approve go-tests-pass --dir $(API_DIR) -- env APPROVE_RESPONSES=1 go test -count=1 -short ./...
	@HARNESS_SCOPE=full $(MAKE) approved-check

# In make check, runs only when the spec, an approved response or a harness file changed (rules.md#harness-scope).
approved-check: $(HARNESS) ## Every approved response (testdata/approved/*.json) is one the OpenAPI spec allows
	@if scripts/harness/unchanged.sh $(SPEC) '*/testdata/approved/*'; then \
	  echo "$@: skipped, neither the spec nor an approved response changed (make check-all runs it)"; \
	else $(RUN) approved-check api-approved-match-contract -- $(HARNESS) approved-check .; fi

contract-check: $(HARNESS) ## oasdiff breaking-change check vs merge base with BASE_REF (contract-gate)
	@$(RUN) oasdiff api-no-breaking-change -- env BASE_REF="$(BASE_REF)" scripts/harness/contract-check.sh

# Network (fetches each source's head): drift.yml runs it weekly; never in make check or check-all.
upstream-drift: $(HARNESS) ## oasdiff breaking changes from each vendored upstream spec (UPSTREAM_SPECS in layout.env) to its source's head; changes nothing
	@$(RUN) upstream-drift api-upstream-no-breaking-change -- scripts/harness/upstream-drift.sh

##@ Database
.PHONY: lint-migrations squawk migrations-append-only gen-schema-doc schema-doc-fresh test-integration test-scaffold
BASE_REF ?= $(shell scripts/harness/merge-base.sh --ref)
ATLAS_LINT_SCOPE ?= --git-base "$(BASE_REF)"

lint-migrations: ## Atlas migrate lint (destructive + incompatible), needs Docker
	@$(RUN) atlas-lint db-no-destructive-migration,db-backward-compatible-migration --dir $(API_DIR) -- atlas migrate lint --env local $(ATLAS_LINT_SCOPE)

squawk: ## Postgres locking checks on migrations
	@$(RUN) squawk db-safe-locking -- scripts/harness/squawk.sh

migrations-append-only: ## Migrations on BASE_REF are never edited or deleted (migration-gate)
	@$(RUN) migrations db-append-only-migrations -- env BASE_REF="$(BASE_REF)" scripts/harness/migrations-append-only.sh

gen-schema-doc: ## Regenerate docs/generated/db-schema.md (needs Docker)
	@scripts/gen-schema-doc.sh

schema-doc-fresh:
	@$(RUN) schema-doc db-schema-doc-fresh -- scripts/gen-schema-doc.sh --check

test-integration: ## Integration tests against the worktree's Postgres (.env.local)
	@$(WITH_ENV) unset "$${!OTEL_@}"; $(RUN) go-integration go-tests-pass --dir $(API_DIR) -- go test -race -count=1 -tags=integration ./...

# CI's generator probe (ci.yml): a scaffold whose tests compile but skip or vanish would pass test-integration.
test-scaffold: ## After make new-domain NAME=<name>: each of its scaffolded tests ran and passed, none skipped (NFR-B1)
	@$(WITH_ENV) unset "$${!OTEL_@}"; scripts/harness/scaffold-tests.sh $(NAME)

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

##@ Secrets and dependencies (Trivy: trivy.yaml, scripts/harness/trivy.sh; Swift lockfiles: .bin/harness)
.PHONY: secrets vuln-deps swift-resolved
# The sensor id is the rule id: budgets.txt row repo-no-secrets (NFR-H9). $(TOOLS_STAMP) runs install-tools.sh
# (Trivy too) once tool-versions.env changes, before the scan, not racing it under -j4.
secrets: $(TOOLS_STAMP) ## Offline secret scan of the git-known files (in make check, only the changed ones)
	@$(RUN) repo-no-secrets repo-no-secrets -- scripts/harness/trivy.sh secret

vuln-deps: $(TOOLS_STAMP) ## Known vulns in pnpm-lock.yaml, every go.mod and Package.resolved (fetches the Trivy DB into .cache/trivy)
	@$(RUN) trivy-vuln deps-no-known-vulns -- scripts/harness/trivy.sh vuln

swift-resolved: $(HARNESS) ## A Swift package with remote dependencies has its Package.resolved in git
	@$(RUN) swift-resolved deps-swift-resolved -- $(HARNESS) swift-resolved .

##@ CI and harness integrity (.github/, protected harness files)
.PHONY: lint-ci layout-check harness-integrity orphans harness-sync-check adoption-probe commits specs specs-tested
lint-ci: ## actionlint (+ shellcheck on run: blocks) on .github/workflows
	@$(RUN) actionlint ci-workflows-valid -- actionlint -no-color

layout-check: $(HARNESS) ## scripts/harness/layout.env parses, its paths exist and LAYER_GLOB matches a Go package
	@$(RUN) layout-check harness-layout-valid -- $(HARNESS) layout-check .

harness-integrity: $(HARNESS) ## Added suppressions carry a reason; harness configs are not loosened (vs BASE_REF)
	@$(RUN) integrity harness-suppression-reason,harness-config-not-loosened -- env BASE_REF="$(BASE_REF)" $(HARNESS) integrity-check .

orphans: $(HARNESS) ## Every file under scripts/ is named by a target, workflow, hook, skill, agent or guide; every make target is in make help or called
	@$(RUN) orphans repo-no-orphan-scripts -- $(HARNESS) orphans .

# Reads the network (fetches HARNESS_UPSTREAM), so not in check or check-all: drift.yml runs it weekly. The URL
# comes from scripts/tool-versions.env; a command-line HARNESS_UPSTREAM=<url> reaches the script by make's export.
harness-sync-check: ## Harness files changed upstream since .harness-version and not taken here (HARNESS_UPSTREAM in tool-versions.env or on the command line; empty: the template); applies nothing
	@$(RUN) harness-sync harness-upstream-in-sync -- scripts/harness/sync-check.sh

adoption-probe: ## README (b) followed on a root-go.mod repo ends green: setup, check, harness-selftest (minutes; CI's adoption-probe job)
	@$(RUN) adoption-probe harness-adoption-probe -- scripts/harness/adoption-probe.sh

# PR_TITLE reaches these from the pull_request_target job as an env var (ADR 0011): never write $(PR_TITLE)
# in a recipe, make would expand a title like `$(shell …)` in a job that holds the base branch's checkout.
commits: ## Commit subjects (and PR_TITLE) since BASE_REF follow Conventional Commits
	@$(RUN) commits git-conventional-commits -- env BASE_REF="$(BASE_REF)" scripts/harness/check-commits.sh

specs: ## spec-kit specs/ are well formed; feat changes to PRODUCT_GLOBS (layout.env) come with a spec (vs BASE_REF)
	@$(RUN) specs specs-structure,specs-required-for-features -- env BASE_REF="$(BASE_REF)" scripts/harness/check-specs.sh

# In make check, runs only when specs/, a test file or another harness file changed (rules.md#harness-scope).
specs-tested: $(HARNESS) ## Every acceptance scenario in specs/ is named by a Go, TS or Swift test, or marked (untested: <reason>)
	@if scripts/harness/unchanged.sh 'specs/*' '*_test.go' '*.test.*' '*.spec.*' '*.e2e.*' '*Tests.swift'; then \
	  echo "$@: skipped, neither specs/ nor a test file changed (make check-all runs it)"; \
	else $(RUN) specs-tested specs-scenarios-tested -- $(HARNESS) specs-tested .; fi

##@ Docs
.PHONY: docs-links references-pinned docs-quality-shape docs-quality fixture-claims rule-lists
docs-links: $(HARNESS) ## Relative links and anchors in Markdown resolve; every AGENTS.md has a CLAUDE.md importing it
	@$(RUN) check-links docs-links-resolve,docs-claude-imports-agents -- $(HARNESS) check-links .

references-pinned: $(HARNESS) ## Every docs/references/*-llms.txt names the tool version the repo pins
	@$(RUN) references-pinned docs-references-pinned -- $(HARNESS) references-pinned .

# Deterministic (reads no clock): in check-all and the CI generator probe (ADR 0028).
docs-quality-shape: $(HARNESS) ## docs/QUALITY.md has one well-formed row per domain, graded per layer
	@$(RUN) quality-shape docs-quality-rows-well-formed,docs-quality-rows-match-domains -- $(HARNESS) quality-rows -no-clock .

# Reads the clock, and only the domain's reviewer can re-grade a row, so a PR's author could not fix it: the
# weekly drift sweep runs it, never check or check-all (ADR 0028).
docs-quality: $(HARNESS) ## docs/QUALITY.md rows were graded in the last 30 days (drift only)
	@$(RUN) quality-fresh docs-quality-row-fresh -- $(HARNESS) quality-rows .

# In make check, runs only when rules.md, a fixture or another harness file changed (rules.md#harness-scope).
fixture-claims: $(HARNESS) ## Every rules.md Fixture line names fixtures that exist and test that rule, or none (<reason>)
	@if scripts/harness/unchanged.sh docs/harness/rules.md 'scripts/harness/fixtures/*'; then \
	  echo "$@: skipped, neither rules.md nor a fixture changed (make check-all runs it)"; \
	else $(RUN) fixture-claims rules-fixture-claims -- $(HARNESS) fixture-claims .; fi

# In make check, runs only when rules.md or a harness file changed: the review agents it reads are harness files.
rule-lists: $(HARNESS) ## The review agents' Skip lists name only rules.md ids, and every rule of a section they skip
	@if scripts/harness/unchanged.sh docs/harness/rules.md; then \
	  echo "$@: skipped, neither rules.md nor a harness file changed (make check-all runs it)"; \
	else $(RUN) rule-lists harness-rule-lists-match -- $(HARNESS) rule-lists .; fi

##@ Generation
.PHONY: gen gen-check gen-sqlc gen-atlas-sum gen-oapi gen-api-client
gen: $(call stacked,gen-oapi gen-api-client gen-sqlc gen-atlas-sum) ## Regenerate all generated code (then commit it)

gen-oapi: # one strict server per OpenAPI tag: every $(DOMAINS_DIR)/**/oapi-codegen.yaml
	@for cfg in $$(find $(DOMAINS_DIR) -name oapi-codegen.yaml | sort); do \
	  (cd $$(dirname $$cfg) && oapi-codegen -config oapi-codegen.yaml $(CURDIR)/$(SPEC)) || exit 1; \
	done

gen-api-client: # orval: TanStack Query hooks + zod schemas into $(API_CLIENT_DIR)/src/gen
	@cd $(API_CLIENT_DIR) && node_modules/.bin/orval --config orval.config.ts >/dev/null

gen-sqlc:
	@cd $(API_DIR) && if grep -q -- '- engine:' sqlc.yaml; then sqlc generate; fi

gen-atlas-sum:
	@cd $(API_DIR) && atlas migrate hash --env local

gen-check: ## gen + fail if anything changed (generated code must be committed and untouched)
	@if scripts/harness/gen-check.sh --unchanged; then \
	  echo "$@: skipped, generator inputs and outputs unchanged since the last green run (make check-all runs it)"; \
	else $(RUN) gen-check gen-up-to-date -- scripts/harness/gen-check.sh; fi
