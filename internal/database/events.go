package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EventType defines standardized UAV connection lifecycle events.
type EventType string

const (
	EventDiscovered EventType = "DISCOVERED"
	EventLost       EventType = "LOST"
	EventRestored   EventType = "RESTORED"
)

// RecordDeviceEvent inserts an audit event into the device_events hypertable.
func RecordDeviceEvent(
	ctx context.Context,
	pool *pgxpool.Pool,
	eventTime time.Time,
	deviceID string,
	eventType EventType,
	details any,
) error {
	if pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	var detailsJSON []byte
	var err error
	if details != nil {
		detailsJSON, err = json.Marshal(details)
		if err != nil {
			return fmt.Errorf("failed to marshal event details to JSON: %w", err)
		}
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO device_events (time, device_id, event_type, details)
		VALUES ($1, $2, $3, $4);
	`, eventTime.UTC(), deviceID, string(eventType), detailsJSON)
	if err != nil {
		return fmt.Errorf("failed to execute insert into device_events: %w", err)
	}

	return nil
}
