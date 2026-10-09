import { useState, useEffect } from 'react';
import { Header } from './features/layout/Header';
import { TacticalMap } from './features/map/TacticalMap';
import { FleetLedger } from './features/ledger/FleetLedger';
import { useTelemetry } from './features/telemetry/useTelemetry';

export function App() {
  const { status, devices, selectedDeviceId, selectDevice } = useTelemetry();
  const [isLedgerOpen, setIsLedgerOpen] = useState(true);
  
  const [theme, setTheme] = useState<'dark' | 'light'>(() => {
    const saved = localStorage.getItem('uav-theme');
    return (saved === 'dark' || saved === 'light') ? saved : 'dark';
  });

  useEffect(() => {
    localStorage.setItem('uav-theme', theme);
  }, [theme]);

  const activeCount = devices.filter((d) => d.status === 'ONLINE').length;

  return (
    <div className={`flex flex-col h-screen w-screen overflow-hidden font-sans transition-colors duration-300 ${theme === 'dark' ? 'dark' : ''} bg-slate-50 dark:bg-[#0b0f19] text-slate-900 dark:text-slate-100`}>
      <Header
        status={status}
        activeCount={activeCount}
        totalCount={devices.length}
        theme={theme}
        onToggleTheme={() => setTheme(t => t === 'dark' ? 'light' : 'dark')}
      />
      <div className="flex-1 relative flex w-full h-[calc(100vh-3.5rem)] overflow-hidden">
        {/* Left: Collapsible Prioritized Fleet Ledger */}
        <FleetLedger
          devices={devices}
          selectedDeviceId={selectedDeviceId}
          onSelectDevice={selectDevice}
          isOpen={isLedgerOpen}
          onToggleOpen={() => setIsLedgerOpen((prev) => !prev)}
        />

        {/* Right / Center: 3D Tactical Airspace Map */}
        <main className="flex-1 relative h-full">
          <TacticalMap
            devices={devices}
            selectedDeviceId={selectedDeviceId}
            onSelectDevice={selectDevice}
            theme={theme}
          />
        </main>
      </div>
    </div>
  );
}

export default App;
