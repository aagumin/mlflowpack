#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

MODELS=(pyfunc sklearn)

for model in "${MODELS[@]}"; do
  log "============================================================"
  log "Running full e2e verification for model '${model}'"
  "${SCRIPT_DIR}/verify-runtime.sh" "${model}"
done

log "All e2e checks passed"
