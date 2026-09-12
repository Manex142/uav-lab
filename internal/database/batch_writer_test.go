package database

import (
	"context"
	"testing"
	"time"

	telemetryv1 "github.com/Manex142/uav-lab/gen/go/telemetry/v1"
)

func TestBatchWriter_Persistence(t *testing.T) {
	cfg := DefaultConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Ensure migrations are applied
	if err := RunMigrations(cfg.DSN()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// 2. Connect pool
	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect to pool: %v", err)
	}
	defer pool.Close()

	// Clean telemetry table for this test
	_, err = pool.Exec(ctx, "TRUNCATE telemetry;")
	if err != nil {
		t.Fatalf("Failed to truncate telemetry: %v", err)
	}

	// 3. Initialize BatchWriter with small batch for fast testing
	bwCfg := BatchWriterConfig{
		BatchSize:     50,
		FlushInterval: 50 * time.Millisecond,
		ChannelSize:   1000,
		FlushTimeout:  2 * time.Second,
	}

	writer := NewBatchWriter(pool, bwCfg)

	// 4. Enqueue 125 records (should trigger 2 size-based flushes + 1 timer/close flush)
	const totalRecords = 125
	baseTime := time.Now().UnixNano()

	for i := 0; i < totalRecords; i++ {
		record := &telemetryv1.TelemetryRecord{
			DeviceId:       "uav-test-persister",
			SequenceNumber: uint64(i + 1),
			TimestampNs:    baseTime + int64(i*10_000_000), // +10ms per record (100 Hz)
			Position: &telemetryv1.Vector3{
				X: float64(i) * 1.5,
				Y: float64(i) * -0.5,
				Z: -25.0,
			},
			Orientation: &telemetryv1.Quaternion{
				W: 1.0,
				X: 0.0,
				Y: 0.0,
				Z: 0.0,
			},
			EulerAngles: &telemetryv1.EulerAngles{
				Roll:  0.01,
				Pitch: -0.02,
				Yaw:   1.57,
			},
			LinearVelocity: &telemetryv1.Vector3{
				X: 5.0,
				Y: 2.0,
				Z: 0.0,
			},
			AngularVelocity: &telemetryv1.Vector3{
				X: 0.0,
				Y: 0.0,
				Z: 0.1,
			},
			Armed:        true,
			FlightMode:   telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD,
			SystemStatus: telemetryv1.SystemStatus_SYSTEM_STATUS_ACTIVE,
			Battery: &telemetryv1.BatteryState{
				VoltageV:     16.4,
				CurrentA:     15.2,
				Percentage:   0.92,
				TemperatureC: 32.5,
			},
		}

		if ok := writer.Enqueue(record); !ok {
			t.Fatalf("Failed to enqueue record %d", i)
		}
	}

	// 5. Close gracefully to flush remaining records
	writer.Close()

	// 6. Verify stats
	stats := writer.Stats()
	t.Logf("BatchWriter stats: %+v", stats)

	if stats.RecordsEnqueued != totalRecords {
		t.Errorf("Expected %d enqueued, got %d", totalRecords, stats.RecordsEnqueued)
	}
	if stats.RecordsPersisted != totalRecords {
		t.Errorf("Expected %d persisted, got %d", totalRecords, stats.RecordsPersisted)
	}
	if stats.FlushErrors != 0 {
		t.Errorf("Expected 0 flush errors, got %d", stats.FlushErrors)
	}

	// 7. Verify rows in database
	var count int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM telemetry WHERE device_id = 'uav-test-persister';").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count telemetry rows: %v", err)
	}

	if count != totalRecords {
		t.Fatalf("Expected %d rows in database, found %d", totalRecords, count)
	}

	// 8. Verify specific row content (latest row)
	var (
		seq        int64
		posX, posY float64
		armed      bool
		mode       string
		battV      float32
	)
	err = pool.QueryRow(ctx, `
		SELECT sequence_number, pos_x, pos_y, armed, flight_mode, battery_voltage
		FROM telemetry
		WHERE device_id = 'uav-test-persister'
		ORDER BY sequence_number DESC
		LIMIT 1;
	`).Scan(&seq, &posX, &posY, &armed, &mode, &battV)
	if err != nil {
		t.Fatalf("Failed to query latest telemetry row: %v", err)
	}

	if seq != totalRecords {
		t.Errorf("Expected sequence %d, got %d", totalRecords, seq)
	}
	if !armed {
		t.Errorf("Expected armed=true")
	}
	if mode != telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD.String() {
		t.Errorf("Expected flight_mode %s, got %s", telemetryv1.FlightMode_FLIGHT_MODE_OFFBOARD.String(), mode)
	}
	if battV != 16.4 {
		t.Errorf("Expected battery_voltage 16.4, got %f", battV)
	}

	t.Log("Batch persistence verified successfully with exact schema mapping.")
}
