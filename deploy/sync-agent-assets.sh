#!/usr/bin/env bash
set -euo pipefail

SRC_DIR="${1:-./artifacts}"
DEST_DIR="${2:-./agent}"

required_files=(
  "gost-amd64"
  "gost-amd64.sha256"
  "gost-arm64"
  "gost-arm64.sha256"
)

if [[ ! -d "${SRC_DIR}" ]]; then
  echo "source directory not found: ${SRC_DIR}" >&2
  exit 1
fi

mkdir -p "${DEST_DIR}"

for f in "${required_files[@]}"; do
  src_file="${SRC_DIR}/${f}"
  if [[ ! -f "${src_file}" ]]; then
    echo "missing required file: ${src_file}" >&2
    exit 1
  fi
  cp -f "${src_file}" "${DEST_DIR}/${f}"
done

chmod 0755 "${DEST_DIR}/gost-amd64" "${DEST_DIR}/gost-arm64"
chmod 0644 "${DEST_DIR}/gost-amd64.sha256" "${DEST_DIR}/gost-arm64.sha256"

echo "agent assets synced to ${DEST_DIR}"
