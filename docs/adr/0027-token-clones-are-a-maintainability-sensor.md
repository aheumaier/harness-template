# ADR 0027: Token clones are a maintainability sensor (extends 0024)

Status: accepted · 2026-10-29

## Context
Gap G11 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P3.7): agents replicate
the code they see, so a pasted block spreads, and a fix lands in one copy and misses the others. The plan asks
for a clone sensor in `make check-all` for Go and TypeScript. [ADR 0024](0024-size-and-complexity-limits-are-maintainability-sensors.md)
already treats size and complexity limits as maintainability sensors: measured thresholds that only go down.
Copy-paste fits that frame. Whether code is a token clone is something a machine can decide. Whether the same
concept is written twice in different code is a judgement, and review keeps it.

Two tools were measured on this tree (2026-10-29):

- **Go: golangci-lint's `dupl`**, already in the pinned custom golangci build. It counts syntax-tree tokens
  and compares the files of one package only. Warm `golangci-lint run` over `apps/api` took 0.69 s / 1.09 s
  with `dupl` and 0.74 s / 1.09 s without it, in two rounds of five runs each on a loaded machine. That is
  within noise, so adding the linter costs nothing measurable ([ADR 0013](0013-harness-nfr-budgets.md)).
- **TS/JS: jscpd 5.4.1**, one Rust binary for all formats. A full scan of `apps/` and `packages/` takes
  10–100 ms.

## Decision
- **One rule, `repo-no-clones`, team-convention.** Tests are included and generated code is excluded. There is
  no baseline and no ignore file for existing clones. A clone found today is fixed today, or it sets the
  threshold.
- **Go clones go red in `make check`, TS clones only in `make check-all`.** `dupl` runs inside `lint-go`, so it
  shares the scoping of `make check` (changed packages plus their importers, [ADR 0014](0014-make-check-scoped-to-the-change.md)).
  This deviates from the plan's "in check-all". Running `dupl` separately in check-all would mean a second
  golangci pass over the whole module. Inside `lint-go` the cost is zero and the agent hears about the clone
  while the change is still small. jscpd runs as `make clones-web` in `check-all`, as the plan says, over all JS
  workspaces at once. The two stacks are therefore asymmetric: a Go clone fails the next `make check`, while a
  TS clone fails only before the PR. jscpd is fast enough to move into `check` later. Doing so would be a
  harness PR that also scopes it.
- **Thresholds are measured after the cheap fixes, only go down, and never use a baseline.** Before measuring,
  the clones that one small edit removes were removed:
  - the `make new-domain` test template now asserts through a `t.Helper()` function;
  - `apps/web` renders through one `AppProviders` module, used by `main.tsx` and the test helper.

  The Go threshold was then measured with a scaffolded domain in the tree, because every product will have one.
  `dupl` 26 is the strictest value that passes; 25 trips `internal/platform/problem/problem_test.go`. jscpd
  `minTokens` 16 is the strictest that passes; 15 trips `apps/web/e2e/global-setup.test.ts`.

  At 16 tokens, two identical import lines already count as a clone. The protected `scripts/harness/jscpd.json`
  therefore skips import and re-export statements with an `ignorePattern`, and does not raise the threshold.
  Lowering a threshold is a harness PR, made once the clone that sets it is gone. Raising one needs an ADR that
  supersedes this one.
- **jscpd reads only the protected config.** `clones-web` passes `--config scripts/harness/jscpd.json`. That
  turns off jscpd's own config discovery (`.jscpd.json`, `.config/jscpd.json`, a `package.json` `"jscpd"` key), so a
  config planted elsewhere cannot widen `ignore` or raise `minTokens` (fixture
  `web-clone-stray-config`). jscpd's `jscpd:ignore-start`/`-end` comments are never allowed: the integrity check
  treats them like `@ts-ignore`. A Go clone that must stay takes `//nolint:dupl // reason: … (rule: repo-no-clones)`
  on each copy, as the size rules do.
- **Per-package blind spot, and where shared code goes.** `dupl` cannot see a block copied from one domain into
  another, the clone agents make most ([TD-52](../exec-plans/tech-debt-tracker.md)). A module-wide pass is not
  added now, because at today's threshold it would flag every second scaffolded domain. Until it exists, this
  policy decides where deduplicated code lives:
  - shared plumbing and test helpers may live in `internal/platform` (precedent: `internal/platform/approvaltest`,
    [ADR 0022](0022-approved-api-responses.md));
  - domain logic never does: `platform` imports no domain and knows no business rule;
  - one domain reuses another's behaviour through the owning domain's `service` or `model`, never by copying it
    and never through its `store` or `api` (the layer rule);
  - the `make new-domain` scaffold emits only domain-specific code: shared behaviour it needs goes to
    `platform` once, not into every domain's copy.
- **Supply chain.** jscpd is an npm package that delivers a native binary through per-platform
  `optionalDependencies` (`jscpd-darwin-arm64`, …). It has no install script, so nothing runs at install time
  and `allowBuilds` stays unchanged. It is not a `.bin` tool, so [ADR 0016](0016-tool-supply-chain.md)'s
  sha256 pin and 14-day rule for downloaded binaries do not apply. It relies on the JS controls instead:
  `pnpm-lock.yaml`'s sha512 integrity for the wrapper and every platform package, and pnpm's
  `minimumReleaseAge`. 5.4.1 was published on 2026-10-09, 20 days before adoption. A jscpd bump changes
  sensor behaviour, so it is a harness PR: re-run the fixtures and re-measure both thresholds.

## Consequences
- Copy-paste inside a Go package fails `make check`, and copy-paste anywhere in the web code fails
  `make check-all`. The finding names both copies. The remediation in [rules.md](../harness/rules.md#repo-no-clones)
  says where the one remaining copy goes.
- The thresholds are tight: two or three identical lines of logic trip them. That is intended. They were
  measured on a scaffold-sized tree and only go down. The alternative, a baseline, would preserve the clones
  this rule exists to remove.
- Cross-package Go clones stay invisible until TD-52 is paid. Review, and the placement policy above, carry
  that gap.
- A suppression for a Go clone is reviewable, like any `//nolint`. A TS clone has no exception, so it is
  always fixed in the code.
