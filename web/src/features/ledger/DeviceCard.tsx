import { type FC, useEffect, useRef } from 'react';
import type { DeviceTelemetryDTO } from '../../types/telemetry';
import {
  Activity,
  AlertTriangle,
  Battery,
  BatteryLow,
  BatteryWarning,
  Gauge,
  Navigation,
  ShieldAlert,
  Wifi,
  WifiOff,
} from 'lucide-react';
import { getDeviceSeverity, OperationalSeverity } from './priority';
import { formatLastSeen } from '../../utils/formatters';

interface DeviceCardProps {
  device: DeviceTelemetryDTO;
  isSelected: boolean;
  onSelect: (id: string) => void;
}

export const DeviceCard: FC<DeviceCardProps> = ({
  device,
  isSelected,
  onSelect,
}) => {
  const cardRef = useRef<HTMLDivElement>(null);
  const severity = getDeviceSeverity(device);

  // Auto-scroll card into view when selected from the map
  useEffect(() => {
    if (isSelected && cardRef.current) {
      cardRef.current.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }
  }, [isSelected]);

  const isOnline = device.status === 'ONLINE';
  const isCritical = severity === OperationalSeverity.CRITICAL;
  const isWarning = severity === OperationalSeverity.WARNING;

  // Determine battery color tokens
  const batteryPct = Math.round(device.battery_pct);
  let batteryColorClass = 'bg-emerald-500 text-emerald-400';
  let batteryBorderClass = 'border-emerald-500/30';
  let BatteryIcon = Battery;

  if (batteryPct < 15) {
    batteryColorClass = 'bg-rose-500 text-rose-400';
    batteryBorderClass = 'border-rose-500/50';
    BatteryIcon = BatteryLow;
  } else if (batteryPct < 25) {
    batteryColorClass = 'bg-amber-400 text-amber-300';
    batteryBorderClass = 'border-amber-400/40';
    BatteryIcon = BatteryWarning;
  }

  // Format last seen timestamp dynamically (e.g. LIVE, 15s ago, 2m 10s ago)
  const lastSeenStr = formatLastSeen(device.last_seen_ms, isOnline);

  return (
    <div
      ref={cardRef}
      onClick={() => onSelect(device.id)}
      className={`p-3 rounded-lg border transition-all cursor-pointer select-none text-xs font-mono relative overflow-hidden group ${
        isSelected
          ? 'bg-slate-900/90 border-cyan-400 ring-1 ring-cyan-400/70 shadow-lg shadow-cyan-500/10'
          : isCritical
          ? 'bg-rose-950/20 border-rose-600/50 hover:border-rose-500 hover:bg-rose-950/30'
          : isWarning
          ? 'bg-amber-950/15 border-amber-600/40 hover:border-amber-500 hover:bg-amber-950/25'
          : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 hover:bg-slate-900/70'
      }`}
    >
      {/* Top Banner Alert Indicator for Distress Units */}
      {isCritical && (
        <div className="flex items-center space-x-1 text-[10px] text-rose-300 font-bold bg-rose-500/20 px-2 py-0.5 -mx-3 -mt-3 mb-2 border-b border-rose-500/40">
          <ShieldAlert className="w-3 h-3 text-rose-400 animate-pulse" />
          <span>OPERATIONAL RISK: {isOnline ? 'CRITICAL BATTERY' : 'COMM LINK LOST'}</span>
        </div>
      )}
      {isWarning && isOnline && (
        <div className="flex items-center space-x-1 text-[10px] text-amber-300 font-bold bg-amber-500/15 px-2 py-0.5 -mx-3 -mt-3 mb-2 border-b border-amber-500/30">
          <AlertTriangle className="w-3 h-3 text-amber-400" />
          <span>OPERATIONAL WARNING: LOW BATTERY TIER</span>
        </div>
      )}

      {/* Card Header: Device ID, Link Status, Armed & Flight Mode */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center space-x-2">
          <span
            className={`w-2 h-2 rounded-full ${
              isOnline ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'
            }`}
          />
          <span
            className={`font-bold text-sm tracking-wide ${
              isSelected ? 'text-cyan-300' : 'text-slate-100 group-hover:text-cyan-400'
            }`}
          >
            {device.id}
          </span>
          {isOnline ? (
            <span className="flex items-center text-[10px] text-emerald-400 font-semibold bg-emerald-500/10 px-1.5 py-0.2 rounded border border-emerald-500/20">
              <Wifi className="w-2.5 h-2.5 mr-1" />
              ONLINE
            </span>
          ) : (
            <span className="flex items-center text-[10px] text-rose-400 font-semibold bg-rose-500/10 px-1.5 py-0.2 rounded border border-rose-500/20">
              <WifiOff className="w-2.5 h-2.5 mr-1" />
              LOST
            </span>
          )}
        </div>

        <div className="flex items-center space-x-1.5">
          <span
            className={`text-[9px] px-1.5 py-0.5 rounded font-bold ${
              !isOnline
                ? 'bg-slate-800/80 text-slate-500 border border-slate-700/50'
                : device.armed
                ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                : 'bg-slate-800 text-slate-400 border border-slate-700'
            }`}
          >
            {!isOnline ? 'ARMED: ?' : device.armed ? 'ARMED' : 'SAFE'}
          </span>
          <span
            className={`text-[9px] px-1.5 py-0.5 rounded font-semibold ${
              !isOnline
                ? 'bg-rose-950/40 text-rose-400/90 border border-rose-800/40'
                : 'bg-indigo-950/60 text-indigo-300 border border-indigo-700/40'
            }`}
          >
            {!isOnline ? 'LINK LOST' : device.flight_mode.replace('FLIGHT_MODE_', '')}
          </span>
        </div>
      </div>

      {/* Battery State-of-Charge Bar Gauge */}
      <div className={`mb-2 bg-slate-950/60 p-2 rounded border ${isOnline ? batteryBorderClass : 'border-slate-800 border-dashed'}`}>
        <div className="flex items-center justify-between text-[11px] mb-1">
          <div className="flex items-center space-x-1.5">
            <BatteryIcon className={`w-3.5 h-3.5 ${isOnline ? batteryColorClass.split(' ')[1] : 'text-slate-500'}`} />
            <span className="text-slate-400">BATTERY:</span>
            {isOnline ? (
              <span className={`font-bold ${batteryColorClass.split(' ')[1]}`}>
                {batteryPct}%
              </span>
            ) : (
              <span className="text-slate-500 font-bold flex items-center space-x-1">
                <span>? %</span>
                <span className="text-[9px] text-slate-600 font-normal">(STALE)</span>
              </span>
            )}
          </div>
          <div className="text-[10px] text-slate-500 space-x-2">
            {isOnline ? (
              <>
                <span>{device.battery_v.toFixed(1)}V</span>
                <span>|</span>
                <span>{device.battery_a.toFixed(1)}A</span>
                <span>|</span>
                <span>{device.battery_temp_c.toFixed(0)}&deg;C</span>
              </>
            ) : (
              <span>--- V | --- A | --- &deg;C</span>
            )}
          </div>
        </div>

        {/* Linear progress bar */}
        <div className="w-full h-1.5 bg-slate-800 rounded-full overflow-hidden">
          <div
            className={`h-full transition-all duration-300 ${isOnline ? batteryColorClass.split(' ')[0] : 'bg-slate-700/40'}`}
            style={{ width: isOnline ? `${Math.min(100, Math.max(0, batteryPct))}%` : '100%' }}
          />
        </div>
      </div>

      {/* Telemetry Metrics Grid: Altitude, Speed, Pos, Last Seen */}
      <div className="grid grid-cols-3 gap-1.5 text-[10px] bg-slate-950/40 p-1.5 rounded border border-slate-800/40">
        <div className="flex flex-col">
          <span className="text-slate-500 flex items-center space-x-1">
            <Navigation className="w-2.5 h-2.5 text-amber-400" />
            <span>ALTITUDE</span>
          </span>
          <span className={`font-semibold ${isOnline ? 'text-slate-200' : 'text-slate-500 italic'}`}>
            {isOnline ? `${device.alt.toFixed(1)} m` : '? m'}
          </span>
        </div>

        <div className="flex flex-col">
          <span className="text-slate-500 flex items-center space-x-1">
            <Gauge className="w-2.5 h-2.5 text-cyan-400" />
            <span>SPEED</span>
          </span>
          <span className={`font-semibold ${isOnline ? 'text-slate-200' : 'text-slate-500 italic'}`}>
            {isOnline ? `${device.speed.toFixed(1)} m/s` : '? m/s'}
          </span>
        </div>

        <div className="flex flex-col">
          <span className="text-slate-500 flex items-center space-x-1">
            <Activity className="w-2.5 h-2.5 text-slate-400" />
            <span>LAST SEEN</span>
          </span>
          <span
            className={`font-semibold flex items-center ${
              lastSeenStr === 'LIVE'
                ? 'text-emerald-400 font-bold'
                : isOnline
                ? 'text-slate-300'
                : 'text-rose-400 font-bold'
            }`}
          >
            {lastSeenStr === 'LIVE' && (
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse mr-1" />
            )}
            <span>{lastSeenStr}</span>
          </span>
        </div>
      </div>
    </div>
  );
};
