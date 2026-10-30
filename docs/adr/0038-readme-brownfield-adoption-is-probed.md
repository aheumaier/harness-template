# ADR 0038: README (b) brownfield adoption is probed by a required check (amends 0009)

Status: proposed · 2026-10-30

## Context
README.md "(b) Brownfield project" is the procedure for adopting the harness into an existing repo. It is prose
that names the harness's files, paths, make targets, required checks and fixtures, and nearly every harness PR
can make one of those names wrong. Nothing noticed until an adopter followed it and got stuck. The harness
already proves its rules reject their fixtures ([ADR 0009](0009-harness-selftest.md)) and reads the product
layout from one file ([ADR 0030](0030-one-layout-file-for-every-sensor.md), with the stack groups of
[ADR 0033](0033-stack-profiles.md)), so the procedure can be replayed mechanically. Exec plan 0003, slice P4.10.

## Decision
- **README (b) is a procedure that a probe replays.** `scripts/harness/adoption-probe.sh` (`make adoption-probe`)
  commits a minimal brownfield repo (a root `go.mod`, `internal/<domain>/<layer>`, an OpenAPI contract, no db,
  no web) into a temp git repo, follows steps 1–7 on it with this checkout's harness, and ends with
  `HARNESS_STACK=go make setup`, `make check` and `make harness-selftest` green there. It does only the hand
  edits the README asks for; each must change its file. Before any heavy step it checks that README (b) step 3
  names exactly the paths it copies or skips and step 6's probe bullet exactly the paths it drops. A mismatch,
  a no-op hand edit or a failed step is a finding at that step's README line (rule `harness-adoption-probe`).
- **The brownfield repo is a protected skeleton**: `scripts/templates/adoption-probe/` holds the repo, the
  `layout.env` step 5 writes, the `drop.txt` paths step 6 drops and the five fixtures step 6 rewrites. It is
  `config` in `protected-paths.txt` and owned in `CODEOWNERS`, so loosening the probe's inputs needs the
  `harness-change-approved` label. Its files carry a `.tmpl` suffix that only hides them from the repo-wide
  sensors; nothing but the probe reads them.
- **A required CI job, not a drift report.** CI's `adoption-probe` job runs beside `check-all` and is a
  required check, held to the `check-all` budget (NFR-H7). Unlike template sync, which reports drift weekly
  ([ADR 0026](0026-template-sync-is-a-drift-report.md)), README (b) breaks on the harness PR that causes it, and
  that PR is the one that should fix it.
- **The probe and its skeleton are template-only.** Step 6 tells adopters to drop the probe like a rule (its
  script, fixture, tests, rules.md entry, make target, CI job, required check, budget and the skeleton's
  protected-paths and CODEOWNERS lines) and to decline its files in `.harness-version` so template sync stops
  offering them. The probe does this itself, so the instruction is probed too.

## Consequences
- Every harness PR pays a CI job of a few minutes, in parallel with `check-all`.
- One layout is probed: a root module on the go stack. A repo with its own Makefile, workflows or Claude Code
  setup takes README merge paths that the probe never walks, and README text outside code spans is not
  compared.
- The skeleton drops eleven bats files whose tests assume the template layout instead of porting them, and
  replaces five template fixtures with its own copies (tech-debt tracker TD-99, TD-100).
- To follow README (b) literally, the probe also drops `make new-domain`, its skill and CI's `generator-probe`
  job (tracker TD-67), so the template's generator is never exercised in a brownfield layout.
- Amends [ADR 0009](0009-harness-selftest.md): a rule may be proven by a probe of a whole procedure, besides its
  fixtures.
