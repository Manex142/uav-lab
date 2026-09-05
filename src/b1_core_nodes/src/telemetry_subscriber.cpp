#include <chrono>
#include <deque>
#include <memory>
#include <numeric>
#include <utility>

#include "geometry_msgs/msg/pose_stamped.hpp"
#include "rclcpp/rclcpp.hpp"

namespace uav_lab
{

/**
 * @brief Moving Average Filter for 3D position telemetry.
 */
class MovingAverageFilter
{
public:
  explicit MovingAverageFilter(size_t window_size = 10)
  : window_size_(window_size),
    sum_x_(0.0),
    sum_y_(0.0),
    sum_z_(0.0)
  {
  }

  void add_sample(double x, double y, double z)
  {
    buffer_.push_back({x, y, z});
    sum_x_ += x;
    sum_y_ += y;
    sum_z_ += z;

    if (buffer_.size() > window_size_) {
      const auto & oldest = buffer_.front();
      sum_x_ -= oldest.x;
      sum_y_ -= oldest.y;
      sum_z_ -= oldest.z;
      buffer_.pop_front();
    }
  }

  [[nodiscard]] std::tuple<double, double, double> get_average() const
  {
    if (buffer_.empty()) {
      return {0.0, 0.0, 0.0};
    }
    size_t count = buffer_.size();
    return {sum_x_ / count, sum_y_ / count, sum_z_ / count};
  }

  [[nodiscard]] size_t current_samples() const
  {
    return buffer_.size();
  }

private:
  struct Point3D
  {
    double x;
    double y;
    double z;
  };

  size_t window_size_;
  std::deque<Point3D> buffer_;
  double sum_x_;
  double sum_y_;
  double sum_z_;
};

class TelemetrySubscriber : public rclcpp::Node
{
public:
  TelemetrySubscriber()
  : Node("telemetry_subscriber"),
    filter_(10)
  {
    // QoS profile matching publisher: SensorData (Best Effort, Volatile, Depth 5)
    rclcpp::QoS qos_profile(5);
    qos_profile.reliability(rclcpp::ReliabilityPolicy::BestEffort);
    qos_profile.durability(rclcpp::DurabilityPolicy::Volatile);

    subscription_ = this->create_subscription<geometry_msgs::msg::PoseStamped>(
      "telemetry",
      qos_profile,
      std::bind(&TelemetrySubscriber::telemetry_callback, this, std::placeholders::_1));

    filtered_publisher_ = this->create_publisher<geometry_msgs::msg::PoseStamped>(
      "telemetry_filtered", qos_profile);

    RCLCPP_INFO(
      this->get_logger(),
      "📥 C++20 Telemetry Subscriber Node initialized (Window size: 10, QoS: Best Effort)");
  }

private:
  void telemetry_callback(const geometry_msgs::msg::PoseStamped::SharedPtr msg)
  {
    // 1. Add raw sample to sliding window filter
    filter_.add_sample(msg->pose.position.x, msg->pose.position.y, msg->pose.position.z);

    // 2. Compute current 3D moving average
    auto [avg_x, avg_y, avg_z] = filter_.get_average();

    // 3. Create filtered PoseStamped message
    auto filtered_msg = std::make_unique<geometry_msgs::msg::PoseStamped>();
    filtered_msg->header = msg->header;
    filtered_msg->header.stamp = this->now();  // Current processing timestamp

    filtered_msg->pose.position.x = avg_x;
    filtered_msg->pose.position.y = avg_y;
    filtered_msg->pose.position.z = avg_z;

    // Retain orientation from input telemetry
    filtered_msg->pose.orientation = msg->pose.orientation;

    filtered_publisher_->publish(std::move(filtered_msg));
  }

  MovingAverageFilter filter_;
  rclcpp::Subscription<geometry_msgs::msg::PoseStamped>::SharedPtr subscription_;
  rclcpp::Publisher<geometry_msgs::msg::PoseStamped>::SharedPtr filtered_publisher_;
};

}  // namespace uav_lab

int main(int argc, char * argv[])
{
  rclcpp::init(argc, argv);
  rclcpp::spin(std::make_shared<uav_lab::TelemetrySubscriber>());
  rclcpp::shutdown();
  return 0;
}
