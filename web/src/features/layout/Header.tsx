import { Plane, Radio, Shield, Sun, Moon } from "lucide-react";
import type { FC } from "react";
import type { SocketStatus } from "../telemetry/useTelemetry";

interface HeaderProps {
  status: SocketStatus;
  activeCount: number;
  totalCount: number;
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
}

export const Header: FC<HeaderProps> = ({
  status,
  activeCount,
  totalCount,
  theme,
  onToggleTheme,
}) => {
  return (
    <header className="h-14 bg-white/90 dark:bg-slate-950/90 backdrop-blur border-b border-slate-200 dark:border-slate-800 px-4 flex items-center justify-between z-20 select-none transition-colors duration-300">
      {/* Brand & System Title */}
      <div className="flex items-center space-x-3">
        <div className="w-8 h-8 rounded bg-gradient-to-br from-indigo-500 to-cyan-500 flex items-center justify-center shadow-lg shadow-indigo-500/20">
          <Plane className="w-5 h-5 text-white" />
        </div>
        <div>
          <div className="flex items-center space-x-2">
            <span className="font-mono font-bold tracking-wider text-slate-900 dark:text-slate-100 text-sm truncate max-w-[200px] sm:max-w-none block sm:inline">
              UAV LAB <span className="hidden sm:inline">// TACTICAL FLEET OPERATIONS</span>
            </span>
            <span className="text-[10px] font-mono px-1.5 py-0.5 rounded bg-indigo-100 dark:bg-indigo-500/20 text-indigo-700 dark:text-indigo-300 border border-indigo-200 dark:border-indigo-500/30 uppercase tracking-widest font-semibold hidden sm:inline-block">
              v1.0 SCADA
            </span>
          </div>
          <p className="text-[11px] text-slate-500 dark:text-slate-400 font-mono hidden sm:block">
            High-Throughput Cyber-Physical Gateway &bull; WGS84 Geodesy
          </p>
        </div>
      </div>

      {/* Center / Fleet Stats */}
      <div className="hidden md:flex items-center space-x-6">
        <div className="flex items-center space-x-2 text-xs font-mono">
          <span className="text-slate-500 dark:text-slate-400">FLEET ACTIVE:</span>
          <span className="font-bold text-cyan-700 dark:text-cyan-400 bg-cyan-100 dark:bg-cyan-950/40 px-2 py-0.5 rounded border border-cyan-200 dark:border-cyan-800/40">
            {activeCount} / {totalCount} UNITS
          </span>
        </div>

        <div className="flex items-center space-x-2 text-xs font-mono">
          <span className="text-slate-500 dark:text-slate-400">STREAM RATE:</span>
          <span className="font-bold text-amber-700 dark:text-amber-400 bg-amber-100 dark:bg-amber-950/40 px-2 py-0.5 rounded border border-amber-200 dark:border-amber-800/40">
            10.0 HZ
          </span>
        </div>

        <div className="flex items-center space-x-1.5 text-xs font-mono text-slate-500 dark:text-slate-400">
          <Shield className="w-3.5 h-3.5 text-emerald-600 dark:text-emerald-400" />
          <span>AIRSPACE NOMINAL</span>
        </div>
      </div>

      {/* Right / Connection Status Indicator */}
      <div className="flex items-center space-x-3">
        <button
          onClick={onToggleTheme}
          className="p-1.5 rounded bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-slate-500 dark:text-slate-400 hover:text-cyan-600 dark:hover:text-cyan-400 transition-colors"
          title="Toggle Theme"
        >
          {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
        </button>

        <div className="flex items-center space-x-2 px-2.5 py-1 rounded bg-slate-100 dark:bg-slate-900 border border-slate-200 dark:border-slate-800 text-xs font-mono">
          <Radio className="w-3.5 h-3.5 text-slate-500 dark:text-slate-400" />
          {status === "CONNECTED" ? (
            <span className="flex items-center text-emerald-600 dark:text-emerald-400 font-semibold">
              <span className="w-2 h-2 rounded-full bg-emerald-500 dark:bg-emerald-400 animate-pulse mr-1.5" />
              WS ONLINE
            </span>
          ) : status === "CONNECTING" ? (
            <span className="flex items-center text-amber-600 dark:text-amber-400 font-semibold">
              <span className="w-2 h-2 rounded-full bg-amber-500 dark:bg-amber-400 animate-ping mr-1.5" />
              CONNECTING
            </span>
          ) : (
            <span className="flex items-center text-rose-600 dark:text-rose-400 font-semibold">
              <span className="w-2 h-2 rounded-full bg-rose-600 dark:bg-rose-500 mr-1.5" />
              DISCONNECTED
            </span>
          )}
        </div>
      </div>
    </header>
  );
};
