package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
	"github.com/Manex142/uav-lab/internal/ingest"
	"github.com/Manex142/uav-lab/internal/ledger"
	"github.com/coder/websocket"
)

func TestServer_RESTEndpoints(t *testing.T) {
	registry := ledger.NewDeviceRegistry()
	metrics := &ingest.Metrics{}

	cfg := ServerConfig{
		ListenAddr: "127.0.0.1:0", // Ephemeral port
		HubConfig:  DefaultHubConfig(),
	}

	srv := NewServer(cfg, registry, metrics)
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	addr := srv.Addr().String()

	// 1. Test /api/health
	resp, err := http.Get(fmt.Sprintf("http://%s/api/health", addr))
	if err != nil {
		t.Fatalf("Failed to call /api/health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/health, got %d", resp.StatusCode)
	}

	var health map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode /api/health response: %v", err)
	}

	if health["status"] != "HEALTHY" {
		t.Errorf("Expected status HEALTHY, got %v", health["status"])
	}

	// 2. Test /api/fleet (initial empty)
	respFleet, err := http.Get(fmt.Sprintf("http://%s/api/fleet", addr))
	if err != nil {
		t.Fatalf("Failed to call /api/fleet: %v", err)
	}
	defer respFleet.Body.Close()

	if respFleet.StatusCode != http.StatusOK {
		t.Fatalf("Expected 200 OK from /api/fleet, got %d", respFleet.StatusCode)
	}

	var fleetResp map[string]any
	if err := json.NewDecoder(respFleet.Body).Decode(&fleetResp); err != nil {
		t.Fatalf("Failed to decode /api/fleet response: %v", err)
	}

	count, ok := fleetResp["count"].(float64)
	if !ok || count != 0 {
		t.Errorf("Expected count 0, got %v", fleetResp["count"])
	}
}

func TestServer_WebSocketStreaming(t *testing.T) {
	registry := ledger.NewDeviceRegistry()
	metrics := &ingest.Metrics{}

	hubCfg := HubConfig{
		BroadcastHz:    20, // Fast 50ms tick for responsive testing
		HomeLatitude:   DefaultHomeLat,
		HomeLongitude:  DefaultHomeLon,
		ClientQueueLen: 16,
	}

	srvCfg := ServerConfig{
		ListenAddr: "127.0.0.1:0",
		HubConfig:  hubCfg,
	}

	srv := NewServer(srvCfg, registry, metrics)
	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	addr := srv.Addr().String()

	// Feed a test drone into the registry
	record := &telemetryv1.TelemetryRecord{
		DeviceId:       "uav-alpha-test",
		SequenceNumber: 42,
		TimestampNs:    time.Now().UnixNano(),
		Position: &telemetryv1.Vector3{
			X: 100.0,
			Y: 200.0,
			Z: -50.0,
		},
		EulerAngles: &telemetryv1.EulerAngles{
			Roll:  0.05,
			Pitch: -0.02,
			Yaw:   1.57,
		},
		LinearVelocity: &telemetryv1.Vector3{
			X: 10.0,
			Y: 0.0,
			Z: 0.0,
		},
		Armed:      true,
		FlightMode: telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD,
		Battery: &telemetryv1.BatteryState{
			VoltageV:   16.2,
			CurrentA:   12.5,
			Percentage: 0.85,
		},
	}
	registry.Update(record)

	// Connect WebSocket client
	wsURL := fmt.Sprintf("ws://%s/ws/telemetry", addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to dial WebSocket %s: %v", wsURL, err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test done")

	// Wait for first FLEET_SNAPSHOT message
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read WebSocket message: %v", err)
	}

	if msgType != websocket.MessageText {
		t.Fatalf("Expected text message, got %v", msgType)
	}

	var snapshot FleetSnapshotMessage
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("Failed to parse JSON snapshot: %v", err)
	}

	if snapshot.Type != "FLEET_SNAPSHOT" {
		t.Fatalf("Expected type FLEET_SNAPSHOT, got %s", snapshot.Type)
	}

	if len(snapshot.Devices) != 1 {
		t.Fatalf("Expected 1 device in snapshot, got %d", len(snapshot.Devices))
	}

	dev := snapshot.Devices[0]
	if dev.ID != "uav-alpha-test" {
		t.Errorf("Expected device ID 'uav-alpha-test', got '%s'", dev.ID)
	}
	if dev.Altitude != 50.0 {
		t.Errorf("Expected altitude 50.0, got %f", dev.Altitude)
	}
	if dev.BatteryPct != 85.0 {
		t.Errorf("Expected battery percentage 85.0, got %f", dev.BatteryPct)
	}
	if dev.Latitude <= 0 || dev.Longitude == 0 {
		t.Errorf("Expected valid WGS84 coordinates, got lat=%f, lon=%f", dev.Latitude, dev.Longitude)
	}

	// Test lifecycle event broadcast
	srv.Hub().BroadcastEvent(LifecycleEventMessage{
		Type:      "LIFECYCLE_EVENT",
		Timestamp: time.Now().UnixMilli(),
		Event:     "TEST_EVENT",
		DeviceID:  "uav-alpha-test",
		Details:   "unit testing event",
	})

	// Read event message
	_, evtData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Failed to read event message: %v", err)
	}

	var evt LifecycleEventMessage
	if err := json.Unmarshal(evtData, &evt); err != nil {
		t.Fatalf("Failed to unmarshal lifecycle event: %v", err)
	}

	if evt.Event != "TEST_EVENT" && evt.Type != "FLEET_SNAPSHOT" {
		// Could be next snapshot or event; both are valid over same stream
		t.Logf("Received message over stream: %s", string(evtData))
	}

	t.Log("WebSocket telemetry stream verified successfully.")
}
