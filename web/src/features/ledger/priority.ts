import type { DeviceTelemetryDTO } from '../../types/telemetry';

export const OperationalSeverity = {
  CRITICAL: 1, // OFFLINE/LOST communication or critical battery (< 15%)
  WARNING: 2,  // Low battery (< 25%) or degraded
  ADVISORY: 3, // Armed but inactive or low altitude
  NOMINAL: 4,  // Healthy cruise flight
} as const;

export type OperationalSeverity = (typeof OperationalSeverity)[keyof typeof OperationalSeverity];

/**
 * Evaluates the operational risk tier of a given asset.
 */
export function getDeviceSeverity(device: DeviceTelemetryDTO): OperationalSeverity {
  if (device.status === 'OFFLINE' || (device.status as string) === 'LOST') {
    return OperationalSeverity.CRITICAL;
  }
  if (device.battery_pct < 15) {
    return OperationalSeverity.CRITICAL;
  }
  if (device.battery_pct < 25) {
    return OperationalSeverity.WARNING;
  }
  return OperationalSeverity.NOMINAL;
}

/**
 * Sorts devices dynamically by operational risk priority:
 * 1. Critical assets (LOST, critical battery) bubble to the top.
 * 2. Warning assets (low battery) follow.
 * 3. Nominal assets sorted by battery then ID.
 */
export function sortDevicesByPriority(devices: DeviceTelemetryDTO[]): DeviceTelemetryDTO[] {
  return [...devices].sort((a, b) => {
    const sevA = getDeviceSeverity(a);
    const sevB = getDeviceSeverity(b);

    if (sevA !== sevB) {
      return sevA - sevB; // Lower severity enum value = higher visual urgency
    }

    // Secondary sort: lower battery percentage first
    if (Math.abs(a.battery_pct - b.battery_pct) > 2) {
      return a.battery_pct - b.battery_pct;
    }

    // Fallback: natural alphanumeric ID order
    return a.id.localeCompare(b.id);
  });
}
