# Base image: Official OSRF ROS 2 Jazzy Desktop image (Ubuntu 24.04)
FROM osrf/ros:jazzy-desktop

# Set non-interactive frontend during build
ENV DEBIAN_FRONTEND=noninteractive

# Install additional simulation packages, dev tools, and GPU utilities
RUN apt-get update && apt-get install -y --no-install-recommends \
    ros-jazzy-ros-gz \
    ros-jazzy-gz-ros2-control \
    ros-jazzy-teleop-twist-keyboard \
    python3-colcon-common-extensions \
    python3-rosdep \
    build-essential \
    cmake \
    gdb \
    git \
    mesa-utils \
    libgl1-mesa-dri \
    sudo \
    && rm -rf /var/lib/apt/lists/*

# Arguments for matching host user UID/GID (Default 1000:1000)
ARG USERNAME=manex
ARG USER_UID=1000
ARG USER_GID=1000

# Create user with host UID/GID or modify existing user/group 1000 (e.g. ubuntu user in Ubuntu 24.04 base)
RUN (getent group $USER_GID >/dev/null && groupmod -n $USERNAME $(getent group $USER_GID | cut -d: -f1) || groupadd --gid $USER_GID $USERNAME) \
    && (getent passwd $USER_UID >/dev/null && usermod -l $USERNAME -g $USER_GID -d /home/$USERNAME -m $(getent passwd $USER_UID | cut -d: -f1) || useradd --uid $USER_UID --gid $USER_GID -m $USERNAME) \
    && echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" >> /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Copy entrypoint script
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Set working directory to workspace root
WORKDIR /workspace

# Switch to non-root user
USER $USERNAME

ENTRYPOINT ["/entrypoint.sh"]
CMD ["bash"]
