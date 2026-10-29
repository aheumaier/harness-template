# Core beliefs

Every rule in [docs/harness/rules.md](../harness/rules.md) and every guide should trace back to one of
these. If a proposed rule does not, question the rule.

## 1. Agent legibility first

Code is read far more often than it is written, and most readers here are agents with no memory of
yesterday. Prefer what an agent can understand from the repo alone: explicit wiring over DI magic,
generated code over reflection, one obvious place per kind of code, sensor output that names the file,
the rule and the fix. *Consequences:* `make` is the single entry point; `run.sh` prints the remediation;
each domain has the same four layers and its own short AGENTS.md.

## 2. Parse at boundaries

Untrusted data is parsed into typed values where it enters (HTTP request, HTTP response in the client,
environment, database row) and nothing inside re-validates it. *Consequences:* the generated strict server
validates requests against the contract; zod parses responses in the web client; `platform/config` fails
fast on bad environment; errors leave as RFC 9457 problems.

## 3. Enforce invariants, not implementations

Rules constrain what must always be true (dependency direction, no root contexts, no breaking contract
change, no destructive migration) and leave the how open. Every invariant has a deterministic sensor and a
fixture proving the sensor fires. *Consequences:* no style rules beyond formatters; new rules are earned by
real mistakes, not anticipated, unless an ADR adopts them as `team-convention`, as
[ADR 0024](../adr/0024-size-and-complexity-limits-are-maintainability-sensors.md) (proposed) does for
function length, file length, complexity and parameter count, which are maintainability sensors, not style.

## 4. Prefer boring, in-repo-reasonable dependencies

Choose tools and libraries whose behaviour can be understood from the repo, its docs and its tests: the
standard library, widely used single-purpose tools, generated code that is committed. Avoid frameworks
whose behaviour lives in conventions or runtime magic. *Consequences:* `net/http` and `slog` over a web
framework; sqlc over an ORM; versions pinned and installed into `.bin/`.

## 5. Repo is the system of record

Anything not in the repository does not exist for an agent: decisions, plans, debt, quality and schema
all live here as text and are kept current by sensors where possible. *Consequences:* ADRs, exec plans, feature specs,
`docs/QUALITY.md`, `docs/exec-plans/tech-debt-tracker.md` and the generated schema doc; Slack threads and
tribal knowledge are copied in or lost.
