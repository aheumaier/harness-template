# ADR 0036: Acceptance scenarios are traced to tests by a positional id

Status: proposed · 2026-10-29

## Context
Gap G13 of exec plan 0003 (`docs/exec-plans/*/0003-harness-gap-closure.md`, slice P5.1), from Nandyala and
Pani, [Engineering the harness](https://www.thoughtworks.com/insights/blog/architecture/engineering-the-harness-a-practical-pattern-for-reliable-coding-agents)
(Thoughtworks, 2026-09-29): its REVIEW step checks the tests against the acceptance criteria and sends the work
back to RED when one is missing. Here `specs-structure` and `specs-required-for-features` check that a spec
exists and is well formed, but nothing ties a spec-kit `Given/When/Then` scenario
(`.specify/templates/spec-template.md`, the numbered list under `**Acceptance Scenarios**:` of each
`### User Story <n>`) to a test. An agent ticks off tasks, so a story can ship with half its scenarios
unproven. [ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) asks for an incident or an ADR for each
rule beyond its five; this is that ADR, as [ADR 0011](0011-conventional-commits-and-spec-kit.md) was for the
other spec rules.

## Decision
- **One rule, `specs-scenarios-tested`, team-convention**, checked by `harness specs-tested` in the Go harness
  binary ([ADR 0012](0012-parsing-sensors-in-go.md)) as `make specs-tested`. It runs in `make check` when
  `specs/`, a test file or another harness file changed ([ADR 0014](0014-make-check-scoped-to-the-change.md)),
  and always in `make check-all` and CI. It reads no network or clock (NFR-R1).
- **The id is positional and never written into the spec (A17):** `<NNN>/US<n>-AS<m>`, with `NNN` from the
  directory, `n` from the story heading (`### User Story 2 - …`, the number spec-kit's tasks call `US2`) and `m`
  the item's position in the scenario list, 1-based, whatever number the item is written with (Markdown lists
  are often all `1.`). Writing ids into the spec would change spec-kit's vendored template, a `.specify`
  harness PR. The spec is parsed with goldmark (as `docs-links-resolve` parses docs), so fenced code, HTML
  blocks and comments hold nothing. A story is a heading of level 1 to 3 starting `User Story <n>` and runs to
  the next heading of level 1 to 3; its scenarios are the items of the ordered list right after its
  `**Acceptance Scenarios**:` paragraph, nested lists being steps of one item.
- **The structure fails closed.** A spec the parser finds no story in would otherwise pass, so these are
  findings: no story at all (spec-kit's User Scenarios section is mandatory), a story heading without a number,
  of level 4 or below, or numbered twice in a spec, a story without an ordered scenario list, and an
  `**Acceptance Scenarios**:` paragraph outside a story.
- **A test names an id when the id appears in it as a whole token**, with no letter, digit, `_`, `/`, `.` or
  `-` next to it (so `001/US1-AS1-old` and `001/US1-AS1.5` name nothing). Go identifiers cannot hold `/` or
  `-`, so for Go that is a subtest title (`t.Run("001/US1-AS2 pays", …)`) or a `// spec: 001/US1-AS2` comment,
  for TS a test title or the same comment. The sensor does not parse test syntax: any whole-token occurrence in
  a test file counts, except on a line that also skips or defers a test (`.todo(`, `.skip(`, `.fixme(`,
  `xit(`, `xtest(`, `xdescribe(`, `t.Skip`), which passes a green suite while checking nothing. Test files are
  `*_test.go`, `*.test.*`, `*.spec.*` and `*.e2e.*` (the Playwright journeys) under `PRODUCT_GLOBS` and
  `WEB_GLOBS` of `layout.env` (so the pure `packages/domain`, where AGENTS.md wants domain logic unit-tested,
  counts), minus `API_CLIENT_DIR`, `GENERATED_GLOBS` and the harness files of `protected-paths.txt`. So the
  harness's own tests (`scenarios_test.go`) and fixtures cannot satisfy an adopter's ids.
- **An exemption is written into the spec:** the scenario's text ends with `(untested: <reason>)`. It stays
  visible in the spec and in review, unlike a list of ids kept elsewhere. The reason may hold one level of
  parentheses; an empty one or a placeholder (`n/a`, `na`, `tbd`, `todo`, `none`, `-`, `x`, `?`) is a finding.
- **Whether a test that names an id really checks its scenario, and whether an exemption's reason holds, is
  review's call**, advisory ([ADR 0019](0019-inferential-sensors-are-advisory.md)). The sensor proves
  traceability, not adequacy.

## Consequences
- An agent that finishes a story without a test for each scenario sees `make check` red with the ids and
  their text, and goes back to the test step without a human.
- Inserting a scenario above others renumbers them, and their tests then name the wrong scenario (a passing
  run, a wrong trace). Merged specs rarely renumber (A17); if they do, ids move into the spec. A test naming an
  id that no longer exists is not reported today.
- Blind spots, left to review: an id in a commented-out test counts, as does an id in a test that checks
  something else, and one in a test skipped other than on its own line (`t.Skip` in a helper or on the next
  line, a skipped enclosing `describe`, a build tag, a `-run` filter); a single scenario hidden in an HTML
  comment, or an ordered list split from its `**Acceptance Scenarios**:` paragraph, is not a scenario. A
  hidden or unrecognised story section, by contrast, is the sensor's finding: a spec without a story fails, and
  so does an Acceptance Scenarios paragraph outside one.
- Measured on this tree: `.bin/harness specs-tested .` takes 0.01 s with no specs and 0.03 s with one, far below the 5 s
  per-sensor budget, so it needs no row in `budgets.txt` ([ADR 0013](0013-harness-nfr-budgets.md)).
- Fixture: `specs-untested-scenario`.
