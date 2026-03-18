#!/usr/bin/env bash
set -euo pipefail

image_tag="${1:-aipack-stack-run-smoke:test}"

docker build -t "${image_tag}" stack/run >/dev/null

container_id="$(docker create "${image_tag}")"
cleanup() {
  docker rm -f "${container_id}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

archive_listing="$(docker export "${container_id}" | tar -tf -)"

for forbidden in \
  "usr/bin/python3" \
  "usr/bin/python3.11" \
  "usr/local/bin/pip3.11" \
  "usr/local/bin/mlserver"; do
  if grep -qx "${forbidden}" <<<"${archive_listing}"; then
    echo "forbidden runtime artifact present: ${forbidden}" >&2
    exit 1
  fi
done

echo "run image is slim: no system python/mlserver artifacts found"
