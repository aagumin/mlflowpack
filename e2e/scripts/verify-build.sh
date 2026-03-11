#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

MODEL_NAME="${1:-}"
[[ -n "${MODEL_NAME}" ]] || fail "Usage: $0 <pyfunc|sklearn>"

MODEL_PATH="$(model_dir "${MODEL_NAME}")"
IMAGE_TAG="${IMAGE_TAG:-$(image_tag_for_model "${MODEL_NAME}")}"

require_cmd "${CONTAINER_TOOL}"

log "Building image '${IMAGE_TAG}' from model '${MODEL_NAME}'"
log "Builder: $(builder_image)"

build_image_for_model "${IMAGE_TAG}" "${MODEL_PATH}"

log "Build verification passed for model '${MODEL_NAME}'"
