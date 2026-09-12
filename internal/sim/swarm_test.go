package sim

import (
	"net"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"google.golang.org/protobuf/proto"
)

func TestSwarm_ConcurrentEmission(t *testing.T) {
	// 1. Setup temporary UDP test server
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to bind UDP test listener: %v", err)
	}
	defer conn.Close()

	testAddr := conn.LocalAddr().String()

	// 2. Configure 5 drones at 50 Hz
	const droneCount = 5
	cfg := SwarmConfig{
		DroneCount:   droneCount,
		TargetAddr:   testAddr,
		FrequencyHz:  50,
		EnableJitter: true,
		MinRadius:    15.0,
		MaxRadius:    25.0,
		MinSpeed:     5.0,
		MaxSpeed:     10.0,
		BaseAltitude: 20.0,
	}

	swarm := NewSwarm(cfg)
	if err := swarm.Start(); err != nil {
		t.Fatalf("Failed to start swarm: %v", err)
	}

	// 3. Receive datagrams for 300ms
	receivedDrones := make(map[string]int)
	buf := make([]byte, 1024)
	deadline := time.Now().Add(300 * time.Millisecond)

	for time.Now().Before(deadline) {
		_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}

		var record telemetryv1.TelemetryRecord
		if err := proto.Unmarshal(buf[:n], &record); err != nil {
			t.Errorf("Failed to decode Protobuf datagram: %v", err)
			continue
		}

		receivedDrones[record.GetDeviceId()]++
	}

	// 4. Stop swarm
	swarm.Stop()

	// 5. Verify all drones emitted packets
	t.Logf("Received packet counts by drone: %+v", receivedDrones)
	if len(receivedDrones) != droneCount {
		t.Errorf("Expected packets from %d drones, got %d", droneCount, len(receivedDrones))
	}

	metrics := swarm.Metrics()
	if metrics.PacketsSent.Load() == 0 {
		t.Errorf("Expected positive packets sent count")
	}
	if metrics.NetworkErrors.Load() != 0 {
		t.Errorf("Expected 0 network errors, got %d", metrics.NetworkErrors.Load())
	}
}
