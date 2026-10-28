#!/usr/bin/env bash
# Runs in the repo copy: plants a fake AWS access key. It is assembled here, so no key-shaped string is in the
# repo (GitHub push protection and other scanners stay quiet). Not the AWS docs example key: Trivy allows that.
set -euo pipefail
printf '[default]\naws_access_key_id = %s%s\n' AKIA QYLPMN5HHHFPZAM2 >fx-credentials.txt
