-- +goose Up
-- Add automated retention policies to avoid unbounded disk growth from high-frequency telemetry.
-- Drop telemetry hypertable chunks older than 3 days.
SELECT add_retention_policy('telemetry', INTERVAL '3 days', if_not_exists => TRUE);

-- Drop device audit and lifecycle events older than 14 days.
SELECT add_retention_policy('device_events', INTERVAL '14 days', if_not_exists => TRUE);

-- +goose Down
SELECT remove_retention_policy('device_events', if_exists => TRUE);
SELECT remove_retention_policy('telemetry', if_exists => TRUE);
