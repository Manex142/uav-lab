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

  interface MarkerEntry {
    marker: Marker;
    el: HTMLElement;
    arrowEl: HTMLElement;
    pillEl: HTMLElement;
    statusDotEl: HTMLElement;
    altSpan: HTMLElement;
    batSpan: HTMLElement;
    iconCircle: HTMLElement;
    arrowSvg: SVGElement;
    lastAngle: number;

    // LERP (Linear Interpolation) state for 60 FPS smooth motion
    fromLng: number;
    fromLat: number;
    targetLng: number;
    targetLat: number;
    currentLng: number;
    currentLat: number;
    startTime: number;
    durationMs: number;
  }

  const markersRef = useRef<Map<string, MarkerEntry>>(new Map());

  // 1. Initialize MapLibre GL 3D Map
  useEffect(() => {
    if (!mapContainerRef.current || mapRef.current) return;

    const map = new MapLibreMap({
      container: mapContainerRef.current,
      style: MAP_STYLE,
      center: DEFAULT_CENTER,
      zoom: 16.5,
      pitch: 52, // 3D Perspective angle
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

  // 2. 60 FPS LERP (Linear Interpolation) Animation Loop
  useEffect(() => {
    let animId: number;

    const animate = () => {
      const now = performance.now();

      for (const entry of markersRef.current.values()) {
        const elapsed = now - entry.startTime;
        const progress = Math.min(1.0, Math.max(0.0, elapsed / entry.durationMs));

        // LERP formula: P(t) = P0 + (P1 - P0) * t
        const lng = entry.fromLng + (entry.targetLng - entry.fromLng) * progress;
        const lat = entry.fromLat + (entry.targetLat - entry.fromLat) * progress;

        entry.currentLng = lng;
        entry.currentLat = lat;
        entry.marker.setLngLat([lng, lat]);
      }

      animId = requestAnimationFrame(animate);
    };

    animId = requestAnimationFrame(animate);

    return () => {
      cancelAnimationFrame(animId);
    };
  }, []);

  // 3. Synchronize active drone markers with WebSocket telemetry
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

        // 1. Info Pill
        const pillEl = document.createElement('div');
        pillEl.className = 'mb-1.5 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700 text-slate-200 text-[10px] font-mono whitespace-nowrap backdrop-blur flex items-center space-x-1.5 pointer-events-none transition-all';

        const statusDotEl = document.createElement('span');
        statusDotEl.className = 'w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse';

        const idSpan = document.createElement('span');
        idSpan.className = 'font-bold';
        idSpan.textContent = dev.id;

        const sep1 = document.createElement('span');
        sep1.className = 'text-slate-400';
        sep1.textContent = '|';

        const altSpan = document.createElement('span');
        altSpan.className = 'text-amber-300';
        altSpan.textContent = `${dev.alt.toFixed(1)}m`;

        const sep2 = document.createElement('span');
        sep2.className = 'text-slate-400';
        sep2.textContent = '|';

        const batSpan = document.createElement('span');
        batSpan.className = 'text-emerald-300';
        batSpan.textContent = `${dev.battery_pct.toFixed(0)}%`;

        pillEl.append(statusDotEl, idSpan, sep1, altSpan, sep2, batSpan);

        // 2. Rotating Arrow Container
        const arrowEl = document.createElement('div');
        arrowEl.className = 'transition-transform duration-100 ease-linear';

        const iconCircle = document.createElement('div');
        iconCircle.className = 'w-9 h-9 rounded-full bg-indigo-600/30 ring-1 ring-indigo-400/60 flex items-center justify-center relative shadow-md backdrop-blur';

        const arrowSvg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
        arrowSvg.setAttribute('class', 'w-5 h-5 text-indigo-300 drop-shadow');
        arrowSvg.setAttribute('viewBox', '0 0 24 24');
        arrowSvg.setAttribute('fill', 'currentColor');
        arrowSvg.innerHTML = '<path d="M12 2L4.5 20.29l.71.71L12 18l6.79 3 .71-.71z" />';

        iconCircle.appendChild(arrowSvg);
        arrowEl.appendChild(iconCircle);

        const wrapper = document.createElement('div');
        wrapper.className = 'relative flex flex-col items-center group';
        wrapper.append(pillEl, arrowEl);
        el.appendChild(wrapper);

        const marker = new Marker({
          element: el,
          anchor: 'center',
          rotationAlignment: 'map',
          pitchAlignment: 'map',
        })
          .setLngLat([dev.lon, dev.lat])
          .addTo(map);

        const now = performance.now();
        entry = {
          marker,
          el,
          arrowEl,
          pillEl,
          statusDotEl,
          altSpan,
          batSpan,
          iconCircle,
          arrowSvg,
          lastAngle: dev.yaw,
          fromLng: dev.lon,
          fromLat: dev.lat,
          targetLng: dev.lon,
          targetLat: dev.lat,
          currentLng: dev.lon,
          currentLat: dev.lat,
          startTime: now,
          durationMs: 100,
        };
        markersRef.current.set(dev.id, entry);
      } else {
        const now = performance.now();
        const elapsed = now - entry.startTime;
        // Dynamically estimate interval between updates (~100ms for 10 Hz)
        entry.durationMs = elapsed > 40 && elapsed < 500 ? elapsed : 100;

        // Teleport safeguard: snap immediately if coordinates jump > ~5km
        if (Math.hypot(dev.lon - entry.currentLng, dev.lat - entry.currentLat) > 0.05) {
          entry.currentLng = dev.lon;
          entry.currentLat = dev.lat;
        }

        entry.fromLng = entry.currentLng;
        entry.fromLat = entry.currentLat;
        entry.targetLng = dev.lon;
        entry.targetLat = dev.lat;
        entry.startTime = now;
      }

      // Angle Unwrapping: compute shortest angular distance to prevent 360-degree reverse spin
      const targetDeg = dev.yaw;
      let diff = (targetDeg - entry.lastAngle) % 360;
      diff = ((diff + 540) % 360) - 180;
      const continuousAngle = entry.lastAngle + diff;
      entry.lastAngle = continuousAngle;
      entry.arrowEl.style.transform = `rotate(${continuousAngle}deg)`;

      // Dynamic telemetry data updates without rebuilding DOM
      entry.altSpan.textContent = `${dev.alt.toFixed(1)}m`;
      entry.batSpan.textContent = `${dev.battery_pct.toFixed(0)}%`;
      entry.batSpan.className = dev.battery_pct < 20 ? 'text-rose-400 font-bold' : 'text-emerald-300';
      entry.statusDotEl.className = `w-1.5 h-1.5 rounded-full ${isOnline ? 'bg-emerald-400 animate-pulse' : 'bg-rose-500'}`;

      // Selection state styling
      if (isSelected) {
        entry.pillEl.className = 'mb-1.5 px-2 py-0.5 rounded bg-slate-900/90 border border-cyan-400 text-cyan-300 shadow-lg shadow-cyan-500/20 text-[10px] font-mono whitespace-nowrap backdrop-blur flex items-center space-x-1.5 pointer-events-none transition-all';
        entry.iconCircle.className = 'w-9 h-9 rounded-full bg-cyan-500/20 ring-2 ring-cyan-400 flex items-center justify-center relative shadow-md backdrop-blur';
        entry.arrowSvg.setAttribute('class', 'w-5 h-5 text-cyan-400 drop-shadow');
      } else {
        entry.pillEl.className = 'mb-1.5 px-2 py-0.5 rounded bg-slate-900/90 border border-slate-700 text-slate-200 text-[10px] font-mono whitespace-nowrap backdrop-blur flex items-center space-x-1.5 pointer-events-none transition-all';
        entry.iconCircle.className = 'w-9 h-9 rounded-full bg-indigo-600/30 ring-1 ring-indigo-400/60 flex items-center justify-center relative shadow-md backdrop-blur';
        entry.arrowSvg.setAttribute('class', 'w-5 h-5 text-indigo-300 drop-shadow');
      }
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
