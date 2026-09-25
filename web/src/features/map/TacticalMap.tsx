import { useEffect, useRef, type FC } from 'react';
import { Map as MapLibreMap, NavigationControl, Marker, setWorkerUrl } from 'maplibre-gl';
import workerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url';
import type { DeviceTelemetryDTO } from '../../types/telemetry';
import { Compass, Crosshair } from 'lucide-react';

// Explicitly register MapLibre Web Worker URL for Vite bundler
setWorkerUrl(workerUrl);

interface TacticalMapProps {
  devices: DeviceTelemetryDTO[];
  selectedDeviceId: string | null;
  onSelectDevice: (id: string | null) => void;
}

const DEFAULT_CENTER: [number, number] = [-1.9812, 43.3183]; // San Sebastián / Donostia
const MAP_STYLE = 'https://basemaps.cartocdn.com/gl/dark-matter-gl-style/style.json';

export const TacticalMap: FC<TacticalMapProps> = ({
  devices,
  selectedDeviceId,
  onSelectDevice,
}) => {
  const mapContainerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markersRef = useRef<Map<string, { marker: Marker; el: HTMLElement }>>(new Map());

  // 1. Initialize MapLibre GL with 3D Perspective
  useEffect(() => {
    if (!mapContainerRef.current || mapRef.current) return;

    const map = new MapLibreMap({
      container: mapContainerRef.current,
      style: MAP_STYLE,
      center: DEFAULT_CENTER,
      zoom: 16.5,
      pitch: 52, // 3D perspective tilt
      bearing: -25, // Slight angle for tactical depth
      maxPitch: 80,
    });

    map.addControl(
      new NavigationControl({
        visualizePitch: true,
        showCompass: true,
        showZoom: true,
      }),
      'top-right'
    );

    mapRef.current = map;

    return () => {
      map.remove();
      mapRef.current = null;
    };
  }, []);

  // 2. Synchronize active drone markers with WebSocket telemetry
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    const currentDeviceIds = new Set<string>();

    for (const dev of devices) {
      if (!dev.lat || !dev.lon) continue;
      currentDeviceIds.add(dev.id);

      const isSelected = dev.id === selectedDeviceId;
      const isOnline = dev.status === 'ONLINE';

      let entry = markersRef.current.get(dev.id);

      if (!entry) {
        // Create marker container element
        const el = document.createElement('div');
        el.className = 'drone-marker-container cursor-pointer select-none';
        el.addEventListener('click', (e) => {
          e.stopPropagation();
          onSelectDevice(dev.id);
        });

        const marker = new Marker({
          element: el,
          anchor: 'center',
          rotationAlignment: 'map',
          pitchAlignment: 'map',
        })
          .setLngLat([dev.lon, dev.lat])
          .addTo(map);

        entry = { marker, el };
        markersRef.current.set(dev.id, entry);
      } else {
        // Smoothly update marker coordinates
        entry.marker.setLngLat([dev.lon, dev.lat]);
      }

      // Compute heading in degrees (Tait-Bryan yaw radians to degrees, 0 = North)
      // MapLibre uses clockwise degrees
      const yawDeg = (dev.yaw * 180) / Math.PI;

      // Update inner DOM markup with telemetry telemetry badge & orientation
      entry.el.innerHTML = `
        <div class="relative flex flex-col items-center group">
          <!-- Floating Info Pill -->
          <div class="mb-1.5 px-2 py-0.5 rounded bg-slate-900/90 border ${
            isSelected ? 'border-cyan-400 text-cyan-300 shadow-lg shadow-cyan-500/20' : 'border-slate-700 text-slate-200'
          } text-[10px] font-mono whitespace-nowrap backdrop-blur flex items-center space-x-1.5 pointer-events-none transition-all">
            <span class="w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'}"></span>
            <span class="font-bold">${dev.id}</span>
            <span class="text-slate-400">|</span>
            <span class="text-amber-300">${dev.alt.toFixed(1)}m</span>
            <span class="text-slate-400">|</span>
            <span class="${dev.battery_pct < 20 ? 'text-rose-400 font-bold' : 'text-emerald-300'}">${dev.battery_pct.toFixed(0)}%</span>
          </div>

          <!-- Rotating Tactical Drone Icon -->
          <div style="transform: rotate(${yawDeg}deg);" class="transition-transform duration-100 ease-out">
            <div class="w-9 h-9 rounded-full ${
              isSelected ? 'bg-cyan-500/20 ring-2 ring-cyan-400' : 'bg-indigo-600/30 ring-1 ring-indigo-400/60'
            } flex items-center justify-center relative shadow-md backdrop-blur">
              <!-- Heading Pointer Arrow -->
              <svg class="w-5 h-5 ${isSelected ? 'text-cyan-400' : 'text-indigo-300'} drop-shadow" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2L4.5 20.29l.71.71L12 18l6.79 3 .71-.71z" />
              </svg>
            </div>
          </div>
        </div>
      `;
    }

    // Clean up markers for devices no longer reported
    for (const [id, entry] of markersRef.current.entries()) {
      if (!currentDeviceIds.has(id)) {
        entry.marker.remove();
        markersRef.current.delete(id);
      }
    }
  }, [devices, selectedDeviceId, onSelectDevice]);

  // 3. Recenter camera on selected device
  const handleRecenter = () => {
    const map = mapRef.current;
    if (!map) return;

    if (selectedDeviceId) {
      const selected = devices.find((d) => d.id === selectedDeviceId);
      if (selected && selected.lat && selected.lon) {
        map.flyTo({
          center: [selected.lon, selected.lat],
          zoom: 17.5,
          pitch: 60,
          speed: 1.2,
        });
        return;
      }
    }

    // Default overview
    map.flyTo({
      center: DEFAULT_CENTER,
      zoom: 16.5,
      pitch: 52,
      bearing: -25,
      speed: 1.2,
    });
  };

  return (
    <div className="relative w-full h-[calc(100vh-3.5rem)] bg-[#0b0f19] overflow-hidden">
      {/* MapLibre WebGL Canvas Container */}
      <div ref={mapContainerRef} className="absolute inset-0 w-full h-full" />

      {/* Floating Tactical Overlay Controls (Bottom-Left) */}
      <div className="absolute bottom-5 left-5 z-10 flex flex-col space-y-2 pointer-events-auto">
        <div className="p-3 rounded-lg bg-slate-950/85 backdrop-blur border border-slate-800 text-xs font-mono shadow-xl text-slate-300 space-y-1">
          <div className="flex items-center space-x-2 text-cyan-400 font-bold border-b border-slate-800 pb-1">
            <Compass className="w-3.5 h-3.5" />
            <span>3D TACTICAL AIRSPACE</span>
          </div>
          <div className="flex justify-between space-x-4">
            <span className="text-slate-400">PITCH:</span>
            <span className="font-semibold text-slate-200">52&deg; (PERSPECTIVE)</span>
          </div>
          <div className="flex justify-between space-x-4">
            <span className="text-slate-400">DATUM:</span>
            <span className="font-semibold text-slate-200">WGS84 / EGM96</span>
          </div>
          <div className="flex justify-between space-x-4">
            <span className="text-slate-400">BASE:</span>
            <span className="font-semibold text-slate-200">SAN SEBASTIÁN, ES</span>
          </div>
        </div>

        {/* Action Button: Recenter on fleet */}
        <button
          onClick={handleRecenter}
          className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-indigo-600/90 hover:bg-indigo-500 text-white font-mono text-xs font-semibold shadow-lg shadow-indigo-600/30 border border-indigo-400/40 transition-all cursor-pointer active:scale-95"
        >
          <Crosshair className="w-4 h-4" />
          <span>{selectedDeviceId ? `FOCUS [${selectedDeviceId}]` : 'RESET TACTICAL VIEW'}</span>
        </button>
      </div>
    </div>
  );
};
