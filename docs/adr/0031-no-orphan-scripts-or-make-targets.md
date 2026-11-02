# ADR 0031: No orphan scripts or make targets; dead Go code is gated in check-all

Status: accepted · 2026-10-29 · amended 2026-11-02 by [ADR 0045](0045-harness-scripts-and-tests-in-go.md) (subcommands need a caller)

## Context
Gap G11 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P3.6): agents add helper
scripts and make targets, then leave them behind when the caller goes. An orphan looks like a live sensor.
Someone keeps it in sync for nothing, and the next agent learns the wrong idea of how the harness runs. Dead Go
functions are the same failure in code. `go-no-dead-code` existed, but only as a weekly drift report, so a PR
could add dead code and stay green. [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) asks for an
incident or an ADR for each rule beyond its five; this is that ADR.

Measured on this tree (2026-10-29): `make deadcode-go` takes 1.0–1.8 s over `apps/api` alone and 1.4–1.5 s
with `apps/api/tools/harness` added, with no finding. `.bin/harness orphans .` takes 0.04–0.05 s and also has
no finding. `apps/api/tools/analyzers` is not clean under deadcode. It reports four false positives in
`plugin.go`: the `init` registration, `newPlugin`, `BuildAnalyzers` and `GetLoadMode`. golangci-lint's
module-plugin loader calls them, and deadcode cannot see that.

## Decision
- **One rule, `repo-no-orphan-scripts`, team-convention**, checked by `harness orphans` in the Go harness
  binary ([ADR 0012](0012-parsing-sensors-in-go.md)) as `make orphans` in `make check-all`, not in
  `make check`. Mid-change a new script or target is normally an orphan until the caller lands, so a gate
  after every edit would fire on work in progress; before a PR it must be wired up.
- **A script is referenced** when its repo path appears as a whole token in one of these referencers: the
  Makefile, a workflow or action under `.github/`, a Claude hook or `.claude/settings.json`, a skill, an agent,
  a guide, or a shell script that is itself referenced. In the executable ones (Makefile, workflow, action,
  hook, setting, shell script) only code counts: a `#` comment or make help text does not, so a commented-out
  caller keeps nothing alive. A `/` before the path counts only after `.`, `..` or a variable expansion:
  `$ROOT/scripts/x.sh`, `${ROOT}/…`, `$(CURDIR)/…` and `"$root"/…` name `scripts/x.sh`, while
  `apps/scripts/x.sh` and `.specify/scripts/x.sh` do not. A guide is Markdown outside `docs/adr/`,
  `docs/exec-plans/`, `docs/generated/`, `specs/` and `.specify/`: those record history or are generated. Go
  code, `package.json` and `compose.yaml` are not referencers. Tests (`scripts/test/`) and data files are
  reached but reference nothing: some data files list script paths without calling them
  (`protected-paths.txt` names `scripts/install-tools.sh`, `scripts/gen-schema-doc.sh` and
  `scripts/github-branch-protection.sh`), so counting them would keep those scripts alive with no caller.
- **Directory references are an allowlist of runner directories.** `scripts/test` (`bats scripts/test`) and
  `scripts/git-hooks` (`core.hooksPath scripts/git-hooks`) are the only directories a tool runs by name, so
  naming one of them in the code of an executable referencer wires up its direct children. No other directory
  counts, wherever it is named: `cd scripts/harness` runs nothing by itself. A new runner directory is a
  change to `runnerDirs` in `orphans.go` and to this list.
- **Fixtures are never referencers.** A script that only a fixture names is an orphan. This keeps the
  repo-orphan-script fixture honest: its `apply.sh` names the orphan it plants.
- **A make target is an entry point when `make help` lists it**: a header line matching its awk pattern
  `^[a-z0-9-]+:.*##` (one name, no blank before the colon). This refines the plan's "`## ` help comment": make
  help needs no blank after `##`, and the rule checks exactly what make help shows. An entry point needs no
  caller. Every other target must be named by another target's prerequisites or recipe, a workflow, an
  action, a Claude hook, setting, skill or agent, a guide or a referenced script. The target's own rule,
  comments, `.PHONY`, help text and target-specific variable lines (`check-all: export X := y`) do not count
  as callers. Only targets of the Makefile itself named `[a-z0-9][a-z0-9-]*` are checked: variable-named,
  file and double-colon targets, other names and included makefiles are skipped.
- **`deadcode-go` moves into `check-all`** and also covers `apps/api/tools/harness`. drift.yml keeps running
  it, as it does for knip. `apps/api/tools/analyzers` stays out until its plugin entry points can be declared
  ([TD-70](../exec-plans/tech-debt-tracker.md)). Both sensors are far below the 5 s per-sensor budget, so
  neither needs a row in `budgets.txt` ([ADR 0013](0013-harness-nfr-budgets.md)).

## Consequences
- A PR that drops a caller must also drop the script or target, or document it for `make help`. A data file
  stays referenced while the script that reads it names it.
- Blind spots: a target whose name is a common word (`check`, `gen`) is found in any prose, but all such
  targets are entry points today. A script that a guide names counts even when nothing runs it. These are
  accepted: the rule catches what agents forget, not a deliberate hiding place, and review covers the rest.
- Fixtures: `repo-orphan-script`, `repo-orphan-make-target`, and `go-dead-function` for `go-no-dead-code`.
