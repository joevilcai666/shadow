import { useEffect, useRef } from 'react';

const REFRESH_EVENTS = ['rule.', 'source.created'];

export function useRealtimeRefresh(refresh: () => void) {
  const refreshRef = useRef(refresh);

  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const socket = new WebSocket(`${protocol}//${window.location.host}/ws`);

    socket.onmessage = (event) => {
      try {
        const message = JSON.parse(event.data);
        const name = String(message.event ?? '');
        if (REFRESH_EVENTS.some(prefix => name.startsWith(prefix))) {
          refreshRef.current();
        }
      } catch {
        // Ignore malformed websocket messages from older daemons.
      }
    };

    return () => socket.close();
  }, []);
}
