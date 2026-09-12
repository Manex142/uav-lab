-- +goose Up
-- Enable TimescaleDB extension if not already present
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

-- 1. Telemetry Timeseries Table
CREATE TABLE IF NOT EXISTS telemetry (
    time             TIMESTAMPTZ      NOT NULL,
    device_id        VARCHAR(64)      NOT NULL,
    sequence_number  BIGINT           NOT NULL,
    
    -- Kinematics / Position (m)
    pos_x            DOUBLE PRECISION,
    pos_y            DOUBLE PRECISION,
    pos_z            DOUBLE PRECISION,
    
    -- Attitude Quaternion
    rot_w            DOUBLE PRECISION,
    rot_x            DOUBLE PRECISION,
    rot_y            DOUBLE PRECISION,
    rot_z            DOUBLE PRECISION,
    
    -- Attitude Euler Angles (rad)
    roll             DOUBLE PRECISION,
    pitch            DOUBLE PRECISION,
    yaw              DOUBLE PRECISION,
    
    -- Velocities
    vel_x            DOUBLE PRECISION,
    vel_y            DOUBLE PRECISION,
    vel_z            DOUBLE PRECISION,
    ang_vel_x        DOUBLE PRECISION,
    ang_vel_y        DOUBLE PRECISION,
    ang_vel_z        DOUBLE PRECISION,
    
    -- Autopilot State & Power
    armed            BOOLEAN          NOT NULL DEFAULT FALSE,
    flight_mode      VARCHAR(32),
    system_status    VARCHAR(32),
    battery_voltage  REAL,
    battery_current  REAL,
    battery_pct      REAL,
    battery_temp     REAL
);

-- Convert to TimescaleDB Hypertable partitioned by time (2-hour chunks)
SELECT create_hypertable(
    'telemetry',
    'time',
    chunk_time_interval => INTERVAL '2 hours',
    if_not_exists => TRUE
);

-- Compound index for fast temporal range filtering per UAV device
CREATE INDEX IF NOT EXISTS idx_telemetry_device_time ON telemetry (device_id, time DESC);

-- 2. Device Lifecycle & Audit Events Table
CREATE TABLE IF NOT EXISTS device_events (
    time        TIMESTAMPTZ NOT NULL,
    device_id   VARCHAR(64) NOT NULL,
    event_type  VARCHAR(32) NOT NULL,
    details     JSONB
);

-- Convert to Hypertable partitioned by time (1-day chunks)
SELECT create_hypertable(
    'device_events',
    'time',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_device_events_device_time ON device_events (device_id, time DESC);

-- +goose Down
DROP TABLE IF EXISTS device_events;
DROP TABLE IF EXISTS telemetry;
