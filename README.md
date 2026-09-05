# 🛸 UAV Lab: Cyber-Physical Systems & Edge AI Roadmap

[![ROS2 Jazzy](https://img.shields.io/badge/ROS2-Jazzy_Jalisco-blue.svg)](https://docs.ros.org/en/lyrical)
[![C++20](https://img.shields.io/badge/C++-20-blue.svg)](https://en.cppreference.com/w/cpp/20)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

An engineering-focused repository tracking my progression into **Unmanned Aerial Vehicles (UAVs)**, **Robotics Middleware**, **Edge AI**, and **Cyber-Physical Systems**. 

Building upon a background in **Software Engineering and IoT**, this lab focuses on low-latency messaging, distributed ROS2 architectures, PX4 autopilot integration, computer vision on edge devices, and autonomous navigation.

---

## 🗺️ Roadmap & Epics (Pragmatic MVP)

### 📌 `[EPIC-01] Block 1: ROS2 Middleware Core & Messaging Performance`
* **Objective:** Master ROS2 cyber-physical software architecture, DDS QoS policies, and multi-threading execution in `rclcpp` using standard simulation models.
* **Core Competencies:** ROS2 Graph, Executors, Multi-threaded Spinners, DDS QoS profiles (Reliable vs. Best Effort, Volatile vs. Transient Local), Latency benchmarking, Shared-Memory IPC (`rclcpp`).
* **Key Deliverables:**
  * High-frequency (100 Hz) C++ Pub/Sub pipeline measuring DDS latency & QoS profiles.
  * Integration with standard Gazebo Harmonic robot models (PX4 X500 / TurtleBot3) controlled via ROS2 velocity topics (`geometry_msgs/msg/Twist`) while monitoring odometry.

---

### 📌 `[EPIC-02] Block 2: Hardware Interfacing, Protocols & Go Edge Gateway (Core Focus)`
* **Objective:** Bridge high-level edge software with PX4 SITL UAV autopilot, microservices, and secure Cloud IoT infrastructure using Go and ROS2.
* **Core Competencies:** MAVLink protocol, Micro-XRCE-DDS Agent integration, Go (`Golang`) telemetry ingestion service, WireGuard encrypted VPN tunnels, Time-Series Storage (TimescaleDB).
* **Key Deliverables:**
  * C++ ROS2 sensor telemetry parser node running on Raspberry Pi 5 / Edge environment.
  * **Cyber-Physical Edge Gateway (Capstone Project):** PX4 running in SITL mode, ROS2 bridge streaming encrypted telemetry over WireGuard from Pi 5 to a **Go backend ingestion service**, storing real-time data in TimescaleDB with a live React web dashboard.

---

### 📌 `[EPIC-03] Block 3: Computer Vision & Edge AI (Python Pipeline)`
* **Objective:** Implement real-time perception pipelines and Deep Learning inference running on edge hardware via Python (`rclpy`).
* **Core Competencies:** Python ROS2 nodes (`rclpy`), OpenCV video stream capture, Ultralytics YOLOv8 inference on Pi 5 / Edge CPU, target coordinate transformation, and ROS2 topic publishing.
* **Key Deliverables:**
  * `rclpy` camera node publishing target detections (`geometry_msgs/msg/Pose2D`) from live/simulated video streams to the ROS2 graph for downstream guidance.

---

### 📌 `[EPIC-04] Block 4: Autonomous Navigation & State Estimation (Single-Agent Autonomy)`
* **Objective:** Implement GPS/Odometry-based single-agent waypoint navigation and state estimation without multi-robot complexity.
* **Core Competencies:** EKF state estimation, Nav2 stack integration, Waypoint follow action servers, ROS2 action clients.
* **Key Deliverables:**
  * Autonomous single-agent navigation mission in a simulated Gazebo environment executing a sequence of waypoints received from the external Go API gateway.

---

## 🛠️ Quick Start & Build Workflow

### Prerequisites
* Docker & Docker Compose installed on the host system.

### 1. Launch ROS2 Jazzy Container
From the repository root on your host machine:
```bash
./docker/run.sh
```

### 2. Build Workspace
Inside the interactive container environment (`/workspace`):
```bash
colcon build
```

### 3. Source Workspace Environment
To register compiled ROS2 nodes and environment hooks in your active terminal session:
```bash
source install/setup.bash
```

---

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

