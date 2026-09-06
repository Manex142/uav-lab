#!/usr/bin/env bash
set -euo pipefail

# Ensure PATH includes Go and protoc binaries
export PATH="${PATH}:/usr/local/go/bin:${HOME}/go/bin:${HOME}/.local/bin"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "==> Compiling Protocol Buffers from ${ROOT_DIR}/proto..."

# 1. Generate Go code
mkdir -p "${ROOT_DIR}/pkg/telemetry/v1"
protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --go_out="${ROOT_DIR}" \
  --go_opt=module=github.com/Manex142/uav-lab \
  "${ROOT_DIR}/proto/telemetry/v1/telemetry.proto"

# 2. Generate Python code
mkdir -p "${ROOT_DIR}/pkg/telemetry/v1/python"
touch "${ROOT_DIR}/pkg/telemetry/v1/python/__init__.py"
protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --python_out="${ROOT_DIR}/pkg/telemetry/v1/python" \
  "${ROOT_DIR}/proto/telemetry/v1/telemetry.proto"

# 3. Generate C++ code
mkdir -p "${ROOT_DIR}/src/b1_core_nodes/include/proto"
protoc \
  --proto_path="${ROOT_DIR}/proto" \
  --cpp_out="${ROOT_DIR}/src/b1_core_nodes/include/proto" \
  "${ROOT_DIR}/proto/telemetry/v1/telemetry.proto"

echo "==> Successfully generated Protobuf bindings for Go, Python, and C++."
