package database

import (
	"context"
	"testing"
	"time"
)

func TestDatabaseConnectionAndMigrations(t *testing.T) {
	cfg := DefaultConfig()

	// 1. Run Migrations
	t.Log("Applying goose migrations...")
	if err := RunMigrations(cfg.DSN()); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}
	t.Log("Migrations applied successfully.")

	// 2. Connect via pgxpool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect to pool: %v", err)
	}
	defer pool.Close()

	// 3. Verify Hypertables in TimescaleDB catalog
	rows, err := pool.Query(ctx, `
		SELECT hypertable_name, num_dimensions 
		FROM timescaledb_information.hypertables 
		WHERE hypertable_name IN ('telemetry', 'device_events')
		ORDER BY hypertable_name;
	`)
	if err != nil {
		t.Fatalf("Failed to query hypertables: %v", err)
	}
	defer rows.Close()

	foundHypertables := make(map[string]int)
	for rows.Next() {
		var name string
		var dims int
		if err := rows.Scan(&name, &dims); err != nil {
			t.Fatalf("Failed to scan hypertable row: %v", err)
		}
		foundHypertables[name] = dims
	}

	if _, ok := foundHypertables["telemetry"]; !ok {
		t.Errorf("Expected 'telemetry' to be registered as a TimescaleDB hypertable")
	}
	if _, ok := foundHypertables["device_events"]; !ok {
		t.Errorf("Expected 'device_events' to be registered as a TimescaleDB hypertable")
	}

	t.Logf("Verified TimescaleDB hypertables: %+v", foundHypertables)

	// 4. Verify compound index
	var indexCount int
	err = pool.QueryRow(ctx, `
		SELECT count(*) 
		FROM pg_indexes 
		WHERE tablename = 'telemetry' AND indexname = 'idx_telemetry_device_time';
	`).Scan(&indexCount)
	if err != nil {
		t.Fatalf("Failed to check index: %v", err)
	}
	if indexCount != 1 {
		t.Errorf("Expected 1 index named 'idx_telemetry_device_time', found %d", indexCount)
	}
}
