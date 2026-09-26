import { type FC, useMemo, useState } from 'react';
import type { DeviceTelemetryDTO } from '../../types/telemetry';
import { DeviceCard } from './DeviceCard';
import { getDeviceSeverity, OperationalSeverity, sortDevicesByPriority } from './priority';
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Filter,
  Layers,
  Search,
  ShieldCheck,
  X,
} from 'lucide-react';

interface FleetLedgerProps {
  devices: DeviceTelemetryDTO[];
  selectedDeviceId: string | null;
  onSelectDevice: (id: string | null) => void;
  isOpen: boolean;
  onToggleOpen: () => void;
}

type StatusFilter = 'ALL' | 'ONLINE' | 'ALERTS';

export const FleetLedger: FC<FleetLedgerProps> = ({
  devices,
  selectedDeviceId,
  onSelectDevice,
  isOpen,
  onToggleOpen,
}) => {
  const [searchQuery, setSearchQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('ALL');

  // Compute fleet-wide health counters
  const { totalCount, onlineCount, alertCount } = useMemo(() => {
    let online = 0;
    let alerts = 0;

    for (const dev of devices) {
      if (dev.status === 'ONLINE') online++;
      const sev = getDeviceSeverity(dev);
      if (sev === OperationalSeverity.CRITICAL || sev === OperationalSeverity.WARNING) {
        alerts++;
      }
    }

    return {
      totalCount: devices.length,
      onlineCount: online,
      alertCount: alerts,
    };
  }, [devices]);

  // Filter and sort devices dynamically
  const displayedDevices = useMemo(() => {
    let result = devices;

    // 1. Text Search Filter (Asset ID)
    if (searchQuery.trim() !== '') {
      const q = searchQuery.toLowerCase().trim();
      result = result.filter((d) => d.id.toLowerCase().includes(q));
    }

    // 2. Operational Status Filter
    if (statusFilter === 'ONLINE') {
      result = result.filter((d) => d.status === 'ONLINE');
    } else if (statusFilter === 'ALERTS') {
      result = result.filter((d) => {
        const sev = getDeviceSeverity(d);
        return sev === OperationalSeverity.CRITICAL || sev === OperationalSeverity.WARNING;
      });
    }

    // 3. Priority Risk Sorting (Bubbles distressed units to the top)
    return sortDevicesByPriority(result);
  }, [devices, searchQuery, statusFilter]);

  return (
    <aside
      className={`relative z-20 flex flex-col h-[calc(100vh-3.5rem)] bg-slate-950/95 backdrop-blur-md border-r border-slate-800 transition-all duration-300 ease-in-out select-none ${
        isOpen ? 'w-80 sm:w-96' : 'w-0'
      }`}
    >
      {/* Floating Sidebar Toggle Button */}
      <button
        onClick={onToggleOpen}
        title={isOpen ? 'Collapse Fleet Ledger' : 'Expand Fleet Ledger'}
        className="absolute -right-9 top-4 z-30 p-2 rounded-r-md bg-slate-900/90 hover:bg-slate-800 text-slate-300 hover:text-cyan-400 border-y border-r border-slate-800 shadow-xl cursor-pointer transition-colors backdrop-blur active:scale-95"
      >
        {isOpen ? <ChevronLeft className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
      </button>

      {/* Sidebar Content (Hidden smoothly when collapsed) */}
      <div
        className={`flex-1 flex flex-col h-full overflow-hidden transition-opacity duration-200 ${
          isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        }`}
      >
        {/* Header Title & Health Metric Badges */}
        <div className="p-3.5 border-b border-slate-800/80 space-y-2.5">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2 text-cyan-400 font-bold font-mono text-xs tracking-wider">
              <Layers className="w-4 h-4" />
              <span>FLEET LEDGER</span>
            </div>
            {selectedDeviceId && (
              <button
                onClick={() => onSelectDevice(null)}
                className="flex items-center space-x-1 text-[10px] font-mono text-slate-400 hover:text-rose-400 transition-colors"
                title="Deselect active asset"
              >
                <span>CLEAR FOCUS</span>
                <X className="w-3 h-3" />
              </button>
            )}
          </div>

          {/* Fleet Health Counter Chips */}
          <div className="grid grid-cols-3 gap-1.5 font-mono text-[10px]">
            <div className="p-1.5 rounded bg-slate-900/80 border border-slate-800 flex flex-col items-center">
              <span className="text-slate-400">TOTAL</span>
              <span className="font-bold text-slate-100">{totalCount}</span>
            </div>
            <div className="p-1.5 rounded bg-emerald-950/30 border border-emerald-500/20 flex flex-col items-center">
              <span className="text-emerald-400 flex items-center space-x-1">
                <ShieldCheck className="w-2.5 h-2.5" />
                <span>ONLINE</span>
              </span>
              <span className="font-bold text-emerald-300">{onlineCount}</span>
            </div>
            <div className="p-1.5 rounded bg-rose-950/30 border border-rose-500/20 flex flex-col items-center">
              <span className="text-rose-400 flex items-center space-x-1">
                <AlertTriangle className="w-2.5 h-2.5" />
                <span>ALERTS</span>
              </span>
              <span className="font-bold text-rose-300">{alertCount}</span>
            </div>
          </div>

          {/* Search Box */}
          <div className="relative">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500" />
            <input
              type="text"
              placeholder="Search by asset ID (e.g. uav-005)..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-slate-900/80 border border-slate-800 focus:border-cyan-500/60 rounded-md pl-8 pr-7 py-1.5 text-xs font-mono text-slate-200 placeholder-slate-500 outline-none transition-all"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Status Filter Tabs */}
          <div className="flex items-center space-x-1 font-mono text-[10px]">
            <Filter className="w-3 h-3 text-slate-500 mr-1" />
            {(['ALL', 'ONLINE', 'ALERTS'] as StatusFilter[]).map((tab) => (
              <button
                key={tab}
                onClick={() => setStatusFilter(tab)}
                className={`flex-1 py-1 rounded border transition-all cursor-pointer font-semibold ${
                  statusFilter === tab
                    ? 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40 shadow-sm shadow-cyan-500/10'
                    : 'bg-slate-900/50 text-slate-400 border-slate-800 hover:bg-slate-800 hover:text-slate-200'
                }`}
              >
                {tab}
              </button>
            ))}
          </div>
        </div>

        {/* Scrollable Device Cards Catalog */}
        <div className="flex-1 overflow-y-auto p-3 space-y-2.5 scrollbar-thin scrollbar-thumb-slate-800 scrollbar-track-transparent">
          {displayedDevices.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-48 text-slate-500 font-mono text-xs space-y-2">
              <Search className="w-6 h-6 text-slate-600" />
              <span>No assets match operational query</span>
              {searchQuery && (
                <button
                  onClick={() => setSearchQuery('')}
                  className="text-cyan-400 hover:underline text-[11px]"
                >
                  Clear search query
                </button>
              )}
            </div>
          ) : (
            displayedDevices.map((dev) => (
              <DeviceCard
                key={dev.id}
                device={dev}
                isSelected={dev.id === selectedDeviceId}
                onSelect={(id) => onSelectDevice(id)}
              />
            ))
          )}
        </div>
      </div>
    </aside>
  );
};
