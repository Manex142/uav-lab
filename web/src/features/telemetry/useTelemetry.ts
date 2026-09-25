import { useEffect, useRef, useState, useCallback } from 'react';
import type { DeviceTelemetryDTO, WebSocketMessage, LifecycleEventMessage } from '../../types/telemetry';

export type SocketStatus = 'CONNECTING' | 'CONNECTED' | 'DISCONNECTED';

const WS_URL = 'ws://localhost:8080/ws/telemetry';

export function useTelemetry() {
  const [status, setStatus] = useState<SocketStatus>('CONNECTING');
  const [devices, setDevices] = useState<DeviceTelemetryDTO[]>([]);
  const [events, setEvents] = useState<LifecycleEventMessage[]>([]);
  const [selectedDeviceId, setSelectedDeviceId] = useState<string | null>(null);

  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptRef = useRef<number>(0);

  const connect = useCallback(() => {
    if (socketRef.current?.readyState === WebSocket.OPEN) return;

    setStatus('CONNECTING');
    const ws = new WebSocket(WS_URL);
    socketRef.current = ws;

    ws.onopen = () => {
      setStatus('CONNECTED');
      reconnectAttemptRef.current = 0;
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as WebSocketMessage;

        if (msg.type === 'FLEET_SNAPSHOT') {
          setDevices(msg.devices);
        } else if (msg.type === 'LIFECYCLE_EVENT') {
          setEvents((prev) => [msg, ...prev.slice(0, 49)]); // Keep last 50 events
        }
      } catch (err) {
        console.error('[WebSocket] Error parsing message:', err);
      }
    };

    ws.onerror = () => {
      ws.close();
    };

    ws.onclose = () => {
      setStatus('DISCONNECTED');
      socketRef.current = null;

      // Exponential backoff reconnect: 1s, 2s, 4s, capped at 5s
      const delay = Math.min(1000 * Math.pow(2, reconnectAttemptRef.current), 5000);
      reconnectAttemptRef.current += 1;

      reconnectTimeoutRef.current = window.setTimeout(() => {
        connect();
      }, delay);
    };
  }, []);

  useEffect(() => {
    connect();

    return () => {
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
      if (socketRef.current) {
        socketRef.current.close();
      }
    };
  }, [connect]);

  const selectDevice = useCallback((id: string | null) => {
    setSelectedDeviceId(id);
  }, []);

  return {
    status,
    devices,
    events,
    selectedDeviceId,
    selectDevice,
  };
}
