#!/usr/bin/env bash
set -euo pipefail

image_tag="${1:-aipack-stack-build-smoke:test}"

docker build -t "${image_tag}" stack/build >/dev/null

container_id="$(docker create "${image_tag}")"
cleanup() {
  docker rm -f "${container_id}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

config_user="$(docker inspect --format '{{.Config.User}}' "${container_id}")"
if [[ "${config_user}" != "1001:1001" ]]; then
  echo "expected runtime user 1001:1001, got ${config_user}" >&2
  exit 1
fi

config_workdir="$(docker inspect --format '{{.Config.WorkingDir}}' "${container_id}")"
if [[ "${config_workdir}" != "/workspace" ]]; then
  echo "expected workdir /workspace, got ${config_workdir}" >&2
  exit 1
fi

config_env="$(docker inspect --format '{{join .Config.Env "\n"}}' "${container_id}")"
if ! grep -qx 'CNB_USER_ID=1001' <<<"${config_env}"; then
  echo "expected CNB_USER_ID=1001 in image env" >&2
  exit 1
fi

if ! grep -qx 'CNB_GROUP_ID=1001' <<<"${config_env}"; then
  echo "expected CNB_GROUP_ID=1001 in image env" >&2
  exit 1
fi

passwd_contents="$(docker export "${container_id}" | tar -xOf - etc/passwd)"
if ! grep -q '^cnb:x:1001:1001:' <<<"${passwd_contents}"; then
  echo "expected cnb passwd entry missing" >&2
  exit 1
fi

group_contents="$(docker export "${container_id}" | tar -xOf - etc/group)"
if ! grep -q '^cnb:x:1001:' <<<"${group_contents}"; then
  echo "expected cnb group entry missing" >&2
  exit 1
fi

echo "build image uses cnb user 1001:1001"
