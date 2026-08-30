import { signal } from "@preact/signals-core";
import { getWorkspaces } from "@/api";
import type { DtoUserResponse, DtoWorkspaceResponse } from "@/api";
import { auth } from "@/store/auth";
import { fetchProjects } from "@/store/projects";
import { fetchPinnedViews } from "@/features/views/store";
import { fetchUnreadCount } from "@/features/notifications/store";
import { connectWs, disconnectWs } from "@/store/ws";
import { switchWorkspace } from "@/features/workspaces/api";

export interface WorkspaceState {
  workspaces: DtoWorkspaceResponse[];
  activeOrgID: string;
  isLoading: boolean;
}

export const workspaces = signal<WorkspaceState>({
  workspaces: [],
  activeOrgID: "",
  isLoading: false,
});

/**
 * Hydrate the workspace list from an /auth/me or /auth/login response. Called
 * once after fetchMe/login (and after a workspace switch) so the switcher
 * reflects the current membership set without a separate round-trip.
 */
export function hydrateWorkspaces(
  list: DtoWorkspaceResponse[] | undefined,
  activeOrgID: string | undefined,
): void {
  workspaces.value = {
    workspaces: list ?? workspaces.value.workspaces,
    activeOrgID: activeOrgID ?? workspaces.value.activeOrgID,
    isLoading: false,
  };
}

/** Refetch the workspace list from the server (e.g. after creating one). */
export async function fetchWorkspaces(): Promise<void> {
  workspaces.value = { ...workspaces.value, isLoading: true };
  try {
    const { data } = await getWorkspaces({ throwOnError: true });
    workspaces.value = {
      workspaces: data ?? [],
      activeOrgID: workspaces.value.activeOrgID,
      isLoading: false,
    };
  } catch {
    workspaces.value = { ...workspaces.value, isLoading: false };
  }
}

/**
 * Switch the active workspace. Calls the server (which sets a new session
 * cookie scoped to the target org + revokes the old session), then refreshes
 * auth state and all org-scoped sidebar data (projects, pinned views, unread
 * count) so the UI reflects the new workspace immediately.
 */
export async function switchActiveWorkspace(orgID: string): Promise<void> {
  const me = await switchWorkspace(orgID);
  auth.value = {
    user: me,
    isLoading: false,
    isAuthenticated: true,
  };
  hydrateWorkspaces(me.workspaces, me.active_org_id);
  // Reconnect the WebSocket so it picks up the new session cookie (the old
  // JWT was revoked server-side) and is scoped to the new workspace's
  // presence/messages. Without this, the old-org socket stays open and
  // streams stale data until it drops.
  disconnectWs();
  connectWs();
  // Sidebar data is org-scoped; refetch for the new active workspace.
  fetchProjects();
  fetchPinnedViews();
  fetchUnreadCount();
}

// DtoUserResponse is referenced by switchActiveWorkspace's return; re-export
// to keep the type visible to callers that inspect the refreshed user.
export type { DtoUserResponse };
