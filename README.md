# 🛸 UAV Lab: Cyber-Physical Systems & Edge AI Roadmap

[![ROS2 Jazzy](https://img.shields.io/badge/ROS2-Jazzy_Jalisco-blue.svg)](https://docs.ros.org/en/lyrical)
[![C++20](https://img.shields.io/badge/C++-20-blue.svg)](https://en.cppreference.com/w/cpp/20)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

An engineering-focused repository tracking my progression into **Unmanned Aerial Vehicles (UAVs)**, **Robotics Middleware**, **Edge AI**, and **Cyber-Physical Systems**. 

Building upon a background in **Software Engineering and IoT**, this lab focuses on low-latency messaging, distributed ROS2 architectures, PX4 autopilot integration, computer vision on edge devices, and autonomous navigation.

---

## 🗺️ Roadmap & Epics

### 📌 `[EPIC-01] Block 1: ROS2 Middleware Core & C++ (Distributed Systems)`
* **Objective:** Master ROS2 cyber-physical software architecture using C++20 and physical simulation in Gazebo Harmonic without relying on physical hardware.
* **Core Competencies:** ROS2 Graph, Executors, Lifecycle Nodes, DDS Data Distribution Service (QoS policies, Zero-Copy shared memory IPC via `rclcpp`), URDF/Xacro kinematics, Gazebo Harmonic plugins.
* **Key Deliverables:**
  * High-frequency (100 Hz) C++ Pub/Sub pipeline measuring DDS latency & QoS profiles.
  * Differential Drive robot designed in URDF/Xacro, simulated in Gazebo, controlled via C++ velocity commands (`geometry_msgs/msg/Twist`) with real-time odometry feedback.

---

### 📌 `[EPIC-02] Block 2: Hardware Interfacing, Protocols & Edge Networking`
* **Objective:** Bridge high-level software with physical hardware on Raspberry Pi 5, integrate UAV autopilots via SITL, and connect ROS2 with IoT/Cloud infrastructure.
* **Core Competencies:** Linux low-level buses (I2C, SPI, UART, CAN), MAVLink protocol, PX4 Autopilot integration via Micro-XRCE-DDS agent, ROS2-to-MQTT/WebSockets bridging, WireGuard VPN tunneling.
* **Key Deliverables:**
  * C++ ROS2 node reading raw IMU sensor data via I2C on Raspberry Pi 5.
  * **Remote Cyber-Physical Gateway:** PX4 running in SITL mode, ROS2 bridge streaming encrypted telemetry over WireGuard from Pi 5 to a Node.js/TS backend storing data in TimescaleDB with a real-time web UI.

---

### 📌 `[EPIC-03] Block 3: Computer Vision & Edge AI`
* **Objective:** Implement real-time perception pipelines and Deep Learning inference running locally on low-power edge hardware (Raspberry Pi 5).
* **Core Competencies:** Low-latency video pipelines using OpenCV and `image_transport`, C++ model execution with ONNX Runtime, FP32 to INT8/FP16 quantization (YOLOv8 / MobileNet), 3D target tracking & relative position estimation.
* **Key Deliverables:**
  * C++ camera capture node applying OpenCV color/contour filters publishing 2D target coordinates.
  * **Visual Target Tracker:** Quantized YOLO model executing in C++ on Pi 5, estimating target tracking vectors and issuing trajectory adjustments to the Gazebo simulator.

---

### 📌 `[EPIC-04] Block 4: State Estimation, SLAM & Autonomous Navigation`
* **Objective:** Develop advanced autonomy algorithms for state estimation, mapping, and GPS-denied navigation, culminating in multi-agent robot coordination.
* **Core Competencies:** Extended Kalman Filters (EKF) via `robot_localization` (IMU + Odometry), 2D/3D SLAM (SLAM Toolbox, ORB-SLAM3), Nav2 Stack (Costmaps, Planners, Controllers, Behavior Trees), Multi-robot systems (Namespaces, DDS domain isolation).
* **Key Deliverables:**
  * 2D occupancy grid mapping of a simulated Gazebo environment using SLAM Toolbox and teleoperation.
  * **Multi-Agent Autonomous Navigation (Capstone Project):** Dual simulated robots navigating a warehouse environment, processing destination goals via external API, dynamically avoiding obstacles with Nav2, and coordinating routes.

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

