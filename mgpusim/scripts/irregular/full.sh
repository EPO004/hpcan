#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
bash scripts/irregular/build.sh
python3 scripts/irregular/run.py --suite full "$@"
