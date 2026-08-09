#!/bin/bash
set -e

# Enable local container access to host X11 display
xhost +local:root > /dev/null 2>&1 || xhost +local:$USER > /dev/null 2>&1 || true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🛸 Launching UAV Lab ROS 2 Jazzy (Ubuntu 24.04) Environment..."

# Build container image if --build argument is passed
if [[ "$1" == "--build" ]]; then
    echo "🔨 Building Docker image..."
    docker compose -f "$SCRIPT_DIR/docker-compose.yml" build
fi

# Run interactive shell session in container
docker compose -f "$SCRIPT_DIR/docker-compose.yml" run --rm ros2-jazzy bash
