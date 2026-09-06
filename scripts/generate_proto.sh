#!/usr/bin/env bash
set -euo pipefail

# Ensure PATH includes Go and protoc binaries
export PATH="${PATH}:/usr/local/go/bin:${HOME}/go/bin:${HOME}/.local/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "==> Compiling Protocol Buffers from ${ROOT_DIR}/proto..."

# 1. Generate Go code
mkdir -p "${ROOT_DIR}/gen/go/telemetry/v1"
protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --go_out="${ROOT_DIR}" \
  --go_opt=module=github.com/Manex142/uav-lab \
  "${ROOT_DIR}/proto/telemetry/v1/telemetry.proto"

# 2. Generate Python code (for testing / Python scripts)
mkdir -p "${ROOT_DIR}/gen/python/telemetry/v1"
touch "${ROOT_DIR}/gen/python/__init__.py"
touch "${ROOT_DIR}/gen/python/telemetry/__init__.py"
touch "${ROOT_DIR}/gen/python/telemetry/v1/__init__.py"
protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --python_out="${ROOT_DIR}/gen/python" \
  "${ROOT_DIR}/proto/telemetry/v1/telemetry.proto"

echo "==> Successfully generated Protobuf bindings in ${ROOT_DIR}/gen/."
