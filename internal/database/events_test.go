package database

import (
	"context"
	"testing"
	"time"
)

func TestRecordDeviceEvent_Persistence(t *testing.T) {
	cfg := DefaultConfig()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := NewPool(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to connect pool: %v", err)
	}
	defer pool.Close()

	// Clear device_events for this test
	_, err = pool.Exec(ctx, "DELETE FROM device_events WHERE device_id = 'uav-event-test';")
	if err != nil {
		t.Fatalf("Failed to clean device_events: %v", err)
	}

	// 1. Record DISCOVERED event
	now := time.Now()
	err = RecordDeviceEvent(ctx, pool, now, "uav-event-test", EventDiscovered, map[string]any{
		"ip":   "127.0.0.1:9876",
		"mode": "offboard",
	})
	if err != nil {
		t.Fatalf("Failed to record DISCOVERED event: %v", err)
	}

	// 2. Record LOST event
	lostTime := now.Add(5 * time.Second)
	err = RecordDeviceEvent(ctx, pool, lostTime, "uav-event-test", EventLost, map[string]any{
		"reason": "heartbeat_timeout",
	})
	if err != nil {
		t.Fatalf("Failed to record LOST event: %v", err)
	}

	// 3. Verify rows in database
	rows, err := pool.Query(ctx, `
		SELECT event_type, details->>'reason' 
		FROM device_events 
		WHERE device_id = 'uav-event-test' 
		ORDER BY time ASC;
	`)
	if err != nil {
		t.Fatalf("Failed to query device_events: %v", err)
	}
	defer rows.Close()

	var events []string
	for rows.Next() {
		var et string
		var reason *string
		if err := rows.Scan(&et, &reason); err != nil {
			t.Fatalf("Failed to scan event row: %v", err)
		}
		events = append(events, et)
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0] != "DISCOVERED" || events[1] != "LOST" {
		t.Errorf("Unexpected event types: %+v", events)
	}
}
