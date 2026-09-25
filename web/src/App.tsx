import { Header } from './features/layout/Header';
import { TacticalMap } from './features/map/TacticalMap';
import { useTelemetry } from './features/telemetry/useTelemetry';

export function App() {
  const { status, devices, selectedDeviceId, selectDevice } = useTelemetry();

  const activeCount = devices.filter((d) => d.status === 'ONLINE').length;

  return (
    <div className="flex flex-col h-screen w-screen bg-[#0b0f19] text-slate-100 overflow-hidden">
      <Header
        status={status}
        activeCount={activeCount}
        totalCount={devices.length}
      />
      <main className="flex-1 relative w-full h-[calc(100vh-3.5rem)]">
        <TacticalMap
          devices={devices}
          selectedDeviceId={selectedDeviceId}
          onSelectDevice={selectDevice}
        />
      </main>
    </div>
  );
}

export default App;
