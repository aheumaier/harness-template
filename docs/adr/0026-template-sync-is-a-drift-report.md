# ADR 0026: Template sync is a weekly drift report that never applies anything

Status: proposed · 2026-10-29

## Context
Exec plan 0003 (G7, P4.3): a product cut from this template keeps the harness of the day it was cut.
Böckeler (*Harness engineering for coding agent users*) names it: harness templates drift from upstream.
There is no marker of which template commit a product has, and no way to see what changed since. Core belief
3 and [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) ask a new rule for an incident or its own
ADR; this is that ADR.

## Decision
- **Marker**: `.harness-version` at the root holds one 40-hex upstream commit (after `#` comment lines): the
  template commit whose harness files the repo last took, which for a new product is the commit it was copied
  from. The template has no release tags, so a commit, not a tag. It is a protected `config` file: bumping it
  without porting would turn the report green, so a bump goes in the harness PR that ports the changes, with
  the `harness-change-approved` label. A `# declined <path>: <why>` comment line records a harness file the
  product chose not to take; the check lists it as declined and does not count it.
- **Upstream**: `HARNESS_UPSTREAM` in `scripts/tool-versions.env`, or `make harness-sync-check
  HARNESS_UPSTREAM=<url>` for one run. Empty means "this repository is the template": the check prints a
  one-line notice and exits 0, so the template makes no weekly fetch of its own origin. **The rename is the
  moment a repo stops being the template**: `make rename` writes `HARNESS_UPSTREAM=https://<template module>`
  after its rewrite pass, exempt from the rewrite and from the leftover report. A brownfield adoption, where
  rename may not run, sets it by hand. Keeping the URL in `.harness-version` next to the marker was
  considered and rejected: the URL is configuration that `make rename` writes once and make reads like every
  other value in `tool-versions.env`, while the marker changes in every sync PR.
- **Check**: `make harness-sync-check` (`scripts/harness/sync-check.sh`) fetches upstream's default branch
  into a throwaway bare repo and compares three trees: the marker commit, upstream's tip and the working tree.
  It lists each harness file that changed upstream since the marker and whose upstream mode and content differ
  from the working tree's: "changed upstream", "added upstream" or "deleted upstream", plus "changed here too
  (merge by hand)", "present here too (merge by hand)" or "missing here" for the local side. A file already
  taken is not listed, so a marker older than the real cut over-reports at most; a marker newer than the real
  cut under-reports, because the files changed between the two count as seen. It never applies anything.
- **Sync surface: protected paths only.** Harness files are the globs of `protected-paths.txt`, the repo's
  and upstream's, minus `.harness-version` itself (each side's marker is its own). What follows:
  `scripts/setup-worktree.sh` and the other unprotected scripts and docs are not compared; the template's ADRs
  do not come along (a product keeps its own record); a protected file the product adapted on purpose (its
  `tool-versions.env`, which holds the product's own `HARNESS_UPSTREAM`; a CODEOWNERS with its own owners)
  shows "changed here too" after every upstream change to it, until the product declines it. Exec plan 0003
  P4.9 narrowed `.github/*` to the harness's own files, which shrank the surface with it
  ([ADR 0035](0035-harness-files-under-github-are-named.md)).
- **Read-only and bounded**: the fetch is anonymous (no credential prompt, ssh in batch mode with a 15 s
  connect timeout, a transfer slower than 1 KB/s for 30 s aborts, and the whole fetch has 120 s where
  `timeout(1)` exists). `GIT_ALLOW_PROTOCOL=file:https:ssh`: `ext::` would run a command, and `git://` and
  `http://` are unauthenticated plain text a network attacker could answer. A URL with embedded credentials
  is refused (exit 2) and never printed: use a credential helper or a deploy key.
- **Fails closed** (exit 2): a missing or malformed marker, a refused URL, a failed or timed-out fetch, a
  marker commit that is not in upstream's history (a wrong `HARNESS_UPSTREAM`, or a marker from another
  history), or any unexpected error. Exit 1 lists the files behind; make reports either as its exit 2.
- **A drift report, never a gate**: it reads the network, so it is in neither `make check` nor
  `check-all` (NFR-R1). `drift.yml` runs it weekly; its findings go to the `drift` issue.

## Consequences
- The template's own drift run is a no-op notice; every product, renamed or adopted, compares itself with
  the template's URL.
- Each product decides what to port; the report names files, not hunks. Porting is a normal harness PR.
- In a product, `scripts/tool-versions.env` always shows "changed here too (merge by hand)" when upstream
  changes it, since the product's `HARNESS_UPSTREAM` line differs from the template's empty one.
- The drift job's fetch is anonymous: a private upstream exits 2 there every week by design. Run the check
  locally with your own credentials; an optional read-only token step for drift.yml is tracked in the
  tech-debt tracker.
