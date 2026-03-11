#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=lib.sh
source "${SCRIPT_DIR}/lib.sh"

MODEL_NAME="${1:-}"
[[ -n "${MODEL_NAME}" ]] || fail "Usage: $0 <pyfunc|sklearn>"

MODEL_PATH="$(model_dir "${MODEL_NAME}")"
IMAGE_TAG="${IMAGE_TAG:-$(image_tag_for_model "${MODEL_NAME}")}"
PORT="${E2E_PORT:-$(default_port_for_model "${MODEL_NAME}")}"
READINESS_ATTEMPTS="${READINESS_ATTEMPTS:-60}"
READINESS_SLEEP_SECONDS="${READINESS_SLEEP_SECONDS:-2}"

require_cmd "${CONTAINER_TOOL}"
require_cmd curl
require_cmd python3

log "Running build verification before runtime checks"
IMAGE_TAG="${IMAGE_TAG}" "${SCRIPT_DIR}/verify-build.sh" "${MODEL_NAME}"

log "Starting container '${IMAGE_TAG}' on port ${PORT}"
CONTAINER_ID="$("${CONTAINER_TOOL}" run -d -p "${PORT}:8080" -e MLSERVER_PARALLEL_WORKERS=0 "${IMAGE_TAG}")"
RESPONSE_FILE="$(mktemp)"

cleanup() {
  rm -f "${RESPONSE_FILE}" || true
  if [[ -n "${CONTAINER_ID:-}" ]]; then
    "${CONTAINER_TOOL}" rm -f "${CONTAINER_ID}" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

ready=0
for _ in $(seq 1 "${READINESS_ATTEMPTS}"); do
  if ! "${CONTAINER_TOOL}" ps -q --no-trunc | grep -q "${CONTAINER_ID}"; then
    break
  fi

  if curl -fsS "http://127.0.0.1:${PORT}/v2/health/ready" >/dev/null; then
    ready=1
    break
  fi
  sleep "${READINESS_SLEEP_SECONDS}"
done

if [[ "${ready}" -ne 1 ]]; then
  log "Container logs for debugging:"
  "${CONTAINER_TOOL}" logs "${CONTAINER_ID}" || true
  fail "MLServer did not become ready for model '${MODEL_NAME}'"
fi

log "Sending inference request for '${MODEL_NAME}'"
HTTP_STATUS="$(
  curl -sS -o "${RESPONSE_FILE}" -w "%{http_code}" -X POST "http://127.0.0.1:${PORT}/v2/models/model/infer" \
  -H "Content-Type: application/json" \
  -d @"${MODEL_PATH}/test-request.json"
)"

if [[ "${HTTP_STATUS}" != "200" ]]; then
  log "Inference failed with HTTP status ${HTTP_STATUS}"
  log "Inference response body:"
  cat "${RESPONSE_FILE}" || true
  log "Container logs for debugging:"
  "${CONTAINER_TOOL}" logs "${CONTAINER_ID}" || true
  fail "Inference request failed for model '${MODEL_NAME}'"
fi

python3 - "${RESPONSE_FILE}" "${MODEL_PATH}/expected-response.json" <<'PY'
import json
import math
import sys

response_path, expected_path = sys.argv[1], sys.argv[2]

with open(response_path, encoding="utf-8") as fp:
    response = json.load(fp)

with open(expected_path, encoding="utf-8") as fp:
    expected = json.load(fp)

outputs = response.get("outputs")
if not outputs:
    raise SystemExit("[e2e] ERROR: inference response has no 'outputs'")

data = outputs[0].get("data")
if data is None:
    raise SystemExit("[e2e] ERROR: inference response has no 'outputs[0].data'")

if isinstance(data, list) and len(data) == 1 and isinstance(data[0], list):
    data = data[0]

expected_data = expected.get("output_data")
if not isinstance(expected_data, list):
    raise SystemExit("[e2e] ERROR: expected-response.json must contain list field 'output_data'")

if len(data) != len(expected_data):
    raise SystemExit(
        f"[e2e] ERROR: output length mismatch: got {len(data)}, expected {len(expected_data)}"
    )

tolerance = float(expected.get("tolerance", 0))

for idx, (actual, exp) in enumerate(zip(data, expected_data)):
    if isinstance(exp, (int, float)):
        if math.fabs(float(actual) - float(exp)) > tolerance:
            raise SystemExit(
                f"[e2e] ERROR: output[{idx}] mismatch: got {actual}, expected {exp} (tol={tolerance})"
            )
    else:
        if str(actual) != str(exp):
            raise SystemExit(
                f"[e2e] ERROR: output[{idx}] mismatch: got {actual!r}, expected {exp!r}"
            )

print("[e2e] Runtime inference output matches expected response")
PY

log "Runtime verification passed for model '${MODEL_NAME}'"
