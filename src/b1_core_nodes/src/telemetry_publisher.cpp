#include <chrono>
#include <cmath>
#include <memory>
#include <utility>

#include "geometry_msgs/msg/pose_stamped.hpp"
#include "rclcpp/rclcpp.hpp"

using namespace std::chrono_literals;

namespace uav_lab
{

class TelemetryPublisher : public rclcpp::Node
{
public:
  TelemetryPublisher()
  : Node("telemetry_publisher"),
    counter_(0)
  {
    // QoS profile: SensorData (Best Effort, Volatile, Depth 5) for low latency
    rclcpp::QoS qos_profile(5);
    qos_profile.reliability(rclcpp::ReliabilityPolicy::BestEffort);
    qos_profile.durability(rclcpp::DurabilityPolicy::Volatile);

    publisher_ = this->create_publisher<geometry_msgs::msg::PoseStamped>(
      "telemetry", qos_profile);

    // 100 Hz timer (10ms period)
    timer_ = this->create_wall_timer(
      10ms, std::bind(&TelemetryPublisher::timer_callback, this));

    RCLCPP_INFO(
      this->get_logger(),
      "🚀 High-frequency C++20 Telemetry Publisher Node started at 100 Hz");
  }

private:
  void timer_callback()
  {
    auto message = std::make_unique<geometry_msgs::msg::PoseStamped>();

    // Header metadata
    message->header.stamp = this->now();
    message->header.frame_id = "base_link";

    // Simulated helical trajectory for UAV telemetry
    double t = counter_ * 0.01;  // 10ms increments
    message->pose.position.x = 2.0 * std::cos(t);
    message->pose.position.y = 2.0 * std::sin(t);
    message->pose.position.z = 1.0 + 0.01 * counter_;

    // Simulated orientation quaternion
    message->pose.orientation.w = std::cos(t * 0.5);
    message->pose.orientation.z = std::sin(t * 0.5);

    publisher_->publish(std::move(message));
    counter_++;
  }

  rclcpp::Publisher<geometry_msgs::msg::PoseStamped>::SharedPtr publisher_;
  rclcpp::TimerBase::SharedPtr timer_;
  uint64_t counter_;
};

}  // namespace uav_lab

int main(int argc, char * argv[])
{
  rclcpp::init(argc, argv);
  rclcpp::spin(std::make_shared<uav_lab::TelemetryPublisher>());
  rclcpp::shutdown();
  return 0;
}
