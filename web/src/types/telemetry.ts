export type ConnectionStatus = 'ONLINE' | 'OFFLINE' | 'LOST';

export interface DeviceTelemetryDTO {
  id: string;
  status: ConnectionStatus;
  seq: number;
  lat: number;
  lon: number;
  alt: number;
  pos_x: number;
  pos_y: number;
  pos_z: number;
  yaw: number;
  pitch: number;
  roll: number;
  speed: number;
  armed: boolean;
  flight_mode: string;
  battery_pct: number;
  battery_v: number;
  battery_a: number;
  battery_temp_c: number;
  last_seen_ms: number;
}

export interface FleetSnapshotMessage {
  type: 'FLEET_SNAPSHOT';
  timestamp: number;
  devices: DeviceTelemetryDTO[];
}

export interface LifecycleEventMessage {
  type: 'LIFECYCLE_EVENT';
  timestamp: number;
  event: 'DISCOVERED' | 'LOST' | 'RESTORED' | string;
  device_id: string;
  details?: string;
}

export type WebSocketMessage = FleetSnapshotMessage | LifecycleEventMessage;

export interface GatewayHealth {
  status: string;
  uptime_sec: number;
  active_drones: number;
  ws_clients: number;
  packets_recv: number;
  bytes_recv: number;
  avg_latency_ms: number;
  dropped_packets: number;
  decode_errors: number;
}
