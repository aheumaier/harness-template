#!/usr/bin/env bash
# Runs in the repo copy: a harness script nothing runs. This file names it, but a fixture is not a referencer,
# so the script stays an orphan.
set -euo pipefail
printf '#!/usr/bin/env bash\necho "nobody calls me"\n' >scripts/harness/orphan.sh
chmod +x scripts/harness/orphan.sh
