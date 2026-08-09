cmake_minimum_required(VERSION 3.8)
project(b1_core_nodes)

if(NOT CMAKE_CXX_STANDARD)
  set(CMAKE_CXX_STANDARD 20)
endif()

if(CMAKE_COMPILER_IS_GNUCXX OR CMAKE_CXX_COMPILER_ID MATCHES "Clang")
  add_compile_options(-Wall -Wextra -Wpedantic)
endif()

find_package(ament_cmake REQUIRED)
find_package(rclcpp REQUIRED)
find_package(geometry_msgs REQUIRED)

add_executable(telemetry_publisher src/telemetry_publisher.cpp)
target_include_directories(telemetry_publisher PUBLIC
  $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
  $<INSTALL_INTERFACE:include>
)
target_compile_features(telemetry_publisher PUBLIC cxx_std_20)
ament_target_dependencies(telemetry_publisher
  rclcpp
  geometry_msgs
)

install(TARGETS
  telemetry_publisher
  DESTINATION lib/${PROJECT_NAME}
)

ament_package()
