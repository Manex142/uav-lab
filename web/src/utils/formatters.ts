/**
 * Formats an elapsed millisecond duration into a clean, human-readable relative time string.
 *
 * Rules:
 * - Active online stream (< 1.5s): "LIVE" (rock-solid, no clock jitter or negative ms flicker)
 * - Under 1 min (< 60s): e.g. "14s ago"
 * - Under 1 hour (< 60m): e.g. "2m 15s ago"
 * - Under 1 day (< 24h): e.g. "1h 22m ago"
 * - 1 day or more: e.g. "2d 4h ago"
 */
export function formatLastSeen(ms: number, isOnline = true): string {
  // If asset is actively streaming online, any recent timestamp is LIVE
  if (isOnline && ms < 1500) {
    return 'LIVE';
  }

  const nonNegativeMs = Math.max(0, ms);
  const totalSeconds = Math.floor(nonNegativeMs / 1000);

  if (totalSeconds < 1) {
    return isOnline ? 'LIVE' : 'Just now';
  }

  if (totalSeconds < 60) {
    return `${totalSeconds}s ago`;
  }

  const minutes = Math.floor(totalSeconds / 60);
  const remSeconds = totalSeconds % 60;

  if (minutes < 60) {
    return remSeconds > 0 ? `${minutes}m ${remSeconds}s ago` : `${minutes}m ago`;
  }

  const hours = Math.floor(minutes / 60);
  const remMinutes = minutes % 60;

  if (hours < 24) {
    return remMinutes > 0 ? `${hours}h ${remMinutes}m ago` : `${hours}h ago`;
  }

  const days = Math.floor(hours / 24);
  const remHours = hours % 24;

  return remHours > 0 ? `${days}d ${remHours}h ago` : `${days}d ago`;
}
