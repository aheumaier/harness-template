# ADR 0040: CI agents may authenticate with a Claude subscription token (amends 0019)

Status: proposed · 2026-10-30 · amends [ADR 0019](0019-inferential-sensors-are-advisory.md)

## Context
The CI agents (`pr-review.yml`; the scheduled reviews and `agent-gc` in `drift.yml`) read the
`ANTHROPIC_API_KEY` repository secret, which ADR 0019 asks to be a dedicated Anthropic workspace key with a hard
spend limit. The owner has a Claude subscription and no such key, and set a `CLAUDE_CODE_OAUTH_TOKEN` secret
instead, made with `claude setup-token`. Claude Code reads that variable, but not under `--bare`, which every
CI agent ran with: "Bare mode does not read `CLAUDE_CODE_OAUTH_TOKEN`"
([authentication](https://code.claude.com/docs/en/authentication)). Naming the token `ANTHROPIC_API_KEY` does
not work either: the CLI sends that variable as an API key, which the API refuses for an OAuth token.

`--bare` also confined the agent: it skips hooks, plugins, skills, auto memory and CLAUDE.md discovery
([headless](https://code.claude.com/docs/en/headless)). Without it, CLAUDE.md and a git status snapshot of
the checkout (in `pr-review.yml`, the PR's base commit) would enter the agent's system prompt, above the
untrusted-data preamble.

## Decision
- **Either secret runs the agents.** The gate passes when `ANTHROPIC_API_KEY` or `CLAUDE_CODE_OAUTH_TOKEN` is
  set; the agent step gets both. When both are set, the CLI uses the API key ("the key is always used when
  present" under `-p`), so a workspace key still wins where there is one.
- **No `--bare`; its confinement is restored piece by piece**, all available in the pinned 2.1.300:
  `--restricted` (already set: ignores user, project and local settings files, so no repository hook,
  `apiKeyHelper`, `env` or `enabledPlugins` loads), `--disable-slash-commands` (no skills or commands),
  `CLAUDE_CODE_DISABLE_CLAUDE_MDS=1` (no CLAUDE.md), `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1`,
  `CLAUDE_CODE_DISABLE_GIT_INSTRUCTIONS=1` (no git status snapshot), `--strict-mcp-config` and the
  `--settings` file's `disableAllHooks: true` (both already set). `--tools Read,Grep,Glob` leaves out the
  Agent tool, so no project subagent runs. `scripts/test/pr-review.bats` fails on a `--bare` in any
  `.bin/claude -p` step and checks the new flags and variables, each on a line of its own, in every one.
- **The leak screen covers both secrets**: each one that is set, verbatim, plus `sk-ant-` in any case, which
  is assumed to also prefix a `setup-token` token. It fails closed only when neither is set.

## Consequences
- **A personal credential is exposed to write-access collaborators.** The trust boundary is ADR 0019's: anyone
  who can push a branch can edit the workflow to read the secret. The token stands for the owner's
  subscription, not a workspace key an admin can scope and revoke: only the owner can revoke it, and a stolen
  token makes model requests on the subscription from anywhere until it is revoked or expires, with no spend
  limit to cap that (it "can only make model requests", so no Remote Control or claude.ai connectors). Where
  to revoke one and whether that also ends the owner's other sessions is not in the docs and unverified here;
  rotating is `claude setup-token` again, then replacing the secret. Anthropic's GitHub Actions docs recommend
  an API key for shared secrets for this reason. The Consumer Terms restrict
  automated access to the Services; the owner accepts that question by choosing the token.
- **No spend cap.** `--max-budget-usd` checks a client-side estimate, and the docs do not say it applies under
  a subscription; `--max-turns` and `REVIEW_MAX_MINUTES` are the hard bounds. CI shares the owner's session and
  weekly usage limits with interactive use: a burst of PRs can use them up, and a limit reached ends as an API
  error, which is a notice, not a red job.
- **The token expires** a year after `claude setup-token` ("a one-year OAuth token",
  [authentication](https://code.claude.com/docs/en/authentication#generate-a-long-lived-token)): this one,
  made about 2026-10-30, expires about 2027-10-30 ([TD-38](../exec-plans/tech-debt-tracker.md) holds the renewal). An expired or revoked
  token ends every agent run as an API error, a skip notice whose text carries the error, not a red job; the
  notices are the only signal.
- **An unknown variable is silently ignored.** The CLI rejects an unknown flag (a red job) but not an unknown
  `CLAUDE_CODE_DISABLE_*` variable, so a Claude Code bump that renames one drops its guard silently; the
  note at `CLAUDE_CODE_VERSION` in `scripts/tool-versions.env` asks to re-check them on every bump.
- **The first live run is the PR that brings this ADR.** The token is already set, and on `pull_request` the
  job runs that PR's workflow with the base's `ci-agent.sh`, whose leak screen still needs `ANTHROPIC_API_KEY`:
  that one review goes red (exit 2) and its usage is lost. Confinement is tested live after it, since a dummy
  key fails authentication before any tool runs (`docs/harness/ci.md#pr-review`).
- An API key set later takes over without a change, and turns the spend limit back on only once the
  `CLAUDE_CODE_OAUTH_TOKEN` secret is deleted and the token revoked: while the secret exists, the agent step
  still gets it.
- ADR 0019's flag list now reads `--restricted --disable-slash-commands` plus the three variables, and its
  hard cost cap is the turn and minute caps under the token; `docs/harness/ci.md` and `docs/SECURITY.md` name
  both secrets.
