#!/usr/bin/env bash
# Runs in the repo copy: appends a rule whose Fixture line names a fixture that does not exist. Appended,
# not a copy of rules.md, which would drift from the real one.
set -euo pipefail
printf '\n### fx-uncovered\n- **Fixture:** `fixtures/fx-never-written` · Status: seed\n' >>docs/harness/rules.md
