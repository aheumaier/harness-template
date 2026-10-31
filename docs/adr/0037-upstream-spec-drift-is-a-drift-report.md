# ADR 0037: Cross-repo contracts: vendored upstream specs are a weekly drift report, consumers are named on a break

Status: accepted · 2026-10-30 · amends [ADR 0030](0030-one-layout-file-for-every-sensor.md)

## Context
Exec plan 0003 (G14, P5.2): a product often consumes another team's API through a vendored copy of its
OpenAPI spec (7ed-emergencies vendors five, under `api/upstream/`, with the upstream commit in a `COMMIT`
file), and other repos vendor its own contract. `api-no-breaking-change` guards only this repo's spec against
its own merge base. Nothing notices when an upstream ships a breaking change after the copy was taken: code
generated from the copy keeps compiling until calls fail. And the `breaking-change-approved` label on this
repo's contract stands for "clients migrate", without saying which repos those clients live in. Core belief 3
and [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) ask a new rule for an incident or its own ADR;
this is that ADR, so `api-upstream-no-breaking-change` is a team convention.

## Decision
- **Declared in layout.env** (amends ADR 0030): two optional keys, the only ones that may be empty, so a
  `layout.env` without them (a base branch's, an older product's) still reads, as with `STACK`
  ([ADR 0033](0033-stack-profiles.md)). Both parsers (`layout.sh`, `layout.go`) know them; neither takes them
  from the environment, where CI's setup action exports every key.
  - `UPSTREAM_SPECS`: each vendored spec as `<vendored spec>@<source repo>@<spec path in the source>@<revision>`.
    The source is a host path (`github.com/acme/rooms`, fetched as `https://…`) or an absolute local repo path,
    the latter for tests and fixtures only: CI has no such repo, so a committed one fails the weekly run with
    exit 2. `:` separates list entries, so no scheme fits, and the layout charset stays as it is. The revision
    is the commit the copy was taken at: information for the person who refreshes the copy, never read by the
    check (see Check). This widens ADR 0030: `layout.env` said where the code lives and now also carries
    version data, a value that changes on every refresh.
  - Why the revision sits here despite [ADR 0026](0026-template-sync-is-a-drift-report.md), which kept
    `.harness-version`'s marker out of `tool-versions.env` because a value that changes every sync does not
    belong next to configuration written once: the plan asks one file to declare where every input lives, and
    an entry without its revision would say where the copy came from but not from when. The revision is read
    by no sensor, so a stale one misleads a person but changes no verdict. The price stands: refreshing a
    copy edits a protected file and needs the `harness-change-approved` label each time. TD-117 tracks
    moving the revision to an unprotected per-copy file (7ed's `COMMIT`), or checking it.
  - `CONTRACT_CONSUMERS`: repos that vendor this repo's contract, names for people (not checked).
- **Check**: `make upstream-drift` (`scripts/harness/upstream-drift.sh`) fetches each distinct source once,
  shallow, into a throwaway bare repo, and runs `oasdiff breaking --fail-on ERR` with the vendored copy as base
  and the source's spec at head as revision. A spec missing at head is drift too. Per entry it reports or
  skips; the run exits 1 on drift and 2 when any entry cannot be checked (malformed, missing copy, refused
  source or path, failed fetch, external `$ref` in either spec, a spec oasdiff cannot load), after checking
  the others. It compares against head, not against the pinned revision: drift since the copy is the
  question, and the vendored file is what the product builds from. Whether the copy still equals the source
  at its revision is not checked (TD-117): a shallow fetch has only head.
- **The vendored copy is not protected**: it is product code, refreshed in ordinary PRs. A PR that edits the
  copy to hide drift (say, removes the path the upstream dropped) also changes what the client code is
  generated from, so the generated client loses that call and the code using it no longer builds; and the
  next refresh from the source brings the truth back. That is why the copy needs no protection of its own.
- **Read-only and bounded**: the fetch is the bounded one of [ADR 0026](0026-template-sync-is-a-drift-report.md)
  (no prompt, low-speed abort, 120 s), narrowed to `GIT_ALLOW_PROTOCOL=file:https` (no ssh: every source is
  turned into an https URL or is a local path) and `--depth 1`. Anonymous in CI; locally your git credential
  helpers apply. Before any fetch, an entry is refused for a vendored path that is absolute or has `..`, a
  source starting with `-`, not shaped like a host path or an absolute path, or with `..`, a spec path that
  is absolute or has a `.`, `..` or empty segment, and a revision that is not hex. `make layout-check` (in
  `make check`, `layout.go`) refuses the same entries, and also a missing vendored copy;
  `apps/api/tools/harness/testdata/upstream-spec-vectors.txt` holds the two to the same rule.
- **Internal `$ref`s only**: oasdiff follows an external `$ref`, so both specs of an entry, and this repo's
  contract in `contract-check.sh`, must keep every `$ref` internal (`#/...`). `.bin/harness spec-refs` parses
  the YAML or JSON rather than grepping lines, so a quoted or spaced key, a value on the next line or a YAML
  alias cannot hide one.
- **A drift report, never a gate**: it reads the network, so it is in neither `make check` nor `check-all`
  (NFR-R1); `drift.yml` runs it weekly and its findings go to the `drift` issue (A18), each in a code fence
  longer than any backtick run in it, so upstream text renders as nothing but text. An empty
  `UPSTREAM_SPECS` (the template) exits 0 without any fetch.
- **Consumers on a break**: `contract-check.sh` prints `CONTRACT_CONSUMERS` when the contract breaks, read
  from the `layout.env` next to the script, so in contract-gate the base branch's: a PR cannot empty the list
  to hide a consumer. The list names the repos the approval must cover: the rule's remediation asks the PR to
  describe each named repo's migration before a code owner adds the label.

## Consequences
- A product with private upstreams (7ed-emergencies) gets exit 2 from the weekly drift job by design, like a
  private template under ADR 0026; it runs `make upstream-drift` locally with its own git credentials. A
  read-only token step is TD-115.
- A spec oasdiff cannot load (7ed's `ums.json`: `bad data in "#/components/schemas/ScimUserResponse"
  (expecting ref to response object)`) is a standing exit 2 for that entry until the product fixes the copy or drops the entry; the others still report.
- The fetch block is copied from `sync-check.sh` (TD-116), and the oasdiff call from `contract-check.sh`
  (TD-118).
- Consumers are only named. Whether they really vendor the contract, and at which revision, stays with them;
  their own `UPSTREAM_SPECS` and weekly drift run are what notices this repo's breaks on their side.
