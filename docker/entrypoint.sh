#!/bin/bash
set -e

# Unset WAYLAND_DISPLAY inside container to avoid XWayland/Mesa GPU teardown crashes
unset WAYLAND_DISPLAY
export QT_QPA_PLATFORM=xcb
export QT_X11_NO_MITSHM=1

# Source ROS 2 Jazzy environment
if [ -f "/opt/ros/jazzy/setup.bash" ]; then
    source "/opt/ros/jazzy/setup.bash"
fi

# Source workspace install overlay if built
if [ -f "/workspace/install/setup.bash" ]; then
    source "/workspace/install/setup.bash"
fi

exec "$@"
