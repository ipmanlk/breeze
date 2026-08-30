import { signal } from "@preact/signals-core";
import { getAuthMe, postAuthLogin, postAuthLogout } from "@/api";
import type { DtoUserResponse } from "@/api";
import { hydrateWorkspaces } from "@/store/workspaces";

export interface AuthState {
  user: DtoUserResponse | null;
  isLoading: boolean;
  isAuthenticated: boolean;
}

export const auth = signal<AuthState>({
  user: null,
  isLoading: true,
  isAuthenticated: false,
});

/** Apply a /auth/me-shaped response to auth + workspace signals. */
function applyUser(user: DtoUserResponse | null): void {
  auth.value = {
    user,
    isLoading: false,
    isAuthenticated: user != null,
  };
  if (user) {
    hydrateWorkspaces(user.workspaces, user.active_org_id);
  }
}

export async function fetchMe(): Promise<void> {
  try {
    const { data } = await getAuthMe({ throwOnError: true });
    applyUser(data ?? null);
  } catch {
    auth.value = { user: null, isLoading: false, isAuthenticated: false };
  }
}

export async function login(email: string, password: string): Promise<void> {
  const { data } = await postAuthLogin({
    body: { email, password },
    throwOnError: true,
  });
  applyUser(data ?? null);
}

export async function logout(): Promise<void> {
  try {
    await postAuthLogout({ throwOnError: true });
  } finally {
    // Full page navigation, not a soft route change. Module-level stores
    // (chat messages, task caches, sidebar data) and app-shell one-shot
    // flags (#sidebarLoaded/#themeLoaded/#wsConnected) would otherwise
    // leak the previous user's data into the next session on this tab.
    window.location.assign("/login");
  }
}
