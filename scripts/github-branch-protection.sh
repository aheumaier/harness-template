#!/usr/bin/env bash
# Configure branch protection for main with the GitHub CLI. Run by a repository admin, once, after the
# first push (the required checks must have reported at least once to be selectable).
#
#   scripts/github-branch-protection.sh              # dry run: prints the request, changes nothing
#   CONFIRM=1 scripts/github-branch-protection.sh    # applies it (needs `gh auth login` with admin rights)
#
# GH_REPO (owner/name) defaults to the current gh repository; BRANCH defaults to main. Required checks are
# the job names in .github/workflows/{ci,contract-gate,harness-integrity}.yml; migration-gate is
# path-filtered, so it is not required (its sensors also run in check-all). See docs/harness/ci.md.
# enforce_admins is false on purpose: with a single code owner, GitHub forbids approving your own PR, so
# the admin must be able to merge their own reviewed changes. Everyone else needs a code-owner approval.
set -euo pipefail

BRANCH="${BRANCH:-main}"
REQUIRED_CHECKS=(check-all harness-selftest generator-probe adoption-probe api-no-breaking-change harness-integrity pr-conventions)

# app_id 15368 = GitHub Actions: only a workflow job can satisfy a required check, not any app posting
# a status with the same name.
checks="$(printf '{ "context": "%s", "app_id": 15368 },' "${REQUIRED_CHECKS[@]}")"
body="$(
  cat <<JSON
{
  "required_status_checks": { "strict": true, "checks": [${checks%,}] },
  "enforce_admins": false,
  "required_pull_request_reviews": {
    "required_approving_review_count": 1,
    "require_code_owner_reviews": true,
    "dismiss_stale_reviews": true
  },
  "restrictions": null,
  "required_linear_history": true,
  "allow_force_pushes": false,
  "allow_deletions": false,
  "required_conversation_resolution": true
}
JSON
)"

if [[ "${CONFIRM:-0}" != 1 ]]; then
  echo "Dry run. PUT repos/${GH_REPO:-<current repo>}/branches/$BRANCH/protection with:"
  echo "$body"
  echo "Re-run with CONFIRM=1 to apply."
  exit 0
fi

repo="${GH_REPO:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"
printf '%s\n' "$body" | gh api -X PUT "repos/$repo/branches/$BRANCH/protection" \
  -H "Accept: application/vnd.github+json" --input - >/dev/null
echo "Branch protection applied to $repo:$BRANCH."
