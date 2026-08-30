import { signal } from "@preact/signals-core";

import { getAuthMe } from "@/api";

/**
 * Global WebSocket state.
 *
 * The WebSocket is connected once at the app level (app-shell.ts) when the
 * user is authenticated, and stays connected across page navigations.
 *
 * Features subscribe to messages by reading `wsClient` and adding their own
 * `message` event listeners. When the socket reconnects, `wsClient` changes,
 * and features re-add listeners via their signal-watched effects.
 *
 * Reconnect handling:
 * - Retries indefinitely. Exponential backoff grows to MAX_RECONNECT_DELAY and
 *   stays there (with jitter); MAX_RECONNECT_ATTEMPTS is not a hard stop;
 *   reaching it resets the counter so retries continue at the capped delay.
 * - Before each reconnect attempt the client probes /api/auth/me; a 401 means
 *   the session is gone, so we stop reconnecting and trigger logout().
 */

export type ConnectionStatus = "disconnected" | "connecting" | "connected";

export const wsClient = signal<WebSocket | null>(null);
export const connectionStatus = signal<ConnectionStatus>("disconnected");
export const wsUserId = signal<string | null>(null);

let ws: WebSocket | null = null;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let reconnectAttempts = 0;
let isActive = false;
const MAX_RECONNECT_DELAY = 30000;
const MAX_RECONNECT_ATTEMPTS = 10;

function getWsUrl(): string {
  const raw = import.meta.env.VITE_API_BASE_URL;
  if (raw) {
    const url = new URL(raw);
    const proto = url.protocol === "https:" ? "wss:" : "ws:";
    return `${proto}//${url.host}/api/ws`;
  }
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${proto}//${window.location.host}/api/ws`;
}

function getReconnectDelay(): number {
  const base = Math.min(
    1000 * Math.pow(2, reconnectAttempts),
    MAX_RECONNECT_DELAY,
  );
  return base + Math.random() * 1000;
}

/** Probe the session before reconnecting. Returns false on an auth error. */
async function isSessionValid(): Promise<boolean> {
  try {
    const { error } = await getAuthMe();
    return !error;
  } catch {
    // Network error: assume still valid and let the reconnect proceed.
    return true;
  }
}

/** Notifies the app that the session is gone so it can log out + redirect. */
function onSessionLost(): void {
  connectionStatus.value = "disconnected";
  wsClient.value = null;
  ws = null;
  isActive = false;
  // Defer to break the import cycle with the auth store: dispatch a custom
  // event the app listens for to reset auth state and redirect to /login.
  window.dispatchEvent(new CustomEvent("breeze:session-expired"));
}

let isSchedulingReconnect = false;

export function connectWs(): void {
  if (
    isActive && ws &&
    (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)
  ) {
    return;
  }
  // Guard against overlapping connect attempts racing with scheduleReconnect's
  // async auth probe: two near-simultaneous onclose events would otherwise
  // launch two probes + two timers. The flag serializes them.
  if (isSchedulingReconnect) return;
  isActive = true;
  connectionStatus.value = "connecting";

  const socket = new WebSocket(getWsUrl());
  ws = socket;

  socket.onopen = () => {
    // Guard against a stale socket arriving after a newer one was opened.
    if (!isActive || ws !== socket) {
      socket.close();
      return;
    }
    reconnectAttempts = 0;
    connectionStatus.value = "connected";
    wsClient.value = socket;
  };

  socket.onmessage = (event: MessageEvent) => {
    try {
      const data = JSON.parse(event.data);
      if (data?.type === "connected" && data?.payload?.user_id) {
        wsUserId.value = data.payload.user_id;
      }
    } catch {
      // ignore malformed messages
    }
  };

  socket.onclose = () => {
    // Only react if this is still the active socket.
    if (ws !== socket) return;
    connectionStatus.value = "disconnected";
    wsClient.value = null;
    ws = null;
    if (isActive) {
      scheduleReconnect();
    }
  };

  socket.onerror = () => {
    // Closing triggers onclose, which schedules a reconnect.
    socket.close();
  };
}

async function scheduleReconnect(): Promise<void> {
  if (reconnectTimer || isSchedulingReconnect) return;
  isSchedulingReconnect = true;
  try {
    if (reconnectAttempts >= MAX_RECONNECT_ATTEMPTS) {
      reconnectAttempts = 0;
    }
    if (!(await isSessionValid())) {
      onSessionLost();
      return;
    }
    const delay = getReconnectDelay();
    reconnectAttempts++;
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null;
      connectWs();
    }, delay);
  } finally {
    isSchedulingReconnect = false;
  }
}

export function disconnectWs(): void {
  isActive = false;
  if (reconnectTimer) {
    clearTimeout(reconnectTimer);
    reconnectTimer = null;
  }
  reconnectAttempts = 0;
  if (ws) {
    ws.onclose = null;
    ws.close();
    ws = null;
  }
  wsClient.value = null;
  connectionStatus.value = "disconnected";
  wsUserId.value = null;
}

export function sendWsMessage(data: Record<string, unknown>): void {
  if (ws?.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(data));
  }
}
