package telemetryv1_test

import (
	"math"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/pkg/telemetry/v1"
	"google.golang.org/protobuf/proto"
)

func sampleTelemetryRecord() *telemetryv1.TelemetryRecord {
	return &telemetryv1.TelemetryRecord{
		DeviceId:       "uav-alpha-01",
		SequenceNumber: 10425,
		TimestampNs:    time.Now().UnixNano(),
		Position: &telemetryv1.Vector3{
			X: 124.582,
			Y: -45.129,
			Z: -15.402, // 15.4 meters altitude in NED frame
		},
		Orientation: &telemetryv1.Quaternion{
			W: 0.9238795,
			X: 0.0,
			Y: 0.0,
			Z: 0.3826834, // ~45 deg yaw
		},
		EulerAngles: &telemetryv1.EulerAngles{
			Roll:  0.05,
			Pitch: -0.02,
			Yaw:   math.Pi / 4,
		},
		LinearVelocity: &telemetryv1.Vector3{
			X: 12.5,
			Y: 2.1,
			Z: -0.2,
		},
		AngularVelocity: &telemetryv1.Vector3{
			X: 0.01,
			Y: -0.02,
			Z: 0.05,
		},
		Armed:        true,
		FlightMode:   telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD,
		SystemStatus: telemetryv1.SystemStatus_SYSTEM_STATUS_ACTIVE,
		Battery: &telemetryv1.BatteryState{
			VoltageV:     15.2,
			CurrentA:     22.4,
			Percentage:   0.82,
			TemperatureC: 34.5,
		},
	}
}

func TestTelemetryRecord_Serialization(t *testing.T) {
	record := sampleTelemetryRecord()

	data, err := proto.Marshal(record)
	if err != nil {
		t.Fatalf("failed to marshal TelemetryRecord: %v", err)
	}

	payloadSize := len(data)
	t.Logf("Serialized TelemetryRecord payload size: %d bytes", payloadSize)

	// Acceptance criteria: compact footprint (under 256 bytes, well below WireGuard MTU)
	const maxAllowedBytes = 256
	if payloadSize > maxAllowedBytes {
		t.Errorf("expected serialized size <= %d bytes, got %d bytes", maxAllowedBytes, payloadSize)
	}

	// Verify deserialization
	var decoded telemetryv1.TelemetryRecord
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TelemetryRecord: %v", err)
	}

	if decoded.GetDeviceId() != record.GetDeviceId() {
		t.Errorf("expected DeviceId %q, got %q", record.GetDeviceId(), decoded.GetDeviceId())
	}
	if decoded.GetSequenceNumber() != record.GetSequenceNumber() {
		t.Errorf("expected SequenceNumber %d, got %d", record.GetSequenceNumber(), decoded.GetSequenceNumber())
	}
	if decoded.GetPosition().GetX() != record.GetPosition().GetX() {
		t.Errorf("expected Pos.X %f, got %f", record.GetPosition().GetX(), decoded.GetPosition().GetX())
	}
	if !decoded.GetArmed() {
		t.Errorf("expected Armed true, got false")
	}
	if decoded.GetFlightMode() != telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD {
		t.Errorf("expected FlightMode OFFBOARD, got %v", decoded.GetFlightMode())
	}
}

func BenchmarkTelemetryRecord_Marshal(b *testing.B) {
	record := sampleTelemetryRecord()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := proto.Marshal(record)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTelemetryRecord_Unmarshal(b *testing.B) {
	record := sampleTelemetryRecord()
	data, err := proto.Marshal(record)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		var decoded telemetryv1.TelemetryRecord
		if err := proto.Unmarshal(data, &decoded); err != nil {
			b.Fatal(err)
		}
	}
}
