#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
E2E_DIR="${ROOT_DIR}/e2e"
MODELS_DIR="${E2E_DIR}/models"

CONTAINER_TOOL="${CONTAINER_TOOL:-docker}"
PULL_POLICY="${PULL_POLICY:-never}"

fail() {
  echo "[e2e] ERROR: $*" >&2
  exit 1
}

log() {
  echo "[e2e] $*"
}

require_cmd() {
  local cmd="$1"
  command -v "${cmd}" >/dev/null 2>&1 || fail "Required command not found: ${cmd}"
}

resolve_pack_command() {
  if [[ -n "${PACK_CMD:-}" ]]; then
    echo "${PACK_CMD}"
    return
  fi

  if [[ "$(uname -s)" == "Darwin" ]]; then
    echo "lima pack"
    return
  fi

  echo "pack"
}

run_pack() {
  local pack_cmd
  pack_cmd="$(resolve_pack_command)"

  if [[ "${pack_cmd}" == "lima pack" ]]; then
    require_cmd lima
    lima pack "$@"
    return
  fi

  require_cmd pack
  pack "$@"
}

default_builder_image() {
  local version
  version="$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo "0.1.0")"
  echo "aagumin/mlserver-builder:${version}"
}

builder_image() {
  echo "${BUILDER_IMAGE:-$(default_builder_image)}"
}

model_dir() {
  local model_name="$1"
  local dir="${MODELS_DIR}/${model_name}"

  [[ -d "${dir}" ]] || fail "Model directory not found: ${dir}"
  [[ -f "${dir}/MLmodel" ]] || fail "MLmodel file not found in: ${dir}"

  echo "${dir}"
}

image_tag_for_model() {
  local model_name="$1"
  local prefix="${IMAGE_PREFIX:-aipack-e2e}"
  local suffix="${IMAGE_SUFFIX:-local}"

  echo "${prefix}-${model_name}:${suffix}"
}

default_port_for_model() {
  local model_name="$1"

  case "${model_name}" in
    pyfunc)
      echo "18080"
      ;;
    sklearn)
      echo "18081"
      ;;
    *)
      echo "18082"
      ;;
  esac
}

build_image_for_model() {
  local image_tag="$1"
  local model_path="$2"

  local args=(
    build "${image_tag}"
    --builder "$(builder_image)"
    --path "${model_path}"
    --pull-policy "${PULL_POLICY}"
    --trust-builder
  )

  if [[ "$(uname -s)" == "Darwin" ]]; then
    args+=(--docker-host=inherit)
  fi

  run_pack "${args[@]}"
}
