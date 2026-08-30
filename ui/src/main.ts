import { client } from "@/api/client.gen";
import { auth, logout } from "@/store/auth";
import { disconnectWs } from "@/store/ws";
import "./app-shell.ts";
import "@/store/motion";
import "@/styles/animations.css";
import "@/styles/motion-disable.css";

const { VITE_API_BASE_URL } = import.meta.env;
client.setConfig({
  baseUrl: VITE_API_BASE_URL || "/api",
  credentials: "include",
});

// Global API error handling
// Intercept errors so a 401 on a non-auth endpoint (i.e. after the session
// has expired) resets auth state and redirects to /login, instead of leaving
// the UI showing stale "authenticated" state and piling up 401 errors.
const AUTH_PATHS = ["/auth/login", "/auth/logout"];
let redirectingToLogin = false;

function isAuthEndpoint(url: string | undefined): boolean {
  return url != null && AUTH_PATHS.some((p) => url.includes(p));
}

async function handleSessionExpired(): Promise<void> {
  if (redirectingToLogin) return;
  redirectingToLogin = true;
  try {
    await logout();
  } catch {
    // Best effort: the cookie is gone either way.
    auth.value = { user: null, isLoading: false, isAuthenticated: false };
  }
  disconnectWs();
  if (window.location.pathname !== "/login") {
    const next = window.location.pathname + window.location.search;
    window.location.href = `/login?next=${encodeURIComponent(next)}`;
  }
  redirectingToLogin = false;
}

// Error interceptors receive (error, response, request, options). The
// response carries the HTTP status; the request the URL.
client.interceptors.error.use(
  async (error, response, request) => {
    const status = response?.status;
    const url = request?.url;
    if (status === 401 && !isAuthEndpoint(url) && auth.value.isAuthenticated) {
      await handleSessionExpired();
    }
    throw error;
  },
);

// WebSocket session-expired handling
// The WS layer dispatches this when a reconnect probe to /api/auth/me returns
// 401. Reset auth state and redirect to login.
window.addEventListener("breeze:session-expired", () => {
  if (redirectingToLogin) return;
  handleSessionExpired();
});
