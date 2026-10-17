import { AppState } from 'react-native';

import { post } from '@/api/client';
import { API_URL } from '@/lib/config';

type Handler = (data: any) => void;

const handlers = new Map<string, Set<Handler>>();
let ws: WebSocket | null = null;
let attempts = 0;
let timer: ReturnType<typeof setTimeout> | null = null;
let wanted = false;
let connected = false;
const statusListeners = new Set<(up: boolean) => void>();

export function on(type: string, fn: Handler) {
  if (!handlers.has(type)) handlers.set(type, new Set());
  handlers.get(type)!.add(fn);
  return () => {
    handlers.get(type)?.delete(fn);
  };
}

export function onStatus(fn: (up: boolean) => void) {
  statusListeners.add(fn);
  return () => {
    statusListeners.delete(fn);
  };
}

export function isConnected() {
  return connected;
}

function setConnected(v: boolean) {
  connected = v;
  statusListeners.forEach((l) => l(v));
}

function emit(type: string, data: unknown) {
  handlers.get(type)?.forEach((h) => h(data));
  handlers.get('*')?.forEach((h) => h({ type, data }));
}

function schedule() {
  if (!wanted || timer) return;
  // Exponential backoff with jitter, capped at 30s, so a metro tunnel doesn't hammer the server.
  const delay = Math.min(30_000, 1000 * 2 ** Math.min(attempts, 5)) * (0.5 + Math.random() / 2);
  attempts++;
  timer = setTimeout(() => {
    timer = null;
    open();
  }, delay);
}

async function open() {
  if (!wanted || ws) return;
  try {
    const { ticket } = await post<{ ticket: string }>('/v1/ws/ticket');
    const sock = new WebSocket(`${API_URL.replace(/^http/, 'ws')}/ws?ticket=${ticket}`);
    ws = sock;
    sock.onmessage = (e) => {
      try {
        const msg = JSON.parse(String(e.data));
        if (msg.type === 'hello') {
          attempts = 0;
          setConnected(true);
        }
        emit(msg.type, msg.data);
      } catch {
        // ignore malformed frames
      }
    };
    sock.onclose = () => {
      if (ws === sock) ws = null;
      setConnected(false);
      schedule();
    };
    sock.onerror = () => sock.close();
  } catch {
    ws = null;
    schedule();
  }
}

export function send(type: string, data: unknown) {
  if (ws && connected) ws.send(JSON.stringify({ type, data }));
}

export function startRealtime() {
  wanted = true;
  attempts = 0;
  open();
}

export function stopRealtime() {
  wanted = false;
  if (timer) clearTimeout(timer);
  timer = null;
  ws?.close();
  ws = null;
  setConnected(false);
}

// Sockets are closed in the background (the OS would kill them anyway) and reopened on return.
AppState.addEventListener('change', (s) => {
  if (!wanted) return;
  if (s === 'active') {
    attempts = 0;
    open();
  } else if (s === 'background') {
    ws?.close();
  }
});
