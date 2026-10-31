# ADR 0022: Handler tests pin API responses to approved files, checked against the contract

Status: accepted · 2026-10-29

## Context
Exec plan 0003 (P2.4) asks for approved-fixture snapshots of API responses: golden JSON per operation and
status, checked against the OpenAPI schema, diffed on change, an intended change approved by `make approve`
and shown in review. Handler tests today assert a status code and a Content-Type; a refactor that renames a
field, drops a header or rewords an error passes them. The strict server generated from `api/openapi.yaml`
types what the handler returns but not what the encoded response holds, and nothing compares the encoded
response with the spec.

This adds two rules that no incident has earned yet, and a third that fences off the helper they need.
[ADR 0015](0015-anticipated-rules-and-nfr-classes-b-o.md) adopts only its five listed rules as
team-convention; any other rule needs an incident or its own ADR. This is that ADR.

## Decision
- **Opt-in pinning.** A handler test pins a response with
  `approvaltest.Verify(t, "<operationId>", rec, [approvaltest.Case("<name>")])`. `make approve` runs the unit
  tests with `APPROVE_RESPONSES=1`, which writes the files instead of comparing; the approval is that run plus
  the reviewed diff of the files in the PR. Any other non-empty value of the variable except `0` fails the
  test. Under `make approve`, two different responses for one file name in a run fail the test.
- **Format and key.** One file per response at `<package>/testdata/approved/<operationId>.<status>[.<case>].json`:
  `{"headers": {<name>: <value>}, "body": <JSON>}`. Several values of a header are joined with `", "` (wrong
  for `Set-Cookie`). A JSON body is re-encoded with sorted keys and its numbers as sent; any other body is a
  JSON string; an empty body has no `body`. The case segment is lower-case kebab-case, so two names never
  differ only in case (macOS file systems ignore it). `approvaltest.File` and `approvedcheck.go` define the
  format and point at each other.
- **Status lookup.** A file's status selects the spec response exactly, then `<n>XX`, then `default`; its
  Content-Type selects the media type exactly, then `<type>/*`, then `*/*`.
- **The two rules.** [api-responses-approved](../harness/rules.md#api-responses-approved) (sensor: the test
  itself, in `make test-go`) and [api-approved-match-contract](../harness/rules.md#api-approved-match-contract)
  (sensor: `.bin/harness approved-check`, in `make check` and run by `make approve`). Both are
  team-convention.
- **Not yet.** No completeness check (every operation × declared status has a file,
  [TD-30](../exec-plans/tech-debt-tracker.md)): the new-domain scaffold emits no approval test yet, and the
  CI generator probe would go red. Stale files that no test verifies any more go undetected
  ([TD-31](../exec-plans/tech-debt-tracker.md)). Tests wrap only the middleware their author adds, so the
  file pins the handler's response, not what production middleware makes of it
  ([TD-33](../exec-plans/tech-debt-tracker.md)). Headers the spec does not declare are not reported
  ([TD-16](../exec-plans/tech-debt-tracker.md)); array and object headers and path-item `$ref`s are not
  checked.
- **Parsers (amends [ADR 0012](0012-parsing-sensors-in-go.md)).** `apps/api/tools/harness` adds
  `github.com/santhosh-tekuri/jsonschema/v6` as its JSON Schema validator (Draft 2020-12, the OpenAPI 3.1
  dialect, formats asserted, no loader, so nothing is fetched) and `go.yaml.in/yaml/v3` as its YAML parser.
  `golang.org/x/text` is a direct dependency because the validator's messages need a `message.Printer`;
  `github.com/dlclark/regexp2` appears only in `go.sum` (the validator's module graph; not compiled in). All
  are accepted.
- **Not kin-openapi.** Its response validator (`openapi3filter`) targets OpenAPI 3.0 and its JSON Schema
  subset, while the spec is 3.1; and it would add a dependency to the production module `apps/api` for a
  test-time check.
- **Not the zod schemas.** `packages/api-client`'s generated zod schemas validate bodies only: they do not see
  headers, and they do not assert every string format the spec declares.
- **The helper lives in `apps/api/internal/platform/approvaltest`.** It is test-only, but a domain's `api`
  tests must import it: `go-arch-lint` lets `api` depend only on `model`, `service`, `api` and `platform`, and
  `domainiso` forbids importing another domain's `api` or `store`, so every other location breaks one of them.
  A third rule, [go-test-helpers-only-in-tests](../harness/rules.md#go-test-helpers-only-in-tests)
  (depguard, `$all` minus `$test`), keeps it out of production code. Because it decides what the first rule
  enforces, the package is a harness file (`protected-paths.txt`, CODEOWNERS).

## Consequences
- A change to a pinned response fails `make check` until `make approve` rewrites the file, and the PR shows
  the old and new response side by side.
- A file that pins a response the spec forbids fails `make check`, so the spec and the approved files cannot
  drift apart unnoticed.
- `approved-check` fails to run (exit 2) when no approved file exists: the template ships `getHealth`'s, so
  none means the layout or the pathspec drifted.
- Only operations whose tests opt in are pinned (today `getHealth`), until TD-30 is paid.
- Four more modules in the harness module's `go.sum` (three compiled in), scanned by `deps-no-known-vulns`
  like the others.
