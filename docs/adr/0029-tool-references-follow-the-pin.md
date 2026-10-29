# ADR 0029: Tool references are taken from upstream at the pinned version and follow the pin

Status: proposed · 2026-10-29

## Context
Exec plan 0003 (P4.5): agents write code against oapi-codegen, sqlc, orval and Atlas from training data or
from docs that track the latest release, while the repo runs one pinned version of each. Option names and
defaults differ between versions (orval's `override.query.runtimeValidation` drives only `angular-query` at
8.45.0; sqlc v1.31 added `emit_pointers_for_null_enum_types`). A reference in the repo helps only while it
matches the pin, and nothing ties the two together. Core belief 3 and
[ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) ask a new rule for an incident or its own ADR; this
is that ADR.

## Decision
- **Files**: `docs/references/<tool>-llms.txt`, one per generator or migration tool the agent drives through
  `make gen` or migrations: oapi-codegen, sqlc, orval, Atlas. Each is taken from upstream at the pinned tag
  (tagged docs, or README and CLI help text where the tag has no docs), trimmed to what an agent needs in
  this repo, with a "how this repository uses it" section first. Not a vendored doc site.
- **Header**: in the first 20 lines, `Version: <pinned version>`, `Pin: <file> <key>` (where the repo pins
  the tool: `scripts/tool-versions.env` for Go-installed tools, `packages/api-client/package.json` for
  orval) and one `Source: <URL at that tag>` per upstream page used.
- **Sensor** [docs-references-pinned](../harness/rules.md#docs-references-pinned): `harness
  references-pinned` compares `Version` with the pin, exact string, and requires every `Source` to name
  it as a path segment or tag. It runs in `make check`, and so in CI through ci.yml (`make check-all`),
  instead of the weekly drift workflow plan 0003 named: it is offline, takes under 1 s, and so fails the
  PR that raises the pin rather than a later scheduled run.
- **Attribution**: every file carries a `License:` line (the upstream licence and holder, checked at the
  tag) next to its `Source:` URLs at that tag, and states that the text is adapted and trimmed, not the
  upstream text. That meets the attribution terms of the two licences involved (MIT, Apache-2.0); none of
  the four repositories has a NOTICE file at these tags.
- The sensor checks the header claim only. Whether the body still matches upstream is the job of whoever
  raises the pin; a body check would need the network or a vendored copy.

## Consequences
- Raising one of these pins (by hand or by a dependency bot) fails `make check` until the reference is taken again at
  the new tag, which adds a step to every such bump; the Remediation says how.
- A new tool reference needs only the header; a pin file other than `*.env` or `package.json` is reported
  and needs a sensor change.
- The header can be bumped without retaking the body; review of the bump PR catches that, not the sensor.
- Supply chain: the references are third-party text that agents read as guidance, and an upstream tag can
  be moved after the file was taken. Review of the PR that adds or retakes a file is the only control. Each
  file says that where upstream conflicts with this repository's rules or with its "How this repository
  uses it" section, the repository wins. Recording the upstream commit SHA next to the tag is TD-62.
- `docs/references/*` is a protected guide (scripts/harness/protected-paths.txt, CODEOWNERS): a change to
  what agents read as tool guidance gets owner review like the other guides.
